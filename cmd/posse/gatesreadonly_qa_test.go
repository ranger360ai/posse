package main

// QA, ranger-base-zbg8o (escaped from ranger-base-151nr) — `posse gates
// <persona>` RE-RENDERS the shim dir as a side effect of being asked to
// report it, so it was unrunnable from any caged seat, including for the
// seat's OWN persona, and printed no gate rows at all:
//
//	$ posse gates <persona>
//	posse: unlinkat ~/.config/posse/state/gates/<persona>/bin/pkill: operation not permitted
//
// It failed CLOSED, with no rows, on a box where the gates were in fact
// correct — and the verify step ranger-base-151nr was assigned was
// `posse gates <persona> | grep -E 'pkill|killall'` -> two check rows, handed
// to exactly such a seat. That verify had to read the rendered shims by hand
// instead. Reported here rather than written up, because the render is by
// design (gateshellinherit_qa_test.go runs this command to PRODUCE the shim
// dir): what changed is that a render which cannot write degrades to a read.
//
// Driven through the built binary, like the other two pins on this command:
// both halves — the fallback and the exit status — live in the `gates` case.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// roGatesHome writes a persona denying pkill/killall the way every crew PID
// does (operator ruling 2026-09-03, ranger-base-jjx19) and returns RHQ_HOME.
func roGatesHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: builder\ndescription: builder\nruntime: claude\n" +
		"deny: [\"Bash(pkill:*)\", \"Bash(killall:*)\"]\n---\nbuilder\n"
	if err := os.WriteFile(filepath.Join(home, "agents", "builder.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func roGatesRun(t *testing.T, bin, home string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, "gates", "builder")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"HOME=" + filepath.Join(home, "h"), "RHQ_HOME=" + home, "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse gates builder: %v\n%s", err, out)
	}
	return string(out), code
}

// sealGatesBin makes the rendered bin dir unwritable, which is what a cage
// does to the whole gates dir: the re-render's first unlink fails. Restored
// on cleanup or t.TempDir cannot remove it.
func sealGatesBin(t *testing.T, home string) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the mode bits this arm needs; the cage does not")
	}
	binDir := filepath.Join(home, "state", "gates", "builder", "bin")
	if err := os.Chmod(binDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(binDir, 0o755) })
	return binDir
}

// The bead's repro. Three things, because a report that merely stops dying
// is not the fix: it has to still NAME the shims (that is what the verify
// recipe greps for) and still say it did not re-render (a stale dir read as
// current is the failure this degradation could introduce).
func TestQAGatesReportsFromDiskWhenItCannotRender(t *testing.T) {
	bin := buildRhq(t)
	home := roGatesHome(t)

	// The control, and it is load-bearing twice: it renders the dir the arm
	// below reads, and it proves the warning is not a line the report always
	// carries. A rig that cannot fail pins nothing.
	first, code := roGatesRun(t, bin, home)
	if code != 0 {
		t.Fatalf("the writable render exited %d, so the arm below would measure the wrong failure:\n%s", code, first)
	}
	if strings.Contains(first, "NOT re-rendered") {
		t.Fatalf("a writable render said it did not re-render — the warning below would pass for free:\n%s", first)
	}

	sealGatesBin(t, home)
	out, code := roGatesRun(t, bin, home)
	if code != 0 {
		t.Errorf("`posse gates builder` exited %d when the gates dir was read-only — a caged seat cannot run its own verify:\n%s", code, out)
	}
	// What the verify recipe greps for.
	for _, want := range []string{"bin/pkill", "bin/killall"} {
		if !strings.Contains(out, want) {
			t.Errorf("the read-only report does not name %s, so `posse gates <persona> | grep -E 'pkill|killall'` still answers nothing:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "NOT re-rendered") {
		t.Errorf("the report read the shims off disk without saying so — a stale dir would read as current:\n%s", out)
	}
}

// ...and the read-only path can still say NO. A degradation that reported
// every unwritable gates dir as fine would be worse than the crash it
// replaces: the PID denies two verbs, so a dir carrying one shim is a deny
// unrealized at L1, and that is a verdict an exit code has to carry.
func TestQAGatesReadOnlyStillFailsOnAnUnrealizedDeny(t *testing.T) {
	bin := buildRhq(t)
	home := roGatesHome(t)
	if _, code := roGatesRun(t, bin, home); code != 0 {
		t.Fatalf("the writable render exited %d", code)
	}
	binDir := filepath.Join(home, "state", "gates", "builder", "bin")
	if err := os.Remove(filepath.Join(binDir, "killall")); err != nil {
		t.Fatal(err)
	}
	sealGatesBin(t, home)

	out, code := roGatesRun(t, bin, home)
	if code == 0 {
		t.Errorf("`posse gates builder` exited 0 with Bash(killall:*) carrying no shim — a wall of green over a deny nothing realizes:\n%s", out)
	}
	if !strings.Contains(out, "NO SHIM") {
		t.Errorf("the missing shim is not named anywhere in the report:\n%s", out)
	}
	if !strings.Contains(out, "bin/pkill") {
		t.Errorf("the shim that IS on disk stopped being reported, so the arm above is measuring a blank report:\n%s", out)
	}
}
