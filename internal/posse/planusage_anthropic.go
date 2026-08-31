package posse

// The shipped plan-window adapter (ADR 0012 D4): Anthropic's OAuth usage
// endpoint, read with the Claude Code credential the credential seam hands
// it (credential.go, ADR 0019).
//
// This file is the provider surface and the ONLY place in posse that names
// this provider, its endpoint, or its window vocabulary — the store its
// credential comes out of is the seam's (credential.go), one layer down.
// Everything the guard does with a reading — thresholds, the
// blind clock, the tightest-window arithmetic, the header — lives in
// planusage.go and knows none of it. A second provider is another file like
// this one plus a line in planAdapters; nothing else moves.
//
// Source: GET https://api.anthropic.com/api/oauth/usage with the Claude Code
// OAuth access token (the business manager's plan-usage.sh is the reference
// method). The token is read into memory for the one request and never
// written anywhere — not to logs, meta files, or bead comments; the errors
// this file returns are deliberately generic for that reason.
//
// WHERE that token is read from is no longer this file's business: the
// credential seam (ADR 0019, ranger-base-x584) owns the per-platform store
// of record, and this adapter asks it for `MeterToken("claude")`. What is
// left here is the provider's endpoint, its window vocabulary, and nothing
// else — which is what makes a second provider a second file.
//
// Everything here is fail-open: a monitoring failure never halts the fleet.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	// PlanUsageURL is the OAuth usage endpoint; RHQ_PLAN_USAGE_URL overrides
	// it (tests point it at a local server), and that override is honoured
	// only when it names loopback — credpin.go has the why. It is also the
	// ONLY url this process credentials, byte for byte (credpin.go rule 4).
	PlanUsageURL = "https://api.anthropic.com/api/oauth/usage"
	// PlanUsageHost is the host that answers it: besides this machine, the
	// one host this process will ask at all.
	PlanUsageHost  = "api.anthropic.com"
	planBetaHeader = "oauth-2025-04-20"
)

// The window names this adapter reports, in reading order: the rolling
// five-hour session window first — it is the tighter one and the one the
// person at the keyboard feels — then the seven-day one. These two strings
// are what `plan_guard_5h:` / `plan_guard_7d:` match, and this is the only
// file in posse where either means anything.
const (
	anthropicWindow5h = "5h"
	anthropicWindow7d = "7d"
)

// anthropicPlanAdapter is this file's entry in planAdapters.
//
// Unavailable is one question, and the seam answers it: is there a store of
// record for this credential on this machine? The endpoint is reachable from
// anywhere. Until ADR 0019 the answer was "only on macOS", and off darwin
// this adapter reported `keychain item unreadable` every pass — a fake
// outage, and on 2026-08-24 a fake outage is what got the shop's only
// automated brake switched off on a wrong diagnosis (credential.go's
// GateRefusal). Now darwin reads the keychain, everything else reads the
// runtime's own credentials file, and only a machine where that file has
// never been written answers no — as *NoSource, which is the guard OFF and
// not a fleet parked on a condition no retry can change (ADR 0019 D3).
var anthropicPlanAdapter = planAdapter{
	Name: "anthropic",
	Unavailable: func() error {
		// A loopback RHQ_PLAN_USAGE_URL is asked WITHOUT the credential and
		// reads no store at all (credpin.go rule 4), so there is no
		// credential question to answer and this adapter serves that seam
		// anywhere. Not a test hatch: it is the same rule stated where the
		// question is asked, and it is what keeps the seam runnable on a
		// machine with no credential instead of a code path only a logged-in
		// box can reach.
		if raw := os.Getenv("RHQ_PLAN_USAGE_URL"); raw != "" {
			if _, err := loopbackOverride("RHQ_PLAN_USAGE_URL", raw); err == nil {
				return nil
			}
		}
		return MeterUnavailable("claude")
	},
	New: func() PlanReader { return NewAnthropicPlanReader() },
}

// AnthropicPlanReader reads this provider's two windows. Token and HTTP are
// fields so tests can inject a fake credential and a fake endpoint; nothing
// else needs to.
//
// This is the reader rangerhq-25p asked for: Dial E's step-down may take
// max(cost-window %, plan-window %) — 25p decides, this only exposes Read.
type AnthropicPlanReader struct {
	URL   string
	Token func() (string, error)
	HTTP  *http.Client
	// URLErr is a refused RHQ_PLAN_USAGE_URL (credpin.go), carried here
	// rather than returned by the constructor: every caller of that wants a
	// reader, and the honest place to say "no reading, and why" is the one
	// that would have made the request. Non-nil = Read refuses and asks
	// nobody — no keychain read, no request.
	URLErr error
	// Now is the clock a Retry-After date is measured against (nil =
	// time.Now). Only the HTTP-date form of that header needs it.
	Now func() time.Time
	// Shared reports whether a reading this reader produces may become the
	// instance's shared fact — `$StateDir/plan-usage.json`, which every
	// posse process on the machine reads for the TTL (rangerhq-tdy8).
	// credpin.go rule 5: only the compiled-in endpoint's answers may. One
	// `posse cost --plan` under a loopback override would otherwise publish
	// a caller's numbers to the whole shop, and that needs no credential at
	// all (ranger-base-dr6u, reproduced live on ranger-base-7nlw).
	//
	// FALSE is the zero value on purpose: a reader nobody vouched for is
	// nobody's fact, so a struct literal that forgets this field fails
	// safe. The constructor sets it true while the URL is still PlanUsageURL,
	// and a test that wants the real reader's caching sets it too.
	//
	// It gates the STORE, not the load: reading the fleet's snapshot under
	// an override is not poisoning, and `posse cost --plan` in a test home
	// is meant to answer off a seeded one.
	Shared bool
}

// MayShare is the PlanReader seam's half of credpin.go rule 5; the Shared
// field above is where the answer is decided.
func (r *AnthropicPlanReader) MayShare() bool { return r.Shared }

func (r *AnthropicPlanReader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func NewAnthropicPlanReader() *AnthropicPlanReader {
	r := &AnthropicPlanReader{
		URL:   PlanUsageURL,
		Token: MeterToken("claude"),
		HTTP:  pinnedClient(10*time.Second, "usage endpoint", PlanUsageHost),
	}
	if raw := os.Getenv("RHQ_PLAN_USAGE_URL"); raw != "" {
		if u, err := loopbackOverride("RHQ_PLAN_USAGE_URL", raw); err != nil {
			r.URLErr = err
		} else {
			r.URL = u
		}
	}
	// Derived from where the URL ended up, never from whether an env var
	// was set: a refused override leaves the compiled-in endpoint, and that
	// one still shares.
	r.Shared = credentialedURL(r.URL, PlanUsageURL)
	return r
}

// Read fetches the current utilization of both windows.
//
// Every failure returns a nil PlanUsage rather than a zeroed one: with
// windows as a slice, "no reading" and "a reading of 0%" are finally
// different values, and the guard must never be handed the second when it
// got the first.
func (r *AnthropicPlanReader) Read() (PlanUsage, error) {
	if r.URLErr != nil {
		return nil, r.URLErr
	}
	if r.URL == "" {
		return nil, Die("plan reader not configured")
	}
	// Before the credential, not after: a host posse will not ask is answered
	// by asking nobody and reading nothing (credpin.go rule 2).
	if err := pinnedEndpoint("usage endpoint", r.URL, PlanUsageHost); err != nil {
		return nil, err
	}
	// credpin.go rule 4. A loopback override is a seam, not an account: it
	// is asked WITHOUT the credential, and no credential store is read for
	// it at all — so an env var and a socket can no longer make posse pull
	// the token out of its store and put it in front of a listener the
	// caller chose (ranger-base-dr6u). The seam keeps working; what it
	// stops getting is the bearer.
	credentialed := credentialedURL(r.URL, PlanUsageURL)
	var tok string
	if credentialed {
		if r.Token == nil {
			return nil, Die("plan reader not configured")
		}
		var err error
		if tok, err = r.Token(); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest("GET", r.URL, nil)
	if err != nil {
		return nil, Die("bad usage url")
	}
	if credentialed {
		// Belt, on the request that is about to be made: the header goes on
		// only after this URL has been checked one more time.
		if err := pinnedRequest("usage endpoint", req, PlanUsageHost); err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
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
			return nil, pin
		}
		// A transport error can quote the URL but never a header.
		return nil, Die("usage endpoint unreachable")
	}
	defer resp.Body.Close()
	// A redirect a client without our CheckRedirect followed: the answer
	// came from a host we do not credential, so it is not an answer.
	if err := pinnedResponse("usage endpoint", resp, PlanUsageHost); err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, &RateLimit{Status: resp.Status, RetryAfter: retryAfter(resp.Header.Get("Retry-After"), r.now())}
	}
	// The auth statuses are their own class (planusage.go's AuthFailure,
	// bead rangerhq-ytyj): availability failures are weather and heal by
	// themselves, and these two do not. The status line is all this file
	// contributes — the sentences are the harness's, because "what does an
	// operator do about it" is not provider knowledge.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &AuthFailure{Status: resp.Status, Code: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, Die("usage endpoint returned %s", resp.Status)
	}
	var body struct {
		FiveHour *anthropicWindowBody `json:"five_hour"`
		SevenDay *anthropicWindowBody `json:"seven_day"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, Die("usage response is not the expected JSON")
	}
	// A 200 is not a reading (ranger-base-65s). Decoded into values, a body
	// of the wrong shape — `{}`, an error envelope from a middlebox, a
	// renamed key after an endpoint change — left both windows at their
	// zero value and came back as a SUCCESSFUL reading of 0% utilization.
	// The guard opened, and plan_guard_blind_max never armed because the
	// reads kept "succeeding": fail-open in the one direction the interlock
	// must never fail. Absent is now an error, so the blind clock runs.
	five, ok := body.FiveHour.pct()
	if !ok {
		return nil, Die("usage response is not the expected JSON: no %s utilization", anthropicWindow5h)
	}
	seven, ok := body.SevenDay.pct()
	if !ok {
		return nil, Die("usage response is not the expected JSON: no %s utilization", anthropicWindow7d)
	}
	// Tightest first: the guard trips on the first window over its
	// threshold, and the 5h window is the one the operator feels.
	return PlanUsage{
		{Name: anthropicWindow5h, Pct: five},
		{Name: anthropicWindow7d, Pct: seven},
	}, nil
}

// anthropicWindowBody is one window as the endpoint reports it. Both the
// window and its utilization are pointers so that MISSING and 0.0 are
// different values all the way down: a key that is present and zero is a
// legitimate reading (a fresh window really is 0% used), a key that is not
// there at all is no reading.
type anthropicWindowBody struct {
	Utilization *float64 `json:"utilization"`
}

// pct is the window's utilization and whether the response actually
// carried one. Nil-receiver safe: the absent window and the present-but-
// empty one are the same answer.
func (w *anthropicWindowBody) pct() (float64, bool) {
	if w == nil || w.Utilization == nil {
		return 0, false
	}
	return *w.Utilization, true
}
