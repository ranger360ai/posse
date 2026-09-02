package posse

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// liveProcess starts a real process that outlives the call and returns its
// pid; the script's path carries `name`, which is what the autostart hook
// greps for when it guards against pid reuse.
func liveProcess(t *testing.T, name string) int {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, name)
	if err := os.WriteFile(script, []byte("sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return cmd.Process.Pid
}

// liveArgv is the production-shaped cousin of liveProcess: argv0 is what
// `ps -p -o command=` shows (macOS exec -a). The autostart hook greps that
// string, not the pidfile's cmd: field.
func liveArgv(t *testing.T, argv0 string) int {
	t.Helper()
	cmd := exec.Command("bash", "-c", "exec -a "+shq(argv0)+" sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return cmd.Process.Pid
}

func TestWatchPidRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state", "dispatch-watch.pid")
	started := time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)
	if err := WriteWatchPid(path, WatchPid{Pid: os.Getpid(), Started: started, Cmd: "posse dispatch --watch 5m"}); err != nil {
		t.Fatal(err)
	}
	// The file is flat-YAML with `pid:` first: plugin/autostart.sh reads it
	// with one sed, so the spelling is a contract, not an implementation
	// detail.
	body, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(body), "pid: ") {
		t.Errorf("pidfile must lead with `pid: <n>`, got:\n%s", body)
	}
	w, ok := ReadWatchPid(path)
	if !ok || w.Pid != os.Getpid() || !w.Started.Equal(started) || w.Cmd != "posse dispatch --watch 5m" {
		t.Fatalf("round trip lost something: %+v (ok=%v)", w, ok)
	}
}

// The record is identity and says nothing about liveness (rangerhq-gir5):
// a dead loop leaves it behind, and it stays perfectly readable — the lock
// is what makes that harmless. What this pins is that reading it never
// silently invents a record where there is none.
func TestWatchPidIsARecordNotALiveness(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "dispatch-watch.pid")

	if _, ok := ReadWatchPid(path); ok {
		t.Error("no file must read as no record")
	}
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	dead := cmd.Process.Pid
	if err := WriteWatchPid(path, WatchPid{Pid: dead}); err != nil {
		t.Fatal(err)
	}
	w, ok := ReadWatchPid(path)
	if !ok || w.Pid != dead {
		t.Fatalf("a stale record must still name its pid: %+v (ok=%v)", w, ok)
	}
	for _, junk := range []string{"", "pid: nonsense\n", "pid: 0\n", "started: now\n"} {
		os.WriteFile(path, []byte(junk), 0o644)
		if _, ok := ReadWatchPid(path); ok {
			t.Errorf("unusable pidfile %q must read as no record", junk)
		}
	}
}

func TestRemoveWatchPidKeepsAnotherLoopsRecord(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "dispatch-watch.pid")
	WriteWatchPid(path, WatchPid{Pid: 424242})
	RemoveWatchPid(path, os.Getpid())
	if _, err := os.Stat(path); err != nil {
		t.Error("a loop shutting down must not delete the record of the loop that replaced it")
	}
	RemoveWatchPid(path, 424242)
	if _, err := os.Stat(path); err == nil {
		t.Error("its own record must go")
	}
}

// The loop stamps itself while it runs and clears the stamp when it ends.
func TestWatchStampsAndClearsPidfile(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var errs strings.Builder
	d.Err = &errs
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	// A foreign record whose pid happens to be alive is overwritten, in
	// silence. It used to earn "another dispatch --watch loop looks live",
	// which was an inference from a decayed file and cried wolf at every
	// recycled pid; the lock is what knows now, and it let this loop start
	// (rangerhq-gir5).
	other := liveProcess(t, "dispatch-watch-stub")
	WriteWatchPid(WatchPidPath(b.App), WatchPid{Pid: other})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	var seen *WatchPid
	for i := 0; i < 200; i++ {
		if w, ok := ReadWatchPid(WatchPidPath(b.App)); ok && w.Pid == os.Getpid() {
			seen = w
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if seen == nil {
		t.Fatalf("the loop never stamped %s", WatchPidPath(b.App))
	}
	if seen.Started.IsZero() || !strings.Contains(seen.Cmd, os.Args[0]) {
		t.Errorf("stamp is missing its provenance: %+v", seen)
	}
	if _, err := os.Stat(WatchPidPath(b.App)); err == nil {
		t.Error("a loop that ends cleanly must clear its stamp")
	}
	if strings.Contains(errs.String(), "looks live") {
		t.Errorf("a stale record is not a second loop — the lock decides that: %q", errs.String())
	}
}
