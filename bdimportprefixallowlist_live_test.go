package posse

// Live pin for ranger-base-6yf, run against the real bd rather than a fixture:
//
//	`allowed_prefixes` does not let a strict import CREATE a foreign-prefix
//	issue. It clears the importer's batch gate and is then overruled by the
//	storage layer's own single-prefix check. What actually admits the row is
//	bd's LENIENT auto-import, which skips prefix validation altogether — and
//	it admits a disallowed prefix just as happily, in the same command that
//	then reports the refusal at exit 1.
//
//	CORRECTED 2026-08-28 (ranger-base-gebs). This pin previously asserted the
//	opposite of its third arm — that the refusal was a rollback and the NEXT
//	ordinary bd command was the writer. It was green because inDB read the
//	database with `?immutable=1`, which does not read the -wal. The refusing
//	command commits the row to the WAL; the next command merely CHECKPOINTS
//	it into the main file, at which point a WAL-blind reader can finally see
//	it. Measured: after the refusal, wal_bytes=98912 and a WAL-aware read
//	says 1 while the immutable read still says 0. There is no silent second
//	writer. Exit 1 was never a rollback.
//
//	RHQ_LIVE_BD=1 go test . -run TestLiveBdImportPrefixAllowList -v
//
// This corrects the finding that shipped with ranger-base-nj9 and was written
// into the (private) cut-over runbook: "the append path is not a way around
// validation. It honors allowed_prefixes." Both halves are measured false
// here. The append path is exactly a way around validation, and the
// allowed_prefixes step is not what made the ingest work.
//
// bd 0.49.1, two checks, only one of which consults the allow-list:
//   - internal/importer.handlePrefixMismatch — reads allowed_prefixes from the
//     database config, allow-list aware. Passing it is not admission.
//   - internal/storage/sqlite.ValidateIssueIDPrefix, reached from
//     CreateIssueImport with the importer's SkipPrefixValidation — compares
//     against issue_prefix ALONE. It is the one that decides a create.
//
// Every lenient caller (cmd/bd/autoflush.go autoImportIfNewer, autoimport.go,
// daemon_sync.go importToJSONLWithStore) hard-codes SkipPrefixValidation:true,
// so both checks are off on that path. `bd import` and `bd sync --import-only`
// leave it false and share one strict path — which is why this pin can stand
// on `bd sync --import-only` without running `bd import`, which is denied.
//
// Env-gated and skipped by default, like the other live pins: it shells out to
// the operator's bd, which has a version and a daemon, neither of which belongs
// in a hermetic suite. Everything happens inside one t.TempDir — the `bd init`
// and `bd config set` here are the throwaway-database case, never a repo or a
// config anybody keeps.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bdPrefixRig builds a throwaway bd repo whose issue_prefix is "mainx" and
// whose allowed_prefixes names a second, foreign prefix "otherx". Returns the
// root, a bd runner rooted there, and a row appender.
func bdPrefixRig(t *testing.T) (string, func(...string) (string, error), func(string)) {
	t.Helper()
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}

	root := t.TempDir()
	bd := func(args ...string) (string, error) {
		cmd := exec.Command("bd", append([]string{"--no-daemon"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "BEADS_AUTO_START_DAEMON=false")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	if out, err := bd("init", "--prefix", "mainx"); err != nil {
		// Naming the reason matters. Under a persona PID that denies `bd init`
		// and `bd config set` — laurie's does, and laurie is who runs the QA
		// pins — this skip is not an environment gap, it is the whole arm
		// silently measuring nothing. The two arms that need no allow-list use
		// bdPrefixRigNoInit below and run under any PID.
		if strings.Contains(out, "refused by posse gate") {
			t.Skipf("this PID cannot build the allow-list rig (bd init is denied); "+
				"the disallowed-prefix arms still ran: %s", strings.TrimSpace(out))
		}
		t.Skipf("bd init did not take in a throwaway repo: %v %s", err, out)
	}
	// Past this line the rig is bd's problem, not the environment's: a failure
	// here is a broken pin, and a pin that reports itself as SKIP is worse
	// than one that reports itself as FAIL.
	if out, err := bd("config", "set", "allowed_prefixes", "mainx,otherx"); err != nil {
		t.Fatalf("bd config set allowed_prefixes did not take: %v %s", err, out)
	}
	// `bd init` alone leaves no issues.jsonl. Mint one native row so the file
	// exists and the appends below land in a JSONL bd already agrees with.
	if out, err := bd("create", "rig seed", "-t", "task"); err != nil {
		t.Fatalf("bd create (rig seed): %v %s", err, out)
	}
	if out, err := bd("sync", "--flush-only"); err != nil {
		t.Fatalf("bd sync --flush-only: %v %s", err, out)
	}

	jsonl := filepath.Join(root, ".beads", "issues.jsonl")
	if _, err := os.Stat(jsonl); err != nil {
		t.Fatalf("no JSONL to append to, so this pin would measure nothing: %v", err)
	}
	// Settle the database against the JSONL so the append below is the only
	// change either check can be reacting to.
	if out, err := bd("sync", "--import-only"); err != nil {
		t.Fatalf("could not settle the rig: %v %s", err, out)
	}

	appendRow := func(id string) {
		t.Helper()
		row := `{"id":"` + id + `","title":"6yf probe ` + id + `","status":"open",` +
			`"priority":2,"issue_type":"task","created_at":"2026-08-28T03:00:00.000000-04:00",` +
			`"updated_at":"2026-08-28T03:00:00.000000-04:00"}` + "\n"
		f, err := os.OpenFile(jsonl, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			t.Fatalf("open %s: %v", jsonl, err)
		}
		if _, err := f.WriteString(row); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close %s: %v", jsonl, err)
		}
	}
	return root, bd, appendRow
}

// bdPrefixRigNoInit builds the same throwaway repo without `bd init` and
// without `bd config set` — both of which a QA persona's PID denies, which is
// how every arm of this pin came to report SKIP in the hands of the persona
// most likely to run it (ranger-base-zaj7).
//
// bd needs no init verb: a directory holding only .beads/issues.jsonl is
// materialised into a full database by the first ordinary command, and
// issue_prefix is inferred from the seed rows. What this rig cannot do is set
// allowed_prefixes, so it serves the disallowed-prefix arms only — which are
// exactly the arms that pin the WAL mechanism.
func bdPrefixRigNoInit(t *testing.T) (string, func(...string) (string, error), func(string)) {
	t.Helper()
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}

	root := t.TempDir()
	bd := func(args ...string) (string, error) {
		cmd := exec.Command("bd", append([]string{"--no-daemon"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "BEADS_AUTO_START_DAEMON=false")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init in a throwaway dir: %v %s", err, out)
	}

	jsonl := filepath.Join(root, ".beads", "issues.jsonl")
	if err := os.MkdirAll(filepath.Dir(jsonl), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	seed := `{"id":"mainx-1","title":"rig seed","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-08-28T03:00:00.000000-04:00",` +
		`"updated_at":"2026-08-28T03:00:00.000000-04:00"}` + "\n"
	if err := os.WriteFile(jsonl, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed %s: %v", jsonl, err)
	}
	// The first ordinary command materialises the database from the JSONL.
	if out, err := bd("list", "--limit", "1"); err != nil {
		t.Skipf("bd could not materialise a database from a JSONL alone: %v %s", err, out)
	}
	// Settle, so the append below is the only change either check reacts to.
	if out, err := bd("sync", "--import-only"); err != nil {
		t.Fatalf("could not settle the rig: %v %s", err, out)
	}
	// Past this line a failure is a broken pin, not an environment: if the
	// seed row is not in the database, the rig imported nothing and every
	// assertion below would be satisfied by an empty database.
	if !inDB(t, root, "mainx-1") {
		t.Fatalf("the rig's own seed row never reached the database, so this pin would measure nothing")
	}

	appendRow := func(id string) {
		t.Helper()
		row := `{"id":"` + id + `","title":"zaj7 probe ` + id + `","status":"open",` +
			`"priority":2,"issue_type":"task","created_at":"2026-08-28T03:00:00.000000-04:00",` +
			`"updated_at":"2026-08-28T03:00:00.000000-04:00"}` + "\n"
		f, err := os.OpenFile(jsonl, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatalf("open %s: %v", jsonl, err)
		}
		if _, err := f.WriteString(row); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close %s: %v", jsonl, err)
		}
	}
	return root, bd, appendRow
}

// idBytesOnDisk counts the id's raw bytes in beads.db and in beads.db-wal.
// It is the third reader, and the only one that shares no blind spot with a
// SQL reader: it answers WHERE the row is, not merely whether some connection
// can see it. That is what makes "the later command is a checkpointer, not a
// writer" falsifiable — a command cannot have written bytes that were already
// on disk before it ran.
func idBytesOnDisk(t *testing.T, root, id string) (mainDB, wal int) {
	t.Helper()
	count := func(path string) int {
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return 0
			}
			t.Fatalf("read %s: %v", path, err)
		}
		return bytes.Count(b, []byte(id))
	}
	db := filepath.Join(root, ".beads", "beads.db")
	return count(db), count(db + "-wal")
}

// inDB answers from the JSONL-independent side: the row is in the database or
// it is not. `bd show` cannot be the instrument here — running it is one of the
// things under test.
//
// It reads a `cp -a` SNAPSHOT of .beads rather than the live file, and opens it
// without `?immutable=1`. Both halves matter. immutable does not read the -wal,
// so it reports a committed row as absent until some later command checkpoints
// — that single flag is what made the earlier version of this pin assert, and
// pass, the opposite mechanism. Opening the live database read-write instead
// would checkpoint it, and the checkpoint is one of the things under test; the
// snapshot carries beads.db-wal and recovers it out of harm's way.
func inDB(t *testing.T, root, id string) bool {
	t.Helper()
	snap := filepath.Join(t.TempDir(), "beads")
	if out, err := exec.Command("cp", "-a", filepath.Join(root, ".beads"), snap).CombinedOutput(); err != nil {
		t.Fatalf("snapshot .beads: %v %s", err, out)
	}
	out, err := exec.Command("sqlite3", filepath.Join(snap, "beads.db"),
		"SELECT count(*) FROM issues WHERE id='"+id+"';").Output()
	if err != nil {
		t.Fatalf("sqlite3 read: %v", err)
	}
	return strings.TrimSpace(string(out)) == "1"
}

func TestLiveBdImportPrefixAllowListDoesNotReachTheCreate(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("no sqlite3 on PATH")
	}

	// The claim under test. An ALLOWED foreign prefix, offered to the strict
	// importer alone, is refused at the create — allowed_prefixes never
	// reaches that check.
	t.Run("strict import refuses an allowed foreign prefix", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRig(t)
		appendRow("otherx-6yf1")

		out, err := bd("--no-auto-import", "sync", "--import-only")
		if err == nil {
			t.Fatalf("the strict import took an allowed foreign prefix — the runbook's reading of allowed_prefixes would be right after all:\n%s", out)
		}
		if !strings.Contains(out, "does not match configured prefix") {
			t.Errorf("refused, but not by the single-prefix check this pin is about:\n%s", out)
		}
		if inDB(t, root, "otherx-6yf1") {
			t.Errorf("refused at exit %v and the row is in the database anyway:\n%s", err, out)
		}
	})

	// The witness for the arm above: the same fixture, the same row, the one
	// difference being the lenient auto-import. Without this the arm above is
	// satisfied by a rig that imports nothing at all.
	t.Run("auto-import takes the same row at exit 0", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRig(t)
		appendRow("otherx-6yf1")

		out, err := bd("sync", "--import-only")
		if err != nil {
			t.Fatalf("the runbook's own F3 command failed on an allowed prefix: %v\n%s", err, out)
		}
		if !inDB(t, root, "otherx-6yf1") {
			t.Fatalf("exit 0 and the row is not in the database:\n%s", out)
		}
		// Trap 1, with its mechanism: the lenient pass created the row, so the
		// strict pass that prints the summary saw nothing left to create.
		if !strings.Contains(out, "0 created") {
			t.Errorf("expected the summary to report 0 created for a row this command just created:\n%s", out)
		}
	})

	// Trap 2. The refusal is NOT a rollback: the command that reports the
	// prefix mismatch at exit 1 has already committed the row. Measured
	// deterministic 3/3 with a WAL-aware read; the earlier version of this
	// pin asserted the reverse and passed only because its reader could not
	// see the WAL (see the CORRECTED note in the header).
	t.Run("the refusing command has already written the row", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRigNoInit(t)
		appendRow("zzspike-6yf2")

		// Negative control on the reader itself. Without this, an inDB that
		// always answered "yes" would satisfy every assertion below.
		if inDB(t, root, "zzspike-6yf2") {
			t.Fatalf("the row is in the database before any import ran — inDB is not discriminating")
		}

		out, err := bd("sync", "--import-only")
		if err == nil {
			t.Fatalf("a disallowed prefix was accepted by the F3 command:\n%s", out)
		}
		if !strings.Contains(out, "prefix mismatch") {
			t.Errorf("refused, but not by the batch gate this arm is about:\n%s", out)
		}
		if !inDB(t, root, "zzspike-6yf2") {
			t.Fatalf("exit 1 really was a rollback here; the trap this pin is about is gone or has moved:\n%s", out)
		}

		// And it stays wedged: the row is in the database, so every later
		// import fails identically. This is the half the runbook's recovery
		// step turns on — the JSONL line is not where the row lives now.
		again, againErr := bd("sync", "--import-only")
		if againErr == nil {
			t.Errorf("the second import recovered on its own, so the wedge is not permanent:\n%s", again)
		}
		if !inDB(t, root, "zzspike-6yf2") {
			t.Errorf("the row left the database between the two imports:\n%s", again)
		}
	})

	// The instrument, pinned as a finding in its own right. `?immutable=1` is
	// the natural way to read a database you must not disturb, and on bd it is
	// wrong: it reports a committed row as absent until some later command
	// checkpoints the WAL. Anyone auditing an import this way — the runbook's
	// reader included — gets a clean answer that is not true.
	t.Run("immutable=1 cannot see the row the refusal committed", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRigNoInit(t)
		appendRow("zzspike-6yf3")

		out, err := bd("sync", "--import-only")
		if err == nil {
			t.Fatalf("a disallowed prefix was accepted by the F3 command:\n%s", out)
		}

		// The reader-independent half (ranger-base-zaj7). Where the bytes are
		// decides which command wrote them: after the refusal they are in the
		// -wal and not in the main file, and the NEXT ordinary command moves
		// them without adding anything. A checkpointer, not a writer — and no
		// SQL reader is consulted to establish it.
		// Negative control on the byte reader: an id nobody ever appended must
		// be in neither file. Without it, a reader that counted anything at
		// all would satisfy every assertion below.
		if m, w := idBytesOnDisk(t, root, "zzspike-never-appended"); m != 0 || w != 0 {
			t.Fatalf("the byte reader found an id that was never written (db=%d wal=%d) — it is not discriminating", m, w)
		}
		mainDB, wal := idBytesOnDisk(t, root, "zzspike-6yf3")
		if wal == 0 {
			t.Fatalf("the refusal left nothing in beads.db-wal, so the WAL is not where the row went and this arm measures nothing:\n%s", out)
		}
		if mainDB != 0 {
			t.Errorf("the refusing command checkpointed as well as committed (%d hits in beads.db) — "+
				"the two readers no longer disagree and the instrument note is stale", mainDB)
		}

		immutable, immErr := exec.Command("sqlite3",
			"file:"+filepath.Join(root, ".beads", "beads.db")+"?immutable=1",
			"SELECT count(*) FROM issues WHERE id='zzspike-6yf3';").Output()
		if immErr != nil {
			t.Fatalf("sqlite3 immutable read: %v", immErr)
		}
		if !inDB(t, root, "zzspike-6yf3") {
			t.Fatalf("the WAL-aware read cannot see it either, so this arm measured nothing:\n%s", out)
		}
		// Deliberately red rather than skipped if this expires. The runbook
		// tells the operator not to audit an import with immutable=1; if the
		// two readers ever agree, that instruction is stale and somebody has
		// to be told, and a SKIP tells nobody.
		if strings.TrimSpace(string(immutable)) != "0" {
			t.Errorf("immutable=1 saw the row (%s) — bd now checkpoints inside the refusing "+
				"command, the two readers no longer disagree, and $RB/docs/runbooks/0012-cutover.md "+
				"§1 F8 needs its instrument note updated", strings.TrimSpace(string(immutable)))
		}

		// And the later command is the checkpointer. It must move the bytes
		// into beads.db without the WAL ever holding a second copy: if the
		// row had been WRITTEN here rather than checkpointed, the count in
		// the -wal would rise, not fall to nothing.
		later, laterErr := bd("list", "--limit", "1")
		if laterErr != nil {
			t.Fatalf("an ordinary read failed after the refusal, so the checkpoint half is unmeasured: %v\n%s", laterErr, later)
		}
		mainAfter, walAfter := idBytesOnDisk(t, root, "zzspike-6yf3")
		if mainAfter == 0 {
			t.Errorf("the later command did not checkpoint the row into beads.db (main=%d wal=%d)", mainAfter, walAfter)
		}
		if walAfter > wal {
			t.Errorf("the later command ADDED to the WAL (%d -> %d) rather than draining it — "+
				"it may be writing the row after all, which is the claim this pin retired", wal, walAfter)
		}
	})
}
