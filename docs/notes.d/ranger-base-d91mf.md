## A branch whose fix landed under another bead, minus two comment lines (ranger-base-d91mf)

`merge-back blocked: posse/jian-yang-posse-ranger-base-tenf5 does not land on
main` names a real rebase conflict, and the conflict is a DUPLICATE: the
function that commit restores was already restored on `main` 5m39s later
(`0837f2b` 08:14:38, `6ecb521` 08:20:17) under a different bead. **Verdict: DO-NOT-LAND the branch.** But this is
not the clean do-not-land of `docs/notes.d/ranger-base-fmift.md` — the branch
carried eight lines `main` never got, and they are re-landed here.

### The accounting, in one command

The branch is one commit, `0837f2b`, touching one file. Diffing `main`'s tree
against the branch's tree is therefore the whole question:

```sh
git diff main 0837f2b -- internal/posse/herdr_test.go
```

It prints exactly two hunks, both comment-only. The 22 restored lines of
`fakeBdDropClosed` do **not** appear, which is the proof that `6ecb521`
(ranger-base-jzoci) put back byte-for-byte what `0837f2b` put back — both were
reverse-applications of `5b4e686`'s own deletion hunk, so the bytes and the
placement agree and the diff is empty there.

The residual, and why it is worth a commit rather than a shrug:

| line | what `main` had | what the branch had |
|---|---|---|
| `:800` | `// fakeBdDropClosed is bd's own first contract on ready` | the name corrected to `fakeBdReadyDropClosed` |
| `:530` | nothing | six lines saying this is one of a NAMED PAIR and that deleting either leaves the other's call site undefined |

Those are the two comments the third seat read before it deleted the wrong
copy. `455d344` renamed the ready-side helper and left the old name standing in
its own doc comment, so the file carried two comments claiming one name;
`5b4e686` then removed the definition that the surviving comment appeared to
describe. `6ecb521` restored the function and said the comment "is now true
again", which is true of the pair comment at `:820` and not of the header at
`:800` — that one still names a function that has not existed since `455d344`.

### The shape, for the next reader

A merge-back block where the fix landed under another bead has three possible
residuals, and only the diff tells you which:

1. **empty** — hand re-land, close do-not-land (`ranger-base-fmift`).
2. **superseded** — replaying would red the suite, close do-not-land and say
   which commit won (`ranger-base-tenf5`'s own finding on `3365977`).
3. **partial** — the shape here. Two seats fixed one defect and each carried a
   piece the other did not. The branch stays do-not-land, because rebasing it
   means resolving a duplicate-restore conflict by hand for eight lines that
   apply cleanly on their own.

`git diff main <branch-tip> -- <paths>` answers all three in one reading, and
it is the reading `equivalentOnBase` cannot do: its three arms
(`docs/notes.d/ranger-base-fmift.md`) all key on whole commits, and none of
them can report "most of it landed."

### Verified

Comment-only diff, so the bar is that the package still builds and the two
pins these helpers carry still pass. Measured at this commit, each exit code
read off the command itself and not off a pipe:

```
go build ./...                                                  exit 0
go vet ./...                                                    exit 0
go test ./internal/posse -run TestHerdr|<j8qmj + y3x6n pins>     exit 0, ok 5.608s
```

The full `internal/posse` package was run green at `925.683s` on `0837f2b`,
whose tree differs from this one by these comments alone.

### The branch, afterwards

`posse/jian-yang-posse-ranger-base-tenf5` still exists and still does not
fast-forward, and nothing here changes that — `git cherry` will keep printing
`+` for `0837f2b` because its patch-id is not on `main` and never will be.
Closing this block is what makes the verdict stand: `c3ab918`
(ranger-base-j8qmj) reads closed blocks in the dedupe, so the sweep should not
re-file unless the branch tip moves. If it re-files anyway, the branch moved or
the filing binary predates `c3ab918` — check those two before re-deriving the
table above (`docs/notes.d/ranger-base-8nsc6.md` has the binary-ancestry
reading order).
