package treepins

// QA pins for rangerhq-cmfj (verified under rangerhq-o0el).
//
// Claim: `bd init` plants an AGENTS.md whose "Landing the Plane" section
// orders the reader to push, dispatch hands that file to a persona as
// orientation, and every PID denies `Bash(git push:*)` — so the orientation
// orders the one thing the wall refuses, and orders it retried. Observed in
// the M1 cold rehearsal: a persona spent a turn pushing into the pre-push
// gate. §9 fixes it with a cut-and-append recipe; this repo's own AGENTS.md
// was reconciled the same way.
//
// Measured under rangerhq-o0el against the fleet's pinned bd 0.49.1:
//
//	bd init template   -> "# Agent Instructions … ```\n%s\n"  (rodata)
//	the %s it fills    -> "## Landing the Plane (Session Completion)…"
//	bd onboard         -> a "## Issue Tracking" pointer, no push mandate
//	bd prime, upstream -> "[ ] 6. git push  (push to remote)" + "NEVER skip this."
//	bd prime, none     -> "ephemeral branch (no upstream) … not pushed"
//
// Nothing pinned either half before this file: §9's recipe was executed once
// by its author and never again, and this repo's AGENTS.md could be
// regenerated back into the bug by a `bd init` in the checkout or a bd
// upgrade — which §9 itself warns about ("neither edit survives being
// regenerated"). The doc's `guardrails:` quote is pinned against the constant
// it quotes in internal/posse/pushmandate_qa_test.go.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// bdPlantedLanding is the section bd 0.49.1 substitutes into the AGENTS.md it
// plants, extracted byte-for-byte from the binary's rodata (offset of
// "## Landing the Plane" through the last CRITICAL RULE). It is the fixture
// the recipe has to survive, and the text the checkers below have to reject.
const bdPlantedLanding = "## Landing the Plane (Session Completion)\n" + `
**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until ` + "`git push`" + ` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ` + "```bash" + `
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ` + "```" + `
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until ` + "`git push`" + ` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds`

// bdPlantedAgentsMd is the whole file `bd init` writes: its own header, then
// the section above.
const bdPlantedAgentsMd = "# Agent Instructions\n" + `
This project uses **bd** (beads) for issue tracking. Run ` + "`bd onboard`" + ` to get started.

## Quick Reference

` + "```bash" + `
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
` + "```" + "\n" + bdPlantedLanding + "\n"

// readerDirectedPushOrders returns the mandate phrases an orientation file
// must not carry — each one tells the *reader* to push, which every PID
// denies. It is deliberately keyed on bd's own wording: those are the lines
// that come back if the file is ever regenerated.
func readerDirectedPushOrders(doc string) []string {
	var found []string
	for _, phrase := range []string{
		"PUSH TO REMOTE",
		"Work is NOT complete until `git push` succeeds",
		"NEVER stop before pushing",
		"YOU must push",
		"If push fails, resolve and retry until it succeeds",
		"All changes committed AND pushed",
	} {
		if strings.Contains(doc, phrase) {
			found = append(found, phrase)
		}
	}
	return found
}

// namesWhoPushes is the positive half of the reconciliation: silence about
// push is not an instruction, so the reconciled text has to say who does it.
// Keyed on the two claims rather than on one sentence's punctuation — the
// wording is shared with AGENTS.md and moves with it (ranger-base-wnsf),
// while "who pushes" is the thing that must never stop being said.
func namesWhoPushes(doc string) bool {
	return strings.Contains(doc, "Never push") && strings.Contains(doc, "operator pushes")
}

// pushMentionFaults is §9's own Verify, in code: every line of an orientation
// file that mentions push and does not say the operator pushes. Returned as
// `line:text` so a caller can name where.
//
// The match is on push as a WORD, not as a substring, and that distinction is
// load-bearing rather than tidy (ranger-base-rulbl). The substring form fired
// on `TestTestCorpusHidesNoCrewNameBehindAnEscape` — "cor-PUSH-ides" — a test
// name AGENTS.md has every reason to write down, and a fence that reds on
// correct documentation is one people learn to route around. Narrowed to a
// non-letter before the match, so `push`, `pushes`, `pushed`, `pushing`,
// `git push` and `PUSH TO REMOTE` all still land; only a push buried inside a
// longer word is let go.
//
// WORD is now "a word of PROSE", because the narrowing above left the same
// hole one step out (ranger-base-y0qvh). A letter precedes the match in
// `NoPushMandate` and in `CorpusHides`, so those were let by — but nothing
// precedes it in `pushmandate_qa_test.go`, `pushMentionFaults` or
// `internal/treepins/pushmandate_qa_test.go`, which are the spellings of
// every identifier in THIS file. The page that has to document this pin was
// the one page the pin forbade from naming it, and the workaround a seat
// reaches for under a red suite is to drop the filename — which costs the
// reader the pointer. So the match is also rejected when the TOKEN around it
// names code: it carries an underscore, a slash, an interior dot, or a
// camelCase hump. Those are the characters an English word cannot have, which
// is why the discriminator is the token's shape and not its position — and
// not a backticked span either, since `git push` in "Work is NOT complete
// until `git push` succeeds" is the mandate itself (the mutant arm below
// holds that reading dead). `pushes`, `pushed`, `pushing` and `PUSH` stay
// prose: they are letters all the way, and all-caps is not a hump.
//
// WHAT IT STILL CANNOT SEE, stated rather than discovered later: an order
// spelled ONLY as code — "run `scripts/push.sh` to finish" — is let by, because
// by shape that is the same token as `internal/treepins/pushmandate_qa_test.go`
// and nothing short of asking the filesystem tells them apart. Two things
// cover it. readerDirectedPushOrders matches bd's six exact phrases and is
// where the mandate bd actually plants is caught, whatever this scanner thinks;
// and §9's human `grep -n -i "push"` is wider than both on purpose. A reader
// who wants the narrowest honest claim for the line scanner: it catches an
// order written in English, and says nothing about one written as a command.
//
// INSTALL.md §9 still prescribes the human form, `grep -n -i "push"`, and
// that is deliberate: a person reading a grep hit can tell a test name from a
// mandate in one glance, and a wider net is the right one when a person is
// the filter. This is the automated half, where a false positive costs a red
// suite instead of a second of attention.
func pushMentionFaults(doc string) []string {
	return pushFaultsWith(mentionsPushAsAWord, doc)
}

// pushFaultsWith is the per-line half — the four exempting spellings and the
// line numbering — over a substitute mention predicate. The real scanner is
// the predicate below; the mutants in TestPushMandateScannerMutantsDieInTheirOwnGroup
// are spelled as the one decision each of them changes and share everything
// else with it, so a mutant reading can only be about that decision
// (ranger-base-y0qvh).
func pushFaultsWith(mentions func(string) bool, doc string) []string {
	var faults []string
	for i, line := range strings.Split(doc, "\n") {
		if !mentions(line) {
			continue
		}
		if strings.Contains(line, "Never push") || strings.Contains(line, "operator pushes") ||
			strings.Contains(line, "pre-push") || strings.Contains(line, "git push:*") {
			continue
		}
		faults = append(faults, fmt.Sprintf("%d:%s", i+1, line))
	}
	return faults
}

// mentionsPushAsAWord reports whether the line names push, case-insensitively,
// as a word of prose: at a position no letter precedes, and not inside a token
// that names code. The loop keeps looking after a rejected match, so a line
// that names a colliding identifier AND orders the reader still answers true
// (ranger-base-3nyqf for the first rule, ranger-base-y0qvh for the second).
//
// The fold is asciiLowered and not strings.ToLower, because both rules read
// the line by BYTE OFFSET at an index taken from the folded copy, and
// strings.ToLower does not preserve byte length: 27 runes change width under
// it, 25 narrower and 2 wider (MEASURED 2026-11-14 over all of Unicode,
// ranger-base-y0qvh; the pin is below). A narrower one slides the index left
// and the order escapes — `İpush it yourself` and `Kpush it yourself` (U+0130,
// U+212A Kelvin) both read as faultless under ToLower, because the byte the
// letter rule inspects is the `i`/`k` the fold produced rather than the line's
// own. A wider one slides it right and the token reader walks off the end:
// `Ⱥpush` (U+023A) panicked with `slice bounds out of range [:7] with length
// 6`, and a panic takes the whole package binary down with it, so every other
// test in the run is lost (ranger-base-ietka's class). The needle is ASCII, so
// an ASCII-only fold loses nothing: no non-ASCII rune lowercases to any of
// `p`, `u`, `s` or `h`.
func mentionsPushAsAWord(line string) bool {
	low := asciiLowered(line)
	for at := 0; ; {
		i := strings.Index(low[at:], "push")
		if i < 0 {
			return false
		}
		i += at
		if (i == 0 || !isASCIILetter(low[i-1])) && !pushTokenNamesCode(pushTokenAround(line, i)) {
			return true
		}
		at = i + len("push")
	}
}

// pushTokenAround returns the identifier-or-path token the match at i sits
// inside: the maximal run of characters a Go identifier or a repo path can be
// spelled with. Leading and trailing `.`, `/` and `_` are trimmed, because at
// a token's EDGE each of those is punctuation rather than part of a name, and
// it is the interior ones that say "code":
//
//   - `.` — without the trim, "The operator pushes." reads as code, the
//     sentence's own full stop being taken for a path's extension dot.
//   - `_` — without it, markdown emphasis does: "you must _push_ it yourself"
//     is an order, and `_push_` carries an underscore. Found attacking the
//     widening rather than writing it (ranger-base-y0qvh); both are group-1
//     cases now.
//
// An interior one survives the trim, so `pushmandate_qa_test.go` and
// `_internal/push_` are both still code.
func pushTokenAround(line string, i int) string {
	return strings.Trim(pushTokenSpanAround(line, i), "./_")
}

// pushTokenSpanAround is that span before the edges are trimmed. It is split
// out so the trim can have a mutant of its own: dropping it is a one-token
// edit, and what it reopens is an order reading as code.
func pushTokenSpanAround(line string, i int) string {
	lo, hi := i, i+len("push")
	for lo > 0 && isPushTokenByte(line[lo-1]) {
		lo--
	}
	for hi < len(line) && isPushTokenByte(line[hi]) {
		hi++
	}
	return line[lo:hi]
}

func isPushTokenByte(b byte) bool {
	return isASCIILetter(b) || b >= '0' && b <= '9' || b == '_' || b == '.' || b == '/'
}

// pushTokenNamesCode reports whether a token is a code identifier or a path
// rather than an English word: it carries a character English cannot (`_`,
// `/`, an interior `.`) or a camelCase hump. A hump is a lowercase letter
// followed by an uppercase one, NOT just any capital past position zero —
// `PUSH TO REMOTE` is the mandate's own shouting and has to stay prose.
func pushTokenNamesCode(token string) bool {
	if strings.ContainsAny(token, "_/.") {
		return true
	}
	for i := 1; i < len(token); i++ {
		if isASCIILower(token[i-1]) && isASCIIUpper(token[i]) {
			return true
		}
	}
	return false
}

// asciiLowered folds A-Z and nothing else, so the result is byte-for-byte the
// same length as its input and an index into it is an index into the original.
func asciiLowered(s string) string {
	b := []byte(s)
	for i, c := range b {
		if isASCIIUpper(c) {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func isASCIILetter(b byte) bool { return isASCIILower(b) || isASCIIUpper(b) }
func isASCIILower(b byte) bool  { return b >= 'a' && b <= 'z' }
func isASCIIUpper(b byte) bool  { return b >= 'A' && b <= 'Z' }

// TestPushMandateCheckerDiscriminates is the control: without it the pins
// below would pass on any text at all, the mandate included.
func TestPushMandateCheckerDiscriminates(t *testing.T) {
	if got := readerDirectedPushOrders(bdPlantedAgentsMd); len(got) < 5 {
		t.Errorf("checker does not recognize the file bd actually plants: found only %v", got)
	}
	reconciled := "- **Never push. The operator pushes.** Every persona's PID denies\n" +
		"  `Bash(git push:*)` and this repo's `pre-push` gate refuses it.\n"
	if got := readerDirectedPushOrders(reconciled); len(got) != 0 {
		t.Errorf("checker fires on the reconciled wording — it would be red forever: %v", got)
	}
	if namesWhoPushes(bdPlantedAgentsMd) {
		t.Error("namesWhoPushes accepts the file bd plants — it would pass on the bug")
	}
	if !namesWhoPushes(reconciled) {
		t.Error("namesWhoPushes rejects the reconciled wording — it would be red forever")
	}

	// The line scanner, three groups, each held by its own mutant in
	// TestPushMandateScannerMutantsDieInTheirOwnGroup below.
	for _, order := range pushOrdersThatMustFault {
		if got := pushMentionFaults(order); len(got) != 1 {
			t.Errorf("the line scanner misses a reader-directed push order — the narrowing went too far: %q", order)
		}
	}
	for _, fine := range pushLinesThatMustNotFault {
		if got := pushMentionFaults(fine); len(got) != 0 {
			t.Errorf("the line scanner reds a line that orders nobody to push: %q -> %v", fine, got)
		}
	}
	for _, both := range pushBothOnOneLine {
		if got := pushMentionFaults(both); len(got) != 1 {
			t.Errorf("the line scanner stops at the first push-shaped substring: a line that names a colliding identifier AND orders the reader must still fault: %q -> %v", both, got)
		}
	}
}

// The three groups, lifted out of the arm above so the mutation arm can say
// which group kills which wrong scanner (ranger-base-y0qvh). A widening of
// the scanner adds a case to each.

// pushOrdersThatMustFault is group 1: the reader-directed orders. A narrowing
// is only worth having if it still catches what it was narrowed from
// (ranger-base-rulbl).
var pushOrdersThatMustFault = []string{
	"- Work is NOT complete until `git push` succeeds",
	"- NEVER stop before pushing - that leaves work stranded locally",
	"you must push it yourself when the branch is ready",
	"6. **Verify** - All changes committed AND pushed",
	"PUSH TO REMOTE",
	// ranger-base-y0qvh's three, for the token reader: a sentence's full
	// stop is not a path's interior dot, markdown's emphasis underscore is
	// not an identifier's, and all-caps is not a camelCase hump. Each
	// confusion turns an order into code — the last one the mandate's own
	// shouting. The first two were found attacking the widening, which is
	// why they are orders nobody would otherwise think to write down.
	"3. Then push. That is the last step.",
	"you must _push_ it yourself when the branch is ready",
	"4. **PUSH TO REMOTE** - This is MANDATORY:",
}

// pushLinesThatMustNotFault is group 2: the four exempting spellings, the
// collision that made the narrowing necessary, and the code this file is
// written in.
var pushLinesThatMustNotFault = []string{
	"- **Never push.** The operator pushes.",
	"every crew PID denies `Bash(git push:*)`",
	"this repo's `pre-push` gate refuses it",
	// The collision that made the narrowing necessary: a real test
	// name, written down in a real table in AGENTS.md.
	"  TestTestCorpusHidesNoCrewNameBehindAnEscape  no fast door yet",
	// ranger-base-y0qvh: the four spellings of this pin's OWN code, which
	// are what a page documenting its door has to write down. Three of
	// these faulted before the widening; the fourth was let by only by
	// accident — the word sits mid-identifier in NoPushMandate, where a
	// letter precedes it. All four are now let by on purpose.
	"the four spellings are in `pushmandate_qa_test.go`'s own scanner",
	"`make orientation-check` runs internal/treepins/pushmandate_qa_test.go",
	"QA_ORIENTATION_PINS is the door; pushMentionFaults is the scanner",
	"see TestRepoAgentsMdCarriesNoPushMandate for the AGENTS.md half",
}

// pushBothOnOneLine is group 3: BOTH ON ONE LINE, the case the other two
// groups cannot see (ranger-base-3nyqf, found verifying ranger-base-rulbl
// under ranger-base-y0r3m). Every line in those groups holds exactly one
// "push", so the loop in mentionsPushAsAWord — the part that keeps looking
// after it rejects a match — was never exercised, and replacing the whole
// function with a single strings.Index passed all nine.
//
// A line naming a colliding identifier AND ordering the reader is not
// hypothetical: the AGENTS.md bullet ranger-base-rulbl added censuses
// tree-wide pins by name in a table, TestTestCorpusHidesNoCrewNameBehindAn
// Escape among them, and ranger-base-ik44f is the bead for giving those rows
// a door — which is prose written beside those names. Without this group, the
// seat who simplifies the scanner restores the blind spot it was narrowed to
// remove, and the suite says nothing.
var pushBothOnOneLine = []string{
	"  TestTestCorpusHidesNoCrewNameBehindAnEscape — run it before you push",
	"See TestTestCorpusHidesNoCrewNameBehindAnEscape, then push the branch yourself",
	// ranger-base-y0qvh's: here the match that must be SKIPPED is skipped
	// by the token rule and not the letter rule — a backtick precedes it,
	// not a letter — so this is the case that keeps the new rule per-MATCH
	// instead of per-line: a page that names this pin's file is free to do
	// so, and is still held to the mandate in the same sentence.
	"`pushmandate_qa_test.go` is the pin; push the branch yourself when it is green",
}

// mentionsPushLetterRuleOnly is the scanner as ranger-base-rulbl left it: a
// match no LETTER precedes is a mention, whatever the token around it looks
// like. It is the pre-widening behaviour, kept as a mutant so group 2 has to
// go on proving the widening is there — three of that group's four code lines
// fault under it, and the fourth (`NoPushMandate`) does not, which is the
// accident ranger-base-y0qvh found.
//
// Every mutant here takes the FIXED fold (asciiLowered), including this one,
// which rulbl's scanner did not have. A mutant is worth reading only when it
// differs from the real scanner in the one decision it is named for, and the
// fold is a separate defect with its own arm below.
func mentionsPushLetterRuleOnly(line string) bool {
	low := asciiLowered(line)
	for at := 0; ; {
		i := strings.Index(low[at:], "push")
		if i < 0 {
			return false
		}
		i += at
		if i == 0 || !isASCIILetter(low[i-1]) {
			return true
		}
		at = i + len("push")
	}
}

// mentionsPushBacktickExempt is the OTHER widening ranger-base-y0qvh's bead
// listed — "or it sits inside a backticked span" — and the reason the one
// that landed keys on the token's shape instead. The mandate's central line
// spells its order in backticks ("Work is NOT complete until `git push`
// succeeds"), so this reading lets the push mandate itself through. Group 1
// is what says so.
func mentionsPushBacktickExempt(line string) bool {
	low := asciiLowered(line)
	for at := 0; ; {
		i := strings.Index(low[at:], "push")
		if i < 0 {
			return false
		}
		i += at
		if (i == 0 || !isASCIILetter(low[i-1])) && strings.Count(line[:i], "`")%2 == 0 {
			return true
		}
		at = i + len("push")
	}
}

// mentionsPushUntrimmedToken is the token rule with the edge trim dropped —
// the token taken as the raw span. Group 1 is what says so, twice: a
// sentence's full stop and markdown's emphasis underscore both land at a
// token's edge, so "Then push." and "_push_" are code under this reading and
// the orders escape.
func mentionsPushUntrimmedToken(line string) bool {
	low := asciiLowered(line)
	for at := 0; ; {
		i := strings.Index(low[at:], "push")
		if i < 0 {
			return false
		}
		i += at
		if (i == 0 || !isASCIILetter(low[i-1])) && !pushTokenNamesCode(pushTokenSpanAround(line, i)) {
			return true
		}
		at = i + len("push")
	}
}

// mentionsPushWholeLineCodeExempt asks the token question of the LINE rather
// than of the match: if any push on the line names code, the line goes. It is
// the shortcut a seat writes when the four code spellings are all they have
// in front of them, and it passes every case in groups 1 and 2. Group 3 is
// the only reader it has.
func mentionsPushWholeLineCodeExempt(line string) bool {
	low := asciiLowered(line)
	prose, code := false, false
	for at := 0; ; {
		i := strings.Index(low[at:], "push")
		if i < 0 {
			break
		}
		i += at
		if i == 0 || !isASCIILetter(low[i-1]) {
			if pushTokenNamesCode(pushTokenAround(line, i)) {
				code = true
			} else {
				prose = true
			}
		}
		at = i + len("push")
	}
	return prose && !code
}

// mentionsPushFirstMatchOnly is ranger-base-3nyqf's mutant, carried forward:
// the loop replaced by a single strings.Index. It passed all nine arms that
// existed then, and the widening gives it a second way in — the first match
// on a line that names this file is now a REJECTED match, so a scanner that
// stops there sees nothing at all.
func mentionsPushFirstMatchOnly(line string) bool {
	low := asciiLowered(line)
	i := strings.Index(low, "push")
	if i < 0 {
		return false
	}
	return (i == 0 || !isASCIILetter(low[i-1])) && !pushTokenNamesCode(pushTokenAround(line, i))
}

// TestPushMandateScannerMutantsDieInTheirOwnGroup is the mutation check the
// control arm's three groups exist to be: each group is the named killer of a
// wrong scanner, so a group that stopped discriminating is reported as such
// instead of sitting green beside two that still do (ranger-base-y0qvh).
//
// ranger-base-3nyqf's finding was that nine arms in two groups passed a
// scanner with the loop replaced by a single strings.Index, and it was found
// by a verifier running that mutant by hand. Running it by hand is what this
// arm stops being necessary: the mutants are in the file, beside the groups
// that kill them, and a widening that adds a case to each group has to say
// which mutant the new case is for.
//
// Named `PushMandateScanner…`, as are the two arms below it, so that the
// filter this file's own notes prescribe — `go test ./internal/treepins
// -count=1 -run PushMandate`, which is what ranger-base-n7hsa measured at
// 0.089s — selects every pin in the file. A pin here whose name does not
// carry that substring is a pin nothing a seat types will reach, which is
// n7hsa's finding one level in.
func TestPushMandateScannerMutantsDieInTheirOwnGroup(t *testing.T) {
	for _, m := range []struct {
		name     string
		group    string
		lines    []string
		want     int
		mentions func(string) bool
	}{
		{
			name:  "the pre-widening scanner (a letter before the match, and nothing about the token)",
			group: "the lines that must not fault",
			lines: pushLinesThatMustNotFault, want: 0,
			mentions: mentionsPushLetterRuleOnly,
		},
		{
			name:  "a backticked-span exemption instead of a token-shape one",
			group: "the orders that must fault",
			lines: pushOrdersThatMustFault, want: 1,
			mentions: mentionsPushBacktickExempt,
		},
		{
			name:  "the token's edges left untrimmed",
			group: "the orders that must fault",
			lines: pushOrdersThatMustFault, want: 1,
			mentions: mentionsPushUntrimmedToken,
		},
		{
			name:  "the token question asked of the line instead of the match",
			group: "both on one line",
			lines: pushBothOnOneLine, want: 1,
			mentions: mentionsPushWholeLineCodeExempt,
		},
		{
			name:  "no loop: one strings.Index (ranger-base-3nyqf's mutant)",
			group: "both on one line",
			lines: pushBothOnOneLine, want: 1,
			mentions: mentionsPushFirstMatchOnly,
		},
	} {
		// The real scanner answers `want` on every line of the group —
		// otherwise a mutant reading below is an artifact of a group that
		// is already broken rather than of the mutant's own edit.
		for _, line := range m.lines {
			if got := pushMentionFaults(line); len(got) != m.want {
				t.Fatalf("%q: the real scanner answers %d faults, want %d — fix that before reading any mutant verdict: %v", line, len(got), m.want, got)
			}
		}
		var killers []string
		for _, line := range m.lines {
			if len(pushFaultsWith(m.mentions, line)) != m.want {
				killers = append(killers, line)
			}
		}
		if len(killers) == 0 {
			t.Errorf("no case in %q kills the mutant %q — that group has stopped being the reader of its own rule, and the rule can be edited out of mentionsPushAsAWord with every pin in this file green", m.group, m.name)
			continue
		}
		t.Logf("%s — killed by %d of %d cases in %q, first: %q", m.name, len(killers), len(m.lines), m.group, killers[0])
	}
}

// TestPushMandateScannerReadsTheFileBdPlants is the group-1 case that
// is not a hand-written line at all: the file `bd init` actually plants, read
// by the LINE scanner. The control arm has always read that fixture with
// readerDirectedPushOrders, which matches six exact phrases — so a widening
// of the line scanner could have let the real mandate through while every
// phrase arm stayed green, and the only reader of the line scanner was seven
// lines somebody typed (ranger-base-y0qvh).
//
// MEASURED 2026-11-14, this box, at posse main fcfcdbf6: eight of the planted
// file's lines fault, and all eight are reader-directed. Pinned as a floor
// rather than an equality — bd's template is upstream's to change, and a
// NINTH mandate line is not a reason to red this tree — but the floor is the
// count, so a widening that silences any one of the eight is.
func TestPushMandateScannerReadsTheFileBdPlants(t *testing.T) {
	got := pushMentionFaults(bdPlantedAgentsMd)
	if len(got) < 8 {
		t.Errorf("the line scanner finds %d faults in the file bd 0.49.1 plants, want at least 8 — a widening has silenced one of the mandate's own lines:\n%s", len(got), strings.Join(got, "\n"))
	}
	// The four exempting spellings must not be what is carrying it: the
	// planted file names none of them, so every fault above is the scanner's
	// own judgement and not an accident of the skip list.
	for _, spelling := range []string{"Never push", "operator pushes", "pre-push", "git push:*"} {
		if strings.Contains(bdPlantedAgentsMd, spelling) {
			t.Errorf("the planted fixture now carries the exempting spelling %q — the arm above is no longer reading what it claims to", spelling)
		}
	}
	t.Logf("pushMentionFaults(bdPlantedAgentsMd) = %d faults", len(got))
}

// TestPushMandateScannerFoldIsByteLengthPreserving holds the defect that attacking
// the widening turned up (ranger-base-y0qvh). Both of the scanner's rules
// read the line by BYTE OFFSET at an index taken from the folded copy, and
// `strings.ToLower` is not length-preserving, so the two disagree on where
// the match is:
//
//   - narrower (25 runes): the index slides left and the ORDER ESCAPES.
//     `İpush it yourself` (U+0130) and `Kpush it yourself` (U+212A Kelvin)
//     both read as faultless, because the byte the letter rule inspects is
//     the `i`/`k` the fold produced, not the line's own. That half is
//     ranger-base-rulbl's, live since the letter rule landed.
//   - wider (2 runes): the index slides right and the token reader walks off
//     the end. `Ⱥpush` (U+023A) panicked with `slice bounds out of range
//     [:7] with length 6`. That half is this bead's own, introduced by the
//     token rule and never shipped.
//
// A panic is the worse of the two: it takes the package BINARY down, so every
// other test in the run is lost with it and the reader meant to name one line
// of AGENTS.md names nothing (ranger-base-ietka's class).
func TestPushMandateScannerFoldIsByteLengthPreserving(t *testing.T) {
	var wider, narrower []rune
	for r := rune(0); r <= 0x10FFFF; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		s := string(r)
		if got := asciiLowered(s); len(got) != len(s) {
			t.Fatalf("asciiLowered is not byte-length-preserving at U+%04X: %d bytes -> %d — every byte offset the scanner takes from it is then wrong", r, len(s), len(got))
		}
		switch l := len(strings.ToLower(s)); {
		case l > len(s):
			wider = append(wider, r)
		case l < len(s):
			narrower = append(narrower, r)
		}
	}
	// If this ever reads zero, the hazard has gone away and the comment above
	// is wrong — which is worth a red, because the next seat will read it and
	// conclude the fold does not matter.
	if len(wider) == 0 || len(narrower) == 0 {
		t.Fatalf("strings.ToLower no longer changes byte width (%d wider, %d narrower) — this arm is pinning a hazard that no longer exists", len(wider), len(narrower))
	}
	t.Logf("strings.ToLower changes byte width on %d runes: %d narrower, %d wider", len(wider)+len(narrower), len(narrower), len(wider))

	// Every one of them, in front of an order, with the panic caught so the
	// failure names the rune instead of killing the binary.
	for _, r := range append(append([]rune{}, wider...), narrower...) {
		for _, line := range []string{string(r) + "push", string(r) + "push it yourself", "see " + string(r) + "push"} {
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						t.Errorf("the scanner panicked on U+%04X before an order (%q): %v", r, line, rec)
					}
				}()
				if got := pushMentionFaults(line); len(got) != 1 {
					t.Errorf("U+%04X before an order (%q) reads as %d faults, want 1 — a rune whose case fold changes width is not a letter and does not bury the word after it", r, line, len(got))
				}
			}()
		}
	}
}

// TestRepoAgentsMdCarriesNoPushMandate reads the orientation file dispatch
// actually hands this crew. A `bd init` in the checkout, a second
// `bd onboard`, or a bd upgrade can put the mandate back; this is what says so.
func TestRepoAgentsMdCarriesNoPushMandate(t *testing.T) {
	b, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	doc := string(b)
	if got := readerDirectedPushOrders(doc); len(got) > 0 {
		t.Errorf("AGENTS.md orders the reader to push, which every PID denies (rangerhq-cmfj): %v", got)
	}
	// Silence is not an instruction: the file has to say who does push.
	for _, want := range []string{"Never push", "operator pushes"} {
		if !strings.Contains(doc, want) {
			t.Errorf("AGENTS.md no longer names who pushes: missing %q (rangerhq-cmfj)", want)
		}
	}
	// §9's own Verify, run here: every surviving mention says the operator
	// pushes, or explains the wall — never orders the reader.
	for _, f := range pushMentionFaults(doc) {
		t.Errorf("AGENTS.md:%s — mentions push without saying the operator pushes", f)
	}
}

// installSection9 returns §9, or fails the test if the pin has stopped
// reading its subject. The section's own heredoc contains a `## ` line, so
// the bounds are the numbered headings, not the next `## `.
func installSection9(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatalf("read INSTALL.md: %v", err)
	}
	doc := string(b)
	i := strings.Index(doc, "## 9. The work repo and its queue")
	j := strings.Index(doc, "## 10. ")
	if i < 0 || j < 0 || j < i {
		t.Fatal("INSTALL.md: §9 not found — the pin has stopped reading its subject")
	}
	return doc[i:j]
}

func TestInstallSection9ReconcilesThePlantedAgentsMd(t *testing.T) {
	sec := installSection9(t)

	// The conflict, stated with the evidence that it is not theoretical.
	for _, want := range []string{
		"Work is NOT complete until `git\npush` succeeds",
		"NEVER stop before pushing",
		"Every reference PID in `examples/agents/` denies\n`Bash(git push:*)`",
		"refusals.log",
		"rangerhq-cmfj",
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("INSTALL.md §9 no longer states the AGENTS.md/PID conflict: missing %q (rangerhq-cmfj)", want)
		}
	}

	// The fix a cold installer copies, and the check that it worked.
	for _, want := range []string{
		"## Landing the plane",
		`grep -n -i "push" AGENTS.md`,
		"Every surviving mention must say the operator pushes",
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("INSTALL.md §9 no longer prescribes the reconciliation: missing %q (rangerhq-cmfj)", want)
		}
	}
	if !namesWhoPushes(sec) {
		t.Errorf("INSTALL.md §9's appended section no longer names who pushes (rangerhq-cmfj)")
	}

	// The second copy is not a file, so no repo edit reaches it.
	for _, want := range []string{"bd\nprime", "upstream-conditional", "after any `bd` upgrade"} {
		if !strings.Contains(sec, want) {
			t.Errorf("INSTALL.md §9 no longer names bd prime's copy of the mandate: missing %q (rangerhq-gmnm)", want)
		}
	}
}

// section9Recipe pulls the two shell blocks §9 prescribes back out of the
// doc — the awk that cuts and the heredoc that appends — as one snippet, so
// what runs below is the text a reader copies, not a paraphrase of it.
func section9Recipe(t *testing.T) string {
	t.Helper()
	sec := installSection9(t)
	lines := strings.Split(sec, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "$ awk ") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("INSTALL.md §9: the awk cut is gone — the pin has stopped reading its subject")
	}
	end := -1
	for i := start; i < len(lines); i++ {
		if lines[i] == "EOF" {
			end = i
			break
		}
	}
	if end < 0 {
		t.Fatal("INSTALL.md §9: the appended-section heredoc has no terminator")
	}
	var b strings.Builder
	for _, l := range lines[start : end+1] {
		b.WriteString(strings.TrimPrefix(l, "$ "))
		b.WriteByte('\n')
	}
	snippet := b.String()
	if !strings.Contains(snippet, "cat >> AGENTS.md <<'EOF'") {
		t.Fatalf("INSTALL.md §9: the cut is no longer followed by the append:\n%s", snippet)
	}
	return snippet
}

// runSection9Recipe runs that snippet verbatim over the given AGENTS.md and
// returns what the installer is left holding.
func runSection9Recipe(t *testing.T, agents string) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(agents), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cmd := exec.Command("sh", "-c", section9Recipe(t))
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("§9 recipe failed: %v\n%s", err, out)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	return string(b)
}

// TestInstallSection9RecipeCutsTheMandateOutOfWhatBdPlants is the pin that
// runs rather than reads: the recipe, verbatim, against the file bd 0.49.1
// actually writes.
func TestInstallSection9RecipeCutsTheMandateOutOfWhatBdPlants(t *testing.T) {
	got := runSection9Recipe(t, bdPlantedAgentsMd)

	if left := readerDirectedPushOrders(got); len(left) > 0 {
		t.Errorf("§9's recipe leaves the mandate in place: %v\n---\n%s", left, got)
	}
	if !namesWhoPushes(got) {
		t.Errorf("§9's recipe cut the mandate but named nobody in its place:\n%s", got)
	}
	// It cuts one section, not the file: everything above survives.
	for _, want := range []string{"# Agent Instructions", "## Quick Reference", "bd ready              # Find available work"} {
		if !strings.Contains(got, want) {
			t.Errorf("§9's recipe ate %q — it is meant to drop one section:\n%s", want, got)
		}
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Errorf("§9's recipe leaves a doubled blank at EOF:\n%q", got[len(got)-40:])
	}
}

// TestInstallSection9RecipeStopsAtTheNextHeading is the case §9 actually
// creates: `bd onboard`'s region is appended *before* the reconciliation, so
// the mandate is no longer the last section. A cut that runs to EOF would
// take the queue instructions with it.
func TestInstallSection9RecipeStopsAtTheNextHeading(t *testing.T) {
	const onboard = "## Issue Tracking\n\nThis project uses **bd (beads)** for issue tracking.\n" +
		"\n**Quick reference:**\n- `bd ready` - Find unblocked work\n\nFor full workflow details: `bd prime`\n"
	got := runSection9Recipe(t, bdPlantedAgentsMd+onboard)

	if left := readerDirectedPushOrders(got); len(left) > 0 {
		t.Errorf("§9's recipe leaves the mandate in place when a section follows it: %v", left)
	}
	for _, want := range []string{"## Issue Tracking", "- `bd ready` - Find unblocked work", "For full workflow details: `bd prime`"} {
		if !strings.Contains(got, want) {
			t.Errorf("§9's recipe cut past the next `##` and took %q with it:\n%s", want, got)
		}
	}
}

// TestInstallSection9RecipeIsSafeWithNothingToCut covers §9's own escape
// hatch ("If grep found nothing, skip it and just append") — an older bd, or
// a second run of the recipe, must not corrupt the file.
func TestInstallSection9RecipeIsSafeWithNothingToCut(t *testing.T) {
	const plain = "# Agent Instructions\n\n## Quick Reference\n\n- `bd ready`\n\n## House Rules\n\nKeep it short.\n"
	got := runSection9Recipe(t, plain)

	for _, want := range []string{"## Quick Reference", "- `bd ready`", "## House Rules", "Keep it short."} {
		if !strings.Contains(got, want) {
			t.Errorf("§9's recipe damaged a file with no Landing section: %q missing\n%s", want, got)
		}
	}
	if !namesWhoPushes(got) {
		t.Errorf("§9's recipe appended nothing when there was nothing to cut:\n%s", got)
	}
}

// TestInstallSection9RecipeReplacesRatherThanDuplicatesOnASecondRun is the
// shape TestInstallSection9RecipeIsSafeWithNothingToCut's comment already
// claimed to cover ("a second run of the recipe") but never measured: that
// test's fixture has no Landing section on its first pass, so its "twice"
// check ran the recipe over a file whose heading the recipe itself had just
// written — the exact case ranger-base-23va7 found broken. The cut
// (`/^## Landing the Plane/`, no fold) never matched the heading the
// heredoc appends (`## Landing the plane`), so a second pass — over what
// bd init plants and gets reconciled once, or over this repo's own
// AGENTS.md — cut nothing and appended a second copy instead of replacing
// the first. Fixed by folding case in the awk match.
func TestInstallSection9RecipeReplacesRatherThanDuplicatesOnASecondRun(t *testing.T) {
	once := runSection9Recipe(t, bdPlantedAgentsMd)
	if n := strings.Count(once, "## Landing the plane"); n != 1 {
		t.Fatalf("first pass over what bd init plants left %d Landing sections, want 1:\n%s", n, once)
	}

	twice := runSection9Recipe(t, once)
	if n := strings.Count(twice, "## Landing the plane"); n != 1 {
		t.Errorf("second pass over an already-reconciled AGENTS.md left %d Landing sections, want 1 (it must replace, not duplicate):\n%s", n, twice)
	}
	if left := readerDirectedPushOrders(twice); len(left) > 0 {
		t.Errorf("second pass reinstated the push mandate: %v", left)
	}
	if !namesWhoPushes(twice) {
		t.Errorf("second pass lost who pushes:\n%s", twice)
	}

	b, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	own := runSection9Recipe(t, string(b))
	if n := strings.Count(own, "## Landing the plane"); n != 1 {
		t.Errorf("running §9's recipe over this repo's own, already-reconciled AGENTS.md left %d Landing sections, want 1:\n%s", n, own)
	}
	if left := readerDirectedPushOrders(own); len(left) > 0 {
		t.Errorf("running §9's recipe over this repo's own AGENTS.md reinstated the push mandate: %v", left)
	}
}
