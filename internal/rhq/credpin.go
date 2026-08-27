package rhq

// Where a credentialed request may go (ranger-base-17i).
//
// Two readers put the account's Claude Code OAuth token on the wire: the
// plan guard (planusage.go) and the tier preflight (modelavail.go). Both
// took their endpoint from an environment variable with no validation of
// any kind, so `RHQ_PLAN_USAGE_URL=http://listener/…` — or the preflight's
// `RHQ_MODEL_LIST_URL`, which is on by default — was enough to make posse
// itself pull the credential out of the keychain and hand it to a host of
// the caller's choosing. Confirmed live against a synthetic token,
// 2026-08-25. Below the container tier a session could already read the
// item directly; what the override added was that the read happened INSIDE
// the harness, so nothing landed in refusals.log and the Bash(security:*)
// tripwire never fired — and that it would have outlived ADR 0019's
// keychain re-lock, after which posse is the process that may read the item
// and arbitrary sessions are not.
//
// The rule, one place, both callers:
//
//  1. An endpoint override is honoured only when its host is LOOPBACK.
//     Anything else is REFUSED and said out loud — never silently swapped
//     back for the real endpoint, because an override the operator set and
//     posse ignored is the other way to be wrong.
//  2. Belt: a reader may be POINTED at the compiled-in host or at this
//     machine, and nowhere else — whatever set the URL: an env var, a
//     struct field, or a caller who forgets rule 1. That is reachHost
//     below, and it is deliberately NOT the credential question.
//  3. Belt: the host that ANSWERED is checked too. Go already strips
//     Authorization across a redirect to another domain; this stops the
//     other half, which is that the override was a cache-poisoning
//     primitive as well — a 200 from somewhere else must never become the
//     fact in state/model-catalog.json or state/plan-usage.json.
//
// The override this file does not mention is RHQ_MODEL_LIST_URL: it is
// gone. Nothing read it but the vulnerability — the tests inject
// App.ModelLister, the seam built for exactly this.
//
// ─── what ranger-base-dr6u changed ───────────────────────────────────────
//
// 17i's premise for rule 1 was "a test server always is loopback; an exfil
// destination never is". The second half is false when the adversary is a
// LOCAL process, and on this machine it is: SeatbeltProfile is `(allow
// default)` + `(deny file-write*)` with no network restriction, and
// `Bash(posse:*)` is granted crew-wide — so an env var and a socket are all
// it takes to be a caller-chosen destination that is also loopback
// (verified live against a synthetic token, ranger-base-7nlw). Reaching a
// local listener is a seam. Being CREDENTIALED and being BELIEVED are not,
// so those two are their own rules now:
//
//  4. The account's credential goes only to the compiled-in endpoint,
//     byte for byte — credentialedURL below, never a host test. A loopback
//     override is asked WITHOUT an Authorization header, and the keychain
//     is not read for it at all: the seam keeps working, uncredentialed.
//  5. An override's answer is not the fleet's fact. `$StateDir/
//     plan-usage.json` is read by every posse process on the instance for
//     the TTL (rangerhq-tdy8), so one `posse cost --plan` under an override
//     would otherwise PUBLISH a caller's numbers to the whole shop — 0%
//     disarms the plan guard, 99% parks the fleet, and neither needs a
//     credential. Enforced at the store, in plancache.go, off
//     PlanReader.MayShare.
//
// Rules 4 and 5 are the PLAN reader's. ModelLister keeps the loopback
// carve-out of rules 2 and 3 because it has no override to reach it with:
// RHQ_MODEL_LIST_URL is deleted and its URL is the compiled-in constant in
// every path that is not a test injecting a struct field. If a model-list
// override is ever re-introduced, it needs rules 4 and 5 first.

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// pinLoopbackRule, pinReachRule and pinCredentialRule are the refusals in
// the operator's terms. They are what an operator reads when a launch says
// no, so they name the fix, not the failure.
const pinLoopbackRule = "an endpoint override is honoured only when its host is loopback (127.0.0.1, [::1], localhost)"

// pinReachRule is rule 2: where a reader may be pointed at all.
func pinReachRule(host string) string {
	return "posse asks " + host + " or this machine and nowhere else"
}

// pinCredentialRule is rule 4: where the credential may go. Loopback is
// absent by design — see the dr6u section above.
func pinCredentialRule(host string) string {
	return "posse puts the account's credential in front of " + host + " and nowhere else"
}

// PinRefusal is posse declining to point a credentialed request at a host
// it does not credential. A distinct type because it is not a transport
// failure and must not be reported as one: nothing was unreachable, and
// nothing was asked. It never quotes the credential, a header, or the raw
// override — the host is the whole diagnosis.
type PinRefusal struct {
	What     string // what named the host: an env var, or the endpoint
	Host     string // the host named ("" = the url had none)
	Why      string // the rule it broke
	Redirect bool   // the host answered a redirect rather than being asked
}

func (e *PinRefusal) Error() string {
	host := e.Host
	if host == "" {
		host = "a url with no host"
	}
	verb := "names"
	if e.Redirect {
		verb = "redirected to"
	}
	return fmt.Sprintf("%s %s %s — refused: %s", e.What, verb, host, e.Why)
}

// reachHost: may this process point a reader at u, or believe an answer
// from it? The compiled-in host of the endpoint, or this machine.
//
// It used to be called credentialedHost and used for both questions. It is
// not the credential question (ranger-base-dr6u): a loopback listener is a
// host any local process can be, so "posse may ask it" and "posse may hand
// it the account's token" are different permissions. credentialedURL is the
// second one.
func reachHost(u *url.URL, want string) bool {
	if u == nil {
		return false
	}
	h := u.Hostname()
	if h == "" {
		return false
	}
	if strings.EqualFold(h, want) {
		return true
	}
	return loopbackHost(u)
}

// loopbackHost: does u name this machine? The test-seam question, and the
// one loopbackOverride is built on.
func loopbackHost(u *url.URL) bool {
	if u == nil {
		return false
	}
	h := u.Hostname()
	if h == "" {
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// credentialedURL is rule 4, and it is deliberately not a host test: the
// account's credential goes to the endpoint posse COMPILED IN, byte for
// byte, and to no URL that merely resolves somewhere that looks like it.
// Anything an env var or a caller substituted — including a variant
// spelling of the real endpoint — is an override, and an override is
// answered by a host somebody else chose.
func credentialedURL(raw, compiled string) bool {
	return raw == compiled
}

// loopbackOverride reads an endpoint override. It returns the URL only when
// its host is loopback; otherwise a *PinRefusal naming the host, which the
// caller must surface rather than quietly fall back.
func loopbackOverride(name, raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", &PinRefusal{What: name, Why: pinLoopbackRule}
	}
	// The compiled-in host is deliberately NOT accepted here: an override
	// that names the real endpoint buys nothing and would make the rule two
	// rules. Loopback, or refused.
	if !loopbackHost(u) {
		return "", &PinRefusal{What: name, Host: u.Host, Why: pinLoopbackRule}
	}
	return raw, nil
}

// pinnedEndpoint is belt (2) made one step early: before the keychain is
// read at all, on a URL that has not become a request yet. A host this
// process will not ask costs the credential nothing — not even a read into
// memory.
func pinnedEndpoint(what, raw, want string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return &PinRefusal{What: what, Why: pinReachRule(want)}
	}
	if reachHost(u, want) {
		return nil
	}
	return &PinRefusal{What: what, Host: u.Host, Why: pinReachRule(want)}
}

// pinnedRequest is the check made where the Authorization header is about
// to be set, on the request that is about to be made. Callers that set that
// header for a loopback URL must not use it — it is rule 2's host test, and
// the plan reader answers rule 4 with credentialedURL before it gets here.
func pinnedRequest(what string, req *http.Request, want string) error {
	if reachHost(req.URL, want) {
		return nil
	}
	return &PinRefusal{What: what, Host: req.URL.Host, Why: pinReachRule(want)}
}

// pinnedResponse is belt (3): the check made on the host that answered, so
// a redirect off the pinned host is refused rather than decoded and cached.
func pinnedResponse(what string, resp *http.Response, want string) error {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return nil
	}
	if reachHost(resp.Request.URL, want) {
		return nil
	}
	return &PinRefusal{What: what, Host: resp.Request.URL.Host, Why: pinReachRule(want), Redirect: true}
}

// pinnedClient is the client the two readers build for themselves: one that
// will not follow a redirect off the pinned host at all. A client injected
// by a test is not required to have this — pinnedResponse catches the same
// redirect one step later, which is why both exist.
func pinnedClient(timeout time.Duration, what, want string) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if !reachHost(req.URL, want) {
				return &PinRefusal{What: what, Host: req.URL.Host, Why: pinReachRule(want), Redirect: true}
			}
			return nil
		},
	}
}
