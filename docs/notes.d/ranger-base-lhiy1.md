# The watchdog on the detector, and the three readings it is gated on (ranger-base-lhiy1)

*2026-11-12 · developer · code bead under ADR 0088 D6.3 (amendment
ranger-base-7idk4), on the wall ranger-base-wti8j split · adds
`internal/posse/landedhealth.go` and nine pins*

## What this is

ADR 0088 D6.3, built: a second consecutive unanswered judged run files one
`-t bug` P1 `-l architecture` bead, an answered run over `landedCheckCeiling`
files one of the same class with its wall, and an answered run while such a
bead is open comments `answered at <tree> in <wall>` on it. Closing is
architecture's, so the filer has no close arm at all — the same structural
pin D5's filer carries, extended to both files.

The record is D6.3's own, `posse.landedCheckQuiet = <tree> <n>`. D4's key is
untouched, because D5 reads its green value as the far edge of every window a
red bead names.

Three things in the build were decisions rather than transcription, and this
fragment is those three.

## 1. The store is asked only when the run has something to say

D6.3's table reads, taken literally, as "every answered run asks whether a
health bead is open". That is one `bd list` per landing on a healthy main, on
the pass that JOINS this goroutine — and `TestAGreenLandingSpendsNoBdCall`
(ADR 0088 D5's own cost pin) exists to refuse exactly that: *"a rule that
asked the store a question per green landing would spend the launcher's own
wall on a repo where nothing is wrong."*

So the read is gated on the run having something to say: a counter that was
not zero (a silent stretch just ended), or a wall over the price (a crossing
stretch is running). Everything the ADR's four rows ask for is still reached
— a silence-filed bead is commented on by the next answered run, and a
crossing bead by the next crossing — and what is given up is a comment on an
answered run that is inside the price with nothing on the counter, where the
open bead already carries its `answered at` line from the run that ended its
stretch.

MEASURED: removing that gate (`M11`) reds `TestAGreenLandingSpendsNoBdCall`
and nothing else, which is the cost claim being pinned rather than asserted.
An ordinary answered landing on a healthy main spends no bd call and no git
write under D6.3 — only the one `git config --get` that reads the counter.

## 2. "Both `said` lines" is held in memory, and the bead says when it is not

D6.3 wants the filed bead to carry both unanswered runs' sentences, and
spells its record `<tree> <n>` — which has no room for a sentence. The
previous sentence is therefore held per repo on the checker
(`landedChecker.quietSaid`), touched only from the goroutine the run loop
runs on, which is `saidOwnCopy`'s rule and for its reason.

A crossing is two runs of one launcher process by construction
(`Dispatcher.Run` is long-lived, ADR 0028 §1), so the ordinary case has both.
ASSUMED: the launcher restarting between the two silences is rare. When it
happens the bead says the earlier sentence is not recorded and why, rather
than inventing one or filing with one line where the reader expects two.

## 3. A reset is `<tree> 0` and never `git config --unset`

The filing is gated on the counter write landing (ranger-base-w9w37's rule,
which D6.3 names): the COUNT is the silent row's whole dedupe, so over a
record that cannot be written every later silence recomputes the same n in
memory and would file again — one P1 per landing for one broken detector,
which is the ranger-base-4v4r6 shape with a new subject.

That makes "the write failed" a reading the rule acts on, and `--unset`
destroys it: `git config --unset` exits non-zero for a key that is not there,
so a clear-when-already-clear and a clear-that-failed are the same answer.
`<tree> 0` is a write that can only fail for the reason the gate cares about,
and an absent key parses as zero anyway, so the two readings stay one.

**And the gate is only measurable if the FIRST silence landed.** The naive arm
— hold `.git/config.lock` from the start and judge three times — measures
nothing: the record never advances past absent, every run computes n = 1,
and nothing is filed because the threshold is never reached. The ungated
mutant survives it. `TestSilencesOverAnUnwritableCounterFileNothingUntilItLands`
lets silence 1 land (`<tree> 1`), takes the lock, and then every run reaches
the threshold and is refused by the gate alone — where `M8` files a bead per
landing.

## The fixture gotcha: D4's cache decides whether a silent run happens at all

A silent stretch starting from a FRESH repo re-judges the same tree forever,
because silence records nothing, so `once`'s cache test
(`had == tree && prev != landUnanswered`) never matches. A silent stretch
starting from a repo that already has a verdict on record needs a new tree
per run or the second run returns before it reaches the judge — which is how
`TestASilentJudgeAndARedMainAreTwoBeadsInTwoLanes` first read "0 open
architecture beads" over a rule that was working.

## Mutants

All eleven died, one change at a time, over a committed tree with the
baseline asserted clean before each (MEASURED 2026-11-12, arm 2, the nine
pins plus D5's cost pin, ~6s per run):

| mutant | reds |
|---|---|
| `landedQuietThreshold = 1` — file on the first silence | the two-silence pin, the counter-gate pin |
| remember this run's sentence before reading the previous one | the two-silence pin |
| a further silence falls through and files | the third-silence pin |
| `over := false` — never read the price | the ceiling pin, the second-crossing pin |
| `ceilingOf` ignores its field | the ceiling pin, the second-crossing pin |
| never reset the counter | the answered-after-silence pin |
| never comment on the open bead | the answered-after-silence pin, the second-crossing pin |
| ignore the counter write's error | the counter-gate pin |
| `landedHealthLabel = landedFileLabel` — file into D5's lane | the two-lanes pin |
| `len(f) < 2` — read a record of any shape | the parse-table pin |
| ask the store on every answered landing | D5's `TestAGreenLandingSpendsNoBdCall` |

The seam mutant is worth its line: `landedChecker.ceiling` exists because a
240s wall is unreachable inside a test, so the arm D6.3 exists to file would
otherwise have no spelling. It is a field with a zero default, production
reads the constant, `TestTheCeilingSeamDefaultsToTheShippedPrice` holds that
at both constructors, and it can only change WHEN this rule files — the kill
(`landedCheckLimit`) has no seam at all.
