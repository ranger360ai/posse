## The replay cannot run over a detached HEAD, and the remedy that handed it one (ranger-base-wg4z1)

From ranger-base-3iybl, which recovered a session whose only commit was on no
branch, ran the remedy `landed()` printed, and watched the pass after it strand
the work again. Two defects, one root: **`git rebase <base>` moves whatever
HEAD names**, and over a detached session tree that is HEAD and not the branch.

### What the replay did

`MergeSessionWork` replays a branch the base has moved under by running
`git rebase <base>` in the session's own tree, then fast-forwarding the base
onto the BRANCH. Over a detached tree those are two different refs: the rebase
moves HEAD and leaves the branch where it was, so the replay's output is on no
ref at all while the fast-forward takes a branch the rebase never touched and
whose parent is no longer the base. The pass then prints

```
main still would not fast-forward after the rebase: … Not possible to fast-forward
```

about work it had *just* replayed onto main — one unreferenced commit per pass,
no landing ever, and the merge-back block re-filing against each new sha.

MEASURED 2026-11-14, this box, git 2.56.0, one fixture both ways
(`scratchpad/probe-detached-rebase.sh`, reproduced as the test below):

| tree HEAD | what the rebase says | branch afterwards | `merge --ff-only` |
|---|---|---|---|
| detached at the work | `Successfully rebased and updated detached HEAD` | still the pre-rebase commit | `fatal: Not possible to fast-forward` |
| attached to the branch | `Successfully rebased and updated refs/heads/posse/s1` | the replay's output | fast-forward, landed |

### It reached BOTH kinds of detach, and the designed one was the surprise

The accidental detach gets there the moment the off-branch refusal's own cure is
run: `git branch -f <branch> HEAD` moves the branch and leaves HEAD detached, so
it satisfies the off-branch test for exactly one pass and hands the replay the
state in row 1. That is ranger-base-3iybl's whole second half.

The **designed** detach gets there with nothing typed by hand at all.
`spliceDetachedWork` puts the branch on a container-tier session's work and
deliberately leaves HEAD detached (`PrepareSessionHead`, ranger-base-t4f1) —
which is row 1. So a caged session landed only while its base had not moved,
and every one whose base moved stranded. MEASURED by the four-of-five arms that
go red with the fix removed: the designed-detach arm is one of them, and it had
no coverage before this bead because every existing fixture for that tier lands
by plain fast-forward.

### The fix: attach before the replay, refuse when that is not honest

`attachForReplay` (worktree.go), asked after the dirt and before the first
attempt, because an attach is a fact about the tree and not about an attempt.

It attaches **only when the branch already names the exact commit the tree's
HEAD names** — no ref moves, no commit changes hands, nothing reachable stops
being reachable. That is what keeps ranger-base-oou58's invariant: putting work
back on a branch the work is NOT on is the *splice*, it is gated on
`launchedDetached` on purpose so an accidental detach stays loud, and this
function never does it. When the two tips hold different work the replay is
refused and the sentence says why. So the bead's option (b) — "should posse
re-attach an accidental detach itself" — is answered without touching the
guard: nothing here is spliced, and the sentence an accidental detach gets is
still `landed()`'s.

It cannot discard a persona's uncommitted work, by two independent facts: the
dirt check has already returned above it (`o.Dirty`), and the checkout names the
commit HEAD is **already on** and passes no `--force`, so git moves no file and
would refuse rather than overwrite one. A blind dirt read — `dirtyPaths` reports
nothing when git will not answer — therefore still cannot cost anything here.

Re-attaching a tree posse detached on purpose costs nothing *here*, and that is
a fact about where this runs: every caller holds the launcher lock, the sweep
merges only a tree whose bead is already closed, the next launch into that tree
detaches it again from the record, and a session that could still commit in its
cage would have had its HEAD rewritten by the replay regardless. What the
attach buys downstream is the rest of ADR 0087 working as written — both
`resetReplay` and `baseLandCheck` refuse over a detached tree and land
unjudged.

### The remedy sentence, in one place

`offBranchCure` is now the single prescription the four surfaces print —
`landed()`'s two refusals, `removalTips`' tail and the listing's
`appendOffBranch`. It reads `git -C <tree> checkout -B <branch> HEAD`, which
does both halves: the ref move `branch -f` did, and the attach that lets the
replay behind it carry the branch.

**Except for a tree posse detached on purpose**, which keeps `branch -f`: its
HEAD must stay off the branch to commit inside its cage, prescribing an attach
to the operator of a live caged session would take that away, and the ref move
is the whole of the cure there — it is the one posse runs itself.

### Not closed here

Nothing. The design question the bead flagged for the architecture lane — should
posse re-attach an *accidental* detach itself — is answered above by not needing
it: the attach runs only where the branch already names the work, so no splice
happens and the guard stays exactly as loud as it was.

### Tests

- `TestTheReplayCarriesTheBranchOverADetachedSessionTree` (worktree_test.go,
  arm 2) — five arms: the cure as printed, the ref move alone (which must land
  too, or the fix rests on a human having typed the better command), the
  designed detach over a moved base, the on-branch control, and the refusal.
  Four of the five are red with the fix removed; the control stays green.
  Its orphan assertion is asked of the TREE's HEAD and not the branch — `main`
  must reach the commit the tree is actually on — because a landing that
  reported `Merged` while the replay's output sat on no ref is exactly what
  this bead measured.
- `mergeBlockedCases` gains the refusal's row (arm
  `DETACHED HEAD, which \`git rebase\` moves instead of`), so
  `TestQAEveryMergeRefusalSentenceHasACase` keeps its two-way census total: 17
  sentences, 17 cases.
- The `branch -f` assertions in the detached family now split by the stamp —
  `checkout -B` on every accidental arm, `branch -f` on every caged one —
  across worktree_test.go, landdetachedgate_test.go, landdetached_qa_test.go,
  retiredetached_qa_test.go and retiresweep_qa_test.go.
