# ranger-base-3s2ig — the landing chain asks about parks now, and what reading one costs

2026-11-22. The enforcement half of ranger-base-zlvyk's finding. The detective
half is that bead's; this is the guard.

## The defect

A push hold PARKS the commits it refuses: the held tip gets a ref of its own
under `refs/heads/parked/` and the base is reset out from under it. That leaves
every session branch already cut from that tip reaching held commits, and
`MergeSessionWork` (internal/posse/worktree.go) had no step that knew it. The
fast-forward of such a branch refuses, because the base has moved — and the
REPLAY behind it rebases onto the reset base and fast-forwards that, which puts
the held work back under new shas.

MEASURED 2026-11-22 by ranger-base-zlvyk, read-only in this repo
(`git merge-tree --write-tree` writes a tree and moves no ref): on the live
carrier the ff refused and the merge-tree exited 0, so the replay was clean and
the landing would have gone through.

No guard already in that chain can answer for it, and this is why it needed its
own: a held commit reaches the base under no sha at all, so `equivalentOnBase`
pairs nothing — and a sanitized re-commit of the same work pairs nothing either,
because on the live pair it was ONE commit against the held TWO with an author
date 38 minutes later, so neither the patch-id arm nor the
(author email, author date, subject) arm matched.

The fast-forward is the same defect and the bead's title does not name it: a
seat whose base has NOT moved since the park lands the held commits by sha, with
no rebase involved at all. That arm is not an inference — it was OBSERVED, and
recorded on ranger-base-8h17w by the push gate the same evening: after the reset
the launcher fast-forwarded a CLOSED carrier seat's branch onto local main (the
reflog line is a `merge` of that branch), the held commits came straight back,
and the gate refused them again on both the content scan and the patch-id.
One park, three carriers, one landing inside the hour. Both arms are pinned.

## The two decisions this bead was filed to make

**What a park IS.** `refs/heads/parked/` — the convention that exists, read as
one constant (`parkRefPrefix`, internal/posse/park.go). It is an enforcement
input this code inherits rather than owns: the park is a HAND procedure, posse
writes none of these refs, and 7 of them stood here when the guard was written
(MEASURED 2026-11-22, `git for-each-ref refs/heads/parked`). A park spelled
some other way is a park the guard cannot see.

The alternative weighed was a namespace posse owns (`refs/posse/parked/*`,
beside the `refs/posse/merge-blocked/*` this very chain already writes), which
no convention could silently rename. It was rejected for one measured reason:
nothing writes it. A guard reading a namespace with no writers refuses nothing
while looking exactly like a guard, which is worse than one coupled to a
convention that is at least real.

Saying so where the procedure is WRITTEN was the other half, and it was filed
as a handoff (ranger-base-jv2q3) rather than assumed: nothing documented the
prefix at all when the guard was written. That bead landed while this one was
verifying — docs/runbooks/push-hold-park.md names the prefix and what spelling
it otherwise costs, AGENTS.md carries the seat's one line of it, and
internal/treepins/parknamespace_qa_test.go reds if this constant, the carrier
census's glob and that page ever stop agreeing. So the drift this fragment was
going to have to warn about has a pin instead.

**What the refusal COSTS.** It blocks. A park is somebody's answer to a push
hold; a landing that undoes it costs a second park, a second hold bead and a
revert on the base, and the branch cannot be repaired from here, because a
carrier is checked out by a LIVE seat whose ref nothing may move. So it joins
every other verdict in that chain — reported, never repaired — and one park can
print a standing block on every pass for as long as it stands. That is the cost,
and it is paid where the alternative is the base holding work a hold refused.

It is asked after the range is counted and before every arm that can move
anything: both fast-forwards, the replay, and `refreshGeneratedIndex`'s commit
on the branch. One consequence worth stating: it preempts the equivalence report
below it, so a carrier whose own work HAS reached the base under other shas is
reported blocked rather than Merged, and its tree is not retired while a park
stands over it. That is the side to be on — the retire of a carrier tree during
a park is what ranger-base-zlvyk measured the re-landing out of.

## The sweep's half

`blockStillStands` (internal/posse/landsweep.go) asks the same question FIRST,
in the same order the landing asks it, which is that function's whole contract.
Without it the arms below reach `mergesCleanly`, which answers "clean — worth
asking git for real" over a carrier (that is the defect's own measurement), so
every pass re-entered `MergeSessionWork` to be refused again. Nothing in the
park arm writes the session tree: the reads are a `for-each-ref` and `rev-list`s
in the REPO, which is ADR 0058 fact 4's constraint. A read that FAILS falls
through rather than standing — the authority on an unaskable question is
`MergeSessionWork`, which refuses on it in words.

And the block has the pass that can see it stop being true, which every reason
there is held to: the park ref is lifted, or its commits reach the base, and the
question answers empty on the very next pass.

## What it costs per landing

MEASURED 2026-11-22, this box, against the 7 live parks:

- `git for-each-ref refs/heads/parked/*` — **9ms**. On a repo with no parks
  that is the whole of it: one read, and the guard returns.
- the 7 held-set reads (`rev-list <park> --not main`) — **92ms** for all seven,
  11 held commits between them. The guard stops at the first park that holds a
  commit of the landing's range, so that is a ceiling.
- one `rev-list <base>..<branch>` for the range, which the chain above it
  already reads as a count.

One of those 7 parks holds NOTHING off main (its commits were landed), which is
not a hypothetical control arm: a lifted park is an ordinary state and must not
block anybody, and it does not — the held set is what is read, never the
existence of a ref.

## Mutation runs

Driven from a scratchpad script, restoring from a copy rather than
`git checkout --` (which would take an uncommitted fix with it).

- the `parkedLanding` guard dropped from `MergeSessionWork` → 3 reds:
  `TestAParkedCommitIsNotFastForwardedBackOntoTheBase`,
  `TestAParkedCommitIsNotReplayedBackOntoTheBase`, and the
  `a park holds a commit the landing would put back` row of
  `TestMergeBlockedReasonsNeverPromiseTheBranch`.
- the `parkedInRange` arm dropped from `blockStillStands` → 1 red:
  `TestAParkKeepsAStandingBlockStandingUntilItIsLifted`, and only that one.
- restored: green both times, and the pre-mutation baseline was green, so
  neither red was the fixture.

Each pin also carries its own wrong arm in-test: the park is lifted, nothing
else changes, and the same call lands — including through the replay, with
`o.Rebased` asserted, which is the escape itself.

## Not closed here

Nothing. Both halves of the finding this came out of are closed:
ranger-base-zlvyk, which carries the carrier census, and ranger-base-jv2q3,
which this bead filed and which landed the procedure page and the pin that ties
both readers of the namespace to it.
