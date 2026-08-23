package rhq

// QA, rangerhq-y1je — the chain INSTALL.md §9 prescribes, exercised rather
// than read. §9 has to hand both slots to bd's shims AND keep the posse gates,
// so the gate runs as its own process with its status checked and then execs
// bd's hook. Everything here runs the hook files the way git does; nothing is
// asserted from their text. Self-contained (own repo, own fixtures) so it
// stands whatever the next persona does to gates_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaChainRepo builds the state §9 leaves behind: posse-<slot> holds the gate,
// bd-<slot> stands in for `bd hooks install`'s shim (it records the argv and
// stdin it was handed, which is the only way to see whether the chain reached
// it and with what), and <slot> is §9's dispatcher, copied verbatim.
func qaChainRepo(t *testing.T) (repo, witness string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	witness = filepath.Join(t.TempDir(), "bd.log")
	for slot, stdin := range map[string]string{"pre-push": " </dev/null", "prepare-commit-msg": ""} {
		if err := os.Rename(filepath.Join(hooks, slot), filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Fatal(err)
		}
		bd := "#!/bin/sh\nprintf 'argv[%s]\\n' \"$*\" >> " + witness +
			"\nprintf 'stdin[%s]\\n' \"$(cat)\" >> " + witness + "\nexit 0\n"
		if err := os.WriteFile(filepath.Join(hooks, "bd-"+slot), []byte(bd), 0o755); err != nil {
			t.Fatal(err)
		}
		chain := "#!/bin/sh\nd=$(dirname \"$0\")\n" +
			"\"$d/posse-" + slot + "\" \"$@\"" + stdin + " || exit $?\n" +
			"exec \"$d/bd-" + slot + "\" \"$@\"\n"
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(chain), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo, witness
}

// runHook runs one slot the way git does — argv, stdin, and an env that
// carries nothing this suite's own pane exported.
func runHook(t *testing.T, repo, slot, stdin string, env ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(repo, ".git", "hooks", slot))
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append([]string{"PATH=" + PathOutsideGates("")}, env...)
	switch slot {
	case "pre-push":
		cmd.Args = append(cmd.Args, "origin", "https://example.invalid/x.git")
	default:
		cmd.Args = append(cmd.Args, filepath.Join(repo, ".git", "COMMIT_EDITMSG"), "message")
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v", slot, err)
	}
	return string(out), code
}

// The whole point of the chain: the gate's refusal ends the slot (bd's hook
// is never reached), and when the gate passes, bd's hook gets its argv and —
// on pre-push, where git feeds a ref list — the stdin the gate did not eat.
func TestQADocChainRefusesFirstAndOtherwiseReachesBdsHook(t *testing.T) {
	repo, witness := qaChainRepo(t)
	refs := "refs/heads/main a1 refs/heads/main b1\nrefs/tags/v1 a2 refs/tags/v1 b2\n"
	read := func() string { b, _ := os.ReadFile(witness); return string(b) }

	out, code := runHook(t, repo, "pre-push", refs, "RHQ_PERSONA=qa", "RHQ_TOOLS_DENY=Bash(git push:*)", "RHQ_GATES_DIR="+t.TempDir())
	if code != 1 || !strings.Contains(out, "refused by posse gate: git push") {
		t.Errorf("denied push must refuse with exit 1: code=%d %q", code, out)
	}
	if read() != "" {
		t.Errorf("a refused push must not reach bd's hook: %q", read())
	}

	out, code = runHook(t, repo, "prepare-commit-msg", "", "RHQ_PERSONA=qa", "RHQ_GATES_DIR="+t.TempDir())
	if code != 1 || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("unqualified commit must refuse with exit 1: code=%d %q", code, out)
	}
	if read() != "" {
		t.Errorf("a refused commit must not reach bd's hook: %q", read())
	}

	if out, code := runHook(t, repo, "pre-push", refs); code != 0 {
		t.Errorf("no RHQ_TOOLS_DENY must pass: code=%d %q", code, out)
	}
	if got := read(); !strings.Contains(got, "argv[origin https://example.invalid/x.git]") ||
		!strings.Contains(got, "refs/heads/main a1") || !strings.Contains(got, "refs/tags/v1 a2") {
		t.Errorf("bd's pre-push hook must get its argv and the WHOLE ref list on stdin: %q", got)
	}

	if err := os.Truncate(witness, 0); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "prepare-commit-msg", ""); code != 0 {
		t.Errorf("no RHQ_PERSONA (the operator's own commit) must pass: code=%d %q", code, out)
	}
	if got := read(); !strings.Contains(got, "COMMIT_EDITMSG message]") {
		t.Errorf("bd's prepare-commit-msg hook must get $1 and $2 unchanged: %q", got)
	}
}

// rangerhq-xo65: the chain execs bd-<slot> unconditionally, so a repo where
// bd never took that slot (older bd, `bd hooks install --beads/--shared`) has
// no commit path at all — the failure hits the operator, whom the gate is
// careful to exempt.
func TestQADocChainSurvivesAMissingNeighbourHook(t *testing.T) {
	t.Skip("rangerhq-xo65: a missing bd-<slot> makes the slot exit 126, so every commit in the repo dies")
	repo, _ := qaChainRepo(t)
	if err := os.Remove(filepath.Join(repo, ".git", "hooks", "bd-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "prepare-commit-msg", ""); code != 0 {
		t.Errorf("the operator's commit must survive a missing neighbour: code=%d %q", code, out)
	}
}

// rangerhq-lrnp: the guard's own comment exempts the commits git drives
// itself, on the strength of a marker file in the git dir. A clean
// `git revert` writes none of them before prepare-commit-msg runs (git
// 2.39.3), so the persona is refused — and refused only after the revert is
// already staged in the shared index the guard exists to protect.
func TestQAGuardLetsAGitDrivenRevertThrough(t *testing.T) {
	t.Skip("rangerhq-lrnp: no REVERT_HEAD exists at prepare-commit-msg time, so the guard refuses the revert")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(extra []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), env...), extra...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	git(nil, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644)
	git(nil, "add", "a.txt")
	git(nil, "commit", "-qm", "add a")
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	if out, err := git(persona, "revert", "--no-edit", "HEAD"); err != nil {
		t.Errorf("a git-driven revert must be let through: %v %s", err, out)
	}
	if out, _ := git(nil, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Errorf("a refused revert must not leave the shared index dirty: %q", out)
	}
}

// rangerhq-b38m: git runs hooks from core.hooksPath when it is set, and
// hooksDir() never reads it — so both gates land in .git/hooks, install
// reports success, §9's probes (which run the file directly) go green, and
// git runs none of it.
func TestQAInstallHooksHonoursCoreHooksPath(t *testing.T) {
	t.Skip("rangerhq-b38m: install-hooks writes .git/hooks even when core.hooksPath points elsewhere")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	elsewhere := filepath.Join(repo, "myhooks")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	exec.Command("git", "-C", repo, "init", "-q", "-b", "main").Run()
	exec.Command("git", "-C", repo, "config", "core.hooksPath", elsewhere).Run()
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "pre-push")); err != nil {
		t.Errorf("the gate must go where git will run it (core.hooksPath): %v", err)
	}
}

// rangerhq-j4sq — §9's closing claim, run rather than read: a repo where
// `bd hooks install` got there first is covered by NOTHING when you dispatch
// into it. This is session create's own install path (herdrback.go: both
// installs are best effort, their errors discarded), not the CLI's, so the
// silence is the point — the operator sees no failure anywhere.
//
// bd's shim stands in as a hook carrying its `# bd-shim v1` header and an
// exit-0 body: installHook keys only on the ABSENCE of our own marker, and a
// stand-in keeps the real `exec bd hooks run` off a throwaway repo.
func TestQASessionCreateInstallsNothingIntoABdHookedRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := "#!/usr/bin/env sh\n# bd-shim v1\nexit 0\n"
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Verbatim what a session create does, error handling included.
	InstallPrePushHook(repo)
	installCommitGuard(repo)

	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		b, err := os.ReadFile(filepath.Join(hooks, slot))
		if err != nil {
			t.Fatal(err)
		}
		if got := string(b); got != shim {
			t.Fatalf("%s: session create's install changed the slot; §9 may now be understating what a dispatch does:\n%s", slot, got)
		}
	}
	// And the gate is not hiding under another name either.
	ents, err := os.ReadDir(hooks)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		b, _ := os.ReadFile(filepath.Join(hooks, e.Name()))
		if strings.Contains(string(b), "posse-gate") {
			t.Fatalf("found a posse gate at %s — §9's closing paragraph needs re-checking", e.Name())
		}
	}
	// The consequence, at the slot: §9's own probe 1 goes green-through.
	out, code := runHook(t, repo, "pre-push", "refs/heads/main a refs/heads/main b\n",
		"RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)")
	if code != 0 || strings.Contains(out, "refused by posse gate") {
		t.Fatalf("pre-push refused after all (exit %d): %s — §9's closing paragraph needs re-checking", code, out)
	}
}
