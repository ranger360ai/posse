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
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
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
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
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
// as the model being gone. Since ADR 0003 §3 neither one moves a launch —
// what separates them now is whether anything is SAID, and an unreadable
// catalog says nothing at all.
func TestUnknownCatalogSaysNothing(t *testing.T) {
	t.Parallel()
	a := preflightApp(t) // no snapshot, unconfigured lister
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Line != "" {
		t.Errorf("an unreadable catalog must launch the tier as asked in silence, got %q", pf.Line)
	}
	if pf.Wanted != "claude-fable-5-1" {
		t.Errorf("the ask is still the ask: %+v", pf)
	}
}

func TestEmptyCatalogIsUnknownNotAnAccountWithNoModels(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	cs := newCatalogServer(t) // 200, zero entries
	a.ModelLister = cs.lister()
	if pf := a.TierPreflight("architect", "claude", TierStrong, nil); pf.Line != "" {
		t.Errorf("an empty catalog must read as unknown, got %q", pf.Line)
	}
}

func TestPreflightOffSwitch(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	writeCfg(t, a, "model_preflight: false\n")
	seedCatalog(t, a, time.Minute, "claude-opus-5") // fable absent
	if pf := a.TierPreflight("architect", "claude", TierStrong, nil); pf.Line != "" {
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
	if pf := a.TierPreflight("security", "blankcli", TierStrong, nil); pf.Line != "" {
		t.Errorf("a runtime with no per-tier model has nothing to check, got %q", pf.Line)
	}
	// The control: give the SAME fixture a model id the catalog does not
	// hold and the preflight must speak. Without this the assertion above is
	// satisfied by a preflight that checks nothing at all.
	os.WriteFile(filepath.Join(a.RuntimesDir(), "blankcli.yaml"),
		[]byte("command: blankcli {model} --sys {file}\negress: [api.anthropic.com]\nmodel_strong: claude-fable-5\n"), 0o644)
	if pf := a.TierPreflight("security", "blankcli", TierStrong, nil); pf.Line == "" {
		t.Error("control: a mapped id absent from the catalog must be reported")
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
		if pf := a.TierPreflight("security", "codex", tier, nil); pf.Line != "" {
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
	if pf.Line != "" {
		t.Errorf("the model is on the account; nothing to say, got %q", pf.Line)
	}
	if pf.Wanted != "claude-fable-5-1" {
		t.Errorf("%+v", pf)
	}
}

// What the operator's own example line became when ADR 0003 §3 struck dial
// H: the same opening clause, and then nothing about a substitute, because
// there is not one.
func TestPreflightUnavailableSaysSoAndNamesNoSubstitute(t *testing.T) {
	t.Parallel()
	a := preflightApp(t)
	seedCatalog(t, a, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if !strings.HasPrefix(pf.Line, "architect: tier strong wants claude-fable-5-1 — unavailable on this account") {
		t.Errorf("line = %q", pf.Line)
	}
	if !strings.Contains(pf.Line, "launching as asked") {
		t.Errorf("the line must say the launch goes ahead: %q", pf.Line)
	}
	// The words the removed walk rendered. Their absence is the assertion:
	// a line that still offers a substitute is a walk that came back.
	for _, gone := range []string{"falling back to", "ALSO unavailable", "claude-opus-5"} {
		if strings.Contains(pf.Line, gone) {
			t.Errorf("line names a substitute (%q): %q", gone, pf.Line)
		}
	}
	if pf.Wanted != "claude-fable-5-1" {
		t.Errorf("the ask is unmoved: %+v", pf)
	}
}

// ─── the map that is no longer read ──────────────────────────────────────────

// `tier_fallback:` is gone (ADR 0003 §3). It is not enough that posse stops
// walking it: an operator's config still HOLDS the key, on this instance and
// on every other one that configured it, and the removal is only honest if
// that config now changes nothing.
//
// Each row is a shape the walk used to obey and a model id it used to reach.
// The assertion is the same for all of them — the line names the asked-for
// id and no other, so nothing was substituted — and it is checked against
// the LAUNCH, not only against the preflight, because the walk's whole
// product was a pair the launch then opened on.
func TestTierFallbackConfigIsInertInEveryShapeItUsedToObey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, cfg string
	}{
		{"persona names a runtime hop", "tier_fallback:\n  architect: codex\n"},
		{"tier names a cheaper tier", "tier_fallback:\n  strong: standard\n"},
		{"the explicit none", "tier_fallback:\n  strong: none\n"},
		{"a cycle", "tier_fallback:\n  strong: standard\n  standard: strong\n"},
		{"a value that is neither", "tier_fallback:\n  strong: gpt-9\n"},
		{"no key at all — the removed default strong→standard", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			var warn strings.Builder
			b.Warn = &warn
			pfPersona(t, b, "architect", TierStrong)
			if tc.cfg != "" {
				writeCfg(t, b.App, tc.cfg)
			}
			seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone
			if err := b.CreateSession(NewSessionOpts{Name: "tf", Agent: "architect", Dir: t.TempDir()}); err != nil {
				t.Fatalf("availability never refuses a launch: %v", err)
			}
			log := launchLog(t, b.App, fake)
			if !strings.Contains(log, "--model 'claude-fable-5-1'") {
				t.Errorf("the launch did not open on the asked-for model:\n%s", log)
			}
			for _, sub := range []string{"claude-opus-5", "claude-sonnet-5", "gpt-5.6"} {
				if strings.Contains(log, sub) {
					t.Errorf("%s was substituted onto the launch line:\n%s", sub, log)
				}
			}
			if m, _ := b.readMeta("tf"); m.Runtime != "claude" || m.Tier != TierStrong {
				t.Errorf("the session records a pair it was not asked for: %+v", m)
			}
			if !strings.Contains(warn.String(), "unavailable on this account") {
				t.Errorf("unavailable must still be loud: %q", warn.String())
			}
		})
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

// End to end over the fake CLI: the typed line names the model the tier
// asked for, the meta records the pair the launch really opened on, the
// operator hears that it is unavailable — and `posse list` wears no mark,
// because there is no longer a second fact for a mark to carry.
func TestLaunchTypesTheAskedForModelAndSaysItIsUnavailable(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	pfPersona(t, b, "architect", TierStrong)
	seedCatalog(t, b.App, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone

	if err := b.CreateSession(NewSessionOpts{Name: "r1", Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatalf("the preflight must never refuse a launch (rule 3): %v", err)
	}

	log := launchLog(t, b.App, fake)
	if !strings.Contains(log, "--model 'claude-fable-5-1'") {
		t.Errorf("the typed line must name the model that was asked for:\n%s", log)
	}
	if strings.Contains(log, "claude-opus-5") {
		t.Errorf("the typed line was substituted:\n%s", log)
	}
	if !strings.Contains(warn.String(), "architect: tier strong wants claude-fable-5-1 — unavailable on this account") {
		t.Errorf("unavailable was silent: %q", warn.String())
	}

	m, ok := b.readMeta("r1")
	if !ok {
		t.Fatal("no meta")
	}
	// tier: is what the session IS, the way cage: and degraded: are. Since
	// nothing can move it, it is also what was asked for.
	if m.Runtime != "claude" || m.Tier != TierStrong {
		t.Errorf("meta pair = %s/%s, want the pair that was asked for", m.Runtime, m.Tier)
	}
	// The mark's own byte, asserted absent by the one reader that would
	// have written it: a meta file that grows a `fallback:` line again fails
	// here rather than in a display test three surfaces away.
	if raw := metaBytes(t, b, "r1"); strings.Contains(raw, "fallback:") {
		t.Errorf("the removed mark is back in the meta:\n%s", raw)
	}

	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(list.String(), "⤵️") {
		t.Errorf("posse list still wears a fallback mark:\n%s", list.String())
	}
	if !strings.Contains(list.String(), b.App.RuntimeTierTag("claude", TierStrong)) {
		t.Errorf("posse list must name the tier that is really running:\n%s", list.String())
	}
}

func TestLaunchOnAnAvailableModelIsUnremarked(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	pfPersona(t, b, "architect", TierStrong)
	seedCatalog(t, b.App, time.Minute, "claude-fable-5-1", "claude-opus-5")

	if err := b.CreateSession(NewSessionOpts{Name: "r2", Agent: "architect", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if log := launchLog(t, b.App, fake); !strings.Contains(log, "--model 'claude-fable-5-1'") {
		t.Errorf("available means launch it:\n%s", log)
	}
	if m, _ := b.readMeta("r2"); m.Tier != TierStrong {
		t.Errorf("nothing happened, so nothing moved: %+v", m)
	}
	if strings.Contains(warn.String(), "unavailable") {
		t.Errorf("an available model has nothing to say: %q", warn.String())
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
	if log := launchLog(t, b.App, fake); !strings.Contains(log, "--model 'claude-fable-5-1'") {
		t.Errorf("unknown launches as asked:\n%s", log)
	}
	if raw := metaBytes(t, b, "r3"); strings.Contains(raw, "fallback:") {
		t.Errorf("a mark recorded with no catalog to justify it:\n%s", raw)
	}
}

// metaBytes reads a session's meta FILE, not its parsed struct. The parse
// is what stopped carrying `fallback:` (ADR 0003 §3), so a test that asks
// the struct can only ever agree with itself; the bytes on disk are the
// thing a future reader — a relaunch, another posse version — would find.
func metaBytes(t *testing.T, b *HerdrBackend, name string) string {
	t.Helper()
	raw, err := os.ReadFile(b.metaPath(name))
	if err != nil {
		t.Fatalf("meta for %s: %v", name, err)
	}
	return string(raw)
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

// catalogAt is the instant every frozen-clock catalog fixture was "read"
// at. It is deliberately nowhere near the suite's wall clock: a pin seeded
// here that renders "48h00m ago" can only have got there through App.Now,
// so a constructor that stops threading the clock reds it instead of
// passing by the grace of time.Now.
var catalogAt = time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

// seedCatalogAged is seedCatalog for a pin that asserts the RENDERED age.
// The preflight line dates its reading to the whole minute, so over a
// wall-clock fixture the assertion holds only while under 60s pass between
// the write and the render — and a loaded parallel run has been measured
// past that (ranger-base-5hjyh). This form writes the reading at catalogAt
// and pins the App's clock exactly `age` later, and returns that clock so
// a caller can date a cooldown against it.
func seedCatalogAged(t *testing.T, a *App, age time.Duration, ids ...string) time.Time {
	t.Helper()
	now := catalogAt.Add(age)
	seedCatalogEntry(t, a, modelEntry{At: catalogAt, Models: ids})
	a.Now = func() time.Time { return now }
	return now
}

// failingLister is the probe this instance has actually had since
// 2026-08-31: a 401 that leaves the retained reading ruling on every
// launch. It counts its calls, because "how many times was the endpoint
// asked" is half of what D3b is about.
func failingLister(hits *atomic.Int64) *ModelLister {
	return &ModelLister{
		URL:   "https://127.0.0.1:9/v1/models",
		Token: func() (string, CredMeta, error) { return fakeToken, CredMeta{}, nil },
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
	seedCatalogAged(t, a, 48*time.Hour, "claude-opus-5", "claude-sonnet-5") // frozen: the line is pinned to the minute

	// The launch's own loud line, in the ADR's shape (0039 D3c): what is
	// absent, from a reading named and dated, then the verdict.
	const want = "architect: tier strong wants claude-fable-5-1 — not in the catalog read 48h00m ago " +
		"and the probe is failing (model list endpoint returned 401 Unauthorized); availability UNKNOWN, launching as asked"
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if pf.Line != want {
		t.Errorf("line =\n  %q\nwant\n  %q", pf.Line, want)
	}
	// It prints, and the ask is unmoved: the launch renders
	// --model claude-fable-5-1. Since ADR 0003 §3 that is true of the
	// unavailable verdict too, and this arm is now about the WORDS — an
	// UNKNOWN line must not read as one that ruled.
	if pf.Wanted != "claude-fable-5-1" || strings.Contains(pf.Line, "unavailable") {
		t.Errorf("an UNKNOWN verdict must not read as a verdict: %+v", pf)
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
// inside model_probe_ttl a reading still RULES — it reaches the unavailable
// verdict rather than the UNKNOWN one — and the line carries no age. What
// the verdict no longer does is move anything (ADR 0003 §3); the lease still
// bounds which of the two sentences is entitled to be printed.
func TestAReadingInsideItsLeaseCarriesNoAgeClause(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	a := preflightApp(t)
	a.ModelLister = failingLister(&hits)
	seedCatalog(t, a, time.Minute, "claude-opus-5", "claude-sonnet-5") // fable gone, read a minute ago

	const want = "architect: tier strong wants claude-fable-5-1 — unavailable on this account; " +
		"launching as asked, and only an explicit --runtime/--tier/--model or a PID change moves it"
	if got := a.TierPreflight("architect", "claude", TierStrong, nil).Line; got != want {
		t.Errorf("line =\n  %q\nwant\n  %q", got, want)
	}
	if got := a.PreflightReport("", "claude", TierStandard, nil); got != "claude: tier standard → claude-opus-5 (available)" {
		t.Errorf("got %q", got)
	}
	if hits.Load() != 0 {
		t.Errorf("a reading inside its lease must ask nobody, %d requests", hits.Load())
	}
	// The same fixture, aged past the lease, stops ruling and starts dating
	// itself — the arm that proves the assertions above are measuring the
	// age of the reading and not something that never moves.
	seedCatalogAged(t, a, 48*time.Hour, "claude-opus-5", "claude-sonnet-5") // frozen: the clause is pinned to the minute
	pf := a.TierPreflight("architect", "claude", TierStrong, nil)
	if strings.Contains(pf.Line, "unavailable") {
		t.Errorf("control: past the lease no verdict may be reached: %+v", pf)
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
	// Frozen: the line below is pinned to "48h00m ago", and the cooldown
	// is dated against the same clock the preflight reads.
	now := catalogAt.Add(48 * time.Hour)
	a.Now = func() time.Time { return now }
	seedCatalogEntry(t, a, modelEntry{
		At:      catalogAt,
		Models:  []string{"claude-opus-5", "claude-sonnet-5"},
		RetryAt: now.Add(10 * time.Minute),
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
	// still rules, and still reaches the unavailable verdict rather than the
	// UNKNOWN one. --probe asked for maxAge 0 and was refused; how long a
	// reading may rule is the operator's number, not the caller's, so the
	// report keeps printing what a launch would print. Since ADR 0003 §3
	// "rules" is the whole of what a verdict does — it decides which
	// sentence, never which model (ranger-base-hv2zr).
	seedCatalogEntry(t, a, modelEntry{
		At:      now.Add(-30 * time.Minute),
		Models:  []string{"claude-opus-5", "claude-sonnet-5"},
		RetryAt: now.Add(10 * time.Minute),
	})
	const wantRules = "architect: tier strong wants claude-fable-5-1 — unavailable on this account; " +
		"launching as asked, and only an explicit --runtime/--tier/--model or a PID change moves it"
	if got := a.PreflightReportOn(a.ProbeCatalog(nil), "architect", "claude", TierStrong); got != wantRules {
		t.Errorf("a cooled-down reading inside its lease must still rule: %q", got)
	}
	// The control: with the cooldown expired, the same fixture DOES ask.
	seedCatalogEntry(t, a, modelEntry{
		At:      catalogAt,
		Models:  []string{"claude-opus-5", "claude-sonnet-5"},
		RetryAt: now.Add(-time.Minute),
	})
	a.ProbeCatalog(nil).known()
	if hits.Load() != 1 {
		t.Errorf("control: past the cooldown --probe must ask, %d requests", hits.Load())
	}
}
