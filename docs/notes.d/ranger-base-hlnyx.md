# A prompt into a seat holding a MODAL is refused before the keystrokes (ranger-base-hlnyx)

2026-11-10. From ranger-base-8eeoy, which went looking for one failure mode
on Rex's `Prompt` and found a worse one: a prompt delivered into a claude
seat holding a modal dialog is reported as submitted, is LOST, and its Enter
actuates whatever the dialog had highlighted. That bead measured it
(docs/notes.d/ranger-base-8eeoy.md has the venue and the table of 22
attempts). This one closes it, and ADR 0076 Amendment 7 is the decision.

The code is `refuseWithoutComposer` (internal/posse/muxrex.go) over
`captureComposerSight` (internal/posse/capturedcomposer.go); the arms are
internal/posse/promptnocomposer_qa_test.go and the live pin
`TestLiveRexPromptSeesThisSeatsOwnComposer` (capturedhold_live_test.go).

## The three questions the bead left open, and what they were decided on

**(a) Is the no-composer-region reading specific enough on its own, or does
it want the dialog's own marks beside it?** On its own — as the RULE. Both
measured modals do carry one of `composerChromePhrases` in their last line,
so a presence rule would have caught both; it would also be exactly as good
as that list, which is five strings measured off three of claude's dialogs.
A dialog spelling its hint a sixth way reads as a box, and that is the
failure the gate exists to stop. Absence fails the other way — a screen that
really is idle, refused — and that costs a line somebody can read.

The marks are kept for the two jobs they are honest at. They are half of what
makes a screen READABLE, which is how `/model` is judged at all (it draws no
horizontal rule), and the refusal quotes the screen's last line so the
operator is told WHICH dialog is up.

The mid-redraw worry is answered by a second reading rather than by narrowing
the rule: a no-box screen is captured again after 250 ms and only then
believed. ASSUMED, not measured — the reason is in `promptComposerRecheck`'s
note and it is an asymmetry, not a number: a redraw is a repaint between two
21 ms captures, a dialog is up until a human acts on it.

**(b) RexMux.Prompt or the readiness gate?** `Prompt`. Every caller goes
through it — dispatch's two, the pulse, relaunch, the runtime probe, the
cockpit, `posse prompt` — where the readiness gate is on the hand path only
and `--now` bypasses it. `--now` must not reach this one: it is the operator
saying they mean this pane as it is, which is about posse's DETECTION of the
pane and not a licence to press whatever Enter a dialog has highlighted.

**(c) Does the herdr `pane send-text` + Enter route have the same hole?**
Not decided here, and nothing in either bead was measured on herdr — this box
is a `multiplexer: rex` instance. Filed as **ranger-base-xdaw9** (`-l code`,
`discovered-from`), with the repro recipe, the warning not to type into a
dialog whose highlighted row cannot be undone, and the note that the TELL is
different there: herdr's manifest NAMES the composer region and it was
measured carrying the box line and a selection menu at once
(`composerChromePhrases`, ranger-base-i6t90), so the split needed on that
route is "the region is there and empty" against "the region is a menu drawn
over the box".

The leading-'/' refusal's MESSAGE went with (c) as the bead said, but only
half of it: the Rex copy is corrected here, because that outcome WAS measured
on this route (the joined line was submitted as `Unknown command: /PROBED1`,
not left in the composer as ADR 0060 D5 measured on an older CLI). The herdr
copy and its substring pin in reportedagent_qa_test.go are untouched and are
step 3 of ranger-base-xdaw9.

## MEASURED: the screens, and why the mark is not the tell

Quoted from ranger-base-8eeoy's session log, 2026-11-10, rex
`0.0.1794069975-g6c6689`, claude 2.1.322, `rex capture --trim`. These are the
fixtures in promptnocomposer_qa_test.go, because the reading is entirely
about what claude draws and an invented screen would pin posse against
posse. Two normalizations, both of which the reading is indifferent to and
neither of which is a glyph: the rules are written at 80 columns rather than
the capturing terminal's width (`isFooterRule` asks what the line is MADE of,
not how long it is), and one scratch path inside the permission dialog's
"always allow" row is replaced, because a crew worktree path in `internal/`
is what `make crew-check` is for.

| screen | U+2500 rules | highlighted row | last line | sight |
|---|---|---|---|---|
| settled idle seat | 2, box between them | — | `⏸ manual mode on` | open |
| mid-turn, prompt queued | 2 | — | `esc to interrupt` | open |
| Bash permission dialog | **1** | `❯ 1. Yes` | `Esc to cancel · Tab to amend` | covered |
| `/model` picker | **0** | `❯ 5. Haiku ✔` | `Enter to set as default · s to use this session only · Esc to cancel` | covered |
| folder-trust prompt | **0** | `❯ No, exit` | `Enter to confirm · Esc to cancel` | covered |

Two things fall out of that table and both are in the arms:

- **All three dialogs draw their highlighted row with the composer's own
  mark.** So `❯` can never be what says "there is a box here" — which is
  `composerMark`'s own warning (panework.go) arriving on the capture route.
- **The permission dialog's separators are U+254C (`╌`), not U+2500 (`─`).**
  `isFooterRule` reads only the second, which is why that screen has one rule
  and not three. ranger-base-8eeoy's own analyser treated both as rules and
  so read `box='touch probe-perm-test.txt'` off that screen — the line
  between the two `╌` separators. posse's reading does not, and the arms pin
  it.

`program_status` cannot carry any of this: its root record is `state`, `app`,
`effective_app`, `title`, `msg`, `updated_at` and an `owner` (ADR 0076
Amendment 6's measured field list) and holds no field about what the runtime
has drawn. What `state` each of these dialogs produces was not read, and is
not guessed at in the code either — `/model` and the trust prompt can be up
with nothing running at all, so there is no one turn-state they share.

## MEASURED: the live screen reads as a box

`RHQ_LIVE_REX=1 go test ./internal/posse/ -run
TestLiveRexPromptSeesThisSeatsOwnComposer`, run in the seat that wrote this,
2026-11-10: `live sight = 2 (open=2), last line = "⏵⏵ auto mode on
(shift+tab to cycle)"`, and `RuntimeOwner` names claude. That pin is not
decoration — it is the guard on the hazard an absence rule buys, which ADR
0076 Amendment 7 states plainly: if a later claude stops drawing the two
rules around its box, this gate refuses EVERY prompt on the instance. Loud,
named and pinned, where the failure it replaces was silent.

## What it costs

Three rex calls before anything is typed: `program_status` and `process` for
the owner (the scope — the reading is claude's chrome and may only decide
about claude), and one `capture` at ~21 ms. Per prompt, not per pass. A
refusal costs one more capture and 250 ms, and only a refusal does.

The fake's capture answer became a QUEUE for this (`rex-capture-q.json`,
fakerex_test.go) for the reason the status queue exists: a reading that takes
a second look at the same block cannot be driven from outside it by a
constant.

## The refusal's code, and why it is herdr's

`agent_not_ready`, borrowed by hand the way this route already borrows
`agent_prompt_stalled` rather than coining a code its three call sites have
no arm for. It matters to exactly one reader — dispatch's hand-back
allow-list (`isNotDelivered`, promptstall.go, ADR 0084 D1) — and it is the
honest side of it: a claim kept over a prompt that was never typed is a bead
claimed by a seat nobody asked anything, waiting out a grace. D1 says adding
to that list needs "herdr's own words that nothing was sent"; this is posse's
own words about a keystroke posse itself declined to send.
