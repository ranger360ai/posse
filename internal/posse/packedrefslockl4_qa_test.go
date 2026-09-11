//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0059 D4 at the LIST, because at the mount layer it cannot be measured
// (ranger-base-qfkw1, verifying ranger-base-hcbsc).
//
// D4 says L4 gets no `packed-refs.lock` twin of the L2 grant: a bind of an
// ABSENT source makes the source a DIRECTORY on the host (ADR 0038 decision
// 4), and that lock is absent whenever nothing holds it, so the twin would
// leave a `packed-refs.lock` DIRECTORY in the operator's git dir — the
// ranger-base-msex landmine with no file for a human to remove.
//
// ranger-base-hcbsc added `packed-refs.lock` to the wantNoMount list in
// TestWorktreeGitCommonDirIsTheGitCarveOut to pin that. MEASURED here
// 2026-09-11: that row cannot fail. Append the twin to sessionCommonDirWrites
// — which is where cage.go's own comment sends the next reader, and the list
// cage.go feeds to cageOverlay — and the row stays green, with or without the
// lock file on disk, because cageOverlay drops every read-write overlay whose
// source is not an existing DIRECTORY (cage.go, isExistingDir). The positive
// control is the same one-word-different mutant aimed at `refs`, a directory:
// that one reds the row. So the mount list is blind to this entry by
// construction, and every FILE row in that list (`packed-refs` too) is
// decoration rather than a pin.
//
// The list is where the twin would actually be typed, so the list is where
// D4 is assertable. This reds on the twin whatever cageOverlay would have
// done with it afterwards.
//
// MUTATION-CHECKED (2026-09-11): appending filepath.Join(common,
// "packed-refs.lock") to sessionCommonDirWrites' return reds this test and
// leaves TestWorktreeGitCommonDirIsTheGitCarveOut green.
//
// The L2 half — that sessionGitGrants DOES name the lock — is not restated
// here; TestQAWorktreeGrantNamesObjectsLogsAndItsOwnRefOnly and
// TestQAGatesReportShowsThePackedRefsLockAsOneWriteLine own it, and both were
// shown able to fail. What this adds is the arm those two cannot carry: the
// two walls disagreeing on purpose.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQAL4CommonDirWritesNameNoPackedRefsLock(t *testing.T) {
	t.Parallel()
	wt := gitWorktreeFixture(t)
	dirs := LinkedGitDirs(wt)
	if len(dirs) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", dirs)
	}
	common := dirs[1]

	got := sessionCommonDirWrites(wt, common)
	if len(got) == 0 {
		t.Fatal("the L4 common-dir write list is empty — this fixture is not the shape the test is about, and an absence is true of nothing here")
	}
	// The positive control, in the same test and against the same list: the
	// three regions D4 DOES grant must be in it, or the assertion below is
	// satisfied by a list that says nothing at all.
	for _, want := range []string{dirs[0], filepath.Join(common, "objects"), filepath.Join(common, "logs")} {
		if !wantsPath(got, want) {
			t.Fatalf("L4 no longer grants %s — the three regions a detached-HEAD commit writes are this list's reason to exist:\n  %v", want, got)
		}
	}
	lock := filepath.Join(common, "packed-refs.lock")
	if wantsPath(got, lock) {
		t.Errorf("L4 names %s. ADR 0059 D4 declines that twin: at L4 this is a BIND, and a bind of an absent source makes the source a DIRECTORY on the host (ADR 0038 decision 4), so the twin leaves a packed-refs.lock DIRECTORY in the operator's git dir — the ranger-base-msex landmine with no file for a human to remove. L2 grants the lock (seatbelt.go, ADR 0059 D1) and L4 deliberately does not.\n  %v", lock, got)
	}
	// And it is not reached by a sibling spelling either.
	for _, p := range got {
		if strings.Contains(filepath.Base(p), "packed-refs") {
			t.Errorf("L4 names a packed-refs path (%s): packed-refs, packed-refs.new and packed-refs.lock all stay under the :ro common mount", p)
		}
	}
}

// wantsPath compares the way the mount layer does — after absResolve, so a
// /tmp vs /private/tmp spelling cannot make this pass for the wrong reason.
func wantsPath(list []string, p string) bool {
	for _, got := range list {
		if absResolve(got) == absResolve(p) {
			return true
		}
	}
	return false
}
