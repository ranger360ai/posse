package posse

// QA pins for the landing sweep (ranger-base-nurl), one per clause of the
// bead's DONE WHEN: a bead closed AFTER its dispatch pass stopped watching
// still lands, and where it cannot, the shop sees it without anyone running
// the census by hand.
//
// The fixture is the incident itself: a session tree with a commit on its
// branch, a bead the store calls closed, and NOTHING watching — no live
// agent, no ready work, no meta. That is what a pass finds when a persona
// closes a bead whose wait ran out on the pass before.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nurlStranded builds the incident: repo, a session branch with one commit
// on it, and no session alive anywhere. `stamp` says whether the branch
// carries the record of which bead it is holding.
func nurlStranded(t *testing.T, status string, stamp bool) (*Dispatcher, string, *SessionTree) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	// No ready work: the pass this sweep runs on is not the pass that
	// dispatched the bead, and by now there is nothing left to dispatch.
	repo := wtqaRepo(t, b.App, `[]`, `[{"id":"a-1","status":"`+status+`"}]`)
	idleClaude(t, fake)

	tr, err := b.App.EnsureSessionTree(repo, SessionForBead("ranger", repo, "a-1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")
	if stamp {
		if err := recordBead(tr.Repo, tr.Branch, "a-1"); err != nil {
			t.Fatal(err)
		}
	}
	return d, repo, tr
}

// The headline: the pass lands it, and it is a PASS that does — not a human
// running `posse worktrees --land`.
func TestPassLandsABeadClosedAfterItsPassStoppedWatching(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the persona's work\n" {
		t.Fatalf("a closed bead's code is still not on the repo's branch: %v\n%s", err, out)
	}
	if !strings.Contains(out, "a-1") || !strings.Contains(out, "1 commit(s) fast-forwarded") || !strings.Contains(out, "closed after its pass") {
		t.Errorf("the pass did not say what it landed:\n%s", out)
	}
	// The tree and the branch are still standing, and since ADR 0058 that
	// is fact 4 and not a rule: this pass just committed in the tree, so
	// the retire's grace keeps it (silently — retiresweep_qa_test.go is
	// where the retire itself is pinned). What the sweep still never does
	// on this pass is remove a tree it has only this second landed.
	if _, err := os.Stat(tr.Path); err != nil {
		t.Errorf("the sweep removed a session tree: %v", err)
	}
	if !branchExists(tr.Repo, tr.Branch) {
		t.Errorf("the sweep deleted %s", tr.Branch)
	}
	// And a second pass over the same tree has nothing left to say.
	d2 := newTestDispatcher(t, d.HB)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d2); strings.Contains(out, tr.Branch) {
		t.Errorf("a tree with nothing unlanded is still being reported:\n%s", out)
	}
}

// The wrong arm: an OPEN bead is a persona at work, and its tree is not the
// launcher's to land. Without this the sweep is just `--land`, which lands
// everything and would take a persona's half-finished commits onto main.
func TestSweepLeavesTheTreeOfABeadThatIsStillOpen(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "in_progress", true)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err == nil {
		t.Fatalf("an open bead's work was landed on the repo's branch:\n%s", dispatcherOut(d))
	}
	if out := dispatcherOut(d); strings.Contains(out, tr.Branch) {
		t.Errorf("an open bead's tree was reported as unlanded work:\n%s", out)
	}
}

// A tree nothing points at is REPORTED, never guessed at. The branch name
// carries the bead id, and parsing it back out is a guess — persona names
// and repo basenames both contain '-' — so the only two honest answers are
// the recorded one and "I cannot tell", and the second one has to be loud.
func TestSweepWillNotGuessWhichBeadATreeHolds(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", false)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err == nil {
		t.Fatalf("a tree whose bead nothing records was landed anyway:\n%s", out)
	}
	if !strings.Contains(out, tr.Branch) || !strings.Contains(out, "no record says which bead") {
		t.Errorf("an unlanded tree with no record must be named on the pass:\n%s", out)
	}
	// And it must name a command that actually lands it. `posse worktrees
	// --land` alone stopped being one when --land learned to read the same
	// record this sweep does (ranger-base-atxe) — a prescription the sweep's
	// own refusal makes untrue is worse than none.
	if !strings.Contains(out, "--land --force") {
		t.Errorf("the pass prescribes a command that would refuse this very tree:\n%s", out)
	}
}

// The record is on the BRANCH and not only in the session meta, and this is
// the whole reason: a kill removes the meta and leaves the tree standing
// (rangerhq-09o2), so a pointer that lives only in the meta disappears
// exactly when the work is stranded. This one goes through a real dispatch
// pass, so it pins the launch's own stamping and not a helper.
func TestTheBranchRecordsItsBeadAndSurvivesTheMetaGoingAway(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"open"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	session := SessionForBead("ranger", repo, "a-1")
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("no meta for %s:\n%s", session, dispatcherOut(d))
	}
	if got := beadOf(m.Repo, m.Branch); got != "a-1" {
		t.Fatalf("the launch did not stamp the bead on %s: beadOf = %q", m.Branch, got)
	}
	// The kill's half: the meta goes, the tree and the branch stay.
	if err := os.Remove(b.metaPath(session)); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta(session); ok {
		t.Fatal("the meta was not removed — the test is not measuring the case it is about")
	}
	trees, err := SessionTreesIn([]string{repo})
	if err != nil || len(trees) != 1 {
		t.Fatalf("session trees = %+v, %v; want the one the pass made", trees, err)
	}
	if trees[0].Bead != "a-1" {
		t.Errorf("with the meta gone the tree no longer names its bead: %q", trees[0].Bead)
	}
}

// --dry-run is the operator's diagnostic: it says what a real pass would
// land and lands nothing. Moving a repo's branch is exactly the kind of
// acting the flag exists to withhold.
func TestSweepUnderDryRunSaysWhatItWouldLandAndLandsNothing(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)
	d.DryRun = true

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err == nil {
		t.Fatalf("--dry-run landed a session branch on the repo:\n%s", out)
	}
	if !strings.Contains(out, "would land "+tr.Branch) || !strings.Contains(out, "bead a-1 closed") {
		t.Errorf("--dry-run did not say what a real pass would land:\n%s", out)
	}
}

// The slot session (SessionFor(persona, dir)) is reused across beads, and
// NoteBead moves the meta's pointer when a new bead resumes into it. The
// branch's copy moves with it — a stale one would have the sweep asking
// about the wrong bead, and answering "closed" for a tree still being
// worked in.
func TestNoteBeadMovesTheBranchRecordToo(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	tr, err := b.App.EnsureSessionTree(repo, "ranger-repo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.writeMeta(&HerdrMeta{Name: "ranger-repo", Dir: tr.Path, Repo: tr.Repo, Branch: tr.Branch, Bead: "a-1"}); err != nil {
		t.Fatal(err)
	}
	if err := recordBead(tr.Repo, tr.Branch, "a-1"); err != nil {
		t.Fatal(err)
	}
	b.NoteBead("ranger-repo", "a-2")
	if got := beadOf(tr.Repo, tr.Branch); got != "a-2" {
		t.Errorf("the branch still names %q after a-2 resumed into the slot", got)
	}
}

// Every tree standing when this landed was cut before the branch was
// stamped, so the sweep would have nothing but "I cannot tell" to say about
// all of them. Where the session's meta is still alive it names the bead —
// the record mergeBack itself reads — and the sweep joins on it, lands the
// work, and writes the answer onto the branch so the next pass does not need
// the meta at all.
func TestSweepBackfillsTheBeadFromASurvivingMeta(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", false) // no branch stamp: a legacy tree
	session := SessionForBead("ranger", repo, "a-1")
	if err := d.HB.writeMeta(&HerdrMeta{Name: session, Dir: tr.Path, Repo: tr.Repo, Branch: tr.Branch, Bead: "a-1"}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the persona's work\n" {
		t.Fatalf("a legacy tree whose meta names its bead was not landed: %v\n%s", err, out)
	}
	if got := beadOf(tr.Repo, tr.Branch); got != "a-1" {
		t.Errorf("the sweep did not stamp the branch it joined: beadOf = %q", got)
	}
}

// The join is on the BRANCH, not on the session name: a meta whose session
// was recreated against another tree is a different tree's record, and
// landing on it would take one bead's close as another's.
func TestSweepWillNotJoinAMetaThatNamesAnotherBranch(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", false)
	session := SessionForBead("ranger", repo, "a-1")
	if err := d.HB.writeMeta(&HerdrMeta{Name: session, Dir: tr.Path, Repo: tr.Repo, Branch: "posse/somewhere-else", Bead: "a-1"}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err == nil {
		t.Fatalf("a tree was landed on another branch's run record:\n%s", out)
	}
	if !strings.Contains(out, "no record says which bead") {
		t.Errorf("the sweep must say it cannot tell rather than join across branches:\n%s", out)
	}
}

// --dry-run stamps nothing either: the backfill is a git config write, and
// the flag's promise is that a diagnostic pass changes no state.
func TestSweepUnderDryRunDoesNotBackfillTheStamp(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", false)
	d.DryRun = true
	session := SessionForBead("ranger", repo, "a-1")
	if err := d.HB.writeMeta(&HerdrMeta{Name: session, Dir: tr.Path, Repo: tr.Repo, Branch: tr.Branch, Bead: "a-1"}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if got := beadOf(tr.Repo, tr.Branch); got != "" {
		t.Errorf("--dry-run wrote the branch record: beadOf = %q", got)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "would land "+tr.Branch) {
		t.Errorf("--dry-run must still say what it would land:\n%s", out)
	}
}
