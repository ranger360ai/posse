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
//     (ranger-base-vofbl). Two halves, one derivation, at the foot of this
//     file: the DOCUMENTED DEFAULTS over duration-valued lines, and the
//     DOCUMENTED NUMBERS over bare-number ones (ranger-base-p9qve). They
//     share seedReaderDecl and seedDocumentedLines and differ only in the
//     two predicates those take, so neither can go narrower than the other
//     while both look green.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
// THE REGISTERS are three, and each silences one key for a stated reason:
//
//   - seedDocumentedOnPurposeNotTheDefault, for a key the file documents at
//     something other than its default on purpose. Empty today.
//   - seedDurationDefaultNotAConstant, for a key whose default is a RULE and
//     not a constant, which this pin cannot compare against anything.
//     `backup_max_age` is the one (ADR 0036 §6). Such a key must also stay
//     UNDOCUMENTED in the seed, because a documented line for it would be a
//     claim nothing here can hold.
//   - seedDocumentedDurationOutsideTheCensus, for a duration-valued line in
//     the seed whose key the derivation reaches no reader for, because there
//     is no constant for the line to drift from. Three members, all named
//     in the paragraph below.
//
// Each register's stale half is as loud as its live half: an entry whose key
// is no longer a duration key, is no longer documented, or whose documented
// value has quietly become the default again, fails.
//
// AND THE CENSUS IS TOTAL FROM THE SEED'S SIDE TOO, which it was not until
// ranger-base-ghcx3 finding 1. Everything above walks the TREE's readers and
// then asks the seed about each key it found; nothing walked the SEED and
// asked whether each documented duration line had been reached. A documented
// key whose reader matches neither derived shape was never compared, never
// registered and never reported — `compared` stayed at ten, the floor passed,
// and the operator-facing spec could name a default ten times the real one in
// silence, which is the exact sentence ranger-base-2vynj finding 1 was filed
// for. MEASURED 2026-10-04 with a planted third-shape reader (a body-form
// reader with one extra parameter) documented at 10x its default: these pins
// were green, and the same reader one parameter closer to the body form
// reddened two of them. So seedDocumentedDurations enumerates the seed's own
// duration-valued lines and each one must be accounted for in exactly one of
// three ways — compared, in seedDurationDefaultNotAConstant, or in the
// register above. That turns the `compared < 10` floor into a SET, which is
// the whole difference: deleting a documented line reds at `compared 9`, and
// adding one is invisible to any count.
//
// WHAT IS STILL OUTSIDE, on purpose. `autostart_interval` has no default at
// all — its presence is the arm switch and absent means off (govern.go) —
// and `autostart_max_interval` defaults to 8x the base rather than to a
// constant. `backup_interval` is the first key's shape one surface over
// (ranger-base-s1dmh): its presence arms the backup clock and absent starts
// no clock at all (backuploop.go), so there is no default for its documented
// line to be a statement about either. None of the three is read through a
// reader of this shape, so none is derived, and none should be: a register
// row for a key with no constant to drift from is the wrong answer to the
// DRIFT question. It is the right answer to the COVERAGE one, and that is
// what the third register is: the three are named there with the reason, so
// the seed-side enumeration accounts for them instead of either comparing
// them against nothing or skipping them in silence. MEASURED 2026-10-04: the
// body-form rule scans `YamlGet` and `CfgGet` key literals alike and the
// first two keys above are read through neither inside a duration reader, so
// widening the key source adds no member and closes the escape a future
// CfgGet-based reader would otherwise have. `backup_interval` is the case
// that widening would not have caught anyway: LoadBackupConfig does read it
// through `CfgGet`, but it returns a BackupConfig and an error rather than a
// duration, so it is not a reader of either shape (MEASURED 2026-10-09).

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

// seedDocumentedDurationOutsideTheCensus records a duration-valued line in
// examples/config.yaml whose key the derivation reaches no reader for, so
// there is no constant a documented value could drift from. Key -> why.
//
// It is the third of the three ways a documented duration line can be
// accounted for, and the only one that admits a line at all: the other two
// either compare it or require it to be absent. Without it, such a line is
// invisible — which is ranger-base-ghcx3 finding 1 — and with it as a BARE
// LIST it would be a hole the next key falls into, so the staleness half in
// the pin below is what keeps each row describing something: a key here that
// the seed no longer documents with a duration, or that the census has since
// learned to compare, fails.
var seedDocumentedDurationOutsideTheCensus = map[string]string{
	"autostart_interval": "its presence IS the arm switch — absent means autostart is off (govern.go) — so " +
		"there is no default for the documented 5m to be a statement about, and a constant would have to " +
		"stand for \"off\"",
	"autostart_max_interval": "the ceiling defaults to 8x the base interval rather than to a constant " +
		"(govern.go), so the documented 40m is a statement about THE LINE ABOVE IT and not about a number " +
		"this pin could hold",
	"backup_interval": "its presence IS the backup clock's arm switch — absent starts no ticker at all " +
		"(LoadBackupConfig, backuploop.go) — so the documented 24h is a shape for the interval and not a " +
		"statement about a default. The key IS read through CfgGet, but inside a reader that answers a " +
		"BackupConfig and an error rather than a duration, so neither derived shape reaches it; the one " +
		"backup duration with a default is backup_max_age:, whose default is a rule and which is " +
		"therefore in seedDurationDefaultNotAConstant and must stay undocumented (ranger-base-s1dmh)",
}

// seedDocumentedLine is one commented `key: <value>` line in
// examples/config.yaml: the seed-side half of a census.
type seedDocumentedLine struct {
	key   string
	value string
	line  int
}

// seedLooksLikeDuration is the discriminator the seed-side enumeration needs
// and the one place its LIMIT lives: a value with a unit, and never a bare
// number.
//
// Bare numbers are the limit, and it is a real one stated rather than
// papered over. attnAge's grammar accepts bare SECONDS (govern.go), so
// `attn_parked_age: 1209600` is a documented duration — and nothing in the
// text of such a line distinguishes it from `load_guard: 25`, which is a
// load average, or `grok_guard_week: 85`, which is a percentage. MEASURED
// 2026-10-04: examples/config.yaml has ten commented bare-number lines and
// not one of them is a duration, so reading them as durations would mean ten
// register rows saying "this is a percentage" — noise that would bury the one
// row that ever means anything. The uncovered case is therefore a NEW
// duration key documented in bare seconds; every documented duration line in
// the file today, and every spelling an operator would reach for, carries its
// unit.
func seedLooksLikeDuration(v string) bool {
	if v == "" {
		return false
	}
	if c := v[len(v)-1]; (c >= '0' && c <= '9') || c == '.' {
		return false
	}
	_, err := time.ParseDuration(v)
	return err == nil
}

// seedDocumentedLines enumerates every commented `key: <value>` line in the
// seed whose value `looksLike` accepts, in file order.
//
// The line rule is seedDocumentedValue's, said once more over an unknown key
// rather than a known one: `#` at column 0, optional blank, then `key:`. The
// two matchers agreeing is not assumed — the pins below fail when a key one
// of them compared is not in this enumeration, which is what would catch
// them drifting apart.
//
// `looksLike` is the only thing the duration half and the number half
// disagree about here, for the same reason seedReaderDecl takes `isValue`:
// one line matcher, two discriminators, so the half added by
// ranger-base-p9qve cannot read a different file than the half it was added
// beside.
func seedDocumentedLines(cfgText string, looksLike func(string) bool) []seedDocumentedLine {
	var out []seedDocumentedLine
	for i, ln := range strings.Split(cfgText, "\n") {
		if !strings.HasPrefix(ln, "#") {
			continue
		}
		rest := strings.TrimLeft(ln[1:], " \t")
		c := strings.Index(rest, ":")
		if c <= 0 {
			continue
		}
		key := rest[:c]
		if strings.IndexFunc(key, func(r rune) bool {
			return !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
		}) >= 0 {
			continue
		}
		v := rest[c+1:]
		if j := strings.Index(v, "#"); j >= 0 {
			v = v[:j]
		}
		v = strings.TrimSpace(v)
		if !looksLike(v) {
			continue
		}
		out = append(out, seedDocumentedLine{key: key, value: v, line: i + 1})
	}
	return out
}

// seedDocumentedDurations and seedDocumentedNumbers are the two
// discriminators applied: a value with a UNIT, and a bare decimal number.
func seedDocumentedDurations(cfgText string) []seedDocumentedLine {
	return seedDocumentedLines(cfgText, seedLooksLikeDuration)
}

func seedDocumentedNumbers(cfgText string) []seedDocumentedLine {
	return seedDocumentedLines(cfgText, seedLooksLikeNumber)
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

// seedPairing is one derived pairing: a config key, the Default-shaped
// constant it falls back to, and the reader whose grammar resolves it.
//
// `shift` is the power of two the reader applies to that constant on the way
// out, and 0 for the readers that return it as it stands. It exists because
// one live reader answers in a different UNIT than its key is named for —
// BackupMinFree returns `DefaultBackupMinFreeMB << 20`, bytes from a key in
// MB — so the number a documented line has to agree with is the constant
// times 2^shift, and without the shift there is no one number at all
// (ranger-base-c768n finding 3).
type seedPairing struct {
	key      string
	constant string
	shift    uint
	reader   string
	form     seedReaderForm
	site     string
}

// seedRetDefault is one Default-shaped constant a reader returns, with the
// shift it is scaled by. The spelling is what the pairing rule counts, so a
// reader returning one constant both bare and scaled reads as TWO returned
// defaults and is reported unpairable rather than paired against whichever
// came first.
type seedRetDefault struct {
	name  string
	shift uint
}

func (d seedRetDefault) spelling() string {
	if d.shift == 0 {
		return d.name
	}
	return fmt.Sprintf("%s<<%d", d.name, d.shift)
}

// seedMaxShift is the largest shift this census will pair. float64 is the
// number half's carrier (see that section's header), and it holds every
// integer below 2^53 exactly, so a shift above 52 could not be compared with
// `==` whatever the constant is. A larger one is REPORTED as unpairable
// rather than paired against a number the comparison cannot represent; the
// tree's one shift is 20.
const seedMaxShift = 52

// seedReturnedDefault reads one returned expression and says which
// Default-shaped constant it hands back, scaled by what.
//
// Two shapes, and the second is the tree's: a bare identifier
// (`return DefaultBackupKeep`), and that identifier shifted left by an
// integer literal (`return DefaultBackupMinFreeMB << 20`). Anything else —
// a sum, a product, a call, a bare literal — is not a stated pairing, and a
// reader whose default is one of those is reported unpairable by the caller.
func seedReturnedDefault(e ast.Expr) (seedRetDefault, bool) {
	if id, ok := e.(*ast.Ident); ok && seedDefaultShaped(id.Name) {
		return seedRetDefault{name: id.Name}, true
	}
	be, ok := e.(*ast.BinaryExpr)
	if !ok || be.Op != token.SHL {
		return seedRetDefault{}, false
	}
	id, ok := be.X.(*ast.Ident)
	if !ok || !seedDefaultShaped(id.Name) {
		return seedRetDefault{}, false
	}
	lit, ok := be.Y.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return seedRetDefault{}, false
	}
	n, err := strconv.ParseUint(lit.Value, 0, 8)
	if err != nil || n > seedMaxShift {
		return seedRetDefault{}, false
	}
	return seedRetDefault{name: id.Name, shift: uint(n)}, true
}

// seedUnpairedKey is a config key the tree reads but this pin cannot pair
// with a constant — the default is a call, a computation, or absent. It is
// reported rather than skipped, because a silent skip is the defect
// ranger-base-khqvr was filed for.
type seedUnpairedKey struct {
	key  string
	site string
	why  string
}

// seedCensus is everything the tree says about config keys of one value
// type. The three name slices are what make the hand tables total.
type seedCensus struct {
	keys      []seedPairing     // derived pairings, sorted by key
	resolvers []string          // call-site reader names found in the tree
	readers   []string          // body-form reader names found in the tree
	unpaired  []seedUnpairedKey // key sites with no constant to compare
	scanned   int
}

// seedIsTimeDuration reports whether an AST type expression is time.Duration.
func seedIsTimeDuration(e ast.Expr) bool { return seedIsQualified(e, "time", "Duration") }

// seedIsIOWriter reports whether an AST type expression is io.Writer.
func seedIsIOWriter(e ast.Expr) bool { return seedIsQualified(e, "io", "Writer") }

// seedNumericTypes are the result types that make a config reader a NUMBER
// reader, and the list is the tree's own: `int` (verifyBatch, BackupKeep),
// `float64` (LoadGuard, GrokGuardWeek) and `uint64` (BackupMinFree) are live
// today, and the rest of Go's numeric basics are here so a reader added in
// one of them is derived rather than invisible. `bool` and `string` are not
// numbers and a documented one is not a number claim.
var seedNumericTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

// seedIsNumeric reports whether an AST type expression is one of Go's
// numeric basic types, by NAME. A name is all the syntax says — this census
// does not type-check — and a local alias shadowing `int` would fool it,
// which is a shape this tree does not have and would be visible in review.
func seedIsNumeric(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && seedNumericTypes[id.Name]
}

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

// seedReaderDecl reads one function declaration and says which of the two
// reader shapes it is, if either.
//
// `isValue` is the ONE thing the duration half and the number half disagree
// about: which result type makes a method a config reader at all
// (seedIsTimeDuration, seedIsNumeric). Everything else — the two shapes, the
// key-literal rule, the Default-shaped-constant rule (seedReturnedDefault,
// including the SCALE a reader applies on the way out), the near-misses
// and the reporting of an unpairable key — is said once here and shared, so
// the number half is a WIDENING of this derivation and not a second,
// narrower copy of it (ranger-base-p9qve; the shape seedconfig_test.go's own
// header argues against).
//
// Split out from the walk so TestSeedDurationDerivationCanStillSayNo and
// TestSeedNumberDerivationCanStillSayNo can drive it over planted source: a
// derivation whose near-misses are never exercised is a derivation nobody
// has seen refuse anything.
func seedReaderDecl(fd *ast.FuncDecl, isValue func(ast.Expr) bool) (callSiteResolver bool, body []seedPairing, unpaired []seedUnpairedKey) {
	if fd.Body == nil || !seedRecvIsApp(fd) {
		return false, nil, nil
	}
	res := seedFieldTypes(fd.Type.Results)
	if len(res) != 1 || !isValue(res[0]) {
		return false, nil, nil
	}
	params := seedFieldTypes(fd.Type.Params)

	// What the body does with config: the key literals it reads, and the
	// parameter names it passes to a reader instead.
	var litKeys, paramKeys, retDefaults []string
	// The returned constants by SPELLING, so the pairing rule below counts
	// `DefaultX` and `DefaultX << 20` as two and the scale is carried
	// through to the pairing rather than dropped.
	retDefault := map[string]seedRetDefault{}
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
				if d, ok := seedReturnedDefault(r); ok {
					retDefaults = append(retDefaults, d.spelling())
					retDefault[d.spelling()] = d
				}
			}
		}
		return true
	})
	litKeys, paramKeys, retDefaults = seedUniqSorted(litKeys), seedUniqSorted(paramKeys), seedUniqSorted(retDefaults)

	// The call-site form: (key string, def <value>, errw io.Writer), reading
	// the key it was HANDED. Its own keys live at its call sites.
	if len(params) == 3 && seedIsIdent(params[0], "string") && isValue(params[1]) && seedIsIOWriter(params[2]) {
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
	d := retDefault[retDefaults[0]]
	return false, []seedPairing{{
		key:      litKeys[0],
		constant: d.name,
		shift:    d.shift,
		reader:   fd.Name.Name,
		form:     seedFormBody,
	}}, nil
}

// seedCensusOf parses both pairing shapes out of the tree, for whichever
// value type `isValue` names.
//
// Two passes over one parse, because the call-site pass needs the resolver
// set the declaration pass derives: matching call sites against the DERIVED
// set rather than against the half's hand-written resolver table
// (seedDurationResolvers, seedNumberResolvers) is what makes that table total
// instead of a filter with a hole in it.
func seedCensusOf(t *testing.T, root string, isValue func(ast.Expr) bool) seedCensus {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	var rels []string
	census := seedCensus{}

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
			isResolver, body, unpaired := seedReaderDecl(fd, isValue)
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
			// seedReturnedDefault, the same rule the body form reads its
			// own return with: said once, so a scaled default states its
			// pairing in both shapes or in neither.
			d, ok := seedReturnedDefault(call.Args[1])
			if !ok {
				census.unpaired = append(census.unpaired, seedUnpairedKey{key: key, site: at, why: fmt.Sprintf(
					"the default argument at the %s call site is not a Default-shaped constant", fn)})
				return true
			}
			census.keys = append(census.keys, seedPairing{
				key:      key,
				constant: d.name,
				shift:    d.shift,
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

// seedDurationKeyCensus is seedCensusOf over time.Duration readers: the
// duration half's entry point, unchanged in behaviour by the widening.
func seedDurationKeyCensus(t *testing.T, root string) seedCensus {
	t.Helper()
	return seedCensusOf(t, root, seedIsTimeDuration)
}

// seedNumberKeyCensus is seedCensusOf over NUMERIC readers: the number
// half's entry point (ranger-base-p9qve).
func seedNumberKeyCensus(t *testing.T, root string) seedCensus {
	t.Helper()
	return seedCensusOf(t, root, seedIsNumeric)
}

// seedDurationKeySites is the duration census's pairings alone, for the
// register staleness pin, which asks only "is this key still a duration
// key".
func seedDurationKeySites(t *testing.T, root string) (keys []seedPairing, scanned int) {
	t.Helper()
	c := seedDurationKeyCensus(t, root)
	return c.keys, c.scanned
}

// seedResolveFor is the production read of one pairing: a closure that
// answers what a fresh instance would get for a config file, computed by the
// instance's own reader. One closure for both forms, so the drift check has
// a single code path and the grammar is never a test's.
func seedResolveFor(t *testing.T, k seedPairing, def time.Duration) func(cfgPath string, errw io.Writer) time.Duration {
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

	// NO SCALED DEFAULT ON THIS SIDE. The shared derivation reads a default
	// returned shifted as well as bare (seedReturnedDefault,
	// ranger-base-c768n finding 3), and the number half APPLIES that scale
	// before comparing. This half does not: a duration is already a scaled
	// integer and nothing in the tree returns `Default<X> << n` as one
	// (MEASURED 2026-10-09 — `<<` appears over a Default-shaped constant at
	// exactly one site, BackupMinFree). So a scaled duration pairing arriving
	// here would be compared against the UNSHIFTED constant and read as
	// drift, or worse not read as drift at all; it is a named failure
	// instead, and whoever writes that reader decides what the comparison
	// should be.
	for _, k := range keys {
		if k.shift != 0 {
			t.Errorf("%s pairs %s with %s scaled by << %d (%s) — the duration half compares the constant as it stands, so a reader that shifts its default on the way out is outside this comparison. Give this half the scale the number half has (seedNumberAnswerFor), or state why the shift does not belong in the comparison",
				k.reader, k.key, k.constant, k.shift, k.site)
		}
	}

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
	byKey := map[string]seedPairing{}
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
	comparedKeys := map[string]bool{}
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
		comparedKeys[k.key] = true
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

	// THE DIRECTION THAT HURTS (ranger-base-ghcx3 finding 1). Everything
	// above walks the tree's readers and asks the seed about each key it
	// found. This walks the SEED and asks whether each duration-valued line
	// it ships was reached — the input-side check the notes fragment for
	// ranger-base-khqvr states the rule for one input short of this one
	// ("a derivation is only total up to its own hand-written inputs, and
	// each one needs a check in the direction that hurts"). The seed file is
	// an input of this census, and the direction that hurts is: the seed
	// documents a duration key the census does not reach.
	documented := seedDocumentedDurations(text)
	documentedKeys := map[string]bool{}
	for _, d := range documented {
		documentedKeys[d.key] = true
	}

	// The two matchers must agree about what a documented line IS, or the set
	// check below is a set over the wrong lines. Derived rather than counted:
	// every key the loop above COMPARED has to be one this enumeration found,
	// which needs no number and fails the moment the two parsers drift.
	for _, key := range seedSortedKeys(comparedKeys) {
		if !documentedKeys[key] {
			t.Errorf("this pin compared a documented value for %s, and seedDocumentedDurations did not find that line — the two line matchers disagree, so the coverage check below is reading a different file than the comparison above",
				key)
		}
	}

	for _, d := range documented {
		switch {
		case comparedKeys[d.key]:
			// Reached, compared, held.
		case seedDurationDefaultNotAConstant[d.key] != "":
			// Registered as having no constant to compare against. The
			// stronger half above is what fails on the line existing at all,
			// so this is not a second complaint about the same row.
		case seedDocumentedDurationOutsideTheCensus[d.key] != "":
			t.Logf("examples/config.yaml:%d documents %s as %s — registered as outside the census (%s)",
				d.line, d.key, d.value, seedDocumentedDurationOutsideTheCensus[d.key])
		default:
			t.Errorf("examples/config.yaml:%d documents %s as %s, and this census never reached that key: a fresh instance reads that line as a statement about the harness and nothing holds it, so the number can be wrong by any factor in silence (ranger-base-ghcx3).\n"+
				"  three ways out, and they are not interchangeable: give the key a reader of one of the two derived shapes (a call-site reader, or `(errw io.Writer) time.Duration` naming its key and returning one Default-shaped constant) so this pin COMPARES it; or add it to seedDurationDefaultNotAConstant if its default is a rule, which also means dropping the documented value; or add it to seedDocumentedDurationOutsideTheCensus with the reason there is no constant for the line to drift from",
				d.line, d.key, d.value)
		}
	}

	// The third register's stale half, as loud as the other two's. A row here
	// admits a documented line nothing compares, so each of the two ways it
	// can stop describing anything is a failure.
	for _, key := range seedSortedKeys(seedDocumentedDurationOutsideTheCensus) {
		why := seedDocumentedDurationOutsideTheCensus[key]
		if strings.TrimSpace(why) == "" {
			t.Errorf("seedDocumentedDurationOutsideTheCensus[%q] carries no reason — the row admits a documented duration nothing holds, so the why IS the row", key)
		}
		if !documentedKeys[key] {
			t.Errorf("seedDocumentedDurationOutsideTheCensus names %q, which examples/config.yaml no longer documents with a duration value — the row admits a line that is not there; drop it", key)
		}
		if comparedKeys[key] {
			t.Errorf("seedDocumentedDurationOutsideTheCensus names %q, which this pin now COMPARES against %s — the key has a constant after all, so the row is excusing a line that is already held; drop it and let the comparison be what holds it", key, byKey[key].constant)
		}
	}

	// The floor, and the one assertion that failed at HEAD before
	// ranger-base-khqvr's widening: the seed documents TEN duration keys the
	// census reaches and the old rule reached four of them. Ten is the
	// measured population (ranger-base-2vynj finding 1, "THE POPULATION").
	// It is kept beside the set check above and not replaced by it, because
	// the two fail in opposite directions: this one catches a documented line
	// that LEFT (deleting one reds at `compared 9`), and the set catches one
	// that ARRIVED, which no count can see.
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
	byKey := map[string]seedPairing{}
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
			k := seedPairing{key: "attn_parked_age", constant: "DefaultAttnParkedAge", reader: "attnAge", form: seedFormCallSite}
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

	// And the SEED-SIDE enumeration, over planted text for the same reason:
	// it is the half that fails on a documented duration the census never
	// reached (ranger-base-ghcx3 finding 1), so a rule nobody has watched
	// refuse anything is a rule that will admit the next key. Every row here
	// is a line that is NOT a documented duration and the reasons differ,
	// which is the point — one matcher saying no four ways.
	enum := "# model_probe_ttl: 1h        # the default\n" +
		"# load_guard: 25             # a load average, and attnAge would read it as 25 seconds\n" +
		"# grok_pool_usd_per_point: 0.50\n" +
		"# budget_day: 250\n" +
		"# `plan_usage_ttl:` is the key the plan guard reads\n" +
		"  # verify_batch_age: 24h\n" +
		"# attn_parked_age: two weeks\n" +
		"# dispatch_epoch: 1h\n" +
		"model_probe_ttl: 9h\n"
	got := seedDocumentedDurations(enum)
	var names []string
	for _, d := range got {
		names = append(names, fmt.Sprintf("%s=%s@%d", d.key, d.value, d.line))
	}
	if want := []string{"model_probe_ttl=1h@1", "dispatch_epoch=1h@8"}; !slices.Equal(names, want) {
		t.Errorf("seedDocumentedDurations found %v, want %v — it must read the value before the trailing comment and refuse a bare number, prose, a key merely NAMED in prose, an indented line and a LIVE key",
			names, want)
	}
	// The bare-number limit is stated in seedLooksLikeDuration's comment and
	// held here, so the next reader meets it as a decision and not a bug.
	for _, tc := range []struct {
		text string
		dur  bool
	}{
		{"336h", true}, {"1h30m", true}, {"5m", true}, {"250ms", true}, {"0s", true},
		{"1209600", false}, {"25", false}, {"0.50", false}, {"0", false}, {"", false},
		{"two weeks", false}, {"14d", false}, {"true", false},
	} {
		if seedLooksLikeDuration(tc.text) != tc.dur {
			t.Errorf("seedLooksLikeDuration(%q) = %v, want %v", tc.text, !tc.dur, tc.dur)
		}
	}

	// The same matcher through a BODY-form reader, which is the half the pin
	// gained in ranger-base-khqvr and the half whose six mutants all survived
	// before it. seedResolveFor hands the reader a config and nothing else,
	// so the grammar here is PlanUsageTTL's own: bare seconds are the same
	// five minutes, and a text it refuses leaves the default standing.
	body := seedPairing{key: "plan_usage_ttl", constant: "PlanUsageTTLDefault", reader: "PlanUsageTTL", form: seedFormBody}
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

// ───────────────────────────────────────────────────────────────────────────
// The DOCUMENTED NUMBERS (ranger-base-p9qve).
//
// Property 4's other half, and the half the duration census could not see BY
// CONSTRUCTION rather than by omission: its derivation keys on readers whose
// result is a time.Duration, so a key read as an int, a float64 or a uint64
// was outside it however many bare numbers the seed documented.
//
// Eleven commented bare-number lines ship in examples/config.yaml today
// (MEASURED 2026-10-05: autostart_max_beads, budget_day, budget_pass,
// grok_guard_week, grok_pool_usd_per_point, launcher_behind_max, load_guard,
// plan_guard_5h, plan_guard_7d, uncounted_cap_codex, verify_batch) and
// nothing compared any of them to anything. The cost is the one
// ranger-base-ghcx3 named for the duration half: a fresh instance reads a
// documented number as a statement about the harness, nothing holds it, and
// the number can be wrong by any factor in silence.
//
// launcher_behind_max is the key this bead was filed beside and the one that
// proves the derivation works on arrivals rather than on a list: it did not
// exist when this half was written. ranger-base-y13h7 landed it mid-bead with
// a reader of exactly the body form below — `(errw io.Writer) int`, one
// CfgGet key literal, one Default-shaped constant — and the pin met it on the
// merge, named the two table rows it needed, and then compared the seed's 16
// against DefaultLauncherBehindMax. It carries a MEASURED threshold
// (docs/notes.d/ranger-base-y13h7.md), so a drifting seed line there would
// misquote a measurement.
//
// ONE MECHANISM, TWO PREDICATES. This is a WIDENING of the census above and
// not a second one beside it, which is the distinction the bead was filed on:
// a one-off pin for a seventh key would be a second implementation going
// narrower than the derived one while both looked green. Everything
// structural is shared and said once —
//
//   - seedReaderDecl derives both reader shapes, takes `isValue`, and gets
//     seedIsTimeDuration from one half and seedIsNumeric from the other.
//   - seedDocumentedLines enumerates the seed's own commented `key: value`
//     lines, takes `looksLike`, and gets seedLooksLikeDuration from one half
//     and seedLooksLikeNumber from the other.
//   - seedDocumentedValue, seedDefaultShaped, seedConfigKeyArg and the
//     four near-miss rules are not restated at all.
//
// — so what this half adds is three hand tables, three registers, and the
// comparison in the right type. A reader shape that stops matching stops
// matching for both halves at once, which is the property a parallel copy
// would not have.
//
// WHAT IS COMPARED, AND IN WHAT TYPE. The carrier is float64, because the
// tree's number readers answer in three different types — `int`
// (verifyBatch, BackupKeep, LauncherBehindMax), `float64` (LoadGuard) and
// `uint64` (BackupMinFree) — and one comparison type keeps the drift check a
// single code path. float64 holds every integer below 2^53 exactly, so every
// count, percent, dollar figure and megabyte ceiling this config can carry
// resolves exactly and the comparison is `==` rather than a tolerance nobody
// measured. A default above 2^53 would be outside this pin; the tree has
// none and a new one would be visible in review. seedMaxShift is the same
// line drawn over the scale below.
//
// AND IN WHOSE UNIT (ranger-base-c768n finding 3). One reader answers in a
// unit its key is not named for: BackupMinFree returns
// `DefaultBackupMinFreeMB << 20`, bytes from a key called
// `backup_min_free_mb:`. That key was registered as having no constant
// default for a release — the reasoning being that a census comparing the
// reader's own answer has no single number to compare, since the constant is
// 384 and the answer is 402653184 — and the seed documented a live 384 that
// could drift by any factor in silence. The reasoning was about the
// MECHANISM. The default is a constant, in the key's own unit, and the seed
// documents it in that unit, so there is one number; what was missing is the
// SCALE between them, which is stated in the reader's own return expression
// and is now derived from it (seedReturnedDefault -> seedPairing.shift ->
// seedNumberAnswerFor). Nothing about the resolution changes — the next
// paragraph's rule still holds, the documented text still goes through the
// instance's own method — only what the answer is held against: the
// constant shifted by the shift the parse read, rather than the constant
// bare. So a documented value, a moved constant and a changed scale each
// red (MEASURED 2026-10-09, three mutants), and the body-form cross-check
// below holds the derived shift against the binary's before any line is
// compared at all.
//
// It is still a read through the PRODUCTION reader and never a text compare,
// for the duration half's reason one type over: `25` and `25.0` and ` 25 `
// are the same load average and `twenty-five` is not a load average at all,
// and a value the reader refuses makes it name the value on errw and return
// the default — which would otherwise let a line that proves nothing pass by
// landing on the very number it was supposed to prove.
//
// WHAT THE SEED'S THIRTEEN LINES TURN OUT TO BE, and it is lopsided. Five of
// the tree's number keys pair a key literal with one Default-shaped constant
// — backup_keep, backup_min_free_mb, launcher_behind_max, load_guard,
// verify_batch — and the seed documents all five of them (it documented
// three until ranger-base-s1dmh gave the backup keys a block of their own,
// and backup_min_free_mb was unpairable until ranger-base-c768n gave the
// derivation the scale). So of the thirteen documented lines: FIVE are
// compared (load_guard agrees with LoadGuardDefault, launcher_behind_max
// with DefaultLauncherBehindMax, backup_keep with DefaultBackupKeep,
// backup_min_free_mb with DefaultBackupMinFreeMB shifted into bytes;
// verify_batch is documented at four times DefaultVerifyBatch on purpose and
// is registered for it), ONE the derivation reaches and cannot pair
// (grok_guard_week), and SEVEN it reaches no reader for at all.
//
// The reason the seven are seven is the house vocabulary for a number key,
// which is overwhelmingly "unset means OFF": budget_pass:/budget_day: unset
// is no cap, plan_guard_<window>: unset is no guard, uncounted_cap_<runtime>:
// unset is unlimited, and grok_guard_week: unset is the pool meter off. A
// line beside such a key is a SUGGESTED SETTING and not a claim about a
// default, and the seed says so in its own prose ("suggested:", "shapes, not
// recommendations", "ARBITRARY SHAPES chosen to be obviously not anyone's").
// That is why this half's third register is larger than the duration half's
// and why it is not a hole: each row names the reason there is no constant
// for its line to drift from, and each row's staleness half fails when it
// stops describing anything.
//
// THE ONE DIFFERENCE FROM THE DURATION HALF, stated rather than quietly
// taken: seedDurationDefaultNotAConstant carries a STRONGER half — a key in
// it must also stay UNDOCUMENTED, because its one member's default is a rule
// with no number at all, so a documented line for it would be a claim about
// nothing. seedNumberDefaultNotAConstant carries no such half. Its one
// member has something a reader can state — GrokGuardWeek's default is an
// explicit "off" — so a documented line beside it is a suggestion rather
// than a false claim, and refusing it would mean deleting a live seed line
// the operator uses to see what shape the setting takes. What keeps the drop
// on the record instead is the row's reason plus the seed-side accounting
// below, which logs the line by name and by file position every run.
//
// It had a second member, and losing it is the distinction worth keeping:
// backup_min_free_mb's default was never a non-constant, only a constant in
// another unit, and "the census cannot pair it" is not the same claim as
// "there is nothing to pair" (ranger-base-c768n finding 3). A register whose
// rows are reasons the MECHANISM stopped short will accumulate live
// documented numbers nothing holds; a row belongs here when the tree states
// no single default, and the fix for anything else is the derivation.

// seedNumberResolvers are the number readers that take the key and the
// default as ARGUMENTS, the duration half's seedDurationResolvers one type
// over. Made total by seedNumberKeyCensus, which derives the same set from
// the tree's function signatures and fails on one this map does not name.
//
// EMPTY TODAY, and that is a measurement and not a stub: no method on App
// has the shape `(key string, def <numeric>, errw io.Writer) <numeric>`
// (MEASURED 2026-10-05 by the census below — `derived 0 call-site
// reader(s)`). budgetDollars and planPercent are the near misses, and they
// miss for the same reason: both are `(key string, errw io.Writer) float64`
// with no default parameter at all, because unset is no cap and no guard
// rather than a number. The map stays because the ARM stays — the
// derivation's call-site pass runs over numeric readers exactly as it does
// over duration ones, so the first numeric call-site reader anyone adds is a
// named failure here instead of a key that quietly resolves nothing.
var seedNumberResolvers = map[string]func(*App, string, float64, io.Writer) float64{}

// seedNumberBodyReaders are the number readers that spell their key and
// their default in their OWN body — one `YamlGet`/`CfgGet` key literal, one
// `Default`-shaped constant returned.
//
// The closure is where the three result types become one, and the
// conversion is hand written, which is a place a mistake could hide: wiring
// a key to the wrong reader, or widening a uint64 wrongly, would feed every
// comparison the wrong number. It cannot sit there green — the pin below
// hands each body-form reader a config with its own key ABSENT and requires
// the answer to be seedNumberDefaults' value for the constant the
// derivation paired it with, so a mis-wired closure reds by name.
var seedNumberBodyReaders = map[string]func(*App, io.Writer) float64{
	"BackupKeep": func(a *App, w io.Writer) float64 { return float64(a.BackupKeep(w)) },
	// The one reader whose answer is not in its key's unit: MB in
	// `backup_min_free_mb:`, bytes out. The widening above derives the << 20
	// from the reader's own return and seedNumberAnswerFor applies it, so
	// this closure stays the plain production read and the scale is stated
	// in exactly one place — the binary's.
	"BackupMinFree":     func(a *App, w io.Writer) float64 { return float64(a.BackupMinFree(w)) },
	"LauncherBehindMax": func(a *App, w io.Writer) float64 { return float64(a.LauncherBehindMax(w)) },
	"LoadGuard":         func(a *App, w io.Writer) float64 { return a.LoadGuard(w) },
	"verifyBatch":       func(a *App, w io.Writer) float64 { return float64(a.verifyBatch(w)) },
}

// seedNumberDefaults is the Default-shaped constant NAME -> its value, the
// duration half's seedDurationDefaults one type over. Hand written, because
// a test cannot evaluate a constant it only parsed — and made total by the
// derivation, which fails on a constant it finds at a reader and cannot find
// here, and on an entry here no reader names any more.
var seedNumberDefaults = map[string]float64{
	"DefaultBackupKeep":        DefaultBackupKeep,
	"DefaultBackupMinFreeMB":   DefaultBackupMinFreeMB,
	"DefaultLauncherBehindMax": DefaultLauncherBehindMax,
	"DefaultVerifyBatch":       DefaultVerifyBatch,
	"LoadGuardDefault":         LoadGuardDefault,
}

// seedDocumentedNumberOnPurposeNotTheDefault records a number key that
// examples/config.yaml documents at a value that is deliberately NOT the
// code default — a suggested setting rather than a statement about the
// harness. Key -> why, and the why is the whole point of the entry.
//
// Unlike its duration twin, this one has a live member, so its arm is a
// check that runs rather than one a future entry would be the first to meet.
var seedDocumentedNumberOnPurposeNotTheDefault = map[string]string{
	"verify_batch": "DefaultVerifyBatch is 1 — one verify bead per close — and the seed documents 4 as a " +
		"shape to copy for a queue whose branching factor is above 1.0, with its own paragraph saying so " +
		"(verifyafter.go's header makes the same argument). The 4 is a suggestion, so this pin must not " +
		"read it as a claim about the default; what it holds instead is that the two still differ",
}

// seedNumberDefaultNotAConstant records a number key the derivation REACHES
// and cannot pair with one constant, so there is no single value a
// documented line could be compared against. Key -> why.
//
// Without it such a key drops out of the census in silence, which is the
// shape ranger-base-2vynj finding 1 was filed for. It carries no "must stay
// undocumented" half — see the section header for why that half is the
// duration twin's and not this one's.
//
// It held backup_min_free_mb until ranger-base-c768n, on the reasoning that
// a reader answering bytes from a key in MB leaves no one number to compare.
// That reasoning was about the census MECHANISM and not about the fact: the
// default IS a constant, in the key's own unit, and the seed documents it in
// that unit, so the thing missing was the SCALE and not the number. The
// derivation reads it now (seedReturnedDefault), the comparison applies it
// (seedNumberAnswerFor), and the key is compared like any other — which is
// why a key whose default is a constant no longer sits under this name.
var seedNumberDefaultNotAConstant = map[string]string{
	"grok_guard_week": "unset IS the guard off and GrokGuardWeek returns a bare 0 rather than a constant " +
		"(the plan_guard_<window>: rule, for the reason that file's header gives), so there is no default " +
		"for the documented 85 to drift from — and the seed's own paragraph calls the three grok numbers " +
		"ARBITRARY SHAPES chosen to be obviously not anyone's",
}

// seedDocumentedNumberOutsideTheCensus records a bare-number line in
// examples/config.yaml whose key the derivation reaches NO reader of either
// shape for, so there is no constant a documented value could drift from.
// Key -> why.
//
// It is the third of the three ways a documented number line can be
// accounted for, and the only one that admits a line nothing holds. As a
// BARE LIST it would be the hole the next key falls into, so the staleness
// half in the pin below is what keeps each row describing something: a key
// here that the seed no longer documents with a number, that the census has
// since learned to compare, or that the derivation now REACHES (which makes
// it the other register's row, not this one's), fails.
var seedDocumentedNumberOutsideTheCensus = map[string]string{
	"autostart_max_beads": "its default is a SHELL literal: plugin/autostart.sh reads the key and falls " +
		"back to 3 in a `case`, so no Go reader of either derived shape reads it and there is no Go " +
		"constant for an AST census over Go sources to compare — outside this census by construction and " +
		"not by omission. It is NOT unheld: ranger-base-m9mwc, filed from this bead, compares the seed " +
		"line to the fallback the script actually applies, derived on both sides and parsing no shell " +
		"(TestAutostartShellFallbacksAreTheSeedsDocumentedValues in autostart_test.go). This row is why " +
		"that pin exists, so a reader who finds the line here goes there rather than concluding nothing " +
		"holds it",
	"budget_day": "budgetDollars is `(key string, errw io.Writer) float64` — the call-site shape with no " +
		"DEFAULT argument, because unset is no cap at all — so neither derived shape reaches it, and the " +
		"seed's own line reads `suggested:`",
	"budget_pass": "budgetDollars, as budget_day: above — no default argument because unset is no cap, and " +
		"the seed's own line reads `suggested:`",
	"grok_pool_usd_per_point": "posse ships NO value for it on purpose: the factor is empirical, derived " +
		"from the operator's own calibration bracket, and it drifts the day xAI reprices " +
		"(GrokPoolUSDPerPoint's header), so it is config rather than a constant and there is nothing in " +
		"the tree for the documented 0.50 to drift from. Its reader also returns `(float64, bool)`, which " +
		"is not a reader shape at all",
	"plan_guard_5h": "read by PlanGuardThresholds' prefix scan over `plan_guard_<window>:`, where the " +
		"window names belong to whichever provider adapter is installed and no key literal exists for the " +
		"derivation to find. No key set is the guard off, so there is no default percent, and the seed " +
		"calls these numbers shapes rather than recommendations",
	"plan_guard_7d": "PlanGuardThresholds' prefix scan, as plan_guard_5h: above — a window name, no key " +
		"literal, and unset is the guard off",
	"uncounted_cap_codex": "the key is per-runtime and composed at the read (`\"uncounted_cap_\"+runtime`), " +
		"so there is no key literal for the derivation to find; unset is unlimited rather than a constant " +
		"(ADR 0013 §5), and the documented 20 is a suggested cap for one runtime this instance happens to " +
		"name",
}

// seedLooksLikeNumber is the number half's discriminator: a bare decimal
// number, with at most the two decorations the tree's own readers strip.
//
// The decorations are derived from the tree and not invented: `$` is
// stripped by budgetDollars and GrokPoolUSDPerPoint, `%` by planPercent and
// GrokGuardWeek, and nothing else is stripped by any numeric reader
// (MEASURED 2026-10-05, every TrimPrefix/TrimSuffix over a raw config value
// in internal/posse). Accepting them is what keeps a documented `$30` or
// `85%` inside the enumeration instead of in a silent hole — the shape this
// bead is about.
//
// It is disjoint from seedLooksLikeDuration by construction: that one
// requires a trailing unit letter and refuses a value ending in a digit or a
// dot, and this one requires the whole value to parse as a float once the
// two decorations are off, which `336h` and `5m` do not. The pin below
// asserts the disjointness over the real seed rather than trusting the
// argument.
func seedLooksLikeNumber(v string) bool {
	if v == "" {
		return false
	}
	v = strings.TrimSuffix(strings.TrimPrefix(v, "$"), "%")
	if v == "" {
		return false
	}
	_, err := strconv.ParseFloat(v, 64)
	return err == nil
}

// seedResolveNumberFor is the production read of one number pairing: a
// closure that answers what a fresh instance would get for a config file,
// computed by the instance's own reader. One closure for both forms, so the
// drift check has a single code path and the grammar is never a test's.
func seedResolveNumberFor(t *testing.T, k seedPairing, def float64) func(cfgPath string, errw io.Writer) float64 {
	t.Helper()
	if k.form == seedFormBody {
		fn, ok := seedNumberBodyReaders[k.reader]
		if !ok {
			t.Fatalf("no body-form reader %q in seedNumberBodyReaders", k.reader)
		}
		return func(p string, w io.Writer) float64 { return fn(&App{ConfigPath: p}, w) }
	}
	fn, ok := seedNumberResolvers[k.reader]
	if !ok {
		t.Fatalf("no resolver %q in seedNumberResolvers", k.reader)
	}
	return func(p string, w io.Writer) float64 { return fn(&App{ConfigPath: p}, k.key, def, w) }
}

// seedNumberAnswerFor is the number a pairing's reader ANSWERS for its own
// default: the constant, scaled by the power of two the reader applies on
// the way out.
//
// For every reader but one it is the constant itself. BackupMinFree is the
// one, and the reason this exists: its key is `backup_min_free_mb:` and its
// answer is bytes, so the constant (384) and the reader's unset answer
// (402653184) are a unit apart, and comparing a documented line against
// either alone is comparing it against the wrong number. Scaled, there IS
// one number — which is the whole of ranger-base-c768n finding 3, and the
// scale is DERIVED from the reader's own return expression rather than
// stated here, so a reader that stops shifting stops being compared that way
// on the same parse.
func seedNumberAnswerFor(k seedPairing, def float64) float64 {
	return def * float64(uint64(1)<<k.shift)
}

// seedNumberUnit says how a pairing's answer relates to its constant, for a
// failure message that has to name two numbers in two units without the
// reader guessing which is which.
func seedNumberUnit(k seedPairing) string {
	if k.shift == 0 {
		return ""
	}
	return fmt.Sprintf(" (the reader answers the constant << %d, so the key's own unit is 2^%d smaller than its answer)", k.shift, k.shift)
}

// seedNumberDrift resolves the text a documented line carries through the
// production reader and says what is wrong with it, or "" when the file and
// the constant agree. seedDefaultDrift's argument, one type over — see that
// function's comment for why the resolution is a real read of a real config
// file and why the reader's stderr is checked too.
func seedNumberDrift(t *testing.T, resolve func(cfgPath string, errw io.Writer) float64, key, text string, def float64) (float64, string) {
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
		return got, fmt.Sprintf("%s is not a value this reader accepts, so the default stands and the line proves nothing: %s", text, s)
	}
	if got != def {
		return got, fmt.Sprintf("documents %s (= %v), the constant is %v", text, got, def)
	}
	return got, ""
}

// Property 4, the number half: every number key examples/config.yaml
// documents is documented at its own default, compared as a number through
// the harness's own reader (ranger-base-p9qve).
func TestSeedConfigDocumentedNumberDefaultsAreTheConstants(t *testing.T) {
	t.Parallel()
	// qibRepoRoot, not a hand-rolled climb: the tree-wide door census in
	// internal/treepins derives this pin's class from that one helper
	// (ranger-base-sx2dq), and a pin outside the class gets no door.
	root := qibRepoRoot(t)

	c := seedNumberKeyCensus(t, root)
	keys, scanned := c.keys, c.scanned
	// Positive witnesses: a pin over a derived set is satisfied by deriving
	// nothing, so say what was measured and fail a walk that measured too
	// little to mean anything.
	if scanned < 20 {
		t.Fatalf("parsed %d non-test .go files under %s — the walk found no tree, so the census below measures nothing", scanned, root)
	}
	t.Logf("parsed %d non-test .go files, derived %d number key/constant pairings through %d call-site reader(s) (%s) and %d body-form reader(s) (%s)",
		scanned, len(keys), len(c.resolvers), strings.Join(c.resolvers, ", "), len(c.readers), strings.Join(c.readers, ", "))

	// THE THREE TABLES ARE TOTAL, the duration half's rule said over the
	// number half's tables: a reader the tree has and a table does not is a
	// failure that names the reader, because the alternative is every key
	// that reader owns reading as undocumented and nothing held about them
	// (ranger-base-khqvr).
	for _, got := range []struct {
		kind  string
		found []string
		named []string
		table string
	}{
		{"call-site", c.resolvers, seedSortedKeys(seedNumberResolvers), "seedNumberResolvers"},
		{"body-form", c.readers, seedSortedKeys(seedNumberBodyReaders), "seedNumberBodyReaders"},
	} {
		named := map[string]bool{}
		for _, n := range got.named {
			named[n] = true
		}
		found := map[string]bool{}
		for _, n := range got.found {
			found[n] = true
			if !named[n] {
				t.Errorf("the tree has a %s number reader %s does not name: %s — add an entry for it, converting its result to float64, or this pin reads every key that reader owns as undocumented and holds nothing about them (ranger-base-khqvr)",
					got.kind, got.table, n)
			}
		}
		for _, n := range got.named {
			if !found[n] {
				t.Errorf("%s names %s, which is no longer a %s number reader in the tree — the entry resolves nothing; drop it", got.table, n, got.kind)
			}
		}
	}

	// The constant table, from the other side: an entry no reader names any
	// more is a value this pin can no longer be wrong about.
	namedConstants := map[string]bool{}
	for _, k := range keys {
		namedConstants[k.constant] = true
	}
	for _, name := range seedSortedKeys(seedNumberDefaults) {
		if !namedConstants[name] {
			t.Errorf("seedNumberDefaults names %s, which no number reader falls back to any more — the row holds nothing; drop it", name)
		}
	}

	// A key whose default is not a constant cannot be compared with
	// anything, so it is registered with a reason or it is a failure — never
	// a silent drop.
	unpairedKeys := map[string]bool{}
	for _, u := range c.unpaired {
		unpairedKeys[u.key] = true
		why, registered := seedNumberDefaultNotAConstant[u.key]
		if !registered {
			t.Errorf("%s (%s) is a number key this pin cannot pair with a constant: %s — give it a constant default, or add it to seedNumberDefaultNotAConstant with the reason, so the drop is on the record instead of silent",
				u.key, u.site, u.why)
			continue
		}
		if strings.TrimSpace(why) == "" {
			t.Errorf("seedNumberDefaultNotAConstant[%q] carries no reason — the entry drops a key out of this census, so the why IS the entry", u.key)
		}
		t.Logf("%s: %s — registered as having no constant default (%s)", u.key, u.why, u.site)
	}
	for _, key := range seedSortedKeys(seedNumberDefaultNotAConstant) {
		if !unpairedKeys[key] {
			t.Errorf("seedNumberDefaultNotAConstant names %q, which no number reader reads with a non-constant default any more — either it has a constant now (drop the entry, the census will pair it) or the key is gone", key)
		}
	}

	// The floor, below the totality checks on purpose: when a whole reader
	// rule stops matching, the blocks above name each table entry that no
	// longer resolves and this says how much of the census went with it. A
	// Fatalf, because every comparison below a census this thin is vacuous.
	//
	// Five is the measured population (MEASURED 2026-10-09, four until
	// ranger-base-c768n gave the scaled rule its member): backup_keep
	// through BackupKeep, backup_min_free_mb through BackupMinFree,
	// launcher_behind_max through LauncherBehindMax, load_guard through
	// LoadGuard, verify_batch through verifyBatch. It is deliberately not
	// thirteen — the count of documented lines — because the house
	// vocabulary for a number key is "unset means off" and most of this
	// file's numbers have no constant at all; the section header has the
	// reason and the third register has the rows.
	if len(keys) < 5 {
		t.Fatalf("derived %d number key/constant pairings from %d files, want at least 5 — the two reader rules in seedReaderDecl have stopped matching the tree's number readers, so this pin holds almost nothing", len(keys), scanned)
	}

	// One key, one default.
	byKey := map[string]seedPairing{}
	for _, k := range keys {
		if prev, dup := byKey[k.key]; dup && prev.constant != k.constant {
			t.Errorf("%s falls back to %s at %s and to %s at %s — one key with two defaults, so no documented value can be right about it",
				k.key, prev.constant, prev.site, k.constant, k.site)
			continue
		}
		byKey[k.key] = k
	}

	// The constant table's VALUES, the hand-written float64 conversion in
	// seedNumberBodyReaders, AND the derived scale, cross-checked where the
	// tree lets them be. A body-form reader handed a config with its own key
	// absent returns its own default, so a pairing the parse got wrong — or
	// a closure wired to the wrong reader, or a widening that lost the value
	// — cannot sit here green.
	//
	// backup_min_free_mb is the case that proves it bites, and it is now on
	// this side of the line rather than registered as unpairable: its reader
	// answers `DefaultBackupMinFreeMB << 20`, so this arm is what holds the
	// 20 the parse read against the 20 the binary applies. A shift the
	// derivation got wrong fails HERE, by name, before any documented line
	// is compared against it (ranger-base-c768n finding 3).
	absent := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(absent, []byte("# every key commented out\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if k.form != seedFormBody {
			continue
		}
		def, known := seedNumberDefaults[k.constant]
		if !known {
			continue // already reported below
		}
		var errw strings.Builder
		want := seedNumberAnswerFor(k, def)
		if got := seedResolveNumberFor(t, k, want)(absent, &errw); got != want {
			t.Errorf("%s over a config with %s absent returns %v, and %s (%v) scaled by the shift the derivation read (<< %d) is %v (%s) — either the derivation paired the key with the wrong constant, read the wrong scale, or the seedNumberBodyReaders closure is wired wrong, so every comparison it feeds is against the wrong number",
				k.reader, k.key, got, k.constant, def, k.shift, want, k.site)
		}
	}

	cfg := seedConfigPath(t)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

	compared := 0
	comparedKeys := map[string]bool{}
	for _, key := range seedSortedKeys(byKey) {
		k := byKey[key]
		def, known := seedNumberDefaults[k.constant]
		if !known {
			t.Errorf("%s (%s) falls back to %s, which seedNumberDefaults does not name — add `%q: %s,` to it so this pin can compare the seed's documented value against it",
				k.key, k.site, k.constant, k.constant, k.constant)
			continue
		}

		// The key must still be COMMENTED in the seed, for the duration
		// half's reason: seedDocumentedValue reads comment lines, so an
		// armed key would make THIS pin vacuous for it — it would find
		// nothing and skip in silence.
		if yamlHasKey(cfg, k.key) {
			t.Errorf("seed config declares %s: live — a number key must ship commented out (%s reads the live file, so arming it changes every instance), and an armed key is invisible to this pin", k.key, k.reader)
			continue
		}

		value, lines := seedDocumentedValue(text, k.key)
		switch {
		case lines == 0:
			// Undocumented is allowed: a key the seed says nothing about
			// makes no claim that can go stale. Logged, not failed.
			t.Logf("%s: undocumented in examples/config.yaml (default %v) — nothing to hold", k.key, def)
			continue
		case lines > 1:
			t.Errorf("examples/config.yaml documents %s on %d commented lines — two documented values for one key, and a reader cannot tell which is the default", k.key, lines)
			continue
		}

		compared++
		comparedKeys[k.key] = true
		// `want` and not `def`: for every reader but BackupMinFree they are
		// the same number, and for that one the answer is the constant
		// shifted — so this is the one line that turns "a default exists but
		// in another unit" into "there is one number to compare"
		// (ranger-base-c768n finding 3).
		want := seedNumberAnswerFor(k, def)
		got, drift := seedNumberDrift(t, seedResolveNumberFor(t, k, want), k.key, value, want)
		if why, deliberate := seedDocumentedNumberOnPurposeNotTheDefault[k.key]; deliberate {
			// The register's live half: the entry says this line is NOT the
			// default, so the line matching the default makes the entry a
			// lie, and the next reader believes the wrong one.
			if drift == "" {
				t.Errorf("examples/config.yaml documents %s at %s, which IS %s (%v)%s — but seedDocumentedNumberOnPurposeNotTheDefault says it is deliberately not the default (%q). Drop the register entry, or restore the value it describes",
					k.key, value, k.constant, def, seedNumberUnit(k), why)
			} else {
				t.Logf("%s: documented %q is deliberately not the default (%s = %v): %s", k.key, value, k.constant, def, drift)
			}
			continue
		}
		if drift != "" {
			t.Errorf("examples/config.yaml documents %s as the default and %s says otherwise: %s%s.\n"+
				"  the line is a claim a fresh instance reads; fix the line, or — if the value is documented at something other than its default on purpose — say so in seedDocumentedNumberOnPurposeNotTheDefault",
				k.key, k.site, drift, seedNumberUnit(k))
			continue
		}
		t.Logf("%s: documented %q resolves to %v = %s (%s %s)%s", k.key, value, got, k.constant, k.reader, k.form, seedNumberUnit(k))
	}

	// THE DIRECTION THAT HURTS (ranger-base-ghcx3 finding 1, said over
	// numbers). Everything above walks the tree's readers and asks the seed
	// about each key it found. This walks the SEED and asks whether each
	// bare-number line it ships was reached — which is the direction the
	// whole of this bead is: six of the seven keys ranger-base-p9qve named
	// are reached by no number reader at all, so a census that only asked
	// the tree would have held one line and reported nothing about the rest.
	documented := seedDocumentedNumbers(text)
	documentedKeys := map[string]bool{}
	for _, d := range documented {
		documentedKeys[d.key] = true
	}

	// The two matchers must agree about what a documented line IS, or the set
	// check below is a set over the wrong lines. Derived rather than counted:
	// every key the loop above COMPARED has to be one this enumeration found.
	for _, key := range seedSortedKeys(comparedKeys) {
		if !documentedKeys[key] {
			t.Errorf("this pin compared a documented value for %s, and seedDocumentedNumbers did not find that line — the two line matchers disagree, so the coverage check below is reading a different file than the comparison above",
				key)
		}
	}

	for _, d := range documented {
		switch {
		case comparedKeys[d.key]:
			// Reached, compared, held.
		case seedNumberDefaultNotAConstant[d.key] != "":
			t.Logf("examples/config.yaml:%d documents %s as %s — the derivation reaches the key and cannot pair it, so the line is a suggested setting and not a claim about a default (%s)",
				d.line, d.key, d.value, seedNumberDefaultNotAConstant[d.key])
		case seedDocumentedNumberOutsideTheCensus[d.key] != "":
			t.Logf("examples/config.yaml:%d documents %s as %s — registered as outside the census (%s)",
				d.line, d.key, d.value, seedDocumentedNumberOutsideTheCensus[d.key])
		default:
			t.Errorf("examples/config.yaml:%d documents %s as %s, and this census never reached that key: a fresh instance reads that line as a statement about the harness and nothing holds it, so the number can be wrong by any factor in silence (ranger-base-p9qve).\n"+
				"  three ways out, and they are not interchangeable: give the key a reader of one of the two derived shapes (a call-site reader, or `(errw io.Writer) <numeric>` naming its key and returning one Default-shaped constant) so this pin COMPARES it; or add it to seedNumberDefaultNotAConstant if the derivation reaches it and cannot pair it; or add it to seedDocumentedNumberOutsideTheCensus with the reason there is no constant for the line to drift from",
				d.line, d.key, d.value)
		}
	}

	// The third register's stale half, as loud as the other two's. A row here
	// admits a documented line nothing compares, so each of the ways it can
	// stop describing anything is a failure — the last of them being what
	// keeps this register and seedNumberDefaultNotAConstant from both
	// claiming the same key: the discriminator is whether the derivation
	// REACHES it, which is mechanical and not prose.
	for _, key := range seedSortedKeys(seedDocumentedNumberOutsideTheCensus) {
		why := seedDocumentedNumberOutsideTheCensus[key]
		if strings.TrimSpace(why) == "" {
			t.Errorf("seedDocumentedNumberOutsideTheCensus[%q] carries no reason — the row admits a documented number nothing holds, so the why IS the row", key)
		}
		if !documentedKeys[key] {
			t.Errorf("seedDocumentedNumberOutsideTheCensus names %q, which examples/config.yaml no longer documents with a number value — the row admits a line that is not there; drop it", key)
		}
		if comparedKeys[key] {
			t.Errorf("seedDocumentedNumberOutsideTheCensus names %q, which this pin now COMPARES against %s — the key has a constant after all, so the row is excusing a line that is already held; drop it and let the comparison be what holds it", key, byKey[key].constant)
		}
		if unpairedKeys[key] {
			t.Errorf("seedDocumentedNumberOutsideTheCensus names %q, which the derivation now REACHES through a reader it cannot pair — that is seedNumberDefaultNotAConstant's row and not this one's, and two registers claiming one key is how each stops being read", key)
		}
	}

	// THE TWO SEED-SIDE ENUMERATIONS MUST NOT BOTH CLAIM ONE LINE, or a key
	// is accounted for in one half's registers while the other half's set
	// check demands it too — and two registers over one line is how each
	// stops being read. The claim lives here, in the pin the widening gave a
	// door, rather than in a test of its own: a pin over this ONE tracked
	// file is not the tree-wide class (treewidedoor_qa_test.go is explicit
	// that a reading of one path at the root is not), so a test of its own
	// would get no door and would be a ~950s package run away, which is the
	// gap the doors exist to close.
	//
	// The floors are the positive witnesses that stop it passing by finding
	// nothing on one side.
	durationLines := seedDocumentedDurations(text)
	if len(durationLines) < 13 {
		t.Errorf("seedDocumentedDurations found %d documented duration lines in the seed, want at least 13 (MEASURED 2026-10-09 — twelve, plus backup_interval from ranger-base-s1dmh) — the number half's disjointness check below is reading one empty side", len(durationLines))
	}
	if len(documented) < 13 {
		t.Errorf("seedDocumentedNumbers found %d documented number lines in the seed, want at least 13 (MEASURED 2026-10-09 — eleven as of ranger-base-p9qve and ranger-base-y13h7, plus backup_keep and backup_min_free_mb from ranger-base-s1dmh)", len(documented))
	}
	t.Logf("the seed documents %d duration-valued and %d bare-number commented lines", len(durationLines), len(documented))
	for _, d := range durationLines {
		if !documentedKeys[d.key] {
			continue
		}
		t.Errorf("%s is enumerated as a duration (examples/config.yaml:%d) and as a number — the two discriminators overlap, so both halves demand an accounting for one line and a register row in either one looks stale from the other", d.key, d.line)
	}

	// The floor, kept beside the set check above and not replaced by it,
	// because the two fail in opposite directions: this one catches a
	// documented line that LEFT (deleting one reds at `compared 3`), and the
	// set catches one that ARRIVED, which no count can see.
	//
	// Four is the measured population (MEASURED 2026-10-09): the seed
	// documents launcher_behind_max, load_guard, verify_batch and — since
	// ranger-base-s1dmh gave the backup keys a block — backup_keep with a
	// number the census reaches. It was three until that bead, with
	// backup_keep the one pairing the seed said nothing about.
	if compared < 4 {
		t.Errorf("compared %d documented number defaults, want at least 4 — the seed documents backup_keep, launcher_behind_max, load_guard and verify_batch with a value this census reaches, so a census that found fewer stopped reading the file", compared)
	}
}

// The number register's stale half, as loud as an undocumented key. An entry
// in seedDocumentedNumberOnPurposeNotTheDefault silences the drift complaint
// for one key, so each of the three ways it can quietly stop describing
// anything is a failure: the key is no longer a number key, the seed no
// longer documents it, or the value it describes no longer parses.
//
// (The fourth way — the documented value has become the default again — is
// held in the pin above, where the comparison already is.)
func TestSeedConfigDocumentedNumberRegisterIsNotStale(t *testing.T) {
	t.Parallel()
	if len(seedDocumentedNumberOnPurposeNotTheDefault) == 0 {
		t.Log("seedDocumentedNumberOnPurposeNotTheDefault is empty — every number key the seed documents is documented at its own default")
		return
	}
	root := qibRepoRoot(t)
	byKey := map[string]seedPairing{}
	for _, k := range seedNumberKeyCensus(t, root).keys {
		byKey[k.key] = k
	}
	cfg := seedConfigPath(t)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range seedSortedKeys(seedDocumentedNumberOnPurposeNotTheDefault) {
		why := seedDocumentedNumberOnPurposeNotTheDefault[key]
		if strings.TrimSpace(why) == "" {
			t.Errorf("seedDocumentedNumberOnPurposeNotTheDefault[%q] carries no reason — an entry here silences the documented-default pin for that key, so the why IS the entry", key)
		}
		k, derived := byKey[key]
		if !derived {
			t.Errorf("seedDocumentedNumberOnPurposeNotTheDefault names %q, which no number reader pairs with a constant any more — the key, or the reader, is gone, and the entry now excuses nothing", key)
			continue
		}
		value, lines := seedDocumentedValue(string(b), key)
		if lines != 1 {
			t.Errorf("seedDocumentedNumberOnPurposeNotTheDefault names %q, and examples/config.yaml documents it on %d commented lines — an entry describing a line that is not there is stale; drop it", key, lines)
			continue
		}
		def, known := seedNumberDefaults[k.constant]
		if !known {
			t.Errorf("seedDocumentedNumberOnPurposeNotTheDefault names %q, whose constant %s is not in seedNumberDefaults", key, k.constant)
			continue
		}
		want := seedNumberAnswerFor(k, def)
		_, drift := seedNumberDrift(t, seedResolveNumberFor(t, k, want), key, value, want)
		if strings.Contains(drift, "is not a value this reader accepts") || strings.Contains(drift, "no value") {
			t.Errorf("examples/config.yaml documents %s at %q, and the register says that is deliberate — but %s: a value documented on purpose still has to be a value an instance could type", key, value, drift)
		}
	}
}

// The number matcher has to be able to say no, or the pin above is a
// spelling exercise. Drives seedNumberDrift over planted texts rather than
// the tree, so it is not itself a tree-wide pin.
//
// Two readers, on purpose. The carrier is float64 for both, but the GRAMMAR
// is each reader's own — LoadGuard parses a float and accepts 0 as its
// documented escape hatch, verifyBatch parses an int and refuses 0 — so the
// same text is drift through one and not the other, which is the whole
// reason this resolves through the production reader instead of comparing
// numbers a test parsed itself.
func TestSeedDocumentedNumberDriftCheckCanStillSayNo(t *testing.T) {
	t.Parallel()

	// LoadGuardDefault, said locally so a change to the constant does not
	// move this control's subject.
	const loadDef = 25.0
	load := seedPairing{key: "load_guard", constant: "LoadGuardDefault", reader: "LoadGuard", form: seedFormBody}
	for _, tc := range []struct {
		name  string
		text  string
		drift bool
	}{
		{"the spelled default", "25", false},
		// Compared as a NUMBER, not as text: these three are the same load
		// average and a text compare would call two of them drift.
		{"the same default with a decimal point", "25.0", false},
		{"the same default with padding the reader trims", " 25 ", false},
		{"a different ceiling", "50", true},
		{"zero, which LoadGuard accepts and means the guard off", "0", true},
		{"negative, which it refuses", "-1", true},
		{"a documented line with no value at all", "", true},
		{"prose where a value should be", "twenty-five", true},
		// A duration where a number belongs: the reader refuses it, which is
		// what keeps a line that proves nothing from passing by landing on
		// the very number it was supposed to prove.
		{"a duration where a number belongs", "25h", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, drift := seedNumberDrift(t, seedResolveNumberFor(t, load, loadDef), load.key, tc.text, loadDef)
			if got := drift != ""; got != tc.drift {
				t.Errorf("seedNumberDrift(%q) through LoadGuard drift=%v (%q), want drift=%v", tc.text, got, drift, tc.drift)
			}
		})
	}

	// And through an INT reader, which is the half the duration census could
	// not have at all. `1.5` is the row that matters: it is a number, so a
	// float64 compare against the default would have had an opinion about
	// it, and verifyBatch's own grammar refuses it — the line proves nothing.
	const batchDef = 1.0 // DefaultVerifyBatch
	batch := seedPairing{key: "verify_batch", constant: "DefaultVerifyBatch", reader: "verifyBatch", form: seedFormBody}
	for _, tc := range []struct {
		name  string
		text  string
		drift bool
	}{
		{"the spelled default", "1", false},
		{"the value the seed suggests", "4", true},
		{"a float where a count belongs", "1.5", true},
		{"zero, which this reader refuses", "0", true},
		{"prose where a value should be", "one", true},
	} {
		t.Run("int-reader/"+tc.name, func(t *testing.T) {
			t.Parallel()
			_, drift := seedNumberDrift(t, seedResolveNumberFor(t, batch, batchDef), batch.key, tc.text, batchDef)
			if got := drift != ""; got != tc.drift {
				t.Errorf("seedNumberDrift(%q) through verifyBatch drift=%v (%q), want drift=%v", tc.text, got, drift, tc.drift)
			}
		})
	}

	// The discriminator, including the two decorations the tree's own
	// readers strip and every spelling that is NOT a bare number — so the
	// next reader meets the rule as a decision and not a bug.
	for _, tc := range []struct {
		text string
		num  bool
	}{
		{"25", true}, {"0", true}, {"384", true}, {"1209600", true},
		{"0.50", true}, {"-1", true}, {"$30", true}, {"85%", true},
		{"", false}, {"$", false}, {"%", false},
		{"336h", false}, {"5m", false}, {"0s", false}, {"14d", false},
		{"two weeks", false}, {"true", false}, {"wed 14:30", false},
	} {
		if seedLooksLikeNumber(tc.text) != tc.num {
			t.Errorf("seedLooksLikeNumber(%q) = %v, want %v", tc.text, !tc.num, tc.num)
		}
	}

	// And the SEED-SIDE enumeration, over planted text: it is the half that
	// fails on a documented number the census never reached, so a rule nobody
	// has watched refuse anything is a rule that will admit the next key.
	enum := "# load_guard: 25            # a load average\n" +
		"# budget_pass: $30           # the dollar spelling budgetDollars strips\n" +
		"# grok_guard_week: 85%       # the percent spelling GrokGuardWeek strips\n" +
		"# model_probe_ttl: 1h        # a duration, which is the OTHER half's line\n" +
		"# grok_pool_reset: wed 14:30\n" +
		"# autostart_dry_run: false\n" +
		"# `verify_batch:` is the key the gate reads\n" +
		"  # backup_keep: 3\n" +
		"# attn_parked_age: two weeks\n" +
		"verify_batch: 9\n"
	var names []string
	for _, d := range seedDocumentedNumbers(enum) {
		names = append(names, fmt.Sprintf("%s=%s@%d", d.key, d.value, d.line))
	}
	if want := []string{"load_guard=25@1", "budget_pass=$30@2", "grok_guard_week=85%@3"}; !slices.Equal(names, want) {
		t.Errorf("seedDocumentedNumbers found %v, want %v — it must read the value before the trailing comment, take the $ and %% spellings, and refuse a duration, a time, a boolean, prose, a key merely NAMED in prose, an indented line and a LIVE key",
			names, want)
	}
}

// The number derivation has to be able to say no, and to say it about each
// near-miss, or the pin above is a rule nobody has watched refuse anything —
// which is exactly how six duration keys sat outside the duration census for
// six weeks (ranger-base-khqvr). Drives the whole census, both passes, over a
// PLANTED tree rather than the repo, so it is not itself a tree-wide pin and
// a change to the harness's own readers cannot move its subject.
//
// Three of its rows exist only on this side of the widening:
//
//   - the CALL-SITE arm, which has no live member at all
//     (seedNumberResolvers is empty today), so this planted tree is the only
//     place the number half's call-site pass is ever exercised;
//   - the SCALED constant (`DefaultPlantedShift << 20`), which is
//     BackupMinFree's real shape — MB in the key, bytes in the answer. It
//     was a NEAR-MISS until ranger-base-c768n, reported rather than paired,
//     on the reasoning that a default returned through an expression states
//     no single number; it is a PAIRING now, carrying the shift, because the
//     constant is the number the key's own unit is in and the shift is what
//     relates the two. Its near-miss moved one step along with it:
//     `planted_two_scales` returns one constant both bare and shifted, which
//     is two stated defaults and no single answer, and is still reported;
//   - a time.Duration reader, which must be invisible here exactly as a
//     numeric one is invisible to the duration half. The two halves share
//     one derivation and differ only in `isValue`, so a leak either way
//     would make one half's registers stale from the other's side.
func TestSeedNumberDerivationCanStillSayNo(t *testing.T) {
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
	PlantedKeepDefault       = 3
	DefaultPlantedCeiling    = 25.0
	PlantedViaCfgGetDefault  = 7
	DefaultPlantedCap        = 12
	DefaultPlantedShift      = 384
	plantedBareConstant      = 9
	DefaultPlantedGrace      = time.Hour
)

// A body-form int reader naming its constant with the SUFFIX spelling.
func (a *App) PlantedKeep(errw io.Writer) int {
	if YamlGet(a.ConfigPath, "planted_keep") == "" {
		return PlantedKeepDefault
	}
	return PlantedKeepDefault
}

// The same shape in float64 with the PREFIX spelling: both are live in the
// real tree, and the result types differ between real readers too.
func (a *App) PlantedCeiling(errw io.Writer) float64 {
	_ = YamlGet(a.ConfigPath, "planted_ceiling")
	return DefaultPlantedCeiling
}

// uint64 through CfgGet, whose key is its FIRST argument where YamlGet's is
// its second.
func (a *App) PlantedViaCfgGet(errw io.Writer) uint64 {
	_ = a.CfgGet("planted_via_cfgget", "")
	return PlantedViaCfgGetDefault
}

// Near-miss 1: the default is a rule, not a constant.
func (a *App) PlantedRuleDefault(errw io.Writer) int {
	_ = YamlGet(a.ConfigPath, "planted_rule_default")
	return a.plantedRule()
}

func (a *App) plantedRule() int { return 1 }

// Near-miss 2: one reader, two keys — no single constant is its default.
func (a *App) PlantedTwoKeys(errw io.Writer) int {
	if YamlGet(a.ConfigPath, "planted_first") != "" {
		return PlantedKeepDefault
	}
	_ = YamlGet(a.ConfigPath, "planted_second")
	return PlantedKeepDefault
}

// Near-miss 3: the fallback is not Default-shaped at all.
func (a *App) PlantedNoConstant(errw io.Writer) int {
	_ = YamlGet(a.ConfigPath, "planted_no_constant")
	return plantedBareConstant
}

// The SCALED form, this half's own and BackupMinFree's shape: MB in the key,
// bytes in the answer. A pairing with a shift of 20 (ranger-base-c768n).
func (a *App) PlantedShifted(errw io.Writer) uint64 {
	_ = YamlGet(a.ConfigPath, "planted_shifted")
	return DefaultPlantedShift << 20
}

// Near-miss 5, this half's own: ONE constant returned at TWO scales, so the
// reader states two defaults and no single answer. The scaled rule's own
// negative case — without it the derivation would pair whichever spelling
// the parse reached first and every comparison for the key would be against
// a number the reader answers only half the time.
func (a *App) PlantedTwoScales(errw io.Writer) uint64 {
	if YamlGet(a.ConfigPath, "planted_two_scales") == "" {
		return DefaultPlantedShift
	}
	return DefaultPlantedShift << 20
}

// A call-site reader: it reads the key it was HANDED, so its keys live at
// its call sites and not here. The number half has no live member of this
// shape, which is why it is planted.
func (a *App) plantedCap(key string, def int, errw io.Writer) int {
	_ = YamlGet(a.ConfigPath, key)
	return def
}

// A thin wrapper: no key of its own, and the pairing is one line down.
func (a *App) PlantedCapped(errw io.Writer) int {
	return a.plantedCap("planted_capped", DefaultPlantedCap, errw)
}

// Near-miss 4: a call site whose default argument is a call.
func (a *App) PlantedCallSiteRule(errw io.Writer) int {
	return a.plantedCap("planted_call_rule", a.plantedRule(), errw)
}

// Not a method on App.
func plantedFree(errw io.Writer) int {
	_ = YamlGet("", "planted_free")
	return PlantedKeepDefault
}

// A DURATION reader: the other half's subject, and invisible to this one.
func (a *App) PlantedGrace(errw io.Writer) time.Duration {
	_ = YamlGet(a.ConfigPath, "planted_grace")
	return DefaultPlantedGrace
}

// Neither half's subject: a bool and a string are not numbers, and a
// documented one is not a number claim.
func (a *App) PlantedFlag(errw io.Writer) bool {
	_ = YamlGet(a.ConfigPath, "planted_flag")
	return false
}

func (a *App) PlantedName(errw io.Writer) string {
	_ = YamlGet(a.ConfigPath, "planted_name")
	return "x"
}
`
	// A reader in a _test.go file is not the harness, and a file that does
	// not parse is the build's finding and not this pin's: both are skipped,
	// and the scanned count is what says so.
	const plantedTest = `package planted

import "io"

func (a *App) PlantedInATestFile(errw io.Writer) int {
	_ = YamlGet(a.ConfigPath, "planted_in_a_test_file")
	return PlantedKeepDefault
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

	c := seedNumberKeyCensus(t, root)
	if c.scanned != 1 {
		t.Errorf("scanned %d files of the planted tree, want 1 — the walk read the _test.go file or the file that does not parse", c.scanned)
	}

	if got, want := strings.Join(c.resolvers, ","), "plantedCap"; got != want {
		t.Errorf("derived call-site readers %q, want %q", got, want)
	}
	if got, want := strings.Join(c.readers, ","), "PlantedCeiling,PlantedKeep,PlantedShifted,PlantedViaCfgGet"; got != want {
		t.Errorf("derived body-form readers %q, want %q", got, want)
	}

	gotPairs := map[string]string{}
	for _, k := range c.keys {
		// The SHIFT is in the expectation, not just the constant: a
		// derivation that found the right constant and dropped its scale
		// would pair the key against a number the reader never answers
		// (ranger-base-c768n finding 3).
		gotPairs[k.key] = seedRetDefault{name: k.constant, shift: k.shift}.spelling() + " via " + k.reader + " (" + k.form.String() + ")"
		if k.site == "" {
			t.Errorf("pairing for %s carries no site — a census that cannot say WHERE cannot be acted on", k.key)
		}
	}
	for key, want := range map[string]string{
		"planted_keep":       "PlantedKeepDefault via PlantedKeep (body-form reader)",
		"planted_ceiling":    "DefaultPlantedCeiling via PlantedCeiling (body-form reader)",
		"planted_via_cfgget": "PlantedViaCfgGetDefault via PlantedViaCfgGet (body-form reader)",
		"planted_shifted":    "DefaultPlantedShift<<20 via PlantedShifted (body-form reader)",
		"planted_capped":     "DefaultPlantedCap via plantedCap (call site)",
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
		// each near-miss, reported rather than dropped
		"planted_rule_default", "planted_first", "planted_second",
		"planted_no_constant", "planted_two_scales", "planted_call_rule",
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

	// And the keys nothing should have seen at all: a free function, the
	// OTHER half's duration reader, a bool, a string, a test file, and the
	// key a wrapper never reads itself.
	for _, key := range []string{
		"planted_free", "planted_grace", "planted_flag", "planted_name", "planted_in_a_test_file",
	} {
		if got, paired := gotPairs[key]; paired {
			t.Errorf("%s was paired as %s; it is not a number reader on App", key, got)
		}
		for _, u := range c.unpaired {
			if u.key == key {
				t.Errorf("%s was reported unpairable; it is not a number reader on App at all, so the rule has widened past its subject", key)
			}
		}
	}

	// The other direction of the same leak, over the same planted tree: the
	// DURATION half must see planted_grace and nothing numeric. One
	// derivation, two predicates — this is the assertion that they are two.
	d := seedDurationKeyCensus(t, root)
	gotDur := map[string]string{}
	for _, k := range d.keys {
		gotDur[k.key] = k.constant
	}
	if got, want := gotDur["planted_grace"], "DefaultPlantedGrace"; got != want {
		t.Errorf("the duration half paired planted_grace with %q, want %q — the shared derivation has stopped reaching duration readers", got, want)
	}
	delete(gotDur, "planted_grace")
	for key, constant := range gotDur {
		t.Errorf("the duration half paired %s with %s over the number half's planted tree — isValue is leaking, so each half's registers look stale from the other's side", key, constant)
	}
}
