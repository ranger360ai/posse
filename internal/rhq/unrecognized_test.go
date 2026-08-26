package rhq

// ranger-base-3j8, the part argv could not retire: when a launch does not
// become promptable, the failure line must say what herdr was looking at.
//
// Three different screens on this bead — a consent banner, a version
// splash, and a pane whose OSC chrome had not been emitted yet — produced
// the identical sentence "herdr never saw a screen it recognizes there,
// only idle". Two of them needed opposite fixes, and telling them apart
// cost a hand-launch and a peek every time. herdr already carries the
// answer in `evaluated_rules`; these pin that dispatch reads it, that the
// block groups by region rather than reprinting one evidence set per rule,
// and that a herdr which does not emit the field leaves the old message
// exactly as it was.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWhatHerdrSawGroupsByRegionAndNamesTheRules(t *testing.T) {
	const raw = `{"state":"idle","matched_rule":null,"visible_idle":false,
	  "fallback_reason":"default_known_agent_idle_fallback","evaluated_rules":[
	   {"id":"osc_title_blocked","matched":false,"region":"osc_title","state":"blocked",
	    "evidence":{"region_bytes":0,"region_preview":""}},
	   {"id":"osc_title_idle","matched":false,"region":"osc_title","state":"idle",
	    "evidence":{"region_bytes":0,"region_preview":""}},
	   {"id":"osc_title_working","matched":false,"region":"osc_title","state":"working",
	    "evidence":{"region_bytes":0,"region_preview":""}},
	   {"id":"osc_progress_idle","matched":false,"region":"osc_progress","state":"idle",
	    "evidence":{"region_bytes":0,"region_preview":""}},
	   {"id":"startup_splash","matched":false,"region":"whole_recent","state":"idle",
	    "evidence":{"region_bytes":687,"region_preview":"\n  Help improve Grok   [Opt out] [Opt in]\n  Off by default.\n  ..."}}]}`
	var det AgentDetection
	if err := json.Unmarshal([]byte(raw), &det); err != nil {
		t.Fatal(err)
	}
	got := det.WhatHerdrSaw()
	// One row per region, in herdr's own evaluation order — not one per
	// rule, which is four repeats of the same empty osc_title.
	if n := strings.Count(got, "\n      "); n != 3 {
		t.Errorf("want 3 region rows (osc_title, osc_progress, whole_recent), got %d:%s", n, got)
	}
	for _, want := range []string{
		"evaluated 5 rules there and matched none",
		`osc_title `, // the region, padded
		`0 bytes  ""`,
		"osc_title_blocked, osc_title_idle, osc_title_working",
		"687 bytes",
		"Help improve Grok",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the block must carry %q:%s", want, got)
		}
	}
	// An empty region and a full one are the two diagnoses this exists to
	// separate: nothing drawn yet vs. parked on a screen posse cannot name.
	title := line(t, got, "osc_title ")
	splash := line(t, got, "whole_recent")
	if strings.Contains(title, "Help improve") || !strings.Contains(splash, "Help improve") {
		t.Errorf("evidence landed on the wrong region:\n%s\n%s", title, splash)
	}
	// The preview is flattened and bounded: a pass log stays a pass log.
	if strings.Contains(splash, "\n  Help") {
		t.Errorf("a region preview's newlines must not break the row:%s", got)
	}
	if len([]rune(splash)) > 200 {
		t.Errorf("region row is %d runes, too wide for a pass log:\n%s", len([]rune(splash)), splash)
	}
}

// An older herdr does not emit `evaluated_rules`. The failure message must
// stand on its own then — the block is an addition, never the sentence.
func TestWhatHerdrSawIsEmptyWithoutHerdrsWorking(t *testing.T) {
	var det AgentDetection
	if err := json.Unmarshal([]byte(`{"state":"idle","matched_rule":null,"visible_idle":false,
	  "fallback_reason":"default_known_agent_idle_fallback"}`), &det); err != nil {
		t.Fatal(err)
	}
	if got := det.WhatHerdrSaw(); got != "" {
		t.Errorf("no working to report must render nothing, got %q", got)
	}
}

// A row is truncated by runes, not bytes: grok's chrome is box-drawing
// characters and a byte cut mid-rune prints a replacement glyph.
func TestWhatHerdrSawTruncatesOnRuneBoundaries(t *testing.T) {
	var det AgentDetection
	det.EvaluatedRules = []EvaluatedRule{{ID: "r", Region: "whole_recent"}}
	det.EvaluatedRules[0].Evidence.RegionBytes = 4000
	det.EvaluatedRules[0].Evidence.RegionPreview = strings.Repeat("╰─╯", 200)
	got := det.WhatHerdrSaw()
	if strings.ContainsRune(got, '\uFFFD') {
		t.Errorf("truncation split a rune:%s", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("a truncated preview must say so:%s", got)
	}
}

// Border art is not a preview. Measured on the wide grok splash: 70 of the
// first 72 characters were one repeated box rule, so the row said nothing
// at all. Collapsing runs spends the budget on the text between the borders
// — which on that capture is `╰── Grok 4.6 (high) ─╯`, this bead's own tell
// (a session that never shows `· auto` is never recognized).
func TestWhatHerdrSawSpendsThePreviewOnTextNotBorders(t *testing.T) {
	var det AgentDetection
	det.EvaluatedRules = []EvaluatedRule{{ID: "prompt_hints_idle", Region: "bottom_non_empty_lines(2)"}}
	det.EvaluatedRules[0].Evidence.RegionBytes = 601
	det.EvaluatedRules[0].Evidence.RegionPreview = "╰" + strings.Repeat("─", 60) + " Grok 4.6 (high) ─╯" + "\n\n  [stable]"
	got := det.WhatHerdrSaw()
	if !strings.Contains(got, "Grok 4.6 (high)") || !strings.Contains(got, "[stable]") {
		t.Errorf("the text between the borders is the diagnosis and must survive:%s", got)
	}
	if strings.Contains(got, strings.Repeat("─", 4)) {
		t.Errorf("a run of border rule must be collapsed:%s", got)
	}
	// A run of letters or digits is content, not art, and is left alone.
	if collapseRules("aaaaa 11111") != "aaaaa 11111" {
		t.Errorf("collapseRules ate content: %q", collapseRules("aaaaa 11111"))
	}
}

// THE CONTRACT WITH THE REAL BINARY. Field names decoded from a hand-written
// fixture prove nothing about herdr; this explains a capture that really is
// this bead's failure — the production-width boxed grok splash, which
// resolves to fallback idle with no matched rule (ranger-base-z6n) — and
// checks that the block posse would have printed names its regions.
func TestQAWhatHerdrSawOnARealFallbackCapture(t *testing.T) {
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	file := filepath.Join("testdata", "grok-startup-splash-wide-boxed.txt")
	if _, err := os.Stat(file); err != nil {
		t.Skip("capture not present: " + err.Error())
	}
	out, err := exec.Command("herdr", "agent", "explain", "--file", file, "--agent", "grok", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("herdr agent explain: %v\n%s", err, out)
	}
	var det AgentDetection
	if err := json.Unmarshal(out, &det); err != nil {
		t.Fatalf("detection json: %v\n%s", err, out)
	}
	if det.Seen() {
		t.Skipf("this capture is named now (rule %q) — pick another fallback capture", det.Rule.ID)
	}
	got := det.WhatHerdrSaw()
	if got == "" {
		t.Fatalf("a real herdr's `evaluated_rules` did not decode — the field names moved:\n%s", out)
	}
	// The two regions the diagnosis on this bead turned on: grok's OSC
	// title (empty = the CLI has not spoken) and the screen text (full =
	// it is up, on something posse cannot name).
	for _, want := range []string{"osc_title", "whole_recent", "matched none"} {
		if !strings.Contains(got, want) {
			t.Errorf("the block must name %q:%s", want, got)
		}
	}
	if !strings.Contains(line(t, got, "osc_title "), `0 bytes  ""`) {
		t.Errorf("grok emits no OSC title on this screen — the block must show that:%s", got)
	}
	t.Logf("what a failed launch would now print:%s", got)
}

// Typed path (awaitSettled): the loud refusal carries herdr's working.
func TestPromptableFailureSaysWhatHerdrWasLookingAt(t *testing.T) {
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 200 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644) // guess forever
	os.WriteFile(filepath.Join(fake, "explain-rules"), []byte(fakeEvaluatedRules), 0o644)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "never saw a screen it recognizes") {
		t.Fatalf("the old sentence must survive:\n%s", out)
	}
	for _, want := range []string{"What it was reading:", "osc_title", "whole_recent", "Grok Build"} {
		if !strings.Contains(out, want) {
			t.Errorf("a promptability failure must say what herdr read (%q):\n%s", want, out)
		}
	}
}

// Argv path (awaitDelivered): same working, on the line that reports a
// delivered prompt whose screen was never recognized.
func TestArgvUnrecognizedScreenSaysWhatHerdrWasLookingAt(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 150 * time.Millisecond
	argvPersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644)
	os.WriteFile(filepath.Join(fake, "explain-rules"), []byte(fakeEvaluatedRules), 0o644)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "herdr never recognized a screen") {
		t.Fatalf("the old sentence must survive:\n%s", out)
	}
	for _, want := range []string{"What it was reading:", "osc_title", "Grok Build"} {
		if !strings.Contains(out, want) {
			t.Errorf("an undelivered-looking argv launch must say what herdr read (%q):\n%s", want, out)
		}
	}
}

// A launch that FAILS for any other reason must not grow the block: the
// working is only meaningful when nothing matched.
func TestSeenScreenAddsNoDiagnosticBlock(t *testing.T) {
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	os.WriteFile(filepath.Join(fake, "explain-rules"), []byte(fakeEvaluatedRules), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 {
		t.Fatalf("a seen screen still dispatches, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "What it was reading:") {
		t.Errorf("herdr's working belongs to failures only:\n%s", out)
	}
}

// fakeEvaluatedRules is the shape a real `agent explain --json` carries,
// trimmed to the two regions that told the three screens on this bead
// apart: an empty osc_title and a screen full of grok's splash.
const fakeEvaluatedRules = `[
  {"id":"osc_title_idle","matched":false,"region":"osc_title","state":"idle",
   "evidence":{"region_bytes":0,"region_preview":""}},
  {"id":"startup_splash","matched":false,"region":"whole_recent","state":"idle",
   "evidence":{"region_bytes":891,"region_preview":"  Grok Build 1.0.5\n  Grok 4.6 is here!\n  New worktree   ctrl+w\n  ..."}}]`

// line returns the one line of s containing want, for assertions that care
// which row a fact landed on.
func line(t *testing.T, s, want string) string {
	t.Helper()
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, want) {
			return l
		}
	}
	t.Fatalf("no line containing %q in:%s", want, s)
	return ""
}
