# The 25m ceiling, outrun twice: what the second look measured

ranger-base-pj87l (folding ranger-base-naf7j), the third bead on
`internal/posse`'s wall after docs/notes.d/ranger-base-2ggb.md (the ceiling)
and docs/notes.d/ranger-base-i7fa.md + ranger-base-aupee.md (the two halves of
`t.Parallel`). Read those first; this one only records what is new.

The bead asked for two things: the package inside its clock with headroom that
is not a coin flip, and the reading that justifies the arrangement written
where the next person can compare against it. It also left one question open —
whether `scripts/test-times.sh` should FAIL rather than warn.

## 1. The first finding is that the fix was already written and never landed

`ranger-base-aupee` closed 2026-09-02 with 134 files and the sentence "DONE,
landed 861b0e6". The commit was real, on its own branch, and never became an
ancestor of `main` — laurie's verify pass filed it as **ranger-base-yy1jz**.
So the 1483.4s this bead measured is not a package that outgrew aupee's work;
it is a package that never received it. Anything measured on `main` between
2026-09-02 13:11 and this bead is a reading of the 810-parallel tree.

Re-landed here as a cherry-pick onto this bead's own branch. Priced before
picking, as yy1jz did: three conflicting files of 131.

| file | what the conflict was | resolved |
|---|---|---|
| `credgatecollision_qa_test.go` | main rewrote the file (ranger-base-te3ib); aupee only added `t.Parallel` | main's side, re-marked by the tool |
| `verify_z84xi_qa_test.go` | main un-skipped the FIFO pin (ranger-base-92n5p); aupee's side still had the skip | main's side, re-marked |
| `watchlock_test.go` | both sides had the `RHQ_FAKE_HERDR` filter — main by ranger-base-cecvu, aupee by its own row four | main's side plus aupee's file header |

The lesson worth keeping is the resolution rule: **take main's side of every
conflict and re-derive the mechanical half with the tool.** aupee's side of
those three files was `t.Parallel()` lines and nothing else, and
`cmd/testparallel` regenerates exactly that set — so the merge question was
never "which of two edits wins", it was "which side is the hand-written one".
The tool then re-marked 39 tests, including the ones main had landed in the
23 commits since aupee's base, which a straight pick would have left serial.

## 2. Where the wall actually is now: a census, not a guess

`cmd/testparallel <pkg> envroots` is new here. For every function that writes
the process environment it prints how many top-level tests it holds serial
that **no other root also holds** — the actionable number, because a root
sharing all its tests with another buys nothing on its own.

At the re-landed tree, 198 env roots held 609 of 2179 tests out of any
concurrency, and they were not a long tail:

```
ONLY   REACH  root / variables
67     137    wtqaHome        HOME
60      71    wtApp           HOME
53      53    govRepo         RHQ_BD_BIN
31      36    refreshApp      HOME
27      28    promoteFixture  RHQ_HOME
13      14    idProbeSocket   HERDR_SOCKET_PATH
...     ...   188 more, none over 12
```

Five helpers held 238 tests serial. **Every one of the five was buying a
guarantee some struct field already carries** — which is what ADR 0047 built
and what nothing had gone back to spend:

  - `govRepo`/`pulseIn` set `RHQ_BD_BIN` so `NewBd()` finds the fake bd.
    `HerdrBackend.Bd` is that value as a field; `b.Bd = Bd{Bin: fakeBinFor(t,
    "bd")}` says the same thing to the same code and 53 tests stop being
    serial. The comment warning that without it "the real bd answers from the
    operator's own queue" still holds — the field is read by the same call.
  - `wtApp` set `$HOME` so `DefaultWorktreeRoot()` landed in a temp dir. That
    is `hermetic(t, a)`'s job since ADR 0047 D2, per test rather than per
    process. It now calls `hermetic`, which also hands it the `Load1`/`TopCPU`
    fakes it was silently missing (the ranger-base-w4fb class).
  - `wtqaHome` did the same for the QA half, at 50 call sites. 48 of them go
    through `newTestBackend` → `hermetic` and needed nothing; the calls came
    out. The **two that stayed** are the interesting ones: `sbRoot`, whose
    sandbox pins create and delete operator-owned paths OFF that home
    (`~/.claude.json`, `~/Library/...`), and one init test that is env-tainted
    anyway. A worktree root under $HOME is not the same guarantee as a $HOME
    of one's own, and only two callers wanted the second.
  - `promoteFixture`/`initTestApp`/`seedQAHome` set `RHQ_HOME` and called
    `NewApp()`. `NewAppAt(home)` is `NewApp()` with the home named instead of
    read out of the environment.
  - `refreshApp` set `$HOME` for the meter adapter's credentials-file lookup.
    TestMain gives the binary a temp `$HOME` (ADR 0047 D1), which is the same
    guarantee; nothing in the file writes under `$HOME`.
  - 15 `t.Setenv(EnvPersona, "")` calls were dead: TestMain does it once for
    the whole binary, also ADR 0047 D1. Hoisting a variable to per-binary
    makes every per-test call of it redundant, and nobody swept.

Result: **609 → 338 serial, 1428 → 1630 marked `t.Parallel`**, no test's
assertion weakened.

### The two the census could not clear, and why they are the honest cost

`t.Setenv` is a serial marker as well as a hazard, so the D3 read (ADR 0047)
has to run again over whatever the change newly frees:

  - `TestRefreshMeterWritesNothingAndNamesTheStoreOfRecord` snapshots `$HOME`
    WHOLE and requires it unchanged. That is D3's "asserts the file absent"
    class exactly: on a home shared with 1600 parallel tests it reds on
    whatever else wrote there. It now takes its own `$HOME` with `t.Setenv`,
    which keeps it serial by the runtime's own rule — one test, deliberately.
  - `TestQAWorktreeRootRefusesASymlinkOutOfHome` creates `$HOME/trees` and a
    symlink at `$HOME/scratch`. The rule under test is "under $HOME", so the
    fixtures have to be there; they take `$HOME/<t.Name()>/` instead of the
    two bare names, which is per test without leaving the rule.

## 3. The measurement

Method is i7fa §8's, unchanged, because it is the only one this box supports:
one `go test -c` binary per arm, both arms in the same session, load recorded.

### The controlled pair: one binary per arm, same session, same afternoon

```
arm  tree                                          seconds  result  load at start → end
A    re-land + re-marked filter (1428 t.Parallel)      875  FAIL*     87 52 32 → 79 60 45
C    + the five helpers off the env (1630)             700  FAIL†     40 52 44 → 58 66 54
```

`*` three failures, `†` four; both arms carry the same two — see §5. Every
failure in every run below is one of: the two digest pins main owns, or one
of the three defects §6 found and fixed. The last two runs are green apart
from the two.
C is **0.80 of A** with more load on it, not less.

### The reading the bead actually asked for: `make test`, three times

`go test -timeout 25m ./...`, which is where the ceiling lives and where the
three packages starve each other. Same tree at each of the last two; the
first is one commit earlier (before the `fakeDir` fix, which changed no
timing).

```
run  internal/posse  share of the 1500s budget  load at start  other packages
1           476.952s                       32%     43 59 53     150.3s + 117.9s
2           415.553s                       28%     20 27 37     both cached
3           446.896s                       30%     10 14 21     152.0s + 115.7s
4           506.291s                       34%     10 11 16     170.2s + 125.1s
```

Run 4 is at the tip after the last rebase, three tests further on than run 3
and on the quietest box of the four — which is the honest reading of how the
number grows: the package takes 30 seconds a day even now.

**Against the bead's own reading of the same command on `main`: 1500.984s,
expired, exit 1, no `--- FAIL` line.** And against the standalone 1483.4s it
recorded immediately after.

So the package now finishes in **28-34% of its budget**, and the spread
across a box carrying load 10 to 59 is 91 seconds — 6% of the ceiling. That
is the difference between headroom and a coin flip: the worst of these four
readings would have to nearly triple before the clock came into it again.
The runs also bracket the box: run 1 is a machine with another session's
suite on it, runs 3 and 4 are quiet ones, and the spread between them is
smaller than the spread the package's own growth produced in one hour.

What is NOT claimed: that this is the change's doing alone. Runs 1-3 are on a
quieter box than arms A and C, and the honest attribution is the controlled
pair — A to C is the 0.80. The rest of the distance from 1500s is the
re-land, which was measured on its own bead and never reached anyone.

## 4. The open question: should test-times.sh fail?

**No, and the reason is written in its own header.** `scripts/test-times.sh`
refuses to go red on a wall clock because "a gate that goes red on elapsed
time is the class tvmh and fsil were: a deterministic-looking red that is
really the box's mood". Every number in this file argues the same thing from
the other side: the loads recorded above span 19 to 111 on the same machine in
one afternoon, and a 900-second package is 700 or 1300 depending on who else
is running. A fraction-of-budget failure would have fired on a quiet tree and
passed on a busy one.

But the bead's complaint is right and unanswered by that: the warning fired
correctly on every run for four days and nothing acted on it.

**So the gate goes on the deterministic half.** `make verify-parallel`
(~1s, in `make test`'s prerequisites beside `verify-test-times`) runs
`cmd/testparallel <pkg> check` and FAILS when a test that could take
`t.Parallel` does not. That is the decay that made the package one
1483-second binary: a test lands, is eligible, carries no `t.Parallel`, and
nothing says so until the ceiling arrives four days later. It reads the same
files every time; no clock, no box, no mood. The failure names each test and
the two ways to satisfy it — mark it, or give it a reason in the tool's serial
map.

The slow-package warning now also names the lever (`testparallel`,
`envroots`, `verify-parallel`) rather than only saying "it wants splitting".

**Splitting the package is still the answer eventually, and this bead does not
do it.** 96 non-test files, 50k lines of product code and 107k of tests in one
package: that is a design bead, not a mechanical one. What the census says is
that it is not yet the cheapest answer — 338 tests still hold the serial floor
and they are reachable one helper at a time by a change nobody has to design.

## 5. What is NOT done, and what was filed instead

  - `SLOW_PACKAGE_SECONDS=300` is still not reached and no reading here can
    say whether it would be on a quiet box. That is i7fa §5's finding, and
    two beads later it is still the finding.
  - **338 tests still hold the serial floor**, and after the five helpers the
    census is a genuine long tail: no remaining env root holds more than 13
    tests exclusively, and the largest are `idProbeSocket` and `hermeticGen`
    (`HERDR_SOCKET_PATH`, 13 and 9), `grokPoolHome` and `newVisWallNamed`
    (`HOME`, 12 and 11) and `qApp` (11). `HERDR_SOCKET_PATH` is the next
    single lever worth pricing — it is a `Herdr` field the same way the bd
    binary was a `Bd` one.
  - The 25 tests carrying `t.Parallel` that the filter would not give it
    (i7fa's, unchanged by either later bead). `TestQACostAdapterPriceTableIs
    Consulted` calls `RegisterCostProvider` — a package-level map WRITE —
    while parallel. Filed as **ranger-base-btdvw**, not swept: taking
    `t.Parallel` off a green test is a decision.
  - `TestEveryEmbeddedExamplePIDIsInTheShippedTable` and
    `TestShippedExampleTableCoversEveryVersionInGitHistory` are red on `main`,
    not here: commit 0211551 changed nine example PIDs and never appended to
    `shippedExampleDigests`, whose last update (e045c0d) precedes it. Already
    filed by laurie as **ranger-base-34vqk**; this bead's duplicate
    (ranger-base-c7m63) is closed pointing at it, having added one thing —
    the two pins fail identically on the tree before and after this diff.
  - `TestClosedDirtyFilesOneBeadForTheCloserAndOnlyOne` failed in arm A and
    was filed as a flake (ranger-base-1y3dp) before §6 found the cause. The
    bead carries the answer and is closed; filing it first was still right —
    a one-run failure with its rate written down is what let the second
    instance be recognised as the same thing an hour later.
  - ranger-base-9l77f (the lock tests) and ranger-base-ehllm (the backup
    pin's flipped byte) are both still open and still not this bead's.

## 6. What the suite caught that the filters could not, and it is the good part

Three defects, all mine, all found by running the thing rather than by
reading it. They are worth writing down because each one is a rule.

**1. A second `t.Setenv` in a test is not the redundant one.** The sweep that
removed 15 dead `t.Setenv(EnvPersona, "")` calls took three live ones with
it: `TestCrewMarkedByOperatorPromptOnly` and `TestCrewMarkMissedIsReported`
set the persona to `coordinator` and then clear it — the clear is the ARM
SWITCH, not a redundant hermetic. `TestTheRunbookQuotesTheRefusalsRefresh
ActuallyGives` is the same shape. Two of the three reded in arm C; the third
would have gone on measuring the wrong arm silently. The rule the sweep
should have carried: **a per-test env write is redundant only when it is the
FIRST one in its test** — which is aupee's own lesson (hoisting a variable to
per-binary changes who reads it) read from the other end.

**2. `fakeDir()` is not `fakeDirOf(t)`.** Three sites in
`closeddirty_test.go` read a bd call log through the no-argument `fakeDir()`,
which since ADR 0047 D1 answers **the binary's own directory** in the parent
— shared by every test — because `$RHQ_FAKE_DIR` is no longer set there.
Once the tests around them went parallel, two of the three failed in two
different full runs, and both failures read as the product not doing its job:
*"no closed-dirty create in the bd log"*, *"the sweep filed the handoff at
nobody"*. The `fakeDirs` map IS partitioned by test; this call is the one way
to read past the partition. Now a **fourth filter** in `cmd/testparallel`,
and the only one whose violation `check` treats as fatal rather than a note
— a parallel test reading the shared log is a live defect, not a legacy
decision.

**3. `-count>1` was broken by the per-test keys, and repeat runs are how this
shop measures a flake rate.** `go test -count=3` over the closeddirty family
gave five FAILs that were green at `-count=1`. Both per-test keys were on
`t.Name()`, and `-count=N` gives N live tests one name — resumed together
under `t.Parallel`, each one's `Cleanup` deleting the other's entry, each
one's session tree landing on the other's path (*"exists and is not a git
worktree"*, a fixture collision wearing a product refusal's clothes). Fixed
by keying on the thing that is unique per RUN: `fakeDirs` on the `*testing.T`
itself, and `hermetic`'s worktree root on an atomic counter. Green at
`-count=3` and at `-count=2` over the 11 tree-cutting families.

And the tail of that fix is its own lesson: **the atomic counter immediately
cost 722 tests their `t.Parallel`**, because filter 2 counts any written
package-level var as shared state and every test calls `hermetic`. Caught by
`make verify-parallel` in the same minute, one commit later, exempted by name
with the argument written beside it (`atomic.Int64`, `Add(1)` only,
concurrency is the type's whole job). A gate that runs on every `make test`
found in seconds what a census would have had to be re-read to notice — the
best argument for §4's decision that this file has.
