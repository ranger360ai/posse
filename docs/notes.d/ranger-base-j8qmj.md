## The merge-back dedupe now reads CLOSED beads (ranger-base-j8qmj)

`noteMergeBlocked` deduped its handoff by title through a read that saw only
OPEN beads. For a branch whose content is already on `main` — superseded, or
re-landed by an earlier rescue — the correct outcome is to close the block
with a do-not-land verdict, merge-back being ff-only. So **the right answer
was the act that destroyed the key**, and the next sweep filed a
byte-identical P1 against the same untouched branch. `docs/notes.d/`
`ranger-base-4ri4n.md`, `ranger-base-avq12.md` and `ranger-base-gn0ch.md` are
three seats spent re-deriving one verdict.

### The change

- `Bd.AllLabeledAny` (`internal/posse/beads.go`) — `bd list --all
  --label-any <l> --json --limit 0`. Kept apart from `OpenLabeledAny` rather
  than merged with it: G3 and the closed-dirty handoff ask what is still
  *waiting*, and a closed row is answered; this asks what has already been
  *answered*, and the closed row is the whole point.
- `priorMergeBlocked` (`dispatch.go`) replaces `openMergeBlocked` in the
  dedupe. An OPEN row still wins outright; among closed rows the **latest**
  verdict answers, because a re-filed branch carries several.
- `workHeadTime` (`worktree.go`) — the committer date of `workHead`'s commit.

### Why "closed exists" is not the whole rule

`EnsureSessionTree` is idempotent by design — *"a relaunch, a resume, or a
second pass over the same bead lands in the tree that already exists"* — so a
reopened bead re-dispatched into its old tree commits onto the same branch. A
dedupe that stopped at "a closed bead exists" would swallow that handoff
forever, which is the first of the two questions the bug left open. So the
verdict stands only while **the branch has not moved since it was recorded**.

The comparison is sound because the ordering is causal, not lucky: the block
cannot be filed before the merge was attempted, the merge cannot precede the
commit it failed to land, and the close comes after the bead exists. The
committer date and not the author's, because a rebase, a cherry-pick or an
amend rewrites it — "the branch is the same branch" is the question, not "the
same patch was written".

The bug's second suggestion — a commit count in the title — was not taken.
The count moves the key for every branch at once, so the first pass after it
landed would re-file all fifteen; and a count cannot see a rebase that keeps
the number. The timestamp comparison answers the same question without
changing a key that is also a human-readable title.

### Measured against the live store, 2026-09-04

`bd list --all --label-any code --json --limit 0` → 800 rows in 0.22s (the
open-only read returns 123). 23 merge-back filings, 20 closed, **every closed
row carries `closed_at`**. Replaying the rule over each re-file — was there a
closed bead of the same title whose `closed_at` was after the branch's tip? —

| branch | tip | re-files | rule says |
|---|---|---|---|
| `…dinesh-…-ifiz5` | 09-03 21:28:56 | 17ui3 | suppressed |
| `…gilfoyle-…-nw9zg` | 09-02 16:49:05 | avq12, emgdb | suppressed |
| `…gwart-…-4ts30` | 09-02 16:20:12 | 77e3h | suppressed |
| `…jian-yang-…-9a53x` | 08-31 02:15:50 | gn0ch, qesej | suppressed |
| `…jian-yang-…-nr3eq` | 08-31 08:21:07 | rvtbb, ygp08 | suppressed |

**8 of 8 re-files suppressed, 0 first filings lost.** The tightest margin is
`nr3eq`: tip 08:21:07, verdict 08:23:22 — 2m15s, and on the right side
because it is that causal chain and not a coincidence of clocks.

### The fixture that was hiding it

The fake `bd`'s `list` served its files whole, ignoring `--all`, so an
open-only dedupe and a closed-aware one behaved identically against it — no
test in the suite could have failed on this bug. `fakeBdDropClosed`
(`herdr_test.go`) now implements real bd's own default. Measured: with the
old fake and the defect restored, `TestSweepDoesNotRefileABlockThatWasAlready`
`Closed` passes; with the new fake it fails on the fresh P1.

### What this does NOT stop

The bead only. The `posse worktrees` line for a superseded branch stays until
the operator retires the tree — `ranger-base-s0eo2` (nw9zg) and its siblings.
