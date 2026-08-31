package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// ADR 0033 §5 where an operator meets it: `posse agent check`. The alarm is
// "mechanical and advisory ... never the enforcement", and at this surface
// advisory means exactly one thing — the line is printed and the exit code
// does not move. The unit pin (internal/posse TestCheckAgentCoordinatorParity)
// holds the two arms inside CheckAgent, but it reads the warnings return
// directly, so it is green over a command that counts warnings toward its
// exit status: measured 2026-08-28 on rangerhq-l72e, `findings += len(fs) +
// len(ws)` in main.go left the whole cmd/posse package green while turning
// §5 into the enforcement the ADR forbids. This test is that mutant's death,
// and the control arm below is what makes its exit-zero assertions mean
// something: a real finding on the same PID in the same home still exits 1.
func TestAgentCheckCoordinatorDriftIsAdvisory(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	a := &posse.App{
		Home:       home,
		AgentsDir:  filepath.Join(home, "agents"),
		ConfigPath: filepath.Join(home, "config.yaml"),
	}
	path, err := a.ScaffoldAgent("push-holder")
	if err != nil {
		t.Fatal(err)
	}
	// The scaffold is contract-clean, so every finding below is one this
	// test planted — and the grant is the only edit.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const grant = "  - Bash(git push:*)\n"
	body := strings.Replace(string(raw),
		"allow:\n  # permission rules added to the repo floor, e.g.\n  # - Bash(bd:*)\n",
		"allow:\n"+grant, 1)
	if body == string(raw) {
		t.Fatalf("the scaffold's allow: block moved — this fixture plants no grant:\n%s", raw)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	check := func(t *testing.T) (string, int) {
		t.Helper()
		cmd := exec.Command(bin, "agent", "check", "push-holder")
		cmd.Env = statusEnv(t, home)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("posse agent check: %v\n%s", err, out)
		}
		return string(out), code
	}
	config := func(t *testing.T, s string) {
		t.Helper()
		if err := os.WriteFile(a.ConfigPath, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Arm 1, the one that matters: no coordinator: at all. Printed, prefixed
	// `warning:`, and the shop still passes its lint.
	out, code := check(t)
	for _, want := range []string{"warning: ", "Bash(git push:*)", "no coordinator: is configured", "ADR 0033 §5"} {
		if !strings.Contains(out, want) {
			t.Errorf("the no-coordinator arm must say %q:\n%s", want, out)
		}
	}
	if code != 0 {
		t.Errorf("an advisory warning moved the exit code to %d — ADR 0033 §5 is a drift alarm, not enforcement:\n%s", code, out)
	}

	// Arm 2: a coordinator is named and it is somebody else.
	config(t, "coordinator: someone-else\n")
	out, code = check(t)
	for _, want := range []string{"warning: ", "not the coordinator (coordinator: someone-else)", "ADR 0033 §5"} {
		if !strings.Contains(out, want) {
			t.Errorf("the drift arm must say %q:\n%s", want, out)
		}
	}
	if code != 0 {
		t.Errorf("the drift warning moved the exit code to %d:\n%s", code, out)
	}

	// Arm 3: the coordinator's own grant is silent — the alarm is about
	// drift, and a shop whose config agrees with its PIDs has none.
	config(t, "coordinator: push-holder\n")
	out, code = check(t)
	if strings.Contains(out, "ADR 0033 §5") || strings.Contains(out, "warning: ") {
		t.Errorf("the coordinator's own push grant warned:\n%s", out)
	}
	if code != 0 {
		t.Errorf("a clean PID exited %d:\n%s", code, out)
	}

	// Control: exit 1 IS reachable from this command, in this home, on this
	// PID — so the zeros above measure the advisory rule and not a rig that
	// cannot fail. A finding and the §5 warning ride out together, told
	// apart by the prefix, and only the finding is counted.
	config(t, "")
	planted := strings.Replace(body, "tier: standard\n", "tier: nonsense\n", 1)
	if planted == body {
		t.Fatal("the scaffold's tier: line moved — this control plants no finding")
	}
	if err := os.WriteFile(path, []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = check(t)
	if !strings.Contains(out, `tier: "nonsense" is not`) || !strings.Contains(out, "1 finding(s)") {
		t.Fatalf("the control planted no finding — the exit codes above measure nothing:\n%s", out)
	}
	if code != 1 {
		t.Errorf("a real finding exited %d, want 1:\n%s", code, out)
	}
	if !strings.Contains(out, "no coordinator: is configured") {
		t.Errorf("the §5 warning must still print beside a finding:\n%s", out)
	}
}
