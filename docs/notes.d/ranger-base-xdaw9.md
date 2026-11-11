# herdr's typed route loses a prompt into a modal too, and the tell is the screen (ranger-base-xdaw9)

2026-11-11. From ranger-base-hlnyx, which closed this on the Rex route and
filed its open question (c) as this bead: whether `Herdr.sendTextPrompt`'s
`pane send-text` + `pane send-keys enter` has the same hole, and what its tell
is. **It has. Three for three. And the tell ranger-base-hlnyx predicted is
measured wrong** — herdr's own composer region cannot tell a covered box from
an unread one, so the reading that works is the capture route's own.

ADR 0076 Amendment 8 is the decision. The code is `refuseWithoutComposerPane`
(internal/posse/reportedagent.go) over `captureComposerSight`
(internal/posse/capturedcomposer.go); the arms are
internal/posse/promptnocomposerpane_qa_test.go.

## The venue

MEASURED 2026-11-11, 00:15-00:35 EST, on this box (linux, omarchy 7.2.15-1).
This is a `multiplexer: rex` instance and no herdr server was running, so the
probe started a scratch one (`herdr server`, the default session, the
operator's own socket) and stopped it at the end; `go run ./cmd/checkorphans`
clean afterwards.

- herdr `0.9.1`, client protocol 22, server private protocol 22
- claude `2.1.322`, model `claude-haiku-5-5`, manual permission mode for the
  dialog rows, the folder-trust prompt for the launch row
- claude's detection manifest
  `remote:/home/posse/.local/state/herdr/agent-detection/remote/claude.toml
  2026.10.11.1`
- posse built from this worktree at e3f0290f
- one pane, `w1:p1`, cwd a scratch dir under this session's scratchpad

The route was driven by its own two calls rather than through `posse prompt`,
because posse's wrapper adds nothing to the keystrokes and this box's instance
does not run herdr: what was under test is what the CLI does when those two
calls arrive while a modal is up.

## MEASURED: how a claude pane reaches a route for panes herdr does not detect

This had never been measured and it is the whole mechanism. **`pane
report-agent` overrides the LABEL and does not blind the DETECTION.** Same
pane, same instant, with claude's folder-trust modal up:

```
pane report-agent w1:p1 --source probe-xdaw9 --agent fakebob-xdaw9 --state idle
agent get w1:p1      → {"agent":"fakebob-xdaw9","agent_status":"idle",…}
agent explain w1:p1  → agent: claude
                       state: blocked
                       rule: live_blocked_form (region=after_last_horizontal_rule priority=980)
agent explain --file <empty> --agent fakebob-xdaw9 --json
                     → {"fallback_reason":"unknown_agent","manifest_version":null}
agent explain --file <empty> --agent claude --json
                     → {"fallback_reason":"default_known_agent_idle_fallback",
                        "manifest_version":"2026.10.11.1"}
agent prompt w1:p1 "PROBEXDAW9ZERO"
                     → {"error":{"code":"agent_not_ready",
                        "message":"agent w1:p1 is not an active named agent"}}
```

So posse's two authorities disagree about one pane. `ReportedAgent` reads
`agent get`, `AgentManifest` says `unknown_agent`, and the prompt is routed to
the typed path — into a live claude. `agent prompt` really does refuse the
pane, which is why that path exists (ADR 0061, ranger-base-8eqaa) and also why
herdr's own blocked rejection (rangerhq-ejf) is not standing in front of these
keystrokes. The `agent_blocked` guard at the head of `sendTextPrompt` does not
catch it either: the status it reads is the REPORTER's, and the reporter said
`idle` over a pane herdr's own detection called `blocked`.

The label also survives the CLI: after the first attempt quit claude, `agent
get` still read `fakebob-xdaw9` / `idle` over a pane that was a bare shell.
That is ADR 0061 Claims fact 1 reproduced on this herdr, and the liveness half
of the one detection reading (`paneShellForeground`) is what answers it.

## MEASURED: three attempts, the prompt gone from every source, the Enter spent

| # | modal up | highlighted row | token in `pane read` visible / recent / recent-unwrapped | what the Enter did |
|---|---|---|---|---|
| 1 | folder-trust prompt | `❯ No, exit` | 0 / 0 / 0 | claude EXITED — the pane was a shell prompt 3s later |
| 2 | Bash permission dialog | `❯ 1. Yes` | 0 / 0 / 0 | the `touch` RAN; `xdaw9-approved.txt` written at 00:25 |
| 3 | Bash permission dialog | `❯ 1. Yes` | 0 / 0 / 0 | the `touch` RAN; `xdaw9-second.txt` written at 00:28 |

Neither herdr call failed on any of the three, so this route would have
reported a delivered prompt every time. `herdr agent prompt`'s refusal above
is the only thing in the stack that was ever going to say anything, and it is
the call this route does not make.

Both approved commands were a `touch` of a scratch file chosen for that
reason; the trust prompt's `No, exit` costs a relaunch. `/model` was NOT typed
into — ranger-base-8eeoy's own run wrote a default model into the operator's
`~/.claude/settings.json` that way and could not revert it
(ranger-base-rpte4). Its screen was read with the dialog up and dismissed with
Esc.

## MEASURED: why herdr's composer region is not the tell

ranger-base-hlnyx filed this bead expecting the region to carry the dialog's
MENU, so that `AgentDetection.Composer()` would answer `""` through its chrome
clause (`composerChromePhrases`, panework.go, ranger-base-i6t90) and the split
needed here would be "the region is there and empty" against "the region is a
menu drawn over the box". The region does three different things:

| screen | `prompt_box_body` preview | herdr's own `state` | rules on screen |
|---|---|---|---|
| Bash permission dialog | `""` | `blocked` | 1 U+2500 (own separators U+254C) |
| the `/model` picker | `""` | **`unknown`**, no rule matched | 0 U+2500 (own rule U+2594) |
| the folder-trust prompt | `" Accessing workspace:…"` | `blocked` | 2 U+2500 |
| a box on screen, idle | `"❯ check that the file was created\n"` | `idle` | 2 U+2500, box between |

- **The chrome clause fires on none of them.** Two previews are empty and the
  third does not lead with the mark, so `Composer()` answers `""` for all
  three through its FIRST clause. The five phrases do no work on this route.
- **The region reading is two-valued where the refusal needs three.** An empty
  preview is also what "no rule read that region" looks like — an older herdr,
  another manifest, a pane mid-redraw — and `regionPreview` cannot tell those
  from a covered box. ADR 0076 Amendment 7 D3's third value has nowhere to
  come from.
- **herdr's own `state` cannot stand in for it.** `blocked` on two of the
  dialogs, `unknown` on `/model`, and unknown may never refuse (ADR 0013 §1).
  A refusal built on `state` would have passed the two dialogs anybody would
  have tested it against and waved the picker through.

## MEASURED: the one reading judges all four herdr screens

`captureComposerSight`, written off `rex capture` screens for
ranger-base-hlnyx, over the same four screens taken with `herdr pane read
--source detection --format text`:

```
idle   sight=composerOpen     lastline="⏸ manual mode on · ? for shortcuts"           rules=2
perm   sight=composerCovered  lastline="Esc to cancel · Tab to amend"                 rules=1
model  sight=composerCovered  lastline="Enter to set as default · s to use this…"     rules=0
trust  sight=composerCovered  lastline="Enter to confirm · Esc to cancel"             rules=2
```

`--source detection` and not the CLI's default `recent`: it is the snapshot
`herdr agent explain --file` replays and `etc/herdr/agent-detection/testdata`
was captured with (ADR 0066 D3, `PaneReadDetection`'s note), so the screen
posse judges is the screen herdr judged. One reading, two multiplexers, one
vocabulary. The screens are the fixtures in
promptnocomposerpane_qa_test.go, normalized three ways — rules written at 80
columns rather than the terminal's 120, the scratch paths replaced for
`make crew-check`, and the transcript above each screen's chrome trimmed —
and none of those is a glyph.

One of them earns a pin of its own: the idle screen's TRANSCRIPT opens with
`❯ Run exactly this bash command…`, an echoed prompt. A reading that took a
mark anywhere on the screen for the box would read every covered screen as
open, which is `composerMark`'s own warning arriving on a third route.

## MEASURED: the leading-`/` refusal's stated reason was wrong here too

ADR 0060 D5 measured an older claude whose command picker ate the Enter and
left the text in the composer "through Esc and C-u", and this route's refusal
said so until now. Through this route, box open and idle:

```
pane send-text w1:p1 "/PROBEXDAW9SLASH answer with exactly ok"
  → the line is in the box, no picker drawn (send-text pastes, it does not type)
pane send-keys w1:p1 enter
  → ● Unknown command: /PROBEXDAW9SLASH
    ● Args from unknown skill: answer with exactly ok
  → the box CLEARS
```

Submitted and lost, not held — the same correction ranger-base-hlnyx made on
the Rex route off its own measurement. The refusal stands either way; only its
sentence moved, and the substring pin in reportedagent_qa_test.go moved with
it.

## What the fix costs, and the hazard it adds

Two herdr calls before anything is typed: one `agent explain` for the scope
and one `pane read` for the screen, per prompt and not per pass. A refusal
costs one more read and `promptComposerRecheck` (250 ms), and only a refusal
does.

The hazard is the absence rule's, which ADR 0076 Amendment 7 already states:
if a later claude stops drawing the two rules around its box, this reads
covered on every settled pane of this shape and refuses every typed prompt on
it. Loud — the first refusal says so and quotes the screen's last line — where
the failure it replaces was silent. It is pinned against the live binary and
not only against fixtures, on a herdr box:
`RHQ_LIVE_HERDR=1 go test -tags posse_arm3 ./internal/posse -run
TestLiveHerdrPaneSeesThisPanesOwnComposer` reads the pane the suite is running
in. It SKIPS here, because this box is a rex instance — which is also why the
Rex pin cannot stand in for it: one function, two screen sources, and neither
occupant's box can run the other's pin.

## What the fake herdr could not do before

Its `agent explain` branch derived the refusal from the reported LABEL alone,
and its comment said a reported-but-explainable herdr "has never existed".
That herdr is 0.9.1, measured above. The fake now derives the same refusal
from whether herdr has a manifest for the label it DETECTED
(`explain-agent-name` beside `herdr-kinds`), so it can be in the pane shape
the hazard was measured in — and the levers stay derived rather than
free-form, which is what keeps a pin of the reported arm from being satisfied
by the detected path.

## MEASURED: the fix, against the real binaries, both directions

The hermetic arms drive a fake herdr, so the fix was also run against the
scratch server above — `Herdr.AgentPrompt` called directly, on the same pane,
with claude's Bash permission dialog up and `❯ 1. Yes` highlighted:

```
ReportedAgent = ("fakebob-xdaw9"/"idle", true)      ← the typed route is chosen
AgentExplain  = (agent "claude" state "blocked")    ← the scope evidence
sight         = composerCovered, last line "Esc to cancel · Tab to amend"
AgentPrompt   → agent_not_ready: nothing was sent: w1:p1 is a fakebob-xdaw9
                pane herdr labels but does not detect … the last line of it
                reads "Esc to cancel · Tab to amend" …
isNotDelivered(err) = true
```

and nothing was typed: the token is absent from the whole scrollback
(`grep -c` = 0), the dialog is still up and unactuated, and
`xdaw9-fix.txt` — the command it was holding — does **not** exist, where the
same route ran that command twice before the fix.

Then `Esc`, and the same call over the same pane with its box back:

```
AgentExplain  = (agent "claude" state "idle")
sight         = composerOpen, last line "⏸ manual mode on · ? for shortcuts"
AgentPrompt   → {"type":"agent_prompted","via":"pane send-text",
                 "agent":{"agent":"fakebob-xdaw9","pane_id":"w1:p1"}}
```

and the prompt is in the transcript with claude's answer under it
(`❯ PROBEXDAW9OK answer with exactly ok` / `● ok`). So the gate refuses the
measured hazard and does not refuse the pane this route exists to type into.

Teardown: the reported label released, claude exited, `herdr server stop`,
`go run ./cmd/checkorphans` clean. The box is a `multiplexer: rex` instance
again with no herdr server running, which is how it started.
