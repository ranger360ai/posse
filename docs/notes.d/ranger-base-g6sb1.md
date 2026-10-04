## The tree-wide door register was blind to a whole package, and what the second key had to be (ranger-base-g6sb1)

2026-10-04

`treewidedoor_qa_test.go` exists to make one promise: a QA pin whose subject
is the TREE, living in a package nobody runs whole, has a Makefile door a
seat can type in seconds. It has been widened twice before — once for a twin
repo-root helper (ranger-base-sx2dq), once for a root taken from `git
rev-parse --show-toplevel` (ranger-base-xndgk FINDING 5). Both times the
widening was within one package. This one is a second package.

### What escaped

Adding one `docs/notes.d/<bead>.md` fragment landed a commit that `make
fmt-check`, `make tree-check` (all eight doors as it then stood) and a full
`go test -tags posse_arm2 ./internal/posse` all called clean. The whole-tree
arm 1 run said, at 589.965s of `internal/treepins`:

```
--- FAIL: TestNotesFragmentIndexIsCurrent (0.13s)
    notesindex_qa_test.go:13: docs/notes.d/README.md: stale index;
    run python3 scripts/notes-index.py
```

A 0.13s property that cost 589.965s to reach — rulbl's gofmt bead, one
directory over. Writing a notes fragment is something nearly every bead on
this box does.

### Why no rule could see it

Every rule in the register keys, one way or another, on a test COMPUTING the
repo root: it calls `qibRepoRoot`, or reaches a helper that walks from a
root, or asks git. That key exists because `go test` runs internal/posse's
binary with the PACKAGE directory as its working directory, so reading the
tree costs an explicit climb.

`internal/treepins` has no climb. Its `TestMain` (configdirfence_test.go:49)
chdirs the whole binary to the repo root before any test runs, so a pin there
reads the tree through a plain relative path — `"scripts/notes-index.py"`,
`"docs/adr"` — and names no root helper at all. The register parsed only
`internal/posse/*_test.go` and would have found nothing in the other package
even if it had.

### The key, and the one that looked right and was not

The bead proposed "a test in a package whose TestMain chdirs to the repo
root, that reads a tree path". MEASURED 2026-10-04, by an AST census over
`internal/treepins/*_test.go`: **291 of that package's 362 tests name a path
the tree holds.** Four fifths of a package is not a class, and a door set
built on it would be the package.

What separates a pin an unrelated bead can red from one it cannot is whether
the reading ENUMERATES — whether the set of files read is spelled in the
test, or discovered. A test that reads `"Makefile"` reds for the bead that
edited the Makefile. A test that globs `"docs/notes.d/*.md"` reds for every
bead that writes a fragment. So the second key is a
`WalkDir`/`Walk`/`ReadDir`/`Glob` rooted at a relative tree path, propagated
through the package's own helpers. MEASURED: **20 tests**, against 9 if the
root had to be the repo root itself.

That is deliberately WIDER than the internal/posse rule, which excludes a
walk rooted at a subdirectory (`detectionRig` ReadDirs one directory under
the root; nine tests do that shape and none is tree-wide). The exclusion is
defensible there and would have been the bug here: `docs/notes.d` IS a
subdirectory, and it is the subdirectory every bead writes into. Drawing the
line at the root reproduces exactly the blind spot the bead was filed for.

### The member no Go rule can derive

`TestNotesFragmentIndexIsCurrent` enumerates nothing in Go. Its whole body is
`exec.Command("python3", "scripts/notes-index.py", "--check")`, and the glob
is a line of Python. A shell/Python source scanner for "does this program
read a directory" would be a second implementation of the pin's own reading —
the narrower-than-the-pin door this file refuses everywhere else.

So that half is REGISTERED (`twdIndirect`), and held honest three ways: the
row must name a real test, that test's body must really run that program, and
the program's source must still name the directory the row claims it
enumerates. Around it is a dispositioned census (`twdExecDispositions`) of
every tracked program a test in this package hands to `exec.Command` — eight
today, seven of which read only what they are handed. An undispositioned
program is a red, because a program's reading is outside Go and the default
has to be inclusion.

### What the register caught on the way in

Mid-change, `twdParseDir`'s glob was composed from its `pkg` parameter
(`filepath.Glob(filepath.Join(filepath.FromSlash(pkg), "*_test.go"))`). The
rule cannot read a variable, so every test reaching the parser stopped being
derived, and arm 2 red immediately with two doors it could no longer justify.
The globs are spelled out one literal per package now (`twdTestFiles`), for
the same reason internal/posse is held to one repo-root helper: a tree
reading is written in a spelling the rule can see. Arm 2's message for that
case says so by name, because it is the failure mode that will recur.

### What landed

Six doors over `internal/treepins`, and the register made two-way per
package. `notes-check` is a TOOL door — `python3 scripts/notes-index.py
--check`, 0.13s, no `go test` — allowed for `fmt-check`'s reason and no
other: the pin's whole body IS that command, so the two cannot disagree. Arm
1 reads the pin's argv out of the AST and holds the recipe to it, so a flag
added to either side without the other reds.

`make tree-check` is fourteen doors over forty-nine pins now: MEASURED
2026-10-04, 46.5s / 65.2s / 77.2s over three warm runs and 92.6s cold,
against 14.9-16.5s before. The spread is the box and not the build cache —
the one-minute load average went 8.9 to 23.8 across the three runs with a
sibling seat holding a suite slot throughout. The six new doors, measured
individually on the same box at load ~12: notes 0.31s, adr 1.52s, corpus
4.02s, scripts 0.88s, pid 10.04s, register 20.28s. The drift arm plants a
fragment with no index entry in a scratch copy of the tree and requires
`make notes-check` to refuse it, naming the README, "stale index" and the
script to run.

What is NOT proven for the five `-run` doors over internal/treepins is that
they can FAIL on real drift: the drift rig is a `git archive` scratch copy
with no `.git`, several of those pins read the tree through git, compiling
that package in the copy is a cold build of the module, and register-check's
own pins include the drift arm itself — arm 3 running it would be arm 3
running arm 3. What IS proven, by `TestQAEveryTreepinsDoorFilterNamesItsPins`,
is that make's own expansion of each filter selects exactly the pins its
variable names: `-list` substituted for `-run` in make's text, ~0.3s per door
against the ~20s of running them. A filter that matches nothing exits 0 and
prints `ok`, which is the half a `-run` door actually gets wrong.
