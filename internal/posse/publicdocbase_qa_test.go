package posse

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The base half of publicDocBase, which nothing compared (ranger-base-c768n
// finding 1, escaped from ranger-base-x8bv0).
//
// publicdoc.go's own header argues that the branch name is PINNED and not
// avoided — "`blob/main/<path>` is already the house spelling, www/index.html
// ships two of them and internal/treepins/quickstart_test.go pins them" — and
// that what makes the URL safe is that it is composed rather than written
// out, so one pin can stat the path half in the tree. Both halves of that
// argument were true of the PATH and neither was true of the BASE: host, org,
// repo and branch were a literal compared with nothing.
//
// MEASURED 2026-10-09 — by the bead that filed this and again here before
// the fix — two mutants, both SURVIVED the detection doors'
// existing pin: rewriting `ranger360ai` to another org, or `blob/main` to
// `blob/no-such-branch`, left
// `TestQADetectionDoorCitesAFilingThatExists` green, because its comparison
// is `publicDoc(rel) == publicDocBase+rel` — a tautology over the base. Every
// URL a detection refusal hands an operator could point at a fork, or at a
// branch that does not exist, and every pin stayed green.
//
// So this derives the base from the site, which is the other half of the
// tree the header already named. It is a real derivation and not a second
// literal: the hrefs in www/index.html are pinned verbatim, with their
// INSTALL.md paths, by internal/treepins/quickstart_test.go, so the pair
// cannot be made to agree by editing both — a rewrite of one reds here and a
// rewrite of both reds there.
func TestQAPublicDocBaseIsTheSpellingTheSiteShips(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not a hand-rolled climb: the tree-wide door census in
	// internal/treepins derives this pin's class from that one helper
	// (ranger-base-sx2dq), and a pin outside the class gets no door.
	root := qibRepoRoot(t)

	rel := filepath.Join("www", "index.html")
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("%s: %v — the published site is where this base's other spelling lives, and a pin that cannot read it holds nothing", rel, err)
	}

	// `https://github.com/<org>/<repo>/blob/<ref>/`, up to and including the
	// slash the path hangs off, which is exactly what publicDocBase is. The
	// host is literal and the three segments are not: a move to another host
	// is a change to this rule, which is the right size of edit for it, while
	// a drifted org, repo or branch is what the comparison below catches. The
	// segment class refuses a quote, a space and a `#`, so an href's
	// fragment and the attribute's closing quote stop the match.
	base := regexp.MustCompile(`https://github\.com/[^/"'\s#]+/[^/"'\s#]+/blob/[^/"'\s#]+/`)
	hits := base.FindAllString(string(b), -1)

	// The positive witness, below which this pin is satisfied by finding
	// nothing: two hrefs ship today (www/index.html:354,364, MEASURED
	// 2026-10-09), both pinned in quickstart_test.go.
	if len(hits) < 2 {
		t.Fatalf("%s carries %d blob URL(s), want at least 2 (MEASURED 2026-10-09: www/index.html:354,364) — the site has stopped spelling this base, so there is nothing left to derive publicDocBase from and it is an unheld literal again", rel, len(hits))
	}

	seen := map[string]bool{}
	var bases []string
	for _, h := range hits {
		if !seen[h] {
			seen[h] = true
			bases = append(bases, h)
		}
	}
	sort.Strings(bases)
	if len(bases) != 1 {
		t.Fatalf("%s spells %d different blob bases (%s) — and this derivation cannot tell which case that is.\n"+
			"  if the site drifted, make them one and this pin holds publicDocBase to it.\n"+
			"  if one of them is a link to ANOTHER repository's blob URL — a legitimate thing for a landing page to carry — then the rule that the site spells ONE base has stopped being the right derivation, and this pin needs narrowing to the hrefs that name THIS repository. Narrow it deliberately: a derivation widened until it matches whatever is there holds nothing (ranger-base-c768n finding 1).",
			rel, len(bases), strings.Join(bases, ", "))
	}

	if got, want := publicDocBase, bases[0]; got != want {
		t.Errorf("publicDocBase is %q and %s ships %q — one base, two literals, and a reader with no checkout follows whichever the refusal rendered.\n"+
			"  host, org, repo and branch are all in that string: a fork's org, or a branch that does not exist, renders a 404 for every page the detection doors cite and nothing else says so (ranger-base-c768n finding 1).\n"+
			"  the site's hrefs are pinned verbatim in internal/treepins/quickstart_test.go, so fix the const — or, if the repository really did move, move both and that pin will say so.",
			got, rel, want)
	}

	// And the composition, stated here rather than assumed: the base is a
	// PREFIX of what publicDoc renders, so the derivation above is about the
	// URLs the doors actually hand over and not about an unused const.
	const sample = "docs/runbooks/agent-detection-manifest.md"
	if got := publicDoc(sample); !strings.HasPrefix(got, bases[0]) {
		t.Errorf("publicDoc(%q) = %q, which does not begin with the base the site ships (%q) — publicDoc has stopped composing from publicDocBase, so holding that const holds nothing about the rendered URL", sample, got, bases[0])
	}
}
