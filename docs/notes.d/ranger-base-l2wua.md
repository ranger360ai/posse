## A fix can be UNLANDED and not stranded: sweep every branch by patch-id, not main by sha (ranger-base-l2wua)

Measured 2026-09-03, closing the `ranger-base-zikpp` findings bundle. Its one
finding — `reconcileSeats` releasing a seat on a listing `Sessions()` only
ABSTAINED on — carried its own bead (`ranger-base-6swlr`, ADR 0006 §1's live
dispatch-defect exception), which closed with `0e08c42`. Every strand signal
we own then fired at once, and every one of them was answering a narrower
question than "is this fixed":

    git merge-base --is-ancestor 0e08c42 main   → FALSE
    git merge-base --is-ancestor 0e08c42^ main  → TRUE     (a plain strand, not a rebase)
    git log --oneline main --grep=ranger-base-6swlr        → empty
    git grep -n listSessions main -- internal/posse/       → empty
    posse worktrees  → "1 commit(s) not on main, for ranger-base-6swlr"
    git cherry main posse/<persona>-posse-ranger-base-6swlr → +

All true, and the rescue they prescribe — cherry-pick it onto the bead you
are holding — would have been wrong. The fix was already carried forward on a
**different, still-open** bead's branch: the handoff `ranger-base-5kiu4` was
cut from the fix's own worktree, so `posse/<persona>-posse-ranger-base-5kiu4`
holds `c1b8ac3`, a `-x` pick of `0e08c42` rebased onto main's current tip,
with an identical `git patch-id --stable` (`3e6d2bc6…`). Picking it a third
time manufactures the duplicate that `docs/notes.d/ranger-base-as19.md`'s
equivalence machinery then has to reason about.

**The sweep that answers the actual question** — who holds this work — asks
every branch, not just `main`, and asks by content:

    PID=$(git show <sha> | git patch-id --stable | cut -d' ' -f1)
    for b in $(git for-each-ref --format='%(refname:short)' refs/heads); do
      for c in $(git rev-list main..$b 2>/dev/null); do
        [ "$(git show $c | git patch-id --stable | cut -d' ' -f1)" = "$PID" ] && echo "$b $c"
      done
    done

Two rows here, one of them a branch cut off main's tip. That is the shape a
handoff produces **by construction**: a follow-up bead worked in a tree cut
from the fix's tree carries the fix, so a closed bead's strand is routinely
also an open bead's inheritance. `git cherry`, `--is-ancestor` and
`--grep=<bead-id>` are all main-relative and cannot see it.

Unlanded is still not landed, and the sweep is not a reason to stop worrying:
`landsweep.go` lands the tree of every bead the store now calls closed, keyed
on `branch.<b>.posseBead` (present on both branches here), and
`git merge-tree --write-tree --merge-base=<mb> main 0e08c42` exits 0, so this
one lands clean whenever the sweep next runs. Say WHICH of those it is on the
bead. "Closed" is a claim about the store; "fixed" is a claim about the tree.

**And neither answers whether the defect is live.** That takes the repro at
HEAD, two arms, same binary — the bug bead's own repro compiled against the
API that exists in *both* arms (no `listSessions`), with the fix supplied as
an overlay rather than a checkout — a build-time file swap, so no golden copy
and nothing left in the tree:

    # the repro body from ranger-base-6swlr, dropped into internal/posse as a
    # scratch _test.go, run in both arms, then removed — it is not committed,
    # because the shipped pin for it rides on the fix's own commit.
    git show c1b8ac3:internal/posse/dispatch.go   > $D/dispatch.go
    git show c1b8ac3:internal/posse/herdrback.go  > $D/herdrback.go
    # {"Replace":{"<repo>/internal/posse/dispatch.go":"$D/dispatch.go", ...}}
    go test -overlay $D/overlay.json -count=1 -run <the scratch test> ./internal/posse/

ARM A, main at `6ce7ca0`: **FAIL**, reproducing the reported ACTUAL to the
character — `busy=map[ranger-003:a-2]`, `↺ seat ranger-003 released: no
session (held a-1)`, and two live sessions for one persona.
ARM B, the same fixture with those two files replaced: **PASS**.

So the P1 is live on `main` today and the fix that closes it is correct and
sitting on two branches. That is the sentence a findings bead owes its
reader, and no single git command produces it.
