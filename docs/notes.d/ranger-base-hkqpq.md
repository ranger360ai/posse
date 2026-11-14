## The pulse feed's first slice: where the delta line is appended, and why not where the bead says (ranger-base-hkqpq)

ADR 0093 D2 asks for two things and this bead landed both: `state/pulse.yaml`
persists `delivered_keys` in place of `prompted_fingerprint`, and a due tick
appends one delta line to `state/pulse.feed`. Three choices inside that are
not in the ADR and are worth having written down, because the next slice
(ranger-base-cgr1h, the waiter; and D3, the lock) touches all three.

### 1. The append site: after the prompt lands, not on every due tick

The bead says "on a due tick append one line". D3's end state says the same
thing more precisely — the line goes out on any due tick, and the lock then
decides whether anything is also typed. **In this slice the line is appended
after the typed prompt succeeds**, in `deliverPulse`, beside the record
advance.

Not a reading of "due" but a consequence of what `delivered_keys` is. The
append and the record advance are ONE act: the record says what was said, and
the feed is where it was said. Split them and both available orders are
broken, measured against ADR 0093's own 70h window (321 deliveries, 986 ticks
skipped because the coordinator was blocked, 16 because it was working):

- **advance on the append, append on any due tick.** The 986 blocked ticks
  consume their deltas into a feed nothing is reading yet — the waiter does
  not exist until ranger-base-cgr1h — and the typed pulse, which IS the
  channel in this slice, never carries them. Deltas lost, silently.
- **append on any due tick, advance only on a delivered prompt.** Every
  blocked tick re-appends the same delta. At a 2m `pulse_interval` the
  500-line cap is full in about 17 hours of a sleeping coordinator, and the
  oldest real entries are gone.

So the two move together, and in this slice the thing they move with is the
typed delivery. D3 moves them to the lock arm, which is where they belong
once a waiter can hold it: there the feed is the delivery, so advancing with
it is advancing with a delivery, and the invariant is the same one.

The existing pins that say a skipped or failed prompt leaves the record
unadvanced (`pulse_delivery_test.go`,
`pulserenagconfigured_qa_test.go`) therefore stand unchanged, which is the
other half of the bead's "typed delivery unchanged in this slice".

### 2. The partial hold is blanket, not attributed

D2: "A partial read is never a clear: a key from a store that could not be
read is held, not marked `-`." **Implemented as: on a partial tick, EVERY
delivered key that this tick did not see is held.**

Attribution is not available. `ShopCheck` returns `failed` as a list of
errors; nothing maps an error back to the keys that store would have
produced, and inventing that mapping would mean a key→row table maintained
by hand beside the G-table. A blanket hold is the only version of the rule
this code can enforce, and it fails in the direction the rule names: a
condition held one tick too long, never an unobserved condition announced as
cleared. Keys that APPEARED on a partial tick are still news — those were
seen — and so is a count that moved, because both of its halves were.

### 3. A partial EMPTY tick holds too, which reverses a pin

Before this bead, an empty computed set cleared the delivery record whether
or not the read was complete, and `TestQAEmptySetResetIsNotAnAllClear`
pinned exactly that. What made it safe was written on the test: "it forgets a
delivery, it does not assert an all-clear, and nothing downstream reads this
file for one."

Both halves of that are now false. The next tick's delta reads
`delivered_keys`, and the line this arm appends — `cleared all` — is an
all-clear in as many words, said to the coordinator. So the reset is gated on
`!tick.Partial`: a partial empty tick appends nothing and leaves the record
alone, and a complete one clears as it always did, saying `cleared all` once
if there was anything to clear.

The pin kept its name and gained the second arm, because the claim it was
always making ("the reset says nothing about the shop") is the claim that is
now enforced rather than argued.

### The line format, and the one place it diverges from the ADR's prose

```
1 2026-11-14T09:31:04Z +unpushed:/home/posse/src/posse:1 standing: 1
2 2026-11-14T09:33:04Z +blocked:dinesh-work standing: 2
3 2026-11-14T09:41:04Z ~unpushed:/home/posse/src/posse:1→2 standing: 2
4 2026-11-14T10:11:04Z still: blocked:dinesh-work; unpushed:/home/posse/src/posse:2
5 2026-11-14T10:15:04Z cleared all
```

`<seq> <RFC3339> <body>`; `+` then `-` then `~`, in D2's order, then
`standing: N` with no keys. A move renders as the key with its count
replaced, `~<stem>:<from>→<to>`, where D2's prose writes `unpushed 5→7`: the
difference buys a body whose every token is space-free, so `posse pulse wait`
splits the line on whitespace and needs no grammar to find the sequence
number. `still:` is the only body that names keys, because its whole content
is that nothing changed and that is not a message without a subject.

`PulseFeedSeq` is exported for the waiter, so the writer and the reader agree
on what the first field is by sharing the reading rather than by having two
of them.

### Two smaller decisions

- **The `~` pairing is generic**, not keyed on `unpushed:`: any key ending
  `:<digits>` pairs with a vanished key under the same stem. A rule that knew
  one prefix would render the next counted key somebody adds as a `+`/`-`
  pair — the noise D2 exists to remove — and would do it silently. The cost
  is that a trailing number that is not a count reads as one
  (`~guard-credential:1→2`), which is still true and still shorter than the
  pair.
- **The next sequence number is one past the highest READABLE entry**, not
  one past the last line. A corrupt tail would otherwise restart the
  sequence and hand an armed waiter numbers it has already passed, which
  looks exactly like a shop where nothing is happening — silent on both
  sides.

### What this slice does not do

The waiter is ranger-base-cgr1h (`posse pulse wait --after <seq>`, D1/D3) and
nothing reads the feed until it lands; D3's lock, the `pulse-unarmed`
condition and D4's composer hold are not here either. The typed pulse is
still the delivery, and `pulse_feed: off` — D2's exit hatch — is not wired,
because in this slice the feed costs one appended line per delivery and
turning it off would turn nothing else off.
