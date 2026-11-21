//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-pe5g5 (design
// ranger-base-dvw0w D2): a seat commits on a DETACHED HEAD at every tier
// posse walls — L2 seatbelt, L3 bwrap, L4 container — and so its git grant
// names no branch ref. A launch with no OS wall keeps the re-attach arm.
//
// THE PRODUCERS WERE ALREADY RIGHT; THE ORDER WAS NOT. sessionGitGrants,
// sessionRefDirs and bwrapGitSwap each answer no ref over a detached tree,
// and have since ranger-base-m2wf/uuze. What made L2/L3 seats carry the ref
// grant was that only the cage detached, and that it did so AFTER the wall
// was rendered. So the arms here are about the two things this slice moved:
//
//   - the walled-not-caged arm of PrepareSessionHead detaches and records,
//     and does NOT make the cage's furniture (config.worktree);
//   - planLaunch detaches BEFORE it renders, so the wall a first launch
//     writes names nothing under refs/heads — read off the file the launch
//     line hands the mechanism, not recomposed from the producers.
//
// The live half — a seat inside a real bwrap committing detached and refused
// every refs/heads write — is bgLiveFixtureDetached's arm in
// bwrapgitrefslive_test.go, beside the D3 row the bead names.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The walled arm, without the cage. Detached, recorded, nothing under
// refs/heads granted by either mechanism's producer, no config.worktree made
// (that file is the L4 bind's source and L2/L3 render its absence themselves)
// — and the close lands the work exactly as it does for a caged seat.
//
// MUTATIONS RUN (each reds this test): PrepareSessionHead ignoring `walled`
// (keying on `caged` again); prepareCageFurniture run for walled launches.
func TestQAWalledLaunchCommitsDetachedAndTheCloseLandsIt(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	cutAt := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch)
	// CONTROL: the attached tree really is granted the ref, or the absence
	// asserted below would hold for a reason that is not the detach.
	if !wdNamesARef(sessionGitGrants(tr.Path)) {
		t.Fatalf("CONTROL: an attached session tree's L2 grant names no refs/heads entry, so this test cannot see the detach take it away: %v", sessionGitGrants(tr.Path))
	}

	if err := PrepareSessionHead(tr, true, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, ok := treeDetachedHead(tr.Path); !ok {
		t.Fatal("a walled (L2/L3) launch must put the session on a detached HEAD")
	}
	if !launchedDetached(tr.Repo, tr.Branch) {
		t.Fatal("the walled detach was not recorded, so the close would read it as the accidental (dybv) case and refuse to land it")
	}
	own := LinkedGitDirs(tr.Path)
	if len(own) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", own)
	}
	if _, err := os.Lstat(filepath.Join(own[0], "config.worktree")); err == nil {
		t.Error("a walled-not-caged launch made config.worktree — that is the container bind's source, and L2/L3 render its absence on their own")
	}
	if g := sessionGitGrants(tr.Path); wdNamesARef(g) {
		t.Errorf("a detached walled seat's L2 grant still names a branch ref: %v", g)
	}
	if d := sessionRefDirs(tr.Path); len(d) != 0 {
		t.Errorf("a detached walled seat still gets create-only ref dirs: %v", d)
	}
	if _, keep := bwrapGitSwap(nil, own[1], repoBranch(tr.Path)); len(keep) != 0 {
		t.Errorf("a detached walled seat still gets bwrap's ref directory: %v", keep)
	}

	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != cutAt {
		t.Fatalf("a detached commit moved %s (%s → %s): the commit wrote a ref, which is the write this slice takes out of the grant", tr.Branch, cutAt[:12], now[:12])
	}

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || o.Commits != 1 {
		t.Fatalf("the close did not land a walled seat's detached work: %+v", o)
	}
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != head {
		t.Errorf("the splice did not move %s onto the tree's HEAD: %s vs %s", tr.Branch, now[:12], head[:12])
	}
}

// The relaunch half: a walled relaunch into a tree it already detached moves
// the branch up to the work FIRST, and an unwalled relaunch re-attaches —
// both as at L4, now from the walled arm.
func TestQAWalledRelaunchSplicesAndAnUnwalledOneReattaches(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareSessionHead(tr, true, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "a.txt", "one\n", "s-1: one")
	first := mustGit(t, tr.Path, "rev-parse", "HEAD")
	if _, err := a.EnsureSessionTree(repo, "s-1", nil); err != nil {
		t.Fatalf("a relaunch refused the tree posse itself detached at L2/L3: %v", err)
	}
	if err := PrepareSessionHead(tr, true, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != first {
		t.Fatalf("a walled relaunch did not move %s up to the first session's work before running: %s vs %s", tr.Branch, now[:12], first[:12])
	}
	if _, ok := treeDetachedHead(tr.Path); !ok {
		t.Fatal("a walled relaunch put the tree back on its branch")
	}

	commitIn(t, tr.Path, "b.txt", "two\n", "s-1: two")
	second := mustGit(t, tr.Path, "rev-parse", "HEAD")
	if err := PrepareSessionHead(tr, false, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := git(tr.Path, "symbolic-ref", "--short", "--quiet", "HEAD"); b != tr.Branch {
		t.Fatalf("an unwalled launch must put the tree back on %s, HEAD is on %q", tr.Branch, b)
	}
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != second {
		t.Errorf("the re-attach lost the detached commits: %s vs %s", now[:12], second[:12])
	}
	if launchedDetached(tr.Repo, tr.Branch) {
		t.Error("the record still says detached after an unwalled launch, so the dybv guard stays muted on this branch")
	}
}

// The ORDER, at the launch. A first launch of a seatbelt-tier PID into a
// fresh session tree must write a wall that names nothing under refs/heads,
// which is only true if the detach ran before the render. Read off the file
// the launch line hands the mechanism — bwrap.args or seatbelt.sb — because
// that is what the seat actually runs behind.
//
// MUTATION RUN (reds this test): moving the PrepareSessionHead call back
// below WallPrefix, beside the cage wrap.
func TestQAFirstWalledLaunchRendersNoBranchRef(t *testing.T) {
	// No t.Parallel: the package-level cage table is read, as in every
	// planLaunch pin of this tier.
	if !AvailableCages[CageSeatbelt] {
		t.Skipf("NOTHING MEASURED: this host realizes no seatbelt tier (%s unavailable)", WallMechanism())
	}
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(a.RuntimesDir(), "mycli.yaml"), "command: mycli {file}\nstate_dir: ~/.mycli-wd\n")
	write(t, filepath.Join(a.AgentsDir, "ranger.md"),
		"---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\n---\nwork\n")
	repo := wtRepo(t)

	if _, err := b.planLaunch(NewSessionOpts{Name: "wd-1", Dir: repo, Agent: "ranger", Worktree: true}); err != nil {
		t.Fatalf("a seatbelt-tier launch into a session worktree was refused: %v", err)
	}
	tree, err := a.SessionTreePath(repo, "wd-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := treeDetachedHead(tree); !ok {
		t.Fatal("a seatbelt-tier (walled) launch left its session tree on the branch")
	}
	if !launchedDetached(repo, SessionBranch("wd-1")) {
		t.Fatal("the walled launch's detach was not recorded")
	}
	file := "seatbelt.sb"
	if WallMechanism() == WallBwrap {
		file = bwrapArgsName
	}
	body, err := os.ReadFile(filepath.Join(a.GatesDir("ranger"), file))
	if err != nil {
		t.Fatalf("the launch wrote no %s to read the wall off: %v", file, err)
	}
	// NUL-separated on bwrap; a readable line either way.
	wall := strings.ReplaceAll(string(body), "\x00", "\n")
	// CONTROL: the wall really reaches this session's git dirs, or an absent
	// ref proves nothing.
	common := LinkedGitDirs(tree)
	if len(common) != 2 || !strings.Contains(wall, absResolve(filepath.Join(common[1], "objects"))) {
		t.Fatalf("CONTROL: the rendered %s does not grant this session's objects dir, so it is not the wall over this tree:\n%s", file, wall)
	}
	if strings.Contains(wall, filepath.Join("refs", "heads")) {
		t.Errorf("the first walled launch rendered a refs/heads grant — the detach ran after the render (ranger-base-pe5g5):\n%s", wdLinesWith(wall, filepath.Join("refs", "heads")))
	}
}

// The ORDER on the OTHER path that renders a persona line (ranger-base-r9slg).
// RelaunchAgent re-types a full line into a pane that already exists, renders
// its own wall to do it, and until this landed never asked PrepareSessionHead
// — so a walled seat whose tree was ATTACHED when the relaunch ran got a wall
// naming a refs/heads entry, and ran behind that file for the rest of its
// life. Two states arrive here attached and both are relaunched by this
// binary: a seat launched by a posse from before ranger-base-pe5g5, and a seat
// that ran `git checkout <branch>` in its own tree, which is the move the work
// prompt warns against in words (dispatch.go).
//
// The arm above drives the create and this one drives the re-type, because the
// render is per path: TestQAWalledRelaunchSplicesAndAnUnwalledOneReattaches
// drives PrepareSessionHead DIRECTLY and so can see neither.
//
// MUTATION RUN (reds this test): removing RelaunchAgent's PrepareSessionHead
// call, which is the state this tree was in when the bead was filed.
func TestQAAWalledRelaunchRendersNoBranchRef(t *testing.T) {
	// No t.Parallel, and the skip: the package-level cage table is read, as
	// in every walled-render pin of this tier.
	if !AvailableCages[CageSeatbelt] {
		t.Skipf("NOTHING MEASURED: this host realizes no seatbelt tier (%s unavailable)", WallMechanism())
	}
	b, fake := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(a.RuntimesDir(), "mycli.yaml"), "command: mycli {file}\nstate_dir: ~/.mycli-wd\n")
	write(t, filepath.Join(a.AgentsDir, "ranger.md"),
		"---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\n---\nwork\n")
	// A runtime herdr can NAME, or the re-type refuses before it renders
	// anything (ADR 0013 §1 property 3, relaunchdetection_qa_test.go).
	herdrNames(t, fake, "claude", "mycli")
	repo := wtRepo(t)
	mustCreate(t, b, NewSessionOpts{Name: "wdr-1", Dir: repo, Agent: "ranger", Worktree: true})
	tree, err := a.SessionTreePath(repo, "wdr-1")
	if err != nil {
		t.Fatal(err)
	}
	branch := SessionBranch("wdr-1")
	// The state the relaunch meets: a seat back on its branch, with the
	// launcher's record cleared, which is what a pre-pe5g5 launch left and
	// what a `git checkout <branch>` in the seat's own tree makes today.
	if _, err := git(tree, "checkout", branch); err != nil {
		t.Fatalf("fixture: the session tree could not be put back on %s: %v", branch, err)
	}
	if err := recordDetached(repo, branch, false); err != nil {
		t.Fatal(err)
	}
	// CONTROL: an attached tree really is granted the ref, or an absent ref
	// below would hold for a reason that is not the detach.
	if !wdNamesARef(sessionGitGrants(tree)) {
		t.Fatalf("CONTROL: an attached session tree's L2 grant names no refs/heads entry, so this test cannot see the detach take it away: %v", sessionGitGrants(tree))
	}

	// The fixture that reaches the re-type path: herdr stops reporting an
	// agent, and the launch is pushed past the relaunch grace.
	os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("wdr-1")
	m.Launched = m.Launched.Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	if m.Repo == "" || m.Branch == "" {
		t.Fatalf("fixture: the meta records no session tree (repo %q, branch %q), so the relaunch has nothing to prepare", m.Repo, m.Branch)
	}
	if ok, err := b.RelaunchAgent("wdr-1", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}

	if _, ok := treeDetachedHead(tree); !ok {
		t.Error("a walled relaunch left its session tree on the branch — the next commit in it writes a ref the wall is meant to deny")
	}
	if !launchedDetached(repo, branch) {
		t.Error("the walled relaunch's detach was not recorded, so the close would read it as the accidental (dybv) case and refuse to land the work")
	}
	file := "seatbelt.sb"
	if WallMechanism() == WallBwrap {
		file = bwrapArgsName
	}
	body, err := os.ReadFile(filepath.Join(a.GatesDir("ranger"), file))
	if err != nil {
		t.Fatalf("the relaunch wrote no %s to read the wall off: %v", file, err)
	}
	// NUL-separated on bwrap; a readable line either way.
	wall := strings.ReplaceAll(string(body), "\x00", "\n")
	// CONTROL: the wall really reaches this session's git dirs, or an absent
	// ref proves nothing.
	common := LinkedGitDirs(tree)
	if len(common) != 2 || !strings.Contains(wall, absResolve(filepath.Join(common[1], "objects"))) {
		t.Fatalf("CONTROL: the rendered %s does not grant this session's objects dir, so it is not the wall over this tree:\n%s", file, wall)
	}
	if strings.Contains(wall, filepath.Join("refs", "heads")) {
		t.Errorf("the walled RELAUNCH rendered a refs/heads grant — the re-type path renders without asking PrepareSessionHead (ranger-base-r9slg):\n%s", wdLinesWith(wall, filepath.Join("refs", "heads")))
	}
}

// An unwalled launch (`cage: shims`) keeps today's shape: on the branch, no
// record. The control for the arm above, at the same door.
func TestQAUnwalledLaunchStaysOnTheBranch(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(a.RuntimesDir(), "mycli.yaml"), "command: mycli {file}\nstate_dir: ~/.mycli-wd\n")
	write(t, filepath.Join(a.AgentsDir, "ranger.md"),
		"---\nname: ranger\ndescription: t\nruntime: mycli\ncage: shims\n---\nwork\n")
	repo := wtRepo(t)
	if _, err := b.planLaunch(NewSessionOpts{Name: "wd-2", Dir: repo, Agent: "ranger", Worktree: true}); err != nil {
		t.Fatalf("a shims-tier launch into a session worktree was refused: %v", err)
	}
	tree, err := a.SessionTreePath(repo, "wd-2")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := git(tree, "symbolic-ref", "--short", "--quiet", "HEAD"); b != SessionBranch("wd-2") {
		t.Fatalf("an unwalled launch must stay on %s, HEAD is on %q", SessionBranch("wd-2"), b)
	}
	if launchedDetached(repo, SessionBranch("wd-2")) {
		t.Error("an unwalled launch recorded a detach")
	}
}

// The L2 row on darwin: the shipped profile, rendered over a tree the walled
// prepare detached, under a real sandbox-exec. The commit goes through and
// moves no ref; the session's own branch, a peer's and a new one are all
// refused. Skips everywhere sandbox-exec cannot apply a profile — on this
// fleet it runs on ci.yml's darwin leg (the bwrap twin is
// TestLiveBwrapWalledSeatCommitsDetachedAndCannotWriteARef).
func TestQASeatbeltWalledSeatCommitsDetachedAndCannotWriteARef(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := wgNewFixture(t)
	st := &SessionTree{Repo: f.repo, Path: f.tree, Branch: f.branch, Base: "main"}
	if err := PrepareSessionHead(st, true, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	// The seat's git commits into its own store, as the L1 shim points it
	// (ranger-base-5blmt): the shipped profile denies `<common>/objects`.
	prof, pre := wgSeatProfile(t, f.a, f.ag, f.repo, f.tree)
	if strings.Contains(mustRead(t, prof), filepath.Join("refs", "heads")) {
		t.Errorf("the profile rendered over a detached walled seat names refs/heads:\n%s", wdLinesWith(mustRead(t, prof), filepath.Join("refs", "heads")))
	}
	cut := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch)
	sh := "echo walled >> " + filepath.Join(f.tree, "work.txt") +
		" && git -C " + f.tree + " add work.txt" +
		" && git -C " + f.tree + " commit -q -m 'walled commit' -- work.txt"
	if ok, out := wgRun(t, prof, pre+sh); !ok {
		t.Fatalf("a walled seat cannot commit on its detached HEAD under the profile:\n%s", out)
	}
	head := mustGit(t, f.tree, "rev-parse", "HEAD")
	if head == cut {
		t.Fatal("the commit exited 0 and HEAD did not move")
	}
	// The commit is the seat's alone until the landing imports it.
	if siHas(t, f.repo, head) {
		t.Errorf("the walled seat's commit %s is already in the shared store — it was written past the seat store", head)
	}
	for _, c := range []string{
		"git -C " + f.tree + " update-ref refs/heads/" + f.branch + " " + head,
		"git -C " + f.tree + " update-ref refs/heads/posse/other-probe " + head,
		"git -C " + f.tree + " update-ref refs/heads/posse/new-probe " + head,
		"git -C " + f.tree + " update-ref refs/heads/main " + head,
	} {
		if ok, out := wgRun(t, prof, pre+c); ok {
			t.Errorf("a walled seat's profile allowed `%s`:\n%s", c, out)
		}
	}
	if now := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch); now != cut {
		t.Errorf("%s moved from %s to %s under a profile that grants no ref", f.branch, cut, now)
	}
	if err := spliceDetachedWork(st); err != nil {
		t.Fatal(err)
	}
	if now := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch); now != head {
		t.Errorf("the splice did not land the walled seat's work: %s vs %s", now, head)
	}
	if !siHas(t, f.repo, head) {
		t.Errorf("the splice moved the branch to %s and the shared store does not hold it — the import was a no-op on this tier", head)
	}
}

func wdNamesARef(grants []string) bool {
	for _, g := range grants {
		if strings.Contains(g, filepath.Join("refs", "heads")) {
			return true
		}
	}
	return false
}

func wdLinesWith(s, sub string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
