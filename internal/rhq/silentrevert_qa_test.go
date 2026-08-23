package rhq

// QA pins for the silent-revert detector (scripts/audit-silent-reverts.sh,
// rangerhq-8rtf, verified under rangerhq-jkhb).
//
// Two claims, one live and one pinned open:
//
//   1. The detector's own --self-test proves the detector FIRES, which is the
//      only thing that separates "the audit ran" from "the audit works". But
//      `make test` runs the script with --quiet only, so --self-test had no
//      trigger — it was a line in the Makefile comment, i.e. a thing to
//      remember, which is the objection rangerhq-2f5r raised in the first
//      place. It runs here now.
//
//   2. The detector covers the ADD-ONLY half of its own mechanism
//      (rangerhq-ypn1, fixed): when the change landed from a private index
//      consisted of NEW files, the shared index has no entry for them, so the
//      next commit from that index DELETES them rather than rolling content
//      back. The scan used to skip deletions, so that half scored clean and
//      exited 0. It now treats absence as a state a path can be rolled back
//      to, and both halves flag. This test is what says so.
//
// Self-contained on purpose (own helpers, own fixture): they must survive
// whatever the next persona does to the script's neighbours.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// srScript resolves the audit script, skipping if this checkout does not carry
// it (a tarball, a worktree pruned for a build).
func srScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../scripts/audit-silent-reverts.sh")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("audit script not present: %v", err)
	}
	return p
}

// srEnv is the caller's environment with the two variables that would make
// these tests depend on WHO runs them removed: RHQ_PERSONA arms the commit
// wall, and an inherited GIT_INDEX_FILE would point the fixture's git at
// somebody else's index.
func srEnv() []string {
	var e []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RHQ_PERSONA=") || strings.HasPrefix(kv, "GIT_INDEX_FILE=") {
			continue
		}
		e = append(e, kv)
	}
	return append(e,
		"GIT_AUTHOR_NAME=qa", "GIT_AUTHOR_EMAIL=qa@t",
		"GIT_COMMITTER_NAME=qa", "GIT_COMMITTER_EMAIL=qa@t")
}

// srGit runs one git command in dir and fails the test if it does not.
// extra is appended to the environment (this is how the private-index commit
// is spelled).
func srGit(t *testing.T, dir string, extra []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(srEnv(), extra...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// srAudit runs the audit script with cwd=dir and returns its output and exit
// code. The script cds to the toplevel of whatever repo dir sits in, so dir
// selects the repo under audit.
func srAudit(t *testing.T, script, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(script, args...)
	cmd.Dir = dir
	cmd.Env = srEnv()
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s: %v\n%s", script, err, out)
	}
	return string(out), code
}

// TestSilentRevertSelfTestStillFires gives --self-test a trigger. A clean
// `scripts/audit-silent-reverts.sh --quiet` in `make test` proves only that the
// script ran; this proves the detector still flags the rangerhq-8rtf mechanism
// when it is planted in front of it.
func TestSilentRevertSelfTestStillFires(t *testing.T) {
	script := srScript(t)
	if err := exec.Command("git", "rev-parse", "--show-toplevel").Run(); err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	out, code := srAudit(t, script, ".", "--self-test")
	if code != 0 || !strings.Contains(out, "self-test PASS") {
		t.Fatalf("detector self-test did not fire: exit %d\n%s", code, out)
	}
}

// srPlantAddOnlyRevert builds rangerhq-8rtf's mechanism in a throwaway repo,
// with one difference from the incident: the change that lands is a NEW file
// rather than an edit to an existing one. Returns the repo path.
func srPlantAddOnlyRevert(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	srGit(t, repo, nil, "init", "-q", ".")
	write("existing.go", "v1\n")
	write("other.txt", "o\n")
	srGit(t, repo, nil, "add", "-A")
	srGit(t, repo, nil, "commit", "-qm", "base")

	// The fix: one new file, landed from a PRIVATE index. HEAD gets it; the
	// shared .git/index never hears about it.
	write("newpin_test.go", "package x // the regression pin\n")
	priv := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	srGit(t, repo, priv, "read-tree", "HEAD")
	srGit(t, repo, priv, "add", "--", "newpin_test.go")
	srGit(t, repo, priv, "commit", "-qm", "the fix: add newpin_test.go")

	// The next commit taken from the shared index — a bd sync. It writes a
	// tree with no newpin_test.go in it, so the fix is deleted, silently.
	write("other.txt", "synced\n")
	srGit(t, repo, nil, "add", "other.txt")
	srGit(t, repo, nil, "commit", "-qm", "bd sync: batch")

	tree := exec.Command("git", "ls-tree", "--name-only", "HEAD")
	tree.Dir = repo
	tree.Env = srEnv()
	out, err := tree.CombinedOutput()
	if err != nil {
		t.Fatalf("ls-tree: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "newpin_test.go") {
		t.Fatalf("rig did not reproduce the revert; HEAD still has the pin:\n%s", out)
	}
	return repo
}

// TestAuditFlagsAddOnlySilentRevert is rangerhq-ypn1. The commit under test
// undid a landed change from the stale shared index, which is precisely the
// class the audit exists for — but because the landed change was an ADD, the
// undo is a deletion, and scan() used to skip deletions on the rationale that
// "a removal is visible in review". rangerhq-8rtf is the disproof of that
// rationale: nobody reviewed dcca7b5. Unskipped when ypn1 closed; it fails
// again the moment the deletion rule is weakened.
func TestAuditFlagsAddOnlySilentRevert(t *testing.T) {
	script := srScript(t)
	repo := srPlantAddOnlyRevert(t)
	out, code := srAudit(t, script, repo, "HEAD")
	if code != 1 {
		t.Fatalf("add-only silent revert not flagged: exit %d, want 1\n%s", code, out)
	}
}

// TestAuditFlagsAddOnlySilentRevertIsStillTheMechanism guards the FIXTURE, not
// the defect, so the pin above cannot rot into a claim about a rig that stopped
// reproducing: if the plant ever stops building the three commits it means to
// build, the pin above would pass for the wrong reason and this one fails.
func TestAuditFlagsAddOnlySilentRevertIsStillTheMechanism(t *testing.T) {
	script := srScript(t)
	repo := srPlantAddOnlyRevert(t)
	out, _ := srAudit(t, script, repo, "HEAD")
	if !strings.Contains(out, "scanned 3 commits") {
		t.Fatalf("fixture no longer scans the 3 planted commits:\n%s", out)
	}
}
