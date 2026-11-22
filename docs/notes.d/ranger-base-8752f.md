# Three arms of the wall-member censuses, each priced before it was widened (ranger-base-8752f)

ranger-base-udagn verified four closes and found three NEW arms the closes
themselves added — each found by extending the closer's own key with a planted
pair. All three said "lane's call" and named two shapes. One widening landed,
one bound came out, and one widening was measured and **not** taken. This page
is the readings that decided each, because two of the three decisions are the
opposite of the shape the finding leaned toward.

Environment for every reading below: linux 7.2.15-1-omarchy x86_64, go1.27.3,
16 cores, at 330d6940, 2026-11-22. Each mutant was applied ALONE, run, and
restored from a copy taken before any mutation; every restore read back
sha256-identical and `git status` clean between them.

## 1. The abstain census could not see a shell-wrapped seat

`internal/posse/wallseatabstain_qa_test.go`. Arm 1's key was `exec.Command`
whose argv[0] is the literal `"bwrap"`. Arm 2's was the NEGATIVE of "names its
program with a literal at all". A program that IS a literal but is `sh`,
`script`, `setsid` or `env`, with `bwrap --…` in its argument, fell between
them: not arm 1's key, not a variable, so no register row was demanded and
nothing said so.

The finding leaned toward folding the wrapper route into arm 1, the way
`internal/treepins`' `wmaSeatLaunches` reads it one package over — "two keys
for one claim is also two answers about one member set". **That shape reds the
tree today**, and the reason is a granularity the finding's own measurement
could not see: `wmaSeatLaunches` grades a whole top-level function, while arm 1
owns routing at the INNERMOST frame that runs something.

    MEASURED: the wrapper route folded into arm 1's key
      -> RED at wallunixlive_test.go:473 (a func literal) "runs a seat inside
         a rendered wall and does not route the abstention"

That site is the second loop's `sh -c 'bwrap "$@"; :'` arm, in a sibling
`t.Run` closure that does not route — and must not. THIS TEST SIGKILLs the sh
after reading the seat's self-report and discards its `Wait` error, so routing
it would abstain on every ordinary run and hold the wall check at exit 3
forever (ADR 0097 D3). That is the `grokleader` row's reason exactly. The
finding read the test as covered "because the SAME function also launches a
seat directly", which is true of the test and not of the frame.

So the route went where this pin's other un-gradable spelling already goes:
the REGISTER, which has the dispositions and the prose for it. What landed:

- The program is read as a command WORD now (`wsaSeatRe`, character for
  character `wmaSeatRe`), so `exec.Command("/usr/bin/bwrap", …)` is the same
  direct run arm 1 always meant.
- `wsaWrappedSeatRun` is the third answer: a literal program that is not
  bwrap, with the word in an argument — one hop included, so a line built in a
  local reaches it.
- Arm 2's walk is `wsaUngradableSeatRuns`, a function the controls drive over
  planted source, and one register row covers `wallunixlive_test.go`.

Five mutants, one at a time, every one red and on its own rows:

| mutant | what red |
|---|---|
| the wrapper half of arm 2's key stops matching | the live wrapper floor, the stale-row reading, 5 planted rows |
| the one-hop name read goes | the 2 planted one-hop rows |
| `wsaSeatWord` as a `strings.Contains` | the `bwrap-wrapper` program row, the neighbour row |
| the new register row deleted | `wallunixlive_test.go:473` named, with the wrapper spelling in the message |
| arm 1 back to `prog == "bwrap"` | the absolute-path row |

Live reading after: 56 members in 24 files, 33 literal bwrap seat runs, 5
un-gradable member tests (1 through a wrapper), 9 watcher-run sites.

## 2. The mechanism census's build side is a frame short — and stays one

`internal/treepins/wallmechanswer_qa_test.go`. `member() = ships && builds`.
The close widened `ships` to a fixpoint because extracting the SHIP left the
test `builds && !ships` and the helper `ships && !builds`. `builds` is still
set by a `newLandedChecker` call in the body itself, so extracting the BUILDER
— the mirror refactor — walks through the same way, with the suite green. That
is a false GREEN in the direction the file exists for, and the blindness list
named five directions and not this one.

**The closure was built, measured, and dropped.** It cannot be the ship side's:
`wckFixture` builds a checker for some thirty arms and the whole point of it is
that it answers for them, so a closure that read every caller of a builder as a
builder reds all thirty. Narrowed to carry up only from a builder that settles
nothing — answers nowhere, reaches no wall check — it adds exactly one member
to today's tree:

    MEASURED: the build side closed, narrowed as above
      arm 1 -> RED, 4 members, the new one
               TestEveryVerdictLineNamesTheLauncherThatRanTheCheck
               (internal/posse/landedlauncher_test.go:73)
               "ships through wckFixture — builds through landedCheckFixture"

That body is CORRECT on macos today: its three wall arms take `wckFixture` and
get its `wallMechFn`; the unanswered checker it inherits comes from
`landedCheckFixture` in three OTHER arms, which assert landed verdict lines and
never touch the wall step. The pin's decl-level reading cannot localize which
checker reaches the wall check, so the finding is a false red.

What settled it is the PRICE of that false red, which is not the one line this
file prices elsewhere for an answer that was not needed. The test is
`t.Parallel()` with parallel subtests. Of the three seams: the package seam
writes a package-level var and is serial-only by its own comment
(`TestATreeThatShipsNoWallCheckIsLeftAloneEntirely` says so, and
`wallMechanismForTest` is a bare assignment plus a `t.Cleanup` restore); the
host seam is a skip, which would cost the macos leg three landed verdict lines.
What is left is three `c.wallMechFn` lines inside arms that never touch the wall
step. A false red at that price buys less than it costs.

So the build side stays one body at a time, and the deliverable is that it is
now a DECISION rather than an oversight: a "THE BUILD SIDE DOES NOT CLOSE
EITHER" paragraph carrying both readings, a sixth entry on the blindness list
saying it is the one false green there, a row in the graded-against-the-tree
table, and arm 3's `the BUILD one frame down` row. That row asserts the member
set exactly, so it is a live witness and not a comment:

    MEASURED: the same closure mutant, against arm 3
      -> RED, "member(s) = [TestBuildsOneFrameDown], want []" and one finding

The day the build side closes, that row reds and its replacement says so.

## 3. A launch ceiling nothing can trip

`internal/treepins/wallmemberaccount_qa_test.go:573`. The widening of the
seat-launch key came with `if len(wrapped) > 8` beside the floor, on the
reasoning that widening a key is how a census starts grading functions that
launch nothing. It cannot perform the detection its own message claims:

    MEASURED, one mutant at a time
      the whole `if len(wrapped) > 8` block deleted   -> nothing reds
      wmaSeatWord as a strings.Contains, ceiling at 1 -> still 2 of 32
      the same strings.Contains mutant, arm 3         -> RED, and only at
          "a shell line naming a neighbour of bwrap"

The bound is four times slack over the canonical over-read, and no value of it
distinguishes the two keys over this tree: the neighbour names an over-reading
key would newly match (`bwraptest`, `bwrap-wrapper`, `bwrap.sh`) live in arm
3's fixtures and in no member file. Arm 3 holds the claim hermetically, which
is what that comment already called "the precise half of this". The ceiling is
gone, the head comment carries the measurement, and arm 1 logs its census (24
member files, 32 launchers — 31 direct, 2 wrapped — 37 child errors graded)
instead of asserting a bound nothing witnesses.
