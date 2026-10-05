//go:build posse_arm2

package posse

// QA pins for ranger-base-dckhf — the reading that TYPED rides on the record
// its keystrokes produced (ADR 0066 D1 as amended 2026-10-04,
// ranger-base-o1aoi; the measurement and the priced alternatives are in
// docs/notes.d/ranger-base-o1aoi.md).
//
// THE GAP THESE CLOSE, in ranger-base-3xt9y §4's own words: ADR 0066 D1's
// five consequences are five ways of NOT acting, so a pane-state reading
// that says *idle* over a dialog, a splash or a shell — the reading that
// types — leaves no record. It is not invisible to the fleet, though, only
// to the log: typed text that starts no turn ends in the D5 stall verdict,
// which this log already counts. So the gate's reading and a capture from
// each side of the keystrokes ride on THAT record, and the census counts the
// seen-idle ones as the false-IDLE candidates.
//
// They live in arm 2 beside promptstall_qa_test.go because the mechanism is
// the stall verdict's: two of the five arms drive a whole dispatch pass to a
// stall and read what it wrote.
//
// WHY EACH ARM IS BUILT THE WAY IT IS, since every one of them could be
// written in a shape that measures nothing:
//
//  1. The two captures carry DIFFERENT screens, armed through the fake's
//     numbered pane-text lever. The claim the amendment rests on is about
//     two instants — "the splash with its banner, then a composer with the
//     text gone" — and one fixture screen read twice would be green over a
//     writer that captured once and copied it.
//  2. The turn-starts arm asserts a capture WAS taken and still nothing was
//     written. Without the first half it would pass against a fire that
//     never took one.
//  3. The launch-line arm pins both ends — the launch carries no gate
//     reading, and the writer handed no gate writes no block and spends no
//     pane read — with the typed path on the same fixture as its control,
//     because a pin on a zero value measures nothing unless something
//     fills it in.
//  4. The census arm runs the real script, and asserts the count is 0 over a
//     log of records with no gate block — the shape every D5 record had
//     before this bead — as well as 1 over one that has them.
//  5. The ceiling arm reads the WHOLE log file and not the parsed record,
//     because a redaction that put the text back through a second field
//     would be green against a parsed read of the first one; and its second
//     half reads the evidence the pass is still holding, which is the one
//     place this change could have corrupted a bead mid-judgment.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The two screens, one per side of the keystrokes. Shaped like the measured
// incident (rangerhq-37c): grok drew a first-run splash that herdr read as a
// seen idle screen, posse typed, the splash swallowed the text, and the pane
// afterwards was a composer with nothing in it. Either screen alone reads as
// "idle" or "empty"; the PAIR is what says "eaten".
const (
	fiSplash   = "Welcome to grok!\n\n  Press enter to continue\n"
	fiAfterKey = "❯ \n\n  ? for shortcuts\n"
)

// fiSeat is the fixture the stall arms share: a real git repo (so the
// session gets a worktree whose git dir can hold the log), one ready `go`
// bead, an idle claude, herdr's working on every explain, the two screens,
// and a prompt that comes back agent_prompt_stalled.
//
// The REPO IS A REAL ONE and that is load-bearing: LogReading resolves the
// log through the session meta's Dir and a session with no tree gets no log
// at all (readingslog.go), so a pass over a plain temp dir would write
// nothing and every assertion below would be about an absent file.
func fiSeat(t *testing.T) (*Dispatcher, string, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)
	// The rule that matched, named like the real thing rather than left at
	// the fake's default: the gate block's whole purchase on rangerhq-7ia is
	// that it says WHICH rule said idle, and a pin asserting `fake_idle`
	// would be asserting the fixture.
	write(t, filepath.Join(fake, "explain-rule"), "claude_idle_composer")
	// herdr's working, so the gate block is asserted to carry it BESIDE the
	// captures rather than instead of them.
	write(t, filepath.Join(fake, "explain-rules"),
		`[{"id":"claude_idle_composer","matched":true,"region":"footer","state":"idle",`+
			`"evidence":{"region_bytes":15,"region_preview":"? for shortcuts"}}]`)
	// Read 1 is the screen posse typed at; read 2 is the screen the stall
	// was judged over.
	write(t, filepath.Join(fake, "pane-text", "w1_p1-1"), fiSplash)
	write(t, filepath.Join(fake, "pane-text", "w1_p1-2"), fiAfterKey)
	write(t, filepath.Join(fake, "prompt-error"), "agent_prompt_stalled")
	return d, fake, repo
}

// fiReadings is every reading the pass wrote under the session it dispatched.
// An absent log is no readings, which is a real answer and the one two arms
// here assert.
func fiReadings(t *testing.T, d *Dispatcher, repo string) []Reading {
	t.Helper()
	session := SessionForBead("ranger", repo, "a-1")
	m, ok := d.HB.readMeta(session)
	if !ok || m.Dir == "" {
		t.Fatalf("the pass left no meta with a dir for %s — nothing could have been logged, so no arm here measures anything", session)
	}
	log, err := d.App.ReadingsLogPath(m.Dir)
	if err != nil {
		t.Fatalf("ReadingsLogPath(%s): %v", m.Dir, err)
	}
	if _, err := os.Stat(log); os.IsNotExist(err) {
		return nil
	}
	rs, err := ReadReadings(log)
	if err != nil {
		t.Fatalf("ReadReadings(%s): %v", log, err)
	}
	return rs
}

// fiD5 is the one stall record a pass wrote, and a fatal if it wrote any
// other number of them.
func fiD5(t *testing.T, rs []Reading) Reading {
	t.Helper()
	var out []Reading
	for _, r := range rs {
		if r.Decision == DecisionStallVerdict {
			out = append(out, r)
		}
	}
	if len(out) != 1 {
		var all []string
		for _, r := range rs {
			all = append(all, string(r.Decision)+"/"+string(r.Consequence))
		}
		t.Fatalf("the pass wrote %d D5 records, want 1 (every reading it wrote: %v)", len(out), all)
	}
	return out[0]
}

// ARM 1. A typed prompt that stalls writes ONE D5 record, and it carries the
// settle gate's reading as its own evidence block with a capture from each
// side of the keystrokes.
func TestQAStalledTypedPromptRecordsTheReadingThatTypedAndBothScreens(t *testing.T) {
	t.Parallel()
	d, fake, repo := fiSeat(t)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a stalled prompt must not fail the pass: %v", err)
	}
	// The verdict this record is keyed on, so a change that stopped handing
	// the bead back would not pass this arm by writing some other record.
	if out := dispatcherOut(d); !strings.Contains(out, "unclaimed") {
		t.Fatalf("the fixture must reach the hand-back verdict, or this arm is about a record nobody writes:\n%s", out)
	}

	r := fiD5(t, fiReadings(t, d, repo))
	if r.Consequence != ConsequenceHandBack {
		t.Errorf("the D5 record says consequence %q, want %q", r.Consequence, ConsequenceHandBack)
	}
	if r.Gate == nil {
		t.Fatalf("the D5 record carries no gate block — the reading that TYPED is the whole of what ADR 0066 D1's amendment adds; the record holds only %+v", r.Herdr)
	}

	// The gate's own reading, labelled as the gate's.
	if !r.Gate.Seen || r.Gate.State != "idle" {
		t.Errorf("the gate block says state=%q seen=%v, want idle/true — this is the reading the census counts as a false-IDLE candidate", r.Gate.State, r.Gate.Seen)
	}
	if r.Gate.Rule != "claude_idle_composer" {
		t.Errorf("the gate block names rule %q, want the rule that matched — the rule or the chrome that said idle is the thing a reword breaks (rangerhq-7ia)", r.Gate.Rule)
	}
	if reg, ok := r.Gate.RegionOf("footer"); !ok || reg.Text != "? for shortcuts" {
		t.Errorf("the gate block lost herdr's own working (%q, present=%v) — the captures are added to the evidence, never instead of it", reg.Text, ok)
	}

	// BOTH SCREENS, in order. This is the pair the amendment rests on.
	at, ok := r.Gate.RegionOf(PaneCaptureAtPromptRegion)
	if !ok {
		t.Fatalf("no %q region: the record cannot say what was on screen when the keys went in", PaneCaptureAtPromptRegion)
	}
	after, ok := r.Gate.RegionOf(PaneCaptureRegion)
	if !ok {
		t.Fatalf("no %q region: the record cannot say what the screen became", PaneCaptureRegion)
	}
	if !strings.Contains(at.Text, "Welcome to grok") {
		t.Errorf("the type-time capture is not the screen that was read first:\n%s", at.Text)
	}
	if !strings.Contains(after.Text, "? for shortcuts") || strings.Contains(after.Text, "Welcome to grok") {
		t.Errorf("the stall-time capture is not the screen as it was at the verdict:\n%s", after.Text)
	}
	if at.Text == after.Text {
		t.Errorf("both captures hold the same bytes, so this record was taken at one instant and the pair proves nothing:\n%s", at.Text)
	}
	for _, reg := range []ReadingRegion{at, after} {
		if reg.Truncated || reg.Bytes != len(reg.Text) {
			t.Errorf("capture %q: Truncated=%v Bytes=%d over %d bytes — a capture is one whole read, and a census comparing the two would read a truncation that did not happen",
				reg.Name, reg.Truncated, reg.Bytes, len(reg.Text))
		}
	}

	// D5's OWN block is untouched, which is ranger-base-3xt9y §4's rule: the gate's
	// reading is labelled as the gate's, never as D5's. A stall verdict
	// reads no screen (promptstall.go), so a region here would be this
	// record carrying a reading somebody else took.
	if len(r.Herdr.Regions) != 0 {
		t.Errorf("the D5 verdict's own evidence grew regions %+v — D5 reads no screen, and the gate's screens belong in the gate block", r.Herdr.Regions)
	}

	// TWO pane reads and not one: the second capture is a second instant,
	// and a writer that reused the first would be green on every assertion
	// above but this one and the pair check.
	reads, _ := os.ReadFile(filepath.Join(fake, "pane-read-log"))
	if got := strings.Count(string(reads), "w1:p1"); got != 2 {
		t.Errorf("the pass spent %d pane read(s) on w1:p1, want 2 — one before the keystrokes and one at the verdict:\n%s", got, reads)
	}
}

// ARM 2. A typed prompt whose turn starts writes NOTHING: the capture is
// taken, held in memory, and dropped with the bead's other in-flight state.
//
// Its first half is what makes it a measurement — a fire that never took a
// capture at all would pass the second half on its own.
func TestQATypedPromptWhoseTurnStartsWritesNoStallRecord(t *testing.T) {
	t.Parallel()
	d, fake, repo := fiSeat(t)
	// herdr's window missed the start of the turn; posse's own wait sees it
	// (ranger-base-uauvn arm 1). The verdict is a rewait.
	write(t, filepath.Join(fake, "stall-wait-status"), "working")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a stalled prompt whose turn started must not fail the pass: %v", err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "a turn is under way") {
		t.Fatalf("the fixture must reach the rewait verdict, or this arm is not about the path it names:\n%s", out)
	}

	// The capture WAS taken — the keystrokes happened, so the reading that
	// typed exists and was held.
	reads, _ := os.ReadFile(filepath.Join(fake, "pane-read-log"))
	if !strings.Contains(string(reads), "w1:p1") {
		t.Fatalf("no capture was taken before the keystrokes, so the arm below measures nothing:\n%s", reads)
	}

	// And nothing was written. Not "no gate block" — no record: a rewait is
	// not logged at all, and the held evidence goes with the rest of the
	// in-flight state.
	for _, r := range fiReadings(t, d, repo) {
		if r.Decision == DecisionStallVerdict {
			t.Errorf("a rewait wrote a D5 record (%q) — a reading that found a turn under way has never been the thing anybody had to diagnose", r.Verdict)
		}
		if r.Gate != nil {
			t.Errorf("a %s record carries a gate block — nothing is written at type time (ADR 0066 D1 as amended)", r.Decision)
		}
	}
}

// ARM 3. The launch-line path carries no gate block, pinned at both ends.
//
// It cannot be driven to a stall record at all — gather asks judgeStall only
// for `!p.delivered` — so the two halves of the exclusion are pinned where
// they live: the launch hands back no gate reading, and the writer handed no
// gate writes no block.
func TestQALaunchLinePromptCarriesNoGateReading(t *testing.T) {
	t.Parallel()
	// The SHIPPED runtimes, and their declarations are read rather than
	// written here: `grok` declares prompt: argv and `claude` does not
	// (dispatchparity_qa_test.go's table). A hand-written pair would pin
	// this arm to a fixture, and the exclusion it is about is a property of
	// the two real paths.
	//
	// A BACKEND EACH, because one fake herdr serves one board: the second
	// session of a shared fake opens a workspace its agents.json does not
	// list, and the launch then fails on "no agent detected" — a fixture
	// collision that would read as a product refusal (hermetic's own note
	// on repeat runs).
	for _, tc := range []struct {
		runtime string
		argv    bool
	}{
		{runtime: "grok", argv: true},
		{runtime: DefaultRuntime, argv: false},
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			dispatcherErr(t, d)
			writePersona(t, b.App, "ranger", "[go]")
			repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
			idleClaude(t, fake)

			rt, err := b.App.LoadRuntime(tc.runtime)
			if err != nil {
				t.Fatalf("LoadRuntime(%s): %v", tc.runtime, err)
			}
			if got := rt.PromptMode() == PromptArgv; got != tc.argv {
				t.Fatalf("%s declares prompt: argv = %v, this arm needs %v — one of them moved", tc.runtime, got, tc.argv)
			}

			is := RepoIssue{Dir: repo, BdIssue: BdIssue{ID: "a-1", Title: "t"}}
			l, err := d.launchSession(is, "ranger", "seat", tc.runtime, "fast", func() string { return "work" }, nil, false)
			if err != nil {
				t.Fatalf("launchSession on %s: %v\n%s", tc.runtime, err, dispatcherOut(d))
			}
			if l.delivered != tc.argv {
				t.Fatalf("%s took the %s path, which is not the one this case is about", tc.runtime, map[bool]string{true: "launch-line", false: "typed"}[l.delivered])
			}
			if tc.argv {
				// Nothing is typed on the launch line, so there is no
				// reading that typed and the record can carry no gate block.
				if l.gate.Seen() || l.gate.State != "" {
					t.Errorf("the launch-line path handed back a gate reading (state=%q rule=%q seen=%v)",
						l.gate.State, l.gate.Rule.ID, l.gate.Seen())
				}
				// The writer's end, on the same fixture: a bead that typed
				// nothing holds no gate, and stallGate must neither invent a
				// block nor spend a pane read for one.
				before, _ := os.ReadFile(filepath.Join(fake, "pane-read-log"))
				if g := d.stallGate(&pendingBead{session: "seat", target: l.target, delivered: true}); g != nil {
					t.Errorf("a bead that typed nothing produced a gate block %+v", g)
				}
				after, _ := os.ReadFile(filepath.Join(fake, "pane-read-log"))
				if len(after) != len(before) {
					t.Errorf("a bead with no gate reading still spent a pane read:\n%s", strings.TrimPrefix(string(after), string(before)))
				}
				return
			}
			// The CONTROL, and it is what makes the case above a measurement
			// rather than a claim about a zero value nobody fills in: the
			// typed path hands back the reading its settle gate opened on.
			if !l.gate.Seen() || l.gate.State != "idle" {
				t.Errorf("the typed path handed back state=%q seen=%v, want the seen idle reading the gate opened on", l.gate.State, l.gate.Seen())
			}
		})
	}
}

// ARM 4. The census. The record still parses, the new count reads 1 over a
// log with a seen-idle gate reading and 0 over a log of the shape that
// existed before this bead, and the corpus exports both captures as region
// files like every other region.
//
// It runs the REAL script, for TestQAReadingsCensusCountsByDayAndExportsTheCorpus's
// reason: ADR 0066 D1 names an instrument, and a pin that reimplemented its
// arithmetic would be measuring a second copy.
func TestQAFalseIdleCensusCountsTheSeenIdleGateReadings(t *testing.T) {
	t.Parallel()
	script := filepath.Join("..", "..", "scripts", "readings-census.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s: %v — ADR 0066 D1 names a census script and this arm is the only thing that runs the new count", script, err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatalf("no python3 on PATH, and %s is python3: %v", script, err)
	}

	a, _, tree := rlTree(t)
	d5 := func(gate *ReadingEvidence) Reading {
		return Reading{
			Decision: DecisionStallVerdict, Consequence: ConsequenceHold,
			Verdict: "keep: no turn started", Rule: RuleStallVerdict,
			Herdr: ReadingEvidence{State: stallNoTurn, Seen: true},
			Gate:  gate,
		}
	}
	gate := &ReadingEvidence{State: "idle", Rule: "claude_idle_composer", Seen: true, Regions: []ReadingRegion{
		{Name: "footer", Bytes: 15, Text: "? for shortcuts", Truncated: false},
		{Name: PaneCaptureAtPromptRegion, Bytes: len(fiSplash), Text: fiSplash},
		{Name: PaneCaptureRegion, Bytes: len(fiAfterKey), Text: fiAfterKey},
	}}
	for _, r := range []Reading{
		d5(gate),
		// A gate herdr did NOT see: a stall the gate cannot be blamed for,
		// so it is in the D5 row and not in the count.
		d5(&ReadingEvidence{State: "idle", Fallback: "default_known_agent_idle_fallback", Seen: false}),
		// And a record with no gate block at all — the launch-line path,
		// and every D5 record written before this bead.
		d5(nil),
	} {
		if err := a.AppendReading(tree.Path, r); err != nil {
			t.Fatalf("AppendReading: %v", err)
		}
	}
	log, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}

	// IT STILL PARSES, through posse's own reader, with the gate block
	// intact. The log is one JSON object per line and the new field is one
	// more key; a corpus that could not be read back would be the one way
	// this change could cost the instrument rather than add to it.
	rs, err := ReadReadings(log)
	if err != nil || len(rs) != 3 {
		t.Fatalf("ReadReadings: %v (%d records, want 3)", err, len(rs))
	}
	if rs[0].Gate == nil || !rs[0].Gate.Seen {
		t.Fatalf("the gate block did not survive the round trip: %+v", rs[0])
	}
	if _, ok := rs[0].Gate.RegionOf(PaneCaptureAtPromptRegion); !ok {
		t.Errorf("the type-time capture did not survive the round trip: %+v", rs[0].Gate.Regions)
	}
	if rs[2].Gate != nil {
		t.Errorf("a record written with no gate block came back with one: %+v", rs[2].Gate)
	}

	c := fiCensus(t, script, log)
	if c.FalseIdle.Candidates != 1 {
		t.Errorf("the census counts %d false-IDLE candidates, want 1 — this is the number ADR 0066 opened with as UNKNOWN", c.FalseIdle.Candidates)
	}
	if c.FalseIdle.D5 != 3 || c.FalseIdle.WithGate != 2 {
		t.Errorf("the census says d5=%d with_gate=%d, want 3 and 2 — the residual has to be visible or the count is a rate over an unknown denominator",
			c.FalseIdle.D5, c.FalseIdle.WithGate)
	}
	// The D5 row counts what it always counted: no sixth consequence, no new
	// record kind, no change to the denominator.
	if got := c.PerDecision["D5"]["total"]; got != 3 {
		t.Errorf("the D5 row counts %d, want 3 — the amendment adds bytes to a record that already existed", got)
	}
	if c.Replayable != 0 {
		t.Errorf("the census calls %d of these replayable, want 0 — a D5 verdict is a reading of a herdr wait and a commit count, and the gate's screens are some other reader's bytes", c.Replayable)
	}
	// Both captures are counted as regions the record carries, under the two
	// names the writer gives them.
	if c.Regions[PaneCaptureAtPromptRegion] != 1 || c.Regions[PaneCaptureRegion] != 1 {
		t.Errorf("the census does not count the gate's captures: %v", c.Regions)
	}

	// ZERO over the shape that existed before this bead. A count that read
	// its own absence as a candidate would make every pre-amendment log a
	// fleet of false IDLEs.
	bare, _, bareTree := rlTree(t)
	for i := 0; i < 3; i++ {
		if err := bare.AppendReading(bareTree.Path, d5(nil)); err != nil {
			t.Fatal(err)
		}
	}
	bareLog, _ := bare.ReadingsLogPath(bareTree.Path)
	if bc := fiCensus(t, script, bareLog); bc.FalseIdle.Candidates != 0 || bc.FalseIdle.D5 != 3 {
		t.Errorf("over a log of records with no gate block the census says candidates=%d d5=%d, want 0 and 3",
			bc.FalseIdle.Candidates, bc.FalseIdle.D5)
	}

	// THE CORPUS. Both captures land as region files, so `herdr agent
	// explain --file` can be pointed at either side of the keystrokes —
	// which is how every real screen-reading bead was closed (capture →
	// fixture → rule).
	dir := t.TempDir()
	out, err := exec.Command("python3", script, "--log", log, "--export-corpus", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("%s --export-corpus: %v\n%s", script, err, out)
	}
	for region, want := range map[string]string{
		PaneCaptureAtPromptRegion: fiSplash,
		PaneCaptureRegion:         fiAfterKey,
	} {
		matches, err := filepath.Glob(filepath.Join(dir, "regions", "*", region+".txt"))
		if err != nil || len(matches) != 1 {
			t.Errorf("the corpus exported %d file(s) for %q, want 1 (%v)", len(matches), region, err)
			continue
		}
		got, err := os.ReadFile(matches[0])
		if err != nil || string(got) != want {
			t.Errorf("%s holds %q, want %q (%v)", matches[0], got, want, err)
		}
	}
	// And the case itself says which side of the keystrokes it is a
	// candidate on, so a reader of corpus.jsonl alone can select them.
	body, err := os.ReadFile(filepath.Join(dir, "corpus.jsonl"))
	if err != nil {
		t.Fatalf("no corpus.jsonl: %v", err)
	}
	var candidates int
	for _, ln := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		var one struct {
			Gate *struct {
				Candidate bool `json:"false_idle_candidate"`
			} `json:"gate"`
		}
		if err := json.Unmarshal([]byte(ln), &one); err != nil {
			t.Fatalf("corpus case is not json: %v\n%s", err, ln)
		}
		if one.Gate != nil && one.Gate.Candidate {
			candidates++
		}
	}
	if candidates != 1 {
		t.Errorf("corpus.jsonl marks %d case(s) as false-IDLE candidates, want 1", candidates)
	}
}

// ARM 5. The gate block's captures go through the data ceiling like every
// other region, and the record names the classes it lost.
//
// The ceiling is keyed on no region name at all (redactRegions), but it was
// keyed on one FIELD — the record's own `herdr` — and a second block of
// regions is a second way for a capture of somebody's terminal to reach a
// local file untouched (ADR 0050 D2). The second arm is the one that would
// have caught the shape this was nearly written in: redacting through the
// Gate pointer would leave the pendingBead that is still in flight holding
// redacted bytes, so the stall-time capture would be appended to a block
// whose earlier half had already lost its text.
func TestQAGateCaptureGoesThroughTheDataCeiling(t *testing.T) {
	// No t.Parallel: qaCeilingWall sets the environment.
	w := qaCeilingWall(t, "")
	a := hermetic(t, &App{ConfigPath: filepath.Join(w.home, "config.yaml"), Home: w.home, StateDir: filepath.Join(w.home, "state")})
	repo := wtRepo(t)
	tree, err := a.EnsureSessionTree(repo, "ranger-posse-fi-1", nil)
	if err != nil || tree == nil {
		t.Fatalf("EnsureSessionTree: %v", err)
	}

	// The hit is drawn on the screen posse typed at, which is the real
	// shape: a capture is whatever the pane was holding.
	screen := "Welcome!\n" + qaCeilingHit + "\n  Press enter to continue\n"
	held := &ReadingEvidence{State: "idle", Rule: "claude_idle_composer", Seen: true,
		Regions: []ReadingRegion{{Name: PaneCaptureAtPromptRegion, Bytes: len(screen), Text: screen}}}
	if err := a.AppendReading(tree.Path, Reading{
		Decision: DecisionStallVerdict, Consequence: ConsequenceHandBack,
		Verdict: "hand back: no turn started", Rule: RuleStallVerdict,
		Herdr: ReadingEvidence{State: stallNoTurn, Seen: true},
		Gate:  held,
	}); err != nil {
		t.Fatalf("AppendReading: %v", err)
	}

	path, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// THE WHOLE FILE, not the parsed record: a redaction that put the text
	// back through a second field would be green against a parsed read of
	// the first one.
	if strings.Contains(string(raw), qaCeilingHit) {
		t.Errorf("the gate block carries data-ceiling content verbatim — this content may not exist in a local file at all (ADR 0050):\n%s", raw)
	}
	if !strings.Contains(string(raw), "["+RedactedMark+":"+qaCeilingClass+"]") {
		t.Errorf("the ceiling took the text and left no marker naming the class:\n%s", raw)
	}
	rs, err := ReadReadings(path)
	if err != nil || len(rs) != 1 {
		t.Fatalf("ReadReadings: %v (%d records)", err, len(rs))
	}
	if got := rs[0].Redacted; len(got) != 1 || got[0] != qaCeilingClass {
		t.Errorf("the record names redacted classes %v, want exactly [%s] — a capture whose bytes are not the bytes read has to say so, whichever block carried it", got, qaCeilingClass)
	}

	// THE IN-FLIGHT EVIDENCE IS UNTOUCHED. Gate is a pointer and the writer
	// takes its Reading by value, so a redaction applied through the pointer
	// would reach back into a bead that is still being judged.
	if reg, ok := held.RegionOf(PaneCaptureAtPromptRegion); !ok || reg.Text != screen {
		t.Errorf("writing the record redacted the evidence the pass is still holding (%q) — the capture at type time must survive until the verdict is written", reg.Text)
	}
}

// fiCensus runs the census script over one log and returns the JSON it
// prints — the real instrument, read the way an operator reads it.
func fiCensus(t *testing.T, script, log string) struct {
	Readings    int                       `json:"readings"`
	PerDecision map[string]map[string]int `json:"per_decision"`
	Regions     map[string]int            `json:"regions"`
	Replayable  int                       `json:"replayable"`
	FalseIdle   struct {
		D5         int `json:"d5"`
		WithGate   int `json:"with_gate"`
		Candidates int `json:"candidates"`
	} `json:"false_idle"`
} {
	t.Helper()
	var c struct {
		Readings    int                       `json:"readings"`
		PerDecision map[string]map[string]int `json:"per_decision"`
		Regions     map[string]int            `json:"regions"`
		Replayable  int                       `json:"replayable"`
		FalseIdle   struct {
			D5         int `json:"d5"`
			WithGate   int `json:"with_gate"`
			Candidates int `json:"candidates"`
		} `json:"false_idle"`
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
