# ranger-base-y13h7 — at what depth does a lagging launcher need a human?

*The threshold behind G11 (`launcher_behind_max`, default 16), measured
2026-10-05 on darwin 25.4.0 in `~/src/posse`. The number is re-derivable from
the commands below; every figure is labelled MEASURED or ASSUMED.*

## The bug this is the number for

`posse status` on this box, 2026-10-05T23:4xZ, verbatim and two lines apart:

    launcher · 0.5.0+193c8782 is 101 commit(s) behind main in ~/src/posse — …
      and only installing closes it
    …
    nothing needs a human

The lag sentence asks for a human by name — installing over a binary that is
dispatching a live fleet is a live change reserved to the operator (guardrail
3) — and the summary two lines down says none is needed. The cause was scope,
not a wrong reading: `nothing needs a human` is `GovReport`'s answer for an
EMPTY `GovSet`, and the lag was rendered by a different path
(`LauncherLag.Line`, printed by the status command itself). No governance
condition read it, so four days of passes each printed the figure and nobody
acted on it.

`launcherlag.go`'s header makes a deliberate decision that the reading is "A
READING, NEVER A CONTROL", and that decision is right and untouched: a `go
run` or a checkout build must not be able to stop a fleet. A governance
condition is not a control either — it gates nothing, declines nothing and
installs nothing; it says a human is needed, which is the one thing the
surface it joins is for. The two hold at once.

## What was measured

Two populations, and the first one turns out not to be the bound.

### 1 · this box's install-to-install distances (MEASURED)

Every install of `~/.local/bin/posse` this box can still account for, from two
sources: the loop-start preambles in `$StateDir/dispatch-watch.log{,.1}`,
which print the running binary at every loop start (`possebinary.go`), and the
`posse binary ·` lines in `~/.claude/projects/*/*.jsonl`.

    grep -h 'posse binary ·' ~/.config/posse/state/dispatch-watch.log{,.1}
    grep -rhoE 'posse binary · [^ ]+ · [0-9.]+\+[0-9a-f]+(-dirty)?' \
      ~/.claude/projects --include='*.jsonl'

14 installs, 2026-09-03..10-03, in commit order:

    0b5c1c4 eff354f c592683 9920e75 12dd7b9 6d55e97f 162b341 9c9e7b51
    062b765e aca7786a 8fa107db 1b8ffe2d 763de34f 193c8782

Each one is an ancestor of the next (`git merge-base --is-ancestor`), so the
13 closed episodes have a well-defined depth: the lag the running binary had
reached when the next install replaced it,
`git rev-list --count <stamp_i>..<stamp_i+1>`.

| sorted depths | 9 · 20 · 26 · 34 · 42 · 46 · 48 · 51 · 52 · 79 · 105 · 165 · 180 |
|---|---|
| median | 48 |
| reach ≥ 8 | 13/13 |
| reach ≥ 16 | 12/13 |
| reach ≥ 32 | 10/13 |
| reach ≥ 64 | 4/13 |
| reach ≥ 128 | 2/13 |

The open episode when this bead was filed (193c8782, installed 2026-10-03
15:28) was at 101.

EXCLUDED, as fixtures rather than installs: the three transcript readings of
`0.5.0+02dd122` at 40, 42 and 40 behind on 2026-09-29, 10-01 and 10-04 — a
non-monotonic depth for one stamp over three separate days, and the string is
quoted verbatim in `docs/notes.d/ranger-base-vso72.md`.

The install set is a FLOOR on the number of installs: one whose stamp reached
neither a preamble nor a transcript is invisible here, which makes every
distance above an upper bound. That cuts toward a SHALLOWER threshold, not a
deeper one.

**And it is not the bound.** The median episode (48) is already past the depth
at which this lag has twice cost a session. "Deeper than this box's normal" is
not available as a rule, because normal is the defect.

### 2 · the depths at which the lag has actually cost something (MEASURED)

Both costs are traceable commit by commit with
`git rev-list --count <stamp>..main --before=<instant>`.

**ranger-base-z3hx6 / ranger-base-tvorm** · c592683, installed 2026-09-03
23:54, replaced 07:16.

| event | depth |
|---|---|
| the first fix the fleet was then missing landed — 67effd0, ranger-base-emgdb | 21 |
| the second landed — c3ab918, ranger-base-j8qmj | 31 |
| the cost: a merge-back block filed a FOURTH time, one dispatched session re-deriving a do-not-land verdict two commits on main already held | 34 |

**ranger-base-6pnab / ranger-base-xxgp3** · 193c8782, installed 2026-10-03
15:28.

| event | depth |
|---|---|
| the first of the four ci-watch fixes it was missing landed — 540cc22c, ranger-base-m46kr | 90 |
| the cost: a false ci-red P1 filed against a green gate (2026-10-05T20:08:21Z) | 93 |
| the reading printed in the bead | 101 |

So the shallowest depth at which a fix the fleet needed was already on main is
**21**, and the shallowest depth at which the gap demonstrably cost a session
is **34**.

## The number, and what each alternative costs

**16** — the largest doubling step strictly below 21. That is the property
being bought: at 16 the row is up BEFORE the fleet is missing anything, so an
operator who acts on it installs a current binary rather than closing a hole
that is already open. The steps are the watch log drumbeat's own
(`lagDrumbeat`), because the row's Key carries them and the pulse
fingerprints Keys — so a deepening lag re-prompts at 16, 32, 64 and nowhere in
between, log2 prompts over an episode instead of one per tick, and the pass
line and the row escalate on one cadence rather than two.

| rejected | measured consequence |
|---|---|
| 0 | wrong, and already measured: a binary is built from the tip, and the count at the instant of the build was 0 both times it was read (`launcherlag.go`'s header; the next commit landed 1m47s later) |
| 8 | 13/13 episodes, and it buys nothing — no incident's first missing fix was shallower than 21 — while spending the one episode (9) where the operator installed promptly, which is the behaviour the row wants |
| 32 | 10/13. In the shallower incident the row would first rise after BOTH missing fixes were already on main (21 and 31), two commits before the cost landed: a report of an existing hole, not a warning |
| 64 | 4/13, and SILENT through the whole of that incident, which reached 34. Excluded by measurement rather than by taste |

**It is a floor, not a proof.** 13 episodes, 2 with a cost traced to them.
Nothing here establishes that the other 11 cost nothing: a stale launcher does
not fail — it dispatches, merges back and files beads exactly as designed —
so the absence of a filed bead is not the absence of a cost. Erring LOW is
therefore the cheap direction: a row that rises early costs one install the
shop wanted anyway and clears itself the moment that install lands, while a
row that rises late costs a dispatched session, twice measured.

ASSUMED, and this half is policy rather than measurement: that an operator who
is told "16 behind" will install within the hours it takes the next sixteen to
land. `launcher_behind_max:` is the dial, and `0` means any lag at all is a
condition — the shape an instance that installs from the tip every time wants.

## What the row is

`G11`, class LANE, key `launcher-behind:<step>`, in ADR 0029's table.

LANE and not URGENT on the `backup-stale` rule: ADR 0029 defines URGENT as
"the shop is stopped", and a stale launcher stops nothing — it keeps running,
with the defects its own repo fixed hours ago. LANE still exits `posse status`
non-zero, still draws in the cockpit's GOVERNANCE block and still counts in
the header, which is the whole of what the bead needed. Making the one class
that means stop-everything also mean "an install is overdue" would cost the
pulse the distinction it escalates on.

**It counts over the CONFIGURED checkouts alone** (`beads:`), where the status
LINE counts over those plus the process's working directory. Two reasons, both
in `govern.go`'s `lag()`: a condition is a statement about the fleet this
instance dispatches and the fleet is `beads:`, where a line is owed to whoever
typed the command; and a reading that fell through to cwd would count every
`posse status` pin in the tree against the operator's live `main`, because a
test binary built from a worktree of this repo carries that worktree's stamp —
red per hour rather than per commit (the class ranger-base-rp2y cost a day
to). The gap it leaves is named rather than papered over: on a box whose
`beads:` names no posse checkout the line speaks and the row abstains.

**An abstention raises nothing and is not a partial set.** "Names no commit"
is the ordinary shape of a checkout build, so filing it in `failed` would make
every `go run ./cmd/posse status` exit non-zero over a build working exactly
as designed. The rule it would otherwise fall under is satisfied out loud
elsewhere and unconditionally: `posse status` prints `Line()` in every case,
and the watch preamble says it once.

## Where the pins are

| claim | pin |
|---|---|
| the all-clear cannot print beneath a lagging launcher, with the control arm that proves the fixture can print it | `internal/posse/launcherlaggov_qa_test.go` TestTheAllClearCannotPrintBeneathALaggingLauncher |
| the threshold, both sides of its boundary; LANE; the row name | TestG11FiresAtTheThresholdAndNotBelowIt |
| the Key carries the doubling step, 16/31→16, 32/63→32, 64/101→64 | TestG11KeyCarriesTheDoublingStep |
| the step doubles the CONFIGURED threshold, not a power of two | TestLagBucketDoublesTheConfiguredThreshold |
| a current launcher is silent, including at a threshold of 0 | TestG11IsSilentForACurrentLauncher |
| the config key moves the line; a typo is named and the default stands | TestLauncherBehindMaxIsConfigurable |
| an abstention raises nothing and does not make the set partial | TestG11AbstainsOnAStampThatNamesNoCommit |
| the seam defaults to this instance's own reading | TestShopCheckDefaultsToThisInstancesOwnLauncherReading |
| the governance reading does not fall through to the process cwd | TestTheGovernanceReadingDoesNotFallThroughToTheProcessCwd |
| the line and the row land in one `posse status` view, exit non-zero, no all-clear | `cmd/posse/statuslaunchergov_qa_test.go` TestStatusCarriesTheLauncherLagAsACondition |
| the cockpit hands the check no reading of its own, so a zero `LauncherLag` cannot be wired in as a silent all-clear | `cmd/posse/statuslaunchergov_qa_test.go` TestTheCockpitDoesNotStandTheLauncherReadingDown |

Each of the first set reds when `GovRows`' predicate is short-circuited to
`nil` (MEASURED: 5 of them, plus the cmd-level one), and the cwd arm reds when
`launcherCandidates` is given the cwd candidate unconditionally.
