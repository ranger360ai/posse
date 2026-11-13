# ranger-base-0wvu6 — an arm that read the whole box, and a fixture that was never alive

*2026-11-12 · gwart · `scripts/reap-test-tmp.sh --self-test`, finding 4's rig.*

Every number below is MEASURED on this box (linux 7.2.15 x86_64, go1.27.3,
uid 1001, /tmp a 14G tmpfs) on 2026-11-12, unless it says otherwise.

## What was wrong

The rig for ranger-base-2smyx finding 4 — `claude -p` must stay out of
`claude_procs`, or the blindness control refuses the claude-scratch class for
a reason that is not blindness — measured its claim as the DIFFERENCE of two
box-wide censuses taken about a second apart:

```
n0=$(count_claude_procs)       # every */claude process on this box
"$fakeclaude" -p 30 & ; sleep 1
n1=$(count_claude_procs)       # wants n1 == n0
"$fakeclaude" 30 & ; sleep 1
n2=$(count_claude_procs)       # wants n2 == n0 + 1
```

The counter counts every process on the box whose `/proc/<pid>/exe` ends in
`/claude`, which is its job and is what the blindness control is built on. So
the subtraction above is over a population this box moves all day: a sibling
seat's launch between two readings shifts both of them, and both arms red
together — `verify-reap-test-tmp` being a prerequisite of `make test`, the
seat loses its whole suite before a package compiles, to a message that names
the reaper. Caught 2026-11-11 (`10 -> 11 -> 12`, the rig's own contribution
`0 -> 0 -> +1`) and again 2026-11-12 in ranger-base-r2uxm's run (`12 -> 13`),
both passing on a rerun minutes later.

**And the `-p` arm was vacuous.** The fixture was a copy of `sleep(1)`, run as
`claude -p 30`:

```
$ cp "$(command -v sleep)" ./fakeclaude && ./fakeclaude -p 30
./fakeclaude: invalid option -- 'p'
rc=1
```

Gone in under a millisecond, well before the rig's `sleep 1`. The arm was
therefore reading "the process I started is not counted" over a process that
had already exited, and would have read exactly the same with
`claude_could_register` deleted. A delta over an absent process and a delta
over an excluded one are the same number, which is why the fix is not a
longer sleep or a tolerance.

## What it is now

- The production counter reports **which pids it counted**, on its own line
  below the count and derived in the same loop, so the two cannot disagree.
  That line is also what a human chases after the `REFUSED` message, which
  until now gave a number and no way to look at it.
- The arm asks the question it means: **is my fixture's pid in that set**.
  No baseline, no second census, nothing of anyone else's traffic in the
  answer.
- The fixture is a copy of **`tee(1)`**, because `-p` has to be an argument it
  SURVIVES: `tee -p` is a real option (`--output-error=warn-nopipe`) and `tee`
  with no file operand idles on stdin. Its stdin is a fifo held open by a
  `sleep` whose pid the rig also keeps, so neither process is left behind.
- Both arms assert the fixture is **alive at the moment of the census**
  (`kill -0`, a builtin — an assertion arm's matcher must not fork,
  `TestQANoAssertionArmDecidesThroughAForkedMatcher`). A dead fixture is
  reported as a broken rig, never as a passing exclusion.
- A third arm compares the count to the pid list **within one run**, so the
  evidence the other two decide through is itself pinned, and a missing or
  unparseable line is caught where it cannot masquerade as "my pid is not in
  the list".

## The flake, reproduced and gone

A harness that starts one new ambient, countable claude-exe process every 3
seconds for the length of a `--self-test` run — what a box of seats does on
its own, concentrated:

| rig | interlopers | result |
|---|---|---|
| HEAD before this change | 11 | `FAIL … changed claude_procs from 15 to 16`, `FAIL control: … from 15 to 19, want 16`, exit 1 |
| after this change | 8 | all three claude arms ok, `all arms pass`, exit 0 |

The old spelling's failure text is byte-for-byte the shape both incident
reports carry, so the reproduction is of the reported defect and not of a
cousin.

## The arms are not vacuous

Four mutants of the script, each demanding that the named arm reds:

| mutation | arm that red |
|---|---|
| `claude_could_register` deleted (`if true`) | the `-p` arm: "a live 'claude -p' run (pid N) IS in claude_procs" |
| the exe match broken (`*/claude-never`) | the control arm: "a non-print claude process … is NOT in claude_procs" |
| the pid list no longer accumulated | the agreement arm: "claude_procs says 12 but the script reported 0 pid(s)" |
| the fixture back to a copy of `sleep` | both membership arms: "was already gone when the census ran … the rig is what is broken" |

The last row is the one the old rig could not produce: that mutation is the
old rig's own fixture, and under the old spelling it was green.

`go run ./cmd/checkorphans` clean after every run above.
