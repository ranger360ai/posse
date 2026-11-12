# The landed check runner: what the cage costs, and the two things it is NOT (ranger-base-5fond)

ADR 0088 D1-D4 says the launcher reads main's new tree after every landing:
caged, in a detached worktree of the landed commit, outside the lock, one run
at a time, coalesced to the newest tip, cached by tree id. `scripts/posse-landed-check`
— this repo's body of D6 — landed under ranger-base-r3pqf and is measured
there. This is the runner that calls it (`internal/posse/landedcheck.go`),
and the D5 filing it feeds is ranger-base-ddgfr.

Dated 2026-11-11. Every number below was taken on the D1 host (Linux
7.2.15-1-omarchy, 16 cores, `go version go1.27.3-X:nodwarf5 linux/amd64`,
bwrap at `/usr/bin/bwrap`), from this bead's worktree, judging its own
`436f467c`.

## The cage costs nothing measurable, which is the reading that mattered

D3 puts the check behind the box's own cage tier — bwrap here, sandbox-exec
on darwin — with the landed worktree and the shared build cache writable and
nothing else. The open question was never whether that renders; it was
whether that write set is **enough** for a check that forks `make`, which
forks `go vet`, `go test`, `python3` and `bash` across nine doors. A grant
list one entry short does not render wrong. It renders, and the landing reads
red for a reason that has nothing to do with the tree.

| run | exit | wall | loadavg (1m) before |
|---|---|---|---|
| caged, warm 1 | 0 | **70s** | 4.26 |
| caged, warm 2 | 0 | **71s** | 5.51 |
| caged, warm 3 | 0 | **71s** | 6.39 |
| **uncaged**, same script, same shape of worktree | 0 | **72s** | 4.18 |
| caged, git dir resolved out of git (see below) | 0 | **69s** | 4.84 |
| caged, same | 0 | **81s** | 5.70 |

**So the cage is free.** Five caged readings, four of them inside 2s of each
other, and the uncaged control is slower than any of those four. Against D6's
120s ceiling — at which the price goes back to architecture — that leaves
39-51s of room, and against ranger-base-r3pqf's uncaged 49-64s the difference
is this tree and this box's load rather than the wall: the control above was
taken with the same bytes on the same afternoon and came out at 72s too. The
81s reading is the box and not the change — it was taken at a five-minute
load average of 8.5, the busiest of the six, over bytes identical to the 69s
one.

The write set that makes it work is three entries (`landedCheckWritable`):

- **the run directory**, which holds the detached worktree, the judge
  extracted out of git, and the `TMPDIR` the check's children stage and link
  in. One grant rather than three, because they are one directory the
  launcher makes and removes;
- **the worktree's private git directory**, `<repo>/.git/worktrees/<name>`,
  **resolved by asking git and never spelled**. A linked worktree keeps its
  own HEAD, index and logs there, and `history-check` and the silent-revert
  audit read git history — which is *why* D3 says a worktree and not a
  `git archive` export. What stays out is the repo's COMMON dir, where
  `packed-refs.lock` lives and where a stranded lock is the operator's to
  remove and never a seat's (ADR 0059 D3).

  The first three readings above were taken with that path spelled by hand as
  `<run>/tree.git`, which is **not where git puts it**: the repo's own `.git`
  can be a file, a `commondir` can redirect it, and the directory name is
  git's to choose. `BwrapArgs` creates a write grant that is not there yet, so
  what the wall got was a bind of an empty directory nothing reads — while the
  real admin dir stayed read-only. **The check passed anyway**, which is
  precisely why it was silent, and it is the shape of defect this whole bead
  exists to catch one layer up. `git rev-parse --absolute-git-dir` inside the
  worktree is the question's own answer;
- **the shared build cache**, resolved without forking `go env`
  (`$GOCACHE`, else `$XDG_CACHE_HOME/go-build`, else the per-OS default). A
  private cache would make every landed check a cold build.
  **Withdrawn by ranger-base-4nzpo**: the shared cache is the launcher's own
  build's, so the judge now builds through a fresh `<run>/gocache` inside the
  run dir, and the write set is the first two entries.

## Two things the runner is NOT, each because a measurement said so

### 1. It is not synchronous, and it is not unjoined either

D2 says the check runs outside the lock and never blocks the landing. D4 says
at most one is in flight. Both are the scheduler (`landedChecker`): a landing
SAYS which repo moved and returns; one goroutine reads the newest tip when it
gets there.

What is easy to miss is the other half. A caged `make` that outlives the
process that started it is an unmarked orphan, and the fleet load guard's kill
arm ends one on any tick of a `--watch` loop (AGENTS.md, "Something you MEANT
to leave running"). So **the pass that schedules is the pass that waits**:
`defer d.landedChecks().Wait()` at the top of `Dispatcher.Run`, covering all
nine of that function's early returns, and the same join at the end of
`posse worktrees --land`. The landing returns immediately; the *pass* does
not return over a live child.

`posse kill`'s landing arm schedules nothing, and that is a stated residual
rather than an oversight: a kill must stay instant (it is the one site that
takes the launcher lock *without waiting*, for the cockpit's sake), so a kill
that scheduled one would either block the cockpit for a minute or leak the
child into a process about to exit. A tree landed by a kill is read by the
next landing into that repo, one tip later.

### 2. It is not judged by the tree it is judging

D1: the copy that runs comes from the tree last recorded green, not from the
tree being judged — a landing that weakens the script does not change what
judges it. The judge is extracted with `git cat-file blob <tree>:<path>` and
written beside the worktree, never inside it.

**The record is one entry, so "last green" is the record when it is green.**
D4 spells `posse.landedCheck` as `<tree-id> <green|red>`, which holds the last
tree *judged* and not a history — so during a red stretch there is no green
tree on record to take the judge from. That is read as D1's documented absent
case (the landed tree's own copy, said once) rather than as a reason to coin a
second config key. The alternative is a judge that gets older every landing
until somebody fixes main, and a red stretch is exactly when the script is most
likely to be the thing being fixed. ASSUMED, and it is the one place this
implementation decides something D4's wording leaves open.

## The suite's own marker is not the launcher's environment

The first live run read **red at `crew-check` in 0.005s**, where the same door
run by hand was green in 4.0s. The cause was the suite, not the cage: the
child inherits this process's environment (D3 changes `TMPDIR` and nothing
else, which is ADR 0087's land check's rule too), and `internal/posse`'s
`TestMain` turns **any** posse test binary carrying `RHQ_FAKE_HERDR` into a
fake `bd`. This repo's check runs `make`, whose doors run
`go test ./internal/posse`, so the marker reached the inner test binary, which
answered `fake bd: unhandled -test.run=…` and exited 1.

A launcher carries none of those, so the live pin sheds them and the three
readings above are what a launcher gets. It is worth writing down because it is
the mirror of the finding ranger-base-r3pqf's close already made from the other
side — there, `MAKEFLAGS`/`MAKELEVEL` inherited from a parent `make`; here,
`RHQ_FAKE_HERDR` inherited from a parent `go test`. **The caller's environment
is an input to this check, and the only actor that scrubs any of it is the
script.** Posse scrubs nothing of its own. Stated, not fixed: the launcher
process sets none of these, and a runner that decided which of the operator's
variables a repo's check may see would be posse legislating about a contract
whose whole point is that the repo owns what "green" means.

## What runs this

`internal/posse/landedcheck_test.go`, suite arm 2, **twelve arms, 0.6s** for
all twelve, over planted repos and never over this tree. The cage is the one
thing stubbed, through the seam the production code carries for it
(`landedCageFn`), for two reasons: a suite that rendered a real wall would be
pinning bwrap's mount semantics rather than this runner, and the arm most
worth having — **a cage that REFUSES** — is unreachable on a box where the
cage works. The stub is not a bypass; `plainCage` still returns the judge as
the argv, and `TestQATheLandedCheckExecsNothingButTheCagesOwnArgv` holds
structurally, over the real file, that the only child this runner starts is
the cage's own rendered argv. D3 has no uncaged arm, and the rule is about a
branch that does not exist rather than one that is not taken.

The four done-whens the bead named are four of the twelve:

| done-when | arm |
|---|---|
| runs coalesce to the newest tip | `TestRunsCoalesceToTheNewestTip` — three landings, one check held open, exactly two runs, and the intermediate tip judged by neither |
| the judge is the last-green tree's copy | `TestTheJudgeIsTheLastGreenTreesCopyAndNotTheLandedTrees` — main lands a check that says `v2`, and the log's second line says `v1` |
| a refused cage gives no answer and no uncaged exec | `TestARefusedCageGivesNoAnswerAndNeverRunsUncaged` — plus the AST pin above |
| the landing returns before the check finishes | `TestTheLandingReturnsBeforeTheCheckFinishes` — driven through `Dispatcher.scheduleLandedCheck`, the call every landing arm makes, and asserted from *inside* a run that is still holding |

The live reading is `internal/posse/landedchecklive_test.go`, off by default
and a prerequisite of nothing:

```
RHQ_LIVE_LANDEDCHECK=1 go test -tags posse_arm2 ./internal/posse \
  -run TestLiveLandedCheck -v -timeout 10m
```

It writes to the shared store — `git worktree add` registers under the repo's
common `.git/worktrees/` — which is why the env gate is not a formality, and
it puts `posse.landedCheck` back the way it found it so a measurement cannot
hand the next landing a cached verdict the launcher did not take.
