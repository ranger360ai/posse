package posse

// The two reads the box-wide loop census needs that only a `/proc` can give:
// which pids exist, and which instance a pid declares. boxloops.go is the
// filter over them.

import (
	"bytes"
	"os"
	"strconv"
	"strings"
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
	for _, kv := range bytes.Split(raw, []byte{0}) {
		if v, ok := strings.CutPrefix(string(kv), "RHQ_HOME="); ok {
			if v == "" {
				return "", false
			}
			return v, true
		}
	}
	return "", false
}
