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
- **An ORPHANED descendant**, added here because it is now measured rather
  than reasoned about (ranger-base-in8dr, 2026-11-11). Reading the path
  argument needs `PTRACE_MODE_ATTACH` on the caller, which the supervisor
  has by being its ancestor; on a host with `yama/ptrace_scope=1`, which
  this one is, an ancestor is the only thing that is allowed. A descendant
  whose parent EXITED is reparented to init and leaves that reach, so its
  execs read `unreadable` and a plant it execs is never delayed. MEASURED:
  145 of the internal/treepins arm's 29744 notifications (134 unreadable,
  11 gone), 0.49%, against 0 of cmd/posse's 8427 — the treepins holes
  cluster in the suite-lock self-tests, which orphan holder processes on
  purpose. Reproduced in isolation at D = 500ms with two identical fresh
  plants: the child still in the tree is delayed, the orphan's is
  unreadable. It is counted and named, never silently called old, which is
  what D5's taxonomy is for. ranger-base-orgvt holds the candidate fix
  (`PR_SET_CHILD_SUBREAPER` on the supervisor, so an orphan reparents to it
  rather than to init) and will either land it with a pin or record the
  refusal here.

At its end the run prints the number of notifications and the number of
cold execs it delayed. Zero delayed is a failed door, not a clean suite.

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
which is neither this class nor a package that wants splitting. Price: MEASURED for the content rewrite, +571.1s
over the three arms (280.3s uninjected, 851.4s injected, 3.04x, loadavg
4-11, ranger-base-kiwvn). ASSUMED to be at least that for the amended
door, because it delays everything the rewrite did plus hooks, shims,
production renders and binaries. The price is measured again when the
door is rebuilt, before anyone proposes running it more often. Whether CI
pays for it is still the operator's call.

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
- The first full run under the amended door will red some tests that the
  content rewrite never reached: hooks, shims, production renders and
  copied binaries exec'd under a deadline. Those reds are in the class. Fix
  each with a `--warm` pre-flight, with both halves asserted.
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
