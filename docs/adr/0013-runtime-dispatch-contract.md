# ADR 0013 — Runtime dispatch contract: what a runtime must guarantee to take work

*Status: accepted 2026-08-24 · owner: architect · amends 0003 §1
display, 0010 §1/§5, 0012 D4.1*

> ADR 0002 answered "can a persona *launch* safely on any runtime." ADR
> 0012 D4 answered "can a third engine be *added* without patching the
> harness." Neither asked whether a launched persona can *reliably take
> work*. One evening of two non-claude runtimes in production produced a
> new failure roughly every hour, each patched ad hoc (ranger-base-9fq).
> This is the dispatch-lifecycle contract. It does not re-open the cage.

## Context

Dispatch assumes a claude-shaped life: the CLI starts, herdr *sees* a
screen, a prompt is typed, the model follows `Done:`, `posse cost` can
price the turn, and the plan guard's meter is the meter the pass spends.
Each clause is an assumption wearing a universal name.

Measured 2026-08-24, on beads:

| # | assumption | what broke |
|---|---|---|
| 1 | a launched runtime is promptable | three first-run interstitials (privacy banner, version splash, update menu whose default is `brew upgrade`) made herdr report only `idle` / `default_known_agent_idle_fallback` (ranger-base-3j8) |
| 2 | 45s is enough, and a failed launch benches the persona | grok's cold start exceeds 45s on a *clean* screen; one miss burns the persona's busy key for the whole pass |
| 3 | `Done:` is acted on | 3/3 codex sessions did the work and skipped the bead; one left a dirty shared checkout (ranger-base-0fb). Claude and grok complied the same evening |
| 4 | spend is visible | `posse cost` reports codex/grok **uncounted** (ADR 0003 §4 already requires this honesty; it does not put a brake on the channel) |
| 5 | the plan guard gates the meter being spent | it gates the *pass* on one provider's windows, including when blind, so an off-meter drain was parked by an unreadable on-meter credential (ranger-base-ri4) |
| 6 | `tier: strong` means a strong model | `{model}` renders empty on grok/codex; the catalog preflight is Anthropic-only and cannot see allotment exhaustion (ranger-base-1cc) |
| 7 | the PID is the instruction | grok and codex natively load `AGENTS.md` (and kin). That file already collided with a PID deny (rangerhq-cmfj). Whether that is the bookkeeping defect is untested |

ADR 0012 D4.1 currently says a runtime "starts idle awaiting a typed
prompt." That sentence is a *delivery method*. It became the dispatch
path. The idle-fallback is a liveness guess over an unrecognized screen
(`liveness-and-identity`); a prompt typed into it is at-most-once
delivery into a hole (`delivery-and-idempotency`).

Concepts in play: **liveness vs identity** (herdr `idle` is not "promptable"),
**single-writer / store of record** (the bead, not agent settle, is work's
truth — ADR 0011), **at-least-once + claim-as-fence** (the work prompt is a
message; the claim is what makes a retry safe), **TOCTOU** (a failed pane
read as "this persona is busy").

## Decision

A runtime is **dispatchable** when it can be declared against six stages.
Missing a stage is not a patch: it is a named degrade or a named refuse.
A new runtime is onboarded by filling the grid, not by discovering each
quirk in production.

```
launch → promptable → work → record → settle → account
```

Interactive `posse new` is unchanged: idle, awaiting a typed prompt.
This contract is what **dispatch** requires of that same process.

### 1. The grid — observable, who declares, missing →

| stage | observable | declared by | missing → |
|---|---|---|---|
| **launch** | argv0 herdr recognizes; PID delivered; unattended flag on the line | runtime template + herdr manifest (ADR 0002 / 0012 D4.1–3,6) | **refuse** the launch |
| **promptable** | the work prompt is the first user turn, *without* posse answering a dialog | runtime `prompt: argv` (preferred) or `prompt: typed` + `startup_wait:` | **refuse this launch**, loudly; see §2 |
| **work** | herdr `working` then a settled state | herdr detection (already) | wait ladder as today (NOTES §6–7); a timeout is a check-in, never an unclaim |
| **record** | bead `closed`, or a comment plus an ASK/question that takes it out of `bd ready` | runtime `record: trusted\|untrusted` (§4) | settle-without-record is **incomplete**, never ✓; unattended `--resume` re-prompts; see §4 |
| **settle** | herdr `idle`/`done`/`blocked` *Seen()* — a matched rule, not the idle-fallback | herdr (already) | existing ignorance path: claim kept |
| **account** | a cost-adapter reading, or an explicit uncounted cap | adapter (ADR 0012 D4) or config `uncounted_cap_<runtime>:` | **account-degraded**: loud every pass; dispatchable; the cap is the brake (§5) |

`posse runtime check <name>` prints this grid for a runtime. Unknown is
the expensive column to get wrong: a template-only yaml with no
declarations is `prompt: typed`, `record: untrusted`, uncounted, unmapped
tiers — dispatchable and noisy, not silent.

No new template placeholder. `{prompt}` is rejected: an unrendered token
is a literal argv (ADR 0001/0002 lesson). Dispatch that has a prompt file
appends `"$(cat <file>)"` to the already-rendered line when `prompt:
argv`. Interactive launches append nothing.

### 2. Promptable — argv-first; never keystroke a machine-mutating menu

**Delivery.** All three built-in CLIs accept a positional prompt that
starts an *interactive* session (MEASURED via `--help` 2026-08-24:
`claude [prompt]`, `codex [PROMPT]`, `grok [PROMPT]`; each also has a
headless flag, `-p`/`exec`/`-p --single`, which this is not). Dispatch
uses that surface:

1. **Claim first** (the fence). A lost claim creates nothing.
2. Write the assembled work prompt (ADR 0005) to a per-bead file under
   `$RHQ_HOME/state/`.
3. Create the session with `"$(cat <file>)"` on the launch line.
4. Await a *Seen()* state, not "idle enough to type at." The prompt is
   already delivered; we are waiting for work to start, not for a
   composer to appear.
5. Create-fails-after-claim → unclaim (same cleanup as today's prompt
   failure).

Resume into a live session stays `agent prompt` (typed). `prompt: typed`
runtimes keep today's create → await promptable → claim → prompt, with a
per-runtime `startup_wait:` (default 45s).

**ASSUMED, and a probe may falsify:** argv skips the welcome/update
interstitial while remaining a herdr-detectable TUI. If the probe says
no, that runtime stays `prompt: typed` plus a *measured* `startup_wait:`;
it does not grow a keystroke table.

**Interstitials, three layers, cheapest first:**

1. **Sidestep** — argv (above). The screen is not the delivery channel.
2. **Instance config** — operator-owned facts posse documents and never
   writes: grok coding-data consent (already an operator click;
   `[Opt in]` donates private-repo prompts — a visibility line), grok's
   version pin (`etc/grok/version-pin.toml`, `[cli] auto_update = false`),
   codex skip-until-next-version (`~/.codex/version.json`). A first-run
   dialog whose **default action mutates the machine** is a launch
   **refuse** until that config silences it. Nothing blind-sends Enter.
   The coordinator's string-match Escape watchdog is a stopgap and is
   not the architecture.
3. **Declared keystrokes** — last resort, keyed on a herdr *rule id*
   (today: `startup_splash` → Esc only). Keys are pressed once. Enter is
   not in the table.

**Busy key.** Dial F already gives each bead its own session. A failure
of *this pane* is not a fact about the persona. Split:

- **session failure** (un-promptable, splash, this startup timed out) →
  slot stays free; the next bead gets a fresh Dial F session.
- **persona/runtime failure** (exe missing, auth, wall refuse) → bench
  the slot for the rest of the pass, as today.

Tonight's "one miss sterilises the queue" is the first arm wearing the
second arm's consequence.

### 3. Plan guard — the meter gates the beads that spend it

Amends ADR 0010 §1 and §5. The pass **always runs**. The reading is
still taken (shared snapshot, same TTL). The skip moves per bead:

```
off the guarded meter                    → launch, even if the guard is blind
on-meter and blind                       → skip this bead (park; never overflow)
on-meter and over threshold              → ADR 0010 ladder (overflow / skip)
```

A grok-only drain is no longer parked by an unreadable Anthropic
credential. On-meter beads still fail closed when unattended and blind.
Overflow remains a judgement *on a reading*; §5's "blind never overflows"
stands. A pass whose every bead skipped is a quiet pass; `--watch`
backs off on that, not on a whole-pass skip at the top of `Run`.

The `plan_guard_blind_max: 0` escape (ranger-base-ri4) is unnecessary
once this lands and must not be the way off-meter work is kept alive.

### 4. Record — the bead is the store of record; the runtime is not

The observable is the bead (ADR 0011, NOTES step 6). Agent settle is a
hint. That was already true; what was missing is what dispatch does when
the runtime does not write the store.

- `record: trusted` — a dispatched session of this runtime has been
  *measured* to close (claude; grok via the qa lane tonight). Built-in
  default only where measured. Promotion is a yaml/built-in edit after a
  measured close, not a new store.
- `record: untrusted` — default for every other runtime, including
  today's codex. Dispatch still launches. Gather never prints ✓ on
  settle-without-close. Unattended `--resume` stays on (ranger-base-f0g)
  so a skip is retried, not parked behind a busy key. This is not
  "monica closes the bead."

**Reap guard.** A session whose bead is still `in_progress` and whose
cwd is dirty is **not killed**. The 353-line near-miss is a shared
checkout plus a reap, not a missing `Done:` line. L3's pathspec rule
already stops an unqualified commit; it does not stop `posse kill`.

**Native rulebooks.** A runtime declares `native_rules: [AGENTS.md, …]`
(grok's list is longer — `Agents.md`, `CLAUDE.md`, …). Posse does not
rewrite the operator's `AGENTS.md` (shared checkout, operator's file).
The work-prompt line "PID guardrails override repo docs" stays. Whether
a native file outranks the PID is a **probe**, not a patch; a runtime
that fails it stays `record: untrusted`. There is no flag on either CLI
tonight that silences project rules (ASSUMED; the probe says).

### 5. Account — uncounted is a degrade, not a zero

Uncounted-never-$0 stands (ADR 0003 §4). Two live spend channels with a
human eyeballing them is the hole. A runtime with no cost adapter is
**account-degraded**:

- every pass names how many launches it sent there;
- config `uncounted_cap_<runtime>:` (beads / rolling 7 days, same shape
  as ADR 0010's overflow cap) is the brake; **unset = unlimited and
  loud**, the budget_* dormancy pattern;
- filling the cost-adapter seam (0012 D4) is how a runtime *leaves*
  this column. Numbers and which meters to read are the operator's;
  the mechanism does not invent a price table.

No autonomous spending: the cap is a count of beads posse itself
launched, not a bill.

### 6. Tier — the name is intent; the mapping is declared

Amends ADR 0003 §1 display, not the three names. `strong` / `standard` /
`fast` remain "judged / building / mechanical." A runtime that does not
map a tier **does not wear it**:

- `{model}` empty stays the render;
- `posse list` / cockpit / prompt header show `grok/default`, never
  `grok/strong`;
- PID `tier: strong` on an unmapped runtime is a `posse agent check` /
  `runtime check` warning, not a quality guarantee;
- dispatch's own overflow still never moves `strong` (ADR 0010 §2b);
- an explicit `--runtime` the operator typed is their decision and
  launches.

Availability preflight is per cost/plan adapter. No adapter → no
preflight. Dead-on-arrival (allotment message, one assistant turn, idle)
is a **turn outcome**, not a catalog miss (ranger-base-1cc already
shipped the detection half). A runtime without that probe launches what
it was asked and must say so when the first turn is a limit.

## Consequences

- `runtime.go` / yaml: `prompt: argv|typed`, `startup_wait:`,
  `record: trusted|untrusted`, `native_rules:`, existing `model_<tier>:`.
  Built-in defaults after the probe: claude `typed` (works; argv is an
  allowed later unify), grok/codex `argv` *if the probe holds*, else
  `typed` plus a measured wait; claude+grok `record: trusted`, codex
  `untrusted`.
- `dispatch.go`: claim-then-argv path; busy-key split; plan-guard skip
  is per-bead including blind; gather never ✓ on untrusted
  settle-without-close.
- `posse kill` / refresh land: dirty+open-bead reap refuse.
- `posse runtime check <name>`: the grid, one screen.
- Config: `uncounted_cap_<runtime>:`. NOTES/INSTALL: instance
  interstitial keys (document, don't write).
- ADR 0010 "Fixes in passing: off-meter launches are no longer skipped"
  was true only of a *threshold trip*. Blind is the other stop. This
  ADR is that remainder.
- Metric: `record-skip-rate` by runtime (settled-open / dispatched) —
  how a runtime earns `trusted`.

## Alternatives rejected

- **Declared keystroke lists as the architecture** (the clever one).
  Runtime yaml names screens, posse types keys. Tonight's existence
  proof is the coordinator's Escape watchdog. It races a splash
  (rangerhq-5cj/7sbo), cannot tell "Update now" from "Skip" without
  peeking, and makes Enter on an unknown menu a `brew upgrade` of the
  operator's tooling. Argv retires the screen as the delivery channel.
  Keystrokes stay last-resort and Esc-only.
- **Headless (`grok -p` / `codex exec` / `claude -p`).** No splash by
  construction. Also no herdr agent, no peek, no wait ladder. Oversight
  lives in herdr (DIRECTION). A headless runtime is a different product.
- **Refuse to dispatch any runtime that is not claude-shaped until a
  full conformance suite is green.** Purity. The qa lane on grok already
  closed work properly tonight; parking that is the opposite of
  degrade-loudly. Unknown starts noisy and untrusted, not forbidden.
- **Pre-answer interstitials from the harness** (write
  `privacy_banner_acked`, write `version.json`). Coding-data consent is
  a visibility line; update-skip is the operator's pin. Posse documents
  the keys. Posse does not write the operator's CLI config.
- **Rename tier to a claude-only word.** The three names are intent;
  the lie was displaying an unmapped mapping. Honest display is cheaper
  than a vocabulary split.
- **Per-pool budget model** (ADR 0010 already rejected). Still no
  meters to feed it. `uncounted_cap_` is the substitute in beads, same
  as overflow's cap, dormant when unset.
- **`{prompt}` in the closed placeholder set.** Unrendered, it is a
  literal argument to the CLI. Append-on-dispatch does not touch PID
  `command:` templates.
- **Harness closes the bead on the agent's behalf.** Hides the defect
  and puts a human (or a heuristic) in the loop dispatch exists to
  replace. Resume-until-record is the harness's job; `bd close` is the
  persona's.
- **Silence `AGENTS.md` by writing a session copy.** The session cwd
  *is* the repo. That file is the operator's. A competing rulebook is
  declared and probed, not overwritten.
- **A fifth store for "this runtime is trusted."** Trust is a built-in
  / yaml default updated after a measured close. Derived auto-promotion
  is a store that can disagree with the bead (ADR 0011's class).

## Claims

**MEASURED**

- Three interstitials, 45s miss on a clean grok screen, busy-key
  sterilise, 3/3 codex bookkeeping skips, one dirty checkout, uncounted
  cost, plan-guard-gates-the-pass including blind, `{model}` empty on
  grok/codex, oay preflight Anthropic-only, `AGENTS.md` collision on
  claude resolved by PID (ranger-base-3j8, 0fb, ri4, 1cc, cmfj).
- All three CLIs accept a positional interactive prompt (`--help`
  2026-08-24). Grok `-p` / codex `exec` / claude `-p` are the headless
  cousins and are not this path.
- Grok splash: composer is live under the menu; Esc does not undraw;
  Enter on grok's menu is dangerous; Enter on codex's menu runs `brew
  upgrade` (rangerhq-7sbo, ranger-base-3j8).
- Grok pin already kills auto-update (`etc/grok/version-pin.toml`).
- Qa on grok closed a bead properly; that is why grok is `record:
  trusted` and codex is not.
- ADR 0010 already launches off-meter beads through a *tripped*
  threshold. Blind still whole-pass-skips. `OnGuardedMeter` already
  knows the built-ins.

**ASSUMED** (probe replaces these; implementation of argv delivery
depends on the first)

- `grok "<prompt>"` / `codex [PROMPT]` / `claude [prompt]` skip the
  welcome/update interstitial, remain a herdr-detectable TUI, and treat
  `"$(cat file)"` as the first user turn rather than a subcommand.
- No CLI flag tonight silences project-rule discovery (`AGENTS.md` et
  al.).
- Codex's update prompt has a persist-skip that is an instance config
  write, not a fleet keystroke (coordinator's working hypothesis on
  3j8).
- `startup_wait` for grok-typed fallback, if argv is falsified, is
  longer than 45s; the number is measured on the probe, not guessed here.
- Cost-adapter internals for grok/codex exist behind 0012 D4 and are
  not designed here; `uncounted_cap_` is the brake until they do.

**Unverified (needs the probe bead):** every ASSUMED line above, plus
whether argv-delivered work still trips herdr `working` so the wait
ladder can see it.
