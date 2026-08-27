# ADR 0022 — A path-limited commit narrows a sweep by file, never by writer: shared files get one writer by construction

*Status: accepted 2026-08-27 · owner: architect · extends ADR 0002 §3's
commit guard · re-scopes the provenance promise in AGENTS.md and the
work-prompt blueprints (ADR 0005) · from ranger-base-yuwy*

> Two personas held edits in NOTES.md on the same afternoon. One declined
> to sweep and paid a deferred bead and two days; the other's *blessed*
> `git commit -- NOTES.md` swept both hunks into 808da1b under the wrong
> bead id. Nobody broke a rule. The rule was smaller than it read.

## Context

The shared checkout (`~/src/<repo>`) is one working tree and one
`.git/index` used by every crew and operator session; dispatched sessions
get private worktrees (worktree.go, rangerhq-09o2/nyqj). Inside one
checkout git has **no per-writer boundary**: `git commit -- <path>`
commits the working-tree content of the named path, whoever wrote it
(measured, rangerhq-2f5r). So the `--` rule closes exactly the failure it
was written for — a bare commit sweeping *other files* (rangerhq-nyqj) —
and cannot close the same-file one: two in-flight writers of one file,
first to commit takes both, silently, under one bead id.

The cost is the record, not the bytes. Every work prompt promises
`git log --grep <id>` finds the work, and ADR 0006 §3 hands verify beads
that commit list. After a sweep the verifier lands on a commit that does
not contain the change. Both measured incidents (ranger-base-yuwy,
2026-08-27, both directions in one afternoon) were NOTES.md — the one
file every session edits from the shared checkout.

The git-native answer for two writers in one file — stage your own hunks
with `git apply --cached`, commit the index — is refused by the guard,
and rightly: a commit that bypasses the shared index leaves it holding
the pre-fix blobs, and the next shared-index commit silently reverts the
work (measured, rangerhq-8rtf; `scripts/audit-silent-reverts.sh` exists
because of it). That recipe presumes a single-writer index; this one has
N writers.

This is the single-writer principle (Thompson, "Single Writer Principle",
2011 — [blog]): contention machinery exists only because multiple writers
mutate one resource; partition until each resource has one writer and the
machinery vanishes. The finest partition git offers inside one checkout
is the file, and the only per-file writer we can establish *without
attributing hunks* is **the bead that created the file**.

## Decision

**1. Reclassify the rule.** In the shared checkout, `git commit --
<paths>` is *sweep-narrowing*, not isolation. The provenance promise is
scoped honestly: it holds in a session worktree unconditionally; in the
shared checkout it holds only for files with one in-flight writer.
AGENTS.md "Landing the plane" and the work-prompt blueprint text say so
in one sentence each; the instance PIDs' Work-prompt paragraphs carry the
same sentence.

**2. One writer per file in the shared checkout; for NOTES.md, by
construction.** Personas in the shared checkout do not edit or commit
NOTES.md. Two routes, both keeping `git log --grep <id>` true:

- **Fragment.** Write `docs/notes.d/<bead-id>.md` — a file the bead
  creates, sole writer by construction — and commit it path-limited under
  the bead id. No waiting on other writers, exact provenance, and the
  content is live documentation where it sits. Folding fragments into
  NOTES.md is ordinary work: a docs bead, worked in a worktree.
- **Worktree.** Edit NOTES.md from a dispatched session's own tree, where
  same-file divergence surfaces at land time as a rebase conflict the
  launcher reports (worktree.go: ff-only, rebase in the session tree,
  abort-and-report on conflict) — visible, never a silent sweep, and each
  commit keeps its own bead id through the replay.

**3. The wall.** The prepare-commit-msg guard (ADR 0002 §3 slot, same
marker, keyed on `RHQ_PERSONA`, shared-checkout-only via the git-dir ==
git-common-dir discriminator it already computes) gains a third arm: a
persona commit whose index changes root `NOTES.md` is refused, naming
both routes. Feasible and measured (2026-08-27, git 2.39.3): during
`git commit -- <paths>` the hook runs with `GIT_INDEX_FILE` set to the
`next-index-<pid>.lock` temp index and `git diff --cached --name-only
HEAD` lists exactly the pathspec'd changed paths. The arm must run
*before* the existing next-index exemption `exit 0`s. Files join this
list by a **measured cross-sweep**, never by guess; today the list is
root NOTES.md.

**4. What does not move.** The `--` requirement stays (it still closes
the cross-file sweep). The private-index refusal stays (rangerhq-8rtf).
Worktree sessions are untouched by the new arm. 808da1b is not rewritten;
the corrected attribution lives on ranger-base-82u/i0s8's beads, where
the verify trail already looks.

## Consequences

- `gates.go`: third arm in `sharedIndexBody`, placed ahead of the
  temp-index exemption; refusal text names the fragment path and the
  worktree route; `refusals.log` line; tests alongside the existing
  gateschain pins (ranger-base-hokh).
- AGENTS.md "Landing the plane" + NOTES.md (a short section introducing
  `docs/notes.d/`) + this repo's example-PID work-prompt text: the
  re-scoped sentence (ranger-base-8zhr). Instance PIDs are the
  operator's copy of the same sentence, same bead, which the wall bead
  waits on so the refusal never points at an undocumented convention.
- `docs/notes.d/` exists from the first fragment; no scaffolding.
- The defensive-direction cost inverts: rangerhq-ybec's two-day deferral
  becomes "write the fragment now, fold later."

## Alternatives rejected

- **Attribute working-tree hunks to sessions and refuse foreign hunks**
  (the cheap partial proposed on the bead). No store records who made a
  working-tree edit; the only candidate journal is runtime tool-hooks,
  which are L0, runtime-specific, and blind to `sed -i` — ADR 0014 §5
  already ruled a hook is not a wall. And any check of a live tree is a
  check-then-act race. An unattributable fact cannot back a refusal.
- **Teach the gate to allow subset-index commits** (`git apply --cached`
  + private index — the clever one, and the one I wanted). Subset-ness is
  attribution again, and rangerhq-8rtf measured the deeper hazard: every
  commit that bypasses the shared index primes the next commit to
  silently revert it. Correct recipe in a single-writer-index world;
  ours is not one.
- **A lease/claim per file** (bd claim on NOTES.md before editing).
  Fencing, expiry, and liveness machinery to serialize a notebook —
  Thompson's exact point is that partitioning makes that machinery
  vanish. It would also enforce at the layer that already failed:
  discipline.
- **Serialize NOTES.md through one owning persona.** An assignee is not a
  mutex: dispatch runs per-bead sessions, so one persona can be two
  writers; the operator writes outside any persona.
- **Abolish shared-checkout persona sessions** (crew gets worktrees too).
  Structurally complete, and possibly the horizon — but crew sessions are
  long-lived, multi-bead, operator-facing (ADR 0008), merge-back is keyed
  to bead close, and the measured recurrence is one file. Restructuring
  crew to protect a notebook is the elegant fix collapsing under its own
  weight. Revisit on a measured cross-sweep in a non-NOTES file.
- **Generate NOTES.md from fragments permanently.** NOTES.md is topical,
  curated; a concatenation converts it into a log and discards the
  curation, which is where its value lives.

## Verification (QA's checklist)

1. Shared checkout, `RHQ_PERSONA` set, NOTES.md and another file dirty:
   `git commit -- NOTES.md` → refused, message names `docs/notes.d/` and
   the worktree route; `git commit -- <other>` → lands, NOTES.md
   untouched and still dirty.
2. Same commit with `RHQ_PERSONA` unset (operator) → lands.
3. Session worktree, `RHQ_PERSONA` set: `git commit -- NOTES.md` → lands
   (git-dir ≠ common-dir exits the guard first).
4. `git commit --amend -- NOTES.md` in the shared checkout as a persona →
   refused (amend takes a pathspec; measured class from the existing
   guard).
5. A fragment commit `git commit -- docs/notes.d/<id>.md` as a persona in
   the shared checkout → lands; `git log --grep <id>` finds it.

## Measured versus assumed

| claim | status |
|---|---|
| `git commit -- <path>` takes working-tree content, ignores the index | **MEASURED** (rangerhq-2f5r; worktree.go header) |
| Both sweep directions live in NOTES.md, one afternoon | **MEASURED** 2026-08-27 (ranger-base-yuwy; 808da1b) |
| Private-index commits leave the shared index stale → silent reverts | **MEASURED** (rangerhq-8rtf; audit-silent-reverts.sh) |
| prepare-commit-msg sees the pathspec'd changed paths via the temp index (`git diff --cached --name-only HEAD`) | **MEASURED** 2026-08-27, git 2.39.3, scratch repo, this bead |
| The existing exemption `exit 0`s before any per-path check would run | **MEASURED** (gates.go `sharedIndexBody`, read this session) |
| Worktree landing surfaces same-file divergence as a reported conflict, never a sweep | **MEASURED** for the mechanism (worktree.go ff-only/rebase/abort, rangerhq-jbyr); the specific NOTES.md-conflict path is **ASSUMED** from rebase semantics — QA item for the docs bead if doubted |
