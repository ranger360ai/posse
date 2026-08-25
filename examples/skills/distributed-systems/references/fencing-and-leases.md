# Leases and fencing tokens

## The concept

A **lease** is "a contract that gives its holder specified rights over
property for a limited period of time" (Gray & Cheriton, SOSP 1989) —
a lock with an expiry, renewed by a live holder, reclaimed from a dead
one. It is the field's standard answer to "the lock holder might die",
and its safety rests on bounded clocks and bounded pauses.

The standard failure: **a paused holder does not know it is dead.** A GC
pause, swap storm, or scheduler stall can outlast the lease; the holder
resumes and "may go ahead and make some unsafe change" (Kleppmann) while
a second holder legitimately owns the lease. Expiry converts "holder
died" into "two holders", silently.

The standard fix is the **fencing token**: a number that increases every
time the lock is acquired, sent with every write, and **checked by the
resource itself**, which rejects tokens older than one it has seen.
Chubby (Google's production lock service) ships this as the "sequencer"
carrying "the lock generation number" that a receiving server must
validate (Burrows, OSDI 2006). The load-bearing insight: the lock
service cannot protect a resource that does not participate — *the
resource is the last line of defense, not the lock*.

## The debate, honestly

Kleppmann's essay (2016) distinguishes locks held **for efficiency**
(duplicate work is waste) from locks held **for correctness** (duplicate
work is corruption), and argues lease-based locking without fencing is
only fit for the first. Sanfilippo's rebuttal ("Is Redlock safe?")
counters that pauses hurt the fencing scheme too — "the order in which
the token was acquired does not necessarily respect the order in which
the clients will attempt to work" — and defends Redlock's clock checks.
Both are practitioner essays; the fencing mechanism itself is the
peer-reviewed, production-proven part (Leases 1989, Chubby 2006), and
"lease + fencing checked at the resource" is the field's standard
answer for correctness-critical locks. Unbounded process pauses are
treated at length in Kleppmann, *DDIA* ch. 8.

## Standard rebuttals

- *"We'll set the timeout long enough."* Pauses are unbounded; a long
  timeout only trades double-holder frequency for slower recovery from
  real death. No timeout value is correct.
- *"The holder checks the clock before writing."* The pause can land
  *between* the check and the write — TOCTOU again (`toctou.md`).
- *"Heartbeats prove the holder is alive."* They prove the heartbeat
  thread runs, not that the work is current — see
  `liveness-and-identity.md`.

## Where posse deliberately diverges

**bd claims are leases-without-expiry** (ADR 0011): no auto-expiry, no
fencing token; recovery from a stranded claim is the operator
(`posse dispatch --resume`, visible in the cockpit). This is the standard
theory applied, not ignored — expiry without fencing is double-dispatch,
and bd, the resource, has no token check to fence with, so the
double-holder path is *removed* rather than fenced, at the price of
manual recovery. ADR 0011 Appendix A records the honest residual: manual
recovery is expiry with the operator as the clock — claim surgery while a
paused pass is mid-flight recreates Kleppmann's zombie writer, and no
real fence is buildable without the resource participating. The revisit
trigger is named: if beads grows leases, fence. A dispatch `--wait`
timeout is therefore a check-in, never a verdict.

## Sources (verified 2026-08-20)

- **[paper]** Gray & Cheriton, "Leases: An Efficient Fault-Tolerant
  Mechanism for Distributed File Cache Consistency", SOSP 1989 —
  https://web.eecs.umich.edu/~mosharaf/Readings/Leases.pdf (record:
  https://dl.acm.org/doi/10.1145/74850.74870, ACM bot-blocks fetches)
- **[paper]** Burrows, "The Chubby lock service for loosely-coupled
  distributed systems", OSDI 2006 —
  https://research.google/pubs/the-chubby-lock-service-for-loosely-coupled-distributed-systems/
- **[blog]** Kleppmann, "How to do distributed locking", 2016 —
  https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
- **[blog]** Sanfilippo, "Is Redlock safe?", 2016 —
  http://antirez.com/news/101
- **[book]** Kleppmann, *Designing Data-Intensive Applications*,
  O'Reilly 2017, ch. 8 (process pauses, fencing) — https://dataintensive.net/
