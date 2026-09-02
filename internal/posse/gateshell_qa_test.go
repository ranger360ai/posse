package posse

// QA pin written verifying ranger-base-92n5p's close under ranger-base-vd5nl.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// ─── ranger-base-o5fpa ───────────────────────────────────────────────────────

// The fourth reader of the class ranger-base-gs9r opened: the launch path
// opens a non-regular file and open(2) never returns. ranger-base-92n5p
// fixed installHook and hookInstalled, the folded ranger-base-92rt fixed
// projectConfigTrustFile, ranger-base-fvfve is filed for identityRedirectTarget
// — and isGateWrapper (gates.go, os.Open with no type check) is still open, in
// the same file 92n5p was fixing.
//
// REACHED ON EVERY LAUNCH: PrepareGatesWrap -> App.RenderGates ->
// renderGateShell -> realShell -> isGateWrapper, from $SHELL (which is only
// stat'ed for !IsDir, and a FIFO passes that) and from the PATH search for
// zsh/bash. A named pipe at either wedges the render with nothing printed.
//
// NOT REACHABLE FROM A CAGED SESSION TODAY, which is why the bead is P3: every
// present entry of a live caged session's PATH was measured refused to it. The
// setter is an uncaged process, the operator's own shell, or a container
// image's own PATH — the same accident shape ranger-base-r5wpk is about.
//
// The controls run first, through the same call, so a BLOCKED verdict is the
// file type and not the harness. isGateWrapper answers false on any error
// already, so a stat before the open changes no other answer.
//
// Un-skip when ranger-base-o5fpa lands.
func TestQAAFifoOnThePathMustNotWedgeTheGateShellRender(t *testing.T) {
	t.Parallel()
	t.Skip("ranger-base-o5fpa: isGateWrapper opens a candidate shell before any type check")

	dir := t.TempDir()
	ask := func(name, p string, budget time.Duration) {
		t.Helper()
		done := make(chan bool, 1)
		go func() { done <- isGateWrapper(p) }()
		select {
		case v := <-done:
			t.Logf("%s: returned %v", name, v)
		case <-time.After(budget):
			t.Errorf("%s: BLOCKED past %s — open(2) on a non-regular file never returned, so every launch resolving this candidate hangs with nothing printed", name, budget)
		}
	}

	// Controls first, in this rig, through this call.
	reg := filepath.Join(dir, "zsh-regular")
	if err := os.WriteFile(reg, []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ask("an ordinary executable candidate", reg, 10*time.Second)
	ask("an absent candidate", filepath.Join(dir, "nope"), 10*time.Second)

	fifo := filepath.Join(dir, "zsh")
	if err := exec.Command("mkfifo", fifo).Run(); err != nil {
		t.Skipf("mkfifo unavailable, so the arm this test exists for cannot run: %v", err)
	}
	if err := os.Chmod(fifo, 0o755); err != nil {
		t.Fatal(err)
	}
	// Fixture guard: if this is not actually a pipe, the arm below measures
	// nothing and would pass for the wrong reason.
	if fi, err := os.Lstat(fifo); err != nil || fi.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("the fixture is not a named pipe (%v, %v), so this arm pins nothing", fi, err)
	}
	ask("a 0755 named pipe named zsh", fifo, 10*time.Second)
}
