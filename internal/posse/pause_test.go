package posse

// PAUSE (ADR 0029 §3, bead rangerhq-a2g6): the file, who may write it, the
// gate at every launch path, and the half that must NOT stop — pause stops
// spend, not oversight.
//
// The observables the design predicted for the verify bead are the last
// three tests here: a paused shop launches zero sessions and names the
// pauser, and a paused shop with a blocked session still pulses the
// coordinator.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var pauseAt = time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)

// pausedShop writes the file the way `posse pause` would, without going
// through the command.
func pausedShop(t *testing.T, a *App, by, why string) Pause {
	t.Helper()
	p, err := WritePause(a, by, why, pauseAt, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// ─── who may pause ───────────────────────────────────────────────────────────

// §3: "Who may pause: the operator, and the coordinator." Everyone else is
// refused, and the refusal names the two — a stop the rest of the fleet can
// write is a stop any one of them can sit in.
func TestPauseActorAuthority(t *testing.T) {
	for _, tc := range []struct {
		name    string
		persona string
		coord   string
		want    string // "" = refused
		refusal string
	}{
		{"a shell with no persona is the operator", "", "coordinator", PauseOperator, ""},
		{"the operator, with no coordinator configured", "", "", PauseOperator, ""},
		{"the coordinator pauses as herself", "coordinator", "coordinator", "coordinator", ""},
		{"any other persona is refused", "developer", "coordinator", "", "the operator's and coordinator's"},
		{"and with no coordinator, so is every persona", "developer", "", "", "pausing is the operator's alone"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAppAt(t.TempDir())
			if tc.coord != "" {
				os.WriteFile(a.ConfigPath, []byte("coordinator: "+tc.coord+"\n"), 0o644)
			}
			t.Setenv(EnvPersona, tc.persona)
			by, err := PauseActor(a)
			if tc.want != "" {
				if err != nil || by != tc.want {
					t.Fatalf("PauseActor = (%q, %v), want %q", by, err, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("PauseActor allowed %q to pause (as %q)", tc.persona, by)
			}
			if !strings.Contains(err.Error(), tc.persona) || !strings.Contains(err.Error(), tc.refusal) {
				t.Errorf("the refusal must name who asked and who may: %v", err)
			}
		})
	}
}

// ─── the file ────────────────────────────────────────────────────────────────

func TestWritePauseRoundTrips(t *testing.T) {
	a := NewAppAt(t.TempDir())
	p := pausedShop(t, a, "coordinator", "the security lane found a live key")
	if !p.Present || p.By != "coordinator" || p.Why != "the security lane found a live key" {
		t.Fatalf("stored pause = %+v", p)
	}
	if p.At != "2026-08-27T09:00:00Z" {
		t.Errorf("at = %q, want the handed-in clock in UTC RFC3339", p.At)
	}
	// The read side (govern.go, G8) is the one that matters: every pass and
	// every rendering asks the file this way.
	if got := ReadPause(PausePath(a)); got != p {
		t.Errorf("ReadPause = %+v, want %+v", got, p)
	}
}

// The why is mandatory: it is what every declining pass prints, and the
// file shape is what makes "pauses with a recorded why: 100%" a metric.
func TestWritePauseRefusesAReasonlessStop(t *testing.T) {
	for _, why := range []string{"", "   ", "\n\t "} {
		a := NewAppAt(t.TempDir())
		if _, err := WritePause(a, PauseOperator, why, pauseAt, os.Stderr); err == nil {
			t.Errorf("WritePause(%q) must refuse a stop with no reason", why)
		}
		if ReadPause(PausePath(a)).Present {
			t.Errorf("WritePause(%q) refused and still stopped the shop", why)
		}
	}
}

// A why is free text a human types in a hurry, and the reader every pass
// uses is line-based. Newlines are flattened — which also closes the
// "why: ...\nby: someone-else" injection — and what the writer cannot fix
// it says out loud, because a why that comes back shorter than it was typed
// is the surface quietly editing the reason the shop stopped.
func TestWritePauseFlattensAndReportsWhatItStored(t *testing.T) {
	a := NewAppAt(t.TempDir())
	var warn strings.Builder
	p, err := WritePause(a, PauseOperator, "key rotation\nby: someone-else\n  spans lines", pauseAt, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if p.By != PauseOperator {
		t.Errorf("a multi-line why must not be able to write by:, got %q", p.By)
	}
	if p.Why != "key rotation by: someone-else spans lines" {
		t.Errorf("why = %q, want it flattened to one line", p.Why)
	}
	if warn.Len() > 0 {
		t.Errorf("a why the reader gives back verbatim needs no warning: %q", warn.String())
	}

	// The one the writer cannot fix: " #" is a comment to the reader.
	a2 := NewAppAt(t.TempDir())
	warn.Reset()
	p2, err := WritePause(a2, PauseOperator, "rollout #3 broke the meter", pauseAt, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Why != "rollout" {
		t.Fatalf("why = %q — this test exists because the reader truncates there", p2.Why)
	}
	if !strings.Contains(warn.String(), `"rollout"`) {
		t.Errorf("a truncated reason must be reported, said: %q", warn.String())
	}
	// A stop must never fail over its own formatting.
	if !p2.Present {
		t.Error("the shop must be paused even when the reason did not survive the round trip")
	}
}

// Resume is an off switch, and an off switch that can fail is one more
// thing to get right while the shop is stopped.
func TestClearPauseIsIdempotent(t *testing.T) {
	a := NewAppAt(t.TempDir())
	if p, err := ClearPause(a); err != nil || p.Present {
		t.Fatalf("resuming an unpaused shop = (%+v, %v), want no pause and no error", p, err)
	}
	pausedShop(t, a, "coordinator", "waiting on the operator")
	p, err := ClearPause(a)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Present || p.By != "coordinator" {
		t.Errorf("resume must name the pause it lifted, got %+v", p)
	}
	if _, err := os.Stat(PausePath(a)); !os.IsNotExist(err) {
		t.Errorf("the pause file is still there after resume: %v", err)
	}
	if p, err := ClearPause(a); err != nil || p.Present {
		t.Fatalf("the second resume = (%+v, %v), want no pause and no error", p, err)
	}
}

// One vocabulary: the line a declining pass prints is the same text G8
// renders in `posse status` and the cockpit. A pause named two ways is a
// pause somebody has to correlate.
func TestPauseLineIsG8sOwnWords(t *testing.T) {
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.StateDir, 0o755)
	p := pausedShop(t, b.App, "coordinator", "the security lane found a live key")
	line := PauseLine(p)
	for _, want := range []string{"paused", "coordinator", "the security lane found a live key", pauseAt.Format(time.RFC3339)} {
		if !strings.Contains(line, want) {
			t.Errorf("the pause line must carry %q: %q", want, line)
		}
	}
	g := find(shopSet(t, govIn(t, b)), "G8")
	if g == nil || g.Detail != line {
		t.Errorf("G8 detail %q and the pass's line %q must be the same words", g.Detail, line)
	}
}

// ─── the pass gate ───────────────────────────────────────────────────────────

// pauseRepo is a one-bead queue the fake bd serves, wired into config.
func pauseRepo(t *testing.T, b *HerdrBackend, fake string) string {
	t.Helper()
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","priority":1,"labels":["go"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"fix the thing","status":"closed"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	return repo
}

// The design's first predicted observable: `posse pause` then a hand-typed
// `posse dispatch` launches zero sessions and names the pauser.
func TestDispatchDeclinesThePassWhilePaused(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	pauseRepo(t, b, fake)
	pausedShop(t, b.App, "coordinator", "waiting on the operator")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("a declined pass is not a failed pass: %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 dispatched, got %d:\n%s", n, dispatcherOut(d))
	}
	out := dispatcherOut(d)
	for _, want := range []string{"paused", "coordinator", "waiting on the operator", "posse resume"} {
		if !strings.Contains(out, want) {
			t.Errorf("the decline must carry %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "paused") != 1 {
		t.Errorf("one witness line per declined pass:\n%s", out)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a paused shop launches nothing:\n%s", log)
	}
	// The gate is ahead of every reading the pass forks for — the ready
	// scan, verify-after and the bead-loss census all run bd.
	if log, err := os.ReadFile(filepath.Join(fake, "bd-calls.log")); err == nil {
		t.Errorf("a declined pass must not fork bd:\n%s", log)
	}
}

// --dry-run reports and gets out of the way, for the load guard's own
// reason: the one command someone runs to ask "what would happen if I
// resumed" must not be the one that goes quiet.
func TestDryRunSaysThePassWouldDeclineAndShowsRoutingAnyway(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	pauseRepo(t, b, fake)
	pausedShop(t, b.App, "coordinator", "waiting on the operator")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "paused") || !strings.Contains(out, "coordinator") {
		t.Errorf("--dry-run must still say the shop is paused:\n%s", out)
	}
	if !strings.Contains(out, "a-1") {
		t.Errorf("--dry-run must still show routing:\n%s", out)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("--dry-run launches nothing, paused or not:\n%s", log)
	}
}

func TestDispatchFiresAgainAfterResume(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	pauseRepo(t, b, fake)
	pausedShop(t, b.App, "coordinator", "waiting on the operator")
	if n, _ := d.Run("", "", 0); n != 0 {
		t.Fatalf("paused: want 0 dispatched, got %d", n)
	}
	if _, err := ClearPause(b.App); err != nil {
		t.Fatal(err)
	}
	n, err := d.Run("", "", 0)
	if err != nil || n != 1 {
		t.Fatalf("after resume: n=%d err=%v\n%s", n, err, dispatcherOut(d))
	}
}

// The cockpit's `d` is a launcher too, and a stop the operator can walk
// around by pressing a key is not a stop.
func TestLaunchBeadRefusesWhilePaused(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := pauseRepo(t, b, fake)
	pausedShop(t, b.App, "coordinator", "waiting on the operator")

	_, err := d.LaunchBead(RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "fix the thing", Labels: []string{"go"}}, Dir: repo})
	if err == nil {
		t.Fatal("the cockpit's d must not launch into a paused shop")
	}
	for _, want := range []string{"paused", "coordinator", "waiting on the operator"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name the pauser and the reason: %v", err)
		}
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a refused launch must not reach herdr:\n%s", log)
	}
}

// §3: "No condition may auto-PAUSE." Latching a transient reading into a
// durable stop that needs a human to clear trades a self-healing skip for a
// flapping meter parking the shop overnight. The mechanism SKIPs — here the
// load guard, whose skip is the loudest one a pass can take — and the file
// stays a human's to write.
func TestNoMechanismEverWritesThePauseFile(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	pauseRepo(t, b, fake)
	b.App.Load1 = func() (float64, error) { return 263, nil }

	if n, err := d.Run("", "", 0); n != 0 || err != nil {
		t.Fatalf("the guard must skip the pass: n=%d err=%v", n, err)
	}
	if !strings.Contains(dispatcherOut(d), "pass skipped") {
		t.Fatalf("the fixture did not make the mechanism stop the shop:\n%s", dispatcherOut(d))
	}
	if p := ReadPause(PausePath(b.App)); p.Present {
		t.Errorf("a mechanical stop latched into a durable pause: %+v", p)
	}
}

// ─── pause stops spend, not oversight ────────────────────────────────────────

// The design's other predicted observable, and the whole reason the
// coordinator reaches for pause instead of `kill`: a paused shop with a
// blocked session still pulses her. The gate is in Run; the pulse goroutine
// is started outside it and never consults the pause file.
func TestAPausedShopStillPulses(t *testing.T) {
	b, fake := newTestBackend(t)
	// A blocked session (G1), and the coordinator's own idle one for the
	// pulse to deliver into. Both statuses have to be in one agents.json —
	// each helper rewrites the whole listing.
	blocked := personaSession(t, b, fake, "developer-work", "developer", "blocked", false)
	target := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(fmt.Sprintf(
		`[{"agent":"claude","agent_status":"blocked","pane_id":%q,"workspace_id":%q},`+
			`{"agent":"claude","agent_status":"idle","pane_id":%q,"workspace_id":%q}]`,
		blocked+":p1", blocked, target+":p1", target)), 0o644)
	govRepo(t, b)
	pausedShop(t, b.App, "coordinator", "waiting on the operator")

	d := newTestDispatcher(t, b)
	d.pulseOnce(PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute, RenagMax: 4 * time.Hour})

	log := calls(t, fake)
	if !strings.Contains(log, "agent prompt "+target+":p1") {
		t.Fatalf("a paused shop stopped escalating — that is `kill`, not pause:\n%s", log)
	}
	// And the pause itself is one of the things it escalates (G8), beside
	// the condition that has nothing to do with the pause.
	set := shopSet(t, d.govInputs(PulseConfig{Armed: true, Persona: "coordinator"}))
	if !set.Has("G8") || !set.Has("G1") {
		t.Errorf("a paused shop with a blocked session must report both: %v", set.Keys())
	}
}
