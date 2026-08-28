## The dead-loop honesty check, and the rendering that failed it (rangerhq-mgvx)

The governance surface's load-bearing claim is that it does not depend on
the thing it monitors: "the view does not depend on the loop — `posse
status` reads the stores directly and reports G7 itself, via the flock
probe (release *is* death, no staleness class). What dies with the loop is
*delivery* only" (the archive's governance-surface ADR §2; its numbers do
not resolve here — HISTORY.md "ADR numbering"). rangerhq-81y0 built the
surface; this bead killed the loop and looked.

`posse status` passed on the first try, uncorrected: with the loop killed
`-9` it printed `URGENT G7 … nothing is being delivered` and exited 1. The
**cockpit did not**. Two defects, both of the same shape — a rendering that
answers a governance question with silence:

1. **`displayOnly` never ran the check.** The non-tty cockpit (a piped
   pane, a logged one, every capture you can take from a shell) drew
   sessions and issues and left `c.gov` at its zero value, so it rendered
   no GOVERNANCE block and said nothing at all. Measured before the fix, in
   one rig, one second apart: `posse status` → `URGENT G7`, exit 1; the
   cockpit → a clean shop.
2. **The header never carried it in any mode.** The block is body. It
   scrolls out of the viewport the moment the cursor walks down into READY
   WORK, and no key scrolls back to it. The ADR gives the header a specific
   job — "a dead loop pulses nobody, and the residual witness is the
   operator's glance at the cockpit header" — and a witness that scrolls
   out of the frame is not one.

### What the header says now

    🤠 posse   14:05:09 · 0.3.0+… · 5h 42% · 7d 61%        gov 1 URGENT · loop dead

Three states, kept apart for `planSegment`'s reason (rangerhq-6h1) — an
empty segment is indistinguishable from "this rendering does not do
governance", which is the silence a dead loop must not be able to hide in:

| segment | means |
|---|---|
| `gov …` | no check has landed yet. **Unknown, not clear.** |
| `gov clear` | the check ran and found nothing. |
| `gov 1 URGENT · 2 LANE` | the summary; `· partial` when a store could not be read. |
| `… · loop dead` | G7 is in the set. |

G7 is **named rather than counted**. Every other row in that count is a
condition somebody still has to be told about, and the loop is what tells
them: `1 URGENT` with delivery dead understates by exactly the row that
matters most.

The segment is a **fixed column at the right edge, not text appended to the
flex column**. The flex column truncates from its tail, so a governance
segment appended there is the first thing an 80-column pane throws away —
and the row it throws away is the one that says nothing is being delivered.
Fixed, it costs the clock and the version instead. That was found by a test
at 80 columns, not by reading: the live rig's pane was wide and its plan
segment empty, so the rig alone would have shipped it.

### `scripts/verify-govern-honesty.sh`

The rig is versioned (`make verify-govern-honesty`), and it is the pin for
the half no unit test reaches: a real binary, a real kernel lock, a real
process killed. Scratch `--session` herdr plus a scratch `RHQ_HOME` on
**both** the caller and the watch-loop process (the rangerhq-snd wipe was
the reverse), `beads:` present and empty so the bd half has nothing to scan
and G7 is the only row that can move the verdict, `autostart_dry_run: true`
worn anyway. Seventeen arms; the five that make it a probe rather than a
sticker:

- **the control arm** — loop alive: status exits 0, says "nothing needs a
  human", and the header says `gov clear`. Without this one, a surface
  that shouted G7 unconditionally would pass everything else.
- **kill -9**, never a graceful stop: release-is-death is the whole
  argument for the flock over a pidfile (rangerhq-gir5), and a clean
  shutdown path would be testing the wrong thing.
- **a stale `dispatch-watch.pid` naming a LIVE pid** does not suppress G7.
  That is the husk check the flock retired; a binary that still consulted
  it passes every other arm and fails this one.
- **autostart disarmed** → no G7 and exit 0. The row is "dead *while
  armed*", and this is also what makes the dead arm's non-zero exit
  attributable to G7 rather than to the rig.
- **the pulse, and only the pulse, dies with the loop**: `state/pulse.yaml`
  (its tick is the file's only writer) stops changing while `posse status`
  answers off the live stores in the same second.

Mutation-checked, because a pin that survives the absence of its fix pins
nothing: the same script against the binary built from the commit before
this one fails exactly `alive-cockpit-header-clear`,
`dead-cockpit-header-says-loop-dead` and `dead-cockpit-block-has-G7`, and
passes all fourteen others — including every `posse status` arm, which
rangerhq-81y0 had already earned.

### The audit: what else depends on the loop

Every row was walked against its store. Nothing else on the surface goes
silent when the loop dies, and the two that look like exceptions are not:

- **G4 (guard skipping, sustained)** is the one row a hand-typed `posse
  status` never reports, because the streak lives in the watch process's
  memory. That is not a lost fact — a guard streak is a property of a
  running loop, and a dead loop is skipping nothing. The row it would have
  raised is subsumed by G7, which is URGENT.
- **G5 (guard blind)** reads the *shared* plan snapshot's timestamp, so a
  fresh shell answers it, and a loop that dies while the endpoint is down
  makes the blind window grow rather than freeze. Right direction.
- **The watch log** dies with the loop, as history does. It is audit, not
  truth, and the ADR never gave it a store's job.

One shape worth naming for whoever arms this: **G7 is gated on
`autostart_interval:` being present**, and the pulse is gated on
`pulse_interval:`. An instance that arms the pulse *without* arming
autostart has a delivery path that dies with a hand-run loop and no row
that says so — correct as designed (nobody promised a loop, so a standing
G7 would be a false alarm every hour of every day), but it means "the pulse
is armed" is not by itself a promise that anyone will be pulsed.
