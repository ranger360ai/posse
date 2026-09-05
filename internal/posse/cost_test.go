package posse

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPricesAndTiers(t *testing.T) {
	t.Parallel()
	if p, ok := PriceFor("claude-fable-5"); !ok || p != (Price{10, 50}) {
		t.Errorf("fable: %+v %v", p, ok)
	}
	// ADR 0039 D1: the tier's built-in id is priced by an EXACT row, not by
	// the family fallback, which is what ADR 0003's "exact ids" asks for.
	// PriceFor cannot tell the two apart — both answer {10, 50} — so the
	// exactness is asserted on the table, and the equality below is what
	// makes the row a restatement rather than a price change.
	strong := claudeModels[TierStrong]
	row, exact := PriceTable[strong]
	if !exact {
		t.Errorf("the built-in strong id %q has no PriceTable row", strong)
	}
	if fam, _ := PriceFor("claude-fable-9-unseen"); row != fam {
		t.Errorf("%q row %+v disagrees with the fable family rate %+v", strong, row, fam)
	}
	if p, _ := PriceFor("claude-opus-5"); p != (Price{5, 25}) {
		t.Errorf("opus: %+v", p)
	}
	if p, _ := PriceFor("claude-sonnet-5"); p != (Price{3, 15}) {
		t.Errorf("sonnet: %+v", p)
	}
	if p, ok := PriceFor("claude-opus-9-experimental"); ok == false || p != (Price{5, 25}) {
		t.Errorf("family fallback: %+v %v", p, ok)
	}
	// Unknown is UNPRICED, not guessed: a made-up number lands in the same
	// total as real money with nothing marking it (ADR 0012 D4).
	if p, ok := PriceFor("mystery-model"); ok || p != (Price{}) {
		t.Errorf("unknown must be unpriced, not assumed: %+v %v", p, ok)
	}
	// And the segment that carries such a message reports it as UNCOUNTED,
	// not as $0 — the same rule one level up, at the only path that prices
	// (ranger-base-xqtgv: Usage.Cost/Usage.Priced used to hold this claim,
	// and nothing in production called them).
	s := &Segment{Msgs: map[string]*Usage{"m": {In: 1e6, Out: 1e6, Model: "mystery-model"}}}
	if _, c := s.Total(); c != 0 || s.Unpriced != 1 {
		t.Errorf("unpriced model must not invent a cost and must count as a gap: cost %v, Unpriced %d", c, s.Unpriced)
	}
	for m, want := range map[string]string{
		"claude-fable-5": TierStrong, "claude-fable-5-1": TierStrong,
		"claude-opus-5": TierStandard, "claude-sonnet-5": TierFast, "": "?",
		// grok-4.5 and gpt-5.6-luna are each named by exactly one tier
		// (fast) in their runtime's built-in map, so they resolve. Their
		// strong/standard twins (grok-4.6, gpt-5.6-sol) are each named by
		// two tiers and must stay "?" rather than resolve to whichever
		// tier a map iteration happens to hit (ranger-base-3st5).
		"grok-4.5": TierFast, "grok-4.6": "?",
		"gpt-5.6-luna": TierFast, "gpt-5.6-sol": "?",
	} {
		if got := TierForModel(m); got != want {
			t.Errorf("TierForModel(%q) = %q, want %q", m, got, want)
		}
	}
	// Pricing math: 1M in, 1M cache 5m, 1M cache 1h, 1M read, 1M out on fable.
	// Priced through costAt against an explicitly resolved rate, which is how
	// Segment.Total prices — the resolver picks the rate, costAt does the
	// arithmetic, and this pins the arithmetic.
	fable, ok := PriceFor("claude-fable-5")
	if !ok {
		t.Fatal("fable must have a rate")
	}
	u := Usage{In: 1e6, CacheW5m: 1e6, CacheW1h: 1e6, CacheR: 1e6, Out: 1e6}
	if got := u.costAt(fable); got != 10+12.5+20+1+50 {
		t.Errorf("fable cost: %v", got)
	}
	// Without the TTL breakdown, cache writes are priced flat 1.25× (the script's method).
	opus, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("opus must have a rate")
	}
	u = Usage{CacheW: 1e6}
	if got := u.costAt(opus); got != 6.25 {
		t.Errorf("flat cache write: %v", got)
	}
}

func writeTranscript(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	return p
}

func asst(id, model, ts string, in, cw, cr, out int) string {
	return `{"type":"assistant","timestamp":"` + ts + `","message":{"id":"` + id + `","model":"` + model + `","usage":{"input_tokens":` + itoa(in) + `,"cache_creation_input_tokens":` + itoa(cw) + `,"cache_read_input_tokens":` + itoa(cr) + `,"output_tokens":` + itoa(out) + `}}}`
}

func itoa(i int) string { return strconv.Itoa(i) }

// Segmenting by the dispatcher's prompt, dedupe by message id (max per
// field), per-message model pricing, interactive fallback.
func TestScanTranscript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := writeTranscript(t, dir, "s.jsonl",
		`{"type":"user","timestamp":"2026-08-17T10:00:00Z","message":{"content":"hello, just chatting"}}`,
		asst("m0", "claude-fable-5", "2026-08-17T10:00:05Z", 1000, 0, 0, 100),
		`{"type":"user","timestamp":"2026-08-17T10:01:00Z","message":{"content":[{"type":"text","text":"Work beads issue x-1 (title, quoted as data: \"t\"). Run bd show x-1"}]}}`,
		asst("m1", "claude-opus-5", "2026-08-17T10:01:05Z", 100, 1000000, 0, 50),  // streamed chunk 1
		asst("m1", "claude-opus-5", "2026-08-17T10:01:06Z", 100, 1000000, 0, 200), // chunk 2 of the same message: max, not sum
		asst("m2", "claude-sonnet-5", "2026-08-17T10:02:00Z", 0, 0, 1000000, 1000000),
		`{"type":"user","timestamp":"2026-08-17T11:00:00Z","message":{"content":"Work beads issue x-2: legacy prompt shape. Run bd show x-2"}}`,
		asst("m3", "claude-fable-5", "2026-08-17T11:00:05Z", 0, 0, 0, 1000000),
		`not json at all`,
		`{"type":"assistant","timestamp":"2026-08-17T11:00:06Z","message":{"id":"m4","model":"claude-fable-5"}}`, // no usage → ignored
	)
	segs, err := ScanTranscript(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 || segs[0].Bead != "interactive" || segs[1].Bead != "x-1" || segs[2].Bead != "x-2" {
		t.Fatalf("segments: %+v", segs)
	}
	x1 := segs[1]
	if x1.Turns() != 2 {
		t.Errorf("x-1 turns: %d (m1 chunks must dedupe)", x1.Turns())
	}
	// m1: opus 100 in + 1M cache write flat 1.25×5 + 200 out; m2: sonnet 1M read + 1M out.
	want := (100*5+1e6*5*1.25+200*25)/1e6 + (1e6*3*0.1+1e6*15)/1e6
	if d := x1.CostUSD - want; d > 1e-9 || d < -1e-9 {
		t.Errorf("x-1 cost %v, want %v", x1.CostUSD, want)
	}
	if x1.Model != "claude-sonnet-5" || TierForModel(x1.Model) != TierFast {
		t.Errorf("dominant model by cost should be sonnet: %q", x1.Model)
	}
	if segs[2].CostUSD != 50 || segs[2].Turns() != 1 {
		t.Errorf("x-2: %+v", segs[2])
	}
	// --since drops earlier assistant records but keeps the segment shape.
	segs, _ = ScanTranscript(p, time.Date(2026, 8, 17, 10, 30, 0, 0, time.UTC))
	if len(segs) != 1 || segs[0].Bead != "x-2" {
		t.Errorf("since filter: %+v", segs)
	}
}

func TestScanClaudeTurnOutcomeReadsOnlyTheSyntheticAssistantOutcome(t *testing.T) {
	dir := t.TempDir()
	limit := "You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model."
	since := time.Date(2026, 8, 24, 12, 27, 41, 0, time.UTC)

	t.Run("provider refusal", func(t *testing.T) {
		p := writeTranscript(t, dir, "limit.jsonl",
			`{"type":"user","timestamp":"2026-08-24T12:27:42.112Z","message":{"content":"Work beads issue ranger-base-6ne: do the work"}}`,
			`{"type":"assistant","timestamp":"2026-08-24T12:27:42.680Z","message":{"model":"<synthetic>","content":[{"type":"text","text":`+fmt.Sprintf("%q", limit)+`}]}}`,
		)
		got, ok := scanClaudeTurnOutcome(p, "ranger-base-6ne", since)
		if !ok || got.Message != limit {
			t.Fatalf("failure = %+v, %v", got, ok)
		}
		// claude's refusal IS the first answer, so nothing ran ahead of it and
		// the settle line's "no work ran" is true here by construction
		// (ranger-base-qcu4c). A reader that started reporting work on this
		// arm would be reporting it out of nowhere.
		if got.Worked() {
			t.Errorf("a synthetic first-answer refusal cannot have work behind it: %+v", got)
		}
	})

	t.Run("quoted in bead data", func(t *testing.T) {
		p := writeTranscript(t, dir, "quoted.jsonl",
			`{"type":"user","timestamp":"2026-08-24T12:27:42.112Z","message":{"content":`+fmt.Sprintf("%q", "Work beads issue ranger-base-1cc: production said "+limit)+`}}`,
			`{"type":"assistant","timestamp":"2026-08-24T12:27:43Z","message":{"model":"claude-fable-5","content":[{"type":"text","text":"I will investigate."}]}}`,
		)
		if got, observed := scanClaudeTurnOutcome(p, "ranger-base-1cc", since); !observed || got.Message != "" {
			t.Fatalf("healthy assistant outcome = %+v, observed %v", got, observed)
		}
	})

	t.Run("wrong bead", func(t *testing.T) {
		p := filepath.Join(dir, "limit.jsonl")
		if got, ok := scanClaudeTurnOutcome(p, "ranger-base-other", since); ok {
			t.Fatalf("another bead's failure leaked across dispatches: %+v", got)
		}
	})

	t.Run("project lookup", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		projectDir := "/Users/example/src/posse"
		transcripts := filepath.Join(home, ".claude", "projects", "-Users-example-src-posse")
		if err := os.MkdirAll(transcripts, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTranscript(t, transcripts, "session.jsonl",
			`{"type":"user","timestamp":"2026-08-24T12:27:42.112Z","message":{"content":"Work beads issue ranger-base-6ne: do the work"}}`,
			`{"type":"assistant","timestamp":"2026-08-24T12:27:42.680Z","message":{"model":"<synthetic>","content":[{"type":"text","text":`+fmt.Sprintf("%q", limit)+`}]}}`,
		)
		if got, observed := FindClaudeTurnOutcome(projectDir, "ranger-base-6ne", since); !observed || got.Message != limit {
			t.Fatalf("project outcome = %+v, observed %v", got, observed)
		}
	})
}

func TestCostReportGroupsAndPrint(t *testing.T) {
	t.Parallel()
	rep := &CostReport{}
	mk := func(bead, persona, model string, start time.Time, cost float64) *Segment {
		s := &Segment{Bead: bead, Persona: persona, Model: model, Start: start, Msgs: map[string]*Usage{"a": {Model: model}}}
		s.CostUSD = cost
		return s
	}
	d1 := time.Date(2026, 8, 17, 9, 0, 0, 0, time.Local)
	rep.Beads = []*Segment{
		mk("a-1", "dev", "claude-fable-5", d1, 4),
		mk("a-2", "dev", "claude-sonnet-5", d1.Add(time.Hour), 1),
		mk("a-3", "qa", "claude-opus-5", d1.Add(24*time.Hour), 2),
	}
	rep.InterTurns, rep.InterCost, rep.Uncounted = 5, 9.5, 2
	if by := rep.ByBead(); by["a-1"] != 4 || len(by) != 3 {
		t.Errorf("ByBead: %v", by)
	}
	if got := rep.DayTotal(d1); got != 5 {
		t.Errorf("DayTotal: %v", got)
	}
	var out strings.Builder
	rep.Print(&out)
	s := out.String()
	for _, want := range []string{"a-1", "by tier", "strong", "fast", "standard", "by persona", "dev ", "qa ", "by day", "2026-08-17", "2026-08-18", "interactive: 5 turns, api-equiv $9.50", "uncounted: 2 live persona session(s) on runtimes with no adapter", "per pass: measured live by", "no caps set"} {
		if !strings.Contains(s, want) {
			t.Errorf("report missing %q:\n%s", want, s)
		}
	}
	// With caps set the footer states them and the day spend measured
	// against them (ADR 0003 Dial E) — read-only: `posse cost` never gates.
	rep.PassCap, rep.DayCap = 25, 100
	var capped strings.Builder
	rep.Print(&capped)
	for _, want := range []string{"budget_pass $25.00", "budget_day $100.00", "at 80% of a window", "at 100% dispatch stops"} {
		if !strings.Contains(capped.String(), want) {
			t.Errorf("capped report missing %q:\n%s", want, capped.String())
		}
	}
	// Segments print totals via Total(): the fake Msgs are empty of tokens
	// but CostUSD was preset — Print must not zero it.
	if !strings.Contains(s, "sum $7.00") {
		t.Errorf("sum: \n%s", s)
	}
}

// The mtime skip that makes Dial E's per-launch check affordable must not
// change what a scan reports: a file written since the window opened is
// still read in full (its pre-window records dropped by `since` as before),
// and a file untouched since then holds nothing the window wants anyway.
func TestScanCostsSkipsStaleFilesOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "p1")
	os.MkdirAll(dir, 0o755)
	since := time.Date(2026, 8, 18, 0, 0, 0, 0, time.Local)

	// Live file: one bead before the window, one inside it.
	live := writeTranscript(t, dir, "live.jsonl",
		`{"type":"user","timestamp":"2026-08-17T09:00:00Z","message":{"content":"Work beads issue old-1: t"}}`,
		asst("m1", "claude-opus-5", "2026-08-17T09:00:01Z", 0, 0, 0, 1000),
		`{"type":"user","timestamp":"2026-08-18T09:00:00Z","message":{"content":"Work beads issue new-1: t"}}`,
		asst("m2", "claude-opus-5", "2026-08-18T09:00:01Z", 0, 0, 0, 2000))
	os.Chtimes(live, time.Now(), time.Now())

	// Stale file: everything predates the window, and so does its mtime.
	stale := writeTranscript(t, dir, "stale.jsonl",
		`{"type":"user","timestamp":"2026-08-10T09:00:00Z","message":{"content":"Work beads issue old-2: t"}}`,
		asst("m3", "claude-opus-5", "2026-08-10T09:00:01Z", 0, 0, 0, 9000))
	old := since.Add(-48 * time.Hour)
	os.Chtimes(stale, old, old)

	rep := ScanCosts("", since)
	if len(rep.Beads) != 1 || rep.Beads[0].Bead != "new-1" {
		t.Fatalf("want only the in-window segment, got %+v", rep.Beads)
	}
	if got := rep.Beads[0].CostUSD; got != 2000*25/1e6 {
		t.Errorf("in-window cost %v", got)
	}
}

// A receipt that cannot be read in full must say so. ADR 0018 §3 made the
// GATING path honest (dispatch parks, govern G6 marks a floor); `posse cost`
// renders the same report, and a day whose transcript root is unreadable
// printed $0.00 as if counted. Both arms here: the line appears when the
// scan reported a read failure, and does not when it did not.
func TestCostReportPrintNamesUnreadableTranscripts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mk := func() *CostReport {
		s := &Segment{Bead: "a-1", Model: "claude-sonnet-5", Start: time.Now(), Msgs: map[string]*Usage{"a": {Model: "claude-sonnet-5"}}}
		s.CostUSD = 4
		return &CostReport{Beads: []*Segment{s}, PassCap: 30, DayCap: 250}
	}
	render := func(r *CostReport) string {
		var b strings.Builder
		r.Print(&b)
		return b.String()
	}

	// The without-arm first: a clean scan claims a total, not a floor.
	clean := render(mk())
	for _, unwanted := range []string{"unreadable", "at least", "floor"} {
		if strings.Contains(clean, unwanted) {
			t.Errorf("a clean scan must not hedge, found %q:\n%s", unwanted, clean)
		}
	}
	if !strings.Contains(clean, "budget_day $250.00 (spent $4.00 today)") {
		t.Errorf("clean report lost its day spend:\n%s", clean)
	}

	rep := mk()
	rep.noteUnread(fmt.Errorf("open /x/session.jsonl: permission denied"))
	rep.noteUnread(fmt.Errorf("open /x/other.jsonl: permission denied"))
	got := render(rep)
	// Count, first error, and what the omission means — the shape
	// dispatch's stderr witness already uses (dispatch.go budget()).
	for _, want := range []string{
		"unreadable: 2 transcript(s) unreadable",
		"open /x/session.jsonl: permission denied",
		"the ledger counts less than was spent",
		"every total above is a floor",
		// The Dial E footer measures the day against a cap; when the scan
		// is partial that reading is a floor too.
		"budget_day $250.00 (spent at least $4.00 today)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("degraded report missing %q:\n%s", want, got)
		}
	}
	// The second failure is counted, not printed: one error, not a wall.
	if strings.Contains(got, "other.jsonl") {
		t.Errorf("only the first error is printed:\n%s", got)
	}
}

// $CLAUDE_CONFIG_DIR moves claude's store, and posse's three other readers
// of it — trust.go's config file, sentline.go's history.jsonl and
// credential.go's credentials file — follow it. This walk stayed at
// ~/.claude until ranger-base-yqdov, so under an override it walked a root
// that does not exist, and an absent root is "never ran the CLI" by design
// (TestScanCostsMissingRootIsNoRecords below): $0 of claude spend, no error
// and no uncounted line — the ADR 0018 §3 collapse ranger-base-z65xu fixed
// for grok and codex, here on the one runtime that carries dollars.
//
// The wrong arm is real here: $HOME holds no .claude at all, so a walk that
// ignores the override finds nothing and reports no error.
func TestClaudeTranscriptsFollowClaudeConfigDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.claude: the pre-fix walk sees an empty box
	moved := filepath.Join(t.TempDir(), "claude-elsewhere")
	t.Setenv("CLAUDE_CONFIG_DIR", moved)
	dir := filepath.Join(moved, "projects", "-Users-x-src-posse")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeTranscript(t, dir, "s.jsonl",
		`{"type":"user","timestamp":"2026-09-05T09:00:00Z","message":{"content":"Work beads issue rangerhq-ccd1 (title)"}}`,
		asst("m1", "claude-opus-5", "2026-09-05T09:00:01Z", 0, 0, 0, 40_000))

	files, errs := claudeCost{}.Transcripts("")
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(files) != 1 || files[0] != p {
		t.Fatalf("a transcript under $CLAUDE_CONFIG_DIR must be listed: %v (want [%s])", files, p)
	}
	// Counted, not merely located: the dollars have to reach the report,
	// because $0 wearing "nothing was spent"'s clothes is the whole defect.
	if got := ScanCosts("", time.Time{}).ByBead()["rangerhq-ccd1"]; got < 0.9999 || got > 1.0001 {
		t.Errorf("spend under $CLAUDE_CONFIG_DIR = %v, want 1.00 (0 is the silent $0 this bead fixes)", got)
	}
}

// An EMPTY $CLAUDE_CONFIG_DIR is not a config dir: the walk falls back to
// the home's .claude rather than rooting at a relative "projects" under
// whatever cwd the process launched in. Same rule ClaudeConfigDirIn holds
// for every other reader of the variable, and the same divergence from the
// runtime's own `??` that its doc comment records.
func TestClaudeTranscriptsEmptyConfigDirFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	dir := filepath.Join(home, ".claude", "projects", "p1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeTranscript(t, dir, "s.jsonl",
		`{"type":"user","timestamp":"2026-09-05T09:00:00Z","message":{"content":"Work beads issue rangerhq-ccd2 (title)"}}`)

	files, errs := claudeCost{}.Transcripts("")
	if len(errs) != 0 || len(files) != 1 || files[0] != p {
		t.Fatalf("empty override must fall back to <home>/.claude: %v %v (want [%s])", files, errs, p)
	}
}
