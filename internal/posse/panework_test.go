package posse

// ranger-base-htafy — what a settled pane is still holding, read off the
// screen regions `herdr agent explain --json` previews (panework.go).
//
// Every footer and every prompt box in this file is a VERBATIM capture from
// the live shop on 2026-09-02 (herdr 0.8.2, claude 2.1.258, seven panes),
// including the one this bead was filed for: an agent herdr called idle
// with a re-prompt sitting unsent in its composer. The synthesized cases
// below them are the rest of claude's own task vocabulary, taken from the
// 2.1.258 binary — kinds this shop has not run yet, but that the reading
// must not read as "nothing is running".

import (
	"strings"
	"testing"
)

// detectionWith builds the shape AgentExplain decodes: a state and the
// screen regions herdr previewed while evaluating its rules. The rule ids
// are the real manifest's, and each region is carried by the rule that
// actually reads it — a preview attached to the wrong rule would pass a
// reading that keys on the region name and fail against a real herdr.
func detectionWith(state, footer, box string) AgentDetection {
	var det AgentDetection
	det.State = state
	det.EvaluatedRules = []EvaluatedRule{
		{ID: "live_blocked_form", Region: footerRegion, State: "blocked"},
		{ID: "live_prompt_box", Region: composerRegion, State: "idle"},
	}
	det.EvaluatedRules[0].Evidence.RegionPreview = footer
	det.EvaluatedRules[0].Evidence.RegionBytes = len(footer)
	det.EvaluatedRules[1].Evidence.RegionPreview = box
	det.EvaluatedRules[1].Evidence.RegionBytes = len(box)
	return det
}

// ─── the footer ──────────────────────────────────────────────────────────────

// The bead itself: an agent that went idle behind its own suite run reads
// idle in every store posse has, and its footer says what it is waiting on.
func TestBackgroundWorkReadsClaudesOwnFooterSummary(t *testing.T) {
	for _, c := range []struct {
		what, footer, want string
	}{
		// MEASURED, live panes.
		{"working, nothing running",
			"  ⏵⏵ auto mode on (shift+tab to cycle) · esc to interrupt · ← for agents\n", ""},
		{"idle, nothing running",
			"  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents\n", ""},
		{"one background shell",
			"  ⏵⏵ auto mode on · 1 shell · esc to interrupt · ← for agents · ↓ to manage\n", "1 shell"},
		{"a shell and a monitor — the bead's own state",
			"  ⏵⏵ auto mode on · 1 shell, 1 monitor · esc to interrupt · ← for agents · ↓ to manage\n", "1 shell, 1 monitor"},
		// THE CAPTURE THIS WHOLE READING RESTS ON: a pane herdr's listing
		// called idle and its detection matched live_prompt_box in — settled
		// by every reading posse had before this bead — with a background
		// shell still going. Note what the turn ending removed (`esc to
		// interrupt`) and what it did not.
		{"IDLE with one still running",
			"  ⏵⏵ auto mode on · 1 shell · ← for agents · ↓ to manage\n", "1 shell"},
		{"a blocked pane's dialog, not a footer at all",
			"  6. Chat about this\n\nEnter to select · ↑/↓ to navigate · Esc to cancel\n", ""},

		// The rest of claude 2.1.258's vocabulary for the same line.
		{"plural shells and monitors",
			"  ⏵⏵ auto mode on · 3 shells, 2 monitors · ↓ to manage\n", "3 shells, 2 monitors"},
		{"mixed kinds fold into one count",
			"  ⏵⏵ auto mode on · 4 background tasks · ↓ to manage\n", "4 background tasks"},
		{"an MCP task",
			"  ⏵⏵ auto mode on · 1 MCP task · ↓ to manage\n", "1 MCP task"},
		{"a cloud session, behind the glyph claude prefixes it with",
			"  ⏵⏵ auto mode on · ✳ 2 cloud sessions · ↓ to manage\n", "2 cloud sessions"},
		{"an Artifact comment monitor",
			"  ⏵⏵ auto mode on · 1 Artifact comment monitor · ↓ to manage\n", "1 Artifact comment monitor"},
		{"the uncounted ones",
			"  ⏵⏵ auto mode on · dreaming · ↓ to manage\n", "dreaming"},
		{"ultraplan, whose summary carries a phase",
			"  ⏵⏵ auto mode on · ✳ ultraplan needs your input · ↓ to manage\n", "ultraplan needs your input"},

		// The parts that must never read as live work, one at a time —
		// every one of them shares the line with a real summary above.
		{"the mode hint alone", "  ⏵⏵ auto mode on\n", ""},
		{"the interrupt hint alone", "  esc to interrupt\n", ""},
		{"the agents hint alone", "  ← for agents\n", ""},
		{"the manage hint alone", "  ↓ to manage\n", ""},
		{"prose that happens to hold a number and a noun",
			"  ⏵⏵ 1 shell is not what this part says\n", ""},
		{"a zero count is not work",
			"  ⏵⏵ auto mode on · 0 shells · ↓ to manage\n", ""},
	} {
		if got := detectionWith("idle", c.footer, "❯\n").BackgroundWork(); got != c.want {
			t.Errorf("%s: BackgroundWork() = %q, want %q\nfooter: %q", c.what, got, c.want, c.footer)
		}
	}
}

// ─── the composer ────────────────────────────────────────────────────────────

// The other half of the incident: a prompt that was typed and never
// submitted. The capture is the third occurrence's, taken at 13:37:16 while
// herdr's listing reported the agent `done`.
func TestComposerReadsTextLeftUnsentInThePromptBox(t *testing.T) {
	for _, c := range []struct {
		what, box, want string
	}{
		{"an empty box", "❯\n", ""},
		{"the measured unsent re-prompt",
			// U+00A0, not a space: it is what claude draws between the mark
			// and the text, and it is the byte the live capture holds.
			"❯ block hs0dl on uzyd2 and re-trigger it off the ruling\n",
			"block hs0dl on uzyd2 and re-trigger it off the ruling"},
		{"an ordinary space after the mark", "❯ commit and close the bead\n", "commit and close the bead"},
		{"a dialog drawn over the box is not typed text",
			" ☐ uzyd2 quiet gap\n\n│ uzyd2 — the meter 429 storm cannot drain while posse keeps re-asking\n", ""},
		{"no box on screen at all", "", ""},
	} {
		if got := detectionWith("idle", "", c.box).Composer(); got != c.want {
			t.Errorf("%s: Composer() = %q, want %q\nbox: %q", c.what, got, c.want, c.box)
		}
	}
}

// ─── the hold ────────────────────────────────────────────────────────────────

// Waiting() is what every decision site asks, and Why() is what it prints.
// Both halves must be able to answer alone: the bead's first two
// occurrences had a monitor and an empty box, the third had an unsent
// prompt and no monitor at all.
func TestHoldAnswersEitherHalfAlone(t *testing.T) {
	work := detectionWith("idle", "  ⏵⏵ auto mode on · 1 shell, 1 monitor · ↓ to manage\n", "❯\n").Hold()
	if !work.Waiting() || work.Typed != "" {
		t.Errorf("a monitor alone is not read as a wait: %+v", work)
	}
	if !strings.Contains(work.Why(), "1 shell, 1 monitor still running") {
		t.Errorf("Why() does not name the work: %q", work.Why())
	}

	unsent := detectionWith("done", "  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents\n",
		"❯ close 1gak4 and then close jwcxu\n").Hold()
	if !unsent.Waiting() || unsent.Work != "" {
		t.Errorf("an unsent prompt alone is not read as a wait: %+v", unsent)
	}
	if !strings.Contains(unsent.Why(), "UNSENT") || !strings.Contains(unsent.Why(), "close 1gak4") {
		t.Errorf("Why() does not show the operator what is in the box: %q", unsent.Why())
	}

	settled := detectionWith("idle", "  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents\n", "❯\n").Hold()
	if settled.Waiting() || settled.Why() != "" {
		t.Errorf("a pane holding nothing must read as settled: %+v (%q)", settled, settled.Why())
	}
}

// Ignorance is not evidence. A herdr that reports no evaluated rules — an
// older one, or a manifest whose rules no longer read those regions — must
// answer the empty hold, or every settle in the shop would be held forever
// by a reading nobody could take.
func TestAnUnreadableScreenHoldsNothing(t *testing.T) {
	var blind AgentDetection
	blind.State = "idle"
	if h := blind.Hold(); h.Waiting() {
		t.Errorf("a detection with no evaluated rules invented a hold: %+v", h)
	}
	// The same, one step out: a manifest that still evaluates rules but
	// none over the two regions this reading needs.
	other := AgentDetection{State: "idle", EvaluatedRules: []EvaluatedRule{{ID: "osc_title_idle", Region: "osc_title"}}}
	other.EvaluatedRules[0].Evidence.RegionPreview = "◐ 1 shell, 1 monitor"
	if h := other.Hold(); h.Waiting() {
		t.Errorf("a reading keyed on the region name matched another region: %+v", h)
	}
}
