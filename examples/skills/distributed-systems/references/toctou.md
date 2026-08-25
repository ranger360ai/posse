# TOCTOU / check-then-act

## The concept

A time-of-check to time-of-use (TOCTOU) race: "The product checks the
state of a resource before using that resource, but the resource's state
can change between the check and the use in a way that invalidates the
results of the check" (CWE-367). Named for file-access security bugs
(`access()` then `open()`) by Bishop & Dilger in 1996, but the shape is
universal: **any `if <check> then <act>` where the checked state is
mutable by another actor is a race**, and the gap between check and act
is a window every concurrent actor is invited into.

The generalization that matters for design work: *a snapshot decays*. A
listing, a poll, an exit code, a cached read — each is evidence about the
instant it was taken, not about the instant you act on it.

## Standard answers, strongest first

1. **Make check-and-act one atomic operation.** One syscall
   (`open(O_CREAT|O_EXCL)`), one statement (`UPDATE … WHERE
   status='ready'`), one CAS, one atomic claim (`bd update --claim`).
   The check happens *inside* the act, under the store's own concurrency
   control.
2. **Check at use time, not before.** Operate on the handle you already
   hold (`fstat` the open fd, not `stat` the path); ask the resource
   itself at the moment of action.
3. **Hold a lock across check + act** — turns the window into a critical
   section; see `fencing-and-leases.md` for when the lock itself can lie.
4. **Re-validate after taking ownership, before irreversible action** —
   the safe-reclamation shape (`safe-reclamation.md`).
5. **Make staleness harmless** — idempotent acts tolerate a decayed
   check (`delivery-and-idempotency.md`).

## Standard rebuttals (for an ADR's alternatives-rejected)

- *"I'll re-check right before acting."* Narrows the window; never
  closes it. A narrowed race fires less often, which makes it worse to
  debug, not safer.
- *"The window is milliseconds."* The window's width is set by
  scheduling, GC, and load — unbounded in practice (see the process-pause
  literature in `fencing-and-leases.md`).
- *"This is single-user, nothing else writes."* Count the writers
  honestly: two passes of the same tool, a human and a timer, a crashed
  run's survivor — all are "another actor".

## How posse applies it

ADR 0011 names TOCTOU as the disease behind a whole class of dispatch
bugs: "one store's momentary reading taken as evidence about another
store's durable fact." Its three disciplines are answers 1–4 above, one
each — an atomic claim (`bd update --claim`, whose outcome is read back
from the bead and never from an exit code), a kernel lock held across
check and act, and a prune that re-validates before it destroys. The
worked checking of each against the literature is ADR 0011 Appendix A.

## Sources (verified 2026-08-20)

- **[docs]** CWE-367, "Time-of-check Time-of-use (TOCTOU) Race
  Condition" — https://cwe.mitre.org/data/definitions/367.html
- **[paper]** Bishop & Dilger, "Checking for Race Conditions in File
  Accesses", Computing Systems 9(2), 1996 —
  https://nob.cs.ucdavis.edu/bishop/papers/1996-compsys/racecond.pdf
  (also https://www.usenix.org/legacy/publications/compsystems/1996/spr_bishop.pdf)
