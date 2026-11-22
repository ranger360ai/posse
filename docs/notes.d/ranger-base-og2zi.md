# Suite slots: a bounded queue, and a holder named by the kernel (ranger-base-og2zi)

2026-11-22. `scripts/suite-lock.sh` flocks `suite-slot.*.lock` under
`~/.cache/posse`, which every wall and shim grants. Any seat could flock both
slots and stay alive, and every `make test` on the box then queued forever.
The waiting line and `--status` named the holder from the stamp the holder
wrote itself. This is the same class as the upgrade lock (ranger-base-u66yb),
and the fix has the same shape, with two differences that the script's design
forces.

## What changed

- **The wait is bounded.** `POSSE_SUITE_LOCK_WAIT` defaults to 3600s, the
  upgrade lock's number and coldexec's `-timeout 60m`. The number is ASSUMED,
  not measured. When the bound expires the run goes ahead **unserialized** and
  prints the holders. It does not refuse, because the header's rule is that
  the lock never makes the suite unrunnable: an expired wait becomes the
  `POSSE_SUITE_LOCK=0` run. The bound gives up the queue and leaves the slot
  with its holder.
- **The holder is named from `/proc/<pid>/fdinfo`, not from `/proc/locks`.**
  The upgrade lock reads `/proc/locks`, but here that file names the pid that
  TOOK the flock, which is always the python3 or flock(1) child that exits
  straight after. MEASURED 2026-11-22, linux 7.2, tmpfs: a slot held by shell
  3011565 read `FLOCK ADVISORY WRITE 3011566`, the dead child. fdinfo puts a
  `lock:` line on every fd that shares the holding description, and on no
  other open of the same file (a second `exec 8>>` in the holder had none). So
  it names the shell and its inheritors and never a waiter. Each match is
  confirmed by stat of `/proc/<pid>/fd/<n>`, comparing dev and inode. The scan
  took 83ms over 443 processes.
- **The stamp is always labelled** "the slot's own stamp, which any seat can
  write". On darwin, with no python3, or with the holder in another pid
  namespace or under another uid, the stamp is the only answer, and the label
  is how the reader knows that.

## Pins

The self-test has a new arm 18 (`squat: …`). A real holder rewrites its stamp
to name pid 1 and `/forged/stamp`. The waiter (`POSSE_SUITE_LOCK_WAIT=1`) must
first queue, then run on slot `none` and say so. The squatter must still hold
the slot, and both the waiting line and `--status` must name the squatter's
real pid. Two mutants, each run against a copy of the script, made the arm
fail:

- M1: the kernel reader never answers. The arm read "the kernel did not name
  the squatter".
- M2: the bound never fires. The arm read "the waiter never got through the
  acquire".

The Go pins register the arm by label (`suitelock_qa_test.go`) and add
`POSSE_SUITE_LOCK_WAIT=1` to the hostile ambient environment that the rig must
neutralize (`suitelockrigenv_qa_test.go`).

## Not done

- A squat of both slots still costs each queued arm up to the bound, and a
  `make test` queues three arms one after another. That is up to three hours
  against a deliberate squatter, against forever before. Resetting the clock
  when the kernel's holder set changes would let a real queue wait longer, but
  a squatter can rotate its holders, so this change does not do it.
