//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-gi1gi — ADR 0080 D20: git's ref writes on the
// bwrap wall.
//
// EVERY ARM RUNS ON DARWIN, for the reason bwrapargs_qa_test.go's header
// gives: the substitution and the render are pure, so what a row needs is a
// real linked worktree (wgNewFixture, seatbeltworktreegit_qa_test.go) and
// not a Linux host. The wall itself is measured in bwrapgitrefslive_test.go,
// which skips off Linux, and by the probe's T2-T5
// (docs/adr/0080-bwrap-tier.probe.sh).
//
// WHAT MAKES THESE WORTH PINNING. The grant this bead replaces is a pair of
// FILE subpaths, and every way of getting the replacement wrong is quiet:
//
//  1. the leaf or its `.lock` left in the set. The leaf exists, so BwrapArgs
//     refuses the launch and names it — loud. The `.lock` does NOT exist, so
//     shape 1 prep `mkdir -p`s it into a DIRECTORY named `<ref>.lock`, in
//     the operator's git dir, which fails every commit on that branch for
//     every session on the repo until a human removes it. The arm that
//     catches it is the one that asserts the set, not the one that asserts
//     the render.
//  2. `packed-refs.lock` left in the set. Same shape, worse blast radius: a
//     DIRECTORY by that name kills the operator's every `gc` and
//     `pack-refs` permanently (ADR 0059 D4 declined the container twin for
//     exactly this).
//  3. the ref directory granted for a SLASHLESS branch, which is
//     `refs/heads` itself — `refs/heads/main` writable from a dispatched
//     session, the capability ranger-base-m2wf was opened to close. Nothing
//     about the argv says so; it is one more `--bind` line.
//  4. the keep file not asked for, or asked for before its directory exists.
//     An outside `gc` then empties and prunes the bound directory under a
//     live seat, and the seat keeps writing into an unlinked inode. Measured
//     at the other mechanism (ranger-base-uuze) and the control for it here
//     is the ORDER of the prep list.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bgSwap is the shipped path: the writable set a worktree session really
// gets, through the substitution this bead adds. Spelled once so that no arm
// below hand-assembles a set — a row built from a different list would be
// measuring two things at once (wgFixture.wide's reason).
func bgSwap(t *testing.T, f wgFixture, cwd string) (set []string, keep []string) {
	t.Helper()
	return BwrapWritable(f.a.SeatbeltWritable(f.ag, cwd, f.gates), cwd)
}

// bgPrepUnder is the prep list narrowed to the shared git dir, which is the
// only region this bead has anything to say about. The full list is longer
// and will stay longer: SeatbeltWritable grants the generic CLI caches under
// the home unconditionally, and in a fixture home none of them exist, so
// shape 1 asks the launcher to make them. That is D17's residual and D18's
// to narrow (ADR 0080 D17, "a missing grant is not free here") — asserting
// over the whole list here would make every arm below fail the day it is
// narrowed, and pass today for the wrong reason.
func bgPrepUnder(prep []PrepStep, dir string) []string {
	var out []string
	for _, s := range prep {
		if underDir(dir, s.Path) {
			out = append(out, s.String())
		}
	}
	return out
}

func bgHas(set []string, p string) bool {
	for _, w := range set {
		if absResolve(w) == absResolve(p) {
			return true
		}
	}
	return false
}

// ─── the substitution ────────────────────────────────────────────────────────

// The fleet's own shape, which is the one every dispatched session has:
// `posse/<session>`, so the ref is a file two directories deep and the
// directory to bind is `<common>/refs/heads/posse`.
//
// Three entries leave the set and one joins it, and each is asserted by
// NAME: a count would go green over a swap that dropped the right number of
// the wrong things.
func TestQABwrapWritableSwapsTheRefPairForItsDirectory(t *testing.T) {
	f := wgNewFixture(t)
	set, keep := bgSwap(t, f, f.tree)
	ref := absResolve(f.ref(f.branch))
	parent := filepath.Dir(ref) // <common>/refs/heads/posse

	for _, gone := range []string{ref, ref + ".lock", f.lock()} {
		if bgHas(set, gone) {
			t.Errorf("%s is still in the bwrap writable set — at this mechanism it is a FILE bind, which is the thing D20 removes:\n  %s",
				gone, strings.Join(set, "\n  "))
		}
	}
	if !bgHas(set, parent) {
		t.Errorf("the ref's parent directory %s is not in the set, so nothing grants the lock-then-rename a commit makes:\n  %s",
			parent, strings.Join(set, "\n  "))
	}
	// The three that were already directories stay exactly as they were:
	// this bead narrows the ref grant and touches nothing else.
	for _, stays := range []string{f.own, filepath.Join(f.common, "objects"), filepath.Join(f.common, "logs"), f.tree} {
		if !bgHas(set, stays) {
			t.Errorf("%s left the set — it is a directory at both mechanisms and D20 does not touch it:\n  %s",
				stays, strings.Join(set, "\n  "))
		}
	}
	if len(keep) != 1 || absResolve(keep[0]) != absResolve(filepath.Join(parent, ".posse-keep")) {
		t.Fatalf("keep = %v, want [%s]", keep, filepath.Join(parent, ".posse-keep"))
	}
	// And the sibling sessions' refs are what the widening costs, said here
	// so that the ADR's Consequences entry has a measurement under it and
	// not a reading: `posse/other-probe` is in the directory this grants.
	if !bgHas([]string{parent}, parent) || !underDir(parent, f.ref("posse/other-probe")) {
		t.Fatalf("the fixture has no peer ref inside %s, so the widening this grant carries is not measured here", parent)
	}
}

// A branch with more than one slash: the bind is the DEEPEST parent alone,
// and that is enough — `refs/heads/a` cannot be pruned while `refs/heads/a/b`
// sits in it, and git's two writes both land in `a/b`.
//
// The arm that would catch the tempting spelling, which is sessionRefDirs'
// own: that function grants EVERY ancestor, because a create-only grant has
// to name each mkdir. A mount list that copied it would bind `refs/heads/a`
// as well — a directory holding every `a/*` branch in the repo, granted for
// nothing.
func TestQABwrapWritableBindsTheDeepestRefParentOnly(t *testing.T) {
	f := wgNewFixture(t)
	mustGit(t, f.tree, "checkout", "-q", "-b", "a/b/c")
	set, keep := bgSwap(t, f, f.tree)
	heads := absResolve(filepath.Join(f.common, "refs", "heads"))

	want := filepath.Join(heads, "a", "b")
	if !bgHas(set, want) {
		t.Errorf("a/b/c did not grant %s:\n  %s", want, strings.Join(set, "\n  "))
	}
	for _, never := range []string{heads, filepath.Join(heads, "a")} {
		if bgHas(set, never) {
			t.Errorf("a/b/c also granted %s — every ancestor is sessionRefDirs' create-only shape and not a bind:\n  %s",
				never, strings.Join(set, "\n  "))
		}
	}
	if len(keep) != 1 || filepath.Dir(absResolve(keep[0])) != absResolve(want) {
		t.Fatalf("keep = %v, want one file in %s", keep, want)
	}
}

// A slashless branch in a linked worktree gets NO ref grant: the directory
// would be `refs/heads` itself. The leaf pair still leaves the set — it is a
// file bind this mechanism refuses whatever the branch is shaped like — so
// what this arm pins is that nothing takes its place.
//
// Written as an assertion of ABSENCE over a NAMED path rather than over a
// count, and paired with the positive half above: a swap that granted
// nothing at all would satisfy half of this file on its own.
func TestQABwrapWritableGrantsNoRefDirForATopLevelBranch(t *testing.T) {
	f := wgNewFixture(t)
	mustGit(t, f.tree, "checkout", "-q", "-b", "flat")
	set, keep := bgSwap(t, f, f.tree)
	heads := absResolve(filepath.Join(f.common, "refs", "heads"))

	if len(keep) != 0 {
		t.Errorf("a top-level branch asked for a keep file: %v", keep)
	}
	for _, never := range []string{heads, absResolve(f.ref("flat")), absResolve(f.ref("flat")) + ".lock", absResolve(f.ref("main"))} {
		if bgHas(set, never) {
			t.Errorf("a top-level branch granted %s — binding refs/heads is refs/heads/main with it (ranger-base-m2wf):\n  %s",
				never, strings.Join(set, "\n  "))
		}
	}
	if bgHas(set, f.lock()) {
		t.Errorf("packed-refs.lock survived the swap on a top-level branch:\n  %s", strings.Join(set, "\n  "))
	}
	// Still a worktree session, so everything that is not the ref is still
	// granted: this is a narrowing of one entry and not a refusal to render.
	if !bgHas(set, f.own) {
		t.Errorf("the session's own git dir left the set with the ref grant:\n  %s", strings.Join(set, "\n  "))
	}
}

// A detached HEAD has no branch, so there is no ref to grant and nothing to
// say about one — the same answer sessionGitGrants gives at L2, and the state
// the launcher puts a CAGED worktree session in on purpose
// (PrepareSessionHead). `packed-refs.lock` still goes: it is dropped for the
// mechanism's reason and not for the branch's.
func TestQABwrapWritableGrantsNoRefDirWhenHeadIsDetached(t *testing.T) {
	f := wgNewFixture(t)
	mustGit(t, f.tree, "checkout", "-q", "--detach")
	set, keep := bgSwap(t, f, f.tree)

	if len(keep) != 0 {
		t.Errorf("a detached HEAD asked for a keep file: %v", keep)
	}
	for _, w := range set {
		if underDir(filepath.Join(f.common, "refs"), w) {
			t.Errorf("a detached HEAD granted %s under the shared refs tree:\n  %s", w, strings.Join(set, "\n  "))
		}
	}
	if bgHas(set, f.lock()) {
		t.Errorf("packed-refs.lock survived the swap on a detached HEAD:\n  %s", strings.Join(set, "\n  "))
	}
	if !bgHas(set, f.own) {
		t.Errorf("the session's own git dir left the set:\n  %s", strings.Join(set, "\n  "))
	}
}

// A main checkout is the pass-through arm: `.git` is inside cwd and already a
// directory, there is no common dir of anyone else's, and the set comes back
// as it went in. The one thing that would be wrong is dropping something —
// `<repo>/.git/packed-refs.lock` is not in the set to begin with, and a swap
// that built its drop list from cwd rather than from a common dir would take
// the repo's whole `.git` with it.
func TestQABwrapWritableLeavesAMainCheckoutAlone(t *testing.T) {
	f := wgNewFixture(t)
	in := f.a.SeatbeltWritable(f.ag, f.repo, f.gates)
	set, keep := BwrapWritable(in, f.repo)

	if len(keep) != 0 {
		t.Errorf("a main checkout asked for a keep file: %v", keep)
	}
	if strings.Join(set, "\n") != strings.Join(in, "\n") {
		t.Fatalf("the set changed in a main checkout:\n  got  %s\n  want %s",
			strings.Join(set, "\n       "), strings.Join(in, "\n       "))
	}
	// cwd whole is what covers `.git` in a main checkout, and that is why
	// there is nothing to swap: sessionGitGrants answers nothing there, so
	// the grant that carries a commit is the session directory itself.
	if !bgHas(set, f.repo) {
		t.Fatalf("the checkout itself is not granted, so nothing covers its .git:\n  %s", strings.Join(set, "\n  "))
	}
}

// ─── the render over the swapped set ─────────────────────────────────────────

// The join, and the reason the two halves are one bead: the swapped set
// RENDERS and the shipped one does not. The control is the arm — a refusal
// with the leaf named is what BwrapArgs does with a writable FILE (B8), so
// a swap that stopped working would be loud here and only here.
func TestQABwrapArgsRendersTheWorktreeGrantOnlyAfterTheSwap(t *testing.T) {
	f := wgNewFixture(t)
	shipped := f.a.SeatbeltWritable(f.ag, f.tree, f.gates)
	set, keep := BwrapWritable(shipped, f.tree)
	parent := filepath.Dir(absResolve(f.ref(f.branch)))

	// The control: the L2 set, unswapped, is a wall this mechanism cannot
	// render — and it refuses rather than rendering one with a note beside
	// it.
	if _, _, err := BwrapArgs(f.tree, shipped, SeatbeltCarveOut{}); err == nil {
		t.Fatal("BwrapArgs rendered the L2 writable set — the ref leaf is a FILE, and binding one refuses the atomic replace git makes (B8), so this render must refuse")
	} else if !strings.Contains(err.Error(), absResolve(f.ref(f.branch))) {
		t.Errorf("the refusal does not name the ref leaf, so an operator cannot tell which entry to narrow: %v", err)
	}

	argv, prep, err := BwrapArgs(f.tree, set, SeatbeltCarveOut{}, keep...)
	if err != nil {
		t.Fatalf("the swapped set does not render: %v", err)
	}
	if !strings.Contains(bwJoin(argv), "--bind "+parent+" "+parent) {
		t.Errorf("no rw bind of the ref directory %s:\n%s", parent, bwJoin(argv))
	}
	// The keep file renders NO mount. It is the one input that does not, and
	// a bind of it would be a bind of a FILE — B8's refusal one level down,
	// and for a file git renames nothing onto.
	if strings.Contains(bwJoin(argv), absResolve(keep[0])) {
		t.Errorf("the keep file is mounted — it is a prep step and nothing else:\n%s", bwJoin(argv))
	}
	// The directory is already there — `worktree add` made it and put the
	// branch ref in it — and the keep file is not, which is the state every
	// repo on this fleet is in until a posse launch touches it. So the one
	// write asked for in the operator's git dir is that touch, and a render
	// that asked for a SECOND one there would be a write nobody asked this
	// bead for.
	want := "touch " + absResolve(keep[0])
	if p := bgPrepUnder(prep, f.common); len(p) != 1 || p[0] != want {
		t.Errorf("prep inside the shared git dir = %v, want [%s]", p, want)
	}
}

// The gc case, which is the whole reason the keep file exists: refs packed
// and the directory pruned (wgPack asserts that state was really reached),
// so the launcher has to make the directory AND leave the keep file in it —
// in that order, which is the prep list's promise.
//
// The order is the claim. A launcher cannot touch a file in a directory that
// does not exist yet, and nothing but this list's order says the mkdir comes
// first.
func TestQABwrapArgsAsksForTheRefDirThenItsKeepFile(t *testing.T) {
	f := wgNewFixture(t)
	wgPack(t, f) // refs packed, the loose dir pruned — one `git gc` away
	set, keep := bgSwap(t, f, f.tree)
	parent := filepath.Dir(absResolve(f.ref(f.branch)))

	_, prep, err := BwrapArgs(f.tree, set, SeatbeltCarveOut{}, keep...)
	if err != nil {
		t.Fatalf("the swapped set does not render over a packed repo: %v", err)
	}
	mkdir, touch := -1, -1
	for i, s := range prep {
		switch {
		case s.Kind == PrepMkdirAll && absResolve(s.Path) == absResolve(parent):
			mkdir = i
		case s.Kind == PrepTouch && absResolve(s.Path) == absResolve(keep[0]):
			touch = i
		}
	}
	if mkdir < 0 {
		t.Fatalf("no `mkdir -p %s` — a pruned ref directory is a bind source that is not there:\n  %v", parent, bwPrep(prep))
	}
	if touch < 0 {
		t.Fatalf("no `touch %s` — without it the next outside pack prunes the bound directory under the live seat:\n  %v", keep[0], bwPrep(prep))
	}
	if mkdir > touch {
		t.Fatalf("the keep file is touched at %d, before its directory is made at %d — the launcher runs this list in order:\n  %v",
			touch, mkdir, bwPrep(prep))
	}
	// And those two are the ONLY writes asked for in the shared git dir. The
	// prep list is writes made on the HOST, outside the wall, in the seat's
	// name — a third entry in the operator's repo would be one nobody asked
	// this bead for.
	if p := bgPrepUnder(prep, f.common); len(p) != 2 {
		t.Errorf("the render asked for %d writes inside the shared git dir, want the mkdir and the touch alone: %v", len(p), p)
	}
}

// A keep file inside no write grant refuses. The render's job is to keep a
// BOUND directory from being pruned, so a keep file over a directory nothing
// binds is a prep write on the host that buys nothing and hides that the
// grant is missing.
func TestQABwrapArgsRefusesAKeepFileOutsideEveryGrant(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj", "elsewhere"}, nil)

	_, _, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj")}, SeatbeltCarveOut{}, bwAt("elsewhere", ".posse-keep"))
	if err == nil {
		t.Fatal("a keep file outside every write grant rendered")
	}
	if !strings.Contains(err.Error(), bwAt("elsewhere", ".posse-keep")) {
		t.Errorf("the refusal does not name the keep file: %v", err)
	}
}

// And a DIRECTORY at the keep file's name refuses, in the tier's fail-closed
// direction: the launcher's touch would fail there, and an empty directory
// is exactly what `pack-refs --prune` removes — so the thing meant to hold
// the parent open would be pruned with it.
func TestQABwrapArgsRefusesADirectoryAtTheKeepFilesName(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj", "heads/posse/.posse-keep"}, nil)

	_, _, err := BwrapArgs(bwAt("proj"), []string{bwAt("proj"), bwAt("heads", "posse")}, SeatbeltCarveOut{},
		bwAt("heads", "posse", ".posse-keep"))
	if err == nil {
		t.Fatal("a directory at the keep file's name rendered")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("the refusal does not say what was wrong with it: %v", err)
	}
}

// ─── the parity row ──────────────────────────────────────────────────────────

// A top-level branch in a linked worktree: parity says why there is no ref
// grant, before the launch, naming the directory that would have to be bound
// and the ref that is in it. Without this line the only signal is a `cannot
// lock ref` at the session's first commit, with nothing to connect it to the
// wall.
func TestQABwrapParityNamesTheRefGrantItWillNotMake(t *testing.T) {
	f := wgNewFixture(t)
	mustGit(t, f.tree, "checkout", "-q", "-b", "flat")

	got := bwrapRefGrantDifference(WallBwrap, CageSeatbelt, true, f.tree)
	if got == "" {
		t.Fatal("parity says nothing about a top-level branch that gets no ref grant")
	}
	for _, want := range []string{"flat", "NO write grant", filepath.Join("refs", "heads", "main"), "ADR 0080 D20"} {
		if !strings.Contains(got, want) {
			t.Errorf("the line does not name %q:\n%s", want, got)
		}
	}
	// It must not read as a grant that WAS made: the widening arm's sentence
	// over this launch would tell an operator the opposite of the truth.
	if strings.Contains(got, "PEER") {
		t.Errorf("the no-grant line carries the widening arm's words:\n%s", got)
	}
}

// Where the grant IS made, the same row states the widening it carries (ADR
// 0080's Consequences): the directory holds every peer session's ref. An
// operator comparing this wall to darwin's leaf grant has one line that says
// so, and it is the line the design is checked against.
func TestQABwrapParityStatesTheRefDirWidening(t *testing.T) {
	f := wgNewFixture(t)

	got := bwrapRefGrantDifference(WallBwrap, CageSeatbelt, true, f.tree)
	if got == "" {
		t.Fatal("parity says nothing about the ref directory it grants")
	}
	// The directory it names is the one the render binds — read off the
	// render, so the line and the argv cannot disagree.
	_, keep := bgSwap(t, f, f.tree)
	parent := filepath.Dir(absResolve(keep[0]))
	if !strings.Contains(got, AbbrevHome(parent)) {
		t.Errorf("the line does not name the directory the render binds (%s):\n%s", parent, got)
	}
	for _, want := range []string{"PEER", filepath.Join("refs", "heads", "main"), "ADR 0080 D20"} {
		if !strings.Contains(got, want) {
			t.Errorf("the line does not name %q:\n%s", want, got)
		}
	}
}

// The three conditions that make this row silent, each for its own reason,
// and all three are what keep it from being a claim about a wall nobody has.
func TestQABwrapParityIsSilentWhereTheGrantIsNotThisMechanisms(t *testing.T) {
	f := wgNewFixture(t)

	for _, c := range []struct {
		name            string
		mech, cage, dir string
		available       bool
	}{
		{"the darwin mechanism renders the leaf pair and this says nothing about it", WallSandboxExec, CageSeatbelt, f.tree, true},
		{"another tier's wall is not rendered from this list at all", WallBwrap, CageContainer, f.tree, true},
		{"a mechanism this host cannot put up grants nothing", WallBwrap, CageSeatbelt, f.tree, false},
		{"a main checkout has no common dir of anyone else's", WallBwrap, CageSeatbelt, f.repo, true},
		{"no launch dir, so there is no repo to ask about", WallBwrap, CageSeatbelt, "", true},
	} {
		if got := bwrapRefGrantDifference(c.mech, c.cage, c.available, c.dir); got != "" {
			t.Errorf("%s: parity printed\n%s", c.name, got)
		}
	}
	// A detached HEAD is the fifth, and it is silent because there is no
	// DIFFERENCE: seatbelt grants no ref there either, so there is nothing
	// for a declared difference to declare.
	mustGit(t, f.tree, "checkout", "-q", "--detach")
	if got := bwrapRefGrantDifference(WallBwrap, CageSeatbelt, true, f.tree); got != "" {
		t.Errorf("a detached HEAD printed a ref-grant difference:\n%s", got)
	}
}

// The row is a DECLARED DIFFERENCE and never degradation (ADR 0017 §2,
// ranger-base-d17a): a narrower wall than the tier's other mechanism is not
// a wall posse asked for and did not get, and putting it in Degraded would
// refuse dispatch for a shape the design chose.
//
// Driven through the App method, because what a pin of the pure function
// cannot catch is the one line in checkParityIn appending to the wrong list.
func TestQABwrapRefGrantIsADeclaredDifferenceNotDegradation(t *testing.T) {
	f := wgNewFixture(t)
	p := Parity{Cage: CageSeatbelt, Realized: map[string]RealizedGate{}}
	before := len(p.Degraded)

	f.a.applyBwrapRefGrant(&p, f.tree)
	if len(p.Degraded) != before {
		t.Fatalf("the ref grant degraded the launch: %v", p.Degraded)
	}
	// On darwin the mechanism is sandbox-exec, so the row is silent and the
	// assertion above is vacuous on its own. The pure function is where the
	// Linux answer is read (the arms above); what this adds is that the
	// shipped call site cannot route it into Degraded, measured over the
	// same Parity by substitution.
	if WallMechanism() == WallBwrap && !AvailableCages[CageSeatbelt] {
		t.Skip("bwrap is this host's mechanism but the tier is unavailable — the row is silent here by design")
	}
	if WallMechanism() == WallBwrap && len(p.DeclaredDifference) == 0 {
		t.Fatal("on a bwrap host with the tier available the row said nothing")
	}
}

// ─── the keep file against a real `git pack-refs --all --prune` ──────────────

// What the keep file buys, MEASURED rather than assumed, and measurable here
// because this half of it is git's behaviour and not the kernel's: a loose
// ref directory holding a dot-named file survives the prune that empties it,
// and git reads nothing in it as a ref.
//
// ADR 0080 D20 lists this as ASSUMED, read by probe T4b-T4d on the Linux
// host. T4's remaining Linux-only question is narrower than the probe's row
// suggests, and it is the only part a darwin box cannot settle: whether the
// prune can take a directory that is a MOUNTPOINT in a live seat's namespace
// at all (bwrapgitrefslive_test.go's control carries that one).
//
// MEASURED 2026-11-08, darwin 25.6.0, the box's git: with the keep file the
// directory is there after the pack and the next commit lands; with it
// removed and a loose ref back in the directory, the same pack removes it.
func TestQABwrapRefKeepFileSurvivesAnOutsidePackRefs(t *testing.T) {
	f := wgNewFixture(t)
	_, keep := bgSwap(t, f, f.tree)
	dir := filepath.Dir(absResolve(keep[0]))
	fh, err := os.OpenFile(absResolve(keep[0]), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	fh.Close()

	mustGit(t, f.repo, "pack-refs", "--all", "--prune")
	if !wgExists(t, dir) {
		t.Fatalf("the prune removed %s with the keep file in it — the file does not do what D20 asks of it", dir)
	}
	// T4c and T4d: the file is in a loose ref directory, so the question is
	// whether git reads it as a ref or as damage. Neither, by the dot.
	if out := mustGit(t, f.repo, "for-each-ref"); strings.Contains(out, bwrapRefKeepName) {
		t.Errorf("git lists the keep file as a ref:\n%s", out)
	}
	if out, _ := git(f.repo, "fsck"); strings.Contains(out, bwrapRefKeepName) {
		t.Errorf("git fsck flags the keep file:\n%s", out)
	}
	// And the commit after the pack lands, which is T4's git half: the loose
	// ref is gone, the directory is not, so the lock-then-rename has
	// somewhere to go. Unwalled — the wall is the live pin's.
	if _, err := git(f.tree, "-c", "user.name=p", "-c", "user.email=p@example.com", "commit", "-q", "--allow-empty", "-m", "post-pack"); err != nil {
		t.Fatalf("the commit after an outside pack failed with the directory in place: %v", err)
	}

	// The control, and it is what makes every line above load-bearing. A
	// loose ref goes back in the directory first: pack-refs prunes only a
	// directory it EMPTIED, so a second pack with nothing loose left prunes
	// nothing and a control without this line reads "survived" over a pack
	// that never tried (measured — it is how this test was first written).
	if err := os.Remove(absResolve(keep[0])); err != nil {
		t.Fatal(err)
	}
	mustGit(t, f.repo, "update-ref", "refs/heads/"+filepath.Base(dir)+"/control", "HEAD")
	mustGit(t, f.repo, "pack-refs", "--all", "--prune")
	if wgExists(t, dir) {
		t.Fatalf("the prune left %s with no keep file in it — the keep file is not what keeps it, and D20's reason for creating it needs revisiting instead of this test", dir)
	}
}
