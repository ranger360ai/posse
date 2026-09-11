//go:build posse_arm2

package posse

// QA pins for ranger-base-6o7wm: a composer drawing claude's own suggestion
// draws it FAINT, and posse reads the escapes back.
//
// THE HALF ranger-base-l51p1 LEFT. That bead dropped the G2 row while a seat
// has live background work — the incident it was filed for — and said out
// loud what it did not reach: a settled seat with NOTHING running whose box
// previews a suggestion is filed `settled-unsent:` on every tick, by a row
// that cannot go false, because nobody typed the text and so nobody can clear
// it. The discriminator is in ghostbox.go and the corpus behind it is on the
// bead: 60 dim suggestions, 17 empty boxes carrying no dim run at all, 0
// typed lines, over 77 reads of every idle claude pane on one box.
//
// EVERY ARM HERE IS PAIRED WITH ITS WRONG ARM, one field different: the SAME
// composer text, previewing identically through `explain`, with no dim run in
// the ansi read. Without that pair, "the row lost its -unsent subtype" is
// also what a pass that stopped reading composers prints.
//
// AND ONE ARM IS THE REFUSAL. dispatch's --resume re-prompt is the only
// caller of this reading that TYPES, and it still skips on a dim box —
// because the other half of the measurement (that a TYPED line is not drawn
// dim too) does not exist yet. TestQAResumeStillRefusesToTypeIntoAGhostBox is
// that line, and it is the one test here that must go red the day somebody
// finishes the measurement and wires it through.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the corpus ──────────────────────────────────────────────────────────────

// The two shapes measured 2026-09-11T07:22Z-08:19Z (herdr 0.8.2, claude
// manifest 2026.09.04.1), as `agent read --format ansi` hands them over:
// SGR 0 then SGR 2 opening at the text and a reset closing it, and an empty
// box carrying no SGR run anywhere.
const (
	ghostWaitText = "keep waiting, then commit and close"
	ghostWaitANSI = "prompt$ claude\n\n" +
		"\x1b[2m╭──────────────╮\x1b[0m\n" +
		"❯ \x1b[0m\x1b[2m" + ghostWaitText + "\x1b[0m\n"
	// The wrong arm, and the whole measurement: the same characters with no
	// dim run over them. This is what a TYPED line is ASSUMED to look like
	// and what the corpus could not stage — see the file note.
	typedWaitANSI = "prompt$ claude\n\n❯ " + ghostWaitText + "\n"
	emptyBoxANSI  = "prompt$ claude\n\n❯ \n"
)

// armComposerANSI is what the fake herdr shows when posse asks for the
// attributes rather than the characters.
func armComposerANSI(t *testing.T, fake, ansi string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fake, "composer-ansi"), []byte(ansi), 0o644); err != nil {
		t.Fatal(err)
	}
}

// suggestBox is the `explain` preview that goes with ghostWaitANSI:
// character for character what a typed line previews as, which is the defect.
const suggestBox = "❯ " + ghostWaitText + "\n"

// ─── the reading ─────────────────────────────────────────────────────────────

// composerIsGhost answers on the DRAWN STATE and not on the measured bytes:
// a claude or a herdr that spells the same faint line differently is still
// drawing it faint. Every row that answers false is a row where posse must
// keep calling the text a hold.
func TestComposerIsGhostReadsTheDrawnState(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		what, ansi, plain string
		want              bool
	}{
		// MEASURED, live panes.
		{"the measured suggestion", ghostWaitANSI, ghostWaitText, true},
		{"the same characters with no dim run — a typed line", typedWaitANSI, ghostWaitText, false},
		{"an empty box carries no dim run to confuse it", emptyBoxANSI, "", false},

		// The state, not the bytes.
		{"one sequence rather than two", "❯ \x1b[0;2m" + ghostWaitText + "\x1b[0m\n", ghostWaitText, true},
		{"the run broken in two, still covering every character",
			"❯ \x1b[2mkeep waiting,\x1b[0m\x1b[2m then commit and close\x1b[0m\n", ghostWaitText, true},
		{"faint turned off by 22 partway through",
			"❯ \x1b[2mkeep waiting,\x1b[22m then commit and close\x1b[0m\n", ghostWaitText, false},
		{"only the first word drawn faint",
			"❯ \x1b[2mkeep\x1b[0m waiting, then commit and close\n", ghostWaitText, false},
		{"colour before the run does not undim it",
			"❯ \x1b[33m\x1b[2m" + ghostWaitText + "\x1b[0m\n", ghostWaitText, true},

		// The sub-parameter trap: the 2 in a direct-RGB colour selects a
		// COLOUR SPACE, not faint. A grey line is not a suggestion.
		{"a 38;2;r;g;b grey is a colour and not a dim",
			"❯ \x1b[38;2;128;128;128m" + ghostWaitText + "\x1b[0m\n", ghostWaitText, false},
		{"a 38;5;n grey is a colour and not a dim",
			"❯ \x1b[38;5;244m" + ghostWaitText + "\x1b[0m\n", ghostWaitText, false},

		// The join: two calls, two instants, two snapshots.
		{"the ansi read and the preview disagree about the text",
			"❯ \x1b[2msomething else entirely\x1b[0m\n", ghostWaitText, false},
		{"no composer on screen at all", "prompt$ echo hi\nhi\n", ghostWaitText, false},
		{"nothing previewed to join to", ghostWaitANSI, "", false},
		{"a dim box with nothing printing in it is not a suggestion",
			"❯ \x1b[2m   \x1b[0m\n", "", false},

		// The live composer is the LAST mark on the screen; a suggestion
		// scrolled off above it decides nothing.
		{"an older dim line in the scrollback does not answer for the live box",
			"❯ \x1b[2m" + ghostWaitText + "\x1b[0m\n\n❯ " + ghostWaitText + "\n", ghostWaitText, false},
		{"the live box is read past the scrollback above it",
			"❯ an older line\n\n❯ \x1b[2m" + ghostWaitText + "\x1b[0m\n", ghostWaitText, true},

		// Terminal chrome that is not an attribute must come out of the text
		// without being counted as printing characters.
		{"an erase-to-end-of-line inside the run",
			"❯ \x1b[2m" + ghostWaitText + "\x1b[K\x1b[0m\n", ghostWaitText, true},
		{"an OSC title sequence inside the run",
			"❯ \x1b[2m\x1b]0;claude\x07" + ghostWaitText + "\x1b[0m\n", ghostWaitText, true},
	} {
		if got := composerIsGhost(c.ansi, c.plain); got != c.want {
			t.Errorf("%s: composerIsGhost = %v, want %v\nansi:  %q\nplain: %q", c.what, got, c.want, c.ansi, c.plain)
		}
	}
}

// ─── the hold ────────────────────────────────────────────────────────────────

// PaneHolding is where the reading is made, so every caller comes right at
// once. A dim box is Ghost and not Typed; the same box undimmed is the hold
// it has always been.
func TestQAPaneHoldingSeparatesASuggestionFromATypedLine(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	armScreen(t, fake, idleFooter, suggestBox)

	armComposerANSI(t, fake, ghostWaitANSI)
	ghost := b.PaneHolding("w1:p1")
	if ghost.Typed != "" {
		t.Errorf("a suggestion claude wrote itself is still a prompt sitting unsent: %+v", ghost)
	}
	if ghost.Ghost != ghostWaitText {
		t.Errorf("the row does not carry what the box is previewing: %+v", ghost)
	}
	if ghost.Waiting() {
		t.Errorf("a seat with nothing running and nothing typed reads as waiting: %+v", ghost)
	}

	// THE WRONG ARM. Same preview, same footer, same everything — one dim
	// run less — and the hold is exactly what it was before this bead.
	armComposerANSI(t, fake, typedWaitANSI)
	typed := b.PaneHolding("w1:p1")
	if typed.Ghost != "" {
		t.Errorf("a line nobody drew faint was called claude's own: %+v", typed)
	}
	if typed.Typed != ghostWaitText || !typed.Waiting() {
		t.Errorf("an unsent prompt stopped being a hold: %+v", typed)
	}
	if !strings.Contains(typed.Why(), "UNSENT") {
		t.Errorf("Why() no longer tells the operator what to fix: %q", typed.Why())
	}
}

// Ignorance is not evidence that claude wrote the text. A herdr that will not
// read the pane leaves the box a hold, exactly as it was before the reading
// existed — the same direction every other reading in this family fails.
func TestQAAPaneHerdrWillNotReadIsStillAHold(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	armScreen(t, fake, idleFooter, suggestBox)
	armComposerANSI(t, fake, ghostWaitANSI)
	if err := os.WriteFile(filepath.Join(fake, "agent-read-error"),
		[]byte("timeout|no response from the herdr server"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := b.PaneHolding("w1:p1")
	if h.Ghost != "" || h.Typed != ghostWaitText || !h.Waiting() {
		t.Fatalf("a herdr that could not read the pane decided the box anyway: %+v", h)
	}
}

// The store of record outranks the screen. A box echoing the line this pane
// last SUBMITTED is answered by claude's own submit log (ranger-base-2hvtv)
// and never reaches the ansi read — including when the screen would also have
// called it a ghost, which is the ordering this pins.
func TestQATheSubmitLogAnswersBeforeTheScreenDoes(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	armScreen(t, fake, idleFooter, suggestBox)
	armComposerANSI(t, fake, ghostWaitANSI)
	armPaneSession(t, fake, probeSession)
	armSubmitted(t, b, probeSession, "an older line", ghostWaitText)

	h := b.PaneHolding("w1:p1")
	if h.Sent != ghostWaitText {
		t.Fatalf("the submit log stopped answering for a box that echoes it: %+v", h)
	}
	if h.Ghost != "" || h.Typed != "" {
		t.Fatalf("two readings claimed the same box: %+v", h)
	}
}

// ─── govern's G2 row ─────────────────────────────────────────────────────────

// govGhostRow is TestQAGovG2KeepsAHolderWithAnUnsentPrompt's rig with the
// ansi read under the caller's control.
func govGhostRow(t *testing.T, box, ansi string) *GovCondition {
	t.Helper()
	b, fake := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	sessionAgent(t, fake, "developer-x", "idle")
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})
	armScreen(t, fake, idleFooter, box)
	armComposerANSI(t, fake, ansi)
	return find(shopSet(t, govIn(t, b)), "G2")
}

// THE BEAD. A settled seat with nothing running whose box previews claude's
// own suggestion is a plain settled holder — not a coordinator's errand to go
// and clear a line claude wrote.
func TestQAGovG2DoesNotCallASuggestionAnUnsentPrompt(t *testing.T) {
	g := govGhostRow(t, suggestBox, ghostWaitANSI)
	if g == nil {
		t.Fatal("a settled holder dropped off the surface entirely")
	}
	if g.Key != "settled:bd-1" {
		t.Fatalf("G2 key = %q, want settled:bd-1 — the coordinator is still being sent to clear claude's own suggestion", g.Key)
	}
	if !strings.Contains(g.Detail, "claude's own suggestion") || !strings.Contains(g.Detail, ghostWaitText) {
		t.Errorf("the row does not say which reading changed its shape: %q", g.Detail)
	}
}

// THE WRONG ARM. The same box with no dim run over it is the -unsent row it
// has always been, and it still tells the operator what to fix.
func TestQAGovG2KeepsUnsentWhenNothingIsDrawnFaint(t *testing.T) {
	g := govGhostRow(t, suggestBox, typedWaitANSI)
	if g == nil {
		t.Fatal("a session whose prompt never landed dropped off the surface entirely")
	}
	if g.Key != "settled-unsent:bd-1" {
		t.Fatalf("G2 key = %q, want settled-unsent:bd-1", g.Key)
	}
	if !strings.Contains(g.Detail, "UNSENT") {
		t.Errorf("the row does not tell the operator what to fix: %q", g.Detail)
	}
}

// The control both of the above rest on: an EMPTY box says nothing about a
// suggestion. Without it, "the key is settled:bd-1" is also what a pass that
// never read a composer prints.
func TestQAGovG2SaysNothingAboutASuggestionOnAnEmptyBox(t *testing.T) {
	g := govGhostRow(t, emptyBox, emptyBoxANSI)
	if g == nil || g.Key != "settled:bd-1" {
		t.Fatalf("the settled-but-holding row stopped firing for a holder that really did stop: %+v", g)
	}
	if strings.Contains(g.Detail, "suggestion") {
		t.Errorf("an empty box was reported as previewing a suggestion: %q", g.Detail)
	}
}

// ─── the settle judgement ────────────────────────────────────────────────────

// ghostSettlePass is settleWaitingPass with the ansi read armed: one resuming
// pass over a bead whose persona settles without closing it.
func ghostSettlePass(t *testing.T, box, ansi string) (string, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := settleRepo(t)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	armScreen(t, fake, idleFooter, box)
	armComposerANSI(t, fake, ansi)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	return dispatcherOut(d), repo
}

// The heaviest of the three callers, and the "forever" in the bead's title:
// a settled bead whose box previews a suggestion was never JUDGED, so the
// claim was kept and the seat never refilled — for as long as claude kept
// drawing the suggestion, which is every tick.
func TestQAASuggestionDoesNotHoldASettleUnjudged(t *testing.T) {
	t.Parallel()
	out, repo := ghostSettlePass(t, suggestBox, ghostWaitANSI)
	if strings.Contains(out, "waiting, not judged this pass") {
		t.Fatalf("a settle was held unjudged by a line claude wrote itself:\n%s", out)
	}
	if !strings.Contains(out, `settled "idle" but issue is "in_progress"`) {
		t.Fatalf("the pass never reached a settle-without-close:\n%s", out)
	}
	if cs := readComments(t, repo); len(cs) != 1 {
		t.Fatalf("the pass recorded %d settle-opens, want 1: %v", len(cs), cs)
	}
}

// THE WRONG ARM: the same box undimmed is a prompt that never landed, and the
// pass holds the settle exactly as ranger-base-htafy left it.
func TestQAAnUndimmedBoxStillHoldsTheSettle(t *testing.T) {
	t.Parallel()
	out, repo := ghostSettlePass(t, suggestBox, typedWaitANSI)
	if !strings.Contains(out, "waiting, not judged this pass") || !strings.Contains(out, "UNSENT") {
		t.Fatalf("the pass judged a settle over a prompt that never landed:\n%s", out)
	}
	if cs := readComments(t, repo); len(cs) != 0 {
		t.Fatalf("the pass counted %d settle-open(s) against a prompt that never landed: %v", len(cs), cs)
	}
}

// ─── the one caller that types ───────────────────────────────────────────────

// THE REFUSAL, and the only test here whose subject is a measurement that has
// not been taken. `--resume` types into the box. A dim box is claude's own
// suggestion by everything measured so far, but that a TYPED line is not
// ALSO drawn dim is measured zero times — and if it were, this line would
// re-prompt on top of a prompt somebody is still writing. So it keeps
// refusing, and says in its own output what would retire the refusal.
//
// This test goes red the day somebody finishes the measurement and wires it
// through. That is the point of it: the refusal is deliberate and dated, not
// an oversight for a later reader to quietly delete.
func TestQAResumeStillRefusesToTypeIntoAGhostBox(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := settleRepo(t)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	armScreen(t, fake, idleFooter, suggestBox)
	armComposerANSI(t, fake, ghostWaitANSI)

	for i := 0; i < 2; i++ {
		if _, err := d.Run("", "", 0); err != nil {
			t.Fatalf("pass %d: %v", i+1, err)
		}
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "not re-prompted") || !strings.Contains(out, "claude's own suggestion") {
		t.Fatalf("a resuming pass typed into a box drawing claude's own suggestion:\n%s", out)
	}
	if !strings.Contains(out, "a TYPED line has not been measured undimmed") {
		t.Errorf("the skip does not say what would retire it:\n%s", out)
	}
	_ = repo
}

// ─── the read-back ───────────────────────────────────────────────────────────

// `posse prompt` warns that its own text is still sitting in the box. The box
// a SUCCESSFUL submit leaves behind is usually not empty — claude writes its
// next suggestion into it — so that warning fired on delivered prompts.
func TestQAConfirmSubmittedDoesNotWarnAboutASuggestion(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	armScreen(t, fake, idleFooter, suggestBox)

	armComposerANSI(t, fake, ghostWaitANSI)
	if left := b.ConfirmSubmitted("w1:p1"); left != "" {
		t.Errorf("a suggestion claude drew after the submit was reported as unsubmitted text: %q", left)
	}
	// THE WRONG ARM: the same box undimmed is the prompt that never left,
	// and the warning is the one ranger-base-htafy measured three times.
	armComposerANSI(t, fake, typedWaitANSI)
	if left := b.ConfirmSubmitted("w1:p1"); left != ghostWaitText {
		t.Errorf("a prompt still sitting in the box read as submitted: %q", left)
	}
}
