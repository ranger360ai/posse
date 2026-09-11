# ADR 0053 — Exact model selection is an explicit crew-session canary

*Status: accepted 2026-09-03 · amended 2026-09-06 (ranger-base-4pee8:
decision 3 restated to the property the canary needs — the tier's
availability VERDICT is skipped — in place of the mechanism it named, the
automatic substitution ADR 0003 §3 removes) · amended 2026-09-07
(ranger-base-uih37: the Verification bullet for "a runtime without a model
flag" marked unsatisfiable as written and its dead planLaunch refusal
removed) · amended 2026-09-11 (ranger-base-ujhut: D3 rests on the
runtime's `unknown_model:` declaration, not on a premise every CLI was
assumed to share; D4 ratifies the `?unchecked` listing mark; D6 rules that
the model a session is RUNNING is not read off its pane — ADR 0013 §7's
observation seam does not widen) · owner: architect · amends ADR 0003 for
interactive launches only*

## Context

OpenAI announced `gpt-6-astra` on 2026-09-03 and says access is rolling out
over the following days. The operator wants to try Astra, and later other
strong models, as soon as an account can serve them. This is a canary, not a
new fleet default.

Today `posse new` can override a persona's runtime and tier, but the tier
still resolves through the persistent built-in or `runtimes/<name>.yaml`
map. A PID or overlay edit therefore outlives the experiment. `--cmd` is not
an escape: a persona launch deliberately re-renders the runtime template so
the PID, skills and gates cannot be replaced by a raw command.

## Decision

1. `posse new` gains `--model <id>`. It is accepted only with all of:
   `--agent`, explicit `--runtime`, and explicit `--tier`. `posse new` marks
   the result as crew-owned already. The id must be one non-control,
   non-whitespace token, and the selected runtime must declare a model flag.
   Missing companions or an unrenderable id refuse before a workspace or
   session record exists.

2. The exact id replaces only the model id that `{model}` renders. Runtime,
   tier, cage, PID delivery, native rules, skills, gates, env sets and
   reasoning effort follow the ordinary persona launch. The runtime's model
   flag and the existing shell quoting carry the id; no raw command is
   introduced.

3. An exact model asks the provider, not the catalog. The point of the
   launch is to learn whether this account can run that exact id, and the
   provider's answer — a model response or a refusal — is the canary
   result. So the launch prints the exact-model line where an ordinary
   launch prints the tier availability verdict (the 0003 §5 catalog reading
   of the TIER's model): a verdict about the tier map's id would describe a
   launch nobody made. The id reaches the launch line as typed (D2), and
   nothing else about the launch moves — floor, parity and the first-run
   refusals rule on the runtime/tier pair the operator typed, exactly as
   they do for an ordinary launch, and a launch that names no exact model
   asks the preflight as it always did.

   *Amended 2026-09-06 (ranger-base-4pee8). The original read "An exact
   model bypasses tier availability substitution … posse must not turn it
   into a successful launch on the tier map's usual model." That named the
   automatic fallback ADR 0003 §3 removes (ranger-base-hv2zr, replayed
   under ranger-base-xpwlc): once it is gone, the sentence forbids a thing
   nothing can do. The exception was always the same one in both worlds —
   the canary never calls the preflight, so it never receives its verdict,
   and before the removal the substitution lived inside that same call and
   was skipped as a consequence, not as the rule. Dated snapshot: at
   2026-09-06 the removal is on the xpwlc seat branch and not on main;
   `git log --grep ranger-base-xpwlc` on main is the record of its landing,
   and neither state changes this paragraph. The four code sites that still
   quote the old wording — the `Model` field comment in herdrback.go, the
   `ExactModelLine` doc and rendered clause in exactmodel.go, the `--model`
   help in cmd/posse/main.go, and the test pin on the rendered clause — are
   reworded after that landing (ranger-base-lh6h5, dep-blocked on the
   replay), because the pin sits inside a hunk the replay carries verbatim.*

   *Amended 2026-09-11 (ranger-base-ujhut, from ranger-base-jzm04). "The
   provider's answer — a model response or a refusal — is the canary
   result" is true only of a CLI that CARRIES an id it does not know to the
   provider. MEASURED 2026-09-09 (ranger-base-jzm04): grok 1.0.5 does not —
   `-m grok-4.7`, an id that does not exist, launched with no refusal
   anywhere and the composer border read "Grok 4.6 (high)"; the CLI ran its
   own default and said nothing. Codex-cli 0.150.1 the same minute carried
   `-c model=gpt-6-astra` into its own banner. So the premise is a
   per-runtime fact, and D3 now rests on the runtime's DECLARATION of it:
   `unknown_model: carry | swap` with `unknown_model_why:`, a `Runtime`
   field, overlayable per box like `record:` and `rules_precedence:`,
   rendered as the `unknown_id` row of `posse runtime check`, unset is
   UNMEASURED and loud (ADR 0017 §5). The launch line's last clause is
   chosen on it — three sentences, one per value, and the unset one is not
   the swap one softened: "nobody measured this CLI" and "this CLI was
   measured to run something else" are different facts to an operator
   deciding whether the pane in front of them is the canary they asked for.
   Nothing refuses on the field and nothing picks a model from it: the id
   posse was told to ask for is still the id it asks for. Built-in values:
   grok `swap`, codex `carry`, claude unset — one claude datapoint would be
   one launch on an id this account cannot serve, and none was authorized.
   Dated snapshot, 2026-09-11: this lands with ranger-base-jzm04, whose
   seat tree holds it uncommitted; `git log --grep ranger-base-jzm04` on
   main is the record of the landing, this sentence is a snapshot.*

4. `model:` in `state/herdr/<session>.yaml` is the store of record for the
   override. The rendered argv and listing are derived from it, matching ADR
   0011's one-authority rule. Read/write, recreate, recovery command and the
   relaunch plan carry it. The listing suffix is
   `@<runtime>/<tier>=<model>`; an old or ordinary record with no `model:`
   renders exactly as it does today. Relaunch preserves the canary because it
   is the same session. Killing it ends the override.

   *Amended 2026-09-11 (ranger-base-ujhut). The suffix is
   `@<runtime>/<tier>=<model>`, and on a runtime that declares
   `unknown_model: swap` it is `@<runtime>/<tier>=<model>?unchecked`
   (`ModelUncheckedMark`, exactmodel.go; ranger-base-jzm04's divergence,
   ratified). `?unchecked` follows the listing's existing vocabulary for a
   kind of unknown — `mode:?covered`, `mode:?unnamed` — and qualifies the
   recorded id rather than replacing it: the row still renders the store of
   record, and the mark is the difference between "the model this session
   is running" and "the model posse was asked to run", which on a swapping
   CLI is the difference between an id and nothing. It is keyed on the
   MEASURED swap, never on the absence of a measurement (an unmeasured
   runtime is loud in `runtime check` already, and a mark that also meant
   "unmeasured" would stop meaning the one thing a canary row needs it to
   mean). The listing reads the declaration, not the session, so D4's
   one-authority rule holds: no second store.*

5. No PID key, runtime overlay key, config key, label rule, recipe field or
   dispatch flag is added. Nothing infers an exact model from `strong` or
   from a persona. The operator types the id for every new canary session.
   The first Astra invocation is:

       posse new richard-astra --agent richard --runtime codex \
         --tier strong --model gpt-6-astra

6. *Ruled 2026-09-11 (ranger-base-ujhut).* **The model a session is
   RUNNING is not read off its pane.** ranger-base-jzm04's fix shape asked
   posse to read the runtime's own model line — grok's composer border,
   codex's `model:` banner — and print "requested grok-4.7, running
   grok-4.6". That is a second observation under the one name-keyed
   exception ADR 0013 §7 grants, and the register that carries the
   exception says in as many words that a second observation is a
   decision, not a precedent (absencerules_qa_test.go, the
   `paneReaderFor` row). The decision is no, and it has to be written here
   because no pin holds the line: the reading would ride the SAME switch in
   the same file, so the abstraction audit would stay green while the
   exception quietly became two.

   What the reading would buy is a glance. The `?unchecked` row says "go
   look"; the widened row would say "Grok 4.6". One attach saved per canary
   on a swapping runtime — and a canary is an operator-typed crew session
   the operator is sitting in (the Context, and the dispatch alternative
   rejected below), so the pane
   is the surface they learn the model from either way. The only incident
   (jzm04) predates the mark, and the operator read the border themselves.
   That is a scenario, not a measured failure of the mark, and under the
   2026-09-03 ruling the bigger shape does not ship on a scenario.

   What it would cost, beyond the exception: the border's word is a display
   name (`Grok 4.6`) and the record's is an id (`grok-4.6`). Saying
   "running grok-4.6" needs a name↔id table — the second registry ADR 0057
   refused; saying "matches" needs the comparison; rendering the raw string
   beside the id is the only honest render, and it hands the operator the
   same comparison the mark already asks of them. Today the reading is
   grok-only besides — codex is `carry` and spends no pane read (NEVER),
   claude has no capture corpus — one CLI, one release, whose border shows
   its default for every unknown id by construction.

   What reopens it: a second runtime declared `swap`, or a marked row acted
   on as the running model. The build is then small and is recorded so the
   reopen is cheap: the model token is on the line `grokPaneMode` already
   parses (the text before the last `· `, or before the padding when there
   is no suffix; five captures in `etc/herdr/agent-detection/testdata/grok/`
   carry "Grok 4.6 (high)" on that border), carried on `PaneMode` as a
   verbatim string, rendered by a `PaneMode` method and nowhere else, with
   `TestPaneModeReadingDecidesNothing`'s owner set and symbol census
   covering it. That build is a reader change, not a new site.

## Consequences

- Any persona can be opened on an explicitly named strong model while the
  fleet and persona defaults remain unchanged.
- A bad or not-yet-entitled id may leave a crew workspace containing a CLI
  error. It is visible, operator-owned, and removable with the ordinary
  `posse kill` path; no bead was claimed.
- On a runtime declared `unknown_model: swap` the same id leaves a CLEAN
  workspace on the CLI's own default (2026-09-11). The launch line says so
  at the time and the listing row carries `?unchecked` for the session's
  life; the model that answered is on the pane, and posse does not read it
  (D6). The canary's question on such a runtime is answered by the CLI's
  own model list, run by the operator — MEASURED 2026-09-11, `grok models`
  on grok 1.0.5 lists `grok-4.6 (default)` and `grok-4.5` for this account
  and nothing else.
- `tier:` remains the operator's statement of workload intent. `model:` is
  the concrete model used for this run. Posse does not attempt to judge
  whether an arbitrary id deserves the word `strong`.
- The override does not change `model_reasoning_effort`; Astra supports the
  current `xhigh` value, so the canary preserves it.

## Alternatives rejected

- **Change Richard's PID or the Codex strong map.** Small edit, wrong
  lifetime: every later launch inherits the experiment and an overlay also
  changes other Codex/strong launches.
- **Add an `astra` tier or PID `model:` key.** A release name is not the
  durable judged/building/mechanical intent ADR 0003 assigns to tiers, and a
  PID key recreates the persistent default the operator rejected.
- **Launch with `--cmd`.** It drops the persona render path that binds the
  PID, gates and skills. Reconstructing that line by hand creates a second,
  drifting runtime template.
- **Add the override to dispatch now.** A pass-wide model experiment is a
  different risk boundary. The requested scope is an operator-visible crew
  session, where provider refusal is immediately visible and no work is
  claimed.
- **Check the exact id against the catalog before launching** (priced
  2026-09-06 under the D3 amendment). The catalog is a leased reading of
  one provider's model list, claude-only today, and a canary on an id that
  is rolling out is exactly the case where yesterday's reading says no. A
  pre-check would refuse or warn on the one launch whose purpose is to get
  a fresher answer than the catalog holds. The launch is the probe. *On a
  runtime declared `swap` the launch is NOT the probe (2026-09-11) — and
  the answer is still not a pre-check, see the next three.*
- **Leave D3 as written** (priced 2026-09-11). MEASURED false on grok, and
  the sentence is what the launch line printed, so the operator acted on
  it. Not an option.
- **Read the running model off the pane** (priced 2026-09-11, D6). One
  glance bought for a second observation under a by-name exception, a
  name↔id table or a raw string, and a pin's owner set widened — on a
  scenario. Recorded with its build recipe so the reopen is cheap.
- **Run the CLI's own model list at launch on a swapping runtime** (priced
  2026-09-11). `grok models` is the fresh, provider-owned answer the
  catalog objection above does not touch — and it is the right thing for
  the OPERATOR to run. As a posse preflight it is a third launch mechanism
  (a `models_cmd:` key, a subprocess, a parse of one CLI's own text —
  either a name-keyed reader, a third register row, or a substring test,
  the language ADR 0057 refused) for an event that is operator-typed and
  rare, and whether a rolling-out id appears in that list before it serves
  is UNMEASURED, so half the catalog objection returns.
- **Mark on absence as well as on swap** (priced 2026-09-11). `?unchecked`
  on claude too, "nobody measured". Rejected in D4: it would make one mark
  carry two facts, and the second one already has a louder surface.
- **Delete D3 once the substitution is gone** (priced 2026-09-06). The
  exception is live code and a live test — the branch in planLaunch that
  prints the exact-model line instead of calling the preflight, and the
  two-arm pin that proves the same fixture reaches a verdict without
  `--model`. A cite with no sentence behind it is a hole, not a tidy-up;
  the sentence stays and names the verdict.

## Verification and evidence

- Pin parsing and preflight refusal for every missing companion and
  whitespace or control bytes.
- "A runtime without a model flag" is unsatisfiable as originally written
  (ranger-base-uih37, escaped from ranger-base-1oyio): a file runtime
  defaults its `ModelFlag` to `"--model %s"` and only a non-empty
  `model_flag:` key overwrites it, and all three built-ins declare one, so
  no loadable runtime can reach planLaunch with an empty `ModelFlag` —
  there is nothing for a fixture to construct and no refusal to pin. The
  `Die` this bullet asked for was removed as dead code rather than kept
  unpinned; `ExactModelText`'s own `rt.ModelFlag == ""` guard (runtime.go)
  is the actual backstop if a runtime ever gains a way to declare no model
  flag, and that future change is what would make this bullet reachable
  again.
- Pin the rendered Codex line, `model:` record, exact listing tag, relaunch
  plan and recovery command. Pin an ordinary launch byte-for-byte against
  today's output.
- Run the Astra command above once after the built binary is installed.
  What the answer proves depends on the runtime's `unknown_model:`
  (amended 2026-09-11): on `carry`, a model response proves AVAILABLE and
  an explicit entitlement/model error proves NOT YET AVAILABLE; on `swap`,
  a clean launch proves NOTHING — the CLI's own model list (`grok models`)
  is the reading, run by hand — and only the CLI's own screen names what
  answered; on unset, the launch line says UNMEASURED and `posse runtime
  check <runtime>` says how to declare it after one launch on an id the
  account cannot serve. A local network denial proves neither on any of
  the three.
- `unknown_model:` values are dated measurements of one release, and the
  `_why` carries the date and release for that reason. MEASURED 2026-09-09
  (ranger-base-jzm04): grok 1.0.5 `swap`, codex-cli 0.150.1 `carry`. On
  2026-09-11 this box carried codex-cli 0.153.4, unre-measured; claude
  2.1.268, unmeasured — one launch on an id the account cannot serve
  settles each, and an overlay `runtimes/<name>.yaml` declares the answer
  without waiting for a posse (ADR 0021 D1).

MEASURED 2026-09-03: Codex CLI 0.150.1 accepts an exact `--model` / `-m`
selection and this instance's built-in runtime renders model ids through its
model flag; persona `--cmd` is overwritten by the safe render path; the
operator's configured reasoning effort is `xhigh`. The current caged shell
could not reach the provider, so account availability is UNMEASURED.

VERIFIED SOURCE 2026-09-03: OpenAI's
[GPT-6 Astra guide](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra)
names `gpt-6-astra`, supports `xhigh`, and describes the rollout window.

ASSUMED until the implementation pins it: one optional flat-record field can
round-trip through every session-meta rewrite without changing legacy bytes.
