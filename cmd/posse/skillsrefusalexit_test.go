package main

// ranger-base-f6hiy, second ask: "a skills failure must not exit 0 with no
// session — that combination is what made this look like the load guard
// rather than the skills step".
//
// This is the pin, not a fix: measured against the binary the operator
// actually ran (0.4.0+0b5c1c4) and against HEAD, `posse new` on a skills
// refusal exits 1 with the line on STDERR and creates nothing. So does
// `posse recipe`. The exit-0 in the report belongs to a caller that lost
// the status, or to one of the two paths that survive a failed launch by
// design and say so on their own: a dispatch pass (one bead skipped, the
// pass goes on) and the cockpit (a TUI, which exits when it is quit).
//
// It is pinned here because the refusal's whole value is that it stops the
// launch: a future refactor that turns it into a warn line — the shape the
// L3 wall line above it already has — would put posse back in the state
// this bead was filed for, a persona launching without the skills its PID
// says its work depends on, and nothing but a line in the scrollback.
//
// Hermetic: no herdr. The refusal is raised in planLaunch, above every
// herdr call, which is exactly why it can be measured this way.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandTypedLaunchExitsNonZeroOnASkillsRefusal(t *testing.T) {
	bin := buildRhq(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git: %v", err)
	}

	home := t.TempDir()
	rhq := filepath.Join(home, "posse")
	for _, d := range []string{
		filepath.Join(rhq, "skills", "distributed-systems"),
		filepath.Join(rhq, "agents"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(rhq, "skills", "distributed-systems", "SKILL.md"),
		"---\nname: distributed-systems\ndescription: the distributed-systems skill\n---\nbody\n")
	// codex is the runtime that has no skills flag, so the binding
	// materializes in the session dir — the only surface this refusal
	// exists on (ADR 0007's last resort).
	write(filepath.Join(rhq, "agents", "developer.md"),
		"---\nname: developer\ndescription: probe persona\nruntime: codex\ntier: standard\nskills: [distributed-systems]\n---\nYou are developer.\n")
	// load_guard: 0 is not decoration. The guard runs at the top of
	// planLaunch, ABOVE the skills render, so on a loaded box it answers
	// first and this pin measures the wrong refusal — which is also the
	// operator's original confusion in f6hiy: two load-guard refusals, then
	// a skills refusal that read as a third one (ranger-base-jfe5z).
	write(filepath.Join(rhq, "config.yaml"), "default_dir: ~\nload_guard: 0\nbeads:\n  - "+home+"\n")

	repo := t.TempDir()
	if out, err := exec.Command(git, "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// A live foreign entry: the operator's own dir of that name, which the
	// never-clobber rule refuses and goes on refusing after f6hiy (a
	// DANGLING link is the case that stopped refusing — skills.go).
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "skills", "distributed-systems"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "new", "f6hiy-probe", "--dir", repo, "--agent", "developer", "--runtime", "codex")
	cmd.Env = []string{"HOME=" + home, "RHQ_HOME=" + rhq, "PATH=" + filepath.Dir(git) + ":/usr/bin:/bin"}
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()

	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse new: %v\nstdout %q\nstderr %q", err, out.String(), errb.String())
	}
	if code == 0 {
		t.Errorf("a skills refusal exited 0 — the launch failed and the caller cannot tell\nstdout %q\nstderr %q", out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "not overwriting") {
		t.Errorf("the refusal must reach stderr, not stdout and not silence\nstdout %q\nstderr %q", out.String(), errb.String())
	}
	// ...and no session behind it: the failure this bead is about was one
	// where the operator could not tell "refused" from "launched".
	if ents, _ := os.ReadDir(filepath.Join(rhq, "state", "herdr")); len(ents) > 0 {
		names := make([]string, 0, len(ents))
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("a refused launch left session state behind: %v", names)
	}
}
