package posse

// ADR 0024 D2 check 4 (ranger-base-ihp7y, from the operator ruling on
// ranger-base-lcuec): AGENTS.md "Landing the plane" says a residual still
// open under a `vuln-private` bead is named by bead id and nothing else —
// no description, path, mechanism or reproduction — until that residual's
// own fix lands and the bead closes. That was prose with no wall. This file
// is the wall's judge: a filter the commit-guard hook pipes its already-
// computed scan text through (the staged markdown additions, or the
// cleaned commit message — gates.go's existing $posse_added, the same text
// checks 0-3 already read) and that answers which of this repo's currently
// OPEN `vuln-private` beads is named beside more than its own id.
//
// WHY THIS IS A posse SUBCOMMAND AND NOT A SHELL ARM (the bead's own open
// question: "the hook reaching the bd store at all, and what it does when
// it cannot"). Every other arm in this hook is pure POSIX sh for a reason —
// gates.go's own comments catalogue the class of defect that shell-side
// JSON/diff parsing has produced here before (ranger-base-qg0k8's quoting,
// ranger-base-x9xbk's rename blind spot) — but none of those arms needs a
// LIVE answer from an external process with no bound on how long it takes.
// bd's own timeout is a Go-level mechanism (Bd.Timeout, BdTimeout,
// beads.go) built on exec.CommandContext; POSIX sh has no portable
// equivalent that does not mean backgrounding a process and killing it from
// a sibling — the shape AGENTS.md's "Leaked gate-shell children" paragraph
// already warns is a recurring defect on this box, and inventing a new one
// inside every commit on every repo this hook is installed in is a worse
// trade than one more subprocess. Going through the `posse` binary gets
// Bd.Available()'s PATH-shim-aware probe and bd.OpenLabeledAny's own
// --limit 0, both-store-class-correct open filter (beads.go) for free,
// and keeps the judgment in a language with real string handling and a
// unit test, instead of a second implementation in sed that can disagree
// with the first.
//
// FAILS OPEN (ADR 0068's lockout lesson, which the bead cited by name): no
// bd on PATH, bd unreachable, bd erroring for any reason, or no
// vuln-private bead open at all — every one of these returns no hit and
// leaves the commit to every other arm. A wall that refused on "I could
// not find out" rather than "this is wrong" would be exactly ADR 0068's
// class: a refusal with no remedy the committer can perform, since
// rewriting the message or the staged file cannot fix a store the hook
// could not reach.

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
)

// VulnPrivateLabel is the bd label AGENTS.md "Landing the plane" names
// (operator ruling 2026-11-11, ranger-base-lcuec): a bead carrying it is a
// security residual whose DETAIL belongs in the private store alone.
const VulnPrivateLabel = "vuln-private"

// VulnPrivateRule is what a check-4 refusal names.
const VulnPrivateRule = "AGENTS.md \"Landing the plane\" (operator ruling 2026-11-11, ranger-base-lcuec), " +
	"read with ADR 0024 D1: a residual still open under a `vuln-private` bead is named " +
	"by bead id and nothing else — no description, path, mechanism or reproduction — " +
	"until that residual's own fix lands and the bead closes."

// VulnPrivateWayThrough is the content/path-arm remedy: the staged file is
// still on disk and still the writer's own to edit, unlike the message arm
// below.
//
// IT NAMES THE RULE AND NOT A LIST OF BYTES (ranger-base-68631 finding 6).
// The sentence used to promise "a list bullet, backticks, bold and
// surrounding punctuation", over a whitelist of eleven punctuation bytes
// that admitted none of `[id]`, `"id"`, `'id'`, `_id_`, `<id>`, `> id`,
// `| id |`, a bullet glyph, an em dash or a trailing `/` — ten shapes a
// writer types and the remedy said were fine. A remedy that is wrong about
// what it allows is ADR 0068's class one step in: the way through does not
// go through. vulnPrivateBare is the rule now — no LETTER beside the id, in
// any script — so this sentence can be true without enumerating anything.
const VulnPrivateWayThrough = "the way through: write the bare id alone — whitespace, a list bullet or " +
	"numbering, backticks, bold markers, brackets, quotes and punctuation are all " +
	"fine; a LETTER is not, because a word beside the id is the description the " +
	"rule refuses. The detail belongs on the bead, where the audience is already " +
	"the right one."

// VulnPrivateMessageWayThrough mirrors IdentityMessageWayThrough's shape
// (visibility.go): the text is not in a file the writer can edit, it is
// .git/COMMIT_EDITMSG, local and unreplicated until the next commit
// overwrites it.
const VulnPrivateMessageWayThrough = "the way through: rewrite the commit message so the open `vuln-private` id " +
	"stands alone — punctuation and decoration are fine, a LETTER beside the id " +
	"is not, so a description, path, mechanism or reproduction is out. " +
	"Nothing was committed: HEAD is unchanged and your tree " +
	"and index are exactly where you left them. The message you typed is still " +
	"in .git/COMMIT_EDITMSG, a local file that never replicates and that your " +
	"next commit overwrites."

// vulnPrivateBare reports whether what is left of a unit after every open
// vuln-private id in it is removed still reads as "named by bead id and
// nothing else": NO LETTER, in any script.
//
// THE RULE IS THE COMPLEMENT OF A WHITELIST, and that is the whole of
// ranger-base-68631 findings 5 and 6. This used to be
// `strings.Trim(rest, " \t.,:;()*+#`-0123456789") == ""`, and the comment on
// that set already said the operative half out loud — "A LETTER IS NEVER IN
// THIS SET" — while the set itself had to guess every byte that is not one.
// It guessed short twice, in opposite directions:
//
//   - `\r` was missing, so a BARE id on a CRLF line was refused with a
//     remedy telling the writer to do the thing they had just done
//     (finding 5). Base rate zero in this tree — `git grep -lI $'\r'` finds
//     no tracked file with a CR and there is no .gitattributes eol policy —
//     but the MESSAGE arm reads text git never normalizes.
//   - ten ordinary quoting and decoration shapes were missing, all of them
//     promised by the remedy sentence (finding 6).
//
// A letter test cannot guess short: descriptive text needs words, a word
// needs a letter, and unicode.IsLetter says so for every script rather than
// for ASCII alone — so a Cyrillic or Han description is refused by the same
// line that refuses an English one, which no byte whitelist of this shape
// was ever going to do. Digits stay allowed, as they were (`1.` numbering,
// and a date beside an id discloses nothing a bd id does not).
func vulnPrivateBare(rest string) bool {
	for _, r := range rest {
		if unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// vulnPrivateContent discounts ONE leading unified-diff `+` from a line.
//
// The content arm's text is `git diff --cached -U0 … | grep -a '^+'`
// (gates.go's $posse_added), so every line it scans carries one and the
// indentation this file measures sits AFTER it — without this, `+- <id>`
// and `+  <description>` both read as column 0 and the continuation rule
// below would be dead in the one arm that reads a file. The message arm's
// lines carry no `+`, and a message line that genuinely begins with one
// loses a column of indent, which can only make a unit narrower.
func vulnPrivateContent(line string) string {
	return strings.TrimPrefix(line, "+")
}

// vulnPrivateIndent is the column a line's content starts at.
func vulnPrivateIndent(line string) int {
	s := vulnPrivateContent(line)
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') {
		n++
	}
	return n
}

// vulnPrivateListItem reports whether a line opens a markdown LIST item — a
// bullet or a numbered item — as opposed to a heading or ordinary prose.
// A list marker needs whitespace after it, which is both markdown's own rule
// and what keeps `**bold**` on an indented line from reading as a bullet and
// buying itself a unit.
//
// It is separate from vulnPrivateOpensUnit because a list item is the only
// block here that takes a LAZY CONTINUATION — see vulnPrivateUnits.
func vulnPrivateListItem(line string) bool {
	s := strings.TrimLeft(vulnPrivateContent(line), " \t")
	if s == "" {
		return false
	}
	switch s[0] {
	case '-', '*', '+':
		return len(s) == 1 || s[1] == ' ' || s[1] == '\t'
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')')
}

// vulnPrivateOpensUnit reports whether a line begins a markdown block of its
// own — a list item or a heading — rather than continuing the one above it.
func vulnPrivateOpensUnit(line string) bool {
	if vulnPrivateListItem(line) {
		return true
	}
	s := strings.TrimLeft(vulnPrivateContent(line), " \t")
	return s != "" && s[0] == '#'
}

// vulnPrivateUnits groups lines into the blocks the bareness test judges:
// each line's index maps to the index of the line that OPENS its block, so a
// bullet and the lines that continue it are one unit.
//
// WHY A UNIT AND NOT A LINE (ranger-base-68631 finding 1). The check was
// strictly per-line, and the shape AGENTS.md's own ruling prescribes is a
// "Not closed here" SECTION — so a description on the line BELOW a bare id
// passed clean, MEASURED end-to-end through the shipped subcommand:
//
//   - <open id> <text>      flagged, rc 1
//   - <open id>\n  <text>   SILENT, rc 0
//
// The second is how anyone writes a bullet with detail under it, and it is
// the whole of what this file exists to refuse. A unit is a bullet plus its
// continuation, in BOTH directions — a bare id indented under a line that
// carries the text is the same escape read upwards — and the grouping is one
// pass with a stack, so the cost is linear in the text however deep it nests.
//
// AND THE CONTINUATION IS LAZY, NOT INDENTED (ranger-base-ckcly finding 1,
// escaped from the fix above). The pop loop dropped every opener at or past
// the current line's indent, so text at COLUMN 0 under a top-level bullet
// bought a unit of its own and passed SILENT — one space fewer than the
// indented shape this grouping was written to catch:
//
//   - <open id>\n  <text>   flagged, rc 1
//   - <open id>\n<text>     SILENT, rc 0, before this
//
// Markdown reads the second as a lazy continuation of the list item, so a
// line that does not open a block of its own never pops a LIST item off the
// stack, whatever its indent. It is the list item alone, and not every
// opener: a heading wrapped onto a second line is NOT a lazy continuation —
// its second line opens a new paragraph — and charging one to the heading
// above it reds a bare id written under a wrapped `### Finding N: …`, which
// this tree already contains (docs/notes.d/ranger-base-nr3qf.md, MEASURED:
// 2 ids newly flagged over 674 tracked markdown files by the wider rule,
// 0 by this one).
//
// THREE RESIDUALS ARE DELIBERATE, and the first is the PARAGRAPH:
//
//	## Not closed here\n\n- <open id>\n\n<text>   SILENT, rc 0
//
// A blank line closes every open unit, so a description in its own paragraph
// below the section is not charged to the ids above it. Widening to the
// paragraph or to the SECTION was weighed and refused: ranger-base-lcuec.md's
// own "Not closed here" names ordinary beads — not vuln-private, and so not
// this check's business at all (TestVulnPrivateHitsIgnoresUnknownIDs) — in
// full sentences, in the same section as the bare ids. Section scope reds
// that document, which is a refusal with no remedy the writer can perform
// over text the rule does not bind: ADR 0068's class, bought to close a
// shape nobody has typed. The continuation rule has no such cost, so it
// lands and the paragraph stays open, stated here and pinned as deliberate
// by TestVulnPrivateHitsLeavesTheSeparateParagraphOpen.
//
// The second is a description written as a block of its OWN under the bare
// id — a nested bullet, a numbered item, a heading:
//
//   - <open id>\n  - <text>   SILENT, rc 0
//
// That one is bought rather than merely left: ranger-base-lcuec.md's own
// section names ANOTHER bead, in a sentence, in a nested bullet under a bare
// id, and charging it upwards reds that document for text the rule does not
// bind. Pinned as a control in
// TestVulnPrivateHitsFlagsADescriptionOnTheContinuationLine.
//
// The third is a prose RUN at column 0 with no list item above it:
//
//	<open id>\n<text>   SILENT, rc 0
//
// Markdown reads that as one paragraph and this grouping reads two units,
// because a non-opener continues a LIST item and nothing else. Closing it —
// letting a non-opener continue a non-opener too — was MEASURED at 36 ids
// newly flagged over the 679 tracked markdown files of this tree, every one
// of them ordinary prose naming a bead id on its own line, so it is ADR
// 0068's class again and stays open. The remedy AGENTS.md prescribes is a
// bulleted "Not closed here" section, which the first rule above covers.
//
// And one more the TEXT cannot carry: the content arm reads ADDED lines
// only, so a continuation already in the file is not in $posse_added at all
// — the same bound checks 0-3 share, and not this grouping's to fix.
func vulnPrivateUnits(lines []string) []int {
	unitOf := make([]int, len(lines))
	var openers []int // stack of opener line indices, by ascending indent
	for i, line := range lines {
		unitOf[i] = i
		if strings.TrimSpace(vulnPrivateContent(line)) == "" {
			openers = openers[:0] // a blank line closes every open unit
			continue
		}
		ind := vulnPrivateIndent(line)
		opens := vulnPrivateOpensUnit(line)
		for len(openers) > 0 {
			top := openers[len(openers)-1]
			if !opens && vulnPrivateListItem(lines[top]) {
				break // markdown's lazy continuation of that list item
			}
			if vulnPrivateIndent(lines[top]) < ind {
				break
			}
			openers = openers[:len(openers)-1]
		}
		if len(openers) > 0 && !opens {
			unitOf[i] = unitOf[openers[len(openers)-1]]
			continue
		}
		openers = append(openers, i)
	}
	return unitOf
}

// vulnPrivateHits reports which of ids is named, in some UNIT of text
// (vulnPrivateUnits: a line and the lines that continue it), beside
// anything vulnPrivateBare refuses — the shape AGENTS.md "Landing the plane"
// refuses. Every id present in a unit is stripped from it before the
// bareness test runs, so "- ranger-base-a, ranger-base-b" reads as bare for
// both (two bare ids sharing one bullet, exactly the form
// ranger-base-lcuec.md's own "Not closed here" uses), and an id entirely
// absent from text costs nothing beyond one strings.Contains.
//
// ids are bd's own issue ids — alnum and a hyphen, dispatch.go's beadIDRe —
// so a literal strings.ReplaceAll never needs escaping the way a shell ERE
// would.
//
// CASE-INSENSITIVE (ranger-base-68631 finding 7). It was strings.Contains
// over the text as typed, so `- RANGER-BASE-<ID> heap overflow` was
// invisible — the lowest-cost miss of that bead's seven and the least likely
// to be typed, but a check whose whole job is a mechanical reading of an id
// has no business caring how the writer shifted it. The fold is done ONCE
// over the whole text rather than per comparison: a bd id is ASCII, so
// ASCII-folding it is exact, and nothing downstream reads the text's case —
// vulnPrivateBare asks about letter-ness, which survives folding, and
// vulnPrivateHitLine prints the id as the CALLER spelled it and never the
// line (which is the one thing this check exists to keep out of a terminal).
func vulnPrivateHits(text string, ids []string) []string {
	if text == "" || len(ids) == 0 {
		return nil
	}
	lower := strings.ToLower(text)
	// folded ids, paired with the caller's own spelling for the report.
	type foldedID struct{ id, folded string }
	present := make([]foldedID, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if f := strings.ToLower(id); strings.Contains(lower, f) {
			present = append(present, foldedID{id, f})
		}
	}
	if len(present) == 0 {
		return nil
	}
	lines := strings.Split(lower, "\n")
	unitOf := vulnPrivateUnits(lines)
	units := make(map[int]string, len(lines))
	order := make([]int, 0, len(lines))
	for i, line := range lines {
		u := unitOf[i]
		if _, seen := units[u]; !seen {
			order = append(order, u)
		}
		units[u] += line + "\n"
	}
	hit := make(map[string]bool, len(present))
	for _, u := range order {
		unit := units[u]
		var inUnit []foldedID
		for _, p := range present {
			if strings.Contains(unit, p.folded) {
				inUnit = append(inUnit, p)
			}
		}
		if len(inUnit) == 0 {
			continue
		}
		rest := unit
		for _, p := range inUnit {
			rest = strings.ReplaceAll(rest, p.folded, "")
		}
		if !vulnPrivateBare(rest) {
			for _, p := range inUnit {
				hit[p.id] = true
			}
		}
	}
	if len(hit) == 0 {
		return nil
	}
	out := make([]string, 0, len(hit))
	for id := range hit {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// vulnPrivateHitLine is one flagged id's line in RunGatesVulnPrivateScan's
// output — which the shell arm folds straight into $posse_bad (gates.go) —
// and it is deliberately the only thing about the match that prints: never
// the line, never the description, which is the one thing this check
// exists to keep out of a terminal and refusals.log alike (the same
// class-only rule ADR 0048 D2 and ADR 0050 D2 already hold a CONFIGURED
// pattern's value to).
//
// It names the UNIT and not the line (ranger-base-68631 finding 1): the
// scope is a line plus whatever continues it, and a committer told "a line
// here" who then reads a clean-looking bullet has been sent to the wrong
// place to look. Which line it was stays withheld either way.
//
// It says CONTINUING and not "indented under" (ranger-base-ckcly finding 1):
// markdown's lazy continuation sits at column 0, so the old wording sent a
// committer looking for an indent that is not there.
func vulnPrivateHitLine(id string) string {
	return "  " + id + ": a line here, or a line continuing it, carries more than the bare id — the text is withheld (AGENTS.md \"Landing the plane\")\n"
}

// RunGatesVulnPrivateScan is check 4's judge: stdin is the text a
// commit-guard arm already computed (the staged markdown additions, or the
// cleaned commit message — gates.go's $posse_added either way), dir is the
// repo whose beads store answers, and the return says whether anything was
// printed — which is also the whole of what the shell side needs to know
// (gates.go's posse_vp_check: empty output is a clean scan, exactly
// posse_check's own idiom).
//
// THE QUERY RUNS ONCE PER CALL, not once per candidate id in the text:
// bd.OpenLabeledAny already pages --limit 0 and already drops closed rows
// on both store classes bd ships (beads.go, OpenLabeledAny's own doc) —
// the exact "still open" set this check needs, and not what a bare
// `bd list --status open` would answer (beads.go: in_progress, blocked and
// deferred are open questions too).
func RunGatesVulnPrivateScan(bd Bd, dir string, stdin io.Reader, stdout io.Writer) (hit bool) {
	content, err := io.ReadAll(stdin)
	if err != nil || len(content) == 0 {
		return false
	}
	if !bd.Available() {
		return false
	}
	issues, err := bd.OpenLabeledAny(dir, VulnPrivateLabel)
	if err != nil || len(issues) == 0 {
		return false
	}
	ids := make([]string, len(issues))
	for i, is := range issues {
		ids[i] = is.ID
	}
	flagged := vulnPrivateHits(string(content), ids)
	for _, id := range flagged {
		fmt.Fprint(stdout, vulnPrivateHitLine(id))
	}
	return len(flagged) > 0
}
