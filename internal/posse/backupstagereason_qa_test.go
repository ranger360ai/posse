//go:build posse_arm3

package posse

// QA pin — ranger-base-w0w7u, the third half of ranger-base-va8v5's one live
// finding: `posse backup` staged the beads db with
// protocolCmd(exec.Command("sqlite3", …)) and rendered the failure as
// Die("sqlite3 .backup: %s", outp) with err DISCARDED.
//
// WHY THAT IS A DEFECT AND NOT A STYLE NOTE. The cases where this child
// fails are exactly the cases where it wrote nothing: a child that never
// started has written to neither stream, and protocolCmd's own refusal
// lives on cmd.Err (ADR 0077 D3), which CombinedOutput returns beside an
// EMPTY slice. So the one reading that said why was thrown away and the
// operator was handed the sentence `sqlite3 .backup: ` — on a half-built
// staging dir, since this runs after os.MkdirAll and after the queue bundle
// is staged. worktree.go:289-298 has carried the fallback for every other
// child of this tree since it was written; this site did not.
//
// THE FIXTURE IS A PLAIN NON-GATES FAKE, deliberately, and not the gates-dir
// shim the finding was measured with. That shim reaches this line only while
// the ADR 0036 §2 tool gate upstream still probes with exec.LookPath — the
// OTHER half of the same finding, whose fix (protocolAvailable, landing with
// the census arm of ranger-base-372iq) refuses at the gate and never reaches
// stageBackup at all. A pin keyed on that route would go green by becoming
// unreachable the day the gate is fixed, which is the one way a pin can lie
// while nothing about its own subject changed. An exec that fails silently is
// the whole class: a shim, a sqlite3 killed by a signal, an ENOENT race
// between the gate and the stage.
//
// MUTATION-CHECKED: restoring `Die("sqlite3 .backup: %s",
// strings.TrimSpace(string(outp)))` reds arm (a) alone — the message is then
// the bare prefix with nothing after it — and leaves (b) green, which is why
// (b) is here: the fallback must not REPLACE a reason the child did give.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// backupFakeSqlite puts a sqlite3 of its own in front of the real one for
// the rest of the test, and returns nothing: the point is the PATH. Serial,
// like every test here that sets PATH.
func backupFakeSqlite(t *testing.T, body string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(filepath.Join(bin, "sqlite3"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	// In FRONT of the box's own PATH and not instead of it: git is the other
	// tool this verb stages with, and the bundle runs before sqlite3 does.
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestQABackupStageNamesTheReasonWhenSqliteSaysNothing(t *testing.T) {
	// The rig first, while the box's own sqlite3 is still the one in front:
	// it makes the beads db this backup is of.
	a, _ := backupRig(t)

	// (a) THE DEFECT. A sqlite3 that fails and says nothing at all, which is
	// what a child that never started looks like from CombinedOutput.
	backupFakeSqlite(t, "exit 3\n")
	_, err := a.RunBackup(BackupOpts{Out: io.Discard, Now: func() time.Time { return backupAt }})
	if err == nil {
		t.Fatal("backup published an archive with a sqlite3 that exits 3 — the fixture is not the failing stage this arm is about")
	}
	msg := err.Error()
	if !strings.Contains(msg, "sqlite3 .backup:") {
		t.Fatalf("the refusal is not the staging one: %q", msg)
	}
	if strings.TrimSpace(strings.TrimPrefix(msg, "sqlite3 .backup:")) == "" {
		t.Errorf("refusal names no reason at all, just the prefix: %q — err was discarded (ranger-base-w0w7u)", msg)
	}
	if !strings.Contains(msg, "exit status 3") {
		t.Errorf("refusal = %q, want the exec's own reading (exit status 3) where the child printed nothing", msg)
	}
}

func TestQABackupStagePrefersWhatSqliteActuallySaid(t *testing.T) {
	// (b) The other direction, so the fix is not "always print err": a child
	// that DID say why keeps its sentence, and the exec's own reading — which
	// is never more than "exit status N" — stays out of the way.
	a, _ := backupRig(t)
	const said = "Error: unable to open database file"
	backupFakeSqlite(t, "echo '"+said+"' >&2\nexit 1\n")
	_, err := a.RunBackup(BackupOpts{Out: io.Discard, Now: func() time.Time { return backupAt }})
	if err == nil {
		t.Fatal("backup published an archive with a sqlite3 that exits 1 — the fixture is not the failing stage this arm is about")
	}
	msg := err.Error()
	if !strings.Contains(msg, said) {
		t.Errorf("refusal = %q, want what the child said: %q", msg, said)
	}
	if strings.Contains(msg, "exit status") {
		t.Errorf("refusal = %q, want the child's own sentence and not the exec's reading beside it", msg)
	}
}
