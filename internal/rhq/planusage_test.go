package rhq

// Hermetic tests for the plan-utilization guard (rangerhq-jgm): a fake
// usage endpoint and a fake keychain, so nothing here touches the network
// or the operator's credentials.

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fakeToken = "sk-fake-do-not-log-me"

// planServer is a fake usage endpoint. It counts requests and records the
// last Authorization/anthropic-beta headers it saw.
type planServer struct {
	URL    string
	client *http.Client
	hits   atomic.Int64
	closed atomic.Bool
	auth   string
	beta   string
	status int
	body   string
	retry  string // Retry-After, when the fake endpoint is rate-limiting
}

func newPlanServer(t *testing.T, fiveH, sevenD float64) *planServer {
	t.Helper()
	ps := &planServer{status: http.StatusOK}
	// Loopback, because the endpoint override and the credentialed request
	// are both pinned to it (credpin.go). Nothing listens on it: the fake
	// transport below answers, so the port is decoration.
	ps.URL = "https://127.0.0.1:9/usage"
	ps.body = fmt.Sprintf(`{"five_hour":{"utilization":%g,"resets_at":"2026-08-18T12:00:00Z"},`+
		`"seven_day":{"utilization":%g,"resets_at":"2026-08-24T12:00:00Z"}}`, fiveH, sevenD)
	ps.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if ps.closed.Load() || r.URL.String() != ps.URL {
			return nil, fmt.Errorf("fake usage endpoint down")
		}
		ps.hits.Add(1)
		ps.auth = r.Header.Get("Authorization")
		ps.beta = r.Header.Get("anthropic-beta")
		h := make(http.Header)
		if ps.retry != "" {
			h.Set("Retry-After", ps.retry)
		}
		return &http.Response{
			StatusCode: ps.status,
			Status:     fmt.Sprintf("%d %s", ps.status, http.StatusText(ps.status)),
			Header:     h,
			Body:       io.NopCloser(strings.NewReader(ps.body)),
			Request:    r,
		}, nil
	})}
	t.Cleanup(ps.Close)
	return ps
}

func (ps *planServer) Close() { ps.closed.Store(true) }

// reader is the fake endpoint as a PlanReader. Shared is set explicitly:
// the field's zero value is "this reading is nobody's fact" (credpin.go
// rule 5), and every cache test below is about a reading that IS shared —
// so the fake stands in for the compiled-in endpoint, not for an override.
// The tests that are about an override build their reader from
// NewPlanReader with RHQ_PLAN_USAGE_URL set, which is the only way a
// running posse can get an unshared one.
func (ps *planServer) reader() *PlanReader {
	return &PlanReader{
		URL:    ps.URL,
		Token:  func() (string, error) { return fakeToken, nil },
		HTTP:   ps.client,
		Shared: true,
	}
}

// keychainOnly points a plan reader at the compiled-in endpoint with the
// token source a test wants to fail. Since ranger-base-dr6u the keychain is
// read only for that url (credpin.go rule 4), so a test about a CREDENTIAL
// failure has to be pointed there — the fake endpoint is not one. Nothing
// is dialled either way: PlanReader asks for the token first, so the
// failure is the credential and not the transport.
func keychainOnly(r *PlanReader, tok func() (string, error)) {
	r.URL, r.Token = PlanUsageURL, tok
}

// planConfig writes a config with the guard thresholds and one beads repo.
// Thresholds go first: the flat-YAML block list runs to the end of its key.
func planConfig(t *testing.T, a *App, repo, thresholds string) {
	t.Helper()
	cfg := thresholds
	if cfg != "" && !strings.HasSuffix(cfg, "\n") {
		cfg += "\n"
	}
	if repo != "" {
		cfg += "beads:\n  - " + repo + "\n"
	}
	if err := os.WriteFile(a.ConfigPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

// planRepo is qaRepo without the config write — planConfig owns that here.
func planRepo(t *testing.T, ready, show string) string {
	t.Helper()
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(ready), 0o644)
	if show != "" {
		os.WriteFile(filepath.Join(repo, "fake-show.json"), []byte(show), 0o644)
	}
	return repo
}

// planDispatcher: a dispatcher whose guard talks to ps and whose stderr is
// captured.
func planDispatcher(t *testing.T, b *HerdrBackend, ps *planServer) (*Dispatcher, *strings.Builder) {
	t.Helper()
	d := newTestDispatcher(t, b)
	var errb strings.Builder
	d.Err = &errb
	if ps != nil {
		d.Plan = ps.reader()
	}
	return d, &errb
}

// Below both thresholds: the pass runs exactly as it does today.
func TestPlanGuardBelowThresholdsRunsPass(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 42, 61)
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70\nplan_guard_7d: 85")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 dispatched below thresholds, got %d\n%s", n, dispatcherOut(d))
	}
	if got := ps.hits.Load(); got != 1 {
		t.Errorf("want exactly 1 usage fetch per pass, got %d", got)
	}
	if out := dispatcherOut(d); strings.Contains(out, "— skipped") {
		t.Errorf("pass must not be skipped below thresholds:\n%s", out)
	}
	if errb.Len() != 0 {
		t.Errorf("nothing should reach stderr on a clean read: %q", errb.String())
	}
	// The token is a credential: it may not appear in any output.
	if strings.Contains(dispatcherOut(d)+errb.String(), fakeToken) {
		t.Error("the access token was written to dispatch output")
	}
}

// Above the 5h threshold: the on-meter bead parks with the exact reason.
// Zero dispatched still makes --watch read it as a quiet pass.
func TestPlanGuardSkipsAbove5h(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 78, 40)
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	planConfig(t, b.App, repo, "plan_guard_5h: 70\nplan_guard_7d: 85")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("want 0 dispatched above the 5h threshold, got %d", n)
	}
	out := dispatcherOut(d)
	if want := "plan 5h at 78% > 70% — skipped"; !strings.Contains(out, want) {
		t.Errorf("want %q, got:\n%s", want, out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "--claim") {
		t.Errorf("a parked bead must not be claimed, got: %s", calls)
	}
	if errb.Len() != 0 {
		t.Errorf("a working guard says nothing on stderr: %q", errb.String())
	}
	// A skipped pass dispatches nothing, so --watch backs off.
	base := 30 * time.Second
	if got := NextInterval(base, base, 8*base, n); got != 2*base {
		t.Errorf("--watch backoff after a skipped pass: %s, want %s", got, 2*base)
	}
}

// Above the 7d threshold with the 5h window fine: same skip, other window
// named.
func TestPlanGuardSkipsAbove7d(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 12, 88)
	d, _ := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	planConfig(t, b.App, repo, "plan_guard_5h: 70\nplan_guard_7d: 85")
	idleClaude(t, fake)

	n, _ := d.Run("", "", 0)
	out := dispatcherOut(d)
	if want := "plan 7d at 88% > 85% — skipped"; n != 0 || !strings.Contains(out, want) {
		t.Errorf("want %q (0 dispatched), got %d:\n%s", want, n, out)
	}
}

// One threshold set, the other unset: only the set one gates.
func TestPlanGuardOneWindowOnly(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 40, 99)
	d, _ := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70")
	idleClaude(t, fake)

	if n, _ := d.Run("", "", 0); n != 1 {
		t.Errorf("7d unset must not gate (7d was 99%%): %d dispatched\n%s", n, dispatcherOut(d))
	}
}

// Exactly at the threshold is not above it.
func TestPlanGuardAtThresholdRuns(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 70, 0)
	d, _ := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: 70")
	idleClaude(t, fake)

	if n, _ := d.Run("", "", 0); n != 1 {
		t.Errorf("5h exactly at the threshold must run: %d dispatched\n%s", n, dispatcherOut(d))
	}
}

// Guard off (no thresholds in config): not one request is made.
func TestPlanGuardOffMakesNoRequest(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 99, 99) // would skip every pass if it were read
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("guard unset is today's behaviour: want 1 dispatched, got %d\n%s", n, dispatcherOut(d))
	}
	if got := ps.hits.Load(); got != 0 {
		t.Errorf("guard unset must make zero usage requests, got %d", got)
	}
	if errb.Len() != 0 {
		t.Errorf("guard unset is silent: %q", errb.String())
	}
}

// The endpoint (or the keychain) is unreadable: fail open — the pass runs,
// with one line on stderr, and nothing about it on the pass output.
func TestPlanGuardUnreadableFailsOpen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mut   func(*planServer, *PlanReader)
		wants string
	}{
		{"endpoint 500", func(ps *planServer, r *PlanReader) { ps.status = http.StatusInternalServerError },
			"500"},
		{"endpoint down", func(ps *planServer, r *PlanReader) { ps.Close(); r.URL = ps.URL },
			"unreachable"},
		{"garbage body", func(ps *planServer, r *PlanReader) { ps.body = "<html>nope" },
			"not the expected JSON"},
		{"keychain locked", func(ps *planServer, r *PlanReader) {
			keychainOnly(r, func() (string, error) {
				return "", Die("keychain item %q unreadable", KeychainService)
			})
		}, "keychain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			ps := newPlanServer(t, 99, 99)
			d, errb := planDispatcher(t, b, ps)
			tc.mut(ps, d.Plan)
			writePersona(t, b.App, "ranger", "[go]")
			repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"closed"}]`)
			planConfig(t, b.App, repo, "plan_guard_5h: 70\nplan_guard_7d: 85")
			idleClaude(t, fake)

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("a monitoring failure must never halt the fleet: %d dispatched\n%s", n, dispatcherOut(d))
			}
			lines := strings.Split(strings.TrimRight(errb.String(), "\n"), "\n")
			if len(lines) != 1 || !strings.Contains(lines[0], "plan guard:") || !strings.Contains(lines[0], tc.wants) {
				t.Errorf("want exactly one stderr line naming %q, got: %q", tc.wants, errb.String())
			}
			if strings.Contains(errb.String(), fakeToken) {
				t.Error("the access token leaked into the failure line")
			}
			if out := dispatcherOut(d); strings.Contains(out, "— skipped") {
				t.Errorf("fail-open must not skip:\n%s", out)
			}
		})
	}
}

// A threshold that is not a percent is a typo, not a guard: say so once and
// treat that window as unset (rather than silently gating nothing).
func TestPlanGuardBadThreshold(t *testing.T) {
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 99, 10)
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, "plan_guard_5h: seventy\nplan_guard_7d: 85")
	idleClaude(t, fake)

	if n, _ := d.Run("", "", 0); n != 1 {
		t.Errorf("a bad 5h threshold must not gate: %d dispatched", n)
	}
	if !strings.Contains(errb.String(), "plan_guard_5h") || !strings.Contains(errb.String(), "not a percent") {
		t.Errorf("a bad threshold must be reported, got: %q", errb.String())
	}
	if strings.Contains(errb.String(), "plan_guard_7d") {
		t.Errorf("the good threshold must not be complained about: %q", errb.String())
	}
}

// The seam still works: RHQ_PLAN_USAGE_URL redirects NewPlanReader, the
// beta header the endpoint requires is sent, and the response parses. What
// does NOT go with it is the account's credential — a loopback override is
// a host the caller named, so it is asked uncredentialed and the keychain
// is not read at all (credpin.go rule 4, ranger-base-dr6u). The token
// below is a fake and it still must not arrive.
func TestPlanReaderRequest(t *testing.T) {
	ps := newPlanServer(t, 42.4, 61.4)
	t.Setenv("RHQ_PLAN_USAGE_URL", ps.URL)
	r := NewPlanReader()
	if r.URL != ps.URL {
		t.Fatalf("RHQ_PLAN_USAGE_URL ignored: %s", r.URL)
	}
	r.Token = func() (string, error) {
		t.Error("the keychain was read for a caller-named endpoint")
		return fakeToken, nil
	}
	r.HTTP = ps.client

	u, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if u.FiveHour != 42.4 || u.SevenDay != 61.4 {
		t.Errorf("parsed %+v, want 42.4/61.4", u)
	}
	if ps.auth != "" {
		t.Errorf("an override was handed a credential: Authorization = %q", ps.auth)
	}
	if ps.beta != "oauth-2025-04-20" {
		t.Errorf("anthropic-beta header = %q", ps.beta)
	}
	// The cockpit header / posse cost line: percentages, no history.
	if got := u.Line(); got != "5h 42% · 7d 61%" {
		t.Errorf("Line() = %q, want %q", got, "5h 42% · 7d 61%")
	}
}

// The compiled-in endpoint is the one that is credentialed, and the one
// whose answer the fleet may believe. Nothing here dials: the assertion is
// about how NewPlanReader is configured with no override in the
// environment.
func TestPlanReaderCompiledInEndpointIsCredentialedAndShared(t *testing.T) {
	os.Unsetenv("RHQ_PLAN_USAGE_URL")
	r := NewPlanReader()
	if r.URL != PlanUsageURL {
		t.Fatalf("default URL = %s, want %s", r.URL, PlanUsageURL)
	}
	if !r.Shared {
		t.Error("the compiled-in endpoint's reading must still be the fleet's fact")
	}
	if !credentialedURL(r.URL, PlanUsageURL) {
		t.Error("the compiled-in endpoint must still be credentialed")
	}
}

// Default construction points at the real endpoint (no env override).
func TestPlanReaderDefaultURL(t *testing.T) {
	os.Unsetenv("RHQ_PLAN_USAGE_URL")
	if r := NewPlanReader(); r.URL != PlanUsageURL {
		t.Errorf("default URL = %s, want %s", r.URL, PlanUsageURL)
	}
}
