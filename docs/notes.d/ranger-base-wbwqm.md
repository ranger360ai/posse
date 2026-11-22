# Two clauses whose stated payoff no pin read (ranger-base-wbwqm)

`ranger-base-qnfan` verified four closes by execution and found two live
things, both of one genre and the genre is this lineage's own: **a clause
whose stated payoff no pin reads.** Neither is a bug in shipped code. In both
cases the code is right and the pin beside it is narrower than the sentence
above it, so the mutant that takes the payoff away is green everywhere.

MEASURED 2026-11-22, linux 7.2.15-1-omarchy, go1.27.3, this tree at
4e344271 unless another base is named.

## 1. `vulnPrivateUndiff`'s two `atFile = false` clears

`internal/posse/vulnprivate.go`'s switch has four state-affecting clauses and
until this bead two of them were graded by nobody:

| clause | graded by |
|---|---|
| `atFile = true` at the `diff --git ` case | mutation-killed before `ranger-base-tcho6` (`ranger-base-yusee`) |
| the `+++ ` prefix guard on the second case | `ranger-base-tcho6` |
| `atFile = false` in `default:` | **this bead** |
| `atFile = false` in the `+++ ` case | **this bead** |

There is no fifth clause, so the site is closed rather than closed for one
more round.

**Both survivors FAIL OPEN**, which is what makes them pins and not comments:
a described open `vuln-private` id reads CLEAN where real code reds, so what
moves under the mutant is the control that keeps an open residual's detail off
this public tree. The reachability argument is `tcho6`'s own and not a new
one — `RunGatesVulnPrivateScan` judges arbitrary reader text, which is exactly
why that bead's `+++ ` guard was found load-bearing — so neither row needs a
stream git writes.

The two rows added to `TestRunGatesVulnPrivateScanClosesEveryUnitAtAFileBoundary`:

- **the `default:` clear** — a header, a bare id, a `+++ `-prefixed CONTENT
  line, then the description. Real code is at `atFile=false` by that content
  line, so the `+++ ` line is content (`++ …`), the unit holds and the id is
  described. Sticky `atFile` blanks the line instead, the unit closes, the
  description detaches and the hit is gone.
- **the `+++ ` arm's own clear** — two consecutive `+++ ` lines, the second of
  them content carrying the id and its description. Real code cleared
  `atFile` at the header, so the second line is judged as content. Sticky
  there blanks it too and the added text is gone altogether.
- **the control for both**, paired as every other row in that table is: the
  same added lines with NO header above them, so `atFile` is never set and
  the three readings cannot differ. That is what makes the two rows above
  attributable to the clears rather than to a judge broken outright.

Before these rows each mutant was green over internal/posse arm 1, arm 2 and
arm 3 filtered on `VulnPrivate|Undiff` and internal/treepins'
`CommitGate|BdWrite` pins — six runs, rc=0 with the mutant planted
(`ranger-base-qnfan`'s measurement, re-measured here).

## 2. `boxJoinRows`'s dropped COUNT

`internal/posse/boxloops.go`'s bound is real and `ranger-base-m4w7d` landed
it; three of that close's own five mutants were re-graded under `qnfan` and
all three red. What no pin read was the NUMBER in the truncation clause,
although `boxJoinRows`'s doc comment says the clause "says HOW MANY it
dropped". `TestQABoxCensusRendersOneBOUNDEDLineWhateverPeersDeclare` asserted
the clause's PRESENCE (`strings.Contains(s, "not shown")`) and the TOTAL ahead
of the list (`strconv.Itoa(peers)+" "+arm.clause`), and neither reads the
integer between them.

`len(rows)-n` → the literal `1`, measured through that pin's own fixture seam
(64 peers each declaring a distinct 256-byte home the fence accepts):

| | line | clause |
|---|---|---|
| real | 748 bytes | `and 63 not shown` |
| mutant | 747 bytes | `and 1 not shown` |

`go test -run '^Test(QA)?(Live)?Box' ./internal/posse` is rc=0 under it. The
real reading at that fixture shows ONE row of 64 — a 266-byte row against a
512-byte budget — so the dropped count is not a tail case, it is what the
clause says on every truncated reading. A reader is told one row was dropped
where sixty-three were, with every pin green.

**The assertion is derived from the LINE, not from the function.** Count the
rows that actually reached the rendered sentence — every row of either list
names its pid — and the clause must account for the rest. An assertion that
recomputed `len(rows)-n` would be the same arithmetic twice, and would hold
under a join that had lost the subtraction. A fatal guards the fixture itself:
if the line dropped nothing (or showed nothing) the arm is not about a
truncated line and says so rather than passing.

### and the budget's own boundary

`qnfan` named a second, weaker survivor at the same site so it would not be
re-derived: `>` → `>=` at the break also survives the whole box family, so the
512-byte boundary was unpinned where the sibling value cap's boundary IS
pinned (256 accepted / 257 refused, re-measured under `ranger-base-m4w7d`).
It is one row either way, and the row it costs is a row a reader is told
about instead of shown.

Pinned at `boxJoinRows` directly rather than through the fixture seam, because
the boundary is a byte sum the seam cannot land on — a row there is a quoted
home plus a pid, so no number of peers sums to exactly the budget.
`255 + len("; ") + 255` is exactly `boxRowsMaxBytes`, so two 255-byte rows are
the boundary: the join is 512 bytes with no clause, and its control one byte
over (255 + 256) must truncate. A fatal checks the sum before the assertion,
so a fixture that drifts off the boundary says so instead of passing.

## The grading, all four at once

`arm 0` unplanted green, every mutant red on exactly the row written for it
(MEASURED 2026-11-22, this tree plus this bead's diff):

| mutant | run | result |
|---|---|---|
| the `default:` clear removed | `-run TestRunGatesVulnPrivateScanClosesEveryUnitAtAFileBoundary` | rc=1, the `default:`-arm row alone |
| the `+++ ` arm's clear removed | the same | rc=1, the `+++ `-arm row alone |
| `len(rows)-n` → `1` | `-run TestQABoxCensusRendersOneBOUNDEDLineWhateverPeersDeclare` | rc=1, both arms (`says 1 … 63 were dropped`) |
| `>` → `>=` | the same | rc=1, the boundary row |

## What the suite said

**The reading of record is a plain `make test`, exit 0, no skips of any kind**
— MEASURED 2026-11-22, three times over, each on this branch merged onto a
main that had moved again in between, zero FAIL lines in all three:

| main merged in (this branch's merge) | cmd/posse | internal/posse arms 1/2/3 | internal/treepins |
|---|---|---|---|
| `d1c7f7f8` (`59469337`) | 121.5s | 251.6 / 69.6 / 154.2s | 149.6s |
| `175e7c3b` (`7433c417`) | 131.1s | 283.1 / 70.5 / 109.4s | 166.8s |
| `cd1c2333` (`182173d3`) | 116.4s | 268.2 / 60.8 / 108.9s | 160.7s |

`go run ./cmd/checkorphans` clean after each. Three readings because main
landed a sibling bead roughly every ten minutes while this one was being
verified, and a branch is only mergeable if it contains the main it was read
against; the four mutants below were re-graded on the first and the last.

That reading was only available at the end. The rest of this section is how
the same work was verified before it, and it is kept because the lesson in it
outlives this bead.

## The base red it had to be read around first

For most of this bead's life `make test` died at its 12th `tree-check`
prerequisite on a red that was not this bead's and was already owned:
`ranger-base-89fsg`, "main red at 8b6d0c2776d3",
`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` — the head comment said
163 pins where the Makefile had 164. Measured here with this bead's two files
restored to HEAD, so it was the base's and not the diff's. Per AGENTS.md
("A red `make test` on a fresh `main`") no second bead was filed, and
`8b83a18a` landed that one-word fix while this bead was in its last suite
run.

**And the obvious way around it does not work**, which is worth writing down
because the next seat on this base will reach for it: `make test
QA_TREEPINS_EXCLUDED='<the eleven>|TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles'`
reds a SECOND pin instead —
`TestQATheTreepinsComplementDoorIsThePackageMinusItsPricedTable`, which reads
the regex make assembles against `twdTreepinsExcluded`'s priced table and
refuses a door that is "silent over tests nobody priced". That is ADR 0098 D4
working exactly as designed: the complement door exists so that no pin can be
hand-exempted, and a `make` variable is a hand exemption. MEASURED
2026-11-22, `make test` exit 2 at 115.0s.

The route that does work is `ranger-base-mlw6q`'s: leave the Makefile alone —
so the complement door compares two sets that agree and is green — and put
the skip on the `go test` line instead. This bead's intermediate readings were
taken that way, through `scripts/test-times.sh` so each run still takes a
suite slot:

- the other thirteen `tree-check` doors, each by name: all green
- `scripts/test-times.sh go test ./internal/treepins -count=1 -skip
  '^(<the eleven priced>|TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles)$'`
- `make test`'s own non-`tree-check` prerequisites (`fmt-check`, the nine
  `verify-*` targets)
- `TestQAEverySuiteArmTypeChecks`, then arm 1 (`./...`), arm 2, arm 3, then
  `scripts/audit-silent-reverts.sh --quiet`

Arm 1 is `./...`, so it reaches internal/treepins too and needs the same skip
on its own line — the first pass around the red forgot that and arm 1 was the
one red in an otherwise green set, which is how the omission announces itself.
