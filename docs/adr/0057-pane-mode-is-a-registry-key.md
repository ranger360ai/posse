# ADR 0057 — What a pane says about its permission mode is a registry key, not a table

*Status: accepted 2026-09-05 · owner: architect · relates 0017 (§1 missing-row
test, §3 shadow-predicate rule, §4 declarability shapes), 0035 §3 (the
DECLARED DIFFERENCE this makes literal), 0021 (built-in overlay), 0012 D4
(adapter seam) · source bead ranger-base-re4kb, from ranger-base-vwgt*

## Context

ranger-base-vwgt built the pane-mode reading: `ReadPaneMode` in
`internal/posse/permissionmode.go`, `HerdrSession.PermissionMode`, rendered
by `posse list` and `posse gates <persona>`. It is three-valued per runtime
because the three built-ins were MEASURED to offer three different
contracts (2026-08-29): claude names all six modes in its footer, grok names
two of six on its composer border, codex renders none on any screen. The
readers are keyed on the runtime NAME — a `paneModeReaders` map plus an
`if runtime == "codex"` branch — and vwgt registered both in
`arShadowAllowed` as CLI-own-state with the row saying ADJACENT, NOT RULED.

The question is ADR 0017 §1's own. A fourth runtime lists every session as
`mode:?` with the why "no pane reader for runtime X — nobody has measured
what its pane says", forever, and the only way out is editing Go. That is
a loud UNKNOWN, which §2 permits — but `posse runtime check` has no row
for the dimension at all, so an onboarder filling the grid meets it in
`posse list` after launch instead. By §1's test that is the missing-row
bug: three measured values exist in code and no checklist screen says so.

Nothing branches on the reading today. The stakes are the grid, the
declaration, and whether the codex branch stays a name-keyed dimension.

## Decision

**1. `Runtime` gains `PaneModeAdapter string`, declarable as `pane_mode:`,
in the REGISTRY-KEY shape of ADR 0017 §4** — the `turn_outcome:` shape,
verbatim. Legal values are the keys of a reader registry in
`permissionmode.go` keyed by READER name, not runtime name:

| key | reads the pane | absence on screen means | provenance |
|---|---|---|---|
| `claude-footer` | yes, last three non-empty lines | COVERED — a dialog replaced the footer; clears on the next read | corpus in permissionmodepane_qa_test.go |
| `grok-border` | yes, composer border | UNNAMEABLE — four modes and the splash render nothing | same corpus |
| `none` | no herdr call at all | NEVER — measured to render no mode on any screen; the column is permanently `—` | the declaring file |

Absent is the loud default: `PaneModeUnread` with today's why, unchanged.
Present-but-unregistered REFUSES at load naming what is on offer, exactly as
`turn_outcome:` does (runtime.go, the `turnOutcomeReaders[v] == nil` arm).
The three built-ins set the field to the constants; the name-keyed map and
the codex branch are retired, and their two `arShadowAllowed` rows go with
them. `fillPaneModes` resolves the adapter through `LoadRuntime` once per
distinct runtime name per listing; a name this box cannot load is one more
why-carrying `PaneModeUnread`, never a blank.

**2. `pane_mode:` is a MECHANISM key under ADR 0021** — it joins
`builtinMechanismKeys` beside `turn_outcome`, and refuses in a built-in's
overlay. Which reader parses a CLI's screen is code measured against that
CLI's captures, not a fact about this box. A claude release that rewords its
footer is fixed at the corpus, where the measurement lives, not by an
overlay nobody measured.

**3. `runtime check` grows a `pane_mode` row**, in the ADR 0002/0007 set,
rendered from the registry entry rather than spelled: the adapter and what
absence means for it; `none` spelled as a DECLARED DIFFERENCE, not a
failure (ADR 0035 §3's sentence becomes a row); unset spelled UNDECLARED
with the remedy — declare one of the registered readers if measured, and a
new screen vocabulary is a reader plus its corpus, which is the same price
`turn_outcome:` charges. The `missing:` column says what the blindness
costs: the ADR 0035 §3 compensating control cannot see a flag-lost session
here. Pinned on the RENDERED row (`gridRow`), and the onboarding footer
names the key (`TestOnboardingFooterNamesEveryDeclarableKey` reds until it
does). `PaneModeAdapter` classifies as **consumed** (herdrback.go,
permissionmode.go) in `TestEveryRuntimeFieldIsClassified`.

**4. The vocabulary of ADR 0035 §4 is unchanged.** A named mode is a
default disposition, never a non-blocking promise; the row and the column
name the mode and nothing else.

## Consequences

- A yaml runtime whose CLI paints a claude-shaped footer declares
  `pane_mode: claude-footer` and is read today, changing no Go. One measured
  to render nothing declares `none` and gets codex's `—` instead of `?`.
  One with a new vocabulary still needs a reader — and the grid now says so
  on the checklist, before the first launch.
- The `if runtime == "codex"` branch stops being a dimension implemented
  by name (ADR 0017 §3): codex's measured absence becomes a declaration the
  grid can render, and the shadow register shrinks by two rows.
- codex's why loses its `/status` sentence on the listing line — the `none`
  reader's why is generic; the measured detail stays on the built-in's
  comment and in the grid row's note. Accepted: the listing token is `—`
  either way, and one why per reader beats one per runtime name.
- Cost: one yaml read per distinct runtime name per mode-rendering listing.
  ASSUMED cheap and unmeasured; the cockpit's per-tick refresh never turns
  `PaneModes` on, so no hot path pays it.
- Build order: both beads dep-block on ranger-base-vwgt LANDING — its
  reader is uncommitted in a session tree at this HEAD (measured: no vwgt
  commit on any branch, `git status` in that tree lists permissionmode.go
  untracked), and a bead built on a sentence about a sibling tree races by
  construction.

## Alternatives rejected

- **Do nothing; the readers stay CLI-own-state.** Priced first. §2 is
  satisfied today (`?` and `—` are distinguishable), so the loss is only
  the grid row and Bob's exit. But the codex branch is a dimension keyed on
  a name — the one shape §3 forbids — and the row alone (option below)
  would have to read the name-keyed map to render, which is scenery over a
  declaration that does not exist.
- **Row only, no field.** The cost seam's shape (registry keyed by runtime
  name, no struct field, `CostRead()` resolves through it). Honest for cost
  because registering an adapter IS the act; wrong here because the bead's
  question is whether Bob can declare anything, and under that shape the
  answer is permanently no.
- **A declared table of footer spellings** (the clever one, and the bead's
  own worry: `pane_mode_<mode>: "<literal>"` as an `interstitial_<slug>:`
  family over a generic substring reader). Rejected three ways. It covers
  claude's reader shape and not grok's — the border parse, the
  always-approve layer ambiguity, and the UNNAMEABLE absence are per-reader
  facts a table cannot carry. Absence semantics need a second declared
  bit (is the table complete over this CLI's mode set?) that posse cannot
  validate, and ADR 0017 §4 already ruled that prose without a probe is a
  comment. And it manufactures the p84 class at scale: a misspelled
  literal is a reader that never matches and reports `?covered` forever,
  indistinguishable from a real dialog — a lever born inert wearing a
  reading. A registry key names a reader with a committed corpus and a
  mutation-checked pin; that is what "measured" means for this dimension.
- **Enum plus `pane_mode_why:`.** The §4 test is *who reads the value*:
  code does — `fillPaneModes` skips the herdr call on `none` and the
  renderer branches on the state. Registry key. Provenance for `none` is
  the declaring file, which `declaredBy` already prints.

## Claims

**MEASURED** (this worktree, HEAD 3106bc7, 2026-09-05): no row in
runtimecheck.go mentions a permission mode; `runtimeYamlKeys()` carries no
mode key; the flat yaml reader supports one-level maps and prefix families
(yamlflat.go) so the table shape was buildable, not merely undesirable;
`turn_outcome:` refuses an unregistered name at load and refuses in a
built-in overlay; the vwgt reader exists only uncommitted in its session
tree; ADR 0057 is unclaimed on every local branch.

**ASSUMED**: that a fourth runtime with a claude-shaped footer is the
realistic borrow case — no fourth runtime is named today, so the field's
first yaml declarant is hypothetical, and the row is what pays for itself
regardless; that the per-listing `LoadRuntime` is cheap.
