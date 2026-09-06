//go:build posse_arm2

package posse

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

// ranger-base-dr6u, over the wire and from the listener's side: this is
// qa's repro on ranger-base-7nlw with the assertion inverted.
//
// The seam still works end to end, with the reader's own transport and no
// fake anywhere — the request arrives, the beta header arrives, the body
// parses. What does NOT arrive is the account's credential, and the
// keychain is not read to produce one: on this machine a 127.0.0.1 listener
// is something any seatbelt-caged session can start, so a loopback override
// is a destination the CALLER chose. Reaching it is a seam; being handed
// the account's token is not.
//
// A fake transport cannot prove this half either: the assertion is about
// the bytes a socket that is genuinely listening did not receive.
func TestLoopbackOverrideCarriesNoBearerToTheSocket(t *testing.T) {
	w := newWireServer(t, `{"five_hour":{"utilization":42},"seven_day":{"utilization":43}}`)
	w.reachable(t)
	t.Setenv("RHQ_PLAN_USAGE_URL", w.URL+"/usage")

	r := NewAnthropicPlanReader() // HTTP left alone: the real pinnedClient dials.
	if r.URLErr != nil {
		t.Fatalf("loopback override refused: %v", r.URLErr)
	}
	r.Token = func() (string, CredMeta, error) {
		t.Error("the keychain was read for a listener the caller named")
		return fakeToken, CredMeta{}, nil
	}

	u, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if win(u, "5h") != 42 || win(u, "7d") != 43 {
		t.Errorf("parsed %+v, want 42/43 — the seam must keep working", u)
	}
	if w.hits != 1 {
		t.Fatalf("%d requests reached the endpoint, want 1", w.hits)
	}
	if w.auth != "" {
		t.Errorf("the account's credential reached a caller-named listener: %q", w.auth)
	}
	if w.beta != planBetaHeader {
		t.Errorf("anthropic-beta header on the wire = %q", w.beta)
	}
	// And the reading is the caller's own: it is not the fleet's fact.
	if r.Shared {
		t.Error("an override's reading must not be shareable")
	}
}

// A non-loopback override, with a real listener up on loopback the whole
// time: no bytes anywhere, and the keychain is not read at all. The fake
// transports cannot show the first half of that.
func TestNonLoopbackOverrideReachesNoSocketAndNoKeychain(t *testing.T) {
	w := newWireServer(t, `{"five_hour":{"utilization":99}}`)
	w.reachable(t)
	t.Setenv("RHQ_PLAN_USAGE_URL", "http://listener.example/usage")

	r := NewAnthropicPlanReader()
	r.Token = func() (string, CredMeta, error) {
		t.Error("the keychain was read for a host the pin refuses")
		return fakeToken, CredMeta{}, nil
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
	t.Parallel()
	w := &wireServer{}
	w.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.hits++
		http.Redirect(rw, r, "http://198.51.100.7:9/v1/models", http.StatusFound)
	}))
	t.Cleanup(w.Close)

	l := &ModelLister{
		URL:   w.URL + "/v1/models",
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
		HTTP:  pinnedClient(30*time.Second, "model list endpoint"),
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

// WHAT THIS FILE CLAIMS, AND WHAT IT STILL DOES NOT (ranger-base-8rff's
// verdict, closed by ranger-base-dr6u and ranger-base-07ep).
//
// It used to end here saying the loopback half was a narrowed hole rather
// than a closed one: loopback is honoured by NAME because a test server is
// always on it, and the wire test above USED to show the account's bearer
// arriving at a listener the caller chose. That is now inverted — the
// credential does not leave for an override and the keychain is not read
// for one, and the answer an override gives is not written to the snapshot
// the rest of the fleet reads (credpin.go rules 4 and 5).
//
// The paragraph below it named the second gap: belt 3 accepted an answer
// from loopback when the request had gone to the compiled-in host, so a
// redirect from a compromised or intercepted upstream to a local listener
// was decoded and — because that reader IS the compiled-in one, and because
// ModelCache.store has no share gate at all — cached. That is inverted too
// (ranger-base-07ep). Rule 3 no longer asks reachHost's "compiled-in OR
// this machine"; it asks whether the host that answered is the host this
// reader ASKED, so a redirect must stay on its own host in both directions.
// The pins are in credpin_test.go under that bead's heading, with a mutant
// restoring the old rule reddening exactly the three arms that name it, and
// a path-only same-host redirect still followed as the control.
//
// What is still not claimed, and is deliberate rather than pending: the
// unit of rule 3 is the HOSTNAME, so one loopback listener redirecting to
// another loopback listener is not refused by it. Reaching that needs the
// reader already pointed at a listener the caller chose, which is rules 4
// and 5's ground — no credential goes there and its answer is not the
// fleet's fact. Comparing host:port instead would refuse a plain
// `https://host` → `https://host:443` as an outage in front of a launch.
