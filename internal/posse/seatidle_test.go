package posse

// ADR 0028 §5 observable 1 — the control arm's own tests.
//
// The subtraction is pinned on a hand-written ledger, because the number
// this instrument exists to produce is a duration and a live pass produces
// one measured in milliseconds. The pass tests pin the wiring: that a real
// dispatch cycle writes both halves, emits the per-seat line, and that
// --dry-run writes neither.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seatLedger writes a ledger fixture and returns its path.
func seatLedger(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "seat-cadence.log")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func seatAt(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// The window is settle → next launch INTO THE SAME SEAT: another seat's
// settle in between is not this seat's idle, and neither is an older settle
// of its own.
func TestSeatIdleMeasuresLastSettleOfThisSeat(t *testing.T) {
	t.Parallel()
	p := seatLedger(t,
		"2026-08-27T18:00:00Z launch ranger-posse a-1 claude",
		"2026-08-27T18:20:00Z settle ranger-posse a-1 idle",
		"2026-08-27T18:25:00Z settle hopper-posse b-9 idle",
	)
	r := SeatIdleAt(p, "ranger-posse", "a-2", seatAt(t, "2026-08-27T18:50:00Z"))
	if !r.Measured() {
		t.Fatalf("want a measured window, got %q", r.Why)
	}
	if r.Idle != 30*time.Minute {
		t.Errorf("want 30m idle, got %s", r.Idle)
	}
	if r.After != "a-1" {
		t.Errorf("want the window opened by a-1, got %q", r.After)
	}
	if line := r.Line(); !strings.Contains(line, "ranger-posse") || !strings.Contains(line, "30m0s") {
		t.Errorf("account line does not carry the seat and the figure: %s", line)
	}
}

// A seat nobody has settled yet has nothing to subtract from, and says so.
// Zero would read as an instant refill — the exact number the ADR's target
// claims — off a seat that was never measured at all.
func TestSeatIdleFirstLaunchIsNamedNotZero(t *testing.T) {
	t.Parallel()
	p := seatLedger(t, "2026-08-27T18:00:00Z launch hopper-posse b-1 claude")
	r := SeatIdleAt(p, "ranger-posse", "a-1", seatAt(t, "2026-08-27T18:50:00Z"))
	if r.Measured() {
		t.Fatalf("a seat with no settle on record must not report a figure, got %s", r.Idle)
	}
	if r.Idle != 0 || r.After != "" {
		t.Errorf("unmeasured refill must carry no window: idle=%s after=%q", r.Idle, r.After)
	}
	if !strings.Contains(r.Line(), "first launch") {
		t.Errorf("the line must say why there is no figure: %s", r.Line())
	}
}

// A seat refilled without its settle ever being recorded — the pass died
// mid-gather, the bead settled blocked and a human freed it — has an
// on-file settle that is a whole bead old. Subtracting across it would
// charge that bead's runtime to idle.
func TestSeatIdleUnobservedSettleIsRefused(t *testing.T) {
	t.Parallel()
	p := seatLedger(t,
		"2026-08-27T10:00:00Z settle ranger-posse a-1 idle",
		"2026-08-27T10:05:00Z launch ranger-posse a-2 claude",
	)
	r := SeatIdleAt(p, "ranger-posse", "a-3", seatAt(t, "2026-08-27T18:00:00Z"))
	if r.Measured() {
		t.Fatalf("want a refusal across an unobserved settle, got %s", r.Idle)
	}
	if !strings.Contains(r.Why, "a-2") {
		t.Errorf("the refusal must name the launch it could not see past: %q", r.Why)
	}
}

// A settle stamped after the launch it precedes is a clock, not a seat.
func TestSeatIdleClockSkewIsNamedNotNegative(t *testing.T) {
	t.Parallel()
	p := seatLedger(t, "2026-08-27T19:00:00Z settle ranger-posse a-1 idle")
	r := SeatIdleAt(p, "ranger-posse", "a-2", seatAt(t, "2026-08-27T18:00:00Z"))
	if r.Idle < 0 {
		t.Fatalf("a negative idle would poison every aggregate: %s", r.Idle)
	}
	if r.Measured() || !strings.Contains(r.Why, "clock skew") {
		t.Errorf("want a named skew refusal, got measured=%v why=%q", r.Measured(), r.Why)
	}
}

// personaActive reads blocked as the persona being busy, so a blocked
// settle is not a free seat and must never open an idle window: the wait
// that follows it is a human's, not dispatch's.
func TestSeatBlockedSettleDoesNotFreeTheSeat(t *testing.T) {
	for _, st := range []string{"idle", "done"} {
		if !SeatFreeing(st) {
			t.Errorf("%s frees the seat", st)
		}
	}
	for _, st := range []string{"blocked", "working", ""} {
		if SeatFreeing(st) {
			t.Errorf("%s does not free the seat", st)
		}
	}
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	p := &pendingBead{is: RepoIssue{BdIssue: BdIssue{ID: "a-1"}, Dir: "/tmp/posse"}, persona: "ranger"}
	d.noteSeatSettle(p, "blocked", time.Now())
	if _, err := os.Stat(b.App.SeatCadenceLogPath()); !os.IsNotExist(err) {
		body, _ := os.ReadFile(b.App.SeatCadenceLogPath())
		t.Fatalf("a blocked settle must not be recorded as a freed seat:\n%s", body)
	}
	d.noteSeatSettle(p, "idle", time.Now())
	body, err := os.ReadFile(b.App.SeatCadenceLogPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "settle ranger-posse a-1 idle") {
		t.Fatalf("an idle settle must be recorded: %s", body)
	}
}

// What the dropped blocked settle does to the NEXT refill, which is the
// only thing the ledger's reader ever sees of it (ranger-base-yuu8).
//
// Two arms, because the writer guard is the whole control. The first is the
// real sequence: a-1 settles blocked, nothing is written, so the seat's
// newest event stays a-1's LAUNCH and a-2's refill fails SeatIdleAt's
// last.Kind guard into "previous settle not observed" — not a figure taken
// across a human's response time. The second plants that same line by hand
// to show what the guard is holding back: SeatIdleAt never reads the state
// column, so a blocked settle that reached this file WOULD be measured.
// Nothing downstream would catch it.
func TestSeatBlockedSettleLeavesTheNextRefillUnmeasured(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dir := "/tmp/posse"
	seat := SessionFor("ranger", dir)
	path := b.App.SeatCadenceLogPath()

	prev := RepoIssue{BdIssue: BdIssue{ID: "a-0"}, Dir: dir}
	d.noteSeatLaunch(prev, seat, "claude", seatAt(t, "2026-08-27T10:00:00Z"))
	d.noteSeatSettle(&pendingBead{is: prev, persona: "ranger"}, "idle", seatAt(t, "2026-08-27T10:10:00Z"))
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1"}, Dir: dir}
	d.noteSeatLaunch(is, seat, "claude", seatAt(t, "2026-08-27T10:20:00Z"))
	d.noteSeatSettle(&pendingBead{is: is, persona: "ranger"}, "blocked", seatAt(t, "2026-08-27T10:25:00Z"))

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "settle "+seat+" a-0 idle") {
		t.Fatalf("the freeing settle is the fixture's witness that anything was written at all:\n%s", body)
	}
	if strings.Contains(string(body), "blocked") {
		t.Fatalf("a blocked settle must not reach the ledger:\n%s", body)
	}

	r := SeatIdleAt(path, seat, "a-2", seatAt(t, "2026-08-27T18:00:00Z"))
	if r.Measured() {
		t.Fatalf("the refill after a blocked settle must not carry a figure, got %s (after %s)", r.Idle, r.After)
	}
	if !strings.Contains(r.Why, "previous settle not observed") || !strings.Contains(r.Why, "a-1") {
		t.Errorf("want the refusal to name the launch it could not see past, got %q", r.Why)
	}

	planted := seatLedger(t,
		"2026-08-27T10:20:00Z launch ranger-posse a-1 claude",
		"2026-08-27T10:25:00Z settle ranger-posse a-1 blocked",
	)
	if p := SeatIdleAt(planted, "ranger-posse", "a-2", seatAt(t, "2026-08-27T18:00:00Z")); !p.Measured() {
		t.Fatalf("the hazard arm measures nothing: SeatIdleAt already filters the state column, so the writer guard is no longer the control this test and noteSeatSettle's comment say it is (%q)", p.Why)
	}
}

// The pass wiring, end to end: pass 1 has no window to measure, pass 2
// measures the one pass 1 opened, and the account line names the seat.
func TestDispatchPassEmitsSeatIdleFigures(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"},{"id":"a-2","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	first := dispatcherOut(d)
	if !strings.Contains(first, "first launch") {
		t.Fatalf("the first pass into a fresh seat must say it has no window:\n%s", first)
	}
	ledger, err := os.ReadFile(b.App.SeatCadenceLogPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), "launch ranger-"+filepath.Base(repo)+" a-1") ||
		!strings.Contains(string(ledger), "settle ranger-"+filepath.Base(repo)+" a-1") {
		t.Fatalf("pass 1 must record both halves of the window:\n%s", ledger)
	}

	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(`[{"id":"a-2","title":"t","labels":["go"]}]`), 0o644)
	d.Out = &strings.Builder{}
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	second := dispatcherOut(d)
	seat := "ranger-" + filepath.Base(repo)
	if !strings.Contains(second, "idle-to-next "+seat) {
		t.Fatalf("pass 2 must emit a per-seat idle-to-next line for %s:\n%s", seat, second)
	}
	if !strings.Contains(second, "a-1 settled") || !strings.Contains(second, "a-2 launched") {
		t.Fatalf("the line must name the bead that freed the seat and the one that took it:\n%s", second)
	}
	if !strings.Contains(second, "1 of 1 refill(s) measured") {
		t.Fatalf("the pass must summarise what it measured:\n%s", second)
	}
}

// --dry-run acts on nothing. Writing either half would leave the next real
// launch subtracting from a window that never happened.
//
// The two recorders are pinned DIRECTLY, and that is the discriminating
// half of this test: a dry pass never reaches fire or gather at all, so
// removing the guards inside them leaves the pass assertion below green.
// The guards are what keeps that an accident of the call graph rather than
// the only thing standing between a dry pass and a poisoned ledger.
func TestSeatCadenceDryRunWritesNothing(t *testing.T) {
	{
		b, _ := newTestBackend(t)
		d := newTestDispatcher(t, b)
		d.DryRun = true
		is := RepoIssue{BdIssue: BdIssue{ID: "a-1"}, Dir: "/tmp/posse"}
		d.noteSeatLaunch(is, "ranger-posse", "claude", time.Now())
		d.noteSeatSettle(&pendingBead{is: is, persona: "ranger"}, "idle", time.Now())
		if _, err := os.Stat(b.App.SeatCadenceLogPath()); !os.IsNotExist(err) {
			body, _ := os.ReadFile(b.App.SeatCadenceLogPath())
			t.Fatalf("--dry-run recorded a seat event:\n%s", body)
		}
		if len(d.seatRefills) != 0 {
			t.Fatalf("--dry-run measured %d refill(s) it did not make", len(d.seatRefills))
		}
	}

	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","title":"t","status":"open"}]`)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.App.SeatCadenceLogPath()); !os.IsNotExist(err) {
		body, _ := os.ReadFile(b.App.SeatCadenceLogPath())
		t.Fatalf("--dry-run must write no seat ledger:\n%s", body)
	}
	if out := dispatcherOut(d); strings.Contains(out, "idle-to-next") {
		t.Fatalf("--dry-run measured a window it did not open:\n%s", out)
	}
}

// The median is the low middle of an even set: one seat waiting on a
// 75-minute bead is the distribution, not an outlier to average away.
func TestSeatIdleMedianIsTheLowMiddle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   []time.Duration
		want time.Duration
	}{
		{nil, 0},
		{[]time.Duration{time.Minute}, time.Minute},
		{[]time.Duration{time.Minute, 3 * time.Minute}, time.Minute},
		{[]time.Duration{time.Minute, 3 * time.Minute, 9 * time.Minute}, 3 * time.Minute},
	}
	for _, c := range cases {
		if got := medianDuration(c.in); got != c.want {
			t.Errorf("median(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}
