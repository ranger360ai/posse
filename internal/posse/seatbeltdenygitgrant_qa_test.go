//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-8t6j1 — ADR 0086 D3: a PID that denies the whole
// tree gets no write grant into any git dir of its session.
//
// Claim: for such a PID, at BOTH mechanisms of the seatbelt tier, the
// session's repo contributes exactly one write grant — `<cwd>/.beads`, so bd
// can still claim, comment and close — and nothing else. Not `<cwd>/.git`,
// not the per-worktree git dir, not `objects`, not `logs`, not the ref pair,
// not `packed-refs.lock`, and not the create-only ref directory either.
//
// WHY THESE ARMS AND NOT THE ONES NEXT DOOR. D6 names the lesson R0 taught
// (and ppnds finding 3 before it): every bwrap fixture with `deniesFiles`
// was a plain directory or a main checkout, so the one shape a dispatched
// session actually has — a LINKED WORKTREE, where `<cwd>/.git` is a file —
// was pinned by nothing. The grant was dead on darwin, because the
// carve-out denies the same path and SBPL's last match wins, and it refused
// the whole bwrap render on Linux. A wall can be wrong in both directions at
// once and look green from either host.
//
// So every arm here drives the REAL fixtures, both of them: wgNewFixture's
// linked worktree, and the operator's checkout it was made from. And each
// asserts the KIND of every grant (wgKinds) rather than only where it lies:
// "inside a git dir" is the question D3 answers, and "is it a file" is the
// question that decides whether the render refuses at all.
//
// THE CONTROL IS THE SAME CALL FOR A PID THAT DENIES NOTHING. Every arm
// below would also pass over a producer that returned an empty set for
// everyone, which is the failure that would take ranger-base-uuze's lost
// commit and ranger-base-m2wf's narrowing with it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wgDeny is the fixture's PID with a whole-tree write deny — the rule a
// findings-only persona carries, and the one ADR 0086's parity reading found
// on exactly one of its host's PIDs. A COPY, so the arm that needs the open
// PID beside it can have both from one fixture.
//
// `Edit` alone and not the triple: deniesFileWrite is an any-of union
// (cageinner.go), and a pin that spelled all three could not tell a wall
// that reads the union from one that happens to match on `Write`.
func wgDeny(f wgFixture) *AgentFile {
	ag := *f.ag
	ag.Deny = []string{"Edit"}
	return &ag
}

// wgDenyFixture is wgNewFixture's repo, worktree and sibling under a root
// this file chooses. wgNewFixture takes sbRoot's, which SKIPS wherever
// TMPDIR resolves under /tmp — true on every Linux box, which is the host
// R0 was measured on and the one these arms most need to run on. Nothing
// here runs a command under a wall, so the blanket temp grant that makes
// sbRoot skip is handled where it belongs instead: wgRepoGrants takes it out
// of the set by its own spelling.
func wgDenyFixture(t *testing.T) wgFixture {
	t.Helper()
	wtqaHome(t) // the home cache grants then name a fixture home, not the operator's
	return wgFixtureIn(t, gitTempDir(t))
}

// wgRepoGrants narrows a writable set to the SESSION'S OWN REPOSITORY: its
// worktree, and the checkout whose `.git` holds the per-worktree and common
// git dirs. Everything else SeatbeltWritable grants belongs to the launch
// and not to this question — TMPDIR and `/tmp` (which this fixture sits
// INSIDE, the whole reason sbRoot skips), the home caches, the gates dir,
// the memory dir, posse's state dir.
//
// BY LOCATION AND NEVER BY SUBTREE SUBTRACTION, which is what makes it exact
// rather than convenient: it keeps every entry inside the repo and drops
// every entry outside it, so a grant on `<cwd>/.git` or on the shared
// `objects` is still in the narrowed set and still reds the arms below. The
// blanket grant is dropped because `/tmp` is not inside this repo, not
// because anything under /tmp is forgiven.
func wgRepoGrants(set []string, repo, tree string) []string {
	var out []string
	for _, p := range set {
		if underDir(repo, p) || underDir(tree, p) {
			out = append(out, p)
		}
	}
	return out
}

// wgGrants is that narrowing over the shipped producer for one PID and one
// launch dir, with the one check that keeps every arm below from passing
// over an empty list.
func wgGrants(t *testing.T, f wgFixture, ag *AgentFile, cwd string) []string {
	t.Helper()
	g := wgRepoGrants(f.a.SeatbeltWritable(ag, cwd, f.gates), f.repo, f.tree)
	if len(g) == 0 {
		t.Fatalf("the writable set for %s in %s reaches nothing in this repo, so every assertion over it is vacuous", ag.Name, cwd)
	}
	return g
}

// wgKinds is every entry of a writable set beside what the box says it IS:
// `d` a directory, `f` a file, `-` not there. The render's own shapes
// (BwrapArgs step 2) are exactly these three — a directory is bound, an
// absent path is prepped and then bound, and a FILE refuses the render — so
// a pin that reads containment alone is blind to the one of the three that
// decides whether a seat launches.
func wgKinds(set []string) []string {
	out := make([]string, 0, len(set))
	for _, p := range set {
		kind := "-"
		if st, err := os.Stat(p); err == nil {
			if kind = "f"; st.IsDir() {
				kind = "d"
			}
		}
		out = append(out, kind+" "+p)
	}
	return out
}

// wgUnder is the entries of a set that lie inside dir, resolved both sides.
func wgUnder(set []string, dir string) []string {
	var out []string
	for _, p := range set {
		if underDir(dir, p) {
			out = append(out, p)
		}
	}
	return out
}

// ─── the writable set, in the shape a dispatched session has ─────────────────

// A linked worktree: the session's own tree, its per-worktree git dir inside
// the operator's `.git`, and the shared common dir. The PID's repo
// contributes `<cwd>/.beads` and nothing else.
func TestQAWholeTreeDenyGetsNoGitGrantInALinkedWorktree(t *testing.T) {
	f := wgDenyFixture(t)
	deny := wgGrants(t, f, wgDeny(f), f.tree)
	open := wgGrants(t, f, f.ag, f.tree)

	// The control first: the open PID really does get the git grants, so a
	// producer that answered nothing for everybody cannot pass this file.
	for _, want := range []string{f.own, filepath.Join(f.common, "objects"), filepath.Join(f.common, "logs"), f.ref(f.branch)} {
		if !writeGranted(open, want) {
			t.Fatalf("the control PID is granted no write on %s, so this fixture cannot measure D3 taking it away:\n  %s",
				want, strings.Join(wgKinds(open), "\n  "))
		}
	}

	// ADR 0086 D3: nothing in either git dir, and `<cwd>/.git` named by
	// nothing — in a worktree that is the FILE whose grant refused the whole
	// bwrap render (R0).
	for _, gone := range []string{
		filepath.Join(f.tree, ".git"),
		f.own,
		filepath.Join(f.common, "objects"),
		filepath.Join(f.common, "logs"),
		f.lock(),
		f.ref(f.branch),
		f.ref(f.branch) + ".lock",
		filepath.Join(f.common, "refs", "heads", "main"),
		f.other,
	} {
		if writeGranted(deny, gone) {
			t.Errorf("a PID that denies the whole tree is granted a write that reaches %s (ADR 0086 D3):\n  %s",
				gone, strings.Join(wgKinds(deny), "\n  "))
		}
	}

	// What it keeps, and the whole of it: the bead store, so claim, comment
	// and close still work (SeatbeltWritable's own first arm).
	beads := filepath.Join(f.tree, ".beads")
	if !writeGranted(deny, beads) {
		t.Errorf("the bead store is not granted, so the seat cannot claim or close its own bead:\n  %s", strings.Join(wgKinds(deny), "\n  "))
	}
	for _, p := range wgUnder(deny, f.tree) {
		if !underDir(beads, p) {
			t.Errorf("the session's tree contributes %s beside %s — D3 leaves the repo exactly one grant:\n  %s",
				p, beads, strings.Join(wgKinds(deny), "\n  "))
		}
	}

	// The KIND of every grant, which is R0 generalized: in a linked worktree
	// the path that refused the bwrap render was a FILE in the set, and no
	// path in this PID's set is one. (The open PID's set still holds three,
	// which is what BwrapWritable swaps away at the other mechanism — so
	// this assertion belongs to this arm alone and is not a property of the
	// producer.)
	for _, k := range wgKinds(deny) {
		if strings.HasPrefix(k, "f ") {
			t.Errorf("the set names a FILE, which bwrap refuses to bind (BwrapArgs shape 1, probe B8) and which is how R0 refused a whole-tree-deny PID's every worktree launch: %s", k)
		}
	}
}

// The operator's own checkout, where `.git` is a DIRECTORY inside cwd. The
// grant that reaches `refs/heads/main` here is `cwd` itself, which this PID
// is not given — W15's STATIC half, read off the set rather than argued.
func TestQAWholeTreeDenyGetsNoGitGrantInAMainCheckout(t *testing.T) {
	f := wgDenyFixture(t)
	deny := wgGrants(t, f, wgDeny(f), f.repo)
	open := wgGrants(t, f, f.ag, f.repo)

	git := filepath.Join(f.repo, ".git")
	if !writeGranted(open, filepath.Join(git, "refs", "heads", "main")) {
		t.Fatalf("the control PID cannot write main's ref in a main checkout, so this arm measures nothing:\n  %s",
			strings.Join(wgKinds(open), "\n  "))
	}
	for _, gone := range []string{
		git,
		filepath.Join(git, "refs", "heads", "main"),
		filepath.Join(git, "objects"),
		filepath.Join(git, "hooks", "pre-push"),
		filepath.Join(git, "config"),
		f.repo,
	} {
		if writeGranted(deny, gone) {
			t.Errorf("a PID that denies the whole tree is granted a write that reaches %s in a main checkout (ADR 0086 D3, W15):\n  %s",
				gone, strings.Join(wgKinds(deny), "\n  "))
		}
	}
	if !writeGranted(deny, filepath.Join(f.repo, ".beads")) {
		t.Errorf("the bead store is not granted in a main checkout:\n  %s", strings.Join(wgKinds(deny), "\n  "))
	}
}

// ─── the two mechanisms render the same rule ─────────────────────────────────

// R0, by render. The bwrap argv for this PID in a linked worktree is the
// thing that refused — and the control is the same render with the grant D3
// removed put back by hand, which must still refuse and must still name the
// file. Without that control a render that succeeded for some other reason
// would read as this bead working.
func TestQAWholeTreeDenyRendersAtTheBwrapMechanism(t *testing.T) {
	f := wgDenyFixture(t)
	deny := wgGrants(t, f, wgDeny(f), f.tree)

	set, keep := BwrapWritable(wgDeny(f), deny, f.tree)
	if len(keep) != 0 {
		t.Errorf("the render asked for a ref keep file for a PID granted no ref directory: %v", keep)
	}
	if strings.Join(set, "\n") != strings.Join(deny, "\n") {
		t.Errorf("the swap changed a set that holds no git grant to swap:\n  got  %s\n  want %s",
			strings.Join(set, "\n       "), strings.Join(deny, "\n       "))
	}
	for _, p := range set {
		if underDir(f.common, p) || underDir(f.own, p) {
			t.Errorf("the bwrap set reaches a git dir at %s — the ref directory is this mechanism's own addition, derived from cwd and the branch, so it is added back here or nowhere:\n  %s",
				p, strings.Join(wgKinds(set), "\n  "))
		}
	}
	if _, _, err := BwrapArgs(f.tree, set, SeatbeltCarveOut{}); err != nil {
		t.Fatalf("the render refuses a whole-tree-deny PID in a linked worktree, which is R0 and is what ADR 0086 D3 exists to fix: %v", err)
	}

	// The control: `<cwd>/.git` back in the set is R0 exactly.
	back := append(append([]string{}, set...), absResolve(filepath.Join(f.tree, ".git")))
	if _, _, err := BwrapArgs(f.tree, back, SeatbeltCarveOut{}); err == nil {
		t.Fatal("BwrapArgs rendered a set naming <cwd>/.git in a linked worktree — that path is a FILE and binding one refuses the atomic replace its writer makes (B8), so R0 was not the refusal this bead removed")
	} else if !strings.Contains(err.Error(), absResolve(filepath.Join(f.tree, ".git"))) {
		t.Errorf("the control's refusal does not name the file, so R0's message is not what this arm measured: %v", err)
	}
}

// The same rule at the darwin mechanism, read off the profile text: no
// create-only block at all, because the only thing that block ever carries
// is the directory git makes for a ref this PID is not granted
// (sessionRefDirsFor). The control is the open PID, whose profile has one.
func TestQAWholeTreeDenyRendersNoCreateOnlyRefDirInTheProfile(t *testing.T) {
	f := wgDenyFixture(t)
	ag := wgDeny(f)

	if got := sessionRefDirsFor(ag, f.tree); len(got) != 0 {
		t.Errorf("the create-only half still answers for a PID granted no ref: %v", got)
	}
	if got := sessionRefDirsFor(f.ag, f.tree); len(got) == 0 {
		t.Fatal("the control PID gets no create-only ref directory either, so this arm measures nothing — ranger-base-uuze's lost commit is what that costs")
	}

	prof := f.denyProfile(t, ag)
	if strings.Contains(prof, "file-write-create") {
		t.Errorf("the profile carries a create-only grant for a PID granted no ref — a grant for a write that cannot complete is what an operator reads off `posse gates` as a capability:\n%s", prof)
	}
	if ctrl := f.denyProfile(t, f.ag); !strings.Contains(ctrl, "file-write-create") {
		t.Fatal("the control profile has no create-only block, so the assertion above holds for the wrong reason")
	}
	// The allow side of the profile reaches neither git dir. The carve-out
	// still NAMES paths in the common dir — the hooks dir and the config
	// chain are denied there — so this is asserted over the grants and not
	// over the text as a whole.
	for _, p := range wgGrants(t, f, ag, f.tree) {
		if underDir(f.common, p) || underDir(f.own, p) {
			t.Errorf("the darwin profile grants %s, so D3 narrowed one mechanism and not the other", p)
		}
	}
}

// denyProfile renders the profile for one PID from the SHIPPED composition
// (RenderSeatbelt's own line), so a door that forgot the PID-aware half
// cannot pass by agreeing with a hand-assembled call.
func (f wgFixture) denyProfile(t *testing.T, ag *AgentFile) string {
	t.Helper()
	w := wgGrants(t, f, ag, f.tree)
	carve := f.a.SeatbeltCarveOut(ag, f.tree, f.gates, w)
	return SeatbeltProfile(ag.Name, w, SeatbeltSiblings(nil, carve), carve, sessionRefDirsFor(ag, f.tree)...)
}

// ─── the gate that asked for the write D3 removed ────────────────────────────

// The record stage's targets (ADR 0013 §4) are the store of record and the
// git dirs a `bd sync` commit of the JSONL locks — and that second half is a
// write this PID no longer has anywhere in its OWN repo. Probing it refuses
// every such launch for a capability the PID's own rule forbids, which is a
// gate nothing can satisfy rather than a wall.
//
// MEASURED under the real bwrap wall before this narrowing (D1 host,
// 2026-11-10, a main checkout OUTSIDE /tmp, because the blanket temp grant
// makes a fixture inside it pass for the wrong reason): the row read
// `<cwd>/.git … Read-only file system — a caged session cannot bd sync, bd
// export or commit the JSONL there`, and `--allow-degraded` would have been
// the only way to launch. The open PID read OK then and reads OK now.
//
// Both halves are asserted here, because the narrowing is NOT "a deny PID
// gets fewer targets": where a `.beads/redirect` puts the store in another
// repo, D3 KEEPS those git dirs granted (bd commits there) and they stay
// targets.
func TestQAWholeTreeDenyRecordTargetsSkipOnlyItsOwnGitDir(t *testing.T) {
	f := wgDenyFixture(t)
	deny, open := wgDeny(f), f.ag

	// A LOCAL store: the git dirs here are the session's own.
	sbMkdir(t, filepath.Join(f.repo, beadsDirName))
	local := beadsHome(f.repo)
	want := []string{absResolve(local)}
	if got := wgKinds(recordTargets(deny, f.repo, local)); strings.Join(got, "\n") != "d "+want[0] {
		t.Errorf("a whole-tree-deny PID is still asked to write a git dir of its own repo (ADR 0086 D3):\n  %s", strings.Join(got, "\n  "))
	}
	ctrl := recordTargets(open, f.repo, local)
	if !bgHas(ctrl, filepath.Join(f.repo, ".git")) {
		t.Fatalf("the control PID is not asked for its git dir either, so the arm above measures nothing:\n  %s", strings.Join(wgKinds(ctrl), "\n  "))
	}

	// A REDIRECT: the store of record is another repo, D3 keeps its git
	// dirs, and they stay targets for both PIDs.
	store := sbMkdir(t, filepath.Join(filepath.Dir(f.repo), "store"))
	sbGitInit(t, store)
	sbMkdir(t, filepath.Join(store, beadsDirName))
	sbMkdir(t, filepath.Join(f.tree, beadsDirName))
	sbWrite(t, filepath.Join(f.tree, beadsDirName, beadsRedirect), filepath.Join(store, beadsDirName)+"\n")
	redirected := beadsHome(f.tree)
	if redirected != absResolve(filepath.Join(store, beadsDirName)) {
		t.Fatalf("the redirect does not resolve, so this half measures nothing: beadsHome = %s", redirected)
	}
	for _, p := range []string{redirected, filepath.Join(store, ".git")} {
		if !bgHas(recordTargets(deny, f.tree, redirected), p) {
			t.Errorf("the store of record in another repo lost %s as a target — D3 keeps that grant because bd commits there:\n  %s",
				p, strings.Join(wgKinds(recordTargets(deny, f.tree, redirected)), "\n  "))
		}
	}
}

// ─── parity says nothing a wall does not do ──────────────────────────────────

// The ref-grant row is bwrap's statement about a grant darwin spells
// differently. For this PID the two mechanisms agree — neither grants a ref
// — so a line printed on Linux alone would read as a capability of this
// mechanism, which is the reverse of what D3 did. Silent, like the detached
// HEAD beside it, and the control is the same row for the open PID.
func TestQAWholeTreeDenyParitySaysNothingAboutARefGrantItHasNot(t *testing.T) {
	f := wgDenyFixture(t)

	if got := bwrapRefGrantDifference(wgDeny(f), WallBwrap, CageSeatbelt, true, f.tree); got != "" {
		t.Errorf("parity declares a ref grant for a PID that has none (ADR 0086 D3):\n%s", got)
	}
	if got := bwrapRefGrantDifference(f.ag, WallBwrap, CageSeatbelt, true, f.tree); got == "" {
		t.Fatal("the row is silent for the open PID too, so the assertion above holds for the wrong reason")
	}
}
