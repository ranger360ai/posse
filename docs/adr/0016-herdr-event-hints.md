# ADR 0016 — Herdr events are wake hints; level-triggered passes remain the truth

*Status: accepted 2026-08-26 · amended 2026-08-27 (ranger-base-7t1w: §1's
selector shape was measured wrong against herdr 0.8.0; the pane registry §1
rejected is now the decision) · owner: architect · follows ADR 0011 and
ADR 0013 (monica pulse)*

## Context

`posse cockpit` re-reads Herdr and beads every two seconds
(`cmd/posse/cockpit.go`); `posse dispatch --watch` runs a pass and waits on
`time.After(NextInterval(...))` (`internal/rhq/watch.go`). Both are correct but
late: an agent can become idle just after a tick and leave its next bead waiting
for the whole backoff. ADR 0013's pulse has the same shape: its shop check is
level-triggered, but an agent-state event can ask it to check early.

Herdr 0.8.0, socket protocol 19, exposes `events.subscribe` as newline-delimited
JSON on the local API socket. Its CLI has snapshot/schema commands but no
long-lived event-stream command. The original text here claimed "the installed
schema accepts unscoped subscriptions"; MEASURED on the live socket
(2026-08-27, ranger-base-0s36 / ranger-base-7t1w) that holds for every
subscribed event except the one that matters. The protocol facts any
implementation must be built around:

- `pane.agent_status_changed` — the settle event — **requires `pane_id`**. An
  unscoped subscription is refused (`invalid_request: missing field pane_id`)
  and the refusal closes the connection.
- A connection takes exactly **one** `events.subscribe`. A second request on a
  live connection is not answered; the server closes it. Amending the
  subscription means reconnecting.
- One request may freely mix unscoped and pane-scoped subscriptions, answered
  by a single `subscription_started`.
- A `pane_id` herdr does not know is refused (`pane_not_found`) and takes the
  whole connection — one stale id costs every subscription on it.

The project already has the exact socket resolver in `herdrSocketPath`. Posse
ships only Darwin and Linux, so the Unix-socket boundary does not narrow its
release surface.

Events cannot be the store of record. They may be duplicated, replayed on
subscribe by some Herdr versions, arrive just before a disconnect, or be lost
while the server hands off. They also say nothing about changes made only in
beads. ADR 0011 therefore still governs: an event changes **when** posse reads
Herdr and beads, never what it trusts.

## Decision

### 1. One event-hint adapter, one connection per long-running process

Add a small adapter beside `Herdr.Run` in `internal/rhq` using only `net` and
`encoding/json`. It resolves the same socket as the CLI, opens one Unix-socket
connection, writes one `events.subscribe` request, verifies the matching request
id and `subscription_started` acknowledgement, then decodes pushed envelopes
until context cancellation or EOF. The one request carries two kinds of
subscription:

- unscoped, the lifecycle events: `pane.agent_detected`, `workspace.created`,
  `workspace.closed`;
- pane-scoped, the settle event: one `pane.agent_status_changed` entry per
  pane herdr currently reports an agent in. Unscoped is not an option —
  protocol 19 refuses it and closes the connection (measured; the live control
  arm in §3 goes red if a later herdr relents).

The pane list is the mutable registry this section originally rejected, blessed
now that it is the only route to the settle event — with its teeth pulled: it
is never persisted and is never posse's own record. It is re-derived from
herdr's agent listing at every dial, both because that listing is its only
truth and because one stale id refuses the entire subscription
(`pane_not_found`; the refusal is a reason to redial with a fresh list, never a
reported outage). Since a connection takes exactly one subscribe, the set
cannot be amended in place: `pane.agent_detected` and `workspace.created` are
read as "the pane set moved" and cause an immediate planned reconnect with a
fresh list — silent, because nothing failed. Detection events are an unreliable
announcer (measured: a seat that appeared after the dial settled unseen, with
no detection event arriving), so the adapter also exposes a refresh poke, fed
from each consumer's own truth path — the watch after every pass, whose
cadence is minutes; the cockpit only when a refresh shows a pane set
different from the one the subscription was dialled with, because poking on
its two-second tick would redial constantly. The residual gap, a pane that appears and settles between two
dials, is bounded and swept by the timer, which is this ADR's completeness
guarantee anyway. This is the field's list-then-watch shape with a periodic
resync: the agent listing is the list, the subscription the watch, the
pass-poke the resync — the standard answer for exactly this gap, not a new
mechanism.

Decode only the top-level event kind plus `workspace_id`, `pane_id`, and
`agent_status`; ignore unknown fields and event kinds. Protocol 19 pushes
underscore envelopes (`pane_agent_status_changed`) for dotted subscriptions
(`pane.agent_status_changed`), so the adapter explicitly folds both spellings
to one internal enum. It never applies an event payload to posse state.

The adapter publishes into a capacity-one hint channel. A burst means "look
again" once, not N refreshes. On dial, acknowledgement, decode, or EOF failure it
closes the connection, reports the transition once, and retries after a
context-cancellable five-second delay; a planned pane-set reconnect redials at
once and reports nothing. A successful acknowledgement clears the outage. There
is no cursor, checkpoint, replay request, or persisted subscriber state.

"One adapter" does not mean a new broker. The cockpit and watch loop are
separate processes, so when both run there are two Herdr connections, each owned
and cancelled by its process. Sharing one physical connection would require a
daemon and an IPC protocol, larger and less replaceable than this feature.

### 2. Consumers re-run their existing truth paths

**Watch.** Start the subscriber once for the lifetime of `Watch`. After every
pass, wait on one resettable timer, the hint channel, or `ctx.Done()`.
`pane.agent_status_changed` with `idle` or `done`, and `workspace.closed`, wake
the next pass early; other subscribed events are ignored by this consumer.
After every pass, poke the adapter's pane-set refresh (§1) — the pass just
re-read the shop, so it is the authority on the seat list having moved. Before
an event-driven pass, stop and drain the old timer; after every pass arm
exactly one new timer. A hint received while a pass is running stays coalesced
and causes at most one immediate following pass. Passes remain serial and still
take ADR 0011's launcher lock.

`blocked` is deliberately not a settle for this consumer, although dispatch's
own `AgentWait` settles on it. A blocked seat is not free: the bead claim is
kept and the working/blocked guard still denies that persona a launch, so a
woken pass has nothing it can do with the seat — and herdr reads transient
states as `blocked` (a session sitting on a splash screen, `dispatch.go`), so
treating it as a settle would wake empty passes at interruption frequency. The
timer judges blocked seats; the operator-visible ⛔ flag arrives at tick
latency in the watch log and at event latency in the cockpit, whose consumer
applies its own filter to the same stream — the subscription carries every
status change; the settle filter is client-side. This is the semantics ADR
0028 §1's refill (S4) inherits: a refill fires on `idle`/`done` and
`workspace.closed` only.

An event does not reset backoff. The pass result still goes through
`NextInterval`: work actually dispatched resets to the base interval; another
quiet pass backs off. The existing timer is always armed, even while the stream
is healthy. Thus stream death cannot miss a pass, and a replayed or duplicate
event costs only a harmless early level-triggered pass. The pulse implementation
may consume the same agent-status hint to run its existing shop check early; no
judgement or fact is added to the event payload.

**Cockpit.** The TTY and display-only loops select on the same hint channel in
addition to their existing two-second tick. Any subscribed event — including a
`blocked` status change, which the watch's settle filter drops — calls the
existing full `refresh()` and redraws when the cockpit is in normal mode; while
it is in prompt/confirm/peek mode, record one dirty bit and refresh on return to
normal instead of drawing over operator input. The two-second refresh remains
the completeness path for beads-only changes, unsupported Herdr event kinds,
and subscriber failure. This bead is a latency change, not a promise to reduce
poll traffic.

Subscriber failure is non-fatal. Watch writes one `events unavailable — polling`
line per outage and one recovery line; the cockpit uses its status area. Neither
command exits, skips a scheduled pass, clears a list, or lengthens its current
fallback interval because the socket is absent or unreadable.

### 3. Hermetic fake and done-when checks

Do not turn the current fake CLI into a daemon. `fakeHerdr` is intentionally one
re-exec per CLI request and exits after its JSON envelope; making it own a
long-lived socket would entangle every existing backend test. Add a small test
Unix-socket server under `internal/rhq` instead. It validates the subscribe
request and id, sends the real acknowledgement, can push each selected envelope,
and can force malformed JSON, EOF, refusal, and a replacement listener. Existing
fake CLI files remain the truth that `Sessions()` and `Dispatcher.Run` re-read.

Done when hermetic tests prove:

- one mixed request — the three lifecycle selectors unscoped plus a
  pane-scoped settle entry per agent pane — acknowledged by a single
  `subscription_started`; unknown events are ignored and a burst never blocks
  the socket reader;
- `pane_not_found`, a detection event, and a refresh poke each cause a redial
  with a freshly derived pane list, silently — never a reported outage;
- EOF/refusal/malformed input retries, while cancellation closes promptly;
- a watch with a long next interval starts a second pass on an idle/done event,
  never overlaps passes or holds the launcher lock while waiting, and still
  advances by its timer with no socket;
- both cockpit loops redraw from an event, preserve modal input, and still
  refresh by the two-second fallback with no socket; and
- no test reaches a real Herdr server (`go test ./...`, plus the Linux gate
  because the production adapter uses Unix sockets).

The protocol facts in Context are pinned by a live control arm, skipped by
default: `RHQ_LIVE_HERDR_EVENTS=1 go test ./internal/rhq -run
TestLiveHerdrEvent` (`internal/rhq/herdrevents_live_test.go`). It goes red if
a later herdr accepts the unscoped settle subscription — the signal to
revisit this amendment, not to keep the registry out of loyalty.

## Consequences

- Idle-to-next-bead and agent-state-to-cockpit latency becomes one local socket
  delivery plus the existing level-triggered read; fallback latency is exactly
  today's two-second tick / `NextInterval` schedule.
- Duplicates and gaps do not create a second correctness model. The launch lock,
  bd claim, current Herdr listing, and current bead remain authoritative. That
  now includes the registry's own gap: a pane that appears and settles between
  two dials is missed by the stream and swept by the timer.
- The settle path costs reconnects: every pane-set movement redials the one
  connection. Reconnects are planned, silent, and bounded by seat churn — not
  by event volume — and a redial that loses an event costs latency, as above.
- The dependency holds no state hostage. If Herdr adds a CLI stream, changes the
  socket protocol, or is replaced, only the hint adapter changes; both consumers
  keep a `<-chan hint` plus their timers. Deleting the adapter restores today's
  behavior byte-for-byte.

## Alternatives rejected

- **Subscribe to the settle event unscoped** — the original decision here,
  and the clean one: no registry, no reconnects. Refused by the actual
  protocol: `pane.agent_status_changed` requires `pane_id`, and the refusal
  closes the connection (measured 2026-08-27, pinned by the §3 control arm).
  The old rejection argued the registry "would miss panes created after it and
  require a second mutable registry"; both halves were right and neither is
  disqualifying — the miss is bounded by the timer, and the registry holds no
  state herdr does not already own.
- **Recover the settle from unscoped `pane.updated` by edge detection.** It is
  accepted unscoped and carries the whole PaneInfo, `agent_status` included —
  it looks like the substitute, and it shipped for a day. Measured twice, on
  two seats, minutes apart: it does not carry the settle. A working pane emits
  it several times a second (3707 envelopes in 15 minutes on a five-seat
  fleet); a settling pane simply stops emitting, and the idle/done transition
  rides only on `pane.agent_status_changed`. Level-polling that stream is not
  a substitute for the edge; it is 4 Hz of noise and a way to never see one.
- **Treat `blocked` as a settle for the watch/refill.** Dispatch's `AgentWait`
  settles on it, so symmetry argues for it — but a blocked seat is not free
  (claim kept, persona launch-guarded), so the woken pass can do nothing, and
  transient states read as `blocked`. The cockpit gets `blocked` at event
  latency through its own filter; the timer judges the seat for the watch.
- **Use a Herdr CLI stream.** There is none in 0.8.0. Repeated `events.wait`
  calls recreate polling and add a request race; the raw socket is the documented
  surface for long-lived subscriptions.
- **Apply event payloads directly to cockpit rows or dispatch decisions.** This
  makes replay/order/gap handling a new state machine and cannot observe beads.
  Re-reading the existing truth path is smaller and failure-safe.
- **Disable timers while subscribed.** A live socket is not proof that every
  relevant event was delivered, and Herdr has no event for a beads-only change.
  The timer is the completeness guarantee, not wasted duplicate authority.
- **One shared subscriber daemon.** It saves one local connection by adding
  process supervision, IPC, client recovery, and another liveness store — the
  single-writer daemon rejected in ADR 0011 for the same reason.
