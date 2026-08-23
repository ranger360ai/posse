package rhq

// The watch pidfile (rangerhq-ct9): a `posse dispatch --watch` loop leaves a
// record of itself at $RHQ_HOME/state/dispatch-watch.pid, so anything asking
// "is the fleet's loop running?" can answer from liveness instead of from
// appearances.
//
// The appearance that lied was the workspace. plugin/autostart.sh read "a
// session named `dispatch` exists at server start" as "herdr restored the
// layout without re-running the command" and replaced it. But herdr runs
// plugin [[startup]] hooks on a LIVE HANDOFF too (`herdr update --handoff`
// → run_handoff_import_server), and there the workspace comes back *with*
// its command still running — same pid, PTY fd passed across. Same
// appearance, opposite truth: the hook would have killed a live loop
// mid-pass, with a bead claimed and a prompt in flight, and started a
// second one on the same queue.
//
// One file per RHQ_HOME, not one per session: the invariant worth
// protecting is "at most one loop dispatching this queue", and that does
// not care what the workspace is called. A loop killed with its pane leaves
// the file behind, so existence proves nothing — every reader checks
// liveness. The hook additionally matches the process's argv, because a
// stale file whose pid has been recycled would read as live and silently
// leave the fleet unarmed.

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

// Alive says the recorded process still exists.
func (w *WatchPid) Alive() bool {
	return w != nil && pidAlive(w.Pid)
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
