package posse

// The grok pool guard (rangerhq-myso) — the SuperGrok weekly pool, metered
// from disk.
//
// The plan guard (planusage.go) watches one provider's rate windows through
// that provider's own usage endpoint. xAI publishes no such endpoint: pool
// utilisation is shown on a settings page, to a human, and every design that
// wanted a weekly grok cap was told — correctly — that it was designing
// against a meter that does not exist. ADR 0010 §3 wrote the consequence
// down: `plan_guard_overflow_cap:` counts BEADS because "the overflow pool
// has no meter the harness can read", and ADR 0010 §4 named a provider-side
// usage endpoint as the loop closer.
//
// It closes a different way. grok writes its own cost per turn to disk
// (cost_grok.go), so dollars spent on this pool since its weekly reset are
// already readable with no network call, no credential and no keychain. One
// empirical conversion — dollars per percentage point of pool, from a
// calibration bracket the operator holds — turns those dollars into the
// number the settings page shows. That is the whole meter:
//
//	utilisation% = (USD since the last weekly reset) / (USD per point)
//
// # Why this pool and not the 5h window
//
// The Claude 5h window heals in five hours. The SuperGrok week has no
// intra-week reset, so exhaustion is DAYS of nothing, and it takes the
// operator's own Grok — Chat, Voice, Imagine, same bucket — down with the
// fleet. A blown weekly pool is a much worse outcome than a blown 5h window,
// which is why this guard exists at all when a bead cap already bounds the
// same pool.
//
// # Three things this reading is NOT
//
//  1. It is not the vendor's number. It is an ESTIMATE derived from grok's
//     own list-price accounting through a factor somebody measured once, and
//     every line it prints says so. Nothing here may ever present it as
//     authoritative.
//  2. It is not complete. It sees grok sessions written on THIS box —
//     dispatch's and the operator's own CLI — and cannot see the same pool
//     spent from a phone or the web. So the estimate is a FLOOR, it
//     under-reports, and a threshold set against it must be sized knowing
//     that. Unread transcripts push it the same direction, never the other.
//  3. It is not blindable. The plan guard's whole blind-clock apparatus
//     (ADR 0018) exists because a credential or an endpoint can stop
//     answering; this reads local files, so there is no transient outage to
//     wait out and no clock to run. A guard armed with no reset or no
//     conversion factor is OFF and says so once a pass — the
//     `plan_guard_overflow` half-configured rule — and never parks a bead on
//     a condition no retry will change.
//
// # Where it applies
//
// Per BEAD, on the runtime the launch is actually going to, beside the
// account stage (ADR 0013 §3/§5): a meter gates only the work that can spend
// it. Skipping a whole pass because grok is drained would park claude lanes
// on somebody else's pool — the exact defect ADR 0010 §1 moved the plan
// guard's verdict per-bead to fix. A bead ADR 0010 MOVED to grok faces this
// guard too, for the reason the account cap is charged to the pool a bead
// lands on and not the one it came from.

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// GrokPoolRuntime is the runtime whose weekly pool this guard meters. It is
// a const and not a config key on purpose: the reset, the factor and the
// threshold are all the instance's, but WHICH pool they describe is fixed by
// the transcripts the reading is taken from.
const GrokPoolRuntime = "grok"

// WeeklyReset is a pool's weekly reset instant: a weekday and a local
// wall-clock time. Config `grok_pool_reset:` — `mon 09:00`.
//
// Local, not UTC, because it is the operator's own calendar the vendor's
// settings page is displayed in, and a reset an hour off matters only in the
// hour after it. Same reason DST is not special-cased: on a spring-forward
// Sunday `Last` may land on a wall-clock time that did not exist and Go
// normalises it forward, which moves the window boundary by an hour once a
// year on an estimate that is already a floor.
type WeeklyReset struct {
	Day  time.Weekday
	Hour int
	Min  int
}

// weekdayNames maps the accepted spellings. Both the three-letter form and
// the full name, case-insensitive: this is a value an operator types into a
// config file by hand, and refusing `Monday` because the parser wanted `mon`
// is a typo line for no reason.
var weekdayNames = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "weds": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// ParseWeeklyReset reads `<weekday> <HH:MM>`. Anything else is an error the
// caller names on stderr — a malformed reset must be visible, because the
// alternative is a guard measuring a week that starts on the wrong day.
func ParseWeeklyReset(raw string) (WeeklyReset, error) {
	f := strings.Fields(strings.TrimSpace(raw))
	if len(f) != 2 {
		return WeeklyReset{}, fmt.Errorf("want `<weekday> <HH:MM>` (two fields, got %d)", len(f))
	}
	day, ok := weekdayNames[strings.ToLower(f[0])]
	if !ok {
		return WeeklyReset{}, fmt.Errorf("%q is not a weekday", f[0])
	}
	hm := strings.SplitN(f[1], ":", 2)
	if len(hm) != 2 {
		return WeeklyReset{}, fmt.Errorf("%q is not HH:MM", f[1])
	}
	h, herr := strconv.Atoi(hm[0])
	m, merr := strconv.Atoi(hm[1])
	if herr != nil || merr != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return WeeklyReset{}, fmt.Errorf("%q is not a time of day 00:00–23:59", f[1])
	}
	return WeeklyReset{Day: day, Hour: h, Min: m}, nil
}

// Last is the most recent reset instant at or before now, in now's location.
//
// At the reset minute exactly the window has just rolled: the boundary is
// inclusive, so a pass run at 09:00:00 on reset day measures a fresh week and
// not the one that just ended.
func (r WeeklyReset) Last(now time.Time) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), r.Hour, r.Min, 0, 0, now.Location())
	// Eight, not seven: today may be the right weekday with the reset time
	// still ahead of us, which spends one step before the weekday walk starts.
	for i := 0; i < 8; i++ {
		if t.Weekday() == r.Day && !t.After(now) {
			return t
		}
		t = t.AddDate(0, 0, -1)
	}
	return t // unreachable: seven days back always meets every weekday
}

// GrokPoolReset reads config `grok_pool_reset:`. Unset is (zero, false) and
// silent — an unarmed guard reads nothing and says nothing. Malformed is
// named on errw and also false: the guard then runs OFF rather than against a
// week starting on a day nobody chose.
func (a *App) GrokPoolReset(errw io.Writer) (WeeklyReset, bool) {
	raw := strings.TrimSpace(a.CfgGet("grok_pool_reset", ""))
	if raw == "" {
		return WeeklyReset{}, false
	}
	r, err := ParseWeeklyReset(raw)
	if err != nil {
		fmt.Fprintf(errw, "grok pool: config grok_pool_reset: %q — %v — pool guard off\n", raw, err)
		return WeeklyReset{}, false
	}
	return r, true
}

// GrokPoolUSDPerPoint reads config `grok_pool_usd_per_point:` — the dollars
// one percentage point of the weekly pool costs, from the operator's own
// calibration bracket.
//
// It is CONFIG and never a constant in this repo, and that is the caveat the
// bead spent a paragraph on: the factor is empirical, derived from grok's own
// list-price accounting, and it drifts the day xAI reprices. A number baked
// in here would go wrong silently on a machine nobody recalibrated. Posse
// therefore ships none, logs the one it used every time it uses it, and holds
// no calibration figure in its source or its docs.
func (a *App) GrokPoolUSDPerPoint(errw io.Writer) (float64, bool) {
	raw := strings.TrimSpace(a.CfgGet("grok_pool_usd_per_point", ""))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimPrefix(raw, "$"), 64)
	if err != nil || v <= 0 {
		fmt.Fprintf(errw, "grok pool: config grok_pool_usd_per_point: %q is not a positive dollar amount — pool guard off\n", raw)
		return 0, false
	}
	return v, true
}

// GrokGuardWeek reads config `grok_guard_week:` (percent of the weekly pool).
// Unset — the default — is the guard off and today's behaviour exactly: no
// transcript is scanned and nothing is said. A value that is not a percent is
// named on errw and dropped, the `plan_guard_<window>:` rule, for its reason:
// a typo must be visible, not a silently disabled guard.
func (a *App) GrokGuardWeek(errw io.Writer) float64 {
	raw := strings.TrimSpace(a.CfgGet("grok_guard_week", ""))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64)
	if err != nil || v <= 0 || v > 100 {
		fmt.Fprintf(errw, "grok pool: config grok_guard_week: %q is not a percent 1–100 — pool guard off\n", raw)
		return 0
	}
	return v
}

// PoolReading is one estimate of a weekly pool's utilisation.
//
// Unread is how many transcripts could not be read, and ReadErr the first
// reason. They are carried rather than folded into the number for ADR 0018
// §3's rule — "nothing was spent" and "nothing could be counted" are
// different facts — but unlike the ledgers, an unread transcript here does
// NOT fail the guard closed. Two reasons, both specific to this meter:
// UsedUSD is already a floor by construction (see the file header), so a lost
// file moves it in the direction the threshold is already sized for; and a
// grok session directory that cannot be read is not a transient outage that a
// later pass recovers from, so failing closed on one would be a brake with no
// release. Every line the guard prints names the count instead.
type PoolReading struct {
	UsedUSD     float64
	Pct         float64
	Since       time.Time
	UsdPerPoint float64
	Unread      int
	ReadErr     error
}

// Line is the reading, said out loud, and the word ESTIMATED is not
// decoration: this number is not the vendor's and must never be printed as if
// it were. The factor is on the line because it is the one input that goes
// stale without anything failing (see GrokPoolUSDPerPoint).
func (r PoolReading) Line() string {
	s := fmt.Sprintf("estimated %.0f%% of the weekly pool used — $%.2f since %s at $%.4f per point",
		r.Pct, r.UsedUSD, r.Since.Format("Mon 2006-01-02 15:04"), r.UsdPerPoint)
	if r.Unread > 0 {
		s += fmt.Sprintf(", %d transcript(s) unreadable (%v)", r.Unread, r.ReadErr)
	}
	return s + " — a FLOOR: grok spent off this box is not visible here"
}

// poolSpendSince sums one provider's reported dollars since a time, across
// every session on this machine — dispatch's and the operator's own
// interactive ones alike, because they come out of the same pool and the pool
// is what is being metered. No project filter, for the same reason.
//
// It returns what it could read plus what it could not: the count of
// unreadable transcripts and the first reason. A partial sum is still the
// best floor available (scanProvider's rule), and the caller says how bad.
func poolSpendSince(p CostProvider, since time.Time) (usd float64, unread int, first error) {
	files, errs := p.Transcripts("")
	for _, err := range errs {
		unread++
		if first == nil {
			first = err
		}
	}
	for _, f := range files {
		// The mtime skip scanProvider already relies on: a file untouched
		// since the reset holds no record after it, and this runs on every
		// pass that has a grok bead in it.
		if !since.IsZero() {
			if st, err := os.Stat(f); err == nil && st.ModTime().Before(since) {
				continue
			}
		}
		segs, err := p.Decode(f, since)
		if err != nil {
			unread++
			if first == nil {
				first = err
			}
			continue
		}
		for _, s := range segs {
			_, c := s.Total()
			usd += c
		}
	}
	return usd, unread, first
}

// GrokPoolUsage takes the reading: dollars on the grok pool since its most
// recent weekly reset, converted to percent. The error is the one structural
// absence — no cost adapter for grok is compiled in, so there is no meter at
// all — which is guard-OFF and not a failed reading (planusage.go's
// NoPlanAdapter distinction, applied to this seam).
func (a *App) GrokPoolUsage(reset WeeklyReset, usdPerPoint float64, now time.Time) (PoolReading, error) {
	p, ok := CostProviderFor(GrokPoolRuntime)
	if !ok {
		return PoolReading{}, fmt.Errorf("no cost adapter reads %s, so its pool cannot be metered from disk", GrokPoolRuntime)
	}
	since := reset.Last(now)
	usd, unread, err := poolSpendSince(p, since)
	return PoolReading{
		UsedUSD:     usd,
		Pct:         usd / usdPerPoint,
		Since:       since,
		UsdPerPoint: usdPerPoint,
		Unread:      unread,
		ReadErr:     err,
	}, nil
}

// grokPoolState is one pass's pool guard: the threshold, the reading, or the
// reason an armed guard could not take one. Nil until the pass first resolves
// a bead onto the metered runtime — a fleet with no grok work scans no
// transcripts and prints no line, uncountedReport's rule.
type grokPoolState struct {
	Threshold float64
	Reading   PoolReading
	Read      bool
	// Off is why an armed guard is not running — a missing reset, a missing
	// factor, no adapter. Guard OFF, said once a pass, nothing parked: the
	// half-configured `plan_guard_overflow` rule, and for its reason. A
	// brake the operator believes in and does not have must be loud.
	Off string
}

// grokPoolGuard is the pass's memoized reading, taken lazily on the first
// bead that resolves onto the metered runtime.
func (d *Dispatcher) grokPoolGuard() *grokPoolState {
	if d.grokPool != nil {
		return d.grokPool
	}
	st := &grokPoolState{}
	d.grokPool = st
	st.Threshold = d.App.GrokGuardWeek(d.errw())
	if st.Threshold == 0 {
		return st // unarmed: today's behaviour, silently
	}
	reset, okR := d.App.GrokPoolReset(d.errw())
	factor, okF := d.App.GrokPoolUSDPerPoint(d.errw())
	switch {
	case !okR && !okF:
		st.Off = "grok_pool_reset: and grok_pool_usd_per_point: are both unset or unusable"
	case !okR:
		st.Off = "grok_pool_reset: is unset or unusable"
	case !okF:
		st.Off = "grok_pool_usd_per_point: is unset or unusable"
	}
	if st.Off != "" {
		d.eprintf("grok pool: grok_guard_week: %.0f%% is set but %s — no reading is possible, so the pool guard is OFF, not blind: nothing will park on this\n",
			st.Threshold, st.Off)
		return st
	}
	r, err := d.App.GrokPoolUsage(reset, factor, d.now())
	if err != nil {
		st.Off = err.Error()
		d.eprintf("grok pool: grok_guard_week: %.0f%% is set but %v — the pool guard is OFF, not blind: nothing will park on this\n",
			st.Threshold, err)
		return st
	}
	st.Reading, st.Read = r, true
	// The reading, once per pass, on the report and not on stderr: it is an
	// outcome the pass reached, not a warning about one, and it is where the
	// conversion factor in use gets logged.
	d.printf("! grok pool: %s (threshold grok_guard_week: %.0f%%)\n", r.Line(), st.Threshold)
	return st
}

// grokPoolSkip is the brake: the line this bead gets instead of a launch, or
// "" to launch. Called with the runtime the launch is actually going to, so a
// bead ADR 0010 moved onto grok is judged by grok's pool.
//
// Strictly above the threshold, matching planGuard: at the threshold exactly
// the bead still runs.
func (d *Dispatcher) grokPoolSkip(runtime string) string {
	if runtime != GrokPoolRuntime {
		return ""
	}
	st := d.grokPoolGuard()
	if !st.Read || st.Reading.Pct <= st.Threshold {
		return ""
	}
	return fmt.Sprintf("grok pool: estimated %.0f%% of the weekly pool used > grok_guard_week: %.0f%% — skipped",
		st.Reading.Pct, st.Threshold)
}
