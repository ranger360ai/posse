# Four more red CI families, and three of them asserted one platform's answer everywhere (ranger-base-5oiz4)

2026-11-21/22. Filed off run 44947866559 (acdaf81f) while ranger-base-jyqk4
closed; diagnosed and fixed against run **44968403951** (cbd86ca1), which is
the first leg that carries every landing in this bead's own window.

## What was red, where, and whose it is

| family | leg · job (44968403951) | at cbd86ca1 | whose |
|---|---|---|---|
| `TestATreeThatShipsNoWallCheckIsLeftAloneEntirely` | ubuntu arm 2 · 137042528728 | red | fixed here |
| `TestLiveWallWatchBackstopFilesAWatcherThatDiedUnderARunningSeat` | ubuntu arm 1 · 137042528869 | **gone** | ranger-base-ebv8j (ada0c5bb) |
| `TestLiveWallUnixScopesSignalsToTheSeat` | ubuntu arm 1 · 137042528869 | red, both death arms | ranger-base-6ec6n (P1, open) |
| `TestQATheWrapperStillProvesItReusesTheBinary{,WithoutShasum}` | macos arm 1 · 137042528883 | red | fixed here |
| `TestScratchRexShimFencesWhatABodyMayDo`, `TestScratchRexRefusesAServerItDidNotStart/{count,fleet,fleetzero}` | macos arm 1 · 137042528883 | red | fixed here |
| `TestQAAGitGlobalSnapshotVerifyRefusesWhatIsNotAFileAtThatName/a_symlink_…` | macos arm 3 · 137042528870 | red | fixed here |

The bead's own ubuntu arm-1 reading is two families, and reading the leg at
cbd86ca1 rather than at acdaf81f is what separated them: the backstop one was
already fixed by a landing inside the window, and the die-with-parent one had
by then been diagnosed to the kernel under ranger-base-6ec6n — AppArmor's
exec transition clears the pdeathsig `bwrap --die-with-parent` sets, so no
expectation in the test was the defect. Neither needed anything here.

**ALSO READ, AND NOT THIS BEAD'S:**
`TestQACarveOutRefusesUnderSandboxExecAndTheControlDoesNot` is red on macos
arms 2 and 3 at cbd86ca1, in the two rows ranger-base-jyqk4 rewrote, at the
third profile's control line (`seatbeltcarveout_qa_test.go:518`). jyqk4 said
so in its own close — "the KERNEL reading is the next macos leg's. I am not
claiming a pass for those two" — and that leg has now answered. Filed as
ranger-base-al4n9 (P1, code); the detail is there.

## The one thing three of the four had in common

Each was a pin asserting what ONE platform does, on every platform, with its
only disagreeing venue a CI leg. That is ranger-base-jyqk4's sentence about
its own four, and it held again here. What differs between the two beads is
that two of these three can be read from a linux box once the thing they
assert is driven rather than inherited.

### 1. ubuntu arm 2: a substring collision, and an arm that ran once

`!strings.Contains(out, "wall")` over the whole pass, at a repo that ships no
`scripts/posse-wall-check`. The pass carries one sentence about a wall that is
not this step's: `landedJudgeSocketWallNote`, ADR 0080 D31 on the JUDGE's
verdict line, which names the socket wall, the file wall and `wall-unix` in
one breath on any host whose Landlock ABI is below `WallUnixABIMin`
(ranger-base-l2gk9, ranger-base-e5un1).

MEASURED 2026-11-22 at cbd86ca1: red on ubuntu arm 2 (ABI 8), green on macos
arm 2 (seatbelt — the note is empty), green on this box (D1, Linux
7.2.15-1-omarchy, ABI 10).

Two changes, and the second is the one that matters:

- the note is **subtracted** from the output before the grep, derived from
  `landedJudgeSocketWallNote()` itself. Not a narrower grep over the step's
  three phrases ("wall check", "wall pins", `wallCheckScript`): a list of
  phrases is a second copy of `wallcheck.go`'s output to keep in step by hand,
  and this arm's claim is the broad one.
- the arm is a **table over the Landlock ABI** — `WallUnixABIMin` and one
  below — with `wallUnixABI` and `wallMechanismHost` stubbed, so both halves
  of the note are driven here and neither row depends on the kernel or the
  GOOS the suite is on. Serial, because those are package vars.

Each row carries its own control: at ABI 9 the note must be empty and below it
it must not, or the subtraction is measuring nothing.

GRADED: with the step's opt-in question mutated away (`wallcheck.go`,
`if false && !landedCheckShipped(...)`) so the step SPEAKS at a repo that never
opted in, the arm reds at **both** ABIs. The subtraction did not blind it.

### 2. macos arm 1: a write deny that is not every kernel's

`scripts/gotest.sh --self-test`, two arms of ranger-base-nywsk's pair:

```
FAIL  control: a write over a live executable is refused: the write landed while it was running
FAIL  plant: the in-place plant waits out a live executable: the first write landed while the holder still held it (1 try) — the retry was not exercised
```

MEASURED 2026-11-22 on macos-26-arm64 (Darwin 25.6.0 arm64, job 137042528883):
the naive write over the running holder LANDED, and the plant's first attempt
landed too — two independent writes, both through. The arms asserted linux's
answer, so they red on every macos arm-1 leg since arm 1 began reaching Go
tests there at all (ranger-base-zcj0z).

The control is a **probe** now: it records the reading in `denied` and says
which one it got on its own `ok` line. The plant arm reads that verdict — on
an enforcing kernel it still requires `PLANT_TRIES >= 2`, and on a box with no
deny it holds that the plant LANDS and says the retry's evidence is the
enforcing kernel's, which ci.yml's ubuntu arm-1 leg supplies on every run. Not
a skip and not silent: "there was nothing to refuse" and "the refusal stopped
working" are different facts and the line says which.

**And the row needed a third content to be worth anything there.** The control
writes `$holdsrc` over the holder; on a no-deny box that write lands, which
left the holder byte-identical to the plant's source and `plant_over`'s
read-back (`cmp -s -- "$src" "$dst"`) with nothing to read — the exact
condition the script's own comment says makes a plant that writes nothing look
like one that works. `$plantsrc` is a third content.

GRADED over copies of the script in the seat's scratchpad, the tracked tree
untouched (`no-deny` releases the holder before the control's write, which
models darwin; `nowrite` makes `plant_over` report success and write nothing;
`oldsrc` points the plant back at `$holdsrc`):

| run | the plant row |
|---|---|
| baseline | `ok` (enforcing path, retry exercised) |
| no-deny | `ok … NOTHING TO WAIT OUT on this box (1 try)` |
| no-deny + nowrite | **FAIL** — "the write reported success but … does not hold the planted bytes" |
| no-deny + nowrite + oldsrc | `ok` — the pre-fix hole, surviving |

### 3. macos arm 1: `/run/user/<uid>` is a linux path

```
scratch-rex: no runtime directory at /run/user/501
```

`scripts/scratch-rex.sh` resolved its scratch root as
`${XDG_RUNTIME_DIR:-/run/user/$(id -u)}` and died unless it was a directory.
No darwin host has one, so the harness aborted at exit 2 before guard 1 ran
and four pins measured nothing while reading `exit 2, want 70`.

The rule the runtime dir was named for is **length** — a unix socket path is
capped, 104 bytes on darwin — and not a platform, so the rule is now the rule:
`$XDG_RUNTIME_DIR`, `/run/user/<uid>`, `$TMPDIR`, `/tmp`, first that exists
AND leaves room for `posse-scratch-rex.XXXXXX/s.sock` (33 bytes with the NUL).
That keeps the original argument — a dispatched seat's TMPDIR is its session
scratchpad, ~130 characters, and is passed over by the arithmetic — without
ruling out the hosts that have no runtime dir at all. `/tmp` is last because
it is the one directory every uncaged process on the box also writes.

Six rows in the script's own `--self-test` table (now 26, floor 15): the
boundary from both sides, a short runtime dir, a macos runner's
`/var/folders/<…>/T` (49 characters, from job 137042528870's own log), a
dispatched seat's scratchpad TMPDIR, and that the chain names a directory on
whatever host it is run on. No row BINDS a socket, deliberately: `sun_path` is
108 bytes here, so a socket at the 104-byte boundary binds on this box
whatever the number says and such a row would be green over any value of it.

MEASURED 2026-11-22 over a copy of the script with `/run/user/<uid>` made not
to exist and `XDG_RUNTIME_DIR` shed — a darwin-shaped host — with the QA
file's own fake rex:

- `self` (the server answers for the socket this run started): socket at
  `/tmp/posse-scratch-rex.<…>/s.sock`, **the body ran**, exit 0. That is
  `TestScratchRexShimFencesWhatABodyMayDo`'s precondition, the one its failure
  message says the arm is worthless without.
- `fleet` (it answers with the box's default socket): `REFUSING: the server
  that answered is unix:///run/user/1001/rex/server.sock, not
  unix:///tmp/posse-scratch-rex.<…>/s.sock`, **exit 70**, body did not run.
  That is the `exit 2, want 70` the three rows of
  `TestScratchRexRefusesAServerItDidNotStart` read.

Before the fix both runs died at exit 2.

`internal/treepins/mktemproot_qa_test.go`'s exemption row for this script
carried the old resolution as its reason and now carries the new one; the
`mktemp` template it matches on is unchanged.

### 4. macos arm 3: a control that guessed a symlink's mode

```
CONTROL FAILED: the plant is mode 0755, so the mode arm can answer = true, want false
```

`modeArmCanAnswer` read `runtime.GOOS != "darwin"` for the symlink row, on the
strength of lchmod being something a darwin caller COULD do. That row's plant
calls no chmod at all, so what it gets is whatever the kernel gives a fresh
symlink. MEASURED 2026-11-22: **0777** on this box, **0755** on macos-26-arm64
(job 137042528870). Neither is 0444, so the mode arm can answer on both and
the row is `true` with no GOOS in it — the symlink row asserts the refusal and
only the fifo row isolates which arm made it, on every platform. A box that
ever gives a bare symlink 0444 reds this control rather than silently turning
the row into the fifo's, which is what a control is for.

## What is owed to a CI leg, and what is not

Families 1 and 4 are read from this box at both of their readings: family 1's
table drives the ABI, and family 4's two platform modes are 0777 and 0755,
both `true`. Family 2's two arms are read here through models of the platform
— a released holder for the missing write deny, a non-existent `/run/user` for
the missing runtime dir — and the macos leg is what turns those models into
readings. Nothing in either is a claim about darwin that this box made up: the
darwin behaviour each models is quoted above from job 137042528883's log.

## Not closed here

- ranger-base-6ec6n
- ranger-base-al4n9
