# The upgrade report's two loop lines, held by a report pin (ranger-base-r1zdr)

ADR 0092 D8's claim is one sentence — "Both lines print, and neither claims
the other's reach" — and until this bead nothing in the suite held the wiring
that reaches either renderer. The mechanism is
`internal/posse/upgrade.go`'s `upgradeLoopReaders` and the pins in
`internal/posse/upgradereportloops_qa_test.go`. This page holds the
measurement; the code holds the claim.

## What was held, and what was not

The renderers were witnessed. `BoxReading.Sentence` has eight mutants over
`boxloops.go` behind it, each reddening its own pin
(docs/notes.d/ranger-base-is1qp.md), and `WatchImage.Sentence` has
`watchimage_qa_test.go`. What was unwitnessed was everything between
`cmdUpgrade` and those two sentences: the two assignments that give a plan its
readers, and the two prints that put their output in the report.

laurie found it verifying ranger-base-is1qp's close (ranger-base-msir3,
finding 2) and the closer had named it in `boxloops_qa_test.go` without filing
it. The reason it was a gap at all is the shape of the verb: `cmdUpgrade`'s
reads go in through `upgradeReads`, but a PLAN is built behind a real build
stamp, a real checkout and a box lock, and `d4Fixture.run` — the one fixture
that drives a plan — assigned the plan's fields outright rather than deriving
them. So no pin reached the wiring from either end. This is the same shape,
and gets the same answer, as `upgradeHookWallRepos` one derivation over
(ranger-base-1fsc9): a package function the pins can call, with nothing added
to move it.

## MEASURED 2026-11-21, this box (linux 7.2.15-1-omarchy, gilfoyle seat, uid 1001)

Filter: `^Test(QA)?(Live)?Box(Loop|Census|Proc)|^TestQATheVerbsBoxLine|^TestLiveBoxCensus|^TestUpgrade|^TestQAUpgrade`
— 98 top-level tests, 3.167s clean. Each arm applied one mutant to
`internal/posse/upgrade.go`, ran that filter, and restored the file
byte-identical (`scripts` were a scratchpad battery, per AGENTS.md).

BEFORE the pins in this bead:

| arm | mutant | rc | wall |
|---|---|---|---|
| B1 | delete `plan.Box = func() BoxReading { … }` | 0 | 3.174s |
| B1L | delete `plan.Loop = func() (WatchImage, bool) { … }` | 0 | 3.248s |
| B2 | the box line's print made unreachable | 0 | 3.367s |
| B2L | the per-instance line's print made unreachable | 0 | 3.360s |

B1 and B2 are laurie's two; B1L and B2L are the same two mutants over the
per-instance line, which the bead predicted were open as well and which this
run confirms. Four ways for D8's sentence to become false with a green suite.

AFTER, with the same filter plus the new pins — every arm rc=1, each on the
pin that owns its line:

| arm | reds |
|---|---|
| B1 | `TestQACmdUpgradeInstallsBothLoopReadersOnItsPlan` — "no Box reader on the plan" |
| B1L | the same pin — "no Loop reader on the plan" |
| B2 | `TestQATheUpgradeReportPrintsBothLoopLines…` and the wiring pin — "no `dispatch (box):` line" |
| B2L | both — "no per-instance `dispatch:` line" |
| B3 | the wiring pin — the box reading was about the home, not the plan's target |
| B4 | both — "2 per-instance loop lines and 0 box lines" |

B3 and B4 were added here and are not in the bead: they are the mutants that
keep a line in the report and change what is in it, which a pin asserting only
"both lines are present" would pass. B3 is D8's subject rule (the file the
install renames is box-wide, so the reading is against `plan.Target` and never
this home's anything). B4 prints the per-instance renderer twice, which reads
as two readings and is one.

## What the pins assert, and what they deliberately do not

Three tests, all through `upgradeRunPlan`, which is where the D4 pins are:

- the report carries both lines, each from its own reader, in that order
  (per-instance first — it is the loop the delegate's own next seat comes
  through — then the box line, then D6's record line last), and neither line
  carries the other's scope clause;
- a plan carrying no reader prints neither line, which is the control that
  makes the first test about the readers and not about something else in the
  report;
- `upgradeLoopReaders` installs both, the box reading's subject is
  `plan.Target`, and the loop reading's subject is the `*App` it was given.

Nothing here asserts what either sentence says beyond its own reach clause.
That is the renderers' job and they have it.

One choice worth naming: the subject assertions are on the READERS and not on
the printed lines. The box line names its path only when the census answered,
and two correct readings name none — off linux it abstains
(`boxloops_other.go`), and from a seat in a child user namespace every
same-uid peer is refused (ranger-base-rph5z). `BoxReading.Path` is the subject
the reading was about in every branch including those two, so it is the
assertion that holds on every box this suite runs on.
