//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-um1bi — ConfirmSubmitted's read-back on an
// occupant with no `agent explain`.
//
// WHAT WAS WRONG. ConfirmSubmitted is the only thing that says a prompt was
// typed and never submitted: MEASURED three times on 2026-09-02, `herdr
// agent prompt` answered agent_prompted, the text sat in the composer, and
// the last one sat four hours while every store reported the agent idle. The
// reading is `agent explain`'s composer preview, which is ADR 0076 §4 — so
// on a `multiplexer: rex` instance `b.explain` answered "not available" on
// the first poll, the function returned "" for every seat on every prompt,
// and the one line that reports a lost keystroke could not fire on this box
// at all. Not "the box cleared" but "the question was never asked", which is
// this family's whole distinction (panework.go's head comment).
//
// WHAT THESE ARMS PIN. The ROUTE, in both directions, which is the pair
// capturedhold_qa_test.go holds for the work half and for the same reason:
// two silences wear one error here and only one of them is permanent. A slot
// with NO such verb reaches the capture (arm 1); a herdr that WAS asked and
// FAILED does not, because that is news and may be gone by the next prompt,
// and reading a different store on a transient failure would make the
// read-back mean two things (arm 2).
//
// WHAT THEY CANNOT PIN YET, said out loud rather than papered over:
// capturedHold fills Work and leaves Typed, Sent and Ghost empty on this
// route until ranger-base-qzujz's composer reading is in the tree. So arm 1
// asserts the answer against THAT reading's own Typed rather than against a
// literal — true in both worlds, where a literal would be an assertion
// somebody has to come back and edit. What fails today if the route is taken
// away again is the capture beside it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// confirmSubmittedBox is a prompt sitting unsent in the box, in the shape a
// `rex capture --format text` puts it: the mark, the U+00A0 claude draws
// after it, then the text (capturedcomposer.go's measurement).
const confirmSubmittedBox = composerMark + " commit and close the bead once the suite is green"

// ─── arm 1: the route — a §4 refusal reaches the capture ─────────────────────

func TestQAConfirmSubmittedReadsTheCaptureWhereTheSlotHasNoAgentExplain(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	b.Slot = rexSeatWithRuntime(t)
	seat := rexTestSeat()
	rexSeedCapture(t, rexTestBlock, claudeScreenBox(
		"  I'll commit and close now.",
		confirmSubmittedBox,
		"  ⏵⏵ auto mode on · 1 shell · esc to interrupt · ← for agents"))

	got := b.ConfirmSubmitted(seat)

	// THE ROUTE, and the whole of what this bead changed: before it, the §4
	// refusal on the first poll returned "" and posse read no screen at all.
	var captured bool
	for _, c := range rexCalls(t) {
		if strings.Contains(c, "capture") && strings.Contains(c, "-b "+rexTestBlock) {
			captured = true
		}
	}
	if !captured {
		t.Fatalf("ConfirmSubmitted took no capture on a slot with no `agent explain` — it answered off the §4 refusal, which is \"the question was never asked\" reported as \"the box cleared\". rex calls were:\n%s", strings.Join(rexCalls(t), "\n"))
	}
	// THE REPORT, against the one reading that takes it rather than against a
	// literal. Equal today because the capture route's composer half is
	// ranger-base-qzujz's and not yet in the tree (both ""), and equal after
	// it lands because this function's only job on this route is to report
	// that reading and nothing of its own.
	if want := b.capturedHold(seat).Typed; got != want {
		t.Errorf("ConfirmSubmitted = %q but the capture route's own composer reading is %q — the read-back must report that reading", got, want)
	}
	t.Logf("capture-route read-back = %q (empty until ranger-base-qzujz's composer half lands)", got)
}

// ─── arm 2: a herdr that FAILED is still ignorance, not a capture ────────────

func TestQAConfirmSubmittedOnAFailedHerdrReadsNoScreen(t *testing.T) {
	t.Parallel()

	// herdr IS the slot here and its explain ERRORS — a diagnostic that did
	// not answer, which is news and may be gone by the next prompt. The
	// concession this function has always made is the right one there: a
	// reading that will not answer is not evidence of a lost keystroke. Only
	// a slot with NO such verb reaches the capture, because that silence is
	// permanent.
	//
	// The error is armed rather than provoked with an unknown pane: this
	// fake's `agent explain` answers for ANY pane (an idle fallback with no
	// evaluated rules), so a pane nobody seeded exercises the success path
	// and would make this arm vacuous.
	//
	// And the assertion is the FORK, not the answer, because "" is what both
	// the right code and the wrong code return today — the capture route's
	// composer half is still ranger-base-qzujz's. `pane read` is what the
	// wrong code SPENDS to get there (HerdrMux.Read is a herdr occupant's
	// only screen read), and the fake logs every one it serves.
	b, fake := newTestBackend(t)
	if err := os.WriteFile(filepath.Join(fake, "explain-error"),
		[]byte("internal|fake herdr: no detection for w1:p1"), 0o644); err != nil {
		t.Fatal(err)
	}

	if left := b.ConfirmSubmitted(fakeHerdrSeat); left != "" {
		t.Fatalf("a herdr explain that failed produced a read-back (%q) — a diagnostic that will not answer is not evidence that a keystroke was lost", left)
	}
	if reads, err := os.ReadFile(filepath.Join(fake, "pane-read-log")); err == nil && strings.TrimSpace(string(reads)) != "" {
		t.Fatalf("a failed herdr explain reached the capture route — it forked a screen read for panes %q, where the reading is defined over `agent explain`. That route is for a slot with no such verb, whose silence is permanent", strings.Fields(string(reads)))
	}
}
