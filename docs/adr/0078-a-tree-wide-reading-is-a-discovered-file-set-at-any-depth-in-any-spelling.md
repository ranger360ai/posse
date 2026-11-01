# ADR 0078 — a tree-wide reading is a discovered file set, at any depth below the root and in any spelling of the root

*Status: accepted 2026-11-01 (ranger-base-er0gn, from ranger-base-zm6yw) · owner: architect · reverses ranger-base-sx2dq's radius sentence in `twdRootExprs`; confirms ranger-base-m94g2 / ranger-base-na4rm and ranger-base-8yv7z · builds in the code bead named on ranger-base-er0gn (dinesh)*

## Context

`internal/treepins/treewidedoor_qa_test.go` derives the tree-wide class: the
QA pins that an unrelated bead can red, living in a package nobody runs whole,
so each needs a `make` door. Two rulings about the same reading disagree.

- **Out** (ranger-base-sx2dq, `twdRootExprs`' head comment): `detectionRig`
  (`internal/posse/verifydetection_qa_test.go`) climbs to the root and
  ReadDirs `<root>/etc/herdr/agent-detection`; "only a walk rooted AT the root
  is tree-wide".
- **In** (ranger-base-8yv7z, ranger-base-zm6yw): the known-screen reader
  (`internal/posse/d3knownscreen_qa_test.go`) enumerates
  `etc/herdr/agent-detection/testdata`, and the bob tripwire
  (`internal/posse/bobtripwire_qa_test.go`) enumerates
  `etc/herdr/agent-detection/upstream/bob`. Both are doored in
  `$(QA_SEED_PINS)`. ranger-base-m94g2 / -na4rm made a discovered file set the
  criterion outright, and the treepins half has always said a subdirectory
  exclusion "would be the bug here" (docs/notes.d is a subdirectory).

What separates the two today is **spelling**, not behaviour. The `qibRepoRoot`
arm (`root && walk`) does not look at depth, so a subdirectory walk spelled
with the helper is in; the same walk spelled through a hand-rolled bare-root
identifier (`repo := filepath.Join("..", "..")`) is scored by nothing
(MEASURED by ranger-base-zm6yw through `twdAnalyze`, main 5620d3b9). The
register exists to make membership independent of spelling.

MEASURED 2026-11-01, main 5620d3b9, darwin 25.6.0, go 1.26.10, load average
~20: `detectionRig` has **five** callers, not the "nine" the register's prose
repeats — `TestQAVerifyDetection{FailsACommittedRuleDeletion,
CheckInstallFailsAStaleInstall, ReplaysEveryFixtureItShips,
FailsAnOverrideDeletion, NamesTheManifestThatAnswered}`, all under
`posse_arm3`. Together they ran in 3.8s (9.5s wall with the build) and each
rig copied 2 manifests and 15 fixtures. A grep for a bare-root identifier in
`internal/` and `cmd/` test sources finds `detectionRig` and
`internal/posse/codexupdatemenu_qa_test.go`; the latter reads two NAMED files
and enumerates nothing, so it is out under either ruling.

## Decision

1. **The criterion is a discovered file set.** A pin is in the class when its
   reading enumerates a directory of the tree (`WalkDir`/`Walk`/`ReadDir`/
   `Glob`, or a registered indirect program). How far below the root that
   directory is, and how the root was reached — `qibRepoRoot`, `git rev-parse
   --show-toplevel`, or a hand-rolled climb in a literal, a `Join`, or an
   identifier — are not criteria. A pin that reads only named files is not in
   the class. This is the m94g2 / na4rm rule; sx2dq's sentence is reversed.
2. **The fence reads a subpath joined onto a bare-root identifier.**
   `twdClimbSet` binds `filepath.Join(<bare-root ident>, <segment>, ...)` the
   way it binds `twdJoinClimbsToASubpath` — zm6yw's `twdBareRootJoinReadings`
   promoted from detector to arm. The bare-root identifier on its own stays
   the root arms' (a walk of it is `hidden`), so nothing double-fires.
3. **`detectionRig` takes its root from `qibRepoRoot`**, and its five callers
   (with `detectionRig` and `runDetection`) move out of `posse_arm3` into a
   file with the arm-1 build line, then join `$(QA_SEED_PINS)`. A door takes
   no `-tags`, and moving arm is the precedent policydropins and
   shippedpidsdenyrefresh set for this door. seed-check's comment already says
   to type it when adding a detection fixture, which is what reds these.
4. **The live-defect pin goes red and is rewritten** as the fence's census,
   which must then be empty: `TestQATheClimbFenceStillHasItsBareRootJoinHole`
   becomes a pin that the hole is CLOSED, keeping its three fixtures as
   positive rows.
5. **Prose follows.** `twdRootExprs`' head comment, `twdClimbSet.bind`'s
   jurisdiction and residual paragraphs, the treepins-half paragraph that
   calls the posse exclusion "defensible", and every "nine tests/callers"
   sentence are rewritten to this rule. None of them states a caller count;
   if one must, it names the command that measured it.

## Consequences

- Five pins join the class and seed-check grows by about four seconds; arm 3
  shrinks by the same five tests. The register's count sentence moves, and
  `TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` enforces it.
- A detection bead that adds a manifest or fixture is now stopped by
  `make seed-check` (and so `make tree-check`) when the rig rejects it.
  Before this, those five were in arm 3, where an untagged `-run` selects
  nothing and prints `ok`.
- The rule is spelling-independent at last, so any later climb spelling is a
  fence GAP to close, never a jurisdiction question to escalate.
  - AMENDED 2026-11-01 (`ranger-base-pruuq`), the first time that bullet was
    exercised, and it held: the gap was closed without a ruling. It was not a
    climb spelling, though — it was a SECOND HELPER, and the fence that holds
    each package to one had two bodies, each reading only the route its own
    package's helper is written in (`runtime.Caller` plus a two-level climb in
    internal/posse, a climb from `os.Getwd()` to a `go.mod` in
    internal/treepins). internal/posse carried two helpers written in
    internal/treepins' spelling, and both of them ask the filesystem for the
    marker rather than computing a climb, so every climb-reading rule here was
    structurally blind and MEASURED false. Two tree-wide pins stood undoored
    for 52 days. The lesson for this bullet: spelling-independence has to hold
    for the ONE-HELPER fence too, or the derivation it guarantees is bounded by
    a spelling again one level up. `twdTwinRoutes` is one function both halves
    now call. See `docs/notes.d/ranger-base-pruuq.md`.
- Exit hatch: the rule is a predicate in one file. If the class proves
  over-inclusive, the remedy is a dispositioned exemption row naming the pin
  and why, not a depth limit.

## Alternatives rejected

- **(a) Keep sx2dq: depth decides.** Depth is not what lets an unrelated bead
  red a pin; a directory that other beads write into does.
  `etc/herdr/agent-detection` is written by every detection bead, and
  `docs/notes.d` (depth 2) is the escape this register was built for. (a)
  would also mean reversing two recent rulings (8yv7z, zm6yw's bob tripwire,
  same directory) to keep one, and adding a NEW depth test to the
  `qibRepoRoot` arm, which today ignores depth.
- **(c) "Who writes into the directory" as the criterion.** This is the
  honest criterion, but no AST reading can derive it, so it would become a
  hand-kept registry. A discovered file set is its mechanical proxy, and it
  errs toward including too much. An extra door entry costs seconds; a
  missing one costs a red `make test` for every seat.
- **(d) A depth threshold** (in at ≤1 below the root, out deeper). It invents
  a number nobody measured, and docs/notes.d already defeats it.
- **(e) Status quo, with the hole pinned.** That leaves a known blind spot
  over a directory with active writers, held open only by spelling. Removing
  spelling as a factor is the reason this file exists.
- **The clever one: merge `twdRootExprs` and `twdClimbSet` into one root
  dataflow** that tags every expression with "root" or "root + subpath", so
  the fence, the hidden arm, and the list arm become three reads of one map.
  It is the elegant end state and would delete the jurisdiction paragraphs
  outright. Rejected for now: it rewrites the core of a 4.5k-line register
  while ranger-base-zm6yw is editing it, to fix one live instance. Decision 2
  is one arm beside an existing one.
