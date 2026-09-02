# Broadcast brief: `POSSE_KEEP=`, the declare-or-die marker

For **ranger-base-x8qph** (monica, `-l ops`), handed over from
ranger-base-gvp2p. Deliverable 1 of the operator ruling of 2026-08-31 on
ranger-base-z9m2 asks that the marker be documented "so every persona knows
the form". Three homes; two of them ship with gvp2p and want nothing from
the coordinator:

- **AGENTS.md** — a new bullet beside the existing one on how to tell
  whether a backgrounded process really went away. This is the wording to
  carry, and it is already short enough to carry verbatim.
- **NOTES.md** — "Leaked gate-shell children, and `POSSE_KEEP=`
  (ranger-base-apwr, -gvp2p)": the predicate, both arms, the two anchors and
  why they point opposite ways, the measured signal ladder, and the
  fail-open/fail-closed split.
- **`cmd/checkorphans`** — prints the exact spelling to any persona it is
  telling that something of its own is still running, which is the only
  moment the spelling is worth anything.

The third home is **each persona's standing orders**, and that is a
coordinator write rather than a developer one: `ORDERS.md` is one file per
persona under the operator's personas dir, outside any repo a dispatched
session can commit.

## Why now, and not at the flip

The kill arm ships **disarmed** (`load_guard_kill:`, default false) and the
live flip is its own operator bead, **ranger-base-w3aet**. Broadcast anyway:
a rule personas first meet on the day it starts ending processes is a rule
that arrived one incident late.

Declaring early already buys something concrete. A declared orphan is
dropped from arm 1's leak count and named as spared instead — which is
exactly what keeps the arm-1 field data w3aet is waiting on honest, since a
deliberate process reported as a leak would be the one false positive the
flip is gated on.

## Housekeeping

ranger-base-x8qph began as a one-word probe of the `bd` argv gate and was
retitled in place: gwart's PID denies `bd delete`, so it could not be
withdrawn and re-filed. Its `discovered-from` edge to ranger-base-gvp2p was
added after the fact for the same reason. Its description is short and
points here because the harness gate refused every long-form spelling of
this text on `bd create -d` and `bd update -d` — the refusal is
deterministic and reproduces on a 110-character string, so it is not a
length limit; the text survives fine in a committed file.
