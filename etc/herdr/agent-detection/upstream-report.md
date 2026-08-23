# herdr: screens that hold the keyboard are detected as idle, not blocked

(codex modals; grok's startup splash)

**Not filed.** This is a draft for the operator to send; nothing here has been
published. See `README.md` in this directory.

- herdr 0.8.0 (macOS 15 / darwin 25.4.0)
- codex-cli 0.147.0
- manifest: remote `codex` 2026.08.09.1

## Summary

`herdr agent explain` reports **idle** while codex is showing a modal that
cannot accept a prompt. Detection matches no rule and falls through to
`default_known_agent_idle_fallback`. Because the fallback for a known agent is
`idle` rather than `unknown`, an orchestrator that waits for idle before
prompting will type its prompt into the dialog. Nothing errors; the prompt is
simply consumed by the modal.

Two screens are affected:

1. **"Hooks need review"** — codex's startup modal, shown with
   `features.hooks = true` and a new or changed `~/.codex/hooks.json`.
2. **Every codex modal whose footer reads `Press enter to confirm or esc to go
   back`** — `/model` is the easiest to reproduce.

The existing `live_strong_blocker` rule already matches the *other* wording of
the same footer, `press enter to confirm or esc to cancel`. codex uses both, so
this looks like the wording drifted rather than a deliberate exclusion.

## Reproduce

```sh
export CODEX_HOME=$(mktemp -d)
cp ~/.codex/auth.json "$CODEX_HOME"/
printf '[features]\nhooks = true\n' > "$CODEX_HOME"/config.toml
printf '{"hooks":{"SessionStart":[{"hooks":[{"command":"true","timeout":5,"type":"command"}]}]}}' \
  > "$CODEX_HOME"/hooks.json
# run codex in a herdr pane, then:
herdr agent explain <pane> -v
```

Or, without codex, against the captured snapshot:

```sh
herdr agent explain --file blocked-hooks-review.txt --agent codex -v
```

`herdr pane read <pane> --source detection` for the hooks dialog:

```
  Hooks need review
  1 hook is new or changed.
  Hooks can run outside the sandbox after you trust them.

› 1. Review hooks
  2. Trust all and continue
  3. Continue without trusting (hooks won't run)

  Press enter to confirm or esc to go back
```

`explain` output:

```
state: idle
rule: none
fallback_reason: default_known_agent_idle_fallback
```

Every rule is evaluated and misses. `live_strong_blocker`'s region *does*
contain the footer — only the wording differs:

```
  ✗ live_strong_blocker priority=900 region=after_last_prompt_marker state=blocked
    region: bytes=121 preview="  2. Trust all and continue\n  3. Continue without trusting
             (hooks won't run)\n\n  Press enter to confirm or esc to go back\n"
```

## Suggested fix

Two rules, both verified locally against real pane snapshots — see the
override in this directory for the exact TOML.

1. Add `press enter to confirm or esc to go back` to `live_strong_blocker`'s
   `any` list, next to the `esc to cancel` wording.
2. Add a dedicated rule for the hooks modal, keyed on its own text rather than
   the shared footer, so a footer reword cannot regress it:

```toml
[[rules]]
id = "hooks_review"
state = "blocked"
priority = 960
region = "top_non_empty_lines(20)"
visible_blocker = true
all = [
  { contains = ["Hooks need review"] },
  { regex = ['(?s)hooks?\s+(?:is|are)\s+new\s+or\s+changed'] },
]
```

Verified: hooks dialog and `/model` both report `blocked`; the composer still
reports `idle` via `osc_title_idle`; a live turn still transitions
`working` → `idle` via `osc_title_working`.

## Second instance: grok's startup splash

- grok (Grok Build) 1.0.5
- manifest: **bundled** `grok` 2026.07.16.2 (the cached remote 2026.07.16.1 is
  older and correctly ignored)

A fresh `grok` pane opens on a startup screen — a New worktree / Resume
session / Changelog / Quit menu, a `<version> is here!` line, and a "Help
improve Grok" consent banner — drawn *above* an already-rendered composer box.
`herdr agent explain` reports **idle**, `rule: none`,
`fallback_reason: default_known_agent_idle_fallback`. The OSC title region is
empty at that point, so none of the `osc_*` rules fire either.

The failure is slightly different from codex's and worth stating precisely:
text sent to that screen is **buffered, not discarded** — it appears in the
composer as soon as the splash yields — but the **Enter that would submit it
is consumed by the splash**. The net effect for an orchestrator is a prompt
that is never submitted and an agent that looks idle and willing. Reproduced
by sending `PROBE_BRAVO` to a fresh pane: nothing appeared, one `Esc` later it
was in the composer, unsent.

Suggested rule (what we run locally), keyed on the menu plus the version
footer rather than on the banner:

```toml
[[rules]]
id = "startup_splash"
state = "blocked"
priority = 1250
region = "whole_recent"
visible_blocker = true
contains = ["new worktree", "resume session"]
line_regex = ['(?i)^\s*Grok Build\s+\S+\s+\[[^\]]+\]\s*$']
not = [
  { contains = ["ctrl+.:shortcuts"] },
  { contains = ["esc:cancel"] },
  { contains = ["enter:send"] },
]
```

One trap worth flagging for whoever writes the upstream version: the "Help
improve Grok" consent banner is **not** a usable anchor. It persists after the
splash yields and is still on screen in a live, prompt-accepting composer, so a
rule keyed on it pins the pane `blocked` permanently.

Verified: the splash reports `blocked`; a live composer still reports `idle`
via `prompt_hints_idle` *with the consent banner still displayed*; a live turn
still reports `working` via `osc_progress_working`.

## The point both instances share

`default_known_agent_idle_fallback` fails toward `idle`, which is the unsafe
direction for automation — an unrecognised full-screen modal is indistinguishable
from a ready composer. Falling back to `unknown` when no rule matches but the
screen does not look like the agent's normal prompt would turn silent
prompt-loss into a visible stall.

Two independent agents (codex modals, grok's startup splash) hit this the same
way within a fortnight, and in both the *specific* missing rule was less
important than the direction of the fallback: a manifest can only enumerate the
screens someone has already seen, and every screen nobody has seen yet is
currently classified "ready for a prompt". A third signal is that grok's splash
is not an edge case at all — it is what **every** fresh grok pane shows.
