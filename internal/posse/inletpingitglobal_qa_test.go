//go:build posse_arm3

package posse

// The GIT_CONFIG_GLOBAL row, for ranger-base-obdnk (from ranger-base-3m6xy).
//
// Three env names steer git's global scope — GIT_CONFIG_GLOBAL,
// XDG_CONFIG_HOME and HOME — and all three ran an attacker's core.hooksPath
// past the pin until this row existed. The row carries the file git resolves
// with the variable unset, so it closes all three without moving the
// operator's identity; on a box with TWO global files it is absent and the
// launch says so, never /dev/null. Every test here runs against a fixture
// HOME, so what is graded is the rule and not whatever this box holds.
//
// Not parallel: inletPin() and DeriveIdentityLiterals read this process's
// own HOME, XDG_CONFIG_HOME and PATH, so the fixtures are t.Setenv.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// globalScopeFixture points this process at a fresh HOME, with XDG_CONFIG_HOME
// and every GIT_CONFIG* off, and writes the global files asked for. Each file
// holds its own user.email, so a scope that went missing shows.
func globalScopeFixture(t *testing.T, homeFile, xdgFile bool) (home, homeCfg, xdgCfg string) {
	t.Helper()
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && strings.HasPrefix(k, "GIT_CONFIG") {
			unsetenvForTest(t, k)
		}
	}
	unsetenvForTest(t, "XDG_CONFIG_HOME")
	home = t.TempDir()
	t.Setenv("HOME", home)
	homeCfg = home + "/.gitconfig"
	xdgCfg = home + "/.config/git/config"
	if homeFile {
		if err := os.WriteFile(homeCfg, []byte("[user]\n\temail = home@example.invalid\n\tname = qa\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if xdgFile {
		if err := os.MkdirAll(filepath.Dir(xdgCfg), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(xdgCfg, []byte("[user]\n\temail = xdg@example.invalid\n[alias]\n\tqa = status\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home, homeCfg, xdgCfg
}

// pinRow is inletPin()'s value for key, and whether the row is there at all.
func pinRow(key string) (string, bool) {
	for _, v := range inletPin() {
		if v.Key == key {
			return v.Value, true
		}
	}
	return "", false
}

// TestQATheGitConfigGlobalPinIsGitsOwnFile: one file is that file, none is
// git's write target, and the value agrees with git's own listing — asked a
// different way from the resolver — in every shape.
func TestQATheGitConfigGlobalPinIsGitsOwnFile(t *testing.T) {
	for _, c := range []struct {
		name          string
		homeF, xdgF   bool
		wantHomeOrXDG string // "home" or "xdg"
	}{
		{"only ~/.gitconfig (this box's shape)", true, false, "home"},
		{"only the XDG file", false, true, "xdg"},
		{"neither: git's --global write target", false, false, "home"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, homeCfg, xdgCfg := globalScopeFixture(t, c.homeF, c.xdgF)
			want := homeCfg
			if c.wantHomeOrXDG == "xdg" {
				want = xdgCfg
			}
			got, ok := pinRow("GIT_CONFIG_GLOBAL")
			if !ok || got != want {
				t.Fatalf("inletPin()[GIT_CONFIG_GLOBAL] = %q (present %v), want %q", got, ok, want)
			}
			if indep, ok := gitGlobalScopePath(t); ok && indep != got {
				t.Errorf("the row says %q but git's own listing says %q", got, indep)
			}
			if d := GitGlobalScopeDegraded(); d != "" {
				t.Errorf("a resolved global scope printed a degraded line: %s", d)
			}
		})
	}

	// XDG_CONFIG_HOME moves the XDG candidate, and the row follows it.
	t.Run("XDG_CONFIG_HOME set, its file the only one", func(t *testing.T) {
		globalScopeFixture(t, false, false)
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		want := xdg + "/git/config"
		if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(want, []byte("[user]\n\temail = x@example.invalid\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _ := pinRow("GIT_CONFIG_GLOBAL"); got != want {
			t.Errorf("inletPin()[GIT_CONFIG_GLOBAL] = %q, want %q", got, want)
		}
	})
}

// TestQATheGitConfigGlobalPinShutsAllThreeGlobalInlets is the execution pin:
// each attacker arm FIRES with the row taken out of the pin (so it measures
// something) and is QUIET with the whole pin applied.
func TestQATheGitConfigGlobalPinShutsAllThreeGlobalInlets(t *testing.T) {
	globalScopeFixture(t, true, false)
	repo := gitConfigProbeRepo(t)
	hooks := gitConfigProbeHooks(t)
	evil := "[core]\n\thooksPath = " + hooks + "\n"
	xdg := t.TempDir()
	if err := os.MkdirAll(filepath.Join(xdg, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "git", "config"), []byte(evil), 0o644); err != nil {
		t.Fatal(err)
	}
	evilHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(evilHome, ".gitconfig"), []byte(evil), 0o644); err != nil {
		t.Fatal(err)
	}

	pin := inletPin()
	if _, ok := pinRow("GIT_CONFIG_GLOBAL"); !ok {
		t.Fatalf("CONTROL FAILED: a fixture HOME with one global file produced no GIT_CONFIG_GLOBAL row")
	}
	fires := func(withRow bool, attack string) bool {
		marker := filepath.Join(hooks, "marker")
		_ = os.Remove(marker)
		env := append(gitConfigScrubbedEnv(), attack)
		for _, v := range pin {
			if v.Key == "GIT_CONFIG_GLOBAL" && !withRow {
				continue
			}
			env = append(env, v.Key+"="+v.Value)
		}
		gitConfigCheckoutRoundTrip(repo, env)
		_, err := os.Stat(marker)
		return err == nil
	}
	if fires(true, "POSSE_QA_NO_ATTACK=1") {
		t.Fatal("CONTROL FAILED: the hook fired with no attack at all")
	}
	for _, arm := range []struct{ name, attack string }{
		{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_GLOBAL=" + gitConfigProbeGlobal(t, hooks)},
		{"XDG_CONFIG_HOME", "XDG_CONFIG_HOME=" + xdg},
		{"HOME", "HOME=" + evilHome},
	} {
		if !fires(false, arm.attack) {
			t.Errorf("CONTROL FAILED: the %s attack did not fire without the GIT_CONFIG_GLOBAL row, so the pinned arm measures nothing", arm.name)
			continue
		}
		if fires(true, arm.attack) {
			t.Errorf("the %s attack ran its hook with the GIT_CONFIG_GLOBAL row applied — the inlet ranger-base-obdnk closes is open", arm.name)
		}
	}
}

// TestQATheGitConfigGlobalPinDegradesNeverDevNullWithTwoFiles: both global
// files exist, so no single value is neutral. No row, a named DEGRADED line,
// and never a closing constant — at the table, in the rendered payload, and
// by git's own listing.
func TestQATheGitConfigGlobalPinDegradesNeverDevNullWithTwoFiles(t *testing.T) {
	globalScopeFixture(t, true, true)
	if v, ok := pinRow("GIT_CONFIG_GLOBAL"); ok {
		t.Fatalf("inletPin() carries GIT_CONFIG_GLOBAL=%q with two global files. No single value reads both; %s and \"\" misattribute every commit — the row must be absent", v, os.DevNull)
	}
	if indep, ok := gitGlobalScopePath(t); !ok || indep != "" {
		t.Errorf("git's own listing does not see two global files here (%q, %v) — the fixture is not the shape this test is about", indep, ok)
	}
	var payload struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(ClaudeFleetSettingsJSON(nil)), &payload); err != nil {
		t.Fatal(err)
	}
	if v, ok := payload.Env["GIT_CONFIG_GLOBAL"]; ok {
		t.Errorf("the rendered settings payload carries GIT_CONFIG_GLOBAL=%q with two global files", v)
	}
	d := GitGlobalScopeDegraded()
	for _, want := range []string{"DEGRADED", "git global scope has two files; GIT_CONFIG_GLOBAL inlet NOT pinned", "XDG_CONFIG_HOME", "HOME"} {
		if !strings.Contains(d, want) {
			t.Errorf("the degraded launch line does not say %q:\n%q", want, d)
		}
	}
}

// TestQATheGitConfigGlobalPinKeepsTheIdentityWall: DeriveIdentityLiterals
// returns the same set unset and pinned. The /dev/null arm is the failing
// wrong arm — it is what ranger-base-37y0z refused, and it still empties it.
func TestQATheGitConfigGlobalPinKeepsTheIdentityWall(t *testing.T) {
	globalScopeFixture(t, true, false)
	repo := gitConfigProbeRepo(t)
	pinned, ok := pinRow("GIT_CONFIG_GLOBAL")
	if !ok {
		t.Fatal("CONTROL FAILED: no GIT_CONFIG_GLOBAL row for a one-file fixture")
	}
	wall := func(value string, set bool) []string {
		t.Helper()
		if set {
			t.Setenv("GIT_CONFIG_GLOBAL", value)
		} else {
			unsetenvForTest(t, "GIT_CONFIG_GLOBAL")
		}
		lits, err := DeriveIdentityLiterals(repo)
		if err != nil {
			t.Fatalf("DeriveIdentityLiterals: %v", err)
		}
		var out []string
		for _, l := range lits {
			out = append(out, l.Class+"="+l.Value)
		}
		slices.Sort(out)
		return out
	}
	unset := wall("", false)
	if !slices.Contains(unset, "email=home@example.invalid") {
		t.Fatalf("CONTROL FAILED: the fixture's global user.email is not in the wall unset (%v), so nothing below can show it moving", unset)
	}
	if got := wall(pinned, true); !slices.Equal(got, unset) {
		t.Errorf("the identity wall moved under GIT_CONFIG_GLOBAL=%s:\n unset  %v\n pinned %v", pinned, unset, got)
	}
	if got := wall(os.DevNull, true); slices.Contains(got, "email=home@example.invalid") {
		t.Errorf("GIT_CONFIG_GLOBAL=%s kept the global user.email in the wall. The ALSO NOT COVERED paragraph says it empties it — re-measure and rewrite it", os.DevNull)
	}
}

// TestQAGitGlobalScopeFallsBackToGitsDocumentedRule: a git that cannot answer
// `git var GIT_CONFIG_GLOBAL` (older than 2.42) gets the documented rule over
// the same environment, and the rule is held against what a git that CAN
// answer says.
func TestQAGitGlobalScopeFallsBackToGitsDocumentedRule(t *testing.T) {
	home, homeCfg, xdgCfg := globalScopeFixture(t, false, false)
	real := resolveOutside("git", "")
	if real == "" {
		t.Skip("no git on the gates-free PATH")
	}
	old := filepath.Join(t.TempDir(), "git")
	body := "#!/bin/sh\n[ \"$1\" = var ] && { echo 'usage: git var' >&2; exit 129; }\nexec " + real + " \"$@\"\n"
	if err := WriteExecutable(old, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	env := gitSystemScopeEnv()
	got, err := gitGlobalScopeCandidates(old, env)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{xdgCfg, homeCfg}; !slices.Equal(got, want) {
		t.Errorf("older-git candidates = %v, want %v", got, want)
	}
	if fromGit, err := gitGlobalScopeCandidates(real, env); err == nil && !slices.Equal(fromGit, got) {
		t.Errorf("the documented rule says %v but `git var GIT_CONFIG_GLOBAL` says %v under HOME=%s", got, fromGit, home)
	}
}

// TestQAChooseGitGlobalScope is the decision table, alone.
func TestQAChooseGitGlobalScope(t *testing.T) {
	t.Parallel()
	const x, h = "/h/.config/git/config", "/h/.gitconfig"
	for _, c := range []struct {
		name  string
		have  []string
		want  string
		isErr bool
	}{
		{"none: the last candidate", nil, h, false},
		{"home only", []string{h}, h, false},
		{"xdg only", []string{x}, x, false},
		{"both: no value", []string{x, h}, "", true},
	} {
		got, err := chooseGitGlobalScope([]string{x, h}, func(p string) bool { return slices.Contains(c.have, p) })
		if got != c.want || (err != nil) != c.isErr {
			t.Errorf("%s: got %q, %v", c.name, got, err)
		}
		if got == os.DevNull {
			t.Errorf("%s: chose %s", c.name, os.DevNull)
		}
	}
	if _, err := gitGlobalScopeDocumented([]string{"PATH=/bin"}); err == nil {
		t.Errorf("no HOME and no XDG_CONFIG_HOME resolved to a global file; git reads none")
	}
}
