## A probe three orders of magnitude coarser than the window it was looking for (ranger-base-6sbng)

2026-11-14

`TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort` (internal/posse/upgrade_test.go,
arm 1) observed the target with a full 8 MiB `os.ReadFile`. One observation
cost ~1.7 ms; the install(1)-shaped window it exists to catch is 2.6-4.2 ms at
that size. So the pin passed over a swap that was genuinely leaving the target
absent, most of the time, and nothing in it could say so — ranger-base-51ttd's
verification found it at 3 trials of 12 and handed it here.

The fix is a denser observation, a positive control, and a budget on the
probe's own cost. Everything below is MEASURED on this Linux box (16 cores,
go1.27.3, tmpfs `TMPDIR`) on 2026-11-14, with the one-minute load average
given per table because it is the variable that tracks.

### 1. The probe

One observation is now `open` + `Fstat` + the file's two end bytes, and it
resolves every state the pin switches on:

| observation | verdict |
|---|---|
| `open` fails | absent |
| size != the binary's | short |
| first and last byte disagree | a blend — the target is being rewritten in place |
| otherwise, the byte | which version |

~5-6 µs against ~1.7 ms, and "absent or short" is the pin's name rather than a
proxy for it.

### 2. Power, over 50 trials a reader, loadavg 7.5

A throwaway rig ran each reader against each swap shape, 50 trials, same 20 ms
samples, same 8 MiB, everything else equal. "Caught" is absent or non-version
observations > 0.

| reader | swap | observations / 40 ms | caught the gap |
|---|---|---|---|
| 8 MiB `os.ReadFile` | install(1): unlink, then 64 KiB chunks | 19-26 | **14 of 50** |
| `open` + first byte | install(1) | 7,400-8,500 | **50 of 50** |
| 8 MiB `os.ReadFile` | the real `rename(2)` | 19-26 | 0 of 50 |
| `open` + first byte | the real `rename(2)` | 7,400-8,500 | 0 of 50 |
| 8 MiB `os.ReadFile` | truncate in place, then chunks | 108-214 | 10 of 10 |
| `open` + first byte | truncate in place, then chunks | 7,200-8,500 | 10 of 10 |

The rig's dense reader reads one byte; the probe that shipped reads both ends,
for the reason section 5's M7 gives, and costs 5.5-6.2 µs an observation where
the rig's read 4.9-7.2 µs. Those are the two numbers, and neither is the
other's.

The window itself, measured as the wall time of the non-atomic write: 2.6-4.2 ms
at 8 MiB, 5.8-8.4 ms at 16 MiB, 11.5-19.1 ms at 32 MiB, 23.2-25.1 ms at 64 MiB
(5 trials each, loadavg 15). The coarse reader's count rises inside a gap —
an ENOENT `open` and a short read are both cheap — which is why its catch rate
is not zero and why its count cannot be read as density.

### 3. The witness: why the control, and why the mean rather than the max

The bead asked whether the "saw old, then saw new" witness should become a
coverage witness asserting a maximum inter-observation gap. It should not, and
the reason is a measurement:

| | reading |
|---|---|
| widest hole in a trial, 74 `rename(2)` trials, loadavg 7-17 | 23 µs - 2.67 ms |
| the window a bound would have to be narrower than | 2.6 - 4.2 ms |

The largest honest bound is therefore ~2.5 ms against a window of 2.6 ms: a few
percent of margin, and a constant to re-measure on every box. At the high end
the hole is a GC pause (`debug.ReadGCStats` over the same trials: one cycle per
trial, pauses to 1.59 ms), which no probe design removes.

What the pin does instead is run the **same reader** a second time against a
deliberately install(1)-shaped write and require it to see that gap
(`installShapedSwap`). That is the probe's power measured on this box under
this run's load rather than inferred.

The control is not sufficient on its own. With the probe mutated back to the
8 MiB read, the control still saw its own gap in 2 runs of 3 — and with the
control's three attempts, in 2 of 3 again — because inside a gap a coarse probe
catches up. So the pin also budgets the **mean** observation at a tenth of the
window it must resolve (`swapProbeMaxMeanGap`, 260 µs). There the separation is
decisive:

| probe | mean observation | against a 260 µs budget |
|---|---|---|
| open + two ends, every run taken, loadavg 2-17 | 5.5-6.2 µs | 42x under |
| 8 MiB `os.ReadFile` | 1.7-2.1 ms | 7x over |

The subject's mean is the load-bearing half: a control spends part of its window
inside the gap it made, so it reads cheaper than it is (158 µs measured with the
probe coarsened, in a run whose subject read 1.95 ms). Both are checked, because
an OR of the two can only be stricter.

All three witnesses **skip** rather than red, after the three end-state checks
have run unconditionally — ranger-base-51ttd's ruling, unchanged.

### 4. The control's attempts

At `GOMAXPROCS=1` the control was missed in 2 runs of 10, each time with a
widest hole of 3.7 ms, which is the whole window: the reader was descheduled
across it. On this box's sixteen Ps, 0 in 60. That is a scheduling accident and
not a fact about the probe, so the control runs up to three times and stops at
the first attempt that sees its gap. `GOMAXPROCS=1 -count=30`, twice: 60 PASS,
0 SKIP. Each extra attempt costs ~55 ms and only on the runs that need it.

### 5. Mutations

Each mutant was applied to a committed tree and restored with
`git checkout --`; `-count=3` each; loadavg 2.1.

| mutant | verdict | designed |
|---|---|---|
| M1 `upgradeSwap` unlinks then writes in chunks — install(1) is back | FAIL x3 | red |
| M2 `upgradeSwap` truncates in place and writes in chunks | FAIL x3 | red |
| M3 `upgradeSwap` rewrites in place without truncating — never absent, never short, blended | FAIL x3 | red |
| M4 the probe is coarsened back to the 8 MiB read | SKIP x3, on the density witness | skip |
| M5 the control is made atomic (write + rename) | SKIP x3, in 3 attempts | skip |
| M6 the size arm never fires AND M2's swap | FAIL x3 | — |
| M7 the blend arm never fires AND M3's swap | PASS x3 — SKIP x3 since ranger-base-0eysw, see the correction below | — |
| M8 the density witness never fires AND the probe is coarsened | PASS SKIP PASS | — |

M6 and M7 are the two arms' own readings and neither is a clean kill, which is
the honest result. **M6**: with the size arm dead, M2's truncate shape still
reds, because a zero-length file fails the read of its last byte — 121-134
detections against 483-599 with the arm live, over three battery runs, so the
size arm is roughly three quarters of them and not the only one. **M7**: with the blend arm dead, M3's shape is invisible and the
pin is green, so reading the *last* byte as well as the first is exactly what
keeps the shape the full-content compare used to see. **M8** is what the density
witness buys: without it, a coarsened probe reaches a green in 2 runs of 3 on
the control's luck alone, where M4 shows the witness catching the same mutant
3 of 3.

CORRECTION 2026-11-14 (laurie, ranger-base-0eysw). M7's reading stands and the
conclusion drawn from it was half the available one. Both mutants above combine
a dead arm with the defect that arm reads, which says the arm is NECESSARY; run
UN-COMBINED — the arm dead, `upgradeSwap` untouched — the two part company.
MEASURED on this box, three runs each, at the tree 5f865bdb landed:

| un-combined mutant | verdict | what it says |
|---|---|---|
| the ABSENT arm never fires, real swap | PASS x3 | and M1's shape still reds through the size arm — the detection survives, only its attribution moves |
| the SIZE arm never fires, real swap | PASS, SKIP, PASS | the control notices: with the size arm dead it has only the ENOENT window left, and a run that misses it skips |
| the BLEND arm never fires, real swap | PASS x3 | nothing notices, in any configuration |

So the blend arm was the one arm of the three whose death was invisible to this
pin: `installShapedSwap` unlinks the target and writes a new one, so every
observation it leaves is ABSENT or SHORT and the two-ends comparison is never
the arm that fires. The absent and short arms each had a control in the pin;
the arm this bead deliberately went past its own brief to add had a mutant
nobody runs. Closed under ranger-base-0eysw by a SECOND control
(`blendShapedSwap`, an in-place rewrite that never unlinks and never
truncates) and a `blend` sub-count the control requires by name, so one
shape's detections cannot vouch for another shape's arm — M7 and the
un-combined blend mutant both read SKIP x3 now, and M3 still reds FAIL x3.
docs/notes.d/ranger-base-0eysw.md has the readings.

### 6. Verified

| reading | result |
|---|---|
| focused, `-count=12` then `-count=6`, `-count=4` | all PASS, 0.10 s each (was 0.02-0.06 s; it runs its probe twice now) |
| `GOMAXPROCS=1`, `-count=30` twice | 60 PASS, 0 SKIP |
| `-race`, `-count=5` | clean, 0.18 s each |
| mutation battery, 8 mutants | all 8 behaved as designed (above) |

No synthetic box load was generated for any of this — standing ruling, no load
testing here. The rig that produced the tables in sections 2-4 is a throwaway
program that replaces `upgradeSwap` with a non-atomic write and is not
committed, which is the same reason ranger-base-51ttd's was not.
