# The coldexec door's arm list, derived (ranger-base-s64a5)

2026-11-11. Discovered from ranger-base-in8dr; ADR 0085 D6.

## What was wrong

`make coldexec-check` (ADR 0085 D6, built under ranger-base-97w45) runs a
suite inside `cmd/coldexec`, the seccomp execve supervisor, so the FIRST exec
of every freshly planted executable is held for `COLDEXEC_DELAY`. The door is
only as wide as its arm list, and the arm list was hand-written recipe lines.

The set those arms should equal is derivable. A package whose tests can flake
on darwin's cold first exec is a package that PLANTS an executable, which is a
package with a `WriteExecutable` call site — the one executable writer in this
tree, which `make execwrite-check`'s
`TestQATreeGoFilesWriteExecutablesUnderTheForkLock` holds separately
(ranger-base-o6oj4, widened by ranger-base-6cznr).

Nothing compared the two sets. execwrite-check's floors are a per-directory
.go **file** count — a different claim, and one a package nobody injects into
satisfies — and the Makefile's coldexec block stated the coupling in prose
alone. A fourth package that started planting would have got no arm, no red,
and a door green over tests it never ran.

## The census

MEASURED 2026-11-11, by the pin, on this tree: **331 plant call sites in
exactly three packages** — internal/posse 261, internal/treepins 53,
cmd/posse 17.

The bead's description said 345 in those same three, which was
`grep -rn 'WriteExecutable('`. Reconciled: 331 calls, plus the two
`WriteExecutable` DECLARATIONS (internal/posse/execwrite.go, and
internal/treepins/execwrite_test.go — the package-local copy internal/posse's
`init.go` import cycle forces), plus twelve quoted mentions, six in
execwritesiblings_qa_test.go's positive-witness table and six in
execwritegates_qa_test.go's. The pin parses rather than greps, so a call
written inside a string literal is not a call site and a `func
WriteExecutable` is not one either.

## What landed

`internal/treepins/coldexecarms_qa_test.go`, two doored pins behind `make
corpus-check` and one undoored control:

- `TestQAEveryPlantingPackageHasAColdexecArm` — the derivation. Every
  planting package has an arm reaching it; no arm covers only packages that
  plant nothing; and every arm runs inside `$(COLDEXEC_BIN)`, because an arm
  outside the supervisor delays nothing and is a green suite over exactly the
  cost the door charges.
- `TestQAColdexecArmRegisterIsNotStale` — the coupling on the waiver register.
- `TestQAColdexecArmCensusCanStillSayNo` — the control, over both readers.

Door: `corpus-check`, whose subject is a census over this repo's own .go
sources. Not `execwrite-check`: that door runs internal/posse and asks whether
an exec-bit write is routed through `WriteExecutable` at all, which is not
this question.

## The register, and why there is one

coldexec-check had three arms on this date, all internal/posse. Extending it
to cmd/posse and internal/treepins is **ranger-base-in8dr**'s own deliverable
— after ADR 0085 D6's amendment (ranger-base-dacil) one recipe line per
package, inside the supervisor. So this pin could not demand those two arms
without reddening the suite over another bead's open work, and going quiet
about them was not an option either.

`cxaUncoveredPlanters` is the third answer: a row per package naming the bead
that will cover it. The derivation reds for a FOURTH planting package
immediately; the register's partner reds the moment a row stops being true —
the package has an arm now, or it has stopped planting. The row is the one
route into the derivation that does not go through the census, which is the
parking spot `treewidedoor_qa_test.go`'s `twdDoorHolders` keeps warning about,
so holding it to being true is what keeps this door from going back to a
remembered list.

**in8dr's seat will see that red** when it lands its two arms. The fix is to
delete the row, and the instruction is in the register and in the message.

## The reader the control arm caught

The first `cxaReadArm` stripped a `/...` suffix and compared DIRECTORIES. The
control arm's `./internal/...` case failed it: one pattern over both planting
packages under `internal/` read as an arm over a directory called `internal`
that plants nothing — two false findings in opposite directions over a recipe
line that is correct. The two spellings are not distinguishable without
resolving the path, and they do not need to be: an arm's coverage is a
PREFIX, which is what the pattern means to `go test`. `cxaArm.covers` is that,
with its own table in the control arm.

## Mutations

Four, because what this bead adds is two rules and a floor and all three are
assertions about absence. MEASURED 2026-11-11, each reverted immediately.

1. A plant call site in a fourth package (`cmd/coldexec`) reds the derivation
   alone, naming the package, its site count and the recipe line to add.
2. A `./cmd/posse` arm added while its register row stays reds the register
   arm alone — the case that will actually happen, when in8dr lands.
3. An arm with `./$(COLDEXEC_BIN) --` removed reds the derivation with the
   supervisor message, not the coverage one.
4. `cxaPlantWriter` going quiet reds the control arm's matcher cases AND both
   doored pins through the site floor, with the floor's own message. That is
   the case the floor exists for: without it the census would report every
   package as a non-planter and the derivation would pass with nothing to say.

## One red that was the tree working

`make execwrite-check` red on the control arm's own fixture. That census is a
BYTE matcher over every .go file in the tree, this file is inside its walk,
and the negative case `os.WriteFile(p, b, 0o755)` — written to prove the plant
matcher does NOT count a plain write — is a hit on this file's own source
whatever the Go around it says. Assembled now
(`"os.Write" + "File(p, b, 0o755)"`), which is what
`execwritesiblings_qa_test.go`'s own control already does and says why.

## Readings

All MEASURED 2026-11-11 on this box, one suite slot held by a sibling seat
(`dinesh-posse-ranger-base-in8dr`) for most of them.

- `make corpus-check`: 5.0s whole; the two new pins report 0.26s and 0.33s.
- `make register-check`: 29.8s, green with the head comment at one hundred and
  thirty-five pins and seventeen doors.
- `make tree-check` whole, three warm runs at the new class size: 46s, 38s,
  37s, at a one-minute load average of 3.43, 11.58 and 8.64 at each start.
  The slowest was the one taken at the LOWEST load, which is the entry worth
  reading — the register's earlier entries read the spread as tracking how
  much the load MOVES rather than how high it is, and three readings cannot
  settle that, but the direction is plainly not monotone at this end. An
  earlier run on this bead, loadavg 7.87 with BOTH slots held, read 46.2s.
- `internal/treepins` whole (`make treepins`): 130.5s, exit 0, after queueing
  161s for a suite slot.

A note on a number this page does NOT carry forward: the register's head
comment prices that package at 589.965s whole, and this run was 130.5s. That
is one reading against one, on a box in a different state and with a warm
build cache, so nothing here claims the package got faster — it is recorded
only so the next reader is not surprised by the gap.
