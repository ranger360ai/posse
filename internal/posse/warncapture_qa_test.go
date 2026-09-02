package posse

import (
	"os"
	"strings"
	"testing"
)

// The harness gives every test backend its own warning stream, and it is
// wired all the way through the launch — not just set on the struct.
//
// The mechanism this pins is the chain planLaunch actually walks:
// CreateSession → planLaunch → b.warnWriter() → EnsureSessionTree. A nil
// Warn there is os.Stderr, and a line on the test binary's stderr is a line
// no assertion can read and `go test` will hang under some other test's
// `--- FAIL` (ranger-base-ihd2, filed off ranger-base-ljiu).
//
// The fixture is the cheapest launch that provokes a warning without a
// failure: a detached checkout, which EnsureSessionTree fails open on with a
// line naming the SHARED checkout.
func TestQATestBackendCapturesTheLaunchWarningStream(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	if b.Warn == nil || b.warnWriter() == os.Stderr {
		t.Fatalf("a test backend's warnings go to the test binary's stderr: Warn = %v", b.Warn)
	}
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")

	if err := b.CreateSession(NewSessionOpts{Name: "s-warned", Dir: repo, Cmd: "true", Worktree: true}); err != nil {
		t.Fatalf("the launch must fail open on a detached checkout: %v", err)
	}
	got := warnBuf(t, b).String()
	if !strings.Contains(got, "SHARED checkout") {
		t.Errorf("the launch's warning did not reach the harness buffer, so it went to stderr:\n%q", got)
	}
}
