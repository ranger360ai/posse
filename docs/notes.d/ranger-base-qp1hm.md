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
in §7.

### 1. The subject

`internal/posse` at 6f94a99c: **112 product files, 430 test files, 2869
top-level tests**, one flat `package posse` (a `^func Test` sweep finds 2870;
the extra is `TestMain`). 2122 of the 2869 carry `t.Parallel()`; 747 do not.
CI run 34035314208 puts the package at **640.9s**, 42% of the 1500s budget and
over `SLOW_PACKAGE_SECONDS` (300) by 2.1x.

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

Measured rather than argued, because "raise `-parallel`" is the cheap answer
somebody will propose again. One `go test -c` binary, run from the package
directory over the same 427-test slice (`-test.run '^Test[CDL]'`, 751s of
weight), the values interleaved so a drift in box load could not pass for the
effect:

| `-test.parallel` | wall | 1-min loadavg |
|---|---|---|
| 4 | 168s | 67 |
| 24 | 126s | 135 |
| 24 | 138s | 197 |
| 4 | **296s** | 164 |
| 8 | 166s | 151 |
| 8 | 193s | 166 |

Six-fold more parallelism buys about 25%. The box's own load moves the
identical arm by 76%. Neither number is the 2.1x the 300s line needs.

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
set does not snowball: **61 files, 897 tests**. Every one of those 61 is
compiled into every arm, so any test still living in one runs once per arm.
Splitting a shared file (helpers stay untagged, its tests move to the tagged
original) buys that weight back, and the weight is in a handful of them.

#### Bin-pack on the predicted wall, not on the weight

The first cut balanced raw weight and put all three arms at 1705s each, which
*looked* even and was not: **arm 1 drew 325s of the 537.6s of serial weight**,
60% of it, because the door pins that have to live in arm 1 are the tree-wide
ones and tree-wide pins are exactly the tests that cannot take `t.Parallel`.
Weight is the wrong unit — a second of serial test costs the wall far more
than a second of parallel test, because only the serial ones cannot overlap.

Fit the two coefficients against CI's own reading. CI ran the whole 4563s of
weight in 640.9s, so it is 640.9/1091.8 = 0.587x this box per unit of weight;
that puts the serial stream at 537.6 × 0.587 = 316s there and leaves 325s for
everything else:

```
predicted CI seconds = 0.587 × serial_weight  +  0.0808 × parallel_weight
```

A second of serial weight costs **7.3x** what a second of parallel weight
costs, and by construction the whole package comes back as 640.9s. Under that
model the weight-balanced cut was arm 1 **302s**, arm 2 262s, arm 3 221s —
arm 1 sitting on the line it was supposed to clear, while the table said all
three were identical. Bin-packing the same components on predicted seconds
instead, and splitting five more shared files (the five whose duplicated cost
is highest — `dataceiling_qa_test.go`, `planblind_test.go`,
`launchlock_test.go`, `grokpool_test.go`, `instancepatternscope_qa_test.go`),
gives three arms of **235.9s each**: 21% under the line, evenly.

| shared files split | shared cost duplicated per arm | 3 arms | 4 arms |
|---|---|---|---|
| 15 | 71.9s | 261.6s | 214.1s |
| **20** | **33.4s** | **235.9s** | 185.3s |
| 61 (all) | 0s | 213.6s | 160.2s |

Three arms clears it, so three is what shipped. If CI ever disagrees, a
fourth arm is one tag, one Makefile target and one matrix entry, and the
table above says what it buys.

### 5. The plan, with its file lists

Tags: arm 1 is `//go:build !posse_arm2 && !posse_arm3` (so a bare
`go test ./internal/posse` still runs an arm rather than nothing), arm 2 is
`//go:build posse_arm2`, arm 3 `//go:build posse_arm3`. Shared files carry no
build line at all and compile into every arm. **The mechanism is verified**:
tagging one leaf file, `go test -list` finds its test under `-tags posse_arm2`
and does not find it in the default arm, `go vet` passes on both, and
`make fmt-check` is green over the `//go:build` line.

**The shared set starts as 61 files** — the 30 providers with the most
test-file users, closed over what they themselves use. Twenty are split, so
their helpers stay untagged and their tests take an arm; the other 41 keep
their tests and pay the duplication:

```
SPLIT: autoreap_qa_test.go autostart_test.go beadloss_qa_test.go
       constitutionwall_qa_test.go dataceiling_qa_test.go dispatch_qa_test.go
       gates_test.go gateschain_qa_test.go grokpool_test.go
       hookcluster_qa_test.go hookwallsweep_qa_test.go
       instancepatternscope_qa_test.go l3_hookspath_qa_test.go
       launchhookpreheal_qa_test.go launchlock_test.go planblind_test.go
       relaunch_test.go verifyafter_test.go worktree_qa_test.go
       worktree_test.go
SHARED: agents_test.go autoreap_test.go beadloss_test.go cage_test.go
        ciwatch_test.go commitwall_qa_test.go credcomposite_test.go
        credseam_test.go gatedkeychain_test.go govern_test.go herdr_test.go
        initembed_test.go instancebound_qa_test.go
        instancerefusalvalue_qa_test.go interstitial_qa_test.go
        launchline_qa_test.go launchlock_qa_test.go metaidentity_test.go
        modelavail_test.go plancache_test.go planguardpark_test.go
        planseam_test.go planusage_test.go promote_test.go
        pulse_delivery_test.go readyscan_test.go runtimecheck_test.go
        runtimegrid_qa_test.go runtimeoverlay_test.go seatbeltapply_qa_test.go
        seatbeltcarveout_qa_test.go seatbeltconstitution_qa_test.go
        seatbeltredirect_test.go seatbeltworktreegit_qa_test.go
        shelfguard_qa_test.go skills_test.go trust_test.go uncounted_test.go
        verifyesa0j_qa_test.go watch_test.go watchpid_test.go
```

**Split the helpers out, not the tests.** The obvious direction — move the
`func Test*` into a tagged companion and leave the helpers behind — was tried
and rejected on one file (`autoreap_qa_test.go`): it dropped 20 comment lines,
including the whole file-header paragraph that says what the pin family is
for, because in this tree that paragraph sits *after* the package clause and
is a floating comment, not anybody's doc. The right direction is the other
one: the original file keeps its prose and its tests and takes the arm tag,
and the non-test declarations move to an untagged `<base>_helpers_test.go`.
Those carry their own doc comments and travel cleanly.

**Four constraints the arm assignment has to honour**, all of them
door-shaped and all of them silent when broken:

1. Every test named by a Makefile door must land in **arm 1**, because the
   doors run `go test -run` with no tag. That is 24 names today:
   `$(QA_CREW_PINS)` (3), `$(QA_SEED_PINS)` (2), `$(QA_HISTORY_PINS)` (4),
   `$(QA_DOC_PINS)` (5), `$(QA_IDENTITY_PINS)` (2), `$(QA_OPS_PINS)` (4),
   plus `TestTreeIsGofmtClean` and `TestLiveRuntimeContractWalk`.
2. **A `-run` door that selects nothing exits 0.** So the door pins need a
   pin of their own — every door regex must match at least one test in the
   arm the door runs in — or the first pin to drift into arm 2 takes its door
   green and empty with it.
3. `go vet ./...` only sees arm 1. `make vet` has to vet each arm, or two
   thirds of the test tree stops being vetted the day this lands.
4. `make test` must still mean everything: three `go test` lines, each with
   its own `-timeout` (suitetimeout_qa_test.go's arm 2 sweeps the Makefile,
   `scripts/*.sh` and the workflows for a `go test` that inherits the
   default), and `scripts/test-times.sh` around each so all three arms are
   reported and measured against the 300s line.

CI runs the three arms as three jobs per platform, which is where the wall
clock is actually bought back.

### 6. What shipped

Option 2, three arms, sized by the model in §4.

- **389 test files carry an arm tag**; 41 carry none and are the shared
  helper set every arm compiles. Twenty files were split — the non-test
  declarations moved to an untagged `<base>_helpers_test.go`, the original
  kept its prose, its tests and the tag.
- **1276 / 1291 / 1234 tests**, 466 of them shared and therefore run in each,
  and the union is all 2869 (`go test -list` per arm). **235.9s an arm
  projected on CI**, evenly, against the 300s line.
- `make test` keeps its own recipe and gains two lines; `make test-arm1/2/3`
  exist for CI. `make vet` vets each arm — `go vet ./...` is the default build
  and would otherwise have left two thirds of the package unvetted.
- `ci.yml` gains an `arm: [1, 2, 3]` matrix dimension: six jobs, three clocks
  per platform, and a red that names which third it came from. Each job vets
  its own arm rather than all three.
- `suite_lock_wanted` now counts a tagged arm as a full suite. Without it
  `make test` took one slot and ran its other two arms unqueued — the
  five-concurrent-suites incident the lock exists for, rebuilt out of one
  seat. Self-test arm 5b, mutation-checked.
- `armtags_qa_test.go` at the repo root pins the partition in five arms:
  every test-bearing file is shared or in a named arm; `make test` runs every
  arm and every per-arm target repeats one of its lines verbatim; arm 1 is
  the default build and its target passes no `-tags`; every Makefile `-run`
  door pin is reachable in the default arm (a door whose filter selects
  nothing exits 0 — that is the failure that would have been found last); and
  the classifier itself is shown able to say no. Mutation-checked.

#### What this box could and could not measure

`make test` is green end to end. The per-arm walls it reports here are **not**
evidence about the 300s line and must not be quoted as if they were: the
weight-balanced cut ran 383.9s / 411.6s / 475.6s on 2026-09-06 with the
1-minute loadavg between 15 and 40, in an order that puts the *lightest* arm
last and slowest. The same box swung a fixed 427-test arm from 168s to 296s
inside one hour (§1). Arm 1 also runs inside `./...`, sharing the machine with
the root package and `cmd/posse`, where arms 2 and 3 each get it to
themselves. Nothing in that spread separates the arms from the hour.

**The 300s claim is CI's to settle**, and the first CI run on this branch is
the measurement. What this box does establish is that the partition is total,
that all three arms compile and pass, and that `make test` still means
everything.

And one cost to name rather than discover: a seat's `make test` runs the arms
in SEQUENCE, so its total wall goes up — three binaries do not do less work
than one, and each arm has fewer tests to overlap across the same eight cores.
The win is CI's three parallel jobs and three separate clocks, not a faster
seat. A seat in a hurry can run `make test-arm2` and `make test-arm3`
concurrently with `make test-arm1`; the suite lock will hold it to two at a
time, which is the point of the lock.

### 7. Reproducing the numbers

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
