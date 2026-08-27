package main

// ADR 0012 D4, rangerhq-2v2s: the operator-facing half of the yaml v2 keys.
// The unit tests prove the parse and the parity lines; this one proves the
// two surfaces an onboarder actually reads — `posse runtimes` and `posse
// runtime check` — through the real process, which is also the only place
// the unknown-key warning is observable as a thing that reaches a terminal.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const scratchProfile = `command: testcli {model} {skills} -a never --rules="$(cat {file})"
model_flag: -c model=%s
model_standard: sol
skills_cwd: true
self_sandbox: true
project_config: .testcli/config.toml
skils_flag: --oops
`

func TestRuntimesListsAYamlV2Profile(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "runtimes", "test.yaml"), []byte(scratchProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "RHQ_HOME="+home, "RHQ_HERDR_BIN=no-such-herdr-binary")

	cmd := exec.Command(bin, "runtimes")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse runtimes: %v\n%s", err, out)
	}
	got := string(out)
	// The catalog line: the profile, its tier dial (fast falls back to
	// standard; strong really is unmapped and must say so), and the template.
	for _, want := range []string{
		"test     template-only",
		"tiers: standard=sol fast=sol · UNMAPPED: strong",
		`testcli {model} {skills} -a never --rules="$(cat {file})"`,
		// The typo'd key is named on the same run, rather than dropped in
		// silence — the whole point of the key list existing.
		"declares skils_flag:",
		"known keys:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("posse runtimes missing %q in:\n%s", want, got)
		}
	}

	check := exec.Command(bin, "runtime", "check", "test")
	check.Env = env
	cout, err := check.CombinedOutput()
	if err != nil {
		t.Fatalf("posse runtime check test: %v\n%s", err, cout)
	}
	grid := string(cout)
	for _, want := range []string{
		"standard=sol fast=sol (rendered with -c model=%s)",
		"skills_flag: OR skills_cwd:, self_sandbox:, project_config:",
	} {
		if !strings.Contains(grid, want) {
			t.Errorf("posse runtime check missing %q in:\n%s", want, grid)
		}
	}
}
