//go:build posse_arm3

package posse

// QA pins for the escalating 429 backoff (ranger-base-rwwp6, off spike
// ranger-base-dvxac).
//
// The defect was a poller that honoured Retry-After exactly and asked again
// at the boundary, forever: fourteen consecutive 429s on 2026-09-02 between
// 03:30Z and 16:35Z, each naming an hour, three of them drawn by asks made
// AFTER the previous window had ended — and a plan guard blind for thirteen
// hours behind them. So the pins here are about a CADENCE, and the arm that
// makes them worth having is the pre-fix one: honouring the header verbatim
// puts an ask on every hour of the storm, and each test below states the
// number that behaviour would produce beside the number this one does.
//
// Hermetic: the fake endpoint, an injected clock, files under a t.TempDir.
// Nothing here waits for real time — the storm is 24 simulated hours.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stormRig is a poller: one PlanCache caller ticking at a fixed cadence
// against an endpoint that is answering 429, with the times it actually got
// a request out. The tick is the cockpit's two minutes, because the point of
// the cooldown is that the CADENCE of the file is not the cadence of the
// caller.
type stormRig struct {
	*cacheRig
	asks []time.Duration // when a request actually left, from t=0
}

func newStormRig(t *testing.T, retryAfter string) *stormRig {
	t.Helper()
	r := &stormRig{cacheRig: newCacheRig(t)}
	r.ps.status, r.ps.retry = http.StatusTooManyRequests, retryAfter
	return r
}

// tick runs the poller for d, asking every 2 minutes, and records every tick
// on which the fake endpoint's counter moved.
func (r *stormRig) tick(t *testing.T, d time.Duration) {
	t.Helper()
	for at := time.Duration(0); at <= d; at += 2 * time.Minute {
		r.at(at)
		before := r.hits()
		r.caller("dispatch").Read(5 * time.Minute)
		if r.hits() > before {
			r.asks = append(r.asks, at)
		}
	}
}

// seedCooldown puts a live cooldown on the shared snapshot, keeping
// whatever reading is already in it — the state the file is in mid-storm,
// which is what the loud line has to render.
func seedCooldown(t *testing.T, a *App, until time.Time, streak int) {
	t.Helper()
	path := filepath.Join(a.StateDir, "plan-usage.json")
	var e planEntry
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	e.RetryAt, e.Streak = until, streak
	out, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The storm, as a cadence. An endpoint that answers `429 Retry-After: 3600`
// to everything sees six requests in a day, spaced 1h, 2h, 4h, 8h, 8h — not
// the twenty-four the pre-fix code made, one on every hour boundary it was
// told to come back on.
func TestQAPlan429BackoffEscalatesAcrossAStorm(t *testing.T) {
	t.Parallel()
	r := newStormRig(t, "3600")
	r.tick(t, 24*time.Hour)

	want := []time.Duration{0, time.Hour, 3 * time.Hour, 7 * time.Hour, 15 * time.Hour, 23 * time.Hour}
	if len(r.asks) != len(want) {
		t.Fatalf("a day of 429s cost %d requests (%v), want %d (%v) — 24 is the pre-fix hourly cadence",
			len(r.asks), r.asks, len(want), want)
	}
	for i := range want {
		if r.asks[i] != want[i] {
			t.Errorf("ask %d at %s, want %s (asks: %v)", i+1, r.asks[i], want[i], r.asks)
		}
	}
}

// …and the property under that arithmetic, stated without the numbers: a
// storm asks at the boundary the endpoint named AT MOST ONCE. The first 429
// is still honoured exactly — the endpoint's own number is the best
// information anybody has about it, and an isolated rate limit must not cost
// more blindness than it asked for. Every ask after that waits longer than
// the window it was told to come back on, which is the loop the measured
// storm did not have: its three re-asks were 29s, 28s and 118s PAST a stated
// window and each drew a fresh hour.
func TestQAPlan429BackoffAsksTheBoundaryOnce(t *testing.T) {
	t.Parallel()
	r := newStormRig(t, "3600")
	r.tick(t, 24*time.Hour)

	if len(r.asks) < 3 {
		t.Fatalf("setup: want a storm, got asks %v", r.asks)
	}
	boundary := 0
	for i := 1; i < len(r.asks); i++ {
		if gap := r.asks[i] - r.asks[i-1]; gap <= time.Hour {
			boundary++
			if boundary > 1 {
				t.Errorf("ask %d came %s after ask %d — a second re-ask at the stated boundary is the loop that cannot terminate (asks: %v)",
					i+1, gap, i, r.asks)
			}
		}
	}
}

// A 200 is proof the endpoint answers, so it resets the schedule: the next
// storm starts at the endpoint's own number again, not at the ceiling the
// last one climbed to. Sticky escalation would make one bad afternoon cost
// the following morning.
func TestQAPlan429BackoffResetsOnSuccess(t *testing.T) {
	t.Parallel()
	r := newStormRig(t, "3600")
	r.tick(t, 4*time.Hour) // asks at 0, 1h, 3h — a streak of three
	if len(r.asks) != 3 {
		t.Fatalf("setup: want three asks, got %v", r.asks)
	}

	// The endpoint comes back at 7h, which is when the fourth ask is due.
	r.ps.status, r.ps.retry = http.StatusOK, ""
	r.at(7 * time.Hour)
	if _, _, err := r.caller("dispatch").Read(time.Minute); err != nil {
		t.Fatalf("the endpoint answered: %v", err)
	}

	// One 429 after that is the FIRST 429 again: an hour, not eight.
	r.ps.status, r.ps.retry = http.StatusTooManyRequests, "3600"
	r.at(8 * time.Hour)
	if _, _, err := r.caller("dispatch").Read(0); err == nil {
		t.Fatal("setup: want the 429")
	}
	before := r.hits()
	r.at(8*time.Hour + 59*time.Minute)
	r.caller("dispatch").Read(time.Minute)
	if r.hits() != before {
		t.Error("inside the first cooldown nobody asks")
	}
	r.at(9*time.Hour + time.Minute)
	r.caller("dispatch").Read(time.Minute)
	if r.hits() != before+1 {
		t.Errorf("a success must reset the schedule: the next 429 is honoured at %s, not at the ceiling the old streak reached", planCooldownMax)
	}
}

// The schedule itself, at its edges. Three inputs matter and each is a
// different clause of planCooldown: the header the endpoint sent, the cap on
// believing it, and the ceiling on escalating it.
func TestQAPlan429BackoffSchedule(t *testing.T) {
	t.Parallel()
	hours := func(h ...float64) []time.Duration {
		var out []time.Duration
		for _, x := range h {
			out = append(out, time.Duration(x*float64(time.Hour)))
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		retry time.Duration
		want  []time.Duration // the 1st, 2nd, 3rd… consecutive 429
	}{
		// The measured storm: an hour, doubling, stopped at the ceiling.
		{"the measured hour", time.Hour, hours(1, 2, 4, 8, 8, 8)},
		// No header is policy, not the endpoint — and policy escalates the
		// same way, from the number this file chose.
		{"no Retry-After", 0, []time.Duration{
			5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute,
			80 * time.Minute, 160 * time.Minute}},
		// A day-long Retry-After is still capped at an hour on the FIRST
		// 429 — the escalation does not lift planCooldownMax, it starts
		// from it.
		{"a day, capped first", 24 * time.Hour, hours(1, 2, 4, 8, 8, 8)},
		// A short one costs what it says: an isolated 429 asking for a
		// minute must not buy two.
		{"a short one", time.Minute, []time.Duration{
			time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute,
			16 * time.Minute, 32 * time.Minute}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prev := time.Duration(0) // the first 429 of a storm
			for i, want := range tc.want {
				got := planCooldown(tc.retry, prev)
				if got != want {
					t.Fatalf("429 #%d: honoured %s, want %s", i+1, got, want)
				}
				prev = got
			}
			// The ceiling holds however long the storm runs.
			if got := planCooldown(tc.retry, planCooldownCeiling); got != planCooldownCeiling {
				t.Errorf("at the ceiling the next 429 honoured %s, want %s", got, planCooldownCeiling)
			}
		})
	}
	// A wait a hand-edited file could hold. Nothing here may return a
	// negative or a shrinking duration.
	for _, prev := range []time.Duration{-9 * time.Hour, 0} {
		if got := planCooldown(time.Hour, prev); got != time.Hour {
			t.Errorf("prev %s honoured %s, want the endpoint's own hour", prev, got)
		}
	}
	// The escalation doubles the wait IN FORCE, so a storm never walks back
	// down its own schedule: two hours in, a 429 that names a minute — or
	// names nothing — still costs four.
	for _, header := range []time.Duration{0, time.Minute} {
		if got := planCooldown(header, 2*time.Hour); got != 4*time.Hour {
			t.Errorf("mid-storm header %s honoured %s, want 4h — a shrinking wait re-asks inside a window the endpoint already stated", header, got)
		}
	}
}

// The cadence log is the instrument the 429 incidents get reconstructed from
// (rangerhq-tdy8), and an escalating wait gives it three numbers to keep
// apart: what the box honoured, what the endpoint asked for, and which 429
// of the storm this was. Before this bead `cooldown=` was the capped value
// and nothing else, so a reader could not tell an hour the endpoint asked
// for from a day posse truncated.
func TestQAPlan429BackoffLogsTheRawRetryAfter(t *testing.T) {
	t.Parallel()
	r := newStormRig(t, "86400") // the endpoint asks for a day
	r.caller("dispatch").Read(time.Minute)
	r.at(90 * time.Minute)
	r.caller("cockpit").Read(time.Minute)

	lines := strings.Split(strings.TrimSpace(r.log(t)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one line per request that left, got %q", r.log(t))
	}
	for i, want := range []string{
		"dispatch 429 cooldown=1h00m retry-after=24h00m streak=1",
		"cockpit 429 cooldown=2h00m retry-after=24h00m streak=2",
	} {
		if !strings.HasSuffix(lines[i], want) {
			t.Errorf("line %d = %q, want it to end %q", i+1, lines[i], want)
		}
	}
	// A 429 with no header says so in words: "none" is not "0s".
	r.ps.retry = ""
	r.at(4 * time.Hour)
	r.caller("cost").Read(time.Minute)
	if got := r.log(t); !strings.Contains(got, "cost 429 cooldown=4h00m retry-after=none streak=3") {
		t.Errorf("a 429 that named no Retry-After must say none, got %q", got)
	}
}

// The new fields must not cost the streak reader its class. planLogClass
// takes the status off the FIRST field of the outcome and planFailStreak
// counts the lines — both are contracts with logRead's shape, and a log line
// that grew a tail is exactly the change that silently breaks them (the loud
// line would go back to saying "3 consecutive failed reads").
func TestQAPlan429BackoffLogStaysReadable(t *testing.T) {
	t.Parallel()
	r := newStormRig(t, "3600")
	r.tick(t, 4*time.Hour)

	fails, class := planFailStreak(filepath.Join(r.dir, "plan-usage.log"))
	if fails != 3 || class != "429" {
		t.Errorf("the streak reader sees %d/%q over the new line shape, want 3/%q", fails, class, "429")
	}
}

// The upgrade path, which is a live one: this ships to a box that may be
// mid-storm, and the snapshot it finds there names a cooldown and no streak.
// That cooldown is still honoured — a rate limiter is honoured off any entry
// that names one — and the 429 after it starts the schedule from the
// endpoint's own header, which is the pre-fix behaviour and the right first
// step of the new one.
func TestQAPlan429BackoffReadsAPreUpgradeSnapshot(t *testing.T) {
	t.Parallel()
	r := newStormRig(t, "3600")
	legacy := `{"at":"0001-01-01T00:00:00Z","windows":null,"retry_at":"` +
		blindT.Add(30*time.Minute).UTC().Format(time.RFC3339) + `"}`
	if err := os.WriteFile(filepath.Join(r.dir, "plan-usage.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	r.at(29 * time.Minute)
	if _, _, err := r.caller("dispatch").Read(time.Minute); err == nil {
		t.Fatal("a cooldown written by the old binary is still a cooldown")
	}
	if r.hits() != 0 {
		t.Fatalf("nobody asks inside it, got %d requests", r.hits())
	}
	r.at(31 * time.Minute)
	r.caller("dispatch").Read(time.Minute)
	if r.hits() != 1 {
		t.Fatalf("past it the next caller asks, got %d requests", r.hits())
	}
	// …and that 429 is the first of the storm as far as this binary knows:
	// an hour, not a doubling off a streak no file recorded.
	r.at(31*time.Minute + 59*time.Minute)
	r.caller("dispatch").Read(time.Minute)
	if r.hits() != 1 {
		t.Error("inside the first honoured hour nobody asks")
	}
	r.at(31*time.Minute + 61*time.Minute)
	r.caller("dispatch").Read(time.Minute)
	if r.hits() != 2 {
		t.Errorf("the first 429 after an upgrade is honoured verbatim, got %d requests", r.hits())
	}
}

// The escalation, said out loud. planCooldownMax's comment refuses "a guard
// that stops asking for a day" precisely because a long silent mute is
// indistinguishable from a guard that is off — so a wait this file chose for
// itself belongs on the line the operator reads (ranger-base-lpoui's), not
// only in a JSON field.
func TestQAPlan429BackoffIsOnTheLoudLine(t *testing.T) {
	t.Parallel()
	a := staleApp(t, "")
	at := lpouiT.Add(-10 * time.Hour)
	seedReading(t, a, at, fakeWindows(41, 89))
	seedReadLog(t, a, "ok", "429 cooldown=1h00m retry-after=1h00m streak=1",
		"429 cooldown=2h00m retry-after=1h00m streak=2",
		"429 cooldown=4h00m retry-after=1h00m streak=3")
	seedCooldown(t, a, lpouiT.Add(3*time.Hour+12*time.Minute), 3)

	line := a.PlanStaleness("qa", lpouiT, os.Stderr).Line()
	if !strings.HasSuffix(line, "3 consecutive 429, next ask in 3h12m") {
		t.Errorf("the wait is the fact the operator is owed:\n got %q\nwant it to end %q",
			line, "3 consecutive 429, next ask in 3h12m")
	}
	// An expired cooldown is not a wait: the line must not promise an ask
	// that is already due.
	seedCooldown(t, a, lpouiT.Add(-time.Minute), 3)
	if line := a.PlanStaleness("qa", lpouiT, os.Stderr).Line(); strings.Contains(line, "next ask") {
		t.Errorf("an expired cooldown holds nothing back, got %q", line)
	}
}
