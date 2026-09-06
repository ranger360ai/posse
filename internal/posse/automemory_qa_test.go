package posse

import (
	"encoding/json"
	"strings"
	"testing"
)

// ranger-base-7uhip: claude resolves its auto-memory directory from the git
// WORKING-COPY root, never the session cwd, so every seat launched into
// ~/.posse/worktrees/posse/<session> appended its lessons to the OPERATOR's
// project memory index — measured at 199 of a 200-line cap, ~100 distinct
// session ids, and not one of the 1470 per-worktree project dirs holding a
// memory/ subdir of its own.
//
// The fix is one key in the launch payload, so the pin is one parse of it.
// Parsed and not string-matched for the reason TestFleetSettingsCarryPermission
// ModeAuto gives: key order inside the blob is not the contract.
//
// BOTH forms are checked on purpose. The const is what a reader of the launch
// line looks at; ClaudeFleetSettingsJSON() is what the line actually carries,
// and it re-marshals the const through a map — a merge that drops or retypes
// the key would leave the const green and ship the defect.
func TestQAFleetSettingsDisableAutoMemory(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{"ClaudeFleetSettings", ClaudeFleetSettings},
		{"ClaudeFleetSettingsJSON", ClaudeFleetSettingsJSON()},
	} {
		var m map[string]any
		if err := json.Unmarshal([]byte(tc.payload), &m); err != nil {
			t.Fatalf("%s is not valid JSON: %v\n%s", tc.name, err, tc.payload)
		}
		v, ok := m["autoMemoryEnabled"]
		if !ok {
			t.Errorf("%s does not carry autoMemoryEnabled — every seat's auto-memory lands back in the operator's ~/.claude/projects/<main checkout>/memory/MEMORY.md, whose harness cap is 200 lines (ranger-base-7uhip):\n%s", tc.name, tc.payload)
			continue
		}
		b, isBool := v.(bool)
		if !isBool {
			// One wrong-typed row voids the WHOLE --settings payload in
			// silence (ranger-base-i7cy4), taking the credential dirs and
			// the permission mode with it. The schema types this key
			// boolean; a string "false" is not it.
			t.Errorf("%s autoMemoryEnabled = %#v, want a JSON boolean — a wrong-typed row makes the runtime discard the entire payload without a word", tc.name, v)
			continue
		}
		if b {
			t.Errorf("%s autoMemoryEnabled = true: auto-memory is back ON for the fleet", tc.name)
		}
	}
}

// And the key has to reach the launch LINE, not just the payload function:
// {settings} is rendered into the claude template, and a template that stopped
// naming it would leave this const true-but-unread.
func TestQAFleetLaunchLineDisablesAutoMemory(t *testing.T) {
	t.Parallel()
	def := loadTestAgent(t, "---\nname: p\n---\nYou are p.\n")
	got := def.RenderCommand()
	if !strings.Contains(got, "--settings "+shellQuote(ClaudeFleetSettingsJSON())) {
		t.Fatalf("default claude command does not carry the fleet settings payload:\n%s", got)
	}
	if !strings.Contains(got, `"autoMemoryEnabled":false`) {
		t.Errorf("the rendered launch line does not disable auto-memory (ranger-base-7uhip):\n%s", got)
	}
}
