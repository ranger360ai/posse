# ADR 0032 — Engine onboarding: what a work-authorized engine actually requires

*Status: superseded 2026-09-05 by ADR 0013 · ADR simplification, operator ruling 2026-09-05.*

The surviving decision is in [0013 — current contract](0013-runtime-dispatch-contract.md), Decision §9 and the existing runtime grid. This page keeps its number and dated evidence; the body below is historical, not current policy.

## Historical record (superseded in full)

*Status: accepted 2026-08-22 · bead ranger-base-ucv · richard · builds on
ADR 0002 (gates), 0009 (gate shell), 0012 D4 (runtime contract) ·
re-landed 2026-08-28 under a free number, bead ranger-base-gtxw ·
amended 2026-09-05 (§1 rule 1 and verification 1: the record's CLI path
is the one the SESSION resolved, and drift is compared like for like;
ranger-base-385x)*

> Restated from the private archive of the instance this harness was
> developed in, where it was accepted 2026-08-22 as its ADR 0017. The
> seed that created this repo carried ADRs 0001–0012 and left it behind,
> and this repo's own ADR 0017 is the unrelated runtime-equivalence ADR
> — so it re-lands here renumbered, text otherwise verbatim (including
> the archive's own later rhq→posse command rename). Until the
> rangerhq-66e2 citation sweep lands, code or notes reading "ADR 0017
> §1" beside `posse runtime probe`, `probe.json`, or parity's
> assumed-until-probed mean THIS document, not 0017.

## Context

M2 (ranger-base docs/milestones/work-install.md) needs "different
inferencing engines that are work-authorized." ADR 0012 D4 fixed the
route — declarative `runtimes/<name>.yaml`, no Go plugin API — and cut
yaml v2 (rangerhq-2v2s) and the runtime preflight (rangerhq-tr8k). Three
questions stayed open; this ADR answers them: (1) what a template profile
honestly promises for a CLI the harness has never seen — which gates
degrade, and whether the wall refuses or warns; (2) where the code line
sits — what a genuinely new engine needs code-side and how much
generalizes; (3) the API-only case. Whether the work engine is a CLI or
an endpoint is unknown until ranger-base-afl answers — both shapes are
covered here, neither assumed.

Facts from the current code, each *measured* unless marked *assumed*:

- Parity refuses a launch whose gates it knows it cannot realize;
  `--allow-degraded` waives (never at tier fast) and marks the session
  (parity.go:334–345). For a template runtime that means: `Edit`/`Write`
  denies refuse below `cage: seatbelt` (no native `Enforced` credit,
  parity.go:167–179); `WebFetch`/`mcp__*`/tool-name denies refuse below
  `container` (parity.go:180–189); `egress:` likewise — and the
  container tier's inner gates and egress route are not built
  (cage.go:71–74), so today those refuse at every tier.
- `Bash(…)` denies are counted **realized** by L1 + the gate shell on
  *any* runtime that does not declare `gate_shell: false`
  (parity.go:148–166). For the built-ins that is probe-backed (ADR
  0009's argv table, rangerhq-e43). For a novel CLI it rests on three
  unmeasured behaviors: (a) child commands inherit the typed-line env —
  a CLI that scrubs or rebuilds PATH before exec defeats the shim; (b) a
  runtime that re-execs a login shell resolves it from `$SHELL` (or a
  documented equivalent like `GROK_SHELL`) — one that hardcodes
  `/bin/zsh -l` reintroduces the path_helper demotion with no wrapper in
  front; (c) it invokes the shell in argv shapes the wrapper parses.
  (c) fails **loud** by design (ADR 0009 §1). (b) fails **silent** — it
  is exactly the day the fleet believed L1 held on grok and it did not.
- `{allow}` and `{deny}` render to nothing on a template runtime
  (runtime.go:7–10): no L0 politeness. Safe by construction; the model
  discovers denies by hitting the wall, visible in `refusals.log`.
- `Unattended` is deliberately empty for template runtimes
  (runtime.go:104–120): the flag belongs in the hand-written `command:`,
  and nothing verifies it. The failure mode is a session blocked on an
  approval dialog nobody is watching — liveness, not safety.
- Dispatch needs herdr to classify the pane: manifest keyed on argv0,
  local override dir `~/.config/herdr/agent-detection/<agent>.toml`
  (etc/herdr/agent-detection/README.md). *Assumed*: an override file can
  introduce a wholly new agent name, not just amend a known one — tr8k's
  `posse runtime check` must verify via `herdr server agent-manifests
  --json`. If upstream accepts only known names, what is lost is
  dispatch, not `posse new`, and the fix is an upstream ask.
- codex's config surface includes `model_providers` with `base_url` +
  `env_key` (*measured*, runtime.go:396–411); claude honours a base-URL
  override env for gateway deployments (*assumed* — verify on the day it
  matters).

## Decision

### 1. The template promise, and the one dishonest cell

What `runtimes/<name>.yaml` promises a novel engine, by gate class, at
the default cage:

| gate class | at `shims` | honest? |
|---|---|---|
| `Bash(verb …)` deny | claimed realized (L1 + gate shell; L3 for `git push`) | **no — assumed, not measured** (behaviors a–c above) |
| `Edit`/`Write` deny | refuses; realized at `seatbelt` (L2 wraps any command) | yes |
| `WebFetch`/`mcp__*`/tool names | refuses below `container` | yes |
| `egress:` | refuses (route unbuilt) | yes |
| `allow:` | renders to nothing — friction lost, safety unaffected | yes |
| unattended approval | unverified template text | no — invisible today |

So the answer to "does the wall refuse or warn" is: **it refuses
everything it knows it cannot realize, and the operator's waiver is
explicit and marked. The gap is what it wrongly believes it realizes.**
Two rules close it:

**Rule 1 — assumed-until-probed.** On a template-only runtime, `Bash(…)`
denies land in **Degraded** ("assumed, not measured — run `posse runtime
probe <name>`") until a recorded live probe exists. Same machinery as
every other shortfall: refuse by default, `--allow-degraded` waives and
marks, never waivable at tier fast. A probe record —
`state/runtimes/<name>/probe.json`: CLI path, version string, date,
observables seen — flips the claim to realized. `posse runtime check`
compares the recorded version against the installed exe and calls for a
re-probe on drift (the ADR 0002 verification-7 discipline, mechanized).

*(amended 2026-09-05, ranger-base-385x: "CLI path" is **two** paths,
because there are two PATHs. The probe's pane is a child of the herdr
daemon and resolves the CLI in that daemon's environment; posse's own
process resolves in the launcher's. Until this amendment the record
wrote the launcher's answer over four observables measured on the
session's — measured: a decoy planted in front of the posse process's
PATH alone produced `passed: true`, `version: "decoy 9.9.9"`, and a
`cli_path` naming a two-line shell script that cannot launch anything,
with the drift check then comparing that decoy against itself and
reporting current. So `cli_path` is now what the SESSION resolved,
read by typing `command -v` into the probe's own pane under the launch
line's own PATH prefix, before the launch line and before a model turn
is spent; a pane that will not answer gets no record at all, because a
record that cannot name its binary is the state this rule exists to
prevent. `launcher_cli_path` is posse's own answer, kept because it is
the only side `runtime check` can cheaply re-read: the drift comparison
is launcher against launcher, and where the two disagreed at probe time
the surface says version drift on the measured binary cannot be checked
from outside a pane and asks for a re-probe after any upgrade, rather
than reporting a drift it did not measure or a currency it did not
check. A record carrying no `launcher_cli_path` was written before this
and is never current.)

**Rule 2 — the probe checks observables, not intentions.** `posse runtime
probe <name>` launches the CLI headless with a scratch PID carrying a
canary deny, and passes only when every line below is seen:

1. `command -v <canary>` inside the session resolves into
   `gates/<p>/bin` (behavior a+b hold).
2. The canary verb refuses and lands in `refusals.log`, reached three
   ways: direct, `sh -c '<canary> …'`, and through a script/Makefile
   (the subprocess shapes ADR 0009 verified for grok).
3. The turn that ran them completed with nobody approving anything —
   the template's unattended flag holds, which is how the invisible row
   in the table above becomes visible.
4. herdr classified the pane as the runtime's exe, not
   `agent_not_found`, and saw a real idle (`visible_idle`), not a
   fallback guess.

A probe costs one model turn on the engine being onboarded. dinesh
shapes the mechanics; these observables are the contract.

What no probe can see, so the onboarding doc must ask: **what does this
CLI read from the session directory unconditionally?** `ProjectConfig`
empty means "posse types no trust flag", which for a novel CLI does not
prove it takes nothing from cwd. A checklist question, answered per
engine, recorded in the yaml (`project_config:`, yaml v2) when the
answer is a path.

### 2. The code line: yaml declares limitations, never enforcement

The principle that separates yaml v2 from built-in code: **a template
may declare what a runtime cannot do (`gate_shell: false`,
`self_sandbox:`, no `skills_flag:`) or where its surfaces are
(`model_flag:`, `project_config:`, `cage_cred:`); it may never declare
that something is enforced.** Declaring a limitation is safe — the wall
compensates or the launch degrades out loud. Declaring a strength is
self-certification.

Stays code, deliberately:

- **L0 realizers and L0Spellings** — per-CLI dialect knowledge. The
  rule→flag compiler stays rejected (ADR 0002). A novel engine has no
  L0 and loses nothing the wall doesn't cover.
- **`Enforced` credit** (a native sandbox counting as the wall, codex
  `-s read-only`) — earned by code with the probe evidence cited, never
  declarable.
- **Gate-shell argv shapes** — the wrapper is generic and fails loud; a
  new shape it must *recognize* is code, and the probe is what
  discovers the need.
- **`EnsureUnattended` re-append** — a built-in guarantee. On a
  template the flag is hand-written in `command:` and the probe
  (observable 3) is what verifies it, not a guessed re-append.

Graduation path: an engine becomes a built-in by PR when someone wants
L0 politeness, `Enforced` credit, or fleet-flag guarantees. Until then
template + probe is a fully supported citizen, not a second-class one.

Deferred with the container tier, as D4 already scoped: image baking,
first-run seeding — plus one addition for the day rangerhq-9d0 lands:
template yaml must grow `api_hosts:` (the launcher knows the built-ins'
API hosts; a caged template engine otherwise cannot reach its own API).

### 3. API-only: the contract stays "a CLI in a pane" — the question is who supplies it

posse's runtime contract (D4) is a command line in a pane that herdr can
watch and posse can prompt. An endpoint does not satisfy it; something
must supply the agent loop and the terminal surface. Ranked:

- **(b) An already-authorized CLI re-pointed at the endpoint** —
  smallest adapter is *configuration*: codex `model_providers`
  (`base_url` + `env_key`, measured) or claude's base-URL override
  (assumed) targeting an OpenAI/Anthropic-compatible gateway. A
  template profile plus an env set; zero new code. Requires the CLI
  itself to be in policy, not just the endpoint — afl question.
- **(c) A third-party agent CLI work blesses** that speaks the
  endpoint — a template profile under §1–2, probe and all.
- **(d) A bespoke wrapper CLI, ours** — last resort, and if it ever
  exists it is a **ranger360.ai product in its own repo**, not harness
  code: posse stays agnostic, and a wrapper we ship inside the harness is
  ADR 0002's rejected fourth-CLI-shaped-thing resurrected. Its one
  design advantage: it can be built to pass the probe by construction.
  File nothing until afl answers — this is the expensive branch.
- **(a) posse speaks the API itself** (a new runtime *kind* with an
  in-process agent loop) — **rejected permanently.** It makes the thin
  harness an inference client holding metered credentials, rebuilds
  every gate and detection assumption bespoke, and puts autonomous
  spending inside the dispatcher.

What afl must tell us, per branch: (b) is the client CLI in policy even
when pointed at the blessed endpoint; (c) which CLIs are blessed; (d)
whether running our own binary is in policy at all.

## Consequences

- One new code bead (dinesh, after tr8k/2v2s): `posse runtime probe` +
  probe record + parity's assumed-until-probed for template `Bash(…)`
  denies. Ship the parity change and the probe together — refusing
  without offering the unlock is the alternative rejected below.
- INSTALL.md gains the probe step for template runtimes (ride-along on
  that bead).
- M1 is unaffected in substance: redeclaring codex or grok as a
  template profile (acceptance criterion 3) now includes one probe
  command, and that pass doubles as the probe's own first verification.
- M2 spec-work (ranger-base-1wx) reads §3's ranking against afl's
  answers; the a-rejection stands whatever they are.

## Alternatives rejected

- **`probed: true` as a hand-set yaml key.** Self-certification; a
  hand-maintained boolean is exactly the flag that drifted silently in
  the ADR 0009 history. The record must say what was measured and when.
- **Treat template `Bash(…)` denies as flatly unrealized** (the
  honest-sounding one). Punishes every novel engine forever with no
  path to parity, and trains the operator to type `--allow-degraded`
  out of habit — a waiver typed by habit is not a waiver.
- **Keep the status quo** (claimed realized, runbook says "please
  probe"). The claim was probe-backed for three CLIs and nobody would
  notice the fourth arriving unprobed; discipline is not a mechanism.
- **Generic `allow_flag:`/`deny_flag:` yaml keys** to synthesize L0.
  The rule→flag compiler again, and worse: L0 earns no Realized credit,
  so it is all maintenance cost and no wall.
- **A wrapper CLI shipped inside posse** for the API-only case. See §3
  (d) — product, not harness.

## Verification (laurie's checklist)

1. A scratch template yaml wrapping a fake CLI that hardcodes
   `/bin/zsh -l` for its commands (silent case b): parity shows the
   `Bash(…)` deny as Degraded/assumed; `posse runtime probe` **fails**
   naming observable 1; the launch refuses without a waiver.
   *(amended 2026-09-05, ranger-base-385x: observables 1 and 2 fail
   TOGETHER here and the checklist item is met when they do — one
   demotion causes both, since a shim that path_helper put behind
   /usr/bin never runs and so never writes a refusal to log. What must
   still HOLD is 3 and 4, which is how "the demotion was measured" is
   told apart from "the probe failed for some other reason". The live
   arm carries this: `RHQ_LIVE_PROBE_FAKE` in
   internal/posse/runtimeprobe_live_test.go, whose shim is reached by
   absolute path through the profile's own `command:` and re-execs the
   login shell itself — through PATH and through `$SHELL` it reached
   nothing, and the arm passed while accusing the probe.)*
2. codex redeclared as a template profile: probe passes all four
   observables, parity flips to realized, `posse list` shows the session
   clean — the M1 criterion-3 flow end to end.
3. Edit the probe record's version string: `posse runtime check` calls
   for a re-probe; parity is back to assumed.
4. The same PID at tier fast with an unprobed template runtime: refused,
   `--allow-degraded` not accepted (ADR 0003 §3 unchanged).
