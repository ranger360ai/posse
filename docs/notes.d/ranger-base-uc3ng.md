# ranger-base-uc3ng — the merge-back block's citation sent the seat to a pre-fix copy of its own recipe

Landed 2026-11-23, main `d13842d4`, git 2.56.0 / linux, go 1.26.10.

`ranger-base-ghtov` put two steps into `mergeBlockedReplay`
(internal/posse/dispatch.go) because a dispatched seat's HEAD is its **launch
base** and every command in the replay loop reads `HEAD`: `git merge --ff-only
<base>` with a `rev-parse` witness at the top, and `git merge-base
--is-ancestor <base> HEAD` at the bottom. That fix is good and it is pinned
four ways.

The copy's own last line then sends the seat somewhere else:

```
docs/notes.d/ranger-base-xea2y.md is the worked example and
docs/notes.d/ranger-base-4r8fw.md the per-commit loop, both MEASURED.
```

Both fragments carry **their own copy of the recipe**, both written before
ghtov. `ranger-base-4r8fw.md:42-69` is literally headed *"## The recipe, per
commit"* and reads `HEAD` in three commands (`:52`, `:53`, `:68`);
`ranger-base-xea2y.md` is the single-commit worked example, with `merge-tree
--write-tree` at `:13`. **MEASURED 2026-11-23**, `grep -n` over both: neither
names `ff-only` and neither names `is-ancestor`.

So a seat that does what the block's last line tells it — and that line is the
block's only MEASURED account of the loop — gets the **pre-ghtov procedure**.
It is faithful, `equivalentOnBase` pairs it, the fragment's own read-back diff
is empty (both its sides are key lists computed from the same stale pair), and
the work lands nowhere. That is ghtov's defect reached by obeying ghtov's own
fixed text, which is the shape `ranger-base-4r8fw`'s own paragraph warns about
one layer up.

## Which of the two candidates, and why

The bead named two. **The citation was fixed, not the fragments.** A
`docs/notes.d/<bead>.md` fragment is that bead's record, and editing an old
record to carry a later bead's fix is the thing this house usually refuses —
and the citation is where the authority claim is made, so it is where the claim
can be scoped. The text now says what it is citing FOR and names the two steps
its fragments lack:

```
… both MEASURED — and
CITED FOR THE LOOP ONLY. Both PREDATE the head: neither fragment names
`git merge --ff-only` or `git merge-base --is-ancestor`, and 4r8fw's own
recipe block reads HEAD throughout. …
```

One spelling for both arms of `mergeBlockedReplay`, for the same reason
`ffFirst` has one: a filing that could read no head still sends its seat to the
same two fragments. The no-sha arm gains the only mention of the ancestry check
it has ever carried.

## The pin that was missing is the EDGE

One claim, two copies, and **nothing read the second**: internal/posse's pins
read the rendered STRING and cannot open a file; the notes pins (`make
notes-check`, `adrtestcitation_qa_test.go`) resolve the PATH and never the
content. Either candidate fix would have left that hole, so the edge is pinned:

- `TestMergeBlockedRunbookCitationNamesTheStepsItsFragmentsLack`
  (`internal/treepins/mergeblockedrunbook_qa_test.go`) — reads
  `mergeBlockedReplay`'s `runbooks` const out of the Go source, resolves every
  `docs/notes.d/` path in it, **reads those files**, and reds when a cited
  fragment that carries a copy of the recipe (keyed on `merge-tree
  --write-tree`, the launcher's own operand) names a head step that the
  citation does not name either. It is a **disjunction** on purpose — the
  fragment may carry the step, or the citation may say it does not — so the
  other candidate stays open to a later bead and the pin holds either way.
  The citation is read as the CONST and not as the function body
  (`goConstLiteral`): the rest of that body names both steps as runnable
  commands, so a body-wide read would be green over a citation that said
  nothing.
- `TestMergeBlockedRunbookCitationGapCheckCanFail` — the control. The pre-fix
  citation above, verbatim, against a fragment carrying the recipe mark, must
  report **both** steps; each of the three ways to close the gap (scope the
  citation, put the steps in the fragment, a fragment with no recipe in it)
  must report none; and `goConstLiteral` must stop at the end of the
  concatenation.
- `TestMergeBlockedReplayPutsHeadOnTheCurrentBaseFirstAndChecksItLast`
  (`internal/posse/mergeblocked_test.go`, arm 2) — the cheap half, extended:
  the rendered copy says `CITED FOR THE LOOP ONLY` and names the two commands
  in the citation's own spelling, in **both** arms.

Reading named files and enumerating nothing, the new pin is the treepins
derivation's own stated negative case, so it needs no door row: `make
treepins-check` is that package minus `$(QA_TREEPINS_EXCLUDED)`.

### Both halves red on the defect — MEASURED

2026-11-23, by planting the pre-fix two-line citation back into
`internal/posse/dispatch.go` and running each half (script in this bead's
session; the file was restored from a byte copy, not from git, since the fix
itself was uncommitted):

| planted | verdict |
|---|---|
| `go test ./internal/treepins -run MergeBlockedRunbookCitation` | **FAIL**, naming both fragments and both steps |
| `go test -tags posse_arm2 ./internal/posse -run MergeBlocked` | **FAIL**: `the recipe does not say "CITED FOR THE LOOP ONLY"` |

Unplanted, both exit 0.

## Scoped out

`docs/notes.d/ranger-base-v9hv1.md:6` cites `ranger-base-4r8fw.md` for the
per-commit loop as well. That is a third citation of the same fragment, and it
is left alone: a notes fragment is a record nobody is dispatched onto, and the
defect here is that a **dispatched block** claimed an authority its citation
could not carry.

## Verified

`make fmt-check`, `go vet ./internal/posse ./internal/treepins` untagged and
under both arm tags, `make notes-check`, the two pins above under `-run` both
planted and clean, and **`make test` whole** — all three arms green,
2026-11-23, both suite slots free at the start: arm 1 (internal/posse 240.8s,
internal/treepins 301.4s, cmd/posse 115.7s), arm 2 68.6s, arm 3 115.8s,
`make treepins-check` 74.7s inside `tree-check`, `scripts/posse-land-check`
through all seven steps, and `scripts/audit-silent-reverts.sh` 3025 commits /
0 untriaged. `go run ./cmd/checkorphans` clean; the suite ran as one
harness-tracked background command and both slots read free afterwards.
