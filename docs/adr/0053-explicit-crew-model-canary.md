# ADR 0053 — Exact model selection is an explicit crew-session canary

*Status: accepted 2026-09-03 · owner: architect · amends ADR 0003 for
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

3. An exact model bypasses tier availability substitution. The point of the
   launch is to ask the provider whether that exact id is available. A
   provider refusal is the canary result; posse must not turn it into a
   successful launch on the tier map's usual model. This does not alter the
   normal tier preflight.

4. `model:` in `state/herdr/<session>.yaml` is the store of record for the
   override. The rendered argv and listing are derived from it, matching ADR
   0011's one-authority rule. Read/write, recreate, recovery command and the
   relaunch plan carry it. The listing suffix is
   `@<runtime>/<tier>=<model>`; an old or ordinary record with no `model:`
   renders exactly as it does today. Relaunch preserves the canary because it
   is the same session. Killing it ends the override.

5. No PID key, runtime overlay key, config key, label rule, recipe field or
   dispatch flag is added. Nothing infers an exact model from `strong` or
   from a persona. The operator types the id for every new canary session.
   The first Astra invocation is:

       posse new richard-astra --agent richard --runtime codex \
         --tier strong --model gpt-6-astra

## Consequences

- Any persona can be opened on an explicitly named strong model while the
  fleet and persona defaults remain unchanged.
- A bad or not-yet-entitled id may leave a crew workspace containing a CLI
  error. It is visible, operator-owned, and removable with the ordinary
  `posse kill` path; no bead was claimed.
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

## Verification and evidence

- Pin parsing and preflight refusal for every missing companion, whitespace
  or control bytes, and a runtime without a model flag.
- Pin the rendered Codex line, `model:` record, exact listing tag, relaunch
  plan and recovery command. Pin an ordinary launch byte-for-byte against
  today's output.
- Run the Astra command above once after the built binary is installed. A
  model response proves AVAILABLE; an explicit entitlement/model error proves
  NOT YET AVAILABLE. A local network denial proves neither.

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
