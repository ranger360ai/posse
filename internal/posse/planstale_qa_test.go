//go:build posse_arm3

package posse

// QA pins for meter blindness being LOUD (ranger-base-lpoui, the operator's
// 2026-09-02 option-C ruling on ranger-base-wkai3).
//
// The defect was silence, so every pin here is about bytes that must be
// printed and a number that must be right: the threshold at its EDGE (a
// `<` for a `<=` is the whole mutation), the age arithmetic and the exact
// sentence it renders, the failure-class suffix and the streak behind it,
// and the governance key's hourly escalation — which has to change once an
// hour and NOT once a tick, because a key that changes every tick is a
// storm and a key that never changes is the ten hours of quiet this bead
// was filed for.
//
// Hermetic: files under a t.TempDir, a fixed clock, no endpoint, no
// credential. PlanStaleness makes no request by construction and these
// prove it by never providing one to make.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// lpouiT is the measured hour, kept as the fixture's now: the incident's
// snapshot was stamped 2026-09-02T03:23Z and the census was taken at
// 13:32Z, ten consecutive 429s later.
var lpouiT = time.Date(2026, 9, 2, 13, 32, 0, 0, time.UTC)

// staleApp is an app with the plan guard armed and nothing else: the
// staleness surface reads two files and a config, so that is all a rig for
// it may need.
func staleApp(t *testing.T, cfg string) *App {
	t.Helper()
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, guardOn+"\n"+cfg)
	return b.App
}

// seedReading writes the shared snapshot as if a reading had succeeded at
// `at` — the same store plancache.go's LastReading serves and G5's blind
// clock counts from.
func seedReading(t *testing.T, a *App, at time.Time, u PlanUsage) {
	t.Helper()
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(planEntry{At: at, Windows: u})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "plan-usage.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedReadLog writes $StateDir/plan-usage.log with the outcomes given,
// oldest first — the same three-field shape logRead writes, because the
// streak reader's whole contract is with that shape.
func seedReadLog(t *testing.T, a *App, outcomes ...string) {
	t.Helper()
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i, o := range outcomes {
		b.WriteString(lpouiT.Add(time.Duration(i-len(outcomes)) * time.Hour).UTC().Format(time.RFC3339))
		b.WriteString(" dispatch ")
		b.WriteString(o)
		b.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "plan-usage.log"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── the threshold, at its edge ──────────────────────────────────────────────

// `plan_usage_stale_after:` is a boundary and the pin is written at it, not
// near it: a reading exactly AT the threshold is not yet stale, one tick
// past it is. That pair is what a `<` mutated to a `<=` (or the reverse)
// has to fail, and a test that only checks "2h is quiet, 10h is loud"
// passes under both.
func TestQAPlanStaleThresholdIsAnEdge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  string
		age  time.Duration
		want bool
	}{
		{"default, a tick inside", "", 2*time.Hour - time.Second, false},
		{"default, exactly at", "", 2 * time.Hour, false},
		{"default, a tick past", "", 2*time.Hour + time.Second, true},
		{"configured, exactly at", "plan_usage_stale_after: 30m\n", 30 * time.Minute, false},
		{"configured, a tick past", "plan_usage_stale_after: 30m\n", 30*time.Minute + time.Second, true},
		{"bare seconds, a tick past", "plan_usage_stale_after: 1800\n", 30*time.Minute + time.Second, true},
		// 0 is the documented escape hatch, and it has to survive an age
		// nothing else in this file would tolerate.
		{"escape hatch, forty hours", "plan_usage_stale_after: 0\n", 40 * time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := staleApp(t, tc.cfg)
			seedReading(t, a, lpouiT.Add(-tc.age), fakeWindows(41, 89))
			if got := a.PlanStaleness("qa", lpouiT, os.Stderr).Stale; got != tc.want {
				t.Fatalf("age %s under %q: stale = %v, want %v", tc.age, tc.cfg, got, tc.want)
			}
		})
	}
}

// The three states that are not staleness, and each would make the line a
// lie. The guard-off row is the one that matters most: with no
// `plan_guard_<window>:` there is no headroom rule ruling on anything, and
// a cockpit that has written a snapshot for its own display must not be
// told the shop is ruling on it.
func TestQAPlanStaleQuietWhereTheLineWouldLie(t *testing.T) {
	t.Parallel()
	t.Run("guard off", func(t *testing.T) {
		t.Parallel()
		b, _ := newTestBackend(t)
		seedReading(t, b.App, lpouiT.Add(-10*time.Hour), fakeWindows(41, 89))
		if st := b.App.PlanStaleness("qa", lpouiT, os.Stderr); st.Stale {
			t.Fatalf("no plan_guard_<window>: is no headroom rule: %q", st.Line())
		}
	})
	t.Run("no reading ever taken", func(t *testing.T) {
		t.Parallel()
		a := staleApp(t, "")
		if st := a.PlanStaleness("qa", lpouiT, os.Stderr); st.Stale {
			t.Fatalf("a machine with no snapshot is ruling on nothing: %q", st.Line())
		}
	})
	t.Run("stamped in the future", func(t *testing.T) {
		t.Parallel()
		a := staleApp(t, "")
		seedReading(t, a, lpouiT.Add(time.Hour), fakeWindows(41, 89))
		if st := a.PlanStaleness("qa", lpouiT, os.Stderr); st.Stale {
			t.Fatalf("a clock step is a bad reading, not an old one: %q", st.Line())
		}
	})
}

// A typo is visible and the default stands — the plan guard's rule for
// every threshold it has, and the reason a mistyped key cannot silently
// disarm the loudest line in the shop.
func TestQAPlanStaleAfterTypoIsLoudAndDefaults(t *testing.T) {
	t.Parallel()
	a := staleApp(t, "plan_usage_stale_after: soonish\n")
	var errb strings.Builder
	if got := a.PlanUsageStaleAfter(&errb); got != PlanUsageStaleAfterDefault {
		t.Errorf("a typo must leave the default in force, got %s", got)
	}
	if !strings.Contains(errb.String(), "plan_usage_stale_after") || !strings.Contains(errb.String(), "soonish") {
		t.Errorf("the typo must name itself on stderr, got %q", errb.String())
	}
}

// ─── the age arithmetic, and the sentence it renders ─────────────────────────

// The measured hour, byte for byte. Age is now minus the moment the reading
// was TAKEN — never floored at process start, never aged forward — and the
// line quotes the reading's own timestamp beside it so a reader can check
// the subtraction without trusting it.
func TestQAPlanStaleLineIsTheMeasuredHour(t *testing.T) {
	t.Parallel()
	a := staleApp(t, "")
	at := lpouiT.Add(-(10*time.Hour + 9*time.Minute)) // 2026-09-02T03:23Z
	seedReading(t, a, at, PlanUsage{{Name: "5h", Pct: 41}, {Name: "7d", Pct: 89}})
	seedReadLog(t, a, "ok", "429 cooldown=5m", "429 cooldown=5m", "429 cooldown=5m")

	st := a.PlanStaleness("qa", lpouiT, os.Stderr)
	if !st.Stale {
		t.Fatal("ten hours is stale under any threshold this ships with")
	}
	if st.Age != 10*time.Hour+9*time.Minute {
		t.Errorf("age = %s, want the subtraction now - at", st.Age)
	}
	if !st.At.Equal(at) {
		t.Errorf("at = %s, want the snapshot's own stamp %s", st.At, at)
	}
	const want = "plan meter BLIND 10h09m: last reading 2026-09-02T03:23Z (5h 41% · 7d 89%) — " +
		"ruling on it under the headroom rule; 3 consecutive 429"
	if got := st.Line(); got != want {
		t.Errorf("the line is the deliverable:\n got %q\nwant %q", got, want)
	}
}

// ─── the failure-class suffix, and the streak under it ───────────────────────

// The tail clause, over the shapes the cadence log actually holds. The last
// two rows are the ones an operator would be misled by: a streak that ENDED
// (an `ok` at the tail) is not a streak, and a stale snapshot with no
// failed reads behind it means nothing has ASKED — a different outage from
// a rate limiter, and it must not be reported as one.
func TestQAPlanStaleStreakAndClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		log      []string
		fails    int
		class    string
		sentence string
	}{
		{"the measured ten", append([]string{"ok"}, repeatN("429 cooldown=5m", 10)...),
			10, "429", "10 consecutive 429"},
		{"a credential storm", []string{"ok",
			"failed: usage endpoint returned 401 Unauthorized: credential stale — run `claude` once to refresh [401]",
			"failed: usage endpoint returned 401 Unauthorized: credential stale — run `claude` once to refresh [401]"},
			2, "401", "2 consecutive 401"},
		{"not entitled", []string{"ok", "failed: usage endpoint returned 403 Forbidden: ... [403]"},
			1, "403", "1 consecutive 403"},
		{"our own gate", []string{"ok", "failed: deny: Bash(security:*) [gated]"},
			1, "gated", "1 consecutive gated"},
		// No class is not a fifth class: a dead socket says "failed reads",
		// because failed is exactly what it is.
		{"a dead socket", []string{"ok", "failed: usage endpoint unreachable", "failed: usage endpoint unreachable"},
			2, "", "2 consecutive failed reads"},
		{"the streak ended", []string{"429 cooldown=5m", "429 cooldown=5m", "ok"},
			0, "", "no request has left this machine since"},
		{"nothing has asked", nil,
			0, "", "no request has left this machine since"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := staleApp(t, "")
			seedReading(t, a, lpouiT.Add(-10*time.Hour), fakeWindows(41, 89))
			if tc.log != nil {
				seedReadLog(t, a, tc.log...)
			}
			st := a.PlanStaleness("qa", lpouiT, os.Stderr)
			if st.Fails != tc.fails || st.Class != tc.class {
				t.Fatalf("fails/class = %d/%q, want %d/%q", st.Fails, st.Class, tc.fails, tc.class)
			}
			if !strings.HasSuffix(st.Line(), tc.sentence) {
				t.Errorf("the tail clause is what names the outage:\n got %q\nwant suffix %q", st.Line(), tc.sentence)
			}
		})
	}
}

func repeatN(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// The class comes off the error's TYPE and never off its sentence. The last
// row is the control and the point of the whole pin: an error whose PROSE
// says 401 and whose type says nothing gets no token — a reader that
// grepped the words would undo the three types AuthFailure, RateLimit and
// GateRefusal were each created to keep.
func TestQAPlanFailTokenReadsTheTypeNotTheProse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"401", &AuthFailure{Status: "401 Unauthorized", Code: 401}, "401"},
		{"403", &AuthFailure{Status: "403 Forbidden", Code: 403}, "403"},
		{"429", &RateLimit{Status: "429 Too Many Requests"}, "429"},
		// A 503 that carried Retry-After is the same CLASS and a different
		// status, and filing it as a 429 would send an operator to the
		// wrong dashboard.
		{"503 with Retry-After", &RateLimit{Status: "503 Service Unavailable", RetryAfter: time.Minute}, "503"},
		// The hour a 429 bought: nobody asked the endpoint, so there is no
		// status to quote, and the class still has to be named the same way
		// the 429 was (rangerhq-pwpx).
		{"the cooldown a 429 bought", &planCooldownErr{Left: time.Minute}, "429"},
		{"nil", nil, ""},
		{"a dead socket", Die("usage endpoint unreachable"), ""},
		{"prose that says 401", Die("usage endpoint returned 401 Unauthorized"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := PlanFailToken(tc.err); got != tc.want {
				t.Errorf("PlanFailToken(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// ─── the governance key: hourly, and not per tick ────────────────────────────

// The escalation, and the storm it must not become. `guard-blind` is what
// the pulse fingerprints (govern.go GovCondition.Key), so the key changing
// is what re-reaches the coordinator: once an hour is the ruling, and every
// two minutes would be a fingerprint that re-prompts on every tick and
// restarts the renag clock each time.
func TestQAGuardBlindKeyEscalatesHourlyNotPerTick(t *testing.T) {
	t.Parallel()
	rl := &RateLimit{Status: "429 Too Many Requests"}
	for _, tc := range []struct {
		name  string
		blind time.Duration
		err   error
		want  string
	}{
		{"the zeroth hour", 45 * time.Minute, rl, "guard-blind:0h:429"},
		{"the hour before", 9*time.Hour + 59*time.Minute, rl, "guard-blind:9h:429"},
		{"the hour it turned", 10 * time.Hour, rl, "guard-blind:10h:429"},
		{"a tick later, unchanged", 10*time.Hour + 2*time.Minute, rl, "guard-blind:10h:429"},
		{"fifty-nine minutes later, still unchanged", 10*time.Hour + 59*time.Minute, rl, "guard-blind:10h:429"},
		{"an unclassed failure appends no token", 10 * time.Hour, Die("usage endpoint unreachable"), "guard-blind:10h"},
		// The credential fork is rangerhq-ytyj's and is untouched: a
		// coordinator handed this key runs one command, and that is the
		// whole reason it does not say "blind".
		{"a credential condition keeps its own key", 10 * time.Hour,
			&AuthFailure{Status: "401 Unauthorized", Code: 401}, "guard-credential:401"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key, detail := guardBlindRow(tc.blind, tc.err)
			if key != tc.want {
				t.Errorf("key = %q, want %q", key, tc.want)
			}
			if !strings.Contains(detail, BlindFor(tc.blind)) {
				t.Errorf("the DETAIL still carries the full age for a human: %q", detail)
			}
		})
	}
}

// The two neighbouring hours must produce DIFFERENT keys — stated as its
// own assertion because that inequality is the escalation, and a
// blindHours() mutated to return a constant passes every equality above.
func TestQAGuardBlindKeyDiffersAcrossTheHour(t *testing.T) {
	t.Parallel()
	rl := &RateLimit{Status: "429 Too Many Requests"}
	before, _ := guardBlindRow(9*time.Hour+59*time.Minute, rl)
	after, _ := guardBlindRow(10*time.Hour, rl)
	if before == after {
		t.Fatalf("the hour boundary must change the fingerprint, both %q — ten hours of one delivery is the bead", before)
	}
}

// ─── the log marker the streak reads back ────────────────────────────────────

// logRead appends the class as a token so planLogClass never has to read
// the sentence. Pinned with the bytes the 08-24 misdiagnosis pin greps for
// still present: the marker is APPENDED precisely so `<caller> failed: `
// stays where it was.
func TestQAPlanUsageLogMarksTheFailureClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		marker string
	}{
		{"401", &AuthFailure{Status: "401 Unauthorized", Code: 401}, " [401]"},
		{"429 keeps its own head field", &RateLimit{Status: "429 Too Many Requests"}, ""},
		{"a dead socket gets no token", Die("usage endpoint unreachable"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			c := &PlanCache{
				Path:   filepath.Join(dir, "plan-usage.json"),
				Log:    filepath.Join(dir, "plan-usage.log"),
				Caller: "cost",
				Reader: &fakePlanReader{err: tc.err},
			}
			if _, _, err := c.Read(time.Minute); err == nil {
				t.Fatal("want the failure")
			}
			b, err := os.ReadFile(c.Log)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.TrimSpace(string(b))
			if tc.marker != "" && !strings.HasSuffix(got, tc.marker) {
				t.Errorf("want the class marked at the tail %q, got %q", tc.marker, got)
			}
			if tc.marker == "" && strings.HasSuffix(got, "]") {
				t.Errorf("an unmarked class must add nothing, got %q", got)
			}
			// The streak reader's own answer, off the bytes just written —
			// the round trip is the contract, not either half.
			if n, class := planFailStreak(c.Log); n != 1 || class != PlanFailToken(tc.err) {
				t.Errorf("round trip = %d/%q, want 1/%q", n, class, PlanFailToken(tc.err))
			}
		})
	}
}

// ─── the watch pass preamble ─────────────────────────────────────────────────

// Once per pass, and the same bytes `posse status` and the cockpit print.
// The COUNT is the assertion: a line printed once at loop start would
// satisfy any Contains check and would be exactly the hourly log line that
// was already there and already ignored.
func TestQAWatchPreamblePrintsTheStaleLineEveryPass(t *testing.T) {
	b, _ := newTestBackend(t)
	d, _ := planDispatcher(t, b, nil)
	// Blind for the whole loop, which is the fixture: a reader that
	// SUCCEEDS refreshes the snapshot on pass 1 and the condition heals —
	// correctly, and then there is nothing left to print on pass 2.
	d.Plan = &fakePlanReader{err: &RateLimit{Status: "429 Too Many Requests"}}
	planConfig(t, b.App, planRepo(t, "[]", ""), guardOn)
	at := lpouiT.Add(-(10*time.Hour + 9*time.Minute))
	seedReading(t, b.App, at, PlanUsage{{Name: "5h", Pct: 41}, {Name: "7d", Pct: 89}})
	seedReadLog(t, b.App, "ok", "429 cooldown=5m", "429 cooldown=5m")
	d.Now = func() time.Time { return lpouiT }
	// Taken BEFORE the loop: the streak clause is live, so a pass whose own
	// read fails adds to it. That the number moves is the surface working;
	// what must be one-per-pass is the LINE, so the count below is on the
	// half of it that does not move.
	want := b.App.PlanStaleness("qa", lpouiT, os.Stderr).Line()

	const wantPasses = 3
	tap := newPassTap(wantPasses)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond); done <- p }()
	var passes int
	select {
	case passes = <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}

	got := tap.String()
	const stem = "plan meter BLIND 10h09m: last reading 2026-09-02T03:23Z (5h 41% · 7d 89%) — " +
		"ruling on it under the headroom rule; "
	if n := strings.Count(got, stem); n != passes {
		t.Fatalf("the stale line appeared %d time(s) across %d passes, want one per pass:\n%s", n, passes, got)
	}
	// And the FIRST one is byte-for-byte what `posse status` and the
	// cockpit render off the same state — the whole point of one renderer.
	if !strings.Contains(got, want) {
		t.Fatalf("the preamble must print PlanStale.Line() verbatim:\nwant %q\n%s", want, got)
	}
}

// Inside the threshold the preamble says nothing at all. The control for
// the pin above: a line that printed unconditionally would pass it.
func TestQAWatchPreambleQuietWhenTheReadingIsFresh(t *testing.T) {
	b, _ := newTestBackend(t)
	ps := newPlanServer(t, 12, 40)
	d, _ := planDispatcher(t, b, ps)
	planConfig(t, b.App, planRepo(t, "[]", ""), guardOn)
	seedReading(t, b.App, lpouiT.Add(-time.Minute), fakeWindows(41, 89))
	d.Now = func() time.Time { return lpouiT }

	tap := newPassTap(2)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond); done <- p }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}
	if strings.Contains(tap.String(), "plan meter BLIND") {
		t.Fatalf("a minute-old reading is not blindness:\n%s", tap.String())
	}
}
