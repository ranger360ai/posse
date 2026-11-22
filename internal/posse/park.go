package posse

import "strings"

// A PARK, and what reading it costs (ranger-base-3s2ig, from the finding in
// ranger-base-zlvyk).
//
// A park is what a push hold does to commits that must come off the base: the
// held tip is given a ref of its own and the base is RESET out from under it.
// The work survives, nothing reaches it from the base, and whoever answers the
// hold reads it there.
//
// WHAT IT LEAVES BEHIND IS THE DEFECT THIS FILE EXISTS FOR. A park removes
// nothing from the session branches already cut from that tip: every live seat
// whose branch reaches a held commit is now a carrier. When such a seat closes
// its bead, the base has moved, so `merge --ff-only` refuses — and the REPLAY
// behind it (worktree.go, MergeSessionWork) rebases the branch onto the reset
// base and fast-forwards, which puts the held commits back under new shas.
// MEASURED 2026-11-22 in this repo, read-only: the ff refused and
// `git merge-tree --write-tree` over the same two commits exited 0, so the
// replay was clean and the landing would have gone through. Nothing in that
// chain asked about parks, and no other guard can answer for one: the held
// commits are on the base under no sha at all, so equivalentOnBase pairs
// nothing, and a sanitized re-commit of the same work is a different commit
// count with a different author date, so it pairs nothing either.
//
// WHAT A PARK IS SPELLED AS, which this code INHERITS AS AN ENFORCEMENT INPUT
// and is the reason that sentence is here rather than implied. The park is a
// HAND procedure: posse writes none of these refs, and 7 of them stood here
// when this guard was written (MEASURED 2026-11-22,
// `git for-each-ref refs/heads/parked`). So a park spelled some other way is a
// park this guard cannot see, and the rename that would disarm it is a
// procedure's edit rather than a code change.
//
// The alternative weighed was a namespace posse owns — refs/posse/parked/*,
// beside blockedPinPrefix and refs/posse/merge-blocked/*, which this very
// chain already writes — and nothing could silently rename THAT. It was
// rejected for one measured reason: nothing writes it. A guard reading a
// namespace with no writers refuses nothing while looking exactly like a
// guard, which is worse than one coupled to a convention that is at least
// real.
//
// SO THE PREFIX IS ONE CONSTANT AND THE PROCEDURE SAYS SO. Nothing documented
// it when this guard was written; the handoff this bead filed for that
// (ranger-base-jv2q3) landed docs/runbooks/push-hold-park.md, which names the
// prefix and the consequence of spelling it otherwise, and
// internal/treepins/parknamespace_qa_test.go, which reds if this constant, the
// carrier census's glob (scripts/park-carriers.sh) and that page ever stop
// agreeing. Moving the namespace is still one edit here — it is that pin, and
// not this comment, that makes somebody do the other three.
const parkRefPrefix = "refs/heads/parked/"

// parkedInRange names a park that holds one of the commits base..branch would
// put on the base, as (the park's ref, that commit). ("", "") is the ordinary
// answer and means no park holds any of them — including the two cheap ways to
// get it: no park refs at all, and a park whose commits the base already
// reaches (`rev-list <park> --not <base>` is then empty, which is a park that
// has been lifted and needs no sentence).
//
// An error is a question that could not be ASKED, never a park that is not
// there: its one caller turns it into a refusal, on this chain's own rule that
// nothing lands on a reading git would not give (landingGateReason,
// constitutionOnBranch).
//
// The operand is the BRANCH and not the tree's HEAD, because the branch is
// what both landing arms move the base to — `merge --ff-only <branch>` takes
// it directly, and the replay rebases the branch and fast-forwards onto it.
// A tree whose HEAD is off its branch is refused further down for a better
// reason (attachForReplay), so there is no arm here that lands a commit this
// range does not hold.
//
// Deterministic in what it names: the refs come back in refname order and
// `rev-list` newest-first, so a commit held by two parks is reported against
// the same one every pass — a block whose sentence moved between passes would
// re-file a handoff per pass (priorMergeBlocked dedupes on the title, but the
// restatement reads the body).
func parkedInRange(repo, base, branch string) (string, string, error) {
	parked, err := git(repo, "for-each-ref", "--format=%(refname)", parkRefPrefix+"*")
	if err != nil {
		return "", "", err
	}
	// Fields and not a line split: a refname carries no whitespace, and this
	// runs on a path where the repo may be the thing that is broken.
	parks := strings.Fields(parked)
	if len(parks) == 0 {
		return "", "", nil
	}
	landing, err := git(repo, "rev-list", base+".."+branch)
	if err != nil {
		return "", "", err
	}
	inRange := map[string]bool{}
	for _, c := range strings.Fields(landing) {
		inRange[c] = true
	}
	if len(inRange) == 0 {
		return "", "", nil
	}
	for _, p := range parks {
		held, err := git(repo, "rev-list", p, "--not", base)
		if err != nil {
			return "", "", err
		}
		for _, c := range strings.Fields(held) {
			if inRange[c] {
				return p, c, nil
			}
		}
	}
	return "", "", nil
}

// parkName is a park's ref as a human reads it back to git: `parked/<name>`,
// which is what `git log`, `git branch -D` and a rebase onto it all take.
func parkName(ref string) string { return strings.TrimPrefix(ref, "refs/heads/") }
