package posse

// The composer half of the settled-pane hold on an occupant with no `agent
// explain` — ranger-base-qzujz, from ranger-base-r5131.
//
// WHAT WAS MISSING. ranger-base-r5131 realized the WORK half of
// ranger-base-htafy's hold on a `multiplexer: rex` instance by reading
// claude's footer off a capture (panework.go's capturedHold), and refused the
// COMPOSER half outright: without the runtime's own conversation id the echo
// reading of sentline.go cannot be asked, so every box previewing the line
// the pane last submitted would have read as a prompt sitting UNSENT. That
// invents holds, and an invented hold keeps a genuinely stuck bead claimed
// forever — the one direction panework.go refuses. runtimesession.go is the
// id; this is the reading it unlocks.
//
// THE REGION IS LOCATED, NOT ASKED FOR, the same way captureFooter locates
// the footer and for the same reason: the slot has no detection manifest to
// name a region in. claude draws the composer BETWEEN THE LAST TWO
// HORIZONTAL RULES of its screen — MEASURED 2026-11-10 on this host (linux,
// rex CLI 0.0.1794069975, claude 2.1.322), `rex capture --format text` of a
// live block, four screens taken across the states this reading has to tell
// apart:
//
//	❯<U+00A0>                                     an empty box
//	❯<U+00A0>Try "create a util logging.py that…"  claude's own suggestion
//	❯<U+00A0>this prompt is sitting unsent in the box
//	❯<U+00A0>this prompt is sitting unsent in the box and now it gets long enough to wrap
//	  across more than one screen line because the terminal is eighty columns
//	  wide and this sentence is not
//
// Positional and never a search, for arm 2's reason in
// capturedhold_qa_test.go: a scan of the screen for a `❯` would find this
// file's own note quoting one, and a hold invented out of a quotation can
// never go false.
//
// A WRAPPED BOX IS WHY THE ECHO COMPARISON IS WHITESPACE-INSENSITIVE, and
// that is the one place this route differs from herdr's. herdr's region
// preview is a single string that may be TRUNCATED (243 characters,
// panework.go) and is never wrapped, so submittedEcho compares for equality
// and accepts a prefix only when herdr says it cut the region. A capture is
// the opposite: whole, and wrapped at the terminal's width by the renderer —
// the fourth screen above is three lines for one prompt, broken at spaces
// that are in the submitted text. So the reading rejoins the lines and the
// comparison collapses whitespace runs on BOTH sides, which makes a screen
// wrap and a real space indistinguishable in exactly the way they are
// indistinguishable on screen.
//
// WHAT THAT CANNOT SEE, said out loud. A line claude broke MID-WORD — an
// unbroken token wider than the terminal — comes back with a space the
// submitted text does not have, so the collapse does not rescue it and the
// echo is missed. The reading then keeps the text as a hold, which is a
// withheld settle and not an invented one, and it needs a ~76-character
// token in the line this seat most recently submitted. It is left unhandled
// rather than guessed at: un-wrapping by the terminal's width would need the
// width, and the width is not in the capture.
//
// EVERY FAILURE ANSWERS THE EMPTY COMPOSER: fewer than two rules on the
// screen, nothing between them, a body that does not open with the mark, a
// menu drawn over the box. All of them read as "posse saw no composer",
// which is judged exactly as it was before this bead — by the hold's
// callers. ranger-base-hlnyx is the one caller that needed them told
// apart, and it asks a different question of the same reading rather than
// changing this one: see captureComposerBox, and composerSight at the foot
// of this file.

import "strings"

// captureComposer returns the text sitting in the prompt box of a captured
// screen, or "" when the box is empty, no box is on screen, or what is
// between the rules is a menu drawn over the box rather than the box itself.
// It is captureComposerBox with the one question that reading cannot answer
// thrown away — see composerSight for the caller that needs it.
func captureComposer(screen string) string {
	body, _ := captureComposerBox(screen)
	return body
}

// captureComposerBox is captureComposer plus WHETHER THERE IS A BOX, which
// is the distinction every arm of this file used to flatten and
// ranger-base-hlnyx needed back: `("", false)` is "posse found no box on this
// screen" and `("", true)` is "the box is there and it is empty". The second
// is a seat waiting for a prompt; the first is a seat where the keyboard
// belongs to something else, and typing into it loses the prompt and
// actuates whatever that something has highlighted.
//
// THE CHROME CHECK READS THE WHOLE TAIL and not only the box body, which is
// the one place this is stricter than AgentDetection.Composer. herdr hands
// over a REGION, so its five `not` clauses (composerChromePhrases) are
// matched against exactly the bytes the box held. Here the box is a POSITION
// on a screen, and a dialog does not draw its hint inside it: MEASURED
// 2026-11-10 on claude's folder-trust prompt, the selection was
// `❯ No, exit` / `  Yes, I trust this folder` with its
// `Enter to confirm · Esc to cancel` two lines below, as its own line. That
// screen carried no horizontal rule in its tail at all, so this function
// answers "" for it today by finding no box — and whether a later dialog
// draws a rule between the selection and its hint was NOT measured, which is
// the whole reason the check does not depend on the answer. Reading from the
// second-to-last rule to the end of the screen costs nothing (no footer
// panework.go measured carries any of the five phrases; claude's is `esc to
// interrupt`) and cannot be fooled by which side of a rule the hint lands on.
func captureComposerBox(screen string) (string, bool) {
	lines := strings.Split(screen, "\n")
	topRule, botRule := -1, -1
	for i, l := range lines {
		if isFooterRule(l) {
			topRule, botRule = botRule, i
		}
	}
	if topRule < 0 || botRule <= topRule+1 {
		return "", false
	}
	lower := strings.ToLower(strings.Join(lines[topRule:], "\n"))
	for _, phrase := range composerChromePhrases {
		if strings.Contains(lower, phrase) {
			return "", false
		}
	}
	body := lines[topRule+1 : botRule]
	first := strings.TrimLeft(body[0], " \t")
	if !strings.HasPrefix(first, composerMark) {
		return "", false
	}
	// Each screen line is trimmed and they are rejoined with one space: the
	// left edge is the box's padding and the line ends are the renderer's
	// wrap, neither of which is in the text. U+00A0 — what claude draws
	// between the mark and the text — is whitespace to Go's TrimSpace, so it
	// comes off with the rest.
	parts := make([]string, 0, len(body))
	parts = append(parts, strings.TrimSpace(strings.TrimPrefix(first, composerMark)))
	for _, l := range body[1:] {
		parts = append(parts, strings.TrimSpace(l))
	}
	return strings.TrimSpace(strings.Join(parts, " ")), true
}

// capturedEcho is submittedEcho over a reading whose line breaks are the
// SCREEN's rather than the text's (the note above).
//
// ONE RULE, TWO ARTEFACTS. The comparison itself stays submittedEcho's —
// there is one answer to "is this the line that was submitted" and it is
// spelled once — and what is named here is the artefact being removed before
// it is asked. `truncated` is passed false because a capture is not a
// preview: nothing cut it down, so submittedEcho's prefix arm must stay shut
// or a short box would match every long submit that starts with it.
func capturedEcho(typed, sent string) bool {
	return submittedEcho(collapseScreenWrap(typed), collapseScreenWrap(sent), false)
}

// collapseScreenWrap folds every run of whitespace into one space. strings.
// Fields splits on Unicode whitespace, which takes the U+00A0 in a composer
// line as well as the renderer's newlines and padding.
func collapseScreenWrap(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// expandCursorForward rewrites `CSI n C` — cursor forward — as n spaces, so
// that a vt capture can be handed to the dim-run reading in ghostbox.go.
//
// WHY THIS ROUTE NEEDS IT AND HERDR'S DOES NOT. ghostbox.go's corpus was
// measured through `herdr agent read --format ansi`, where a composer line
// carries its spaces as spaces (60 dim suggestions, 2026-09-11). `rex
// capture --format vt` hands back what the terminal holds, and a line the
// renderer painted incrementally spells its gaps as cursor moves —
// MEASURED 2026-11-10: a wrapped prompt's first line came back as
// `this^[[1Cprompt^[[1Cis^[[1Csitting…` where the short form of the same
// prompt came back with literal spaces. scanSGRDim drops every escape it is
// not reading, so without this the line's words run together, the join
// against the plain reading fails, and a box that IS claude's own suggestion
// reads as a hold nobody can clear — ranger-base-6o7wm's defect, back on
// this occupant.
//
// IT IS THIS ROUTE'S NORMALIZATION AND NOT GHOSTBOX'S, deliberately: the dim
// rule there is a measured corpus, and widening its escape scan would change
// what that corpus was measured through. Moving the cursor right over cells
// that already hold text would make this wrong, and then the join fails and
// the answer is "not a ghost" — the safe direction, and the same one every
// other failure in this family takes.
func expandCursorForward(vt string) string {
	var b strings.Builder
	for i := 0; i < len(vt); {
		if vt[i] != 0x1b {
			b.WriteByte(vt[i])
			i++
			continue
		}
		n, cols, ok := scanCursorForward(vt[i:])
		if !ok {
			b.WriteByte(vt[i])
			i++
			continue
		}
		b.WriteString(strings.Repeat(" ", cols))
		i += n
	}
	return b.String()
}

// scanCursorForward measures a `CSI n C` at the head of s and says how many
// columns it moves. An omitted parameter is 1, which is what the sequence
// means with no parameter at all (ECMA-48 5th ed., CUF) and is the spelling
// every measured capture used. Anything else — another CSI final byte, a
// parameter that is not a number, a truncated sequence — is not this
// sequence, and the caller then passes the ESC through as an ordinary byte.
func scanCursorForward(s string) (n, cols int, ok bool) {
	if len(s) < 3 || s[0] != 0x1b || s[1] != '[' {
		return 0, 0, false
	}
	i, num := 2, 0
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		num = num*10 + int(s[i]-'0')
		if num > captureCursorForwardMax {
			return 0, 0, false
		}
	}
	if i >= len(s) || s[i] != 'C' {
		return 0, 0, false
	}
	if i == 2 {
		num = 1
	}
	return i + 1, num, true
}

// captureCursorForwardMax bounds what one cursor move may expand to. A
// terminal line is columns, not kilobytes, so a parameter past this is not
// describing a gap in a composer — and a reading that allocated whatever
// number a screen happened to carry would be a screen deciding how much
// memory posse spends.
const captureCursorForwardMax = 4096

// capturedComposerHold fills the three composer fields of a hold read off a
// capture: text sitting unsent, the line the box is echoing back, and the
// line claude drew faint.
//
// IT TAKES THE SCREEN IT WAS GIVEN, so Work and Typed are one instant and
// one snapshot — which is strictly better than the herdr route, where the
// region preview and the ansi read are two calls and the join can fail on a
// box that changed between them. The further calls are made only when there
// is text to ask about, which is the grain the herdr route already works at:
// an empty box costs one capture and nothing else.
//
// WHAT A NON-EMPTY BOX COSTS, so that the grain above is a number and not a
// reassurance. Three more occupant calls: `program_status` and `process` for
// the owner, and one ansi capture for the dim run. When this reading was
// written each of them was handed a PANE-ONLY seat — PaneHolding had a
// `target string` — and so paid RexMux.hold's resolution as well, 14 + 16N ms
// over N sessions by that file's own measurement, ~110 ms on this box, which
// put a settled holder whose box has text at roughly a third of a second of
// rex calls. ranger-base-k0hd6 then threaded the Seat through that whole
// path, so `target` here carries both halves and hold's guard returns
// immediately: the three calls are three calls now. What is left is one
// ~20 ms capture for an empty box, and three more for one with text. The
// callers ask per settled holder and not per pass, which is what made even
// the pane-only price affordable.
//
// THE STORE IS ASKED FIRST, for the reason panework.go gives: it is the
// authority for "was this submitted" and the screen is a derived copy of a
// fact it owns. Every failure on the way answers "not an echo" — a block no
// live runtime holds, a runtime that is not claude, a pid the fence rejects,
// a store that will not open, a session with no row in it — and the text
// then stays a hold, exactly as ranger-base-r5131 left it.
func (b *HerdrBackend) capturedComposerHold(target Seat, screen string, hold PaneHold) PaneHold {
	hold.Typed = captureComposer(screen)
	if hold.Typed == "" {
		return hold
	}
	hold.Read.Regions = append(hold.Read.Regions, ReadingRegion{
		Name:  captureComposerRegion,
		Bytes: len(hold.Typed),
		Text:  hold.Typed,
	})
	if sent, ok := b.paneLastSubmitted(target); ok && capturedEcho(hold.Typed, sent) {
		hold.Sent, hold.Typed = sent, ""
		return hold
	}
	// ranger-base-6o7wm's reading, through this occupant's own ansi capture.
	// Recorded whether or not it retired the claim, for ADR 0066 D1's reason
	// and because the half of that bead's measurement which is still missing
	// — that a line somebody TYPED is not drawn dim — is only ever closed by
	// a log that kept the boxes this declined to retire.
	if vt, err := b.Mux().Read(target, 0, true); err == nil {
		ghost, read := composerGhostRead(expandCursorForward(vt), hold.Typed)
		if read != "" {
			hold.Read.Regions = append(hold.Read.Regions, ReadingRegion{
				Name:  composerANSIRegion,
				Bytes: len(read),
				Text:  read,
			})
		}
		if ghost {
			hold.Ghost, hold.Typed = hold.Typed, ""
		}
	}
	return hold
}

// ─── is there a box to type into at all (ranger-base-hlnyx) ──────────────────

// composerSight is what posse can see of claude's prompt box on one captured
// screen. It is the reading RexMux.Prompt refuses on, and it is three-valued
// rather than two for the reason every reading in this family is: "posse
// cannot read this screen" and "posse read it and the box is not there" are
// different facts, and only the second is evidence.
type composerSight int

const (
	// composerUnreadable: nothing of claude's chrome is on the screen posse
	// was handed — no horizontal rule, none of the five menu phrases. A
	// block mid-boot, a screen rex answered with nothing, a runtime whose
	// screen this vocabulary was never written for. Ignorance, and ADR 0013
	// §1's unknown-is-not-no applies: it licenses nothing and refuses
	// nothing.
	composerUnreadable composerSight = iota
	// composerCovered: claude's chrome IS on the screen and the box is not
	// — something else is drawn where it goes. This is the modal.
	composerCovered
	// composerOpen: a box posse can type into, empty or holding text.
	composerOpen
)

// captureComposerSight judges one captured screen.
//
// WHY THE MIDDLE VALUE IS DECIDED BY ABSENCE AND NOT BY THE DIALOG'S OWN
// MARKS, which is the question ranger-base-hlnyx was filed with open. Both
// modal screens measured on this occupant DO carry one of
// composerChromePhrases in their last line — `Esc to cancel · Tab to amend`
// on the Bash permission dialog, `Enter to set as default · s to use this
// session only · Esc to cancel` on `/model` (MEASURED 2026-11-10, the bytes
// are in docs/notes.d/ranger-base-hlnyx.md) — so a presence rule would have
// caught both. It is still the wrong rule: it is exactly as good as the
// phrase list, and the phrase list is five strings measured off three of
// claude's dialogs in 2026. A dialog whose hint is spelled some sixth way
// would read as a box, and that failure is the one this reading exists to
// stop — the prompt is discarded and the Enter actuates the highlighted row.
// Absence has the other failure, a box posse cannot see on a screen that
// really is idle, and that one costs a refusal the operator can read.
//
// THE MARKS EARN THEIR KEEP ANYWAY, twice. They are half of what makes a
// screen READABLE below, which is how a `/model` picker that draws no
// horizontal rule at all is still judged rather than conceded; and the
// refusal quotes the screen's last line, so an operator is told which dialog
// is up rather than that something is.
func captureComposerSight(screen string) composerSight {
	if _, ok := captureComposerBox(screen); ok {
		return composerOpen
	}
	if captureShowsClaudeChrome(screen) {
		return composerCovered
	}
	return composerUnreadable
}

// captureShowsClaudeChrome is whether this screen carries anything posse
// recognizes as claude drawing its own frame: one of its horizontal rules,
// or one of the five menu phrases.
//
// NOT POSITIONAL, AND THAT IS SOUND HERE where it would not be anywhere else
// in this file. Every other reading over these phrases is anchored on a rule
// because a hold invented out of TRANSCRIPT PROSE can never go false
// (captureFooter's note). This one cannot invent anything: the only screen
// whose answer it decides is a screen captureComposerBox already found no box
// on, and on such a screen the two answers are "refuse" and "concede". A
// transcript quoting `Esc to cancel` moves the verdict from concede to
// refuse, which is the direction that costs a readable refusal and never a
// lost prompt. The rule arm is what does the work in practice — a claude
// screen with its composer drawn has two — and the phrase arm is for the
// dialog that replaces the frame entirely.
func captureShowsClaudeChrome(screen string) bool {
	lower := strings.ToLower(screen)
	for _, phrase := range composerChromePhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	for _, l := range strings.Split(screen, "\n") {
		if isFooterRule(l) {
			return true
		}
	}
	return false
}

// ─── the door the pulse's typed fallback reads (ADR 0093 D4) ─────────────────

// rexComposerHold is this file's reading on ONE seat, for a caller that must
// know whether the occupant answered at all: `false` is "not Rex, so this
// reading was never taken", and that is the whole of ADR 0093 D4's "on rex
// only" (ranger-base-r3qz6).
//
// WHY A DOOR AND NOT PaneHolding. PaneHolding asks the occupant and routes
// to the capture only where `agent explain` is missing (ADR 0076 §4), so on
// herdr it answers herdr's own composer reading — the phantom-prone one
// ranger-base-wr624 took out of the pulse's path after ~586 consecutive
// skips over a box `posse peek` showed empty. D4 is explicit that herdr
// stays report-only, so the caller needs the reading that has
// ranger-base-qzujz's discriminator behind it (the runtime's own submit log,
// asked first) and needs to be told when it is not available — which a hold
// that is empty for three different reasons cannot say.
//
// It is also narrower than capturedHold on purpose: no footer, so no work
// reading. The pulse's question is about the KEYBOARD, and whether the seat
// has a background shell running is both already answered upstream (the
// readiness gate's state rule) and none of this delivery's business.
//
// A capture rex would not answer is the EMPTY hold with `true` beside it:
// available, read nothing. That is this family's standing direction — a
// screen posse could not read is not evidence — and here it means the pulse
// types, which is the honest residual D4 names (the window between the
// capture and the Enter).
func (b *HerdrBackend) rexComposerHold(target Seat) (PaneHold, bool) {
	rm, ok := b.rexMux()
	if !ok {
		return PaneHold{}, false
	}
	screen, err := rm.Read(target, captureFooterLines, false)
	if err != nil {
		return PaneHold{}, true
	}
	return b.capturedComposerHold(target, screen, PaneHold{}), true
}

// captureLastLine is the last line of a screen with anything on it, trimmed.
// On claude's dialogs that line is the dialog's own hint — the one line that
// says which dialog it is — which is what the refusal quotes.
func captureLastLine(screen string) string {
	lines := strings.Split(screen, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	return ""
}
