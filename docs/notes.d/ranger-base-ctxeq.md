# ranger-base-ctxeq — witnessing the Edit/Write wall at the launch, and what a negative probe costs to believe

ADR 0086 D5, as built. The decision is one sentence — **parity proves the
Edit/Write rows by a witness at the launch, not by availability** — and the
build is one new door (`applyWallWitness`, `internal/posse/wallwitness.go`)
called from `checkParityIn` beside `applyRecordReach`. What took the work is
not the create. It is that **this is the first probe in the package that
passes when a command FAILS**, and every other probe here is the other way
round.

## The row it replaces, and the one it writes

`wholeTreeWriteWall`'s seatbelt arm answered `L2 seatbelt` whenever the tier
was available and the runtime did not cage itself. That is a fact about the
HOST. R0 is the two disagreeing out loud: tier available, row printed, and
the render for that PID in a dispatched worktree refused outright. The row
described a launch that could not happen.

So the row is now one of three things, and they never read the same way:

| | Detail |
|---|---|
| the wall refused both creates | `L2 seatbelt (<mech>), witnessed at launch` |
| a create landed | the row LEAVES Realized; Unrealized/Degraded name the path |
| nothing could be measured | `L2 seatbelt (<mech>) — NOT WITNESSED: <why>` |

The third is `recordReachUnmeasured`'s rule one file over (ranger-base-fm4p):
a check that measured nothing and a check that measured a pass must not print
the same line, and a launch must not be degraded by a check that did not run.

**It keys on the ROW and not on the deny list.** `wholeTreeSeatbeltClaim` is a
const `parity.go` writes and `wallwitness.go` looks for. Re-reading
`deniesFileWrite` here would be a second reading of the fact that put the
claim in Realized, free to disagree with the row it is judging — and the rows
are also exactly the right filter: another tier's row, a self-sandboxing
runtime, a path-scoped subtree deny and an already-unrealized rule all carry
a different Detail or none, and the witness has nothing to say about any of
them.

## A NEGATIVE PROBE NEEDS TWO CONTROLS, and this is the whole of the design

Every other probe in this package passes when a command succeeds, so a probe
that cannot run at all is loud. This one passes when a command fails — which
means **anything that stops the create from being attempted reads as the wall
holding**. A stub binary that swallows its argv, a namespace the kernel
refused, an args file that cannot be read, a target directory that is not
there: each of them is a green row over a seat with no wall.

So one create is not the measurement. Three readings are:

1. **the baseline**, outside the wall: posse creates and removes the same path
   itself first. A path this process cannot write unwalled says nothing about
   a wall, so that is NOT WITNESSED and never a pass.
2. **the marker**, inside it: the walled command is
   `echo <marker>; : > <path>`, and a run that printed no marker never reached
   the shell behind the prefix. Also NOT WITNESSED. The marker is NOT spelled
   out of the file name, because a refusal prints the path back in the
   kernel's own diagnostic — a marker derived from the path would be "found"
   in the message that says the create failed.
3. **the file**, and not the exit code. `os.Stat` decides whether the create
   landed. The mutation that reads the exit code instead (`err == nil`) reds,
   because the stub-that-exits-0 arm then reports the write as ALLOWED.

Only a create that did not land, with the baseline passed and the marker
printed, is the wall refusing the write.

Targets are two, not one: cwd, and **the session's git objects dir** (D3's
own, `sessionGitObjectsDir` — `rev-parse --git-common-dir` plus `objects`, in
a linked worktree and a main checkout alike). A witness over cwd alone would
say nothing about W14/W15, the capability D3 removed. A target that is not
there is DROPPED rather than probed, for the reason above: ENOENT is not a
wall.

## The fixture pair: one body, two roots, opposite verdicts

D6 asked for a fixture whose wall does not hold. The honest one turned out to
be the one this shop keeps falling into by accident:

- under **`/var/tmp`** (`wallUngrantedDir`, now in `wall_helpers_test.go`) no
  grant in the writable set reaches the tree, so the production render really
  walls it and the witness must read WITNESSED.
- under **`/tmp`** (`gitTempDir`) `SeatbeltWritable` grants the tree
  read-write to everybody, on both mechanisms, so the same render over the
  same PID leaves the create landing and the witness must take the row away.

Same production code, same PID, and the difference is the wall and not the
test. That is ADR 0086's own caveat about its "move main" row turned into an
arm: a fixture under `/tmp` measures nothing about a wall, so here it is the
arm whose subject IS a wall that does not hold. `sbRoot` could not be reused
— it takes its root from TMPDIR and therefore skips wherever TMPDIR resolves
under `/tmp`, which is every Linux box, including the only host this defect
was reachable on.

Every arm runs in a real linked worktree AND in a main checkout (D6), because
the objects dir is in a different repository in those two shapes.

## MEASURED

The D1 host (linux 7.2.15, bubblewrap 0.13.1, git 2.56.0), 2026-11-11, at a
one-minute load average of 5.44, from this bead's own session worktree. 25
iterations each, over the real `bwrap`, a real linked worktree under
`/var/tmp`, through the production render:

| what | min | median | max |
|---|---|---|---|
| `RenderWall` alone | 9.663ms | 10.579ms | 11.684ms |
| one `wallWitnessRow` (cwd) | 3.882ms | 4.723ms | 5.336ms |
| both targets | 8.960ms | 9.546ms | 10.171ms |
| `wallWitnessTargets` alone (one `rev-parse`) | 0.983ms | 1.264ms | 1.463ms |
| **`applyWallWitness` end to end** | **20.047ms** | **21.934ms** | **26.356ms** |

So ADR 0086's "ASSUMED to be milliseconds" is **MEASURED at ~22ms per launch
of such a PID** on this box, and the ADR's "one exec under the wall" is
really two execs (9.5ms) plus a second render of the same wall (10.6ms) plus
one `git rev-parse` (1.3ms). The darwin numbers are unmeasured.

**THE LIVE READING, over the real hoover PID.** 2026-11-11, load 8.11, from
this bead's own session worktree: hoover's deny list read from this box's
live home (`$RHQ_HOME`), the writable set and carve-out from that home,
rendered through `bwrapRender` with the ARTIFACT written to a temp dir so
nothing of the home is touched, `CLAUDE_CONFIG_DIR` set as the launcher sets
it (D19 refuses the render first when it is absent, and the test binary's
TestMain clears it — that refusal was the first reading this probe got). The
13-entry set is the claude config dir, `<cwd>/.beads`, the store of record's
`.beads` and `.git` in the queue repo the redirect names, hoover's memory
dir, the home's `state`, the gates dir, the home caches and `/tmp`. No
`<cwd>`, no git dir of the session's own repo.

    WITNESSED: the live wall refused a create in
               ~/.posse/worktrees/posse/gilfoyle-posse-ranger-base-ctxeq
    WITNESSED: the live wall refused a create in /home/posse/src/posse/.git/objects
    rows parity would claim: [Edit Write]

The second line is W14/W15's own path — the shared object store of the
checkout this worktree hangs off — refused for the PID whose output is
findings. That is D3 and D5 read together, on the box, against the real PID.

**The second render is the honest residual.** `applyRecordReach` renders this
wall too, and the launch renders it again through `WallPrefix`, so a launch
for a whole-tree-deny PID now renders it up to three times — idempotent (the
same args file, the same prep) and ~10ms each. Threading one `WallRender`
through `checkParityIn` would buy that back; it was not done here because the
two arms abstain at different points (one before rendering, one after) and
the coupling is worth more thought than 10ms.

## The mutation table

Each mutation was applied against HEAD, one at a time, and `go test
./internal/posse -run WallWitness` re-run. **Fifteen mutations, fifteen
reds.**

| mutation | reds |
|---|---|
| the witness is not called from `checkParityIn` | the CheckParityIn wiring arm |
| the objects dir is not a target | the live arm, the targets arm |
| the marker is not read (a create that did not land is a pass) | the abstention arm |
| the baseline is not taken | the baseline arm |
| a landed witness file is not removed | the live arm, the row-away arm |
| a landed create leaves the row in Realized | the row-away arm |
| an abstention degrades the launch | the abstention arm |
| the witnessed row's Detail is unchanged | the live arm, the wiring arm |
| the rows are every realized row, not the ones that claimed the wall | the says-nothing arm |
| `wallRendered` is not asked | the says-nothing arm (self-sandbox) |
| the verdict is the exit code and not the file | the abstention arm |
| a wall that cannot be rendered leaves the row realized | the render-failure arm |
| a render that fails is silent | the render-failure arm |
| ADR 0086 D3 reverted: the deny arm grants `cwd/.git` and the session's git dirs again | the live arm, at R0's own refusal, plus two more |

The last one is worth keeping: the D5 arms red if D3 regresses, and they red
at R0's message over the real path.

## What else moved

- **`wallApplyRefusal`** (`wall.go`): the mechanism-neutral "can THIS PROCESS
  put up the wall it is about to type", `sandboxApplyRefusal` on one
  mechanism and `bwrapApplyRefusal` on the other, keyed through
  `renderWallOn`'s own parameter shape so the arm this host does not have is
  drivable from the other one. It is asked BEFORE the render, because
  rendering the bwrap arm writes files and makes host-side prep.
- **`reachProbeReason`** (`reachability.go`) was a nil dereference on the
  launch path for a probe that exited 0 and printed nothing — which its two
  existing callers could not reach, and this one reaches by construction. It
  says so now instead of panicking. The first run of the abstention arm
  SIGSEGV'd there, which is how it was found.
- **THREE EXISTING PINS LAUNCHED A WHOLE-TREE-DENY PID AT SEATBELT IN A
  `/tmp` FIXTURE**, one per arm, and the witness refused all three — on the
  real `bwrap`, correctly, because the create really lands there. Their
  launch dirs moved to `wallUngrantedDir`; nothing about their subjects
  changed, and each is now clean for the reason it always claimed to be.
  - arm 1: `TestQABwrapLaunchDoesNotMaterializeTheStoreOfRecord` (its
    `bwrap` is the real binary — `bwlStubBin` only pins its path).
  - arm 2: `TestExamplePIDsBindTheSeededSkill`, which launches the shipped
    shelf `architect` on claude and grok with no `--allow-degraded`.
  - arm 3: `TestSeatbeltProfileAndLaunch`, the `security` PID's own pin.

  This is the finding behind the finding. ADR 0086's caveat about its "move
  main" row said a fixture under `/tmp` measures nothing about a wall; three
  pins in this tree were launching PIDs through a wall that was not there,
  and nothing could say so until something tried the write. A witness that
  only ever ran in production would have been a row nobody read; the first
  thing it did was audit the suite.

## Not claimed

- **darwin.** The live arm is mechanism-neutral and skips where the host
  provides no wall, so it runs unasked on a darwin seat and on ci.yml's
  macos-latest leg. Nothing on this box measured it.
- **a write the wall allows somewhere the witness does not probe** — D5's own
  stated limit. D3's list is the complete statement of where it allows
  writes, and `posse gates` prints it.
- **the container tier.** L4 mounts `<dir>/.git` read-write for exactly these
  PIDs and is a different row (`ranger-base-56w67`); nothing here touches it.
