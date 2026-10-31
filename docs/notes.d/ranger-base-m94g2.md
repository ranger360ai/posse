# A one-package census IS in the tree-wide class, and what it costs to admit one (ranger-base-m94g2)

2026-10-31. THE DECISION, which has been recorded twice and lost twice.
ranger-base-8wxbn stated it under "ALSO FOUND" and scoped it out of its own
diff — "it is stated here so it is not re-discovered" — in a bead that closed.
ranger-base-49iga (FINDING 3) found it had no live carrier and named
`ranger-base-pyl22`; pyl22 closed the same day. ranger-base-kkneb measured the
population and filed this bead as the carrier. It is decided here.

THE QUESTION. Does a ONE-PACKAGE CENSUS — a QA pin in internal/posse that
reaches an enumeration rooted at a relative literal naming its OWN package
directory (`.`, `*.go`, `*_test.go`) — belong in the tree-wide class
`internal/treepins/treewidedoor_qa_test.go` derives and the Makefile doors?

## DECIDED: IN

The class's NAME is "a QA test whose subject is the TREE, living inside a
package nobody runs whole", and on that wording a package-directory census is
out: its subject is one directory. But the name is not the class's operative
key, and the register says so itself where it had to choose one. From
`treewidedoor_qa_test.go`, "The internal/treepins half of the class":

> What actually separates a pin that an unrelated bead can red from one it
> cannot is whether the reading ENUMERATES — whether the set of files read is
> spelled in the test, or discovered. A test that reads "Makefile" reds when
> somebody edits the Makefile, which is the bead that edited it. A test that
> globs "docs/notes.d/*.md" reds when ANY bead writes its fragment there.

A package-directory census is the second shape, not the first. Its file set is
discovered, so a bead that adds a file to internal/posse — which did not write
the census and has never read it — reds it. No seat's `-run` filter will ever
name it, because a census is nobody's subject. Its only route is the ~950s arm
run. That is the class, in all three parts.

The argument against is written down in `herdrhintsgone_qa_test.go`'s own head
comment and it is a deliberate scoping, not an oversight: "It reads THIS
directory and cockpit.go's own package reads its own, rather than walking from
the repo root: a tree-wide pin is nobody's `-run` subject and needs a Makefile
door." What that answers is "is this pin's subject the whole tree" — no. What
it does not answer is "can a bead that did not write this pin red it" — yes.
The second question is the one the doors exist for.

The narrower blast radius is real and it is not a reason to decide the other
way. A tree-wide pin can be reddened by a seat that only edited AGENTS.md and
would never run internal/posse at all — the ranger-base-rulbl incident shape,
four commits to main not gofmt-clean and three concurrent P1 beads over one
whitespace character. A one-package census is only ever reddened by a seat that
edited internal/posse, and that seat is already building and running that
package's arm-1 binary under a `-run` filter. It just will not name the census.
So the door buys less here than it does for a gofmt pin — and it still buys the
whole of the difference between "red at the filter" and "red at 950s, for
somebody else, later".

## THE COST, which is what nobody had measured

MEASURED 2026-10-31, worktree `dinesh-posse-ranger-base-m94g2`, HEAD 7db75f44,
darwin 25.6.0, go 1.26.10, one-minute load average 34.6-38.0 throughout (a busy
box, three other seats). `go test ./internal/posse -timeout 15m -count=1 -run`
over the fourteen, split by the arm each lives in — which is what three doors
would run:

| door            | pins | cold (links the arm's test binary) | warm | tests themselves |
|-----------------|------|------------------------------------|------|------------------|
| arm 1, no tag   | 7    | 15.1s                              | 2.1s | 0.66-0.95s       |
| `posse_arm2`    | 3    | 12.7s                              | 1.5s | 0.48-0.53s       |
| `posse_arm3`    | 4    | 12.7s                              | 2.1s | 0.69-0.91s       |

So all fourteen cost **~5.7s warm and ~40s cold**, and essentially all of that
is linking a test binary, not running pins: the fourteen report under 2.4s
between them. For a seat that ran only arm 1 — the usual case — the arm-2 and
arm-3 binaries are cold, so the marginal cost of admitting the seven tagged
pins is about 26s on a `make tree-check` that already measures 57.0-82.3s warm
and 92.6s cold (`treewidedoor_qa_test.go`'s own readings). Two more readings
of that command came out of verifying this bead, and they are worth keeping
because they say the same thing the register's widest readings do, harder:
**103.4s at a one-minute load average of 5.91** with both suite slots free, and
**69.2s at 116.31** twenty minutes later. The slow run is the one on the idle
box, by 34 seconds, at a load average twenty times lower — which is the "ordered
by nothing in it" finding (ranger-base-0ckgm, ranger-base-c768n,
ranger-base-9mjxb) arriving from both ends at once. The 103.4s is also above
every reading in that comment's enumeration, and was taken at the quiet end of
it. Not pinned — elapsed seconds belong to the box — and recorded here rather
than in that comment because this bead did not change the class, and because
ranger-base-3e68k's merge-back re-measured and rewrote that very sentence the
same day.

That number settles the one objection nobody could answer before: cost is not
why these are out. It also says which way the implementation should go — the
three doors are three `go test` invocations and the expensive part is shared
with `make test` itself, so a seat that runs the suite pays nothing twice.

## THE TWO MECHANISM CHANGES "IN" REQUIRES

Both are precise, both are smaller than ranger-base-kkneb's gates read, and
NEITHER is in this bead's diff: they land in `treewidedoor_qa_test.go`,
`internal/treepins/armtags_qa_test.go` and the `Makefile`, all three in flight
under ranger-base-3e68k (ranger-base-pyl22's blocked merge-back, which rewrites
316 lines of the register) when this bead was dispatched, so implementing here
would have conflicted with it by construction. Filed as ranger-base-na4rm,
which carries both changes, the cost measured above, and the two things it must
measure rather than assume.

That blocker cleared WHILE this bead was being written: gwart's replay landed
on main as 0011c725 and ranger-base-3e68k closed, so ranger-base-na4rm is ready
rather than blocked, and the register it edits is the version that commit
shipped — not the one quoted above, whose head comment it rewrote. Whoever takes
na4rm re-reads treewidedoor_qa_test.go at main before anything else; the
decision below does not move, the line numbers and the pin and door counts do.
The scope call stands on its own terms: the decision is this bead and the
mechanism is that one, and shipping them together would have put a third
rewrite of that file in flight on the day two already were.

**1. A third seed for the posse half — which is the enumeration rule the
treepins half already has.** Every posse-half rule keys on a test COMPUTING
the repo root (`qibRepoRoot`, or a helper that walks from it), because
`go test` runs internal/posse's binary with the PACKAGE directory as cwd, so
reaching the tree costs an explicit climb. A package-directory census computes
no root at all, so neither rule can see it — and the register's stated lesson
("the class was never bounded by how a test spells its root, so the thing that
bounds it is the single helper, not another rule") does not forbid this, because
it is about root SPELLINGS of tree readings and this is a different key.

The new seed is not new code. `twdRelTreePath`, `twdRelEnv` and `twdReach`
already derive exactly this shape for internal/treepins — "a walker rooted at a
relative tree path", reading a literal, a local, a package const or a range
variable over a literal list. The only thing that differs between the two
packages is what a relative literal MEANS: the repo root in internal/treepins
(its `TestMain` chdirs there), the package directory in internal/posse. So the
posse half unions in the seed `f.relEnum != nil`, with the same closure. Two
things the implementation must then measure rather than assume:

- the live member count after the rule, against the fourteen below. The rule
  will also catch any test that globs package-local FIXTURES rather than
  sources, which is a member by the key and may not be one by intent; whatever
  it finds is dispositioned where it is found, not waived.
- that it does not reach ranger-base-nt0pi's three climbs, and must not be
  asked to. `twdRelTreePath` refuses a `../` prefix on purpose; those three
  are derived from `f.ascent`, which `twdAnalyze` already computes and nothing
  reads. The two fixes compose and neither subsumes the other.

**2. A door that carries `-tags`, which no door does today.** Seven of the
fourteen are in `posse_arm2` or `posse_arm3`, and a door's `-run` runs with no
tag at all — held by `TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm`
(armtags_qa_test.go, arm 4), whose whole point is that `go test -run` over a
name outside the build exits 0, so a door and the pin it cannot reach stay
green together. Admitting the arm-1 seven alone would be a class with a silent
half, and arm 2 of the register is two-way, so it would also be a red register
until every member is placed.

The generalization is one field. `armDoorVars` is a hand-kept list of door
variable NAMES — and it has already drifted once, missing `QA_EXECWRITE_PINS`
until ranger-base-6cznr renamed that door's pin. It becomes a list of
(variable, arm) pairs, arm 4 checks each door pin against the arm its door's
`-tags` names instead of against arm 1, and the Makefile's recipe for a tagged
door passes that tag. The precedent for a tagged line beside a `-run` door is
already in the tree: `test-arm2` and `test-arm3` each run a `-run` door onto
`TestQAEverySuiteArmTypeChecks` before their tagged `./internal/posse` line
(ranger-base-p1r17 put the door first on purpose, because a compile failure in
the tagged line would otherwise abort the recipe before the door ran).

What is NOT acceptable is the third way in: a `twdDoorHolders` row. That map is
"the ONE way into a door variable that does not go through the derivation", its
own comment calls it "exactly the shape of parking spot this file keeps warning
about", and it holds three rows for one cause — a pin that reads named files and
enumerates nothing. A census that enumerates is the opposite case. Fourteen
rows there would be the register conceding the class it exists to derive.

## THE POPULATION, and why it is not pinned here

MEASURED 2026-10-31 at HEAD 7db75f44 (census program and rule:
docs/notes.d/ranger-base-kkneb.md, appendix). Fourteen, with the file and arm
each was read back from, verified here by `grep -ln "^func <name>("` plus the
file's build line:

    TestQAEverySandboxProbeFileIsGated           seatbeltapply_qa_test.go      shared
    TestQAOneThrottleOnlyWatchArmsTheRefill      verifyesa0j_qa_test.go        shared
    TestQANoEventSubscriptionRemainsInInternalPosse    herdrhintsgone_qa_test.go     arm 1
    TestQANoInferredIntentMechanismRemainsInInternalPosse acceptanceexplicit_qa_test.go arm 1
    TestQANoLaunchPathSpellsTheSeatbeltWallPredicateItself credentialdenymove_qa_test.go arm 1
    TestQANoProductGitDiffReaderIsBare           extdiff_qa_test.go            arm 1
    TestQAProcessNoticesResolveThroughTheDeclaredSeams processnotices_qa_test.go     arm 1
    TestQAEveryCountedSkipReasonHasAReportingSite      settledrace_qa_test.go        posse_arm2
    TestQAEveryTestBinaryReExecKeepsTheChildATestBinary reexecenv_qa_test.go         posse_arm2
    TestQAOnlyWorktreeGoWritesAMergeRefusal      mergereasonfloor_qa_test.go   posse_arm2
    TestBackupHasNoOverride                      backuprefusal_qa_test.go      posse_arm3
    TestQAOneDoorForTheReportedReadingAndItNamesNoRuntime reportedreading_qa_test.go    posse_arm3
    TestQAPosseNeverWritesGroksTrustTable        skillsdirtrust_qa_test.go     posse_arm3
    TestQAWatchStreamWritesGoThroughTheDispatcher      watchhang_qa_test.go          posse_arm3

("shared" is no build line at all, so those two compile into every arm. That
is why the fourteen split 7/3/4 across the cost table's three doors and not
7/5/6 as a per-arm tally would read: the untagged door already reaches both
shared pins, so the two tagged doors need name only the pins that carry a tag.)

That list is a reading of a date, and it is deliberately NOT pinned as a
register. The whole point of decision 1 is that a derivation produces the live
membership; a hand-kept list of fourteen would be a second carrier that the
implementation deletes, and this repo has the scar tissue to prove it — every
count in `treewidedoor_qa_test.go`'s head comment that was typed rather than
derived went stale (ranger-base-4jogv, ranger-base-xed72, and the "seven doors"
over an enumeration of eight at d189b623). The live number will be the one the
third seed derives.

What carries the decision until then is not this file alone: the two head
comments that posed the question — `herdrhintsgone_qa_test.go` and
`reexecenv_qa_test.go`, the two pins ranger-base-8wxbn named — now state the
answer and point at ranger-base-na4rm, which is open, rather than at a decision
bead that closed. That is the defect ranger-base-49iga filed, fixed where it was
found.
