# ADR 0033 — The coordinator is not a lane: dispatch never hires the persona that governs it

*Status: accepted 2026-08-22 · bead ranger-base-kb7 · richard ·
re-landed 2026-08-28 under a free number, bead ranger-base-gbkr*

> Restated from the private archive of the instance this harness was
> developed in, where it was accepted 2026-08-22 as its ADR 0018. The
> seed that created this repo carried ADRs 0001–0012 and left it behind,
> and this repo's own ADR 0018 is the unrelated blind-meter-armed-ledger
> ADR — so it re-lands here renumbered, text otherwise verbatim, except
> that citations of the archive's 0013 (monica-pulse) and 0016
> (governance-surface) are respelled as their restatements here, ADR
> 0027 and ADR 0029 (HISTORY.md "ADR numbering": cross-era references
> never go by bare number). This design is already built here — §1–2 are
> dispatch.go's Route refusals keyed on config `coordinator:`, §5 is
> pidcheck's drift alarm, §4's G9 row is ADR 0029 §1's — and the code
> cited it as "ADR 0018" before it was restated; the commit that lands
> this file repoints every coordinator-meaning "ADR 0018" citation in
> code and docs to this document.

## Context

The g9md sighting (ranger-base-kb7): gilfoyle assigned a queue-hygiene
bead to monica; the autostart loop did the normal Dial-F thing and minted
a per-bead session carrying monica's full PID — the one PID holding
session direction (`Bash(posse:*)`) *and* `git push` — which worked the
bead and pushed to origin, unattended. The work was benign; the
capability is the finding: any writer of issues.jsonl can escalate a
bead into coordinator authority. The queue routed it, exactly as built —
`Route` returns any loadable assignee, and monica loads. Two more paths
share the hole: her PID's `labels: [orchestration, ops]` make her
label-routable, and `default_persona:` would accept her name.

The record already decides the role question. ADR 0029 §4: monica is
the exception handler, not the scheduler — a human-adjacent role that
judges, with work delivered by the pulse (ADR 0027), never by the queue.
ADR 0027's push condition rests on a singleton premise the archive
original stated verbatim — "monica is the only persona with push" —
(0027's restatement respells the names as roles) and a per-bead clone is a
second monica: two holders of session direction and push acting
concurrently is the multi-writer shape ADR 0011 exists to remove
(single writer; Thompson 2011, Helland CIDR 2007). And the pulse never
*launches* her (no live session → "undeliverable, nothing prompted"),
so dispatch was the only machinery that could mint her authority with
nobody watching. Close it and the invariant is structural: coordinator
authority exists only in a session a human opened.

The tier finding dissolves under the same light: the clone ran fast
because `BeadTier` resolves `tier_by_label` (Dial B, `hygiene → fast`)
above the PID's tier (Dial A) — ADR 0003 §2 as written, "the bead's
shape says more than the persona's title". Correct for lanes; for a
dispatchable coordinator it means whoever labels a bead picks the model
her authority runs at. Not a bug to fix — one more symptom of the
category error, and moot once she is not dispatchable. Loud tier
fallback stays rangerhq-oay's.

## Decision

**1. The instance names its coordinator; the engine carries no crew
name.** New config key `coordinator:` (ranger-base sets `monica`). One
fact, one store: config.yaml, the operator's file. Absent key = no
coordinator = today's behavior — the engine ships crew-agnostic
(hardcoding a persona name is the rangerhq-gk4k bug class). ADR 0027's
`pulse_persona` (unbuilt) now defaults to `coordinator:`'s value, so
"who is delivered to" and "who is never hired" cannot drift apart
(amended there). Exit hatch: the key holds no state; removing it
restores old behavior wholesale.

**2. `Route` never returns the coordinator, by any path.** Assignee is
the coordinator → unroutable, why: `assigned to the coordinator — not a
lane; <name> triages by hand (reassign, or take to the operator)`. The
label loop skips the coordinator's PID. `default_persona:` naming the
coordinator → unroutable, why names the config error. An explicitly
assigned bead never falls through to label routing — silently rerouting
an assignment hands the work to the wrong actor; unroutable-and-loud is
honest. Both launchers already share `Route` (the fire loop and the
cockpit's `LaunchBead`), so one refusal covers the pass, `--watch`, and
`d`. No flag overrides it — not `--persona`, not `--resume`, not
`--tier`. The operator reaches monica through her crew session (ADR
0008), which dispatch already refuses to touch.

**3. The refusal lives at hire time, not filing time.** bd is a store
any session writes; the queue is data on the outside (Helland CIDR
2005), and dispatch already treats it as hostile input (`beadIDRe`).
A filing-time ban would be a check at the wrong end of a
check-then-act window whose act — the launch — happens later and
elsewhere; enforce where the privilege is exercised. `assignee: monica`
stays legal in bd and keeps its honest uses: question beads she carries
to the operator (rangerhq-6ts), sightings she owns (ranger-base-kb7).

**4. The stuck bead is surfaced, not silently parked.** ADR 0029 §1
gains row G9 (amended there): a ready bead routed to the coordinator —
predicate: bd assignee/labels + config `coordinator:`, computable by
any process; class LANE. It rides into the §1–2 build (rangerhq-81y0).
Until that lands, the residual is the pass line every pass prints plus
monica's standing watch from the kb7 interim, which retires when §2
ships.

**5. A drift alarm, mechanical and advisory.** The parity check warns
when a PID's `allow:` grants `git push` — the coordinator's defining
permission — and that persona is not the named `coordinator:`, or when
such a PID exists and no coordinator is named. By ANY spelling that
grants it, not just the `Bash(git push:*)` the coordinator's own PID
carries: bare `Bash`, `Bash(*)`, `Bash(git * push)` and `Bash(git -C
<repo> push)` all hand a persona push, and all four were silent while
the check keyed on the L1 shim's rule parser (ranger-base-b2os, from
ranger-base-telz). The alarm over-approximates on purpose — an
unrecognized grant is silence, and silence is the failure mode. This
is the guardrail expressed twice (prose in the record, rule in the
checker),
never the enforcement: authorization is §2's refusal, keyed on config
alone. Inferring *authority* from permission strings would be a parse
in costume; inferring a *warning* is lint.

## Consequences

- Implementation, dependency-ordered, one session each: **§1–2**
  `coordinator:` key + Route refusal + tests over the fake queue
  (assigned / label-routed / default_persona paths, `LaunchBead`,
  `--resume`, the g9md repro launching nothing at any tier) → **§5**
  parity warning (independent, small). G9 is a comment on
  rangerhq-81y0, not a new bead. ranger-base config gains
  `coordinator: monica` now (inert until the code lands); the live
  `~/.config/rhq/config.yaml` copy is the operator's hand.
- Beads already `in_progress` under monica's claim become unroutable
  and are left alone — today's interim behavior made permanent; the
  cockpit still shows them, G9 will name them.
- monica's PID and ORDERS need no edit; every authority cited here is
  already granted or already hers. Her boot-time watch for
  monica-assigned beads retires when §2 lands.
- Observables for the verify bead, predicted now: a ready bead assigned
  `monica` → one refusal line, zero sessions created, zero claims; an
  `ops`-labeled unassigned bead routes to a lane persona, never monica;
  `default_persona: monica` → every fallthrough bead unroutable naming
  the config; parity output names any push-granting PID that is not the
  coordinator.
- Cost: none at runtime (one string compare per routed bead); the
  refusal line per pass is the price of honesty, delivered once G9
  ships.

## Alternatives rejected

- **Hire with a stripped profile** (no push, no session direction). A
  second session holding any slice of coordinator authority breaks the
  singleton premise ADR 0027's push condition and the one-pass rule
  stand on — coordinator authority is a single-writer resource, and
  contention machinery for it is overhead nobody meant to buy. Worse,
  the PID's prose still *commands* what the harness denies ("push crew
  commits promptly"), so every dispatched bead converges on a
  `REFUSED:` escalation loop — noise manufactured by design. And the
  misfiled work itself is lane work: a weaker monica doing it is the
  wrong persona succeeding, when the right outcome is routing it right.
- **Forbid assignment-to-monica at filing time.** Unenforceable at the
  boundary that matters (any session, or a hostile jsonl, writes bd),
  TOCTOU-shaped (check at write, act at launch), and it outlaws the
  legitimate uses §3 keeps. At most an advisory lint later; never the
  enforcement.
- **Route monica-assigned beads to another persona automatically.**
  The bead's own suggestion, half-adopted: the *surface* routes to
  attention (G9); the *work* is not silently reassigned — an explicit
  assignment rerouted without a human is dispatch guessing intent.
- **PID `dispatchable: false`.** The clever-simple one — I wanted it:
  general, self-describing, next to `tier_floor:`. But it puts a
  safety-critical default in the wrong place: every PID that omits the
  line ships hireable, so a fresh instance or a rewritten monica.md
  reopens the hole silently — and the architecture defines exactly one
  exception-handler *role*, not a per-persona toggle. If a second
  human-adjacent persona ever exists, that ADR promotes `coordinator:`
  to a list and inherits this one's reasoning.
- **Hardcode "monica" in Route.** rangerhq-gk4k names the class: a
  crew name compiled into a harness that carries no crew.
