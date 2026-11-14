# ADR 0098 — The treepins door is the package minus a priced exclusion list

*Status: accepted 2026-11-14 (ranger-base-m6mzg) · owner: architect · changes
the internal/treepins half of the tree-wide-pin class in
`internal/treepins/treewidedoor_qa_test.go` · the internal/posse half is
unchanged*

## Context

`twdDoorHolders` is the one hand route into a door variable. Its comment set
a bound of about two rows. It had three rows before ranger-base-n7hsa and nine
after. Seven of the nine are there for one reason: the pin reads a tracked
file it names, so the enumeration key cannot see it, and any bead that edits
that file turns the pin red.

The key is "a walk rooted at a relative tree path". That rule was written to
exclude named reads (ranger-base-g6sb1), for two reasons:

1. "A test that reads `Makefile` reds when somebody edits the Makefile, which
   is the bead that edited it."
2. "291 of internal/treepins' 362 tests name a path the tree holds. Four
   fifths of a package is not a class."

**Reason 1 is false in practice.** The bead that edited the file is
responsible for the red, but its seat never sees it. Nothing leads from
AGENTS.md to `pushmandate_qa_test.go`, so no `-run` filter names the pin.
n7hsa measured exactly that: `make tree-check` was green over a red AGENTS.md,
and the red surfaced 137s into `make test`'s arm 1, which then aborted before
arms 2 and 3 ran. The doors exist for a pin whose red a seat cannot reach. A
named tracked file can be that kind of red as surely as a directory walk can.

**The key also misses in a second direction.**
`TestNoTrackedFileCarriesADigitRealisticDetectorShape` reads every tracked
file through `exec.Command("git", "ls-files")`. That is the widest reading of
the tree in the package. It has no door and no row, because an exec is not a
WalkDir. No rule built on the shape of the source code will be complete.

**Reason 2 is about cost, and the cost was never measured.** MEASURED
2026-11-14, D1 host, Linux 7.2.15, go1.27.3, 16 cores, warm build cache, at
37a94cd9, through `scripts/test-times.sh`, with one suite slot held by another
seat:

| run | wall | load (1m) |
|---|---|---|
| `go test ./internal/treepins` (all 478 tests that ran) | 159.8s | 2.6 → 13.4 |
| the same, `-skip` the 15 undoored tests ≥ 2.0s | 68.9s | 9.6 → 7.6 |
| today's treepins doors (notes, adr, corpus, register, scripts) | ~33s | 11.6 → 11.4 |

In the complement run, about 34s is one pin that is already doored
(`TestQATheTreeWideDoorsReportRealDrift`). The 15 tests left out are
self-tests of the suite wrappers (`test-times.sh`, `suite-lock.sh`) and the
race-arm census, which compiles the arms. They account for 172s of the
package's 245s of summed test time. The 589.965s figure the register quotes is
a darwin reading from 2026-10-04 and is not this box's price.

So "four fifths of a package" is affordable as one door, as long as the door
is spelled as everything except a short list. It is not affordable as a
four-fifths list of names, and that list was the only spelling g6sb1 priced.

## Decision

**D1. In internal/treepins, a test is in the class unless it is excluded.**
The door is `go test ./internal/treepins -skip '^($(QA_TREEPINS_EXCLUDED))$'`.
It is a new target, and `tree-check` lists it as a prerequisite in place of
the five treepins doors. Those five, and orientation-check, stay as targets a
seat can type on its own, because each runs a subset of the new door. A new
pin is doored the moment it is written. Nobody has to notice it after an
escape.

**D2. An exclusion is a priced row, and the table is bounded by price, not by
a count.** `twdTreepinsExcluded` maps a test name to a row. Each row gives the
MEASURED seconds, the box and the date, and the route that still runs the test
(`make test`, or a named door). The Makefile variable is generated from the
table, or the two are held equal by a two-way arm. The row's reason is its
price. A row may not be justified by "it reads a file by name" or "it is a
control arm". Those were the old parking-spot reasons, and under D1 neither
one earns an exclusion. The floor is 2.0s measured. A test cheaper than that
costs the door less than the miss would cost a seat.

**D3. An excluded test keeps a door if the old key derives it.** If the
enumeration rule or the root-helper rule derives a test (the existing
derivation, kept as is), the test cannot be excluded unless it also has a door
of its own. So nothing that is doored today loses its door. Under D1 the
derivation no longer decides which tests are members. What it still decides is
which members cannot be excluded without a door of their own.

**D4. `twdDoorHolders` retires for internal/treepins.** When every test that
is not excluded is a member, a door variable that names any treepins test is
valid without a row. The two-way check for this package becomes: every
declared test is either run by the complement door or has an exclusion row,
and every row names a real test. The nine rows are deleted, along with the
"two rows" sentence and its replacement.

## Consequences

- `make tree-check` costs about 36s more here: 68.9s for the complement door
  against ~33s for the doors it replaces (MEASURED, one reading each, load
  7-12). The figure is per box and per load, like every tree-check reading
  in this repo.
- Its CPU is about 109s of summed test time over about 69s of wall, roughly
  1.6 cores. ASSUMED: that does not make it a suite by cost, so it takes no
  slot. The implementation measures this under a loaded box and corrects the
  assumption if it is wrong.
- A slow test added to treepins slows `tree-check` until someone gives it a
  row. That cost is visible: `test-times.sh` prints the package time. Today's
  failure is invisible: a pin red only at 137s into `make test`.
- The internal/posse half keeps its derived key. Its package is three arms
  totalling about 950s, and no complement door of that size fits. If the same
  shape escapes there, it needs its own ruling.
- **Exit hatch.** The table is the only state. Raising the 2.0s floor makes
  the door cheaper and the escape window wider, one row at a time. To revert,
  restore `tree-check`'s five prerequisites and `twdDoorHolders` from git.
  Nothing outside the register and the Makefile depends on this.

## Alternatives rejected

- **Widen the key to "reads a named tracked file".** This is the bead's
  shape 1. The class becomes four fifths of the package, and its door variable
  would be about 400 names the Makefile spells by hand. It would still miss
  the `git ls-files` census. It fixes membership but keeps the spelling that
  made membership expensive.
- **Keep the hand register and bound it with a written rule** (each row must
  name the unrelated writer). That makes the rows honest, but the register
  still grows only after an escape. Every row so far arrived with the red that
  justified it, and about 290 named-file pins have no row because nothing has
  escaped from them yet.
- **Move the heavy pins to their own package and run treepins whole.** This
  is the clever one, because the package boundary would be the exclusion list
  and no table would be needed. Rejected because it moves 15 tests and their
  shared helpers across a package boundary (the TestMain chdir, armtags,
  wrapper fixtures) to save a table of about 15 rows, and it puts a second
  package of tree pins into a register that already has two rules for two
  packages.
- **A derived exclusion** ("reaches an exec of `go` or a suite wrapper"). Most
  of the 15 fit that rule. `TestQABdArgvGateResolvesTheVerb` (2.79s) does
  not, and a cheap test that happens to exec `go` would be excluded by the
  rule without anyone pricing it. The price is the reason, so the price is
  what the row records.
