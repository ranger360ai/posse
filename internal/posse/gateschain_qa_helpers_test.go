package posse

// Helpers lifted out of gateschain_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// docChainDispatcher is §9's dispatcher for slot, read out of INSTALL.md
// itself rather than retyped here.
//
// That distinction is the whole of ranger-base-m45t's finding against
// rangerhq-xo65: this file used to hold its own copy of the chain, so every
// pin below ran a body no shipped path renders. Deleting the neighbour guard
// from BOTH chainRender and INSTALL.md left all three packages green
// (measured 2026-08-29) — the byte pin below saw doc and render agree, and
// the execution pins ran the third copy, which still had the guard. A pin
// whose fixture is a copy of the thing under test measures the copy.
//
// So the doc is the fixture now, and TestQADocChainMatchesTheRenderedDispatcher
// holds the renderer to it: a guard dropped from the doc is caught by
// execution, one dropped from the renderer by that byte pin, and one dropped
// from both is caught by both.
func docChainDispatcher(t *testing.T, slot string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	// The block as INSTALL.md pastes it: a heredoc into the slot. §9 asks git
	// for the dispatch dir rather than assuming .git/hooks (rangerhq-b38m),
	// so the heredoc target is `"$h"/<slot>`.
	open := "$ cat > \"$h\"/" + slot + " <<'EOF'\n"
	i := strings.Index(string(b), open)
	if i < 0 {
		t.Fatalf("INSTALL.md §9 no longer writes %s with a heredoc", slot)
	}
	rest := string(b)[i+len(open):]
	j := strings.Index(rest, "EOF\n")
	if j < 0 {
		t.Fatalf("INSTALL.md §9's %s heredoc has no terminator", slot)
	}
	return rest[:j]
}

// qaChainRepo builds the state §9 leaves behind: posse-<slot> holds the gate,
// bd-<slot> stands in for `bd hooks install`'s shim (it records the argv and
// stdin it was handed, which is the only way to see whether the chain reached
// it and with what), and <slot> is §9's dispatcher — the bytes of it, taken
// from the doc.
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
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if err := os.Rename(filepath.Join(hooks, slot), filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Fatal(err)
		}
		bd := "#!/bin/sh\nprintf 'argv[%s]\\n' \"$*\" >> " + witness +
			"\nprintf 'stdin[%s]\\n' \"$(cat)\" >> " + witness + "\nexit 0\n"
		if err := os.WriteFile(filepath.Join(hooks, "bd-"+slot), []byte(bd), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(docChainDispatcher(t, slot)), 0o755); err != nil {
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

// pathWithoutCmp builds a PATH offering every command PathOutsideGates("")
// would, except one literally named cmp — the shape of a minimal
// Fedora/RHEL/Arch box (ranger-base-rmgz: cmp ships in diffutils, measured
// absent by default on three of the four clean-room distros, see
// scripts/cleanroom.sh hook-deps). First hit for a given name wins, same as
// real PATH resolution, so this does not change which "git" or "sh" a
// symlink shadows relative to the ambient PATH.
func pathWithoutCmp(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(PathOutsideGates("")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if name == "cmp" || seen[name] {
				continue
			}
			seen[name] = true
			if err := os.Symlink(filepath.Join(dir, name), filepath.Join(bin, name)); err != nil {
				continue
			}
		}
	}
	if _, err := os.Stat(filepath.Join(bin, "cmp")); !os.IsNotExist(err) {
		t.Fatalf("pathWithoutCmp must not offer cmp, stat err=%v", err)
	}
	return bin
}

// qaGuardRepo builds a throwaway repo carrying the shared-index guard and
// one commit on main, and returns a runner that drives git as a persona
// (extra = the persona env) or, with nil, as the operator. HOME is the repo
// so nothing here reads the operator's ~/.gitconfig.
func qaGuardRepo(t *testing.T) (string, func(extra []string, args ...string) (string, error)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo, "GIT_EDITOR=true",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(extra []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), env...), extra...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	git(nil, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "a.txt")
	git(nil, "commit", "-qm", "a1", "--", "a.txt")
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	return repo, git
}

// bdShimBody is `bd hooks install`'s real 0.49.x shim for a slot, trimmed to
// the parts that decide anything: the `# bd-shim v1` header --chain keys on,
// and the `exec bd hooks run <slot>` body. Planted verbatim rather than
// paraphrased, because both facts below turn on posse recognizing this exact
// shape — and on the launch NOT taking it over.
func bdShimBody(slot string) string {
	return "#!/usr/bin/env sh\n# bd-shim v1\n# bd-hooks-version: 0.49.1\n" +
		"if ! command -v bd >/dev/null 2>&1; then\n    exit 0\nfi\n" +
		"exec bd hooks run " + slot + " \"$@\"\n"
}

// qaWorkingForeignChain is a foreign hook that PASSES all three of §9's own
// printed probes: it refuses under RHQ_PERSONA with exit 1 in either slot,
// and stands down (exit 0) for the operator. Nothing about it is broken —
// which is the whole point of ranger-base-nlhz.
const qaWorkingForeignChain = `#!/bin/sh
if [ -n "$RHQ_PERSONA" ]; then echo "refused by posse gate: foreign but working"; exit 1; fi
exit 0
`

// qaEnvWithout is os.Environ() with the named variables removed, so a probe
// that must run as the operator does not inherit this process's persona env.
func qaEnvWithout(names ...string) []string {
	var out []string
	for _, e := range os.Environ() {
		drop := false
		for _, n := range names {
			if strings.HasPrefix(e, n+"=") {
				drop = true
			}
		}
		if !drop {
			out = append(out, e)
		}
	}
	return out
}
