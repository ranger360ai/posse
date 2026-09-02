package posse

// QA pins for ranger-base-wnsf.
//
// Claim: INSTALL.md §9's heredoc and this repo's AGENTS.md are two copies of
// one "Landing the plane" section, and nothing kept them in step. §9's copy
// was written before per-session worktrees existed: it had no "know which
// tree you are in" bullet, and it gave "every persona shares this checkout
// and its index" as the ONLY reason to name your paths in a commit — which a
// session in its own worktree reads as "not me", and then has its commit
// refused anyway. The reason that holds in every tree is the PID deny
// `Bash(git commit unless --)`, realized as a PATH shim that reads argv and
// never the tree (ranger-base-5xv1, measured; AGENTS.md was corrected under
// ranger-base-8zhr). So running §9's documented recipe over an AGENTS.md that
// had already been reconciled would DELETE the current block and reinstate
// the older, worktree-blind one.
//
// §9's copy cannot simply be dropped: it is what a cold installer appends to
// a fresh work repo's AGENTS.md, and that reader has no this-repo bead ids,
// no `~/src/posse`, no `docs/notes.d/` and no `cmd/checkorphans` to resolve.
// So the copy stays, minus those, and this file is what stops it drifting:
// every shared claim has to be readable in BOTH files.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// readRepoFile reads a file from the repo root, where these doc pins run.
func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// sharedLandingClaims is the substance the two copies must agree on — the
// bullets a persona acts on, not the prose around them. Each entry is matched
// against whitespace-collapsed text, so a rewrap is not a failure and a
// reworded claim is.
var sharedLandingClaims = []string{
	"**Know which tree you are in.**",
	"worktree of this repo (under `~/.posse/worktrees/`), on a branch `posse/<session>`",
	"its own index, its own HEAD, nobody else's",
	"commit **naming your own paths** (`git commit -F - -- <paths>`)",
	"That form is unconditional: every crew PID carries `deny: Bash(git commit unless --)`",
	"a PID-level deny realized as a PATH shim that reads argv and never the tree, so it refuses an unqualified commit in your own worktree too",
	"in a session worktree nothing is shared and that gate stands down — the PID does not",
	"**A NEW file needs two steps**",
	"did not match any file(s) known to git",
	"never `git add -A` or `git add .`",
	"**In the shared checkout a revert is two steps**",
	"`git restore --source=HEAD --staged --worktree -- <those paths>`, never `git reset --hard`",
	"**Commit everything you want kept.** Only commits move",
	"`bd sync`, so `.beads/issues.jsonl` matches the database",
	"Never push",
	"The operator pushes and the launcher merges.",
	"Every persona's PID denies `Bash(git push:*)`",
}

var landingWS = regexp.MustCompile(`\s+`)

// flattenLanding collapses every run of whitespace to one space, so the two
// copies can be wrapped to different widths and still be compared.
func flattenLanding(s string) string {
	return strings.TrimSpace(landingWS.ReplaceAllString(s, " "))
}

// missingLandingClaims names the shared claims a section does not make.
func missingLandingClaims(section string) []string {
	flat := flattenLanding(section)
	var missing []string
	for _, c := range sharedLandingClaims {
		if !strings.Contains(flat, flattenLanding(c)) {
			missing = append(missing, c)
		}
	}
	return missing
}

// landingSection returns the "## Landing the plane" section of doc, from the
// heading to the next `## ` heading or EOF. Both copies spell the heading in
// lowercase; bd's planted one is "## Landing the Plane (Session Completion)"
// and is deliberately NOT matched.
func landingSection(t *testing.T, what, doc string) string {
	t.Helper()
	const head = "## Landing the plane\n"
	i := strings.Index(doc, head)
	if i < 0 {
		t.Fatalf("%s: no %q section — the pin has stopped reading its subject", what, head)
	}
	rest := doc[i+len(head):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j+1]
	}
	return rest
}

// staleLandingCopy is §9's heredoc as it stood before this bead — the arm
// that has to fail. Without it the checker below would pass on the defect.
const staleLandingCopy = "" +
	"- Close the bead, and commit **naming your own paths** (`git commit -F - --\n" +
	"  <paths>`) — every persona shares this checkout and its index, so an\n" +
	"  unqualified commit takes whatever another persona has staged.\n" +
	"- **A new file needs two steps here** — `git add -- <the new paths>`, then\n" +
	"  `git commit -F - -- <all your paths>`. A pathspec only matches a file git\n" +
	"  already has an index entry for, so the path-limited form alone answers\n" +
	"  `did not match any file(s) known to git`. Scope that add with `--`; never\n" +
	"  `git add -A` or `git add .`, which stage every persona's file into the\n" +
	"  shared index.\n" +
	"- `bd sync`, so `.beads/issues.jsonl` matches the database.\n" +
	"- **Never push. The operator pushes.** Every persona's PID denies\n" +
	"  `Bash(git push:*)` and this repo's `pre-push` gate refuses it, so a push\n" +
	"  is a refused turn, not a landing. Work is complete when it is committed\n" +
	"  locally and the bead is closed.\n"

// TestLandingClaimCheckerDiscriminates is the control. The stale copy is
// green on the claims it always made and red on exactly the ones the bead is
// about — if this ever reports nothing missing, the checker has stopped
// measuring and the pin below is decoration.
func TestLandingClaimCheckerDiscriminates(t *testing.T) {
	missing := missingLandingClaims(staleLandingCopy)
	if len(missing) == 0 {
		t.Fatal("checker finds nothing missing in §9's pre-fix heredoc — it would pass on the bug (ranger-base-wnsf)")
	}
	// The defect the bead names, not merely "something differs".
	for _, want := range []string{
		"**Know which tree you are in.**",
		"That form is unconditional: every crew PID carries `deny: Bash(git commit unless --)`",
	} {
		found := false
		for _, m := range missing {
			if m == want {
				found = true
			}
		}
		if !found {
			t.Errorf("checker does not notice the stale copy is missing %q", want)
		}
	}
	// And it is not simply red on everything: the stale copy really did
	// carry these, so a checker that flagged them would be measuring nothing.
	flat := flattenLanding(staleLandingCopy)
	for _, had := range []string{"did not match any file(s) known to git", "Never push"} {
		if !strings.Contains(flat, had) {
			t.Errorf("control fixture is wrong: the pre-fix heredoc did carry %q", had)
		}
	}
}

// TestAgentsMdLandingSectionMakesTheSharedClaims reads the original. If this
// is red, AGENTS.md moved and INSTALL.md §9 has to move with it — decide the
// wording here first, then bring §9 to it.
func TestAgentsMdLandingSectionMakesTheSharedClaims(t *testing.T) {
	sec := landingSection(t, "AGENTS.md", readRepoFile(t, "AGENTS.md"))
	if missing := missingLandingClaims(sec); len(missing) > 0 {
		t.Errorf("AGENTS.md's Landing section no longer makes these claims (ranger-base-wnsf): %q", missing)
	}
}

// TestInstallSection9AndAgentsMdAgreeOnTheLandingClaims is the drift pin: it
// runs §9's recipe rather than reading it, and holds the section a cold
// installer is actually left with against the original.
func TestInstallSection9AndAgentsMdAgreeOnTheLandingClaims(t *testing.T) {
	installed := landingSection(t, "INSTALL.md §9's result", runSection9Recipe(t, bdPlantedAgentsMd))

	if missing := missingLandingClaims(installed); len(missing) > 0 {
		t.Errorf("INSTALL.md §9 appends a section that has drifted from AGENTS.md — missing (ranger-base-wnsf): %q\n---\n%s", missing, installed)
	}

	// The two documented exclusions, pinned so the prose stays true: the copy
	// resolves in a fresh work repo, which has none of these.
	if m := regexp.MustCompile(`\b(?:rangerhq|ranger-base)-[a-z0-9]+\b`).FindAllString(installed, -1); len(m) > 0 {
		t.Errorf("§9's appended section cites this repo's bead ids, which a fresh work repo cannot resolve: %q", m)
	}
	for _, unresolvable := range []string{"~/src/posse", "docs/notes.d", "cmd/checkorphans", "NOTES.md"} {
		if strings.Contains(installed, unresolvable) {
			t.Errorf("§9's appended section names %q, which exists only in this repo", unresolvable)
		}
	}

	// And the recipe still cut what it exists to cut.
	if left := readerDirectedPushOrders(installed); len(left) > 0 {
		t.Errorf("§9's appended section carries the push mandate: %v", left)
	}
}
