//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-r5131 — htafy's hold on an occupant with no `agent explain`.
//
// THE INCIDENT THIS PINS. A seat that ends its turn with a background shell
// running is read `done` by every store posse has on Rex, so dispatch
// counted a settle-open against it and re-prompted it; a second one files a
// `-l question` bead and escalates to the operator. The guard already
// existed — ranger-base-htafy's PaneHold — and it was off: it read herdr
// region previews through `HerdrBackend.explain`, which is ADR 0076 §4 and
// answers "not available" on a `multiplexer: rex` instance, so every settled
// seat on this box answered the empty hold from the day the slot changed.
// MEASURED 2026-11-10 on this host in the seat that filed the bead, which
// took its own settle-open comment at 01:15Z while taking the measurement.
//
// WHY THESE ARMS. The two halves that can each silently undo the fix are the
// ROUTE (does a §4 refusal reach the capture at all, and does a herdr that
// merely failed still answer the empty hold) and the ANCHOR (does the
// vocabulary see only claude's footer, or the whole screen). The second is
// the one that bites in the dangerous direction: a hold invented out of
// prose keeps a genuinely stuck bead claimed forever, and this bead's own
// notes quote `· 1 shell ·` in running text.
//
// The fixture is a real capture, trimmed: `rex capture --format text` of a
// live claude 2.1.322 block, MEASURED 2026-11-10 at 60 lines / 8317 bytes,
// whose tail is a rule, the composer and its U+00A0, a rule, a blank, then
// the footer.

import (
	"path/filepath"
	"strings"
	"testing"
)

// claudeScreen is the shape of a captured claude screen: some transcript,
// then the composer between two rules, then the footer. The footer is the
// caller's to supply, because it is the only part these arms vary.
func claudeScreen(transcript, footer string) string {
	return claudeScreenBox(transcript, "❯ ", footer)
}

// claudeScreenBox is claudeScreen with the composer's contents given, for the
// arms whose subject is the BOX rather than the footer
// (confirmsubmitted_qa_test.go, ranger-base-um1bi). One spelling of the
// screen's shape, because the two readings are located against the same two
// rules and a fixture that drifted apart would hide a reading that had.
func claudeScreenBox(transcript, box, footer string) string {
	rule := strings.Repeat(string(footerRuleRune), 78)
	return strings.Join([]string{
		transcript,
		"",
		rule,
		box,
		rule,
		"",
		footer,
	}, "\n")
}

// ─── arm 1: the route — a §4 refusal reaches the capture ─────────────────────

func TestQAACaptureCarriesTheHoldWhereTheSlotHasNoAgentExplain(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.Slot = rexSeatWithRuntime(t)
	// The footer exactly as it was measured on this box, middle dots and all.
	rexSeedCapture(t, rexTestBlock, claudeScreen(
		"  I'll hold here until the arms report.",
		"  ⏵⏵ auto mode on · 1 shell · esc to interrupt · ← for agents"))

	hold := b.PaneHolding(rexTestSeat())
	if hold.Work != "1 shell" {
		t.Fatalf("PaneHolding().Work = %q, want \"1 shell\" — on an occupant with no `agent explain` the footer is the ONLY store that carries it (program_status holds one root record saying `done`, and the process table is the block-shell→claude chain only; both MEASURED 2026-11-10)", hold.Work)
	}
	if !hold.Waiting() {
		t.Error("Waiting() = false over live background work — this is the whole consequence: gather counts a settle-open, --resume re-prompts, and the second one escalates to the operator")
	}
	if got := hold.Why(); !strings.Contains(got, "1 shell still running") {
		t.Errorf("Why() = %q, want it to name the work — it is what the ◷ line and the readings record both quote", got)
	}
	// ADR 0066 D1: the consequence sites log hold.Read, so a withheld settle
	// whose record named no region could not say what withheld it.
	var named bool
	for _, r := range hold.Read.Regions {
		if r.Name == captureFooterRegion {
			named = true
			if !strings.Contains(r.Text, "1 shell") {
				t.Errorf("the %s region text does not hold the footer: %q", captureFooterRegion, r.Text)
			}
		}
		if r.Name == footerRegion {
			t.Errorf("the capture route recorded herdr's own region name %q — the two are the same line located two different ways, and a census that cannot tell them apart cannot say which route withheld a settle", footerRegion)
		}
	}
	if !named {
		t.Errorf("no %s region in the record: %+v", captureFooterRegion, hold.Read.Regions)
	}
	if hold.Read.Seen {
		t.Error("Read.Seen = true on the capture route — Seen means herdr recognized a screen, and herdr was never asked")
	}
	// This fixture's box is EMPTY, and that is now the only reason the three
	// composer fields are: ranger-base-qzujz found the runtime-session store
	// on this occupant (ADR 0076 Amendment 6) and the composer half reads
	// here too, off this same capture. An empty box is still an empty hold,
	// so the arm stands — what moved is the sentence that used to say the
	// reading could not be taken at all. The composer arms are in
	// capturedcomposer_qa_test.go.
	if hold.Typed != "" || hold.Sent != "" || hold.Ghost != "" {
		t.Errorf("the capture route read a composer out of an EMPTY box (Typed=%q Sent=%q Ghost=%q) — `❯` and its U+00A0 are the box, not text in it", hold.Typed, hold.Sent, hold.Ghost)
	}
}

func TestQAANoFooterWorkOnTheCaptureRouteIsTheEmptyHold(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.Slot = rexSeatWithRuntime(t)
	rexSeedCapture(t, rexTestBlock, claudeScreen(
		"  Done; the bead is closed.",
		"  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents"))

	if hold := b.PaneHolding(rexTestSeat()); hold.Waiting() {
		t.Fatalf("a settled seat with an idle footer read as waiting (%q) — a hold invented here keeps a genuinely stuck bead claimed forever", hold.Why())
	}
}

// ─── arm 2: the anchor — only claude's footer feeds the vocabulary ───────────

func TestQAATheTaskVocabularyOnlyEverSeesTheFooter(t *testing.T) {
	t.Parallel()

	// This bead's own prose, on screen above the composer. It carries the
	// vocabulary between middle dots on purpose: a free scan of the screen
	// matches it, and the resulting hold can never go false, because nobody
	// can clear a line of transcript.
	b, _ := newTestBackend(t)
	b.Slot = rexSeatWithRuntime(t)
	rexSeedCapture(t, rexTestBlock, claudeScreen(
		"  the footer read · 1 shell · in all of samples 01-19, so\n"+
			"  the fact exists · 2 monitors · on Rex too",
		"  ⏵⏵ auto mode on (shift+tab to cycle)"))

	if hold := b.PaneHolding(rexTestSeat()); hold.Waiting() {
		t.Fatalf("transcript prose quoting the task vocabulary was read as live work (%q) — the reading must be positional, anchored on the last horizontal rule, and never a search of the screen", hold.Why())
	}
}

func TestQAACaptureFooterIsAfterTheLastRule(t *testing.T) {
	t.Parallel()

	rule := strings.Repeat(string(footerRuleRune), 40)
	for _, c := range []struct {
		name, screen, want string
	}{
		{"footer after the last rule", claudeScreen("hi", "  the footer"), "the footer"},
		{"no rule at all is no footer", "just lines\nand more", ""},
		{"a rule as the last line is no footer", "a\n" + rule, ""},
		{"a long dash inside prose is not a rule", "a\n" + rule + "\nx " + rule + " y\n  the footer", "x " + rule + " y\n  the footer"},
		{"blank lines around the footer are trimmed", "a\n" + rule + "\n\n\n  the footer\n\n", "the footer"},
	} {
		if got := captureFooter(c.screen); got != c.want {
			t.Errorf("%s: captureFooter = %q, want %q", c.name, got, c.want)
		}
	}
}

// ─── arm 3: a herdr that FAILED is still ignorance, not a capture ────────────

func TestQAAHerdrThatFailedDoesNotFallBackToTheCapture(t *testing.T) {
	t.Parallel()

	// herdr IS the slot here and its explain errors — a diagnostic that did
	// not answer. The note at the top of panework.go is about exactly this
	// case and it must not move: ignorance is not evidence either way, and
	// taking a capture instead would make a transient herdr failure read a
	// DIFFERENT store than the one the reading is defined over.
	//
	// THE ERROR IS ARMED AND NOT PROVOKED WITH AN UNKNOWN PANE
	// (ranger-base-1tt2e, from ranger-base-s3a9g). This fake's `agent
	// explain` explains ANY pane — an idle fallback with no evaluated rules
	// — so `Seat{Pane: "no-such-pane:0"}` exercised the SUCCESS path, and
	// this arm then passed on the absence of regions in a reading that had
	// never failed at all. MEASURED: replacing the muxVerbMissing branch and
	// the `return PaneHold{}` under it with a bare
	// `return b.capturedHold(target)` left this arm green and the whole
	// package green. confirmsubmitted_qa_test.go:86 is the worked example,
	// and that arm's own comment already says why it had to.
	b, fake := newTestBackend(t)
	write(t, filepath.Join(fake, "explain-error"), "internal|fake herdr: no detection for w1:p1")
	// And the screen the WRONG code would read is seeded on the route it
	// would take — `pane read`, a herdr occupant's only screen read — so the
	// fork shows up in the HOLD and not only in what it spent. With no screen
	// there the fallback answers the empty hold too, and the assertion below
	// could not tell a capture from a refusal.
	write(t, filepath.Join(fake, "pane-text", "w1_p1"),
		claudeScreen("x", "  ⏵⏵ auto mode on · 1 shell"))

	hold := b.PaneHolding(fakeHerdrSeat)
	if hold.Waiting() || len(hold.Read.Regions) != 0 {
		t.Fatalf("a herdr explain that failed produced a reading (%+v) — only a slot with NO such verb (ADR 0076 §4) reaches the capture, because that silence is permanent and this one is news", hold)
	}
}
