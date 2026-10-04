# ranger-base-9c5bh — the settled skip keeps its branch and loses its line

ranger-base-bknod's three findings, decided. The code half of
ranger-base-eh1kr's reachability measurement: with the fake bd no longer able
to serve an in_progress row out of `bd ready`, dispatch's settled-holder skip
(`rangerhq-zom`) turned out to have no store-driven caller.

## The decision (finding 1)

**The branch stays. The line goes.** `holder != "" && holderStatus != "" &&
!d.Resume` still `continue`s, and prints nothing.

Why it stays: it is not dead, it is *raced*. `holder` is non-empty only for an
in_progress bead assigned to this persona; `bd ready` carries no in_progress
row on any store class (MEASURED 2026-10-03, bd 0.50.3, both classes —
ranger-base-bknod); and since eh1kr the claimed half reaches the fire loop only
through `interruptedRuns`, whose `interruptedRun` declines a settled holder
unless `--resume` asked — under which this branch's own `!d.Resume` is false.
What is left is a holder that settles BETWEEN the scan's `heldSession` walk and
the loop's, or a bare shell the scan offered that has an agent in it by the
time the loop asks. Delete the branch and that race falls through to a
re-prompt on an unattended pass, which is zom's token loop one pass later.

Why the line goes: it was a per-pass report of what is now a race, phrased as
a routine verdict ("stopped on purpose? (--resume re-prompts)"). FOUR pins
asserted it and every one could only reach it through the old fixture's
impossible ready queue; all four were re-aimed under bknod at what the pass
does instead — nothing, with the seat and the holder untouched. The surface
that reports a holder which stopped without closing is the governance
surface's G2 row (`settled:<bead>`, govern.go), once per FINDING rather than
once per pass. `skipSettled` left `refillreport.go`'s counted-reason table with
the line: a refill cannot tally a reason no site reports.

## What the operator loses, and does not

Nothing that was being printed. On every pass a store can produce, the decline
already happened upstream in silence — eh1kr's own notes name it ("A pass with
no flag re-prompts no settled holder at all"), and TestDispatchHeldBeadNotReprompted
pins it. What changes is that the one raced pass through the fire loop is
silent too, and G2 is now the whole of what the operator hears about a settled
holder. That is the honest grain: G2 reads `bd list --status in_progress`
against herdr per tick and says it once per finding.

## The pins (internal/posse/settledrace_qa_test.go, arm 2)

`TestQASettledHolderRacingTheScanIsSkipped` — the race, staged with the lever
ranger-base-rrg2 built for this window (`unhide-when-locked`): the holder is in
the fake's world from the start, hidden from `workspace list` until the
launcher lock is held. The scan runs before `fireLoop` takes that lock, so it
reads a listing without the holder and everything from `reconcileSeats` down
reads one with it. It is the ONLY caller that can reach the branch.

Two guards, because the branch's whole output is silence and so is a pass that
never reached it: the lever must have FIRED (`unhide-when-locked` is removed
when it does, so a listing really was taken under the lock) and the pass must
not have said "no ready work" (the queue was not empty, so the bead really did
reach the loop). The control is the same fixture under `--resume`, which types
into the holder — so the silence above is this branch's decision and not a
fixture nothing could ever have fired.

`TestQAEveryCountedSkipReasonHasAReportingSite` — every `skip*` constant
declared in `refillreport.go` is the first argument of some `skipf`/`skipNf`
call in the package. Parsed, not grepped: a `skipFoo` in a comment or a test is
not a site that reports it. Read from the source because that is the direction
the rot runs — a reason with no caller produces no output to assert on.

MUTATION-CHECKED 2026-10-03:

| mutation | red |
|---|---|
| delete the `holder != "" && holderStatus != "" && !d.Resume` branch | the race pin: `n=1`, `agent prompt` typed into the holder, the claim touched |
| put `skipSettled` back in the counted-reason table | the table pin names it |
| `d.skipf(skipBudget, …)` → `d.skipf("budget window spent", …)` | the table pin names `skipBudget` |

## Finding 2

`govern.go`'s G2 comment quoted the skip's line as something "dispatch
already" does. The DECLINING is still true and is still G2's whole subject;
the quote is gone, and the comment now names where the decline happens
(`interruptedRuns`, plus the loop's raced skip) and says that this row is the
only surface that reports it.

## Finding 3 — the shape, not an edit

The fire loop's three `is.Status == "in_progress" && is.Assignee == persona`
conjunctions are now guaranteed upstream: `heldLane` (interrupted.go) offers
only a claim whose assignee is a lane of ONE at its own holder with the canon
name equal to the assignee as written, and the loop's persona is that lane's
seat. They are KEPT — nothing there may assume its caller — but a mutant that
drops any of the three leaves every pin in the package green. Recorded as a
comment at the site, naming the copy that still is load-bearing: the cockpit's
`d` (`LaunchBead`), which takes the row the IN PROGRESS section displays
(`Bd.InProgressAll`, unassigned rows included) with no upstream filter in front
of it, and which `TestQAOrphanedClaimDoesNotParkAClaimThisPersonaDoesNotHold`
is aimed at since bknod.
