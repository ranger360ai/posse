package posse

// QA pins for ranger-base-a0vdz / ADR 0072 B2 — RunGatesBdLine itself,
// offline: no shell, no subprocess, the same census fixtures bdline_qa_test.go
// (B1) already uses for ProjectBdWrite. The shell-wiring half (the shim
// actually calling a real posse, propagating its exit code, never calling
// posse for a read) is bdlineshim_test.go.

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestQARunGatesBdLineUsageNeedsDoubleDash(t *testing.T) {
	t.Parallel()
	if _, err := RunGatesBdLine([]string{"comments", "add", "t-1", "hi"}, &bytes.Buffer{}); err == nil {
		t.Fatalf("missing -- must be an error, not a silent pass")
	}
}

func TestQARunGatesBdLineNoCensusIsSilent(t *testing.T) {
	t.Setenv("BEADS_DIR", filepath.Join(t.TempDir(), "no-such-store"))
	t.Setenv("RHQ_PERSONA", "security")
	var errb bytes.Buffer
	refused, err := RunGatesBdLine([]string{"--", "comments", "add", "t-1", "hi"}, &errb)
	if err != nil || refused {
		t.Fatalf("no census: refused=%v err=%v, want false/nil", refused, err)
	}
	if errb.Len() != 0 {
		t.Errorf("no census must print nothing: %q", errb.String())
	}
}

func TestQARunGatesBdLineRefusesACrossingComment(t *testing.T) {
	census := writeCensus(t, bdRow("t-1", "small"), bdRow("t-2", "small too"))
	home := filepath.Dir(census)
	t.Setenv("BEADS_DIR", home)
	t.Setenv("RHQ_PERSONA", "security")
	logPath := filepath.Join(t.TempDir(), "refusals.log")
	t.Setenv("RHQ_GATE_LOG", logPath)

	text := strings.Repeat("x", 70000)
	var errb bytes.Buffer
	refused, err := RunGatesBdLine([]string{"--", "comments", "add", "t-1", text}, &errb)
	if err != nil || !refused {
		t.Fatalf("a 70000-byte comment: refused=%v err=%v, want true/nil", refused, err)
	}
	out := errb.String()
	if !strings.Contains(out, "refused by posse gate:") || !strings.Contains(out, "t-1") || !strings.Contains(out, "ADR 0072") {
		t.Errorf("refusal text missing the D2 shape:\n%s", out)
	}
	if !strings.Contains(out, "comment the pointer") {
		t.Errorf("refusal is missing its remedy:\n%s", out)
	}
	if strings.Contains(out, strings.Repeat("x", 50)) {
		t.Errorf("refusal echoed the comment text")
	}
	log, rerr := os.ReadFile(logPath)
	if rerr != nil {
		t.Fatalf("refusals.log: %v", rerr)
	}
	if !strings.Contains(string(log), "bd line cliff [bd shim] t-1") || !strings.Contains(string(log), "session security") {
		t.Errorf("refusals.log missing the D2 line: %q", string(log))
	}
}

func TestQARunGatesBdLinePassesASmallComment(t *testing.T) {
	census := writeCensus(t, bdRow("t-1", "small"))
	t.Setenv("BEADS_DIR", filepath.Dir(census))
	t.Setenv("RHQ_PERSONA", "security")
	var errb bytes.Buffer
	refused, err := RunGatesBdLine([]string{"--", "comments", "add", "t-1", "hi"}, &errb)
	if err != nil || refused {
		t.Fatalf("a small comment: refused=%v err=%v, want false/nil", refused, err)
	}
	if errb.Len() != 0 {
		t.Errorf("a passing call must print nothing: %q", errb.String())
	}
}

// create has no census line yet (Projection.ID == ""); the refusal text and
// log line both have to carry a sentence that still makes sense with no id.
func TestQARunGatesBdLineRefusesAnOversizedCreate(t *testing.T) {
	census := writeCensus(t, bdRow("t-1", "small"))
	home := filepath.Dir(census)
	t.Setenv("BEADS_DIR", home)
	t.Setenv("RHQ_PERSONA", "security")
	logPath := filepath.Join(t.TempDir(), "refusals.log")
	t.Setenv("RHQ_GATE_LOG", logPath)

	desc := strings.Repeat("y", 70000)
	var errb bytes.Buffer
	refused, err := RunGatesBdLine([]string{"--", "create", "a title", "-d", desc}, &errb)
	if err != nil || !refused {
		t.Fatalf("an oversized create: refused=%v err=%v, want true/nil", refused, err)
	}
	out := errb.String()
	if !strings.Contains(out, "refused by posse gate:") || !strings.Contains(out, "create") || !strings.Contains(out, "ADR 0072") {
		t.Errorf("refusal text missing the D2 create shape:\n%s", out)
	}
	log, rerr := os.ReadFile(logPath)
	if rerr != nil {
		t.Fatalf("refusals.log: %v", rerr)
	}
	if !strings.Contains(string(log), "bd line cliff [bd shim] (create)") {
		t.Errorf("refusals.log missing the no-id D2 line: %q", string(log))
	}
}

func TestQAAppendBdLineGateLogIsANoopWithNoLogSet(t *testing.T) {
	t.Setenv("RHQ_GATE_LOG", "")
	// Must not panic or create anything — there is nowhere to write.
	appendBdLineGateLog(Projection{ID: "t-1", After: 70000})
}

func TestQABdLineCensusHomePrefersBEADS_DIR(t *testing.T) {
	t.Setenv("BEADS_DIR", "/explicit/store")
	if got := bdLineCensusHome(); got != "/explicit/store" {
		t.Errorf("bdLineCensusHome() = %q, want the explicit BEADS_DIR", got)
	}
	// No chdir: beadsHome(".") is cwd-relative, and this package's suite
	// runs other tests in parallel that must not see this one's cwd move
	// (the ranger-base-i7fa rule, applied here the same way runningPosse's
	// own test comment applies it to PATH). Comparing against beadsHome(".")
	// directly is the same answer without touching anything process-global.
	t.Setenv("BEADS_DIR", "")
	if got, want := bdLineCensusHome(), beadsHome("."); got != want {
		t.Errorf("bdLineCensusHome() with no BEADS_DIR = %q, want beadsHome(\".\") = %q", got, want)
	}
}

// Sanity on the byte math the two integration-level fixtures lean on, so a
// future BdLineHeadroom change fails HERE with a clear number rather than
// as a baffling flip in TestRenderedShimRefusesAndPassesTheLineCliff.
func TestQABdLineCliffThresholdIsWhatTheFixturesAssume(t *testing.T) {
	t.Parallel()
	const want = 65536 - 2048
	if got := BdLineLimit - BdLineHeadroom; got != want {
		t.Fatalf("BdLineLimit-BdLineHeadroom = %s, want %s", strconv.Itoa(got), strconv.Itoa(want))
	}
}
