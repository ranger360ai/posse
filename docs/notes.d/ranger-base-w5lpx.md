## A merge-back block that is superseded in one file and stranded in the other (ranger-base-w5lpx)

[ranger-base-tb90m](ranger-base-tb90m.md) recorded the third outcome of a
merge-back block: main had already fixed the same red, main's side was a
superset, and landing the strand would have made main worse. This is the
fourth, and it is the one that punishes deciding the question **per
commit**:

**the strand is superseded in one file and genuinely missing in the other.**

The instance. `d25b5ec` on `posse/holden-posse-ranger-base-pqc4v` is a
single commit adding 183 lines to two files:

| file | claim | state on main |
|---|---|---|
| `internal/posse/revertquoting_qa_test.go` | `TestQAGuardRefusalNamesEveryPathGitQuotes`, `t.Skip`-**parked** on ranger-base-qg0k8 | superseded — un-parked and rewritten by `5531283` (qg0k8's own fix), then widened by `8121bd1`/`39e9d7b` (pp7k1) and `4936e73` (2d4f4) |
| `internal/posse/worktree_test.go` | `TestListSessionTreesWillNotCallAHalfLandedOrSquashedBranchLanded` | **absent** — main has no pin for either partial shape |

Reading the commit as one unit gives two wrong answers. "It is superseded,
drop it" loses the only pin main has for a half-landed branch. "It is
stranded, pick it" re-parks a test that passes at main and deletes three
later beads' work in the same file. The replay conflict the handoff
reported is entirely the first file; the second applies with
`git apply -3` clean.

### Price the block per file, and read the deliverable, not the sha

`git show --stat <sha>` names the files; then, for each one:

```sh
git cat-file -e main:<path>                      # does it even exist there?
git log --oneline <merge-base>..main -- <path>   # who touched it since the branch point
git diff <sha> main -- <path>                    # which side is the superset
```

Here the last command showed main's copy of `revertquoting_qa_test.go`
*deleting* the branch's `t.Skip` line and *adding* two whole tests — main's
side is a strict superset, exactly the tb90m shape — while
`git cat-file -e main:` plus a `grep` for the function name showed the
`worktree_test.go` half absent from main entirely. `git cherry` cannot
answer this: it is per commit, and this commit is both things at once.

Cheap confirmation that the surviving half still fits: main's helpers
(`wtApp`, `wtRepo`, `commitIn`, `mustGit`, `write`) and the
`ListSessionTrees(&out, []string{repo})` call shape had not drifted, even
though ranger-base-pj87l and ranger-base-aupee both rewrote that file for
parallelism in between. Check the signatures the stranded test calls before
concluding a clean `apply -3` means anything.

### The strand's own stated wrong arm was not a discriminating one

`d25b5ec`'s message says both new arms "die under
`case true || measuredOnBase(eq)`". True, and it measures nothing: that
mutant makes every branch read `nothing unlanded`, so the three pins main
already had (`TestListSessionTreesNamesWhatHasNotLanded`,
`…NamesWhichBeadTheUnlandedWorkIsFor`,
`…TellsACherryPickedBranchFromAStrand`) fail with it. A wrong arm that
kills the incumbents too cannot show the new pin covers anything new.
One mutant per arm does:

| mutant | site | new arm that dies | pre-existing pins |
|---|---|---|---|
| account for what you can, ignore the rest — `return nil` → `continue` in the unaccounted-commit branch | `worktree.go:1105` | `half the commits picked` | all 5 survive |
| per-commit accounting failed, fall back to CONTENT: every path the branch changed already matches the base | `worktree.go:1105`, before the `return nil` | `squashed onto the base` | all 5 survive |

Both are widenings somebody would plausibly write — the first is "be
lenient about the commits you cannot pair", the second is "the content is
obviously there". Each makes the listing invite the operator to delete a
branch that is the last copy of real work, and each is caught by exactly
one of the two new arms. That is the pair of readings that says the
re-land is worth landing.

A false start worth recording: the content mutant was first written
*before* the pairing loop, and it then killed
`a clean cherry-pick reads as nothing unlanded` as well — not on the
verdict, which was still `nothing unlanded`, but on the **note text**,
which that pin asserts verbatim. Moving the widening to the loop's failure
path made it discriminating. When a mutant kills an incumbent, check
whether it changed the answer or only the wording before believing the
overlap.

### A day-old stranded test reds a guard in another package

The re-landed half compiled, ran and passed in `internal/posse`, and the
only red it produced was in the **root** package:

```
--- FAIL: TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared
    the unmutated copy is not clean (exit 1):
    testparallel: 1 tests in … can take t.Parallel and do not:
      worktree_test.go  TestListSessionTreesWillNotCallAHalfLandedOrSquashedBranchLanded
```

`d25b5ec` was written 2026-09-01; ranger-base-i7fa and ranger-base-pj87l
then made `t.Parallel()` mandatory for every eligible test in that package
and pinned the rule from the root package. A faithful pick is therefore
*not* the finished job — the mechanical half has to be re-derived under
the policy that landed in between (the pj87l rule). Here that is one line,
`t.Parallel()` first in the function body, matching the three sibling
`TestListSessionTrees*` pins; the subtests take nothing, because `wtApp`
and `wtRepo` are keyed on `t.Name()`.

So run the **root** package as well as the one you edited before believing
a re-land is clean. It also pays for itself: with `t.Parallel()` the
`-run TestListSessionTrees` set went 15.8s → 2.7s, and the two mutant arms
above were re-measured at that price.
