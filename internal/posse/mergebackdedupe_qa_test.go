//go:build posse_arm2

package posse

// Filed verifying ranger-base-eul1's close (ranger-base-ogzh). The close's
// five mutations cover the read-back and the dedupe's existence; what nothing
// covered is the dedupe's KEY. Loosening `b.Title == title` to a prefix match
// on "merge-back blocked: " — dropping the branch the whole title exists to
// carry — left the three new pins and the pre-existing one green (measured
// 2026-08-28, mutation E6).
//
// The consequence is the bead's own failure, arrived at from the other side:
// one persona's open merge-back bead would silently swallow every other
// persona's, and a closed bead's commits would sit on a branch with nothing
// filed and nothing to retry it. The dedupe key is branch+base because a
// branch is cut per bead; this pin is what says so.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAMergeBlockedDedupeIsPerBranchNotPerLabel(t *testing.T) {
	t.Parallel()
	d, repo, _ := wtqaPassWithWork(t, func(repo, _ string) {
		commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
		// Another seat's blocked merge, open, in the same lane and with the
		// same title SHAPE — everything but the branch.
		write(t, filepath.Join(repo, "fake-list-labeled.json"), fmt.Sprintf(
			`[{"id":"m-8","title":%q,"status":"open","labels":["code"]}]`,
			mergeBlockedTitle("posse/someone-else-posse-a-99", "main")))
	})
	out := dispatcherOut(d)

	if strings.Contains(out, "m-8 already filed") {
		t.Errorf("another branch's merge-back bead is not this branch's handoff:\n%s", out)
	}
	if bd := bdCalls(t, fakeDirOf(t)); !strings.Contains(bd, "create merge-back blocked") {
		t.Fatalf("this branch got no handoff at all:\n%s", bd)
	}
	// And what landed is about THIS branch, not the planted one.
	b, err := os.ReadFile(filepath.Join(repo, "fake-list-labeled.json"))
	if err != nil {
		t.Fatalf("no labeled listing: %v", err)
	}
	if n := strings.Count(string(b), `"merge-back blocked: `); n != 2 {
		t.Errorf("want the planted bead and this branch's own, got %d:\n%s", n, b)
	}
	if strings.Contains(out, "someone-else") {
		t.Errorf("the pass named another seat's branch:\n%s", out)
	}
}
