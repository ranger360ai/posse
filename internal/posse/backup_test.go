package posse

// `posse backup` (ADR 0036, build bead ranger-base-a0ln0): the archive, the
// verify that gates publication, the disk floor, single-flight, and prune.
//
// Every test here builds its own queue repo and its own home under
// t.TempDir. None reads the operator's — a backup test that ran against the
// live store would be slow, and would be writing 45MB archives into
// somebody's state dir on every `go test ./...`.

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

var backupAt = time.Date(2026, 9, 1, 3, 15, 0, 0, time.UTC)

// backupRig is an instance with a queue repo, a constitution home carrying
// both the archived set and the two directories that must never be archived,
// and a backup dir. It returns the app and the queue repo.
func backupRig(t *testing.T) (*App, string) {
	t.Helper()
	root := t.TempDir()
	queue := filepath.Join(root, "queue")
	store := filepath.Join(queue, ".beads")
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	mustGit(t, queue, "init", "-q", "-b", "main", ".")
	mustGit(t, queue, "config", "user.email", "t@example.com")
	mustGit(t, queue, "config", "user.name", "t")
	write(t, filepath.Join(store, beadsJSONL), `{"id":"x-1","title":"one"}`+"\n")
	write(t, filepath.Join(store, beadsDeleted), `{"id":"x-0"}`+"\n")
	mustSqlite(t, filepath.Join(store, "beads.db"), "create table issues(id text); insert into issues values('x-1');")
	mustGit(t, queue, "add", "--", filepath.Join(".beads", beadsJSONL), filepath.Join(".beads", beadsDeleted))
	mustGit(t, queue, "commit", "-q", "-m", "seed", "--", filepath.Join(".beads", beadsJSONL), filepath.Join(".beads", beadsDeleted))

	home := filepath.Join(root, "home")
	a := NewAppAt(home)
	write(t, a.ConfigPath, "queue_repo: "+queue+"\n")
	write(t, filepath.Join(home, "agents", "dev.md"), "name: dev\n")
	write(t, filepath.Join(home, "recipes", "r.yaml"), "cmd: true\n")
	write(t, filepath.Join(home, "runtimes", "claude.yaml"), "name: claude\n")
	write(t, filepath.Join(home, PromoteManifestFile), `{"version":1,"files":{}}`+"\n")
	// The two that must never appear in an archive.
	write(t, filepath.Join(home, ConstitutionEnvsDir, "prod.env"), "ANTHROPIC_API_KEY=sk-live\n")
	write(t, filepath.Join(home, "secrets", "anthropic.env"), "ANTHROPIC_API_KEY=sk-live\n")
	write(t, filepath.Join(home, "personas", "dev", "ORDERS.md"), "orders\n")
	return a, queue
}

// mustSqlite makes the rig's beads db. sqlite3 is a hard dependency of the
// verb (ADR 0036 §2, preflighted with its own exit), so a box without it
// SKIPS rather than reds: the test would be measuring the box.
func mustSqlite(t *testing.T, db, sql string) {
	t.Helper()
	out, err := exec.Command("sqlite3", db, sql).CombinedOutput()
	if err != nil {
		t.Skipf("sqlite3 is not usable here: %s", strings.TrimSpace(string(out)))
	}
}

// mustBackup runs one backup at a fixed clock and fails the test on any
// refusal — the tests that WANT a refusal call RunBackup themselves.
func mustBackup(t *testing.T, a *App, at time.Time) BackupResult {
	t.Helper()
	res, err := a.RunBackup(BackupOpts{Out: io.Discard, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	return res
}

// tarNames is every member name in a published archive, in archive order.
func tarNames(t *testing.T, archive string) []string {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	return names
}

// ─── the archive ─────────────────────────────────────────────────────────────

// The archive holds both stores, and the manifest is the FIRST entry —
// which is not cosmetic: VerifyBackup streams the tar once and needs the
// hashes before the members arrive.
func TestBackupArchivesBothStoresManifestFirst(t *testing.T) {
	a, _ := backupRig(t)
	res := mustBackup(t, a, backupAt)

	names := tarNames(t, res.Archive)
	if len(names) == 0 || names[0] != backupManifestName {
		t.Fatalf("first entry is %q, want %s — a verifier streams and cannot seek back for it", names[0], backupManifestName)
	}
	for _, want := range []string{
		"queue/queue.bundle", "queue/beads.db", "queue/issues.jsonl", "queue/deleted.jsonl",
		"home/config.yaml", "home/agents/dev.md", "home/recipes/r.yaml",
		"home/runtimes/claude.yaml", "home/" + PromoteManifestFile,
	} {
		if !containsPrefix(names, want) {
			t.Errorf("the archive is missing %s:\n%s", want, strings.Join(names, "\n"))
		}
	}
}

// "envs NEVER — secrets stay out" (the 2026-09-01 sub-ruling). The archive
// is a file an operator will reasonably copy around, and a token in it is a
// token in every copy. personas/ and state/ are out too, and the manifest
// says all four are out by name so a restore reads an absence as policy.
func TestBackupNeverArchivesSecrets(t *testing.T) {
	a, _ := backupRig(t)
	res := mustBackup(t, a, backupAt)

	for _, name := range tarNames(t, res.Archive) {
		for _, banned := range []string{"envs", "secrets", "personas", "state"} {
			if strings.HasPrefix(name, "home/"+banned) {
				t.Errorf("the archive holds %s — %s/ may never be archived", name, banned)
			}
		}
	}
	// The control: the same walk DOES find something under home/, so an
	// empty archive could not have passed the loop above.
	if !containsPrefix(tarNames(t, res.Archive), "home/agents/") {
		t.Fatal("no home/ member at all — the exclusion loop above proved nothing")
	}
	sort.Strings(res.Manifest.Excluded)
	if got, want := strings.Join(res.Manifest.Excluded, ","), "envs,personas,secrets,state"; got != want {
		t.Errorf("manifest excluded = %q, want %q", got, want)
	}
}

// ADR 0036 verification observable 1: the sources are untouched. The db is
// read through sqlite's backup API and the repo through `git bundle`, and
// neither may check point, rewrite or restat anything.
func TestBackupLeavesTheSourcesAlone(t *testing.T) {
	a, queue := backupRig(t)
	store := beadsHome(queue)
	before := map[string][2]int64{}
	ents, err := os.ReadDir(store)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		st, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		before[e.Name()] = [2]int64{st.Size(), st.ModTime().UnixNano()}
	}
	mustBackup(t, a, backupAt)
	for name, want := range before {
		st, err := os.Stat(filepath.Join(store, name))
		if err != nil {
			t.Errorf("%s went away: %v", name, err)
			continue
		}
		if got := [2]int64{st.Size(), st.ModTime().UnixNano()}; got != want {
			t.Errorf("%s changed: size/mtime %v, was %v", name, got, want)
		}
	}
}

// ─── the verify that gates publication ───────────────────────────────────────

// The mutation arm ADR 0036 §7 asks for, on the arm this build kept: flip
// one byte of a published archive and the verify must go red. A verify that
// cannot go red measures nothing.
func TestVerifyCatchesAFlippedByte(t *testing.T) {
	a, _ := backupRig(t)
	res := mustBackup(t, a, backupAt)
	if _, err := VerifyBackup(res.Archive); err != nil {
		t.Fatalf("the archive posse just published does not verify: %v", err)
	}

	body, err := os.ReadFile(res.Archive)
	if err != nil {
		t.Fatal(err)
	}
	body[len(body)/2] ^= 0xff
	corrupt := filepath.Join(t.TempDir(), filepath.Base(res.Archive))
	if err := os.WriteFile(corrupt, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyBackup(corrupt); err == nil {
		t.Fatal("a flipped byte verified green — the verify measures nothing")
	}
}

// The same arm one level up: the sidecar. A copy whose bytes changed but
// whose sidecar did not is caught before the tar is even opened.
func TestVerifyCatchesASidecarMismatch(t *testing.T) {
	a, _ := backupRig(t)
	res := mustBackup(t, a, backupAt)
	if err := os.WriteFile(res.Sidecar, []byte(strings.Repeat("0", 64)+"  x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := VerifyBackup(res.Archive)
	if err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("a wrong sidecar verified as %v, want a sidecar refusal", err)
	}
}

// Publication is gated on the verify, which is what makes the DIRECTORY the
// freshness store (ADR 0036 §6 — one fact, one owner): a file that is there
// verified, and there is no second stamp to disagree with it.
//
// Two halves. First: an archive whose manifest names a member the tar does
// not carry is red — the shape a truncated write leaves behind, hand-built
// here because a run that produced one would be the bug.
func TestVerifyCatchesAMissingMember(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, backupPrefix+backupAt.Format(backupStamp)+backupSuffix)
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	man := BackupManifest{
		Version:   backupManifestVersion,
		CreatedAt: backupAt.Format(time.RFC3339),
		Members:   []BackupMember{{Name: "queue/issues.jsonl", Bytes: 1, SHA256: strings.Repeat("0", 64)}},
	}
	body, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: backupManifestName, Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	for _, c := range []io.Closer{tw, gz, f} {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := VerifyBackup(archive); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("an archive missing a member its manifest names verified as %v", err)
	}
}

// The gate's RED path. The three arms above all call VerifyBackup by hand
// over a copy of an already-published archive, so they say the verify can go
// red — not that RunBackup's own call to it is load-bearing. Delete the three
// gate lines from RunBackup and every one of them stays green.
//
// ADR 0036 arm 8's doctrine is "a drill that cannot go red measures nothing",
// and the 2026-09-01 sub-ruling folded that drill INTO this gate, so this is
// the folded drill's red arm (ranger-base-31p1b, found verifying
// ranger-base-x3de).
//
// afterStage is the seam: it rewrites one staged member with same-size,
// different bytes after the manifest has hashed it and before the tar reads
// those same files back. Same size matters — a size change is caught by
// writeBackupArchive's own "changed size while it was being archived" arm,
// which is a different branch and would leave this one unmeasured.
func TestARunThatFailsItsOwnVerifyPublishesNothing(t *testing.T) {
	a, _ := backupRig(t)
	dir := a.BackupDir()

	staged := false
	_, err := a.RunBackup(BackupOpts{
		Out: io.Discard,
		Now: func() time.Time { return backupAt },
		afterStage: func(stage string) {
			staged = true
			member := filepath.Join(stage, "home", "agents", "dev.md")
			body, rerr := os.ReadFile(member)
			if rerr != nil || len(body) == 0 {
				t.Fatalf("staged %s: %v (%d bytes) — the seam has nothing to corrupt", member, rerr, len(body))
			}
			body[0] ^= 0x01
			if werr := os.WriteFile(member, body, 0o600); werr != nil {
				t.Fatal(werr)
			}
		},
	})
	if !staged {
		t.Fatal("the seam never ran, so the run refused before it staged anything and nothing below is about the verify")
	}

	// The run returns a refusal, and it says what happened: the archive was
	// written, it did not verify, and it was therefore not published.
	// Not fatal: the three observables are separate claims, and a run that
	// returned nil here is exactly the run whose store must still be checked.
	if err == nil || !strings.Contains(err.Error(), "was not published") {
		t.Errorf("a run whose own archive fails verify returned %v, want the refusal that says it was not published", err)
	}
	// Nothing survives it. Read the whole directory rather than stat the two
	// names this run would have used: an archive published under any name,
	// and a .part or a staging tree left behind, are the same defect.
	ents, rerr := os.ReadDir(dir)
	if rerr != nil {
		t.Fatal(rerr)
	}
	for _, e := range ents {
		if e.Name() == ".lock" {
			continue // the single-flight lock, which outlives every run
		}
		t.Errorf("a run that failed its own verify left %s in %s", e.Name(), AbbrevHome(dir))
	}
	if names, lerr := listBackups(dir); lerr != nil || len(names) != 0 {
		t.Fatalf("listBackups = %v (err %v) after a failed verify, want none", names, lerr)
	}

	// The control, on the same rig and the same clock: with the seam gone the
	// run publishes and the archive verifies. So the red above is the byte
	// this test changed and not a rig that could never have produced a
	// backup at all.
	res := mustBackup(t, a, backupAt)
	if _, verr := VerifyBackup(res.Archive); verr != nil {
		t.Fatalf("the control run does not verify: %v", verr)
	}
	if names, lerr := listBackups(dir); lerr != nil || len(names) != 1 {
		t.Fatalf("listBackups = %v (err %v) after the control run, want exactly one", names, lerr)
	}
}

// Second: a run in flight is not a backup. The `.part` and the staging
// directory carry no verdict, so listBackups must not count either — and a
// finished run leaves neither behind.
func TestARunInFlightIsNotABackup(t *testing.T) {
	a, _ := backupRig(t)
	dir := a.BackupDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(dir, backupPrefix+backupAt.Format(backupStamp)+backupSuffix+backupPartSuffix)
	write(t, part, "half an archive")
	if err := os.MkdirAll(filepath.Join(dir, ".staging-x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if names, err := listBackups(dir); err != nil || len(names) != 0 {
		t.Fatalf("listBackups = %v (err %v) over a .part and a staging dir, want none", names, err)
	}
	// The control: the same listing DOES see a real one, so the zero above
	// is the filter and not an unreadable directory.
	res := mustBackup(t, a, backupAt.Add(time.Hour))
	names, err := listBackups(dir)
	if err != nil || len(names) != 1 || names[0] != filepath.Base(res.Archive) {
		t.Fatalf("listBackups = %v (err %v), want just %s", names, err, filepath.Base(res.Archive))
	}
	// And a finished run leaves nothing of its own behind.
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".staging-") || strings.HasSuffix(e.Name(), backupPartSuffix) {
			if e.Name() == filepath.Base(part) || e.Name() == ".staging-x" {
				continue // the two this test planted
			}
			t.Errorf("the run left %s behind", e.Name())
		}
	}
}

// ─── refusals ────────────────────────────────────────────────────────────────

func TestBackupRefusesWithNoQueueRepo(t *testing.T) {
	t.Parallel()
	a, _ := backupRig(t)
	write(t, a.ConfigPath, "")
	_, err := a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "queue_repo") {
		t.Fatalf("err = %v, want a refusal naming queue_repo:", err)
	}
}

// ADR 0036 §3 and verification observable 2: the 2c ruling enforced on the
// SOURCE. A queue repo that grew a remote already has an off-box copy posse
// did not sanction, and backup refuses over it rather than making a second.
// ADR 0049 observable 1 widens it: with `queue_remote:` unset the refusal
// stands, and the line names the key as the sanctioned way out.
func TestBackupRefusesAQueueRepoWithARemote(t *testing.T) {
	a, queue := backupRig(t)
	mustGit(t, queue, "remote", "add", "origin", "https://example.com/queue.git")
	_, err := a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "remote") {
		t.Fatalf("err = %v, want a refusal naming the remote", err)
	}
	if !strings.Contains(err.Error(), "queue_remote:") {
		t.Errorf("err = %v, want the refusal to name config queue_remote: as the way out (ADR 0049 D1)", err)
	}
	// And it heals: the refusal is about the repo's state, not a latch.
	mustGit(t, queue, "remote", "remove", "origin")
	mustBackup(t, a, backupAt)
}

// The disk floor: refuse rather than fill the disk, and say the number.
func TestBackupRefusesBelowTheDiskFloor(t *testing.T) {
	t.Parallel()
	a, _ := backupRig(t)
	appendConfig(t, a, "backup_min_free_mb: 999999999\n")
	_, err := a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "backup_min_free_mb") {
		t.Fatalf("err = %v, want a refusal naming the floor", err)
	}
}

// Single-flight (ADR 0036 §3): two callers, one writer. The lock is held by
// the open file description, so this test takes it the way another process
// would and asserts the refusal names the directory.
func TestBackupIsSingleFlight(t *testing.T) {
	a, _ := backupRig(t)
	dir := a.BackupDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	held, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := flock(held, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	_, err = a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("err = %v, want the single-flight refusal", err)
	}
	// The control: released, the same call succeeds — so the refusal above
	// was the lock and not the rig.
	flock(held, syscall.LOCK_UN)
	held.Close()
	mustBackup(t, a, backupAt)
}

// ─── retention ───────────────────────────────────────────────────────────────

// Prune keeps the newest `backup_keep:` and never runs before the new
// archive has verified — which this asserts positionally: the archive from
// the last run is always among the survivors.
func TestBackupPrunesToKeepNewest(t *testing.T) {
	a, _ := backupRig(t)
	appendConfig(t, a, "backup_keep: 2\n")
	var last BackupResult
	for i := 0; i < 4; i++ {
		last = mustBackup(t, a, backupAt.Add(time.Duration(i)*time.Hour))
	}
	names, err := listBackups(a.BackupDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("kept %d archives, want 2: %v", len(names), names)
	}
	if names[len(names)-1] != filepath.Base(last.Archive) {
		t.Errorf("the newest kept is %s, want the one just written (%s)", names[len(names)-1], filepath.Base(last.Archive))
	}
	// Sidecars go with their archives; an orphan sidecar is a hash for a
	// file nobody has.
	ents, _ := os.ReadDir(a.BackupDir())
	sidecars := 0
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), backupSidecarSuffix) {
			sidecars++
		}
	}
	if sidecars != 2 {
		t.Errorf("%d sidecars for 2 archives", sidecars)
	}
	// keep: 0 would be "delete the copy you just made" and is refused into
	// the default rather than honoured.
	write(t, a.ConfigPath, "backup_keep: 0\n")
	if got := a.BackupKeep(io.Discard); got != DefaultBackupKeep {
		t.Errorf("backup_keep: 0 read as %d, want the default %d", got, DefaultBackupKeep)
	}
}

// A queue repo with no commits yet is not a reason to refuse: `git bundle`
// will not write an empty bundle, and a freshly cut queue is exactly the
// state an operator most wants a first archive of. The store still travels;
// the missing journal is said out loud.
func TestBackupOfAQueueWithNoCommits(t *testing.T) {
	a, queue := backupRig(t)
	fresh := filepath.Join(t.TempDir(), "fresh")
	if err := os.MkdirAll(filepath.Join(fresh, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustGit(t, fresh, "init", "-q", "-b", "main", ".")
	write(t, filepath.Join(fresh, ".beads", beadsJSONL), `{"id":"y-1"}`+"\n")
	mustSqlite(t, filepath.Join(fresh, ".beads", "beads.db"), "create table issues(id text);")
	write(t, a.ConfigPath, "queue_repo: "+fresh+"\n")
	_ = queue

	var say strings.Builder
	res, err := a.RunBackup(BackupOpts{Out: &say, Now: func() time.Time { return backupAt }})
	if err != nil {
		t.Fatalf("a queue with no commits refused the whole backup: %v", err)
	}
	names := tarNames(t, res.Archive)
	if !containsPrefix(names, "queue/issues.jsonl") || !containsPrefix(names, "queue/beads.db") {
		t.Errorf("the store did not travel: %v", names)
	}
	if containsPrefix(names, "queue/queue.bundle") {
		t.Errorf("a repo with no commits produced a bundle: %v", names)
	}
	if !strings.Contains(say.String(), "no commits") {
		t.Errorf("the missing journal was silent: %q", say.String())
	}
	// The control: the rig's real repo, one commit, DOES carry a bundle —
	// so the assertion above is about the commits and not about the code
	// having quietly stopped bundling.
	write(t, a.ConfigPath, "queue_repo: "+queue+"\n")
	res = mustBackup(t, a, backupAt.Add(time.Hour))
	if !containsPrefix(tarNames(t, res.Archive), "queue/queue.bundle") {
		t.Error("a repo with a commit produced no bundle")
	}
}

// A directory that is not a git repository is refused by name, rather than
// failing later inside a git call nobody can read.
func TestBackupRefusesAQueueThatIsNotARepo(t *testing.T) {
	t.Parallel()
	a, _ := backupRig(t)
	plain := t.TempDir()
	if err := os.MkdirAll(filepath.Join(plain, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, a.ConfigPath, "queue_repo: "+plain+"\n")
	_, err := a.RunBackup(BackupOpts{Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("err = %v, want a refusal naming the repository", err)
	}
}

// Two archives in one second: the second gets a disambiguator, and the
// ordering must still call it the newer one. A plain string sort does not —
// '-' sorts before '.', so `…Z-2.tar.gz` would read as older than
// `…Z.tar.gz` and prune would reach the wrong one of the pair.
func TestSameSecondArchivesOrderByTheirSequence(t *testing.T) {
	dir := t.TempDir()
	stamp := backupAt.UTC().Format(backupStamp)
	base := backupPrefix + stamp + backupSuffix
	second := backupPrefix + stamp + "-2" + backupSuffix
	write(t, filepath.Join(dir, base), "x")
	write(t, filepath.Join(dir, second), "x")
	names, err := listBackups(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[1] != second {
		t.Fatalf("listBackups = %v, want the -2 archive last (it is the later one)", names)
	}
	// And a name that does not carry the stamp at all is not an archive:
	// the reader dates archives by their NAME, so a file it cannot date is
	// one it must not count.
	write(t, filepath.Join(dir, backupPrefix+"nonsense"+backupSuffix), "x")
	if names, _ := listBackups(dir); len(names) != 2 {
		t.Errorf("an undatable name was counted as an archive: %v", names)
	}
}

// Retention reports; it never becomes the run's verdict. By the time it
// runs the archive is published, so an error here would make a good backup
// read as a failed one — and one undeletable candidate must not stop the
// rest of the sweep.
//
// The undeletable candidate is REAL, not asserted: a rig where nothing can
// fail proves nothing about a loop that keeps going past a failure. macOS
// gives a non-root owner exactly one way to make unlink(2) fail on their own
// file — the user-immutable flag — and this uses it; elsewhere the arm is
// skipped rather than faked.
func TestPruneReportsAndKeepsGoing(t *testing.T) {
	dir := t.TempDir()
	var names []string
	for i := 0; i < 5; i++ {
		n := backupPrefix + backupAt.Add(time.Duration(i)*time.Hour).UTC().Format(backupStamp) + backupSuffix
		write(t, filepath.Join(dir, n), "x")
		names = append(names, n)
	}
	// The OLDEST is the first candidate the loop reaches, so a loop that
	// stops at a failure stops before it has pruned anything.
	oldest := filepath.Join(dir, names[0])
	immutable(t, oldest)

	var say strings.Builder
	pruned := pruneBackups(&say, dir, 2)
	if len(pruned) != 2 {
		t.Fatalf("pruned %v, want the two it COULD remove — the loop stopped at the one it could not", pruned)
	}
	if !fileExists(oldest) {
		t.Error("the immutable archive was removed; the rig is not testing what it claims")
	}
	for _, n := range names[3:] {
		if !fileExists(filepath.Join(dir, n)) {
			t.Errorf("%s is inside the keep window and was pruned", n)
		}
	}
	if !strings.Contains(say.String(), "left "+names[0]+" in place") {
		t.Errorf("the failure was silent: %q", say.String())
	}
	if !strings.Contains(say.String(), "pruned "+names[1]) {
		t.Errorf("the removals were silent: %q", say.String())
	}
	// An unreadable directory is a line, not a panic and not a verdict —
	// pruneBackups has no error to return by construction.
	var say2 strings.Builder
	if got := pruneBackups(&say2, filepath.Join(dir, "nope"), 1); got != nil {
		t.Errorf("pruned %v out of a directory that does not exist", got)
	}
}

// immutable makes p undeletable by its own owner, and registers the undo —
// without it t.TempDir's cleanup fails and takes an unrelated test with it.
func immutable(t *testing.T, p string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("the user-immutable flag is the darwin/BSD way to make unlink fail for the owner")
	}
	if out, err := exec.Command("chflags", "uchg", p).CombinedOutput(); err != nil {
		t.Skipf("chflags uchg is not usable here: %s", strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() { exec.Command("chflags", "nouchg", p).Run() })
	if err := os.Remove(p); err == nil {
		t.Fatal("the immutable file removed anyway — this rig cannot fail, so it measures nothing")
	}
}
