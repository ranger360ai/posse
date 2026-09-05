package posse

// Pins for ranger-base-rflee: the launch pins the transport and exec
// inlets — the variables that move CODE and TRAFFIC — at the same scope
// ranger-base-rq83c pins the credential dirs at, because a settings file a
// persona can write reaches every one of them.
//
// The measurement each VALUE stands on is in inletPin's own doc comment and
// was made by execution against the real readers (bash, sh, dyld, git,
// node, and the claude bundle). What is pinned HERE is posse's half: that
// the names are all present, that the values are the measured-neutral ones,
// and that they reach the rendered launch line inside the one --settings
// flag it is allowed to carry.
//
// The values matter as much as the names, and that is why they are asserted
// literally rather than "non-empty". A wrong value here is not a refusal, it
// is a fleet-wide outage that looks like a security fix: GIT_SSH_COMMAND=""
// breaks every ssh remote, and DYLD_INSERT_LIBRARIES=/dev/null aborts every
// child exec with SIGABRT. Both were candidate spellings until they were
// run.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The table is the deliverable. Each row is a name the audit
// (ranger-base-0w16r) found survives every user-scope filter, and the value
// measured to deny the inlet while leaving this box's behaviour where it
// already was.
func TestQAInletPinCarriesEveryMeasuredNameAtItsMeasuredValue(t *testing.T) {
	t.Parallel() // reads no environment: the table is a constant
	want := map[string]string{
		// Exec: a shell, the dynamic linker, node.
		"BASH_ENV":              "/dev/null",
		"ENV":                   "/dev/null",
		"DYLD_INSERT_LIBRARIES": "",
		"LD_PRELOAD":            "",
		"NODE_OPTIONS":          " ",

		// Exec: what git runs.
		"GIT_SSH_COMMAND":       "ssh",
		"GIT_EXTERNAL_DIFF":     "",
		"GIT_PAGER":             "",
		"GIT_CONFIG_SYSTEM":     "/dev/null",
		"GIT_CONFIG_PARAMETERS": "",

		// Transport: where the bearer goes and who may terminate its TLS.
		"ANTHROPIC_BASE_URL":           "https://api.anthropic.com",
		"CLAUDE_CODE_API_BASE_URL":     "",
		"HTTPS_PROXY":                  "",
		"HTTP_PROXY":                   "",
		"ALL_PROXY":                    "",
		"https_proxy":                  "",
		"http_proxy":                   "",
		"all_proxy":                    "",
		"NODE_EXTRA_CA_CERTS":          "/dev/null",
		"NODE_TLS_REJECT_UNAUTHORIZED": "1",
		"CLAUDE_CODE_CERT_STORE":       "",
		"CLAUDE_CODE_CLIENT_CERT":      "",
		"CLAUDE_CODE_CLIENT_KEY":       "",
	}
	// Why each non-empty value is the one it is — so a reader who wants to
	// "tidy" one of them to "" finds the measurement first.
	why := map[string]string{
		"BASH_ENV":                     "a path bash sources to nothing. Non-empty so it also reaches the root-owned policy tier, where an empty value was measured NOT to take (ranger-base-sn0w8)",
		"ENV":                          "same, for sh",
		"NODE_OPTIONS":                 "a single space: non-empty, and parses to no options",
		"GIT_SSH_COMMAND":              `git's own default. "" is not "no command", it is the command "" — ` + "`error: cannot run :`" + ` on every ssh remote`,
		"GIT_CONFIG_SYSTEM":            "a config file with nothing in it. Neutral on Apple git 2.50.1 by a byte-identical `git config --list --show-origin`: its bundled CommandLineTools gitconfig is read by a path this variable does not govern, so osxkeychain survives",
		"NODE_EXTRA_CA_CERTS":          "a cert file with no certs in it; the variable is additive, so adding nothing is neutral",
		"NODE_TLS_REJECT_UNAUTHORIZED": `"1" is verify-on, measured against a self-signed server; "0" is the value that completes an MITM`,
		"ANTHROPIC_BASE_URL":           "the bundle resolves ANTHROPIC_BASE_URL || CLAUDE_CODE_API_BASE_URL || this. A non-empty value in the FIRST name short-circuits the chain and closes the second one too",
	}

	got := map[string]string{}
	var order []string
	for _, v := range inletPin() {
		if _, dup := got[v.Key]; dup {
			t.Errorf("inletPin names %s twice — the later row silently wins in the rendered map", v.Key)
		}
		got[v.Key] = v.Value
		order = append(order, v.Key)
	}
	for name, val := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("inletPin does not carry %s — a name that is not in the pin is not covered by it, because a settings payload can only SET keys, never remove them", name)
			continue
		}
		if g != val {
			t.Errorf("inletPin[%s] = %q, want %q — %s", name, g, val, why[name])
		}
	}
	for _, name := range order {
		if _, ok := want[name]; !ok {
			t.Errorf("inletPin carries %s, which this table does not account for. A new row needs its own three-arm measurement (unset/attack/pin) before it lands, not just a plausible name", name)
		}
	}
}

// The two halves compose, in an order that does not move. The credential
// dirs come first because rq83c's pins already assert that order.
func TestQASettingsPinIsBothHalvesInAStableOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")

	pin := settingsPin()
	if len(pin) != len(credentialDirPin())+len(inletPin()) {
		t.Fatalf("settingsPin has %d rows, want both halves whole", len(pin))
	}
	for i, name := range credentialDirVars {
		if pin[i].Key != name {
			t.Errorf("settingsPin[%d] = %s, want %s — rq83c's pins assert the credential dirs lead", i, pin[i].Key, name)
		}
	}
	// Never empty, whatever the box looks like: the guard in the two JSON
	// producers reads len(pin)==0, and the inlet half is what makes that
	// unreachable. Asserted as the property rather than the branch.
	t.Setenv("HOME", "")
	if len(settingsPin()) == 0 {
		t.Errorf("settingsPin is empty with no home — the inlet rows name no path and must survive")
	}
}

// The payload, end to end: what {settings} renders has to carry both halves
// AND still carry the fleet policy the const is read for.
func TestQAClaudeFleetSettingsJSONCarriesTheInletPin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")

	var got struct {
		Env         map[string]string `json:"env"`
		Permissions struct {
			DefaultMode string `json:"defaultMode"`
		} `json:"permissions"`
	}
	payload := ClaudeFleetSettingsJSON()
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, payload)
	}
	if got.Permissions.DefaultMode != "auto" {
		t.Errorf("the widened merge dropped the permission mode the const is for")
	}
	for _, v := range inletPin() {
		g, ok := got.Env[v.Key]
		if !ok {
			t.Errorf("env does not carry %s — this payload is the only scope the launch can pin it at", v.Key)
		}
		if g != v.Value {
			t.Errorf("env[%s] = %q, want %q", v.Key, g, v.Value)
		}
	}
	// An empty-string row has to SURVIVE the JSON round trip as a present
	// key. `omitempty` anywhere on this path would drop most of the pin and
	// leave a payload that still looks right at a glance.
	if _, ok := got.Env["HTTPS_PROXY"]; !ok {
		t.Errorf("the empty-valued rows did not survive marshalling:\n%s", payload)
	}
}

// The line, not the payload: the inlet pin has to reach the rendered launch
// line inside the single --settings flag that line is allowed to carry.
func TestQATheRenderedClaudeLineCarriesTheInletPin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetenvForTest(t, "CLAUDE_SECURESTORAGE_CONFIG_DIR")
	unsetenvForTest(t, "CLAUDE_CONFIG_DIR")

	ag := &AgentFile{Name: "pin", Path: home + "/pin.md", MemoryDir: home + "/mem"}
	for _, tc := range []struct{ name, line string }{
		{"builtin template", ag.RenderCommandFor(claudeRuntime(t), "claude", DefaultTier)},
		{"DefaultAgentCommand (the legacy render)", ag.RenderCommand()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if n := strings.Count(tc.line, "--settings"); n != 1 {
				t.Fatalf("the line names --settings %d times, want exactly 1:\n%s", n, tc.line)
			}
			for _, v := range inletPin() {
				if !strings.Contains(tc.line, v.Key) {
					t.Errorf("the rendered line does not carry %s:\n%s", v.Key, tc.line)
				}
			}
		})
	}
}

// EnsureSettingsPin's payload is the security guarantee owed to a template
// posse did not write, and since rflee that guarantee is both halves.
func TestQAEnsureSettingsPinCarriesTheInletPinToo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claude := &Runtime{Name: "claude", Command: "claude", FleetSettings: ClaudeFleetSettingsJSON, SettingsPin: credentialDirPinJSON}

	got := claude.EnsureSettingsPin("claude --model x")
	for _, v := range inletPin() {
		if !strings.Contains(got, v.Key) {
			t.Errorf("the appended pin does not carry %s: %s", v.Key, got)
		}
	}
	// Still the pin ALONE: a hand-written command: is owed the security
	// guarantee and none of posse's fleet policy.
	if strings.Contains(got, "skillOverrides") || strings.Contains(got, "defaultMode") {
		t.Errorf("the appended payload carries fleet policy a hand-written command: never asked for: %s", got)
	}
}

// The policy-tier drop-in is the OTHER end of this fix and it is a file,
// not code, so nothing but a pin keeps it honest.
//
// Why there are two ends at all: the launcher's `--settings` payload rides
// at flag scope and therefore reaches only sessions POSSE launches. The
// session this bead's threat model is actually about is the operator's own
// uncaged claude, which no launcher flag touches — the only scope above a
// persona-writable ~/.claude/settings.json that reaches it is the root-owned
// policy tier. `etc/claude/managed-settings.d/10-posse-inlet-pin.json` is
// that file, versioned here so the change exists in config and not only on
// somebody's box; installing it is the operator's, per-change.
//
// Confirmed against the 2.1.261 bundle rather than assumed: the resolver's
// getDropInDir() joins the managed directory with "managed-settings.d",
// reads every file in it and merges each into the policy settings, treating
// ENOENT/ENOTDIR as "no drop-ins" — so the directory is real and an absent
// one is not an error.
func TestQAThePolicyDropInMatchesTheInletPin(t *testing.T) {
	t.Parallel()
	const path = "../../etc/claude/managed-settings.d/10-posse-inlet-pin.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the policy-tier half of the pin is missing: %v", err)
	}
	var got struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	want := map[string]string{}
	for _, v := range inletPin() {
		want[v.Key] = v.Value
	}
	for k, v := range want {
		g, ok := got.Env[k]
		if !ok {
			t.Errorf("%s does not carry %s — the two ends of the pin have drifted, and the end that is missing a row is the one covering the operator's own session", path, k)
			continue
		}
		if g != v {
			t.Errorf("%s[%s] = %q, want %q", path, k, g, v)
		}
	}
	for k := range got.Env {
		if _, ok := want[k]; !ok {
			t.Errorf("%s carries %s, which inletPin does not — a row here that no measurement stands behind", path, k)
		}
	}
	// The credential dirs are deliberately NOT in this file: the operator
	// already installed those two at the same tier (ranger-base-sn0w8), and
	// a second file setting them would make which value wins depend on
	// drop-in merge order.
	for _, name := range credentialDirVars {
		if _, ok := got.Env[name]; ok {
			t.Errorf("%s carries %s — already pinned in managed-settings.json; two policy sources for one key make the winner an ordering accident", path, name)
		}
	}
}
