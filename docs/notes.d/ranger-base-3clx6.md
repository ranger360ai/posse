# A pinned producer and a pinned render, and nothing on the line between them (ranger-base-3clx6)

*2026-11-11 · holden · verifying ranger-base-ludpp, ranger-base-zmk4q,
ranger-base-m0bfv and ranger-base-ntnst*

## The class

Four security closes landed with good pins. Each one has the same shape: a
PRODUCER that computes a list, a RENDER that turns a list into a wall, and
one assignment wiring the two together. The producer is pinned. The render is
pinned — the arms hand it a list of their own. **The assignment is pinned by
nothing**, and it is the only part of the three that a seat's kernel sees.

MEASURED 2026-11-11, this tree at 1044e68e. Five such lines were reverted at
once and all three arms of `internal/posse` were run:

| the line | what it stops doing |
|---|---|
| `bwrap.go` `bwrapRender` → `Carve.Hide` | every bwrap seat gets the host's user runtime dir back |
| `herdrback.go` `planLaunch` → `MCPConfigWallRefusal` | a launch line may name a server list the seat can rewrite |
| `herdrback.go` `RelaunchAgent` → the same | the same, on relaunch |
| `runtimecheck.go` `launchRow` → `strictMCPClause` | `posse runtime check` stops reporting the flag |
| `wall.go` + `reachability.go` → `SeatbeltStateDirs` | the gates report and the reach probe judge a write set the profile does not grant |

**Arm 1, arm 2 and arm 3 were all green** with every one of those five
reverted, bar one failure that reds unmutated too (ranger-base-5bn54). A
closer who runs the suite after such a revert is told the wall is fine.

## Why the arms miss it

An arm that drives the render needs a list, and the cheapest list is one the
arm makes. `BwrapArgs(cwd, nil, SeatbeltCarveOut{Hide: hide})` is a good
render pin and a good producer pin sits beside it — and between them the
test has substituted itself for the one caller that matters. The tell is a
pin file whose every arm names the producer or the struct field, and none of
which names the function that a launch actually calls.

**Ask of a wall fix: which call does a SEAT go through, and does an arm start
there?** Not "is the behaviour pinned" — it is, twice.

## What landed here

`TestBwrapRenderWiresTheRuntimeDirsIntoTheWall`
(`internal/posse/bwraphide_qa_test.go`) renders posse's own wall through
`bwrapRender` and asks the argv for `--tmpfs <runtime dir>` and the three
`--unsetenv` names. Mutations: dropping the assignment, and dropping the
tmpfs, and emptying the unset list each red it for their own reason; a
comment edit does not.

The other four lines went to the code lane as debt, with the repro above.

## The pin that scored a kill it had not earned

The first draft of that arm looped over `bwrapUnsetEnv` — the package
variable the render reads — asserting each name was in the argv. Setting
`bwrapUnsetEnv = []string{}` left the arm **green**: zero names, zero
assertions, nothing said. A rig that builds its expectations from the list
under test makes every mutant of that list equivalent. The names are spelled
in the test now, and emptying the list reds it.
