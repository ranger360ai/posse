package main

// ranger-base-qnn6j deliverable 2: `posse new <name>` where <name> is the
// name of a PID and no `--agent` came with it.
//
// The two answers on offer were "launch that persona" and "print the hint",
// and the hint is the one that keeps both ADRs. ADR 0008 §1 marks every
// `posse new` crew — the operator made that session to talk to it — so the
// argument is a SESSION name and a session named after a persona is a
// legitimate thing to open. And a launch would bind what only a PID may
// bind: ADR 0002's wall (cage tier, allow/deny, envs, skills) plus BD_ACTOR
// and RHQ_PERSONA, for the life of the session, inferred from a string. So
// the plain pane is created exactly as typed and the difference is said out
// loud beside it.
//
// END-TO-END, and hermetic — no herdr. The hint is printed in main.go
// BEFORE the create, which is what makes it measurable this way: `--dir` at
// a path that does not exist refuses inside planLaunch, above every herdr
// call, and the hint is already on stderr by then. That ordering is the
// pin's subject as much as the text is — a hint printed after a successful
// create would be invisible on exactly the launches that fail.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pphHome is an instance with one PID in it and nothing else moving.
func pphHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	rhq := filepath.Join(home, "posse")
	if err := os.MkdirAll(filepath.Join(rhq, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rhq, "agents", "developer.md"),
		[]byte("---\nname: developer\ndescription: probe persona\n---\nYou are the developer.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// load_guard: 0 for the reason the skills pin gives — the guard answers
	// at the top of planLaunch and would be the refusal a loaded box sees.
	if err := os.WriteFile(filepath.Join(rhq, "config.yaml"), []byte("default_dir: ~\nload_guard: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return rhq
}

func pphRun(t *testing.T, bin, rhq string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = []string{"HOME=" + filepath.Dir(rhq), "RHQ_HOME=" + rhq, "PATH=/usr/bin:/bin"}
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	// The run is EXPECTED to fail: the launch refuses on the missing
	// directory. What is being read is what was said before it did.
	_ = cmd.Run()
	if strings.Contains(out.String(), "created ") {
		t.Fatalf("the probe created a session — this pin is meant to refuse before herdr:\nstdout %q", out.String())
	}
	return errb.String()
}

func TestPosseNewNamingAPersonaWithoutAgentSaysItIsAPlainPane(t *testing.T) {
	bin := buildRhq(t)
	rhq := pphHome(t)
	missing := filepath.Join(t.TempDir(), "not-a-directory")

	got := pphRun(t, bin, rhq, "new", "developer", "--dir", missing)
	for _, want := range []string{"plain pane", "--agent developer", "ADR 0008"} {
		if !strings.Contains(got, want) {
			t.Errorf("`posse new developer` (no --agent) does not say %q:\n%s", want, got)
		}
	}
	// The refusal the probe rides on still happened, so this is the launch
	// path and not an early exit that would print the hint anywhere.
	if !strings.Contains(got, "directory not found") {
		t.Errorf("the probe did not reach planLaunch; the ordering claim above is untested:\n%s", got)
	}
}

// Two controls, because a hint that fires on everything is noise: the
// operator who named the persona, and the operator who named a session.
func TestPosseNewSaysNothingWhenTheLaunchAlreadyNamesThePersona(t *testing.T) {
	bin := buildRhq(t)
	rhq := pphHome(t)
	missing := filepath.Join(t.TempDir(), "not-a-directory")

	got := pphRun(t, bin, rhq, "new", "developer", "--agent", "developer", "--dir", missing)
	if strings.Contains(got, "plain pane") {
		t.Errorf("a launch that named --agent was told it is a plain pane:\n%s", got)
	}
}

func TestPosseNewSaysNothingForASessionNameThatNamesNoPersona(t *testing.T) {
	bin := buildRhq(t)
	rhq := pphHome(t)
	missing := filepath.Join(t.TempDir(), "not-a-directory")

	got := pphRun(t, bin, rhq, "new", "scratch", "--dir", missing)
	if strings.Contains(got, "plain pane") {
		t.Errorf("an ordinary session name drew the persona hint:\n%s", got)
	}
}
