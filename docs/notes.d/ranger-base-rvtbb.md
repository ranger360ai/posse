## Superseded, and landing it by hand reds the suite (ranger-base-rvtbb)

`docs/notes.d/ranger-base-tb90m.md` records the third merge-back outcome —
**another bead fixed the same red first, so the strand must NOT land** — and
its instance was one where landing the strand would have reverted a decision
*under a green suite*. This is the same class with the failure turned up: on
`posse/jian-yang-posse-ranger-base-nr3eq` the branch's one commit does not
merely duplicate `main`'s fix, it picks an escape hatch that a wall landing
**45 minutes before the strand was written** had already closed. Doing what
`noteMergeBlocked` literally prescribes — "resolve the conflict by rebasing
onto main by hand" — lands a **failing test**.

### The two fixes, and the 45 minutes between them

`TestBeadsVisibilityGuardHook` (internal/posse/visibility_test.go) has one arm
asserting that a file *outside* `.beads` is not the visibility guard's
business. It used to probe with `NOTES.md`. Once `InstallCommitGuardHook`
began carrying the ADR 0022 shared-index wall as well, that arm tripped the
`NOTES.md` guard in the test's own plain `git init` fixture — a wall with no
opinion on the thing being asserted. That is the red `ranger-base-nr3eq` was
opened for, and both fixes are a change of probe file:

| | commit | landed | probe |
|---|---|---|---|
| `main` | `643ef9b` (ranger-base-v88i) | 2026-08-31 07:36 | `notes.txt` |
| branch | `3365977` (ranger-base-nr3eq) | 2026-08-31 08:21 | `README.md` |

`643ef9b` is the commit that *added* ADR 0024 D2 checks 1+2, and it moved the
probe as part of its own work: check 2 scans the added lines of every staged
**markdown** file for `OpsPatterns`, and the fixture's line is deliberately
ops-class, so after D2 any `*.md` probe is refused for a second unrelated
reason. `notes.txt` is outside both walls. `README.md` is outside only the
first — it is markdown.

The branch was cut before 07:36, so its author could not see the second wall.
Nothing is wrong with either commit at its own base. `3bfea78`
(ranger-base-dmsbu) has since sharpened `main`'s comment; the probe file has
not moved.

### The wrong arm, which here is cheap

Reading 4 of tb90m — *the pin the strand existed for is green at `main`, with
a wrong arm* — is one command, because this test reads its subject through the
compiler and an overlay is therefore a real control (contrast tb90m, whose
subject was read from disk at run time):

```sh
sed 's/"notes\.txt"/"README.md"/g' internal/posse/visibility_test.go > /tmp/m.go
printf '{"Replace":{"%s/internal/posse/visibility_test.go":"/tmp/m.go"}}\n' "$PWD" > /tmp/o.json
go test ./internal/posse/ -run TestBeadsVisibilityGuardHook -count=1              # ok
go test -overlay /tmp/o.json ./internal/posse/ -run TestBeadsVisibilityGuardHook -count=1
#   refused by posse gate: … matched in the staged markdown additions … FAIL
```

The refusal names both `OpsPatterns` rules the fixture line trips. So the
strand's content is not "already on `main` in a different spelling" — it is
content `main` **must not take**, and the block's own instruction is what
would take it.

### The reading that costs nothing, again

Per-path content, the 4ri4n recipe, over the branch's single touched path:

```sh
B=posse/jian-yang-posse-ranger-base-nr3eq P=internal/posse/visibility_test.go
comm -23 <(git show $B:$P | sort -u) <(git show main:$P | sort -u)
```

Seventeen branch-only lines, and every one of them is either the strand's own
`README.md` hunk or a line from the branch's **base** that `main` has since
rewritten — the file is 639 lines on the branch and 1261 on `main`. The strand
itself is `1 file changed, 7 insertions(+), 3 deletions(-)`.

`TestBeadsVisibilityGuardHook` passes at `main` (`89add6a`). Nothing is owed.
The branch is untouched and holds one obsolete commit; retiring the tree is
the operator's.

### The one-line rule

A merge-back block whose branch predates a **new wall** is not the ordinary
supersession: the branch's fix is not stale, it is *invalidated*, and the
instruments say the same word (`STRAND`) for both. Before hand-rebasing any
block, run the pin the strand existed for at `main` **with the strand's own
content overlaid**. Green-at-main says the deliverable is met; the overlay arm
says whether landing it would take it back.

### Third pass, and the exit this branch was missing (ranger-base-ygp08)

2026-09-04. The same block was filed a **third** time against the same
untouched branch (`ranger-base-c3xtt` 08-31 → `ranger-base-rvtbb` 09-03 →
`ranger-base-ygp08` 09-04), because `noteMergeBlocked` dedupes over OPEN beads
only (`ranger-base-j8qmj`) and a do-not-land verdict is closed by definition.
Re-confirmed at `main` `e88366a`, both arms, in ~10s:

```
control    go test ./internal/posse/ -run 'TestBeadsVisibilityGuardHook$' -count=1   ok 5.6s
wrong arm  same, with the strand's notes.txt->README.md probe overlaid              FAIL 4.8s
```

The wrong arm still names both OpsPatterns rules the fixture's deliberately
ops-class line trips — the `cost` and `guard` classes, quoted in the test
fixture itself and not restated here. Per-path content is unchanged from the
rvtbb reading — 17 branch-only lines, 9 of them the strand's own hunk
and 8 base lines `main` rewrote; the file is 639 on the branch against 1263 on
`main` now. `git cherry main <B>` says `+` and no `-x` trailer for `3365977`
exists on `main`, so `equivalentOnBase`/`measuredOnBase` have nothing to read
and never will: the strand was superseded by an independent fix, never picked.

**The lesson is not about the verdict, it is about the exit.** Three sibling
branches in exactly this state each carry an `OPERATOR: retire ...` question
bead (`ranger-base-s0ih6` 4ts30, `ranger-base-enaij` 9a53x, `ranger-base-s0eo2`
nw9zg); this one carried none, which is the whole reason a settled verdict was
re-derived twice. So the second close of a merge-back block is not "comment the
verdict again" — it is **file the operator ask**, because the tree is the only
thing that stops the loop and no persona can remove it (`git worktree remove`
is denied by the auto-mode classifier, and `RemoveSessionTree` counts commits,
not content). Filed for this branch as `ranger-base-4nco1`.
