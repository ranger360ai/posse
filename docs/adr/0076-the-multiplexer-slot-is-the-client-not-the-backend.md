# ADR 0076 — the multiplexer slot is the client, not the backend: ten verbs, herdr and Rex behind them, and on Rex the runtime reports its own state

*Status: accepted 2026-10-31 (ranger-base-lcxr3, operator direction 2026-10-24) · owner: architect · applies ADR 0075 §2 (the slot rule) to its first slot · probe: `docs/adr/0076-multiplexer-slot.probe.sh` · spike ranger-base-ch2rb: `docs/adr/0076-hook-route.probe.sh`, see "Spike" below · amended 2026-10-31 (ranger-base-zkeik): §3 admission is decided before the claim and confirmed by the delivery mode; codex and grok stay refused, see "Amendment" below · probe: `docs/adr/0076-argv-admission.probe.sh` · amended 2026-11-06 (ranger-base-dok0a, ADR 0080 D3): codex is admitted on Rex through hooks and trust typed on its launch line; grok and bob stay refused pending their spikes, see "Amendment 2" below · probes: `docs/adr/0076-flag-trust.probe.sh` (the design), `docs/adr/0076-reporter.probe.sh` (the reporter slice, ranger-base-z7kle), `docs/adr/0076-trust-ask.probe.sh` (the trust-ask slice, ranger-base-fwwl8), `docs/adr/0076-reporter.probe.sh` arm `tty` (the admission slice's darwin measurement, ranger-base-8gh58) · amended 2026-11-06 (ranger-base-1vgrw): grok is admitted on Rex by its own report once the operator's grok config sets `program_status = "on"`; D3's `grokon` arm never set it, and grok's hooks cannot reach the tty, see "Amendment 3" below · probe: `docs/adr/0076-grok-hook.probe.sh` · amended 2026-11-06 (ranger-base-778y0): bob is herdr-only on Rex, because `chat` takes no argv prompt and no record exists before a typed one, see "Amendment 4" below · probe: `docs/adr/0076-bob-argv.probe.sh`*

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
- **Env in argv costs no more on Rex than on herdr** (ranger-base-ytxy0,
  same binary). The crew's `default` env set carries the runtime's OAuth
  token, so this was checked before the Rex `Create` was built. I launched
  `rex new --keep-open -- /usr/bin/env PROBE_SECRET=<dummy> /bin/sh`. The
  dummy value was absent from `session.list`, `session.view`,
  `session.describe_block` and Rex's support and cache directories on disk.
  The block's `process` reading showed plain `sh`, because `env` execs
  away. The value is visible only in the `rex` CLI's own argv for the
  length of the call, which is the exposure herdr's `--env` already has.
  The meta file keeps env-set names and never values (`HerdrMeta.Envs`).
  The Rex `Mux` must keep it that way: no argv in meta, logs or
  `Evidence`.

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
that the runtime has the keyboard. *(Amended by ranger-base-zkeik, below: which
runtimes may launch on Rex is a declared, measured fact read before the
claim, and the wait for the first record is its confirmation, keyed on
the runtime's `prompt:` column.)*

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
dead when its seat is missing from a fresh `List()` of the occupant on
that server. That is the occupant's listing, not rex's `session.list`
method: a Rex session outlives its blocks, so the raw method never drops
a finished seat. *(Corrected by ranger-base-p7mwt, below.)*

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
- Scheduled dispatch is started by herdr's cockpit plugin (its
  `[[startup]]` hook runs `posse dispatch --watch`). Rex has no such hook,
  so on a Rex instance the operator starts the loop from any shell. The
  loop is a plain CLI and needs nothing from the multiplexer it runs in.
  The cockpit stays herdr's. A Rex cockpit gets a bead when someone asks
  for one.
- Every crew PID on this instance is `runtime: claude` (MEASURED
  2026-10-31, `grep '^runtime:'` over the instance's `agents/`). So the
  §3 refusals (grok, codex and bob, as the ranger-base-zkeik amendment
  below settles them) do not stop the crew running on Rex. They stop only
  a seat whose runtime is overridden to one of those three.
- `Wait` polls. The rates are ASSUMED low enough (one seat at a time,
  deadlines in seconds) that an event subscriber would cost more than it
  saves.

  *MEASURED 2026-11-07 (ranger-base-74tkp), and the ASSUMED held while its
  premise did not: a dispatch wait leg's deadline is `PromptWaitMS` —
  fifteen minutes — and not seconds, and what polls it is not this verb.
  `Wait` kept its own one-second interval throughout; the rate that ran
  away was the LEG rate above it. dispatch's gather re-waited on whatever
  the last leg returned without asking how long that leg took, so on this
  occupant — where a leg ends when its own deadline does, rather than
  blocking inside a server's wait verb — one agent turn became 295 progress
  lines in ~3m30s, ~1.4 legs a second, each one re-reading
  `program_status`. Nothing was misjudged: the claim was kept and the
  settle was read when it came. The fix is a floor under the interval
  between two legs and a separate interval for the progress line
  (`Dispatcher.LegFloor`, `Dispatcher.ProgressEvery`), and it is in the leg
  loop rather than in this occupant, because the same runaway was then
  reproduced on HERDR at 24 legs a second by answering every leg with the
  `timeout` code a real server answers a blown deadline with. The event
  subscriber this bullet declines is still declined: at four readings a
  minute per waiting seat there is nothing for it to save.*
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

## Amendment: what admits a runtime that reports only after its first prompt (ranger-base-zkeik, 2026-10-31)

The spike above left one question: §3 admits on a launch record, and only
claude writes one. Is there an admission rule that is still positive
evidence (ADR 0013 §1) for a runtime that reports only once it has a
prompt? And is codex on Rex worth posse owning a hook layer?

**The fact that reframes it.** posse does not type codex's or grok's
work prompt. Both are `prompt: argv` (ADR 0013 §2): the prompt rides on
the launch line, so the launch *is* the first prompt. herdr already
lives with this shape for grok: `awaitDelivered` waits for a screen
after an argv launch, because "detectability is a property of having
been prompted" (dispatch.go). So the question for an argv runtime is not
"does it report before a prompt" but "does it report after an argv
launch". The probe measured that.

**MEASURED 2026-10-31**, Rex CLI 0.0.1793377904, codex-cli 0.153.4,
grok 1.0.52, `docs/adr/0076-argv-admission.probe.sh`. No model turn was
possible: every CLI ran under `sandbox-exec (deny network*)`, in a scratch
home with a synthetic key, and codex's UserPromptSubmit hook exits 2.

| arm | first record | owner |
|---|---|---|
| grok, argv prompt, `program_status` auto | **none** at 2, 5, 10, 20 s while its turn was live (`Retrying…`) | — |
| grok, argv prompt, forced `on` in `.grok/config.toml` | **none** at 2 s through **131 s**, turn still live | — |
| codex, argv prompt, hooks declared but not trusted | **none**: the "Hooks need review" dialog holds the prompt | — |
| codex, argv prompt, hook trust seeded in that home | **`working` by t+2 s** (SessionStart → idle, then UserPromptSubmit → working) | `{pid, "codex"}` |

Also MEASURED: grok's composer again held `n,auth` (the tail of Rex's
`7501;?` reply) in both grok arms. grok reads the reply and keeps part of
it as input, so a broken parse of that reply is a plausible reason its
`auto` never turns on. That cause is UNPROVEN, and "forced on" did not
help either. codex keys project trust by the resolved path
(`/private/var/…`, not `/var/…`); an unresolved key draws the
directory-trust dialog instead. The trust table's `trusted_hash` matched
none of seven obvious serializations of the declaration, so posse could
not compute it in this session. It is derivable from codex's source, but
nobody documents it.

**Decision.**

**D1. Rex admission is a declared fact, read before the claim.** The
builtin runtime table carries one builtin-only bool, `ReportsOnRex`. It
is set on claude alone and is never declarable in a runtime yaml. On a
`multiplexer: rex` instance, dispatch refuses a runtime without it
before the claim and before any worktree or session exists. That sits
beside the `danger:` refusal, with the same busy-key arm (ADR 0013 §1
rule 4) and the herdr message. This is ADR 0013 §1 rule 3, "before
anything is spent", which a try-and-see refusal would break for argv
runtimes (rejected below). It mirrors herdr's launch row, where the
manifest is the declaration that the runtime can be read. Interactive
`posse new` warns and proceeds instead (§1 rule 5): the operator is at
the keyboard, and `State` reads `unknown` until a record arrives.

**D2. The first-record wait confirms the declaration, keyed on
`prompt:`.**
- `typed`: wait for a record *before* typing, as §3 says today. No record
  within the bound means a refusal with nothing typed. claude is
  MEASURED here.
- `argv`: wait for a record of any state *after* the launch, with the
  owner check from §3. No record within the bound is a **HOLD**, not a
  refusal (ADR 0066 D1, the `awaitDelivered` consequence). The prompt is
  already with the CLI, so the claim is kept and the bead is not judged
  this pass.
Positive evidence is the same in both cases: a record whose owner is the
runtime and not the shell.

**D3. grok is refused on Rex (MEASURED).** *(Replaced by Amendment 3's D12: the `grokon` arm set the key in a project file grok does not read for `ui.*`, so only `auto` was measured.)* No record in 131 s of a live
argv turn. The reason is UNPROVEN, and the fix is upstream. Re-admit
when the probe's `grok` and `grokon` arms show a record within the
bound on a new version, then set `ReportsOnRex` and re-run the first-turn
leak check. The `n,auth` leak also lands in the composer that a later
typed prompt (a re-prompt, `--resume`) would use. It has to be gone
before grok is admitted.

**D4. codex is refused on Rex by price, not by mechanism.** *(Reversed by Amendment 2, below: ADR 0080 D3 accepts the price, and D6–D9 pay it without the per-seat home or the hash.)* The hook
route works: an argv launch with seeded trust reports `working` in 2 s
under codex's own owner (MEASURED). But admitting codex means posse owns
these:
1. a 7501 hook script and a `hooks.json` declaration per launch;
2. trust seeding. Trust lives in `$CODEX_HOME/config.toml`, and posse
   runs codex against the operator's own `~/.codex`. So seeding means
   writing the operator's global config, which posse never does (§
   Alternatives, `rex skill setup`). The other way is a per-seat
   `CODEX_HOME`, which moves codex's credential file, the session rollouts the cost
   adapter and plan hints read (`cost_codex.go`, `planhint_codex.go`) and
   the interstitial silences (`interstitial.go`);
3. an undocumented hash keyed by absolute path, which has to be
   re-audited on every codex bump;
4. dropping `--disable hooks` from `CodexFleetFlags`. That turns the
   operator's global hooks back on in fleet seats (today one:
   herdr's `herdr-agent-state.sh` SessionStart);
5. `blocked`, which is still UNPROVEN live for codex.
All of that buys a capability no seat uses. MEASURED 2026-10-31,
`grep -rh '^runtime:' ~/.config/posse --include='*.md' | sort | uniq -c`:
20 of 20 PIDs run claude. The `multiplexer` key is per instance (§6), and
codex runs on herdr unchanged. **Reopen when** a PID that runs codex is
wanted on a Rex instance. The four rows above are then the slices, the
probe's `codex` arm is the acceptance test, and deriving the hash from
codex's source comes first.

**D5. bob stays refused.** *(Restated by Amendment 2's D11; settled by Amendment 4's D14: herdr-only.)* It is `prompt: typed` with no launch record,
so D2's typed arm refuses it. It also has no `blocked` (spike, STATIC).

**Consequences.** ranger-base-phq39's refusal of codex and grok is the D1
refusal before the claim, not a timed-out wait. On an all-claude
instance, the only effect today is that a Rex instance cannot strand a
claim on a runtime it can never read. Nothing else is cut.

**Alternatives rejected.**
- **Owner-of-foreground plus `process.name`, no record.** It proves the
  process exists, not that its state can be read. `State` would read
  `unknown` for the whole seat, so dispatch could never settle it. That is
  herdr's idle-fallback guess in a new form, which is what ADR 0013 §1's
  settle row refuses.
- **A first-prompt handshake** (posse types a throwaway prompt to provoke
  a record). It spends a turn on every launch and types into a composer
  posse cannot see is ready, which the promptable row refuses. For argv
  runtimes it is unnecessary.
- **Try-and-see for argv runtimes** (launch, wait, refuse on silence).
  The evidence arrives after the prompt is delivered, so the refusal
  comes after a claim, a worktree and a live turn. On grok that is a real
  billed turn that nobody observes. This breaks ADR 0013 §1 rule 3, which
  is why D1 reads a declaration first.
- **The codex hook layer now** (the clever one, again). It works, and it
  is priced in D4. It is deferred on demand, not on doubt.
- **`--dangerously-bypass-hook-trust`** instead of seeding. It trusts
  every hook in reach, including a `.codex/hooks.json` committed in the
  worktree the seat runs in, and hooks run outside codex's sandbox. Any
  repo could plant code that the fleet then runs unsandboxed.

## Correction: §5's liveness rule reads the occupant's `List()`, not `session.list` (ranger-base-p7mwt, 2026-10-31)

**Context.** §5 said a Rex seat is dead "when its session id is missing
from `session.list` on that server". Read literally, that never fires.
MEASURED 2026-10-31 on this box (Rex CLI 0.0.1793377904,
ranger-base-qlxb5, whose notes fragment §4 has the method):
when a terminal block's program exits, the block closes but the session
stays in `rex ls` indefinitely. `rex session inspect` shows `windows: 0`.
Every landed seat would be immortal, and no prune could ever settle its
claim.

**Decision.** The listing in §5 is the occupant's `List()`, the verb in
§2's table. On Rex, `List()` does not return a session that holds no
terminal block. A seat is dead when it is absent from a **fresh**
`List()` on its recorded server. The fresh read is the confirmation, since
the ten verbs have no per-id query (ranger-base-q5gc2 notes §4). This is
what both implementations already do. Nothing in the code changes.

This keeps ADR 0011 §2's rule. A missing block is positive evidence that
the program in it ended. It is not absence read as death. The rule
covers three cases:

- A listing that **fails**, because the server is unreachable or rex
  errors, is unknown. It is not "absent" and kills nothing.
- Reads pass `--autostart=false`. An autostarted server holds no
  sessions, so one autostarted read would make every recorded seat
  look absent (ranger-base-qlxb5).
- The zero-window session id stays addressable, so `Close` still runs
  `session.destroy` on it — which is what ADR 0080 D10 then has the prune
  do, before it unlinks the record (ranger-base-43mxx).

**Alternatives rejected.**
- **Leave the prose as it was and rely on the code.** The next occupant,
  or a rewrite of this one, implements the sentence it reads. That is
  the bug this bead names.
- **`--keep-open` on the block**, so the session and the block share a
  lifetime. That is the wrong direction: the block outlives its program,
  and the seat never ends at all.
- **Have `Close`, or a reaper, destroy a session when its block exits.**
  That could look like the fix, since `session.list` would then mean
  what §5 said. It adds a second writer to the session's lifetime, and
  it races `List()` in the window between the exit and the destroy. A
  crash in the reaper brings the immortal seats back. The occupant
  already reads the block, which is the fact itself, so no reaper is
  needed. ADR 0080 D10's destroy is **not** this reaper: it runs after
  liveness has been proven by that `List()`, by the one home whose meta
  names the id, and it is the prune's own line and not a second writer
  (ranger-base-43mxx).
- **Judge by the root block's `process` liveness, not by its presence.**
  That is §2's `State` verb, and it answers a different question: is the
  runtime alive inside a seat that exists? Liveness of the SEAT stays a
  matter of listing.

## Amendment 2: codex on Rex through its launch line; grok and bob wait on measurements (ranger-base-dok0a, 2026-11-06)

**Context.** ADR 0080 D3 (operator direction, 2026-11-06) makes herdr and
Rex interchangeable for every runtime, and accepts the price D4 refused.
This amendment chooses how to pay it. D4 assumed two of its five items:
that trust has to live in a file (so either the operator's `~/.codex` or a
per-seat `CODEX_HOME`), and that posse has to compute codex's trust hash.
Both turn out to be false for codex 0.153.4.

Read from codex's source at tag `rust-v0.153.4` (STATIC):
- `config.toml` carries hooks as well as `hooks.json` does
  (`HookEventsToml`), so a `-c hooks={…}` override declares them. That is
  codex's session-flags layer. Its hooks are keyed under a synthetic path,
  `/<session-flags>/config.toml:<event>:<group>:<handler>`, which no
  directory and no home appears in.
- `hooks.state` is read from the user layer and the session-flags layer
  only (`hook_states_from_stack`). A project's files cannot set trust or
  re-enable a hook.
- The hash is sha256 over sorted-key compact JSON of the normalized
  declaration (`hook_hash`, `version_for_toml`), not over the source text.
- The startup review fires for any hook that is untrusted or modified,
  enabled or not (`hook_needs_review`). A disabled hook never runs
  (`hook_enabled`). A managed hook (system, MDM, enterprise) can be neither
  disabled nor re-trusted.
- `codex app-server`'s `hooks/list` reports every hook that would load,
  with its key, source, managed flag and `currentHash`. The app-server is
  labelled `[experimental]`.

**MEASURED 2026-11-06**, this box, Rex CLI 0.0.1793377904, codex-cli
0.153.4, `docs/adr/0076-flag-trust.probe.sh`, four runs. No model turn was
possible. Every codex ran under `sandbox-exec (deny network*)` in a scratch
`CODEX_HOME` whose only provider is a dead loopback port, and the hook's
UserPromptSubmit exits 2.

| arm | review dialog | first record | home `config.toml` |
|---|---|---|---|
| `seed`: hooks as `-c` flags, no trust | **yes** | none | "Trust all" wrote 8 keys under `/<session-flags>/…` |
| `flag`: fresh home, the same flags, plus that trust as `-c hooks.state` | no | `working`, owner `{pid,"codex"}`, 3 of 4 runs by t+2–5 s, 1 run by t+42 s (startup, see below) | byte-identical before and after |
| `userhook`: plus an untrusted user-layer `hooks.json`, `enabled=false` only | **yes**, "1 hook is new or changed" | none | — |
| `ask`: the whole `hooks.state` built from `hooks/list`, nothing taken from `seed` | no | `working`, owner `{pid,"codex"}`, by t+2 s; the user-layer hook did not run | byte-identical |

Also MEASURED: `hooks/list` answered in 0.1–0.3 s, and its `currentHash`
equalled the hash the dialog wrote. A Python model of the hash reproduced
codex's own values 8 of 8. The one slow `flag` run sat on
"model: loading" for 20–40 s before it submitted the argv prompt. That was
codex's model-metadata fetch failing under the network deny. A seat with a
network is ASSUMED faster, and the slice measures it against the 45 s
`startup_wait` bound.

**Decision.**

**D6. codex's 7501 hooks ride its launch line as session flags.** On a Rex
instance, codex's line adds `-c hooks={…}`, one group with one handler per
event. The handler is posse's own binary (a hidden reporter subcommand),
not a rendered script. It reads `hook_event_name` from stdin and writes one
OSC 7501 report to `/dev/tty`:

| codex event | report |
|---|---|
| SessionStart, Stop, Interrupt | `idle` |
| UserPromptSubmit, PreToolUse, PostToolUse | `working` |
| PermissionRequest | `blocked`, `kind=permission` |
| SessionEnd | `state=clear` |

No `hooks.json` is written anywhere. Fleet codex runs `-a never`, so
`blocked` should never fire. It is mapped anyway, and it is still UNPROVEN
live (D4 item 5).

*Realized 2026-11-06 (ranger-base-z7kle): `posse hook-report <app>` is the
subcommand, `posse hook-report --declare <app>` prints the declaration, and
both live in `internal/posse/codexreport.go`; the handler is read ahead of
`NewApp` in `cmd/posse/main.go`, as a fourth entry point, because a hook runs
in whatever environment the runtime hands it and `NewApp`'s no-home refusal is
an exit 1. MEASURED on the slice, `docs/adr/0076-reporter.probe.sh` (three
arms, no model turn), with the table above held byte for byte: codex's own
`hooks/list` accepts posse's declaration and reports all eight hooks as
`sessionFlags`, one group and one handler each; the launch draws no review
dialog and the record reads `working`, `app=codex`,
`owner={pid,"codex"}` at t+2 s with the home byte-identical. Two findings
the design did not have — `state=clear` REMOVES the record rather than naming
a sixth state, and codex splits a hook `command` on whitespace with no shell
but honours single quotes, so the handler path is quoted — are in
`docs/notes.d/ranger-base-z7kle.md`. What that run could not reach, and
`ranger-base-8gh58` must: a record that SETTLES. The turn under the probe's
no-turn lock failed on a missing credential and the record stayed `working`
for 60 s with no Stop or Interrupt, so what codex reports after a turn that
ends is still UNPROVEN.*

**D7. Trust is asked of codex, then typed.** Before the line is typed,
posse runs `codex app-server` once, with the seat's `CODEX_HOME` pin
(ADR 0077 D1), the seat's cwd and the seat's `-c` line. It sends
`initialize`, `initialized` and `hooks/list`, and types one
`-c hooks.state={…}` built from the answer:
- posse's session-flags hooks get `{trusted_hash}`;
- every other hook, from any source, gets `{enabled=false, trusted_hash}`.
  Trusting a hook that is disabled only keeps the review dialog from
  firing. The hook still does not run.

posse never computes a hash. Two cases refuse the launch before the line
is typed, so no turn is spent:
- a **managed** hook, because posse can neither switch it off nor vet it;
- an ask that **fails** (no answer, a malformed answer, or a timeout).

The worktree and the claim are released by the ordinary launch-failure
path (ASSUMED that it covers this stage; the slice checks).

This is safe against the file changing after the ask, because trust binds
to the hash. A hook added or edited between the ask and the launch reads
untrusted or modified. It does not run, the dialog holds the prompt, no
record arrives, and D2's argv arm HOLDs. The failure is a stall, never
execution of a hook nobody vetted.

*Realized 2026-11-06 (ranger-base-fwwl8): `CodexHooksAsk.Flags()` in
`internal/posse/codextrust.go` is the ask, and it hands back the PAIR —
`CodexHooksFlags{Declaration, State}` — rather than a renderer to call twice,
because trust binds to the hash of the declaration asked about and two call
sites rendering the same string is how they come to render different ones.
`posse hook-report --ask codex [-c <value>]…` runs it by hand, printing the
trust value on stdout and codex's own inventory on stderr, so both halves of
the line are observable before a launch. Acceptance:
`docs/adr/0076-trust-ask.probe.sh`, three arms, no model turn.*

*ONE THING THIS AMENDMENT DID NOT KNOW, and it decides whether any of the
rest works: **the declaration must be typed BEFORE the trust.** codex applies
`-c` overrides in order and `hooks={…}` sets the whole `hooks` table,
`state` subtable and all, so a `hooks.state` that arrives first is wiped by
the declaration that follows it. MEASURED 2026-11-06 (probe arm `order`, the
same two values both ways round, one scratch home): declaration first, codex
read all nine hooks back `trusted` with the foreign one `enabled=false`;
trust first, all nine `untrusted` and the foreign one `enabled=true` — a line
carrying every byte of the right answer, a launch that completes, and a seat
on the review dialog. `ranger-base-8gh58` types the line and this is the
constraint on it. Also MEASURED: a declaration rendered by a posse at another
path reads `trustStatus: "modified"`, a third value of that field, so a trust
line built against one binary is worthless for another (the caged seat's
`/usr/local/bin/posse`, D6's Realized note).*

*Measured on the slice, beyond what the amendment's table had: the whole
three-message conversation answers in 0.41 s; `hooks/list` returns
`result.data[]` per asked cwd with `errors[]` and `warnings[]` beside
`hooks[]`, which are the only account of a declaration that FAILED to load;
an id-less `remoteControl/status/changed` notification arrives BETWEEN the two
answers, so a reader that takes the next line reads it; and `command` comes
back byte-identical to what posse typed, which is what lets the ask refuse an
answer about a line other than the one it asked about. Three refusals beyond
D7's two, each for the same reason — the alternative is a stall nobody can
read: an answer that does not account for all of posse's own declared hooks,
a key or hash posse cannot put through both shell quoting and a TOML string,
and the same key twice (which would make the inline table invalid TOML).
`docs/notes.d/ranger-base-fwwl8.md` has the tables and the mutants.*

*ANSWERED by the admission slice (ranger-base-8gh58), and the ASSUMED above
was HALF right. The ask is called from `planLaunch`, so its error is an
ordinary launch failure, and:*

- *the CLAIM is handed back. `createSession` returns the error,
  `launchWithPrompt` wraps it in `unclaimAfterLaunchFailure`, the bead goes
  back to open, and the refusal — a plain error — benches the persona/runtime
  slot for the pass rather than being paid once per bead (ADR 0013 §2
  property 4).*
- *the WORKTREE IS NOT REMOVED. `EnsureSessionTree` cuts it near the top of
  `planLaunch`, above every refusal in that function, and nothing unwinds it —
  which is equally true of the danger, detection, parity, PID-channel and
  version refusals. For THIS refusal it cannot be otherwise: the ask is ABOUT
  the tree (codex discovers hooks from the cwd and keys project trust by the
  resolved path), so it cannot be asked before there is one.*

*The tree has no commits, so `posse worktrees --land` removes it ("had no
commits — worktree and the branch removed"). So the cost of a refused ask is
one launch and a tree that waits for a sweep, and never a turn. Both halves
are pinned, including the surviving tree, in
`TestQACodexRexAskRefusalUnclaimsTheBeadAndLeavesTheTreeForTheSweep`.*

**D8. Rex only. herdr's codex line does not change.** `CodexFleetFlags`
keeps `--disable hooks` on herdr, which reads codex through its own
detection (§6: herdr's code paths do not change). On Rex the line drops
`--disable hooks` and adds D6 and D7. The posture D4 item 4 protected
holds in both cases: in a fleet seat only posse's hooks run.

*Realized 2026-11-06 (ranger-base-8gh58): `RexReportLine`
(`internal/posse/codexrex.go`) is the whole of it, and it makes three edits to
a line that is already rendered — `--disable hooks` off, the declaration on,
the trust on after it — rather than growing a second codex template. It is a
no-op on herdr and on any runtime whose record is its own, keyed on
`Runtime.RexRoute` (built-in only) so that no launch path asks a runtime's
NAME for a fact about its mechanism (ADR 0017 §3). `CodexFleetFlags` is
unchanged and pinned byte for byte, and `CodexDisableHooks` is split out of it
only so the removal and the refusal name one string.*

*Two things the design did not say, both found by building it:*

- ***the removal has to be WHOLE-WORD.** A plain substring removal takes the
  same bytes out of `--disable hooks,mcp` and leaves `,mcp` behind — codex's
  hook layer still off, through a flag the guard can no longer SEE, because
  the string it looks for is the string that was just removed. So anything
  posse's own spelling does not match is LEFT in place, where a refusal finds
  it. A PID's `command:` is the one template posse did not write, and this is
  the shape that would have turned it into a silent seat.*
- ***both values are shell-quoted, and the declaration's own single quotes are
  why.** The line is TYPED — into a pane, or into the launch script a line
  over `PaneLineMax` spills to — and the declaration carries `'…'` around the
  handler path by construction (codex splits a `command` on whitespace with no
  shell and honours single quotes, MEASURED under ranger-base-z7kle). The
  `'\''` idiom carries it, and a real `/bin/sh` is what the pin asks (it
  splits the typed line into the exact argv codex must receive).*

*The pair adds roughly 1.6 KB to codex's line, which spends headroom that was
already spent: every persona line has spilled to `state/launch/<session>.sh`
since 2026-09-01 (1867-2058 bytes against the 1023 limit), so the spill is
where this lands and no behaviour changes.*

*Also typed on the RELAUNCH path (`RelaunchAgent`), through the same function,
and the ask runs AGAIN there rather than its answer being remembered: trust
binds to the hash of the declaration asked about, and a re-render can name a
posse upgraded since the launch. A revived codex carrying the launch's old
trust is a seat on the review dialog; one carrying no declaration is a seat
posse cannot read.*

**D9. Admission is still a declaration read before the claim (D1).**
`ReportsOnRex` becomes true for codex. The field's meaning widens to "a
record whose owner is the runtime, written natively or by posse's route
on the runtime's own events". Both are positive evidence under ADR 0013
§1: the report is caused by the runtime's own event and stamped with its
foreground process. Silence is not evidence.

The route rests on two things codex does not promise: an experimental RPC
and session-flag hook semantics. So the declaration carries the version it
was measured on (`codex-cli 0.153.4`). Before the claim, a codex that
reports another version is refused on Rex, with the probe's `ask` arm
named as the re-measurement. That costs one probe run (about two minutes,
no turn) per deliberate codex bump. The cask is pinned.

*Realized 2026-11-06 (ranger-base-8gh58). `ReportsOnRex` is true for codex,
and the field's doc carries D9's widened meaning. The version is a second
built-in-only key, `RexVerified` (`codex-cli 0.153.4`), read as
`RexVersionDrift` at the SAME two sites D1 is read at — `dispatch.go` above
the claim and `planLaunch` as the backstop — with D1's interactive asymmetry
(a dispatched launch refuses, `posse new` warns and proceeds, because
launching is how the next measurement gets taken). `posse gates`' Rex listing
reads both, so a drifted codex is shown in a refused column instead of under
"launches here".*

*Three things the decision did not settle, decided here:*

- ***an unreadable version is refused too.** D9's sentence is "a codex that
  reports another version"; a codex that will not answer `--version` reports
  none. The pinned claim is "this route was measured on THIS version", and a
  binary that cannot be identified leaves it unsupported — ADR 0013 §1's
  "unknown is never read as no". The cost of refusing is one launch that would
  have opened on a codex posse cannot name; the cost of admitting is the stall
  below. A codex that cannot answer `--version` was not going to hand the seat
  a record inside its 45 s `startup_wait` either.*
- ***a NATIVE report carries no pin, and that is not an omission.** claude's
  record is claude's own output: a regression there shows up as D2's
  first-report wait missing, with nothing typed and nothing spent. codex's
  exists because posse typed two `-c` values derived from an experimental RPC,
  and the likely failure of a moved codex is not an error but a line codex
  ACCEPTS — a launch that completes, a seat on the startup review dialog, and
  `prompt: argv`'s HOLD keeping the claim over a turn nobody can read. The pin
  is for that asymmetry and `RexVerified` is empty wherever it does not hold.*
- ***the reader is parity.go's own `--version` seam** (`skillsDriftVersion` →
  `ProbeCLIVersion`), compared through `skillsVersionRelease` (ADR 0007 §6c).
  One reader, one per-process memo — so the dispatch loop pays one exec per
  process, and a codex upgraded under a running `posse dispatch --watch` is
  not noticed until the next process, which is the property `posse runtime
  drift` exists for.*

*MEASURED by mutation (ranger-base-8gh58): with `dispatch.go`'s version arm
deleted, every pin that called `RexVersionDrift` directly stayed green. The
pin that reds it drives a whole pass and asserts that bd is never asked to
CLAIM — which is D9's placement, not its message
(`TestQACodexRexVersionRefusalLandsAboveTheClaim`).*

**How D4's five items are paid:**
1. Hook script and declaration per launch: no script, and no file. One
   `-c` on the line, pointing at posse's own binary.
2. Trust seeding: on the line. No home is written, `~/.codex` is never
   touched, and there is no per-seat `CODEX_HOME`. So the credential file,
   the rollouts and the interstitial silences stay where they are.
3. The undocumented hash: never computed. codex reports it.
4. Dropping `--disable hooks`: Rex only, with every non-posse hook switched
   off by key.
5. `blocked` UNPROVEN live: still unproven, and moot under `-a never`.

**D10. grok stays refused on Rex (D3, MEASURED, not reversed).** *(Replaced by Amendment 3's D12: grok's hooks cannot reach the tty, and grok reports natively under `program_status = "on"`.)* The
question was whether a posse-side route exists that is still positive
evidence. The answer has two parts:
- **It would be evidence.** grok runs hooks before any model turn
  (`.grok/hooks/*.json`, and the hooks in `.claude/settings.json`, which
  RAN in a trusted directory: MEASURED, ranger-base-1g8gf). A hook record
  stamped with grok's own process would stand where codex's stands.
- **It is not routable today.** grok has no flag that carries hooks or
  settings (`grok --help`, 1.0.52). Its hooks would need a file posse does
  not own: a per-seat `GROK_HOME` (D4 item 2's price again, which moves the
  session store the turn-outcome reader uses) or a file in the session
  worktree. And the `n,auth` leak comes from grok's native `7501;?` probe,
  which a hook does not stop. Whether `program_status = "off"` stops it is
  unmeasured.

That makes it a measurement, and it can be taken without a turn (the
1g8gf rig). It is filed as a spike. grok stays on herdr until the spike
lands or upstream fixes grok's own report. ADR 0080 D3's "every runtime"
waits on that spike for grok.

**D11. bob stays refused on Rex (D5).** *(Replaced by Amendment 4's D14: no argv route, MEASURED with no turn spent, and bob is herdr-only on Rex.)* bob is `prompt: typed` with no
launch record, so D2's typed arm refuses it: posse cannot see that the
composer is ready, and §3 forbids screen rules on Rex. `bob chat` (2.0.9)
has no initial-prompt flag, and nothing on the line that declares,
disables or trusts a hook. Its hooks come from workspace and global files,
and the operator's global herdr-bob hooks fire as well. bob cannot be
fenced off the network (spike, above), so measuring whether the top-level
`bob -p` opens the interactive client costs a billed turn. That is the
spike's first question, and it needs operator authorization.

**Consequences.**
- A codex PID runs on a Rex instance. That is ADR 0080 D3 for codex, and
  ranger-base-phq39's refusal shrinks to grok and bob.
- Each codex launch on Rex spawns one `codex app-server` (MEASURED
  0.1–0.3 s with the network denied). Whether it reaches the network at
  `initialize` with a live login is ASSUMED harmless and unmeasured. The
  launch lock is held for that time.
- confirmReports' argv arm ("unreachable today") becomes reachable. A
  codex seat with no record within `startup_wait` HOLDs and is not judged.
  *(Realized 2026-11-06, ranger-base-8gh58: the comments in `multiplexer.go`
  and `herdrback.go` say so now. What reaches that arm is a reporter codex did
  not run or a record that did not arrive in time — the version pin, the
  declaration render and the trust ask all refuse on the far side of the
  launch, before a turn — so the HOLD is the LAST thing between posse and a
  turn it cannot see rather than the first.)*
- codex's record stays `working` after a prompt a hook blocked (spike,
  MEASURED). posse's hook never blocks, so a fleet seat does not reach
  that state.
- A dead owner pid reads `unknown` (spike). SessionEnd clears the record,
  which covers a clean `/quit`.
- The probe is darwin-only (`sandbox-exec`). On the ADR 0080 Linux host
  the route is ASSUMED to behave the same: it is codex's config machinery
  and Rex's protocol, with nothing macOS-specific. The shakedown
  (ranger-base-offid) re-runs it under a Linux network fence.
- On darwin, the hook runs as codex's child under posse's seatbelt when the
  PID is caged. That the hook may open `/dev/tty` there is ASSUMED, and
  the slice tests it. *(MEASURED 2026-11-06, ranger-base-8gh58, and the
  PREMISE was wrong: posse renders **no** seatbelt for codex at all. codex
  declares `SelfSandbox`, macOS refuses to nest `sandbox-exec`, and
  `seatbeltWallRendered` is false for such a runtime whatever tier the PID
  asked for — posse says so as a DeclaredDifference instead (ADR 0017 §2). So
  at `cage: seatbelt` there is no posse profile in front of a codex seat and
  this bullet is moot at that tier; codex is refused at `cage: container`
  today for want of a `CageCred`. The counterfactual was measured anyway,
  because posse's profile is the one wall shape that COULD have refused the
  open: with a profile `RenderSeatbelt` really rendered,
  `sandbox-exec -f <prof> /bin/sh -c 'exec 3>/dev/tty'` SUCCEEDS and posse's
  reporter runs under it. The grant is that profile's own
  `(allow file-write* (regex #"^/dev/"))`, which outvotes its
  `(deny file-write*)` for `/dev/tty`. The OPEN and not a write, so no OSC
  bytes reach anyone's terminal. Pinned as
  `TestQATheHookTTYUnderACagedCodexPID`, whose live arms SKIP where there is
  no controlling terminal — which is every `go test` in a dispatched seat, and
  the reason `docs/adr/0076-reporter.probe.sh` grew a `tty` arm that compiles
  the pin out here and runs it inside a Rex block, where there is a tty and
  where Rex spawns outside this seat's cage. Triangulated by the two earlier
  slices, whose codex ran under `sandbox-exec (deny network*)` AND codex's own
  `-s workspace-write` with the hook writing records throughout.)*

**Alternatives rejected.**
- **A per-seat `CODEX_HOME`**, with symlinks back to the credential file
  and `sessions/`. It is D4's own route. It moves or fakes three stores
  posse reads, a fresh home draws the update menu and the sign-in check,
  and a symlinked config would carry the trust write into the operator's
  file. The line does the same job with no home at all.
- **Writing trust into the operator's `~/.codex/config.toml`.** posse never
  writes global config (`rex skill setup`, above).
- **posse computing the hash.** The model reproduced codex 8 of 8, so it
  is buildable. That was the clever one. But turning off the operator's,
  the project's and the plugins' hooks needs every hook's hash, which means
  re-implementing codex's whole discovery, and every codex bump would need
  a re-audit. Asking is one RPC, and a failed ask refuses safely. The model
  stays in the probe, as a check on the reading.
- **`enabled=false` alone for foreign hooks.** MEASURED: it still draws
  the review dialog.
- **`--dangerously-bypass-hook-trust`.** It would run every hook in reach.
  Rejected in the first amendment, and nothing here changes that.
- **One codex line for both multiplexers.** That changes a working herdr
  path for no herdr gain, and nobody has measured what herdr does with
  OSC 7501. §6 keeps herdr's paths as they are.
- **A `.codex/hooks.json` written into the session worktree.** It is a
  file in the repo tree that an agent can commit, and a flag does the same
  without it.

**Measured versus assumed.** MEASURED (this box, 2026-11-06): the four
probe arms, `hooks/list` latency and agreement with the dialog's hash, the
hash model 8 of 8, that `enabled=false` alone draws the dialog, the CLI
surfaces of `grok --help` (1.0.52) and `bob chat --help` (2.0.9). STATIC
(source at `rust-v0.153.4`): the key format, the layers `hooks.state` is
read from, managed hooks, and the review predicate. ASSUMED: codex startup
with a network inside 45 s, a harmless `initialize` with a live login,
`/dev/tty` under seatbelt, the launch-failure path releasing the claim at
this stage, and the Linux behaviour. UNPROVEN: codex `blocked` live.

*Two of those ASSUMED were MEASURED by the admission slice
(ranger-base-8gh58), and both came back half-true — see D7's and the
Consequences' own notes: the launch-failure path releases the CLAIM and leaves
the WORKTREE for `posse worktrees --land` to sweep, and `/dev/tty` under
seatbelt is moot for codex (posse renders no profile for a `SelfSandbox`
runtime) while being permitted under the profile posse would render. What
remains ASSUMED is unchanged: codex startup inside 45 s with a network, a
harmless `initialize` with a live login, and the Linux behaviour — the first
of those is what the live acceptance measures, against the 45 s
`startup_wait`.*

## Amendment 3: grok reports on Rex natively once its home config says `on`; the hook route is closed (ranger-base-1vgrw, 2026-11-06)

**Context.** D10 left grok on herdr until a spike measured three things: whether
a grok hook can write a record stamped with grok's own process, whether
`program_status = "off"` stops grok's `7501;?` query and with it the `n,auth`
composer leak, and where a hook should live (a per-seat `GROK_HOME` or a file in
the session worktree). The spike answered all three. It also found that the arm
D3 rests on never set the key it was testing.

**MEASURED 2026-11-06**, this box (darwin 25.6.0), Rex CLI 0.0.1793377904,
grok 1.0.52 (0920c8169e27), `docs/adr/0076-grok-hook.probe.sh`. No model turn
was possible. Every arm ran the ranger-base-1g8gf rig: its own fixture `HOME`, a
junk API key, every `*_PROXY` at `127.0.0.1:9`, telemetry off. Each turn
died locally ("Connection failed | Retrying").

Where grok reads `program_status` from, and what follows:

| arm | where `program_status` was set | record | `n,auth` in composer |
|---|---|---|---|
| `auto` | nowhere (default `auto`) | none, through a working turn | **yes** |
| `offenv` | `"off"` in the `GROK_CONFIG` overlay only | none | **yes**: the overlay dropped it |
| `off` | `"off"` in `$GROK_HOME/config.toml` | none | no |
| `on` | `"on"` in `$GROK_HOME/config.toml` | **`working`** by t+4 s, `app=grok-build`, `msg="Connection failed \| Retrying (2/15)"`, owner `{pid, "grok-1.0.52-macos-aarch64"}` | no |
| `onidle` | `on`, launched with no prompt | **`idle`** by t+3 s, the same owner | no |
| `onlife`, `onend` | `on`, argv prompt | `working` during the turn; one Ctrl-C with nothing queued → **`idle`**; `/quit` → **record cleared**; a dead turn stays `working` through 8 min 20 s of retries (retry 9 of 15) | no |

The hook carriers. Every hook mapped grok's event to a report and wrote it to
`/dev/tty`. The `UserPromptSubmit` hook also exited 2:

| arm | carrier | grok loaded it | SessionStart fired at launch | `/dev/tty` write |
|---|---|---|---|---|
| `hhome` | `$GROK_HOME/hooks/posse.json` | yes (`inspect`: Hooks (7), `user`) | yes | **failed** |
| `hwt` | the session dir's `.grok/hooks/`, dir trusted | yes (`project`) | yes | **failed** |
| `hagent` | an agent file outside the tree, `--agent <path>` | yes | yes | **failed** |
| `hglobal` | `~/.grok/hooks/`, env-gated (herdr's own pattern) | yes | yes | **failed** |
| `hcfgpath` | `GROK_CONFIG_PATH` overlay file | **no** (`inspect`: Hooks (None)) | — | — |
| `ctl` | the same hook run from the bare shell | — | — | ok: the record lands, with no owner |

In every arm that ran, grok started the hook as the leader of a session of its
own (the witness line reads `ppid = sid = pgid`). A process in a new session
has no controlling terminal, so `/dev/tty` cannot be opened. codex and bob run
their hooks in the pane's session, and their hooks reach the tty (Spike, above).

**Answers.**
1. *Does a grok hook's record land, owned by grok?* **No.** No carrier changes
   this: grok detaches its hooks from the terminal. The hook route for grok is
   closed by measurement, not by price.
2. *Does `off` stop the probe and the leak?* **Yes**, when it is set in grok's
   home config. It does nothing in the `GROK_CONFIG` overlay, whose allowlist
   is `models`, `features`, `toolset` and `shell_environment_policy`
   (grok's 05-configuration.md). But `off` also stops every record. The setting
   that matters is **`on`**. It skips the query, so nothing leaks, and grok
   reports by itself: `idle` at launch, `working` during a turn, `idle` after a
   cancel, cleared at `/quit`, all stamped with grok's own pid.
3. *Which placement costs least?* The question changes, because a hook needs a
   placement and `on` does not. The key is `[ui.notifications]`, and grok reads
   it only from `$GROK_HOME/config.toml`, `managed_config.toml` and
   `requirements.toml`. A project's `.grok/config.toml` contributes four tables
   only (`mcp_servers`, `plugins`, `permission`, `mcp.max_output_bytes`;
   26-config-reference.md), and the overlay drops it. Both candidates the bead
   named lose:
   - **A file in the session worktree cannot carry it.** Commit risk does not
     come into it. **This is why D3's `grokon` arm read silent**: it set the
     key in a project `.grok/config.toml`, which grok does not read for `ui.*`.
     D3's "forced `on`" was never forced. `ReportsOnRex = false` for grok rests
     on a measurement of `auto` only.
   - **A per-seat `GROK_HOME` could carry it, at D4's price.** It moves
     grok's credential file, which grok rewrites on refresh by atomic rename
     (its per-pid temp files sit beside it), so a symlink is replaced and the
     seat's token forks from the operator's. It moves `sessions/`, which
     `turnfailure_grok.go` and `cost_grok.go` read through posse's own
     `grokHome()`, not the seat's. It moves `trusted_folders.toml`, which gates
     the skills root and the project config (`GrokSkillsDirTrust`,
     ranger-base-1g8gf), so a fresh home degrades every `skills:` launch. And it
     moves the two interstitial keys `GrokInterstitials` reads. That is five
     stores to fake for one line.

   The cheapest placement is one line in the operator's own
   `~/.grok/config.toml`, where the fleet pin's `[cli]` keys already live.

**Decision.**

**D12. grok is admitted on Rex by its own report, behind one operator-owned
config line.** It replaces D3 and D10. `ReportsOnRex` becomes true for grok, and
admission (D1, read before the claim) also requires a builtin-only precondition:
`$GROK_HOME/config.toml` (through `grokHomeIn`, the HOME pin of ADR 0077 D1)
carries `program_status = "on"` under `[ui.notifications]`.
- **Absent, `auto` or `off`:** refused before the claim, and the message names
  the line to add. `auto` leaks `n,auth` and reports nothing on Rex. `off`
  reports nothing. All three are MEASURED.
- **Unreadable file:** "unknown", which also refuses. That is the interstitial
  probes' rule: unknown is never yes.
- D2's argv arm then confirms with the first record, as for claude and codex.
  `onidle` shows that grok satisfies even §3's original typed rule.

**D13. posse documents the line and never writes it.** It joins the `[cli]`
keys in `etc/grok/version-pin.toml` as a key posse requires in the operator's
config, and `make verify-grok-pin` asserts it. This is `interstitial.go`'s rule.
`config.toml` belongs to the operator and grok rewrites it, so copying a file
over it would clobber both, and posse never writes global config (`rex skill
setup`, above). posse reads the file itself, section-aware. It cannot ask grok:
`grok inspect --json` reports no value for the key, and its validator calls
`ui.notifications` an "unrecognized config key" (MEASURED, `configWarnings`),
even though the TUI obeys the key in every arm above. Since grok will not say,
the declaration carries the version it was measured on, as D9 does for codex. A
grok whose version is not `1.0.52 (0920c8169e27)` is refused on Rex, and the
re-measurement is the probe's `auto on off onidle` arms (about two minutes, no
turn). The cask is pinned, so that costs one run per deliberate bump.

**Consequences.**
- ADR 0080 D3's "every runtime" is now short only of bob (D11).
- **The line is box-wide.** Every grok the operator runs then writes OSC 7501
  in every terminal, including herdr panes. A terminal that does not know an
  OSC is meant to consume it unseen (ECMA-48 string controls). That herdr's
  emulator does so, and that herdr's own grok detection still settles, is
  ASSUMED. The slice's shakedown measures it on both multiplexers before the
  line is required.
- grok's `blocked` is STATIC: the binary carries
  `7501;state=…:kind=permission:app=grok-build`. A live permission prompt needs
  a model turn, so it is UNPROVEN. Fleet grok runs `--permission-mode auto`.
- A turn that cannot reach the network stays `working` while grok retries
  (15 attempts, with backoff; MEASURED still `working` at retry 9, 8 min 20 s
  in). How a turn ENDS, `done` or `error`, is in grok's docs ("working, waiting
  for you, done, or failed") but UNPROVEN live: the rig cannot finish a turn
  without a model. Admission does not rest on it, because D2's argv arm accepts
  a first record of any state. Settling does, so the slice's live shakedown
  (one real turn, operator-run) is where it is measured.
- A grok killed without `/quit` leaves its last record behind. The Spike's rule
  covers this: a dead owner pid reads `unknown`.
- `runtime.go`'s `ReportsOnRex` comment repeats D3's "again forced `on`". The
  slice corrects it.
- The probe defaults to the darwin Rex app path (`REX=` overrides it). On the ADR 0080 Linux
  host the line is ASSUMED to behave the same, since it is grok's config and
  Rex's protocol, and the shakedown (ranger-base-offid) re-runs `onidle` there.

**Alternatives rejected.**
- **A grok hook, wherever it lives.** MEASURED dead: grok gives its hooks no
  terminal. That was the route this spike was filed to price, and the price was
  never reached.
- **A hook that reports through Rex's API instead of the tty** (`REX_SESSION`,
  `REX_BLOCK`). It might land, but the owner would be whoever Rex stamps for an
  API caller, not grok's own process (unmeasured). A native report needs no
  hook, so this is not worth measuring.
- **A per-seat `GROK_HOME`.** Five stores moved or faked for one line (priced
  above).
- **`GROK_CONFIG` / `GROK_CONFIG_PATH`.** MEASURED: the overlay drops both
  `ui.*` and `hooks`. That is by design, since the overlay "cannot spawn
  commands".
- **Spoofing `TERM_PROGRAM`, so grok skips its version query and falls back on
  `REX_SESSION`/`REX_BLOCK` under `auto`.** This was the clever one. It lies to
  grok about the terminal, which also drives its rendering and keyboard
  choices, and it rests on an undocumented list of terminals. Not measured.
- **`managed_config.toml` in `$GROK_HOME`.** grok's console sync owns that file
  ("console-synced"), so a line there can be overwritten by xAI.
- **Waiting for upstream to fix `auto`.** The leak and the silence under `auto`
  are one grok bug: Rex's reply to the query arrives after grok has stopped
  listening ("dropping stalled terminal reply fragment" is in the binary), so
  `auto` stays off and the reply's tail reaches the composer. That is worth
  reporting upstream, but `on` does not need the fix.

**Measured versus assumed.** MEASURED (this box, 2026-11-06): every row of both
tables, `inspect`'s `configWarnings` reading, and the absence of a value in
`inspect --json`. STATIC (grok 1.0.52's shipped docs and binary strings): the
config layers and the overlay allowlist, `blocked`'s report string, the stalled
reply message, and the absence of any `GROK_*` variable for `program_status`.
ASSUMED: herdr consuming the OSC unseen, the Linux behaviour, and
the credential file's rename-on-refresh (inferred from its temp files, not
traced).
UNPROVEN: `blocked` live.

## Amendment 4: bob is herdr-only on Rex; it has no argv prompt and no record before one (ranger-base-778y0, 2026-11-06)

**Context.** D11 kept bob refused and left four questions: (1) does the
top-level `bob -p <prompt>` open the interactive client with that prompt
submitted, so bob could be `prompt: argv`; (2) if so, does a workspace hook's
OSC 7501 report land on that first turn, owned by bob's node process; (3) can
the operator's global herdr-bob hooks be kept from firing in a fleet seat
without writing global config; (4) bob has no `blocked` and no clear-on-exit
event, so is the dead-owner-pid rule enough? D11 said measuring (1) costs a
billed turn. It does not. The operator authorized up to three
(ranger-base-57oel). **None was spent.**

D11 also missed that (1) had been answered already, on bob 2.0.5:
`runtime.go`'s note above `BobCommand` records `-p … chat` delivering
nothing and top-level `-p` going headless (ranger-base-5jjtn, MEASURED
2026-10-01). This amendment re-measures it on 2.0.9 and reads the reason out
of the bundle.

**STATIC, bob 2.0.9 (commit 9690f20b0), `dist/bob.js`:**
- The top-level action picks `run` (headless) when a prompt is given or stdin
  is not a tty, and `chat` otherwise (`dFl`: `!process.stdin.isTTY || !!prompt`).
- `chat` parses its options through a zod schema with no `prompt` field
  (`GBl = MQ.extend({format})`), and zod drops keys it does not know. A global
  `-p` reaches `chat` through `optsWithGlobals()` and is discarded there. Only
  `run`'s schema carries `prompt` (`QBl`). No `BOB_*` variable carries one.
- The hook events are `SessionStart, UserPromptSubmit, PreCompact,
  PostCompact, PreToolUse, PostToolUse, Stop`. 2.0.7's list plus the two
  compaction events. There is still no permission, interrupt or session-end
  event, and `SessionStart` still fires inside the first turn's input path,
  not at launch.
- Hooks run through `child_process.exec`, in bob's own process group.
- `disableGlobalHooks` is read from the user config only. A workspace
  `.bob/settings.json` contributes `hooks` and `disableWorkspaceHooks`, and
  nothing else the hook manager reads.
- bob's whole home is `os.homedir()/.bob`, so `$HOME` moves all of it: the
  global hooks, the login, the task database and the folder-trust list live
  under that one directory.

**MEASURED 2026-11-06**, this box (darwin 25.6.0), Rex CLI 0.0.1793377904,
bob 2.0.9, node v26.12.1, `docs/adr/0076-bob-argv.probe.sh`. Arms `chat`,
`p` and `pchat` cannot make a turn: each has its own scratch `HOME` (no
login), bob's key variables unset, and every proxy at `127.0.0.1:9`. Arm `live`
runs under the real login, and its workspace `UserPromptSubmit` hook exits 2,
so bob blocks every prompt before a request is built.

| arm | line | screen | hooks ran | record |
|---|---|---|---|---|
| `chat` | `bob chat --accept-license -w .` | the TUI (folder-trust menu) | — | none |
| `p` | `bob --accept-license -p <text>` | **headless**: an error asking for an API key, back at the shell | — | none |
| `pchat` | `bob -p <text> chat --accept-license -w .` | the TUI, the same as `chat` | — | none |
| `live`, untyped | `bob -p <text> chat --accept-license --trust -w .`, real login | splash, "Logged in", **an empty composer** at t+20 s | **none** | none |
| `live`, `/hooks` | workspace file also sets `"disableGlobalHooks": true` | "(12/12) active": all **5 Global** herdr-bob hooks `● active` beside the 7 Workspace ones | — | — |
| `live`, one typed prompt | posse's typed route | prompt blocked by the hook | SessionStart → idle, UserPromptSubmit → working, `tty-write=ok`, hook `pgid` = bob's node pid | **`working`**, `owner={pid,"node"}` |
| `live`, `/quit` | — | back at the shell | — | **none** (`working` dropped at exit) |

**Answers.**
1. **No.** Top-level `-p` is headless `run`, which has no interactive client.
   `-p … chat` opens the client and discards the prompt. bob has no argv route,
   so it stays `prompt: typed`.
2. **Moot for argv.** There is no argv first turn for a hook to report. On the
   typed route the hook works as it did on 2.0.7: the report lands, stamped
   with bob's node process. But it lands only *after* posse has typed into a
   composer it could not see was ready. That is the order D2's typed arm and
   the first amendment's rejected handshake both refuse.
3. **No.** A workspace file cannot switch the global hooks off (MEASURED). The
   remaining routes are writing `disableGlobalHooks` into the operator's
   `~/.bob/settings/settings.json`, which posse never does, or a per-seat
   `HOME`. A per-seat `HOME` moves the login (its refresh then forks
   from the operator's), the task database, `trustedFolders.json` and the
   skills roots. That is D4's and D13's price again, and it still buys no argv
   prompt.
4. **Enough for death, not for a live seat.** A dead owner reads `unknown`,
   and `/quit` cleared a `working` record (MEASURED). A stale `idle` with a
   dead pid is the case the Spike's rule already catches. A *live* bob is a
   different problem: it has no permission event, so a seat waiting on an
   approval reports `working`, which is a wrong state and not an unknown one.
   Fleet bob runs `--auto-approve`, which makes that rare for a dispatched seat
   ("when safety rules allow"). It is still a gap posse could not detect.

**Decision.**

**D14. bob is herdr-only on Rex.** It replaces D5 and D11. `ReportsOnRex`
stays false for bob, and D1's refusal before the claim, which names herdr, is
the whole behaviour. No code changes. The reason is answer 1. Nothing writes a
record before a prompt, and bob takes no prompt except one that posse types.
ADR 0013 §1 needs positive evidence before posse types, and §3 forbids the
screen rules that herdr-bob uses to supply it. Answers 3 and 4 are separate
costs on top of that. Either would be priced if answer 1 changed. Neither can
rescue the refusal on its own.

ADR 0080 D3's "interchangeable for every runtime" therefore excludes bob. A
bob crew on the ADR 0080 host runs on herdr beside Rex. 20 of 20 PIDs on this
instance run claude (D4's count), so nothing that runs today is affected.

**Reopen when** a bob release carries all three:
- an initial prompt that `chat` submits (an option on `chat`, or `-p` honoured
  there);
- a session-scoped switch for global hooks, such as a flag or an env var. The
  alternative is the operator accepting that herdr-bob's hooks fire in fleet
  seats;
- for `blocked`, a permission event. Without one, `--auto-approve` remains the
  only answer.

The re-measurement is this probe's `pchat` and `live` arms on the new version.
Neither spends a turn. If both turn positive, the slice follows codex's shape
(D6, D9): posse's reporter as the workspace hook's handler, a version guard,
and D2's argv arm as the confirmation.

**Alternatives rejected.**
- **Type the first prompt blind and admit on the hook record that follows.**
  It works mechanically (the `live` arm's typed row). It is the first-prompt
  handshake the first amendment rejected: posse types into a composer it
  cannot see is ready, and a refusal could come only after a claim, a
  worktree and a typed turn.
- **A composer-ready screen rule on Rex, only for bob.** It is small and would
  admit bob. It is also a second detection engine for one runtime, which §3
  and the first Alternatives refuse. herdr already runs that engine, and it
  runs bob.
- **Headless `bob run` as the Rex seat.** It is `argv` in the literal sense.
  But it runs one task and exits: no composer for a re-prompt or `--resume`,
  no terminal state to report, and no seat in the sense the ten verbs mean.
  Under a scratch home it demanded an API key (MEASURED). Whether it takes
  the operator's login instead is unmeasured, because measuring it would spend
  a turn for a seat model nobody has asked for.
- **`bob acp`, the Agent Client Protocol server.** This was the clever one:
  structured sessions, prompts and state updates, with no screen and no OSC.
  It is not Rex admission. It is a new kind of occupant with no terminal,
  which is a runtime-driver design and not a multiplexer slot. Nothing is
  filed for it, because no PID wants bob off herdr (ranger-base-57oel's
  recommendation, and the 20 of 20 count).
- **A per-seat `HOME`.** It moves the login and three other stores (answer 3),
  and it still buys no argv prompt.
- **`disableGlobalHooks` in the workspace file.** MEASURED: ignored.

**Measured versus assumed.** MEASURED (this box, 2026-11-06): every row of
the table, the empty composer at t+20 s, the five global hooks active under a
workspace `disableGlobalHooks`, the hook's process group, the record's owner,
and its removal at `/quit`. STATIC (bob 2.0.9 bundle): the top-level
dispatch, `chat`'s schema dropping `prompt`, the event list, `exec`, the
layer `disableGlobalHooks` is read from, and that the login lives under the same home. ASSUMED:
nothing the decision rests on. UNPROVEN: `bob run` under the real login, and
how bob's Stop record behaves after a turn that completes (it needs a billed
turn and changes nothing above). Billed turns spent: 0 of the 3 authorized.
One write outside the scratch dir: `--trust` added the `live` arm's scratch
workspace to `~/.bob/trustedFolders.json`, the same kind of entry ch2rb's bob
arm left.
