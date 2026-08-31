package posse

// ranger-base-ntsz shipped sameRemoval: a deletion recorded before its branch
// merged stays owned afterwards, because the record's commit is an ancestor of
// the commit the census now blames and nothing between them puts the id back.
// These are the two shapes that fix has to hold in beyond the one merge the
// bead measured — a second merge, and a genuinely new loss that reaches the
// census through one.

import (
	"os"
	"os/exec"
	"testing"
)

// qblCommitAt is qblCommit with an explicit commit second. The census walks
// `git log` newest-FIRST and keeps the first removal it meets per id, and git
// orders that walk by commit date: two removals of one id sharing a second
// leave which one the census blames to the traversal's tie-break. Measured
// while verifying ranger-base-ntsz — one repo answered "owned, quiet" on one
// run and "lost" on the next, because every fixture commit landed in the same
// second. The branch fixtures below stamp their own seconds, so what is under
// test is the code and not the clock.
func qblCommitAt(t *testing.T, repo, when, msg string, lines ...string) {
	t.Helper()
	qblWrite(t, repo, lines...)
	qblGit(t, repo, "add", "-A")
	cmd := exec.Command("git", "-C", repo, "commit", "-q", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit %q: %v %s", msg, err, out)
	}
}

// ranger-base-ntsz measured ONE merge. A posse branch can reach main through
// more than one — a persona's branch merges into an integration branch and
// that merges on — and every merge's first-parent net diff shows the same
// removal again, and is newer again. If ownership only survived the first hop
// the alarm would come back a merge later: the same bead with a longer fuse.
//
// Mutation-checked: put the exemption back on sha identity (the pre-ntsz
// `rec.Commit == lb.Commit`) and this reds, alongside
// TestLedgerRecordSurvivesAMergeOfTheDroppingCommit.
func TestLedgerRecordSurvivesAMergeOfAMerge(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommitAt(t, repo, "2026-08-01T10:00:00", "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "branch", "-M", "main")

	qblGit(t, repo, "checkout", "-q", "-b", "feature")
	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommitAt(t, repo, "2026-08-01T10:01:00", "side drops q-2", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("setup: a recorded deletion is owned on its own branch, got %+v", l)
	}

	qblGit(t, repo, "checkout", "-q", "feature")
	qblGit(t, repo, "merge", "-q", "--no-ff", "--no-edit", "side")
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("one merge un-recorded an owned deletion: %+v", l)
	}

	qblGit(t, repo, "checkout", "-q", "main")
	qblGit(t, repo, "merge", "-q", "--no-ff", "--no-edit", "feature")
	qblLive(t, repo, "q-1")
	if l := qblLost(t, repo); len(l) != 0 {
		t.Errorf("a merge OF a merge un-recorded an owned deletion: %+v", l)
	}
}

// The false-negative arm of the same fix. sameRemoval exempts across a merge
// because nothing in rec..found puts the id back, so that re-addition is the
// whole discriminator. TestLedgerDoesNotExemptALaterLoss measures it on a
// linear history, where rec..found is a straight line. This measures it where
// the restore and the second drop happen on a BRANCH and reach the census
// through a merge: the range is a graph there, and a reader that missed the
// re-addition in it would exempt a loss nobody owns and say nothing — the one
// direction this mechanism may never fail in.
//
// Mutation-checked: remove the re-addition guard (sameRemoval returning true
// on ancestry alone) and this reds.
func TestLedgerDoesNotExemptASecondLossThatArrivesThroughAMerge(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommitAt(t, repo, "2026-08-01T10:00:00", "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "branch", "-M", "main")
	qblCommitAt(t, repo, "2026-08-01T10:01:00", "main drops q-2", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("setup: the first loss is owned, got %+v", l)
	}

	// A branch restores it — an import is the operator's documented move —
	// and then loses it again. Nobody owned THAT removal.
	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommitAt(t, repo, "2026-08-01T10:02:00", "side restores q-2",
		qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblCommitAt(t, repo, "2026-08-01T10:03:00", "side drops q-2 again", qblLine("q-1", "open"))
	qblGit(t, repo, "checkout", "-q", "main")
	qblGit(t, repo, "merge", "-q", "--no-ff", "--no-edit", "side")
	qblLive(t, repo, "q-1")

	lost := qblLost(t, repo)
	if len(lost) != 1 || lost[0].ID != "q-2" {
		t.Errorf("a second loss of q-2, restored and re-dropped on a branch, must still alarm: got %+v", lost)
	}
}
