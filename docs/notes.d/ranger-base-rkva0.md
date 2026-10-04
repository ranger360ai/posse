# ranger-base-rkva0 — verifying four closes, and the two arms their mutants could not reach

QA verify of ranger-base-pm5zo, ranger-base-9c5bh, ranger-base-jqe3b and
ranger-base-1a0hi. All four VERIFIED on their own acceptance. Every mutation
each closer listed was re-cut and reproduced; this file holds the arms that
went BEYOND those lists, and the two that survived.

Environment for everything below: this worktree at 3f869735, go1.26.5,
git 2.50.1, darwin 25.4.0, bd 0.50.3, box load 2.3–4 with both suite slots
free. Mutants are `go test -overlay` against a scratchpad copy unless a row
says in-tree; every in-tree mutant was restored from a golden copy taken
first and checksummed back.

## The closers' own mutants, re-cut

| close | mutant | reproduced |
| --- | --- | --- |
| pm5zo | `staleIndefinitePark` → `return false` | yes — every LOUD arm, no quiet or dated one |
| pm5zo | the clock → `is.Created` | yes — every QUIET arm plus the pulse control |
| pm5zo | `>= max` → `> max` | yes — `exactly_the_horizon` alone |
| pm5zo | the `Updated.IsZero()` abstain dropped | yes — `WithNoClockStaysQuiet` alone |
| pm5zo | the `DeferUntil != nil` guard → `if false` | yes — `DatedPark…/future_date` only |
| pm5zo | row key `parked:` → `question:` | yes — every key assertion |
| 9c5bh | the settled branch deleted | yes — the race pin, on the re-prompt assertion |
| 9c5bh | `skipSettled` back in the table | yes (in-tree) — the table pin, naming it |
| 9c5bh | `skipBudget`'s call site made a bare string | yes (in-tree) — naming `skipBudget` |
| jqe3b | the restore back on the commit arm alone | yes — both new landing pins |
| jqe3b | the cockpit's `c.disp.Err` assignment dropped | yes |
| 1a0hi | `deferredNow` → the date-only predicate | yes — the NEW dateless arm red, the dated arm green |
| 1a0hi | `treepins:` deleted / bare `go` / `-run` filter / rule removed | yes, all four, each with its own message |

The two mutants of pm5zo that move in OPPOSITE directions (`return false`
and the `Created` clock) are caught by DISJOINT arm sets, which is what says
the horizon is a window and not a one-sided skip. Confirmed cell for cell.

## What was added past them

**ranger-base-1a0hi's escape, reproduced both ways.** The pre-fix
`workprompt_test.go` arm really was decorative: `git show 12b35db0^` of that
file, overlaid together with the date-only `deferredNow` mutant, stays
**GREEN**; the post-fix arm under the same mutant is **RED**. Both measured.

**`make treepins` queues, by execution and not by reading.** With a scratch
`POSSE_SUITE_LOCK_DIR`, `POSSE_SUITE_SLOTS=1` and slot 1 held by another
process: `make treepins GOBIN=/usr/bin/true` printed `waiting for suite lock
held by another worktree`, then `slot 1 of 1 acquired after 37s`. And
`suite_lock_wanted` was driven over nine spellings — the Makefile's own
recipe argv, `./internal/treepins{,/,/...}`, `-count=1` first, an absolute
path, `go vet`, `./internal/posse`, `./...`, and the filtered arm-tags door —
and answers SLOT for exactly the unfiltered ones.

**The by-door census, re-run.** 1,114 transcripts: 124 unfiltered bare
`go test ./internal/treepins` segments from 49 sessions, 71 bare `go vet`.
Corpus drift from the close's 122/65/48, same instrument, same headline. The
one wrapper hit is post-fix text. NOTE A TRAP: the verifying session's own
`bd comments` bodies and mutation-harness lines are IN the corpus by the time
anyone reads the number back.

**ranger-base-9c5bh finding 3's claim, graded.** "A mutant that drops any of
the three `is.Assignee == persona` conjunctions leaves every pin in this
package green": all three dropped individually, each against a **FULL arm 2
run** (238s / 224s / 236s, one suite slot each) — all three SURVIVED. The
claim holds. LIMIT: arms 1 and 3 were not re-run per mutant.

**jqe3b finding 2's "fourth slice" closed by census.** Every writer in
`internal/posse` that can resolve to a process stream was enumerated and
dispositioned: the four package vars are all four of `RouteProcessNotices`'
assignments; `HerdrBackend.Warn` is ws20a's; a launch's `o.Warn` falls back
to it through `warnWriterFor`; `warnw(warn)` is threaded from that same
writer; `GovInputs.errw()` defaults to `io.Discard` and the cockpit passes
`io.Discard` anyway; `App` retains no writer at all (`newApp`'s stderr is
used once, before the alt screen exists). **There is no fifth slice.**

**pm5zo's population, re-measured.** `~/src/hcn/.beads/issues.jsonl` holds
exactly five dateless parks — hcn-4/7/11/12/15 — every one `status:
deferred`, `defer_until` absent, carrying `question`, stamped
2026-10-03T15:05–15:48. So the row is reachable for the whole population it
was built for (`OpenLabeledAny` drops only `closed`), and the close's
prediction is dated: they go loud 2026-10-17 15:05–15:48.

## The two arms nothing reached, and the pins that now hold them

### 1. A forgotten park was one `continue` away from being two rows

`internal/posse/govern.go` — the G3 park row ends in `continue`, and nothing
asserted it. Cut it and ONE bead produces TWO G3 rows:

```
G3 row 1: key="parked:q-forgotten"   "q-forgotten parked with no end date 40d ago and nobody came back"
G3 row 2: key="question:q-forgotten" "q-forgotten open 2376h00m unanswered"
PULSE LINE: "parked:q-forgotten; question:q-forgotten"
```

`-run 'TestGovG3|TestGovPulseLine'` is **ok** over that. The second row is
verbatim the rendering the park row's own sentence exists to replace — a
reader meets "open 2376h00m unanswered" about a bead `bd list` shows ❄
DEFERRED with no date, which is the ranger-base-nkjjg symptom the close
argued the new sentence prevents. Both rows reach the pulse, which
fingerprints keys, so it is two findings for one bead.

Why no arm could see it: the park family all read the FIRST G3 row
(`find(set, "G3")`), and no arm in `govern_test.go` counts rows.
It is reachable for every forgotten park, because `Updated` is never before
`Created` — a park past `attn_parked_age` is necessarily past
`attn_question_age` too.

PINNED: `TestGovG3AForgottenParkIsOneRowAndNotTwo` (govern_test.go, arm 1).
Red with the `continue` cut; green under the unrelated `>= max` → `> max`
mutant, so it is about the row count and not the horizon.

### 2. The guard the fix introduced, over-granting

`internal/posse/generatedindex.go` — `restoreGeneratedIndex` falls through to
`os.Remove` only when git's own answer is `??`. Drop that guard and
`-run 'TestLanding|TestTheLanding'` is **ok**: nothing held the condition the
fix had just added.

The other reason is reachable, and MEASURED 2026-10-04 with
`.git/index.lock` present: `restore --source=HEAD --staged --worktree` exits
**128** writing nothing (`Unable to create …index.lock: File exists`), while
`status --porcelain --untracked-files=all` still exits **0** and answers
` M`. So the guard is handed a tracked path and a failed restore in one
breath — the exact pair it must tell apart — and without it the function
deletes a TRACKED file whose bytes `restore` was never asked to put back.

PINNED:
`TestQARestoreGeneratedIndexDeletesNothingWhenTheRestoreFailedForAnotherReason`
(landgeneratedindex_test.go, arm 2), three arms: tracked → RESTORED,
untracked → REMOVED (jqe3b's own half, driven on the function directly),
tracked under a held `index.lock` → LEFT. The two that already worked are
what make the third evidence. Each arm asserts its own premise, so a fixture
that stopped reaching the branch reds instead of passing. Red for the `??`
guard dropped (third arm only) and for the `os.Remove` fallthrough dropped
(second arm only) — disjoint.

## Method notes

**`go test -overlay` CANNOT reach a pin that reads SOURCE FROM DISK.** Two of
ranger-base-9c5bh's three mutants read SURVIVED under an overlay and KILLED
in-tree: `TestQAEveryCountedSkipReasonHasAReportingSite` parses
`refillreport.go` and `dispatch.go` with `go/parser` at run time, and the
overlay is the COMPILER's view only. Any AST/corpus/tree-wide pin is in this
class. A contradicted close is the signal — re-cut before writing it down.

**A mutant whose `-run` selected nothing reads exactly like a survivor.** A
harness that derives the package from the mutated file's directory sends a
Makefile mutant to the repo-root package: `? …/posse [no test files]`, exit 0,
four "survivors" that measured nothing. Count `=== RUN` lines and refuse the
verdict at zero.
