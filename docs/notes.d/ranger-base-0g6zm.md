# ranger-base-0g6zm — replaying a TWO-commit merge-back: never squash it

Landed 2026-11-06, git 2.50.1 (Apple Git-155) / macOS 26.7.1 / darwin 25.6.0.

`ranger-base-8gh58` closed with two commits on
`posse/dinesh-posse-ranger-base-8gh58` and `main` moved twelve commits on
underneath it. The launcher's replay conflicted on the first of the two,
aborted, and filed this bead.

`docs/notes.d/ranger-base-xea2y.md` is the mechanics and they are unchanged:
`merge-tree --write-tree`, write the blobs, resolve, `git add --`, commit
path-limited, no sequencer. It was written over a ONE-commit branch and says
so. This fragment is the part that only shows up at two.

## Never squash a multi-commit replay — the pairing is per COMMIT

The temptation is one replay commit holding both: it is one conflict
resolution instead of two, and the merged tree is the same either way.

It does not clear the block. `equivalentOnBase`
(`internal/posse/worktree.go`) walks **every** commit in `base..tip` and
returns `nil` the moment one of them is unaccounted for:

```go
for _, sha := range strings.Fields(out) {
        ...
        twin := replay[key]
        if twin == "" {
                return nil
        }
```

A squashed replay can pair with at most one of the two originals, so
`equivalentOnBase` answers "no" over a branch whose work is entirely on main,
`landed()` reports `Merged: false`, and **every later launcher pass re-files
this bead**. Two commits in, two commits out, each carrying the three fields
`replayKey` reads — author email, AUTHOR date (not the committer's, which the
replay rewrites), and subject — reproduced with `GIT_AUTHOR_DATE` plus
`git commit --date=` and asserted afterwards.

Both replays also carry git's `-x` trailer, `(cherry picked from commit
<sha>)`, typed by hand because nothing writes it here. That is worth the two
lines: it is the arm `equivalentOnBase` tries BEFORE the identity pairing, so
the block clears even if a later hand-edit disturbs a subject or a date.

MEASURED over the replayed branch, all three arms asked separately — every one
of them pairs both commits, so three independent reasons the block clears:

| commit | patch-id (`git cherry`) | `-x` trailer | identity (`replayKey`) |
|---|---|---|---|
| `68266f54` (code) | `+` — differs, the resolution changed a line | → `0816bfe4` | → `0816bfe4` |
| `fae7e566` (docs) | `-` — equivalent | → `e67014fd` | → `e67014fd` |

The first commit is `+` and that is expected, not a problem: resolving the
conflict changed one line, so its patch-id cannot match. It is also why the
branch will not be auto-deleted — `measuredOnBase` requires every pairing to
be `byPatch`, and a hand resolution is deliberately never licence to delete
(`ranger-base-as19`). The branch waits for a human, which is correct.

## Replay commit-by-commit, and the whole-branch merge is the cross-check

Each commit gets its own `merge-tree`, the second one against the FIRST
replay rather than against `main`:

```
T1=$(git merge-tree --write-tree --merge-base=<base> main <c1> | head -1)   # conflicted
T2=$(git merge-tree --write-tree --merge-base=<c1> HEAD <c2> | head -1)     # rc=0, clean
```

`T2` then has a free check on it. Diff it against the whole-branch
`merge-tree --merge-base=<base> main <c2>`: the only difference must be the
conflict hunk, five marker lines against the one resolved line. Here it was
exactly `1 file changed, 5 insertions(+), 1 deletion(-)` — which says the
commit-by-commit replay reproduced the whole-branch merge and resolved only
what was conflicted. Cheap, and it catches a blob written from the wrong tree.

## The conflict was two disjoint insertions into ONE line — compute it

The whole conflict was the ADR's `*Status:* ` line. `main` had appended
Amendment 4's clause (`ranger-base-778y0`); the work had inserted a probe arm
into Amendment 2's probe list. One line, 1.6 kB, both sides' text sitting in
it, and typing the union by hand is a transcription of ~1.6 kB of prose with
no diff to check it against.

So it was computed from the three stage blobs — `git cat-file blob` on stages
1, 2 and 3 — with `difflib` opcodes on the base line, asserting that every
opcode is an `insert` and that no two inserts share an offset (disjoint), then
applying both. The assertion that makes it evidence rather than a guess is the
round trip:

- remove the work's insertion from the result → must equal `main`'s line byte
  for byte;
- remove `main`'s insertion from the result → must equal the work's line.

Both held. A union that passes those two cannot have dropped or invented a
character on either side.

## What auto-merged was still checked, by xea2y's rule

- `docs/notes.d/README.md` is GENERATED, so it was regenerated rather than
  hand-merged (`python3 scripts/notes-index.py`, `--check` exits 0). The
  auto-merge happened to match the generator byte for byte this time — which
  is luck about the sort order, not a reason to skip the step.
- `internal/posse/herdrback.go` is the one `.go` file both sides edited,
  against `ranger-base-43mxx`'s `1106aa4b`. Diffed the merged file against the
  work commit, and that diff against `1106aa4b`'s own hunks for the path:
  identical but for blob hashes. So it is the work's version plus exactly
  43mxx's hunks, nothing dropped.
- the ADR BODY auto-merged around the conflicted line, and prose gives no
  compiler. Checked as added/removed line multisets in both directions:
  `main`→result minus base→work is empty, and so is the converse, with the
  Status line excepted and proven above. That reading is worth having for a
  markdown file where "it looks right" is the only other instrument.

## The whole-replay check that costs one command

After the two commits were in, every one of the work's paths was diffed
against the ORIGINAL work commit:

```
git diff --name-only <work> HEAD -- $(git diff --name-only main...<work>)
```

Three paths differed and twelve were byte-identical, and the three are
exactly the three `main` had also touched — the ADR, the generated index, and
`herdrback.go` — each of which had already been accounted for above. Twelve
of fifteen files provably carrying the original bytes, and a named reason for
each of the other three, is the strongest statement a replay can make about
having kept the work. It is also the check that would catch the quiet
failure this recipe has: a `cat-file blob` loop that wrote one path out of the
wrong tree.

## The first full suite reddened on a KNOWN flake, not on the replay

Worth writing down because the next replay will meet it and the reflex is to
suspect the merge. `make test` over the replayed tree failed arm 1 on
`TestQAAFailedDestroyKeepsTheRecordAndRetries` — a `ranger-base-43mxx` pin
over `reclaim`, in `herdrback.go`, which is the one `.go` file this replay
merged. Every signal pointed at the merge.

It was not the merge. `docs/notes.d/ranger-base-43mxx.md` section 8 already
has it, MEASURED and named "a lock flake that is not this bead": `flock` is
per open file description, a child between fork and exec still holds a
reference to the one the parent just closed, and this package forks its fakes
constantly, so a test can see `a launcher is running` on its OWN
per-test home. The warn stream in the failure says exactly that — pass 1
attempted the destroy, passes 2-20 were all spared by the launcher lock.

Two readings settle it, and both are worth taking before suspecting a merge:
the pin is green 3/3 focused (`-run`, no suite slot, at a load average of
75), and the merged `herdrback.go` was already proven byte-equal to the
work's version plus exactly 43mxx's own hunks. `pruneUntil`'s bound of 20
listing passes is what absorbs the flake, and 20 was not enough on a box at a
load average of 49-75 with both suite slots held. Filed rather than fixed
here — it is 43mxx's code and this bead's scope is the replay.

## Verified

`make fmt-check`, `go build ./...`, `go vet ./internal/posse ./cmd/posse`,
`make tree-check` (rc=0, 212s at a one-minute load average of 48.86 — the
AGENTS.md band for a saturated box), `python3 scripts/notes-index.py --check`,
`make test` whole over the final tree, and `go run ./cmd/checkorphans`.

One caution on reading a backgrounded suite, learned here: `make test > log;
echo EXIT=$?` reports the ECHO's status to the harness, so the run was
announced as exit 0 while `make` had exited 2 and the log said so. Grep the
log for `FAIL` and read the recorded exit; never take the wrapper's.
