# The land check at the replay site: four judgment calls ADR 0087 left open (ranger-base-ej47m)

ADR 0087 is the design and `internal/posse/landcheck.go`'s header is the
mechanism. This fragment holds only what the build decided that the ADR did
not say, so the next reader of either can tell a choice from a transcription.

MEASURED/decided 2026-11-11, this box (linux 7.2.15 x86_64, go1.27.3), from
the dinesh-posse-ranger-base-ej47m worktree.

## 1. The opt-in is the FILE, not its execute bit (a stated divergence)

D1 says "a repo opts in by shipping an **executable** at
`scripts/posse-land-check`", and the bead repeated it: `landCheckShipped` =
executable. Shipped reads a REGULAR FILE and does not ask about the mode.

Why: the two readings differ on exactly one input — a file at that path
without `+x` — and they differ in opposite directions. Reading the bit as the
opt-in makes that file mean "this repo has no check", which is the one failure
that silently restores the a2327230 hole: judged nothing, said nothing, main
goes red again. Reading the FILE as the opt-in sends the same case through
exec, whose refusal is a "could not answer" that lands with a ⚠ naming the
file (D3's bottom row, which is reachable either way — a bad shebang, a FIFO,
a wrong-arch binary). Nothing on this box has a file at that path, so the
choice is theoretical today and the `posse-` prefix is what makes the
file-alone reading safe: D1 coined the prefix so that a repo's own `scripts/`
cannot match it by accident.

Non-regular files are still refused, for ranger-base-gs9r's reason one
function over: an executable FIFO passes a mode check and then blocks its
reader — here, the launcher, holding the fleet lock, for `generatorTimeout`.

## 2. `exec.ErrWaitDelay` is a third answer, and it is "no answer"

`generatorKillGrace` exists because os/exec's Wait does not return while a
GRANDCHILD holds the child's output pipe. This repo's check is a shell script
running `go`, so it has grandchildren by construction — the grace is
load-bearing here rather than held on principle (generatedindex.go holds it on
principle, and says so).

A check that exits 0 and leaks a child reaches `cmd.Run()` as
`exec.ErrWaitDelay`, with a real exit status beside it. It is read as
`landUnanswered` and not as that status: the output was truncated at the grace
by an unknown amount, and a repo whose check leaks a process into the launcher
is not one whose verdict should move main. Pinned by
`TestACheckWhoseDescendantOutlivesItIsNotAVerdict` (5.0s, the grace).

## 3. A block that cannot be realized is not reported as a block

D5's undo is `reset --hard <pre-replay tip>`. If that cannot happen — `pre`
unread, the tree's HEAD not on its branch, git refusing — the branch is left on
the replayed tip, and the ratchet is then WORSE than not blocking: the next
pass takes that tip by plain fast-forward, which is the arm ADR 0087 does not
judge, and the same red tree lands with nothing said at all. So that arm lands
now and warns, naming the reason the reset failed.

MUTATION-CHECKED, and this is how the arm was found: deleting the reset reds
four of the six landing tests, and `TestASecondReplayOverTheSameBaseTakesThe
BaseVerdictOffTheRecord` reds *because the second call fast-forwards* — the
same sequence, one pass later, with no ⚠ anywhere.

The reset also restores `o.Regenerated`/`o.Commits` to their pre-replay
values, because a generated-index commit made on the far side of the replay is
discarded with it and `RegeneratedNote` is printed on refusals as well as on
landings (landsweep.go says so in as many words) — a ⚠ pointing a reader at a
commit that is no longer in `git log`.

## 4. The sentence lives in worktree.go, the mechanism in landcheck.go

`mergereasonfloor_qa_test.go` derives the class of merge refusals by parsing
`worktree.go` alone and FAILS CLOSED on a sentence it cannot follow. The first
build produced the refusal inside `landCheckGate`, and the pin said so by
name. So `landCheckGate` returns the check's output and `landCheckRefusal`
(worktree.go, beside `constitutionLandRefusal`) is the wording — the split
`refreshGeneratedIndex` already uses. The census is 16 sentences / 16 cases
now; the new row is in `mergeBlockedCases` with the arm
`landing it would turn a green`, and its fixture (`landCheckBlockedTree`) is
the a2327230 shape.

## What is pinned, and what is not

Nine tests in `internal/posse/landcheck_test.go` (arm 2), the whole file
costing ~6s, 5.0s of which is the grace arm above. The ratchet's four rows,
the record, the base probe and its restore, D5, whose copy of the script runs,
and the sweep's one-probe-per-base-movement bound.

NOT pinned here: the mtime half of the quiet-tree property. A land-check block
is the hardest case for it — the branch merges cleanly, so `mergesCleanly`
opens the gate on every pass and only `probedBaseKey` closes it — but it takes
the identical path through `standingMergeBlock`, and
`TestABaseThatMovesAndStillConflictsKeepsTheBlockAndTheTreeQuiet` and
`TestAFilterThatDisagreesWithTheReplayProbesOncePerBaseMove`
(mergeblocked_qa_test.go) are where that property is measured. What this file
measures instead is which passes run the check at all, counted by the check
itself.
