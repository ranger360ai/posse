## The selector the ADR could not have (ranger-base-0s36)

ADR 0016 §1 names four `events.subscribe` selectors and rests them on one
sentence: *"The installed schema accepts unscoped subscriptions."* Three of
the four do. The fourth — `pane.agent_status_changed`, the only one §2's
watch consumer is actually written around — does not, and finding out costs
one connection:

    $ probe '[{"type":"pane.agent_status_changed"}]'
    {"id":"","error":{"code":"invalid_request",
     "message":"invalid request: missing field `pane_id` at line 1 column 123"}}
    EOF

Two facts in four lines. The subscription is refused for want of a pane id,
and the refusal **closes the connection** — so a selector list is not a list
at all on the wire, it is a chain whose weakest member takes the stream down
with it. Probe one type per connection or you cannot say which member failed.

The ADR anticipated the shape of the trap and rejected the escape: scoping
the subscription to today's pane ids "would miss panes created after it and
require a second mutable registry." What it did not have is that there is no
third option, and the second option is what nearly shipped.

### The substitute that would have shipped a channel that never fires

`pane.updated` **is** accepted unscoped, and it carries the whole `PaneInfo`
— `agent_status` and `workspace_id` included — on every revision bump. That
looks exactly like a level-triggered stream you can edge-detect: keep the
last status per pane, fire on the transition into idle or done, no registry.
It reads well, it passes a hermetic test that pushes both levels, and it is
wrong.

A working pane emits `pane.updated` several times a second — 3707 envelopes
in fifteen minutes on a five-seat fleet. A **settling** pane emits nothing.
The output stops changing, so the updates stop, and the status change rides
only on `pane.agent_status_changed`. Measured on two seats, minutes apart,
with both subscriptions live on one connection:

      241.5s pane.agent_status_changed    wVQ:p1   -> done
      486.4s pane.agent_status_changed    wVS:p1   -> done

and `pane_updated` never reported either one, before or after. Level-polling
that stream is not a substitute for the edge; it is a way to never see one.

What caught it was not a test. The hermetic suite was green, both DONE WHEN
arms passed, and the ADR-shaped story was written. What caught it was
running the real loop against the real socket and noticing that two panes had
gone idle while its log stayed empty. **A hint channel is a thing that says
nothing almost all the time, so "it printed nothing" is indistinguishable
from "it works" — the only honest check is to make something settle and
watch.**

### What the wire actually permits

Three more measurements, each of which decides a line of code:

- **One `events.subscribe` per connection.** A second request on a live
  connection is not answered; the server closes it. The pane set therefore
  cannot be amended in place — changing it means reconnecting, which is why
  `pane.agent_detected` and `workspace.created` return `errRefreshPanes`
  rather than updating a map.
- **One request may mix unscoped and pane-scoped subscriptions**, answered
  by a single `subscription_started`. So the registry costs one connection,
  not one per pane.
- **A pane id herdr does not know is refused** (`pane_not_found`) and takes
  the whole connection with it. One stale id costs every subscription on it,
  so the pane list is re-read on every dial and a refusal is just another
  reason to redial — deliberately without an outage line, because nothing is
  wrong with herdr when a seat goes away.

Also: lifecycle envelopes come back underscored (`pane_updated`), pane-scoped
ones dotted (`pane.agent_status_changed`). Both spellings fold to one kind,
which is one `strings.ReplaceAll` — and a decoder written from the request
schema alone would match nothing and report nothing, forever.

### And the registry needs a level-triggered belt, because events are hints

The same live run that proved the settle arrives also proved the registry
does not maintain itself. A seat came up after the subscription was dialled
(`wVY:p1`), settled, and the loop logged nothing — while a raw socket
subscribed to the current pane list saw it. No `pane.agent_detected` and no
`workspace.created` reached the loop to say the set had moved, so it was
still watching the panes that existed when it dialled.

The event-driven refresh is kept, because it costs nothing when it does
fire. What makes the set eventually right is the belt: the watch loop pokes
the subscription after every pass, and the poke redials with the list as it
now stands. That is ADR 0016's own philosophy applied one level down — the
level-triggered pass is the truth, and the event stream only reduces
latency. A subscription that only ever learned about the world from the
event stream would have exactly the failure mode the ADR spends three
paragraphs refusing to build.

Measured end to end against the live socket, with a raw subscriber and the
watch process both stamped by the same clock: herdr's event at
`1787886300.472`, the line in the watch process's log at `1787886300.476`.
**Four milliseconds**, against a DONE WHEN of ~1s. (Taken on the loop before
the belt landed — the belt changes which panes are subscribed, not what
delivery costs once one is.)

### The seam, which is about the suite rather than herdr

Every other herdr read in this package goes through the fake CLI, one re-exec
per request; this one **dials**. A dial has no fake binary in front of it, so
the socket a Watch test resolves without being told otherwise is the
operator's live server — nine existing tests would have started reading the
live fleet the moment this landed. The fix is the seam the package already
uses for exactly this (`Spend`, `TurnOutcome`, `Load1`): a nil-able
`Dispatcher.Hints`, stubbed in `newTestDispatcher` with a nil channel that
never fires. The alternative — setting `HERDR_SOCKET_PATH` globally in
`newTestBackend` — reaches much further than it looks, because that value is
stamped into every session meta and moves the prune and refusal semantics
with it.

Slice S2 logs the hint and changes nothing else. The tick is untouched, on
purpose: a hint-only slice that also moved the schedule would leave the next
red suite with two suspects instead of one.
