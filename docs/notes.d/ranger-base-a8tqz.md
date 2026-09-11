## `git status` writes the index, so the blocked tree never went quiet (ranger-base-a8tqz)

ci.yml went red on main at `26090db3` and stayed red. Three pins in
`internal/posse/mergeblocked_qa_test.go` failed on **ubuntu arm 2 only**,
under the full suite, and passed everywhere else — including on the same
commits on macOS, and including in isolation on the runner's own platform.
Two rounds of fixes (`ranger-base-yct7l` ×3, ending at `3580651c`) treated
the reds as a settle window measured too tight and widened it. They were
not a window problem.

### What the pins were actually reading

`ranger-base-9u5zy` removed the every-pass rebase probe so a stranded
worktree's git dir could stop being written and `posse worktrees --retire`
could finally take it — ADR 0058 fact 4, read through `lastTreeWrite`
(`retire.go`), which is the **newest mtime of every file in the session's
git dir**. `ranger-base-ejju3` then made `standingMergeBlock` re-read its
operands each pass, and `blockStillStands`' header claimed the three reads
it added write nothing, of `git status`:

> MEASURED writes nothing over a blocked tree in the steady state, dirt or
> no dirt

That was measured on an idle mac and it is false. `git status` is not
read-only: when the stat cache is racy — every file whose mtime is not
comfortably older than the index's, which is exactly what an **aborted
rebase** leaves behind — git re-hashes, finds the file clean, and writes the
refreshed index back. `dirtyPaths` (`worktree.go`) runs
`git status --porcelain` on the sweep's path, so that write landed in the
git dir on pass after pass, and `lastTreeWrite` read it as "just written".
That is `ranger-base-9u5zy`'s bug surviving its own fix through a second
writer.

### MEASURED 2026-09-10 · macOS 26.4.1 / APFS · git 2.50.1

Fixture: init, commit, branch, conflicting commit on main, `git rebase main`
then `git rebase --abort`. Then eight consecutive status calls over a tree
**nobody touched**, watching `.git/index`:

| trial | `git status --porcelain` | `git --no-optional-locks status --porcelain` |
|---|---|---|
| 1 | wrote passes 2–3, quiet from 4 | quiet from pass 2 — never wrote |
| 2 | **wrote every pass, 2–8 — never quiet** | quiet from pass 2 — never wrote |
| 3 | wrote passes 2–6, quiet from 7 | quiet from pass 2 — never wrote |

The settle is **not a window that can be widened**. Trial 2 never went
quiet at all, and a grace clock only reaches zero once the writing ends, so
no pass-count bound for it can be made correct. On the ubuntu runner under
the full suite the same unboundedness showed as writes through passes 4–5,
past the 3-pass window `3580651c` had just set — which is why the reds moved
between commits and between platforms rather than tracking any regression.

### The fix

`dirtyPaths` passes `--no-optional-locks` (git 2.19+, and the spelling the
repo already uses on the push gate). The flag skips sub-operations that take
a lock, which for `status` is precisely the index refresh. Output is
unchanged — git still computes the true status, it just does not cache what
it learned — so all eleven callers read the same answer and pay a re-hash
instead of a write. `retire.go:376` is one of them, so before this the
retire path's own dirt check reset the clock the retire was about to read.

The sweep-level pins are **left as they are**: they were right, the product
was wrong, and they are what caught it. What they could not do is catch it
on an idle box, so `TestDirtyPathsDoesNotWriteTheIndexOfABlockedTree` asks
the question underneath directly — `dirtyPaths` over an aborted-rebase tree
must not write the index, not once. Verified to fail on call 1 without the
flag and pass with it, on macOS, where every sweep-level pin passes either
way.
