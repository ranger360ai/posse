# ranger-base-dgbys — two rules in cmd/coldexec that nothing pinned

Both findings came off ranger-base-aifu2's verify of ranger-base-j0riv, and
both reduce to the same shape: a rule the tree states — in a close, in a code
comment, in ADR 0085 — with no test that reds when it stops being true. Neither
is a code defect this bead changed. What landed is two pins and the prose
pointers to them.

Filed against cf6506e4; **the tree had moved** by the time this ran. HEAD here
is 436f467c, with ranger-base-4kars, ranger-base-91fu2 and ranger-base-w5z7u
landed in between, so both findings were re-measured from scratch rather than
taken from the bead. Environment for everything below: this box, linux 7.2.15
x86_64, go1.27.3, D = 600ms (`pinDelay`), loadavg 2-6.

## Finding 1 — the HOLE half of ranger-base-j0riv's rule

The rule landed in two parts. (a) a withdrawal is not a hole. (b) a hole is an
execve the door let through without knowing what it was, so **whether it was
let through at all is decided at the ANSWER and nowhere else** — `adjudicate`
records no hole, every hole rides down to `answer()`, and `answer()` records it
only once `ID_VALID` has said the notification was live.

MEASURED at 436f467c by mutation, each mutant reverted and the file's md5 proved
identical after (`223e285a140f3410f505f2cec293f028`):

| mutant | what it does | result |
|---|---|---|
| `holes` | `handle` records its own pending HOLE before the answer | **green**, `-count=1` and `-count=5` (75 arm-executions) |
| `all` | the same for holes *and* delays | reds ARM 14, ARM 16, ARM 17 |

So the delay half of (b) is pinned — by the arms' `wantDelayed` arithmetic,
which ranger-base-4kars added — and the hole half was pinned by nothing, three
beads after it landed. That is live rather than tidy because j0riv's own close
says (b) is what keeps its symptom away: an intermittent `wantNoHoles` red in
arm 1, which is `make test`'s first test line and nobody's optional door.

### The number the bead asked for, and it changes the pin

The bead asked the runner to measure how many repetitions a live fixture needs,
rather than take 4% from a quiet box. MEASURED: **zero of 900**. Nine nudge
intervals (tight loop, 50µs, 100µs, 200µs, 500µs, 1ms, 2ms, 5ms, 10ms), 100
EINTR callers each, 207 of them withdrawn, and not one produced a hole.

The mechanism says why. The hole needs the caller to be GONE while the door is
inside `readPath`/`stat` **for that same notification**, and the only thing that
gets the caller there is the withdrawal — so the window is the microseconds
between the door taking the notification off the listener and finishing its
/proc reads, against a python3 caller that takes milliseconds to die after
EINTR. Nudge faster and the signal lands before the door's `RECV` instead, which
removes the notification entirely and there is no trial at all. The 11-in-250
the finding read was taken under `go test ./...`, which is where j0riv's
original red lived and where this file's header says a wall-clock fixture is a
flake.

So a fixture pin would have cost seconds a run to assert nothing here, and red
somewhere else. **ARM 19 `TestAHoleIsNotRecordedWhenTheKernelTookTheNotificationBack`
reads the rule off `handle()` instead** — the only pin in that file that is not
a supervised run, and the comment says so and why. Two cases, both deterministic
and both costing 0.00s: `unreadable` (a reaped pid, which is the hole this tree
actually produces — ranger-base-orgvt measured 134 in one arm) and `foreign` (a
syscall number the switch does not name, which reaches the same rule with no
/proc in the way). The listener is a closed fd: `answer()` branches on `ID_VALID`
returning an error and on nothing else, deliberately, so that is the branch the
pin asserts. The positive control is that `adjudicate` really does come back
with the hole and really does leave it unrecorded, so what the assertion reads
is a hole DROPPED and not a hole never made.

## Finding 2 — what a truncated charge leaves behind

`adjudicate` marks `d.seen[key]` before it sleeps and nothing unmarks it, so a
charge the kernel cut short with nothing to restart it leaves the plant marked
warm: the next caller's execve of that plant — **the first exec of that inode
that occurs at all** — reads `warm` and goes through at full speed.

MEASURED at 436f467c, deterministic:

```
coldexec: delayed     pid 689497 /proc/689497/cwd/./plant     <- charged, never ran
InterruptedError: [Errno 4] Interrupted system call
CALLER EXIT 1
coldexec: warm        pid 689501 /proc/689501/cwd/./plant     <- the real first exec
ELAPSED firstreal 2ms
coldexec: truncated   pid 689497 /proc/689497/cwd/./plant
coldexec: 12 execve notifications, 1 cold first execs delayed 600ms each
```

**This is a stated edge now, not a silent one.** The bead was filed against a
tree where the door reported that charge as `delayed`; ranger-base-4kars gave it
its own verdict and took it out of the delayed count, and ADR 0085 D5, doc.go's
"WHAT IT DOES NOT COVER" and the note above `recordWithdrawal` all now say in as
many words that the plant is marked warm and a later exec is not held. What none
of them had was a witness.

ARM 20 `TestATruncatedChargeLeavesItsPlantWarm` is that witness, on
ARM 17's fixture plus one ordinary exec of the same plant. It asserts the
verdict and the run's `delayed` count rather than a wall time — the door holds
an execve if and only if it answers `delayed`, so the verdict says "not held"
exactly, with none of the load sensitivity a ceiling carries.

### Why the edge was pinned and not closed

Not from preference. Closing it means unmarking the inode when a charge is cut
short, and `d.seen` is also what stops two concurrent first execs by DIFFERENT
tasks from both paying D — the door's cost model is D once per cold exec
(ranger-base-in8dr), and ADR 0085 D5 decided that trade the other way in as many
words ("the `seen` mark stays where it was"). Re-opening it is the architecture
lane's, and the reachable half is already gone: ranger-base-91fu2 made the
RESTART case pay, which is every truncation this tree has produced — the only
callers here that can be withdrawn at all are Go runtimes, which carry
`SA_RESTART` — and measured the truncation count over the arm that had them go
11 to 0. A fix would also be larger than it looks, which the mutation below
shows.

MEASURED, the unmark mutant, written two ways:

- unmark whenever the charge was not handed over → reds ARM 1 and ARM 2 and
  never ARM 20. `handedOver` is false for every ORDINARY first exec too, so that
  spelling makes every plant cold forever. A fix has to distinguish the
  truncation, and `recordWithdrawal` is where that is known and it does not
  carry the inode.
- unmark only when `ID_VALID` has already gone, checked at the deadline → ARM 20
  reds, every other arm green. That is the narrowest honest spelling of the fix
  and it is what ARM 20 holds the door to.

ARM 20's fixture waits `D + 400ms` before the second exec, derived from
`pinDelay`. That is the door's own hold and not a guess at one: the handler
holding the truncated charge is asleep until its deadline, and until it wakes
there is nothing a fix could have done about the mark yet. It is not an
assertion — a sleep that fell short would cost the pin its grip on such a fix
and could never red a door that has none.

## The fixture family's own flake rate, MEASURED

Both of the new pins had to be shown not to add a red to `make test` arm 1,
which is what `go test ./...` builds and where cmd/coldexec lives. That turned
into a census, and the census says more about the two beads already open on this
family than about either new pin. 150 executions of each of the four pins that
share `withdrawalFixture`, on a QUIET box (loadavg 3-5), 436f467c:

| pin | ending | failures | message |
|---|---|---|---|
| ARM 14 `…NotificationIsNotAHole` | SA_RESTART | **0** / 150 | — |
| ARM 16 `…PaysTheRestOfItsCharge` | SA_RESTART | **10** / 150 | 8 × "1 delayed, want 2"; 2 × the plant killed after its exec landed |
| ARM 17 `…RestartsReadsTruncated` | EINTR | **1** / 150 | "no truncated verdict names the plant" |
| ARM 20 `…LeavesItsPlantWarm` | EINTR | **1** / 150 | the same message, the same cause |

**ARM 16 at 6.7% on a quiet box is not "load-only"**, which is how
ranger-base-k9xsd reads today, and its root cause is a residual door_linux.go
already states: a restart that arrives after the charge's deadline finds no
in-flight entry and is answered `warm`, so the run spends `truncated` + `warm`
on an exec that DID take D and `delayed` reads 1. The run that caught it read
`ELAPSED nudged 625ms`, so that pin's actual subject held and only its
accounting assertion broke. Its second mechanism — 2 of the 10 — is the nudger
killing the plant in the microseconds after its execve lands and before /bin/sh
reaches its `trap`: `CALLER EXIT 138`, which is SIGUSR1 at its default
disposition, so the signal reached the NEW program.

That second mechanism is **ranger-base-ghhgb**, filed from here, because it is
not ARM 16's: it reds ARM 14 too, and it took ARM 14 in this bead's own first
`make test` — a full arm 1, at 0f3100a4, with 2 cold execs delayed and the
plant delayed 614ms, so the door did everything that pin asserts and what died
was the plant. The retry shape ARM 20 uses does not reach it: the trial there
really ran, and what the signal broke is the pin's subject rather than its
positive control. The deadline-edge half stays on ranger-base-k9xsd with its
root cause. Neither is this bead's and no assertion of either pin was touched
here.

**The suite**: `make test` red once on ranger-base-ghhgb (ARM 14), then ran
GREEN whole on the re-run at 0f3100a4 — all three arms, `silent-revert audit:
2531 commits, 0 untriaged`, exit 0. `make tree-check` green, 64.3s. `go run
./cmd/checkorphans` clean.

**ARM 20 inherits ranger-base-9n4y9 and nothing else**: identical rate and
identical message to ARM 17, which is that bead's own subject — the nudge
reaching the caller as a signal that interrupts the execve before the door is
holding its notification at all, so the run reaches none of the mechanism. So
ARM 20 **retries its positive control**, up to 4 runs, which is the fix
ranger-base-9n4y9's own description asks for ("the pin treats an EINTR exit as
'no truncation happened, retry the trial' rather than as a failure"). A trial
that cannot be run is not a failing trial; a door that has stopped truncating
fails all four, so the `Fatalf` still fires for the thing the pin exists to
catch. At the measured rate four runs is 0.0067^4. The retry is in ARM 20 and
not in `withdrawalFixture`, because lifting it to the other three is
ranger-base-9n4y9's to do — the shape is there to copy.

## Also read, and not filed

ranger-base-aifu2 recorded two things as comment prose rather than defects:
door_linux.go:78's "one verdict per NOTIFICATION" claim, and ranger-base-8vk23's
missing duplicate-close record. The first is no longer the claim that file
makes — the door struct's comment names the exception and points at
ranger-base-9uvsx (still open, P3), which is where the decision about closing it
lives. The second is not this lane's.
