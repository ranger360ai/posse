package main

// ranger-base-g27p4, the operator-visible half. The refusal itself
// (CheckRecipeCmdAgent, pinned over LaunchRecipe in
// internal/posse/recipecmdagent_qa_test.go) only reaches the operator AFTER
// they wrote the recipe and ran it; the catalog reaches them before. Pinned
// for TestCmdHelpNamesTheAgentRefusal's reason one layer down, and asserted
// through helpcatalog_test.go's own commandOf so the rule cannot drift under
// a neighbouring command while still reading as well-formed (rangerhq-6izv's
// class, which is why that helper exists).

import (
	"strings"
	"testing"
)

func TestRecipeHelpNamesTheCommandAgentRefusal(t *testing.T) {
	out := helpText(t)
	// The fix, in the spelling the seed recipe prescribes and the refusal
	// prints — and under `posse recipe` and not under its neighbours.
	if cmd := commandOf(t, out, "command: null"); cmd != "posse recipe <name>" {
		t.Errorf("the `command: null` rule sits under %q, not `posse recipe <name>`", cmd)
	}
	// And it states the rule rather than only the prohibition: a reader told
	// the pair is illegal, without being told the persona's line is RENDERED,
	// writes it the other way round.
	block := recipeHelpBlock(t, out)
	for _, want := range []string{"agent:", "command: null", "rendered", "PID"} {
		if !strings.Contains(block, want) {
			t.Errorf("the `posse recipe` entry does not name %q:\n%s", want, block)
		}
	}
}

// recipeHelpBlock is the catalog's `posse recipe <name>` entry: its own line
// and every continuation indented under it, up to the next command header.
// flagHelpBlock's body over the catalog's verb column rather than its flag
// column.
func recipeHelpBlock(t *testing.T, out string) string {
	t.Helper()
	var b strings.Builder
	in := false
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "  posse ") {
			if in {
				break
			}
			in = strings.HasPrefix(ln, "  posse recipe <name>")
		}
		if in {
			b.WriteString(ln + "\n")
		}
	}
	if b.Len() == 0 {
		t.Fatalf("the usage catalog no longer has a `posse recipe <name>` entry — this pin is reading nothing")
	}
	return b.String()
}
