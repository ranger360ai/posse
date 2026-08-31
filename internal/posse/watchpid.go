package posse

// The watch record (rangerhq-ct9): a `posse dispatch --watch` loop leaves a
// note of itself at $RHQ_HOME/state/dispatch-watch.pid — which pid, since
// when, under what argv.
//
// It is IDENTITY, not evidence (rangerhq-gir5). It was written to answer
// "is the fleet's loop running?", because the appearance that lied was the
// workspace: plugin/autostart.sh read "a session named `dispatch` exists at
// server start" as "herdr restored the layout without re-running the
// command" and replaced it, when herdr runs plugin [[startup]] hooks on a
// LIVE HANDOFF too (`herdr update --handoff` → run_handoff_import_server)
// and there the workspace comes back *with* its command still running. Same
// appearance, opposite truth.
//
// But a file cannot answer that question either. A loop killed with its
// pane never removes this one, so its truth decays, and every reader had to
// reconstruct liveness from the decayed copy — signal 0 plus an argv match,
// patched twice and leaking still (rangerhq-ppy9, rangerhq-mugy). The
// question moved to WatchLockPath, where the kernel answers it and release
// is process death. This file now says who, once something else has said
// whether: a missing or stale one costs a name in a message and no more.
//
// One file per RHQ_HOME, not one per session: the invariant worth
// protecting is "at most one loop dispatching this queue", and that does
// not care what the workspace is called.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// WatchPid is what the file holds: flat-YAML, `pid:` first, so a shell
// reader is one `sed`. Started and Cmd are for the human reading it.
type WatchPid struct {
	Pid     int
	Started time.Time
	Cmd     string
}

// WatchPidPath is the one pidfile of an RHQ_HOME, beside dispatch-watch.log.
func WatchPidPath(a *App) string {
	return filepath.Join(a.StateDir, "dispatch-watch.pid")
}

func WriteWatchPid(path string, w WatchPid) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var s strings.Builder
	fmt.Fprintf(&s, "pid: %d\n", w.Pid)
	if !w.Started.IsZero() {
		fmt.Fprintf(&s, "started: %s\n", w.Started.UTC().Format(time.RFC3339))
	}
	if w.Cmd != "" {
		fmt.Fprintf(&s, "cmd: %s\n", w.Cmd)
	}
	return os.WriteFile(path, []byte(s.String()), 0o644)
}

// ReadWatchPid returns the record, or false when there is no readable one.
func ReadWatchPid(path string) (*WatchPid, bool) {
	if _, err := os.Stat(path); err != nil {
		return nil, false
	}
	pid, err := strconv.Atoi(YamlGet(path, "pid"))
	if err != nil || pid <= 0 {
		return nil, false
	}
	started, _ := time.Parse(time.RFC3339, YamlGet(path, "started"))
	return &WatchPid{Pid: pid, Started: started, Cmd: YamlGet(path, "cmd")}, true
}

// pidAlive asks whether a pid names a live process. Signal 0 is the
// question, not an action; EPERM means it exists and is somebody else's,
// which for "is something running" is still yes. It answers liveness and
// not identity — a recycled pid is alive and is a stranger — so no caller
// may use it alone as proof that the process it recorded is the one there.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || err == syscall.EPERM
}

// RemoveWatchPid deletes the file only while it still names pid — a loop
// shutting down must not delete the record of the loop that replaced it.
func RemoveWatchPid(path string, pid int) {
	if w, ok := ReadWatchPid(path); ok && w.Pid == pid {
		_ = os.Remove(path)
	}
}
