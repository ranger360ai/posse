# Liveness vs identity

## The concept

"Is that process alive, and is it the one I think it is?" is **two**
questions, and the classic pidfile answers both badly by conflating
them. A pidfile is a *snapshot* of a durable claim: the file survives
the process's death (crash, `kill -9`, pane closed), and the pid it
names gets recycled — so file-exists proves nothing about liveness, and
pid-answers-signal-0 proves nothing about identity. Every pidfile check
is a TOCTOU (`toctou.md`) against the process table.

The standard answer: **derive liveness from state the kernel ties to
the process's existence**, so that release *is* death and no snapshot
decays. `flock(2)` is the canonical form: the lock is "associated with
an open file description" and released "when all such file descriptors
have been closed" — process termination closes them, so the kernel
revokes the lock at death, atomically, with no staleness class at all
(man7). Same family: bound sockets, abstract-namespace sockets, session
leaders. **Identity remains a separate check** — the kernel says "a
process holds this", not "the process you recorded holds this"; if you
need identity (is this pid still *my* loop?), verify a property of the
process itself (argv, start time), never just the pid number.

Heartbeats are the distributed cousin, with the dual failure: a paused
process misses beats without being dead (false death → the two-holder
problem, `fencing-and-leases.md`), and a live heartbeat thread proves
the heart beats, not that the work progresses. Process pauses and their
unbounded length: Kleppmann, *DDIA* ch. 8.

## Standard rebuttals

- *"We clean up the pidfile on exit."* Crash paths don't run cleanup;
  the file's whole failure mode is the exits you didn't write.
- *"We check the pid is alive before trusting the file."* Pid recycling:
  alive, but somebody else. Liveness check without identity check.
- *"flock is Linux-arcana."* It is POSIX-era, portable (macOS included),
  and ~30 lines; the arcana is maintaining a staleness-detection layer a
  pidfile then needs.

## How posse applies it

ADR 0011 §1: the launcher serializes on `flock` of
`state/dispatch-launch.lock` rather than a pidfile — "flock's release
*is* process death, kernel-owned, no staleness class". The watch pidfile
survives only for the *identity* half of the husk check: the pid answers
signal 0 **and** its argv is still a dispatch loop. Two flock properties
posse pays for and so would you (ADR 0011 Appendix A): it is **advisory**
— only the paths that take it are serialized, so the perimeter, not the
mechanism, is where it fails — and it is **single-host, local-filesystem
only**, degrading to emulation or a no-op on network mounts, which is why
`RHQ_HOME/state/` must stay on a local FS.

## Sources (verified 2026-08-20)

- **[docs]** flock(2), Linux man-pages —
  https://man7.org/linux/man-pages/man2/flock.2.html
- **[book]** Kleppmann, *Designing Data-Intensive Applications*, ch. 8
  ("The Trouble with Distributed Systems": pauses, clocks, detection) —
  https://dataintensive.net/
- **[blog]** Kleppmann, "How to do distributed locking", 2016 (the
  pause→false-death argument) —
  https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
