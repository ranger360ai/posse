package posse

import (
	"strings"
	"testing"
)

// TestQAGatesWritesExecutablesUnderTheForkLock guards ranger-base-ntsvf's
// fix: every shim and hook gates.go writes with an exec bit went through
// plain os.WriteFile, which leaves the golang/go#22315 window open (see
// execwrite.go) between the write and the close. A concurrent fork anywhere
// else in the same test binary can land in that window, and on linux the
// forked child's inherited write descriptor answers a later exec of that
// same file with ETXTBSY — exactly the ci.yml run 34065190602 arm 1 (ubuntu)
// failure: TestQAL1CommitRefusalNamesTheNewFileRoute failed in 0.00s with
// every "must carry" assertion seeing an EMPTY refusal, which is what
// exec.Cmd.CombinedOutput returns when Start() itself fails rather than the
// shim ever running.
//
// ranger-base-qp1hm's arm repacking (1b840d96) raised this from
// theoretical to observed by raising the concurrency inside each arm
// (more tests sharing one binary and clock), which is exactly the traffic
// execwrite.go's own doc comment says widens the window. The fix is
// WriteExecutable at every such call site in gates.go; this is the reader
// that keeps it that way. It is a source census, not a timing race, because
// the race itself is already covered — deeply, and expensively — by
// execwrite_test.go; what a regression here needs is proof the fixed call
// sites still ROUTE through that mechanism, not a second timing rig.
//
// Not a fix for the arm 3 (macos) red on the same run
// (TestQAConstitutionWallPassesTheIdenticalCommitUnmarked, a t.TempDir
// RemoveAll cleanup racing .git/objects) — execwrite.go's own doc says
// darwin does not enforce ETXTBSY at all, so this mechanism cannot be that
// failure's cause. That one is undiagnosed; see the bead comments.
func TestQAGatesWritesExecutablesUnderTheForkLock(t *testing.T) {
	t.Parallel()
	src := i9dbbRead(t, "internal", "posse", "gates.go")
	for _, want := range []string{
		`WriteExecutable(filepath.Join(binDir, c), []byte(script), 0o755)`,
		`WriteExecutable(p, []byte(script), 0o755)`,
		`WriteExecutable(chained, []byte(script), 0o755)`,
		`WriteExecutable(p, []byte(chainHookDispatcherWith(slot, neighbour)), 0o755)`,
		`WriteExecutable(p, []byte(chainHookDispatcherWith(slot, neighbor)), 0o755)`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("gates.go must write this executable through WriteExecutable, not os.WriteFile: %q not found", want)
		}
	}
	// The other direction: no exec-bit os.WriteFile call is left behind for
	// a future edit to reintroduce beside a WriteExecutable one, which a
	// pure substring-presence check above would miss entirely.
	execModes := []string{"0o755", "0o750", "0o751", "0o770", "0o771", "0o775"}
	for _, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "os.WriteFile(") {
			continue
		}
		for _, mode := range execModes {
			if strings.Contains(line, mode) {
				t.Errorf("gates.go still writes an executable with plain os.WriteFile (ETXTBSY window, golang/go#22315): %s", strings.TrimSpace(line))
			}
		}
	}
}
