//go:build posse_arm2

package posse

// QA pins for the Enter-screen dismissal in scripts/verify-ghost-composer.sh
// (ranger-base-trb9z). The rig SCRIPT is the subject here;
// ghostcomposer_qa_test.go next door is the posse-side reading the rig was
// written to measure, and the two share nothing but a name.
//
// WHAT WAS WRONG. That script drives a real claude in a scratch herdr pane and
// waits for a composer, pressing `enter` blindly a bounded number of times on
// the way to clear claude's one-key setup choosers. On 2026-09-11 from an
// uncaged shell (claude 2.1.268, herdr 0.8.2) the run printed `setup choosers
// dismissed: 4 / FAIL claude-reached-a-live-composer` and the capture was the
// screen claude draws AFTER a completed OAuth login and BEFORE the composer:
// "Login successful. Press Enter to continue...". The four blind presses had
// gone to the choosers; the screen that arrives last got none. The fix matches
// screens that name Enter on their own text and gives them their own budget.
//
// WHY THE PIN IS HERE AND NOT IN A MAKE TARGET. The script as a whole needs an
// UNCAGED shell and a live claude -- scripts/verify-box.sh lists it under
// UNTARGETED for exactly that -- so nothing in this repo has ever run a line of
// it. The dismissal is the half that does not need any of that: a text match
// over a captured screen. `--self-test` drives it with no herdr, no claude, no
// pane and no filesystem, and this file is its trigger. A self-test nothing
// runs is a line in a comment, i.e. a thing to remember.
//
// THE FIXTURE IS THE BEAD, NOT THE CAPTURE. The .ansi captures from the failing
// run were kept under a session scratchpad and that scratchpad is gone; what
// survived is the bead, which quotes the screen. So the self-test screens carry
// the quoted lines verbatim and stand in for the banner art, which nothing
// matches on. A real capture, if one is taken again, belongs in the same arms.
//
// Three claims, because they fail differently:
//
//  1. the arms still pass, and there are still four of them;
//  2. the matcher can still say NO -- a pattern that no longer matches the
//     screen must red, or the arms are a spelling exercise;
//  3. --self-test still needs no live box, or this file is a skip on every
//     machine that does not have one and claim 1 is unmeasured where it
//     matters most.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gcScript resolves the rig, skipping if this checkout does not carry it (a
// tarball, a worktree pruned for a build).
func gcScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../scripts/verify-ghost-composer.sh")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("ghost-composer rig not present: %v", err)
	}
	return p
}

// gcRun runs the rig and returns its output and exit code.
func gcRun(t *testing.T, script string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(script, args...)
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s: %v\n%s", script, err, out)
	}
	return string(out), code
}

// TestGhostComposerDismissesTheLoginInterstitial is the claim itself: the
// screen ranger-base-trb9z stopped at is matched, it stays matched under the
// blank rows a pane read leaves beneath a drawn screen, and neither a live
// composer nor an empty read is.
//
// The name list and the arm floor are both asserted because they fail
// differently: a renamed arm is a rewrite to read, a missing one is coverage
// that quietly left.
func TestGhostComposerDismissesTheLoginInterstitial(t *testing.T) {
	t.Parallel()
	out, code := gcRun(t, gcScript(t), "--self-test")
	if code != 0 {
		t.Fatalf("rig self-test exited %d:\n%s", code, out)
	}
	for _, want := range []string{
		"self-test PASS: the login interstitial is matched",
		"self-test PASS: and is still matched under trailing blank rows",
		"self-test PASS: a live composer is not matched",
		"self-test PASS: a pane read that came back empty is not matched",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("self-test no longer reports %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "self-test PASS: "); n < 4 {
		t.Fatalf("self-test reported only %d passing arms, want >= 4:\n%s", n, out)
	}
}

// TestGhostComposerSelfTestStillSaysNo mutates the dismissal list to a pattern
// the captured screen does not carry -- the shape of the defect, a rig that
// waits for a composer behind a screen it does not recognise -- and demands
// the self-test red. Without this, four arms that all answered "no match"
// would still print four passes for the two negative ones and the pin would
// hold over a matcher that matches nothing.
func TestGhostComposerSelfTestStillSaysNo(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(gcScript(t))
	if err != nil {
		t.Fatalf("read rig: %v", err)
	}
	const head = "ENTER_SCREENS=\"Press Enter to continue\""
	if !strings.Contains(string(src), head) {
		t.Fatalf("ENTER_SCREENS not found as written; this pin's mutation no longer applies")
	}
	broken := strings.Replace(string(src), head,
		"ENTER_SCREENS=\"Press Any Key\"   # ranger-base-trb9z: matches no screen", 1)

	mutant := filepath.Join(t.TempDir(), "ghost-mutant.sh")
	if err := WriteExecutable(mutant, []byte(broken), 0o755); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	out, code := gcRun(t, mutant, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with a list that matches no screen:\n%s", out)
	}
	if strings.Contains(out, "self-test PASS: the login interstitial is matched") {
		t.Fatalf("the interstitial arm passed over a list that cannot match it:\n%s", out)
	}
}

// TestGhostComposerSelfTestNeedsNoLiveBox is the property that lets the two
// pins above run at all, and it is not free: the rig resolves herdr and claude
// and refuses with exit 2 without them, and its scratch root is a darwin
// /private/tmp path. --self-test must reach its arms before any of that, or
// this file is a skip on every machine that does not already have the live box
// -- CI, linux, and every caged seat. Pointing both variables at paths that do
// not exist is the guard's own failing condition, handed to it deliberately.
func TestGhostComposerSelfTestNeedsNoLiveBox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := exec.Command(gcScript(t), "--self-test")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HERDR="+filepath.Join(dir, "no-such-herdr"),
		"CLAUDE="+filepath.Join(dir, "no-such-claude"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("self-test refused with no herdr and no claude: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "self-test PASS: ") {
		t.Fatalf("self-test reported no arms with no herdr and no claude:\n%s", out)
	}
}
