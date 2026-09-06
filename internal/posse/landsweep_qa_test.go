//go:build posse_arm3

package posse

// QA pin for the landing sweep's THIRD answer (ranger-base-gs9j, verifying
// ranger-base-nurl).
//
// unlandedCount returns two values, and its own doc comment says why: false
// is "the question could not be answered" — a detached repo, or a branch git
// would not count — and that is NOT "nothing to land". The nine mutations
// the close was checked against moved the sweep's other arms; this one was
// unheld. Collapsing `ok && n == 0` to `n == 0` — reading an unanswerable
// count as an empty one — left the whole internal/rhq package green, and it
// turns the exact tree this bead exists for into a silent skip: a closed
// bead's commits, a repo that cannot say where they should go, and a pass
// that says nothing at all.
//
// The wrong arm is what makes this a pin: with the collapse applied the ⚠
// line disappears and the test fails.

import (
	"strings"
	"testing"
)

func TestSweepDoesNotReadAnUnanswerableCountAsNothingToLand(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)

	// Make the count unanswerable, the way a repo does it in the field: the
	// base this tree was cut from is unrecorded and the checkout is detached,
	// so there is no branch to count against.
	if _, err := git(repo, "config", "--unset", baseKey(tr.Branch)); err != nil {
		t.Fatal(err)
	}
	if _, err := git(repo, "checkout", "--detach"); err != nil {
		t.Fatal(err)
	}

	// The fixture's positive witness: an assertion about what a pass SAYS is
	// worth nothing if the state it is supposed to say it about was never
	// built. The tree must really be one whose count cannot be taken, and it
	// must really still be holding the work.
	trs, err := SessionTreesIn([]string{repo})
	if err != nil || len(trs) != 1 {
		t.Fatalf("fixture: session trees %v %v", len(trs), err)
	}
	if trs[0].Base != "" {
		t.Fatalf("fixture: the tree still has a base %q — the count is answerable and this test measures nothing", trs[0].Base)
	}
	if _, ok := unlandedCount(trs[0]); ok {
		t.Fatal("fixture: unlandedCount still answers — the mutation this pins would be a no-op here")
	}

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, tr.Branch) {
		t.Errorf("a closed bead's unlanded tree went unmentioned because its count could not be taken — the silence this bead exists to remove:\n%s", out)
	}
	if !strings.Contains(out, "did NOT reach") || !strings.Contains(out, "detached HEAD") {
		t.Errorf("the pass must say WHY it could not land, not merely name the branch:\n%s", out)
	}
}
