package posse

// Live pin for ranger-base-6yf, run against the real bd rather than a fixture:
//
//	`allowed_prefixes` does not let a strict import CREATE a foreign-prefix
//	issue. It clears the importer's batch gate and is then overruled by the
//	storage layer's own single-prefix check.
//
//	RHQ_LIVE_BD=1 go test . -run TestLiveBdImportPrefixAllowList -v
//
// That one claim is all that survives from bd 0.49.1. The two mechanisms this
// pin was built around are both retired on the pinned 0.50.3, and the arms
// below now pin their ABSENCE — see the two notes next.
//
// RETIRED 2026-09-04 (ranger-base-v0v6l), measured on bd 0.50.3 (bd25acbc):
// **the lenient auto-import is gone.** On 0.49.1 every lenient caller
// (cmd/bd/autoflush.go autoImportIfNewer, autoimport.go, daemon_sync.go
// importToJSONLWithStore) hard-coded SkipPrefixValidation:true, so an appended
// row was admitted by the auto-import pass that ran ahead of the strict one —
// in the same command that then reported the refusal at exit 1. On 0.50.3 an
// ordinary command does not import an appended row at all: it REFUSES with
// `Database out of sync with JSONL. Run 'bd sync --import-only' to fix.`, and
// `--allow-stale` skips the check without importing anything. Measured with an
// in-prefix row, which nothing could have rejected: it does not reach the
// database. So the pin's second and third arms had no writer left, which is
// the whole reason they were red.
//
// RETIRED with it: **exit 1 IS a rollback now.** The refusing command was
// never the writer — the lenient pass was — so with that pass gone the refusal
// leaves nothing behind. Measured after the refusal: zero bytes of the id in
// beads.db AND zero in beads.db-wal, and the two SQL readers agree at 0.
// Consequently this pin can no longer demonstrate that `?immutable=1` is
// WAL-blind: on 0.50.3 no bd command observed here leaves a committed row
// resident in the WAL (a successful `sync --import-only` and a `bd create`
// both land in beads.db with no -wal residue at all). The sqlite fact is
// unchanged; the bd state that exhibited it is gone. `$RB/docs/runbooks/
// 0012-cutover.md` §1 F2/F8 still tell an operator that the failure is not a
// rollback and that recovery means getting the row out of the DATABASE — on
// 0.50.3 the row never reaches the database and recovery is the JSONL line
// alone (pinned in the third arm below). Handed off as ranger-base-my66u.
//
//	CORRECTED 2026-08-28 (ranger-base-gebs), kept for the record because it is
//	how the 0.49.1 mechanism was established. This pin previously asserted the
//	opposite of its third arm — that the refusal was a rollback and the NEXT
//	ordinary bd command was the writer. It was green because inDB read the
//	database with `?immutable=1`, which does not read the -wal. On 0.49.1 the
//	refusing command committed the row to the WAL and the next command merely
//	CHECKPOINTED it. Measured then: wal_bytes=98912, WAL-aware read 1 against
//	immutable read 0. That the same shape now measures 0/0 everywhere is a
//	change in bd, not a return to the pre-correction reading.
//
// This corrects the finding that shipped with ranger-base-nj9 and was written
// into the (private) cut-over runbook: "the append path is not a way around
// validation. It honors allowed_prefixes." Both halves are measured false
// here. The append path WAS exactly a way around validation on 0.49.1, and the
// allowed_prefixes step is not what made the ingest work.
//
// The two checks, both still present on 0.50.3, only one of which consults the
// allow-list:
//   - internal/importer.handlePrefixMismatch — reads allowed_prefixes from the
//     database config, allow-list aware. Passing it is not admission. It is
//     the one that says `prefix mismatch detected`.
//   - internal/storage/sqlite.ValidateIssueIDPrefix, reached from
//     CreateIssueImport — compares against issue_prefix ALONE. It is the one
//     that decides a create, and it says `does not match configured prefix`.
//
// The first arm below separates them by their messages, with both controls: a
// rig WITHOUT the allow-list stops the same row at the batch gate, and an
// in-prefix row on the allow-list rig imports at exit 0.
//
// Env-gated and skipped by default, like the other live pins: it shells out to
// the operator's bd, which has a version, neither of which belongs in a
// hermetic suite. Everything happens inside one t.TempDir.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bdPrefixRig builds a throwaway SQLite bd store whose issue_prefix is "mainx"
// and, when allowedPrefixes is non-empty, whose allowed_prefixes config names
// whatever it says. Returns the root, a bd runner rooted there, and a row
// appender.
//
// It builds the store WITHOUT `bd init` and WITHOUT `bd config set`, both of
// which every persona's gate shim denies — which is how all four arms of this
// pin once came to report SKIP in the hands of the persona most likely to run
// it (ranger-base-zaj7). It does not do that by leaving the store to bd's
// inference, which is what broke here (ranger-base-v0v6l): on bd 0.49.1 a
// directory holding only `.beads/issues.jsonl` was materialised into a full
// database with issue_prefix INFERRED from the seed rows; on 0.50.3 the same
// directory materialises a SQLite store with NO issue_prefix in its config
// table, no rows imported, and every `sync --import-only` against it refused
// with `database not initialized: issue_prefix config is missing`.
//
// So the config rows are written by name, with sqlite3 — the same third-party
// reader this pin already needs — into the config table bd materialised. What
// keeps that honest is that bd is asked to agree before any arm runs: the
// settle below is a real `bd sync --import-only`, which fails loudly on a
// prefix bd cannot read, and the seed row must then be in the database. The
// store CLASS is checked too, ahead of both: these arms are about a SQLite
// store's WAL and config table, so a bd that builds a different class must
// fail here by name rather than surface as an empty arm somewhere below.
func bdPrefixRig(t *testing.T, allowedPrefixes string) (string, func(...string) (string, error), func(string)) {
	t.Helper()
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("no sqlite3 on PATH")
	}

	root := t.TempDir()
	bd := func(args ...string) (string, error) {
		cmd := exec.Command("bd", append([]string{"--no-daemon"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "BEADS_AUTO_START_DAEMON=false")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	// bd resolves .beads from the GIT ROOT, not from $PWD: without this the
	// rig would find the enclosing repo's redirect and every write below
	// would land in the live fleet queue (ranger-base-rs8j).
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
	// The first ordinary command materialises the store. It imports nothing
	// on 0.50.3 and is not expected to.
	if out, err := bd("list", "--limit", "1"); err != nil {
		t.Skipf("bd could not materialise a store from a JSONL alone: %v %s", err, out)
	}

	// The class, before anything is read from it. A no-db store has no
	// config table for the rows below and no WAL for the third arm.
	db := filepath.Join(root, ".beads", "beads.db")
	if _, err := os.Stat(db); err != nil {
		t.Fatalf("this pin measures a SQLite store and bd built something else (%s: %v) — "+
			"the config table the rig writes and the WAL the third arm reads are both gone", db, err)
	}

	rows := "('issue_prefix','mainx')"
	if allowedPrefixes != "" {
		rows += ",('allowed_prefixes','" + allowedPrefixes + "')"
	}
	if out, err := exec.Command("sqlite3", db,
		"INSERT INTO config (key,value) VALUES "+rows+";").CombinedOutput(); err != nil {
		t.Fatalf("could not write the rig's config rows (bd's config table is not where this expects): %v %s", err, out)
	}

	// bd's own agreement that it can read what was written, and the settle
	// that makes the append below the only change either check reacts to.
	// Before ranger-base-v0v6l this is exactly where the pin died, with
	// `issue_prefix config is missing`.
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
		row := `{"id":"` + id + `","title":"6yf probe ` + id + `","status":"open",` +
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

// dropRow rewrites the rig's JSONL without any line naming id, and reports how
// many lines it removed. It is the recovery step the third arm pins.
func dropRow(t *testing.T, root, id string) int {
	t.Helper()
	jsonl := filepath.Join(root, ".beads", "issues.jsonl")
	b, err := os.ReadFile(jsonl)
	if err != nil {
		t.Fatalf("read %s: %v", jsonl, err)
	}
	var kept []string
	dropped := 0
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if strings.Contains(line, `"`+id+`"`) {
			dropped++
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(jsonl, []byte(strings.Join(kept, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("rewrite %s: %v", jsonl, err)
	}
	return dropped
}

// idBytesOnDisk counts the id's raw bytes in beads.db and in beads.db-wal.
// It is the third reader, and the only one that shares no blind spot with a
// SQL reader: it answers WHERE the row is, not merely whether some connection
// can see it. On 0.49.1 that is what made "the later command is a
// checkpointer, not a writer" falsifiable. On 0.50.3 it is what makes "the
// refusal wrote nothing" falsifiable, which is the stronger claim of the two:
// a SQL reader that could not see the row would be satisfied by a row sitting
// in a WAL it does not read, and this reader would not.
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
// so on 0.49.1 it reported a committed row as absent until some later command
// checkpointed — that single flag is what made an earlier version of this pin
// assert, and pass, the opposite mechanism. Opening the live database
// read-write instead would checkpoint it, and on 0.50.3 the third arm's whole
// claim is about what is on disk after the refusal; the snapshot carries
// beads.db-wal and recovers it out of harm's way.
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

const (
	// The batch gate's message (allow-list aware) and the create's message
	// (issue_prefix alone). Which one a refusal carries is the whole of the
	// first arm.
	batchGateRefusal = "prefix mismatch detected"
	createRefusal    = "does not match configured prefix"
	stalenessRefusal = "Database out of sync with JSONL"
)

func TestLiveBdImportPrefixAllowListDoesNotReachTheCreate(t *testing.T) {
	// The claim this pin is named for, and the only 0.49.1 claim that still
	// holds on 0.50.3. An ALLOWED foreign prefix clears the batch gate and is
	// refused at the create — allowed_prefixes never reaches that check.
	//
	// Both controls run here, because the arm is about WHICH check refused
	// and one refusal on its own cannot say: without the allow-list the same
	// row is stopped earlier, by the other message, and an in-prefix row on
	// the allow-list rig imports at exit 0. A rig that refused everything, or
	// that imported nothing at all, fails one of the two.
	t.Run("the allow-list clears the batch gate and is overruled at the create", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRig(t, "mainx,otherx")
		appendRow("otherx-6yf1")

		out, err := bd("--no-auto-import", "sync", "--import-only")
		if err == nil {
			t.Fatalf("the strict import took an allowed foreign prefix — the runbook's reading of allowed_prefixes would be right after all:\n%s", out)
		}
		if !strings.Contains(out, createRefusal) {
			t.Errorf("refused, but not by the single-prefix check this pin is about:\n%s", out)
		}
		if strings.Contains(out, batchGateRefusal) {
			t.Errorf("the batch gate refused an ALLOWED prefix, so allowed_prefixes was never read and this arm is measuring a rig fault, not bd:\n%s", out)
		}
		if inDB(t, root, "otherx-6yf1") {
			t.Errorf("refused at exit %v and the row is in the database anyway:\n%s", err, out)
		}

		// Control 1: the same row, the same command, no allow-list. It must
		// be stopped EARLIER, by the other message. This is what proves the
		// arm above measured the allow-list rather than a bd that refuses
		// every foreign prefix at the same place.
		_, plainBd, plainAppend := bdPrefixRig(t, "")
		plainAppend("otherx-6yf1")
		plainOut, plainErr := plainBd("--no-auto-import", "sync", "--import-only")
		if plainErr == nil {
			t.Fatalf("without allowed_prefixes a foreign prefix imported anyway, so the allow-list is not what the arm above measured:\n%s", plainOut)
		}
		if !strings.Contains(plainOut, batchGateRefusal) {
			t.Errorf("without allowed_prefixes the refusal did not come from the batch gate, so the two checks are no longer distinguishable by message and this arm cannot tell them apart:\n%s", plainOut)
		}

		// Control 2: the rig imports. An in-prefix row, otherwise identical,
		// at exit 0 — without this every assertion above is satisfied by a
		// bd that refuses everything.
		okRoot, okBd, okAppend := bdPrefixRig(t, "mainx,otherx")
		okAppend("mainx-6yf1b")
		okOut, okErr := okBd("--no-auto-import", "sync", "--import-only")
		if okErr != nil {
			t.Fatalf("an IN-PREFIX appended row was refused too, so this rig imports nothing and the refusals above mean nothing: %v\n%s", okErr, okOut)
		}
		if !inDB(t, okRoot, "mainx-6yf1b") {
			t.Fatalf("exit 0 and the in-prefix row is not in the database, so the rig's import path is not reached:\n%s", okOut)
		}
	})

	// The retired mechanism, pinned as its absence. On 0.49.1 this same
	// command took the row at exit 0, because a lenient auto-import pass ran
	// ahead of the strict one with SkipPrefixValidation:true. If bd ever
	// restores that pass, the disallowed row lands and this arm fails — which
	// is the point: the header's RETIRED note, the runbook's F3 ingest step
	// and the "append path is a way around validation" finding all turn on it.
	t.Run("the lenient auto-import that used to admit the row is gone", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRig(t, "mainx,otherx")
		appendRow("otherx-6yf2")

		// Half 1: with auto-import ENABLED, the same refusal as the
		// --no-auto-import spelling above. The flag no longer selects
		// between two behaviours.
		out, err := bd("sync", "--import-only")
		if err == nil {
			t.Fatalf("the lenient auto-import is back: the F3 command took an allowed foreign prefix at exit 0:\n%s", out)
		}
		if !strings.Contains(out, createRefusal) {
			t.Errorf("refused, but not at the create — the two spellings of this command no longer agree:\n%s", out)
		}
		if inDB(t, root, "otherx-6yf2") {
			t.Fatalf("the row is in the database after a refusal, so something still writes ahead of the strict pass:\n%s", out)
		}

		// Half 2: an ordinary command does not import an appended row at
		// all. It refuses, and --allow-stale skips the check WITHOUT
		// importing. Measured with an IN-PREFIX row, which no prefix check
		// could have rejected — so a row that fails to arrive here was not
		// imported and refused, it was never imported.
		appendRow("mainx-6yf2b")
		readOut, readErr := bd("list", "--limit", "0")
		if readErr == nil {
			t.Errorf("an ordinary read imported the appended rows instead of refusing, so the auto-import path is live again:\n%s", readOut)
		} else if !strings.Contains(readOut, stalenessRefusal) {
			t.Errorf("the ordinary read failed, but not with the staleness refusal this arm is about:\n%s", readOut)
		}
		staleOut, staleErr := bd("--allow-stale", "list", "--limit", "0")
		if staleErr != nil {
			t.Fatalf("--allow-stale did not get past the staleness check, so this half measured nothing: %v\n%s", staleErr, staleOut)
		}
		if inDB(t, root, "mainx-6yf2b") {
			t.Errorf("--allow-stale IMPORTED an in-prefix appended row rather than skipping the check:\n%s", staleOut)
		}
	})

	// The other retired mechanism. On 0.49.1 the command that reported the
	// prefix mismatch at exit 1 had already committed the row — the lenient
	// pass wrote it, the strict pass refused, and a WAL-blind reader was the
	// only thing that made it look like a rollback. With the lenient pass
	// gone the refusal really is a rollback, and this arm pins that the row
	// reaches NOTHING: not the main file, not the WAL, not either SQL reader.
	//
	// It is deliberately red rather than skipped if that changes back: a row
	// that reappears in the -wal is the trap returning, and $RB/docs/runbooks/
	// 0012-cutover.md §1 F2/F8 would be right again for a reason nobody
	// re-measured.
	t.Run("the refusing command writes nothing, and the JSONL line is the recovery", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRig(t, "")
		appendRow("zzspike-6yf3")

		// Negative control on the SQL reader. Without this, an inDB that
		// always answered "yes" would satisfy every assertion below.
		if inDB(t, root, "zzspike-6yf3") {
			t.Fatalf("the row is in the database before any import ran — inDB is not discriminating")
		}

		out, err := bd("sync", "--import-only")
		if err == nil {
			t.Fatalf("a disallowed prefix was accepted by the F3 command:\n%s", out)
		}
		if !strings.Contains(out, batchGateRefusal) {
			t.Errorf("refused, but not by the batch gate this arm is about:\n%s", out)
		}

		// Negative control on the byte reader: an id nobody ever appended
		// must be in neither file. Without it, a reader that counted nothing
		// at all would satisfy the two assertions after it — which is the
		// live hazard now that they assert zeroes.
		if m, w := idBytesOnDisk(t, root, "zzspike-never-appended"); m != 0 || w != 0 {
			t.Fatalf("the byte reader found an id that was never written (db=%d wal=%d) — it is not discriminating", m, w)
		}
		// And its positive control: the id it is asked about IS findable by
		// this reader somewhere, so a zero below is about the database and
		// not about a reader that cannot see anything.
		jsonl := filepath.Join(root, ".beads", "issues.jsonl")
		b, readErr := os.ReadFile(jsonl)
		if readErr != nil || bytes.Count(b, []byte("zzspike-6yf3")) == 0 {
			t.Fatalf("the appended row is not in %s (%v), so the rig never offered bd the row this arm is about", jsonl, readErr)
		}

		mainDB, wal := idBytesOnDisk(t, root, "zzspike-6yf3")
		if mainDB != 0 || wal != 0 {
			t.Errorf("the refusal left the row on disk (beads.db=%d beads.db-wal=%d) — exit 1 is a partial write again, "+
				"the trap this pin retired is back, and $RB/docs/runbooks/0012-cutover.md §1 F2/F8 need re-reading:\n%s",
				mainDB, wal, out)
		}
		if inDB(t, root, "zzspike-6yf3") {
			t.Errorf("the WAL-aware read sees the row the refusal was supposed to have rolled back:\n%s", out)
		}
		// The instrument that was a finding in its own right on 0.49.1:
		// `?immutable=1` does not read the -wal, so it reported a committed
		// row as absent. Here the two readers must AGREE — not because
		// immutable became safe, but because there is no longer a
		// WAL-resident row for them to disagree about. A disagreement is the
		// trap returning by another route.
		immutable, immErr := exec.Command("sqlite3",
			"file:"+filepath.Join(root, ".beads", "beads.db")+"?immutable=1",
			"SELECT count(*) FROM issues WHERE id='zzspike-6yf3';").Output()
		if immErr != nil {
			t.Fatalf("sqlite3 immutable read: %v", immErr)
		}
		if got := strings.TrimSpace(string(immutable)); got != "0" {
			t.Errorf("immutable=1 saw the row (%s) while the WAL-aware read did not — the row is in the main file after a refusal", got)
		}

		// It does not self-heal: the offending line is still in the JSONL,
		// so every later import fails identically.
		again, againErr := bd("sync", "--import-only")
		if againErr == nil {
			t.Errorf("the second import recovered on its own, so the refusal is not about the JSONL line:\n%s", again)
		}
		if inDB(t, root, "zzspike-6yf3") {
			t.Errorf("the second import admitted the row the first one refused:\n%s", again)
		}

		// And this is the whole recovery on 0.50.3, which is what changed for
		// an operator: drop the LINE. Nothing has to be got out of the
		// database, because nothing ever reached it.
		if n := dropRow(t, root, "zzspike-6yf3"); n != 1 {
			t.Fatalf("expected to drop exactly one JSONL line, dropped %d", n)
		}
		healed, healErr := bd("sync", "--import-only")
		if healErr != nil {
			t.Fatalf("removing the offending JSONL line did not heal the store, so recovery is more than the line after all: %v\n%s", healErr, healed)
		}
		if !inDB(t, root, "mainx-1") {
			t.Errorf("the seed row is gone after the recovery import:\n%s", healed)
		}
		if ordinary, ordErr := bd("list", "--limit", "0"); ordErr != nil {
			t.Errorf("an ordinary read still fails after the recovery import: %v\n%s", ordErr, ordinary)
		}
	})
}
