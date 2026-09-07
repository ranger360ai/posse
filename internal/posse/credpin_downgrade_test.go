package posse

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const probeToken = "Bearer probe-token-synthetic-all-letters"

// Arm A: real sockets. https://127.0.0.1:A 302s to http://127.0.0.1:B (same
// hostname, scheme downgrade). Expected AFTER the fix: *PinRefusal{Redirect}
// and the plaintext listener never dialed. Red today = the finding.
func TestProbe246xoSocketDowngradeCarriesTheBearer(t *testing.T) {
	t.Parallel()
	var got string
	dialed := false
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dialed = true
		got = r.Header.Get("Authorization")
		w.Write([]byte(`{"data":[{"id":"probe-model"}],"has_more":false}`))
	}))
	defer plain.Close()
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
	}))
	defer tls.Close()

	cl := pinnedClient(10*time.Second, "model list endpoint")
	cl.Transport = tls.Client().Transport
	req, _ := http.NewRequest("GET", tls.URL+"/v1/models", nil)
	req.Header.Set("Authorization", probeToken)
	resp, err := cl.Do(req)
	t.Logf("asked=%s plainDialed=%v plainSawAuthorization=%v err=%v", tls.URL, dialed, got != "", err)
	var pin *PinRefusal
	if !errors.As(err, &pin) || !pin.Redirect {
		if resp != nil {
			t.Logf("answered by %s status %s; pinnedResponse=%v", resp.Request.URL, resp.Status, pinnedResponse("model list endpoint", resp, askedHost(tls.URL), askedScheme(tls.URL)))
			resp.Body.Close()
		}
		t.Errorf("want *PinRefusal{Redirect:true}, got err=%v", err)
	}
	if dialed {
		t.Errorf("the plaintext listener was dialed")
	}
	if got != "" {
		t.Errorf("the plaintext listener received the Authorization header (value withheld; len=%d)", len(got))
	}
}

// Control: same shape, different HOSTNAME (localhost vs 127.0.0.1) — refused today.
func TestProbe246xoControlCrossHostIsRefused(t *testing.T) {
	t.Parallel()
	dialed := false
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { dialed = true }))
	defer plain.Close()
	to := strings.Replace(plain.URL, "127.0.0.1", "localhost", 1)
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to+r.URL.Path, http.StatusFound)
	}))
	defer tls.Close()
	cl := pinnedClient(10*time.Second, "model list endpoint")
	cl.Transport = tls.Client().Transport
	req, _ := http.NewRequest("GET", tls.URL+"/v1/models", nil)
	req.Header.Set("Authorization", probeToken)
	_, err := cl.Do(req)
	var pin *PinRefusal
	if !errors.As(err, &pin) || !pin.Redirect || dialed {
		t.Fatalf("control: want refusal, got err=%v dialed=%v", err, dialed)
	}
	t.Logf("control refused: %v", err)
}

// downgradeTransport answers the compiled-in https URL with a 302 to the same
// host over http, and answers the http one with 200, recording the header. No
// socket is dialed — the compiled-in host is never reached.
type downgradeTransport struct {
	body  string
	auth  string
	plain bool
}

func (d *downgradeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme == "http" {
		d.plain = true
		d.auth = r.Header.Get("Authorization")
		return &http.Response{StatusCode: 200, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(d.body)), Request: r}, nil
	}
	if r.URL.Scheme != "https" || r.URL.Host != "api.anthropic.com" {
		return nil, errors.New("probe rig: unexpected request to " + r.URL.String())
	}
	h := make(http.Header)
	h.Set("Location", "http://"+r.URL.Host+r.URL.RequestURI())
	return &http.Response{StatusCode: 302, Status: "302 Found", Header: h,
		Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
}

// Arm B: both shipped readers, at their compiled-in URLs (so the plan reader
// is credentialed), through the shipped pinnedClient with the rig transport.
func TestProbe246xoModelListerHandsTheBearerToPlaintext(t *testing.T) {
	t.Parallel()
	d := &downgradeTransport{body: `{"data":[{"id":"probe-model"}],"has_more":false}`}
	cl := pinnedClient(10*time.Second, "model list endpoint")
	cl.Transport = d
	l := &ModelLister{URL: ModelListURL, Token: func() (string, CredMeta, error) { return "sk-probe-token-246xo", CredMeta{}, nil }, HTTP: cl}
	ids, err := l.List()
	t.Logf("List ids=%v err=%v plainAsked=%v plainSawAuthorization=%v", ids, err, d.plain, d.auth != "")
	var pin *PinRefusal
	if !errors.As(err, &pin) || d.plain || d.auth != "" {
		t.Errorf("want refusal and no plaintext request; got err=%v plain=%v auth=%v", err, d.plain, d.auth != "")
	}
}

func TestProbe246xoPlanReaderHandsTheBearerToPlaintext(t *testing.T) {
	t.Parallel()
	d := &downgradeTransport{body: `{"five_hour":{"utilization":0},"seven_day":{"utilization":0}}`}
	cl := pinnedClient(10*time.Second, "usage endpoint")
	cl.Transport = d
	r := &AnthropicPlanReader{URL: PlanUsageURL, Token: func() (string, CredMeta, error) { return "sk-probe-token-246xo", CredMeta{}, nil }, HTTP: cl}
	u, err := r.Read()
	t.Logf("Read usage=%+v err=%v plainAsked=%v plainSawAuthorization=%v", u, err, d.plain, d.auth != "")
	var pin *PinRefusal
	if !errors.As(err, &pin) || d.plain || d.auth != "" {
		t.Errorf("want refusal and no plaintext request; got err=%v plain=%v auth=%v", err, d.plain, d.auth != "")
	}
}

// Arm C: the ANSWER belt alone (an injected client without CheckRedirect):
// does pinnedResponse accept a plain-http answer to an https ask?
func TestProbe246xoAnswerBeltAcceptsAPlaintextAnswer(t *testing.T) {
	t.Parallel()
	d := &downgradeTransport{body: `{"data":[],"has_more":false}`}
	cl := &http.Client{Transport: d}
	req, _ := http.NewRequest("GET", ModelListURL, nil)
	req.Header.Set("Authorization", probeToken)
	resp, err := cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	verdict := pinnedResponse("model list endpoint", resp, askedHost(ModelListURL), askedScheme(ModelListURL))
	t.Logf("answer from %s; pinnedResponse=%v; plainSawAuthorization=%v", resp.Request.URL, verdict, d.auth != "")
	if verdict == nil {
		t.Errorf("answer belt accepted a plaintext answer to an https ask")
	}
}
