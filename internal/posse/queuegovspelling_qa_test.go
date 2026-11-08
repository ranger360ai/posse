//go:build posse_arm3

package posse

// QA pins for ranger-base-qm7eg — ADR 0082 D4 at its THIRD site: the
// `beads_visibility:` key G13's `queue-unmarked` row tells the operator to
// paste is spelled the way the kernel names the directory, however
// `queue_repo:` spelled it.
//
// WHAT WAS WRONG, and why it is the same defect one function over.
// QueueReadiness.GovRows() printed the queue repo twice as
// `AbbrevHome(r.Repo)` — once to name it, once inside the remedy's
// `` `%s: private` `` — and `r.Repo` is `QueueRepo()`'s value, the spelling
// the operator's config carries. So a case-changed or symlinked
// `queue_repo:` made the governance surface prescribe a key in a spelling
// that is not the one the identity wall of that same repo prints, which is
// exactly what ranger-base-w79sc closed at ADR 0081 D7's refusal in this
// same file. D4's Radius paragraph MEASURED two sites; this was the third,
// and it escaped because that grep's own sentence stopped at two.
//
// WHY A GOVERNANCE ROW GETS THE SAME RULE AS A REFUSAL. D4's subject is a
// path printed for the operator to PASTE INTO CONFIG, and the audience is
// not part of the test. `posse status` and the cockpit print this Detail
// verbatim, so the paste happens from here; and ADR 0081 D7's refusal cites
// this row by name for the same cutover step, so two lines reachable for one
// remedy agreed on the step and disagreed on the spelling.
//
// WHY THE SIBLING ROW IS PINNED THE OTHER WAY (third arm). `queue-not-a-repo`
// keeps `AbbrevHome` deliberately: it prescribes nothing and QUOTES the
// config line that is already there (`config queue_repo: names %s`), so the
// canonical spelling — which by construction is not the one in the file —
// would quote the operator's file back to them in words it does not contain,
// in the row whose whole job is to send them to that line. That is a
// judgement, not a derivation, so it is pinned in both directions rather
// than left to the next reader to re-decide from the diff.
//
// WHY A CASE ARM AND A SYMLINK ARM, the same reason as
// queuerefusalspelling_qa_test.go: they separate canonicalPath's two halves
// (ADR 0082 D1), so a render that only ran EvalSymlinks — the ADR's rejected
// option (a) — passes the symlink arm and reds the case arm.
//
// WHAT THESE ARMS DO NOT TOUCH: the VALUE. Every arm asserts that
// `QueueReady().Repo` is still the config's own spelling, because the
// reading that produced it also feeds `hooksDir` and
// `BeadsVisibility(hookRepo(q))` — so a future pass that "fixed" this by
// canonicalising `QueueRepo()` reds here, carrying D4's reason, instead of
// passing quietly.
//
// MEASURED 2026-11-08 (ranger-base-qm7eg, darwin 25.6.0, go1.26.10
// darwin/arm64, APFS case-insensitive): before these arms nothing in the
// suite read this Detail at all. govern_test.go's three G13 arms assert Key,
// ID and Class only (`TestGovG13UnmarkedQueueRepo` and the two that assert a
// row's absence), so the wording — and the spelling of the key inside it —
// was unconstrained, which is why the defect could be filed as cheap.
//
// MUTATION-CHECKED 2026-11-08, same box, each mutant run over these three
// names alone:
//
//   - reverting this site's render to `AbbrevHome(r.Repo)` (D4 undone here,
//     which is the state this bead was filed against) reds BOTH unmarked
//     arms, three assertions each: the repo the row names, the key it
//     prescribes, and the config's own spelling still being in the Detail.
//     The case arm's key was the config's `…/001/REPO` where the repo on
//     disk is `…/001/Repo`; the symlink arm's was `…/002/link/Repo` for a
//     repo at `…/001/Repo`. The not-a-repo arm stayed green, which is the
//     point of having it: it is not a weaker copy of these two.
//   - rendering the SIBLING row through CanonicalAbbrevPath as well (the
//     symmetry a later reader is most likely to reach for) reds the third
//     arm alone, naming the spelling it would have quoted the operator's
//     own `queue_repo:` line back as.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// qgsPrescribedKey captures the `beads_visibility:` key the row tells the
// reader to add. Read back out of the Detail rather than assumed, because
// the finding is that this one substring was the config's spelling: an arm
// that only asked whether the Detail contains SOME canonical path would pass
// on a row that printed both.
var qgsPrescribedKey = regexp.MustCompile("`([^`]+): private`")

// qgsRow is the G13 row of `key` as the real reading produces it, for a
// `queue_repo:` spelled `spelling` over a repo on disk as `repo`.
func qgsRow(t *testing.T, repo, spelling, key string) GovCondition {
	t.Helper()

	// THE PREMISE, in both directions: the two spellings are one directory,
	// and they are genuinely two strings — or the arm cannot tell a rendered
	// path from an unrendered one. qaCanonicalPath, so a fixture this
	// process created that cannot be named is a Fatal here rather than an
	// ignored error comparing "" (ADR 0082 D5).
	canonSpelling, canonRepo := qaCanonicalPath(t, spelling), qaCanonicalPath(t, repo)
	if canonSpelling == spelling {
		t.Fatalf("fixture premise: %q is already its own canonical spelling", spelling)
	}
	if canonSpelling != canonRepo {
		t.Fatalf("fixture premise: %q and %q canonicalise to %q and %q — two directories, so the row is right either way",
			spelling, repo, canonSpelling, canonRepo)
	}

	a := qApp(t, spelling) // queue_repo:, spelled the other way
	r := a.QueueReady()

	// And the VALUE is the config's spelling, NOT canonicalised: it is what
	// hooksDir and BeadsVisibility(hookRepo(q)) are asked in this same
	// reading (ADR 0082 D4 — the render happens at the render point).
	if r.Repo != spelling {
		t.Errorf("QueueReady().Repo = %q, want the config's own spelling %q — D4 keeps this a VALUE, and canonicalising it moves what hooksDir and BeadsVisibility are asked", r.Repo, spelling)
	}

	rows := r.GovRows()
	for _, c := range rows {
		if c.Key == key {
			if c.ID != "G13" || c.Class != GovLane {
				t.Errorf("the %s row is no longer a G13 LANE row: %+v", key, c)
			}
			return c
		}
	}
	t.Fatalf("no G13 %s row for queue_repo: %s (%+v): %+v", key, spelling, r, rows)
	return GovCondition{}
}

// qgsAssertUnmarked is the claim: both places the row prints the repo carry
// the kernel's spelling, and the config's spelling is in neither.
func qgsAssertUnmarked(t *testing.T, detail, repo, spelling string) {
	t.Helper()
	want := CanonicalAbbrevPath(repo)
	if !strings.Contains(detail, want) {
		t.Errorf("the row does not name the repo as %s, which is how the identity wall in that same repo spells it:\n%s", want, detail)
	}
	key := qgsPrescribedKey.FindStringSubmatch(detail)
	if key == nil {
		t.Fatalf("the row no longer prescribes a `<repo>: private` key, which is the whole of what makes it actionable (ADR 0015 §4 cutover step 5):\n%s", detail)
	}
	if key[1] != want {
		t.Errorf("the row prescribes the key %q, want %q — the spelling ADR 0081 D7's refusal and the identity wall of that repo both print, so an operator who greps for one finds one:\n%s", key[1], want, detail)
	}
	// And the config's own spelling appears NOWHERE: the two assertions
	// above both pass on a Detail that printed both spellings, which is the
	// shape this row had when it named the repo canonically and prescribed
	// the caller's key.
	if strings.Contains(detail, AbbrevHome(spelling)) {
		t.Errorf("the row still carries the config's spelling %s beside the canonical one:\n%s", AbbrevHome(spelling), detail)
	}
}

// A case-changed `queue_repo:` — ranger-base-99gww's axis, the one
// EvalSymlinks cannot close and the one ADR 0082 D1 opens the path for
// F_GETPATH to close.
func TestQAAQueueUnmarkedRowPrescribesTheKernelsCaseNotTheConfigs(t *testing.T) {
	repo := qcaseRepo(t, "Repo")
	recasedRepo := recased(t, repo)
	row := qgsRow(t, repo, recasedRepo, "queue-unmarked")
	qgsAssertUnmarked(t, row.Detail, repo, recasedRepo)
}

// A symlinked `queue_repo:` — the axis every platform's half of
// canonicalPath closes, so this arm runs where the case arm skips.
func TestQAAQueueUnmarkedRowPrescribesTheRepoNotTheSymlinkItWasReachedBy(t *testing.T) {
	repo := qcaseRepo(t, "Repo")
	link := filepath.Join(gitTempDir(t), "link")
	if err := os.Symlink(filepath.Dir(repo), link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	viaLink := filepath.Join(link, filepath.Base(repo))
	row := qgsRow(t, repo, viaLink, "queue-unmarked")
	qgsAssertUnmarked(t, row.Detail, repo, viaLink)
}

// The sibling row, pinned the other way: it prescribes no key, it quotes the
// `queue_repo:` line the operator has to go and edit, and so it keeps that
// line's own spelling. ranger-base-qm7eg decided this rather than deriving
// it, so both directions are asserted — a later pass that canonicalises
// every path in this function for symmetry reds here with the reason.
func TestQAAQueueNotARepoRowKeepsTheConfigsOwnSpelling(t *testing.T) {
	// A directory that exists and is not a checkout: NotARepo, with two
	// spellings of it to tell apart. qcaseRepo's repo would be a checkout.
	dir := filepath.Join(gitTempDir(t), "NotRepo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	spelling := recased(t, dir)

	row := qgsRow(t, dir, spelling, "queue-not-a-repo")
	if !strings.Contains(row.Detail, AbbrevHome(spelling)) {
		t.Errorf("the row does not quote the config's own spelling %s, which is the line it is sending the operator to edit:\n%s", AbbrevHome(spelling), row.Detail)
	}
	if strings.Contains(row.Detail, CanonicalAbbrevPath(dir)) {
		t.Errorf("the row quotes `config queue_repo:` as %s, a spelling that file does not contain — ADR 0082 D4 governs a path prescribed for PASTING, and this row prescribes none:\n%s",
			CanonicalAbbrevPath(dir), row.Detail)
	}
}
