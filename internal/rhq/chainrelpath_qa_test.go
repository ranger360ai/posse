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
// FIXED 2026-08-30 (ranger-base-87c9): chainDispatcher resolves the
// repo argument to an absolute path before abbreviating it, so step 3 still
// names the repo after step 1's cd. `.` from inside the repo now survives on
// purpose rather than by luck. Measured at HEAD with a binary built from this
// worktree and a scratch RHQ_HOME: `posse gates install-hooks r` prints the
// re-invocation as an absolute path, the block pasted verbatim exits 0 leaving
// pre-push, posse-pre-push, theirs-pre-push and prepare-commit-msg, and the
// block's own verify step prints "refused by posse gate" and exits 1 — the
// EXPECTED this bead was filed for. Full `go test ./...` green (exit 0).
//
// Two mutants, git as baseline, tree clean before and after each — the two
// halves of this pin see different defects:
//
//   - the argument echoed as typed again: the two relative spellings red,
//     absolute and `.` stay green, which is the split the bead reports;
//   - the dispatcher's `|| exit $?` weakened to `|| true`: only the RUN half
//     reds, and all four arms do. Without that one, the run below would be
//     decoration nobody had shown able to fail.
//
// The pin both states the rule and runs the block. The rule: every path the
// prescription prints has to still mean the same thing after step 1's `cd`.
// The run: the steps are performed from the directory the block's own `cd`
// names, with the arguments the block printed — step 3 is InstallPrePushHook
// with the process cwd where the paste put it, which is exactly what `posse
// gates install-hooks <arg>` is there, so no posse binary on PATH is needed
// and nothing is retyped. Then the block's own verify step runs the slot.

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

// inParent is the ordinary case: the operator stands beside the repo.
func inParent(parent, repo string) string { return parent }

var (
	relCdLine      = regexp.MustCompile(`(?m)^cd (\S+)$`)
	relInstallLine = regexp.MustCompile(`(?m)^posse gates install-hooks (\S+)$`)
)

func TestChainPrescriptionPathsSurviveItsOwnCd(t *testing.T) {
	for _, tc := range []struct {
		name string
		// from is the directory the operator types the command in, and spell
		// the argument they hand install-hooks there. Both are given the
		// parent directory and the repo inside it.
		from  func(parent, repo string) string
		spell func(parent, repo string) string
	}{
		{"relative to the parent", inParent, func(parent, repo string) string { return filepath.Base(repo) }},
		{"absolute", inParent, func(parent, repo string) string { return repo }},
		{"dot-slash relative", inParent, func(parent, repo string) string { return "./" + filepath.Base(repo) }},
		// `.` from inside the repo used to survive by luck: the block became
		// `cd .git/hooks` then `install-hooks .`, and git discovers the repo
		// upwards from inside .git/hooks. It has to keep working when the
		// argument stops being echoed as typed.
		{"dot from inside the repo", func(parent, repo string) string { return repo }, func(parent, repo string) string { return "." }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			repo := filepath.Join(parent, "r")
			hooks := relGitRepo(t, repo)
			typedIn := tc.from(parent, repo)
			t.Chdir(typedIn)

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
				cwd = filepath.Join(typedIn, cwd)
			}
			arg := inst[1]
			if !filepath.IsAbs(arg) {
				arg = filepath.Join(cwd, arg)
			}
			// Which repo that argument names is git's lookup, not string
			// arithmetic — which is why `.` survived the defect: from the
			// hooks directory it names no worktree at all, and git resolves
			// the hooks path upwards anyway (`--show-toplevel` there is
			// fatal; `--git-path hooks` is not). So ask the question step 3
			// asks, with hooksDir, the function install-hooks resolves with.
			gotHooks, err := hooksDir(arg)
			if err != nil {
				t.Fatalf("step 3 hands install-hooks %q, which from %q is %q — no git repository there, so the gate is never written and the steps below build a dispatcher around nothing (%v)",
					inst[1], cd[1], arg, err)
			}
			if got, want := mustEval(t, gotHooks), mustEval(t, hooks); got != want {
				t.Errorf("step 3 hands install-hooks %q, which from %q installs into %q, not the repo's hooks dir %q",
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

			// And now run it, from where its own first step stands.
			relPaste(t, text, cwd)

			if !PrePushHookInstalled(repo) {
				t.Error("the pasted chain is not recognized as installed")
			}
			q32oNoSelfExec(t, hooks)
			// The prescription's own verify step: it says this must print the
			// refusal and exit 1. Around a gate that was never written the
			// dispatcher exits 127 instead, on every push, denied or not.
			if out, code := q32oRun(t, repo, "pre-push", "RHQ_PERSONA=qa", "RHQ_TOOLS_DENY=Bash(git push:*)", "RHQ_GATES_DIR="+t.TempDir()); code != 1 ||
				!strings.Contains(out, "refused by posse gate: git push") {
				t.Errorf("the prescription's verify step must refuse with exit 1: code=%d %q", code, out)
			}
			// A permitted push is the only run that reaches the neighbour.
			if out, code := q32oRun(t, repo, "pre-push"); code != 0 || !strings.Contains(out, "their pre-push ran") {
				t.Errorf("permitted push must pass and reach their hook: code=%d %q", code, out)
			}
		})
	}
}

// relPaste performs the printed prescription the way an operator does: from
// the directory step 1 names, with the arguments step 3 printed. Nothing here
// is retyped — a fixture that is a copy of the thing under test measures the
// copy.
func relPaste(t *testing.T, text, cwd string) {
	t.Helper()
	t.Chdir(cwd)

	moves := q32oMove.FindAllStringSubmatch(text, -1)
	if len(moves) != 2 {
		t.Fatalf("prescription no longer has exactly two mv steps: %q", text)
	}
	arg := relInstallLine.FindStringSubmatch(text)[1]

	if err := os.Rename(moves[0][1], moves[0][2]); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPrePushHook(ExpandTilde(arg)); err != nil {
		t.Fatalf("step 3 `posse gates install-hooks %s` fails from %s: %v — the gate is never written, and the mv and heredoc below build a dispatcher around a file that is not there",
			arg, cwd, err)
	}
	if err := os.Rename(moves[1][1], moves[1][2]); err != nil {
		t.Fatalf("step 4 `mv %s %s` fails from %s: %v", moves[1][1], moves[1][2], cwd, err)
	}

	open := "cat > pre-push <<'EOF'\n"
	i := strings.Index(text, open)
	if i < 0 {
		t.Fatalf("prescription no longer writes pre-push with a heredoc: %q", text)
	}
	rest := text[i+len(open):]
	j := strings.Index(rest, "\nEOF\n")
	if j < 0 {
		t.Fatalf("prescription's heredoc has no terminator: %q", text)
	}
	if err := os.WriteFile("pre-push", []byte(rest[:j+1]), 0o755); err != nil {
		t.Fatal(err)
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
