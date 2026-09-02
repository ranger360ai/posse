package posse

// Probes filed by QA verifying the close of ranger-base-2y96
// (verify bead ranger-base-n1hi). 2y96 taught the OVERFLOW ledger that a
// readable-but-unwritable ledger is a cap of zero that reads as room
// forever. uncounted.log is the same shape — a rolling-7d count that
// `uncounted_cap_<runtime>:` is compared against, appended to after the
// launch — and uncountedSkip asked only whether it was READABLE. Fixed on
// ranger-base-ws09: uncountedFor probes UncountedAppendable whenever a cap is
// set and uncountedSkip refuses on it, so the probe below runs. Kept in their
// own file so they survive whatever the next persona does to
// uncounted_test.go.

import (
	"os"
	"testing"
)

// PROBE (QA, verifying ranger-base-2y96). The 2y96 fix taught the
// OVERFLOW ledger that an unwritable ledger is a cap of zero that reads as
// room. uncounted.log is the same shape — a rolling-7d count that
// `uncounted_cap_<rt>:` is compared against, appended to after the launch —
// and uncountedSkip only asked whether it is READABLE. Un-skipped unchanged
// by the fix (ranger-base-ws09).
func TestQAProbeUncountedUnwritableLedger(t *testing.T) {
	const cfg = "uncounted_cap_codex: 1\n"
	f1 := oneCodexBead(t, cfg)
	f2 := oneCodexBead(t, cfg)
	f2.b.App.StateDir = f1.b.App.StateDir

	if err := os.MkdirAll(f1.b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f1.b.App.UncountedLogPath(), nil, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f1.b.App.UncountedLogPath(), 0o444); err != nil {
		t.Fatal(err)
	}
	// The hostile condition must be realized, or zero launches proves nothing.
	if err := f1.b.App.AppendUncounted(LedgerEntry{Runtime: "codex"}); err == nil {
		t.Skip("test process can append to a 0444 ledger")
	}

	total := 0
	for _, f := range []*uncountedFixture{f1, f2} {
		n, err := f.d.Run("", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		total += n
		t.Logf("pass out:\n%s\npass stderr:\n%s", dispatcherOut(f.d), f.errb.String())
	}
	if n := qaProbeLedgerBytes(t, f1.b.App.UncountedLogPath()); n != 0 {
		t.Fatalf("ledger has %d bytes; the fixture expects it to stay empty", n)
	}
	if total != 0 {
		t.Fatalf("readable but unwritable uncounted ledger admitted %d unrecorded launches with cap 1; want 0", total)
	}
}

// The ledger is left exactly as found — empty — so every later pass reads it
// as room again. That is what makes the leak unbounded rather than a one-off.
func qaProbeLedgerBytes(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return len(b)
}

// CONTROL for the probe above: the same two passes over the same StateDir
// with a WRITABLE ledger. The cap holds — one launch, then the skip — so the
// probe's "2 under cap 1" is the ledger's writability and nothing else about
// the rig.
func TestQAProbeUncountedWritableLedgerControl(t *testing.T) {
	const cfg = "uncounted_cap_codex: 1\n"
	f1 := oneCodexBead(t, cfg)
	f2 := oneCodexBead(t, cfg)
	f2.b.App.StateDir = f1.b.App.StateDir
	if err := os.MkdirAll(f1.b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f1.b.App.UncountedLogPath(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, f := range []*uncountedFixture{f1, f2} {
		n, err := f.d.Run("", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		total += n
	}
	if total != 1 {
		t.Fatalf("a writable ledger under cap 1 must admit exactly 1 launch over two passes, got %d\npass2 out:\n%s", total, dispatcherOut(f2.d))
	}
	if n := qaProbeLedgerBytes(t, f1.b.App.UncountedLogPath()); n == 0 {
		t.Fatal("the control's ledger must actually have been written")
	}
}
