package posse

// What these pin (ranger-base-d3fwo): claude 2.1.258 asks, once per config
// dir, "Allow reads outside the working directories?" — mid-turn, on the
// first file-tool read past the working directories of a session already in
// auto mode. A persona cannot answer it. One sat blocked on it reading a
// runbook out of the instance tree until the coordinator sent the keystroke
// by hand.
//
// The launch answers it the way the accept arm of the dialog does: by
// setting hasSeenAutoModeOutsideReadPrompt in the config claude reads.
// That is a record of an answer, not a permission decision — which is the
// half of this the settings key cannot do, and the reason these tests also
// pin what the fleet's --settings must NOT carry.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fresh case: a HOME that has never run claude gets both answers, not
// one. Before this, a launch answered the modal and left the notice armed.
func TestSeedAnswersTheOutsideReadNoticeForAFreshConfig(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	dir := t.TempDir()

	if _, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir); err != nil {
		t.Fatal(err)
	}
	state := readConfig(t, cfg)
	if !claudeOutsideReadSeen(state) {
		t.Errorf("%s not seeded: %v", ClaudeOutsideReadSeenKey, state)
	}
	// Top level, not under projects[dir]: the CLI asks this once per config
	// dir, and a per-project spelling of it would be seeded into the one dir
	// the launch happened to open in and armed everywhere else.
	if e := project(t, state, dir); e[ClaudeOutsideReadSeenKey] != nil {
		t.Errorf("the notice is not a per-project key; it must not be written under projects[dir]: %v", e)
	}
}

// The half the already-trusted test used to hide: a dir claude trusts, in a
// config that has never seen the notice, is still a launch with something
// to write. Getting this wrong is the exact failure the bead reports —
// every long-lived fleet dir is already trusted.
func TestSeedWritesForATrustedDirWhoseNoticeIsUnanswered(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	dir := t.TempDir()
	writeConfig(t, cfg, map[string]any{
		"userID": "kept",
		"projects": map[string]any{
			ClaudeTrustKey(dir): map[string]any{"hasTrustDialogAccepted": true},
		},
	})

	wrote, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != cfg {
		t.Fatalf("seed reported %q; a trusted dir with the notice unanswered is still a write", wrote)
	}
	state := readConfig(t, cfg)
	if !claudeOutsideReadSeen(state) {
		t.Errorf("%s not seeded: %v", ClaudeOutsideReadSeenKey, state)
	}
	if state["userID"] != "kept" {
		t.Errorf("the operator's state was not merged: %v", state)
	}
}

// And the mirror: the notice answered, the dir untrusted. Each key is
// written when it is the one missing, so neither seed can be reached only
// through the other.
func TestSeedWritesTrustWhenOnlyTheNoticeIsAnswered(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	dir := t.TempDir()
	writeConfig(t, cfg, map[string]any{ClaudeOutsideReadSeenKey: true})

	wrote, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != cfg {
		t.Fatalf("seed reported %q; an untrusted dir is a write whatever the notice says", wrote)
	}
	if !ClaudeTrusted(readConfig(t, cfg), dir) {
		t.Error("the trust key was not written")
	}
}

// Read it the way claude reads it — strictly true. A false, or a value of
// another type, is a config that still arms the notice, so it is one the
// launch has something to write. Read it as "present" instead and a config
// carrying `false` silently stays armed.
func TestOutsideReadNoticeIsReadStrictlyTrue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		set  any
	}{
		{"false", false},
		{"string true", "true"},
		{"number", float64(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if claudeOutsideReadSeen(map[string]any{ClaudeOutsideReadSeenKey: tc.set}) {
				t.Fatalf("%v must not read as answered", tc.set)
			}
			cfg := filepath.Join(t.TempDir(), ".claude.json")
			dir := t.TempDir()
			writeConfig(t, cfg, map[string]any{
				ClaudeOutsideReadSeenKey: tc.set,
				"projects":               map[string]any{ClaudeTrustKey(dir): map[string]any{"hasTrustDialogAccepted": true}},
			})
			if _, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir); err != nil {
				t.Fatal(err)
			}
			if !claudeOutsideReadSeen(readConfig(t, cfg)) {
				t.Errorf("a %s value was left unanswered", tc.name)
			}
		})
	}
}

// Nothing to do stays nothing to do: both answered is the only pair that
// writes no file. The already-trusted test asserts the file is untouched;
// this one asserts the conjunction is what decides it.
func TestSeedWritesNothingWhenBothAreAnswered(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	dir := t.TempDir()
	writeConfig(t, cfg, map[string]any{
		ClaudeOutsideReadSeenKey: true,
		"projects":               map[string]any{ClaudeTrustKey(dir): map[string]any{"hasTrustDialogAccepted": true}},
	})
	st, _ := os.Stat(cfg)
	time.Sleep(10 * time.Millisecond)

	wrote, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != "" {
		t.Errorf("seed reported a write (%s) with both questions already answered", wrote)
	}
	if after, _ := os.Stat(cfg); !after.ModTime().Equal(st.ModTime()) {
		t.Error("the config was rewritten with nothing to add")
	}
}

// A cage HOME is fresh by construction, so it is the tier where the notice
// is guaranteed to be armed — and the tier where no operator is going to
// answer it by hand.
func TestSeedCageHomeAnswersTheOutsideReadNotice(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	ag := cageAgent(t, a, "")
	claude, _ := a.LoadRuntime("claude")
	home, err := a.SeedCageHome(ag, claude, "/work/repo")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	state := map[string]any{}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	if !claudeOutsideReadSeen(state) {
		t.Errorf("%s not seeded into the cage HOME: %v", ClaudeOutsideReadSeenKey, state)
	}
}

// The key the SCREEN names is not the silence, and the fleet's --settings
// must not carry it in either value: `false` leaves the notice armed
// (claude's guard tests strictly true), `true` silences it by refusing the
// read it was asking about. Both were measured on 2.1.258; shipping the
// first would look settled on the launch line and change nothing.
func TestFleetSettingsDoesNotShipTheReadBlockKey(t *testing.T) {
	t.Parallel()
	const key = "blockReadsOutsideWorkingDirectories"
	for _, s := range []string{ClaudeFleetSettings, DefaultAgentCommand} {
		if strings.Contains(s, key) {
			t.Errorf("%s must not name %s — see ClaudeOutsideReadSeenKey (trust.go) for why neither value settles the notice:\n%s", key, key, s)
		}
	}
	// And the settings payload is still parseable JSON naming the mode
	// layer, so this pin cannot be satisfied by emptying it.
	var m map[string]any
	if err := json.Unmarshal([]byte(ClaudeFleetSettings), &m); err != nil {
		t.Fatalf("fleet settings must stay parseable JSON: %v", err)
	}
	perms, _ := m["permissions"].(map[string]any)
	if perms["defaultMode"] != "auto" {
		t.Errorf("the mode layer is gone from the settings payload: %v", m)
	}
}

// `posse runtime check` has to name this screen: it is the second key posse
// WRITES rather than names-and-refuses, and an undeclared exception is what
// the interstitial table exists to prevent.
func TestClaudeDeclaresTheOutsideReadNoticeItSeeds(t *testing.T) {
	t.Parallel()
	rt := claudeRuntime(t)
	var in *Interstitial
	for i := range rt.Interstitials {
		if strings.Contains(rt.Interstitials[i].Key, ClaudeOutsideReadSeenKey) {
			in = &rt.Interstitials[i]
		}
	}
	if in == nil {
		t.Fatalf("claude declares no outside-read screen, so `runtime check` still claims posse writes no such key: %+v", rt.Interstitials)
	}
	if !in.Seeded {
		t.Error("the notice is seeded by the launch; the row must say so")
	}
	if in.Probe == nil {
		t.Error("the row needs a probe: the operator's question is whether this box would draw it today")
	}
	if in.Danger != "" {
		t.Error("Danger means LAUNCH REFUSE until the operator silences it — this one the launch answers")
	}
	// The row has to carry the trap, or an operator reading it reaches for
	// the key the dialog itself names and ships a no-op.
	if !strings.Contains(in.Silence, "blockReadsOutsideWorkingDirectories") {
		t.Errorf("the row must name the settings key that is NOT the silence: %q", in.Silence)
	}
}

// Probes read the operator's config and never create or touch it — the
// same rule the grok/codex probes are held to.
func TestOutsideReadProbeReadsAndDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", home)

	if s := claudeOutsideReadProbe(); !s.Unknown {
		t.Errorf("no config at all must read as unknown, not as answered/unanswered: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err == nil {
		t.Error("the probe created the operator's config")
	}

	cfg := filepath.Join(home, ".claude.json")
	writeConfig(t, cfg, map[string]any{"userID": "u"})
	if s := claudeOutsideReadProbe(); s.Silenced || s.Unknown {
		t.Errorf("an unanswered config must read as not silenced: %+v", s)
	}
	writeConfig(t, cfg, map[string]any{ClaudeOutsideReadSeenKey: true})
	if s := claudeOutsideReadProbe(); !s.Silenced {
		t.Errorf("an answered config must read as silenced: %+v", s)
	}
}
