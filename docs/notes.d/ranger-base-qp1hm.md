## Splitting `internal/posse`'s test binary: what each of the three options costs (ranger-base-qp1hm)

ranger-base-qp1hm names three ways to split the package and asks for "the
simplest split that gets under the line". This is the measurement that turns
that into a choice: **all three are expensive, and two of them are closed.**
Whoever executes the split starts here rather than from an estimate.

Everything below is measured on the tree at 6f94a99c with `go/types` over the
real test package (`packages.Load` with `Tests: true`), not by grep — an
identifier census by name alone over-counts badly, because locals shadow
package-level names (`line`, `write`, `err`, `out`, `cfg`, `at`) in dozens of
files. The tools are throwaway; the numbers are reproducible from the recipe
in §5.

### 1. The subject

`internal/posse` at 6f94a99c: **112 product files, 430 test files, 2870
top-level tests**, one flat `package posse`. 2122 of the 2870 tests carry
`t.Parallel()`; 748 do not. CI run 34035314208 puts the package at **640.9s**,
42% of the 1500s budget and over `SLOW_PACKAGE_SECONDS` (300) by 2.1x.

One full `-json` run on this box, 2026-09-06, `go test -json -count=1
-timeout 60m ./internal/posse` at 6f94a99c, 1-minute loadavg between 26 and
182 across the window (44 worktrees, other seats' suites running):

```
ok  github.com/ranger360ai/posse/internal/posse  1091.786s   0 FAIL, 31 skip
```

**1091.8s wall against 4563s of summed per-test elapsed** — an effective
parallelism of 4.2x on eight cores, with `-parallel` at its GOMAXPROCS
default. Two thirds of that summed time is the QA half:

| | summed elapsed | share |
|---|---|---|
| `*_qa_test.go` (265 files) | 2995s | 66% |
| everything else (165 files) | 1568s | 34% |

**Half the wall clock is a stream no flag can widen.** Go runs the
non-`t.Parallel` tests one at a time in the main goroutine; only the parallel
ones overlap. Split the same run by that flag:

| | tests | summed elapsed |
|---|---|---|
| serial (no `t.Parallel`) | 747 | **537.6s** |
| parallel | 2122 | 4025.7s |

537.6s of serial tests inside a 1091.8s wall — **49% of the run is a single
serial stream**. Raising `-parallel` above GOMAXPROCS is the obvious cheap
alternative to a split and it cannot touch that half: it can only compress the
4025.7s the other 2122 tests spend overlapping. Scaled to CI's own rate
(640.9s of wall for the same 4563s of weight, so ~1.7x this box), the serial
stream alone is ~316s there — over the 300s line **before a single parallel
test runs**. The only thing that divides a serial stream is more than one
binary, which is what the bead asked for and why no flag substitutes for it.

Summed elapsed is not CPU time — a `t.Parallel` test's Elapsed includes the
time it spent waiting — so treat it as the weight to bin-pack by, not as a
wall-clock budget. The ten heaviest files by that weight:
`autostart_test.go` 246s, `silentrevert_qa_test.go` 194s,
`queuecutover_qa_test.go` 186s, `queuecutoverspelling_qa_test.go` 159s,
`worktree_test.go` 150s, `dispatch_qa_test.go` 129s,
`constitutionwall_qa_test.go` 129s, `worktree_qa_test.go` 128s,
`gateschain_qa_test.go` 121s, `hooksredirectprobe_qa_test.go` 118s. That is
16% of the weight in ten of 430 files: still a tail, still no long pole to
cut instead (ranger-base-2ggb's finding, re-measured two weeks and 1428
tests later).

### 2. Option 1 — split by sub-package: CLOSED

A sub-package needs an import DAG. The product files do not have one.

Directed file-level graph over the 112 product files (an edge `a → b` when a
file uses a package-level identifier declared in another), Tarjan:

| SCCs | sizes |
|---|---|
| 12 | **101**, then eleven singletons |

**101 of 112 product files are one strongly connected component.** Six files
have no incoming edge at all (`cost_claude.go`, `cost_codex.go`,
`cost_grok.go`, `execwrite.go`, `planhint_codex.go`, `runtimecheck.go`); peel
those and nothing else peels. There is no sub-package to carve without first
breaking a 101-file mutual recursion, which is a different and much larger
bead than this one.

### 3. Option 3 — move `*_qa_test.go` into their own package: CLOSED at this price

The bead calls this one out as the bulk (265 of the 430 test files are
`*_qa_test.go`), and it would be the cleanest shape if the QA pins were
self-contained. They are not.

A moved file loses access to two things at once, and the second is the one
that surprises: **a test file's declarations are not importable from any other
package, exported or not** — test files are not part of the importable
package. So a helper in `herdr_test.go` cannot be reached from a sibling
package by exporting it; it has to move too, or be duplicated.

Counting the unexported identifiers the QA set uses that are declared outside
the QA set:

| what | distinct identifiers |
|---|---|
| declared in a product file (would need exporting) | **308** |
| declared in a non-QA test file (would need moving or duplicating) | **191** |
| total | **499** |

Only 20 of the 265 QA files use no unexported identifier at all. And the cost
does not concentrate — the union grows almost linearly with the set, because
the pins share very little:

| QA files moved (cheapest first) | distinct externals to export/move |
|---|---|
| 20 | 0 |
| 60 | 16 |
| 100 | 69 |
| 140 | 139 |
| 200 | 250 |
| 265 (all) | 499 |

The set that moves at **zero** cost — no unexported product identifier, and
closed under its own test-file dependencies — is **33 files** (30 of them
`*_qa_test.go`).

**And that is where it stops, at any budget.** The closure runs both ways: a
moved file needs everything it uses, and a moved file that *provides* a
declaration drags every file that uses it, because the ones left behind can
no longer see it. `herdr_test.go` — `TestMain`, `newTestBackend`, the fake
herdr — has **197 distinct test-file users**. So the moment one moving file
wants `newTestBackend`, `herdr_test.go` moves, and its 197 users move with
it, and their providers move, and the set is the whole package again.

Solving for the largest set that closes in both directions, greedily, at
increasing export budgets:

| exports allowed | files that can move | summed elapsed they carry |
|---|---|---|
| 0 | 22 | 22s (0.5%) |
| 10 | 33 | 41s (1%) |
| 25 | 45 | 48s (1%) |
| 50 … 320 | 55 | 49s (**1%**) |

Exporting more buys nothing after 50: what is left is not held back by
visibility, it is held back by the helper graph. **Option 3 can move at most
1% of the runtime.** It is closed, and it is closed for a reason no
identifier count would have shown.

### 4. Option 2 — build tags: OPEN, and the only one that is

Build tags keep every file in `package posse`, so nothing is exported and no
helper moves. `go test` and `go test -tags X` compile **different binaries**
with **different clocks**, which is what the bead asks for. The cost is
elsewhere: each arm has to compile on its own.

A file is atomic with respect to a build tag, so a file that both declares a
helper another arm needs *and* holds tests of its own has to be split in two —
helpers into an untagged companion (compiled into every arm), tests into the
tagged file.

Measured on the test-file dependency graph (968 edges over 430 files):

- **159 of 430 test files provide a declaration another test file uses**, and
  158 of those also hold tests of their own (1709 of the 2870 tests).
- Undirected, the graph is **one component of 375 files** — so no partition
  exists that splits nothing.
- On the natural QA / non-QA line, **90 provider files straddle the cut**
  (1162 tests). Those 90 would each need splitting.
- Untag the hubs instead and the graph shatters fast. With the top **30**
  provider files untagged, the largest remaining component is **42 files /
  285 tests** and there are 192 components — small enough to bin-pack into
  arms by measured seconds.

| top providers | distinct test-file users | own tests |
|---|---|---|
| `herdr_test.go` | 197 | 26 |
| `dispatch_qa_test.go` | 95 | 52 |
| `worktree_test.go` | 75 | 45 |
| `planusage_test.go` | 31 | 13 |
| `launchlock_test.go` | 22 | 12 |
| `cage_test.go` | 21 | 7 |
| `govern_test.go` | 18 | 49 |
| `gates_test.go` | 17 | 40 |

So the build-tag split is **~30 file splits plus a tag line on every
test-bearing file**, and then a Makefile arm and a CI job per arm. It is the
only one of the three that reaches the line, and it is not small.

#### How big the shared set is, and how few files really have to be split

Untag the top 30 providers and close over what *they* use, and the untagged
set does not snowball: **61 files, 897 tests, 1607s of weight**. Every one of
those 61 is compiled into every arm, so any test still living in one runs
once per arm.

That is the whole cost, and it is not paid evenly — the weight is in a
handful of them. Splitting a shared file (helpers stay untagged, its tests
move to a tagged companion) buys back its weight, and the heaviest ten buy
back 1183s of the 1607s:

| shared files split | shared weight still duplicated | 3 arms | 4 arms |
|---|---|---|---|
| 0 | 1607s | ~364s CI | ~329s CI |
| 5 | 824s | ~291s CI | ~247s CI |
| **10** | **424s** | **~253s CI** | ~205s CI |
| 15 | 276s | ~239s CI | ~189s CI |
| 61 (all) | 0s | ~214s CI | ~160s CI |

**Ten splits and three arms clears the line**; fifteen clears it with room.
The other ~46 shared files keep their tests and pay the duplication, which is
cheaper than splitting them. The ten are `autostart_test.go` (246s),
`worktree_test.go` (150s), `dispatch_qa_test.go` (129s),
`constitutionwall_qa_test.go` (129s), `worktree_qa_test.go` (128s),
`gateschain_qa_test.go` (121s), `gates_test.go` (106s),
`l3_hookspath_qa_test.go` (94s), `verifyafter_test.go` (41s),
`autoreap_qa_test.go` (38s).

So the shape is: **~10-15 file splits, a build-tag line on the ~369 remaining
test-bearing files, three Makefile arms, three CI jobs, and one pin that the
partition is total.**

The CI projection assumes each arm keeps the whole package's parallelism
profile. That is the first thing to check on the branch that builds this: the
747 serial tests do not spread evenly by weight, and an arm that inherits a
disproportionate share of them will run longer than its weight says.

### 5. Reproducing the numbers

The census is a ~120-line `golang.org/x/tools/go/packages` program: load
`./internal/posse` with `Tests: true`, take the test variant of the package,
map every package-scope object (plus methods and struct fields) to its
declaring file, then walk `TypesInfo.Uses` per file and record every use whose
declaring file is not this one. Classify the declaring file as product /
non-QA test / QA test, and the identifier as exported or not. Everything in
§2-§4 is a projection of that one table. Doing it by name alone (`grep`, or an
AST walk with no scope resolution) inflates the count by ~15% and puts
`line`, `write`, `err` at the top of the ranking, which is how you know it is
wrong.
