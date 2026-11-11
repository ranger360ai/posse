//go:build linux

package main

// THE BOX IS A COVERAGE QUESTION, not a comfort one (ranger-base-w5z7u).
//
// WHAT HAPPENED. `make coldexec-check`'s five arms, run back to back from a
// seat with TMPDIR unset, exhausted this box's shared 14G /tmp tmpfs and the
// suite red with `disk quota exceeded` — on the TESTS and not on the door, so
// what the seat read was a hundred unrelated red test names and no statement
// about the class the door exists to measure. MEASURED 2026-11-11 (linux
// 7.2.15 x86_64, go1.27.3, D=2.5s), per arm, FAILs and the lines naming a
// space or quota error: arm 1 4/2, arm 2 2/7, arm 3 1/0, cmd/posse 112/122,
// internal/treepins 12/15. The cmd/posse arm held 34 cold first execs against
// the 231 ranger-base-in8dr measured for the same arm on the same day — its
// tests died before they could plant, so the door's own number went with
// them. That is ADR 0085 D5's case exactly: a run that is not covered and
// does not say so.
//
// WHY THE DOOR MAKES IT WORSE RATHER THAN CAUSING IT. The injection holds
// every plant's first execve for D, so a test under the door lives roughly
// as many times longer as the arm's doored wall is longer than its undoored
// one — 6.5x for cmd/posse, 12.0x for internal/treepins (ranger-base-in8dr)
// — and every one of those tests is holding a working-tree copy in its own
// temp dir for that whole time. The PEAK is what overflows. The undoored
// arms do not come near it.
//
// SO THE DOOR READS THE ROOM BEFORE IT SPENDS AN HOUR OF BOX. Not free space
// — the number that binds here is a quota, and ranger-base-akk9f established
// that `df` cannot see it: /tmp here is a 14G tmpfs mounted `usrquota`, the
// quota is per UID, every seat on this box runs as the same user, so it is
// ONE BOX-WIDE budget that all concurrent suites spend together and the
// device reports room throughout. akk9f left the usage/limit reading out as
// "a bigger question than a caveat" because there is no `quota` binary on
// this box. There does not need to be: `quotactl_fd(2)` answers Q_GETQUOTA
// for the CALLER's own uid unprivileged, and tmpfs implements it.
// MEASURED 2026-11-11 on this box, uid 1001, /tmp: 11130 MB hard limit,
// 8888 MB used, 2242 MB of the share left — while `df` reported 5026 MB free
// on the device. Two numbers, and the smaller one is the one a test hits.
//
// AND IT REFUSES RATHER THAN RUNNING UNCOVERED, which is the shape ADR 0085
// D5 already gives this door for an architecture its filter does not name and
// a kernel with no user notification. A box it cannot fit in is the same kind
// of edge: the arms would run, the tests would die on writes, and the report
// would be a count of held execs that measures the box rather than the class.
// Exit 2, the same refusal, before anything has run.
//
// WHAT IT CANNOT PROMISE, said rather than implied. The quota is box-wide and
// this reading is an instant: a sibling seat's suite can spend the share
// between this check and the arm's first write, so a run that starts can
// still die on EDQUOT. When it does, scripts/test-times.sh's EDQUOT block
// names it (ranger-base-akk9f). What the check buys is the common case — a
// box that is ALREADY too small — caught in milliseconds instead of at the
// end of an hour, with the binding number on the screen.

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// quotaCmd builds quotactl(2)'s first argument: the subcommand in the high
// bits, the quota type in the low byte. QCMD() in <sys/quota.h>.
func quotaCmd(cmd, typ int) uintptr { return uintptr((cmd << 8) | (typ & 0xff)) }

const (
	qGetQuota = 0x800007 // Q_GETQUOTA
	usrQuota  = 0        // USRQUOTA
)

// ifDqblk is <linux/quota.h>'s struct if_dqblk, the Q_GETQUOTA payload.
// Block fields are in 1024-byte blocks; dqb_curspace is in BYTES. The
// trailing u32 is padding the kernel does not name and Go would add anyway.
type ifDqblk struct {
	BHardLimit uint64
	BSoftLimit uint64
	CurSpace   uint64
	IHardLimit uint64
	ISoftLimit uint64
	CurInodes  uint64
	BTime      uint64
	ITime      uint64
	Valid      uint32
	_          uint32
}

// quotaUnmeasured is the share of a filesystem that has no quota on it, or
// one whose quota this process may not read. It is NOT zero: zero is "your
// share is spent", which is the opposite verdict.
const quotaUnmeasured = int64(-1)

// roomReading is what the door can learn about the directory every
// t.TempDir() and every `go build` work dir under it will land in.
type roomReading struct {
	Dir     string // the directory itself, as the suite will see it
	FreeMB  int64  // the DEVICE, which is what df reports
	QuotaMB int64  // the caller's share of it, or quotaUnmeasured
	QuotaBy string // how the share was read, or why it was not
}

// BindingMB is the number a write actually hits: the smaller of the two,
// because both bind and either can be the short one.
func (r roomReading) BindingMB() int64 {
	if r.QuotaMB != quotaUnmeasured && r.QuotaMB < r.FreeMB {
		return r.QuotaMB
	}
	return r.FreeMB
}

// Line is the one-line report. Printed on every run that asks for the check,
// red or green, because ADR 0085 D5's rule for this door is that the gap is a
// number and not a silence — and because a seat reading an EDQUOT red an hour
// later needs to know what the room was when the arm started.
func (r roomReading) Line() string {
	if r.QuotaMB == quotaUnmeasured {
		return fmt.Sprintf("coldexec: TEMP ROOM: %s — %d MB free on the device, no quota reading (%s)", r.Dir, r.FreeMB, r.QuotaBy)
	}
	return fmt.Sprintf("coldexec: TEMP ROOM: %s — %d MB left of your quota share, %d MB free on the device (%s)", r.Dir, r.QuotaMB, r.FreeMB, r.QuotaBy)
}

// tempDir is where the suite's temp files will land, read the way Go reads
// it: os.MkdirTemp("", …) and therefore t.TempDir() honour TMPDIR, and cmd/go
// hands the test binary GOTMPDIR as its own TMPDIR — which defaults to this
// same value, so one directory answers for the fixtures AND the build work
// dirs (ranger-base-krra). os.TempDir IS that rule: $TMPDIR, else /tmp.
func tempDir() string { return os.TempDir() }

// readRoom measures one directory. It never fails: a filesystem whose free
// space cannot be read reports 0 MB, which refuses — the door's bias
// everywhere else — and a filesystem with no quota reports the device alone.
func readRoom(dir string) roomReading {
	r := roomReading{Dir: dir, QuotaMB: quotaUnmeasured}

	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		r.QuotaBy = fmt.Sprintf("statfs: %v", err)
		return r
	}
	r.FreeMB = int64(st.Bavail) * int64(st.Bsize) / (1 << 20)

	mb, how := quotaShareMB(dir)
	r.QuotaMB, r.QuotaBy = mb, how
	return r
}

// quotaShareMB reads the CALLER's own block quota on the filesystem holding
// dir, in megabytes remaining.
//
// quotactl_fd(2) AND NOT quotactl(2), which is the whole reason this is
// readable at all: quotactl takes a block device path, and the filesystem
// that binds here is a tmpfs, which has none. quotactl_fd takes an fd of the
// mount instead (Linux 5.14) and the kernel allows an unprivileged caller to
// read its OWN uid's quota, so this needs no `quota` binary — there is none
// on this box — and no privilege.
//
// Every errno is a reading of "no quota here" and not a failure: ENOSYS is a
// kernel without the call, EINVAL and ENOTBLK a filesystem with no quota on
// it, ESRCH no quota type enabled, EPERM a quota this process may not read.
// The door refuses on a SHORT room, never on an unreadable one — a refusal on
// an unmeasured quota would ground the door on every box that does not run
// one, which is most of them, darwin included.
func quotaShareMB(dir string) (int64, string) {
	f, err := os.Open(dir)
	if err != nil {
		return quotaUnmeasured, fmt.Sprintf("open %s: %v", dir, err)
	}
	defer f.Close()

	var d ifDqblk
	uid := os.Getuid()
	if _, _, errno := syscall.Syscall6(unix.SYS_QUOTACTL_FD, f.Fd(),
		quotaCmd(qGetQuota, usrQuota), uintptr(uid),
		uintptr(unsafe.Pointer(&d)), 0, 0); errno != 0 {
		return quotaUnmeasured, fmt.Sprintf("quotactl_fd: %v", errno)
	}

	// A limit of zero is no limit, which is a filesystem with quotas turned
	// on and none set for this uid. The soft limit binds first where it is
	// lower, because crossing it starts a grace period and writes fail at
	// its end — so the honest "left" is against whichever is smaller.
	limit := d.BHardLimit
	if d.BSoftLimit > 0 && (limit == 0 || d.BSoftLimit < limit) {
		limit = d.BSoftLimit
	}
	if limit == 0 {
		return quotaUnmeasured, fmt.Sprintf("quotactl_fd: no block limit set for uid %d", uid)
	}

	left := int64(limit)*1024 - int64(d.CurSpace)
	if left < 0 {
		left = 0
	}
	return left / (1 << 20), fmt.Sprintf("quotactl_fd, uid %d, %d MB limit", uid, int64(limit)/1024)
}

// requireRoom is the refusal. floorMB <= 0 turns the check off entirely,
// which is what every caller but `make coldexec-check` gets.
func requireRoom(floorMB int64) {
	if floorMB <= 0 {
		return
	}
	r := readRoom(tempDir())
	fmt.Fprintln(os.Stderr, r.Line())
	if r.BindingMB() >= floorMB {
		return
	}
	fmt.Fprintf(os.Stderr,
		"coldexec: THE TEMP ROOM IS SHORT. Every t.TempDir() and every `go build` work\n"+
			"  dir of this arm lands in\n"+
			"      %s\n"+
			"  and the door multiplies what is live at once: a test under it lives as long\n"+
			"  as the arm's doored wall is longer than its undoored one (6.5x for\n"+
			"  cmd/posse, 12.0x for internal/treepins), each one holding a working-tree\n"+
			"  copy for that whole time. Started short, the arm does not report the class\n"+
			"  — its tests die on their writes, and what the log holds is a hundred\n"+
			"  unrelated red names and a held-exec count that measures the box\n"+
			"  (ranger-base-w5z7u).\n"+
			"  The two ways out, and the second one is not free:\n"+
			"    - wait, or free the room. The share is BOX-WIDE and it moves without you:\n"+
			"      MEASURED 2026-11-11, fourteen minutes apart and with this seat writing\n"+
			"      nothing to %s, it went from 2242 MB left to 7425 MB. Where it goes:\n"+
			"          du -sh %s/posse-testhome-* 2>/dev/null | sort -h | tail\n"+
			"      (428 of those at the first reading, 5.26 GB of it in three trees; one at\n"+
			"      the second.) Whether a tree is a live sibling's or residue is the thing a\n"+
			"      seat cannot tell, so clearing is the operator's call and not this door's.\n"+
			"    - run the arm with TMPDIR on a filesystem with room. MEASURED 2026-11-11\n"+
			"      (ranger-base-w5z7u, ranger-base-akk9f): that buys the room and COSTS A\n"+
			"      CLASS — pins whose premise is that their fixture lies inside a granted\n"+
			"      root red, because a bare /tmp is in every rendered seatbelt and a fixture\n"+
			"      outside it is not covered, and surfaces that abbreviate $HOME to ~ red\n"+
			"      when TMPDIR is under $HOME. A TMPDIR under /tmp costs neither and buys\n"+
			"      no room; one anywhere else buys the room and costs both. The pins that\n"+
			"      pass on that bare /tmp are ranger-base-u8xk4.\n",
		r.Dir, r.Dir, r.Dir)
	refuse("the temp room is %d MB and this arm needs %d (-min-temp-mb)", r.BindingMB(), floorMB)
}
