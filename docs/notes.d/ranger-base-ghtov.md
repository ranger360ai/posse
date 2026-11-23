# ranger-base-ghtov — the replay recipe read a stale HEAD, so a faithful replay landed nowhere

Landed 2026-11-23, linux 7.2.15-1-omarchy, git 2.56.0, go 1.26.10.

`mergeBlockedReplay` (internal/posse/dispatch.go) is the HOW every seat
dispatched on a merge-back block reads. It said *"In YOUR tree, on `<base>`:"*
and then every command in its loop read `HEAD`:

    TREE=$(git merge-tree --write-tree --merge-base=$C^ HEAD $C | head -1)
    git diff --diff-filter=D --name-only HEAD $TREE

A dispatched seat's `HEAD` is where the base stood when it was **launched**
(AGENTS.md, "Landing the plane"). Nothing in the copy moved it onto the current
base, and nothing told the seat to ask whether the base had moved since. So a
seat launched before a sibling lands resolves every conflict against a base
that no longer exists.

## Why the prescribed read-back says nothing about it

The copy's last step is the whole-SET key read-back:

    diff <(git log --format='%ae%x1f%aI%x1f%s' <base>..<sha> | sort) \
         <(git log --format='%ae%x1f%aI%x1f%s' <base>..HEAD | sort)

Both sides are **identities** — (author email, AUTHOR date, subject), which is
the one arm of `equivalentOnBase` a hand-resolved replay leaves. A replay
written on the launch base reproduces those keys exactly, and the stale base is
an ancestor of the current one, so `<base>..HEAD` lists precisely the replay
commits and the diff is **empty**. The step a seat is told to stop on cannot
see the defect it is stopping over: the keys are an identity, never a position,
and the landing is a position — `git merge --ff-only`, which needs the base to
be an ANCESTOR of the commit.

Note that `<base>` here is a ref NAME (`main`) and not a sha frozen at filing
time, so `<base>..HEAD` already reads the current base. The blindness is not a
stale operand in the read-back; it is that no operand in it is about position.

## The reading that filed it

MEASURED 2026-11-22/23 (recorded on ranger-base-56gd0's verify of
ranger-base-0v6a3, and on ranger-base-ghtov):

    git log -1 --format=%P 1c9a6f26   -> 7dc3313e   (the replay's base = the
                                                     session's LAUNCH base)
    main at 7dc3313e 19:07:14, then 958b4172 20:41:03, a25855a0 20:57:55,
    6706f7c0 21:06:06
    git log -1 --format=%cI 1c9a6f26  -> 2026-11-22T21:35:47-05:00

The base had been four commits behind for the 29 minutes before the replay
commit was written. The replay itself was faithful — its three key fields were
byte-identical to the original's, so `equivalentOnBase` would pair it — and:

    git merge-tree --write-tree --merge-base=1c9a6f26^ main 1c9a6f26
    -> exit 1, conflicted

The close's own read-back was empty and true, the seat closed honestly, and the
launcher re-filed the same block (ranger-base-zr2mf) with the work still off
the base. This is the second way a replay lands nowhere; the first, a squash, is
the only one the copy named.

## The fix, and what each half is worth

The recipe now opens with the step and closes with the check:

    git merge --ff-only <base>
    git rev-parse <base> HEAD   # the same sha twice, before you start
    ...
    git merge-base --is-ancestor <base> HEAD   # exit 0, or it lands nowhere

MEASURED 2026-11-22, git 2.56.0, linux, in a scratch repo with a worktree:

- `git merge --ff-only <base>` in a session worktree moves `HEAD` onto the base
  with that base **checked out in another tree** — attached (the branch moves
  with it) and detached (HEAD alone moves) both exit 0;
- it writes no commit and leaves no sequencer state: the git dir afterwards
  holds `HEAD index logs ORIG_HEAD refs` and nothing of `MERGE_HEAD`,
  `CHERRY_PICK_HEAD` or `sequencer`, which is why it is not the hazard
  AGENTS.md warns about for `rebase`/`cherry-pick`;
- it is **REFUSED** once the seat has committed on the stale base (diverged),
  which is why the refusal is given a meaning and a way round.

The way round is measured too: `git reset --keep <base>` from a diverged,
attached HEAD moved HEAD and the branch onto the base, **kept** the untracked
and uncommitted files in the tree, added no sequencer state, and left
`git rev-list --reverse <base>..$T` naming the replay commits to walk again —
so the recovery is the same loop with the seat's own commits as the originals,
which already carry the originals' keys. The two shapes it must not reach for
are a rebase (the copy's own warning) and a detached HEAD holding work the
branch does not, which blocks the landing by itself (`attachForReplay`,
internal/posse/worktree.go).

## What is pinned, and the mutation that found the weak pin

- `TestMergeBackReplayOnAStaleLaunchBaseLandsNowhereAndTheFastForwardFixesIt`
  (internal/posse/mergebackreplay_test.go, arm 2) is the honest half: one
  blocked branch, two seat trees launched at the same base, the base moving
  under both, and the recipe run twice. The stale run's key read-back is
  asserted **empty** — `mergesCleanly` refuses the tip, the ancestry check is
  nonzero, and `MergeSessionWork` reports a block, which is what re-files. The
  fixed run differs by the fast-forward alone: it lands, the stranded commit is
  then accounted for on the base (`replayed`, not `byPatch`), and the block does
  not re-file.
- `TestMergeBlockedReplayPutsHeadOnTheCurrentBaseFirstAndChecksItLast`
  (internal/posse/mergeblocked_test.go) is the cheap half, because a seat reads
  the TEXT — and it asserts POSITIONS: the fast-forward before the walk over
  the originals, the ancestry check after the key read-back. A fast-forward
  named after the loop is one the seat reaches with its replay already
  committed, where git refuses it; an ancestry check named before the loop is
  exit 0 for every stale seat there has ever been.

MEASURED 2026-11-22, by reverting each half with the pins in place: deleting
the behavioural test's fast-forward, deleting either command from the recipe,
and moving the ancestry check before the loop are all red. One mutation
**survived** first time round — deleting the runnable `git merge --ff-only
<base>` line left every pin green, because the copy names that command in its
prose as well and the pin was a bare substring. Both pins now carry the
indentation that makes a line a command (`"\n  git merge --ff-only main\n"`),
and that mutation is red.
