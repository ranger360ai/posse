package posse

// The kernel's own start time for a pid — the fence runtimesession.go's note
// explains, on the platform that has one.

import (
	"bytes"
	"os"
	"strconv"
	"strings"
)

// procStatStartField is which field of `/proc/<pid>/stat` carries the
// process's start time: 22, counting from 1 (proc(5), `starttime`).
const procStatStartField = 22

// procStartOf is `/proc/<pid>/stat` field 22, verbatim, as claude writes it
// into a session row's `procStart` (MEASURED 2026-11-10 over six live claude
// processes: an exact string match in every one).
//
// THE COMM FIELD IS SKIPPED BY ITS LAST PAREN, not by splitting on spaces
// from the left. Field 2 is the executable name in parentheses and it may
// hold spaces and parentheses of its own, so a plain field split counts wrong
// for any process whose name has one — which is a thing an attacker or an
// unlucky binary chooses, not a thing posse does. proc(5) documents the last
// `)` as the way to find the end of it.
func procStartOf(pid int) (string, error) {
	if pid <= 0 {
		return "", Die("no process start time for pid %d", pid)
	}
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", Die("process start time for pid %d: %v", pid, err)
	}
	i := bytes.LastIndexByte(raw, ')')
	if i < 0 {
		return "", Die("process start time for pid %d: /proc/%d/stat has no comm field", pid, pid)
	}
	// Past the comm, the fields start at 3, so field 22 is index 19.
	fields := strings.Fields(string(raw[i+1:]))
	if want := procStatStartField - 3; len(fields) <= want {
		return "", Die("process start time for pid %d: /proc/%d/stat holds %d fields after its comm, want at least %d", pid, pid, len(fields), want+1)
	}
	return fields[procStatStartField-3], nil
}

// procCwdOf is the kernel's own reading of where pid is running:
// `readlink /proc/<pid>/cwd`. runtimesession.go's private-first read takes
// it to find a caged seat's own session row (ranger-base-lx5jk), and it sits
// here because it is the same file family as the fence above: the kernel's
// answer about a process, not the process's answer about itself. A row's own
// `cwd` field would be the thing being located saying where it is.
//
// A cwd that has been removed reads back with a " (deleted)" suffix; that is
// returned as it is, names no seat cache anyone made, and the caller's read
// of it fails into the shared row.
func procCwdOf(pid int) (string, error) {
	if pid <= 0 {
		return "", Die("no working directory for pid %d", pid)
	}
	cwd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	if err != nil {
		return "", Die("working directory of pid %d: %v", pid, err)
	}
	return cwd, nil
}
