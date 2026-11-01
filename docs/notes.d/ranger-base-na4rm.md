# The one-package censuses are doored, and a door can carry `-tags` (ranger-base-na4rm)

2026-10-31. ranger-base-m94g2 DECIDED that a one-package census — a QA pin in
internal/posse that reaches an enumeration rooted at a relative literal naming
its own package directory — is in the tree-wide class. This is the
implementation. The decision, both sides of it and the three doors' cost are in
docs/notes.d/ranger-base-m94g2.md and are not re-derived here; what is here is
what the implementation MEASURED, which differs from the record in three places.

## FIFTEEN, not fourteen

MEASURED 2026-10-31 at HEAD 72381699, by the seed itself rather than by a
census program: the posse half of the derivation, with `len(f.relEnum) > 0`
unioned into its seeds, derives **fifteen** members. Fourteen are the
record's. The fifteenth is **`TestNoSecondReadOfARuntimeHomeVariable`**
(internal/posse/runtimehome_qa_test.go, arm 1), which landed as e59a3dc1 under
ranger-base-uz4cs — hours after the decision bead's population was read, and
the same day.

It is the useful one, because its own head comment had just written the OLD
answer down: *"Reaching cmd/ would mean climbing to the repo root, which puts a
pin in the tree-wide class and wants a Makefile door; this one reads its own
directory and stays out of it."* That is the fourth carrier of a scoping
ranger-base-m94g2 had already reversed — after herdrhintsgone_qa_test.go,
reexecenv_qa_test.go and the two closed beads ranger-base-49iga went looking
for — and it was written by a bead that had no reason to read any of them. It
is the whole case for deriving the membership instead of listing it: a list of
fourteen written on 2026-10-31 would have been wrong on 2026-10-31.

All four carriers state the answer now and point at the door.

## The fixture question: asked, and the answer is zero

The decision told this bead to look for, and not waive, a census that globs
package-local FIXTURES rather than sources — a member by the key and perhaps
not one by intent. MEASURED 2026-10-31: **none of the fifteen chdirs**, so
every `.`, `*.go` and `*_test.go` among them resolves against the cwd `go test`
installs, which is the package directory, which is the package's own sources.
Nothing to disposition. The check is worth recording rather than just passing,
because the rule's precision rests on it: `twdRelTreePath` refuses a path
carrying a variable, so a `t.TempDir()` root cannot be read as a tree path —
but a test that chdirs to a scratch directory and then reads `"."` would be a
false member and no part of the rule would say so.

## And it composes with ranger-base-nt0pi, as predicted

MEASURED over all three of that bead's climbs
(`TestQAPrescribedGitDiffChecksStateTheirFormat`,
`TestQAPrescribedGitDiffChecksRunUnderADiffDriver`,
`TestShelfPIDsLintCleanAsASet`): the enumeration seed reaches none of them —
`relEnum` is empty for each. `twdRelTreePath` refuses a `../` prefix on
purpose and those three derive from `f.ascent`. Neither fix subsumes the other
and the second to land reads the first's answer, which is what the decision
record said would happen.

## THE TAG, which is the part that is new mechanism

Seven of the fifteen are compiled out of arm 1. A door's `-run` ran with no
tag, and `go test -run` over a name that is not in the build **exits 0** — so
one untagged door would have stood green in front of seven pins it could not
select. That is rot class 4 of `armtags_qa_test.go`, doored into existence.

Three doors, one subject, split by build tag and by nothing else:

| door | pins | arm |
| --- | --- | --- |
| `make pkgcensus-check` | 8 | 1 and shared, no `-tags` |
| `make pkgcensus-arm2-check` | 3 | `-tags posse_arm2` |
| `make pkgcensus-arm3-check` | 4 | `-tags posse_arm3` |

(8/3/4 and not 6/5/6 as a per-arm tally of the fifteen would read: the two
files with no build line at all compile into every arm, so the untagged door
already reaches them and the tagged doors need name only what carries a tag.)

**The arm is one field in one place.** `twdDoor` (treewidedoor_qa_test.go) gains
`arm int`, and the two things that have to agree about it both read that field:

- arm 1 of the register holds each recipe's `-tags` to it, both ways — a door
  the table calls arm 1 must pass no tag, and a door it calls arm 2 must pass
  `-tags posse_arm2`;
- arm 4 of `armtags_qa_test.go` asks each door pin for the arm **its own
  door** names, instead of asking every door pin for arm 1.

Neither half is sufficient alone. Arm 4 alone would pass a tagged door that
quietly lost its tag (the pins are still where the table says, and the question
it asked was about the pins). Arm 1 alone would pass a door whose tag is right
and whose pin has moved. The pin was renamed with its subject:
`TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm` →
`...InItsDoorsArm`.

**`armDoorVars` is derived now, not hand-kept.** It was a typed list of door
variable names and it went one door short for as long as `QA_EXECWRITE_PINS`
existed — found by ranger-base-6cznr renaming that door's pin, which is the
exact drift arm 4 exists to catch, on the one door it could not see. It reads
`twdDoors` for every posse door that is not a tool door, so a door added to the
register arrives there with its arm attached. That is a smaller mechanism than
the (variable, arm) pair list the decision costed out, and it closes the drift
class rather than widening it.

**NOT a `twdDoorHolders` row**, for the reason the decision gave: that map is
the one way into a door variable that does not go through the derivation, and
fifteen rows there would be the register conceding the class it exists to
derive.

## THE CLASS CROSSED A HUNDRED, which broke the count sentence's rule

One hundred and one pins and seventeen doors. `twdSpelled` answered with
DIGITS above ninety-nine, and `twdCountClaim` matched `([a-z]+(?:-[a-z]+)?)` —
so the one live count sentence would have stopped matching altogether and arm
4 would have failed with *"the head comment carries 0 claims"*: a red naming
the rule and not the class, on a bead that had nothing to do with either.

Both are widened, and the widening is bounded by the same vocabulary:

- `twdSpelled` spells to 999 in house prose — `one hundred and one`;
- the numeral in `twdCountClaim` is a PHRASE built from `twdSpelled`'s own
  words, joined by a hyphen or by ` and `, longest alternative first. Not
  `[a-z]+( [a-z]+)*`, which could read a run of ordinary prose as a count:
  only number words can appear, so the two cannot disagree about what a
  numeral looks like. The `pins and … doors` skeleton is unchanged, and it is
  what keeps the frozen history out — "those twelve pins and the rest was the
  box" has no `doors` after it.
- `twdProseCount`'s alternation runs to 199 for the same reason, so a count
  past a hundred written into AGENTS.md or ADR 0067 is still caught by the
  rule that exists to keep a second copy of the number off those pages.

## A FLOOR FOR THE THIRD SEED

The posse half has three rules now and they fail apart, so `twdPosseEnumOnlyTests`
reports the members that are in the class by enumeration ALONE — naming no
root, reaching no rooted walk — and arm 2 reds under ten of them. Without it a
seed that stopped matching would take a seventh of the class out while the two
root rules held the union floor green: exactly the shape ranger-base-sx2dq and
ranger-base-8wxbn each cost a bead to find.

## COST, MEASURED

2026-10-31, worktree `gwart-posse-ranger-base-na4rm`, darwin 25.6.0, go
1.26.10, `make` itself included in each reading.

| door | cold | warm | the pins alone |
| --- | --- | --- | --- |
| `pkgcensus-check` | 14.6s | 2.9s | 1.270s |
| `pkgcensus-arm2-check` | 11.0s | 3.3s | 0.537s |
| `pkgcensus-arm3-check` | 13.4s | 2.8s | 0.909s |

39.1s cold and 9.1s warm over the three, at a one-minute load average of 35.5
rising to 43.0. Almost all of it is linking each arm's test binary — the
fifteen pins report 2.7s between them — which agrees with ranger-base-m94g2's
~40s/~5.7s over fourteen of them measured without `make`. A seat that runs
`make test` pays the linking once either way; a seat that ran only arm 1 pays
about 26s more for the two tagged doors, whose binaries are cold.

`make tree-check` over the whole new class: **60-62s warm**, three runs, at a
one-minute load average of 15.28, 9.49 and 6.90 before them — the quiet end of
every range the register's head comment carries, read at the largest class the
command has ever run, with the three new doors in it. Which is the "ordered by
nothing in it" finding arriving one more time: the class grew by fifteen pins
and three `go test` starts and the command came in FASTER than the 81-89s
ranger-base-3e68k read at eighty-three pins on a box whose load was moving.
