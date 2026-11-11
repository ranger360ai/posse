// Command coldexec is the cold-first-exec door's supervisor (ADR 0085 D2).
// It wraps a command line and makes the FIRST execve of every file created
// after it started slow, for every process under it, at any depth:
//
//	coldexec [-delay D] [-min-temp-mb N] -- <argv…>
//
// D defaults to POSSE_COLD_EXEC_DELAY, else 2.5s (ADR 0085 D4). The one
// caller is `make coldexec-check`, which wraps each of its five arms in one
// of these: internal/posse's three tagged arms, cmd/posse and
// internal/treepins. Those are the three packages in the tree that plant an
// executable at all (ranger-base-in8dr), and the last two needed nothing of
// their own to join — the door is a process around `go test`, so an arm is a
// Makefile line (ADR 0085 D6).
//
// WHAT THE CLASS IS. A test plants a brand-new executable and execs it under
// a deadline of a few hundred ms. macOS assesses a freshly written
// executable on its FIRST exec and charges two orders of magnitude more for
// it than for the exec that follows — MEASURED on darwin 25.6.0 at a mean of
// 156-714ms and a worst of 2.17s against 6.3-10.9ms warm
// (docs/notes.d/ranger-base-aru4c.md). So a small deadline over a cold exec
// gives its verdict about process startup rather than about the code. Linux
// charges nothing — MEASURED 2026-11-10 on this host (7.2.15, x86_64, n=40):
// first exec mean 1.07ms, worst 1.29ms. The crew writes tests here (ADR
// 0083) and darwin still runs the suite (ci.yml's macos-latest leg and the
// laptop kept as the rollback home), so a new unwarmed fixture passes where
// it is written and flakes only where nobody is watching. This puts the
// darwin cost back on this box, deterministically, with the fatal the test
// already prints.
//
// WHY A PROCESS AND NOT A HOOK IN THE TREE. The door this replaces rewrote
// the plant's `#!` line to point at a sleeping shim. That is visible to
// every test whose subject is the planted file's own bytes, and ADR 0085's
// first full run (ranger-base-kiwvn) found 26 such tests — 23 of them
// certifying a git hook against posse's own render. It also could not reach
// a plant with no `#!` at all, which was 49% of them. Moving the cost into
// the execve leaves the bytes alone: `$0`, line numbers, the shebang, a
// hook's identity, a shim's header and a copied binary all reach the disk
// and the reader exactly as the test wrote them, and a copied binary is
// covered like anything else.
//
// THE MECHANISM, end to end:
//
//   - the parent makes itself a child subreaper, socketpairs, then re-execs
//     itself as `coldexec -child -- <argv…>` with that socket on fd 3. The
//     door starts at that moment: "a plant" means an inode whose ctime is
//     later than this. The subreaper flag is what keeps a descendant whose
//     own parent exited inside the supervisor's ptrace reach instead of
//     reparented past it — see door_linux.go's readPath — and it makes the
//     supervisor the process that has to wait on those orphans, which
//     reapOrphans does.
//   - the child locks its OS thread, sets PR_SET_NO_NEW_PRIVS, and installs
//     a seccomp filter with SECCOMP_FILTER_FLAG_NEW_LISTENER that answers
//     SECCOMP_RET_USER_NOTIF for execve and execveat and ALLOW for
//     everything else. It sends the listener fd back over fd 3 with
//     SCM_RIGHTS, closes it, and unix.Exec's the argv from that same locked
//     thread — no TSYNC, because exec keeps only the calling thread, and
//     the filter is inherited by every descendant from there.
//   - the parent reads notifications off the listener, one goroutine each.
//     For each it reads the path argument out of /proc/<pid>/mem, resolves a
//     relative one against /proc/<pid>/cwd (execveat: against
//     /proc/<pid>/fd/<dirfd>, or the fd itself under AT_EMPTY_PATH), and
//     stats it. If the ctime is after the door started and this
//     (dev, ino, ctime) has not been seen, it marks it and sleeps D. Then
//     SECCOMP_IOCTL_NOTIF_ID_VALID, then SEND with
//     SECCOMP_USER_NOTIF_FLAG_CONTINUE, so the kernel runs the original
//     execve. A file rewritten in place gets a new ctime and is cold again,
//     which is what the old shim's per-write nonce did.
//   - the delay is CHARGED AGAINST THE EXECVE and not against the
//     notification, and it is counted at the answer and not at the sleep.
//     seccomp's wait is interruptible: a deliverable signal makes the
//     kernel take the notification back, which releases the caller at that
//     signal instead of at D, and a caller whose handler carries SA_RESTART
//     then restarts the execve and raises a fresh notification. The charge
//     is an entry keyed by the task and the inode, carrying the absolute
//     deadline, so that restart sleeps the REMAINDER and the execve
//     completes no earlier than D from where it entered the kernel, however
//     many notifications it spends getting there (ranger-base-91fu2). The
//     cut-short notification reads `withdrawn`, the restart reads `delayed`
//     once its answer lands, and one that was holding nothing reads
//     `withdrawn` too (ranger-base-j0riv). `truncated` is what is left: a
//     charge NOTHING restarted, which is a caller whose handler lacks
//     SA_RESTART (EINTR) or has none at all, and in both of those the
//     execve does not happen. None of the three is a coverage hole: the
//     kernel never completes an execve whose notification it took back.
//
// `sh <path>` execs /bin/sh, an old inode, so it is never delayed — the
// adjudication ADR 0085 D1 wanted out of execution rather than out of
// reading. The pid is the forked child's own, as on darwin, where the
// assessment also happens inside execve, so a pid file, a kill or a `$$`
// names the process it would have named with no door in the way.
//
// WHAT IT DOES NOT COVER, each stated rather than left silent (ADR 0085 D5):
// files older than the door, which is the checkout and the toolchain; a
// process that installs its own seccomp listener underneath it, which is
// EBUSY because the kernel allows one listener per filter chain, and which
// therefore fails loudly and never silently; a 32-bit process under a
// 64-bit kernel, whose arch the filter does not name and whose execs are
// ALLOWed untouched; and a kernel or an architecture it does not know, where
// it refuses to start rather than run a suite uncovered. And one edge that
// is about the delay rather than the exec: a charge the kernel cut short
// that NOTHING restarted. The restarting case is held now — the restart
// inherits the remainder (ranger-base-91fu2) — and what is left is a caller
// that took EINTR or died, so no exec of that plant happened at all; what
// the door loses is the charge, and the plant is marked warm from there on,
// so a LATER exec of it is not held. See the note above recordWithdrawal in
// door_linux.go, and ADR 0085 D5.
//
// And one thing it CHANGES rather than fails to cover, which is the price
// of covering orphaned descendants at all: under the door an orphaned
// descendant's ppid is the supervisor's and not 1. A test that plants a
// real orphan and reads its parentage back — `Orphaned()` in internal/posse
// is `PPID == 1` — would see the door and not itself. Nothing in the arms
// does that today: orphancontrol_test.go is the one place that asserts
// ppid 1 on a planted orphan, and it runs only inside
// scripts/verify-orphan-report.sh's throwaway container, skipped here; and
// `go run ./cmd/checkorphans` runs after the door has exited, so its
// ppid-1 reading is untouched. The alternative was leaving those execs
// un-adjudicated and silent, which is what ADR 0085 D1 is written against.
// What the door does NOT change is liveness: reapOrphans waits on every
// orphan it adopts, so `kill -0` answers ESRCH once it is gone, exactly as
// it would have under init.
//
// AND IT READS THE ROOM FIRST, when -min-temp-mb asks it to. The arms spend
// the better part of an hour and every test under them writes into $TMPDIR;
// a box with no room left for that does not produce a verdict about the
// class, it produces a hundred unrelated red test names and a held-exec
// count that measures the box (ranger-base-w5z7u, MEASURED 2026-11-11: the
// cmd/posse arm held 34 cold first execs against its usual 231, because its
// tests died before they could plant). So below the floor it refuses with
// exit 2, like any other edge it cannot cover, before anything has run.
//
// The number it reads is the QUOTA share where the filesystem has one and
// free space otherwise, whichever is smaller — on this box /tmp is a tmpfs
// mounted usrquota, the quota is per uid and every seat runs as the same
// user, so the binding number is a box-wide budget `df` cannot see at all
// (ranger-base-akk9f). quotactl_fd(2) answers it for the caller's own uid
// unprivileged and tmpfs implements it, so this needs no `quota` binary and
// no privilege. See room_linux.go.
//
// At the end it prints the notifications it took and the cold execs it
// delayed — held to the end and let through, which is what that count means
// and not what it was charged. ZERO DELAYED IS A FAILED DOOR, not a clean
// suite, and it exits 3.
// Otherwise the exit code is the command's own, and a refusal to start is 2.
//
// THE EXIT HATCH is this directory and the wrapper in the Makefile line.
// It holds no state beyond an in-memory set of seen inodes; nothing in
// internal/posse refers to it.
package main
