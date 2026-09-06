//go:build posse_arm3

package posse

// Live pin for ranger-base-rq83c, run against the real claude rather than a
// transcription of its bundle:
//
//	RHQ_LIVE_CLAUDE=1 go test ./internal/posse -run TestLiveClaudeSettingsPin -v
//
// THE READOUT is `claude auth status --json`, which runs with no login, no
// network turn and no trust prompt: `loggedIn` is true exactly when the
// resolved credential directory holds an envelope. The envelope planted
// here is a FAKE — a measurement never copies a live token (ADR 0019), and
// nothing here reads one. Its SHAPE is the one the runtime's own login loop
// writes (NOTES.md, measured on the release artifact), because a shape the
// reader rejects would read as "not logged in" from every directory and
// this pin would be green for the wrong reason.
//
// The same command's `projectsDirectory` is NOT asserted on, and that is a
// finding rather than a shortcut: it reads null whenever a flag-scope
// settings source sets CLAUDE_CONFIG_DIR, whatever the directory resolves
// to. Only the credential half has a readout that says what it means.
//
// WHY THE LAUNCHER ENV IS SET THE WAY IT IS. On darwin the store is a
// composite that tries the KEYCHAIN first, and the keychain item's name
// carries a sha256 of the configured directory (credentialDirPin's
// transcription of the bundle). Both arms below therefore leave the pin
// resolving to a NON-EMPTY scratch path, which is an item name no box has —
// so the keychain cannot answer in either arm and the file is the whole
// store, on a caged box and on the operator's own, logged in or not.
//
// THE CONTROL is arm one of each pair. It runs with the launcher exporting
// the right directory in the child's real environment and still measures
// the redirect happening: that is what says the fix had to be a settings
// payload rather than an export, and it is what stops this pin from going
// green on a rig where nothing could have moved.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// claudeLoggedIn runs the readout under `home` with `extra` in front of the
// subcommand. A non-zero exit is normal — `auth status` exits 1 when it is
// not logged in — so the JSON is read whatever the exit was, and only an
// unparseable answer is fatal.
func claudeLoggedIn(t *testing.T, home string, extra ...string) bool {
	t.Helper()
	cmd := exec.Command("claude", append(append([]string{}, extra...), "auth", "status", "--json")...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Dir = home
	out, _ := cmd.Output()
	var m struct {
		LoggedIn bool `json:"loggedIn"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("claude auth status %v: unparseable answer: %v\n%s", extra, err, out)
	}
	return m.LoggedIn
}

// fakeEnvelope is a credentials file no server would ever answer to.
const fakeEnvelope = `{"claudeAiOauth":{"accessToken":"FAKE-ranger-base-rq83c","refreshToken":"FAKE-ranger-base-rq83c","expiresAt":4102444800000,"refreshTokenExpiresAt":4102444800000,"scopes":["user:inference"],"subscriptionType":"max"}}`

func TestLiveClaudeSettingsPinRefusesACredentialDirRedirect(t *testing.T) {
	if os.Getenv("RHQ_LIVE_CLAUDE") == "" {
		t.Skip("set RHQ_LIVE_CLAUDE=1 (shells out to the real claude)")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("no claude on PATH")
	}

	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	own := filepath.Join(home, ".claude")
	attacker := filepath.Join(dir, "attacker")
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(attacker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attacker, ".credentials.json"), []byte(fakeEnvelope), 0o600); err != nil {
		t.Fatal(err)
	}

	// The launcher's environment — what credentialDirPin renders from, and
	// what the child inherits.
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", own)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")

	// What a caged persona can write: the cage grants ~/.claude whole (ADR
	// 0012 D4), and a user-scope settings.json `env` block is applied to
	// process.env of every claude that starts on the box afterwards.
	for _, attack := range []struct{ name, env string }{
		{"the credential dir named outright", `{"CLAUDE_SECURESTORAGE_CONFIG_DIR":"` + attacker + `"}`},
		{"the config dir alone, which the credential dir falls back to", `{"CLAUDE_CONFIG_DIR":"` + attacker + `"}`},
		{"both", `{"CLAUDE_CONFIG_DIR":"` + attacker + `","CLAUDE_SECURESTORAGE_CONFIG_DIR":"` + attacker + `"}`},
	} {
		t.Run(attack.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(own, "settings.json"), []byte(`{"env":`+attack.env+`}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if !claudeLoggedIn(t, home) {
				t.Fatalf("CONTROL: loggedIn=false with no pin on the line — the redirect did not happen, so this arm measures nothing. Either the runtime changed how it applies user-scope settings env, or the fake envelope is not where %s points", attack.env)
			}
			payload := ClaudeFleetSettingsJSON()
			if claudeLoggedIn(t, home, "--settings", payload) {
				t.Errorf("loggedIn=true with the pin on the line — the persona's settings.json still moved the credential store.\npayload: %s", payload)
			}
		})
	}

	// The half a false green would hide: a pin that resolved to a directory
	// nothing uses reads loggedIn=false above for the wrong reason.
	t.Run("the pin names the store the launcher meant, not nowhere", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(own, ".credentials.json"), []byte(fakeEnvelope), 0o600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(filepath.Join(own, ".credentials.json"))
		if !claudeLoggedIn(t, home, "--settings", ClaudeFleetSettingsJSON()) {
			t.Errorf("loggedIn=false with an envelope in %s, the directory the pin names — the pin points somewhere else", own)
		}
	})
}
