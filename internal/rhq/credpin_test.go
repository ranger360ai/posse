package rhq

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
func refusingToken(t *testing.T) func() (string, error) {
	t.Helper()
	return func() (string, error) {
		t.Error("the keychain was read for a host the pin refuses")
		return fakeToken, nil
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
			r := NewPlanReader()
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
		if got := NewPlanReader(); got.URL != raw || got.URLErr != nil {
			t.Errorf("loopback override %s not honoured: url=%s err=%v", raw, got.URL, got.URLErr)
		}
	}
}

// ─── the belt: whatever set the URL ──────────────────────────────────────────

// A field set in code, a test, or a caller who forgot the rule: the
// Authorization header is never put in front of a host we do not credential.
func TestReadersRefuseAnUncredentialedHostWhateverSetTheURL(t *testing.T) {
	t.Run("plan", func(t *testing.T) {
		r := &PlanReader{URL: "https://listener.example/usage", Token: refusingToken(t), HTTP: refusingTransport(t)}
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
	a := preflightApp(t)
	rt := newRedirectTransport("listener.example", `{"data":[{"id":"probe-model"}],"has_more":false}`)
	a.ModelLister = &ModelLister{
		URL:   "http://127.0.0.1:9/v1/models",
		Token: func() (string, error) { return fakeToken, nil },
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
	rt := newRedirectTransport("listener.example", `{"five_hour":{"utilization":1}}`)
	r := &PlanReader{URL: "http://127.0.0.1:9/usage", Token: func() (string, error) { return fakeToken, nil }, HTTP: rt.client}
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
	rt := newRedirectTransport("listener.example", "{}")
	cl := pinnedClient(time.Second, "model list endpoint", ModelListHost)
	cl.Transport = rt.client.Transport

	l := &ModelLister{URL: "http://127.0.0.1:9/v1/models", Token: func() (string, error) { return fakeToken, nil }, HTTP: cl}
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
