package posse

// THE COMMIT PATH HAS NO ADR CITATION CHECK, and these pins are what stops
// one growing back (ADR 0051 as simplified by operator ruling 2026-09-05,
// ranger-base-bp0yj).
//
// WHAT WAS REMOVED. A fourth arm of the prepare-commit-msg hook rendered
// ADR 0051's predicate over the ADDED lines of every staged docs/adr file:
// each 7–40 hex token that resolved in the clone and was not an ancestor of
// the main checkout's branch was a refusal, unless a patch-id twin of it sat
// in the same staged blob. The ruling removed it. A citation helps find work
// and does not prove landing, so an editorial verdict was being bought at the
// price of object lookup, ancestry classification and patch-id equivalence on
// every commit that touched an ADR — and ADR 0006, not this, is where landing
// is established.
//
// WHAT SURVIVES: `posse gates adr-census`, asked for. Its pins are in
// adrcensus_qa_test.go and they are unchanged, because the audit's verdicts
// did not change — only who has to walk into them.
//
// A PIN THAT ONLY ASSERTS AN ABSENCE MEASURES NOTHING, so no cell here
// stands alone:
//
//   - every "it commits" cell names a token the AUDIT still refuses, asked
//     in the same fixture in the same test, so the pass is evidence the
//     check MOVED and not evidence it stopped being able to see anything;
//   - the three walls that stayed are exercised in that same repo, so a
//     fixture whose hook silently stopped running — the way this whole
//     family would go green — fatals instead of passing;
//   - the render assertion is two-way: the citation machinery is gone AND
//     the arms that stayed are still in the bytes.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// adrCitationRefusalMarks are the words the removed arm's refusal carried.
// None of them may appear in a commit's output ever again, and none of them
// may appear in the rendered hook. They are kept verbatim rather than
// paraphrased so that a re-added arm which reuses the old prose — the likely
// shape of a revert — is caught by the words it would actually print.
var adrCitationRefusalMarks = []string{
	"refused by posse gate: a staged docs/adr line names a sha that is not on",
	"has no landed twin in the record",
	"the citation of record for a landed change is the BEAD ID",
	"put the landed twin beside it in the same file",
	"ADR sha-stamp guard [prepare-commit-msg hook]",
	"ADR sha-stamp check judged nothing",
}

// adrOtherArmMarks are the three walls that STAYED. A cell that refuses with
// one of these has measured the wrong wall; a control that does NOT refuse
// with one of these has measured a hook that is not running.
var adrOtherArmMarks = []string{
	"This working tree's .git/index is shared",
	"a persona commit touching the constitution",
	"ops-class content",
	"a commit changing NOTES.md in the shared checkout",
}

// assertNoCitationRefusal is the whole verdict for a pass cell: the commit
// landed, and nothing in what the hook printed is the removed arm's or any
// other wall's. "Committed quietly" is the claim, and both halves of it are
// checked — an exit 0 with a refusal-shaped warning on stderr would be a
// gate that lost only its exit code.
func assertNoCitationRefusal(t *testing.T, what, out string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s must commit — the commit path has no citation check (ADR 0051, ranger-base-bp0yj): %v\n%s", what, err, out)
	}
	for _, gone := range adrCitationRefusalMarks {
		if strings.Contains(out, gone) {
			t.Errorf("%s: the removed ADR citation arm is back on the commit path — %q is in the output:\n%s", what, gone, out)
		}
	}
	for _, wrong := range adrOtherArmMarks {
		if strings.Contains(out, wrong) {
			t.Fatalf("%s: refused by a DIFFERENT wall (%q) — this cell is measuring the wrong thing:\n%s", what, wrong, out)
		}
	}
}

// TestQAAdrCitationHasNoCommitTimeGate is the removal, measured through the
// real installed hook and real git over every citation shape the old arm
// told apart — including the two the bead names by hand, an equivalent-patch
// case and an absent object. Each cell commits; each cell's token is then
// put to `posse gates adr-census`, which is where the same question is still
// answered.
func TestQAAdrCitationHasNoCommitTimeGate(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	// A 40-hex token that resolves to NO object here. The old arm passed
	// this one too (it could not judge it), and the reason it must be a cell
	// is the bead's: "absent object" has to leave the commit path without
	// costing a lookup, in a clone whose objects were pruned as much as in
	// one where they never existed.
	const absent = "0123456789abcdef0123456789abcdef01234567"
	if _, err := r.git(nil, "cat-file", "-e", absent+"^{commit}"); err == nil {
		t.Fatalf("fixture: %s must resolve to nothing in this repo", absent)
	}

	for _, c := range []struct {
		name    string
		tok     func(*adrRepo) string
		audited bool // the audit still REFUSES this token, asked directly
	}{
		{"a stale sha with no twin anywhere", func(r *adrRepo) string { return r.stale }, true},
		{"the same stale sha in capitals", func(r *adrRepo) string { return strings.ToUpper(r.stale) }, true},
		{"a session sha whose patch has an equivalent on the base", func(r *adrRepo) string { return r.twinStale }, true},
		{"a non-ancestor with an empty patch-id", func(r *adrRepo) string { return r.emptyStale }, true},
		{"an object absent from this store", func(r *adrRepo) string { return absent }, false},
		{"a landed sha", func(r *adrRepo) string { return r.landed }, false},
	} {
		rel := "docs/adr/0999-" + strings.ReplaceAll(c.name, " ", "-") + ".md"
		tok := c.tok(r)

		r.stage(t, rel, adrStamp(tok))
		out, err := r.commitPath(t, rel)
		assertNoCitationRefusal(t, c.name+" ("+tok+")", out, err)

		// And the audit, over the same file, still says what it always said.
		// This is what makes the cell above evidence: the shape is not
		// invisible now, it is only unpoliced at the wall.
		census, _, refused := r.census(t, rel)
		if refused != c.audited {
			t.Errorf("%s: `posse gates adr-census` must %s this record — the audit is what ADR 0051 retained:\n%s",
				c.name, map[bool]string{true: "refuse", false: "pass"}[c.audited], census)
		}
	}
}

// The three walls that stayed, in the same repo and through the same hook.
// Without this the whole file is consistent with a fixture whose hook never
// ran — which is exactly how a suite goes green over a deleted gate.
func TestQAAdrCitationRemovalLeftTheOtherWallsStanding(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)

	// The shared-index wall: an unqualified commit, the form that takes
	// another persona's staged work. Deliberately NOT a docs/adr path and
	// deliberately not a sha: this control has to answer for the walls that
	// stayed even in a tree where the removed arm is somehow back, or a
	// revert reds it too and the reader cannot tell the two claims apart.
	r.stage(t, "README.md", "# fixture\n")
	out, err := r.git(r.persona, "commit", "-m", "sweep")
	if err == nil {
		t.Fatalf("control: an unqualified commit must still be refused:\n%s", out)
	}
	if !strings.Contains(out, "This working tree's .git/index is shared") {
		t.Errorf("control: the shared-index wall must be the one that refused:\n%s", out)
	}

	// The constitution wall: a persona commit touching the settings file that
	// fences the persona's own destructive verbs (ADR 0015 §2/§3).
	const settings = ".claude/settings.json"
	r.stage(t, settings, "{}\n")
	out, err = r.git(r.persona, "commit", "-m", "settings", "--", settings)
	if err == nil {
		t.Fatalf("control: a persona commit touching %s must still be refused:\n%s", settings, out)
	}
	if !strings.Contains(out, "a persona commit touching the constitution") {
		t.Errorf("control: the constitution wall must be the one that refused:\n%s", out)
	}
}

// The production shape the removed arm was built for, end to end: a session
// worktree cut from the base branch, a commit made in it, and that commit's
// own sha — the only sha a persona can read, and an ancestor of the
// worktree's HEAD and of nothing else — pasted into an ADR. This is the cell
// that used to refuse, and it is the one the ruling is about.
func TestQAAdrCitationOfTheWritersOwnShaCommits(t *testing.T) {
	t.Parallel()
	r := newAdrRepo(t)
	wt := filepath.Join(t.TempDir(), "session")
	if out, err := r.git(nil, "worktree", "add", "-q", "-b", "sess", wt, "main"); err != nil {
		t.Skipf("git worktree add unavailable here: %v %s", err, out)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + r.dir,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	wtgit := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	adrStage(t, wt, wtgit, r.persona, "work.txt", "work\n")
	if out, err := wtgit(r.persona, "commit", "-m", "work (ranger-base-bp0yj)", "--", "work.txt"); err != nil {
		t.Fatalf("a path-limited commit in a session worktree must land: %v\n%s", err, out)
	}
	mineOut, err := wtgit(nil, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse in the worktree: %v %s", err, mineOut)
	}
	mine := strings.TrimSpace(mineOut)
	if _, err := r.git(nil, "merge-base", "--is-ancestor", mine, "refs/heads/main"); err == nil {
		t.Fatalf("fixture: the session commit %s must not be on main yet — that is the whole shape", mine)
	}

	adrStage(t, wt, wtgit, r.persona, "docs/adr/0999-fixture.md", adrStamp(mine))
	out, err := wtgit(r.persona, "commit", "-m", "adr", "--", "docs/adr/0999-fixture.md")
	assertNoCitationRefusal(t, "a session sha in an ADR, from the session worktree", out, err)

	// Nothing was written to the refusals log either — the removed arm's own
	// line is how an operator would still see it running.
	log, _ := os.ReadFile(filepath.Join(r.gates, "refusals.log"))
	if strings.Contains(string(log), "ADR sha-stamp guard") {
		t.Errorf("the removed arm logged a refusal to refusals.log:\n%s", string(log))
	}
}

// The render, both ways: none of the citation machinery is in the hook's
// bytes, and the three walls that stayed still are. A one-way pin here goes
// green against a hook that renders nothing at all.
func TestQAAdrCitationMachineryIsNotInTheRenderedHook(t *testing.T) {
	t.Parallel()
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})

	for _, gone := range append([]string{
		"posse_adr_judge",
		"posse_adr_files",
		"posse_adr_refused",
		`git cat-file -e "$posse_adr_t^{commit}"`,
		`git merge-base --is-ancestor "$posse_adr_t" "$posse_adr_base"`,
		"git patch-id --stable",
		":(icase)docs/adr/*",
	}, adrCitationRefusalMarks...) {
		if strings.Contains(render, gone) {
			t.Errorf("the ADR citation gate is back in the rendered prepare-commit-msg hook: %q\n"+
				"ADR 0051's check is `posse gates adr-census`, asked for — not a commit-time refusal (ranger-base-bp0yj)", gone)
		}
	}
	// The predicate itself is retained for the audit and must be reachable
	// from exactly one renderer. If it is ever in the hook again, the line
	// above catches it; this is the other half — it must still be in the
	// census, or the removal took the audit with it.
	if !strings.Contains(AdrCensusScript(), "posse_adr_judge") {
		t.Error("the audit lost the predicate — ADR 0051 retains `posse gates adr-census`")
	}
	// And the walls that stayed are still rendered. Without this, every
	// assertion above is satisfied by a hook that renders an empty string.
	for _, want := range []string{
		"This working tree's .git/index is shared",
		"a persona commit touching the constitution",
		"ops-class content",
	} {
		if !strings.Contains(render, want) {
			t.Errorf("a wall ADR 0051 did not touch is missing from the render: %q", want)
		}
	}
}
