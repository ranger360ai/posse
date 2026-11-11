# This repo's landed check: what it costs, and the incident it is read against (ranger-base-r3pqf)

ADR 0088 D1 says a repo opts in to the landed check by shipping an executable
at `scripts/posse-landed-check`, and D6 says what this repo's copy does: the
prerequisites of the Makefile's `test:` line, derived rather than listed, run
with `make` in `$PWD`. The runner that calls it after every landing is
ranger-base-5fond and the bead filing is ranger-base-ddgfr; this is the
script, measured.

Dated 2026-11-11. Every number below was taken on the D1 host (Linux
7.2.15-1-omarchy, 16 cores, `go version go1.27.3-X:nodwarf5 linux/amd64`,
GNU Make 4.4.1), from this bead's worktree, against detached worktrees of the
two commits the bead names.

## The body, and that it is D6's

`scripts/posse-landed-check --plan` prints what it would run and runs nothing.
In this tree today:

```
posse-landed-check: make test's prerequisites here: fmt-check verify-test-times
  verify-parallel verify-suite-lock verify-pattern-kill-census
  verify-silent-reverts verify-shell-syntax verify-python-syntax tree-check
posse-landed-check: step 1: make --no-print-directory fmt-check
  verify-test-times verify-parallel verify-suite-lock
  verify-pattern-kill-census verify-silent-reverts verify-shell-syntax
  verify-python-syntax tree-check
```

(wrapped here; the script prints each as one line.) Nine prerequisites, one
`make`, in the order the Makefile declares them — so `fmt-check` at ~1.5s is
the first thing that can red, as it is for a seat. `tree-check` and
`verify-silent-reverts` are both on it, which is the bead's first done-when —
and with `fmt-check` they are all three of the doors ADR 0088's Context names:
ranger-base-rulbl's gofmt, ranger-base-quzcd/n8wek's silent-revert audit, and
this ADR's own `corpus-check`, which is a `tree-check` door.

The `test:` RECIPE — the suite arms and the silent-revert sweep that closes
them, about fifteen minutes and three suite slots — is not run. D6 says so and
`TestQALandedCheckRunsThePrerequisitesAndNotTheSuiteArms` pins it over a
planted Makefile whose recipe leaves a witness file behind.

## The derivation, and why make is asked rather than the file scanned

The prerequisites come out of `make -p -n` (print the data base, run nothing)
over the tree's Makefile plus a second makefile holding one no-op target,
which is then the only goal. **MEASURED: 0.072-0.073s over three runs**,
against 1.837s for the first form tried, a bogus goal that make refuses after
printing the data base.
The no-op goal is the one that ships, for two reasons that are not the 1.8s: a
bogus goal exits 2, which is a status the script could not then tell from a
Makefile that does not parse, and on a Makefile carrying a match-anything rule
it would be a recipe the script asked for by accident. It arrives on make's
STDIN (`-f -`) rather than through a process substitution or a temp file,
because under D3 the landed worktree and the build cache are the only writable
things in the cage and `<(...)` wants /dev/fd; a pipe wants neither.

Asking make, rather than scanning the `test:` line's text, buys three things
that a scan gets wrong, and each is pinned as an arm rather than asserted
here:

- a prerequisite named through a variable (`test: $(DOORS)`) arrives expanded;
  a scan plans the goal `$(DOORS)`, which is no target at all;
- make unions the prerequisites of **every** rule line for a target, so a door
  added by a second `test:` line elsewhere in the file is a door; a scan of the
  first line never runs it;
- a double-colon rule prints as `test:: <prerequisites>`, and a scan stripping
  one colon plans the goal `:`.

MEASURED by mutation: replacing the derivation with
`sed -n 's/^test:[ \t]*//p' Makefile | head -1` — the text scan — reds exactly
those three arms and leaves the other ten green. Narrowing only the colon
(`^test:` for `^test::?`) reds the third alone.

## The two runs the bead asked for

Both taken with the bytes that ship, each in a detached worktree of the commit
named:

| tree | what it is | exit | wall | loadavg (1m) before |
|---|---|---|---|---|
| `99cc5324` | ranger-base-in8dr replayed over ranger-base-s64a5 — the incident | **2** | 30s | 3.49 |
| `50e8277a` | ranger-base-s64a5, the base in8dr was landed over | **0** | 49s | 3.85 |

The red's tail, which is the whole of what reaches a bead under D5:

```
    coldexecarms_qa_test.go:408: cxaUncoveredPlanters still waives
      internal/treepins, and `make coldexec-check` has an arm reaching it now:
    ...
FAIL	github.com/ranger360ai/posse/internal/treepins	4.064s
FAIL
make: *** [Makefile:974: corpus-check] Error 1
```

make's own last line names the door, which is why the script prints nothing
after the failing child. The red is the *cheaper* side here, and only because
`corpus-check` sits before `register-check`, `scripts-check` and `pid-check`
in `tree-check`'s prerequisite order — a red is not reliably faster than a
green, it is as fast as the door that reds.

## Every reading, and the 120s ceiling

| tree | run | exit | wall | loadavg (1m) before |
|---|---|---|---|---|
| `50e8277a` | first ever in that checkout — COLD | 0 | **74s** | 4.31 |
| `50e8277a` | warm 1 | 0 | **50s** | 7.67 |
| `50e8277a` | warm 2 | 0 | **52s** | 5.10 |
| `50e8277a` | warm 3 | 0 | **57s** | 5.25 |
| `50e8277a` | warm, shipped bytes | 0 | **49s** | 3.85 |
| this bead's tree | warm, before the environment fix | 0 | **64s** | 6.34 |
| this bead's tree | warm, shipped bytes | 0 | **51s** | 2.44 |
| `99cc5324` | red at `corpus-check` | 2 | **30s** | 3.49 |

**So the warm run is 49-64s, against D6's 120s ceiling and its ASSUMED
50-60s.** The assumption holds and nothing goes back to architecture. The
cold figure, 74s, is what a landing pays when the landed tree's packages are
cold for the build cache, which is every landing that moved them; price it at
a minute and a quarter and the ceiling still holds with room. The loadavg
column is not decoration: these were read beside other seats' suites on a box
whose one-minute average moved between 2.4 and 7.7 across them, and the spread
of the walls is 15s.

The run before that 64s one exited **2 in 7s**, on `verify-parallel`: the arms
of this script's own pin had no `t.Parallel()`. That is the check reading its
own author's tree, and it is also the shape of the thing it is for — a door
seven prerequisites before `tree-check`, red in seven seconds, naming the file
and every test in it. Nothing else in a seat's day would have said so before
`make test`.

## The caller's environment was an input, and that was a bug

The first time the suite ran this script's own pin — under `make test-arm2`,
so with `MAKELEVEL` set — one arm red: the last non-empty line of a red run
was `make[1]: Leaving directory '/tmp/…'`, not the door that died. Under D5
that line is the whole of what reaches the bead.

Pulling on it found the larger one. make takes options out of **`MAKEFLAGS`**
and **`GNUMAKEFLAGS`** and extra makefiles out of **`MAKEFILES`**, so until
this was fixed the caller's environment decided what the check did:

- an inherited `-n` (or `--dry-run`) — every door prints, none runs, exit 0.
  A green over an empty room, from a variable nobody typed at this script;
- an inherited `-k` — the run carries on past the first red, so the tail that
  reaches the bead is a *green* door's output;
- `MAKELEVEL` — the directory banners above.

The script now scrubs `MAKEFLAGS GNUMAKEFLAGS MAKELEVEL MAKEFILES MFLAGS` and
runs make with `--no-print-directory`, so the run is the same from a launcher,
a seat's shell and a Makefile recipe. Three of the thirteen pin arms are that
(`…IgnoresAnInheritedMakeEnvironment`, `…IgnoresAnInheritedKeepGoing`,
`…TailNamesTheDoorUnderAParentMake`), and MEASURED by mutation: removing the
scrub and the flag reds all three.

This is worth saying out loud because the runner (ranger-base-5fond) will
invoke this script from Go, where none of those variables is set, so none of
it would have been found there. It was found by the one actor that runs the
script under a make: its own suite.

## It queues for nothing, which is D2 in practice

Runs 2 and 3 above were taken with **both** box-wide suite slots held by other
seats (`scripts/suite-lock.sh --status`: gwart and laurie), and neither run
waited. Every one of the nine prerequisites is a self-test or a `-run`-filtered
`go test`, and a filter takes no slot by design (AGENTS.md, "The full suite").
So the landed check holds no lock, holds no slot, and makes nobody else queue —
and the reverse is also true, which is what the loadavg column is for: these
walls were read beside other people's suites, not on a quiet box.

## What runs this

`internal/posse/landedcheckplan_qa_test.go`, suite arm 2, **thirteen arms,
0.040s** for all thirteen, over planted Makefiles and never over this tree. The
land check's own pin (ranger-base-n2yaw) argues the shape: a pin that read
this repo's own `test:` line would assert a door list every unrelated landing
can move, and it would be a tree-wide pin inside a package nobody runs whole.
Two of the thirteen are the witnesses that make the rest mean anything — one
proves `--plan` ran no recipe (the same fixture without `--plan` is red), and
one proves a real run leaves the `test:` recipe untouched.
