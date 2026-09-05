package posse

// ranger-base-hslbb: a process does NOT survive closing the workspace its own
// pane is in. Measured 2026-09-04 against herdr 0.8.2 on a scratch-HOME named
// session, three runs identical:
//
//	SELF=died  DETACHED=died  SETSID=survived
//
// That is the whole question under `posse relaunch <name> --no-land` typed
// inside the session it names. RelaunchSession's destructive half runs
// closeRecorded(m) and then recreateSession() in one critical section; in the
// self case m.Workspace is the caller's own workspace, so the close kills the
// process that must run the recreate — with the meta already unlinked and the
// name freed. The caller dies INSIDE the close call: its own `close rc=` line
// is never written.
//
// DETACHED=died is `nohup ... &`, which only ignores SIGHUP and leaves the
// child in the pane's process group. SETSID=survived is a child that called
// setsid(2) first — new session, no controlling terminal — which closed its
// own workspace, got `{"type":"ok"}` back and completed the create. So the
// death is the process group going down with the pane, and refusing the self
// case is not the only fix available to relaunch.
//
// scripts/verify-self-close.sh is the live pin. This file pins that the
// script still carries its fleet fences and still asserts the controls that
// make the verdict evidence rather than a broken rig.
//
// Live run (scratch HOME and scratch --session; the fleet is never addressed):
//
//	scripts/verify-self-close.sh
//	RHQ_LIVE_SELFCLOSE=1 go test ./internal/posse -run TestQASelfCloseScriptPassesAgainstAScratchServer -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func selfCloseScript(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "scripts", "verify-self-close.sh")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestQASelfCloseScriptKeepsItsFleetFences(t *testing.T) {
	t.Parallel()
	s := selfCloseScript(t)
	// Every arm in this script closes a workspace, so the fences are the
	// reason it is safe to run at all. Losing one is not a style regression.
	for _, needle := range []string{
		"Never aims at the fleet default server",
		// The word "FLEET_SOCK" alone is undiscriminating: it survives in the
		// assignment and in prose while the refusal that uses it is deleted
		// (measured — that mutant stayed green). Pin the comparison itself.
		`[ "$sock" != "$SESS_SOCK" ]`,
		`[ "$sock" = "$FLEET_SOCK" ]`,
		"unset HERDR_ENV HERDR_SOCKET_PATH",
		`HHOME=$(mktemp -d /private/tmp/pselfclose.XXXXXX)`,
		`env HOME="$HHOME"`,
		"REFUSING:",
		"session delete",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("verify-self-close.sh no longer contains %q — the probe can aim at the fleet or leak a session", needle)
		}
	}
	// A scratch SOCKET alone is not isolation: it restores the default
	// session's whole layout and re-runs every workspace's command. The
	// scratch HOME is the fence that makes the server come up empty, and
	// this assertion is what proves it did.
	if !strings.Contains(s, "scratch-home-server-is-empty") {
		t.Error("verify-self-close.sh no longer asserts the scratch server came up empty — a scratch socket alone would restore the fleet")
	}
}

func TestQASelfCloseScriptKeepsTheControlsThatMakeTheVerdictEvidence(t *testing.T) {
	t.Parallel()
	s := selfCloseScript(t)
	// Without these, a missing marker 2 in the self arm is a broken rig, a
	// pane that never ran, a close that no-opped, or a wait that was too
	// short — every one of which reads exactly like the defect.
	for _, needle := range []string{
		// the rig can reach marker 2 at all, from a pane, closing someone else
		"control-outlives-closing-another-workspace",
		"control-recreate-landed",
		// the self arm's script actually ran
		"self-pane-ran",
		// the close was not a no-op, so the arm measured something
		"self-close-actually-happened",
		// absence is a dead writer, not a slow one
		"self-marker-2-absent-is-final-not-slow",
		// the setsid child started, so its marker 2 is an answer not a shrug
		"setsid-arm-inner-started",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("verify-self-close.sh no longer asserts %q — the verdict stops being evidence without it", needle)
		}
	}
}

func TestQASelfCloseScriptPassesAgainstAScratchServer(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_SELFCLOSE") == "" {
		t.Skip("set RHQ_LIVE_SELFCLOSE=1 to run scripts/verify-self-close.sh against a scratch herdr session")
	}
	script := filepath.Join("..", "..", "scripts", "verify-self-close.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "HERDR_SESSION=", "HERDR_SOCKET_PATH=", "HERDR_ENV=")
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("verify-self-close.sh: %v", err)
	}
	if !strings.Contains(string(out), "verify-self-close: PASS") {
		t.Fatalf("script exited 0 without PASS:\n%s", out)
	}
	// The measured sentence, not merely a green run. relaunch's self case is
	// decided by these three words; a herdr that changes any of them should
	// bring someone back to this bead rather than silently widening what
	// `--no-land` is allowed to do.
	if !strings.Contains(string(out), "verify-self-close: SELF=died DETACHED=died SETSID=survived") {
		t.Fatalf("the self-close verdict moved off what ranger-base-hslbb measured (herdr 0.8.2, 2026-09-04) — re-read relaunch's self case:\n%s", out)
	}
}
