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
const VulnPrivateWayThrough = "the way through: write the bare id alone — a list bullet, backticks, bold " +
	"and surrounding punctuation are fine, nothing else is. The detail belongs " +
	"on the bead, where the audience is already the right one."

// VulnPrivateMessageWayThrough mirrors IdentityMessageWayThrough's shape
// (visibility.go): the text is not in a file the writer can edit, it is
// .git/COMMIT_EDITMSG, local and unreplicated until the next commit
// overwrites it.
const VulnPrivateMessageWayThrough = "the way through: rewrite the commit message so the open `vuln-private` id " +
	"stands alone — decoration is fine, a description, path, mechanism or " +
	"reproduction is not. Nothing was committed: HEAD is unchanged and your tree " +
	"and index are exactly where you left them. The message you typed is still " +
	"in .git/COMMIT_EDITMSG, a local file that never replicates and that your " +
	"next commit overwrites."

// vulnPrivateDecoration is every byte a line may carry beside an open
// vuln-private id and still read as "named by bead id and nothing else":
// whitespace, a list bullet or numbering, backticks, bold markers, and the
// ordinary punctuation that separates two bare ids sharing one line or
// bullet. A LETTER IS NEVER IN THIS SET — "see", "stays open", "and" are
// exactly the descriptive text the rule refuses, and the strictness is
// deliberate: AGENTS.md says "and nothing else", not "and little else".
const vulnPrivateDecoration = " \t.,:;()*+#`-0123456789"

// vulnPrivateHits reports which of ids is named, on some line of text,
// beside anything but vulnPrivateDecoration — the shape AGENTS.md "Landing
// the plane" refuses. Every id present on a line is stripped from it
// before the bareness test runs, so "- ranger-base-a, ranger-base-b" reads
// as bare for both (two bare ids sharing one bullet, exactly the form
// ranger-base-lcuec.md's own "Not closed here" uses), and an id entirely
// absent from text costs nothing beyond one strings.Contains.
//
// ids are bd's own issue ids — alnum and a hyphen, dispatch.go's beadIDRe —
// so a literal strings.ReplaceAll never needs escaping the way a shell ERE
// would.
func vulnPrivateHits(text string, ids []string) []string {
	if text == "" || len(ids) == 0 {
		return nil
	}
	present := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && strings.Contains(text, id) {
			present = append(present, id)
		}
	}
	if len(present) == 0 {
		return nil
	}
	hit := make(map[string]bool, len(present))
	for _, line := range strings.Split(text, "\n") {
		var onLine []string
		for _, id := range present {
			if strings.Contains(line, id) {
				onLine = append(onLine, id)
			}
		}
		if len(onLine) == 0 {
			continue
		}
		rest := line
		for _, id := range onLine {
			rest = strings.ReplaceAll(rest, id, "")
		}
		if strings.Trim(rest, vulnPrivateDecoration) != "" {
			for _, id := range onLine {
				hit[id] = true
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
func vulnPrivateHitLine(id string) string {
	return "  " + id + ": a line here carries more than the bare id — the text is withheld (AGENTS.md \"Landing the plane\")\n"
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
