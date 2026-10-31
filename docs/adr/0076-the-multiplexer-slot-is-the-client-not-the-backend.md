# ADR 0076 — the multiplexer slot is the client, not the backend: ten verbs, herdr and Rex behind them, and on Rex the runtime reports its own state

*Status: accepted 2026-10-31 (ranger-base-lcxr3, operator direction 2026-10-24) · owner: architect · applies ADR 0075 §2 (the slot rule) to its first slot · probe: `docs/adr/0076-multiplexer-slot.probe.sh` · spike ranger-base-ch2rb: `docs/adr/0076-hook-route.probe.sh`, see "Spike" below*

## Context

ADR 0075 made the multiplexer a slot and said the interface gets extracted
when a second occupant is being built. Rex is that occupant. Its beta is
installed on this box, so this record measures the binary, not the docs.

MEASURED 2026-10-31, this worktree at `2cd39cc5`, part 1 of the probe:

- 150 non-test `.go` files. 21 hold or build a `Herdr` or `HerdrBackend`.
- The `Herdr` client has 25 verbs. 19 are called outside `herdr.go`, from
  14 files.
- `HerdrBackend` has 22 exported methods. Almost none of them is herdr:
  the meta files, launch planning, kill-and-land, crew marks, bead notes
  and relaunch are posse's own logic. They reach herdr only through the
  client's verbs.

MEASURED 2026-10-31 against Rex Beta 0.1.0 (CLI `rex 0.0.1793377904`, API
revision 8), part 2 of the probe and by hand:

- Rex has a status protocol, OSC 7501 (spec revision 0.4, 2026-10-10,
  superlogical.com/rex/docs/build/program-status). A program writes
  `ESC ]7501;state=…:kind=…:app=…:msg=<base64> ESC \` to its terminal.
  Rex keeps one record per id, and `block.method program_status` reads the
  records. States are `idle | working | done | blocked | error`. `kind`
  (`permission | question | auth`) is set on `blocked` only. Rex drops
  `working` and `blocked` when the process exits. The 2026-10-24 finding
  that "Rex documents no agent state" was wrong: the docs had it under
  `build/`, and the 10-24 search read only `automate/`.
- **Claude Code 2.1.314 reports it natively.** Launched in a Rex block
  with no hooks, its record read `idle`, `app=claude-code`, with
  `owner={pid, name:"2.1.314"}`. One prompt turned it to `working` and
  then `done`. After exit there was no record, and `process.last_exit`
  gave the exit code. (The turn was meant to be a local `!sleep 6`. Rex
  delivered the `!` and the text separately, so claude made one small
  model turn on the subscription. `blocked` was not exercised live; the
  binary has `kind=`, `blockedOn` and `progress=` beside the emitter.)
- **grok-build 1.0.52 reports it natively.** This is STATIC: the binary
  holds `]7501;state=…:kind=permission:app=grok-build:msg=`, the `7501;?`
  feature probe and a `[ui.notifications] program_status` setting. It was
  not run in Rex. *(Spike, MEASURED: launched in Rex it writes no record
  before a prompt, so §3 refuses it. See "Spike" below.)*
- **codex 0.153.4 and bob (bobshell) carry no `7501;` string.** Also
  STATIC. Neither reports state on Rex.
- **The hook route needs a controlling tty.** A child process with its
  stdout redirected that writes the report to `/dev/tty` lands, as
  record E shows. A `setsid` child gets `ENXIO` and lands nothing. A
  record written this way has no `owner`. *(Spike, MEASURED: that holds
  only when the shell is in the foreground. Rex stamps the tty's
  foreground program, so a codex hook's record carries `codex`.)* A probe SessionStart hook passed
  to claude through `--settings` never fired, and I did not find why, so
  the hook route is UNPROVEN for every runtime.
- Rex ids are UUIDs (`session:01a1902c-…`). Terminal blocks take
  `command`, `cwd` and `shell`, but no env. Blocks also push
  `program_status_changed`, `program_status_removed` and `child_exited`
  events.

## Decision

**1. The slot boundary is the client.** The interface is extracted from
the client verbs posse calls, not from `HerdrBackend`'s 22 methods.
`HerdrBackend` is foundation code that happens to carry herdr in its name.
It keeps all of its logic and calls a `Mux` where it now calls `b.H`. The
14 files that call the client directly do the same. The type rename
(`HerdrBackend` to `Backend`) is a later, mechanical bead. It is not this
design.

**2. Ten verbs.** A `Seat` is an opaque handle that the backend fills and
the meta file stores: server, session or workspace id, and pane or block
id.

| verb | herdr today | Rex |
|---|---|---|
| `Create(label, cwd, argv, env) Seat` | `workspace create` + `pane run` | `session.create` with a terminal block `{command, cwd}`; env goes into the argv as `env K=V …`, because blocks take none |
| `Close(Seat)` | `workspace close` | `session.destroy` (`rex kill`) |
| `List() []SeatInfo` | `workspace list`, `agent list` | `session.list`, plus `program_status` per block |
| `Focus(Seat)` | `workspace focus` | `session.focus_block` |
| `SendText(Seat, text)` | `pane send-text` | `rex send -b` |
| `SendKeys(Seat, keys…)` | `agent`/`pane send-keys` | `rex send-key -b` |
| `Read(Seat, lines, ansi)` | `pane read`, `agent read --format ansi` | `rex capture -b [--format vt]` |
| `State(Seat) Reading` | `agent explain` / `agent get` | `program_status`, plus `process` for liveness |
| `Wait(Seat, until, timeout) Reading` | `agent wait` | `State` polled until a deadline |
| `Prompt(Seat, text, wait, timeout)` | `agent prompt` (native) | `SendText` + `enter`, then `Wait(working\|blocked, 5000 ms)`; a miss returns the same `agent_prompt_stalled` dispatch already branches on |

`Reading` holds `State` (`working | blocked | idle | done | unknown`, the
vocabulary posse already uses), `Kind`, `Owner` and a free-text
`Evidence`. On Rex, `error` maps to `done`: the turn ended, and posse's
turn-failure readers judge the screen as they do today. No record maps to
`unknown`, never to `idle`.

**3. Agent state on Rex comes from the runtime's own report.** posse
reads the report and does not invent one. There are no screen rules on
Rex and no posse-written hooks. The launch-stage rule of ADR 0013 §1
(no detection means no dispatch) applies here as: a Rex launch waits a
bounded time for the first record. If none arrives, it fails with a
message that names herdr as the way to run that runtime. Today that admits
claude (MEASURED). grok, codex and bob are refused on Rex and keep
running on herdr. (grok was "probably admitted" here until the spike
below measured no launch record.) A record's `owner` is checked by
pid and name (Rex warns that pids are reused), and `owner` must not be the
pane's shell. That is the Rex form of herdr's `Seen()`: positive evidence
that the runtime has the keyboard.

**4. Verbs only herdr has stay herdr's.** They are reached by type
assertion on the herdr `Mux`: the `EvaluatedRules` diagnostics, the
readings-log detection read, `PaneAgentSession`, the detection preflight
(`KnownAgentKinds`, `AgentManifest`, `HasReportAgent`) and the
report-agent route. On Rex each of these answers "not available", never
"no", which is ADR 0013's unknown-is-not-no rule.

**5. posse keeps what it already owns, whichever backend runs.** That is
the meta files, seatbelt, gates, the bd queue, kill-and-land, crew marks
and launch planning. The meta file gains a `Mux` key, and an absent key
means herdr, so every file on disk reads unchanged. `Socket` holds the Rex
server address. `Gen` stays empty on Rex. herdr needs a generation to tell
a reused workspace id apart; Rex's UUIDs are never reused, so a seat is
dead when its session id is missing from `session.list` on that server.

**6. Rex is opt-in per instance: `multiplexer: rex`.** The default stays
herdr, and herdr's code paths do not change. This is not a fork. Rex is
the second occupant behind one interface, as ADR 0075 decided. I found no
reason to diverge from that.

## Consequences

- One refactor bead lands the interface with no change in behaviour. The
  suite is the proof. After that, the Rex occupant is about 10 verbs over
  one CLI.
- A Rex instance cannot run codex or bob seats until they report state.
  That is a capability gap posse names at launch, not a bug it hides.
- Rex's CLI is not on `PATH`. The Rex `Mux` takes the binary from config,
  the way `RHQ_HERDR_BIN` works for herdr.
- `Wait` polls. The rates are ASSUMED low enough (one seat at a time,
  deadlines in seconds) that an event subscriber would cost more than it
  saves.
- ADR 0075's cost line held: the seam widened from 13 to 21 files while
  it waited, and the extraction is still one bead.

## Alternatives rejected

- **An interface over `HerdrBackend`'s 22 methods**, which is the bead
  title read literally. Each backend would carry its own meta, landing
  and crew logic: three thousand lines of foundation written twice.
- **Screen rules on Rex** (the herdr-bob `rules.json` approach). posse
  would maintain a second detection engine for a multiplexer whose own
  answer is that the runtime reports its state, and claude already does.
- **posse-installed hooks that emit OSC 7501 for every runtime.** This is
  the clever one I wanted: one state source on both multiplexers, which
  might even replace herdr's detection one day. It rests on a hook having
  a controlling tty, and the one hook I tried did not fire at all. That
  makes it UNPROVEN, and posse would own a detection layer. It is filed as
  a spike for codex and bob only.
- **`rex skill setup`.** It writes Rex's hooks into the operator's global
  claude and codex config. Those hooks tell an agent which session it is
  in, not what state it is in. posse seeds per launch and never writes
  global config.
- **An event subscriber for `Wait`** (`rex events --json`, with
  `program_status_changed`). It is a long-lived actor and a stream to
  resume, and polling with a deadline answers the same question.
- **A per-PID `multiplexer` key.** That would allow mixed instances, so
  codex on herdr beside claude on Rex. No persona needs it today, and it
  can be added later without breaking anything.
- **A posse fork for Rex** (the operator's "posse under Rex native", read
  literally). ADR 0075 §2 already answers it, and nothing measured here
  argues for a fork.

## Spike: the hook route, and grok at launch (ranger-base-ch2rb, 2026-10-31)

The question was: can a codex or bob hook report OSC 7501 on Rex, and does
grok report at launch? The probe is `docs/adr/0076-hook-route.probe.sh`.
It ran against Rex CLI 0.0.1793377904 (API revision 8), codex-cli 0.153.4,
bobshell 2.0.7 and grok 1.0.52, on this box. No model turn was spent: every
UserPromptSubmit hook exits 2 (both CLIs block the prompt before any
request), codex's only provider is a dead loopback port, and grok was never
prompted.

**Answers, MEASURED:**

| runtime | launch record | hook reaches `/dev/tty` | record `owner` |
|---|---|---|---|
| grok 1.0.52 | **no**, at 3, 6 and 12 s, with `program_status` `auto` and again forced `on` in a project `.grok/config.toml` that `grok inspect` lists | n/a | n/a |
| codex 0.153.4 | **no**. SessionStart fires on the first prompt, not at launch | **yes**: the hook runs in the pane's session (own process group), and the record lands | `{pid, "codex"}` |
| bob 2.0.7 | **no**. SessionStart fires on the first prompt (`onTurnInput`) | **yes**: the hook runs in bob's process group, and the record lands | `{pid, "node"}` |

Rex sets `owner` to the tty's foreground process, whoever writes the
report. When the same hook is run from the bare shell, the record has no
owner (shell in the foreground) or names `bash`. That is the case §3's
"not the shell" rule exists to catch.

**Event mapping**, as the probe's hook reported it:

- codex: SessionStart → idle, UserPromptSubmit → working (MEASURED),
  **Interrupt → idle** (MEASURED: Esc on a running turn) and
  **SessionEnd → `state=clear`** (MEASURED: `/quit` removed the record).
  PreToolUse and PostToolUse → working, PermissionRequest →
  `blocked:kind=permission` and Stop → idle are STATIC: they need a model
  turn. One gap is MEASURED: a prompt that a hook blocks fires no Stop and
  no Interrupt, so the record stays `working`.
- bob: SessionStart → idle and UserPromptSubmit → working (MEASURED).
  Pre/PostToolUse → working and Stop → idle are STATIC. bob 2.0.7 has
  **no** permission, interrupt or session-end event (STATIC, bundle event
  list), so a bob hook can never report `blocked`, and a cancelled turn
  keeps `working` until bob exits.
  This matches the mapping herdr-bob's `bob-hook` already uses.

**What it costs to turn on (MEASURED):**

- codex opens a "Hooks need review" dialog before any hook runs. Its
  default action is review, and posse launches with `--disable hooks`
  today. "Trust all" writes `hooks.state."<hooks.json>:<event>:<i>:<j>".trusted_hash = "sha256:…"`
  into the config of the home codex was given. The hash covers the
  declaration, not the script body: editing the script needed no re-trust.
  A per-invocation `--dangerously-bypass-hook-trust` exists; this probe did
  not use it.
- A fresh `CODEX_HOME` also draws the update menu, whose default runs
  `brew upgrade --cask codex`. The probe's first draft typed Enter into it.
  The upgrade was refused only because the cask is pinned.
  `check_for_update_on_startup = false` (posse's fleet pin) prevents the
  menu.
- A hook's last `idle` **outlives the runtime**. Rex drops only `working`
  and `blocked` at exit, so after a `/quit` without a SessionEnd hook the
  record stayed `idle`, with a dead owner pid. A State reader that skips
  the pid-liveness check reads a dead seat as idle.
- bob merges workspace hooks with the operator's global ones (herdr-bob's
  five fire too) and runs them only under `--trust` (STATIC). Behind a dead proxy it
  never leaves "Initializing…", so it cannot be fenced off the network the
  way codex can.

**grok's support query leaks into its composer (MEASURED 3 of 3).** Rex
answers `7501;?` with
`ESC]7501;?:states=idle,working,done,blocked,error:kinds=permission,question,auth ESC\`.
grok 1.0.52 shows the last six bytes, `n,auth`, as typed input. The first
prompt posse typed would be `n,auth<prompt>`.

**What this changes:** §3's launch rule, "wait for the first record",
admits claude alone. grok is refused, not "probably admitted". The hook
route works mechanically for codex (every state except a measured
`blocked`) and only partly for bob (no `blocked`, no clear-on-exit). But no
hook writes a record before the first prompt, so even with hooks codex and
bob fail §3 as written. The hook route stays rejected for now (Alternatives,
above). Reopening it is a design question, not a slice: what admits a
runtime that reports only after its first prompt. It is filed as its own
bead. One more case for the Rex State reader: a record whose owner pid is
dead is `unknown`, never its last state.

UNPROVEN still: why claude's `--settings` SessionStart hook did not fire
in the 0076 probe, `blocked` from any runtime live, and grok's reports
during a turn. Each needs a model turn or a permission prompt, and this
spike spent none. In 1 of 3 scripted codex runs, the keystrokes after the
blocked prompt were lost (cause not found); the other two and the hand runs
were clean.
