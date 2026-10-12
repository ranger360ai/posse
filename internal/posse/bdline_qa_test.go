package posse

// QA pins for ranger-base-292wj / ADR 0072 B1 — the line-cliff judge
// (CensusLongLines, ProjectBdWrite), its two typed errors, and the D3/D4
// wiring into Bd.Comment/SetDescription/Create/Close and Bd.run/runOnce.
//
// Every arm here is offline: CensusLongLines and ProjectBdWrite read a
// hand-written census directly, and the D3/D4 wiring is driven through the
// package's fake bd (herdr_test.go), never the real binary. The live arm —
// RHQ_LIVE_BD=1, verification items 4 and 5 of the ADR — is
// bdlinelive_test.go.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bdRow is one hand-written census line, the shape bd itself writes
// (ADR 0072 Context/probe), with description as its one variable-length
// field.
func bdRow(id, description string) string {
	return `{"id":"` + id + `","title":"row","description":"` + description +
		`","status":"open","priority":2,"issue_type":"task","created_at":"2026-10-12T12:00:00Z","updated_at":"2026-10-12T12:00:00Z","created_by":"rig","owner":"rig","labels":[],"dependencies":[],"comments":[]}` + "\n"
}

func writeCensus(t *testing.T, rows ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "issues.jsonl")
	write(t, path, strings.Join(rows, ""))
	return path
}

// The pin the bead was filed for: ProjectBdWrite's comment growth matches
// what bd's own encoder actually produces, within the fixture's own
// tolerance (ADR 0072 Context, D1): 1,000 each of `<`, `\n`, `"`, é and
// `\t` — 6,000 raw bytes — grows the line by 9,072 bytes, comment envelope
// included.
func TestQAProjectBdWriteMatchesRealLineGrowth(t *testing.T) {
	t.Parallel()
	census := writeCensus(t, bdRow("t-1", "small"), bdRow("t-2", "x"), bdRow("t-3", "small too"))
	text := strings.Repeat("<", 1000) + strings.Repeat("\n", 1000) + strings.Repeat(`"`, 1000) +
		strings.Repeat("é", 1000) + strings.Repeat("\t", 1000)
	if len(text) != 6000 {
		t.Fatalf("fixture text is %d bytes, want 6000 (the ADR's own count)", len(text))
	}
	proj, ok, err := ProjectBdWrite(census, []string{"comments", "add", "t-2", text})
	if err != nil || !ok {
		t.Fatalf("ProjectBdWrite(t-2, 6000B fixture) = ok=%v err=%v, want a judged write", ok, err)
	}
	if proj.ID != "t-2" {
		t.Errorf("ID = %q, want t-2", proj.ID)
	}
	growth := proj.After - proj.Before
	const want, tolerance = 9072, 64
	if growth < want-tolerance || growth > want+tolerance {
		t.Errorf("growth = %d bytes, want %d ± %d", growth, want, tolerance)
	}
}

// Every residual ProjectBdWrite is told not to judge (ADR 0072 D1): an
// absent census, an id the census does not carry, and text arriving on
// bd's stdin via `-f -`, `--body-file -`, `-f /dev/stdin`.
func TestQAProjectBdWriteDoesNotJudgeResiduals(t *testing.T) {
	t.Parallel()
	census := writeCensus(t, bdRow("t-1", "small"))
	missing := filepath.Join(t.TempDir(), "no-such-census.jsonl")

	cases := []struct {
		name string
		path string
		argv []string
	}{
		{"census absent", missing, []string{"comments", "add", "t-1", "hi"}},
		{"no exact id line", census, []string{"comments", "add", "t-9", "hi"}},
		{"text on stdin via -f -", census, []string{"comments", "add", "t-1", "-f", "-"}},
		{"text on stdin via --body-file -", census, []string{"update", "t-1", "--body-file", "-"}},
		{"text on stdin via /dev/stdin", census, []string{"comments", "add", "t-1", "-f", "/dev/stdin"}},
		{"unrecognized flag", census, []string{"update", "t-1", "--due", "tomorrow"}},
		{"bare read verb", census, []string{"show", "t-1"}},
		{"update touching no text field", census, []string{"update", "t-1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ok, err := ProjectBdWrite(c.path, c.argv)
			if err != nil {
				t.Fatalf("ProjectBdWrite(%v) returned an error instead of ok=false: %v", c.argv, err)
			}
			if ok {
				t.Errorf("ProjectBdWrite(%v) = ok=true, want ok=false (do not judge)", c.argv)
			}
		})
	}
}

// update/close/create route through the same judge with their own field
// mapping — most notably `--acceptance`, which bd writes under the JSON key
// `acceptance_criteria` rather than its own flag name (MEASURED).
func TestQAProjectBdWriteFieldMapping(t *testing.T) {
	t.Parallel()
	census := writeCensus(t, bdRow("t-1", "short"))

	t.Run("update --acceptance", func(t *testing.T) {
		proj, ok, err := ProjectBdWrite(census, []string{"update", "t-1", "--acceptance", strings.Repeat("a", 500)})
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if proj.After <= proj.Before {
			t.Errorf("adding a 500-byte acceptance_criteria field must grow the line: before=%d after=%d", proj.Before, proj.After)
		}
	})
	t.Run("close -r", func(t *testing.T) {
		proj, ok, err := ProjectBdWrite(census, []string{"close", "t-1", "-r", strings.Repeat("r", 500)})
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if proj.After <= proj.Before {
			t.Errorf("a 500-byte close reason must grow the line: before=%d after=%d", proj.Before, proj.After)
		}
	})
	t.Run("create with description", func(t *testing.T) {
		proj, ok, err := ProjectBdWrite(census, []string{"create", "a new title", "-d", "a description"})
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if proj.ID != "" || proj.Before != 0 {
			t.Errorf("a create has no existing line: want ID=\"\" Before=0, got ID=%q Before=%d", proj.ID, proj.Before)
		}
		if proj.After == 0 {
			t.Errorf("After must be the projected row's length, got 0")
		}
	})
	t.Run("create -f is a different shape (multi-issue)", func(t *testing.T) {
		_, ok, err := ProjectBdWrite(census, []string{"create", "-f", "issues.md"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if ok {
			t.Error("create -f creates MULTIPLE issues from a markdown file — must not be judged as one new row")
		}
	})
	t.Run("--actor sets the comment author when none is given", func(t *testing.T) {
		proj, ok, err := ProjectBdWrite(census, []string{"--actor", "tester", "comments", "add", "t-1", "hi"})
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if proj.ID != "t-1" {
			t.Errorf("ID = %q, want t-1 (the global --actor must still be skipped to find the verb)", proj.ID)
		}
	})
}

// CensusLongLines over a small hand-written census: the short rows pass,
// the long one is named by line number, id and byte count, and a line
// longer than the reader's own 4 KB buffer (the ErrBufferFull path
// censusHasRecord's idiom exists for) is still measured whole.
func TestQACensusLongLinesNamesTheOversizedLine(t *testing.T) {
	t.Parallel()
	long := bdRow("t-2", strings.Repeat("x", 10000))
	census := writeCensus(t, bdRow("t-1", "small"), long, bdRow("t-3", "small too"))

	got, err := CensusLongLines(census, 1000)
	if err != nil {
		t.Fatalf("CensusLongLines: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d long lines, want 1: %+v", len(got), got)
	}
	want := LongLine{Line: 2, ID: "t-2", Bytes: len(long)}
	if got[0] != want {
		t.Errorf("got %+v, want %+v", got[0], want)
	}
}

func TestQACensusLongLinesOverNothing(t *testing.T) {
	t.Parallel()
	census := writeCensus(t, bdRow("t-1", "small"), bdRow("t-2", "also small"))
	got, err := CensusLongLines(census, 1000)
	if err != nil {
		t.Fatalf("CensusLongLines: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want no lines over the threshold", got)
	}
}

// ─── D3: the write-time refusal, through Bd.Comment ──────────────────────────

// The offline analogue of the ADR's live verification item 4: a write that
// would cross the cliff is refused before bd ever runs.
func TestQACommentRefusesACrossingWrite(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	// A row sized so a short comment's envelope pushes it past
	// BdLineLimit-BdLineHeadroom (63488) but a tiny row stays well clear.
	big := bdRow("t-2", strings.Repeat("x", BdLineLimit-BdLineHeadroom-200))
	write(t, filepath.Join(dir, beadsDirName, beadsJSONL), bdRow("t-1", "small")+big)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	err := bd.Comment(dir, "t-2", "a short comment that still tips it over", "tester")
	if err == nil {
		t.Fatal("want a refusal, got nil")
	}
	if !IsBdLineWouldCross(err) {
		t.Fatalf("want a typed BdLineWouldCrossError, got %T: %v", err, err)
	}
	if strings.Contains(bdCalls(t, fake), "comments add") {
		t.Errorf("bd must not have been exec'd: bd-calls.log = %q", bdCalls(t, fake))
	}

	// The control: the same comment on the SMALL row is not refused, and
	// does reach the fake bd.
	if err := bd.Comment(dir, "t-1", "fine", "tester"); err != nil {
		t.Errorf("a comment that does not cross the cliff must not be refused: %v", err)
	}
	if !strings.Contains(bdCalls(t, fake), "comments add t-1") {
		t.Errorf("the unrefused comment must have reached bd: %q", bdCalls(t, fake))
	}
}

// ─── D4: the read-time report, through Bd.Ready ──────────────────────────────

// The offline analogue of the ADR's live verification item 5 and its own
// "Offline" pin: a failed bd call over a census carrying a line past
// BdLineLimit is reported as BdStoreLineTooLongError, naming the line, its
// id and its byte count — never matched against bd's own wording
// (ADR 0056, ADR 0072 D4).
func TestQABdReadyReportsLineTooLong(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	long := bdRow("t-2", strings.Repeat("x", 70000))
	write(t, filepath.Join(dir, beadsDirName, beadsJSONL), bdRow("t-1", "small")+long)
	// The fake's generic "the scan failed" marker (herdr_test.go fakeBd,
	// `ready` case) — bd's own real failure here is "token too long", but
	// D4 must never need bd's wording to agree (ADR 0056).
	write(t, filepath.Join(dir, "fake-ready-fail"), "")
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	_, err := bd.Ready(dir, "")
	if err == nil {
		t.Fatal("want a refusal, got nil")
	}
	if !IsBdStoreLineTooLong(err) {
		t.Fatalf("want a typed BdStoreLineTooLongError, got %T: %v", err, err)
	}
	var le *BdStoreLineTooLongError
	if !errors.As(err, &le) {
		t.Fatalf("errors.As must agree with IsBdStoreLineTooLong: %T: %v", err, err)
	}
	if len(le.Lines) != 1 {
		t.Fatalf("Lines = %+v, want exactly the one oversized row", le.Lines)
	}
	got := le.Lines[0]
	want := LongLine{Line: 2, ID: "t-2", Bytes: len(long)}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !strings.Contains(err.Error(), "t-2") || !strings.Contains(err.Error(), "ADR 0072") {
		t.Errorf("the error must name the line's id and cite ADR 0072: %v", err)
	}
	if strings.Contains(err.Error(), "token too long") {
		t.Errorf("the error must never echo bd's own wording (ADR 0056): %v", err)
	}
}
