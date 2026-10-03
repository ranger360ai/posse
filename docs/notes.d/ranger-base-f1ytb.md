## Bob's PID channel is a workspace custom mode, and an unknown mode slug falls back in silence (ranger-base-f1ytb)

*bead: ranger-base-f1ytb (P1, architecture) · from ranger-base-5jjtn · record:
ADR 0062 · measurements below are MEASURED 2026-10-03 on this box (darwin
25.4.0, bob 2.0.5 / node 25.2.1 from Homebrew, herdr 0.9.1 client and server)
unless marked ASSUMED*

## 1. What the bundle says (read, not run)

`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js`, 15,485,333 bytes,
grepped with `perl -ne` because one 15 MB line exceeds ugrep's pattern limits.

| fact | evidence |
|---|---|
| modes file name | `Bhe="custom_modes.yaml"` |
| workspace glob | `Odr="{custom_modes.yaml,plugins/*/custom_modes.yaml}"`, joined under `wA=".bob"` of each workspace dir (`_workspaceBobDirs`) |
| global glob | `LJo="{settings/custom_modes.yaml,plugins/*/custom_modes.yaml}"`, `depth:2`, under `_globalDirectory` |
| mode selection | `resolveMode(e)`: `modeId ?? session.defaultMode`, `find(id===t)`, else `throw new Error('Mode with id "…" not found')` — the chat startup catches it (see §2, it prints and falls back) |
| roleDefinition | the default prompt config's first section: `{id:"roleDefinition",tag:"role_definition",template:"{{ roleDefinition }}"}` — the system prompt's role, not a user turn |
| slug | `ke.string().regex(/^[a-zA-Z0-9-]+$/)` |
| groups default | `for(let m of l.groups||[])` → an omitted `groups` is `[]`; `"command"` maps to `"execute"` |
| agent mode's groups | `["read","edit","execute","browser","mcp","skill","todo","artifact","subagent","mode"]` |
| `hidden` | a field of the mode schema (`hidden:l.hidden`); its picker consumer was not found — ASSUMED it hides the mode from Shift+Tab and `/mode` |
| rules dirs | `readRuleFiles(t,n)` reads `dirname(modesFile)/rules-<slug>/*` — so a workspace mode may carry `.bob/plugins/posse/rules-posse-<x>/` too |
| env | nine `process.env.BOB_*` reads in the bundle: the API-key name the built-in spells in Go (`BobCageCred`, kept out of markdown on purpose), two more key-shaped siblings, and `BOB_EXTENSIONS`, `BOB_GATEWAY_URL`, `BOB_LOG_LEVEL`, `BOB_SHELL_TEMP_VERSION`, `BOB_SUPERVISED`, `BOB_WEB_LOGIN_URL` — nothing relocates the home or names a modes file |

`bob chat --help` (2.0.5) lists `--mode <mode>  Mode to use (built-in or
custom mode slug) (default: "agent")`. ADR 0060 read the same help screen and
priced only the global file.

## 2. Two scratch panes, no turn, no spend

Mode file written to `<scratch>/ws/.bob/plugins/posse/custom_modes.yaml`:

```yaml
customModes:
  - slug: posse-f1ytb
    name: Posse f1ytb probe
    description: posse ranger-base-f1ytb no-spend probe; delete after
    roleDefinition: |
      You are the ranger-base-f1ytb probe persona. If asked, reply with the single word PROBE.
    groups: [read, edit, command, browser, mcp, skill, todo, artifact, subagent, mode]
    hidden: true
```

> Annotated 2026-10-03 (ranger-base-mkcsy): the YAML above is what this probe
> ran, verbatim, and stays that way. **It is not the shipped list any more.**
> `mode` is the `switch_mode` tool, so a persona mode declaring it lets the
> model swap the PID for bob's built-in role mid-session; posse ships the
> nine without it (ADR 0062 D1.1 as amended, claim 9). Copy the list from
> `bobPersonaMode.Groups`, not from here.

```
herdr workspace create --label posse-f1ytb-probe-a --no-focus --cwd <scratch>/ws   → w2MZ:p1
herdr pane run w2MZ:p1 "bob chat --accept-license --trust --auto-approve -w . --mode posse-f1ytb"
herdr pane wait-output --regex 'Build Anything|Mode|not found|Error|error' --timeout 40000 w2MZ:p1
   → matched "❯   Build Anything, @ for context, / for commands, $ for skills"
herdr pane read --source visible --format text w2MZ:p1 | tail
   → "(ℹ) Logged in as …"
     "❯   Build Anything, @ for context, / for commands, $ for skills"
     "Posse f1ytb probe Mode (auto-approve)"                              ← the mode took
herdr pane process-info --pane w2MZ:p1
   → shell_pid 27333; foreground node …/bob chat --accept-license --trust --auto-approve -w . --mode posse-f1ytb

herdr workspace create --label posse-f1ytb-probe-b --no-focus --cwd <scratch>/ws   → w2M0:p1
herdr pane run w2M0:p1 "bob chat --accept-license --trust --auto-approve -w . --mode nosuch-f1ytb"
herdr pane read --source visible --format text w2M0:p1 | tail
   → "(ℹ) Unknown mode "nosuch-f1ytb". Falling back to "agent" mode."    ← one grey line
     "(ℹ) Logged in as …"
     "❯   Build Anything, @ for context, / for commands, $ for skills"
     "Agent Mode (auto-approve)"                                          ← no persona

herdr workspace close w2MZ ; herdr workspace close w2M0 ; go run ./cmd/checkorphans → clean
```

So: (1) the workspace plugins glob is real and `--mode` selects a mode from
it, with `hidden: true` accepted; (2) an unknown slug is **not** a refusal —
Bob prints an info line and runs as Agent. The second is the hazard class
ranger-base-5jjtn paid a probe for (`.allowUnknownOption()`: accepted and
ignored looks like delivered), one layer up, and it is why ADR 0062 D2 reads
the footer before the first keystroke rather than trusting the flag.

The `(ℹ) Logged in as …` line and the splash are the operator's signed-in
state; nothing was typed into either composer.

## 3. The grid sentence with no reading behind it

`internal/posse/runtimecheck.go`, `launchRow`:

```go
value: seen + "; PID delivered by the template; " + un,
```

Unconditional. The only PID-delivery check on any launch path is
`Runtime.PIDVoided` (runtime.go), which asks whether a *flag* on the line
voids the PID (`--system-prompt-override` and friends) and never whether the
template carries a `{file}` at all. With 5jjtn's `BobCommand` the row still
reads "PID delivered". Fourth instance of the ADR 0013 §1 class (9r33, vbp3,
i3q6g, now the PID row).

## 4. Fixture count, for the refusal's blast radius

```
grep -rn 'command:' internal/posse/*_test.go | wc -l                                   → 222  (every string, prose included)
… | grep -v '{file}' | wc -l                                                           → 109
grep -rnE 'command: *"?(claude|codex|grok|bob|true|sh |/|x\b|echo|sleep|env )' … | grep -v '{file}' | wc -l   → 19
```

The 19, by file: cageinner_test 4, cage_test 4, cagelauncher_test 2, and one
each in verify_x8mzn_qa, unattended, runtimecheck, pidcheck, pathscoped,
loadguard, herdr, egress, accountconsumers. The bead's "~92" was the 109 less
a guess at cage keys. Only those that reach a bead-carrying launch meet the
new refusal; that subset is ≤ 19 and is the builder's to count.

## 5. What posse already does in the session tree

`RenderAgentsSkills` (skills.go) writes `<cwd>/.agents/skills/<name>` symlinks
for a `SkillsCwd` runtime — bob is one — refuses to overwrite a path it did
not write ("not written by posse — not overwriting"), sweeps dead links, and
appends the pattern to `.git/info/exclude` (`excludeFromGit`). ADR 0062 D1
is that function's sibling for `.bob/plugins/posse/custom_modes.yaml`, and
every property it names (session-local, excluded, refuses a foreign file,
rides into the cage with the mounted tree) is one the skills tree already has.

`bobProjectConfig` lists `.bob/settings`, `.bob/mcp.json`, `.bob/hooks`,
`.bob/custom_modes.yaml` and not `.bob/plugins` — but §1's glob says Bob
loads `plugins/*/custom_modes.yaml` from the workspace, so a repo can hand Bob
a system prompt and tool groups through a path the trust check does not look
at. ADR 0062 D1.4 widens the list and exempts posse's own entry.

## 6. Claims, labelled

MEASURED: §1 rows except `hidden`'s consumer; §2 both panes; §3; §4.
ASSUMED: `hidden: true` hides the mode from the picker; the `roleDefinition`
reaches the model as the persona (the footer proves selection, not a turn);
rules_precedence of `roleDefinition` against the workspace's AGENTS.md. All
three are one billed Bob turn each, on the instance side's lane.
