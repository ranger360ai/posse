package posse

// The catalog probe's credential (ADR 0039 D3d, ranger-base-mvrke).
//
// Everything here is hermetic in the two ways this package's tests already
// are: the endpoint is an httptest server the lister is handed, and the
// session credential is an env set FILE under a scratch home — the store of
// record ADR 0019 D1 names and the one the seam reads (ADR 0039 D3d as
// amended, ranger-base-q3n4e retracted the process-environment arm). So the
// seam under test is the real one and not a stand-in for it. The meter half
// is a fake token func; a test that read the operator's keychain would be a
// different kind of test and this file has none.
//
// Every arm below ALSO puts a value in the test process's environment under
// the credential's real name — one that must never be presented. Without it
// the absence arms would pass against a seam that still read `os.Getenv` and
// merely found nothing there.
//
// No assertion below prints a credential. They compare, and on failure they
// name which credential was expected — session or meter — because a test
// that leaks the value it was checking is worse than the bug it catches.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const (
	// Both are fakes, and neither is ever printed. The shapes differ only
	// so a mismatch is unambiguous to the code, never to a reader of a log.
	fakeSessionMint = "sk-ant-oat01-fake-session-do-not-log-me"
	fakeMeterMint   = "sk-ant-fake-meter-do-not-log-me"
)

// bearerLog is a fake /v1/models that records the credential every request
// presented and can refuse a fixed number of leading requests. Two questions
// this file asks that catalogServer cannot answer: WHICH credential was on
// each request, and HOW MANY requests there were.
type bearerLog struct {
	*httptest.Server
	mu      sync.Mutex
	bearers []string
	refuse  int // this many leading requests answer refuseCode
	code    int // the refusal status; 0 = 401
}

func newBearerLog(t *testing.T, ids ...string) *bearerLog {
	t.Helper()
	b := &bearerLog{}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		n := len(b.bearers)
		b.bearers = append(b.bearers, r.Header.Get("Authorization"))
		refuse, code := b.refuse, b.code
		b.mu.Unlock()
		if n < refuse {
			if code == 0 {
				code = http.StatusUnauthorized
			}
			w.WriteHeader(code)
			return
		}
		var items []string
		for _, id := range ids {
			items = append(items, fmt.Sprintf(`{"type":"model","id":%q}`, id))
		}
		fmt.Fprintf(w, `{"data":[%s],"has_more":false}`, strings.Join(items, ","))
	}))
	t.Cleanup(b.Close)
	return b
}

// presented is the credentials the endpoint saw, in order, as the NAMES this
// file gives them — never the values. An unrecognised bearer reads as
// "other" rather than as itself, so a failure message stays safe to paste.
func (b *bearerLog) presented() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, got := range b.bearers {
		switch got {
		case "Bearer " + fakeSessionMint:
			out = append(out, "session")
		case "Bearer " + fakeMeterMint:
			out = append(out, "meter")
		case "":
			out = append(out, "none")
		default:
			out = append(out, "other")
		}
	}
	return out
}

// fixedToken is a credential source that answers, standing in for the meter
// store — which this file never reads for real.
func fixedToken(tok string) func() (string, CredMeta, error) {
	return func() (string, CredMeta, error) {
		return tok, CredMeta{Source: "a fake meter store"}, nil
	}
}

// sessionCredApp is an App whose home holds no runtimes overlay, so
// LoadRuntime("claude") is the built-in and CageCredential names the
// variable the launch names. It has an envs/ because that is now where the
// session half looks.
func sessionCredApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	return &App{Home: home, StateDir: home + "/state",
		EnvsDir: filepath.Join(home, "envs"), ConfigPath: filepath.Join(home, "config.yaml")}
}

// haveSessionMint puts the mint where the launch would: in an env set under
// the home, named as this persona-less home's `default_env` so the seam's
// persona-less list reaches it. The environment gets the poison value.
func haveSessionMint(t *testing.T, a *App, rt *Runtime) {
	t.Helper()
	t.Setenv(CageCredential(rt), envArmPoison)
	sessionSet(t, a, "default", CageCredential(rt), fakeSessionMint)
	namedDefaultEnv(t, a, "default")
}

// noSessionMint is the absence arm: no set under this home carries the name,
// which is the only way the session credential is absent now — and the
// environment holds a value anyway, which must change nothing.
func noSessionMint(t *testing.T, a *App, rt *Runtime) {
	t.Helper()
	t.Setenv(CageCredential(rt), envArmPoison)
	sessionSet(t, a, "default", "SOMETHING_ELSE", "x")
	namedDefaultEnv(t, a, "default")
}

// The preference itself: with the mint in the env set this launch names,
// that is the credential the catalog endpoint sees — not the meter store's,
// which is sitting right there answering, and not the process environment's,
// which is holding a different value the whole time.
func TestCatalogProbePresentsTheSessionCredential(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	haveSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1", "claude-opus-5")
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	ids, err := l.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "claude-fable-5-1" {
		t.Errorf("catalog = %v, want the two ids the endpoint served", ids)
	}
	if got := srv.presented(); len(got) != 1 || got[0] != "session" {
		t.Errorf("credentials presented = %v, want [session]", got)
	}
}

// And it is the VALUE, not merely the shape: the bearer the endpoint reads
// is byte-for-byte the mint the env set carries. Asserted inside the
// handler's own record and reported as a bool, so neither arm of this test
// can put a credential in a test log.
func TestCatalogProbeSendsTheEnvSetValueItself(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	haveSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	if _, err := l.List(); err != nil {
		t.Fatal(err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.bearers) != 1 {
		t.Fatalf("requests = %d, want 1", len(srv.bearers))
	}
	if srv.bearers[0] != "Bearer "+fakeSessionMint {
		t.Error("the Authorization header did not carry the env-set mint exactly")
	}
}

// Absence, the first of the two ways the preference does not answer: no env
// set this launch names carries the variable, which is what
// readSessionCredential now reports as absence — the environment carrying it
// is not an answer. The meter store answers instead, and the endpoint is
// asked exactly once: nothing was asked before the fallback, so it costs no
// request.
func TestNoSessionCredentialReadsTheMeterStore(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	noSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	ids, err := l.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Errorf("catalog = %v, want the one id the endpoint served", ids)
	}
	if got := srv.presented(); len(got) != 1 || got[0] != "meter" {
		t.Errorf("credentials presented = %v, want [meter]", got)
	}
}

// A lister with no fallback at all — the bare constructor, and every lister
// a test injects a Token into — is unchanged by D3d: the absent credential
// is the error it has always been, and nothing is asked.
func TestNoFallbackKeepsTheCredentialErrorAndAsksNobody(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	noSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), HTTP: srv.Client()}
	if _, err := l.List(); err == nil {
		t.Fatal("a lister with no fallback answered without a credential")
	}
	if got := srv.presented(); len(got) != 0 {
		t.Errorf("credentials presented = %v, want none — the endpoint must not be asked", got)
	}
}

// Refusal, the second way: the session credential was there, it was
// presented, and the endpoint said 401. The meter store is presented ONCE
// more and the read completes. Two requests, in that order — a third would
// be the retry storm this bound exists for (rangerhq-tdy8).
func TestRefusedSessionCredentialFallsThroughToTheMeterStoreOnce(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	haveSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	srv.refuse = 1
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	ids, err := l.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "claude-fable-5-1" {
		t.Errorf("catalog = %v, want the id the endpoint served the second credential", ids)
	}
	if got := srv.presented(); len(got) != 2 || got[0] != "session" || got[1] != "meter" {
		t.Errorf("credentials presented = %v, want [session meter]", got)
	}
}

// The same fall-through when the endpoint refuses both: still exactly two
// requests, and the answer is the endpoint's refusal. "Once" is a bound on
// the reads, not a promise that one of them works.
func TestBothCredentialsRefusedStopsAtTwoRequests(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	haveSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	srv.refuse = 9
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	if _, err := l.List(); err == nil {
		t.Fatal("two refused credentials answered as a catalog")
	} else if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want the endpoint's own refusal", err)
	}
	if got := srv.presented(); len(got) != 2 {
		t.Errorf("credentials presented = %v, want two — one per credential and no more", got)
	}
}

// And the fall-through does not fire twice over one read: when there was no
// session credential, the meter store is what was ALREADY presented, so a
// 401 on it must not re-present the same bytes to the same endpoint. That
// duplicate is precisely the traffic the bound is about.
func TestRefusedFallbackIsNotPresentedTwice(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	noSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	srv.refuse = 9
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	if _, err := l.List(); err == nil {
		t.Fatal("a refused credential answered as a catalog")
	}
	if got := srv.presented(); len(got) != 1 || got[0] != "meter" {
		t.Errorf("credentials presented = %v, want [meter] — the same credential must not be asked twice", got)
	}
}

// 403 is the same class and the same move: a credential that was never
// entitled is not one to keep presenting either (ADR 0019 D2).
func TestForbiddenSessionCredentialAlsoFallsThrough(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	haveSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	srv.refuse, srv.code = 1, http.StatusForbidden
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	if _, err := l.List(); err != nil {
		t.Fatal(err)
	}
	if got := srv.presented(); len(got) != 2 || got[1] != "meter" {
		t.Errorf("credentials presented = %v, want [session meter]", got)
	}
}

// A refusal that is NOT a credential class stays one request: 500 is the
// endpoint having a bad day, and presenting a second credential to it buys
// nothing and doubles the traffic.
func TestServerErrorDoesNotSpendTheSecondCredential(t *testing.T) {
	a := sessionCredApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	haveSessionMint(t, a, rt)

	srv := newBearerLog(t, "claude-fable-5-1")
	srv.refuse, srv.code = 1, http.StatusInternalServerError
	l := &ModelLister{URL: srv.URL, Token: a.sessionCatalogToken(), Fallback: fixedToken(fakeMeterMint), HTTP: srv.Client()}
	if _, err := l.List(); err == nil {
		t.Fatal("a 500 answered as a catalog")
	}
	if got := srv.presented(); len(got) != 1 || got[0] != "session" {
		t.Errorf("credentials presented = %v, want [session] — a 500 is not a credential verdict", got)
	}
}

// credSource is where a credential func's BODY was written: the file and
// line of the closure literal itself. That is the identity this file needs
// and a code pointer is not — the compiler specialises an inlined closure
// per call site, so two `MeterToken(...)` values built from one literal
// compare unequal by reflect.Pointer while reporting the same source
// position (MEASURED, go1.26.5). Building a credential func reads no store;
// only calling one does, and nothing here calls the meter half.
func credSource(f func() (string, CredMeta, error)) string {
	pc := reflect.ValueOf(f).Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	file, line := fn.FileLine(pc)
	return fmt.Sprintf("%s:%d", file, line)
}

// The wiring: the App-constructed path prefers the session credential and
// keeps the bare constructor's meter store behind it. Asked of the funcs
// that were wired rather than of their answers, because asking the meter
// half for an answer means reading the operator's keychain, and the claim
// here is about which acquisition path each field got.
func TestModelCacheWiresSessionFirstAndTheMeterStoreBehindIt(t *testing.T) {
	a := sessionCredApp(t)
	l := a.ModelCache().Lister
	if l.Token == nil || l.Fallback == nil {
		t.Fatal("the App path did not wire both credentials")
	}
	meter := credSource(MeterToken(modelCatalogRuntime))
	if got := credSource(l.Fallback); got != meter {
		t.Errorf("the fallback is not the meter store's reader: %s, want %s", got, meter)
	}
	if credSource(l.Token) == meter {
		t.Error("the App path still presents the meter store first")
	}
	// And the preference really is the session half — the same seam the
	// pins above drove, asked through the field a launch will use.
	rt, err := a.LoadRuntime(modelCatalogRuntime)
	if err != nil {
		t.Fatal(err)
	}
	haveSessionMint(t, a, rt)
	tok, _, err := l.Token()
	if err != nil {
		t.Fatal(err)
	}
	if tok != fakeSessionMint {
		t.Error("the App path's first credential is not the session mint in this environment")
	}
}

// The control for the pin above: the bare constructor's own Token reports
// that same source, so "is the meter store's reader" is a check that can
// come out the other way rather than a position every func shares.
func TestCredSourceTellsTheTwoCredentialsApart(t *testing.T) {
	t.Parallel()
	a := sessionCredApp(t)
	meter := credSource(MeterToken(modelCatalogRuntime))
	if got := credSource(NewModelLister().Token); got != meter {
		t.Errorf("the bare constructor's credential = %s, want the meter store's reader %s", got, meter)
	}
	if credSource(a.sessionCatalogToken()) == meter {
		t.Error("credSource cannot tell the session reader from the meter one")
	}
}

// And the bare constructor is untouched: the meter store, one credential,
// no fallback. Every test in this package that injects a Token relies on
// that, which is why D3d added a field instead of changing this one.
func TestBareModelListerStaysOnTheMeterStore(t *testing.T) {
	t.Parallel()
	l := NewModelLister()
	if l.Fallback != nil {
		t.Error("the bare constructor grew a second credential")
	}
	if l.Token == nil {
		t.Error("the bare constructor lost its credential")
	}
}

// An injected lister is left exactly as injected — the seam the preflight's
// hermetic tests are built on. The App path may only fill in what it built.
func TestInjectedListerIsNotRewired(t *testing.T) {
	t.Parallel()
	a := sessionCredApp(t)
	mine := &ModelLister{URL: "https://example.invalid/v1/models", Token: fixedToken(fakeMeterMint)}
	a.ModelLister = mine
	l := a.ModelCache().Lister
	if l != mine {
		t.Fatal("the App path replaced the injected lister")
	}
	if l.Fallback != nil {
		t.Error("the App path added a credential to an injected lister")
	}
	if credSource(l.Token) != credSource(mine.Token) {
		t.Error("the App path moved the injected lister's credential")
	}
}
