# Four closes, and the clock arithmetic nobody had a positive control for (ranger-base-dutum)

Verification record for ranger-base-dutum: the closes of ranger-base-kbwdf,
ranger-base-gt0q9, ranger-base-jvwcb and ranger-base-6vvgs. MEASURED
2026-11-04 in `~/.posse/worktrees/posse/holden-posse-ranger-base-dutum` at
HEAD 3f4299f9 (= `main`), in the real seat environment with the launch pin
present — `GIT_CONFIG_GLOBAL` naming the operator's resolved global config file,
`GIT_CONFIG_SYSTEM=/etc/gitconfig`, no `env -u`, which is the condition that
reds a dispatched seat. Load 12–45 over the session; both suite slots were
held by sibling seats throughout, and every run here was `-run`-filtered and
so took no slot.

All four verified. Three prose findings recorded under ADR 0006 §6; no live
findings, no bead.

## §1 — the mutants that decided it

Each fix was reverted in place and the arm re-run, md5 checked before and
after so a no-op edit could not score as a kill.

| mutant | arm | result |
|---|---|---|
| `git checkout fe39111a^ --` the three loi28 test files | `-run TestDeriveIdentityLiterals ./internal/posse` | FAIL, `visibility_test.go:1042: no configured email, no .beads at all — must not derive email` |
| the same | `-run TestQAHookFreshness ./internal/treepins` | FAIL, every finding `prepare-commit-msg is STALE — it does not match this binary's render`, the bead's named tests among them |
| `wallSince` returns `+ 2*time.Hour` | `-run 'TestCostPlan\|TestCostFooter\|TestCostSandbox' ./cmd/posse` | exactly 6 SKIP, each naming its line and `the box's wall clock gained 2h0m0s` |
| `skipIfTheBoxSleptMidTest(t, at.UTC())` at costplan_test.go:625 | `-run TestCostPlanHintSurvives… ./cmd/posse` | FAIL, `needs a reference straight from time.Now()` |
| `git checkout 2c840e5d -- internal/posse/herdr_test.go` | `-run TestQAGoCachesDoNotFollowTheTempHome` | FAIL twice at `gocachetemphome_qa_test.go:49`, naming GOMODCACHE and GOCACHE |
| the same | `GOPROXY=off -run TestPosseSupportsBdWriteOnARealBinary` | FAIL, `go: downloading golang.org/x/{term,sys,text}` — the three modules and the failure of CI run 41811904569 |

Unmutated: 3/3 identity tests PASS, 26/26 `TestQAHookFreshness` PASS (99s),
9/9 of the costplan family PASS (15.6s), the hide-direction pin PASS, the
offline bdwrite test PASS. `make fmt-check`, `make notes-check` and
`make tree-check` (99s at load 21–45) green; `go run ./cmd/checkorphans`
clean.

## §2 — the positive control the gt0q9 close did not have

ranger-base-gt0q9's own control arms were a temporarily negative tolerance
(the guarded tests SKIP) and a stripped reference (FAIL). The first proves
the skip path is reachable but makes `j > tolerance` trivially true, so it
exercises no arithmetic. **Inflating `wallSince` by two hours is the arm that
does**: it is what a box sleep looks like to this code — the wall clock gains
time the monotonic clock did not — and under it `wallJump` reads 2h0m0s and
`skipIfTheBoxSleptMidTest` short-circuits all six guarded launches before
their first assertion.

That run also settles, by measurement rather than by prose, which launches
have no wall-clock premise: under the same 2h gain,
`TestCostSandboxHoldsAgainstAnInheritedClaudeConfigDir`,
`TestCostPlanFetchesThroughTheSeamAndNamesItselfInTheReadLog` and
`TestCostFooterDoesNotGrowTheCodexHint` all still PASS. Those are the three
unguarded launches in the file, and all three are correctly unguarded.

The box sleep itself re-measured independently, `pmset -g log`:

```
2026-11-04 01:50:26 -0500 Sleep   Entering Sleep state due to 'Low Power Sleep':TCPKeepAlive=inactive Using Batt (Charge:1%) 10262 secs
2026-11-04 04:41:28 -0500 Wake    Wake from Hibernate [CDNVA] : due to pwrbtn/UserActivity Assertion Using AC (Charge:17%)
```

2h51m02s to the second, and 2h51m02s + `agedSeed` (3m30s) = 2h54m32s, which
`BlindFor` prints as `2h54m` — the age the bead reported, to the minute.

**The one clock direction the close does not name is backwards.** The guard
fires on `j > time.Minute`, so a backward wall step is never a skip, and
`agedAges` now derives its upper bound from `wallSince`: a backward step past
30s puts that bound (2) below the loop's lower bound (3), `want` comes back
empty, and `wantOneOf` then reds with an empty want list. It reds rather than
passing — `wantOneOf` falls through its loop to `t.Errorf` when `want` is
empty, so this is not a check that passes by producing nothing — but the
message names no wanted value. Not filed: unmeasured, and the pre-fix
monotonic bracket could not go backwards, so it is a direction the switch to
wall arithmetic opened rather than a defect it shipped.

## §3 — `remote --no-verbose prune` also deletes the ref

MEASURED 2026-11-04, git 2.50.1 (Apple Git-155), plain git in a scratch repo,
with a positive control (`remote prune origin` deletes) and a negative one
(`status --porcelain` does not):

```
POS CTRL remote prune           rc=0    REF DELETED
remote -v prune                 rc=0    REF DELETED
remote --verbose prune          rc=0    REF DELETED
remote -v -v prune              rc=0    REF DELETED
remote -vv prune                rc=0    REF DELETED
remote --verb prune             rc=0    REF DELETED
remote --no-verbose prune       rc=0    REF DELETED      <- not on c1pqo's leg list
remote -v update --prune        rc=0    REF DELETED
remote -- prune                 rc=129  ref intact       error: unknown subcommand
remote -v -- prune              rc=129  ref intact       error: unknown subcommand
worktree -v list                rc=129  ref intact       error: unknown switch `v'
NEG CTRL status                 rc=0    ref intact
```

Every spelling ranger-base-jvwcb and ranger-base-c1pqo name reproduces.
`--no-verbose` is the one they do not: `git remote -h` spells the option
`-v, --[no-]verbose`, so the negated long form is a real top-level option and
it prunes. ranger-base-c1pqo's fix shape — classify on the first word that
does not start with `-` — covers it by construction; what is short by one
measured spelling is its **owed pin's** REFUSED leg list, which matters
because a later narrowing of the classifier back to literal `-v`/`--verbose`
would leave that list green (the `[[literal-option-tables-lose-to-prefixes]]`
class). It belongs on c1pqo's legs, and ranger-base-ks6a2 is the bead that
runs them.

`git remote` has exactly one top-level option and it takes no value
(`usage: git remote [-v | --verbose]`, "must be placed before a subcommand"),
so skipping leading option words cannot swallow a subverb — c1pqo's stated
premise, confirmed. `git worktree` has no top-level options at all, so
`remote` is the only affected verb in `hookSkipReadSubverbs`.

**No live exposure on this box.** The read arm exists only at 72f5fa69
(`posse/dinesh-posse-ranger-base-ap4p6`, still that tip); `main` carries no
`hookSkipReadSubverbs` and no `posse_hs_readverb`, and the installed shim at
`~/.config/posse/state/gates/holden/bin/git` contains neither — probed by
execution, `git -c core.hooksPath=/dev/null remote -v prune origin` and
`… status --porcelain` are both refused `hookskip:config-override`. The
containment holds in the graph too: ranger-base-hogeb depends on
ranger-base-c1pqo via `blocks`, read back.

## §4 — the gate was already green when the 6vvgs close said it was not

bd renders comment timestamps in UTC; the close times in a verify bundle are
local. Lining the two up:

```
10:51:03Z  ci.yml GREEN on main, e30e6b73, run 41816913260
11:12Z     posse ci-watch comments on ranger-base-6vvgs: "ci-red cleared"
11:30:10Z  ci.yml GREEN on main, fe39111a, run 41821383467
11:56Z     gilfoyle's close comment: "origin/main is still bdd3954b, the red tip"
```

`origin/main` is fe39111a — `git ls-remote origin main` agrees — and a ci.yml
run on main is a push run, so fe39111a was on origin/main by 11:30:10Z, 26
minutes before the comment that says it was not. The mechanism is a stale
remote-tracking ref in that worktree. The bead's DONE WHEN ("ci.yml is green
on main again") was met; the close's own prose denies it, and contradicts a
comment already sitting on the bead.

Re-measured at verify time, because a close whose deliverable is a STATE has
a shelf life: the newest ci.yml run on main is still the green one at
fe39111a. The two later commits — 2c840e5d (gt0q9) and 3f4299f9 (6vvgs) — are
local only, so **CI has never run the macos half of this fix**. What stands
for it is §1's offline pair: pre-fix FAIL on exactly run 41811904569's three
modules, post-fix PASS.

## §5 — rebases, and what the patch-ids mean here

Neither landed commit matches its close's named sha under
`git patch-id --stable`, and both are clean rebases anyway:

- gt0q9: cae8d2d0 → 2c840e5d. The whole difference is
  `docs/notes.d/README.md` context; `cmd/posse/costplan_test.go` is
  byte-identical.
- 6vvgs: 816f2635 → 3f4299f9. The notes index again, plus a different base
  blob for `internal/posse/herdr_test.go` — 1493a31e (ranger-base-rviq5)
  landed in between and touched the same file. The hunks gilfoyle added are
  unchanged; the merged result carries both.

So a differing patch-id does not refute a landing when the commit regenerates
a generated index: diff the two diffs and read what moved before concluding
anything.

## §6 — recorded findings (ADR 0006 §6 · no bead filed)

1. `bd:ranger-base-6vvgs`, gilfoyle's close comment 2026-11-04 11:56Z, last
   paragraph · "origin/main is still bdd3954b, the red tip" and "ci.yml is
   not green yet" were both false at close time; see §4 · escaped from
   ranger-base-6vvgs.
2. `bd:ranger-base-gt0q9`, jian-yang's close comment, "Guarded, by premise
   and not by reflex" · the exclusion list names 2 of the 3 unguarded
   launches in `cmd/posse/costplan_test.go`; the third is
   `TestCostPlanFetchesThroughTheSeamAndNamesItselfInTheReadLog`
   (costplan_test.go:496), which is correctly unguarded (§2) · escaped from
   ranger-base-gt0q9.
3. `bd:ranger-base-kbwdf`, jian-yang's close comment · the `-run` filter
   quoted as the evidence,
   `'TestDeriveIdentityLiterals|TestQAVisibilityWallsEveryScopesEmail'`, has
   no test matching its second alternative (it is
   `TestDeriveIdentityLiteralsWallsEveryScopesEmail`). `go test -list` over
   that filter names 3 tests, all from the first alternative, which is a
   prefix of all three — so the conclusion stands and nothing was measured
   wrongly. Recorded because a filter alternative that matches nothing is the
   class this shop tracks: `ok` over no test looks exactly like `ok` over a
   passing one · escaped from ranger-base-kbwdf.

None of the three changes what a process does or what a pin proves, so each
is RECORDED and no bead is filed (ADR 0006 §6, operator ruling 2026-09-06).

## §7 — the limit of this verification

Filtered runs plus `make tree-check`, not a full `make test`. Both suite slots
were held by sibling seats for the whole session and three queued arms would
have bought nothing these four beads need: kbwdf changed no code, jvwcb
changed no code, gt0q9's diff is one `cmd/posse` test file and 6vvgs's is two
`internal/posse` test files, and every test either bead names was run here
with its fix reverted and restored. The full-suite verify of the landing all
four sit on is filed separately as ranger-base-1agm6, and gilfoyle's own
`make test` at fe39111a is on ranger-base-6vvgs.
