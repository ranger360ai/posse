# Safe reclamation — prove death before destroying

## The concept

Deleting a shared record is check-then-act (`toctou.md`) with an
**irreversible act**: "unreachable in my snapshot" decays like any
snapshot, and the cost of acting on the stale reading is not a retry
but data loss. The concurrent-memory-reclamation literature is the
sharpest statement of the discipline, and its two canonical shapes
generalize directly to records, sessions, and files:

- **Grace periods (RCU)**: first *unlink* — make the record unreachable
  to new readers — then "wait for all pre-existing read-side critical
  sections to completely finish", and only then free (McKenney, LWN).
  Removal and reclamation are **two phases separated by a wait that
  covers every actor who could have seen the record live**.
- **Proof at reclaim time (hazard pointers)**: before freeing, "the
  thread scans the hazard pointers of other threads … If a retired node
  is not matched by any of the hazard pointers, then it is safe … to be
  reclaimed" (Michael, IEEE TPDS 2004). Not "it looked dead when I
  scanned the list" — a **direct, current check for live holders at
  the moment of destruction**.

The combined law: **deletion requires proof of death at delete time,
plus a grace period covering in-flight actors** — evidence of death at
scan time is never enough. Tombstones/deferred deletion are the same
idea stretched over longer horizons.

## Standard rebuttals

- *"It wasn't in the listing, so it's gone."* A listing is one store's
  snapshot; absence is evidence about the listing, not the resource —
  ask the resource itself, by identity, at delete time.
- *"We'll re-scan before deleting."* A second snapshot is a narrower
  window, not proof (`toctou.md`). The hazard-pointer scan works
  because holders *announce* into state the reclaimer reads directly.
- *"Grace period OR direct check is enough."* The grace period covers
  actors already past your check; the direct check covers ones your
  scan couldn't see. The young-record window needs both.

## How posse applies it

ADR 0011 §2: "**Prune must prove death, not infer it**" — a session meta
is deleted only when its `launched:` stamp is older than a grace window
(RCU's wait, sized to the race window itself) **and** a direct per-id
workspace query at prune time confirms death (the hazard-pointer scan);
"a listing snapshot is never sufficient evidence", and only an explicit
not-found answers death — error and silence keep the file. The identity
half rides in front: a listing from the wrong server is evidence about
nothing (`liveness-and-identity.md`). The create side mirrors the same
predicate rather than inventing a second one, because overwriting a
record destroys it exactly as a delete does. ADR 0011 Appendix A names
the two residual asterisks honestly: the unlink itself is still
check-then-act on a path, and a per-id liveness query proves identity
only if ids are never recycled.

## Sources (verified 2026-08-20)

- **[docs/blog]** McKenney & Walpole, "What is RCU, Fundamentally?",
  LWN 2007 (matches kernel documentation) —
  https://lwn.net/Articles/262464/
- **[paper]** Michael, "Hazard Pointers: Safe Memory Reclamation for
  Lock-Free Objects", IEEE TPDS 15(6), 2004 —
  https://www.cs.otago.ac.nz/cosc440/readings/hazard-pointers.pdf
  (record: https://dl.acm.org/doi/10.1109/TPDS.2004.8)
