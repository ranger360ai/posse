# ranger-base-dpvlh — a merge-back of a DERIVED gate has to re-run the gate

Landed 2026-10-04, git 2.50.1 / macOS 26.4.1. The replay mechanics are
docs/notes.d/ranger-base-xea2y.md; this is the part that file does not cover.

`ranger-base-g6sb1` closed with one commit on its branch, `main` moved on, the
launcher's rebase conflicted and aborted, and this bead was filed. Replayed
with `git merge-tree --write-tree` per xea2y — no rebase, no cherry-pick, no
sequencer state. Two things about THIS conflict are worth keeping.

## The textual conflict was the .PHONY line, and the union is the resolution

`ranger-base-1a0hi` inserted `treepins` into `.PHONY`; `g6sb1` inserted six
door names into the same line. git cannot union one line, so it conflicts, and
both additions are wanted. This will recur on every merge-back whose branch
adds a Makefile target, because this Makefile keeps every target on ONE
`.PHONY` line — the same shape as the notes index's single newest-month
heading, and the same resolution: take both.

What auto-merged is the part worth checking. 1a0hi appended a `treepins:`
target immediately after `tree-check:` while g6sb1 rewrote `tree-check`'s
comment and prerequisite list immediately above it — adjacent edits to one
region, which is exactly where a textually clean auto-merge can still drop a
hunk. The reading that is evidence costs one command: compare the two diffs'
+/- line SETS, the branch against its base and the worktree against main.

```
diff <(git diff --no-ext-diff <base> <work> | grep '^[+-]' | grep -v '^[+-][+-]') \
     <(git diff --no-ext-diff main      | grep '^[+-]' | grep -v '^[+-][+-]')
```

Empty but for the line you resolved by hand means every line the work added is
present and nothing else is. Here it printed exactly the `.PHONY` pair.

## A gate whose MEMBERSHIP is derived can be redded by the other side

g6sb1's whole subject is a register that derives the tree-wide pin class
mechanically — `internal/treepins`' arm 2b finds every test enumerating a
relative tree directory and requires each to sit behind a Makefile door. A
gate like that is not a file two branches might both edit; its input is the
WHOLE package, so the other side of the merge can add a member of the class
without touching a line the merge sees.

1a0hi did add seven tests to `internal/treepins/suitelock_qa_test.go` while
g6sb1 sat blocked. None of the seven enumerates, so nothing redded — but the
merge could not have told anyone that. **Re-run the derived gate on the merged
tree; a clean merge is not an answer about it.** MEASURED here: `make
tree-check` 61.9s green, register arm 23.1s, and `make test` whole green,
three arms (arm 1 `internal/treepins` 317.0s / `internal/posse` 273.1s, arm 2
239.1s, arm 3 250.4s), `go run ./cmd/checkorphans` clean.

The class is "a merge-back whose branch adds a gate that derives its own
membership", and the cost of skipping the re-run lands on the next seat, as a
red package nobody's diff explains.
