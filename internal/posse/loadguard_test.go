package posse

// The load guard (ranger-base-innx): nothing new is launched into a box
// whose 1-minute load average is over `load_guard:`. Every test here hands
// the guard its reading through App.Load1 — the one thing this feature must
// never do in a test is read the machine the suite is running on.

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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

// A launch inside a test must not consult the box the suite is running on.
// newTestBackend stubs App.Load1 for that (hermetic, herdr_test.go); an App
// that loses the stub reads SysLoad1 instead, and then every launch arm in
// that test is refused whenever the machine is busy. That is how
// TestQAHomeCutoverRehearsal came to fail under exactly the thing everyone
// does — a full `go test ./...`, which is itself a load source, more so
// with several personas verifying at once (ranger-base-w4fb).
//
// The assertion is the guard's own effect rather than a nil check, so it
// holds however the reading is lost. The ceiling is set below any load a
// box that is running this test can show: the stub's 0 clears it, a live
// reading does not.
func TestTestBackendLaunchesDoNotReadThisBox(t *testing.T) {
	b, _ := newTestBackend(t)
	if err := os.WriteFile(b.App.ConfigPath, []byte("load_guard: 0.01\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The control, taken from the box directly — the one reading the
	// mutation this pins cannot reach. Unless the live number really is
	// over the ceiling, a green below is a green that measured nothing.
	live, err := SysLoad1()
	if err != nil || live <= 0.01 {
		t.Skipf("this box reads %g (%v), not over the test's ceiling: nothing was measured", live, err)
	}
	if why := b.App.LoadHigh(io.Discard); why != "" {
		t.Errorf("a test App read the machine the suite is running on: %s", why)
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

// ─── the culprit line (ranger-base-0p6x) ────────────────────────────────────
//
// The guard printed nine identical witness lines over 45 minutes and named
// not one process, while sixteen orphaned shells burned ~30% CPU each and
// the fleet sat frozen for 2.5h (ranger-base-teau). Every test here hands
// the reading through App.TopCPU for the reason every test above hands the
// load through App.Load1: a witness assembled from the suite's own machine
// is red per-day, not per-commit.

// procRows is the incident, shrunk: two orphaned spinners at the top, a
// busy child of a live session between them, and more orphans below the cut.
func procRows() []Proc {
	rows := []Proc{
		{PID: 49235, PPID: 1, CPU: 36.9, Age: 2*time.Hour + 30*time.Minute, Comm: "zsh"},
		{PID: 812, PPID: 4021, CPU: 31.2, Age: 4 * time.Minute, Comm: "node"},
		{PID: 49241, PPID: 1, CPU: 30.1, Age: 2*time.Hour + 29*time.Minute, Comm: "zsh"},
		{PID: 1, PPID: 0, CPU: 0.1, Age: 300 * time.Hour, Comm: "launchd"},
	}
	for i := 0; i < 3; i++ { // the ones below the cut, still leaking
		rows = append(rows, Proc{PID: 50000 + i, PPID: 1, CPU: 24.5, Age: time.Hour, Comm: "zsh"})
	}
	return rows
}

func TestCulpritLineNamesTheBurnersAndCountsTheRest(t *testing.T) {
	a := NewAppAt(t.TempDir())
	a.TopCPU = func() ([]Proc, error) { return procRows(), nil }
	got := a.LoadCulpritLine()

	for _, want := range []string{
		"load guard: top CPU:",
		"36.9% pid 49235 zsh [ORPHANED 2h30m]", // the top burner, named and flagged
		"31.2% pid 812 node",                   // second by CPU, so it is listed
		"30.1% pid 49241 zsh [ORPHANED 2h29m]",
		"3 more orphaned processes over 20% CPU", // below the cut, counted
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the culprit line must carry %q:\n%s", want, got)
		}
	}
	// §3: a busy child of a live session is ordinary fleet work. It may be
	// listed as a top burner; it must never be presented as a leak.
	if strings.Contains(got, "pid 812 node [ORPHANED") {
		t.Errorf("a live session's child was flagged as a leak:\n%s", got)
	}
	// Three named, the rest counted — a log line, not a process listing.
	if strings.Count(got, "pid ") != loadCulpritTop {
		t.Errorf("want %d processes named outright, got:\n%s", loadCulpritTop, got)
	}
	// It appends to a witness sentence, so it brings its own second line.
	if !strings.HasPrefix(got, "\n") || strings.Count(got, "\n") != 1 {
		t.Errorf("the culprit line must be exactly one appended line: %q", got)
	}
}

// The tail counts orphans, not busy processes: a fleet at full tilt with no
// leak at all must not be told it has three.
func TestCulpritTailCountsOnlyOrphansOverTheFloor(t *testing.T) {
	rows := []Proc{
		{PID: 11, PPID: 900, CPU: 99, Comm: "go"},
		{PID: 12, PPID: 900, CPU: 98, Comm: "go"},
		{PID: 13, PPID: 900, CPU: 97, Comm: "go"},
		{PID: 14, PPID: 900, CPU: 96, Comm: "go"},                        // busy, below the cut, not an orphan
		{PID: 15, PPID: 1, CPU: LoadCulpritOrphanCPU - 0.1, Comm: "zsh"}, // orphan under the floor
	}
	a := NewAppAt(t.TempDir())
	a.TopCPU = func() ([]Proc, error) { return rows, nil }
	if got := a.LoadCulpritLine(); strings.Contains(got, "more orphaned") {
		t.Errorf("nothing here is a leak over %g%%:\n%s", LoadCulpritOrphanCPU, got)
	}
	rows[4].CPU = LoadCulpritOrphanCPU
	if got := a.LoadCulpritLine(); !strings.Contains(got, "1 more orphaned process over 20% CPU") {
		t.Errorf("one orphan at the floor must be counted, singular:\n%s", got)
	}
}

// §1: it must never be able to delay or fail a pass. Every way the reading
// can go wrong renders nothing at all, and the witness above it stands.
func TestCulpritLineDegradesToSilence(t *testing.T) {
	for _, tc := range []struct {
		name string
		read func() ([]Proc, error)
	}{
		// Both error arms hand back rows AS WELL as the error, which is the
		// arm that measures something: a reading that failed must be
		// discarded, not rendered from whatever partial table came with it.
		{"ps timed out on a box that cannot fork", func() ([]Proc, error) {
			return procRows(), Die("signal: killed")
		}},
		{"ps is not on this box", func() ([]Proc, error) {
			return procRows(), Die("exec: \"ps\": not found")
		}},
		{"a table it could not parse", func() ([]Proc, error) { return nil, nil }},
		{"an idle table", func() ([]Proc, error) {
			return []Proc{{PID: 1, PPID: 0, CPU: 0, Comm: "launchd"}}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAppAt(t.TempDir())
			a.TopCPU = tc.read
			if got := a.LoadCulpritLine(); got != "" {
				t.Errorf("a reading it could not take must print nothing, got %q", got)
			}
		})
	}
}

// §4: the columns are read from the FRONT, because only the last one may
// hold spaces. Both platforms' shapes go through the same parser.
func TestParseProcTableReadsTheColumnsFromTheFront(t *testing.T) {
	// darwin's comm is a full path; procps' is a bare truncated name.
	out := "" +
		"49235     1  36.9    02:30:15 /bin/zsh\n" +
		"  812  4021  31.2       04:11 node\n" +
		" 3300     1   9.0 3-02:00:00 /Applications/Some App.app/Contents/MacOS/Some App\n" +
		"\n" +
		"garbage line that is not a process\n" +
		"    x     1   1.0       00:01 nope\n"
	got := parseProcTable(out)
	if len(got) != 3 {
		t.Fatalf("want 3 readable rows, got %d: %+v", len(got), got)
	}
	want := []Proc{
		{PID: 49235, PPID: 1, CPU: 36.9, Age: 2*time.Hour + 30*time.Minute + 15*time.Second, Comm: "zsh"},
		{PID: 812, PPID: 4021, CPU: 31.2, Age: 4*time.Minute + 11*time.Second, Comm: "node"},
		{PID: 3300, PPID: 1, CPU: 9.0, Age: 74 * time.Hour, Comm: "Some App"},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseEtime(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"00:01", time.Second},
		{"04:11", 4*time.Minute + 11*time.Second},
		{"02:30:15", 2*time.Hour + 30*time.Minute + 15*time.Second},
		{"3-02:00:00", 74 * time.Hour},
		{"11", 0}, // ps never prints this; an age it cannot read is 0
		{"1:2:3:4", 0},
		{"-01:00", 0},
		{"", 0},
	} {
		if got := parseEtime(tc.in); got != tc.want {
			t.Errorf("parseEtime(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// The reader for whatever platform this is built for, the sibling of
// TestSysLoad1ReadsThisBox. Not a measurement — what this box is running is
// whatever it is — only that `ps` answers and the columns land where the
// parser looks, which is the half that is platform code.
func TestSysTopCPUReadsThisBox(t *testing.T) {
	procs, err := SysTopCPU()
	if errors.Is(err, os.ErrPermission) {
		t.Skipf("SysTopCPU: %v — cage denies exec of ps, not a parser fault", err)
	}
	if err != nil {
		t.Fatalf("SysTopCPU: %v", err)
	}
	if len(procs) < 2 {
		t.Fatalf("a box running this test has processes, got %+v", procs)
	}
	self := os.Getpid()
	for _, p := range procs {
		// comm itself may hold spaces on darwin — "Core Audio Driver
		// (ParrotAudioPlugin.driver)" is a real row on this box — which is
		// exactly why it is the last column and read as the remainder.
		if p.PID <= 0 || p.PPID < 0 || p.CPU < 0 || p.Comm == "" || strings.Contains(p.Comm, "/") {
			t.Errorf("unreadable row %+v — the columns moved", p)
		}
		// The census's SECOND read is scoped to the rows the orphan report
		// could be about (ranger-base-apwr). Every other row's argv is
		// unread, which is what keeps the extra fork off a healthy box —
		// and this is the arm that measures it on a real table.
		if p.Args != "" && !p.orphanSuspect() {
			t.Errorf("argv was read for a row that is not a suspect: %+v", p)
		}
		if p.PID == self {
			if p.PPID != os.Getppid() {
				t.Errorf("this test's own row says ppid %d, want %d — the columns moved", p.PPID, os.Getppid())
			}
			// comm is what this binary is CALLED. args would be that plus
			// the -test.* flags go test invoked it with, and reading args
			// is the mistake that named teau's spinners after the shell
			// that had merely spawned them. (procps truncates comm to 15
			// characters; darwin does not.)
			want := filepath.Base(os.Args[0])
			if p.Comm != want && !strings.HasPrefix(want, p.Comm) {
				t.Errorf("own comm = %q, want %q — that is args, not comm", p.Comm, want)
			}
			return
		}
	}
	t.Errorf("the test binary's own pid %d is not in its own process table", self)
}

// The whole point, end to end: the operator reading dispatch-watch.log gets
// the load AND who is holding it, on the pass that was skipped.
func TestASkippedPassNamesTheCulprits(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	b.App.Load1 = func() (float64, error) { return 149.08, nil }
	b.App.TopCPU = func() ([]Proc, error) { return procRows(), nil }

	if n, err := d.Run("", "", 0); n != 0 || err != nil {
		t.Fatalf("the guard must skip the pass: n=%d err=%v", n, err)
	}
	out := dispatcherOut(d)
	for _, want := range []string{
		"149.08 is over load_guard: 25",
		"pass skipped",
		"top CPU: 36.9% pid 49235 zsh [ORPHANED 2h30m]",
		"3 more orphaned processes over 20% CPU",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("a skipped pass must say %q:\n%s", want, out)
		}
	}
	// The consequence stays on the witness line; the culprits follow it.
	witness := strings.Index(out, "pass skipped")
	culprit := strings.Index(out, "top CPU:")
	if witness < 0 || culprit < witness {
		t.Errorf("the culprit line must follow the witness it explains:\n%s", out)
	}
	if strings.Count(out[witness:culprit], "\n") != 1 {
		t.Errorf("the culprits belong on the line under the witness:\n%s", out)
	}
}

// The same, for the report that would have ended the incident at 04:35: the
// operator reading dispatch-watch.log on the skipped pass gets the leaks by
// name, on the pass itself, in one write (ranger-base-apwr).
func TestASkippedPassNamesTheLeakedGateShellChildren(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	b.App.Load1 = func() (float64, error) { return 149.08, nil }
	b.App.TopCPU = func() ([]Proc, error) { return leakRows(16), nil }

	if n, err := d.Run("", "", 0); n != 0 || err != nil {
		t.Fatalf("the guard must skip the pass: n=%d err=%v", n, err)
	}
	out := dispatcherOut(d)
	for _, want := range []string{
		"pass skipped",
		"16 orphaned gate-shell children",
		"REPORT ONLY, nothing was killed",
		"36.9% pid 49235 2h30m: MARK=teau2;",
		"13 more like these",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("a skipped pass must say %q:\n%s", want, out)
		}
	}
	// One printf, under the load it explains: a concurrent gather() must not
	// be able to land between the witness and the leaks it is about.
	witness := strings.Index(out, "pass skipped")
	report := strings.Index(out, "orphaned gate-shell children")
	if witness < 0 || report < witness {
		t.Errorf("the report must follow the witness it explains:\n%s", out)
	}
	if strings.Count(out[witness:report], "\n") != 2 { // witness, culprits, report
		t.Errorf("the report belongs under the culprit line, with nothing between:\n%s", out)
	}
}

// §2: it runs ONLY when the guard is already over the limit. A healthy
// fleet pass must not fork `ps` — that is the cost this feature promised
// never to charge.
func TestAHealthyPassNeverReadsTheProcessTable(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","priority":1,"labels":["go"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)

	reads := 0
	b.App.Load1 = func() (float64, error) { return 2.5, nil }
	b.App.TopCPU = func() ([]Proc, error) { reads++; return procRows(), nil }

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a healthy pass: %v", err)
	}
	if reads != 0 {
		t.Errorf("a pass under the limit read the process table %d time(s):\n%s", reads, dispatcherOut(d))
	}
	if strings.Contains(dispatcherOut(d), "top CPU") {
		t.Errorf("a healthy pass named culprits it has no reason to look for:\n%s", dispatcherOut(d))
	}
}

// ─── the orphan report, arm 1 (ranger-base-apwr) ────────────────────────────
//
// Nothing on this box ends a leaked gate-shell child. A non-interactive zsh
// does not hang up its background jobs, so a Bash line that backgrounds
// something and returns leaves it running with launchd as its parent; the
// load guard then correctly declines to launch into the wreckage and waits
// forever, because the wreckage has no living parent and nothing is looking
// for it. That is teau's 2.5h freeze. Arm 1 makes it loud and kills nothing.

// gateArgv is what a forked gate-shell child carries: the shell, the -c, our
// whole preamble (with the per-persona middle that cannot be matched
// literally), and then the persona's own command.
func gateArgv(payload string) string {
	return "/bin/zsh -c " + gateShellPreambleHead +
		`_rge=${_rgr%%:*}; _rgr=${_rgr#*:}; case "$_rge" in ''|*/gates/*) ;; *) _rgp="$_rgp:$_rge";; esac; done; PATH="/h/state/gates/developer/bin$_rgp"` +
		gateShellPreambleTail + payload
}

const teauPayload = `MARK=teau2; for i in 1 2 3 4 5 6 7 8; do (while :; do :; done) & done; spins=$(jobs -p); sleep 3; uptime`

// leakRows is the incident: two batches of eight, orphaned, still burning.
func leakRows(n int) []Proc {
	var rows []Proc
	for i := 0; i < n; i++ {
		rows = append(rows, Proc{
			PID: 49235 + i, PPID: 1, CPU: 36.9 - float64(i), Comm: "zsh",
			Age: 2*time.Hour + 30*time.Minute, Args: gateArgv(teauPayload),
		})
	}
	return rows
}

func TestOrphanReportNamesTheLeakedGateShellChildren(t *testing.T) {
	a := NewAppAt(t.TempDir())
	a.TopCPU = func() ([]Proc, error) { return leakRows(16), nil }
	got := a.LoadCulpritLine()

	for _, want := range []string{
		"16 orphaned gate-shell children (ppid 1, over 20% CPU, over 1m)",
		"REPORT ONLY, nothing was killed",
		"36.9% pid 49235 2h30m: MARK=teau2; for i in 1 2 3 4 5 6 7 8;", // pid, CPU, age, payload
		"13 more like these",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the orphan report must carry %q:\n%s", want, got)
		}
	}
	// The payload is the persona's command — our own preamble is what the
	// operator already knows and is exactly what cost teau its first hour.
	if strings.Contains(got, gateShellPreambleHead) || strings.Contains(got, "_rgp") {
		t.Errorf("the report must show the payload AFTER the preamble, not the preamble:\n%s", got)
	}
	// Three named outright, the rest counted: a log line, not a listing.
	if n := strings.Count(got, "2h30m:"); n != loadOrphanTop {
		t.Errorf("want %d orphans named outright, got %d:\n%s", loadOrphanTop, n, got)
	}
	// It rides under the culprit line, off the same one census reading.
	if !strings.HasPrefix(got, "\n  load guard: top CPU:") {
		t.Errorf("the culprit line must still come first:\n%s", got)
	}
	// A long command is cut, and cut visibly.
	long := leakRows(1)
	long[0].Args = gateArgv(strings.Repeat("x", 400))
	a.TopCPU = func() ([]Proc, error) { return long, nil }
	one := a.LoadCulpritLine()
	if !strings.Contains(one, strings.Repeat("x", loadOrphanPayload)+"…") ||
		strings.Contains(one, strings.Repeat("x", loadOrphanPayload+1)) {
		t.Errorf("the payload must be cut to %d and marked:\n%s", loadOrphanPayload, one)
	}
	if !strings.Contains(one, "1 orphaned gate-shell child (") || strings.Contains(one, "more like these") {
		t.Errorf("one leak is one child, singular, with nothing left to count:\n%s", one)
	}
}

// Everything the predicate must NOT fire on. Each row here is a top CPU
// burner, so each one reaches the culprit line and none may reach the report.
func TestOrphanReportSkipsWhatIsNotALeak(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  Proc
	}{
		{"a busy child of a live session, gate shell and all", Proc{
			PID: 812, PPID: 4021, CPU: 99, Age: time.Hour, Comm: "zsh", Args: gateArgv(teauPayload)}},
		{"an orphan under the CPU floor", Proc{
			PID: 813, PPID: 1, CPU: LoadCulpritOrphanCPU - 0.1, Age: time.Hour, Comm: "zsh", Args: gateArgv(teauPayload)}},
		{"an orphan younger than the age floor", Proc{
			PID: 814, PPID: 1, CPU: 99, Age: LoadOrphanMinAge - time.Second, Comm: "zsh", Args: gateArgv(teauPayload)}},
		{"the operator's own orphaned build", Proc{
			PID: 815, PPID: 1, CPU: 99, Age: time.Hour, Comm: "go", Args: "go build ./..."}},
		{"an orphan whose argv we could not read", Proc{
			PID: 816, PPID: 1, CPU: 99, Age: time.Hour, Comm: "zsh", Args: ""}},
		// The teau misreading in a new costume: a process whose argv TALKS
		// about the preamble is not a process that came out of one.
		{"something merely holding our preamble in its argv", Proc{
			PID: 817, PPID: 1, CPU: 99, Age: time.Hour, Comm: "grep",
			Args: "grep -n " + gateShellPreambleHead + " internal/posse/gates.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAppAt(t.TempDir())
			a.TopCPU = func() ([]Proc, error) { return []Proc{tc.row}, nil }
			got := a.LoadCulpritLine()
			if !strings.Contains(got, "top CPU:") {
				t.Fatalf("the row must still reach the culprit line, or this proves nothing: %q", got)
			}
			if strings.Contains(got, "orphaned gate-shell chil") {
				t.Errorf("this is not a leaked gate-shell child:\n%s", got)
			}
		})
	}
}

func TestGateShellForkPayloadOpensWithThePreamble(t *testing.T) {
	for _, tc := range []struct {
		name string
		args string
		want string
		ours bool
	}{
		{"a forked child of a gated -c line", gateArgv(teauPayload), teauPayload, true},
		{"the -c string on its own, no argv0", gateArgv(teauPayload)[len("/bin/zsh -c "):], teauPayload, true},
		{"nothing of ours in it", "node /x/server.js", "", false},
		// A -c ANYWHERE ahead of it is not enough: what must precede the
		// preamble is the shell's own `-c `, immediately.
		{"our preamble, but not at the head of a command string",
			"cat /tmp/gates.log " + gateShellPreambleHead + "_rge=${_rgr%%:*}" + gateShellPreambleTail + "rm -rf /", "", false},
		{"a head with no tail: ours, but we cannot see where our text stops",
			"/bin/zsh -c " + gateShellPreambleHead + "_rge=${_rgr%%:*}", "", true},
		{"an empty argv", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ours := gateShellForkPayload(tc.args)
			if ours != tc.ours || got != tc.want {
				t.Errorf("gateShellForkPayload(...) = %q, %v; want %q, %v", got, ours, tc.want, tc.ours)
			}
		})
	}
}

// The second read is scoped to the suspects, and that scope is the whole
// reason it costs nothing: on a box that is loaded but not leaking there is
// no second fork at all.
func TestOrphanSuspectPIDsScopeTheSecondRead(t *testing.T) {
	// A loaded box with nothing reparented takes no second read at all,
	// which is what keeps the extra fork off every healthy box.
	var busy []Proc
	for i := 0; i < 8; i++ {
		busy = append(busy, Proc{PID: 900 + i, PPID: 4021, CPU: 99, Age: time.Hour, Comm: "go"})
	}
	if ids := orphanSuspectPIDs(busy); ids != nil {
		t.Errorf("a loaded box with no orphan must take no second read, got %v", ids)
	}
	// Each half of the cheap predicate excludes on its own.
	busy = append(busy,
		Proc{PID: 910, PPID: 1, CPU: LoadCulpritOrphanCPU - 0.1, Age: time.Hour, Comm: "zsh"},
		Proc{PID: 911, PPID: 1, CPU: 99, Age: LoadOrphanMinAge - time.Second, Comm: "zsh"},
		Proc{PID: 912, PPID: 1, CPU: LoadCulpritOrphanCPU, Age: LoadOrphanMinAge, Comm: "zsh"},
		Proc{PID: 913, PPID: 1, CPU: 36.9, Age: 2*time.Hour + 30*time.Minute, Comm: "zsh"},
	)
	got := orphanSuspectPIDs(busy)
	if len(got) != 2 || got[0] != "912" || got[1] != "913" {
		t.Errorf("only the suspects may have their argv read, and both floors are inclusive; got %v", got)
	}
}

func TestParseArgsTable(t *testing.T) {
	got := parseArgsTable("" +
		"49235 /bin/zsh -c echo  hi   there\n" +
		"  812 node /x/server.js\n" +
		"\n" +
		"nopid something\n" +
		"999\n")
	want := map[int]string{
		49235: "/bin/zsh -c echo  hi   there", // the command's own spacing survives
		812:   "node /x/server.js",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %v", len(got), len(want), got)
	}
	for pid, w := range want {
		if got[pid] != w {
			t.Errorf("pid %d = %q, want %q", pid, got[pid], w)
		}
	}
}

// The predicate against a REAL argv, produced by the real gate shell on the
// box this suite is running on: render the wrapper, run a gated -c line that
// forks a subshell and returns, then read that subshell's argv with the same
// `ps` the census uses and hand it to the same predicate.
//
// It is the argv half of the control, and it is the half that can be taken
// safely anywhere: the child blocks on a fifo instead of burning a core, so
// nothing here loads the box. The ppid-1 and CPU halves need a leak that is
// really orphaned and really spinning, which is scripts/verify-orphan-report.sh
// in a CPU-limited container, per the operator's ruling on ranger-base-teau.
//
// A detector that has never fired has not been shown able to fire, and the
// arm that shows it CAN fail is right below: the same shell, the same fork,
// the same `ps` read, with the wrapper taken out of the line.
func TestGateShellForkArgvIsRecognisedForReal(t *testing.T) {
	if _, err := exec.LookPath("ps"); err != nil {
		t.Skipf("no ps: %v", err)
	}
	dir := t.TempDir()
	gatesDir := filepath.Join(dir, "gates")
	wrapper, err := writeGateShell("developer", gatesDir, filepath.Join(gatesDir, "bin"), "/bin/sh", "sh")
	if err != nil {
		t.Fatalf("writeGateShell: %v", err)
	}

	// fork(2) with no exec(2) is the whole mechanism: the child keeps its
	// parent's -c string, preamble and all. `read` is a builtin, so the
	// subshell blocks in ITSELF rather than in a grandchild — nothing to
	// leak but the one process this test opens the fifo to release. stdout
	// is redirected because a backgrounded child inherits the pipe Output()
	// is waiting on, and that wait would never end (teau RCA, probe note).
	fifo := filepath.Join(dir, "hold")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	payload := "( read x < " + fifo + " ) >/dev/null 2>&1 & echo $!"

	fork := func(shell string) (int, string) {
		t.Helper()
		out, err := exec.Command(shell, "-c", payload).Output()
		if err != nil {
			t.Fatalf("%s -c: %v", shell, err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			t.Fatalf("%s printed no pid: %q", shell, out)
		}
		t.Cleanup(func() {
			// Release it the way it is meant to end, and kill only if that
			// did not take. O_NONBLOCK so an already-dead child cannot hang
			// this open forever.
			if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				f.Write([]byte("\n"))
				f.Close()
			}
			syscall.Kill(pid, syscall.SIGKILL)
		})
		table, err := exec.Command("ps", "-ww", "-o", "pid=,args=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			t.Fatalf("ps -p %d: %v", pid, err)
		}
		args := parseArgsTable(string(table))[pid]
		if args == "" {
			t.Fatalf("ps said nothing about pid %d: %q", pid, table)
		}
		return pid, args
	}

	_, gated := fork(wrapper)
	got, ours := gateShellForkPayload(gated)
	if !ours {
		t.Fatalf("a real forked child of a real gate shell was not recognised as ours:\n%s", gated)
	}
	if got != payload {
		t.Errorf("the payload after the preamble must be the persona's own command:\n got  %q\n want %q\n argv %s", got, payload, gated)
	}

	// The wrong arm. Same shell, same fork, same read — no wrapper, so no
	// preamble, so not ours. Without this the pin above is green over a
	// predicate that could be answering "yes" to everything.
	if _, ungated := fork("/bin/sh"); func() bool { _, o := gateShellForkPayload(ungated); return o }() {
		t.Errorf("a fork of an UNGATED shell must not be ours:\n%s", ungated)
	}
}
