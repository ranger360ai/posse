---
name: distributed-systems
description: Use when designing, reviewing, or building anything where concurrent actors share state — locks, leases, queues, work claims, prune/cleanup of shared records, retries, timeouts, liveness checks, or a fact read from one store and acted on in another. Before inventing a mechanism or coining a vocabulary, load references/<concept>.md — the field has a standard answer and a named failure mode for each of these.
---

# Distributed systems — the canon concurrent work keeps rediscovering

One rule: **before you invent, check whether the field already named it.**
Every concept below has a fifty-year-old name, a standard answer, and a
named failure mode, and every one of them is routinely re-derived at the
cost of an incident. The reference files carry the mechanism, the failure
modes, the standard rebuttals, and the primary sources — load only the
one in play; do not read all seven.

## Index — reach for the file when you see the trigger

| trigger in the work | concept | file |
|---|---|---|
| `if <check> then <act>` on state another actor can mutate | check-then-act races | `references/toctou.md` |
| a lock or claim with a timeout; "the holder might be paused/slow" | leases & fencing tokens | `references/fencing-and-leases.md` |
| "is that process alive?" / pidfiles / stale liveness state | liveness vs identity | `references/liveness-and-identity.md` |
| retries, `--wait` timeouts, "did it run twice?", message redelivery | at-least-once & idempotency | `references/delivery-and-idempotency.md` |
| a work queue on a database or issue tracker; claiming without races | database-as-queue | `references/db-as-queue.md` |
| deleting/pruning a record something might still be using | safe reclamation | `references/safe-reclamation.md` |
| "who owns this fact?"; the same fact in two stores; coordination cost | single writer & store of record | `references/single-writer-and-stores.md` |

## How to use it in a bead

- **Design bead**: name which concepts the design touches, cite the
  standard answer, and put the known failure modes in *Alternatives
  rejected* — each file's "standard rebuttals" section is written for
  exactly that section of an ADR.
- **Code/review bead**: check the mechanism you are building or judging
  against the file's failure-mode list before trusting it.
- Where the harness **deliberately diverges** from the field's answer,
  the file says so and cites the ADR (e.g. bd claims are
  leases-without-expiry — ADR 0011). Divergence is allowed;
  *undocumented* divergence is not.

## Your instance's answers go in a local appendix

This is the shipped, generic canon: what any deployer could have written
(ADR 0012 D2). Your own incidents, measured numbers, and tracker ids are
instance facts and do not belong in these files — a `posse init` of a
newer release would be arguing with them anyway. Keep them beside the
canon instead: add `references/<concept>.local.md` in your
`RHQ_HOME/skills/distributed-systems/`, and let it carry "our answer" —
the incident that taught you the invariant, the ADR section that decided
it, the number you settled on.

Going the other way, a lesson that survives with your facts removed is
harness-worthy: generalize it by CONTRIBUTING.md's rule and send it
upstream to this file rather than keeping it local.

## Sourcing conventions (hold the next editor to these)

Every claim carries a primary source verified **live** on the date given
in the file's Sources section, each labeled **[paper]**, **[docs]**, or
**[blog]** — a blog can be right, but say which kind of authority you are
leaning on. ACM/O'Reilly pages bot-block fetches, so DOIs are given as
the citation of record there; one canonical post (Ringer on SKIP LOCKED)
is dead at its original URL and cited via the Wayback Machine, marked as
such. Your own recall is not a source: re-verify before adding a
citation.
