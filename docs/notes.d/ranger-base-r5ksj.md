# A cost cap that also decided the case it was sized for (ranger-base-r5ksj)

`ciJobsVetCap = 3` bounds how many head-of-page `failure` runs one ci-watch
reading will ask the jobs endpoint about (ranger-base-rdi79's job-level vet).
The bound is right and stays: a queue broken for everybody tops the run list
with phantom after phantom, and walking 100 of those would put 100 `gh`
children on a dispatch pass.

What it also did, until 2026-10-05, was decide the verdict at its own
boundary. Past the cap the loop broke and the next run STOOD as `gh` reported
it — red — so a 4th consecutive queue-only run was a verdict about the branch
and filed a P1. The cap's own doc comment named the case and then argued
against itself:

> 3 because the class is 1 run in 300 and three in a row is already a fact
> about GitHub rather than about this branch. Past the cap the run stands as
> gh reported it.

If three in a row is already a fact about GitHub rather than about this
branch, the 4th is **more** of that fact and not less. And the run it
convicted was the one run on the page the reading had deliberately declined to
fetch the jobs of — not a red that might be a phantom, but a verdict
attributed to a page nobody looked at.

## The fix is a direction, not a mechanism

The abstention it needed already existed and was already tested: the
`len(verdicts) == 0` branch in `ReadCI`, a could-not-READ with `NoGate` false,
pinned by `TestReadCIAbstainsWhenEveryRunItCanSeeNeverRan`. So the cap-exit
routes into that same branch. No new field on `CIState`, no new state machine,
and `ciAbstain` already prints the Why with the set-aside evidence in it.

```go
cappedOut := vetted == ciJobsVetCap &&
	len(verdicts) > 0 && reds[0] && verdicts[0].Conclusion == "failure"
if len(verdicts) == 0 || cappedOut {
```

It is the vet loop's own continuation conditions with the counter **at** the
cap instead of below it — "the cap, and only the cap, stopped the loop" —
because every other way of stopping already reaches the right answer and has
to keep it. The two Whys differ in what they can claim: an empty window looked
at everything there was, while the capped one says which run it declined to
convict so a reader can go look.

## Note the safety direction is the opposite of rdi79's

rdi79 demotes a red only on positive evidence, because a suppressed genuine
red leaves no trace anywhere. Here the positive evidence is already in hand —
three consecutive runs whose jobs pages were read and found queue-only — and
the only question is what to do about the uninspected next one. Abstaining on
it suppresses no red anybody measured; it declines to assert one nobody looked
at. The two rules agree on the principle and differ only in which side of the
boundary the evidence sits on.

## `!stood` is not a term, and that is pinned

An earlier draft of `cappedOut` carried a `!stood` flag for "no run stood
inside the cap". It is redundant: `break` fires before the `for` post
statement, so a loop that stopped on a run that stood leaves `vetted` at most
`ciJobsVetCap-1` and can never read as a cap-out. That is an invariant of the
loop's **shape**, which no condition states — so it is pinned by a test
(`TestReadCIStillReadsAVerdictWhenTheCapsBudgetRanOutOnOne`, first subtest)
rather than restated as a condition that would look load-bearing and fool the
next reader into keeping it.

## MEASURED: three mutations, 2026-10-05

Each term earns its place, and the second subtest is the one nothing else
covered. Run from `internal/posse`:

| mutation | what reds |
|---|---|
| `cappedOut := false` (the old direction) | `TestReadCIVetsAtMostTheCapAndOnlyAtTheHead`: `a verdict past the cap: red=true latest="sha00003" — sha00003 was never looked at` |
| drop `vetted == ciJobsVetCap` | 6 tests, including `TestReadCIKeepsAFailedRunWhoseJobActuallyFailed` and `TestReadCIAbstainsOnARunListPageTheIndexServedFromBehind` — every red in the suite becomes an abstention |
| keep only `vetted == ciJobsVetCap` | `…CapsBudgetRanOutOnOne/exactly_the_cap_in_phantoms_over_a_green` **alone**: `abstained with a green run in plain sight under the phantoms` |

That third row is the regression the extra terms exist to prevent, and it is
the worse bug of the two: an outage of exactly `ciJobsVetCap` runs would hide
the gate **clearing**, which is this file's founding incident in miniature
(191 consecutive reds over five days, nobody looking). A gate that can neither
go red nor go green during an outage is not a safer gate.

## The pinned sentence was reversed, not deleted

`TestReadCIVetsAtMostTheCapAndOnlyAtTheHead` asserted the disputed direction
in as many words — `green past the cap: the run that was never vetted must
stand as gh reported it`, with `st.Latest` = the first unvetted run. Its cost
assertions are untouched (`ciJobsVetCap` jobs calls, `ciJobsVetCap`
set-asides, and nothing asked about the run past the cap); only the direction
flipped, to `Known()` false, `NoGate` false, `Latest` naming no run, and a Why
that names the run it declined to convict. The name is kept because
[ranger-base-hxcqe](ranger-base-hxcqe.md) cites it as the pin on the behavior
this bead disputes, and a renamed test would leave that citation dangling.

## The one abstention that repeats

`ciAbstain` says each line once per process, keyed on `dir` + the Why. The
capped Why names the run it declined to convict, so a sha is in that key and
this abstention — alone among them — prints again when the tip moves. That is
wanted: during the outage each new push is a new run starved by the queue, and
a line per starved tip is the only trace those commits get anywhere, since a
run that was set aside on an otherwise-green gate leaves none (`QueueOnly` is
read on the red and abstain paths only). The empty-window Why carries no sha
and still says itself once.

## The occurrence that produced this

The streak that reaches the cap did not happen on 2026-10-05, and by one run.
Inside GitHub's declared Actions outage (component `major_outage`, opened
19:11:58Z) the head of the page held two consecutive queue-only runs at 21:05
— 37372180663 (`64f18288`, four of six jobs starved) then 37367869180
(`eebe9737`, three of six) — with 37362765576 (`bab60e51`) green beneath them
under `filter=latest`. Two demotions against a cap of three; the green below
was there only because 37362765576's one starved job had been re-run. rdi79's
census sized the class at 1 run in 300 (0 queue-only reds in the 300 runs
through 19:21:00Z). Inside the outage it was 3 of 3, all cut at 15m00s /
15m03s / 15m03s. A sustained outage with pushes continuing is the condition,
and it recurs.

The cost per occurrence was one false P1 and one dispatched session, which is
why this was filed rather than fixed on the spot — and the reproduction did
not need an outage to supply a fourth phantom:
`TestReadCIWalksPastTwoConsecutivePhantomsToTheVerdict` (c5cf6832) is built
from those two runs' real jobs pages, and mutating the cap down to 1 reds it
on exactly the complaint.
