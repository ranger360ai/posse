# `posse upgrade` installs the report's loop readers, held at the call (ranger-base-xtg6z)

ranger-base-r1zdr moved the two loop-reader assignments out of `cmdUpgrade`
into `upgradeLoopReaders` so that a pin could witness them, and witnessed the
function. The one line that calls it was then held by nothing: the gap
narrowed from four mutants to one and moved one line up, out of the function
the new pins reach and into the verb that calls it. laurie found that
verifying the close (ranger-base-0qhkn → this bead). The mechanism is
`r1zdrVerbRun` and `TestQATheUpgradeVerbPutsTheReportsLoopReadersOnThePlanIt-
Runs` in `internal/posse/upgradereportloops_qa_test.go`; this page holds the
measurement.

## The verb IS reachable, and r1zdr's close was wrong about why it was not

r1zdr closed saying `d4Fixture.run` builds the plan itself, "so the assignment
in cmdUpgrade is reachable from no test in either direction". The conclusion
held — nothing reached it — but the reason did not, and the reason is what
decided the shape of that fix and of this one.

Three things were already true:

- the three live readings a plan is built behind are PARAMETERS, not calls.
  That is the whole of what `upgradeReads` is a seam for (ADR 0092,
  ranger-base-anczm), and `upgrade_test.go`'s `upgradeSeams` supplies all
  three: a build stamp, a source checkout, and a box lock that locks nothing.
- `TestUpgradeTargetsTheRunningBinaryAndTheLagsCheckout` already enters
  `a.cmdUpgrade` that way and reads the report's FIRST line. What stops it
  there is its own stub — a red `make test`, which is that pin's subject —
  and not anything structural about the verb.
- `d4Fixture` already supplies the other half: the fake binary at the target,
  the fake binary the build produces, the probe, and three GREEN step stubs.

So a run that is both reaches the report's LAST lines, which is where the two
loop lines are. `r1zdrVerbRun` is those two halves on one line and nothing
else; no seam was added to `upgrade.go` for it, and the helper asserts the
read order (`bin,lag,lock`) so that a report produced down some other path is
not mistaken for the verb's.

## MEASURED 2026-11-21, this box (linux 7.2.15-1-omarchy, go1.27.3, gilfoyle seat, uid 1001)

Tree at 9c2a95d0. Filter — and this is the spelling that selects the pins in
that file, which the one r1zdr recorded does not, every name there beginning
`TestQA`:

```
^Test(QA)?(Live)?Box(Loop|Census|Proc)|^TestQATheVerbsBoxLine|^TestLiveBoxCensus|^TestUpgrade|^TestQA(The)?Upgrade
```

102 top-level tests, 3.4s package time, ~11s wall including the build. Every
arm applied one textual mutation to `internal/posse/upgrade.go`, ran that
filter, restored the file and checked its sha256 was unchanged
(`mutants.py`, a scratchpad battery run by path, per AGENTS.md). Walls below
are wall-clock for `go test` including the build, which is why they are not
comparable with r1zdr's package-time figures.

BEFORE the pin in this bead — the residual laurie filed:

| arm | mutant | rc | wall | reds |
|---|---|---|---|---|
| M1 | delete `upgradeLoopReaders(&plan, a)` from `cmdUpgrade` | 0 | — | **nothing** |

laurie's own arm is the licensing one, because a filter cannot say "nothing
catches this": the WHOLE arm-1 package, `go test -count=1 -timeout 40m
./internal/posse`, ok 250.912s, rc=0, zero FAIL lines, tree at 32bce0e4. Every
pin over this area is arm 1, so no other arm answers either. Re-measured here
over the filter above: rc=0, 3.519s package time.

AFTER, M1 plus all six arms r1zdr recorded, re-run with the two new pins in
place. `verb` is this bead's pin, `fn` is
`TestQAUpgradeLoopReadersPutsBothReadersOnThePlanItIsGiven` (r1zdr's, renamed
here), `report` is `TestQATheUpgradeReportPrintsBothLoopLines…`:

| arm | mutant | rc | wall | reds |
|---|---|---|---|---|
| M1 | the `upgradeLoopReaders` call deleted | 1 | 11.0s | verb |
| B1 | `plan.Box`'s assignment deleted | 1 | 13.9s | verb, fn |
| B1L | `plan.Loop`'s assignment deleted | 1 | 13.4s | verb, fn |
| B2 | the box line's print made unreachable | 1 | 12.8s | report, verb, fn |
| B2L | the per-instance print made unreachable | 1 | 15.7s | report, verb, fn |
| B3 | box read against `a.Home`, not `plan.Target` | 1 | 12.8s | verb, fn |
| B4 | `upgradeWatchLine` in the box line's slot | 1 | 14.9s | report, verb, fn |

M1 reds the new pin and only it, which is the claim. B3 reddening the verb pin
as well is the conditional subject assertion earning its place: on a box whose
census ANSWERED, the box line names the path the run replaced, and a reading
taken against the home does not.

## The two arms that are still green (ranger-base-6e5jm)

The same escape sits two and three lines above M1's, at `cmdUpgrade`'s other
two plan assignments, and this bead measured it rather than fixing it:

| arm | mutant | rc | wall | reds |
|---|---|---|---|---|
| S1 | delete `plan.Personas = a.ListAgents()` | 0 | 13.1s | **nothing** |
| S2 | delete the `plan.Repos` append | 0 | 13.0s | **nothing** |

S1 is ADR 0092 D4's roster: with it gone the verify block judges an empty
roster and passes vacuously, so a red gate on every persona in the home would
not be seen. S2 is D4 amendment 1's repo list, an absent instruction rather
than a wrong one — ADR 0052 D1's own hole is in the filter
`upgradeHookWallRepos` and stays closed (ranger-base-1fsc9); what S2 loses is
the filter's output reaching the report. Both are now cheap, because
`r1zdrVerbRun` is the seam either one needs; filed as ranger-base-6e5jm rather
than taken here, so that this close stays the one finding it was filed for.

## Two sentences re-tensed

Both said the gap was closed when it was narrowed, and they said it twice:

- the renamed pin. `TestQACmdUpgradeInstallsBothLoopReadersOnItsPlan` named
  `cmdUpgrade` in its title and in two failure messages and called
  `upgradeLoopReaders` directly, so a reader who grepped for the verb found a
  green pin that did not reach the verb — the exact reading that hid M1. It is
  `TestQAUpgradeLoopReadersPutsBothReadersOnThePlanItIsGiven` now, which names
  what it measures.
- `boxloops_qa_test.go`'s "it is closed in upgradereportloops_qa_test.go".
  Narrowed, in two steps, and the comment now says which step did what.

`docs/notes.d/ranger-base-r1zdr.md` carries the same correction at its end,
plus the filter it recorded, which selects none of the pins whose reds its own
AFTER table reports.
