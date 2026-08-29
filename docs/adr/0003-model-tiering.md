# ADR 0003 — Model tiering: which model, how much, per persona and bead

*Status: accepted 2026-08-18 · owner: architect · amended 2026-08-24
(ADR 0013 §6: unmapped tier displays as `default`, not the intent name)
· amended 2026-08-25 (§1/§3: the mapping can miss; rangerhq-oay shipped
the mechanism, ranger-base-lzx writes it down) · amended 2026-08-28
(§2: the mark survives a relaunch — ranger-base-twaq) · amended
2026-08-29 (§1: the grok column — rangerhq-jp6, ruled in
ranger-base-tg7c)*

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

| tier | claude | codex | grok | other runtimes | meant for |
|---|---|---|---|---|---|
| `strong` | fable-5 | gpt-5.6-sol | grok-4.6 | runtime default | design, audit, spec, anything judged |
| `standard` | opus-5 | gpt-5.6-sol | grok-4.6 | runtime default | building, testing, ops chores |
| `fast` | sonnet-5 | gpt-5.6-luna | grok-4.5 | (unset → standard) | mechanical: scaffolds, doc moves, bd hygiene, groom |

*(Amended 2026-08-26, ranger-base-arm.)* The codex column is filled; grok
is still runtime default. codex carried NO map at all until then, so
`tier:` was inert there — `{model}` empty, no warning, the CLI's own
choice. `strong` and `standard` naming the same id is the honest reading
and not an oversight: sol is what a codex session defaults to on this box
and codex offers nothing above it, and naming it makes the launch a fact
rather than a CLI default that can move between releases. `fast` = luna is
a **cost** lever only — measured 2026-08-25, moving a session to luna did
not lift an account-level usage wall, because that wall is on the account
and not on the model, so dispatch's budget step-down buys a cheaper model
there and no extra allotment.

*(Amended 2026-08-29, ranger-base-tg7c — the architect's ruling on the
grok column rangerhq-jp6 built and routed here for review. Self-contained
on purpose: it names the column's values so it stands whether it lands
before or after jp6's own amendment.)* Two calls.

**The shape stands: `strong` = `standard` = grok-4.6, `fast` = grok-4.5**
— the codex shape, ratified over the strong=4.6 / standard=4.5 map
rangerhq-jp6 originally asked for. `standard` is the everyday lane, and
this table's precedent (the arm amendment above) names the box default
there so the launch is a fact, not a demotion: standard=grok-4.5 would
have quietly moved ordinary grok work below the model it runs on today,
justified only by a pool saving nobody has priced — xAI publishes no
per-model rate against the weekly pool, and grok-4.5 has never run on
this box (181 of 181 priced turns in `~/.grok/sessions` are grok-4.6;
MEASURED, rangerhq-jp6). And the asked-for map left `fast` unmapped, so
its fallback rendered the same id as `standard` — a map where dispatch's
budget step-down (standard → fast) buys nothing at all. `fast` =
grok-4.5 is ratified as a **capability** step-down for mechanical work;
that it also saves pool is ASSUMED, plausible, and unpriced — nothing
may cite this row as a saving.

**`--reasoning-effort` stays OUT of the tier map.** It is arguably
grok's bigger spend dial (more positions than the model dial), and the
answer is still no, three times over:

- The load-bearing number does not exist: nothing on this box can price
  an effort step against the weekly pool — no usage endpoint, no
  per-model rate; grokpool.go has to *estimate* the meter at all. A
  spend dial with no gauge cannot be tiered honestly.
- The vocabulary is not uniform across the two models this map names:
  MEASURED 2026-08-29 (grok 1.0.5, `~/.grok/models_cache.json`),
  grok-4.6 offers xhigh/high/medium/low, grok-4.5 only
  high/medium/low — **no xhigh**. So a per-tier effort is not one new
  key, it is a validity matrix (tier→model × model→efforts) plus a
  second placeholder that, unrendered, is a literal argv to the CLI
  (the ADR 0001 lesson) — which is exactly why this was not a ten-line
  change and got routed here.
- Both models default to `high`, so leaving effort unset runs each at
  its shipped default — the same posture this table takes toward
  models. And the exit hatch needs no new vocabulary: a declared
  runtime's or PID's `command:` can append `--reasoning-effort <v>`
  literally today, fleet-wide or per-persona.

Revival condition, written down so this is a decision and not a taboo:
a measured pool cost per effort step (an xAI usage endpoint, or an
operator-funded A/B on the weekly pool). If that number arrives and is
material, the shape to build is `model_effort_<tier>:` + `{effort}`
with per-model validity enforced in `runtime check` — and not before.

Alternatives rejected: (a) standard=grok-4.5 — rests on an unpriced
saving and demotes the everyday lane, above; (b) a per-tier effort key
now — the clever one; three or four positions (per model) of a dial
nobody can read, priced above; (c) smuggling effort through the model
value (`"grok-4.6 --reasoning-effort low"` as the map's string) —
`{model}` renders one argv token via `ModelFlag`, and a map whose values
are secretly command lines makes every reader of the map wrong.

*(Shipped 2026-08-29, rangerhq-jp6 — the column the ruling above governs.)*
Ids read from the CLI on the day rather than from a doc: grok 1.0.5,
`grok models` ("Default model: grok-4.6"; grok-4.5 also served) and
`~/.grok/models_cache.json` fetched the same morning, both agreeing. Both
models carry a 500K context window and are served from
cli-chat-proxy.grok.com under the subscription session, not an API key.
Take the ids from the CLI again next time this map is touched — it
self-updates, and it moved 1.0.0 → 1.0.5 mid-verification once
(rangerhq-vjl). Display consequence: every built-in now maps every tier,
so no built-in reads `UNMAPPED` and ADR 0013 §6 bites on declared runtimes
only.


Rendering: built-in templates gain **`{model}`** → `--model <id>` /
`-c model=<id>` / `-m <id>`, empty when the tier has no mapping. PIDs
that carry their own `command:` don't get a model unless they add
`{model}` — never render an unknown token: an unrendered placeholder is a
literal argument to the CLI (`posse agent check` warns).

*(Amended 2026-08-24, ADR 0013 §6.)* The three names are intent (judged /
building / mechanical), not a promise that every runtime selects a
model. When `{model}` is empty the surfaces that *display* the tier
(`posse list`, cockpit, work-prompt header) show `<runtime>/default`,
never `<runtime>/strong`. A PID `tier: strong` on an unmapped runtime is
a `posse agent check` / `runtime check` warning. Overflow still never
moves `strong` (ADR 0010 §2b); an explicit `--runtime` is the operator's
decision and launches. The mapping can miss: a resolved tier names a
model id this account may not have. That is a launch substitution, not
a display question — Amendment 2026-08-25. ADR 0013 §6 still holds for
the unmapped case: no catalog posse can read → no preflight.

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

*(Amended 2026-08-25.)* The availability preflight never refuses on its
own. It runs after §2 resolution and before this check, and hands the
parity check and `tier_floor:` the *substituted* (runtime, tier) pair —
both rule on what would really launch, not on what was asked for. A
launch that would drop below the floor still refuses; the fallback line
prints either way.

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
- Session meta gains `fallback:`; listings wear `⤵️fallback` beside a
  `@runtime/tier` tag that already names the substitute. `posse cost`
  needed no change: `TierForModel` reads the model out of the
  transcript. Dispatch reads the launch record back (`effectiveTier`)
  rather than probing again.

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

## Amendment 2026-08-25 — the mapping can miss

*Shipped as rangerhq-oay; this section is the record, not a second
design. Operator decisions 2026-08-20 (on that bead's comments) are
quoted as rules, not re-opened.*

§1 said a tier is a name, mapped per runtime to a model id. It did not
say what happens when that id is not on the account. On 2026-08-20 the
operator's own session lost the strong model mid-day; a persona resolving
`tier: strong` would have launched anyway, the CLI would have served
whatever it falls back to, and nobody would have been told the shop had
stopped thinking at the tier its PID claims. `posse cost` would have
filed the spend under the substitute's tier with no line saying why.

**Decision.** After §2 has resolved a (runtime, tier) pair and before §3
runs, the launch asks whether the model that pair names is one this
account can run. Unavailable → **substitute**, loudly, recorded, and
never as a refusal of the preflight's own. Four rules, in the operator's
words:

1. **Cheapest honest probe.** A zero-token GET of the account's model
   catalog, same credential the plan guard already reads, shared through
   `$RHQ_HOME/state/model-catalog.json` behind `model_probe_ttl:`
   (default **1h**, `0` = every launch asks). Rate-limit cooldowns are
   shared across processes the same way `plan-usage.json` is. The
   catalog is the provider's store of record; the file is a derived
   snapshot with an age on it (Helland: data on the outside is from the
   past). Last-writer-wins is right for a snapshot. A runtime is probed
   only when posse knows a catalog for its ids — today, when its
   `egress:` names the Anthropic models host — so a template-only
   runtime on the same API is covered and a built-in that moves is not
   miscategorised by a stale name check. That is the precise form of
   ADR 0013 §6's "no adapter → no preflight."
2. **Unavailable is loud.** One line naming persona, asked-for tier,
   wanted model, and substitute (`richard: tier strong wants
   claude-fable-5 — unavailable, falling back to claude-opus-5`).
   Session meta gains `fallback:` (the line); `tier:` / `runtime:`
   already name what actually launched, the way `cage:` names the cage
   it got. Listings wear `⤵️fallback` *beside* the `@runtime/tier` tag,
   which now names the substitute. Dispatch reads that record back
   (`effectiveTier`) so the work prompt tells the persona the tier it
   is actually thinking at — one probe, one line, the meta as the
   launch's store of record (ADR 0011 §3). `posse cost` was already
   honest: `TierForModel` reads the transcript, not the PID.

   *(Amended 2026-08-28, ranger-base-twaq.)* The mark states a fact —
   *this session is not running the pair its PID names* — so it lasts
   as long as the fact does, a refresh included. `posse relaunch`
   recreates from the meta, which records the pair the last launch
   **fell to**; asked about the substitute the preflight finds it
   available and falls nowhere, so the mark used to be re-derived as
   empty and the session went on running the substitute with nothing
   anywhere saying so — `posse list`, the cockpit, the receipt and
   `effectiveTier` all silent, which is the lie this preflight exists
   to kill. The recreate therefore *carries* the line rather than
   re-deriving it, for exactly as long as the launching pair still
   differs from the PID's own; an operator who edits `tier:` down to
   what the session is really running has made the substitute the
   asked-for pair, and the mark is dropped. Nothing here moves the
   pair: a session degraded during an outage stays on the substitute
   until it is created afresh, which is §3's trade, not this rule's.
3. **The preflight never refuses.** "A degraded model is worse than
   nothing" is the operator's judgement, recorded in advance as
   `tier_floor:` (and as §3's no-`--allow-degraded` at `fast`). Both
   of those still bite, on the substituted pair. The fallback line
   prints either way.
4. **Reuse the keys that exist.** `tier_fallback:` for where a miss
   lands; the runtime's own `model_<tier>:` for what a tier means
   there. Two operational keys beside them, modelled on
   `plan_usage_ttl:`: `model_preflight:` (off switch; absent or
   anything but `false` means on) and `model_probe_ttl:`.

⚙ **Dial H — where a miss lands** (config `tier_fallback:`, one-level
map). Key = a persona name **or** a tier name (persona wins — a lane
can need a different substitute than its tier's; the standing example
is the security lane, whose fallback from the strong model may be a
different *runtime*). Value = a tier (drop a tier, same runtime), a
runtime (hop runtimes, same tier), or `none` (no substitute: run the
unavailable model and say so). Default is `strong` → `standard`,
which on claude is the strongest model falling to the mid one, and
it is a **floor rather than a seed**: a map that names other keys
does **not** take it away. Deliberately unlike `tier_by_label:`,
where a present key replaces the Dial B default wholesale — here the
operator's rule is that *everyone* falls back, so adding one persona
line must not silently switch the rest of the shop off. The walk is
bounded (four hops); a cycle or a typo is a clause in the loud line,
not a launch refusal.

**Fail-open, one direction only.** Only a catalog that was actually
read and does not contain the model moves a launch. Unreadable
credential, unreachable endpoint, 429, empty answer, runtime with no
per-tier mapping, preflight off — all UNKNOWN, and unknown launches
exactly what it was asked to, with no launch warning. The request
outcome is recorded in `state/model-catalog.log` so UNKNOWN is
diagnosable without changing that launch. A preflight that guessed
"unavailable" would silently downgrade the whole shop, which is the
failure it exists to prevent, one level up. The check-then-launch
window is a TOCTOU on a catalog another actor (the provider) mutates;
staleness is made harmless by failing open in the expensive-to-be-wrong
direction, not by narrowing the window.

**What the meta records, relaunch replays.** A session degraded during
an outage stays there until it is *recreated* after the model returns.
Re-deciding a live session's model under a running CLI is a claim
posse cannot make good on. Same trade as `cage:`.

**Out of scope — catalog membership is not allotment.** A model can
remain in the catalog while the account cannot spend on it. Measured
2026-08-24/25: a strong-tier session returned a synthetic "Fable 5
limit" message and settled idle without doing work; the preflight saw
nothing wrong. That is a **turn outcome**, not a catalog miss (ADR
0013 §6; detection already shipped). Demotion-on-exhaustion is a
different signal and a different bead.

**Consequences of this amendment**

- `internal/rhq/modelavail.go`: `App.TierPreflight` / `PreflightReport`;
  `planLaunch` calls it after §2 and before §3, reloads the runtime on
  a hop, records `launchPlan.Fallback`.
- `posse gates <persona>` prints the verdict per runtime — how an
  operator tells "the strong model is gone" from "the probe never
  answers on this box" without launching.
- Credential cadence is a second consumer of the same Keychain item
  at every launch, not only in an armed guard. Security review of
  that is a different lane (already filed); this ADR does not reopen
  the extraction path.

**Alternatives rejected (this amendment)**

- **Let the CLI fall back silently.** The incident. Spend moves
  tiers, listings lie, the PID's claim is theatre.
- **Refuse the launch.** The clever one: don't start work you can't
  think at. Operator, 2026-08-20: a degraded model is worse than
  nothing, and `tier_floor:` is where that call is recorded in
  advance. A launcher that refuses over availability second-guesses
  a dial that already exists.
- **Hardcode strong → the mid model.** Cheaper than a map. Rejected
  because one lane's fallback may be a *runtime hop*, not a cheaper
  model (operator, 2026-08-20, security lane). A per-persona key
  that can name a runtime is the whole reason the map exists.
- **`tier_by_label:`-style "present key replaces the default."** One
  persona line would switch everyone else off. The operator's rule
  is that everyone falls back; the default is a floor.
- **Treat unknown as unavailable.** Silently demotes the fleet the
  first time the credential or the endpoint blinks. The expensive
  guess, inverted.
- **Probe per prompt, or re-decide on relaunch.** A live session's
  model was chosen when it started. Re-probing under a running CLI
  cannot change it; re-writing `tier:` on relaunch would.
- **New vocabulary** (`model_fallback:`, a fourth tier, a PID key).
  The keys already existed; two operational knobs beside them.
- **Read allotment from the catalog.** Different fact. The catalog
  stayed populated through two measured exhaustion incidents. Folding
  them together would have the preflight "save" a case it cannot see.

**MEASURED vs ASSUMED (this amendment)**

- `/v1/models` exists, accepts this account's OAuth token, and
  returns the ids the claude tier table names — **MEASURED**
  2026-08-23 (unauthenticated 401 vs `/api/oauth/models` 404; live
  catalog of ten ids including all three mapped models).
- Both fallback shapes (default tier drop; per-persona runtime hop)
  — **MEASURED** on a doctored snapshot against the real binary.
- `TierForModel` already counts the model that ran — **MEASURED**
  (regression pinned at close of rangerhq-oay).
- Catalog membership ≠ allotment exhaustion — **MEASURED**
  2026-08-24/25.
- TTL 1h, 5s probe timeout, 4-hop bound — **ASSUMED** (access
  changes on subscription scale, not pass scale; a launch must not
  hang on a monitoring call; a long chain is a config the operator
  should see). The 1h was chosen to be generous to the endpoint and
  still catch a day-long outage on the next pass, not measured as
  optimal.
- Relaunch-replays-the-substitute — **JUDGED** (honest with `cage:`),
  not A/B'd against re-deciding.
