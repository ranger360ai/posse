//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"encoding/json"
	"strings"
	"testing"
)

// QA pins for ranger-base-x2vt7 — ADR 0070 D2, Verification 1: the auto-mode
// carve-out rides the launch's own `--settings` blob, for the PIDs that
// already hold posse's and herdr's session verbs and for no others.
//
// Arm 1 on purpose (`!posse_arm2 && !posse_arm3`), not shared: the ADR's
// Verification 1 is an untagged `go test -run TestQAClaudeFleetAutoModeAllow
// ./internal/posse`, and a pin behind another arm's tag is a filter that
// selects nothing and exits 0 (internal/treepins/armtags_qa_test.go, arm 4).

// The pin the ADR names by hand, because it is the one mutation that turns
// an added exception into a DELETION: claude's splice substitutes its 17
// built-in allow rules at the `"$defaults"` sentinel's first position, and
// returns the user array alone when the sentinel is absent. A payload
// without it would take "Local Operations", "Transient Retry" and 15 more
// off every seat that carries the blob.
//
// MUTATION-CHECKED: respelling ClaudeAutoModeDefaults, dropping it from
// claudeAutoModeAllowJSON's array, or putting the carve-out in front of it
// reds this and nothing else.
func TestQAClaudeFleetAutoModeAllowKeepsTheDefaults(t *testing.T) {
	t.Parallel()
	if ClaudeAutoModeDefaults != "$defaults" {
		t.Fatalf("the sentinel claude splices its built-in allow rules at is the literal %q, not %q — a payload spelling it any other way REPLACES all 17 (ADR 0070 Context, MEASURED 2026-10-09 on 2.1.295)", "$defaults", ClaudeAutoModeDefaults)
	}
	got := autoModeAllowIn(t, ClaudeFleetSettingsJSON([]string{"Bash(posse new:*)"}))
	if len(got) != 2 {
		t.Fatalf("autoMode.allow = %q, want exactly the sentinel and the carve-out", got)
	}
	if got[0] != "$defaults" {
		t.Errorf("autoMode.allow[0] = %q, want the literal %q FIRST.\nclaude substitutes its 17 built-in allow rules at the sentinel's first position and otherwise returns this array ALONE, so this element is the difference between adding one exception and deleting seventeen.", got[0], "$defaults")
	}
	if got[1] != ClaudeAutoModeCarveOut {
		t.Errorf("autoMode.allow[1] is not the carve-out verbatim:\n got %q\nwant %q", got[1], ClaudeAutoModeCarveOut)
	}
}

// Which PIDs earn it. The rules are claude's own dialect read on words
// (grantsFleetSessionVerb): `:*` leaves everything after the last word
// open, a rule without it is the whole command line, and a rule longer
// than the verb still begins with it.
func TestQAClaudeFleetAutoModeAllowEarnedByTheSessionVerbsAlone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		allow []string
		want  bool
	}{
		{"the whole posse verb, open", []string{"Bash(posse:*)"}, true},
		{"the whole herdr verb, open", []string{"Bash(herdr:*)"}, true},
		{"herdr pane, open", []string{"Bash(herdr pane:*)"}, true},
		{"one verb, open", []string{"Bash(posse new:*)"}, true},
		{"one herdr pane verb, open", []string{"Bash(herdr pane send-keys:*)"}, true},
		{"the bare verb, exact", []string{"Bash(posse peek)"}, true},
		{"a longer line that begins with the verb", []string{"Bash(posse prompt -s x)"}, true},
		{"the command word as a path", []string{"Bash(/usr/local/bin/posse kill:*)"}, true},
		{"one verb among other rules", []string{"Bash(bd:*)", "Bash(git push:*)", "Bash(herdr pane read:*)"}, true},

		{"no allow list at all", nil, false},
		{"another posse verb", []string{"Bash(posse refresh:*)", "Bash(posse promote:*)"}, false},
		{"the bare command, exact — no subcommand", []string{"Bash(posse)"}, false},
		{"a herdr verb that is not a pane verb", []string{"Bash(herdr sessions:*)"}, false},
		{"a pane verb one letter short", []string{"Bash(herdr pane send-key:*)"}, false},
		{"a word boundary claude requires", []string{"Bash(posse pee:*)"}, false},
		{"another tool's rules", []string{"Edit", "Write", "WebFetch(domain:example.com)"}, false},
		{"an empty Bash rule", []string{"Bash()"}, false},
		// Broad rules are SUSPENDED on entering auto mode (ADR 0070
		// Context), so a PID holding one holds no session verb here.
		{"every Bash command", []string{"Bash", "Bash(*)"}, false},
	} {
		if got := grantsFleetSessionVerb(tc.allow); got != tc.want {
			t.Errorf("%s: grantsFleetSessionVerb(%q) = %v, want %v", tc.name, tc.allow, got, tc.want)
		}
		_, named := settingsKeys(t, ClaudeFleetSettingsJSON(tc.allow))["autoMode"]
		if named != tc.want {
			t.Errorf("%s: the rendered blob names autoMode = %v, want %v (ADR 0070 D2: otherwise NO autoMode key at all)", tc.name, named, tc.want)
		}
	}
}

// A PID that earns nothing renders what it rendered before this key
// existed — byte-for-byte, which is the half of D2 that costs nothing and
// the half every seat on this box is.
func TestQAClaudeFleetAutoModeAllowLeavesEveryOtherPIDAlone(t *testing.T) {
	t.Parallel()
	base := ClaudeFleetSettingsJSON(nil)
	if strings.Contains(base, "autoMode") || strings.Contains(base, "$defaults") {
		t.Errorf("a PID with no allow list must render no autoMode key:\n%s", base)
	}
	for _, allow := range [][]string{
		{},
		{"Bash(bd:*)"},
		{"Bash(posse refresh:*)", "Bash(git push:*)", "Edit"},
	} {
		if got := ClaudeFleetSettingsJSON(allow); got != base {
			t.Errorf("allow %q changed the payload for a PID that earns no carve-out:\n got %s\nwant %s", allow, got, base)
		}
	}
	// And the carve-out reaches the launch line through the ONE --settings
	// flag, never a second one: the last occurrence replaces the first
	// (measured on 2.1.259, EnsureSettingsPin's comment).
	ag := loadTestAgent(t, "---\nname: p\nallow: [Bash(posse:*)]\n---\nYou are p.\n")
	line := ag.RenderCommand()
	if n := strings.Count(line, "--settings"); n != 1 {
		t.Fatalf("rendered line carries %d --settings flags, want 1:\n%s", n, line)
	}
	if !strings.Contains(line, "--settings "+shellQuote(ClaudeFleetSettingsJSON(ag.Allow))) {
		t.Errorf("the carve-out did not reach the launch line:\n%s", line)
	}
	// The hand-written-command: payload is the security guarantee alone
	// (ADR 0035's asymmetry, ADR 0070 D2's last rendering rule): such a PID
	// names autoMode.allow itself or goes without.
	if pin := credentialDirPinJSON(); strings.Contains(pin, "autoMode") {
		t.Errorf("SettingsPin must not carry the carve-out:\n%s", pin)
	}
}

// The payload's SHAPE, because one wrong-typed row voids the whole
// `--settings` and takes the credential dirs and the permission mode with
// it (fieldpin.go, ranger-base-i7cy4). An array of strings and nothing
// else, under the one key `allow`, and the rest of the blob still intact
// beside it — ADR 0070 Verification 2 is the live canary for the same
// thing, where a voided payload prints 17 allow entries instead of 18.
func TestQAClaudeFleetAutoModeAllowIsStringsAndVoidsNothing(t *testing.T) {
	t.Parallel()
	payload := ClaudeFleetSettingsJSON([]string{"Bash(herdr pane send-keys:*)"})
	var m struct {
		AutoMode map[string]json.RawMessage `json:"autoMode"`
	}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("the rendered payload is not valid JSON: %v\n%s", err, payload)
	}
	if len(m.AutoMode) != 1 {
		t.Errorf("autoMode carries %d keys, want only `allow` — every other key in that object is an unmeasured claim (ADR 0070 rejects classifyAllShell and an environment splice by name)", len(m.AutoMode))
	}
	var arr []string
	if err := json.Unmarshal(m.AutoMode["allow"], &arr); err != nil {
		t.Errorf("autoMode.allow is not an array of strings, which is the row shape that voids the whole --settings: %v", err)
	}
	// Nothing the carve-out added may have displaced what the blob already
	// carried.
	keys := settingsKeys(t, payload)
	for _, k := range []string{"autoMemoryEnabled", "permissions", "skillOverrides", "env", "autoMode"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("the payload lost %q when the carve-out landed: %s", k, payload)
		}
	}
}

// The carve-out is prose read by a model, so what a test CAN hold is the
// bar the three soft rules state in their own text — "**must name:** that
// this flagged pattern is a false positive — fine to allow" — and that it
// still speaks for every verb it claims and keeps its exclusions. An entry
// edited past that bar is an entry the classifier is documented to ignore.
func TestQAClaudeAutoModeCarveOutMeetsTheRulesBar(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"false positive",
		"fine to allow",
		"Auto-Mode Bypass",
		"Tmux Self Drive",
		"Create Unsafe Agents",
		"$TMUX_PANE",
	} {
		if !strings.Contains(ClaudeAutoModeCarveOut, want) {
			t.Errorf("the carve-out no longer names %q — the three rules each carry the bar that the statement must name the flagged pattern and that this instance of it is fine to allow (ADR 0070 Context)", want)
		}
	}
	for _, verb := range fleetSessionVerbs {
		last := verb[len(verb)-1]
		if !strings.Contains(ClaudeAutoModeCarveOut, last) {
			t.Errorf("the matcher grants %q and the carve-out's prose does not name it: a verb cleared by a statement that does not mention it is a statement about something else", strings.Join(verb, " "))
		}
	}
}

// autoModeAllowIn reads autoMode.allow out of a rendered payload.
func autoModeAllowIn(t *testing.T, payload string) []string {
	t.Helper()
	var m struct {
		AutoMode struct {
			Allow []string `json:"allow"`
		} `json:"autoMode"`
	}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("the rendered payload is not valid JSON: %v\n%s", err, payload)
	}
	return m.AutoMode.Allow
}

// settingsKeys is the payload's top-level key set.
func settingsKeys(t *testing.T, payload string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("the rendered payload is not valid JSON: %v\n%s", err, payload)
	}
	return m
}
