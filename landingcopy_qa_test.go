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
	"fmt"
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
	"**In the shared checkout, never `--amend`, `rebase` or `reset`.**",
	"an amend rebuilds whatever HEAD is NOW",
	"a pathspec governs what is ADDED from the working tree, never what the base tree already holds",
	"`git restore --source=HEAD --staged --worktree -- <those paths>`, never `git reset --hard`",
	"**Commit everything you want kept.** Only commits move",
	"`bd sync`, so `.beads/issues.jsonl` matches the database",
	"Never push",
	"The operator pushes and the launcher merges.",
	"Every persona's PID denies `Bash(git push:*)`",
	"**Enforced, not advised**",
	"every crew PID denies `Bash(pkill:*)` and `Bash(killall:*)` beside `Bash(git push:*)`",
	"`kill`, `kill -0` and `pgrep` still run",
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
	// The alternation is assembled, not spelled: a bare instance name in
	// source is itself what internal/posse's seed-surface count rejects.
	beadCite := regexp.MustCompile(`\b(?:ranger-base|ranger` + `hq)-[a-z0-9]+\b`)
	if m := beadCite.FindAllString(installed, -1); len(m) > 0 {
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

// ---------------------------------------------------------------------------
// Bullet parity (ranger-base-69nqp, verifying ranger-base-wnsf).
//
// sharedLandingClaims above is a hand-maintained list of phrases, so it catches
// a REWORDING of a claim it already names and nothing else. The drift that
// produced this bead was an ADDITION: AGENTS.md gained the "Know which tree you
// are in" bullet under ranger-base-8zhr, §9's copy did not, and no arm was red
// — MEASURED here by inserting a twelfth bullet into AGENTS.md at a bullet
// boundary and re-running the package: green. The next such addition would go
// the same way, because adding a bullet without adding a claim string is one
// edit and nobody is reminded to make the second.
//
// So this pass compares the two copies by BULLET rather than by phrase: every
// top-level bullet of AGENTS.md's section is either present in the section §9
// leaves a cold installer, or registered below as a deliberate drop with a why.
// The register's stale half is as loud as an unregistered bullet: a row that
// matches no bullet, or that pardons a bullet §9 actually carries, is a fault.

// landingKeyLen is how much of a flattened bullet is compared. Long enough to
// separate every bullet in either file, short enough that the two copies'
// documented divergences (a bead cite, this repo's own checkout path) fall
// outside it.
const landingKeyLen = 40

// landingBeadCite matches this shop's bead ids together with the space in front
// of them, so stripping one closes the gap it leaves: AGENTS.md's
// "two steps** (<id>): `git add" and §9's "two steps**: `git add" become the
// same key. The instance name is assembled, not spelled — a bare one in a root
// *_test.go is what internal/posse's seed-surface count rejects.
var landingBeadCite = regexp.MustCompile(`\s*\((?:ranger-base|ranger` + `hq)-[a-z0-9]+\)`)

// landingBullet is one top-level bullet: Full is its whole flattened text, Key
// the leading landingKeyLen bytes of it. The register below matches on Full — a
// row has to be able to name a bullet by something further in than the key —
// while presence in the other copy is compared on Key.
type landingBullet struct {
	Full string
	Key  string
}

// landingBullets splits a "Landing the plane" section body into its top-level
// bullets, in order. A bullet runs from a `- ` line to the next `- ` line, blank
// line, or end of section; continuation lines are folded in. It takes the
// section text rather than a path, so the same scanner grades the live files and
// the hand-typed fixtures below.
func landingBullets(section string) []landingBullet {
	var out []landingBullet
	var cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		full := flattenLanding(landingBeadCite.ReplaceAllString(strings.Join(cur, " "), ""))
		k := full
		if len(k) > landingKeyLen {
			k = k[:landingKeyLen]
		}
		out = append(out, landingBullet{Full: full, Key: k})
		cur = nil
	}
	for _, line := range strings.Split(section, "\n") {
		switch {
		case strings.HasPrefix(line, "- "):
			flush()
			cur = []string{strings.TrimPrefix(line, "- ")}
		case len(cur) > 0 && strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "":
			cur = append(cur, strings.TrimSpace(line))
		default:
			flush()
		}
	}
	flush()
	return out
}

// landingDrop is one registered divergence: a bullet AGENTS.md carries that §9's
// copy deliberately does not. match is a distinctive substring of the bullet —
// it must name exactly one, and that one must really be absent from §9.
type landingDrop struct {
	match string
	why   string
}

// landingDropRegister is INSTALL.md's own stated exclusion list, in code. The
// prose after §9's recipe says the copy drops "this repo's bead ids, and its own
// checkout path" plus "the bullets that are only about this repo — bd's
// `pre-commit` flush, the `docs/notes.d/` convention, `cmd/checkorphans`". These
// are those three bullets.
var landingDropRegister = []landingDrop{
	{
		match: "after a clean commit is not work",
		why:   "the pre-commit flush of the queue database: a fresh work repo has no such hook and no beads file to read MM over",
	},
	{
		match: "is this shop's provenance",
		why:   "the docs/notes.d convention and this repo's own commit census — neither exists in a fresh work repo",
	},
	{
		match: "background process actually died",
		why:   "ends in `go run ./cmd/checkorphans`, a command that lives only in this repo",
	},
	{
		match: "The full suite: `make test`, never a bare",
		why:   "the box-wide suite queue is scripts/suite-lock.sh, its self-test and its census — three files that live only in this repo, and a fresh work repo has neither them nor a `make test` to hang them off (ranger-base-uvzjk)",
	},
}

// landingParityFaults reports, for one pair of sections, the AGENTS.md bullets
// §9's copy does not carry and is not registered as dropping, and separately the
// register rows that have gone stale. Both halves are returned so a caller can
// say which happened.
func landingParityFaults(agentsSec, installSec string, register []landingDrop) (unregistered, registerFaults []string) {
	agents := landingBullets(agentsSec)
	installFlat := flattenLanding(landingBeadCite.ReplaceAllString(installSec, ""))
	carried := func(key string) bool { return strings.Contains(installFlat, key) }

	dropped := make(map[string]string) // key -> why
	for _, d := range register {
		var hits []string
		for _, b := range agents {
			if strings.Contains(b.Full, d.match) {
				hits = append(hits, b.Key)
			}
		}
		switch {
		case len(hits) == 0:
			registerFaults = append(registerFaults, fmt.Sprintf("register row %q pardons no bullet — the bullet left or was reworded, so the row now hides whatever takes its place", d.match))
		case len(hits) > 1:
			registerFaults = append(registerFaults, fmt.Sprintf("register row %q matches %d bullets %q — it must name exactly one", d.match, len(hits), hits))
		case carried(hits[0]):
			registerFaults = append(registerFaults, fmt.Sprintf("register row %q says the copy drops %q, but the copy carries it", d.match, hits[0]))
		default:
			dropped[hits[0]] = d.why
		}
	}
	for _, b := range agents {
		if _, ok := dropped[b.Key]; ok {
			continue
		}
		if !carried(b.Key) {
			unregistered = append(unregistered, b.Key)
		}
	}
	return unregistered, registerFaults
}

// TestLandingBulletParityScannerDiscriminates is the control: the scanner has to
// report an added bullet, and has to stay quiet on a pair that agrees. Without
// it the pin below is a green function nobody has seen refuse anything.
func TestLandingBulletParityScannerDiscriminates(t *testing.T) {
	const agreeing = "" +
		"- **One.** The first claim, wrapped\n  over two lines (ranger-base-aaaa).\n" +
		"- **Two.** The second claim.\n" +
		"- **A bullet whose first forty bytes say nothing\n  distinctive at all:** run `cmd/checkorphans`.\n"
	const copyOf = "" +
		"- **One.** The first claim, wrapped over\n  two lines.\n" +
		"- **Two.** The second claim.\n"
	// The row matches past landingKeyLen on purpose: a register keyed on the
	// truncated key instead of the whole bullet is red here.
	register := []landingDrop{{match: "run `cmd/checkorphans`", why: "names a command only this repo has"}}

	if un, rf := landingParityFaults(agreeing, copyOf, register); len(un) > 0 || len(rf) > 0 {
		t.Errorf("scanner fires on a pair that agrees — it would be red forever: unregistered=%q registerFaults=%q", un, rf)
	}

	// (a) A bullet added to the original and not to the copy is the shape that
	// produced this bead. It must be reported.
	added := agreeing + "- **Three.** A claim the copy never got.\n"
	un, _ := landingParityFaults(added, copyOf, register)
	if len(un) != 1 || !strings.Contains(un[0], "**Three.**") {
		t.Errorf("scanner does not report a bullet added to the original only: %q", un)
	}

	// (b) The register's stale half. A row that pardons nothing is a row that
	// will silently pardon the next bullet worded like it.
	_, rf := landingParityFaults(agreeing, copyOf, []landingDrop{{match: "no such bullet", why: "x"}})
	if len(rf) != 1 || !strings.Contains(rf[0], "pardons no bullet") {
		t.Errorf("scanner does not report a register row matching no bullet: %q", rf)
	}

	// (c) And a row that pardons a bullet the copy does carry.
	_, rf = landingParityFaults(agreeing, copyOf, []landingDrop{{match: "**Two.**", why: "x"}})
	if len(rf) != 1 || !strings.Contains(rf[0], "but the copy carries it") {
		t.Errorf("scanner does not report a register row that pardons a carried bullet: %q", rf)
	}

	// (d) The key really is a key: two bullets must not collapse into one.
	if got := landingBullets(agreeing); len(got) != 3 {
		t.Errorf("scanner read %d bullets from a 3-bullet section: %q", len(got), got)
	}
}

// TestInstallSection9CarriesEveryAgentsMdLandingBullet is the addition pin.
// sharedLandingClaims catches a claim being reworded; this catches one being
// ADDED to AGENTS.md and never reaching §9 — which is how the copy went stale
// the first time (ranger-base-wnsf). A new bullet is either copied into §9's
// heredoc or registered above with the reason a fresh work repo cannot resolve
// it.
func TestInstallSection9CarriesEveryAgentsMdLandingBullet(t *testing.T) {
	agentsSec := landingSection(t, "AGENTS.md", readRepoFile(t, "AGENTS.md"))
	installSec := landingSection(t, "INSTALL.md §9's result", runSection9Recipe(t, bdPlantedAgentsMd))

	// Floors: an emptied or unparsed section must red rather than pass at zero.
	if n := len(landingBullets(agentsSec)); n < 8 {
		t.Fatalf("AGENTS.md's Landing section parsed as %d bullets — the scanner has stopped reading its subject", n)
	}
	if n := len(landingBullets(installSec)); n < 5 {
		t.Fatalf("§9's appended section parsed as %d bullets — the scanner has stopped reading its subject", n)
	}

	unregistered, registerFaults := landingParityFaults(agentsSec, installSec, landingDropRegister)
	for _, k := range unregistered {
		t.Errorf("AGENTS.md's Landing section has a bullet §9's copy does not carry (ranger-base-69nqp): %q\n"+
			"    copy it into INSTALL.md §9's heredoc, or add it to landingDropRegister with the reason a fresh work repo cannot resolve it", k)
	}
	for _, f := range registerFaults {
		t.Errorf("landingDropRegister has gone stale (ranger-base-69nqp): %s", f)
	}
}
