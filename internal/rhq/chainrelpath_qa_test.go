package rhq

// QA, ranger-base-87c9 (found verifying ranger-base-q32o under
// ranger-base-x4ah) — the prescription's own re-invocation of install-hooks.
//
// Step 1 of the printed chain block is `cd <hooks dir>`. Step 3 re-invokes
// `posse gates install-hooks <repo>` — and prints the repo ARGUMENT AS TYPED.
// When the operator named the repo with a relative path, that argument does
// not resolve from the directory step 1 moved into: install-hooks fails, the
// gate file is never written, and the remaining steps build a dispatcher
// around it. The slot then LOOKS chained and exits 127 on every push.
//
// MEASURED at posse c892569 and at its pre-q32o parent 3c92563 (macOS 25.4,
// git 2.50.1): pasting the block after `posse gates install-hooks r` prints
//
//	not installed: pre-push — r is not a git repository
//	mv: rename pre-push to posse-pre-push: No such file or directory
//
// and the prescription's own verify step then reads rc=127 instead of the
// refusal it says to expect. An absolute argument is unaffected, and `.` from
// inside the repo survives by luck (git discovers the repo upwards from
// .git/hooks).
//
// The pin is on the printed TEXT rather than a pasted run, because pasting it
// needs a real posse on PATH: what is wrong is one printed word, and the rule
// it breaks states cleanly — every path the block prints has to still mean the
// same thing after step 1's `cd`.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// relGitRepo initialises repo (which need not be a TempDir root, unlike
// q32oChainedRepo's) and returns its hooks directory with the third party's
// hook already in the slot.
func relGitRepo(t *testing.T, repo string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte(q32oTheirHook), 0o755); err != nil {
		t.Fatal(err)
	}
	return hooks
}

var (
	relCdLine      = regexp.MustCompile(`(?m)^cd (\S+)$`)
	relInstallLine = regexp.MustCompile(`(?m)^posse gates install-hooks (\S+)$`)
)

func TestChainPrescriptionPathsSurviveItsOwnCd(t *testing.T) {
	t.Skip("ranger-base-87c9: the re-invocation prints the repo argument as typed, so a relative path does not resolve after the cd")

	for _, tc := range []struct {
		name string
		// spell returns the argument to hand install-hooks, given the parent
		// directory the test has chdir'd into and the repo inside it.
		spell func(parent, repo string) string
	}{
		{"relative to the parent", func(parent, repo string) string { return filepath.Base(repo) }},
		{"absolute", func(parent, repo string) string { return repo }},
		{"dot-slash relative", func(parent, repo string) string { return "./" + filepath.Base(repo) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			repo := filepath.Join(parent, "r")
			hooks := relGitRepo(t, repo)
			t.Chdir(parent)

			_, err := InstallPrePushHook(tc.spell(parent, repo))
			if err == nil {
				t.Fatal("install-hooks must refuse a foreign pre-push and print the chain")
			}
			text := err.Error()

			cd := relCdLine.FindStringSubmatch(text)
			if cd == nil {
				t.Fatalf("prescription no longer starts with a cd: %q", text)
			}
			inst := relInstallLine.FindStringSubmatch(text)
			if inst == nil {
				t.Fatalf("prescription no longer re-invokes install-hooks: %q", text)
			}

			// Where the paste actually stands when step 3 runs.
			cwd := cd[1]
			if !filepath.IsAbs(cwd) {
				cwd = filepath.Join(parent, cwd)
			}
			arg := inst[1]
			if !filepath.IsAbs(arg) {
				arg = filepath.Join(cwd, arg)
			}
			got, err := filepath.EvalSymlinks(arg)
			if err != nil {
				t.Fatalf("step 3 hands install-hooks %q, which from %q is %q — that path does not exist, so the gate is never written and the steps below build a dispatcher around nothing (%v)",
					inst[1], cd[1], arg, err)
			}
			want, err := filepath.EvalSymlinks(repo)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("step 3 hands install-hooks %q, which from %q resolves to %q, not the repo %q",
					inst[1], cd[1], got, want)
			}

			// The `cd` itself has to land in the repo's hooks directory, or
			// the two mv steps and the heredoc write somewhere else entirely.
			if h, err := filepath.EvalSymlinks(cwd); err != nil || h != mustEval(t, hooks) {
				t.Errorf("step 1 cd's to %q (%q), not the hooks directory %q", cd[1], cwd, hooks)
			}
			if !strings.Contains(text, "mv pre-push ") {
				t.Errorf("prescription no longer moves the slot aside: %q", text)
			}
		})
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	e, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
