package posse

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
// Fixed by ranger-base-58to: gates.go now builds these lines from
// posse_qcached, which single-quotes each path, POSIX-escaping any embedded
// quote, instead of interpolating git's quoted, space-delimited list raw.
// That fix reached for core.quotePath=false and so covered only the two byte
// classes THIS pin names; ranger-base-qg0k8 replaced the reader with -z and
// covers the rest, in the test below.
func TestQAGuardRefusalNamesQuotedPathsUsably(t *testing.T) {
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

// ranger-base-qg0k8: the same claim as the test above — the refusal's
// prescribed lines are commands, so they are verified by running them — over
// EVERY byte class git has to quote, not just the two the earlier pin
// covered.
//
// The ranger-base-58to fix turned git's non-ASCII quoting off with
// core.quotePath=false and single-quoted each path. core.quotePath only
// governs non-ASCII bytes, though: git C-quotes a path holding a double
// quote, a backslash or a control byte whatever quotePath says, so the reader
// wrapped git's ALREADY-QUOTED spelling in single quotes and the literal
// C-escape reached the pathspec — `'"q\"uote.md"'`. Measured, that killed the
// whole line with "did not match any file(s) known to git", and because git
// validates pathspecs all-or-nothing the correctly-spelled paths on the same
// line were not committed or restored either: the persona was left with a
// dirty SHARED index and two commands that do not work, directly above the
// sentence telling them not to reach for `git reset --hard`.
//
// Fixed by reading `--name-only -z`, whose output is the raw path bytes for
// every byte class, with no quotePath override.
func TestQAGuardRefusalNamesEveryPathGitQuotes(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	// One path per byte class git treats differently. The three the earlier
	// fix missed are the middle three; `two.md` is plain and is there to show
	// the whole line dies, not just the awkward paths, and the command
	// substitution is there to show the single-quoting still keeps it inert.
	paths := []string{
		"my file.md",        // a space: bare in --name-only, splits a pathspec in two
		"café.md",           // non-ASCII: the only class core.quotePath governs
		"it's.md",           // an embedded single quote: the escape the wrapper does
		"q\"uote.md",        // a double quote: C-quoted whatever quotePath says
		"back\\slash.md",    // a backslash: likewise
		"tab\there.md",      // a control byte: likewise
		"$(touch PWNED).md", // command substitution planted in a filename
		"two.md",            // plain
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
	seed := append([]string{"add", "--"}, paths...)
	for _, n := range paths {
		if err := os.WriteFile(filepath.Join(repo, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := git(nil, seed...); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if out, err := git(nil, append([]string{"commit", "-qm", "seed", "--"}, paths...)...); err != nil {
		t.Fatalf("seed: %v\n%s", err, out)
	}
	git(nil, "commit", "--allow-empty", "-qm", "base")
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

	// Every path has to be NAMED, in its exact shell spelling, before running
	// the lines can mean anything: a line that silently DROPPED q"uote.md
	// would leave the tree dirty below and this pin would report it as a
	// quoting failure instead of an omission. The expected spelling is the
	// POSIX one — single-quoted, an embedded quote closed and reopened — so
	// this also pins the escape itself rather than just the bytes.
	for _, p := range paths {
		want := "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
		if !strings.Contains(undo, want) || !strings.Contains(finish, want) {
			t.Errorf("both prescribed lines must name %s\n  undo:   %s\n  finish: %s", want, undo, finish)
		}
	}

	// The undo the refusal names must actually undo it. Pre-fix it exited 1
	// with three "did not match any file(s) known to git" and left the shared
	// index exactly as dirty as it was.
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
	if o, err := sh(persona, "printf 'r\\n' | "+finish); err != nil {
		t.Errorf("the finish the refusal names does not run: %v\n  line: %s\n%s", err, finish, o)
	}
	if st := status(); st != "" {
		t.Errorf("after the named finish the tree is clean, got %q", st)
	}

	// The single-quoting keeps a command substitution planted in a filename
	// inert through both pasted lines.
	if _, err := os.Stat(filepath.Join(repo, "PWNED")); err == nil {
		t.Error("a filename's $(...) executed: the prescribed lines are not single-quoting")
	}
}
