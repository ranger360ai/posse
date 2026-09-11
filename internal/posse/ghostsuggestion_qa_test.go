//go:build posse_arm2

package posse

// QA pins for ranger-base-l51p1: claude's own next-prompt SUGGESTION is not
// a prompt anybody has to send, and a seat waiting on its own work files no
// row whatever its box previews.
//
// THE INCIDENT (2026-09-10 ~22:30Z, watch pid 10513, posse 0.4.0+062b765e).
// Two dispatched seats settled with their suite shells still running. G2
// filed settled-unsent for both, the coordinator sent the settle text with
// `posse prompt --now`, and both seats took it and went back to waiting. On
// the NEXT tick both were settled-unsent again — the boxes now previewing
// "keep waiting, then commit and close" and "ping me when it's green",
// which are claude's own suggestions and were typed by nobody. A
// coordinator that obeys that row types a suggestion into a working seat,
// every tick, for as long as the suite runs.
//
// WHY THE READING CANNOT TELL. `prompt_box_body` is one plain-text region
// (herdr 0.8.2, claude manifest 2026.09.04.1 — no typed-vs-suggested rule,
// field or region anywhere in it), and a suggestion previews there character
// for character like typed input. Nor is there a store to check it against:
// claude logs SUBMITS (sentline.go) and never suggestions, so the
// ranger-base-2hvtv echo reading answers "not an echo" for every one of
// them — correctly, and uselessly.
//
// SO LIVE WORK DECIDES, and the box does not get a vote while there is any.
// That is a DEFERRAL and not a silencing, which is the whole reason it is
// safe: when the work ends, the footer's task summary goes with it, and a
// box that really is holding an unsent prompt files its -unsent row on the
// next tick, unchanged. Every arm below is paired with that wrong arm.
//
// WHAT IS LEFT, said out loud: a seat with NOTHING running whose box
// previews a suggestion is still filed settled-unsent, because the row is
// right (a settled holder) and only its subtype is wrong. Fixing that needs
// a discriminator this bead measured half of — `herdr agent read --format
// ansi` draws the suggestion dim, `❯ <ESC>[0m<ESC>[2m…<ESC>[0m`, measured on
// a live pane 2026-09-11T07:22Z — and the other half of that measurement, an
// unsent TYPED line in the same read, is not something this bead could
// stage. It is filed rather than guessed at.

import (
	"strings"
	"testing"
)

// The two suggestions off the incident, verbatim, in the shape herdr
// previews a composer in (settlewaiting_qa_test.go's fixtures are the other
// two shapes of the same region).
const (
	ghostSuggestWait  = "❯ keep waiting, then commit and close\n"
	ghostSuggestGreen = "❯ ping me when it's green\n"
)

// govG2RowOn is govG2Row (ghostcomposer_qa_test.go) with the FOOTER under
// the caller's control: the footer is what carries claude's live-task
// summary, and live work is what decides this row.
func govG2RowOn(t *testing.T, footer, box string, submitted []string) *GovCondition {
	t.Helper()
	b, fake := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	sessionAgent(t, fake, "developer-x", "idle")
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})
	armScreen(t, fake, footer, box)
	armPaneSession(t, fake, probeSession)
	armSubmitted(t, b, probeSession, submitted...)
	return find(shopSet(t, govIn(t, b)), "G2")
}

// THE BEAD. A seat idle behind its own suite run is not settled-but-holding
// and files no row — whatever claude has drawn in its box. The store is
// armed with a line that is NOT the box, which is what a suggestion always
// looks like to it: claude never logged one.
func TestQAGovG2DropsAWaitingHolderWhateverItsBoxPreviews(t *testing.T) {
	for _, c := range []struct{ what, box string }{
		{"the first seat's suggestion", ghostSuggestWait},
		{"the second seat's suggestion", ghostSuggestGreen},
	} {
		if g := govG2RowOn(t, busyFooter, c.box, []string{"an older line"}); g != nil {
			t.Errorf("%s: a seat waiting on its own suite run was filed as %s — that row sends the coordinator to type the suggestion back in: %q",
				c.what, g.Key, g.Detail)
		}
	}
}

// THE WRONG ARM, and the reason the one above is a deferral rather than a
// third narrowing: the same two boxes, the same pass, one field different —
// nothing is running behind the pane — and the -unsent row stands exactly as
// it did before this bead.
func TestQAGovG2StillRaisesUnsentWhenNothingIsRunning(t *testing.T) {
	for _, c := range []struct{ what, box string }{
		{"the first seat's box", ghostSuggestWait},
		{"the second seat's box", ghostSuggestGreen},
	} {
		g := govG2RowOn(t, idleFooter, c.box, []string{"an older line"})
		if g == nil {
			t.Fatalf("%s: a settled holder dropped off the surface entirely", c.what)
		}
		if g.Key != "settled-unsent:bd-1" {
			t.Errorf("%s: G2 key = %q, want settled-unsent:bd-1 — a box nobody is running behind is still the operator's to clear",
				c.what, g.Key)
		}
		if !strings.Contains(g.Detail, "UNSENT") {
			t.Errorf("%s: the row does not tell the operator what to fix: %q", c.what, g.Detail)
		}
	}
}

// The control both of them rest on, because "no row" is also what a pass
// that never reached the rung prints: the same waiting seat with an EMPTY
// box filed no row before this bead either, and a herdr that shows no screen
// at all still files the plain settled row.
func TestQAGovG2WaitingAndEmptyIsUnchangedAndIgnoranceStillFiles(t *testing.T) {
	if g := govG2RowOn(t, busyFooter, emptyBox, []string{"an older line"}); g != nil {
		t.Errorf("the ranger-base-htafy drop stopped working: %+v", *g)
	}
	g := govG2RowOn(t, idleFooter, emptyBox, []string{"an older line"})
	if g == nil || g.Key != "settled:bd-1" {
		t.Fatalf("the settled-but-holding row stopped firing for a holder that really did stop: %+v", g)
	}
}
