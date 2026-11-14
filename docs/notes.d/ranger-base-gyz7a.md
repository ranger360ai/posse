# ranger-base-gyz7a — the hand exemption register retires for internal/treepins

ADR 0098 D4, implemented. `twdDoorHolders` is gone: in
`internal/treepins/treewidedoor_qa_test.go` membership is the complement now,
so a door variable may name any test in that package with no row anywhere, and
the two-way check reads the complement instead of the derivation.

The predecessor is [ranger-base-3cpcg](ranger-base-3cpcg.md), which built the
door (`make treepins-check` = the package minus `$(QA_TREEPINS_EXCLUDED)`) and
left the nine rows standing as its own first residual.

## What changed

| file | change |
| --- | --- |
| `internal/treepins/treewidedoor_qa_test.go` | `twdDoorHolders` and its comment deleted; arm 2's per-package loop split; `twdTreepinsExcluded` down to eleven rows; the floor's necessary-vs-sufficient reading written down; six prose paragraphs re-pointed at ADR 0098 |
| `Makefile` | `QA_TREEPINS_EXCLUDED` loses one name; four comments that named the retired register |
| `AGENTS.md` | the complement bullet drops the row COUNT and names D4 |
| `internal/treepins/README.md` | the nine exemptions are named as retired, with the ADR link |
| `internal/treepins/coldexecarms_qa_test.go` | its "parking spot" citation now says that register was retired for being one |

### The nine rows

All nine were internal/treepins tests, so the map went rather than shrank:
`TestQAEveryTreepinsDoorFilterNamesItsPins` and
`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` (the register's own
arms, from the map's first version), `TestQABoxCheckCensusCoversEveryVerifyTarget`
(ranger-base-hrf47) and ranger-base-n7hsa's six push-mandate pins. Every one
of them reads one or two tracked files BY NAME and enumerates nothing, which
is the enumeration rule's own negative case — seven of the nine for that one
cause, which is what ADR 0098 reads as an argument about the key.

### Arm 2 is two loops now, not one loop over two packages

`TestQAEveryTreeWidePinHasADoor` ran one body over `{internal/posse, names}`
and `{internal/treepins, tpNames}`. Neither direction of that shape means
anything in the second package any more: every test there is a member, so "no
door names it" is false by construction, and a door variable may name any test
there, so "named but not derived" is permitted rather than a bug. The posse
half is unchanged (ADR 0098 Consequences). The treepins half is D4's sentence:

1. every name in `$(QA_TREEPINS_EXCLUDED)` has a row in `twdTreepinsExcluded`
   — the door does not run it, so something has to say what it cost;
2. every row names a test the package declares;
3. a treepins door variable may name any declared test, and may not name
   something the package does not declare — a `-run` filter matching nothing
   prints `ok`, which is the one failure the complement does not make
   impossible.

**(1) is read from the MAKEFILE and not from the Go table**, which is what
keeps it a coverage check rather than a tautology: the names the door skips are
what `$(QA_TREEPINS_EXCLUDED)` holds, and the question is whether the tests it
therefore does not run are the tests this file has priced. Arm 5 holds the same
pair from the other end — price, box, date, route, D3 and the recipe's shape —
so a name added to one side alone reds twice, for two different reasons. That
overlap is deliberate and is the only one.

The derivation stays, all of it, and so do arm 2's floors over it: under D3 a
test the old key DERIVES may not be priced out without a door of its own, so a
derivation that quietly stops matching stops guarding the table.

### The twelfth priced row came out

`TestNoTrackedFileCarriesADigitRealisticDetectorShape` reads every tracked
file through `git ls-files`. That is the widest reading of the tree in the
package, and ADR 0098's Context names it as the second direction the old key
missed ("an exec is not a WalkDir"). 3cpcg priced it out at 2.95s, legally: it
clears the 2.0s floor.

**The floor is a necessary condition on a row and never a sufficient one** —
said in `twdExcludedFloor`'s comment now, because D2's "the price is the
reason" can be read as denying it. Nothing obliges a row above the floor, and
what a row costs is a door. MEASURED 2026-11-14, this box (D1 host, Linux
7.2.15-1-omarchy, go1.27.3, 16 cores), three warm runs at a one-minute load
average of 10.1-10.8: **3.08s, 3.16s, 3.09s** for that test alone. Four per
cent of a seventy-second door, for the pin the ADR was filed over.

The table is eleven rows and 72.4s of the package's 167.3s of summed test time
now — 3cpcg's run 2 reading less that row's 2.95s, not a re-measurement.

## Verification

### The plant (the bead's DONE WHEN, and 3cpcg's own shape one bead on)

`internal/treepins/twd_gyz7a_plant_test.go`: one FAILING test that reads
`AGENTS.md` by name and enumerates nothing — the exact shape that needed a
`twdDoorHolders` row before this bead — with no edit to the Makefile or the
register. MEASURED 2026-11-14, load ~10, both suite slots free:

| command | verdict |
| --- | --- |
| `make treepins-check` | **RED in 97.3s, naming `TestPlantProbeReadsOneNamedFileAndNobodyDoorsIt`** |
| `make register-check` | green — nothing asks for a row |
| `make notes-check` | green |
| `make orientation-check` | green |
| `make adr-check` | green |
| `make scripts-check` | green |

The plant was removed afterwards; this fragment is the only record of it.

A green `register-check` over a red tree is the whole of D4: before this bead
that plant needed a hand-written row before any door would accept it, and the
register would have demanded one.

### The mutations

Four, each on an isolated copy of the worktree's tracked files so no sibling
seat's suite was reddened by the planting (`git ls-files -z | tar` into a fresh
`mktemp -d`). Arms 2 and 5 only; the battery script is the scratchpad's, not
the tree's.

| # | mutation | verdict |
| --- | --- | --- |
| M2 | a name added to `$(QA_TREEPINS_EXCLUDED)` with no priced row | arm 2 RED (the new half, naming the test) **and** arm 5 RED (set equality + the `-list` of make's own skip) |
| M3 | a priced row naming a test the package does not declare | arm 2 RED and arm 5 RED, each with its own sentence |
| M4 | `$(QA_SCRIPTS_PINS)` naming a test that is not in the package | arm 2 RED — the one failure the complement leaves |
| M6 | `treepins-check` taken off `tree-check`'s prerequisite line | arm 5 RED; arm 2 silent, which is right — the wiring is arm 5's |

M1 is the plant above. **M5 needs no planting: it is the live tree.**
`$(QA_ORIENTATION_PINS)` names six tests that read `AGENTS.md` and `INSTALL.md`
by name and that nothing derives, and `register-check` is green over them with
no row anywhere. Before this bead each of those six was a hand-written
exemption.

### The doors

`make fmt-check` green. `make register-check` green, 34.0s (35.3s wall) at a
load of ~10.

`make tree-check` green, three warm runs: **110s, 117s then 102s** in that
order (MEASURED 2026-11-14, this box, at a one-minute load average of 18.84,
7.21 and 20.04, with ONE suite slot held by a sibling seat through the first
two and BOTH held by the end of the third). `make treepins-check` reported
74.0s inside the first. The entry it follows in the register's reading chain
is 83.1-91.5s with neither slot held, so the 15s band is the slots and not
this bead's three seconds — and the class is identical either side of it, 157
pins and 14 doors, which makes this the one entry in that chain over an
unchanged class. Those three readings are in the head comment too, which is
the one place a count of this class may be written.

The full `make test` is in the bead's closing comment.

## Residuals

- The exclusion table is still the only hand-kept state in this half, and its
  exit hatch is still the floor (ADR 0098's Consequences). What this bead adds
  is that the floor does not oblige a row — which is a reading, not a
  mechanism. A row above the floor that nobody should have written is still
  caught by nothing but a reader.
- The internal/posse half keeps its derived key and its own route into a door
  variable is `twdIndirect`, which is a different register with a different
  reason (an enumeration no rule over Go source can derive). ADR 0098's
  Consequences leave it alone: its package is three arms totalling ~950s and no
  complement door of that size fits. If the named-file shape escapes there, it
  needs its own ruling.
- `make treepins-check` costs about 3.1s more than it did at 3cpcg's close.
  The door's own three-run band at that close was 67.7-85.3s, so the step is
  inside the load's own spread and nothing here re-prices the door.
