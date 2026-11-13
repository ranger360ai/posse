# ADR 0093 — The pulse reaches the coordinator through a waiter it arms, not through its keyboard

*Status: proposed 2026-11-13 (ranger-base-96ve6, operator question of that
day) · owner: architect · amends [ADR 0027](0027-monica-pulse.md) (delivery
channel, payload, the composer exception and persisted fields) · nothing here
changes live pulse config; arming stays the operator's*

## Context

The pulse types `Pulse check: <every condition>` and then Enter into the
coordinator's pane (`rex send`, then `rex send-key enter`; muxrex.go
`Prompt`). The operator types into that same pane from a remote client. A
pane's input is one byte stream with two writers and no lock, so when the
operator has typed half a sentence the pulse appends to it and submits both:
three spliced messages on 2026-11-13 alone (`I think thats accurPulse
check: …`). ADR 0027 had a composer veto and removed it, because herdr's
composer reading kept showing a line that had already been sent (~586
skips, a day of muted pulses; ranger-base-wr624). On rex there is no
reading at the pulse at all: `AwaitPromptable` takes its never-answered
concession, so `det.Composer()` is always empty and the report line never
prints.

MEASURED 2026-11-13 from the instance's watch log (2026-11-10 18:53 to
2026-11-13 17:08, about 70h, 8 watch generations), classifying each
delivered pulse against the one before it:

| delivered pulses | 321 (≈4.6/h) |
|---|---|
| a key appeared only / cleared only / both | 136 / 141 / 20 |
| only a count moved (`unpushed:<repo>:N`) | 9 |
| identical set, i.e. a renag | **14** |
| deliveries carrying `cage-realize:linux` | 321 of 321 |
| deliveries carrying the oldest standing question | 241 |
| ticks skipped because the coordinator was blocked / working | 986 / 16 |

So the coordinator's reading ("most pulses repeat an unchanged set") is
right about the payload and wrong about the cause. 96% of deliveries are
caused by a change, but each one re-sends the whole standing set, so every
turn re-reads the same seven or eight keys around the one that moved.
Renag accounts for 4%.

Also MEASURED the same day: `rex client inspect` on the operator's client
reports the sessions it is attached to (ten, since it connected that
morning), with no focus and no last-input time. Being attached says nothing
about being active.

## Decision

**D1. The primary channel is a waiter the coordinator runs, not keystrokes.**
New verb `posse pulse wait --after <seq>`. It blocks until the pulse feed
(D2) holds an entry with a sequence number greater than `<seq>`, prints every
such entry, ends with `next: posse pulse wait --after <last>`, and exits 0.
The coordinator runs it as a background command (`POSSE_KEEP=pulse`), and
its runtime re-invokes the session when the command exits. That is the
documented contract of the claude Bash tool's `run_in_background`, and the
coordinator's runtime is claude. Nothing is written to the pane's input, so
a splice is impossible by construction, not merely unlikely. Overnight it
wakes the coordinator just as a typed pulse did. Deltas that arrive during
the turn pile up in the feed and wake it once when it re-arms, which is
batching the coordinator gets for free. The cursor lives on the command
line, so the waiter holds no state. A late re-arm with an old `<seq>` gets
the backlog rather than losing it.

**D2. Deltas, not sets.** The watch stays the only component that computes
conditions (ADR 0029) and the only one that writes delivery state (ADR
0011). Where it used to prompt, it now appends one line to
`state/pulse.feed`: seq, time, `+` keys that appeared, `-` keys that cleared,
`~` keys whose count moved (`unpushed 5→7`), and `standing: N` with no keys.
The full set is still in `posse status`. To compute the delta, `state/pulse.yaml`
persists `delivered_keys` in place of `prompted_fingerprint`. That partly
reverses ADR 0027's "no persisted conditions", deliberately: this field is
what was *said*, not a mirror of status, and nothing reads it as health. An
empty computed set appends a `cleared all` line and clears `delivered_keys`.
A partial read is never a clear: a key from a store that could not be read
is held, not marked `-`. The renag keeps its one interval. When no delta
has gone out for `pulse_renag`, the line is `still: <keys>`, and that line,
unlike a delta, names the keys. The feed is capped by line count (the
newest 500); `--after` asking for a seq older than the oldest kept line
prints `gap` and then what is kept.

**D3. Waiter liveness is the switch, read by the kernel.** The waiter holds
`flock(state/pulse.wait.lock)` for its whole life, with its pid and start time written inside, and the lock goes when
the waiter or its session dies. At each due tick the watch tries the lock
without blocking. If it is held, the feed line is the delivery and nothing
is typed. If it is free, no one is listening: the watch appends the line
anyway and falls back to the typed pulse (D4), whose text gains `— arm the
feed: posse pulse wait --after <seq>`. A coordinator that forgets to re-arm
therefore gets reminded through the old channel rather than going dark. A
condition `pulse-unarmed` joins the set while the lock is free and a
coordinator session is live, so `posse status` shows it as well.

**D4. The typed fallback refuses to type over text the operator typed, on
rex only.** Before `Prompt`, read the composer with qzujz's capture reading:
`captureComposer`, then the ghost check, then the echo check against the
seat's last submitted line. That chain is the one ADR 0076 §4's settled-pane
hold already relies on, and it is the discriminator wr624 lacked. If what
remains is *Typed*, skip the tick, leave delivery state unadvanced, and log
`pulse: held — <seat> box holds operator text`. herdr stays report-only as
wr624 left it, because its reading is still the phantom-prone one. The
residual window is honest: the operator can type in the few ms between the
capture and `rex send`, or between `send` and `enter`. D1 is what closes
the window; D4 shrinks it on the path D3 should make rare.

## Consequences

- The splice class ends wherever the waiter is armed. On the fallback it
  shrinks to a capture-to-Enter window measured in milliseconds.
- Each delivery still costs the coordinator a turn. The waiter batches
  whatever arrives during a turn. The change in turn count is ASSUMED, not
  measured; the done-when row prices it.
- What breaks if this is wrong: a waiter that keeps the lock but cannot
  print (a wedged process) silences the typed path. The waiter therefore
  exits after `pulse_renag` even with no delta, printing `still:`, so a
  waiter older than that is wedged. The watch logs one, and `posse status`
  shows it as `pulse-unarmed`. A rex composer reading that sees phantom
  text holds only the fallback, and it says so in the log on every tick.
- Exit hatch: if `pulse_feed: off`, the watch uses ADR 0027's typed path
  unchanged. The feed is a text file a human can tail, and it holds no state
  that anything else depends on.

**Done when** (the verify lane's checklist): (1) in 24h of watch log, zero typed
pulses while the waiter lock was held; (2) zero spliced coordinator turns
(prompt text containing `Pulse check:` that does not start with it) over the
same window, read from the coordinator's transcript; (3) the feed's line
count compared with this page's 4.6/h, and the coordinator's wake count
compared with it, both stated; (4) separate acceptance for the following:
waiter killed, then `pulse-unarmed` plus a typed reminder; stale `--after`,
then `gap`; partial read, with no `-` line; rex box holding typed text,
then held and unadvanced; rex box holding ghost text, then delivered.

## Alternatives rejected

**(a) alone: a composer gate in front of the same keystrokes.** It is
cheap, the reading exists, and it is D4. On its own it cannot see an
operator who is mid-thought with an empty box, and it keeps the race
between check and type. Its herdr ancestor muted the pulse for a day. A
lock on the keyboard is only as good as the reading behind it, and the
channel can be moved off the keyboard instead.

**(b) alone: delta payload over keystrokes.** It shortens the text and
leaves the splice exactly where it is. It is kept as D2.

**(d) Do not pulse while the operator is attached.** MEASURED above:
attachment is "ten sessions, all day" and carries no focus and no input
time, so this would mean never pulsing while the remote app is open.

**A `UserPromptSubmit` hook that adds the delta to the operator's next
message.** It never fires overnight, which is when the wake matters, and
it is a write into the runtime's own settings (the class ADR 0090 and ADR
0091 exist to fence).

**The coordinator self-paces with a timer loop.** Every wake is a turn
whether or not anything changed: the polling version of D1, paying the cost
the coordinator complained about to learn what the watch already knows.

**A desktop notification to the operator.** That feeds the operator, not
the coordinator, and pushes shop state to a device the shop does not run.
Visibility guardrail 4: where the audience is unclear, it does not move.

**The clever one: a dedicated pulse pane, a second rex block the
coordinator watches.** This is the one I wanted, because it keeps typing
and gives the pulse its own input stream. But claude has one conversation
per process, so a second block is a second coordinator with a second
context. The coordinator would have to read the other block, which is the
waiter again with a terminal in the way.
