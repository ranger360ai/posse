## A merge-back block whose commit main already superseded (ranger-base-tb90m)

The merge-back handoff (`noteMergeBlocked`, dispatch.go) says the branch
"still holds the work" and asks the closer to resolve the conflict. Two
outcomes were already on record: re-land the strand on your own branch
(ranger-base-0qiny, the picked-by-hand shape), or find the block was
spurious. This is the third, and it is the one where doing what the
handoff literally says makes `main` worse:

**another bead fixed the same red first, and the strand must NOT land.**

The instance. `3a604c1` on `posse/dinesh-posse-ranger-base-ifiz5` turned the
two `PARKED (laurie, …)` headers in `internal/posse/queueremote_qa_test.go`
into `PARKED (qa, …)` — the last red left at `main`, ADR 0012 App.A 5. While
that tree was running the suite, `45b0c3c` (ranger-base-xvtur) landed on
`main` for the same red and dropped the parenthetical name **entirely**:
`PARKED (ranger-base-vtyst verifying …)`. Same two lines, so the replay
conflicts; main's side is a strict superset of the branch's, so there is
nothing left to pick.

### Telling "superseded" from "stranded", in readings that cost nothing

A strand and a re-land under another sha look the same in `posse worktrees`
(`equivalentOnBase`, worktree.go), and neither of them is this. Four
readings separate all three:

1. **It really is stranded**, not already landed under another sha:
   `git merge-base --is-ancestor <sha> main` false while the same on
   `<sha>^` is true (a plain strand, and it dates the branch point);
   `git cherry main <branch>` marks it `+`; the patch-ids differ.
2. **The content is on main anyway.** `git diff <sha>:<path> main:<path>`
   is the whole question in one command. Here it prints only the word the
   branch added and main removed — i.e. main's side subsumes the branch's,
   and a pick would revert `main`, not advance it.
3. **Name the commit that did it**, so the close cites a landing and not a
   feeling: `git log -S'<string the strand removed>' --oneline main -- <path>`
   → `45b0c3c`. `git log --grep <bead-id> main` does not answer this — the
   one hit for ranger-base-ifiz5 is `7fba3e4`, which cites it as *filed, not
   fixed*.
4. **The pin the strand existed for is green at `main` — with a wrong arm.**
   Green alone measures nothing.

### The wrong arm here cannot be an overlay

`TestShippedTreeNamesRolesNotThisCrew` (instancebound_qa_test.go) reads the
shipped tree from **disk at run time**. `go test -overlay` swaps files at
*build* time, so the pre-fix content went in through an overlay and the test
came back `ok` — a wrong arm that proves nothing, and reads exactly like a
pass. The honest arm writes the pre-fix bytes to the real path:

```sh
git hash-object internal/posse/queueremote_qa_test.go        # before
git show 85139cd:internal/posse/queueremote_qa_test.go > internal/posse/queueremote_qa_test.go
go test ./internal/posse/ -run TestShippedTreeNamesRolesNotThisCrew -count=1
#   --- FAIL … the shipped tree names the originating instance (2 hits):
#         internal/posse/queueremote_qa_test.go:8: laurie   (and :49)
git checkout -- internal/posse/queueremote_qa_test.go
git hash-object internal/posse/queueremote_qa_test.go        # equal to before
```

Re-hashing is the restore's own proof, and it belongs in the same call as
the mutation. The general rule: **an overlay arm is only a control for a
test that reads its subject through the compiler.** A test that walks the
repo needs the bytes on disk.

Note what the suite would *not* have caught: `qa` is a role, not a crew
name, so the pin passes on either side. Landing the strand would have
reverted a landed decision under a green suite.

### "Rebasing onto main by hand" always means your own branch

Under `RHQ_CAGE=seatbelt` a dispatched seat can write **only its own tree**.
Measured from the ranger-base-tb90m worktree, one create per arm:

| target | result |
|---|---|
| `$REPO/.git/worktrees/<my session>/` | writable |
| `$REPO/.git/worktrees/<other session>/` | EPERM |
| the other session's working tree | EPERM |
| the shared checkout, and its `.git/` | EPERM |

So `git -C <blocked tree> rebase main` dies before it touches anything —
`fatal: update_ref failed for ref 'ORIG_HEAD': … Operation not permitted` —
and it leaves no `MERGE_HEAD`, no partial state, nothing to clean up. This
is not a gap: the crew's actual practice never edits the old branch. You
pick into the branch you were dispatched onto and let the launcher
fast-forward that (ranger-base-0qiny). The old tree is retired by the
operator afterwards — `git worktree remove …` plus `git branch -D …` — and
the same EPERM is why ranger-base-0qiny had to remove `CHERRY_PICK_HEAD` by
hand.

### Leaving a superseded branch standing is safe

Merge-back is `--ff-only`. A branch that conflicts cannot fast-forward, so
`posse worktrees --land` reports it and stops; `--force` only widens the
skip for a branch naming **no** bead, and this one names its bead
(`branch.<branch>.posseBead`). The stale branch costs a line in
`posse worktrees` and can never silently regress `main`. The bead is where
the "do not land this" lives, because git cannot hold it.
