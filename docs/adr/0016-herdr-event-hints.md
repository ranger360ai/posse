# ADR 0016 — Herdr events are wake hints; level-triggered passes remain the truth

*Status: accepted 2026-08-26 · amended 2026-08-27 (ranger-base-7t1w: §1's
selector shape was measured wrong against herdr 0.8.0; the pane registry §1
rejected is now the decision) · amended 2026-09-02 (ranger-base-u5rqp: §2's
cockpit clause said "the existing full `refresh()`", which spends the
bead-scan cadence floor on every event; the sessions are re-read at event
latency, the bead lists stay on the floor) · amended 2026-09-04
(ranger-base-3wkxf: §1's planned reconnect is floored at one second inside a
burst; "bounded by seat churn" was measured to be no bound on this shop) ·
owner: architect · follows ADR 0011 and ADR 0013 (monica pulse)*

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
read as "the pane set moved" and cause a planned reconnect with a fresh list —
silent, because nothing failed, and immediate above the floor (amended
below). Detection events are an unreliable
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

*(Amended 2026-09-04, ranger-base-3wkxf, from the ranger-base-7hjy4
measurement.)* "Immediate" was written for a shop where a seat appears and
the fleet is otherwise still. MEASURED on the live socket (herdr 0.8.2,
protocol 19, 2026-09-04): a subscribe-only probe took 89 lifecycle envelopes
in 3.16 s and then 27 s of silence, and 29 of its 30 `pane_agent_detected`
named a pane that was not in the set the subscription had been dialled with.
The seats are real and distinct, so the reconnect the sentence above orders
was correct every time — and the tight loop it produced redialled 275 times in
32 s (8.6/s), each redial a forked `herdr agent list` (~33 ms wall), a dial and
a subscribe, about 6 % of one core per consumer, with the cockpit and the watch
each paying it. Two dials of the one subscription are therefore at least
**one second** apart, counted from where the dial being replaced *started* —
the same shape ranger-base-un5y5 put under the cockpit's hint-driven refresh
(`herdrRedialFloor`, `internal/posse/herdrevents.go`). Everything that asks
for a redial without an outage goes through the floor: a detection event, a
refresh poke, and a `pane_not_found` refusal, so a listing that keeps
returning an id herdr has already forgotten is also held to one dial a
second rather than spinning.

Immediate still holds exactly for the case the sentence describes. A seat
appearing on a shop that has been quiet longer than the floor is redialled the
moment its detection lands, because the connection it replaces is already
older than the floor; the word bends only inside a burst (pinned:
`TestHerdrHintsRedialIsImmediateAboveTheFloor` and
`TestHerdrHintsRedialFloorBoundsAStorm`, `internal/posse/herdrevents_test.go`,
landing with ranger-base-7hjy4). Inside a burst the floored redial carries a
*better* list than the one it replaces: `panes()` is read when the dial
happens, so waiting reads it later and one subscription covers every seat
that appeared during the wait, where the tight loop dialled once per event
with a list already stale on arrival.

What the floor costs is the gap this section already prices, made longer: a
pane that appears and settles inside the wait is missed by the stream and
swept by the timer. The value is bounded above by that sweep, MEASURED as
code: one second is half the cockpit's two-second completeness tick and orders
below the watch's `NextInterval`, so the floor never outlives the timer that
covers it. Its lower edge is the dial's own cost (the floor is
`max(1 s, that dial's cost)`, so anything under ~33 ms would be decorative).
Where inside that band it sits is ASSUMED — one second is the round number
under the ceiling, chosen once and cheap to move; the bead that moves it
brings a measurement.

Decode only the top-level event kind plus `workspace_id`, `pane_id`, and
`agent_status`; ignore unknown fields and event kinds. Protocol 19 pushes
underscore envelopes (`pane_agent_status_changed`) for dotted subscriptions
(`pane.agent_status_changed`), so the adapter explicitly folds both spellings
to one internal enum. It never applies an event payload to posse state.

The adapter publishes into a capacity-one hint channel. A burst means "look
again" once, not N refreshes. On dial, acknowledgement, decode, or EOF failure it
closes the connection, reports the transition once, and retries after a
context-cancellable five-second delay; a planned pane-set reconnect redials
as soon as the one-second floor above allows and reports nothing. A successful
acknowledgement clears the outage. There
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
`blocked` status change, which the watch's settle filter drops — re-reads the
sessions and redraws when the cockpit is in normal mode; while it is in
prompt/confirm/peek mode, record one dirty bit and refresh on return to normal
instead of drawing over operator input. The two-second refresh remains the
completeness path for beads-only changes, unsupported Herdr event kinds, and
subscriber failure. This bead is a latency change, not a promise to reduce poll
traffic.

*(Amended 2026-09-02, ranger-base-u5rqp.)* The clause above read "calls the
existing full `refresh()`", and the code took it literally. `refresh()` forces
the bead scan past its cadence floor, because every other caller of it — `c`,
`u`, `x`, `o`, `r`, a landed dispatch — just changed the bead lists itself. A
herdr event changes no bead, so an external stream on that path drove `bd` at
whatever rate herdr emits, past a floor whose stated job is to hold `bd` to at
most half the wall clock wherever a cockpit is open; worse, a force arriving
mid-scan is remembered and spent the moment that scan lands, so a sustained
stream chained scans with no gap. The hint path therefore re-reads the sessions
— the store the event is actually about, and the whole latency point — and
kicks the bead scan **on the floor**, exactly as the two-second tick does. That
is this section's own next sentence, not a new decision.

*(Reviewed 2026-09-02, ranger-base-v4923: accepted as a clarification; the code
in 189719e matches on both loops.)* What the bead lists pay for it, stated so
nobody rediscovers it: after a settle they lag by at most the floor — the larger
of `beadsEvery` and the last scan's own cost, counted from when it landed — plus
one tick, the bound every beads-only change already carries, while the session
row moves at event latency. The middle path, forcing the scan on `idle`/`done`
alone because those settles are so often a persona's own `bd close`, was priced
and refused: a settle is herdr's fact about a pane, not a bead write — seats end
without closing a bead, and beads close without a settle — and the floor's
guarantee is denominated in `bd`'s share of the wall clock, so a hole that opens
only on settles is still a hole, and a fleet finishing together chains through
it. The timer sweeps the gap inside one floor. The pin in
`cmd/posse/cockpithint_qa_test.go` exercises no settle, so the refusal is
remembered here and not yet enforced; ranger-base-zx1kz adds the `idle`/`done`
arm.

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
  with a freshly derived pane list, silently — never a reported outage; a
  storm of detection events causes at most one redial per floor, and a lone
  detection on a connection older than the floor redials without waiting
  (amended 2026-09-04);
- EOF/refusal/malformed input retries, while cancellation closes promptly;
- a watch with a long next interval starts a second pass on an idle/done event,
  never overlaps passes or holds the launcher lock while waiting, and still
  advances by its timer with no socket;
- both cockpit loops redraw from an event, preserve modal input, and still
  refresh by the two-second fallback with no socket; and
- no test reaches a real Herdr server (`go test ./...`, plus the Linux gate
  because the production adapter uses Unix sockets).

The protocol facts in Context are pinned by a live control arm, skipped by
default: `RHQ_LIVE_HERDR_EVENTS=1 go test ./internal/posse -run
TestLiveHerdrEvent` (`internal/posse/herdrevents_live_test.go`). It goes red if
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
  connection. Reconnects are planned, silent, and bounded by the floor — at
  most one dial per second per consumer, two per box with a cockpit and a
  watch both open, however fast seats churn (amended 2026-09-04; the earlier
  "bounded by seat churn, not by event volume" was measured to be the same
  number on a churning shop) — and a redial that loses an event costs
  latency, as above.
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
- *(2026-09-04, on the redial floor.)* **Keep "immediate" and pay the
  rate.** MEASURED: 8.6 redials/s, ~6 % of one core per consumer, on both
  consumers, for as long as the shop churns — and a redial storm is also
  load on herdr's one socket. Priced and refused: the floor is one constant
  and one `select`, and the sweep that pays for it already exists.
- **Compare the freshly derived pane set against the dialled one and skip
  the redial when they match.** The alternative the fix's own comment
  measured: 29 of 30 detections in the probe named a pane outside the
  dialled set, so on this shop it would have saved at most one redial in
  thirty — and it still forks `herdr agent list` per event, which is the
  cost. It also adds a comparison to a list this ADR insists is never
  posse's record.
- **Feed the pane list from the events themselves** — `pane_agent_detected`
  carries `pane_id`, so the registry could be amended from the stream
  without re-asking herdr. That is the registry becoming posse's own record,
  the one tooth §1 pulled: an id herdr has since forgotten refuses the whole
  subscription (`pane_not_found`), and the stream is exactly the source that
  is allowed to drop, duplicate, or replay. Re-deriving from the listing at
  every dial stays; the floor only spaces the dials.
- **An adaptive floor, or one per consumer.** Backing off with the burst,
  or a longer floor for the watch whose sweep is minutes, both fit — and
  neither has a measurement asking for it. A second number with no bead
  demanding it is the racing signal; one constant, moved by the bead that
  brings the measurement.
