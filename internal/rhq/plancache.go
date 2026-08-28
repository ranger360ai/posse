package rhq

// One plan reading, shared (rangerhq-tdy8).
//
// GET /api/oauth/usage is a metering endpoint, and posse had three
// independent pollers on it: the cockpit every 2m for as long as it is open
// (~30 requests/hour), one per dispatch pass, one per `posse cost`. On
// 2026-08-22 the endpoint answered 429 continuously from ~17:21 to ~20:35
// ($StateDir/dispatch-watch.log, three loop generations) and every poller
// went on asking at its own cadence. The plan guard was blind the whole
// time, and an unattended --watch fails closed past
// `plan_guard_blind_max:` — so a 429 storm stops the fleet.
//
// The fix is one derived copy with an age on it. The endpoint is the store
// of record for the reading; `$StateDir/plan-usage.json` is a snapshot of
// it, and every caller states how stale a snapshot it can act on (Read's
// maxAge). Past that age one caller fetches and replaces the file, and
// every other caller's read costs nothing — so the whole instance makes at
// most one request per TTL no matter how many cockpits, passes and cost
// runs are asking.
//
// Two rules the incident bought:
//
//   - Retry-After is honoured across processes. A 429 writes its cooldown
//     into the same file, so the next asker — usually a different process —
//     does not re-ask until it expires. Re-asking a rate limiter every two
//     minutes is how a storm gets extended.
//   - Every request that actually leaves the machine is logged to
//     `$StateDir/plan-usage.log` with who asked and what came back. The
//     cadence is the evidence: next time the endpoint 429s for hours, that
//     file settles whether it was us.
//
// No lock. Writers here never mutate shared state, they replace a snapshot
// with a newer one, and last-writer-wins is the right answer for a
// snapshot: a success landing after a 429 clears the cooldown, which is
// exactly what a success means. Taking a flock (ADR 0011 §1) across a
// ten-second HTTP call would trade a rate problem for a liveness one.
//
// Fail-quiet, like the rest of the plan guard: an unwritable state dir
// costs the sharing, never the reading — and never a line on a stderr the
// cockpit does not own.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// PlanUsageTTLDefault is how stale a shared reading may be before
	// somebody fetches a new one. Five minutes against a five-hour rolling
	// window is a reading that has barely moved, and it caps the whole
	// instance at ~12 requests/hour where the cockpit alone made ~30.
	PlanUsageTTLDefault = 5 * time.Minute
	// planCooldownDefault is how long a 429 that named no Retry-After is
	// honoured for. The endpoint did not say; this is the policy.
	planCooldownDefault = 5 * time.Minute
	// planCooldownMax caps a Retry-After we believe. A guard that stops
	// asking for a day is a guard that is off, and the blind window
	// (rangerhq-6h1) is the thing that must decide what to do about a long
	// outage — not this file, silently.
	planCooldownMax = time.Hour
)

// planEntry is the file: one snapshot, plus whatever cooldown the endpoint
// last asked for. The cooldown rides along with the reading rather than in
// its own file — one fact, one store.
type planEntry struct {
	At time.Time `json:"at"`
	// Windows is the reading, named by whichever adapter took it (ADR 0012
	// D4). It replaced two provider-shaped fields, so a snapshot written by
	// a posse from before that split decodes to zero windows — which load
	// below reads as a miss rather than as a plan with no limits.
	Windows PlanUsage `json:"windows"`
	RetryAt time.Time `json:"retry_at,omitempty"`
}

// PlanCache is the shared reading as one caller sees it. Caller is the name
// that lands in the read log ("dispatch", "cockpit", "cost") — the log is
// only evidence if it says who.
type PlanCache struct {
	Path   string // the snapshot; "" = no sharing, every read is a request
	Log    string // the read-cadence log; "" = no log
	Caller string
	Reader PlanReader       // the instance's adapter; nil = there is none
	Now    func() time.Time // nil = time.Now
	// NoAdapter is why Reader is nil (planusage.go). Carried rather than
	// returned by the constructor, for PlanReader.URLErr's reason: every
	// caller wants a cache, and the honest place to say "no reading, and
	// why" is the one that would have made the request.
	NoAdapter error
}

// PlanCache builds the instance's cache for one caller. Every posse process
// that reads the usage endpoint goes through this.
func (a *App) PlanCache(caller string) *PlanCache {
	r, err := PlanAdapter()
	return &PlanCache{
		Path:      filepath.Join(a.StateDir, "plan-usage.json"),
		Log:       filepath.Join(a.StateDir, "plan-usage.log"),
		Caller:    caller,
		Reader:    r,
		NoAdapter: err,
	}
}

func (c *PlanCache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Read returns a reading no older than maxAge, and the time that reading
// was taken — which is the caller's, not now: a cache hit must not reset
// anybody's blind clock to the present (dispatch.go planGuard).
//
// maxAge 0 means "fresh only" and always makes a request. It does not mean
// "ignore Retry-After": honouring a rate limiter is not caching, and no
// setting turns it off.
func (c *PlanCache) Read(maxAge time.Duration) (PlanUsage, time.Time, error) {
	// No adapter, no reading — and not a stale one either. A snapshot on
	// this machine was written by a posse that could reach a meter; an
	// instance that cannot refresh it has no business acting on it, and
	// saying so is the whole point of NoPlanAdapter being a type.
	r := c.Reader
	if r == nil {
		if c.NoAdapter != nil {
			return nil, time.Time{}, c.NoAdapter
		}
		return nil, time.Time{}, &NoPlanAdapter{Why: "no plan-window adapter"}
	}
	now := c.now()
	e, have := c.load()
	if have && maxAge > 0 && !e.At.IsZero() && now.Sub(e.At) < maxAge && now.Sub(e.At) >= 0 {
		return e.Windows, e.At, nil
	}
	if have && now.Before(e.RetryAt) {
		return nil, time.Time{}, Die("usage endpoint rate-limited, not asking again for %s", BlindFor(e.RetryAt.Sub(now)))
	}
	u, err := r.Read()
	// The read log is written either way. A request that left the machine is
	// evidence whoever answered it, and an override that is refused a place
	// in the snapshot should still be visible in the cadence file.
	c.logRead(now, err)
	if err != nil {
		var rl *RateLimit
		if errors.As(err, &rl) {
			// Keep the last reading and its age — only the cooldown moves.
			// The reading is still the newest fact anyone has, and whether
			// it is too old to act on is the caller's question, not ours.
			e.RetryAt = now.Add(planCooldown(rl.RetryAfter))
			c.share(r, e)
		}
		return nil, time.Time{}, err
	}
	c.share(r, planEntry{At: now, Windows: u})
	return u, now, nil
}

// share is store with credpin.go rule 5 in front of it: an answer only
// becomes the instance's fact when the reader that fetched it was still
// pointed at the compiled-in endpoint (ranger-base-dr6u).
//
// It gates the 429 branch as well as the success one, and for the same
// reason: a cooldown IS a fact the whole fleet acts on, so a loopback
// listener answering `429 Retry-After: 3600` would park every posse process
// on the machine for an hour without ever being asked for a credential.
//
// The caller gets its own reading regardless — an override still works, it
// just works for the process that set it. Nothing is said out loud here:
// this is not a refusal, it is the snapshot declining to adopt a stranger,
// and plan-usage.log already records that the request happened.
func (c *PlanCache) share(r PlanReader, e planEntry) {
	if r == nil || !r.MayShare() {
		return
	}
	c.store(e)
}

// LastReadAt is when the shared snapshot's reading was TAKEN, and whether
// there is one. It makes no request and it never falls back to now.
//
// It exists so the blind window is answerable by a process that is not the
// watch loop (the governance surface's G5, run from a fresh shell). The
// guard's own blindSince is per-process memory; this file is the instance's
// record of the same fact, written every time a reading succeeds — so "how
// long have we been blind" has one answer on the machine rather than one per
// process. A snapshot holding only a cooldown is not a reading and says so.
func (c *PlanCache) LastReadAt() (time.Time, bool) {
	e, have := c.load()
	if !have || e.At.IsZero() || len(e.Windows) == 0 {
		return time.Time{}, false
	}
	return e.At, true
}

// Line is the reading as a person reads it: `plan windows: ` and whatever
// the adapter's windows are called, plus how old the snapshot is once that
// is worth saying. One
// rendering for the tail of `posse cost` and for `posse cost --plan`, so a
// persona greps the same bytes either way.
//
// The age matters more than it looks: a shared snapshot (rangerhq-tdy8) is
// routinely minutes old, and a number presented as newer than it is, is the
// one way this display can lie.
func (c *PlanCache) Line(maxAge time.Duration) (string, error) {
	u, at, err := c.Read(maxAge)
	if err != nil {
		return "", err
	}
	age := ""
	if d := c.now().Sub(at); d >= time.Minute {
		age = fmt.Sprintf(", read %s ago", BlindFor(d))
	}
	return fmt.Sprintf("plan windows: %s%s", u.Line(), age), nil
}

// planCooldown turns a Retry-After into how long every process waits.
func planCooldown(d time.Duration) time.Duration {
	switch {
	case d <= 0:
		return planCooldownDefault
	case d > planCooldownMax:
		return planCooldownMax
	}
	return d
}

func (c *PlanCache) load() (planEntry, bool) {
	var e planEntry
	if c.Path == "" {
		return e, false
	}
	b, err := os.ReadFile(c.Path)
	if err != nil {
		return e, false
	}
	// A truncated or hand-edited file is a cache miss, never a crash and
	// never a wrong number: json leaves e zeroed and the caller fetches.
	if err := json.Unmarshal(b, &e); err != nil {
		return planEntry{}, false
	}
	// A snapshot with no windows is not a reading: it is a pre-seam file, a
	// truncated one, or a 429 entry that never held one. Only the cooldown
	// survives — a rate limiter must be honoured off any entry that names
	// one, and that is the one fact here that does not need a reading.
	if len(e.Windows) == 0 && e.RetryAt.IsZero() {
		return planEntry{}, false
	}
	return e, true
}

// store replaces the snapshot atomically — a reader in another process sees
// the old file or the new one, never half of either.
func (c *PlanCache) store(e planEntry) {
	if c.Path == "" {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.CreateTemp(dir, ".plan-usage-*.json")
	if err != nil {
		return
	}
	tmp := f.Name()
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		os.Remove(tmp)
		return
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, c.Path); err != nil {
		os.Remove(tmp)
	}
}

const (
	// planLogMax is when the read log gets trimmed, and planLogKeep is what
	// survives. Bounded by the TTL to a few hundred lines a day, so the
	// trim is a backstop against a misconfigured TTL, not a routine event.
	planLogMax  = 128 << 10
	planLogKeep = 1000
)

// logRead records one request that actually went out — cache hits are not
// requests and write nothing, which is the point of the file: its cadence
// IS the endpoint's view of us.
func (c *PlanCache) logRead(now time.Time, err error) {
	if c.Log == "" {
		return
	}
	caller := c.Caller
	if caller == "" {
		caller = "-"
	}
	outcome := "ok"
	if err != nil {
		var rl *RateLimit
		if errors.As(err, &rl) {
			outcome = fmt.Sprintf("%s cooldown=%s", statusCode(rl.Status), BlindFor(planCooldown(rl.RetryAfter)))
		} else {
			// planusage.go's errors are written to be quotable: generic by
			// construction, never the token and never a header.
			outcome = "failed: " + err.Error()
		}
	}
	line := fmt.Sprintf("%s %s %s\n", now.UTC().Format(time.RFC3339), caller, outcome)
	if err := os.MkdirAll(filepath.Dir(c.Log), 0o755); err != nil {
		return
	}
	f, ferr := os.OpenFile(c.Log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if ferr != nil {
		return
	}
	f.WriteString(line)
	f.Close()
	trimReadLog(c.Log)
}

// statusCode is "429" out of "429 Too Many Requests" — the log wants the
// code, not the reason phrase.
func statusCode(status string) string {
	if f := strings.Fields(status); len(f) > 0 {
		return f[0]
	}
	return "?"
}

// trimReadLog keeps the newest planLogKeep lines once a provider-probe log
// passes planLogMax. Newest, because both logs answer questions about the
// last few hours.
func trimReadLog(path string) {
	st, err := os.Stat(path)
	if err != nil || st.Size() <= planLogMax {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > planLogKeep {
			lines = lines[1:]
		}
	}
	f.Close()
	if err := sc.Err(); err != nil {
		return
	}
	if err := os.WriteFile(path+".tmp", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		os.Remove(path + ".tmp")
		return
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		os.Remove(path + ".tmp")
	}
}

// PlanUsageTTL reads config `plan_usage_ttl:` (the house's duration form:
// 5m, 300, 0). Unset = PlanUsageTTLDefault. **0 is the operator's escape
// hatch**: no sharing, every caller asks the endpoint for itself, which is
// the behaviour that produced rangerhq-tdy8 — Retry-After is still
// honoured. A value that is not a duration is named on errw and the default
// stands, the same rule the other plan-guard settings use: a typo must be
// visible, and the visible failure is the safe one.
func (a *App) PlanUsageTTL(errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "plan_usage_ttl"))
	if raw == "" {
		return PlanUsageTTLDefault
	}
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
		return d
	}
	fmt.Fprintf(errw, "plan guard: config plan_usage_ttl: %q is not a duration (5m, seconds, or 0) — using %s\n",
		raw, BlindFor(PlanUsageTTLDefault))
	return PlanUsageTTLDefault
}

// planGuardMaxAge is how stale a reading the *guard* may decide a pass on,
// which is not the same question as how stale a reading the header may
// show. The guard fails closed past `plan_guard_blind_max:`, and the blind
// clock counts from the reading it acted on — so a cache hit must never be
// able to spend more than half that budget on its own. Everything it does
// not spend stays available for a real outage.
func planGuardMaxAge(ttl, blindMax time.Duration) time.Duration {
	if blindMax > 0 && ttl > blindMax/2 {
		return blindMax / 2
	}
	return ttl
}
