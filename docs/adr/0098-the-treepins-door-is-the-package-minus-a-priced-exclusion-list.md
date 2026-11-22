# ADR 0098 — The treepins door is the package minus a priced exclusion list

*Status: accepted 2026-11-14 (ranger-base-m6mzg) · D2 amended 2026-11-14
(ranger-base-a7sqc): the run shape the floor is measured in · D5 added
2026-11-22 (ranger-base-o065a): the internal/posse half, for the off-/tmp
venue · owner: architect · changes the internal/treepins half of the
tree-wide-pin class in `internal/treepins/treewidedoor_qa_test.go` · the
internal/posse half keeps its derived key, and D5 rules where its one
run-shaped door lives*

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

*(amended 2026-11-14, ranger-base-a7sqc.)* **Measured over what.** A row's
seconds, and the floor they are held against, are the test's `-json`
`Elapsed` in an unfiltered, warm-build-cache run of the whole package
(`make treepins`, which is `scripts/test-times.sh`). They are not the test run
alone under `-run`, and not a cold-cache run. That is two rulings on two axes:

- *Warm*, because a cold build is paid once per tree and not once per door
  run. The race-arm rows read 25-28s cold and about 2.9s warm.
- *In the package*, because the door and `make test` both run the test there,
  beside its siblings, and nothing runs it alone. A price no route pays
  cannot say what a row saves. An excluded test is absent from the door's own
  run, so the whole package is the nearest shape anyone can re-read, and it
  is the shape the table was first taken in.

A reading in another shape is not evidence against a row. To challenge a row,
re-read it in this shape. Known limit: for a `t.Parallel` test (the race-arm
census is one), `Elapsed` overstates the wall the door saves. That saving is
unmeasured. It is one more reason the floor is necessary and never
sufficient.

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

**D5 (added 2026-11-22, ranger-base-o065a). The off-/tmp venue is typed, not
queued. Its venue of record is ci.yml's macos legs.** The Consequences below
said the internal/posse half "needs its own ruling" if the shape escaped
there. It did, in the bwrap fixture-home class (ranger-base-jhmm7): a render
that must succeed is green only where the fixture `$HOME` from a bare
`t.TempDir()` lands under `/tmp`, because that is the only place
`bwrapWallRoot` stands the wall-root refusal down. No scan finds the members
(ranger-base-shf97 got 2 of 4 at 15 false reads). A run does:
`make tmpdir-check` (ranger-base-m33n5), at 527s of wall over the three arms
(MEASURED there, 2026-11-22, this box, load 10-16), with all of the yield in
arm 1.

The ruling:

1. **`make tmpdir-check` is a prerequisite of nothing**, neither `tree-check`
   nor `make test`. It stays a door a seat types, like `make coldexec-check`
   (ADR 0085 D6).
2. **A seat MUST type it in two cases, and both are named by what the change
   touches, not by a filter.** (a) Unfiltered, when the change alters WHERE a
   wall rule stands down or what it refuses (the predicates over
   `bwrapTmpMount` and the wall root in `internal/posse/bwrap.go`, and
   anything new of that shape). That is how most of the class's members
   arrived: ranger-base-5f5kb's refusal made a batch of existing fixtures
   venue-dependent in one landing, and no test file changed. (b) Filtered
   (`RUN=`), when the change writes or edits a test whose fixture `$HOME`
   drives a render, unless that home comes from `wallUngrantedDir` or
   `wallStandDownDir` (`internal/posse/wall_helpers_test.go`), which are
   placed on purpose.
3. **Detection after landing is the macos legs, and that is deliberate.**
   darwin's `t.TempDir()` is under `/var/folders/…`, so ci.yml's macos arm
   jobs ARE this venue on every push to main. A red there already files one
   bead through ci-watch (`internal/posse/ciwatch.go`), with the landings in
   the window. Option (c), a new ubuntu leg, would buy a linux-readable log of
   a red that is already caught and already filed. It costs runner time, and
   no ASK is filed for it.
4. **Re-ruling trigger, the exit hatch.** Once arm 1 is green in the venue
   (that is, once ranger-base-shf97's and ranger-base-mlw6q's fixes are both
   on main), count the escapes: any ci-watch bead whose fix is a fixture
   moved off a bare temp home. **Two in 30 days re-opens this ruling toward
   arm 1 alone as a `make test` prerequisite** (293s, MEASURED in
   ranger-base-m33n5). That is the price of the whole live yield measured so
   far, and it is the cheapest pre-land option that still finds the class.

**What this costs, said plainly.** A member of this class that does not fall
under 2(a) or 2(b) lands, reds macos arm 1, and comes back as a bead. That
was the cost of every member so far. The difference now is that the bead is
filed by a machine, and the seat that caused it was told which door to type.

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
  shape escapes there, it needs its own ruling. *(It did, and D5 is that
  ruling.)*
- **D5's cost is borne after landing, not before.** A seat's `make test` is
  unchanged. An escape costs one macos-arm red on main and one ci-watch bead.
  The alternative was 527s on every `make test` (+54% on its ~970s across
  three arms). Box-wide that is paid through a two-slot queue, so every
  waiting seat pays it again in queue time. D5.4 says when that trade flips.
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

### Rejected for D5 (ranger-base-o065a)

- **(a) A prerequisite of `tree-check`.** +527s on a line that reads 83-92s,
  6.7x. That is a second full pass of internal/posse, and D2's own floor
  exists to keep the door line cheap. A door that costs six times the line
  it joins is a suite, not a door.
- **(b) A prerequisite of `make test`.** The same 527s, in the one place a
  seat expects to wait. Rejected on yield per second. Arms 2 and 3 are green
  in the venue. Arm 1's four live members sit in two files written on
  2026-11-11, and they became venue-dependent when 5f5kb's refusal landed on
  2026-11-15. That is one production change, which D5.2(a) names, and not a
  steady rate of new tests. Kept as D5.4's fallback, arm 1 only.
- **Swap the venue rather than add one: run `make test`'s arms with TMPDIR
  off `/tmp`.** This is the clever one, because it costs no second pass.
  Rejected for three reasons. First, it moves the
  blind spot rather than closing it. ranger-base-6ca29's invariant is that the
  verdict does not move with TMPDIR, and one venue cannot check an invariant
  over two. A refusal-expecting fixture on a bare temp home would then be
  green on the seat and red on ubuntu, which mirrors today's escape. 81 calls
  across 38 files already place refusal-side homes with `wallUngrantedDir`,
  so that side has real authoring traffic. Second, a caged seat cannot create
  a `/var/tmp` root at all (m33n5's measured refusal), so `make test` would
  refuse or abstain on every caged seat. Third, `/tmp` here is tmpfs and
  `/var/tmp` is btrfs, so the swap is not free either. See the A/B in the
  bead comment.
- **(c) A new ubuntu ci.yml leg.** The macos legs already run the venue, and
  ci-watch already files the red (D5.3). A ubuntu leg buys the same
  discovery on the host where the fix is written, for runner time the
  operator would have to approve. A log being easier to read does not justify
  an ASK.
- **A conditional prerequisite keyed on the diff** (run the venue when
  `bwrap.go` changed). This is D5.2(a) enforced by a machine. Rejected for
  now because the key is a hand-written file list over a predicate that can
  move files, which is ranger-base-c3zb8's problem one layer up. If D5.4
  fires, price this against arm 1 alone before choosing.
