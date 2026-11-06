# A wall-clock budget on the box, standing in for a budget on the arm (ranger-base-91rep)

`TestQABdCeilingTwentyCallsFastEnough` (internal/posse/dataceilingargv_qa_test.go)
is ADR 0069's verification 7: the data ceiling's bd-shim arm must not cost
much per call. It asserted 20 ceiling calls under **5s of absolute wall**,
and on 2026-11-04 it read 5.490003708s in monica's push lane at bdd3954b
while the 15-minute load average was ~85. It passed `-count=3` in isolation
at load ~10. Same class as ranger-base-191em.

## The number was never about the arm

The arm is unchanged across every reading below. What moves is the box.

| 20 ceiling calls | one-minute load | source |
|---|---|---|
| 0.30–0.48s | idle | ADR 0069 V7, the prototype |
| 1.29s | three-arm `make test` | the budget's own comment, when 5s was chosen |
| 2.58–2.97s | 30–60 | MEASURED 2026-11-04, this worktree |
| 5.490s | ~85 | the reported flake, bdd3954b |

An 11x spread over identical code, against a bound that had ~10x of headroom
when it was written. The budget had to be crossed eventually; load 85 is where
it happened. ADR 0069's actual claim is "15–24 ms per bd call, linear in
classes and in file size" — a claim about the arm, which an absolute wall
reading cannot separate from the scheduler.

## What replaced it

The same shim rendered **without** the arm (`qaBdCeilingShim(t, false)`) is
the baseline, measured in the same process, interleaved call-for-call with
the ceiling shim, both warmed three calls each first. A plain call and a
ceiling call in the same window pay the same scheduler tax, so the ratio
between them does not move with the box.

MEASURED 2026-11-04/06, this worktree, darwin, loads as noted:

- **Interleaved ratio: 1.78–2.27 over 15 readings at loads 26–60.** Six of
  those, with the symmetric warm-up the committed pin uses, at load 26–30:
  1.972, 2.124, 1.842, 1.814, 1.909, 1.960.
- **Two sequential loops instead of interleaving, same box, same minutes:
  1.80–3.48.** A 1.7x spread against the interleaved 0.5x one — a loop that
  runs second sees whatever the load did meanwhile. This is why the pin
  interleaves rather than timing two loops.
- Warming the ceiling shim 20 calls more than the plain one pulls the ratio
  to **1.04–1.29**. Hence the equal warm-up, and hence alternating which
  shim leads each pair.

Committed bound: `ceiling <= 6*plain + 300ms`. The slack term covers the one
case a ratio cannot speak to — a plain baseline so fast the arm's fixed cost
dominates it — and is larger than the whole measured ceiling leg, so it can
only forgive, never accuse.

## Repro, and what the new bound can and cannot catch

**The flake class, deterministically.** A uniform per-call slowdown added
inside the measured window on *both* shims is what a loaded box does. At
+300ms/call the old absolute assertion reads 10.276s and 9.079s — red twice,
against its 5s budget — while the ratio reads 0.979 and 0.996. At +0ms the
same run is 2.581s/2.971s absolute, under 5s, ratio 1.040/1.292.

**The regression the pin exists for.** Injecting N extra scans per call into
the rendered arm models a lost short-circuit. MEASURED, with the committed
6x+300ms bound:

| extra scans/call | ratio | bound fires |
|---|---|---|
| 8 | 4.646 | no |
| 14 | 6.783 | **yes** |
| 18 | 7.599 | **yes** |
| 25 | 7.750 | **yes** |
| 40 | 13.823, 14.337 | **yes** |

So the bound sits at roughly **3–3.5x the arm's own current cost**: it will
not notice a 2x regression and it will catch a 5x one. That is the order of
magnitude ADR 0069's "linear in classes and file size" claim breaks by, and
it is deliberately not tighter — a bound tight enough to pin 15–24ms is the
bound this bead was filed about.

Both repro levers were throwaway tests, not committed: the measurement is the
deliverable, and a committed copy would be a second slow pin asserting the
first one's arithmetic.
