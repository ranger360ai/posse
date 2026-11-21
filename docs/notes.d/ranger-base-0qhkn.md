# ranger-base-0qhkn — four closes verified, and the rendered shell that is not a shim

Verify bead over four closes: ranger-base-8m9d0 (the die-with-parent arm's
control and the seat's self-report), ranger-base-bekle (G18 matches a pid to
the record or abstains), ranger-base-i36bf (the L1 git shim parses under
darwin's bash 3.2 again, plus a door), ranger-base-r1zdr (the upgrade
report's two loop lines go in through one function).

All four hold on their own checklists. One live finding
(ranger-base-xtg6z), one gap closed here as a test addition, and two
closes whose own residual done-conditions turned out to be readable from CI
after the fact.

## The thing worth keeping: a close's residual can be discharged by a log that did not exist yet

Two of these four closes named a done-condition they could not reach from a
linux seat and said so honestly — ranger-base-8m9d0's "the next ubuntu arm-1
run" and ranger-base-i36bf's "green on a macos arm-3 leg". Both landed on
main, main reached the remote on its ordinary cadence, and by verify time CI
had run on heads carrying each fix. So the first thing to ask of a close
carrying a CI residual is not whether it was honest about it: it is
`git ls-remote --heads origin main` and `gh run list`, because the reading may
already be sitting there. Here it was, for both, within ninety minutes of the
closes.

## MEASURED 2026-11-21, this box (linux 7.2.15-1-omarchy, go1.27.3, 16 cores), tree at 32bce0e4

### ranger-base-8m9d0 — the instrument works, and it has already answered

The live pin from this (uncaged) seat: all four subtests PASS in 2.70s, and
the self-report appears in `-v` with every row — `pdeathsig 9 · ppid N ·
NoNewPrivs: 1 · Seccomp: 0 · Seccomp_filters: 0 · apparmor <read
/proc/self/attr/current: invalid argument> · END`, identical in both death
arms. That reproduces the closer's reading independently.

Three mutants over `internal/posse/wallunixlive_test.go`, each restored
byte-identical:

| mutant | result |
|---|---|
| M1 the death deadline cut to 0 in BOTH arms (the closer's own) | each arm reds on its own naming itself, and the post-loop dump carries BOTH reports |
| M2 the deadline cut to 0 in the STEP arm only — mine | the control PASSES, the step arm reds, and the dump still carries the passing arm's report |
| M3 `anyArmRed = true` taken off the `Errorf` path, both deadlines 0 — mine | the dump goes QUIET, so the flag is load-bearing and the dump is not some other print |

M2 is the one that grades the un-briefed half of that close. The claim is
"whichever arm reds dumps every arm's report, the PASSING ones included", and
M1 — both arms red — is satisfied by a dump that can only print failing arms.
Only the asymmetric arm asks the question, and CI has never produced it.

AND THE CI READING, which is the bead's own purpose rather than its
deliverable: run 44934045502 job 136936698379 (ubuntu-latest, 1) at head
09732acf, and again run 44940074456 job 136955115294 at head aa3c77d0 — both
arms red, both reporting `pdeathsig 0 · NoNewPrivs: 1 · apparmor
bwrap//&unpriv_bwrap (enforce)`, with the post-loop dump present in a job that
passes no `-v`. That is reading 1 of the four-outcome table the close put in
`wuSeatSelfReport`'s doc comment: the runner clears the pdeathsig before the
seat runs, the control has no Landlock layer and reds identically, and the
residual is ranger-base-z56s9's. Posted there.

### ranger-base-bekle — nine mutants, nine reds

Six are the closer's, one per rule; three are mine. Each applied alone, each
red on the pin that owns it, `internal/posse/watchimage.go` and
`internal/posse/upgrade.go` restored byte-identical after each.

| mutant | pin that spoke |
|---|---|
| W1 the stamped-image comparison dropped | G18AbstainsRatherThanJudgeAProcessTheRecordCannotBeMatchedTo |
| W2 the start-time comparison dropped | the same, plus G18ReadsNoVerdictFromARecordWrittenBeforeTheProcessItNames |
| W3 the truncation dropped | both of those plus G18FiresWhenTheInstallRenamesOverALiveLoop and ALoopReadingCoversItsOwnInstance — the live flake this bead is about, reintroduced by its own fix, exactly as the close says |
| W4 a pid nothing could be matched against read anyway | G18AbstainsRatherThanJudge… |
| W5 the record road's own recycled-pid check dropped | the same |
| W6 an abstention returned as a reading | G18ReadsNoVerdict… |
| W7 (mine) the comparison made TWO-SIDED (`.Before`) | five arms of G18Abstains… plus G18ReadsNoVerdict… |
| W8 (mine) `upgradeWatchLine`'s `could not be identified` branch made dead | G18ReadsNoVerdict… |
| W9 (mine) the abstention stops carrying `Why` at the producer | six arms of G18Abstains… plus G18ReadsNoVerdict… |

W4 needs a note for whoever repeats it: `if !checked {` → `if false {` leaves
`checked` declared-and-unused, which is a `[build failed]` and not a kill
(the house rule). `if !checked && false {` is the mutant that compiles.

W7 is the arm that grades the fix's own reasoning rather than its code. The
two comparisons are one-sided because "only one direction is evidence", and a
two-sided version is the plausible mis-edit; it reds six arms, so the
asymmetry is witnessed and not merely written down.

THE KERNEL PREMISE, re-measured rather than inherited: `/proc/<pid>`'s own
mtime IS the task's start as wall clock. `/proc/1` reads
2026-11-08 00:22:46 and `uptime -s` reads 2026-11-08 00:22:46; a process
started one second before the read reads its own start second. So the fix's
read needs neither the boot epoch nor USER_HZ, as its comment says.

And the symptom: `TestQAG18FiresWhenTheInstallRenamesOverALiveLoop` is green
on ubuntu arm 1 at head 52ce199f and again at aa3c77d0, against red at
eeaa4f4a. The fixture half of that (ranger-base-rph5z, c9ca4d6e) is what
stops the pin redding; this close is the production half, and the close says
so rather than claiming the flake.

### ranger-base-i36bf — the asymmetry that justifies the door, and the corpus it does not read

Four mutants, each restored byte-identical:

| mutant | result |
|---|---|
| I1 the pre-fix `renderSeatObjectEnv` planted back (the closer's own) | the structural half reds, 64 findings, the first naming the `-C)` arm darwin's bash named — and `sh -n` stays GREEN over every shim, because bash 5.3.20 parses the shape |
| I2 the pre-fix INLINE form with the POSIX leading paren on all 8 arms | the door stays GREEN: the rule is a wall against the error, not a vote between the two fixes, as the close claims |
| I3 the leading paren taken off `renderHookSkipGlobalScope`'s `(system…)` case — a different renderer, a different file | the door reds, 8 findings |
| I4 the same renderer's `GIT_CONFIG_NOSYSTEM` case, in a second substitution | the door reds, 8 findings |

I1 is the whole argument for the structural half in one reading: the shape the
door exists for is invisible to `sh -n` on every linux box. I3/I4 answer the
coverage question the close does not — the door is not keyed on the one
renderer the bead was about.

AND THE RESIDUAL DONE-CONDITION, from CI rather than from a darwin box:
`TestQAMirrorIsAFourthForcePushSpellingThatOnlyTheVerbRuleCloses` is absent
from the whole macos arm-3 log at head aa3c77d0 (run 44940074456, job
136955115310) — no `Mirror`, no `syntax error` anywhere in it — against the
same job one landing earlier at head 52ce199f (job 136950090705), where it
reds with `line 403: syntax error near unexpected token ';;'` at the `-C)`
arm. The other four reds in that leg are identical in both runs and are
somebody else's (ranger-base-blkse and family). That is the before-and-after
the close could not take.

## The gap closed here (a test addition, the verify lane's own)

`TestQAEveryRenderedShimParsesAndCarriesNoBash32SubstitutionHazard` reads one
directory — a persona's gates `bin/` — so its corpus is shims. The close said
rendered hooks and the gate shell "are their own corpus and this door does not
reach them", having READ them for the class. Measured through the production
renderers:

| rendered body | bytes | `case` | `$(` | heredocs | backquotes |
|---|---|---|---|---|---|
| prepare-commit-msg (`App.CommitGuardHook`) | 85,614 | 11 | 110 | 0 | 16 |
| the gate shell (`renderGateShell`, `<gatesDir>/shell/bash`, outside `bin/`) | 2,893 | 5 | 2 | 0 | 0 |
| pre-push (`const PrePushHook`) | 2,016 | 2 | 1 | 0 | 0 |

The read was right — all three are clean of the hazard — which is why
`TestQAEveryRenderedHookAndGateShellCarriesNoBash32SubstitutionHazard`
(internal/posse/shimparse_qa_test.go) lands green, and is the reason to land
it: a `case` arm added inside one of those 113 substitutions tomorrow is as
invisible as the one ranger-base-i36bf was filed for, and that one cost six
days of every darwin seat's git.

Shown able to fail, three ways, each with the SHIM door staying green beside
it — which is the measurement that says the new pin is not a second copy of
the old one:

| mutant | new pin | shim door | `sh -n` |
|---|---|---|---|
| N1 a hazardous `case` inside `$( )` appended to the prepare-commit-msg body | reds on both its prepare rows | green | green |
| N2 the same in `renderGateShell` | reds on the gate-shell row | green | green |
| N3 the pin's own judge made blind | reds on the synthetic control arm | green | green |

N3 is the arm that matters most: every real row asserts an ABSENCE, so a judge
that found nothing would make the pin green forever. The synthetic hazard body
goes through the same call and has to come back as one finding.

The 16 backquotes are worth knowing about. The scanner knows no heredoc and
gives up on a backquote, and all 16 sit inside single-quoted strings, which it
skips whole — so it reads this corpus clean today, and an UNQUOTED backquote
added to one of these bodies answers through the scanner's third state, which
this pin FAILS on ("could not be read, so it is NOT measured") rather than
passing.

## The live finding — ranger-base-xtg6z (devops, debt)

ranger-base-r1zdr's checklist is met: its four mutants (B1, B1L, B2, B2L), the
two the closer added beyond it (B3 the box read against `a.Home`, B4 the
per-instance renderer in the box line's slot) and one more of mine (B5, the two
print blocks swapped, which reds the order clause) each red on the pin that
owns the line.

What the fix did not close, it moved. `upgradeLoopReaders(&plan, a)` —
internal/posse/upgrade.go:951, the one line in `cmdUpgrade` that installs both
readers — is held by nothing: taken out, `go test -count=1 -timeout 40m
./internal/posse` answers `ok 250.912s`, rc=0, zero FAIL lines. That is the
WHOLE arm-1 package and not a filter, because a filter cannot license "nothing
catches this", and every pin over this area is arm 1. With that line gone the
verb prints neither loop line and ADR 0092 D8's "Both lines print, and neither
claims the other's reach" is silently false again.

`TestQACmdUpgradeInstallsBothLoopReadersOnItsPlan` names `cmdUpgrade` in its
title and in two failure messages and calls `upgradeLoopReaders` directly, so
it witnesses the function and never its caller — a pin wrong about what it
measures, which is why this is live rather than recorded. The closer's reason
(`d4Fixture.run` builds the plan itself) holds in effect even though the
sentence overstates it: `cmdUpgrade` IS called from seven test sites, but every
one is a refusal, a lock claim or the `--classes` early answer, and none reaches
`upgradeRunPlan`.

RECORDED, no bead: internal/posse/boxloops_qa_test.go:727's rewritten comment
says the gap "is closed in upgradereportloops_qa_test.go". It is narrowed from
four mutants to one, not closed. That line and the pin's title are the same
sentence twice.
