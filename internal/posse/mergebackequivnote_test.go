// The ≡ line's own evidence check (ranger-base-dmzk7).
//
// EquivalentNote is the surface that decides whether a human is told at all:
// on every equivalence Blocked() is false, no merge-back-blocked bead is
// filed, and this sentence is the whole record. It asserted "nothing here is
// unlanded" for all three kinds of pairing undifferentiated — a measurement
// claim, made from evidence that is a measurement in only one of the three —
// while unaccountedFor, on the same tree in the same pass, said "an identity
// match and not a measurement of what the replay kept; compare before
// retiring the tree". Two confidences, one piece of evidence.
//
// The shape that made it cost something: the launcher lands v1 of a
// session's commit, then the session AMENDS it. `--amend` keeps the author,
// the AUTHOR date and the subject, so the identity pairing still matches
// while the content no longer does, and the new bytes are on no ref but this
// branch.
package posse

import (
	"strings"
	"testing"
)

// amendedAfterLandingFixture is that shape: main holds v1 of the session's
// commit under its own sha, and the branch holds an amended v2 whose extra
// line main has never seen.
func amendedAfterLandingFixture(t *testing.T) (string, *SessionTree) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "notes.txt", "one\n", "seed the notes")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	const subject = "the session's own work (ranger-base-dmzk7)"
	const authored = "2026-09-02T16:28:53-04:00"
	commitAtIn(t, tr.Path, "notes.txt", "one\ntwo\n", subject, authored)
	// The launcher lands v1 under another sha, the way a rebase does.
	commitAtIn(t, repo, "notes.txt", "one\ntwo\n", subject, authored)
	// …and the session amends its commit afterwards. Author, author date and
	// subject survive; the bytes do not.
	write(t, tr.Path+"/notes.txt", "one\ntwo\nthree-main-has-never-seen-this\n")
	mustGit(t, tr.Path, "add", "notes.txt")
	mustGit(t, tr.Path, "commit", "-q", "--amend", "--no-edit", "--", "notes.txt")
	return repo, tr
}

func TestMergeBackNoteWillNotClaimAMeasurementItDoesNotHave(t *testing.T) {
	t.Parallel()
	repo, tr := amendedAfterLandingFixture(t)

	// The fixture only measures anything while the pairing is the UNMEASURED
	// kind: a patch-id twin would license the confident sentence honestly.
	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 1 || !eq[0].replayed || eq[0].byPatch {
		t.Fatalf("fixture: the amended commit must pair by identity alone, got %+v", eq)
	}
	// And while the branch really is the last copy of something. This is the
	// tree's own measurement of content, and the sentence under test was
	// contradicting it.
	lost, err := contentNotOnBase(repo, "main", tr.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if len(lost) != 1 || lost[0] != "notes.txt" {
		t.Fatalf("fixture: main must not hold the branch's bytes for notes.txt, got %v", lost)
	}

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	note := o.EquivalentNote()
	if strings.Contains(note, "nothing here is unlanded") {
		t.Errorf("the ≡ line claimed nothing is unlanded over a branch holding a line main has never seen: %q", note)
	}
	for _, want := range []string{
		"accounted for on main",
		"an identity match and not a measurement of what the replay kept",
		"compare (`git log main.." + tr.Branch + "`) before retiring the tree",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("the ≡ line does not say %q:\n%s", want, note)
		}
	}

	// The two surfaces, on one tree in one pass: the sentence a human reads
	// before retiring the tree must not depend on which one printed it.
	gate := unaccountedFor(tr, false)
	for _, want := range []string{
		"an identity match and not a measurement of what the replay kept",
		"compare (`git log main.." + tr.Branch + "`) before retiring the tree",
	} {
		if !strings.Contains(gate, want) {
			t.Errorf("--land's gate no longer says %q, so the two surfaces are being pinned apart:\n%s", want, gate)
		}
	}

	// What deliberately did NOT change. Reporting a strand here would put
	// back the every-pass P1 over work that IS landed (ranger-base-emgdb):
	// contentNotOnBase cannot tell this amend from a rebase conflict a human
	// resolved by keeping both sides, where the branch's bytes are on main
	// nowhere and the work is nevertheless entirely there. The sentence
	// sends a person to read the two commits; it does not guess for them.
	if o.Blocked() {
		t.Errorf("an identity-matched branch was reported blocked, which files a P1 on every rebased landing: %q", o.Reason)
	}
	if !o.Merged || len(o.Equivalent) != 1 {
		t.Errorf("the pairing itself was dropped: %+v", o)
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the branch was deleted on evidence that is not a measurement")
	}
}

// The control the pin above is worthless without: the same fixture landed as
// a CLEAN pick, which is a patch-id measurement of content, must still print
// the confident sentence. Without it "does not say nothing is unlanded" is
// satisfied by an EquivalentNote that can no longer say it at all.
func TestMergeBackNoteStillSaysNothingIsUnlandedWhenItMeasuredIt(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "notes.txt", "one\n", "seed the notes")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "notes.txt", "one\ntwo\n", "the session's own work (ranger-base-dmzk7)")
	sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
	// The base moves first, or the pick rebuilds the identical commit object
	// and the base reaches it by sha with nothing measured.
	commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	mustGit(t, repo, "cherry-pick", sha)

	eq := equivalentOnBase(repo, "main", tr.Branch)
	if !measuredOnBase(eq) {
		t.Fatalf("fixture: the clean pick must be a patch-id measurement, got %+v", eq)
	}
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if note := o.EquivalentNote(); !strings.Contains(note, "nothing here is unlanded") {
		t.Errorf("a measured pairing lost the sentence it earned: %q", note)
	}
	if o.Unmeasured != "" {
		t.Errorf("a wholly measured pairing must carry no unmeasured clause, got %q", o.Unmeasured)
	}
}

// Why the OTHER fix the bead offered is not available, pinned so a later
// pass does not take it and re-strand landed work.
//
// contentNotOnBase answers the DELETE question — is this branch the last
// copy of these bytes — and gating Merged on it would report a strand here.
// This fixture is ranger-base-nw9zg's: a rebase whose conflict a human
// resolved by keeping both sides. The branch's bytes are on main nowhere,
// and the work is nevertheless entirely on main. It is indistinguishable
// from the amend above by content alone, so only a person reading the two
// commits can say which is which — and reporting a strand on this evidence
// puts back the every-pass P1 over landed work that ranger-base-emgdb
// removed, on the 36% of landings ADR 0051 measures as rebases.
func TestContentNotOnBaseCannotDecideTheReportingHalf(t *testing.T) {
	t.Parallel()
	_, repo, tr := replayFixture(t)

	lost, err := contentNotOnBase(repo, "main", tr.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if len(lost) != 1 || lost[0] != "shared.mk" {
		t.Fatalf("the resolution kept both sides, so main holds the branch's bytes for no path it touched — got %v", lost)
	}
	// And yet the work IS on main: the branch's target is in main's line.
	if line := mustGit(t, repo, "show", "main:shared.mk"); !strings.Contains(line, "verify-gotest") {
		t.Fatalf("fixture: main must carry the session's target, got %q", line)
	}
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Blocked() || !o.Merged {
		t.Errorf("landed work was reported as a strand on a content measurement that cannot decide it: %+v", o)
	}
}
