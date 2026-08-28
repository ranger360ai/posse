package rhq

// QA, rangerhq-705k — verifying the close of rangerhq-cqq1 (the private-index
// refusal was one FILENAME wide). The close matched git's own temp index by
// LOCATION and pid shape instead of by spelling, and pinned it in
// TestSharedIndexCommitHookRefusesHandRolledNextIndex. Two things that pin
// does not reach, and both are where the claim is actually load-bearing:
//
//   - it installs the guard DIRECTLY into the slot, while every repo the crew
//     commits in carries the INSTALL.md §9 chain (a dispatcher that runs
//     posse-prepare-commit-msg and checks its status) because bd owns that
//     slot too. A wall verified only in the unchained shape is verified in a
//     shape nobody runs;
//   - the residual the fix deliberately left — `GIT_INDEX_FILE=$GIT_DIR/
//     next-index-<digits>`, a private index placed inside the repo's own git
//     dir — is asserted in gates.go and in NOTES.md and measured nowhere. It
//     is the one spelling that still reproduces rangerhq-8rtf end to end under
//     a persona, so it is exactly the sentence a doc claim about the class
//     being "out of the crew's reach" has to be judged against.
//
// So: the refusal is exercised through the chain, and the residual is
// MEASURED rather than asserted — the same split commitwall_qa_test.go keeps.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaPrivateIndexChainRepo is a repo carrying the real wall behind INSTALL.md
// §9's dispatcher, with a witness standing in for bd's shim so "the refusal
// ended the slot" is observed and not inferred. Unlike qaChainRepo it runs
// real commits, because a private index is a property of `git commit`, not of
// the hook's argv.
func qaPrivateIndexChainRepo(t *testing.T) (repo, witness string, git func(env []string, args ...string) (string, error), persona []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	witness = filepath.Join(t.TempDir(), "bd.log")
	if err := os.Rename(filepath.Join(hooks, "prepare-commit-msg"), filepath.Join(hooks, "posse-prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	bd := "#!/bin/sh\nprintf 'reached[%s]\\n' \"$*\" >> " + witness + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(hooks, "bd-prepare-commit-msg"), []byte(bd), 0o755); err != nil {
		t.Fatal(err)
	}
	chain := "#!/bin/sh\nd=$(dirname \"$0\")\n" +
		"\"$d/posse-prepare-commit-msg\" \"$@\" || exit $?\n" +
		"[ -x \"$d/bd-prepare-commit-msg\" ] || exit 0\n" +
		"exec \"$d/bd-prepare-commit-msg\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte(chain), 0o755); err != nil {
		t.Fatal(err)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git = func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	// The fixture is only a fixture if the wall is really behind the chain and
	// the base commit really landed — an absence proves nothing about a rig
	// that was never built (ranger-base-z4vx).
	body, err := os.ReadFile(filepath.Join(hooks, "posse-prepare-commit-msg"))
	if err != nil || !strings.Contains(string(body), sharedIndexMarker) {
		t.Fatalf("fixture: no wall behind the chain: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "fix.go"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(nil, "add", "-A"); err != nil {
		t.Fatalf("fixture add: %v %s", err, out)
	}
	if out, err := git(nil, "commit", "-qm", "base"); err != nil {
		t.Fatalf("fixture base commit: %v %s", err, out)
	}
	return repo, witness, git, []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
}

// TestQAPrivateIndexRefusedThroughTheChain: rangerhq-cqq1's recipe, run in the
// shape the crew's repos are actually hooked in. The private index is refused
// under its own name, HEAD does not move, and bd's hook is never reached —
// then the blessed path-limited form still lands AND still reaches bd's hook,
// which is the control that makes the refusals mean something.
func TestQAPrivateIndexRefusedThroughTheChain(t *testing.T) {
	repo, witness, git, persona := qaPrivateIndexChainRepo(t)
	head, _ := git(nil, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	read := func() string { b, _ := os.ReadFile(witness); return string(b) }
	// The fixture's own base commit is the operator's, so it PASSED the gate
	// and already reached bd's hook — which is itself the first evidence the
	// chain is wired. Clear it, or every arm below inherits that line.
	if got := read(); !strings.Contains(got, "reached[") {
		t.Fatalf("fixture: the operator's base commit must have reached bd's hook, got %q", got)
	}
	if err := os.Truncate(witness, 0); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"next-index-mine", "next-index-1234", "next-index-99.lock", "index"} {
		dir := t.TempDir()
		idx := filepath.Join(dir, name)
		env := append(append([]string(nil), persona...), "GIT_INDEX_FILE="+idx)
		if err := os.WriteFile(filepath.Join(repo, "fix.go"), []byte("v2-THE-FIX\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := git(env, "read-tree", "HEAD"); err != nil {
			t.Fatalf("read-tree %s: %v %s", name, err, out)
		}
		if out, err := git(env, "add", "--", "fix.go"); err != nil {
			t.Fatalf("add %s: %v %s", name, err, out)
		}
		out, err := git(env, "commit", "-m", "the fix")
		if err == nil {
			t.Errorf("through the chain, <tmp>/%s must be refused, it landed: %s", name, out)
		}
		if name != "index" && !strings.Contains(out, "refused by posse gate: a commit from a private GIT_INDEX_FILE") {
			t.Errorf("<tmp>/%s must be refused AS a private index: %s", name, out)
		}
		if now, _ := git(nil, "rev-parse", "HEAD"); strings.TrimSpace(now) != head {
			t.Fatalf("<tmp>/%s moved HEAD through the chain: %s", name, now)
		}
		if got := read(); got != "" {
			t.Errorf("a refused commit must not reach bd's hook: %q", got)
		}
		if err := os.Truncate(witness, 0); err != nil {
			t.Fatal(err)
		}
	}

	// The control. If this does not land, every refusal above is satisfied by
	// a chain that refuses everything, which is not a wall.
	if err := os.WriteFile(filepath.Join(repo, "fix.go"), []byte("v2-THE-FIX\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(persona, "commit", "-m", "safe", "--", "fix.go"); err != nil {
		t.Fatalf("the blessed form must still land through the chain: %v %s", err, out)
	}
	if got := read(); !strings.Contains(got, "reached[") {
		t.Errorf("a commit the gate passes must reach bd's hook: %q", got)
	}
	if now, _ := git(nil, "rev-parse", "HEAD"); strings.TrimSpace(now) == head {
		t.Error("the blessed form left HEAD where it was")
	}
}

// TestQAPrivateIndexInsideTheGitDirIsTheMeasuredResidual pins the boundary the
// fix chose, by measuring what it costs rather than by repeating the sentence
// that describes it: `$GIT_DIR/next-index-<digits>` is exempt, and it still
// runs rangerhq-8rtf end to end under a persona — the commit lands, the SHARED
// index is left holding the pre-fix blob, and the next unqualified commit
// reverts the landed fix in silence.
//
// This test passing is not "the residual is fine". It is the measurement any
// claim about the class being out of the crew's reach has to be read against;
// if a later change closes the residual, this test goes red and the doc
// sentence it guards can finally be written without a caveat.
func TestQAPrivateIndexInsideTheGitDirIsTheMeasuredResidual(t *testing.T) {
	repo, _, git, persona := qaPrivateIndexChainRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "fix.go"), []byte("v2-THE-FIX\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := filepath.Join(repo, ".git", "next-index-1")
	env := append(append([]string(nil), persona...), "GIT_INDEX_FILE="+idx)
	if out, err := git(env, "read-tree", "HEAD"); err != nil {
		t.Fatalf("read-tree: %v %s", err, out)
	}
	if out, err := git(env, "add", "--", "fix.go"); err != nil {
		t.Fatalf("add: %v %s", err, out)
	}
	if out, err := git(env, "commit", "-m", "the fix"); err != nil {
		t.Fatalf("the residual: $GIT_DIR/next-index-1 is exempt by design and must land: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "HEAD:fix.go"); !strings.Contains(out, "v2-THE-FIX") {
		t.Fatalf("the residual commit did not carry the fix: %s", out)
	}

	// …and the shared index it left behind still holds the PRE-fix blob.
	staged, err := git(nil, "ls-files", "-s", "--", "fix.go")
	if err != nil {
		t.Fatalf("ls-files: %v %s", err, staged)
	}
	f := strings.Fields(staged)
	if len(f) < 2 {
		t.Fatalf("ls-files gave no blob for fix.go: %q", staged)
	}
	if blob, _ := git(nil, "cat-file", "-p", f[1]); !strings.Contains(blob, "v1") {
		t.Errorf("expected the SHARED index to be stale at v1, got %q", blob)
	}

	// …so the next unqualified commit — the operator's `bd sync`, which the
	// wall exempts by design — silently reverts the landed fix.
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("synced\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(nil, "add", "other.txt"); err != nil {
		t.Fatalf("add other.txt: %v %s", err, out)
	}
	if out, err := git(nil, "commit", "-qm", "bd sync: batch"); err != nil {
		t.Fatalf("operator commit: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "HEAD:fix.go"); !strings.Contains(out, "v1") {
		t.Errorf("rangerhq-8rtf no longer reproduces through the residual — the residual "+
			"is closed, and NOTES.md's account of it is now the thing that is stale: %s", out)
	}
}
