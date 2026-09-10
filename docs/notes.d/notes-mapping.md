# NOTES — how posse works under the hood (herdr-native)

The tmux-era implementation — Ghostty splits, the 2×2 grid, the bash spec,
the bubbletea launcher — lives on the **tmux-reference** branch with its own
NOTES. This file describes the herdr-native harness on main.

## The mapping

Every herdr call goes through `Herdr.Run` (`internal/posse/herdr.go`), which
shells out to the `herdr` CLI and decodes its JSON envelope
(`{"result":…}` / `{"error":{code,message}}`). The mapping
(`internal/posse/herdrback.go`):

| posse concept | herdr concept |
|---|---|
| session `name` | workspace with `label: name` |
| session dir | workspace `cwd` |
| env set values | `workspace create --env K=V` (per-workspace injection) |
| command / persona launch | `pane run` into the root pane's shell |
| attach / focus | `workspace focus` (re-aims the herdr UI) |
| kill | `workspace close` |

posse never wraps the agent process — `pane run` types the command into the
root pane's interactive shell, and herdr's own detection recognizes the
agent (claude, codex, …) and reports lifecycle state: `working`, `blocked`,
`idle`/`done`. `posse list` and the cockpit surface that state, but only when
herdr actually detects an agent — herdr says `unknown` at the workspace
level even for plain shells, so posse cross-references `agent list`.

