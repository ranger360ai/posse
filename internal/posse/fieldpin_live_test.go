//go:build posse_arm3

package posse

// Live pin for ranger-base-i7cy4, run against the real claude rather than a
// transcription of its bundle:
//
//	RHQ_LIVE_CLAUDE=1 go test -tags posse_arm3 ./internal/posse -run TestLiveClaudeFieldPin -v
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
// WHERE THE ATTACK ARM LIVES, which is the half ranger-base-yglkp had to
// move. There are three scopes a planted apiKeyHelper could be read from
// here, and on a box carrying this repo's own policy drop-in only one of
// them can still host the arm:
//
//   - USER scope is the threat, and this box cannot host it: a root-owned
//     managed-settings.json nails CLAUDE_CONFIG_DIR to the operator's real
//     ~/.claude (ranger-base-sn0w8), which beats process env and $HOME
//     both, so a scratch-HOME rig silently reads the operator's live file
//     as user scope and every arm comes back identical for the wrong
//     reason.
//   - PROJECT scope was the stand-in, and it was a faithful one for THIS
//     question: both scopes are folded into the merged settings object by
//     one left fold over userSettings, projectSettings, localSettings,
//     flagSettings, policySettings with one customizer, so a pin at
//     flagSettings sits above both and the fold cannot reach the flag row
//     for one scope and not the other. What killed it is the other end of
//     that same fold. `etc/claude/managed-settings.d/20-posse-field-pin.json`
//     is installed on this box, and it carries `apiKeyHelper: ""` at
//     policySettings — the LAST source in the fold. So nothing below policy
//     can arm an apiKeyHelper here any more, our own flag row included, and
//     the project-scope arm reads "not in force" with no pin on the line.
//     That is the stronger guarantee arriving, not the attack being
//     measured: the arm below runs it, and when it does not fire it
//     requires an OS-admin file on this box to say why rather than passing.
//     MEASURED 2026-10-09, claude 2.1.288/2.1.291/2.1.292/2.1.294/2.1.295,
//     user, project and flag scope, both under this session's environment
//     and under `env -i`: not in force at any of them, and a `-p` run with
//     a helper that would have left a marker file left none and printed
//     "Not logged in · Please run /login".
//   - FLAG scope under `--bare`, which is where the arm is now. `--bare`
//     (CLAUDE_CODE_SIMPLE=1) resolves apiKeyHelper from `me("flagSettings")`
//     DIRECTLY instead of from the merged object — its own help text says
//     so: "Anthropic auth is strictly ANTHROPIC_API_KEY or apiKeyHelper via
//     --settings (OAuth and keychain are never read)" — so it reaches past
//     the policy row and a planted helper is in force again (MEASURED
//     2026-10-09 on all five of the above).
//
// AND THAT VENUE MEASURES THE HAZARD THIS PIN EXISTS FOR, better than the
// old one did. The arms below plant the helper INSIDE the rendered payload
// rather than in a one-row payload beside it: a payload that is the fleet's
// own rows plus a planted apiKeyHelper must read in force, and the same
// payload as shipped must not. The first is the canary — if any OTHER row
// of the real payload has stopped being accepted, the runtime discards the
// whole thing and the planted row vanishes with it, so the canary goes red
// where the old single-row canary could not see it at all. MEASURED
// 2026-10-09 on 2.1.295, with `statusLine` mutated back to a bare "" in the
// planted payload: not in force, which is the discard, which is the red.
// The second arm is the pin itself, and it is only evidence BECAUSE the
// first one fired.
//
// What this venue no longer claims is precedence over a lower scope: under
// `--bare` flagSettings is the only source read, so there is no lower scope
// to beat. That half of i7cy4 is now held by the policy drop-in, which
// outranks every scope the old arm could reach, and by the merge
// measurements transcribed in fieldpin.go.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// claudeAPIKeyHelperInForce runs the readout in `dir` with `extra` in front
// of the subcommand — `--settings` and `--bare` are GLOBAL options and must
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

// plantedHelperCommand is the command the arms below plant. It would hand
// its stdout to the runtime as an API key; nothing here executes it (see
// the header), and the command is a no-op either way.
const plantedHelperCommand = "/bin/echo FAKE-ranger-base-i7cy4"

// payloadWithPlantedHelper is `payload` with its apiKeyHelper row replaced
// by a command of ours, and every other row left exactly as the launcher
// renders it. That is what makes the canary arm a statement about the whole
// payload: one wrong-typed row anywhere in it and the runtime throws the
// object away, taking the planted row with it.
func payloadWithPlantedHelper(t *testing.T, payload string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("the rendered payload is not JSON, so nothing below can be measured: %v\n%s", err, payload)
	}
	b, err := json.Marshal(plantedHelperCommand)
	if err != nil {
		t.Fatal(err)
	}
	m["apiKeyHelper"] = b
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// policyAPIKeyHelperPin names the OS-admin settings file on this box that
// pins apiKeyHelper, if one does. It is what the project-scope arm consults
// when its control does not fire: a box whose policy tier already pins the
// field has closed the hole above the scope that arm can reach, and a box
// where nothing does has a rig or a runtime that moved.
//
// Darwin's managed directory, the one cost.go and scripts/verify-policy-pins.sh
// already name. The resolver joins the managed dir with "managed-settings.d"
// and treats an absent one as "no drop-ins".
func policyAPIKeyHelperPin() (string, bool) {
	const managed = "/Library/Application Support/ClaudeCode"
	paths := []string{filepath.Join(managed, "managed-settings.json")}
	if ents, err := os.ReadDir(filepath.Join(managed, "managed-settings.d")); err == nil {
		for _, e := range ents {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
				paths = append(paths, filepath.Join(managed, "managed-settings.d", e.Name()))
			}
		}
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var m struct {
			APIKeyHelper *string `json:"apiKeyHelper"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if m.APIKeyHelper != nil && *m.APIKeyHelper == "" {
			return p, true
		}
	}
	return "", false
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
	planted := `{"apiKeyHelper":"` + plantedHelperCommand + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}

	// THE READOUT'S OWN CONTROL, first because it tells a broken readout
	// apart from a broken payload: a planted helper on its own, in the one
	// mode that reads flagSettings for this field directly.
	if !claudeAPIKeyHelperInForce(t, dir, "--bare", "--settings", planted) {
		t.Fatalf("CONTROL: a flag-scope apiKeyHelper is not in force under --bare with nothing else on the line — this readout can no longer tell a resolved helper from none, so every arm below measures nothing. Either the runtime stopped reading apiKeyHelper from --settings in minimal mode, or `claude auth status --json` stopped reporting authMethod=api_key_helper.\npayload: %s", planted)
	}

	// THE PROJECT-SCOPE ARM, which on a box with no policy-tier field pin
	// is the faithful precedence measurement (see the header) and on a box
	// with one cannot fire. Either way it is not allowed to pass silently.
	t.Run("a planted project-scope helper, if any scope below policy can still arm one", func(t *testing.T) {
		if !claudeAPIKeyHelperInForce(t, dir) {
			src, ok := policyAPIKeyHelperPin()
			if !ok {
				t.Fatalf("CONTROL: no apiKeyHelper in force with nothing pinned, and no OS-admin settings file on this box pins the field either — the attack arm did not fire and nothing explains why. Either the runtime stopped reading apiKeyHelper from a settings file, or this readout stopped reporting it (ranger-base-yglkp)")
			}
			t.Skipf("nothing below the policy tier can arm an apiKeyHelper on this box: %s pins the field at policySettings, the last source in the merge fold, which outranks every scope this arm can write (ranger-base-yglkp)", src)
		}
		if claudeAPIKeyHelperInForce(t, dir, "--settings", ClaudeFleetSettingsJSON(nil)) {
			t.Errorf("the planted apiKeyHelper survived the launch payload.\nEither the pin lost the precedence, or ONE ROW OF THE PAYLOAD IS WRONG-TYPED and the runtime discarded the whole thing — the credential dirs and every env row with it.\npayload: %s", ClaudeFleetSettingsJSON(nil))
		}
		if claudeAPIKeyHelperInForce(t, dir, "--settings", credentialDirPinJSON()) {
			t.Errorf("the planted apiKeyHelper survived the appended pin.\npayload: %s", credentialDirPinJSON())
		}
	})

	// THE REAL PAYLOAD, graded by the real reader. Each builder gets both
	// arms: the planted variant must be in force, which is the whole
	// payload being accepted, and the shipped one must not, which is the
	// row doing its job. The second is only evidence because the first
	// fired — a discarded payload reads exactly like a pin that worked.
	for _, p := range []struct{ name, payload string }{
		{"the launch payload", ClaudeFleetSettingsJSON(nil)},
		{"the pin appended to a hand-written command", credentialDirPinJSON()},
	} {
		t.Run(p.name+" is read whole, and its field row pins the helper", func(t *testing.T) {
			withHelper := payloadWithPlantedHelper(t, p.payload)
			if !claudeAPIKeyHelperInForce(t, dir, "--bare", "--settings", withHelper) {
				t.Fatalf("CANARY: this payload carrying a planted apiKeyHelper is NOT in force — the runtime threw the whole object away, which means ONE ROW OF IT IS WRONG-TYPED, and on a real launch that takes the credential dirs, every inlet row and the fleet's permission mode with it, in silence. The arm below measures nothing until this is green.\npayload: %s", withHelper)
			}
			if claudeAPIKeyHelperInForce(t, dir, "--bare", "--settings", p.payload) {
				t.Errorf("the payload's own apiKeyHelper row left a helper in force — the field is not pinned by what the launcher ships.\npayload: %s", p.payload)
			}
		})
	}
}
