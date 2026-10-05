//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-zt45t — ADR 0067 D5's three rules for lines the
// binary writes, checked over turnOutcomeLines' output.
//
// Claim: every line turnOutcomeLines writes states one fact or one
// instruction and not both, joins no fact to an instruction with a dash,
// uses the dictionary's state words and none of A3's rejected ones, puts the
// instruction's condition first, dates the fact, and stays under the 25-word
// cap.
//
// WHERE D5 SAYS TO PUT IT. "Each rule is checked where the line is rendered,
// by the rendering function's own unit test, which names its process (ADR
// 0006 §7); no tree-wide pin reads prose for this, and this appendix is not
// a fixture." So this pin reads one function's return value and no file —
// nothing in it walks the tree, and it needs no Makefile door.
//
// THE PROCESS IT PROTECTS: a settle clause grows one bead at a time. The
// blind arm came with ranger-base-02zr and the unobserved arm with
// ranger-base-1mei, and each wrote the fact it had just made readable, the
// condition a reader has to rule out, and what to run — three things, one
// sentence, because a clause inside somebody else's parenthesis has nowhere
// else to put them. Nobody writes a 36-word line; a third arm appends to a
// 24-word one. The cap and the line count below are what a third arm has to
// get past.
//
// NOT this pin's subject: WHICH fact each arm states. That is pinned on the
// line an operator actually reads, through production Run, by
// TestQABlindAndUntrustedBothLandOnTheSettleLine and
// TestQAUnobservedTurnOutcomeSettleLineIsNamed (both arm 2). A rewrite that
// collapsed the two arms into one would be green here and red there, which
// is the right way round: the arms are a behaviour and this is a shape.

import (
	"strings"
	"testing"
	"time"
)

// adr0067WordCap is D5's "a new line over 25 words is split".
const adr0067WordCap = 25

func TestQATurnOutcomeLinesMeetADR0067LineRules(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 4, 14, 3, 22, 0, time.Local)
	const session = "ranger-posse-a-1"
	looked := TurnOutcomeReader(func(string, string, time.Time) (TurnOutcome, bool) {
		return TurnOutcome{}, false
	})

	// A reader that answered is no lines at all — the two rules a clause
	// cannot break are the ones it does not print (the healthy arm's own pin
	// is TestQAReadableRuntimeSettleCarriesNoBlindnessClause).
	if got := turnOutcomeLines(looked, "", session, true, at); got != "" {
		t.Errorf("an answered turn must print nothing, got %q", got)
	}

	for _, c := range []struct {
		name  string
		find  TurnOutcomeReader
		runtm string
	}{
		{"blind: no reader declared", nil, "codex"},
		{"unobserved: a reader looked and found none", looked, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := turnOutcomeLines(c.find, c.runtm, session, false, at)
			if !strings.HasSuffix(got, "\n") {
				t.Fatalf("the block must end its last line, got %q", got)
			}
			lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("want one fact line and one instruction line, got %d:\n%s", len(lines), got)
			}

			// Rule 1, the half a word count cannot see: the fact carries no
			// instruction and the instruction leads with its condition. The
			// dash is the shape the old 36-word clause had — fact, dash,
			// explanation, "so", command — and the one D5 names outright.
			for i, line := range lines {
				body, ok := strings.CutPrefix(line, "  ↳ ")
				if !ok {
					t.Errorf("line %d is not a continuation line of the settle line above it: %q", i+1, line)
					continue
				}
				if n := len(strings.Fields(body)); n > adr0067WordCap {
					t.Errorf("line %d is %d words, over ADR 0067 D5's cap of %d: %q", i+1, n, adr0067WordCap, body)
				}
				if strings.ContainsAny(body, "—–") {
					t.Errorf("line %d joins its parts with a dash, which D5's first rule names: %q", i+1, body)
				}
				// A3's rejected state words, each of which has been read two
				// ways on a surface D3 governs. `settles` and `refused` below
				// are the rows they point at.
				for _, no := range []string{"idle", "deferred", "merged", "snoozed", "shelved", "on hold", "finished", "exited", "cleaned up", "pruned"} {
					if strings.Contains(body, no) {
						t.Errorf("line %d uses the not-approved state word %q (ADR 0067 A3): %q", i+1, no, body)
					}
				}
			}

			fact, instruction := lines[0], lines[1]

			// Rule 3: the fact's own clock, in the format the pass header and
			// the seat-idle report already use, and the clock it was HANDED
			// — a line that reads time.Now() for itself cannot be dated by a
			// test and is not the line's own time either.
			stamp := at.Local().Format(turnOutcomeStamp)
			if !strings.HasPrefix(fact, "  ↳ "+stamp+" ") {
				t.Errorf("the fact must lead with its own timestamp %q (ADR 0067 D5 rule 3): %q", stamp, fact)
			}

			// Rule 1: one fact per line. The fact says what posse does not
			// have and stops; the command and its condition are the next
			// line's.
			for _, no := range []string{"posse peek", "if ", "so "} {
				if strings.Contains(fact, no) {
					t.Errorf("the fact line carries the instruction's %q — D5's first rule puts it on its own line: %q", no, fact)
				}
			}
			if !strings.HasPrefix(instruction, "  ↳ if ") {
				t.Errorf("the instruction must lead with its condition (D5: \"condition first\"): %q", instruction)
			}
			for _, want := range []string{
				// Rule 2, read off the dictionary: a seat SETTLES (A2;
				// never "idle", A3) after an account REFUSED the turn (A2's
				// rung-outcome row).
				"a seat settles exactly like this",
				"refused the turn",
				// The instruction is useless without the seat to run it on.
				"posse peek " + session,
			} {
				if !strings.Contains(instruction, want) {
					t.Errorf("the instruction must carry %q: %q", want, instruction)
				}
			}
		})
	}
}
