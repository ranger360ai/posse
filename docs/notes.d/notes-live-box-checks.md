## The live-box checks, and the one command that runs them (ranger-base-51z8j)

Most `verify-*` targets in the Makefile assert **this checkout**. Seven assert
**this machine**, and until 2026-09-06 nothing ran any of them.

Measured across the tree that day, before this section's own two targets
existed: of the 21 `verify-*` targets there were then, four are
prerequisites of `make test` (`verify-test-times`, `verify-parallel`,
`verify-suite-lock`, `verify-silent-reverts`) and so run in CI; two more have
their *script* run by a target a person types (`verify-gate-freshness.sh
--warn` at the end of `make install`, `verify-detection.sh --check-install` at
the end of `make install-detection`); the remaining fifteen executed only when
a person typed them. No aggregate target listed one as a prerequisite, no
workflow named one, no LaunchAgent ran one. The QA tests that *do* execute
these scripts — `bdpin_qa_test.go`, `grokpin_qa_test.go`,
`hookfreshness_qa_test.go`, `credentialpaths_qa_test.go` — run them against a
scratch `HOME` and a stub `PATH`, which is right for pinning the logic and is
exactly why none of them noticed: not one asks the box anything.

That is how a version pin can lapse and stay quiet for a day (ranger-base-k4lza).
A one-shot remediation of a condition that **regenerates** is not a control;
only a recurring detective check is.

```
make verify-box              # the seven live-box checks, ~40s, read-only
make verify-box-self-test    # ten arms proving the aggregate can still fail
```

**Why it is not a `for` loop over `&&`.** Each of these scripts already
separates three verdicts — `0` ok, `1` finding, `2` nothing measured — and the
third is the one an aggregate gets wrong. A machine with no codex answers `2`
correctly, and scoring that green is the same silence one level up. So
`verify-box` classifies each check, never stops at the first red, exits `1` on
any finding or unexpected status, and exits `2` saying **NOTHING MEASURED**
when every check answered `2`.

**CI cannot be its home.** A GitHub runner has none of these things installed
and would answer `2` to all of them — correctly, and uselessly. The home has to
be on-box, which is a launchd install, and *where a finding surfaces* is one
decision for all seven rather than seven decisions. Both are the operator's and
both are asked on ranger-base-0x1wc. Until one is answered this is still a
command a person types; what changed is that it is **one** command.

**The roster cannot fall behind in silence.** `scripts/verify-box.sh` carries
the roster *and* a table of every other `verify-*` target with the reason it is
not on a clock, and `boxcheck_qa_test.go` fails on a target in neither. Two
exclusions are load-bearing rather than housekeeping:

- `verify-runtime-walk` **spends a real turn** on the runtime under test. It is
  event-triggered by design — before switching a lane back onto a runtime, and
  after a version bump — and a schedule would be spending on a clock.
- `verify-pid-deny-set` as a *target* reads `HOME_DIR=examples`, i.e. this
  repo's own seed PIDs: a tree check wearing a live-box name. Its live readers
  (`--live`, `--settings`) are off the target on purpose, and `--live` answers
  `1` whenever a session is mid-bead behind a PID edit — correct, and a
  nuisance generator on a clock.

**And the census is over `scripts/` too** (ranger-base-bbl6r, which is this
section's own defect in the one shape its guard could not see). Both lists
above are keyed on a **Makefile target name**, so a check shipped as
`scripts/verify-*.sh` with *no target at all* was in neither list, named by
nothing, with a green board over it. Two were in the tree the day this landed —
`verify-ghost-composer.sh` and `verify-orphan-report.sh` — and the close that
wrote the census counted 21 `verify-*` *targets* and never enumerated
`scripts/`, so neither had ever been classified. Neither is schedulable today
(one needs an uncaged seat, one needs a container), so nothing was
unrun-and-needed; the gap was the guard's. A third table, `UNTARGETED`, now
carries every `verify-*` script that is no target's recipe with the reason it
cannot have one, and the QA test globs the directory against it both ways.

Three tables of *sentences* are checked rather than read, for the same reason:

- an exclusion that says `make test` runs it is checked against the `test:`
  prerequisite line. Drop the prerequisite and the check runs **nowhere** while
  the table still says CI has it.
- a roster **command** is checked against that target's own recipe. A recipe
  that gains or loses a flag would otherwise drift in silence; only a *renamed*
  script is loud today, because the runner's `[ ! -x ]` arm makes that an ERROR.
- every row's reason or command must be non-empty, so a row cannot satisfy the
  census by existing.

**It remediates nothing, and that is pinned.** Every rostered script is
read-only by its own contract; the aggregate kills no process, deletes no file
and fixes nothing. A finding prints the line for a person to type. This is
`verify-gate-freshness`'s rule applied one level up — a persona-writable tree
that could repair the box would be that tree, one flag away.

