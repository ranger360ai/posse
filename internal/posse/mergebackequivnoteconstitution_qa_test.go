package posse

// THE ≡ LINE'S SECOND CALL SITE IS THE CONSTITUTION ARM, AND NOTHING
// MEASURED IT (ranger-base-n3s4s, verifying ranger-base-dmzk7's close).
//
// dmzk7 fixed the sentence at BOTH sites MergeSessionWork reports an
// equivalence from (worktree.go, the constitution arm and the pre-rebase
// arm). Its pins reach the second one. MEASURED on the close's own tree
// (2d2f139), `go test -overlay`: revert the constitution arm alone —
// `o.Unmeasured = ""` there and nowhere else — and the whole merge-back
// suite AND every `-run Constitution` pin stay GREEN, while "nothing here is
// unlanded" prints again over a branch that edits the law and is only an
// identity match.
//
// Why the arm's own pin cannot see it:
// TestQAConstitutionLandStillReportsWorkAlreadyOnTheBase lands its work with
// `cherry-pick -x`, which is a patch-id twin, so `Unmeasured` is empty there
// whether the fix is present or not — and it asserts `Merged` and a non-empty
// `Equivalent` rather than the sentence. It is a correct pin of a different
// fact.
//
// This arm is where the confident sentence costs the most. Blocked() is
// false on every equivalence, so no merge-back-blocked bead is filed and this
// sentence is the whole record — and the branch it is printed about is one
// ADR 0015 §2/§3 says a human must read before it becomes main's.

import (
	"strings"
	"testing"
)

func TestQAConstitutionArmsEquivalenceNoteAsksItsEvidenceToo(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	// The marker on MAIN is what makes this the constitution repo, so the
	// branch below reaches the constitution arm rather than the one dmzk7's
	// own pins go through.
	commitIn(t, repo, ConstitutionRepoMarker+"/keep.md", "the law\n", "seed the constitution")
	const rel = ConstitutionRepoMarker + "/developer.md"
	commitIn(t, repo, rel, "one\n", "seed the developer law")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	// dmzk7's shape on a constitution path: the operator applies v1 by hand
	// under its own sha, and the session AMENDS afterwards. `--amend` keeps
	// the author, the AUTHOR date and the subject, so the pairing still
	// matches by identity while the bytes no longer do.
	const subject = "s-1: edit the law (ranger-base-n3s4s)"
	const authored = "2026-09-02T16:28:53-04:00"
	commitAtIn(t, tr.Path, rel, "one\ntwo\n", subject, authored)
	commitAtIn(t, repo, rel, "one\ntwo\n", subject, authored)
	write(t, tr.Path+"/"+rel, "one\ntwo\nthree-main-has-never-seen-this\n")
	mustGit(t, tr.Path, "add", rel)
	mustGit(t, tr.Path, "commit", "-q", "--amend", "--no-edit", "--", rel)

	// PREMISE 1: the branch really does touch the constitution class, which
	// is what routes it to the arm under test.
	hit, why := constitutionOnBranch(tr)
	if why != "" || len(hit) == 0 {
		t.Fatalf("fixture: the branch must reach the constitution arm, got hit=%v why=%q", hit, why)
	}
	// PREMISE 2: the pairing is the UNMEASURED kind. A patch-id twin would
	// license the confident sentence honestly and measure nothing here —
	// which is exactly why the arm's existing pin cannot see this.
	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 1 || !eq[0].replayed || eq[0].byPatch {
		t.Fatalf("fixture: the amended commit must pair by identity alone, got %+v", eq)
	}
	// PREMISE 3: and the branch really is the last copy of something.
	lost, lerr := contentNotOnBase(repo, "main", tr.Branch)
	if lerr != nil {
		t.Fatal(lerr)
	}
	if len(lost) != 1 || lost[0] != rel {
		t.Fatalf("fixture: main must not hold the branch's bytes for %s, got %v", rel, lost)
	}

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	note := o.EquivalentNote()
	if strings.Contains(note, "nothing here is unlanded") {
		t.Errorf("the ≡ line claimed nothing is unlanded over a CONSTITUTION branch holding a line main has never seen "+
			"(worktree.go, the constitution arm of MergeSessionWork — ranger-base-dmzk7's other call site): %q", note)
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

	// CONTROL, and the pin is worthless without it: the same arm, the same
	// repo, the work landed as a CLEAN pick — a patch-id measurement of
	// content — must still earn the confident sentence. Otherwise "does not
	// say nothing is unlanded" is satisfied by a note that can no longer say
	// it at all, and by re-filing the every-pass refusal ranger-base-emgdb
	// removed.
	a2 := wtApp(t)
	repo2 := wtRepo(t)
	commitIn(t, repo2, ConstitutionRepoMarker+"/keep.md", "the law\n", "seed the constitution")
	tr2, err := a2.EnsureSessionTree(repo2, "s-2", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr2.Path, rel, "rewritten by the session\n", "s-2: edit the law")
	sha, err := git(tr2.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := git(repo2, "cherry-pick", "-x", sha); err != nil {
		t.Skipf("git cherry-pick: %v %s", err, out)
	}
	o2, err := MergeSessionWork(tr2)
	if err != nil {
		t.Fatal(err)
	}
	if !o2.Merged || len(o2.Equivalent) == 0 {
		t.Fatalf("control: work the base already holds must still report as landed from this arm: %+v", o2)
	}
	if !strings.Contains(o2.EquivalentNote(), "nothing here is unlanded") {
		t.Errorf("control: a measured patch-id twin must still earn the confident sentence from the constitution arm:\n%s", o2.EquivalentNote())
	}
}
