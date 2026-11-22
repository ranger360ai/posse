# The ubuntu "pdeathsig 0" was read on the wrong thread, so it never established that the runner clears it (ranger-base-z56s9)

2026-11-21, gilfoyle.

## What the instrument read, and why it could not say it

ranger-base-8m9d0's seat self-report put a `pdeathsig` row in the death arms
of `TestLiveWallUnixScopesSignalsToTheSeat`, and the first two ubuntu-26.04
arm-1 logs carrying it (runs 44934045502 and 44940074456) read `pdeathsig 0`
in BOTH arms, with the control red identically. That was ruled reading 1 of
the decision table: "the runner clears it before the seat runs".

It was not a reading of the seat. `pdeath_signal` is per-task. bwrap sets it
on the one thread that then calls `execvp`, which becomes the seat's
thread-group leader, and `copy_process` zeroes it on every thread the Go
runtime clones after that. The row was read from a test function, which runs
on whatever OS thread its goroutine landed on. From any non-leader thread it
says 0 while the leader still holds 9. This is the same per-task fact
ranger-base-vxz74 found in the wall step, one layer over, in the instrument
that was supposed to tell the two apart.

MEASURED 2026-11-21 on D1 (Linux 7.2.15-1-omarchy, go1.27.3, bubblewrap
0.13.1, no AppArmor): a Go program run under
`bwrap --die-with-parent --ro-bind / / --dev-bind /dev /dev --unshare-user`
read 9 at init on its leader, 9 from a goroutine that landed on the leader,
and 0 from each of three goroutines locked to other threads. On this box the
test goroutine happens to land on the leader, which is why the row read 9
here every time. A loaded 4-vCPU runner has no such guarantee.

The seat really did outlive its parent on ubuntu, in both arms. The row just
cannot say why.

## What the next red log says instead

- `pdeathsig`: the LEADER's value. It is read during package init, which
  `runtime.main` runs with the main goroutine locked to the main thread, and
  it certifies itself: it says `leader, read at init` only when
  gettid()==getpid(), and `NOT THE LEADER` with both ids otherwise.
- `pdeathsig-this-thread`: the old read, with its tid beside it, so a log
  can show the two disagreeing.
- `AT_SECURE`: from `/proc/self/auxv`, type 23. `begin_new_exec` zeroes
  `pdeath_signal` on a secure exec. That is the one exec-time clear, and on
  ubuntu only the AppArmor transition could make this exec secure.
- On a red, the parent's `outer bwrap` line says whether the SIGKILLed sh
  took the outer bwrap at all. If it did not, the break is bwrap's own
  `--die-with-parent`, one layer up, and the seat's rows are not where the
  fault is. The red also gives the seat's ppid after the 10s.

Graded by mutation on D1: with `--die-with-parent` removed from the argv,
both arms red, the leader row reads 0, and the outer bwrap line reads STILL
ALIVE. Restored, the pin is green three runs out of three, and
`go run ./cmd/checkorphans` is clean.

## The AppArmor premise, read from source

These are source readings, not measurements on the runner.

- Mainline `security/apparmor/domain.c` (`profile_transition`) sets
  `secure_exec` only when the matched x-rule lacks `AA_X_UNSAFE`. A
  lowercase `px`/`pix` carries that bit. The ix-fallback path jumps past the
  check altogether.
- Ubuntu resolute's `bwrap-userns-restrict` (from
  `git.launchpad.net/ubuntu/+source/apparmor`, branch
  `applied/ubuntu/resolute`) transitions bwrap's children with
  `allow pix /** -> &bwrap//&unpriv_bwrap`, lowercase. Its own comment says
  this is deliberate: the environment is not sanitised across that boundary,
  because flatpak depends on glibc's secure-mode variables surviving it.
- bubblewrap 0.11.1 (resolute's) and 0.13.1 (D1's) both call
  `PR_SET_PDEATHSIG` as the last step before `execvp`, after every
  credential change. Between the prctl and the seat there is only the
  execve. ~~The execve could clear it only through a secure exec or a
  commit_creds euid/fsuid/caps change, and neither appears in that path.~~
  **Corrected by ranger-base-evp5k / ranger-base-6ec6n:** there is a third
  clear, and it is on that path. AppArmor's own exec hook,
  `apparmor_bprm_committing_creds` (mainline `security/apparmor/lsm.c`),
  sets `current->pdeath_signal = 0` on every exec whose new label differs
  from the old one and is not unconfined — secure exec or not. The
  `pix /** -> &bwrap//&unpriv_bwrap` rule is exactly such a change.

~~So by source, nothing on that runner should clear the leader's pdeathsig.~~
So by source the AppArmor transition DOES clear it, and the first ubuntu log
carrying these rows agreed: leader `pdeathsig 0`, `AT_SECURE 0`, outer bwrap
gone, in both arms (ranger-base-evp5k). ADR ranger-base-sv89t's Landlock
premise still stands — the control arm was red with no Landlock on the line;
what fails is `--die-with-parent` itself under AppArmor. The fix is
ranger-base-6ec6n's (docs/notes.d/ranger-base-6ec6n.md).

## Still open

- ranger-base-z56s9's question moves to the follow-up bead filed at its
  close: why the seat outlived its parent on ubuntu-26.04, read from the
  first red log that carries these rows.
