# ranger-base-np54s — a test helper 638 sites read after the writer stopped, and one that reads it live

`internal/posse/suspend_qa_test.go`'s "no budget, still a witness" polls for
the SUSPENDED line while `d.watchdogLoop` is still running in a goroutine of
its own. It polled through `dispatcherOut`, which is
`d.baseOut().(*strings.Builder).String()` and takes no lock, against a writer
that holds `outMu` for every write it makes (`d.quietf`, dispatch.go). So the
two did not agree on a lock, and a read landing inside the Builder's append
observes a torn slice header:

    panic: runtime error: unsafe.String: ptr is nil and len is not zero

That is worse than a red assertion. A panic ends the test BINARY, so the whole
of arm 3 reports nothing and `make test` exits non-zero with no test named —
and the test passes in 0.034s run alone and unloaded, so a seat that re-runs
it to triage gets a green and learns nothing. Found by ranger-base-h5iae's own
`make test`, whose change could not reach this package.

2026-11-14.

## What landed

| file | what |
| --- | --- |
| `internal/posse/herdr_test.go` | `dispatcherOutLive`, the locked sibling of `dispatcherOut`, beside it and untagged so every arm compiles it |
| `internal/posse/suspend_qa_test.go` | the poll and its deadline `t.Fatalf` read through it; a comment at the site saying which lock and why |

No production file changed, and deliberately: the defect is in the apparatus
reading the output, not in anything that writes it. `outMu` already serialized
every writer — its doc says so and `watchhang_qa_test.go` pins it — and what
was missing was a READER holding it.

## Why two helpers and not one widened one

`dispatcherOut` has 638 call sites across ~130 files in this package
(2026-11-14), and a census of `for` loops polling it finds exactly one — this
site. Every other read is after the writer has STOPPED: the pass returned, or
the loop was cancelled and its goroutine joined, and the join already orders
the read. Putting `outMu` inside the common helper would buy those sites a
lock acquisition to describe a hazard they do not have, and would cost the
next reader the ability to tell the two cases apart — the live read is worth
being able to see at its call site. The test's own sibling subtest, "Watch
seeds the witness", already reads `d.clockWall` under `d.mu` for the same
reason, and the package has `syncBuf` (launchlock_helpers_test.go) for the
case where the fixture owns the writer.

`syncBuf` was the other candidate — swap the rig's `Out` for one and read
`buf.String()`. It was not taken because `dispatcherOut`'s type assertion
would then PANIC for that dispatcher rather than fail, so any later assertion
written with the ordinary helper would come back as a crash in an unrelated
line. The locked sibling leaves all 638 sites working.

`dispatcherOutLive` guards the Builder and not the `Out` field. Watch's swap of
`Out`/`rawOut` (`teeWatchLog`, watch.go) takes no lock and needs none: it runs
in Watch's preamble, before any writer goroutine exists.

## Verified

MEASURED 2026-11-14, this box (Linux 7.2.15-1-omarchy, go1.27.3, 16 cores,
warm build cache), all at a one-minute load average of 2.9-5.1 with at most
one suite slot held by a sibling seat:

| run | reading |
| --- | --- |
| `go test -race -tags posse_arm3 ./internal/posse -count=20 -run '^TestQASuspendWitnessRunsWithNoBudgetAndIsSeededByWatch$'` | ok, 82.355s — the bead's DONE WHEN |
| the same at `-count=5` with the poll reverted to `dispatcherOut` | WARNING: DATA RACE through `herdr_test.go:4198`, FAIL 20.391s |
| `make tree-check` | green, 125.8s |
| `make test` | green, three arms: arm 1 205.5s, arm 2 87.5s, arm 3 136.7s (plus treepins 173.2s, the doors, and the silent-revert audit over 2731 commits) |

Arm 3 is the reading that matters — it is the arm the panic was ending, and it
reports a time rather than nothing.

The negative control is the half worth keeping: it is what says the LOCK is
what fixed this and not a timing shift from the edit. It reproduces the bead's
own 20.3s reading to within 0.1s, and the race's read frame is the unlocked
helper itself.

Both MUTATION lines in the test's head comment still red their arm, checked
one at a time with the other reverted: restoring `watchdogLoop`'s
`if every <= 0 || (budget <= 0 && passBudget <= 0) { return }` reds "no
budget, still a witness" on its own message at the 5s deadline, and dropping
`d.noteClock()` from watch.go reds "Watch seeds the witness" in 0.034s. The
witness is still ASSERTED — the fix changes which function the poll calls and
nothing about what it waits for, so nothing was polled away into a sleep.
