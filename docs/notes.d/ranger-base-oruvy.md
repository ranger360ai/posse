## A merge-back block overtaken while it was measuring (ranger-base-oruvy)

`merge-back blocked: posse/gwart-posse-ranger-base-g7br6 does not land on main`
is the third link in one chain — `hlv21` → `g7br6` → this — and all three are
the same collision in `internal/posse/herdr_test.go`. **Verdict: PARTIAL.**
One of `b3c0266`'s two hunks is a new file and lands; the other is half on
`main` already, and it got there *during* `g7br6`'s own verification run.

That is the fact worth keeping. `docs/notes.d/ranger-base-d91mf.md` names three
residual shapes (empty, superseded, partial) and `ranger-base-g7br6.md` adds a
fourth about the branch (retired). This one is about the **clock**: the window a
seat spends proving its diff is longer than the interval in which `main` moves
under the line it is editing, so "nobody else touched it" is a reading with a
shelf life measured in minutes.

### The overtake, in committer dates on `main`

| time | commit | bead | what |
|---|---|---|---|
| 09:10:47 | `c08e3fb` | yqfxo | `g7br6`'s base — `main`'s tip when the seat was cut |
| **09:25:01** | **`46d9ec3`** | **m4730** | lands directly on `c08e3fb`; repairs the **second** name on `dispatch_qa_test.go:1089` |
| 09:37:46 | `b3c0266` | g7br6 | commits the stranded work; repairs **both** names on that same line |

14m14s between the base and the overtake. `g7br6` read `main` at `c08e3fb`,
where its table row *"nobody else touched it"* was true, and wrote the whole
line — so the rebase conflicts on a line where the two seats agree about one
name and only one of them says anything about the other.

The measurement that makes this structural rather than unlucky. `internal/posse`
is the package all three beads verify: `g7br6` clocked it at **1121.423s**, this
block at **844.399s**. `main` took **64 commits between 00:33:13 and 10:20:19**
today — median gap **346s**, mean 559s — and **47 of those 63 gaps are shorter
than the faster of the two runs**, 52 of 63 shorter than the slower. A seat that
verifies this package in full should expect `main` to have moved once or twice
under it, and should re-read `main` at commit time rather than trust the reading
it took at cut time.

This note measured it on itself: `main` gained three commits (`effe9e0`,
`4240a5d`, `374d3b8`) during the 844s run above. None of them touches these
paths, so this one replays clean — but that is luck, and the check is one
command, not an assumption:

```sh
git log <cut-point>..main -- <the paths you are about to commit>
```

### Unlike `g7br6`, the branch is still here

`ranger-base-g7br6.md`'s fourth shape was a block whose branch had already been
deleted, so the bead's boilerplate — *"The branch is untouched and still holds
every commit"* — was false at dispatch. Here it is true:

```sh
git for-each-ref --contains b3c0266   # refs/heads/posse/gwart-posse-ranger-base-g7br6
git worktree list | grep g7br6        # the worktree is still mounted
```

So `d91mf`'s one-command recipe runs with a branch name, and the reading below
is `git diff main posse/gwart-posse-ranger-base-g7br6`. Do not assume either
way: check the ref, the shelf life is real in both directions.

### The two hunks

| hunk | verdict | why |
|---|---|---|
| `docs/notes.d/ranger-base-g7br6.md` (new file, 157 lines on the branch) | **re-land** | `main` has no such path, so nothing conflicts. It is the record of the `hlv21` verdict and `main` never got it. Landed with two claims marked in place, below. |
| `dispatch_qa_test.go:1089` citation | **re-land partially** | `46d9ec3` already fixed `fakeBdDropClosed` → `fakeBdReadyDropClosed`. Still dangling on `main`: `fakeBdNoteClosed`. |

`fakeBdNoteClosed` has never been a function. Before this commit,
`git log -S fakeBdNoteClosed --all` returned exactly two — `3075168`, which
wrote the citation and added `fakeBdShownStatus` in the same commit, and the
stranded `b3c0266`, which removed the name and never reached `main`. So `main`
carried it one bead and two merge-back blocks later, and the third commit that
list names is this one.

Re-landed here is the name plus the paragraph that says why, with the
**attribution split**: `m4730` fixed one name, this bead fixes the other. The
version on `b3c0266` credits both to `g7br6`, which was true when it was written
and is not true on `main`, so it could not be replayed verbatim.

Two claims in `ranger-base-g7br6.md` were overtaken the same way and are marked
in place rather than rewritten — the note is that session's record, and what it
knew at its base is part of what this note is about.

### Verified

Comment-only in the Go file; the rest is one new note and two marked claims in
another. Each exit code read off the command itself, never off a pipe
(`docs/notes.d/ranger-base-fmift.md`).

```
go build ./...                                  exit 0
go vet ./...                                    exit 0
go test ./internal/posse -count=1 -timeout 25m  exit 0, ok 844.399s
```

`internal/posse` needs `-timeout 25m`; `go test`'s default is 10m and a FAIL
near 601s is the default firing, not the diff (`ranger-base-hlv21`).
