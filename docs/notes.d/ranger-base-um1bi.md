# The two report-only composer readings, on an occupant with no `agent explain` (ranger-base-um1bi)

2026-11-10. From ranger-base-qzujz, which carried `PaneHold`'s composer half
onto a `multiplexer: rex` instance and correctly left these two outside its
scope: neither decides anything.

## What was off, and why it was invisible

Three readers look at claude's `prompt_box_body`. One of them decides —
`PaneHolding`, whose hold parks a `--resume` re-prompt, drops a settle-open
and files the pulse's G2 `-unsent` row. The other two only ever print a line:

1. `internal/posse/panework.go` `ConfirmSubmitted` — the read-back after a
   prompt is submitted, which reports the text still sitting in the box.
2. `cmd/posse/main.go`'s `posse prompt` — "warning: `<name>` already has
   `<hold>` — this text is typed after it", before anything is typed.

All three read the region through `HerdrBackend.explain`, which is ADR 0076
§4. On this box that answers "not available" on every call, for the life of
the instance. `PaneHolding` grew a second route for it (ranger-base-r5131 for
the work half, ranger-base-qzujz for the composer half); these two did not.

So on a Rex instance `ConfirmSubmitted`'s first poll took the §4 refusal,
returned `""`, and the command said nothing — and the pre-type warning read
the ZERO detection the readiness gate hands back when it concedes, so
`Composer()` was `""` for every seat. Neither is "the box is empty". Both are
"the question was never asked", which is the distinction panework.go's head
comment is written around and the one a report cannot make for itself.

## The question the bead asked first: a reading, or the degrade line?

`explainDegrade` (muxherdr.go, ranger-base-l32dl) is the other honest answer
to a §4 refusal, and for a warning nobody acts on it is a real candidate. It
loses on its own terms, and differently for each of the two:

- **The pre-type warning already has one.** `AwaitPromptable` degrades on
  this box and prints `this instance's multiplexer has no agent explain (ADR
  0076 §4) — prompting without the readiness gate` one line above. A second
  line for the same missing verb on the same command is noise.
- **`ConfirmSubmitted` would print one forever.** That note's own words for
  the §4 cause are "unavailable on every launch by design — so it printed
  once per seat, on every pass, forever". A line for a reading that IS
  takeable, one capture away, is the shape it warns against.

So: take the reading in both, print nothing extra, and report exactly what the
herdr arm reports. The pre-type warning asks `PaneHolding`, which is the one
reading of that region and routes per occupant; `ConfirmSubmitted` keeps its
herdr loop byte for byte and grows the §4 arm beside it.

## The second question: does Rex's Prompt leave text in the box at all?

NOT MEASURED on this occupant, and that is why the reading was built rather
than the warning retired. What is written down is the MECHANISM, on this exact
delivery shape:

- `herdr agent prompt` was MEASURED leaving text in the box three times on
  2026-09-02 — `agent_prompted` returned, the text typed, the submit never
  happening, the last one sitting four hours while every store reported the
  agent idle. No `/` was involved in any of them. That is a type-then-Enter
  race, not a herdr defect.
- `RexMux.Prompt` is that same shape by construction: `rex send`, then
  `send-key enter`, two calls (ADR 0076 §2).
- ADR 0060 D5 MEASURED the picker variant on a `pane send-text` + Enter
  route: "the picker eats Enter and the composer keeps the text through Esc
  and C-u", while the call reports success. `RexMux.Prompt` refuses a leading
  `/` outright, so that one cause cannot arrive through `posse prompt`.
- `RexMux.Prompt`'s own note ends "Rex has even less to catch it with than
  herdr — no envelope, and no detection to read the box back with". That
  sentence names this absence, and on this occupant the window is WIDER than
  on herdr, because the readiness gate that guards it is itself §4 and
  concedes here.

The reading is report-only either way, which makes the choice asymmetric: if
the failure mode turns out not to exist on Rex it costs one capture and never
prints a word, and if it does, the alternative is a prompt sitting for hours
with nothing saying so. Measuring it needs a live seat and a deliberately
missing Enter, which is its own venue — filed, not invented here.

## What `PaneHolding` brought the pre-type warning for free

The call site read `det.Composer()` and printed `det.Hold().Why()`, and
`AgentDetection.Hold()` is `{Work, Typed}` — no `Sent`, no `Ghost`. So on
herdr, today, that line fires over two boxes nobody is holding text in:

- a box previewing the line the pane last SUBMITTED, which claude's own submit
  log identifies (ranger-base-2hvtv), and
- a box claude DREW FAINT, which is its own suggested next prompt
  (ranger-base-6o7wm).

Both retirements live inside `PaneHolding` and neither reached this call site.
A box carrying a suggestion is the ordinary state of a settled seat, so the
warning was usually wrong — and a warning that is usually wrong is one the
operator learns to skip past, which costs exactly the case it exists for.

## What it costs

One more `agent explain` on a hand command, MEASURED at 0.00s against herdr
0.9.1 (promptready.go's own note), plus the two retirement reads only when
there is text to retire.

On the capture route: an empty or cleared box is one ~20ms capture per poll —
~160ms of rex calls spread across `ConfirmSubmitted`'s two-second window. A
box with text also pays the echo join and the dim read (~330ms per sample,
`capturedComposerHold`'s own measurement on this box), so a genuinely held
prompt costs four or five of those inside the window and a RETIRED one costs
exactly one, because the retirement answers `""` on the first poll.

There is no separate ghost arm at the capture route's deadline, unlike the
herdr loop. There the dim read is one extra `agent read` and is deferred to
the one poll that is about to warn; here the dim read and the echo join are
inside the reading, off the SAME capture, so by the time a `Typed` is seen at
all both retirements have run. The echo one is the better reading for this
call site and the herdr arm does not have it: posse has just submitted, so a
box showing the line claude's log says was submitted last IS the successful
submit.

## The gap this leaves, said out loud

`capturedHold` fills `Work` and leaves `Typed`, `Sent` and `Ghost` empty until
ranger-base-qzujz's composer reading is in the tree (5fa6bafc, committed on
`posse/gwart-posse-ranger-base-qzujz`, not on main when this landed). So the
ROUTE is here, wired and pinned as a route, and the REPORT begins the day the
reading behind it does. Nothing further is owed at either call site.

That is also why arm 1 of `internal/posse/confirmsubmitted_qa_test.go` asserts
the answer against `capturedHold(seat).Typed` rather than against a literal:
it is true in both worlds, where a literal would be an assertion somebody has
to come back and edit. What fails today if the route is taken away again is
the capture assertion beside it.

## The arms, and the mutant each one kills

MUTATION-CHECKED, 2026-11-10, each against the exact wrong code:

| arm | kills |
|---|---|
| `TestQAConfirmSubmittedReadsTheCaptureWhereTheSlotHasNoAgentExplain` | dropping the §4 branch — the status quo, where no screen is read at all |
| `TestQAConfirmSubmittedOnAFailedHerdrReadsNoScreen` | taking the capture on ANY explain error, so a transient herdr failure reads a different store than the reading is defined over |
| `TestQAPromptDoesNotWarnAboutClaudesOwnSuggestion` | the call site back on `det.Composer()` |
| `TestQAPromptStillWarnsWhenTheSameBoxIsNotDrawnFaint` | the same box with no dim run — a prompt that really is sitting there |
| `TestQAPromptDoesNotWarnAboutABoxEchoingTheLastSubmit` | the call site back on `det.Composer()` |
| `TestQAPromptStillWarnsWhenTheStoreDoesNotKnowTheBox` | a store whose last row for this session is another line |

Arm 2 needs its explain error ARMED (`explain-error`) rather than provoked
with an unknown pane: this package's fake herdr answers `agent explain` for
any pane with an idle fallback, so a pane nobody seeded exercises the success
path. `capturedhold_qa_test.go`'s arm 3 reads as though it provokes one and
does not; it passes on the absence of regions instead, which is a weaker claim
than its name makes. Not changed here — it is ranger-base-r5131's arm and it
is not wrong, only narrower than it reads.

## Landed beside it

`ranger-base-17puk` — main did not build. a2327230 replayed
ranger-base-k0hd6 onto a main that had meanwhile landed ranger-base-r5131,
and took both sides' text without reconciling three call sites of the shape
k0hd6 deleted. `capturedHold` takes a `Seat` now, `PaneHolding` passes the
one it already holds, and r5131's two test files hand it one. Its own commit
(e8a6da1d), its own bead, because it is another close's reland and not this
bead's subject — but it is the floor of this work and nobody on the box could
run a suite until it landed.
