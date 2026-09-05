# ADR 0057 — Concrete pane-mode observations without a declaration registry

*Status: accepted 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · pane_mode registry removal approved; implementation deferred.*

## Decision

Keep current permission-mode observations through their concrete readers in
`permissionmode.go`, carried on `HerdrSession.PermissionMode` and rendered by
`posse list` and `posse gates`. Remove the independently declared `pane_mode`
YAML key, runtime field, reader-name registry and registry-derived checklist
row. This is an explicit narrow exception to the general adapter abstraction
rule in [ADR 0013](0013-runtime-dispatch-contract.md): these display observations may select
their measured CLI reader directly. No other dispatch dimension gains a
runtime-name exception.

| Concrete observation | Preserved meaning |
|---|---|
| Claude footer, live last three nonempty lines | Named mode; absent footer is COVERED, e.g. a modal dialog replaced it |
| Grok composer border | Only recognized auto/always-approve suffixes name modes; absent/unrecognized suffix is UNNAMEABLE, never an inferred default |
| Codex's measured screen limitation | NEVER: permanent `—`; do not spend a pane read to relearn this limitation |
| Unknown runtime or failed/unattempted read | UNREAD with a reason, `?`; cannot claim a measured absence |

Retain the distinct observation states and explanations, including the
always-approve layer ambiguity. Parse live screen tail, never echoed launch
argv or quoted scrollback as proof of current mode. A mode is a default
disposition, never a promise that a session cannot block
([ADR 0035](0035-mode-second-layer-typed-line-only.md)). No launch eligibility or guard may branch
on these display readings merely because they exist.

Remove the `none` **adapter entry**, not the NEVER observation state. Unknown
CLIs remain loudly unmeasured. Onboarding a different screen requires a
concrete reader and capture corpus until an actual external declaration shows
that another seam earns its cost. Do not replace YAML with a generic footer
substring language, a second registry or a silent unknown-to-none fallback.

## Deferred deletion and acceptance

Delete `Runtime.PaneModeAdapter`, `pane_mode` load/validation/overlay-key
handling, the `PaneModeReader` registry and registry resolution/list helpers,
and the declarable-key/checklist row. Change `fillPaneModes` to use the
concrete readers, preserving its actual read/no-read outcomes; remove runtime
YAML loads needed only to select this observer. Retain any runtime loads
needed for other dimensions. Update the abstraction audit's narrow exception
and field/overlay/onboarding tests with the same scope.

Files: `internal/posse/runtime.go`, `runtimeyaml.go`, `runtimecheck.go`,
`permissionmode.go`, `herdrback.go`, with tests. Price: roughly 4–6 source
files; one YAML key, one runtime field and registry/none-adapter state; zero
stores, background actors or operator flags. The current implementation still
has these until the deferred task lands. Stale external keys must be named
through normal unknown-key diagnostics, not silently certify a reader.

First done-when row: **count of working pane-mode adapters supplied through
instance declarations that cannot use the built-in readers.** State census
scope and distinguish a real working external declaration from the review's
hypothetical fourth CLI. Zero leaves hypothetical extension value; a real
dependent declaration needs its compatibility consequence reported before
accepting removal. The measurement is on the code task, not a docs blocker.

Other acceptance preserves each row above, distinct visible unknowns, no
Codex pane call, no screen-mode-based launch decision, and the narrowness of
the abstraction exception. What breaks if wrong: a working external runtime
loses declarative reader selection; or overbroad cleanup collapses an unknown
into a false mode, defeating the existing detection control.

## Lineage and evidence

| Record | Surviving decision |
|---|---|
| ranger-base-vwgt / 0035 | Concrete observations, state distinctions and corpus |
| 0057 earlier 2026-09-05 proposal | Registry shipped; its declaration seam now approved for removal |
| Operator ruling 2026-09-05 | Preserves observed behavior through direct readers |

The earlier page's “uncommitted/unbuilt” claims describe its earlier HEAD,
not today's code: the registry is present at this execution baseline. Its
claimed fourth-runtime borrow case remains ASSUMED; the 2026-08-29 captures
remain dated evidence, not fresh vendor verification. Removing the reader
itself would lose a useful observation and is not authorized by this decision.

[Prior registry design and dated claims](history/0057-pane-mode-is-a-registry-key-before-2026-09-05.md).
