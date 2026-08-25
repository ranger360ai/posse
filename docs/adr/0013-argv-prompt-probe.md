# ADR 0013 argv prompt probe

Measured for `ranger-base-cl7` on 2026-08-25. This is a trace, not fleet
code. Versions were Codex CLI 0.147.0, Grok 1.0.5, and herdr 0.8.0.

## Result

`prompt: argv` holds for both Codex and Grok. In each interactive TUI the
positional prompt became the first user turn without a keystroke from the
harness, the process entered its normal working UI, and the corresponding
herdr manifest classified a cleaned capture as `working`.

The interstitial result is more precise than "the screen disappears":

- Grok briefly drew its `New worktree` / `Resume session` startup splash,
  then cleared it and submitted the argv text as turn `#1`.
- Codex, when seeded with an available-version record, drew its update
  banner before the session. It did not wait for a selection: the argv text
  still became the first user turn and reached `Working`. Thus argv
  sidesteps this interstitial as a delivery blocker; it does not suppress
  the informational banner.

No typed fallback or `startup_wait` value is required by this probe.

## No-spend controls

The probe did not use an operator credential. Each CLI ran with an isolated
writable state directory and a synthetic invalid API key. Network access was
also denied by the execution sandbox. The invalid model name was attempted
first, as requested, but neither CLI rejected it locally: Codex warned that
it was using fallback metadata and Grok displayed its default `Grok 4.6`.
Both then entered their working/retry UI without producing a model response.

The marker was `ARGV_PROMPT_MARKER_CL7`. Relevant cleaned terminal traces:

```text
# Codex, after the non-blocking update banner
› ARGV_PROMPT_MARKER_CL7
• Working (0s • esc to interrupt)

# Grok, after the startup splash cleared
#1 ARGV_PROMPT_MARKER_CL7
⠙ Waiting for response… 0.1s   0.1s ⇣1.63k [stop]
```

The launches were interactive positional-prompt commands, not `codex exec`
or Grok single/headless mode. The state directories and credentials were
throwaway probe data.

## Herdr detection

The sandbox denied both connection to the installed herdr socket and binding
a scratch server socket, so no live fleet pane was created or mutated.
Detection was checked offline with the installed manifests using cleaned
captures taken from the raw PTYs:

```text
herdr agent explain --agent codex --file codex-working.txt --json
  state=working matched_rule=screen_working_fallback visible_working=true

herdr agent explain --agent grok --file grok-working.txt --json
  state=working matched_rule=spinner_status_working visible_working=true
```

The raw commands' foreground argv0 values were `codex` and `grok`, matching
the manifest labels. This proves the captured post-argv screens are
herdr-detectable; it does not claim a live server round-trip that the sandbox
would not permit.

## Native project rules

A throwaway git worktree contained marker files `AGENTS.md` and `CLAUDE.md`.
Local inspection produced this matrix without starting a model turn:

| runtime/config | discovered project instructions |
|---|---|
| Codex baseline `debug prompt-input` | `AGENTS.md` marker present |
| Codex `-c project_doc_max_bytes=0` | project marker absent; argv marker present |
| Codex `project_doc_fallback_filenames=[]` | `AGENTS.md` marker still present |
| Grok baseline `inspect --json` | `Agents.md`, `Claude.md` |
| Grok `--verbatim` | `Agents.md`, `Claude.md` |
| Grok `--system-prompt-override=...` | `Agents.md`, `Claude.md` |
| Grok both candidate flags | `Agents.md`, `Claude.md` |

ADR 0013's assumption that neither CLI has a flag which silences native
project-rule discovery is therefore false for Codex: its generic CLI config
override can set `project_doc_max_bytes=0`, which removes `AGENTS.md` from
the model-visible prompt. No tested Grok flag silenced its project-rule
discovery.

The architecture owner must decide whether dispatch should use that Codex
override or continue declaring the native rulebook and treating the PID-vs-
project-rule relationship as a trust probe. This measurement does not make
that design choice.
