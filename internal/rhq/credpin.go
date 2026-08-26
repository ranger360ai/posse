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
//  1. An endpoint override is honoured only when its host is LOOPBACK. A
//     test server is; an exfil destination is not. Anything else is
//     REFUSED and said out loud — never silently swapped back for the real
//     endpoint, because an override the operator set and posse ignored is
//     the other way to be wrong.
//  2. Belt: the Authorization header goes on a request only when that
//     request's host is the compiled-in one or loopback — whatever set the
//     URL: an env var, a struct field, or a caller who forgets rule 1.
//  3. Belt: the host that ANSWERED is checked too. Go already strips
//     Authorization across a redirect to another domain; this stops the
//     other half, which is that the override was a cache-poisoning
//     primitive as well — a 200 from somewhere else must never become the
//     fact in state/model-catalog.json or state/plan-usage.json.
//
// The override this file does not mention is RHQ_MODEL_LIST_URL: it is
// gone. Nothing read it but the vulnerability — the tests inject
// App.ModelLister, the seam built for exactly this.

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// pinLoopbackRule and pinCredentialRule are the two refusals in the
// operator's terms. They are what an operator reads when a launch says no,
// so they name the fix, not the failure.
const pinLoopbackRule = "an endpoint override is honoured only when its host is loopback (127.0.0.1, [::1], localhost)"

func pinCredentialRule(host string) string {
	return "posse puts the account's credential in front of " + host + " or loopback and nowhere else"
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

// credentialedHost: may this process put the account's credential in front
// of u? The compiled-in host of the endpoint, or this machine.
func credentialedHost(u *url.URL, want string) bool {
	if u == nil {
		return false
	}
	h := u.Hostname()
	if h == "" {
		return false
	}
	if strings.EqualFold(h, want) || strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
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
	if !credentialedHost(u, "") {
		return "", &PinRefusal{What: name, Host: u.Host, Why: pinLoopbackRule}
	}
	return raw, nil
}

// pinnedEndpoint is belt (2) made one step early: before the keychain is
// read at all, on a URL that has not become a request yet. A host this
// process does not credential costs the credential nothing — not even a
// read into memory.
func pinnedEndpoint(what, raw, want string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return &PinRefusal{What: what, Why: pinCredentialRule(want)}
	}
	if credentialedHost(u, want) {
		return nil
	}
	return &PinRefusal{What: what, Host: u.Host, Why: pinCredentialRule(want)}
}

// pinnedRequest is the same check where the Authorization header is about
// to be set, on the request that is about to be made.
func pinnedRequest(what string, req *http.Request, want string) error {
	if credentialedHost(req.URL, want) {
		return nil
	}
	return &PinRefusal{What: what, Host: req.URL.Host, Why: pinCredentialRule(want)}
}

// pinnedResponse is belt (3): the check made on the host that answered, so
// a redirect off the pinned host is refused rather than decoded and cached.
func pinnedResponse(what string, resp *http.Response, want string) error {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return nil
	}
	if credentialedHost(resp.Request.URL, want) {
		return nil
	}
	return &PinRefusal{What: what, Host: resp.Request.URL.Host, Why: pinCredentialRule(want), Redirect: true}
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
			if !credentialedHost(req.URL, want) {
				return &PinRefusal{What: what, Host: req.URL.Host, Why: pinCredentialRule(want), Redirect: true}
			}
			return nil
		},
	}
}
