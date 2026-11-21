//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-rrgnc — G18, the dispatch loop that runs an image
// the install replaced. The acceptance asked for "a pin that starts a watch
// loop, swaps the binary by rename and asserts the condition fires": the loop
// here is a real process exec'd from a real file, holding this home's real
// watch lock and record, and the swap is the rename `posse upgrade` does.

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// rrgncCopy writes src's bytes to dst, executable, through a temp file and a
// rename — the swap ADR 0092 D3.5 does.
// rrgncLoopLife is how long the stand-in loop sleeps, and it is this long for
// a reason that cost a red suite (ranger-base-rph5z's run, 2026-11-21).
//
// It was "60". These tests start `sleep <n>` as a stand-in dispatch loop,
// record its pid, and then read that pid back — and `WatchLoopRunning` plus
// `ReadWatchPid` answer LIVENESS and not identity, which watchpid.go's own
// `pidAlive` comment says in so many words: "a recycled pid is alive and is a
// stranger". internal/posse is one parallel test binary that ran 262.7s on
// the failing pass, and a parallel test can be descheduled for longer than a
// minute on a loaded box. So the sleep EXITED mid-test, its pid was reused by
// another test's re-exec of the test binary, and the reading came back
// pointing at `posse.test` in the go-build cache with `Image == OnDisk` —
// rendering as "this loop is not stale" and failing an assertion about a
// rename that had worked perfectly.
//
// 1500 is the per-package `-timeout` the Makefile passes, so the child cannot
// outlive the budget that would kill the suite anyway, and cannot exit inside
// it. `sleep` burns no CPU, so a leaked one is not what the load guard's kill
// arm is for (AGENTS.md, "Something you MEANT to leave running"); each test
// kills its own child in a t.Cleanup regardless.
const rrgncLoopLife = "1500"

// rrgncStartLoop starts a stand-in dispatch loop from bin and does not return
// until /proc says that pid really runs it.
//
// THE WAIT IS THE POINT, and it is the second half of what cost a red suite.
// `exec.Cmd.Start` returns once the child's exec has been reported, but
// `/proc/<pid>/exe` is not reliably the new image by the time the PARENT is
// next scheduled: MEASURED 2026-11-21, 2 failures in 20 runs of this file's
// own fixtures, each reading the forked-but-not-yet-settled child as the TEST
// BINARY (`/tmp/go-build…/posse.test`) a fifth of a second into the test. The
// same window is why `cmd/checkorphans` refuses to judge a process that is
// not "old enough not to be a fork/exec teardown window" (AGENTS.md) — this
// package had simply never waited for it.
//
// A fixture may re-establish its own precondition, and that is all this does:
// the product code under test is unchanged and is not being accommodated.
// What it must never do is proceed on an unsettled reading, because then the
// assertion below it is about a stranger's image and the failure arrives as a
// verdict about staleness.
func rrgncStartLoop(t *testing.T, bin string) *exec.Cmd {
	t.Helper()
	loop := exec.Command(bin, rrgncLoopLife)
	if err := loop.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { loop.Process.Kill(); loop.Wait() })
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, path, ok := procExe(loop.Process.Pid); ok && path == bin {
			return loop
		}
		if time.Now().After(deadline) {
			_, path, ok := procExe(loop.Process.Pid)
			t.Fatalf("pid %d never settled onto %s (last reading: %q ok=%v) — a fixture that proceeds here asserts against a stranger's image", loop.Process.Pid, bin, path, ok)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// rrgncChildStillOurs fails loudly when the pid a test recorded is no longer
// the child it started. It is the READ-TIME half: rrgncStartLoop settles the
// child before it is recorded, and this says so again before each reading, so
// a child that dies mid-test (or a pid reused after it) arrives as "your
// fixture died" and never as a verdict about staleness.
func rrgncChildStillOurs(t *testing.T, pid int, bin string) {
	t.Helper()
	_, path, ok := procExe(pid)
	if !ok {
		t.Fatalf("pid %d: this fixture's stand-in loop is gone before the reading — the assertions below would be about a stranger", pid)
	}
	if path != bin {
		t.Fatalf("pid %d now runs %s, not this fixture's %s — so every reading below is a stranger's (see rrgncStartLoop and rrgncLoopLife)", pid, path, bin)
	}
}

func rrgncCopy(t *testing.T, src, dst string) {
	t.Helper()
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	// WriteExecutable and not a bare write: a sibling's fork inside the write
	// window would make the exec below answer ETXTBSY (ranger-base-d26ak).
	tmp := dst + ".new"
	if err := WriteExecutable(tmp, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		t.Fatal(err)
	}
}

func TestQAG18FiresWhenTheInstallRenamesOverALiveLoop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the /proc road; the record road is pinned below on every OS")
	}
	t.Parallel()
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep(1) to stand in for the loop's image")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "posse")
	rrgncCopy(t, sleep, bin)

	// The loop: a process running the installed file. It stamps nothing —
	// like every loop alive on the day this lands, started by a binary that
	// predates the record fields — so /proc is all G18 has.
	loop := rrgncStartLoop(t, bin)

	a := &App{StateDir: filepath.Join(root, "state")}
	lock, held, err := lockWatch(a)
	if err != nil || held {
		t.Fatalf("lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: loop.Process.Pid, Started: time.Now()}); err != nil {
		t.Fatal(err)
	}

	rrgncChildStillOurs(t, loop.Process.Pid, bin)
	img, ok := ReadWatchImage(a)
	if !ok || img.Via != "proc" || img.Path != bin {
		t.Fatalf("before the swap the reading must come from /proc and name %s: ok=%v %+v", bin, ok, img)
	}
	if img.Stale() || len(img.GovRows()) != 0 {
		t.Fatalf("a loop running the installed file is not stale: %+v", img)
	}

	// The install.
	rrgncCopy(t, sleep, bin)

	rrgncChildStillOurs(t, loop.Process.Pid, bin)
	img, ok = ReadWatchImage(a)
	if !ok || !img.Stale() {
		t.Fatalf("after a rename over the loop's path G18 must fire: ok=%v %+v", ok, img)
	}
	rows := img.GovRows()
	if len(rows) != 1 || rows[0].ID != "G18" || rows[0].Class != GovLane {
		t.Fatalf("want one LANE G18 row, got %+v", rows)
	}
	for _, want := range []string{"dispatcher pid", "restart the dispatch loop, then relaunch seats", "guardrail 3"} {
		if !strings.Contains(rows[0].Detail, want) {
			t.Errorf("G18 detail is missing %q: %s", want, rows[0].Detail)
		}
	}

	// And the governance surface carries it, through the seam the row is
	// wired with — not just the reading.
	// govIn's hermetic shop, with an all-clear control: the fixture CAN print
	// "nothing needs a human", and does until the reading is stale.
	b, _ := newTestBackend(t)
	clearIn, staleIn := govIn(t, b), govIn(t, b)
	cur := img
	cur.OnDisk = cur.Image
	clearIn.Image = func() (WatchImage, bool) { return cur, true }
	staleIn.Image = func() (WatchImage, bool) { return img, true }
	var clear, stale bytes.Buffer
	GovReport(&clear, shopSet(t, clearIn), nil)
	GovReport(&stale, shopSet(t, staleIn), nil)
	if !strings.Contains(clear.String(), "nothing needs a human") {
		t.Fatalf("the control arm raised something else — this fixture cannot state the claim:\n%s", clear.String())
	}
	if got := stale.String(); strings.Contains(got, "nothing needs a human") || !strings.Contains(got, "G18") {
		t.Errorf("a loop on a replaced image printed no G18 row — this IS the bead:\n%s", got)
	}

	// The negative control on the same loop: a restart onto the new file
	// clears it. (Here: the record names a pid whose image IS the new file.)
	fresh := rrgncStartLoop(t, bin)
	if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: fresh.Process.Pid}); err != nil {
		t.Fatal(err)
	}
	rrgncChildStillOurs(t, fresh.Process.Pid, bin)
	if img, ok := ReadWatchImage(a); !ok || img.Stale() {
		t.Errorf("a loop restarted onto the installed file must clear G18: ok=%v %+v", ok, img)
	}
}

// Where /proc cannot answer (darwin, another pid namespace), the loop's own
// stamp is the reading — and a loop too old to have stamped is an abstention.
func TestQAG18ReadsTheLoopsOwnStampWhereProcCannotAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bin := filepath.Join(root, "posse")
	if err := os.WriteFile(bin, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	noProc := func(int) (string, string, bool) { return "", "", false }
	rec := &WatchPid{Pid: 4242, Exe: bin, Image: fileIdentity(bin), Version: "0.5.3+aaaaaaaa"}
	if rec.Image == "" {
		t.Skip("this platform's stat carries no inode")
	}

	if img := readWatchImage(rec, noProc); img.Via != "record" || img.Stale() {
		t.Fatalf("the stamped image is still installed: %+v", img)
	}
	tmp := bin + ".new"
	if err := os.WriteFile(tmp, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, bin); err != nil {
		t.Fatal(err)
	}
	img := readWatchImage(rec, noProc)
	if !img.Stale() || !strings.Contains(img.Sentence(), "posse 0.5.3+aaaaaaaa") {
		t.Fatalf("after the rename the stamp must read stale and name its version: %+v / %s", img, img.Sentence())
	}

	old := &WatchPid{Pid: 4242}
	if img := readWatchImage(old, noProc); img.Stale() || len(img.GovRows()) != 0 {
		t.Errorf("an unstamped loop with no /proc is an abstention, not a row: %+v", img)
	}
}

// The record round-trips the three stamp fields, and the stamp a starting
// loop writes names this process's own image.
func TestQAWatchPidCarriesTheLoopsImage(t *testing.T) {
	t.Parallel()
	a := &App{StateDir: filepath.Join(t.TempDir(), "state")}
	d := &Dispatcher{App: a, Out: io.Discard, Err: io.Discard}
	d.stampWatchPid()
	rec, ok := ReadWatchPid(WatchPidPath(a))
	if !ok {
		t.Fatal("no record")
	}
	id, path := selfImage()
	if rec.Image != id || rec.Exe != path || rec.Version != VersionString() {
		t.Errorf("stamp = exe %q image %q version %q; want %q %q %q", rec.Exe, rec.Image, rec.Version, path, id, VersionString())
	}
	if id == "" {
		t.Errorf("this process could not name its own image")
	}
}

// D6's line before the last one: it names the stale loop, and it is absent
// when the plan carries no reader (every pin that does not ask).
func TestQAUpgradeNamesTheLoopTheRenameLeftBehind(t *testing.T) {
	t.Parallel()
	if got := upgradeWatchLine(upgradePlan{}); got != "" {
		t.Errorf("no reader must print nothing, got %q", got)
	}
	stale := WatchImage{Pid: 77, Path: "/x/posse", Image: "1:2", OnDisk: "1:3", Version: "0.5.3+old"}
	got := upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return stale, true }})
	for _, want := range []string{"dispatch: dispatcher pid 77 runs posse 0.5.3+old", "restart the dispatch loop, then relaunch seats"} {
		if !strings.Contains(got, want) {
			t.Errorf("line is missing %q: %s", want, got)
		}
	}
	cur := WatchImage{Pid: 77, Path: "/x/posse", Image: "1:3", OnDisk: "1:3"}
	if got := upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return cur, true }}); !strings.Contains(got, "already runs") {
		t.Errorf("a current loop must say so: %s", got)
	}
}

// ─── the reading's scope (ranger-base-nqaa1, finding 1) ──────────────────────

// EVERY rendering of a loop reading says it read one instance's loop, and
// names which. The verdict branches are not the interesting ones: a stale
// loop is already being told to restart, so the clause only sharpens it.
// What this pin is for is the other two — "already runs" and "could not be
// read" — because those are the lines an operator finishes believing the BOX
// is covered, and the binary the rename replaced is the box's.
func TestQAEveryLoopReadingSaysWhichInstanceItRead(t *testing.T) {
	t.Parallel()
	const home = "/home/posse/homes/ranger"
	// The clause has to name the instance and has to say the subject is
	// wider than the reading. "loops" plural is ADR 0092 D6's own word for
	// the remedy, and the whole finding is that the plural was being read as
	// satisfied by a singular reading.
	says := func(t *testing.T, what, line string) {
		t.Helper()
		for _, want := range []string{"scope: one instance's loop", home, "box-wide", "restart the dispatch loops", "sibling instances this reading did not look at"} {
			if !strings.Contains(line, want) {
				t.Errorf("%s does not say %q:\n  %s", what, want, line)
			}
		}
	}

	stale := WatchImage{Pid: 77, Path: "/x/posse", Image: "1:2", OnDisk: "1:3", Version: "0.5.3+old", Home: home}
	says(t, "the G18 sentence", stale.Sentence())
	rows := stale.GovRows()
	if len(rows) != 1 {
		t.Fatalf("want one G18 row, got %+v", rows)
	}
	says(t, "the G18 row's detail", rows[0].Detail)
	says(t, "the verb's line over a stale loop", upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return stale, true }}))

	cur := stale
	cur.OnDisk = cur.Image
	says(t, `the verb's "already runs" line`, upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return cur, true }}))

	// The abstention: ok=false, and the reading still names its instance —
	// which is the one line where "this home" was all the reader had.
	abstain := WatchImage{Home: home}
	says(t, "the verb's abstention line", upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return abstain, false }}))

	// A reader that could not name the instance says so instead of naming
	// nothing: the clause is never silently absent.
	anon := upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return WatchImage{}, false }})
	if !strings.Contains(anon, "scope: one instance's loop — this instance's") {
		t.Errorf("a reading with no instance name still has to carry the clause:\n  %s", anon)
	}
}

// TWO INSTANCES, TWO LOOPS, ONE BINARY — the box this finding was measured
// on, as a fixture. MEASURED 2026-11-21: pid 2066610 under ~/homes/ranger and
// pid 1991667 under ~/homes/weller, both exec'ing ~/.local/bin/posse at
// dev:ino 54:12591441.
//
// What it pins is the SHAPE and not a widening: each instance's reading names
// its own loop and nothing of the other's, so an install's report and that
// instance's G18 row cover one of the two loops the rename left behind. The
// clause above is what carries that to the reader. Widening the reading needs
// a notion of "the instances on this box", which exists nowhere in this tree.
func TestQAALoopReadingCoversItsOwnInstanceAndNotItsSiblings(t *testing.T) {
	// FIRST, before the GOOS skip, which is the order `make verify-parallel`
	// reads: it takes the body's first statement and nothing else, and this
	// test is env-clean and writes no package var, so it is eligible.
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("the /proc road; the record road is pinned above on every OS")
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep(1) to stand in for a loop's image")
	}
	root := t.TempDir()
	// ONE binary, as the box has one: both loops exec this file and the
	// rename below replaces it under both of them.
	bin := filepath.Join(root, "bin", "posse")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	rrgncCopy(t, sleep, bin)

	type instance struct {
		app  *App
		pid  int
		home string
	}
	var insts []instance
	for _, name := range []string{"ranger", "weller"} {
		home := filepath.Join(root, "homes", name)
		loop := rrgncStartLoop(t, bin)

		a := &App{Home: home, StateDir: filepath.Join(home, "state")}
		lock, held, err := lockWatch(a)
		if err != nil || held {
			t.Fatalf("%s: lock held=%v err=%v — each instance holds its OWN watch lock", name, held, err)
		}
		defer lock.Release()
		if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: loop.Process.Pid, Started: time.Now()}); err != nil {
			t.Fatal(err)
		}
		insts = append(insts, instance{app: a, pid: loop.Process.Pid, home: home})
	}

	// The install: one rename, both loops left on the old inode.
	rrgncCopy(t, sleep, bin)

	for i, in := range insts {
		other := insts[(i+1)%len(insts)]
		rrgncChildStillOurs(t, in.pid, bin)
		img, ok := ReadWatchImage(in.app)
		if !ok || !img.Stale() {
			t.Fatalf("%s: after the rename this instance's loop is stale: ok=%v %+v", in.home, ok, img)
		}
		if img.Pid != in.pid {
			t.Errorf("%s: the reading names pid %d, want this instance's %d", in.home, img.Pid, in.pid)
		}
		if img.Home != in.home {
			t.Errorf("%s: the reading says its instance is %q", in.home, img.Home)
		}
		rows := img.GovRows()
		if len(rows) != 1 || rows[0].ID != "G18" {
			t.Fatalf("%s: want one G18 row, got %+v", in.home, rows)
		}
		// The claim, both halves: the row is about THIS loop, and it says
		// so rather than leaving the reader to assume the box.
		if strings.Contains(rows[0].Detail, strconv.Itoa(other.pid)) || strings.Contains(rows[0].Detail, other.home) {
			t.Errorf("%s: this instance's G18 row names the sibling's loop — the reading is per-instance and the row must not claim otherwise:\n  %s", in.home, rows[0].Detail)
		}
		if !strings.Contains(rows[0].Detail, "scope: one instance's loop — "+in.home) {
			t.Errorf("%s: the row does not say whose loop it read:\n  %s", in.home, rows[0].Detail)
		}
	}

	// And the sibling's staleness is real, not an artifact of reading it
	// through the first instance's App: the box genuinely has two stale
	// loops after one rename, which is the whole finding.
	a, b := insts[0], insts[1]
	ia, _ := ReadWatchImage(a.app)
	ib, _ := ReadWatchImage(b.app)
	if ia.Pid == ib.Pid {
		t.Fatalf("the fixture read the same loop twice: %d", ia.Pid)
	}
	if !ia.Stale() || !ib.Stale() {
		t.Errorf("one rename left both loops stale; got %+v and %+v", ia, ib)
	}
}
