package main

// ADR 0032 §1 rule 2, through the real process. The library half is pinned
// in internal/posse; this is the wiring, which is a SEPARATE declaration —
// `posse runtime probe` is a second arm of the same switch as `runtime
// check`, and a package that is perfectly tested is still unreachable if
// nothing routes to it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeProbeIsWiredAndRefusesWithoutHerdr(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "runtimes", "shell.yaml"), []byte("command: sh {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "RHQ_HOME="+home, "RHQ_HERDR_BIN=no-such-herdr-binary")
	run := func(args ...string) (string, error) {
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// Wired: the refusal it gives is the PROBE's, not the subcommand
	// dispatcher's. A `usage:` here would mean nothing routes to the probe.
	out, err := run("runtime", "probe", "shell")
	if err == nil {
		t.Errorf("no herdr, no probe — the command must exit non-zero:\n%s", out)
	}
	if strings.Contains(out, "usage: posse runtime") {
		t.Fatalf("`runtime probe` is not routed — the switch answered with usage:\n%s", out)
	}
	if !strings.Contains(out, "herdr") {
		t.Errorf("the refusal must name what is missing:\n%s", out)
	}
	// And it left no record: an absent record is the honest state for a
	// probe that never ran, and a file here would read as "measured".
	if _, statErr := os.Stat(filepath.Join(home, "state", "runtimes", "shell", "probe.json")); statErr == nil {
		t.Error("a refused probe wrote a record")
	}

	// The flag surface refuses what it does not know rather than swallowing
	// it — a typo'd --timeoutt that silently used the default would be a
	// probe timing out on a number the operator did not choose.
	if out, err := run("runtime", "probe", "shell", "--timeoutt", "9m"); err == nil || !strings.Contains(out, "unknown flag") {
		t.Errorf("an unknown probe flag must refuse: %v\n%s", err, out)
	}
	if out, err := run("runtime", "probe", "shell", "--timeout", "banana"); err == nil || !strings.Contains(out, "positive duration") {
		t.Errorf("a bad --timeout must refuse: %v\n%s", err, out)
	}
	if out, err := run("runtime", "probe"); err == nil || !strings.Contains(out, "usage: posse runtime check|probe") {
		t.Errorf("`runtime probe` with no name must print the usage naming both verbs: %v\n%s", err, out)
	}

	// The grid points at the probe, and the catalog does too — an onboarder
	// who never learns the command never learns the claim is conditional.
	if out, err := run("runtime", "check", "shell"); err != nil {
		t.Errorf("a profile whose only gaps are non-blocking must still exit 0: %v\n%s", err, out)
	} else if !strings.Contains(out, "posse runtime probe shell") || !strings.Contains(out, "ASSUMED") {
		t.Errorf("the grid must name the probe and the state it is in:\n%s", out)
	}
	if out, err := run("runtimes"); err != nil || !strings.Contains(out, "posse runtime probe <name>") {
		t.Errorf("the catalog must point at the probe: %v\n%s", err, out)
	}
	if out, err := run("--help"); err != nil || !strings.Contains(out, "posse runtime probe <name>") {
		t.Errorf("help must list the probe: %v\n%s", err, out)
	}
}
