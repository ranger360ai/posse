package posse

// The seed config (examples/config.yaml) is what `posse init` copies verbatim
// into every fresh instance, so it is the de facto instance spec — and the
// highest-leverage place for one of this instance's facts to escape into
// everyone else's (ADR 0012 Appendix A 4). Three properties are worth a test
// rather than a reviewer's eye:
//
//  1. It ARMS NOTHING. Every key that changes dispatch's behaviour ships
//     commented out, so a fresh instance's first pass does what the harness
//     defaults do and the deployer turns things on deliberately. Uncommenting
//     one key by accident while editing this file is the regression.
//  2. It names no machine. An absolute home path in the seed is an instance
//     fact that would follow the file onto every other laptop.
//  3. Every key it DOES ship armed is read by something. An armed key no
//     code looks up is documentation of a feature that is not there
//     (ranger-base-aox).

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedConfigPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(qibRepoRoot(t), "examples", "config.yaml")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("no seed config at %s: %v", p, err)
	}
	return p
}

// The keys the seed is allowed to set live, and nothing else: session
// cosmetics. Everything with teeth — routing, tiering, ceilings, the arm
// switch — is documented in comments and unset.
func TestSeedConfigArmsNothing(t *testing.T) {
	t.Parallel()
	cfg := seedConfigPath(t)

	for _, key := range []string{
		"operator", "coordinator", "default_persona",
		// instance: renames every workspace this home creates in herdr's one
		// shared list (rangerhq-ouf9). A seed that shipped it set would tag a
		// single-instance fleet for a coexistence it does not have.
		"instance",
		"default_runtime", "default_tier", "default_engine", "cage_image",
		"verify_assignee",
		// verify_batch: is the gate's ratio and therefore the operator's
		// call (ranger-base-bah7 decision 2): a seed that shipped N>1 would
		// change how much gets verified per bead without anyone deciding to.
		"verify_batch", "verify_batch_age",
		"plan_guard_5h", "plan_guard_7d", "plan_guard_blind_max",
		"budget_pass", "budget_day",
		// dispatch_epoch: denominates both of the caps above and
		// autostart_max_beads: below (ADR 0028 §2). A seed that shipped it
		// set would change how much spend authority and how many launches a
		// fresh instance gets per unit time without anyone deciding to.
		"dispatch_epoch",
		"autostart_interval", "autostart_max_interval", "autostart_max_beads",
		"autostart_dry_run", "autostart_resume", "autostart_session", "autostart_dir",
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

	// autostart_interval's presence alone is the arm switch, so it gets the
	// stronger check: not merely empty, absent. A present-but-empty key is
	// not the middle ground it looks like — the hook refuses it by name and
	// exits 1 rather than reading it as a disarm (ranger-base-cxyk), so a
	// seed that declared it bare would ship a fresh instance whose herdr
	// startup hook fails, not one that is disarmed.
	if yamlHasKey(cfg, "autostart_interval") {
		t.Error("seed config declares autostart_interval: — presence is the arm switch; a fresh instance must ship disarmed")
	}
}

// The emoji map is the seed's live content; it must still work, and still
// name nobody's machine.
func TestSeedConfigNamesNoMachine(t *testing.T) {
	t.Parallel()
	cfg := seedConfigPath(t)

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

// The keys a fresh instance is allowed to ship SET. Shared by the two tests
// below: one says nothing outside this set is declared, the other says
// everything in it is read by the harness.
var seedLiveKeys = map[string]bool{
	"default_dir":   true, // session cosmetics — no dispatch behaviour
	"default_env":   true,
	"default_emoji": true,
	"emoji":         true, // session name → glyph
}

// The two lists above are hand-maintained, which is the failure mode they
// were meant to prevent: a key added to the seed by a later bead is armable
// and silently uncovered. `plan_usage_ttl:` and `beads_visibility:` both
// arrived that way and could be shipped armed with the suite green
// (rangerhq-fpv9). So the real guard is inverted — enumerate the handful of
// keys the seed is ALLOWED to declare, and let everything else fail by
// default. A new key then has exactly two ways past this test: ship it
// commented out, or say here, on purpose, that a fresh instance sets it.
func TestSeedConfigDeclaresOnlyTheLiveKeys(t *testing.T) {
	t.Parallel()
	cfg := seedConfigPath(t)

	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A declared key is one at column 0: `# key:` is documentation, `key:`
	// is configuration. That is the same rule YamlGet/yamlHasKey apply, so
	// this test sees exactly what the harness sees.
	for _, ln := range strings.Split(string(b), "\n") {
		i := strings.Index(ln, ":")
		if i <= 0 || ln[0] == ' ' || ln[0] == '\t' || ln[0] == '#' {
			continue
		}
		key := ln[:i]
		if strings.ContainsAny(key, " \t") {
			continue
		}
		if !seedLiveKeys[key] {
			t.Errorf("seed config declares %s: — a fresh instance must ship it commented out, "+
				"or add it to `seedLiveKeys` and say why arming it by default is right", key)
		}
	}
}

// The other half of the allowlist, and the one `dirs:` failed for two years
// (ranger-base-aox). Being on `seedLiveKeys` says a fresh instance ships this
// key ARMED; that is only defensible if something then reads it. `dirs:` was
// seeded with three roots and a comment describing a TUI directory picker
// this branch does not have — the picker belongs to the tmux-era launcher —
// so every fresh instance was armed with a key no code has ever looked up,
// and both tests above blessed it. It is deleted; this pin is what keeps the
// next one from arriving the same way.
//
// The check is the bead's own repro promoted to an assertion: the harness
// reads config through YamlGet/YamlList/YamlMapPairs/CfgGet, all keyed by a
// string literal, so a live key with no `"key"` literal in any non-test Go
// file is read by nothing. A matching literal is necessary, not sufficient —
// this cannot tell a config read from a same-named recipe field — but the
// failure it catches is total absence, which is the failure that happened.
// Test files are excluded on purpose: a key whose only reader is the test
// asserting it has one would satisfy a scan that included them.
func TestSeedConfigLiveKeysAreRead(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not a hand-rolled `..`/`..` ascent: this walk starts at
	// the repo root, which puts the test in the tree-wide class the door
	// census derives, and that census can only see tests that reach the
	// root through the one helper (ranger-base-sx2dq).
	root := qibRepoRoot(t)

	var src strings.Builder
	scanned := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		src.Write(b)
		scanned++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Positive witness: an empty or mis-rooted walk would pass every
	// assertion below by measuring nothing.
	if scanned < 20 {
		t.Fatalf("scanned %d non-test .go files under %s — the walk found no tree, so the check below measures nothing", scanned, root)
	}
	t.Logf("scanned %d non-test .go files under %s", scanned, root)

	all := src.String()
	for key := range seedLiveKeys {
		if !strings.Contains(all, `"`+key+`"`) {
			t.Errorf("seed config ships %s: armed, but no non-test Go file names %q — "+
				"a key nothing reads must not be in the seed (delete it, or delete it from seedLiveKeys)", key, key)
		}
	}
}
