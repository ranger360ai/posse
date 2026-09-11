package posse

// Whether a composer is holding a prompt, or drawing claude's own next-prompt
// SUGGESTION — ranger-base-6o7wm, from ranger-base-l51p1.
//
// THE READING THIS NARROWS. panework.go reads claude's `prompt_box_body`
// region and calls any text there "a prompt sitting UNSENT in its box".
// ranger-base-2hvtv retired one way that is wrong — a box echoing the line
// the pane last SUBMITTED — by asking claude's own submit log (sentline.go).
// This retires the other one, and it is the common case rather than the edge:
// claude writes a suggested next prompt into an EMPTY box, and that text is
// in no store at all, because claude logs submits and never suggestions. So
// the echo reading answers "not an echo" for every suggestion — correctly,
// and uselessly — and a settled seat with nothing running is filed
// `settled-unsent:` on every tick, by a row nobody can make go false: nobody
// typed the text, so nobody can clear it (the ranger-base-wr624 property).
//
// WHAT DOES ANSWER IT, and it is not a store this time but the same screen
// read differently. herdr's region preview is ANSI-STRIPPED — one plain-text
// region, and the detection manifest has no typed-vs-suggested rule, field or
// region anywhere in it (herdr 0.8.2, claude manifest 2026.09.04.1) — but the
// escapes are on the wire one call away, and claude draws its suggestion
// FAINT:
//
//	herdr agent explain <pane> --json  prompt_box_body preview:  "❯ <line>"
//	herdr agent read <pane> --format text:                       "❯ <line>"
//	herdr agent read <pane> --format ansi:   "❯ \e[0m\e[2m<line>\e[0m"
//
// SGR 2 is dim. So the discriminator posse does not have in `explain` is in
// the read beside it, and this file is the ansi read plus the join that says
// the two are looking at the same box.
//
// MEASURED 2026-09-11T07:22Z-08:19Z, 77 reads of every idle claude pane on
// one box, 20s apart (herdr 0.8.2, claude manifest 2026.09.04.1, posse
// 0.4.0+d8f1a644). The corpus is on ranger-base-6o7wm; its three shapes:
//
//	60  suggestion, dim   "❯ \e[0m\e[2m<line>\e[0m"   (two distinct lines)
//	17  empty box         "❯"                          (no SGR run at all)
//	 0  a TYPED unsent line
//
// The empty-box rows are what rule out the two cheap ways this could be an
// artefact rather than a discriminator: the same pane, same read, same minute
// previews a box with no SGR-2 anywhere in it, so the dim run is neither the
// pane's own theme nor herdr's rendering.
//
// THE HALF THAT IS STILL MISSING, and every caller below is placed around it.
// That a TYPED unsent line is NOT dim in the same read is unmeasured. It
// could not be staged: typing into a live pane's composer is typing into
// somebody's seat, posse's own sends submit, and the state is rare enough
// (ranger-base-wr624's three episodes are hours apart over one day) that an
// hour of watching the whole fleet turned up none. `scripts/verify-ghost-
// composer.sh` stages exactly that arm in a scratch herdr against a scratch
// claude — arm A types a marker and never submits it — and it cannot be run
// from a caged seat, because claude reads its OAuth token by shelling out to
// the keychain CLI and ADR 0042's L1 shims deny that (three arms measured
// 2026-09-05, claude 2.1.261, all three stop at the sign-in screen). Run from
// an UNCAGED shell it answers this in one turn.
//
// So the reading is allowed to RETIRE a claim and never to license a
// keystroke. If a typed line turned out to be dim too, every caller wired to
// it below loses a report it used to make — a `settled-unsent:` row keeps its
// plain `settled:` shape, a settle is judged, a `posse prompt` warning goes
// quiet — and none of them types over the text. The one caller that types
// (dispatch's --resume re-prompt) keeps refusing on a dim box exactly as it
// refuses today, and says in its own line what would retire that.
//
// IT GOES FALSE ON ITS OWN, which is the property the plain-text matcher
// lacked and the reason this is worth a second herdr call: the moment
// somebody types, the box is not wholly dim any more and the row comes back
// with nothing cleared by hand.
//
// EVERY FAILURE ANSWERS "NOT A GHOST", the same direction as every other
// reading in panework.go's family: a herdr that will not read the pane, a
// read with no composer in it, a screen posse cannot join to the preview it
// already has. Ignorance is not evidence that claude wrote the text.

import (
	"strings"
	"unicode/utf8"
)

// composerIsGhost reports whether the composer previewing `plain` is drawing
// the whole of it DIM — text claude wrote into an empty box rather than text
// somebody typed.
//
// `ansi` is a whole-screen `agent read --format ansi`; `plain` is what
// AgentDetection.Composer already read out of `explain`'s region preview, and
// passing it is the JOIN: the two calls are two instants and two snapshots,
// so this answers only when the last composer line in the ansi read carries
// character for character the text the preview carried. A box that changed
// between the calls, a preview herdr truncated (243 characters, panework.go),
// a multi-line prompt, a `❯` that turns out to be somewhere else on the
// screen: all of them fail the join and answer false.
func composerIsGhost(ansi, plain string) bool {
	if plain == "" {
		return false
	}
	line, ok := lastComposerLine(ansi)
	if !ok {
		return false
	}
	text, dim, any := scanSGRDim(line)
	if !any || !dim {
		return false
	}
	return strings.TrimSpace(text) == plain
}

// lastComposerLine returns everything after the prompt mark on the LAST line
// of the read that carries one — the live composer, whatever scrollback is
// above it. The mark is required for the same reason Composer requires it:
// without it there is no composer on screen to have an opinion about.
func lastComposerLine(ansi string) (string, bool) {
	for _, line := range reverseLines(ansi) {
		if i := strings.Index(line, composerMark); i >= 0 {
			return line[i+len(composerMark):], true
		}
	}
	return "", false
}

// reverseLines splits on newlines and hands them back bottom-first, with the
// carriage returns a terminal read carries taken off.
func reverseLines(s string) []string {
	all := strings.Split(s, "\n")
	out := make([]string, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		out = append(out, strings.TrimRight(all[i], "\r"))
	}
	return out
}

// scanSGRDim walks one line of terminal output and answers three things at
// once, because they are one pass: the text with the escapes taken out,
// whether EVERY printing character in it was drawn under SGR 2, and whether
// there was a printing character at all.
//
// `allDim` is the measured property and not the measured BYTES: the corpus
// shape is `\e[0m\e[2m…\e[0m`, but a herdr or a claude that spells the same
// intent with `\e[0;2m`, or breaks the run in two, is still drawing the line
// faint and must still read as one. Whitespace is not evidence either way —
// the U+00A0 claude puts between the mark and the text sits outside the run
// in every capture — so it is skipped rather than counted as undimmed.
//
// An `any` of false is a box with nothing printing in it, which the caller
// must not read as "wholly dim": an empty box vacuously satisfies "every
// character is dim", and that is the one answer this must never give.
//
// Today that guard is NOT independently reachable, and it is kept anyway.
// Mutating it away survives the whole suite (ranger-base-6o7wm, measured
// 2026-09-11), because composerIsGhost's join gets there first: a `plain` it
// could disagree with is non-empty and already trimmed, so matching it
// requires a printing character. The join is the falsifiable guard and `any`
// is the one that still holds if the join is ever loosened — said out loud
// rather than pinned, because a pin that cannot go red is worse than none.
func scanSGRDim(line string) (text string, allDim, any bool) {
	var b strings.Builder
	dim := false
	allDim = true
	for i := 0; i < len(line); {
		if line[i] == 0x1b {
			n, isSGR, params := scanEscape(line[i:])
			if n > 0 {
				if isSGR {
					dim = applySGR(dim, params)
				}
				i += n
				continue
			}
		}
		r, sz := utf8.DecodeRuneInString(line[i:])
		b.WriteString(line[i : i+sz])
		i += sz
		if isBoxSpace(r) {
			continue
		}
		any = true
		if !dim {
			allDim = false
		}
	}
	return b.String(), allDim, any
}

// scanEscape measures one escape sequence at the head of s and says whether
// it was an SGR (`CSI … m`), handing back its parameter bytes. Every other
// sequence — cursor moves, erases, OSC chrome — is measured only so that it
// comes out of the text rather than being counted as printing characters.
// n == 0 is "this is not a sequence this reads", and the ESC is then treated
// as an ordinary byte, which is the safe direction: a stray ESC makes the
// join fail rather than making a line look dim.
func scanEscape(s string) (n int, isSGR bool, params string) {
	if len(s) < 2 || s[0] != 0x1b {
		return 0, false, ""
	}
	switch s[1] {
	case '[': // CSI: parameter and intermediate bytes, then one final byte
		i := 2
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
			i++
		}
		if i >= len(s) || s[i] < 0x40 || s[i] > 0x7e {
			return 0, false, ""
		}
		return i + 1, s[i] == 'm', s[2:i]
	case ']': // OSC: runs to BEL or to ST (ESC \)
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1, false, ""
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2, false, ""
			}
		}
		return 0, false, ""
	}
	return 0, false, ""
}

// applySGR folds one SGR sequence's parameters into the dim state, in order,
// the way a terminal does. Only the three that decide it are read: 0 and an
// empty parameter list are a full reset, 2 turns faint on, 22 turns it off
// (ECMA-48 5th ed., SGR). Everything else leaves it alone.
//
// The extended-colour forms are the reason this walks parameters with an
// index rather than ranging over them. `38;2;<r>;<g>;<b>` selects a direct
// RGB foreground, and its 2 is a COLOUR SPACE id, not faint — a
// `\e[38;2;128;128;128m` grey would otherwise read as dim and call an
// ordinary coloured line claude's own suggestion. 38, 48 and 58 therefore
// consume their arguments here: `;5;n` one, `;2;r;g;b` three.
func applySGR(dim bool, params string) bool {
	ps := strings.Split(params, ";")
	if len(ps) == 1 && strings.TrimSpace(ps[0]) == "" {
		return false
	}
	for i := 0; i < len(ps); i++ {
		switch strings.TrimSpace(ps[i]) {
		case "", "0":
			dim = false
		case "2":
			dim = true
		case "22":
			dim = false
		case "38", "48", "58":
			if i+1 < len(ps) {
				switch strings.TrimSpace(ps[i+1]) {
				case "5":
					i += 2
				case "2":
					i += 4
				default:
					i++
				}
			}
		}
	}
	return dim
}

// isBoxSpace is the whitespace a composer line pads with: ASCII blanks and
// the U+00A0 claude draws between the mark and the text.
func isBoxSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n', 0x00a0:
		return true
	}
	return false
}
