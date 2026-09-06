# ranger-base-g4s6o — landing ranger-base-qp1hm by REPLAY: one conflict, and it was a sibling landing

`posse/jian-yang-posse-ranger-base-qp1hm` (the three-arm split of
`internal/posse`) closed with six commits that would not rebase onto main.
The block was real and not the stale shape the bead warns about: at dispatch
both the branch and `refs/posse/merge-blocked/posse/jian-yang-posse-ranger-base-qp1hm`
still resolved to `292ed09e`.

The six are replayed here as `19169de8 640ced2b d64368da 55c0237c 9944aec4
5c95c01d`, per commit — never squashed. Squashing would drop five of the six
`-x` trailers, and the trailer is the only pairing `equivalentOnBase` can
find for a commit whose patch a resolution amended.

## The overlap is 8 files of 416, and 7 of them merge unaided

main gained 37 files in this window. Eight of them are files the branch also
touched, and in seven the branch's whole edit is a `//go:build` line at the
top while main's is in the body, so `git apply -3` resolves them without a
marker: `extdiff_qa_test.go`, `holderjoin_qa_test.go`, `inletpin_qa_test.go`,
`inletpinextdiff_qa_test.go`, `inletpingitconfig_qa_test.go`,
`staleindex_qa_test.go`, `verify_i9dbb_qa_test.go`.

## The eighth is `internal/posse/verifyafter_test.go`, and it resolves BOTH ways

`fc425851` (ranger-base-l3jaq, ADR 0006 §6 as amended 2026-09-06) rewrote the
same region `5e58afb3` was lifting helpers out of. The two halves of that
region take opposite sides, which is the thing worth keeping:

- **the prose takes main's side.** l3jaq's paragraph describes the RECORDED
  channel — a finding whose fix runs nothing files no bead. The branch has no
  opinion about it; it was adding a build tag. main is the later state.
- **`vaClosedClassed` takes the branch's side.** The branch lifts it into the
  shared `verifyafter_helpers_test.go` so every arm compiles it. Keeping
  main's copy where it sat left the function declared twice.

Both halves are **shown able to fail**, under `go test -overlay` so the tree
stayed clean. `verifyafter_test.go` is arm 3 after `55c0237c` repacks the arms:

- drop `vaClosedClassed` from the shared helpers file → arm 3 reds,
  `undefined: vaClosedClassed` at `verifyafter_test.go:1819`;
- restore the duplicate in `verifyafter_test.go` → arm 3 reds,
  `vaClosedClassed redeclared in this block` at `:1702`.

Neither red reaches arm 1 or arm 2, which is the split doing its job rather
than a leak between the arms.

## What the replay was checked against

`git diff-tree -r --name-only 292ed09e HEAD` names 37 differing files and
every one is a file main itself changed — zero unexplained. Tag parity is
file-by-file against `5e58afb3`: every `internal/posse` test file has a
byte-identical first line and the untagged (shared) set is identical, so
nothing that landed on main falls outside the partition. Every line main
ADDED in the eight overlap files survives the replay, checked line by line —
including the lines it added OUTSIDE the conflict markers, which `git apply -3`
keeps silently because the branch's base never had them.

Read the diffs with `git diff-tree -p`, not `git diff`: a posse-launched seat
exports an empty `GIT_EXTERNAL_DIFF` and `git diff` dies with
`cannot run : No such file or directory`.

## The source tree strands, and nothing is filed for it

Probed against the live repo (`equivalentOnBase`, base = this branch, tip =
`292ed09e`): `len(eq)=6`, so no new merge-back block re-files;
`measuredOnBase=false`, because `5e58afb3` pairs only by the hand-written
trailer; `contentNotOnBase` names the same eight overlap paths, correctly —
their exact branch bytes are a pre-merge state main never held.

That is the paired-tip class ADR 0058's 2026-09-06 amendment
(ranger-base-qz3cr) now covers: once this bead closes no landing is owed on
the branch, and the tree retires with its tip kept under
`refs/posse/retired/<branch>`. That mechanism is ranger-base-daa60, in
progress. The amendment names the pile of seven unexecuted hand-recipe
question beads as the problem daa60 exists to fix, so this bead files no
eighth. Failing daa60, the tree wants `git worktree remove` plus
`git branch -D` by hand.

## The suite ran green and the arm 1 number is the box

`make test` exit 0, all three arms, 2026-09-06 16:56:25–17:28:21 EDT: arm 1
`./...` (posse 840.978s, cmd/posse 442.032s, internal/posse 988.077s), arm 2
317.285s, arm 3 336.969s. Zero FAIL lines, gofmt clean, `go vet` clean under
each arm, silent-revert audit 1562 commits / 0 untriaged.

**988s is not a reading of what the split bought**, and the second run says
so rather than the reasoning. `suite-lock` queued the first run 67s behind
two other full suites on the box (dinesh on daa60, gwart on hna69) and arm 1
runs its three packages concurrently against them. Arm 1 re-run at
`fe0c0938` — the same tree plus this file — on a box that had gone quiet:
`internal/posse` **452.114s** against **988.077s**, `posse` 471.461s against
840.978s, `cmd/posse` 254.814s against 442.032s. Same commits, same arms,
2.2x apart. So neither run measures the split, and the numbers in
`docs/notes.d/ranger-base-qp1hm.md` — taken on a quiet box — are neither
confirmed nor refuted here. Grade a suite-time claim on this box by what
else held a `suite-lock` slot, and read the queue line at the top of the run.

## Two seat facts for the next replayer

- Every path-limited commit here printed
  `Unable to create '…/.git/packed-refs.lock': Operation not permitted` and
  the commit succeeded anyway. That line is the repack, not the write.
- A path-limited commit takes the paths you name and no others. `5e58afb3`
  also adds `armtags_qa_test.go` at the repo root; a commit limited to
  `internal/posse/` left it staged and out of the replay, and it needed an
  `--amend` naming both paths.
