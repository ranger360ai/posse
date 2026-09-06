# ADR 0016 — Polling and bounded reconciliation own readiness

*Status: accepted 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · socket-hint removal approved and landed 2026-09-06 (ranger-base-4dxpo).*

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

## The removal, as landed

`internal/posse/herdrevents.go`, the `Dispatcher.Hints` seam and watch's
subscription/refresh branches, and the cockpit's hint channel, floor, dirty
and pending bits, pane-set poke and report line are gone (ranger-base-4dxpo),
with their tests and the `selector-check` tree door. `SocketID()` stays — it
is the shared resolver, and the fake-CLI reads still spell a socket. The one
non-event caller of the deleted file's vocabulary, dispatch's `--resume`
holder check, reads `settledStatus` in `govern.go`, which was already the
same predicate.

Price, against the ASSUMED estimate of 3–5 source files plus tests, and it
landed inside it: one source file deleted (`herdrevents.go`) and four edited
(`watch.go`, `cockpit.go`, `dispatch.go` for the seam, `passcarry.go` for the
prose that named the socket); five test files deleted, four edited, two added
for the acceptance rows below. One connection and two goroutines per active
subscription are gone, as are pane-set, retry, refresh and hint-channel state.
No new config key, operator flag, store or background process. Polling and
carried worker waits remain. This was a removal, not permission to build
another notification seam.

First done-when row: **p95 seat-ready-to-next-dispatch latency with hints
disabled under ordinary work, holding reconciliation cadence fixed** —
controlled observation, not a load test. Observed on the operator's own
`state/dispatch-watch.log` over one 15h31m loop block at a cadence pinned to
3m every pass, and it needed no counterfactual: the loop under observation
already carried the subscriber, and it woke a pass on a hint **zero** times
in 142 passes with no outage line, so the window IS the hints-disabled one.
Of 95 seat refills, the 41 where ready work existed have p50 22s, p90 1m46s,
**p95 2m6s**, max 2m48s — every one inside one reconciliation interval, so
the removed mechanism could not have saved a seat more than the interval it
never had to wait. The longer tail is the other 54, where the seat waited on
the QUEUE and not on dispatch; a hint wakes a pass, and a pass with no ready
bead for that persona dispatches nothing. No material capacity loss appears.

Why the subscriber delivered nothing is measured, not assumed. Three arms
sharing one live 10-minute window, same socket, same panes: a raw connection
dialled ONCE with posse's own request shape took 63 envelopes, 23 of them
settles; the settle-filtered production adapter took 0; the unfiltered one
took 599, every one a `pane.agent_detected`. On this shop a detection is a
planned reconnect and they arrive faster than a dial completes, so the
adapter spent its life redialling and the connection was rarely up when a
settle went past. The subscription was not a latency trade this shop was
paying — it was redial cost for a signal that never arrived. Numbers on
ranger-base-4dxpo; this page keeps the shape.

Other acceptance rows preserve concurrent seat progress, cancellation,
positive reconciliation, cadence floors, modal input, and useful refresh with
no event socket, and each is pinned. The carry's local wake is
`internal/posse/passcarry_qa_test.go` behaviourally and
`internal/posse/herdrhintsgone_qa_test.go` structurally, which also holds
watch's timer, backoff and cancellation arms and censuses this package for the
removed vocabulary; `cmd/posse/cockpitcadence_qa_test.go` holds both cockpit modes on their
two-second tick, keeps the forced bead scan off either cadence path, holds the
tick's redraw behind its mode gate — which since this removal is the whole of
the modal-input protection, the tick being the only self-starting repaint left
— and censuses `cockpit.go`. Every one of those arms was shown able to fail by
mutation before it was committed (nine mutants, each restored;
ranger-base-4dxpo).

What breaks if wrong: ready seats idle until reconciliation,
the cockpit displays old state longer, or an accidental gather barrier loses
capacity. An attempted polling compensation can also spend more CPU and bd
time than the removed subscription saved.

## Dated transition evidence (not a second active design)

The subscriber and its tests are gone; what follows is why they were shaped
the way they were, kept as history. Nothing here requires a future
implementation to subscribe, and no check reads these strings any more.

Measured 2026-08-27: unscoped settle subscriptions were refused; one stale pane
could refuse the connection; subscription changes required a reconnect.
Measured 2026-09-04: a burst caused 275 redials in 32 seconds before a
one-second reconnect floor was added. The floor bounds reconnect work while
accepting a longer gap covered by polling. The cockpit's separate bead-scan
floor corrected event-driven scans that otherwise chained without a gap.
The one-second floor is measured above as the mechanism's undoing rather
than its fix: it bounded the redial rate without ever letting a subscription
live long enough to deliver. The cockpit's bead-scan floor is not part of
that and stays — it is ordinary refresh cadence, kept by the Decision.

## Alternatives rejected

- Keep a subscription solely for hypothetical sub-poll capacity: measured on
  the removal task, and the benefit was zero — the code census alone could
  not have proved that either way, which is why the measurement was a
  done-when row and not a footnote.
- Restore blocking gather or treat a socket event as a claim: changes
  correctness and concurrency, beyond the latency optimization being removed.
- Share a connection through a daemon: adds a process and IPC protocol to
  replace a mechanism whose durable state is already empty.

## Lineage

| Record | Surviving decision |
|---|---|
| 0016, 2026-08-26 through 2026-09-04 | Protocol and incident evidence archived; socket implementation removed 2026-09-06 |
| 0011, including former 0028 | Sole authority for rolling bounded reconciliation |
| Operator ruling 2026-09-05 | Approved the removal; ranger-base-4dxpo landed it and took the measurement |

Prior decision and measurements: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0016-herdr-event-hints.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
