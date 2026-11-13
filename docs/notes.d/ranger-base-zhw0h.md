# install(1) leaves the target absent for part of every install (ranger-base-zhw0h)

ADR 0092 D3.5 chose `rename(2)` over `install(1)` for the binary swap on an
assumption: "`install(1)` unlinks the target and then writes it. A launch that
execs `~/.local/bin/posse` inside that window gets a missing or partial file.
ASSUMED from GNU coreutils behaviour; not probed." The build bead
(ranger-base-zhw0h) was asked to probe it. It is true, and the window is not
small.

## MEASURED 2026-11-13

Box: linux 7.2.15-1-omarchy, GNU coreutils 9.12, python3. Probe: a 64 MiB
source replacing a 64 MiB target **on tmpfs**, with a reader thread `stat`ing
the target in a tight loop for the whole replacement. Three trials of each
method, alternating. Tmpfs is the fastest surface this box has, so every
window below is a FLOOR — the same install onto a disk-backed `~/.local/bin`
has a wider window, not a narrower one.

| method | reads | ENOENT | short | distinct sizes seen | wall |
|---|---|---|---|---|---|
| `install -m 0755` | 82,847 | 1,791 | 12,606 | 10,599 | 0.058 s |
| `install -m 0755` | 85,028 | 2,473 | 13,384 | 11,253 | 0.059 s |
| `install -m 0755` | 84,197 | 2,272 | 12,678 | 10,780 | 0.058 s |
| write-beside + `rename(2)` | 70,938 | 0 | 0 | 1 | 0.032 s |
| write-beside + `rename(2)` | 71,296 | 0 | 0 | 1 | 0.031 s |
| write-beside + `rename(2)` | 70,907 | 0 | 0 | 1 | 0.026 s |

"short" is any observation whose size was not the full 64 MiB; "distinct sizes
seen" is how many different partial lengths the reader caught, which is the
shape of a sequential write rather than of one unlink. `install(1)` is both: it
unlinks (the ENOENT column) and then writes the new bytes in sequence (the ten
thousand intermediate sizes). Both halves of the window are reachable by an
exec, and the second is the larger one — about 15% of observations caught the
file absent-or-short across a 58 ms replacement.

Scaled to the real binary (17.8 MB on this box 2026-11-13) that is a window of
roughly 16 ms per install. A fleet that launches several sessions a minute will
not hit it often. It will hit it, and what it gets is `posse: cannot execute
binary file` or a truncated ELF, from a command the operator has no reason to
suspect.

`rename(2)` closed it in every trial: the reader saw one size, no ENOENT, and
exactly two inodes — the old one and the new one, never neither. Both methods
give the target a new inode, so a running process or a bind mount of the old
name keeps the old file either way; that is unchanged from `make install`.

The probe lives in this bead's session scratch and is reproduced in the pin:
`TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort`
(internal/posse/upgrade_test.go) is the same reader loop at 8 MiB, asserting
zero absent and zero neither-version observations over `upgradeSwap`.
Mutation-checked: replacing the `rename(2)` with an in-place `os.WriteFile`
reds it with 172 neither-version reads out of 192.

## What this does NOT say

The Makefile's `install` target is unchanged and still uses `install(1)`. This
measurement is the argument for ADR 0092 D3.5's choice inside `posse upgrade`,
not a finding against `make install`, which a human runs when nothing is
launching. Narrowing that window for the Makefile too would be a separate
change to a target this bead did not touch.

## What step 2 costs

ADR 0092's Consequences ASSUMED 15-25 min for the `make test` of D3.2, "to be
measured on the first real run". MEASURED 2026-11-13 at 983ae5d plus this
bead's changes, default TMPDIR, this box with other seats working and both
suite slots in normal contention: **11m57s wall** (18m02 user, 13m02 sys),
EXIT=0, all three arms plus the tree-wide doors `make test` takes as
prerequisites. Each arm queued for and took slot 1 of 2 in turn.

Read it as a floor for the verb rather than as the verb's own number, for two
reasons that both point the same way: this run was in an existing worktree
with a warm build cache, where `posse upgrade` adds a `git worktree add` and
compiles a commit the cache may not have seen. The three end-to-end runs below
each reached ~4m before dying inside arm 1, which is consistent with it. So
the ADR's 15-25 min is a safe envelope and its lower end is about right; a
delegate should still background the verb and wait on its pid rather than hold
a foreground call, which is what AGENTS.md says and what D3 already assumes.

## What the verb was run against, for real

Three end-to-end runs on 2026-11-13, each a binary stamped at an older commit
of origin/main and pointed at a throwaway target under this session's scratch,
so nothing of `~/.local/bin` was ever at stake. All three ended at step 2 with
the suite red for the TMPDIR reasons above, which is the refusal working:

- the resolution is live — run 3 picked up `983ae5d` where runs 1 and 2 had
  resolved `70ab74b`, because the coordinator pushed in between, and the count
  went 1 → 4 with it;
- on every red run NOTHING was written: no `<target>.new`, no `<target>.prev`,
  the target byte-identical, and `git worktree list` in the source checkout
  back to what it was — the throwaway worktree removed by the deferred
  cleanup even on the refusal path;
- the refusals an operator will actually meet were taken against the live
  source checkout rather than a fixture: the default target already installed,
  a commit on `main` but not on `origin/main`, a downgrade (which printed the
  rollback command as its remedy), and a sha the repo does not hold.

What no end-to-end run reached is the green path — build, swap, epilogue, D6
line — because no run got a green suite out of this box's TMPDIR. That path is
pinned instead by `TestUpgradeGreenPathSwapsAndEndsWithTheRecordLine`, which
makes a real detached worktree of a real commit, asserts the step order, the
swap, the `.prev` copy and that D6's line is last; by
`TestUpgradeBuildHandsCleanBuildTheRealSourceCheckout`, which runs the real
`upgradeBuild` against a script that reports what it was given; and by
`TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort`. Every one of those is
mutation-checked.

## The build-source stamp, which `posse upgrade` would have lost

`scripts/clean-build.sh` stamps the checkout it was run in as the binary's
build source, so the freshness reading can find its own checkout on a box whose
`beads:` never names posse (ranger-base-eiwhk). ADR 0092 D3.3 runs that script
inside a throwaway detached worktree of the commit being installed — so `$repo`
there is a directory the verb removes minutes later, and every binary installed
by the verb would have carried a dead path. That is exactly the `$src` case
`internal/treepins/buildsource_qa_test.go` already refuses, reached by a new
route, and it would have shipped green: a dead stamp degrades silently to the
UNKNOWN reading ranger-base-eiwhk fixed.

The script now reads `source_repo=${POSSE_BUILD_SOURCE:-$repo}` and stamps
that. The variable has one caller — `upgradeBuild` in internal/posse/upgrade.go
— and is unset for every other build, which is every `make build`, `make
release` and `make install`, where it is exactly `$repo` as before. The pin was
widened to hold the default as well as the symbol, because a `source_repo=`
that stopped falling back would leave ordinary installs stamping nothing with
the symbol line still green.

## TMPDIR decides whether step 2's verdict means anything

ADR 0092 D3.2 runs `make test` in "a detached worktree of `<sha>` in a fresh
`mktemp -d`". `mktemp -d` is TMPDIR, and this tree's suite turns out to be
sensitive to it in two directions, both of which read as "this commit is red".
MEASURED 2026-11-13 at 983ae5d, one `go test -run` filter per test:

| test | /tmp | under $HOME | /var/tmp |
|---|---|---|---|
| `TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate` | PASS | FAIL | FAIL |
| `TestQAStatusPrintsTheSecondStoreLine` | PASS | FAIL | PASS |
| `TestClaudeConfigDirIsFenced` | PASS | FAIL | PASS |
| `TestCutDestinationKeysChangeNothing` | PASS | FAIL | PASS |

- **under `$HOME`:** `AbbrevHome` renders such a path as `~/…`, so every test
  asserting its own absolute temp path in rendered output reds. The real count
  is unmeasured — these are the ones one full run surfaced.
- **anywhere but `/tmp`:** the bwrap sandbox's view of `/tmp` is not its view
  of `/var/tmp`, so the wall-witness probe reaches a different verdict about
  whether the wall holds, and ADR 0086 D5's pin turns out to be a claim about
  `/tmp` alone.

And a third, environmental rather than in-tree, which is how this was found:
this box's `/tmp` is `tmpfs … usrquota` and was at the user's limit, so a full
run there answered `disk quota exceeded` from `git` ("unable to write loose
object file"), from the linker ("mapping output file failed") and from tests
writing fixtures — ~30 failures, none naming the cause as a class. Moving
TMPDIR off `/tmp` is the obvious response to that, and it walks straight into
the other two.

None of it is fixed here: TMPDIR is the operator's dial, its default is right,
and what to do about a suite that silently moves with it is a decision with
three plausible answers and no costing. It is filed as **ranger-base-6ca29**
with the matrix. What the verb does is PRINT the worktree path under its `make
test` header, so a red run says in one line which filesystem the verdict was
taken on — a delegate whose upgrade reds with any of these signatures should
re-run with TMPDIR at its default before filing anything against the commit.

## Not closed here

- ranger-base-v87a1
- ranger-base-41l0r
