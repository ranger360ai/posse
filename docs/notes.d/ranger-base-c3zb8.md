# ranger-base-c3zb8 — the arm-list venues are held to the partition, not to three

`ranger-base-qp1hm` split `internal/posse`'s tests into three build-tag
binaries, and `internal/treepins/armtags_qa_test.go` registers that partition:
`armExpr` maps each build-tag expression to its arm number and reds when a
file carries a tag the table does not name. Several places then RUN those
arms, each as a hand-written list. Nothing compared any of those lists to
`armExpr`, so a fourth arm tomorrow gets an `armExpr` row, a
`//go:build posse_arm4` file and a tag every existing census is satisfied by —
and then no venue line anywhere, no red, and a door reading green over an arm
it never ran. `go test` over a package whose tag matched no file exits 0.

`ranger-base-m33n5` filed it from `make tmpdir-check`, whose three recipe
lines were the newest member.

## The venue census

MEASURED 2026-11-22 at `97979d7d` — every place in the tree that enumerates
`internal/posse`'s arms, and what held each one before this bead:

| venue | shape | held by |
|---|---|---|
| `make test` | arm 1 (`./...`) + two tagged lines | `TestQAMakefileRunsEverySuiteArm`, with `for a := 2; a <= 3` |
| `make vet` | the same three — `go vet ./...` is arm 1 only | the same pin, the same literal 3 |
| `make test-arm<N>` | one target per arm, for ci.yml's jobs | the same pin, for `a := 1; a <= 3` |
| `make pkgcensus[-arm<N>]-check` | one `-run` door per arm | nothing |
| `make coldexec-check` | three of its five arms | `coldexecarms_qa_test.go` — over PACKAGES, not arms |
| `make tmpdir-check` | three arms, `$TMPDIR` off `/tmp` | nothing |
| `ci.yml` `arm: [1, 2, 3]` | which job numbers exist at all | nothing |
| `scripts/test-race.sh` | `arm_tag`, two `for i in 0 1 2` loops | its own census, and only via a candidate test |

The three pins that did read a venue each carried the number three as a
literal, so the pin was short by exactly the arm the venue was short by and
both were green. `coldexecarms_qa_test.go` is the worked example of a Makefile
arm list held to a derivation (`ranger-base-s64a5`), but its subject is which
packages plant an executable — `internal/posse`'s three tagged lines are one
entry of its five and the arm count is not its claim.

`scripts/posse-land-check` is the one venue that was already derived: it scans
the tree for `posse_arm[0-9]+` tags and vets each one it finds (its header
says so at line 49), and it is not a member.

## What landed

`internal/treepins/armvenues_qa_test.go`, six pins over one derivation
(`avArms`, the arm numbers `armExpr` declares, required to be `1..n` with no
gap because arm 1 is read as the default build):

1. `TestQAEveryPosseSuiteArmHasALineInEveryArmListVenue` — forward: every arm
   has a line in every registered venue (`avVenues`, the six Makefile rows).
2. `TestQAEveryMakefileArmLineIsInARegisteredVenue` — backward, swept over
   EVERY recipe in the Makefile rather than over the register's own targets. A
   hand-kept list of venues goes blind in one direction only — it cannot name
   the venue nobody added to it — and the derivation available here is the tag
   itself: a `-tags posse_arm*` line IS a declaration that something runs an
   arm, wherever it is typed. An arm tag outside a recipe is refused for the
   same reason.
3. `TestQACIArmMatrixIsTheSuiteArmPartition` — ci.yml's `arm:` matrix, both
   directions, the same shape as armtags' arm 8 over the `os:` matrix one key
   over.
4. `TestQATheRaceArmTagListIsTheSuiteArmPartition` — `arm_tag` and the index
   set of the loops that read it.
5. `TestQATheArmLineReaderRefusesTheWrongShapes` and
   `TestQATheMakefileRecipeReaderRefusesTheWrongShapes` — the controls. Arms 1
   to 4 are all one recipe-line reader, and a reader that answers "arm 1" to
   anything leaves arm 1 satisfied by a `go build`.

No Makefile, ci.yml or script change: all three lists are correct today at
three arms. What was missing was anything that would say so tomorrow.

## What it does not see

`scripts/test-race.sh`'s per-arm summary `printf` (three `%s` rows) and
`racearm_qa_test.go`'s `want 3` on the census output are still typed. Both RED
on a fourth arm rather than going quiet — a four-arm census fails `!= 3`, and
the printf drops a row — so they are a confusing red and not a silence, and
pin 4's error message names them both so the red is actionable.

## Mutation table

MEASURED 2026-11-22, linux 7.2.15 x86_64, go1.27.3, at `97979d7d`. Each mutant
applied to a tracked file and restored from a byte copy taken before the edit;
the battery is `scratchpad/mutate.sh`, not committed, and reproduced by the
rows below.

| mutant | reds |
|---|---|
| `"posse_arm4": 4` added to `armExpr` | pins 1, 3 and 4 — three venues short. **This is the bead.** |
| `tmpdir-check` loses its `-tags posse_arm3` line | pin 1 alone |
| `pkgcensus-arm2-check` renamed | pins 1 and 2 — no such target, and its tagged line is now unregistered |
| a tagged line rewritten `-tags=posse_arm2` | nothing (control: both flag spellings read) |
| `ci.yml` `arm: [1, 2]` | pin 3 alone |
| `ci.yml` `arm: [1, 2, 3, 4]` | pin 3 alone, other direction |
| `arm_tag` loses `posse_arm3` | pin 4 alone |
| one `for i in 0 1 2` becomes `for i in 0 1` | pin 4 alone (control: the array and its index set are two claims) |

Pin 2 staying green under mutant 1 is correct and is the complaint restated:
nothing in the tree runs the new arm, so there is no unregistered venue to
find.

## The door

Doored by the complement, unnamed (ADR 0098, `ranger-base-3cpcg`). Every pin
here reads a tracked file by relative literal and enumerates no directory,
which is the treepins rule's own negative case, so no `-run` door could find
them. `make treepins-check` is this package minus `$(QA_TREEPINS_EXCLUDED)`,
none of these names is in it, and that target is on `make tree-check`'s line.
