# ADR 0035 — Permission modes use the launch line; runtime homes retain their owner

*Status: accepted; simplified 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · proposed duplicate Codex flag cancelled; current behavior retained.*

## Decision

Posse does not create permission-mode profiles in a runtime configuration
home owned by the runtime/operator. A flag plus a separate foreign file adds
two artifacts and competing writers; a missing profile can silently fall
back to the runtime default. Existing launcher-owned settings and scoped
credential surfaces remain governed by [ADR 0002](0002-runtimes-and-gates.md) and
[ADR 0019](0019-credential-architecture.md); this is not a ban on those owned artifacts.

Keep the existing typed launch-mode flags. Cancel the unbuilt instruction
to append Codex `-c approval_policy=never` beside `-a never`. The built-in
launch line currently has `-a never`; the second spelling was approved on
ranger-base-ay3dr and filed as ranger-base-6tj5r, but has not shipped. This
operator ruling supplies the new decision required to cancel it. No runtime
flag is removed in this session and the existing Claude inline settings
layer is not part of that cancellation.

The older experiment established that the extra spelling parsed on the
tested CLI, not an independently measured reduction in failed launches.
One-spelling drift remains possible and must be caught by launch assembly
checks and actual observations. A different launch subcommand needing its
own syntax still follows the runtime contract; this decision is not a claim
that every CLI subcommand accepts `-a`.

Keep concrete pane-mode observations and their limitations under
[ADR 0057](0057-pane-mode-is-a-registry-key.md). Grok's measured border observation can reveal
a flag-lost mode; it does not prevent that mode. Codex's tested screen did
not reveal a permission mode. Runtime checklist differences remain explicit
under [ADR 0013](0013-runtime-dispatch-contract.md), without a new YAML observer registry.

A permission mode is a default disposition, **never a non-blocking promise**.
Mode and blocked/queue state remain separate facts. The recorded Claude auto
mode classifier still requested confirmation in the 2026-08-29 probe.

## Price, risk and rejected alternatives

Zero present source files, keys, states or actors removed; one future flag
avoided. The existing deferred build bead ranger-base-6tj5r now has an
obsolete premise for Monica to dispose of when lifting the pause. Do not
file a duplicate implementation task to remove code that never existed.

What is forgone: a second spelling might survive an edit that drops the
first. Its incremental protection is ASSUMED; both ride the same command
line. Existing independent enforcement boundaries remain.

Reject foreign-home profiles and relocating an entire runtime home just to
own mode configuration: the latter also moves authentication and can require
a fresh login. Reject global config edits whose audience includes the
operator's unrelated sessions. The dated scratch-home and bogus-variant
experiments remain evidence about their named CLI versions, not promises
about future ones.

## Lineage

| Record | Surviving decision |
|---|---|
| 0035, 2026-08-29 | No foreign-home mode writer; mode is not a liveness guarantee |
| Operator ruling 2026-09-05 | Reverses ay3dr/6tj5r's approved but unbuilt redundant Codex spelling |

Prior proposal and measurements: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0035-mode-second-layer-typed-line-only.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
