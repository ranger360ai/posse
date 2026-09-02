package posse

// Hermetic tests for the grok pool guard (rangerhq-myso).
//
// Nothing here reaches xAI, and nothing here needs to: the whole point of
// this meter is that it is a local file read. The fixtures are grok's own
// `updates.jsonl` records under a temp $HOME, and the assertion is what a
// dispatch pass did with the number they add up to.

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// grokPoolTurn is one turn_completed at a chosen instant. It carries the
// modelUsage breakdown grok really writes — which restates the same spend and
// is the 2× trap (cost_grok.go) — so the dollars this guard trips on are
// measured through the same duplication production sees.
func grokPoolTurn(ts time.Time, promptID string, ticks int64) string {
	rec := map[string]any{
		"timestamp": ts.Unix(),
		"method":    "session/update",
		"params": map[string]any{
			"update": map[string]any{
				"sessionUpdate": "turn_completed",
				"prompt_id":     promptID,
				"usage": map[string]any{
					"inputTokens":  1000,
					"outputTokens": 100,
					"costUsdTicks": ticks,
					"modelUsage": map[string]any{
						"grok-4.6-build": map[string]any{"costUsdTicks": ticks},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(rec)
	return string(b) + "\n"
}

func grokPoolUser(ts time.Time, text string) string {
	rec := map[string]any{
		"timestamp": ts.Unix(),
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

// grokPoolHome gives the caller its own $HOME and returns it. Anything that
// plants a grok session must take one: the pool reading is the SUM over
// every session under $HOME/.grok, and after ADR 0047 D1 the home is one
// temp directory for the whole test binary — so a fixture that plants into
// it reads back whatever every other test planted too. Measured on the run
// that first shared the home: five of these read 100%, 180% and 200% of a
// pool their own fixtures spend a fraction of.
//
// Setting the environment is also what keeps the caller SERIAL, by the
// runtime's own rule and with no list to maintain — which is the answer ADR
// 0047 D3 names for a test that writes into the shared home.
func grokPoolHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// grokPoolSession plants one session transcript. The session id is a
// parameter because the pool is the sum over MANY sessions, which is the
// thing being measured.
func grokPoolSession(t *testing.T, home, id, body string) string {
	t.Helper()
	dir := filepath.Join(home, ".grok", "sessions", url.PathEscape("/tmp/proj"), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "updates.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// usd renders a dollar amount as grok's nano-dollar ticks.
func usdTicks(d float64) int64 { return int64(d * 1e9) }

// The clock every pass here runs on: Thursday 2026-08-27 12:00 local, three
// days after a `mon 09:00` reset.
var grokPoolNow = time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local)

// grokPoolReset is the reset every fixture configures, and grokPoolLastReset
// the instant it resolves to under grokPoolNow.
const grokPoolReset = "mon 09:00"

var grokPoolLastReset = time.Date(2026, 8, 24, 9, 0, 0, 0, time.Local)

// grokPoolCfg is the guard, fully configured. The conversion factor is a
// ROUND TEST NUMBER and deliberately not the operator's calibration: $0.50
// per point makes a full pool exactly $50, so every assertion below is
// arithmetic a reader can check, and no measured figure enters this repo.
const grokPoolCfg = "grok_pool_reset: " + grokPoolReset + "\ngrok_pool_usd_per_point: 0.50\n"

type grokPoolFixture struct {
	d    *Dispatcher
	errb *strings.Builder
	b    *HerdrBackend
	fake string
	home string
}

// grokPoolPass wires a pass whose ready bead routes to a persona pinned to
// grok, with no plan guard armed at all — the pool guard is the only brake
// in the way, which is what makes the skip lines below unambiguous.
func grokPoolPass(t *testing.T, cfg string) *grokPoolFixture {
	t.Helper()
	return grokPoolPassOn(t, cfg, "runtime: grok\n")
}

// grokPoolPassOn is the same pass with the persona's runtime line under the
// caller's control, so "a claude bead is not gated by grok's pool" is one
// character of difference from the case that is.
func grokPoolPassOn(t *testing.T, cfg, runtimeLine string) *grokPoolFixture {
	t.Helper()
	return grokPoolPassFull(t, cfg, runtimeLine, `["go"]`)
}

// grokPoolPassFull adds the ready bead's labels, for the one case that needs
// a tier on them: ADR 0010 will not move `strong` work to a second pool.
func grokPoolPassFull(t *testing.T, cfg, runtimeLine, beadLabels string) *grokPoolFixture {
	t.Helper()
	home := grokPoolHome(t)
	b, fake := newTestBackend(t)
	d, errb := planDispatcher(t, b, nil)
	d.Now = func() time.Time { return grokPoolNow }
	os.MkdirAll(b.App.AgentsDir, 0o755)
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\n" + runtimeLine + "---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":`+beadLabels+`}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, cfg)
	idleClaude(t, fake)
	return &grokPoolFixture{d: d, errb: errb, b: b, fake: fake, home: home}
}

// spend plants one session's worth of dollars inside the current week.
func (f *grokPoolFixture) spend(t *testing.T, id string, dollars float64) {
	t.Helper()
	at := grokPoolLastReset.Add(time.Hour)
	grokPoolSession(t, f.home, id,
		grokPoolUser(at, "Work beads issue rangerhq-myso (t)")+
			grokPoolTurn(at, "p-"+id, usdTicks(dollars)))
}

func (f *grokPoolFixture) run(t *testing.T) (int, string) {
	t.Helper()
	n, err := f.d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	return n, dispatcherOut(f.d)
}

// ─── the reset arithmetic ────────────────────────────────────────────────

func TestParseWeeklyReset(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want WeeklyReset
		bad  string
	}{
		{raw: "mon 09:00", want: WeeklyReset{time.Monday, 9, 0}},
		{raw: "Monday 09:00", want: WeeklyReset{time.Monday, 9, 0}},
		{raw: "  SAT 23:59 ", want: WeeklyReset{time.Saturday, 23, 59}},
		{raw: "thu 0:05", want: WeeklyReset{time.Thursday, 0, 5}},
		{raw: "", bad: "two fields"},
		{raw: "mon", bad: "two fields"},
		{raw: "mon 09:00 utc", bad: "two fields"},
		{raw: "moonday 09:00", bad: "not a weekday"},
		{raw: "mon 0900", bad: "not HH:MM"},
		{raw: "mon 24:00", bad: "time of day"},
		{raw: "mon 09:60", bad: "time of day"},
		{raw: "mon ab:cd", bad: "time of day"},
	} {
		got, err := ParseWeeklyReset(tc.raw)
		if tc.bad != "" {
			if err == nil || !strings.Contains(err.Error(), tc.bad) {
				t.Errorf("ParseWeeklyReset(%q) = %v, %v; want an error naming %q", tc.raw, got, err, tc.bad)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseWeeklyReset(%q) = %v, %v; want %v", tc.raw, got, err, tc.want)
		}
	}
}

// Last is the window boundary, and the two cases that decide a whole week's
// worth of dollars are both on reset day: before the reset time the week is
// still the old one, at it and after it the week is fresh.
func TestWeeklyResetLast(t *testing.T) {
	t.Parallel()
	r := WeeklyReset{Day: time.Monday, Hour: 9}
	for _, tc := range []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"midweek", time.Date(2026, 8, 27, 12, 0, 0, 0, time.Local),
			time.Date(2026, 8, 24, 9, 0, 0, 0, time.Local)},
		{"reset day, before the minute", time.Date(2026, 8, 24, 8, 59, 0, 0, time.Local),
			time.Date(2026, 8, 17, 9, 0, 0, 0, time.Local)},
		{"reset day, at the minute", time.Date(2026, 8, 24, 9, 0, 0, 0, time.Local),
			time.Date(2026, 8, 24, 9, 0, 0, 0, time.Local)},
		{"reset day, after the minute", time.Date(2026, 8, 24, 9, 0, 1, 0, time.Local),
			time.Date(2026, 8, 24, 9, 0, 0, 0, time.Local)},
		{"the day before", time.Date(2026, 8, 23, 23, 59, 0, 0, time.Local),
			time.Date(2026, 8, 17, 9, 0, 0, 0, time.Local)},
	} {
		if got := r.Last(tc.now); !got.Equal(tc.want) {
			t.Errorf("%s: Last(%s) = %s, want %s", tc.name, tc.now, got, tc.want)
		}
	}
}

// ─── the reading ─────────────────────────────────────────────────────────

// The pool is metered over the WEEK, so dollars spent before the last reset
// are somebody else's week. Without that boundary the $40 below would put
// this pass at 140% and skip the bead.
func TestGrokPoolCountsOnlySpendSinceTheReset(t *testing.T) {
	f := grokPoolPass(t, "grok_guard_week: 70\n"+grokPoolCfg)
	before := grokPoolLastReset.Add(-2 * time.Hour)
	grokPoolSession(t, f.home, "old",
		grokPoolUser(before, "Work beads issue rangerhq-old (t)")+
			grokPoolTurn(before, "p-old", usdTicks(40)))
	f.spend(t, "new", 10) // 10/0.50 = 20%

	n, out := f.run(t)
	if n != 1 {
		t.Fatalf("last week's spend must not gate this week, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "estimated 20% of the weekly pool used") {
		t.Errorf("want 20%%, the post-reset spend alone:\n%s", out)
	}
}

// The operator's OWN grok sessions come out of the same pool, so they count.
// This session is interactive — no work prompt, no bead — and it is 80% of
// the pool on its own.
func TestGrokPoolCountsInteractiveSessionsToo(t *testing.T) {
	f := grokPoolPass(t, "grok_guard_week: 70\n"+grokPoolCfg)
	at := grokPoolLastReset.Add(time.Hour)
	grokPoolSession(t, f.home, "operator",
		grokPoolUser(at, "what is the airspeed velocity")+
			grokPoolTurn(at, "p-op", usdTicks(40)))

	n, out := f.run(t)
	if n != 0 || !strings.Contains(out, "estimated 80% of the weekly pool used > grok_guard_week: 70%") {
		t.Fatalf("the operator's own grok spends the same pool, got n=%d:\n%s", n, out)
	}
}

// ─── the guard ───────────────────────────────────────────────────────────

// Unset is today's behaviour, exactly: the bead launches and nothing is
// said. A pool at 200% changes nothing, because nothing reads it.
func TestGrokPoolUnarmedIsSilentAndLaunches(t *testing.T) {
	f := grokPoolPass(t, grokPoolCfg)
	f.spend(t, "s1", 100)

	n, out := f.run(t)
	if n != 1 {
		t.Fatalf("an unarmed guard gates nothing, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "grok pool") || strings.Contains(f.errb.String(), "grok pool") {
		t.Errorf("an unarmed guard says nothing:\nout=%s\nerr=%s", out, f.errb.String())
	}
}

// Over the threshold, the bead gets a line instead of a launch — and the
// line says ESTIMATED, because this number is not the vendor's.
func TestGrokPoolOverThresholdSkipsTheBead(t *testing.T) {
	f := grokPoolPass(t, "grok_guard_week: 70\n"+grokPoolCfg)
	f.spend(t, "s1", 30)
	f.spend(t, "s2", 10) // 40/0.50 = 80%

	n, out := f.run(t)
	if n != 0 {
		t.Fatalf("want the bead parked at 80%% > 70%%, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "grok pool: estimated 80% of the weekly pool used > grok_guard_week: 70% — skipped") {
		t.Errorf("want the skip line naming the estimate and the threshold:\n%s", out)
	}
	if calls := bdCalls(t, f.fake); strings.Contains(calls, "--claim") {
		t.Errorf("a parked bead must not be claimed, got: %s", calls)
	}
}

// AT the threshold the bead still runs — planGuard's rule, strictly above —
// and the pass logs the reading with the conversion factor it used, which is
// the one input that goes stale with nothing failing.
func TestGrokPoolAtThresholdRunsAndLogsTheFactor(t *testing.T) {
	f := grokPoolPass(t, "grok_guard_week: 80\n"+grokPoolCfg)
	f.spend(t, "s1", 40) // exactly 80%

	n, out := f.run(t)
	if n != 1 {
		t.Fatalf("at the threshold the bead still runs, got n=%d:\n%s", n, out)
	}
	for _, want := range []string{
		"estimated 80% of the weekly pool used",
		"$40.00 since Mon 2026-08-24 09:00",
		"$0.5000 per point",
		"a FLOOR",
		"grok_guard_week: 80%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the pass must log %q:\n%s", want, out)
		}
	}
}

// ADR 0013 §3: a meter gates only the work that can spend it. A drained grok
// pool must not park a claude lane.
func TestGrokPoolDoesNotGateAnotherRuntime(t *testing.T) {
	f := grokPoolPassOn(t, "grok_guard_week: 70\n"+grokPoolCfg, "")
	f.spend(t, "s1", 100) // 200% of the pool

	n, out := f.run(t)
	if n != 1 {
		t.Fatalf("grok's pool says nothing about a claude bead, got n=%d:\n%s", n, out)
	}
	if strings.Contains(out, "grok pool") {
		t.Errorf("a pass with no grok bead in it takes no reading at all:\n%s", out)
	}
}

// Armed with a missing input, the guard is OFF and says so — never blind,
// never parked. Local files have no outage to wait out, so there is no
// reading that would ever arrive to lift a brake set here.
func TestGrokPoolHalfConfiguredIsOffAndLoud(t *testing.T) {
	for _, tc := range []struct {
		name, cfg, want string
	}{
		{"no reset", "grok_guard_week: 70\ngrok_pool_usd_per_point: 0.50\n",
			"grok_pool_reset: is unset or unusable"},
		{"no factor", "grok_guard_week: 70\ngrok_pool_reset: " + grokPoolReset + "\n",
			"grok_pool_usd_per_point: is unset or unusable"},
		{"neither", "grok_guard_week: 70\n",
			"grok_pool_reset: and grok_pool_usd_per_point: are both unset or unusable"},
		{"malformed reset", "grok_guard_week: 70\ngrok_pool_reset: someday\ngrok_pool_usd_per_point: 0.50\n",
			"grok_pool_reset: is unset or unusable"},
		{"malformed factor", "grok_guard_week: 70\ngrok_pool_reset: " + grokPoolReset + "\ngrok_pool_usd_per_point: lots\n",
			"grok_pool_usd_per_point: is unset or unusable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := grokPoolPass(t, tc.cfg)
			f.spend(t, "s1", 100)

			n, out := f.run(t)
			if n != 1 {
				t.Fatalf("an unusable guard parks nothing, got n=%d:\n%s", n, out)
			}
			err := f.errb.String()
			if !strings.Contains(err, tc.want) || !strings.Contains(err, "OFF, not blind") {
				t.Errorf("want stderr naming %q and the OFF state, got: %q", tc.want, err)
			}
		})
	}
}

// A threshold that is not a percent is a typo, and a typo must be visible —
// not a guard that quietly stopped guarding.
func TestGrokGuardWeekMalformedIsNamedAndOff(t *testing.T) {
	for _, raw := range []string{"lots", "0", "-5", "101"} {
		t.Run(raw, func(t *testing.T) {
			f := grokPoolPass(t, "grok_guard_week: "+raw+"\n"+grokPoolCfg)
			f.spend(t, "s1", 100)

			n, out := f.run(t)
			if n != 1 {
				t.Fatalf("a malformed threshold gates nothing, got n=%d:\n%s", n, out)
			}
			if !strings.Contains(f.errb.String(), "grok_guard_week: "+`"`+raw+`"`+" is not a percent") {
				t.Errorf("want the typo named on stderr, got: %q", f.errb.String())
			}
		})
	}
}

// An unreadable transcript is spend nobody can count, and this guard says so
// without failing closed on it: the estimate is already a floor by
// construction, and a session file that cannot be read is not an outage a
// later pass recovers from.
func TestGrokPoolUnreadableTranscriptIsNamedNotFatal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads an unreadable file")
	}
	f := grokPoolPass(t, "grok_guard_week: 70\n"+grokPoolCfg)
	f.spend(t, "readable", 30) // 60%, under the threshold
	f.spend(t, "sealed", 10)
	sealed := filepath.Join(f.home, ".grok", "sessions", url.PathEscape("/tmp/proj"), "sealed", "updates.jsonl")
	if err := os.Chmod(sealed, 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(sealed); err == nil {
		t.Skip("this filesystem does not enforce the mode; nothing was blocked")
	}

	n, out := f.run(t)
	if n != 1 {
		t.Fatalf("an unreadable transcript must not park the bead, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "1 transcript(s) unreadable") {
		t.Errorf("the reading must name what it could not count:\n%s", out)
	}
	if !strings.Contains(out, "estimated 60% of the weekly pool used") {
		t.Errorf("what it COULD read still counts:\n%s", out)
	}
}

// ADR 0010's ladder ends where this meter begins. A tripped plan guard would
// step this bead over to grok — and grok's own week is gone, so it is skipped
// there instead, by the pool guard and not by the bead cap. Without this the
// overflow becomes a way to drain the one pool whose exhaustion lasts days.
func TestGrokPoolStopsAnOverflowMoveOntoADrainedPool(t *testing.T) {
	f := grokPoolPassFull(t,
		"plan_guard_5h: 70\nplan_guard_overflow: grok\nplan_guard_overflow_cap: 5\n"+
			"grok_guard_week: 70\n"+grokPoolCfg,
		"", `["go","tier:standard"]`)
	f.d.Plan = newPlanServer(t, 78, 40).reader()
	f.spend(t, "s1", 40) // 80% of the week

	n, out := f.run(t)
	if n != 0 {
		t.Fatalf("a drained grok pool must refuse the overflow move, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "grok pool: estimated 80% of the weekly pool used > grok_guard_week: 70% — skipped") {
		t.Errorf("the pool guard, not the bead cap, is what says no:\n%s", out)
	}
	if b, err := os.ReadFile(f.b.App.OverflowLogPath()); err == nil {
		t.Errorf("a bead that never launched writes no overflow ledger line: %q", b)
	}
}
