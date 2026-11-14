package main

// QA, ranger-base-apf87 — THE BINARY MUST ANSWER `posse gates render-probe`,
// and that is a pin rather than a style note because of WHO CALLS IT.
//
// ADR 0092 D4 amendment 3 has `posse upgrade` ask this verb of two binaries
// around its swap, and the process doing the asking is the OLD one. So the
// verb is part of the interface every FUTURE binary owes the one installed
// today: drop it, and `upgradeVerdict` reads the new binary's answer as "not
// asked" and rolls the install back — correctly, by its own rule, and
// forever, because each rollback leaves the old binary in place to ask again.
// A release that removed the verb would be uninstallable through the gate
// that is supposed to install it, and nothing else in the tree would say so.
//
// MEASURED 2026-11-13 at 34bbe053, the commit before the verb existed:
// `posse gates render-probe` on a binary without it prints `posse: no such
// agent: render-probe` and exits 1 — the gates arm falls through its
// sub-verb ladder to LoadAgent. That is the SAME exit code the verb uses for
// "a render does not refuse", which is why the reading is a mark on stdout
// (posse.RenderProbeMark) and not the exit code, and why this pin asserts
// the mark.
//
// It runs the REAL binary rather than reading main.go for the string: an arm
// that greps the source would stay green over a verb that is spelled in the
// switch and dies before it prints anything.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestQAGatesRenderProbeVerbIsInTheBinary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	bin := buildRhq(t)

	home := t.TempDir()
	repo := filepath.Join(home, "checkout")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}

	cmd := exec.Command(bin, "gates", "render-probe", repo)
	cmd.Env = append(os.Environ(), "HOME="+home, "RHQ_PERSONA=")
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse gates render-probe: %v\n%s", err, out)
	}

	if code != 0 {
		t.Fatalf("`posse gates render-probe <repo>` exited %d over this binary's own renders, want 0:\n%s", code, out)
	}
	if !strings.Contains(string(out), posse.RenderProbeMark) {
		t.Fatalf("the output carries no %q line, so `posse upgrade` would read this binary as one WITHOUT the verb (ADR 0092 D4 amendment 3):\n%s", posse.RenderProbeMark, out)
	}
	// Read-only by contract: the verb must not have installed the wall it
	// was asked to describe.
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", slot)); err == nil {
			t.Errorf("render-probe installed a %s hook — the verb writes no hook", slot)
		}
	}

	// And the verb is named in the usage line the bare `posse gates` prints,
	// which is the only place a delegate or an operator would find it.
	usage := exec.Command(bin, "gates")
	usage.Env = append(os.Environ(), "HOME="+home)
	uout, _ := usage.CombinedOutput()
	if !strings.Contains(string(uout), "render-probe") {
		t.Errorf("`posse gates` usage does not name render-probe:\n%s", uout)
	}
}
