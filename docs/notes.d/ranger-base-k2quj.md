# The wall abstention reaches all 55 members now (ranger-base-k2quj)

2026-11-22, linux 7.2.15 x86_64, go1.27.3, bwrap 0.13.1, 16 cores. The three
live findings of the ranger-base-2unmh verify, which verified four closes
(ranger-base-evx07, -zbmxc, -byqv0, -ayubz) and handed on what outlived them.

## FINDING 1 — the abstention was routed in 1 of 24 member files

ranger-base-evx07 landed the criterion (`bgSeatEndedFromOutside`) and the
abstention (`bgSeatAbstain`) and routed them in `bwrapgitrefslive_test.go`
alone. Measured against the check's own definition of the set —
`wallVenueMemberPattern`, read rather than retyped — the set is **55 members
in 24 files**, 8 of them in that file and 47 in 23 files that kept their own
seat runners with no abstention at all. evx07's close said exactly that and
filed nothing.

What it costs: a seat the box ends says NOTHING and exits 128+9 or 128+15,
and evx07 measured bwrap rendering both as an ordinary status. An unguarded
runner reduces that to `err == nil`, and both polarities are then wrong — a
refusal arm shaped `if ok { fail }` reads it as the wall refusing (a FALSE
PASS, ADR 0086 D5's open obligation), and a success arm reads it as the red
ADR 0097 D3 reports as exit 1 and ADR 0088 D5's ratchet files as one P1
`-l code` bead against the NEWEST LANDING's persona. That class has done it
four times — ranger-base-a688i, -w6kke, -axt1f, -s9h52 — and none of those
landings had anything to do with it.

### the fix is one routing door, not 33 pairs

`bgSeatAbstainIfOutside(t, what, err, out)` (bwrapgitrefslive_test.go, beside
the criterion) is the guard and the skip spelled once. **33 bwrap seat runs**
across the 16 member files that have any, and almost none share a runner:
most are a local closure (`try`, `read`, `mv`) or an inline `CombinedOutput`
in the test body. Three lines at 33 sites is 33 chances for the guard and the
abstention to come apart. `bgSeatEndedFromOutside` and `bgSeatAbstain` now
have exactly one caller each — that wrapper — plus the criterion's own two
pins, which call the decision function directly because what they grade is
the decision.

Four runners needed more than a line, and each is in the notes because the
shape repeats:

* **a seat that is `exec sleep 60` and killed on a defer** (two arms of
  `wallclaudesessionstate_qa_test.go`, both arms of
  `wallwatchbackstoplive_linux_test.go`). The verdict there is a file landing,
  not an exit, so there was no status to read at all. Each grew ONE waiter
  goroutine, and the fatal path reads it **non-blocking and before this
  process's own `Kill`** — after the kill every run answers `signal: killed`,
  which would abstain on every real "the seat never ran".
* **a 90s deadline the test enforces itself** (`wallclaudeprojects`,
  `wallclaudeprojectsattack`). The abstention is routed on the `<-done` arm
  only: the timeout arm kills the seat from this process, and a deadline that
  really elapsed is a finding and not an absence of one.
* **a builder** (`mcpstrict_live_test.go`'s `walled` returns the `*exec.Cmd`
  and runs nothing). The routing belongs at the three reads that take `err`
  as the wall's verdict, not in the builder, which has no status.
* **`exec.Command(args[0], …)`** — a program named by a variable, invisible to
  any census keyed on the `bwrap` literal. Routed, and carried by the register
  below.

### the pin, and what it is graded on

`TestQAEveryWallMemberSeatRunRoutesTheAbstention`
(`internal/posse/wallseatabstain_qa_test.go`, arm 1, `make pkgcensus-check`)
derives the member files from `wallVenueMemberPattern`, finds every
`exec.Command("bwrap", …)` / `exec.CommandContext(ctx, "bwrap", …)` in them,
and requires the **innermost enclosing function that RUNS something** to
route the abstention — stepping outward through builders, which is what makes
mcpstrict's shape readable. A function that calls `bgSeatEndedFromOutside`
directly is grading the criterion and is exempt, derived rather than listed.

It cannot be graded live: the guarded branch is reached only by a real silent
supervisor kill, which no fixture manufactures inside a runner (MEASURED under
ranger-base-2unmh — deleting both call sites in the one converted file left
arm 1 green, unfiltered). So it is graded by mutation of the real tree.

| mutant | pin |
|---|---|
| delete the routing call from `wallseatcache_qa_test.go` (1 call) | RED, 1 unrouted |
| from `wallunixlive_test.go` (3) | RED, 3 unrouted |
| from `bwrapgitrefslive_test.go` (2) | RED, 2 unrouted |
| from `mcpstrict_live_test.go` (3) | RED, 1 unrouted (the builder's caller) |
| delete a ROUTED register row's call (`bwraphide`, `wallwatchbackstoplive`) | RED, in both arms |
| give the EXEMPT register row a routing call | RED |
| add a new variable-program seat run no row names | RED |
| rename the test a register row points at | RED twice (unnamed site + stale row) |
| the real tree | GREEN |

All MEASURED 2026-11-22. `TestQAWallMemberAbstentionRuleMatchesWhatItClaims`
drives the same rule over 17 planted sources, positive and negative, which is
what catches the seed going quiet while the tree is clean — it enumerates
nothing, so it is not in the tree-wide class and needs no door.

### the hole, named rather than covered

The rule reads the `bwrap` LITERAL, so a seat run whose program is a variable
is invisible to it. `wsaVariableCommandRegister` carries those: 4 member tests
today, three ROUTED and one EXEMPT. The exempt one is
`TestLiveWallGrokLeaderUseLeaderTUIStartsNoLeader`, and it is the one shape
where routing would be WRONG — the program is util-linux `script` and THIS
TEST kills it on both arms by design, so every run ends `signal: killed` and
the criterion cannot tell that from the box's kill. Routing it would abstain
always, holding the wall check at exit 3 forever, which is the one direction
ADR 0097 D5 reports as silence rather than as a wall regression. Arm 2 reds on
a variable-program site no row names and on a row that names nothing, so the
hole cannot grow unread.

**Not widened to non-bwrap seat runners in general.** The two
`wallwatchbackstoplive` arms are in because they are seat runners with the
same false red; the 8 member files with no `exec` at all have nothing to
route.

## FINDING 2 — `CageBeadsDir`'s LoadEngine-error arm is witnessed now

`internal/posse/cage.go`, `CageBeadsDir`: the first fallback (LoadEngine
fails, answer `beadsHome`'s home) had no pin. MEASURED 2026-11-22 under
ranger-base-2unmh: patching it to return `""` survived arms 1-5 of
`cagebeadsdir_qa_test.go`, `beadshomespelling_qa_test.go` and
`beadsdirenv_qa_test.go`'s five arms under `-tags posse_arm3` — every pin over
the subject. It was left that way deliberately, as the control proving the
three pins that bead DID add are narrow.

The finding offered two honest options: a pin that drives an unresolvable
engine, or a comment that stops asserting a state the suite cannot reach. The
pin wins, because the state is reachable with no container and no mount set:
`default_engine:` is config, and `LoadEngine` answers `Die` for a name that is
neither built-in nor a `cages/<name>.yaml`. That is what a reader calling this
function on a misconfigured home actually gets.

`TestQACagedBeadsDirFallsBackToBeadsHomeWhenTheEngineDoesNotLoad` (arm 6 of
`cagebeadsdir_qa_test.go`) asserts its premise both ways — the store IS
covered, so a green cannot be arm 5's answer arriving by the other door, and
the engine really does stop loading — then requires `beadsHome`'s own
spelling. Re-running the 2unmh mutant: **RED on this arm alone**; the
`beadsHome` pins and all five arm-3 `beadsdirenv` pins stayed green, which is
the finding reproduced and the gap closed.

`""` is the one value worse than an unresolved one: `-e BEADS_DIR` forwards
it, and a `--no-db` bd with nothing there resolves `$cwd/.beads` and writes
the container's own throwaway `issues.jsonl` — the silent record loss ADR 0055
D1 exists to prevent.

## FINDING 3 — the live L4 reading is still not taken

ranger-base-ayubz's DONE WHEN asked for a before/after on a live L4 seat with
a store behind a symlink. Its close declined honestly and handed it to the
verify lane; the verify lane could not take it either, and neither can this
one. MEASURED 2026-11-22 from this seat, the third reading in a row to say the
same thing: `docker info` exits 1 with `permission denied while trying to
connect to the docker API at unix:///var/run/docker.sock` — uid 1001(posse)
against a socket owned by a group this uid is not in.

Everything else about that bead is held by the suite, including the namespace
fact the fix rests on, measured under bwrap as a labelled stand-in
(docs/notes.d/ranger-base-ayubz.md). What is missing is one reading: with a
`.beads/redirect` behind a symlink, confirm a caged `bd comments add` lands in
the store rather than in the container. It needs a box where the engine is
reachable, or the operator's own hands. Filed as **ranger-base-wkffs**
(`-l debt -l question`, unassigned, `discovered-from` this bead) rather than
left on a closed bead's comment, carrying what to measure and its stopping
condition — which is either the reading or a ruling that this instance will
not have a reachable engine, so the obligation can be discharged instead of
handed on a fourth time.
