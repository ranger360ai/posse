# A seat whose wall-watch AND bwrap are both gone has no lifetime bound, so the watch tick kills it (ranger-base-5k7vf)

2026-11-22, jian-yang. The finding is hoover's, a residual of ranger-base-6ec6n
(its own "Not covered here", first bullet).

## What breaks

On a host whose AppArmor moves bwrap's exec into another label,
`apparmor_bprm_committing_creds` zeroes the pdeathsig bwrap set on the seat.
`posse wall-unix` re-arms it — but only where the seat line HAS that step, and
a host below the Landlock signal-scope ABI renders none. On such a host the
only bound left on the seat's lifetime was ranger-base-6ec6n's other half:
wall-watch killing the seat when bwrap exits with the leader still alive.

A watcher that is SIGKILLed never makes that kill. bwrap dies through its own
pdeathsig (no exec follows it, so nothing clears that one), the seat's leader
is reparented, and the seat runs on — caged, but supervised by nothing and
bounded by nothing. The dispatch backstop's dead-watcher arm
(ranger-base-0qpsw D3) found exactly that seat, filed its P1 and left it
running.

WHERE IT IS REAL: ubuntu 24.04 ships `bwrap-userns-restrict` — the same
`unpriv_bwrap` transition — on a 6.8 kernel at Landlock ABI 4, so no step is
rendered for that whole host class. ASSUMED from the release notes and not
measured on such a box; that assumption is the bead's, carried here unchanged.

## What changed

The dead-watcher arm kills (`wallWatchBackstop`, internal/posse/wallwatch.go).
Per record, per tick, with every wall still up and the watcher gone, it now
asks one more question — `wallWatchSeatOrphaned`:

- is the record's leader alive and in the record's mount namespace
  (`wallWatchLeaderLive`'s two checks, through the probe's new `LeaderParent`);
- and is its parent something OTHER than the bwrap pid the record names?

Both halves are positive evidence, as the watcher's own gate is: a record
naming no bwrap buys no kill, because the comparison that would say
"reparented" cannot be made against it. A yes is a seat nothing bounds, and the
tick kills the seat's mount-namespace census as wall-watch does, writes
`KILLED seat …` to the verdict file (the seat's pane went with its watcher —
ranger-base-pfcci), and says so in bd: a new P1 whose title is "outlived its
wall-watch and its bwrap", or a comment on the dead-watcher bead this seat
already has, because that bead says nothing had been killed and that stops
being true.

The question is asked BEFORE the `OrphanFiled` short-circuit, deliberately: a
seat can be read in the bounded shape on one tick and the unbounded one on a
later tick, and that later tick is the one that must act.

WHAT IS LEFT ALONE, and it is the control the pins keep: a watcher that is gone
with the leader still a child of its bwrap. That bwrap ends the seat, so the
arm stays what it was — one bead, no kill.

## Measured, and what is not

MEASURED 2026-11-22 on D1 (Linux 7.2.15-1-omarchy, bubblewrap 0.13.1, go1.27.3,
no AppArmor):

- `TestLiveWallWatchBackstopKillsASeatOrphanedWithItsWatcherGone`, the live
  arm, models the cleared pdeathsig the way the 6ec6n pin does — by taking
  `--die-with-parent` off the rendered bwrap line — then SIGKILLs the watcher
  and then bwrap. Reading, one run: leader pid 3105341 reparented from bwrap
  pid 3105336 to pid 1140 with watcher pid 3105307 gone; the next tick killed
  the leader and its backgrounded job, removed the record, and filed one P1.
- **The orphaned shape appears transiently during an ordinary teardown**, and
  this cost the first draft of the sibling arm a false branch. A pdeathsig that
  was never cleared fires at bwrap's EXIT, so between that exit and the
  leader's own the leader is alive AND already reparented — indistinguishable,
  in one reading, from the host class this bead is about.
  `TestLiveWallWatchBackstopFilesAWatcherThatDiedUnderARunningSeat` branches on
  which host it is running on, and its reading now waits for the leader to GO
  before it decides: a leader still there, still reparented, when that wait is
  over is the AppArmor shape. The tick itself is not exposed to this — it reads
  once and acts on what it read, and a leader in that window is a leader that
  genuinely had no parent and no watcher at the instant it was read.
- The hermetic half: `wallWatchSeatOrphaned`'s six rows over a written table,
  `wallWatchProcProbe.LeaderParent`'s six over real /proc and a live child of
  the test binary, plus the backstop's three arms (kill, the bwrap-still-parent
  control, and the already-filed seat whose bwrap goes later).
- Mutants, both red on the new pins and green before them: the arm disabled
  (`orphaned && false`) reds the two kill arms; the parent comparison dropped
  (`return ppid, true`) reds the decision pin, the control and the already-filed
  arm.

ASSUMED, not measured: that an AppArmor host reaches this arm at all, which
needs a box with the transition and a sub-ABI-6 kernel. D1 has neither, and the
fixture models the clear rather than producing it.

## Not closed here

- The window is the dispatch watch interval the operator chose, not the
  watcher's 250 ms. A seat orphaned between two ticks runs until the next one.
  That is ranger-base-0qpsw D3's accepted residual and this arm does not narrow
  it.
- A watcher gone while bwrap still lives leaves the seat bounded by that bwrap
  alone — nothing kills it and the P1 says so. Decided, not overlooked.
