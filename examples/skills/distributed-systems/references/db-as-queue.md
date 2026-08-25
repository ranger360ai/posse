# Database-as-queue

## The concept

The folklore says "don't use your database as a queue"; the field's
modern answer is that a database queue is fine — **if you use the
database's own claim primitive instead of inventing one**. In Postgres
that is `SELECT … FOR UPDATE SKIP LOCKED`: "any selected rows that
cannot be immediately locked are skipped. Skipping locked rows provides
an inconsistent view of the data, so this is not suitable for general
purpose work, but can be used to avoid lock contention with multiple
consumers accessing a queue-like table" (Postgres docs — the docs
themselves name the queue use-case). Claim = atomic check-then-act
inside the store's concurrency control (`toctou.md` answer #1); SKIP
LOCKED removes the herd blocking on the head row.

Ringer's canonical walkthrough opens with the observation that most
homegrown DB work-queues are "buggy in one of a few ways" — the bugs
being exactly this canon: non-atomic claim (two workers, one row),
blocking claim (head-of-line convoy), and claims tied to a connection
so a crash silently releases the row — which is **auto-expiry**, i.e.
at-least-once redelivery, which demands idempotent handlers
(`delivery-and-idempotency.md`) whether you noticed or not.

**Tradeoffs vs a dedicated broker**, honestly: the DB queue buys
transactional enqueue/claim *with your data* (state change and queue
entry commit or don't, together — no dual-write) and one store fewer
(`single-writer-and-stores.md`). It costs polling (or LISTEN/NOTIFY
plumbing), a throughput ceiling, and dead-row/vacuum churn on hot queue
tables. For work measured in tasks-per-minute, not messages-per-second,
the DB side of the trade wins.

## Standard rebuttals

- *"We need a real queue (Rabbit/SQS/Kafka)."* At low throughput that
  buys a second store of record and a dual-write problem, and you still
  need idempotent consumers. Name the throughput before buying it.
- *"Row locks make claims automatic."* Connection-scoped claims are
  leases-without-fencing (`fencing-and-leases.md`): crash-release
  redelivers work that may still be running elsewhere in a degraded
  form. Decide *explicitly* whether claims should survive the claimer.
- *"We'll poll faster / add LISTEN-NOTIFY later."* Fine — that changes
  when you look, not what you trust; keep the claim atomic either way.

## How posse applies it

ADR 0011, verbatim: "**No queue. bd is the queue** — dependency-aware,
atomically claimed, aggregated across repos. A second queue store would
be a fifth store and the disagreement class's next member." `bd update
--claim` is the atomic primitive; losing a race is a clean skip, and the
outcome is read from the bead, not from the exit code. The deliberate
divergence from the connection-scoped norm: bd claims are *durable*
assignments, not connection lifetimes — connection-scoped release is
auto-expiry, auto-expiry is redelivery, and redelivery demands an
idempotence dispatch does not have, since prompting a live agent is not
idempotent (`delivery-and-idempotency.md`). A work queue with
lease/heartbeat/expiry is in ADR 0011's alternatives-rejected by name.

## Sources (verified 2026-08-20)

- **[docs]** PostgreSQL, SELECT — The Locking Clause (`SKIP LOCKED`) —
  https://www.postgresql.org/docs/current/sql-select.html
- **[blog]** Ringer, "What is SKIP LOCKED for in PostgreSQL 9.5?",
  2ndQuadrant 2016 — original URL dead; cite the archive:
  https://web.archive.org/web/*/blog.2ndquadrant.com/what-is-select-skip-locked-for-in-postgresql-9-5/
- **[paper]** Helland, "Life beyond Distributed Transactions", CIDR
  2007 (at-least-once messaging between single-owner entities) —
  https://www.cidrdb.org/cidr2007/papers/cidr07p15.pdf
