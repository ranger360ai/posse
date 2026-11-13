# Only a launch writes a persona's gates dir (ranger-base-6s5mo)

`posse gates <persona>` is the command AGENTS.md points a seat at to READ its
wall. It was not a reader. It re-rendered the named persona's shims and wall
as a side effect of being asked to describe them, from the environment of
whoever typed the command — and the git shim bakes the launcher's HOME in as
a literal and refuses any git whose HOME is not it (rule
`hookskip:config-env`, ADR 0079 D1, ranger-base-r3pri). So
`HOME=/elsewhere posse gates <p>` left a RUNNING seat's git refused by its
own gate.

MEASURED 2026-11-11 by holden on a cage:shims seat (ranger-base-atq78, the
design bead): 26 FAIL in that seat's `make test`, 24 of them in cmd/posse,
every one a fixture's own `git init` refused — and `posse gates` exited 0 with
no output line saying anything had been written. hoover's security half
(2026-11-12): breakage only, no new capability for a seat, because on the
seats where the re-render lands the L1 git rail is cooperative by declaration
anyway. P2, public.

## The four writers that were on the report path

MEASURED 2026-11-12, this tree, linux 7.2.15, a scratch `RHQ_HOME` with a
`cage: seatbelt` persona and a `.beads` store in the cwd. Two runs of
`posse gates builder` with two different `HOME`s, from a bwrap-caged seat:

| writer | what it rewrote | reached from |
|---|---|---|
| `RenderGates` (cmd/posse, the `gates` case) | `bin/*`, `shell/bash` | every run |
| `BwrapReport` / `SeatbeltReport` → the render | `bwrap.args`, `bwrap.txt`, `seatbelt.sb`, plus the host-side prep | every run at `cage: seatbelt` |
| `applyRecordReach` (reachability.go) | the same artifact and prep | once per runtime in the catalog, where the cwd has a store |
| `applyWallWitness` (wallwitness.go) | the same | once per runtime, where the PID has a whole-tree write deny |

Before: `bin/git` went from sha256 `5ad96d62…` to `5c971c5d…` between the two
runs, and line 160 went from naming the first HOME to naming the second;
`bwrap.args` and `bwrap.txt` changed with it. After: `bin/git` reads
`4e2f0c58…` in both runs and no `bwrap.args` is under `state/gates` at all —
the wall a launch reads is the launch's to write.

## The rule

**A report may CREATE a wall that is not there and never REPLACES one that
is.** The reason to render at all is `renderBwrapWall`'s own — a report whose
head line names a file that is not there is worse than no report — and it is
satisfied by rendering only where the dir holds nothing. Where a wall IS
there, reading it is both the safe answer and the TRUE one: on-disk is what a
live session is running behind. A PID edit reaches the shims at the next
launch, which is already how every render-time input reaches them
(`RenderedByPosse`, ranger-base-mmhhc).

For the wall, the render is unchanged and only its destination moved. The
persona's live gates dir is still an INPUT to it, so the mount set a report
prints and a probe judges is the set a launch would grant; the bytes land
under `reportWallDir` (wall.go) — one temp dir per process, keyed by instance
and persona, reused, so the head line names a file that is still there when
the operator goes to read it. `wallWrite` carries the two decisions: where
the artifact goes, and whether the prep runs.

- a LAUNCH: the live gates dir, prep and all.
- a REPORT: `reportWallDir`, **no prep**. The prep is a write outside the
  namespace in the seat's name; a command whose whole job is to describe the
  wall has no business making it.
- a PROBE: `reportWallDir`, **prep kept**. `applyRecordReach` and
  `applyWallWitness` render and then APPLY; bwrap binds only a source that
  exists, so a probe with no prep would measure a wall that cannot come up
  and report the difference as this host's.
  **Amended by ranger-base-qd9ei (2026-11-12), this bead's residual: kept
  for a LAUNCH's probe and not for a reader's.** A reader's probe requires
  the furniture to be there already and abstains, with the path named, when
  it is not — so the three answers are four, and which one a probe takes is
  `parityAsk`. The reason the bullet above gives still holds, and is why a
  reader abstains rather than skipping. See
  docs/notes.d/ranger-base-qd9ei.md.

## Not closed here

- ranger-base-qd9ei

## Pins

- `cmd/posse/gatesreportwrites_qa_test.go` — the bead's repro through the
  built binary: render, record the shim's bytes and the HOME literal it
  bakes, run the report from another HOME, and every file under
  `state/gates/<p>` is byte-identical. Plus: no wall artifact is ever written
  there by a report, and the report still names the shims the verify recipes
  grep for and says it read rather than rendered.
- `internal/posse/reportwallwrite_qa_test.go` — both reports leave a planted
  live artifact byte-identical while still printing a mount set off a copy
  that exists; the report's render makes none of the prep paths the launch's
  render makes (with the launch render as the control); and the live
  Edit/Write witness, which really renders and applies, leaves the live
  artifact alone.
- `internal/posse/bwraplaunchline_qa_test.go` — the head line now names the
  report copy, that file exists, and the line is not the live wall's path
  while the output still names where the live wall is.
- `internal/posse/wallwitness_qa_test.go` — the abstention arm stats the
  probe's own render dir, because a gates dir with no artifact in it is now
  true of every run and would have said nothing about that one.

Control: both halves of the defect restored (`wallWriteToRead` returning
`wallWrite{}`, and the unconditional `RenderGates` in the `gates` case) reds
all seven, 11 failure lines in internal/posse and 7 through the binary,
naming `bin/git`, `bwrap.args` and `bwrap.txt`.
