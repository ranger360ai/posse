package posse

// The endpoint pin (ranger-base-17i): posse hands the account's OAuth token
// to the compiled-in host or to this machine, and to nothing else.
//
// Hermetic like the two suites it sits between — no keychain, no network.
// The transports below answer for hosts that do not exist, which is exactly
// how an exfil destination would have been reached before the pin: the
// point of every assertion here is that the request is never made.

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// refusingTransport fails the test if anything reaches the wire.
func refusingTransport(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("a request was made to %s — the pin must refuse before asking", r.URL.Host)
		return nil, errors.New("must not be reached")
	})}
}

// refusingToken fails the test if the keychain is read at all: a refused
// host must cost the credential nothing, not even a read into memory.
func refusingToken(t *testing.T) func() (string, CredMeta, error) {
	t.Helper()
	return func() (string, CredMeta, error) {
		t.Error("the keychain was read for a host the pin refuses")
		return fakeToken, CredMeta{}, nil
	}
}

// ─── the override that is gone ───────────────────────────────────────────────

// RHQ_MODEL_LIST_URL was the default-on half of the vulnerability: the
// preflight runs unless switched off, so nothing had to be armed. It is
// deleted, not pinned — the tests that need a catalog inject App.ModelLister.
func TestModelListURLOverrideIsGone(t *testing.T) {
	t.Setenv("RHQ_MODEL_LIST_URL", "http://listener.example/v1/models")
	l := NewModelLister()
	if l.URL != ModelListURL {
		t.Errorf("RHQ_MODEL_LIST_URL still moves the endpoint: %s", l.URL)
	}
}

// And no other env var moves it either — the whole environment is not a
// place this URL comes from.
func TestModelListerURLIsCompiledIn(t *testing.T) {
	for _, k := range []string{"RHQ_MODEL_LIST_URL", "RHQ_PLAN_USAGE_URL", "RHQ_MODEL_URL"} {
		t.Setenv(k, "http://listener.example/v1/models")
	}
	if got := NewModelLister().URL; got != "https://api.anthropic.com/v1/models" {
		t.Errorf("model list endpoint = %s, want the compiled-in one", got)
	}
}

// ─── the override that is pinned ─────────────────────────────────────────────

// A non-loopback RHQ_PLAN_USAGE_URL: no request, no keychain read, and an
// error that names the variable and the host it named. Never a silent swap
// back to the real endpoint — an override posse ignored is the other way to
// be wrong.
func TestPlanUsageURLOverrideRefusesNonLoopback(t *testing.T) {
	for _, raw := range []string{
		"http://listener.example/v1/usage",
		"https://api.anthropic.com.listener.example/usage", // the near miss
		"http://169.254.169.254/usage",                     // the metadata service
		"not a url at all",
		"file:///etc/passwd",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("RHQ_PLAN_USAGE_URL", raw)
			r := NewAnthropicPlanReader()
			r.Token, r.HTTP = refusingToken(t), refusingTransport(t)

			if r.URL != PlanUsageURL {
				t.Errorf("a refused override still moved the URL: %s", r.URL)
			}
			_, err := r.Read()
			var pin *PinRefusal
			if !errors.As(err, &pin) {
				t.Fatalf("Read error = %v, want a *PinRefusal", err)
			}
			if !strings.Contains(err.Error(), "RHQ_PLAN_USAGE_URL") || !strings.Contains(err.Error(), "loopback") {
				t.Errorf("the refusal must name the variable and the rule: %q", err)
			}
			if strings.Contains(err.Error(), fakeToken) {
				t.Errorf("the refusal quotes the credential: %q", err)
			}
		})
	}
}

// Loopback is honoured, in every spelling a test server uses.
func TestPlanUsageURLOverrideHonoursLoopback(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:8080/usage",
		"http://localhost:8080/usage",
		"http://[::1]:8080/usage",
	} {
		t.Setenv("RHQ_PLAN_USAGE_URL", raw)
		if got := NewAnthropicPlanReader(); got.URL != raw || got.URLErr != nil {
			t.Errorf("loopback override %s not honoured: url=%s err=%v", raw, got.URL, got.URLErr)
		}
	}
}

// ─── the belt: whatever set the URL ──────────────────────────────────────────

// A field set in code, a test, or a caller who forgot the rule: the
// Authorization header is never put in front of a host we do not credential.
func TestReadersRefuseAnUncredentialedHostWhateverSetTheURL(t *testing.T) {
	t.Parallel()
	t.Run("plan", func(t *testing.T) {
		r := &AnthropicPlanReader{URL: "https://listener.example/usage", Token: refusingToken(t), HTTP: refusingTransport(t)}
		_, err := r.Read()
		var pin *PinRefusal
		if !errors.As(err, &pin) {
			t.Fatalf("Read error = %v, want a *PinRefusal", err)
		}
		if !strings.Contains(err.Error(), PlanUsageHost) {
			t.Errorf("the refusal must name what is allowed: %q", err)
		}
	})
	t.Run("models", func(t *testing.T) {
		l := &ModelLister{URL: "https://listener.example/v1/models", Token: refusingToken(t), HTTP: refusingTransport(t)}
		_, err := l.List()
		var pin *PinRefusal
		if !errors.As(err, &pin) {
			t.Fatalf("List error = %v, want a *PinRefusal", err)
		}
		if !strings.Contains(err.Error(), ModelListHost) {
			t.Errorf("the refusal must name what is allowed: %q", err)
		}
	})
}

// ─── the belt: the host that answered ────────────────────────────────────────

// redirectTransport answers `from` with a 302 to `to`, and answers `to` with
// a 200 carrying a poisoned catalog. It records whether the second host was
// asked at all.
type redirectTransport struct {
	to     string
	body   string
	asked  bool
	client *http.Client
}

func newRedirectTransport(to, body string) *redirectTransport {
	rt := &redirectTransport{to: to, body: body}
	rt.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == rt.to {
			rt.asked = true
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader(rt.body)), Request: r,
			}, nil
		}
		h := make(http.Header)
		h.Set("Location", "https://"+rt.to+r.URL.Path)
		return &http.Response{
			StatusCode: http.StatusFound, Status: "302 Found", Header: h,
			Body: io.NopCloser(strings.NewReader("")), Request: r,
		}, nil
	})}
	return rt
}

// A 200 that came from a redirect off the pinned host is not an answer, and
// above all it never becomes the cached fact: the override was a
// cache-poisoning primitive as well as an exfil one.
func TestARedirectedCatalogNeverReachesTheCache(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	rt := newRedirectTransport("listener.example", `{"data":[{"id":"probe-model"}],"has_more":false}`)
	a.ModelLister = &ModelLister{
		URL:   "http://127.0.0.1:9/v1/models",
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
		HTTP:  rt.client,
	}

	if ids, ok := a.ModelCache().Models(time.Hour); ok {
		t.Fatalf("a redirected catalog was believed: %v", ids)
	}
	b, err := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.json"))
	if err == nil && strings.Contains(string(b), "probe-model") {
		t.Errorf("the redirected answer was cached: %s", b)
	}
	log, _ := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.log"))
	if !strings.Contains(string(log), "listener.example") || !strings.Contains(string(log), "refused") {
		t.Errorf("the refusal must be diagnosable in the probe log:\n%s", log)
	}
	if strings.Contains(string(log), fakeToken) {
		t.Errorf("the probe log contains the credential: %s", log)
	}
}

// The plan reader's half of the same thing.
func TestARedirectedUsageResponseIsRefused(t *testing.T) {
	t.Parallel()
	rt := newRedirectTransport("listener.example", `{"five_hour":{"utilization":1}}`)
	r := &AnthropicPlanReader{URL: "http://127.0.0.1:9/usage", Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil }, HTTP: rt.client}
	_, err := r.Read()
	var pin *PinRefusal
	if !errors.As(err, &pin) {
		t.Fatalf("Read error = %v, want a *PinRefusal", err)
	}
	if !pin.Redirect || !strings.Contains(err.Error(), "listener.example") {
		t.Errorf("the refusal must say a redirect took it elsewhere: %q", err)
	}
}

// The client the readers build for themselves does not even follow the
// redirect — pinnedResponse above is the second line, for an injected one.
func TestThePinnedClientWillNotFollowARedirectOffTheHost(t *testing.T) {
	t.Parallel()
	rt := newRedirectTransport("listener.example", "{}")
	cl := pinnedClient(time.Second, "model list endpoint")
	cl.Transport = rt.client.Transport

	l := &ModelLister{URL: "http://127.0.0.1:9/v1/models", Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil }, HTTP: cl}
	_, err := l.List()
	var pin *PinRefusal
	if !errors.As(err, &pin) {
		t.Fatalf("List error = %v, want a *PinRefusal", err)
	}
	if rt.asked {
		t.Error("the redirect target was asked — a pinned client must not follow it")
	}
	if strings.Contains(err.Error(), "unreachable") {
		t.Errorf("a refusal must not be reported as an outage: %q", err)
	}
}

// ─── ranger-base-dr6u: the loopback carve-out, closed ────────────────────────
//
// 17i pinned the override to loopback on the premise that "a test server
// always is loopback; an exfil destination never is". The second half is
// false when the adversary is a LOCAL process — a 127.0.0.1 listener is
// both loopback and a destination the caller chose, and on this machine any
// seatbelt-caged persona can bind one and set an env var for a `posse`
// command. So loopback still buys the SEAM and nothing else: not the
// account's credential (rule 4), and not a place in the snapshot the rest
// of the fleet reads (rule 5).
//
// The listeners below are real: httptest binds 127.0.0.1, which is the only
// way an override is honoured at all, and the readers dial them with their
// own pinnedClient.

// overrideRig is one loopback listener the caller "chose", plus the state
// dir that stands in for the instance's shared one.
type overrideRig struct {
	dir   string
	hits  int
	auth  string
	cache *PlanCache
}

func newOverrideRig(t *testing.T, status int, body, retryAfter string) *overrideRig {
	t.Helper()
	rig := &overrideRig{dir: t.TempDir()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rig.hits++
		rig.auth = r.Header.Get("Authorization")
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("RHQ_PLAN_USAGE_URL", srv.URL+"/usage")
	r := NewAnthropicPlanReader()
	if r.URLErr != nil {
		t.Fatalf("the loopback seam must keep working: %v", r.URLErr)
	}
	if r.Shared {
		t.Fatal("an override's reading must not be shareable")
	}
	r.Token = func() (string, CredMeta, error) {
		t.Error("the keychain was read for a listener the caller named")
		return fakeToken, CredMeta{}, nil
	}
	rig.cache = &PlanCache{
		Path:   filepath.Join(rig.dir, "plan-usage.json"),
		Log:    filepath.Join(rig.dir, "plan-usage.log"),
		Caller: "cost",
		Reader: r,
	}
	return rig
}

func (rig *overrideRig) snapshot(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(rig.cache.Path)
	if err != nil {
		return ""
	}
	return string(b)
}

func (rig *overrideRig) seedTheFleetsReading(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(rig.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","windows":[{"name":"5h","pct":42},{"name":"7d","pct":61}]}`
	if err := os.WriteFile(rig.cache.Path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
}

// HALF A. The override is asked, and it is asked with nothing: no
// Authorization header, and no keychain read to produce one. This is
// qa's repro (ranger-base-7nlw) with the expectation it asked for.
func TestALoopbackOverrideIsAskedWithoutTheCredential(t *testing.T) {
	rig := newOverrideRig(t, http.StatusOK, `{"five_hour":{"utilization":7},"seven_day":{"utilization":8}}`, "")

	u, _, err := rig.cache.Read(time.Hour)
	if err != nil {
		t.Fatalf("the seam must keep working: %v", err)
	}
	if win(u, "5h") != 7 || win(u, "7d") != 8 {
		t.Errorf("the caller must still get its own reading, got %+v", u)
	}
	if rig.hits != 1 {
		t.Fatalf("%d requests reached the listener, want 1", rig.hits)
	}
	if rig.auth != "" {
		t.Errorf("the account's credential reached a caller-named listener: %q", rig.auth)
	}
}

// HALF A, whatever set the URL: a struct field is not a better provenance
// than an env var. Only the compiled-in url is credentialed.
func TestOnlyTheCompiledInURLIsCredentialed(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"http://127.0.0.1:9/usage",
		"http://localhost:9/usage",
		"https://api.anthropic.com/api/oauth/usage/", // the near miss: one byte
	} {
		if credentialedURL(raw, PlanUsageURL) {
			t.Errorf("%s is treated as the compiled-in endpoint", raw)
		}
	}
	if !credentialedURL(PlanUsageURL, PlanUsageURL) {
		t.Error("the compiled-in endpoint must be credentialed")
	}
}

// HALF B. The poisoning primitive needs no credential at all: an override
// answering 0% would disarm the plan-window guard for every posse process
// on the instance for the TTL. The caller gets its own reading; the fleet's
// snapshot is not written.
func TestAnOverridesAnswerIsNotTheFleetsFact(t *testing.T) {
	rig := newOverrideRig(t, http.StatusOK, `{"five_hour":{"utilization":0},"seven_day":{"utilization":0}}`, "")

	u, _, err := rig.cache.Read(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if win(u, "5h") != 0 {
		t.Fatalf("setup: the caller should have read the listener's 0%%, got %+v", u)
	}
	if got := rig.snapshot(t); got != "" {
		t.Errorf("an override published to the instance's snapshot: %s", got)
	}
	// Declining to adopt it is not declining to record it: the request left
	// the machine, and the cadence log is the evidence that it did.
	log, err := os.ReadFile(rig.cache.Log)
	if err != nil || !strings.Contains(string(log), "cost ok") {
		t.Errorf("the request must still be in the read log, got %q (%v)", log, err)
	}
}

// And it cannot overwrite a reading the fleet already has. maxAge 0 always
// asks, so the snapshot is not merely answering ahead of the listener.
func TestAnOverrideCannotOverwriteTheFleetsReading(t *testing.T) {
	rig := newOverrideRig(t, http.StatusOK, `{"five_hour":{"utilization":0},"seven_day":{"utilization":0}}`, "")
	rig.seedTheFleetsReading(t)

	if _, _, err := rig.cache.Read(0); err != nil {
		t.Fatal(err)
	}
	if rig.hits != 1 {
		t.Fatalf("setup: maxAge 0 must ask, got %d requests", rig.hits)
	}
	got := rig.snapshot(t)
	if !strings.Contains(got, `"pct":42`) || strings.Contains(got, `"pct":0`) {
		t.Errorf("the fleet's reading was overwritten by an override: %s", got)
	}
}

// The other direction of the same primitive, and the one that needs neither
// a credential nor a plausible number: a listener answering `429
// Retry-After: 3600` would write an hour-long cooldown into the file every
// posse process honours, parking the fleet. The caller still gets the 429.
func TestAnOverrides429DoesNotParkTheFleet(t *testing.T) {
	rig := newOverrideRig(t, http.StatusTooManyRequests, `{}`, "3600")
	rig.seedTheFleetsReading(t)

	_, _, err := rig.cache.Read(0)
	var rl *RateLimit
	if !errors.As(err, &rl) {
		t.Fatalf("Read error = %v, want a *RateLimit for the caller", err)
	}
	if got := rig.snapshot(t); strings.Contains(got, "retry_at") {
		t.Errorf("an override wrote a fleet-wide cooldown: %s", got)
	}
}

// ─── ranger-base-n052 (verify of dr6u): the spelling rule 4 is there for ──────
//
// Rule 4 is URL equality precisely so that a URL which merely LOOKS like the
// compiled-in endpoint is still an override. The near miss above is a
// trailing slash; this is the one an attacker would actually reach for,
// because it puts the real endpoint's own name in front of the caller's
// listener: `http://api.anthropic.com@127.0.0.1:PORT/api/oauth/usage` has
// Hostname() == 127.0.0.1 and userinfo "api.anthropic.com". Any host-shaped
// credential test that read the authority carelessly would credential it.
//
// Measured live during the verify: Go's client does put an Authorization
// header on this request — `Basic base64("api.anthropic.com:")`, 30 bytes,
// synthesised from the caller's OWN userinfo. That is the caller handing its
// own string back to itself, not the account's token, and the assertion
// below says so in those terms so a future reader who sees a header arrive
// does not read it as a leak.
func TestAUserinfoSpellingOfTheEndpointIsStillAnOverride(t *testing.T) {
	var gotAuth string
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"five_hour":{"utilization":0},"seven_day":{"utilization":0}}`))
	}))
	defer srv.Close()

	// srv.URL is http://127.0.0.1:PORT — put the real endpoint's host in the
	// userinfo, and its path on the end, so the whole string reads like it.
	raw := "http://" + PlanUsageHost + "@" + strings.TrimPrefix(srv.URL, "http://") + "/api/oauth/usage"
	if credentialedURL(raw, PlanUsageURL) {
		t.Fatalf("%s was treated as the compiled-in endpoint", raw)
	}

	t.Setenv("RHQ_PLAN_USAGE_URL", raw)
	r := NewAnthropicPlanReader()
	if r.URLErr != nil {
		t.Fatalf("the host is loopback, so the seam must still work: %v", r.URLErr)
	}
	if r.Shared {
		t.Fatal("a userinfo spelling of the endpoint must not be the fleet's fact")
	}
	r.Token = refusingToken(t)

	dir := t.TempDir()
	c := &PlanCache{
		Path:   filepath.Join(dir, "plan-usage.json"),
		Log:    filepath.Join(dir, "plan-usage.log"),
		Caller: "cost",
		Reader: r,
	}
	if _, _, err := c.Read(0); err != nil {
		t.Fatalf("the seam must keep working: %v", err)
	}
	if hits != 1 {
		t.Fatalf("%d requests reached the listener, want 1", hits)
	}
	if strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("the account's credential reached a caller-named listener: %q", gotAuth)
	}
	if _, err := os.ReadFile(c.Path); err == nil {
		t.Error("a userinfo spelling of the endpoint published to the instance's snapshot")
	}
}

// ─── ranger-base-07ep: belt 3 is the host that was ASKED ─────────────────────
//
// Rules 2 and 3 used to share reachHost — "the compiled-in host OR this
// machine" — which is the right SET for where a reader may be POINTED and
// the wrong one for which host may ANSWER. Under it a 302 from
// api.anthropic.com to a listener on 127.0.0.1 was both followed and
// believed, and neither sink behind belt 3 would have caught it: the reader
// that followed the redirect IS the compiled-in one, so rule 5's store gate
// is open, and ModelCache.store has no store gate at all.
//
// The precondition is control of the network path or of the upstream (a
// trusted CA, DNS + TLS interception, a compromised endpoint, an operator's
// proxy) rather than the env var and socket dr6u closed, which is what makes
// this hardening rather than exploit-now — and no reason at all to leave a
// belt that says "the answer came from where I asked" not doing that.
//
// Every transport below redirects to a LOOPBACK authority. That is the
// point: loopback is exactly the host the old rule waved through, so a
// listener.example target would pass these tests against the bug.

// The pinned client does not dial the redirect at all, and nothing reaches
// the catalog on disk. rt.asked is the assertion that carries it: the second
// host was never asked, which is what "refused before dialing" means from
// the listener's side.
func TestTheEndpointMayNotRedirectTheCatalogToThisMachine(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	rt := newRedirectTransport("127.0.0.1:9", `{"data":[{"id":"probe-model"}],"has_more":false}`)
	cl := pinnedClient(time.Second, "model list endpoint")
	cl.Transport = rt.client.Transport
	a.ModelLister = &ModelLister{
		URL:   ModelListURL, // the compiled-in endpoint, not an override
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
		HTTP:  cl,
	}

	if ids, ok := a.ModelCache().Models(time.Hour); ok {
		t.Fatalf("a catalog redirected to this machine was believed: %v", ids)
	}
	if rt.asked {
		t.Error("the loopback redirect target was dialed — the pinned client must refuse first")
	}
	b, err := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.json"))
	if err == nil && strings.Contains(string(b), "probe-model") {
		t.Errorf("the redirected answer was cached: %s", b)
	}
	log, _ := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.log"))
	if !strings.Contains(string(log), "127.0.0.1") || !strings.Contains(string(log), "refused") {
		t.Errorf("the refusal must be diagnosable in the probe log:\n%s", log)
	}
	if strings.Contains(string(log), fakeToken) {
		t.Errorf("the probe log contains the credential: %s", log)
	}
}

// And the second line holds on its own. An injected client without our
// CheckRedirect follows the 302 and gets its 200, so rt.asked is TRUE here
// on purpose: this arm is about the answer that came back, and belt 3 is
// the only thing between it and model-catalog.json.
func TestACatalogAnsweredByThisMachineIsNotAnAnswer(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	rt := newRedirectTransport("127.0.0.1:9", `{"data":[{"id":"probe-model"}],"has_more":false}`)
	a.ModelLister = &ModelLister{
		URL:   ModelListURL,
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
		HTTP:  rt.client,
	}

	if ids, ok := a.ModelCache().Models(time.Hour); ok {
		t.Fatalf("a catalog redirected to this machine was believed: %v", ids)
	}
	if !rt.asked {
		t.Fatal("setup: the loopback target was never asked, so belt 3 is not what refused")
	}
	b, err := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.json"))
	if err == nil && strings.Contains(string(b), "probe-model") {
		t.Errorf("the redirected answer was cached: %s", b)
	}
}

// The plan sink, and the arm that shows belt 3 is not redundant with rule
// 5: this reader is the compiled-in one, so MayShare() is true and the
// store gate is standing wide open. `$StateDir/plan-usage.json` is read by
// every posse process on the instance for the TTL (rangerhq-tdy8), and 0%
// disarms the plan guard for all of them.
func TestAUsageAnswerFromThisMachineNeverBecomesTheFleetsFact(t *testing.T) {
	t.Parallel()
	rt := newRedirectTransport("127.0.0.1:9", `{"five_hour":{"utilization":0},"seven_day":{"utilization":0}}`)
	r := &AnthropicPlanReader{
		URL:    PlanUsageURL,
		Shared: true, // the compiled-in reader: rule 5 does not stand here
		Token:  func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
		HTTP:   rt.client,
	}
	if !r.MayShare() {
		t.Fatal("setup: this arm is only interesting while the store gate is open")
	}
	dir := t.TempDir()
	c := &PlanCache{
		Path:   filepath.Join(dir, "plan-usage.json"),
		Log:    filepath.Join(dir, "plan-usage.log"),
		Caller: "cost",
		Reader: r,
	}
	seed := `{"at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","windows":[{"name":"5h","pct":42},{"name":"7d","pct":61}]}`
	if err := os.WriteFile(c.Path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := c.Read(0)
	var pin *PinRefusal
	if !errors.As(err, &pin) {
		t.Fatalf("Read error = %v, want a *PinRefusal", err)
	}
	if !pin.Redirect || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("the refusal must say a redirect took it elsewhere: %q", err)
	}
	got, _ := os.ReadFile(c.Path)
	if !strings.Contains(string(got), `"pct":42`) || strings.Contains(string(got), `"pct":0`) {
		t.Errorf("a redirected answer overwrote the fleet's reading: %s", got)
	}
}

// The rule is symmetric, and the symmetry is what makes it "the host you
// asked" rather than a second spelling of "the compiled-in host": a
// loopback override may not be answered by api.anthropic.com either.
// Nobody is attacked by this direction — it is here because a rule with one
// arm is a coincidence.
func TestALoopbackOverrideMayNotBeAnsweredByTheEndpoint(t *testing.T) {
	t.Parallel()
	rt := newRedirectTransport(PlanUsageHost, `{"five_hour":{"utilization":1}}`)
	r := &AnthropicPlanReader{
		URL:   "http://127.0.0.1:9/usage",
		Token: refusingToken(t), // an override is uncredentialed (rule 4)
		HTTP:  rt.client,
	}
	_, err := r.Read()
	var pin *PinRefusal
	if !errors.As(err, &pin) || !pin.Redirect {
		t.Fatalf("Read error = %v, want a redirect *PinRefusal", err)
	}
	if !strings.Contains(err.Error(), PlanUsageHost) {
		t.Errorf("the refusal must name the host that answered: %q", err)
	}
}

// The control, so this is not read as "refuse every redirect": a path-only
// redirect on the SAME host is the endpoint moving its own URL, and it is
// still followed. Over a real socket, because a fake transport cannot tell
// a redirect that was followed from one that was answered where it stood.
func TestAPathOnlyRedirectOnTheSameHostIsStillFollowed(t *testing.T) {
	t.Parallel()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path != "/v2/models" {
			http.Redirect(w, r, "/v2/models", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"probe-model"}],"has_more":false}`))
	}))
	defer srv.Close()

	l := &ModelLister{
		URL:   srv.URL + "/v1/models",
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
		HTTP:  pinnedClient(5*time.Second, "model list endpoint"),
	}
	ids, err := l.List()
	if err != nil {
		t.Fatalf("a same-host redirect must still be followed: %v", err)
	}
	if len(ids) != 1 || ids[0] != "probe-model" {
		t.Errorf("List = %v, want the catalog behind the redirect", ids)
	}
	if len(paths) != 2 {
		t.Errorf("the server saw %v, want the redirect and the answer", paths)
	}
}

// A want nobody could name is not a want everybody satisfies. An
// unparseable configured URL yields an empty asked host, and belt 3 refuses
// on it rather than waving it through — the direction a belt is allowed to
// be wrong in.
func TestAnAnswerIsRefusedWhenTheAskedHostIsUnknown(t *testing.T) {
	t.Parallel()
	if got := askedHost("://not a url"); got != "" {
		t.Errorf("askedHost(garbage) = %q, want empty", got)
	}
	u, err := url.Parse("https://" + ModelListHost + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	resp := &http.Response{Request: &http.Request{URL: u}}
	if err := pinnedResponse("model list endpoint", resp, ""); err == nil {
		t.Error("an empty asked host accepted an answer — belt 3 must fail closed")
	}
}
