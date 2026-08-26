# ADR 0016 — Herdr events are wake hints; level-triggered passes remain the truth

*Status: accepted 2026-08-26 · owner: architect · follows ADR 0011 and
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
long-lived event-stream command. The installed schema accepts unscoped
subscriptions and acknowledges them with `subscription_started`; the project
already has the exact socket resolver in `herdrSocketPath`. Posse ships only
Darwin and Linux, so the Unix-socket boundary does not narrow its release
surface.

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
until context cancellation or EOF. Subscribe, unfiltered, to:

- `pane.agent_detected`
- `pane.agent_status_changed`
- `workspace.created`
- `workspace.closed`

Unfiltered is deliberate: a subscription scoped to today's pane ids would miss
panes created after it and require a second mutable registry. Decode only the
top-level event kind plus `workspace_id`, `pane_id`, and `agent_status`; ignore
unknown fields and event kinds. Protocol 19 exposes both lifecycle envelopes
(`workspace_created`, `pane_agent_status_changed`) and dotted subscription
envelopes, so the adapter explicitly maps the schema's dotted/underscore
spellings for these four events to one internal enum. It never applies an event
payload to posse state.

The adapter publishes into a capacity-one hint channel. A burst means "look
again" once, not N refreshes. On dial, acknowledgement, decode, or EOF failure it
closes the connection, reports the transition once, and retries after a
context-cancellable five-second delay. A successful acknowledgement clears the
outage. There is no cursor, checkpoint, replay request, or persisted subscriber
state.

"One adapter" does not mean a new broker. The cockpit and watch loop are
separate processes, so when both run there are two Herdr connections, each owned
and cancelled by its process. Sharing one physical connection would require a
daemon and an IPC protocol, larger and less replaceable than this feature.

### 2. Consumers re-run their existing truth paths

**Watch.** Start the subscriber once for the lifetime of `Watch`. After every
pass, wait on one resettable timer, the hint channel, or `ctx.Done()`.
`pane.agent_status_changed` with `idle` or `done`, and `workspace.closed`, wake
the next pass early; other subscribed events are ignored by this consumer.
Before an event-driven pass, stop and drain the old timer; after every pass arm
exactly one new timer. A hint received while a pass is running stays coalesced
and causes at most one immediate following pass. Passes remain serial and still
take ADR 0011's launcher lock.

An event does not reset backoff. The pass result still goes through
`NextInterval`: work actually dispatched resets to the base interval; another
quiet pass backs off. The existing timer is always armed, even while the stream
is healthy. Thus stream death cannot miss a pass, and a replayed or duplicate
event costs only a harmless early level-triggered pass. The pulse implementation
may consume the same agent-status hint to run its existing shop check early; no
judgement or fact is added to the event payload.

**Cockpit.** The TTY and display-only loops select on the same hint channel in
addition to their existing two-second tick. Any of the four events calls the
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

- acknowledgement and all four selectors; unknown events are ignored and a
  burst never blocks the socket reader;
- EOF/refusal/malformed input retries, while cancellation closes promptly;
- a watch with a long next interval starts a second pass on an idle/done event,
  never overlaps passes or holds the launcher lock while waiting, and still
  advances by its timer with no socket;
- both cockpit loops redraw from an event, preserve modal input, and still
  refresh by the two-second fallback with no socket; and
- no test reaches a real Herdr server (`go test ./...`, plus the Linux gate
  because the production adapter uses Unix sockets).

## Consequences

- Idle-to-next-bead and agent-state-to-cockpit latency becomes one local socket
  delivery plus the existing level-triggered read; fallback latency is exactly
  today's two-second tick / `NextInterval` schedule.
- Duplicates and gaps do not create a second correctness model. The launch lock,
  bd claim, current Herdr listing, and current bead remain authoritative.
- The dependency holds no state hostage. If Herdr adds a CLI stream, changes the
  socket protocol, or is replaced, only the hint adapter changes; both consumers
  keep a `<-chan hint` plus their timers. Deleting the adapter restores today's
  behavior byte-for-byte.

## Alternatives rejected

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
