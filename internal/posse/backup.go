package posse

// `posse backup` — the store of record's backup is a harness verb, not a
// script plus a plist nobody armed (ADR 0036, docs/adr/0036-posse-backup.md;
// build bead ranger-base-a0ln0).
//
// **What the operator's 2026-09-01 sub-ruling on ranger-base-ay3dr changed,
// and why the file is shorter than the record.** ADR 0036 designed six
// intents around an OFF-BOX destination: a `sweep` verb, an age identity so
// copies that leave custody are ciphertext, and a freshness model split into
// on-box and off-box halves. The sub-ruling cut the destination: this verb
// writes an on-box archive and REFUSES any remote target, and the refusal is
// the design rather than a flag. Three of the record's decisions follow the
// destination out:
//
//	§1 sweep / §6 off-box recency   there is no destination to sweep to, so
//	                                there is no second clock and no stamp
//	                                store for one.
//	§5 age identity + go.mod dep    the identity protected copies AT the
//	                                destination (the record says so in as
//	                                many words). With every copy on the box
//	                                that already holds the plaintext store of
//	                                record, an identity beside the ciphertext
//	                                guards nothing and buys posse its first
//	                                dependency outside golang.org/x. Archives
//	                                are plaintext tar.gz, 0600 in a 0700 dir
//	                                — exactly the exposure the store already
//	                                has. If an off-box destination is ever
//	                                ruled back in, §5 comes back WITH it;
//	                                that is the order the argument runs in.
//	§4 the ticker                   BUILT, later and separately, by bead
//	                                ranger-base-zv3y6 — see backuploop.go.
//	                                a0ln0 defined no `backup_interval:` key
//	                                on purpose: a key that reads like a
//	                                schedule and schedules nothing is the
//	                                plist that was never installed, wearing a
//	                                config key, so the key and the loop
//	                                landed together. The staleness threshold
//	                                stays its own key (`backup_max_age:`) and
//	                                says only what it means; what it now
//	                                takes from the schedule is its DEFAULT,
//	                                §6's 2x the interval.
//
// What did NOT change, and is enforced below: no remote TARGET (ADR 0036 §3,
// the operator's 2c ruling), the disk floor, single-flight,
// publish-by-rename, and prune-never-before-a-verified-newer. The SOURCE
// half of the no-remote rule — a queue repo that grew a remote is refused
// before anything is read from it — is per-instance since ADR 0049
// (2026-09-02, bead ranger-base-ymgbo): config `queue_remote:` names the
// one remote an instance sanctions, and checkQueueRemote below refuses every
// other. Unset, the 2c posture holds unchanged.
//
// **One store for freshness.** §6 gives on-box freshness exactly one owner:
// the archive files themselves. This build makes that literal — an archive
// is renamed into place only AFTER it has been read back and verified, so
// the presence of a file IS a green verdict and there is no second stamp
// store that can disagree with the directory. `posse backup status` reads
// the directory and nothing else.

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// BackupDirName is the default archive directory under the home's
	// state/. state/ is machine-local and session-writable by design
	// (promote.go), which is the correct class for archives: they are
	// derived from the two stores, never promoted, and never committed.
	BackupDirName = "backup"

	// DefaultBackupKeep is the on-box retention (ADR 0036 §1).
	DefaultBackupKeep = 3

	// DefaultBackupMinFreeMB is the disk floor (ADR 0036 §1), derived there
	// from the predecessor's MEASURED ~180MB transient peak.
	DefaultBackupMinFreeMB = 384

	// DefaultBackupMaxAge is when the newest on-box archive becomes a
	// governance condition on an instance with NO schedule. ADR 0036 §6
	// sets the threshold at 2x the backup interval, which defaultBackupMaxAge
	// now applies wherever `backup_interval:` is armed (backuploop.go); with
	// no interval to double, the fallback is 2x the cadence the predecessor
	// actually ran at — a nightly 03:15 archive (hl2p) — which is 48h.
	DefaultBackupMaxAge = 48 * time.Hour

	backupManifestName    = "MANIFEST.json"
	backupManifestVersion = 1
	backupPrefix          = "posse-backup-"
	backupSuffix          = ".tar.gz"
	backupSidecarSuffix   = ".sha256"
	backupStamp           = "20060102T150405Z"
	backupPartSuffix      = ".part"
)

// ErrBackupTool is the class ADR 0036 §2 gives its own exit: an external
// tool this verb needs is not on PATH. It is not a failed backup, it is a
// backup that never started, and the caller reports it as such.
var ErrBackupTool = errors.New("required tool missing")

// volume is what the platform's statfs reader answers (backupvolume_*.go).
// Local is the whole refusal: false, or a reading that could not be taken
// at all, and no archive is written.
type volume struct {
	FSType    string
	Local     bool
	FreeBytes uint64
}

// ─── config ──────────────────────────────────────────────────────────────────

// BackupDir is where archives live: config `backup_dir:`, or state/backup
// under the home. The value is expanded and cleaned but NOT certified here
// — CheckBackupTarget is the gate, and it runs at every use.
func (a *App) BackupDir() string {
	if v := strings.TrimSpace(a.CfgGet("backup_dir", "")); v != "" {
		return filepath.Clean(ExpandTilde(v))
	}
	return filepath.Join(a.StateDir, BackupDirName)
}

// BackupMaxAge is how old the newest on-box archive may be before the
// governance surface says so (config `backup_max_age:`).
func (a *App) BackupMaxAge(errw io.Writer) time.Duration {
	return a.attnAge("backup_max_age", a.defaultBackupMaxAge(), errw)
}

// defaultBackupMaxAge is what `backup_max_age:` means when the operator has
// not written one, and it is ADR 0036 §6's rule rather than a number: 2x the
// scheduled interval. An instance that arms the clock has said how often it
// wants an archive, and having missed two of its own intervals is what
// "stale" means there — no second number to keep in step with the first.
//
// DefaultBackupMaxAge (48h) survives as the fallback for the instance with
// NO schedule, where §6 has no interval to double and 48h is 2x the cadence
// the predecessor actually ran at (nightly 03:15, hl2p). A backup_interval:
// that is present but unparseable falls back there too, and says nothing:
// the watch loop is what complains about a broken cadence, once, and a
// freshness reading that repeated it would print it on every `posse status`
// and every pulse tick.
func (a *App) defaultBackupMaxAge() time.Duration {
	cfg, err := LoadBackupConfig(a)
	if err != nil || !cfg.Armed {
		return DefaultBackupMaxAge
	}
	return BackupMaxAgeFactor * cfg.Interval
}

// BackupKeep is the on-box retention count. Zero is refused rather than
// honoured: "keep none" is a backup verb that deletes the only copy it just
// made, and a typo must not be able to ask for that.
func (a *App) BackupKeep(errw io.Writer) int {
	raw := strings.TrimSpace(a.CfgGet("backup_keep", ""))
	if raw == "" {
		return DefaultBackupKeep
	}
	if n, err := strconv.Atoi(raw); err == nil && n >= 1 {
		return n
	}
	fmt.Fprintf(errw, "backup: config backup_keep: %q is not a count of at least 1 — using %d\n", raw, DefaultBackupKeep)
	return DefaultBackupKeep
}

// BackupMinFree is the disk floor in bytes (config `backup_min_free_mb:`).
func (a *App) BackupMinFree(errw io.Writer) uint64 {
	raw := strings.TrimSpace(a.CfgGet("backup_min_free_mb", ""))
	if raw == "" {
		return DefaultBackupMinFreeMB << 20
	}
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return uint64(n) << 20
	}
	fmt.Fprintf(errw, "backup: config backup_min_free_mb: %q is not a size in MB — using %d\n", raw, DefaultBackupMinFreeMB)
	return DefaultBackupMinFreeMB << 20
}

// backupKeys are the config keys that ARM the freshness reading. An instance
// that has never written one of them, and has no archive on disk, reports
// nothing at all: installing a posse that knows how to back up must not
// start telling an operator their backups are late (the same inertness rule
// `queue_repo:` keeps — ADR 0015 §4). `queue_remote:` is NOT one of them
// (ADR 0049 D3): it sanctions a remote, it does not ask for backups, so an
// instance that declares its remote and no backup_* key stays unarmed.
var backupKeys = []string{"backup_dir", "backup_interval", "backup_max_age", "backup_keep", "backup_min_free_mb"}

// BackupConfigured reports whether the operator has written any backup key.
// It is half of "armed" — the other half is an archive already on disk from
// a hand-typed run, which BackupFreshness folds in from the listing it takes
// anyway rather than by reading the directory twice.
func (a *App) BackupConfigured() bool {
	for _, k := range backupKeys {
		if yamlHasKey(a.ConfigPath, k) {
			return true
		}
	}
	return false
}

// ─── the sanctioned remote (ADR 0049) ────────────────────────────────────────

// QueueRemote is config `queue_remote:` (ADR 0049 D1): the URL of the one
// git remote this instance's queue repo may carry, spelled exactly as `git
// remote get-url <name>` prints it. Unset, empty, `~` or `null` is "" — the
// 2c posture, under which any remote on the queue refuses. It is an
// instance fact and nothing more: not a backup key (see backupKeys), not a
// flag on the verb (BackupOpts is unchanged), and not a remote NAME —
// `origin` is what every clone mints, so a name would sanction a shape
// where the ruling sanctioned a place.
func (a *App) QueueRemote() string { return strings.TrimSpace(a.CfgGet("queue_remote", "")) }

// checkQueueRemote is the SOURCE half of "no remote" (ADR 0036 §3 as ADR
// 0049 D1/D2 amend it). A queue with no remote passes under either posture:
// the key sanctions, it does not require. With declared == "" any remote
// refuses, with the way out named. With the key set the queue may hold
// exactly one remote, of any name, whose fetch URL and push URL both equal
// declared — `git remote get-url --push` answers the push URL and falls
// back to the fetch URL when none is set (MEASURED 2026-09-02, git 2.50.1),
// so a repo with only a fetch URL passes on that URL alone, and one whose
// pushurl points elsewhere is caught. Every refusal prints what was
// declared beside what was found: the fix is a paste from `git remote
// get-url`, never a spelling table (0049's rejected alternatives).
//
// The rule's record is the operator's 2c ruling (ranger-base-xhsb) and ADR
// 0036's Context — not ADR 0015 §4, which the refusal used to cite and
// which states no such rule (0049 Context, MEASURED).
func checkQueueRemote(queue, declared string) error {
	remotes, err := git(queue, "remote")
	if err != nil {
		// The caller has already proved queue is a repository; a `git
		// remote` that fails here is a git that cannot list, and the
		// bundle step will name it. Not a remote, not a refusal.
		return nil
	}
	names := strings.Fields(remotes)
	if len(names) == 0 {
		return nil
	}
	if declared == "" {
		return Die("%s has git remote(s) %s — the queue repo never grows a remote (the operator's 2c ruling, ranger-base-xhsb; ADR 0036 Context, per-instance since ADR 0049); remove it before backing up, or declare the one remote this instance sanctions as config queue_remote: <url>",
			AbbrevHome(queue), strings.Join(names, ", "))
	}
	if len(names) > 1 {
		return Die("%s has %d git remotes (%s) and config queue_remote: sanctions exactly one — declared %s; remove the others before backing up (ADR 0049 D2)",
			AbbrevHome(queue), len(names), strings.Join(names, ", "), declared)
	}
	name := names[0]
	// --all on BOTH sides, because a remote may carry more than one url.
	// git-remote(1) of get-url: "By default, only the first URL is listed",
	// and git-config(1) of remote.<name>.url: "the first is used for
	// fetching, and all are used for pushing (assuming no
	// remote.<name>.pushurl is defined)". So a remote whose FIRST url is the
	// sanctioned one and whose second is anywhere else printed declared on
	// both single reads, passed, and every operator push landed at both — an
	// off-box copy nobody sanctioned, which is the thing D2 exists to refuse
	// (ranger-base-m6szh, escaped from ranger-base-ymgbo; MEASURED, git
	// 2.50.1). A second push url was already caught; a second fetch url was
	// not. Each side must now be exactly the declared URL and nothing else,
	// and multi-line output fails that comparison on its own.
	fetch, err := git(queue, "remote", "get-url", "--all", name)
	if err != nil {
		return err
	}
	push, err := git(queue, "remote", "get-url", "--all", "--push", name)
	if err != nil {
		return err
	}
	if fetch != declared || push != declared {
		return Die("%s remote %s is not the sanctioned one — declared %s (config queue_remote:), found fetch %s and push %s; the value is the URL exactly as `git remote get-url` prints it, and a remote carrying a second URL is refused because every push lands at both (ADR 0049 D2)",
			AbbrevHome(queue), name, declared, urlList(fetch), urlList(push))
	}
	return nil
}

// urlList renders what `git remote get-url --all` printed for one refusal
// that a human reads: every URL found, comma-separated, on the refusal's own
// line. The count is the finding — one line saying "found fetch A" while a
// second URL sat below it is how the second URL stayed invisible.
func urlList(out string) string {
	return strings.Join(strings.Split(out, "\n"), ", ")
}

// BackupRemoteLine is the posture, one line, for `posse backup status`
// (ADR 0049 D6). It reads config and nothing else — no git — because the
// status verb reports what the instance DECLARED, and the refusal at run
// time reports what it found.
func (a *App) BackupRemoteLine() string {
	if u := a.QueueRemote(); u != "" {
		return fmt.Sprintf("  remote · %s (config queue_remote:) — the operator pushes; posse never does", u)
	}
	return "  remote · none declared (config queue_remote: unset) — any remote refuses"
}

// ─── the refusal (ADR 0036 §3, as the sub-ruling narrowed it) ────────────────

// CheckBackupTarget is the whole of "refuses any remote target". It is not a
// flag and there is no override: every path that writes an archive calls it,
// and it runs again after the directory is created, because the second call
// is the one that reads the FINAL directory rather than its deepest existing
// ancestor.
//
// Four shapes are refused by reading alone, before any syscall — a URL, an
// scp-style host:path, a UNC //host/share, and a bare hostname with a colon
// — because those never name a local directory and a statfs on them would
// answer about the wrong thing (a relative directory literally called
// "host:" in cwd). Everything that survives that is resolved through its
// symlinks and handed to the kernel: the volume must be LOCAL, and a volume
// whose locality cannot be read is refused too. A refusal that fails open is
// not a refusal.
func CheckBackupTarget(dir string) error { return checkBackupTarget(dir, volumeOf) }

// checkBackupTarget is CheckBackupTarget with the kernel reading as a
// parameter. The seam exists for one reason and it is the reason ADR 0036 §7
// gives its own drill arm: a refusal nobody has watched refuse is a refusal
// nobody has measured, and this box has exactly one non-local volume to
// point at (autofs, MEASURED 2026-09-01) — one, on one operator's laptop, is
// not a test. The fake supplies the volumes this box does not mount: an nfs
// share, and a reading that fails.
func checkBackupTarget(dir string, read func(string) (volume, error)) error {
	raw := strings.TrimSpace(dir)
	if raw == "" {
		return Die("backup target is empty — name an on-box directory (config backup_dir:, or --to)")
	}
	refuse := func(what string) error {
		return Die("%s is not an on-box path: %s — posse backup writes on-box archives only, and refuses every remote target (ADR 0036 §3; the operator's 2c ruling)", raw, what)
	}
	if strings.Contains(raw, "://") {
		return refuse("it is a URL")
	}
	if strings.HasPrefix(raw, "//") {
		return refuse("it is a UNC //host/share path")
	}
	if i := strings.IndexByte(raw, ':'); i >= 0 && !strings.Contains(raw[:i], "/") {
		return refuse("it is an scp-style host:path")
	}
	abs, err := filepath.Abs(ExpandTilde(raw))
	if err != nil {
		return Die("backup target %s: %v", raw, err)
	}
	probe, err := existingAncestor(abs)
	if err != nil {
		return Die("backup target %s: %v", AbbrevHome(abs), err)
	}
	real, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return Die("backup target %s: %v", AbbrevHome(probe), err)
	}
	v, err := read(real)
	if err != nil {
		// Unreadable is refused, not blessed: see backupvolume_other.go.
		return Die("backup target %s: %v — posse backup refuses a target it cannot certify is on this box", AbbrevHome(real), err)
	}
	if !v.Local {
		return refuse(fmt.Sprintf("%s is on a %s volume", AbbrevHome(real), v.FSType))
	}
	return nil
}

// existingAncestor is the deepest component of p that exists. A target
// directory posse is about to create has none of its own, and the volume its
// parent is on is the volume it will be created on.
func existingAncestor(p string) (string, error) {
	for {
		if _, err := os.Lstat(p); err == nil {
			return p, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", Die("no existing ancestor")
		}
		p = parent
	}
}

// ─── the archive ─────────────────────────────────────────────────────────────

// BackupMember is one file inside the archive, as the manifest records it.
type BackupMember struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// BackupManifest is the archive's own account of itself, and the FIRST entry
// in the tar — deliberately, so a verifier can stream the whole archive once,
// hashing each member as it arrives, without extracting anything.
//
// Excluded is not decoration: it records by name what was deliberately left
// out, so a restore reads an absent envs/ as a policy rather than as a loss.
type BackupManifest struct {
	Version    int            `json:"version"`
	CreatedAt  string         `json:"created_at"`
	Posse      string         `json:"posse"`
	Home       string         `json:"home"`
	QueueRepo  string         `json:"queue_repo"`
	QueueStore string         `json:"queue_store"`
	QueueHead  string         `json:"queue_head,omitempty"`
	Excluded   []string       `json:"excluded"`
	Members    []BackupMember `json:"members"`
}

// BackupOpts is one run's inputs. Dir empty = the configured directory; Now
// nil = time.Now.
type BackupOpts struct {
	Dir string
	Out io.Writer
	Now func() time.Time
}

// BackupResult is what one run wrote.
type BackupResult struct {
	Archive  string
	Sidecar  string
	Bytes    int64
	Manifest BackupManifest
	Pruned   []string
}

// BackupHomePaths is what the archive takes from the constitution home: the
// promoted set — which since ADR 0039 D2 (2026-09-01, ranger-base-ight8)
// includes `runtimes/`, so the union below is now what keeps this list from
// naming it twice rather than what adds it — plus the promote manifest
// itself, which is the anchor that makes the copied constitution attestable
// at all.
//
// What it never takes is spelled in BackupExcluded, and the two lists are
// the same fact from both sides.
func BackupHomePaths() []string {
	out := append([]string{}, PromotedPaths...)
	for _, extra := range []string{"runtimes", PromoteManifestFile} {
		if !slices.Contains(out, extra) {
			out = append(out, extra)
		}
	}
	sort.Strings(out)
	return out
}

// BackupExcluded is what lives at the home and never enters an archive.
// `envs/` and `secrets/` are the ruling ("envs NEVER — secrets stay out"):
// both hold plaintext credentials, and a copy of them is a second place a
// token can leak from, on a file the operator will reasonably hand around.
// `state/` is machine-local runtime data and holds this directory itself.
// `personas/` is persona memory, whose durable copy is the commit each
// session lands (ADR 0015 §5 keeps it out of the promoted set for the same
// reason).
var BackupExcluded = []string{ConstitutionEnvsDir, "secrets", "state", "personas"}

// RunBackup builds one archive and publishes it. Every refusal it can make
// happens before it writes anything, and the archive it writes is verified
// before it is named.
func (a *App) RunBackup(o BackupOpts) (BackupResult, error) {
	var res BackupResult
	out := o.Out
	if out == nil {
		out = io.Discard
	}
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}

	// The target refusal runs FIRST, before anything about the source is
	// read. An operator who typed a remote destination must hear about the
	// destination, on every box, whatever state the queue is in — a refusal
	// that only fires once the source checks out is a refusal with a
	// precondition (ADR 0036 §3, the 2026-09-01 sub-ruling).
	dir := o.Dir
	if strings.TrimSpace(dir) == "" {
		dir = a.BackupDir()
	}
	if err := CheckBackupTarget(dir); err != nil {
		return res, err
	}

	queue := a.QueueRepo()
	if queue == "" {
		return res, Die("config queue_repo: is unset — the store has not moved yet (ADR 0015 §4), so there is nothing to back up")
	}
	store := beadsHome(queue)
	if st, err := os.Stat(store); err != nil || !st.IsDir() {
		return res, Die("%s has no beads store at %s", AbbrevHome(queue), AbbrevHome(store))
	}
	if _, err := git(queue, "rev-parse", "--git-dir"); err != nil {
		return res, Die("%s is not a git repository — queue_repo: must name a checkout (ADR 0015 §4)", AbbrevHome(queue))
	}
	// The 2c ruling, enforced rather than obeyed (ADR 0036 §3), as ADR 0049
	// makes it per-instance: a queue repo that has grown a remote is a store
	// of record with an off-box copy — refused unless it is exactly the one
	// this instance declared as `queue_remote:`.
	if err := checkQueueRemote(queue, a.QueueRemote()); err != nil {
		return res, err
	}
	for _, tool := range []string{"git", "sqlite3"} {
		if _, err := exec.LookPath(tool); err != nil {
			return res, fmt.Errorf("%w: %s is not on PATH — posse backup needs it to stage the store (ADR 0036 §2)", ErrBackupTool, tool)
		}
	}

	dir, err := filepath.Abs(ExpandTilde(strings.TrimSpace(dir)))
	if err != nil {
		return res, err
	}
	// 0700: the archive holds the whole work graph in the clear, which is
	// what the store of record already is, and no wider.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return res, Die("backup dir %s: %v", AbbrevHome(dir), err)
	}
	// The second call, on the directory that now exists: the first one
	// answered about its deepest existing ancestor, and a mount at the leaf
	// is exactly the case that hides behind that.
	if err := CheckBackupTarget(dir); err != nil {
		return res, err
	}

	v, err := volumeOf(dir)
	if err != nil {
		return res, err
	}
	if floor := a.BackupMinFree(out); v.FreeBytes < floor {
		return res, Die("%s has %s free and the floor is %s (config backup_min_free_mb:) — refusing rather than filling the disk (ADR 0036 §3)",
			AbbrevHome(dir), humanBytes(int64(v.FreeBytes)), humanBytes(int64(floor)))
	}

	// Single-flight (ADR 0036 §3): one writer in this directory. Held is not
	// an error worth a stack — it is the other run doing the job.
	lock, err := lockBackupDir(dir)
	if err != nil {
		return res, err
	}
	defer lock.Close()

	at := now().UTC()
	stage := filepath.Join(dir, ".staging-"+at.Format(backupStamp)+"-"+strconv.Itoa(os.Getpid()))
	if err := os.MkdirAll(stage, 0o700); err != nil {
		return res, Die("staging %s: %v", AbbrevHome(stage), err)
	}
	defer os.RemoveAll(stage)

	man, err := a.stageBackup(out, stage, queue, store, at)
	if err != nil {
		return res, err
	}

	name := backupName(dir, at)
	part := filepath.Join(dir, name+backupPartSuffix)
	size, sum, err := writeBackupArchive(part, stage, man)
	if err != nil {
		os.Remove(part)
		return res, err
	}
	// Read it back before it has a name. A backup nobody has opened is the
	// predecessor's measured failure mode (ADR 0036 §1), and the cheapest
	// possible opening is this one: stream the archive once, hash every
	// member, compare with the manifest that traveled inside it.
	if _, err := VerifyBackup(part); err != nil {
		os.Remove(part)
		return res, Die("the archive just written does not verify, so it was not published: %v", err)
	}
	archive := filepath.Join(dir, name)
	sidecar := archive + backupSidecarSuffix
	if err := os.WriteFile(sidecar+backupPartSuffix, []byte(sum+"  "+name+"\n"), 0o600); err != nil {
		os.Remove(part)
		return res, err
	}
	// Publish by rename, sidecar first: a reader that sees the archive must
	// never fail to find its hash.
	if err := os.Rename(sidecar+backupPartSuffix, sidecar); err != nil {
		os.Remove(part)
		os.Remove(sidecar + backupPartSuffix)
		return res, err
	}
	if err := os.Rename(part, archive); err != nil {
		os.Remove(part)
		os.Remove(sidecar)
		return res, err
	}

	res = BackupResult{Archive: archive, Sidecar: sidecar, Bytes: size, Manifest: man}
	fmt.Fprintf(out, "backup · %s (%s, %d files) · verified\n", AbbrevHome(archive), humanBytes(size), len(man.Members))

	// Prune last, and only now: the archive just written is verified and is
	// the newest, so every candidate below is a copy with a newer green one
	// above it (ADR 0036 §3). A run that built garbage never reaches here.
	res.Pruned = pruneBackups(out, dir, a.BackupKeep(out))
	return res, nil
}

// stageBackup lays the two stores out under stage and returns the manifest
// describing what it laid down.
func (a *App) stageBackup(out io.Writer, stage, queue, store string, at time.Time) (BackupManifest, error) {
	man := BackupManifest{
		Version:    backupManifestVersion,
		CreatedAt:  at.Format(time.RFC3339),
		Posse:      VersionString(),
		Home:       AbbrevHome(a.Home),
		QueueRepo:  AbbrevHome(queue),
		QueueStore: AbbrevHome(store),
		Excluded:   append([]string{}, BackupExcluded...),
	}

	qstage := filepath.Join(stage, "queue")
	if err := os.MkdirAll(qstage, 0o700); err != nil {
		return man, err
	}

	// The journal. `git bundle --all` is the whole history in one file and
	// it is what makes the bead-loss census (beadloss.go) survivable: the
	// census IS the git log of issues.jsonl. MEASURED on this instance
	// 2026-09-01: 1.17GiB of loose objects over 573 commits bundles to 30MB
	// in 12s, which is the number ADR 0036 §2 asked the implementation bead
	// to take.
	// A repo with no commits has no bundle to make — `git bundle` refuses to
	// write an empty one — and that is not a reason to refuse the whole
	// backup: the db and the projections are still the store of record, and
	// a freshly cut queue is exactly the state an operator most wants a
	// first archive of. Said out loud, never silent.
	if head, err := git(queue, "rev-parse", "HEAD"); err != nil {
		fmt.Fprintf(out, "  no commits in %s yet — the archive carries the store without a journal\n", AbbrevHome(queue))
	} else {
		man.QueueHead = strings.TrimSpace(head)
		if _, err := git(queue, "bundle", "create", filepath.Join(qstage, "queue.bundle"), "--all"); err != nil {
			return man, err
		}
	}

	// The database, through sqlite's online backup API against a source
	// posse never writes to (ADR 0036 §7). pairCheckReader is the harness's
	// existing answer to "how do you read this db safely" — including the
	// WAL-with-no-live-writer case where mode=ro cannot open it at all — so
	// the staging path reuses it rather than inventing a second one.
	src, cleanup, err := pairCheckReader(filepath.Join(store, "beads.db"))
	if err != nil {
		return man, Die("staging the beads db: %v", err)
	}
	defer cleanup()
	cmd := exec.Command("sqlite3", src, ".backup 'beads.db'")
	// cwd is the staging dir and the target is a bare literal, so no
	// operator-configured path is ever handed to sqlite's dot-command
	// tokenizer to quote.
	cmd.Dir = qstage
	if outp, err := cmd.CombinedOutput(); err != nil {
		return man, Die("sqlite3 .backup: %s", strings.TrimSpace(string(outp)))
	}

	// The projections beside it. issues.jsonl is what bd exports and what
	// the census reads; deleted.jsonl is the deletion ledger. A torn jsonl
	// caught mid-export is tolerable and stated (ADR 0036 §7): the db above
	// is the recovery path.
	for _, n := range queueJSONLPaths {
		p := filepath.Join(store, n)
		if !fileExists(p) {
			continue
		}
		if err := stageCopy(p, filepath.Join(qstage, n)); err != nil {
			return man, err
		}
	}

	// The constitution home: the promoted set, runtimes/, the manifest.
	// Nothing walks into envs/ or secrets/ because nothing names them —
	// the copy path only ever walks BackupHomePaths, which is the same
	// discipline promote.go keeps.
	for _, rel := range BackupHomePaths() {
		src := filepath.Join(a.Home, rel)
		st, err := os.Lstat(src)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return man, err
		}
		dst := filepath.Join(stage, "home", rel)
		if st.IsDir() {
			if err := copyTreeInto(src, dst); err != nil {
				return man, err
			}
			continue
		}
		if !st.Mode().IsRegular() {
			fmt.Fprintf(out, "  skipped %s (not a regular file)\n", AbbrevHome(src))
			continue
		}
		if err := stageCopy(src, dst); err != nil {
			return man, err
		}
	}

	members, err := walkMembers(stage)
	if err != nil {
		return man, err
	}
	man.Members = members
	return man, nil
}

// copyTreeInto copies regular files under src to dst, keeping the shape. A
// symlink or a device node is not copied and not followed: an archive that
// followed a link out of the tree would be an archive whose contents are a
// property of the box's link graph rather than of the set posse named.
func copyTreeInto(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return stageCopy(p, filepath.Join(dst, rel))
	})
}

// stageCopy is copyFile with the destination's directory made first.
// copyFile (beadpairs.go) copies into a directory its one other caller has
// already created; the staging tree is built path by path and has no such
// caller.
func stageCopy(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return copyFile(src, dst)
}

// walkMembers hashes the staged tree and returns it in stable slash-path
// order — the order the tar is written in and the order the manifest
// records, so a verifier's comparison is a walk down two sorted lists.
func walkMembers(stage string) ([]BackupMember, error) {
	var out []BackupMember
	err := filepath.WalkDir(stage, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, rerr := filepath.Rel(stage, p)
		if rerr != nil {
			return rerr
		}
		st, serr := os.Stat(p)
		if serr != nil {
			return serr
		}
		sum, serr := sha256File(p)
		if serr != nil {
			return serr
		}
		out = append(out, BackupMember{Name: filepath.ToSlash(rel), Bytes: st.Size(), SHA256: sum})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, err
}

// writeBackupArchive writes the gzip'd tar: the manifest first, then every
// member in the manifest's own order. It returns the archive's size and its
// sha256, which is what the sidecar carries.
func writeBackupArchive(part, stage string, man BackupManifest) (int64, string, error) {
	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, "", err
	}
	h := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(f, h))
	tw := tar.NewWriter(gz)
	at, _ := time.Parse(time.RFC3339, man.CreatedAt)

	write := func(name string, mode int64, body []byte, from string, size int64) error {
		hdr := &tar.Header{Name: name, Mode: mode, Size: size, ModTime: at, Typeflag: tar.TypeReg}
		if body != nil {
			hdr.Size = int64(len(body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if body != nil {
			_, err := tw.Write(body)
			return err
		}
		src, err := os.Open(from)
		if err != nil {
			return err
		}
		defer src.Close()
		n, err := io.Copy(tw, src)
		if err != nil {
			return err
		}
		if n != size {
			return Die("%s changed size while it was being archived", name)
		}
		return nil
	}

	fail := func(err error) (int64, string, error) {
		tw.Close()
		gz.Close()
		f.Close()
		return 0, "", err
	}
	body, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return fail(err)
	}
	if err := write(backupManifestName, 0o600, append(body, '\n'), "", 0); err != nil {
		return fail(err)
	}
	for _, m := range man.Members {
		if err := write(m.Name, 0o600, nil, filepath.Join(stage, filepath.FromSlash(m.Name)), m.Bytes); err != nil {
			return fail(err)
		}
	}
	if err := tw.Close(); err != nil {
		return fail(err)
	}
	if err := gz.Close(); err != nil {
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		return fail(err)
	}
	st, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		return 0, "", err
	}
	return st.Size(), hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyBackup opens an archive and checks it against the manifest inside
// it: the sidecar's hash if one is beside it, then every member's size and
// sha256, then that no member is missing and none arrived that the manifest
// does not name. It extracts nothing.
func VerifyBackup(archive string) (BackupManifest, error) {
	var man BackupManifest
	if side := sidecarFor(archive); fileExists(side) {
		want, err := os.ReadFile(side)
		if err != nil {
			return man, err
		}
		got, err := sha256File(archive)
		if err != nil {
			return man, err
		}
		if fields := strings.Fields(string(want)); len(fields) == 0 || fields[0] != got {
			return man, Die("%s does not match its sha256 sidecar", filepath.Base(archive))
		}
	}
	f, err := os.Open(archive)
	if err != nil {
		return man, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return man, Die("%s is not readable as gzip: %v", filepath.Base(archive), err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	seen := map[string]bool{}
	want := map[string]BackupMember{}
	first := true
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return man, Die("%s: %v", filepath.Base(archive), err)
		}
		if hdr.Typeflag != tar.TypeReg {
			return man, Die("%s holds a non-regular entry %q", filepath.Base(archive), hdr.Name)
		}
		if first {
			first = false
			if hdr.Name != backupManifestName {
				return man, Die("%s does not begin with %s — this is not a posse backup", filepath.Base(archive), backupManifestName)
			}
			body, err := io.ReadAll(tr)
			if err != nil {
				return man, err
			}
			if err := json.Unmarshal(body, &man); err != nil {
				return man, Die("%s: %s is not readable: %v", filepath.Base(archive), backupManifestName, err)
			}
			if man.Version > backupManifestVersion {
				return man, Die("%s is manifest version %d; this posse understands %d", filepath.Base(archive), man.Version, backupManifestVersion)
			}
			for _, m := range man.Members {
				want[m.Name] = m
			}
			continue
		}
		m, ok := want[hdr.Name]
		if !ok {
			return man, Die("%s holds %q, which its manifest does not name", filepath.Base(archive), hdr.Name)
		}
		h := sha256.New()
		n, err := io.Copy(h, tr)
		if err != nil {
			return man, err
		}
		if n != m.Bytes || hex.EncodeToString(h.Sum(nil)) != m.SHA256 {
			return man, Die("%s: %s does not match the manifest", filepath.Base(archive), hdr.Name)
		}
		seen[hdr.Name] = true
	}
	if first {
		return man, Die("%s is empty", filepath.Base(archive))
	}
	var missing []string
	for _, m := range man.Members {
		if !seen[m.Name] {
			missing = append(missing, m.Name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return man, Die("%s is missing %d member(s) its manifest names: %s", filepath.Base(archive), len(missing), strings.Join(missing, ", "))
	}
	return man, nil
}

// ─── freshness (ADR 0036 §6) ─────────────────────────────────────────────────

// BackupFreshness is what the directory says, and the directory is the only
// store: an archive is published only after it verifies, so a file's
// presence is a green verdict and its NAME is the stamp (it is the manifest
// timestamp, written into the filename at publish).
type BackupFreshness struct {
	Armed  bool
	Dir    string
	Newest string
	At     time.Time
	Age    time.Duration
	Bytes  int64
	Count  int
	MaxAge time.Duration
	Stale  bool
	Err    error

	// The future-stamped archives, which are Count's but never Newest's
	// (ADR 0036 §6, bead ranger-base-rgv61). An archive that cannot be
	// older than now is not a reading, so it is reported and skipped
	// rather than dated; FutureAhead is how far the newest of them leads
	// the clock, which is roughly how long the fault has left to run.
	Future       int
	FutureNewest string
	FutureAhead  time.Duration
}

// BackupFreshness reads the archive directory. It never creates it, never
// takes a lock, and never opens an archive: `posse status` and the cockpit
// run this on every tick.
func (a *App) BackupFreshness(now time.Time, errw io.Writer) BackupFreshness {
	f := BackupFreshness{Dir: a.BackupDir(), MaxAge: a.BackupMaxAge(errw)}
	configured := a.BackupConfigured()
	names, err := listBackups(f.Dir)
	if err != nil {
		// An unreadable directory only matters to an instance that asked
		// for backups; on any other it is a path posse has no business
		// having an opinion about.
		f.Armed, f.Err = configured, err
		return f
	}
	f.Count = len(names)
	f.Armed = configured || f.Count > 0
	if !f.Armed {
		return f
	}
	// A stamp after now is not a reading (ADR 0036 §6, bead
	// ranger-base-rgv61). Reported, never dated: BlindFor renders every
	// negative age as "0s", so an archive from the future used to make
	// this surface say "0s ago" — the freshest possible answer — for as
	// long as the stamp led the clock.
	usable, future := splitBackupsAt(names, now)
	if f.Future = len(future); f.Future > 0 {
		f.FutureNewest = future[len(future)-1]
		f.FutureAhead = backupTimeOf(f.FutureNewest).Sub(now)
	}
	if len(usable) == 0 {
		// Armed with nothing usable on disk is the predecessor's exact
		// failure — the arrangement that was configured and never ran (ADR
		// 0036 Context) — so it is stale, not silent. An empty directory
		// and a directory holding only future stamps are the same fact
		// here: no archive whose age can be believed.
		f.Stale = true
		return f
	}
	f.Newest = usable[len(usable)-1]
	f.At = backupTimeOf(f.Newest)
	if st, err := os.Stat(filepath.Join(f.Dir, f.Newest)); err == nil {
		f.Bytes = st.Size()
	}
	f.Age = now.Sub(f.At)
	f.Stale = f.Age > f.MaxAge
	return f
}

// Line is the one-line rendering `posse status` prints and `posse backup
// status` opens with.
func (f BackupFreshness) Line() string {
	switch {
	case f.Err != nil:
		return fmt.Sprintf("backup · %s could not be read: %v", AbbrevHome(f.Dir), f.Err)
	case f.Count == 0:
		return fmt.Sprintf("backup · NONE on box · %s (max age %s)", AbbrevHome(f.Dir), BlindFor(f.MaxAge))
	case f.Newest == "":
		// Files on the box, none of them a reading. The count still goes
		// out, because "NONE on box" would be a second lie next to a
		// directory that is not empty.
		return fmt.Sprintf("backup · NO USABLE ARCHIVE · %s · %d on box · %s (max age %s)",
			f.FutureClause(), f.Count, AbbrevHome(f.Dir), BlindFor(f.MaxAge))
	default:
		stale := ""
		if f.Stale {
			stale = fmt.Sprintf(" · STALE, older than %s", BlindFor(f.MaxAge))
		}
		return fmt.Sprintf("backup · %s ago · %s (%s) · %d on box · %s%s%s",
			BlindFor(f.Age), f.Newest, humanBytes(f.Bytes), f.Count, AbbrevHome(f.Dir), stale, f.futureSuffix())
	}
}

// FutureClause is the one sentence both surfaces say about a stamp that
// leads the clock — the watch loop's tick line (ADR 0036 §4) and this
// reading's own Line and GovDetail (§6). One vocabulary, because the whole
// defect was two readers of one stamp disagreeing about what it meant
// (bead ranger-base-rgv61); it is empty when there is nothing to say.
func (f BackupFreshness) FutureClause() string {
	return backupFutureClause(f.Future, f.FutureNewest, f.FutureAhead)
}

// backupFutureClause is that sentence from the three facts alone, so the
// watch loop can say it from the slice splitBackupsAt just handed it
// without re-reading the directory it has already read.
func backupFutureClause(n int, newest string, ahead time.Duration) string {
	switch {
	case n == 0:
		return ""
	case n == 1:
		return fmt.Sprintf("%s is stamped %s AHEAD of this box's clock — not a usable reading, ignored", newest, BlindFor(ahead))
	default:
		return fmt.Sprintf("%d archives are stamped AHEAD of this box's clock (newest %s, by %s) — not usable readings, ignored",
			n, newest, BlindFor(ahead))
	}
}

func (f BackupFreshness) futureSuffix() string {
	if f.Future == 0 {
		return ""
	}
	return " · " + f.FutureClause()
}

// GovDetail is the governance surface's rendering of the same fact: one
// line, and it names the threshold so the row can be acted on without a
// second command.
func (f BackupFreshness) GovDetail() string {
	switch {
	case f.Count == 0:
		return fmt.Sprintf("no backup of the store of record on this box — %s is empty (config backup_max_age: %s)",
			AbbrevHome(f.Dir), BlindFor(f.MaxAge))
	case f.Newest == "":
		return fmt.Sprintf("no usable backup of the store of record on this box — every one of the %d archives in %s is stamped ahead of the clock (%s)",
			f.Count, AbbrevHome(f.Dir), f.FutureClause())
	}
	return fmt.Sprintf("the newest backup of the store of record is %s old, past backup_max_age: %s — %s%s",
		BlindFor(f.Age), BlindFor(f.MaxAge), AbbrevHome(f.Dir), f.futureSuffix())
}

// ─── the directory ───────────────────────────────────────────────────────────

// listBackups is every published archive in dir, oldest first. Published is
// the whole test: a `.part` is a run in flight and a staging directory is
// not an archive at all, so neither is ever counted as a backup that exists.
// A directory that is not there yet is not an error — it is an instance with
// no backups.
func listBackups(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, backupPrefix) || !strings.HasSuffix(n, backupSuffix) {
			continue
		}
		if backupTimeOf(n).IsZero() {
			continue
		}
		out = append(out, n)
	}
	// By STAMP, then by the same-second sequence — not by the raw name.
	// The disambiguator a second run inside one second gets is `-2` before
	// `.tar.gz`, and '-' sorts BEFORE '.', so a plain string sort would call
	// the later archive the older one. The ages tie either way; what it
	// would get wrong is which of the pair prune reaches first.
	sort.Slice(out, func(i, j int) bool {
		ti, tj := backupTimeOf(out[i]), backupTimeOf(out[j])
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return backupSeqOf(out[i]) < backupSeqOf(out[j])
	})
	return out, nil
}

// splitBackupsAt divides a listBackups listing at now: usable is every
// archive whose stamp is at or before now, future is every archive whose
// stamp is after it. Both keep the listing's oldest-first order, so the
// last element of either is its newest.
//
// This is the one place the harness decides what a stamp from the FUTURE
// means, and both readers of the archive directory go through it — the
// watch loop's level trigger (ADR 0036 §4) and BackupFreshness (§6). They
// used not to, and that was bead ranger-base-rgv61: `now - stamp` is
// NEGATIVE for a future stamp, a negative age is under every interval, so
// one such file stopped the schedule for as long as the stamp led the
// clock; and BlindFor renders every negative age as "0s", so the surface
// that exists to catch a missing archive called it "0s ago" at the same
// time. Both went quiet together, which is the one combination with no
// witness left over.
//
// An archive that cannot be older than now is not evidence the duty was
// done, so it is not a reading — but it is not a forgery either, and
// nothing here deletes or renames it. A clock that was ahead when an
// archive was published and then corrected, a restore from a box whose
// clock was ahead, or a laptop clock jump all produce one; the file may
// well be a perfectly good archive wearing a time nobody can trust. The
// treatment is therefore: skip it as a reading, name it on every surface
// that would otherwise have been silent, and let the level trigger fall
// through to the next-newest archive it CAN date — or to none, which runs
// the verb. That direction is the safe one by cost: an extra archive costs
// disk that `backup_keep:` already bounds, and a missing one costs the
// store.
//
// The stamp is written by Format at one-second granularity from the same
// clock, so a freshly published archive is never ahead of its own writer
// and needs no grace here; strictly After is the whole test.
func splitBackupsAt(names []string, now time.Time) (usable, future []string) {
	for _, n := range names {
		if backupTimeOf(n).After(now) {
			future = append(future, n)
			continue
		}
		usable = append(usable, n)
	}
	return usable, future
}

// backupSeqOf is the same-second disambiguator in an archive's name: 1 for
// the plain form, n for `-n`.
func backupSeqOf(name string) int {
	s := strings.TrimSuffix(strings.TrimPrefix(name, backupPrefix), backupSuffix)
	i := strings.IndexByte(s, '-')
	if i < 0 {
		return 1
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// backupTimeOf reads the stamp out of an archive's name. The name IS the
// manifest timestamp (writeBackupArchive puts the same value inside), and
// reading it here is what lets `posse status` answer the age without opening
// a 45MB file on every tick. A name that does not parse is not an archive
// this posse wrote, and listBackups drops it rather than dating it from an
// mtime anything can touch.
func backupTimeOf(name string) time.Time {
	s := strings.TrimSuffix(strings.TrimPrefix(name, backupPrefix), backupSuffix)
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[:i]
	}
	t, err := time.Parse(backupStamp, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// backupName is the archive's filename, with a disambiguator only if a
// second run lands inside the same second.
func backupName(dir string, at time.Time) string {
	base := backupPrefix + at.UTC().Format(backupStamp)
	name := base + backupSuffix
	for n := 2; fileExists(filepath.Join(dir, name)); n++ {
		name = fmt.Sprintf("%s-%d%s", base, n, backupSuffix)
	}
	return name
}

func sidecarFor(archive string) string { return archive + backupSidecarSuffix }

// pruneBackups keeps the newest keep archives and removes the rest with
// their sidecars. Its safety is positional and comes from its ONE caller:
// it runs after a freshly verified archive has been published, so every
// candidate it can reach has a newer green copy above it (ADR 0036 §3). It
// is not exported, so there is no second caller to lose that property.
//
// It REPORTS rather than returning an error, and that is the design and not
// a swallow: by the time it runs the archive is written, verified and
// published, so handing a caller an error here would make a good backup
// report as a failed one. Reclamation that could not finish is a line; a
// backup that did not happen is a verdict. It also keeps going past a
// candidate it could not remove — one undeletable file must not stop the
// rest of the retention.
func pruneBackups(w io.Writer, dir string, keep int) []string {
	if keep < 1 {
		return nil
	}
	names, err := listBackups(dir)
	if err != nil {
		fmt.Fprintf(w, "  warning: retention could not read %s: %v\n", AbbrevHome(dir), err)
		return nil
	}
	if len(names) <= keep {
		return nil
	}
	var pruned []string
	for _, n := range names[:len(names)-keep] {
		p := filepath.Join(dir, n)
		if err := os.Remove(p); err != nil {
			fmt.Fprintf(w, "  warning: retention left %s in place: %v\n", n, err)
			continue
		}
		os.Remove(sidecarFor(p))
		pruned = append(pruned, p)
		fmt.Fprintf(w, "  pruned %s\n", n)
	}
	return pruned
}

// lockBackupDir is the single-flight lock (ADR 0011 §1's answer, as ADR 0036
// §3 asks for it): flock on a file in the archive directory, non-blocking,
// because a second backup is never worth waiting for — the first one is
// making the archive the second one would have made.
func lockBackupDir(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, Die("backup lock: %v", err)
	}
	if err := flock(f, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, Die("another posse backup is already running in %s — one writer per archive directory", AbbrevHome(dir))
	}
	return f, nil
}

// ─── small shared helpers ────────────────────────────────────────────────────

// humanBytes is the size as an operator reads it. Sizes here run from a few
// KB (a fresh instance) to tens of MB (this one), so two decimal places
// would be noise and none would round a 30MB bundle to "30 MB" either way.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 3; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %sB", float64(n)/float64(div), []string{"K", "M", "G", "T"}[exp])
}
