package main

// QA, ranger-base-w1cv4 — a runtimes/<builtin>.yaml declaring an ADR 0021
// Decision 2 mechanism key (e.g. unattended:) refuses the LOAD
// (internal/posse/runtime.go's overlayBuiltin). That refusal was loud on
// `posse runtime check <name>` and silent everywhere else: `posse runtimes`
// omitted the runtime's row and exited 0 with nothing on stderr, and
// `posse gates <persona>` silently dropped the row from its per-runtime
// catalog table whenever the refused runtime was not the persona's own.
// The Die error already carries the key and the why; these two enumerating
// callers just dropped it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeRefusingOverlay drops a runtimes/claude.yaml declaring the
// unattended: mechanism key — one of the builtinMechanismKeys (ADR 0021
// Decision 2) — so LoadRuntime("claude") refuses in home.
func writeRefusingOverlay(t *testing.T, home string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "runtimes", "claude.yaml"), []byte("unattended: --yolo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The bead's own repro on `posse runtimes`: a refusing overlay must be
// named, with the key and the why, and the command must not exit 0 over a
// profile it could not load.
func TestQARuntimesNamesARefusingOverlayAndExitsNonZero(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	writeRefusingOverlay(t, home)

	cmd := exec.Command(bin, "runtimes")
	cmd.Env = []string{"HOME=" + t.TempDir(), "RHQ_HOME=" + home, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse runtimes: %v\n%s", err, out)
	}
	if code == 0 {
		t.Errorf("`posse runtimes` exited 0 over a runtimes/claude.yaml that refuses the load:\n%s", out)
	}
	if !strings.Contains(string(out), "claude") || !strings.Contains(string(out), "unattended:") {
		t.Errorf("the refused runtime and its key are not named:\n%s", out)
	}
	if !strings.Contains(string(out), "ADR 0021 Decision 2") {
		t.Errorf("the why is missing from the listing:\n%s", out)
	}
	// codex and grok load clean off the same catalog and must still get
	// their rows — one refused profile must not blank the rest.
	for _, want := range []string{"codex", "grok"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("an unrelated built-in %q lost its row over claude's refusal:\n%s", want, out)
		}
	}
}

// The control: with no overlay at all, `posse runtimes` stays green.
func TestQARuntimesNoOverlayStaysGreen(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()

	cmd := exec.Command(bin, "runtimes")
	cmd.Env = []string{"HOME=" + t.TempDir(), "RHQ_HOME=" + home, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse runtimes: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "REFUSED") {
		t.Errorf("nothing refused here, so nothing should read REFUSED:\n%s", out)
	}
}

// `posse gates <persona>` walks the same catalog for its per-runtime parity
// table. A refusing overlay on a runtime the persona does NOT launch on
// used to vanish from that table with the persona's own report reading
// clean — the wall of green rangerhq-qz51 already named for the persona's
// OWN unresolvable runtime, recurring here for every OTHER runtime in the
// catalog.
func TestQAGatesNamesARefusingOverlayOnAnUnrelatedRuntime(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	writeRefusingOverlay(t, home)
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: builder\ndescription: builder\nruntime: codex\n---\nbuilder\n"
	if err := os.WriteFile(filepath.Join(home, "agents", "builder.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}

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
	// The persona launches fine on codex — an unrelated runtime's broken
	// overlay must not fail ITS report.
	if code != 0 {
		t.Errorf("`posse gates builder` exited %d though builder's own runtime (codex) resolves fine:\n%s", code, out)
	}
	if !strings.Contains(string(out), "claude") || !strings.Contains(string(out), "unattended:") {
		t.Errorf("the refused claude overlay is not named in the report:\n%s", out)
	}
	if !strings.Contains(string(out), "codex @ ") {
		t.Errorf("codex's own parity row is missing:\n%s", out)
	}
}

// When the refusing overlay IS the persona's own runtime, the existing
// rangerhq-qz51 line already names it and exits non-zero — the catalog
// loop must not repeat the same runtime a second time.
func TestQAGatesRefusingOwnRuntimeNamesItOnceAndExitsNonZero(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	writeRefusingOverlay(t, home)
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: builder\ndescription: builder\nruntime: claude\n---\nbuilder\n"
	if err := os.WriteFile(filepath.Join(home, "agents", "builder.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}

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
	if code == 0 {
		t.Errorf("`posse gates builder` exited 0 though builder's own runtime refuses the load:\n%s", out)
	}
	if got := strings.Count(string(out), "unattended:"); got != 1 {
		t.Errorf("the refusal must be named exactly once, got %d:\n%s", got, out)
	}
}
