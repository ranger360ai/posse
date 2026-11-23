# ranger-base-q5fee — the other two ladders, and a §2 verb nothing called

The ruling is ADR 0076 Amendment 13. This file is the measurements behind it,
the one fact about the occupant that decided where the pins go, and the
mutation battery.

## The census

MEASURED 2026-11-23 (UTC; 2026-11-22 -05:00 local) over the live fleet's two
dispatch pass logs — `state/dispatch-watch.log` and `.log.1`, 66,766 lines,
the earliest `== dispatch --watch armed ==` line 2026-11-10 18:53:28 — read
while the live log was still being appended to, so every count is that
instant's. This is ranger-base-25otk's census re-read on a log 290 lines
longer; nothing of its finding moved.

| shape | n |
| --- | --- |
| `prompting without the readiness gate` — `AwaitPromptable`'s concession | 518 |
| of those, printed by the PULSE (`pulse: ` prefix) | 518 |
| `the work prompt is on its launch line; gathering anyway` — `awaitDelivered`'s | 0 |
| `prompting on its … anyway` — the settle ladder's, 25otk's subject | 862 |
| every §4 degrade line, all three sites | 1380 |

518 + 862 = 1380 exactly, which is the arithmetic that says the third site has
never run here.

Two readings decide the ruling rather than describe it:

- **518 of 518 are the pulse.** Not the operator's hand prompt and not the
  cockpit: every readiness-gate concession on this instance was a shop check.
  The pulse's own rule is `det.Seen() && !pulseTakesPrompt(det.State)` over
  the detection this gate hands back (pulse.go, ranger-base-k99a) — "a shop
  check is a nudge nobody asked for and must not land on a persona mid-turn"
  — and the concession hands back the ZERO detection, so that rule has never
  once been able to fire on this box.

  What kept it from mattering is the cheap pre-filter one line above it. The
  pulse reads the session listing first, and on Rex the listing is
  evidence-backed: `RexMux.List` drops a block whose reading has no `Seen`,
  and the state it prints is the record's. So a visibly-working persona was
  skipped there. What was missing is the RE-READ at the instant of typing,
  which is the whole reason this gate is a second reader at all —
  `awaitSettled`'s own comment: "`agent wait` and `agent explain` read the
  same detection at two different instants, and the second one wins… a fresh
  claude drew its trust dialog in the 30ms between the two calls."
- **0 at the delivered wait.** No `prompt: argv` runtime has launched on this
  instance (`work prompt on the launch line`, the print one frame up: 0), so
  that half is a shape closed before it is paid. It is counted and named as
  zero rather than left out, because the two sites read as one bead and a
  reader deserves to know which of them the box has exercised.

## The verb nothing called

MEASURED 2026-11-23 on the HEAD that landed Amendment 12 (commit 4e03731d),
by grep over every non-test `.go` file under `internal/` and `cmd/`: the only
callers of `Mux().State` are `RexMux.Wait`'s poll loop and
`RexMux.AwaitFirstReport`, both inside `internal/posse/muxrex.go`. No ladder
read it. It is one of ADR 0076 §2's ten verbs, so every occupant answers it —
which is the whole of this bead: the reading was in hand on every poll of
every one of those 518 concessions and nothing asked for it.

## One fact about the occupant that decided where the pins go

MEASURED by reading this HEAD, and it is why the two sites' pins are not
symmetric: **on Rex, `awaitDelivered`'s concession is unreachable from either
side.** With evidence it takes the new observation arm. WITHOUT evidence there
is no seat to observe — `RexMux.List` drops a block whose reading has no
`Seen` ("the Rex form of the unknown MEANS NONE rule"), so `AgentTarget`
resolves nothing and `awaitTarget` fails with `no agent detected` before the
ladder below it runs. Observed as a red pin while splitting arm 5: the
concession arm, driven with a shell-written record on the real occupant, never
printed its line — the test died one frame earlier.

The readiness gate is not like that, because it is HANDED its target: a
record the block's own shell wrote (§3's second guard) is a reading Rex
answers, with `unknown` and no evidence, and the gate concedes on it exactly
as before. So arm 3b runs on the real occupant and arm 5b needs the stand-in
that answers §2's verbs with a state and nothing else — the same occupant
Amendment 12's arm 4b introduced, now with `State`, `List` and `Server` on it
(`noEvidenceMux`).

## What landed

- `muxSeenGate` (internal/posse/muxsettle.go): the evidence half on its own,
  and now also the half `muxSettledGate` reads, so the two gates cannot
  disagree about what evidence is. `MuxSeen` is still set in one file.
- `(*HerdrBackend).muxSeen` (same file): the rule asked of the occupant,
  through the one door it may be asked through — a §4 answer of "not
  available" (`muxVerbMissing`), never a verb that failed. That door is the
  blast radius: `HerdrMux.State` IS `agent explain` flattened, so on herdr the
  read would be a retry of the call that just failed, one more subprocess per
  poll, and the fake herdr's two call-counting error levers off by one.
- `AwaitPromptable` (promptready.go): the arm, plus the `opened` closure so
  the note rule has one copy, plus the occupant's arm in `waitedFor`/`seenBy`
  — whose existing last branch would otherwise have rendered `reported for
  agent "" (herdr detects no such kind)` over a reading that is neither.
- `awaitDelivered` (dispatch.go): the arm, returning silently like its
  `det.Seen()` arm.
- Pins: `muxsettle_qa_test.go` arms 7-9 (the predicate's table beside arm 1's,
  the door as a call that is never made, and herdr's own failures never
  opening it), and `muxnoverb_qa_test.go`'s arms 3 and 5 split into accept and
  concession the way Amendment 12 split arm 4.

## Did the pins catch the fix being taken out

Five mutations, planted one at a time against the filtered pins (the battery
is a scratchpad script, not tracked). MEASURED 2026-11-23 on this box:

| mutation | verdict |
| --- | --- |
| the readiness gate's arm is removed (half the bug as filed) | red — muxnoverb 3a: the gate hands back the zero detection, which is the reading the pulse's rule cannot fire on |
| the delivered wait's arm is removed (the other half) | red — muxnoverb 5a: the observation prints the degrade line instead of answering |
| the door reads "the explain errored" instead of §4's "not available" | red — muxsettle arm 8, on both legs: the occupant answers over a herdr failure, and the call census counts the 4 calls that answer cost |
| the readiness predicate adds the settle gate's state half | red — muxsettle arm 7's table, and muxnoverb 3a over `working`: the gate a hand prompt needs stops opening mid-turn |
| `muxSettledGate` loses its state half to the shared helper | red — muxsettle arm 1 and arm 7's cross-check, each from the other gate's table |

The last two rows are the pair that matters: the two gates share one evidence
half and differ in exactly one clause, and each one's table reds when the
other's rule is copied onto it.

**The third row is the one that needed a census and not an assertion.** Under
that mutation arm 9 stays GREEN — on herdr the extra read is `agent explain`
again, the fake's error lever is armed on every call, so the answer is still
false and no behaviour moves. What moves is the COST: one more subprocess per
poll, and two call-counting error levers off by one. The only thing that can
see that is the occupant's own call log, which is why arm 8 reads it.

## Not closed here

- A no-evidence reading still opens nothing and refuses nothing at either
  site: the concession stands and the keystroke is refused at the delivery
  verb instead (`refuseAtDeadOwner`, `refuseWithoutComposer`). Whether the
  readiness gate should refuse a §4-missing seat whose runtime has ended —
  earlier, in its own words — is left open in Amendment 13's last
  consequence, on purpose: new behaviour on the operator's own path, a
  sentence that would have to be written for an occupant that is not herdr,
  and no reading on this box shown to need it.
- **The grace is unmoved and that is the same open question wearing its other
  face.** promptready.go's own rule is "a guess is a real answer and holds the
  full wait; only the never-answered case is cut short" — and on a §4-missing
  occupant an error-free `unknown` IS a real answer from a working verb, so
  that rule read literally would hold the full `startup_wait:` rather than
  concede at `promptExplainGrace`. It does not, deliberately: `answered` still
  means "herdr explained", the concession fires at one second as it always
  did, and the only thing this bead changed is that a reading WITH evidence
  now opens the gate before the grace is spent. Holding the full wait would
  only be worth doing in order to refuse at the end of it, which is the
  bullet above.
- The pulse's end-to-end Rex case is pinned as two halves that compose —
  `TestPulseSkipsAWorkingScreenTheListingCalledIdle` for "the pulse applies
  its rule to THIS gate's detection", muxnoverb arm 3a for "this detection now
  carries the occupant's reading" — and not as one test. Driving `pulseOnce`
  against a Rex occupant needs the listing and the gate to read different
  entries of one status queue, and a pin whose meaning depends on how many
  times something polls is a pin that reds on an unrelated bead.
