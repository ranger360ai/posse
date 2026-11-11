# Four closes verified, and the half of a rule its own close argued for (ranger-base-aifu2)

Verify batch of 2026-11-11: ranger-base-j0riv (gwart), ranger-base-8vk23,
ranger-base-orgvt and ranger-base-akk9f (jian-yang). All four verified. Two
live findings, both against j0riv, bundled as **ranger-base-dgbys**; two
recorded on the verify bead's close and no bead filed for either.

Read at `HEAD == main cf6506e4`, clean tree, linux 7.2.15 x86_64, go1.27.3.
Every commit the sections named is an ancestor of that HEAD (`ef842dbe`,
`57dbf70b`, `537cd425`, `0b4bafb8`, `cf6506e4`); gwart's close names
`47a71d4e`, which is the pre-rebase sha of `ef842dbe`.

Eight mutants in all, each reverted with `md5sum` identical before and after
and `git status --porcelain` empty afterwards.

## The two live findings

### 1. A rule shipped in two parts, and only one part has a pin

`ranger-base-j0riv` landed one rule with two halves:

- **(a)** `withdrawn` is not a hole — the old `gone` verdict, which named the
  wrong thing, leaves `holes`.
- **(b)** a hole is an execve the door let through without knowing what it
  was, and *whether it was let through at all is decided at the ANSWER and
  nowhere else*: `adjudicate` records no hole, every hole rides down to
  `answer()` and is recorded only once `ID_VALID` has said the notification
  was live.

ARM 14 (`TestAWithdrawnNotificationIsNotAHole`) pins (a) and does not reach
(b). MEASURED by mutation at `cf6506e4`:

| mutant | result |
|---|---|
| `vWithdrawn` back into `holes` (`door_linux.go:169`) | FAIL ARM 14, nothing else |
| the `ID_VALID` arm stops recording `vWithdrawn` | FAIL ARM 14, nothing else |
| **(b) reverted** — `handle` records the pending hole itself before the answer | **`ok 1.572s`, all 15 arms; `ok 14.372s` under `-count=10`** |

The third is three lines, and it is exactly the state the close describes
rejecting:

```go
pending, pendingTarget := d.adjudicate(n)
if pending != vNone {
	d.record(pending, int(n.Pid), pendingTarget)
}
d.answer(n, vNone, "")
```

**The sentence to read as a mutation recipe is the one describing the narrow
fix the author rejected.** j0riv's close says: "Reclassifying `withdrawn`
alone left the same bug twice over: 1500 runs after that narrow fix still
produced one un-adjudicated run, because the caller can be UNREADABLE, or its
pid gone, precisely BECAUSE the kernel withdrew the notification under us."
So the close knows (b) is what keeps the bead's own symptom away, and shipped
it unpinned.

**The fixture is ARM 14's with one character changed.** j0riv measured three
endings of a withdrawn execve; it pinned the first:

| the caller's handler | what the execve does | the caller |
|---|---|---|
| with `SA_RESTART` (the Go runtime's shape) | RESTARTS, fresh notification | lives, exit 0 |
| without `SA_RESTART` (`siginterrupt(sig, True)`) | returns `EINTR`, never happens | **exits** |
| none, default-fatal | never happens | **dies stopped** |

The last two are the shapes in which the caller is unreadable or gone at the
door's `readPath`/`stat` — the case (b) exists for. So: ARM 14's script with
`signal.siginterrupt(sig, True)` instead of `False`.

MEASURED 2026-11-11, same fixture, both doors:

| door | runs | runs exercising a withdrawal | runs reporting un-adjudicated execs |
|---|---|---|---|
| shipped, `cf6506e4` | 210 | 198 | **0** |
| with (b) reverted | 250 | 248 | **11** (8/60, 0/40, 3/150) |

One of the eleven, verbatim — the same notification carrying both verdicts,
and a report accusing the door of a hole it could not have filled:

```
coldexec: unreadable  pid 3258444 open /proc/3258444/mem: no such process
coldexec: withdrawn   pid 3258444 (the kernel took the notification back before the answer)
coldexec: 4 execve notifications, 0 cold first execs delayed 600ms each (0 warm, 3 older than the door, 0 absent)
coldexec: 1 execs went un-adjudicated (1 unreadable) — those were NOT covered; -verbose names each one
```

**Not a complexity finding, checked the other way** (operator standing order,
2026-09-03): the smallest diff that turns ARM 14 red-then-green is the
reclassification alone, and that smaller diff is the one the close measured
and rejected on evidence. The rule that landed is the right size; what is
missing is a second pin, not a smaller fix.

**The number the pin needs and nobody has.** The hole is a race — about 4% of
runs under the mutant on a quiet box. j0riv's original red lived in arm 1's
concurrency, where it will be higher, and no load was generated to find out
(standing ruling: no load testing on this box). `ranger-base-dgbys` says so
rather than offering 4% as the answer. The withdrawal itself is near
deterministic (248 of 250), so it can be the pin's own positive control the
way ARM 14 already uses it.

### 2. A withdrawn-then-ABANDONED execve leaves its plant marked warm

Deterministic, 30/30, and found after the bundle was filed.

`adjudicate` marks `d.seen[key]` (`door_linux.go:506`) and records `vDelayed`
(`:515`) before it sleeps, and the comment at `:512` says that charge "stands
even if the notification is withdrawn under us, because the restart that
follows reads the inode as warm."

That reasoning holds for the `SA_RESTART` ending only. In the other two
endings above there is no restart: **the execve the door charged D for never
happens, and the inode stays marked.** So the next caller's genuine FIRST
execve of that plant reads `warm` and goes through at full speed, under a
report saying the class was injected.

MEASURED 2026-11-11 against the shipped door, 30 of 30 runs. Verbatim from
one:

```
coldexec: delayed     pid 2472383 /proc/2472383/cwd/./plant
CALLER EXIT 9
coldexec: warm        pid 2472406 /proc/2472406/cwd/./plant
the plant ran
ELAPSED second 5ms
coldexec: withdrawn   pid 2472383 (the kernel took the notification back before the answer)
coldexec: 11 execve notifications, 1 cold first execs delayed 600ms each (1 warm, 9 older than the door, 0 absent)
```

Read the two pids. `2472383` was charged 600ms for an execve that never ran;
`2472406`'s exec — the only exec of that plant that happened, by a different
process, the first real one — took 5ms. "1 cold first execs delayed 600ms
each" is true of nothing.

**It is a coverage claim, not arithmetic.** ADR 0085 D5 is that every first
execve, by any process under the door, of an inode created after it started is
held D. This is a counterexample, and j0riv's close asserts the opposite in as
many words — "D5's coverage claim ... is unchanged and still true, because the
restart re-covers the withdrawn notification" — which is true of the ending it
pinned and false of the two it measured and did not.

**Overlap with `ranger-base-4kars`, stated so nobody fixes it twice:** same
line, probably the same fix (moving the `d.seen` mark), different claim. 4kars
is the *restart* case, where the exec happens and is merely truncated to
10-20ms against the 600ms charged. This is the *no-restart* case, where the
charged exec does not happen at all and a different process's genuine first
exec is the one let through. 4kars' own note — that moving the mark is also
what stops two concurrent first execs of one inode from both paying D, and
that the cost rests on an unmeasured number — still applies.

A pin here is cheap because it is deterministic: the charged-and-abandoned
execve, then the next exec of the same plant paying D.

## Recorded, and no bead filed (ADR 0006 §6)

Both are prose that no process and no pin reads.

- **`cmd/coldexec/door_linux.go:78`** — "Every notification lands on exactly
  one verdict ... One verdict per NOTIFICATION, note, not per execve" is false
  by one, deterministically, in ARM 14's own run. A delayed notification
  withdrawn inside the D sleep records BOTH `delayed` and `withdrawn`.
  MEASURED 12/12 runs of that fixture: 5 notifications, 6 verdict lines, with
  `delayed` and `withdrawn` on one pid and `warm` as the restart's own
  notification. The record site's comment says the right thing; the struct
  comment contradicts it. Escaped from ranger-base-j0riv.
  Summing a report's own counters is a cheap attack on any door that prints
  them.
- **`ranger-base-8vk23` (the bead record)** — carries no record of its own
  close. No description, no comment naming `ranger-base-r5tnw`
  (`bd show | grep -c r5tnw` → 0), and `bd dep list` holding only the
  `discovered-from` edge to orgvt. The duplicate statement lives solely inside
  another bead's close comment. Its one comment points a reader at
  `ranger-base-qregu` and says "Whoever gets there first should say so on the
  other", which nobody did on this side — so a reader of 8vk23 alone cannot
  tell the defect was fixed. Escaped from ranger-base-8vk23.

## The other three closes

### ranger-base-8vk23 — verified on the substance; acceptance MISSING

The bead carries no description, so it states no completion criteria and none
were supplied here. That is this section's limit.

The defect is gone, and by `ranger-base-r5tnw` rather than by anything on this
bead: `cxaUncoveredPlanters` is `map[string]string{}`
(`internal/treepins/coldexecarms_qa_test.go:152`) since `537cd425`, and
`TestQAColdexecArmRegisterIsNotStale`,
`TestQAEveryPlantingPackageHasAColdexecArm` and
`TestQAColdexecArmCensusCanStillSayNo` are `ok 0.287s`. `make tree-check` exit
0, so `corpus-check` no longer aborts it at `Makefile:974`. The duplicate call
is also correct: r5tnw is the same two rows, filed earlier and with the
stronger repro.

### ranger-base-orgvt — verified

ARM 15 (`TestAnOrphanedDescendantsPlantPaysDAndLeavesNoZombie`) green. Two
mutants, each red it and nothing else, each on its own message: the `prctl`
removed (the orphan's ppid is not the supervisor's, so the pin says it is
measuring the pre-fix door) and the reaper never started (the orphan still
answered `kill -0`; 11.5s).

Both claims the close rests on were re-checked rather than taken:

- **The census** in `doc.go:88` and ADR 0085 D5 — nothing in the five doored
  arms reads a planted orphan's ppid as 1 — holds. Every `PPID: 1` in
  internal/posse is a synthetic `Proc{}` table row; `orphancontrol_test.go` is
  `//go:build posse_arm3` *and* gated on `RHQ_ORPHAN_CONTROL=1`, so it skips;
  and `cagelauncher.go:347`'s `for os.Getppid() == ppid` asks "is my ORIGINAL
  parent still my parent", which survives reparenting onto a subreaper. **A
  ppid read that is relative rather than absolute is immune, and that is the
  shape to look for.**
- **The liveness claim, where the close's own run did not reach.** Its
  end-to-end was `internal/treepins -run SuiteLock`, and the arms that
  actually exercise process liveness sat outside that filter. Running them
  under the door at D=300ms —
  `PatternKill|GroupReaper|PkAudit|ReapingScripts|LandingCopy|SuiteLockDeadline`
  — cost 2.014s: ok, 46 notifications, 0 holes, 1 orphan reaped.
  `TestTheGroupReaperEndsWhatThePatternKillCouldNot` reads `kill -0` and
  `pgrep -g`, both of which `reapOrphans` leaves meaning what they meant. **A
  filtered end-to-end run is a scope claim to re-run.**

**A NOTE, not a finding, because it confirms rather than contradicts.** The
close and `reapTick`'s comment say the `pid == childPid` skip "reds NOTHING"
and that its window "did not open in any run here", calling it a guard rather
than a pinned behaviour. True at the shipped 200ms tick: 0 of 40 direct runs
here, and 0 of 5 rounds of `-count=20` against 0 of 5 for HEAD. The window
*is* real, though, and it costs the exit code `make` reads — with the skip
removed **and `reapTick` at 1µs**, 1 of 40 runs answered 1 instead of the
command's 7, with `coldexec: waiting for the supervised command: waitid: no
child processes`: the reaper taking the status `os/exec` was blocked on. Two
factors, so the fast tick alone is exonerated (skip present + 1µs tick: 0 of
40). So the guard is load-bearing, the comment's reasoning about why it is
unpinned is sound, and leaving it unpinned is a disclosed judgement and not an
escape. **Widen a race rather than chase it** — a single unreproducible suite
red proved nothing; two minutes of this decided it and printed the mechanism.

### ranger-base-akk9f — verified, and it earned its keep inside this verification

`--self-test` ok, 63 arms. Four mutants of my own, each red the arm it should
and no other:

| mutant | the arm that red |
|---|---|
| the `edquot` grep made case-sensitive | `edquot: no block naming the count` |
| the quota header printing the ENOSPC sentence | + `edquot: reported as ENOSPC — a quota red sent to df, where it looks fine` |
| the preflight caveat removed | `disk: the preflight prints free space with no word that a quota binds sooner` |
| the quota block made unconditional | + `enospc: reported as a quota too`, `clean log: quota block printed with no EDQUOT in the log` |

The treepins guard reds for the right reason too: renaming one arm label gives
`suitedisk_qa_test.go:145: … --self-test no longer proves "edquot: not
reported as a full filesystem" — the arm is gone, failing, or renamed`.

Rendered live on this box: `DISK: 2862 MB free on /tmp — t.TempDir`, then
`that is the DEVICE — /tmp is mounted usrquota, so your share of it runs out
first and df cannot see that (EDQUOT, ranger-base-akk9f)`, then the floor
warning.

**And it worked on me, three times, which is better evidence than any
fixture.**

1. The first `make tree-check` on the house root died at 41s on `compile:
   writing output: write $WORK/b132/_pkg_.a: disk quota exceeded`, surfaced as
   `make crew-check` "reports drift that is not there" — krra's shape exactly,
   through a door whose name has nothing to do with disks.
2. Through this bead's documented TMPDIR-on-`/home` workaround the same
   command came back exit 0 in 40s, 17/17 doors. Then `cf6506e4`'s own
   correction earned itself: `make treepins` under that TMPDIR red
   `TestClaudeConfigDirIsFenced` ("the trust probe never names the fenced
   config dir /home/posse/.cache/posse-tmp-…"), and that one test on the house
   default root is `ok 0.425s` — the workaround's artifact, which is the CLASS
   `cf6506e4` landed to say rather than the two pins the 10:41 comment had
   said.
   One wording correction, nobody's defect: `TestClaudeConfigDirIsFenced`
   exists in BOTH `cmd/posse` and `internal/treepins`
   (`configdirfence_test.go` in each). The close names the cmd/posse one; the
   red a full treepins run produces is treepins'. **Say which package.**
3. The same `make treepins` on the house default root came back exit 2 with
   seven reds — `TestQAHookFreshnessReadsTheChainedLayout`,
   `TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared`,
   `TestQATheRaceArmCensusRefusesTheWrongShapes` and four more, every name
   unrelated to every other — and the explainer caught it:

   ```
   test-times: THE DISK QUOTA RAN OUT — 11 failure(s) said "disk quota exceeded"
               (this run reported 7 failing test(s) in total)
   ```

   followed by the EDQUOT half naming the per-UID budget and the workaround.
   That is krra's shape on a REAL red rather than a fixture, and before
   `0b4bafb8` that log would have gone from FAIL straight to the package table
   with no cause named anywhere. **The bead's premise is discharged on this
   box, live.** The quota condition itself is `ranger-base-32bdv`, open.

## What ran, and what did not

Ran: `go test ./cmd/coldexec` ok 1.780s (15 arms) and ok 14.321s under
`-count=10` over ARMs 3/14/15; the three coldexec-register pins ok 0.287s; the
two suitedisk pins ok; the full `internal/treepins` package through `make
treepins` twice, both roots (both reds accounted for above); the treepins
liveness arms under the door, ok 2.014s, 0 holes;
`scripts/test-times.sh --self-test` ok, 63 arms; `make tree-check` exit 0 in
40s, 17/17 doors. `go run ./cmd/checkorphans` clean; both background suite
runs took suite slot 2 through `make treepins` and both exited. No bd write
taken on its exit code — the create, both comments, the retitle and the edge
were each read back.

Not run, and why: `make test` arms 2 and 3. All five commits touch only
`cmd/coldexec`, `internal/treepins`, `scripts/` and `docs/` — internal/posse,
which is the whole of arms 2 and 3, is untouched by every one of them — and a
full-suite reading on this box today is the subject of `ranger-base-akk9f` and
`ranger-base-32bdv` rather than a thing this bead could get: the one
full-package run taken on the house default root died on the quota, as above.
Also not run: `make coldexec-check`, five arms and most of an hour, which is
`ranger-base-xpxo6`'s number.

## How to exercise cmd/coldexec

`go test ./cmd/coldexec` is ~1.8s and all 15 arms. Mutate `door_linux.go` in
place and `git checkout --` between arms: the package is small, the rebuild is
the cost, and nothing else in the tree reads the file.

To drive the door by hand, which is what `runDoor` does:

```sh
POSSE_COLD_EXEC_DELAY= <door> -delay 600ms -verbose -- /bin/sh -c '<script>'
```

in a fresh cwd. The report's own lines are the assertions: `un-adjudicated` is
a hole, `withdrawn   pid` is the j0riv mechanism firing, and `orphaned
descendants were reparented` is the orgvt arm's witness. Sum the per-verdict
lines against the notification count — the door prints both and they do not
have to agree.
