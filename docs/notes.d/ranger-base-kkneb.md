# The ordering pin ranger-base-cwsi8 left in a comment, and seventeen internal/posse censuses nothing derives (ranger-base-kkneb)

2026-10-31. Two of the three findings ranger-base-49iga filed against four
verified closes. FINDING 2 is ranger-base-sczjy's, a live dispatch-correctness
defect with its own bead and its own assignee. What is here is FINDING 1 —
built — and FINDING 3, whose deliverable was a live carrier for a decision
and which the measurement below widens into two.

## FINDING 1 — the resolution ordering, now pinned

`Bd.runOnce` resolves the binary it is about to exec BEFORE it takes ADR
0073's per-store write lock (internal/posse/beads.go). ranger-base-cwsi8 made
that an explicit claim in a code comment — "Before the lock, not after: a
binary we cannot resolve is an answer already, and taking a store lock to
find that out would hold the store shut for nothing" — and pinned the
resolution without pinning the ORDER.

MEASURED 2026-10-31, worktree dinesh-posse-ranger-base-kkneb, main/HEAD
3951338d, darwin 25.6.0, go 1.26.10. Moving the `bin, err := b.bin()` block
from above the `if bdVerbWrites(argv)` block to immediately before `limit :=
b.timeout()` left all four pins in bdgatesfreepath_test.go green — exit 0,
0.998s on ranger-base-49iga's reading, reproduced here. The fixture of
`TestBdBinUnresolvableOutsideTheGatesIsAnError` holds no contending lock, so
its third assertion ("the error says which binary it could not find, not that
a lock was held") cannot fire on that mutant: with nothing holding the lock,
the reversed order takes it, finds the unresolvable binary one line later and
returns the same refusal the right order returns.

What the reversed order costs on a live seat is the diagnosis
ranger-base-c33j6 was filed for: a box whose PATH holds no `bd` outside the
gates AND a concurrent posse write in flight waits the whole of
`BdWriteLockWait` and then reports `<store>'s bd write lock was held for 30s`
— posse waiting on itself, about a binary it could have refused at once.

THE PIN: `TestBdBinUnresolvableRefusesWithoutWaitingOnTheStoreLock`
(internal/posse/bdgatesfreepath_test.go). The same wall as the test above it
— a gates bin dir holding a `bd`, no non-gates element that answers — plus
the holder that fixture has not got: this store's write lock, taken by the
test itself under its own `XDG_CACHE_HOME`, held across the call. Two
assertions, because what it says and how long it took to say it are different
claims: the refusal names the unresolvable binary and is not the lock line,
and the call returns inside `BdWriteLockWait / 6`. A control first — a second
attempt on the same store must time out — so the arms below cannot be green
because nothing contends.

MUTATION-CHECKED 2026-10-31: the move above reds this pin on all three arms
and nothing else in the file — 30.002s against a 5s budget, the refusal being
`... bd write lock was held for 30s ...` verbatim. Restored, md5
`bb208464f36ee57d33c636969e38360a`, re-run green at 0.16s.

## FINDING 3 — the decision had no carrier, and the class is seventeen

ranger-base-8wxbn recorded a decision under "ALSO FOUND" and scoped it out:
whether a ONE-PACKAGE census — a QA pin that reads its own package directory
through a relative literal — belongs in the tree-wide class that
`treewidedoor_qa_test.go` derives and the Makefile doors. Its words: "it is
stated here so it is not re-discovered." It was stated in a bead that closed.
ranger-base-49iga named `ranger-base-pyl22` as the natural carrier; pyl22
closed the same day (its own merge-back is ranger-base-3e68k), so the
decision had no live pointer at all.

It is also bigger than the two pins the finding names.

MEASURED 2026-10-31, over `internal/posse/*_test.go` at main/HEAD 3951338d, by
the one-off census in the appendix: **17 tests** reach an enumeration
(`os.ReadDir` / `filepath.Glob` / `filepath.Walk` / `filepath.WalkDir`) rooted
at a relative literal — directly, through a helper they reach, or through a
helper's root PARAMETER resolved at the call site (the one hop
ranger-base-pyl22 landed for internal/treepins). **None of the 17 is named by
any Makefile door variable**, and `make register-check` is green over all of
them, which is the register saying it derives none of them.

They split in two, and the split is the whole of why this is two beads:

FOURTEEN read the PACKAGE directory and nothing else (`.`, `*.go`,
`*_test.go`). These are the decision. By the class's own words — "a QA test
whose subject is the TREE" — they are out, and `herdrhintsgone_qa_test.go`'s
head comment already says so from the other side: "It reads THIS directory
and cockpit.go's own package reads its own, rather than walking from the repo
root: a tree-wide pin is nobody's `-run` subject and needs a Makefile door."
That is a deliberate scoping, not an oversight. What it does not answer is
reachability: an unrelated landing that adds a file to internal/posse can red
any of the fourteen, and the only route to them is the ~950s package run —
the ranger-base-ik44f shape one definition over.

THREE do not read the package directory at all. They climb OUT of it with a
relative literal:

    TestQAPrescribedGitDiffChecksRunUnderADiffDriver  extdiff_qa_test.go  arm 1
    TestQAPrescribedGitDiffChecksStateTheirFormat     extdiff_qa_test.go  arm 1
    TestShelfPIDsLintCleanAsASet                      pidcheck_test.go    arm 2

The first two reach `extDiffSurfaces`, which reads `../../AGENTS.md`,
`../../NOTES.md` and globs `../../docs/notes.d/notes-*.md`; the third globs
`../../examples/skills/*`. Their subject is the TREE on any reading, and a
bead that adds a `git diff` prescription to AGENTS.md, NOTES.md or a
`notes-*.md` section reds them. Neither posse-half rule can see them: both
key on the ONE repo-root helper (`qibRepoRoot`, instancebound_qa_test.go), and
none of these three files names it. This is ranger-base-sx2dq's third hole
re-opened — that bead folded a hand-rolled `filepath.Abs("../..")` into the
helper for exactly this reason, and the fence it left
(`TestQAOneRepoRootHelperInTheTestPackage`) holds the HELPER unique without
being able to see a climb that never asks for one.

Filed, not fixed, both of them (strict scope, and the register file and
Makefile are in flight under ranger-base-3e68k):

- `ranger-base-m94g2` — the decision, `-l debt`: fourteen package-directory
  censuses, in or out, and if in then by which rule.
- `ranger-base-nt0pi` — the three climbs, `-t bug`: tree-wide pins outside
  the class because of how they spell their root.

What makes either of them non-trivial is the same two gates, and they are
written down here so neither bead re-derives them:

1. A door variable may only name a test the register DERIVES — arm 2 is
   two-way — or a `twdDoorHolders` row, which that var's own comment calls the
   parking spot this file keeps warning about. So no member of the 17 can be
   doored by adding a name to the Makefile; the derivation decides first.
2. A door's `-run` runs with NO `-tags`, held by
   `TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm` over `armDoorVars`.
   EIGHT of the 17 are in `posse_arm2` or `posse_arm3` — seven of the
   fourteen, plus `TestShelfPIDsLintCleanAsASet` from the three — and they
   include `TestQAEveryTestBinaryReExecKeepsTheChildATestBinary`, one of the
   two ranger-base-8wxbn named. For those a door is not a Makefile row at
   all: either the test moves arm (and `armtags_qa_test.go` keeps the
   partition total) or the door mechanism learns to pass a tag, which no door
   does today. So the two beads hit one shared blocker, and whichever lands
   second reads the other's answer rather than inventing a second one.

## Appendix — the census, and how to re-run it

Kept here rather than tracked: it is a one-off measurement, not a pin, and a
program a tree pin runs would need a disposition
(`TestQAEveryTrackedProgramATreePinRunsIsDispositioned`). `go run . <pkgdir>`
with a `go.mod` of its own, outside this module.

Rule: a hit is an enumeration whose root argument is, or is composed of, a
relative string literal — read in the test, in any function the test reaches,
or handed to a callee's enumeration-rooting parameter at the call site (one
hop, no transitive value flow — the propagation both head comments in
internal/treepins refuse).

```go
// pkgcensus — which tests in a package census a relative literal root.
// (Walks *_test.go in one directory; prints test, file, build tag, roots.)
//
//  1. parse every *_test.go, recording for each func: the relative-literal
//     roots it enumerates (a BasicLit, or a filepath.Join of literals), and
//     the indexes of its own parameters that root an enumeration (the ident
//     itself, or any ident inside a Join that builds the root);
//  2. for every call site, if the callee roots an enumeration at parameter i
//     and the site passes a relative literal there, record a hop;
//  3. a test is a hit if it reaches a func with a root, or makes such a hop.
```

Live answer, 2026-10-31: 17 hits — 14 package-directory, 3 climbing out.
Arms: 9 in arm 1 (`!posse_arm2 && !posse_arm3`, or no build tag at all and so
shared), 4 in `posse_arm2`, 4 in `posse_arm3`. The per-test table, with file
and root, is on the two beads.
