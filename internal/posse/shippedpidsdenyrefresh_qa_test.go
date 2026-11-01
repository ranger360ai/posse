//go:build !posse_arm2 && !posse_arm3

package posse

// TestShippedPIDsDenyRefresh, moved out of refresh_test.go by
// ranger-base-8yv7z. IT IS HERE FOR ITS ARM, the same reason
// shelfpidlint_qa_test.go and policydropins_qa_test.go are in files of their
// own (ranger-base-nt0pi and this bead).
//
// The pin ENUMERATES the shelf — every examples/agents PID denies the verb —
// so retiring or adding a reference PID reds it, which is what the tree-wide
// class in internal/treepins/treewidedoor_qa_test.go means and what a
// Makefile door ($(QA_SEED_PINS), `make seed-check`) is for. A door's `-run`
// runs with NO `-tags` (TestQAEveryMakefileDoorPinIsReachableInTheDefaultArm)
// and a filter that matches nothing exits 0, so behind `//go:build
// posse_arm2` — where the rest of refresh_test.go sits — a door would have
// been green over a pin it never ran. The pin moves to arm 1 rather than the
// door mechanism learning to pass a tag, which no door does today; that tag
// question is ranger-base-m94g2's and nothing here prejudges it.
//
// Nothing else in refresh_test.go moved: the rest of that file drives the
// refresh verb against planted homes and is nobody's tree-wide anything.
//
// AND ITS ROOT COMES FROM qibRepoRoot. It spelled
// `dir := filepath.Join("..", "..", "examples", "agents")` and read the shelf
// through that local, which reaches the tree while computing no root at all —
// so every rule the register derives internal/posse's half of the class from
// was blind to it. Its sibling TestShippedPIDsDenyPromote (promote_test.go)
// was spelled identically and is fixed in place; it was already in arm 1.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The binary refuses a persona session and a pipe (the tests above). ADR 0019
// D4 asks for the guardrail twice, prose and rule, so the deny line has to be
// in the PIDs `posse init` lays down — otherwise a fresh instance carries the
// gate once and the crew's own file never says the verb is not theirs
// (ranger-base-kryn). Same shape as TestShippedPIDsDenyPromote, and for the
// same reason: politeness against a determined session.
func TestShippedPIDsDenyRefresh(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not the `../..` climb this used to hold in a local
	// (ranger-base-8yv7z) — see the head comment.
	dir := filepath.Join(qibRepoRoot(t), "examples", "agents")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seed := &App{AgentsDir: dir}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		n++
		ag, err := seed.LoadAgent(strings.TrimSuffix(e.Name(), ".md"))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		found := false
		for _, d := range ag.Deny {
			if d == "Bash(posse refresh:*)" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not deny Bash(posse refresh:*) (ADR 0019 D4)", e.Name())
			continue
		}
		// And it has to be a rule L1 can realize, or the second spelling is
		// prose on the tier most of the fleet runs at — and parity would call
		// every launch DEGRADED, which is how a fence becomes muscle memory
		// for --allow-degraded.
		var refreshRule shimRule
		for _, r := range ParseShimRules(ag.Deny)["posse"] {
			if strings.Contains(r.Rule, "refresh") {
				refreshRule = r
			}
		}
		if refreshRule.Rule == "" {
			t.Errorf("%s: the refresh deny does not parse into a posse shim rule", e.Name())
			continue
		}
		if kind, faithful := matcherFor("posse", refreshRule); !faithful {
			t.Errorf("%s: %s realizes only as %q — parity will call every launch DEGRADED", e.Name(), refreshRule.Rule, kind)
		}
	}
	if n < 5 {
		t.Fatalf("read only %d shipped PIDs", n)
	}
}
