package posse

// QA pin written verifying ranger-base-k5fnr under ranger-base-qvdyr, filed
// as finding 1 of ranger-base-8dnuy.
//
// ADR 0026's status line said "prompt/skill implementation deferred" for a
// day after the prompt implementation landed (efa4aaa3) and after the skill
// surfaces were censused as needing no change. The clause was written by
// 62d7a3f3 before the code existed and was never revisited, while the
// IDENTICAL clause one file over — docs/adr/0005-work-prompt-blueprints.md,
// "The 2026-09-05 ruling is implemented in the rendered rung as of
// ranger-base-k5fnr" — WAS updated in the landing commit. So two ADRs
// contradicted each other on main about whether the work was done.
//
// This is the same defect shape as
// TestQAADR0036StatusLineDoesNotCarryTheRetractedUnbuiltStamp
// (verify_i9dbb_qa_test.go): a status line asserting a now-false unbuilt
// state, in a shipped record, with nothing but a reader's memory holding it
// true. The guard is the reader.
//
// It bans a CLAUSE, never the word "deferred": a deferral is live convention
// in this tree and ADR 0027's status line legitimately carries "code
// deferred" today. Asserting on 0027 is deliberately NOT done — building
// 0027's code is a legitimate change that would red a control here for no
// defect.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAADR0026StatusLineDoesNotDeferTheImplementedRung(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(qibRepoRoot(t), "docs", "adr", "0026-research-spikes.md"))
	if err != nil {
		t.Fatalf("the guard must read the file it judges: %v", err)
	}
	adr := string(b)

	// Positive witness first: "the clause is absent" is equally true of a
	// read that got the wrong file (pass-count-is-not-a-coverage-floor).
	// Witnessed on the record's identity and on the presence of a status
	// line, NOT on a bead id a legitimate rewording may drop.
	if !strings.HasPrefix(adr, "# ADR 0026") {
		t.Fatalf("this guard is not reading ADR 0026 — first line %q", strings.SplitN(adr, "\n", 2)[0])
	}
	if !strings.Contains(adr, "*Status:") {
		t.Fatal("ADR 0026 has no status line — this guard has nothing to judge")
	}

	for _, dead := range []string{
		"prompt/skill implementation deferred",
		"implementation deferred",
	} {
		if strings.Contains(adr, dead) {
			t.Errorf("0026's status carries %q — the prompt implementation landed in efa4aaa3 (the SPIKE rung in internal/posse/dispatch.go) and the skill surfaces were censused as needing no change. ADR 0005 line 137 says so already; leaving this here makes the two records contradict", dead)
		}
	}

	// And the reason, measured rather than asserted from the ADR's own
	// prose: the ruling is in the rendered rung. If this ever goes false the
	// deferral is no longer a falsehood and this whole guard should be
	// revisited.
	if !strings.Contains(EscalationLadder("b-1", ""), "research it in THIS bead when the question is bounded") {
		t.Error("the rendered SPIKE rung no longer carries the 2026-09-05 ruling — the premise of this guard (the deferral is false) no longer holds")
	}
}
