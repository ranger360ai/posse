//go:build !posse_arm2 && !posse_arm3

package posse

// Live pin for ranger-base-ihnnl (ranger-base-0qpsw D1+D2): a real bwrap seat
// rendered by bwrapArgsWatched, watched by runWallWatch, loses a wall to a
// HOST rename — and is killed, with one bead filed. Skips off Linux and
// without bwrap, like the other live bwrap pins (bgLiveSkip's rule).
//
//	go test ./internal/posse -run TestLiveWallWatch -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type wwLiveSeat struct {
	grant, priv, privSrc, deny, bgpid, mounts, args string
}

func wwLiveFixture(t *testing.T) wwLiveSeat {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := wwLiveSeat{
		grant:   filepath.Join(root, "cfg"),
		privSrc: filepath.Join(root, "seat", "shell-snapshots"),
		mounts:  filepath.Join(root, "bwrap.mounts"),
		args:    filepath.Join(root, "bwrap.args"),
	}
	s.priv = filepath.Join(s.grant, "shell-snapshots")
	s.deny = filepath.Join(s.grant, "settings.json")
	s.bgpid = filepath.Join(s.grant, "bg.pid")
	for _, d := range []string{s.grant, s.priv, s.privSrc} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(s.deny, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return s
}

// launch renders the seat, starts it under the watcher, and returns once the
// seat's command is running (it has written the pid of a background child
// it leaves behind, so the kill is judged on a process bwrap did not start).
func (s wwLiveSeat) launch(t *testing.T, st wallWatchStore, seat string, lock string) (code chan int, bwrapPid *int) {
	t.Helper()
	devnull, _ := os.Open(os.DevNull)
	t.Cleanup(func() { devnull.Close() })
	return s.launchOn(t, st, seat, lock, devnull)
}

// launchOn is launch with the watcher's stdin named — the pane's terminal,
// for a pin about what is typed at it (ranger-base-heypq).
func (s wwLiveSeat) launchOn(t *testing.T, st wallWatchStore, seat string, lock string, stdin *os.File) (code chan int, bwrapPid *int) {
	t.Helper()
	carve := SeatbeltCarveOut{
		Deny:     []string{s.deny},
		DenyFile: []string{s.deny},
		Private:  []PrivateBind{{Src: s.privSrc, Dst: s.priv}},
	}
	argv, prep, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	if len(prep) != 0 {
		t.Fatalf("fixture needs prep %v", prep)
	}
	if err := os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(s.bgpid)
	af, err := os.Open(s.args)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { af.Close() })
	var stderr strings.Builder
	code = make(chan int, 1)
	pid := new(int)
	go func() {
		code <- runWallWatch(wallWatchRun{
			Opts: wallWatchOpts{Mounts: s.mounts, Seat: seat, Dir: s.grant,
				Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", "sleep 300 & echo $! > " + shellQuote(s.bgpid) + "; wait"}},
			Args: af, Stdin: stdin, Stdout: &stderr, Stderr: &stderr,
			Store: st, Lock: lock, PollInterval: 50 * time.Millisecond,
			OnDetach: func(d wallWatchDetach) { *pid = d.BwrapPid },
		})
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if b, err := os.ReadFile(s.bgpid); err == nil && strings.TrimSpace(string(b)) != "" {
			break
		}
		select {
		case c := <-code:
			t.Fatalf("the seat ended before it ran (exit %d):\n%s", c, stderr.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the seat never ran:\n%s", stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	return code, pid
}

func (s wwLiveSeat) bg(t *testing.T) int {
	b, err := os.ReadFile(s.bgpid)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func wwWaitKilled(t *testing.T, code chan int, what string) {
	t.Helper()
	select {
	case c := <-code:
		if c != 1 {
			t.Errorf("%s: watcher exit = %d, want 1 (it killed the seat)", what, c)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("%s: the seat is still running 10s after its wall came off", what)
	}
}

func wwGone(t *testing.T, pid int, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		// A zombie still answers kill -0; its parent (sh, in the seat) is
		// dead, so init reaps it. Read the state instead of waiting on it.
		if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
			if st, _, ok := procStatStateParent(string(b)); ok && st == 'Z' {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Errorf("%s: pid %d is still alive after the kill — a seat process outlived its wall", what, pid)
			syscall.Kill(pid, syscall.SIGKILL)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// renameOver replaces `dst` on the host by rename, the way an atomic writer
// (or a purge) does.
func renameOver(t *testing.T, dst string, dir bool) {
	t.Helper()
	tmp := dst + ".new"
	if dir {
		if err := os.Mkdir(tmp, 0o755); err != nil {
			t.Fatal(err)
		}
	} else if err := os.WriteFile(tmp, []byte("{\"replaced\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// syscall.Rename and not os.Rename: os.Rename refuses an existing
	// directory as the target itself (EEXIST), which rename(2) does not.
	if err := syscall.Rename(tmp, dst); err != nil {
		ents, _ := os.ReadDir(dst)
		t.Fatalf("host rename over %s: %v (it holds %v)", dst, err, ents)
	}
}

func TestLiveWallWatchKillsASeatWhoseWallComesOffAndFilesOneBead(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	st := &fakeWallWatchStore{}
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")

	// 1. A Deny bind over a FILE, replaced on the host by rename.
	code, _ := s.launch(t, st, "seat-a", lock)
	bg := s.bg(t)
	renameOver(t, s.deny, false)
	wwWaitKilled(t, code, "deny")
	wwGone(t, bg, "deny")
	if len(st.issues) != 1 {
		t.Fatalf("deny: %d beads filed, want 1: %+v", len(st.issues), st.issues)
	}
	if d := st.issues[0].Description; !strings.Contains(d, wallWatchMarker(s.deny)) || !strings.Contains(d, "seat: seat-a") {
		t.Errorf("deny: the bead does not name the seat and the path:\n%s", d)
	}

	// 2. A Private bind over a DIRECTORY, replaced on the host by rename.
	// A different path, so a second bead.
	code, _ = s.launch(t, st, "seat-b", lock)
	bg = s.bg(t)
	renameOver(t, s.priv, true)
	wwWaitKilled(t, code, "private")
	wwGone(t, bg, "private")
	if len(st.issues) != 2 || !strings.Contains(st.issues[1].Description, wallWatchMarker(s.priv)) {
		t.Fatalf("private: want a second bead naming %s, got %+v", s.priv, st.issues)
	}

	// 3. The deny path again inside ten minutes: the seat is still killed,
	// and the detection is a comment on the first bead, not a third bead.
	code, _ = s.launch(t, st, "seat-c", lock)
	bg = s.bg(t)
	renameOver(t, s.deny, false)
	wwWaitKilled(t, code, "deny again")
	wwGone(t, bg, "deny again")
	if len(st.issues) != 2 {
		t.Errorf("deny again: %d beads, want still 2", len(st.issues))
	}
	if c := st.comments[st.issues[0].ID]; len(c) != 1 || !strings.Contains(c[0], "seat-c") {
		t.Errorf("deny again: comments on %s = %q, want one naming seat-c", st.issues[0].ID, c)
	}
}

// A seat whose walls stay up runs to its own end, and the watcher hands back
// its exit code.
func TestLiveWallWatchPassesTheSeatsOwnExitCode(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	carve := SeatbeltCarveOut{Deny: []string{s.deny}, DenyFile: []string{s.deny}}
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644)
	os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644)
	af, _ := os.Open(s.args)
	defer af.Close()
	var out strings.Builder
	st := &fakeWallWatchStore{}
	got := runWallWatch(wallWatchRun{
		Opts: wallWatchOpts{Mounts: s.mounts, Seat: "x", Dir: s.grant,
			Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", "echo hi > " + shellQuote(s.deny) + " 2>/dev/null && exit 9; exit 7"}},
		Args: af, Stdout: &out, Stderr: &out, Store: st, PollInterval: 50 * time.Millisecond,
	})
	if got != 7 {
		t.Errorf("exit = %d, want 7 (the seat's own; a write to the denied file must have failed):\n%s", got, out.String())
	}
	if len(st.issues) != 0 {
		t.Errorf("a seat that kept its walls filed %d beads", len(st.issues))
	}
}

// A wall that is already off when the watcher first reads the table — here,
// a path in the list the render never mounted — stops the seat before its
// command runs.
func TestLiveWallWatchRefusesToReleaseASeatMissingAWallAtLaunch(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	watch = append(watch, WallWatchMount{WallWatchDeny, s.deny})
	os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644)
	os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644)
	af, _ := os.Open(s.args)
	defer af.Close()
	ran := filepath.Join(s.grant, "ran")
	var out strings.Builder
	st := &fakeWallWatchStore{}
	got := runWallWatch(wallWatchRun{
		Opts: wallWatchOpts{Mounts: s.mounts, Seat: "x", Dir: s.grant,
			Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", ": > " + shellQuote(ran)}},
		Args: af, Stdout: &out, Stderr: &out, Store: st, PollInterval: 50 * time.Millisecond,
	})
	if got != 1 {
		t.Errorf("exit = %d, want 1:\n%s", got, out.String())
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("the seat's command ran with a wall missing")
	}
	if len(st.issues) != 1 || !strings.Contains(st.issues[0].Description, "at launch") {
		t.Errorf("want one bead saying the wall was missing at launch, got %+v", st.issues)
	}
}
