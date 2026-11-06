# A merge-back whose conflict markers were not conflicts (ranger-base-mfk8v)

Landed 2026-11-06, git 2.50.1 (Apple Git-155), darwin 25.6.0.

`ranger-base-ava0x` closed with one commit (`81e4c957`, ADR 0079 pin 9) on
`posse/dinesh-posse-ranger-base-ava0x`; `main` moved on and the launcher's
replay conflicted. `docs/notes.d/ranger-base-xea2y.md` is the standing worked
example for the replay mechanics and all of it held. What this bead adds is
the shape of the conflict, because **three of the four conflicted paths needed
no judgement at all, and the one that looked worst needed none either.**

## The block's own reading was already stale, in the safe direction

The block quoted one conflicted path from a reading taken at `main fd2f7e24`.
By dispatch `main` was `bc792266`, six commits later, and
`git merge-tree --write-tree --merge-base=1f4da269 main 81e4c957` reported
**three** conflicts, not one — the ADR and `docs/notes.d/README.md` had joined
the test file. So re-run `merge-tree` yourself rather than resolving the paths
the block names: a block is a snapshot and `main` is not.

The reading that did survive six commits was the behavioural one. Nothing
between `fd2f7e24` and `bc792266` touched `internal/posse/hookskip.go` or
`hookskipglobal_qa_test.go` (`git diff --no-ext-diff --stat fd2f7e24 main -- <both>`, empty),
so holden's "the conflict is textual, not behavioural" still read true and the
pin did not need re-deriving. **That is the diff worth running before you trust
a block's measurement** — not the dates, the paths.

## merge-tree's markers over-report an add/add on shared boilerplate

`internal/posse/hookskipglobal_qa_test.go` came out of `merge-tree` with 6
marker lines in 2 conflicts, and **neither was a real conflict.** Both sides
are purely additive and disjoint:

| side | what it changed |
|---|---|
| `main` (ranger-base-87zme) | one hunk at the exec count (`2+len` → `3+len`), then two new tests appended at EOF |
| `81e4c957` (ranger-base-ava0x) | head comment, the `hsWrapped` → `hsWrappedDir` refactor (2 hunks), then helpers + pin 9 appended at EOF |

Both append at EOF, and in Go a test function's first lines are boilerplate —
here `t.Parallel()`, then
`for _, rtName := range []string{"claude", "codex"}`, then `t.Run(...)`,
`fx := hsSetup(t)`. The diff algorithm aligned **87zme's** opening lines
against **ava0x's**, split both functions down the middle, and emitted two
conflicts whose two sides are halves of two different functions. The markers
are an artifact of the alignment, not a disagreement about content.

`git apply --3way` of `base..work` onto `main`'s file reproduces it exactly —
same 6 markers, same 2 conflicts — because it is the same algorithm. MEASURED:
`Applied patch to '…' with conflicts.` Do not read a second tool agreeing as a
second opinion.

## The resolution: reset to one side, replay the other side's hunks

Hand-untangling interleaved markers is a guess at which half of which function
you are looking at. Replaying is mechanical:

```
git checkout main -- <path>                       # one side, whole
git diff --no-ext-diff <base> <work> -- <path> > <scratch>/other.patch
```

then, per hunk of that patch, build its old text (context + `-` lines) and new
text (context + `+` lines) and do one exact string replacement, **asserting the
old text occurs exactly once** in the file. A hunk whose old side is pure
context (an append at EOF) is appended at EOF instead, since the other side has
appended there too and in Go the order of declarations is free.

Two things that bite: the patch for this file had **four** hunks, not the three
a `--stat` of two refactor sites suggests, and the assert is what catches a
mis-parse — a hunk whose old text matches zero or many times is a splitter bug,
not a conflict.

## The evidence is the two diffs, in BOTH directions

xea2y's check is "diff the merged file against the work commit, and compare
that against the other side's own commit". With both sides appending, run it
each way and know what residue is legitimate:

```
# A: ava0x's hunks landed whole, nothing dropped
diff <(git diff --no-ext-diff main -- $F) <(git diff --no-ext-diff $BASE $WORK -- $F)
# B: 87zme's hunks landed whole, nothing dropped
diff <(git diff --no-ext-diff $WORK -- $F) <(git diff --no-ext-diff $BASE main -- $F)
```

Modulo blob hashes and `@@` offsets, A's only difference was the hunk header's
**function context name** (`…UnexpandableTildeFailsClosed` against
`…MovedHomeFollowsNoTildeInclude`) and B's were the append **boundary** context
lines. Both say the same fact: ava0x's block now sits after 87zme's tests
instead of at EOF. Neither is content.

The check that does not depend on reading diff residue, and the one to prefer:
the merged file's declaration list equals the union of both sides', and
`git diff --no-ext-diff --stat main -- $F` reads `143 insertions(+), 10 deletions(-)` —
byte-identical to `git diff --no-ext-diff --stat $BASE $WORK -- $F`.

## The other three paths

- `docs/notes.d/README.md` — **generated**, as xea2y says. Reset to `main`,
  then `python3 scripts/notes-index.py`; `--check` exits 0. One added line.
  This recurs on every merge-back carrying a notes fragment.
- `docs/notes.d/ranger-base-ava0x.md` — add/none, no markers. A NEW file, so
  `git add --` it before the path-limited commit (rangerhq-4pbt).
- The ADR — **a real conflict, and the only one.** Both sides rewrote the two
  lines ending "Pins owed" item 9: `main` left it `owed by the bead cut under
  ranger-base-o6xr6` and appended items 10-12; `81e4c957` replaced that tail
  with the pin it had just shipped. Resolution is ava0x's tail (item 9 IS
  shipped now) followed by main's 10-12. One site — `grep -n 'owed'` over the
  ADR confirms item 9 is stated once, so nothing else needed the same edit.

## No sequencer state, on purpose

As xea2y says, and it is the reason to replay rather than `rebase`: nothing
here ran `rebase`, `cherry-pick` or `revert`, so there was no `CHERRY_PICK_HEAD`
to leak and no later path-limited commit dying at `fatal: cannot do a partial
commit during a cherry-pick`. Checked `git rev-parse --git-dir` for
`CHERRY_PICK_HEAD`, `REVERT_HEAD`, `MERGE_MSG`, `AUTO_MERGE` and `sequencer/`:
none created.

## Verified

- `go vet ./internal/posse`, `gofmt -l` on the resolved file: clean.
- `go test -count=1 -run 'TestQAHookSkip' ./internal/posse`: 15 tests green,
  10.592s — both of 87zme's new tests and ava0x's pin in one run. Pin 9 alone,
  real legs: claude 1.52s, codex 1.70s.
- **The pin is still a witness in the resolved tree, not just under holden's
  graft.** Mutating D2's count in `main`'s own `internal/posse/hookskip.go`
  (the `WriteString` at the `[ "$posse_hs_gl" -eq "$posse_hs_gn" ])` line) to
  `[ 0 -eq 0 ])` reds it on BOTH runtimes for the right reason: both legs
  `code=0` with no refusal printed — the full bypass — plus the in-test
  stale-witness guard firing. Restored and `git diff --no-ext-diff --stat` on that file
  confirmed empty before committing.
- `make tree-check`: green, 94.5s at a one-minute load average of 12.95.
- `make test`: green (see the bead comment).

## Two lines of my own ORDERS were stale, and the tree says so

ORDERS (compacted 2026-09-01) told me to strip `$RHQ_GATES_DIR/bin` out of
`PATH` and shed `POSSE_HOME` before any suite run. Both are now handled in
tree and the hand-surgery is unnecessary: `internal/posse`'s `TestMain` sets
`PATH=PathOutsideGates("")` (`herdr_test.go`), which the Makefile's
`verify-audit-silent-reverts` comment documents as fixed on both halves
(ranger-base-lbftm), and `TestNewAppHomeSelectionEdges` calls `clearHomeEnv(t)`
itself. Worth saying because the hand-strip is also **bypass-shaped** — an
auto-mode classifier refused the `PATH=` surgery outright, correctly, since
that dir is where this seat's `git commit unless --` deny shim lives. `make
test` plain is the right command; the suite insulates itself.

## Then main moved again, and the replay loop has its own trap

Between the suite finishing and the close, `main` gained
`ranger-base-y9i3o` — ADR 0079 D2 witness work, touching the ADR,
`docs/notes.d/README.md` and `hookskipglobal_qa_test.go`: every path this
bead had just resolved. `merge-tree` this time exited **0**, clean. Two
checks earned the trust, because a clean merge is not evidence the tests
hold:

- `go vet ./internal/posse`, for the collision a clean textual merge cannot
  see — y9i3o added `internal/posse/hookskipwitness_qa_test.go` with its own
  helpers, and a `func` redeclared across two `_test.go` files compiles
  nothing and reads like an environment problem. Clean: its `hsRCStub` and
  three tests do not collide with this bead's `hsWrappedDir`, `hsGitOut` or
  `hsCountTest`/`hsCountMutant`.
- what y9i3o actually changed in the SHARED file, read as a diff rather
  than assumed from the subject: a comment block on
  `TestQAHookSkipMovedHomeFollowsNoTildeInclude` saying what that pin does
  not witness. No helper this bead's pin calls moved, so the premise held —
  and `git diff --no-ext-diff --stat HEAD -- <the file>` still read
  `143 insertions(+), 10 deletions(-)`, byte for byte what it read against
  the previous `main`.

**The trap, and it is not in xea2y:** the replay writes out the paths that
differ from `main`, but your worktree is at your OLD head, so main's own new
files — ones the merged tree holds and `main` holds identically — are not in
that path list and are simply ABSENT from your tree. `git status` after
`git reset --mixed main` then reads `D docs/notes.d/ranger-base-y9i3o.md`
and `D internal/posse/hookskipwitness_qa_test.go`: two deletions of another
bead's work, staged to be committed under yours. The deletion check the
block prescribes does not catch it — `git diff --diff-filter=D --name-only
main $TREE` is empty, correctly, because the merged TREE deletes nothing.
The check that catches it is comparing the WORKTREE to the merged tree
(`git diff --no-ext-diff --stat $TREE`, which must be empty), and the fix is
`git restore --source=HEAD -- <main's new paths>`. Either start the replay
from a worktree already at `main`, or run that comparison before you commit.
