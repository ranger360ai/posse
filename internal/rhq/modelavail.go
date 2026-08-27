package rhq

// Tier availability preflight (rangerhq-oay).
//
// A tier is a NAME, not a model id (ADR 0003 §1), and the launch turns it
// into one: `strong` on claude renders `--model claude-fable-5`. Nothing
// until now asked whether that id is one THIS ACCOUNT can actually run. On
// 2026-08-20 the operator's own session lost access to the strong model
// mid-day; a persona resolving `tier: strong` would have gone on launching,
// the CLI would have quietly served whatever it falls back to, and the shop
// would have stopped thinking at the tier its PID claims with nobody told.
// `posse cost` groups by the model that did the work, so a silent
// substitution also moves that spend into another tier's row with no line
// anywhere saying why.
//
// So: check, and when the answer is no, say so and substitute on purpose.
// Four rules, in the operator's words (2026-08-20):
//
//  1. The probe is the cheapest honest one on the box. Anthropic's model
//     list is a zero-token GET, read with the same credential the plan
//     guard already reads (planusage.go) and shared through a snapshot in
//     state/ with a TTL. Successful readings and rate-limit cooldowns are
//     shared across the fleet; other failures stay UNKNOWN and may retry.
//  2. Unavailable is LOUD: one line naming persona, tier, wanted model and
//     substitute, `fallback:` in the session meta so `posse list` and the
//     cockpit show it, and the tier column of `posse cost` already counts
//     the model that actually ran (TierForModel reads the transcript, not
//     the PID) — which is only true because (2) makes the substitution a
//     decision instead of an accident.
//  3. It NEVER refuses the launch on its own. "A degraded model is worse
//     than nothing" is the operator's judgement, not the launcher's. What
//     may still refuse afterwards is the PID's own `tier_floor:` and the
//     wall's own rule at `fast` (ADR 0003 §3) — both of those ARE the
//     operator's decision, recorded in advance, and the preflight hands
//     them the substituted pair rather than the asked-for one so they rule
//     on what would really launch. The fallback line prints either way.
//  4. It reuses the keys that exist: config `tier_fallback:` for where a
//     tier lands, and a runtime's own `model_<tier>:` for what a tier means
//     there. No new vocabulary for either.
//
// EVERYTHING HERE FAILS OPEN, and the asymmetry is the whole design: only a
// list that was actually read, and that does not contain the wanted id,
// demotes anything. An unreadable credential, an unreachable endpoint, a
// rate limit, an empty answer, a runtime whose models posse cannot name —
// all of those are UNKNOWN, and unknown launches exactly what it was asked
// to launch. The request outcome is recorded in state/model-catalog.log so
// UNKNOWN is diagnosable without changing that launch. A preflight that
// guesses "unavailable" would
// silently downgrade the whole shop, which is the failure it exists to
// prevent, one level up.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// ModelListURL is Anthropic's model catalog — a plain GET, no tokens
	// billed. There is deliberately NO override for it and none for the
	// credential: a second way to hand this process a token is a new
	// credential path, and a second way to POINT that token is the same
	// argument. RHQ_MODEL_LIST_URL used to be that second way and is gone
	// (ranger-base-17i) — nothing read it but the vulnerability, since the
	// tests inject App.ModelLister, the seam built for exactly this.
	//
	// It is /v1/models and not an /api/oauth/… route like the plan guard's:
	// probed unauthenticated 2026-08-23, /v1/models answers 401 (it exists
	// and wants a credential) while /api/oauth/models answers 404 (there is
	// no such route). Whether THIS account's OAuth token is accepted there
	// is the one thing a launch finds out at run time — and a 401 is
	// unknown, not unavailable, so finding out costs nothing.
	ModelListURL = "https://api.anthropic.com/v1/models"
	// ModelListHost is the API host that answers it, and the one host —
	// besides this machine — the preflight will put the account's
	// credential in front of (credpin.go). A runtime is probed
	// only when its own `egress:` names this host — that is posse saying
	// "this runtime's model ids are ids I know a catalog for", and it is
	// data rather than a hardcoded runtime name, so a template-only runtime
	// on the same API is covered too.
	ModelListHost = "api.anthropic.com"
	// ModelProbeTTLDefault is how long one reading of the catalog is
	// shared. An account's model access changes on the scale of a
	// subscription change, not of a dispatch pass, so an hour is generous
	// to the endpoint and still catches the outage that produced this bead
	// within one pass of a fleet that runs all day.
	ModelProbeTTLDefault = time.Hour
	// modelProbeTimeout bounds the request. Shorter than the plan guard's
	// 10s on purpose: this one sits in front of a LAUNCH, and a launcher
	// that hangs ten seconds on a monitoring call has traded the problem
	// for a worse one. Past it the answer is unknown, which launches.
	modelProbeTimeout = 5 * time.Second
	// modelCooldownDefault is how long a rate-limited catalog is left alone
	// when the response named no Retry-After.
	modelCooldownDefault = 15 * time.Minute
	// modelFallbackHops bounds the walk down `tier_fallback:`. A chain
	// longer than this is a config the operator should see, not one the
	// launcher should keep following.
	modelFallbackHops = 4
	// FallbackNone is the `tier_fallback:` value that means "there is no
	// substitute for this one" — the explicit way to turn the default off
	// for a tier without turning the map into a place things vanish.
	FallbackNone = "none"
)

// ─── the catalog ─────────────────────────────────────────────────────────────

// ModelLister reads the account's model catalog. Token and HTTP are fields
// for the same reason PlanReader's are: tests inject a fake endpoint, and
// nothing else needs to.
type ModelLister struct {
	URL   string
	Token func() (string, error)
	HTTP  *http.Client
}

func NewModelLister() *ModelLister {
	return &ModelLister{
		URL:   ModelListURL,
		Token: MeterToken("claude"),
		HTTP:  pinnedClient(modelProbeTimeout, "model list endpoint", ModelListHost),
	}
}

// List returns the model ids this account can use. Errors never quote a
// header or the credential — same rule as planusage.go.
func (r *ModelLister) List() ([]string, error) {
	if r.Token == nil || r.URL == "" {
		return nil, Die("model lister not configured")
	}
	// Before the credential, not after: a URL this process does not
	// credential is answered by asking nobody and reading nothing. getPage
	// checks again where the header is actually set (credpin.go).
	if err := pinnedEndpoint("model list endpoint", r.URL, ModelListHost); err != nil {
		return nil, err
	}
	tok, err := r.Token()
	if err != nil {
		return nil, err
	}
	cl := r.HTTP
	if cl == nil {
		cl = &http.Client{Timeout: modelProbeTimeout}
	}
	var ids []string
	url := r.URL + "?limit=100"
	// Bounded: the catalog is dozens of entries, so three pages is already
	// more than the endpoint has. A cursor that never advances must not
	// become a loop in front of a launch.
	for i := 0; i < 3; i++ {
		page, err := r.getPage(cl, url, tok)
		if err != nil {
			return nil, err
		}
		for _, m := range page.Data {
			if m.ID != "" {
				ids = append(ids, m.ID)
			}
		}
		if !page.HasMore || page.LastID == "" {
			break
		}
		url = r.URL + "?limit=100&after_id=" + page.LastID
	}
	return ids, nil
}

// modelPage is one response of the catalog's cursor pagination.
type modelPage struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	HasMore bool   `json:"has_more"`
	LastID  string `json:"last_id"`
}

func (r *ModelLister) getPage(cl *http.Client, url, tok string) (modelPage, error) {
	var page modelPage
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return page, Die("bad model list url")
	}
	if err := pinnedRequest("model list endpoint", req, ModelListHost); err != nil {
		return page, err
	}
	// The same pair the plan guard sends: a Claude Code OAuth token goes on
	// Authorization: Bearer with the oauth beta header, never on x-api-key.
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("anthropic-beta", planBetaHeader)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := cl.Do(req)
	if err != nil {
		// A refused redirect comes back through here wrapped in *url.Error;
		// it is not an outage and must not be reported as one.
		var pin *PinRefusal
		if errors.As(err, &pin) {
			return page, pin
		}
		// A transport error can quote the URL but never a header.
		return page, Die("model list endpoint unreachable")
	}
	defer resp.Body.Close()
	// A redirect a client without our CheckRedirect followed: the answer
	// came from a host we do not credential, so it is not an answer — and
	// an error here is what keeps it out of ModelCache.store.
	if err := pinnedResponse("model list endpoint", resp, ModelListHost); err != nil {
		return page, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return page, &RateLimit{Status: resp.Status, RetryAfter: retryAfter(resp.Header.Get("Retry-After"), time.Now())}
	}
	if resp.StatusCode != http.StatusOK {
		return page, Die("model list endpoint returned %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&page); err != nil {
		return page, Die("model list response is not the expected JSON")
	}
	return page, nil
}

// modelEntry is the shared snapshot: what was read, when, and any cooldown
// the endpoint asked for. Same shape and same reasoning as plancache.go —
// one derived copy with an age on it, replaced rather than mutated, so
// last-writer-wins is the right answer.
type modelEntry struct {
	At      time.Time `json:"at"`
	Models  []string  `json:"models"`
	RetryAt time.Time `json:"retry_at,omitempty"`
}

// ModelCache is the instance's one reading of the catalog.
type ModelCache struct {
	Path   string // "" = no sharing, every ask is a request
	Log    string // catalog-probe log; "" = no log
	Caller string
	Lister *ModelLister
	Now    func() time.Time
	// Errw is where the one failure this preflight must not swallow gets
	// said: posse's own gate shim refusing posse's credential read
	// (ranger-base-r64). Every other read failure is UNKNOWN and UNKNOWN
	// launches, so the log is the right place for it; a refusal by our own
	// deny: is a misconfiguration of ours and the only witness before this
	// was one line in model-catalog.log. nil = say nothing.
	Errw io.Writer
}

func (a *App) ModelCache() *ModelCache {
	l := a.ModelLister
	if l == nil {
		l = NewModelLister()
	}
	return &ModelCache{
		Path: filepath.Join(a.StateDir, "model-catalog.json"),
		Log:  filepath.Join(a.StateDir, "model-catalog.log"), Caller: "preflight", Lister: l,
	}
}

func (c *ModelCache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Models returns the catalog, no older than maxAge. The bool is the only
// thing callers may act on: false means "posse does not know", and nothing
// downstream may read that as "unavailable".
func (c *ModelCache) Models(maxAge time.Duration) ([]string, bool) {
	now := c.now()
	e, have := c.load()
	if have && maxAge > 0 && !e.At.IsZero() && now.Sub(e.At) >= 0 && now.Sub(e.At) < maxAge && len(e.Models) > 0 {
		return e.Models, true
	}
	if have && now.Before(e.RetryAt) {
		// Re-asking a rate limiter on every launch is how a fleet extends a
		// 429 (rangerhq-tdy8). The last reading, if there is one, is still
		// the newest fact anyone has.
		return e.Models, len(e.Models) > 0
	}
	l := c.Lister
	if l == nil {
		l = NewModelLister()
	}
	ids, err := l.List()
	c.logRead(now, ids, err)
	c.noteGateRefusal(err)
	if err != nil {
		var rl *RateLimit
		if errors.As(err, &rl) {
			e.RetryAt = now.Add(modelCooldown(rl.RetryAfter))
			c.store(e)
		}
		return e.Models, have && len(e.Models) > 0
	}
	if len(ids) == 0 {
		// An empty catalog is not an account with no models; it is an
		// answer posse does not understand. Never cached as fact.
		return nil, false
	}
	c.store(modelEntry{At: now, Models: ids})
	return ids, true
}

// gateRefusalNotices keeps the line below to once per process per rule —
// the launch preflight runs in front of every launch, and a shop dispatching
// all day inside a gated pane must not get the same sentence per launch.
// Same shape and same reason as app.go's legacyHomeNotices.
var gateRefusalNotices sync.Map

// noteGateRefusal says, on stderr, that the catalog read failed because
// posse's own gate refused posse's reader. The preflight's UNKNOWN branch is
// otherwise silent by design (it launches at the asked-for tier and writes
// only to model-catalog.log), which is right for an outage and wrong for a
// refusal we configured ourselves.
func (c *ModelCache) noteGateRefusal(err error) {
	var g *GateRefusal
	if c.Errw == nil || !errors.As(err, &g) {
		return
	}
	if _, loaded := gateRefusalNotices.LoadOrStore(g.Cmd+"\x00"+g.Rule, struct{}{}); loaded {
		return
	}
	fmt.Fprintf(c.Errw, "posse: %v; tier availability UNKNOWN, launches take the tier as asked\n", g)
}

// logRead makes the UNKNOWN side of the preflight observable. Before this
// log, a successful catalog read containing the wanted model and a 401 that
// left no snapshot had the same visible launch outcome: no line at all.
// Errors from ModelLister are deliberately generic and never carry the
// credential or a header, so they are safe to quote here.
func (c *ModelCache) logRead(now time.Time, ids []string, err error) {
	if c.Log == "" {
		return
	}
	caller := c.Caller
	if caller == "" {
		caller = "-"
	}
	outcome := fmt.Sprintf("ok models=%d", len(ids))
	if err != nil {
		var rl *RateLimit
		if errors.As(err, &rl) {
			outcome = fmt.Sprintf("%s cooldown=%s", statusCode(rl.Status), BlindFor(modelCooldown(rl.RetryAfter)))
		} else {
			outcome = "failed: " + err.Error()
		}
	} else if len(ids) == 0 {
		outcome = "failed: model list returned an empty catalog"
	}
	line := fmt.Sprintf("%s %s %s\n", now.UTC().Format(time.RFC3339), caller, outcome)
	if err := os.MkdirAll(filepath.Dir(c.Log), 0o755); err != nil {
		return
	}
	f, ferr := os.OpenFile(c.Log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if ferr != nil {
		return
	}
	f.WriteString(line)
	f.Close()
	trimReadLog(c.Log)
}

func modelCooldown(d time.Duration) time.Duration {
	switch {
	case d <= 0:
		return modelCooldownDefault
	case d > planCooldownMax:
		return planCooldownMax
	}
	return d
}

func (c *ModelCache) load() (modelEntry, bool) {
	var e modelEntry
	if c.Path == "" {
		return e, false
	}
	b, err := os.ReadFile(c.Path)
	if err != nil {
		return e, false
	}
	if err := json.Unmarshal(b, &e); err != nil {
		return modelEntry{}, false
	}
	return e, true
}

// store replaces the snapshot atomically — a reader in another process sees
// the old file or the new one, never half of either. Fail-quiet: an
// unwritable state dir costs the sharing, never the reading.
func (c *ModelCache) store(e modelEntry) {
	if c.Path == "" {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.CreateTemp(dir, ".model-catalog-*.json")
	if err != nil {
		return
	}
	tmp := f.Name()
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		os.Remove(tmp)
		return
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, c.Path); err != nil {
		os.Remove(tmp)
	}
}

// ─── config ──────────────────────────────────────────────────────────────────

// ModelPreflight reads config `model_preflight:` — the off switch. Absent
// or anything but the word `false` means on, because a check that fails
// open costs nothing when it is wrong and catches the outage that produced
// this bead when it is right.
func (a *App) ModelPreflight() bool {
	return strings.TrimSpace(YamlGet(a.ConfigPath, "model_preflight")) != "false"
}

// ModelProbeTTL reads config `model_probe_ttl:` (the house's duration form:
// 1h, 3600, 0). Unset = ModelProbeTTLDefault; **0 = no sharing**, every
// launch asks for itself — the same meaning `plan_usage_ttl: 0` has, and
// the same escape hatch. A value that is not a duration is named on errw
// and the default stands: a typo must be visible, and the visible failure
// is the safe one.
func (a *App) ModelProbeTTL(errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, "model_probe_ttl"))
	if raw == "" {
		return ModelProbeTTLDefault
	}
	if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
		return d
	}
	if errw != nil {
		fmt.Fprintf(errw, "posse: config model_probe_ttl: %q is not a duration (1h, seconds, or 0) — using %s\n",
			raw, BlindFor(ModelProbeTTLDefault))
	}
	return ModelProbeTTLDefault
}

// FallbackFor answers where a persona at (runtime, tier) goes when the
// model that pair names is not on the account.
//
// config `tier_fallback:` is a one-level map. Its KEY is a persona name or
// a tier name — persona first, because a lane can need a different
// substitute than its tier's: the operator's standing example is the
// security lane, whose fallback from the strong model may be a different
// RUNTIME rather than a cheaper model on the same one (rangerhq-u2p). Its
// VALUE is either a tier name (hop down on the same runtime) or a runtime
// name (hop across at the same tier), or `none` for "there is no substitute
// for this one".
//
// The default is `strong` → `standard`, which on claude is fable-5 →
// opus-5, and it is a floor rather than a seed: a map that names other keys
// does NOT take it away. That is deliberately unlike `tier_by_label:`,
// where a present key replaces the ADR default wholesale — here the
// operator's rule is that EVERYONE falls back, so adding one persona line
// must not be able to silently switch the rest of the shop off.
//
// Returns ("", "", why) when there is no substitute; why is the clause that
// goes in the loud line.
func (a *App) FallbackFor(persona, runtime, tier string) (string, string, string) {
	pairs := YamlMapPairs(a.ConfigPath, "tier_fallback")
	v, from := "", ""
	for _, kv := range pairs {
		if kv[0] == persona && persona != "" {
			v, from = kv[1], "tier_fallback "+persona
			break
		}
	}
	if v == "" {
		for _, kv := range pairs {
			if kv[0] == tier {
				v, from = kv[1], "tier_fallback "+tier
				break
			}
		}
	}
	if v == "" {
		if tier != TierStrong {
			return "", "", fmt.Sprintf("tier_fallback names no substitute for %s", tier)
		}
		v, from = TierStandard, "the default"
	}
	if v == FallbackNone {
		return "", "", fmt.Sprintf("%s says none", from)
	}
	if ValidTier(v) {
		if v == tier {
			return "", "", fmt.Sprintf("%s points %s at itself", from, tier)
		}
		return runtime, v, ""
	}
	if _, err := a.LoadRuntime(v); err == nil {
		if v == runtime {
			return "", "", fmt.Sprintf("%s points %s at itself", from, runtime)
		}
		return v, tier, ""
	}
	// A value that is neither is a config error, and the honest place to
	// report it is the line the operator is already being shown — a launch
	// is not the moment to refuse over a typo in a fallback map.
	return "", "", fmt.Sprintf("%s: %q is neither a tier nor a runtime", from, v)
}

// ─── the preflight ───────────────────────────────────────────────────────────

// Preflight is what a launch gets back: the (runtime, tier) it should
// actually use, the model ids on both ends, and the one line to print when
// they differ. Line == "" means there is nothing to say, which is the
// answer for every launch on an available model AND for every launch posse
// cannot check.
type Preflight struct {
	Runtime string // launch on this
	Tier    string // at this
	Wanted  string // the model the asked-for pair named ("" = the runtime's own default)
	Got     string // the model the returned pair names
	Line    string // the loud line, "" = nothing happened
}

// Fell reports whether the pair moved.
func (p Preflight) Fell() bool { return p.Line != "" }

// TierPreflight checks that the model a resolved tier names is one this
// account can run, and substitutes per `tier_fallback:` when it is not.
// persona may be "" (a session with no PID) — then only the tier keys of
// the map apply.
//
// It is called once per LAUNCH, on the pair the launch has already resolved
// (herdrback.go planLaunch), and never per prompt: a live session's model
// was decided when it started, and re-deciding it under a running CLI would
// be a claim posse cannot make good on.
func (a *App) TierPreflight(persona, runtime, tier string, errw io.Writer) Preflight {
	p := Preflight{Runtime: runtime, Tier: tier}
	rt, err := a.LoadRuntime(runtime)
	if err != nil {
		return p
	}
	p.Wanted, p.Got = rt.Model(tier), rt.Model(tier)
	// Nothing to check: this runtime maps no model for this tier (grok
	// today — {model} renders empty and the CLI picks its own), the
	// operator turned the preflight off, or posse knows no catalog for this
	// runtime's ids (codex maps gpt-5.6-* since ranger-base-arm, and this
	// catalog is Anthropic's — anthropicAPI below is what keeps those ids
	// from being checked against a list that will never hold them).
	if p.Wanted == "" || !a.ModelPreflight() || !anthropicAPI(rt) {
		return p
	}
	have, known := a.availableModels(errw)
	if !known || have[p.Wanted] {
		return p
	}

	clauses := []string{}
	curRT, curTier := runtime, tier
	seen := map[string]bool{curRT + "/" + curTier: true}
	landed := false
	for hop := 0; hop < modelFallbackHops && !landed; hop++ {
		nextRT, nextTier, why := a.FallbackFor(persona, curRT, curTier)
		if why != "" {
			clauses = append(clauses, why)
			break
		}
		if key := nextRT + "/" + nextTier; seen[key] {
			clauses = append(clauses, "tier_fallback loops back to "+key)
			break
		}
		seen[nextRT+"/"+nextTier] = true
		rt2, err := a.LoadRuntime(nextRT)
		if err != nil {
			clauses = append(clauses, fmt.Sprintf("tier_fallback names runtime %s, which will not load", nextRT))
			break
		}
		got := rt2.Model(nextTier)
		curRT, curTier = nextRT, nextTier
		p.Runtime, p.Tier, p.Got = curRT, curTier, got
		switch {
		case got == "" || !anthropicAPI(rt2):
			// Off the catalog posse can read: the hop is taken and stated,
			// and what that runtime serves is its own business.
			clauses = append(clauses, "falling back to "+hopDesc(runtime, curRT, curTier, got))
			landed = true
		case have[got]:
			clauses = append(clauses, "falling back to "+hopDesc(runtime, curRT, curTier, got))
			landed = true
		default:
			clauses = append(clauses, got+" — ALSO unavailable")
		}
	}
	if !landed {
		// Rule (3): the launch happens anyway. Whatever it lands on is the
		// best this map could do, and saying so is the whole job.
		clauses = append(clauses, "launching on "+hopDesc(runtime, p.Runtime, p.Tier, p.Got)+" anyway")
	}
	who := persona
	if who == "" {
		who = "session"
	}
	p.Line = fmt.Sprintf("%s: tier %s wants %s — unavailable, %s", who, tier, p.Wanted, strings.Join(clauses, ", "))
	return p
}

// PreflightReport is the same question `posse gates` asks out loud: for
// this persona on this runtime at this tier, what model does the launch
// want, and what would it actually get?
//
// It exists because every other input to a launch is on this machine and
// this one is not — the catalog belongs to the ACCOUNT, and an operator
// with no way to run the probe by hand cannot tell "fable is gone" from
// "the probe never answers here". Reading it costs whatever the cached
// snapshot costs, which is usually nothing.
func (a *App) PreflightReport(persona, runtime, tier string, errw io.Writer) string {
	rt, err := a.LoadRuntime(runtime)
	if err != nil {
		return ""
	}
	want := rt.Model(tier)
	// Its own lines lead with the runtime, because that is what they
	// describe. The fallback branch does not: it returns the LAUNCH's line
	// verbatim, persona and all, so the operator reads here the same bytes
	// a launch would print — one rendering, no drift.
	switch {
	case want == "":
		return fmt.Sprintf("%s: tier %s → this runtime maps no model; the CLI picks its own", runtime, tier)
	case !a.ModelPreflight():
		return fmt.Sprintf("%s: tier %s → %s (preflight off: config model_preflight: false)", runtime, tier, want)
	case !anthropicAPI(rt):
		return fmt.Sprintf("%s: tier %s → %s (no model catalog posse can read for this runtime)", runtime, tier, want)
	}
	have, known := a.availableModels(errw)
	if !known {
		return fmt.Sprintf("%s: tier %s → %s (availability UNKNOWN — the catalog could not be read; the launch takes the tier as asked)", runtime, tier, want)
	}
	if have[want] {
		return fmt.Sprintf("%s: tier %s → %s (available)", runtime, tier, want)
	}
	return a.TierPreflight(persona, runtime, tier, errw).Line
}

// hopDesc names where a hop landed, the way a person would say it: the
// model id alone when only the tier moved, and the runtime too when the
// runtime moved or when that runtime picks its own model.
func hopDesc(fromRT, toRT, toTier, model string) string {
	if model == "" {
		return fmt.Sprintf("%s/%s (the runtime's own default model)", toRT, toTier)
	}
	if toRT != fromRT {
		return fmt.Sprintf("%s on %s", model, toRT)
	}
	return model
}

// availableModels is the catalog as a set, plus whether posse actually
// knows it. Nothing may treat a false here as "unavailable".
func (a *App) availableModels(errw io.Writer) (map[string]bool, bool) {
	mc := a.ModelCache()
	mc.Errw = errw
	ids, ok := mc.Models(a.ModelProbeTTL(errw))
	if !ok {
		return nil, false
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, true
}

// anthropicAPI: are this runtime's model ids ids the catalog above lists?
// Asked of the runtime's own `egress:` (ADR 0002 §4) rather than of its
// name, so a template-only runtime pointed at the same API is covered and a
// built-in that moves is not miscategorised by a stale name check.
func anthropicAPI(rt *Runtime) bool {
	for _, h := range rt.Egress {
		if h == ModelListHost {
			return true
		}
	}
	return false
}
