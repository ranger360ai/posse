package posse

// Helpers lifted out of l3_hookspath_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func qaHookRepo(t *testing.T) (repo, hooks string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	h, err := hooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	return repo, h
}

func qaArm(t *testing.T, hooks string, slots ...string) {
	t.Helper()
	for _, slot := range slots {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func qaGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// qaInstallBothGates installs the two L3 gates into repo, wherever git says
// this repo's hooks are dispatched from.
func qaInstallBothGates(t *testing.T, repo string) {
	t.Helper()
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := (&App{}).InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
}

// qaHasSlots asserts both L3 slots are, or are not, present in dir. The
// fixtures below are only discriminating while exactly one directory is armed.
func qaHasSlots(t *testing.T, dir string, want bool) {
	t.Helper()
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		_, err := os.Stat(filepath.Join(dir, slot))
		if want && err != nil {
			t.Fatalf("fixture: %s is missing from %s: %v", slot, dir, err)
		}
		if !want && err == nil {
			t.Fatalf("fixture: %s is also in %s — the two arms would not differ", slot, dir)
		}
	}
}

func qaCodesAre(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// qaSection9VerifyBlock returns the shell block under §9's "Verify — by
// running the hooks" heading, as the operator would paste it: the `$ ` prompt
// stripped, continuation lines untouched.
func qaSection9VerifyBlock(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	i := strings.Index(doc, "**Verify — by running the hooks")
	if i < 0 {
		t.Fatal("INSTALL.md §9: the hook Verify block is gone — the pin has stopped reading its subject")
	}
	rest := doc[i:]
	open := strings.Index(rest, "```sh\n")
	if open < 0 {
		t.Fatal("INSTALL.md §9: the hook Verify heading is no longer followed by a shell block")
	}
	rest = rest[open+len("```sh\n"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		t.Fatal("INSTALL.md §9: the hook Verify block has no terminator")
	}
	var lines []string
	for _, ln := range strings.Split(rest[:end], "\n") {
		lines = append(lines, strings.TrimPrefix(ln, "$ "))
	}
	return strings.Join(lines, "\n")
}

// qaRunSection9Probes runs the pasted block in repo and returns the exit codes
// its `echo $?` lines printed, in order, plus the whole transcript.
func qaRunSection9Probes(t *testing.T, repo, script string) ([]int, string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = repo
	cmd.Env = qaEnvWithout("RHQ_PERSONA", "RHQ_TOOLS_DENY", "GIT_INDEX_FILE")
	b, _ := cmd.CombinedOutput()
	out := string(b)
	var codes []int
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		n, err := strconv.Atoi(ln)
		if err == nil {
			codes = append(codes, n)
		}
	}
	return codes, out
}
