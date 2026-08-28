package rhq

// ADR 0018 §3 — "the degraded brake must itself be honest" — at the three
// grains the first cut of it could not see.
//
// §3's own words: "an unreadable transcript root today reads as $0 spent,
// i.e. an armed brake that counts nothing", so "the cost scan learns to
// distinguish *no records* from *cannot read*". What landed first guarded
// the listing with one os.Stat on the root and then called filepath.Glob —
// and Glob ignores every I/O error by design (path/filepath/match.go
// glob(): "ignore I/O error"). So only a root whose STAT failed was
// reported, which is the one arm blinddegrade_test.go's TestScanCosts
// UnreadableRootIsNotAQuietDay exercises (it chmods the PARENT, ~/.claude).
// The root itself and every directory under it still read as a quiet day.
//
// Filed as ranger-base-e06g and fixed by walking with os.ReadDir. These
// tests were committed asserting the LIVE defect, per NOTES.md's silent-
// revert lesson; each is now inverted to assert the contract.

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
// own example, and the arm a stat guard cannot see: stat on a directory
// needs only +x on its PARENT, so os.Stat succeeded and Glob swallowed the
// ReadDir failure underneath it.
func TestScanCostsRootItselfUnreadableIsAReadFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this arm needs an unprivileged uid")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := claudeProjects(t, home)
	chmodBack(t, root)

	rep := ScanCosts("", time.Time{})
	if rep.ReadErr == nil || rep.Unread != 1 {
		t.Fatalf("a chmod-000 transcript root must not read as $0 spent, got %v (%d unread)", rep.ReadErr, rep.Unread)
	}
	if !strings.Contains(rep.ReadErr.Error(), root) {
		t.Errorf("the error must name what it could not read: %v", rep.ReadErr)
	}
	if len(rep.Beads) != 0 {
		t.Errorf("nothing was readable, so nothing may be reported: %d bead segments", len(rep.Beads))
	}
}

// ARM B — the root replaced by a regular file. transcriptFiles' own doc
// comment names this case ("a directory replaced by a file") as one that
// "is a read failure and says so". Stat succeeds on a file, so it did not.
func TestScanCostsRootReplacedByAFileIsAReadFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "projects"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if rep := ScanCosts("", time.Time{}); rep.ReadErr == nil {
		t.Fatal("a transcript root replaced by a file is a fault, not an empty ledger")
	}
}

// ARM C — one project directory unreadable. That project's entire spend
// would disappear from the ledger with nothing saying so: the same fault
// one level down, a floor presented as a total. What the scan CAN still
// read stays in the report — a partial ledger beats no ledger — but it
// travels with the fact that it is partial.
func TestScanCostsUnreadableProjectDirIsAReadFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this arm needs an unprivileged uid")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := claudeProjects(t, home)
	// A second project, readable, with a real segment in it: the floor.
	q := filepath.Join(root, "q")
	if err := os.MkdirAll(q, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTranscript(t, q, "s.jsonl",
		`{"type":"user","timestamp":"2026-08-27T09:00:00Z","message":{"content":"Work beads issue a-1: t"}}`,
		asst("m1", "claude-opus-5", "2026-08-27T09:00:01Z", 0, 0, 0, 1000))
	chmodBack(t, filepath.Join(root, "p"))

	rep := ScanCosts("", time.Time{})
	if rep.ReadErr == nil || rep.Unread != 1 {
		t.Fatalf("an unreadable project dir must be reported, got %v (%d unread)", rep.ReadErr, rep.Unread)
	}
	if !strings.Contains(rep.ReadErr.Error(), filepath.Join(root, "p")) {
		t.Errorf("the error must name the directory it could not read: %v", rep.ReadErr)
	}
	if len(rep.Beads) != 1 {
		t.Errorf("the readable project is still the floor: %d bead segments", len(rep.Beads))
	}
}

// And what all three cost, at the grain the ADR cares about. §1's degrade
// runs the pass under the ledger; §3 says a ledger that cannot be read is
// no floor at all and parks "exactly as an unarmed Dial E would". With the
// root chmod-000 the scan used to report $0.00, the receipt printed those
// zeroes as if they were counted, and the pass dispatched.
//
// This is the ONLY test in the suite that puts blindFork over the real
// ScanCosts — the ADR's arms are all pinned through an injected Spend,
// which is why the gap between "what the scan reports" and "what the scan
// can see" had nowhere to show. Hermetic all the same: HOME is this test's
// own temp dir, so the real ScanCosts reads a real broken root that is not
// the operator's (a nil d.Spend over the live $HOME would read their
// ledger — see TestBlindDegradeIsUnattendedOnly, ranger-base-rp2y).
func TestDegradedPassOverUnreadableRootParksOnTheLedger(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this needs an unprivileged uid")
	}
	home := t.TempDir()
	r := newBlindRig(t, ledgerArmedCfg)
	// After the rig, not before: newTestBackend gives every backend test a
	// temp $HOME of its own (ranger-base-gvrh), and this test needs $HOME to
	// be the one whose ledger root it just broke.
	t.Setenv("HOME", home)
	chmodBack(t, claudeProjects(t, home))

	r.d.Unattended = true
	r.d.Spend = nil // the real ScanCosts, against a real broken root
	r.blind()
	r.at(4 * time.Hour)

	if n := r.run(t); n != 0 {
		t.Fatalf("a degraded pass over an unreadable ledger must park: %d dispatched\n%s", n, r.out())
	}
	out := r.out()
	for _, want := range []string{"plan guard: blind 4h00m", "ledger unreadable", "permission denied", "— skipped"} {
		if !strings.Contains(out, want) {
			t.Errorf("the park line must carry %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "degraded") {
		t.Errorf("a pass whose ledger counted nothing must not claim a brake:\n%s", out)
	}
	// The receipt this bead is named for: zeroes printed as if counted.
	if strings.Contains(out, "epoch $0.00/$30.00") {
		t.Errorf("an empty receipt for a ledger that could not be read:\n%s", out)
	}
}

// The without-arm the test above needs, and the third of this bead's three
// (ranger-base-4s6f): "empty-but-readable root still reads $0 and does not
// park." Same rig, same real ScanCosts, a root that OPENS and holds no
// priced record — $0 from a store that read cleanly is a reading, not a
// fault, so the pass degrades and dispatches.
//
// Without it, "a degraded pass over an unreadable root parks" is satisfied
// by a rig that parks over ANY real scan: the numbers a broken root
// produces and the numbers a bug in the nil-Spend path produces are the
// same numbers. This arm is what makes the pair discriminate.
func TestDegradedPassOverReadableEmptyRootRunsOnTheLedger(t *testing.T) {
	home := t.TempDir()
	r := newBlindRig(t, ledgerArmedCfg)
	// After the rig, for the reason its without-arm gives (ranger-base-gvrh).
	t.Setenv("HOME", home)
	claudeProjects(t, home) // readable; one record, nothing priced in it

	r.d.Unattended = true
	r.d.Spend = nil // the real ScanCosts, against a real readable root
	r.blind()
	r.at(4 * time.Hour)

	if n := r.run(t); n != 1 {
		t.Fatalf("$0 over a ledger that opened is a reading: %d dispatched\n%s", n, r.out())
	}
	out := r.out()
	for _, want := range []string{
		"plan guard: blind 4h00m",
		"degraded, running under ledger brake",
		"epoch $0.00/$30.00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the degraded line must carry %q, got:\n%s", want, out)
		}
	}
	for _, never := range []string{"ledger unreadable", "— skipped"} {
		if strings.Contains(out, never) {
			t.Errorf("a readable ledger must not park (%q), got:\n%s", never, out)
		}
	}
	if e := r.err(); strings.Contains(e, "unreadable") {
		t.Errorf("nothing was unreadable; stderr must not say so: %q", e)
	}
}

// ARM D — TWO unreadable project directories are TWO unknown piles of
// spend, not one.
//
// This arm exists because of how the seam re-landed (ranger-base-k7nb).
// 6217c9f wrote CostProvider.Transcripts as (files, error) — one error per
// provider — while ARM C's fix, which landed in between, made the locator
// keep walking and return one error per directory it could not open. The
// merge had to pick, and picking the single error would have collapsed
// every unreadable dir into Unread=1 with ARM C still green: it only ever
// breaks one. So the seam carries []error, and this is the arm that says
// so. Delete the loop in scanProvider and count the first error only, and
// this test reads 1 where it must read 2.
func TestScanCostsCountsEveryUnreadableProjectDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this arm needs an unprivileged uid")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := claudeProjects(t, home)
	second := filepath.Join(root, "p2")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}
	chmodBack(t, filepath.Join(root, "p"))
	chmodBack(t, second)

	rep := ScanCosts("", time.Time{})
	if rep.Unread != 2 {
		t.Fatalf("two unreadable project dirs are two unknown piles of spend: %d unread (%v)", rep.Unread, rep.ReadErr)
	}
}
