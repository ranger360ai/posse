# ranger-base-emgdb — a rebase landing leaves no evidence either of the two equivalence arms can read

`merge-back blocked: posse/gilfoyle-posse-ranger-base-nw9zg does not land on
main`, P1, filed by the landsweep against gilfoyle. **The premise was false.
Every commit on that branch was already on main, and had been for two days.**

## What was actually on main

The branch is 5 commits ahead of main by sha. All five have a counterpart on
main with the same subject and the same author date, and main's copies were
committed together at `2026-09-02 22:15:47 -0400` — one launcher rebase.

| branch | main | patch-id | why it drifted |
|---|---|---|---|
| `34a27b4` (ranger-base-vd1bo) | `6a230eb` | **differs** | main scrubbed the Slack token fixtures: `xoxb-1111111111-…` → `xoxb-AAAAAAAAAAAA-…` (4 literals) |
| `c663a9a` | `d889cc7` | same | — |
| `54ed42b` | `4ddf15d` | **differs** | the `.PHONY` line absorbed a sibling bead's `verify-parallel` |
| `51ca0a7` | `fc835aa` | same | — |
| `df434d8` | `55d0e8e` | **differs** | same `.PHONY` line |

The rebase in the bead's reason text agrees: it skipped `c663a9a` and
`51ca0a7` as *previously applied* — the two patch-id twins — and conflicted
on the first of the three that had drifted. **The conflict was main's own
later work, not unlanded work.**

Whole-tree check: `git diff main <branch>` is 0-added on every file the
branch's commits touched. Main holds strictly more.

## Why the detector could not see it

`equivalentOnBase` (internal/posse/worktree.go) knew two ways a commit's work
reaches the base under another sha:

1. **patch-id**, via `git cherry` — what a clean non-interactive rebase drops.
2. **git's `-x` trailer** — the only record a *cherry-pick* resolved by hand
   leaves.

A **rebase** resolved by hand leaves neither: the resolution amends the patch
so the patch-id drifts, and a rebase never writes an `-x` trailer. Both arms
miss, the pairing comes back `nil`, and MergeSessionWork emits a strand
report word for word identical to a real one.

ADR 0051 (ranger-base-xzg0x) measured **48 of 134 landings — 36% — as rebases**, so this
is the common case, not the corner. Cost per occurrence: one P1 and one seat.

## The fix

A third pairing kind: the commit's own identity — **author, AUTHOR date,
subject** — the three fields a rebase carries through unchanged. Bounded to
`base --not tip`, the same bound the trailer lookup already uses, and indexed
once per call rather than one `git log` per commit.

Guarded two ways:

- **Ambiguity is not a pairing.** A key two commits on the base share maps to
  `""`, which reads as unaccounted-for. Base rate measured before relying on
  it: **1163 commits on main, 1163 distinct keys, zero collisions.**
- **It can never license a delete.** The pairing is filed with
  `byPatch: false`, and `measuredOnBase` — the question every destructive
  caller asks (autoreap.go:609, RemoveSessionTree, unaccountedFor, treeState)
  — is all-or-nothing on `byPatch`. So the *reporting* half stops filing P1s
  and the *destructive* half is exactly as strict as it was. That matters
  here concretely: `34a27b4`/`6a230eb` is a pair whose resolution did **not**
  keep every byte, and a branch paired that way must still be read by a human
  before it is thrown away.

The three sentences that describe the unmeasured half (`RemoveSessionTree`'s
refusal, `unaccountedFor`, `treeState`) used to say "-x trailer" and "recorded
as landed in" unconditionally. Over a replay pairing there is no record of
anything, so the evidence phrase is now chosen from the kinds actually
present — the same overstatement class ranger-base-x8jp and ranger-base-hk02
each removed once.

## Pins — `internal/posse/mergebackreplay_test.go`

Four arms, each with a mutant that kills it and only it (`go test -overlay`):

| mutant | killed |
|---|---|
| `D` the replay arm removed (pre-fix behaviour) | `…PairsACommitReplayedOntoTheBaseByRebase`, `…NeverLicensesADelete` |
| `A2` key drops the author date on both sides (pair on author+subject) | `…DoesNotPairOnTheSubjectAlone` |
| `C` an ambiguous key keeps the first commit seen | `…DoesNotPairAnAmbiguousReplayKey` |
| `B` the pairing is filed as a trailer, not a replay | `…NeverLicensesADelete`, `…PairsACommitReplayed…` |

An earlier mutant `A` changed the lookup key without the index key, so it
merely broke the arm and killed the wrong tests — a misaimed probe, re-aimed
as `A2` above before it was believed.

## Verified against the live branch

In-package probe against `~/src/posse`, `main` vs
`posse/gilfoyle-posse-ranger-base-nw9zg`:

```
commits ahead by sha: 5
measuredOnBase (delete licence): false
  df434d8a4c37 as 55d0e8e39b95 (same author, author date and subject)
  51ca0a776fd8 as an equivalent patch on main
  54ed42ba2ea4 as 4ddf15d92506 (same author, author date and subject)
  c663a9ab2064 as an equivalent patch on main
  34a27b4d85f3 as 6a230ebcf234 (same author, author date and subject)
→ 5 commit(s) … are already on main under other sha(s) (…) — nothing here is unlanded
```

Same five pairings this note derived by hand. The sweep now prints `≡ nothing
here is unlanded` for that branch instead of filing a P1, and the branch is
kept.

**Not yet in force on this box.** The installed `posse` binary still carries
the old `equivalentOnBase`; promoting it is the operator's act, not this
bead's.

## Fleet census with the fix in

Every `posse/*` branch in `~/src/posse` (68 refs, 15 ahead of main by sha),
run through the fixed `equivalentOnBase`:

| branch | ahead | before | after |
|---|---|---|---|
| `gilfoyle-…-nw9zg` | 5 | strand | **paired — replay** (this bead, P1 `ranger-base-emgdb`) |
| `dinesh-…-uzgkz` | 3 | strand | **paired — replay** (open P1 `ranger-base-8nsc6`) |
| `dinesh-…-5jdzh`, `…-olwk`, `…-zwk8`, `gilfoyle-…-4myz`, `gwart-…-zag6`, `holden-…-pqc4v`, `jian-yang-…-aupee` | 1–4 | paired | unchanged |
| `dinesh-…-ifiz5`, `…-vbp3`, `gwart-…-4ts30`, `…-vxbfm`, `jian-yang-…-9a53x`, `…-nr3eq` | 1–4 | strand | strand — **real, and correctly still reported** |

So the arm clears **2 of the 4 open merge-back-blocked P1s** and moves none
of the six genuine strands. `posse/gwart-…-4ts30`, whose operator ask
(`ranger-base-s0ih6`) says "no patch-id or `-x` trailer will ever account for
it", is still not accounted for — the new arm does not weaken that claim.
`delete-licence` (`measuredOnBase`) is unchanged on every branch listed.

## What this bead does NOT do

`ranger-base-s0eo2` ("OPERATOR: retire the superseded worktree
posse/gilfoyle-posse-ranger-base-nw9zg … the only exit from a P1 re-filed
every pass") predates this. Its premise is now wrong — there is a second
exit, and it is the code above rather than a hand retirement per branch — but
**retiring a worktree and promoting a `posse` binary are both the operator's
acts**, so this bead does neither. Until the promote, the sweep on this box
still runs the old `equivalentOnBase` and will re-file.
