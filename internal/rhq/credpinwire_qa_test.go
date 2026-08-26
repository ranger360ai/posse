package rhq

// QA, ranger-base-8rff — the endpoint pin (ranger-base-17i) over a REAL
// socket.
//
// credpin_test.go proves the pin against RoundTripper fakes, and a fake
// transport cannot prove the one thing the bead is about: that the request
// was never MADE. It answers whatever it is asked, so "the pin refused" and
// "the pin followed the override to somewhere else" look the same from
// inside it. Every server below is an httptest listener on loopback with
// the readers' own real transport in front of it, so an override that still
// moved the endpoint would show up here as bytes arriving at a socket that
// is genuinely listening.
//
// Verified by hand the same way before it was written down: a listener on
// 127.0.0.1 and the posse binary, with a stub `security` handing out a
// synthetic token and a marker file. The 17i repro
// (`RHQ_MODEL_LIST_URL=http://<listener>/v1/models posse gates <persona>`)
// delivered nothing to the listener; `posse cost --plan` under a
// non-loopback override was refused by name with the marker file never
// created. Both are pinned below.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// wireServer is a real listener on loopback that records what arrived.
type wireServer struct {
	*httptest.Server
	hits  int
	auth  string
	beta  string
	paths []string
}

func newWireServer(t *testing.T, body string) *wireServer {
	t.Helper()
	w := &wireServer{}
	w.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.hits++
		w.auth, w.beta = r.Header.Get("Authorization"), r.Header.Get("anthropic-beta")
		w.paths = append(w.paths, r.URL.Path)
		rw.Header().Set("Content-Type", "application/json")
		rw.Write([]byte(body))
	}))
	t.Cleanup(w.Close)
	return w
}

// reachable proves the assertions below are not vacuous: an exfil
// destination that was never listening would make "nothing arrived" true
// for the wrong reason.
func (w *wireServer) reachable(t *testing.T) {
	t.Helper()
	resp, err := http.Get(w.URL + "/control")
	if err != nil {
		t.Fatalf("the test listener is not up, so nothing below is proven: %v", err)
	}
	resp.Body.Close()
	w.hits, w.paths = 0, nil
}

// ─── the override that is gone, against a listener that is up ────────────────

// The 17i repro from the listener's side: a listener is running, every name
// the override could plausibly have is pointed at it, and the model list
// endpoint still names the compiled-in host. Nothing arrives.
func TestNoEnvVarPointsTheModelListAtAListener(t *testing.T) {
	w := newWireServer(t, `{"data":[{"id":"probe-model"}],"has_more":false}`)
	w.reachable(t)
	for _, k := range []string{
		"RHQ_MODEL_LIST_URL", "RHQ_MODEL_URL", "RHQ_MODELS_URL",
		"RHQ_PLAN_USAGE_URL", "RHQ_CATALOG_URL", "ANTHROPIC_BASE_URL",
	} {
		t.Setenv(k, w.URL+"/v1/models")
	}
	if got := NewModelLister().URL; got != ModelListURL {
		t.Fatalf("model list endpoint = %s, want the compiled-in %s", got, ModelListURL)
	}
	if w.hits != 0 {
		t.Errorf("%d request(s) reached the listener: %v", w.hits, w.paths)
	}
}

// ─── the override that is pinned, over the wire ──────────────────────────────

// The seam still works end to end, with the reader's own transport and no
// fake anywhere: a loopback override is honoured and the bearer arrives.
// This is the behaviour the pin deliberately keeps — see the comment at the
// bottom of this file for what it costs.
func TestLoopbackOverrideCarriesTheBearerToTheSocket(t *testing.T) {
	w := newWireServer(t, `{"five_hour":{"utilization":42},"seven_day":{"utilization":43}}`)
	w.reachable(t)
	t.Setenv("RHQ_PLAN_USAGE_URL", w.URL+"/usage")

	r := NewPlanReader() // HTTP left alone: the real pinnedClient dials.
	if r.URLErr != nil {
		t.Fatalf("loopback override refused: %v", r.URLErr)
	}
	r.Token = func() (string, error) { return fakeToken, nil }

	u, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if u.FiveHour != 42 || u.SevenDay != 43 {
		t.Errorf("parsed %+v, want 42/43", u)
	}
	if w.hits != 1 {
		t.Fatalf("%d requests reached the endpoint, want 1", w.hits)
	}
	if w.auth != "Bearer "+fakeToken || w.beta != planBetaHeader {
		t.Errorf("headers on the wire: auth=%q beta=%q", w.auth, w.beta)
	}
}

// A non-loopback override, with a real listener up on loopback the whole
// time: no bytes anywhere, and the keychain is not read at all. The fake
// transports cannot show the first half of that.
func TestNonLoopbackOverrideReachesNoSocketAndNoKeychain(t *testing.T) {
	w := newWireServer(t, `{"five_hour":{"utilization":99}}`)
	w.reachable(t)
	t.Setenv("RHQ_PLAN_USAGE_URL", "http://listener.example/usage")

	r := NewPlanReader()
	r.Token = func() (string, error) {
		t.Error("the keychain was read for a host the pin refuses")
		return fakeToken, nil
	}
	_, err := r.Read()
	var pin *PinRefusal
	if !errors.As(err, &pin) {
		t.Fatalf("Read error = %v, want a *PinRefusal", err)
	}
	if r.URL != PlanUsageURL {
		t.Errorf("a refused override still moved the URL: %s", r.URL)
	}
	if w.hits != 0 {
		t.Errorf("%d request(s) reached the listener: %v", w.hits, w.paths)
	}
}

// ─── the belt, over the wire ─────────────────────────────────────────────────

// The pinned client refuses a redirect off the pinned host before it dials
// it. The target is TEST-NET-3 (RFC 5737, never routed), so a refusal that
// did not happen would show up as a dial error rather than as a quiet pass:
// what is asserted is that the error is the REFUSAL, not the timeout.
func TestPinnedClientRefusesARedirectBeforeDialingIt(t *testing.T) {
	w := &wireServer{}
	w.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.hits++
		http.Redirect(rw, r, "http://198.51.100.7:9/v1/models", http.StatusFound)
	}))
	t.Cleanup(w.Close)

	l := &ModelLister{
		URL:   w.URL + "/v1/models",
		Token: func() (string, error) { return fakeToken, nil },
		HTTP:  pinnedClient(30*time.Second, "model list endpoint", ModelListHost),
	}
	start := time.Now()
	_, err := l.List()
	var pin *PinRefusal
	if !errors.As(err, &pin) {
		t.Fatalf("List error = %v, want a *PinRefusal", err)
	}
	if !pin.Redirect || !strings.Contains(err.Error(), "198.51.100.7") {
		t.Errorf("the refusal must name the redirect and its host: %q", err)
	}
	if strings.Contains(err.Error(), "unreachable") {
		t.Errorf("a refusal must not be reported as an outage: %q", err)
	}
	if strings.Contains(err.Error(), fakeToken) {
		t.Errorf("the refusal quotes the credential: %q", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("took %s — the redirect was dialed rather than refused", d)
	}
	if w.hits != 1 {
		t.Errorf("the pinned host was asked %d times, want 1", w.hits)
	}
}

// WHAT THIS FILE DOES NOT CLAIM (ranger-base-8rff verdict, and the reason
// for the residual filed alongside it): loopback is honoured by NAME for a
// reason — a test server is always on it — and the test above shows the
// account's bearer arriving at a listener the CALLER chose. On this machine
// a listener is something any session with a shell can start, so the
// loopback half of the pin is a narrowed hole, not a closed one. That is
// the bead's own design decision (17i fix 2), not a defect in it.
