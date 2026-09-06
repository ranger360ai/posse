//go:build posse_arm3

package posse

// ranger-base-1xh24, verifying ranger-base-n6s2u: the two sign-in rules'
// `region` numbers were pinned one line short of their real edge, because
// codex's startup ASCII logo is NOT a constant height and every fixture
// committed with the fix happens to carry the shorter one.
//
// MEASURED on codex-cli 0.153.4, 60 fresh empty CODEX_HOMEs in tmux panes at
// 60 columns: 22 distinct logo arts, at TWO non-empty heights — 15 rows (40
// draws) and 16 rows (20). The extra row is a thin tail of the art, and it
// pushes everything below it down one line:
//
//	logo rows   "2. Sign in with Device Code"   "Paste or type your API key"
//	15          non-empty line 23              non-empty line 18
//	16          non-empty line 24              non-empty line 19
//
// signin_menu reads top_non_empty_lines(24) and signin_api_key reads 20, so
// both are still correct — no live pane is misread. But one 60-column menu in
// three lands on 24, the LAST line the region admits, and the fix's own pins
// could not see it: blocked-signin{,-narrow}.txt and blocked-signin-api-key.txt
// all carry 15-row logos, so `region 24 -> 23` and `region 20 -> 18` both went
// green over `make verify-detection` and every arm of
// codexsignin_qa_test.go — while a real 16-row-logo pane reads `idle` / rule
// `none` under either, which is the default_known_agent_idle_fallback that
// ranger-base-n6s2u, rangerhq-9py0 and rangerhq-7ia all exist to close.
//
// So the tall-logo captures are committed beside the short ones and the region
// of each rule is mutated at the line that actually decides it. The heights
// are asserted first: a re-capture that happens to draw a 15-row logo would
// otherwise return these fixtures to the shallow case and silently retire
// every arm below.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// The deep clause of each rule, and the non-empty line it sits on when
	// codex draws the TALLER of its two logos on a 60-column pane.
	codexTallMenuClause  = "2. Sign in with Device Code"
	codexTallMenuLine    = 24
	codexTallKeyClause   = "Paste or type your API key"
	codexTallKeyLine     = 19
	codexShortMenuLine   = 23
	codexShortKeyLine    = 18
	codexTallMenuFixture = "blocked-signin-tall-logo.txt"
	codexTallKeyFixture  = "blocked-signin-api-key-tall-logo.txt"
)

// codexNonEmptyIndex reports which non-empty line of the capture first carries
// clause, counting the way herdr's top_non_empty_lines() does — a row of the
// logo's U+00A0 padding is blank, which is why `strings.TrimSpace` and not a
// test for "" is the right question. It returns 0 when the clause is absent.
func codexNonEmptyIndex(t *testing.T, fixture, clause string) int {
	t.Helper()
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
		if strings.Contains(line, clause) {
			return n
		}
	}
	return 0
}

// codexSetRegion rewrites one rule's region depth, leaving every other rule
// alone. It fails rather than returning the manifest unchanged: a rewrite that
// matched nothing is an arm measuring the shipped number twice.
func codexSetRegion(t *testing.T, toml, ruleID, depth string) string {
	t.Helper()
	var kept []string
	hit, inRule := 0, false
	for _, line := range strings.Split(toml, "\n") {
		if strings.Contains(line, `id = "`+ruleID+`"`) {
			inRule = true
		} else if strings.HasPrefix(line, "[[rules]]") {
			inRule = false
		}
		if inRule && strings.HasPrefix(line, "region = ") {
			line = `region = "top_non_empty_lines(` + depth + `)"`
			hit++
		}
		kept = append(kept, line)
	}
	if hit != 1 {
		t.Fatalf("codexSetRegion(%s): rewrote %d region lines, want exactly 1 — the rule's shape moved under this test", ruleID, hit)
	}
	return strings.Join(kept, "\n")
}

// TestQACodexSignInTallLogoCapturesAreTheDeepCase is the guard on the two arms
// below. Both region mutants are only worth running against a capture that
// actually reaches the region's last line; a re-capture drawing codex's
// 15-row logo would put the clause one line higher, and then the mutants pass
// for the same reason the short fixtures do — measuring nothing, greenly.
func TestQACodexSignInTallLogoCapturesAreTheDeepCase(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(detectionDir(t), "testdata", "codex")
	for _, tc := range []struct {
		fixture, clause string
		want            int
	}{
		{codexTallMenuFixture, codexTallMenuClause, codexTallMenuLine},
		{codexTallKeyFixture, codexTallKeyClause, codexTallKeyLine},
		// The short captures the fix shipped with, named here so the pair is
		// visible: these are the ones that cannot see the edge. Both numbers
		// are one less than the rows above, which is the whole finding.
		{"blocked-signin-narrow.txt", codexTallMenuClause, codexShortMenuLine},
		{"blocked-signin-api-key.txt", codexTallKeyClause, codexShortKeyLine},
	} {
		got := codexNonEmptyIndex(t, filepath.Join(dir, tc.fixture), tc.clause)
		if got != tc.want {
			t.Errorf("%s: %q is non-empty line %d, want %d — codex draws a 15-row or a 16-row logo and this "+
				"capture is no longer the one this file says it is; re-capture until the taller logo comes up "+
				"(about one launch in three) rather than relaxing the number",
				tc.fixture, tc.clause, got, tc.want)
		}
	}
}

// TestQACodexSignInTallLogoCapturesAreBlockedByTheShippedManifest is the arm
// that actually holds the two region numbers, and it is the arm the fix was
// missing. Every other region test in this package — the two below and
// codexsignin_qa_test.go's — REWRITES the region before explaining, so it
// asks its own question and passes identically no matter what the committed
// manifest says. Only a capture deep enough to need the shipped depth,
// explained against the manifest UNMODIFIED, fails when someone walks 24 back
// to 23 or 20 back to 18.
func TestQACodexSignInTallLogoCapturesAreBlockedByTheShippedManifest(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	manifest := codexManifest(t)
	dir := filepath.Join(detectionDir(t), "testdata", "codex")

	for _, tc := range []struct{ fixture, rule string }{
		{codexTallMenuFixture, "signin_menu"},
		{codexTallKeyFixture, "signin_api_key"},
	} {
		state, rule, fallback := codexExplain(t, filepath.Join(dir, tc.fixture), manifest)
		if state != "blocked" || rule != tc.rule {
			t.Errorf("%s under the committed manifest: state=%q rule=%q fallback=%q, want blocked/%s — "+
				"this is codex's TALLER logo, one line deeper than the captures %s shipped with, and it "+
				"needs the region the manifest declares; a pane reading idle here is typed into",
				tc.fixture, state, rule, fallback, tc.rule, tc.rule)
		}
	}
}

// TestQACodexSignInMenuRegionIsPinnedAtItsRealEdge walks signin_menu's region
// down to 23 — the value codexsignin_qa_test.go stops one short of, and the
// value a "make it match its siblings" edit would pass through on its way to
// 20. At 23 the tall-logo pane is the one that breaks, and the short one does
// not: that pair is what says the 24th line is load-bearing rather than slack.
func TestQACodexSignInMenuRegionIsPinnedAtItsRealEdge(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	manifest := codexManifest(t)
	dir := filepath.Join(detectionDir(t), "testdata", "codex")

	for _, tc := range []struct{ region, fixture, want string }{
		// Shipped. Both real logo heights are named.
		{"24", codexTallMenuFixture, "blocked"},
		{"24", "blocked-signin-narrow.txt", "blocked"},
		// One line back. The 16-row-logo pane falls out of the region and
		// reads the unrecognized-idle this rule was written to end...
		{"23", codexTallMenuFixture, "idle"},
		// ...while the 15-row-logo pane of the same width still passes, which
		// is what makes the row above a measurement of the 24 and not of the
		// capture being broken.
		{"23", "blocked-signin-narrow.txt", "blocked"},
	} {
		state, rule, fallback := codexExplain(t, filepath.Join(dir, tc.fixture), codexSetRegion(t, manifest, "signin_menu", tc.region))
		if state != tc.want {
			t.Errorf("signin_menu region %s on %s: state=%q rule=%q fallback=%q, want %s",
				tc.region, tc.fixture, state, rule, fallback, tc.want)
		}
		if tc.want == "blocked" && rule != "signin_menu" {
			t.Errorf("signin_menu region %s on %s: rule=%q, want signin_menu", tc.region, tc.fixture, rule)
		}
	}
}

// TestQACodexSignInAPIKeyRegionIsPinnedAtItsRealEdge is the same arm for
// signin_api_key, which shipped with no region mutant at all and no capture
// deeper than its 15-row-logo one. Its region is 20 and the deep clause lands
// on 19, so 19 must hold and 18 must not.
func TestQACodexSignInAPIKeyRegionIsPinnedAtItsRealEdge(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	manifest := codexManifest(t)
	dir := filepath.Join(detectionDir(t), "testdata", "codex")

	for _, tc := range []struct{ region, fixture, want string }{
		{"20", codexTallKeyFixture, "blocked"},
		{"20", "blocked-signin-api-key.txt", "blocked"},
		// 19 still reaches the tall pane's clause: the last line that does.
		{"19", codexTallKeyFixture, "blocked"},
		// 18 does not — and the short capture, which is all the fix shipped
		// with, stays green over exactly that edit.
		{"18", codexTallKeyFixture, "idle"},
		{"18", "blocked-signin-api-key.txt", "blocked"},
	} {
		state, rule, fallback := codexExplain(t, filepath.Join(dir, tc.fixture), codexSetRegion(t, manifest, "signin_api_key", tc.region))
		if state != tc.want {
			t.Errorf("signin_api_key region %s on %s: state=%q rule=%q fallback=%q, want %s",
				tc.region, tc.fixture, state, rule, fallback, tc.want)
		}
		if tc.want == "blocked" && rule != "signin_api_key" {
			t.Errorf("signin_api_key region %s on %s: rule=%q, want signin_api_key", tc.region, tc.fixture, rule)
		}
	}
}
