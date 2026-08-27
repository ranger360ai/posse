package rhq

// Plan-utilization guard (rangerhq-jgm) — the one budget signal that is not
// a proxy. On a subscription plan the constraint is not API-equivalent
// dollars (`posse cost`, ADR 0003 §4) but the plan's own rate windows, each
// reported as a utilization percentage. The point of watching them is the
// operator's interactive headroom — a fleet that eats the tightest window
// leaves the person at the keyboard staring at a rate limit.
//
// This file is the guard's HARNESS half (ADR 0012 D1/D4): the window shape,
// the thresholds, the blind clock, the fail-open rule. It does not know what
// a window is called, which provider reports one, or how a credential for
// one is read. That is an adapter's job, behind the PlanReader seam below,
// and the harness ships exactly one (planusage_anthropic.go).
//
// The split is the ADR's line drawn at the one place it is load-bearing:
// mechanism is harness, "the provider's usage endpoint and credential read"
// is an adapter, and thresholds and the credential are the instance's. An
// instance whose provider has no adapter runs the guard OFF — and says so.
//
// Everything here is fail-open: a monitoring failure never halts the fleet.

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Window is one rate window's utilization, named by the adapter that read
// it. The name is display text and the suffix of a config key
// (`plan_guard_<name>:`) — nothing outside an adapter may branch on it.
type Window struct {
	Name string  `json:"name"`
	Pct  float64 `json:"pct"`
}

// PlanUsage is one reading of a plan's rate windows, in the order the
// adapter reports them. That order is meaning, not presentation: the guard
// trips on the FIRST window over its threshold, so an adapter lists the
// window whose exhaustion hurts most first.
//
// A nil PlanUsage is "not consulted" — the zero value every caller that did
// not read a meter hands on.
type PlanUsage []Window

// Line is the one-line rendering for `posse cost` and the cockpit header —
// `<window> 42% · <window> 61%`, in the adapter's own vocabulary. No
// history: this is the current reading or nothing.
func (u PlanUsage) Line() string {
	if len(u) == 0 {
		return "no windows"
	}
	parts := make([]string, 0, len(u))
	for _, w := range u {
		parts = append(parts, fmt.Sprintf("%s %.0f%%", w.Name, w.Pct))
	}
	return strings.Join(parts, " · ")
}

// PlanReader is the plan-window seam (ADR 0012 D4): one provider's usage
// read, and the only place a provider's endpoint, credential or window
// vocabulary is allowed to appear. Everything downstream — thresholds, the
// blind clock, Dial E's tightest-window arithmetic, the header, `posse
// cost` — is label-agnostic and gets no say in what a window is called.
//
// Two methods, because two are all the harness ever needed from the
// concrete reader this replaced.
type PlanReader interface {
	// Read fetches the current utilization of every window this provider
	// reports. A *RateLimit error is the one failure a caller must not
	// answer by asking again (plancache.go).
	Read() (PlanUsage, error)
	// MayShare reports whether a reading this reader produces may become
	// the instance's shared fact — `$StateDir/plan-usage.json`, which every
	// posse process on the machine reads for the TTL (rangerhq-tdy8).
	// credpin.go rule 5: only the compiled-in endpoint's answers may.
	//
	// FALSE is the safe answer and the one a reader nobody vouched for must
	// give: an unvouched reading is nobody's fact. It gates the STORE, not
	// the load.
	MayShare() bool
}

// planAdapter is one shipped implementation of that seam: what it is
// called, whether this machine can run it, and how to build it.
type planAdapter struct {
	Name string
	// Unavailable says why this adapter cannot serve this machine, or nil.
	// Checked BEFORE New, so the reason can be reported without building a
	// reader whose only future is to fail.
	Unavailable func() error
	New         func() PlanReader
}

// planAdapters is the shipped list, in preference order. One entry today —
// the plan guard's entire provider surface. A second provider is a new file
// and a line here; no caller of PlanReader changes.
//
// A var, not a const list, for one reason beyond taste: the tests that
// prove this seam is a seam replace it — with a fake two-window adapter
// whose windows are named nothing like the shipped one's, and with the
// empty list that is an instance no shipped adapter can serve.
var planAdapters = []planAdapter{anthropicPlanAdapter}

// NoPlanAdapter is the guard with no implementation to run: not a failed
// reading, not a credential outage — no meter on this machine at all.
//
// It is a distinct type because the two states it must never be confused
// with are the two the plan guard already distinguishes at some cost.
// "Blind" means a meter exists and could not be read, and blindness has a
// clock on it that eventually stops an unattended fleet (ADR 0018). Failing
// a pass closed against a provider posse was never able to meter would be a
// brake with no release. And "unarmed" means nobody asked for a guard,
// which is silence on purpose. This is the third thing, and it gets said
// out loud: the operator armed a guard that cannot exist here.
type NoPlanAdapter struct{ Why string }

func (e *NoPlanAdapter) Error() string { return e.Why }

// PlanAdapter returns the adapter this instance runs, or the reason no
// shipped one can serve it. The reason is a FACT the caller reports, never
// a nil it swallows: posse has already paid once for a monitoring silence
// that read like health (planusage_anthropic.go's GateRefusal has the
// receipt).
func PlanAdapter() (PlanReader, error) {
	var why []string
	for _, a := range planAdapters {
		if a.Unavailable != nil {
			if err := a.Unavailable(); err != nil {
				why = append(why, fmt.Sprintf("%s (%v)", a.Name, err))
				continue
			}
		}
		return a.New(), nil
	}
	if len(why) == 0 {
		return nil, &NoPlanAdapter{Why: "no plan-window adapter is compiled in"}
	}
	return nil, &NoPlanAdapter{Why: "no plan-window adapter serves this machine: " + strings.Join(why, "; ")}
}

// RateLimit is the endpoint saying "not now": 429, or a 503 that carries
// Retry-After. It is a distinct type because it is the one read failure a
// caller must not answer by asking again — plancache.go turns it into a
// cooldown every posse process honours (rangerhq-tdy8).
//
// RetryAfter is what the header said, and 0 means the response did not say.
// How long to wait when the endpoint did not say is a policy and belongs to
// the caller, not to this fact.
type RateLimit struct {
	Status     string
	RetryAfter time.Duration
}

func (e *RateLimit) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("usage endpoint returned %s, retry after %s", e.Status, BlindFor(e.RetryAfter))
	}
	return fmt.Sprintf("usage endpoint returned %s", e.Status)
}

// retryAfter parses the header's two forms (RFC 9110 §10.2.3): delta
// seconds, or an HTTP date. Anything else — including a date already past
// — is "the endpoint did not say".
func retryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n <= 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// planGuardReserved are the `plan_guard_` config keys that are NOT window
// thresholds. Every other key under that prefix is read as
// `plan_guard_<window>:` — which is how a threshold can name a window this
// harness has never heard of, and how the shipped adapter's own two keys
// stopped being special cases in posse's source.
//
// A new non-window `plan_guard_` setting must be added here. Forgetting
// costs one "is not a percent" line on stderr per pass, which is loud, and
// loud is the intended failure: the alternative is a setting silently read
// as a threshold for a window nobody has.
var planGuardReserved = map[string]bool{
	"blind_max":    true,
	"overflow":     true,
	"overflow_cap": true,
}

// PlanGuardThresholds reads config `plan_guard_<window>:` (percent), keyed
// by window name. No key set — the default — means the guard is off and
// nothing is ever fetched. A value that is not a percent is reported on
// errw and dropped: a typo must be visible, not a silently disabled guard.
//
// The names are the operator's, and this function does not know whether any
// of them is real. Matching them against the windows an adapter actually
// reports is dispatch.planGuard's job, and a threshold that matches nothing
// gets said out loud there for the same reason a malformed one does here.
func (a *App) PlanGuardThresholds(errw io.Writer) map[string]float64 {
	var th map[string]float64
	for _, key := range YamlKeysWithPrefix(a.ConfigPath, "plan_guard_") {
		name := strings.TrimPrefix(key, "plan_guard_")
		if name == "" || planGuardReserved[name] {
			continue
		}
		v := a.planPercent(key, errw)
		if v <= 0 {
			continue
		}
		if th == nil {
			th = make(map[string]float64)
		}
		th[name] = v
	}
	return th
}

func (a *App) planPercent(key string, errw io.Writer) float64 {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, key))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64)
	if err != nil || v <= 0 || v > 100 {
		fmt.Fprintf(errw, "plan guard: config %s: %q is not a percent 1–100 — guard off for that window\n", key, raw)
		return 0
	}
	return v
}

// PlanGuardBlindMaxDefault is how long an unattended pass may run without a
// successful reading before it stops running at all (rangerhq-6h1).
//
// The rationale, not the measurement: a fleet at full fan-out burns the
// tightest window at a roughly steady rate, so the gap between a sane threshold and
// the rate limit itself buys a bounded number of minutes of *blind*
// dispatch. The default spends about a third of that gap and keeps the
// rest — a deployer who has measured their own burn rate against their own
// threshold should set `plan_guard_blind_max:` from those two numbers
// rather than inherit this one.
const PlanGuardBlindMaxDefault = 10 * time.Minute

// PlanGuardBlindMax reads config `plan_guard_blind_max:` (the house's
// duration form: 30s, 5m, or bare seconds). Unset = the default above.
// **0 is the operator's escape hatch**: never fail closed, which is
// pre-6h1 behaviour everywhere, attended or not. A value that is not a
// duration is named on errw and the default stands — the plan guard's rule
// for a malformed threshold, for the same reason: a typo must be visible,
// and here the visible failure is the safe one.
func (a *App) PlanGuardBlindMax(errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "plan_guard_blind_max"))
	if raw == "" {
		return PlanGuardBlindMaxDefault
	}
	// ParseInterval is the same grammar but rejects zero (a --watch interval
	// of 0 is nonsense); here zero is the whole point, so parse it directly.
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
		return d
	}
	fmt.Fprintf(errw, "plan guard: config plan_guard_blind_max: %q is not a duration (30s, 5m, or seconds) — using %s\n",
		raw, BlindFor(PlanGuardBlindMaxDefault))
	return PlanGuardBlindMaxDefault
}

// BlindFor renders a blind duration for a log line and the cockpit header:
// whole minutes once past one ("12m"), seconds below it, hours above. The
// number is a witness, not a measurement — no decimals.
func BlindFor(d time.Duration) string {
	switch {
	case d < 0:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}
