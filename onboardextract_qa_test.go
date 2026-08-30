package posse

// QA pins for ranger-base-3fhb.
//
// Claim: INSTALL.md §9 extracts `bd onboard`'s snippet by cutting between two
// prose delimiters, and that cut is silent on any bd whose output is not
// shaped like bd 0.49.1's. The step had no **Verify:**, which every other
// block in §9 has and whose own rule is "If a Verify fails, do not continue".
//
// MEASURED, 2026-08-30, running the §9 pipeline verbatim over stand-in
// `bd onboard` outputs (bd 0.49.1 0d99d153, macOS 25.4.0):
//
//	both markers as bd 0.49.1 prints -> the snippet, and nothing else
//	both markers renamed             -> 0 lines appended, exit 0, no message
//	END renamed, BEGIN kept          -> `sed` runs to EOF and `$d` drops only
//	                                    the last line, so the Copilot prose
//	                                    lands in AGENTS.md behind the snippet
//
// Neither failing arm is reachable on bd 0.49.1; both are reachable on the
// next bd that reformats `bd onboard`, and the step is written as a permanent
// instruction. §9 now carries a Verify keyed on the delimiter count in bd's
// own output — `2`, `0`, `1` respectively — and this file is what says the
// Verify still discriminates, by running it.
//
// The check §9 explicitly forbids is pinned too: `grep -c "bd ready"` over
// AGENTS.md cannot tell a correct paste from either failure, because the
// AGENTS.md `bd init` plants already contains that line.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bdOnboardOutput is what bd 0.49.1 prints, byte-for-byte — the fixture the
// recipe is written against, and the baseline the failing arms mutate.
const bdOnboardOutput = "\nbd Onboarding\n\nAdd this minimal snippet to AGENTS.md (or create it):\n\n" +
	"--- BEGIN AGENTS.MD CONTENT ---\n" +
	"## Issue Tracking\n\nThis project uses **bd (beads)** for issue tracking.\n" +
	"Run `bd prime` for workflow context, or install hooks (`bd hooks install`) for auto-injection.\n" +
	"\n**Quick reference:**\n" +
	"- `bd ready` - Find unblocked work\n" +
	"- `bd create \"Title\" --type task --priority 2` - Create issue\n" +
	"- `bd close <id>` - Complete work\n" +
	"- `bd sync` - Sync with git (run at session end)\n" +
	"\nFor full workflow details: `bd prime`\n" +
	"--- END AGENTS.MD CONTENT ---\n" +
	"\nFor GitHub Copilot users:\nAdd the same content to .github/copilot-instructions.md\n" +
	"\nHow it works:\n" +
	"   • bd prime provides dynamic workflow context (~80 lines)\n" +
	"   • bd hooks install auto-injects bd prime at session start\n" +
	"   • AGENTS.md only needs this minimal pointer, not full instructions\n" +
	"\nThis keeps AGENTS.md lean while bd prime provides up-to-date workflow details.\n\n"

// snippetLastLine is the last line of the region bd 0.49.1 delimits — what
// §9's Verify tells the installer to expect out of `tail -n 1 AGENTS.md`.
const snippetLastLine = "For full workflow details: `bd prime`"

// onboardStandIns are the three `bd onboard` shapes the pipeline meets: the
// one it was written for, and the two the Verify exists to catch.
var (
	onboardBothMarkersRenamed = strings.ReplaceAll(bdOnboardOutput, "AGENTS.MD CONTENT", "AGENTS_MD_SNIPPET")
	onboardEndMarkerRenamed   = strings.ReplaceAll(bdOnboardOutput,
		"--- END AGENTS.MD CONTENT ---", "--- END AGENTS_MD_SNIPPET ---")
)

// installOnboardBlock pulls §9's `bd onboard` block — the extraction pipeline
// and the two Verify commands under it — back out of the doc, so what runs
// below is the text a cold installer copies rather than a paraphrase of it.
func installOnboardBlock(t *testing.T) []string {
	t.Helper()
	lines := strings.Split(installSection9(t), "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "$ bd onboard | sed -n ") {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("INSTALL.md §9: the bd onboard extraction is gone — the pin has stopped reading its subject")
	}
	var block []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(l, "```") {
			return block
		}
		block = append(block, strings.TrimPrefix(l, "$ "))
	}
	t.Fatal("INSTALL.md §9: the bd onboard block has no closing fence")
	return nil
}

// runOnboardBlock runs that block over one stand-in `bd onboard` output,
// against the AGENTS.md `bd init` plants, and returns what the installer sees
// on stdout and what the file is left holding. Only the command name is
// rewritten: every pipe, pattern and redirection is the doc's.
func runOnboardBlock(t *testing.T, onboard string) (stdout, agents string) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "onboard.txt"), []byte(onboard), 0o644); err != nil {
		t.Fatalf("write stand-in: %v", err)
	}
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(bdPlantedAgentsMd), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	script := strings.Join(installOnboardBlock(t), "\n")
	if !strings.Contains(script, "bd onboard") {
		t.Fatalf("INSTALL.md §9: the block no longer runs `bd onboard`:\n%s", script)
	}
	script = strings.ReplaceAll(script, "bd onboard", "cat onboard.txt")
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// The defect under pin is that the pipeline is silent, not that it
		// fails — a non-zero exit here means the recipe changed shape.
		t.Fatalf("§9's bd onboard block failed: %v\n%s", err, out)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	return string(out), string(b)
}

// verifyCount reads the delimiter count §9's Verify prints — the first line of
// the block's output, which is the `grep -c` the installer is told to read.
func verifyCount(t *testing.T, stdout string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("INSTALL.md §9's Verify prints fewer than the two values it documents:\n%q", stdout)
	}
	return lines[0]
}

func verifyTail(t *testing.T, stdout string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	return lines[len(lines)-1]
}

// TestOnboardExtractionLandsTheSnippetOnBd0491 is the arm the recipe was
// written for: it must still work, and the Verify must still say so.
func TestOnboardExtractionLandsTheSnippetOnBd0491(t *testing.T) {
	stdout, agents := runOnboardBlock(t, bdOnboardOutput)

	if got := verifyCount(t, stdout); got != "2" {
		t.Errorf("§9's Verify prints %q on bd 0.49.1's own output, not the `2` it documents", got)
	}
	if got := verifyTail(t, stdout); got != snippetLastLine {
		t.Errorf("§9's Verify tails %q, not the snippet's last line %q", got, snippetLastLine)
	}
	for _, want := range []string{"## Issue Tracking", "- `bd ready` - Find unblocked work", snippetLastLine} {
		if !strings.Contains(agents, want) {
			t.Errorf("the extraction dropped %q out of the delimited region:\n%s", want, agents)
		}
	}
	// The prose the step exists to keep out, and the delimiters themselves.
	for _, unwanted := range []string{
		"bd Onboarding", "Add this minimal snippet", "For GitHub Copilot users",
		"How it works:", "AGENTS.MD CONTENT",
	} {
		if strings.Contains(agents, unwanted) {
			t.Errorf("the extraction let %q into AGENTS.md:\n%s", unwanted, agents)
		}
	}
}

// TestOnboardExtractionIsASilentNoOpWhenNoMarkerMatches is arm (A). The
// pipeline appends nothing and says nothing; the Verify is the only thing
// between the installer and an AGENTS.md with no queue pointer in it.
func TestOnboardExtractionIsASilentNoOpWhenNoMarkerMatches(t *testing.T) {
	stdout, agents := runOnboardBlock(t, onboardBothMarkersRenamed)

	if agents != bdPlantedAgentsMd {
		t.Errorf("arm (A) is no longer a no-op — the fixture the Verify is calibrated on has moved:\n%s", agents)
	}
	if strings.Contains(agents, "## Issue Tracking") {
		t.Error("arm (A) appended a queue pointer, which no marker could have delimited")
	}
	if got := verifyCount(t, stdout); got != "0" {
		t.Errorf("§9's Verify prints %q on an output with neither marker — it must print `0`, "+
			"or the silent no-op passes the check (ranger-base-3fhb)", got)
	}
}

// TestOnboardExtractionLeaksProseWhenOnlyOneMarkerMatches is arm (B): the
// failure rangerhq-5ofl was filed about, re-armed by a one-marker change.
// `sed` prints to EOF and `$d` drops only the last line.
func TestOnboardExtractionLeaksProseWhenOnlyOneMarkerMatches(t *testing.T) {
	stdout, agents := runOnboardBlock(t, onboardEndMarkerRenamed)

	leaked := false
	for _, prose := range []string{"For GitHub Copilot users", "How it works:", "--- END AGENTS_MD_SNIPPET ---"} {
		if strings.Contains(agents, prose) {
			leaked = true
		}
	}
	if !leaked {
		t.Errorf("arm (B) no longer leaks prose — the fixture the Verify is calibrated on has moved:\n%s", agents)
	}
	if got := verifyCount(t, stdout); got != "1" {
		t.Errorf("§9's Verify prints %q when only one marker matches — it must print `1`, "+
			"or the prose leak passes the check (ranger-base-3fhb)", got)
	}
	if got := verifyTail(t, stdout); got == snippetLastLine {
		t.Error("§9's Verify tails the snippet's last line on arm (B) — it cannot see the prose behind it")
	}
}

// TestSnippetGrepCannotDiscriminate is the control for the sentence §9 adds
// under the Verify. The obvious check — grep the file for a line of the
// snippet — is green on all three arms, so it certifies a paste that never
// happened. Without this, a future edit could "simplify" the Verify back into
// something that measures nothing.
func TestSnippetGrepCannotDiscriminate(t *testing.T) {
	count := func(onboard string) int {
		_, agents := runOnboardBlock(t, onboard)
		return strings.Count(agents, "bd ready")
	}
	good, noOp, leak := count(bdOnboardOutput), count(onboardBothMarkersRenamed), count(onboardEndMarkerRenamed)

	if noOp == 0 {
		t.Error("`grep bd ready` would in fact catch arm (A) — §9's warning against it is now wrong")
	}
	if good != leak {
		t.Errorf("`grep -c bd ready` separates the good paste (%d) from arm (B) (%d) — "+
			"§9's warning against it is now wrong", good, leak)
	}
	if !strings.Contains(bdPlantedAgentsMd, "bd ready") {
		t.Error("bd's planted AGENTS.md no longer contains `bd ready` — the reason §9 gives has moved")
	}
}

// TestInstallSection9NamesTheBdVersionAtTheOnboardStep pins the doc's half of
// the fix: the paragraph describes bd's onboard output as fact, so it has to
// say which bd's — the way §9 already does for `core.hooksPath`.
func TestInstallSection9NamesTheBdVersionAtTheOnboardStep(t *testing.T) {
	sec := installSection9(t)
	i := strings.Index(sec, "Give the repo an `AGENTS.md`")
	j := strings.Index(sec, "**Then reconcile that file")
	if i < 0 || j < 0 || j < i {
		t.Fatal("INSTALL.md §9: the onboard step is not where the pin reads it")
	}
	step := sec[i:j]

	if !strings.Contains(step, "bd 0.49.1") {
		t.Error("INSTALL.md §9's onboard step describes bd's output without naming the bd (ranger-base-3fhb)")
	}
	if !strings.Contains(step, "**Verify:**") {
		t.Error("INSTALL.md §9's onboard step has no Verify, and §9's own rule is " +
			"\"If a Verify fails, do not continue\" (ranger-base-3fhb)")
	}
	if !strings.Contains(step, "`bd ready`") {
		t.Error("INSTALL.md §9's onboard step no longer warns off the grep that cannot discriminate")
	}
}

// TestOnboardVerifyGrepsTheMarkersTheSedCutsOn keeps the check keyed to the
// recipe. If a future edit changes the delimiters in the `sed` and not in the
// `grep -c`, the Verify would count markers the extraction never used and go
// green on a cut that found nothing.
func TestOnboardVerifyGrepsTheMarkersTheSedCutsOn(t *testing.T) {
	block := strings.Join(installOnboardBlock(t), "\n")
	for _, marker := range []string{"--- BEGIN AGENTS.MD CONTENT ---", "--- END AGENTS.MD CONTENT ---"} {
		if n := strings.Count(block, marker); n != 2 {
			t.Errorf("INSTALL.md §9: %q appears %d times in the onboard block; the sed that cuts on it "+
				"and the grep that counts it must both name it (ranger-base-3fhb)", marker, n)
		}
	}
	if !strings.Contains(block, "grep -c") {
		t.Error("INSTALL.md §9's onboard block no longer counts the delimiters (ranger-base-3fhb)")
	}
	if !strings.Contains(block, "tail -n 1 AGENTS.md") {
		t.Error("INSTALL.md §9's onboard block no longer reads back what landed (ranger-base-3fhb)")
	}
}
