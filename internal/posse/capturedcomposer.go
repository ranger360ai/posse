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
// which is judged exactly as it was before this bead.

import "strings"

// captureComposer returns the text sitting in the prompt box of a captured
// screen, or "" when the box is empty, no box is on screen, or what is
// between the rules is a menu drawn over the box rather than the box itself.
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
func captureComposer(screen string) string {
	lines := strings.Split(screen, "\n")
	topRule, botRule := -1, -1
	for i, l := range lines {
		if isFooterRule(l) {
			topRule, botRule = botRule, i
		}
	}
	if topRule < 0 || botRule <= topRule+1 {
		return ""
	}
	lower := strings.ToLower(strings.Join(lines[topRule:], "\n"))
	for _, phrase := range composerChromePhrases {
		if strings.Contains(lower, phrase) {
			return ""
		}
	}
	body := lines[topRule+1 : botRule]
	first := strings.TrimLeft(body[0], " \t")
	if !strings.HasPrefix(first, composerMark) {
		return ""
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
	return strings.TrimSpace(strings.Join(parts, " "))
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
	if sess, err := b.paneAgentSession(target); err == nil {
		if sent, ok := lastSubmitted(b.ClaudeHistory, sess); ok && capturedEcho(hold.Typed, sent) {
			hold.Sent, hold.Typed = sent, ""
			return hold
		}
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
