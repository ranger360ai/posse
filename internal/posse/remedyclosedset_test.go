package posse

// A REMEDY SENTENCE IS A CLOSED SET (ranger-base-mxbq0 finding 1, discovered
// from the verify at ranger-base-6rj8w).
//
// THE CLASS is ranger-base-68631 finding 6, "the remedy promises more than
// the set allows" — ADR 0068's class one step in, a way through that does
// not go through. A refused writer does what the sentence tells them; if the
// sentence is wrong about what the rule allows, the remedy is a loop.
//
// WHY A BAG OF REQUIRED PHRASES IS NOT ENOUGH, which is what every reader of
// these four constants was before this file. remedyPromises asks only
// whether a named phrase is STILL in the sentence, and the behavioural rows
// beside it cover only the nouns the sentence already names. So the
// drop-a-promise direction reds and the ADD-a-promise direction has no
// reader at all: a clause appended to a remedy is a way through that nobody
// measured, with every pin green and the test's own name
// ("PromisesOnlyWhatTheRuleAllows") claiming the closed-set thing it does
// not measure.
//
// MEASURED 2026-11-13 (this Linux host, go1.27.3, git 2.56.0, at 983ae5d3),
// two mutants through `go test -overlay` with the tracked tree unmodified,
// each coherent and each displacing no phrase any pin greps for:
//
//   - IdentityWayThrough += "Either arm also clears if you stage the file
//     under docs/, which this check does not scan."
//   - IdentityMessageWayThrough += "If you would rather keep the wording,
//     put the identifier in the commit TRAILER instead: trailers are not
//     scanned."
//
// Both PASSED every check-3 and check-4 pin in the tree. And the second is
// FALSE, measured rather than argued: a visWall commit whose only identity
// literal is in a trailer line, over a clean staged file, is refused with
// the header "an operator identity literal in the commit MESSAGE" —
// IdentityRule's own "every line of the COMMIT MESSAGE". A refused writer
// sent there by the remedy is refused again, which is the loop exactly.
//
// SO THE READER IS CLOSED: remedyIsClosed holds each constant to its whole
// text, and the author of an edit has to come back to the test that holds
// the behavioural rows. That is the only mechanism that catches an ADDED
// clause, because a new promise has by construction no phrase for a pin to
// be missing. The cost is that re-WRAPPING a refusal reds too — which is why
// both halves collapse whitespace first (remedyPromises's own reason: these
// sentences are hard-wrapped at the width a terminal refusal prints, so a
// phrase worth naming straddles a line break).
//
// ONE PATTERN OVER BOTH CHECKS. ranger-base-2nj35 matched the house
// precedent when it read check 3's two constants one-directionally: check
// 4's two readers (vulnprivate_test.go) were a plain strings.Contains in the
// same direction. Both checks are held here, so the precedent is the fixed
// one and not the broken one.
//
// UNTAGGED, so it runs in all three arms — deliberately, and it is the one
// thing in this file that is a preference. The golden texts could have sat
// beside the behavioural rows they point at, and two of those row sets live
// in arm 3 (checkthreeremedy_qa_test.go, which drives the real rendered
// hook). A text compare needs no fixture and costs nothing, so putting it
// here makes an edit to a check-3 remedy red in a bare `go test ./...` as
// well as in `make test`; the failure message carries the pointer to the
// rows instead.

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// remedyPromises reports whether remedy sentence s makes promise p.
//
// Whitespace-collapsed on both sides, which is the one difference from a
// plain strings.Contains: check 3's two constants are HARD-WRAPPED at the
// width a terminal refusal prints, so a phrase worth naming straddles a
// line break ("this\ninstance's path", "a fresh added\nentry") and a raw
// Contains would be a pin on the wrapping. Re-wrapping a refusal is not
// drift; re-wording it is. Check 4's two are concatenated single-line
// literals, where the collapse is a no-op — one spelling for both checks
// beats two that differ for a reason no reader of either can see.
func remedyPromises(s, p string) bool {
	return strings.Contains(remedyText(s), remedyText(p))
}

func remedyText(s string) string { return strings.Join(strings.Fields(s), " ") }

// remedyIsClosed holds remedy constant `name` to the WHOLE of want: the
// closed-set half of the pattern above, and the only half that can see a
// promise that was ADDED.
//
// rows is where the behavioural rows for this constant live — the message
// names it, because the step this reader exists to force is not "update the
// golden text", it is "go to the rows and measure the clause you just
// wrote".
func remedyIsClosed(t *testing.T, name, rows, got, want string) {
	t.Helper()
	g, w := remedyText(got), remedyText(want)
	if g == w {
		return
	}
	t.Errorf("%s has changed, and this pin is CLOSED over its whole text — every clause of a remedy is a promise a refused writer performs, so a clause with no behavioural row is a way through that nobody measured (ranger-base-68631 finding 6, re-found as ranger-base-mxbq0 finding 1).\n"+
		"  is:   %q\n  want: %q\n  first difference at byte %d:\n    is:   %q\n    want: %q\n"+
		"Go to %s, add or amend the row that measures what changed, and then make this text match. A re-WRAP is not drift and does not reach here: both sides are whitespace-collapsed.",
		name, g, w, remedyDiffAt(g, w), remedyTail(g, w), remedyTail(w, g), rows)
}

// remedyDiffAt answers the byte offset of the first difference.
func remedyDiffAt(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// remedyTail answers what is left of a from the first byte it differs from b,
// clipped so a failure is readable.
//
// Clipped by RUNE and not by byte: every one of these sentences carries an
// em dash, so a byte clip would print a half rune in the one message a
// reader of this pin ever sees.
func remedyTail(a, b string) string {
	i := remedyDiffAt(a, b)
	for i > 0 && !utf8.RuneStart(a[i]) {
		i-- // the divergence fell inside a multibyte rune
	}
	r := []rune(a[i:])
	if len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return string(r)
}

// The four remedy constants, as a closed set. Each `want` is the sentence
// the behavioural rows in `rows` measure, and nothing else.
func TestQARemedySentencesAreClosedSets(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, rows, got, want string }{
		{
			"IdentityWayThrough (check 3, the content and path arms)",
			"TestQAIdentityRemedyPromisesOnlyWhatCheckThreeAllows (checkthreeremedy_qa_test.go, arm 3)",
			IdentityWayThrough,
			`the way through: nothing that names this operator, this box, or this
instance's path belongs in a public repo's history — generalize whatever
carries it (a comment, a fixture, a hardcoded path), or move the content to
the instance tree (ADR 0024 D3, restate-and-cite). For a PATH: name the file
without it, or write it in the instance tree — a rename is a fresh added
entry and is scanned again, so the new name has to be clean too.`,
		},
		{
			"IdentityMessageWayThrough (check 3, the message arm)",
			"TestQAIdentityMessageRemedyPromisesOnlyWhatTheRuleAllows (checkthreeremedy_qa_test.go, arm 3)",
			IdentityMessageWayThrough,
			`the way through: rewrite the commit message. Nothing that names this
operator, this box, or this instance's path belongs in a public repo's
history, and a message replicates with the branch exactly as a staged line
does — generalize whatever carries it, or say what you did without the
identifier and cite the bead id (ADR 0024 D3, restate-and-cite). Nothing was
committed: HEAD is unchanged and your tree and index are exactly where you
left them. The message you typed is still in .git/COMMIT_EDITMSG, a local
file that never replicates and that your next commit overwrites; that is the
same residual a refused staged file leaves.`,
		},
		{
			"VulnPrivateWayThrough (check 4, the content and path arms)",
			"TestVulnPrivateHitsAcceptsEveryDecorationTheRemedyPromises and\n\t\t\t\tTestRunGatesVulnPrivateScanExemptsTheCommittingTreesOwnBead (vulnprivate_test.go)",
			VulnPrivateWayThrough,
			`the way through: write the bare id alone — whitespace, a list bullet or
numbering, backticks, bold markers, brackets, quotes and punctuation are all
fine; a LETTER is not, because a word beside the id is the description the
rule refuses. The detail belongs on the bead, where the audience is already
the right one. One id is never flagged here: the bead this tree's branch is
recorded for, whose fix is landing in this commit. So a flagged id names
ANOTHER hole — or this tree carries no such record, which a session worktree
always has and a shared checkout never does: write the fragment from your
worktree.`,
		},
		{
			"VulnPrivateMessageWayThrough (check 4, the message arm)",
			"TestVulnPrivateMessageRemedyPromisesOnlyWhatTheRuleAllows and\n\t\t\t\tTestRunGatesVulnPrivateScanExemptsTheCommittingTreesOwnBead (vulnprivate_test.go)",
			VulnPrivateMessageWayThrough,
			`the way through: rewrite the commit message so the open ` + "`vuln-private`" + ` id
stands alone — punctuation and decoration are fine, a LETTER beside the id
is not, so a description, path, mechanism or reproduction is out. Nothing
was committed: HEAD is unchanged and your tree and index are exactly where
you left them. The message you typed is still in .git/COMMIT_EDITMSG, a
local file that never replicates and that your next commit overwrites. One
id is never flagged here: the bead this tree's branch is recorded for, whose
fix is landing in this commit. So a flagged id names ANOTHER hole — or this
tree carries no such record, which a session worktree always has and a
shared checkout never does.`,
		},
	} {
		remedyIsClosed(t, c.name, c.rows, c.got, c.want)
	}
}

// remedyPromises's own door. It is the one helper both checks' row tests
// read, so a bug in it would make every row above vacuously green — which
// is the failure this whole file is about, one level down.
func TestQARemedyPromisesReadsAcrossAHardWrap(t *testing.T) {
	t.Parallel()
	const wrapped = "or move the content to\nthe instance tree"
	if !remedyPromises(wrapped, "move the content to the instance tree") {
		t.Error("remedyPromises cannot read a phrase across a line break, so every row that names one is a pin on the wrapping")
	}
	if remedyPromises(wrapped, "move the content to the operator tree") {
		t.Error("remedyPromises answered true for a phrase that is not there — every row above is then vacuous")
	}
	// The collapse is not a licence to ignore word boundaries.
	if remedyPromises(wrapped, "contentto") {
		t.Error("remedyPromises collapsed a phrase's own spaces away")
	}
}
