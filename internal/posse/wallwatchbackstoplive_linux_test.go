//go:build !posse_arm2 && !posse_arm3

package posse

// Live pins for ranger-base-zd4d4 (ranger-base-0qpsw D3): the dispatch guard
// tick's backstop over a real bwrap seat whose watcher has died. Split out of
// wallwatchlive_test.go under ranger-base-tea4e because these two arms are the
// only pins in that file that name a Linux-only symbol — wallWatchProcProbe,
// the backstop's reading of the live process table, which lives in
// wallwatch_linux.go. The rest of wallwatchlive_test.go compiles on every GOOS
// and skips at runtime, and the land gate's `GOOS=darwin go vet ./...` (step 5)
// reds on a test file that does not compile there even though nothing would
// run it. The `_linux_test.go` SUFFIX is what constrains this file, not a tag
// in the build line, so armtags_qa_test.go's classifier still reads one arm
// here — the same shape as wallwatchtty_linux_test.go.
//
// The fixture, the skips (off Linux, without bwrap, inside a wall that forbids
// nesting) and the helpers are wallwatchlive_test.go's; only these two tests
// moved.
//
//	go test ./internal/posse -run TestLiveWallWatchBackstop -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// ranger-base-zd4d4 (0qpsw D3): a seat whose watcher is KILLED, then loses a
// wall to a host rename, is killed and filed by the next dispatch tick. The
// watcher is this test binary re-run as `posse wall-watch` (TestMain), so it
// is a process the test can SIGKILL. What survives that kill is what D3 is
// for: bwrap and its direct child die with the watcher (--die-with-parent),
// the seat's backgrounded child does not, and it keeps the seat's view.
func TestLiveWallWatchBackstopKillsASeatWhoseWatcherIsGone(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	// The launch's layout: the mounts file in a persona's gates dir under a
	// state dir, which is where the tick looks for records.
	state := filepath.Dir(s.grant)
	gates := filepath.Join(state, "gates", "seat-d")
	if err := os.MkdirAll(gates, 0o755); err != nil {
		t.Fatal(err)
	}
	s.mounts = filepath.Join(gates, "bwrap.mounts")
	carve := SeatbeltCarveOut{Deny: []string{s.deny}, DenyFile: []string{s.deny}}
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644)
	os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644)
	af, err := os.Open(s.args)
	if err != nil {
		t.Fatal(err)
	}
	defer af.Close()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// A FILE and not a Builder: the seat's background child inherits the
	// watcher's stdout and outlives it, and Wait on a pipe waits for that.
	out, err := os.Create(filepath.Join(t.TempDir(), "watcher.out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	said := func() string { b, _ := os.ReadFile(out.Name()); return string(b) }
	cmd := exec.Command(exe, WallWatchVerb, "--mounts", s.mounts, "--seat", "seat-d", "--dir", s.grant, "--",
		"bwrap", "--args", "3", "--", "sh", "-c", "sleep 300 & echo $! > "+shellQuote(s.bgpid)+"; wait")
	cmd.ExtraFiles = []*os.File{af}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	// werr, where this used to discard Wait's error (ranger-base-k2quj).
	// The watcher is this file's seat runner, and the fatal below reads "no
	// bgpid file" as the seat never running — so a watcher the box SIGKILLs
	// silently arrives as ADR 0097 D3's exit 1, which D5's ratchet files as
	// a P1 against whoever landed last. Written once, read after `waited`
	// closes and never before, so nothing races the goroutine.
	var werr error
	go func() { werr = cmd.Wait(); close(waited) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-waited })

	deadline := time.Now().Add(10 * time.Second)
	for {
		if b, err := os.ReadFile(s.bgpid); err == nil && strings.TrimSpace(string(b)) != "" {
			break
		}
		if time.Now().After(deadline) {
			// ONE LINE WITH THE WATCHER'S STATUS IN IT, which this fatal
			// did not have (ranger-base-k2quj): `scripts/posse-wall-check`
			// carries a failed member's FIRST `file.go:NN:` line and
			// nothing after it, so a `:\n%s` spent the whole budget on the
			// output and said nothing about whether the watcher was even
			// alive — the row that discriminates (ADR 0097 D3,
			// ranger-base-a688i/-axt1f). `werr` is only an answer once the
			// waiter has closed, so a watcher that is merely slow is named
			// as that and not as `exit 0`.
			status := "the watcher is still running, and said " + wwOneLine(said())
			select {
			case <-waited:
				bgSeatAbstainIfOutside(t, "the wall-watch supervisor over seat-d", werr, said())
				status = "the watcher is gone: " + bgSeatAccount(werr, said())
			default:
			}
			t.Fatalf("the seat never ran — %s", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	bg := s.bg(t)
	t.Cleanup(func() { syscall.Kill(bg, syscall.SIGKILL) })
	recs, _ := filepath.Glob(wallWatchRecordGlob(state))
	if len(recs) != 1 {
		t.Fatalf("the watcher wrote %d records under %s, want 1", len(recs), state)
	}

	// A tick over a healthy seat under a live watcher says nothing.
	st := &fakeWallWatchStore{}
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")
	if lines := wallWatchBackstop(state, st, lock, time.Now, wallWatchProcProbe{}); len(lines) != 0 || len(st.issues) != 0 {
		t.Fatalf("a healthy watched seat: lines %q, %d beads", lines, len(st.issues))
	}

	// Kill the watcher. The seat's background child outlives it.
	cmd.Process.Kill()
	<-waited
	time.Sleep(100 * time.Millisecond)
	if syscall.Kill(bg, 0) != nil {
		t.Fatal("the fixture's background child died with the watcher, so there is no unwatched seat to test")
	}

	// Rename over the Deny bind on the host. Nobody is watching.
	renameOver(t, s.deny, false)

	// The next tick: the seat is killed and the detach is filed.
	lines := wallWatchBackstop(state, st, lock, time.Now, wallWatchProcProbe{})
	wwGone(t, bg, "backstop")
	if len(st.issues) != 1 {
		t.Fatalf("%d beads, want 1 (lines %q)", len(st.issues), lines)
	}
	d := st.issues[0].Description
	for _, want := range []string{wallWatchMarker(s.deny), "seat: seat-d", "dispatch watch tick", "was gone"} {
		if !strings.Contains(d, want) {
			t.Errorf("the bead does not say %q:\n%s", want, d)
		}
	}
	if recs, _ := filepath.Glob(wallWatchRecordGlob(state)); len(recs) != 0 {
		t.Errorf("the dead watcher's record survived the seat's end: %v", recs)
	}
}

// The other D3 arm, live: the watcher is gone and every wall is still up —
// one bead, no kill, and nothing more on the next tick.
func TestLiveWallWatchBackstopFilesAWatcherThatDiedUnderARunningSeat(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	state := filepath.Dir(s.grant)
	gates := filepath.Join(state, "gates", "seat-e")
	os.MkdirAll(gates, 0o755)
	s.mounts = filepath.Join(gates, "bwrap.mounts")
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, SeatbeltCarveOut{Deny: []string{s.deny}, DenyFile: []string{s.deny}})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644)
	os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644)
	af, _ := os.Open(s.args)
	defer af.Close()
	exe, _ := os.Executable()
	cmd := exec.Command(exe, WallWatchVerb, "--mounts", s.mounts, "--seat", "seat-e", "--dir", s.grant, "--",
		"bwrap", "--args", "3", "--", "sh", "-c", "sleep 300 & echo $! > "+shellQuote(s.bgpid)+"; wait")
	cmd.ExtraFiles = []*os.File{af}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// ONE WAITER, for the reason the arm above gives (ranger-base-k2quj):
	// the fatal below reads "no bgpid file" as the seat never running, and
	// a watcher the box SIGKILLs silently would arrive as that finding —
	// ADR 0097 D3's exit 1, which D5's ratchet files as a P1 against
	// whoever landed last. The read is non-blocking, because a watcher that
	// is merely slow must still reach that fatal; `Process.Signal(0)` would
	// not do, since a child that has exited but is not yet reaped still
	// takes signal 0 without error.
	var werr error
	waited := make(chan struct{})
	go func() { werr = cmd.Wait(); close(waited) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if b, err := os.ReadFile(s.bgpid); err == nil && strings.TrimSpace(string(b)) != "" {
			break
		}
		if time.Now().After(deadline) {
			// BEFORE the Kill, and non-blocking: a watcher this process
			// killed answers `signal: killed` too, so reading the status
			// after the Kill would abstain on every real "the seat never
			// ran". What is asked is whether it was ALREADY gone.
			// The status on the line, for the arm above's reason
			// (ranger-base-k2quj): this fatal carried no account at all,
			// and the one line the wall check puts on a P1 bead could not
			// say whether the watcher was alive.
			status := "the watcher is still running"
			select {
			case <-waited:
				bgSeatAbstainIfOutside(t, "the wall-watch supervisor over seat-e", werr, "")
				status = "the watcher is gone: " + bgSeatAccount(werr, "")
			default:
			}
			cmd.Process.Kill()
			<-waited
			t.Fatalf("the seat never ran — %s", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
	bg := s.bg(t)
	t.Cleanup(func() { syscall.Kill(bg, syscall.SIGKILL) })
	cmd.Process.Kill()
	<-waited
	time.Sleep(100 * time.Millisecond)

	st := &fakeWallWatchStore{}
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")
	wallWatchBackstop(state, st, lock, time.Now, wallWatchProcProbe{})
	wallWatchBackstop(state, st, lock, time.Now, wallWatchProcProbe{})
	if syscall.Kill(bg, 0) != nil {
		t.Error("a seat whose walls were all up was killed")
	}
	if len(st.issues) != 1 || !strings.Contains(st.issues[0].Title, "wall-watch of seat seat-e is gone") {
		t.Fatalf("want one bead naming the dead watcher over two ticks, got %+v", st.issues)
	}
	if !wwStillRunningNames(st.issues[0].Description, bg) {
		t.Errorf("the bead does not name the still-running process %d:\n%s", bg, st.issues[0].Description)
	}
}

// wwStillRunningNames: does the bead's "still running:" line name pid p as one
// of its pids? ranger-base-ebv8j: wallWatchPidList renders the probe's pids in
// PROBE order, and bg is only ONE of them — the seat `sh` is in the same mount
// namespace and lives too. On every ubuntu-latest arm-1 job the probe returned
// the seat pid first, so an assertion anchored on the substring "pids <bg>"
// red over a bead that was correct ("pids 166926 166927" for bg 166927). Which
// pid the probe returns first is not this test's subject.
//
// It reads the LIST and not the whole description, which is what keeps the
// negative control: a bead naming no live pid renders "still running: none",
// the prefix does not match, and this answers false. Comparing whole FIELDS
// and not a substring is what keeps 1669 out of 166927.
func wwStillRunningNames(desc string, p int) bool {
	for _, line := range strings.Split(desc, "\n") {
		rest, ok := strings.CutPrefix(line, "still running: pids ")
		if !ok {
			continue
		}
		for _, f := range strings.Fields(rest) {
			if f == strconv.Itoa(p) {
				return true
			}
		}
	}
	return false
}

// The helper above, over rendered descriptions: the live arm can only exercise
// whatever order the probe happens to return on the box it runs on — here the
// wanted pid came first, which is why the old assertion passed on this box and
// red on every runner — so the order-independence and the negative control are
// pinned hermetically, against the real renderer.
func TestWallWatchStillRunningNamesReadsTheListAndNotItsFirstPid(t *testing.T) {
	t.Parallel()
	desc := func(live ...int) string {
		return wallWatchOrphanDescription(wallWatchRecord{
			Seat: "seat-e", Dir: "/d", WatcherPid: 166915, BwrapPid: 166925, SeatPid: 166926,
		}, live, time.Now())
	}
	for _, tc := range []struct {
		name string
		desc string
		want bool
	}{
		{"first of two", desc(166927, 166926), true},
		{"last of two", desc(166926, 166927), true},
		{"only pid", desc(166927), true},
		{"not in the list", desc(166926, 166928), false},
		{"no live pid at all", desc(), false},
		{"a prefix of a listed pid is not a match", desc(1669271), false},
		{"named only as the seat's own pid", wallWatchOrphanDescription(wallWatchRecord{
			Seat: "seat-e", Dir: "/d", WatcherPid: 1, BwrapPid: 2, SeatPid: 166927,
		}, nil, time.Now()), false},
	} {
		if got := wwStillRunningNames(tc.desc, 166927); got != tc.want {
			t.Errorf("%s: wwStillRunningNames = %v, want %v:\n%s", tc.name, got, tc.want, tc.desc)
		}
	}
}

// TestWallWatchLeaderLiveNeedsPositiveEvidenceOnBothCounts is ranger-base-xxtmo's,
// verifying ranger-base-6ec6n. wallWatchLeaderLive is the WHOLE gate on the kill
// that close added to the watcher's bwrap-exit path (wallwatch_linux.go), and it
// makes two distinctions its own comment claims and nothing read:
//
//   - the pid is in the SEAT'S mount namespace, so a leader pid that was reaped
//     and recycled by an unrelated process does not spend a kill — this is the
//     load-bearing one;
//   - the pid is not a zombie.
//
// MEASURED 2026-11-21 on D1 (Linux 7.2.15-1-omarchy, go1.27.3): dropping EITHER
// arm left `go test -run 'WallWatch|WallUnix' ./internal/posse` wholly green, and
// no test named the symbol at all. The live arms of
// TestLiveWallWatchKillsASeatItsBwrapLeftRunning cannot reach them — its seat's
// leader is alive in the seat's own namespace, and its control's leader has been
// reaped by bwrap, so the /proc read answers ENOENT before either arm is asked.
//
// WHICH ROWS GRADE WHICH ARM, so that nobody reads more into them than they
// hold. The namespace row is the one that grades a branch: dropping the inode
// comparison, or the whole namespace read, reds it and nothing else does
// (MEASURED 2026-11-21, both spellings). The other rows read the OBSERVABLE and
// grade no branch of their own, because on this kernel two of
// wallWatchLeaderLive's guards are belt over braces and the reads below them
// answer the same way:
//
//   - a zombie: MEASURED 2026-11-21, its /proc/<pid>/ns/mnt symlink exists but
//     does not resolve (`stat -L` answers ENOENT), so the namespace arm already
//     says false and removing the state read changes no verdict here;
//   - pid <= 0, a negative pid, and mntns == 0: the /proc read fails, or the
//     inode cannot equal 0, so the early guard changes no verdict either —
//     MEASURED, deleting it leaves this pin green.
//
// They are pinned anyway, because what must not change is that none of those
// four shapes buys a kill, whichever arm is the one that says so.
//
// THE FIRST ROW IS THE POSITIVE CONTROL: without a row that reads true, every
// row below is satisfied by a predicate that cannot say yes.
func TestWallWatchLeaderLiveNeedsPositiveEvidenceOnBothCounts(t *testing.T) {
	t.Parallel()
	var self unix.Stat_t
	if err := unix.Stat("/proc/self/ns/mnt", &self); err != nil {
		t.Skipf("this kernel does not answer /proc/self/ns/mnt (%v), so there is no namespace to compare against", err)
	}
	me := os.Getpid()

	// A zombie: a child that has exited and has not been waited for. Reaped in
	// the cleanup, so the pin leaves nothing behind.
	zombie := exec.Command("/bin/sh", "-c", "exit 0")
	if err := zombie.Start(); err != nil {
		t.Fatal(err)
	}
	zpid := zombie.Process.Pid
	t.Cleanup(func() { _ = zombie.Wait() })
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		b, err := os.ReadFile("/proc/" + strconv.Itoa(zpid) + "/stat")
		if err != nil {
			break
		}
		if st, _, ok := procStatStateParent(string(b)); ok && st == 'Z' {
			break
		}
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(zpid) + "/stat")
	if err != nil {
		t.Fatalf("NOTHING MEASURED: the zombie child (pid %d) left /proc before it could be read (%v), so the zombie row below is not a zombie", zpid, err)
	}
	if st, _, ok := procStatStateParent(string(b)); !ok || st != 'Z' {
		t.Fatalf("NOTHING MEASURED: pid %d reads state %q, want 'Z' — the zombie row below would be measuring a live process", zpid, string(st))
	}

	for _, c := range []struct {
		name  string
		pid   int
		mntns uint64
		want  bool
	}{
		{"control: this process, in its own mount namespace", me, self.Ino, true},
		{"the same live pid against ANOTHER namespace — the recycled leader", me, self.Ino + 1, false},
		{"a zombie buys no kill", zpid, self.Ino, false},
		{"no pid", 0, self.Ino, false},
		{"a negative pid", -me, self.Ino, false},
		{"no namespace on the record", me, 0, false},
		{"a pid that is not there", 1 << 30, self.Ino, false},
	} {
		if got := wallWatchLeaderLive(c.pid, c.mntns); got != c.want {
			t.Errorf("%s: wallWatchLeaderLive(%d, %d) = %v, want %v — this is the whole gate on the kill ranger-base-6ec6n added to the bwrap-exit path, so a true here is a kill and a false here is a seat left running (ranger-base-xxtmo)",
				c.name, c.pid, c.mntns, got, c.want)
		}
	}
}
