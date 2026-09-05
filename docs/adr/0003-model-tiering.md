# ADR 0003 — Explicit model tiers; availability is advisory

*Status: accepted; simplified 2026-09-05 by operator ruling · automatic-substitution removal pending deferred implementation · owner: architect.*

## Context

Doing nothing keeps automatic fallback, cross-runtime substitutions and
carried explanation state. The review found repeated catalog errors, not
measured useful recovery: 336 of 497 catalog-log lines mentioned 401;
those are lines, not distinct failed launches. Keep tier and budget policy;
remove availability-driven model substitution. Runtime maps remain explicit.

## Decision

**1. A tier is intent, mapped per runtime.** `strong`, `standard`, `fast`
mean judged work, ordinary work and mechanical work. The current built-in
model ids and exact price rows live in runtime.go/cost.go; an instance may
overlay `model_<tier>` through 0013 §8. The overlay moves first and the
built-in follows in the next release (folded 0039 D1). Mapping multiple
tiers to one model is honest if that is the available shape. A missing
mapping displays `<runtime>/default`, never a model selection that did not
happen (0013 §6). `{model}` is one rendered argument, not a command string.
Reasoning effort remains outside the tier map; no unmeasured saving earns
an effort key or a second model-validity matrix.

**2. Preserve precedence and safety floors.** Resolve tier in this order:
CLI `--tier`, bead `tier:<x>`, `tier_by_label`, PID `tier:`,
`default_tier`, then strong. A `tier_floor` refuses lower choices. Fast
requires full gate parity and cannot waive degradation (0002). Preserve
explicit runtime/model selection and current per-session runtime/tier facts.

**3. Availability never chooses a replacement.** Remove `tier_fallback`,
its default/override walk, cycles/hop accounting, and carried automatic
fallback marks from launch, relaunch and display. Report the requested
mapping's known availability; the runtime may refuse an unavailable choice,
and an operator can select another explicitly. Do not let deleting a mark
falsely report an existing session's actual pair: retain current runtime/tier
identity and actual-model accounting; stop recreating old fallback metadata.
The implementation bead must price and verify the existing-session transition.

**4. Retain the independent dials.** Persona, label and bead defaults are
Dials A–C; `tier_floor` is D. Dial E's configured pass/day budget arithmetic
and budget-triggered step-down/stop remain, including judged-work and
explicit-choice exemptions. 0011 §5 denominates `budget_pass` in epochs.
No cost adapter/pricing means uncounted or unpriced, never zero (0013 §5).
Dial F keeps a fresh dispatched session per bead and resumes an existing
in-progress run; G leaves interactive spend visible and operator-selected.
Dial H, automatic availability fallback, is removed. Overflow is decided
separately in 0010; this page does not spend another pool to hide a miss.

**5. Keep the catalog lease and observer.** A successful reading is fresh
only while `now - at < model_probe_ttl`. Failed refresh does not renew it;
cooldown controls requests, not trust. Expired data is UNKNOWN with age and
failure reason. `model_preflight` remains the observer's off switch;
`posse runtimes` and its existing `--probe` expose advisory availability,
with forced reads still respecting cooldown. Never hand-edit catalog state.
Credential acquisition stays in 0019 D1. The catalog read presents the
session mint of the env sets the launch names — read from the set files
under the home, last assignment winning, never the process environment —
and the meter store is the fallback: nothing to read spends no request,
a 401/403 spends one more read and never a loop (0039 D3d: spike 200
ranger-base-au0o4, built ranger-base-mvrke, ruled ranger-base-q3n4e and
built through the preflight ranger-base-hr49g, all 2026-09-05; the ruling's
text, its rejected arms and V6–V8 stay on the superseded 0039 page). The
cockpit's persona-less reads keep the persona-less list; an empty list is
an answer, not a request for `default_env`. Overlay promotion belongs only
to 0015 §§2–3.

## Consequences and alternatives

ASSUMED implementation price: 6–10 source files plus tests/docs, one config
key and fallback-chain/carried-mark states removed; no new store, actor or
operator flag. Catalog cache remains replaceable derived data; explicit
maps remain in the promoted source. Useful unattended fallback recovery is
unmeasured. First done-when on the deferred removal: count actual automatic
substitutions that led to successful closes when the requested model could
not run, identifying distinct launches and the observation window. If the
removal is wrong, unattended continuity is lost until an operator reselects.

Rejected: doing nothing (carry an unpriced recovery mechanism), a second
fallback registry or bypass flag, permanent authority for expired catalog
data, and deleting budget floors with availability fallback (different
decisions). Documentation accepts the smaller contract now; code still
implements fallback until its deferred removal lands.

## Lineage

| Was | Here |
|---|---|
| 0003 §§1–4, Dials A–G | §§1–2 and §4, unchanged tier/budget intent |
| 0003 availability amendment and Dial H | §3, remove automatic substitution |
| 0039 D1 and D3a–c | §§1 and 5, built-in follow-through and advisory lease |
| 0039 D2 | 0015 §§2–3 promotion, referenced directly |
| 0039 D3d, built and ruled 2026-09-05 | §5 catalog credential; acquisition mechanics in 0019 D1; ruling text and V6–V8 on the 0039 page |

Dated model measurements and prior alternatives: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0003-model-tiering.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
are history, not current model recommendations.
