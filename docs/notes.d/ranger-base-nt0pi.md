## The fourth spelling of a tree reading: no root at all (ranger-base-nt0pi)

2026-10-31

The tree-wide door class (`internal/treepins/treewidedoor_qa_test.go`) is
"a QA test whose subject is the TREE, living inside a package nobody runs
whole". Its `internal/posse` half is derived, every rule of it, from a body
that reaches the repo ROOT: `qibRepoRoot` directly, a helper that walks from
it, a `runtime.Caller` twin, a hand-rolled `../..` climb, or
`git rev-parse --show-toplevel`. That key exists because `go test` runs the
package's binary with the PACKAGE directory as its cwd, so reading the tree
costs a climb.

It costs a climb. It does not cost a root.

```go
filepath.Glob("../../docs/notes.d/notes-*.md")
filepath.Glob(filepath.Join("..", "..", "examples", "skills", "*"))
```

Three tree-wide pins were spelled that way. Neither class rule could see
them, `TestQAEveryTreeWidePinHasADoor` never asked for a door, none existed,
and their only route was the ~950s package run:

```
TestQAPrescribedGitDiffChecksRunUnderADiffDriver  extdiff_qa_test.go  arm 1
TestQAPrescribedGitDiffChecksStateTheirFormat     extdiff_qa_test.go  arm 1
TestShelfPIDsLintCleanAsASet                      pidcheck_test.go    posse_arm2
```

The first two reach `extDiffSurfaces`, which read `../../AGENTS.md` and
`../../NOTES.md` by name and globbed `../../docs/notes.d/notes-*.md`; the
third globbed `../../examples/skills/*`. Their subject is the tree on any
reading — a bead that writes a `git diff` prescription into AGENTS.md,
NOTES.md or any notes section reds the first two, and a change under
`examples/skills/` reds the third. MEASURED (ranger-base-kkneb, worktree
dinesh-posse-ranger-base-kkneb, main/HEAD 3951338d, darwin 25.6.0,
go 1.26.10): `grep -c qibRepoRoot` is 0 in both files, no Makefile door
variable names any of the three, and `make register-check` was green over
all of them — the register saying it derives none of them.

This is ranger-base-sx2dq's hole re-opened a third time. That bead folded a
hand-rolled `filepath.Abs("../..")` into the helper and left
`TestQAOneRepoRootHelperInTheTestPackage` as the fence. The fence holds the
HELPER unique; it cannot see a climb that never asks for one.

### The fix: the root, not a third rule

The register's own stated lesson is the fix as well as the diagnosis — "the
class was never bounded by how a test spells its root, so the thing that
bounds it is the single helper, not another rule". So the three climbs take
their root from `qibRepoRoot` and join onto it. Nothing in the derivation
widened; the pins became derived, arm 2 then demanded doors, and each went
into the door whose subject it already shared:

```
TestQAPrescribedGitDiffChecksStateTheirFormat     make doc-check
TestQAPrescribedGitDiffChecksRunUnderADiffDriver  make doc-check
TestShelfPIDsLintCleanAsASet                      make seed-check
```

`doc-check`'s subject is prose pins over shipped code and docs, which is
exactly `extDiffSurfaces`'; `seed-check`'s is the published seed surface,
which is exactly `examples/agents` and `examples/skills`. No new door, so
the README door table and AGENTS.md's door block do not move. The head
comment's count sentence does: eighty-three pins to eighty-six.

1.26s between the three (0.00s, 1.21s and 0.05s, at a one-minute load
average of 97.58 — a busy box, and the number is a formality at this size).

What that costs the two doors, MEASURED 2026-10-31 on this box at a one-minute
load average of 8 (two readings each, the pins' own `ok ... Ns`):

```
doc-check    0.55s, 0.72s  →  1.19s, 1.06s     AGENTS.md's figure: ~0.1s → ~1.2s
seed-check   0.62s, 0.88s  →  1.23s, 0.64s, 0.84s   left at ~0.2s
```

`doc-check` really does carry the diff-driver arm's 1.21s — in parallel with
its siblings, so the door rises by about half of it. `seed-check`'s spread is
wider than the 0.05s the shelf pin adds, in both directions and with and
without it, so its figure is noise and the page keeps the number it had: a
price tag nobody can read to two digits is worse than a stale one.

`make tree-check` over the finished tree: **59s**, rc 0, at a one-minute load
average of 5.62 — the first reading of that command at the class's real size
(eighty-six pins, fourteen doors), and the quietest box it has been read on.
The three readings before it were 81-89s at eighty-three pins and a load of
25-28 (ranger-base-3e68k), which is the same answer the pinned sentence
already gives: price it at ninety seconds when the box is steady.

### The arm move, and the question it does not answer

`TestShelfPIDsLintCleanAsASet` was behind `//go:build posse_arm2`. A door's
`-run` runs with NO `-tags`
(`TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm`), and a `go test
-run` that matches nothing exits 0 — so a door there would have been green
over a pin it never ran. The pin moved to arm 1, in its own file
(`internal/posse/shelfpidlint_qa_test.go`); the rest of `pidcheck_test.go`
is the checker's own unit tests over a planted home and stayed.

That is one test changing files, not a mechanism. The same blocker stands
for the seven `posse_arm2`/`posse_arm3` members of ranger-base-m94g2's
fourteen one-package censuses, and whether a door learns to pass a tag is
that bead's decision. Nothing here prejudges it.

### The fence, and its control

The spelling is refused now, not merely fixed: a `WalkDir`/`Walk`/`ReadDir`/
`Glob` whose root is a literal climbing out of the package is the fourth arm
of `TestQAOneRepoRootHelperInTheTestPackage` (`twdClimbRoot`), beside the
twin helper, the hand-rolled climb and git.

Its live answer is zero by construction once the three are fixed, and a
prohibition nothing violates cannot be told from a broken detector — so the
arm carries a fixture table through `twdAnalyze`, the same function that
reads the tree: a climbing glob, a `ReadDir("../..")`, a climbing
`filepath.Join`, that Join under a `filepath.Clean`, and the three negatives
(one named file read through a climb is not an enumeration, an enumeration of
`"."` is the package's own directory, and a climb arriving through an
identifier is unread).

MUTATION-CHECKED: restoring `extDiffSurfaces`' glob to its literal reds the
fence alone, naming `internal/posse/extdiff_qa_test.go:673 extDiffSurfaces`.

### Residual, said out loud

A climb that arrives at the walker through an IDENTIFIER is unread by the
fence. It is live in the tree today —
`const dir = "../../etc/claude/managed-settings.d"; os.ReadDir(dir)` in
`policyhooks_qa_test.go`, which ranger-base-kkneb's own census of seventeen
did not see either, for the same reason. Closing it adds a class member and
a door; this bead's three were a defect, so that one is filed rather than
folded in here.

`shelfPIDs` (`shelfguard_qa_test.go`) is the same shape one hop further out:
it hands `shelfPIDsIn` a `filepath.Join("..", "..", "examples", "agents")`
and the glob happens at that helper's parameter, so three more tests read the
shelf through a climb no rule sees. Filed beside it.
