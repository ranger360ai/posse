//go:build posse_arm3

package posse

// Live pins for rangerhq-09o2's beads clause, run against the real bd rather
// than the fake:
//
//	a persona in its session worktree reads and writes the SAME work graph
//	as the main checkout — no forked database, no staleness warning, and no
//	--allow-stale — and the store posse tells the CAGE about is the one bd
//	actually opens.
//
//	RHQ_LIVE_BD=1 go test ./internal/posse -run TestLiveWorktree -v
//
// Env-gated and skipped by default, like the other live pins: it shells out
// to the operator's bd, which has a version, a daemon and a cache, and none
// of those belong in a hermetic suite. Everything here happens inside one
// t.TempDir — the `bd init` calls are the throwaway-database case, never a
// repo anybody keeps.
//
// WHICH HALF IS WHOSE (measured 2026-08-28, bd 0.49.1, git 2.39.3 —
// ranger-base-vczf, and it corrects what worktree.go used to say):
//
//   - bd resolves a linked worktree to the MAIN checkout's `.beads` on its
//     own, and posse's `.beads/redirect` is not what does it. Every "one
//     graph" assertion below — the main checkout's rows visible in the
//     worktree, no database of its own, a bead filed from the worktree
//     landing in the main database, no staleness warning — holds with
//     seedBeadsRedirect suppressed entirely. Measured in all three shapes:
//     the worktree carrying a checked-out `.beads` with a tracked
//     issues.jsonl, the worktree carrying no `.beads` at all, and the main
//     checkout holding a jsonl but no database yet (the "fresh clone" case,
//     where bd builds the database in the MAIN checkout, not in the
//     worktree). So an arm of the form "did the graph fork" cannot fail for
//     want of the redirect on this bd, and TestLiveWorktreeBdResolvesThe-
//     WorktreeItself is the pin that says so out loud — the day bd loses
//     that resolution it goes red, and posse's redirect becomes the thing
//     holding the graph together.
//
//   - What the redirect posse seeds is load-bearing for is POSSE, not bd.
//     beadsHome (beadloss.go) reads it, and the seatbelt writable set and
//     the codex launch line are built from what it answers (ADR 0012 D3-C;
//     TestCodexLaunchLineNamesTheStoreOfRecord). A caged persona whose
//     grant names a directory bd never opens gets `failed to open database:
//     … operation not permitted` out of a resolution that was perfectly
//     correct — which is ranger-base-0fb, verbatim. So the live claim on
//     our own code is AGREEMENT: the directory posse seeds and resolves is
//     the directory bd resolves to. That is the arm that fails when
//     seedBeadsRedirect does not run.
//
// WHICH bd, AND WHICH STORE CLASS (measured 2026-09-04, bd 0.50.3 —
// ranger-base-9lrzx). Everything above was measured on bd 0.49.1, where a
// plain `bd init` built a SQLite-backed store. On 0.50.3 the pins in this
// file still hold FOR THAT CLASS — the worktree resolution, the redirect
// precedence, the agreement arm and the staleness trap all pass against it.
// (The one shape above no pin covers is the "fresh clone", and a spot-check
// says the half that matters here survives too: with the database deleted
// and the tracked jsonl left, bd rebuilds it in the MAIN checkout and not in
// the worktree. It came back empty and prefixless in that spot-check, which
// is not pinned and was not chased — see ranger-base-9lrzx.) What did change
// on 0.50.3 is which class `bd init` builds by default: the default is
// `--backend dolt`, and a bd built without CGO — the binary on this box —
// falls back to a no-db (JSONL-only) store with nothing but a note on stdout ("dolt backend requires CGO (not available in this build).
// Falling back to JSONL-only mode."). The class is not cosmetic. A redirect
// into a no-db store is resolved by `bd where` and then ignored by the reads
// and the writes, which go to the LOCAL `.beads/issues.jsonl` instead —
// TestLiveWorktreeNoDbStoreForksTheGraph pins that shape, and it is why the
// witness arm at the bottom of TestLiveWorktreeBdResolvesTheWorktreeItself
// went dead rather than red. So liveBeadsRepo NAMES the class rather than
// taking this box's default; the operator's own queue is SQLite, so SQLite
// is both the class these findings were measured on and the class posse
// actually runs against.
//
// WHICH CLAIMS ARE DATABASE-MODE ONLY, AND WHAT CLOSES THE REST (measured
// 2026-09-04, bd 0.50.3 — ranger-base-e3ima, ADR 0055). Read the paragraph
// above as scoped: every "one graph" finding in this file — the worktree
// resolution, the redirect precedence, the AGREEMENT arm, the staleness trap
// — is a finding about bd in DATABASE mode. None of them holds when bd is in
// no-db mode, and no-db is a MODE rather than a class: `no-db: true` in the
// store's config.yaml is one door, `--no-db` on the command line is another,
// and posse opens that one itself at the CONTAINER tier and only there
// (CageBdFlags, written onto the inner PATH by renderCageBd, whose one
// non-test call site is the container inner render in cageinner.go), over
// whatever class the store happens to be. So at that tier the fork is the
// shipped configuration, not an accident of how somebody's `bd init` went.
// NOT "every caged session", which this paragraph used to say
// (ranger-base-zt61m, escaped from ranger-base-43ux4 into the very file that
// is the live pin for the claim): a `cage: seatbelt` seat's `bd` is the
// rendered gate shim, which carries no `--no-db` at all — measured from
// inside a seatbelt seat, `grep -c no-db $RHQ_GATES_DIR/bin/bd` -> 0.
// TestLiveWorktreeNoDbStoreForksTheGraph runs both doors.
//
// What closes it is the launch ENVIRONMENT and not this file: every session
// posse launches carries `BEADS_DIR=beadsHome(dir)` (planLaunch,
// herdrback.go; ADR 0055 D1), and with it the no-db create from the worktree
// lands in the MAIN store and the worktree's `bd list` reads it back — on
// both doors, which is the second arm of each cell there. The variable is
// shown not to move the database class in TestLiveWorktreeSharesOneGraph,
// whose "filed from the worktree" arm runs once more under it: same rows,
// same database, no staleness warning, no database of the worktree's own.
// What no pin here can fix is a `bd` run with the variable shed (`env -u
// BEADS_DIR`, `env -i`) — the resolution is bd's, and seedBeadsRedirect is
// already naming the right directory.
//
// Also measured 2026-08-25, and still true: the worktree's own checked-out
// issues.jsonl, materialized with a fresh mtime, raises no staleness warning
// however far forward its mtime is moved — bd checks the jsonl beside the
// database it RESOLVED to. Touching the MAIN repo's jsonl forward is what
// raises "Database out of sync with JSONL", which is a fact about the main
// checkout and not about worktrees.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// liveBd is the shelling-out half these pins share.
//
// `--no-daemon` on EVERY call, and it is the fix for ranger-base-42mv rather
// than a speed knob. bd 0.49.1 auto-starts a per-database daemon on first use
// and nothing ever stops it, so a call in a t.TempDir leaves a process
// holding a directory that is about to be deleted — this test filed two such
// orphans on 2026-08-25, and the daemon holding the temp dirs open is what
// defeated t.TempDir's own cleanup, so it leaked a process AND a directory.
// Measured 2026-08-28 in a throwaway repo: `init` + `create` + `list` plain
// leaves one daemon and a .beads/daemon.pid; the same three with
// `--no-daemon` leave neither and read back the same rows. The daemon is not
// part of the claim here — the claim is which database bd resolves to, which
// it decides before it ever reaches for a socket. (Contrast liveCageBeadStore
// in cageinnerlive_test.go, where a RUNNING daemon is the claim: it imports a
// newer JSONL before answering. That fixture keeps its daemon and stops it in
// cleanup instead.)
func liveBd(t *testing.T) func(dir string, args ...string) (string, error) {
	t.Helper()
	run := liveBdEnv(t)
	return func(dir string, args ...string) (string, error) {
		return run(nil, dir, args...)
	}
}

// liveBdEnv is liveBd with extra environment for the one call, because from
// ADR 0055 D5 onward the environment is the subject: `BEADS_DIR` is the whole
// difference between an arm where the graph forks and an arm where it does
// not, so a runner that cannot vary it cannot ask the question. Appended
// LAST, so it beats whatever the inherited set carried in — the same
// precedence planLaunch gives it over the env sets (herdrback.go).
//
// And SHED from the inherited set rather than merely overridden by the arms
// that want it (ranger-base-zt61m, escaped from ranger-base-e3ima): an arm
// that hands in `env == nil` is a SHED arm — cell A of each door below is
// named "no BEADS_DIR" and is the arm that has to be able to fail for cell B
// to say anything — and appending to a bare `os.Environ()` made it "whatever
// this session already had" instead. That is not a hypothetical inheritance:
// ADR 0055 D1 is the decision that EVERY posse-launched session carries the
// variable, so the sessions these pins run in are exactly the ones that have
// it. Measured 2026-09-04 at 63d44db, before this line: with `BEADS_DIR` set
// to a scratch store, both doors of TestLiveWorktreeNoDbStoreForksTheGraph
// went red, and not only in cell A — the fixture's own plain `bd` calls
// diverted too, so its seed row "in the main checkout" was written into the
// scratch store and the failure named `git commit ... nothing to commit`
// rather than the environment. It fails CLOSED either way, which is why this
// is debt and not a bug; a red that names the wrong cause is still a red an
// operator spends the evening on.
//
// The value handed in is always `beadsHome(<the session tree>)`, never a path
// typed at the call site: that is what planLaunch computes, and a literal
// would pin the fixture's own arithmetic instead of the resolver's (D5 item
// 3).
//
// One trap worth naming, since every store here lives in a t.TempDir: bd
// refuses a `BEADS_DIR` under its unsafe prefixes, and `/private` — what
// macOS resolves a temp dir through — is one of them. It does not fire here
// because `isPathInSafeBoundary` allows anything under the resolved
// `os.TempDir()` first (read in bd 0.49.1's internal/beads/context.go;
// measured on 0.50.3: a `BEADS_DIR` under /var/folders is accepted by `where`
// and by `create`). bd reads its OWN os.TempDir, so the escape holds only
// while t.TempDir and bd agree on $TMPDIR — a GOTMPDIR that moves t.TempDir
// out from under $TMPDIR would land these arms on "BEADS_DIR points to unsafe
// location", which is the fixture talking and not the claim.
func liveBdEnv(t *testing.T) func(env []string, dir string, args ...string) (string, error) {
	t.Helper()
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}
	return func(env []string, dir string, args ...string) (string, error) {
		cmd := exec.Command("bd", append([]string{"--no-daemon"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(append(shedBeadsDir(os.Environ()), "PATH="+PathOutsideGates("")), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
}

// beadsDirEnv is the launch's own answer for a session tree, in the form bd
// reads it. Fatals rather than returning a wrong one: an arm whose BEADS_DIR
// names the worktree's own `.beads` IS the fork, and would pass the "the row
// is in the worktree jsonl" half of every cell below while proving nothing.
//
// The guard is not theoretical, and a mutant is what said so (2026-09-04,
// ranger-base-e3ima): with `BEADS_DIR` pointed at the worktree's own `.beads`
// the DATABASE-class arm still passes, because bd's redirect detection runs
// against a pre-set `BEADS_DIR` too (bd-wayc3, ADR 0055 Consequences) and the
// redirect posse seeded there hands it back to the main store. Only the no-db
// cells go red, because no-db mode reads no redirect at all. So a wrong value
// here is caught by half the pins in this file and not the other half — which
// is exactly the shape a guard is for.
func beadsDirEnv(t *testing.T, tree, mainRepo string) []string {
	t.Helper()
	home := beadsHome(tree)
	if !isDirPath(home) {
		t.Fatalf("beadsHome(%s) answers %q, which is not a directory — planLaunch would set no BEADS_DIR at all and this arm has nothing to measure", tree, home)
	}
	if got, want := resolveExisting(home), resolveExisting(filepath.Join(mainRepo, ".beads")); got != want {
		t.Fatalf("beadsHome(%s) answers %q, not the main checkout's %q — the env below would carry the fork rather than the fix", tree, got, want)
	}
	return []string{"BEADS_DIR=" + home}
}

// stopLeakedDaemons is the backstop, because `--no-daemon` on OUR calls does
// not cover every bd invocation these tests cause. `bd init` installs bd's own
// git pre-commit hook, and that hook runs a bare `bd sync --flush-only` with
// no such flag — so a `git commit` starts a daemon in the throwaway repo
// whatever we pass (measured 2026-08-28: with every direct call carrying
// --no-daemon, one daemon and one .beads/daemon.pid still appear, written by
// the hook). That is a second leak vector, it is bd's hook rather than our
// code, and cleanup is the only lever we have on it.
//
// The pid file was written beside this fixture's own database seconds ago, in
// a directory nothing else has ever seen, so signalling it is not a guess
// about somebody else's process — the shape liveCageBeadStore uses, and never
// a global `bd daemon stop-all`, which would take the live queue's daemon with
// it. Wait for it to go: t.TempDir's RemoveAll runs right after and a daemon
// still writing into `.beads` is what makes that fail with "directory not
// empty" — the leaked DIRECTORY half of ranger-base-42mv.
func stopLeakedDaemons(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		b, err := os.ReadFile(filepath.Join(dir, ".beads", "daemon.pid"))
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil || pid <= 0 {
			continue
		}
		t.Logf("reaping the daemon bd's pre-commit hook started here: pid %d in %s", pid, dir)
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			continue
		}
		for i := 0; i < 40; i++ {
			if err := syscall.Kill(pid, 0); err != nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// liveBeadsRepo is a real git repo with a real SQLite-backed bd database
// holding one row titled `row`, committed. The daemon bd's own pre-commit
// hook starts here is reaped in cleanup.
//
// SQLite is asked for by name because the default drifts: see WHICH bd, AND
// WHICH STORE CLASS in the header. It is the class every finding in this
// file was measured on and the class the operator's queue is.
func liveBeadsRepo(t *testing.T, bd func(string, ...string) (string, error), row string) string {
	t.Helper()
	return liveBeadsRepoOfClass(t, bd, row, "sqlite")
}

// liveBeadsRepoOfClass is that repo in a named store class: "sqlite" for the
// beads.db store, "no-db" for the JSONL-only one.
//
// The class is CHECKED after `bd init` rather than trusted to the flag, and
// that check is the whole lesson of ranger-base-9lrzx: a fixture that takes
// whatever class bd feels like building measures a different bd on a
// different box, and reports it as an arm failing somewhere far away. If bd
// ever accepts the flag and builds the other class, this fatals HERE, naming
// both classes, instead.
func liveBeadsRepoOfClass(t *testing.T, bd func(string, ...string) (string, error), row, class string) string {
	t.Helper()
	initArgs := []string{"init", "--backend", "sqlite"}
	if class == "no-db" {
		initArgs = []string{"init", "--no-db"}
	}
	repo := wtRepo(t)
	if out, err := bd(repo, initArgs...); err != nil {
		t.Skipf("bd %s did not take in a throwaway repo: %v %s", strings.Join(initArgs, " "), err, out)
	}
	t.Cleanup(func() { stopLeakedDaemons(t, repo) })
	built := "no-db"
	if _, err := os.Stat(filepath.Join(repo, ".beads", "beads.db")); err == nil {
		built = "sqlite"
	}
	if built != class {
		t.Fatalf("bd accepted %q and built a %s store, not the %s one: the class this pin was measured on is gone, and every arm below would be asking a different bd", strings.Join(initArgs, " "), built, class)
	}
	if out, err := bd(repo, "create", row, "-t", "task"); err != nil {
		t.Fatalf("bd create in %s: %v %s", repo, err, out)
	}
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "commit", "-q", "-m", "beads")
	return repo
}

// resolvedBeads is the `.beads` directory bd itself decides to use from dir —
// bd's own answer, never our model of it. Symlink-resolved because a t.TempDir
// is /var/folders/… to us and /private/var/folders/… to bd.
func resolvedBeads(t *testing.T, bd func(string, ...string) (string, error), dir string) string {
	t.Helper()
	out, err := bd(dir, "where", "--json")
	if err != nil {
		t.Fatalf("bd where in %s: %v %s", dir, err, out)
	}
	i, j := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if i < 0 || j < i {
		t.Fatalf("bd where --json printed no object:\n%s", out)
	}
	var w struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(out[i:j+1]), &w); err != nil || w.Path == "" {
		t.Fatalf("bd where --json did not answer with a path (%v):\n%s", err, out)
	}
	return resolveExisting(w.Path)
}

// beadsDirWhere is resolvedBeads through a runner carrying environment — the
// same question, asked of the bd the persona actually runs.
func beadsDirWhere(t *testing.T, bdEnv func([]string, string, ...string) (string, error), env []string, dir string) string {
	t.Helper()
	return resolvedBeads(t, func(d string, args ...string) (string, error) { return bdEnv(env, d, args...) }, dir)
}

// settleMainCheckout is the control, and it is load-bearing: bd's own
// pre-commit hook rewrites the main jsonl during the fixture's commit, so the
// MAIN checkout can be mid-import at this moment. If it is, that is a fact
// about the main checkout and says nothing about worktrees — settle it there,
// the way bd tells you to, and then ask the worktree the real question.
func settleMainCheckout(t *testing.T, bd func(string, ...string) (string, error), repo string) {
	t.Helper()
	if out, err := bd(repo, "list"); err != nil && strings.Contains(out, "out of sync") {
		if out, err := bd(repo, "sync", "--import-only"); err != nil {
			t.Fatalf("bd sync --import-only in the main checkout: %v %s", err, out)
		}
	}
	if out, err := bd(repo, "list"); err != nil {
		t.Fatalf("the main checkout itself is not readable, so nothing below means anything: %v %s", err, out)
	}
}

func TestLiveWorktreeSharesOneGraph(t *testing.T) {
	t.Parallel()
	bdEnv := liveBdEnv(t)
	bd := func(dir string, args ...string) (string, error) { return bdEnv(nil, dir, args...) }
	a := wtApp(t)
	repo := liveBeadsRepo(t, bd, "in the main checkout")

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopLeakedDaemons(t, tr.Path) })
	settleMainCheckout(t, bd, repo)

	// One graph: the row created in the main checkout is visible here, and
	// nothing about the read suggests a sync. bd's doing, not ours — see the
	// header, and TestLiveWorktreeBdResolvesTheWorktreeItself.
	out, err := bd(tr.Path, "list")
	if err != nil {
		t.Fatalf("bd list in the session worktree: %v %s", err, out)
	}
	if !strings.Contains(out, "in the main checkout") {
		t.Errorf("the worktree does not see the main database's rows:\n%s", out)
	}
	for _, bad := range []string{"out of sync", "--import-only", "allow-stale", "Fresh clone"} {
		if strings.Contains(out, bad) {
			t.Errorf("bd said %q in the worktree — the graph is not one:\n%s", bad, out)
		}
	}
	if _, err := os.Stat(filepath.Join(tr.Path, ".beads", "beads.db")); err == nil {
		t.Error("the worktree built a database of its own — the graph forked")
	}

	// Writes go to the one database too.
	if out, err := bd(tr.Path, "create", "from the worktree", "-t", "task"); err != nil {
		t.Fatalf("bd create in the worktree: %v %s", err, out)
	}
	if out, err := bd(repo, "list"); err != nil || !strings.Contains(out, "from the worktree") {
		t.Errorf("a bead filed in the worktree is not in the main database: %v\n%s", err, out)
	}

	// The same write once more, this time with the launch's own BEADS_DIR in
	// the environment (ADR 0055 D1, D5 item 2). Every claim above is a claim
	// about the DATABASE class, which is the class that already worked and
	// the class the operator's queue is; D1 sets the variable on every
	// session regardless of class, so what has to be shown here is that it
	// moves NOTHING — same rows, same database, no staleness warning, no
	// database of the worktree's own. bd reaches the same directory through
	// its BEADS_DIR branch instead of its worktree branch, and ADR 0055 has
	// this arm marked ASSUMED until this pin turns it.
	//
	// The value comes from beadsHome, not from a path spelled out here, so
	// what this measures is planLaunch's resolver (D5 item 3).
	env := beadsDirEnv(t, tr.Path, repo)
	if out, err := bdEnv(env, tr.Path, "create", "filed under BEADS_DIR", "-t", "task"); err != nil {
		t.Fatalf("bd create in the worktree under %v: %v %s", env, err, out)
	}
	// Read from the MAIN checkout, plain: the row has to be in the database
	// the operator reads, not merely in whatever the variable pointed at.
	if out, err := bd(repo, "list"); err != nil || !strings.Contains(out, "filed under BEADS_DIR") {
		t.Errorf("BEADS_DIR moved the write off the main database on the class that already worked: %v\n%s", err, out)
	}
	// Same database, by bd's own answer, and the same rows read back.
	if got, want := resolveExisting(beadsDirWhere(t, bdEnv, env, tr.Path)), resolveExisting(filepath.Join(repo, ".beads")); got != want {
		t.Errorf("under BEADS_DIR bd resolves %q, not the main checkout's %q", got, want)
	}
	out, err = bdEnv(env, tr.Path, "list")
	if err != nil {
		t.Fatalf("bd list in the worktree under %v: %v %s", env, err, out)
	}
	// Three DISJOINT titles: "from the worktree" is not a prefix of the
	// BEADS_DIR row's title, so this loop cannot report the plain row as
	// present because the new one is (contains-hides-a-repeat).
	for _, want := range []string{"in the main checkout", "from the worktree", "filed under BEADS_DIR"} {
		if !strings.Contains(out, want) {
			t.Errorf("the worktree does not see %q under BEADS_DIR — the variable moved the rows:\n%s", want, out)
		}
	}
	for _, bad := range []string{"out of sync", "--import-only", "allow-stale", "Fresh clone"} {
		if strings.Contains(out, bad) {
			t.Errorf("bd said %q in the worktree under BEADS_DIR — the variable bought a staleness warning the plain arm does not have:\n%s", bad, out)
		}
	}
	if _, err := os.Stat(filepath.Join(tr.Path, ".beads", "beads.db")); err == nil {
		t.Error("the worktree built a database of its own under BEADS_DIR — the variable forked the class it was meant to leave alone")
	}

	// AGREEMENT, and this is the arm that fails when seedBeadsRedirect does
	// not run (ranger-base-vczf: nothing above it does). posse does not tell
	// bd where the store is — bd already knows. posse tells the CAGE, through
	// beadsHome, and a grant that names a directory bd never opens denies bd
	// its own database while the resolution stays correct. So: what we seed,
	// what beadsHome answers, and what bd reads are one directory.
	reads := resolvedBeads(t, bd, tr.Path)
	if got := resolveExisting(beadsHome(tr.Path)); got != reads {
		t.Errorf("beadsHome answers %q; bd reads %q — the seatbelt writable set and the codex launch line are built from beadsHome, so the cage denies bd its own database", got, reads)
	}
	// beadsHome above is the claim; this one is the diagnosis, and it fatals
	// last so a missing redirect reports both.
	if got := resolveExisting(readRedirect(t, tr.Path)); got != reads {
		t.Errorf("the seeded redirect names %q; bd reads %q — the cage would grant a directory bd never opens", got, reads)
	}

	// The trap named in rangerhq-09o2: the worktree's checked-out jsonl has a
	// fresh mtime by construction. Push it far forward and the read is still
	// clean, because bd checks the jsonl beside the database it resolved to.
	jsonl := filepath.Join(tr.Path, ".beads", "issues.jsonl")
	if _, err := os.Stat(jsonl); err != nil {
		t.Skipf("this bd does not track issues.jsonl here, so there is no trap to spring: %v", err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(jsonl, future, future); err != nil {
		t.Fatal(err)
	}
	out, err = bd(tr.Path, "list")
	if err != nil || strings.Contains(out, "out of sync") {
		t.Errorf("the worktree's own stale jsonl fired bd's staleness check: %v\n%s", err, out)
	}
}

// What actually keeps the graph unforked on bd 0.49.1, written down so the
// day it changes is a red test rather than a silent one (ranger-base-vczf).
//
// bd resolves a linked worktree to the MAIN checkout's `.beads` by itself,
// and while the main checkout has one it does not read the worktree's own
// `redirect` at all: a redirect pointing at a DIFFERENT live database is
// ignored, and bd goes on reading the main graph. That is why no "did the
// graph fork" arm can fail for want of posse's redirect, and it is why the
// arm in TestLiveWorktreeSharesOneGraph pins agreement instead.
//
// The second half is the positive witness the first half needs. An assertion
// that a planted file changed nothing is satisfied by a file bd could not
// read, a path that does not exist, or a fixture that was never built — so
// the SAME redirect, over a main checkout with no `.beads` of its own, must
// be honoured. It is: bd falls back to the local one and reads the other
// database. The plant is real; the precedence is the finding.
//
// If this ever goes red at the first arm, posse's redirect has become
// load-bearing for the graph too, and worktree.go's note should say so.
func TestLiveWorktreeBdResolvesTheWorktreeItself(t *testing.T) {
	t.Parallel()
	bd := liveBd(t)
	a := wtApp(t)
	repo := liveBeadsRepo(t, bd, "in the main checkout")
	other := liveBeadsRepo(t, bd, "in the other database")
	settleMainCheckout(t, bd, repo)
	settleMainCheckout(t, bd, other)

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopLeakedDaemons(t, tr.Path) })
	misdirect(t, tr.Path, filepath.Join(other, ".beads"))

	if got, want := resolvedBeads(t, bd, tr.Path), resolveExisting(filepath.Join(repo, ".beads")); got != want {
		t.Errorf("bd read the worktree's own redirect: resolved %q, want the main checkout's %q", got, want)
	}
	out, err := bd(tr.Path, "list")
	if err != nil {
		t.Fatalf("bd list in the misdirected worktree: %v %s", err, out)
	}
	if !strings.Contains(out, "in the main checkout") || strings.Contains(out, "in the other database") {
		t.Errorf("a misdirected redirect moved the rows bd returns, so the graph CAN fork on this bd:\n%s", out)
	}

	// The witness: same plant, main checkout with no `.beads`, and now bd
	// reads it. (posse writes no redirect in this shape — seedBeadsRedirect
	// returns early with nothing to keep unforked — so the plant is by hand,
	// as it is above.) It witnesses on the SQLite class only: a redirect into
	// a no-db store resolves here and still answers from the local jsonl, so
	// a fixture that quietly built one left this arm asserting nothing
	// (ranger-base-9lrzx, and the header says how that happened).
	bare, err := a.EnsureSessionTree(wtRepo(t), "s-2", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopLeakedDaemons(t, bare.Path) })
	misdirect(t, bare.Path, filepath.Join(other, ".beads"))

	if got, want := resolvedBeads(t, bd, bare.Path), resolveExisting(filepath.Join(other, ".beads")); got != want {
		t.Fatalf("the planted redirect is not one bd honours anywhere, so the arm above measured nothing: resolved %q, want %q", got, want)
	}
	if out, err := bd(bare.Path, "list"); err != nil || !strings.Contains(out, "in the other database") {
		t.Fatalf("the planted redirect resolves but returns no rows, so it is not a live database — check the STORE CLASS before anything else, a no-db target resolves and then answers from the local jsonl (ranger-base-9lrzx): %v\n%s", err, out)
	}
}

// misdirect points a tree's `.beads/redirect` somewhere posse never would,
// overwriting whatever seedBeadsRedirect left.
func misdirect(t *testing.T, tree, target string) {
	t.Helper()
	dir := filepath.Join(tree, ".beads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redirect"), []byte(target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The store class the findings above do NOT hold for, and the one line of
// environment that closes it (measured 2026-09-04, bd 0.50.3 —
// ranger-base-9lrzx for the fork, ranger-base-e3ima for the four cells here).
//
// With bd in NO-DB mode, bd resolves the session worktree to the main
// checkout's `.beads` — `bd where` says so and names posse's redirect as what
// took it there — and then reads and writes the worktree's OWN
// `.beads/issues.jsonl` anyway. A bead filed from the worktree is in the
// worktree's jsonl and never reaches the main one: the graph forks, while the
// resolution the cage's grant is built from stays perfectly correct. (The
// read half hides it: the worktree's checked-out jsonl carries the main
// checkout's rows by construction, so "the worktree sees the main rows" is
// true there for a reason that has nothing to do with one graph. Only a write
// tells the two apart, which is why every cell here writes.)
//
// NO-DB IS A MODE, NOT A CLASS, which is why this pin has two doors and not
// one. `no-db: true` in the store's config.yaml is the door the bead was
// filed about; `--no-db` on the command line is the door POSSE OPENS ITSELF,
// at the CONTAINER tier and only there (CageBdFlags, cageinner.go; a
// `cage: seatbelt` seat's `bd` is the rendered gate shim and carries no
// `--no-db` — ranger-base-zt61m), over whatever class the store happens to
// be — so at that tier the fork is the shipped configuration and not a
// store-class accident. Both doors are driven below over their own fixture:
// the first over a `bd init --no-db` store, the second over the SQLite store
// every other pin in this file uses.
//
// And two arms per door, which is the point of the pin rather than a
// courtesy. The first arm is the fork, and it is what makes the second one
// mean something: without it, "the row reached the main store" is satisfied
// by a bd that was never in no-db mode, by a worktree that was never linked,
// and by a fixture that never wrote. The second arm is ADR 0055 D1 —
// `BEADS_DIR=beadsHome(<the session tree>)`, the value planLaunch puts in
// every session's environment, and with it the same create lands in the MAIN
// store and the worktree's own `bd list` reads it back.
//
// MUTATION-CHECKED (rig-must-be-shown-able-to-fail): five mutants, each red
// at the assertion named for it — drop BEADS_DIR from cell B; point it at the
// worktree; hand cell A the variable; drop the door's `--no-db`; and, for
// TestLiveWorktreeSharesOneGraph's own BEADS_DIR arm, point it at a third
// store. The table is re-run and quoted on ranger-base-e3ima rather than
// carried here, where nothing re-measures it.
//
// So what posse fixed is the LAUNCH, not this: bd's resolution is unchanged,
// and a `bd` run with the variable shed (`env -u BEADS_DIR`, `env -i`) forks
// exactly as the first arm of each door does — that first arm IS a shed arm,
// and liveBdEnv drops `BEADS_DIR` from the inherited environment to make it
// one whatever the session running this test carries (ranger-base-zt61m).
// If the first arm ever goes red, bd has started honouring the redirect in
// no-db mode too — that is bd fixing it, and this header and worktree.go's
// note should then say the mode no longer matters.
func TestLiveWorktreeNoDbStoreForksTheGraph(t *testing.T) {
	t.Parallel()
	for _, door := range []struct {
		name  string
		class string   // what liveBeadsRepoOfClass must BUILD, checked there
		flags []string // what puts the session's own calls in no-db mode
	}{
		// Door 1 (ADR 0055 Context): `no-db: true` in the resolved store.
		{"no-db in the store config", "no-db", nil},
		// Door 2, and posse opens it: the cage wrapper's flag, over a
		// database-class store. Measured at the bd level only — the
		// container tier cannot run on this box, so this stands in for it.
		{"flag over a database store", "sqlite", []string{"--no-db"}},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			bdEnv := liveBdEnv(t)
			// bd, plain: the fixture's own calls and the RESOLUTION arm.
			// The door's flags are what the SESSION runs with, and the
			// resolution is the same either way — asking `bd where` through
			// the flag would only measure the flag's own answer.
			bd := func(dir string, args ...string) (string, error) { return bdEnv(nil, dir, args...) }
			// door is what a caged session's bd looks like: the flags in
			// front of the verb, nothing in the environment yet.
			through := func(env []string, dir string, args ...string) (string, error) {
				return bdEnv(env, dir, append(append([]string{}, door.flags...), args...)...)
			}

			a := wtApp(t)
			repo := liveBeadsRepoOfClass(t, bd, "in the main checkout", door.class)
			settleMainCheckout(t, bd, repo)

			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { stopLeakedDaemons(t, tr.Path) })

			// The resolution is right, and posse's redirect is what did it.
			if got, want := resolvedBeads(t, bd, tr.Path), resolveExisting(filepath.Join(repo, ".beads")); got != want {
				t.Fatalf("bd does not resolve the worktree to the main checkout in this shape either: resolved %q, want %q", got, want)
			}
			if got := resolveExisting(readRedirect(t, tr.Path)); got != resolveExisting(filepath.Join(repo, ".beads")) {
				t.Fatalf("posse seeded %q, not the main checkout's `.beads` — this pin is measuring the wrong plant", got)
			}

			// The jsonl is where the claim lives, on BOTH doors, and it
			// is the honest reading rather than a convenience: a no-db bd
			// writes the store's `issues.jsonl` and nothing else. On door 2
			// the main store also has a `beads.db`, and the routed row is
			// NOT in it until something imports — so "lands in the main
			// store" means the directory posse resolved, which is the whole
			// claim ADR 0055 D1 makes. Asserting on the database instead
			// would be asserting on bd's import schedule.
			mainJSONL := filepath.Join(repo, ".beads", "issues.jsonl")
			treeJSONL := filepath.Join(tr.Path, ".beads", "issues.jsonl")

			// ── cell A: no BEADS_DIR — the SHED arm, and liveBdEnv is what
			// sheds it (a bare os.Environ() would hand this arm the very
			// variable it is named for). The write forks, and this is the
			// arm that has to be able to fail for cell B to say anything.
			if out, err := through(nil, tr.Path, "create", "forked from the worktree", "-t", "task"); err != nil {
				t.Fatalf("bd create in the worktree: %v %s", err, out)
			}
			if got := readFileString(t, mainJSONL); strings.Contains(got, "forked from the worktree") {
				t.Errorf("bd now honours the redirect in no-db mode: the graph no longer forks, so this file's header and worktree.go's note are out of date:\n%s", got)
			}
			if got := readFileString(t, treeJSONL); !strings.Contains(got, "forked from the worktree") {
				t.Errorf("the row is in neither jsonl, so the fork is somewhere this pin does not describe:\n%s", got)
			}

			// ── cell B: the same create, with the launch's own BEADS_DIR.
			// beadsDirEnv fatals unless the value is the main store, so a
			// pass here cannot come from an env that named the worktree.
			env := beadsDirEnv(t, tr.Path, repo)
			if out, err := through(env, tr.Path, "create", "routed from the worktree", "-t", "task"); err != nil {
				t.Fatalf("bd create in the worktree under %v: %v %s", env, err, out)
			}
			if got := readFileString(t, mainJSONL); !strings.Contains(got, "routed from the worktree") {
				t.Errorf("%s did not route the write to the main store — ADR 0055 D1 does not close the fork on this bd:\n%s", strings.Join(env, " "), got)
			}
			if got := readFileString(t, treeJSONL); strings.Contains(got, "routed from the worktree") {
				t.Errorf("the write landed in the worktree's own jsonl as well — BEADS_DIR moved the read but not the write-back:\n%s", got)
			}
			// Read it back the way the persona would: same door, same env.
			out, err := through(env, tr.Path, "list")
			if err != nil {
				t.Fatalf("bd list in the worktree under %v: %v %s", env, err, out)
			}
			if !strings.Contains(out, "routed from the worktree") || !strings.Contains(out, "in the main checkout") {
				t.Errorf("the worktree does not read back the one graph under BEADS_DIR — the main checkout's row, or its own, is missing:\n%s", out)
			}
			// And the fork stays forked: BEADS_DIR is not a repair verb.
			// This is what says cell A's row was real and not a fixture
			// artefact that cell B quietly swept up.
			if strings.Contains(out, "forked from the worktree") {
				t.Errorf("the row cell A stranded in the worktree jsonl came back under BEADS_DIR — then cell A never measured a fork:\n%s", out)
			}
		})
	}
}

// shedBeadsDir drops `BEADS_DIR` from an inherited environment. It is the one
// variable every pin in this file is about, so the arms that do not name it
// have to be arms where it is ABSENT rather than arms where it was not
// mentioned — see liveBdEnv above for what that cost when it was missing.
func shedBeadsDir(env []string) []string {
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		if strings.HasPrefix(kv, "BEADS_DIR=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// readFileString is the jsonl half of the cells above: a missing file is a
// failure of the fixture, not a row that is absent, and the two must not read
// alike.
func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
