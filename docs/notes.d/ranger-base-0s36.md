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
require a second mutable registry." That is still true. What the ADR did not
have is the third option, which the same probe finds two lines later:
`pane.updated` **is** accepted unscoped, and it carries the whole `PaneInfo`
— `agent_status` and `workspace_id` included — on every revision bump.

So posse subscribes to the level and derives the edge. `pane.updated` is a
level-triggered stream: it repeats the current status every time anything
about the pane changes, which on a working seat is several times a second.
A settle is the **transition into** idle or done, and `settleGate` is one map
of last-seen levels per pane. The first sighting of a pane is deliberately
not a hint — it is a level, not a transition, and a subscriber that opened
while eleven seats sat idle would otherwise announce eleven settles that
never happened.

This is the whole substitution: same socket, same status field, no registry,
one map. It is not a second correctness model, because a hint was never
allowed to be one. A duplicate costs an early level-triggered read, a lost
edge costs latency until the tick, and the tick is still armed — which is the
property that lets a level→edge conversion be this cheap. Convert the other
way and it would not be: an edge stream you cannot re-read has to be
remembered, checkpointed, and replayed, and that is the state machine ADR
0016 declined to build.

Two smaller measurements, both load-bearing and neither guessable:

- Subscriptions are spelled dotted (`pane.updated`); the envelopes pushed
  back are spelled with underscores (`pane_updated`). §1 anticipated this
  and it is one `strings.ReplaceAll`, but a decoder written from the request
  schema alone would match nothing and report nothing, forever.
- A Unix socket path over ~104 bytes fails to **dial** with `EINVAL`, not
  `ENOENT`. Test fixtures under a long temp path hit this before they hit
  anything they were written to test (`shortTempDir` exists for it).

The last note is about the suite rather than herdr. Every other herdr read in
this package goes through the fake CLI, one re-exec per request; this one
**dials**. A dial has no fake binary in front of it, so the socket a Watch
test resolves without being told otherwise is the operator's live server —
nine existing tests would have started reading the live fleet the moment this
landed. The fix is the seam the package already uses for exactly this
(`Spend`, `TurnOutcome`, `Load1`): a nil-able `Dispatcher.Hints`, stubbed in
`newTestDispatcher` with a nil channel that never fires. The alternative —
setting `HERDR_SOCKET_PATH` globally in `newTestBackend` — reaches much
further than it looks, because that value is stamped into every session meta
and moves the prune and refusal semantics with it.

Slice S2 logs the hint and changes nothing else. The tick is untouched, on
purpose: a hint-only slice that also moved the schedule would leave the next
red suite with two suspects instead of one.
