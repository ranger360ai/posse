package posse

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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	for i, line := range strings.Split(doc, "\n") {
		if !strings.Contains(strings.ToLower(line), "push") {
			continue
		}
		if strings.Contains(line, "Never push") || strings.Contains(line, "operator pushes") ||
			strings.Contains(line, "pre-push") || strings.Contains(line, "git push:*") {
			continue
		}
		t.Errorf("AGENTS.md:%d mentions push without saying the operator pushes: %q", i+1, line)
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
		"**Never push. The operator pushes.**",
		`grep -n -i "push" AGENTS.md`,
		"Every surviving mention must say the operator pushes",
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("INSTALL.md §9 no longer prescribes the reconciliation: missing %q (rangerhq-cmfj)", want)
		}
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
	if !strings.Contains(got, "**Never push. The operator pushes.**") {
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
	if !strings.Contains(got, "**Never push. The operator pushes.**") {
		t.Errorf("§9's recipe appended nothing when there was nothing to cut:\n%s", got)
	}
	if twice := runSection9Recipe(t, got); strings.Count(twice, "**Never push. The operator pushes.**") != 2 {
		// Not a defect — §9 says to run it once — but if the append ever
		// starts eating its own output, this is where it shows.
		t.Errorf("§9's recipe is not additive on a second run:\n%s", twice)
	}
}
