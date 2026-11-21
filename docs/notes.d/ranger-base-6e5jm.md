# `cmdUpgrade`'s other two plan assignments, held at the call (ranger-base-6e5jm)

Three beads in a row answered "this derivation cannot be reached from a test"
the same way: move it into a function and pin the function. The function then
has a pin and its CALL does not. ranger-base-1fsc9 did it for ADR 0052 D1's
repo filter, ranger-base-r1zdr for the report's two loop readers, and
ranger-base-xtg6z closed the third call site — the one line that calls
`upgradeLoopReaders`. This bead is the two assignments one and two lines above
it, which xtg6z measured as still green and filed rather than took:

```go
plan.Personas = a.ListAgents()                                                   // S1
plan.Repos    = append(plan.Repos, upgradeHookWallRepos(a.SweepHookWallIdentity())...) // S2
```

Both are now pinned at the call, one pin each, in
`internal/posse/upgradeverbplan_qa_test.go`. Nothing was added to
`upgrade.go`: `r1zdrVerbRun` (ranger-base-xtg6z,
`internal/posse/upgradereportloops_qa_test.go`) already drives `a.cmdUpgrade`
over `upgradeSeams` and a `d4Fixture` all the way to the report, and that is
the whole seam either pin needs.

## Why the fixture hid both

`d4Fixture.run` assigns `plan.Personas` and `plan.Repos` outright
(`upgradeverify_test.go`) rather than deriving them, so every D4 pin states
its claim over a roster and a repo list the FIXTURE supplied. That is right
for those pins — a roster read from the ambient home would make each of them
depend on whose box the suite is on — and it is exactly what makes the verb's
own two assignments deletable green: the fixture's assignment stands in for
the verb's everywhere a pin looks.

The fix is to make the App the only source of the asserted value. Each pin
declares the thing on the App it hands the verb — two persona files in
`a.AgentsDir` for S1, a `beads_visibility:` block for S2 — and asserts a
report that is not derivable from anything else in the fixture.

## What each loses, and why S1 is the graver one

S1 is **ADR 0092 D4's roster**. With the line gone, `upgradeRead` is called
with an empty persona list on both sides of the swap, so D4's second bullet —
`posse gates <p>` moving 0 → non-zero, or gaining a DEGRADED/REFUSED row — is
evaluated over an empty map and passes vacuously. A new binary whose wall is
red for every persona in the home installs and stands, and the step that
exists to roll such an install back reports that nothing is newly red. So the
pin's subject is the VERDICT and not the roster line: the new binary is red
for one of the two personas this home declares, and the claim is that the
install is undone.

S2 is **D4 amendment 1's repo list**. With it gone, `upgradeHookWallLine`
falls to its no-repos branch and prints `posse gates install-hooks <repo>…` —
an absent instruction rather than a wrong one, which makes it the milder of
the two. ADR 0052 D1's own hole, the one ranger-base-1fsc9 closed, is in the
filter (`upgradeHookWallRepos`) and stays closed there; what S2 loses is that
filter's OUTPUT reaching the report. The pin therefore declares two
checkouts, one real git repo and one path that is not there, and asserts the
line names the first by path, does not name the second, and is not the
placeholder.

## MEASURED 2026-11-21, this box (linux 7.2.15-1-omarchy, go1.27.3, gilfoyle seat, uid 1001)

Tree at acdaf81f plus this bead's pin file. Filter, the one xtg6z's close
recorded, which selects both new pins (`^TestQA(The)?Upgrade`):

```
^Test(QA)?(Live)?Box(Loop|Census|Proc)|^TestQATheVerbsBoxLine|^TestLiveBoxCensus|^TestUpgrade|^TestQA(The)?Upgrade
```

Every arm applied one textual mutation to `internal/posse/upgrade.go` by
deleting the named line, ran that filter, restored the file and checked its
sha256 was unchanged (two scratchpad batteries run by path, per AGENTS.md).
Walls are wall-clock for `go test` including the build; package time was
4.0-4.7s throughout.

BEFORE — the escape reproduced at this tree, with the new pin file moved
aside. The control is the same filter with no mutation:

| arm | mutant | rc | wall | reds |
|---|---|---|---|---|
| — | control, no pins, no mutation | 0 | 12s | — |
| S1 | delete `plan.Personas = a.ListAgents()` | 0 | 12s | **nothing** |
| S2 | delete the `plan.Repos` append | 0 | 13s | **nothing** |

AFTER — the same two arms with the pin file in place:

| arm | mutant | rc | wall | reds |
|---|---|---|---|---|
| S1 | delete `plan.Personas = a.ListAgents()` | 1 | 13s | `TestQATheUpgradeVerbSnapshotsEveryPersonaTheHomeDeclares` |
| S2 | delete the `plan.Repos` append | 1 | 12s | `TestQATheUpgradeVerbPrescribesTheReRenderOverTheReposConfigDeclares` |

Each arm reds exactly one pin and nothing else under the filter, and each reds
on the line that owns it: S1 on "installed a binary whose wall is newly red
for persona `ops` and let it stand", S2 on the missing path and on the
placeholder.

THE LICENSING ARM, because a filter cannot say "nothing catches this". S1
mutated, this bead's pin file absent, over the WHOLE of arm 1 — every pin over
this area is arm 1, so no other arm answers either: `go test -count=1 -timeout
40m ./internal/posse`, **ok 248.387s, rc=0, zero FAIL lines**, at a loadavg of
9.38 at start. Nothing in the package caught it. That is measured for S1 and
not for S2: S2's green is the filter's, and what the package-wide arm licenses
for it is the class — the same function, the same derivation shape, the same
pins — and not that one line.

And the clean whole of arm 1 WITH both pins in it, so the pins are known not
to red anything else and nothing else in arm 1 reds behind them: ok 244.863s,
rc=0, zero FAIL lines, taken with a `make tree-check` overlapping most of it,
which is why it is not a reading of a quiet box and does not need to be.
`make tree-check` itself: green, 2m8.9s, under that run.

## Two things the pins had to be written around

- **The roster count is three, not two.** `upgradeReading.Line` is the only
  thing that prints `gates N persona(s)`, and the run prints three readings:
  D4's pre-swap snapshot, then the verify block's `before:`/`after:` pair, the
  first two being the same reading standing where a reader who has to roll
  back will look for it. A first draft asserted two and red on the real
  report.
- **A fixture must not plant the string its pin searches for.** S2's
  assertion is a path search over the whole report, so the two G12 rows the
  fake binary raises name `~/src/x` and neither configured path. `upgradeRead`
  only counts those rows, so a row carrying the asserted path could not have
  reached the report anyway — but a fixture that plants it is one nobody can
  read as evidence later.

## Not closed here

Nothing. `cmdUpgrade`'s plan assignments are now three for three: the roster,
the repo list, and the loop readers (ranger-base-xtg6z).
