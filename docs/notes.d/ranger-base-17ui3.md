## The same block, filed a second time — what a do-not-land close costs (ranger-base-17ui3)

`ranger-base-tb90m` closed the merge-back block on
`posse/dinesh-posse-ranger-base-ifiz5` do-not-land on 2026-09-03 23:25, with
its reading committed as `docs/notes.d/ranger-base-tb90m.md`. Two hours
later the sweep filed `ranger-base-17ui3` — byte-identical title, same
untouched branch, a fresh P1 at a fresh seat. This fragment is the second
pass. It re-confirms the verdict at `main` 9920e75, and records the three
things the first pass could not know.

### 1. tb90m's last section was right about `main` and wrong about the cost

It is titled "Leaving a superseded branch standing is safe" and it argues,
correctly, that merge-back is `--ff-only` so a conflicting branch can never
regress `main`. What it then says — that the branch "costs a line in
`posse worktrees`" — was the whole bill only while the bead stayed open.

The dedupe read OPEN beads only (`openMergeBlocked` → `openTitledBead` →
`openMatchedBead` → `OpenLabeledAny`), so **closing the block is what
destroyed its own dedupe.** A do-not-land verdict is the correct end for a
superseded branch, and reaching it re-armed the filing.

Already diagnosed and fixed by someone else: `ranger-base-j8qmj`, landed on
`main` as `c3ab918`. `priorMergeBlocked` now reads closed beads too, and a
closed verdict stands while the branch has not moved since — committer date
of the tip against the bead's `closed_at`. For this branch the margin is
comfortable and on the right side:

    tip 3a604c1 committer  2026-09-03T21:28:56-04:00
    tb90m closed_at        2026-09-03T23:25:52-04:00

So under `c3ab918` this second filing would not have happened.

### 2. The fix is on `main` and NOT in the binary running the sweep

    $ posse version
    posse 0.4.0+c592683 (herdr-native)      # ~/.local/bin/posse, built 09-03 23:54
    $ git merge-base --is-ancestor c3ab918 c592683 ; echo $?
    1                                        # the running binary predates the fix

`c592683` is 34 commits behind `main`, and `c3ab918` is one of the 34. **A
do-not-land close recorded today is still re-filed by the sweep** until a
build carrying `c3ab918` is promoted. The stamp is the reading that decides
this, not the source tree — see the same caveat on `ranger-base-s0eo2`,
where `81469a8`'s wider detector is on `main` and the sweep still runs the
old one.

`posse promote` is denied by the dinesh PID, so that half is the operator's.

### 3. Retiring the tree is not a seat's to do, and it is TWO walls, not one

`ranger-base-s0eo2` records that `git worktree remove` "is denied outright by
Claude Code's auto-mode classifier from a seat". True, and measured again
here — but a reader could conclude that a Bash permission rule would fix it.
It would not. Underneath the harness there is a kernel wall, measured from
this seat under `RHQ_CAGE=seatbelt`, **with a control arm that succeeds**:

| target | create a file |
|---|---|
| `~/src/posse/.git/` | EPERM |
| `~/src/posse/.git/worktrees/<the ifiz5 session>/` | EPERM |
| the ifiz5 working tree | EPERM |
| **my own worktree (control)** | **writable** |

Removing a directory needs write on it, so `git worktree remove` and
`git branch -D` both die on the kernel before the classifier is even the
question. Lifting one wall reveals the other. This is the same EPERM
tb90m's fragment measured for a rebase, and the same one `ranger-base-j8qmj`
names for the missing `cherry-picked-from` trailers — one wall, three
consequences.

The consequence for practice: **a seat can only ever file the retire ask.**
`OPERATOR: retire the superseded worktree …` is the established shape
(`ranger-base-s0eo2`, `-s0ih6`, `-4nco1`, `-enaij`); this branch's is
`ranger-base-6uqev`.

### 4. The verdict re-confirmed at `main` 9920e75, and it has hardened

`3a604c1` turns two `PARKED (laurie, …)` headers into `PARKED (qa, …)` in
`internal/posse/queueremote_qa_test.go`. At tb90m's close, `main`'s side
differed by exactly one word — `45b0c3c` had dropped the parenthetical name
entirely. It has since gone much further:

    git show <branch>:internal/posse/queueremote_qa_test.go | wc -l   ->  75
    git show main:internal/posse/queueremote_qa_test.go     | wc -l   -> 190

`ranger-base-m6szh` and `ranger-base-hp327` **fixed and un-parked both
pins**. The branch still carries `t.Skip("parked: …")` on tests `main` now
runs, and its two edited lines no longer exist there in any form. Landing it
would re-park two passing pins.

This is `ranger-base-avq12`'s monotone observation on a second branch: a
superseded strand drifts *further* from `main`, so each re-file makes
landing more destructive and the answer can only get more certain. A
do-not-land verdict never flips back.

### 5. An overlay cannot be a disk-reading pin's wrong arm — but it CAN be its scaffold

tb90m's fragment establishes the rule: `TestShippedTreeNamesRolesNotThisCrew`
walks the shipped tree **from disk at run time**, `go test -overlay` swaps
files at *build* time, so the pre-fix bytes fed through an overlay come back
`ok` and prove nothing. That still holds. The other direction does not
follow from it, and it is what made this pass measurable at all.

`internal/posse` **does not compile at `main` 9920e75** — `fakeBdDropClosed`
is declared twice in `herdr_test.go` (`ranger-base-5im1q`, P1, open; `3075168`
added the superseding copy without deleting the old one). Another persona's
file and another bead's commit, so not repaired here (ADR 0022). The pin
could not run at all.

An overlay that deletes the dead duplicate fixes exactly that, and it is
sound here for the same reason it was unsound as a wrong arm: **it changes
only what the compiler sees, and the compiler is not this pin's subject.**
The on-disk bytes it walks are untouched.

    sed '526,546d' internal/posse/herdr_test.go > $SP/herdr_test.go
    printf '{"Replace":{"%s/internal/posse/herdr_test.go":"%s/herdr_test.go"}}' "$PWD" "$SP" > $SP/overlay.json
    go vet -overlay=$SP/overlay.json ./internal/posse/          # clean

    # ARM A, control:  ok 3.164s
    go test -overlay=$SP/overlay.json ./internal/posse/ \
        -run TestShippedTreeNamesRolesNotThisCrew -count=1

    # ARM B, wrong arm: the crew name back on disk, in the comment the strand edited
    sed -i '' 's|^// FIXED (ranger-base-m6szh;|// FIXED (laurie, ranger-base-m6szh;|' \
        internal/posse/queueremote_qa_test.go
    #   --- FAIL … the shipped tree names the originating instance (2 hits):
    #         internal/posse/queueremote_qa_test.go:10: laurie   (and :92)
    git checkout -- internal/posse/queueremote_qa_test.go       # hash-object equal, 0ed000db

The general rule, both halves together: **an overlay is a control for what
the compiler reads and a scaffold for everything else.** Never the wrong arm
for a test that walks the repo; fair game for getting that test to build
past a red that is not the one you are measuring.
