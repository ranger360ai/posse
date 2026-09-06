//go:build posse_arm3

package posse

import (
	"os"
	"strings"
	"testing"
)

// The third way a commit's work reaches the base under another sha: the
// launcher REBASED the branch onto main and a conflict was resolved by hand.
//
// That leaves neither of the two records equivalentOnBase used to ask for. A
// rebase writes no `-x` trailer at all — only a cherry-pick does — and the
// resolution amends the patch, so `git cherry` says `+`. Both arms miss, the
// pairing comes back nil, and MergeSessionWork reports a strand word for
// word identical to a real one over work that is entirely on main.
//
// Measured, not supposed: ranger-base-nw9zg's five commits landed on main at
// 2026-09-02 22:15:47 and the branch was still called unlanded two days
// later. Two of the five were patch-id twins; the other three had absorbed a
// sibling bead's target into a shared Makefile `.PHONY` line and a scrub of
// the fixture literals main made on top. ADR 0051 puts the rebase share of
// landings at 36%, so this is the common case, not the corner
// (ranger-base-emgdb).
//
// The fixture is that shape and nothing else: same author, same AUTHOR date,
// same subject, different bytes, no trailer.
func TestMergeBackPairsACommitReplayedOntoTheBaseByRebase(t *testing.T) {
	t.Parallel()
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

	// main moves on with a sibling's target in the same line, then the
	// launcher replays the session's commit onto it and the conflict is
	// resolved by keeping both. The replay carries author and author date
	// through unchanged; the bytes are neither side's.
	commitIn(t, repo, "shared.mk", "PHONY := build test verify-parallel\n", "a sibling bead's target")
	commitAtIn(t, repo, "shared.mk", "PHONY := build test verify-parallel verify-gotest\n", subject, authored)

	// The fixture only measures anything while BOTH old arms miss it.
	if c := mustGit(t, repo, "cherry", "main", tr.Branch); !strings.HasPrefix(c, "+ ") {
		t.Fatalf("git cherry calls this a patch-id twin (%q); the fixture measures nothing new", c)
	}
	if body := mustGit(t, repo, "log", "--format=%B", "-1", "main"); strings.Contains(body, "cherry picked from commit") {
		t.Fatalf("the landing wrote an -x trailer (%q); the fixture is the trailer arm again", body)
	}

	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 1 {
		t.Fatalf("the commit main replayed is not paired with its twin: %+v", eq)
	}
	if !eq[0].replayed || eq[0].byPatch {
		t.Errorf("the pairing must be marked a replay and never a measurement of content: %+v", eq[0])
	}
	if want := abbrevSHA(mustGit(t, repo, "rev-parse", "main")); !strings.Contains(eq[0].note, want) {
		t.Errorf("the note must name the commit on main a human can check it against (%s): %q", want, eq[0].note)
	}

	// The whole point: no strand report, no P1, and the branch kept.
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Blocked() {
		t.Errorf("a branch whose work is already on main was reported blocked: %q", o.Reason)
	}
	if !o.Merged || len(o.Equivalent) != 1 {
		t.Errorf("the outcome must say the base already holds this work: %+v", o)
	}
	// The sentence, on the evidence this arm actually has. It may say the
	// commit is accounted for — that is the whole fix above — and it may not
	// say it MEASURED that, because an identity match is not a measurement
	// of what the replay kept (ranger-base-dmzk7). The words are
	// unaccountedFor's, which is the other surface answering the same
	// question about the same tree in the same pass.
	note := o.EquivalentNote()
	for _, want := range []string{
		"accounted for on main",
		"an identity match and not a measurement of what the replay kept",
		"compare (`git log main.." + tr.Branch + "`) before retiring the tree",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("the operator-facing sentence does not say %q, got: %q", want, note)
		}
	}
	if strings.Contains(note, "nothing here is unlanded") {
		t.Errorf("an identity match claimed a measurement it does not have: %q", note)
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the branch was moved or deleted by a read-only equivalence check")
	}
}

// The wrong arm the pin above is worthless without: everything the same
// EXCEPT the author date, which a rebase never rewrites. A commit main
// happens to share a subject with is not a commit main replayed, and this is
// the arm that would go green if the pairing keyed on the subject alone.
func TestMergeBackDoesNotPairOnTheSubjectAlone(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "shared.mk", "PHONY := build test\n", "seed the makefile")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	const subject = "the reuse guard gets a make target (ranger-base-nw9zg)"
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test verify-gotest\n", subject, "2026-09-02T16:28:53-04:00")
	commitIn(t, repo, "shared.mk", "PHONY := build test verify-parallel\n", "a sibling bead's target")
	commitAtIn(t, repo, "shared.mk", "PHONY := build test verify-parallel\n# unrelated\n", subject, "2026-09-03T09:00:00-04:00")

	if eq := equivalentOnBase(repo, "main", tr.Branch); len(eq) != 0 {
		t.Fatalf("a same-subject commit authored at another time was paired as a replay: %+v", eq)
	}
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Blocked() {
		t.Fatalf("real unlanded work was reported as landed: %+v", o)
	}
	if len(o.Equivalent) != 0 {
		t.Errorf("a strand must claim no equivalence: %+v", o.Equivalent)
	}
}

// An ambiguous key is not a key. Two commits on the base carrying one
// (author, author date, subject) identify neither, and "unaccounted for" is
// the honest default everywhere else in equivalentOnBase.
func TestMergeBackDoesNotPairAnAmbiguousReplayKey(t *testing.T) {
	t.Parallel()
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
	commitAtIn(t, repo, "one.txt", "one\n", subject, authored)
	commitAtIn(t, repo, "two.txt", "two\n", subject, authored)

	if eq := equivalentOnBase(repo, "main", tr.Branch); len(eq) != 0 {
		t.Fatalf("two commits on main share the key and one of them was still named as the twin: %+v", eq)
	}
}

// A replay pairing is an inference about somebody's resolution, not a
// measurement of content — so it must move the REPORTING half and nothing
// else. RemoveSessionTree still refuses, and the refusal names the evidence
// it actually has: saying "-x trailer" over a rebase that wrote no trailer
// is the same overstatement ranger-base-x8jp removed from the sentence
// beside it.
func TestReplayPairingNeverLicensesADelete(t *testing.T) {
	t.Parallel()
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

	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 1 {
		t.Fatalf("fixture: the replay must pair, got %+v", eq)
	}
	if measuredOnBase(eq) {
		t.Fatal("a replay pairing was read as a measurement of content — that is a licence to delete the last copy of those bytes")
	}

	err = RemoveSessionTree(tr, false)
	if err == nil {
		t.Fatal("a branch accounted for only by a replay pairing was retired unattended")
	}
	if !strings.Contains(err.Error(), "a replay of the same commit") {
		t.Errorf("the refusal must name the evidence it has, got: %v", err)
	}
	if strings.Contains(err.Error(), "-x trailer") {
		t.Errorf("nothing in this fixture has a trailer; the refusal must not claim one, got: %v", err)
	}
	if _, serr := os.Stat(tr.Path); serr != nil {
		t.Error("the refused tree was removed anyway")
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the refused branch was deleted anyway")
	}
}

// commitAtIn is commitIn with the AUTHOR date pinned — the one field a
// rebase carries through a replay unchanged, and therefore the only way to
// build a replay fixture that is not just two commits made a second apart.
func commitAtIn(t *testing.T, dir, path, body, msg, date string) {
	t.Helper()
	write(t, dir+"/"+path, body)
	mustGit(t, dir, "config", "user.email", "p@example.com")
	mustGit(t, dir, "config", "user.name", "p")
	mustGit(t, dir, "add", path)
	mustGit(t, dir, "commit", "-q", "--date="+date, "-m", msg, "--", path)
}
