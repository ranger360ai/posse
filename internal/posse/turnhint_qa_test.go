//go:build !posse_arm2 && !posse_arm3

package posse

// The one token that says a claude turn is in flight — ranger-base-4yefx.
//
// WHY A READING WAS NEEDED AT ALL. MEASURED 2026-11-23 on this box, one seat
// sampling its own block every 20s across its own turn boundary with a
// background task live: 36 of 36 root program_status records read `working`,
// 22 in-turn and 14 after the turn had ended. So on this instance the
// occupant's state cannot tell a seat mid-turn from a seat that finished and
// left a shell running, and the sweep that reads `working` as "somebody is in
// there" held one lane for 41.6h over a CLOSED bead (reapworking.go).
//
// The footer told them apart cleanly and one-way in the same samples:
// `esc to interrupt` in 22 of 22 in-turn footers, 0 of 14 post-turn, one
// switch, no flapping. All 36 carried `· 1 shell ·`, which is why the hint and
// the task summary have to be read off ONE screen at one instant — the pair
// is the evidence, and either half alone is not.
//
// WHAT THESE ARMS HOLD. The reading's two safety properties, which are
// opposite in kind and both load-bearing:
//
//   - the ANCHOR. Only claude's own footer feeds it. A transcript quoting the
//     hint — this bead's notes do, and so does a persona reading this file out
//     loud — must not read as a live turn, or the sweep's line is wrong in the
//     direction that is merely annoying.
//   - the DIRECTION of the match. Inside the footer it is a substring, on
//     purpose: a missed hint licenses a reap of a live turn, while a spurious
//     one keeps a seat exactly as today. A pin that demanded an exact footer
//     part would be pinning the dangerous direction.
//
// The fixture is capturedHold's: a real `rex capture --format text` shape
// (claudeScreen), which is also the route this instance actually reads on
// (ADR 0076 §4 — there is no `agent explain` here).

import (
	"testing"
)

// The measured pair, twenty seconds apart with the same background task live
// in both: sample 22 (in-turn) and sample 24 (post-turn), 2026-11-23.
const (
	turnHintFooterWorking = "  ⏵⏵ auto mode on · 1 shell · esc to interrupt · ↓ to manage"
	turnHintFooterEnded   = "  ⏵⏵ auto mode on · 1 shell · ↓ to manage"
)

func TestQAClaudesInterruptHintDiscriminatesALiveTurnFromABackgroundTask(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		transcript string
		footer     string
		work       string
		liveTurn   bool
	}{
		{
			name:       "mid-turn, one task running",
			transcript: "  I'll hold here until the arms report.",
			footer:     turnHintFooterWorking,
			work:       "1 shell",
			liveTurn:   true,
		},
		{
			name:       "the turn has ended, the task is still running",
			transcript: "  Waiting on the suite.",
			footer:     turnHintFooterEnded,
			work:       "1 shell",
			liveTurn:   false,
		},
		{
			// The anchor. The hint is in the TRANSCRIPT, above the rules,
			// which is where a persona reading reapworking.go's own comment
			// aloud puts it. Reading it there would call a settled seat
			// busy forever.
			name:       "the hint quoted in the transcript is not a turn",
			transcript: "  reapworking.go reads claude's `esc to interrupt` out of the footer.",
			footer:     turnHintFooterEnded,
			work:       "1 shell",
			liveTurn:   false,
		},
		{
			// The direction. If claude ever renders the hint with words
			// around it, the reading has to keep the seat rather than
			// license a reap of the turn it is in.
			name:       "the hint inside a longer footer part still says a turn is in flight",
			transcript: "  Thinking.",
			footer:     "  ⏵⏵ auto mode on · 1 shell · press esc to interrupt this turn · ↓ to manage",
			work:       "1 shell",
			liveTurn:   true,
		},
		{
			// Nothing running and no turn: the hold is empty, and
			// capturedHold answers the zero PaneHold — whose LiveTurn is
			// false for the reason the field's doc gives (not seen), not
			// because anything was read about a turn.
			name:       "an idle footer with nothing running holds nothing",
			transcript: "  Done; the bead is closed.",
			footer:     "  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents",
			work:       "",
			liveTurn:   false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			b.Slot = rexSeatWithRuntime(t)
			rexSeedCapture(t, rexTestBlock, claudeScreen(tc.transcript, tc.footer))

			hold := b.PaneHolding(rexTestSeat())
			if hold.Work != tc.work {
				t.Errorf("Work = %q, want %q — the hint is only ever spent beside live background work, so a wrong Work makes the pair meaningless", hold.Work, tc.work)
			}
			if hold.LiveTurn != tc.liveTurn {
				t.Errorf("LiveTurn = %v, want %v, off footer %q", hold.LiveTurn, tc.liveTurn, tc.footer)
			}
		})
	}
}

// The herdr route reads the same line out of the manifest's own region
// preview rather than off a capture, and the two must not drift: one
// vocabulary, two ways in (backgroundWorkIn's own note). Without this arm the
// field could be set on one route and silently absent on the other, which is
// exactly the shape ranger-base-r5131 was — htafy's guard live on one
// occupant and off on the other for weeks.
func TestQATheInterruptHintIsReadOnTheExplainRouteToo(t *testing.T) {
	t.Parallel()
	det := AgentDetection{EvaluatedRules: []EvaluatedRule{
		func() EvaluatedRule {
			r := EvaluatedRule{ID: "live_blocked_form", Region: footerRegion, State: "blocked"}
			r.Evidence.RegionPreview = turnHintFooterWorking
			r.Evidence.RegionBytes = len(turnHintFooterWorking)
			return r
		}(),
	}}
	hold := det.Hold()
	if hold.Work != "1 shell" {
		t.Errorf("Work = %q off the explain route, want \"1 shell\"", hold.Work)
	}
	if !hold.LiveTurn {
		t.Error("LiveTurn = false off the explain route over a footer carrying the hint — the capture route reads it, and a field set on one route only is a guard that is off wherever the other one runs")
	}
}
