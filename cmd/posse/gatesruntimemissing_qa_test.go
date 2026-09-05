package main

// QA, rangerhq-qz51 — `posse gates <persona>` silently omitted a persona's
// unresolvable runtime and exited 0. The parity table walks the CATALOG
// (a.ListRuntimes), so a `runtime:` naming neither a built-in nor a
// runtimes/<name>.yaml produced no row at all rather than a row saying so:
// what INSTALL.md §7 sends an operator to read before their first dispatch
// returned a wall of green for a persona that cannot launch.
//
// Driven through the built binary rather than against the renderer, because
// the defect is the command's: both halves of it (the omission and the exit
// status) live in the `gates` case, not in anything under internal/.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gatesQAHome writes a persona naming runtime rt and returns the RHQ_HOME.
func gatesQAHome(t *testing.T, rt string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: builder\ndescription: builder\nruntime: " + rt + "\n---\nbuilder\n"
	if err := os.WriteFile(filepath.Join(home, "agents", "builder.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

// gatesQARun runs `posse gates builder` from a scratch cwd — the report is
// computed for the caller's directory, and this test is about the persona.
func gatesQARun(t *testing.T, bin, home string) (string, int) {
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

// The bead's own repro. Two things are asserted because the bead asked for
// two: the name is IN the output (an operator reading the report learns
// which runtime is missing), and the status is non-zero (a script or an
// operator reading only the exit code is not told "cleared to dispatch").
func TestQAGatesNamesUnresolvableRuntimeAndExitsNonZero(t *testing.T) {
	bin := buildRhq(t)
	out, code := gatesQARun(t, bin, gatesQAHome(t, "codex-local"))
	if code == 0 {
		t.Errorf("`posse gates builder` exited 0 for a persona whose runtime resolves to nothing — INSTALL.md §7 reads this before a first dispatch:\n%s", out)
	}
	if !strings.Contains(out, "codex-local") {
		t.Errorf("the unresolvable runtime is not named anywhere in the report:\n%s", out)
	}
	if !strings.Contains(out, "cannot launch") {
		t.Errorf("the report names the runtime but never says the persona cannot launch on it:\n%s", out)
	}
}

// The control, and the arm that makes the one above mean something: with the
// yaml present the SAME persona gets its row and exits 0. Without it a `gates`
// that exited 1 on every persona alive would pass the test above.
func TestQAGatesResolvableRuntimeRowsAndExitsZero(t *testing.T) {
	bin := buildRhq(t)
	home := gatesQAHome(t, "codex-local")
	if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "runtimes", "codex-local.yaml"), []byte("command: codex-local exec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := gatesQARun(t, bin, home)
	if code != 0 {
		t.Errorf("`posse gates builder` exited %d with the persona's runtime resolvable:\n%s", code, out)
	}
	if !strings.Contains(out, "codex-local @ ") {
		t.Errorf("a resolvable runtime must get its own parity row:\n%s", out)
	}
	if strings.Contains(out, "cannot launch") {
		t.Errorf("nothing may say this persona cannot launch once its runtime resolves:\n%s", out)
	}
}

// A built-in runtime is resolved by name and not by a runtimes/*.yaml, so it
// exercises the other half of LoadRuntime's fork — the default persona shape,
// which must stay silent and green.
func TestQAGatesBuiltinRuntimeStaysGreen(t *testing.T) {
	bin := buildRhq(t)
	out, code := gatesQARun(t, bin, gatesQAHome(t, "claude"))
	if code != 0 {
		t.Errorf("`posse gates builder` exited %d on runtime: claude:\n%s", code, out)
	}
	if strings.Contains(out, "cannot launch") {
		t.Errorf("a built-in runtime must not be reported unresolvable:\n%s", out)
	}
}

// FINDING 1 of ranger-base-xndgk (verifying this bead's own close). The exit
// is at the END of the case rather than the top for a reason the code states
// out loud — "the gates dir, the gate shell, the shims and the refusals log
// are true whatever the runtime is, so the report still prints in full" — and
// nothing held it. The arm above asserts only that the runtime is NAMED and
// the status is non-zero, and both are true of a report that stops two lines
// in: moving the `os.Exit(1)` up to immediately after the "cannot launch"
// line leaves all three pins in this file green over three lines of output.
//
// So the claim is asserted as what it is, an ORDERING: every section of the
// report that does not depend on the runtime must still be there, and must
// be BELOW the warning. Below and not merely present, because "present" is
// satisfied by a report that prints the tail and then discovers the runtime.
func TestQAGatesUnresolvableRuntimeStillPrintsTheWholeReport(t *testing.T) {
	bin := buildRhq(t)
	out, code := gatesQARun(t, bin, gatesQAHome(t, "codex-local"))
	if code == 0 {
		t.Fatalf("fixture: this arm is the unresolvable-runtime one and must exit non-zero:\n%s", out)
	}
	cut := strings.Index(out, "cannot launch")
	if cut < 0 {
		t.Fatalf("fixture: the report never says the persona cannot launch, so there is no exit point to measure the tail against:\n%s", out)
	}
	// One row per runtime in the CATALOG, then the four runtime-independent
	// sections. Each is a line an operator reads off `posse gates` and none
	// of them is a fact about the persona's own runtime.
	for _, section := range []struct{ want, why string }{
		{"claude @ ", "the parity table's built-in rows"},
		{"codex @ ", "the parity table's built-in rows"},
		{"grok @ ", "the parity table's built-in rows"},
		{"/state/gates/builder", "the gates dir"},
		{"gate shell ", "the gate shell (ADR 0009)"},
		{"no shell-verb denies", "the shims line"},
		{"refusals.log", "the refusals log"},
	} {
		i := strings.Index(out, section.want)
		if i < 0 {
			t.Errorf("%s (%q) is missing from the report — a persona whose runtime does not resolve still has a gates dir, a gate shell, shims and a refusals log, and `posse gates` exits AFTER printing them:\n%s", section.why, section.want, out)
			continue
		}
		if i < cut {
			t.Errorf("%s (%q) prints ABOVE the runtime warning — the warning is reported where the runtime is read and exited on at the end of the case, so every section below it is below it:\n%s", section.why, section.want, out)
		}
	}
}
