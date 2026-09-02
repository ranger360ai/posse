package posse

// Hermetic tests for the tier availability preflight (rangerhq-oay).
//
// Nothing here reads the operator's keychain or reaches the network. Two
// seams do it, and both already existed in shape: an injected ModelLister
// (App.ModelLister, the twin of Dispatcher.Plan) for the tests that want a
// catalog, and a seeded $StateDir/model-catalog.json for the tests that
// want the launch path to find one without a request at all — the pattern
// rangerhq-p3z established for the plan guard. A test that seeds the
// snapshot and leaves the lister unconfigured PROVES the reading came off
// the file: an unconfigured lister cannot answer.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// catalogServer is a fake /v1/models. It counts requests, records the
// headers it saw, and pages when asked to.
type catalogServer struct {
	*httptest.Server
	hits   atomic.Int64
	auth   string
	beta   string
	status int
	retry  string
	pages  [][]string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newCatalogServer(t *testing.T, pages ...[]string) *catalogServer {
	t.Helper()
	cs := &catalogServer{status: http.StatusOK, pages: pages}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.hits.Add(1)
		cs.auth = r.Header.Get("Authorization")
		cs.beta = r.Header.Get("anthropic-beta")
		if cs.retry != "" {
			w.Header().Set("Retry-After", cs.retry)
		}
		if cs.status != http.StatusOK {
			w.WriteHeader(cs.status)
			return
		}
		// Which page: the first, or the one after `after_id`.
		idx := 0
		if after := r.URL.Query().Get("after_id"); after != "" {
			for i, p := range cs.pages {
				if len(p) > 0 && p[len(p)-1] == after {
					idx = i + 1
				}
			}
		}
		if idx >= len(cs.pages) {
			fmt.Fprint(w, `{"data":[],"has_more":false}`)
			return
		}
		var items []string
		for _, id := range cs.pages[idx] {
			items = append(items, fmt.Sprintf(`{"type":"model","id":%q,"display_name":%q}`, id, id))
		}
		last := cs.pages[idx][len(cs.pages[idx])-1]
		fmt.Fprintf(w, `{"data":[%s],"has_more":%t,"last_id":%q}`,
			strings.Join(items, ","), idx+1 < len(cs.pages), last)
	}))
	t.Cleanup(cs.Close)
	return cs
}

func (cs *catalogServer) lister() *ModelLister {
	return &ModelLister{
		URL:   cs.URL,
		Token: func() (string, error) { return fakeToken, nil },
		HTTP:  cs.Client(),
	}
}

// seedCatalog writes the shared snapshot as if some earlier process had
// read it `age` ago. The launch path then finds it without asking anyone —
// which is the point: every preflight assertion below is about the
// resolution, not about HTTP.
func seedCatalog(t *testing.T, a *App, age time.Duration, ids ...string) {
	t.Helper()
	os.MkdirAll(a.StateDir, 0o755)
	b, err := json.Marshal(modelEntry{At: time.Now().Add(-age), Models: ids})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "model-catalog.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// preflightApp is an App with a state dir, a config path, and NO lister —
// so anything it learns, it learned from a seeded snapshot.
func preflightApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	return &App{
		Home:        home,
		ConfigPath:  filepath.Join(home, "config.yaml"),
		StateDir:    filepath.Join(home, "state"),
		AgentsDir:   filepath.Join(home, "agents"),
		ModelLister: &ModelLister{}, // unconfigured: cannot answer, must not ask
	}
}

func writeCfg(t *testing.T, a *App, body string) {
	t.Helper()
	if err := os.WriteFile(a.ConfigPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── the catalog reader ──────────────────────────────────────────────────────

func TestModelListerReadsTheCatalogAndPages(t *testing.T) {
	t.Parallel()
	cs := newCatalogServer(t, []string{"claude-fable-5", "claude-opus-5"}, []string{"claude-sonnet-5"})
	ids, err := cs.lister().List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"claude-fable-5", "claude-opus-5", "claude-sonnet-5"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Errorf("catalog = %v, want %v", ids, want)
	}
	if cs.hits.Load() != 2 {
		t.Errorf("two pages, %d requests", cs.hits.Load())
	}
	// The credential rides where an OAuth token rides, with the oauth beta
	// header beside it — the same pair planusage.go sends.
	if cs.auth != "Bearer "+fakeToken {
		t.Errorf("Authorization = %q", cs.auth)
	}
	if cs.beta != planBetaHeader {
		t.Errorf("anthropic-beta = %q, want %q", cs.beta, planBetaHeader)
	}
}

func TestModelListerErrorsNeverQuoteTheCredential(t *testing.T) {
	t.Parallel()
	cs := newCatalogServer(t, []string{"claude-opus-5"})
	cs.status = http.StatusUnauthorized
	_, err := cs.lister().List()
	if err == nil {
		t.Fatal("401 must be an error")
	}
	if strings.Contains(err.Error(), fakeToken) {
		t.Errorf("error quotes the token: %v", err)
	}
}

func TestModelCacheSharesOneReading(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	cs := newCatalogServer(t, []string{"claude-opus-5"})
	a.ModelLister = cs.lister()

	for i := 0; i < 3; i++ {
		ids, ok := a.ModelCache().Models(time.Hour)
		if !ok || len(ids) != 1 {
			t.Fatalf("read %d: %v %v", i, ids, ok)
		}
	}
	if cs.hits.Load() != 1 {
		t.Errorf("three reads through one snapshot made %d requests", cs.hits.Load())
	}
	// Past the age, one caller refreshes.
	if _, ok := a.ModelCache().Models(time.Nanosecond); !ok {
		t.Fatal("stale snapshot must be refreshed, not dropped")
	}
	if cs.hits.Load() != 2 {
		t.Errorf("a stale snapshot must cost exactly one refresh, got %d requests", cs.hits.Load())
	}
	log, err := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.log"))
	if err != nil {
		t.Fatalf("catalog requests must leave an observable log: %v", err)
	}
	if got := strings.Count(string(log), "preflight ok models=1"); got != 2 {
		t.Errorf("cache hits must not be logged as probe attempts; got %d real reads:\n%s", got, log)
	}
}

func TestModelCacheLogsAnUnreadableCatalogWithoutTheCredential(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	a.ModelLister = &ModelLister{
		// Loopback: the credentialed request is pinned to this machine or
		// the compiled-in host (credpin.go), and the fake transport below
		// is what actually answers.
		URL:   "https://127.0.0.1:9/v1/models",
		Token: func() (string, error) { return fakeToken, nil },
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})},
	}

	if _, ok := a.ModelCache().Models(time.Hour); ok {
		t.Fatal("a 401 is unknown, not a catalog")
	}
	log, err := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.log"))
	if err != nil {
		t.Fatalf("the failed read was silent: %v", err)
	}
	got := string(log)
	if !strings.Contains(got, "preflight failed: model list endpoint returned 401 Unauthorized") {
		t.Errorf("failed read not named:\n%s", got)
	}
	if strings.Contains(got, fakeToken) {
		t.Errorf("catalog log contains the credential: %s", got)
	}
}

func TestModelCacheHonoursRetryAfterAcrossProcesses(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	cs := newCatalogServer(t, []string{"claude-opus-5"})
	cs.status, cs.retry = http.StatusTooManyRequests, "600"
	a.ModelLister = cs.lister()

	if _, ok := a.ModelCache().Models(0); ok {
		t.Fatal("a 429 with no prior reading is not a catalog")
	}
	// A second process asking again is how a 429 storm gets extended
	// (rangerhq-tdy8): the cooldown is in the file, so it does not.
	if _, ok := a.ModelCache().Models(0); ok {
		t.Fatal("still no catalog")
	}
	if cs.hits.Load() != 1 {
		t.Errorf("Retry-After ignored: %d requests", cs.hits.Load())
	}
}

// ─── fail-open ───────────────────────────────────────────────────────────────

// The rule the whole file turns on: posse not knowing is not the same fact
// as the model being gone, and only the second one may move a launch.
func TestUnknownCatalogNeverDemotesAnything(t *testing.T) {
	t.Parallel()
	a := preflightApp(t) // no snapshot, unconfigured lister
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Fell() {
		t.Errorf("an unreadable catalog must launch the tier as asked, got %q", pf.Line)
	}
	if pf.Runtime != "claude" || pf.Tier != TierStrong || pf.Got != "claude-fable-5-1" {
		t.Errorf("pair moved: %+v", pf)
	}
}

func TestEmptyCatalogIsUnknownNotAnAccountWithNoModels(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	cs := newCatalogServer(t) // 200, zero entries
	a.ModelLister = cs.lister()
	if pf := a.TierPreflight("architect", "claude", TierStrong, nil); pf.Fell() {
		t.Errorf("an empty catalog must read as unknown, got %q", pf.Line)
	}
}

func TestPreflightOffSwitch(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	writeCfg(t, a, "model_preflight: false\n")
	seedCatalog(t, a, time.Minute, "claude-opus-5") // fable absent
	if pf := a.TierPreflight("architect", "claude", TierStrong, nil); pf.Fell() {
		t.Errorf("model_preflight: false must check nothing, got %q", pf.Line)
	}
}

func TestRuntimeWithNoModelMappingIsNotChecked(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	seedCatalog(t, a, time.Minute, "claude-opus-5")
	// The fixture declares api.anthropic.com and NO model_<tier>:, so
	// `p.Wanted == ""` is the only thing standing between it and the
	// catalog — the arm this test is about. It used to be grok, which stops
	// short one line later on `!OnModelCatalog` and since rangerhq-jp6 maps
	// every tier; keeping grok here would have left the empty-map arm
	// covered by nothing while the test stayed green off the other branch.
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "blankcli.yaml"),
		[]byte("command: blankcli {model} --sys {file}\negress: [api.anthropic.com]\n"), 0o644)
	rt, err := a.LoadRuntime("blankcli")
	if err != nil {
		t.Fatal(err)
	}
	if !rt.OnModelCatalog() || rt.Model(TierStrong) != "" {
		t.Fatalf("fixture must be on the catalog and map nothing: egress %v model %q", rt.Egress, rt.Model(TierStrong))
	}
	if pf := a.TierPreflight("security", "blankcli", TierStrong, nil); pf.Fell() {
		t.Errorf("a runtime with no per-tier model has nothing to check, got %q", pf.Line)
	}
	// The control: give the SAME fixture a model id the catalog does not
	// hold and the preflight must fall. Without this the assertion above is
	// satisfied by a preflight that checks nothing at all.
	os.WriteFile(filepath.Join(a.RuntimesDir(), "blankcli.yaml"),
		[]byte("command: blankcli {model} --sys {file}\negress: [api.anthropic.com]\nmodel_strong: claude-fable-5\n"), 0o644)
	if pf := a.TierPreflight("security", "blankcli", TierStrong, nil); !pf.Fell() {
		t.Errorf("control: a mapped id absent from the catalog must fall, got %q", pf.Line)
	}
}

// codex maps gpt-5.6-* since ranger-base-arm, and this catalog is
// Anthropic's: a mapped id off that API must NOT be reported missing just
// because a list that will never hold it does not hold it. The predicate
// is the runtime's egress: (Runtime.OnModelCatalog), not whether Models is empty.
func TestMappedNonAnthropicRuntimeIsNotCheckedAgainstTheAnthropicCatalog(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	seedCatalog(t, a, time.Minute, "claude-opus-5")
	for _, tier := range Tiers {
		if pf := a.TierPreflight("security", "codex", tier, nil); pf.Fell() {
			t.Errorf("codex/%s: no catalog posse can read, got %q", tier, pf.Line)
		}
	}
	line := a.PreflightReport("security", "codex", TierStrong, nil)
	if !strings.Contains(line, "gpt-5.6-sol") || !strings.Contains(line, "no model catalog") {
		t.Errorf("report must name the id AND why it went unchecked: %q", line)
	}
}

// ─── the three cases the bead named ──────────────────────────────────────────

func TestPreflightAvailableSaysNothing(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	seedCatalog(t, a, time.Minute, "claude-fable-5-1", "claude-opus-5", "claude-sonnet-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Fell() {
		t.Errorf("the model is on the account; nothing to say, got %q", pf.Line)
	}
	if pf.Tier != TierStrong || pf.Got != "claude-fable-5-1" {
		t.Errorf("%+v", pf)
	}
}

// The operator's own example line, verbatim in shape.
func TestPreflightFallsBackToOpusAndSaysSo(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	seedCatalog(t, a, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	const want = "architect: tier strong wants claude-fable-5-1 — unavailable, falling back to claude-opus-5"
	if pf.Line != want {
		t.Errorf("line =\n  %q\nwant\n  %q", pf.Line, want)
	}
	if pf.Runtime != "claude" || pf.Tier != TierStandard || pf.Got != "claude-opus-5" {
		t.Errorf("substituted pair = %+v", pf)
	}
}

// Rule (3): the launch is never refused over availability, so a fallback
// that is also gone still launches — loudly, on the best the map reached.
func TestPreflightFallbackAlsoUnavailableIsLoudAndStillLaunches(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	seedCatalog(t, a, time.Minute, "claude-sonnet-5") // neither fable nor opus
	pf := a.TierPreflight("developer", "claude", TierStrong, nil)
	for _, want := range []string{
		"developer: tier strong wants claude-fable-5-1 — unavailable",
		"claude-opus-5 — ALSO unavailable",
		"launching on claude-opus-5 anyway",
	} {
		if !strings.Contains(pf.Line, want) {
			t.Errorf("line %q missing %q", pf.Line, want)
		}
	}
	if pf.Tier != TierStandard || pf.Got != "claude-opus-5" {
		t.Errorf("must still land somewhere and say where: %+v", pf)
	}
}

// ─── the map ─────────────────────────────────────────────────────────────────

// rangerhq-u2p's requirement: the security lane's fallback may be a
// different RUNTIME, not a cheaper model on the same one.
func TestPerPersonaFallbackCanNameARuntimeHop(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	writeCfg(t, a, "tier_fallback:\n  security: codex\n")
	seedCatalog(t, a, time.Minute, "claude-opus-5")
	pf := a.TierPreflight("security", "claude", TierStrong, nil)
	if pf.Runtime != "codex" || pf.Tier != TierStrong {
		t.Errorf("security must hop runtimes and keep its tier: %+v", pf)
	}
	// The hop names the RUNTIME, which is the requirement. It reads
	// "<id> on codex" rather than "codex/strong" because codex maps a model
	// per tier since ranger-base-arm — hopDesc says "codex/strong (the
	// runtime\'s own default model)" only where nothing is mapped.
	if !strings.Contains(pf.Line, "falling back to gpt-5.6-sol on codex") {
		t.Errorf("line = %q", pf.Line)
	}
}

// The operator's rule is that EVERYONE falls back, so naming one persona
// must not silently switch the rest of the shop off — unlike tier_by_label,
// where a present key replaces the ADR default wholesale.
func TestNamingOnePersonaKeepsTheDefaultForEveryoneElse(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	writeCfg(t, a, "tier_fallback:\n  security: codex\n")
	seedCatalog(t, a, time.Minute, "claude-opus-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Tier != TierStandard || pf.Got != "claude-opus-5" {
		t.Errorf("the default strong→standard must survive another persona's entry: %+v", pf)
	}
}

func TestFallbackNoneMeansNoSubstitute(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	writeCfg(t, a, "tier_fallback:\n  strong: none\n")
	seedCatalog(t, a, time.Minute, "claude-opus-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Tier != TierStrong || pf.Got != "claude-fable-5-1" {
		t.Errorf("`none` must leave the launch where it was: %+v", pf)
	}
	if !strings.Contains(pf.Line, "says none") || !strings.Contains(pf.Line, "launching on claude-fable-5-1 anyway") {
		t.Errorf("line = %q", pf.Line)
	}
}

func TestFallbackValueThatIsNeitherATierNorARuntimeIsReportedNotObeyed(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	writeCfg(t, a, "tier_fallback:\n  strong: gpt-9\n")
	seedCatalog(t, a, time.Minute, "claude-opus-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Tier != TierStrong {
		t.Errorf("a typo must not move a launch: %+v", pf)
	}
	if !strings.Contains(pf.Line, `"gpt-9" is neither a tier nor a runtime`) {
		t.Errorf("a typo in the map must be named on the line the operator is already reading: %q", pf.Line)
	}
}

func TestFallbackCycleStops(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	writeCfg(t, a, "tier_fallback:\n  strong: standard\n  standard: strong\n")
	seedCatalog(t, a, time.Minute, "claude-sonnet-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if !strings.Contains(pf.Line, "loops back to") {
		t.Errorf("a cycle must be reported, not walked: %q", pf.Line)
	}
	if pf.Tier != TierStandard {
		t.Errorf("stops at the last hop it took: %+v", pf)
	}
}

// ─── the launch ──────────────────────────────────────────────────────────────

// pfPersona writes a PID on claude at `tier`, whose gates the wall fully
// realizes at shims.
func pfPersona(t *testing.T, b *HerdrBackend, name, tier string) {
	t.Helper()
	os.MkdirAll(b.App.AgentsDir, 0o755)
	body := fmt.Sprintf("---\nname: %s\ntier: %s\ndeny: [Bash(git push:*)]\n---\nYou are %s.\n", name, tier, name)
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// End to end over the fake CLI: the typed line names the substitute, the
// meta records both the tier that ran and the line saying it was not the
// one asked for, and `posse list` wears the mark.
func TestLaunchTypesTheSubstituteAndRecordsIt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	pfPersona(t, b, "architect", TierStrong)
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone

	if err := b.CreateSession(NewSessionOpts{Name: "r1", Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatalf("the preflight must never refuse a launch (rule 3): %v", err)
	}

	log := calls(t, fake)
	if !strings.Contains(log, "--model 'claude-opus-5'") {
		t.Errorf("the typed line must name the substitute:\n%s", log)
	}
	if strings.Contains(log, "claude-fable-5-1") {
		t.Errorf("the typed line still names the unavailable model:\n%s", log)
	}
	if !strings.Contains(warn.String(), "architect: tier strong wants claude-fable-5-1 — unavailable, falling back to claude-opus-5") {
		t.Errorf("the substitution was silent: %q", warn.String())
	}

	m, ok := b.readMeta("r1")
	if !ok {
		t.Fatal("no meta")
	}
	// tier: is what the session IS, the way cage: and degraded: are — and
	// fallback: is the line that says it is not what was asked for.
	if m.Tier != TierStandard {
		t.Errorf("meta tier = %q, want the tier that actually launched", m.Tier)
	}
	if !strings.Contains(m.Fallback, "wants claude-fable-5-1") {
		t.Errorf("meta fallback = %q", m.Fallback)
	}

	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list.String(), FallbackTag) {
		t.Errorf("posse list does not show the fallback:\n%s", list.String())
	}
	if !strings.Contains(list.String(), b.App.RuntimeTierTag("claude", TierStandard)) {
		t.Errorf("posse list must name the tier that is really running:\n%s", list.String())
	}
}

func TestLaunchOnAnAvailableModelRecordsNoFallback(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	pfPersona(t, b, "architect", TierStrong)
	seedCatalog(t, b.App, time.Minute, "claude-fable-5-1", "claude-opus-5")

	if err := b.CreateSession(NewSessionOpts{Name: "r2", Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, fake); !strings.Contains(log, "--model 'claude-fable-5-1'") {
		t.Errorf("available means launch it:\n%s", log)
	}
	if m, _ := b.readMeta("r2"); m.Fallback != "" || m.Tier != TierStrong {
		t.Errorf("nothing happened, so nothing is recorded: %+v", m)
	}
}

// A launch through the test backend reaches no catalog at all — the
// hermetic default. It must still launch, at the tier it was asked for.
func TestLaunchWithNoCatalogIsUnchanged(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	pfPersona(t, b, "architect", TierStrong)
	if err := b.CreateSession(NewSessionOpts{Name: "r3", Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, fake); !strings.Contains(log, "--model 'claude-fable-5-1'") {
		t.Errorf("unknown launches as asked:\n%s", log)
	}
	if m, _ := b.readMeta("r3"); m.Fallback != "" {
		t.Errorf("fallback recorded with no catalog to justify it: %q", m.Fallback)
	}
}

// ─── the cost half ───────────────────────────────────────────────────────────

// The bead's NOTE: per-tier dollars are wrong when a fallback happens
// silently. They are right when it does not happen silently, and this is
// why — `posse cost` reads the model out of the transcript and names the
// tier from THAT, never from the PID or the session meta. A regression
// here would put a fallback session's spend back in the strong row.
func TestCostNamesTheTierOfTheModelThatActuallyRan(t *testing.T) {
	t.Parallel()
	for model, want := range map[string]string{
		"claude-fable-5":  TierStrong,
		"claude-opus-5":   TierStandard,
		"claude-sonnet-5": TierFast,
	} {
		if got := TierForModel(model); got != want {
			t.Errorf("TierForModel(%s) = %s, want %s", model, got, want)
		}
	}
	// A strong-tier session that fell back to opus is opus spend, in the
	// standard row, with the meta's fallback: line explaining the row.
	s := &Segment{Bead: "rangerhq-oay", Model: "claude-opus-5"}
	if TierForModel(s.Model) != TierStandard {
		t.Errorf("a fallback session's spend must be counted as the model that did the work")
	}
}

// ─── the operator's own window on it ─────────────────────────────────────────

// `posse gates <persona>` is the only place the operator can see this
// without launching, so what it says matters as much as what it does.
func TestPreflightReportSaysWhichOfTheThreeItIs(t *testing.T) {
	t.Parallel()
	t.Run("available", func(t *testing.T) {
		a := preflightApp(t)
		seedCatalog(t, a, time.Minute, "claude-fable-5-1")
		if got := a.PreflightReport("architect", "claude", TierStrong, nil); got != "claude: tier strong → claude-fable-5-1 (available)" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		a := preflightApp(t) // no snapshot, unconfigured lister
		got := a.PreflightReport("architect", "claude", TierStrong, nil)
		if !strings.Contains(got, "UNKNOWN") || !strings.Contains(got, "takes the tier as asked") {
			t.Errorf("unknown must be visibly different from unavailable: %q", got)
		}
	})
	t.Run("unavailable is the launch's own line", func(t *testing.T) {
		a := preflightApp(t)
		seedCatalog(t, a, time.Minute, "claude-opus-5")
		// One rendering: what `posse gates` shows and what a launch prints
		// are the same bytes, so neither can drift from the other.
		want := a.TierPreflight("architect", "claude", TierStrong, nil).Line
		if got := a.PreflightReport("architect", "claude", TierStrong, nil); got != want {
			t.Errorf("gates line %q != launch line %q", got, want)
		}
	})
	t.Run("no mapping", func(t *testing.T) {
		a := preflightApp(t)
		seedCatalog(t, a, time.Minute, "claude-opus-5")
		// No built-in maps nothing any more — codex since ranger-base-arm,
		// grok since rangerhq-jp6 — so the runtime with nothing to
		// preflight is a declared one that sets no model_<tier>:.
		os.MkdirAll(a.RuntimesDir(), 0o755)
		os.WriteFile(filepath.Join(a.RuntimesDir(), "blankcli.yaml"),
			[]byte("command: blankcli {model} --sys {file}\n"), 0o644)
		if got := a.PreflightReport("security", "blankcli", TierStrong, nil); !strings.Contains(got, "maps no model") {
			t.Errorf("got %q", got)
		}
	})
	t.Run("mapped but off the catalog", func(t *testing.T) {
		a := preflightApp(t)
		seedCatalog(t, a, time.Minute, "claude-opus-5")
		// grok maps ids the Anthropic catalog will never hold, so the
		// report names the id AND says why it was not checked — the branch
		// codex has held alone until now.
		got := a.PreflightReport("security", "grok", TierStrong, nil)
		if !strings.Contains(got, "grok-4.6") || !strings.Contains(got, "no model catalog posse can read") {
			t.Errorf("got %q", got)
		}
	})
}

// ─── the age of the reading a verdict rests on (ADR 0039 D3a) ────────────────

// seedCatalogEntry writes the snapshot verbatim, for the two facts
// seedCatalog cannot express: a cooldown, and a reading that is retained
// but past its lease.
func seedCatalogEntry(t *testing.T, a *App, e modelEntry) {
	t.Helper()
	os.MkdirAll(a.StateDir, 0o755)
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "model-catalog.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// failingLister is the probe this instance has actually had since
// 2026-08-31: a 401 that leaves the retained reading ruling on every
// launch. It counts its calls, because "how many times was the endpoint
// asked" is half of what D3b is about.
func failingLister(hits *atomic.Int64) *ModelLister {
	return &ModelLister{
		URL:   "https://127.0.0.1:9/v1/models",
		Token: func() (string, error) { return fakeToken, nil },
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			hits.Add(1)
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})},
	}
}

// The bead that produced D3a and the ruling that produced D3c: strong moved
// to an id the retained reading was taken before, the probe cannot refresh
// it, and "unavailable" was a true sentence that taught the operator the
// wrong thing — they edited a state file by hand. Past its lease that
// reading no longer reaches a verdict at all: it is QUOTED, with its age and
// the probe's outcome, so what the line teaches is "refresh a credential",
// and the launch takes the tier it was asked for.
func TestVerdictNamesTheAgeOfTheReadingAndTheProbeOutcome(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	a := preflightApp(t)
	a.ModelLister = failingLister(&hits)
	seedCatalogEntry(t, a, modelEntry{At: time.Now().Add(-48 * time.Hour), Models: []string{"claude-opus-5", "claude-sonnet-5"}})

	// The launch's own loud line, in the ADR's shape (0039 D3c): what is
	// absent, from a reading named and dated, then the verdict.
	const want = "architect: tier strong wants claude-fable-5-1 — not in the catalog read 48h00m ago " +
		"and the probe is failing (model list endpoint returned 401 Unauthorized); availability UNKNOWN, launching as asked"
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Line != want {
		t.Errorf("line =\n  %q\nwant\n  %q", pf.Line, want)
	}
	// It prints, and nothing fell: the pair is the one that was asked for,
	// so the launch renders --model claude-fable-5-1 and the session meta
	// gets no `fallback:` mark to carry into a relaunch.
	if pf.Fell() || !pf.Unknown || pf.Tier != TierStrong || pf.Got != "claude-fable-5-1" {
		t.Errorf("an UNKNOWN verdict must move nothing: %+v", pf)
	}
	// The error is ModelLister's own generic string and nothing else: a
	// clause printed in front of an operator is one more place the
	// credential must not appear.
	if strings.Contains(pf.Line, fakeToken) {
		t.Errorf("the age clause quotes the credential: %q", pf.Line)
	}

	// The id the reading DOES list is not "available" either — the same
	// reading is past the same lease, and the report says so rather than
	// reporting a verdict posse is no longer entitled to.
	got := a.PreflightReport("", "claude", TierStandard, nil)
	if !strings.Contains(got, "(availability UNKNOWN — the catalog read 48h00m ago is past model_probe_ttl and the probe is failing (model list endpoint returned 401 Unauthorized); the launch takes the tier as asked)") {
		t.Errorf("a listed id on an expired reading is not available: %q", got)
	}
}

// The control on the clause, and the rangerhq-oay case D3c had to preserve:
// inside model_probe_ttl a reading still RULES, an absent id still demotes,
// and the line is the byte-for-byte one the shop has been reading since that
// bead. The lease bounds the old rule; it does not replace it.
func TestAReadingInsideItsLeaseCarriesNoAgeClause(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	a := preflightApp(t)
	a.ModelLister = failingLister(&hits)
	seedCatalog(t, a, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone, read a minute ago

	const want = "architect: tier strong wants claude-fable-5-1 — unavailable, falling back to claude-opus-5"
	if got := a.TierPreflight("architect", "claude", TierStrong, nil).Line; got != want {
		t.Errorf("line =\n  %q\nwant\n  %q", got, want)
	}
	if got := a.PreflightReport("", "claude", TierStandard, nil); got != "claude: tier standard → claude-opus-5 (available)" {
		t.Errorf("got %q", got)
	}
	if hits.Load() != 0 {
		t.Errorf("a reading inside its lease must ask nobody, %d requests", hits.Load())
	}
	// The same fixture, aged past the lease, stops demoting and starts
	// dating itself — the arm that proves the assertions above are
	// measuring the age of the reading and not something that never moves.
	seedCatalog(t, a, 48*time.Hour, "claude-opus-5", "claude-sonnet-5")
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Fell() || pf.Tier != TierStrong {
		t.Errorf("control: past the lease nothing may be substituted: %+v", pf)
	}
	if !strings.Contains(pf.Line, "not in the catalog read 48h00m ago") || !strings.Contains(pf.Line, "availability UNKNOWN") {
		t.Errorf("control: past the lease the reading must be quoted and dated: %q", pf.Line)
	}
}

// ─── one reading per report (ADR 0039 D3b) ───────────────────────────────────

// `posse runtimes` prints a line per mapped tier per runtime on this API.
// A reading taken per line is a request per line the moment the snapshot
// cannot be refreshed — a failed read stores nothing, so nothing shares
// it — and one state/model-catalog.log line for each.
func TestOneCatalogReadingServesEveryLineOfAReport(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	a := preflightApp(t)
	a.ModelLister = failingLister(&hits)
	seedCatalogEntry(t, a, modelEntry{At: time.Now().Add(-48 * time.Hour), Models: []string{"claude-opus-5", "claude-sonnet-5"}})

	cat := a.ProbeCatalog(nil)
	for _, tier := range Tiers {
		if line := a.PreflightReportOn(cat, "", "claude", tier); line == "" {
			t.Fatalf("no line for tier %s", tier)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("--probe over 3 tiers made %d requests, want 1", hits.Load())
	}
	log, err := os.ReadFile(filepath.Join(a.StateDir, "model-catalog.log"))
	if err != nil {
		t.Fatalf("the forced read left no log line: %v", err)
	}
	if got := strings.Count(string(log), "\n"); got != 1 {
		t.Errorf("--probe wrote %d model-catalog.log lines, want 1:\n%s", got, log)
	}

	// The arm that proves the count above is measuring the sharing: the
	// per-call form, which is what every one of these lines used to be,
	// asks once per line over the same fixture.
	before := hits.Load()
	for _, tier := range Tiers {
		a.PreflightReport("", "claude", tier, nil)
	}
	if n := hits.Load() - before; n < 3 {
		t.Errorf("control: a reading per line must cost a request per line, got %d for 3 lines", n)
	}
}

// A forced read is still not allowed to become the rangerhq-tdy8 storm:
// Models checks RetryAt before it asks, and --probe goes through Models.
func TestProbeHonoursALiveCooldown(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	a := preflightApp(t)
	a.ModelLister = failingLister(&hits)
	seedCatalogEntry(t, a, modelEntry{
		At:      time.Now().Add(-48 * time.Hour),
		Models:  []string{"claude-opus-5", "claude-sonnet-5"},
		RetryAt: time.Now().Add(10 * time.Minute),
	})

	cat := a.ProbeCatalog(nil)
	line := a.PreflightReportOn(cat, "architect", "claude", TierStrong)
	if hits.Load() != 0 {
		t.Errorf("--probe asked a cooling-down endpoint %d times", hits.Load())
	}
	// A cooldown says "do not ask again yet", never "the answer is still
	// good" (ADR 0039 D3c, and the alternative it rejects by name): the
	// reading is 48h old, so it is quoted and dated and rules on nothing.
	if !strings.Contains(line, "not in the catalog read 48h00m ago") || !strings.Contains(line, "availability UNKNOWN") {
		t.Errorf("a cooldown must not renew trust in a reading past its lease: %q", line)
	}
	// Nothing was asked, so there is no probe outcome to report: the age
	// is the whole clause, and the 429 that set the cooldown is in the log.
	if strings.Contains(line, "the probe is failing") {
		t.Errorf("a read that never happened must not be reported as a failing probe: %q", line)
	}
	// The arm that proves the line above is measuring the LEASE and not the
	// cooldown: the same cooldown over a reading INSIDE model_probe_ttl
	// still rules, and still demotes. --probe asked for maxAge 0 and was
	// refused; how long a reading may rule is the operator's number, not
	// the caller's, so the report keeps printing what a launch would do.
	seedCatalogEntry(t, a, modelEntry{
		At:      time.Now().Add(-30 * time.Minute),
		Models:  []string{"claude-opus-5", "claude-sonnet-5"},
		RetryAt: time.Now().Add(10 * time.Minute),
	})
	if got := a.PreflightReportOn(a.ProbeCatalog(nil), "architect", "claude", TierStrong); got != "architect: tier strong wants claude-fable-5-1 — unavailable, falling back to claude-opus-5" {
		t.Errorf("a cooled-down reading inside its lease must still rule: %q", got)
	}
	// The control: with the cooldown expired, the same fixture DOES ask.
	seedCatalogEntry(t, a, modelEntry{
		At:      time.Now().Add(-48 * time.Hour),
		Models:  []string{"claude-opus-5", "claude-sonnet-5"},
		RetryAt: time.Now().Add(-time.Minute),
	})
	a.ProbeCatalog(nil).known()
	if hits.Load() != 1 {
		t.Errorf("control: past the cooldown --probe must ask, %d requests", hits.Load())
	}
}
