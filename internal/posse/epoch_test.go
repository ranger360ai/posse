package posse

// ADR 0028 §2 — the epoch: the wall-clock window `budget_pass:` and `-n` are
// both denominated in, and ADR 0028 §5 observable 3's restart arm.
//
// Every test here is hermetic: an injected Spend for the dollars, the fake
// herdr/bd for the shop, and no reading of the operator's own transcripts,
// ledger or clock beyond `time.Now`. The two that could race the wall clock
// say so where they do, and neither can pass by accident — the control arm
// (what a PASS-denominated window would have read) is asserted, not assumed.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ─── the arithmetic ──────────────────────────────────────────────────────────

// The epoch is a wall-clock grid anchored at local midnight, so the answer
// is a function of the CLOCK and of nothing else — not of when a Run
// started, which is the whole property §2 asks for.
func TestEpochStartIsWallClockAligned(t *testing.T) {
	t.Parallel()
	// A day with no DST transition in any zone this can run in, at a time
	// nobody's midnight is near.
	base := time.Date(2026, 8, 27, 0, 0, 0, 0, time.Local)
	at := func(h, m int) time.Time { return base.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute) }
	for _, tc := range []struct {
		name  string
		epoch time.Duration
		now   time.Time
		want  time.Time
	}{
		{"an hour in", time.Hour, at(13, 37), at(13, 0)},
		{"exactly on a boundary", time.Hour, at(13, 0), at(13, 0)},
		{"the first epoch of the day", time.Hour, at(0, 5), at(0, 0)},
		{"midnight itself", time.Hour, base, base},
		{"half-hour epochs", 30 * time.Minute, at(13, 37), at(13, 30)},
		{"epochs that do not divide the hour", 90 * time.Minute, at(13, 37), at(13, 30)},
		{"an epoch longer than the day is the day", 48 * time.Hour, at(13, 37), base},
		// Nothing configured this; EpochStart is called from a path where a
		// zero would mean "no window at all", so it falls back rather than
		// dividing by zero.
		{"a zero epoch falls back to the default", 0, at(13, 37), at(13, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EpochStart(tc.now, tc.epoch); !got.Equal(tc.want) {
				t.Errorf("EpochStart(%s, %s) = %s, want %s",
					tc.now.Format("15:04:05"), tc.epoch, got.Format("15:04:05"), tc.want.Format("15:04:05"))
			}
		})
	}
}

// §2's reason for existing, at the arithmetic level: two Runs at different
// instants inside one epoch measure against the SAME window opening, and the
// next epoch is a different one. The control arm is asserted — the two
// instants really are different — so the equality below cannot be trivially
// true, which is exactly how a per-Run window would have passed this.
func TestEpochStartIsTheSameAcrossARestart(t *testing.T) {
	t.Parallel()
	first := time.Date(2026, 8, 27, 13, 2, 0, 0, time.Local)
	restart := time.Date(2026, 8, 27, 13, 49, 30, 0, time.Local)
	next := time.Date(2026, 8, 27, 14, 0, 1, 0, time.Local)

	if first.Equal(restart) {
		t.Fatal("the two instants must differ, or this test proves nothing")
	}
	if a, b := EpochStart(first, time.Hour), EpochStart(restart, time.Hour); !a.Equal(b) {
		t.Errorf("a Run that died at %s and restarted at %s must measure against one window: %s vs %s",
			first.Format("15:04:05"), restart.Format("15:04:05"), a, b)
	}
	if a, b := EpochStart(restart, time.Hour), EpochStart(next, time.Hour); a.Equal(b) {
		t.Errorf("the epoch must turn at %s — %s is not a new window", next.Format("15:04:05"), b)
	}
}

// ─── the config key ──────────────────────────────────────────────────────────

func TestDispatchEpochConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, raw string
		want      time.Duration
		warn      bool
	}{
		{"unset", "", DefaultDispatchEpoch, false},
		{"a duration", "dispatch_epoch: 30m\n", 30 * time.Minute, false},
		{"bare seconds, as --watch takes them", "dispatch_epoch: 90\n", 90 * time.Second, false},
		{"empty value", "dispatch_epoch:\n", DefaultDispatchEpoch, false},
		// A typo in the window that denominates both brakes must be VISIBLE.
		// Silently widening it would raise spend authority; silently
		// narrowing it would refill `-n` every pass.
		{"a word", "dispatch_epoch: hourly\n", DefaultDispatchEpoch, true},
		{"zero", "dispatch_epoch: 0\n", DefaultDispatchEpoch, true},
		{"negative", "dispatch_epoch: -1h\n", DefaultDispatchEpoch, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			if err := os.WriteFile(b.App.ConfigPath, []byte(tc.raw), 0o644); err != nil {
				t.Fatal(err)
			}
			var errb strings.Builder
			if got := b.App.DispatchEpoch(&errb); got != tc.want {
				t.Errorf("DispatchEpoch(%q) = %s, want %s", tc.raw, got, tc.want)
			}
			if warned := strings.Contains(errb.String(), "dispatch_epoch"); warned != tc.warn {
				t.Errorf("warned=%v, want %v for %q (stderr: %q)", warned, tc.warn, tc.raw, errb.String())
			}
		})
	}
}

// The malformed value is named once per PROCESS, not once per pass: a
// --watch loop must not write the same configuration fact into its log
// twelve times an hour (blindWarned's rule).
func TestMalformedEpochIsNamedOncePerProcess(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errb := dispatcherErr(t, d)
	os.WriteFile(b.App.ConfigPath, []byte("dispatch_epoch: hourly\n"), 0o644)

	for i := 0; i < 3; i++ {
		d.rollEpoch(time.Now().Add(time.Duration(i) * time.Hour))
	}
	if got := strings.Count(errb.String(), "dispatch_epoch"); got != 1 {
		t.Errorf("want the typo named once per process, said %d times:\n%s", got, errb.String())
	}
}

// ─── observable 3: spend across a Run restart ────────────────────────────────

// ADR 0028 §5 observable 3, the restart arm. A Run spends money, dies, and a
// fresh Run starts inside the same epoch: the money it spent is still inside
// the window the new Run measures `budget_pass:` against, so the cap holds
// across the restart instead of being re-granted by it.
//
// The control arm is the point and it is asserted, not assumed: the same
// spend, measured against a PASS-denominated window (one that opens when
// this Run does — which is what shipped before ADR 0028 §2), reads as $0 and
// would have launched.
func TestEpochSpendSurvivesARunRestart(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "budget_pass: 30\ndispatch_epoch: 1h")
	idleClaude(t, fake)

	// The dead Run: it opened this epoch and spent $40 in it.
	dead := newTestDispatcher(t, b)
	dead.rollEpoch(time.Now())
	spentAt := dead.epochStart
	// The spend must be in the PAST by the time the restart's pass begins,
	// or the control arm below is not a control at all. Costs at most the
	// sliver of a nanosecond-old epoch.
	for !spentAt.Before(time.Now()) {
		time.Sleep(time.Millisecond)
	}
	spend := func(time.Time) *CostReport {
		return &CostReport{Beads: []*Segment{{Bead: "a-0", Start: spentAt, CostUSD: 40}}}
	}

	// The control arm: a window that opens with THIS pass cannot see a cent
	// of it. That is the reading this test exists to make impossible.
	passStart := time.Now()
	if got := spend(time.Time{}).PassTotal(passStart); got != 0 {
		t.Fatalf("control arm broken: a pass-denominated window read $%.2f, so this test would pass either way", got)
	}

	restart := newTestDispatcher(t, b)
	restart.Spend = spend
	n, err := restart.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	// The skip is computed from the WALL CLOCK alone, never from what the
	// pass believed its window was: an hour boundary crossed mid-test is the
	// only honest reason not to measure, and a pass that opened a window of
	// its own must fail here rather than skip itself out of the assertions.
	if !EpochStart(passStart, time.Hour).Equal(EpochStart(time.Now(), time.Hour)) {
		t.Skip("the epoch turned while this test ran — nothing to measure")
	}
	if want := EpochStart(passStart, time.Hour); !restart.epochStart.Equal(want) {
		t.Errorf("the pass measured against %s, want the wall-clock epoch %s — a window that opens with the Run is what §2 removed",
			restart.epochStart.Format("15:04:05.000"), want.Format("15:04:05.000"))
	}
	out := dispatcherOut(restart)
	if n != 0 {
		t.Errorf("a restart must not re-grant budget_pass: %d dispatched over $40 of $30\n%s", n, out)
	}
	if want := "budget: epoch $40.00 of $30.00"; !strings.Contains(out, want) {
		t.Errorf("the refusal must name the epoch and its numbers (%q):\n%s", want, out)
	}
}

// The other half of "behaviour-neutral where the pass and the epoch
// coincide": spend from BEFORE this epoch is not charged to it. A window
// that never turned would hold yesterday's dollars against today's beads.
func TestEpochSpendExcludesTheEpochBefore(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "budget_pass: 30\ndispatch_epoch: 1h")
	idleClaude(t, fake)

	d := newTestDispatcher(t, b)
	d.rollEpoch(time.Now())
	before := d.epochStart.Add(-time.Nanosecond)
	d.Spend = func(time.Time) *CostReport {
		return &CostReport{Beads: []*Segment{{Bead: "a-0", Start: before, CostUSD: 40}}}
	}

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("$40 spent before this epoch opened is not this epoch's spend: %d dispatched\n%s", n, dispatcherOut(d))
	}
}

// ─── `-n` per epoch ──────────────────────────────────────────────────────────

// ADR 0028 §2: `-n`/`autostart_max_beads` bound launch attempts per EPOCH.
// Two failing personas and `-n 1` — the shape TestDispatchMaxBoundsAttempts
// pins per pass — asked three times: the second pass of the epoch gets
// nothing, and the first pass of the next one gets the cap back.
//
// "no agent detected" is the attempt counter: it is printed exactly once per
// attempt that reached a launch, so counting it counts attempts without
// reading the private tally.
func TestLaunchCapIsSpentPerEpochNotPerPass(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 100 * time.Millisecond
	writePersona(t, b.App, "ranger", "[go]")
	writePersona(t, b.App, "scout", "[py]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["py"]}]`, "")

	attempts := func(pass int) int {
		t.Helper()
		before := strings.Count(dispatcherOut(d), "no agent detected")
		if _, err := d.Run("", "", 1); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		return strings.Count(dispatcherOut(d), "no agent detected") - before
	}

	if got := attempts(1); got != 1 {
		t.Fatalf("the epoch's first pass gets the whole cap: %d attempt(s)\n%s", got, dispatcherOut(d))
	}
	if got := attempts(2); got != 0 {
		t.Errorf("`-n 1` is one launch per EPOCH, not per pass: pass 2 made %d attempt(s)\n%s", got, dispatcherOut(d))
	}
	if want := "launch cap: 1 of 1 attempt(s) spent this epoch"; !strings.Contains(dispatcherOut(d), want) {
		t.Errorf("a pass with ready work that launches nothing must say why (%q):\n%s", want, dispatcherOut(d))
	}

	// The epoch turns. rollEpoch compares the computed opening against the
	// stored one, so backdating the stored one is exactly what a wall-clock
	// turn does to it.
	d.epochStart = d.epochStart.Add(-time.Hour)
	if got := attempts(3); got != 1 {
		t.Errorf("a new epoch refills the cap: %d attempt(s)\n%s", got, dispatcherOut(d))
	}
}

// A --dry-run pass launches nothing, so it costs the epoch nothing. The
// alternative — a read-only diagnostic that eats the loop's launch budget —
// is the one thing `--dry-run` promises it will not do (ADR 0028's
// consequences: semantics unchanged).
func TestDryRunSpendsNoEpochAttempts(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)

	n, err := d.Run("", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("the dry pass must still report what a real one would do: %d\n%s", n, dispatcherOut(d))
	}
	if d.epochAttempts != 0 {
		t.Errorf("a dry pass booked %d attempt(s) against the epoch — it launched nothing", d.epochAttempts)
	}
}

// The cap line names the epoch's end, and it has to be the END and not the
// start: "nothing until 14:00" is actionable, "nothing since 13:00" is not.
func TestLaunchCapLineNamesWhenTheEpochTurns(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	os.WriteFile(b.App.ConfigPath, []byte("dispatch_epoch: 1h\n"), 0o644)
	d.rollEpoch(time.Now())
	d.epochAttempts = 3

	room, ok := d.epochRoom(3)
	if ok || room != 0 {
		t.Fatalf("a spent cap has no room: room=%d ok=%v", room, ok)
	}
	want := d.epochStart.Add(time.Hour).Local().Format("15:04:05")
	if out := dispatcherOut(d); !strings.Contains(out, want) {
		t.Errorf("the cap line must name when the epoch turns (%s):\n%s", want, out)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "autostart_max_beads") {
		t.Errorf("the cap line must name the knob that raises it:\n%s", out)
	}
}

// A cap with room left is silent and returns exactly the room: the pass
// fires as it always did, and `-n 0` is no cap at all.
func TestEpochRoomIsQuietWhileItHasRoom(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.rollEpoch(time.Now())
	d.epochAttempts = 1

	for _, tc := range []struct{ max, want int }{{3, 2}, {0, 0}, {-1, -1}} {
		room, ok := d.epochRoom(tc.max)
		if !ok || room != tc.want {
			t.Errorf("epochRoom(%d) = %d,%v — want %d,true", tc.max, room, ok, tc.want)
		}
	}
	if out := dispatcherOut(d); out != "" {
		t.Errorf("a cap with room says nothing:\n%s", out)
	}
}

// The reset is the EPOCH's, not the pass's: a pass inside the epoch that
// rolls nothing must not hand the cap back.
func TestRollEpochResetsOnlyWhenTheEpochTurns(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	now := time.Now()

	if !d.rollEpoch(now) {
		t.Fatal("the first roll of a process always opens an epoch")
	}
	d.epochAttempts = 2
	if d.rollEpoch(now.Add(time.Millisecond)) {
		t.Error("a second pass in the same epoch must not open a new one")
	}
	if d.epochAttempts != 2 {
		t.Errorf("attempts reset inside an epoch: %d, want 2", d.epochAttempts)
	}
	if !d.rollEpoch(EpochStart(now, time.Hour).Add(time.Hour + time.Second)) {
		t.Error("the next wall-clock hour is a new epoch")
	}
	if d.epochAttempts != 0 {
		t.Errorf("a new epoch must hand the cap back: %d, want 0", d.epochAttempts)
	}
}

// ─── the wiring nobody sees ──────────────────────────────────────────────────

// A pass points itself at the epoch and does not open one of its own — the
// property a `time.Now()` window silently loses. Pinned through Run, because
// the field being right in rollEpoch is not the claim; the claim is that
// Dial E's window is that field.
func TestRunMeasuresAgainstTheEpochNotTheRun(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "budget_pass: 30\ndispatch_epoch: 1h")
	idleClaude(t, fake)

	d := newTestDispatcher(t, b)
	var since time.Time
	d.Spend = func(t time.Time) *CostReport { since = t; return &CostReport{} }
	started := time.Now()
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if d.epochStart.After(started) {
		t.Errorf("the epoch opened after the pass did (%s > %s) — that is a pass window wearing an epoch's name",
			d.epochStart.Format("15:04:05.000"), started.Format("15:04:05.000"))
	}
	// One scan feeds both windows and it still starts at the day's floor:
	// an epoch anchored at local midnight never opens before it.
	floor := startOfDay(started)
	if !since.Equal(floor) {
		t.Errorf("the ledger scan started at %s, want the day floor %s", since, floor)
	}
	if d.epochStart.Before(floor) {
		t.Errorf("an epoch (%s) must never open before the day window's floor (%s)", d.epochStart, floor)
	}
}

// The account line an operator reads says EPOCH, because that is the window
// the number was taken over. `budget_pass:` stays the config key — renaming
// a public config key is not this ADR's business — so the two must not be
// confused for each other in the output.
func TestBudgetOutputNamesTheEpochWindow(t *testing.T) {
	t.Parallel()
	st := BudgetState{PassCap: 30, PassSpend: 40, DayCap: 250, DaySpend: 40}
	st.resolve()
	if st.Window != "epoch" {
		t.Errorf("the tightest window is %q, want %q", st.Window, "epoch")
	}
	for _, got := range []string{st.Line(), st.Ledger(), st.Short(), budgetSkipLine(st)} {
		if strings.HasPrefix(got, "pass ") || strings.Contains(got, " pass $") {
			t.Errorf("%q still calls the epoch a pass", got)
		}
	}
	if want := "epoch $40.00 of $30.00 (133%)"; st.Line() != want {
		t.Errorf("Line() = %q, want %q", st.Line(), want)
	}
	if !strings.Contains(budgetSkipLine(st), "budget_pass:") {
		t.Errorf("the refusal must still name the config key that raises it: %q", budgetSkipLine(st))
	}
}

// examples/config.yaml is the config surface; a key with teeth that is not
// documented there is a key nobody can find. (seedconfig_test.go pins the
// other half: the seed must ship it unset.)
func TestDispatchEpochIsDocumented(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(seedConfigPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "dispatch_epoch:") {
		t.Error("examples/config.yaml does not document dispatch_epoch: — a key with teeth nobody can find")
	}
}
