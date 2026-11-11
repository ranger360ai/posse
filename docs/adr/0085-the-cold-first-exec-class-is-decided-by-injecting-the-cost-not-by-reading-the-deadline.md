# ADR 0085 — The cold-first-exec class is decided by injecting the cost, not by reading the deadline

*Status: accepted 2026-11-10 (ranger-base-gdzu5, from ranger-base-aru4c) ·
amended 2026-11-10 (ranger-base-dacil: D2, D3, D5 and D6 move the cost
from the file's bytes to its `execve`; the first full run under the
content rewrite, ranger-base-kiwvn, is the evidence) · owner: architect ·
builds on ADR 0006 §7 (a pin names the process it protects)*

## Context

A test plants a new executable (`WriteExecutable` into a `t.TempDir()`)
and execs it under a deadline of a few hundred ms. macOS checks a newly
written executable on its FIRST exec. MEASURED on darwin 25.6.0
(ranger-base-y9q7l, ranger-base-aru4c): first exec mean 156-714ms, worst
2.17s; re-exec of the same path 6.3-10.9ms. So a small deadline over a
cold exec gives a verdict about process startup, not about the code. The
class has been found three times (hangingBin 2026-09-03, y9q7l, aru4c).
The fix is settled: a `--warm` pre-flight outside the deadline, with both
halves asserted.

aru4c's census found 14 candidate rows. 10 of them turned on a judgement no
matcher made: does this duration bound an `execve` of the planted file at
all? Two readings settled them: (1) a fixture driven through `Herdr` pays
its cold exec inside a synchronous `mux.Start` before any caller's deadline
starts; (2) `sh <path>` execs `/bin/sh`, not the planted file.

New, MEASURED 2026-11-10 on the Linux host (7.2.15, x86_64, loadavg 7.56,
n=40 fresh `#!/bin/sh` scripts): first exec mean 1.07ms, worst 1.29ms;
re-exec mean 0.96ms. **Linux has no cold-exec cost.** The main crew writes
tests here now (ADR 0083), but darwin still runs the suite: ci.yml's
`macos-latest` leg and the laptop, kept as the rollback home. So a new
unwarmed fixture now passes where it is written and flakes only where
nobody is watching. Prose in hangingBin's header cannot catch that. A gate
that runs here can.

## Decision

**D1. The subject is the verdict, not the source.** The class is "a test
whose verdict changes when the first exec of a planted file is slow." That
is decidable by running the test with the first exec made slow. No reading
of the source is needed. Both of aru4c's readings fall out of execution:
a `Herdr`-driven fixture pays the injected cost inside `mux.Start`, outside
its deadline, so it stays green. `sh <path>` never execs the path, so it is
never delayed.

**D2. The cost is injected at the `execve`, not in the file** (amended,
ranger-base-dacil). A supervisor, `cmd/coldexec` (Linux only), wraps the
door's `go test` command line. Its child sets `no_new_privs`, installs a
seccomp filter that returns `SECCOMP_RET_USER_NOTIF` for `execve` and
`execveat` (and `ALLOW` for every other call), hands the listener fd to
the parent over a socketpair (`SCM_RIGHTS`), and execs the command. Every
process under it, at any depth, inherits the filter: the test binary, the
`sh` it starts, the `git` that runs a hook.

For each notification the parent reads the path argument from
`/proc/<pid>/mem`, resolves a relative path against `/proc/<pid>/cwd`
(`execveat`: against `/proc/<pid>/fd/<dirfd>`, or the fd itself under
`AT_EMPTY_PATH`), and `stat`s it. **A plant is an inode whose ctime is
after the door started.** The first time the parent sees a given
`(dev, ino, ctime)`, it sleeps D. Then it checks
`SECCOMP_IOCTL_NOTIF_ID_VALID` and answers `SECCOMP_USER_NOTIF_FLAG_CONTINUE`,
so the kernel runs the original `execve`. A file rewritten in place gets a
new ctime and is cold again, which is what the old nonce did. Each
notification is handled on its own goroutine, so one cold exec does not
hold up the others.

The file's bytes are never touched. `$0`, line numbers, the shebang, a
hook's identity, a shim's header and a copied binary all reach the disk and
the reader exactly as the test wrote them. The pid is the forked child's
own, as on darwin, where the assessment also happens inside `execve`.
`sh <path>` execs `/bin/sh`, an old inode, so it is never delayed (D1
holds).

MEASURED 2026-11-10 (Linux 7.2.15 x86_64, tree 6365a923, loadavg 10-33, a
scratch probe of this mechanism, D = 2.5s; the numbers are on the bead):

- first exec of a new `#!` script, a copied `/bin/true`, a hook that `git
  hook run` execs, a relative `./x`, and a script rewritten in place: each
  +D once, then 1-11ms. `sh <path>`: 2ms. An old `/bin/true`: 5ms.
- the 26 tests (28 rows across the three arms) that redded under the
  content rewrite: 28 of 28 green, with 92 planted files still delayed
  2.5s on their first exec.
- `TestRunOnceDoesNotLockOnARead` with its `--warm` pre-flight removed:
  red with its own fatal ("a read verb waited on a write lock held by
  somebody else"). With the pre-flight it is green. This is the acceptance
  this ADR set.
- `bwrapgitrefslive_test.go` and `wallmechanism_qa_test.go` under the
  filter: 19 pass, 1 skip, and the same skip appears without the door.
  bwrap is not setuid on this host, so `no_new_privs` does not change it.

**D3. Nothing inside the tested code changes** (amended). The door is a
process around `go test`. No package variable, no `TestMain` arm and no
shipped binary knows about it. The content-rewrite hook in `execwrite.go`,
the shim arm in `herdr_test.go`'s `TestMain`, and
`coldExecProtocolReason` are deleted once the supervisor is green. The
old rule that the hook "applies only when the caller is a `_test.go`
file" is dropped too. A shim that production code renders and a test then
execs under a deadline is just as cold on darwin, so the door now reaches
it. Reds that this produces are in the class.

**D4. D = 2.5s.** That is above the measured darwin worst case (2.17s). A
deadline that survives 2.5s of cold exec survives darwin. ASSUMED: the
2.17s tail is the true worst. If darwin ever measures worse, raise D. It is
one constant.

**D5. Coverage is defined on execs, not on writes** (amended). The
original D5 passed plants without `#!` through and assumed there were few
of them. MEASURED by ranger-base-kiwvn (2026-11-10, Linux, tree 10d046f9,
D = 2.5s): 840 of 1711 plant events had no `#!`, which is 49%. Per arm
that was 325/205/310 passed through against 346/108/417 rewritten. The
interim fix for the 26 byte-subject reds then passed through another 199
hook and shim plants in arms 1 and 2. Of the plants that were rewritten,
212 of 262 in those two arms were ever exec'd. Nobody measured what the
840 are. Under the amended D2 that question no longer matters: a copied
binary that is exec'd is delayed, and one that is never exec'd costs
nothing either way.

The door covers **every first `execve`, by any process under it, of an
inode created after it started**. These are the edges it does not cover,
each stated rather than left silent:

- **Files older than the door**: the checkout and the toolchain. On CI's
  macOS leg a fresh checkout's `scripts/` are new as well. That exposure
  is not in this class, and the content rewrite did not cover it either.
- **A process that installs its own seccomp listener under the door.**
  MEASURED: `EBUSY`, because the kernel allows only one listener in a
  filter chain. That process fails loudly, never silently. internal/posse
  installs no listener, and the bwrap tests above pass.
- **A kernel without user notification**, or an architecture the filter
  does not name: the supervisor refuses to start. It never runs a suite
  uncovered. Requires Linux 5.5 or later (`FLAG_CONTINUE`). The host runs
  7.2.
- **An ORPHANED descendant's execs — measured under ranger-base-in8dr,
  CLOSED under ranger-base-orgvt, and at a stated price.** Reading the path
  argument needs `PTRACE_MODE_ATTACH` on the caller, which the supervisor
  has by being its ancestor; on a host with `yama/ptrace_scope=1`, which
  this one is, an ancestor is the only thing allowed. A descendant whose
  own parent EXITED was reparented past the supervisor and left that
  reach, so its execs read `unreadable` and a plant it exec'd was never
  delayed — the test that planted it was not covered at all, silently as
  far as that test could tell. MEASURED 2026-11-11, Linux 7.2.15, over the
  two arms in8dr added: 145 of the internal/treepins arm's 29744
  notifications un-adjudicated (134 unreadable, 11 then called `gone`),
  0.49%, against 0 of cmd/posse's 8427, whose tests do not orphan. In
  treepins they cluster in the suite-lock self-tests, which orphan holder
  processes on purpose. Of that 145, it is the **134 unreadable** that are
  this edge: ranger-base-j0riv has since retired the `gone` verdict
  altogether, so the other 11 were never this and are no longer counted as
  holes by anything.

  Not "reparented to init", which is what the open bullet said: MEASURED
  here as `systemd --user`, pid 1140, because a systemd user session makes
  itself a subreaper. Immaterial to the mechanism — either target is
  outside the reach — but it is why the fix is the flag and not a check
  against pid 1.

  **The fix** is `prctl(PR_SET_CHILD_SUBREAPER, 1)` on the supervisor
  before it forks: an orphan lands on the supervisor instead of past it,
  so it stays a descendant and its execs are adjudicated like any other's.
  MEASURED on the repro: the orphan's plant goes from 2-3ms and
  `unreadable` to 504ms and `delayed`, with no holes left in the run.
  `PR_SET_PTRACER` in the child was measured and REJECTED — the prctl
  succeeds and changes nothing, because yama's exception is per-task and
  does not reach the declaring process's descendants, so the orphan's own
  child is refused exactly as before.

  **Two prices, and neither was in the ASSUMED "one prctl" estimate.** The
  flag makes the supervisor the parent of every orphan, so it is the only
  process that can wait on one: without a reaper an orphan that exits
  stays a zombie and answers `kill -0`, which is this tree's own liveness
  predicate (`codextrust_test.go` reads ESRCH as "ended and reaped",
  anything else as a leaked zombie). MEASURED with the prctl and no
  reaper: 20 orphans leave 20 `Z (defunct)` children, and on the pin's
  script the orphan's pid still answered `kill -0` ten seconds after it
  exited. `reapOrphans` closes that, so liveness is unchanged.

  What is NOT recoverable is parentage: **under the door an orphaned
  descendant's ppid is the supervisor's and not 1**, so a test that plants
  a real orphan and reads `Orphaned()` (`PPID == 1`) sees the door and not
  itself. Nothing in the arms does that today — `orphancontrol_test.go` is
  the one place that asserts ppid 1 on a planted orphan, and it runs only
  inside `scripts/verify-orphan-report.sh`'s throwaway container, skipped
  here — and `cmd/checkorphans` runs after the door has exited, so its
  ppid-1 reading is untouched. So this bullet does not leave the edge
  list, it changes halves: it stops being an exec the door fails to cover
  and becomes the one thing the door CHANGES about the suite under it,
  traded knowingly for 134 silent un-adjudicated execs. Pinned both halves
  by `TestAnOrphanedDescendantsPlantPaysDAndLeavesNoZombie`.

- **A CHARGE THE KERNEL CUT SHORT**, added here for the same reason as the
  row above — it is measured rather than reasoned about (ranger-base-4kars,
  2026-11-11). seccomp's wait for the supervisor's answer is interruptible.
  When the stopped caller takes a deliverable signal the kernel REMOVES the
  notification, and a caller whose handler carries `SA_RESTART` restarts the
  `execve`. Nothing escapes the door unadjudicated (ranger-base-j0riv): the
  restart IS adjudicated, so this was never a gap in the coverage — it was a
  gap in the DELAY. Until ranger-base-91fu2 the restart read the inode as
  warm, because the charge was marked before the sleep, and that plant's
  first exec completed in milliseconds against the D the door was holding it
  for. MEASURED over all five arms, 364,642 notifications: 12 of 2,453
  charges, 0.49%. Every one of them is a planted `bd` exec'd by
  `internal/posse/bdwriteshim.go`'s `syscall.Exec` — a live Go runtime
  performing a direct `execve`, which is the one caller shape in this tree
  that can be withdrawn at all, since a plant exec'd through `os/exec`
  blocks every signal across fork+exec. The eleven in the internal/treepins
  arm reproduced identically across two runs, so for that family it was not
  a rare race.

  **THE RESTART PAYS IT NOW** (ranger-base-91fu2, 2026-11-11), and the
  charge is what the door holds rather than the notification: an entry keyed
  by the TASK and the inode, carrying the absolute deadline. A restarted
  `execve` necessarily arrives on the same task — the kernel restarts the
  syscall in place — and a task can only be inside one `execve` at a time,
  so a notification matching a live entry IS that restart and nothing else;
  it sleeps the REMAINDER, and the `execve` completes no earlier than D from
  where it entered the kernel however many notifications it spends getting
  there. MEASURED on the pin's fixture, D = 600ms: the nudged plant's exec
  goes from 10-11ms to 612ms, and the run reports two delayed and no
  truncation. The charge stays ONE charge, so the cost model (D once per
  cold exec, additive within 5% — ranger-base-in8dr) is unchanged, and the
  `seen` mark stays where it was, so two concurrent first execs by DIFFERENT
  tasks still pay D once between them. The key is unique only while
  `seccomp_notif.pid` is the task id and not the thread-group id, which is
  MEASURED rather than assumed — a non-leader thread's `execve` of a plant
  is adjudicated under that thread's own tid, 3 of 3, tid != tgid in all
  three — and pinned by
  `TestANonLeaderThreadsExecIsAdjudicatedUnderItsOwnTid`.

  So this bullet does not leave the edge list either; like the row above it
  changes halves. What is left is a charge NOTHING restarts: a caller whose
  handler lacks `SA_RESTART` takes EINTR and one with no handler dies, and
  in both of those the `execve` never happens, so no plant exec went through
  at full speed. The door still spends the charge and still names it
  `truncated`, never folded into the delayed count, because what it leaves
  behind is a plant marked warm — a LATER exec of that inode is not held.
  Pinned both halves: `TestARestartedExecvePaysTheRestOfItsCharge` (the
  wall time, which there is now something to read) and
  `TestAChargeNothingRestartsReadsTruncated`.

At its end the run prints the number of notifications and the number of
cold execs it delayed — held to the end and let through, which is what that
count means and not merely what was charged (ranger-base-4kars). Zero
delayed is a failed door, not a clean suite.

**D6. The door is `make coldexec-check`**, with `RUN=<regex>`, as before.
What changes is that each arm's command line runs inside the supervisor.
`POSSE_COLD_EXEC_DELAY` now sets the supervisor's D. Suite slots, the
`RUN=` filter and staying out of `tree-check` and `make test` are all
unchanged. The supervisor wraps any `go test`, so cmd/posse and
internal/treepins (ranger-base-in8dr) each become one Makefile line, with
no injector of their own. **Those two landed 2026-11-11
(ranger-base-in8dr), and the five arms are now a census rather than a
choice**: `make execwrite-check`'s tree-wide pin
(`TestQATreeGoFilesWriteExecutablesUnderTheForkLock`, which walks every
`.go` file in the repository) establishes that every exec-bit write in
this tree goes through a `WriteExecutable`, so counting those calls counts
the plants — MEASURED 2026-11-11: 345 call sites in exactly three
packages, internal/posse 274, internal/treepins 54, cmd/posse 17, and no
other package in the tree has one. internal/treepins' own copy of the
writer and cmd/posse's call into `posse.WriteExecutable` needed no change
of any kind, which is D2/D3 paying for itself. The list is still WRITTEN
and not derived: a fourth package that starts planting tomorrow is covered
by nothing until its line is added, and that pin's per-directory floors
are a file count and cannot notice. ranger-base-s64a5 is the bead for
deriving it. **Price, MEASURED 2026-11-11 (this host, go1.27.3, warm build
cache, loadavg 5-15, one sibling seat holding the other suite slot):**
cmd/posse 674.8s injected against 103.5s uninjected, 6.5x, with 231 cold
execs held; internal/treepins 1581.8s against 131.8s, 12.0x, with 608 held.
Both arms GREEN, no red in the class and no `--warm` pre-flight needed. The
cost is additive and barely overlapped — the injected wall is the
uninjected wall plus D once per cold exec, within 3% on cmd/posse (+571.3s
against 231 x 2.5s = 577.5s) and 5% on internal/treepins (+1450.0s against
608 x 2.5s = 1520.0s) — so an arm's doored wall can be predicted from its
cold-exec count before anybody spends the box on it. These two arms also
needed the one number the undoored run does not: `-timeout 60m` rather than
`make test`'s 25m, because `go test -timeout` bounds the test binary's own
wall and the door spends 1520s of it. internal/treepins at 25m died on
`panic: test timed out after 25m0s` at 1502s with tests still running,
which is neither this class nor a package that wants splitting. **Price,
MEASURED 2026-11-11 through the supervisor itself (this host, go1.27.3,
warm build cache, loadavg 1.7-10.7, both halves back to back):
+1688.4s over internal/posse's three arms** — 275.7s undoored against
1964.1s doored, 7.12x, with 1622 cold execs held (948.9s/476.3s/538.9s
doored against 106.6s/73.5s/95.6s undoored, ranger-base-xpxo6). That
retires the ASSUMED floor this line used to carry, the content rewrite's
+571.1s (280.3s to 851.4s, 3.04x, loadavg 4-11, ranger-base-kiwvn): the
amended door costs **2.96x** that in added wall, and the undoored
baseline reproduced kiwvn's to within 1.6%, which is what makes the two
injected numbers comparable at all. With in8dr's two arms, **the
unfiltered five-arm run is 4220.7s of package time — seventy minutes of
box**, before five suite-slot waits.

And in8dr's additive rule does not travel to this package: 1622 execs at
D is 4055.0s of nominal sleep against +1688.4s of wall, **41.6%
realised** (48.1%/49.9%/29.6% by arm), because internal/posse's plants
are exec'd from tests that run in parallel — which is what kiwvn measured
under the rewrite too. So D x cold execs is an upper bound everywhere,
tight where the plants are exec'd serially (cmd/posse within 3%,
internal/treepins within 5%) and about twice the truth where they are
not. Price a new arm from the bound. Whether CI pays for it is still the
operator's call.

## Consequences

- The class stops depending on someone remembering it. Running the door
  turns the darwin tail into a deterministic red on Linux, with the fatal
  the test already prints.
- Acceptance is still aru4c's census, and the amended door has reproduced
  its two key rows (D2's MEASURED list). `TestProbeSessionExeRefusesWhatItCannotName`
  (300ms, Herdr), the `ProbeOpts{Timeout: 2s}` sites, `watchpid_test.go`
  `liveProcess` (`sh script`) and every warmed helper (hangingBin,
  hangingGit) must stay green under it.
- **The artifact class was a family, not a tail.** This ADR first said a
  test that changes `$0` or reads its own line numbers "sees one extra
  line". MEASURED by ranger-base-kiwvn: 28 red rows over 26 tests, and none
  of them was in the class. All 26 had a planted file's own bytes as their
  subject. 23 planted a git hook that posse certifies against its own
  render, 2 planted a gate shim identified by a marker on line 2, and 1
  compared a copy against the literal it planted. Any injector that
  rewrites content is visible to every such test, so the amendment moves
  the cost to where those tests cannot see it.
- **That full run has happened, and it red nothing in the class**
  (ranger-base-xpxo6, 2026-11-11). This bullet predicted reds the content
  rewrite never reached — hooks, shims, production renders and copied
  binaries exec'd under a deadline — each to be fixed with a `--warm`
  pre-flight. MEASURED over internal/posse's three arms: arms 1 and 2
  green first time, all three green undoored, and **no test in this
  package has a verdict that changes when the first exec of its own
  planted file is slow**. The advice stands for a red that is in the
  class; none has been found yet. Of the five red rows the run did
  produce, four were a `/tmp` whose tmpfs inode table was 100% full
  (`df -h` read 80% with 2.8G free; ranger-base-vw1cp) and one was a
  **third kind this ADR had not named: a pin whose own real-time budget
  loses to the slower box the door creates.** Its planted executable was
  already warmed and no planted exec happened inside its deadline — it
  passed 10 of 10 focused under the door — but the window in which its
  watchdog goroutine had to catch a 20ms tick was 31.8ms wide under arm
  3's load against ~167ms on a quiet box. Such a red is neither in the
  class nor a supervisor bug, and the fix belongs in the test's own
  margin, not in a pre-flight and never in a pass-through list.
- Exit hatch: delete `cmd/coldexec` and the wrapper in the Makefile line.
  It holds no state beyond an in-memory set of seen inodes. Nothing in
  internal/posse refers to it.

## Alternatives rejected

- **Keep the content rewrite and admit a second pass-through class**
  (kiwvn's interim `coldExecProtocolReason`: pass a plant through when its
  basename is a hook slot, its parent or grandparent dir is `hooks`, or a
  shim marker is in its first two lines). It turns the run green by
  removing every hook and shim fixture from the door. Those are exactly the
  files git and posse exec on a commit. The rule is also a list of
  protocol names that grows each time posse adds a protocol. With D5's 49%
  on top, the door would have covered about half of what it claims. It
  stays as the interim until the supervisor replaces it.
- **A static matcher over duration literals near `WriteExecutable`.**
  aru4c's census is the measurement against it: 10 of 14 rows were
  undecidable. It either reds the runtimeprobe/watchpid family or misses
  the two sites that were actually wrong.
- **A pin over the fixture helpers** (every plant goes through a
  registered warming helper, or carries an exemption comment). Across 322
  call sites, most of them never timed, it becomes an exemption list whose
  entries are comments. ADR 0006 §7 asks what process changes if such a
  pin is deleted, and the answer is none: the comment would be the subject.
- **Prose only** (hangingBin's header plus the notes fragment). That is
  what held the class until now. It was rediscovered three times, and on
  Linux the darwin cost is now invisible to the people writing tests.
- **Hold the exec in the kernel by other means.** fanotify
  `FAN_OPEN_EXEC_PERM` needs CAP_SYS_ADMIN in the initial namespace. A
  bubblewrap namespace with `/bin/sh` bound to a shim replaces the shell
  every test uses. ptrace stops on every syscall and conflicts with
  anything else that traces. An `LD_PRELOAD` `execve` wrapper misses Go,
  whose `os/exec` makes raw syscalls. The first version of this ADR
  rejected "the kernel route" on the first two and never priced seccomp
  user notification. That route needs only `no_new_privs`, leaves the bytes
  alone and catches binaries, which are the three things that paragraph
  wanted. It is now D2.
- **Make `WriteExecutable` warm every plant by exec'ing it once.** Running
  arbitrary test fixtures for their side effects is not safe: half of them
  write marks, pid files or exit codes that the test then reads.
