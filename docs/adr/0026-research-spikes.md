# ADR 0026 — Research spikes: when the nagging feeling means read first

*Status: accepted 2026-08-27 · owner: architect · realized by the SPIKE
rung, ADR 0005 §2 (bead rangerhq-qe37); this restatement is bead
ranger-base-5t6i · amended 2026-08-30 (§5: provenance is a comment, not
`--deps discovered-from:` — bd will not carry that edge and the block
together, ranger-base-rs8j)*

> Restated from the private archive of the instance this harness was
> developed in (its research-spikes ADR, bead rangerhq-dfz8); incident
> citations reference that instance's history. Persona names are restated
> as roles; measurements from that instance are restated as rationale,
> not quoted. In the archive this document carries a number that resolves
> to a different ADR here — see HISTORY.md "ADR numbering" for why
> cross-repo references go by title, never by number.

## Context

The SPIKE rung shipped in ADR 0005 §2's escalation ladder: every work
prompt tells every persona that when the gap is knowledge rather than
permission, the move is a `spike:` bead that dep-blocks the deciding
bead. The rung states its own triggers and queue mechanics, and 0005
records *where the rung sits and why* — but the practice the rung
realizes (what a spike owes back, what counts as a source, what one
should cost) had no public record. The ladder was instructing personas
to run a practice whose contract they could not read.

The practice earned its ADR in the archive the hard way. ADR 0011 was
derived entirely from this crew's own bruises, and the operator
recognised the diagnosis as fifty-year-old computer science — TOCTOU,
pidfile staleness, fencing tokens, database-as-queue, at-least-once. The
annotation pass (0011 Appendix A) confirmed it: the crew reached the
standard answers, mostly independently, after three implementation
passes at one invariant plus an incident evening. The operator's words
for the moment the reading should have happened: "the nagging feeling."
That pass was a one-off; this practice is the standing version, so the
next design does not need the operator to notice.

Two precedents predate the practice and are kept, not replaced: the
**empirical** spike (archive bead rangerhq-89a: probe the tool on the
actual host, measure, recommend — it corrected an assumption ADR 0002
had dressed as a measurement; `0002-container-tier.probe.sh` in this
directory is its surviving artifact), and the **literature** spike
(archive bead rangerhq-gsy3: primary sources verified live before the
distributed-systems skill was written — the skill's `references/` files
are its artifact, ADR 0007).

## Decision

**1. The trigger.** Any persona may pull the cord, on itself, mid-bead —
not only the operator, not only the architect. The nagging feeling is
any of:

- **Unnamed territory** — you are about to invent a mechanism or coin a
  vocabulary for one. If you are naming it, the field almost certainly
  already has; a name is a search key you are about to throw away.
- **Third attempt at one invariant** — two closed beads already tried to
  hold the same property and it broke again. Two reversals mean a model
  problem, not a code problem; a third code-first pass is the expensive
  move.
- **Expensive to reverse** — a store schema, a lock protocol, an
  interface other beads will build against, anything whose undo costs
  more than the doing.
- **A load-bearing number nobody measured** — the design's argument
  rests on a cost or behaviour that is assumed, not stated as measured
  (ADR 0002's assumed-then-measured filesystem tax is the cautionary
  example).

Check the shelf first: the skills a persona carries (ADR 0007) are the
cache of past spikes. A shelf hit means read the reference file and
continue — no spike. A shelf miss on a trigger means spike.

**2. The shape.** Two kinds, one contract. Both are time-boxed (default:
one working session), and the output is **knowledge plus beads, never
implementation**. A spike that lives only in a session transcript does
not exist: findings land in an ADR section/appendix or a notes file, and
the recommendations land as dependency-ordered beads.

- **EMPIRICAL**: probe the real tool on the real host, measure,
  recommend. Owes back: every number labeled *measured* or *assumed*; a
  re-runnable probe script committed next to the ADR it serves (the
  `*.probe.sh` files in this directory are the pattern); the machine
  left clean.
- **LITERATURE**: name the known problem, find what the field calls it,
  the standard answers *and their known failure modes*, then the calls —
  what we adopt, what we deliberately reject and why. "We reached the
  standard answer independently" is a fine result and worth recording.
  When a concept is general rather than shop-specific, the spike also
  owes a `references/<concept>.md` to the relevant skill — the shelf is
  how the next trigger becomes a cache hit.

**3. The honest limits.** Personas reach the web freely, so sourcing
rules carry the weight:

- Prefer primary sources and name them, labeled [paper] / [docs] /
  [blog]. Distinguish "the field's standard answer" from "one blog
  post's opinion" — both are usable, only one is load-bearing.
- A citation the operator cannot check is not evidence. Dead URL: cite
  the Wayback capture and mark it. Paywalled or bot-blocked: give the
  DOI as the record and say the fetch failed.
- The model's own recall is not a source. Every claim is verified live
  on the authoring date, and the ADR/notes section states that date.

**4. The cost rule.** A spike is real tokens. Measured on the developing
instance (restated as rationale, not quoted): each spike shape cost
single-digit dollars — about half the implementation bead it de-risked —
while the rediscovery it competes with was three dispatched
implementation passes at one invariant, manual re-claims by the
coordinator, and an incident evening, strictly more than either spike
before counting the operator's attention. Rule of thumb: **a spike costs
about half an implementation bead; one prevented reversal repays it.**
When a trigger fires, the burden of proof is on *skipping* the spike,
not on running it — and the time-box is what keeps the presumption
affordable.

**5. Where it lives in the queue.** A spike is an ordinary bead — no new
machinery:

- Title prefix `spike:` phrased as the question, label = the lane of the
  persona who runs it (literature → the architect's lane; empirical →
  the developer's or ops' lane).
- Priority: inherit the priority of the bead it blocks — a spike on a
  P1's critical path is P1 work.
- **The spike dep-blocks the bead it serves** (`bd dep add <deciding>
  <spike>`), so the dispatch queue itself enforces read-before-decide;
  nothing relies on anyone remembering. The queue, not `bd ready`: since
  ranger-base-lpz0o (2026-09-01) `Bd.Ready` is `bd ready` *minus*
  `bd blocked`, because a store bd makes today answers both with the same
  bead. Nothing here may take `bd ready` alone as the definition of
  unblocked.
- Provenance is a **comment** on the spike, `discovered-from:
  <deciding>`, *not* a `--deps discovered-from:` on the create. This
  clause read the other way until 2026-08-30 (ranger-base-rs8j) and the
  two halves cannot both exist: bd's cycle check spans every dependency
  type, so a spike holding that edge makes the `bd dep add` above a
  cycle, in either order, measured. What bd does about the cycle is a
  property of the STORE, not of its version (ranger-base-lpz0o, measured
  2026-09-01 on one 0.50.3 binary): a SQLite `beads.db` refuses the add,
  exit 1, and a `no-db: true` store — what `bd init` writes today —
  accepts it and leaves `<deciding>` in `bd ready` anyway. Either way the
  block does not take, so state the outcome and never the refusal.
  The block is the mechanism this ADR rests on, so the block wins and
  the provenance goes where nothing can refuse it. Confirm the block
  (`bd dep list <deciding>`), not the edge: reading the spike back looks
  right in the shape that never blocked anything.
- The trigger is realized as the **SPIKE rung** (ADR 0005 §2,
  `EscalationLadder` in `dispatch.go`), between ASSUME and ASK — the gap
  is knowledge, not permission, so it sits below the rungs that spend
  the operator's attention. The ladder is the one text every persona
  reads on every bead; PID prose is reinforcement, not the mechanism.

## Consequences

- The queue, not vigilance, enforces "read before decide" — a design
  bead behind a spike leaves `bd ready` until the reading is done.
- The rung itself already shipped with ADR 0005 §2; adopting this ADR
  changes no code. Cross-references that named the practice without a
  number (deliberately, while this record did not exist) can now cite
  ADR 0026.
- Over-pulling (spikes as procrastination) is bounded three ways: the
  time-box, the "owes beads back" contract, and the cost rule's
  half-a-bead shape keeping the spend legible in `posse cost`.
- `designs-implemented-unchanged` is the metric that should move; a
  spike whose design bead later collects `DIVERGED:` comments is
  evidence the practice is not working.

## Alternatives rejected

- **Fold the practice into ADR 0005 §2.** 0005 is the work-prompt and
  ladder *mechanism*; §2 already carries the rung's placement rationale
  and stops there on purpose. The practice — triggers as vocabulary,
  spike shapes, sourcing contract, cost rule — is persona behaviour, not
  prompt assembly, and filing practice into a mechanism document is how
  mechanism documents stop being readable (the archive rejected the
  mirror-image move for the same reason).
- **Ship the mechanism without the practice.** The rung's own text
  instructs every persona to run spikes; a public repo whose prompts
  command a practice with no readable contract makes the rung cargo
  cult. Restating one page of methodology costs less than that.
- **Mandatory literature pass before every ADR.** Most designs sit in
  territory the shelf already covers; an always-on pass burns the cost
  rule's margin and dulls the trigger. The trigger discipline *is* the
  decision.
- **Operator-only trigger.** The charter of this practice is precisely
  that the next spike should not need the operator to notice the nagging
  feeling on the crew's behalf.
- **Automated trigger detection** — the clever one: a watch-loop that
  flags "third bead touching one invariant" from bd history and
  auto-files the spike. Rejected because an *invariant* is not a bd
  field: a reversal lineage is legible to a reader of titles, not to a
  counter, and auto-filed spikes that misfire would teach the crew to
  ignore the real ones. Revisit only if bd grows a way to mark "this bug
  reverses <bead>".
- **PID prose only, no ladder rung.** Non-uniform: the persona most
  likely to hit a trigger mid-bead (the developer, deep in a code bead)
  is the least likely to have a practice line in its PID, and PIDs are
  the operator's files — enforcement should not wait on per-persona
  edits.
