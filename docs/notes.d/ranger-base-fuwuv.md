# The land check's cross product: every arm under both platforms, and what the two new steps cost (ranger-base-fuwuv)

`scripts/posse-land-check` had five steps. Two of them vetted the build arms,
one of them vetted the not-native GOOS, and no step did both — so the cross
product of the thing this gate derives (the arm tags) and the thing only this
gate asks about (the other platform) was empty.

```
step 1: go build ./...
step 2: go vet ./...
step 3: go vet -tags posse_arm2 ./internal/posse     <- native GOOS only
step 4: go vet -tags posse_arm3 ./internal/posse     <- native GOOS only
step 5: GOOS=darwin GOARCH=amd64 go vet ./...        <- untagged only
```

Dated 2026-11-22. Every number below was taken on the D1 host (Linux 7.2.15,
16 cores, `go version go1.27.3-X:nodwarf5 linux/amd64`, `GOTOOLCHAIN=local`),
from this bead's worktree, at the load average each row names.

## What the hole was

**MEASURED 2026-11-22: 340 of internal/posse's 981 `.go` files are compiled by
neither of the two vets that ran.** The package's build lines come in three
spellings — `//go:build !posse_arm2 && !posse_arm3` (322 files),
`//go:build posse_arm3` (178) and `//go:build posse_arm2` (162), with 319 more
carrying no arm constraint at all — and the 340 under the two positive tags
are out of the default build. `go vet ./...` does not compile them (that is
arm 1), `go test ./...` does not either (AGENTS.md's ranger-base-qp1hm
paragraph), and the one step in this tree that asks about the other platform
carried no `-tags`. By test count rather than file count the same two arms are
about two thirds of the package (ranger-base-qp1hm).

So a file under either tag naming a linux-only symbol — `unix.Ppoll`,
`unix.Pidfd*`, anything with a `_linux.go`-shaped API — read green through
`make fmt-check`, `make tree-check`, `make test` and `make test-arm2` here and
failed to BUILD on ci.yml's macos leg. That is ADR 0087's residual 1 and this
script's own "BOTH PLATFORMS" paragraph, one tag deeper: the class those two
exist for, in the two thirds of a package they could not see.

**Found by hand, which is the whole argument.** ranger-base-njj5z's fix added
`golang.org/x/sys/unix` (Open/Read/Poll/PollFd/POLLIN) to
`internal/posse/refusalleaf_test.go`, a `//go:build posse_arm2` file. Its seat
ran `GOOS=darwin GOARCH=arm64 go vet -tags posse_arm2 ./internal/posse` and
the amd64 form BY HAND, because nothing in the tree asked. Those symbols are
portable, so that landing was fine. The next one is fine only if its seat has
the same instinct.

Not a vet question about njj5z's actual red: that was a runtime capability
(`SetReadDeadline` over a FIFO), which no vet can see. This is the compile
half of the same wall.

## The fix

The arm loop became a function, `arm_vets`, called once with no arguments and
once per cross-GOOS pair with `GOOS=… GOARCH=…` as its leading words — env's
own operands, the same spelling the untagged cross step already used, so a
tagged cross step reads in the log exactly like the untagged one above it. In
this tree today:

```
step 6: GOOS=darwin GOARCH=amd64 go vet -tags posse_arm2 ./internal/posse
step 7: GOOS=darwin GOARCH=amd64 go vet -tags posse_arm3 ./internal/posse
```

A function and not a second loop, because two call sites deriving two arm
lists is this hole reopening with nobody editing the derivation — which is
also what the pin's fixture is shaped to catch (below).

Both new steps are **green at HEAD**: `GOOS=darwin GOARCH=amd64 go vet -tags
posse_arm2 ./internal/posse` and the arm3 form each exit 0, so this closes a
hole rather than reporting a break. Under `GOOS=darwin`, `go list` names 261
test files for arm 2 and 277 for arm 3 against 416 for the default build, so
the steps are compiling a real file set and not resolving to nothing.

## What it costs (MEASURED 2026-11-22)

BEFORE is `git show HEAD:scripts/posse-land-check` run from the scratchpad,
AFTER is this worktree's copy; the readings alternate, and each pair's order
is reversed in the next pair, because the first run against any tree pays for
whatever that tree's path has not been compiled under yet.

| state of the cache and the tree | loadavg | before | after |
|---|---|---|---|
| warm, this worktree, nothing to recompile (3 pairs) | 6.40-7.33 | 1.65 / 1.66 / 1.82s | 2.03 / 2.05 / 2.47s |
| the darwin build cache cold, native halves warmed first (2 pairs) | 7.02-11.07 | 11.38 / 11.07s | 13.35 / 14.76s |
| a freshly exported tree per reading, shared cache warm (3 pairs) | 6.00-8.39 | 11.45 / 9.63 / 9.53s | 14.15 / 13.37 / 15.81s |
| cold, an empty `GOCACHE` per reading (2 pairs) | 7.09-12.34 | 24.62 / 21.55s | 24.71 / 24.33s |

Read down the last column: **+0.4-0.65s fully warm, +2.3-3.4s with the darwin
cache cold, +3.9-6.3s on a tree nothing has compiled, and +0.1-3.2s cold** —
the cold row being the cheapest of the four in relative terms, because an
empty cache makes step 5 compile the whole darwin dependency tree anyway and
the tagged steps then type-check test files over objects that are already
there.

The exported-tree row is the one that prices a replay (it is pn621's method:
`git archive | tar -x` into a directory nothing has ever compiled, with the
fleet's shared `GOCACHE` warm). Each reading is a SEPARATE export, because
running both scripts over one directory gives the second one a tree the first
one warmed — the confounded version of that row read 11.9s before and 4.1s
after, in that order, which is an artifact and not a finding.

The three cross-GOOS steps alone, fully warm and fully cached: 0.30s untagged,
0.17s arm 2, 0.17s arm 3 — 0.65s for the three where the Makefile's note
recorded 0.3s for the one.

`make land-check` is **2.0-2.2s warm** over three runs at a loadavg of
8.48-8.95, where it was 1.5s with five steps.

**The hard bound is untouched**: D6 gives the check `generatorTimeout`, which
is 2 minutes (`internal/posse/generatedindex.go`), and the slowest reading of
any kind above is 24.7s.

**Against ADR 0087's budget, warm holds and no handback is filed.**
ranger-base-pn621 set a ~30s *warm* ceiling at which the lock-time price in
the ADR's cost paragraph stops holding and this goes back to architecture. The
slowest warm reading here is 15.8s, a little over half of it.

**Worth saying anyway, because it is the trend and not the step count**:
pn621's exported-tree readings on 2026-11-11 were 7.1-8.7s for the five-step
script, and the same five steps read 9.5-11.5s today at a comparable load.
About 2.5s of the gap to today's 13.4-15.8s is the tree growing under the
check in eleven days, and about 4-6s is this bead. The headroom against the
ceiling went from roughly 3.5x to roughly 2x, and the next thing to eat it
will be the tree rather than a step.

## The pin

`internal/posse/landcheckplan_qa_test.go` arm 9,
`TestQALandCheckCrossVetsEveryDeclaredArmAndNotOnlyTheUntaggedBuild`. Like the
eight beside it, it reads `--plan` output over a PLANTED module and runs no
toolchain step, so the nine arms together cost **0.043-0.059s** over four runs
(`go test -tags posse_arm2 -run TestQALandCheck ./internal/posse`, package
time; the earlier note's 0.14s for eight was taken on another day's box).

What it asserts is that the cross arm list IS the native arm list, verbatim
and in order, under one platform — not a count, and not a literal step list.
The fixture declares a third arm in a SECOND package for that reason: a cross
half that re-derived its own list, or kept a literal `./internal/posse`, would
agree by coincidence over a one-package fixture.

Two mutants, both run against the real script:

- the cross call deleted → `the plan cross-vets ⟨nothing⟩ and vets natively
  posse_arm2/posse_arm3/posse_arm4`, one red naming the three;
- the cross call rewritten as its own loop over a literal `./internal/posse` →
  red on the third row, `posse_arm4 ./internal/posse` against
  `posse_arm4 ./internal/newthing`.

The first mutant found a defect in the pin rather than in the script: with no
cross arms at all, the platform-agreement loop indexed `platforms[1:]` over an
empty slice and turned the report above into a panic. It is bounded now, and
the comment says why — a plan with no tagged cross vets is the hole itself,
already reported, and a panic there would bury the report it was printed with.

## Two residuals, one of them inherited

**STATED, unchanged**: the `!darwin && !linux` files compile under neither
GOOS, here or in ci.yml. A third GOOS and a price nobody has measured.

**INHERITED, and not new to the cross product**: the derivation names the
PACKAGE that declares a tag, and `go vet -tags T ./pkg` over a package whose
files are all excluded under the GOOS being vetted exits 1 with `build
constraints exclude all Go files` — a spurious red at a gate where a spurious
red blocks a landing. That is already reachable at the NATIVE arm vets: an arm
declared only in darwin-only files reds on a linux launcher today, with no
cross step involved. The cross product widens the surface and adds no class.
MEASURED 2026-11-22: no package in this tree declares an arm in a file
constrained to one GOOS, and both new steps are green. Filed as a defect by
nobody, because nothing measured it wrong today; the day it bites, the red
names the package and the fix is to constrain the vet rather than the file.
