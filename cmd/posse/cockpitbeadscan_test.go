package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/rhq"
)

// ranger-base-txio: the two bd scans behind IN PROGRESS and READY WORK cost
// 5.3s EACH on the shop this was filed from, and refresh() ran both of them
// synchronously off a 2s ticker — so the tick was always already ready when
// refresh returned, the loop never idled, a keystroke waited up to ten
// seconds behind a scan, and the box carried two `bd list` processes
// forever. These pin both halves: the scan is off the event loop, and its
// cadence is bounded by its own cost rather than running back to back.

// beadScanRig builds a cockpit over stub herdr and bd binaries. bdDelay is
// how long each bd call sleeps; every call appends a line to the returned
// counter file, so a test can ask how many subprocesses a path actually
// spent rather than trusting that it spent none.
func beadScanRig(t *testing.T, bdDelay time.Duration, bdOut string) (*cockpit, string) {
	t.Helper()
	home, binDir := t.TempDir(), t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	herdr := filepath.Join(binDir, "herdr")
	if err := os.WriteFile(herdr, []byte(`#!/bin/sh
if [ "$1" = "workspace" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"workspaces":[]}}'
  exit 0
fi
if [ "$1" = "agent" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"result":{"agents":[]}}'
  exit 0
fi
printf '%s\n' '{"error":{"code":"no","message":"unexpected"}}'
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(home, "bd-calls")
	sleep := ""
	if bdDelay > 0 {
		sleep = "sleep " + bdDelay.Truncate(time.Millisecond).String() + "\n"
	}
	// Verb-aware: `bd ready` answers with bdOut and `bd list --status
	// in_progress` with nothing. One canned answer for both would put every
	// ready bead in IN PROGRESS as well, and readyOnly would then empty
	// READY WORK — a rig that looks like the bug it is meant to measure.
	bd := filepath.Join(binDir, "bd")
	if err := os.WriteFile(bd, []byte("#!/bin/sh\necho \"$@\" >> "+calls+"\n"+sleep+
		"if [ \"$1\" = ready ]; then cat <<'JSON'\n"+bdOut+"\nJSON\nelse echo '[]'; fi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// ServerGen() stats the operator's live herdr socket unless this points
	// it somewhere else (ranger-base-ouf9): a test must not read the box.
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(home, "no-such.sock"))
	a := &rhq.App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
	}
	c := &cockpit{
		app:   a,
		hb:    &rhq.HerdrBackend{App: a, H: rhq.Herdr{Bin: herdr}, Warn: io.Discard},
		bd:    rhq.Bd{Bin: bd},
		beads: make(chan beadRead, 1),
	}
	return c, calls
}

func bdCalls(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(strings.Split(strings.TrimSpace(string(b)), "\n"))
}

const twoReady = `[{"id":"x-1","title":"one","status":"open"},{"id":"x-2","title":"two","status":"open"}]`

// The defect in one assertion: refresh() must not sit inside the bd scans,
// and a keystroke must not queue behind them.
func TestCockpitRefreshDoesNotBlockOnTheBeadScan(t *testing.T) {
	// 3s per bd call: a synchronous refresh costs 6s here, and the herdr
	// stub the split DOES still pay costs well under one. The threshold
	// sits between them rather than near zero, so a slow box cannot turn
	// this into a flake.
	const bdDelay = 3 * time.Second
	c, calls := beadScanRig(t, bdDelay, twoReady)

	start := time.Now()
	c.refresh()
	if d := time.Since(start); d >= bdDelay {
		t.Fatalf("refresh sat inside the bd scans for %v — it must start them, not run them", d)
	}
	// Consequence 1 of the bead: the event loop can still reach the keys.
	start = time.Now()
	if _, err := c.handleKey([]byte("j")); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d >= bdDelay {
		t.Errorf("a keystroke waited %v behind the scan", d)
	}

	select {
	case r := <-c.beads:
		c.applyBeads(r)
	case <-time.After(20 * time.Second):
		t.Fatal("the scan never landed on c.beads")
	}
	if n := bdCalls(t, calls); n != 2 {
		t.Errorf("one scan is one InProgressAll and one ReadyAll: got %d bd calls", n)
	}
	if len(c.issues) != 2 {
		t.Errorf("the landed scan must fill READY WORK, got %+v", c.issues)
	}
	if c.beadsIn {
		t.Error("a landed scan must clear the in-flight guard")
	}
}

// The cadence floor is the scan's OWN cost, never a flat constant: a store
// where the pair costs 10.6s cannot be asked again every 5s without being
// the same back-to-back storm one goroutine further out, and a store where
// it costs 100ms must not be held to 10.6s.
func TestCockpitBeadCadenceFloorIsTheScansOwnCost(t *testing.T) {
	c := &cockpit{beads: make(chan beadRead, 1)}

	c.applyBeads(beadRead{took: 30 * time.Second})
	if gap := c.beadsNext.Sub(c.beadsAt); gap != 30*time.Second {
		t.Errorf("an expensive scan buys a %v gap, want its own 30s", gap)
	}
	c.applyBeads(beadRead{took: 100 * time.Millisecond})
	if gap := c.beadsNext.Sub(c.beadsAt); gap != beadsEvery {
		t.Errorf("a cheap scan buys a %v gap, want the %v floor", gap, beadsEvery)
	}
}

// The floor is a ticker's business only. An operator action — c, u, x, o, r,
// or a dispatch that landed — goes through refresh() and rescans at once,
// because it is the thing that just changed those lists.
func TestCockpitBeadKickHonoursTheFloorButNotTheOperator(t *testing.T) {
	c, calls := beadScanRig(t, 0, "[]")
	c.beadsNext = time.Now().Add(time.Hour)

	c.kickBeads(false)
	if c.beadsIn || bdCalls(t, calls) != 0 {
		t.Fatalf("a ticker kick inside the floor must not scan: inFlight=%v calls=%d", c.beadsIn, bdCalls(t, calls))
	}
	c.kickBeads(true)
	if !c.beadsIn {
		t.Fatal("an operator action must rescan through the floor")
	}
	// One at a time, forced or not: a second scan while one is out is the
	// storm again, and its goroutine would block forever on a full channel.
	c.kickBeads(true)
	<-c.beads
	if n := bdCalls(t, calls); n != 2 {
		t.Errorf("%d bd calls — a forced kick must still refuse to start a second scan", n)
	}
}

// The non-tty loop has no keystrokes to starve, so it pays the scan inline
// and its first frame is complete — but it keeps the floor, which is the
// half of this bead that is about the box's load rather than latency.
func TestCockpitDisplayFrameIsCompleteThenRateLimited(t *testing.T) {
	c, calls := beadScanRig(t, 0, twoReady)
	c.out = io.Discard

	c.displayFrame()
	if len(c.issues) != 2 {
		t.Fatalf("the first non-tty frame must already carry the beads, got %+v", c.issues)
	}
	if n := bdCalls(t, calls); n != 2 {
		t.Fatalf("first frame: %d bd calls, want 2", n)
	}
	c.displayFrame()
	c.displayFrame()
	if n := bdCalls(t, calls); n != 2 {
		t.Errorf("%d bd calls over three frames — the 2s frame loop is rescanning inside the %v floor", n, beadsEvery)
	}
}

// Until the first scan lands, both bead sections are empty because nothing
// has looked yet — not because there is nothing there. `(none)` under a
// `READY WORK (0)` is an answer an operator acts on, and for the first ten
// seconds of a cockpit on a big store it would be one nobody had measured.
func TestCockpitBeadHeadingsSayScanningUntilTheFirstScanLands(t *testing.T) {
	c := &cockpit{beads: make(chan beadRead, 1)}
	headings := func() string {
		c.buildRows()
		var b strings.Builder
		for _, r := range c.rows {
			if r.kind == rowHeading {
				b.WriteString(r.cols[0].text + "\n")
			}
		}
		return b.String()
	}

	c.beadsIn = true // the first scan is out
	got := headings()
	for _, want := range []string{"IN PROGRESS (scanning…)", "READY WORK (scanning…)"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q while the first scan is out:\n%s", want, got)
		}
	}
	// A shop that really is empty says so, the moment any scan has landed.
	c.applyBeads(beadRead{})
	got = headings()
	for _, want := range []string{"IN PROGRESS (0)", "READY WORK (0)"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q once a scan has landed:\n%s", want, got)
		}
	}
	if strings.Contains(got, "scanning") {
		t.Errorf("a landed scan is not a scanning one:\n%s", got)
	}
}

// rangerhq-llse across the split: the ready-scan failure and the herdr
// failure that outranks it no longer land in the same call, so the fact that
// herdr is down has to survive from one to the other.
func TestCockpitHerdrDownStillOutranksAReadyScanFailure(t *testing.T) {
	c, _ := beadScanRig(t, 0, "[]")
	c.hb = &rhq.HerdrBackend{App: c.app, H: rhq.Herdr{Bin: "/nonexistent/herdr"}, Warn: io.Discard}
	c.bd = rhq.Bd{Bin: "/nonexistent/bd"}

	c.refreshSessions()
	if !c.herdrDown {
		t.Fatal("a failed session read must record that herdr is down")
	}
	down := c.status
	if down == "" {
		t.Fatal("a failed session read must reach the status line")
	}
	c.applyBeads(beadRead{failed: []error{rhq.ScanError{Dir: "/repo", Err: rhq.Die("database is locked")}}})
	if c.status != down {
		t.Errorf("a ready-scan failure took the line from a down herdr: %q", c.status)
	}
	// The control: herdr up, and the ready-scan failure is what the
	// operator is told.
	c.herdrDown, c.status = false, ""
	c.applyBeads(beadRead{failed: []error{rhq.ScanError{Dir: "/repo", Err: rhq.Die("database is locked")}}})
	if !strings.Contains(c.status, "ready scan failed") || !strings.Contains(c.status, "database is locked") {
		t.Errorf("status must name the failed scan, got %q", c.status)
	}
}
