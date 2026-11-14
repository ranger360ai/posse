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
//
// THE NARROWING IS WHAT MAKES THE REFUSALS REAL, and ADR 0086's own
// measurement is the cautionary tale: a code seat rendered its wall over a
// scratch repo under `/tmp`, which the real set binds READ-WRITE for
// TMPDIR, so its "move main" row measured nothing. This fixture's repo is
// under `/tmp` too (gitTempDir), so the set below is SeatbeltWritable's
// answer narrowed to this fixture's own repo and worktree — the blanket temp
// grant, the home caches and the gates dir left out, the same reason sbRoot
// puts its root outside TMPDIR at the other mechanism.
//
// SINCE ADR 0086 D3 THE SET COMES FROM THE SHIPPED PRODUCER AND NOT FROM A
// HAND-BUILT LIST (bgLiveSessionGrants). The git half used to be typed here
// as `{cwd} + sessionGitGrants(cwd)`, which is the shape of a PID that
// denies nothing — and D3's whole subject is a PID whose cwd arm is a
// DIFFERENT list. A hand-built set cannot measure a producer's own branch,
// which is D6's lesson said twice: R0 escaped because no pin ever drove a
// whole-tree-deny PID through this render at all.

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
//
// bwrapNestSkip is the third question and the one that used to be missing:
// every arm below execs a bwrap, and a process already inside a seat cannot
// start one (ADR 0096 D1). Until D3 these arms FAILED there rather than
// abstaining — the wall working read as a regression.
func bgLiveSkip(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16); this host's is sandbox-exec")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	bwrapNestSkip(t)
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
	own, common      string
	argv             []string
	keep             string
	ag               *AgentFile
}

// bgLiveFixture is the fixture for a PID that denies nothing: the arms below
// it measure what the git grants OPEN, so the PID has to be one that gets
// them.
func bgLiveFixture(t *testing.T) bgLive {
	t.Helper()
	return bgLiveFixtureFor(t, nil)
}

// bgLiveFixtureFor is that fixture for one PID's deny list, which is the one
// input ADR 0086 D3 reads. `nil` is every arm written before it.
func bgLiveFixtureFor(t *testing.T, deny []string) bgLive {
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
	f := bgLive{root: root, repo: repo, tree: tree,
		own: absResolve(dirs[0]), common: absResolve(dirs[1]),
		ag: &AgentFile{Name: "bg-live", Deny: deny}}
	set, keep := BwrapWritable(f.ag, bgLiveSessionGrants(t, repo, tree, f.ag), tree)
	// The keep file is the ref directory's, so a PID granted no ref
	// directory must be named none — and asserting the COUNT either way is
	// what keeps this fixture from quietly measuring the wrong wall (ADR
	// 0086 D3; D20 for the grant it is the keep file of).
	wantKeep := 1
	if deniesFileWrite(f.ag.Deny) {
		wantKeep = 0
	}
	if len(keep) != wantKeep {
		t.Fatalf("the render named %d keep files for posse/s1 under deny %v, want %d: %v", len(keep), deny, wantKeep, keep)
	}
	if wantKeep == 1 {
		f.keep = absResolve(keep[0])
	}
	argv, prep, err := BwrapArgs(tree, set, SeatbeltCarveOut{}, keep...)
	if err != nil {
		t.Fatalf("BwrapArgs refused the worktree grant for deny %v: %v", deny, err)
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
	if f.keep != "" {
		if _, err := os.Stat(f.keep); err != nil {
			t.Fatalf("the prep list did not leave the keep file in place: %v", err)
		}
	}
	return f
}

// bgLiveSessionGrants is the cwd half of the SHIPPED writable set for one
// PID, narrowed to this fixture's own repo and worktree. The narrowing is
// the file header's third paragraph: everything else SeatbeltWritable grants
// — TMPDIR (which this fixture is UNDER), the home caches, the gates dir,
// the memory dir, posse's state dir — belongs to the launch and not to the
// question, and binding the first of them read-write would make every
// refusal below meaningless.
//
// The App's home is a sibling of the repo, so the filter is the whole of the
// narrowing and no grant is kept by accident. It is wgRepoGrants, the same
// narrowing the D3 set pins use (seatbeltdenygitgrant_qa_test.go), because
// two spellings of "this fixture's own repo" is two answers waiting to
// disagree about which wall was measured.
func bgLiveSessionGrants(t *testing.T, repo, tree string, ag *AgentFile) []string {
	t.Helper()
	a := NewAppAt(filepath.Join(filepath.Dir(repo), "home"))
	ag.MemoryDir = filepath.Join(a.Home, "personas", ag.Name)
	out := wgRepoGrants(a.SeatbeltWritable(ag, tree, a.GatesDir(ag.Name)), repo, tree)
	if len(out) == 0 {
		t.Fatalf("the writable set reaches nothing in %s or %s, so the wall below is the read-only root and every arm passes for the wrong reason", repo, tree)
	}
	return out
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

// ─── ADR 0086 D3: the same render for a PID that denies the whole tree ───────

// G1-G4 and W14/W15 of ADR 0086's table, under the argv posse renders.
//
// This is the arm R0 escaped through. Every bwrap fixture with `deniesFiles`
// before it was a plain directory or a main checkout, so no pin ever drove a
// whole-tree-deny PID through this render in a LINKED WORKTREE — where
// `<cwd>/.git` is a FILE and the grant that named it refused the whole
// render (`bwrap cannot grant write on <cwd>/.git: it is not a directory`).
// The render succeeding is therefore half of what this measures, and
// bgLiveFixtureFor fatals on that half before a single arm runs.
//
// The other half is why D3 took the git dirs away and not only the file: a
// seat that cannot write a byte in its tree could still mint a commit by
// plumbing and move `posse/<session>`, which the launcher fast-forwards onto
// `main` when the bead closes. So the commit arm reads the OPERATOR's
// checkout afterwards, like every other arm in this file: a ref write that
// lands somewhere nothing reads exits 0.
func TestLiveBwrapWholeTreeDenyGetsNoGitWriteInItsSession(t *testing.T) {
	t.Parallel()
	bgLiveSkip(t)
	f := bgLiveFixtureFor(t, []string{"Edit", "Write"})
	before := f.branchSha(t, "posse/s1")

	// What the wall must refuse. The cwd create is D5's witness by hand, and
	// the two plumbing arms are W14/W15: `hash-object -w` is the write that
	// makes a blob, and the commit is the whole capability.
	for _, c := range []struct {
		name string
		arg  []string
	}{
		{"create a file in the session's own tree", []string{"sh", "-c", ": > " + filepath.Join(f.tree, ".posse-wall-witness")}},
		{"write a loose object with git plumbing", []string{"sh", "-c", "echo x | " + strings.Join(f.gitIn("hash-object", "-w", "--stdin"), " ")}},
		{"commit on the session's own branch", f.gitIn("commit", "-q", "--allow-empty", "-m", "d3")},
		{"write the session's own per-worktree git dir", []string{"sh", "-c", "echo x > " + filepath.Join(f.own, "PWNED")}},
		{"write the shared object store", []string{"sh", "-c", "echo x > " + filepath.Join(f.common, "objects", "PWNED")}},
		{"move the session's own ref by hand", []string{"sh", "-c", "echo " + before + " > " + filepath.Join(f.common, "refs", "heads", "posse", "s1")}},
	} {
		if ok, out := f.walled(t, c.arg...); ok {
			t.Errorf("%s was ALLOWED under the rendered wall for a PID that denies Edit and Write:\n%s", c.name, out)
		}
	}
	if got := f.branchSha(t, "posse/s1"); got != before {
		t.Fatalf("posse/s1 moved from %s to %s under a wall that is supposed to grant no git write — the Edit/Write deny exists to make a findings-only seat's output findings, and the launcher fast-forwards this branch onto main (ADR 0086 D3, W14/W15)", before, got)
	}

	// What it must still allow, G1-G3: reading the repo is the whole of what
	// a findings-only seat does. `git status` exits 0 and simply does not
	// save its index refresh — a refusal here is the cost D3 priced, and it
	// would make the tier unusable for the one PID it was built for.
	for _, c := range []struct {
		name string
		arg  []string
	}{
		{"git status", f.gitIn("status", "--porcelain")},
		{"git log", f.gitIn("log", "--oneline", "-1")},
		{"git diff", f.gitIn("diff", "HEAD")},
		{"read a tracked file", []string{"sh", "-c", "cat " + filepath.Join(f.tree, ".git")}},
	} {
		if ok, out := f.walled(t, c.arg...); !ok {
			t.Errorf("%s was REFUSED under the rendered wall, and ADR 0086 G1-G3 measured it allowed:\n%s", c.name, out)
		}
	}

	// The store of record is the one thing D3 keeps, because bd commits
	// there (SeatbeltWritable's beadsHome arm). Here it is `<cwd>/.beads`,
	// which the prep made, so this is the arm that says the seat can still
	// claim, comment and close.
	if ok, out := f.walled(t, "sh", "-c", ": > "+filepath.Join(f.tree, ".beads", "witness")); !ok {
		t.Errorf("the bead store is not writable, so a findings-only seat cannot claim or close its own bead:\n%s", out)
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
