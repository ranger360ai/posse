//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-slece: the three links of ranger-base-zd4d4's backstop
// chain (ranger-base-0qpsw D3) that every other pin stepped around. The diff
// and its responses are pinned in wallwatch_test.go and
// wallwatchbackstoplive_linux_test.go, all of them by calling the lower-case
// wallWatchBackstop directly, so none of them could see:
//
//  1. the dispatch guard tick never calling the backstop at all;
//  2. the exported wrapper handing it the wrong state dir, which makes every
//     tick glob nothing and stay silent forever;
//  3. the watcher releasing a seat whose backstop record it could not write,
//     where only its own (killable) process can see it.
//
// The first two are read through d.guardTick() itself, over the App's own
// StateDir, with the production probe: nothing between the clock and the
// glob is substituted. The third is a live bwrap seat (skips without one).
//
// `_linux_test.go` for wallwatchbackstoplive_linux_test.go's reason: the
// backstop and runWallWatch are Linux-only, and the land gate vets darwin.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Links 1 and 2. One record whose seat is over (a mount namespace no process
// has) and whose watcher is not the process that wrote it: the backstop's
// answer is to drop the record, so the record's absence after ONE guard tick
// says the tick ran the backstop over THIS App's state dir. A second record
// that is not a record says the tick prints what the backstop returns.
func TestWallWatchBackstopRidesTheGuardTickOverTheAppsStateDir(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.App.Load1 = func() (float64, error) { return 0.1, nil }
	d.App.TopCPU = func() ([]Proc, error) { return nil, nil }
	out, ok := d.Out.(*strings.Builder)
	if !ok {
		t.Fatalf("newTestDispatcher's Out is %T, want *strings.Builder", d.Out)
	}

	dir := filepath.Join(d.App.StateDir, "gates", "seat-persona", "wall-watch")
	ended := filepath.Join(dir, "4242.json")
	if err := writeWallWatchRecord(ended, wallWatchRecord{
		Seat: "seat-ended", Dir: d.App.StateDir,
		// Our own pid under a start time that is not ours: a watcher pid
		// recycled into a different process, which is "gone".
		WatcherPid: os.Getpid(), WatcherStart: "not-this-process",
		BwrapPid: 1, SeatPid: 1,
		MntNS:    1, // no process lives in mount namespace inode 1
		Released: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if m, _ := filepath.Glob(wallWatchRecordGlob(d.App.StateDir)); len(m) != 1 {
		t.Fatalf("fixture: the record glob over the App's state dir finds %v, want the one record", m)
	}
	garbage := filepath.Join(dir, "4343.json")
	if err := os.WriteFile(garbage, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	d.guardTick()

	if _, err := os.Stat(ended); !os.IsNotExist(err) {
		t.Errorf("one guard tick left %s in place (stat: %v): the tick did not run the backstop over the App's state dir — check guardTick's call and the arguments App.WallWatchBackstop binds", ended, err)
	}
	if got := out.String(); !strings.Contains(got, "⛨ wall-watch backstop:") || !strings.Contains(got, "4343.json") {
		t.Errorf("the guard tick printed %q, want the backstop's line about the unreadable record 4343.json", got)
	}
}

// Link 3. The watcher cannot write its backstop record (the record dir is a
// FILE), so it must kill the seat before its command runs — fail closed —
// rather than release a seat the dispatch tick cannot find.
func TestLiveWallWatchKillsASeatWhoseBackstopRecordCannotBeWritten(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	if err := os.WriteFile(wallWatchRecordDir(s.mounts), []byte("not a dir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	carve := SeatbeltCarveOut{
		Deny:     []string{s.deny},
		DenyFile: []string{s.deny},
		Private:  []PrivateBind{{Src: s.privSrc, SrcRoot: filepath.Dir(s.privSrc), Dst: s.priv}},
	}
	argv, prep, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	// The launcher's own half, run as the launcher runs it. It used to
	// assert the render needed NO prep, which held only while the fixture
	// happened to have made everything the render wanted: ranger-base-ua625
	// gave the private bind's empty mount point an anchor seed and the
	// tripwire fired on a step a real launch makes without comment. A
	// fixture that refuses the launcher's prep is a fixture measuring a
	// wall no launch puts up.
	if err := runBwrapPrep(prep); err != nil {
		t.Fatalf("prep: %v", err)
	}
	if err := os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644); err != nil {
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
	devnull, _ := os.Open(os.DevNull)
	defer devnull.Close()
	ran := filepath.Join(s.grant, "ran")
	var stderr strings.Builder
	code := make(chan int, 1)
	go func() {
		code <- runWallWatch(wallWatchRun{
			Opts: wallWatchOpts{Mounts: s.mounts, Seat: "seat-norecord", Dir: s.grant,
				Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", "echo ran > " + shellQuote(ran)}},
			Args: af, Stdin: devnull, Stdout: &stderr, Stderr: &stderr,
			Store: &fakeWallWatchStore{}, Lock: filepath.Join(t.TempDir(), "wall-watch.lock"),
			PollInterval: 50 * time.Millisecond,
		})
	}()
	var c int
	select {
	case c = <-code:
	case <-time.After(10 * time.Second):
		t.Fatalf("the watcher has not returned 10s after a record write that cannot succeed:\n%s", stderr.String())
	}
	// A seat the box ends is silent and exits 128+9, and this `want 1` is
	// the watcher's own refusal code — so an unrouted read turns that death
	// into a wall regression ADR 0088 D5's ratchet charges to whoever landed
	// last (ranger-base-jqdm2).
	wwAbstainIfOutside(t, "the seat whose backstop record cannot be written", c, stderr.String())
	if c != 1 {
		t.Errorf("watcher exit = %d, want 1 (it killed the seat): %s", c, wwOneLine(stderr.String()))
	}
	if _, err := os.Stat(ran); err == nil {
		t.Errorf("the seat's command RAN although its backstop record was never written — the watcher released a seat only it can see:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "cannot write this seat's record for the dispatch backstop") {
		t.Errorf("the watcher did not say why it killed the seat:\n%s", stderr.String())
	}
}

// LINK 2, THE OTHER FOUR VALUES (ranger-base-a9gbq, out of ranger-base-0a1jk
// finding 4). The pin above covers the state dir and only the state dir: its
// record's seat is OVER, so the backstop drops the record and returns before
// any of `st`, `lock` or `now()` is read. Three mutants therefore survived
// the whole WallWatch family — `go test ./internal/posse -run WallWatch`, ok
// 1.79s each over 29 pins (MEASURED 2026-11-14 at 3d4703e7):
//
//	(a) wallWatchLockPath()            -> wallWatchLockPath() + ".mutant"
//	(b) time.Now                       -> func() time.Time { return time.Time{} }
//	(c) guardclock.go's d.Bd           -> nil
//
// and there is nowhere else to read them: the exported wrapper has exactly
// one caller in the tree (guardclock.go) and no test calls it directly. The
// probe is the fifth value and is not mutable in-tree — wallWatchProcProbe is
// its only implementation outside a _test.go — so the wrapper was 1 of 5
// pinned rather than 1 of 4.
//
// WHAT THIS FIXTURE NEEDS, and why it is the shape it is. Reaching
// fileWallWatchDetach — the one place all three are read — means reaching the
// detach branch, and the detach branch KILLS the seat with the production
// probe. `wallWatchKillWith` SIGKILLs every process in the record's mount
// namespace, round after round, so a record naming this host's namespace
// would end this box. So the fixture makes a mount namespace of its own and
// puts exactly one expendable process in it: `sleep`, started under
// CLONE_NEWUSER|CLONE_NEWNS, which needs no privilege and no bwrap. The
// namespace is asserted DISTINCT from this process's before the record is
// written, and that assertion is load-bearing rather than decorative — a
// clone that quietly failed would leave the record naming the host namespace
// and the kill would take the suite, the session and the box with it.
//
// The walls come off without touching a mount: wallWatchDetached compares the
// record's `Base` mount ids against the table it reads, so a Base naming an id
// that namespace has never had IS a detached wall.
//
// All three values are read through d.guardTick(), which is the production
// call site, so (c) is the argument guardclock.go passes and not one this
// test chose.
//
// MUTATION-CHECKED 2026-11-14 at 1d03f9d1 on linux, all three mutants above,
// each red with its own message and reverted with md5 taken on both sides:
// (a) `nothing at <lock> after the tick`; (b) `the bead says the wall came
// off at 0001-01-01T00:00:00Z`; (c) a nil-pointer panic in the tick, which is
// what a real detach would do in production. Each was ok over `-run
// WallWatch` before this pin existed.
func TestWallWatchBackstopWrapperBindsTheStoreTheClockAndTheLockPath(t *testing.T) {
	// Not parallel: t.Setenv. XDG_CACHE_HOME is what makes the production
	// wallWatchLockPath() a path this test owns, so that its existence
	// afterwards is this tick's work and not another test's leftover.
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	lock := wallWatchLockPath()
	if !strings.HasPrefix(lock, cache) {
		t.Fatalf("wallWatchLockPath() is %q, want a path under this test's XDG_CACHE_HOME %q — "+
			"without that the assertion below cannot tell this tick's lock from another test's", lock, cache)
	}

	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.App.Load1 = func() (float64, error) { return 0.1, nil }
	d.App.TopCPU = func() ([]Proc, error) { return nil, nil }

	// One expendable process in a mount namespace of its own.
	sleeper := exec.Command("sleep", "300")
	sleeper.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	if err := sleeper.Start(); err != nil {
		t.Skipf("this box will not give an unprivileged mount namespace (%v): the detach branch kills every "+
			"process in the record's namespace, so there is no namespace this test may name", err)
	}
	reaped := make(chan struct{})
	go func() { sleeper.Wait(); close(reaped) }()
	pid := sleeper.Process.Pid
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL); <-reaped })

	var ns, self syscall.Stat_t
	for deadline := time.Now().Add(5 * time.Second); ; {
		if err := syscall.Stat("/proc/"+strconv.Itoa(pid)+"/ns/mnt", &ns); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("the sleeper's mount namespace never appeared: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Stat("/proc/self/ns/mnt", &self); err != nil {
		t.Fatalf("stat this process's own mount namespace: %v", err)
	}
	// THE SAFETY ASSERTION. Everything below hands this inode to a probe
	// whose Kill ends every process in it.
	if ns.Ino == self.Ino || ns.Ino == 0 {
		t.Fatalf("the sleeper shares this process's mount namespace (inode %d): the clone did not take, and a "+
			"record naming it would have the backstop SIGKILL this whole box", ns.Ino)
	}
	if pids := (wallWatchProcProbe{}).SeatPids(wallWatchRecord{MntNS: ns.Ino}); len(pids) != 1 || pids[0] != pid {
		t.Fatalf("the production probe reads %v in the sleeper's namespace, want exactly [%d] — a namespace "+
			"holding anything else is not a namespace this test may hand a kill", pids, pid)
	}

	// A record whose seat is LIVE and whose wall is off: the branch that
	// reads the store, the clock and the lock.
	const wall = "/posse-wall-that-was-never-mounted"
	rec := filepath.Join(d.App.StateDir, "gates", "seat-wiring", "wall-watch", strconv.Itoa(pid)+".json")
	if err := writeWallWatchRecord(rec, wallWatchRecord{
		Seat: "seat-wiring", Dir: d.App.StateDir,
		// Our own pid under a start time that is not ours, as the pin above:
		// a watcher pid recycled into another process, which is "gone".
		WatcherPid: os.Getpid(), WatcherStart: "not-this-process",
		BwrapPid: pid, SeatPid: pid,
		MntNS: ns.Ino,
		Watch: []WallWatchMount{{Kind: WallWatchDeny, Path: wall}},
		// An id this namespace's table has never carried, so the wall at
		// that path reads as gone without anything being mounted.
		Base:     map[string][]string{wall: {"mount-id-that-never-was"}},
		Released: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	before := time.Now()
	d.guardTick()

	// (c) THE STORE, and it is guardclock.go's argument: the create went to
	// the dispatcher's own Bd. A nil store does not reach the fake bd at all.
	log, err := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if err != nil {
		t.Fatalf("the tick filed nothing through the store the wrapper was handed (bd-calls.log: %v) — "+
			"check the store argument guardclock.go passes App.WallWatchBackstop", err)
	}
	if !strings.Contains(string(log), "create security: wall-watch killed seat seat-wiring") {
		t.Errorf("bd-calls.log holds no create for this detach — the store the wrapper was handed is not the "+
			"dispatcher's:\n%s", log)
	}

	// (b) THE CLOCK. The detach time in the bead is this tick's, so a clock
	// bound to anything constant reads here.
	stamp := ""
	for _, line := range strings.Split(string(log), "\n") {
		if i := strings.Index(line, "detached at: "); i >= 0 {
			if f := strings.Fields(line[i+len("detached at: "):]); len(f) > 0 {
				stamp = f[0]
			}
			break
		}
	}
	if stamp == "" {
		t.Fatalf("the filed bead carries no `detached at:` stamp, so the clock the wrapper bound is unobservable:\n%s", log)
	}
	at, perr := time.Parse(time.RFC3339Nano, stamp)
	if perr != nil {
		t.Fatalf("`detached at: %s` does not parse: %v", stamp, perr)
	}
	if at.Before(before.Add(-time.Minute)) || at.After(time.Now().Add(time.Minute)) {
		t.Errorf("the bead says the wall came off at %s and this tick ran at %s — the wrapper bound a clock that "+
			"is not the real one", at.UTC().Format(time.RFC3339Nano), before.UTC().Format(time.RFC3339Nano))
	}

	// (a) THE LOCK PATH, which is the one value with no other trace: the
	// production path is the file fileWallWatchDetach opens and flocks.
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("nothing at %s after the tick (stat: %v) — the wrapper bound a lock path that is not "+
			"wallWatchLockPath(), so two ticks on one box do not serialize against each other", lock, err)
	}

	// And the seat really was killed, which is what says the detach branch
	// ran rather than some cheaper path reaching the store.
	select {
	case <-reaped:
	case <-time.After(10 * time.Second):
		t.Errorf("the seat's process %d is still alive after a detach — the tick did not kill it", pid)
	}
	if _, err := os.Stat(rec); !os.IsNotExist(err) {
		t.Errorf("the dead watcher's record survived its seat's end (stat %s: %v)", rec, err)
	}
}
