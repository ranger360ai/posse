//go:build darwin

package posse

import (
	"bytes"
	"context"
	"encoding/binary"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// sysSelfProcs on darwin is the self-check's process table without the fork —
// which is the whole point, for the reason sysLoad1 gives one file over
// (loadavg_darwin.go) and for a second one measured here.
//
// `/bin/ps` on darwin is SETUID ROOT (`-rwsr-xr-x root wheel`, darwin 25.4.0),
// and seatbelt refuses to exec a setuid binary from inside a sandbox whatever
// the profile says: a seatbelt seat's `go run ./cmd/checkorphans` answered
// `fork/exec /bin/ps: operation not permitted`, exit 2, leak status unknown —
// four closes across three personas in one day, each inventing its own
// substitute (ranger-base-yxmwx). MEASURED 2026-09-28 under the rendered persona profile,
// which is `(allow default)` plus file-write denies and does not mention exec
// at all: an explicit `(allow process-exec* (literal "/bin/ps"))` does NOT
// lift it, and the only profile line that does is `(with no-sandbox)`, i.e.
// running a setuid-root binary outside the wall the profile exists to be.
// So the route is the syscall, not the carve-out.
//
// It is the same table `ps` reads and one row wider on the same box (452 vs
// 451: ps does not list itself). AGE is better than etime — a real start
// timestamp instead of a value rounded to the second — and ARGV is whole,
// with no second read needed to get past a column width, because sysctl hands
// over the argv vector rather than a fixed-width rendering of it.
//
// CPU IS NOT AVAILABLE THIS WAY and that is why SysTopCPU still forks: the
// kernel leaves `p_pctcpu` at 0 in every row (MEASURED 2026-09-28, all 451
// rows readable both ways, including one at ps_pcpu=58.4), so a %CPU this
// route reported would be a fabricated zero. The self-check has no CPU term
// by design (ranger-base-6mhxw), so it loses nothing; the load guard's
// culprit line does, and keeps its `ps`. Rows from here carry CPU 0 and must
// never be fed to a predicate with a CPU floor.
//
// The context is taken for the seam's sake and not used: there is no fork to
// bound and no deadline a sysctl pair can outrun.
func sysSelfProcs(_ context.Context) ([]Proc, error) {
	procs, err := sysctlProcTable()
	if err != nil {
		return nil, err
	}
	fillArgsFromSysctl(procs, selfCheckSuspectPIDs(procs))
	return procs, nil
}

// sysctlProcTable reads kern.proc.all into the same Proc the `ps` parser
// fills, minus CPU (see above). Comm is cut at the FIRST NUL and not
// right-trimmed: the kernel's 17-byte p_comm can hold another process's
// leftovers past the terminator — "bird\x00oxy\x00sk" was on the box the day
// this was written — and a trailing-NUL trim would carry that garbage into
// the culprit line.
func sysctlProcTable() ([]Proc, error) {
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	procs := make([]Proc, 0, len(kps))
	for i := range kps {
		start := time.Unix(kps[i].Proc.P_starttime.Sec, int64(kps[i].Proc.P_starttime.Usec)*1000)
		procs = append(procs, Proc{
			PID:  int(kps[i].Proc.P_pid),
			PPID: int(kps[i].Eproc.Ppid),
			Age:  now.Sub(start),
			Comm: filepath.Base(sysctlComm(kps[i].Proc.P_comm[:])),
		})
	}
	return procs, nil
}

// sysctlComm is p_comm as a string: up to the first NUL, or the whole field
// when the kernel filled it to the brim.
func sysctlComm(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

// fillArgsFromSysctl is the darwin half of "only the suspects get their argv
// read" — the property that keeps a self-check on a healthy box to a single
// table read, here as in fillArgsForPIDs. It reports nothing for the same
// reason that one does not: an argv it could not get is an empty Args, which
// the predicate reads as "not shown to be ours" and skips.
//
// ids arrive as strings because that is what the `ps` route wants them as and
// what selfCheckSuspectPIDs is pinned to hand over; a row whose id does not
// parse cannot match a pid here either way.
func fillArgsFromSysctl(procs []Proc, ids []string) {
	if len(ids) == 0 {
		return
	}
	want := make(map[int]bool, len(ids))
	for _, id := range ids {
		if pid, err := strconv.Atoi(id); err == nil {
			want[pid] = true
		}
	}
	for i := range procs {
		if !want[procs[i].PID] {
			continue
		}
		if args, err := sysctlProcArgs(procs[i].PID); err == nil {
			procs[i].Args = args
		}
	}
}

// sysctlProcArgs is one process's UNTRUNCATED argv, space-joined the way `ps
// -o args=` renders it so gateShellForkPayload reads the same string either
// way.
//
// kern.procargs2 answers: argc as a native int32, the exec path, NUL padding
// to an alignment the caller does not get to know, then argc NUL-terminated
// argv strings (the environment follows, and is none of our business). Only
// the calling user's processes are readable — 165 of 452 rows came back
// refused on the box this was measured on, every one of them another uid's,
// and a gate-shell child is always ours.
func sysctlProcArgs(pid int) (string, error) {
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return "", err
	}
	if len(raw) < 4 {
		return "", Die("kern.procargs2 for pid %d returned %d bytes, not an argc", pid, len(raw))
	}
	argc := int(int32(binary.NativeEndian.Uint32(raw[:4])))
	rest := raw[4:]
	// the exec path, then however many NULs the kernel padded with.
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return "", Die("kern.procargs2 for pid %d has no exec path", pid)
	}
	rest = rest[i:]
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}
	// No capacity from argc: it is a kernel-supplied int32 this code has to
	// treat as a number it did not choose, and the loop is bounded by the
	// buffer anyway.
	var args []string
	for n := 0; n < argc && len(rest) > 0; n++ {
		j := bytes.IndexByte(rest, 0)
		if j < 0 {
			args = append(args, string(rest))
			break
		}
		args = append(args, string(rest[:j]))
		rest = rest[j+1:]
	}
	return strings.Join(args, " "), nil
}
