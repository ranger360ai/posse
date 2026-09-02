package posse

// Live pins for rangerhq-09o2's beads clause, run against the real bd rather
// than the fake:
//
//	a persona in its session worktree reads and writes the SAME work graph
//	as the main checkout — no forked database, no staleness warning, and no
//	--allow-stale — and the store posse tells the CAGE about is the one bd
//	actually opens.
//
//	RHQ_LIVE_BD=1 go test ./internal/rhq -run TestLiveWorktree -v
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
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}
	return func(dir string, args ...string) (string, error) {
		cmd := exec.Command("bd", append([]string{"--no-daemon"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
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

// liveBeadsRepo is a real git repo with a real bd database holding one row
// titled `row`, committed. The daemon bd's own pre-commit hook starts here is
// reaped in cleanup.
func liveBeadsRepo(t *testing.T, bd func(string, ...string) (string, error), row string) string {
	t.Helper()
	repo := wtRepo(t)
	if out, err := bd(repo, "init"); err != nil {
		t.Skipf("bd init did not take in a throwaway repo: %v %s", err, out)
	}
	t.Cleanup(func() { stopLeakedDaemons(t, repo) })
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
	bd := liveBd(t)
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
	// as it is above.)
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
		t.Fatalf("the planted redirect resolves but returns no rows, so it is not a live database: %v\n%s", err, out)
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
