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
			"[ -x \"$d/bd-" + slot + "\" ] || exit 0\n" +
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
	// The form that passes is the path-limited one — git's own next-index
	// temp file in the git dir — for a persona and, since rangerhq-lt2w,
	// for a shell with no RHQ_PERSONA too: the carve-out that used to make
	// THIS the passing arm is gone, so an unqualified commit is refused
	// whoever types it.
	if out, code := runHook(t, repo, "prepare-commit-msg", ""); code != 1 ||
		!strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("no RHQ_PERSONA must be refused too (rangerhq-lt2w): code=%d %q", code, out)
	}
	if read() != "" {
		t.Errorf("a refused commit must not reach bd's hook: %q", read())
	}
	safe := "GIT_INDEX_FILE=" + filepath.Join(repo, ".git", "next-index-4242")
	if out, code := runHook(t, repo, "prepare-commit-msg", "", safe); code != 0 {
		t.Errorf("a path-limited commit must pass: code=%d %q", code, out)
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
	repo, _ := qaChainRepo(t)
	if err := os.Remove(filepath.Join(repo, ".git", "hooks", "bd-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	safe := "GIT_INDEX_FILE=" + filepath.Join(repo, ".git", "next-index-4242")
	if out, code := runHook(t, repo, "prepare-commit-msg", "", safe); code != 0 {
		t.Errorf("a commit the gate passes must survive a missing neighbour: code=%d %q", code, out)
	}
	// A neighbour that is there but not executable execs to 126 just the
	// same, so the guard is -x, not -e.
	if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", "bd-prepare-commit-msg"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "prepare-commit-msg", "", safe); code != 0 {
		t.Errorf("a commit the gate passes must survive a non-executable neighbour: code=%d %q", code, out)
	}
	// And the degrade is only about the neighbour: the gate behind the
	// dispatcher still refuses what it refused before.
	if out, code := runHook(t, repo, "prepare-commit-msg", "", "RHQ_PERSONA=probe"); code != 1 ||
		!strings.Contains(out, "refused by posse gate") {
		t.Errorf("the gate must still refuse with a missing neighbour: code=%d %q", code, out)
	}
	// The same for pre-push, whose exec carries git's ref list.
	if err := os.Remove(filepath.Join(repo, ".git", "hooks", "bd-pre-push")); err != nil {
		t.Fatal(err)
	}
	if out, code := runHook(t, repo, "pre-push", "refs/heads/main a1 refs/heads/main b1\n"); code != 0 {
		t.Errorf("a push must survive a missing neighbour: code=%d %q", code, out)
	}
}

// rangerhq-xo65, the same defect from git's side: with the chain naming a
// neighbour that is not there, the commit the gate itself PASSES — the
// path-limited form it prescribes, typed here with no RHQ_PERSONA at all —
// must still land. Since rangerhq-lt2w that shell is walled like any other,
// which is exactly why this arm matters: an exec failure would take the one
// form left with it.
func TestQAOperatorCanCommitThroughAChainMissingItsNeighbour(t *testing.T) {
	repo, _ := qaChainRepo(t)
	if err := os.Remove(filepath.Join(repo, ".git", "hooks", "bd-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "X.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = []string{"PATH=" + PathOutsideGates(""), "HOME=" + t.TempDir(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := git("add", "X.md"); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if out, err := git("commit", "-qm", "operator commit", "--", "X.md"); err != nil {
		t.Errorf("the operator's own commit must not be blocked by a missing neighbour: %v %s", err, out)
	}
}

// rangerhq-lrnp: the guard's own comment exempted "the commits git drives
// itself" on the strength of a marker file in the git dir, and a clean
// `git revert` writes none of them before prepare-commit-msg runs (git
// 2.39.3) — so the persona was refused under a comment promising otherwise,
// and refused only AFTER git had staged the revert into the shared index the
// guard exists to protect. The verdict kept is the refusal (no exemption is
// safe: the only two signals a clean revert leaves, MERGE_MSG and
// AUTO_MERGE, both outlive the operation that wrote them, and the last arm
// here pins that). What changed is that the refusal now names the two-step
// way through and the bounded dirt it is leaving behind. All of it is run,
// not read.
func TestQAGuardRefusesACleanRevertAndNamesTheWayThrough(t *testing.T) {
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
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	status := func() string {
		out, _ := git(nil, "status", "--porcelain")
		return strings.TrimSpace(out)
	}
	git(nil, "init", "-q", "-b", "main")
	write("a.txt", "a")
	git(nil, "add", "a.txt")
	git(nil, "commit", "-qm", "add a", "--", "a.txt")
	write("b.txt", "b")
	git(nil, "add", "b.txt")
	git(nil, "commit", "-qm", "add b", "--", "b.txt")
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}

	// A clean revert is refused — and the refusal has to be usable, because
	// git has already staged the revert by the time it prints.
	out, err := git(persona, "revert", "--no-edit", "HEAD")
	if err == nil {
		t.Fatalf("a clean revert is refused (no marker exists to exempt it): %s", out)
	}
	for _, want := range []string{
		"refused by posse gate",
		"git prepared this commit itself (revert)",
		"finish it:  git commit -F - -- b.txt",
		"or undo it: git restore --source=HEAD --staged --worktree -- b.txt",
		"next time:  git revert --no-commit <sha>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must name the way through and the dirt it leaves, missing %q:\n%s", want, out)
		}
	}
	// The dirt is bounded: git revert only starts from an index matching
	// HEAD, so the refusal leaves exactly the revert and nothing of another
	// persona's — which is what makes the path-limited undo above correct.
	if st := status(); st != "D  b.txt" {
		t.Errorf("the refusal leaves exactly the revert staged, got %q", st)
	}
	if out, err := git(persona, "restore", "--source=HEAD", "--staged", "--worktree", "--", "b.txt"); err != nil {
		t.Fatalf("the undo the refusal names must work: %v %s", err, out)
	}
	if st := status(); st != "" {
		t.Errorf("after the named undo the tree is clean, got %q", st)
	}

	// The two-step way through, end to end under the gate. The second step
	// needs no exemption: a path-limited commit gets its own next-index temp
	// index even mid-revert, so it passes on its own merits.
	if out, err := git(persona, "revert", "--no-commit", "HEAD"); err != nil {
		t.Fatalf("git revert --no-commit stages without committing: %v %s", err, out)
	}
	if out, err := git(persona, "commit", "-m", "Revert add b", "--", "b.txt"); err != nil {
		t.Fatalf("the path-limited commit must land the revert: %v %s", err, out)
	}
	if st := status(); st != "" {
		t.Errorf("the way through leaves a clean tree, got %q", st)
	}
	if out, _ := git(nil, "log", "--oneline", "-1"); !strings.Contains(out, "Revert add b") {
		t.Errorf("HEAD must carry the revert: %q", out)
	}
	for _, marker := range []string{"REVERT_HEAD", "MERGE_MSG", "AUTO_MERGE", "sequencer"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", marker)); err == nil {
			t.Errorf("the way through leaves no %s behind", marker)
		}
	}

	// And the arm that says why no exemption was added: AUTO_MERGE OUTLIVES
	// the revert that wrote it. Since rangerhq-lt2w no shell completes a
	// clean revert in one step — the guard covers the operator too — so the
	// lingering file is produced the only way it still can be: with the hook
	// out of the slot, which is also what a repo hooked later looks like.
	// Then the guard goes back in and the next unqualified persona commit
	// must still be refused; exempting on that file would have taken the
	// wall down for good.
	slot := filepath.Join(repo, ".git", "hooks", "prepare-commit-msg")
	body, err := os.ReadFile(slot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(slot); err != nil {
		t.Fatal(err)
	}
	if out, err := git(nil, "revert", "--no-edit", "HEAD"); err != nil {
		t.Fatalf("a revert with no hook in the slot must complete: %v %s", err, out)
	}
	if err := os.WriteFile(slot, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "AUTO_MERGE")); err != nil {
		t.Fatalf("AUTO_MERGE is expected to linger past a completed revert: %v", err)
	}
	write("c.txt", "c")
	git(nil, "add", "c.txt")
	if out, err := git(persona, "commit", "-m", "sweep"); err == nil ||
		!strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("a lingering AUTO_MERGE must not exempt anything: %v %s", err, out)
	}
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

// ranger-base-08a2. The guard's git-driven exemption was written as "an
// operation is in progress", justified as "git refuses a pathspec during
// those outright". Measured on git 2.39.3, that premise holds for exactly
// two of the five markers, and each arm below is one row of the table in
// sharedIndexBody's doc comment — run rather than read.
//
// The subtests each get their own repo: these are git *states*, and one
// probe's cleanup is the next probe's fixture otherwise.
func TestQAGuardExemptsOnlyWhereGitRefusesAPathspec(t *testing.T) {
	personaEnv := func(t *testing.T) []string {
		return []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	}
	writeIn := func(t *testing.T, repo, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// diverge leaves main at a3 with a2 revertable-with-conflict, which is
	// the second of the two REVERT_HEAD states.
	diverge := func(t *testing.T, repo string, git func([]string, ...string) (string, error)) string {
		t.Helper()
		writeIn(t, repo, "a.txt", "a2\n")
		git(nil, "commit", "-qm", "a2", "--", "a.txt")
		sha, _ := git(nil, "rev-parse", "HEAD")
		writeIn(t, repo, "a.txt", "a3\n")
		git(nil, "commit", "-qm", "a3", "--", "a.txt")
		return strings.TrimSpace(sha)
	}

	// The window rangerhq-lrnp's own blessed recipe opens: `git revert
	// --no-commit` DOES write REVERT_HEAD, so under the old list every
	// unqualified commit was exempt for as long as the revert was unfinished
	// — rangerhq-nyqj, inside the way through.
	t.Run("revert --no-commit exempts nothing", func(t *testing.T) {
		repo, git := qaGuardRepo(t)
		persona := personaEnv(t)
		writeIn(t, repo, "b.txt", "b\n")
		git(nil, "add", "b.txt")
		git(nil, "commit", "-qm", "add b", "--", "b.txt")
		if out, err := git(persona, "revert", "--no-commit", "HEAD"); err != nil {
			t.Fatalf("git revert --no-commit: %v %s", err, out)
		}
		if _, err := os.Stat(filepath.Join(repo, ".git", "REVERT_HEAD")); err != nil {
			t.Fatalf("the fixture is the exemption's own trigger; REVERT_HEAD must be there: %v", err)
		}
		writeIn(t, repo, "theirs.txt", "theirs\n")
		git(nil, "add", "theirs.txt")
		out, err := git(persona, "commit", "-m", "sweep")
		if err == nil || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
			t.Fatalf("REVERT_HEAD must not exempt an unqualified commit: %v %s", err, out)
		}
		if !strings.Contains(out, "A revert is in progress (REVERT_HEAD)") {
			t.Errorf("the refusal must say a revert is in progress:\n%s", out)
		}
		if out, _ := git(nil, "diff", "--cached", "--name-only", "HEAD"); !strings.Contains(out, "theirs.txt") {
			t.Errorf("the other persona's staged file must survive the refusal, got %q", out)
		}
		// And the exemption bought nothing, because the safe form works here.
		if out, err := git(persona, "commit", "-m", "Revert add b", "--", "b.txt"); err != nil {
			t.Fatalf("a pathspec IS accepted mid-revert — that is why no exemption is owed: %v %s", err, out)
		}
	})

	// A conflicted revert finished with a message of the persona's own:
	// $2 is "message", so only the marker ever exempted this.
	t.Run("a conflicted revert finished by hand is refused", func(t *testing.T) {
		repo, git := qaGuardRepo(t)
		persona := personaEnv(t)
		sha := diverge(t, repo, git)
		if out, err := git(persona, "revert", "--no-edit", sha); err == nil {
			t.Fatalf("the fixture needs a CONFLICTED revert: %s", out)
		}
		writeIn(t, repo, "a.txt", "resolved\n")
		git(nil, "add", "a.txt")
		out, err := git(persona, "commit", "-m", "mine")
		if err == nil || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
			t.Fatalf("a conflicted revert's REVERT_HEAD must not exempt either: %v %s", err, out)
		}
		if !strings.Contains(out, "finish it:  git commit -F - -- <the paths that are yours>") {
			t.Errorf("the refusal must name the form that works here:\n%s", out)
		}
		// The path-limited commit ENDS the revert — this is why refusing
		// costs nothing, and it is the claim the whole change rests on.
		if out, err := git(persona, "commit", "-m", "mine", "--", "a.txt"); err != nil {
			t.Fatalf("the way through must land: %v %s", err, out)
		}
		for _, marker := range []string{"REVERT_HEAD", "MERGE_MSG", "AUTO_MERGE", "sequencer"} {
			if _, err := os.Stat(filepath.Join(repo, ".git", marker)); err == nil {
				t.Errorf("the path-limited commit must finish the revert, %s left behind", marker)
			}
		}
		if out, _ := git(nil, "revert", "--continue"); !strings.Contains(out, "no cherry-pick or revert in progress") {
			t.Errorf("git itself must agree the revert is over, got %q", out)
		}
	})

	// `git revert --continue` reaches the slot as $2=merge, so it was the
	// `case "$2" in merge|squash)` arm that carried it, not REVERT_HEAD.
	// Both are gone: it is refused, worded by the message-file test, and the
	// same path-limited commit is the way on.
	t.Run("git revert --continue is refused", func(t *testing.T) {
		repo, git := qaGuardRepo(t)
		persona := personaEnv(t)
		sha := diverge(t, repo, git)
		git(persona, "revert", "--no-edit", sha)
		writeIn(t, repo, "a.txt", "resolved\n")
		git(nil, "add", "a.txt")
		out, err := git(persona, "revert", "--continue")
		if err == nil || !strings.Contains(out, "refused by posse gate") {
			t.Fatalf("git revert --continue must not be exempt: %v %s", err, out)
		}
		if !strings.Contains(out, "git prepared this commit itself (revert)") {
			t.Errorf("the message-file test must word this one:\n%s", out)
		}
		if out, err := git(persona, "commit", "-m", "Revert a2", "--", "a.txt"); err != nil {
			t.Fatalf("the way through must land: %v %s", err, out)
		}
	})

	// `git merge --squash` is the same hole one arm over: SQUASH_MSG, no
	// MERGE_HEAD, $2=squash on the bare commit git invites — and a pathspec
	// is accepted throughout.
	t.Run("merge --squash exempts nothing", func(t *testing.T) {
		repo, git := qaGuardRepo(t)
		persona := personaEnv(t)
		git(nil, "checkout", "-q", "-b", "side")
		writeIn(t, repo, "s.txt", "s\n")
		git(nil, "add", "s.txt")
		git(nil, "commit", "-qm", "side", "--", "s.txt")
		git(nil, "checkout", "-q", "main")
		writeIn(t, repo, "a.txt", "a2\n")
		git(nil, "commit", "-qm", "a2", "--", "a.txt")
		if out, err := git(nil, "merge", "--squash", "side"); err != nil {
			t.Fatalf("git merge --squash: %v %s", err, out)
		}
		if _, err := os.Stat(filepath.Join(repo, ".git", "SQUASH_MSG")); err != nil {
			t.Fatalf("the fixture needs SQUASH_MSG, which is what makes $2 squash: %v", err)
		}
		writeIn(t, repo, "theirs.txt", "theirs\n")
		git(nil, "add", "theirs.txt")
		// Bare, so git names the message source itself ($2=squash) — the
		// arm that used to exit 0 before anything else was even read.
		out, err := git(persona, "commit")
		if err == nil || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
			t.Fatalf("$2=squash must not exempt an unqualified commit: %v %s", err, out)
		}
		if out, err := git(persona, "commit", "-m", "squashed", "--", "s.txt"); err != nil {
			t.Fatalf("a pathspec IS accepted after --squash: %v %s", err, out)
		}
		if out, _ := git(nil, "diff", "--cached", "--name-only", "HEAD"); !strings.Contains(out, "theirs.txt") {
			t.Errorf("the other persona's staged file must survive, got %q", out)
		}
	})

	// The positive control, and the exemption's stated justification run
	// rather than read: in these two states git refuses a pathspec itself,
	// so a refusal would leave no way through. Note the persona's commit
	// carries its OWN message ($2=message) — the marker is what holds it.
	for _, tc := range []struct {
		name, fatal string
		start       func(t *testing.T, repo string, git func([]string, ...string) (string, error), sha string) string
		marker      string
	}{
		{
			name: "a conflicted merge stays exempt", fatal: "cannot do a partial commit during a merge",
			marker: "MERGE_HEAD",
			start: func(t *testing.T, repo string, git func([]string, ...string) (string, error), sha string) string {
				out, _ := git(nil, "merge", "--no-edit", "side")
				return out
			},
		},
		{
			name: "a conflicted cherry-pick stays exempt", fatal: "cannot do a partial commit during a cherry-pick",
			marker: "CHERRY_PICK_HEAD",
			start: func(t *testing.T, repo string, git func([]string, ...string) (string, error), sha string) string {
				out, _ := git(nil, "cherry-pick", sha)
				return out
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, git := qaGuardRepo(t)
			persona := personaEnv(t)
			git(nil, "checkout", "-q", "-b", "side")
			writeIn(t, repo, "a.txt", "side\n")
			git(nil, "commit", "-qm", "side", "--", "a.txt")
			sideSha, _ := git(nil, "rev-parse", "HEAD")
			git(nil, "checkout", "-q", "main")
			writeIn(t, repo, "a.txt", "mainc\n")
			git(nil, "commit", "-qm", "mainc", "--", "a.txt")
			if out := tc.start(t, repo, git, strings.TrimSpace(sideSha)); !strings.Contains(out, "CONFLICT") {
				t.Fatalf("the fixture needs a conflict: %s", out)
			}
			if _, err := os.Stat(filepath.Join(repo, ".git", tc.marker)); err != nil {
				t.Fatalf("the fixture needs %s: %v", tc.marker, err)
			}
			writeIn(t, repo, "a.txt", "resolved\n")
			git(nil, "add", "a.txt")
			// git's own refusal, which is the whole justification for the
			// exemption. If this ever stops being fatal, the exemption is
			// owed a re-measurement, not a rename.
			if out, err := git(persona, "commit", "-m", "partial", "--", "a.txt"); err == nil ||
				!strings.Contains(out, tc.fatal) {
				t.Fatalf("git must refuse a pathspec here — the exemption rests on it: %v %s", err, out)
			}
			if out, err := git(persona, "commit", "-m", "mine"); err != nil ||
				strings.Contains(out, "refused by posse gate") {
				t.Fatalf("the only form git allows here must pass the gate: %v %s", err, out)
			}
		})
	}

	// The residual this bead did NOT close, pinned so that closing it is a
	// decision rather than a drift: a pathspec IS accepted mid-rebase, yet
	// `git rebase --continue` still has commits to replay and reaches the
	// slot as $2=message with $GIT_DIR/index — nothing tells it apart from a
	// typed `git commit`. GIT_REFLOG_ACTION would, and is the caller's to
	// spell (rangerhq-cqq1). So during a rebase the wall is down.
	t.Run("a rebase stays exempt, and that is the residual", func(t *testing.T) {
		repo, git := qaGuardRepo(t)
		persona := personaEnv(t)
		writeIn(t, repo, "a.txt", "a2\n")
		git(nil, "commit", "-qm", "a2", "--", "a.txt")
		git(nil, "checkout", "-q", "-b", "side", "HEAD~1")
		writeIn(t, repo, "a.txt", "side\n")
		git(nil, "commit", "-qm", "side", "--", "a.txt")
		if out, err := git(nil, "rebase", "main"); err == nil {
			t.Fatalf("the fixture needs a conflicted rebase: %s", out)
		}
		writeIn(t, repo, "a.txt", "resolved\n")
		git(nil, "add", "a.txt")
		// A pathspec works here — which is why this exemption is wider than
		// its justification, and why the comment says so out loud.
		if out, err := git(persona, "commit", "-m", "partial", "--", "a.txt"); err != nil {
			t.Fatalf("a pathspec is accepted mid-rebase: %v %s", err, out)
		}
		writeIn(t, repo, "theirs.txt", "theirs\n")
		git(nil, "add", "theirs.txt")
		if out, err := git(persona, "rebase", "--continue"); err != nil ||
			strings.Contains(out, "refused by posse gate") {
			t.Fatalf("git rebase --continue is still exempt (the residual): %v %s", err, out)
		}
	})
}

// rangerhq-b38m: git runs hooks from core.hooksPath when it is set, and
// hooksDir() never read it — so both gates landed in .git/hooks, install
// reported success, §9's probes (which run the file directly) went green, and
// git ran none of it. Closed with ranger-base-flz7, which was the same defect
// seen from the probe's side: hooksDir() now asks `git rev-parse --git-path
// hooks`, so install and probe address the one directory git dispatches from.
func TestQAInstallHooksHonoursCoreHooksPath(t *testing.T) {
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

// rangerhq-mgdk: the CLI's own --chain, exercised against a bd-shimmed
// repo the way TestQASessionCreateInstallsNothingIntoABdHookedRepo shows
// dispatch leaves untouched. Both InstallXxxChained calls must build the
// same chain INSTALL.md §9 walks by hand — bd's shim moved to bd-<slot>,
// ours at posse-<slot>, the real slot holding the dispatcher — and the
// gate must still run first and bd's shim must still be reachable behind
// it. hookInstalled() (via PrePushHookInstalled/CommitGuardHookInstalled)
// must see through the result, since neither slot carries our marker
// directly once chained.
func TestQAChainedInstallTakesOverBdsShimAndStaysDetected(t *testing.T) {
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
	witness := filepath.Join(t.TempDir(), "bd.log")
	shim := "#!/bin/sh\n# bd-shim v1\nprintf 'ran[%s]\\n' \"$0\" >> " + witness + "\nexit 0\n"
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(shim), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if PrePushHookInstalled(repo) || CommitGuardHookInstalled(repo) {
		t.Fatal("a repo holding only bd's shim must not report our gate installed")
	}

	if _, err := InstallPrePushHookChained(repo); err != nil {
		t.Fatalf("chained pre-push install must take over bd's shim: %v", err)
	}
	if _, _, _, err := (&App{}).InstallCommitGuardHookChained(repo); err != nil {
		t.Fatalf("chained commit-guard install must take over bd's shim: %v", err)
	}

	if !PrePushHookInstalled(repo) || !CommitGuardHookInstalled(repo) {
		t.Error("hookInstalled must see through the chain it just built")
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(hooks, "bd-"+slot)); err != nil {
			t.Errorf("bd's shim must survive, moved aside: %v", err)
		}
		if _, err := os.Stat(filepath.Join(hooks, "posse-"+slot)); err != nil {
			t.Errorf("our gate must be installed behind the dispatcher: %v", err)
		}
	}

	if out, code := runHook(t, repo, "pre-push", "refs/heads/main a refs/heads/main b\n",
		"RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)"); code != 1 || !strings.Contains(out, "refused by posse gate") {
		t.Errorf("denied push must still refuse through the chain: code=%d %q", code, out)
	}
	if b, _ := os.ReadFile(witness); len(b) != 0 {
		t.Errorf("a refused push must not reach bd's shim: %q", b)
	}
	if out, code := runHook(t, repo, "pre-push", "refs/heads/main a refs/heads/main b\n"); code != 0 {
		t.Errorf("an allowed push must fall through to bd's shim: code=%d %q", code, out)
	}
	if b, _ := os.ReadFile(witness); !strings.Contains(string(b), "bd-pre-push") {
		t.Errorf("bd's shim must have run: %q", b)
	}
}

// rangerhq-mgdk: --chain only recognizes bd's own shim (the `# bd-shim v1`
// header). A hook of unknown shape must still be refused, chain or no
// chain — chaining an arbitrary foreign hook without knowing its exit-code
// semantics is exactly the risk the manual prescription exists to make an
// operator, not rhq, decide.
func TestQAChainedInstallStillRefusesAGenuinelyUnknownHook(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPrePushHookChained(repo); err == nil || !strings.Contains(err.Error(), "not a posse hook") {
		t.Fatalf("an unrecognized foreign hook must still be refused under --chain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hooks, "bd-pre-push")); err == nil {
		t.Error("a non-bd hook must not be moved aside as if it were bd's shim")
	}
}

// rangerhq-xo65: §9's hand-built chain and the one posse writes itself
// (chainBdShim / the printed prescription) are the same arrangement, and a
// fix applied to one and not the other leaves half the fleet dead at the
// next missing neighbour. The doc is the operator's copy of chainRender —
// pin it byte for byte, with bd's names filled in, so drift is a red test
// rather than a repo that cannot commit.
func TestQADocChainMatchesTheRenderedDispatcher(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		want := chainHookDispatcherWith(slot, "bd-"+slot)
		// The block as INSTALL.md pastes it: a heredoc into the slot.
		// §9 asks git for the dispatch dir rather than assuming .git/hooks
		// (rangerhq-b38m), so the heredoc target is `"$h"/<slot>`.
		open := "$ cat > \"$h\"/" + slot + " <<'EOF'\n"
		i := strings.Index(doc, open)
		if i < 0 {
			t.Errorf("INSTALL.md §9 no longer writes %s with a heredoc", slot)
			continue
		}
		rest := doc[i+len(open):]
		j := strings.Index(rest, "EOF\n")
		if j < 0 {
			t.Errorf("INSTALL.md §9's %s heredoc has no terminator", slot)
			continue
		}
		if got := rest[:j]; got != want {
			t.Errorf("INSTALL.md §9's %s chain has drifted from chainHookDispatcherWith:\n got %q\nwant %q", slot, got, want)
		}
	}
}

// rangerhq-xo65: every repo chained before the fix carries the unguarded
// dispatcher, and nothing about it is ever rewritten — so the fix would reach
// new installs only and leave the existing fleet one missing neighbour away
// from a repo that cannot commit. installHook already refreshes the
// posse-<slot> behind a recognized chain on every launch; it upgrades the
// dispatcher itself in the same pass. Safe because the body matched our own
// render byte for byte: we know what it is, and we write back the same shape
// and the same neighbour.
func TestQALegacyChainIsUpgradedInPlace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		t.Run(slot, func(t *testing.T) {
			repo := t.TempDir()
			if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			hooks := filepath.Join(repo, ".git", "hooks")
			install := InstallPrePushHook
			if slot == "prepare-commit-msg" {
				install = installCommitGuard
			}
			if _, err := install(repo); err != nil {
				t.Fatal(err)
			}
			// The pre-fix arrangement, exactly as an older posse left it.
			if err := os.Rename(filepath.Join(hooks, slot), filepath.Join(hooks, "posse-"+slot)); err != nil {
				t.Fatal(err)
			}
			legacy := legacyChainHookDispatcherWith(slot, "bd-"+slot)
			if err := os.WriteFile(filepath.Join(hooks, slot), []byte(legacy), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(hooks, "bd-"+slot), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}

			// A launch-time reinstall, the pass that already refreshes the
			// chained member.
			got, err := install(repo)
			if err != nil {
				t.Fatalf("reinstall over a pre-xo65 chain must succeed: %v", err)
			}
			if want := filepath.Join(hooks, "posse-"+slot); got != want {
				t.Errorf("reinstall must report the chained member: got %q want %q", got, want)
			}
			b, err := os.ReadFile(filepath.Join(hooks, slot))
			if err != nil {
				t.Fatal(err)
			}
			if want := chainHookDispatcherWith(slot, "bd-"+slot); string(b) != want {
				t.Errorf("the dispatcher must be upgraded in place:\n got %q\nwant %q", string(b), want)
			}
			// The neighbour it names is untouched, and a foreign hook stays foreign.
			if nb, err := os.ReadFile(filepath.Join(hooks, "bd-"+slot)); err != nil || string(nb) != "#!/bin/sh\nexit 0\n" {
				t.Errorf("the neighbour must not be rewritten: %q %v", string(nb), err)
			}
		})
	}
}

// The upgrade is only for a body that is byte-for-byte one of our own
// renders. A foreign hook that merely looks chain-shaped is still refused,
// untouched — ADR 0002 §3.
func TestQALegacyUpgradeLeavesForeignHooksAlone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	slot := filepath.Join(hooks, "prepare-commit-msg")
	if err := os.Rename(slot, filepath.Join(hooks, "posse-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	// Ours dispatched, but its exit status discarded — not a chain.
	foreign := "#!/bin/sh\nd=$(dirname \"$0\")\n\"$d/posse-prepare-commit-msg\" \"$@\"\nexec \"$d/bd-prepare-commit-msg\" \"$@\"\n"
	if err := os.WriteFile(slot, []byte(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := installCommitGuard(repo); err == nil {
		t.Error("a foreign hook must still be refused, not upgraded")
	}
	if b, _ := os.ReadFile(slot); string(b) != foreign {
		t.Errorf("a refused hook must be left byte-identical: %q", string(b))
	}
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

// QA, rangerhq-71ki (verify of rangerhq-j4sq) — the same claim as
// TestQASessionCreateInstallsNothingIntoABdHookedRepo, but read through
// CreateSession itself instead of through the two functions it happens to
// call today. That distinction is not academic: flipping herdrback.go's
// InstallPrePushHook to InstallPrePushHookChained (which exists, since
// rangerhq-mgdk) makes INSTALL.md §9's "Session create itself does not pass
// `--chain`" false while the older pin — and, measured 2026-08-28, the whole
// of ./internal/rhq — stays green.
//
// Two facts, both of which §9 states and neither of which was pinned at the
// launch boundary:
//  1. a dispatch into a bd-hooked repo installs NOTHING: both slots stay
//     byte-identical bd shims and no posse-<slot>/bd-<slot> chain appears.
//  2. it is not silent about it. The L3 behavior probe (ranger-base-3c3)
//     degrades naming both slots, and the launch refuses unless the operator
//     passes --allow-degraded.
func TestQADispatchIntoABdHookedRepoInstallsNothingAndRefuses(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	b, _ := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	slots := []string{"pre-push", "prepare-commit-msg"}
	for _, slot := range slots {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(bdShimBody(slot)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Fact 2 first, because it is what the operator actually meets.
	err := b.CreateSession(NewSessionOpts{Name: "s1", Agent: "dev", Runtime: "claude", Dir: repo})
	if err == nil {
		t.Fatal("a dispatch into a bd-hooked repo must refuse: neither gate is realized there")
	}
	for _, slot := range slots {
		if !strings.Contains(err.Error(), slot) {
			t.Errorf("the refusal must name %s — §9 tells the operator both slots are bd's:\n%v", slot, err)
		}
	}
	if !strings.Contains(err.Error(), "--allow-degraded") {
		t.Errorf("the refusal must name the way through:\n%v", err)
	}

	// Fact 1: waive it, and the launch still puts no gate in the repo.
	mustCreate(t, b, NewSessionOpts{Name: "s2", Agent: "dev", Runtime: "claude", Dir: repo, AllowDegraded: true})
	if m, _ := b.readMeta("s2"); m == nil || !strings.Contains(m.Degraded, "foreign hook") {
		t.Errorf("a waived launch must stay marked as degraded: %+v", m)
	}
	for _, slot := range slots {
		got, err := os.ReadFile(filepath.Join(hooks, slot))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != bdShimBody(slot) {
			t.Fatalf("%s: the launch changed the slot — INSTALL.md §9 says a dispatch cannot, and that session create does not pass --chain:\n%s", slot, got)
		}
		for _, side := range []string{"posse-" + slot, "bd-" + slot, "theirs-" + slot} {
			if _, err := os.Stat(filepath.Join(hooks, side)); err == nil {
				t.Fatalf("%s exists: the launch built a chain by itself — §9's closing paragraph needs re-checking", side)
			}
		}
	}
	ents, err2 := os.ReadDir(hooks)
	if err2 != nil {
		t.Fatal(err2)
	}
	for _, e := range ents {
		body, _ := os.ReadFile(filepath.Join(hooks, e.Name()))
		if strings.Contains(string(body), "posse-gate") {
			t.Fatalf("found a posse gate at %s — a dispatch installed one after all", e.Name())
		}
	}
}

// qaWorkingForeignChain is a foreign hook that PASSES all three of §9's own
// printed probes: it refuses under RHQ_PERSONA with exit 1 in either slot,
// and stands down (exit 0) for the operator. Nothing about it is broken —
// which is the whole point of ranger-base-nlhz.
const qaWorkingForeignChain = `#!/bin/sh
if [ -n "$RHQ_PERSONA" ]; then echo "refused by posse gate: foreign but working"; exit 1; fi
exit 0
`

// ranger-base-nlhz: §9 used to tell the operator "a working foreign chain
// passes", and explained the bd-hooked repo's refusal as the behavioral probe
// finding neither slot exits 1. Since ADR 0023 neither is true: the launch
// never execs the dispatched file, and its verdict is byte identity against
// posse's own render. This pins the fact and the sentence together — the
// sibling test above asserts only THAT a bd-hooked repo refuses, which is why
// it stayed green while the doc's account of the cause went stale.
//
// The fixture carries its own witness: the three probes are run against the
// planted chain first, so a hook that silently failed to be "working" is a
// red test rather than a refusal credited to the wrong cause.
func TestQAWorkingForeignChainIsRefusedOnIdentityNotBehavior(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	b, _ := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	slots := []string{"pre-push", "prepare-commit-msg"}
	for _, slot := range slots {
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(qaWorkingForeignChain), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Witness: §9's three probes, verbatim, against what was just planted.
	msg := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(msg, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	probes := []struct {
		what string
		cmd  *exec.Cmd
		env  []string
		want int
	}{
		{"pre-push under a persona", exec.Command("sh", "-c",
			`printf 'refs/heads/main a refs/heads/main b\n' | .git/hooks/pre-push origin x`),
			[]string{"RHQ_PERSONA=probe", "RHQ_TOOLS_DENY=Bash(git push:*)"}, 1},
		{"prepare-commit-msg under a persona",
			exec.Command(filepath.Join(hooks, "prepare-commit-msg"), msg),
			[]string{"RHQ_PERSONA=probe"}, 1},
		{"prepare-commit-msg for the operator",
			exec.Command(filepath.Join(hooks, "prepare-commit-msg"), msg, "message"),
			nil, 0},
	}
	for _, p := range probes {
		p.cmd.Dir = repo
		p.cmd.Env = append(qaEnvWithout("RHQ_PERSONA", "RHQ_TOOLS_DENY"), p.env...)
		out, err := p.cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%s: %v", p.what, err)
		}
		if code != p.want {
			t.Fatalf("the fixture is not a working chain — %s exited %d, want %d:\n%s", p.what, code, p.want, out)
		}
	}

	// And it is refused anyway, on identity.
	err := b.CreateSession(NewSessionOpts{Name: "s1", Agent: "dev", Runtime: "claude", Dir: repo})
	if err == nil {
		t.Fatal("a foreign hook is refused however well it behaves (ADR 0023) — this one passed all three probes and the launch let it through")
	}
	for _, slot := range slots {
		if !strings.Contains(err.Error(), slot) {
			t.Errorf("the refusal must name %s:\n%v", slot, err)
		}
	}
	if !strings.Contains(err.Error(), "a hook it did not write") {
		t.Errorf("the refusal must name identity as the reason — §9 explains it that way:\n%v", err)
	}
	if !strings.Contains(err.Error(), "--allow-degraded") {
		t.Errorf("the refusal must name the way through:\n%v", err)
	}
	mustCreate(t, b, NewSessionOpts{Name: "s2", Agent: "dev", Runtime: "claude", Dir: repo, AllowDegraded: true})
	if m, _ := b.readMeta("s2"); m == nil || !strings.Contains(m.Degraded, "foreign hook") {
		t.Errorf("a waived launch must stay marked as degraded: %+v", m)
	}

	// The sentence, next to the fact.
	doc, err2 := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err2 != nil {
		t.Fatal(err2)
	}
	for _, gone := range []string{
		"a working foreign\nchain passes",
		"the behavioral probe in the paragraph above\nfinds neither slot exits 1",
	} {
		if strings.Contains(string(doc), gone) {
			t.Errorf("INSTALL.md §9 still explains the launch verdict with behavior of the dispatched hook: %q", gone)
		}
	}
	for _, want := range []string{
		"a foreign hook is refused however well it behaves",
		"byte identity",
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("INSTALL.md §9 no longer states the ADR 0023 verdict: %q", want)
		}
	}
}

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
