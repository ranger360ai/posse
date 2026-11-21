package posse

// The two reads the box-wide loop census needs that only a `/proc` can give:
// which pids exist, and which instance a pid declares. boxloops.go is the
// filter over them.

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// boxPids is every pid with a directory under /proc.
//
// A DIRECTORY READ, not a `ps`. The census keys on `/proc/<pid>/exe`, which
// `ps` cannot report at all (loadguard.go's `sysProcTable` gives comm and
// argv), so there is no fork here and nothing for a sandbox to refuse —
// which is also why this runs from a caged seat: `--unshare-pid` is
// deliberately absent from the bwrap argv and /proc stays the host's procfs
// under `--ro-bind / /` (bwrap.go).
//
// An unreadable /proc is an ERROR and not an empty list. The whole point of
// this reader is that a silence used to be read as "nothing stale on this
// box", so the one thing it must never do is answer a failed read with an
// all-clear.
func boxPids() ([]int, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, Die("cannot read /proc to census the processes running this binary (%v), so this box's other dispatch loops are unknown", err)
	}
	var pids []int
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	if len(pids) == 0 {
		return nil, Die("/proc holds no process directories, so this box's dispatch loops cannot be censused")
	}
	return pids, nil
}

// boxPidOwned answers whether /proc/<pid> is there and belongs to this uid.
//
// THE DIRECTORY, NOT THE LINKS, and that distinction is the whole of what
// this function buys. A seat in a child user namespace is REFUSED a same-uid
// peer's `exe` and `environ` — `cap_ptrace_access_check` compares user
// namespaces and not just uids (the ranger-base-rfyth mechanism) — while
// `/proc/<pid>` itself keeps its owner readable. So a failed readlink can be
// told from a process that exited: the directory is still there and still
// ours. Without that, `procExe` answering "not ok" for EACCES and for ENOENT
// alike made every refused sibling vanish from the census, which rendered as
// an all-clear (ranger-base-rph5z).
//
// MEASURED 2026-11-21 by the security persona from a live seat on this box
// whose Bash runs in a child user namespace (`/proc/self/uid_map` =
// `1001 1001 1`), against
// the ranger dispatch loop (pid 1185185, uid 1001): `/proc` listed 460 pids,
// `readlink /proc/1185185/exe` and `cat /proc/1185185/environ` were both
// EACCES, and a child the seat had started itself was readable. The reading
// in boxloops.go's header was taken from a `cage shims` seat, which is NOT in
// a child userns — the one reader class this census most needs to work from
// is the class that first reading did not cover.
//
// A pid of ANOTHER uid stays excluded rather than counted: root's processes
// are refused to everyone and are not subjects of this rename.
func boxPidOwned(pid int) bool {
	st, err := os.Stat("/proc/" + strconv.Itoa(pid))
	if err != nil {
		return false
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(sys.Uid) == os.Getuid()
}

// boxCommOf is `/proc/<pid>/comm`: the kernel's name for the program a
// process is running, which stays readable when `exe` is refused.
//
// WHY IT IS NEEDED, and it is the correction of this file's own first fix
// (ranger-base-rph5z, second round). Counting every same-uid process whose
// exe is refused was too broad to be read: MEASURED 2026-11-21 from a `cage
// shims` seat on this box, FOUR such processes exist permanently and none of
// them could ever be a dispatch loop — `systemd` (the user manager, pid
// 1140), `(sd-pam)`, `sshd-session` and `gpg-agent`, each EACCES (errno 13)
// on `exe` while `/proc/<pid>` is owned by uid 1001, because each is
// non-dumpable. So the caveat fired on every reading on this box forever,
// which is the thing the code comment next door warns against in so many
// words: a caveat nobody can act on is a caveat nobody reads.
//
// `comm` separates them at no cost: it is world-readable with no ptrace
// check, so it answers where `exe` does not.
//
// IT NARROWS AN UNKNOWN AND NEVER CONFIRMS A LOOP, which is the whole of why
// this is not the argv matching AGENTS.md forbids. That rule is about
// choosing a TARGET by a pattern every seat's process shares; this value is
// used for one decision only — whether a process we were refused is worth
// saying we could not read — and a loop is still confirmed by nothing but its
// own instance's record. A hostile `prctl(PR_SET_NAME)` can therefore inflate
// the count and make a reading say it is partial when it is not. That is a
// denial of clarity and not a false all-clear, which is the direction this
// whole file is written to fall in.
//
// ok=false when comm cannot be read either, and the caller COUNTS such a pid:
// knowing nothing about a process we were refused is exactly the case the
// caveat is for.
func boxCommOf(pid int) (string, bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(b), "\n"), true
}

// boxHomeOf is the RHQ_HOME a pid declares, from its own environ.
//
// THE PROCESS'S OWN STATEMENT of which instance it belongs to, which is what
// makes the per-home record usable as a filter without any notion of a homes
// root: the pid names its home, and that home's record either names the pid
// back or does not. ok=false when the environ cannot be read — another uid,
// a /proc that is not there, a process that exited between the listing and
// this read — and the caller NAMES such a candidate rather than dropping it.
//
// environ is NUL-separated (proc(5)). The value is taken up to the first
// NUL and not trimmed otherwise: a home path is whatever the launcher
// exported, and a reader that normalised it could disagree with the record
// path built from it.
//
// ONE VARIABLE LEAVES THIS FUNCTION, AND IT IS A PATH. The file this reads is
// another process's whole environment, which on this box carries credentials
// — so the return type is deliberately one string and not a map, the raw
// bytes are never logged, never rendered and never put on a BoxProc, and
// nothing above this line can reach them. A future reader that needs a second
// variable from here should ask whether it needs the VALUE or only the fact,
// and should not widen this return: secrets never appear in logs or reports
// (crew standing orders), and a census line is a thing an operator pastes on
// a bead.
func boxHomeOf(pid int) (string, bool) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return "", false
	}
	key := []byte("RHQ_HOME=")
	for _, kv := range bytes.Split(raw, []byte{0}) {
		if !bytes.HasPrefix(kv, key) {
			continue
		}
		// Converted HERE and nowhere earlier: a `string(kv)` in the loop
		// condition copied every variable of the peer's environment,
		// credentials included, into Go strings for the sake of one prefix
		// test that bytes can answer (the security review's third item).
		v := string(kv[len(key):])
		if !boxHomeIsWellFormed(v) {
			return "", false
		}
		return v, true
	}
	return "", false
}
