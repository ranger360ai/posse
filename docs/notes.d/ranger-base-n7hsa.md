# ranger-base-n7hsa — the orientation page's own pin had no door

`AGENTS.md` is the file dispatch hands every seat at session start. The pin that
says it carries no reader-directed push mandate lives in a 589.965s package,
behind no Makefile door, in the suite arm that aborts the other two. So the one
page every seat reads could start ordering the one thing every PID denies, and
the whole fast-check surface would call the tree clean.

2026-11-14.

## The escape, measured

MEASURED 2026-11-14, this box (linux), at `main` 37a94cd9, one-minute load
average 8.10 falling to 6.57, neither suite slot held by this seat. Two
reader-directed push orders appended to `AGENTS.md` as a top-level bullet:

    - **When the work is done, push the branch yourself.** Work is not
      complete until the push succeeds; if it fails, retry until it does.

| command | before the door |
| --- | --- |
| `make fmt-check` | exit 0 |
| `make tree-check` | **exit 0, 60s, all seventeen doors** |
| `go test ./internal/treepins -count=1 -run PushMandate` | exit 1, 0.007s, both lines named |

The red was reachable only by an unfiltered run of `./internal/treepins`, which
is arm 1 — so `make test` aborted there and arms 2 and 3 never ran at all.

`AGENTS.md` restored and the md5 verified either side of the plant
(`4fb0bde2d963ab3124b63adf13546bf8`).

## Why the register was right to exclude it, and doored it anyway

`internal/treepins`' half of the tree-wide-pin class keys membership on a
directory **enumeration** rooted at a relative tree path.
`pushmandate_qa_test.go` reads **one named file** — `os.ReadFile("AGENTS.md")`
— which `TestQATheTreepinsEnumerationRuleMatchesWhatItClaims`' own fixture
table lists as deliberately not an enumeration ("reading one named file through
a climb is not an enumeration"). The exclusion is working as designed.

It is doored anyway for the reason `ranger-base-hrf47`'s door-holder row
already wrote down: **nothing about reading a file by name makes a pin
reachable.** Any bead that edits `AGENTS.md` or `INSTALL.md` reds these, and
nothing under 589.965s said so.

## What landed

`make orientation-check`, twelfth of eighteen `tree-check` prerequisites, six
pins in 0.063-0.069s (MEASURED 2026-11-14, three warm runs at a one-minute load
average of 11.74):

| pin | reads |
| --- | --- |
| `TestRepoAgentsMdCarriesNoPushMandate` | `AGENTS.md` |
| `TestInstallSection9ReconcilesThePlantedAgentsMd` | `INSTALL.md` §9 |
| `TestInstallSection9RecipeCutsTheMandateOutOfWhatBdPlants` | §9's recipe, run |
| `TestInstallSection9RecipeStopsAtTheNextHeading` | §9's recipe, run |
| `TestInstallSection9RecipeIsSafeWithNothingToCut` | §9's recipe, run |
| `TestInstallSection9RecipeReplacesRatherThanDuplicatesOnASecondRun` | both pages |

A new door and not a fold, because no existing door's subject is theirs:
`doc-check` is the prose door and runs `./internal/posse`, which these pins do
not live in, and the five `internal/treepins` doors are the notes index, the
ADR citations, this repo's own sources, the register itself and `scripts/`.

**Not `-run PushMandate`**, which is the filter the filing bead named and the
one a seat reaches for. Only two of that file's seven tests carry that
substring (MEASURED 2026-11-14, `go test ./internal/treepins -list
PushMandate`), so it runs the `AGENTS.md` pin and the scanner control and
leaves the five §9 arms exactly where they were. The variable spells all six.

The seventh test, `TestPushMandateCheckerDiscriminates`, is deliberately
undoored: it drives the scanner over planted fixtures, so nothing derives it
and no unrelated bead can red it — the same reason every control arm named in
`treewidedoor_qa_test.go`'s head comment is undoored.

## Mutation check

MEASURED 2026-11-14, golden copies and md5 rather than `git checkout --`
because the fix under test was uncommitted. Seven killed, one survivor:

| mutant | arm | verdict |
| --- | --- | --- |
| the push mandate planted, door in place | `make tree-check` | **exit 2 at 14s** (was exit 0 at 60s) |
| `orientation-check` out of `tree-check`'s prerequisites | arm 1 | killed |
| `orientation-check` out of `.PHONY` | arm 1 | killed |
| one pin out of `QA_ORIENTATION_PINS` | arm 4 | killed (153 against the sentence's 154) |
| one pin out of `QA_ORIENTATION_PINS` | the filter arm | **survived** |
| one `twdDoorHolders` row dropped | arm 2 | killed |
| the README table row dropped | arm 4 | killed |
| the `AGENTS.md` block row dropped | arm 4 | killed |
| the recipe loses `-count=1` | arm 1 | killed |

The 14s is the point of the whole thing: the red now arrives twelve cheap doors
in, rather than 137.3s into the package run that takes the rest of `make test`
down with it.

The `153`/`154` pair in that table is the class at `37a94cd9`, the base the
mutants ran on. The sentence that landed says **156**, for the reason in the
next section.

The survivor is a misaimed probe and not a hole.
`TestQAEveryTreepinsDoorFilterNamesItsPins` compares `make`'s expansion of a
door's `-run` against the names its variable holds, so dropping a name from the
variable drops it from both sides and the two still agree. That arm's subject is
filter-versus-variable; variable-versus-class is arm 2's and arm 4's, and arm 4
killed the same mutant.

## Main moved under this, and the count sentence is why it mattered

`ranger-base-q749c` landed `fcfcdbf6` while this was in flight, adding two pins
to `$(QA_DOC_PINS)`. So the class went 148 to 150 under this work, and a
merge-back would have put a head comment reading "one hundred and fifty-four"
in front of a Makefile holding 156 — **arm 4 red on `main`, for a numeral and
not a mechanism.** That is the drift arm 4 exists to catch, and this file's own
head comment already records it happening twice before.

Rebased onto `fcfcdbf6`. One conflict, in the count sentence alone, resolved to
150 + 6 = 156; q749c's own commit had already put its two pin names in the head
comment's enumeration, and the Makefile auto-merged. No leaked rebase
pseudo-ref in the worktree git-dir (AGENTS.md, "Landing the plane").

**Which base each reading was taken at.** The escape measurement and the eight
mutants: `37a94cd9` plus the working tree. Everything after the rebase —
`make tree-check` clean at 61s over all eighteen doors, and the escape
re-planted at exit 2 in 14s — on `fcfcdbf6` plus this commit.

## A note on the 589.965s figure

This repo quotes 589.965s for an unfiltered `./internal/treepins`, and it is the
number the whole door mechanism is justified by. The run above was **152.470s**,
holding a suite slot with a sibling holding the other.

Not filed, deliberately. One reading is not the box's number, and the house
figure for `make treepins` is a median of 393.5s over 327 wall readings at or
above 60s (679.1s cold, 1501.6s at the longest) — a single fast run does not
overturn a 327-reading median, and whether 589.965s was taken cold or on
another platform was not established here. Written down so the next seat who
reads 152s against 589.965s knows it has been seen once and not chased.

## Two findings, both filed

**`ranger-base-y0qvh` (live).** `pushMentionFaults` faults an `AGENTS.md` line
whose only `push` **opens** an identifier — `pushmandate_qa_test.go`,
`pushMentionFaults` — so the page that has to document this door is the one
page the pin forbids from naming it. Found by the door reddening on the first
draft of the `AGENTS.md` bullet that documents the door. This is
`ranger-base-rulbl`'s class one step out: that bead narrowed the scanner to "no
letter before the match" because the substring form fired on
`TestTestCorpusHidesNoCrewNameBehindAnEscape`, which fixed `push` **buried** in
a longer word and left `push` **opening** one. MEASURED over nine lines run
through the scanner verbatim: three of four code-naming lines fault, the fourth
(`...NoPushMandate`) is let by only because a letter precedes the word; both
reader-directed orders still land and all four exempting spellings are still
let by. The bullet works around it by naming the four spellings instead of the
file — a worked-around hole, not a closed one.

**`ranger-base-m6mzg` (architecture lane).** The membership key reads an
enumeration and not a named orientation page, and `twdDoorHolders` went from
three rows to nine in this one landing. Seven of the nine now share hrf47's
single cause. Its own comment said "two rows is what it should stay near" — and
it was three when it said it, hrf47 having added a row and left the numeral.
That sentence is gone rather than decremented: a cause that arrives six at a
time is a key the derivation should be reading, not a list this register should
be growing. The ruling is not this bead's to make.

## For the next seat

`make orientation-check` whenever you edit `AGENTS.md` or `INSTALL.md`. The
four spellings the scanner lets by — `Never push`, `operator pushes`,
`pre-push`, `git push:*` — are the whole vocabulary either page has for
discussing the mandate, and they are checked **per line**, so a sentence that
wraps has to carry one on each line that names the verb.
