package main

// QA, rangerhq-irrl — the chain prescription `posse gates install-hooks` prints
// when it finds a foreign hook, exercised by RUNNING IT rather than by reading
// it. The prescription exists because appending to the gate silently discarded
// its refusal (rangerhq-kk6e/rangerhq-0g1c); a prescription that cannot be
// followed as printed leaves the same hole open, so the assertion is on the
// state the printed block actually produces. Self-contained (own repo, own
// extractor) so it stands whatever the next persona does to main_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaPrescription pulls the runnable block out of a refusal: everything from
// the `cd <hooks>` line through the `chmod +x <slot>` that ends it, with the
// bare `posse` of step 3 pointed at the binary under test.
func qaPrescription(t *testing.T, out, slot, bin string) string {
	t.Helper()
	start := strings.Index(out, "\ncd ")
	end := strings.Index(out, "\nchmod +x "+slot+"\n")
	if start < 0 || end < 0 {
		t.Fatalf("no runnable prescription for %s in:\n%s", slot, out)
	}
	block := out[start+1 : end+len("\nchmod +x "+slot+"\n")]
	return strings.Replace(block, "\nposse gates install-hooks ", "\n"+bin+" gates install-hooks ", 1)
}

// qaForeignBoth is the state `bd hooks install` leaves and INSTALL.md §9
// walks the operator out of: a foreign shim in each slot posse wants.
func qaForeignBoth(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		shim := "#!/bin/sh\n# bd-shim v1\necho \"bd shim ran: " + slot + "\" >&2\nexit 0\n"
		if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", slot), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

// qaSh runs a prescription block the way an operator pastes it: one /bin/sh,
// no shell state carried in from this suite's own pane.
func qaSh(t *testing.T, script string) (string, int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("sh: %v", err)
	}
	return string(out), code
}

// rangerhq-pon3: install-hooks is fatal on the pre-push slot and only
// reports on the commit slot, so once the pre-push dispatcher is pasted —
// step 5 of the first prescription — every later install-hooks run dies on
// it and the commit gate is never installed. The second prescription then
// renames a file that was never written and pastes a dispatcher pointing at
// nothing: exit 127, and every commit in the repo fails with it.
func TestQAInstallRefusalPrescriptionIsRunnable(t *testing.T) {
	bin := buildRhq(t)
	repo := qaForeignBoth(t)
	hooks := filepath.Join(repo, ".git", "hooks")

	// The refusal for the first slot, and the block it prints.
	first, _ := exec.Command(bin, "gates", "install-hooks", repo).CombinedOutput()
	pre := qaPrescription(t, string(first), "pre-push", bin)
	preOut, code := qaSh(t, pre)
	if code != 0 {
		t.Errorf("the pre-push prescription must run as printed: code=%d\n%s", code, preOut)
	}
	// Step 3 of that block is itself an install-hooks run; it prints the
	// second slot's prescription, which the operator follows next.
	commit := qaPrescription(t, preOut, "prepare-commit-msg", bin)
	commitOut, code := qaSh(t, commit)
	if code != 0 {
		t.Errorf("the prepare-commit-msg prescription must run as printed: code=%d\n%s", code, commitOut)
	}

	// Both gates must exist behind their dispatchers.
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Errorf("the %s gate must be installed after its prescription: %v", slot, err)
		}
	}

	// And the probe each prescription prints must hold: refusal, exit 1.
	msg := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	os.WriteFile(msg, []byte("m\n"), 0o644)
	for _, p := range []struct {
		slot, stdin string
		args        []string
		env         []string
	}{
		{"pre-push", "refs/heads/main a refs/heads/main b\n", []string{"origin", "x"},
			[]string{"RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)"}},
		{"prepare-commit-msg", "", []string{msg}, []string{"RHQ_PERSONA=probe"}},
	} {
		cmd := exec.Command(filepath.Join(hooks, p.slot), p.args...)
		cmd.Dir = repo
		cmd.Stdin = strings.NewReader(p.stdin)
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, p.env...)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		if code != 1 || !strings.Contains(string(out), "refused by posse gate") {
			t.Errorf("%s probe must refuse and exit 1: code=%d %q", p.slot, code, out)
		}
	}

	// The operator's own commit must still work — a dispatcher whose
	// neighbour is missing exits 127 and takes every commit with it.
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644)
	add := exec.Command("git", "-C", repo, "add", "a.txt")
	add.Env = []string{"PATH=" + os.Getenv("PATH")}
	add.Run()
	ci := exec.Command("git", "-C", repo, "commit", "-qm", "operator commit")
	ci.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	if out, err := ci.CombinedOutput(); err != nil {
		t.Errorf("the operator's own commit must survive the chain: %v %s", err, out)
	}
}

// rangerhq-mgdk: a single `install-hooks` call on a repo where BOTH slots
// are foreign must attempt and report both — pre-push failing must not
// cost prepare-commit-msg (or vice versa) — and must exit non-zero exactly
// when something was left uninstalled.
func TestQAInstallHooksAttemptsBothSlotsInOneCall(t *testing.T) {
	bin := buildRhq(t)
	repo := qaForeignBoth(t)

	out, err := exec.Command(bin, "gates", "install-hooks", repo).CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("gates install-hooks: %v %s", err, out)
	}
	if code == 0 {
		t.Errorf("a call that installed neither slot must exit non-zero: %s", out)
	}
	if !strings.Contains(string(out), "not installed: pre-push") {
		t.Errorf("pre-push refusal must be reported: %s", out)
	}
	if !strings.Contains(string(out), "not installed: prepare-commit-msg") {
		t.Errorf("prepare-commit-msg must be ATTEMPTED and reported even though pre-push failed first: %s", out)
	}
}

// rangerhq-mgdk: --chain takes over a repo `bd hooks install` reached
// first — the state TestQASessionCreateInstallsNothingIntoABdHookedRepo
// shows dispatch leaves silently uncovered — in one call, both slots.
func TestQAInstallHooksChainFlagTakesOverBdsShim(t *testing.T) {
	bin := buildRhq(t)
	repo := qaForeignBoth(t)
	hooks := filepath.Join(repo, ".git", "hooks")

	out, err := exec.Command(bin, "gates", "install-hooks", repo, "--chain").CombinedOutput()
	if err != nil {
		t.Fatalf("gates install-hooks --chain: %v %s", err, out)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooks, "bd-"+slot)); err != nil {
			t.Errorf("bd's shim must be moved aside for %s: %v", slot, err)
		}
		if _, err := os.Stat(filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Errorf("our gate must be installed for %s: %v", slot, err)
		}
	}

	msg := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	os.WriteFile(msg, []byte("m\n"), 0o644)
	cmd := exec.Command(filepath.Join(hooks, "pre-push"), "origin", "x")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader("refs/heads/main a refs/heads/main b\n")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)"}
	pushOut, pushErr := cmd.CombinedOutput()
	pushCode := 0
	if ee, ok := pushErr.(*exec.ExitError); ok {
		pushCode = ee.ExitCode()
	}
	if pushCode != 1 || !strings.Contains(string(pushOut), "refused by posse gate") {
		t.Errorf("denied push must still refuse through the --chain result: code=%d %q", pushCode, pushOut)
	}
}
