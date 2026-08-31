package posse

import (
	"os"
	"strings"
	"testing"
)

// The two arms TestRemoveSessionTreeRetiresOnlyWhatIsMeasuredOnTheBase does
// not have, found verifying the close of ranger-base-as19 (ranger-base-nfmq).
// Kept in their own file so they survive whatever the next persona does to
// worktree_test.go.

// ranger-base-x8jp. as19 made a patch-id match LICENSE DELETION, and the
// safety property it rests on — "the base holds this patch, so the branch is
// the last copy of nothing" — is not true of patch-id, which NORMALISES
// WHITESPACE. Two commits differing only by a tab against spaces are
// patch-id equivalent, so measuredOnBase says yes and the branch holding the
// only copy of those bytes is deleted with -D and no --force.
//
// The fixture is the hand-resolved pick as19 set out to protect, resolved in
// the one way that leaves the patch-id equal: it was re-indented. That
// resolution never reaches the trailer arm, because the trailer arm is only
// consulted when the patch-id does NOT match.
//
// Fixed in ranger-base-x8jp: the measured arm now asks contentNotOnBase as
// well, and the branch is KEPT. Un-skipped unchanged — it asserted the
// keeping all along, so the fix is what turned it green.
func TestRemoveSessionTreeDeletesABranchWhoseBytesAreNotOnTheBase(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	// The session's bytes: indented with a TAB.
	commitIn(t, tr.Path, "adr.md", "status: accepted\n\tindented with a TAB\n", "s-1: the fix")
	commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	// The base's bytes: the same change re-applied by hand and re-indented
	// with SPACES. Different content, same patch-id.
	commitIn(t, repo, "adr.md", "status: accepted\n        indented with a TAB\n", "main: re-applied by hand")

	// Ahead by sha, or the guard under test is never reached.
	if n := mustGit(t, repo, "rev-list", "--count", "main.."+tr.Branch); n != "1" {
		t.Fatalf("rev-list --count main..%s = %s; the fixture must be ahead by sha", tr.Branch, n)
	}
	// The fixture is only worth anything while git still calls these two
	// equivalent — a git that stopped normalising whitespace would make this
	// arm measure nothing, so say so instead of passing quietly.
	if c := mustGit(t, repo, "cherry", "main", tr.Branch); !strings.HasPrefix(c, "- ") {
		t.Fatalf("git cherry no longer calls a whitespace-only difference equivalent (%q); this fixture measures nothing", c)
	}
	branchTip := mustGit(t, repo, "rev-parse", tr.Branch)

	err = RemoveSessionTree(tr, false)

	onMain := mustGit(t, repo, "show", "main:adr.md")
	onBranch := mustGit(t, repo, "show", branchTip+":adr.md")
	if onMain == onBranch {
		t.Fatalf("the fixture must diverge in content, both are %q", onMain)
	}
	if err == nil {
		t.Errorf("a branch was retired without --force while its bytes are not on the base:\n  main   = %q\n  branch = %q", onMain, onBranch)
	} else {
		// The refusal has to name THIS evidence. A trailer sentence here
		// would be the mixed-pairing overstatement wearing the fix's
		// clothes: nothing in this fixture has a trailer.
		if !strings.Contains(err.Error(), "patch-id normalises whitespace") {
			t.Errorf("the refusal must say which evidence it does not have, got: %v", err)
		}
		if !strings.Contains(err.Error(), "adr.md") {
			t.Errorf("the refusal must name the path whose bytes are only here, got: %v", err)
		}
		if strings.Contains(err.Error(), "-x trailer") {
			t.Errorf("no commit in this fixture is accounted for by a trailer, got: %v", err)
		}
	}
	if _, serr := os.Stat(tr.Path); serr != nil {
		t.Error("the tree was removed although the base does not hold its content")
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the branch was deleted although the base does not hold its content")
	}
}

// A MIXED pairing: one commit measured by patch-id, one accounted for only by
// git's -x trailer. measuredOnBase is all-or-nothing by design — "nothing
// accounted for is not proof", and neither is "most of it" — so the tree is
// kept. The existing arms are each pure, so nothing pinned the boundary
// between them (ranger-base-nfmq).
func TestRemoveSessionTreeKeepsAMixedPairing(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
	commitIn(t, repo, "other.md", "one\n", "seed other")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the clean one")
	clean := mustGit(t, tr.Path, "rev-parse", "HEAD")
	commitIn(t, tr.Path, "other.md", "two\n", "s-1: the hand-resolved one")
	hand := mustGit(t, tr.Path, "rev-parse", "HEAD")

	commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	// One lands as an equivalent patch, one only as a trailer over a
	// resolution that kept different bytes.
	mustGit(t, repo, "cherry-pick", "-x", clean)
	commitIn(t, repo, "other.md", "two, but hand-resolved differently\n",
		"main: the pick\n\n(cherry picked from commit "+hand+")")

	// The fixture is genuinely mixed, or this arm is just the trailer arm
	// again under another name.
	eq := equivalentOnBase(repo, "main", tr.Branch)
	byPatch, byTrailer := 0, 0
	for _, e := range eq {
		if e.byPatch {
			byPatch++
		} else {
			byTrailer++
		}
	}
	if byPatch == 0 || byTrailer == 0 {
		t.Fatalf("the fixture must pair one of each, got %d by patch and %d by trailer: %+v", byPatch, byTrailer, eq)
	}
	if measuredOnBase(eq) {
		t.Error("a mixed pairing is not a measurement: one of these commits is only somebody's assertion")
	}

	err = RemoveSessionTree(tr, false)
	if err == nil {
		t.Fatal("a mixed pairing was retired; one of its commits is accounted for only by a trailer")
	}
	if !strings.Contains(err.Error(), "-x trailer") {
		t.Errorf("the refusal must name the evidence it does not have, got: %v", err)
	}
	// The SECONDARY of ranger-base-x8jp: the sentence used to count every
	// commit ahead of the base and then list the patch-measured pairing
	// among the ones it says only a trailer accounts for. The refusal is
	// right; the count and the list were not.
	if !strings.Contains(err.Error(), "1 of which have no record of landing beyond") {
		t.Errorf("the refusal must count only the trailer-accounted commits, got: %v", err)
	}
	for _, e := range eq {
		if e.byPatch && strings.Contains(err.Error(), e.note) {
			t.Errorf("the refusal lists a patch-measured pairing among the ones only a trailer accounts for: %q in %v", e.note, err)
		}
	}
	if _, serr := os.Stat(tr.Path); serr != nil {
		t.Error("the refused tree was removed anyway")
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the refused branch was deleted anyway")
	}
}
