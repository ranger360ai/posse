package rhq

// The pulse's sensing half (ADR 0027 §1-2, rangerhq-4ish): condition set,
// fingerprint, arm switch. Delivery (prompting monica) is rangerhq-44w1;
// its tests live in pulse_delivery_test.go.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadPulseConfigUnarmedByDefault(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte("default_persona: monica\n"), 0o644)
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
		"pulse_interval: 2m\npulse_persona: dinesh\npulse_renag: 30m\npulse_renag_max: 4h\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Armed || cfg.Interval != 2*time.Minute || cfg.Persona != "dinesh" ||
		cfg.Renag != 30*time.Minute || cfg.RenagMax != 4*time.Hour {
		t.Errorf("bad armed config: %+v", cfg)
	}
}

func TestLoadPulseConfigDefaultPersona(t *testing.T) {
	a := wtApp(t)
	os.WriteFile(a.ConfigPath, []byte("pulse_interval: 30s\n"), 0o644)
	cfg, err := LoadPulseConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Persona != DefaultPulsePersona {
		t.Errorf("persona = %q, want default %q", cfg.Persona, DefaultPulsePersona)
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

func TestShopCheckBlockedSession(t *testing.T) {
	b, fake := newTestBackend(t)
	blockedSession(t, b, fake, "monica-shop", "monica")

	conditions, err := ShopCheck(b, nil, "monica")
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(conditions, "blocked:monica-shop") {
		t.Errorf("conditions = %v, want blocked:monica-shop", conditions)
	}
	// The session is blocked, not gone — it must not also read as absent.
	if containsPrefix(conditions, "no-live:") {
		t.Errorf("a blocked session is still live: %v", conditions)
	}
}

func TestShopCheckNoLivePersona(t *testing.T) {
	b, _ := newTestBackend(t)
	conditions, err := ShopCheck(b, nil, "monica")
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(conditions, "no-live:monica") {
		t.Errorf("conditions = %v, want no-live:monica", conditions)
	}
}

func TestShopCheckLivePersonaOtherAgentDoesNotCount(t *testing.T) {
	b, _ := newTestBackend(t)
	writePersona(t, b.App, "richard", "code")
	mustCreate(t, b, NewSessionOpts{Name: "richard-work", Agent: "richard"})
	conditions, err := ShopCheck(b, nil, "monica")
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(conditions, "no-live:monica") {
		t.Errorf("richard's session must not stand in for monica: %v", conditions)
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

	conditions, err := ShopCheck(b, []string{repo}, "monica")
	if err != nil {
		t.Fatal(err)
	}
	if !containsPrefix(conditions, "unpushed:"+repo+":") {
		t.Errorf("conditions = %v, want an unpushed: entry for %s", conditions, repo)
	}
}

func TestShopCheckNoUpstreamIsNoCondition(t *testing.T) {
	b, _ := newTestBackend(t)
	repo := wtRepo(t) // one commit, no remote, no upstream configured

	conditions, err := ShopCheck(b, []string{repo}, "monica")
	if err != nil {
		t.Fatal(err)
	}
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
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	blockedSession(t, b, fake, "monica-shop", "monica")

	noUpstream := wtRepo(t) // deterministic: no upstream, so no unpushed: noise
	os.WriteFile(b.App.ConfigPath, []byte(
		"pulse_interval: 15ms\npulse_persona: monica\nbeads:\n  - "+noUpstream+"\n"), 0o644)

	tap := newPassTap(1)
	d.Out = tap

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 20*time.Millisecond, 40*time.Millisecond); done <- p }()

	deadline := time.After(30 * time.Second)
	for {
		if strings.Contains(tap.String(), "pulse: blocked:monica-shop") {
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
	if !strings.Contains(string(state), "blocked:monica-shop") {
		t.Errorf("state/pulse.yaml missing the condition:\n%s", state)
	}
}

func TestWatchPulseUnarmedNoTicker(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	blockedSession(t, b, fake, "monica-shop", "monica")
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
