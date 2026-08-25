# Single writer, and the store of record

## The concept

**Single-writer principle** (Thompson): contention management — locks,
CAS loops, coordination protocols — is overhead you pay only because
multiple writers mutate the same resource; if "any item of data, or
resource, is only mutated by a single writer", the machinery vanishes.
Queue the *requests* to the owner instead of contending on the state.
Helland's CIDR 2007 paper is the distributed form: scalable systems
partition state into uniquely-identified **entities**, each a "disjoint
scope of transactional serializability" with one serializing owner, and
between entities use at-least-once messaging with idempotent handling
(`delivery-and-idempotency.md`) — coordination is pushed to the edges
so it never spans stores.

**Store of record.** For every fact, exactly one store should be the
authority; everything else holding that fact holds a *derived copy*.
Helland (CIDR 2005): data leaving its store of record "is clearly from
the past and not now — it is reasonable to consider these versions as
snapshots." Kleppmann (*DDIA*, Part III) draws the same line as systems
of record vs derived data. The corollary that bites: **a fact readable
in N stores is a fact that can disagree N(N−1)/2 ways**, and every
cross-store inference is a TOCTOU (`toctou.md`) between two clocks.
Either nominate the authority and demote the other copies to hints, or
pay a coordination protocol you didn't mean to sign up for.

## Standard rebuttals

- *"A single writer is a bottleneck."* Measure first: one uncontended
  writer is usually faster than N contended ones (Thompson's point),
  and the bottleneck, if real, shards by entity (Helland), not by
  adding writers to one resource.
- *"A single writer is a SPOF."* So is a coordination protocol that
  loses quorum — the honest comparison is recovery story vs recovery
  story, and single-writer recovery (restart, kernel-released lock —
  `liveness-and-identity.md`) is usually the simpler one.
- *"We'll keep the copies in sync."* That is either the coordination
  protocol you were avoiding, or eventual consistency — fine, but then
  reads of the copy are hints and must be treated as such.
- *"One more small store won't hurt."* Each new store adds N−1 new
  pairwise disagreement channels: four stores is six ways to disagree,
  and each one is somebody's incident waiting to be written up.

## How posse applies it

ADR 0011 end to end. Its diagnosis is this file's corollary in one
sentence — four stores updated independently and read at different
instants, and "every incident in the class is one store's momentary
reading taken as evidence about another store's durable fact". §1 buys
the single-writer property with a kernel lock "for ~30 lines and zero new
state" (the single-writer *daemon* was the clever alternative, rejected
for the IPC substrate the harness would then own). §3 promotes the
session meta to the **run record** — "wherever dispatch needs a run fact,
it reads the record it wrote instead of inferring from a name pattern or
a snapshot." bd stays the store of record for work, and a second queue
store is rejected by name. DIRECTION's memory table is the same
discipline at the harness level: each memory kind has exactly one owning
store, and posse deliberately owns only one of them.

## Sources (verified 2026-08-20)

- **[blog]** Thompson, "Single Writer Principle", Mechanical Sympathy,
  2011 —
  https://mechanical-sympathy.blogspot.com/2011/09/single-writer-principle.html
- **[paper]** Helland, "Life beyond Distributed Transactions: an
  Apostate's Opinion", CIDR 2007 —
  https://www.cidrdb.org/cidr2007/papers/cidr07p15.pdf
- **[paper]** Helland, "Data on the Outside versus Data on the Inside",
  CIDR 2005 — https://www.cidrdb.org/cidr2005/papers/P12.pdf
  (republished CACM 2020: DOI 10.1145/3410623)
- **[book]** Kleppmann, *Designing Data-Intensive Applications*,
  Part III intro (systems of record vs derived data) —
  https://dataintensive.net/
