package posse

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// grokTurn renders one turn_completed record the way grok writes it —
// including the modelUsage breakdown that restates the same spend, which is
// the whole point of these tests.
func grokTurn(promptID string, ticks int64, in, cachedR, out int) string {
	rec := map[string]any{
		"timestamp": 1787054470,
		"method":    "session/update",
		"params": map[string]any{
			"update": map[string]any{
				"sessionUpdate": "turn_completed",
				"prompt_id":     promptID,
				"usage": map[string]any{
					"inputTokens":         in,
					"outputTokens":        out,
					"cachedReadTokens":    cachedR,
					"cacheCreationTokens": 0,
					"reasoningTokens":     0,
					"costUsdTicks":        ticks,
					"modelUsage": map[string]any{
						"grok-4.6-build": map[string]any{
							"inputTokens":  in,
							"outputTokens": out,
							"costUsdTicks": ticks, // sums to the aggregate — the 2x trap
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

func grokUser(text string) string {
	rec := map[string]any{
		"timestamp": 1787054460,
		"method":    "session/update",
		"params": map[string]any{
			"update": map[string]any{
				"sessionUpdate": "user_message_chunk",
				"content":       map[string]any{"type": "text", "text": text},
				"_meta":         map[string]any{"modelId": "grok-4.6"},
			},
		},
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

func writeGrokSession(t *testing.T, home, cwd string, body string) string {
	t.Helper()
	return writeGrokSessionIn(t, filepath.Join(home, ".grok"), cwd, body)
}

// writeGrokSessionIn writes the same store under an explicit grok HOME, for
// the arm that moves it with $GROK_HOME.
func writeGrokSessionIn(t *testing.T, grokHome, cwd string, body string) string {
	t.Helper()
	dir := filepath.Join(grokHome, "sessions", url.PathEscape(cwd), "01a0-session")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "updates.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The defect this adapter exists to avoid: grok's usage carries both an
// aggregate costUsdTicks and a modelUsage breakdown that sums to exactly the
// same number. Reading both reports exactly twice the real spend.
func TestGrokDecodeIgnoresModelUsageBreakdown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	body := grokUser("Work beads issue rangerhq-vojc (title)") +
		grokTurn("p1", 1_000_000_000, 1000, 100, 50) + // $1.00
		grokTurn("p2", 500_000_000, 500, 50, 25) // $0.50

	p := writeGrokSession(t, home, "/src/posse", body)
	segs, err := grokCost{}.Decode(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 {
		t.Fatalf("want 1 segment, got %d", len(segs))
	}
	s := segs[0]
	if s.Bead != "rangerhq-vojc" {
		t.Errorf("bead = %q", s.Bead)
	}
	// $1.50, not $3.00. A decoder that also walked modelUsage would get 3.00.
	if got := s.CostUSD; got < 1.4999 || got > 1.5001 {
		t.Errorf("cost = %v, want 1.50 (3.00 means modelUsage was double-counted)", got)
	}
	if !s.ProviderPriced {
		t.Error("grok segments must be provider-priced, never table-priced")
	}
	if s.Turns() != 2 {
		t.Errorf("turns = %d, want 2 (one usage record per prompt_id)", s.Turns())
	}
}

// Per-turn totals SUM. The bead that ordered this work described grok as
// cumulative-max; the records on this grok are per-prompt totals, and a max
// would silently drop every turn but the priciest.
func TestGrokPerTurnCostsSumRatherThanMax(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	body := grokUser("Work beads issue rangerhq-aaaa (t)") +
		grokTurn("p1", 300_000_000, 10, 0, 1) +
		grokTurn("p2", 100_000_000, 10, 0, 1) +
		grokTurn("p3", 200_000_000, 10, 0, 1)
	p := writeGrokSession(t, home, "/src/posse", body)
	segs, err := grokCost{}.Decode(p, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 {
		t.Fatalf("segments: %d", len(segs))
	}
	// 0.30+0.10+0.20 = 0.60. A max-per-session reading would report 0.30.
	if got := segs[0].CostUSD; got < 0.5999 || got > 0.6001 {
		t.Errorf("cost = %v, want 0.60 (0.30 means a max was taken)", got)
	}
}

// NoteCumulative is still the right call for a provider that genuinely
// restates a running total — max, never sum (ADR 0012 D4).
func TestNoteCumulativeTakesMaxNotSum(t *testing.T) {
	t.Parallel()
	s := &Segment{Msgs: map[string]*Usage{}}
	for _, v := range []float64{1, 3, 7, 7, 2} { // a restated running total
		s.NoteCumulative(v)
	}
	if _, c := s.Total(); c != 7 {
		t.Errorf("cumulative total = %v, want 7 (20 means it summed)", c)
	}
}

func TestGrokTranscriptsFilterByDecodedCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeGrokSession(t, home, "/Users/x/src/posse", grokUser("hi"))
	writeGrokSession(t, home, "/Users/x/src/other", grokUser("hi"))

	all, err := grokCost{}.Transcripts("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("unfiltered: %d", len(all))
	}
	only, err := grokCost{}.Transcripts("src/posse")
	if err != nil {
		t.Fatal(err)
	}
	if len(only) != 1 {
		t.Fatalf("filtered by decoded cwd: %d", len(only))
	}
}

// The defect this bead fixes: filepath.Glob ignores every I/O error by
// contract, so a session directory posse cannot read used to vanish from
// the listing with errs empty — "cannot tell" read as "nothing was spent"
// (ADR 0018 §3, ranger-base-yljd). A second, readable session is the floor
// that must still come back.
func TestGrokUnreadableSessionDirIsAReadFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; this arm needs an unprivileged uid")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeGrokSession(t, home, "/Users/x/src/posse", grokUser("hi"))
	sealed := filepath.Dir(writeGrokSession(t, home, "/Users/x/src/other", grokUser("hi")))
	if err := os.Chmod(sealed, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sealed, 0o755) })

	files, errs := grokCost{}.Transcripts("")
	if len(errs) == 0 {
		t.Fatal("an unreadable session directory must not read as no spend")
	}
	if len(files) != 1 {
		t.Fatalf("the readable session is still the floor: %v", files)
	}
}

// $GROK_HOME moves grok's store, and posse's two other readers of it — the
// interstitial version probe and FindGrokTurnOutcome — follow it. This walk
// stayed at ~/.grok until ranger-base-z65xu, so under an override it walked
// a root that does not exist, and an absent root is "never ran grok" by
// design (the pin below): $0 of grok spend, no error and no uncounted line,
// which is the no-spend-vs-cannot-tell collapse this seam exists to prevent.
//
// The wrong arm is real here: $HOME holds no .grok at all, so a walk that
// ignores the override finds nothing and reports no error.
func TestGrokTranscriptsFollowGrokHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.grok: the pre-fix walk sees an empty box
	moved := filepath.Join(t.TempDir(), "grok-elsewhere")
	t.Setenv("GROK_HOME", moved)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "never-run")) // ScanCosts reads every provider
	p := writeGrokSessionIn(t, moved, "/Users/x/src/posse",
		grokUser("Work beads issue rangerhq-vojc (title)")+grokTurn("p1", 1_000_000_000, 1000, 100, 50))

	files, errs := grokCost{}.Transcripts("")
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if len(files) != 1 || files[0] != p {
		t.Fatalf("a store under $GROK_HOME must be listed: %v (want [%s])", files, p)
	}
	// Counted, not merely located: the dollars have to reach the report.
	if got := ScanCosts("", time.Time{}).ByBead()["rangerhq-vojc"]; got < 0.9999 || got > 1.0001 {
		t.Errorf("spend under $GROK_HOME = %v, want 1.00 (0 is the silent $0 this bead fixes)", got)
	}
}

// A machine that has never run grok has nothing to count; that is not a read
// failure and must not park a scan (ADR 0018 §3).
func TestGrokMissingRootIsNoRecordsNotAnError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	files, err := grokCost{}.Transcripts("")
	if err != nil {
		t.Fatalf("missing root must not be an error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files: %v", files)
	}
}

// grok is registered, so it is counted rather than reported uncounted.
func TestGrokIsARegisteredProvider(t *testing.T) {
	t.Parallel()
	if _, ok := CostProviderFor("grok"); !ok {
		t.Fatal("grok must have an adapter")
	}
	if _, ok := CostProviderFor("codex"); !ok {
		t.Fatal("codex must have an adapter (rangerhq-0va)")
	}
	// The other half of the comma-ok, and the half that has to keep working
	// as adapters are added: a runtime nobody wrote one for is NOT found, so
	// its sessions stay uncounted rather than landing in a total as $0.
	if _, ok := CostProviderFor("gemini"); ok {
		t.Fatal("a runtime with no adapter must not resolve to one")
	}
	if _, ok := CostProviderFor("claude"); !ok {
		t.Fatal("claude must have an adapter")
	}
}

// A record that burned no tokens costs a known zero whatever its model id is.
// Claude Code's `<synthetic>` notices are all of this shape, and counting
// them as "unpriced" would print a floor warning on every report forever.
func TestZeroTokenRecordIsAKnownZeroNotAnUnpricedGap(t *testing.T) {
	t.Parallel()
	s := &Segment{Msgs: map[string]*Usage{
		"syn":  {Model: "<synthetic>"},
		"real": {Model: "claude-opus-5", In: 1e6},
	}}
	if _, c := s.Total(); c != 5 {
		t.Errorf("cost = %v, want 5", c)
	}
	if s.Unpriced != 0 {
		t.Errorf("Unpriced = %d, want 0 — a zero-token record is not a gap", s.Unpriced)
	}
	// But a real unknown model that DID burn tokens is a gap.
	s2 := &Segment{Msgs: map[string]*Usage{"x": {Model: "mystery", Out: 1000}}}
	if _, c := s2.Total(); c != 0 {
		t.Errorf("unpriced cost = %v, want 0 (never guessed)", c)
	}
	if s2.Unpriced != 1 {
		t.Errorf("Unpriced = %d, want 1", s2.Unpriced)
	}
}
