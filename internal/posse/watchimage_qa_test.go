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
	"strings"
	"testing"
	"time"
)

// rrgncCopy writes src's bytes to dst, executable, through a temp file and a
// rename — the swap ADR 0092 D3.5 does.
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
	loop := exec.Command(bin, "60")
	if err := loop.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { loop.Process.Kill(); loop.Wait() })

	a := &App{StateDir: filepath.Join(root, "state")}
	lock, held, err := lockWatch(a)
	if err != nil || held {
		t.Fatalf("lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: loop.Process.Pid, Started: time.Now()}); err != nil {
		t.Fatal(err)
	}

	img, ok := ReadWatchImage(a)
	if !ok || img.Via != "proc" || img.Path != bin {
		t.Fatalf("before the swap the reading must come from /proc and name %s: ok=%v %+v", bin, ok, img)
	}
	if img.Stale() || len(img.GovRows()) != 0 {
		t.Fatalf("a loop running the installed file is not stale: %+v", img)
	}

	// The install.
	rrgncCopy(t, sleep, bin)

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
	fresh := exec.Command(bin, "60")
	if err := fresh.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fresh.Process.Kill(); fresh.Wait() })
	if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: fresh.Process.Pid}); err != nil {
		t.Fatal(err)
	}
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
