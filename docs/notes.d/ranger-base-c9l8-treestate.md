# ranger-base-d8o6 — `posse worktrees` calls an already-landed duplicate unlanded

Found verifying the close of **ranger-base-7tq2** (ranger-base-c9l8, laurie,
2026-08-30, at posse `e3c5cc3`). The detail lives here because the bd payload
gate refused these sentences on the bead; the bead points at this file.

**7tq2 verifies.** Its stranded commit `2aab2df` is on main as `7ff3e4d` with an
identical patch-id (`625b09d5127fc89d9a15fafada7a7828d3312b64`), it is the only
commit the olwk branch held that main did not, no later commit reverts it, and
the change is live at HEAD (`arm64_big_sur` in both generators). This is the
separate bug that close named in advance, in its own words: *"if `posse
worktrees` still names olwk after the next pass, that is a separate bug."*

## Measured 2026-08-30, posse `e3c5cc3`, in the main checkout

```
$ git cherry main posse/dinesh-posse-ranger-base-olwk
- 2aab2df280b024159b7838b6e0263eb0d5a22741        # minus: already upstream
```

The land sweep, three times in `dispatch-watch.log`, most recently line 89728:

```
≡ ranger-base-olwk 1 commit(s) on posse/dinesh-posse-ranger-base-olwk are
  already on main under other sha(s) (2aab2df280b0 as an equivalent patch on
  main) — nothing here is unlanded
```

`posse worktrees`, same binary, same tree, one command later:

```
posse/dinesh-posse-ranger-base-olwk
  ~/.posse/worktrees/posse/dinesh-posse-ranger-base-olwk → main  ·  1 commit(s) not on main, for ranger-base-olwk
```

Two answers to one question from one binary, and the operator-facing one is the
wrong one.

## Mechanism

`treeState` (`internal/rhq/worktree.go:1530`) asks
`git rev-list --count <base>..<branch>` — pure ancestry. The sweep asks
`equivalentOnBase` / `measuredOnBase` (`worktree.go:998`, `:1052`) — patch
equivalence. The comment above `treeState` already knows the two states differ:
it cites ranger-base-atxe and says the count "is true of a strand and of an
already-re-landed duplicate alike", and answers that by naming the *bead* rather
than by telling the two apart. The test that tells them apart is in the same
file, and the sweep prints its answer correctly.

## Why it matters beyond a wrong word

The listing is what an operator reads before running
`posse worktrees --land --force`, so a fully landed duplicate reads as work at
risk, permanently. And nothing retires the tree: the land sweep
(`internal/rhq/landsweep.go:119`) reports `Equivalent` but never calls
`RemoveSessionTree`, whose only caller is the herdr settle path
(`internal/rhq/herdrback.go:2368`) — which for this session ran long before the
equivalent patch reached main. The tree and branch persist unless someone
removes them by hand.

## Repro

1. Land a session branch's commit onto the base under a different sha (a pick
   onto another branch that then reaches the base), so the original branch is
   patch-equivalent to the base but not an ancestor of it. ranger-base-7tq2 did
   exactly this, on purpose.
2. Run the land sweep: it prints the `≡` line and "nothing here is unlanded".
3. Run `posse worktrees`: it prints "N commit(s) not on <base>".

Steps 2 and 3 disagree, and step 3 keeps disagreeing on every later pass.

EXPECTED: `treeState` agrees with the sweep, or at minimum distinguishes an
equivalent duplicate from a strand — the operator decision it feeds differs in
the two cases.
ACTUAL: the ancestry count, unqualified.

## Not pinned here

The fix is production code in `worktree.go` and belongs to the code lane. A pin
for it needs an equivalent-but-not-ancestor branch as its fixture **and a real
strand as its control arm** — a green `treeState` over a tree that could never
have been equivalent proves nothing.
