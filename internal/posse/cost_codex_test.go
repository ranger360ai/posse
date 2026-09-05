package posse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// NOTES.md's `posse cost` section is the bead's other deliverable: it must
// state exactly what is and is not priced. A doc that quotes a rendering is
// only worth its quote while the renderer still produces it, so this asserts
// the agreement rather than the doc's existence — and it asserts the ADAPTER
// LIST against the registry, so a fourth adapter cannot ship with the section
// still naming three.
func TestNotesCostSectionAgreesWithWhatCostPrints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	b, err := os.ReadFile(filepath.Join("..", "..", "NOTES.md"))
	if err != nil {
		t.Fatal(err)
	}
	notes := string(b)

	mk := func(bead, runtime, model string) *Segment {
		s := &Segment{Bead: bead, Runtime: runtime, Model: model, Start: time.Now(),
			Msgs: map[string]*Usage{"m": {Model: model, In: 1000, Out: 200}}}
		s.Total()
		return s
	}
	rep := &CostReport{Beads: []*Segment{
		mk("a-1", "claude", "claude-opus-5"),
		mk("c-1", "codex", "gpt-5.6-sol"),
	}}
	var out strings.Builder
	rep.Print(&out)

	// Every phrase NOTES quotes must be one the report actually renders.
	for _, quoted := range []string{
		"sum is a floor, median and per-bead are over the",
		"no bead here has a rate",
	} {
		if !strings.Contains(notes, quoted) {
			t.Errorf("NOTES.md no longer quotes %q", quoted)
		}
		if !strings.Contains(out.String(), quoted) {
			t.Errorf("the report no longer renders %q, which NOTES.md quotes:\n%s", quoted, out.String())
		}
	}
	// And the section names every counted runtime, so it cannot go on
	// describing a shorter list than the registry has.
	sec := notes[strings.Index(notes, "`posse cost [--since <date>]"):]
	sec = sec[:strings.Index(sec, "`posse agent new")]
	for _, rt := range CountedRuntimes() {
		if !strings.Contains(sec, "**"+rt+"**") {
			t.Errorf("the NOTES cost section does not name the %s adapter", rt)
		}
	}
}

// The codex rollout fixture. Every record here is the shape codex 0.147.0
// actually writes — checked against the 163 rollouts on the operator's box on
// 2026-08-28 — because a fixture that only matches the decoder proves the
// decoder matches the fixture and nothing else.

func codexMeta(cwd string) string {
	return codexLine("2026-08-26T12:16:17.792Z", "session_meta", map[string]any{
		"session_id": "01a03dff", "cwd": cwd, "cli_version": "0.147.0",
	})
}

func codexTurnCtx(ts, model string) string {
	return codexLine(ts, "turn_context", map[string]any{"cwd": "/w", "model": model})
}

func codexUser(ts, text string) string {
	return codexLine(ts, "response_item", map[string]any{
		"type": "message", "role": "user",
		"content": []any{map[string]any{"type": "input_text", "text": text}},
	})
}

// codexDev is the PID codex injects before the work prompt — role "developer",
// never "user", so it must never open a segment.
func codexDev(ts, text string) string {
	return codexLine(ts, "response_item", map[string]any{
		"type": "message", "role": "developer",
		"content": []any{map[string]any{"type": "input_text", "text": text}},
	})
}

// codexUsage is one usage block: input_tokens INCLUDES cached_input_tokens,
// and reasoning_output_tokens is a SUBSET of output_tokens (both measured).
func codexUsage(in, cached, cacheW, out, reason int) map[string]any {
	return map[string]any{
		"input_tokens": in, "cached_input_tokens": cached,
		"cache_write_input_tokens": cacheW, "output_tokens": out,
		"reasoning_output_tokens": reason, "total_tokens": in + out,
	}
}

// codexCount is a token_count event carrying the running total and this
// turn's delta, exactly as codex writes both.
func codexCount(ts string, total, last map[string]any) string {
	return codexLine(ts, "event_msg", map[string]any{
		"type": "token_count",
		"info": map[string]any{
			"total_token_usage": total, "last_token_usage": last,
			"model_context_window": 258400,
		},
		"rate_limits": map[string]any{"limit_id": "codex", "plan_type": "plus"},
	})
}

func codexLine(ts, typ string, payload map[string]any) string {
	b, _ := json.Marshal(map[string]any{"timestamp": ts, "type": typ, "payload": payload})
	return string(b) + "\n"
}

func writeCodexRollout(t *testing.T, home string, lines ...string) string {
	t.Helper()
	return writeCodexRolloutIn(t, filepath.Join(home, ".codex"), lines...)
}

// writeCodexRolloutIn writes the same rollout under an explicit codex HOME,
// for the arm that moves it with $CODEX_HOME.
func writeCodexRolloutIn(t *testing.T, codexHome string, lines ...string) string {
	t.Helper()
	dir := filepath.Join(codexHome, "sessions", "2026", "08", "26")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "rollout-2026-08-26T08-16-17-01a03dff.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "")), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// THE defect this adapter exists to avoid, and the one the bead got wrong.
//
// codex re-emits a token_count with an IDENTICAL snapshot a fraction of a
// second later — on 100 of 163 local rollouts, 15 of them written by the
// version running today. Summing `last_token_usage`, which the bead offered
// as an alternative discipline, therefore reports ~2x the real spend.
//
// The fixture carries a duplicate after each real turn and keeps
// last_token_usage populated so the wrong reading is expressible: the test
// asserts both that the report is right AND that the naive sum would have
// been different, so the fixture can never quietly degenerate into one where
// the two readings agree.
func TestCodexDuplicateSnapshotsAreCountedOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t1 := codexUsage(1000, 0, 0, 100, 40)
	t2 := codexUsage(3000, 800, 0, 250, 90)
	p := writeCodexRollout(t, home,
		codexMeta("/w"),
		codexTurnCtx("2026-08-26T12:16:18Z", "gpt-5.6-sol"),
		codexUser("2026-08-26T12:16:19Z", `Work beads issue x-1 (title, quoted as data: "t"). Run bd show x-1`),
		codexCount("2026-08-26T12:16:25Z", t1, t1),
		codexCount("2026-08-26T12:16:25.9Z", t1, t1), // the duplicate
		codexCount("2026-08-26T12:16:37Z", t2, codexUsage(2000, 800, 0, 150, 50)),
		codexCount("2026-08-26T12:16:37.8Z", t2, codexUsage(2000, 800, 0, 150, 50)), // and again
	)
	segs, err := codexCost{}.Decode(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].Bead != "x-1" {
		t.Fatalf("segments: %+v", segs)
	}
	s := segs[0]
	if s.Turns() != 2 {
		t.Errorf("turns %d, want 2 — a duplicate snapshot is not a turn", s.Turns())
	}
	// In excludes the cached reads it contains; Out excludes nothing,
	// because reasoning is already inside output.
	u := s.Sum()
	if u.In != 2200 || u.CacheR != 800 || u.Out != 250 || u.CacheW != 0 {
		t.Errorf("tokens %+v, want in 2200 / cache_r 800 / out 250", u)
	}
	// The invariant that made this discipline checkable on real data: the
	// deltas sum to the session's final total_token_usage.
	if got, want := u.In+u.CacheR+u.Out, 3250; got != want {
		t.Errorf("reconstructed total %d, want the final snapshot's %d", got, want)
	}
	// The control: the reading this adapter refuses really is a different
	// number on this fixture.
	if naive := 1000 + 100 + 1000 + 100 + 2000 + 150 + 2000 + 150; naive == 3250 {
		t.Fatal("fixture no longer distinguishes summing last_token_usage from the delta reading")
	}
}

// A session that works two beads must not charge the second one the first
// one's spend. This is what rules out maxing the cumulative snapshot per
// segment — dedupe-proof, but it cannot attribute.
func TestCodexChargesEachBeadOnlyItsOwnDelta(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := writeCodexRollout(t, home,
		codexMeta("/w"),
		codexTurnCtx("2026-08-26T12:16:18Z", "gpt-5.6-sol"),
		codexUser("2026-08-26T12:16:19Z", "Work beads issue x-1: first"),
		codexCount("2026-08-26T12:16:25Z", codexUsage(1000, 0, 0, 100, 40), nil),
		codexCount("2026-08-26T12:16:37Z", codexUsage(3000, 800, 0, 250, 90), nil),
		codexUser("2026-08-26T12:20:00Z", "Work beads issue x-2: second"),
		codexCount("2026-08-26T12:21:00Z", codexUsage(5000, 1500, 0, 400, 120), nil),
	)
	segs, err := codexCost{}.Decode(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 {
		t.Fatalf("segments: %+v", segs)
	}
	if u := segs[0].Sum(); u.In != 2200 || u.CacheR != 800 || u.Out != 250 {
		t.Errorf("x-1 tokens %+v", u)
	}
	// The whole point: x-2's own delta, not the running total at its end.
	// A max-the-snapshot reading would say in 3500 / cache_r 1500 / out 400.
	if u := segs[1].Sum(); u.In != 1300 || u.CacheR != 700 || u.Out != 150 {
		t.Errorf("x-2 tokens %+v, want in 1300 / cache_r 700 / out 150 — it must not inherit x-1's spend", u)
	}
	if segs[1].Turns() != 1 {
		t.Errorf("x-2 turns %d", segs[1].Turns())
	}
	if segs[0].Model != "gpt-5.6-sol" {
		t.Errorf("model %q — turn_context names it", segs[0].Model)
	}
}

// The baseline is the SESSION's running total, not the segment's: a snapshot
// dropped by --since must still advance it, or the next segment is charged
// for the turn that snapshot reported.
func TestCodexSinceStillAdvancesTheBaseline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := writeCodexRollout(t, home,
		codexMeta("/w"),
		codexUser("2026-08-26T09:00:00Z", "Work beads issue old-1: before the window"),
		codexCount("2026-08-26T09:00:10Z", codexUsage(1000, 0, 0, 100, 0), nil),
		codexUser("2026-08-26T13:00:00Z", "Work beads issue new-1: inside it"),
		codexCount("2026-08-26T13:00:10Z", codexUsage(2500, 0, 0, 300, 0), nil),
	)
	segs, err := codexCost{}.Decode(p, time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].Bead != "new-1" {
		t.Fatalf("segments: %+v", segs)
	}
	if u := segs[0].Sum(); u.In != 1500 || u.Out != 200 {
		t.Errorf("tokens %+v, want in 1500 / out 200 — the dropped turn is not new-1's", u)
	}
}

// A counter that restarts mid-session is the delta, not a negative: clamping
// to zero there would silently drop everything after the restart.
func TestCodexSnapshotResetIsCountedNotDropped(t *testing.T) {
	t.Parallel()
	prev := codexTokens{In: 3000, Cached: 800, Out: 250, Total: 3250}
	cur := codexTokens{In: 400, Cached: 100, Out: 30, Total: 430}
	if d := cur.since(prev); d != cur {
		t.Errorf("reset delta %+v, want the snapshot itself %+v", d, cur)
	}
	if d := (codexTokens{In: 3400, Cached: 900, Out: 300, Total: 3700}).since(prev); d.In != 400 || d.Cached != 100 || d.Out != 50 || d.Total != 450 {
		t.Errorf("forward delta %+v", d)
	}
}

// codex's PID arrives as a role "developer" message. Only role "user" opens a
// segment, or every dispatched session would report an interactive stretch
// that was really its own persona definition.
func TestCodexDeveloperMessagesOpenNoSegment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := writeCodexRollout(t, home,
		codexMeta("/w"),
		codexDev("2026-08-26T12:16:19Z", "You are the Developer of the Ranger crew."),
		codexCount("2026-08-26T12:16:20Z", codexUsage(500, 0, 0, 10, 0), nil),
		codexUser("2026-08-26T12:16:21Z", "Work beads issue x-1: t"),
		codexCount("2026-08-26T12:16:25Z", codexUsage(1500, 0, 0, 110, 0), nil),
	)
	segs, err := codexCost{}.Decode(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].Bead != "x-1" {
		t.Fatalf("segments: %+v", segs)
	}
}

// A codex bead has no price and no provider-reported money, so its dollars
// are UNKNOWN. The report must print a blank there, name the runtime in the
// legend, and never report it as $0.00 — and a claude bead beside it must
// still print its number (the without-arm).
func TestCodexBeadsPrintABlankDollarNotZero(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cx := &Segment{Bead: "c-1", Runtime: "codex", Model: "gpt-5.6-sol", Persona: "dev",
		Start: time.Date(2026, 8, 26, 9, 0, 0, 0, time.Local),
		Msgs:  map[string]*Usage{"turn-0": {Model: "gpt-5.6-sol", In: 1000, Out: 200}}}
	cl := &Segment{Bead: "a-1", Runtime: "claude", Model: "claude-opus-5", Persona: "qa",
		Start: time.Date(2026, 8, 26, 9, 0, 0, 0, time.Local),
		Msgs:  map[string]*Usage{"m1": {Model: "claude-opus-5", In: 1000, Out: 200}}}
	cx.Total()
	cl.Total()
	if cx.Priced() {
		t.Fatal("a codex segment has no measured dollars")
	}
	if !cl.Priced() {
		t.Fatal("a claude segment does")
	}
	rep := &CostReport{Beads: []*Segment{cl, cx}}
	var b strings.Builder
	rep.Print(&b)
	out := b.String()

	for _, want := range []string{
		"runtime", // the per-bead column header
		"c-1              2026-08-26 09:00 dev      codex",
		"a-1              2026-08-26 09:00 qa       claude",
		"by runtime (the pool the work was paid out of)",
		"codex            1 bead(s)  sum        —",
		"legend: \"—\" in api$ means the runtime reported no cost",
		"(codex)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
	// The dishonesty this replaces: a zero in the money column, and a codex
	// bead averaged into a statistic as if it had cost nothing.
	if strings.Contains(out, "codex    ?            1        0        0          0    0.00") {
		t.Errorf("codex bead printed $0.00:\n%s", out)
	}
	if !strings.Contains(out, "1 bead segments:") {
		t.Errorf("only the priced bead may enter the summary statistics:\n%s", out)
	}
	// claude's own row still carries its number.
	if !strings.Contains(out, "claude           1 bead(s)  sum $") {
		t.Errorf("claude group lost its dollars:\n%s", out)
	}
}

// A bead worked on BOTH runtimes keeps its number: it is a floor, not a
// blank. This is not hypothetical — the live ledger has bead ids appearing on
// a claude segment and a codex segment on the same day, and calling such a
// bead unmeasured would throw away the dollars that WERE measured.
func TestABeadWorkedOnBothRuntimesIsAFloorNotABlank(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mk := func(bead, runtime, model string) *Segment {
		s := &Segment{Bead: bead, Runtime: runtime, Model: model, Start: time.Now(),
			Msgs: map[string]*Usage{"m": {Model: model, In: 1000, Out: 200}}}
		s.Total()
		return s
	}
	rep := &CostReport{Beads: []*Segment{
		mk("both-1", "claude", "claude-opus-5"),
		mk("both-1", "codex", "gpt-5.6-sol"),
		mk("only-1", "codex", "gpt-5.6-sol"),
	}}
	blank := rep.BlankBeads()
	if blank["both-1"] {
		t.Error("a bead with one priced segment has a number — a floor, not a blank")
	}
	if !blank["only-1"] {
		t.Error("a bead with no priced segment at all has no number")
	}
	if got := rep.ByBead()["both-1"]; got <= 0 {
		t.Errorf("the measured half must survive: %v", got)
	}
}

// The scan stamps the runtime from the adapter, so no decoder can forget it
// and a mixed day is readable per bead.
func TestScanStampsTheRuntimeOnEverySegment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cdir := filepath.Join(home, ".claude", "projects", "p1")
	os.MkdirAll(cdir, 0o755)
	writeTranscript(t, cdir, "s.jsonl",
		`{"type":"user","timestamp":"2026-08-26T09:00:00Z","message":{"content":"Work beads issue a-1: t"}}`,
		asst("m1", "claude-opus-5", "2026-08-26T09:00:05Z", 1000, 0, 0, 100))
	writeCodexRollout(t, home,
		codexMeta("/w"),
		codexUser("2026-08-26T12:16:19Z", "Work beads issue c-1: t"),
		codexCount("2026-08-26T12:16:25Z", codexUsage(1000, 0, 0, 100, 0), nil))

	rep := ScanCosts("", time.Time{})
	got := map[string]string{}
	for _, s := range rep.Beads {
		got[s.Bead] = s.Runtime
	}
	if got["a-1"] != "claude" || got["c-1"] != "codex" {
		t.Fatalf("runtimes %v, want a-1 claude and c-1 codex", got)
	}
	if rep.CountUnpriced() == 0 {
		t.Error("the codex turn is unpriced, and the report must say the total is a floor")
	}
}

// The locator: a missing root is no records (a machine that never ran codex),
// the project filter reads each session's recorded cwd, and a rollout whose
// first record names no cwd is an unknown rather than a silent drop.
func TestCodexTranscriptsLocator(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	files, errs := codexCost{}.Transcripts("")
	if len(files) != 0 || len(errs) != 0 {
		t.Fatalf("no ~/.codex is no records, not a fault: %v %v", files, errs)
	}
	p := writeCodexRollout(t, home, codexMeta("/Users/d/src/posse"), codexUser("2026-08-26T12:16:19Z", "hi"))
	files, errs = codexCost{}.Transcripts("src/posse")
	if len(files) != 1 || files[0] != p || len(errs) != 0 {
		t.Errorf("matching project: %v %v", files, errs)
	}
	files, errs = codexCost{}.Transcripts("src/other")
	if len(files) != 0 || len(errs) != 0 {
		t.Errorf("non-matching project: %v %v", files, errs)
	}
	// A file whose first record is not session_meta cannot be placed.
	os.WriteFile(p, []byte(codexUser("2026-08-26T12:16:19Z", "hi")), 0o644)
	files, errs = codexCost{}.Transcripts("src/posse")
	if len(files) != 0 || len(errs) != 1 {
		t.Errorf("unplaceable rollout must be reported, not dropped: %v %v", files, errs)
	}
	// With no filter there is nothing to place, so nothing to fail on.
	files, errs = codexCost{}.Transcripts("")
	if len(files) != 1 || len(errs) != 0 {
		t.Errorf("unfiltered listing: %v %v", files, errs)
	}
}

// $CODEX_HOME moves codex's store, and posse's interstitial version probe
// follows it. This walk stayed at ~/.codex until ranger-base-z65xu, so under
// an override it walked a root that does not exist, and an absent root is
// "never ran codex" by design (the first arm of the locator pin above): no
// turns, no tokens, no error and no uncounted line — "nothing was spent"
// wearing "cannot tell"'s clothes (ADR 0018 §3).
//
// The wrong arm is real here: $HOME holds no .codex at all, so a walk that
// ignores the override finds nothing and reports no error.
func TestCodexTranscriptsFollowCodexHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.codex: the pre-fix walk sees an empty box
	moved := filepath.Join(t.TempDir(), "codex-elsewhere")
	t.Setenv("CODEX_HOME", moved)
	t.Setenv("GROK_HOME", filepath.Join(t.TempDir(), "never-run")) // ScanCosts reads every provider
	p := writeCodexRolloutIn(t, moved,
		codexMeta("/Users/d/src/posse"),
		codexUser("2026-08-26T12:16:19Z", "Work beads issue rangerhq-cdx1 (title)"),
		codexCount("2026-08-26T12:16:25Z", codexUsage(1000, 0, 0, 100, 0), nil))

	files, errs := codexCost{}.Transcripts("")
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(files) != 1 || files[0] != p {
		t.Fatalf("a rollout under $CODEX_HOME must be listed: %v (want [%s])", files, p)
	}
	// Counted, not merely located. codex is read-but-UNPRICED, so what must
	// arrive is the turn and its tokens, never a dollar.
	rep := ScanCosts("", time.Time{})
	var seg *Segment
	for _, s := range rep.Beads {
		if s.Bead == "rangerhq-cdx1" {
			seg = s
		}
	}
	if seg == nil {
		t.Fatal("the segment under $CODEX_HOME never reached the report (the silent no-spend this bead fixes)")
	}
	if u := seg.Sum(); seg.Turns() != 1 || u.In+u.Out != 1100 {
		t.Errorf("turns/tokens = %d/%d, want 1/1100", seg.Turns(), u.In+u.Out)
	}
}

// codex is a counted runtime now: registered, named by CountedRuntimes, and
// no longer counted as an uncounted live session.
func TestCodexIsARegisteredProvider(t *testing.T) {
	t.Parallel()
	p, ok := CostProviderFor("codex")
	if !ok || p.Runtime() != "codex" {
		t.Fatalf("codex adapter: %v %v", p, ok)
	}
	if _, priced := p.PriceFor("gpt-5.6-sol"); priced {
		t.Error("codex runs on a plan seat — no rate card may apply to it")
	}
	if !strings.Contains(strings.Join(CountedRuntimes(), "/"), "codex") {
		t.Errorf("counted runtimes: %v", CountedRuntimes())
	}
}
