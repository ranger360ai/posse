## Pricing a -race arm (ranger-base-d0xvw)

ranger-base-y3x6n measured that `make test` and `scripts/test-times.sh` carry
no `-race` anywhere, and priced a 4-test slice at ~90x (1.7s -> 115-155s).
This bead's question was which of (a) nothing, (b) a narrow on-demand arm
over the concurrency-carrying tests, (c) a periodic full arm off the critical
path — priced by actually running (b), not guessed.

### The candidate set

"Concurrency-carrying" read as: the dispatch gather fan-in (`dispatch.go`'s
`go func` legs feeding `pendingBead.result`), passcarry (`passcarry.go`'s
`go func` fan-out plus its wake channel), and the watch loop (`watch.go`'s
pulse/backup/guard/watchdog goroutines). Concretely, every `Test*` in:
`dispatch_qa_test.go`, `dispatchparity_qa_test.go`, `passcarry_qa_test.go`,
`watch_test.go`, `watchhang_qa_test.go`, `watchlock_test.go`,
`watchlog_test.go`, `watchpid_test.go` — 106 top-level tests, run via one
`-run` regex so the arm is exactly this slice and nothing else in the
611s package.

MEASURED 2026-09-07, darwin/arm64, go1.26.5, same box as y3x6n, no other
load, `-count=1` (default):

| arm | wall |
|---|---|
| candidate set, no `-race` | ~11s (`go test` reports 6.2s, `time` 11.0s) |
| candidate set, `-race` (run 1) | FAIL, package clock 282.1s, `time` 299.1s |
| candidate set, `-race` (run 2) | FAIL, package clock 251.8s, `time` 252.9s |

So ~23-27x on this slice — same order as y3x6n's 90x on its smaller,
more-concurrent 4-test slice; the multiplier depends on which tests are in
the set, not a fixed tax.

### `-race` found no data race — it found a second wall-clock lie

Both runs FAILED. `grep -c 'DATA RACE'` over both full logs: **0**. Same
signature as y3x6n: the detector's cost is what did the work, not its
detection.

Run 2 names the failure: `TestDryRunWatchTailDoesNotSayDispatched`
(watch_test.go:109), `-race` build, FAIL at 30.01s —

    watch_test.go:132: watch never returned: ...
    ── pass 1 · 17:28:24
    ...
    ── pass 2 · 17:28:45

Pass 2 fired ~21s after pass 1, though the test hands `Watch` a 40ms base
interval. The test's own 30s wait is commented as deliberately generous —
"A backstop, not a margin: ... No assertion below depends on the loop being
faster than this" (watch_test.go:85-88) — and it still tripped, because this
arm runs 106 `t.Parallel()` tests (several of them multi-second even without
`-race`: the parity tests alone run 3 runtimes x several sub-cases) all
fighting for the box's cores while `-race`'s instrumentation multiplies each
one's CPU cost. `watch_test.go:93` carries the identical shape
(`TestWatchStopsOnContext`, same 30s backstop, same 40ms interval) and did
not trip this time only because of scheduling luck — it is the same kind of
test, not a different one.

This is not the y3x6n bug reappearing (that was `PromptGrace`, fixed,
committed) — it is a **second instance of the same class**: a wall-clock
assumption sized for a quiet box, tripped by `-race`'s slowdown under
contention rather than by anything `-race` actually detects. Filed as
ranger-base-<handoff-id> rather than fixed here (scope: this bead prices the
arm, it does not harden every test the pricing exposes).

### Which

**(b), not (a) or (c).**

Not (a): `-race` is not useless here — y3x6n found a real dispatch-visible
defect (a fixture serving stale state past a bead's own close) precisely
because a slow arm forced a race window a normal run never reaches, and this
bead's own run found a second, unrelated case of the same class in one try.
That is signal a QUIET box's `go test` will not produce on its own.

Not (c): a full-package periodic arm is a much larger bill than this slice
suggests. The candidate set is ~1.8% of internal/posse's ~611s no-race
runtime but the OTHER 98% is not concurrency-free — `sync.Mutex` and `go
func` appear in most of the package's non-test files (app.go, autoreap.go,
backuploop.go, beads.go, cageinner.go, herdr.go, ...), so `-race`'s per-access
instrumentation tax applies package-wide, not just to this slice.
Extrapolating even the low end of this slice's ~23x to the full package
implies well past the 25m `-timeout` the Makefile already treats as a ceiling
worth naming (test-times.sh, ranger-base-7xla), and CI already splits this
package into three ~239s arms to stay under its budget (ci.yml) — a fourth,
full-package `-race` arm does not fit that shape without its own timeout and
its own scheduled-workflow infrastructure, which this repo has none of today
(`.github/workflows/` is push/PR-triggered only). That is a real proposal,
not a slice of this one, and it inherits today's flake risk on top.

So: **a narrow, on-demand arm over the concurrency-carrying tests, run by
hand when a change touches dispatch/passcarry/watch or a race is suspected**
— exactly how y3x6n was actually found (gwart ran two arms by hand). Not
wired into `make test`, `test-arm{1,2,3}`, or CI: at ~250-300s it would blow
every arm's per-job budget for a detector that has yet to name one real data
race in this package, and it is demonstrably not gate-clean yet — the
watch-loop tests need a bigger (or `-race`-aware) backstop before this arm
can be trusted to fail for the right reason. That hardening is the filed
bead's job, not a prerequisite to deciding what to do with the arm itself.
