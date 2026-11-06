# The box slept 2h51m inside one test, and go test could not see it (ranger-base-gt0q9)

MEASURED 2026-11-04 on this box (darwin 25.6.0). A `make test` arm-1 run
reported `cmd/posse` red with one failure,
`TestCostPlanHintSurvivesAnUnreadableGuardReadingWithoutRescuingIt`, on both
of its assertions at once:

```
costplan_test.go:544: stdout: got "codex 7d 62%, as of 2h54m ago\n", want one of:
      "codex 7d 62%, as of 3m ago\n"
costplan_test.go:546: stderr must still say why the guard's reading failed: "posse: usage endpoint unreachable\n"
```

The test was green standalone in the same tree in 1.591s, and the diff under
test could not reach `cmd/posse` at all. Neither assertion is about a clock
on its face — the first wants the age of a reading seeded 3m30s before the
launch, the second wants the live 30-minute cooldown seeded with it to still
be holding the endpoint off.

## One cause, and it is the wall clock

`pmset -g log`:

```
2026-11-04 01:50:26 -0500 Sleep        Entering Sleep state due to 'Low Power Sleep':TCPKeepAlive=inactive Using Batt (Charge:1%) 10262 secs
2026-11-04 04:41:28 -0500 WakeDetails  DriverReason:pwrbtn
```

**2h51m02s of sleep, at 1% battery, woken by the power button** — a low-power
sleep, which no `caffeinate` assertion prevents, and this box holds one
continuously (pid 1513, 298 hours at the time of reading). The suite was
mid-run, and that test's single `runPosse` launch straddled the boundary.

Go's monotonic clock on darwin is `mach_absolute_time`, which **does not tick
while the box is asleep**; the wall clock does. So the two sides of this test
were measuring different things:

- the test's own bracket is `time.Since(at)` — monotonic — and saw 3m57s, so
  `agedAges` offered exactly one answer, `3m`;
- the binary rendered `now.Sub(h.At)` from a wall-clock timestamp the test had
  written to disk, and saw **2h51m02s + 3m30s = 2h54m32s**, which `BlindFor`
  prints as `2h54m`. To the minute.

The same 2h51m expired the `retry_at` the fixture had put 30 minutes in the
future, so `plancache` stopped honouring a cooldown that was no longer live,
asked the endpoint, got the 127.0.0.1:1 refusal the sandbox points it at, and
said `usage endpoint unreachable` instead of `rate-limited`. **The binary was
right about the wall clock in both halves.** The fixture was 2h51m old by the
time it was read.

`go test -timeout 25m` does not save a run from this either: that timer is
monotonic too, so the process resumes rather than being killed.

## Why widening the bracket was the wrong fix

The bead said so before the cause was known, and the reasoning holds: the two
assertions fail together, and `agedAges` only reaches the first. Widening it
would have turned a red with one explanation into a red with none.

What the fix does instead, all in `cmd/posse/costplan_test.go`:

- `wallSince(at)` is `time.Since` in the clock the binary reads
  (`Round(0)` strips the monotonic reading, so the subtraction is wall against
  wall), and `agedAges` brackets with it. On a box nothing happens to, the two
  clocks agree to about **4µs per second** (MEASURED 2026-11-04: 62µs over
  15s, the wall clock slewing ~4ppm ahead, so ~2.5ms across a 600s test), so
  this changes nothing except on the run that is interrupted — and it is what
  keeps a clock STEP of a few seconds from reddening an age that is right.
- `skipIfTheBoxSleptMidTest(t, at)` runs after each launch and before the
  first assertion, exit status included, and skips when the wall clock gained
  more than a minute beyond the monotonic time the test spent. A fixture whose
  own deadlines expired while it waited is not a statement about the code.
  One minute is bounded from both ends: ordinary divergence is microseconds,
  and the tightest premise in the file has 90 seconds of margin
  (`PlanUsageTTLDefault` 5m less `agedSeed` 3m30s). No margin would be safe
  on its own — nothing bounds how long a laptop sleeps.
- It refuses a reference time with no monotonic reading
  (`.UTC()` and `.Round(0)` strip it), because `wallJump` of such a time is
  identically zero: a guard that looks installed and asserts nothing.

Both arms were run. With the tolerance temporarily negative the guarded tests
SKIP and print the measured jump; with one call site temporarily handed
`at.UTC()` the test fails with the refusal above.

## The class, and where else it lives

The shape is: **a fixture whose premise is an age or a deadline in wall time,
seeded by one process and read by another, with the bracket computed in
monotonic time.** Any wall-clock jump reds whichever test straddles it, with a
message that accuses the code. It is not about load — the loaded box in the
report is a coincidence of a long run being likelier to straddle a sleep.

Two neighbours assert an EXACT age with no bracket at all, so a jump — or just
a launch slower than a minute — reds them the same way, and they are filed
separately:

- `cmd/posse/planquiet_qa_test.go`, `TestQAStatusSaysTheMeterIsQuiet`:
  `"3h00m ago"` over a snapshot seeded `-3h`.
- `cmd/posse/planmeterspend_qa_test.go`: `"plan meter BLIND 3h00m"`, same seed.

Both were bracketed with the same machinery under ranger-base-dsuk7, which
also rules out the "stop the clock" alternative this bead's description
offered: docs/notes.d/ranger-base-dsuk7.md.
