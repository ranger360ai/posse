package rhq

// The load guard (ranger-base-innx): nothing new is launched into a box
// whose 1-minute load average is over `load_guard:`. Every test here hands
// the guard its reading through App.Load1 — the one thing this feature must
// never do in a test is read the machine the suite is running on.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGuardConfig(t *testing.T) {
	for _, tc := range []struct {
		cfg  string
		want float64
		warn string
	}{
		{"", LoadGuardDefault, ""},
		{"load_guard: 40\n", 40, ""},
		{"load_guard: 7.5\n", 7.5, ""},
		{"load_guard: 0\n", 0, ""}, // the escape hatch: guard off
		{"load_guard: lots\n", LoadGuardDefault, "not a load average"},
		{"load_guard: -1\n", LoadGuardDefault, "not a load average"},
	} {
		a := NewAppAt(t.TempDir())
		if tc.cfg != "" {
			os.WriteFile(a.ConfigPath, []byte(tc.cfg), 0o644)
		}
		var errb strings.Builder
		got := a.LoadGuard(&errb)
		if got != tc.want {
			t.Errorf("LoadGuard(%q) = %g, want %g", tc.cfg, got, tc.want)
		}
		if tc.warn == "" && errb.Len() > 0 {
			t.Errorf("LoadGuard(%q) said %q and should have been quiet", tc.cfg, errb.String())
		}
		if tc.warn != "" && !strings.Contains(errb.String(), tc.warn) {
			t.Errorf("LoadGuard(%q) must name the typo, said %q", tc.cfg, errb.String())
		}
	}
}

func TestLoadHighComparesTheReadingToTheLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  string
		load float64
		want string // "" = the guard does not fire
	}{
		{"quiet box", "", 2.5, ""},
		{"the drain after the incident", "", 24.9, ""},
		{"exactly at the limit is not over it", "", 25, ""},
		{"the incident", "", 112.34, "112.34 is over load_guard: 25"},
		{"a lower limit the operator set", "load_guard: 8\n", 12, "12.00 is over load_guard: 8"},
		{"guard off launches into anything", "load_guard: 0\n", 260, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAppAt(t.TempDir())
			if tc.cfg != "" {
				os.WriteFile(a.ConfigPath, []byte(tc.cfg), 0o644)
			}
			a.Load1 = func() (float64, error) { return tc.load, nil }
			why := a.LoadHigh(io.Discard)
			if tc.want == "" && why != "" {
				t.Fatalf("load %g under %q must not gate, got %q", tc.load, tc.cfg, why)
			}
			if tc.want != "" && !strings.Contains(why, tc.want) {
				t.Fatalf("load %g under %q: LoadHigh = %q, want it to carry %q", tc.load, tc.cfg, why, tc.want)
			}
		})
	}
}

// A guard that cannot take its reading is a monitoring failure, and a
// monitoring failure must not be able to stop the shop.
func TestLoadHighFailsOpenAndSaysSo(t *testing.T) {
	a := NewAppAt(t.TempDir())
	a.Load1 = func() (float64, error) { return 0, Die("no /proc/loadavg here") }
	var errb strings.Builder
	if why := a.LoadHigh(&errb); why != "" {
		t.Errorf("an unreadable load must gate nothing, got %q", why)
	}
	if !strings.Contains(errb.String(), "no /proc/loadavg here") {
		t.Errorf("a blind guard must say why: %q", errb.String())
	}
}

// The reader for whatever platform this is built for. Not a measurement —
// the box's load at test time is whatever it is — only that the number
// arrives, which is the half that is platform code.
func TestSysLoad1ReadsThisBox(t *testing.T) {
	load, err := SysLoad1()
	if err != nil {
		t.Fatalf("SysLoad1: %v", err)
	}
	if load < 0 || load > 100000 {
		t.Errorf("SysLoad1 = %g, which is not a load average", load)
	}
}

// The pass half of the ask: one witness line, nothing launched, no error —
// --watch keeps its cadence and the next pass reads fresh.
func TestDispatchSkipsThePassOverTheLoadGuard(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","priority":1,"labels":["go"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	b.App.Load1 = func() (float64, error) { return 112.34, nil }

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("a skipped pass is not a failed pass: %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 dispatched, got %d:\n%s", n, dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "load guard") || !strings.Contains(out, "112.34") || !strings.Contains(out, "pass skipped") {
		t.Errorf("a skipped pass must say so once, with the number:\n%s", out)
	}
	if strings.Count(out, "load guard") != 1 {
		t.Errorf("one witness line per skipped pass, got:\n%s", out)
	}
	// Before every other reading the pass takes: the ready scan, verify-after
	// and the bead-loss census all fork `bd`, which is exactly what a box
	// this loaded cannot do.
	if log, err := os.ReadFile(filepath.Join(fake, "bd-calls.log")); err == nil {
		t.Errorf("a skipped pass must not fork bd:\n%s", log)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a skipped pass must launch nothing:\n%s", log)
	}
}

func TestDispatchRunsThePassUnderTheLoadGuard(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","priority":1,"labels":["go"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","status":"closed"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	b.App.Load1 = func() (float64, error) { return 6.0, nil }

	n, err := d.Run("", "", 0)
	if err != nil || n != 1 {
		t.Fatalf("a quiet box dispatches: n=%d err=%v\n%s", n, err, dispatcherOut(d))
	}
	if strings.Contains(dispatcherOut(d), "load guard") {
		t.Errorf("a guard that did not fire says nothing:\n%s", dispatcherOut(d))
	}
}

// --dry-run launches nothing, so the guard reports and gets out of the way:
// the one command an operator runs on a sick box must not be the one that
// goes quiet.
func TestDryRunSaysTheGuardWouldFireAndShowsRoutingAnyway(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","priority":1,"labels":["go"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	b.App.Load1 = func() (float64, error) { return 112.34, nil }

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "load guard") || !strings.Contains(out, "112.34") {
		t.Errorf("--dry-run must still say the guard would fire:\n%s", out)
	}
	if !strings.Contains(out, "a-1") {
		t.Errorf("--dry-run must still show routing:\n%s", out)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("--dry-run launches nothing, guard or no guard:\n%s", log)
	}
}

// The launch half: every path into planLaunch, which is all of them.
func TestCreateSessionRefusesOverTheLoadGuard(t *testing.T) {
	b, fake := newTestBackend(t)
	b.App.Load1 = func() (float64, error) { return 263, nil }

	err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: t.TempDir()})
	if err == nil {
		t.Fatal("posse new must not launch into a saturated box")
	}
	for _, want := range []string{"load guard", "263.00", "load_guard: 0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must carry %q: %v", want, err)
		}
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a refused launch must not reach herdr:\n%s", log)
	}
}

// Relaunch plans before it kills (rangerhq-v52t), so the guard refuses with
// the session the operator asked to refresh still alive. Losing a session to
// a box that could not start its replacement is the one way this guard could
// cost more than it saves.
func TestRelaunchRefusesOverTheLoadGuardWithoutKilling(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	m1, _ := b.readMeta("s1")
	b.App.Load1 = func() (float64, error) { return 263, nil }

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("relaunch must refuse a saturated box:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "load guard") {
		t.Errorf("the refusal must name the guard: %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace close") {
		t.Errorf("a refused relaunch must not close the workspace:\n%s", log)
	}
	if !b.HasSession("s1") {
		t.Error("the session the operator asked to refresh is gone")
	}
	if m2, ok := b.readMeta("s1"); !ok || m2.Workspace != m1.Workspace {
		t.Errorf("refused relaunch must leave the meta alone: %+v (was %+v)", m2, m1)
	}
}
