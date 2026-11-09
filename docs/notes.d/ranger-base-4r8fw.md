# ranger-base-4r8fw — the merge-blocked replay is a LOOP, one commit per original

Landed 2026-11-08, git 2.50.1 (Apple Git-155) / macOS 25.6.0, go 1.26.10.

`mergeBlockedReplay` (internal/posse/dispatch.go) is the HOW every seat
dispatched on a merge-blocked bead reads, and until this bead it was written
throughout for a branch with **one** commit: one merged tree computed for the
whole branch (`git merge-tree --write-tree <base> <sha>`), one `cat-file blob`
loop over `git diff --name-only <base>...<sha>`, and *"keep **that commit's**
author identity, AUTHOR date and subject"* in the singular.

Followed literally on a three-commit branch it produces **one** commit, which
carries **one** original's `replayKey`. That is exactly what its own last
sentence warns about — for the other two.

## Why that re-files the block forever

`blockStillStands` (internal/posse/landsweep.go) ends a block on
`len(equivalentOnBase(t.Repo, t.Base, head)) > 0`. `equivalentOnBase`
(internal/posse/worktree.go) walks `rev-list base..tip` **one commit at a
time** and `return nil`s on the first commit it cannot account for. Of its
three arms, a hand-resolved replay leaves only the third:

- `git cherry` says `+`, because the resolution changed the patch;
- there is no `(cherry picked from commit …)` trailer, because a replay is not
  a pick;
- so `replayKey` — **(author email, AUTHOR date, subject)** — is the only
  thing that can pair the seat's commit with the stranded original.

One commit for *n* originals pairs one and strands *n-1*. A partial pairing is
not a pairing (`TestMergeBackPairsNothingWhenOnlySomeOfABranchLanded`), so the
answer is `nil`, the block stands on every later pass **forever**, and the
work is fully on `main` the whole time.

Nobody hit it before because every previous entry of this class replayed a
single commit: `ranger-base-xea2y` (the worked example the copy cites),
`-3e68k`, `-h1q5o`, `-v3tt4`, `-yrag8`. `ranger-base-83nrw` was the first
multi-commit one, and it resolved the three commits of
`posse/gwart-posse-ranger-base-8kwa5` by hand, one at a time, reproducing all
three `replayKey`s byte-for-byte.

## The recipe, per commit

```
# the originals, OLDEST FIRST — replay them in this order, one commit each:
git rev-list --reverse <base>..<sha>
```

Then for each original `C` in that order:

```
TREE=$(git merge-tree --write-tree --merge-base=$C^ HEAD $C | head -1)
git diff --diff-filter=D --name-only HEAD $TREE     # must be empty: the loop only writes
git diff --name-only $C^ $C                         # the paths to replay
git cat-file blob "$TREE:<p>" > <p>                 # one per path
# resolve the markers, then ONE commit carrying C's own identity:
GIT_AUTHOR_NAME=$(git log -1 --format=%an $C) \
GIT_AUTHOR_EMAIL=$(git log -1 --format=%ae $C) \
  git commit --date="$(git log -1 --format=%aI $C)" \
    -m "$(git log -1 --format=%s $C)" -- <your paths>
```

And read the whole **set** back before stopping — one missing key is one
stranded commit, and the block that re-files does not say which:

```
diff <(git log --format='%ae%x1f%aI%x1f%s' <base>..<sha> | sort) \
     <(git log --format='%ae%x1f%aI%x1f%s' <base>..HEAD  | sort)
```

`--date` is enough and `--amend` would not be: amend ignores
`GIT_AUTHOR_DATE` (AGENTS.md says so for the shared checkout; it is the same
function here).

## The merge base WALKS THE ORIGINALS, and this is the part that bites

`--merge-base=$C^`, not the branch's own base. "Ours" is **the replay so
far**, so for original *n* the base between ours and theirs is original
*n-1* — which `$C^` names without a special case, since for original 1 it is
where the branch was cut.

It does not merely produce a worse conflict. **MEASURED 2026-11-08**, by
planting `--merge-base=main` in
`TestMergeBackPairsEveryCommitOfATwoCommitBranchReplayedOneByOne`'s loop over
a two-commit fixture (`shared.mk` holds `PHONY := build test`; the branch
takes it to `… test a` then `… test a b`; `main` takes it to `… test verify-x`):

| merge base | original 1's merged `shared.mk` |
|---|---|
| `$C^` (correct) | **CONFLICT** — ours `… verify-x`, theirs `… a` |
| `main` | **CLEAN**: `PHONY := build test a` |

Against `main` as the base, only *theirs* changed that line, so the merge
takes theirs whole and **main's own `verify-x` is silently gone**. A clean
tree that ate the base's work is the shape `ranger-base-83nrw` met and called
*"the one that nearly ate main's work"* — there, taking `ef54685`'s version of
a file wholesale compiled, looked clean, and dropped main's hunks two hundred
lines above the region in conflict. Two rules come out of it, and they are the
same rule:

- **"Take theirs" is a resolution for a conflicted REGION and never for a
  file.** Take the merged blob; resolve between the markers.
- **The replay only ADDS the branch's work.** Any `-` line against the base
  that the branch's own diff does not remove is the base's work going out, so
  a deletion count against `<base>` is the cheap check and catches the
  dropped-hunk case directly.

A `--diff-filter=D` check against `HEAD` is in the recipe for the other half
of this: the loop writes and adds, so a merged tree that *deletes* a path
would land as a replay that quietly kept it.

## Read the branch's own commit bodies before resolving

Also `ranger-base-83nrw`'s, and worth repeating here: a conflict in a file a
later commit on the same branch **withdraws** is a conflict that is not one.
`ef54685` withdrew a census precisely because main's fix was better and said
so in its own body, so the resolution was written down before the conflict
existed — take main's side, one commit earlier than the branch did. A
withdrawal commit often names the merge it was anticipating.

## What is pinned, and where

- `TestMergeBlockedReplayNamesTheCountAndThePerCommitLoop`
  (internal/posse/mergeblocked_test.go) — the **cheap half**: the rendered
  copy names the count (`1 commit in, 1 commit out`, `3 commits in, 3 commits
  out`, and `every commit` when the filing could read none), the loop's walk,
  `--merge-base=$C^`, and the key read-back. The count is `o.Commits` — the
  number the body's own first sentence already prints — so there is one number
  in the text and the recipe cannot disagree with the sentence above it.
- `TestMergeBackPairsEveryCommitOfATwoCommitBranchReplayedOneByOne`
  (internal/posse/mergebackreplay_test.go) — the **honest half**, and the
  claim a prose-only fix cannot make: a two-commit branch blocked by a real
  conflict, replayed *by this recipe* in the test's own code, and
  `equivalentOnBase` accounting for **both**. Each mutant reds in one
  assertion, MEASURED 2026-11-08:

  | mutant | reds at |
  |---|---|
  | commit once for both originals (the squash) | `accounted for 0 time(s), want 2` |
  | `--merge-base=main` instead of `$C^` | original 1's `shared.mk` came out of the merge clean |

  Its negative twin, `TestMergeBackPairsNothingWhenOnlySomeOfABranchLanded`,
  was already there: it measures that a two-commit branch landing as one is
  `nil`. What was missing was the positive — that the corrected recipe
  produces a pairing at all.
- `TestMergeBlockedReplayCitesARunbookThatExists`
  (internal/treepins/mergeblockedrunbook_qa_test.go) keeps both cited
  fragments — `ranger-base-xea2y.md` and this one — in the tree.

## Verified

`make fmt-check`, `make crew-check`, `make notes-check`, `make tree-check`
(90.5s at a one-minute load of 15-33), `go vet` untagged and `-tags
posse_arm2`/`-tags posse_arm3`, the arms above under `-run`, and **`make test`
whole** — all three arms green, 2026-11-08: arm 1 `./...` (internal/posse
713.3s, internal/treepins 819.7s, cmd/posse 329.1s), arm 2 415.6s, arm 3
447.1s, and `scripts/audit-silent-reverts.sh` 2377 commits / 0 untriaged.
`go run ./cmd/checkorphans` exits 1 on one gate-shell child that is not this
bead's — pid 2444, 6h12m old at the last reading, a
`shell-snapshots/snapshot-zsh-*.sh` source that predates this session by
hours. Nothing this bead backgrounded is left: the suite ran detached as pid
2985 and `kill -0 2985` answers gone.
