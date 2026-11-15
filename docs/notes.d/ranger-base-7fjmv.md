## A WRITE-time gate is install-gated exactly like a commit-time one, and it is the fourth row (ranger-base-7fjmv)

ranger-base-m1obc's done-when 4 asked whether ADR 0073 D2's `posse gates
bd-write` belongs in the COMMIT-time gate class or is a fourth one, and did
not price it. The answer is a FOURTH row, and it cost what the third did:
four entries in `categories()`, two fields on `WallRenderer`, one more
`narrowLag` call, and not a line of new branching anywhere that renders.

### 1. The category

A wall is install-gated because the installed binary RENDERS it
(`WallRenderSources`). A launch-time check because the installed binary
DECIDES it at launch (`LaunchCheckSources`). A commit-time gate because the
installed binary JUDGES it at commit time (`CommitGateSources`). This one
because the installed binary JUDGES it at WRITE time, for every bd write any
persona on this box types.

The callback shape is the commit-time row's exactly. The bd gate SHIM is a
render — `gates.go` writes it and is already in `WallRenderSources` — and its
write-verb tail ends in

```sh
RHQ_GATE_LOG="$RHQ_GATE_LOG" exec "$posse_bw_bin" gates bd-write --real "$posse_bd_real" -- "$@"
```

which resolves its judge fresh from whatever `posse` is on PATH, on purpose:
ADR 0073 D2's lock must be held by the process that becomes bd, so the judge
cannot be a render-time stamp. A perfectly current shim can therefore route
every write to a judge a binary behind.

### 2. Why it is not the commit-time row

`wallrenderer.go`'s own rule: two categories that differ in prose and in
pathspecs are two rows.

- the decision is taken at WRITE time, not commit time;
- the consequence is a lost or duplicated bead write and **nobody is
  refused**: a write that loses the race prints `Created issue: <id>` and
  exits 0 like any other (ADR 0073's rig: 34-36 of 48 concurrent creates
  lost, a `close` reading back `in_progress` in 1 of 3 trials under a comment
  storm). A stale commit-time judge refuses a commit to the committer's face;
  a stale write-time judge is silent and the cost lands on whoever reads the
  graph next. Different sentence, so a different row;
- the invoking render is the bd shim and not a hook body, so
  `commitgatesources_qa_test.go`'s derivation excludes it by construction
  rather than by hand — asserted from both rows now, so the two cannot merge
  because a clause widened.

### 3. The pricing (done-when 3), MEASURED 2026-11-14 on this box

**Today's window**, installed `0.5.3+7d0f8c31` against `main` at `35e8cf41`,
75 commits:

| category | commits |
|---|---|
| rendering a wall | 12 |
| deciding a launch-time check | 9 |
| judging a commit-time gate | 4 |
| judging a write-time gate | **0** |

That zero is the row's first value. It is said out loud, in the same
sentence as the other three, so an operator can tell it from a reading that
never ran — which is the silence ranger-base-atvd4 and ranger-base-m1obc were
each filed for, one category at a time.

**Which commits have ever touched these paths**: three, of main's 2,756.

| commit | date | what the reading would have said |
|---|---|---|
| `2a9f4e06` (ADR 0073 B1) | 2026-10-12 14:12 | `bdwritelock.go` + `beads.go` — **no path in any list that existed**, so it sat inside the headline, named by no category at all |
| `21508a1a` (ADR 0073 B2) | 2026-10-12 16:19 | also touched `gates.go`, so it was counted under "rendering a wall" beside a cost sentence about seatbelts, gate shims, cage mounts and hooks; its judge half (`bdwriteshim.go`, `bdwritelock.go`, `cmd/posse`) was named by nothing |
| `6b4cd64d` (ADR 0072 B1) | — | created `bdline.go`, the projection the judge refuses on |

So the two commits that mattered reproduce both failure modes the three
earlier rows were filed for, one row over: one invisible, one mis-attributed
as a wall. `2a9f4e06` is the sharper of the two — it changed the per-store
flock ALONE, which is exactly the shape the third clause of the pin below
exists to reach.

**How long the window was is NOT MEASURED, and cannot be from here.** Of 101
wall-renderer readings across the 490 session transcripts this box retains
(mtimes 2026-11-07..2026-11-14), 10 name a distinct installed stamp; 8 of
those resolve in this tree and all 8 already carry both ADR 0073 commits, so
this row reads 0 in every retained reading and the window in which it would
not predates the corpus. The other 2 are `0.5.0` stamps this tree does not
hold: their lag over these paths is UNKNOWN here, never zero. No live
`bd write lock skipped — posse binary not found at render time` notice exists
in the corpus either; both transcripts that carry that string carry it as
quoted source.

**What the row guards is measured**: 3,264 bd WRITE calls typed in Bash tool
calls across those transcripts in the week to 2026-11-14 — peaking at 1,074
in one day across 123 seats — every one of them serialized and judged by
whichever posse was installed, not by whichever posse had landed.

Reproduce (read-only, no suite slot):

```sh
# the window and its four counts
git rev-list --count 7d0f8c31..main
git log --oneline --no-decorate 7d0f8c31..main -- <each list's pathspecs>

# every wall-renderer reading this box retains, and its installed stamp
LC_ALL=C grep -ho 'wall renderer ([^)]*) · [^ ]* is [0-9]* commit' \
  ~/.claude/projects/*/*.jsonl | sort | uniq -c | sort -rn
git merge-base --is-ancestor 2a9f4e06 <stamp>   # did that binary carry the judge?

# the bd write traffic: assistant Bash tool calls, verb classified the way
# bdVerbWrites does (bd's global options skipped, `comments add` and
# `dep add` split from their read siblings), grouped by day.
# 3,286 matches less 22 that are not bd verbs (`:=`, `2>`, …) = 3,264.
```

### 4. The derived half (done-when 2), and the one clause that had to change

`internal/treepins/bdwritesources_qa_test.go`. Three clauses:

1. **THE INVOCATION**, off the REAL rendered bd shim (`RenderGates`, the way
   `bdpin_qa_test.go` does it), reading what the shim **EXECS**.
2. **THE JUDGE**, off `cmd/posse`'s own dispatch — `cgsJudges` and
   `cgsDeclarers`, the commit-time pin's helpers, **called and not copied**.
3. **THE JUDGE'S OWN HELPERS**, in `wallrendersources_qa_test.go`'s
   call-closure discipline: strip `RunGates` off the entry point, lower-case
   what is left, and follow every call whose callee name carries that stem.
   `RunGatesBdWrite` → `bdwrite` → `bdWriteStore`, `bdWriteLock`,
   `bdWriteSplitArgv`, `ProjectBdWrite`, and so the three files.

**The bead's candidate key does NOT subsume the commit-time clause, and that
is measured.** The candidate was the commit-time pin's first clause with "a
rendered hook body" widened to "a render posse writes" — every `posse gates
<verb>` the render NAMES, with the dispatch half sorting invocations from
mentions. That sorting is sound for the hook body: 8 of its 10 occurrences
are mentions and not one of them is dispatched to a `RunGates*` judge. It is
not sound here. The bd shim's own rendered comment says

> closing the TOCTOU the separate "gates bd-line" call used to carry

and `cmd/posse` **does** dispatch `bd-line` to `RunGatesBdLine`. So the
widened clause derives `bdlineshim.go` off a comment sentence, and the pin
would then order a human to drop that path the day the comment is reworded.
Reading what the shim EXECS is unambiguous in the render itself, and arm 4's
third boundary holds the distinction as a claim about the class.

**So the two rows stay two pins**, sharing the judge clause and nothing else.
They differ in the render they read, they need opposite boundary assertions
(each must prove the other's judge does not derive into it, and a single file
asserting both cannot say which row went), and a shared first clause would
have to carry a render → row attribution map, which is the two rows written
twice.

**Why three files and not one.** `vulnprivate.go` holds its verdict whole;
this judge does not — `bdwriteshim.go` sequences it, `bdwritelock.go` holds
the flock and the write/read classification, `bdline.go` holds the projection
it refuses on, and `2a9f4e06` touched the lock alone. A one-file key (the
sibling's) would have missed the only commit this class has ever had on its
own.

### 5. The floor (done-when 4)

A **FLOOR**, read as one, with nothing keying off the list — the shipped
answer for all three existing rows and for this one. The line always reports
the WHOLE lag and always names `make install`; the per-category counts only
say how loud to read it. "A READING, NEVER A CONTROL" is unchanged, and no
freshness door was added (ranger-base-v81vo section 4 prices and rejects one).

The named residual: `bdlineshim.go`'s refusal wording and gate log are called
by this judge and are NOT in the class, because what they change is what a
refused writer is TOLD rather than whether the write is refused. It is a
floor residual, not a hand-appended path — arm 3 would red on one, and the
`exec` anchor above is why it cannot arrive by accident.

### 6. Prices

| command | wall | box |
|---|---|---|
| `go test -count=1 -run TestBdWrite ./internal/treepins` | 0.843s, then 0.264s and 0.278s warm | 2026-11-14, loadavg 3-8, no suite slot held |
| `go test -count=1 -run 'WallRenderer\|CommitGateSources\|LaunchCheckSources\|BdWriteSources\|Release' ./internal/posse` | 0.331s | same |

The pin takes no suite slot and is nothing a door has to budget for: one
`RenderGates` (one `posse gates bd-write` probe exec, shim bytes to a temp
dir), one `go/parser` pass over `cmd/posse/main.go`, and one `os.ReadDir`
plus a parse per non-test file under `internal/posse`. `make treepins-check`
is the door that runs it — since ADR 0098 named that door by what it does NOT
run, a pin added there is doored the moment it is written, with no Makefile
edit, and a fifth of a second does not go near the priced-row table it would
need an exemption from.
