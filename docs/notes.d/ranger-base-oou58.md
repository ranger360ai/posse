# A two-part guard with one part pinned (ranger-base-oou58)

MEASURED 2026-11-09, this tree at main 34b91d53, git 2.50.1, darwin 25.6.0
arm64.

The verify of four closes — ranger-base-p60mv, ranger-base-0z9ta,
ranger-base-h2uhe, ranger-base-36yvn. All four hold on outcome. This page is
for the one thing the verification found that the closes did not: half of
ranger-base-p60mv's predicate was held by no test in the repo.

## The finding

ranger-base-p60mv made `existingTree` (internal/posse/worktree.go) accept one
off-branch HEAD — the detach posse itself recorded — and keep refusing every
other. The accept is a conjunction:

    if launchedDetached(t.Repo, t.Branch) {
        if _, detached := treeDetachedHead(t.Path); detached {
            return true, nil
        }
    }

`launchedDetached` says posse detached this tree once. `treeDetachedHead` says
it is detached NOW. Dropping the second and keeping the first —

    if launchedDetached(t.Repo, t.Branch) { return true, nil }

— left **the whole of arm 2 green**: 452.888s, zero FAIL, the mutant in place
for the entire run (md5 checked before and after). The two mutations the
close itself ran both red: reverting the accept outright reds p60mv's own pin
and ranger-base-36yvn's porcelain pin, and dropping `launchedDetached` while
keeping `treeDetachedHead` reds p60mv's accidental-detach arm. Neither
isolates the second conjunct.

Why nothing caught it: every fixture that sets the record also leaves the tree
genuinely detached. The three passes of 36yvn's relaunch loop that hand
`EnsureSessionTree` a detached HEAD (passes 1, 3 and 4 — PATH 2 leaves the
tree detached, and the uncaged arm re-attaches and clears the record, so the
loop alternates) all have both halves true; p60mv's accidental arm has the
record false and the tree detached. The state with the record TRUE and the
tree NOT detached — a HEAD sitting on some third branch — is created by no
test: `recordDetached(..., true)` appears in exactly two test files
(worktree_test.go, landdetachedgate_test.go) and both pair it with a real
`checkout --detach`, and nothing in the repo ever checks a session tree out
onto another branch.

That state is what the guard is FOR. `existingTree`'s own header promises the
refusal for "a tree sitting on some third branch", and
`spliceDetachedWork`'s names "a stale record on a reused branch" as a case
posse does not control. Accepting it would hand the launch a tree whose
session branch is not what git would write to — the ranger-base-dybv failure.

## The pin

`TestEnsureSessionTreeRefusesAStaleRecordOverATreeOnAnotherBranch`
(internal/posse/worktree_test.go, arm 2). The caged prepare sets the record,
then the tree is checked out onto a branch of its own, with both premises
asserted as controls rather than assumed. MEASURED: PASS on shipped code,
FAIL under the dropped-conjunct mutant with "a stale detach record let a tree
on another branch through", and GREEN under the other two mutants — those make
the guard STRICTER, so the pin is orthogonal to p60mv's arms rather than a
second copy of them. The three pins together now cover both conjuncts.

The finding is a coverage gap, not a defect: the code on main is correct.

## What else was measured, and holds

- `make test` on main at 34b91d53: exit 0, zero FAIL. Arm 1 (internal/posse
  562.4s, internal/treepins 559.3s), arm 2 329.6s, arm 3 387.2s, silent-revert
  audit clean over 2,426 commits. ranger-base-h2uhe is titled "make test is red
  on main"; it is not red any more. `make tree-check` green, every door,
  corpus-check included.
- The hash-spelling door still works after the two `hshPending` rows were
  deleted, which is the thing a deletion can quietly break. Seven planted arms
  plus a pristine control, each run against only the census: a bare hasher is
  reported by path and line; a one-armed probe is still a finding; a legitimate
  pending row suppresses a real finding; **a STALE pending row reds with "THE
  FIX LANDED"** — the mechanism that caught uxadf, re-proved after the rows it
  caught were removed; a row over a path that hashes nothing reds; the same
  path in both maps reds; a junk reason reds.
- Both scripts the deleted rows named carry a real three-arm probe
  (`sha256sum` -> `shasum` -> die/exit 2) with a known-digest self-check and a
  64-hex validator, so the deletion was substantively right and not just
  bookkeeping. The census cannot see that third arm — it says so — which is why
  it was read by hand.

## The strand

ranger-base-h2uhe's own commit a569c1d6 is NOT on main and conflicts with it,
because ranger-base-0z9ta landed the same deletion first (59e9106f). The
outcome is on main; the commit is not. Already filed as ranger-base-rwlmu.
What that branch carries and main does not is prose only: a sentence telling
the next row to check `git merge-base --is-ancestor <the commit the row cites>
main` before adding it, and an 85-line notes fragment. 0z9ta's own amendment
to docs/notes.d/ranger-base-uxadf.md covers the history; the ancestry sentence
has no home on main yet.

ranger-base-p60mv landed the same way and is fine: the close names 304a5d31,
main carries f9009a83, and the two are patch-identical — same `git patch-id`,
differing only in blob hashes and hunk offsets from the different base.
