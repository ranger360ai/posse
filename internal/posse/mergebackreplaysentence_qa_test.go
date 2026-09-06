//go:build posse_arm3

// QA pins for the SENTENCES ranger-base-emgdb added, built by the verify of
// that close (ranger-base-9yje2).
//
// 67effd0 gave equivalentOnBase a third kind of evidence and two new
// sentence builders to say so — unmeasuredEvidence (3 arms) and
// unmeasuredClause (3 arms). Its own four pins cover the pairing, the wrong
// arm, the ambiguous key and the delete refusal, and between them they reach
// exactly one of those six arms: unmeasuredEvidence's replay-only arm.
// unmeasuredClause's replay arm and both builders' MIXED arm had no reader at
// all, in the one file whose lineage is a sentence that overstated its
// evidence (ranger-base-x8jp, ranger-base-hk02) and which this change touched
// again.
//
// So: the reporting half, said out loud. A replay pairing is an identity
// match and not a record of anybody's decision, and a branch carrying one of
// each must say both.
package posse

import (
	"strings"
	"testing"
)

// replayFixture is a session tree whose one commit main replayed by rebase:
// same author, same AUTHOR date, same subject, different bytes, no trailer.
// The same shape as TestMergeBackPairsACommitReplayedOntoTheBaseByRebase's,
// kept here so this file's arms do not move when that one's do.
func replayFixture(t *testing.T) (*App, string, *SessionTree) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "shared.mk", "PHONY := build test\n", "seed the makefile")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	const subject = "the reuse guard gets a make target (ranger-base-nw9zg)"
	const authored = "2026-09-02T16:28:53-04:00"
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test verify-gotest\n", subject, authored)
	commitIn(t, repo, "shared.mk", "PHONY := build test verify-parallel\n", "a sibling bead's target")
	commitAtIn(t, repo, "shared.mk", "PHONY := build test verify-parallel verify-gotest\n", subject, authored)
	return a, repo, tr
}

// The refusal an operator reads before retiring a tree, over work accounted
// for by a replay alone. It may not borrow the trailer's words: a rebase
// writes no trailer, and "recorded as landed" claims a record that does not
// exist.
func TestQAUnaccountedForNamesAReplayAsAnIdentityMatch(t *testing.T) {
	t.Parallel()
	_, repo, tr := replayFixture(t)

	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 1 || !eq[0].replayed {
		t.Fatalf("fixture: the one commit must pair as a replay, got %+v", eq)
	}

	got := unaccountedFor(tr, false)
	for _, want := range []string{
		"no record says which bead",
		"replayed onto main as",
		"an identity match and not a measurement of what the replay kept",
		"compare (`git log main.." + tr.Branch + "`)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, got)
		}
	}
	for _, unwant := range []string{
		"recorded as landed in", // no trailer was written; claiming one is x8jp's overstatement
		"cherry picked",         //
		"equivalent patch",      // and no measurement of content was made either
		"NOT landed",            // hk02: ahead by sha is not ahead by work
	} {
		if strings.Contains(got, unwant) {
			t.Errorf("the refusal must not say %q over a rebase replay:\n%s", unwant, got)
		}
	}
}

// One branch, one commit of each unmeasured kind. Both sentence builders have
// a MIXED arm and neither had a reader: a refusal that names only the trailer
// over a branch that is half replay is the same class of overstatement, and a
// clause that names only the replay hides the decision somebody actually
// recorded.
func TestQAUnmeasuredSentencesNameBothKindsOfEvidence(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
	commitIn(t, repo, "shared.mk", "PHONY := build test\n", "seed the makefile")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	// One the launcher PICKED and a human resolved: the trailer is the only
	// evidence left, because the resolution changed the patch.
	commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the adr goes in")
	picked := mustGit(t, tr.Path, "rev-parse", "HEAD")
	// One the launcher REBASED and a human resolved: no trailer at all, and
	// the identity is the only evidence left.
	const subject = "the reuse guard gets a make target (ranger-base-nw9zg)"
	const authored = "2026-09-02T16:28:53-04:00"
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test verify-gotest\n", subject, authored)

	commitIn(t, repo, "adr.md", "status: accepted (amended on the way in)\n",
		"s-1: the adr goes in\n\n(cherry picked from commit "+picked+")")
	commitIn(t, repo, "shared.mk", "PHONY := build test verify-parallel\n", "a sibling bead's target")
	commitAtIn(t, repo, "shared.mk", "PHONY := build test verify-parallel verify-gotest\n", subject, authored)

	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 2 {
		t.Fatalf("fixture: both commits must be accounted for, got %+v", eq)
	}
	var trailer, replay int
	for _, e := range eq {
		switch {
		case e.byPatch:
			t.Fatalf("fixture: neither commit may be a patch-id twin, got %+v", eq)
		case e.replayed:
			replay++
		default:
			trailer++
		}
	}
	if trailer != 1 || replay != 1 {
		t.Fatalf("fixture: one of each kind is the whole point, got trailer=%d replay=%d", trailer, replay)
	}

	// The listing's clause names both, and says what neither is.
	clause := unaccountedFor(tr, false)
	for _, want := range []string{
		"recorded as landed in",
		"replayed onto main as",
		"a decision and an identity match, and neither is a measurement of what the landing kept",
	} {
		if !strings.Contains(clause, want) {
			t.Errorf("the clause does not say %q:\n%s", want, clause)
		}
	}

	// And the delete refusal names both, and still refuses.
	err = RemoveSessionTree(tr, false)
	if err == nil {
		t.Fatal("a branch no measurement of content accounts for was retired unattended")
	}
	if !strings.Contains(err.Error(), "git's own -x trailer and a replay of the same commit") {
		t.Errorf("the refusal must name both kinds of evidence it has, got: %v", err)
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the refused branch was deleted anyway")
	}
}
