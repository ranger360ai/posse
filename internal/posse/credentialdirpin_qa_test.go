package posse

// Pins for ranger-base-rq83c: the launch pins CLAUDE_CONFIG_DIR and
// CLAUDE_SECURESTORAGE_CONFIG_DIR at a scope a persona-writable
// ~/.claude/settings.json cannot reach, so a caged persona cannot move the
// runtime's credential store — its own, or the operator's.
//
// The measurement these pins stand on is in credentialDirPin's own doc
// comment, and the runtime half of it is re-run by
// credentialdirpin_live_test.go against the real claude. What is pinned
// HERE is posse's half: the VALUES the pin carries (a wrong value is a
// renamed keychain item, not a refusal — ranger-base-ig4op), that they
// reach the rendered launch line, and that they reach it exactly once.

import (
	"encoding/json"
	"strings"
	"testing"
)

// The rule credentialDirPin implements, one row per arm of the runtime's
// own resolution. Every row's `sec` is what keeps BOTH the credentials file
// path and the keychain item's name where the box already has them.
func TestQACredentialDirPinIsWhatThisEnvironmentAlreadyResolvesTo(t *testing.T) {
	for _, tc := range []struct {
		name    string
		secSet  bool
		sec     string
		cfg     string
		wantSec string
		wantCfg string // "" = $HOME/.claude
		why     string
	}{
		{
			name:    "neither set — the measured normal case on this box",
			wantSec: "",
			why:     "an EMPTY secure-storage dir is the runtime's own spelling of $HOME/.claude, and the one value that leaves the keychain item unsuffixed. Pinning the path itself would add a sha256 suffix that is not there today, and rename the item out from under the operator's login",
		},
		{
			name: "secure storage set to a directory", secSet: true, sec: "/tmp/sec",
			wantSec: "/tmp/sec",
			why:     "verbatim: the runtime already resolves this and already hashes it into the item name",
		},
		{
			name: "secure storage present but EMPTY", secSet: true, sec: "",
			wantSec: "",
			why:     "presence, not truthiness — an empty value means $HOME/.claude to the runtime and must not be turned into a path",
		},
		{
			name: "config dir set, secure storage unset", cfg: "/tmp/cfg",
			wantSec: "/tmp/cfg", wantCfg: "/tmp/cfg",
			why: "the runtime's secure-storage dir FALLS BACK to the config dir and hashes the same string into the item name, so the pin has to say it out loud to keep both where they were",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			if tc.secSet {
				t.Setenv("CLAUDE_SECURESTORAGE_CONFIG_DIR", tc.sec)
			} else {
				unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
			}
			if tc.cfg != "" {
				t.Setenv("CLAUDE_CONFIG_DIR", tc.cfg)
			} else {
				unsetenvForTest(t, "CLAUDE_CONFIG_DIR")
			}
			wantCfg := tc.wantCfg
			if wantCfg == "" {
				wantCfg = home + "/.claude"
			}

			pin := credentialDirPin()
			got := map[string]string{}
			var order []string
			for _, v := range pin {
				got[v.Key] = v.Value
				order = append(order, v.Key)
			}
			if len(pin) != 2 {
				t.Fatalf("pin = %v, want both names — a pin that names one variable leaves the other free to move the store", pin)
			}
			if strings.Join(order, ",") != strings.Join(credentialDirVars, ",") {
				t.Errorf("pin order = %v, want credentialDirVars order %v", order, credentialDirVars)
			}
			if got["CLAUDE_SECURESTORAGE_CONFIG_DIR"] != tc.wantSec {
				t.Errorf("CLAUDE_SECURESTORAGE_CONFIG_DIR = %q, want %q — %s", got["CLAUDE_SECURESTORAGE_CONFIG_DIR"], tc.wantSec, tc.why)
			}
			if got["CLAUDE_CONFIG_DIR"] != wantCfg {
				t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", got["CLAUDE_CONFIG_DIR"], wantCfg)
			}

			// THE INVARIANT, and the one a careless pin breaks: applying
			// the pin must change NOTHING about where this box already
			// resolves. Measured by re-deriving both answers with the
			// pinned values in the environment — the credentials file's
			// path, and the keychain item's NAME, which carries a hash of
			// the directory whenever a variable named it
			// (ranger-base-ig4op, ranger-base-mx4q6). A pin that named
			// $HOME/.claude where nothing was set would pass a path check
			// and still rename the item out from under the operator's
			// login, so the name is checked on its own.
			wantFile, ferr := CredentialsFile()
			wantItem, _ := keychainItem()
			for _, v := range pin {
				t.Setenv(v.Key, v.Value)
			}
			gotFile, gerr := CredentialsFile()
			if (ferr == nil) != (gerr == nil) || (ferr == nil && gotFile != wantFile) {
				t.Errorf("the pin moved the credentials file: %q (%v) -> %q (%v). The read-deny is rendered from the unpinned environment, so the wall and the runtime would disagree about where the file is", wantFile, ferr, gotFile, gerr)
			}
			if gotItem, _ := keychainItem(); gotItem != wantItem {
				t.Errorf("the pin renamed the keychain item: %q -> %q. The operator's login is under the first name; a session pinned to the second reads an empty keychain", wantItem, gotItem)
			}
		})
	}
}

// No home is no CREDENTIAL-DIR pin: there is no path to name, and a launch
// is worth more than a pin rendered against a directory that does not
// exist.
//
// The inlet rows are the other half and they do NOT drop out here
// (ranger-base-rflee): none of them names a path this box has to have, so a
// box with no home still gets its exec and transport pin. That asymmetry is
// the point of the assertion below — before rflee this payload was the
// const alone.
func TestQACredentialDirPinIsAbsentWithNoHome(t *testing.T) {
	t.Setenv("HOME", "")
	if pin := credentialDirPin(); pin != nil {
		t.Errorf("credentialDirPin = %v with no home, want none", pin)
	}
	for _, v := range settingsPin() {
		for _, name := range credentialDirVars {
			if v.Key == name {
				t.Errorf("settingsPin carries %s with no home — there is no directory to name", name)
			}
		}
	}
	if got := ClaudeFleetSettingsJSON(); got == ClaudeFleetSettings {
		t.Errorf("ClaudeFleetSettingsJSON = the const alone with no home — the inlet pin does not depend on a home directory and must survive here")
	}
	if !strings.Contains(ClaudeFleetSettingsJSON(), "BASH_ENV") {
		t.Errorf("ClaudeFleetSettingsJSON with no home does not carry the inlet pin:\n%s", ClaudeFleetSettingsJSON())
	}
}

// The merge is additive: the fleet's own two keys are what the launch line
// is read for, and a pin that quietly took the permission mode off the line
// would be a worse bug than the one it fixes.
func TestQAClaudeFleetSettingsJSONCarriesTheCredentialDirPin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")

	var got struct {
		Env         map[string]string `json:"env"`
		Permissions struct {
			DefaultMode string `json:"defaultMode"`
		} `json:"permissions"`
		SkillOverrides map[string]string `json:"skillOverrides"`
	}
	payload := ClaudeFleetSettingsJSON()
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("ClaudeFleetSettingsJSON is not valid JSON: %v\n%s", err, payload)
	}
	if got.Permissions.DefaultMode != "auto" {
		t.Errorf("permissions.defaultMode = %q, want auto — the merge dropped what the const was for", got.Permissions.DefaultMode)
	}
	if got.SkillOverrides["auto-mode-setup"] != "off" {
		t.Errorf("skillOverrides = %v, want auto-mode-setup off", got.SkillOverrides)
	}
	for _, name := range credentialDirVars {
		if _, ok := got.Env[name]; !ok {
			t.Errorf("env does not carry %s — this payload is the ONLY scope the launch can pin it at, and a settings.json the persona writes wins over anything the launcher exports", name)
		}
	}
	if got.Env["CLAUDE_CONFIG_DIR"] != home+"/.claude" {
		t.Errorf("env.CLAUDE_CONFIG_DIR = %q, want %q", got.Env["CLAUDE_CONFIG_DIR"], home+"/.claude")
	}
	// Rendered twice, byte-identical: the pane line and the spilled launch
	// script are compared against each other elsewhere, and a payload whose
	// key order moved between renders would make every such comparison lie.
	if again := ClaudeFleetSettingsJSON(); again != payload {
		t.Errorf("two renders differ:\n%s\n%s", payload, again)
	}
}

// The line, not the payload: both templates posse ships must carry the
// settings flag, once, with the pin inside it.
func TestQATheRenderedClaudeLineCarriesOneSettingsFlagWithThePin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")

	ag := &AgentFile{Name: "pin", Path: home + "/pin.md", MemoryDir: home + "/mem"}
	// Through LoadRuntime, not `&builtinRuntimes[i]`: taking the address of
	// a built-in table entry is a WRITE to the shared table by
	// cmd/testparallel's reading, and it would cost every test that names
	// that table its t.Parallel.
	for _, tc := range []struct{ name, line string }{
		{"builtin template", ag.RenderCommandFor(claudeRuntime(t), "claude", DefaultTier)},
		{"DefaultAgentCommand (the legacy render)", ag.RenderCommand()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if n := strings.Count(tc.line, "--settings"); n != 1 {
				t.Fatalf("the line names --settings %d times, want exactly 1 — a second occurrence REPLACES the first on this CLI, it does not add a source:\n%s", n, tc.line)
			}
			for _, name := range credentialDirVars {
				if !strings.Contains(tc.line, name) {
					t.Errorf("the rendered line does not carry %s:\n%s", name, tc.line)
				}
			}
			if strings.Contains(tc.line, "{settings}") {
				t.Errorf("the {settings} placeholder survived the render:\n%s", tc.line)
			}
		})
	}
}

// EnsureFleetSettings is the guarantee half, and the conditions are the
// ones EnsureUnattended has plus the one this flag has of its own.
func TestQAEnsureSettingsPinAppendsOnlyWhereItIsSafe(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claude := &Runtime{Name: "claude", Command: "claude", FleetSettings: ClaudeFleetSettingsJSON, SettingsPin: credentialDirPinJSON}
	bare := &Runtime{Name: "codex", Command: "codex"}

	t.Run("appended when the line has none", func(t *testing.T) {
		got := claude.EnsureSettingsPin("claude --model x")
		if !strings.Contains(got, "--settings ") || !strings.Contains(got, "CLAUDE_SECURESTORAGE_CONFIG_DIR") {
			t.Errorf("got %q, want the pin appended", got)
		}
		// The pin, not posse's unattended policy: a template posse did not
		// write is owed the security guarantee and nothing else.
		if strings.Contains(got, "skillOverrides") || strings.Contains(got, "defaultMode") {
			t.Errorf("got %q — the appended payload carries fleet policy a hand-written command: never asked for", got)
		}
	})
	t.Run("never a second one", func(t *testing.T) {
		in := `claude --settings '{"permissions":{"defaultMode":"plan"}}'`
		if got := claude.EnsureSettingsPin(in); got != in {
			t.Errorf("got %q, want unchanged — the last --settings wins on this CLI, so appending would silently drop the template's own payload", got)
		}
		if got := claude.EnsureSettingsPin("claude --settings=/tmp/s.json"); !strings.HasSuffix(got, "--settings=/tmp/s.json") {
			t.Errorf("got %q, want unchanged — the `=` spelling is the same flag", got)
		}
	})
	t.Run("not on a line that is not this CLI", func(t *testing.T) {
		in := "env FOO=1 claude"
		if got := claude.EnsureSettingsPin(in); got != in {
			t.Errorf("got %q, want unchanged — a flag typed at the wrong program is a launch that fails outright", got)
		}
	})
	t.Run("nothing for a runtime with no settings surface", func(t *testing.T) {
		in := "codex -a never"
		if got := bare.EnsureSettingsPin(in); got != in {
			t.Errorf("got %q, want unchanged", got)
		}
	})
}
