//go:build posse_arm2

package posse

// ranger-base-3s2ig: a landing never puts back a commit a park holds.
//
// THE DEFECT, as ranger-base-zlvyk measured it in this repo on 2026-11-22
// (read-only — `git merge-tree --write-tree` writes a tree and moves no ref).
// A push hold parks the held commits — a ref of their own under
// refs/heads/parked/, and the base reset out from under them — and leaves
// every session branch already cut from that tip pointing at them. The
// fast-forward of such a branch REFUSES, because the base has moved; the
// replay behind it rebases the branch onto the reset base and fast-forwards
// that, and the held commits land under new shas. The replay was measured
// clean on the live carrier, so the landing would have gone through.
//
// No guard already in that chain can answer for it. The held commits reach the
// base under no sha at all, so equivalentOnBase pairs nothing — and a
// sanitized re-commit of the same work pairs nothing either: on the live pair
// it was ONE commit against the held TWO, with an author date 38 minutes
// later, so neither patch-id nor the (author email, author date, subject)
// triple matched.
//
// THE PINS HERE ARE BOTH CARRIERS, which is the half the bead's title names
// and the half it does not: the replay is the arm that was measured landing
// held work, and the fast-forward is the SAME defect in a seat whose base has
// not moved since the park — it lands the held commits by sha with no rebase
// involved. Each arm carries its own wrong arm: lift the park and the same
// fixture lands, which is what says the refusal is the guard's and not the
// fixture's.
//
// The park is spelled as the hand procedure spells it (parkRefPrefix,
// park.go), and these fixtures write it with `git update-ref` — no checkout,
// no branch command, nothing a seat would race.

import (
	"strings"
	"testing"
)

// parkAt puts a park at `at` and returns its ref. `update-ref` and not `git
// branch`, so the fixture writes exactly what the field has — one ref under
// the park prefix reaching commits the base does not — and touches no
// checkout.
func parkAt(t *testing.T, repo, name, at string) string {
	t.Helper()
	ref := parkRefPrefix + name
	mustGit(t, repo, "update-ref", ref, at)
	return ref
}

// parkedCarrierTree is the field's shape in miniature: a session branch with
// one commit of its own, and a park holding that commit — a seat cut from a
// tip a push hold later took off the base. The base has NOT moved, so the
// landing arm it reaches is the fast-forward.
func parkedCarrierTree(t *testing.T) (*App, string, *SessionTree, string, string) {
	t.Helper()
	a, repo, tr := wtTreeWithWork(t)
	tip := mustGit(t, repo, "rev-parse", tr.Branch)
	return a, repo, tr, parkAt(t, repo, "qa-hold-"+abbrevSHA(tip), tip), tip
}

// THE FAST-FORWARD CARRIER: the base has not moved since the park, so the ff
// would take the held commit by sha.
//
// MUTATION RUN: removing the parkedLanding call from MergeSessionWork reds
// this test — main goes to the held commit.
func TestAParkedCommitIsNotFastForwardedBackOntoTheBase(t *testing.T) {
	t.Parallel()
	a, repo, tr, ref, tip := parkedCarrierTree(t)
	before := mustGit(t, repo, "rev-parse", tr.Base)
	if !reaches(repo, tr.Branch, before) {
		t.Fatalf("fixture: %s does not reach %s, so this is the replay's arm and not the fast-forward's", tr.Branch, before)
	}

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatalf("a commit the park %s holds was landed: %+v", parkName(ref), o)
	}
	if got := mustGit(t, repo, "rev-parse", tr.Base); got != before {
		t.Fatalf("%s moved to %s over a refused landing", tr.Base, got[:12])
	}
	for _, want := range []string{parkName(ref), abbrevSHA(tip)} {
		if !strings.Contains(o.Reason, want) {
			t.Errorf("the refusal does not name %q — the park and the commit are the two operands a seat cannot discover from the branch:\n%s", want, o.Reason)
		}
	}

	// THE WRONG ARM. Lift the park and nothing else: the same repo, the same
	// branch and the same call land it. Without this the test above is payable
	// by a fixture that was never landable.
	mustGit(t, repo, "update-ref", "-d", ref)
	o2, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o2.Merged || mustGit(t, repo, "rev-parse", tr.Base) != tip {
		t.Fatalf("with the park lifted the same fixture did not land (%+v), so the refusal above measured the fixture and not the guard", o2)
	}
}

// THE REPLAY CARRIER, which is the arm the bead's title names: the base moved,
// so the fast-forward refuses and the replay is what would land the held
// commit. Both fixture controls are asserted, because without them a refusal
// here is payable by the conflict arm — the replay must be CLEAN for this to
// be the park's refusal.
//
// MUTATION RUN: removing the parkedLanding call from MergeSessionWork reds
// this test — the branch is rebased and main takes the held commit's replay.
func TestAParkedCommitIsNotReplayedBackOntoTheBase(t *testing.T) {
	t.Parallel()
	a, repo, tr := wtTreeWithWork(t)
	tip := mustGit(t, repo, "rev-parse", tr.Branch)
	commitIn(t, repo, "other.txt", "meanwhile\n", "main moved")
	before := mustGit(t, repo, "rev-parse", tr.Base)
	if reaches(repo, tr.Branch, before) {
		t.Fatalf("fixture: %s still reaches %s, so the fast-forward would land this and the replay is not the arm under test", tr.Branch, before)
	}
	if !mergesCleanly(repo, before, tip) {
		t.Fatal("fixture: the replay would not be clean, so a refusal below could be the conflict arm's")
	}
	ref := parkAt(t, repo, "qa-hold-replay", tip)

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatalf("a commit the park %s holds was replayed onto %s: %+v", parkName(ref), tr.Base, o)
	}
	if got := mustGit(t, repo, "rev-parse", tr.Base); got != before {
		t.Fatalf("%s moved to %s over a refused landing", tr.Base, got[:12])
	}
	// And the replay never RAN: the guard is above it, so the branch is where
	// the seat left it and a reader of the bead is not reading a rewritten
	// history.
	if got := mustGit(t, repo, "rev-parse", tr.Branch); got != tip {
		t.Errorf("%s was rewritten to %s by a replay that was then refused — the guard is below the rebase", tr.Branch, got[:12])
	}
	if o.Rebased {
		t.Errorf("the outcome reports a replay that must not have happened: %+v", o)
	}

	// THE WRONG ARM: with the park lifted the replay lands it, which is the
	// escape this guard closes.
	mustGit(t, repo, "update-ref", "-d", ref)
	o2, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o2.Merged || !o2.Rebased {
		t.Fatalf("with the park lifted the same fixture did not replay and land (%+v), so the refusal above measured the fixture and not the guard", o2)
	}
}

// A PARK THAT HOLDS NOTHING OFF THE BASE IS NOT AN OBSTACLE, and this is the
// arm that keeps the guard from being a box-wide stop sign: parks are
// long-lived refs, several stand at once (7 in this repo on 2026-11-22), and a
// guard that refused a landing because A park exists would block every seat on
// the box for as long as any hold is open.
//
// Two ways a park holds nothing of this landing's, and both are ordinary:
// the park's commits are already on the base (it was lifted by landing them),
// and the park is over somebody else's work entirely.
func TestAParkHoldingNothingThisLandingWouldPutBackDoesNotBlockIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		park func(t *testing.T, repo string, tr *SessionTree) string
	}{
		{
			// Lifted: the park is at the base's own tip, so `rev-list <park>
			// --not <base>` is empty.
			name: "the base already reaches everything the park holds",
			park: func(t *testing.T, repo string, tr *SessionTree) string {
				return parkAt(t, repo, "qa-lifted", mustGit(t, repo, "rev-parse", tr.Base))
			},
		},
		{
			// Somebody else's: a commit off the base that this branch does
			// not reach — which is what every OTHER seat's park looks like
			// from here. Built with `commit-tree` so the repo's own checkout
			// is not moved to make it.
			name: "the park is over another branch's work",
			park: func(t *testing.T, repo string, tr *SessionTree) string {
				base := mustGit(t, repo, "rev-parse", tr.Base)
				tree := mustGit(t, repo, "rev-parse", base+"^{tree}")
				held := mustGit(t, repo, "commit-tree", tree, "-p", base, "-m", "another seat's held work")
				if reaches(repo, tr.Branch, held) {
					t.Fatalf("fixture: %s reaches %s, so this is not another branch's commit", tr.Branch, held)
				}
				return parkAt(t, repo, "qa-elsewhere", held)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, repo, tr := wtTreeWithWork(t)
			tip := mustGit(t, repo, "rev-parse", tr.Branch)
			if ref := tc.park(t, repo, tr); refSHA(repo, ref) == "" {
				t.Fatalf("fixture: %s does not resolve, so no park was written and this arm proves nothing", ref)
			}
			o, err := MergeSessionWork(a, tr)
			if err != nil {
				t.Fatal(err)
			}
			if !o.Merged {
				t.Fatalf("a park that holds nothing of this landing's refused it: %s", o.Reason)
			}
			if got := mustGit(t, repo, "rev-parse", tr.Base); got != tip {
				t.Fatalf("%s is at %s, not the landed tip %s", tr.Base, got[:12], tip[:12])
			}
		})
	}
}

// THE SWEEP'S HALF (landsweep.go, blockStillStands): while a park holds a
// commit the landing would put back, the standing block is honoured without
// re-entering MergeSessionWork — and the moment the park is lifted the same
// operands un-skip, which is the "every reason has a pass that can see it stop
// being true" rule this mechanism is held to (mergeblocked_test.go's header).
//
// MUTATION RUN: removing the parkedInRange arm from blockStillStands reds the
// first half — mergesCleanly answers clean over a carrier, so the pass probes
// for real on every pass forever.
func TestAParkKeepsAStandingBlockStandingUntilItIsLifted(t *testing.T) {
	t.Parallel()
	_, repo, tr := wtTreeWithWork(t)
	tip := mustGit(t, repo, "rev-parse", tr.Branch)
	commitIn(t, repo, "other.txt", "meanwhile\n", "main moved")
	ref := parkAt(t, repo, "qa-hold-sweep", tip)
	head, ok := workHead(tr)
	if !ok {
		t.Fatal("fixture: the tree has no head")
	}
	prior := priorBlock{ID: "qa-block-1", Open: true, Why: parkLandRefusal(tr, ref, tip)}

	if _, stands := blockStillStands(tr, head, prior); !stands {
		t.Error("the block was un-skipped while a park still held the landing's commit — the pass re-enters MergeSessionWork to be refused again, every pass")
	}

	// THE WRONG ARM, and it is what makes the arm above a reading of the park
	// rather than of the fixture: lift the park and the very next call says
	// "this could land now — probe for real".
	mustGit(t, repo, "update-ref", "-d", ref)
	if _, stands := blockStillStands(tr, head, prior); stands {
		t.Error("with the park lifted the block still stands, so the arm above was reading some other obstacle")
	}
}
