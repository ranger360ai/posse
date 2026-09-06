//go:build posse_arm3

package posse

// Live pin for ranger-base-i7cy4, run against the real claude rather than a
// transcription of its bundle:
//
//	RHQ_LIVE_CLAUDE=1 go test ./internal/posse -run TestLiveClaudeFieldPin -v
//
// WHAT IT MEASURES, and why it is not a duplicate of the QA pins. The QA
// pins say the rendered payload carries the rows. This says the RUNTIME
// ACCEPTS it — which is a separate fact and a sharp one, because a single
// wrong-typed row makes the runtime discard the whole `--settings` payload
// in silence. Measured on 2026-09-05 (2.1.261): with
// `{"apiKeyHelper":"","statusLine":""}` — statusLine is an object, not a
// string — a planted apiKeyHelper was still in force. That is not "the
// statusLine row did nothing"; that is the credential-dir pin, every inlet
// row and the fleet's permission mode gone with it. `statusLine` typed as
// an object was accepted. So the type of every row is load-bearing for
// every other row, and only the real reader can grade it.
//
// THE READOUT is `claude auth status --json`, the same no-login, no-turn,
// no-trust-prompt command the credential-dir pin uses (rq83c). `loggedIn`
// is true when an apiKeyHelper is CONFIGURED — the helper is not executed
// for this readout (measured: a helper that exits 3 with no output still
// reads loggedIn=true), so this probes settings RESOLUTION, which is what a
// precedence pin is about, and it runs no attacker-supplied command.
//
// THE ATTACK ARM plants the helper at PROJECT scope, not user scope, and
// that substitution is the one thing here to read carefully. The threat is
// a persona-writable ~/.claude/settings.json, and this box cannot host that
// arm: a root-owned managed-settings.json nails CLAUDE_CONFIG_DIR to the
// operator's real ~/.claude (ranger-base-sn0w8), which beats process env
// and $HOME both, so a scratch-HOME rig silently reads the operator's live
// file as user scope and every arm comes back identical for the wrong
// reason. Project scope is a faithful stand-in for THIS question because
// both are folded into the merged settings object by one left fold over
// userSettings, projectSettings, localSettings, flagSettings,
// policySettings with one customizer: a pin at flagSettings sits above both
// of them, and the fold cannot reach the flag row for one scope and not the
// other. It is NOT a faithful stand-in for the env pin's question, where
// the scopes are filtered differently — which is why that half is measured
// elsewhere.
//
// THE CONTROL is arm one. It runs the attack with no pin on the line and
// measures the helper being in force: without that, a green here would only
// mean nothing was ever configured (ranger-base: "probe needs a failing
// wrong arm").

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// claudeAPIKeyHelperInForce runs the readout in `dir` with `extra` in front
// of the subcommand — `--settings` is a GLOBAL option on 2.1.261 and must
// precede it, `claude auth status --settings X` is "unknown option" — and
// reports whether an apiKeyHelper resolved. A non-zero exit is normal (the
// readout exits 1 when not logged in), so the JSON is read whatever the
// exit was and only an unparseable answer is fatal.
func claudeAPIKeyHelperInForce(t *testing.T, dir string, extra ...string) bool {
	t.Helper()
	cmd := exec.Command("claude", append(append([]string{}, extra...), "auth", "status", "--json")...)
	cmd.Dir = dir
	out, _ := cmd.Output()
	var m struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("claude %v auth status --json: unparseable answer: %v\n%s", extra, err, out)
	}
	return m.LoggedIn && m.AuthMethod == "api_key_helper"
}

func TestLiveClaudeFieldPinRefusesAPlantedCommandField(t *testing.T) {
	// Safe in parallel where its credential-dir sibling is not: that one
	// moves the process environment with t.Setenv, this one only writes its
	// own t.TempDir and passes everything else on the child's command line.
	t.Parallel()
	if os.Getenv("RHQ_LIVE_CLAUDE") == "" {
		t.Skip("set RHQ_LIVE_CLAUDE=1 (shells out to the real claude)")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("no claude on PATH")
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A helper that would run a command of the attacker's choosing and hand
	// its stdout to the runtime as an API key. Nothing here executes it —
	// see the header — and the command is a no-op either way.
	const planted = `{"apiKeyHelper":"/bin/echo FAKE-ranger-base-i7cy4"}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}

	if !claudeAPIKeyHelperInForce(t, dir) {
		t.Fatalf("CONTROL: no apiKeyHelper in force with nothing pinned — the attack arm did not fire, so every arm below measures nothing. Either the runtime stopped reading apiKeyHelper from a settings file, or this readout stopped reporting it")
	}

	t.Run("the launch payload takes it away", func(t *testing.T) {
		if claudeAPIKeyHelperInForce(t, dir, "--settings", ClaudeFleetSettingsJSON()) {
			t.Errorf("the planted apiKeyHelper survived the launch payload.\nEither the pin lost the precedence, or ONE ROW OF THE PAYLOAD IS WRONG-TYPED and the runtime discarded the whole thing — the credential dirs and every env row with it.\npayload: %s", ClaudeFleetSettingsJSON())
		}
	})

	t.Run("so does the pin appended to a hand-written command", func(t *testing.T) {
		if claudeAPIKeyHelperInForce(t, dir, "--settings", credentialDirPinJSON()) {
			t.Errorf("the planted apiKeyHelper survived the appended pin.\npayload: %s", credentialDirPinJSON())
		}
	})

	// The half a false green would hide: a payload the runtime threw away
	// reads exactly like a pin that worked, from any arm that only looks
	// for the attack being gone. So assert the payload was READ, by a row
	// whose effect this readout can see on its own.
	t.Run("the payload was read, not discarded", func(t *testing.T) {
		if !claudeAPIKeyHelperInForce(t, dir, "--settings", `{"apiKeyHelper":"/bin/echo FLAG-ranger-base-i7cy4"}`) {
			t.Errorf("a flag-scope apiKeyHelper did not reach at all — this readout can no longer tell an accepted payload from a discarded one, and the two arms above are not evidence")
		}
	})
}
