//go:build posse_arm2

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

// Across files the reading must come from the newest DAY. The within-a-day
// half of that ordering is a separate claim and cannot be seen here — this
// test's answer comes from 08-30, so the 08-15 pair below is fixture, not
// measurement; the test after this one measures it.
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

// Within one day the reading comes from the newest FILE — rolloutsInDayDesc's
// own ordering, which no test above reaches: every one of them is answered by
// a different day, so dropping that reverse sort left the whole suite green.
// The rollout filename embeds the session's ISO-8601 start time, which is why
// a lexical reverse sort is a chronological one, and codex opens several
// sessions on a working day, so this is the ordinary case rather than an edge.
func TestReadCodexPlanHintNewestFileWithinOneDay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codexRollout(t, home, "15", "rollout-2026-08-15T09-00-00-a.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-15T09:00:01.000Z", janJunRateLimits(2, 2, "plus")))
	codexRollout(t, home, "15", "rollout-2026-08-15T18-00-00-b.jsonl",
		codexMeta("/w"), codexRateLimitsLine("2026-08-15T18:00:01.000Z", janJunRateLimits(3, 3, "plus")))

	hint := ReadCodexPlanHint()
	if hint == nil || hint.Windows[0].UsedPercent != 3 {
		t.Fatalf("hint = %+v, want the newest FILE in the day (18:00's 3%%), not 09:00's 2%%", hint)
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

// ─── the display half (ADR 0034 D3) ──────────────────────────────────────────

// hintAt builds a reading taken at `at` with the windows given as
// name/percent/reset triples, so the tests below read as the rendering rules
// they are and not as JSON.
func hintAt(at time.Time, w ...HintWindow) *PlanHint { return &PlanHint{Windows: w, At: at} }

func hintWin(name string, pct float64, resets time.Time) HintWindow {
	return HintWindow{Name: name, UsedPercent: pct, ResetsAt: resets}
}

// The shape ADR 0034 D3 names, end to end: the provider word once, the
// window names without their config-key prefix, and the age — always.
func TestPlanHintSegment(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	live := now.Add(2 * time.Hour)
	for _, c := range []struct {
		name string
		hint *PlanHint
		want string
	}{{
		name: "no reading draws nothing",
		hint: nil,
	}, {
		name: "a reading with no windows is not a reading",
		hint: hintAt(now.Add(-time.Hour)),
	}, {
		name: "the ADR's own example",
		hint: hintAt(now.Add(-3*time.Hour), hintWin("codex_7d", 62, live)),
		want: "codex 7d 62%, as of 3h00m ago",
	}, {
		name: "both windows, in the reader's order",
		hint: hintAt(now.Add(-4*time.Minute), hintWin("codex_5h", 12, live), hintWin("codex_7d", 34, live)),
		want: "codex 5h 12% · 7d 34%, as of 4m ago",
	}, {
		// The rule this segment does NOT share with the claude line, which
		// prints an age only once it is a minute old (PlanCache.Line): here
		// the age is the load-bearing half, so a seconds-old hint says so
		// rather than presenting itself as timeless.
		name: "a seconds-old reading still shows its age",
		hint: hintAt(now.Add(-9*time.Second), hintWin("codex_7d", 62, live)),
		want: "codex 7d 62%, as of 9s ago",
	}, {
		name: "a reading taken this instant still shows its age",
		hint: hintAt(now, hintWin("codex_7d", 62, live)),
		want: "codex 7d 62%, as of 0s ago",
	}, {
		// Past its own resets_at the percent is a number about a window
		// that has since rolled over. It is never shown; the window says so.
		name: "a window past its reset shows no percent",
		hint: hintAt(now.Add(-9*time.Hour), hintWin("codex_7d", 96, now.Add(-time.Minute))),
		want: "codex 7d reset, as of 9h00m ago",
	}, {
		name: "one window reset, the other still live",
		hint: hintAt(now.Add(-90*time.Minute), hintWin("codex_5h", 88, now.Add(-time.Second)), hintWin("codex_7d", 34, live)),
		want: "codex 5h reset · 7d 34%, as of 1h30m ago",
	}, {
		name: "every window reset is still a reading, and still a line",
		hint: hintAt(now.Add(-time.Hour), hintWin("codex_5h", 88, now.Add(-time.Hour)), hintWin("codex_7d", 96, now.Add(-time.Minute))),
		want: "codex 5h reset · 7d reset, as of 1h00m ago",
	}, {
		// The boundary belongs to the reset arm: at resets_at the window HAS
		// rolled over, and the percent beside it is the previous window's.
		name: "exactly at resets_at is reset",
		hint: hintAt(now.Add(-time.Hour), hintWin("codex_7d", 62, now)),
		want: "codex 7d reset, as of 1h00m ago",
	}, {
		// A missing resets_at decodes to the epoch, which is past every
		// clock this will ever run under — the arm that shows no stale
		// percent, which is the direction to fail in.
		name: "a window with no resets_at shows no percent",
		hint: hintAt(now.Add(-time.Hour), hintWin("codex_7d", 62, time.Unix(0, 0))),
		want: "codex 7d reset, as of 1h00m ago",
	}, {
		// codexWindowName's fallback arm reaches the header intact: a window
		// duration nobody has seen is named honestly rather than dropped.
		name: "an unseen window duration renders under its own name",
		hint: hintAt(now.Add(-time.Minute), hintWin("codex_90m", 5, live)),
		want: "codex 90m 5%, as of 1m ago",
	}, {
		name: "percentages are whole, like the claude line's",
		hint: hintAt(now.Add(-time.Minute), hintWin("codex_7d", 62.4, live)),
		want: "codex 7d 62%, as of 1m ago",
	}, {
		// Nothing mangles a name the prefix rule does not match.
		name: "an unprefixed name is printed whole",
		hint: hintAt(now.Add(-time.Minute), hintWin("7d", 62, live)),
		want: "codex 7d 62%, as of 1m ago",
	}} {
		if got := c.hint.Segment(now); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// The reading on disk, through the shipped reader, rendered. The table above
// hand-builds its hints and would stay green if ReadCodexPlanHint and
// Segment disagreed about the window names — this is the seam between them.
func TestPlanHintSegmentRendersWhatTheReaderRead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	at := time.Date(2026, 8, 5, 10, 0, 1, 0, time.UTC)
	codexRollout(t, home, "05", "rollout-2026-08-05T10-00-00-a.jsonl",
		codexMeta("/w"),
		codexRateLimitsLine(at.Format(time.RFC3339Nano), janJunRateLimits(12, 34, "plus")),
	)
	// Before both of the fixture's resets_at values (1788136455 =
	// 2026-08-31T00:34:15Z for the 5h window, 1788697627 =
	// 2026-09-06T12:27:07Z for the 7d one), so both windows are still the
	// ones that were measured.
	now := at.Add(20 * time.Minute)
	if got, want := ReadCodexPlanHint().Segment(now), "codex 5h 12% · 7d 34%, as of 20m ago"; got != want {
		t.Errorf("Segment over the shipped reader:\n got %q\nwant %q", got, want)
	}
	// The same reading read past both of those: neither percent survives
	// into the header, and the age says how far past.
	past := time.Unix(1788697627, 0).Add(time.Second)
	if got, want := ReadCodexPlanHint().Segment(past), "codex 5h reset · 7d reset, as of 770h27m ago"; got != want {
		t.Errorf("Segment past both resets:\n got %q\nwant %q", got, want)
	}
}

// A box that has never run codex has nothing to say, and says nothing: the
// nil the reader returns renders as no bytes at all, which is what lets
// every caller render unconditionally.
func TestPlanHintSegmentIsEmptyWhereCodexNeverRan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := ReadCodexPlanHint().Segment(time.Now()); got != "" {
		t.Errorf("no reading must draw nothing, got %q", got)
	}
}

// $CODEX_HOME moves codex's store, and posse's two other readers of it —
// the cost adapter and the interstitial version probe — follow it. This
// walk stayed at ~/.codex until ranger-base-yqdov, so under an override it
// read a root that does not exist, and this type has exactly one answer for
// that: nil, which every caller reads as "no rollout on this machine ever
// wrote a reading" and turns into cap-only. Milder than the cost adapter's
// version of the same defect (a hint informs and never gates, ADR 0034 D1)
// and never a false dollar, but it is the same disagreement inside one
// binary and it silently drops a reading the operator has.
//
// The wrong arm is real here: $HOME holds no .codex at all, so a walk that
// ignores the override finds nothing and returns nil.
func TestReadCodexPlanHintFollowsCodexHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no ~/.codex: the pre-fix walk sees an empty box
	moved := filepath.Join(t.TempDir(), "codex-elsewhere")
	t.Setenv("CODEX_HOME", moved)
	dir := filepath.Join(moved, "sessions", "2026", "08", "05")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := joinLines([]string{
		codexMeta("/w"),
		codexRateLimitsLine("2026-08-05T10:00:01.000Z", janJunRateLimits(12, 34, "plus")),
	})
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-08-05T10-00-00-a.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	hint := ReadCodexPlanHint()
	if hint == nil {
		t.Fatal("a reading under $CODEX_HOME must be found (nil is the silent cap-only this bead fixes)")
	}
	if len(hint.Windows) != 2 || hint.Windows[0].UsedPercent != 12 || hint.Windows[1].UsedPercent != 34 {
		t.Errorf("windows: %+v", hint.Windows)
	}
}
