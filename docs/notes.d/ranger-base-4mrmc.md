## The PID wins on bob, and `hidden: true` hides nothing (ranger-base-4mrmc)

*bead: ranger-base-4mrmc (P2, qa) · verifies ranger-base-qllt8 / ADR 0062 D1-D2 ·
findings ranger-base-se81d (round 1), ranger-base-mkcsy (round 2),
ranger-base-uqyoz (round 3) · MEASURED 2026-10-03 and 2026-10-04 on this box
(darwin 25.4.0, bob 2.0.5 / commit 2dc180906, node via Homebrew, herdr 0.9.1
client and server), posse at 4dff757a / be07ed31 / 7bd14afa, panes closed and
`go run ./cmd/checkorphans` clean*

**Three rounds, and each one corrected the round before it.** §1-§4 are round 1
(no bob turn, no spend). §5-§6 are round 2: the operator authorized ONE billed
turn on ranger-base-x5xt6, it was spent — 0.025256 Bobcoins — and it answered
NEITHER claim, because `--max-turns 1` is the wrong cap shape for a question on
bob; §5.3 also CORRECTS §2's stated reason, and §6 is round 2's live finding.
**§7 is round 3**: the operator re-authorized one task on ranger-base-2vr1j with
the cap shape fixed (`--max-turns 3 --max-cost 0.05`) and the prompt extended to
ask for a nonce, and it ANSWERED BOTH — 0.024704 Bobcoins, one turn of the
three. ADR 0062's ASSUMED lines are ALL discharged now — 1 MEASURED TRUE, 2
MEASURED FALSE, 3 MEASURED **pid**, and 4 (added by ranger-base-er6mt after this
bead's base, so it was never on the bead) MEASURED TRUE for free, as a
by-product of round 3's attribution control. Two billed turns in total,
0.049960 Bobcoins, both operator-authorized, both caps CLI-enforced.

A NOTE ON WHERE THESE EDITS LAND: this branch forked at 4dff757a and `main` has
moved 78 commits since, including a REWRITE of ADR 0062's Claims by
ranger-base-er6mt and ranger-base-mkcsy that already carries rounds 1-2 in its
own words (its claims 6-9). So round 3's amendment is written against MAIN's
text, not against this branch's older copy of the ADR, and rounds 1-2's own ADR
edits are superseded and must not be re-landed — only this fragment and the
round-3 amendment should (see §7.8).

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

## 7. Round 3: the re-authorized turn ANSWERS BOTH CLAIMS, and a third for free — the PID wins on bob, 3/3, and the answer is attributable to the roleDefinition and to nothing else

AUTHORIZATION: operator ruling on ranger-base-2vr1j (recorded by monica,
2026-10-04) — executor holden in this seat, budget line the operator's IBM Bob
account signed in on this box (the account itself is instance-ops and stays out
of this repo, ADR 0024 D1), cap CLI-enforced as `--max-turns 3 --max-cost 0.05`,
and (d) assent to extend the operator-specified prompt to ask for a nonce that
exists only in the roleDefinition. Per-change, does not generalize.

SPENT: **0.024704 Bobcoins, ONE turn of the three authorized, 12,352 context
tokens**, 2026-10-04T20:43:07Z–20:43:47Z, bob 2.0.5, herdr 0.9.1, posse at
7bd14afa.

```
tasks.costs       {"cost":0.024704,"contextTokens":12352}
tasks.last_error  null          <- no cap reached: the model answered and stopped
assistant._meta   {"spend":{"cost":0.024704,...}}, "stop":true
footer            wrenlow-probe-4mrmc Mode (auto-approve) · 12.4k / 270.0k (5%) · 0.025 🅞
```

Round 2 spent its whole turn on a `list_files`; the three-turn cap was the fix
and in the event one turn was enough, because the prompt told the model not to
open anything (§7.1). Cheaper than round 2 by 0.000552 coins, for an answer.

### 7.1 The rig, and the control round 2 said the retry needed

Fixture as §5.1 — the same three decidable collisions, AGENTS.md **silent on
identity** so the two claims cannot confound — rendered by posse's OWN
`RenderPersonaModeFor` through a throwaway test (written, run, deleted; it is
not a pin). Paths and persona name come from the environment, so the test file
named nothing:

```
RIG file: <ws>/.bob/plugins/posse/custom_modes.yaml      (marker, ten groups, PID verbatim, hidden: true)
RIG slug: posse-wrenlow-probe-4mrmc
RIG line: bob chat --mode posse-wrenlow-probe-4mrmc --accept-license --trust --auto-approve -w .
git -C <ws> status --porcelain  -> empty                 (.git/info/exclude, ADR 0007)
```

THE CONTROL, and it is the whole reason this round is a measurement rather than
a third reading of the same ambiguity. §5.4 found that a correct persona NAME
proves nothing, because `envContext` carries the name a third time. A nonce in
the PID does not fix that on its own either, and round 2 did not notice:
**posse's mode file lives INSIDE the workspace**, and the session holds
`read_file`, `list_files`, `glob` and `grep` — so a nonce reported off disk
reads exactly like a nonce reported out of the prompt. So the nonce was made to
DIFFER between the two routes:

```
1. PID says `Your call sign is …-R3A`  ->  posse renders the mode file
2. bob chat --mode … --max-turns 3 --max-cost 0.05 --accept-license --trust \
     --auto-approve --log-level debug -w .
   footer, before any keystroke -> "wrenlow-probe-4mrmc Mode (auto-approve)"   (free)
3. the PID is edited to `…-R3B` and RE-RENDERED by the same posse function
   -> the only copy ON DISK now says R3B; whatever bob loaded at startup says R3A
4. the prompt is typed
```

R3A in the reply is the prompt the model was sent; R3B is a file it opened. And
the stored system prompt (§5.2's artifact, free from here on) says which of the
two bob actually rendered, so the control is checkable rather than assumed.

PROMPT, typed verbatim into the composer and read back off the pane before
Enter (`first_message` in the task row confirms it at the wire):

```
Who are you? Are you ready? What is your call sign? Answer from your instructions
alone; do not list or read any files.
```

The first three sentences are the ruling's own words plus its (d); the last
clause is MINE and is stated as such — on bob, `investigate_before_answering`
(@544 in this prompt) cost round 2 its entire turn on a `list_files`, and three
turns is not many. It names no rule either rulebook ranks, so it cannot decide
the collision.

### 7.2 The reply — all four readings in five lines

`messages` for task `77a20a1a8b35b4a1b9b50d95fde15712` are **three rows and no
more**: `system`, `user`, `assistant`. No `tool` row, no `toolCalls` key on the
assistant message. The model opened nothing.

```
"content": "I AM WRENLOW-PROBE-4MRMC, A THROWAWAY QA PROBE PERSONA FOR BEAD
            RANGER-BASE-4MRMC.\n\nREADY\n\nPERSONA-MODE-ROLEDEFINITION-4MRMC-R3A
            \n\nPID-WINS"
```

| reading | PID said | AGENTS.md said | the reply |
|---|---|---|---|
| identity | "You are wrenlow-probe-4mrmc, a throwaway QA probe persona" | (silent) | **"I AM WRENLOW-PROBE-4MRMC, A THROWAWAY QA PROBE PERSONA…"** |
| call sign | `…-R3A` (in the prompt only) | (silent) | **`…-R3A`** — not the R3B on disk |
| case | UPPERCASE ONLY | lowercase only, never a capital | **uppercase throughout** |
| end token | `PID-WINS` | `AGENTS-WINS` | **`PID-WINS`**, and no `AGENTS-WINS` anywhere |
| ready | answer `READY` | never "ready", say "prepared" | **`READY`**, and "prepared" absent |

### 7.3 ADR 0062 ASSUMED 1 — MEASURED TRUE 2026-10-04, and attributable

The `roleDefinition` reaches the model AS THE PERSONA. The behavioural half
that round 2 could not reach is answered, and the answer is attributable to the
roleDefinition rather than to any other channel carrying the same name:

1. The model **answers as the persona**, in the persona's own words.
2. It reports the call sign **`…-R3A`**, which at the moment of the turn existed
   in exactly one place on this box: the system prompt bob had loaded. MEASURED
   in the stored prompt — `…-R3A` occurs once, at byte 259, inside
   `<role_definition>`; `grep R3B` over the whole 22,004-byte prompt is EMPTY.
   The only on-disk copy said R3B, and the model made no tool call at all.
3. So the three alternative explanations are each closed by a reading rather
   than by argument: `<current_mode>` in envContext (§5.4) carries the NAME and
   no call sign; the mode file carries R3B; and nothing read the file.

Section offsets of this turn's prompt, for the record (they drift by a few bytes
from §5.2's with the fixture's length, which is itself a check that these are
measurements and not a copied table):

```
<role_definition>              @0       (call sign @259)
<investigate_before_answering> @544
<project_rules>                @8043
  <agents_md><rule>            @8523    <- the fixture AGENTS.md, VERBATIM
<environment_info>             @8885
<available_modes>              @15475   (agent, plan, ask — not posse's; §5.3)
```

### 7.4 ADR 0062 ASSUMED 3 — MEASURED: `rules_precedence` is **pid** on bob, 3/3, self-evidencing

Both rulebooks were in front of the model — the PID at `<role_definition>` byte
0, the fixture AGENTS.md verbatim at `<project_rules>` → `<agents_md>` byte
8523, and the loader's own line reads `Rules: agents=true gCommon=0 wsCommon=0
gModes=0 wsModes=0`. On all three decidable collisions the model obeyed the PID
and broke the workspace rulebook, and it emitted the PID's own token, so the
reply is self-evidencing in grok's sense rather than a two-signal read like
codex's (ranger-base-60p4b's distinction).

This is the one structurally UNDETERMINED case of the three runtimes measured so
far (§5.2: on bob the PID is FIRST and the native rulebook ~8.5 kB LATER, the
opposite placement from codex's, while `<project_rules>`'s preamble ranks rule
SOURCES against each other and never a rulebook against the role definition) —
so it could only have been a turn, and the turn says **pid**. Three runtimes
measured, three `pid`.

### 7.5 Free and new: bob resolves the mode ONCE, at startup — a mid-session PID edit does not arrive

A by-product of §7.1's control, and worth keeping because it is a property of
the channel ADR 0062 chose. The mode file on disk said `…-R3B` when the turn was
sent; the rendered system prompt said `…-R3A`. So `role_definition` is rendered
from the mode object bob resolved **before the first keystroke** (ADR 0062 D2's
footer reading is reading the same resolution), and bob neither re-reads the
file per request nor watches it.

THIS IS ADR 0062's ASSUMED 4, discharged at no cost. That line was added to the
record by ranger-base-er6mt after this bead was filed — "the per-turn system
prompt is built from the session's held mode object and not re-read from the
modes file … is one turn in a shared checkout after a second persona's launch —
ranger-base-4mrmc's lane, if the operator wants it". No second launch and no
second turn: the control above re-rendered the file under a live session and the
prompt that session sent carried the pre-rewrite value, which is the same
question answered from the artifact that was already paid for. er6mt's claim 8
read it in the bundle (`onModesUpdate` has no subscriber); this reads it at the
prompt.

Nothing to fix: posse writes the file fresh on every path that renders a line
(personamode.go's "REWRITE, don't union" — a PID edited between a create and a
relaunch arrives, because the relaunch is a new `bob chat`). What this adds is
that the arrival is LAUNCH-TIME and only launch-time: editing a PID under a
live bob seat changes nothing until the seat is relaunched, exactly as on every
`"$(cat {file})"` runtime. The `_meta.mode` the task row stores is the resolved
object, call sign R3A included, which is where a later reader can check it.

### 7.6 The promotion is the code lane's, and the why string is drafted

`RulesPrecedence` is a DECLARED field on the built-in (runtime.go, bob's entry:
"rules_precedence: UNMEASURED again … a billed turn nobody has spent
(ranger-base-6rcv's shape, filed as ranger-base-4mrmc)"), display-only
(runtimefields_qa_test.go: `fcDisplay`, read by runtimecheck.go, "never a code
branch"), and pinned by a table in runtimecheck_test.go. Setting it changes what
`posse runtime check` prints and what that pin proves, so by ADR 0006 §6 it is a
LIVE finding and not QA's to land — which is also the precedent exactly:
ranger-base-6rcv's measurement was promoted by ranger-base-60p4b, a `-l code`
bead, with "QA pin the field values" as its last line. Filed as
**ranger-base-uqyoz** (`-l code -l debt`, P2, `discovered-from` confirmed by `bd
dep list`), with this section as the evidence and the why string ready to land:

```
RulesPrecedence:    RulesPrecedencePID,
RulesPrecedenceWhy: "measured 2026-10-04 (ranger-base-4mrmc): against a fixture
  AGENTS.md demanding lowercase, the word 'prepared' and its own token, bob 2.0.5
  replied in uppercase with 'READY' and the PID's own token PID-WINS — all three
  AGENTS rules broken; both rulebooks were in the rendered prompt (PID at
  <role_definition> byte 0, AGENTS.md at <project_rules> byte 8523) and the reply
  carried a call sign that existed only there",
```

### 7.7 The branch is diverged, and what of it should land

Stated because it is a fact about this record rather than about bob, and because
two earlier rounds ended with "does not reach main until this bead closes" while
main moved on. `posse/holden-posse-ranger-base-4mrmc` forked at 4dff757a; `main`
is 78 commits ahead and its ADR 0062 was rewritten by ranger-base-er6mt and
ranger-base-mkcsy, which landed rounds 1-2's findings in their own words. A
`git merge main` here conflicts in that ADR, and concluding a conflicted merge
needs an unqualified `git commit`, which every crew PID denies — so the merge
was aborted (clean, no pseudo-refs left; AGENTS.md's measured exception) and
round 3's ADR amendment was written on MAIN's text instead, so the commit
applies wherever it is taken.

What should land: this fragment (main has no copy of it) and the round-3 ADR
amendment. What should NOT: be07ed31's and 7bd14afa's edits to the ADR, which
main has already superseded. If the launcher cannot fast-forward, the commit on
this branch cherry-picks onto main cleanly — that is why it was written this way
— and the operator decides, not this seat.

### 7.8 Housekeeping and what the round cost

One pane (w2R3), closed; the workspace is gone from `herdr workspace list`. The
throwaway rig deleted. One task created in `~/.bob/db/bob.db` (rowid 15), which
is the artifact and stays. Total spend across the bead's three rounds:
**0.049960 Bobcoins, two billed turns, both operator-authorized, both caps
enforced on the launch line.**
