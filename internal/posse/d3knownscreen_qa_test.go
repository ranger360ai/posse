//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-6uokf — the ADR 0066 D3 report: the unknown-screen
// failure line and the D3 record name the known screen(s) on the pane
// (knownscreen.go; the ruling is ranger-base-gy3io option B, the measurement
// is docs/notes.d/ranger-base-qk9tr.md).
//
// WHAT HAS TO HOLD, and why each arm is shaped the way it is:
//
//  1. THE TABLE IS NOT A SECOND COPY. `scripts/d3-reader-eval.py`'s OPTIONS
//     is the reference the spike measured; `KnownScreens` is what ships. The
//     two would be a pair kept in sync by hand, which is the thing the
//     tree-wide-pin register exists to refuse — so this arm runs the script
//     as a module and compares them row for row, markers included. It does
//     NOT reimplement either: the script is imported, not parsed.
//  2. THE READER IS EXACT ON EVERY FIXTURE posse owns, scored against the
//     script's own EXPECTED table so there is one labelling and not two. It
//     enumerates the fixture directory rather than reading a list, because a
//     sixteenth capture that nobody labelled is the one way this corpus goes
//     quiet — the script only warns about it on stderr and still exits 0.
//     Both idle screens are asserted BY NAME on top of the sweep (ADR 0066
//     D5's bar: zero of the idle fixtures read as a blocked screen), because
//     an empty expected set is the one case a reader that returned nothing
//     at all would also pass.
//  3. SET-VALUED, with no tree and no script: the conjunction needs both its
//     phrases, an alternative needs one of them, an empty marker matches
//     nothing rather than everything, and unrelated text names nothing.
//     This is the arm that fails first when the reader is rewritten.
//  4. THE FAILURE LINE carries the row and never loses herdr's working. The
//     row is the LAST thing in the block and the block's other rows are
//     untouched, which is ADR 0066 D2's "beside the verdict" in the one
//     place an operator reads it.
//  5. THE RECORD carries the set and the REAL census counts it. ADR 0066 D1
//     names an instrument; a pin that reimplemented its arithmetic would be
//     measuring a second copy. Both ends: 1 of 1 over a record whose capture
//     holds a heading, and 0 of 2 over records of the shape every D3 record
//     had before this bead — because a reader whose absence counted as a
//     naming would make every old log a fleet of free diagnoses.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// d3ksScript is the spike's evaluation script, which arms 1 and 2 import for
// its two tables. Reached by the climb the other fixture pins in this package
// use (splashwide_qa_test.go, codexupdatemenu_qa_test.go): `go test
// ./internal/posse` runs with this package as its working directory.
const d3ksScript = "d3-reader-eval.py"

// d3ksDump is the script's OPTIONS and EXPECTED, as the script itself holds
// them.
//
// IMPORTED AND NOT PARSED. The markers are a Python literal with tuples in
// it, and a Go reader of that literal would be a second implementation of
// Python — green against a table it misread. importlib runs the real module,
// so what this compares against is what `d3-reader-eval.py` would actually
// score with. The script has no import-time side effects: `main` is behind
// `if __name__ == "__main__"`, and nothing above it calls herdr or touches a
// file.
type d3ksDump struct {
	Options []struct {
		Name    string     `json:"name"`
		Rules   []string   `json:"rules"`
		Markers [][]string `json:"markers"`
	} `json:"options"`
	Expected map[string][]string `json:"expected"`
}

// d3ksProgram normalizes the script's two tables onto the Go shapes: a bare
// marker string becomes a one-phrase conjunction, which is exactly what
// `keyword_read` does with it (`phrases = m if isinstance(m, tuple) else
// (m,)`).
const d3ksProgram = `
import importlib.util, json, sys
spec = importlib.util.spec_from_file_location("d3readereval", sys.argv[1])
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
opts = [{"name": k, "rules": list(v["rules"]),
         "markers": [list(m) if isinstance(m, tuple) else [m] for m in v["markers"]]}
        for k, v in mod.OPTIONS.items()]
print(json.dumps({"options": opts,
                  "expected": {k: sorted(v) for k, v in mod.EXPECTED.items()}},
                 ensure_ascii=False))
`

func d3ksTables(t *testing.T) d3ksDump {
	t.Helper()
	script := filepath.Join("..", "..", "scripts", d3ksScript)
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s: %v — ADR 0066 D3's reference reader lives there and this arm is the only thing that holds the shipped table to it", script, err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatalf("no python3 on PATH, and %s is python3: %v", script, err)
	}
	out, err := exec.Command("python3", "-c", d3ksProgram, script).Output()
	if err != nil {
		t.Fatalf("python3 -c <dump> %s: %v", script, err)
	}
	var d d3ksDump
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatalf("the dump of %s is not json: %v\n%s", script, err, out)
	}
	if len(d.Options) == 0 || len(d.Expected) == 0 {
		t.Fatalf("%s dumped %d option(s) and %d expected row(s) — the tables this arm compares against are empty, so a clean result is not evidence",
			script, len(d.Options), len(d.Expected))
	}
	return d
}

// Arm 1: the shipped table and the spike script's OPTIONS are one table.
func TestQAKnownScreenTableIsTheSpikeScriptsOptions(t *testing.T) {
	t.Parallel()
	d := d3ksTables(t)

	if len(KnownScreens) != len(d.Options) {
		var ours, theirs []string
		for _, s := range KnownScreens {
			ours = append(ours, s.Name)
		}
		for _, o := range d.Options {
			theirs = append(theirs, o.Name)
		}
		t.Fatalf("KnownScreens holds %d screen(s) and scripts/%s's OPTIONS holds %d — the shipped reader and the reader ADR 0066 D3 was measured with are reading for different screens:\n  shipped: %v\n  script:  %v",
			len(KnownScreens), d3ksScript, len(d.Options), ours, theirs)
	}

	// Row for row and IN ORDER, because knownscreen.go's own comment says
	// the table is in the script's order. A reorder is harmless to the
	// reader and not to a person diffing the two, and the claim is cheaper
	// to hold than to re-earn.
	for i, want := range d.Options {
		got := KnownScreens[i]
		if got.Name != want.Name {
			t.Errorf("row %d: KnownScreens names %q and the script's OPTIONS names %q", i, got.Name, want.Name)
			continue
		}
		if !d3ksSameStrings(got.Rules, want.Rules) {
			t.Errorf("%s: the shipped row names herdr rules %v and the script's names %v — the rule ids are how a row is checked against the manifests and how an evaluation scores this reader on the residue only",
				got.Name, got.Rules, want.Rules)
		}
		if !reflect.DeepEqual(d3ksMarkers(got.Markers), d3ksMarkers(want.Markers)) {
			t.Errorf("%s: the shipped markers are %v and the script's are %v — these two readers would name different screens on the same pane, and only one of them was measured",
				got.Name, got.Markers, want.Markers)
		}
	}
}

// d3ksMarkers normalizes a marker table for comparison the way the reader
// normalizes it for matching: lower-cased, whitespace-collapsed, alternatives
// and conjunctions in their written order. Comparing the raw strings would
// red on a capital that changes nothing and stay green on a double space
// that changes everything.
func d3ksMarkers(in [][]string) [][]string {
	out := make([][]string, 0, len(in))
	for _, marker := range in {
		phrases := make([]string, 0, len(marker))
		for _, p := range marker {
			phrases = append(phrases, normKnownScreen(p))
		}
		out = append(out, phrases)
	}
	return out
}

func d3ksSameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Arm 2: the reader is exact on every fixture posse owns a capture of.
func TestQAKnownScreenReaderNamesEveryFixtureExactly(t *testing.T) {
	t.Parallel()
	d := d3ksTables(t)
	testdata := filepath.Join("..", "..", "etc", "herdr", "agent-detection", "testdata")

	// ENUMERATED, not listed. The script warns about an unlabelled fixture
	// on stderr and exits 0, so a sixteenth capture would be measured by
	// nothing — and the detection fixtures are exactly the directory a
	// detection bead adds to.
	shipped := map[string]string{}
	agents, err := os.ReadDir(testdata)
	if err != nil {
		t.Fatalf("%s: %v", testdata, err)
	}
	for _, agent := range agents {
		if !agent.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(testdata, agent.Name()))
		if err != nil {
			t.Fatalf("%s: %v", agent.Name(), err)
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".txt") {
				continue
			}
			key := agent.Name() + "/" + strings.TrimSuffix(f.Name(), ".txt")
			shipped[key] = filepath.Join(testdata, agent.Name(), f.Name())
		}
	}
	if len(shipped) == 0 {
		t.Fatalf("no .txt fixtures under %s — this arm is reading nothing, so a clean result is not evidence", testdata)
	}

	var unlabelled, stale []string
	for key := range shipped {
		if _, ok := d.Expected[key]; !ok {
			unlabelled = append(unlabelled, key)
		}
	}
	for key := range d.Expected {
		if _, ok := shipped[key]; !ok {
			stale = append(stale, key)
		}
	}
	sort.Strings(unlabelled)
	sort.Strings(stale)
	for _, key := range unlabelled {
		t.Errorf("%s is shipped under %s and scripts/%s's EXPECTED does not label it — an unlabelled capture is a screen this reader is measured on by nothing, and the script says so on stderr and still exits 0",
			key, testdata, d3ksScript)
	}
	for _, key := range stale {
		t.Errorf("scripts/%s's EXPECTED labels %s and the tree does not ship it — the label is a claim about a capture nobody can replay", d3ksScript, key)
	}

	keys := make([]string, 0, len(shipped))
	for key := range shipped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		want, ok := d.Expected[key]
		if !ok {
			continue
		}
		b, err := os.ReadFile(shipped[key])
		if err != nil {
			t.Errorf("%s: %v", key, err)
			continue
		}
		// The WHOLE capture, which is the input clause ranger-base-qk9tr
		// reversed by measurement and ranger-base-76gc4 put on the record:
		// over herdr's 243-character previews the same reader names 5 of 11
		// residue cases instead of 11.
		got := KnownScreensIn([]string{string(b)})
		if !d3ksSameStrings(got, want) {
			t.Errorf("%s: the reader names %v and the fixture shows %v", key, got, want)
		}
	}

	// The two idle screens, by name. An empty expected set is the one row a
	// reader that returned nothing at all would also satisfy, and ADR 0066
	// D5's bar is that zero of the idle fixtures read as a blocked screen.
	for key, want := range map[string][]string{
		"codex/idle-composer":                    nil,
		"grok/idle-composer-with-consent-banner": {"grok.consent_banner"},
	} {
		if !d3ksSameStrings(d.Expected[key], want) {
			t.Errorf("%s is labelled %v in scripts/%s and this arm was written against %v — the idle screens are ADR 0066 D5's own bar and the two labellings have diverged",
				key, d.Expected[key], d3ksScript, want)
		}
	}

	// And the two-screen case, by name: three of the fifteen fixtures show a
	// splash with the consent banner drawn over it, which is the whole
	// reason this reader returns a set and a `choice` was not enough.
	two := 0
	for _, want := range d.Expected {
		if len(want) > 1 {
			two++
		}
	}
	if two == 0 {
		t.Errorf("no fixture in scripts/%s's EXPECTED shows more than one screen — the set-valued reader's reason for being is gone from the corpus, so nothing here measures it", d3ksScript)
	}
}

// Arm 3: the reader's own rules, with no tree and no script.
func TestKnownScreensInIsSetValuedAndCanSayNothing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		texts []string
		want  []string
	}{{
		name:  "nothing on the pane",
		texts: []string{"$ ", "", "~/src/posse"},
		want:  nil,
	}, {
		name:  "no regions at all",
		texts: nil,
		want:  nil,
	}, {
		// The heading as the screen draws it: padded, capitalized, wrapped
		// in border art. Only whitespace and case are folded.
		name:  "a heading inside border art",
		texts: []string{"╭────────────╮\n│  Update  Available!  │\n╰────────────╯"},
		want:  []string{"codex.update_menu"},
	}, {
		// Two true screens on one pane, which is why this is a set.
		name:  "a splash with the consent banner over it",
		texts: []string{"New worktree\nResume session\n", "Help improve Grok  [Opt out] [Opt in]"},
		want:  []string{"grok.consent_banner", "grok.startup_splash"},
	}, {
		// The conjunction: one phrase of grok's splash marker is not the
		// splash. "New worktree" alone is a row posse's own output carries.
		name:  "half of a conjunction is not a match",
		texts: []string{"New worktree created at ~/.posse/worktrees/posse/x"},
		want:  nil,
	}, {
		// An alternative: either heading alone names codex's sign-in menu,
		// because the narrow layout draws one below the fold.
		name:  "one alternative is enough",
		texts: []string{"2. Sign in with Device Code"},
		want:  []string{"codex.signin_menu"},
	}, {
		// Regions are joined, and a marker must land inside ONE of them or
		// across the join — never by the join inventing a phrase. These two
		// regions each hold half of the conjunction, which is the splash.
		name:  "a conjunction spread across two regions",
		texts: []string{"New worktree", "Resume session"},
		want:  []string{"grok.startup_splash"},
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := KnownScreensIn(c.texts); !d3ksSameStrings(got, c.want) {
				t.Errorf("KnownScreensIn(%q) = %v, want %v", c.texts, got, c.want)
			}
		})
	}

	// Sorted, so a failure line and a census row are stable across runs:
	// two identical readings printed in two orders read as two findings.
	got := KnownScreensIn([]string{"Help improve Grok", "New worktree", "Resume session", "Update available!"})
	if !sort.StringsAreSorted(got) {
		t.Errorf("KnownScreensIn returned %v, which is not sorted", got)
	}

	// An EMPTY marker names nothing. An `all` over no phrases is true, so
	// the obvious spelling of the conjunction would make a row whose markers
	// went missing in an edit match every screen on every pane — the one
	// failure a substring reader cannot report on itself.
	if knownScreenMarkerIn("update available! sign in with chatgpt", nil) {
		t.Error("an empty marker matched — a row that lost its markers would name its screen on every pane")
	}
	if knownScreenMarkerIn("anything at all", []string{}) {
		t.Error("a zero-phrase marker matched — see above")
	}

	// Every shipped row has at least one marker with at least one phrase,
	// which is what the check above makes meaningful.
	for _, s := range KnownScreens {
		if len(s.Markers) == 0 {
			t.Errorf("%s carries no markers, so the reader can never name it", s.Name)
		}
		for i, marker := range s.Markers {
			if len(marker) == 0 {
				t.Errorf("%s marker %d is empty", s.Name, i)
			}
			for _, p := range marker {
				if normKnownScreen(p) == "" {
					t.Errorf("%s marker %d holds a blank phrase", s.Name, i)
				}
			}
		}
	}
}

// Arm 4: the failure line carries the row, last, and loses nothing.
func TestQAUnknownScreenFailureLineCarriesTheLooksLikeRow(t *testing.T) {
	t.Parallel()
	det := AgentDetection{State: "idle", FallbackReason: "default_known_agent_idle_fallback"}
	var er EvaluatedRule
	er.ID = "live_prompt_box"
	er.Region = "bottom_non_empty_lines(2)"
	er.Evidence.RegionBytes = 124
	er.Evidence.RegionPreview = "╰─ Grok 4.6 (high) ─╯"
	det.EvaluatedRules = []EvaluatedRule{er}

	bare := det.WhatHerdrSaw("", nil)
	if !strings.Contains(bare, "herdr evaluated 1 rules") || !strings.Contains(bare, "live_prompt_box") {
		t.Fatalf("herdr's working is not in the block at all, so this arm cannot tell whether the row displaced it:\n%s", bare)
	}
	if strings.Contains(bare, "looks like") {
		t.Errorf("an empty D3 report printed a row — an unrecognized screen that is not one of the eight is the ordinary case, and a row on every refusal is how a diagnostic gets skimmed past:\n%s", bare)
	}

	with := det.WhatHerdrSaw("", []string{"grok.consent_banner", "grok.startup_splash"})
	if !strings.HasPrefix(with, bare) {
		t.Errorf("the D3 report did not APPEND — herdr's working is the diagnostic that can be chased to a codepoint and this reader's cannot (ADR 0066 D2):\n%s", with)
	}
	row := strings.TrimPrefix(with, bare)
	if row != "\n    looks like: grok.consent_banner, grok.startup_splash" {
		t.Errorf("the appended row is %q, want one indented line naming both screens, comma-separated", row)
	}
	if strings.Count(with, "looks like") != 1 {
		t.Errorf("the block carries %d `looks like` rows, want exactly 1:\n%s", strings.Count(with, "looks like"), with)
	}

	// The reported route: no manifest, no capture, so the reader has nothing
	// to read and the block is the reporter's word unchanged (ADR 0061 D3.3).
	reported := AgentDetection{Reported: "some-plugin", State: "idle"}
	if got := reported.WhatHerdrSaw("detection_why: the plugin says so", nil); !strings.Contains(got, "No screen evidence on this route") || strings.Contains(got, "looks like") {
		t.Errorf("the reported block changed shape:\n%s", got)
	}
}

// Arm 5: the record carries the set, and the real census counts it.
func TestQAD3RecordCarriesLooksLikeAndTheCensusCountsIt(t *testing.T) {
	t.Parallel()
	script := filepath.Join("..", "..", "scripts", "readings-census.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s: %v — ADR 0066 D1 names a census script and this arm is the only thing that runs the new count", script, err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatalf("no python3 on PATH, and %s is python3: %v", script, err)
	}

	// The record, built the way logUnrecognized builds one: herdr's previews
	// plus the whole-pane capture, and the set read off the evidence block.
	ev := ReadingEvidence{State: "idle", Fallback: "default_known_agent_idle_fallback"}
	ev.Regions = []ReadingRegion{{
		Name: "top_non_empty_lines(6)", Bytes: 600, Text: "   .:'`'::.\n  ::.  .::\n", Truncated: true,
	}, {
		Name: PaneCaptureRegion, Bytes: 42, Text: "  Update available!\n  1. Update now\n  2. Skip\n",
	}}
	looksLike := ev.LooksLike()
	if !d3ksSameStrings(looksLike, []string{"codex.update_menu"}) {
		t.Fatalf("ReadingEvidence.LooksLike() = %v, want [codex.update_menu] — the rest of this arm measures nothing without it", looksLike)
	}

	// And the preview-only reading is the WORSE one, which is the whole
	// reason the capture is on the record (ranger-base-qk9tr: 5 of 11 over
	// the previews, 11 of 11 over the capture).
	previews := ReadingEvidence{Regions: ev.Regions[:1]}
	if got := previews.LooksLike(); len(got) != 0 {
		t.Errorf("the previews alone named %v — this arm's fixture was written so that only the capture carries the heading, and it no longer does", got)
	}

	a, _, tree := rlTree(t)
	named := Reading{
		Decision: DecisionUnknownScreen, Consequence: ConsequenceRefusal,
		Verdict: "never promptable: only \"idle\"", Rule: RulePromptReady,
		Herdr: ev, LooksLike: looksLike,
	}
	if err := a.AppendReading(tree.Path, named); err != nil {
		t.Fatal(err)
	}
	log, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ReadReadings(log)
	if err != nil || len(back) != 1 {
		t.Fatalf("ReadReadings: %v (%d record(s))", err, len(back))
	}
	if !d3ksSameStrings(back[0].LooksLike, looksLike) {
		t.Errorf("the record came back with looks_like=%v, want %v — the field is the only thing that makes the saving countable", back[0].LooksLike, looksLike)
	}

	if c := d3ksCensus(t, script, log); c.Named.D3 != 1 || c.Named.Named != 1 || c.Named.Screens["codex.update_menu"] != 1 {
		t.Errorf("the census says d3=%d named=%d screens=%v, want 1, 1 and one codex.update_menu",
			c.Named.D3, c.Named.Named, c.Named.Screens)
	}

	// THE OTHER END: records of the shape every D3 record had before this
	// bead. A reader whose absence counted as a naming would make every log
	// written before today a fleet of free diagnoses.
	bareApp, _, bareTree := rlTree(t)
	for i := 0; i < 2; i++ {
		if err := bareApp.AppendReading(bareTree.Path, Reading{
			Decision: DecisionUnknownScreen, Consequence: ConsequenceRefusal,
			Verdict: "never promptable", Rule: RulePromptReady,
			Herdr: ReadingEvidence{State: "idle", Regions: []ReadingRegion{{Name: PaneCaptureRegion, Bytes: 2, Text: "$ "}}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	bareLog, err := bareApp.ReadingsLogPath(bareTree.Path)
	if err != nil {
		t.Fatal(err)
	}
	if c := d3ksCensus(t, script, bareLog); c.Named.D3 != 2 || c.Named.Named != 0 {
		t.Errorf("over a log of D3 records carrying no set the census says d3=%d named=%d, want 2 and 0", c.Named.D3, c.Named.Named)
	}
}

// d3ksCensus runs the real census over one log and returns the figures this
// bead added. The REAL script, for the reason the false-IDLE arm gives: ADR
// 0066 D1 names an instrument, and a pin that reimplemented its arithmetic
// would be measuring a second copy.
func d3ksCensus(t *testing.T, script, log string) struct {
	Readings int `json:"readings"`
	Named    struct {
		D3      int            `json:"d3"`
		Named   int            `json:"named"`
		Screens map[string]int `json:"screens"`
	} `json:"named_screens"`
} {
	t.Helper()
	var c struct {
		Readings int `json:"readings"`
		Named    struct {
			D3      int            `json:"d3"`
			Named   int            `json:"named"`
			Screens map[string]int `json:"screens"`
		} `json:"named_screens"`
	}
	out, err := exec.Command("python3", script, "--log", log, "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --json: %v\n%s", script, err, out)
	}
	if err := json.Unmarshal(out, &c); err != nil {
		t.Fatalf("census --json is not json: %v\n%s", err, out)
	}
	return c
}
