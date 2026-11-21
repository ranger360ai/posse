# Every walled seat commits on a detached HEAD (ranger-base-pe5g5)

*2026-11-15 · dinesh · design ranger-base-dvw0w D2*

Part of:

- ranger-base-21i98

## What changed

A seat that posse walls now commits on a detached HEAD at every walled tier:
L2 seatbelt, L3 bwrap and L4 container. Before this, only L4 did. So no
walled seat's git grant names a branch ref any more. The seatbelt leaf pair
`refs/heads/<branch>` + `.lock` is gone, and so are the create-only ancestor
dirs and bwrap's ref-directory bind. `packed-refs.lock` stays (ADR 0059 D1).
A launch with no OS wall keeps the old shape: on the branch, and it
re-attaches a tree that posse detached.

- `PrepareSessionHead(t, walled, caged, warn)` in `internal/posse/worktree.go`.
  `walled` decides the HEAD. `caged` only adds the container's bind sources
  (`logs/`, an empty `config.worktree`), which moved into
  `prepareCageFurniture`. L2 and L3 already handle an absent `config.worktree`
  in their own render, so this does not create one for them.
- `planLaunch` (`internal/posse/herdrback.go`) now calls it at the first line
  where the tier is final, with `walled = caged || wallRendered(cage, rt)`.
  It used to run beside the cage wrap. The ref grant producers
  (`sessionGitGrants`, `sessionRefDirs`, `bwrapGitSwap`) and the parity row
  (`bwrapRefGrantDifference`) already answered nothing over a detached tree,
  but they read the tree before that old call. The first launch into a tree
  therefore rendered the ref grant. With the detach first, they see the
  detached shape. A launch with no PID keeps a call at the old site, on the
  unwalled arm.
- The work prompt (`workPrompt`, `internal/posse/dispatch.go`) now says that
  behind a wall the HEAD is detached on purpose. It tells the seat to commit
  there and never check the branch out, and says that posse moves the branch
  at close.

The close path is unchanged. `spliceDetachedWork`, `attachForReplay` and the
dybv record (`branch.<b>.posseDetached`) were already there for L4, and an
L2/L3 detach now writes the same record.

## Verified

- `walleddetach_qa_test.go`: the walled-not-caged prepare detaches and
  records it, makes no `config.worktree`, and grants no ref in either
  mechanism's producer, and the close lands the work. A relaunch splices. An
  unwalled launch re-attaches and clears the record. A first seatbelt-tier
  `planLaunch` writes a `bwrap.args` / `seatbelt.sb` with nothing under
  `refs/heads` in it. A `cage: shims` launch stays on its branch.
- `TestLiveBwrapWalledSeatCommitsDetachedAndCannotWriteARef`, a live bwrap
  row next to the ADR 0086 D3 row: the seat commits detached and is refused
  its own branch, a peer's, main, a new session branch, a top-level branch
  and a hand-written ref file. The splice outside the wall then lands it.
- `TestQASeatbeltWalledSeatCommitsDetachedAndCannotWriteARef` is the same
  row under sandbox-exec. It skips on Linux and runs on the darwin CI leg.
- MUTATIONS RUN, each one reds: the detach moved back below the render (only
  the first-launch pin reds, which is the one written for it); `walled`
  ignored; cage furniture made for every walled launch; the live row rendered
  over the attached tree (it reds on four allowed ref writes).

## Not changed

The attached-shape pins (`bwrapgitrefs*`, `seatbeltworktreegit_qa_test.go`)
still pass and still cover the producers' attached arm. That arm now only
serves a walled launch into a linked worktree that is not a posse session
tree. Retiring D20's ref-directory machinery belongs to a later slice, not
this one.

That last sentence was short by one case for as long as this slice reached
only `planLaunch` (ranger-base-r9slg): the attached arm also served a walled
RELAUNCH of a posse session tree, because `RelaunchAgent` rendered the wall —
`wallRendered` then `WallPrefix` — and never asked `PrepareSessionHead`. Two
states arrived there attached, a seat launched by a posse from before this
slice and a seat that had run `git checkout <branch>` in its own tree, and
both got a grant naming a `refs/heads` entry for the rest of the session.
That launch site asks `PrepareSessionHead` now, pinned by
`TestQAAWalledRelaunchRendersNoBranchRef`, so the sentence holds as written
again.
