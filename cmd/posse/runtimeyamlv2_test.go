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
	// EXIT 1, and that is the contract as of rangerhq-tr8k: this profile
	// names a CLI that is not installed, a herdr that cannot be asked, and a
	// key nothing reads. A `check` that reported all three and exited 0
	// would be the green-while-broken class the command exists to end.
	if err == nil {
		t.Errorf("posse runtime check on a profile with blocking gaps must exit non-zero:\n%s", cout)
	}
	grid := string(cout)
	for _, want := range []string{
		"standard=sol fast=sol (rendered with -c model=%s)",
		// The onboarding footer names the declarable keys. Asserted key by
		// key rather than as one wrapped literal: where the line breaks fall
		// is cosmetic, and adding two keys reddened this test for a change
		// that moved no behaviour (ranger-base-ncxa). Exhaustiveness against
		// runtimeYamlKeys() is pinned in internal/rhq by
		// TestOnboardingFooterNamesEveryDeclarableKey, which slices the
		// footer out of the screen instead of matching across it; these are
		// the keys this command's own contract turns on.
		"skills_flag: OR skills_cwd:",
		"self_sandbox:",
		"unattended:",
		"project_config: (+ project_config_keys:)",
		// The preflight, each gap by name (ADR 0012 D4).
		"preflight — ",
		"✗ exe:",
		"✗ yaml:",
		"skils_flag",
	} {
		if !strings.Contains(grid, want) {
			t.Errorf("posse runtime check missing %q in:\n%s", want, grid)
		}
	}
	// And a profile with nothing to report exits 0 and says so. `sh` is on
	// every box this suite runs on, and RHQ_HERDR_BIN points at nothing —
	// which is the UNKNOWN reading, a named degrade, not a refusal.
	if err := os.WriteFile(filepath.Join(home, "runtimes", "shell.yaml"), []byte("command: sh {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok := exec.Command(bin, "runtime", "check", "shell")
	ok.Env = env
	okout, err := ok.CombinedOutput()
	if err != nil {
		t.Errorf("a profile whose only gap is UNKNOWN detection must exit 0: %v\n%s", err, okout)
	}
	if !strings.Contains(string(okout), "nothing blocking") {
		t.Errorf("want the non-blocking verdict in:\n%s", okout)
	}
}
