# herdr: please add an agent kind for IBM Bob (`bobshell`)

**Not filed.** This is a draft for the operator to send; nothing here has been
published. See `README.md` in this directory. (posse-side record: ADR 0060,
bead ranger-base-q0e1y.)

- herdr 0.9.1, client and server (macOS 26.4.1 / darwin 25.4.0); first measured on 0.8.2 the same day
- bobshell 2.0.5, Homebrew, under node 25.2.1
- proposed manifest and pane snapshots: `upstream/bob/` in this directory

The table below was first measured on 0.8.2. Rows 1 and 3 were measured again on
**0.9.1** the same day, with the same answer: a manifest with
`id = "bob"` dropped into `~/.config/herdr/agent-detection/` and reloaded is
ignored, `agent explain --agent bob` → `unknown_agent`, and
`agent-manifests --json` does not list it (ranger-base-p8afi). 0.9's plugin
surface does not change it either — a plugin can be a pane's *status
authority* through `pane report-agent`, which is the third row, but it cannot
add an agent kind.

## Summary

Bob is a CLI coding agent with an interactive TUI (`bob chat`), a headless
mode (`bob run`) and an ACP server. herdr has no `bob` kind, so a pane running
it is **unlabelled**: `agent_status: unknown`, `agent explain` →
`agent_not_found`, nothing in `agent list`. Not a detection bug — a missing
kind. An orchestrator cannot address the pane at all: there is no state to
wait on and no `agent prompt` target.

We could not close this from the outside, and the three routes we measured are
worth stating so the ask is precise:

| route | result (herdr 0.8.2; rows 1 and 3 re-measured on 0.9.1, 2026-09-28) |
|---|---|
| a standalone `~/.config/herdr/agent-detection/bob.toml` | ignored — not listed by `server agent-manifests`, `agent explain --agent bob` → `unknown_agent`, `manifest_source: null`, `evaluated_rules: []` (same on 0.8.0) |
| `aliases = [..., "bob"]` added to a forked manifest for a kind herdr *does* know (tried on `gemini`, then on `claude`, each installed as the ACTIVE local override at a higher version) | `--agent bob` still `unknown_agent`; the compiled alias `claude-code` resolves from the same file. Only the compiled alias table resolves, so a manifest cannot add a label |
| `pane report-agent --source posse --agent bob` | the pane gets the label and `agent wait --until working` returns on a reported transition — but `agent explain` refuses it ("does not have a detected agent label") and `agent prompt` refuses it (`agent_not_ready: not an active named agent`) |

So the kind has to be compiled in. Everything below is what we would put in
the manifest, with the snapshots to check it against.

## What the process looks like

```
argv: ["/opt/homebrew/Cellar/node/25.2.1/bin/node", "/opt/homebrew/bin/bob",
       "chat", "--accept-license", "--trust", "-w", "<dir>"]
```

i.e. `node <prefix>/bin/bob chat …` — the binary is the second argv element,
node the first, which is the shape that has to be recognised. `bob run
[prompt…]` is the headless front and `bob acp` the ACP server; only `chat`
draws a TUI.

## Reproduce

Any `bob chat` pane shows the unlabelled state. The **sign-in screen** below
is the one worth a recipe, because it needs no account and spends nothing —
point Bob at a dead gateway and it falls to the browser sign-in screen even
when a token is stored:

```sh
herdr workspace create --cwd "$scratch" --no-focus \
  --env SANDBOX=1 --env BOB_GATEWAY_URL=http://127.0.0.1:9
herdr pane run <pane> "bob chat --accept-license --trust -w $scratch"
sleep 3
herdr pane read <pane> --source detection    # -> upstream/bob/blocked-signin.txt
herdr agent explain <pane>                   # -> {"error":{"code":"agent_not_found"}}
```

`SANDBOX=1` suppresses the browser open; the dead gateway means no turn can be
spent. The other snapshots are from a signed-in session.

Against a snapshot, with no Bob installed at all:

```sh
herdr agent explain --file upstream/bob/blocked-execute-command.txt --agent bob --json
# {"agent":"bob","state":"unknown","fallback_reason":"unknown_agent",
#  "manifest_source":null,"manifest_version":null,"evaluated_rules":[]}
```

## The screens, and the rules we would write

`upstream/bob/bob.toml` is the whole proposal, with a comment per rule. Each
rule is keyed on **its own text**, never on a shared footer, for the reason
this directory's other override exists: a footer reword silently returns a
blocked screen to `idle`.

### 1. `signin` — blocked (priority 1200)

`upstream/bob/blocked-signin.txt`. A Bob with no usable gateway token draws
this instead of a composer; there is nothing beneath it.

```
                    ●∙∙ Complete sign-in in your browser…
                        (Press ESC or Ctrl+C to exit)
```

```toml
all = [
  { contains = ["Complete sign-in in your browser"] },
  { contains = ["Press ESC or Ctrl+C to exit"] },
]
```

Both lines, not the first alone: a "complete sign-in" notice over a live
composer is a plausible future build, the pair is not. Esc and Ctrl+C exit the
program, so nothing an orchestrator is willing to press clears this — it is a
human's screen.

### 2. `execute_command` — blocked (priority 1190)

`upstream/bob/blocked-execute-command.txt`, and the same dialog with a
completed tool call and two notices above it in
`blocked-execute-command-scrollback.txt`. Bob's permission prompt. It replaces
the composer at the foot of the pane.

```
  ───────────────────────────────────────────────────────────
  Execute Command
  ───────────────────────────────────────────────────────────
  Command:          bd version; command -v bd
  Approve commands:
  ┌──┐┌───────┐
  │bd││command│
  └──┘└───────┘
  → Approve Once
    Always Allow Command for task
    Reject

    ↑↓ (1/3)

  Wait for input after execution: No (Tab to toggle)

  Press Enter to confirm
```

```toml
all = [
  { regex = ['(?m)^\s*Execute Command\s*$'] },
  { regex = ['(?m)^\s*[→>]?\s*Approve Once\s*$'] },
]
```

The heading is line-anchored on purpose: a *completed* call leaves
` Execute Command (completed)` in the scrollback, which that regex excludes
and which the second fixture pins — it has both spellings on screen and must
still resolve to this rule. `Press Enter to confirm` is the footer and is not
keyed on.

### 3. `live_composer` — idle (priority 1105)

`upstream/bob/idle-composer.txt` (fresh signed-in pane) and
`idle-after-turn.txt` (the same composer after a turn settled).

```
 ─────────────────────────────────────────────────────────────
  ❯   Build Anything, @ for context, / for commands, $ for skills
 ─────────────────────────────────────────────────────────────
  Agent Mode · 10.5k / 270.0k (4%) · 0.021 🅞
```

```toml
all = [ { contains = ["Build Anything, @ for context"] } ]
visible_idle = true
```

**Below every blocker, at 1105.** We shipped an idle rule at 1250 for another
agent once and a real permission dialog drawn over it read `idle` — a
dispatched prompt typed into the dialog. `visible_idle` is what lets a caller
tell a *seen* idle from `default_known_agent_idle_fallback`, which answers
idle for any known agent whose screen matched nothing, including a pane that
is still a shell 0.2s into a launch.

### 4. `working` — **no rule, no fixture, deliberately**

We have not captured a mid-turn Bob pane. The recon's `working.txt` turned out
to be byte-identical to its `after-turn.txt` (md5
`b5a03d49b95134eaa1a9390b9a484987`) — taken after the turn had settled. Rather
than invent a spinner, the slot in `bob.toml` is a TODO. For an unrecognised
screen `unknown` is the safe answer; `idle` is not.

What is known, from Bob's own sqlite (`~/.bob/db/bob.db`, WAL): `tasks.status`
reads `running` mid-turn and `active` once settled, and each assistant message
carries `data._meta.spend = {cost, contextTokens}`. That is the database, not
the pane.

### 5. `idle-command-picker.txt` — shipped, with no rule

Typing `/` opens a command picker over the live composer. The placeholder in
§3 is replaced by the typed `/`, so `live_composer` does not match, and the
screen falls through.

We are **not** proposing a rule for it, and the reason may be the more useful
report: the picker **eats Enter**. Text typed there lands in the composer and
survives Esc and C-u, but a prompt that starts with `/` is never submitted.
That is a property of the prompt's content rather than of the pane — there is
a live composer underneath — so no detection rule fixes it, and our own answer
is a documented rule that prompts to Bob never start with `/`. The snapshot is
here so whoever writes the real manifest sees the screen and can decide.

## Why this matters to us specifically

An agent herdr cannot name from argv is one an orchestrator cannot dispatch
to: there is no readiness signal to gate on, no settled state to detect, and
no `agent prompt` path. We have declared Bob's launch row **unmet** and refuse
to dispatch it rather than guess a state — which is the honest posture, but it
means every Bob session is hand-driven until a kind exists.

The rules above are ours to be wrong about; the ask is the kind.
