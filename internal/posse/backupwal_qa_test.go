//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin — the arm ranger-base-a0ln0's close named as NOT covered: "the
// concurrent-bd-writer arm of ADR 0036 verification observable 6 needs a
// live writer in the rig and was not built; the round trip is measured on a
// quiet database" (verify bead ranger-base-5dlx9).
//
// It matters because of how the store of record is actually written. bd
// commits in WAL mode, so a row that is committed and not yet checkpointed
// lives in `beads.db-wal` and NOT in `beads.db` — the newest beads on the
// instance are exactly the ones in that state. An implementation that
// copied the db file, or opened it `immutable=1`, would produce an archive
// that verifies, restores, and is silently missing this week's work
// (memory: sqlite immutable=1 is WAL-blind).
//
// The rig is shown able to fail before anything is measured: the control
// reads the source db FILE as the WAL-blind reading would, and the test
// only proceeds once that reading is genuinely one row short. Without that
// arm a checkpoint at the wrong moment would leave every assertion below
// green over a quiet database — which is the case that was already pinned.

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQABackupCarriesRowsALiveWriterLeftInTheWAL(t *testing.T) {
	a, queue := backupRig(t)
	db := filepath.Join(beadsHome(queue), "beads.db")

	// A writer that holds its connection open, so nothing checkpoints: the
	// sqlite3 CLI checkpoints when it closes, and closing is what this must
	// not do.
	w := exec.Command("sqlite3", db)
	in, err := w.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	outp, err := w.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Skipf("sqlite3 is not usable here: %v", err)
	}
	done := make(chan struct{})
	t.Cleanup(func() {
		in.Close()
		_ = w.Process.Kill()
		<-done
	})
	go func() { _ = w.Wait(); close(done) }()

	if _, err := io.WriteString(in, "PRAGMA journal_mode=WAL;\ninsert into issues values('wal-only');\nselect count(*) from issues;\n"); err != nil {
		t.Fatal(err)
	}
	// Read back through the writer's own connection: the answer is the
	// handshake that says the insert is committed and the WAL is on disk.
	br := bufio.NewReader(outp)
	var live string
	for i := 0; i < 3; i++ {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Skipf("the writer did not answer (%v); read %q", err, live)
		}
		live = strings.TrimSpace(line)
		if live == "2" {
			break
		}
	}
	if live != "2" {
		t.Skipf("the writer's own reading is %q, want 2 rows — no live writer, nothing to measure", live)
	}

	// THE CONTROL, and it is what makes the assertion below mean anything:
	// the WAL-blind reading of the same file must be SHORT. If it is not,
	// something checkpointed and this run would pass over a quiet db.
	blind := sqliteCount(t, "file:"+db+"?immutable=1")
	if blind != 1 {
		t.Skipf("the source db file already carries %d rows — nothing is parked in the WAL, so this rig cannot fail", blind)
	}

	res := mustBackup(t, a, backupAt)

	// And the archive must carry what the writer committed.
	got := filepath.Join(t.TempDir(), "beads.db")
	extractMember(t, res.Archive, "queue/beads.db", got)
	if n := sqliteCount(t, got); n != 2 {
		t.Errorf("the archived db holds %d rows, want 2 — a row committed to the WAL and never checkpointed did not reach the archive (ADR 0036 §7, observable 6)", n)
	}
	ids := sqliteQuery(t, got, "select group_concat(id) from issues")
	if !strings.Contains(ids, "wal-only") {
		t.Errorf("the archived db holds %q — the WAL-resident row is missing", ids)
	}
}

func sqliteCount(t *testing.T, source string) int {
	t.Helper()
	out := sqliteQuery(t, source, "select count(*) from issues")
	n := 0
	for _, c := range out {
		if c < '0' || c > '9' {
			t.Fatalf("count reading %q is not a number", out)
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func sqliteQuery(t *testing.T, source, sql string) string {
	t.Helper()
	out, err := exec.Command("sqlite3", source, sql).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3 %s: %s (%v)", sql, strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out))
}

// extractMember writes one member of an archive to dst.
func extractMember(t *testing.T, archive, name, dst string) {
	t.Helper()
	body := tarMemberBytes(t, archive, name)
	if body == nil {
		t.Fatalf("%s holds no member %s", archive, name)
	}
	if err := os.WriteFile(dst, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// tarMemberBytes is one member's body, or nil if the archive has no such
// member.
func tarMemberBytes(t *testing.T, archive, name string) []byte {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Name != name {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
}
