package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// ranger-base-325q. The cockpit was measured at 101.9% of a core while
// sitting idle on a loaded box, and the mechanism is this: each of the three
// off-loop scans ran from a ticker that fired a fresh goroutine
// unconditionally. The governance check alone takes 23.5s of its 30s budget
// on an IDLE box (measured 2026-08-29 on this shop), so any load at all puts
// a tick inside the previous scan — and the loop started a second one over
// the top of it. Two contend, both get slower, the next tick starts a third.
//
// These tests pin the two halves of the fix: a scan already running is not
// started again, and the flag that says so is cleared by the apply — because
// a guard that is never released is a scan that stops forever, which would
// read as a frozen governance block rather than as a bug.

// ARM A — a tick arriving while the scan is still running starts nothing.
func TestCockpitScanTickDroppedWhileInFlight(t *testing.T) {
	c := &cockpit{}
	var started atomic.Int32
	release := make(chan struct{})
	scan := func() {
		started.Add(1)
		<-release
	}

	c.start(&c.costBusy, scan)
	for i := 0; i < 5; i++ { // five more ticks while the first is in flight
		c.start(&c.costBusy, scan)
	}
	// Wait for the WRONG answer, not the right one: an unguarded start leaves
	// five goroutines runnable and reading the counter too early would call
	// the runaway a pass. Give them a second to show up.
	deadline := time.Now().Add(time.Second)
	for started.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n := started.Load(); n != 1 {
		t.Fatalf("six ticks over one in-flight scan started %d scans, want 1 — this is the runaway", n)
	}
	if !c.costBusy {
		t.Errorf("the guard cleared itself with the scan still running")
	}
	close(release)
}

// ARM B — the guard is released by the apply, so the next tick scans again.
// Each apply is checked on its own: they are three separate wirings and the
// one a refactor drops is silently a scan that never runs a second time.
func TestCockpitScanGuardsAreReleasedByTheirApply(t *testing.T) {
	for _, tc := range []struct {
		name  string
		busy  func(*cockpit) *bool
		apply func(*cockpit)
	}{
		{"cost", func(c *cockpit) *bool { return &c.costBusy }, func(c *cockpit) { c.applyCost(&posse.CostReport{}) }},
		{"plan", func(c *cockpit) *bool { return &c.planBusy }, func(c *cockpit) { c.applyPlan(planRead{}) }},
		{"gov", func(c *cockpit) *bool { return &c.govBusy }, func(c *cockpit) { c.applyGov(govRead{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &cockpit{}
			*tc.busy(c) = true
			tc.apply(c)
			if *tc.busy(c) {
				t.Fatalf("apply%s left the guard set: this scan never runs again", tc.name)
			}
			var started atomic.Int32
			c.start(tc.busy(c), func() { started.Add(1) })
			deadline := time.Now().Add(2 * time.Second)
			for started.Load() == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if started.Load() != 1 {
				t.Fatalf("the tick after a landed result started no scan")
			}
		})
	}
}

// ARM C — the loops go through the guard. ARM A pins the mechanism and ARM B
// pins its release, and both stay green if a timer case is written back as a
// bare `go c.scanGov()` — which is precisely the line this bead is about. The
// source is the only place that fact lives, so read it there.
func TestCockpitTimersNeverStartAScanUnguarded(t *testing.T) {
	src, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	// Comments stripped first. The rule is written down a few lines above
	// c.start in the source it is checking, and a grep that reads its own
	// statement of the rule as a violation is a test that is born red.
	var code []byte
	for _, line := range strings.Split(string(src), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		code = append(append(code, line...), '\n')
	}
	bare := regexp.MustCompile(`go c\.scan[A-Za-z]*\(`)
	if m := bare.FindAllString(string(code), -1); len(m) > 0 {
		t.Fatalf("a scan is started outside the in-flight guard: %v — every timer path must call c.start (ranger-base-325q)", m)
	}
}

// The cost window's opening edge must hold still between ticks, or
// posse.CostScanner's memo — which keys on the `since` a decode was taken
// under — misses on every file and the whole transcript pile is re-read. It
// is the difference between a 1.5 MB scan and a 786 MB one every 30 seconds.
func TestCockpitCostWindowIsStableAcrossTicks(t *testing.T) {
	base := time.Date(2026, 8, 29, 23, 0, 30, 0, time.UTC)
	first := costSince(base)
	for _, d := range []time.Duration{30 * time.Second, 2 * time.Minute, 29 * time.Minute} {
		if got := costSince(base.Add(d)); !got.Equal(first) {
			t.Fatalf("the window moved %v into the hour: %v then %v", d, first, got)
		}
	}
	// It is still a fourteen-day window, not a frozen one: the next hour
	// moves it by exactly an hour.
	if got, want := costSince(base.Add(time.Hour)), first.Add(time.Hour); !got.Equal(want) {
		t.Fatalf("the window did not advance with the hour: %v, want %v", got, want)
	}
	if got, want := first, base.Truncate(time.Hour).Add(-14*24*time.Hour); !got.Equal(want) {
		t.Fatalf("the window is not fourteen days: %v, want %v", got, want)
	}
}

// The governance check runs a cost scan of its own — G6's day window, when
// Dial E is armed — and it is a SEPARATE window from the footer's fourteen
// days. Two things must hold or the cockpit pays for a full decode of the
// transcript pile twice per thirty seconds, which is what it was doing when
// this bead was measured at 101.9% of a core:
//
//	the check scans through a kept scanner, not a bare ScanCosts;
//	that scanner is not the footer's, because a memo entry remembers the one
//	window its answer was taken under and the two ticks would evict each
//	other on every pass.
func TestCockpitGovCheckScansThroughItsOwnMemory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "p1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","timestamp":"2026-08-29T09:00:00Z","message":{"content":"Work beads issue qa-1: t"}}` + "\n" +
		`{"type":"assistant","timestamp":"2026-08-29T09:00:01Z","message":{"id":"m1","model":"claude-opus-5","usage":{"output_tokens":1000}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "t.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("RHQ_HOME", filepath.Join(home, "posse"))
	a, err := posse.NewApp()
	if err != nil {
		t.Fatal(err)
	}
	c := newCockpit(a, nil, io.Discard)
	if c.costScan == nil || c.govScan == nil {
		t.Fatal("a cockpit must be built with both scanners")
	}
	if c.costScan == c.govScan {
		t.Fatal("the two windows share one scanner: each tick would evict the other's memo and both would miss")
	}

	in := c.govInputs()
	if in.Spend == nil {
		t.Fatal("the shop check was handed no scanner: G6 falls back to a bare ScanCosts every tick")
	}
	in.Spend(time.Time{})
	if got := c.govScan.Remembered(); got != 1 {
		t.Fatalf("the governance scan remembered %d files, want 1 — it did not go through govScan", got)
	}
	if got := c.costScan.Remembered(); got != 0 {
		t.Fatalf("the governance scan wrote into the FOOTER's memo (%d files): the windows differ and it would evict on every tick", got)
	}
}
