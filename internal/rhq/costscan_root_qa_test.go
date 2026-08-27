package rhq

// ADR 0018 §3 — "the degraded brake must itself be honest" — verified, and
// three arms of it are not.
//
// §3's own words: "an unreadable transcript root today reads as $0 spent,
// i.e. an armed brake that counts nothing", so "the cost scan learns to
// distinguish *no records* from *cannot read*". What landed guards the
// listing with one os.Stat on the root and then calls filepath.Glob — and
// Glob ignores every I/O error by design (path/filepath/match.go glob():
// "ignore I/O error"). So only a root whose STAT fails is reported, which
// is the one arm blinddegrade_test.go's TestScanCostsUnreadableRootIsNotA
// QuietDay exercises (it chmods the PARENT, ~/.claude). The root itself and
// every directory under it still read as a quiet day.
//
// These tests are GREEN ON PURPOSE: each asserts the LIVE behaviour, not
// the contract, because NOTES.md's silent-revert lesson is that a skipped
// pin is how a defect stays green. Filed as ranger-base-e06g; when it
// lands, invert each one — every failure message below says how.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// claudeProjects makes a transcript root with one readable record in it and
// returns the root, so a test can then break exactly one level of it.
func claudeProjects(t *testing.T, home string) string {
	t.Helper()
	root := filepath.Join(home, ".claude", "projects")
	if err := os.MkdirAll(filepath.Join(root, "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "p", "s.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// chmodBack restores a mode at test end, so t.TempDir's own cleanup can
// still walk what this test broke.
func chmodBack(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o755) })
}

// ARM A — the root ITSELF unreadable, its parent fine. This is the ADR's
// own example, and it is the arm the guard cannot see: stat on a directory
// needs only +x on its PARENT, so os.Stat succeeds and Glob swallows the
// ReadDir failure underneath it.
func TestScanCostsRootItselfUnreadableStillReadsAsZero(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this arm needs an unprivileged uid")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	chmodBack(t, claudeProjects(t, home))

	rep := ScanCosts("", time.Time{})
	if rep.ReadErr != nil {
		t.Fatalf("ranger-base-e06g LANDED — invert this test: a chmod-000 transcript root is now reported (%v, %d unread). Assert ReadErr != nil here.", rep.ReadErr, rep.Unread)
	}
	// The live defect, stated so it cannot be edited away by accident.
	if rep.Unread != 0 || len(rep.Beads) != 0 {
		t.Errorf("unexpected third state: ReadErr=nil but Unread=%d beads=%d", rep.Unread, len(rep.Beads))
	}
}

// ARM B — the root replaced by a regular file. transcriptFiles' own doc
// comment names this case ("a directory replaced by a file") as one that
// "is a read failure and says so". Stat succeeds on a file, so it does not.
func TestScanCostsRootReplacedByAFileStillReadsAsZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "projects"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if rep := ScanCosts("", time.Time{}); rep.ReadErr != nil {
		t.Fatalf("ranger-base-e06g LANDED — invert this test: a transcript root replaced by a file is now reported (%v). Assert ReadErr != nil here.", rep.ReadErr)
	}
}

// ARM C — one project directory unreadable. That project's entire spend
// disappears from the ledger and nothing says so, which is the same fault
// one level down: a floor presented as a total.
func TestScanCostsUnreadableProjectDirStillReadsAsZero(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this arm needs an unprivileged uid")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	chmodBack(t, filepath.Join(claudeProjects(t, home), "p"))

	if rep := ScanCosts("", time.Time{}); rep.ReadErr != nil {
		t.Fatalf("ranger-base-e06g LANDED — invert this test: an unreadable project dir is now reported (%v, %d unread). Assert ReadErr != nil here.", rep.ReadErr, rep.Unread)
	}
}

// And what all three cost, at the grain the ADR cares about. §1's degrade
// runs the pass under the ledger; §3 says a ledger that cannot be read is
// no floor at all and parks "exactly as an unarmed Dial E would". With the
// root chmod-000 the scan reports $0.00, the receipt prints those zeroes as
// if they were counted, and the pass dispatches.
//
// This is the ONLY test in the suite that puts blindFork over the real
// ScanCosts — the ADR's arms are all pinned through an injected Spend,
// which is why the gap between "what the scan reports" and "what the scan
// can see" had nowhere to show.
func TestDegradedPassOverUnreadableRootDispatchesOnAnEmptyReceipt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this needs an unprivileged uid")
	}
	r := newBlindRig(t, ledgerArmedCfg)
	r.d.Unattended = true
	r.d.Spend = nil // the real ScanCosts, against a real broken root
	chmodBack(t, claudeProjects(t, os.Getenv("HOME")))

	r.blind()
	r.at(4 * time.Hour)
	n := r.run(t)

	if n == 0 {
		t.Fatalf("ranger-base-e06g LANDED — invert this test: a degraded pass over an unreadable ledger now parks. Assert n == 0 and that the park line names the ledger.\n%s", r.out())
	}
	// The live defect, with its receipt, so the shape is on the record.
	if !strings.Contains(r.out(), "degraded, running under ledger brake (pass $0.00/$30.00, day $0.00/$250.00)") {
		t.Errorf("want the empty receipt this bead is about, got:\n%s", r.out())
	}
	if strings.Contains(r.out(), "ledger unreadable") {
		t.Errorf("the §3 park line fired but the pass still ran — that is a different bug:\n%s", r.out())
	}
}
