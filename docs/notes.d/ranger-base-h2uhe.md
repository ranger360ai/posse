# A pending row's "unlanded" is a reading of main, and it goes stale while you work (ranger-base-h2uhe)

MEASURED 2026-11-09, this tree at main 972338dc and at 80373085, darwin 25.6.0
arm64.

`make test` and `make tree-check` were RED on main for every seat on the box.
Not from anyone's diff: the hash-spelling census door (ranger-base-uxadf,
972338dc) shipped with two `hshPending` rows whose fix had already landed, and
the arm that exists to catch a stale pending row caught both on the door's
first run against main.

    --- FAIL: TestQAEveryShippedScriptThatHashesPicksItsHasherByProbe (0.12s)
        hashspelling_qa_test.go:334: hshPending names scripts/gotest.sh and
          every hasher mention in it now sits inside a probe — THE FIX
          LANDED. Delete the row ...
        hashspelling_qa_test.go:334: hshPending names
          scripts/verify-hook-freshness.sh ... THE FIX LANDED. Delete the
          row ...
    FAIL  github.com/ranger360ai/posse/internal/treepins   6.900s
    make: *** [corpus-check] Error 1

## The blast radius is why this was P1

`corpus-check` is a `tree-check` door, and `tree-check` is a prerequisite of
`make test` and `make test-arm1` both. So NO arm of the suite ran at all, for
anybody: `make test` exited 2 in about a minute having compiled nothing. Every
seat that rebased onto 972338dc and typed the house command got a red that was
not its own, which is the ranger-base-4v4r6 class AGENTS.md already names —
three concurrent beads, three worktrees and three suite runs spent on one
landing's residue.

## The timeline, which is the whole lesson

| commit | what |
|---|---|
| `196ecca9` | main when ranger-base-uxadf MEASURED. `e2499fcc` is NOT an ancestor — the rows' claim was TRUE here. |
| `e2499fcc` | ranger-base-sydnc lands the fix for both scripts. Two commits later, DURING that session. |
| `972338dc` | ranger-base-uxadf commits the door, four commits after sydnc, with the rows unchanged. |

Nobody wrote a false statement. The statement was measured, correct, and then
the world moved under it — and the mechanism's own commit message still said
"the two findings whose fix is written but unlanded", because that is what the
session had read.

So: **a `hshPending` row's precondition is a reading of MAIN, not of your
tree, and the reading has a shelf life of one landing.** The check is one
command and it belongs at commit time, not at measurement time:

    git merge-base --is-ancestor <the commit the row cites> main && echo STALE

The same shape binds every declaration whose reason is "in flight": a pending
row, a skip naming a bead, an exemption that says "until X lands". If a row's
reason contains a commit SHA, that SHA is a claim about main that you re-check
before you commit, and the arm that reds over it is doing its job on the day
it reds.

## The fix

**Landed twice, by two seats at once** (ranger-base-rwlmu). This bead's commit
a569c1d6 did not reach main: while it sat on its own branch,
ranger-base-0z9ta read the same red and made the same deletion, landing it at
59e9106f. By the time this bead's merge-back was attempted the rows were
already gone, so there was nothing left of the code change to replay and the
merge was refused over the prose both beads had rewritten — the file's head
comment and `docs/notes.d/ranger-base-uxadf.md`'s amendment. What landed under
ranger-base-rwlmu is this page, the README row for it, and the parts of this
bead's prose 0z9ta's did not carry: the blast radius, and the
`git merge-base --is-ancestor` check stated as the thing to type at commit
time. **`git log --grep ranger-base-h2uhe` does not find the deletion** — it
is under ranger-base-0z9ta, and that is the one place to look for it. That
two seats took the same P1 within one hour is itself the cost of a red main:
the fix is cheap, the race is not, and neither session could see the other.

The deletion, in both cases, is what the arm's own message prescribes —
"Delete the row: that is what this arm is for", in the `isPending` arm of
`TestQAEveryShippedScriptThatHashesPicksItsHasherByProbe` (the `:334` in the
quoted failure above is where that line sat on main 972338dc; this change
moved it). Both rows out of `hshPending` in
`internal/treepins/hashspelling_qa_test.go`; the map is left declared and
EMPTY, which is the state it is meant to be in between findings, and its head
comment now says so and says to make the ancestry check before adding a row.
The file's opening paragraph, which also said "WRITTEN AND UNLANDED", is
dated to 196ecca9 rather than deleted.

`hshPending` held exactly those two rows, so there is no third to check: the
census reports every stale pending path in one run and reported these two.

## Verified

- `go test -run 'Hash' ./internal/treepins` — ok 5.776s (was the FAIL above).
- `make corpus-check` — ok 14.797s, the door that was red.
- `make fmt-check` — clean, typed by hand because the runs above were `-run`
  filters and no filter has ever named `TreeIsGofmtClean` (ranger-base-rulbl).
- `make tree-check` — the whole class green, 159s on a box whose load was not
  quiet.
- `make test` — all three arms green, exit 0: arm 1 472.890s, arm 2 320.889s,
  arm 3 373.977s, zero FAIL, and the silent-revert audit clean over 2,422
  commits. This is the command the bead is named for, and on main it had been
  exiting 2 in about a minute having compiled nothing.
- `go run ./cmd/checkorphans` after it, in a later call — clean.
