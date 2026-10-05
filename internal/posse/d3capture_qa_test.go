//go:build posse_arm3

package posse

// QA pins for ranger-base-76gc4 — the D3 readings-log record carries the
// screen, not a 243-character preview of the top of it (ADR 0066 D3 as
// amended, measured in docs/notes.d/ranger-base-qk9tr.md §5 and
// docs/notes.d/ranger-base-76gc4.md).
//
// THE BUG THESE ARE ABOUT IS AN INPUT, NOT A READER. `agent explain` emits a
// PREVIEW of each region capped at 243 characters, and ReadingEvidenceOf
// carries exactly that. Every top-anchored region starts at the top of the
// screen; on codex the top of the screen is 15-16 rows of ASCII logo. So for
// 9 of the 15 labelled screens posse owns a capture of, the screen's own
// heading is outside every preview the record holds, and a D3 record for the
// screens most likely to PRODUCE one reproduces nothing. Over the full
// capture a heading reader names 11 of 11 residue cases on the three
// measured incident classes; over the previews, 5 of 11.
//
// Every arm below is built so it would have been RED before the capture
// landed and goes red again if the capture is narrowed back toward a
// preview: the heading each one looks for sits past 243 characters by
// construction, and arm 1 fails outright rather than passing if that stops
// being true.
//
// NOT PINNED HERE, deliberately: the ceiling. redactRegions is keyed on no
// region name at all, and TestQAReadingsLogRedactsCeilingContentBeforeWrite
// already measures it over the regions a record carries — a second copy
// keyed on this one's name would be the same mechanism asserted twice.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// d3Heading is the line a reader of this record has to be able to find, and
// d3Logo is what sits above it. Both are shaped like the real thing: codex's
// sign-in screen is 15-16 rows of box art before it says what it is
// (etc/herdr/agent-detection/testdata/codex/blocked-signin-tall-logo.txt).
const (
	d3Heading = "Sign in with ChatGPT"
	d3Logo    = "" +
		"  ╭──────────────────────────────────────────────────────────╮\n" +
		"  │   ████  ████  ████   ████  ██  ██                        │\n" +
		"  │  ██    ██  ██ ██  ██ ██    ██ ██                         │\n" +
		"  │  ██    ██  ██ ██  ██ ████  ████                          │\n" +
		"  │  ██    ██  ██ ██  ██ ██    ██ ██                         │\n" +
		"  │   ████  ████  ████   ████  ██  ██                        │\n" +
		"  │                                                          │\n" +
		"  │                                                          │\n" +
		"  ╰──────────────────────────────────────────────────────────╯\n" +
		"\n\n\n\n\n\n"
)

// d3Screen is the whole pane: logo art, then the heading, then the chrome a
// rule would key on.
func d3Screen() string {
	return d3Logo + d3Heading + "\n\n  > 1. Sign in with ChatGPT\n    2. Provide your own API key\n\n  Press enter to continue"
}

// d3Preview is what herdr emits for a top-anchored region of that screen,
// and the whole of what a record carried before this bead: the first 243
// characters, which is logo art.
func d3Preview() string {
	if r := []rune(d3Screen()); len(r) > 243 {
		return string(r[:243])
	}
	return d3Screen()
}

// d3Session stands a session up far enough for AwaitPromptable to refuse and
// for LogReading to resolve a tree to write the record under, and returns
// the log path. The runtime's declared patience is the lever a slow CLI uses
// in production (promptReadySession's note), so a refusal costs a fraction
// of a second instead of the claude-shaped 45.
func d3Session(t *testing.T, b *HerdrBackend, name string) string {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "fastcli", "command: fastcli --sys {file}\nstartup_wait: 400ms\n")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	tree, err := b.App.EnsureSessionTree(repo, name, nil)
	if err != nil || tree == nil {
		t.Fatalf("EnsureSessionTree: %v (tree=%v)", err, tree)
	}
	if err := b.writeMeta(&HerdrMeta{
		Name: name, Workspace: "w1", Pane: "w1:p1", Agent: "ranger",
		Runtime: "fastcli", Dir: tree.Path, Repo: repo, Branch: tree.Branch,
	}); err != nil {
		t.Fatalf("writeMeta: %v", err)
	}
	log, err := b.App.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	return log
}

// d3Unrecognized arms the fake the way the incident arms the real thing:
// herdr guesses forever (matched_rule null, the idle fallback), every region
// it evaluated carries a truncated preview of the top of the screen, and the
// pane itself is showing d3Screen.
func d3Unrecognized(t *testing.T, fake string) {
	t.Helper()
	write(t, filepath.Join(fake, "explain-fallback"), "")
	preview, err := json.Marshal(d3Preview())
	if err != nil {
		t.Fatal(err)
	}
	bytes := strconv.Itoa(len(d3Screen()))
	var rules []string
	for _, reg := range []string{"top_non_empty_lines_20", "top_non_empty_lines_25"} {
		rules = append(rules, `{"id":"rule_`+reg+`","matched":false,"region":"`+reg+`","state":"idle",`+
			`"evidence":{"region_bytes":`+bytes+`,"region_preview":`+string(preview)+`}}`)
	}
	write(t, filepath.Join(fake, "explain-rules"), "["+strings.Join(rules, ",")+"]")
	write(t, filepath.Join(fake, "pane-text", "w1_p1"), d3Screen())
}

// d3OneRecord reads the single reading a refusal wrote.
func d3OneRecord(t *testing.T, log string) Reading {
	t.Helper()
	rs, err := ReadReadings(log)
	if err != nil {
		t.Fatalf("ReadReadings(%s): %v", log, err)
	}
	if len(rs) != 1 {
		t.Fatalf("the refusal wrote %d readings, want 1", len(rs))
	}
	return rs[0]
}

// THE BEAD, arm 1. A D3 refusal whose previews are truncated writes a record
// that carries the screen's own heading — which no region in that record
// held before, and which is the only thing in the bytes that names the
// screen.
func TestQAD3RecordCarriesTheScreenAndNotOnlyItsTruncatedPreviews(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	log := d3Session(t, b, "unseen")
	d3Unrecognized(t, fake)

	// The fixture's own precondition. If the heading ever drifts inside the
	// cap this pin measures nothing, so it says so rather than passing.
	if strings.Contains(d3Preview(), d3Heading) {
		t.Fatalf("the fixture's heading is inside the 243-character preview — this pin can no longer tell a capture from a preview:\n%s", d3Preview())
	}

	if _, _, err := b.AwaitPromptable("unseen", "w1:p1"); err == nil {
		t.Fatal("a screen herdr only guessed at is not promptable")
	}

	r := d3OneRecord(t, log)
	if r.Decision != DecisionUnknownScreen || r.Consequence != ConsequenceRefusal {
		t.Fatalf("want a D3 refusal, got %s/%s", r.Decision, r.Consequence)
	}
	shot, ok := r.ReadingRegionOf(PaneCaptureRegion)
	if !ok {
		var had []string
		for _, reg := range r.Herdr.Regions {
			had = append(had, reg.Name)
		}
		t.Fatalf("the D3 record carries no %q region — it holds only %v, every one of them a preview of the top of the screen", PaneCaptureRegion, had)
	}
	if !strings.Contains(shot.Text, d3Heading) {
		t.Errorf("the capture does not carry the screen's own heading %q, which is the one thing in the bytes that names the screen:\n%s", d3Heading, shot.Text)
	}
	if shot.Truncated {
		t.Error("the capture is the whole screen and must not be marked truncated — Truncated is how a replay knows the bytes are not the bytes")
	}
	if shot.Bytes != len(shot.Text) {
		t.Errorf("Bytes=%d over %d bytes of text: on a capture the two are one read, and a census comparing them would read a truncation that did not happen", shot.Bytes, len(shot.Text))
	}

	// The CONTROL, and it is what makes the arm above a measurement rather
	// than a tautology: herdr's own regions are still previews, still
	// truncated, and still hold no heading. If the capture had quietly
	// replaced them the record would have LOST herdr's working, which is
	// the evidence rangerhq-7ia and ranger-base-0sa5a were closed from.
	previews := 0
	for _, reg := range r.Herdr.Regions {
		if reg.Name == PaneCaptureRegion {
			continue
		}
		previews++
		if !reg.Truncated {
			t.Errorf("region %q lost its Truncated mark — a replay over a preview that claims to be whole proves nothing", reg.Name)
		}
		if strings.Contains(reg.Text, d3Heading) {
			t.Errorf("region %q holds the heading, so this fixture is not the incident shape", reg.Name)
		}
	}
	if previews != 2 {
		t.Errorf("herdr's own working shrank to %d regions, want 2 — the capture is added to the evidence, never instead of it", previews)
	}

	// ONE READ, ON THE REFUSAL PATH, IN THE SHAPE A FIXTURE IS. The bargain
	// logPromptHandBack already strikes: D3 is rare and already costs a
	// refused launch, so one more herdr call buys the only bytes there are.
	// The argv is pinned because the SOURCE is the whole point — MEASURED
	// 2026-10-04, `--source recent` (PaneRead's default) returned 80 lines
	// of scrollback where `--source detection` returned the 50-row screen,
	// and only the latter is what `herdr agent explain --file` reads back.
	ran := calls(t, fake)
	if n := strings.Count(ran, "pane read"); n != 1 {
		t.Errorf("the refusal spent %d pane reads, want exactly 1:\n%s", n, ran)
	}
	if !strings.Contains(ran, "pane read w1:p1 --source detection --format text") {
		t.Errorf("the capture must be read in the shape a fixture is (`--source detection`, no tail) or it is not a candidate testdata/ capture:\n%s", ran)
	}
}

// Arm 2, the route with no screen (ADR 0061 D3.3). A reported pane is one
// herdr does not address — there is no screen it failed to recognize — so
// the record carries the reading and no capture, and posse spends no pane
// read finding that out.
func TestQAD3ReportedPaneSpendsNoCaptureItHasNoScreenFor(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	log := d3Session(t, b, "plugged")
	reportedPane(t, fake, "plugged", "idle")
	atShell(t, fake)

	if _, _, err := b.AwaitPromptable("plugged", "w1:p1"); err == nil {
		t.Fatal("a stale reported label is not promptable")
	}
	r := d3OneRecord(t, log)
	if r.Decision != DecisionUnknownScreen {
		t.Fatalf("want a D3 record, got %s", r.Decision)
	}
	if _, ok := r.ReadingRegionOf(PaneCaptureRegion); ok {
		t.Error("a reported pane got a pane capture — herdr addresses no screen there, so those bytes are posse reading past its own detection")
	}
	if ran := calls(t, fake); strings.Contains(ran, "pane read") {
		t.Errorf("the reported route spent a pane read it has no use for:\n%s", ran)
	}
}

// Arm 3, the concession. The log is best effort, so a capture that cannot be
// taken costs the capture and nothing else: the record is still written, it
// still carries herdr's working, and the refusal still refuses.
func TestQAD3ACaptureThatCannotBeTakenCostsOnlyTheCapture(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	log := d3Session(t, b, "unreadable")
	d3Unrecognized(t, fake)
	// pane-text is served before pane-read-error in the fake, so the text
	// would come out rather than the refusal — remove it and the read fails
	// the way a vanished pane fails.
	if err := os.Remove(filepath.Join(fake, "pane-text", "w1_p1")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(fake, "pane-read-error"), "")

	if _, _, err := b.AwaitPromptable("unreadable", "w1:p1"); err == nil {
		t.Fatal("a screen herdr only guessed at is not promptable, capture or no capture")
	}
	r := d3OneRecord(t, log)
	if _, ok := r.ReadingRegionOf(PaneCaptureRegion); ok {
		t.Error("a pane read that errored still produced a capture region")
	}
	if len(r.Herdr.Regions) != 2 {
		t.Errorf("the record kept %d of herdr's regions, want 2 — a failed capture must not cost the evidence posse already had", len(r.Herdr.Regions))
	}
}

// Arm 4, the single write — the claim AppendReading's comment used to make
// about a PAGE, re-measured because this bead broke it. A record with a
// capture is past 4,096 bytes as a matter of course (six previews plus the
// tallest fixture posse owns came to 6,297 bytes, MEASURED 2026-10-04), so
// the comment had to be restated or withdrawn. What holds a line together is
// that there is ONE write(2), not that it is small: six and sixteen
// concurrent appenders tore nothing at any size up to 131,072 bytes on this
// box the same day (docs/notes.d/ranger-base-76gc4.md §3).
//
// This pin holds the half a unit test can hold — one record is one line, at
// any size the shape can produce, and nothing in the writer splits it.
func TestQAD3AReadingWithAPaneCaptureIsStillOneLine(t *testing.T) {
	t.Parallel()
	a, _, tree := rlTree(t)
	log, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	// A 60x200 screen of box-drawing glyphs: three bytes a column, and the
	// worst shape a `--source detection` read can hand this function.
	wide := strings.Repeat(strings.Repeat("─", 200)+"\n", 60)
	for i, text := range []string{d3Screen(), wide} {
		r := Reading{
			At:          time.Date(2026, 10, 4, 12, i, 0, 0, time.UTC),
			Decision:    DecisionUnknownScreen,
			Consequence: ConsequenceRefusal,
			Verdict:     "unrecognized screen",
			Rule:        RulePromptReady,
		}
		for j := 0; j < 6; j++ {
			r.Herdr.Regions = append(r.Herdr.Regions, ReadingRegion{
				Name: "region_" + strconv.Itoa(j), Bytes: 4000, Text: strings.Repeat("x", 243), Truncated: true,
			})
		}
		r.Herdr.Regions = append(r.Herdr.Regions, ReadingRegion{Name: PaneCaptureRegion, Bytes: len(text), Text: text})
		if err := a.AppendReading(tree.Path, r); err != nil {
			t.Fatalf("AppendReading over %d bytes of capture: %v", len(text), err)
		}
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("two records wrote %d lines — a record that is not one line is a record a census drops", len(lines))
	}
	if len(lines[1]) <= 4096 {
		t.Errorf("the large arm came to %d bytes, which is inside a page — this pin no longer measures what it was written for", len(lines[1]))
	}
	rs, err := ReadReadings(log)
	if err != nil || len(rs) != 2 {
		t.Fatalf("ReadReadings over a %d-byte record: %d records, %v", len(lines[1]), len(rs), err)
	}
	shot, ok := rs[1].ReadingRegionOf(PaneCaptureRegion)
	if !ok || shot.Text != wide {
		t.Errorf("the large capture did not round-trip whole: ok=%v, %d of %d bytes", ok, len(shot.Text), len(wide))
	}
}

// Arm 5, the dry run. A pass that will not WRITE the record must not pay
// for the capture either: the capture is a herdr call, and a Go argument is
// evaluated before the function that would have discarded it, so this rule
// lives in d3Evidence rather than at the two dispatch call sites.
//
// Pinned on d3Evidence directly, and the control is the live pass: an arm
// that only asserted the dry run would be green over a capture that never
// happened at all.
func TestQAD3ADryRunPassSpendsNoCaptureItWillNotWrite(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	write(t, filepath.Join(fake, "pane-text", "w1_p1"), d3Screen())
	det := AgentDetection{State: "idle"}

	d.DryRun = true
	if ev := d.d3Evidence("w1:p1", det); len(ev.Regions) != 0 {
		t.Errorf("a dry run built %d regions, want 0 — the record is not written, so the read that fills it must not be made", len(ev.Regions))
	}
	if ran := calls(t, fake); strings.Contains(ran, "pane read") {
		t.Errorf("a dry-run pass forked a pane read for a record it then refuses to write:\n%s", ran)
	}

	d.DryRun = false
	ev := d.d3Evidence("w1:p1", det)
	shot, ok := Reading{Herdr: ev}.ReadingRegionOf(PaneCaptureRegion)
	if !ok || !strings.Contains(shot.Text, d3Heading) {
		t.Fatalf("a live pass took no capture (ok=%v) — the dry-run arm above measures nothing if this does not read", ok)
	}
}
