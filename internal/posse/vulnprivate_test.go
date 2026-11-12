package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVulnPrivateHitsBareIDIsClean is the compliant shape AGENTS.md
// "Landing the plane" prescribes, every form the rule's own prose and
// ranger-base-lcuec.md's own "Not closed here" use: a bullet, backticks,
// bold, numbering, a trailing colon, two bare ids sharing one bullet.
func TestVulnPrivateHitsBareIDIsClean(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c", "ranger-base-gzl2w"}
	for _, line := range []string{
		"ranger-base-l751c",
		"- ranger-base-l751c",
		"* ranger-base-l751c",
		"1. ranger-base-l751c",
		"- `ranger-base-l751c`",
		"- **ranger-base-l751c**",
		"- ranger-base-l751c:",
		"- ranger-base-l751c, ranger-base-gzl2w",
		"(ranger-base-l751c)",
		"  ranger-base-l751c  ",
	} {
		if got := vulnPrivateHits(line, ids); got != nil {
			t.Errorf("vulnPrivateHits(%q) = %v, want no hit — this is the bare-id shape the rule allows", line, got)
		}
	}
}

// TestVulnPrivateHitsDescriptiveTextIsFlagged is the shape the rule
// refuses: anything beyond decoration, including the exact "see also" case
// the bead's own description worried a mechanical rule could miss without
// reading every line that kind of carefully. It does not: a letter is
// never in the allowed decoration set, so "see" trips it the same as a
// full description would.
func TestVulnPrivateHitsDescriptiveTextIsFlagged(t *testing.T) {
	t.Parallel()
	ids := []string{"ranger-base-l751c"}
	for _, line := range []string{
		"- ranger-base-l751c stays open",
		"see also ranger-base-l751c",
		"ranger-base-l751c: the remote-settings cache",
		"fixed by ranger-base-l751c and the bwrap mask",
	} {
		if got := vulnPrivateHits(line, ids); len(got) != 1 || got[0] != ids[0] {
			t.Errorf("vulnPrivateHits(%q) = %v, want [%s] — descriptive text beside an open vuln-private id must be flagged", line, got, ids[0])
		}
	}
}

// TestVulnPrivateHitsIgnoresUnknownIDs: a bead id that is not in the
// caller's open-vuln-private set is not this check's business at all —
// lcuec.md's own "Not closed here" names two ordinary beads in full
// sentences, and AGENTS.md's rule does not bind them (not vuln-private).
func TestVulnPrivateHitsIgnoresUnknownIDs(t *testing.T) {
	t.Parallel()
	if got := vulnPrivateHits("ranger-base-wuuqi stays open, the operator's call", nil); got != nil {
		t.Errorf("vulnPrivateHits with no known ids = %v, want nil", got)
	}
	if got := vulnPrivateHits("ranger-base-wuuqi stays open, the operator's call", []string{"ranger-base-lcuec"}); got != nil {
		t.Errorf("vulnPrivateHits for an id not present = %v, want nil", got)
	}
}

// TestVulnPrivateHitsMultilineOnlyFlagsTheBadLine: a message or a markdown
// diff is more than one line, and a bare id two lines above a description
// of something else entirely must not be swept up with it.
func TestVulnPrivateHitsMultilineOnlyFlagsTheBadLine(t *testing.T) {
	t.Parallel()
	text := "wire the reaper\n\n## Not closed here\n- ranger-base-l751c\n- ranger-base-gzl2w: the remote-settings cache\n"
	got := vulnPrivateHits(text, []string{"ranger-base-l751c", "ranger-base-gzl2w"})
	if len(got) != 1 || got[0] != "ranger-base-gzl2w" {
		t.Errorf("vulnPrivateHits = %v, want [ranger-base-gzl2w] — the bare line above it must not be flagged", got)
	}
}

// qaFakeBd points a Bd at the package's fake-binary harness (herdr_test.go)
// so RunGatesVulnPrivateScan reads fixture JSON instead of a real store.
func qaFakeBd(t *testing.T) Bd {
	t.Helper()
	return Bd{Bin: fakeBinFor(t, "bd")}
}

func writeFakeOpenLabeled(t *testing.T, dir, json string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "fake-list-labeled.json"), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunGatesVulnPrivateScanFlagsAnOpenResidualWithText is the end-to-end
// shape: one open vuln-private bead, named with a description on stdin.
func TestRunGatesVulnPrivateScanFlagsAnOpenResidualWithText(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenLabeled(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["code","security","vuln-private"]}]`)
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader("## Not closed here\n- ranger-base-l751c: the remote-settings cache\n"), &out)
	if !hit {
		t.Fatalf("want a hit, got none; output: %q", out.String())
	}
	if !strings.Contains(out.String(), "ranger-base-l751c") {
		t.Errorf("output must name the flagged id: %q", out.String())
	}
	if strings.Contains(out.String(), "remote-settings") {
		t.Errorf("output must withhold the description, class-only: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanBareIDIsClean: the same bead, named bare —
// exactly what the wall's own remedy prescribes — must not be flagged.
func TestRunGatesVulnPrivateScanBareIDIsClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenLabeled(t, dir, `[{"id":"ranger-base-l751c","status":"in_progress","labels":["code","security","vuln-private"]}]`)
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader("## Not closed here\n- ranger-base-l751c\n"), &out)
	if hit {
		t.Errorf("a bare id must not be flagged, got: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanClosedResidualIsNotFlagged: the rule's own
// words are "until that residual's own fix lands" — a closed bead is no
// longer a residual, and OpenLabeledAny's whole job is dropping it on
// every store class (openlabeledany_storeclass_test.go).
func TestRunGatesVulnPrivateScanClosedResidualIsNotFlagged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFakeOpenLabeled(t, dir, `[{"id":"ranger-base-l751c","status":"closed","labels":["code","security","vuln-private"]}]`)
	if err := os.WriteFile(filepath.Join(dir, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader("- ranger-base-l751c: now public, the fix shipped\n"), &out)
	if hit {
		t.Errorf("a closed vuln-private residual is no longer this check's business, got: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanFailsOpenWithNoBd is ADR 0068's lesson,
// applied: this check's own answer to "what does it do when it cannot
// reach the store" is never refuse — a refusal with no remedy the
// committer can perform is a lockout, not a gate.
func TestRunGatesVulnPrivateScanFailsOpenWithNoBd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bd := Bd{Bin: filepath.Join(dir, "no-such-bd-binary")}
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(bd, dir, strings.NewReader("- ranger-base-l751c: a description that would otherwise trip this\n"), &out)
	if hit {
		t.Errorf("no bd on PATH must fail open, got: %q", out.String())
	}
}

// TestRunGatesVulnPrivateScanEmptyStdinIsClean: nothing to scan is nothing
// to flag, and the fake binary must never even be asked.
func TestRunGatesVulnPrivateScanEmptyStdinIsClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No fixture written at all: if this call reaches bd, the fake answers
	// with its own usage error rather than an issue list, and a bug that
	// skipped the empty-input guard would surface as a false hit here.
	var out strings.Builder
	hit := RunGatesVulnPrivateScan(qaFakeBd(t), dir, strings.NewReader(""), &out)
	if hit {
		t.Errorf("empty input must never hit, got: %q", out.String())
	}
}
