package posse

// The shared plan reading (rangerhq-tdy8): one snapshot with an age on it,
// a Retry-After every process honours, and a log of what actually left the
// machine. Hermetic — a fake endpoint, a fake keychain, an injected clock.

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cacheRig is a state dir, a fake endpoint and a clock the test drives.
type cacheRig struct {
	dir   string
	ps    *planServer
	clock time.Time
}

func newCacheRig(t *testing.T) *cacheRig {
	t.Helper()
	return &cacheRig{dir: t.TempDir(), ps: newPlanServer(t, 42, 61), clock: blindT}
}

// caller is one posse process's view of the shared reading.
func (r *cacheRig) caller(name string) *PlanCache {
	return &PlanCache{
		Path:   filepath.Join(r.dir, "plan-usage.json"),
		Log:    filepath.Join(r.dir, "plan-usage.log"),
		Caller: name,
		Reader: r.ps.reader(),
		Now:    func() time.Time { return r.clock },
	}
}

func (r *cacheRig) at(d time.Duration) { r.clock = blindT.Add(d) }
func (r *cacheRig) hits() int64        { return r.ps.hits.Load() }

func (r *cacheRig) log(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.dir, "plan-usage.log"))
	if err != nil {
		return ""
	}
	return string(b)
}

// The whole point: three pollers, one request. The cockpit's tick, the
// dispatch pass and `posse cost` all see the same reading, and only the
// first of them costs the endpoint anything.
func TestPlanCacheOneReadingManyCallers(t *testing.T) {
	r := newCacheRig(t)
	u, at, err := r.caller("cockpit").Read(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if win(u, "5h") != 42 || win(u, "7d") != 61 {
		t.Fatalf("wrong reading: %+v", u)
	}
	if !at.Equal(blindT) {
		t.Errorf("reading taken at %s, want %s", at, blindT)
	}

	r.at(90 * time.Second)
	for _, who := range []string{"dispatch", "cost", "cockpit"} {
		u2, at2, err := r.caller(who).Read(5 * time.Minute)
		if err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		if u2.Line() != u.Line() {
			t.Errorf("%s got a different reading: %+v want %+v", who, u2, u)
		}
		// The age travels with it — a hit is a snapshot, not a fresh read.
		if !at2.Equal(blindT) {
			t.Errorf("%s: a cache hit must carry the reading's own time, got %s", who, at2)
		}
	}
	if got := r.hits(); got != 1 {
		t.Errorf("four callers inside the TTL must cost one request, got %d", got)
	}
}

// Past the age the caller asked for, somebody fetches — the cache never
// serves a reading older than what was requested.
func TestPlanCacheRefetchesPastTheAgeAsked(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	if _, _, err := r.caller("dispatch").Read(5 * time.Minute); err != nil {
		t.Fatal(err)
	}
	r.at(5 * time.Minute) // exactly the TTL is already too old
	_, at, err := r.caller("dispatch").Read(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.hits(); got != 2 {
		t.Errorf("want a second request past the TTL, got %d", got)
	}
	if !at.Equal(r.clock) {
		t.Errorf("a refetched reading is taken now: %s, want %s", at, r.clock)
	}
}

// maxAge 0 is the escape hatch: no sharing, every caller asks.
func TestPlanCacheZeroAgeAlwaysAsks(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	for i := 0; i < 3; i++ {
		if _, _, err := r.caller("cost").Read(0); err != nil {
			t.Fatal(err)
		}
	}
	if got := r.hits(); got != 3 {
		t.Errorf("maxAge 0 must always ask, got %d requests", got)
	}
}

// Retry-After crosses process boundaries, because the file does. This is
// the whole (c) of the bead: a 429 must not be answered by asking again in
// two minutes from a different pid.
func TestPlanCacheHonoursRetryAfterAcrossCallers(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	r.ps.status, r.ps.retry = http.StatusTooManyRequests, "120"

	if _, _, err := r.caller("cockpit").Read(5 * time.Minute); err == nil {
		t.Fatal("a 429 is an error to the caller")
	}
	if got := r.hits(); got != 1 {
		t.Fatalf("setup: want 1 request, got %d", got)
	}

	r.at(90 * time.Second)
	_, _, err := r.caller("dispatch").Read(5 * time.Minute)
	if err == nil {
		t.Fatal("still cooling down: the caller is blind, not served")
	}
	if want := "rate-limited"; !strings.Contains(err.Error(), want) {
		t.Errorf("the error must name the cooldown, got %q", err)
	}
	if got := r.hits(); got != 1 {
		t.Errorf("another process must not re-ask inside the cooldown, got %d requests", got)
	}

	// …and it does ask again once the endpoint said it could.
	r.ps.status, r.ps.retry = http.StatusOK, ""
	r.at(2*time.Minute + time.Second)
	if _, _, err := r.caller("dispatch").Read(5 * time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := r.hits(); got != 2 {
		t.Errorf("past Retry-After the next caller asks, got %d requests", got)
	}
}

// A 429 that names no Retry-After still buys a cooldown — the endpoint did
// not say, so the policy does.
func TestPlanCache429WithoutRetryAfter(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	r.ps.status = http.StatusTooManyRequests
	if _, _, err := r.caller("dispatch").Read(time.Minute); err == nil {
		t.Fatal("want an error")
	}
	r.at(planCooldownDefault - time.Second)
	if _, _, err := r.caller("dispatch").Read(time.Minute); err == nil {
		t.Fatal("want an error inside the default cooldown")
	}
	if got := r.hits(); got != 1 {
		t.Errorf("inside the default cooldown nobody asks, got %d", got)
	}
	r.at(planCooldownDefault + time.Second)
	r.caller("dispatch").Read(time.Minute)
	if got := r.hits(); got != 2 {
		t.Errorf("past the default cooldown the next caller asks, got %d", got)
	}
}

// A Retry-After longer than an hour is capped: past that the blind window
// (rangerhq-6h1) is what decides, not a silent hour-long mute.
func TestPlanCacheCapsALongRetryAfter(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	r.ps.status, r.ps.retry = http.StatusTooManyRequests, "86400"
	r.caller("dispatch").Read(time.Minute)

	r.at(59 * time.Minute)
	r.caller("dispatch").Read(time.Minute)
	if got := r.hits(); got != 1 {
		t.Errorf("inside the cap nobody asks, got %d", got)
	}
	r.at(61 * time.Minute)
	r.caller("dispatch").Read(time.Minute)
	if got := r.hits(); got != 2 {
		t.Errorf("the cooldown is capped at %s, got %d requests at 61m", planCooldownMax, got)
	}
}

// A success is proof the endpoint answers, so it clears the cooldown — no
// operator action, no sticky state (the plan guard's own rule).
func TestPlanCacheSuccessClearsTheCooldown(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	r.ps.status, r.ps.retry = http.StatusTooManyRequests, "60"
	r.caller("dispatch").Read(time.Minute)

	r.ps.status, r.ps.retry = http.StatusOK, ""
	r.at(2 * time.Minute)
	if _, _, err := r.caller("dispatch").Read(time.Minute); err != nil {
		t.Fatal(err)
	}
	var e planEntry
	b, err := os.ReadFile(filepath.Join(r.dir, "plan-usage.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	if !e.RetryAt.IsZero() {
		t.Errorf("a good reading must clear the cooldown, got retry_at %s", e.RetryAt)
	}
}

// A 429 must not throw away the reading anyone can still act on: the
// cooldown moves, the snapshot stays.
func TestPlanCacheKeepsTheReadingThroughA429(t *testing.T) {
	r := newCacheRig(t)
	if _, _, err := r.caller("cockpit").Read(time.Minute); err != nil {
		t.Fatal(err)
	}
	r.ps.status = http.StatusTooManyRequests
	r.at(2 * time.Minute)
	if _, _, err := r.caller("cockpit").Read(time.Minute); err == nil {
		t.Fatal("setup: want the 429")
	}
	// A caller that can live with a 3-minute-old reading still gets one.
	u, at, err := r.caller("cost").Read(10 * time.Minute)
	if err != nil {
		t.Fatalf("the last good reading survived the 429: %v", err)
	}
	if win(u, "5h") != 42 || !at.Equal(blindT) {
		t.Errorf("want the original reading at %s, got %+v at %s", blindT, u, at)
	}
}

// A truncated or hand-edited snapshot is a cache miss, never a crash and
// never a wrong number.
func TestPlanCacheCorruptSnapshotRefetches(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	path := filepath.Join(r.dir, "plan-usage.json")
	if err := os.WriteFile(path, []byte(`{"at":"nope","five_`), 0o644); err != nil {
		t.Fatal(err)
	}
	u, _, err := r.caller("dispatch").Read(5 * time.Minute)
	if err != nil {
		t.Fatalf("a corrupt cache must read through to the endpoint: %v", err)
	}
	if win(u, "5h") != 42 {
		t.Errorf("wrong reading: %+v", u)
	}
	var e planEntry
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &e); err != nil {
		t.Errorf("the fetch must leave a valid snapshot behind: %v", err)
	}
}

// No state dir to share through: still a reading, never a failure. The
// cache is an optimization and fails quiet, like the rest of the guard.
func TestPlanCacheWithoutAStateDirStillReads(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	c := r.caller("cost")
	c.Path, c.Log = "", ""
	if _, _, err := c.Read(5 * time.Minute); err != nil {
		t.Fatalf("no cache is not an error: %v", err)
	}
	if got := r.hits(); got != 1 {
		t.Errorf("want the request to go out anyway, got %d", got)
	}
}

// The read log is the evidence for "is the 429 ours?": one line per request
// that actually left the machine, naming who asked. Cache hits are not
// requests and write nothing — a log that counted hits would answer a
// different question than the endpoint is asking.
func TestPlanCacheLogsRequestsNotHits(t *testing.T) {
	r := newCacheRig(t)
	r.caller("cockpit").Read(5 * time.Minute)  // request
	r.at(time.Minute)                          //
	r.caller("dispatch").Read(5 * time.Minute) // hit
	r.at(6 * time.Minute)
	r.ps.status, r.ps.retry = http.StatusTooManyRequests, "300"
	r.caller("cost").Read(5 * time.Minute) // request, rate-limited

	lines := strings.Split(strings.TrimSpace(r.log(t)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one line per request, got %d:\n%s", len(lines), r.log(t))
	}
	if !strings.HasPrefix(lines[0], blindT.UTC().Format(time.RFC3339)+" cockpit ok") {
		t.Errorf("first line: %q", lines[0])
	}
	if !strings.Contains(lines[1], "cost 429 cooldown=5m") {
		t.Errorf("a rate-limited request says so and for how long: %q", lines[1])
	}
	if strings.Contains(r.log(t), fakeToken) {
		t.Error("the access token reached the read log")
	}
}

// A failed read is logged too, in the generic terms planusage.go returns —
// the log is a cadence record, not a credential leak.
func TestPlanCacheLogsAFailedRead(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	c := r.caller("dispatch")
	c.Reader.(*AnthropicPlanReader).URL = deadURL(t)
	if _, _, err := c.Read(time.Minute); err == nil {
		t.Fatal("want the transport error")
	}
	if want := "dispatch failed: usage endpoint unreachable"; !strings.Contains(r.log(t), want) {
		t.Errorf("want %q, got %q", want, r.log(t))
	}
}

// The log is bounded: a misconfigured TTL cannot grow it without limit.
func TestPlanCacheTrimsTheLog(t *testing.T) {
	r := newCacheRig(t)
	path := filepath.Join(r.dir, "plan-usage.log")
	var b strings.Builder
	for b.Len() <= planLogMax {
		b.WriteString(blindT.UTC().Format(time.RFC3339) + " ancient ok\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.caller("cockpit").Read(time.Minute); err != nil {
		t.Fatal(err)
	}
	after := r.log(t)
	lines := strings.Split(strings.TrimSpace(after), "\n")
	if len(lines) != planLogKeep {
		t.Errorf("want the newest %d lines kept, got %d", planLogKeep, len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "cockpit ok") {
		t.Errorf("the trim keeps the NEWEST lines: last is %q", lines[len(lines)-1])
	}
	if int64(len(after)) > planLogMax {
		t.Errorf("log still %d bytes after the trim", len(after))
	}
}

// plan_usage_ttl: the house's duration grammar, an escape hatch at 0, and a
// typo that is visible rather than a setting that quietly changed.
func TestPlanUsageTTLConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want time.Duration
		warn bool
	}{
		{"", PlanUsageTTLDefault, false},
		{"30s", 30 * time.Second, false},
		{"600", 10 * time.Minute, false},
		{"2m30s", 150 * time.Second, false},
		{"0", 0, false},
		{"soon", PlanUsageTTLDefault, true},
		{"-5m", PlanUsageTTLDefault, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			a := &App{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}
			cfg := ""
			if tc.raw != "" {
				cfg = "plan_usage_ttl: " + tc.raw + "\n"
			}
			if err := os.WriteFile(a.ConfigPath, []byte(cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			var errb strings.Builder
			if got := a.PlanUsageTTL(&errb); got != tc.want {
				t.Errorf("plan_usage_ttl %q: got %s, want %s", tc.raw, got, tc.want)
			}
			if warned := strings.Contains(errb.String(), "plan_usage_ttl"); warned != tc.warn {
				t.Errorf("plan_usage_ttl %q: warned=%v, want %v (%q)", tc.raw, warned, tc.warn, errb.String())
			}
		})
	}
}

// A cache hit may never spend more than half the guard's blind budget: the
// rest is reserved for a real outage, so a shared reading can never be the
// thing that fails a pass closed.
func TestPlanGuardMaxAge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		ttl, blindMax, want time.Duration
	}{
		{5 * time.Minute, 10 * time.Minute, 5 * time.Minute},   // the defaults: exactly half
		{5 * time.Minute, 4 * time.Minute, 2 * time.Minute},    // a tight budget wins
		{time.Minute, time.Hour, time.Minute},                  // a generous budget does not stretch the TTL
		{5 * time.Minute, 0, 5 * time.Minute},                  // blind_max 0 = never fail closed: nothing to reserve
		{0, 10 * time.Minute, 0},                               // sharing off stays off
		{30 * time.Second, 30 * time.Second, 15 * time.Second}, //
	} {
		if got := planGuardMaxAge(tc.ttl, tc.blindMax); got != tc.want {
			t.Errorf("planGuardMaxAge(%s, %s) = %s, want %s", tc.ttl, tc.blindMax, got, tc.want)
		}
	}
}

// The endpoint's 429 is its own error type, because it is the one failure a
// caller must not answer by asking again.
func TestPlanReaderRateLimited(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		retry  string
		want   time.Duration
	}{
		{"delta seconds", http.StatusTooManyRequests, "90", 90 * time.Second},
		{"no header", http.StatusTooManyRequests, "", 0},
		{"http date", http.StatusTooManyRequests, blindT.Add(2 * time.Minute).UTC().Format(http.TimeFormat), 2 * time.Minute},
		{"date in the past", http.StatusTooManyRequests, blindT.Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
		{"garbage", http.StatusTooManyRequests, "soonish", 0},
		{"503 with a wait", http.StatusServiceUnavailable, "30", 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ps := newPlanServer(t, 1, 2)
			ps.status, ps.retry = tc.status, tc.retry
			r := ps.reader()
			r.Now = func() time.Time { return blindT }

			_, err := r.Read()
			var rl *RateLimit
			if !errors.As(err, &rl) {
				t.Fatalf("want a *RateLimit, got %#v", err)
			}
			if rl.RetryAfter != tc.want {
				t.Errorf("Retry-After %q: got %s, want %s", tc.retry, rl.RetryAfter, tc.want)
			}
			if strings.Contains(err.Error(), fakeToken) {
				t.Error("the access token reached the error text")
			}
		})
	}
}

// The one rendering both `posse cost` and `posse cost --plan` print
// (rangerhq-p3z). A fresh reading says only the numbers; a snapshot the
// caller accepted as stale says how stale — the number must never be
// presented as newer than it is.
func TestPlanCacheLineSaysHowOldTheReadingIs(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)

	line, err := r.caller("cost").Line(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if want := "plan windows: 5h 42% · 7d 61%"; line != want {
		t.Errorf("a fresh reading is just the numbers: got %q, want %q", line, want)
	}

	r.at(3 * time.Minute)
	line, err = r.caller("cost").Line(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if want := "plan windows: 5h 42% · 7d 61%, read 3m ago"; line != want {
		t.Errorf("a cache hit must carry its age: got %q, want %q", line, want)
	}
	if got := r.hits(); got != 1 {
		t.Errorf("both lines came off one request, got %d", got)
	}
}

// A read that fails renders nothing at all — the caller decides whether
// silence or an exit code is the right answer, and there is no half line
// with a missing number in it.
func TestPlanCacheLineRendersNothingOnAFailedRead(t *testing.T) {
	t.Parallel()
	r := newCacheRig(t)
	c := r.caller("cost")
	c.Reader = &AnthropicPlanReader{URL: deadURL(t), Token: func() (string, error) { return fakeToken, nil }}

	line, err := c.Line(5 * time.Minute)
	if err == nil {
		t.Fatalf("an unreadable endpoint is an error, got line %q", line)
	}
	if line != "" {
		t.Errorf("a failed read renders no line, got %q", line)
	}
}
