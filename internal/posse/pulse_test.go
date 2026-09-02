package posse

// The pulse's sensing half (ADR 0027 §1-2, rangerhq-4ish): condition set,
// fingerprint, arm switch. Delivery (prompting coordinator) is rangerhq-44w1;
// its tests live in pulse_delivery_test.go.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoadPulseConfigUnarmedByDefault(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte("default_persona: coordinator\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Armed {
		t.Errorf("no pulse_interval: key must not arm the pulse, got %+v", cfg)
	}
}

func TestLoadPulseConfigArmed(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte(
		"pulse_interval: 2m\npulse_persona: developer\npulse_renag: 30m\npulse_renag_max: 4h\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Armed || cfg.Interval != 2*time.Minute || cfg.Persona != "developer" ||
		cfg.Renag != 30*time.Minute || cfg.RenagMax != 4*time.Hour {
		t.Errorf("bad armed config: %+v", cfg)
	}
}

// An unset pulse_persona: falls back to the instance's coordinator: and to
// nothing else — the engine ships no persona name of its own (ranger-base-q3gp,
// App.Coordinator's rangerhq-gk4k rule). Both arms matter: the fallback that
// fires, and the one that has nowhere to fall.
func TestLoadPulseConfigDefaultPersonaIsTheCoordinator(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte("pulse_interval: 30s\ncoordinator: product\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Persona != "product" {
		t.Errorf("persona = %q, want the configured coordinator %q", cfg.Persona, "product")
	}
}

func TestLoadPulseConfigDefaultPersonaEmptyWithoutCoordinator(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte("pulse_interval: 30s\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Persona != "" {
		t.Errorf("persona = %q, want \"\" — no coordinator: means no compiled-in target", cfg.Persona)
	}
	if !cfg.Armed {
		t.Error("an armed pulse with no target is still armed; it senses and delivers to nobody")
	}
}

// pulse_persona: still wins over coordinator: — the fallback is a default,
// not an override.
func TestLoadPulseConfigPersonaBeatsCoordinator(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte("pulse_interval: 30s\ncoordinator: product\npulse_persona: qa\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Persona != "qa" {
		t.Errorf("persona = %q, want the explicit pulse_persona: %q", cfg.Persona, "qa")
	}
}

func TestLoadPulseConfigBadInterval(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte("pulse_interval: not-a-duration\n"), 0o644)
	if _, err := LoadPulseConfig(a); err == nil {
		t.Error("bad pulse_interval: must error, not silently disarm or default")
	}
}

// blockedSession creates a live posse session for persona and marks its
// workspace's herdr agent status blocked, the fixture ADR 0027 §1's
// condition (a) reads.
func blockedSession(t *testing.T, b *HerdrBackend, fake, name, persona string) {
	t.Helper()
	writePersona(t, b.App, persona, "code")
	mustCreate(t, b, NewSessionOpts{Name: name, Agent: persona})
	ws := fakeLoadWSFrom(t, fake)
	var id string
	for _, w := range ws {
		if w.Label == name {
			id = w.WorkspaceID
		}
	}
	if id == "" {
		t.Fatalf("no workspace created for session %q: %+v", name, ws)
	}
	agents := fmt.Sprintf(`[{"agent":"claude","agent_status":"blocked","pane_id":%q,"workspace_id":%q}]`, id+":p1", id)
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(agents), 0o644)
}

// pulseIn is the pulse tick's inputs against a test backend: armed, no bd
// (the fake bd is per-repo and these fixtures configure no beads dir), no
// streak, no ledger scan. The G-table rows have their own file.
func pulseIn(t *testing.T, b *HerdrBackend, dirs []string, persona string) GovInputs {
	t.Helper()
	if dirs == nil {
		dirs = []string{t.TempDir()}
	}
	writeBeadsDirs(b.App, dirs)
	// The fake bd, serving no fixtures: the G-table's bd rows come back
	// empty rather than unknown, so these tests assert on the carry-overs
	// alone. Without it BeadsDirs falls back to the process cwd and the
	// real bd answers from the operator's own queue.
	t.Setenv("RHQ_BD_BIN", fakeBinFor(t, "bd"))
	return GovInputs{App: b.App, HB: b, Bd: NewBd(), PulsePersona: persona, Pulsing: true}
}

func TestShopCheckBlockedSession(t *testing.T) {
	b, fake := newTestBackend(t)
	blockedSession(t, b, fake, "coordinator-shop", "coordinator")

	conditions := shopKeys(t, pulseIn(t, b, nil, "coordinator"))
	if !containsStr(conditions, "blocked:coordinator-shop") {
		t.Errorf("conditions = %v, want blocked:coordinator-shop", conditions)
	}
	// The session is blocked, not gone — it must not also read as absent.
	if containsPrefix(conditions, "no-live:") {
		t.Errorf("a blocked session is still live: %v", conditions)
	}
}

func TestShopCheckNoLivePersona(t *testing.T) {
	b, _ := newTestBackend(t)
	conditions := shopKeys(t, pulseIn(t, b, nil, "coordinator"))
	if !containsStr(conditions, "no-live:coordinator") {
		t.Errorf("conditions = %v, want no-live:coordinator", conditions)
	}
}

// The carry-over is a fact about DELIVERY, so a shop with no pulse armed is
// not missing anything by it — and `posse status` must not go non-zero on
// every box where the coordinator's session happens to be closed.
func TestShopCheckNoLivePersonaOnlyWhenPulsing(t *testing.T) {
	b, _ := newTestBackend(t)
	in := pulseIn(t, b, nil, "coordinator")
	in.Pulsing = false
	if conditions := shopKeys(t, in); containsPrefix(conditions, "no-live:") {
		t.Errorf("a disarmed pulse has nowhere to deliver and nothing to report: %v", conditions)
	}
}

func TestShopCheckLivePersonaOtherAgentDoesNotCount(t *testing.T) {
	b, _ := newTestBackend(t)
	writePersona(t, b.App, "architect", "code")
	mustCreate(t, b, NewSessionOpts{Name: "architect-work", Agent: "architect"})
	conditions := shopKeys(t, pulseIn(t, b, nil, "coordinator"))
	if !containsStr(conditions, "no-live:coordinator") {
		t.Errorf("architect's session must not stand in for coordinator: %v", conditions)
	}
}

func TestShopCheckUnpushedCommits(t *testing.T) {
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	bare := t.TempDir()
	mustGit(t, bare, "init", "-q", "--bare")
	mustGit(t, repo, "remote", "add", "origin", bare)
	mustGit(t, repo, "push", "-q", "-u", "origin", "main")
	commitIn(t, repo, "extra.txt", "x", "extra")

	conditions := shopKeys(t, pulseIn(t, b, []string{repo}, "coordinator"))
	if !containsPrefix(conditions, "unpushed:"+repo+":") {
		t.Errorf("conditions = %v, want an unpushed: entry for %s", conditions, repo)
	}
}

func TestShopCheckNoUpstreamIsNoCondition(t *testing.T) {
	b, _ := newTestBackend(t)
	repo := wtRepo(t) // one commit, no remote, no upstream configured

	conditions := shopKeys(t, pulseIn(t, b, []string{repo}, "coordinator"))
	if containsPrefix(conditions, "unpushed:") {
		t.Errorf("no upstream must read as no condition, got %v", conditions)
	}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func containsPrefix(ss []string, prefix string) bool {
	for _, s := range ss {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// The watch loop: armed pulses onto its own output and state/pulse.yaml;
// unarmed starts no ticker at all. rangerhq-4ish's "done when": an armed
// watch logs conditions on a fake-herdr blocked session.
func TestWatchPulseArmedLogsBlockedSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	blockedSession(t, b, fake, "coordinator-shop", "coordinator")

	noUpstream := wtRepo(t) // deterministic: no upstream, so no unpushed: noise
	os.WriteFile(b.App.ConfigPath, []byte(
		"pulse_interval: 15ms\npulse_persona: coordinator\nbeads:\n  - "+noUpstream+"\n"), 0o644)

	tap := newPassTap(1)
	d.Out = tap

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	deadline := time.After(30 * time.Second)
	for {
		if strings.Contains(tap.String(), "pulse: blocked:coordinator-shop") {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("pulse never logged the blocked session:\n%s", tap.String())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("watch never returned after cancel")
	}

	state, err := os.ReadFile(PulsePath(b.App))
	if err != nil {
		t.Fatalf("state/pulse.yaml not written: %v", err)
	}
	if !strings.Contains(string(state), "blocked:coordinator-shop") {
		t.Errorf("state/pulse.yaml missing the condition:\n%s", state)
	}
}

func TestWatchPulseUnarmedNoTicker(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	blockedSession(t, b, fake, "coordinator-shop", "coordinator")
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	// No pulse_interval: key at all — disarmed.
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	const wantPasses = 2
	tap := newPassTap(wantPasses)
	d.Out = tap

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("watch never returned")
	}

	if strings.Contains(tap.String(), "pulse:") {
		t.Errorf("unarmed watch must never log a pulse line:\n%s", tap.String())
	}
	if _, err := os.Stat(PulsePath(b.App)); err == nil {
		t.Error("unarmed watch must never write state/pulse.yaml")
	}
}

// parkTap is a dispatcher sink that holds the FIRST writer of a "pulse: "
// line — pulseOnce is the only thing in the package that writes one — and
// lets every other write through untouched. Parking there parks the pulse
// goroutine INSIDE pulseOnce, which is the state the flake needed: a tick
// mid-flight, past its state/pulse.yaml write or about to take it, while
// the loop that owns it is asked to stop.
type parkTap struct {
	mu      sync.Mutex
	buf     strings.Builder
	once    sync.Once
	freed   sync.Once
	parked  chan struct{} // closed when a tick is held
	release chan struct{} // closed by the test to let it go
}

// let releases a parked tick. Idempotent: the test calls it where it means
// to, and defers it so no tick is ever left holding a goroutine.
func (p *parkTap) let() { p.freed.Do(func() { close(p.release) }) }

func newParkTap() *parkTap {
	return &parkTap{parked: make(chan struct{}), release: make(chan struct{})}
}

func (p *parkTap) Write(b []byte) (int, error) {
	p.mu.Lock()
	n, err := p.buf.Write(b)
	p.mu.Unlock()
	if strings.Contains(string(b), "pulse: ") {
		held := false
		p.once.Do(func() { held = true; close(p.parked) })
		if held {
			<-p.release
		}
	}
	return n, err
}

func (p *parkTap) String() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.String()
}

// Watch owns the pulse ticker, so Watch must not return while one of its
// ticks is still running (ranger-base-el3g). Cancelling the context only
// ASKS the ticker to stop; a tick already inside pulseOnce runs to the end
// and writes state/pulse.yaml on its way out. A Watch that returned on the
// cancel alone therefore left a goroutine writing this instance's state/
// after every caller — the CLI, and a test whose StateDir is a t.TempDir —
// believed the loop was over. That is what failed as
// "TempDir RemoveAll cleanup: ... state: directory not empty": RemoveAll
// unlinked pulse.yaml and the abandoned tick put it back before the rmdir.
//
// The pin holds a tick in flight on its own log line, cancels, and asserts
// Watch is still in Watch. It cannot go red on a slow box: with the join,
// Watch cannot return until the release below, whatever the load.
func TestWatchWaitsForAPulseTickInFlight(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	blockedSession(t, b, fake, "coordinator-shop", "coordinator")

	noUpstream := wtRepo(t) // deterministic: no upstream, so no unpushed: noise
	os.WriteFile(b.App.ConfigPath, []byte(
		"pulse_interval: 15ms\npulse_persona: coordinator\nbeads:\n  - "+noUpstream+"\n"), 0o644)

	tap := newParkTap()
	d.Out = tap

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer tap.let() // never leave the tick parked, however this ends
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond)
	}()

	select {
	case <-tap.parked:
	case <-time.After(30 * time.Second):
		t.Fatalf("the pulse never logged a condition to park on:\n%s", tap.String())
	}

	cancel()
	select {
	case <-done:
		t.Fatal("Watch returned with a pulse tick still in flight: the tick goes on writing state/pulse.yaml after the caller believes the loop is over (ranger-base-el3g)")
	case <-time.After(500 * time.Millisecond):
	}

	tap.let()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Watch never returned after the parked pulse tick was released")
	}
}
