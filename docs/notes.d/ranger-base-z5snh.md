## A window the provider stops reporting drops out of the reading; only the last one voids it (ranger-base-z5snh)

The shipped plan adapter asks its provider's usage endpoint for two named
windows. Until this bead `Read()` called `pct()` for each and returned the
first error, so a body carrying a perfectly good reading of one window was
discarded whole when the other went silent — and `ranger-base-dkda2`
MEASURED that happening live, for 82 minutes at a two-minute read cadence,
on 200 responses with a live credential. The guard was blind over a window
it could still read, with a threshold armed against it, and no configuration
reached the behaviour: `pct()` ran for both windows whatever thresholds were
set, and the one knob that names blindness bounds how long blindness is
TOLERATED — so winding it all the way down disarms the blind brake rather
than restoring a sighted guard.

The measurement, the falsified hypothesis and the shape capture are
`ranger-base-dkda2`'s — fragment `docs/notes.d/ranger-base-dkda2.md`, with
the instance-ops half on that bead per ADR 0024 D3. This fragment is the
DECISION and what it cost.

### The rule, as ruled

OPERATOR RULING (2026-11-01, on `ranger-base-z5snh`): proposal A, in the
adapter only.

- A window key that is **absent or null** drops out of the returned
  `PlanUsage` instead of failing the read, **provided at least one window
  remains**.
- A body that yields **zero** windows stays an error exactly as before, so
  ranger-base-65s's class keeps the ADR 0018 blind machine.
- **Every value check is unchanged and still fails the WHOLE read.**
- The `Dispatcher.unmatchedThresholds` stderr line stays as the announcement.
- Option B — reading the windows out of the body's own self-describing
  window enumeration — was **NOT** chosen. It remains a live design, and it
  still needs the measurement `ranger-base-dkda2` could not buy: the
  enumeration's own vocabulary strings, captured in a venue where nothing
  else on the box asks the endpoint for an hour.

**Only silence is absence**, and that asymmetry is the whole rule:

- **Silence is about one window, and the provider is entitled to it.** A plan
  that no longer has a short window reports no short window. That is a fact,
  not a fault. Three spellings reach it and they are one answer — the key
  missing, the key present as JSON **null**, and the window object present
  with no `utilization`. They cannot be told apart from the inside and must
  not be: the provider's ways of saying nothing are its own, and a reading of
  0% is the one thing none of them means.
- **An implausible value is about the ENDPOINT.** A body sending a number
  posse cannot read as a percent (ranger-base-cb0s's range, ranger-base-959m6's
  whole-percent grammar) has given no reason to trust the numbers beside it,
  so it arms the blind clock on the lot.

The mechanical shape: `pct` answers three ways instead of two —
`(v, true, nil)` a reading, `(0, false, nil)` absence, `(0, false, err)`
drift — and `Read` assembles only the windows that answered, refusing when
none did. Both windows are read before either is judged, so there is no
order in which a dropped window can hide a bad value.

### Why no config could reach it, and why the harness needed no change

Worth keeping because it is the shape of the whole class: the guard's
configuration is *thresholds*, and the failure was in *parsing*. A threshold
cannot express "stop asking about this window" because the adapter asks for
both windows unconditionally, and the one knob that names blindness
(`plan_guard_blind_max`) sets how long blindness is tolerated, not whether a
sighted reading is possible. **A code change was the only route**, and that
is the diagnostic signature: when the operator's whole vocabulary is
thresholds and the problem is upstream of thresholds, no amount of config
reading will find the lever.

The harness half needed nothing. `PlanUsage` has been a slice of named
windows since ADR 0012 D4 and everything downstream — thresholds, the blind
clock, Dial E's tightest-window arithmetic, `Line()`, the cockpit header — is
label-agnostic and counts no windows. `Dispatcher.unmatchedThresholds`
already existed for a `plan_guard_<window>:` aimed at a window the provider
does not report, and now says so for the window that went silent, once per
process, naming the windows that remain.

### THE EXPOSURE THIS BUYS, named

A provider that silently stops reporting the **tighter** window degrades the
guard to the looser one, announced by one stderr line per process and
nothing else. That is ADR 0010's "dollars and window percentage are
different quantities" residue one notch over — a brake measuring a different
*window* rather than a different *quantity*. It is the operator's accepted
trade and the adapter's comment says so rather than re-deciding it. If a
louder surface is ever wanted, the honest place is a per-pass line; never a
refusal, which would hand back the blindness this bead removed.

### What this is the fourth of

The adapter has now been taught a fact of its provider's grammar by an
incident four times: **shape** (ranger-base-65s — a 200 of the wrong shape is
not a reading), **range** (ranger-base-cb0s — a present key is not thereby a
percent), **scale** (ranger-base-959m6 — this endpoint reports whole
percents), and **absence** (this bead — a window reported as nothing is not a
window at 0%). Every one of the four was fail-open in the direction the
interlock exists to hold, and every diagnosis began with having no capture of
what the endpoint actually sent. A credential-reading shape dumper is its own
design question and is still unfiled.

### Pins

`internal/posse/planusage_test.go`:

- `TestPlanReaderShapeDriftIsNotAReading` — now the zero-window rows only,
  and they are the whole of what holds the blind machine, so `{}`, the error
  envelope and a renamed `utilization` each get an explicit arm rather than
  riding on the first `pct` error. Its last subtest is the line neither test
  may cross: a window present at 0.0 is a fresh window and stays a reading.
- `TestPlanReaderUnreportedWindowDropsOut` — the rows that MOVED, in both
  directions and in all three spellings of silence. The dropped window must
  be **absent** from the result, never present at zero; `win`'s -1 for a
  window a reading does not carry is what makes that assertable.
- `TestPlanReaderDroppedWindowDoesNotRescueABadValue` — the cross-product the
  ruling's "every value check is unchanged" turns on. The silent window is
  the one that would have errored FIRST under the old code, so without this
  a reading could come back carrying one window and a value nobody checked.
- `TestPlanGuardSightedOnTheSurvivingWindow` — the point of the ruling
  through the dispatcher and the shipped adapter: with the tighter window
  silent the guard TRIPS on the one that still reads, is not blind, and names
  the now-unmatched threshold once.

### SEPARATELY, and not this commit

The operator rules that the threshold naming the window this provider has
stopped reporting comes **out of the instance's `config.yaml`** — an
operator-only file, so that edit is the operator's own and no persona's.
Until it is made, the stderr line above is the surface: the threshold gates
nothing and says so once per process. Nothing in this change depends on the
edit, and nothing breaks if the window ever comes back — it would simply be
reported again and the threshold would start matching.

Still open and deliberately not answered here: whether that window is gone
from the plan for good. The body's own window enumeration INFERRED yes at the
time of capture, but nothing on this box can tell a plan change from a
provider-side rollout.
