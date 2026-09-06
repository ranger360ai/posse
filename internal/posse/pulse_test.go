//go:build posse_arm3

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
	t.Parallel()
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
	t.Parallel()
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte(
		"pulse_interval: 2m\npulse_persona: developer\npulse_renag: 45m\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Armed || cfg.Interval != 2*time.Minute || cfg.Persona != "developer" ||
		cfg.Renag != 45*time.Minute {
		t.Errorf("bad armed config: %+v", cfg)
	}
}

// pulse_renag_max: went with the doubling ladder (ADR 0027, 2026-09-05,
// ranger-base-thm0j), and the box it went from still has it in config.yaml.
// A retired key is inert, not an error: the family loads, the one repeat
// interval is the one pulse_renag: names, and nothing anywhere reads a
// maximum. Both halves matter — a LoadPulseConfig that started refusing
// unknown pulse_* keys would stand a live shop's pulse down at upgrade.
func TestLoadPulseConfigIgnoresTheRetiredRenagMax(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte(
		"pulse_interval: 2m\npulse_persona: coordinator\npulse_renag: 20m\npulse_renag_max: 4h\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatalf("a config still carrying pulse_renag_max: must load: %v", err)
	}
	// 20m, not the 30m default: a reader that quietly stopped consulting
	// pulse_renag: would answer the default and pass a weaker assertion.
	if !cfg.Armed || cfg.Renag != 20*time.Minute {
		t.Errorf("the surviving keys must load unchanged: %+v", cfg)
	}
	// The field is gone from the struct, so the pin that it is not consulted
	// has to be written against the file: no reader in the package asks for
	// the key, and DefaultPulseRenag is the only interval left.
	if got := a.CfgGet("pulse_renag_max", "unread"); got == "unread" {
		t.Fatal("fixture: the retired key must actually be in the config for this to pin anything")
	}
	if DefaultPulseRenag != 30*time.Minute {
		t.Errorf("DefaultPulseRenag = %s, want the one 30m repeat interval", DefaultPulseRenag)
	}
}

// An unset pulse_persona: falls back to the instance's coordinator: and to
// nothing else — the engine ships no persona name of its own (ranger-base-q3gp,
// App.Coordinator's rangerhq-gk4k rule). Both arms matter: the fallback that
// fires, and the one that has nowhere to fall.
func TestLoadPulseConfigDefaultPersonaIsTheCoordinator(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	return GovInputs{App: b.App, HB: b, Bd: b.Bd, PulsePersona: persona, Pulsing: true}
}

func TestShopCheckBlockedSession(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	b, _ := newTestBackend(t)
	in := pulseIn(t, b, nil, "coordinator")
	in.Pulsing = false
	if conditions := shopKeys(t, in); containsPrefix(conditions, "no-live:") {
		t.Errorf("a disarmed pulse has nowhere to deliver and nothing to report: %v", conditions)
	}
}

func TestShopCheckLivePersonaOtherAgentDoesNotCount(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	writePersona(t, b.App, "architect", "code")
	mustCreate(t, b, NewSessionOpts{Name: "architect-work", Agent: "architect"})
	conditions := shopKeys(t, pulseIn(t, b, nil, "coordinator"))
	if !containsStr(conditions, "no-live:coordinator") {
		t.Errorf("architect's session must not stand in for coordinator: %v", conditions)
	}
}

func TestShopCheckUnpushedCommits(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := wtRepo(t) // one commit, no remote, no upstream configured

	conditions := shopKeys(t, pulseIn(t, b, []string{repo}, "coordinator"))
	if containsPrefix(conditions, "unpushed:") {
		t.Errorf("no upstream must read as no condition, got %v", conditions)
	}
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

	// The file is delivery bookkeeping and NOTHING else since ADR 0027's
	// 2026-09-05 simplification, so the condition that raised this tick is
	// no longer in it — it is on the watch line asserted above, which is
	// where a human reads it. What the file still proves is the thing the
	// disarmed sibling below asserts the negative of: an armed pulse ran.
	state, err := os.ReadFile(PulsePath(b.App))
	if err != nil {
		t.Fatalf("state/pulse.yaml not written: %v", err)
	}
	// The condition text at all, anywhere: it used to reach the file twice,
	// as a `conditions:` item and inside the `fingerprint:` join.
	if strings.Contains(string(state), "blocked:coordinator-shop") {
		t.Errorf("state/pulse.yaml still carries the observed condition:\n%s", state)
	}
	// And the four removed keys, anchored — "fingerprint:" is a suffix of
	// "prompted_fingerprint:", so a plain Contains would call the surviving
	// field a removed one and pass for the wrong reason.
	for _, gone := range []string{"at:", "conditions:", "fingerprint:", "renag_interval:"} {
		if lineHasKey(string(state), gone) {
			t.Errorf("state/pulse.yaml still carries %q, which ADR 0027 removed:\n%s", gone, state)
		}
	}
	if !lineHasKey(string(state), "prompted_fingerprint:") || !lineHasKey(string(state), "prompted_at:") {
		t.Errorf("state/pulse.yaml must still carry the two delivery fields:\n%s", state)
	}
}

// lineHasKey is "does any line of this YAML start with key" — the anchored
// question, because `fingerprint:` is a suffix of `prompted_fingerprint:`
// and a plain Contains would call the surviving field a removed one.
func lineHasKey(yaml, key string) bool {
	for _, ln := range strings.Split(yaml, "\n") {
		if strings.HasPrefix(ln, key) {
			return true
		}
	}
	return false
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
