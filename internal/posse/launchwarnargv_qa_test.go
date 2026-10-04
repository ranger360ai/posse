//go:build !posse_arm2 && !posse_arm3

package posse

// The OTHER call site of the launch-warn stream — ranger-base-z5lmj,
// verifying the ranger-base-lcode close.
//
// ranger-base-lcode routed a dispatched launch's own diagnostic lines to the
// launcher's writer by handing NewSessionOpts.Warn down at the dispatcher's
// two createSession calls: launchSession (dispatch.go) and launchWithPrompt,
// the ADR 0013 §2 argv-delivery path a session on a `prompt: argv` runtime is
// created through. Its pins drive the first one.
//
// MEASURED 2026-10-03: with `Warn: d.launchWarns()` deleted from
// launchWithPrompt ALONE and left in place at launchSession,
// `go test ./internal/posse -count=1` — the whole arm-1 package, no filter —
// is `ok 260.8s`. So that half of the fix was held by nothing: a landing that
// dropped it would put every grok/codex seat's pre-heal drift line and
// DEGRADED line back on the process's stderr, which under a `--watch` loop is
// /dev/null (RE-MEASURED on the loop running that day, pid 22611: fd 0, 1 and
// 2 all on /dev/null), and no pin would say so.
//
// Same shape as TestQADispatchLaunchWarningsLandInTheLoopsRecordNotOnStderr,
// one runtime over, plus the control that says this pass really did take the
// argv path and not the one already pinned.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAArgvLaunchWarningsLandInTheLoopsRecordNotOnStderr(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// A persona on a runtime that declares `prompt: argv`, which is what
	// sends the pass through launchWithPrompt rather than launchSession.
	write(t, filepath.Join(b.App.AgentsDir, "ranger.md"),
		"---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: grok\n---\nYou are ranger.\n")
	repo, hook := lwsStaleWall(t, b, `[{"id":"g-1","title":"t","labels":["go"]}]`, `[{"id":"g-1","status":"closed"}]`)
	agentPerLaunch(t, fake)

	stderrish := warnBuf(t, b)

	n, _ := d.Run("", "", 0)
	out := dispatcherOut(d)
	if n != 1 {
		t.Fatalf("the pass launched nothing, so this test asserts nothing: n=%d\n%s", n, out)
	}
	// THE CONTROL, and the reason this file exists: without it the pin can
	// pass by having taken launchSession, which is already held.
	if !strings.Contains(out, "prompt on the launch line") {
		t.Fatalf("this pass did not take the argv path, so it is a second copy of the launchSession pin:\n%s", out)
	}
	session := SessionForBead("ranger", repo, "g-1")
	if _, err := os.Stat(b.App.WorkPromptFile(session)); err != nil {
		t.Fatalf("no work prompt file for %s, so the argv path was not reached: %v", session, err)
	}

	if !strings.Contains(out, "WRONG before this launch") {
		t.Errorf("the pre-heal finding is not in the pass's own stream, which is the only thing the watch log tees:\n%s", out)
	}
	if !strings.Contains(out, hook) {
		t.Errorf("the finding in the pass's stream does not name the file to fix (%s):\n%s", hook, out)
	}
	if got := stderrish.String(); strings.Contains(got, "WRONG before this launch") {
		t.Errorf("the finding went to the backend's writer — os.Stderr in production, /dev/null under the loop:\n%s", got)
	}
	// lcode's scope note, asserted on this path too: the record is what was
	// wrong, not the healing. The wall is still re-stamped.
	if post := b.App.probeL3Hooks(repo, false); !post.CommitGuard {
		t.Errorf("the dispatched launch did not re-stamp the wall it reported: %s", post.CommitGuardDegraded)
	}
}
