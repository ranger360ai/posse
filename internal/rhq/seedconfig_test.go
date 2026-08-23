package rhq

// The seed config (examples/config.yaml) is what `posse init` copies verbatim
// into every fresh instance, so it is the de facto instance spec — and the
// highest-leverage place for one of this instance's facts to escape into
// everyone else's (ADR 0012 Appendix A 4). Two properties are worth a test
// rather than a reviewer's eye:
//
//  1. It ARMS NOTHING. Every key that changes dispatch's behaviour ships
//     commented out, so a fresh instance's first pass does what the harness
//     defaults do and the deployer turns things on deliberately. Uncommenting
//     one key by accident while editing this file is the regression.
//  2. It names no machine. An absolute home path in the seed is an instance
//     fact that would follow the file onto every other laptop.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedConfigPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "examples", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("no seed config at %s: %v", p, err)
	}
	return p
}

// The keys the seed is allowed to set live, and nothing else: session
// cosmetics and the picker's roots. Everything with teeth — routing,
// tiering, ceilings, the arm switch — is documented in comments and unset.
func TestSeedConfigArmsNothing(t *testing.T) {
	cfg := seedConfigPath(t)

	for _, key := range []string{
		"operator", "coordinator", "default_persona",
		"default_runtime", "default_tier", "default_engine", "cage_image",
		"verify_assignee",
		"plan_guard_5h", "plan_guard_7d", "plan_guard_blind_max",
		"plan_guard_overflow", "plan_guard_overflow_cap",
		"budget_pass", "budget_day",
		"autostart_interval", "autostart_max_interval", "autostart_max_beads",
		"autostart_dry_run", "autostart_session", "autostart_dir",
	} {
		if v := YamlGet(cfg, key); v != "" {
			t.Errorf("seed config sets %s: %q — the seed must ship it commented out", key, v)
		}
	}

	// Keys whose *presence* is the switch (a present-but-empty key means
	// something different from an absent one), so absence is the assertion.
	for _, key := range []string{"beads", "orientation", "verify_labels", "tier_by_label", "metric_ids"} {
		if yamlHasKey(cfg, key) {
			t.Errorf("seed config declares %s: — a present key overrides the harness default; keep it commented", key)
		}
	}

	// autostart_interval's presence alone arms the unattended loop, so it
	// gets the stronger check: not merely empty, absent.
	if yamlHasKey(cfg, "autostart_interval") {
		t.Error("seed config declares autostart_interval: — presence is the arm switch; a fresh instance must ship disarmed")
	}
}

// The picker roots and the emoji map are the seed's live content; they must
// still work, and still name nobody's machine.
func TestSeedConfigNamesNoMachine(t *testing.T) {
	cfg := seedConfigPath(t)

	if len(YamlList(cfg, "dirs")) == 0 {
		t.Error("seed config has no dirs: roots — the TUI picker would offer nothing")
	}
	if len(YamlMapPairs(cfg, "emoji")) == 0 {
		t.Error("seed config has no emoji: map")
	}

	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, abs := range []string{"/Users/", "/home/", "/var/folders/"} {
		if strings.Contains(string(b), abs) {
			t.Errorf("seed config contains the absolute path prefix %q — paths here must be ~-relative or a placeholder", abs)
		}
	}
}
