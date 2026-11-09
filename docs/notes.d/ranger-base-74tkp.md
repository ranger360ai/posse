# The wait leg that did not wait, and why the fix is not in the occupant (ranger-base-74tkp)

2026-11-07. Filed out of ranger-base-8gh58's live acceptance; fixed here.

## The reading

Two dispatch passes, minutes apart, same repo, same runtime (codex),
differing only in the instance's `multiplexer:` key. The line counted is
`still working after Ns — waiting again`, which gather prints once per wait
LEG (dispatch.go).

| instance | progress lines | over | rate |
|---|---|---|---|
| `multiplexer: rex` | 295, over 210 distinct one-second values | ~3m30s | ~1.4 legs/second |
| `multiplexer: herdr` | 0 | a LONGER wait on the same bead class | — |

On the arithmetic that reading gives: a 40-minute bead is about 3,400 lines
and the 4h `WaitCeiling` about 20,000, each iteration also re-reading the Rex
server's `program_status` (MEASURED 2026-10-31 at 50-70 ms a call, two calls
per reading). Under `dispatch --watch`, which writes `state/dispatch-watch.log`
itself and rotates at 5 MiB, that is the fleet's retrospective record drowned
by one seat.

Nothing was misjudged — the claim was kept, the settle was read correctly when
it came, and the acceptance settled through this path. It is cost and
legibility, which is why the bead was P2.

## What it actually was

Not Rex's. `RexMux.Wait` polls at `rexPollInterval` (one second) and sleeps
between samples; it never returned faster than it was asked to. What ran away
is the LEG rate above it: `gather`'s loop re-waited on whatever the last leg
returned without ever asking how long that leg took, and printed a line every
time it went round.

That is invisible on herdr because a herdr leg blocks for `PromptWaitMS` —
fifteen minutes — inside the server's own wait verb, so the loop goes round
once per settle or once per timeout. It is unavoidable on Rex because there is
no blocking wait verb to delegate to: a leg there ends when its own deadline
does, and `judgeStall`/`rewait` sent it round again at once.

**MEASURED 2026-11-07, on HERDR, at 24 legs/second**, in this worktree's
suite: a bounded pass with the fake server's `wait-error` lever answering
every `agent wait` with the `timeout` code a real one answers a blown deadline
with printed 68 progress lines in 2.809s. Which is not a contrived shape —
`TestDispatchRewaitServerGoneKeepsTheClaim` measures a real herdr answering
exactly that way, three fresh processes against a socket whose server was
replaced under an in-flight prompt. So the defect is a property of the leg
loop, on either occupant, and the fix belongs there.

The reason the loop TOLERATED a leg that failed, rather than handing the bead
back, is `pendingBead.delivered`: codex is `prompt: argv`, the prompt rode the
launch line, and on that path no wait failure is a prompt failure at all (ADR
0013 §2). On the typed path only the `timeout` code reaches the same arm.

> **Superseded 2026-11-09 (ranger-base-2mqp3), for the typed path only.** The
> last sentence above was the defect that bead was filed for: a re-wait leg's
> error — a signalled child, a replaced server, anything that is not herdr's
> `timeout` envelope — was spent as a failed prompt and unclaimed a bead whose
> seat was mid-turn. `pendingBead.rewaited` now says for the typed path what
> `delivered` says for the launch line, so EVERY leg error past the first
> reaches the status probe on either path. Nothing above it changes: the
> pacing this note is about is what makes the kept claim affordable, since a
> loop that tolerates a failing leg is a loop that goes round.

## The fix

Two numbers, both on `Dispatcher`, both defaulted by `NewDispatcher`:

- `LegFloor` (`DefaultLegFloor` = **15s**) — a floor under the interval
  between the STARTS of two legs on one bead. Applied in `rewait`, which
  stamps `pendingBead.legAt` forward and sleeps the remainder inside the leg's
  own goroutine, so the gather goroutine goes straight back to its select and
  a stop is still honoured within the stage (ranger-base-e9d9's drain).
- `ProgressEvery` (`DefaultProgressEvery` = **1m**) — how often the progress
  line is printed while one bead waits (`sayWorking`). The first leg to come
  back always says so.

Both are CEILINGS on a rate and never floors under a wall, which is the whole
of "the herdr path keeps blocking as it does": a herdr leg is `PromptWaitMS`
of its own, three orders of magnitude over the floor, so the remainder is
negative every time and nothing sleeps; and its legs are fifteen minutes
apart, so every one of them still prints. A herdr pass is byte-identical.

Why 15s: one reading is two rex calls at 50-70 ms, so fifteen seconds spends
under 1% of the wall and well under 1% of one core per waiting seat — four
readings a minute against the eighty-four the busy-wait was taking. What it
buys back, besides the log, is the only measurement the interval touches: ADR
0028 §5 observable 1 reads a seat's idle window off the DISCOVERY of a settle,
late by up to one poll, and fifteen seconds is a bound that bias never had —
the leg it replaces is `PromptWaitMS`, so on herdr that bias is up to fifteen
MINUTES today and this can only shorten it.

## The pins, and the mutants

`internal/posse/waitlegpace_qa_test.go` (arm 1). Each pin drives `gather`
directly on BOTH occupants in one test — the bead's own ask — with a seat that
reads working throughout and a leg that ends at once. On Rex nothing is
injected: `PromptWaitMS` is 1 ms, so the poller is already past its own
deadline when its first sample returns. On herdr the `wait-error` lever
answers with the same code, because the fake answers out of a file and watches
no clock.

The two rates are pinned with the other knob DISABLED, so neither can ride on
the other: the poll pin sets `ProgressEvery` 0 (every leg says so, and the
line count IS the leg count) and the line pin sets `LegFloor` 0. Both assert a
CEILING on a count in a 2s window, which is the direction a loaded box moves
AWAY from. A third pin holds the two production numbers and their relation.

MEASURED, each mutant reddening its own pin on BOTH occupants:

| mutant | herdr | rex | allowed |
|---|---|---|---|
| `rewait` takes no floor | 35 legs / 2s | 14 legs / 2s | 7 |
| `sayWorking` always true | 36 lines / 2s | 12 lines / 2s | 7 |

With the fix: 4 and 4.

## Not done

The precise rex call that ends a live Rex leg early is NOT identified here,
and did not need to be: `RexMux.Wait` returns early on exactly two things, its
own blown deadline and an error from `State`, and the floor paces the loop
over either. Reproducing the live reading would cost a billed codex turn on a
scratch Rex instance (ranger-base-x7ojx's authorization was for two turns,
both spent).

### The discriminator this section promised, corrected (ranger-base-aj79z)

What stood here said the discriminator was ALREADY in the pass output — "a
leg that ran out names its own deadline (`no idle|done|blocked state in
<block> within <d>`), and a leg that could not read names the rex call".

**MEASURED FALSE 2026-11-07**, darwin 25.6.0, over this bead's own fixture
(`waitLegGather`): neither half could print. On the `delivered` path a non-nil
leg error had no sink at all — both guards in `gather` that consume one carry
`&& !p.delivered`, and codex is `prompt: argv`, so every leg error was dropped
between `if r.err == nil` and the status switch. The loop only goes round
again when the status probe answers `working`, so the deadline half never
reached the output; the "names the rex call" half reached it only through the
switch's `default` arm, which fires when the probe ALSO fails and returns
after one leg. A pass that printed a rex call had run one leg, not 295.

Fixed under ranger-base-aj79z: `gather` names the leg's own error once per
`ProgressEvery`, on its own clock, before the status probe. What the pass
output NOW carries, MEASURED 2026-11-07 over the same fixture, one line per
failing leg with `ProgressEvery` 0:

```
◷ a-1   wait leg ended: herdr: timed out waiting for agent status (timeout)
◷ a-1   still working after 0s — waiting again
```

on herdr, and on rex:

```
◷ a-1   wait leg ended: herdr: no idle|done|blocked state in block:fake-0001 within 1ms (last reading: working) (timeout)
◷ a-1   still working after 0s — waiting again
```

So the discriminator is real now and reads as this section meant it to: the
deadline half names the `until` set, the pane and the deadline, and a leg that
could not read is a raw error naming the rex call instead. The `herdr:` prefix
on the rex line is `HerdrAPIError.Error()`, which `RexMux.Wait` reuses on
purpose (muxrex.go: it carries that route's codes verbatim rather than coining
its own) — it names the ENVELOPE and not the occupant, and the pane in the
message is what says which occupant spoke.
