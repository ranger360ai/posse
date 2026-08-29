## The other half: the gate reports its own seconds (ranger-base-7xla)

ranger-base-2ggb closed the first half of this invariant hours before this
bead ran: `make test` carries `-timeout 25m`, `suitetimeout_qa_test.go` pins
the flag and a 15m floor, and `docs/notes.d/ranger-base-2ggb.md` has the
pooled distribution. **That decision stands and is not re-litigated here.**
7xla asked for two things, and its author separated them deliberately:

> Either the gate carries an explicit `-timeout` it can defend […] or
> internal/rhq stops being a single ~9-minute package. […] WORTH DECIDING
> RATHER THAN DEFAULTING: raising the timeout hides the growth. A gate that
> prints the per-package seconds and warns above a threshold keeps the
> signal. That is a call for whoever owns the gate.

2ggb answered the first. This is the call on the second.

### WHAT WAS STILL OPEN

**The pin holds the flag, not the runtime.** `suitetimeout_qa_test.go` fails
if the `-timeout` is dropped or set below 15m. It cannot fail because
internal/rhq got slower. So the package could walk from nine minutes to
twenty-four with every gate green, and the first notice would be the day it
trips — which is the same day, and the same misreading, this bead started
from.

**And a trip still reads as a hang.** 25m makes the panic rarer; it does not
make it legible. A test timeout is a `panic:` plus a full goroutine dump —
exactly what a deadlock in product code prints — and through the house filter
(`| grep -E '^(---|ok|FAIL)'`) it is a bare `FAIL … 601.010s` naming no test
at all. 2ggb's own Makefile comment concedes the cost: "a genuine deadlock
takes 25m to surface".

### WHAT LANDED

`scripts/test-times.sh`, which `make test` now runs the suite through:

    test: verify-test-times
    	scripts/test-times.sh $(GOBIN) test -timeout 25m ./...

**It owns no number.** The budget it reports against is parsed out of that
recipe line, so `-timeout 25m` remains the single source of truth and remains
literally on the line `suitetimeout_qa_test.go` reads. Mutation-checked
against the landed pin, not just alongside it: with the wrapper in front,
dropping the flag, lowering it to 8m, and replacing the whole recipe each
still fail arm 1. A command carrying no `-timeout` at all is reported against
go's 600s default *and said out loud*, which is the condition 2ggb closed.

Three outputs, in ascending urgency:

- every package's seconds and its share of the budget, every run;
- `SLOW PACKAGE(S)` for anything over `SLOW_PACKAGE_SECONDS` (300). A second
  number with a second job, deliberately not derived from the timeout —
  raising one to quiet the other is exactly how the growth gets hidden. Five
  minutes is the claim: a package that takes longer than that to test as one
  binary wants splitting, whatever the ceiling is. internal/rhq is over the
  line today and warns on every darwin run until it is not. That is the number
  working;
- when the clock does expire, a block naming the package and the budget that
  expired, saying plainly that it **cannot** tell a slow package from a real
  hang, and giving the one command that can.

It never fails on a wall clock — `go test`'s exit status is returned
unchanged. A gate that reds on elapsed time is the class tvmh and fsil were.

`make verify-test-times` (0.4s) pins the reporting and runs first in
`make test`. Sixteen mutants, each killed by the arm that should catch it:
hardcoding the budget instead of reading it from argv, both `-timeout`
spellings, treating a missing flag as explicit, deleting or always printing
the timeout block, dropping the package or the budget from it, silencing the
slow warning, hardcoding the exit status, rewriting the command's argv, and
two parser mutants. Two of those arms were themselves wrong first:

- `ok  \tpkg\t0.4s [no tests to run]` puts the suffix **inside field 3**, so
  comparing the whole field parsed nothing — and "nothing parsed" and
  "nothing slow" look identical, which is why every negative arm carries a
  witness line counting the packages it read;
- the arm asserting the block quotes the expired budget was green over a
  blanked-out budget, because the fixture's own `panic: … after 25m0s` is
  teed through to stdout and satisfied the match. Assert the block's own
  line, not a string the passthrough also contains.

The 300s line is platform-honest rather than a blanket noise generator.
Measured at HEAD on the same day: `make test` on darwin prints
`1 over the 300s line` (internal/rhq 436.5s, 29% of budget), and
`make test-linux` on native linux/arm64 prints `0 over` (internal/rhq 86.2s,
5%). Same threshold, same run, and it fires only where the exposure is. That
linux run is also the check that matters for the parsing: the container's awk
is mawk 1.3.4, and a gawk/mawk split in a shelled-out script is a defect this
repo has already paid for once (ranger-base-hhcu). The self-test's sixteen
arms pass under both.

### AND THE MARGIN ON THE SLOWEST PATH, WHICH NOBODY HAD MEASURED

2ggb chose 25m over 20m because `make test-linux PLATFORM=linux/amd64` —
emulated on an arm mac — was "over 600s every time". That is a floor, not a
number: every reading of that path had ended in a timeout panic, so nobody
knew where it actually lands. Measured here with room to finish:

    PLATFORM=linux/amd64 scripts/test-linux.sh go test ./internal/rhq -count=1 -timeout 40m
    internal/rhq   1122.7s     = 75% of the 25m budget

So **25m holds, with 1.34x of margin on the slowest supported path** — much
less than the 2.4x it clears the worst darwin reading by, and the linux/amd64
column is the one that decides. That is a number worth having on the record
and exactly the kind the budget column now prints on every run.

Two caveats on that reading, stated because they are visible in the log. It
was taken at a635cbf, before this branch fast-forwarded onto main. Four tests
in it failed, and none of them are emulation or a real defect: the checkout
was fast-forwarded *while the container was running*, and the repo is mounted
live, so `TestEmbeddedSeedMatchesExamplesDir` and
`TestQADocChainMatchesTheRenderedDispatcher` compared a binary built from the
old tree against files from the new one. A read-only mount stops the container
writing your tree; it does not stop your tree moving under the container.

### THERE IS NO CHEAP THIRD OPTION EITHER

Measured here independently of 2ggb and agreeing with it. `go test -json
./internal/rhq -count=1`, 475.7s, darwin/arm64, 8 cpu:

    1448 top-level tests; slowest single test 12.44s
    tests >= 10s      3     32.8s    7% of the package
    tests >=  5s      7     57.7s   12%
    tests >=  1s    108    241.3s   51%
    sum of every top-level test's Elapsed        474.9s
    the package's own Elapsed                    475.7s

Those last two lines are the finding: the per-test times **sum to the package
time**, so nothing inside internal/rhq runs concurrently with anything else.
`grep -c 't.Parallel()' internal/rhq/*_test.go` is **0** across 193 test files
and 65,893 lines.

2ggb named the blocker as "55 test files calling `t.Setenv`/`t.Chdir`", and
`t.Parallel` does panic in a test that has called either. Measured again here
it is 56 files and 183 `t.Setenv` calls (no `t.Chdir` in the package at all),
and the count is the wrong shape for the problem: **the blocker is one
helper.** `newTestBackend` (internal/rhq/herdr_test.go:1167) calls `t.Setenv`
three times — `HOME`, `RHQ_FAKE_HERDR`, `RHQ_FAKE_DIR` — and it has **575 call
sites**. The temp `$HOME` is load-bearing and must not be dropped: without it
a test reaching `EnsureSessionTree` cut a real git worktree in the operator's
live `~/.posse` (ranger-base-gvrh). So parallelism is not blocked by 56
scattered decisions to unpick; it is blocked by one helper's use of the
PROCESS environment where it wants per-test state, and unpicking that one
helper is most of the lever.

That is a code change with a real flake budget attached, not a gate change.
Handed to dinesh as ranger-base-i7fa with these numbers rather than guessed at
here.
