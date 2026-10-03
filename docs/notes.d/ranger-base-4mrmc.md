## bob's `hidden: true` hides nothing, and the persona mode's path follows `default_dir` (ranger-base-4mrmc)

*bead: ranger-base-4mrmc (P2, qa) · verifies ranger-base-qllt8 / ADR 0062 D1-D2 ·
findings ranger-base-se81d · the billed half is blocked on ranger-base-x5xt6 ·
MEASURED 2026-10-03 on this box (darwin 25.4.0, bob 2.0.5 / commit 2dc180906,
node via Homebrew, herdr 0.9.1 client and server), posse at 4dff757a, **no bob
turn and no spend**, pane closed and `go run ./cmd/checkorphans` clean*

ADR 0062's Claims carry three ASSUMED lines. This bead measured the one that needs
no model turn, discharged it FALSE, and refused the other two on the money line —
the reasons and the precedent are on ranger-base-x5xt6, and §4 below says what the
no-spend half narrowed them to.

## 1. The rig: posse's own materializer, not a hand-written file

The point of waiting for ranger-base-qllt8 was to exercise `RenderPersonaModeFor`
rather than a copy of what it emits. A throwaway test in `internal/posse` (written,
run, deleted — it is not a pin; the pins are personamode_qa_test.go's) called
production code directly against a scratch git repo:

```
a.RenderPersonaModeFor(ag, rt, <scratch>/ws-a)   ->  <scratch>/ws-a/.bob/plugins/posse/custom_modes.yaml
rt.PersonaModeSlug("holden-probe-4mrmc")         ->  posse-holden-probe-4mrmc
ag.RenderCommandFor(rt, …)                       ->  bob chat --mode posse-holden-probe-4mrmc \
                                                       --accept-license --trust --auto-approve -w .
git -C <scratch>/ws-a status --porcelain         ->  (empty; .git/info/exclude carries /.bob/plugins/posse/)
```

The file it wrote is posse's: marker line, renderer stamp, `slug`, quoted `name`,
`roleDefinition: |` with the PID verbatim *including its frontmatter*, the ten
groups, `hidden: true`. That line above — posse's own, not a hand spelling — is
what the pane ran.

```
herdr workspace create --label posse-4mrmc-probe-a --no-focus --cwd <scratch>/ws-a   -> w2NV:p1
herdr pane run w2NV:p1 "<the line>"
herdr pane wait-output --regex 'Build Anything|Mode|not found|Error|error' --timeout 60000 w2NV:p1
   -> matched "Build Anything, @ for context, / for commands, $ for skills"
herdr pane read --source visible --format text w2NV:p1 | tail -1
   -> "holden-probe-4mrmc Mode (auto-approve)"          <- posse's file took, end to end
```

So ADR 0062 Verification row 1 has a live half now, where the pins have the fixture
half: the materializer, the placeholder and the footer agree on a real bob.

## 2. `hidden: true` keeps the mode out of nothing — MEASURED FALSE

`herdr pane send-keys w2NV:p1 shift+tab`, one press per line, footer read after each:

```
#1 -> Plan Mode (auto-approve)
#2 -> Ask Mode (auto-approve)
#3 -> holden-probe-4mrmc Mode (auto-approve)     <- posse's hidden mode, fourth stop
#4 -> Agent Mode (auto-approve)
#5 -> Plan Mode (auto-approve)
#6 -> Ask Mode (auto-approve)
```

Four stops, not three. The cycle is `listModes()`-ordered and posse's mode is in it.
(The sequence is what decides this: a mode that were absent from `listModes()` makes
`findIndex` answer -1, so every press would land on index 0 and the footer would not
move. It moves, through our mode.)

Then `/mode`, typed as text and confirmed on the one-entry command menu (`/mode -
Switch the active mode`), which opens the picker and spends nothing:

```
  Select mode
  Personas that tailor agent behavior
    Agent · builtin      Take your idea, or plan, and bring it to life
    Plan · builtin       Plans tasks: analyzes requirements, researches and designs …
    Ask · builtin        Ask questions and get explanations
  ★ holden-probe-4mrmc · workspace
    ↑↓ (4/4)
  Enter to switch · Tab to view mode · Esc to close
```

`Tab` on that row then prints the roleDefinition — the PID, frontmatter and `deny:`
list and all:

```
  Mode details
  holden-probe-4mrmc
  ---
  name: holden-probe-4mrmc
  deny: [Bash(git push:*)]
  ---
  You are holden-probe-4mrmc, a throwaway QA probe persona for ranger-base-4mrmc.

  MARKER: PERSONA-MODE-ROLEDEFINITION-4MRMC
```

**Why**, read out of the 2.0.5 bundle (`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js`,
`perl -ne` over the one long line, as ranger-base-f1ytb §1 did):

| fact | evidence |
|---|---|
| the YAML `hidden` is parsed and carried | the modes-file mapper builds `{id,name,description,roleDefinition,…,hidden:f.hidden,configPath,workspaceRoot,workspaceName}` |
| nothing reads it | `modeManager.getModes(ws,all)` returns global + workspace rows with no predicate but path scope; `runtime.getModes(e)` merges the provider rows, filters disabled tool GROUPS and dedupes ids — no `hidden` anywhere |
| both consumers take that list | `listModes()` = `runtime.getModes({workspace})`; the Shift+Tab cycler (`…listModes().then(t=>{… t[(a+1)%t.length] …})`) and the picker (`listModes().then(w=>o(w))` into a component that maps every row) |
| the field that DOES hide is a different one | `isModeEnabled(t,n)` = `!(!entryIsEnabled(slug) || t.hiddenFromUser && !n || …)` — `hiddenFromUser` on a *provider/builtin* registry entry, which a workspace modes file never becomes |
| there is no "loaded but hidden" state | the only switch that drops a workspace row is trust: `!workspaceTrusted` -> `.filter(r=>r.scope!=="workspace")`, i.e. the same switch that stops the mode being SELECTED. Read, not run. |

So the schema accepted a key no consumer reads: ranger-base-5jjtn's
`.allowUnknownOption()` class a third time — accepted and ignored looks exactly
like delivered — in the YAML schema rather than in argv.

## 3. Where the file lands is `default_dir`, and on this box that is the home

Found while reading the write path for §2's consequences, and the sharper finding of
the two (ranger-base-se81d F2). `posse new` never asks for a worktree —
`cmd/posse/main.go:2123` builds `NewSessionOpts{Name: args[0]}` and the command has
no worktree option at all; only `dispatch.go:4506`, `dispatch.go:4591` and
`relaunch.go:345` set `Worktree`. `herdrback.go:1971-1990` then takes
`dir := o.Dir`, else `CfgGet("default_dir", $HOME)`, and builds a session tree only
`if o.Worktree`. This box's `~/.config/posse/config.yaml:4` reads `default_dir: ~`.

posse's own arithmetic on that dir, MEASURED off `PersonaModeFile` (no write):

```
rt.PersonaModeFile("$HOME")  ->  $HOME/.bob/plugins/posse/custom_modes.yaml
```

which is bob's **global** modes glob — `{settings/custom_modes.yaml,
plugins/*/custom_modes.yaml}`, depth 2, under the home (ranger-base-f1ytb §1,
MEASURED by reading) — loaded by every bob session on the box in every workspace,
returned even for an untrusted one (`getModes` drops only `scope!=="workspace"`
there), listed and readable per §2, and removed by nothing: the channel's only
readers in the tree are the three writers, `{mode}`, the footer read and the trust
exemption, and no kill or reap path touches the file.

That is the design ADR 0062 priced and rejected (`~/.bob/plugins/posse-<persona>/
custom_modes.yaml`: "a write under the operator's CLI home", "stale across persona
deletion", "visible to the operator's own Bob sessions in every workspace"), and its
Consequences say "no write under the home" — reached by accident through a config
default.

**NOT RUN, on purpose**: the end-to-end write is a write under the operator's CLI
home, which is the operator's to authorize and not QA's. Everything above is posse's
own path function or the shipped bundle. The repro for whoever owns the fix is one
command — `posse new --runtime bob -a <persona>` with no `--dir`, then
`ls ~/.bob/plugins/posse/`. `~/.bob/plugins` did not exist on 2026-10-03, so nothing
has landed there yet. The same `dir` default already landed the sibling tree:
`~/.agents/skills/` exists, created 2026-09-05 (ADR 0007, the tree personamode.go
names as its model). The mode file is worse than its precedent rather than equal to
it, because `.agents/skills` in the home is read by a session whose cwd is the home
and `.bob/plugins/posse` in the home is read by all of them.

## 4. What the no-spend half narrowed the two billed claims to

Refused here on the money line and asked of the operator on ranger-base-x5xt6 (the
shape is ranger-base-ff9pz's, one runtime over: the ruling has to name the executor,
the budget line and the cap).

- **ASSUMED 1** (`roleDefinition` reaches the model as the persona) is now two
  halves, and the structural one is MEASURED: bob parsed posse's file and holds the
  exact PID bytes as this session's `roleDefinition` — the picker printed them back
  (§2) — and the bundle says that field IS the `role_definition` section of the
  system prompt (f1ytb §1). What no local artifact answers is whether the model
  ANSWERS as the persona. bob writes no pre-turn prompt dump: a `--log-level debug`
  session log (`~/.bob/logs/shell/bob-shell-*.log`, 9.4 kB for this pane) carries
  workspace, TreeSitter, MCP and gateway lines and no prompt, so there is no codex
  `debug prompt-input` equivalent to spend nothing on. One turn.
- **ASSUMED 3** (rules_precedence vs the workspace's AGENTS.md) is untouched by
  anything here; `runtimes/bob.yaml`'s value stays UNMEASURED and must be promoted
  from a measurement, never from plausibility.
- Both can be ONE turn if the fixture contradicts on case, an end token and the word
  "ready" (ranger-base-6rcv's) while saying nothing about identity, and the prompt is
  `Who are you? Are you ready?`. And the cap can be enforced rather than promised:
  `bob chat` takes `--max-turns <n>` and `--max-cost <number>` on 2.0.5 (`--help`,
  MEASURED today) — which is the one thing ff9pz had to take on trust.
