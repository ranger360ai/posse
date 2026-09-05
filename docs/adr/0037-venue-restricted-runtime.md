# ADR 0037 — Venue-restricted runtime: dimensions are public, facts are private

*Status: accepted 2026-08-29 · owner: architect · builds on 0012 (instance
boundary), 0017 (equivalence grid), 0032 (engine onboarding, probe) ·
source bead ranger-base-pj9f (venue ruled on ranger-base-uwp)*

> The fourth runtime ("Bob" in ADR 0017's grid language) is
> **venue-restricted**: by policy — not capability — it may only run on
> one specific instance, and that instance is not this one. The operator
> ruled the venue 2026-08-29. This repo is public. Those two facts
> collide exactly where onboarding wants to write files, and a wrong
> commit here is the one mistake in the program that cannot be reverted
> after a push. This ADR fixes where every onboarding artifact lives and
> where the parity record is anchored, for Bob and for any future
> runtime with the same restriction.

## Context

ADR 0017 already made onboarding "filling the grid" and ADR 0032 already
made a template profile honest (assumed-until-probed; `posse runtime
probe` flips gates to realized via four observables). Neither says what
happens when the probe's venue is an instance this harness will never
see, or which side of a public/private line each artifact falls on. The
prior assumption — that a probe recording would have to "cross machines"
back to the authoring instance — turns out to be the wrong question, and
answering it wrongly would build a transport that violates ADR 0012.

## Decision

### 1. The split rule

**A dimension is harness material; a fact is instance material.**

- Harness material — yaml keys, grid rows, probe mechanics, turn-outcome
  readers, docs about the *shape* of onboarding — lands in this repo,
  public, and reaches the restricted instance by riding a cut release
  binary. Nothing runtime-vendor-specific is ever needed here: if
  onboarding a venue-restricted runtime seems to need code, that is a
  missing *dimension* (file it as one) — the 0017 shadow-predicate rule
  already forbids a name-keyed branch.
- Instance material — the vendor's identity, command strings, model
  names, dialog text, pane captures, test fixtures, the **filled**
  `runtimes/<name>.yaml`, probe records, and every measured fact about
  the runtime — lives only in the restricted instance's own private
  tree and RHQ_HOME. It is never committed to this repo in any form,
  including "scrubbed".
- The placeholder name alone ("bob") is already public precedent in
  this repo (ADR 0017, probe tests) and stays the only public trace.

Corollary for the authoring side: a private tracked tree that gets swept
into a live RHQ_HOME must not carry a `runtimes/<name>.yaml` either — a
registered runtime *is* the personal-instance presence the venue ruling
forbids. Templates travel inside a runbook, copy-pasted only on the
restricted instance.

### 2. The parity record is anchored, not transported

The parity record for a venue-restricted runtime — `probe.json`, the
`runtime check` grid verdicts, the four measurement lanes' matrices — is
owned by the instance where the runtime runs: its RHQ_HOME, its bead db.
Nothing syncs home. What crosses back, per the evidence discipline
already binding that instance (f85 / M2 contract): **generalized harness
bugs** (filed as if the runtime were anonymous) and **a verdict**
("lanes passed/failed") on the home queue's tracking bead. The home
instance has no consumer for the record itself — the runtime can never
run here — so a record transport would be a second store with no reader,
built across the exact boundary ADR 0012 exists to keep.

### 3. Sequencing — what each venue may do

HERE (public harness + private authoring tree), dispatchable now:
grid-completeness gaps the restricted runtime will need
(ranger-base-ncxa remainder: `unattended:`, `project_config_keys:`;
ranger-base-bcpa grid rows), and the private runbook: recon checklist +
skeleton yaml (ranger-base docs/runbooks/bob-runtime-onboarding.md).

THERE (restricted instance), strictly after it exists (M2,
ranger-base-26cd) and running a release that carries the gaps above:
(1) recon — one sitting fills the checklist; (2) fill the yaml from
recon facts; (3) `posse runtime check` until every row is a verdict or
a loud UNDECLARED; (4) `posse runtime probe`; (5) the four parity lanes
to the ranger-base-gz8h standard, ADR 0017 §2 vocabulary. The tracking
bead for this half **must be dep-gated on the instance existing** — an
undated "looks ready" bead here would dispatch a task that cannot be
done, and the failure would look like a crew problem instead of a venue
problem.

## Consequences

- ranger-base gains docs/runbooks/bob-runtime-onboarding.md (recon
  checklist, inline skeleton yaml, work-side sequence). No new posse
  code: Bob-shaped gaps are already ranger-base-ncxa/bcpa.
- The home queue keeps one tracking bead for the work-side half,
  blocked on ranger-base-26cd and ranger-base-bcpa; it inherits the
  runtime-parity track (gz8h) edge so the track stays honest.
- The release cut is a textual dependency (same pattern as me04 branch
  (c)): closing ncxa/bcpa is not enough — the restricted instance must
  install a release *containing* them.

## Alternatives rejected

- **Build the integration here** ("I could probably test it here") —
  capability was real, permission was not; the venue was ruled by
  policy. Not revisitable by any technical argument.
- **A scrubbed example bob.yaml in this repo's runtimes/** — the harness
  already documents every legal key in one place (`runtimeYamlKeys()`,
  and `runtime check`'s onboarding footer prints it); a half-filled
  public template adds nothing except a file whose natural next edit is
  the leak.
- **Transport the probe record home** (sync probe.json / grid verdicts
  into this queue) — a second store with no consumer, across the 0012
  boundary, carrying instance material into the public-adjacent side.
  The verdict comment is the whole requirement.
- **Wait, and design the split during M2 itself** — the visibility
  mistake is the one that cannot be fixed after the fact; deciding
  artifact placement before any artifact exists is the entire value of
  deciding now.

## Claims

**MEASURED** (this worktree, 2026-08-29)

- `posse runtime probe` and its record/parity machinery are landed
  (internal/rhq/runtimeprobe.go; ADR 0032) — the probe half needs no
  new harness code.
- "IBM" appears in this repo only as the Plex Mono font (www/index.html);
  "bob" only as a placeholder runtime name in tests and ADR 0017.
- `runtimeYamlKeys()` (internal/rhq/runtimeyaml.go:103) lacks
  `unattended:` and `project_config_keys:` — the ncxa remainder, per
  laurie's 2026-08-29 measurement on that bead.
- Live RHQ_HOME is `~/.config/posse`; `$CONSTITUTION/rhq` is a
  tracked copy that personas sweep into it — the basis for §1's
  corollary.

**ASSUMED**

- The restricted runtime presents a CLI shape (afl: both CLI and API
  available; the API-only path stays ADR 0032 §3's design, untriggered).
- One recon sitting suffices to fill the checklist — if not, the
  runbook's checklist is amended work-side and the gap filed as a
  generalized bug.
