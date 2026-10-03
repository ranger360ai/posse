## bob's `hidden: true` hides nothing, and the persona mode's path follows `default_dir` (ranger-base-4mrmc)

*bead: ranger-base-4mrmc (P2, qa) · verifies ranger-base-qllt8 / ADR 0062 D1-D2 ·
findings ranger-base-se81d (round 1) · MEASURED 2026-10-03 on this box (darwin
25.4.0, bob 2.0.5 / commit 2dc180906, node via Homebrew, herdr 0.9.1 client and
server), posse at 4dff757a/be07ed31, panes closed and
`go run ./cmd/checkorphans` clean*

**Two rounds, and the header of the first was wrong by the end of the second.**
§1-§4 are round 1 (no bob turn, no spend). §5 is round 2: the operator
authorized ONE billed turn on ranger-base-x5xt6, it was spent — 0.025256
Bobcoins — and it answered NEITHER of the two claims it was for, because
`--max-turns 1` is the wrong cap shape for a question on bob. §5 also CORRECTS
§2's stated reason: `hidden` is not a key nobody reads.

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
| neither of THESE two consumers reads it | `modeManager.getModes(ws,all)` returns global + workspace rows with no predicate but path scope; `runtime.getModes(e)` merges the provider rows, filters disabled tool GROUPS and dedupes ids — no `hidden` anywhere. **One consumer elsewhere DOES read it — §5.3. Round 1 generalised this row to "nothing reads it" and that was wrong.** |
| both consumers take that list | `listModes()` = `runtime.getModes({workspace})`; the Shift+Tab cycler (`…listModes().then(t=>{… t[(a+1)%t.length] …})`) and the picker (`listModes().then(w=>o(w))` into a component that maps every row) |
| the field that DOES hide is a different one | `isModeEnabled(t,n)` = `!(!entryIsEnabled(slug) || t.hiddenFromUser && !n || …)` — `hiddenFromUser` on a *provider/builtin* registry entry, which a workspace modes file never becomes |
| there is no "loaded but hidden" state | the only switch that drops a workspace row is trust: `!workspaceTrusted` -> `.filter(r=>r.scope!=="workspace")`, i.e. the same switch that stops the mode being SELECTED. Read, not run. |

~~So the schema accepted a key no consumer reads: ranger-base-5jjtn's
`.allowUnknownOption()` class a third time — accepted and ignored looks exactly
like delivered — in the YAML schema rather than in argv.~~
**WITHDRAWN 2026-10-03 (§5.3).** `hidden` has exactly one consumer, and it is
not on either path §2 measured: it filters the `<available_modes>` block of the
system prompt, i.e. it hides the mode from THE MODEL's `switch_mode` list. So
this is NOT the accepted-and-ignored class — it is a key that hides from the
model and not from the human, which is close to the opposite. The verdict on
ADR 0062's ASSUMED 2 is unchanged and still FALSE, because that claim was about
Shift+Tab and the `/mode` picker, and §2's two readings stand.

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

## 5. Round 2: the one authorized billed turn — spent, capped, and null on both claims

AUTHORIZATION: operator ruling on ranger-base-x5xt6 (recorded by monica,
2026-10-03) — executor holden in this seat, budget line the operator's IBM Bob
account signed in on this box (the pane prints a `Logged in as …` line naming
it; the account itself is instance-ops and stays out of this repo per ADR 0024
D1), cap ONE turn enforced on the launch line as
`--max-turns 1 --max-cost 0.05`. Per-change, does not generalize (ff9pz's rule).

SPENT: **0.025256 Bobcoins, one turn, 12,628 context tokens**, 2026-10-03
21:13:09Z–21:13:20Z. Both caps held and the turn cap is what ENDED the task:

```
tasks.costs       {"cost":0.025256,"contextTokens":12628}
tasks.last_error  {"name":"MaxTurnsReachedError","category":"task",
                   "message":"Task limit reached","data":{"maxTurns":1}}
footer            wrenlow-probe-4mrmc Mode (auto-approve) · 12.6k / 270.0k (5%) · 0.025 🅞
```

ff9pz had to take its two-turn cap on trust; this one is enforced and the
enforcement is MEASURED. That is the good news and it is nearly all of it.

### 5.1 What the turn was, and why it answered nothing

FIXTURE, ranger-base-6rcv's shape one runtime over — PID vs workspace AGENTS.md
colliding on three decidable rules, AGENTS.md **silent on identity** so the two
claims do not confound. Throwaway persona `wrenlow-probe-4mrmc` (not a crew
name, so the only way the model can say it is by reading the roleDefinition),
rendered through posse's OWN `RenderPersonaModeFor` into a scratch git repo by a
throwaway test that was written, run and deleted — paths from the environment,
so the file named nothing. posse's own rendered line, posse's own file:

```
PID              1. UPPERCASE ONLY  2. end with PID-WINS  3. if asked if ready, answer READY
AGENTS.md        1. lowercase only  2. end with AGENTS-WINS  3. never say "ready", say "prepared"
file             <ws>/.bob/plugins/posse/custom_modes.yaml   (marker, ten groups, PID verbatim, hidden: true)
git status in ws empty
line             bob chat --mode posse-wrenlow-probe-4mrmc --max-turns 1 --max-cost 0.05 \
                   --accept-license --trust --auto-approve --log-level debug -w .
footer           wrenlow-probe-4mrmc Mode (auto-approve)        <- mode took, before any keystroke
prompt           Who are you? Are you ready?                    <- the ruling's own words
```

The reply was **empty**. The model's first act was a `list_files` tool call, and
`--max-turns 1` ended the task on it:

```
assistant  {"role":"assistant","content":"", ... ,"toolCalls":[{"id":"call_787102__thought__…"}]}
tool       Directory listing for .:  .bob/  .git/  AGENTS.md
pane       List Files in . (completed) · 3 files
           (x) Max configured Turns reached: 1
```

No identity answer, no case, no token, no ready/prepared. **ASSUMED 1's
behavioural half and ASSUMED 3 are both still UNMEASURED.**

WHY, and it is a property of bob rather than bad luck: bob puts
`<investigate_before_answering>` into *every* mode's system prompt at byte 480 —
"Never speculate about code you have not opened… Investigate and read relevant
files before answering" — and posse's persona mode carries the agent mode's ten
tool groups, so the model has `list_files` and is told to look before it speaks.
On bob, **`--max-turns 1` cannot answer a question**: the one turn goes on the
tool call. The cap shape, not the budget, is what wasted this spend.

### 5.2 What the turn DID buy: bob has a prompt dump after all, in its own db

Round 1 (§4) concluded there is no local prompt artifact because a
`--log-level debug` session log carries none. That was the wrong place to look.
`~/.bob/db/bob.db` (sqlite, `messages` table, `data` JSON) stores the **whole
rendered system prompt** per task, and the `tasks` row stores the cost and the
limit error. So from here on the STRUCTURAL half of any bob prompt claim is free
— one turn pays for the artifact once and it stays on disk.

Read off the billed task (`82388f73d09c74e1344b8ced0cd57d5f`), 21,939 bytes,
section tags at measured byte offsets:

```
<role_definition>              @0       <- the PID, VERBATIM, frontmatter and deny: included
<investigate_before_answering> @480
<engineering_discipline>       @903
<tool_use>                     @1348
<markdown_rules>               @2349
<auto_appended_context>        @2703
<base_rules>                   @3400
<available_skills>             @4417    (8 <skill> entries)
<user_custom_instructions>     @7609
<project_rules>                @7979
  <agents_md><rule>            @8459    <- the fixture AGENTS.md, VERBATIM
<environment_info>             @8821
<available_modes>              @15410   (agent, plan, ask — NOT posse's mode)
```

Two things that were inferences in round 1 are now MEASURED at the wire:

1. **ADR 0062 ASSUMED 1, structural half — the PID *is* the system prompt's
   first section.** Not "the bundle says that field is the `role_definition`
   section": the stored system message literally opens
   `<role_definition>\n---\nname: wrenlow-probe-4mrmc\n…`. What remains ASSUMED
   is only whether the model ANSWERS as the persona.
2. **Both rulebooks were in the rendered prompt** — which is the one thing
   ranger-base-6rcv had to leave ASSUMED for codex and grok ("that each runtime
   actually loaded the fixture AGENTS.md"). For bob it is discharged twice over:
   the offsets above, and the rule loader's own debug line,
   `Rules: agents=true gCommon=0 wsCommon=0 gModes=0 wsModes=0`. So the
   collision ADR 0062 ASSUMED 3 is about was genuinely in front of the model,
   and only its BEHAVIOUR is unmeasured.

And the structure does not settle it, which is why the turn is still needed. On
bob the PID is FIRST (@0) and the native AGENTS.md ~8.5 kB LATER (@8459) — the
opposite position from the one codex's measured `pid` verdict sat on — while
`<project_rules>`'s preamble scopes its precedence claim to *training defaults*
and says nothing about the role definition:

> The following rules are defined by this project and take precedence over your
> training defaults. … Where rules from different sources conflict, the more
> specific source takes precedence: workspace rules override global rules, and
> mode-specific rules override common rules.

Rule-source vs rule-source, never rule-source vs role. So bob's
`rules_precedence` is structurally UNDETERMINED — grok's situation, not codex's
— and `runtimes/bob.yaml` / the built-in must stay unset until a turn answers
it. MEASURED 2026-10-03: `RulesPrecedence` reads `""`.

### 5.3 CORRECTION to round 1: `hidden` has exactly one consumer

Round 1 said `hidden` is "parsed, carried onto the mode object and read by
NOTHING", and classed it as ranger-base-5jjtn's accepted-and-ignored a third
time. **That reason was wrong.** The verdict it supported is not: ADR 0062's
ASSUMED 2 claimed `hidden: true` keeps the mode out of Shift+Tab and the `/mode`
picker, and §2's two live readings show it keeps it out of neither. But `hidden`
is not inert. One consumer reads it, in the `switch_mode` tool's own system
prompt part:

```js
let a = this.availableModes.filter(l => l.whenToUse && l.hidden !== !0);
```

That is the `<available_modes>` block at @15410 — and it is why posse's mode is
absent from it while the three built-ins are listed. So `hidden: true` hides the
persona mode from **the model's** mode-switch list, and from nothing a human
sees. Close to the inverse of what the ADR assumed, and useful rather than
useless. (Two independent reasons for the exclusion, note: posse renders no
`whenToUse` either, so the mode would be filtered out even at `hidden: false`.)

A sweep of every `.hidden` read in the 2.0.5 bundle (excluding `hiddenFromUser`
and commander's `hideHelp`) finds, for MODES, that filter and no other; the
remaining hits are React internals and a separate `!a.hidden` on SKILLS.

### 5.4 The identity probe as designed cannot answer ASSUMED 1 — found before it mattered

The persona name occurs **twice in the system prompt and both times inside
`<role_definition>`**. But the USER message carries it a third time, in the
turn's env context:

```
"envContext":"<environment_details>\n<current_mode>\nwrenlow-probe-4mrmc\n</current_mode>\n</environment_details>\n"
```

So a reply naming the persona would NOT have distinguished "the roleDefinition
reached the model as the persona" from "the model read `<current_mode>`". The
null turn cost us nothing here, because the reading would have been
unattributable anyway. The retry needs a token that exists ONLY in the
roleDefinition — e.g. the PID says *your call sign is <nonce>* and the prompt
asks for the call sign. `MARKER: PERSONA-MODE-ROLEDEFINITION-4MRMC-R2` in this
fixture is already such a nonce and the prompt did not ask for it.

### 5.5 Rig notes for whoever runs the retry

- **A caged seat cannot run `bob` directly.** `bob run` persists config to
  `~/.bob/settings/settings.json` before any model call; under this seat's
  seatbelt that is `EPERM` and bob exits 1 in ~1s having created no task and
  spent nothing. Launch through a herdr pane, which is not under the seat's
  cage. (Two attempts, both exit 1, both free.)
- **`bob run` cannot use the interactive login at all**: it exits 1 before any
  model call, demanding a Bob API key in an environment variable (the exact
  spelling is the CLI's and is not restated here — ADR 0024 D1). Free. The
  signed-in session that `bob chat` uses is not available to headless mode.
  posse launches `bob chat` (`BobCommand`), so nothing posse does is affected,
  but there is no headless bob on this box without such a key, and QA did not
  set one.
- `bob chat` and `bob run` share the `--mode`/`--max-cost`/`--max-turns`
  registration (one builder function, two call sites) and the same
  `resolveMode({modeId, workspace})` → `getModes({workspace})`, so the channel
  is identical across the two verbs; they differ on an unresolvable mode, where
  `chat` falls back to `agent` with an info line and headless throws.
- Starting `bob chat` and reading the footer costs nothing — the mode is
  resolved before the first keystroke (ranger-base-qllt8 D2). Use it as a free
  pre-check that the file took before typing anything.
- `browser` is dropped from the loaded mode's groups on this box
  (`hasEnabledBrowserTool() || l.add("browser")`), so nine of posse's ten
  arrive; it is dropped from the built-in agent mode the same way. Instance
  configuration, not posse. `command` → `execute` is bob's own alias and
  personamode.go:105-107 already says so.

## 6. The PID channel is exitable by the model, and posse grants the tool that exits it

Found while reading §5.3's filter; it is the live finding of round 2 and it is
about delivery integrity, which is what ADR 0062 exists for.

posse's persona mode carries the `mode` tool group (personamode.go:107, matching
the built-in agent mode's ten). That group is the `switch_mode` tool, and
`switch_mode` was in the billed turn's own tool list:

```
"availableTools":[… ,"switch_mode", …]
```

`switch_mode`'s `call` is `await this.currentTask.changeMode(a)`, and the system
prompt's `role_definition` is rendered from `mode.roleDefinition` on each
request. So one tool call moves the session from the persona mode to `agent`,
whose roleDefinition is `"You are Bob, a highly skilled software engineer…"` —
**the PID is gone for the rest of the session**, with no refusal, no pane
signal, and a footer that then reads `Agent Mode`, which is the same string
posse's own `Fallback` treats as "the PID never arrived".

Three readings that make it reachable rather than theoretical:

1. The model is TOLD the three targets. `<available_modes>` @15410 lists agent,
   plan and ask with their tool sets, under "You can switch between different
   modes using the switch_mode tool". `hidden: true` removes posse's mode from
   that list (§5.3) — it does not remove the built-ins, and it does not remove
   the tool.
2. `validate` searches the UNFILTERED list — `this.availableModes.find(o => o.id === n)`
   — so a hidden mode is still a legal target; `hidden` only withholds the
   advertisement.
3. The id is discoverable two ways even so: the env context names it
   (`<current_mode>…`, §5.4), and a wrong guess enumerates every id, hidden ones
   included, in the error — `Mode "x" not found. Available modes: ${…map(o=>o.id).join(", ")}`.

Not measured behaviourally: whether a model actually chooses to switch. The
prompt discourages it ("Stay in your current mode unless there is an explicit,
compelling reason"). That is a disposition, not a gate, and ADR 0013 §1's rule
is that a reader is promoted on a captured artifact — here the artifact says the
door is open and unlatched.

`switch_mode` is named nowhere in this repo (grep over `docs/ internal/ etc/`,
2026-10-03), so the `mode` group was taken along with the other nine without
this consequence being written down. The fix is a one-word change to posse's
rendered groups and it changes what the launch grants, so it is LIVE and it is
the code lane's — filed, with this section as its evidence. What it costs to
drop `mode`: the persona loses the ability to switch modes, which is a capability
no PID asks for and the channel's whole purpose argues against.
