package rhq

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rangerhq-lrnp verify (ranger-base-pcb1): the refusal's way through is a
// COMMAND, and a command is verified by running it, not by reading it.
//
// gates.go builds both prescribed lines by interpolating
// `git diff --cached --name-only HEAD` unquoted. git C-quotes a path with a
// space or a non-ASCII byte ("caf\303\251.md") and leaves a spaced path bare
// (my file.md), so the printed line is not the command the persona needs:
// pasted into a shell, the quotes are eaten and the literal escape survives,
// and a bare space splits one pathspec into two. Both lines then die on
// "did not match any file(s) known to git" — at the one moment the guard has
// left the revert staged in the SHARED index, directly above the sentence
// telling the persona not to reach for `git reset --hard`.
//
// SKIPPED until ranger-base-58to is fixed. Un-skip and it goes
// red on the first assertion below.
func TestQAGuardRefusalNamesQuotedPathsUsably(t *testing.T) {
	t.Skip("ranger-base-58to: the refusal interpolates git's quoted path list unquoted — un-skip with the fix")

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	// One path git C-quotes, one it leaves bare with a space in it, one
	// plain — the plain one is there to show the whole command dies, not
	// just the awkward paths.
	const spaced, plain = "my file.md", "two.md"
	accented := "café.md"

	repo := t.TempDir()
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(extra []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), env...), extra...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	// The persona's copy-paste: the printed line goes to a shell verbatim.
	sh := func(extra []string, line string) (string, error) {
		cmd := exec.Command("sh", "-c", "cd \"$1\" && "+line, "sh", repo)
		cmd.Env = append(append([]string(nil), env...), extra...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	status := func() string {
		out, _ := git(nil, "status", "--porcelain")
		return strings.TrimSpace(out)
	}

	git(nil, "init", "-q", "-b", "main")
	for _, n := range []string{spaced, accented, plain} {
		if err := os.WriteFile(filepath.Join(repo, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(nil, "add", "--", spaced, accented, plain)
	git(nil, "commit", "-qm", "seed", "--", spaced, accented, plain)
	git(nil, "commit", "--allow-empty", "-qm", "base")
	git(nil, "revert", "--no-edit", "--no-commit", "HEAD")
	git(nil, "revert", "--quit")
	git(nil, "reset", "-q", "--hard", "HEAD")
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}

	out, err := git(persona, "revert", "--no-edit", "HEAD~1")
	if err == nil {
		t.Fatalf("a clean revert is refused: %s", out)
	}
	line := func(prefix string) string {
		re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(prefix) + `\s*(.*?)\s*$`)
		m := re.FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("the refusal must print a %q line:\n%s", prefix, out)
		}
		return m[1]
	}
	undo, finish := line("or undo it:"), line("finish it:")

	// The undo the refusal names must actually undo it. Today it exits 1 with
	// three "did not match any file(s) known to git" errors and the shared
	// index is left exactly as dirty as it was.
	if o, err := sh(persona, undo); err != nil {
		t.Errorf("the undo the refusal names does not run: %v\n  line: %s\n%s", err, undo, o)
	}
	if st := status(); st != "" {
		t.Errorf("after the named undo the tree is clean, got %q", st)
	}

	// And so must the finish, from the same dirty state the refusal left.
	if o, err := git(persona, "revert", "--no-edit", "HEAD~1"); err == nil {
		t.Fatalf("re-refused revert expected: %s", o)
	}
	if o, err := sh(persona, "printf 'r\\n' | "+strings.Replace(finish, "-F -", "-F -", 1)); err != nil {
		t.Errorf("the finish the refusal names does not run: %v\n  line: %s\n%s", err, finish, o)
	}
	if st := status(); st != "" {
		t.Errorf("after the named finish the tree is clean, got %q", st)
	}
}
