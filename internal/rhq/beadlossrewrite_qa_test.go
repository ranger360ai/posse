package rhq

// ranger-base-6mbz: sameRemoval (ranger-base-ntsz) covers a record whose
// commit is an ancestor of the commit the census now blames. A rebase
// replaces the recorded commit with a new sha carrying the same change, and a
// squash merge never carries the dropping commit into the target's history at
// all — either way rec is not an ancestor of found, ancestry has nothing to
// say about the pair, and the id alarmed forever until this bead's fix.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// qblTouch commits a change to a file OTHER than issues.jsonl, so a caller
// can advance main's line without perturbing the census's own diff history —
// exactly what "the branch's base moved on before it rejoined" needs.
func qblTouch(t *testing.T, repo, when, msg, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	qblGit(t, repo, "add", "-A")
	cmd := exec.Command("git", "-C", repo, "commit", "-q", "-m", msg)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit %q: %v %s", msg, err, out)
	}
}

// The bead's own repro, arm one: a rebase replays the exact same drop under a
// new sha. Mutation-checked: revert sameRemoval to the ntsz-only ancestor
// check and this reds.
func TestLedgerRecordSurvivesARebaseOfTheDroppingCommit(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommitAt(t, repo, "2026-08-01T10:00:00", "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "branch", "-M", "main")

	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommitAt(t, repo, "2026-08-01T10:01:00", "side drops q-2", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("setup: a recorded deletion is owned on its own branch, got %+v", l)
	}

	// main moves on before the branch rejoins it — an ordinary conflict,
	// which is why the rebase this provokes exists at all.
	qblGit(t, repo, "checkout", "-q", "main")
	qblTouch(t, repo, "2026-08-01T10:02:00", "main touches another file", "OTHER.md", "unrelated\n")

	qblGit(t, repo, "checkout", "-q", "side")
	qblGit(t, repo, "rebase", "-q", "main")
	qblGit(t, repo, "checkout", "-q", "main")
	qblGit(t, repo, "merge", "-q", "--ff-only", "side")
	qblLive(t, repo, "q-1")

	if l := qblLost(t, repo); len(l) != 0 {
		t.Errorf("a rebase replayed the owned drop under a new sha, not a new loss: got %+v", l)
	}
}

// The bead's own repro, arm two: a squash merge never makes the side
// branch's commit an ancestor of anything — the target's own diff carries the
// same drop instead. Mutation-checked: revert sameRemoval to the ntsz-only
// ancestor check and this reds.
func TestLedgerRecordSurvivesASquashOfTheDroppingCommit(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommitAt(t, repo, "2026-08-01T10:00:00", "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "branch", "-M", "main")

	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommitAt(t, repo, "2026-08-01T10:01:00", "side drops q-2", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("setup: a recorded deletion is owned on its own branch, got %+v", l)
	}

	qblGit(t, repo, "checkout", "-q", "main")
	qblTouch(t, repo, "2026-08-01T10:02:00", "main touches another file", "OTHER.md", "unrelated\n")
	qblGit(t, repo, "merge", "-q", "--squash", "side")
	cmd := exec.Command("git", "-C", repo, "commit", "-q", "-m", "squashed")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-08-01T10:03:00", "GIT_COMMITTER_DATE=2026-08-01T10:03:00")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit squashed: %v %s", err, out)
	}
	qblLive(t, repo, "q-1")

	if l := qblLost(t, repo); len(l) != 0 {
		t.Errorf("a squash merge carried the owned drop in under a new sha, not a new loss: got %+v", l)
	}
}

// The replay fallback must not swallow a genuinely new loss just because its
// history happens to be rebase-shaped (rec not an ancestor of found, and
// found's parent has moved past rec's). Content is the second gate: q-2 comes
// back with different content before main drops it again, so the replayed
// line does not read back identical and the id must still alarm.
func TestARebaseShapedSecondLossWithDifferentContentStillAlarms(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommitAt(t, repo, "2026-08-01T10:00:00", "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "branch", "-M", "main")

	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommitAt(t, repo, "2026-08-01T10:01:00", "side drops q-2", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("setup: a recorded deletion is owned on its own branch, got %+v", l)
	}

	// main moves on, restores q-2 under DIFFERENT content, and drops it
	// again — a real second loss that happens to share the rebase shape.
	qblGit(t, repo, "checkout", "-q", "main")
	qblTouch(t, repo, "2026-08-01T10:02:00", "main touches another file", "OTHER.md", "unrelated\n")
	qblCommitAt(t, repo, "2026-08-01T10:03:00", "main restores q-2 closed",
		qblLine("q-1", "open"), qblLine("q-2", "closed"))
	qblCommitAt(t, repo, "2026-08-01T10:04:00", "main drops q-2 again", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")

	lost := qblLost(t, repo)
	if len(lost) != 1 || lost[0].ID != "q-2" {
		t.Errorf("a second loss with different content must still alarm even in a rebase-shaped history: got %+v", lost)
	}
}
