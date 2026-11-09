//go:build !posse_arm2 && !posse_arm3

package posse

// Live pin for ranger-base-gi1gi — ADR 0080 D20's git grants, run against a
// real bwrap on a real linked worktree. bwrapgitrefs_qa_test.go proves what
// is RENDERED; this is the join:
//
//	the argv posse renders for a worktree session, handed to the real
//	binary, really lets that session commit on its own branch and really
//	refuses it the shared refs, hooks and config beside it.
//
//	go test ./internal/posse -run TestLiveBwrapGitRefs -v
//
// NOT env-gated, unlike the other live pins here: it skips off Linux and
// without `bwrap`, which on this fleet means it skips on every darwin seat
// and runs unasked on the D1 host. Nothing it needs has a version, a daemon
// or a cache — git and bwrap — and it spends no API turn: the "runtime"
// inside the wall is `git` and `sh`.
//
// It mirrors the probe's T2-T4e (docs/adr/0080-bwrap-tier.probe.sh), which is
// the row-for-row twin of this file and the thing to paste on a bead. What
// this adds over the probe is the RENDER: the probe types its own `--bind`
// list by hand, so a probe that passes says nothing about what posse would
// have typed. Here the list comes from BwrapWritable and BwrapArgs, and the
// prep from the same call, so a drift between the design and the render reds
// here and nowhere else.
//
// THE WRITABLE SET IS THE GIT HALF ALONE, deliberately. SeatbeltWritable also
// carries the generic CLI caches under the home — and carried `/private/tmp`
// and three darwin paths unconditionally until D18 narrowed them per GOOS
// (ADR 0080 D17's residual, closed in `homeCacheGrants`). Varying only what
// this bead is about is what makes a refusal below attributable: the one
// thing that differs between the arms is a git grant.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// bgLiveSkip is the one gate. `bwrap` and not a resolved absolute path (D18
// types the absolute one) because this is a test asking whether the host HAS
// the binary, not which file a launch line should name.
func bgLiveSkip(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16); this host's is sandbox-exec")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
}

// bgLive is the fixture: the operator's checkout, this session's linked
// worktree on `posse/s1`, and a PEER session's branch in the same ref
// directory — which is the one the widening is about, so a fixture without
// it could not measure it.
type bgLive struct {
	root, repo, tree string
	common           string
	argv             []string
	keep             string
}

func bgLiveFixture(t *testing.T) bgLive {
	t.Helper()
	// gitTempDir and not t.TempDir(): a `git init` under a bare root leaves
	// TempDir's own RemoveAll racing a concurrent write in the repo it made
	// (gittempdirroot_qa_test.go pins every init in this package to this).
	root := gitTempDir(t)
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	mustGit(t, repo, "commit", "-q", "--allow-empty", "-m", "root")
	tree := filepath.Join(root, "wt")
	mustGit(t, repo, "worktree", "add", "-q", "-b", "posse/s1", tree)
	mustGit(t, repo, "branch", "-q", "posse/s2") // the peer session's ref
	// Packed, as a real checkout is, so the loose refs in play are only the
	// ones made since: `posse/s1` and `posse/s2` stay loose because
	// `worktree add` and `branch` wrote them after this.
	mustGit(t, repo, "pack-refs", "--all")

	dirs := LinkedGitDirs(tree)
	if len(dirs) != 2 {
		t.Fatalf("no linked worktree was made: LinkedGitDirs(%s) = %v", tree, dirs)
	}
	f := bgLive{root: root, repo: repo, tree: tree, common: absResolve(dirs[1])}
	set, keep := BwrapWritable(append([]string{absResolve(tree)}, sessionGitGrants(tree)...), tree)
	if len(keep) != 1 {
		t.Fatalf("the render named %d keep files for posse/s1, want 1: %v", len(keep), keep)
	}
	f.keep = absResolve(keep[0])
	argv, prep, err := BwrapArgs(tree, set, SeatbeltCarveOut{}, keep...)
	if err != nil {
		t.Fatalf("BwrapArgs refused the worktree grant: %v", err)
	}
	f.argv = argv
	// The launcher's half, which runs OUTSIDE the wall: D18 is what makes
	// this production code (RenderBwrap), and until then the test plays
	// launcher exactly as the probe's shell does — in the order the list
	// gives, which is the list's whole promise.
	for _, s := range prep {
		switch s.Kind {
		case PrepMkdirAll:
			if err := os.MkdirAll(s.Path, 0o755); err != nil {
				t.Fatalf("prep %s: %v", s, err)
			}
		case PrepTouch:
			fh, err := os.OpenFile(s.Path, os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatalf("prep %s: %v", s, err)
			}
			fh.Close()
		default:
			t.Fatalf("the render asked for a prep step this test cannot run: %s", s)
		}
	}
	if _, err := os.Stat(f.keep); err != nil {
		t.Fatalf("the prep list did not leave the keep file in place: %v", err)
	}
	return f
}

// walled runs one command inside the rendered wall and answers whether it was
// allowed, with the output for a failure message. --chdir is already in the
// argv, so the command runs in the session's tree the way a pane does.
func (f bgLive) walled(t *testing.T, arg ...string) (bool, string) {
	t.Helper()
	cmd := exec.Command("bwrap", append(append([]string{}, f.argv...), arg...)...)
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}

// gitIn is a git invocation with an identity on the command line: $HOME is
// read-only inside the wall and the fixture's config is the repo's, so a
// commit that had to resolve an identity from the environment would fail for
// a reason that is not the wall.
func (f bgLive) gitIn(arg ...string) []string {
	return append([]string{"git", "-c", "user.name=p", "-c", "user.email=p@example.com", "-C", f.tree}, arg...)
}

// branchSha is what the OPERATOR's checkout thinks the session's branch
// points at, read from outside the wall. It is the question a seat's exit
// code cannot answer: a ref write that lands somewhere nothing reads exits 0.
func (f bgLive) branchSha(t *testing.T, name string) string {
	t.Helper()
	out, err := git(f.repo, "rev-parse", "refs/heads/"+name)
	if err != nil {
		return ""
	}
	return out
}

// ─── T2/T3: what the grant opens and what it leaves closed ───────────────────

// The commit lands, and the operator's checkout sees the branch move. The
// second half is the one the probe's T2 cannot see: the whole shape of D20's
// risk is a write that goes through the mount and nowhere else.
func TestLiveBwrapGitRefsCommitsOnTheSessionBranch(t *testing.T) {
	t.Parallel()
	bgLiveSkip(t)
	f := bgLiveFixture(t)
	before := f.branchSha(t, "posse/s1")

	ok, out := f.walled(t, f.gitIn("commit", "-q", "--allow-empty", "-m", "t2")...)
	if !ok {
		t.Fatalf("the session cannot commit on its own branch under the rendered wall:\n%s", out)
	}
	after := f.branchSha(t, "posse/s1")
	if after == "" || after == before {
		t.Fatalf("the commit exited 0 and the operator's checkout still reads posse/s1 at %s — the ref write landed somewhere nothing reads", before)
	}
}

// What the grant leaves closed, each one a capability ranger-base-m2wf was
// opened to close and none of them reachable through L1 or L3: a ref move is
// not a push, so the push shim never sees it and pre-push never fires.
func TestLiveBwrapGitRefsRefusesTheSharedGitState(t *testing.T) {
	t.Parallel()
	bgLiveSkip(t)
	f := bgLiveFixture(t)
	mainBefore := f.branchSha(t, "main")

	for _, c := range []struct {
		name string
		arg  []string
	}{
		{"move main", f.gitIn("update-ref", "refs/heads/main", "HEAD")},
		{"write the shared hooks dir", []string{"sh", "-c", "echo x > " + filepath.Join(f.common, "hooks", "pre-commit")}},
		{"write the shared config", []string{"sh", "-c", "echo x >> " + filepath.Join(f.common, "config")}},
		{"write packed-refs", []string{"sh", "-c", "echo x >> " + filepath.Join(f.common, "packed-refs")}},
	} {
		if ok, out := f.walled(t, c.arg...); ok {
			t.Errorf("%s was ALLOWED under the rendered wall:\n%s", c.name, out)
		}
	}
	if got := f.branchSha(t, "main"); got != mainBefore {
		t.Fatalf("main moved from %s to %s", mainBefore, got)
	}
}

// The widening, by execution (ADR 0080's Consequences, probe T3d). The ref
// directory holds every peer session's branch, so a bwrap seat can move one;
// the darwin mechanism's leaf grant cannot. It is pinned ALLOWED on purpose:
// parity prints this as a declared difference, and a line claiming a
// widening the wall does not have is worse than no line.
func TestLiveBwrapGitRefsCanMoveAPeerSessionsRef(t *testing.T) {
	t.Parallel()
	bgLiveSkip(t)
	f := bgLiveFixture(t)

	ok, out := f.walled(t, f.gitIn("update-ref", "refs/heads/posse/s2", "HEAD")...)
	if !ok {
		t.Fatalf("a peer session's ref in the granted directory was refused — the widening parity declares is not real, and ADR 0080 D20's Consequences entry needs revisiting instead of this test:\n%s", out)
	}
}

// ─── T4: an outside gc while the seat is up ──────────────────────────────────

// The case the keep file exists for. A `git pack-refs --all --prune` in the
// operator's checkout — which `git gc` runs on its own at gc.auto — writes
// every loose ref into `packed-refs`, deletes the loose files and removes the
// directories that just emptied. The keep file is what makes the bound
// directory non-empty, so the prune leaves it.
//
// Run WHILE the seat holds the mount, which is the only arm that measures
// anything: between launches the prep's `mkdir -p` would simply remake the
// directory (bwrapgitrefs_qa_test.go pins that it asks for it).
func TestLiveBwrapGitRefsCommitsAfterAnOutsidePackRefs(t *testing.T) {
	t.Parallel()
	bgLiveSkip(t)
	f := bgLiveFixture(t)
	before := f.branchSha(t, "posse/s1")
	dir := filepath.Dir(f.keep)

	ok, out := f.packUnderALiveSeat(t, "t4")
	if !ok {
		t.Fatalf("the seat's commit failed after an outside pack-refs --all --prune:\n%s", out)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the keep file did not keep %s: %v", dir, err)
	}
	after := f.branchSha(t, "posse/s1")
	if after == "" || after == before {
		t.Fatalf("the commit exited 0 and the operator's checkout still reads posse/s1 at %s — the ref write landed in a directory nothing reads", before)
	}
}

// The control, and it is written as a COHERENCE assertion rather than as an
// expected outcome, because the outcome is the one thing here that no darwin
// box can settle (ADR 0080's "Measured versus assumed": the keep file and the
// post-pack commit are ASSUMED, read by probe T4-T4e).
//
// Two answers are plausible and the design holds under both. Either the host
// prunes the directory under the live mount, which is what D20 reasons from
// and what makes the keep file load-bearing; or `rmdir` refuses, because the
// directory is a mountpoint in the seat's namespace and the dentry is shared
// — `vfs_rmdir` returns EBUSY for a dentry anything is mounted on — in which
// case the keep file is belt-and-braces and costs an empty file.
//
// WHAT MUST NEVER BE TRUE is the third combination: the commit exits 0 and
// the operator's checkout does not see it. That is a seat writing its branch
// into an unlinked directory, exit 0, nothing red, and the launcher's
// ff-merge later finding no commit — "work left uncommitted did not happen",
// with every signal saying it did. This arm fails on exactly that, whichever
// way the host answers the other question, and logs which answer it gave so
// the first run on the D1 host is the measurement ADR 0080 D22 asks for.
func TestLiveBwrapGitRefsWithoutTheKeepFileNeverCommitsIntoAPrunedDir(t *testing.T) {
	t.Parallel()
	bgLiveSkip(t)
	f := bgLiveFixture(t)
	before := f.branchSha(t, "posse/s1")
	dir := filepath.Dir(f.keep)
	if err := os.Remove(f.keep); err != nil {
		t.Fatalf("the control could not remove the keep file, so it varies nothing: %v", err)
	}

	ok, out := f.packUnderALiveSeat(t, "t4e")
	_, statErr := os.Stat(dir)
	switch {
	case !ok:
		t.Logf("MEASURED: no keep file — the prune took %s (present=%v) and the seat's commit failed, which is D20's reasoning:\n%s",
			dir, statErr == nil, out)
	case f.branchSha(t, "posse/s1") == before:
		t.Fatalf("the seat's commit exited 0 and the operator's checkout still reads posse/s1 at %s: the ref write landed in a directory nothing reads (%s present=%v). A seat cannot be allowed to commit into a pruned ref directory — ADR 0080 D20's keep file is the remedy and this says it did not work",
			before, dir, statErr == nil)
	default:
		t.Logf("MEASURED: no keep file and the commit still landed, so the prune did not take %s (present=%v) — the directory is a mountpoint in the seat's namespace and rmdir refuses. The keep file costs an empty file and buys nothing on this kernel; ADR 0080 D20's T4e expectation is the thing to revisit, not this test",
			dir, statErr == nil)
	}
}

// packUnderALiveSeat is the dance both arms above need, and the reason it is
// one function: a seat is started inside the wall, it signals that its
// namespace is up by writing a file in its own tree, the pack runs in the
// operator's checkout while that mount is held, and only then is the seat
// released to commit. A sleep in place of the ready file would be a race that
// passes by rendering the pack harmless.
func (f bgLive) packUnderALiveSeat(t *testing.T, msg string) (bool, string) {
	t.Helper()
	ready, release := filepath.Join(f.tree, ".ready"), filepath.Join(f.tree, ".go")
	inner := "echo up > " + ready + "; while [ ! -f " + release + " ]; do sleep 0.05; done; exec " +
		strings.Join(f.gitIn("commit", "-q", "--allow-empty", "-m", msg), " ")
	cmd := exec.Command("bwrap", append(append([]string{}, f.argv...), "sh", "-c", inner)...)
	outBuf := &strings.Builder{}
	cmd.Stdout, cmd.Stderr = outBuf, outBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start the walled seat: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			cmd.Wait()
			t.Fatalf("the walled seat never came up (no %s):\n%s", ready, outBuf.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	mustGit(t, f.repo, "pack-refs", "--all", "--prune")
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatalf("could not release the seat: %v", err)
	}
	err := cmd.Wait()
	return err == nil, strings.TrimSpace(outBuf.String())
}
