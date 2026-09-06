//go:build posse_arm3

package posse

// The launch ledger's own contract (ledger.go), read straight through the
// one ledger that has this shape today — `uncounted.log`, ADR 0013 §5. Both
// pins here were bought by defects in the ledger the automatic overflow used
// to keep, and neither is about that mechanism: they are about what a count
// off a file may and may not be allowed to mean.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// countLedger's shape contract (ranger-base-lasj): a line that is not a
// ledger entry makes the WEEK unknown, not the line zero. A skip would say
// "no launch happened", which is the one thing a torn write does not tell
// you, and the caller already fails closed on an error.
func TestLedgerCorruptLineIsUnknownNotZero(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	now := time.Now()
	good := LedgerEntry{At: now.Add(-time.Hour), Runtime: "grok", Bead: "a-1", Persona: "ranger"}.line()

	for _, tc := range []struct {
		name  string
		body  string
		count int  // when it parses
		bad   bool // when it does not
	}{
		{name: "well formed", body: good + good, count: 2},
		{name: "blank lines are not records", body: good + "\n   \n" + good, count: 2},
		{name: "torn timestamp on the target", body: "2026-08-26T12:00 grok prior-1 ranger\n", bad: true},
		{name: "torn timestamp on another pool", body: good + "2026-08-26T12:00 codex prior-1 ranger\n", bad: true},
		{name: "truncated line", body: good + "2026-08-26T12:00:00Z grok\n", bad: true},
		{name: "not a ledger at all", body: "hello\n", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.MkdirAll(b.App.StateDir, 0o755)
			if err := os.WriteFile(b.App.UncountedLogPath(), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			n, err := b.App.UncountedCount("grok", now)
			switch {
			case tc.bad && err == nil:
				t.Errorf("counted a corrupt ledger as %d; want an error so the cap fails closed", n)
			case tc.bad:
				if n != 0 {
					t.Errorf("returned %d alongside its error; an unknown count must not look like a number", n)
				}
			case err != nil:
				t.Errorf("%v", err)
			case n != tc.count:
				t.Errorf("= %d, want %d", n, tc.count)
			}
		})
	}
}

// The rolling window is per runtime and it rolls: entries inside it count,
// entries older than it do not, and another pool's entries are not this
// pool's week.
func TestLedgerCountIsPerRuntimeAndRolls(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	now := time.Now()
	var seed strings.Builder
	for _, e := range []LedgerEntry{
		{At: now.Add(-time.Hour), Runtime: "grok", Bead: "a-1", Persona: "ranger"},
		{At: now.Add(-6 * 24 * time.Hour), Runtime: "grok", Bead: "a-2", Persona: "ranger"},
		{At: now.Add(-8 * 24 * time.Hour), Runtime: "grok", Bead: "a-3", Persona: "ranger"},
		{At: now.Add(-time.Hour), Runtime: "codex", Bead: "a-4", Persona: "ranger"},
	} {
		seed.WriteString(e.line())
	}
	os.MkdirAll(b.App.StateDir, 0o755)
	if err := os.WriteFile(b.App.UncountedLogPath(), []byte(seed.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := b.App.UncountedCount("grok", now); err != nil || got != 2 {
		t.Errorf("grok = %d, %v — want 2 (entries past %s do not count)", got, err, LedgerWindow)
	}
	if got, _ := b.App.UncountedCount("codex", now); got != 1 {
		t.Errorf("codex = %d, want 1 — the count is per pool", got)
	}
}

// The append probe (ranger-base-2y96), on its own. It has to answer for a
// ledger that does not exist yet as well as one that does — the first append
// creates the file, so a directory nothing may write to is the same refusal
// as a file nothing may write to — and it must answer without leaving either
// one changed.
func TestLedgerAppendable(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	if err := os.MkdirAll(b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := b.App.UncountedLogPath()

	t.Run("no ledger yet is appendable and stays absent", func(t *testing.T) {
		if err := b.App.UncountedAppendable(); err != nil {
			t.Fatalf("a writable StateDir with no ledger must be appendable: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("the probe must not create the ledger (%v)", err)
		}
		// And nothing else is left behind either.
		ents, err := os.ReadDir(b.App.StateDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), ".ledger-probe-") {
				t.Errorf("the probe file survived: %s", e.Name())
			}
		}
	})

	t.Run("a writable ledger is appendable and unchanged", func(t *testing.T) {
		body := LedgerEntry{At: time.Now(), Runtime: "grok", Bead: "a-1", Persona: "ranger"}.line()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := b.App.UncountedAppendable(); err != nil {
			t.Fatalf("a 0644 ledger must be appendable: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != body {
			t.Errorf("the probe wrote to the ledger: %q err=%v", got, err)
		}
	})

	t.Run("a 0444 ledger is not", func(t *testing.T) {
		if err := os.Chmod(path, 0o444); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(path, 0o644) })
		// 0444 is a promise about a uid, not about the process: root keeps
		// its write and would turn the repro into a false pass.
		if err := b.App.AppendUncounted(LedgerEntry{Runtime: "grok"}); err == nil {
			t.Skip("test process can append to a 0444 ledger")
		}
		if err := b.App.UncountedAppendable(); err == nil {
			t.Fatal("a ledger this process cannot append to must be refused")
		}
	})

	t.Run("no ledger and an unwritable StateDir is not", func(t *testing.T) {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(b.App.StateDir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(b.App.StateDir, 0o755) })
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			f.Close()
			os.Remove(path)
			t.Skip("test process can create files in a 0555 directory")
		}
		if err := b.App.UncountedAppendable(); err == nil {
			t.Fatal("a StateDir the first append could not create the ledger in must be refused")
		}
	})
}
