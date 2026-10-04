# ranger-base-bknod — the fake bd's `ready` serves open rows, and nothing else

The fixture half of ranger-base-eh1kr, scoped out of it deliberately. eh1kr
taught the pass to read the claimed half of its queue; this is the half that
stops the suite from pretending `bd ready` ever carried it.

## The measurement this rests on

MEASURED 2026-10-03, bd 0.50.3, both store classes, one binary, the same
argv (`ready --json --limit 0`):

| store | holds | `ready` answered |
|---|---|---|
| the shop's SQLite queue (`beads.db` beside its `issues.jsonl`) | 10 open rows + the in_progress bead of the moment | the 10 open rows |
| a hand-written `no-db: true` JSONL store | 1 open, 1 in_progress | the open one |

bd's own help says the same: "Excludes in_progress, blocked, deferred, and
hooked issues."

The fake served `fake-ready.json` WHOLE. So a fixture could put a claimed
bead in the ready queue, and 49 test files did — which left every
in_progress branch of the fire loop (the holder join of ADR 0004 §2, ADR
0008's crew shield, ADR 0030's orphaned-claim tiebreak, rangerhq-zom's
settled skip, `--resume`'s override) green against a queue no store can
produce. ranger-base-eh1kr is what that cost.

## What landed

Three filters compose in the fake's `ready` case now, in this order, and the
order is the contract:

```
fakeBdApplyState       the claim this pass made, overlaid on the canned file
fakeBdReadyDropClosed  dispatch's half: handed out, and `show` says closed
fakeBdReadyOpenOnly    the store's own query: the row's own status is open
```

A row with **no** `status` field is open. That is the convention every
fixture in the suite already writes and the one `fakeBdDropClosed` keeps on
the `list` side; real bd always renders the field, and reading the omission
as "some other status" would empty every ready queue in the package rather
than pin anything.

`fakeBdReadyOpenOnly` is **not** a superset of its neighbour, which is the
thing the next fold will get wrong. `Bd.Unclaim` writes `--status open` and,
with `keepAssignee`, leaves the assignee standing — so the state the status
filter reads says `open` for a row the fake has handed out and whose `show`
answers closed. The status filter keeps that row; the dispatch half drops
it. `TestQAReadyFakeCannotServeAClaimedRow`'s last subtest is the reader.

### The second write: a claim reaches the claimed listing

`fakeBdRecordStatus` (was unnamed; `update --claim` and every `update
--status` now reach it) moves the row in `fake-list.json`, so `bd list
--status in_progress` answers with the bead the fake just handed out and
stops answering with one it handed back. Real bd does; the fake did not, and
it did not show while `ready` served its file whole — a bead claimed in pass
1 came back from `ready` in pass 2 carrying the state overlay's
`in_progress`, so the claimed half never had to exist.

Without it, item 2 of the bead ("a dispatch test that wants a claimed bead
declares fake-list.json") would have meant writing, into a two-pass fixture,
a store that says one bead is both ready-open and in_progress-claimed — the
same class of impossible queue this bead exists to stop serving. With it,
**five** two-pass tests needed no fixture change at all:
`TestQAParitySettleOpenEscalationBoundsTheLoopOnEveryRuntime`,
`TestQAResumeStillRefusesToTypeIntoAGhostBox`,
`TestQAResumeStillWaitsWhenHerdrNamesNoSession`,
`TestQAResumeStillWaitsOnABoxTheStoreDoesNotKnow`,
`TestQAResumeDoesNotRePromptAHolderThatIsWaiting`.

It is also the family's first writer that can CREATE a file rather than
rewrite one, and bd's cwd is not always a fixture repo: a test whose config
carries no `beads:` falls back to the process cwd, which for a test binary
is the package directory. The first version wrote
`internal/posse/fake-list.json` into the source tree, where the next run
with that same fallback would have read it as a store. Guarded on
`fake-ready.json` existing, and pinned by
`TestQAFakeBdWritesNoStoreOutsideAFixtureRepo` (control: the same call in a
real fixture repo records the claim). `fakeBdAddDep` writes
`fake-deps.json` unconditionally and has the same exposure; nothing
triggered it in these runs.

### `claimedRepo`

`qaRepo` for a pass whose queue has a claimed half: `ready`, `list` and
`show` as three bodies, because they are three different questions about one
store and the fixtures that matter are the ones where the answers disagree.
It is the drop-in for every `qaRepo(t, a, <an in_progress row>, <show>)` the
suite carried.

What changes with the move is the GATE the row comes through.
`interruptedRuns` subtracts `bd blocked`, drops a defer in the future, and
offers only a claim whose assignee is a lane of one at its own holder — so a
fixture whose bead is none of those is no longer in the queue at all. That
is the honest answer and it is where three of the per-test decisions below
come from.

## Per-test decisions (item 3)

| test | what it was really asking about | repair |
|---|---|---|
| orphanedclaimnarrow: control, another-repo, non-crew, another-persona | ADR 0030 §1's conjunction in the fire loop | the claimed scan (`claimedRepo`) |
| orphanedclaimnarrow: `DoesNotParkAClaimThisPersonaDoesNotHold` | `is.Assignee == persona` | re-aimed at `LaunchBead` + a control; see below |
| orphanedclaimnarrow: `NeverOverridesAnEvidencedHolder` | `holder == ""` | claimed scan; assertion was VACUOUS and is now `n` + a prompt + a create count |
| orphanedclaim: the park line (3 flag legs), the crashed run | ADR 0030 §1 | claimed scan |
| herdr_test `TestDispatchResume` leg 1 | the resume path | claimed scan |
| herdr_test `TestDispatchResume` leg 2 | the claim-lost skip | the row is now OPEN; see below |
| dispatch_qa `HeldBeadNotReprompted` (3 legs) | rangerhq-zom | claimed scan; leg 1's skip LINE dropped |
| dispatch_qa `AssigneeRoutedBeadReachesInProgress` pass 2 | rangerhq-zom | skip line dropped |
| recordskip `ResumeRePromptsAnUntrustedSettleWithoutClose` | ADR 0013 §4's retry | skip line dropped; the silence is pinned with a before/after prompt count |
| holderjoin `RunRecordHolderIsJoinedUnderAnyName` | the record arm of the join | claimed scan; the normal-pass leg is graded on `n` and the twin, not on a line |
| dispatch_qa `ResumePrefersInProgressBead` | `OrderBeads` under `--resume` | both halves of the queue — a-2 ready, a-1 claimed — so the ordering is pinned across the JOIN of the two reads |
| holderjoin: the five slot/crew/dry-run arms | ADR 0004 §2-3 | claimed scan |
| argvprompt `ArgvResumeIntoALiveSessionStaysTyped` | ADR 0013 §2 | claimed scan |
| coordinator `ResumeLeavesCoordinatorHeldBeadAlone` | ADR 0033 §2 | claimed scan; `heldLane` is the first refusal now and the fire loop's is the second |
| crew `SkipsCrewSessionHoldingTheBeadUnderAnyName` | ADR 0008 through both doors | the table gains a `list` body, one leg per door |
| personaseatidle `IdleHolderWithLiveBeadIsNotAFreeSeat` | `personaActive` | a-2 ready, a-1 claimed |
| dispatch_qa `FakeBdReadyDropsABeadItHasShownClosed` | the fake's own filter | the non-latching arm re-aimed at `Bd.Unclaim`, the write that really reverses a claim |

### Fixtures that could not just move

**An unassigned in_progress row** reaches no pass: `bd ready` never carried
one and `interruptedRuns`' `heldLane` rule drops a row with no assignee.
`is.Assignee == persona` lives in both launchers ADR 0030 names — the fire
loop's copy and the cockpit's `d` — and only `d` can still be HANDED one,
because it takes the row the IN PROGRESS section displays, which is `bd list
--status in_progress` whole (`Bd.InProgressAll`), unassigned rows included.
The arm is asked of `LaunchBead` now, with a control that parks.

**A foreign in_progress claim** reaches no launcher either: not `ready`, and
not the claimed scan, whose `heldLane` rule wants a lane of one at its own
holder. The way a pass MEETS a claim loss is an open bead whose `show` names
somebody else, which is what `Bd.Claim` falls back to — the shape the two
`ClaimLost` pins in dispatch_qa_test.go already use.

**Six fixtures in the reap and refill families became incoherent stores,
and the fix is in the writer, not in them.** `fake-show.json` says `closed`
from the start in all of them — the fake has no event for "the persona
closed it", so an end state declared up front is how that is said — and
recording the claim put the same bead in the claimed listing as
`in_progress`. A store does not answer `show` with closed and `list --status
in_progress` with the same bead, and left disagreeing each pass reaped the
session for a closed bead and then RELAUNCHED that bead out of the claimed
listing, in the same pass, recreating the very session the arm was asking
about.

So `fakeBdRecordStatus` writes nothing for a bead this repo's `show` already
answers closed: fakeBdReadyDropClosed's rule, applied at the write instead
of at a read. It is asked of `show` and never of fake-list.json's own row,
so a row the FIXTURE declared — in_progress in the listing, closed in `show`,
which is `TestQAListAndReadyFakesAreNotOneFake`'s whole discriminator — is
left exactly as written; the clause only ever declines to write.

Patching the six fixtures by hand was the other option and it is a
treadmill: the next fixture written in that idiom hits the same trap, and
the incoherence was mine to begin with. MEASURED, dropping the clause:
`TestAutoReapRetiresAWorktreeSessionsTreeAndBranch`,
`TestAutoReapCommitsThePersonaMemoryAndSpendsNoTurn`,
`TestAutoReapSkipsASessionJustPrompted`,
`TestQARefillSlowEnoughToReapItsOwnSessionStillLaunchesOnePerBead`,
`TestQARefillNamesItsSeatAndSummarisesSkipsInOneLine` and
`TestQARefillNamesTheAccountBrakesRuntimeAndCap` all red, and no fixture in
the suite needed an edit with it in place.

## Mutation-checked

`TestQAReadyFakeCannotServeAClaimedRow`, five mutants, each killed by the
arm it is aimed at (and two of them also by
`TestQAFakeBdReadyDropsABeadItHasShownClosed`):

| mutation | red |
|---|---|
| drop `fakeBdReadyOpenOnly` from the composition | `ready` served a-held, a-blocked, a-deferred |
| status filter BEFORE the state overlay | a bead claimed this pass stayed in `ready` |
| drop `fakeBdReadyDropClosed` as a redundant superset | the unclaimed-with-`keepAssignee` row came back as ready work |
| drop the `fakeBdRecordStatus` call on a granted claim | the claimed listing did not carry the claim |
| drop the recorder's declared-close clause | the six reap/refill pins above |
| treat a row with no `status` as not-open | every bare fixture row left the queue (also red: `TestQAListAndReadyFakesAreNotOneFake`) |

`TestQAFakeBdWritesNoStoreOutsideAFixtureRepo`: removing the guard writes a
store into a directory no fixture declared one in.

And the direction `TestQAListAndReadyFakesAreNotOneFake` no longer kills is
recorded in its own comment as a dated CORRECTION: pointing `ready` at
`fakeBdDropClosed` is now caught by the new pin's last subtest instead,
because a claimed row carries `in_progress` in the state and the status
filter drops it either way. The `list` direction is unchanged and still
killed there.

## Not in scope, found while doing it

Filed for the code lane — see the bead's handoff. In one line each:
`dispatch.go`'s `skipSettled` branch cannot be reached from any store-driven
pass since eh1kr (four pins asserted its line, all four only reachable
through the old fixture); `govern.go`'s G2 comment quotes that line as
something dispatch still does; and the fire loop's `is.Assignee == persona`
tests are now guaranteed upstream by `heldLane`.
