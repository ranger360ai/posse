package posse

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The rate_limits fixtures below are the shapes measured live on the
// operator's box 2026-08-31 (grep '"rate_limits"' across every rollout
// under ~/.codex/sessions): primary=300/secondary=10080 with plan_type
// "plus" or "team" (Jan–Jun 2026), primary=10080/secondary=null with
// plan_type "plus" (Aug 2026), and the empty shell
// {"primary":null,"secondary":null,"credits":null,"plan_type":null} codex
// writes before a session's first turn.

func codexRateLimitsLine(ts string, rateLimits map[string]any) string {
	return codexLine(ts, "event_msg", map[string]any{
		"type": "token_count",
		"info": map[string]any{
			"total_token_usage": codexUsage(100, 0, 0, 10, 0),
		},
		"rate_limits": rateLimits,
	})
}

func codexRollout(t *testing.T, home, day, name string, lines ...string) string {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "08", day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(joinLines(lines)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func joinLines(lines []string) string {
	var out string
	for _, l := range lines {
		out += l
	}
	return out
}

// primaryOnly is the Jan–Jun 2026 shape: primary 5h, secondary 7d, both
// present.
func janJunRateLimits(fivePct, sevenPct float64, planType string) map[string]any {
	return map[string]any{
		"limit_id": "codex", "limit_name": nil,
		"primary":   map[string]any{"used_percent": fivePct, "window_minutes": 300, "resets_at": 1788136455},
		"secondary": map[string]any{"used_percent": sevenPct, "window_minutes": 10080, "resets_at": 1788697627},
		"credits":   map[string]any{"has_credits": false, "unlimited": false, "balance": "0"},
		"plan_type": planType, "spend_control_reached": nil,
	}
}

// augRateLimits is the Aug 2026 shape: window_minutes reshuffled so the
// 7d window is primary and there is no secondary at all.
func augRateLimits(pct float64) map[string]any {
	return map[string]any{
		"limit_id": "codex", "limit_name": nil,
		"primary":   map[string]any{"used_percent": pct, "window_minutes": 10080, "resets_at": 1787630024},
		"secondary": nil,
		"credits":   map[string]any{"has_credits": false, "unlimited": false, "balance": "0"},
		"plan_type": "plus", "spend_control_reached": nil,
	}
}

// emptyShellRateLimits is codex's own zero value before a session's first
// turn: every field null, not a reading at all.
func emptyShellRateLimits() map[string]any {
	return map[string]any{"primary": nil, "secondary": nil, "credits": nil, "plan_type": nil}
}

func TestReadCodexPlanHintJanJunShape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "05", "rollout-2026-08-05T10-00-00-a.jsonl",
		codexMeta("/w"),
		codexRateLimitsLine("2026-08-05T10:00:01.000Z", janJunRateLimits(12, 34, "plus")),
	)
	hint := ReadCodexPlanHint()
	if hint == nil {
		t.Fatal("nil hint, want a reading")
	}
	if len(hint.Windows) != 2 {
		t.Fatalf("windows: %+v, want 2", hint.Windows)
	}
	if hint.Windows[0].Name != "codex_5h" || hint.Windows[0].UsedPercent != 12 {
		t.Errorf("primary window: %+v", hint.Windows[0])
	}
	if hint.Windows[1].Name != "codex_7d" || hint.Windows[1].UsedPercent != 34 {
		t.Errorf("secondary window: %+v", hint.Windows[1])
	}
	if !hint.Windows[0].ResetsAt.Equal(time.Unix(1788136455, 0)) {
		t.Errorf("primary resets_at: %v", hint.Windows[0].ResetsAt)
	}
	if hint.Credits.Balance != "0" {
		t.Errorf("credits balance: %q", hint.Credits.Balance)
	}
}

func TestReadCodexPlanHintAugShapeNoSecondary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "30", "rollout-2026-08-30T09-00-00-a.jsonl",
		codexMeta("/w"),
		codexRateLimitsLine("2026-08-30T09:00:01.000Z", augRateLimits(96)),
	)
	hint := ReadCodexPlanHint()
	if hint == nil {
		t.Fatal("nil hint, want a reading")
	}
	if len(hint.Windows) != 1 {
		t.Fatalf("windows: %+v, want 1 (no secondary in this shape)", hint.Windows)
	}
	if hint.Windows[0].Name != "codex_7d" || hint.Windows[0].UsedPercent != 96 {
		t.Errorf("window: %+v, want codex_7d 96%% — window_minutes drift must not relabel by SLOT", hint.Windows[0])
	}
}

func TestReadCodexPlanHintPlanTypeDriftDoesNotBreakParsing(t *testing.T) {
	for _, planType := range []string{"team", "plus"} {
		home := t.TempDir()
		t.Setenv("HOME", home)
		codexRollout(t, home, "10", "rollout-2026-08-10T09-00-00-a.jsonl",
			codexMeta("/w"),
			codexRateLimitsLine("2026-08-10T09:00:01.000Z", janJunRateLimits(5, 6, planType)),
		)
		hint := ReadCodexPlanHint()
		if hint == nil || len(hint.Windows) != 2 {
			t.Fatalf("plan_type %q: hint=%+v, want a 2-window reading regardless of plan_type", planType, hint)
		}
	}
}

func TestReadCodexPlanHintNoRateLimitsAtAll(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "01", "rollout-2026-08-01T09-00-00-a.jsonl",
		codexMeta("/w"),
		codexTurnCtx("2026-08-01T09:00:01Z", "gpt-5.6-sol"),
		codexUser("2026-08-01T09:00:02Z", "hello"),
	)
	if hint := ReadCodexPlanHint(); hint != nil {
		t.Errorf("hint = %+v, want nil — no rate_limits event exists on this box", hint)
	}
}

func TestReadCodexPlanHintNeverUsed(t *testing.T) {
	home := t.TempDir() // no ~/.codex at all
	t.Setenv("HOME", home)
	if hint := ReadCodexPlanHint(); hint != nil {
		t.Errorf("hint = %+v, want nil — codex was never used here", hint)
	}
}

// The empty shell codex writes before a session's first turn —
// {"primary":null,"secondary":null,"credits":null,"plan_type":null} —
// measured live on this box. It is not a reading and must be treated
// exactly like a line with no rate_limits key at all, not like a reading
// of two absent windows.
func TestReadCodexPlanHintEmptyShellIsNotAReading(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "02", "rollout-2026-08-02T09-00-00-a.jsonl",
		codexMeta("/w"),
		codexRateLimitsLine("2026-08-02T09:00:01.000Z", emptyShellRateLimits()),
	)
	if hint := ReadCodexPlanHint(); hint != nil {
		t.Errorf("hint = %+v, want nil — the empty shell carries no reading", hint)
	}
}

// Within one file the reading must be the LAST (newest) rate_limits event,
// not the first.
func TestReadCodexPlanHintNewestEventWithinFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "15", "rollout-2026-08-15T09-00-00-a.jsonl",
		codexMeta("/w"),
		codexRateLimitsLine("2026-08-15T09:00:01.000Z", janJunRateLimits(1, 2, "plus")),
		codexRateLimitsLine("2026-08-15T09:05:00.000Z", janJunRateLimits(9, 10, "plus")),
	)
	hint := ReadCodexPlanHint()
	if hint == nil || hint.Windows[0].UsedPercent != 9 {
		t.Fatalf("hint = %+v, want the LAST event's 9%%, not the first's 1%%", hint)
	}
}

// Across files the reading must come from the newest DAY and, within a
// day, the newest FILE — never the oldest.
func TestReadCodexPlanHintNewestFileAcrossDays(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "01", "rollout-2026-08-01T09-00-00-a.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-01T09:00:01.000Z", janJunRateLimits(1, 1, "plus")))
	codexRollout(t, home, "15", "rollout-2026-08-15T09-00-00-a.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-15T09:00:01.000Z", janJunRateLimits(2, 2, "plus")))
	codexRollout(t, home, "15", "rollout-2026-08-15T18-00-00-b.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-15T18:00:01.000Z", janJunRateLimits(3, 3, "plus")))
	codexRollout(t, home, "30", "rollout-2026-08-30T09-00-00-a.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-30T09:00:01.000Z", janJunRateLimits(4, 4, "plus")))

	hint := ReadCodexPlanHint()
	if hint == nil || hint.Windows[0].UsedPercent != 4 {
		t.Fatalf("hint = %+v, want the newest day's (08-30) 4%%", hint)
	}
}

// A newer, EMPTY day directory (a session that never turned) must not hide
// an older day's real reading — the scan keeps walking backward past a day
// with no usable event.
func TestReadCodexPlanHintSkipsNewerDayWithNoReading(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "20", "rollout-2026-08-20T09-00-00-a.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-20T09:00:01.000Z", janJunRateLimits(7, 7, "plus")))
	// A newer session that has not turned yet: only session_meta.
	codexRollout(t, home, "25", "rollout-2026-08-25T09-00-00-a.jsonl",
		codexMeta("/w"))

	hint := ReadCodexPlanHint()
	if hint == nil || hint.Windows[0].UsedPercent != 7 {
		t.Fatalf("hint = %+v, want the 08-20 reading (08-25 carries none)", hint)
	}
}

// At is the event's own timestamp, never the file's mtime (ADR 0034 D1) —
// a rollout keeps getting appended to well after its own creation, so the
// file's mtime tracks the session's last write, not this reading's.
func TestReadCodexPlanHintAtIsEventTimestampNotFileMtime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := codexRollout(t, home, "12", "rollout-2026-08-12T09-00-00-a.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-12T09:00:01.000Z", janJunRateLimits(5, 5, "plus")))

	future := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}

	hint := ReadCodexPlanHint()
	want := time.Date(2026, 8, 12, 9, 0, 1, 0, time.UTC)
	if hint == nil || !hint.At.Equal(want) {
		t.Fatalf("At = %v, want the event's own timestamp %v (file mtime was set to %v)", hint.At, want, future)
	}
}

// spend_control_reached and the credits block must survive round-tripping
// even when a reading has no secondary window at all.
func TestReadCodexPlanHintSpendControlReached(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	rl := augRateLimits(100)
	rl["spend_control_reached"] = true
	rl["credits"] = map[string]any{"has_credits": true, "unlimited": false, "balance": nil}
	codexRollout(t, home, "30", "rollout-2026-08-30T09-00-00-a.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-30T09:00:01.000Z", rl))

	hint := ReadCodexPlanHint()
	if hint == nil {
		t.Fatal("nil hint")
	}
	if !hint.SpendControlReached {
		t.Error("SpendControlReached = false, want true")
	}
	if !hint.Credits.HasCredits {
		t.Error("Credits.HasCredits = false, want true")
	}
	if hint.Credits.Balance != "" {
		t.Errorf("Credits.Balance = %q, want empty — this reading's balance is null", hint.Credits.Balance)
	}
}
