## A positive witness bounded by a clock, and what it turned out not to witness (ranger-base-51ttd)

2026-11-14

`TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort` (internal/posse/upgrade_test.go,
arm 1) red during ranger-base-oojeu's `make test`, and not on its subject:

    --- FAIL: TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort (0.13s)
        upgrade_test.go:552: the reader saw the old binary 3 times and the new
        one 0 times over 3 reads — it was not running across the swap, so the
        absence below measures nothing

That is the pin's own positive witness, and the swap it was watching reported
0 absent and 0 partial. A red internal/posse exits `make test` 2 for every
seat on the box, so the cost of an unmeetable witness is a whole arm — paid
by whoever reads it next, over a scheduling fact.

This fragment is two things: the bound fix that landed, and a measurement
taken while verifying it that says the witness never witnessed what its name
claims. The second is handed off as **ranger-base-6sbng**.

### 1. The fix: wait for the witness, sample for the assertion

The witness needed a goroutine reading an 8 MiB file in a loop to be
scheduled on both sides of a `rename(2)`, and it was given two 20 ms sleeps
to manage it in. At 3 reads over that window the reader did all 3 on one side
and the pin `t.Fatal`'d on "I could not measure".

The sleeps are now kept AND followed by a wait:

    time.Sleep(20 * time.Millisecond)
    sawOldBeforeSwap := waitForAtLeast(&sawOld, 1, swapWitnessBudget)
    upgradeSwap(target, staged)
    time.Sleep(20 * time.Millisecond)
    sawNewAfterSwap := waitForAtLeast(&sawNew, 1, swapWitnessBudget)

The two are not alternatives and replacing one with the other was the first
draft's mistake. The **sleeps** are the observation window the absence check
is measured over, so dropping them for a wait that returns the instant
`fresh` is first seen cut the pin from ~24 observations to ~4 — a weaker
assertion bought with a greener witness. The **waits** are what make the
witness reachable when 20 ms was not enough. `swapWitnessBudget` is 10 s,
sized to be unreachable rather than tight: one read a side is the whole ask,
and the focused pin runs in 0.02-0.06 s.

Unmet, the witness now `t.Skipf`s rather than `t.Fatal`s, and the three
checks that can be made from the swap's end state — absent/partial count,
the target holds `fresh`, `.prev` holds `old` — run **first and
unconditionally**, so the skip is reachable only when all of them were green.
What the skip then stands for is exactly "this run observed nothing", which
is neither the red the pin exists to print nor the green it is entitled to.
A real gap outranks it: VERIFIED in a throwaway module that `t.Errorf`
followed by `t.Skipf` reports `--- FAIL`, where `t.Skipf` alone reports
`--- SKIP`.

### 2. What the witness does not witness — MEASURED

Verifying the skip arm meant running the reader against a swap that really
does leave a gap: `os.Remove(target)`, then create and write 8 MiB in 64 KiB
chunks, in place of `upgradeSwap`'s `rename(2)`. MEASURED 2026-11-14, this
Linux box, loadavg 2.8, the shipped reader with its 20 ms samples, 12 trials:

| trial | reads | sawOld | sawNew | absent | partial | gap observed |
|---|---|---|---|---|---|---|
| 1 | 22 | 11 | 11 | 0 | 0 | no |
| 2 | 24 | 12 | 12 | 0 | 0 | no |
| 3 | 25 | 13 | 12 | 0 | 0 | no |
| 4 | 25 | 12 | 12 | 0 | 1 | **yes** |
| 5 | 22 | 10 | 12 | 0 | 0 | no |
| 6 | 24 | 12 | 12 | 0 | 0 | no |
| 7 | 25 | 12 | 13 | 0 | 0 | no |
| 8 | 24 | 12 | 12 | 0 | 0 | no |
| 9 | 26 | 12 | 13 | 0 | 1 | **yes** |
| 10 | 25 | 12 | 13 | 0 | 0 | no |
| 11 | 494 | 12 | 12 | 464 | 6 | **yes** |
| 12 | 25 | 13 | 12 | 0 | 0 | no |

**3 of 12 observed the gap. The witness read met in 12 of 12.**

So "saw old, then saw new" is satisfied just as well by a reader descheduled
across the entire write as by one that read through it: a complete read
before the unlink and a complete read after the write is what both look like.
The witness rules out the reader having been absent from *both* sides — the
vacuous pass it was written for — and nothing more. Nine of those trials are
a clean green over a swap that was genuinely leaving the target absent.

The cause is observation cost, not window length. Each observation is a full
8 MiB `os.ReadFile`, ~1.7 ms warm (24 reads in 40 ms above), and the
install(1) window at 8 MiB out of page cache is mostly narrower than that.
Trial 11 is the one trial where it was not, and there the gap was caught 464
times over. Against the measurement this pin stands for — ADR 0092 D3.5,
~84,000 observations over a 58 ms window, ~0.7 µs an observation — the pin's
probe is ~2,400x coarser than the probe whose result it pins.

ranger-base-6sbng carries that, with the shape of the answer: an observation
of `open` + first byte + `Stat` is ~2-3 µs and still distinguishes everything
this pin switches on (ENOENT is absent, size < 8 MiB is short, first byte
`O`/`N` is old/new), and "absent or short" is the pin's name rather than a
proxy for it. It also carries the question worth deciding with it — whether
this witness should become a *coverage* witness, the reader timestamping each
observation and the pin asserting a maximum inter-observation gap narrower
than the window it claims to rule out, which is the only form of it that
would have caught the nine vacuous trials.

### 3. Verified

| reading | result |
|---|---|
| focused, `-count=12` | 12/12 PASS, 0.731 s total |
| focused, `-count=10`, earlier | 10/10 PASS, 0.02 s each (was 0.06 s) |
| `GOMAXPROCS=1`, `-count=20` | 20/20 PASS, no skips |
| `-race`, `-count=5` | clean |
| skip arm, budget planted at 1 ns | `--- SKIP` 3/3, package `ok`, message reads "the old binary 3 times and the new one 0 times over 3 reads" — the reported failure's own shape |
| `make fmt-check`, `go vet` arms 1/2/3 | clean |

The skip arm was planted by temporarily setting `swapWitnessBudget` to
`1 * time.Nanosecond` and restored in the same session; no synthetic box load
was generated for any of this (standing ruling: no load testing here).
