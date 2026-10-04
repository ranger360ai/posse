# The heaviest named-package run on the box took no suite slot (ranger-base-7zng1)

2026-10-04. Found while verifying ranger-base-5no4s; filed by the verify lane
with both halves measured, and ruled here.

## What happened

A clean `make test` on an unrelated change went red in arm 3:

```
--- FAIL: TestWatchPulseArmedLogsBlockedSession (135.11s)
    pulse_test.go:303: watch never returned after cancel
    pulse_test.go:374: watch never returned   (TestWatchPulseUnarmedNoTicker)
FAIL  github.com/ranger360ai/posse/internal/posse  1459.560s
```

Both failures are one shape: a 30s deadline on `Watch` returning after its
context is cancelled, driven at 20ms/40ms intervals. Alone under their tag the
same two tests pass in 0.93s and 0.55s — 3% of the deadline they blew through.
Arm 3 took 1459.6s under the load and 424.1s without it, 3.4x. Nothing in the
change was near pulse or watch.

The load was one concurrent `go test ./internal/treepins`.

## Why that package is a full suite wearing a named package's clothes

`internal/treepins` holds the pins whose subject is the repository TREE, so
they fork `go build`, `go vet`, `git` and the gate scripts over the whole
repo; `TestQAEverySuiteArmTypeChecks` alone type-checks and vets all three of
`internal/posse`'s suite arms, compiling that package three times over.

`scripts/suite-lock.sh` serializes a full package TREE — `./...`, `all`,
anything ending `/...` — and a tagged arm, and deliberately exempts a `-run`
filter and a named package, because those "are the runs a person does while
thinking, they are seconds long". This one is not seconds long, and that
exemption made it invisible to the runs that queue while making nobody else
queue — the class ranger-base-uvzjk built the lock for.

## Measured

| reading | value | environment |
| --- | --- | --- |
| the package, warm build cache | **363.159s**, 639 pins green | 2026-10-04, this box, go1.26.5, `-count=1`, one sibling treepins run alongside, loadavg ~20-30 |
| the package, cold arm builds | **679.089s** | 2026-10-03, same box and toolchain, `-count=1`, uncontended (ranger-base-7zng1's filing) |
| the arm-tags door alone (`-run '^TestQAEverySuiteArmTypeChecks$'`) | **4.22s** real (1.16 + 1.11 + 2.10s of vet) | 2026-10-04, warm build cache |
| wall readings for this package at or above 60s, across every session transcript on the box | **n=327**, median **393.5s**, mean 431.5s, max **1501.6s**; 274 at or above 300s, 38 at or above 600s, 11 at or above 900s | 2026-10-04, corpus of 7 days |
| unfiltered `go test ./internal/treepins` commands typed | **140**, from **52 sessions**, over 7 days (41 on 09-11, 26 on 09-28, 47 on 10-03) | same corpus |
| minutes in which two DIFFERENT seats each started one | **5** (the last of them 2026-10-04T02:40, while this bead was being worked) | same corpus |

300s is `scripts/test-times.sh`'s own `SLOW_PACKAGE_SECONDS` line, so the
median unfiltered run of this package is a package the suite wrapper is
already built to complain about — and it was costing that outside the queue.

Re-run the wall distribution (the corpus grows):

```sh
# the two backslashes are deliberate: the stamp is a JSON \t in the file,
# so what grep must match is a backslash followed by a t.
grep -ho 'posse/internal/treepins\\t[0-9.]*s' ~/.claude/projects/*/*.jsonl |
	sed 's/.*\\t//;s/s$//' | sort -n
```

Readings below 60s are the filtered runs (median 1.65s over 585 of them);
nothing filtered came within an order of magnitude of 60s, which is why the
threshold separates the two populations rather than cutting one.

The command census reads the same corpus for Bash `tool_use` blocks, splits
each command on the separators a shell would split on, keeps the segments
whose head word is a command and that name this package, and sorts them by
whether they carry `-run`/`-test.run`. That is `scripts/suite-entry-census.py`'s
method narrowed to one package; the three traps it documents (segments not
lines, a `-run` filter is not a suite, the trend not the total) apply here
unchanged.

## Ruled

**The slot.** `suite_lock_wanted` now names this one package, so an
unfiltered run of it takes a slot. The exemption is not withdrawn: a `-run`
filter returns from the first loop and never reaches the rule, so `make
tree-check`, the arm-tags door inside `make test`'s own recipe and every
focused pin run are exactly as unqueued as they were — which is also why
there is no nested-acquire case to handle, nothing that runs inside a held
slot reaches the rule.

It is a NAME and not a cost heuristic. argv cannot be asked what a run will
cost: the same command measured 363.2s warm and 679.1s cold, and no pattern
can read the build cache. A measured name is a claim a reader can check.

The sibling candidate — a bare `go test ./internal/posse`, which is arm 1 of
a ~950s package and unqueued for the same reason — is deliberately left
alone. The asymmetry paragraph in `suite_lock_wanted` argues it (ADR
ranger-base-qp1hm's decision about the run a person does while thinking), and
reversing it is an architecture call, not a line added beside this one.

Arm 5c of `scripts/suite-lock.sh --self-test` is the evidence, and arm 5 —
"a single-package run takes no slot", over `./internal/posse` — is its
control. Either one alone is green over a rule that is a withdrawn exemption
or no rule at all. `internal/treepins/suitelock_qa_test.go` requires both arms
by name.

**The deadlines.** The five 30s bounds in `internal/posse/pulse_test.go` are
one named constant, `pulseHangGuard`, at 90s. They are HANG GUARDS, not
budgets: the distinction and the number are
`hookwallsweep_qa_test.go`'s (ranger-base-isq3, which left an identical
`Watch`-never-returned guard at 90s deliberately), and the reasoning is that a
ceiling ~100x the real cost is no longer a number a loaded box can reach while
still turning a wedged `Watch` into a tap dump naming the line instead of a
package-wide `go test -timeout` panic. Two of those five were the red above;
the other three are the same shape in the same file, and leaving them at 30s
would have been filing the next flake rather than fixing this one.

The pair are not a budget in the other direction either: the tests end on
their own evidence (a tap line, a pass count, a released park), never on the
timer, so a slower box spends no extra wall clock for the wider ceiling.
