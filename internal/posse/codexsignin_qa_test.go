//go:build posse_arm3

package posse

// ranger-base-n6s2u: a codex whose credentials are missing or expired draws a
// SIGN-IN menu at startup — "1. Sign in with ChatGPT / 2. Sign in with Device
// Code / 3. Provide your own API key", footed "Press enter to continue", with
// no composer anywhere beneath it. Stock detection matched nothing and fell
// through to default_known_agent_idle_fallback: `idle`, rule none — the same
// hole rangerhq-7ia (hooks_review) and rangerhq-9py0 (update_menu) were
// written to close, and the reason `blocked` is the honest answer here is
// measured on live 0.153.4 panes rather than assumed (the rangerhq-1xsj
// distinction; the measurement is in etc/herdr/agent-detection/codex.toml and
// the README beside it).
//
// What these tests hold, in the order the fix can rot:
//
//  1. the committed manifest names both screens by their own rules, and the
//     same captures fall back to unrecognized-idle when those rules are cut —
//     without the inversion a rule that never fires would pass (ranger-base
//     flz7, and rangerhq-9py0 before it);
//  2. each `all` clause of signin_menu is load-bearing, drop-one and with a
//     positive witness that the drop is what did it — otherwise half the rule
//     could be deleted with every package green (the ranger-base-ntsz shape);
//  3. the region depth. signin_menu reads 24 non-empty lines where its
//     siblings read 20, because codex draws the menu under a 15-line ASCII
//     logo and a 60-column pane wraps the prose above it. That number is the
//     one thing here with an edge, so it is mutated at its edge rather than
//     trusted: testdata/codex/blocked-signin-narrow.txt is the pane that needs
//     the last two lines of it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func detectionDir(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "..", "..", "etc", "herdr", "agent-detection")
}

func codexManifest(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(detectionDir(t), "codex.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestQACodexSignInScreensAreBlockedByTheirOwnRules(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	manifest := codexManifest(t)
	dir := filepath.Join(detectionDir(t), "testdata", "codex")

	// The captures are shared with scripts/verify-detection.sh rather than
	// copied, for codexupdatemenu's reason: two fixtures of one screen drift
	// apart, and the drift is invisible until one of them is the only one
	// anybody ran.
	for _, tc := range []struct{ fixture, rule string }{
		{"blocked-signin.txt", "signin_menu"},
		{"blocked-signin-narrow.txt", "signin_menu"},
		{"blocked-signin-api-key.txt", "signin_api_key"},
	} {
		fixture := filepath.Join(dir, tc.fixture)
		if _, err := os.Stat(fixture); err != nil {
			t.Fatal(err)
		}
		state, rule, fallback := codexExplain(t, fixture, manifest)
		if state != "blocked" {
			t.Errorf("%s: state=%q (rule %q, fallback %q), want blocked — this pane takes no prompt: "+
				"text sent to it is discarded, there is no composer beneath, and a bare digit starts a sign-in",
				tc.fixture, state, rule, fallback)
		}
		if rule != tc.rule {
			t.Errorf("%s: rule=%q fallback=%q, want %s — the screen must be named by the rule written for it, "+
				"not by a footer rule that happens to overlap", tc.fixture, rule, fallback, tc.rule)
		}

		// The inverted half: cut the rule and the very same capture must go
		// back to the unrecognized-idle it arrived as.
		state, rule, fallback = codexExplain(t, fixture, cutRule(t, manifest, tc.rule))
		if state != "idle" || rule != "" {
			t.Errorf("%s without %s: state=%q rule=%q fallback=%q, want idle with no rule — if some other rule "+
				"already named this screen, %s is dead weight and this test pins nothing",
				tc.fixture, tc.rule, state, rule, fallback, tc.rule)
		}
	}
}

// TestQACodexSignInMenuClausesAreBothLoadBearing drops one numbered option at
// a time out of the capture. signin_menu is an `all` of two clauses — the
// "1. Sign in with ChatGPT" option and the "2. Sign in with Device Code" one —
// and a rule keyed on either alone is a rule that fires on any screen quoting
// one line of this menu.
//
// Each arm carries its positive witness: the same trimmed capture must go
// blocked again once the matching clause is cut from the rule. Without that,
// an arm passes for a capture that stopped being the screen at all — a
// misaimed probe reading as a kill.
func TestQACodexSignInMenuClausesAreBothLoadBearing(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	manifest := codexManifest(t)
	capture, err := os.ReadFile(filepath.Join(detectionDir(t), "testdata", "codex", "blocked-signin.txt"))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, option, clause, keep string }{
		{"no ChatGPT option", "1. Sign in with ChatGPT", "Sign in with ChatGPT", "2. Sign in with Device Code"},
		{"no device-code option", "2. Sign in with Device Code", "Sign in with Device Code", "1. Sign in with ChatGPT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var kept []string
			cut := 0
			for _, line := range strings.Split(string(capture), "\n") {
				if strings.Contains(line, tc.option) {
					cut++
					continue
				}
				kept = append(kept, line)
			}
			if cut != 1 {
				t.Fatalf("removed %d lines matching %q, want exactly 1 — the capture moved under this test", cut, tc.option)
			}
			trimmed := strings.Join(kept, "\n")
			// Say what the derived capture still is: a negative control is
			// satisfied by a fixture that was never the screen.
			if !strings.Contains(trimmed, tc.keep) {
				t.Fatalf("derived capture lost %q as well — it is no longer half of the menu:\n%s", tc.keep, trimmed)
			}
			fixture := filepath.Join(t.TempDir(), "signin-one-option.txt")
			if err := os.WriteFile(fixture, []byte(trimmed), 0o644); err != nil {
				t.Fatal(err)
			}

			state, rule, fallback := codexExplain(t, fixture, manifest)
			if rule == "signin_menu" {
				t.Errorf("half the menu (%s) still matched signin_menu (state %q) — that clause is not checked, "+
					"and the rule fires on any screen carrying the other line", tc.name, state)
			}

			// The positive witness: with the clause for the REMOVED option
			// cut, the same capture must be blocked by signin_menu again.
			loose := cutSignInClause(t, manifest, tc.clause)
			state, rule, fallback = codexExplain(t, fixture, loose)
			if state != "blocked" || rule != "signin_menu" {
				t.Errorf("with the %q clause cut, the half-menu capture read state=%q rule=%q fallback=%q, "+
					"want blocked/signin_menu — if it does not, the arm above failed for some other reason "+
					"and this subtest is pinning the wrong thing", tc.clause, state, rule, fallback)
			}
		})
	}
}

// cutSignInClause drops one of signin_menu's two `all` clauses by the option
// text it matches on.
func cutSignInClause(t *testing.T, toml, clause string) string {
	t.Helper()
	var kept []string
	cut, inRule := 0, false
	for _, line := range strings.Split(toml, "\n") {
		if strings.Contains(line, `id = "signin_menu"`) {
			inRule = true
		} else if strings.HasPrefix(line, "[[rules]]") {
			inRule = false
		}
		if inRule && strings.Contains(line, "regex") && strings.Contains(line, clause) {
			cut++
			continue
		}
		kept = append(kept, line)
	}
	if cut != 1 {
		t.Fatalf("cutSignInClause(%q): removed %d clauses, want exactly 1 — signin_menu's shape moved under this test", clause, cut)
	}
	return strings.Join(kept, "\n")
}

// TestQACodexSignInMenuRegionReachesTheNarrowPane mutates the one number in
// this fix that has an edge.
//
// signin_menu reads top_non_empty_lines(24). Its siblings read 20, and 20 is
// not enough: codex draws the menu under a 15-line ASCII logo, so on a
// 120-column pane "2. Sign in with Device Code" is already the 21st non-empty
// line, and on a 60-column pane — where the two prose lines above it wrap —
// it is the 23rd. Both fixtures must fail at 20, and the narrow one must fail
// at 22, or the 24 is a number nobody measured and a manifest edit that walks
// it back to the sibling value goes green.
func TestQACodexSignInMenuRegionReachesTheNarrowPane(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	manifest := codexManifest(t)
	dir := filepath.Join(detectionDir(t), "testdata", "codex")

	narrow := func(toml string, n string) string {
		t.Helper()
		var kept []string
		cut, inRule := 0, false
		for _, line := range strings.Split(toml, "\n") {
			if strings.Contains(line, `id = "signin_menu"`) {
				inRule = true
			} else if strings.HasPrefix(line, "[[rules]]") {
				inRule = false
			}
			if inRule && strings.HasPrefix(line, "region = ") {
				line = `region = "top_non_empty_lines(` + n + `)"`
				cut++
			}
			kept = append(kept, line)
		}
		if cut != 1 {
			t.Fatalf("rewrote %d region lines, want exactly 1 — signin_menu's shape moved under this test", cut)
		}
		return strings.Join(kept, "\n")
	}

	for _, tc := range []struct {
		region  string
		fixture string
		want    string
	}{
		{"20", "blocked-signin.txt", "idle"},
		{"20", "blocked-signin-narrow.txt", "idle"},
		// 22 reaches option 2 on the 120-column pane and not on the wrapped
		// one, which is the whole reason the narrow capture is committed.
		{"22", "blocked-signin.txt", "blocked"},
		{"22", "blocked-signin-narrow.txt", "idle"},
		{"24", "blocked-signin.txt", "blocked"},
		{"24", "blocked-signin-narrow.txt", "blocked"},
	} {
		state, rule, fallback := codexExplain(t, filepath.Join(dir, tc.fixture), narrow(manifest, tc.region))
		if state != tc.want {
			t.Errorf("region %s on %s: state=%q rule=%q fallback=%q, want %s",
				tc.region, tc.fixture, state, rule, fallback, tc.want)
		}
		if tc.want == "blocked" && rule != "signin_menu" {
			t.Errorf("region %s on %s: rule=%q, want signin_menu", tc.region, tc.fixture, rule)
		}
	}
}
