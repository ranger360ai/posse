# ADR 0017 — Runtime equivalence: the abstraction is the checklist

*Status: superseded 2026-09-05 by ADR 0013 · ADR simplification, operator ruling 2026-09-05.*

The surviving decision is in [0013 — current contract](0013-runtime-dispatch-contract.md), Decision §7 and the existing §§1–6 grid. This page keeps its number and dated evidence; the body below is historical, not current policy.

## Historical record (superseded in full)

*Status: accepted 2026-08-26 · owner: architect · relates 0002 (launch/cage),
0007 (skills), 0012 D4 (engine seam), 0013 (dispatch contract; §5's account
promises are wired by this ADR's beads) · source bead ranger-base-il14 ·
amended 2026-08-28: §3 shadow-predicate register updated — a fourth
behavioural instance (dispatch's turn-outcome read) found and retired,
the counted-ness pair retired by the D4 cost seam; §4 gains
`turn_outcome:` and the two declarability shapes (ranger-base-ivf0) ·
amended 2026-08-30: §4's "not declarable" list is retired — all four
shipped, the last two here (ranger-base-ncxa); §1's pin test exists
(ranger-base-ncxa); §1's grid gained the five non-dispatch dimension rows
(ranger-base-bcpa) · amended 2026-09-05: §4 gains `pane_mode:`, a registry key over the pane-mode readers, and §3's codex name-keyed branch retires through it (ADR 0057, ranger-base-re4kb) · amended 2026-09-05 (ranger-base-kbhlw: §1's row count corrected — `pane_mode` became the sixth row with ranger-base-2p2cy and the "five" sentence had not moved; the number is now dated, and the rendered set is the record)*

> The operator, on the four-area parity breakdown: "make sure richard knows
> to add areas to consider when making sure runtimes are equivalent. we may
> already have this as a runtime abstraction." And then, harder: "assess the
> gaps and plug the gaps in the abstraction layer. gross but only way to be
> ready to switch on a dime." The success criterion is operational: switching
> a lane's runtime is a PID edit and a relaunch, with confidence that nothing
> silently degraded. Scoping constraint (on the track, ranger-base-gz8h):
> parity is a goal, not a hard requirement — a gap must be visible, never
> blocking.

## Context

The four areas (lifecycle / guards / dispatching / session management) were
derived from a sentence, not from the code. The code already carries the
real list twice: `internal/rhq/runtime.go`'s `Runtime` struct — 23 declared
fields, each a dimension on which two runtimes can differ — and ADR 0013's
six-stage grid. The four areas cover some of that and silently drop skills
surfaces, native rulebooks, egress, cage credentials, project-config trust,
gate-shell behaviour, and the whole account stage.

The named failure class (specimen: ranger-base-p84): a field that exists in
the abstraction and does nothing. `startup_wait:` is parsed, validated,
documented, displayed by `posse runtime check`, pinned by a test that
asserts the *getter* — and `rt.Wait()` has no production caller; dispatch
waits a global constant. A lever that reads as connected is worse than a
missing lever. This ADR is the audit of all 23 fields for that class, and
the decisions that close what it found.

## Decision

### 1. The checklist is derived, never authored

The equivalence checklist **is** the `Runtime` struct plus the ADR 0013
grid. A dimension exists when a field (or a stage observable) expresses it;
prose lists of "areas" are projections and carry no authority. Concretely:

- `posse runtime check <name>` is the checklist's one rendering. Onboarding
  a runtime — Bob included — is filling that grid; if onboarding still
  requires git archaeology, the grid is missing a row and *that* is the bug.
- Every `Runtime` field is classified, in a pinned test, as one of:
  **consumed** (a production caller changes behaviour on it),
  **display-by-design** (its contract *is* the grid: provenance and
  measured-fact fields), or **internal**. A new field fails the test until
  classified — a struct member can no longer be added without deciding who
  reads it. This is the anti-drift device: the audit table below is a dated
  snapshot; the test is the living copy.

  **Shipped** as `TestEveryRuntimeFieldIsClassified`
  (`internal/posse/runtimefields_qa_test.go`, ranger-base-ncxa). Each row
  also names the production files the classification rests on, and the test
  asserts each still mentions the field (or a named accessor) outside a
  comment — so deleting or renaming a consumer reds it without anyone
  remembering this ADR exists. What a green does NOT prove is that the
  mention is a live read: a field referenced only from dead code passes.
  Proving consumption is the per-field consumer tests' job; this test is
  the census saying one must exist. Writing it moved two rows of §3's table
  out of INERT (below).

- **The grid draws every dimension a field expresses, not only the six ADR
  0013 stages** (shipped ranger-base-bcpa). `runtime check` printed the six
  stages plus tier and rulebooks and nothing for the ADR 0002/0007
  dimensions the same struct declares — so the skills surface, the
  runtime's own egress hosts, the container credential, the repo→box
  project-config channel and the sandbox/gate-shell pair were facts the
  code knew and no screen said, which is the "missing a row" bug this
  section names. One row per dimension carries them — six at the
  2026-09-05 amendment: bcpa's five plus `pane_mode` (ADR 0057 D3,
  ranger-base-2p2cy), which renders from the pane-mode reader registry
  rather than from a struct field; the count is a dated snapshot and the
  rendered set in `runtimecheck.go` is the record — in the same `stageRow`
  shape with the same `declaredBy` provenance and §2's vocabulary
  throughout: a
  measured-to-differ dimension reads as a DECLARED DIFFERENCE (codex's
  `self_sandbox`; a `gate_shell: false` runtime), an unmeasured one as
  UNDECLARED / UNDECIDED, and the two are never spelled the same way.
  Pinned on the RENDERED row and not on the struct field (`gridRow`,
  `internal/posse/runtimecheck_test.go`): a substring assertion over the
  whole screen answers about whichever row said the word first, which is
  how the rulebooks line came to be pinned by nothing (ranger-base-qm6e).

### 2. What "equivalent" means — three verdicts, and UNDECLARED is loud, never fatal

Per dimension, per runtime, exactly one of:

- **PARITY** — measured to behave as claude does (the exercised baseline).
- **DECLARED DIFFERENCE** — measured or designed to differ, and the
  difference is *data*: a field value plus a why. codex `SelfSandbox`
  cannot nest under seatbelt; codex's cost adapter reads tokens and
  prices none of them (the example was "grok has no cost adapter" until
  grok gained one — ranger-base-0lg6/mykq); a template-only
  runtime realizes no gates. These are first-class runtimes, not failures,
  and nothing may render them as failures.
- **UNKNOWN** — nobody measured it. This is the loud state: `runtime
  check` prints it as UNDECLARED/UNMAPPED/NOT SILENCED, and the parity
  matrices (ranger-base-25fj et al.) score it as a failing cell.

Presentation rule (the load-bearing one, per the operator's scoping
constraint): DECLARED DIFFERENCE and UNKNOWN must be distinguishable at a
glance on every surface, or the goal becomes a requirement by accident of
presentation. And no verdict ever gates a claude-side change: the zero
values are already chosen safe-and-loud (ADR 0013), so an UNKNOWN degrades
noisily, it does not refuse.

### 3. The field audit — snapshot 2026-08-26, HEAD a6c81d3

Verified by grep over production code (tests excluded); each row names its
consumer or its defect. **Consumed** unless marked otherwise.

| field | consumer (production) | verdict |
|---|---|---|
| Name, Command, Builtin | identity, render, everywhere | consumed |
| Path | `runtime check` declaredBy | display-by-design |
| Realize | agents.go render, parity.go Enforced | consumed |
| Models / ModelFlag | `{model}` render, modelavail preflight, tier grid | consumed |
| NoGateShell | gates.go, cageinner.go, parity.go | consumed |
| Skills / SkillsCwd | agents.go, skills.go materialization, parity.go | consumed |
| SelfSandbox | parity.go, herdrback.go seatbelt gate | consumed |
| ProjectConfig / -Keys | parity.go trust check | consumed |
| Unattended | EnsureUnattended (agents.go) | consumed; yaml-declarable since ranger-base-ncxa |
| CageCred | cage.go CageCredential | consumed (built-ins via a side map, cage.go:385) |
| Egress | egress.go allowlist, modelavail predicate | consumed |
| Prompt | dispatch.go:1569, herdrback.go argv path | consumed |
| ~~**StartupWait**~~ | dispatch.go `agentWait` (per-runtime, overriding the pass default), promptready.go, runtimeprobe.go | **consumed** — the p84 inertness retired by ranger-base-ze9p, which split the detection patience from the relaunch grace |
| Record / RecordWhy | dispatch recordClause, gather ✓ suppression; why is provenance | consumed / display-by-design |
| NativeRules | `runtime check` rulebooks line | display-by-design — *decided* in ranger-base-00f ("declare, don't suppress"); not a defect |
| ~~**Interstitials**~~ | grid + probes, and `DangerLine` on BOTH launch paths (dispatch.go, herdrback.go) plus runtimepreflight.go | **consumed** — the ADR 0013 §2 refuse is enforced, not printed |
| **CostAdapter / Counted()** | `runtime check` only. The *real* counted predicate is hardcoded `s.Runtime != "claude"` in cost.go:349 and cockpit.go:218 | **INERT + shadow predicate** — a runtime gaining an adapter, or Bob, changes nothing |

Adjacent inert promise, same class: **`uncounted_cap_<runtime>:`** is
printed by `runtime check` as "the brake" and consumed by nothing — no
ledger counts uncounted launches, nothing brakes, and no pass names how
many launches went uncounted (dispatch's launch line does not even print
the runtime). ADR 0013 §5's account stage is, today, entirely display.

**The shadow-predicate rule** that falls out: per-runtime *behaviour* keyed
on `rt.Name` is allowed only where the behaviour is inherently that CLI's
own state (SeedCageHome writing `~/.claude.json`, trust.go seeding claude's
dialog). A name-keyed branch that implements a *dimension* — counted-ness,
trust-ness, sandbox shape — must go through the declaration, or the
declaration is scenery. cost.go/cockpit.go violate it; trust.go and the
skills state dir (`state/skills/<p>/claude`, a claude-plugin-shaped tree
whose name leaks into every runtime's path) are noted and accepted.

**Register update (2026-08-28, ranger-base-ivf0).** The snapshot's grep
missed a fourth behavioural instance, found only when ranger-base-unzn
drove production `Run` once per *runtime* instead of once per bead:
dispatch's turn-outcome read was guarded by `p.runtime ==
DefaultRuntime`, so the same stubbed account refusal that stops the
pass on claude was never asked for on codex/grok, and an exhausted
account there printed as an ordinary settle wearing the record-degrade
explanation (ranger-base-02zr; measured: asked 1× on claude, 0×
elsewhere). Retired through `turn_outcome:` and the turnfailure.go
reader registry (ADR 0013 §1, settle row); the seam also fixes who may
say "blind" — a test-injected reader is the *reader*, never the
permission, so a stub cannot grant a runtime a reading production
would not do. The counted-ness pair (cost.go/cockpit) was retired the
same week by the ADR 0012 D4 cost seam (ranger-base-k7nb; grok gained
a real adapter), and the per-pass uncounted line plus the
`uncounted_cap_` brake shipped in ranger-base-9mz — so of §6 item (2),
what remains is only the create line naming the runtime
(ranger-base-pjoy's remainder). The name-keyed sites still standing
(cage.go seeding, credential paths, trust.go's claude dialog) are the
accepted CLI-own-state class. Method note for the next audit: grep
found three instances; the consumer-driven parity fixture found the
fourth. The living copy of this register is a fixture that drives
production per-runtime, not the grep.

### 4. Declarability — what Bob can and cannot put in the grid

`runtimes/<name>.yaml` today takes: `command:`, `model_<tier>:`,
`model_flag:`, `skills_flag:`, `egress:`, `cage_cred:`, `gate_shell:`,
`prompt:`, `startup_wait:`, `record:`/`record_why:`, `native_rules:`
(since this snapshot, also `state_dir:`, `env_required:`, `turn_outcome:`
— added 2026-08-28, ranger-base-02zr — and `skills_cwd:`,
`self_sandbox:`, `project_config:`/`project_config_keys:`, `unattended:`,
the four below — and `pane_mode:`, a registry key over the pane-mode readers,
ADR 0057, 2026-09-05). `runtimeYamlKeys()` is the whole surface and the list to
grep; this sentence is a projection of it.

**Was not declarable; all of it now is** (each was a Go field a built-in
set that a yaml runtime could not — the grid had holes exactly where Bob
would need it). Kept as a closed list rather than deleted, because the
snapshot is what dates the claim:

- `skills_cwd:` — cwd-discovery skills (the codex/grok shape); a yaml
  runtime was either `skills_flag:` or no surface. **Shipped** — and
  declaring both surfaces refuses at load ("a runtime has one skill
  surface").
- `self_sandbox:` — a self-sandboxing yaml runtime was seatbelt-wrapped
  and macOS refuses to nest, so the breakage was undeclarable.
  **Shipped.**
- `project_config:` — a runtime reading session-dir config could not be
  declared, so parity's trust check silently skipped an unguarded repo→box
  channel. **Shipped.**
- `project_config_keys:` — the JSON-key narrowing half stayed Go-only for
  a further while (measured at HEAD 2026-08-29, ranger-base-qm6e).
  **Shipped, ranger-base-ncxa.** It is the one declarable key that
  *loosens* a safety check, so it refuses in two directions rather than
  one: without `project_config:` there is no file to narrow, and empty is
  the same declaration wearing a value. The check keeps its floor either
  way — a keyed file that is not a readable top-level JSON object fails
  closed, so keys declared over a TOML config degrade every launch instead
  of narrowing anything.
- `unattended:` — a yaml runtime with an unattended flag could not say so;
  `runtime check` printed "NO unattended flag known" with no remedy.
  **Shipped, ranger-base-ncxa.** What makes appending an operator's flag
  safe where guessing one never was: posse still guesses nothing, and the
  value is validated as something it may append to a shell line — it must
  begin with the flag (a bare word lands as a positional, which an
  interactive CLI reads as its prompt) and carry no shell punctuation.

Present-but-wrong refuses, absent stays loud — the existing `prompt:`/
`record:` semantics, verbatim.

**Method note (2026-08-30).** This list was two-and-a-half items stale
before anyone re-derived it, and the cheap check is why that was caught:
`runtimeYamlKeys()` is the whole load surface and an unknown key warns on
its own file, so "is X declarable" is one grep. Prose lists date; the
generated warning does not. The onboarding footer `runtime check` prints
had drifted the same way in the other direction — `turn_outcome:` shipped
and never reached it — and is now pinned against `runtimeYamlKeys()`
(`TestOnboardingFooterNamesEveryDeclarableKey`).

**Two declarability shapes (added 2026-08-28, ranger-base-ivf0).** The
grid now carries both, and a future seam picks deliberately rather than
by imitation of whichever key it sits beside:

- **Registry key** — for a value *code* consumes. `turn_outcome:`'s
  legal values are the keys of the turnfailure.go reader registry;
  present-but-unregistered **refuses at load**, naming what is on offer.
  The refusal is what keeps the p84 class out: a string that parses but
  names no implementation is a promise the pass would silently break,
  and the load check makes it unparseable instead.
- **Enum + free-text `_why:`** — for a measured fact whose consumer is
  the onboarder and the trust decision, not a code branch:
  `record:`/`record_why:`, §5's `rules_precedence(_why):`. The enum half
  validates; the why is provenance no load-time check can or should
  verify.

The test is one question: *who reads the value?* Code → registry key;
a human deciding trust → enum-plus-why. This refines, not reverses,
this section's rejection of yaml-declarable cost adapters: what stays
rejected is a string naming code that does not exist — a registry key
over readers that already exist is exactly how a yaml runtime borrows
one safely (a Bob whose CLI writes claude's transcript shape declares
`turn_outcome: claude-transcript` and is read today, changing no Go).

**Deliberately NOT yaml-declarable**, and why (a declared non-dimension is
as useful as a dimension):

- *Realizers* — a yaml runtime realizes nothing and every gate goes to the
  wall; safe by construction (ADR 0002). Adding a rule dialect to yaml is a
  rule→flag compiler nobody can maintain (rangerhq-b0y).
- *Cost adapters* — an adapter is code behind the ADR 0012 D4 seam; a yaml
  string naming one would be an inert lever born inert. `""` = uncounted is
  the honest default; the cap (once wired) is the brake.
- *Interstitial probes* — a probe is code. Prose without a probe is a
  comment, and the yaml's own comments already serve.
- *Herdr detection* — lives in herdr's manifests; `runtime check`'s launch
  row already measures recognition live (`KnownAgentKinds`), which beats
  declaring it.

### 5. The dimension no field expresses

**Native-rulebook precedence.** `NativeRules` declares what a runtime
*reads*; nothing declares who *wins* when it collides with the PID (ADR
0013 assumption 7; live collisions rangerhq-cmfj, rangerhq-o0el; probe
ranger-base-xaev). This is the most consequential equivalence question on
the books and it is currently a sentence in a grid footer. Decided: a
measured-fact declaration in the `record:`/`record_why:` pattern —
`rules_precedence: pid|native` + `rules_precedence_why:`, zero value
UNMEASURED and loud in the grid. Display-by-design: its consumer is the
onboarder and the trust decision (§0013 ties `record: trusted` to it), not
a code branch. Filled 2026-09-01: `pid` on codex and grok, from
ranger-base-6rcv's behavioural measurement (one billed turn each, a
contradicting fixture `AGENTS.md` lost on both) — NOT from
ranger-base-xaev's structural probe, which deliberately filled nothing
because placement is why-string material and the value is a claim about
which instruction the model follows. claude stays UNMEASURED: no claude
turn was authorized, and the field exists precisely so one runtime's
answer is not worn by another.

### 6. Sequencing — value first, no gold-plating

Priced by what has already bitten, per the operator's "do not gold-plate":
(1) declarability + grid completeness (Bob's grid, the switch-on-a-dime
surface); (2) the account stage's three inert promises (predicate,
per-pass loudness, cap brake — the only *money-shaped* silence);
(3) `startup_wait:` consumer (p84, already dinesh's); (4) danger-
interstitial refuse (latent until a typed-delivery runtime with a Danger
dialog exists — small, and the promise is already printed); (5)
`rules_precedence` (a field to hold xaev's answer). A dimension nobody
would ever switch on gets no field: none was added beyond these.

## Consequences

- Implementation beads (dinesh, `-l code`, dependency-ordered): yaml
  declarability set + field-classification pin test; `runtime check` rows
  for the non-dispatch dimensions (skills surface, egress, cage cred,
  project config, sandbox/gate-shell); account predicate through
  `Counted()` + per-pass uncounted loudness + runtime named in the launch
  line; `uncounted_cap_` ledger and brake (overflow-ledger pattern);
  danger-interstitial launch refuse on typed delivery; `rules_precedence:`.
  ranger-base-p84 stays the StartupWait wiring and is the worked example:
  its done-when tests the *consumer*, not the getter — every bead above
  inherits that clause.
- The parity children (ranger-base-nlya/25fj/unzn/fbxg/pj9f) keep their
  four areas as *measurement lanes* but score against this ADR's dimension
  list; a fifth lane covers the orphaned instruction-and-skill surfaces.
  Their matrices use the §2 verdict vocabulary verbatim.
- ADR 0013 §5 is unchanged in intent and now has a build plan; its grid
  remains the six-stage projection of this ADR's full dimension set.

## Alternatives rejected

- **Amend ADR 0013 with a §on parity** (monica's instinct, and the honest
  default — 0013 owns the grid, a second document can drift). Rejected
  because 0013's scope is dispatch ("it does not re-open the cage") and
  half the audited dimensions are 0002/0007/0009 territory; a parity
  section would make 0013 the register of everything and its next
  amendment a merge conflict with three other ADRs' subject matter. The
  drift risk is answered structurally instead: the struct + pinned
  classification test is the living checklist, and this document is dated.
- **A living checklist in NOTES.md.** Tribal knowledge with a filename.
  Nothing fails when it goes stale — the exact property that produced the
  four provisional areas.
- **A parity.yaml matrix beside the struct** (the clever one). A second
  store of the same facts, updated by hand, disagreeing with the struct
  within a week — ADR 0011's two-stores class, in checklist costume.
- **Make UNDECLARED refuse dispatch.** Purity, again (0013 already
  rejected it). It also inverts the operator's ruling: a grid where
  UNDECLARED is fatal turns the goal into a requirement.
- **Declare everything in yaml, including adapters and probes.** Levers
  born inert (a string naming code that does not exist) — manufacturing
  the p84 class at scale.

## Claims

**MEASURED** (grep/read at HEAD a6c81d3, this worktree, 2026-08-26)

- `rt.Wait()` has no production caller; `Dispatcher.StartupWait` is set
  once from `DefaultStartupWait` (dispatch.go:146) — p84 confirmed.
- No production code consults `Interstitial.Danger`/`Probe` outside
  runtimecheck.go; the "LAUNCH REFUSE" line is display.
- `Counted()`/`CostAdapter` consumed only by runtimecheck.go; cost.go:349
  and cockpit.go:218 hardcode `!= "claude"`.
- `uncounted_cap_` read only at runtimecheck.go:251; no ledger, no brake,
  no per-pass uncounted count; dispatch's create line prints persona, dir,
  tier — not runtime.
- LoadRuntime parses exactly the eleven keys listed in §4; no yaml path
  sets Unattended, SelfSandbox, SkillsCwd, ProjectConfig(-Keys),
  Interstitials, or CostAdapter.
- All other struct fields have the production consumers named in §3's table.

**ASSUMED**

- That the classified-fields test keeps future fields honest — it forces
  classification, not consumption; a field could still be classified
  "consumed" wrongly. The p84-style consumer-level done-when clause on
  each bead is the counterweight.
- That no live combination currently hits the danger-interstitial hole
  (claude's only interstitial is Seeded; codex/grok are argv-delivered,
  measured to sidestep). Latent, not urgent.
- Skills-surface parity rests on one skill, one day, three runtimes
  (rangerhq-74c6); grok `record: trusted` on one observation; codex
  `record: untrusted` on three. Values, not the mechanism — the
  measurement lanes own them.
