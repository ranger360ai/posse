//go:build posse_arm3

package posse

// ADR 0006 §4, verifying ranger-base-0ezn7 (the QA lane, ranger-base-ps10r).
//
// The removal's own census (acceptanceexplicit_qa_test.go) bans the deleted
// mechanism by NAME — `closerDoneWhen`, `IntentDoneWhen`, `markdownRows(`,
// and the two wire strings it rendered. That is the right list for the code
// that was deleted, and it is the wrong list for the code that could come
// back: a rebuild under fresh identifiers, rendering `- checklist (<closer> ·
// <intent>): <cell>`, spells none of those nine strings and the census stays
// green. (It would red the behavioural pins in verifyafter_test.go, whose
// "gone" lists carry the fixture PID's own cells — but only for as long as
// those fixtures keep a PID whose table the rebuild happens to read.)
//
// So this pin is on the OPERATION instead of the name, which no rename can
// move: to infer anything from the closer's PID, this file must first OPEN
// one, and every door into a PID on disk is one of the five symbols below.
// §4's promise is that none of them is needed — "nothing but the bead's own
// id reaches this line" — and at 953f0be, the commit before the removal,
// `LoadAgent` appeared in verifyafter.go twice, once for each deleted helper.
//
// MUTATION (restored): put `ag, err := a.LoadAgent(closer)` back into
// verifySection and this reds naming the line; point the read at agents.go
// and the two-way control reds instead, because a scan that cannot see the
// token where it IS present proves nothing where it is absent.

import (
	"os"
	"strings"
	"testing"
)

// pidDoors is every symbol that opens a persona document off disk. A caller
// that reaches a PID names one of them: LoadAgent reads the file, ListAgents
// and AgentsDir walk the directory, CanonAgent resolves a name to one, and
// AgentFile is the only type the parsed result comes back in.
var pidDoors = []string{"LoadAgent", "ListAgents", "CanonAgent", "AgentsDir", "AgentFile"}

func TestQAVerifyAfterOpensNoCloserPID(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("verifyafter.go")
	if err != nil {
		t.Fatal(err)
	}
	// The two-way control, and it comes first: these five names must be
	// findable by this scan somewhere, or the absence below is a property of
	// the reader and not of verifyafter.go (ranger-base-ps10r).
	owner, err := os.ReadFile("agents.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, door := range pidDoors {
		if !strings.Contains(string(owner), door) {
			t.Fatalf("control: agents.go does not name %q, so this scan cannot show that "+
				"verifyafter.go is free of it — the door list is stale, not the file", door)
		}
	}
	for i, line := range strings.Split(string(src), "\n") {
		for _, door := range pidDoors {
			if strings.Contains(line, door) {
				t.Errorf("verifyafter.go:%d reaches a persona document through %q — ADR 0006 §4 "+
					"makes acceptance the closed bead's own, and nothing but that bead's id may reach "+
					"the acceptance line (ranger-base-0ezn7 deleted the two LoadAgent calls this had):\n\t%s",
					i+1, door, strings.TrimSpace(line))
			}
		}
	}
}
