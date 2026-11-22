# A park leaves the held commits on every branch already cut from that tip (ranger-base-zlvyk)

MEASURED 2026-11-22 in `~/src/posse`, linux 7.2.15-1-omarchy, git 2.56.0.
Every reading here is read-only: `git merge-base --is-ancestor`,
`git for-each-ref`, `git rev-list`, and `git merge-tree --write-tree`, which
writes a tree to the object store and moves no ref. Nothing of another seat's
was touched, and the one live carrier named below was not modified — its
branch is checked out by a live seat, so its ref cannot be moved at all.

## What a park does and what it leaves

A push hold parks the held commits on a `refs/heads/parked/<name>` ref and
resets the base branch out from under them. That removes ONE ref's claim on
those commits. It does not touch any other ref — and a session branch is cut
from the base's tip at dispatch, so every seat dispatched before the park is
still pointing straight at the held commits.

Nothing enumerated those refs. ranger-base-8h17w's hold named two carrying
branches by hand; there were three, and the third was found by a ten-line
census in seconds. That census is versioned now:
`scripts/park-carriers.sh`, 2.7s over seven parks and 26 refs on this box.

THE CARRYING SET IS CLOSED AT PARK TIME, which is what makes the census cheap
and complete rather than a sample: a seat dispatched AFTER the park cuts from
the reset base and carries nothing. So the one minute after a park is when the
census is worth the most, and its answer then is the whole answer.

## The fast-forward failing is not the stop

IT HAS LANDED ONCE ALREADY, by the simplest of these routes. Recorded on
ranger-base-8h17w by the push gate the same evening: after the reset, the
launcher fast-forwarded the CLOSED carrier seat's branch onto local main
(reflog `merge posse/laurie-posse-ranger-base-repzh`) and the held commits
came straight back. The gate refused them a second time, on the content scan
and on the patch-id both, and the stale worktree and branch were removed. So
the carrier that lands is not a shape anyone has to imagine — it is one
observed landing, from one of three carriers, within an hour of the park.

That one went through the fast-forward, which needs the base not to have
moved. The rest of this section is about the arm behind it, which does not.

The hold's own reasoning was that if the base moves on before a carrier
closes, the launcher's fast-forward of the base onto that branch FAILS, and
that is the safe order. The first half is right and the second does not
follow. `MergeSessionWork` (internal/posse/worktree.go) does not stop at the
fast-forward: behind it is `equivalentOnBase`, and behind THAT is the replay —
`git rebase <base>` in the session's tree with the fast-forward after it as a
compare-and-swap. A base that moved is the case the replay exists for.

MEASURED, with `main` at 82b4adc2, the park at `parked/repzh-a50aa688` holding
2333ca8a + a50aa688, and one live carrier:

| probe | answer |
|---|---|
| `git merge-base --is-ancestor main <carrier>` | exit 1 — the fast-forward refuses |
| `git merge-tree --write-tree main a50aa688` | exit 0 — the replay is CLEAN |

So the replay lands the held pair on the base under new shas. `equivalentOnBase`
does not catch it either: the sanitized re-commit is ONE commit against the
held TWO, and its author date differs from the first held commit's by 38
minutes, so the (author email, author date, subject) triple pairs neither.

WHAT MAKES THE ORDER SAFE IS CONTENT, NOT ANCESTRY. Once the sanitized commit
is on the base, the same replay adds a file the base already has with
different text: `git merge-tree --write-tree <sanitized> 2333ca8a` is exit 1,
`CONFLICT (add/add)`. The launcher stops on that and files a merge-back bead a
human reads. That conflict is the stop — the refused fast-forward never was.

## Still not an exposure, and this is why

The replayed commit's patch-id is BYTE-IDENTICAL to the held one:
`git diff --no-ext-diff main <the merged tree> | git patch-id --stable` over
the replay of 2333ca8a gives `304bcb249a06e8e8bc517b3071a9df1ae509d0c5`, which
is the patch-id the push gate's hold list already refuses. `patch-id --stable`
reads the hunks and not their line numbers, so moving an added line to a
different position in a generated index does not change it.

That is the reading that keeps this P2 rather than P1: a carrier that lands
costs a second park, a second hold bead and a revert on local main. It does
not cost a publication. A gate resting on patch-ids alone would be a different
matter — a replay that had to RESOLVE anything changes the diff and so changes
the id — and this one did not have to.

## A carrier is a ref a landing can read

`MergeSessionWork` lands `refs/heads/<the session branch>` and reads no other
namespace, so only a branch outside the park namespace can put held commits
back on the base. Two other classes reach them and neither can:

- ANOTHER PARK. A park cut from a tip that already carried an earlier park's
  commits reaches them; both are held, and that containment is the point.
  Two of the seven parks here are in this relation.
- POSSE'S OWN RECORDS, `refs/posse/*`. What a `refs/posse/retired/*` ref does
  is keep the objects reachable, so gc will not prune held content the park's
  own ref no longer names. MEASURED: one retired ref here reaches the commits
  TWO parks hold, which is what retiring a branch whose tip was parked leaves
  behind. The census prints both classes and counts neither.

A third shape is a carrier and reads like neither: a session branch whose
WORKTREE directory is gone but whose REF survives. posse prunes the stale
registration and then runs `git worktree add PATH BRANCH` when the branch
exists (`-b PATH BASE` only when it does not), so a re-dispatch into that bead
lands the new seat on the stale branch's TIP rather than on the base. Measured
hermetically under ranger-base-8h17w; the census covers it because it reads
`refs/heads` and not the worktree registrations.

## What was decided, and what was not

The devops call this bead was filed for — should the census run at park time —
is YES, and as a script rather than as a posse verb: parking is a hand
procedure at the push gate, and posse should not grow a verb for a step that
runs once a hold. `scripts/park-carriers.sh` is that script, with a
`--self-test` that builds the hold's exact shape in a scratch repo and proves
the census says YES with a carrier and NO without one.

The census is the detective half and it is advice a human has to run. The
enforcement half is a guard in the landing chain, which is code and is filed
as ranger-base-3s2ig with the two decisions it has to make: what a park IS
(`refs/heads/parked/*` is a convention nothing pins; a namespace posse owns
would not be), and what the refusal costs when the carrier is a live seat
whose ref cannot be moved for hours.

## Not closed here

- ranger-base-3s2ig
- ranger-base-8h17w
