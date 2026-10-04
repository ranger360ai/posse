## `bd ready` excludes `in_progress`, so dispatch could not see an interrupted run (ranger-base-eh1kr)

The fire loop in `internal/posse/dispatch.go` holds the whole recovery
contract for a claimed bead — the holder join (ADR 0004 §2), ADR 0008's crew
shield, ADR 0030's orphaned-claim tiebreak, rangerhq-zom's
stopped-on-purpose skip, `--resume`'s override, ADR 0004 §3's "re-prompt the
holder, or launch it if gone". Every branch of it keys on
`is.Status == "in_progress"`.

**No bead with that status ever reached it from the store.** A pass reads its
queue with `bd ready`, and that is not a query about claimed work:

```
$ bd ready --help
Show ready work (open issues with no blockers).

Excludes in_progress, blocked, deferred, and hooked issues.
```

### MEASURED 2026-10-03, bd 0.50.3, both store classes

| store | shape | `bd ready --json --limit 0` |
|---|---|---|
| the shop's queue db (a `beads.db` beside its `issues.jsonl`) | SQLite | 10 rows, **all `open`**; all 5 claimed beads of the moment (`bd list --status in_progress`) absent |
| a hand-written `.beads` with `no-db: true` | JSONL | 1 open row of 2; the `in_progress` row absent |

Both stores, one binary, the same argv. The store class is the axis
`ranger-base-lpz0o` found a real difference on, and this is not it: **neither
class puts a claimed bead in `bd ready`.** So ADR 0004 §2's parenthesis —
"today `bd ready` may include them; the cockpit filters" — is false on 0.50.3
(whether it was ever true of an older bd is not measured here, and nothing
should be read from the wording either way), and the cockpit's filter is a
no-op.

### What it cost (the bead's own evidence, 2026-10-03 16:0x)

Two beads sat `in_progress` with no live session:

- **ranger-base-4mrmc** — holder `holden-posse-ranger-base-4mrmc` retired by
  `posse kill --no-land` at 14:5x, worktree KEPT at be07ed31, assignee free
  from 16:00.
- **ranger-base-mz8ud** — seat reaped at 08:5x while the bead was blocked on
  ranger-base-knikn; knikn closed at 15:0x, worktree KEPT at adb20fb9.

The 16:00 pass hired four other beads including a P3 and mentioned neither.
`posse dispatch --dry-run -n 0 --resume` printed **no line at all** for
either: not "held by", not "resuming", not a skip. ADR 0013's landing clause
promises the opposite — "nothing is stranded by a keep … so closing the bead
lands it on the next pass" — and the workaround was `bd update --status open`
by hand, which throws away the claim the work was done under.

The shape that DID resume earlier the same day (hcn-1/hcn-3 after a
`kill --no-land`, relaunched by a pass filtered to that repo with "already
claimed by erlich — resuming") is not a counter-example and not a store
difference: those rows were `open` with an assignee, which `bd ready` serves,
and `Bd.Claim`'s resume arm then fires on the assignee alone.

### Why the suite was green over it

The fake bd serves `fake-ready.json` **whole** — `in_progress` rows included
(`herdr_test.go`, the `ready` case). 49 test files mention `in_progress`, and
the dispatch ones put the claimed bead straight into the ready fixture, so
every in_progress branch of the fire loop was exercised against a queue real
bd does not produce. ADR 0030's own pins are of that shape.

That divergence is still there, deliberately scoped out of this fix: making
the fake's `ready` drop `in_progress` rows means repairing every fixture that
relies on it, and it is filed as its own bead. What this fix adds instead is
a pin family that drives the bead through `bd list --status in_progress`
ALONE, with `fake-ready.json` empty — the only fixture shape real bd can
produce (`internal/posse/interruptedrun_qa_test.go`).

### The fix

`internal/posse/interrupted.go`: the pass reads the claimed list too, keeps
the rows that are an interrupted run, and hands them to the fire loop, which
is where every guard above already lives. Both queue reads take it — the fire
pass and `refire`'s refill — because a rolling Run under `--watch` may never
return to the first (7h09m in one measured pass, ranger-base-t8tq).

Offered:

- no live session under any of the three join names (`heldSession`: the run
  record, the Dial F name, the pre-Dial-F slot) — killed, reaped, crashed;
- a live session with **no agent** in it — the CLI exited, ADR 0004 §3
  relaunches in place;
- under `--resume` only, a holder herdr calls settled (idle/done) — the
  flag's own population, "re-prompt in_progress beads whose persona session
  is alive and idle".

Not offered, and each for a reason:

- a holder mid-turn (`working`, or `blocked` at a permission prompt): the
  bead is being worked, the cockpit's IN PROGRESS section is where that is
  read, and a line per pass per in-flight bead buries the lines that mean
  something. A settled holder on a pass with no `--resume` is the same
  judgment one rung over: the governance surface's G2 row (`settled:<bead>`,
  govern.go) is the surface that reports a holder which stopped without
  closing, and a per-pass skip line beside it would say the same thing again
  every pass;
- **blocked by an unmet dependency** — subtracted exactly as `Bd.Ready`
  does, and for ranger-base-lpz0o's reason. `bd blocked` lists `in_progress`
  rows: the live queue had ranger-base-4mrmc in it as this was written,
  blocked on an open authorization question. mz8ud sat there until 15:0x and
  is offered from that moment;
- deferred past now (`deferredNow`: the date when there is one, else the
  status — ranger-base-5aln, amended by ranger-base-nkjjg, which measured a
  store class that discards the date `bd defer --until` is given);
- a claim whose assignee is not a lane of ONE at its own holder: no
  assignee, an assignee that loads no PID, or the coordinator (ADR 0033 §2).
  `laneFor` is asked rather than `CanonAgent` so the rule is routing's own.
  Re-laning a claim by LABEL would offer another persona a bead somebody
  else holds — the claim loses (`ClaimLostError`) after a seat and a session
  have been spent on it, and ADR 0020 §2's rule is that an assignment is
  never silently rerouted;
- a claim whose assignee only FOLDS to a persona (`Ranger` → `ranger`,
  rangerhq-c6u6). Every in_progress branch of the fire loop is gated on
  `is.Assignee == persona` as a string, so a folded name would arrive with
  the holder join, the crew shield and ADR 0030's tiebreak all abstaining
  and be claimed afresh under the other spelling. A resume keeps the claim
  it was made under; that would be a re-attribution.

### Verified against the real bd, before and after

Not the fake. An isolated `RHQ_HOME` (one persona PID, `labels: [go]`), one
`no-db: true` JSONL store written by hand holding exactly one row —
`status: in_progress`, `assignee: ranger` — and no live session named for it.
Same fixture, same argv, two binaries, 2026-10-03:

```
$ env -u BEADS_DIR RHQ_HOME=<scratch> posse dispatch --dry-run -n 0 --resume --dir <scratch-repo>
  posse 0.5.0+193c8782 (installed):   no ready work
                                      0 bead(s) would be dispatched
  posse 0.5.0+dev (this change):      · scr-1  → ranger (assignee) in session ranger-repo-scr-1 [standard via PID]
                                      1 bead(s) would be dispatched
```

"no ready work" over a stranded claim is the bead's own complaint, reproduced
outside the shop's store and fixed in the same breath. With an ordinary ready
bead beside it the claim is reported rather than dispatched — `– scr-1
ranger-repo busy this pass` — because one persona takes one bead per repo per
pass (ADR 0028 §3), which is the right answer and was still a line where
there had been none.

### The accepted cost, named

`--resume` can now reach a settled holder from the store, which it never
could before — so `--watch --resume` can re-prompt a **settle-open** bead (a
session that stopped without closing its bead) once per settle, for as long
as the persona keeps settling without closing. That is not a new hazard and
not a new decision: it is rangerhq-zom's own, in the clause that made the
default a skip — "re-prompting every pass is a token-burning loop under
`--watch` … otherwise the operator asks with `--resume`" — and the flag's
help has promised exactly this population since it was written. What is new
is that the promise is true. Two things bound it today and neither is a
counter: the busy map holds the seat until that bead settles (so a pass
cannot re-prompt a bead it is still running), and the settle-open surface
files its own bead for the shape (`settleopen.go`). A pass with no flag
re-prompts no settled holder at all — that population is `--resume`'s alone;
what it does unattended is relaunch a run with no live agent, which is one
prompt into a session that has none.

Cost: one `bd list --status in_progress` per repo per scan, plus one
`bd blocked` when that list is non-empty — 0.13–0.17s each against the
1551-bead queue db, the grain `Bd.Ready` already pays twice.

### Mutation-checked

| mutation | red |
|---|---|
| drop the `interruptedRuns` call in `Run` | the relaunch and both dry-run pins |
| drop the `interruptedRuns` call in `refire` | the refill pin's rolling arm only (the one-shot arm stays green) |
| offer every holder, working included | `…LeavesAWorkedBeadAlone` |
| drop the `bd blocked` subtraction | `…LeavesABlockedClaimAlone` |
| drop the lane-of-one rule | `…DoesNotRelaneAForeignClaim` |
| drop the exact-spelling clause | `…DoesNotReattributeAFoldedAssignee` |
