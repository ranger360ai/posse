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
| candidate set as RUN — 20 of 106, see CORRECTION, no `-race` | ~11s (`go test` reports 6.2s, `time` 11.0s) |
| candidate set as RUN — 20 of 106, `-race` (run 1) | FAIL, package clock 282.1s, `time` 299.1s |
| candidate set as RUN — 20 of 106, `-race` (run 2) | FAIL, package clock 251.8s, `time` 252.9s |

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

---

## CORRECTION — laurie, 2026-09-10 (verify ranger-base-7npp8)

### The priced arm ran 20 of the 106 tests it names

`internal/posse` became three build-tag binaries on 2026-09-06
(ranger-base-qp1hm, a65d34a0 / 1b840d96) — the day before the measurement
above. The candidate set straddles all three partitions:

| file | build line | tests |
|---|---|---|
| `dispatchparity_qa_test.go` | `!posse_arm2 && !posse_arm3` (arm 1) | 4 |
| `passcarry_qa_test.go` | `!posse_arm2 && !posse_arm3` (arm 1) | 8 |
| `watch_test.go` | none — shared, in all three | 4 |
| `watchpid_test.go` | none — shared, in all three | 4 |
| `dispatch_qa_test.go` | `posse_arm2` | 52 |
| `watchlock_test.go` | `posse_arm2` | 9 |
| `watchlog_test.go` | `posse_arm2` | 6 |
| `watchhang_qa_test.go` | `posse_arm3` | 19 |

One `go test ./internal/posse -run <regex>` with no `-tags` compiles only the
arm-1 and shared files, so 86 of the 106 names matched nothing and were never
built — every one of the 52 dispatch tests included, among them
`TestRunRefillsAFreedSeatInsideOnePass` and
`TestQARefillFiresASecondBeadIntoTheSameSeat`, the two arms whose `-race`
failures are why ranger-base-y3x6n and this bead exist. `go test -run` emits
no diagnostic for a name that matches nothing. MEASURED 2026-09-10, HEAD
736a2a32, by `-list` census against the 106 names: **default build 20,
`-tags posse_arm2` 75, `-tags posse_arm3` 27**.

This is rot #4 in `armtags_qa_test.go`'s own header, verbatim: "a `go test
-run` that matches no test exits 0". The same trap has already eaten y3x6n's
recorded repro — at HEAD, its command prints `ok … 0.509s [no tests to run]`
and exits 0.

### The corrected price: three commands, ~11.4 min, ~13x — and it is WAIT, not CPU

MEASURED 2026-09-10, darwin/arm64, go1.26.5, HEAD 736a2a32, `-count=1`. This
box was NOT quiet — other seats held load average between 13 and 85 across
these runs, so treat the wall figures as an upper bound; the user-CPU column
is the load-insensitive one.

| arm | tests | no-`race` pkg clock | `-race` pkg clock | `-race` user CPU / %CPU |
|---|---|---|---|---|
| default (arm 1 + shared) | 20 | 7.5s | 256.2s FAIL | 58.2s / 31% |
| `-tags posse_arm2` | 75 | 22.2s | 369.1s FAIL | 42.3s / 18% |
| `-tags posse_arm3` | 27 | 21.7s | 45.6s FAIL | 14.3s / 32% |
| **whole candidate set** | **106** | **51.4s** | **670.9s** | — |

With the four known-failing tests skipped every arm is green and the price
does not move: 263.3s + 378.9s + 39.6s = **681.8s, `ok`, `DATA RACE` 0**, at
13-14% CPU. So the arm's cost is not the detector's instrumentation tax — it
is wall-clock waiting: fixed intervals, backstops and barrier timeouts inside
these tests stretch under the `-race` build while the box sits idle. `~23-27x`
above is `~13x` over the whole set, and `~250-300s` is `~11.4 min` in three
commands.

That also undercuts the (c) reasoning: the extrapolation there assumes a
per-access CPU tax spread over the package. Two other claims in it do not
hold as written either — `sync.Mutex`/`go func` appear in **3** of
`internal/posse`'s 117 non-test files (14 if `sync.` of any kind counts), not
"most", and of the six named, `autoreap.go`, `backuploop.go` and `herdr.go`
carry neither token while `app.go` and `beads.go` carry only a `sync.Map`
used to dedupe notices. The (c) verdict may well survive on the other
grounds given (no `schedule:` trigger anywhere in `.github/workflows`, the
25m ceiling, three ~239s CI arms); the arithmetic behind it should be redone
rather than relied on.

### `-race` did name more than time — in the arm that never ran

`-tags posse_arm2` under `-race` fails three tests that are green without it.
Reproduced 2026-09-10 alone, on a quiet box, at 4% CPU — so not contention:

    $ go test -race -tags posse_arm2 ./internal/posse -run '^TestDispatchParallelPass$' -count=1
    dispatch_qa_test.go:1421: prompt 0 was released "timeout" after 10.003964s
        — it was the only one in flight, so the pass awaited serially rather than gathering
    dispatch_qa_test.go:1421: prompts never overlapped — awaited serially, not gathered (13.207167s between them)
    --- FAIL: TestDispatchParallelPass (65.02s)      # 1.08s and PASS without -race

`TestDispatchParallelPassGathersDespiteCreateStagger` fails identically and
`TestStatusAfterTimeoutRidesOutABlink` fails at `dispatch_qa_test.go:715`.
`DATA RACE` count is still 0, and the two readings are y3x6n's two readings —
a fixture that serializes under the `-race` build, or a gather that does. It
is filed, not diagnosed here.

### What this changes

Nothing about the SHAPE of the answer — a narrow on-demand arm is still the
proposal, and this correction is not a reopen. What it changes is the recipe
and the number: the arm is three commands, not one; it costs ~11.4 min, not
~250-300s; and "the detector has yet to name one real data race in this
package" was said over a run that never compiled the package's dispatch
tests.

---

## THE RECIPE IS A TARGET NOW — `make test-race` (ranger-base-nhc23, 2026-09-10)

The decision above stands; its recipe does not. "Run by hand when a change
touches dispatch/passcarry/watch" was written as one `go test -run`, and the
arm is three build-tag binaries — which is how it came to be priced over 19%
of itself. It is `scripts/test-race.sh` now, run by `make test-race`, and it
does the two things a typed recipe cannot: it reads the candidate test names
out of the eight candidate FILES on every run, so no list of names ages in a
document, and it censuses all three arms before it spends a minute, so a name
compiled into NO arm reds in ~8 seconds instead of being skipped in silence
(the run carries the same check a second time, because `[no tests to run]` is
also a zero exit).

Census MEASURED 2026-09-10, HEAD e4341e56: **107 candidate tests — arm 1 21,
`-tags posse_arm2` 76, `-tags posse_arm3` 28**, every one reachable. Pinned by
`internal/treepins/racearm_qa_test.go`, mutation-checked. The price above is
unchanged and the arm is still on-demand only.
Notes: `docs/notes.d/ranger-base-nhc23.md`.
