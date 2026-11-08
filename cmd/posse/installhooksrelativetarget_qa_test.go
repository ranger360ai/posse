package main

// QA pin for ranger-base-k9x8n — `posse gates install-hooks` resolves its
// target BEFORE anything reads it, so the `.` default (the form every
// degraded line, scripts/verify-hook-freshness.sh and most of INSTALL.md
// prescribes) is the same install as an absolute argument.
//
// WHAT WAS WRONG. The arm took `dir` as typed, and hookRepo hands a MAIN
// checkout's dir straight back (LinkedGitDirs == nil), so `dir` was the
// literal `.` through everything downstream. Two consequences, both measured
// (ranger-base-hgq2k's verification of ADR 0081's four closes, then here):
//
//   - D4's refusal named the repo as `.` and prescribed that `.` as a
//     `beads_visibility:` key. Such a key is resolved against the READING
//     process's cwd, so the remedy was inert where it was printed and a
//     wildcard everywhere else. That half is pinned next door, in
//     commitwallowner_qa_test.go's cwd arm.
//
//   - ADR 0024 D2 check 3's instance-path literal is derived from
//     identityRedirectTarget(hookRepo(dir)), which JOINS a relative
//     `.beads/redirect` target onto `dir` — so the no-argument install
//     stamped `posse_check 'instance-path' '\.\./store'`: an identity check
//     matching the two-character string `../store` rather than the instance
//     path it exists to catch. That is this pin.
//
// WHY BYTE EQUALITY IS THE ASSERTION AND NOT A grep FOR THE PATH. The
// identity half of ADR 0023 is byte-for-byte: the launch renders this same
// wall from the session dir, which is absolute, and compares. A body that
// differs by those three lines is not merely weaker — it reads as "ours but
// stale" on every launch in the repo, which is ranger-base-up22's class. One
// equality holds both claims, and it holds them for every literal the
// derivation adds later.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAInstallHooksRendersTheSameWallForARelativeAndAnAbsoluteTarget(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	bin := buildRhq(t)
	// RESOLVED, because the one variable this pin isolates is relative vs
	// absolute. `t.TempDir()` hands back a path through darwin's /var ->
	// private/var symlink, and the binary absolutizes with filepath.Abs
	// over os.Getwd(), which is physical — so an unresolved root would
	// make the two installs differ by that spelling and this pin would be
	// measuring a SECOND divergence it was not written for: two absolute
	// spellings of one repo render two different walls, which is a
	// question about which spelling is canonical across the three
	// renderers of the identity wall, which is ranger-base-5wq6g.
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "checkout")
	for _, d := range []string{home, filepath.Join(repo, ".beads"), filepath.Join(root, "store", ".beads")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v %s", repo, err, out)
	}
	// RELATIVE, which is the whole fixture: an absolute redirect target
	// derives the same literal whatever `dir` was, so a repo with one
	// cannot tell the two installs apart.
	if err := os.WriteFile(filepath.Join(repo, ".beads", "redirect"), []byte("../store/.beads\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// This home owns the repo (ADR 0081 D1), or D4's refusal would be what
	// the run measured.
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads_visibility:\n  "+repo+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	slot := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	install := func(cwd string, args ...string) string {
		t.Helper()
		if err := os.Remove(slot); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, append([]string{"gates", "install-hooks"}, args...)...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "HOME="+home, "RHQ_HOME="+home)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("gates install-hooks %v in %s: %v\n%s", args, cwd, err, out)
		}
		body, rerr := os.ReadFile(slot)
		if rerr != nil {
			t.Fatalf("the install printed no error and wrote no wall: %v\n%s", rerr, out)
		}
		return string(body)
	}

	// The reference: an absolute target, from a cwd that is not the repo.
	absBody := install(root, repo)
	// The form an operator actually types, standing in the repo.
	relBody := install(repo)

	if relBody != absBody {
		t.Errorf("`install-hooks` with no argument rendered a different wall than `install-hooks %s`:\n%s", repo, qaFirstDiffLines(relBody, absBody))
	}
}

// qaFirstDiffLines is the differing lines, so the failure names the literal
// rather than "two bodies of 1897 lines are unequal".
func qaFirstDiffLines(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	n := len(g)
	if len(w) > n {
		n = len(w)
	}
	var b strings.Builder
	shown := 0
	at := func(lines []string, i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return "(no such line)"
	}
	for i := 0; i < n && shown < 6; i++ {
		if at(g, i) != at(w, i) {
			fmt.Fprintf(&b, "  line %d\n    no-argument: %s\n    absolute:    %s\n", i+1, at(g, i), at(w, i))
			shown++
		}
	}
	if b.Len() == 0 {
		return "  (no line differs — the bodies differ only in trailing bytes)\n"
	}
	return b.String()
}
