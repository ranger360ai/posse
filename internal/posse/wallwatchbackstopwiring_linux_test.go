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
	"path/filepath"
	"strings"
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
	if c != 1 {
		t.Errorf("watcher exit = %d, want 1 (it killed the seat):\n%s", c, stderr.String())
	}
	if _, err := os.Stat(ran); err == nil {
		t.Errorf("the seat's command RAN although its backstop record was never written — the watcher released a seat only it can see:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "cannot write this seat's record for the dispatch backstop") {
		t.Errorf("the watcher did not say why it killed the seat:\n%s", stderr.String())
	}
}
