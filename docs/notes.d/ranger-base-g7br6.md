## A merge-back block whose branch was already gone (ranger-base-g7br6)

`merge-back blocked: posse/gwart-posse-ranger-base-hlv21 does not land on main`
names a real rebase conflict, and the conflict is a DUPLICATE of a rename that
reached `main` 39m38s earlier under a different bead. **Verdict: DO-NOT-LAND.**
Residual is the PARTIAL shape `docs/notes.d/ranger-base-d91mf.md` names — one
line `main` never got, re-landed here.

Two things make it worth its own note. The branch was **already deleted** when
this block was worked, which is a residual shape none of the earlier notes has,
and two of the branch's three hunks are do-not-land because `main`'s prose is
*better* than the branch's, not merely equal to it.

### The branch is gone, so d91mf's one-command recipe does not run

`docs/notes.d/ranger-base-d91mf.md` answers a block with

```sh
git diff main <branch-tip> -- <paths>
```

Here there is no branch tip. `posse/gwart-posse-ranger-base-hlv21` is not in
`git branch -a`, its worktree under `~/.posse/worktrees/posse/` does not
exist, and `.git/worktrees/` has no admin dir for it — the tree was
retired between the filing and the dispatch. The bead's own text still says
*"The branch is untouched and still holds every commit."* At dispatch that
sentence was false.

The commit survives anyway, and the block itself is where you get it: the
conflict message quotes the sha.

```
error: could not apply 3f4fd53... internal/posse test build: two sessions added …
```

`3f4fd53` is a real object, and every reading below is `git diff main 3f4fd53`
with a raw sha where d91mf had a branch name. But it is **unreferenced**, not
merely unmerged:

```sh
git for-each-ref --contains 3f4fd53   # prints nothing
git name-rev --name-only 3f4fd53      # undefined
git reflog --all | grep -c 3f4fd53    # 0
git fsck --unreachable --no-reflogs | grep -c 3f4fd53…   # 1
```

No ref and no reflog holds it, so it is gc-eligible under git's default
two-week `gc.pruneExpire`. **A merge-back block on a retired branch has a
shelf life**: work it before a `gc` runs, or the only copy of the evidence is
the diff nobody can regenerate. Nothing on this box currently runs `gc --prune`
on a schedule, which is the only reason a filing from 08:11 was still
answerable.

### What is on main, and how it got there

| time (committer) | commit | bead | what it did |
|---|---|---|---|
| 05:51:26 | `c3ab918` | j8qmj | added the **list**-side `fakeBdDropClosed` |
| 06:25:36 | `3075168` | y3x6n | added a **ready**-side filter, same name — collision |
| 07:21:34 | `0c2609f` | — | hlv21's base: both declarations present, package unbuildable |
| 07:29:12 | `455d344` | pju9t | renamed the ready-side one `fakeBdReadyDropClosed` |
| 07:58:52 | `5b4e686` | 5im1q | cut before `455d344`; deleted the list-side copy as superseded |
| **08:09:46** | **`3f4fd53`** | **hlv21** | **the stranded commit — the same rename, arrived at independently** |
| 08:20:17 | `6ecb521` | jzoci | restored the list-side copy `5b4e686` removed |
| 08:40:59 | `17ddee3` | d91mf | re-landed tenf5's comment lines `6ecb521` did not carry |

hlv21 was cut at 07:21:34, eight minutes before `455d344` existed, and chose
the same new name in the same place. That is why the rebase conflicts and why
the conflict is empty of content: two seats wrote one rename.

### The residual, in one command

```sh
git diff main 3f4fd53 -- internal/posse/herdr_test.go internal/posse/dispatch_qa_test.go \
  | grep -E '^[+-]' | grep -vE '^(\+\+\+|---)' | grep -vE '^[+-]\s*//'
```

prints **nothing**. Every differing line on both files is a comment: the
function bodies, both call sites (`herdr_test.go:184` on the ready filter,
`:245` on the list filter) and the names are byte-identical to `main`'s. So
none of hlv21's *code* is missing, and rebasing the branch would mean
hand-resolving a duplicate-rename conflict for comments.

Of the three comment hunks, two are do-not-land and one is the residual:

| hunk | verdict | why |
|---|---|---|
| `herdr_test.go:527` list-side header | **do-not-land** | `main`'s version (`455d344` + `17ddee3`) says the same thing and more — NAMED PAIR, the `5b4e686`/`6ecb521` history, that deleting either unbuilds the package. Landing hlv21's would *delete* that. |
| `herdr_test.go:796` ready-side header | **do-not-land** | same: `main` carries hlv21's argument (the fake-show.json END-state reading) *and* the 11-line TWO-functions block hlv21 does not have. |
| `dispatch_qa_test.go:1089` citation | **re-land** | nobody else had touched it at this branch's base. This is the whole residual. *(It stopped being true 14 minutes later — see `docs/notes.d/ranger-base-oruvy.md`.)* |

That first column is the part `equivalentOnBase` could not have told anyone
even if it read hunks: "the branch's version of this comment is worse" is not
a shape a landing check has a verdict for. It is a read, and it is why a
partial residual is re-landed by hand rather than by replaying the branch.

### The residual line, and the second name on it

`dispatch_qa_test.go:1089` explains why `TestQARefill…` collapses `PromptGrace`,
and cites the pair of helpers that made the fake honest:

```go
// (fakeBdNoteClosed/fakeBdDropClosed, herdr_test.go). With the fake honest,
```

`3f4fd53` corrected the second name. Both are wrong:

- **`fakeBdNoteClosed` has never existed.** `git log -S` finds it in exactly one
  commit, `3075168` — the commit that wrote this comment. The same commit added
  `fakeBdShownStatus` (what `show` currently answers, by id) and the ready
  filter. It was a mis-citation on arrival, and it outlived three beads.
- **`fakeBdDropClosed`** is a real function, which is worse: since `455d344` it
  names `list`'s filter, and this paragraph is about `ready`. A reader
  following the citation lands on the wrong helper and reads a correct comment
  about the wrong question.

Both are corrected in this commit, and the correction is stated in the comment
rather than only in this note. Only the **first** of them reached `main` this
way: `46d9ec3` (ranger-base-m4730) landed the second name's repair while this
block was still measuring, so this commit conflicted on its own line and was
itself blocked — `docs/notes.d/ranger-base-oruvy.md` is that block, and is what
finally carried both this file and the `fakeBdNoteClosed` half onto `main`.

A dangling name in a doc comment is not cosmetic in this file —
`docs/notes.d/ranger-base-d91mf.md` traces `5b4e686`'s deletion of
the wrong copy to exactly that: `455d344` renamed a function and left its old
name standing in its own header, so the file carried two comments claiming one
name and the next seat believed the wrong one.

### For the next reader

`docs/notes.d/ranger-base-d91mf.md` lists three residual shapes (empty,
superseded, partial). This block adds a fact orthogonal to all three, about the
*branch* rather than the diff:

4. **retired** — the branch and its worktree no longer exist. The recipe still
   runs with the sha from the block's own conflict text, but the object is
   unreferenced and on a gc clock, and the bead's boilerplate ("the branch is
   untouched") will assert otherwise. Check `git for-each-ref --contains <sha>`
   before believing it.

The block stays closed do-not-land. `git cherry main 3f4fd53` prints `+` and
always will — the patch-id is not on `main` — so the verdict is what has to
carry, and `c3ab918` (ranger-base-j8qmj) reads closed blocks in the dedupe. If
it re-files, read `docs/notes.d/ranger-base-8nsc6.md` for the binary-ancestry
order before re-deriving any of this.

### Verified

Comment-only diff in test files, so the bar is that the package still builds
for tests and the pins these helpers carry still pass. Each exit code read off
the command itself, never off a pipe:

```
go build ./...                                  exit 0
go vet ./...                                    exit 0
go test ./internal/posse -count=1 -timeout 20m  exit 0, ok 12.378s
  -run the 8 pins: the five y3x6n/refill arms + j8qmj's three dedupe arms
go test ./internal/posse -count=1 -timeout 25m  exit 0, ok 1121.423s (full)
```

`internal/posse` needs `-timeout 25m`, not `go test`'s 10m default; a FAIL at
~601s is the default timeout firing, not the diff (`ranger-base-hlv21`).
