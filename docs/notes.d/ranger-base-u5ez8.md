# A tree-wide pin over the refusals trail, and the two rules that do not work (ranger-base-u5ez8)

2026-11-22, Linux 7.2.15-1-omarchy x86_64, from this bead's worktree.

The host's own accesses to the refusals trail were pinned one call site at a
time: MEASURED, fifteen tests over four files —
internal/posse/refusalleaf_test.go (8), refusalparent_test.go (3),
cmd/posse/gatesrefusalread_qa_test.go (2) and runtimeprobe_test.go's delta
pair. Not one of them has the SET as its subject, so a new access arrived
guarded by nothing and read green. The class escaped by that exact route
three times, and the detection rule is the whole of the work. This is what
was measured on the way to it.

## What is now pinned

`TestQAEveryHostSideRefusalTrailAccessGoesThroughTheGuard`
(internal/treepins/refusaltrail_qa_test.go), doored by `make corpus-check`
and, since ADR 0098, by `make treepins-check` as a member of the package.
0.13s over 230 non-test .go files under `cmd/` and `internal/`.

It censuses every call in that corpus that is handed a refusals-trail path
and sorts it into three:

| answer | live count 2026-11-22 | what it is |
|---|---|---|
| guarded | 6 | the callee is `openRefusalLeaf`, `readRefusalLeaf`, `refusalLeafSize`, `refusalDeltaAfter`, `openRefusalDirAt` or `ReadRefusalsLog` |
| pass-through | 6 | the callee neither opens the path nor is a guard — printed, rendered into shim text, listed in a carve-out, or handed to a helper that opens it through a guard two frames down |
| unguarded | 0 | the finding |

The guarded six are egress.go:195 and refusalfold.go:78 (`openRefusalLeaf`),
refusalfold.go:171 and refusalleaf.go:194 (`readRefusalLeaf`), and
runtimeprobe.go:831/955 (`refusalLeafSize`, `refusalDeltaAfter`). They are
the positive witness: an assertion that nothing is unguarded is satisfied by
a scanner that found no accesses at all, so a count below four fails.

## Rule 1 that does not work: a literal grep

MEASURED, non-test `.go` under `cmd/` and `internal/`: **88 lines** name
`refusals.log` and **4** name `spool-cursors`, against six real accesses.
Nearly all of the 92 are prose comments or the rendered shim TEXT a seat
appends with — gates.go bakes `>> "$RHQ_GATES_DIR/refusals.log" 2>/dev/null`
into shell it writes at seven sites. Those lines are inside the wall and are
not the host's opens.

## Rule 2 that does not work: keying on the path helpers

`RefusalsLogPath`, `CageSpoolPath`, `spoolCursorPath` and `GatesDir` are the
helpers, and a rule keyed on them misses runtimeprobe.go, which built the
path with its own `filepath.Join(gatesDir, "refusals.log")` of a gatesDir it
already had — i.e. it misses the one that actually escaped
(ranger-base-jielm F3).

## The rule that does work, asked from the open side

An ACCESS is a call in the os / unix / syscall / ioutil open family, or a
call to a function of this tree's own one of whose parameters reaches one. A
FINDING is such a call whose argument is a trail expression and whose callee
is not a guard.

**The interprocedural closure is not optional**, and that is the one part of
the design worth carrying forward: it is what finds two of the three escapes.

- ranger-base-byqv0's cursor writer was `func writeSpoolCursor(path string,
  c spoolCursor)` doing `os.MkdirAll(filepath.Dir(path))` plus
  `os.WriteFile(path, …)`.
- ranger-base-jielm's probe pair was `fileSize(path)` / `readFrom(path, off)`
  doing `os.Stat` and `os.ReadFile`.

In both the open sits in a function that spells no trail literal at all; the
trail path is spelled at the CALL SITE one frame up. A rule that reads one
function body at a time is green over both. So the scanner computes, per
function and to a fixpoint over the corpus, which of its PARAMETER INDICES
reach an unguarded open, and a call hands a trail path to one of those
indices or it does not.

## The two measurements that moved the design

### A bare literal must not be a path (the six phantom producers)

The producer set — functions whose RETURN value is a trail path — is derived
from the body rather than spelled, so a renamed helper is still found. The
first cut let a string literal CONTAINING `refusals.log` be a trail path.
MEASURED over the live tree, that derived **nine** producers instead of three:

| derived | what it actually returns |
|---|---|
| `RefusalsLogPath`, `CageSpoolPath`, `spoolCursorPath` | the three trail paths |
| `usageCatalog` | `posse`'s own usage prose |
| `render`, `CommitGuardHook`, `constitutionGuardBody`, `dataCeilingCheck`, `dataCeilingReport` | rendered hook shell, which appends to the trail from inside the wall |

and **five findings** behind them: gates.go:7132 and :7144
(`installHook(a.CommitGuardHook(…))`), gates.go:7769
(`execOwnRendersAsked(commitRender)`), gates.go:8270
(`execOwnRenders(commitRender)`) — and cockpit.go:2808,
`write(c.render(w, h))`, which is a second `render` entirely, matched only
because callees resolve by name and one function of that name returns shim
text. That is the grep's own failure mode arriving through an AST pass, with
the name resolution compounding it.

The fix is one condition: a trail expression is a `filepath.Join` or a `+`
one of whose elements is a trail path COMPONENT (the literal, trimmed of
separators, equal to `refusals.log` or `spool-cursors`), a call to a
producer, a pure path wrapper over either, or a local bound to any of them.
A bare literal is where a path is SPELLED, never the path. With that
condition the producer set is exactly the three helpers and the live tree is
clean.

### Trail locals must be scope-sensitive

seatbelt.go's carve-out is `for _, f := range []string{"refusals.log",
"shell.log"} { c.Keep = append(c.Keep, filepath.Join(gd, f)) }`. It is a
list, not an access — but the census has to see it, or an open added inside
that loop tomorrow is invisible. Taking the range variable as a trail local
is what does that.

MEASURED with the taint keyed on the NAME alone and live over the whole
function: **six spurious pass-through rows** in the same function
(seatbelt.go:2100-2110), every one of them a different `for _, f := range …`
that has nothing to do with the trail. None was a finding, because `add` is
not an opener — but had any of those six callees opened what it was handed,
they would have been six findings, and a census whose rows a reader cannot
trust line by line is a census a reader stops reading.

So a trail binding carries the source range it is live over: the function
body for an assignment or a `var`, the `for` statement for a range variable.
Dataflow inside a function stays flow-insensitive on purpose — reasoning
about order would be a second, weaker type checker and the direction of its
mistakes would be the unsafe one — but it is scoped.

## The rig, shown able to fail

`TestQARefusalTrailCensusCatchesEachEscape` runs the same scanner over a
clean fixture (three producers, the guard, three guarded accesses and three
pass-throughs — the live tree's shape in miniature, asserted exactly so a
plant arm cannot pass over a rig that changed shape) and then over six
plants, each re-introduced as it was actually written:

| plant | spelling it exercises |
|---|---|
| ranger-base-legou: the spool create on a plain `os.OpenFile` | producer call one line up, open in the same function |
| ranger-base-jielm: `posse gates` reading the leaf by concatenation | `gatesDir + "/refusals.log"`, a `+` and not a Join |
| ranger-base-byqv0: the cursor write, path through a parameter | one interprocedural hop |
| ranger-base-byqv0: the unguarded parent create | `os.MkdirAll(filepath.Dir(cursorPath), 0o755)` — the pure path wrapper |
| ranger-base-jielm: the probe's delta pair, path through a parameter | an inline Join of a gatesDir the function already had |
| two hops to the open | the closure is a fixpoint, not one hop |

All six red, and the clean arm is green, so each arm measures the plant and
not the rig's opinion of ordinary source.

## What a green does not prove

Said in the file's head comment and repeated here because it is the part a
later reader will want:

- a trail path that never spells a trail component anywhere is invisible —
  assembled from fragments, read out of a config value, or handed in from
  outside `cmd/` and `internal/`;
- a trail path parked in a struct field, a map, a slice or a channel and
  opened elsewhere is invisible. cage.go's `Src: a.CageSpoolPath(…)` is that
  shape, and it is a mount the host never opens — but it would not appear
  below if it were one;
- callees resolve by NAME with the receiver ignored, so a call reaches every
  same-named function in the tree. That over-approximates in the safe
  direction: the opener set is bigger than the truth, so a green is stronger
  than the call graph;
- an open's path POSITION is not modelled — every argument of an open-family
  call is treated as a path, which is harmless here (a mode, a `[]byte` or an
  offset is never a trail expression) and costs no arity table that could go
  stale;
- TEST sources are not read, deliberately: refusalleaf_test.go and
  refusalparent_test.go plant these shapes by the dozen because planting them
  is their whole job, so a census over tests would be mostly fixtures.

The complementary guard for the first two is the per-call-site plant tests,
which judge behaviour rather than spelling. This pin's subject is the set;
theirs is the open.

## Where it lives, and why

internal/treepins, not internal/posse. That package's `TestMain` chdirs the
binary to the repo root, so a pin there reads the tree through a plain
relative path, and since ADR 0098 D1 every test it declares is a member of
`make treepins-check` by construction — no Makefile row, no register entry,
no hand exemption. It reuses that package's existing corpus rig (`arParse`,
`arFixture`, `arRoots`, `arLine`, `arStr`, `arFileFloor`) rather than walking
the tree again, which is also what makes it derivable as tree-wide by
treewidedoor_qa_test.go's own rule.

It is in `make corpus-check` as well because that door's subject is exactly
this pin's — a census over this repo's own .go sources — and 6.6s of typed
door beats 85s of complement for a seat that has just edited refusalleaf.go.
