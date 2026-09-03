package posse

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
//     minutes is how a storm gets extended — and honouring the header
//     exactly and re-asking at the boundary is how one gets EXTENDED
//     INDEFINITELY, which is why the wait doubles per consecutive 429 and
//     resets on the first success (planCooldown, ranger-base-rwwp6).
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
	// planCooldownCeiling caps the ESCALATION below: the longest this file
	// will keep the instance quiet, however many 429s in a row it has seen.
	//
	// Eight hours is three asks a day at the worst, so the sentence above
	// still holds — the guard never stops asking for a day, and the blind
	// window still decides what to do about the outage. It is also the
	// doubling the endpoint's own number lands on (1h → 2h → 4h → 8h), not
	// a figure invented next to it.
	planCooldownCeiling = 8 * time.Hour
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
	// Streak is how many 429s in a row this snapshot has seen and Wait is
	// what the newest of them bought — the two facts planCooldown escalates
	// from. They ride in the shared file for the reason the cooldown does:
	// the Nth 429 is usually a different process from the first, and a
	// streak each process counted for itself would escalate nowhere.
	//
	// A successful read replaces the whole entry, which is what resets both
	// (Read's success branch), and a snapshot written before these fields
	// existed decodes to zeroes — the first 429 after an upgrade is honoured
	// verbatim, exactly as it was before.
	Streak int           `json:"cooldown_streak,omitempty"`
	Wait   time.Duration `json:"cooldown_wait,omitempty"`
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
	// Quiet is the instance's ruling that the endpoint is not to be asked
	// at all — the guard is off, or `plan_usage_quiet: true`
	// (planquiet.go). Nil is the normal state.
	//
	// It is a field on the CACHE and not a check in each caller because
	// each caller is exactly what failed: on 2026-09-02 two cockpit ticks
	// re-armed a 429 window an operator had spent 94 minutes draining, and
	// they did it through this struct, past a dispatcher and a governance
	// surface that were each checking the thresholds for themselves. One
	// path to the endpoint, one place that can refuse it
	// (ranger-base-4rfw1).
	Quiet *PlanQuiet
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
		// io.Discard: a malformed `plan_usage_quiet:` is named by the
		// surfaces that own a stderr (dispatch's planGuard), once, and a
		// cockpit tick owns the whole terminal and cannot write to one at
		// all. The safe reading of a typo is "not quiet" either way
		// (PlanMeterQuiet).
		Quiet: a.PlanMeterQuiet(io.Discard),
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
// setting turns it off. Nor does it outrank Quiet, for the same reason and
// one more: a caller that asks for a fresh reading while the shop has
// stopped metering is the caller this file exists to stop.
func (c *PlanCache) Read(maxAge time.Duration) (PlanUsage, time.Time, error) {
	// Quiet first, ahead of every other question (ranger-base-4rfw1). Not
	// even the adapter is consulted: whether this box HAS a meter is a
	// question nobody asked, and answering it here would put a second
	// sentence on a state that has one.
	//
	// A snapshot the caller would have accepted anyway is still served:
	// quiet is "do not ask", not "forget", and refusing a reading this
	// process would have taken as a cache hit a second earlier would make
	// the flag a brake instead of a mute.
	//
	// Past that age it is a REFUSAL and not a stale number returned as a
	// fresh one. Read's guarantee — what comes back is no older than
	// maxAge — is the thing every caller has built on, and quietly widening
	// it for one flag is how a guard ends up ruling on a nineteen-hour-old
	// reading it thinks is current (ranger-base-c3vqe). Surfaces that want
	// the old reading anyway ask for it by name, with its age, through
	// LastReading.
	if c.Quiet != nil {
		if e, have := c.load(); have && planFresh(e, c.now(), maxAge) {
			return e.Windows, e.At, nil
		}
		return nil, time.Time{}, c.Quiet
	}
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
	if have && planFresh(e, now, maxAge) {
		return e.Windows, e.At, nil
	}
	if have && now.Before(e.RetryAt) {
		return nil, time.Time{}, &planCooldownErr{Left: e.RetryAt.Sub(now)}
	}
	u, err := r.Read()
	var rate planRate
	var rl *RateLimit
	if errors.As(err, &rl) {
		// Keep the last reading and its age — only the cooldown moves. The
		// reading is still the newest fact anyone has, and whether it is too
		// old to act on is the caller's question, not ours.
		e.Streak++
		e.Wait = planCooldown(rl.RetryAfter, e.Wait)
		rate = planRate{Asked: rl.RetryAfter, Wait: e.Wait, Streak: e.Streak}
		e.RetryAt = now.Add(e.Wait)
	}
	// The read log is written either way. A request that left the machine is
	// evidence whoever answered it, and an override that is refused a place
	// in the snapshot should still be visible in the cadence file.
	c.logRead(now, err, rate)
	if err != nil {
		if rl != nil {
			c.share(r, e)
		}
		return nil, time.Time{}, err
	}
	// A success replaces the entry, streak and all: proof the endpoint
	// answers is proof the escalation has nothing left to escalate.
	c.share(r, planEntry{At: now, Windows: u})
	return u, now, nil
}

// planRate is what one 429 cost, carried from the branch that decides it to
// the log line that records it: what the endpoint ASKED for, what this file
// honoured, and which consecutive 429 it was. The three differ now that the
// honoured wait escalates, and a log that shows only the third of them
// cannot be read backwards into what the endpoint actually said
// (ranger-base-rwwp6).
type planRate struct {
	Asked  time.Duration // the Retry-After header, 0 = the endpoint named none
	Wait   time.Duration // what every process on this box will honour
	Streak int           // 1 = the first 429 of this storm
}

// planCooldownErr is the refusal to ask again while a Retry-After the endpoint
// asked for is still running. Its SENTENCE is its own — nobody asked the
// endpoint anything this time, so quoting a status line would be a fiction —
// and its CLASS is still the 429 that bought it, which is what the *RateLimit
// underneath is for: a surface that names failure classes (PlanFailureOf)
// must name the hour after a 429 the same way it names the 429, or the
// cockpit header goes back to saying "blind" for the whole tail of it (bead
// rangerhq-pwpx).
//
// Nothing else changes with it. plancache's own 429 branch and its read log
// are only reached by a read that was actually made, and this returns before
// either.
type planCooldownErr struct {
	Left time.Duration
	rl   RateLimit
}

func (e *planCooldownErr) Error() string {
	return fmt.Sprintf("usage endpoint rate-limited, not asking again for %s", BlindFor(e.Left))
}

func (e *planCooldownErr) Unwrap() error { return &e.rl }

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
	_, at, ok := c.LastReading()
	return at, ok
}

// LastReading is the whole of that snapshot — the WINDOWS as well as when
// they were taken. Same rules as LastReadAt, which is now this function
// answering half the question: no request, no fallback to now, and a
// snapshot holding only a cooldown is not a reading.
//
// The windows are needed because the last reading is the only thing a blind
// meter still has to say (blindheadroom.go). It is a snapshot and it is
// treated as one — a hint about the past, never a number to render as the
// present (Helland: data outside its store of record "is clearly from the
// past and not now"). Nothing here interpolates, ages or extrapolates it;
// the caller that acts on it says out loud how old it is.
func (c *PlanCache) LastReading() (PlanUsage, time.Time, bool) {
	e, have := c.load()
	if !have || e.At.IsZero() || len(e.Windows) == 0 {
		return nil, time.Time{}, false
	}
	return e.Windows, e.At, true
}

// Cooling is the live cooldown off the shared snapshot: how long until any
// process on this box may ask again. False once it has expired, and false
// for a snapshot that never had one.
//
// Like LastReadAt it makes no request and reads nothing but the file — it
// exists so the loud line (planstale.go) can say how long the shop has
// chosen to stay quiet. An escalating wait that nothing printed would be the
// silent mute planCooldownMax's comment refuses.
func (c *PlanCache) Cooling(now time.Time) (time.Duration, bool) {
	e, have := c.load()
	if !have || e.RetryAt.IsZero() || !now.Before(e.RetryAt) {
		return 0, false
	}
	return e.RetryAt.Sub(now), true
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

// planCooldown turns a Retry-After into how long every process waits: prev
// is the wait already in force from the last 429, and 0 means this is the
// first of a storm.
//
// The FIRST one is honoured exactly as it was before: the endpoint's own
// number is the best information anybody has about the endpoint, and an
// isolated 429 asking for sixty seconds should cost sixty seconds of
// blindness, not two minutes. What changed is the REPEAT.
//
// Why the repeat cannot be honoured the same way (ranger-base-rwwp6, off
// spike ranger-base-dvxac). On 2026-09-02 this instance drew fourteen
// consecutive 429s between 03:30Z and 16:35Z, each naming Retry-After 3600,
// and three of the asks that drew one were made AFTER the window the
// previous 429 stated had ended — by 29s, by 28s, by 118s. Read with
// ranger-base-au0o4, which watched the window END move when it asked, the
// likely shape is that every ask re-arms the hour: a poller that waits
// exactly one stated window and then asks is then a loop that cannot
// terminate, and the plan guard stays blind for as long as it keeps trying.
// It was blind for thirteen hours that day. The competing reading — that the
// real window is simply longer than the header it sends — is not ruled out
// (the clean experiment needs the poller stopped, ranger-base-uzyd2), and
// gives the same instruction, which is why the fix does not depend on which
// is true.
//
// So the honoured wait doubles per consecutive 429 and resets on the first
// success: 1h, 2h, 4h, 8h. Two asks in, the cadence is no longer the
// endpoint's window, which is the only property that matters — whatever is
// re-arming an hour cannot be re-armed by a request that is not made.
//
// It does not lift planCooldownMax, and the ceiling is why. That constant
// refuses "a guard that stops asking for a day", and this refuses it too:
// the endpoint is never believed past an hour on any single 429, and the
// escalation stops at planCooldownCeiling — three asks a day at its worst.
// The other half of the answer is loudness, not arithmetic: the wait is on
// the blind line (planstale.go) and the streak and the raw Retry-After are
// in the cadence log, so an escalation an operator did not choose is one
// they can still see.
func planCooldown(d, prev time.Duration) time.Duration {
	wait := d
	switch {
	case wait <= 0:
		wait = planCooldownDefault
	case wait > planCooldownMax:
		wait = planCooldownMax
	}
	// The escalation doubles the wait IN FORCE, not the header — so a storm
	// cannot walk backwards down its own schedule when one 429 in the middle
	// of it names a shorter window or none at all. That is not a hypothetical
	// tidy-up: two hours in, a 429 with no header would otherwise be honoured
	// as the five-minute default, and re-asking five minutes into an hour the
	// endpoint has already stated is the exact behaviour this bead is about.
	// Within one storm the honoured wait only ever grows, and only a success
	// takes it back to nothing.
	if prev > 0 && prev*2 > wait {
		wait = prev * 2
	}
	if wait > planCooldownCeiling {
		return planCooldownCeiling
	}
	return wait
}

// planFresh is the one cache-hit test: is this snapshot young enough for a
// caller that will act on readings no older than maxAge?
//
// maxAge 0 is "fresh only" and no stored entry satisfies it. A reading
// stamped in the future is not fresh either — a clock that moved backwards
// is a snapshot nobody can age, and treating it as new would pin the whole
// instance on it.
//
// It is a function because two branches ask it now (the normal path and the
// quiet one), and the failure mode of a second copy is a quiet reader
// accepting a reading the guard beside it would refuse.
func planFresh(e planEntry, now time.Time, maxAge time.Duration) bool {
	if maxAge <= 0 || e.At.IsZero() {
		return false
	}
	age := now.Sub(e.At)
	return age >= 0 && age < maxAge
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
func (c *PlanCache) logRead(now time.Time, err error, rate planRate) {
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
			// `429 cooldown=2h00m retry-after=1h00m streak=2`. The first
			// two fields are the bytes planLogClass and the 08-24 pins
			// already read; the rest is what an escalating wait costs a
			// reader who has only this file. cooldown= is what the box
			// honoured, retry-after= is what the endpoint asked for — before
			// this bead they were the same number and the log said the
			// endpoint had asked for an hour when it may have asked for a
			// day (rangerhq-tdy8 is reconstructed from exactly these lines).
			outcome = fmt.Sprintf("%s cooldown=%s retry-after=%s streak=%d",
				statusCode(rl.Status), BlindFor(rate.Wait), askedFor(rate.Asked), rate.Streak)
		} else {
			// planusage.go's errors are written to be quotable: generic by
			// construction, never the token and never a header.
			outcome = "failed: " + err.Error()
			// …and the class as a TOKEN after it, when the failure has one
			// (ranger-base-lpoui). The sentence is for a person; this is
			// for the reader that counts a failure streak hours later and
			// must say what kind it was (planstale.go planLogClass) without
			// matching on the sentence — the rule AuthFailure, RateLimit
			// and GateRefusal each got a type to keep. Appended rather than
			// prefixed so `<caller> failed: ` stays the bytes the 08-24
			// misdiagnosis pin greps for.
			if tok := PlanFailToken(err); tok != "" {
				outcome += " [" + tok + "]"
			}
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

// askedFor renders a Retry-After the endpoint may not have sent. "none" and
// not "0s": the difference between "the endpoint asked for nothing" and "the
// endpoint asked for no time at all" is the difference between policy and
// header, and the log is the instrument that has to keep them apart.
func askedFor(d time.Duration) string {
	if d <= 0 {
		return "none"
	}
	return BlindFor(d)
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
