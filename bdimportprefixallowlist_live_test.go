package posse

// Live pin for ranger-base-6yf, run against the real bd rather than a fixture:
//
//	`allowed_prefixes` does not let a strict import CREATE a foreign-prefix
//	issue. It clears the importer's batch gate and is then overruled by the
//	storage layer's own single-prefix check. What actually admits the row is
//	bd's LENIENT auto-import, which skips prefix validation altogether — and
//	it admits a disallowed prefix just as happily, one command later, at
//	exit 0, silently.
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

// inDB answers from the JSONL-independent side: the row is in the database or
// it is not. `bd show` cannot be the instrument here — running it is one of the
// things under test.
func inDB(t *testing.T, root, id string) bool {
	t.Helper()
	out, err := exec.Command("sqlite3",
		"file:"+filepath.Join(root, ".beads", "beads.db")+"?immutable=1",
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

	// Trap 2, corrected. The strict refusal IS a rollback. What plants the row
	// is the next ordinary bd command, at exit 0, saying nothing.
	t.Run("a disallowed prefix lands on the next command, silently", func(t *testing.T) {
		root, bd, appendRow := bdPrefixRig(t)
		appendRow("zzspike-6yf2")

		out, err := bd("sync", "--import-only")
		if err == nil {
			t.Fatalf("a disallowed prefix was accepted by the F3 command:\n%s", out)
		}
		if inDB(t, root, "zzspike-6yf2") {
			t.Fatalf("the refused row was written by the refusing command itself:\n%s", out)
		}

		// One read. No flags, no import, nothing that announces a write.
		readOut, readErr := bd("list", "--limit", "1")
		if readErr != nil {
			t.Fatalf("plain read failed, so this arm measured nothing: %v\n%s", readErr, readOut)
		}
		// Measured deterministic, 3/3 on a copy of a real 964-row store and
		// again here. If bd ever stops doing this the pin should go red loudly
		// — that is good news, and good news nobody reads is a SKIP.
		if !inDB(t, root, "zzspike-6yf2") {
			t.Fatalf("the disallowed row did not land on the next read; the silent writer this pin is about is gone or has moved:\n%s", readOut)
		}
		if strings.Contains(readOut, "zzspike") {
			t.Errorf("the read did name the prefix it imported, which would make this survivable:\n%s", readOut)
		}
	})
}
