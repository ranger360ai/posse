package main

// ranger-base-jfe5z, the WIRING half: `NewSessionOpts.ByHand` is what makes
// the load guard the fleet's and not the crew's, and the two CLI entry
// points that set it are single literals in main.go — exactly the kind of
// fact an in-package test on CreateSession cannot reach (a field that is
// APPLIED is not thereby REACHED). So this runs the real binary.
//
// The measurement is indirect on purpose, and it is what makes the test
// hermetic: `load_guard:` is set to a number this box is certainly over,
// and the launch is aimed at a directory that does not exist. "directory
// not found" is raised in planLaunch BELOW the load guard, so reaching it
// proves the guard let the launch past; a refusal would have ended the
// process at the guard with a different sentence and the dir would never
// have been stat'd. No herdr, no session, no cleanup.
//
// The control is the box's own reading: under the ceiling, this measures
// nothing at all and says so rather than passing.
//
// It must be a REAL binary, so mutation-grading it needs GOFLAGS, not the
// `go test -overlay` flag: buildRhq shells out to `go build`, a child
// toolchain process that never sees a flag passed to `go test`, and an
// overlay mutant of main.go then SURVIVES while looking measured.
// `GOFLAGS="-overlay=/abs/mutant.json" go test ./cmd/posse/ -run ...` is
// inherited by that child and kills it (measured, ranger-base-jfe5z).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestHandTypedLaunchesWarnPastTheLoadGuard(t *testing.T) {
	bin := buildRhq(t)
	live, err := posse.SysLoad1()
	if err != nil {
		t.Skipf("no load reading on this box (%v): nothing to measure", err)
	}
	const ceiling = "0.01"
	if live <= 0.01 {
		t.Skipf("this box reads %g, not over the test's %s ceiling: nothing was measured", live, ceiling)
	}
	// Where the launch is aimed: absent, so planLaunch's dir check is the
	// next refusal below the guard.
	missing := filepath.Join(t.TempDir(), "no-such-dir")

	run := func(t *testing.T, guard string, args ...string) (int, string) {
		t.Helper()
		home := t.TempDir()
		rhq := filepath.Join(home, "posse")
		if err := os.MkdirAll(rhq, 0o755); err != nil {
			t.Fatal(err)
		}
		cfg := "default_dir: " + missing + "\nload_guard: " + guard + "\n"
		if err := os.WriteFile(filepath.Join(rhq, "config.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, args...)
		cmd.Env = []string{"HOME=" + home, "RHQ_HOME=" + rhq, "PATH=/usr/bin:/bin"}
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		runErr := cmd.Run()
		code := 0
		if ee, ok := runErr.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if runErr != nil {
			t.Fatalf("%v: %v\nstdout %q\nstderr %q", args, runErr, out.String(), errb.String())
		}
		// STDERR only. The witness is a warning, and a warning on stdout is
		// in whatever the operator was piping the launch's output into.
		if strings.Contains(out.String(), "load guard") {
			t.Errorf("the witness belongs on stderr, not stdout: %q", out.String())
		}
		return code, errb.String()
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"posse new", []string{"new", "jfe5z-probe", "--dir", missing}},
		{"posse up", []string{"up", "jfe5z-probe"}}, // no --dir: default_dir is the missing one
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, said := run(t, ceiling, tc.args...)
			if code == 0 {
				t.Fatalf("the launch should have died on the missing dir:\n%s", said)
			}
			if !strings.Contains(said, "directory not found") {
				t.Errorf("the guard stopped the launch above the dir check — ByHand is not wired:\n%s", said)
			}
			if strings.Contains(said, "refusing to launch") {
				t.Errorf("a hand-typed launch got the fleet's refusal:\n%s", said)
			}
			if !strings.Contains(said, "load guard") {
				t.Errorf("the witness must still be printed — that is what makes the missing refusal safe:\n%s", said)
			}
			// The control arm on the same command: with the guard off the
			// witness is gone and the dir check is still the ending, which
			// is what proves the line above came from the guard firing.
			code, said = run(t, "0", tc.args...)
			if code == 0 || !strings.Contains(said, "directory not found") {
				t.Fatalf("control (guard off) did not reach the dir check: code=%d\n%s", code, said)
			}
			if strings.Contains(said, "load guard") {
				t.Errorf("control (guard off) still printed a witness:\n%s", said)
			}
		})
	}
}
