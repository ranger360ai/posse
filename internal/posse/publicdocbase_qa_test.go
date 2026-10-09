package posse

import (
	"os"
	"os/exec"
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
// INSTALL.md paths, by internal/treepins/quickstart_test.go, so a rewrite of
// one side reds here and a rewrite of both reds there.
//
// That last clause is true of the ORG and the REPO and was NOT true of the
// REF, which is why the ref now gets its own reading below
// (ranger-base-c4uyj finding 2, escaped from ranger-base-c768n).
// quickstart_test.go's reader of the live page is `landingPageBrewPanelLink`,
// and what it requires of an href is the org, `INSTALL.md` and the step-2
// anchor — it never reads `blob/main`. MEASURED 2026-10-09, three arms:
// rewriting `ranger360ai` on BOTH sides reds internal/treepins
// (TestLandingPageBrewPanelLinksToInstallStep2 and
// TestLandingPageCaptionLinksHerdrAndBeadsToInstallStep1), rewriting
// `blob/main` -> `blob/no-such-branch` on ONE side reds the comparison here,
// and rewriting it on BOTH sides left every pin in the tree green over a
// base whose branch does not exist — the census found the ref spelled only
// in publicdoc.go, in those two hrefs, and in hand-built fixture strings.
// Two agreeing literals are not a reading, so the ref is asked of GIT: a
// repository cannot be edited into having a branch by writing its name
// twice.
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

	// THE REF, asked of the repository. Everything above compares two
	// spellings in the tree, and the header says why that holds the org and
	// the repo but cannot hold this segment: both sides are files an edit can
	// move together. `git rev-parse` is the one reader in this pin whose
	// answer no edit to the site or the const can arrange.
	ref := qaBlobRefSegment(t, publicDocBase)
	qibSkipUnlessCheckout(t, root)
	// Local first, then the remote-tracking copy: a clone that fetched only
	// the branch it is on has `origin/main` and no `main`, and that is a
	// checkout of a repository that HAS the ref, not a 404. `^{commit}` so a
	// tag or a SHA in this segment — both legitimate things to publish a doc
	// base from — resolves like a branch does.
	tried := []string{ref, "origin/" + ref}
	have := false
	for _, cand := range tried {
		if exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", cand+"^{commit}").Run() == nil {
			have = true
			break
		}
	}
	if have {
		return
	}
	t.Errorf("publicDocBase is %q and its ref segment %q names nothing this checkout has (tried %s) — every page the detection doors cite renders a 404, and until this reading existed nothing in the tree said so (ranger-base-c4uyj finding 2).\n"+
		"  the org and the repo are held by comparing two files; this segment cannot be, because an edit can move both files together and MEASURED 2026-10-09 one did, with every pin green.\n"+
		"  if the default branch really was renamed, rename it here and in %s together — the comparison above is what makes that one edit instead of two.",
		publicDocBase, ref, strings.Join(tried, ", "), rel)
}

// qaBlobRefSegment returns the ref segment of a `.../blob/<ref>/` base. It
// fails rather than guessing: a base this pin cannot take a ref out of is a
// base whose shape has changed, and a silent "" would make the git reading
// that follows it ask about nothing.
func qaBlobRefSegment(t *testing.T, base string) string {
	t.Helper()
	m := regexp.MustCompile(`/blob/([^/]+)/`).FindStringSubmatch(base)
	if m == nil {
		t.Fatalf("publicDocBase %q has no /blob/<ref>/ segment — this pin's shape rule has changed and the ref reading below would hold nothing", base)
	}
	return m[1]
}
