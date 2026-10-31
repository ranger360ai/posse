## Twelve tree-wide pins behind four spellings of a walk root, and the one spelling still refused (ranger-base-8wxbn)

2026-10-31

`internal/treepins`' half of the tree-wide door class is derived from a
directory ENUMERATION rooted at a relative literal — `twdRelTreePath` over
`twdWalkers` (ranger-base-g6sb1). That key exists because the package's
`TestMain` chdirs the binary to the repo root, so a relative path IS a tree
path and no root helper is ever named.

A literal is one of several ways to write that root. ranger-base-0ckgm found
the first one that is not: a walk root arriving as a PARAMETER
(`scanRealBdCallSites(root)`), invisible to the rule, so the pin was never
derived, never asked for a door, and reachable only by a 589.965s run of the
whole package. Its census said the parameter was one of six spellings. This
bead is the rest of them.

### The census

MEASURED 2026-10-31, over `internal/treepins/*_test.go` at
3b2a49e2 — every `WalkDir`/`Walk`/`ReadDir`/`Glob` call, classified by its
first argument. 39 calls; 23 root at a relative literal the rule reads and 16
do not:

```
a parameter                       arParse(p), cgScan(p), scanRealBdCallSites(root),   6 calls
                                  hfSnapshot(dir), lcsSources(dir), wrsRenderSites(dir)
a range var over a literal list   for _, root := range []string{"cmd","internal"}     4 calls
a root parsed out of the Makefile roots := fmtRoots(t, string(b))  (gofmtdoor)         2 calls
a local assigned a literal        dir := filepath.Join("docs","adr"); ReadDir(dir)     1 call
a named constant                  ReadDir(armPkgDir), armPkgDir = "internal/posse"     1 call
a helper's return                 root, _ := repoRoot()  (root_test.go)                1 call
a scratch root (correctly unseen) releaseFixture(t, version), out                      2 calls
```

28 tests reached a walker without being derived. Eleven of those are fixture
arms — the `...CatchesEachShape` family, `TestCredentialRegisterIsLoudBothWays`,
the two release arms, the hook-freshness arm, and ranger-base-0ckgm's own
three — which walk an `arFixture` or a `t.TempDir` and owe no door. The rest
are tree readings with no door.

### One correction to the bead's own table

The bead filed `lcsSources(dir)` and `wrsRenderSites(dir)` under *a
parameter*, which is what their declarations say and not what their callers
do. Every one of the four call sites reads

```go
root, err := repoRoot()
...
deciders, declarers := lcsSources(t, root)
```

so the root those four pins actually take is the HELPER'S RETURN, the same
spelling as `root_test.go`'s, and the fence the bead proposed for that
spelling covers them. That moves four pins out of the parameter row and is
why this landing doors twelve rather than eight.

### What changed

**`twdRelEnv`** — the identifiers, in one package and one function body, that
hold a relative tree path or a `[]string` of them. Package-scope `const`/`var`
initializers are read over every file at once (a const in one file roots a
walk in another; `armPkgDir` does), then each function body's own locals,
local declarations and range variables are layered on, two passes so an
assignment below the walk still binds. `twdRelTreePaths` resolves a walker's
root through it; `twdRelTreeList` resolves the `[]string` a range ranges over.

Two maps, not one: `for _, c := range "docs"` ranges over a string's runes, so
a range variable may only be bound from something the rule knows is a slice of
paths. One unreadable element makes the whole list unknown, because a range
over that list walks the element the rule could not read too.

Nothing crosses a function boundary. A `filepath.Join` whose first element the
env cannot name resolves to nothing, which is exactly what keeps
`filepath.Join(root, d)` out.

**`twdTreepinsRootHelper = "repoRoot"`** — the second rule, and the second
fence. A test that reaches `repoRoot()` AND reaches a walker is reading the
tree through an absolute root. Two reaches intersected rather than one flag,
because here the root and the walk are in different functions: the test takes
the root, and the helper it hands it to does the walking. The residual, said
out loud: a test that took the root for one purpose and separately called
something that walks a FIXTURE would be a false member. No test in the
package is in that shape, and the two that call `repoRoot()` without reaching
a walker — `TestLaunchCheckSourcesAllExist`,
`TestWallRenderSourcesAllExist` — stay out. What bounds it is the fence, the
same bargain ranger-base-sx2dq struck next door:
`TestQAOneRepoRootHelperInTheTestPackage` now holds `internal/treepins` to one
repo-root helper too, reddening on a `os.Getwd()`/`go.mod` climb under another
name, a hand-rolled `../..` walk, and a kept `git rev-parse --show-toplevel`.

**Twelve doors, no new door variable.** Every one went into the door whose
subject it already shared: the two `docs/adr` sweeps into `adr-check`, the
keychain-seam census and the four `repoRoot()` pins and the three suite-arm
pins into `corpus-check`, the entry-point timeout census into
`scripts-check`, and `TestTreePinsRunFromModuleRoot` into `register-check` —
that last being the marginal one, whose subject is the cwd `TestMain`
installs, which is what makes every relative literal in the package a tree
path.

Derived in `internal/treepins`: 28 → 40. Of the 40, 35 come from the
enumeration rule and 5 from the root helper, and arm 2 now floors both halves
separately, because a union floor stays green on one half while the other
goes quiet. Thirteen fixture cases were added to
`TestQATheTreepinsEnumerationRuleMatchesWhatItClaims` — seven positives for
the new spellings, six negatives including the parameter, a range over a
string, and a local assigned from a helper's return.

### What is still refused, and why

A walk root that arrives as a PARAMETER. `arParse(t, root, dirs)` and
`cgScan(t, root, dirs...)` are genuinely polymorphic — called with `"."` from
four and one tests over the tree, and with an `arFixture` temp root from eight
more — so the parameter cannot be replaced with a literal, and reading it
would mean following a value across a call boundary. That is the
interprocedural constant propagation `twdTestFiles`' head comment turns down,
and the spelling ranger-base-0ckgm was filed to pin rather than derive. Five
pins sit behind it today and are filed as ranger-base-pyl22 with the two
options written out.

One consequence worth stating for whoever lands ranger-base-0ckgm: its
`rpWaived` rows for `arParse` and `cgScan` do NOT come out when this bead
lands, which that bead's description predicted they would. Both helpers are
still parameter-rooted after this change, so a waiver pending a door is still
load-bearing; dropping those rows waits on ranger-base-pyl22.

A root parsed out of the Makefile (`gofmtdoor_qa_test.go`) is unread too and
costs nothing: that test also walks `"."`, so it is derived and doored on
that.

### Numbers

(Seventy-four was this BRANCH's count. `main` landed two more pins while
this bead was open, so the class is seventy-six where it actually landed, and
`make tree-check` was re-read there — 57.0-57.7s, in
`docs/notes.d/ranger-base-v3tt4.md`. The per-pin costs below are the same
twelve pins and carry over unchanged.)

`make tree-check` at seventy-four pins and fourteen doors: 112s, 62s and 56s
over three warm runs, at a one-minute load average of 14.21, 59.55 and 33.38
before the three. Both suite slots read free immediately before the three
and a sibling seat held one of the two by the time the third finished, so
some part of that window overlapped another seat's suite and this reading
cannot say which runs. The slowest run was the one at the lowest load by a
factor of four — ranger-base-c768n's and ranger-base-9mjxb's finding a third time: this
spread is not ordered by the load average.

Eleven of the twelve added pins report 0.70s between them. The twelfth is
`TestQAEverySuiteArmTypeChecks`, whose own line reads 0.14s because its three
subtests run in parallel; measured on its own they are 2.44s, 2.50s and
2.72s, so about 2.7s of wall. The four doors that grew:
`adr-check` 1.6s, `corpus-check` 5.0s, `register-check` 22.5s,
`scripts-check` 0.9s.
