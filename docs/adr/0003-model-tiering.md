# ADR 0003 — Model tiering: which model, how much, per persona and bead

*Status: accepted 2026-08-18 · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> Measurements from that instance are restated as rationale, not quoted;
> every operating value below is a dial the operator sets.

## Context

ADR 0002 gave every persona a *runtime* (precedence CLI > recipe > PID >
config) and a model-agnostic cage. It said nothing about *which model*
inside a runtime, or *how much* to spend. Running several concurrent
sessions at the strongest tier makes that the first question a fleet has
to answer.

What the development instance measured, restated as rationale: the
majority of a dispatched bead's cost is cache-reads of the context it has
accumulated, re-read every turn; the mid tier costs roughly half the top
tier on every price axis; a fresh session per bead removes the
accumulation and cuts a large further share; and interactive sessions
outspent the dispatched fleet by a wide multiple. The fleet is not where
the money goes; the fleet is where policy can *apply*.

Mechanism before this ADR: built-in runtime templates carry no model
flag; each CLI uses its own default. Nothing reads a budget.

## Decision (a framework plus dials; each ⚙ is the operator's call — record the values as bead comments)

**1. Tier is a name, not a model id.** Three tiers, mapped per runtime
in the built-in table (a `runtimes/<name>.yaml` may set `model_<tier>:`):

| tier | claude | other runtimes | meant for |
|---|---|---|---|
| `strong` | fable-5 | runtime default | design, audit, spec, anything judged |
| `standard` | opus-5 | runtime default | building, testing, ops chores |
| `fast` | sonnet-5 | (unset → standard) | mechanical: scaffolds, doc moves, bd hygiene, groom |

Rendering: built-in templates gain **`{model}`** → `--model <id>` /
`-c model=<id>` / `-m <id>`, empty when the tier has no mapping. PIDs
that carry their own `command:` don't get a model unless they add
`{model}` — never render an unknown token: an unrendered placeholder is a
literal argument to the CLI (`posse agent check` warns).

**2. Where the tier comes from — precedence, most specific wins:**

```
CLI --tier  >  bead label tier:<x>  >  config tier_by_label  >  PID tier:  >  config default_tier  >  strong
```

⚙ **Dial A — per-persona defaults** (PID `tier:`). Guidance: personas
whose output is judged rather than verified — architect, security,
product — default `strong`; building, verifying, and ops lanes
`standard`.

⚙ **Dial B — per-label map** (config `tier_by_label:`, one-level map,
wins over the PID because the bead's shape says more than the persona's
title). Example values: `doc, groom, triage, scaffold, hygiene → fast`;
`architecture, security, adr → strong`; everything else inherits.

⚙ **Dial C — per-bead override** — a bead label
`tier:strong|standard|fast` (visible in `bd list`, no new field, set at
grooming); `--tier` at dispatch for one-offs.

**3. Guardrail-reliability floor.** Cheaper models follow *prose* less
reliably; the wall (ADR 0002) doesn't care what model is behind it. So: a
session may run at `fast` **only if the parity check says every PID gate
is realized by the wall** on that (runtime × cage) — no
`--allow-degraded` at `fast`, ever. A PID may also pin `tier_floor:`
(e.g. `standard` for a persona whose critical guardrail is prose-only).
Below the floor → refuse, same message shape as an unrealized gate.

⚙ **Dial D — floor default**. Recommended: `standard` fleet-wide; `fast`
only via an explicit label/map signal *and* full parity.

**4. Budget and degradation — accounting first, caps later.** Config
`budget_pass:` / `budget_day:` in API-equiv dollars, computed by
`posse cost` from runtime transcripts (runtimes without a cost adapter are
**uncounted and said so** — never shown as $0). Dispatch checks before
each launch *when a cap is set*. Recommended: ship the accounting and the
cockpit display with **no caps set**; choose values after a week of your
own numbers. Until then Dial E is dormant.

⚙ **Dial E — what happens when the window is nearly spent**:
  (a) *wait*: stop dispatching new beads at 100%, nothing else changes;
  (b) *step down*: at 80%, `standard`-default sessions drop to `fast`
      (parity permitting), `strong` holds; at 100% stop;
  (c) *step down all*: as (b) but `strong` → `standard` too, floors and
      pinned labels excepted.
Recommended: **(b)** — quality of judged work is never traded silently;
mechanical work slows first, then everything waits.

⚙ **Dial F — fresh session per bead** for dispatch — recommended **on by
default** (context doesn't accumulate across beads, so a fresh session
cuts most of the cache-read cost); a session is reused only when the
bead is `in_progress` by that persona (`--resume` semantics stay). Cost:
startup latency per bead; gain: no context accumulation, cleaner
attribution.

⚙ **Dial G — interactive sessions** — **out of scope**. Tier only via
`--tier` on `posse new`; the cockpit shows tier + running cost so the
interactive spend stays visible, never gated.

## Consequences

- `runtime.go`: tier→model maps + `{model}` in built-in templates;
  `RenderCommand` takes a tier; `RHQ_TIER` in env; meta + cockpit show
  `runtime/tier`.
- Dispatch: tier resolution (label > map > PID > config), budget check
  before launch, degradation per Dial E, fresh-session per Dial F.
- The parity check (ADR 0002) grows one rule: `fast` requires full
  realization; `tier_floor:` refusal.
- `posse cost` owns the accounting; budget arithmetic is per-provider
  behind the cost seam (ADR 0012 D4), uncounted runtimes stated.
- The metric `cost-per-closed-bead` by tier joins the metrics catalog so
  the step-down dials can be judged, not guessed.

## Alternatives rejected

- **Model ids in PIDs.** Ties every persona to one runtime's naming and
  ages weekly; a tier name survives model releases and runtimes.
- **Per-intent map inside the PID** (`intents:` → tier). Intents describe,
  labels route (ADR 0001); the bead's labels are what dispatch has.
- **Auto-downgrade everything on budget pressure.** Trades the judged
  work's quality without anyone deciding; a `strong` design at `fast` is
  cheaper than a bad design only until it's built.
- **Token budgets per persona.** Personas don't spend, beads do; the pass
  and the day are the units the operator actually watches.
