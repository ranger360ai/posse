# ADR 0013 §4 native-rulebook placement probe

Measured for `ranger-base-xaev` on 2026-08-28. This is a trace, not fleet
code. Versions were Codex CLI 0.147.0 and Grok 1.0.5. No billed model turn
was spent on either runtime; see **No-spend controls**.

The question: ADR 0013 §4 reconciles the PID and a runtime's native
rulebook with one sentence in the work prompt — *"PID guardrails override
repo docs."* Does the assembled prompt's **structure** support that
sentence, or undercut it? The PID channel is the built-in `command:` of
each runtime (`internal/rhq/runtime.go`):

| runtime | PID channel | native rulebook |
|---|---|---|
| codex | `-c developer_instructions="$(cat {file})"` | `AGENTS.md`, `AGENTS.override.md`, `~/.codex/AGENTS.md` |
| grok | `--rules="$(cat {file})"` | `Agents.md`, `Claude.md`, `CLAUDE.md`, … |

Absolute paths below are abbreviated `<proj>` (the throwaway git repo) and
`<home>` (the isolated per-runtime home).

## Result

**Codex — different roles, and the PID has the stronger one.** The PID
lands in the `developer` role at index 0 of the model-visible input list;
the native rulebooks land in a `user` message, later, immediately before
the argv work prompt. **Grok — different channels, and their relative
order is not locally observable.** The PID lands at the tail of the
locally-assembled system prompt inside `<human_rules>`; the native
rulebooks are *not in the system prompt at all* — they ride a separate
structured `agents_md_files` field whose model-visible placement is
decided downstream.

So the structure **supports** the PID-wins line on codex and leaves it
**UNMEASURED** on grok. Neither runtime showed a native rulebook
outranking the PID channel, so ADR 0013 §4's revisit trigger
("if the precedence probe ever measures codex's native `AGENTS.md`
overriding the PID guardrails line") **did not fire**: suppression stays
un-adopted.

Structure is not behavior. Whether the model *follows* the native
rulebook over a conflicting PID guardrail needs a billed turn, which this
probe did not spend (ranger-base-xaev's money line).

## Codex placement matrix

Instrument: `codex debug prompt-input [PROMPT]`, which renders the
model-visible prompt input list as JSON without starting a turn. Fixture:
`<proj>/AGENTS.md` and `<home>/AGENTS.md` carrying distinct markers, a
`-c developer_instructions=XAEV_PID_MARKER…` PID marker, and an argv
marker.

Baseline (`-c developer_instructions=<PID> "<argv>"`):

| # | role | content | marker |
|---|---|---|---|
| 0 | developer | `<PID marker>` + codex's own `<skills_instructions>` / `<permissions instructions>` | `XAEV_PID_MARKER` @0 |
| 1 | developer | codex's multi-agent role text | — |
| 2 | developer | `<multi_agent_mode>` | — |
| 3 | **user** | `# AGENTS.md instructions for <proj>` → `<INSTRUCTIONS>` *home doc* `--- project-doc ---` *project doc* `</INSTRUCTIONS>` + `<environment_context>` | `XAEV_HOME_CODEX_AGENTS_MARKER`, then `XAEV_PROJECT_AGENTS_MARKER` |
| 4 | user | the argv work prompt | `XAEV_ARGV_MARKER` |

Four facts fall out of the arms:

1. `developer_instructions` is **prepended verbatim to codex's own first
   developer message** — it does not replace it and is not a separate
   item. Control: with the flag omitted, item 0 is byte-identical minus
   the marker (7437 vs 7482 bytes) and carries no marker.
2. `~/.codex/AGENTS.md` **rides the same channel as the project doc** —
   one user message, home doc first, separated by a literal
   `--- project-doc ---`. ADR 0013 §4 recorded this as ASSUMED; it is now
   measured.
3. `-c project_doc_max_bytes=0` removes the **project** doc only. The
   home `~/.codex/AGENTS.md` **survives at full length** (item 3 shrinks
   1041 → 779 bytes and keeps `XAEV_HOME_CODEX_AGENTS_MARKER`; the
   heading loses its `for <proj>` suffix). At `=40` the project doc is
   truncated mid-marker while the home doc is untouched. So the key is a
   *project-doc byte cap*, not a rulebook switch — ADR 0013 §4's point 4
   ("only half measured … 'single rulebook' is ASSUMED even where the
   flag works") resolves as **false**: the flag never yields a single
   rulebook while `~/.codex/AGENTS.md` exists.
4. `AGENTS.override.md` **replaces** `AGENTS.md` as the project doc when
   both are present (only `XAEV_OVERRIDE_MARKER` appears; the `AGENTS.md`
   marker is gone), matching the `codexNativeRules` list order.

Codex's own harness text names the rulebook as an authority twice —
`"Only set model or reasoning_effort when explicitly requested by the
user, applicable AGENTS.md instructions, or skill instructions"` and
`<multi_agent_mode>… unless the user or applicable AGENTS.md/skill
instructions explicitly ask …"`. That is the countervailing detail: the
PID has the stronger *role*, the rulebook has the later *position* and a
standing endorsement from the runtime's own prompt.

## Grok placement matrix

Instruments, both structural: `grok inspect --json` (discovery), and the
per-session artifacts grok writes under
`$GROK_HOME/sessions/<cwd>/<session-id>/` at session creation —
`system_prompt.txt` (the locally-assembled system prompt) and
`prompt_context.json` (the structured context). Same fixture, plus
`<home>/AGENTS.md`.

| arm | `system_prompt.txt` | `prompt_context.json` |
|---|---|---|
| `--rules <PID>` | 5853 B, ends `…</browser_verification>\n\n<human_rules>\n<PID marker>\n</human_rules>` | `prompt_mode: extend`; `agents_md_files: [<home>/Agents.md, <proj>/Agents.md, <proj>/Claude.md]` with full contents |
| control, no `--rules` | 5779 B, **no** `<human_rules>` block | identical `agents_md_files` |
| `--rules` + `--system-prompt-override` | **29 B** — the override text alone; **no `<human_rules>`, PID marker absent** | identical `agents_md_files` |

Three facts:

1. `--rules` is wrapped in `<human_rules>` and appended **last** in the
   default system prompt. The control arm's 74-byte difference is exactly
   that block, so the placement is the flag's, not an artifact.
2. The native rulebooks are **not in the system prompt**. They are carried
   as `agents_md_files` — home file first, then project files — in the
   structured context. `grok inspect --json` reports the same three files
   with `scope: global|project`. Their model-visible placement relative to
   `<human_rules>` is decided by grok's own harness (the `prompt_mode:
   extend` path) and is not observable from any local artifact: the ACP
   `initialize` / `session/new` / `session/prompt` requests in
   `--debug-file` carry the `_meta.rules` string and the argv prompt, but
   no rulebook content. That is the honest boundary of this probe.
3. **`--system-prompt-override` silently discards `--rules`** — i.e. it
   discards the PID, while leaving every native rulebook in place. Grok's
   shipped documentation (`$GROK_HOME/docs/user-guide/12-project-rules.md`)
   states it: *"Grok uses the text verbatim and skips both the default
   system prompt and `--rules`."* The built-in grok template does not use
   the flag, so nothing in the fleet is affected today; a hand-written
   PID `command:` is the path that could. Filed separately.

The same doc states grok's own conflict rule for instruction files:
*"files in deeper directories appear later in its context and take
precedence when instructions conflict."* Last-position-wins is grok's
declared model, which is precisely why the unobservable half — where
`agents_md_files` lands relative to `<human_rules>` — is the half that
decides the answer on this runtime.

## Claude

Out of scope for this probe (ADR 0013 §4 names codex and grok), and claude
exposes no comparable local prompt dump. The existing datapoint is
behavioral, not structural: an `AGENTS.md` collision on claude was
resolved in the PID's favour (rangerhq-cmfj, already in ADR 0013's
Claims).

## What this fills

`rules_precedence:` (ADR 0017 §5, field built in ranger-base-livv) stays
**UNMEASURED** on all three built-ins after this probe. Structural
placement is `rules_precedence_why:` material, not a value: the value is a
claim about which instruction the model *follows*, and that is the billed
half.

## No-spend controls

Same shape as the cl7 probe. No operator credential was used and no model
turn was produced.

- Codex: isolated `CODEX_HOME` with a synthetic invalid API key.
  `codex debug prompt-input` renders the prompt list locally; it starts no
  session and contacts no API.
- Grok: isolated `GROK_HOME`, a synthetic invalid `XAI_API_KEY`, a scratch
  `--leader-socket` (the operator's leader was never contacted), and the
  whole process wrapped in `sandbox-exec -p '(version 1)(allow default)
  (deny network*)'`. Session creation is local — `session/new` succeeded
  under denied network — so the artifacts above were written; every
  outbound call failed at DNS, so no prompt left the machine and no turn
  was billed.
- Fixture content was synthetic markers written for this probe. No repo or
  operator content was placed in any prompt.
- Note: `grok inspect` reads the operator's `~/.claude` for permissions,
  hooks and skills even under an isolated `GROK_HOME`; that half of its
  output is live-state, and is not part of this matrix.
