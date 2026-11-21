# G18 trusted a recorded pid for the length of its own reading (ranger-base-bekle)

A pid is not an identity. G18's verdict — "the dispatch loop runs an image the
install replaced" — is built entirely out of `/proc/<pid>/exe` for the pid this
home's `dispatch-watch.pid` names. Liveness is sound and always was: it comes
from the watch lock, which the kernel holds and releases at process death, so
there is no staleness class in it (ADR 0011 §1, rangerhq-gir5). But the lock
answers "a loop of this home is running", not "the loop this record describes
is running", and the record's own truth decays — a loop killed with its pane
never removes it. `pidAlive`'s comment in internal/posse/watchpid.go has said
so since gir5: "it answers liveness and not identity — a recycled pid is alive
and is a stranger — so no caller may use it alone as proof that the process it
recorded is the one there." This reader was such a caller.

## What it rendered

MEASURED 2026-11-21 14:36:09Z, from a full arm-1 run on this box (16 cores,
loadavg 10-12, two sibling seats' suites running):

```
watchimage_qa_test.go:77: before the swap the reading must come from /proc
  and name /tmp/TestQAG18…2305482577/001/posse:
  ok=true {Pid:3027437 Started:… Path:/tmp/go-build2655127489/b252/posse.test
  Image:57:203948144 OnDisk:57:203948144 Version: Now:0.5.3+dev Via:proc …}
```

`ok=true`, `Via:proc`, `Image == OnDisk`: a reading that is internally
consistent and about a different process than the one the record named — a
fork of the test binary that had taken the dead stand-in's pid. In production
the same two shapes render as a LANE row reading `dispatcher pid N runs an
image no longer at that path … restart the dispatch loop, then relaunch
seats`, about a pid that is not the loop, with the version dropped (the reader
already knew the stamped image was not the one /proc named — it acted on that
fact only by deleting a word from the sentence) and the stranger's path kept.

The cost of the shape in the suite is a second one: `make test` aborts the
recipe on its first non-zero exit, so a red arm 1 takes arms 2 and 3 with it,
and whoever reads that output re-runs the whole thing to learn whether
anything else was red.

## The two halves

- **The fixture half was ranger-base-rph5z's**, landed as c9ca4d6e an hour
  after the reading above and not present on the branch that produced it
  (bf48d1a5): `rrgncStartLoop` waits until /proc says the child really runs
  the fixture's binary, `rrgncChildStillOurs` says it again before every
  reading, and the stand-in outlives the package timeout instead of 60s. 2
  failures in 20 runs before, 40 clean after; 40 clean again here with this
  change on top.
- **The production half is this bead.** It is not a test change and it was
  deliberately left open: telling "a dead pid, reused" from "a /proc read
  inside a fork/exec window" needs the reader to check what the record
  recorded, which it never did.

## What changed

`internal/posse/watchimage.go`. The reading is taken only once the record can
be matched to the process holding its pid (`watchPidStranger`), by two
comparisons that are each one-sided because only one direction is evidence:

- **A stamped image that is not the image /proc names.** The image a process
  runs cannot change without an exec, and rename(2) — this file's whole
  subject — does not touch it, so a mismatch is never this loop having moved.
- **A process that started after the record was written.** A process cannot
  predate the record that describes it, so this is pid recycling and nothing
  else. The converse says nothing: a loop's start precedes its own stamp by
  however long its preamble took, which on a loaded box has no bound worth
  asserting.

A pid neither comparison can speak about is an abstention too, so the
invariant is total: this reader never reports on a pid it could not attribute.
An abstention carries WHY (`WatchImage.Why`) and `ReadWatchImage` returns
ok=false for it, which its own doc comment already claimed; the install
report's line then says the loop could not be identified instead of that none
could be read, because the loop IS running — its lock says so — and an
operator sent looking for a missing loop looks for nothing.

The start time comes from the mtime of `/proc/<pid>` itself, not from field 22
of `/proc/<pid>/stat`: the directory's mtime IS the task's start as wall
clock, so it needs neither the boot epoch nor USER_HZ. MEASURED 2026-11-21
(linux 7.2.15-1-omarchy, go1.27.3): for a child this process had just forked
it read 1.77µs BEFORE the parent's own `time.Now()` taken immediately before
`exec.Cmd.Start`, was identical across four reads 1.2s apart, and answered
ENOENT the instant the child was reaped; the two dispatch loops live on this
box both read 2026-11-21T07:11:54 and pid 1 read the boot time. It is also the
read that still answers where `/proc/<pid>/exe` is refused, which is how
ranger-base-rph5z tells a refused pid from a dead one — so the identity check
does not evaporate on the reader a cage gives us, and the record road uses it
there too.

**The comparison truncates to the second, and that is not a slack.**
`WriteWatchPid` writes `started:` as RFC3339, which carries no sub-second
part, so the stamp an honest loop wrote reads up to a second before its own
start time. MEASURED here by mutation: comparing untruncated reds three live
pins immediately, including the one this bead is about — a flake of exactly
the class the bead was filed for, introduced by the fix for it.

## Mutation

Six mutants, each rule inverted separately, each red on the pin named for it
(`-run '^TestQAG18|^TestQAWatchPidCarries|^TestQAUpgradeNamesTheLoop|^TestQAEveryLoopReading|^TestQAALoopReading'`):

| mutant | red |
|---|---|
| no identity material is read anyway | `nothing_in_the_record_to_match_the_process_against` |
| the stamped-image comparison | `a_stamped_image_that_is_not_the_image_/proc_names` |
| the start-time comparison | `a_pid_reused_after_the_recorded_loop_died` |
| the truncation | `a_start_time_later_in_the_same_second_as_the_stamp`, and both live pins |
| the record road's own check | `a_refused_exe_link_over_a_pid_that_started_after_the_record` |
| an abstention returned as a reading | `TestQAG18ReadsNoVerdictFromARecordWrittenBeforeTheProcessItNames` |

## What this does not do

An abstention is silence where a stale loop may really be stale, and nothing
here raises a row for "the record does not describe the running loop" — the
condition the abstention now names. What covers that gap is the reading that
does not trust a record at all: the box-wide census (ADR 0092 D8,
ranger-base-is1qp), which finds every loop on this binary from /proc and is
printed beside this line by the same verb. A row of its own would be a new
governance condition and is the design lane's call, not this bead's.
