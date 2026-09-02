package posse

// ranger-base-fxs60: the guard clock. MEASURED 2026-09-02 — a `dispatch
// --watch` loop (pid 19445) printed "── pass 1 · 14:37:16" and then nothing
// but pulse lines and "still working after 30m/45m — waiting again" for
// 1h40m, with a 5-minute interval. Under ADR 0028 §1 the Run does not return
// while a bead is in flight, and the load guard was read only at the top of
// a pass — so while the box climbed to load 75-85 the guard never evaluated,
// arm 2 never ran, and eight orphaned gate-shell children burned ~50% of a
// core each for 37 minutes until the operator ended them by hand.
//
// The addendum is the second half and the more surprising one: after the
// loop was restarted, the 1-minute load at pass start had dipped to 44,
// under that instance's `load_guard: 60`, so the pass was NOT skipped — and
// arm 2, which rides inside the skip, STILL did not run against the same
// eight orphans it matched the whole time. Load is not the predicate.
//
// So there are two things to pin and they fail in different directions:
//
//   - the clock TICKS while a pass is stuck in its gather (the loop arms
//     below, whose fixture is drain_qa_test.go's: a leg two orders of
//     magnitude longer than anything asserted here);
//   - the census runs whether or not the box is over the line (the unit arms
//     above them), and says nothing at all on a box that is holding neither
//     (the silence arm, which is what keeps a clock that ticks every
//     interval for days from being a clock nobody reads).
//
// MUTATIONS RUN — see the bead comment for the outputs:
//   - drop the guardLoop goroutine from Watch → both loop arms red, with the
//     pass still gathering, which is the bug's own shape.
//   - gate GuardTickLine's census on `why != ""` (the shipped behaviour this
//     bead changes) → the under-the-line arms red and the over-the-line arms
//     stay green, which is the addendum's shape exactly.
//   - return the report from GuardTickLine unconditionally (drop the ""
//     branch) → the silence arm reds.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The incident's own orphans: eight of them, ~50% of a core each, 40 minutes
// old, carrying the ADR 0009 gate-shell preamble at the head of a `go test`
// line, and no POSSE_KEEP anywhere on it.
const fxs60Payload = `go test ./internal/posse -run rollingrun -count=3`

func fxs60Orphans(n int) []Proc {
	var rows []Proc
	for i := 0; i < n; i++ {
		rows = append(rows, Proc{
			PID: 71100 + i, PPID: 1, CPU: 50.4, Comm: "zsh",
			Age: 40 * time.Minute, Args: gateArgv(fxs60Payload),
		})
	}
	return rows
}

// A tick has nothing to say about a box that is under the line and holding
// none of ours — including a LOADED one, which is the arm that matters: the
// clock reports a predicate, not a process table. Without this, "the census
// runs every tick" would be indistinguishable from a clock that prints the
// top of `ps` into the watch log every interval forever.
func TestGuardTickIsSilentWhenThereIsNothingTrueToSay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		rows []Proc
	}{
		{"a quiet box", nil},
		// Busy, old, burning a core — and parented, and not ours. Two of the
		// three legs of the predicate, which is what a loose census would
		// call a leak.
		{"busy with somebody else's work", []Proc{
			{PID: 812, PPID: 400, CPU: 88.2, Comm: "node", Age: 3 * time.Hour, Args: "node build.js"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAppAt(t.TempDir())
			a.Load1 = func() (float64, error) { return 0.4, nil }
			a.TopCPU = func() ([]Proc, error) { return tc.rows, nil }
			var errw strings.Builder
			if line := a.GuardTickLine(&errw); line != "" {
				t.Errorf("the clock must write nothing here, said:\n%s", line)
			}
		})
	}
}

// The addendum, as a unit: load 44 under `load_guard: 60` is a pass the
// guard does NOT skip, and the eight orphans must still be named and — the
// arm being armed — ended.
func TestGuardTickRunsTheOrphanCensusUnderTheLine(t *testing.T) {
	t.Parallel()
	a, offered, outcomes := killArmApp(t, true)
	if err := os.WriteFile(a.ConfigPath, []byte("load_guard: 60\nload_guard_kill: true\n"), 0o644); err != nil {
		t.Fatalf("config: %v", err)
	}
	rows := fxs60Orphans(8)
	for _, p := range rows {
		outcomes[p.PID] = killedByTERM
	}
	a.Load1 = func() (float64, error) { return 44, nil }
	a.TopCPU = func() ([]Proc, error) { return rows, nil }

	var errw strings.Builder
	got := a.GuardTickLine(&errw)
	if strings.Contains(got, "is over load_guard") {
		t.Errorf("44 is under load_guard: 60 — the tick must claim no witness it did not take:\n%s", got)
	}
	for _, want := range []string{
		"8 orphaned gate-shell children (ppid 1, over 20% CPU, over 1m)",
		"load_guard_kill: true, 8 of 8 ended",
		fxs60Payload,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the under-the-line tick must carry %q:\n%s", want, got)
		}
	}
	if len(*offered) != len(rows) {
		t.Errorf("the reaper was offered %v; every undeclared leak is its target whatever the load is", *offered)
	}
}

// Over the line, the tick is the reading the pass used to take: the witness,
// the culprits under it, and the report under those — off ONE census.
func TestGuardTickOverTheLineNamesTheWitnessAndTheCulprits(t *testing.T) {
	t.Parallel()
	a, _, _ := killArmApp(t, false)
	a.Load1 = func() (float64, error) { return 85.5, nil }
	a.TopCPU = func() ([]Proc, error) { return fxs60Orphans(8), nil }

	var errw strings.Builder
	got := a.GuardTickLine(&errw)
	for _, want := range []string{
		"load guard: 1-min loadavg 85.50 is over load_guard: 25",
		"guard clock", // said off the pass: this reading skipped no pass
		"load guard: top CPU: 50.4% pid 71100 zsh [ORPHANED 40m]",
		"8 orphaned gate-shell children",
		"REPORT ONLY, nothing was killed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the over-the-line tick must carry %q:\n%s", want, got)
		}
	}
}

// The loop arms: a pass held in its gather by a leg that will not come back,
// and the guard evaluated anyway — repeatedly, at the interval, with no
// second pass anywhere in the log.
func TestQAGuardClockEvaluatesWhileAPassIsStuckInTheGather(t *testing.T) {
	t.Parallel()
	// The 19445 shape: the box goes over the line AFTER the pass is already
	// gathering. Raised then, not at the start, because a pass that read a
	// high load at its own top would have skipped and looped — which is a
	// loop with a second pass in it, and then nothing here would be about
	// the clock.
	t.Run("over the line", func(t *testing.T) {
		rig := guardClockFixture(t, "")
		waitForOut(t, rig.out, "in flight, gathering")
		rig.raise(85.5, fxs60Orphans(8))

		// TWICE, not once: the bug is a reading that happened at pass start
		// and never again, and a single line is what one pass start already
		// produced.
		waitForCount(t, rig.out, "is over load_guard: 25", 2)
		s := rig.out.String()
		rig.stillGathering(t, s)
		if !strings.Contains(s, "8 orphaned gate-shell children") {
			t.Errorf("the tick that took the witness must name who is holding the box:\n%s", s)
		}
	})

	// The addendum, at the loop's own surface: under the line, gathering,
	// and the eight orphans ended anyway.
	t.Run("under the line", func(t *testing.T) {
		rig := guardClockFixture(t, "load_guard: 60\nload_guard_kill: true\n")
		waitForOut(t, rig.out, "in flight, gathering")
		rig.raise(44, fxs60Orphans(8))

		waitForCount(t, rig.out, "load_guard_kill: true, 8 of 8 ended", 1)
		s := rig.out.String()
		rig.stillGathering(t, s)
		if strings.Contains(s, "is over load_guard") {
			t.Errorf("44 is under load_guard: 60 — the loop must claim no witness it did not take:\n%s", s)
		}
		if got := rig.offeredPIDs(); len(got) != 8 {
			t.Errorf("the reaper must be offered all eight undeclared leaks, got %d: %v\n%s", len(got), got, s)
		}
	})
}

// guardRig is one watch loop over a bead that never settles, with the box's
// load, its process table and its reaper all in the test's hand.
type guardRig struct {
	out *syncBuf

	mu      sync.Mutex
	load    float64
	rows    []Proc
	offered map[int]bool
}

func (r *guardRig) raise(load float64, rows []Proc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.load, r.rows = load, rows
}

// offeredPIDs is the DISTINCT set the reaper was handed: the fixture's rows
// do not disappear when a tick "ends" them, so a later tick offers the same
// pids again — real ones are gone by then and skipped on the re-verify
// (loadguardkill.go), and counting repeats here would be counting the
// fixture rather than the selection.
func (r *guardRig) offeredPIDs() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []int
	for pid := range r.offered {
		out = append(out, pid)
	}
	return out
}

// stillGathering is what makes every assertion above about the CLOCK: a loop
// that got its tick back and ran a second pass would report the guard too,
// and would report it for the reason this bead says is broken.
func (r *guardRig) stillGathering(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(s, passHeader+"2") || strings.Contains(s, "next pass in") {
		t.Fatalf("pass 1 must still be holding its one leg — a second pass would be taking these readings, not the clock:\n%s", s)
	}
}

// guardClockFixture is drainFixture's shape (see its doc for why the loop and
// the join are owned here), with cfg appended to the instance config and the
// three load-guard seams pointed at the rig.
func guardClockFixture(t *testing.T, cfg string) *guardRig {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	rig := &guardRig{out: &syncBuf{}, offered: map[int]bool{}}
	d.Out = rig.out
	b.App.Load1 = func() (float64, error) {
		rig.mu.Lock()
		defer rig.mu.Unlock()
		return rig.load, nil
	}
	b.App.TopCPU = func() ([]Proc, error) {
		rig.mu.Lock()
		defer rig.mu.Unlock()
		return rig.rows, nil
	}
	b.App.ReapOrphans = func(targets []Proc) map[int]string {
		rig.mu.Lock()
		defer rig.mu.Unlock()
		out := map[int]string{}
		for _, p := range targets {
			rig.offered[p.PID] = true
			out[p.PID] = killedByTERM
		}
		return out
	}
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	// qaRepo owns the config file, so the guard's keys go on after it.
	if err := os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"+cfg), 0o644); err != nil {
		t.Fatalf("config: %v", err)
	}
	os.WriteFile(filepath.Join(fake, "prompt-delay-ms"), []byte(drainLegMS), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	loop := make(chan struct{})
	go func() {
		defer close(loop)
		d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond)
	}()
	// Registered after every t.TempDir newTestBackend took, so LIFO runs it
	// first: cancel, join the loop, join the abandoned leg, and only then is
	// anything removed (drain_qa_test.go's own note on the 385 stale trees).
	t.Cleanup(func() {
		cancel()
		select {
		case <-loop:
		case <-time.After(30 * time.Second):
			t.Errorf("the watch loop never returned after cancel:\n%s", rig.out.String())
		}
		joinHeldPrompts(t, fake, 1)
	})
	return rig
}

// waitForCount is waitForOut for a line the clock repeats: it blocks until
// want has been said at least n times.
func waitForCount(t *testing.T, buf *syncBuf, want string, n int) {
	t.Helper()
	// A third of waitForOut's budget, and still 750x the need: every caller
	// has already waited out the launch that forks the test binary, so what
	// is left is two 20ms ticks over a census with no `ps` in it. A pin that
	// reds for the right reason should not cost ninety seconds to say so.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Count(buf.String(), want) >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("output said %q %d times, want %d:\n%s", want, strings.Count(buf.String(), want), n, buf.String())
}
