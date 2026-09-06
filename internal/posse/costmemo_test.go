//go:build posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

// ranger-base-325q. The cockpit re-decoded the whole 14-day transcript pile
// every 30 seconds — measured on this shop 2026-08-29 at 1211 files / 786 MB,
// of which 1206 files / 784.6 MB had not been written in the last 30s. A CPU
// profile put 62% of the process's samples in the transcript decoder. These
// tests pin the memo that answers it, and the two ways a memo goes wrong:
// serving an answer for bytes that moved, and handing out its own pointers.

// memoProvider is an adapter whose decode is COUNTED, so a test can say how
// many files a scan actually read rather than how long it took.
type memoProvider struct {
	files   []string
	decodes *atomic.Int32
}

func (memoProvider) Runtime() string { return "qa-memo" }
func (memoProvider) Reads() string   { return "qa fixture" }
func (memoProvider) Prices() bool    { return true }

func (memoProvider) PriceFor(string) (Price, bool) { return Price{}, false }

func (p memoProvider) Transcripts(string) ([]string, []error) { return p.files, nil }

func (p memoProvider) Decode(path string, _ time.Time) ([]*Segment, error) {
	p.decodes.Add(1)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// One segment per file, its bead id the file's contents: enough to tell
	// a stale answer from a fresh one.
	at := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	return []*Segment{{
		Bead: string(b), File: path, Start: at, End: at,
		Msgs: map[string]*Usage{"m1": {Model: "claude-opus-5", Out: 1000}},
	}}, nil
}

// registerMemoProvider installs the fixture adapter over an isolated HOME, so
// the shipped locators find nothing and the scan is exactly these files.
func registerMemoProvider(t *testing.T, names ...string) (memoProvider, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	p := memoProvider{decodes: &atomic.Int32{}}
	for _, n := range names {
		f := filepath.Join(dir, n)
		if err := os.WriteFile(f, []byte(n+"-v1"), 0o644); err != nil {
			t.Fatal(err)
		}
		p.files = append(p.files, f)
	}
	RegisterCostProvider(p)
	t.Cleanup(func() { delete(costProviders, p.Runtime()) })
	return p, dir
}

func beadIDs(rep *CostReport) []string {
	var out []string
	for _, s := range rep.Beads {
		out = append(out, s.Bead)
	}
	sort.Strings(out)
	return out
}

// A kept scanner re-reads only what moved. Three files, three scans: the
// second reads nothing, and after one is appended to the third reads exactly
// that one.
func TestCostScannerRereadsOnlyChangedFiles(t *testing.T) {
	p, _ := registerMemoProvider(t, "a.jsonl", "b.jsonl", "c.jsonl")
	cs := new(CostScanner)
	since := time.Time{}

	if got := beadIDs(cs.Scan("", since)); len(got) != 3 {
		t.Fatalf("first scan: %v", got)
	}
	if n := p.decodes.Load(); n != 3 {
		t.Fatalf("first scan decoded %d files, want 3", n)
	}

	p.decodes.Store(0)
	rep := cs.Scan("", since)
	if n := p.decodes.Load(); n != 0 {
		t.Fatalf("a scan over three UNCHANGED files decoded %d of them, want 0 — this is the 786 MB every 30s", n)
	}
	if got := beadIDs(rep); len(got) != 3 || got[0] != "a.jsonl-v1" {
		t.Fatalf("the memo served the wrong answer: %v", got)
	}

	// Append to one. mtime and size both move; the memo must not answer for
	// it, and must still answer for the other two.
	p.decodes.Store(0)
	if err := os.WriteFile(p.files[1], []byte("b.jsonl-v2-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep = cs.Scan("", since)
	if n := p.decodes.Load(); n != 1 {
		t.Fatalf("one file changed and the scan decoded %d, want 1", n)
	}
	if got := beadIDs(rep); len(got) != 3 || got[1] != "b.jsonl-v2-longer" {
		t.Fatalf("a changed file was served from the memo: %v", got)
	}
}

// The whole point of the key is that a rewrite is caught. Same size, new
// bytes, new mtime — the case an offset-based cache would miss.
func TestCostScannerCatchesARewriteOfTheSameLength(t *testing.T) {
	p, _ := registerMemoProvider(t, "a.jsonl")
	cs := new(CostScanner)
	cs.Scan("", time.Time{})

	p.decodes.Store(0)
	later := time.Now().Add(time.Second)
	if err := os.WriteFile(p.files[0], []byte("a.jsonl-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(p.files[0], later, later)
	rep := cs.Scan("", time.Time{})
	if n := p.decodes.Load(); n != 1 {
		t.Fatalf("a rewritten file was served from the memo (%d decodes)", n)
	}
	if got := beadIDs(rep); len(got) != 1 || got[0] != "a.jsonl-v2" {
		t.Fatalf("stale answer after a rewrite: %v", got)
	}
}

// A different window is a different answer — ScanTranscript drops assistant
// records before `since` — so the memo must not serve across one.
func TestCostScannerDoesNotServeAcrossAWindow(t *testing.T) {
	p, _ := registerMemoProvider(t, "a.jsonl")
	cs := new(CostScanner)
	cs.Scan("", time.Time{})
	p.decodes.Store(0)
	cs.Scan("", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if n := p.decodes.Load(); n != 1 {
		t.Fatalf("a scan under a different `since` was served from the memo (%d decodes)", n)
	}
}

// The memo hands out copies. The scan writes Segment.Runtime and
// AttributePersonas writes Segment.Persona: if callers got the memo's own
// pointers, one report's attribution would rewrite the next one's, and two
// concurrent scans would race over the same struct.
//
// Both the miss path and the hit path must copy, and they are two separate
// returns: writing through a MISS result is the arm that catches a scanner
// storing the same slice it hands back, and writing through a HIT result is
// the arm that catches the memo serving its own pointers on every later scan.
func TestCostScannerHandsOutCopies(t *testing.T) {
	p, _ := registerMemoProvider(t, "a.jsonl")
	cs := new(CostScanner)

	miss := cs.Scan("", time.Time{}) // decoded
	miss.Beads[0].Persona, miss.Beads[0].Bead = "qa-persona", "clobbered-by-the-miss"

	hit := cs.Scan("", time.Time{}) // served
	if n := p.decodes.Load(); n != 1 {
		t.Fatalf("the second scan re-decoded (%d): this test is no longer about the memo", n)
	}
	if hit.Beads[0].Persona != "" || hit.Beads[0].Bead != "a.jsonl-v1" {
		t.Fatalf("a write through the decoded result reached the memo: %+v", hit.Beads[0])
	}
	hit.Beads[0].Persona, hit.Beads[0].Bead = "qa-persona", "clobbered-by-the-hit"

	again := cs.Scan("", time.Time{}) // served again
	if again.Beads[0].Persona != "" || again.Beads[0].Bead != "a.jsonl-v1" {
		t.Fatalf("a write through a SERVED result reached the memo: %+v", again.Beads[0])
	}
	if hit.Beads[0] == again.Beads[0] {
		t.Fatal("two served scans returned the same *Segment — concurrent scans would race over it")
	}
	// The runtime still lands on every copy: it is written by the scan, and
	// a copy that lost it would print an unattributed row.
	if again.Beads[0].Runtime != "qa-memo" {
		t.Errorf("the copy lost its runtime: %q", again.Beads[0].Runtime)
	}
}

// A cockpit open for a week must not remember every transcript ever rotated
// away, so a file that stops being listed is forgotten.
func TestCostScannerForgetsFilesThatVanish(t *testing.T) {
	p, _ := registerMemoProvider(t, "a.jsonl", "b.jsonl")
	cs := new(CostScanner)
	cs.Scan("", time.Time{})
	if got := len(cs.memo); got != 2 {
		t.Fatalf("memo holds %d entries after two files", got)
	}
	gone := memoProvider{files: p.files[:1], decodes: p.decodes}
	RegisterCostProvider(gone)
	cs.Scan("", time.Time{})
	if got := len(cs.memo); got != 1 {
		t.Fatalf("memo holds %d entries after one file was delisted, want 1", got)
	}
}

// ScanCosts keeps NO memory: it is the one-shot form, its callers pass a
// fresh `since` every time, and a hidden process-wide cache under the
// dispatcher's budget guard is not something this bead gets to add.
func TestScanCostsKeepsNoMemoryBetweenCalls(t *testing.T) {
	p, _ := registerMemoProvider(t, "a.jsonl")
	ScanCosts("", time.Time{})
	p.decodes.Store(0)
	rep := ScanCosts("", time.Time{})
	if n := p.decodes.Load(); n != 1 {
		t.Fatalf("ScanCosts served from a cache (%d decodes): its callers pass a fresh window and expect a fresh read", n)
	}
	if got := beadIDs(rep); len(got) != 1 || got[0] != "a.jsonl-v1" {
		t.Fatalf("ScanCosts answer changed: %v", got)
	}
}

// A memoised scan and a cold one must report the same thing. The memo is an
// optimisation and nothing else; this is the arm that would catch it becoming
// a behaviour change.
func TestCostScannerAgreesWithACodeScan(t *testing.T) {
	registerMemoProvider(t, "a.jsonl", "b.jsonl", "c.jsonl")
	cs := new(CostScanner)
	cs.Scan("", time.Time{})         // warm
	warm := cs.Scan("", time.Time{}) // served from the memo
	cold := ScanCosts("", time.Time{})

	if a, b := beadIDs(warm), beadIDs(cold); len(a) != len(b) {
		t.Fatalf("memoised %v vs cold %v", a, b)
	} else {
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("memoised %v vs cold %v", a, b)
			}
		}
	}
	if warm.DayTotal(time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)) != cold.DayTotal(time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("the memoised scan priced the day differently")
	}
}
