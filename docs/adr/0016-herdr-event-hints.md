# ADR 0016 — Polling and bounded reconciliation own readiness

*Status: accepted 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · socket-hint removal approved; implementation deferred.*

## Context

Socket hints were a latency optimization beside authoritative polling. The
adapter also acquired pane discovery, reconnect scheduling, cancellation and
consumer refresh state. There is no measured ordinary-work capacity benefit
in the review sufficient to justify that machinery. The operator approved
its removal, with the latency observation a done-when row on the code task.

## Decision

Remove the Herdr event subscription and its watch/cockpit integration. Keep
level-triggered reconciliation: each pass reads current Herdr sessions and
beads, and acts on those readings under the existing launch lock. A missing,
duplicated or stale observation never authorizes a launch or frees a claim.

[ADR 0011](0011-dispatch-model.md) is the sole home for bounded passes,
rolling seats, selection, carried waits and reconciliation. Removing the
socket does not restore a gather-all barrier. In-flight seats survive a pass;
an unrelated slow or blocked seat cannot hold every other seat's next launch.
Keep the carry's local completion wake-up: it is the result of the wait this
process already owns, independent of Herdr's socket subscription. A completion
still triggers a fresh read; it is not permission to reuse a seat unchecked.

Watch keeps its timer, current base/backoff calculation, cancellation and
serial passes. Keep the gather cadence while work is in flight and central
watch throttling specified in 0011. Do not increase polling frequency to hide
the removed optimization, reset backoff merely on a wake, or add a replacement
broker. Current launch, claim and positive-reconciliation checks remain.
An unreadable session listing is unknown, not evidence that work ended.

The cockpit keeps both its TTY and display-only timer paths, session reads,
bead-scan cadence floor and protection of modal input. Manual operations that
actually change beads may still force their refresh. Remove event-only dirty
bits, pending-hint timers and pane-set refresh bookkeeping; retain any state
also required by ordinary refresh or operator input. The pulse remains its own
watch-owned ticker under [ADR 0027](0027-monica-pulse.md), using the shared
condition computation. No socket is required for completeness or delivery.

## Deferred removal and acceptance

The code bead deletes `internal/posse/herdrevents.go`, the `Dispatcher.Hints`
seam and subscription/refresh branches in `watch.go`, and event-only handling
in `cmd/posse/cockpit.go`, with their tests and obsolete selector/test-door
references. Preserve the shared socket resolver if non-event callers need it.
The implementation census decides the exact dependent test files; no runtime
or test edits are part of this documentation commit.

Price (ASSUMED file estimate, observed mechanism shape): 3–5 source files
plus tests; one connection and two goroutines per active subscription;
pane-set, retry, refresh and hint-channel state removed. No new config key,
operator flag, store or background process. Polling and carried worker waits
remain. This is a removal, not permission to build another notification seam.

First done-when row: **p95 seat-ready-to-next-dispatch latency with hints
disabled under ordinary work, holding reconciliation cadence fixed.** Record
the observation window, sample count and whether any material capacity loss
appears. This is controlled observation, not a load test. The code bead may
not describe the latency as acceptable without the observation; if it shows
material lost capacity, keep the mechanism and return that evidence for an
operator ruling. The document decision is approved now; the measurement is
not a prerequisite to committing it.

Other acceptance rows preserve concurrent seat progress, cancellation,
positive reconciliation, cadence floors, modal input, and useful refresh with
no event socket. What breaks if wrong: ready seats idle until reconciliation,
the cockpit displays old state longer, or an accidental gather barrier loses
capacity. An attempted polling compensation can also spend more CPU and bd
time than the removed subscription saved.

## Dated transition evidence (not a second active design)

Until the deferred bead lands, the existing subscriber and its tests remain.
Its literal protocol-19 request has unscoped `pane.agent_detected`,
`workspace.created`, `workspace.closed`, and one pane-scoped
`pane.agent_status_changed` per listed agent. These strings are retained here
because the current selector check reads this page; they do not require a
future implementation to subscribe. The old live control arm remains a dated
probe, not evidence about a new vendor version.

Measured 2026-08-27: unscoped settle subscriptions were refused; one stale pane
could refuse the connection; subscription changes required a reconnect.
Measured 2026-09-04: a burst caused 275 redials in 32 seconds before a
one-second reconnect floor was added. The floor bounds reconnect work while
accepting a longer gap covered by polling. The cockpit's separate bead-scan
floor corrected event-driven scans that otherwise chained without a gap.
Those safeguards stay in the current implementation until its removal.

## Alternatives rejected

- Keep a subscription solely for hypothetical sub-poll capacity: measure its
  actual benefit on the removal task; the code census alone cannot prove it.
- Restore blocking gather or treat a socket event as a claim: changes
  correctness and concurrency, beyond the latency optimization being removed.
- Share a connection through a daemon: adds a process and IPC protocol to
  replace a mechanism whose durable state is already empty.

## Lineage

| Record | Surviving decision |
|---|---|
| 0016, 2026-08-26 through 2026-09-04 | Protocol and incident evidence archived; socket implementation pending removal |
| 0011, including former 0028 | Sole authority for rolling bounded reconciliation |
| Operator ruling 2026-09-05 | Approves removal; measurement belongs to deferred code acceptance |

Prior decision and measurements: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0016-herdr-event-hints.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
