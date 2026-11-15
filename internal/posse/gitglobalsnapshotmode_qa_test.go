//go:build posse_arm3

package posse

// The GIT_CONFIG_GLOBAL snapshot's MODE is the second half of its wall
// (ranger-base-k2cbh, from the verify of ranger-base-161t2).
//
// The lock DIRECTORY at `<snapshot>.lock` refuses every config write that
// goes through git's O_CREAT|O_EXCL lockfile. MEASURED 2026-11-15, git 2.56.0
// linux, over `--global`, `--add`, `--unset`, `--unset-all`,
// `--remove-section`, `--rename-section`, `--replace-all` and
// `--file <snapshot>`: all rc 255, bytes unmoved.
//
// `git config --global --edit` is the exception. It takes no lock: it hands
// the path to $GIT_EDITOR and the editor writes in place, so the lock
// directory is nowhere in that path and the file's 0444 mode is the whole
// wall. MEASURED the same day: a snapshot WITH its lock directory but at mode
// 0644 is rewritten by `--edit` at rc 0 — user.email emptied, and
// `git var GIT_AUTHOR_IDENT` then answers "Author identity unknown".
//
// Nothing held that mode. The mutant `os.Chmod(tmp.Name(), 0o444)` ->
// `0o644` in gitGlobalSnapshot left all five pins in
// gitglobalsnapshot_qa_test.go, TestQAGitGlobalSnapshotIsUnderNoCagedWriteGrant
// and the whole `-run 'GitGlobal|InletPin|HookSkipGlobal'` family GREEN, while
// the control mutant (never make the lock directory) reds four of them. This
// file is that mutant's door.
//
// Not parallel: the fixture sets HOME, as the rest of this family does.

import (
	"bytes"
	"os"
	"testing"
)

// gitEditorThatWrites is a $GIT_EDITOR that truncates the file it is handed.
// git runs the editor through `sh -c "$editor \"$@\""`, so a shell snippet
// needs no planted executable — which also keeps this test out of ADR 0085's
// cold-exec class. $0 is the `--`; $1 is the path git passes.
const gitEditorThatWrites = `/bin/sh -c 'printf "[user]\n\temail = probe@invalid\n" > "$1"' --`

// TestQAGitGlobalSnapshotIsReadOnlyAgainstTheEditorRoute holds the mode, and
// holds what the mode is FOR: `git config --global --edit` under the rendered
// row must not change the snapshot every seat reads as its global scope. The
// control is the same line against the operator's own file, which is the row
// as it stood before ranger-base-161t2 — it succeeds there, so a green arm
// above measures something.
func TestQAGitGlobalSnapshotIsReadOnlyAgainstTheEditorRoute(t *testing.T) {
	_, homeCfg, _ := globalScopeFixture(t, true, false)
	if err := PrepareGitGlobalSnapshot(); err != nil { // as a launch does
		t.Fatal(err)
	}
	row, ok := pinRow("GIT_CONFIG_GLOBAL")
	if !ok || row == homeCfg {
		t.Fatalf("CONTROL FAILED: the row is %q (present %v), not a snapshot of %s", row, ok, homeCfg)
	}
	want, err := os.ReadFile(row)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(row)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o444 {
		t.Errorf("the snapshot %s is mode %#o, want 0444 — the lock directory does not stand in `git config --global --edit`'s way, so the mode is the only thing refusing it (ranger-base-k2cbh)", row, perm)
	}

	out, rc := seatGit(t, row, []string{"GIT_EDITOR=" + gitEditorThatWrites},
		"config", "--global", "--edit")
	if got, _ := os.ReadFile(row); !bytes.Equal(got, want) {
		t.Errorf("`git config --global --edit` rewrote the snapshot under the rendered row (rc %d):\n before %q\n after  %q\n%s", rc, want, got, out)
	}
	if rc == 0 {
		t.Errorf("`git config --global --edit` under the rendered row exited 0 — the editor route must refuse, not land somewhere quietly:\n%s", out)
	}

	// CONTROL: the row as it stood names the operator's own file, and the same
	// editor line rewrites it. Without this, the arm above could be green
	// because the editor never ran.
	opBefore, err := os.ReadFile(homeCfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, rc := seatGit(t, homeCfg, []string{"GIT_EDITOR=" + gitEditorThatWrites},
		"config", "--global", "--edit"); rc != 0 {
		t.Fatalf("CONTROL FAILED: `git config --global --edit` under GIT_CONFIG_GLOBAL=%s exited %d, so the editor never wrote and the arm above measured nothing", homeCfg, rc)
	}
	if opAfter, _ := os.ReadFile(homeCfg); bytes.Equal(opAfter, opBefore) {
		t.Fatal("CONTROL FAILED: the editor line did not change the operator's own file, so the arm above measured nothing")
	}
}
