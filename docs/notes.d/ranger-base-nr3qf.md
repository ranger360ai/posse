## Three pin gaps, one per code close (ranger-base-nr3qf)

Filed from ranger-base-f70xj's verify of four closes. All three were pin
gaps over correct, shipped code: a mutant that survives the pin the close
shipped, with a repro, and no behaviour change required. Fixed by adding one
arm per finding, each verified MEASURED against the exact mutant the finding
named, then reverted before committing.

### Finding 1: the landed check's order-only filter (escaped from
ranger-base-r3pqf)

`scripts/posse-landed-check`'s prerequisite derivation drops make's `|`
order-only separator (`:182`, `&& f[i] != "|"`), because `make -p -n` prints
an order-only rule as `test: door-a | door-b` verbatim and a derivation that
planned the separator as a goal would hand a real `make` invocation three
words that are not three goals — `make door-a`, a shell pipe, and a bare `|`
that dies at "No rule to make target '|'. Stop." No arm in
`internal/posse/landedcheckplan_qa_test.go` planted a Makefile with `|` in
it; 13 arms stayed green with the conjunct dropped.

Fix: `TestQALandedCheckDropsTheOrderOnlySeparator`, a fixture
`test: door-a | door-b` asserting the derived plan is `door-a door-b`.
MEASURED: reverting `scripts/posse-landed-check:182` to
`if (f[i] != "") print f[i]` reds exactly this one arm
(`go test -tags posse_arm2 -run '^TestQALandedCheck' ./internal/posse`);
the other 13 stay green, matching the bug's own reading.

### Finding 2: coldexec's in-flight charge key (escaped from
ranger-base-91fu2)

`cmd/coldexec/door_linux.go:597`'s in-flight charge is keyed by
`(pid, inode)`, and the `pid` half was asserted by no arm — ARM 18
(`TestANonLeaderThreadsExecIsAdjudicatedUnderItsOwnTid`) pins the KERNEL fact
that makes the key unique (a notification names the task, not the thread
group) but never exercises the door's OWN use of it. Zeroing the pid
(`charge{pid: 0, key: key}`) leaves ARMs 14/15/16/17/18 green, because none
of them puts two DIFFERENT tasks on one fresh inode at the same time — every
restart/withdrawal arm is one task signalled mid-exec, not two tasks racing.

Fix: `TestTwoTasksExecingOnePlantConcurrentlyChargeOnlyOneOfThem` (ARM 21),
two `sh`-forked tasks exec'ing one freshly planted binary concurrently
(`./plant & ./plant & wait`). Shipped: the door's cost model charges an
inode once, not per task (ADR 0085 D5), so this reads 1 delayed, 1 warm.
MEASURED: zeroing the pid collides the two tasks' charges — the loser's own
exec is misread as the winner's restart and sleeps the remainder too — and
this run reads 2 delayed, 0 warm:

    supervisor_linux_test.go:1127: the door reports 2 cold execs delayed, want 1

ARMs 14-18 stayed green under the same mutant, confirming the bug's claim
that nothing else in the file reaches this.

### Finding 3: the inode preflight's no-reading rule (escaped from
ranger-base-vw1cp, landed as the ranger-base-xjehw replay)

`scripts/test-times.sh`'s `free_inodes()` treats `ifree == iused == 0` as
"this filesystem accounts no inodes" rather than a real reading of zero
(`:286`). `--self-test`'s own I3 arm cross-checks this independently, but
only over `$TMPDIR`, and every seat's `$TMPDIR` here is `/tmp`, a tmpfs with
a real table — the zero-zero branch belongs to `GOCACHE` alone (measured:
`/home` prints `0 0 0 -`), and the self-test's stub `go` has never named a
real `GOCACHE`, so that second filesystem's line never appears in `$out` at
all. Replacing the guard with `if (0) exit` left `--self-test` green, all 68
arms, over the default `$TMPDIR`.

Fix: a new arm (I4) hands the self-test's `faketest` stub the ONE real
answer the one call that needs it requires — `REAL_GOCACHE`, exported only
for that one `run_arm` invocation, answering `env GOCACHE` with the real
`go`'s real cache directory — and reads the resulting DISK line the same
independent way I3 does, via a shared `ifree_reading()` helper pulled out of
I3's own inline parsing. Every other arm's stub call is unaffected.
MEASURED, this box: `go env GOCACHE` is `/home/posse/.cache/go-build` on
`/dev/mapper/root` mounted at `/home`, `df -iP` reads `0 0 0 -`; `$TMPDIR`
(`/tmp`) is a tmpfs with a real table. Reverting the guard to `if (0) exit`
reds exactly the new arm:

    FAIL  inodes: the real go build cache filesystem accounts no inodes and the preflight neither said so nor printed a count for it
          | test-times: DISK: 854273 free inodes on /tmp — t.TempDir
          | test-times: DISK: 0 free inodes on /home — the go build cache
          | test-times: FREE INODES BELOW THE 65536 FLOOR — ...

— the exact false alarm the rule exists to prevent, read by the self-test
for the first time.

### What was not re-filed

ranger-base-nr3qf's own description names a fourth, unreproduced item (the
bead filer's id-parsing fragility in `scripts/reap-test-tmp.sh`) as
deliberately NOT filed, for want of a repro on this box. Left as is; nothing
here bears on it.
