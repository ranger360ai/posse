# ranger-base-m0ajs — a line that enumerates, and a census that claims a reach

Two findings of `ranger-base-ou4b3`'s bundle, both of the same shape: a
sentence that says how many things there are, or what it can see, beside a
measurement that says otherwise. Both escaped from closes whose deliverables
VERIFIED in full — this is the arm each close's own claim did not cover.

The bundle's third finding is in **ranger-core**, not posse, and is filed as
`ranger-base-3m7sj` for `ranger-base-jttvs`'s own reason: this seat ran in a
posse worktree and only commits from there move.

## Finding 1 — the trail's silence has three readings, not two

`verifyNoCommitTrailLine` (internal/posse/verifyafter.go) renders for a close
that `git log --grep <id>` answered about and named nothing. It shipped under
`ranger-base-q28yf` offering two readings — nothing was built, or something
was built under a commit that omitted the id — and the batch that verified
that close contained the third: the work landed in ANOTHER repo of this
instance's crew, under a commit that DOES name the bead, in a checkout this
trail never reads.

MEASURED 2026-11-22 (`ranger-base-ou4b3`, this box, live store of 4,207 closed
beads), over the population the close measured — the newest 150 `ranger-base`
closes carrying a `code`/`devops` label, which is `verify_labels:`' default:

| reading | closes |
| --- | --- |
| no commit in posse's git names them | 27 of 150 (the close read 34/150 eight hours earlier; the window moved, the share did not) |
| …and a **ranger-core** commit does | **3 of those 27 — 11%** |

The three are `ranger-base-jttvs` (eda8427), `-oqrat` (29adec1) and `-2qlz2`
(3013107), and the two-reading line was rendered for all three. One of them is
`jttvs`, one of the four closes `ranger-base-ou4b3` was verifying: the first
batch to meet this line contained the reading it did not have.

What made it a misdirection rather than an omission: the line spent most of
its words on "Nothing built: the close_reason is then the whole claim", which
sends the verifier to judge prose when there is a commit, a diff and a test
run to read. And of the three settling sources it named, only one answers
here — `~/src/posse` holds 11 `branch.<b>.posseBead` records and not one names
that batch, so "the branch records below" pointed at a block
`gitBranchLandingFor` had rendered for none of them.

**What landed**: the third reading; the settling sources ordered by what they
can ANSWER (`bd comments <id>` first and named as the only one that reaches a
commit in another repo, the branch records qualified "below WHEN a branch was
cut for this bead in this repo"); and a tail telling the verifier to go read
the diff and the test run in that tree. Nothing is suppressed and no verdict
is reached — the line still refuses to say which reading holds, which is
`q28yf`'s decision and not this one's.

`TestVerifySectionReadsAnAnsweredEmptyCommitTrailAsALineNotASilence`
(verifyafter_test.go, arm 3) grew four rows: the COUNT (`has three readings`),
the third reading, the `bd comments` pointer and that instruction. Mutation-
checked 2026-11-22, each mutant applied alone over a checksummed golden copy
and graded by the pin:

| mutant | verdict |
| --- | --- |
| baseline | green |
| the third reading dropped from the line | RED |
| the `bd comments` pointer dropped | RED |
| the "verify them there" tail dropped | RED |

The third of those is why the tail is pinned at all: it was written first and
SURVIVED, because a line can name the repo and still send the reader to the
close_reason's prose.

## Finding 2 — the membership census could not see two spellings of a run

`TestQAEverySupervisedRunHoldsTheReportArithmetic`
(cmd/coldexec/reportmembership_linux_test.go) is `ranger-base-l9pk7`'s
structural half: a supervised run written outside `runDoor` is red the moment
it is written. Two spellings were green against that claim, measured by
`ranger-base-ou4b3` with a planted probe file:

- **the door path bound to a local first.** `bin := builtDoor(t)` and then
  `exec.Command(bin, …)`. ARM 8 (`TestANestedDoorRefusesRatherThanRunning`)
  already writes that binding, because a function needing the path twice
  naturally does.
- **the second run in an asserting function.** One `bool` stood for the whole
  body and `starts` kept only the FIRST site, so "assert run A, then start run
  B" was green — the original defect, one function over.

Latent at 0 sites either way, which is the point: the cost arrives with the
next run someone writes, and that is the whole reason the pin exists.

**What landed**: `rmScan` resolves a local whose ONE assignment is a call to
`builtDoor` (a name assigned anywhere else is left unresolved, because a red
for a run that is not there is worse than a miss), and it COUNTS runs and
assertions instead of answering one bool — the function has to hold one
assertion per run it starts. The head comment's register now names what is
still out of reach: ARM 8's inner door (a string inside a supervised script),
a path this file cannot resolve in source, and runs PAIRED to assertions —
the two numbers are counted, not matched.

Re-measured 2026-11-22 after the fix, each probe planted alone in
cmd/coldexec and graded by the census (go1.27.3, linux 7.2.15-1-omarchy):

| probe | before | after |
| --- | --- | --- |
| `bin := builtDoor(t)`, exec'd, no assertion | PASS | **RED** |
| the same run spelled `exec.Command(builtDoor(t), …)` (control) | RED | RED |
| run A asserted, then run B unasserted | PASS | **RED** |
| the same two runs, neither asserted (control) | RED | RED |
| one bound run, asserted (negative) | — | PASS |
| ARM 8's shape: the path never exec'd (negative) | — | PASS |
| a local reassigned, so unresolved (negative) | — | PASS |

### The census cannot witness its own reach, so a second pin does

Every red the census files is reached through `rmScan`, and over this
package's real sources `rmScan` answers the same thing with either reading
simplified away — every live site is one direct run per function. That is
exactly how two holes stayed green against a green census.
`TestQATheCensusReadsABoundDoorPathAndCountsEveryRun` is the witness: ten
literal snippets, parsed and scanned, asserting the run count, the assertion
count and whether the census reports. Mutation-checked 2026-11-22, each
mutant REVERTING one half of the reach, graded by both tests:

| mutant | the reach pin | the census |
| --- | --- | --- |
| baseline | green | green |
| the bound-local reading reverted | **RED** | green |
| only the first run kept (the old one-site read) | **RED** | green |
| assertions back to one bool | **RED** | green |

Three mutants, three reds, and the census green through all of them.
