package rhq

// rangerhq-9py0: codex draws an "Update available!" menu on a version delta,
// before the composer exists, and stock detection reports it `idle` with no
// rule matched — the same fallthrough rangerhq-7ia closed for "Hooks need
// review". That answer is worse here than a typed-into modal, because the
// menu's default-selected option runs a package upgrade of the operator's
// pinned tooling (ADR 0013 §2 layer 2, Interstitial.Danger).
//
// Two assertions, and the second is the one that matters: the committed
// manifest names the screen `blocked` via `update_menu`, and the SAME capture
// falls back to `idle` when that rule is removed. Without the second, a rule
// that never fires would pass — the manifest's other blockers are keyed on
// footers this screen does not draw, so a green fixture alone proves nothing
// about which rule earned it.
//
// ranger-base-stbt (verify) added the third arm. update_menu is an `all` of
// two clauses and only the banner was exercised, so the "1. Update now" half
// could be deleted with all three packages green — while deleting it turns a
// passive update notice into a refused launch. The second test below holds it.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQACodexUpdateMenuIsBlockedByItsOwnRule(t *testing.T) {
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "..", "..")
	// The one capture, shared with scripts/verify-detection.sh rather than
	// copied: two fixtures of the same screen drift apart, and the drift is
	// invisible until the day one of them is the only one anybody ran.
	fixture := filepath.Join(repo, "etc", "herdr", "agent-detection", "testdata", "codex", "blocked-update-menu.txt")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(repo, "etc", "herdr", "agent-detection", "codex.toml"))
	if err != nil {
		t.Fatal(err)
	}

	// Executing the CHECKOUT's manifest, not the installed fleet override: a
	// committed detection fix must prove itself before an operator deploys it
	// (the shape splashwide_qa_test.go established).
	explain := func(t *testing.T, toml string) (state, rule, fallback string) {
		t.Helper()
		return codexExplain(t, fixture, toml)
	}

	state, rule, fallback := explain(t, string(manifest))
	if state != "blocked" {
		t.Errorf("state=%q (rule %q, fallback %q), want blocked — a pane on this menu takes no prompt and its default option upgrades the operator's codex", state, rule, fallback)
	}
	if rule != "update_menu" {
		t.Errorf("rule=%q fallback=%q, want update_menu — the screen must be named by the rule written for it, not by a footer rule that happens to overlap", rule, fallback)
	}

	// The inverted half: delete update_menu and the very same capture must go
	// back to the unrecognized-idle it arrived as. A rule that can be removed
	// with the fixture still green is a rule pinning nothing (ranger-base flz7).
	without := cutRule(t, string(manifest), "update_menu")
	state, rule, fallback = explain(t, without)
	if state != "idle" || rule != "" {
		t.Errorf("without update_menu: state=%q rule=%q fallback=%q, want idle with no rule — "+
			"if some other rule already named this screen, update_menu is dead weight and this test is not pinning the fix", state, rule, fallback)
	}
}

// cutRule removes one `[[rules]]` block by id, with the comment block that
// introduces it — the manifest's own layout, one blank-line-separated stanza
// per rule.
func cutRule(t *testing.T, toml, id string) string {
	t.Helper()
	stanzas := strings.Split(toml, "\n\n")
	kept := make([]string, 0, len(stanzas))
	cut := 0
	for _, s := range stanzas {
		if strings.Contains(s, "[[rules]]") && strings.Contains(s, `id = "`+id+`"`) {
			cut++
			continue
		}
		kept = append(kept, s)
	}
	if cut != 1 {
		t.Fatalf("cutRule(%q): removed %d stanzas, want exactly 1 — the manifest's layout moved under this test", id, cut)
	}
	return strings.Join(kept, "\n\n")
}

// codexExplain runs herdr's detection over one capture with `toml` staged as
// the codex override, and reports what it decided. Package-level so both
// tests in this file drive detection through exactly one implementation.
func codexExplain(t *testing.T, fixture, toml string) (state, rule, fallback string) {
	t.Helper()
	config := t.TempDir()
	overrides := filepath.Join(config, "herdr", "agent-detection")
	if err := os.MkdirAll(overrides, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overrides, "codex.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("herdr", "agent", "explain", "--file", fixture, "--agent", "codex", "--json")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+config, "XDG_STATE_HOME="+filepath.Join(config, "state"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("herdr agent explain: %v\n%s", err, out)
	}
	var det struct {
		State       string `json:"state"`
		Fallback    string `json:"fallback_reason"`
		MatchedRule *struct {
			ID string `json:"id"`
		} `json:"matched_rule"`
	}
	// --file is a bare detection object; a pane explain wraps it in
	// {"result":...}. Accept both, as splashwide does.
	var wrap struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(out, &wrap); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	raw := out
	if len(wrap.Result) > 0 && wrap.Result[0] == '{' {
		raw = wrap.Result
	}
	if err := json.Unmarshal(raw, &det); err != nil {
		t.Fatalf("detection json: %v\n%s", err, out)
	}
	if det.MatchedRule != nil {
		rule = det.MatchedRule.ID
	}
	return det.State, rule, det.Fallback
}

// TestQACodexUpdateBannerWithoutTheMenuMustNotBlock pins the half of
// update_menu that the rule's own author argued for and no test held.
//
// update_menu is an `all` of two clauses: the "Update available!" banner AND
// the numbered "1. Update now" option. Only the banner is exercised above —
// drop the second clause and every assertion in this file still passes, which
// is the ranger-base-ntsz shape: an arm nothing holds. It is load-bearing.
// codex draws the banner in two places, and only one of them is a menu that
// takes the keyboard; the other is a passive notice sitting above a live
// composer. Blocking on that one costs a launch: on the argv ladder gather
// prints "⛔ blocked in <session> — intervene" and on the typed ladder
// awaitAgent dies "never settled idle", so a box merely carrying an update
// notice would refuse every codex launch.
//
// The capture is DERIVED from the committed menu capture rather than written
// here: the same bytes with the three numbered options and their footer cut
// away is exactly "the banner without the menu", and it cannot drift from the
// real screen the way a hand-typed second fixture would.
func TestQACodexUpdateBannerWithoutTheMenuMustNotBlock(t *testing.T) {
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "..", "..")
	det := filepath.Join(repo, "etc", "herdr", "agent-detection")
	menu, err := os.ReadFile(filepath.Join(det, "testdata", "codex", "blocked-update-menu.txt"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(det, "codex.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var kept []string
	for _, line := range strings.Split(string(menu), "\n") {
		switch {
		case strings.Contains(line, "Update now"),
			strings.Contains(line, "Skip"),
			strings.Contains(line, "Press enter to continue"):
			continue
		}
		kept = append(kept, line)
	}
	banner := strings.Join(kept, "\n")
	// A negative control is satisfied by a fixture that was never built, so
	// say out loud what this one still is and what it no longer is.
	if !strings.Contains(banner, "Update available!") {
		t.Fatalf("derived capture lost the banner — it is no longer the screen under test:\n%s", banner)
	}
	if strings.Contains(banner, "Update now") {
		t.Fatalf("derived capture still carries the menu, so it cannot show the banner alone:\n%s", banner)
	}
	fixture := filepath.Join(t.TempDir(), "idle-update-banner-no-menu.txt")
	if err := os.WriteFile(fixture, []byte(banner), 0o644); err != nil {
		t.Fatal(err)
	}

	state, rule, fallback := codexExplain(t, fixture, string(manifest))
	if state == "blocked" {
		t.Errorf("banner without the menu: state=%q rule=%q, want not-blocked — "+
			"a passive update notice above a live composer takes prompts fine, and "+
			"refusing it costs every launch on a box that merely has an update pending", state, rule)
	}
	if rule != "" {
		t.Errorf("banner without the menu matched rule %q (state %q, fallback %q), want no rule", rule, state, fallback)
	}

	// The positive witness: the SAME capture must go blocked once the
	// "1. Update now" clause is gone. Without this, the assertion above is
	// green for a manifest in which update_menu never fires at all.
	loose := cutUpdateNowClause(t, string(manifest))
	state, rule, _ = codexExplain(t, fixture, loose)
	if state != "blocked" || rule != "update_menu" {
		t.Errorf("with the \"1. Update now\" clause cut, the banner-only capture read state=%q rule=%q, "+
			"want blocked/update_menu — if it does not, that clause is not what keeps a passive "+
			"notice out of update_menu and this test is pinning the wrong thing", state, rule)
	}
}

// cutUpdateNowClause drops the second `all` clause of update_menu, leaving the
// bare-banner rule the test above proves is wrong.
func cutUpdateNowClause(t *testing.T, toml string) string {
	t.Helper()
	var kept []string
	cut, inRule := 0, false
	for _, line := range strings.Split(toml, "\n") {
		if strings.Contains(line, `id = "update_menu"`) {
			inRule = true
		} else if strings.HasPrefix(line, "[[rules]]") {
			inRule = false
		}
		if inRule && strings.Contains(line, "regex") && strings.Contains(line, "Update now") {
			cut++
			continue
		}
		kept = append(kept, line)
	}
	if cut != 1 {
		t.Fatalf("cutUpdateNowClause: removed %d clauses, want exactly 1 — update_menu's shape moved under this test", cut)
	}
	return strings.Join(kept, "\n")
}
