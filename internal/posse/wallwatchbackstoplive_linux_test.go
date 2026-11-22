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

// ranger-base-5k7vf, live: a seat whose WATCHER and whose BWRAP are both gone
// and whose leader is still running. That seat has no bound on its lifetime at
// all — not its own pdeathsig (an AppArmor exec transition zeroed it,
// ranger-base-6ec6n), not bwrap, and not the watcher whose kill on bwrap's
// exit was the whole of the bound on a seat line with no `posse wall-unix`
// step. The tick must kill it and say KILLED.
//
// THE FIXTURE IS TestLiveWallWatchKillsASeatItsBwrapLeftRunning'S, one step
// further on. D1 has no AppArmor, so the cleared pdeathsig is modelled the
// only honest way it can be: `--die-with-parent` comes off the bwrap line,
// which is exactly what the transition leaves behind. Then the watcher is
// SIGKILLed — and because that flag is off, bwrap outlives it too, which is
// not the production shape — and then bwrap, which reparents the leader. What
// the tick is handed at the end is the production shape: a record whose
// watcher is gone, whose bwrap is gone, and whose leader is alive in the
// seat's own mount namespace under some other parent.
func TestLiveWallWatchBackstopKillsASeatOrphanedWithItsWatcherGone(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	state := filepath.Dir(s.grant)
	gates := filepath.Join(state, "gates", "seat-f")
	if err := os.MkdirAll(gates, 0o755); err != nil {
		t.Fatal(err)
	}
	s.mounts = filepath.Join(gates, "bwrap.mounts")
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, SeatbeltCarveOut{Deny: []string{s.deny}, DenyFile: []string{s.deny}})
	if err != nil {
		t.Fatal(err)
	}
	var cut []string
	for _, a := range argv {
		if a != "--die-with-parent" {
			cut = append(cut, a)
		}
	}
	if len(cut) != len(argv)-1 {
		t.Fatalf("the render carries --die-with-parent %d time(s), want 1 — this fixture takes it off to model the AppArmor clear", len(argv)-len(cut))
	}
	if err := os.WriteFile(s.args, bwrapArgsBytes(cut), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644); err != nil {
		t.Fatal(err)
	}
	af, err := os.Open(s.args)
	if err != nil {
		t.Fatal(err)
	}
	defer af.Close()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// A FILE and not a Builder, for the arm above's reason: the seat's
	// background child inherits the watcher's stdout and outlives it.
	out, err := os.Create(filepath.Join(t.TempDir(), "watcher.out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	said := func() string { b, _ := os.ReadFile(out.Name()); return string(b) }
	cmd := exec.Command(exe, WallWatchVerb, "--mounts", s.mounts, "--seat", "seat-f", "--dir", s.grant, "--",
		"bwrap", "--args", "3", "--", "sh", "-c", "sleep 300 & echo $! > "+shellQuote(s.bgpid)+"; wait")
	cmd.ExtraFiles = []*os.File{af}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// ONE WAITER and a non-blocking read of its status, for the reason the
	// two arms above give (ranger-base-k2quj): the fatal below reads "no
	// bgpid file" as the seat never running, so a watcher the box SIGKILLs
	// silently would arrive as that finding instead of as an abstention.
	var werr error
	waited := make(chan struct{})
	go func() { werr = cmd.Wait(); close(waited) }()
	t.Cleanup(func() { cmd.Process.Kill(); <-waited })
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if b, rerr := os.ReadFile(s.bgpid); rerr == nil && strings.TrimSpace(string(b)) != "" {
			break
		}
		if time.Now().After(deadline) {
			status := "the watcher is still running, and said " + wwOneLine(said())
			select {
			case <-waited:
				bgSeatAbstainIfOutside(t, "the wall-watch supervisor over seat-f", werr, said())
				status = "the watcher is gone: " + bgSeatAccount(werr, said())
			default:
			}
			t.Fatalf("the seat never ran — %s", status)
		}
	}
	bg := s.bg(t)
	t.Cleanup(func() { syscall.Kill(bg, syscall.SIGKILL) })
	recs, _ := filepath.Glob(wallWatchRecordGlob(state))
	if len(recs) != 1 {
		t.Fatalf("the watcher wrote %d records under %s, want 1", len(recs), state)
	}
	r, err := readWallWatchRecord(recs[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(r.SeatPid, syscall.SIGKILL); syscall.Kill(r.BwrapPid, syscall.SIGKILL) })

	// The watcher. With --die-with-parent off the line, bwrap and the seat
	// both outlive it — so this is also the one check that says the fixture
	// is still modelling the host class the bead is about.
	cmd.Process.Kill()
	<-waited
	if !wallWatchLeaderLive(r.SeatPid, r.MntNS) {
		t.Fatalf("NOTHING MEASURED: the seat's leader (pid %d) died with its watcher, so this fixture is not an AppArmor host with a cleared pdeathsig — check that --die-with-parent came off the rendered line", r.SeatPid)
	}
	// Then bwrap, which is what reparents the leader.
	if kerr := syscall.Kill(r.BwrapPid, syscall.SIGKILL); kerr != nil {
		t.Fatalf("cannot kill the seat's bwrap (pid %d): %v", r.BwrapPid, kerr)
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		ppid, ok := wallWatchProcProbe{}.LeaderParent(r)
		if ok && ppid != r.BwrapPid {
			t.Logf("MEASURED: leader pid %d reparented from bwrap (pid %d) to pid %d, with watcher pid %d gone", r.SeatPid, r.BwrapPid, ppid, r.WatcherPid)
			break
		}
		if time.Now().After(deadline) {
			if !ok {
				t.Fatalf("NOTHING MEASURED: 10s after its bwrap (pid %d) was killed the seat's leader (pid %d) cannot be read in the seat's mount namespace — it died with its bwrap, so there is no orphaned seat to test", r.BwrapPid, r.SeatPid)
			}
			t.Fatalf("10s after its bwrap (pid %d) was SIGKILLed, the leader (pid %d) still reads that pid as its parent", r.BwrapPid, r.SeatPid)
		}
	}

	st := &fakeWallWatchStore{}
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")
	lines := wallWatchBackstop(state, st, lock, time.Now, wallWatchProcProbe{})
	wwGone(t, r.SeatPid, "the orphaned seat's leader")
	wwGone(t, bg, "the orphaned seat's backgrounded job")
	if len(st.issues) != 1 {
		t.Fatalf("%d beads, want 1 (lines %q)", len(st.issues), lines)
	}
	d := st.issues[0].Description
	for _, want := range []string{wallWatchOrphanKillMarker(r), "seat: seat-f", strconv.Itoa(r.SeatPid), "KILLED"} {
		if !strings.Contains(d, want) {
			t.Errorf("the bead does not say %q:\n%s", want, d)
		}
	}
	if recs, _ := filepath.Glob(wallWatchRecordGlob(state)); len(recs) != 0 {
		t.Errorf("the killed seat's record survived: %v", recs)
	}
	vs := readWallWatchVerdicts(state, "seat-f")
	if len(vs) != 1 || !strings.Contains(vs[0].Text, "KILLED seat seat-f") {
		t.Fatalf("the durable verdict of a killed seat (%d on disk): %+v", len(vs), vs)
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
	recs, _ := filepath.Glob(wallWatchRecordGlob(state))
	if len(recs) != 1 {
		t.Fatalf("the watcher wrote %d records under %s, want 1", len(recs), state)
	}
	r, rerr := readWallWatchRecord(recs[0])
	if rerr != nil {
		t.Fatal(rerr)
	}
	cmd.Process.Kill()
	<-waited

	// WHICH SEAT THIS NOW IS DEPENDS ON THE HOST (ranger-base-5k7vf), so it
	// is read and not assumed. Where bwrap's --die-with-parent held, the
	// leader went with the watcher and only the backgrounded child is left:
	// this arm's seat, filed and not killed. Where an AppArmor exec
	// transition cleared that pdeathsig, the leader is alive under a new
	// parent — and the tick is then the only bound that seat has left, so it
	// KILLS it. Both outcomes are this arm; the reading below says which.
	//
	// IT HAS TO BE A SETTLED READING, and the first draft of this was not:
	// a pdeathsig that was never cleared fires at bwrap's EXIT, and for the
	// moment between that exit and the leader's own the leader is alive and
	// already reparented — which is the orphaned shape exactly. MEASURED
	// 2026-11-22 on D1 (Linux 7.2.15-1-omarchy, bubblewrap 0.13.1, no
	// AppArmor): a loop that broke on the first orphaned reading took it,
	// and by the time the tick looked the leader was gone and the tick
	// filed D3's bead — the branch under test and the branch taken
	// disagreed. So the wait is for the leader to GO, and only a leader
	// still there, still reparented, when that wait is over is this host
	// leaving a seat behind. It costs the 2s on a host that does.
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if !wallWatchLeaderLive(r.SeatPid, r.MntNS) {
			break
		}
	}
	_, orphaned := wallWatchSeatOrphaned(r, wallWatchProcProbe{})

	st := &fakeWallWatchStore{}
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")
	wallWatchBackstop(state, st, lock, time.Now, wallWatchProcProbe{})
	wallWatchBackstop(state, st, lock, time.Now, wallWatchProcProbe{})
	if orphaned {
		t.Logf("MEASURED: this host left the seat's leader (pid %d) running past both its watcher (pid %d) and its bwrap (pid %d), so the arm under test is ranger-base-5k7vf's kill and not D3's bead", r.SeatPid, r.WatcherPid, r.BwrapPid)
		wwGone(t, r.SeatPid, "the orphaned seat's leader")
		wwGone(t, bg, "the orphaned seat's backgrounded job")
		if len(st.issues) != 1 || !strings.Contains(st.issues[0].Title, "outlived its wall-watch and its bwrap") {
			t.Fatalf("want one bead saying the seat was killed, got %+v", st.issues)
		}
		return
	}
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

// TestWallWatchProcProbeLeaderParentReadsTheRecordsLeaderAndNoOtherProcess is
// ranger-base-5k7vf's reading, over the real /proc and not a fake table. The
// backstop's new arm turns on ONE comparison — the leader's parent against the
// record's bwrap pid — and this is the reading that comparison is made of.
//
// It is hermetic and needs no bwrap: the subject is which process the probe
// answers ABOUT, and a child of this test binary is enough to say. The live
// arm above is the one that measures the AppArmor-shaped seat.
func TestWallWatchProcProbeLeaderParentReadsTheRecordsLeaderAndNoOtherProcess(t *testing.T) {
	t.Parallel()
	var self unix.Stat_t
	if err := unix.Stat("/proc/self/ns/mnt", &self); err != nil {
		t.Skipf("this kernel does not answer /proc/self/ns/mnt (%v), so there is no namespace to compare against", err)
	}
	me := os.Getpid()

	// A LIVE CHILD of this process: its parent is a pid this test knows, so
	// the control row is a reading and not a tautology.
	child := exec.Command("/bin/sh", "-c", "sleep 300")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	cpid := child.Process.Pid
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })

	// A zombie: exited, not yet waited for. Reaped in the cleanup.
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
		name     string
		r        wallWatchRecord
		wantPpid int
		wantOK   bool
	}{
		{"control: a live child, read in its own mount namespace", wallWatchRecord{SeatPid: cpid, MntNS: self.Ino}, me, true},
		{"the same live pid against ANOTHER namespace — the recycled leader", wallWatchRecord{SeatPid: cpid, MntNS: self.Ino + 1}, 0, false},
		{"a zombie is no leader", wallWatchRecord{SeatPid: zpid, MntNS: self.Ino}, 0, false},
		{"a pid that is not there", wallWatchRecord{SeatPid: 1 << 30, MntNS: self.Ino}, 0, false},
		{"no pid on the record", wallWatchRecord{MntNS: self.Ino}, 0, false},
		{"no namespace on the record", wallWatchRecord{SeatPid: cpid}, 0, false},
	} {
		ppid, ok := wallWatchProcProbe{}.LeaderParent(c.r)
		if ok != c.wantOK || (ok && ppid != c.wantPpid) {
			t.Errorf("%s: LeaderParent(%+v) = (%d, %v), want (%d, %v)", c.name, c.r, ppid, ok, c.wantPpid, c.wantOK)
		}
	}

	// And the decision the backstop makes out of that reading, over the same
	// live process: this test binary IS the leader's parent, so a record
	// naming it as the bwrap reads bounded, and one naming any other pid
	// reads orphaned.
	for _, c := range []struct {
		name     string
		bwrapPid int
		want     bool
	}{
		{"its recorded bwrap is still its parent", me, false},
		{"its recorded bwrap is not its parent any more", cpid, true},
	} {
		r := wallWatchRecord{Seat: "probe", SeatPid: cpid, MntNS: self.Ino, BwrapPid: c.bwrapPid}
		if _, got := wallWatchSeatOrphaned(r, wallWatchProcProbe{}); got != c.want {
			t.Errorf("%s: wallWatchSeatOrphaned = %v, want %v — a true here is a kill (ranger-base-5k7vf)", c.name, got, c.want)
		}
	}
}
