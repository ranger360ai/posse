//go:build posse_arm2

package posse

import (
	"os"
	"path/filepath"
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
	o, err := MergeSessionWork(a, tr)
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
	o, err := MergeSessionWork(a, tr)
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

// ALL OR NOTHING over a MULTI-COMMIT branch, which is the half of the rule
// every fixture above leaves untested: they each put one commit on the
// branch, so `return nil` on the first unaccounted-for commit and "return
// what paired so far" cannot be told apart by any of them.
//
// A replayer reads this rule before deciding how many commits to write.
// ranger-base-eawjq's two-commit merge-back decided on TWO twins rather than
// one collapsed landing because of it, and ranger-base-aza46's three-commit
// one wrote three; both say so on the bead. If the loop ever returned the
// partial list, a collapse would read as LANDED — MEASURED 2026-10-04 by
// planting `continue` in place of that `return nil`: this fixture's
// MergeSessionWork flipped to blocked=false and named the one twin as the
// equivalence, so the branch's second commit would be dropped from main with
// a green word over it. The honest answer is the strand: the block re-files,
// and a person looks.
func TestMergeBackPairsNothingWhenOnlySomeOfABranchLanded(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "shared.mk", "PHONY := build test\n", "seed the makefile")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	// The shape of a two-commit merge-back: two subjects, two AUTHOR dates.
	const subjA = "the first half (ranger-base-bwp7h)"
	const subjB = "the second half (ranger-base-bwp7h)"
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test a\n", subjA, "2026-10-03T22:41:58-04:00")
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test a b\n", subjB, "2026-10-04T16:37:43-04:00")

	// The collapse: ONE landing on main, carrying both commits' content under
	// the first commit's identity. All the work is there; only one of the two
	// keys is.
	commitIn(t, repo, "other.txt", "main moved on\n", "a sibling bead")
	commitAtIn(t, repo, "shared.mk", "PHONY := build test verify-x a b\n", subjA, "2026-10-03T22:41:58-04:00")

	if eq := equivalentOnBase(repo, "main", tr.Branch); len(eq) != 0 {
		t.Fatalf("one of two commits paired and the pairing was still returned: %+v\n"+
			"A partial pairing is not an equivalence — MergeSessionWork would call the branch landed and the unpaired commit would be lost.", eq)
	}
	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Blocked() {
		t.Fatalf("a branch whose second commit reached main under no key was reported as landed: %+v", o)
	}
	if len(o.Equivalent) != 0 {
		t.Errorf("a strand must claim no equivalence: %+v", o.Equivalent)
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

// ranger-base-4r8fw: the POSITIVE half of the pin above, and the claim a
// prose fix to the runbook cannot make on its own.
//
// TestMergeBackPairsNothingWhenOnlySomeOfABranchLanded measures the squash:
// two commits landing as one pairs one key and strands the other, and the
// honest answer is nil. That is also exactly what mergeBlockedReplay's text
// used to PRESCRIBE — it computed one merged tree for the whole branch and
// spoke of "that commit's" identity in the singular, which every entry of
// this class until ranger-base-83nrw could get away with because every one of
// them was a one-commit branch (-xea2y, -3e68k, -h1q5o, -v3tt4, -yrag8). A
// seat following it on a multi-commit branch produced the squash above and
// the block re-filed forever over work that was entirely on main.
//
// So this runs the corrected recipe as rendered — `rev-list --reverse`, then
// per original C a `merge-tree --write-tree --merge-base=$C^ HEAD $C`, the
// `cat-file blob` loop, a resolution, and ONE commit carrying C's own author
// date and subject — and asserts equivalentOnBase accounts for BOTH. The
// merge base walking the originals is the non-obvious half: "ours" is the
// replay so far, so for commit 2 the base is commit 1 and not main's own
// merge base with the branch.
//
// Both replays are checked to be `+` under `git cherry` and trailer-free
// first, for the same reason the pins above are: with either cheap arm
// available this fixture would measure nothing about replayKey, which is the
// only arm a hand-resolved replay leaves.
func TestMergeBackPairsEveryCommitOfATwoCommitBranchReplayedOneByOne(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "shared.mk", "PHONY := build test\n", "seed the makefile")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	const subjA, dateA = "the first half (ranger-base-4r8fw)", "2026-10-03T22:41:58-04:00"
	const subjB, dateB = "the second half (ranger-base-4r8fw)", "2026-10-04T16:37:43-04:00"
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test a\n", subjA, dateA)
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test a b\n", subjB, dateB)

	// main moves on in the same line both of them touched, which is what
	// blocks the merge and what makes every per-commit replay below conflict
	// for real.
	commitIn(t, repo, "shared.mk", "PHONY := build test verify-x\n", "a sibling bead")
	if o, err := MergeSessionWork(a, tr); err != nil {
		t.Fatal(err)
	} else if !o.Blocked() {
		t.Fatalf("fixture: the branch has to be blocked or there is nothing for the recipe to replay: %+v", o)
	}

	// The resolution, keyed on each original's subject: keep both sides,
	// which is what a seat decides and what makes the bytes neither side's.
	resolved := map[string]string{
		subjA: "PHONY := build test verify-x a\n",
		subjB: "PHONY := build test verify-x a b\n",
	}
	originals := strings.Fields(mustGit(t, repo, "rev-list", "--reverse", "main.."+tr.Branch))
	if len(originals) != 2 {
		t.Fatalf("fixture: %d original(s) to replay, want 2 — one is the case the old copy was already correct for", len(originals))
	}
	for i, c := range originals {
		// merge-tree exits NONZERO on a conflict and writes the tree anyway,
		// which is the whole case this recipe is for — so the error is read
		// off the output and not off the exit code.
		out, _ := git(repo, "merge-tree", "--write-tree", "--merge-base="+c+"^", "HEAD", c)
		fields := strings.Fields(out)
		if len(fields) == 0 {
			t.Fatalf("original %d: merge-tree wrote no tree:\n%s", i+1, out)
		}
		tree := fields[0]
		if del := mustGit(t, repo, "diff", "--diff-filter=D", "--name-only", "HEAD", tree); del != "" {
			t.Fatalf("original %d: the merged tree deletes %q, which a write-and-add loop cannot carry", i+1, del)
		}
		for _, p := range strings.Fields(mustGit(t, repo, "diff", "--name-only", c+"^", c)) {
			b, err := gitRaw(repo, "cat-file", "blob", tree+":"+p)
			if err != nil {
				t.Fatalf("original %d: %s out of the merged tree: %v", i+1, p, err)
			}
			write(t, filepath.Join(repo, p), string(b))
			// TWO THINGS AT ONCE, and the second is why the merge base is
			// written as $C^.
			//
			// The fixture only measures a RESOLVED replay while the merge
			// needs resolving: a textually clean one is a patch-id twin and
			// `git cherry` would answer it without replayKey.
			//
			// And it is where a merge base of main's own — the branch's base,
			// which is what the one-commit copy computed — shows up. MEASURED
			// 2026-11-08 by planting `--merge-base=main` here: original 1's
			// shared.mk comes out CLEAN as `PHONY := build test a`, because
			// against that base only theirs changed the line, so the merge
			// takes theirs whole and main's own `verify-x` is silently gone.
			// A clean-looking tree that ate the base's work is the shape
			// ranger-base-83nrw's replay met and called "the one that nearly
			// ate main's work" (docs/notes.d/ranger-base-4r8fw.md quotes it),
			// and this is the assertion that sees it.
			if !strings.Contains(string(b), "<<<<<<<") {
				t.Fatalf("original %d: %s came out of the merge clean — either the fixture measures the patch-id arm, or the merge base is not $C^ and this tree has taken theirs over the base's own work:\n%s", i+1, p, b)
			}
		}
		subject := mustGit(t, repo, "log", "-1", "--format=%s", c)
		body, ok := resolved[subject]
		if !ok {
			t.Fatalf("original %d: no resolution for %q", i+1, subject)
		}
		// ONE commit per original, carrying its AUTHOR date and subject —
		// the three fields replayKey reads, and the whole of what pairs it.
		commitAtIn(t, repo, "shared.mk", body, subject, mustGit(t, repo, "log", "-1", "--format=%aI", c))
	}

	// The replay ADDS the branch's work and takes none of the base's away —
	// the cheap half of ranger-base-83nrw's deletion-count reading, asked
	// here because a pairing is about identity and would be satisfied by a
	// replay that dropped every hunk main had.
	if final := mustGit(t, repo, "show", "main:shared.mk"); final != "PHONY := build test verify-x a b" {
		t.Errorf("the replay did not keep main's own work beside the branch's: %q", final)
	}

	// Neither cheap arm may answer this, or the pairing below is not the one
	// a hand-resolved replay has to rely on.
	for _, ln := range strings.Split(mustGit(t, repo, "cherry", "main", tr.Branch), "\n") {
		if !strings.HasPrefix(ln, "+ ") {
			t.Fatalf("git cherry calls a replay a patch-id twin (%q); the fixture measures nothing new", ln)
		}
	}
	if body := mustGit(t, repo, "log", "--format=%B", "main", "--not", tr.Branch); strings.Contains(body, "cherry picked from commit") {
		t.Fatalf("a replay wrote an -x trailer; the fixture is the trailer arm again:\n%s", body)
	}

	// THE CLAIM. Two originals, two replays, and every one of them accounted
	// for — not one pairing and a nil.
	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 2 {
		t.Fatalf("a two-commit branch replayed one commit per original is accounted for %d time(s), want 2: %+v\n"+
			"This is the bug mergeBlockedReplay's text produced when it was written for one commit (ranger-base-4r8fw).", len(eq), eq)
	}
	for i, e := range eq {
		if !e.replayed || e.byPatch {
			t.Errorf("pairing %d must be marked a replay and never a measurement of content: %+v", i+1, e)
		}
	}
	// Each original paired with a DIFFERENT commit on main: one replay
	// claimed twice would be len 2 and still a squash.
	seen := map[string]bool{}
	for _, e := range eq {
		if seen[e.note] {
			t.Errorf("two originals paired with the same commit on main: %+v", eq)
		}
		seen[e.note] = true
	}

	// And the outcome a later pass reports: no strand, no re-filed block.
	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Blocked() {
		t.Errorf("a branch replayed in full was still reported blocked, so the block re-files: %q", o.Reason)
	}
	if !o.Merged || len(o.Equivalent) != 2 {
		t.Errorf("the outcome must account for both commits: %+v", o)
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the branch was moved or deleted by a read-only equivalence check")
	}
}

// ranger-base-ghtov: the SECOND way a faithful replay lands nowhere, and the
// honest half of
// TestMergeBlockedReplayPutsHeadOnTheCurrentBaseFirstAndChecksItLast.
//
// The recipe's every command reads `HEAD`, and a dispatched seat's HEAD is
// where the base stood when it was LAUNCHED (AGENTS.md, "Landing the plane").
// Nothing moved it when the base moved and nothing asked whether the base had,
// so a seat launched before a sibling lands resolves each conflict against a
// base that no longer exists. The replay is faithful in every way the pairing
// can measure — (author email, AUTHOR date, subject) byte-identical, which is
// the only arm a hand-resolved replay leaves — and the prescribed read-back
// comes back EMPTY, because both of its sides are key lists computed from that
// same stale pair. MEASURED under ranger-base-56gd0: the replay 1c9a6f26 was
// written 29 minutes after its base 7dc3313e stopped being main, the close's
// own read-back was empty and said nothing, and the launcher re-filed the same
// block as ranger-base-zr2mf.
//
// So this runs the recipe TWICE over the same blocked branch, in two seat trees
// launched at the same base, with the base moving under both of them
// afterwards:
//
//   - as it was written, reading HEAD. The read-back is empty, and the work
//     lands nowhere — the launcher's own filter refuses it and the pass reports
//     a block, which is what re-files.
//   - with the step the copy now opens with, `git merge --ff-only <base>`.
//     Same originals, same keys, same loop — and it lands, after which the
//     stranded branch is accounted for and the block does not re-file.
//
// The key read-back is asserted EMPTY on both, because that is the whole
// finding: the step a seat was told to stop on cannot tell the two apart, and
// `git merge-base --is-ancestor <base> HEAD` can.
func TestMergeBackReplayOnAStaleLaunchBaseLandsNowhereAndTheFastForwardFixesIt(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "shared.mk", "PHONY := build test\n", "seed the makefile")

	// The stranded branch, and the sibling in the same line that blocks it.
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	const subj, date = "the stranded half (ranger-base-ghtov)", "2026-11-22T19:07:14-05:00"
	commitAtIn(t, tr.Path, "shared.mk", "PHONY := build test a\n", subj, date)
	commitIn(t, repo, "shared.mk", "PHONY := build test verify-x\n", "a sibling bead")

	// The two replay seats, LAUNCHED HERE: each HEAD is the base as it stands
	// at this moment, which is the whole fixture. Both, because the stale run
	// below must not be the thing that makes the fixed run different.
	stale, err := a.EnsureSessionTree(repo, "s-2", nil)
	if err != nil {
		t.Fatal(err)
	}
	fixed, err := a.EnsureSessionTree(repo, "s-3", nil)
	if err != nil {
		t.Fatal(err)
	}
	launch := mustGit(t, repo, "rev-parse", "main")
	for _, s := range []*SessionTree{stale, fixed} {
		if h := mustGit(t, s.Path, "rev-parse", "HEAD"); h != launch {
			t.Fatalf("fixture: %s launched with HEAD at %s, want the base's %s — there is no staleness to measure", s.Branch, h, launch)
		}
	}

	// And the base moves again, after both seats are launched. Nothing in
	// either tree moved with it.
	commitIn(t, repo, "shared.mk", "PHONY := build test verify-x verify-y\n", "a second sibling bead")
	if o, err := MergeSessionWork(a, tr); err != nil {
		t.Fatal(err)
	} else if !o.Blocked() {
		t.Fatalf("fixture: the branch has to be blocked or there is nothing for the recipe to replay: %+v", o)
	}

	// The recipe's per-original loop, run in one seat's tree against whatever
	// its HEAD is — which is the parameter this test varies and the only one.
	replay := func(seat *SessionTree, resolution string) {
		t.Helper()
		originals := strings.Fields(mustGit(t, repo, "rev-list", "--reverse", "main.."+tr.Branch))
		if len(originals) != 1 {
			t.Fatalf("fixture: %d original(s) to replay, want 1", len(originals))
		}
		for _, c := range originals {
			// merge-tree exits NONZERO on a conflict and writes the tree
			// anyway, which is the case this recipe is for.
			out, _ := git(seat.Path, "merge-tree", "--write-tree", "--merge-base="+c+"^", "HEAD", c)
			fields := strings.Fields(out)
			if len(fields) == 0 {
				t.Fatalf("%s: merge-tree wrote no tree:\n%s", seat.Branch, out)
			}
			tree := fields[0]
			if del := mustGit(t, seat.Path, "diff", "--diff-filter=D", "--name-only", "HEAD", tree); del != "" {
				t.Fatalf("%s: the merged tree deletes %q", seat.Branch, del)
			}
			for _, p := range strings.Fields(mustGit(t, seat.Path, "diff", "--name-only", c+"^", c)) {
				b, err := gitRaw(seat.Path, "cat-file", "blob", tree+":"+p)
				if err != nil {
					t.Fatalf("%s: %s out of the merged tree: %v", seat.Branch, p, err)
				}
				write(t, filepath.Join(seat.Path, p), string(b))
				// Both seats must be resolving a REAL conflict, or the replay
				// is a patch-id twin and `git cherry` would pair it without
				// replayKey — the arm this whole class turns on.
				if !strings.Contains(string(b), "<<<<<<<") {
					t.Fatalf("%s: %s came out of the merge clean, so the fixture measures the patch-id arm:\n%s", seat.Branch, p, b)
				}
			}
			commitAtIn(t, seat.Path, "shared.mk", resolution,
				mustGit(t, seat.Path, "log", "-1", "--format=%s", c),
				mustGit(t, seat.Path, "log", "-1", "--format=%aI", c))
		}
	}
	// The recipe's own read-back, both sides of it.
	keys := func(dir, rng string) string {
		t.Helper()
		return mustGit(t, dir, "log", "--format=%ae%x1f%aI%x1f%s", rng)
	}

	// ---- the recipe as it was written: HEAD, and HEAD is the launch base.
	replay(stale, "PHONY := build test verify-x a\n")
	if l, r := keys(repo, "main.."+tr.Branch), keys(stale.Path, "main..HEAD"); l != r {
		t.Fatalf("the prescribed read-back is NOT empty over a stale replay, so this test measures something else:\nleft  %q\nright %q", l, r)
	}
	if _, err := git(stale.Path, "merge-base", "--is-ancestor", "main", "HEAD"); err == nil {
		t.Error("the base is an ancestor of a replay written on the launch base — the fixture did not go stale")
	}
	staleTip := mustGit(t, stale.Path, "rev-parse", "HEAD")
	if mergesCleanly(repo, "main", staleTip) {
		t.Error("the launcher's own filter calls the stale replay landable; the block below would not re-file and there is no bug")
	}
	if o, err := MergeSessionWork(a, stale); err != nil {
		t.Fatal(err)
	} else if !o.Blocked() {
		t.Errorf("a replay written on a stale launch base was landed: %+v", o)
	}
	if h := mustGit(t, repo, "rev-parse", "main"); strings.Contains(keys(repo, h+"~1.."+h), subj) {
		t.Error("the base took the stale replay after all")
	}

	// ---- and with the step the copy now opens with.
	if _, err := git(fixed.Path, "merge", "--ff-only", "main"); err != nil {
		t.Fatalf("the prescribed fast-forward was refused in a clean seat tree with the base checked out elsewhere: %v", err)
	}
	if h, m := mustGit(t, fixed.Path, "rev-parse", "HEAD"), mustGit(t, repo, "rev-parse", "main"); h != m {
		t.Fatalf("the fast-forward left HEAD at %s, want the base's %s", h, m)
	}
	replay(fixed, "PHONY := build test verify-x verify-y a\n")
	// The read-back says exactly what it said about the stale replay — which
	// is why it is not the step that decides anything here.
	if l, r := keys(repo, "main.."+tr.Branch), keys(fixed.Path, "main..HEAD"); l != r {
		t.Errorf("the read-back is not empty over a correct replay:\nleft  %q\nright %q", l, r)
	}
	if _, err := git(fixed.Path, "merge-base", "--is-ancestor", "main", "HEAD"); err != nil {
		t.Fatalf("the base is not an ancestor of a replay made after the fast-forward (%v) — the step the copy prescribes does not do what it says", err)
	}
	o, err := MergeSessionWork(a, fixed)
	if err != nil {
		t.Fatal(err)
	}
	if o.Blocked() || !o.Merged {
		t.Fatalf("the replay the recipe now prescribes did not land: %+v", o)
	}
	// THE CLAIM: the stranded branch is accounted for on the base, so the
	// block does not re-file — which is what the stale run could not buy at
	// any price.
	eq := equivalentOnBase(repo, "main", tr.Branch)
	if len(eq) != 1 {
		t.Fatalf("the stranded commit is accounted for %d time(s), want 1: %+v", len(eq), eq)
	}
	if !eq[0].replayed || eq[0].byPatch {
		t.Errorf("the pairing must be marked a replay and never a measurement of content: %+v", eq[0])
	}
	if o, err := MergeSessionWork(a, tr); err != nil {
		t.Fatal(err)
	} else if o.Blocked() {
		t.Errorf("the block re-files over work the replay put on the base: %q", o.Reason)
	} else if !o.Merged {
		t.Errorf("the pass does not say the base holds the stranded work: %+v", o)
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
