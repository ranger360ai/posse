## The pulse's channel switch: what D3 and D4 landed, and the three places the code reads the page narrowly (ranger-base-r3qz6)

ADR 0093 D3 and D4, on top of D2's feed (ranger-base-hkqpq) and D1's waiter
(ranger-base-cgr1h). From 2026-11-14 the pulse delivers on the feed whenever a
waiter is listening and types into a pane only when one is not — so the splice
class ADR 0093 was filed for (three spliced coordinator turns on 2026-11-13
alone, two writers on one byte stream with no lock) is closed wherever the
coordinator arms `posse pulse wait`.

What landed, in one paragraph each:

- **The switch** (`deliverPulse`, pulse.go). A due tick probes
  `flock(state/pulse.wait.lock)` without blocking (`ReadPulseListener`,
  pulsewait.go). Listening → the feed line IS the delivery, the record
  advances with it, and nothing reaches a pane. Free, or held past the
  waiter's own maximum life → the typed fallback, whose text gains
  `— arm the feed: posse pulse wait --after <seq>`.
- **The wedge.** A waiter that holds the lock and cannot print would silence
  the typed path, and from outside it is indistinguishable from a healthy
  waiter on a quiet shop. The waiter has a maximum life of one `pulse_renag`
  (pulseWaitLife), so a holder older than `pulse_renag + one tick` is wedged
  and the watch types anyway. The extra tick is not slack for its own sake: a
  healthy waiter armed an instant before a tick is exactly `renag` old at the
  tick that judges it.
- **`pulse-unarmed`** (govern.go), a carry-over and not a G-row, LANE. It
  fires while the pulse is armed, the feed is on, a coordinator session is
  live and nobody is listening; its detail names the command that clears it.
- **D4's composer hold**, on rex only: before the keystrokes, read this
  seat's screen once and run ranger-base-qzujz's chain — the composer, the
  echo check against the runtime's own submit log, then the dim check. What
  survives both is text somebody typed, and the tick is skipped with the
  record unadvanced (`pulse: held — <seat> box holds operator text`).
- **`pulse_feed: off`**, D2's exit hatch, wired for the first time: no feed,
  no probe, no reminder, no condition.

### 1. "Appends the line anyway" is implemented as "appends with the delivery"

D3's free-lock arm says "the watch appends the line anyway and falls back to
the typed pulse". Read as *append on any due tick, whatever happens to the
typed delivery*, that is one of the two orders ranger-base-hkqpq measured and
rejected, and a waiter in the world does not weaken the rejection — on this
arm, by construction, no waiter is holding the lock, so nothing is reading the
feed:

- **append and advance on a due tick**: the ticks where the typed pulse
  cannot land — 986 of 1323 in ADR 0093's own 70h window, a coordinator
  blocked or working — consume their delta into a feed nobody is holding.
  Said to nobody, by a record that says it was said.
- **append on a due tick, advance only on delivery**: every such tick
  re-appends the same delta. At a 2m interval the 500-line cap is full in
  about 17 hours of a sleeping coordinator.

So hkqpq's invariant is MOVED rather than retired: the append and the record
advance are one act, with the delivery. Listening → both happen in
`deliverPulseOnFeed`; not listening → both happen after the prompt lands. A
tick that said nothing writes nothing, and the delta it could not say is still
owed, because the record it is computed against has not moved.

What that costs, exactly: a coordinator that re-arms after a stretch of ticks
it could not be typed to gets the NET delta at the next due tick, not one line
per intermediate step, so a key that appeared and cleared inside that stretch
is never named. D1's "a late re-arm with an old `<seq>` gets the backlog"
still holds for everything that was said.

**It also keeps `pulse-unarmed` from flapping through the feed**, which was
not the reason for it but is worth writing down. The window where the row is
true and the coordinator is reachable is narrow: a waiter exits to deliver,
the coordinator spends a turn, and it re-arms at the end of it — and
throughout that turn the session is WORKING, which the typed path refuses. So
the row exists for exactly as long as the typed path will not act on it, the
record never takes it, and the feed never carries a `+pulse-unarmed` /
`-pulse-unarmed` pair for a coordinator that is simply mid-turn. It reaches
the feed only when the coordinator is idle AND unarmed, which is the state the
row is for.

### 2. D4 survives the exit hatch

`pulse_feed: off` turns off the feed, the probe, the reminder and the
condition. It does NOT turn off D4's composer hold, and ADR 0093's word for
the hatch is "unchanged", so this is a deliberate reading rather than an
oversight: the hatch exists so that a feed or a waiter that misbehaves cannot
silence the shop check, and D4 is a guard on the KEYBOARD, not part of the
feed. An operator reaching for the hatch is an operator whose pulse is typing
into a pane, which is when the hold matters most — switching it off with the
feed would make the exit hatch a splice regression.
`TestPulseHoldsOverTheRexBoxWithTheFeedHatchSet` is the arm.

### 3. One condition key for two causes

G7's rule in this file is "same fact, different cause → different KEY, because
the fingerprint moves on it" (`arm-broken`, `loop-dead`, `log-mute`).
`pulse-unarmed` deliberately does not follow it: a free lock and a wedged
holder are one fact — nothing is listening on the feed — the remedy is the
same command either way, and ADR 0093 D3 names this key for both ("a waiter
older than that is wedged. The watch logs one, and `posse status` shows it as
`pulse-unarmed`"). The cause is in the detail, where a human reads it.

### What the probe's error means, and the two callers that disagree

`PulseWaiterArmed` answers "could not ask", never "no waiter", and the two
readers take it in opposite directions on purpose:

- **the watch concedes** (`PulseListener.Listening` is true on an error):
  reading ignorance as "nobody is listening" is what types a pulse into the
  pane of a session that is. The cost of conceding is a delta in a feed nobody
  reads and one wait for the next; the cost of the other direction is the
  measured splice. The tick logs the probe it could not take.
- **`posse status` reports PARTIAL**: whether anyone is listening is exactly
  what could not be read, and this surface's standing discipline is that
  unknown is never an all-clear.

The lock file's BODY is the one place D3 needed something the waiter's own
discipline says decides nothing. Armed stays the kernel's answer; the AGE
comes from `started:`, because nothing else records it. A holder whose body
cannot be read is therefore **not** judged wedged — unknown is not no, in the
direction that keeps the keyboard quiet. The cost is named in the type's own
doc: a wedged waiter whose stamp never landed silences the typed path for as
long as it sits there. `TestPulseConcedesToAHolderItCannotAge` is the arm.

### `pulse_feed:`, and why a typo leaves the channel ON

`off`, `never`, `0` and `false` read as off — `graceAfter`'s vocabulary plus
`model_preflight:`'s boolean spelling. Anything else, including a typo, reads
as ON. Both directions cost something and this is the cheaper one: a mistyped
hatch leaves the new channel running, where "anything but `on` is off" would
let one character take the feed, the waiter and the condition out silently. It
is not an error either, because `LoadPulseConfig`'s error disarms the whole
pulse for the loop, which is far too much for a hatch nobody has to set.

`PulseConfig.Feed` has a zero value of FALSE, which is the opposite of the
shipped default, and that is a trade made with the tests in view:
`LoadPulseConfig` is the only producer of a real `PulseConfig` (a hand-built
one is a test fixture), and the other way round every one of the 37 fixtures
in this package that build the struct by hand would silently acquire the new
channel — and with it `pulse-unarmed` in its condition set.
`TestLoadPulseConfigReadsTheFeedKey` is what pins the real default.

### The fixture that had to change next door

ranger-base-hkqpq's four end-to-end feed arms (pulsefeed_test.go) now arm a
listening waiter (`feedListener`) and so run over the channel the feed is
delivered by. That keeps every expected body byte-identical: on a free lock
the set gains `pulse-unarmed`, which would have put a second standing key in
every line and a `standing: N` one higher, none of it about the delta those
arms are written for. The cost is that the four take an flock, so they lost
`t.Parallel` and are named in `cmd/testparallel`'s serial map beside the rest
of the family (ranger-base-9l77f: two acquisition tests side by side read a
released lock as still held).

### Pins

| claim | pin |
|---|---|
| a listening waiter is delivered on the feed, and nothing is typed | `TestPulseFeedIsTheDeliveryWhileAWaiterHoldsTheLock` (arm 3) |
| a feed delivery cannot be refused — not even by a working coordinator | `TestPulseFeedDeliversToAWorkingCoordinator` |
| a free lock types, and the text carries the arming command | `TestPulseTypesAndRemindsWhenNoWaiterIsArmed` |
| a holder past `renag + one tick` is wedged, so the typed path runs | `TestPulseTypesWhenTheWaiterIsWedged` |
| a holder at exactly `renag` is still listening | `TestPulseWaiterAtExactlyTheRenagIsStillListening` |
| a holder posse cannot age is not a wedge | `TestPulseConcedesToAHolderItCannotAge` |
| a failed append on the feed arm advances nothing and types nothing | `TestPulseFeedArmDoesNotAdvanceTheRecordWhenTheAppendFails` |
| `pulse_feed: off` is ADR 0027's path, with no feed written at all | `TestPulseFeedOffKeepsTheTypedPathUnchanged` |
| the hatch's spellings, and that a typo leaves the channel on | `TestPulseFeedConfigSpellings`, `TestLoadPulseConfigReadsTheFeedKey` |
| `pulse-unarmed`: every gate, both causes, and the unaskable probe | `TestQAPulseUnarmedIsTheRowWhileNobodyIsListening` |
| a rex box holding operator text holds the tick, unadvanced | `TestPulseHoldsWhenTheRexBoxHoldsOperatorText` (arm 1) |
| the hold survives `pulse_feed: off` | `TestPulseHoldsOverTheRexBoxWithTheFeedHatchSet` |
| a box echoing its own last submit is delivered to | `TestPulseDeliversOverARexBoxEchoingItsLastSubmit` |
| a box drawing claude's own suggestion is delivered to | `TestPulseDeliversOverARexGhostBox` |
| the door is rex-only, composer-only, and open over an unreadable screen | `TestQARexComposerHoldIsRexOnly` |
| a probe nobody can take is conceded to the feed, not typed over | `TestPulseConcedesAnUnaskableProbeToTheFeed` |

Measured the other way on 2026-11-14 in this worktree, one mutation at a time,
each restored from a byte copy and verified by sha256 before the next —
**eight mutants, eight red**:

| mutant | pin it reds |
|---|---|
| `Listening()` drops the wedge term | `TestPulseTypesWhenTheWaiterIsWedged`, and the row's wedged arm |
| `Listening()` reads a probe error as "not listening" | `TestPulseConcedesAnUnaskableProbeToTheFeed` |
| the wedge bound loses its one-tick slack (`renag - interval`) | `TestPulseWedgeBound`, `TestPulseWaiterAtExactlyTheRenagIsStillListening` |
| an unreadable holder body is judged wedged | `TestPulseConcedesToAHolderItCannotAge` |
| the feed arm advances the record before the append succeeds | `TestPulseFeedArmDoesNotAdvanceTheRecordWhenTheAppendFails` |
| the D4 hold reports and types anyway | `TestPulseHoldsWhenTheRexBoxHoldsOperatorText`, `TestPulseHoldsOverTheRexBoxWithTheFeedHatchSet` |
| the door drops its rex-only gate | `TestQARexComposerHoldIsRexOnly` |
| the typed arm appends the feed line under the hatch | `TestPulseFeedOffKeepsTheTypedPathUnchanged` |

The second row is why that pin exists at all: the first run of this battery
had **seven** of eight red, and the mutant that survived was the probe-error
concession. The arm written for it was `posse status`'s, which reads
`PulseListener.Err` itself before it ever asks `Listening()` — so the one
reader the concession is FOR, the watch's own tick, had no pin on it, and a
four-character edit would have put the keyboard back in front of a live
waiter. The battery found the hole in the tests, not in the code.

### What this slice does not do

ADR 0093's done-when rows 1-3 are the verify lane's and need 24h of watch log
and the coordinator's own transcript: zero typed pulses while the lock was
held, zero spliced turns, and the feed's line count and the coordinator's wake
count against this page's 4.6/h. Nothing here can produce them. Row 4's four
acceptances are the pins above, at the unit.

The coordinator's PID does not arm the waiter yet — nothing in this slice
writes a PID, and arming stays the operator's (ADR 0093's status line). Until
it is armed, every instance reads `pulse-unarmed` and keeps the typed channel,
which is the designed fallback and not a regression.
