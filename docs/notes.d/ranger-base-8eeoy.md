# Rex's `Prompt` does not leave text in the composer — it loses the prompt instead (ranger-base-8eeoy)

2026-11-10. From ranger-base-um1bi, which wired `ConfirmSubmitted` and `posse
prompt`'s pre-type warning onto the capture route for an occupant with no
`agent explain`, and then said in its own note that whether the failure mode
those two REPORT exists here was not measured. It is measured now, and the
answer is no — while two failure modes that are worse than the reported one
do exist on this occupant, and the composer read-back is structurally blind
to both.

## The venue

MEASURED 2026-11-10, 22:33-22:52 EST, on this box (linux, omarchy 7.2.15-1),
one-minute load average 22 at the start. One disposable crew seat —
`posse new probe-8eeoy --agent gilfoyle --runtime claude --tier fast --model
claude-haiku-5-5 --dir <a scratch cwd>` — driven through `posse prompt`, which
is `RexMux.Prompt` (ADR 0076 §2: `rex send` then `rex send-key enter`) with
`ConfirmSubmitted` as its read-back. Killed at the end; `go run
./cmd/checkorphans` clean.

- rex `0.0.1794069975-g6c6689`, server api revision 11
- claude `2.1.322`, model `claude-haiku-5-5`, `--permission-mode auto` (the
  launch line's), switched by hand to manual mode for the G rows
- posse built from this worktree at 765d6f75

Ground truth was a capture series taken around each attempt, independent of
posse's own reading: `rex capture --format vt --trim` every 300ms, which on
this box's load sampled every **0.38s** — so a hold shorter than ~0.4s is
below this instrument's resolution. `ConfirmSubmitted`'s own two-second
window is the detector that matters and it is not sampled: it either printed
its warning or it did not.

## MEASURED: 22 attempts, 20 of them typed, none left text in the box

| state | attempts | typed | call reported | text left in the box | what actually happened |
|---|---|---|---|---|---|
| seat mid-boot, no `program_status` yet | 2 | **no** | refused, exit 1 | n/a | the §3 detection gate refuses before anything is typed, and `--now` does not bypass it |
| settled idle seat | 14 | yes | success | **never** | typed and submitted; the box was clear by the next sample |
| seat mid-turn | 1 | yes | success | no | taken into claude's queue; the box reads a DIM `Press up to edit queued messages` |
| command picker already open | 1 | yes | success | no | submitted as `Unknown command: /PROBED1` — the prompt was **lost** |
| box already holding typed text | 1 | yes | success | no | the garbled joined line was submitted |
| a modal dialog open | 3 | yes | success | no — there is no composer region at all | the prompt was **discarded** and the Enter **actuated the dialog** |

So the second branch of the bead's stopping condition: **N = 20 typed
prompts across five states, and not one of them was typed-but-not-submitted**,
and `ConfirmSubmitted`'s warning fired on none of them. (Six of the fourteen
idle rows were attempts at the mid-turn state that found the seat already
settled: haiku answered every prompt it was given in 0-3s, and the one that
did hold — CC — held because it was given ten sequential tool calls to make.
They are counted as what they measured, not as what they were aimed at.)
`herdr agent prompt`'s 2026-09-02 failure (three times, the last sitting four
hours) did not reproduce through this occupant's two-call route.

### Why the idle case cannot hold, and what that is worth

`rex send` and `rex send-key enter` arrive on the same pty in order, and
claude's CLI consumes them in order, so on a settled seat the submit is not a
race this instrument can lose: 14 for 14, and the read-back said "cleared"
on all 20 typed attempts. The typed text was CAUGHT in the box twice in
twenty, each time in exactly one sample before it cleared — once on an idle
seat (B1, at 6.53s into its run) and once on the attempt whose box was
already dirty (E1, 5.09s, where what the sample held was the joined line) —
which bounds the transit at under ~0.8s. What is NOT claimed: that no state
exists where the Enter is dropped. What is claimed is that of the four
adversarial states anybody has named for this route, three — mid-turn, picker
open, box already dirty — were typed into and held nothing, the fourth
(mid-boot) is refused before a key is sent, and the fifth, which nobody had
named, produces something else entirely.

## MEASURED: the two modes that ARE here, and the reading cannot see either

**A modal eats the prompt and the Enter operates the dialog.** Three
attempts, all three the same (F1 with `/model` open, G2 and G4 with a Bash
permission dialog open on a `touch`):

- `posse prompt` exited 0 and printed `rex send + enter into block:…`.
- The prompt text appeared **nowhere** — not in the box, not in the
  transcript, not anywhere in the whole scrollback (`grep -c` = 0, three
  times).
- The Enter actuated whatever the dialog had highlighted. F1's highlighted
  row was `Haiku ✔` under `Enter to set as default`, and claude answered `Set
  model to Haiku 5.5 and saved as your default for new sessions` —
  see "What this measurement broke" below. G2's and G4's was `1. Yes`, and
  **the pending shell command ran**: both files exist, written at the second
  the prompt was typed.
- `ConfirmSubmitted` printed nothing, and could not have. While a modal is up
  there is no composer region on the screen at all — claude draws the dialog
  where the box was — so `captureComposer` finds no region, the reading is
  `""`, and the read-back answers "the box cleared". That is um1bi's own
  sentence, "the question was never asked reported as the box cleared",
  arriving through a door um1bi did not close.

That is a worse failure than the one the warning was built for. A prompt
sitting in a box is recoverable and visible to the next person who looks; a
prompt consumed by a modal is gone, and the keystroke that consumed it
approved a tool call nobody reviewed.

**The picker submits instead of holding, so the prompt is lost there too.**
ADR 0060 D5 measured a `send-text` + Enter route whose picker ATE the Enter,
leaving the text in the composer "through Esc and C-u". On claude 2.1.322 that
is no longer what happens: with a leftover `/` in the box, the joined line was
SUBMITTED, claude answered `Unknown command: /PROBED1` plus `Args from unknown
skill: answer with exactly ok`, and the box cleared. `RexMux.Prompt` refuses a
leading `/` in its own argument (D5's rule, still right), but the picker
arrives sideways like this — and when it does, the hazard is a lost prompt
rather than a held one. **ADR 0060 D5's composer claim is CLI-version-bound
and this CLI does not have it.**

### What DID earn its keep, twice

um1bi's other half — the pre-type warning, now read off `PaneHolding` instead
of the readiness gate's zero detection — fired correctly on both attempts
where the box was genuinely dirty, naming what was in it:

```
warning: probe-8eeoy already has a prompt sitting UNSENT in its box ("/") — this text is typed after it
warning: probe-8eeoy already has a prompt sitting UNSENT in its box ("LEFTOVERE1") — this text is typed after it
```

and stayed silent on all 14 idle attempts — at least two of which (B4, C4)
had a dim suggestion sitting in the box in the sample taken just before it
read, so the silence is a retirement and not an empty screen. So the capture route's
`Typed`/`Ghost` split is doing exactly what ranger-base-6o7wm and
ranger-base-qzujz built it to do, on this occupant, against live screens.

## MEASURED: claude's own suggestion predicts the next prompt, verbatim

An accident of running numbered probes, and it is the strongest argument yet
for the dim rule. After `PROBEB1`…`PROBEB3` had been submitted, the composer
on the settled seat held:

```
❯\xa0\e[0m\e[2mPROBEB4 answer with exactly ok\e[0m
```

— claude's ghost suggestion, having extrapolated the series. The same thing
happened again at `PROBEB5` and at `PROBEE2`. So a ghost box's CONTENT can be
indistinguishable from a prompt posse itself would type: it is not always
`Try "create a util logging.py that…"`, and no content rule could have told
these apart. The SGR 2 run is the whole of the distinction, which is what
`ghostbox.go` says, measured here against a suggestion engineered — by the
model, not by me — to look exactly like the thing it must not be mistaken for.

And the queue placeholder is the same shape: `Press up to edit queued
messages` is drawn under SGR 2 as well, so monica's worry on the bead (a
`Typed` reading counting a queued line as unsent) does not arise on this CLI —
not because the reading special-cases the string, but because claude draws it
faint like every other placeholder.

## What this changes

Nothing in the code, which is the answer the bead expected either way:
`ConfirmSubmitted`'s capture route is report-only, costs one ~20ms capture on
a cleared box, and prints nothing. It stays. What changes is the claim —
`confirmSubmittedCaptured`'s note said the mode was unmeasured here, and now
it says what 18 typed prompts found, so the shop stops carrying an unmeasured
claim.

What the measurement FOUND, which is not this bead's to fix, is
**ranger-base-hlnyx** (P1, `-l code`): a prompt delivered into a seat holding
a modal is reported as submitted, is lost, and actuates the dialog. The route
has a cheap tell — while a modal is up there is no composer region on the
screen, which is the same capture `PaneHolding` already takes one call
earlier.

## What this measurement broke, and could not put back

The F1 Enter landed on `/model`'s `Enter to set as default` and claude wrote
`"model": "haiku"` into **`/home/posse/.claude/settings.json`** (mtime
2026-11-10 22:41:57), which is `CLAUDE_CONFIG_DIR` for every claude seat on
this box. No posse-launched session reads it — every launch line passes
`--model` explicitly — so nothing in the fleet changed behaviour; an
interactive `claude` started by hand would pick it up. The key did not exist
before.

Reverting it is one key, and **this seat's own gate refuses the write**: a
session carrying `RHQ_PERSONA` is denied any write to `.claude/settings.json`
(self-modification), which is the gate working. Driving another seat into
doing it is working around the denial, so it was not done. Filed for the
operator with the exact edit as **ranger-base-rpte4** (`-l question`). The file's `"theme": "auto"` may also be from this venue — claude's
first-run theme chooser came up on launch and was dismissed with Enter on the
already-checked `Auto (match terminal)` — and it is left alone deliberately:
removing it would put the chooser back in front of the operator for no gain.
