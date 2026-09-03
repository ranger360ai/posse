package main

// QA pins for ranger-base-mhrta — `posse gates install-hooks` on a MANAGED
// hooks path (ADR 0052 D1, verification 1), exercised by RUNNING the command
// on the fixture the ADR measured: a mode-0555 directory outside the repo,
// named by GIT_CONFIG_GLOBAL `core.hooksPath`, the way an employer's box
// points every git on it at one root-owned hooks directory.
//
// THE DEFECT, in the operator's own words on the cold install: `open
// <dir>/pre-push: permission denied`. installHook swallowed the read error on
// the missing slot and fell through to os.WriteFile, so the command's first
// act in the employer's directory was a write — and its report was errno's
// account of being refused, not posse's account of what it found.
//
// The claim is three things at once and each is asserted separately: the
// command SAYS what it found (the exact line, in full), it exits 0 because
// nothing failed, and the directory is byte-for-byte where it was. The
// control arm — the same directory, same command, write bit ON — must reach
// the install it would have made, or "wrote nothing" is a statement about a
// fixture that could not be written to anyway.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// mhFixture is a repo, an employer hooks directory outside it, and a global
// gitconfig aiming the first at the second. Nothing is chmod'ed yet: the two
// arms differ in that one bit and in nothing else.
func mhFixture(t *testing.T) (home, repo, managed, gitconfig string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home = t.TempDir()
	root := t.TempDir()
	repo = filepath.Join(root, "checkout")
	managed = filepath.Join(root, "managed-hooks")
	for _, d := range []string{repo, managed} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(managed, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitconfig = filepath.Join(home, "gitconfig-managed")
	if err := os.WriteFile(gitconfig, []byte("[core]\n\thooksPath = "+managed+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, repo, managed, gitconfig
}

// mhRun runs the command under test with the managed box's environment.
// RHQ_HOME is pinned to the scratch home on purpose: inherited from the
// operator's own environment it would point the binary under test at the live
// instance's config.
func mhRun(t *testing.T, bin, home, gitconfig string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"gates", "install-hooks"}, args...)...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"RHQ_HOME="+home,
		"GIT_CONFIG_GLOBAL="+gitconfig,
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("gates install-hooks: %v %s", err, out)
	}
	return string(out), code
}

// mhSnapshot is the directory posse must not touch: its own mode and mtime,
// and the name, size, mode and mtime of everything in it. A create-and-remove
// that left no file behind still moves the directory's mtime.
func mhSnapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&b, ". %v %d\n", st.Mode(), st.ModTime().UnixNano())
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		fi, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s %d %v %d\n", e.Name(), fi.Size(), fi.Mode(), fi.ModTime().UnixNano())
	}
	return b.String()
}

func TestQAInstallHooksOnAManagedPathWritesNothingAndSaysWhy(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root: a mode bit is not a wall for uid 0")
	}
	bin := buildRhq(t)
	home, repo, managed, gitconfig := mhFixture(t)
	if err := os.Chmod(managed, 0o555); err != nil {
		t.Fatal(err)
	}
	// Before t.TempDir's cleanup, which cannot unlink through a read-only
	// directory (LIFO: registered later, runs first).
	t.Cleanup(func() { os.Chmod(managed, 0o755) })
	probe := filepath.Join(managed, ".fixture-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o600); err == nil {
		os.Remove(probe)
		t.Skip("the fixture directory is writable at mode 0555 — nothing managed to classify")
	}

	before := mhSnapshot(t, managed)
	out, code := mhRun(t, bin, home, gitconfig, repo)

	want := "L3: managed hooks path " + managed + " (owner " + strconv.Itoa(os.Geteuid()) + ", mode 0555)" +
		" — posse's wall is not installed there; realized by session redirect (ADR 0052)"
	if !strings.Contains(out, want) {
		t.Errorf("install-hooks said:\n%s\nwant the line\n  %s", out, want)
	}
	if strings.Contains(out, "permission denied") {
		t.Errorf("install-hooks still reports errno from a write it should not have attempted:\n%s", out)
	}
	if code != 0 {
		t.Errorf("exit %d — nothing failed here; the wall is realized by the session redirect", code)
	}
	// The line is a REPORT, not a failure to install: reached through
	// installHook's own refusal it would arrive wearing `not installed:` and
	// an exit 1, which is the same news told as a defeat.
	if strings.Contains(out, "not installed:") {
		t.Errorf("the managed path is reported as a failed install:\n%s", out)
	}
	if after := mhSnapshot(t, managed); after != before {
		t.Errorf("the managed directory moved:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	// Not into the git dir either: a hook written where git does not
	// dispatch is a wall that does not exist (ranger-base-flz7).
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", slot)); err == nil {
			t.Errorf("%s was written into .git/hooks, which this repo's git does not dispatch from", slot)
		}
	}
}

// The control, and the arm that says the pin above measured a classification
// rather than a fixture nobody could write to: the SAME directory, the same
// global core.hooksPath, write bit on. No managed line, and both slots land
// where git dispatches — today's foreign-path behaviour, unchanged.
func TestQAInstallHooksOnAWritableForeignPathStillInstalls(t *testing.T) {
	t.Parallel()
	bin := buildRhq(t)
	home, repo, foreign, gitconfig := mhFixture(t)

	out, code := mhRun(t, bin, home, gitconfig, repo)
	if code != 0 {
		t.Fatalf("exit %d installing into a writable hooks path:\n%s", code, out)
	}
	if strings.Contains(out, "managed hooks path") {
		t.Errorf("a writable directory was classified managed:\n%s", out)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(foreign, slot)); err != nil {
			t.Errorf("%s was not installed where git dispatches: %v", slot, err)
		}
	}
}
