package main

// ranger-base-f5jbs: the exit code, through the real process.
//
// The unit arms in internal/posse prove the walk; this one proves the thing
// a roster row actually consumes. An exit code asserted only inside the
// package is an exit code nothing has ever read from a shell — and the whole
// defect here is a correct line that reached no reader.
//
// HERMETIC BY PATH: skills_verified: drift is read off the CLIs installed on
// the box, and the built-ins are in the walk. Each arm runs with PATH
// pointing at a scratch bin dir alone, so the built-ins answer no --version
// (unmeasured) and the planted profile is the only subject.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// plantCLI writes a fake CLI that prints version on its first stdout line,
// which is what ProbeCLIVersion reads.
func plantCLI(t *testing.T, bin, name, version string) {
	t.Helper()
	p := filepath.Join(bin, name)
	if err := posse.WriteExecutable(p, []byte("#!/bin/sh\necho '"+version+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func driftRun(t *testing.T, bin, home, scratchBin string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, "runtime", "drift")
	cmd.Env = []string{
		"HOME=" + home,
		"RHQ_HOME=" + home,
		"PATH=" + scratchBin,
		"RHQ_HERDR_BIN=no-such-herdr-binary",
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse runtime drift: %v\n%s", err, out)
	}
	return string(out), code
}

func TestRuntimeDriftExitsNonzeroOnADriftAndZeroOnNone(t *testing.T) {
	posseBin := buildRhq(t)
	home := t.TempDir()
	scratchBin := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "runtimes", "driftcli.yaml"),
		[]byte("command: driftcli {skills} --sys {file}\nskills_cwd: true\nskills_verified: driftcli 1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Nothing installed at all: every profile in the walk is a profile this
	// box cannot measure, and the status for that is 2. A schedule that read
	// it as green would be a green light on an empty room.
	out, code := driftRun(t, posseBin, home, scratchBin)
	if code != 2 {
		t.Errorf("with no CLI installed, `posse runtime drift` must exit 2 (nothing measured), got %d:\n%s", code, out)
	}
	if !strings.Contains(out, "NOTHING WAS MEASURED") {
		t.Errorf("exit 2 must come with the sentence that explains it:\n%s", out)
	}

	// Installed and moved.
	plantCLI(t, scratchBin, "driftcli", "driftcli 2.0")
	out, code = driftRun(t, posseBin, home, scratchBin)
	if code != 1 {
		t.Errorf("a drifted profile must exit 1, got %d:\n%s", code, out)
	}
	for _, want := range []string{"drift:", "driftcli 1.0", "driftcli 2.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("the drift line must name both versions: missing %q:\n%s", want, out)
		}
	}

	// Installed and matching.
	plantCLI(t, scratchBin, "driftcli", "driftcli 1.0")
	out, code = driftRun(t, posseBin, home, scratchBin)
	if code != 0 {
		t.Errorf("a matching profile must exit 0, got %d:\n%s", code, out)
	}
	if strings.Contains(out, "drift:") {
		t.Errorf("a clean walk must print no drift line:\n%s", out)
	}

	// And the sub-verb takes no name — a runtime argument here would make it
	// a third per-runtime display surface, which is what already existed.
	named := exec.Command(posseBin, "runtime", "drift", "driftcli")
	named.Env = []string{"HOME=" + home, "RHQ_HOME=" + home, "PATH=" + scratchBin, "RHQ_HERDR_BIN=no-such-herdr-binary"}
	nout, err := named.CombinedOutput()
	if err == nil || !strings.Contains(string(nout), "takes no name") {
		t.Errorf("`posse runtime drift <name>` must refuse and say why, got %v:\n%s", err, nout)
	}
}
