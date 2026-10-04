//go:build !posse_arm2 && !posse_arm3

package posse

// The seed config (examples/config.yaml) is what `posse init` copies verbatim
// into every fresh instance, so it is the de facto instance spec — and the
// highest-leverage place for one of this instance's facts to escape into
// everyone else's (ADR 0012 Appendix A 4). Four properties are worth a test
// rather than a reviewer's eye:
//
//  1. It ARMS NOTHING. Every key that changes dispatch's behaviour ships
//     commented out, so a fresh instance's first pass does what the harness
//     defaults do and the deployer turns things on deliberately. Uncommenting
//     one key by accident while editing this file is the regression.
//  2. It names no machine. An absolute home path in the seed is an instance
//     fact that would follow the file onto every other laptop.
//  3. Every key it DOES ship armed is read by something. An armed key no
//     code looks up is documentation of a feature that is not there
//     (ranger-base-aox).
//  4. Every key it documents with a VALUE documents the real one. A
//     commented line carrying a number is a claim about a constant, and a
//     claim nothing reads goes stale for free — change the constant, keep
//     the suite green, and a fresh instance's spec names the old number
//     (ranger-base-vofbl). See the block at the foot of this file.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func seedConfigPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(qibRepoRoot(t), "examples", "config.yaml")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("no seed config at %s: %v", p, err)
	}
	return p
}

// The keys the seed is allowed to set live, and nothing else: session
// cosmetics. Everything with teeth — routing, tiering, ceilings, the arm
// switch — is documented in comments and unset.
func TestSeedConfigArmsNothing(t *testing.T) {
	t.Parallel()
	cfg := seedConfigPath(t)

	for _, key := range []string{
		"operator", "coordinator", "default_persona",
		// instance: renames every workspace this home creates in herdr's one
		// shared list (rangerhq-ouf9). A seed that shipped it set would tag a
		// single-instance fleet for a coexistence it does not have.
		"instance",
		"default_runtime", "default_tier", "default_engine", "cage_image",
		"verify_assignee",
		// verify_batch: is the gate's ratio and therefore the operator's
		// call (ranger-base-bah7 decision 2): a seed that shipped N>1 would
		// change how much gets verified per bead without anyone deciding to.
		"verify_batch", "verify_batch_age",
		"plan_guard_5h", "plan_guard_7d", "plan_guard_blind_max",
		"budget_pass", "budget_day",
		// dispatch_epoch: denominates both of the caps above and
		// autostart_max_beads: below (ADR 0028 §2). A seed that shipped it
		// set would change how much spend authority and how many launches a
		// fresh instance gets per unit time without anyone deciding to.
		"dispatch_epoch",
		"autostart_interval", "autostart_max_interval", "autostart_max_beads",
		"autostart_dry_run", "autostart_resume", "autostart_session", "autostart_dir",
	} {
		if v := YamlGet(cfg, key); v != "" {
			t.Errorf("seed config sets %s: %q — the seed must ship it commented out", key, v)
		}
	}

	// Keys whose *presence* is the switch (a present-but-empty key means
	// something different from an absent one), so absence is the assertion.
	for _, key := range []string{"beads", "orientation", "verify_labels", "tier_by_label", "metric_ids"} {
		if yamlHasKey(cfg, key) {
			t.Errorf("seed config declares %s: — a present key overrides the harness default; keep it commented", key)
		}
	}

	// autostart_interval's presence alone is the arm switch, so it gets the
	// stronger check: not merely empty, absent. A present-but-empty key is
	// not the middle ground it looks like — the hook refuses it by name and
	// exits 1 rather than reading it as a disarm (ranger-base-cxyk), so a
	// seed that declared it bare would ship a fresh instance whose herdr
	// startup hook fails, not one that is disarmed.
	if yamlHasKey(cfg, "autostart_interval") {
		t.Error("seed config declares autostart_interval: — presence is the arm switch; a fresh instance must ship disarmed")
	}
}

// The emoji map is the seed's live content; it must still work, and still
// name nobody's machine.
func TestSeedConfigNamesNoMachine(t *testing.T) {
	t.Parallel()
	cfg := seedConfigPath(t)

	if len(YamlMapPairs(cfg, "emoji")) == 0 {
		t.Error("seed config has no emoji: map")
	}

	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, abs := range []string{"/Users/", "/home/", "/var/folders/"} {
		if strings.Contains(string(b), abs) {
			t.Errorf("seed config contains the absolute path prefix %q — paths here must be ~-relative or a placeholder", abs)
		}
	}
}

// The keys a fresh instance is allowed to ship SET. Shared by the two tests
// below: one says nothing outside this set is declared, the other says
// everything in it is read by the harness.
var seedLiveKeys = map[string]bool{
	"default_dir":   true, // session cosmetics — no dispatch behaviour
	"default_env":   true,
	"default_emoji": true,
	"emoji":         true, // session name → glyph
}

// The two lists above are hand-maintained, which is the failure mode they
// were meant to prevent: a key added to the seed by a later bead is armable
// and silently uncovered. `plan_usage_ttl:` and `beads_visibility:` both
// arrived that way and could be shipped armed with the suite green
// (rangerhq-fpv9). So the real guard is inverted — enumerate the handful of
// keys the seed is ALLOWED to declare, and let everything else fail by
// default. A new key then has exactly two ways past this test: ship it
// commented out, or say here, on purpose, that a fresh instance sets it.
func TestSeedConfigDeclaresOnlyTheLiveKeys(t *testing.T) {
	t.Parallel()
	cfg := seedConfigPath(t)

	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A declared key is one at column 0: `# key:` is documentation, `key:`
	// is configuration. That is the same rule YamlGet/yamlHasKey apply, so
	// this test sees exactly what the harness sees.
	for _, ln := range strings.Split(string(b), "\n") {
		i := strings.Index(ln, ":")
		if i <= 0 || ln[0] == ' ' || ln[0] == '\t' || ln[0] == '#' {
			continue
		}
		key := ln[:i]
		if strings.ContainsAny(key, " \t") {
			continue
		}
		if !seedLiveKeys[key] {
			t.Errorf("seed config declares %s: — a fresh instance must ship it commented out, "+
				"or add it to `seedLiveKeys` and say why arming it by default is right", key)
		}
	}
}

// The other half of the allowlist, and the one `dirs:` failed for two years
// (ranger-base-aox). Being on `seedLiveKeys` says a fresh instance ships this
// key ARMED; that is only defensible if something then reads it. `dirs:` was
// seeded with three roots and a comment describing a TUI directory picker
// this branch does not have — the picker belongs to the tmux-era launcher —
// so every fresh instance was armed with a key no code has ever looked up,
// and both tests above blessed it. It is deleted; this pin is what keeps the
// next one from arriving the same way.
//
// The check is the bead's own repro promoted to an assertion: the harness
// reads config through YamlGet/YamlList/YamlMapPairs/CfgGet, all keyed by a
// string literal, so a live key with no `"key"` literal in any non-test Go
// file is read by nothing. A matching literal is necessary, not sufficient —
// this cannot tell a config read from a same-named recipe field — but the
// failure it catches is total absence, which is the failure that happened.
// Test files are excluded on purpose: a key whose only reader is the test
// asserting it has one would satisfy a scan that included them.
func TestSeedConfigLiveKeysAreRead(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not a hand-rolled `..`/`..` ascent: this walk starts at
	// the repo root, which puts the test in the tree-wide class the door
	// census derives, and that census can only see tests that reach the
	// root through the one helper (ranger-base-sx2dq).
	root := qibRepoRoot(t)

	var src strings.Builder
	scanned := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		src.Write(b)
		scanned++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Positive witness: an empty or mis-rooted walk would pass every
	// assertion below by measuring nothing.
	if scanned < 20 {
		t.Fatalf("scanned %d non-test .go files under %s — the walk found no tree, so the check below measures nothing", scanned, root)
	}
	t.Logf("scanned %d non-test .go files under %s", scanned, root)

	all := src.String()
	for key := range seedLiveKeys {
		if !strings.Contains(all, `"`+key+`"`) {
			t.Errorf("seed config ships %s: armed, but no non-test Go file names %q — "+
				"a key nothing reads must not be in the seed (delete it, or delete it from seedLiveKeys)", key, key)
		}
	}
}

// ───────────────────────────────────────────────────────────────────────────
// The DOCUMENTED DEFAULTS (ranger-base-vofbl).
//
// Property 4, and the one none of the three above can see. The seed ships
// every key with teeth commented out (property 1), and a commented line that
// carries a value is making a claim: `# attn_parked_age: 336h` says "336h is
// what you get if you leave this alone". Nothing read that claim. Change
// DefaultAttnParkedAge and the whole suite stays green while the
// operator-facing document keeps naming the old number — N copies of one
// value with zero of the N-1 edges held.
//
// WHAT IS HELD AND WHAT IS NOT. The bead sorted each literal on such a line
// into coupling-or-claim and only the coupling is pinned:
//
//   - the VALUE (`336h`, `4h`, `2h`, `26h`) is the constant said twice, and
//     that is this pin. It is compared as a DURATION and never as text,
//     because `336h` and `1209600` are the same default and `14d` is not a
//     default at all.
//   - the PROSE beside it (`= 14d, the default`) is NOT pinned. Pinning
//     English is how a file ends up carrying careful sentences about code
//     that left; the value one column over is the thing a machine can hold.
//   - the key being COMMENTED OUT is load-bearing and stays that way.
//     attnAge reads the live file through YamlGet, so uncommenting one to
//     make it readable would change every instance's behaviour. This pin
//     reads the comment, which is why it can hold a key it must not arm.
//
// WHY THE PAIRING IS DERIVED AND NOT A MAP. "Every documented default
// matches its constant" is not a rule over the whole seed file — most
// documented values there are illustrations (`verify_batch: 4`,
// `budget_pass: 30`, `grok_guard_week: 85`), and only the prose says which
// is which, so a key->constant map would be a hand list with a silent hole
// for the next key to fall into. But the harness already states the pairing
// where it cannot drift, in one of exactly two shapes, and the census is
// parsed out of both:
//
//   - the CALL-SITE form. A duration key read through a shared reader names
//     its key literal and its Default* constant in the same argument list:
//     `a.attnAge("attn_parked_age", DefaultAttnParkedAge, errw)`.
//   - the BODY form. A duration key with a reader of its own spells both in
//     that reader's body: `YamlGet(a.ConfigPath, "plan_usage_ttl")` one line
//     and `return PlanUsageTTLDefault` the next. Six of the ten duration
//     keys the seed documents are read this way, and the census missed all
//     six until ranger-base-khqvr — the rule was a hand-written list of two
//     reader NAMES, which is the shape the paragraph above warns about, one
//     level up (ranger-base-2vynj finding 1; MEASURED there, one mutant per
//     documented line, all six SURVIVED against two derived controls that
//     died).
//
// Both shapes are derived structurally, and the three hand-written tables
// below are each made TOTAL by the derivation — a reader, a constant or an
// unpairable key the tree has and a table does not reds this pin by name,
// which is the property the old rule did not have:
//
//   - seedDurationResolvers: call-site readers, because a test cannot call
//     an unexported method it only parsed.
//   - seedDurationBodyReaders: body-form readers, same reason.
//   - seedDurationDefaults: constant NAME -> value, because a test cannot
//     evaluate a constant it only parsed. For a body-form reader the value
//     is cross-checked against what the reader itself answers over a config
//     with the key absent, so a mis-parsed pairing cannot sit there green.
//
// A CONSTANT IS `Default*` OR `*Default`. Both spellings are live in this
// tree — `DefaultAttnParkedAge` and `PlanUsageTTLDefault` — and the prefix
// test alone was the second half of the escape above: four of the six keys
// name their constant with the suffix, so they failed both halves of the old
// rule at once.
//
// THE REGISTERS are two, and each silences one key for a stated reason:
//
//   - seedDocumentedOnPurposeNotTheDefault, for a key the file documents at
//     something other than its default on purpose. Empty today.
//   - seedDurationDefaultNotAConstant, for a key whose default is a RULE and
//     not a constant, which this pin cannot compare against anything.
//     `backup_max_age` is the one (ADR 0036 §6). Such a key must also stay
//     UNDOCUMENTED in the seed, because a documented line for it would be a
//     claim nothing here can hold.
//
// Each register's stale half is as loud as its live half: an entry whose key
// is no longer a duration key, is no longer documented, or whose documented
// value has quietly become the default again, fails.
//
// WHAT IS STILL OUTSIDE, on purpose. `autostart_interval` has no default at
// all — its presence is the arm switch and absent means off (govern.go) —
// and `autostart_max_interval` defaults to 8x the base rather than to a
// constant. Neither is read through a reader of this shape, so neither is
// derived, and neither should be: a register row for a key with no constant
// to drift from is the wrong answer. MEASURED 2026-10-04: the body-form rule
// scans `YamlGet` and `CfgGet` key literals alike and the two keys above are
// read through neither inside a duration reader, so widening the key source
// adds no member and closes the escape a future CfgGet-based reader would
// otherwise have.

// seedDurationResolvers are the config readers that take the key and the
// default as ARGUMENTS: the functions whose grammar decides what a
// documented value MEANS. The pin resolves through the real function rather
// than reimplementing either grammar, so `336h` vs `1209600` vs `14d` is
// settled by the code the instance runs and not by a second parser in a
// test. Made total by seedDurationKeyCensus, which derives the same set from
// the tree's function signatures and fails on one this map does not name.
var seedDurationResolvers = map[string]func(*App, string, time.Duration, io.Writer) time.Duration{
	"attnAge":    (*App).attnAge,
	"graceAfter": (*App).graceAfter,
}

// seedDurationBodyReaders are the config readers that spell their key and
// their default in their OWN body — one `YamlGet`/`CfgGet` key literal, one
// `Default`-shaped constant returned. Resolving through them is the same
// claim as resolving through a call-site reader: the duration a fresh
// instance would get for the documented text, computed by the instance's own
// code. Made total the same way.
var seedDurationBodyReaders = map[string]func(*App, io.Writer) time.Duration{
	"DispatchEpoch":       (*App).DispatchEpoch,
	"ModelProbeTTL":       (*App).ModelProbeTTL,
	"PlanGuardBlindMax":   (*App).PlanGuardBlindMax,
	"PlanUsageStaleAfter": (*App).PlanUsageStaleAfter,
	"PlanUsageTTL":        (*App).PlanUsageTTL,
	"verifyBatchAge":      (*App).verifyBatchAge,
}

// seedDurationDefaults is the Default-shaped constant NAME -> its value.
// Hand written, because a test cannot evaluate a constant it only parsed —
// and made total by the derivation in seedDurationKeyCensus, which fails on
// a constant it finds at a reader and cannot find here, and on an entry here
// no reader names any more.
var seedDurationDefaults = map[string]time.Duration{
	"DefaultAttnQuestionAge":     DefaultAttnQuestionAge,
	"DefaultAttnGuardStuck":      DefaultAttnGuardStuck,
	"DefaultAttnParkedAge":       DefaultAttnParkedAge,
	"DefaultVerifyBoxMaxAge":     DefaultVerifyBoxMaxAge,
	"DefaultCrewReapAfter":       DefaultCrewReapAfter,
	"DefaultUnpointedReapAfter":  DefaultUnpointedReapAfter,
	"DefaultRetireTreeAfter":     DefaultRetireTreeAfter,
	"DefaultVerifyBatchAge":      DefaultVerifyBatchAge,
	"DefaultDispatchEpoch":       DefaultDispatchEpoch,
	"ModelProbeTTLDefault":       ModelProbeTTLDefault,
	"PlanGuardBlindMaxDefault":   PlanGuardBlindMaxDefault,
	"PlanUsageTTLDefault":        PlanUsageTTLDefault,
	"PlanUsageStaleAfterDefault": PlanUsageStaleAfterDefault,
}

// seedDocumentedOnPurposeNotTheDefault records a duration key that
// examples/config.yaml documents at a value that is deliberately NOT the
// code default — a suggested setting rather than a statement about the
// harness. Key -> why, and the why is the whole point of the entry.
//
// Empty on purpose: every duration key the seed documents today documents
// its own default. An entry here is a claim with two halves, and
// TestSeedConfigDocumentedDefaultRegisterIsNotStale holds both.
var seedDocumentedOnPurposeNotTheDefault = map[string]string{}

// seedDurationDefaultNotAConstant records a duration key whose default is a
// RULE and not a constant, so there is no number for a documented line to
// agree with. Key -> why.
//
// Without this register such a key drops out of the census in silence, which
// is the third way `backup_max_age` was invisible to the old rule
// (ranger-base-2vynj finding 1): its key literal is there, but the default
// argument is a CALL. The entry makes the drop deliberate and keeps the
// stronger half available — a key in here must stay UNDOCUMENTED in the
// seed, because a documented line for it would be a claim this pin cannot
// hold.
var seedDurationDefaultNotAConstant = map[string]string{
	"backup_max_age": "ADR 0036 §6 makes the default a rule rather than a number — 2x the scheduled " +
		"backup interval, falling back to DefaultBackupMaxAge only for an instance with no schedule " +
		"(defaultBackupMaxAge) — so there is no single constant a documented line could name",
}

// seedReaderForm is which of the two shapes a pairing was derived from.
type seedReaderForm int

const (
	// seedFormCallSite: the key literal and the Default-shaped constant are
	// the first two arguments of a call to a seedDurationResolvers reader.
	seedFormCallSite seedReaderForm = iota
	// seedFormBody: they are a YamlGet/CfgGet key literal and a returned
	// Default-shaped identifier inside one seedDurationBodyReaders function.
	seedFormBody
)

func (f seedReaderForm) String() string {
	if f == seedFormBody {
		return "body-form reader"
	}
	return "call site"
}

// seedDurationKey is one derived pairing: a config key, the Default-shaped
// constant it falls back to, and the reader whose grammar resolves it.
type seedDurationKey struct {
	key      string
	constant string
	reader   string
	form     seedReaderForm
	site     string
}

// seedUnpairedKey is a duration key the tree reads but this pin cannot pair
// with a constant — the default is a call, a computation, or absent. It is
// reported rather than skipped, because a silent skip is the defect
// ranger-base-khqvr was filed for.
type seedUnpairedKey struct {
	key  string
	site string
	why  string
}

// seedDurationCensus is everything the tree says about duration config keys.
// The three name slices are what make the hand tables total.
type seedDurationCensus struct {
	keys      []seedDurationKey // derived pairings, sorted by key
	resolvers []string          // call-site reader names found in the tree
	readers   []string          // body-form reader names found in the tree
	unpaired  []seedUnpairedKey // key sites with no constant to compare
	scanned   int
}

// seedIsTimeDuration reports whether an AST type expression is time.Duration.
func seedIsTimeDuration(e ast.Expr) bool { return seedIsQualified(e, "time", "Duration") }

// seedIsIOWriter reports whether an AST type expression is io.Writer.
func seedIsIOWriter(e ast.Expr) bool { return seedIsQualified(e, "io", "Writer") }

func seedIsQualified(e ast.Expr, pkg, name string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := s.X.(*ast.Ident)
	return ok && x.Name == pkg && s.Sel.Name == name
}

func seedIsIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

// seedDefaultShaped is the constant-name rule: a prefix OR a suffix, because
// both spellings are live in this tree and the prefix alone hid four keys
// (ranger-base-2vynj finding 1 (b)).
func seedDefaultShaped(name string) bool {
	return strings.HasPrefix(name, "Default") || strings.HasSuffix(name, "Default")
}

// seedFieldTypes flattens a parameter or result list to one type per name,
// so `(a, b string)` counts as two.
func seedFieldTypes(fl *ast.FieldList) []ast.Expr {
	var out []ast.Expr
	if fl == nil {
		return out
	}
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, f.Type)
		}
	}
	return out
}

// seedFirstParamName is the name of a function's first parameter, or "".
func seedFirstParamName(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 || len(fl.List[0].Names) == 0 {
		return ""
	}
	return fl.List[0].Names[0].Name
}

// seedRecvIsApp reports whether a FuncDecl is a method on App or *App.
func seedRecvIsApp(fd *ast.FuncDecl) bool {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return false
	}
	if st, ok := fd.Recv.List[0].Type.(*ast.StarExpr); ok {
		return seedIsIdent(st.X, "App")
	}
	return seedIsIdent(fd.Recv.List[0].Type, "App")
}

// seedCalleeName is the bare function or method name a call expression names.
func seedCalleeName(e ast.Expr) string {
	switch f := e.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// seedConfigKeyArg is the index of the config KEY argument for the two flat
// YAML readers, and -1 for anything else. The two disagree about position —
// `YamlGet(path, key)` and `a.CfgGet(key, fallback)` — which is exactly the
// kind of detail a rule stated in prose gets wrong.
func seedConfigKeyArg(callee string) int {
	switch callee {
	case "YamlGet":
		return 1
	case "CfgGet":
		return 0
	}
	return -1
}

// seedUniqSorted dedupes and sorts, so a census reports the same list twice
// running.
func seedUniqSorted(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// seedDurationReaderDecl reads one function declaration and says which of
// the two reader shapes it is, if either.
//
// Split out from the walk so TestSeedDurationDerivationCanStillSayNo can
// drive it over planted source: a derivation whose near-misses are never
// exercised is a derivation nobody has seen refuse anything.
func seedDurationReaderDecl(fd *ast.FuncDecl) (callSiteResolver bool, body []seedDurationKey, unpaired []seedUnpairedKey) {
	if fd.Body == nil || !seedRecvIsApp(fd) {
		return false, nil, nil
	}
	res := seedFieldTypes(fd.Type.Results)
	if len(res) != 1 || !seedIsTimeDuration(res[0]) {
		return false, nil, nil
	}
	params := seedFieldTypes(fd.Type.Params)

	// What the body does with config: the key literals it reads, and the
	// parameter names it passes to a reader instead.
	var litKeys, paramKeys, retDefaults []string
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			ki := seedConfigKeyArg(seedCalleeName(node.Fun))
			if ki < 0 || len(node.Args) != 2 {
				return true
			}
			switch a := node.Args[ki].(type) {
			case *ast.BasicLit:
				if a.Kind == token.STRING {
					if s, err := strconv.Unquote(a.Value); err == nil && s != "" {
						litKeys = append(litKeys, s)
					}
				}
			case *ast.Ident:
				paramKeys = append(paramKeys, a.Name)
			}
		case *ast.ReturnStmt:
			for _, r := range node.Results {
				if id, ok := r.(*ast.Ident); ok && seedDefaultShaped(id.Name) {
					retDefaults = append(retDefaults, id.Name)
				}
			}
		}
		return true
	})
	litKeys, paramKeys, retDefaults = seedUniqSorted(litKeys), seedUniqSorted(paramKeys), seedUniqSorted(retDefaults)

	// The call-site form: (key string, def time.Duration, errw io.Writer),
	// reading the key it was HANDED. Its own keys live at its call sites.
	if len(params) == 3 && seedIsIdent(params[0], "string") && seedIsTimeDuration(params[1]) && seedIsIOWriter(params[2]) {
		for _, pk := range paramKeys {
			if pk == seedFirstParamName(fd.Type.Params) {
				return true, nil, nil
			}
		}
		return false, nil, nil
	}

	// The body form: (errw io.Writer), spelling its own key and default.
	if len(params) != 1 || !seedIsIOWriter(params[0]) {
		return false, nil, nil
	}
	switch {
	case len(litKeys) == 0:
		// Not a config reader at all, or a thin wrapper that delegates to a
		// call-site reader (AttnParkedAge, VerifyBoxMaxAge, BackupMaxAge).
		// The delegation names the key at the call site, where the other
		// half of the census reads it.
		return false, nil, nil
	case len(litKeys) > 1:
		for _, k := range litKeys {
			unpaired = append(unpaired, seedUnpairedKey{key: k, why: fmt.Sprintf(
				"%s reads %d config keys (%s), so no one constant is its default",
				fd.Name.Name, len(litKeys), strings.Join(litKeys, ", "))})
		}
		return false, nil, unpaired
	case len(retDefaults) != 1:
		named := "none"
		if len(retDefaults) > 0 {
			named = strings.Join(retDefaults, ", ")
		}
		return false, nil, []seedUnpairedKey{{key: litKeys[0], why: fmt.Sprintf(
			"%s reads %s but returns %d Default-shaped constants (%s), so the pairing is not stated in one place",
			fd.Name.Name, litKeys[0], len(retDefaults), named)}}
	}
	return false, []seedDurationKey{{
		key:      litKeys[0],
		constant: retDefaults[0],
		reader:   fd.Name.Name,
		form:     seedFormBody,
	}}, nil
}

// seedDurationKeyCensus parses both pairing shapes out of the tree.
//
// Two passes over one parse, because the call-site pass needs the resolver
// set the declaration pass derives: matching call sites against the DERIVED
// set rather than against seedDurationResolvers is what makes that map total
// instead of a filter with a hole in it.
func seedDurationKeyCensus(t *testing.T, root string) seedDurationCensus {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	var rels []string
	census := seedDurationCensus{}

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			// A tree that does not parse is not this pin's finding — the
			// build says so louder. Skip rather than fail on it.
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			rel = p
		}
		files = append(files, file)
		rels = append(rels, filepath.ToSlash(rel))
		census.scanned++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pass 1: the readers themselves.
	resolvers := map[string]bool{}
	for i, file := range files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			at := fmt.Sprintf("%s:%d", rels[i], fset.Position(fd.Pos()).Line)
			isResolver, body, unpaired := seedDurationReaderDecl(fd)
			if isResolver {
				resolvers[fd.Name.Name] = true
			}
			for _, k := range body {
				k.site = at
				census.keys = append(census.keys, k)
				census.readers = append(census.readers, k.reader)
			}
			for _, u := range unpaired {
				u.site = at
				census.unpaired = append(census.unpaired, u)
			}
		}
	}
	census.resolvers = seedSortedKeys(resolvers)
	census.readers = seedUniqSorted(census.readers)

	// Pass 2: the call sites of every derived call-site reader. A call whose
	// key is a literal states a pairing; one whose default is not a
	// Default-shaped identifier states a key with no constant, which is
	// reported and not dropped.
	for i, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			fn := seedCalleeName(call.Fun)
			if !resolvers[fn] {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			key, uerr := strconv.Unquote(lit.Value)
			if uerr != nil || key == "" {
				return true
			}
			at := fmt.Sprintf("%s:%d", rels[i], fset.Position(call.Pos()).Line)
			id, ok := call.Args[1].(*ast.Ident)
			if !ok || !seedDefaultShaped(id.Name) {
				census.unpaired = append(census.unpaired, seedUnpairedKey{key: key, site: at, why: fmt.Sprintf(
					"the default argument at the %s call site is not a Default-shaped constant", fn)})
				return true
			}
			census.keys = append(census.keys, seedDurationKey{
				key:      key,
				constant: id.Name,
				reader:   fn,
				form:     seedFormCallSite,
				site:     at,
			})
			return true
		})
	}

	sort.Slice(census.keys, func(i, j int) bool {
		if census.keys[i].key != census.keys[j].key {
			return census.keys[i].key < census.keys[j].key
		}
		return census.keys[i].site < census.keys[j].site
	})
	sort.Slice(census.unpaired, func(i, j int) bool {
		if census.unpaired[i].key != census.unpaired[j].key {
			return census.unpaired[i].key < census.unpaired[j].key
		}
		return census.unpaired[i].site < census.unpaired[j].site
	})
	return census
}

// seedDurationKeySites is the census's pairings alone, for the register
// staleness pin, which asks only "is this key still a duration key".
func seedDurationKeySites(t *testing.T, root string) (keys []seedDurationKey, scanned int) {
	t.Helper()
	c := seedDurationKeyCensus(t, root)
	return c.keys, c.scanned
}

// seedResolveFor is the production read of one pairing: a closure that
// answers what a fresh instance would get for a config file, computed by the
// instance's own reader. One closure for both forms, so the drift check has
// a single code path and the grammar is never a test's.
func seedResolveFor(t *testing.T, k seedDurationKey, def time.Duration) func(cfgPath string, errw io.Writer) time.Duration {
	t.Helper()
	if k.form == seedFormBody {
		fn, ok := seedDurationBodyReaders[k.reader]
		if !ok {
			t.Fatalf("no body-form reader %q in seedDurationBodyReaders", k.reader)
		}
		return func(p string, w io.Writer) time.Duration { return fn(&App{ConfigPath: p}, w) }
	}
	fn, ok := seedDurationResolvers[k.reader]
	if !ok {
		t.Fatalf("no resolver %q in seedDurationResolvers", k.reader)
	}
	return func(p string, w io.Writer) time.Duration { return fn(&App{ConfigPath: p}, k.key, def, w) }
}

// seedSortedKeys is the map's keys in order, so a failing census reports the
// same list twice running.
func seedSortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// seedDocumentedValue returns the value examples/config.yaml documents for a
// commented key, and how many commented lines documented it.
//
// A documented line is `#` at column 0, optional blank, then `key:` — the
// mirror of the "declared key is at column 0" rule
// TestSeedConfigDeclaresOnlyTheLiveKeys applies, so prose that merely names
// the key (`# `+"`attn_parked_age:`"+` is the third key...`) is not a hit.
// The value is everything up to the trailing `#` comment, trimmed: on
//
//	# attn_parked_age: 336h      # = 14d, the default: how long a park ...
//
// that is `336h`, and the sentence after the second `#` is deliberately not
// read (see the head comment: the prose is a claim, not a coupling).
func seedDocumentedValue(cfgText, key string) (value string, lines int) {
	for _, ln := range strings.Split(cfgText, "\n") {
		if !strings.HasPrefix(ln, "#") {
			continue
		}
		rest := strings.TrimLeft(ln[1:], " \t")
		if !strings.HasPrefix(rest, key+":") {
			continue
		}
		v := rest[len(key)+1:]
		if i := strings.Index(v, "#"); i >= 0 {
			v = v[:i]
		}
		value = strings.TrimSpace(v)
		lines++
	}
	return value, lines
}

// seedDefaultDrift resolves the text a documented line carries through the
// production reader and says what is wrong with it, or "" when the file and
// the constant agree.
//
// The resolution is a real read of a real config file: the text is written
// out as a LIVE key in a scratch config and handed to the same function the
// instance calls. That is what makes the comparison a duration comparison
// — `1209600` and `336h` reach the same time.Duration, which no text
// compare can see — and it is why the reader's stderr is checked too. A
// value that does not parse makes the reader name it and return the default,
// so a text that is not a duration at all would otherwise pass this pin by
// landing on the very number it was supposed to prove.
//
// `resolve` is the pairing's own reader, from seedResolveFor: both reader
// forms reach here through one closure, so a body-form key is held by the
// same comparison and the same stderr rule as a call-site one.
func seedDefaultDrift(t *testing.T, resolve func(cfgPath string, errw io.Writer) time.Duration, key, text string, def time.Duration) (time.Duration, string) {
	t.Helper()
	if text == "" {
		return 0, "the line carries no value, so it documents nothing"
	}
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte(key+": "+text+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errw strings.Builder
	got := resolve(cfg, &errw)
	if s := strings.TrimSpace(errw.String()); s != "" {
		return got, fmt.Sprintf("%s does not parse as a duration, so the default stands and the line proves nothing: %s", text, s)
	}
	if got != def {
		return got, fmt.Sprintf("documents %s (= %s), the constant is %s", text, got, def)
	}
	return got, ""
}

// Property 4: every duration key examples/config.yaml documents is
// documented at its own default, compared as a duration through the
// harness's own reader.
func TestSeedConfigDocumentedDurationDefaultsAreTheConstants(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not a hand-rolled climb: the tree-wide door census in
	// internal/treepins derives this pin's class from that one helper
	// (ranger-base-sx2dq), and a pin outside the class gets no door.
	root := qibRepoRoot(t)

	c := seedDurationKeyCensus(t, root)
	keys, scanned := c.keys, c.scanned
	// Positive witnesses: a pin over a derived set is satisfied by deriving
	// nothing, so say what was measured and fail a walk that measured too
	// little to mean anything.
	if scanned < 20 {
		t.Fatalf("parsed %d non-test .go files under %s — the walk found no tree, so the census below measures nothing", scanned, root)
	}
	t.Logf("parsed %d non-test .go files, derived %d duration key/constant pairings through %d call-site reader(s) (%s) and %d body-form reader(s) (%s)",
		scanned, len(keys), len(c.resolvers), strings.Join(c.resolvers, ", "), len(c.readers), strings.Join(c.readers, ", "))

	// THE THREE TABLES ARE TOTAL, which is the property the hand-written
	// two-name resolver list did not have: the escape ranger-base-khqvr was
	// filed for was six keys read by a reader nothing named, dropping out in
	// silence. A reader the tree has and a table does not is now a failure
	// that names the reader.
	for _, got := range []struct {
		kind  string
		found []string
		named []string
		table string
	}{
		{"call-site", c.resolvers, seedSortedKeys(seedDurationResolvers), "seedDurationResolvers"},
		{"body-form", c.readers, seedSortedKeys(seedDurationBodyReaders), "seedDurationBodyReaders"},
	} {
		named := map[string]bool{}
		for _, n := range got.named {
			named[n] = true
		}
		found := map[string]bool{}
		for _, n := range got.found {
			found[n] = true
			if !named[n] {
				t.Errorf("the tree has a %s duration reader %s does not name: %s — add `%q: (*App).%s,` to it, or this pin reads every key that reader owns as undocumented and holds nothing about them (ranger-base-khqvr)",
					got.kind, got.table, n, n, n)
			}
		}
		for _, n := range got.named {
			if !found[n] {
				t.Errorf("%s names %s, which is no longer a %s duration reader in the tree — the entry resolves nothing; drop it", got.table, n, got.kind)
			}
		}
	}

	// The constant table, from the other side: an entry no reader names any
	// more is a value this pin can no longer be wrong about.
	namedConstants := map[string]bool{}
	for _, k := range keys {
		namedConstants[k.constant] = true
	}
	for _, name := range seedSortedKeys(seedDurationDefaults) {
		if !namedConstants[name] {
			t.Errorf("seedDurationDefaults names %s, which no duration reader falls back to any more — the row holds nothing; drop it", name)
		}
	}

	// A key whose default is not a constant cannot be compared with
	// anything, so it is registered with a reason or it is a failure — never
	// a silent drop, which is how backup_max_age left the census
	// (ranger-base-2vynj finding 1).
	unpairedKeys := map[string]bool{}
	for _, u := range c.unpaired {
		unpairedKeys[u.key] = true
		why, registered := seedDurationDefaultNotAConstant[u.key]
		if !registered {
			t.Errorf("%s (%s) is a duration key this pin cannot pair with a constant: %s — give it a constant default, or add it to seedDurationDefaultNotAConstant with the reason, so the drop is on the record instead of silent",
				u.key, u.site, u.why)
			continue
		}
		if strings.TrimSpace(why) == "" {
			t.Errorf("seedDurationDefaultNotAConstant[%q] carries no reason — the entry drops a key out of this census, so the why IS the entry", u.key)
		}
		t.Logf("%s: %s — registered as having no constant default (%s)", u.key, u.why, u.site)
	}
	for _, key := range seedSortedKeys(seedDurationDefaultNotAConstant) {
		if !unpairedKeys[key] {
			t.Errorf("seedDurationDefaultNotAConstant names %q, which no duration reader reads with a non-constant default any more — either it has a constant now (drop the entry, the census will pair it) or the key is gone", key)
		}
	}

	// The floor, below the totality checks on purpose: when a whole reader
	// rule stops matching, the three blocks above name each table entry that
	// no longer resolves and this says how much of the census went with it.
	// A Fatalf, because every comparison below a census this thin is
	// vacuous.
	if len(keys) < 10 {
		t.Fatalf("derived %d duration key/constant pairings from %d files, want at least 10 — the two reader rules in seedDurationReaderDecl have stopped matching the tree, so this pin holds almost nothing", len(keys), scanned)
	}

	// One key, one default. Two call sites agreeing is normal
	// (retire_tree_after is read from two); two naming DIFFERENT constants
	// is a key whose default depends on who asks, and no documented line
	// could be right about both.
	byKey := map[string]seedDurationKey{}
	for _, k := range keys {
		if prev, dup := byKey[k.key]; dup && prev.constant != k.constant {
			t.Errorf("%s falls back to %s at %s and to %s at %s — one key with two defaults, so no documented value can be right about it",
				k.key, prev.constant, prev.site, k.constant, k.site)
			continue
		}
		byKey[k.key] = k
	}

	// The constant table's VALUES are cross-checked where the tree lets them
	// be. A body-form reader handed a config with its own key absent returns
	// its own default, so a pairing the parse got wrong — the right key
	// against the wrong constant — cannot sit in seedDurationDefaults green.
	// A call-site reader cannot be checked this way: its default is the
	// argument, so it answers whatever a test hands it.
	absent := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(absent, []byte("# every key commented out\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if k.form != seedFormBody {
			continue
		}
		def, known := seedDurationDefaults[k.constant]
		if !known {
			continue // already reported below
		}
		var errw strings.Builder
		if got := seedResolveFor(t, k, def)(absent, &errw); got != def {
			t.Errorf("%s over a config with %s absent returns %s, but seedDurationDefaults says %s is %s (%s) — the derivation paired the key with the wrong constant, so every comparison it feeds is against the wrong number",
				k.reader, k.key, got, k.constant, def, k.site)
		}
	}

	cfg := seedConfigPath(t)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

	// The stronger half of seedDurationDefaultNotAConstant: a key with no
	// constant default must also stay UNDOCUMENTED in the seed, because a
	// documented line for it is a claim nothing here can hold — the exact
	// stale-for-free shape this whole block exists to prevent.
	for _, key := range seedSortedKeys(seedDurationDefaultNotAConstant) {
		if value, lines := seedDocumentedValue(text, key); lines > 0 {
			t.Errorf("examples/config.yaml documents %s as %q, and seedDurationDefaultNotAConstant says its default is a rule and not a constant (%s) — so that line is a number this pin cannot hold. Drop the documented value, or give the key a constant default",
				key, value, seedDurationDefaultNotAConstant[key])
		}
	}

	compared := 0
	for _, key := range seedSortedKeys(byKey) {
		k := byKey[key]
		def, known := seedDurationDefaults[k.constant]
		if !known {
			t.Errorf("%s (%s) falls back to %s, which seedDurationDefaults does not name — add `%q: %s,` to it so this pin can compare the seed's documented value against it",
				k.key, k.site, k.constant, k.constant, k.constant)
			continue
		}

		// The key must still be COMMENTED in the seed. The property-1 test
		// above is what fails on an armed key in general; this says it here
		// too, because an armed key would make THIS pin vacuous for it —
		// seedDocumentedValue reads comment lines, so it would find nothing
		// and skip in silence.
		if yamlHasKey(cfg, k.key) {
			t.Errorf("seed config declares %s: live — a duration key must ship commented out (%s reads the live file, so arming it changes every instance), and an armed key is invisible to this pin", k.key, k.reader)
			continue
		}

		value, lines := seedDocumentedValue(text, k.key)
		switch {
		case lines == 0:
			// Undocumented is allowed: a key the seed says nothing about
			// makes no claim that can go stale. Logged, not failed.
			t.Logf("%s: undocumented in examples/config.yaml (default %s) — nothing to hold", k.key, def)
			continue
		case lines > 1:
			t.Errorf("examples/config.yaml documents %s on %d commented lines — two documented values for one key, and a reader cannot tell which is the default", k.key, lines)
			continue
		}

		compared++
		got, drift := seedDefaultDrift(t, seedResolveFor(t, k, def), k.key, value, def)
		if why, deliberate := seedDocumentedOnPurposeNotTheDefault[k.key]; deliberate {
			// The register's live half: the entry says this line is NOT the
			// default, so the line matching the default makes the entry a
			// lie, and the next reader believes the wrong one.
			if drift == "" {
				t.Errorf("examples/config.yaml documents %s at %s, which IS %s (%s) — but seedDocumentedOnPurposeNotTheDefault says it is deliberately not the default (%q). Drop the register entry, or restore the value it describes",
					k.key, value, k.constant, def, why)
			}
			continue
		}
		if drift != "" {
			t.Errorf("examples/config.yaml documents %s as the default and %s says otherwise: %s.\n"+
				"  the line is a claim a fresh instance reads; fix the line, or — if the value is documented at something other than its default on purpose — say so in seedDocumentedOnPurposeNotTheDefault",
				k.key, k.site, drift)
			continue
		}
		t.Logf("%s: documented %q resolves to %s = %s (%s %s)", k.key, value, got, k.constant, k.reader, k.form)
	}

	// The floor, and the one assertion that fails at HEAD without this
	// bead's widening: the seed documents TEN duration keys with a value and
	// the old rule reached four of them. Ten is the measured population
	// (ranger-base-2vynj finding 1, "THE POPULATION"), and the two keys it
	// leaves out — autostart_interval, which has no default, and
	// autostart_max_interval, which defaults to 8x the base — are correctly
	// outside it and must stay outside.
	if compared < 10 {
		t.Errorf("compared %d documented duration defaults, want at least 10 — the seed documents attn_guard_stuck, attn_parked_age, attn_question_age, dispatch_epoch, model_probe_ttl, plan_guard_blind_max, plan_usage_stale_after, plan_usage_ttl, verify_batch_age and verify_box_max_age with a value, so a census that found fewer stopped reading the file", compared)
	}
}

// The register's stale half, as loud as an undocumented key. An entry in
// seedDocumentedOnPurposeNotTheDefault silences the pin above for one key,
// so each of the three ways it can quietly stop describing anything is a
// failure: the key is no longer a duration key, the seed no longer
// documents it, or the value it describes no longer parses.
//
// (The fourth way — the documented value has become the default again — is
// held in the pin above, where the comparison already is.)
func TestSeedConfigDocumentedDefaultRegisterIsNotStale(t *testing.T) {
	t.Parallel()
	if len(seedDocumentedOnPurposeNotTheDefault) == 0 {
		// Nothing registered, nothing silenced. Not a skip: the arm below
		// is what a future entry is held to, and this says the register is
		// empty today rather than that the check did not run.
		t.Log("seedDocumentedOnPurposeNotTheDefault is empty — every duration key the seed documents is documented at its own default")
		return
	}
	root := qibRepoRoot(t)
	keys, _ := seedDurationKeySites(t, root)
	byKey := map[string]seedDurationKey{}
	for _, k := range keys {
		byKey[k.key] = k
	}
	cfg := seedConfigPath(t)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range seedSortedKeys(seedDocumentedOnPurposeNotTheDefault) {
		why := seedDocumentedOnPurposeNotTheDefault[key]
		if strings.TrimSpace(why) == "" {
			t.Errorf("seedDocumentedOnPurposeNotTheDefault[%q] carries no reason — an entry here silences the documented-default pin for that key, so the why IS the entry", key)
		}
		k, derived := byKey[key]
		if !derived {
			t.Errorf("seedDocumentedOnPurposeNotTheDefault names %q, which no call site reads through a duration resolver any more — the key, or the reader, is gone, and the entry now excuses nothing", key)
			continue
		}
		value, lines := seedDocumentedValue(string(b), key)
		if lines != 1 {
			t.Errorf("seedDocumentedOnPurposeNotTheDefault names %q, and examples/config.yaml documents it on %d commented lines — an entry describing a line that is not there is stale; drop it", key, lines)
			continue
		}
		def, known := seedDurationDefaults[k.constant]
		if !known {
			t.Errorf("seedDocumentedOnPurposeNotTheDefault names %q, whose constant %s is not in seedDurationDefaults", key, k.constant)
			continue
		}
		if _, drift := seedDefaultDrift(t, seedResolveFor(t, k, def), key, value, def); strings.Contains(drift, "does not parse") || strings.Contains(drift, "no value") {
			t.Errorf("examples/config.yaml documents %s at %q, and the register says that is deliberate — but %s: a value documented on purpose still has to be a value an instance could type", key, value, drift)
		}
	}
}

// The matcher has to be able to say no, or the pin above is a spelling
// exercise. Drives seedDefaultDrift over planted texts rather than the tree,
// so it is not itself a tree-wide pin — and the first two rows are the
// bead's own requirement: the comparison is a DURATION comparison, and a
// text that does not parse must not pass by landing on the default.
func TestSeedDocumentedDefaultDriftCheckCanStillSayNo(t *testing.T) {
	t.Parallel()
	const def = 14 * 24 * time.Hour // DefaultAttnParkedAge, said locally so
	// a change to the constant does not move this control's subject.
	for _, tc := range []struct {
		name  string
		text  string
		drift bool
	}{
		{"the spelled default", "336h", false},
		// Compared as a duration, not as text: bare seconds are a form
		// attnAge accepts, and 1209600 of them are the same fortnight.
		{"the same default in bare seconds", "1209600", false},
		// The claim the bead names: `14d` is a typo, Go has no day unit,
		// and attnAge answers a typo with the DEFAULT. A text compare
		// would call this drift and a naive value compare would call it
		// clean; neither is the truth, which is "this line proves nothing".
		{"a fortnight Go cannot parse", "14d", true},
		{"a different duration", "168h", true},
		{"a different duration in bare seconds", "604800", true},
		{"zero, which attnAge accepts and means every tick", "0", true},
		{"a documented line with no value at all", "", true},
		{"prose where a value should be", "two weeks", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := seedDurationKey{key: "attn_parked_age", constant: "DefaultAttnParkedAge", reader: "attnAge", form: seedFormCallSite}
			_, drift := seedDefaultDrift(t, seedResolveFor(t, k, def), k.key, tc.text, def)
			if got := drift != ""; got != tc.drift {
				t.Errorf("seedDefaultDrift(%q) drift=%v (%q), want drift=%v", tc.text, got, drift, tc.drift)
			}
		})
	}

	// And the extractor: it reads the value, stops at the trailing comment,
	// and does not read prose that merely names the key.
	cfg := "# attn_parked_age: 336h      # = 14d, the default: how long a park\n" +
		"#                            # stays quiet before G3 reports it\n" +
		"# `attn_parked_age:` is the third key, and the only one whose default\n" +
		"  # attn_parked_age: 1h\n"
	if v, n := seedDocumentedValue(cfg, "attn_parked_age"); v != "336h" || n != 1 {
		t.Errorf("seedDocumentedValue = (%q, %d), want (\"336h\", 1) — the extractor reads the trailing comment, the prose mention, or the indented line as a documented value", v, n)
	}
	if v, n := seedDocumentedValue("# attn_guard_stuck: 2h\n# attn_guard_stuck: 4h\n", "attn_guard_stuck"); n != 2 {
		t.Errorf("seedDocumentedValue = (%q, %d) over two documented lines, want 2 — the pin cannot report a second documented value it does not count", v, n)
	}

	// The same matcher through a BODY-form reader, which is the half the pin
	// gained in ranger-base-khqvr and the half whose six mutants all survived
	// before it. seedResolveFor hands the reader a config and nothing else,
	// so the grammar here is PlanUsageTTL's own: bare seconds are the same
	// five minutes, and a text it refuses leaves the default standing.
	body := seedDurationKey{key: "plan_usage_ttl", constant: "PlanUsageTTLDefault", reader: "PlanUsageTTL", form: seedFormBody}
	for _, tc := range []struct {
		name  string
		text  string
		drift bool
	}{
		{"the spelled default", "5m", false},
		{"the same default in bare seconds", "300", false},
		{"the mutant the bead measured", "50m", true},
		{"zero, which this reader accepts and means no sharing", "0", true},
		{"prose where a value should be", "five minutes", true},
	} {
		t.Run("body-form/"+tc.name, func(t *testing.T) {
			t.Parallel()
			_, drift := seedDefaultDrift(t, seedResolveFor(t, body, PlanUsageTTLDefault), body.key, tc.text, PlanUsageTTLDefault)
			if got := drift != ""; got != tc.drift {
				t.Errorf("seedDefaultDrift(%q) through %s drift=%v (%q), want drift=%v", tc.text, body.reader, got, drift, tc.drift)
			}
		})
	}
}

// The derivation has to be able to say no, and to say it about each of the
// four near-misses, or the pin above is a rule nobody has watched refuse
// anything — which is exactly how six keys sat outside the census for six
// weeks (ranger-base-khqvr). Drives the whole census, both passes, over a
// PLANTED tree rather than the repo, so it is not itself a tree-wide pin and
// a change to the harness's own readers cannot move its subject.
func TestSeedDurationDerivationCanStillSayNo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// A miniature of the two shapes and of everything that looks like them.
	// It only has to PARSE: the census reads the syntax and calls nothing.
	const planted = `package planted

import (
	"io"
	"time"
)

type App struct{ ConfigPath string }

func YamlGet(path, key string) string { return "" }

func (a *App) CfgGet(key, def string) string { return def }

const (
	PlantedTTLDefault       = time.Minute
	DefaultPlantedGrace     = time.Hour
	PlantedViaCfgGetDefault = 2 * time.Hour
	DefaultPlantedParked    = 4 * time.Hour
	plantedBareConstant     = 3 * time.Hour
)

// A body-form reader naming its constant with the SUFFIX spelling, which the
// prefix-only rule missed for four live keys.
func (a *App) PlantedTTL(errw io.Writer) time.Duration {
	if YamlGet(a.ConfigPath, "planted_ttl") == "" {
		return PlantedTTLDefault
	}
	return PlantedTTLDefault
}

// The same shape with the PREFIX spelling: both are live in the real tree.
func (a *App) PlantedGrace(errw io.Writer) time.Duration {
	_ = YamlGet(a.ConfigPath, "planted_grace")
	return DefaultPlantedGrace
}

// Keyed through CfgGet, whose key is its FIRST argument where YamlGet's is
// its second.
func (a *App) PlantedViaCfgGet(errw io.Writer) time.Duration {
	_ = a.CfgGet("planted_via_cfgget", "")
	return PlantedViaCfgGetDefault
}

// Near-miss 1: the default is a rule, not a constant (backup_max_age).
func (a *App) PlantedRuleDefault(errw io.Writer) time.Duration {
	_ = YamlGet(a.ConfigPath, "planted_rule_default")
	return a.plantedRule()
}

func (a *App) plantedRule() time.Duration { return time.Hour }

// Near-miss 2: one reader, two keys — no single constant is its default.
func (a *App) PlantedTwoKeys(errw io.Writer) time.Duration {
	if YamlGet(a.ConfigPath, "planted_first") != "" {
		return PlantedTTLDefault
	}
	_ = YamlGet(a.ConfigPath, "planted_second")
	return PlantedTTLDefault
}

// Near-miss 3: the fallback is not Default-shaped at all.
func (a *App) PlantedNoConstant(errw io.Writer) time.Duration {
	_ = YamlGet(a.ConfigPath, "planted_no_constant")
	return plantedBareConstant
}

// A call-site reader: it reads the key it was HANDED, so its keys live at
// its call sites and not here.
func (a *App) plantedAge(key string, def time.Duration, errw io.Writer) time.Duration {
	_ = YamlGet(a.ConfigPath, key)
	return def
}

// A thin wrapper, which is how the real attn_* keys are spelled: no key of
// its own, and the pairing is one line down at the call.
func (a *App) PlantedParked(errw io.Writer) time.Duration {
	return a.plantedAge("planted_parked", DefaultPlantedParked, errw)
}

// Near-miss 4: a call site whose default argument is a call.
func (a *App) PlantedCallSiteRule(errw io.Writer) time.Duration {
	return a.plantedAge("planted_call_rule", a.plantedRule(), errw)
}

// Not a method on App.
func plantedFree(errw io.Writer) time.Duration {
	_ = YamlGet("", "planted_free")
	return PlantedTTLDefault
}

// Not a duration.
func (a *App) PlantedCount(errw io.Writer) int {
	_ = YamlGet(a.ConfigPath, "planted_count")
	return 1
}
`
	// A reader in a _test.go file is not the harness, and a file that does
	// not parse is the build's finding and not this pin's: both are skipped,
	// and the scanned count is what says so.
	const plantedTest = `package planted

import (
	"io"
	"time"
)

func (a *App) PlantedInATestFile(errw io.Writer) time.Duration {
	_ = YamlGet(a.ConfigPath, "planted_in_a_test_file")
	return PlantedTTLDefault
}
`
	for name, body := range map[string]string{
		"planted.go":      planted,
		"planted_test.go": plantedTest,
		"broken.go":       "package planted\n\nfunc (((\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	c := seedDurationKeyCensus(t, root)
	if c.scanned != 1 {
		t.Errorf("scanned %d files of the planted tree, want 1 — the walk read the _test.go file or the file that does not parse", c.scanned)
	}

	if got, want := strings.Join(c.resolvers, ","), "plantedAge"; got != want {
		t.Errorf("derived call-site readers %q, want %q", got, want)
	}
	if got, want := strings.Join(c.readers, ","), "PlantedGrace,PlantedTTL,PlantedViaCfgGet"; got != want {
		t.Errorf("derived body-form readers %q, want %q", got, want)
	}

	gotPairs := map[string]string{}
	for _, k := range c.keys {
		gotPairs[k.key] = k.constant + " via " + k.reader + " (" + k.form.String() + ")"
		if k.site == "" {
			t.Errorf("pairing for %s carries no site — a census that cannot say WHERE cannot be acted on", k.key)
		}
	}
	for key, want := range map[string]string{
		"planted_ttl":        "PlantedTTLDefault via PlantedTTL (body-form reader)",
		"planted_grace":      "DefaultPlantedGrace via PlantedGrace (body-form reader)",
		"planted_via_cfgget": "PlantedViaCfgGetDefault via PlantedViaCfgGet (body-form reader)",
		"planted_parked":     "DefaultPlantedParked via plantedAge (call site)",
	} {
		if got := gotPairs[key]; got != want {
			t.Errorf("pairing for %s = %q, want %q", key, got, want)
		}
		delete(gotPairs, key)
	}
	for key, got := range gotPairs {
		t.Errorf("the derivation paired %s as %s, and nothing in the planted tree states that pairing", key, got)
	}

	gotUnpaired := map[string]string{}
	for _, u := range c.unpaired {
		gotUnpaired[u.key] = u.why
		if u.site == "" {
			t.Errorf("unpaired key %s carries no site", u.key)
		}
	}
	for _, key := range []string{
		// each of the four near-misses, reported rather than dropped
		"planted_rule_default", "planted_first", "planted_second",
		"planted_no_constant", "planted_call_rule",
	} {
		if why, ok := gotUnpaired[key]; !ok {
			t.Errorf("the derivation dropped %s in silence — an unpairable key must be REPORTED, which is the whole finding of ranger-base-khqvr", key)
		} else if strings.TrimSpace(why) == "" {
			t.Errorf("unpaired key %s carries no why", key)
		}
		delete(gotUnpaired, key)
	}
	for key, why := range gotUnpaired {
		t.Errorf("the derivation called %s unpairable (%s), and it is not one of the planted near-misses", key, why)
	}

	// And the keys nothing should have seen at all: a free function, a
	// non-duration reader, a test file, and the key a wrapper never reads
	// itself.
	for _, key := range []string{"planted_free", "planted_count", "planted_in_a_test_file"} {
		if _, paired := gotPairs[key]; paired {
			t.Errorf("%s was paired; it is not a duration reader on App", key)
		}
		for _, u := range c.unpaired {
			if u.key == key {
				t.Errorf("%s was reported unpairable; it is not a duration reader on App at all, so the rule has widened past its subject", key)
			}
		}
	}
}
