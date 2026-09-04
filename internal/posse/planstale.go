package posse

// Meter blindness, said out loud (bead ranger-base-lpoui, the operator's
// 2026-09-02 option-C ruling on ranger-base-wkai3).
//
// ADR 0018 gave the blind meter a clock and a park, and ADR 0019 gave its
// failures a class. What neither gave the operator is the AGE of the
// reading the shop is still ruling on. On 2026-09-02 the instance's
// snapshot was stamped 03:23Z, every hourly re-ask since had 429'd — ten
// consecutively by 13:32Z — and the whole visible trace was one log line
// per hour and the pulse's bare token `guard-blind`. The headroom rule
// (blindheadroom.go) was deciding park-vs-degrade off a ten-hour-old
// number, correctly and silently, and nothing anywhere printed how old that
// number was.
//
// Silence is the defect. The reading is a snapshot, and Helland's rule for
// data outside its store of record is that it "is clearly from the past and
// not now" — a surface that acts on one owes the reader its timestamp. So
// this file computes one fact, from stores that already exist, and renders
// it as ONE line that `posse status`, the watch pass preamble and the
// cockpit all print byte-for-byte:
//
//	plan meter BLIND 10h09m: last reading 2026-09-02T03:23Z (5h 41% · 7d 89%)
//	— ruling on it under the headroom rule; 10 consecutive 429
//
// Observability only. Nothing here is read by the guard, the park, the
// degrade or Dial E — the behaviour is ADR 0018's and stays exactly where
// it is. This changes what the shop SAYS, never what it does.
//
// Two stores, each answering the half it owns:
//
//   - `$StateDir/plan-usage.json` — the reading and when it was taken
//     (plancache.go LastReading). Never aged forward, never extrapolated;
//     it is quoted as the past fact it is.
//   - `$StateDir/plan-usage.log` — how many requests have actually left the
//     machine since the last one that worked, and what the newest of them
//     was. The cadence file was written to be exactly this evidence ("next
//     time the endpoint 429s for hours, that file settles whether it was
//     us"), and a streak is a question only it can answer: the count is of
//     requests, not of ticks, so a cooldown that suppressed an ask does not
//     inflate it.
//
// The threshold is its own key, `plan_usage_stale_after:`, and deliberately
// not `plan_guard_blind_max:`. That one is a PARK boundary measured in
// minutes; this is a "say something" boundary measured in hours, and tying
// the second to the first would make an operator choose between a loud
// header and a fleet that keeps working.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// PlanUsageStaleAfterDefault is how old the last successful reading may get
// before every surface says so. Two hours against a five-hour rolling
// window is a reading that can no longer be assumed to describe now, and it
// is far enough above the TTL (5m) and `plan_guard_blind_max:` (10m) that a
// routine cache hit or a short outage never trips it — this is the line
// between "blind for a moment" and "ruling on yesterday".
const PlanUsageStaleAfterDefault = 2 * time.Hour

// PlanUsageStaleAfter reads config `plan_usage_stale_after:` (the house's
// duration form: 2h, 90m, bare seconds, or 0). Unset = the default above.
// **0 is the operator's escape hatch**: never say it, which is the
// behaviour before this bead. A value that is not a duration is named on
// errw and the default stands — the plan guard's rule for every threshold,
// for the same reason: a typo must be visible, and here the visible failure
// is the loud one.
func (a *App) PlanUsageStaleAfter(errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "plan_usage_stale_after"))
	if raw == "" {
		return PlanUsageStaleAfterDefault
	}
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
		return d
	}
	fmt.Fprintf(errw, "plan guard: config plan_usage_stale_after: %q is not a duration (2h, 90m, or seconds) — using %s\n",
		raw, BlindFor(PlanUsageStaleAfterDefault))
	return PlanUsageStaleAfterDefault
}

// PlanStale is the whole fact, computed once and rendered once. Stale is
// the only field a caller branches on; everything else is what the line
// says, kept as VALUES so a test can assert the arithmetic without reading
// the prose back out of the sentence.
type PlanStale struct {
	Stale   bool          // past `plan_usage_stale_after:` — the only thing to branch on
	At      time.Time     // when the last successful reading was TAKEN
	Age     time.Duration // now - At
	After   time.Duration // the threshold in force, for a caller that reports it
	Windows PlanUsage     // that reading, quoted as the past fact it is
	Fails   int           // requests that left the machine since the last one that worked
	Class   string        // the newest of those, as a token ("429", "401"); "" = no class
	// NextAsk is how long until any process may ask again, off the shared
	// snapshot's live cooldown; 0 = nothing is holding a request back.
	// It is the escalating 429 backoff said out loud (ranger-base-rwwp6):
	// the wait doubles per consecutive 429, and a wait the shop chose for
	// itself has to be visible to the operator whose fleet is blind for it.
	NextAsk time.Duration
	// Spender is why the meter is being read at all while the plan guard is
	// UNARMED (planquiet.go PlanMeterSpender); "" = the guard is armed, the
	// state this file was written for. Non-empty is ranger-base-ddivo's
	// state and it changes the SENTENCE, because in it no headroom rule is
	// ruling on anything — there is no threshold to trip and no park. What
	// is true is worse and has to be the words: the shop is spending, the
	// number it is spending against is this old, and nothing is braking on
	// it.
	Spender string
}

// Line is the one rendering — `posse status`, the watch pass preamble and
// the cockpit print these same bytes, so an operator greps one string and
// finds all three (govern.go's "one function, three renderings", applied to
// a line rather than a set).
//
// BLIND is upper-case on purpose. The header has said "guard blind" in
// lower case beside a dim clock since rangerhq-6h1, and ten hours of that
// read as furniture.
func (s PlanStale) Line() string {
	rule := "ruling on it under the headroom rule"
	if s.Spender != "" {
		rule = fmt.Sprintf("the plan guard is UNARMED (%s), so nothing is ruling on it", s.Spender)
	}
	return fmt.Sprintf("plan meter BLIND %s: last reading %s (%s) — %s; %s",
		BlindFor(s.Age), s.At.UTC().Format("2006-01-02T15:04Z"), s.Windows.Line(), rule, s.streak())
}

// streak is the tail clause: what the meter has been answering since. Three
// sentences and each is a fact off the cadence log — including the third,
// which is the one that looks like an absence and is not. Nought failed
// reads under a ten-hour-old snapshot does not mean the endpoint is fine;
// it means nothing has ASKED, and an operator who reads "10 consecutive
// 429" as the reason will go looking for the wrong outage.
func (s PlanStale) streak() string {
	var sentence string
	switch {
	case s.Fails == 0:
		sentence = "no request has left this machine since"
	case s.Class == "":
		sentence = fmt.Sprintf("%d consecutive failed reads", s.Fails)
	default:
		sentence = fmt.Sprintf("%d consecutive %s", s.Fails, s.Class)
	}
	// …and a fourth, when a cooldown is running: how long the box has
	// decided to stay quiet. It is appended to all three rather than folded
	// into the 429 one, because the two facts come from different stores and
	// the log one can be missing — a live cooldown with an unreadable log
	// still owes the reader the wait.
	if s.NextAsk > 0 {
		sentence += fmt.Sprintf(", next ask in %s", BlindFor(s.NextAsk))
	}
	return sentence
}

// PlanStaleness answers the question for one caller, from files only: no
// request, no credential, no clock but the one it is handed. Every surface
// that prints the line calls this, so `posse status`, the watch loop and
// the cockpit cannot disagree about the age — the same rule that put the
// blind clock in the shared snapshot rather than in each process's memory
// (plancache.go LastReadAt).
//
// Four ways it is not stale, and each is a state where the line would be a
// lie:
//
//   - `plan_usage_stale_after: 0` — the operator turned it off.
//
//   - The meter is QUIET (planquiet.go) — no `plan_guard_<window>:` AND
//     nothing spending, so there is no meter guard, no headroom rule ruling
//     on anything and no window being burnt; or `plan_usage_quiet: true`,
//     where the operator has stopped the asking on purpose and the guard is
//     off for the duration. Either way the snapshot gates nothing, and
//     "ruling on it under the headroom rule" would name a rule that is not
//     running. A cockpit and `posse cost` still show the reading with its
//     age; that is a display, not a brake.
//
//     An unarmed guard on a SPENDING box is not one of the four: the meter
//     is read there (ranger-base-ddivo) and an old reading is exactly the
//     thing that has to be loud, so it is stale like any other — with the
//     clause Line() forks on, because the sentence about the headroom rule
//     is the half that would be untrue.
//
//   - No reading in the snapshot. A machine that has never had one is not
//     ruling on a stale number, and ADR 0018's own reasoning parks nothing
//     on ignorance.
//
//   - The reading is inside the threshold, which is every healthy day.
//
// A snapshot stamped in the FUTURE (a clock step, a copied state dir) has a
// negative age and is not stale: it is a bad reading, not an old one, and
// BlindFor would render it as "0s" while the sentence claimed hours.
func (a *App) PlanStaleness(caller string, now time.Time, errw io.Writer) PlanStale {
	after := a.PlanUsageStaleAfter(errw)
	if after <= 0 {
		return PlanStale{After: after}
	}
	// The cache carries both halves of the meter state from one read — the
	// verdict this gates on, and the spender the sentence names when the
	// guard is unarmed (plancache.go, ranger-base-67mdf). Asking for either
	// separately would be a second read of a state that decays, and the two
	// answers that disagree are exactly the wrong line.
	c := a.PlanCache(caller)
	if c.Quiet != nil {
		return PlanStale{After: after}
	}
	last, at, ok := c.LastReading()
	if !ok {
		return PlanStale{After: after}
	}
	age := now.Sub(at)
	if age <= after {
		return PlanStale{At: at, Age: age, After: after, Windows: last, Spender: c.Spender}
	}
	fails, class := planFailStreak(c.Log)
	next, _ := c.Cooling(now)
	return PlanStale{Stale: true, At: at, Age: age, After: after, Windows: last, Fails: fails, Class: class, NextAsk: next, Spender: c.Spender}
}

// planFailStreak walks $StateDir/plan-usage.log backwards: how many
// requests in a row failed, and the class of the newest of them. A line
// whose outcome is `ok` ends the streak, which is the whole contract this
// has with logRead — every other outcome that file carries is a failure.
//
// Backwards from the tail, because the answer is about the last few hours
// and the file is trimmed to its newest thousand lines. An unreadable or
// absent log is 0 with no class: the line then says no request has left the
// machine, which is what a caller with no evidence may honestly say.
func planFailStreak(path string) (int, string) {
	if path == "" {
		return 0, ""
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, ""
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	if sc.Err() != nil {
		return 0, ""
	}
	n, class := 0, ""
	for i := len(lines) - 1; i >= 0; i-- {
		outcome := planLogOutcome(lines[i])
		if outcome == "" || outcome == "ok" {
			break
		}
		n++
		if class == "" {
			class = planLogClass(outcome)
		}
	}
	return n, class
}

// planLogOutcome is the third field onward of `<rfc3339> <caller>
// <outcome>` — the part logRead composed. A line of the wrong shape returns
// "", which ends the streak: a file this cannot parse is not evidence of
// failure, and over-counting here would put a number in front of an
// operator that no store backs.
func planLogOutcome(line string) string {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return ""
	}
	return strings.TrimSpace(parts[2])
}

// planLogClass reads the failure class back out of one outcome, by the two
// shapes logRead writes it in and never by matching on the sentence — the
// rule AuthFailure, RateLimit and GateRefusal each got a type to keep, and
// a log reader grepping for "401" would undo all three at once:
//
//   - `429 cooldown=5m` — the rate-limit form, whose first field IS the
//     status code (and has been since the cadence log was written).
//   - `failed: <sentence> [401]` — every other classed failure, whose token
//     logRead appends as a marker precisely so this never has to read the
//     sentence.
//
// Anything else is an unclassed failure: a dead socket, a 500, a body of
// the wrong shape. "" is right for those — the line says "failed reads",
// because failed is exactly what they are.
func planLogClass(outcome string) string {
	if i := strings.LastIndex(outcome, "["); i >= 0 && strings.HasSuffix(outcome, "]") {
		if tok := outcome[i+1 : len(outcome)-1]; tok != "" && !strings.ContainsAny(tok, " [") {
			return tok
		}
	}
	if f := strings.Fields(outcome); len(f) > 0 && isStatusCode(f[0]) {
		return f[0]
	}
	return ""
}

// isStatusCode is "three digits", the shape statusCode() puts at the head
// of a rate-limit outcome. Narrow on purpose: `failed:` must not be read as
// a class, and neither must a caller name.
func isStatusCode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// PlanFailToken is a read failure's class as a KEY or LOG segment: short,
// stable, machine-readable, and derived from the error's TYPE. It is
// PlanFailureOf's answer with the prose taken off — the constants there are
// sentences for a header ("credential stale (401)"), and a governance key
// and a log marker need a token.
//
// "" is not a fifth class. It is every failure that has none — a dead
// socket, a 500, a response of the wrong shape — and a caller that gets it
// appends nothing rather than inventing a word.
func PlanFailToken(err error) string {
	switch PlanFailureOf(err) {
	case PlanFailGated:
		return "gated"
	case PlanFailUnreadable:
		return "unreadable"
	case PlanFailStale:
		return "401"
	case PlanFailForbidden:
		return "403"
	case PlanFailRateLimited:
		// The status when the response carried one, so a 503-with-
		// Retry-After is not filed as a 429. A cooldown carries a zero
		// *RateLimit (plancache.go planCooldownErr) and gets the class's own
		// name — which is the rule pwpx already set: "a surface that names
		// failure classes must name the hour after a 429 the same way it
		// names the 429".
		var rl *RateLimit
		if errors.As(err, &rl) {
			if c := statusCode(rl.Status); isStatusCode(c) {
				return c
			}
		}
		return "429"
	}
	return ""
}

// blindHours is a blind duration as a KEY segment: whole hours, floored.
//
// The bucket is the escalation, and it is coarse on purpose. A pulse key
// that changed every minute would re-prompt the coordinator every tick and
// reset the renag backoff each time (pulse.go deliverPulse), which is a
// storm and not a warning; a key that never changes is what let ten hours
// pass with one delivery. An hour is the cadence the operator asked for.
func blindHours(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return strconv.Itoa(int(d.Hours())) + "h"
}
