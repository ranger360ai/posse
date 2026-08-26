package rhq

// Plan-utilization guard (rangerhq-jgm) — the one budget signal that is not
// a proxy. On a subscription plan the constraint is not API-equivalent
// dollars (`posse cost`, ADR 0003 §4) but the plan's own rate windows: a
// rolling five-hour session window and a seven-day one, each reported as a
// utilization percentage. The point of watching them is the operator's
// interactive headroom — a fleet that eats the 5h window leaves the person
// at the keyboard staring at a rate limit.
//
// Source: GET https://api.anthropic.com/api/oauth/usage with the Claude Code
// OAuth access token from the macOS keychain (the business manager's
// plan-usage.sh is the reference method). This is the Anthropic adapter of
// the plan-guard seam (ADR 0012 D1/D4); a provider without one runs the
// guard off. The token is read into memory for the one request and never
// written anywhere — not to logs, meta files, or bead comments; the errors
// this file returns are deliberately generic for that reason.
//
// Everything here is fail-open: a monitoring failure never halts the fleet.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	// PlanUsageURL is the OAuth usage endpoint; RHQ_PLAN_USAGE_URL overrides
	// it (tests point it at a local server), and that override is honoured
	// only when it names loopback — credpin.go has the why.
	PlanUsageURL = "https://api.anthropic.com/api/oauth/usage"
	// PlanUsageHost is the host that answers it: the one host, besides this
	// machine, this process puts the account's credential in front of.
	PlanUsageHost = "api.anthropic.com"
	// KeychainService is the macOS keychain item Claude Code stores its
	// OAuth credentials under.
	KeychainService = "Claude Code-credentials"
	planBetaHeader  = "oauth-2025-04-20"
)

// PlanUsage is one reading of the plan's rate windows, in percent.
type PlanUsage struct {
	FiveHour float64
	SevenDay float64
}

// Line is the one-line rendering for `posse cost` and the cockpit header.
// No history: this is the current reading or nothing.
func (u PlanUsage) Line() string {
	return fmt.Sprintf("5h %.0f%% · 7d %.0f%%", u.FiveHour, u.SevenDay)
}

// PlanReader reads the windows. Token and HTTP are fields so tests can
// inject a fake keychain and a fake endpoint; nothing else needs to.
//
// This is the function rangerhq-25p asked for: Dial E's step-down may take
// max(cost-window %, plan-window %) — 25p decides, this bead only exposes
// Read.
type PlanReader struct {
	URL   string
	Token func() (string, error)
	HTTP  *http.Client
	// URLErr is a refused RHQ_PLAN_USAGE_URL (credpin.go), carried here
	// rather than returned by NewPlanReader: every caller of that wants a
	// reader, and the honest place to say "no reading, and why" is the one
	// that would have made the request. Non-nil = Read refuses and asks
	// nobody — no keychain read, no request.
	URLErr error
	// Now is the clock a Retry-After date is measured against (nil =
	// time.Now). Only the HTTP-date form of that header needs it.
	Now func() time.Time
}

func (r *PlanReader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
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

func NewPlanReader() *PlanReader {
	r := &PlanReader{
		URL:   PlanUsageURL,
		Token: KeychainToken,
		HTTP:  pinnedClient(10*time.Second, "usage endpoint", PlanUsageHost),
	}
	if raw := os.Getenv("RHQ_PLAN_USAGE_URL"); raw != "" {
		if u, err := loopbackOverride("RHQ_PLAN_USAGE_URL", raw); err != nil {
			r.URLErr = err
		} else {
			r.URL = u
		}
	}
	return r
}

// GateRefusal is one of posse's OWN L1 gate shims refusing a command posse
// itself ran (ranger-base-r64). Every persona launch prepends that persona's
// shim dir to PATH (gates.go) and every crew PID denies Bash(security:*), so
// a `posse` command typed inside a persona pane resolves posse's keychain
// read to that persona's refusal shim and gets exit 1.
//
// It is a distinct type because the two things it is NOT are both worse than
// it: it is not a credential outage (the item was never reached), and it is
// not an availability answer. Reporting it as "keychain item unreadable" is
// byte-identical to a real outage — on 2026-08-24 that reading is what got
// plan_guard_blind_max: 0 set for hours, switching off the shop's only
// automated brake on a diagnosis that was wrong.
//
// Cmd is the shimmed binary; Rule is the deny: line that refused it, "" when
// the shim's stderr did not name one.
type GateRefusal struct {
	Cmd  string
	Rule string
}

func (e *GateRefusal) Error() string {
	if e.Rule != "" {
		return fmt.Sprintf("keychain read refused by a posse gate shim: %s (deny: %s) — posse's own gate, not a credential outage", e.Cmd, e.Rule)
	}
	return fmt.Sprintf("keychain read refused by a posse gate shim: %s — posse's own gate, not a credential outage", e.Cmd)
}

// gateRefusal reads an exec failure as a shim refusal, or returns nil. The
// shim writes "refused by posse gate: <cmd> <argv> (deny: <rule>)" to stderr
// and exits 1, and .Output() hands that stderr back on *exec.ExitError.
//
// Only the command name and the rule are lifted out. The rest of the line is
// argv, and this file's standing rule is that nothing it returns quotes a
// command's own bytes.
func gateRefusal(cmd string, err error) *GateRefusal {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return nil
	}
	for _, line := range strings.Split(string(ee.Stderr), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "refused by posse gate: ")
		if !ok {
			continue
		}
		g := &GateRefusal{Cmd: cmd}
		// The deny group is the tail of the line, and the rule inside it
		// has parens of its own — Bash(security:*) — so the closing one is
		// the LAST character, not the first ")" after the marker.
		if i := strings.LastIndex(rest, "(deny: "); i >= 0 {
			g.Rule = strings.TrimSuffix(rest[i+len("(deny: "):], ")")
		}
		return g
	}
	return nil
}

// KeychainToken pulls the Claude Code OAuth access token out of the macOS
// keychain. Errors never quote the command's output — that output is the
// credential blob.
func KeychainToken() (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", KeychainService, "-w").Output()
	if err != nil {
		if g := gateRefusal("security", err); g != nil {
			return "", g
		}
		return "", Die("keychain item %q unreadable", KeychainService)
	}
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(out, &creds); err != nil {
		return "", Die("keychain item %q is not the expected JSON", KeychainService)
	}
	if creds.ClaudeAiOauth.AccessToken == "" {
		return "", Die("keychain item %q has no claudeAiOauth.accessToken", KeychainService)
	}
	return creds.ClaudeAiOauth.AccessToken, nil
}

// Read fetches the current utilization of both windows.
func (r *PlanReader) Read() (PlanUsage, error) {
	var u PlanUsage
	if r.URLErr != nil {
		return u, r.URLErr
	}
	if r.Token == nil || r.URL == "" {
		return u, Die("plan reader not configured")
	}
	// Before the keychain, not after: a host posse does not credential is
	// answered by asking nobody and reading nothing (credpin.go).
	if err := pinnedEndpoint("usage endpoint", r.URL, PlanUsageHost); err != nil {
		return u, err
	}
	tok, err := r.Token()
	if err != nil {
		return u, err
	}
	req, err := http.NewRequest("GET", r.URL, nil)
	if err != nil {
		return u, Die("bad usage url")
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("anthropic-beta", planBetaHeader)
	cl := r.HTTP
	if cl == nil {
		cl = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := cl.Do(req)
	if err != nil {
		// A refused redirect comes back through here wrapped in *url.Error;
		// it is not an outage and must not be reported as one.
		var pin *PinRefusal
		if errors.As(err, &pin) {
			return u, pin
		}
		// A transport error can quote the URL but never a header.
		return u, Die("usage endpoint unreachable")
	}
	defer resp.Body.Close()
	// A redirect a client without our CheckRedirect followed: the answer
	// came from a host we do not credential, so it is not an answer.
	if err := pinnedResponse("usage endpoint", resp, PlanUsageHost); err != nil {
		return u, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return u, &RateLimit{Status: resp.Status, RetryAfter: retryAfter(resp.Header.Get("Retry-After"), r.now())}
	}
	if resp.StatusCode != http.StatusOK {
		return u, Die("usage endpoint returned %s", resp.Status)
	}
	var body struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
		} `json:"five_hour"`
		SevenDay struct {
			Utilization float64 `json:"utilization"`
		} `json:"seven_day"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return u, Die("usage response is not the expected JSON")
	}
	u.FiveHour = body.FiveHour.Utilization
	u.SevenDay = body.SevenDay.Utilization
	return u, nil
}

// PlanGuardThresholds reads config `plan_guard_5h:` / `plan_guard_7d:`
// (percent). Unset — the default — means the guard is off and nothing is
// ever fetched. A value that is not a percent is reported on errw and
// treated as unset: a typo must be visible, not a silently disabled guard.
func (a *App) PlanGuardThresholds(errw io.Writer) (fiveH, sevenD float64) {
	return a.planPercent("plan_guard_5h", errw), a.planPercent("plan_guard_7d", errw)
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
// The rationale, not the measurement: a fleet at full fan-out burns the 5h
// window at a roughly steady rate, so the gap between a sane threshold and
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
