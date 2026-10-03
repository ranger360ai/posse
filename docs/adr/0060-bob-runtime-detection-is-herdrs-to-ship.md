# ADR 0060 — IBM Bob is a fourth built-in runtime; its herdr detection is upstream's to ship, and posse refuses to fake it

*Status: accepted 2026-09-28 · owner: architect · source bead
ranger-base-v1yrt (classification ruling: Bob support is posse product
work, like codex and grok; the instance-side record stays on
ranger-base-6wqe) · sits under ADR 0013 (dispatch contract, §1 grid, §2
delivery, §5 account) and ADR 0012 D4 (adapter seams) · uses the
overlay contract of 0013 §8 for the profile · measurements in
`docs/notes.d/ranger-base-v1yrt.md`*

## Context

Bob (`bobshell` 2.0.5, Homebrew, public software) is a CLI agent with an
interactive TUI (`bob chat`), a headless mode (`bob run`, which demands an API key in the
environment even after a web sign-in) and an ACP server. The recon
(ranger-base-6wqe Phase 1, 2026-09-28) filled every grid dimension but
one, and that one is the gate: **herdr 0.8.2 has no `bob` kind.** The
live process is `node /opt/homebrew/bin/bob chat …`, herdr labels the
pane nothing (`agent_status: unknown`, `agent explain` → `agent_not_found`),
and `posse prompt` refuses an unlabelled pane. Without detection the
launch row of ADR 0013 §1 reads "refuse", and every settle is a guess.

Three detection routes were on the table. This record measured all
three on this box before choosing (MEASURED 2026-09-28, herdr 0.8.2,
recipe in the notes fragment):

1. **Alias-fork of `claude.toml` (`aliases = ["bob"]`).** Dead twice. A
   locally added alias does not resolve: with an active local override
   on gemini and then on claude, each carrying `aliases = [..., "bob"]`
   and a higher version, `agent explain --agent bob` still answers
   `unknown_agent`. The runbook's alias route was measured on
   `grok-build`, which upstream's own manifest already carries — the
   binary resolves compiled aliases, not ours. And even a resolving
   alias would label nothing: the live pane got no label from argv at
   all, so there would be no label to alias.
2. **A standalone `bob.toml` override.** Still ignored on 0.8.2, as it
   was on 0.8.0 (rangerhq-tr8k): not listed by `server agent-manifests`,
   `unknown_agent` on explain.
3. **`herdr pane report-agent --source posse --agent bob`.** Works for
   what it is: the pane gets the label, `agent list` shows it, a reported
   `working` satisfies `agent wait --until working`, and a reported
   working→idle lands as `done`. What it does not give: `agent explain`
   refuses the pane ("does not have a detected agent label"), so
   `Seen()` — the readiness gate of ADR 0013 §2 step 4 — has nothing to
   read; and `agent prompt` refuses it (`agent_not_ready: not an active
   named agent`), so typed delivery would need a second path
   (`pane send-text` + `send-keys Enter`, the recon's hand path). The
   reporter itself has no home: ADR 0016 removed the event subscription
   and ADR 0011 makes readiness a per-pass level read, so a state posse
   must keep current between passes is a daemon in costume.

## Decision

**D1 — `bob` joins `builtinRuntimes` as a fourth built-in, declared from
the recon, and `posse runtime check bob` says out loud that the launch
row is unmet until herdr ships a `bob` kind.** The profile is honest,
not green. Declared facts (MEASURED unless marked):

| key | value | note |
|---|---|---|
| command | `bob chat --accept-license --trust --auto-approve -w . {allow} {deny}` | `-w .` because the pane's cwd is the session dir. The `-p "$(cat {file})"` D3 put at the head of this line is GONE — measured not to deliver, see Claims/ASSUMED 1 and ranger-base-5jjtn |
| unattended | `--auto-approve` | "approve all tool executions without prompting" |
| prompt | `typed` | 20s to a composer signed in (recon); see D3 |
| record | `untrusted` | `bd` runs inside a Bob shell tool; no dispatched close yet |
| self_sandbox | false | `sandbox-exec` wraps it |
| skills | cwd-discovery | reads `.agents/skills` from cwd — the tree posse already materializes, no flag |
| native_rules | `AGENTS.md`, `CLAUDE.md`, `.bob/rules-{agent,plan,ask}/AGENTS.md` | read from the workspace |
| project_config | `.bob/settings`, `.bob/mcp.json`, `.bob/hooks`, `.bob/custom_modes.yaml` | read from the session dir unconditionally; keys ASSUMED, the trust check stays whole-file |
| state_dir | `~/.bob` | settings, db, logs, skills |
| egress | `bob.ibm.com`, `iam.cloud.ibm.com` | live: one connection to a Cloudflare edge for bob.ibm.com; the others from the bundle |
| cage_cred | the API-key env name `bob run` demands (spelled in the built-in) | that `bob chat` honours it in a container is ASSUMED |
| model_flag / models | none | no per-launch model on `chat` or `run`; tiers are UNMAPPED, a DECLARED DIFFERENCE — the model is an instance setting |
| interstitials | license (`--accept-license`), folder trust (`--trust`), browser sign-in (`Complete sign-in in your browser…`; Esc/Ctrl+C exits; nothing typed there acts) | no `danger:` — none of the three mutates the machine on Enter |
| turn_outcome | none | see D4 |
| account | uncounted | see D4 |

`runtimes/bob.yaml` overlays it per 0013 §8 like any built-in.

**D2 — The detection route is upstream.** posse ships the filing, the
operator sends it. Concretely: `etc/herdr/agent-detection/upstream-bob.md`
in the shape of `upstream-report.md`, a draft `bob` manifest beside it
(idle: the composer line `Build Anything, @ for context`; blocked: the
`Execute Command` dialog on `Approve Once` / `Press Enter to confirm`;
blocked: the sign-in screen; working: **capture needed**, no fixture
exists — `working.txt` is byte-identical to `after-turn.txt`), and the
five redacted fixtures. The fixtures live under
`etc/herdr/agent-detection/upstream/bob/`, not `testdata/bob/`, because
`scripts/verify-detection.sh` fails every fixture of an agent herdr
cannot evaluate. One pin protects the handover: a test that asserts
`herdr agent explain --file <fixture> --agent bob` still answers
`unknown_agent`, and reds the day it does not — its message says to move
the fixtures into `testdata/bob/`, delete the draft, and run the probe.
That is the process the pin protects, and without it the day herdr
learns Bob passes in silence.

Until then Bob is a runtime posse can **launch interactively** (`posse
new`, the operator's own keyboard) and cannot **dispatch** — the launch
row refuses by name, which is what ADR 0013 §1 says a missing launch
observable does. No flag, no `--allow-undetected`.

*Snapshot 2026-09-28 (ranger-base-i3q6g): when this record was accepted
the sentence above was true of the grid and not of a launch — the
detection reading reached `posse runtime check` and `posse runtime
probe` and no dispatch path, so a dispatched Bob session would have
spent the worktree, the pane, the PID turn and a `startup_wait` before
timing out as a slow start, and the next pass's `RelaunchAgent` would
have typed the launch line into the live composer. ADR 0013 §1 now
carries the rule and its properties (a reading, never ignorance; above
the claim and before the kill; dispatched refuses, interactive warns;
the line names argv0 and this filing, never the session). The code lands
under the beads cut from ranger-base-i3q6g; `git log --grep
ranger-base-i3q6g` on main is the record, this sentence is a snapshot.*

*Snapshot 2026-09-28 (ranger-base-qa73t): "cannot dispatch" above is now
conditional. A reported label is enough to launch on — as identity, with
herdr's `pane process-info` reading as liveness, because a reported label
outlives its process (MEASURED) — when the instance declares `detection:
reported` in `runtimes/bob.yaml`, an overlayable instance fact naming the
plugin in `detection_why`. This built-in stays `detection: herdr` and
keeps saying it has no detection; D2's tripwire and the upstream filing
stand. ADR 0061 is the record.*

**D3 — The PID rides `-p` as the first user message; the work prompt is
typed.** Bob has no launch-time system channel: no rules flag, no
system-prompt flag. Its instruction files are all read from the
workspace (the project-config channel posse refuses on) or from the
operator's global `~/.bob/settings/custom_modes.yaml` (a persona =
a custom mode with `roleDefinition`, selected by `--mode <slug>`). The
program-level `-p` is consumed by the `chat` action through
`optsWithGlobals()`, and `bob chat` also takes a positional prompt
(`r.join(" ")` when `-p` is unset) — but only one of the two is used, so
PID and work prompt cannot both be argv without a concatenation trick
(rejected below). `-p` goes BEFORE `chat`: the program enables
positional options, so a `-p` after the subcommand is an unknown option
the program is configured to swallow silently. Price, stated: one Bob
turn per launch is spent on the PID, ~0.021 of Bob's own cost unit per
turn on the measured turns (see D4), and the model's reply to it is
noise. Trigger to revisit: a system-level channel that is neither the
session dir nor the operator's global settings.

*Superseded 2026-10-03 (ranger-base-f1ytb, ADR 0062): the trigger fired
and the premise was wrong twice. `-p` never delivered (measured
ranger-base-5jjtn), and Bob does have a launch-time system channel —
`bob chat --mode <slug>` selects a custom mode whose `roleDefinition` is
the `role_definition` section of its own system prompt, and modes load
from the WORKSPACE (`<ws>/.bob/{custom_modes.yaml,plugins/*/custom_modes.yaml}`,
`-w .` being the session dir) as well as from the home. The rejection
below priced only the home variant. ADR 0062 D1 renders the PID as a
hidden mode under `<session dir>/.bob/plugins/posse/`, the skills tree's
sibling (ADR 0007), and `BobCommand` carries `{mode}`; D2 reads the footer
before typing, because an unknown slug falls back to Agent with one grey
line (MEASURED). The price D3 stated — one Bob turn per launch spent on the
PID — is gone with it.*

**D4 — No cost adapter and no turn-outcome reader now; both triggers
named.** The unit behind `tasks.costs.cost` and the pane footer is
**Bobcoins** (the status screen labels it `Bobcoins:` with the `🅞`
glyph, beside `Monthly Budget` and `Monthly Usage`), not dollars. An
adapter would therefore be the codex shape — `Prices() == false`, turns
and a running coin total, a blank in the money column — and it needs a
`sqlite3 -readonly -json` shell-out (a tool `backup.go` already
requires). Until a Bob session can be dispatched there is no bead
segment to attribute, so the adapter earns nothing today; the account
row reads UNCOUNTED and `uncounted_cap_bob:` is the brake (0013 §5).
Trigger: the first dispatched Bob close. Shape when built: a `cost_*.go` adapter for Bob
beside `cost_grok.go`, segment by the `Work beads issue` user message,
sum assistant `_meta.spend.cost` per segment. A turn-outcome reader
joins the registry only after Bob's own refusal artifact is captured
(0013 §1 settle row); nobody has one. The likely home is
`tasks.last_error`; capture-when-it-happens is on ranger-base-6wqe.

**D5 — Typed prompts to Bob never start with `/`.** The picker eats
Enter and the composer keeps the text through Esc and C-u. The work
prompt never does; this is a hazard for `posse prompt` operators and is
documented in the profile's `runtime check` notes, not guarded in code —
there is no caller until detection lands.

*Snapshot 2026-09-28 (ranger-base-8eqaa): the caller arrived, from the
route this record measured half-alive and declined to build. The
MartinLoeper/herdr-bob PLUGIN is the reporter — third-party, installed
by the operator, so the "daemon in costume" posse would have had to run
is somebody else's process (ranger-base-p8afi). That leaves posse one
gap, and it is exactly the one Context 3 named: `agent prompt` refuses a
reported pane (`agent_not_ready`), so typed delivery goes in by `pane
send-text` + Enter. It is keyed on the PANE, never on this runtime's
name — a label herdr has no manifest for, asked of herdr — so there is
no second name-keyed mechanism and nothing to prune the day D2's
tripwire fires. The `/` hazard is now refused in code on that route as
well as printed in the grid; the grid line stays, because the operator
reads it before they ever reach a refusal. `git log --grep
ranger-base-8eqaa` is the record, this paragraph is a snapshot.*

## Consequences

- Built: one built-in entry plus its declared tables; one filing package
  and one tripwire pin; the fixtures move public. Nothing new in the
  dispatcher, no new config key, no new actor, no new flag.
- Not built, on purpose: the alias fork (dead), the reporter (a daemon
  in costume), the cost adapter and turn reader (no numbers yet), a
  generalized sqlite reader (a second registry with one member).
- The parity TRACK (ranger-base-gz8h) reads Bob's grid as UNRECOGNIZED
  on launch and UNCOUNTED on account, both loud, until upstream lands.
  Everything else in the grid is declared.
- The instance side (6wqe) owns: the probe after herdr recognizes Bob,
  the working-state capture, the refusal artifact, and the `-p` delivery
  measurement (ASSUMED here that a `-p` value is submitted as the first
  turn at startup; the recon read the flag's help text, not a turn).

## Alternatives rejected

- **Do nothing** (Bob stays a yaml draft in the private tree). Priced:
  zero build, but the operator ruled Bob product work and the grid
  would keep saying "no profile" instead of the true "no detection" —
  the honest gap is one row, not the whole runtime.
- **Alias-fork of claude.toml.** Measured dead (Context 1). Had it
  worked, the price was owning claude's detection for the default
  runtime's eleven seats, and Bob's rules evaluating on every claude
  pane.
- **report-agent adapter.** Measured half-alive (Context 3): label yes,
  explain no, prompt no, and no process to run it in without a daemon.
  Two mechanisms (a reporter and a send-text delivery path) for one
  runtime, both keyed on its name — the ADR 0017 §3 shadow-predicate
  class.
- **Standalone `bob.toml`.** Measured ignored.
- **PID and work prompt joined into one first turn** (the clever one:
  PID as `chat`'s positional, dispatch appends the work prompt, the
  bundle joins them with a space). One turn instead of two, argv
  delivery — and it rests on a `join(" ")` in a minified bundle whose
  drift would silently drop the work prompt, the at-most-once-into-a-hole
  failure ADR 0013 exists to remove. Revisit only with a pin that reads
  the bundle.
- **PID as a custom mode** in the operator's global
  `custom_modes.yaml`. A real system-level channel, and the mode's
  `groups` are even an L0 realizer surface (tool permissions). Price:
  posse writes and rewrites a file the operator also edits (single
  writer, ADR 0022), every persona appears in their `/mode` menu, and a
  stale mode outlives the session. File it the day a PID-as-first-turn
  measurably misbehaves.
- **Build the cost adapter now.** Coins are not dollars, so it would
  flip UNCOUNTED to UNPRICED and nothing else, with no dispatched
  session to attribute to.
- **Fixtures in `testdata/bob/` today.** `verify-detection` would fail
  them on every run for an agent herdr cannot evaluate; the pin in D2
  is what moves them on the right day.

## Claims

**MEASURED (2026-09-28, this box, herdr 0.8.2 client and server,
bobshell 2.0.5, node 25.2.1)** — recipe and raw output in
`docs/notes.d/ranger-base-v1yrt.md`:
- A locally added `aliases` entry does not resolve (`gemini`, `claude`
  overrides active by version, `--agent bob` → `unknown_agent`); a
  compiled alias (`grok-build`) does.
- A standalone `bob.toml` is not listed and not evaluated.
- The live pane's argv is `node /opt/homebrew/bin/bob chat …`; herdr
  labels it nothing.
- `report-agent`: label and state land; `agent wait --until working`
  returns on a reported transition; working→idle reports as `done`;
  `agent explain` and `agent prompt` refuse the pane.
- `bob chat -w <dir> <positional>` launches the TUI (no argument error);
  `bob run` without a key refuses with `Bob API key is required.`
- The cost unit is labelled `Bobcoins:`; per-message `_meta.spend`
  carries `cost` and `contextTokens` only; `tasks.costs` is the running
  sum; `tasks.status` is `running` mid-turn and `active` settled.
- The recon's per-turn cost: 0.021 coins on the "ready" turn.

**ASSUMED** (each is a probe line on 6wqe):
- ~~A `-p` value is submitted as the first turn when `chat` starts.~~
  **MEASURED FALSE 2026-10-01** (ranger-base-5jjtn,
  `docs/notes.d/ranger-base-5jjtn.md` §1). `bob chat` reads the
  program-level `-p` back through `optsWithGlobals()` and IGNORES it: the
  pane holds the splash and an empty composer at 5/10/20/30/45s, no reply,
  and no lifecycle hook fires. Top-level `-p` without `chat` is headless and
  key-gated (`Bob API key is required.`), and posse cannot reach it anyway —
  `EnsureUnattended` appends `--auto-approve`, which top-level bob refuses.
  So **no argv shape on 2.0.5 opens the TUI with a prompt already
  submitted**, D3's revisit trigger has fired, and `BobCommand` now carries
  no `-p` and no `{file}`: bob has no PID channel, stated rather than
  hidden. D3's reasoning about argv ORDER was correct and moot — the order
  mattered only if the flag delivered. On a program built with
  `.allowUnknownOption()` a swallowed flag and an accepted-then-ignored one
  look identical from outside, which is the whole reason this cost a probe;
  INSTALL's pair probe only ever proved parsing. Which channel replaces it
  is this ADR's to amend, and both candidates are already priced in
  Alternatives rejected (PID typed as the first message; PID as a custom
  mode — "file it the day a PID-as-first-turn measurably misbehaves"), and
  it is filed as ranger-base-f1ytb.
- `bob chat` honours the API-key env name in a container (read from the
  env name list in the bundle, never exercised).
- The project-config key names; the trust check stays whole-file until
  the file shapes are read.
- The sign-in screen discards typed text (it exits on Esc/Ctrl+C; nothing
  else was typed at it).

## Verification (the closer's observables)

1. `posse runtime check bob` exits 1 with the launch row naming herdr
   non-recognition and every other row declared; `runtimes/bob.yaml`
   overlaying `startup_wait:` shows as `runtimes/bob.yaml (startup_wait:)`.
2. `posse new --runtime bob` renders `chat` with `--auto-approve` on the
   line and **no `-p`, `--prompt` or `$(cat …)` anywhere on it** — the
   observable is inverted from "`-p` before `chat`" by Claims/ASSUMED 1
   (ranger-base-5jjtn): the flag delivers nothing, so the ordering it was
   written about bought nothing, and the old row was satisfied on every day
   the channel was dead.
3. The tripwire pin is green today and its failure message names the
   move into `testdata/bob/`.
4. `make verify-detection` passes with the fixtures where D2 puts them.
