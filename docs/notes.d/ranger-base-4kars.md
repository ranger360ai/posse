# A withdrawn notification truncates the injection (ranger-base-4kars)

*2026-11-11 · gwart · why `delayed` counted a delay the door never paid,
the one line that fixes the accounting, and the census that says whether the
delay itself is worth fixing.*

## The claim

`cmd/coldexec` holds the first `execve` of every inode created after it
started, for D. ranger-base-j0riv established what happens when the kernel
takes one of those notifications back: seccomp's wait is interruptible, a
deliverable signal makes the kernel REMOVE the notification and set the
syscall to `ERESTARTSYS`, and a caller whose handler carries `SA_RESTART`
then RESTARTS the execve, raising a fresh notification which the door
adjudicates. Nothing escapes the door; one execve spends two notifications.

That bead's question was **coverage**, and the answer was that a withdrawal
is not a coverage hole. This bead's question is **the delay**, and the answer
is that it was not paid.

`adjudicate` marked `d.seen[key] = true` before it slept, so the restart's
notification read the inode as WARM and was answered at once. The plant's
exec therefore completed in milliseconds, while the first handler went on
sleeping out the full D against a notification nobody was waiting for — and
the door, which recorded `vDelayed` on the way INTO that sleep, reported one
cold exec held for D.

MEASURED 2026-11-11 (this box, linux 7.2.15 x86_64, go1.27.3), D = 600ms, by
the fixture both pins now share with a nudge every 10ms: the plant's exec
goes through at roughly the first nudge, **~10-20ms against the 600ms the
report charged it**, 40/40 runs identical. Without the nudger the same
fixture pays the full 600ms.

A door that reports a delay it did not pay is the silent failure ADR 0085 D5
exists to prevent, one level up — not a test flaking on darwin because the
door missed it, but a door reporting coverage it did not deliver.

## The fix, which is j0riv's own rule applied one verdict further

j0riv's rule was: *a hole is an execve the door let through without knowing
what it was, and whether it was let through at all is decided at the ANSWER
and nowhere else.* `vDelayed` was the one verdict still recorded on the way
in. It now rides down to `answer()` like a hole does, and `answer()` decides
it:

| the notification | the verdict | what it means |
|---|---|---|
| the answer landed | `delayed` | held to the end of D and let through |
| the kernel took it back, and this handler was holding a delay | `truncated` | charged D, released at the signal; its restart read warm, so that first exec went through at full speed |
| the kernel took it back, holding nothing | `withdrawn` | one execve spent two notifications and nothing else |

So `delayed` counts delays that were PAID, the report names the truncations
in their own sentence, and the exit-3 rule (ADR 0085 D5: zero delayed is a
failed door) reads the paid count — a run whose every injection was cut
short is a failed door and not a clean suite, and it says which.

It is also one verdict per notification again for this family, which is what
the `door` struct claimed and did not have: a truncated charge used to record
`delayed` in `adjudicate` and `withdrawn` in `answer`.

## What it deliberately does NOT do

Make the restart pay. The inode is still marked before the sleep, so a
truncated plant's first exec really does go through at full speed — the door
now says so instead of claiming otherwise.

Charging the inode only once the answer has LANDED would make the restart
read cold and pay D again. That is a semantics change and not a fix: the
`seen` mark is also what stops two concurrent first execs of one inode from
both paying D, and the door's whole cost model is "D once per cold exec,
additive within 3-5%" (ranger-base-in8dr). Whether it is worth that is
decided by one number, which is why this bead was filed rather than patched.

## The census: how often does it happen to a plant that matters?

It is not zero, and it is **one call site**.

MEASURED 2026-11-11, this box (linux 7.2.15 x86_64, go1.27.3), D = 2.5s, all
five `make coldexec-check` arms under `-verbose`, taking each arm from its
cleanest run:

| arm | notifications | charges | paid | **truncated** |
|---|---|---|---|---|
| 1 `./internal/posse/...` | 129,555 | 696 | 695 | **1** |
| 2 `-tags posse_arm2` | 113,769 | 320 | 320 | 0 |
| 3 `-tags posse_arm3` | 83,172 | 598 | 598 | 0 |
| `./cmd/posse/...` | 8,403 | 231 | 231 | 0 |
| `./internal/treepins/...` | 29,743 | 608 | 597 | **11** |
| **total** | **364,642** | **2,453** | **2,441** | **12** |

**0.49% of charges go unheld.** Arms 1 and 2 are mildly understated: the
shared 14G /tmp tmpfs ran out mid-run and some tests died before planting
(ranger-base-w5z7u). cmd/posse and treepins above are the clean re-runs with
TMPDIR on /home, and they reproduce ranger-base-in8dr's own numbers for those
arms to within one notification and 1.3% of wall — 231 held and 29,743
notifications against its 231 and 29,744 — which is the evidence that these
readings are the arms and not the accident.

### The 11 are deterministic, and they are all the same exec

Every truncation names a planted `bd`:

- `…/stub/bd` under `TestShippedPIDsLetBeadsOwnHooksRun<persona>` — nine of
  them, architect through security
- `…/stub/bd` under `TestBroadHookDenyWallsTheCommitAndTheCheckout` and
  `TestNarrowedHookDenyStillRefusesInstall`
- `…/real/bd` under `TestRenderedShimRoutesAnUnprojectedWriteThroughPosse`
  (arm 1's, and the only racy one — 2 in one run, 1 in the next)

The treepins eleven came back **identical across two runs** with different
TMPDIRs and different pids: same eleven tests, 11/11 both times. So for that
family this is not a rare race at all — it happens every time.

**They are one `syscall.Exec`.** `internal/posse/bdwriteshim.go:95` takes the
ADR 0073 bd write lock and then BECOMES bd —
`syscall.Exec(real, argv, os.Environ())` — so bd inherits the lock's open
file description. That is a live Go runtime performing a direct `execve`,
which is precisely ranger-base-j0riv's interruptible shape: the runtime has
`SA_RESTART` handlers installed and sends itself preemption signals, so the
kernel can take the notification back and the execve restarts. A plant exec'd
through `os/exec` cannot be withdrawn at all — that child blocks every signal
across fork+exec — which is why this family and no other appears in 364,642
notifications. `bhcWorld` in internal/treepins/bdhookcommit_qa_test.go plants
the stub precisely so "the shim execs the stub and not the operator's bd",
which is the fixture saying what the census found.

### What that buys the decision

The bead's own framing was: if the count is zero, a line in the report is the
honest answer; if not, the key may have to move. It is not zero — so the line
in the report is **necessary and not sufficient**, and the remaining work is
real. It is also not urgent: no test in that family times the bd exec, so
nothing is flaking on it today. What would bite is a test that starts timing
one — it would pass here and flake on ci.yml's macos-latest leg, which is the
one thing ADR 0085 D5 exists to prevent.

So this bead ships the honesty and hands the coverage on: **ranger-base-91fu2**,
carrying this census, the call site, and a candidate mechanism that leaves
`d.seen` alone (an in-flight (pid, inode) deadline, where a restart is
identifiable because the kernel restarts the syscall on the same task) — along
with the one fact that mechanism rests on and must measure first, that
`seccomp_notif.pid` is the task id rather than the thread-group id.


## The pins

Both read one live run, through `withdrawalFixture`: a python3 caller with an
`SA_RESTART` handler (the Go runtime's shape), a forked nudger guarded by the
caller's own `/proc/<pid>/exe`, a nudged plant, and a second plant nothing
signals.

- **ARM 14** `TestAWithdrawnNotificationIsNotAHole` — j0riv's: the
  classification and the coverage. Unchanged in subject; it now accepts
  either verbose spelling of a withdrawal, because which one this run
  produces is ARM 16's business.
- **ARM 16** `TestAWithdrawnNotificationTruncatesTheInjection` — this bead's:
  the accounting. Two cold plants, one nudged and one not, and `delayed == 1`.
  It reds on the one line this bead moved — recording `vDelayed` in
  `adjudicate` before the sleep — which prints 2 delayed, no truncation line,
  and a 600ms charge against a plant that exec'd in 20ms (MEASURED here, both
  doors built from the same tree: the mutant reds ARM 16 and leaves ARM 14
  green, which is j0riv's design holding).
  *(ranger-base-91fu2 inverted this pin on the same day, as the section below
  said it would. ARM 16 is `TestARestartedExecvePaysTheRestOfItsCharge` now
  and asserts the opposite — `delayed == 2`, no truncation, and a wall-time
  FLOOR on the nudged exec, because after that bead there is one to read.
  What is left of this bead's subject is ARM 17,
  `TestAChargeNothingRestartsReadsTruncated`: a charge nothing restarts, and
  an execve that therefore never happens. See
  docs/notes.d/ranger-base-91fu2.md.)*

The second plant is why ARM 14 needed any edit at all: since a truncated
charge is not in the delayed count, a run whose ONLY plant is the nudged one
delays nothing and exits 3 — correctly — and `wantCode(t, 0)` would have
stopped saying anything about either pin's subject.

**No wall time is asserted, in either pin.** The accounting IS the statement:
a charge held to the end is in the count and one cut short is not, so
`delayed == 1` over a run with two cold plants says the nudged plant's exec
was not held, exactly. A pin that read the ratio of two wall times would be
the flake cmd/coldexec's test header warns about on a box that reaches a
loadavg of 50-100.

## ADR

ADR 0085 D5 gains one edge, in the list that exists so the door's gaps are
stated rather than left silent, and in the same shape ranger-base-in8dr's
orphan row took: the mechanism, the measurement, the one call site, and the
bead that holds the candidate fix. D5's closing sentence also now says what
the delayed count means — held to the end, not merely charged.

Nothing else in ADR 0085 moves. D5's claim that the door covers every first
`execve` of an inode created after it started is unchanged and still true: a
truncated charge is a gap in the DELAY, not in the coverage, because the
restart is adjudicated (ranger-base-j0riv).
