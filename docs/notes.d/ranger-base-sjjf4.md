# ranger-base-sjjf4 — one control strengthened, six decisions pinned

2026-11-11. The seven findings ranger-base-7urt1 brought back from attacking
four closes (ranger-base-6rr2e, ranger-base-ctxeq, ranger-base-ej47m,
ranger-base-j687x). All four closes verified on their own acceptance; these
are the gaps found attacking them, and all seven were mutation-checked both
ways here — the pin red before the fix, green after, and every mutant
restored byte-identical (`md5sum -c`).

MEASURED 2026-11-11 on linux 7.2.15 x86_64, go1.27.3, from this session's
own worktree at main b0634dfc.

## 1. The marker control was defeated by a prefix that PRINTS its argv

`wallWitnessRow`'s marker control is the one thing standing between a
NEGATIVE probe and a green row over a seat with no wall: the walled command
echoes a token before the create, and a run that did not print it never
reached the shell behind the prefix. It read the token with
`strings.Contains`.

The token is also a literal INSIDE the argv the prefix is handed (`echo
<marker>; : > <path>`), so a prefix that prints its argument instead of
running it prints the marker too, and a substring reading cannot tell the
shell echoing the marker from the prefix echoing the command it was given.
The create was never attempted, no file landed, and the row read **the wall
REFUSED it** — the exact false pass the file exists to prevent. Reached
through `applyWallWitness`, such a prefix left the row reading `witnessed at
launch`.

**The fix is one line of reading: a whole output LINE equal to the marker**
(`wallWitnessMarkerPrinted`). The inner command echoes the marker ALONE, so
a run that reached the shell prints a line that IS the marker, while a
prefix echoing its argv prints it with the rest of the inner command around
it — on one line (`/usr/bin/echo`) or one argument per line (`/usr/bin/env
printf '%s\n'`). Both spellings are table rows of
`TestQAWallWitnessAbstainsWhereNothingRanBehindThePrefix` now; both read
`why="" unmeasured=""` under the substring reading and abstain under the
line reading, and all three shipped stub rows stayed green.

What this did NOT repair is a live false green: the only two producers of
`WallRender.Prefix` are `BwrapPrefix` and `SeatbeltPrefix`, and neither
binary echoes its argv on any failure. What it repaired is the STRENGTH of
the control, in a package that already stubs the bwrap binary in a fixture
(`bwrapprepauthority_qa_test.go`) — which is the shape that reaches it. ADR
0086 D5's as-built enumeration ("a stub binary that SWALLOWS its argv") is
amended with the shape it was short by.

What the line reading still cannot see is a prefix that prints the marker on
a line of its own with nothing else — a prefix written to defeat this
control rather than one that merely does not run its argument. Said out loud
in the function's comment, because a control whose limit is unstated is read
as having none.

## 2. Two abstention arms with no pin, and the lever already in use

`wallWitnessUnmeasured` is two whole sentences rather than one with the
binary substituted in, because what the abstention IS differs by mechanism
and so does the clause saying the launch is unaffected — the part an
operator acts on. Its one call site keys on `WallMechanism()`, so one arm
was dead on every host, and nothing asserted either sentence: the existing
abstention arm asserts `NOT WITNESSED` and never WHICH wall.

`wall.go`'s own comment claimed the drivability ("so the arm this host does
not have is drivable from the other one") and nothing drove it, while
`wallMechanismForTest` — the lever — was already in use for exactly this in
two other files.

`TestQAWallWitnessAbstentionNamesTheWallThisProcessCannotPutUp` drives both
arms through `applyWallWitness`, because the thing that escaped is the ROW an
operator reads: `wallWitnessAbstain` wraps the sentence in the availability
claim and the mechanism. It asserts each arm's sentence AND the absence of
the other's, so the two cannot collapse into one. The apply is stubbed
refused, so neither arm renders or runs anything and both run on either
host; the one precondition left is the tier, since `wallRendered` is what
puts a launch behind an L2 wall at all.

`wallApplyRefusalOn`'s third arm — "this build renders no wall for mechanism
…" — has no caller but `wallApplyRefusal`, which can only hand it
`WallMechanism()`'s two answers or `""`. Pinned as a unit arm over `""` and
a nonsense mechanism, asserting the quoted mechanism and this host's GOOS:
there is no host to put behind a mechanism that does not exist, and
`wallMechanismForTest` pins a GOOS rather than a mechanism.

## 3-5. Three decisions ADR 0087's close stated and nothing compiled

- **The opt-in is the FILE and not the BIT** (`landCheckShipped`), the
  close's one stated divergence from D1 — and the close named the literal
  reading as a one-liner the architecture lane may want, so the gap sat
  where the next reader would act. Mutant: `&& fi.Mode().Perm()&0o111 != 0`
  appended. Nothing saw it, because `shipLandCheck` chmods every fixture
  0755. Two pins: a unit arm over the modes and the two non-regular shapes
  (FIFO, directory — the same mutant kills `.IsRegular()`), and a LANDING
  over a base whose copy is 0644, which lands with the ⚠ naming the file and
  `permission denied`, records nothing, and puts the work on main. The
  divergence is sound and loud in the right direction; the exec bit is left
  to exec, whose refusal is a "could not answer".
- **D4's record is keyed on the TREE and not the commit, because the check
  reads BYTES.** Both readings agreed on every fixture, because a
  fast-forward landing leaves the branch tip and the new base at the same
  commit. The fixture that discriminates is a base that moves to a commit
  whose tree was already judged — an `--allow-empty` commit here, a
  revert-and-reapply in the field: the tree-keyed lookup hits and the
  commit-keyed one pays a whole base probe (checkout --detach, run,
  restore) for bytes it has already read. The run COUNT is the
  discriminator, deliberately: the record's own first field is written and
  read by `revTreeID`, so an assertion spelled through that function moves
  with the mutant and measures nothing. The pin reads the tree id with its
  own `rev-parse`.
- **The reset asks the HEAD first, and it is REACHABLE.** `baseLandCheck`
  returns the base's verdict off the RECORD before reaching its own copy of
  the guard, so a detached session tree whose base tree is on record green
  with a red replay arrives at `resetReplay` detached — where a `reset
  --hard` would move HEAD and leave the branch on the replayed tip, "the
  undo silently not happening". Pinned as a unit arm asserting both halves:
  the refusal sentence the caller lands with, and that the branch ref did
  not move. `baseLandCheck`'s own copy is pinned in the same arm.

## 6. The ⚠ reaches two of its four print surfaces

The warning was pinned on the `MergeOutcome` FIELD alone; no test asserted
any of the four print sites, so deleting the block that prints it red
nothing. Two of the four are cheap to reach and are pinned here: the kill's
line set (`KillLanding.Lines`, including its ORDER — after the
generated-file note, before what became of the tree) and the operator's own
`posse worktrees --land`. Both mutants killed.

The other two (`dispatch.go`, `landsweep.go`) print the same field from the
same outcome one call later and are left unpinned on purpose: the gap they
share is this package's shape and not this decision's — `RegeneratedNote`,
the sibling note all four were modelled on, is asserted at no print surface
either. Filing that as its own debt would be filing the package's print
surfaces, which is a bead nobody asked for.

## 7. An unreachable conjunct, kept where the producer can reach it

`AtShellPrompt`'s guard opened with `p.ShellPID == 0`, and no input reached
it. A foreground pid herdr really reported is never 0, so the pid comparison
two lines below already answers UNKNOWN for a missing shell_pid — including
the "no shell_pid" subtest named for the conjunct, whose fixture is
`shell_pid 0` beside `foreground [82630]`.

**The finding's recommendation was to DELETE the conjunct; this diverges,
and the divergence is load-bearing.** The ONE input that reached it is a
foreground pid of 0, and that is exactly what a herdr whose foreground rows
lost their `pid` key produces — the moved output shape the paragraph above
the guard is about. Under a bare delete, `{ShellPID: 0, Foreground: [0]}`
reads TRUE: every reported pane promptable, every reported label stale, on a
herdr that simply said less. So the reading is kept where the PRODUCER can
reach it — `if pid == 0 || pid != p.ShellPID` — and the unreachable conjunct
is gone.

**The rewrite changes the answer for no input at all.** Old `true` needs
`ShellPID != 0`, a non-empty foreground, and every pid equal to it (so no
pid is 0); new `true` needs a non-empty foreground, no pid 0, and every pid
equal to `ShellPID` (so `ShellPID != 0`). The two conditions are the same
set. What changed is that every arm is now reachable, and two pins reach
them: a unit arm over `{Foreground: [0]}` and `{ShellPID: 34661, Foreground:
[34661, 0]}`, and a third row in the fake-herdr table —
`pane-foreground "0|0"` — which drives it through the real parse and the
real delivery verb. The bare delete reds both.
