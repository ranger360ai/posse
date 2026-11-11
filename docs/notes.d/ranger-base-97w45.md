## The execve supervisor, as built, and the eight things ADR 0085 left to the build (ranger-base-97w45)

2026-11-10 — ADR 0085 D2/D3/D5/D6 as amended by ranger-base-dacil (commit
618c6cb2) built on Linux 7.2.15 x86_64, go1.27.3, tree 4f797308. Slice 1 of
3: the supervisor and its Makefile wrap. ranger-base-xpxo6 is the first
unfiltered run and the price; ranger-base-7zkxw retires the content rewrite
this replaces.

The mechanism and the argument live in the code: `cmd/coldexec/doc.go` (what
the door is, end to end, and the three edges it does not cover),
`main_linux.go` (the child: no_new_privs, the BPF program, the SCM_RIGHTS
hand-off, the exec), `door_linux.go` (the supervisor: the notification loop,
the path resolution, the verdicts), `supervisor_linux_test.go` (eleven pins,
each naming what it protects), and the `coldexec-check` block in the
Makefile. This fragment holds what none of them can: the acceptance
measurement, the eight decisions the ADR left open, and the mutations that
say the pins can still fail.

### The acceptance measurement

ADR 0085's consequences name one test as ground truth:
`TestRunOnceDoesNotLockOnARead` (`internal/posse/bdwritelock_test.go`), whose
2s bound over the first exec of a planted `bd` sat BELOW darwin's measured
2.17s worst case. MEASURED here at D = 2.5s, through the built supervisor:

| run | verdict |
|---|---|
| its `--warm` pre-flight removed, under the door | **FAIL 2.01s**, `bdwritelock_test.go:486: a read verb waited on a write lock held by somebody else` |
| the pre-flight restored, same door | PASS 2.51s, 2 cold execs delayed |

Which is the row the ADR set, reproduced by the built door rather than by
the scratch probe: the test's OWN fatal, over code that is fine, and the
`--warm` pre-flight is what clears it. Note the second row's 2.51s — the
test pays D inside the pre-flight, outside its own deadline, and its bound
then covers a warm re-exec, which is the only thing a number can honestly
bound.

And the acceptance the amendment set, the 26 tests (28 rows across the three
arms) that redded under the content-rewrite door. MEASURED at D = 2.5s
through `make coldexec-check RUN='^(…26 names…)$'`, with two sibling seats
holding both suite slots and a third full suite on the box:

| arm | rows | notifications | delayed | warm | older than the door | absent | holes | package |
|---|---|---|---|---|---|---|---|---|
| 1 (`./internal/posse/...`) | all PASS | 756 | 41 | 4 | 706 | 5 | **0** | 25.6s |
| 2 (`-tags posse_arm2`) | all PASS | 226 | 23 | 12 | 186 | 5 | **0** | 45.4s |
| 3 (`-tags posse_arm3`) | all PASS | 620 | 31 | 8 | 571 | 10 | **0** | 48.5s |

**28 PASS, 0 FAIL, 0 SKIP**, with 95 cold first execs held 2.5s each — so
the door was demonstrably injecting over exactly the fixtures whose bytes
the old door had to rewrite, and none of the 26 can see it. The run carried
no `posse coldexec: injecting …` line, which is TestMain's own banner and
its absence is how the acceptance's "the rewrite must be OFF" condition is
read off the log rather than argued (decision 5 below).

That table is THREE runs, not one: twice while the work was in the tree and
once more over the commit, and all three produced the same counts to the
notification — 756/226/620 taken, 41/23/31 held. The third is the one to
cite, because it ran over what landed. The run costs 123s wall for the
three filtered arms on a quiet box; the unfiltered price is
ranger-base-xpxo6's to measure. And the house suite over the same commit:
`make test` rc=0 in 614s, three arms, with this door's own eleven pins
inside arm 1 at 1.392s — so the door is affordable where it now lives and
the suite it measures is green without it.

### The eight decisions

**1. The door's epoch is the supervisor's start, and the PARENT takes it.**
"A plant is an inode whose ctime is after the door started" needs a start,
and it is read one line before `cmd.Start()`, so nothing the run creates can
predate it. A consequence worth knowing before you read a report: `go test`
LINKS its test binary during the run, so the test binary is itself a plant
and every arm delays it once. That is not noise to be filtered — it is the
cheapest proof in the output that the door was actually up.

**2. ENOENT is not a coverage hole, and that distinction is measured.** A
shell searching PATH execs one candidate per directory and misses most of
them. MEASURED over one filtered arm: 5 of 48 notifications were PATH misses
for `bash` (`/home/posse/.local/bin/bash`, `/usr/local/sbin/bash`, …), i.e.
10% of the run. Counting those as "the door could not adjudicate this" — as
the first cut of the report did — puts tens of thousands of non-events in
the holes column and buries the ones that matter. They are `absent` now: an
execve that was going to fail anyway, nothing to delay. ENOENT on a
`/proc/<pid>/…` path can ALSO mean the caller died while we were resolving,
so the pid is checked before the verdict is handed out (`statVerdict`).

**3. The verdicts are named and every notification lands on exactly one.**
`delayed`, `warm`, `old`, `absent` are covered; `unreadable`, `unstattable`,
`gone`, `foreign-syscall` are holes, printed with a count at the end and
with `-verbose` naming each one. The reason for the taxonomy is that a door
whose adjudication quietly stopped working and a door over a suite with no
members of the class print the same count otherwise.

**4. Exit codes are the door's vocabulary: 2 refused, 3 delayed nothing,
otherwise the command's own.** ADR 0085 D5 says zero delayed is a failed
door and not a clean suite; that has to be visible in `$?` or a broken
injection reads as a green suite. It is also why the Makefile BUILDS
`bin/coldexec` instead of `go run ./cmd/coldexec`: MEASURED, `go run` reports
every nonzero exit as 1 and prints `exit status 3` on stderr, which collapses
"your suite failed", "the door refused" and "the door injected nothing" into
one code.

**5. `POSSE_COLD_EXEC_DELAY` is stripped from the child's environment.**
The variable is the supervisor's D — and, until ranger-base-7zkxw, also the
switch that makes internal/posse's `TestMain` install the content-rewrite
door. Leaving it in place would run both doors over one suite: the shim
rewriting every plant's bytes and the supervisor delaying the shim's own
exec. ADR 0085's acceptance is explicit that the rewrite must be OFF for
this run, and this is how: one `childEnv()`, verified by the absence of
TestMain's own `posse coldexec: injecting …` banner from the acceptance log.

**6. The ABI is checked against the running kernel, not trusted.**
`SECCOMP_GET_NOTIF_SIZES` answers the kernel's own sizes for
`seccomp_notif`, `seccomp_notif_resp` and `seccomp_data`, and the supervisor
refuses to start if any of them differs from the struct compiled in here.
With `SECCOMP_GET_ACTION_AVAIL` for `SECCOMP_RET_USER_NOTIF` beside it,
"this kernel cannot do what the door needs" is a refusal before the command
runs rather than a misread field per notification.

**7. An arch the filter does not name is ALLOWed, and a GOARCH it does not
name REFUSES.** Those are different edges. The filter tests
`seccomp_data.arch` because a 32-bit process under this kernel arrives with
a different numbering — i386's syscall 59 is not `execve` — so holding on
the number alone would stop some unrelated call; such a process is allowed
through uncovered, which is stated. Building for an arch with no
`AUDIT_ARCH_*` here (anything but amd64 and arm64) is the other case, and
there the supervisor will not start at all.

**8. The drain after the command exits is bounded at D + 5s.** The listener
answers ENOTCONN only when every process carrying the filter is gone, so a
descendant the run leaked would otherwise hold the supervisor open forever.
Bounded, it says so and goes; execs after that point get ENOSYS, which a
suite reports. `go run ./cmd/checkorphans` is the tool for what leaked —
this is not that tool.

### The pins can still say no

Four mutations, each run against the restored tree afterwards:

| mutation | pin that redded |
|---|---|
| the inode key loses its ctime (`{dev, ino}` alone) | `TestAPlantRewrittenInPlaceIsColdAgain` — 1 delayed, want 2 |
| a relative path resolved against the SUPERVISOR's cwd | `TestARelativePlantPaysD` — 0 delayed, 1 absent, exit 3 |
| the epoch test dropped, so every first exec is delayed | `TestAnOldBinaryIsNotDelayed` — 3 delayed, want 0 |
| `consumeDelayEnv` emptied, so the child inherits the variable | `TestTheDelayVariableReachesTheSupervisorAndNothingUnderIt` — `CHILD SAW [600ms]` |

The second one is the mutation worth keeping in mind: it still printed a
report, and a run that happened to delay one plant by an absolute path would
have exited 0 over it.

### What is still stale until ranger-base-7zkxw

`make coldexec-check` no longer injects at `WriteExecutable`, so the AGENTS.md
bullet's "with that cost injected at `WriteExecutable`, the one door every
exec-bit write in this tree goes through" is now a description of the door
that was, and so is `cmd/testparallel`'s `"TestColdExecHookWiringChild":
"reads coldExecRewrite"` clearance reason and the register row in
`internal/treepins/landingcopy_qa_test.go`. The rewrite itself still exists
in `internal/posse` and is simply no longer reached from the Makefile.
ranger-base-7zkxw owns all of it; left alone here on purpose, because
deleting the rewrite before the supervisor has had its unfiltered run
(ranger-base-xpxo6) would leave the tree with no door at all if that run
turns something up.
