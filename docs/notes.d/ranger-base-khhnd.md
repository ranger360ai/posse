## The watch-loop backstop is the binary's own deadline, not 30s (ranger-base-khhnd)

`TestWatchStopsOnContext` and `TestDryRunWatchTailDoesNotSayDispatched`
(`internal/posse/watch_test.go`) each drive the real `Watch` loop at a 40ms
base interval, wait for the loop's own output to reach N pass headers, cancel,
and then wait for `Watch` to return. That last wait was
`time.After(30 * time.Second)`, commented — correctly — as "a backstop, not a
margin": nothing either test asserts depends on the loop being fast, and the
wait exists only so a loop that wedges mid-pass fails with a sentence instead
of hanging until `go test`'s global timeout.

ranger-base-d0xvw priced an on-demand `-race` arm over these tests and the
30s tripped: pass 2 of a 40ms loop landed ~21s behind pass 1 and the dry-run
test reported `watch never returned` about a loop that was running correctly,
just slowly. `grep -c 'DATA RACE'` over the whole log was 0 — the detector
supplied TIME, not a race. Same class as ranger-base-y3x6n's `PromptGrace`
(fixed in ebe3676), second instance.

### Why not a bigger constant

A wider constant moves the guess; it does not retire it. The wall here is not
a property of the code under test — it is how much CPU ~750 `t.Parallel`
tests leave for one loop, times whatever the `-race` build costs, on whatever
box is running. That number has already been measured moving 2.4x in four
days on this repo's own suite (ranger-base-pj87l), and this same class of
assumption has now been re-learned three times (y3x6n, ranger-base-nmab1,
this bead).

The backstop's job states its own honest value: it must beat `go test`'s
timeout panic with a better message. So take that timeout and subtract the
room to print. `t.Deadline()` reports it (it is whatever `-timeout` the
runner passed), so `watchBackstop(t)` is `time.Until(deadline) - 30s`, with
`-timeout 0` falling back to 10m and a deadline already within 30s falling
back to the old 30s rather than a zero or negative wait. A genuinely wedged
loop now costs the run what it was always going to cost — the binary's
timeout — and pays it with the test's sentence and its captured pass output
instead of a goroutine dump. `backstopBefore` is the arithmetic split out so
the ends can be pinned over inputs a test binary cannot produce on demand;
`TestWatchBackstopTracksTheBinaryDeadline` pins them.

### MEASURED 2026-09-10, darwin/arm64, go1.26.5, not a quiet box

Same three commands as d0xvw's corrected recipe (`-race`, the 106-name
candidate regex, `-count=1`, no `-tags` / `posse_arm2` / `posse_arm3`). The
"before" column is d0xvw's own table. `DATA RACE` count is 0 in all three
after-runs, as it was before.

| arm | tests built | before | after |
|---|---|---|---|
| default (arm 1 + shared) | 20 | 256.2s FAIL | **252.5s `ok`, exit 0** |
| `-tags posse_arm2` | 75 | 369.1s FAIL | 366.1s FAIL — only ranger-base-0dt50's three dispatch tests |
| `-tags posse_arm3` | 27 | 45.6s FAIL | **52.1s `ok`, exit 0** |

The two tests themselves, per arm — and this is the part that says the 30s
was not unlucky but simply too small on this box:

| test | arm 1 | arm2 | arm3 |
|---|---|---|---|
| `TestDryRunWatchTailDoesNotSayDispatched` | 32.94s | 33.02s | 33.30s |
| `TestWatchStopsOnContext` | 21.86s | 22.13s | 22.34s |

Three runs out of three, the dry-run test needed MORE than the 30s it used to
carry. The effective backstop in these runs was 9m30s (no `-timeout` on the
command line, so `go test`'s 10m default less the reporting room).

What is still red under this arm is not this bead: the three arm-2 dispatch
failures are ranger-base-0dt50 (`the parallel-pass gather does not gather
under -race`), and they are green without `-race` and carry 0 `DATA RACE`
too.

### Not changed

The other `30 * time.Second` waits in this package were left alone. They are
a different shape — a one-shot wait on a single operation rather than a wait
on N passes of a loop whose per-pass cost is what `-race` multiplies — and
none of them has been measured failing. `launchlock_qa_test.go:220` drives
`Watch` too, but at a 30s interval, where the loop is not expected to
complete passes inside the wait at all.
