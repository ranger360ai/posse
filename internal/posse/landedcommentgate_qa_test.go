//go:build posse_arm2

package posse

// A QA arm added while verifying the close of ranger-base-w9w37
// (ranger-base-9xt70). The fix gates the green→red FILING on whether the
// verdict was recorded, and deliberately does NOT gate the COMMENT rows —
// the close states that choice and the notes fragment carries it as a
// residual. Nothing asked for it: applying the same gate to the red→red arm
// left all 217 tests of the arm-2 landed filter green, so the distinction
// the fix introduces had no arm.
//
// It is worth one, because gating that row would cost two things D5 owes a
// red stretch, and both only while a `git config` write cannot take
// `.git/config.lock`, which is when nobody is reading carefully:
//
//   - the comment naming the newest red tree, which is the only place the
//     verdict is written down once the warn has scrolled; and
//   - the DEGENERATE filing — a stretch whose bead a fixer closed mid-red has
//     no bead at all, and the red→red arm is what puts one back. Gated, a
//     red main with a closed bead and a held lock would have no open
//     attributed bead and nothing saying so, which is D5's last sentence
//     failing in the other direction from the one w9w37 fixed.
//
// MEASURED 2026-11-12 (this file): with the lock held throughout, the comment
// lands and the degenerate filing happens, and the record stays stale the
// whole time — so both halves are read over a write that really did fail.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The comment rows are not gated on the record: over a `git config` write
// that cannot take the lock, a red→red landing still comments on the open
// bead, and a red→red landing whose bead has been closed still files the one
// D5 owes the stretch.
func TestTheCommentRowsAreNotGatedOnTheRecordTheFilingRowIs(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldfJudge(t, c, repo) // green→red, recorded: one bead for the stretch
	first := ldfFiled(t, c, repo)
	redTree, v := landedRecorded(landedJudgeRule, repo)
	if v != landRed {
		t.Fatalf("the fixture did not reach a recorded red (%s %s), so nothing below is measured", redTree, v)
	}

	// From here the record cannot be written at all. Everything below is
	// read over a verdict that failed to record, which is the state the
	// filing row now refuses to act on.
	lock := filepath.Join(repo, ".git", "config.lock")
	write(t, lock, "")
	defer func() { _ = os.Remove(lock) }()

	// ROW 1: red→red with the bead open. The comment is the verdict's only
	// written record once the warn scrolls, so it must still land.
	ldfLandUnderLock(t, repo, "second.txt", "a landing onto a red main\n", "ranger-base-lll11: more work on a red main")
	ldfJudge(t, c, repo)
	if got, gv := landedRecorded(landedJudgeRule, repo); got != redTree || gv != landRed {
		t.Fatalf("the record moved to %s %s although its write failed, so this arm measured nothing", got, gv)
	}
	if is := ldfFiled(t, c, repo); is.ID != first.ID {
		t.Fatalf("a red→red landing over an unwritable record filed %s beside %s", is.ID, first.ID)
	}
	cs := ldfComments(t, c, repo, first.ID)
	if len(cs) != 1 {
		t.Fatalf("%d comment(s) on %s over an unwritable record, want the one naming the newest red tree — the comment rows are NOT gated on the record (ranger-base-w9w37), and this log line is the only place the verdict is written: %s",
			len(cs), first.ID, ldfCommentTexts(cs))
	}
	newTree, _ := landedCheckRecordedOf(t, repo)
	if !strings.Contains(cs[0].Text, "still red at "+abbrevSHA(newTree)) {
		t.Errorf("the comment does not name the tree this landing judged (%s):\n%s", abbrevSHA(newTree), cs[0].Text)
	}

	// ROW 2: the degenerate half. A fixer closes the bead while main is
	// still red; the next red landing has nothing to comment on and nobody
	// attributed, so it files — and that must not be gated on the record
	// either, or a red main with a closed bead and a held lock has no open
	// bead and no twin to say so.
	if err := c.bd.Close(repo, first.ID, ldfNewerWho); err != nil {
		t.Fatal(err)
	}
	if open := ldfBeads(t, c, repo); len(open) != 0 {
		t.Fatalf("the fixture's close left %s open, so the degenerate arm below is not reached", ldfTitles(open))
	}
	ldfLandUnderLock(t, repo, "third.txt", "another landing onto a red main\n", "ranger-base-mmm12: and more")
	ldfJudge(t, c, repo)
	if got, gv := landedRecorded(landedJudgeRule, repo); got != redTree || gv != landRed {
		t.Fatalf("the record moved to %s %s although its write failed, so the degenerate arm measured nothing", got, gv)
	}
	again := ldfFiled(t, c, repo)
	if again.ID == first.ID {
		t.Fatalf("the degenerate arm read the closed bead as open: %s", again.ID)
	}
	if !strings.Contains(again.Description, "not open any more") {
		t.Errorf("the replacement bead does not say why it was filed:\n%s", again.Description)
	}
	// ROW 3: red→green, still over the held lock. This is the line the fixer
	// is waiting for — "main went green" — and D5 leaves the bead OPEN
	// because closing it is the fixer's job. Gated on the record, a main
	// that goes green while the lock is held says so nowhere.
	if err := os.Remove(filepath.Join(repo, "boom.txt")); err != nil {
		t.Fatal(err)
	}
	ldfLandUnderLock(t, repo, "fixed.txt", "the fix for the red\n", "ranger-base-nnn13: fix the red")
	mustGit(t, repo, "commit", "-q", "-m", "ranger-base-nnn13: drop the breakage", "--", "boom.txt")
	ldfJudge(t, c, repo)
	if got, gv := landedRecorded(landedJudgeRule, repo); got != redTree || gv != landRed {
		t.Fatalf("the record moved to %s %s although its write failed, so the green arm measured nothing", got, gv)
	}
	if is := ldfFiled(t, c, repo); is.ID != again.ID {
		t.Fatalf("a red→green landing over an unwritable record filed %s beside %s — D5 leaves the bead open for the fixer and files nothing here", is.ID, again.ID)
	}
	green := ldfComments(t, c, repo, again.ID)
	if len(green) != 1 || !strings.Contains(green[0].Text, "green at ") {
		t.Errorf("no `green at` comment on %s over an unwritable record — that line is what the fixer is waiting for, and gating it on the record would drop it (ranger-base-w9w37): %s",
			again.ID, ldfCommentTexts(green))
	}

	if s := out.String(); !strings.Contains(s, "could not be recorded") {
		t.Errorf("the pass never said the record failed, so the whole arm may have run over a record that wrote:\n%s", s)
	}
}

// landedCheckRecordedOf is the tree of the repo's current tip, which is what
// the judged landing's verdict is ABOUT — distinct from what the record
// holds, which is the point of every arm above.
func landedCheckRecordedOf(t *testing.T, repo string) (string, string) {
	t.Helper()
	tip := mustGit(t, repo, "rev-parse", "HEAD")
	return revTreeID(repo, tip), tip
}
