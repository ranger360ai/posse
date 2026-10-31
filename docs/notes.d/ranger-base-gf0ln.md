## 'posse runtime drift' on the clock: the verify-box ROSTER row (ranger-base-gf0ln)

`ranger-base-f5jbs` built the surface — `posse runtime drift`, a third reader
of `skillsVerifiedDrift` with an exit code in `scripts/verify-box.sh`'s
per-check contract, so that a roster row could be the recipe and nothing else.
Nothing scheduled it. This is that half: the Makefile target, the ROSTER row,
and the three pins the roster census requires.

```
make verify-skills-drift     # every launch profile's skills_verified: against
                             # the CLI installed HERE, now
```

**It is a live-box row and not a tree check.** The fact is which version of
each CLI is on *this machine*, against the version posse measured its skill
discovery on. On a box with none of the runtimes installed the surface answers
`2`, nothing measured, which `verify-box.sh` already refuses to call a pass.

### The runner grew a second way to resolve a roster program

Every other row's program is a `scripts/verify-*.sh` in this checkout; this
one's is an **installed binary**. `run_roster`'s guard was one branch —
`[ ! -x "$root/${cmd%% *}" ]` — so a bare command name was looked for under the
repo root, and the row would have scored `ERROR    posse is missing or not
executable` on a box where the check runs perfectly well. `cannot_run` now
knows the two shapes, and only those two:

| first word of the roster command | resolved | example |
|---|---|---|
| contains a `/` | against `$root`, must be executable there | `scripts/verify-grok-pin.sh` |
| bare name | on `PATH`, via the `command -v` **builtin** | `posse runtime drift` |

`command -v` and not a fork, for the reason the self-test's matcher is a `case`
and its reader a `read` loop: an apparatus that answers nothing must not be
able to make a check report something about the box.

An **absolute** path is the shape that looks like it must work and does not —
it contains a slash, so it goes through the `$root` branch and is looked for at
`<root>/opt/homebrew/bin/...`. `TestQABoxCheckRosterProgramsAreAShapeTheRunnerResolves`
is the pin: absolute is refused, a repo-relative program must exist and be
executable in this checkout, and at least one row must be bare or the PATH
branch is measured by nothing. MEASURED by mutation 2026-10-31 — the row
rewritten to `/Users/.../posse runtime drift` reds the absolute arm *and* the
bare-count arm; the row deleted reds three pins (the two-way census, the
membership rule, the bare-count arm).

**The recipe is the bare command and not `$(POSSE) runtime drift`.** The runner
`eval`s a roster command in a shell, where `$(POSSE)` is command substitution —
the same trap that keeps `verify-nodb-defer`'s two-line recipe off the roster
(ranger-base-d1hax). The schedule's plist already populates `PATH` for exactly
these rows: *"Every check on the roster asks `command -v` for something that
lives outside that — posse, bd, brew, grok, codex"*, and
the `~/.local/bin` that posse is installed into is one of its entries.

**And the roster runs the RECIPE, never `make <target>`.** MEASURED: `make
verify-skills-drift` against this box's installed posse exits **2**, because
make collapses a failed recipe to its own exit 2 — which in this contract reads
as *nothing measured*. The row's per-check statuses survive only because
`run_roster` runs the recipe directly, and that is what
`TestQABoxCheckRosterCommandsAreTheirTargetsRecipe` keeps true in both
directions.

### Three runs, three statuses, all MEASURED 2026-10-31 on this box

Through the real `run_roster` (`--self-test-run` with a one-row
`ROSTER_OVERRIDE`) and a scratch `RHQ_HOME`, so no verdict of a worktree's tree
was published to the live state file G10 reads:

| run | row | verdict |
|---|---|---|
| `posse` installed here (0.5.3+fe3b8036) | `FINDING` | `posse: usage: posse runtime check\|probe\|filing <name>` |
| this tree's `posse`, built to a scratch bin | `FINDING` | 3 of 4 measured, **bob** `2.0.5` vs `2.0.7` |
| `PATH=/usr/bin:/bin` | `ERROR` | `posse is not on PATH -- this row's program is an INSTALLED binary` |

**THE ROW IS RED ON THIS BOX TODAY, for two reasons at once, and both are the
check working.**

1. **The installed posse predates the verb.** `~/.local/bin/posse` is
   0.5.3+fe3b8036 and knows `runtime check|probe|filing`, so the recipe exits 1
   with its usage line. That is a true statement about this machine — the
   binary is behind the tree — and the finding's own output says so in the
   daily log, because `--quiet` suppresses a clean check's output and never a
   finding's. The repair is a promote (`make install`), which is the
   operator's; no persona installs the binary every seat on the box runs.
2. **bob still drifts.** `skills_verified: 2.0.5`, bob reports `2.0.7`
   (ranger-base-lf6it; bob's self-updater is the operator's ranger-base-zneyv).
   grok no longer does — `ranger-base-9f2o6` landed its re-measurement and
   `grok 1.0.52 (0920c8169e27) [stable]` now matches on both sides, so the two
   drifts f5jbs measured are one. The remedy is a **re-measurement**
   (`TestQALiveSkillDiscoveryPerRuntime`, `RHQ_PARITY_LIVE=1`) and never a
   version bump in the declaration, and that probe spends a real turn, so it is
   the operator's per-run call (crew guardrail 1).

The bead said to land the row knowing it reds rather than weaken the check, and
that is what landed. A red row on a box that is not what the repo says it is
costs a reader one line; a row that waited for the box to be clean would have
been a control written and scheduled by nobody, which is the defect
`scripts/verify-box.sh` exists for.

### The self-test grew arm 6

`verify-box.sh --self-test` is 25 arms now (23 before this). The new pair is
the two halves of `cannot_run`: a bare command **on** PATH runs — the witness
is the fixture's own output line, not the exit status, because a runner that
silently skipped an unresolvable row would also exit 0 — and a bare command on
**no** PATH is an `ERROR`, which is arm 5's rule in the one shape arm 5 cannot
express. The fixture gets a directory of its own, prepended to PATH, because
under `$tmp/scripts` it would be reachable as a repo-relative path too and the
arm would pass on the branch it is not measuring.

MEASURED by mutation 2026-10-31: `cannot_run` reduced to the old single branch
reds arm 6 (`exit 1, want 0`) and leaves its control green; the PATH branch
hard-wired to "runnable" reds the control (`output does not carry [is missing
or not executable]`) and leaves arm 6 green. Each arm fails for its own cause.

`boxcheck_qa_test.go` names the arm in `TestQABoxCheckSelfTestStillFires`
beside the two verdict arms: the count check is a floor of ten, so deleting
arm 6 would leave 24 arms passing and the one roster row of that shape
unmeasured. The same test's "it has ten" sentence is now "the floor is ten" —
a count held in two places drifts, and the script prints its own.

### verifybox_qa_test.go: the seam now carries both roster shapes

`TestTheScriptWritesWhatThisReaderParses` is the one test that runs the shipped
script and reads the file back with `VerifyBoxFreshness` instead of planting
bytes. Its roster planted four repo-relative rows; it plants five now, the
fifth a bare name on a scratch `PATH`, asserted back as `VerifyBoxOK` with
counts 2/2/1. MEASURED by mutation: with `cannot_run`'s PATH branch inverted
the run writes `onpath: error` and the counts read 1/3/1.

### Files

- `Makefile` — `verify-skills-drift`, beside the grok and codex version pins
  (same class of fact, different declaration), and `.PHONY`.
- `scripts/verify-box.sh` — the ROSTER row and its paragraph, `cannot_run`, the
  detail line that says where the run looked, self-test arm 6 and its control.
- `internal/treepins/boxcheck_qa_test.go` — the membership row (eight live-box
  checks now), the program-shape pin, the named arm.
- `internal/posse/verifybox_qa_test.go` — the bare-name row in the seam test.
