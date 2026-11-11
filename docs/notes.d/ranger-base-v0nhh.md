# Four closes verified, and the leak the triage blamed the wrong dirs for (ranger-base-v0nhh)

Verify bead over four closes: ranger-base-xpxo6 (dinesh, `-l code`),
ranger-base-byc2i (dinesh, `-l code`), ranger-base-32bdv (gilfoyle,
`-l devops`) and ranger-base-4kars (gwart, `-l code`).

All four verify. Eight mutants across three of them, every file restored and
md5-checked byte-identical. Two findings, both RECORDED (ADR 0006 §6): a
measurement record's prose and a code comment, neither read by any process or
pin. No findings bead filed.

The one thing a reader of main should know before anything else: **4kars's
work is not on main.** `12a40610` is stranded on
`posse/gwart-posse-ranger-base-4kars`, the merge-back block is
ranger-base-v60dy, and everything verified for that bead below was verified
against the branch.

MEASURED 2026-11-11, this box (linux 7.2.15 x86_64, go1.27.3, 16 cores),
unless a line says otherwise.

## ranger-base-byc2i — verified, and the mutant pair says why it mattered

The close is "already fixed on main by ranger-base-r5tnw, verified anyway".
At `04c881a6` the register is `var cxaUncoveredPlanters = map[string]string{}`
(internal/treepins/coldexecarms_qa_test.go:152) and the bead's own three pins
are `ok 0.244s`.

Three mutants, because "does the pin red" was never the interesting question —
it red loudly on main, which is how the bead was filed. What the fix bought is
the derivation, and that takes the pair:

| mutant | result |
|---|---|
| the two waiver rows back, arms intact | `TestQAColdexecArmRegisterIsNotStale` FAIL, naming cmd/posse and internal/treepins in byc2i's own words |
| `./cmd/posse/...` arm deleted, register EMPTY | `TestQAEveryPlantingPackageHasAColdexecArm` FAIL: "cmd/posse plants an executable and `make coldexec-check` has no arm reaching it", 17 call sites, the door's three remaining patterns printed |
| `./cmd/posse/...` arm deleted **and the waiver row back** | **`ok 0.381s`** |

The third row is the finding. In the state byc2i fixed, deleting either of
ranger-base-in8dr's new arms was SILENT — which is exactly what the
register's own message says a stale row costs, and it is the half that makes
a two-line deletion a P1. dinesh's reading of the two pins (the derivation
strictly strengthened, the stale-register pin benignly vacuous, the control
independent of the register) holds on the source.

`make tree-check` at `04c881a6`: **green, 42.1s**, default `/tmp` root,
loadavg 5.38, both suite slots free — so corpus-check, which is what byc2i
was filed about, is clear for every seat.

## ranger-base-32bdv — verified; both pins are live and the residual is real

Landed as `8fd56c6b` (the launcher's rebase of gilfoyle's `39435e7b`; same
six files, same stat).

Both new pins fail on the defect they were written for, and the leak comes
back with them:

| mutant | result |
|---|---|
| `home, ownHome := "", false` (the child stops adopting) | `TestTheTempHomeIsOnePerTestTree` FAIL — "it cut its own, and a parent that kills it leaves that dir on a tmpfs every seat shares" — **and 1 dir left behind** |
| `MISE_DATA_DIR` not set | `TestMiseDirsArePinnedOutsideTheTempHome` FAIL — "a test that execs a mise command-wrapper ... makes mise re-install the toolchain under the temp HOME" |
| neither (HEAD) | `ok 2.0s`, **0 dirs left behind** |

The residual the close declares is real and I collected it by accident. A
full doored arm 3 of internal/posse run for xpxo6's sake below died on
`panic: test timed out after 1h0m0s` and left **exactly one**
`posse-testhome-*` — which is herdr_test.go:193's own sentence ("the parent's
own abnormal exits ... one dir per run and loud") measured from the outside.
`scripts/reap-test-tmp.sh --root <that dir> --min-age 0` found it, named its
size, and stopped at the dry run: "collectable: 1 dirs, 11M".

During the arm, the count under its TMPDIR was **1** — one per test tree,
which is the pin's whole claim.

`make verify-parallel` clean (3043 eligible tests), `make scripts-check`
green. The reaper carries no verify-* target and needs none: `scripts-check`'s
roster census passes without one.

## ranger-base-xpxo6 — verified, including the one thing a focused run cannot show

The deliverables are all on main: the fragment (314 lines, walls, counters,
ctx table, triage), ADR 0085 D6's price paragraph replaced by the
measurement, and the Consequences bullet that PREDICTED in-class reds now
carrying the result instead.

**The arithmetic closes.** Every derived figure in the close and the ADR
reconciles against the four walls:

- `1964.070 / 275.703 = 7.125` → the stated 7.12x
- `1688.4 / 571.1 = 2.956` → the stated 2.96x
- `1622 x 2.5s = 4055.0s` nominal; `1688.4 / 4055.0 = 41.6%` realised
- the per-arm 48.1/49.9/29.6% back-solve to 700.4 + 322.9 + 598 = **1621.3**
  cold execs against the stated 1622
- `275.703` against kiwvn's `280.289` = 1.64% → the stated "within 1.6%"
- `1964.070 + 674.8 + 1581.8 = 4220.67s` → the stated 4220.7s five-arm figure

**The ctx table reproduces exactly.** With the ctx as the only variable,
`-count=5` each on a quiet box: 200ms **FAIL**, 300ms `ok`, 500ms `ok`. That
is dinesh's "50/100/150/200ms fail deterministically, 300ms up pass", and it
is also why the pre-fix 500ms passed everywhere except where it didn't.

**The pin is live.** Watch's `watchdogLoop` call replaced by
`<-dogCtx.Done()`: FAIL with the pin's own fatal. watch.go restored
byte-identical.

**And the red reproduces, which no focused run can show.** The close says the
focused run passes under the door either way, and it does — 3/3 PASS, 29
notifications, 2 cold execs held, and the pre-fix 500ms ctx also passes 3/3
focused. So the fix's necessity rests entirely on the loaded arm. I ran it:
the whole doored arm 3 with the ctx reverted to 500ms, D=2.5s, through
test-times.sh and a suite slot.

```
--- FAIL: TestQAWatchStartsTheWatchdogAndItNamesAStalledPass (5.03s)
    watchhang_qa_test.go:736: a pass stalled in a hung bd child past the budget
    was never named — Watch did not start the watchdog:
      ...
      ── pass 1 · 07:49:38
      ◷ watchdog: no pass has completed for 3m — past its 0s budget
        (last pass 04:53:24). ...
```

The mechanism is visible in the failure's own output: the dog DID fire, and
it produced the **pass-stall** line, not the `written nothing for` line the
test reads. The window for the write-silence reading closed before a tick
landed, which is precisely the diagnosis the fix is built on. The fix is
necessary and the account of it is right.

That run's door counters also reproduce dinesh's arm-3 column independently:
**598 cold first execs delayed** (his: 598, twice), **1723 absent** (his:
1722, twice), 84537 notifications (his: 83362/83004). And the post-orgvt
supervisor reports what his caveat predicted: **0 un-adjudicated**, with
"1148 orphaned descendants were reparented onto this process and reaped".

Two other tests red in that arm —
`TestQAGrantStillReachesTheGitConfigInEveryShape` and
`TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch`. Both are
ranger-base-akk9f's `$HOME`-root artifact class and neither is anybody's
regression: on the default `/tmp` root the first is PASS and the second SKIPs.
The second one's own failure prints the reason — its writable set ends in
`/tmp`, so nothing under a `/home` TMPDIR can be in it.

**My wall is not comparable to his and is not offered as a correction.** That
arm took >3600s against his 538.9s, on a `/home` TMPDIR (he ran on the tmpfs,
as every earlier reading did), beside a sibling seat's suite, at loadavg
13.9. It hit `-timeout 60m` with tests still running, so its FAIL tally is a
floor and not the arm's result. The pin's verdict is what it was run for.

## ranger-base-4kars — verified against the branch; nothing of it is on main

`12a40610` is NOT an ancestor of main and nothing of it reached main another
way: `git grep vTruncated main -- cmd/coldexec/` is empty, and so is
`TestAWithdrawnNotificationTruncatesTheInjection`. Its merge base is
`ef842dbe` and ranger-base-orgvt has since landed in the same three files,
which is the conflict ranger-base-v60dy records. The four beads the close
filed all exist with their `discovered-from` edges confirmed by
`bd dep list`: ranger-base-91fu2, -tow7x (closed), -w5z7u, -9uvsx.

cmd/coldexec imports nothing from this repo, so its package at `12a40610`
runs unchanged in this tree. `go test ./cmd/coldexec -count=3`: **ok 4.286s**,
15 arms.

**The mutation the close names behaves as it says.** Restoring the pre-4kars
shape — `d.record(vDelayed, pid, target)` before `time.Sleep(d.delay)`,
returning `vNone` — reds ARM 15 and leaves ARM 14 green, which is
ranger-base-j0riv's deliberate agnosticism holding. ARM 15 fails at its own
vacuity guard ("no truncated verdict names the plant, so this run did not cut
an injection short and the count below proves nothing"), which is the right
place for it to fail.

**The marker fix is real, both directions.** Under a TMPDIR whose path
contains `ran`:

| version | result |
|---|---|
| `12a40610` (shouted markers) | `ok 1.367s` |
| main (lower-case `the plant ran`) | `--- FAIL: TestFirstExecOfAPlantedScriptPaysDAndTheSecondDoesNot` |
| main, clean TMPDIR (control) | `ok 1.558s` |

So main's ARM 1 is wrong about what it measures whenever a seat moves TMPDIR
somewhere `ranger-` appears — which is the workaround four seats invented
yesterday — and ARM 12's `Contains` would pass on the path with the plant
never running. The fix for that is written; it is sitting on the stranded
branch. That is the price of ranger-base-v60dy, not a new defect, and it is
not filed again here.

The census the bead existed to produce holds up where it can be checked
statically: `internal/posse/bdwriteshim.go:95` is a real
`syscall.Exec(real, argv, os.Environ())`; three of the four named test
families live in internal/treepins (the "treepins eleven") and the fourth,
`TestRenderedShimRoutesAnUnprojectedWriteThroughPosse`, in internal/posse
(the "arm 1 is the only racy one"). The table closes: 696+320+598+231+608 =
2453 charges, 2441 paid + 12 truncated = 2453, 12/2453 = 0.489%.

## RECORDED findings (ADR 0006 §6) — no bead filed

**1. `docs/notes.d/ranger-base-xpxo6.md:211` blames the wrong dirs for the
inode exhaustion.** It says "The inodes were eaten by **425 abandoned
`/tmp/posse-testhome-*` trees holding 5.2G**". MEASURED today, with those
same dirs still in place: 428 `posse-testhome-*` dirs hold **7,650 inodes of
the 1,048,576-inode table — 0.73%**, against the 1,041,640 in use at the
moment of failure. The real consumer was ~24.8k `/tmp/tmp.*` silent-revert
fixtures at ~50 inodes each; after the operator reclaimed those, `/tmp`
reads 11% with every one of the 428 still there. Escaped from
ranger-base-xpxo6's close. It is prose in a measurement record and nothing
reads it: ADR 0085 does not repeat it, and ranger-base-vw1cp — which inherited
the same sentence — was corrected by its own assignee, who measured the real
shape and filed ranger-base-b27pn ("94% of /tmp's inode table"), closed, fixed
on main at `14328f38`. The bytes half of the sentence was separately corrected
by gilfoyle on ranger-base-32bdv: 5.25G of the 5.3G sat in FOUR dirs, so
reclaiming dirs was never going to free the quota either.

**2. `cmd/coldexec/door_linux.go:582` (at `12a40610`) carries a stale
denominator.** It says "MEASURED over all five arms: 12 of 2179 charges,
0.55%". The authoritative table in the same commit says 2,453 charges and
0.49%, and so do ranger-base-4kars's own notes fragment (stranded with it) and ADR 0085 D5 —
`12/2453 = 0.489%`, `12/2179 = 0.551%`. Three places against one. Escaped
from ranger-base-4kars's close. It is a comment, no pin reads it, and the
file is not on main — whoever replays `12a40610` for ranger-base-v60dy can
take the number from the table beside it.

## For the next verifier

- **A stale-waiver finding is decided by the pair, not the red.** The red was
  already on main and already loud. What a mutant has to show is what the
  waiver was COSTING: delete the thing it waives, once with the row and once
  without, and the silent `ok` is the finding.
- **A pin that only reds under load cannot be verified focused, and the close
  may tell you so itself.** xpxo6's said the focused run passes 10 of 10
  either way. Believing the fix then means paying for the loaded run; the
  pre-fix value in the loaded arm is the single measurement that decides it,
  and it cost one slot and one arm.
- **Run the arm anyway when the arm is the condition.** The same run paid for
  four other verdicts for free: 32bdv's residual leak (exactly one dir, from a
  `-timeout` panic), 32bdv's one-per-tree claim during the run, xpxo6's arm-3
  counter column reproduced to within one `absent`, and orgvt's zero holes.
- **`go test -run` that answers `ok` with no test named means the wrong arm**
  (AGENTS.md). It cost me two runs here before I read the build tag:
  `watchhang_qa_test.go`, `seatbeltgitidentity_qa_test.go` and
  `seatbelttelemetry_qa_test.go` are all `//go:build posse_arm3`. Add `-v`
  and look for `no tests to run` rather than trusting the `ok`.
- **A red taken under a moved TMPDIR is not a red until it is re-run on the
  default root** (ranger-base-akk9f's class, and ORDERS says so). Both
  non-watchdog reds in my arm evaporated there — one PASS, one SKIP. The
  second one printed its own reason: its writable set ends in `/tmp`.
- **A number repeated in four places is worth summing in all four.** 4kars's
  census closes exactly in three of them and misses in the fourth, and the
  one that misses is the code comment — the place a reader of the mechanism
  is most likely to land.
- **`nohup ... &` inside a backgrounded Bash call reports exit 0 at once.**
  The harness notified completion while the arm had 55 minutes left to run;
  `kill -0 <the pid test-times.sh printed>` and
  `scripts/suite-lock.sh --status` are what actually answered it. Declare it
  with `POSSE_KEEP=` and wait on the pid, not on the notification.
- **Do not cite a path that lives on a branch.** This fragment first pointed
  at that bead's own fragment, by path, for its census table — and
  `make doc-check` red: "cites ... which is not a file in this tree — the
  evidence pointer resolves to nothing, which is how v0.5.1 was cut over an
  ADR citing a fragment stranded on a branch (ranger-base-nnnf1)". When the
  work you are verifying has not landed, name the BEAD, never the path. A
  verify bead over a stranded close is exactly the shape that trips it.
