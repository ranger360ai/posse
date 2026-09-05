package posse

// Tier availability preflight (rangerhq-oay).
//
// A tier is a NAME, not a model id (ADR 0003 §1), and the launch turns it
// into one: `strong` on claude renders `--model claude-fable-5-1`. Nothing
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
// list that was actually read, INSIDE ITS LEASE, and that does not contain
// the wanted id, demotes anything. An unreadable credential, an unreachable
// endpoint, a rate limit, an empty answer, a runtime whose models posse
// cannot name — all of those are UNKNOWN, and unknown launches exactly what
// it was asked to launch. The request outcome is recorded in
// state/model-catalog.log so UNKNOWN is diagnosable without changing that
// launch. A preflight that guesses "unavailable" would
// silently downgrade the whole shop, which is the failure it exists to
// prevent, one level up.
//
// The LEASE is `model_probe_ttl` — the number the operator already owns to
// say how fresh a reading must be — and it bounds the sentence above (ADR
// 0039 D3c, the operator's ruling 2026-09-01 on ranger-base-v1p66). A
// retained reading past it, with the refresh failing, is QUOTED and obeyed
// by nothing: the verdict is UNKNOWN, the launch takes the tier as asked,
// and when the wanted id is absent from that reading the line still prints
// so nobody is launched on an unlisted id in silence. Before this, a
// reading ruled forever whenever the probe was down — and on this instance
// the probe's credential rots in hours (ranger-base-wkai3), so a model bump
// demoted the whole shop until somebody hand-edited a state file. A
// rate-limit cooldown governs whether posse RE-ASKS, never whether it
// trusts: a 429 renewed all day would otherwise renew trust in a day-old
// list (rangerhq-tdy8, ranger-base-c3vqe).

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
	// modelCatalogRuntime is the runtime whose credential this catalog is
	// read with — the one NewModelLister has always named in its
	// MeterToken call, said once now that the session half names it too
	// (ADR 0039 D3d). It is not the runtime being PROBED: which runtimes
	// this reading answers for is decided by `egress:` naming ModelListHost
	// above, and a template-only runtime on the same API is covered by that
	// without being named here.
	modelCatalogRuntime = "claude"
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
	Token func() (string, CredMeta, error)
	// Fallback is the second credential this lister may present, and it is
	// the whole of ADR 0039 D3d's "meter store as fallback". Token is the
	// PREFERENCE — on the App path, the session mint the launch is about to
	// hand the session — and there are exactly two ways it does not answer:
	// there is none to read, or the endpoint refuses the one there was.
	// Both are answered here, once each, because the alternative is a token
	// func that has to know it is being retried.
	//
	// nil is one credential and one attempt: the bare constructor below,
	// and every lister a test injects a Token into, behave exactly as they
	// did before D3d.
	Fallback func() (string, CredMeta, error)
	HTTP     *http.Client
}

func NewModelLister() *ModelLister {
	return &ModelLister{
		URL:   ModelListURL,
		Token: MeterToken(modelCatalogRuntime),
		HTTP:  pinnedClient(modelProbeTimeout, "model list endpoint"),
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
	// The meter's metadata is dropped here and that is deliberate: this
	// endpoint's failures are not a credential class (no *AuthFailure is
	// built in this file), so naming the store would add a word no sentence
	// here renders.
	tok, _, err := r.Token()
	// ABSENCE, the first of the two ways the preferred credential does not
	// answer: no variable is decided for this runtime (*NoSource), or the
	// one that is decided is not in this process's environment. Nothing has
	// been asked of the endpoint yet, so this fallback costs no request.
	fellBack := false
	if r.Fallback != nil && (err != nil || tok == "") {
		tok, _, err = r.Fallback()
		fellBack = true
	}
	if err != nil {
		return nil, err
	}
	cl := r.HTTP
	if cl == nil {
		cl = &http.Client{Timeout: modelProbeTimeout}
	}
	ids, err := r.read(cl, tok)
	// REFUSAL, the second way: the credential was there, it was presented,
	// and the endpoint said "not you". One more read with the other
	// credential, and exactly one — a probe that keeps re-presenting
	// credentials to a refusing endpoint is how a fleet extends a 429
	// (rangerhq-tdy8), and above this ModelCache decides whether the read
	// happens at all, so the bound that matters is per READ and not per
	// launch. Never asked when the fallback is what was already presented:
	// that is the same request twice, which is the traffic this bounds.
	var refused *modelCredRefusal
	if r.Fallback != nil && !fellBack && errors.As(err, &refused) {
		// A fallback that cannot be read leaves the endpoint's refusal as
		// the answer, which is the one the operator acts on: the store
		// posse could not reach is not why the catalog is unknown.
		if alt, _, ferr := r.Fallback(); ferr == nil {
			ids, err = r.read(cl, alt)
		}
	}
	return ids, err
}

// read walks the catalog's pages with ONE credential. It is separate from
// List because a refused credential is read again with the other one, and a
// retry that resumed mid-pagination would join the two halves of two
// different reads into one answer.
func (r *ModelLister) read(cl *http.Client, tok string) ([]string, error) {
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

// modelCredRefusal is this endpoint refusing the credential that was
// PRESENTED — 401, or 403 — as a TYPE rather than a sentence to match on,
// which is the rule *AuthFailure and *RateLimit each got a type to keep.
//
// It adds no words: it wraps the generic line this file has always returned
// and renders identically, because the operator sentences a credential
// class earns are the usage guard's, whose Error() names that endpoint and
// that operator's next move. This file still builds no *AuthFailure. What
// the type is for is one decision inside List — present the other
// credential, once — and that decision must not be reachable by grepping a
// status code back out of prose.
//
// Both statuses, because they are one class here: 401 is a stale token and
// 403 is one that was never entitled (ADR 0019 D2), and for a probe holding
// a second credential the move is the same either way. Only the 401 arm is
// MEASURED against this endpoint (ranger-base-au0o4, a bogus bearer).
type modelCredRefusal struct{ err error }

func (e *modelCredRefusal) Error() string { return e.err.Error() }
func (e *modelCredRefusal) Unwrap() error { return e.err }

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
	// came from a host we did not ask, so it is not an answer — and an
	// error here is what keeps it out of ModelCache.store, which has no
	// MayShare gate behind it (credpin.go rule 3, ranger-base-07ep).
	if err := pinnedResponse("model list endpoint", resp, askedHost(r.URL)); err != nil {
		return page, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return page, &RateLimit{Status: resp.Status, RetryAfter: retryAfter(resp.Header.Get("Retry-After"), time.Now())}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return page, &modelCredRefusal{err: Die("model list endpoint returned %s", resp.Status)}
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

// sessionCatalogToken is the catalog probe's PREFERRED credential: the
// session mint this launch is about to hand the session (ADR 0039 D3d).
//
// MEASURED 2026-09-02 (ranger-base-au0o4): `/v1/models` answers 200 to that
// mint — eleven ids, the newest among them — while a bogus bearer gets 401
// and the same real token moved onto `x-api-key` gets 401, so the 200 is
// the credential's and not the endpoint's good mood. The usage endpoint's
// refusal of a minted token (planusage.go, 403) is a fact about that
// endpoint and says nothing about this one; D3d was written conditional on
// exactly this measurement.
//
// What it buys: the probe asks "can this account run the id" of exactly the
// credential that will run it, and it rots on the same clock the sessions
// do rather than on the meter store's, which rots in hours and has left the
// probe 401ing since 2026-08-31 (ranger-base-wkai3).
//
// It ACQUIRES nothing (ADR 0019 D1): the value comes back through the seam,
// out of the env set the launch already realized into this process's
// environment. The runtime is loaded rather than assumed so that an
// overlay's `cage_cred:` names the variable here exactly as it does at the
// launch (ADR 0021). Absence — no variable decided, or none in this
// environment — comes back as the error it is, and ModelLister.Fallback
// answers it with the meter store.
func (a *App) sessionCatalogToken() func() (string, CredMeta, error) {
	return func() (string, CredMeta, error) {
		rt, err := a.LoadRuntime(modelCatalogRuntime)
		if err != nil {
			return "", CredMeta{}, err
		}
		return a.ReadCredential(rt, CredSession)
	}
}

func (a *App) ModelCache() *ModelCache {
	l := a.ModelLister
	if l == nil {
		l = NewModelLister()
		// The App path is the only one that CAN prefer the session
		// credential: the seam's session half hangs off *App (ADR 0019),
		// and a bare NewModelLister has no home to read an env set out of.
		// So the bare constructor stays on the meter store — every test
		// that injects a Token is untouched, which is the seam's whole
		// point — and the meter store it already chose becomes the
		// fallback, so neither half of D3d's preference is spelled twice.
		l.Fallback = l.Token
		l.Token = a.sessionCatalogToken()
	}
	return &ModelCache{
		Path: filepath.Join(a.StateDir, "model-catalog.json"),
		Log:  filepath.Join(a.StateDir, "model-catalog.log"), Caller: "preflight", Lister: l,
		Now: a.Now,
	}
}

func (c *ModelCache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// CatalogRead is what the reading a caller is being handed knows about
// itself: how old it is, and what the refresh said when it failed. Models
// computes both already — the age for the gate-refusal notice, the error
// for the log — and used to drop them at the return, which left a verdict
// reading "unavailable" off a list taken before the wanted id existed with
// nothing on the line saying so (ADR 0039 D3a).
//
// Age is of the reading RETURNED, so 0 means "read just now" and never
// "old beyond measure": catalogAge is the one place that judgement is made.
// Err is ModelLister's own generic string — never a header, never the
// credential — so it is safe to put in front of an operator.
type CatalogRead struct {
	Age time.Duration
	Err error
}

// Models returns the catalog, no older than maxAge. The bool is the only
// thing callers may act on: false means "posse does not know", and nothing
// downstream may read that as "unavailable".
func (c *ModelCache) Models(maxAge time.Duration) ([]string, bool) {
	ids, ok, _ := c.ModelsRead(maxAge, maxAge)
	return ids, ok
}

// ModelsRead is Models plus the reading's own age and the failed refresh
// behind it — the same two values `kept` and `catalogAge` already answer
// the notice with, so the sentence an operator reads and the bool a launch
// acts on cannot come from different facts.
//
// Two durations, because they are two questions (ADR 0039 D3c). maxAge is
// how fresh this CALLER wants it before posse re-asks — `--probe` passes 0,
// "fresh only". lease is how long a retained reading may still RULE when
// that ask fails, which is `model_probe_ttl` and is the operator's number,
// not the caller's: were --probe to lease by its own maxAge, a forced read
// over a five-minute-old snapshot would report UNKNOWN for a launch that
// would demote on it, and `posse runtimes` would stop printing the bytes a
// launch prints. Models(maxAge) is the launch's form, where the two are one
// number.
func (c *ModelCache) ModelsRead(maxAge, lease time.Duration) ([]string, bool, CatalogRead) {
	now := c.now()
	e, have := c.load()
	if have && withinLease(e, now, maxAge) && len(e.Models) > 0 {
		return e.Models, true, CatalogRead{Age: catalogAge(e, now)}
	}
	// May the retained reading still rule? Asked once, here, so the
	// cooldown branch and the failed-read branch cannot answer differently.
	rules := kept(e, have) && withinLease(e, now, lease)
	if have && now.Before(e.RetryAt) {
		// Re-asking a rate limiter on every launch is how a fleet extends a
		// 429 (rangerhq-tdy8). The last reading, if there is one, is still
		// the newest fact anyone has — and past its lease that is not
		// enough to rule on: trust and re-ask are different questions, and
		// coupling them lets a cooldown renewed all day renew trust in a
		// day-old list (ranger-base-c3vqe). Nothing was asked, so there is
		// no refresh error to report: the age says the reading is old and
		// state/model-catalog.log holds the 429 that stopped the ask.
		return e.Models, rules, CatalogRead{Age: catalogAge(e, now)}
	}
	l := c.Lister
	if l == nil {
		l = NewModelLister()
	}
	ids, err := l.List()
	c.logRead(now, ids, err)
	// The notice below has to say what Models is about to RETURN, so it is
	// told what a failed read falls back to: the prior reading, when there
	// is one and it may still rule (ranger-base-co5n, bounded by D3c).
	c.noteGateRefusal(err, rules, catalogAge(e, now))
	if err != nil {
		var rl *RateLimit
		if errors.As(err, &rl) {
			e.RetryAt = now.Add(modelCooldown(rl.RetryAfter))
			c.store(e)
		}
		// The ids go back whatever the bool says: past its lease the
		// reading no longer rules, but the line that says so quotes it.
		return e.Models, rules, CatalogRead{Age: catalogAge(e, now), Err: err}
	}
	if len(ids) == 0 {
		// An empty catalog is not an account with no models; it is an
		// answer posse does not understand. Never cached as fact.
		return nil, false, CatalogRead{Age: catalogAge(e, now)}
	}
	c.store(modelEntry{At: now, Models: ids})
	return ids, true, CatalogRead{}
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
//
// Which sentence depends on what the caller is about to be handed, because
// that bool is the only thing TierPreflight acts on (ranger-base-co5n). With
// no snapshot the answer really is UNKNOWN and the launch takes the tier as
// asked. Over a retained reading that STILL RULES — inside its lease — the
// refused refresh changes nothing: Models returns it as known, TierPreflight
// rules on it, and a launch may still be demoted by it, so calling it
// UNKNOWN there describes a launch that does not happen and hides the fact
// the operator needs, which is that the reading being ruled on is as old as
// it is. Past the lease the bool is false (D3c) and so is that sentence:
// nothing is demoted, and UNKNOWN is the truth again.
func (c *ModelCache) noteGateRefusal(err error, rules bool, age time.Duration) {
	var g *GateRefusal
	if c.Errw == nil || !errors.As(err, &g) {
		return
	}
	if _, loaded := gateRefusalNotices.LoadOrStore(g.Cmd+"\x00"+g.Rule, struct{}{}); loaded {
		return
	}
	if rules {
		fmt.Fprintf(c.Errw, "posse: %v; tier availability is still %s, launches rule on that reading\n", g, catalogRead(age))
		return
	}
	fmt.Fprintf(c.Errw, "posse: %v; tier availability UNKNOWN, launches take the tier as asked\n", g)
}

// kept: what a failed read falls back to — a reading to hand on, whether or
// not it may still rule. Named once so the notice and the return cannot
// drift apart.
func kept(e modelEntry, have bool) bool { return have && len(e.Models) > 0 }

// withinLease: is this reading inside a window of `lease`? It answers both
// of ModelsRead's questions — "fresh enough not to re-ask" against maxAge,
// and "may it still RULE when the re-ask fails" against model_probe_ttl
// (ADR 0039 D3c) — because they are the same measurement of the same `at`,
// and two spellings of it would be two places to fix.
//
// A lease of 0 leases nothing: `model_probe_ttl: 0` is "every launch asks
// for itself", so a reading it did not take may not rule for it. An
// UNDATABLE reading (no `at`, or an `at` in our future) has no lease to be
// inside either — the same judgement catalogAge makes about its age, and
// the safe direction: it launches what it was asked to launch.
func withinLease(e modelEntry, now time.Time, lease time.Duration) bool {
	if e.At.IsZero() || lease <= 0 {
		return false
	}
	age := now.Sub(e.At)
	return age >= 0 && age < lease
}

// catalogAge is how old the retained reading is, or 0 when there is nothing
// datable to be old: a snapshot with no `at` is not a snapshot from the
// epoch, and neither is one written by a clock ahead of ours.
func catalogAge(e modelEntry, now time.Time) time.Duration {
	if e.At.IsZero() || now.Before(e.At) {
		return 0
	}
	return now.Sub(e.At)
}

// catalogRead names the retained reading the way the notice needs it, with
// the age when there is one — the age is the operator's whole decision:
// minutes is a blip, a day is a gate that has been refusing since yesterday.
func catalogRead(age time.Duration) string {
	if age <= 0 {
		return "the last catalog reading"
	}
	return "the catalog read " + BlindFor(age) + " ago"
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
// The default is `strong` → `standard`, which on claude is fable-5-1 →
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
	Line    string // the loud line, "" = nothing to say
	// Unknown: the line states an UNKNOWN verdict rather than a
	// substitution (ADR 0039 D3c). It prints — the operator has to hear
	// that the launch is going ahead on an id the newest reading does not
	// list — but NOTHING FELL, so the pair is unmoved and the session meta
	// gets no `fallback:` mark to carry into relaunches.
	Unknown bool
}

// Fell reports whether the pair moved. An UNKNOWN line is not a fall: it
// says posse could not check, which is the one thing that never moves a
// launch.
func (p Preflight) Fell() bool { return p.Line != "" && !p.Unknown }

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
	return a.TierPreflightOn(a.ReadCatalog(errw), persona, runtime, tier)
}

// TierPreflightOn is the same check over a catalog reading the caller
// already holds — the seam a report that rules on many pairs needs, so one
// reading answers all of them (ModelCatalog).
func (a *App) TierPreflightOn(cat *ModelCatalog, persona, runtime, tier string) Preflight {
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
	// catalog is Anthropic's — OnModelCatalog below is what keeps those ids
	// from being checked against a list that will never hold them).
	if p.Wanted == "" || !a.ModelPreflight() || !rt.OnModelCatalog() {
		return p
	}
	if cat.has(p.Wanted) {
		return p
	}
	if !cat.known() {
		// The lease rule (ADR 0039 D3c). posse does not know, so nothing is
		// substituted — that is rule (3) and the fail-open asymmetry at the
		// top of the file. With no reading at all there is nothing to say
		// and this branch stays as silent as it has always been. With a
		// reading past its LEASE there is: it is the newest fact anyone has,
		// it does not list the wanted id, and the operator has to hear that
		// the launch is going ahead anyway — that line is the whole price of
		// the ruling, paid once per launch until the probe comes back.
		if cat.retained() {
			p.Unknown = true
			p.Line = fmt.Sprintf("%s — not in %s%s; availability UNKNOWN, launching as asked",
				preflightWants(persona, tier, p.Wanted), catalogRead(cat.age()), cat.probeTail())
		}
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
		case got == "" || !rt2.OnModelCatalog():
			// Off the catalog posse can read: the hop is taken and stated,
			// and what that runtime serves is its own business.
			clauses = append(clauses, "falling back to "+hopDesc(runtime, curRT, curTier, got))
			landed = true
		case cat.has(got):
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
	// No age clause on this verdict: an "unavailable" only ever rests on a
	// reading inside its lease now (D3c), which is the operator's own
	// freshness number, and the reading that would have needed dating no
	// longer reaches a verdict at all — it prints the UNKNOWN line above.
	p.Line = fmt.Sprintf("%s — unavailable, %s", preflightWants(persona, tier, p.Wanted), strings.Join(clauses, ", "))
	return p
}

// preflightWants is the clause every availability line opens with: WHO the
// line is about and WHAT the pair it names asks for. It is a function and
// not three literals because a carried mark is checked against it a launch
// later (CarriedMark) — a check spelled out by hand would go on passing
// while the sentence it is checking drifted away from it.
func preflightWants(persona, tier, model string) string {
	if model == "" {
		// Reachable only from CarriedMark, asking about a PID whose own
		// runtime maps no model for its tier: the producers above return
		// before rendering a line when Wanted is empty.
		return fmt.Sprintf("%s: tier %s wants the runtime's own default model", preflightWho(persona), tier)
	}
	return fmt.Sprintf("%s: tier %s wants %s", preflightWho(persona), tier, model)
}

// preflightWho names the launch a line is about: a persona, or the session
// itself when there is no PID (`posse runtimes` asks with "").
func preflightWho(persona string) string {
	if persona == "" {
		return "session"
	}
	return persona
}

// CarriedMark is what an availability mark a session is ALREADY wearing
// should say at its next launch (ranger-base-cplx).
//
// ranger-base-twaq made the mark ride `posse relaunch`, because the fact it
// states — this session is not running the pair its PID names — survives a
// refresh, and blanking it would drop the ⤵️fallback tag, the receipt's
// FALLBACK: line and dispatch's effectiveTier answer all at once. What rode
// with the fact was the SENTENCE, and that sentence names the tier and the
// model the PID asked for AT THE FALL. Edit `tier:` to a THIRD value —
// neither what fell nor what is running — and both clauses stop describing
// this PID while the fact stays true: the one surface an operator reads to
// decide whether to act says "tier strong wants claude-fable-5-1" about a
// PID that asks fast.
//
// So the mark is carried verbatim only while it still opens with what this
// PID asks TODAY, and is otherwise re-derived from today's PID and the pair
// this launch really runs. It is never emptied here. Dropping it would take
// the two halves that are still RIGHT down with the stale explanation, and
// the third-tier board is exactly where they are load-bearing: a session on
// claude-opus-5 whose PID says fast is one dispatch would otherwise tell it
// is thinking at fast. The one case where the mark is dropped is the pair
// no longer differing from the PID's own at all, and that is twaq's own
// condition, upstream of this call (herdrback.go planLaunch).
//
// runtime/tier are the pair the launch will really open on, which on this
// path is the pair the session already runs.
func (a *App) CarriedMark(ag *AgentFile, persona, mark, runtime, tier string) string {
	if mark == "" {
		return ""
	}
	own, ownTier := a.ResolveRuntime("", ag), a.ResolveTier("", ag)
	ownRT, err := a.LoadRuntime(own)
	if err != nil {
		// The PID names a runtime that will not load. Nothing here can say
		// what it asks for, and that is a reason to render no new sentence —
		// not a reason to un-say the old fact. Carried as it was, which is
		// what every launch before this did.
		return mark
	}
	wants := ownRT.Model(ownTier)
	// The separator both producers put after the clause is part of the
	// check: without it a model id that is a PREFIX of the one the mark
	// names reads as the same ask, and a runtime overlay rolling
	// `model_strong:` from claude-fable-5-1 back to claude-fable-5 would
	// carry a mark naming the id nothing asks for any more. It is also what
	// makes the check notice if either line above stops opening this way.
	if strings.HasPrefix(mark, preflightWants(persona, ownTier, wants)+" — ") {
		return mark
	}
	got := ""
	if rt, err := a.LoadRuntime(runtime); err == nil {
		got = rt.Model(tier)
	}
	// hopDesc for the tail, so a hopped session reads "… on codex" here
	// exactly as it did in the line it fell with.
	return fmt.Sprintf("%s — this session is running %s from an earlier fall", preflightWants(persona, ownTier, wants), hopDesc(own, runtime, tier, got))
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
	return a.PreflightReportOn(a.ReadCatalog(errw), persona, runtime, tier)
}

// PreflightReportOn is the same report over a catalog reading the caller
// already holds. `posse runtimes` prints a line per mapped tier per
// runtime and `posse gates` one per runtime: one reading answers them all,
// and on a home whose probe is down that is the difference between one
// request and one per line.
func (a *App) PreflightReportOn(cat *ModelCatalog, persona, runtime, tier string) string {
	rt, err := a.LoadRuntime(runtime)
	if err != nil {
		return ""
	}
	want := rt.Model(tier)
	// Its own lines lead with the runtime, because that is what they
	// describe. The fallback branch does not: it returns the LAUNCH's line
	// verbatim, persona and all, so the operator reads here the same bytes
	// a launch would print — one rendering, no drift.
	//
	// Every branch above the catalog is answered without touching it: the
	// reading is lazy, so a report on a runtime posse knows no catalog for
	// asks nobody.
	switch {
	case want == "":
		return fmt.Sprintf("%s: tier %s → this runtime maps no model; the CLI picks its own", runtime, tier)
	case !a.ModelPreflight():
		return fmt.Sprintf("%s: tier %s → %s (preflight off: config model_preflight: false)", runtime, tier, want)
	case !rt.OnModelCatalog():
		return fmt.Sprintf("%s: tier %s → %s (no model catalog posse can read for this runtime)", runtime, tier, want)
	}
	if !cat.known() {
		switch {
		case !cat.retained():
			// Nothing was read, so there is no reading to date. The read's
			// own outcome stays where it was — state/model-catalog.log, and
			// stderr for the one failure that is posse's own doing
			// (noteGateRefusal) — because this line is printed once per
			// mapped TIER and the outcome belongs to the reading: `posse
			// runtimes` on a home with no snapshot would print the same
			// sentence three times under one runtime.
			return fmt.Sprintf("%s: tier %s → %s (availability UNKNOWN — the catalog could not be read; the launch takes the tier as asked)", runtime, tier, want)
		case !cat.has(want):
			// Past its lease AND missing the id: the launch's own line,
			// verbatim, for the same reason the fallback branch below
			// returns it — one rendering of the sentence that matters most.
			return a.TierPreflightOn(cat, persona, runtime, tier).Line
		}
		// Past its lease and the id IS on it. That is not "available": it is
		// the reason the operator is reading this command, so the line dates
		// the reading and names the probe rather than quietly reporting a
		// verdict posse is no longer entitled to.
		return fmt.Sprintf("%s: tier %s → %s (availability UNKNOWN — %s is past model_probe_ttl%s; the launch takes the tier as asked)",
			runtime, tier, want, catalogRead(cat.age()), cat.probeTail())
	}
	if cat.has(want) {
		return fmt.Sprintf("%s: tier %s → %s (available)", runtime, tier, want)
	}
	return a.TierPreflightOn(cat, persona, runtime, tier).Line
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

// ModelCatalog is ONE reading of the account's catalog, ruled on as many
// times as a report needs. `posse runtimes` and `posse gates` ask about
// every mapped tier of every runtime on this API; taking the reading per
// pair asks the endpoint per pair the moment the snapshot cannot be
// refreshed — a failed read stores nothing, so nothing shares it — and
// writes a state/model-catalog.log line for each. The snapshot file shares
// a SUCCESSFUL reading between processes; this shares any reading, failure
// included, inside one command.
//
// Lazy on purpose: a report over runtimes that are none of them on this
// API (or with the preflight off) must ask nobody, and the branch that
// decides that is per runtime, downstream of here.
type ModelCatalog struct {
	a     *App
	fresh bool // --probe: maxAge 0, "fresh only"
	errw  io.Writer

	once  sync.Once
	set   map[string]bool
	ok    bool
	read  CatalogRead
	lease time.Duration // model_probe_ttl: how long one reading rules unremarked
}

// ReadCatalog is the ordinary reading: whatever the shared snapshot holds
// inside `model_probe_ttl`, else a request.
func (a *App) ReadCatalog(errw io.Writer) *ModelCatalog {
	return &ModelCatalog{a: a, errw: errw}
}

// ProbeCatalog is `posse runtimes --probe`: read it NOW. maxAge 0 is
// "fresh only", the meaning plancache.go already gives it — and Models
// checks the RetryAt cooldown before asking, so a forced read cannot
// become the rangerhq-tdy8 storm (ADR 0039 D3b).
func (a *App) ProbeCatalog(errw io.Writer) *ModelCatalog {
	return &ModelCatalog{a: a, errw: errw, fresh: true}
}

func (c *ModelCatalog) load() {
	c.once.Do(func() {
		c.lease = c.a.ModelProbeTTL(c.errw)
		maxAge := c.lease
		if c.fresh {
			maxAge = 0
		}
		mc := c.a.ModelCache()
		mc.Errw = c.errw
		ids, ok, r := mc.ModelsRead(maxAge, c.lease)
		c.ok, c.read = ok, r
		c.set = make(map[string]bool, len(ids))
		for _, id := range ids {
			c.set[id] = true
		}
	})
}

// known: may a verdict rest on this reading? False is "posse does not
// know" — either nothing was read at all, or what was read is past its
// lease (D3c) — and nothing may treat it as "unavailable".
func (c *ModelCatalog) known() bool { c.load(); return c.ok }

// retained: is there a reading to QUOTE, whether or not it may rule? Past
// its lease a reading is still the newest fact anyone has, and the UNKNOWN
// line names it and its age rather than saying nothing (ADR 0039 D3a/D3c).
func (c *ModelCatalog) retained() bool { c.load(); return len(c.set) > 0 }

// has: is this id on the reading? What it means depends on known(): a
// verdict inside the lease, and only a quotation past it.
func (c *ModelCatalog) has(id string) bool { c.load(); return c.set[id] }

// age of the reading being quoted; 0 when there is nothing datable to be
// old (catalogRead says it in words).
func (c *ModelCatalog) age() time.Duration { c.load(); return c.read.Age }

// probeTail is what the refresh that would have replaced this reading said,
// as a clause to hang off the reading's own name (ADR 0039 D3a). "" when
// nothing was asked — a cooldown, say — because a read that never happened
// must not be reported as a failing probe; the 429 that stopped it is in
// state/model-catalog.log. An operator reading `not in the catalog read 2d
// ago and the probe is failing (401 Unauthorized)` knows to refresh a
// credential; one reading `unavailable` learns the wrong thing from a true
// sentence, which is the incident this whole clause comes from.
func (c *ModelCatalog) probeTail() string {
	c.load()
	if c.read.Err == nil {
		return ""
	}
	return " and the probe is failing (" + c.read.Err.Error() + ")"
}

// OnModelCatalog: are this runtime's model ids ids the catalog above lists?
// Asked of the runtime's own `egress:` (ADR 0002 §4) rather than of its
// name, so a template-only runtime pointed at the same API is covered and a
// built-in that moves is not miscategorised by a stale name check.
func (rt *Runtime) OnModelCatalog() bool {
	for _, h := range rt.Egress {
		if h == ModelListHost {
			return true
		}
	}
	return false
}
