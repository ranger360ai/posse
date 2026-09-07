package posse

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
//  3. Belt: the host that ANSWERED is checked too — against the host this
//     reader ASKED, not against rule 2's set. Go already strips
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
// LOCAL process, and the software's own posture permits one: the rendered
// seatbelt profile restricts file writes, not network egress (seatbelt.go),
// and wherever the harness's own command is allowlisted for a session, an
// env var and a socket are all it takes to be a caller-chosen destination
// that is also loopback (verified live against a synthetic token,
// ranger-base-7nlw). Reaching a local listener is a seam. Being
// CREDENTIALED and being BELIEVED are not, so those two are their own
// rules now:
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
// carve-out of rule 2 because it has no override to reach it with:
// RHQ_MODEL_LIST_URL is deleted and its URL is the compiled-in constant in
// every path that is not a test injecting a struct field. If a model-list
// override is ever re-introduced, it needs rules 4 and 5 first.
//
// ─── what ranger-base-07ep changed ───────────────────────────────────────
//
// Rules 2 and 3 both ran on reachHost, which answers "the compiled-in host
// OR this machine". That is the right SET for rule 2: where a reader may be
// POINTED is a set, because a loopback test target is a legitimate place to
// be pointed. It is the wrong set for rule 3, because which host may ANSWER
// is not a set — it is one host, the one that was asked. Conflated, a 302
// from the compiled-in endpoint to `http://127.0.0.1:PORT/…` was followed
// by the pinned client and its 200 accepted by pinnedResponse, both because
// reachHost said loopback was fine. Anyone holding the network path or the
// upstream itself (a trusted CA, DNS + TLS interception, a compromised
// api.anthropic.com, a proxy the operator configured — NOT an env var and a
// socket, which dr6u closed) could therefore hand the whole instance its
// model-catalog.json or plan-usage.json. The model sink is the weaker of
// the two: ModelCache.store has no MayShare gate at all, so belt 3 is the
// only thing standing in front of it.
//
// So rule 3 is answeredHost, not reachHost: A REDIRECT MUST STAY ON ITS OWN
// HOST. A compiled-in request may be answered only by the compiled-in host,
// and a loopback override only by its own loopback target. The want is the
// EFFECTIVE one, never the compiled-in constant — askedHost(r.URL) at the
// response check, and via[0] inside the client. Path-only redirects are
// still followed; a cross-host one is a *PinRefusal{Redirect: true}, which
// is already the not-an-outage shape both readers surface.
//
// The unit is the HOSTNAME, not host:port. A same-host scheme or port
// change is the endpoint's own business, and comparing ports would turn a
// plain `https://host` → `https://host:443` into a refusal in front of a
// launch. The case that leaves — one loopback listener redirecting to
// another — needs the reader already pointed at a listener the caller
// chose, which is rules 4 and 5's ground rather than this one's.
//
// ─── what ranger-base-ji1d3 changed ──────────────────────────────────────
//
// The hostname unit had a gap of its own: same host, https asked, http
// answered — Go's own redirect-header rule (shouldCopyHeaderOnRedirect,
// isDomainOrSubdomain) only ever compares hostnames too, so it copied the
// Authorization header straight across the downgrade, and rule 3 as it
// stood said the host matched and waved it through (ranger-base-246xo).
// Whoever holds the TLS session for the compiled-in endpoint — or the
// upstream itself, 07ep's premise — could turn one interception into an
// ongoing plaintext leak of the OAuth bearer at every catalog and plan
// read, reachable by passive network observers the interception itself is
// not.
//
// So the unit gains one exception, not a second unit: a same-host answer
// still passes rule 3, UNLESS the scheme also stepped from https down to
// http, in which case it is refused regardless of the host matching.
// http asked, https answered stays allowed — that is the endpoint
// upgrading itself — and so does a same-scheme port change; only the
// downgrade direction is new. The refusal names the downgrade, not the
// host: the host DID match, and a refusal phrased like a host mismatch
// would send the operator hunting for the wrong break.

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// pinLoopbackRule, pinReachRule, pinCredentialRule and pinAnswerRule are
// the refusals in the operator's terms — one per rule that can refuse. They
// are what an operator reads when a launch says no, so they name the fix,
// not the failure.
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

// pinAnswerRule is rule 3: which host may ANSWER. Deliberately not
// pinReachRule — being a host posse MAY ask and being the host posse DID
// ask are different questions, and one sentence for both is how belt 3
// came to accept a redirect nobody asked for (ranger-base-07ep).
func pinAnswerRule(host string) string {
	if host == "" {
		return "an answer counts only from the host posse asked"
	}
	return "an answer counts only from " + host + ", the host posse asked"
}

// pinDowngradeRule is rule 3's scheme exception: a same-host answer that
// stepped from https down to http (ranger-base-246xo, ranger-base-ji1d3).
// It says downgrade, not host — the host matched, so a sentence shaped
// like pinAnswerRule's would send the operator hunting the wrong break.
func pinDowngradeRule(host string) string {
	if host == "" {
		return "an https ask is not answered by http, even on the same host"
	}
	return "an https ask of " + host + " is not answered by http, even on the same host"
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

// reachHost: may this process point a reader at u? The compiled-in host of
// the endpoint, or this machine. Rule 2 and nothing else.
//
// It used to be called credentialedHost and used for both questions. It is
// not the credential question (ranger-base-dr6u): a loopback listener is a
// host any local process can be, so "posse may ask it" and "posse may hand
// it the account's token" are different permissions. credentialedURL is the
// second one.
//
// It is not the ANSWER question either, and it used to say here that it was
// (ranger-base-07ep). "Where a reader may be pointed" is a set; "which host
// may answer what this reader asked" is one host. answeredHost is that one.
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

// askedHost is rule 3's effective want: the host of the URL this reader is
// CONFIGURED with. That is the compiled-in host in every path which is not
// a loopback override or a test injecting the struct field — and it is
// deliberately read off the reader rather than taken as the compiled-in
// constant, because the constant is a want that can be wrong about the
// request that was actually made (ranger-base-07ep).
func askedHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// askedScheme is the downgrade exception's other half of rule 3's want:
// the scheme of the URL this reader is CONFIGURED with, read the same way
// askedHost is — off the reader, not the compiled-in constant — so it
// agrees with askedHost about which request was actually asked for.
func askedScheme(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Scheme
}

// answeredHost is rule 3's test. An empty want is FALSE, not true: a host
// nobody could name is not a host everybody satisfies, and the readers
// have already passed this URL through pinnedEndpoint by the time it is
// asked, so an empty one here is a bug rather than a default.
func answeredHost(u *url.URL, asked string) bool {
	if u == nil || asked == "" {
		return false
	}
	h := u.Hostname()
	return h != "" && strings.EqualFold(h, asked)
}

// downgradesScheme is rule 3's scheme exception: true only for https asked,
// http answered. Not a general scheme-mismatch test — http asked answered
// by https is the endpoint upgrading itself and stays allowed, and an
// empty scheme on either side (a parse failure, or via[0] before any
// request exists) is never treated as a downgrade here; answeredHost's
// empty-want-is-false already fails the answer closed on that path.
func downgradesScheme(asked, answered string) bool {
	return asked == "https" && answered == "http"
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
// a redirect off the host the reader ASKED is refused rather than decoded
// and cached. asked is the effective host — askedHost(r.URL) — and
// askedScheme the effective scheme — askedScheme(r.URL) — neither the
// compiled-in constant, which would let a redirect from the compiled-in
// endpoint to this machine (or a downgrade at it) pass as if it had been
// asked for.
func pinnedResponse(what string, resp *http.Response, asked, askedScheme string) error {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return nil
	}
	if !answeredHost(resp.Request.URL, asked) {
		return &PinRefusal{What: what, Host: resp.Request.URL.Host, Why: pinAnswerRule(asked), Redirect: true}
	}
	if downgradesScheme(askedScheme, resp.Request.URL.Scheme) {
		return &PinRefusal{What: what, Host: resp.Request.URL.Host, Why: pinDowngradeRule(asked), Redirect: true}
	}
	return nil
}

// pinnedClient is the client the two readers build for themselves: one that
// will not follow a redirect off the host it asked at all. A client injected
// by a test is not required to have this — pinnedResponse catches the same
// redirect one step later, which is why both exist.
//
// The host is read off via[0] — the request that was actually made — rather
// than passed in, and that is the point: a reader's URL is set AFTER its
// client is built (the plan reader's loopback override is, and so is every
// test that injects the struct field), so a want compiled in here would be
// the stale one. via[0] cannot disagree with the request it belongs to.
func pinnedClient(timeout time.Duration, what string) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			var asked, askedScheme string
			if len(via) > 0 {
				asked = via[0].URL.Hostname()
				askedScheme = via[0].URL.Scheme
			}
			if !answeredHost(req.URL, asked) {
				return &PinRefusal{What: what, Host: req.URL.Host, Why: pinAnswerRule(asked), Redirect: true}
			}
			if downgradesScheme(askedScheme, req.URL.Scheme) {
				return &PinRefusal{What: what, Host: req.URL.Host, Why: pinDowngradeRule(asked), Redirect: true}
			}
			return nil
		},
	}
}
