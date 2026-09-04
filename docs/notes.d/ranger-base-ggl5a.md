## A superseded branch decided by a compiler, not by a line count (ranger-base-ggl5a)

`docs/notes.d/ranger-base-tb90m.md` lists three outcomes for a merge-back
block; `ranger-base-4ri4n` added a fourth, **superseded — do not land**, and
`ranger-base-avq12` showed the verdict is monotone. All three of those are the
same branch, `posse/gilfoyle-posse-ranger-base-nw9zg`, read three times.

This is the class on a **different** branch and a **first** filing, so nothing
here is a re-file: `posse/gwart-posse-ranger-base-4ts30`, two commits, and no
earlier block bead against it. Two things are new. The verdict is reachable by
one command instead of a line-by-line reading, and the reason this branch can
never be retired by machine is not nw9zg's reason.

### The instance

Branch `posse/gwart-posse-ranger-base-4ts30`, base `fc994d2`, two commits, and
`main` holds neither by sha:

| branch commit | what it is | how `main` got it |
|---|---|---|
| `34a27b4` (ranger-base-vd1bo) | the branch's inherited **base** commit, another bead's | `6a230eb`, 2026-09-02 14:15 |
| `a07bc2d` (ranger-base-4ts30) | the deliverable: nine example PID digests | `a5a7cbc` (ranger-base-e6dxv), 2026-09-02 19:13 |

The deliverable was landed by somebody else. `ranger-base-e6dxv` found the
same red while verifying `ranger-base-9ztcy` and fixed it three hours after
`a07bc2d` was committed — independently, not as a pick. Two seats on one
defect, and only one of the two fixes ever reached `main`. All nine digest
**values** `a07bc2d` appends are on `main` today, and `899548b`
(ranger-base-oujxl) has since appended nine more for `b0eae4d`.

### The reading that costs one command: build the branch against `main`

4ri4n and avq12 reached the verdict by comparing lines. For a branch whose
files are Go, there is a cheaper and stricter question — **does `main`'s tree
still compile with this branch's files in it?** — and `go build -overlay`
answers it without touching the working tree or the branch:

```sh
B=posse/gwart-posse-ranger-base-4ts30
O=$(mktemp -d); printf '{"Replace":{' > "$O/ov.json"; sep=
for p in $(git diff --name-only "$(git merge-base main "$B")" "$B"); do
    case "$p" in *.go) ;; *) continue ;; esac
    d="$O/$(echo "$p" | tr / _)"; git show "$B:$p" > "$d"
    printf '%s"%s":"%s"' "$sep" "$p" "$d" >> "$O/ov.json"; sep=,
done
printf '}}' >> "$O/ov.json"
go build -overlay "$O/ov.json" ./...
go vet   -overlay "$O/ov.json" ./internal/posse/
```

Measured 2026-09-04 at `main` `0e78fae`. `go build` exits **0 with zero bytes
of output** — the branch's non-test code is self-consistent, which is why
nothing cheaper than this catches it. `go vet`, which type-checks the test
files too, exits **1**:

```
vet: internal/posse/memorydiffconfig_qa_test.go:280:14: undefined: diffHeaderPath
```

`main`'s `memorydiffconfig_qa_test.go` (ranger-base-y7i7k) calls
`diffHeaderPath`; the branch's `memoryland.go` predates it and does not define
it. **The package's test binary does not build.**

Control arm, because a recipe that only ever prints an error measures nothing:
the same script with `B=main` exits 0 and prints zero bytes, as does a plain
`go vet ./internal/posse/`. So it passes on a branch `main` holds and names a
symbol on one it does not. That is the whole verdict, and no reader has to
weigh lines.

### The line reading, for the record

Restricted to the three paths in `<merge-base>..<branch>`: **23** non-blank
lines the branch holds that `main` does not, against **114** it would remove.
Every one of the 23 is a line `main` deliberately replaced, by a named commit:

| branch-only lines | superseded by |
|---|---|
| 9 digest lines — same values as `main`, older comment text | `899548b` ranger-base-oujxl, 09-03 |
| 2 comment lines spelling `rhq/personas`, `rhq/agents` | `b7da69a` ranger-base-x9ds9, 09-02 |
| 8 lines of the old `+++ b/`-only diff-header scan | `6e9dfc1` ranger-base-y7i7k, 09-02 |
| 4 digit-shaped Slack fixtures (`xoxb-1111111111-…`) | `6a230eb` ranger-base-vd1bo re-land, 09-02 |

The third row is the one that costs something: y7i7k replaced that scan
*because* `+++ b/` alone misses a C-quoted path. The fourth is a scrub — the
seat that re-landed vd1bo re-spelled the digit-form fixtures with `A`s, and
landing this branch puts the digit form back.

### Its own deliverable, landed today, re-reds the pins it was filed to green

The sharpest measurement, and the one that makes "superseded" concrete.
Overlay **only** `internal/posse/exampledigests.go` from the branch onto
`main` and run the two pins `ranger-base-4ts30` exists to green:

```
--- FAIL: TestEveryEmbeddedExamplePIDIsInTheShippedTable (0.00s)
--- FAIL: TestShippedExampleTableCoversEveryVersionInGitHistory (1.92s)
```

Nine files named, both pins, eighteen findings — the identical shape 4ts30 was
filed against, because the branch's table predates `899548b` and so lacks
`b0eae4d`'s nine PID versions. Unoverlaid, both pins and vd1bo's three
credential pins pass on `main` (`ok … 2.927s`). A branch whose deliverable
now *causes* the defect it was written to fix is not a strand.

### Why this tree can never be retired by machine, and it is not nw9zg's reason

`equivalentOnBase` (`internal/posse/worktree.go:1080`) accounts for a commit
two ways: a patch-id match, or git's `-x` trailer. Measured here:

```
$ git cherry main posse/gwart-posse-ranger-base-4ts30
+ 34a27b4d85f3e08de5780c3c928b6b3b46252071
+ a07bc2d587f5a775e811f8d7a599a3b769376f8e
```

`+` for both, and a `--grep='cherry picked from commit <sha>'` over
`main --not <branch>` finds nothing for either. So `equivalentOnBase` returns
nil, every automated reader sees a real strand, and `RemoveSessionTree`
refuses without `--force`.

nw9zg is unaccountable because one commit in `base..tip` belonged to another
bead and was correctly never picked. **This branch is unaccountable because
neither commit was ever picked at all.** `a07bc2d` was superseded by an
independent fix with different comment text, and `34a27b4` by a re-land that
scrubbed a fixture. Both of those were the right call by the seat that made
them, and both moved the patch-id. Generalised: **a re-land that improves the
patch — a reworded comment, a scrubbed fixture, a hand-resolved hunk — is
exactly what makes the original branch permanently unaccountable.** There is
no trailer to find and there never will be, so no amount of re-reading moves
this branch out of the reaper's sight. The exit is the operator's.

The tree is clean (`git status --porcelain` empty, HEAD `a07bc2d`) and holds
zero lines `main` wants, so removing it loses nothing. Filed for the operator
as **ranger-base-s0ih6** with the exact commands and the zero-line blast
radius, beside `ranger-base-s0eo2` for nw9zg. `ranger-base-j8qmj` (the
closed-aware dedupe in `noteMergeBlocked`) is the systemic fix; until it
lands, this branch re-files a P1 every pass exactly as nw9zg did.

### Second pass: the exit was already filed, and the loop fired anyway (ranger-base-77e3h)

2026-09-04. The same block was filed a second time against the same untouched
branch (`ranger-base-ggl5a` → `ranger-base-77e3h`), exactly as this note
predicted it would. Re-confirmed at `main` `f3ad166`, **15 commits** after the
first reading at `0e78fae`. The branch is untouched (`a07bc2d`, tree clean),
and every arm above reproduces unchanged:

| arm | first pass (`0e78fae`) | this pass (`f3ad166`) |
|---|---|---|
| `go build -overlay` (branch .go onto main) | rc 0, zero bytes | rc 0, zero bytes |
| `go vet -overlay ./internal/posse/` | rc 1, `undefined: diffHeaderPath` | rc 1, same symbol, same line 280 |
| control, `B=main` | rc 0 / rc 0, zero bytes | rc 0 / rc 0, zero bytes |
| both pins with only `exampledigests.go` overlaid | FAIL, nine files | FAIL, nine files, both pins named |
| both pins unoverlaid at `main` | ok | ok 2.769s |
| `git cherry main <B>` · `-x` trailers on `main` | `+ +` · none | `+ +` · none |

`ranger-base-avq12` claimed the verdict is monotone in `main`'s advance. That
was one branch read twice; this is a second branch, 15 commits of drift, and
the same six answers. Nothing here is worth re-deriving a third time.

**What is new is about the exit, and it bounds `ranger-base-ygp08`'s lesson.**
ygp08 concluded that the second close of a merge-back block is not "comment the
verdict again", it is *file the operator ask*. This branch's ask was already
filed — `ranger-base-s0ih6`, 00:41; `ggl5a` closed 00:51 — and the block
re-filed at **03:56**, three hours later, with the exit open the whole time.
So filing the ask is **necessary and not sufficient**: nothing in
`noteMergeBlocked` reads it, and the only two things that end the loop are the
operator removing the tree or `ranger-base-j8qmj` landing. The ask is what
makes the loop legible to a human, not what stops it.

And do **not** raise the ask's priority to match the P1 it would silence. An
operator bead sitting in `bd ready` is dispatchable to a persona — status,
assignee and label do not gate readiness, only a blocking dep or `--defer` do
(ranger-base-6y83) — so promoting `s0ih6` from P2 buys a session burned on a
bead no persona can execute. It sits at #13 in `bd ready` today. Leave it.

Census, `main` `f3ad166`, over all 1921 beads: **23 filings across 15
branches, 8 of them re-files on 5 branches** (`nr3eq`, `9a53x`, `nw9zg` three
each; `4ts30`, `ifiz5` two). `j8qmj` was filed on "four branches carry two
each" one day earlier; it is now five, three of them at three. Four
`OPERATOR: retire` asks, all P2 and all open. The one repeat-branch with no
ask is `dinesh-posse-ranger-base-ifiz5`; `ranger-base-17ui3` is live on it, so
this pass names it rather than filing into that lane.

One trap while counting: `bd list --all --limit 300` returns 302 rows and
silently drops all four retire asks. The census above is at `--limit 5000`,
which returns 1921 and is therefore complete.
