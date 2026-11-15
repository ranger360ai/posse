## The first accidental detach, and the one command that caused it (ranger-base-3iybl)

Recovery bead over `ranger-base-cq3vi`, whose single commit `517e116e` was on
no branch at all. This is the `landed()` arm that says "the tree's HEAD is off
its own branch, so no merge here can reach it" — **the first time that arm has
fired in this repo's history**. The guard behaved. Its printed remedy did not:
running it verbatim rescued the work for one pass and the pass after that
stranded it again, which is ranger-base-wg4z1 and the half of this worth
reading.

MEASURED 2026-11-14 over `$BEADS_DIR/issues.jsonl`: **113 `merge-back blocked`
beads, 1 of them this reason** (and none the neighbouring "unreferenced and a
retire would lose it"). The other 112 are real conflicts and other causes. So
this is a one-off, not a class, and it has not earned a line on AGENTS.md.

## What did it

The cq3vi seat ended an unrelated scan loop with a trailing `git checkout -q
HEAD~0`, from the session worktree, 22 minutes before it committed:

```
b5a6f5c3 … 1794708580	checkout: moving from posse/jian-yang-posse-ranger-base-cq3vi to HEAD~0
b5a6f5c3 517e116e … 1794709903	commit: ranger-base-cq3vi: check 4 reads every staged file, …
```

(`.git/worktrees/jian-yang-posse-ranger-base-cq3vi/logs/HEAD`.) `HEAD~0`
resolves to the commit HEAD was already at, so the working tree did not change
by a byte and nothing looked different afterwards — but a checkout of a
*commit* detaches, and the commit 22 minutes later wrote no ref. The seat then
closed its bead on a commit nothing pointed at.

## Why no signal reached the seat, MEASURED

git 2.56.0, throwaway repo, the four readings that matter:

| command | what it says |
|---|---|
| `git checkout HEAD~0` | the full `advice.detachedHead` paragraph, ending `HEAD is now at …` |
| `git checkout -q HEAD~0` | **nothing — stdout and stderr both empty** |
| `git status` (detached) | `HEAD detached at <sha>` — the signal was there |
| `git commit -qm … -- <path>` (detached) | exit 0, no warning; the branch does not move |

So `-q` is what ate the one warning git volunteers, and after that only
`git status` would have said so, which nothing in the close path reads. A
`git log --oneline -1` after the commit shows the commit and tells you
nothing about what reaches it.

## Why the guard caught it anyway

`spliceDetachedWork` puts a detached tree's work back on its branch, but
`MergeSessionWork` runs it only `if launchedDetached(t.Repo, t.Branch)` — the
`branch.<branch>.posseDetached` record posse writes when *it* detached the tree
on purpose for the container tier. That branch carried `possebase` and
`possebead` and no `posseDetached` key, which is the definition of an
accidental detach, so the splice correctly stood down and `landed()` reported
it instead. That asymmetry is deliberate and documented at
`detachedKey` — "Without a record the splice would silence the guard for every
session, which is the one thing it must not do" — and this is the first
field case to exercise it. It behaved.

## The recovery, and the remedy that was not enough

No replay, rebase or cherry-pick was needed or wanted — the block's replay
recipe is for a *conflicting* merge, and this merge is clean:
`git merge-tree --write-tree --merge-base=517e116e^ main 517e116e` exits 0.

So the first step was `git branch -f posse/jian-yang-posse-ranger-base-cq3vi
HEAD`, which is what the block itself named, and it is fast-forward only here
— the branch sat at `b5a6f5c3`, the work's own parent. `posse worktrees` then
read the tree as landable with no refusal.

**That held for exactly one pass.** `branch -f … HEAD` moves the BRANCH to the
detached HEAD and leaves HEAD detached, and the next launcher pass ran its
replay — `git rebase <base>` in the tree (worktree.go:1834) — which on a
detached HEAD moves HEAD and does not touch the branch. The fast-forward under
it takes the BRANCH (`merge --ff-only t.Branch`, worktree.go:~1936), which the
rebase never moved, so the pass rebased successfully, left its output
`04487ef1` on no branch, and refused the landing. The block regenerated,
naming the new sha:

```
517e116e -> d99b51fc  rebase (start): checkout main
d99b51fc -> 04487ef1  rebase (pick): ranger-base-cq3vi: check 4 reads …
```

What actually held is `git checkout -B <branch> 04487ef1` — the branch moved
AND HEAD re-attached, so a later rebase carries the branch with it. The
rebased commit is the right one to land: its tree is byte-identical to the
merge tree verified below, its parent is main, and it keeps the author email,
author date and subject that `equivalentOnBase` pairs on.

Filed as **ranger-base-wg4z1**: the remedy sentence should be `checkout -B`,
and whether posse should itself re-attach an *accidental* detach is a design
call, because `spliceDetachedWork` is gated on `launchedDetached` deliberately
— splicing without a record "would silence the guard for every session, which
is the one thing it must not do".

Verified before re-pointing the branch, because main had moved five commits
deep into `internal/posse` since that work was cut and a red landing there
costs every seat on the box: the merged tree materialized into this worktree
and `make test` green over all three arms (242.6s / 79.3s / 104.3s,
`tree-check` and `posse-land-check` included).

Said exactly: that run was over the merge onto main at `312f823c`, tree
`db147ea8`. What lands is the rebase onto `d99b51fc`, tree `e5124714`, and the
two differ by `docs/adr/0011`, one notes fragment and one index row — a
docs-only delta from a commit already green on main, with **no Go file
different between them**. So the suite result covers every source file that
lands, and the one generated file that combines rows from two landings was
checked on the landing tree itself: `notes-index.py --check` over
`e5124714`'s `docs/notes.d` is generator-exact.

## What not to conclude

Not "never detach a session worktree". The transcript census (518 transcripts,
16 detaching checkouts, 7 sessions) found the other six sessions doing it
safely, in two shapes. One detached its own tree to `main` and to a sha three
times, to run pins at another commit, and re-attached with
`git checkout posse/<branch>` every time. The rest detach *fixture* repos
under `mktemp -d`, which is nobody's session branch. A deliberate detach
paired with a re-attach before the next commit costs nothing, and a detach of
a throwaway repo costs nothing at all.

What cost something was an **unpaired** detach with no reason to be typed at
all, in the same command as unrelated work, under `-q`.

The cheap habit that catches it is `git status -sb`, whose first line is
`## HEAD (no branch)` on a detached tree against `## main` on an attached one.
Plain `--porcelain` will not: MEASURED in the same repo, it printed `?? c` in
both states, identical, because branch information is what `-b` adds. So a
close that reads porcelain for dirtiness — and `dirtyPaths` does — cannot see
this, and `-sb` is one character more.
