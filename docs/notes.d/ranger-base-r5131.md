## A guard that was off, not missing: htafy's hold on a rex instance (ranger-base-r5131)

2026-11-10. The bead asked for a new rule — "treat a seat with a live
background shell as working, or delay settle-open until it clears". It
needed neither. The rule has existed since ranger-base-htafy, two months and
one multiplexer ago; it was *off*, and the interesting part is that nothing
anywhere said so.

### What the bead saw

`gilfoyle-posse-ranger-base-nfc2b` showed `done` in `posse list` and settled
twice without closing its bead. A scoped capture showed the seat idle between
turns on purpose — `1 shell still running`, test arms in flight, "I'll hold
here until the arms report" — with 459 uncommitted lines staged for the
close. The 9hm settle-open rule re-prompted it once; the next pass escalates
to the operator as a `-l question`. A false interrupt on a seat that was
doing exactly what it was told.

### What it actually was

ranger-base-htafy (2026-09-02) had already measured this exact class and built the guard:
`PaneHold` (internal/posse/panework.go) reads claude's own footer summary of
its live background tasks and three callers spend it by *not* acting —
dispatch's `--resume` skip does not re-prompt (dispatch.go), gather does not
count a settle-open (settleopen.go), the pulse does not file a G2 row
(govern.go).

It reads that footer off herdr's `agent explain` region previews. That is
ADR 0076 §4 — a herdr-only verb — and this instance is `multiplexer: rex`
(`~/homes/ranger/config.yaml:306`). So `HerdrBackend.explain` answered
`muxNotHerdr("agent explain")`, `PaneHolding` returned the empty hold,
`Waiting()` was false, and all three guards had been off on this box since
the day the slot changed.

Nothing was broken, which is why nothing reported it. §4's contract is
"not available, never no", every caller of a §4 verb has an arm that
concedes and goes on, and conceding is what each of these did. The failure
mode of a well-behaved degrade is that it is indistinguishable from the
thing it degrades to.

**THE BUG REPRODUCED ON THE BEAD THAT WAS FIXING IT.** The measurement below
was taken by backgrounding a declared recorder in my own seat and ending the
turn across it. The turn ended 20:15:53; at 01:15Z ranger-base-r5131 took its
own `settled open [in_progress]` comment and was re-prompted. One more pass
and this bead would have escalated itself to the operator.

### Which store carries "a background shell is still running"

MEASURED 2026-11-10 20:15-20:17 -05:00, this host (linux, rex CLI
0.0.1794069975, claude 2.1.322, `multiplexer: rex`), session
`ranger/gilfoyle-posse-ranger-base-r5131`, 23 samples at 3s spanning the turn
boundary:

| store | what it said | use |
|---|---|---|
| `program_status` root record | `state=working` → `state=done` at 20:15:53, owner `claude(258895)`, **one** record, no child record, for all 49s of the settled window | none: the right answer to the wrong question |
| `process` table | `child`/`foreground`/`ancestors` = the block-shell → claude chain only; the recorder (pid 321592, a child of CLAUDE) never appeared, nor did the gate shell taking the samples | none |
| claude's footer | `⏵⏵ auto mode on · 1 shell`, in **every** settled sample | the only carrier |

So the fact exists on Rex, in claude's own words, in the one place ADR 0076
did not look. Two of those three rows are worth more than the fix: they are
the reason nobody should go looking for a child `program_status` record for
this, and the reason the process table cannot be made to answer it.

### The fix, and the line it is drawn against

`capturedHold` (panework.go) takes the same footer off §2's `Read` verb —
which both occupants have — when and only when §4 says the slot has no
`agent explain`. ADR 0076 "Amendment 5" carries the design; the short form:

- **Not a screen rule.** ADR 0076 rejected screen rules on Rex and declines
  to invent a state there. This reading cannot produce a state. Its one
  permitted output is "this settle is not one"; it cannot say the opposite,
  so a wrong one costs a pass, where a wrong detection rule closes a bead on
  a turn nobody saw. That asymmetry is the whole licence.
- **One parser.** `backgroundWorkIn` holds claude's task vocabulary once and
  both routes feed it. A second copy is exactly the "second detection
  engine" the ADR refuses.
- **Positional, never a search.** Only the lines after the screen's last
  horizontal rule — herdr's own `after_last_horizontal_rule`, located rather
  than asked for. A free scan matches transcript prose: *this page* quotes
  `· 1 shell ·` above, and a hold invented out of a quotation can never go
  false, because nobody can clear a line of transcript. The pin for it
  reddens when the anchor is removed.
- **A failure is still ignorance.** Only `muxVerbMissing` — the verb does
  not exist, permanently — reaches the capture. A herdr that was asked and
  failed answers the empty hold exactly as before: that silence is news, and
  a transient failure must not switch which store a reading is defined over.

The composer half of htafy is still herdr-only and deliberately so: telling
a prompt sitting unsent from a box echoing one already submitted needs
`paneAgentSession`, itself §4, and without it every echo reads as a hold —
which *invents* holds, the one direction panework.go refuses. Filed as
ranger-base-qzujz.

### Numbers worth keeping

- `rex capture --format text` of a live claude block: **60 lines, 8317
  bytes, 21 ms**, three runs byte-identical. It is the SCREEN and not the
  documented 64 MiB of scrollback, because claude is on the alternate screen
  buffer. The screen tail is a rule, `❯` and its U+00A0, a rule, a blank,
  then the footer.
- Live pin, real server, one background shell live:
  `Work = "1 shell"`, `Why = "1 shell still running"` (MEASURED
  2026-11-10). `RHQ_LIVE_REX=1 RHQ_LIVE_REX_WORK=1 go test ./internal/posse/
  -run TestLiveCapturedHoldReadsThisSeatsOwnFooter` — it reads the seat it
  runs in, off `REX_SESSION`/`REX_BLOCK`, which are in every Rex seat's
  environment.

### The general lesson, which is not about rex

When a guard moves behind a capability boundary, the guard goes quiet and
the thing it guarded against comes back. Every §4 caller in this repo has a
correct "not available" arm, and `explainDegrade` even prints the right
sentence for it. Not one of them says what the shop LOSES when that arm
runs. A reading that exists to withhold a consequence should be checked, on
the occupant that is actually in the slot, by something that fails — which
is what the live pin is for.
