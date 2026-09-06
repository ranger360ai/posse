//go:build posse_arm2

package posse

// Pins for ranger-base-i7cy4 — the command-string FIELD half of the
// settings pin, and the three ways it can go quietly wrong: a row that
// stops being rendered, a row whose TYPE the runtime rejects (which voids
// the whole payload, credential dirs and all), and a name that gets added
// here without a measurement standing behind it.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// fieldPinWant is what every producer of a settings payload must carry,
// spelled independently of fieldPin() so a mistake in the table is a red
// rather than a tautology.
var fieldPinWant = map[string]string{
	"apiKeyHelper":        `""`,
	"awsAuthRefresh":      `""`,
	"awsCredentialExport": `""`,
	"gcpAuthRefresh":      `""`,
	"otelHeadersHelper":   `""`,
	"proxyAuthHelper":     `""`,
	"fileSuggestion":      `{"command":"","type":"command"}`,
	"statusLine":          `{"command":"","type":"command"}`,
	"subagentStatusLine":  `{"command":"","type":"command"}`,
}

// fieldOf pulls one top-level key out of a rendered payload as compact
// JSON, so an object row can be compared without depending on key order.
func fieldOf(t *testing.T, payload, key string) (string, bool) {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("payload is not JSON: %v\n%s", err, payload)
	}
	raw, ok := m[key]
	if !ok {
		return "", false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s is not JSON: %v", key, err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s does not re-marshal: %v", key, err)
	}
	return string(b), true
}

// The launch line's payload. An env pin cannot reach a field, so a launch
// that carries the 23 env rows and none of these is a launch that closed
// the transport inlets and left the exec-by-field ones open.
func TestQAClaudeFleetSettingsJSONCarriesTheFieldPin(t *testing.T) {
	t.Parallel()
	got := ClaudeFleetSettingsJSON()
	for key, want := range fieldPinWant {
		g, ok := fieldOf(t, got, key)
		if !ok {
			t.Errorf("the launch payload does not carry %s — an env pin cannot reach a field, so this row is the only thing closing it", key)
			continue
		}
		if g != want {
			t.Errorf("%s = %s, want %s", key, g, want)
		}
	}
	// The fleet's own settings and the env pin still travel in the same
	// payload: a second --settings replaces the first, so anything that
	// falls out of this object is not on the line at all.
	if !strings.Contains(got, "defaultMode") {
		t.Errorf("the field pin displaced the fleet's permission mode: %s", got)
	}
	if _, ok := fieldOf(t, got, "env"); !ok {
		t.Errorf("the field pin displaced the env pin: %s", got)
	}
}

// The payload a hand-written `command:` gets appended (EnsureSettingsPin).
// It is owed the same guarantee and none of the fleet policy.
func TestQATheAppendedPinCarriesTheFieldPin(t *testing.T) {
	t.Parallel()
	got := credentialDirPinJSON()
	for key, want := range fieldPinWant {
		g, ok := fieldOf(t, got, key)
		if !ok {
			t.Errorf("the appended pin does not carry %s", key)
			continue
		}
		if g != want {
			t.Errorf("%s = %s, want %s", key, g, want)
		}
	}
	if strings.Contains(got, "skillOverrides") || strings.Contains(got, "defaultMode") {
		t.Errorf("the appended payload carries fleet policy a hand-written command: never asked for: %s", got)
	}
}

// The runtime's own list of command-carrying settings fields (2.1.261,
// `uo`). Every name in it is either a pinned row or a documented omission —
// there is no third state, because silence is exactly how ranger-base-rflee
// left GIT_CONFIG_* uncovered and unnoticed.
var runtimeCommandFields = []string{
	"apiKeyHelper", "awsAuthRefresh", "awsCredentialExport", "fileSuggestion",
	"gcpAuthRefresh", "otelHeadersHelper", "processWrapper", "policyHelpers",
	"proxyAuthHelper", "statusLine", "subagentStatusLine",
}

func TestQAEveryCommandFieldIsPinnedOrNamedUnpinned(t *testing.T) {
	t.Parallel()
	pinned := map[string]bool{}
	for _, f := range fieldPin() {
		pinned[f.Key] = true
	}
	for _, name := range runtimeCommandFields {
		_, excused := fieldPinUnpinned[name]
		switch {
		case pinned[name] && excused:
			t.Errorf("%s is both pinned and excused — one of the two is stale", name)
		case !pinned[name] && !excused:
			t.Errorf("%s carries a command and is neither pinned nor named in fieldPinUnpinned with a reason; a reader cannot see that it is uncovered", name)
		}
	}
	for name := range fieldPinUnpinned {
		found := false
		for _, n := range runtimeCommandFields {
			if n == name {
				found = true
			}
		}
		if !found {
			t.Errorf("fieldPinUnpinned excuses %s, which is not a command field the runtime reads", name)
		}
	}
	for _, f := range fieldPin() {
		found := false
		for _, n := range runtimeCommandFields {
			if n == f.Key {
				found = true
			}
		}
		if !found {
			t.Errorf("fieldPin carries %s, which is not in the runtime's own list — a row no measurement stands behind", f.Key)
		}
	}
}

// processWrapper is the row whose absence is load-bearing. Its resolver is
// `[policy, flag, user].find(v => typeof v === "string" && v !== "")`, so a
// "" row would read as pinned and leave a persona's value winning. If a
// later hand adds it, this says why not to add it empty.
func TestQAProcessWrapperIsNotPinnedEmpty(t *testing.T) {
	t.Parallel()
	for _, producer := range []struct {
		name, payload string
	}{
		{"the launch payload", ClaudeFleetSettingsJSON()},
		{"the appended pin", credentialDirPinJSON()},
	} {
		got, ok := fieldOf(t, producer.payload, "processWrapper")
		if !ok {
			continue
		}
		if got == `""` {
			t.Errorf("%s pins processWrapper empty, which its resolver SKIPS — the pin looks made and a persona's value still wins", producer.name)
		}
	}
}

// The policy-tier half of this fix is a file, not code, so nothing but a
// pin keeps it honest — the same argument as
// TestQAThePolicyDropInMatchesTheInletPin, for the other end of the same
// launch. This file is what covers the operator's OWN uncaged claude, which
// no --settings flag reaches.
func TestQAThePolicyDropInMatchesTheFieldPin(t *testing.T) {
	t.Parallel()
	const path = "../../etc/claude/managed-settings.d/20-posse-field-pin.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the policy-tier half of the field pin is missing: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	for key, want := range fieldPinWant {
		raw, ok := got[key]
		if !ok {
			t.Errorf("%s does not carry %s — the two ends of the pin have drifted, and the end missing a row is the one covering the operator's own session", path, key)
			continue
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("%s[%s] is not JSON: %v", path, key, err)
		}
		c, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s[%s] does not re-marshal: %v", path, key, err)
		}
		if string(c) != want {
			t.Errorf("%s[%s] = %s, want %s", path, key, c, want)
		}
	}
	for key := range got {
		if _, ok := fieldPinWant[key]; !ok {
			t.Errorf("%s carries %s, which fieldPin does not — a row here that no measurement stands behind", path, key)
		}
	}
}

// The hooks half moved out of this file with the template it graded.
// `etc/claude/21-posse-hooks-lockdown.json.in` is gone and its successor is
// an ordinary drop-in beside the other two, `30-posse-hooks.json`; the pins
// are in policyhooks_qa_test.go, which grades the shipped command STRINGS by
// running them and not just the file's shape. What retired the template is a
// measurement its own bead could not make: a hook command is shell-expanded,
// so `$HOME` carries the two paths and neither a placeholder nor one box's
// home directory has to ship (ranger-base-bm9cd).
