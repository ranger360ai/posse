# ADR 0021 — Built-in runtimes take a per-key overlay from runtimes/<name>.yaml

*Status: accepted 2026-08-27 · owner: architect · amends 0002 §1 (the
exit-hatch sentence), 0013 §1 (declared-by column) · from
ranger-base-qkmz (split out of ranger-base-arm)*

> `App.LoadRuntime` returns a built-in as soon as the name matches,
> before it ever stats `RHQ_HOME/runtimes/<name>.yaml` (runtime.go).
> So every documented yaml key reaches template-only runtimes only.
> ranger-base-arm made the tool honest about that rule; this ADR decides
> whether the rule is right. It is not.

## Context

Two costs were paid in one week because a built-in's declared facts are
only editable in Go source:

- the codex tier map had to land in `runtime.go` (ranger-base-arm) — a
  `runtimes/codex.yaml` could not have supplied it;
- a per-runtime `startup_wait:` for grok could not be set at all while
  grok was still `prompt: typed`.

Worse than the release friction: the values are **instance facts baked
into shared source**. `codexModels` names `gpt-5.6-sol`/`gpt-5.6-luna`
because that is "what live sessions on **this box** show" (its own
comment). Another instance's codex offers different ids; its operator's
only paths today are editing our source or abandoning the built-in
entirely — a copy under a new name is template-only, which forfeits the
realizer, the fleet flags, the unattended guarantee, the interstitial
declarations and the record grade to change one model id.

ADR 0013 already speaks as if the yaml surface existed for built-ins:
§4 says record promotion is "a yaml/built-in edit after a measured
close", and `runtime.go`'s own `RecordTrusted` comment says "an edit
here (or a yaml key)". The declared-by machinery is also already built:
`declaredBy()` (runtimecheck.go) checks the yaml per key *before*
falling back to "built-in default" — it only needs `rt.Path` set.

MEASURED on this instance: `~/.config/rhq/runtimes/` does not exist.
No config anywhere depends on the current precedence.

## Decision

**A `runtimes/<name>.yaml` naming a built-in is a per-key OVERLAY: the
yaml wins for the keys it declares, the built-in supplies the rest.**
The line that decides which keys qualify: **a key is overlayable iff it
declares a measured instance fact; a key that changes the launch
mechanism is not.**

1. **Overlay keys** — same validation as template-only, present-but-
   wrong still refuses (`record: trused` never demotes silently):
   `model_<tier>:` (per tier — a yaml setting only `model_fast:` leaves
   strong/standard on the built-in map), `model_flag:`, `prompt:`,
   `startup_wait:`, `record:` + `record_why:`, `native_rules:`,
   `egress:`, `cage_cred:`, `gate_shell:`. List-valued keys REPLACE
   (the template-only semantics; a merge rule is a hidden rule).
2. **Refused keys** — `command:` and `skills_flag:` in a built-in's
   yaml refuse the load, naming this split. They are mechanism: a
   hand-written template wearing the built-in's realizer and
   `EnsureUnattended` is a launch nobody measured (the per-persona PID
   `command:` hatch already covers hand-written lines, visibly, ADR
   0002 §1); and the built-ins' skill surfaces are verified mechanisms
   (`skillsClaude`, `SkillsCwd` materialization — rangerhq-1qd measured
   codex/grok have *no* flag), so a flag key there declares something
   measured false and would run two half-bindings at once.
3. **`rt.Path` is set** when an overlay file exists, so `declaredBy()`
   names the source per key with no new code: a key from the file reads
   "runtimes/<name>.yaml (key:)", the rest read "built-in default".
   That column is the answer to "the built-in defaults become silently
   overridable": they become **loudly** overridable — the grid is the
   observable, and ADR 0013 §1's declared-by column becomes literally
   true instead of aspirational.
4. **`record: trusted` requires `record_why:`** — overlay and
   template-only alike. Promotion follows a measurement (ADR 0013 §4);
   a trusted with no measurement named is the silence the contract
   exists to remove. (No existing yaml anywhere; nothing breaks.)
5. **Surfaces flip back**: `runtime check`'s built-in footer and
   `tierFix()` (the arm-era "a runtimes/<name>.yaml is never read"
   honesty) now describe the overlay and name the overlayable keys.
   `ListRuntimes` reports an overlaid built-in once, not twice.

Not touched: the launch-site precedence (CLI > recipe > PID > config >
claude) is *which runtime*; this is *what a named runtime is*. The cage
engine loader (`cage.go`) has the same built-ins-first shape; extending
the overlay there is a separate decision with its own key split — noted,
not decided here.

## Consequences

- `LoadRuntime`: after a built-in name matches, stat the yaml; absent →
  today's behaviour exactly; present → overlay per the key split above.
- Degrade directions stay honest: `gate_shell: false` degrades L0
  toward the wall and parity reports the unrealized denies (ADR 0009
  §2); an `egress:` replacement that drops the catalog host turns the
  availability preflight off (`anthropicAPI` predicate, modelavail.go)
  — visible in the grid's account/tier rows, and the file is the
  operator's own config root, the same trust level as `config.yaml`
  and the PIDs beside it.
- A stale overlay pins a value across a posse upgrade that re-measures
  the built-in. That is what an override is *for*; the grid names the
  file per key, which is where staleness is found.
- ADR 0002 §1's exit hatch sentence gains: "…and, for a built-in name,
  the same file is a per-key overlay of its declared facts."
- Implementation cut as ranger-base beads (mechanics, then surfaces).

## Alternatives rejected

- **Keep built-ins closed** (status quo). Priced twice in one week;
  bakes per-box facts into shared source; the only operator hatch
  abandons every measured guarantee to change one number.
- **Full overlay including `command:`/`skills_flag:`** (the clever
  one — one rule, no key split). A frankenruntime: an untested template
  under a realizer that renders flags for a line it did not write, the
  ADR 0009 drift problem in yaml costume, fleet-wide and outside any
  PID. The PID `command:` hatch already exists per persona.
- **Shadow-replace** (a yaml naming a built-in makes it template-only
  wholesale). Silent loss of the realizer for every session of that
  name; "safe by construction" for a runtime posse never knew is a
  downgrade for one it did.
- **A `runtime_overrides:` block in config.yaml.** A second place to
  say the same thing; `runtimes/<name>.yaml` is the documented surface
  and `declaredBy` already reads it.
- **Env-var overrides** (`POSSE_CODEX_MODEL_STRONG=…`). Invisible in
  the grid, per-process rather than per-instance, undeclared drift.
- **Version-pinned overlays** (the yaml names the CLI version it was
  measured on; mismatch refuses). Posse would have to probe CLI
  versions at every load, and staleness is already visible in the grid;
  where a pin exists it is the operator's own (grok's version-pin).

## Claims

**MEASURED**
- The early return: runtime.go `LoadRuntime` walks `builtinRuntimes`
  and returns before statting the yaml (read in this bead; first
  established in ranger-base-arm).
- The two paid costs (ranger-base-arm; the grok `startup_wait:` gap).
- `declaredBy()` already source-splits per key once `Path` is set
  (runtimecheck.go).
- `ListRuntimes` would list an overlaid name twice today (runtime.go).
- `~/.config/rhq/runtimes/` does not exist on this instance
  (2026-08-27): the precedence flip breaks no existing config.
- The cage-engine loader shares the built-ins-first shape (cage.go).

**ASSUMED**
- That no *other* instance of this restated harness carries a
  `runtimes/<builtin>.yaml` written under the old rule (it was dead
  config there; after this ADR it becomes live — the refused-keys check
  is what keeps a stale `command:` in such a file from silently
  replacing a launch line).
- That the overlay keys' validation paths behave identically when
  reached from a built-in (same `YamlGet` calls; to be covered by the
  implementation bead's tests, not re-measured here).
