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
// where it cannot drift: a duration key is read at exactly one call site
// that names its key literal AND its Default* constant in the same argument
// list — `a.attnAge("attn_parked_age", DefaultAttnParkedAge, errw)`. So the
// set is parsed out of the tree, and the only hand-maintained part is the
// constant NAME -> value table below, which the derivation makes total: a
// new duration key reds this pin until it is in it.
//
// THE REGISTER is seedDocumentedOnPurposeNotTheDefault, for a key the file
// documents at something other than its default on purpose. It is empty
// today, and its stale half is as loud as an undocumented key: an entry
// whose key is no longer a duration key, is no longer documented, or whose
// documented value has quietly become the default again, fails.

// seedDurationResolvers are the config readers whose second argument is the
// default: the functions whose grammar decides what a documented value
// MEANS. The pin resolves through the real function rather than
// reimplementing either grammar, so `336h` vs `1209600` vs `14d` is settled
// by the code the instance runs and not by a second parser in a test.
var seedDurationResolvers = map[string]func(*App, string, time.Duration, io.Writer) time.Duration{
	"attnAge":    (*App).attnAge,
	"graceAfter": (*App).graceAfter,
}

// seedDurationDefaults is the Default* constant NAME -> its value. Hand
// written, because a test cannot evaluate a constant it only parsed — and
// made total by the derivation in seedDurationKeySites, which fails on a
// constant it finds at a call site and cannot find here.
var seedDurationDefaults = map[string]time.Duration{
	"DefaultAttnQuestionAge":    DefaultAttnQuestionAge,
	"DefaultAttnGuardStuck":     DefaultAttnGuardStuck,
	"DefaultAttnParkedAge":      DefaultAttnParkedAge,
	"DefaultVerifyBoxMaxAge":    DefaultVerifyBoxMaxAge,
	"DefaultCrewReapAfter":      DefaultCrewReapAfter,
	"DefaultUnpointedReapAfter": DefaultUnpointedReapAfter,
	"DefaultRetireTreeAfter":    DefaultRetireTreeAfter,
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

// seedDurationKey is one derived pairing: a config key, the Default*
// constant it falls back to, and the reader whose grammar resolves it.
type seedDurationKey struct {
	key      string
	constant string
	resolver string
	site     string
}

// seedDurationKeySites parses the pairing out of the tree. A call to one of
// seedDurationResolvers whose first argument is a string literal and whose
// second is an identifier named Default* states "this key defaults to this
// constant" in one expression, which is the only place in the harness where
// the two are written down together.
func seedDurationKeySites(t *testing.T, root string) (keys []seedDurationKey, scanned int) {
	t.Helper()
	fset := token.NewFileSet()
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
		scanned++
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			var fn string
			switch f := call.Fun.(type) {
			case *ast.Ident:
				fn = f.Name
			case *ast.SelectorExpr:
				fn = f.Sel.Name
			}
			if _, ok := seedDurationResolvers[fn]; !ok {
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
			id, ok := call.Args[1].(*ast.Ident)
			if !ok || !strings.HasPrefix(id.Name, "Default") {
				return true
			}
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				rel = p
			}
			keys = append(keys, seedDurationKey{
				key:      key,
				constant: id.Name,
				resolver: fn,
				site:     fmt.Sprintf("%s:%d", filepath.ToSlash(rel), fset.Position(call.Pos()).Line),
			})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].key < keys[j].key })
	return keys, scanned
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
// value that does not parse makes attnAge name it and return the default,
// so a text that is not a duration at all would otherwise pass this pin by
// landing on the very number it was supposed to prove.
func seedDefaultDrift(t *testing.T, resolver, key, text string, def time.Duration) (time.Duration, string) {
	t.Helper()
	if text == "" {
		return 0, "the line carries no value, so it documents nothing"
	}
	fn, ok := seedDurationResolvers[resolver]
	if !ok {
		t.Fatalf("no resolver %q", resolver)
	}
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte(key+": "+text+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errw strings.Builder
	got := fn(&App{ConfigPath: cfg}, key, def, &errw)
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

	keys, scanned := seedDurationKeySites(t, root)
	// Positive witnesses: a pin over a derived set is satisfied by deriving
	// nothing, so say what was measured and fail a walk that measured too
	// little to mean anything.
	if scanned < 20 {
		t.Fatalf("parsed %d non-test .go files under %s — the walk found no tree, so the census below measures nothing", scanned, root)
	}
	if len(keys) < 4 {
		t.Fatalf("derived %d duration key/constant pairings from %d files, want at least 4 — the call-site rule in seedDurationKeySites has stopped matching the tree, so this pin holds nothing", len(keys), scanned)
	}
	t.Logf("parsed %d non-test .go files, derived %d duration key/constant pairings", scanned, len(keys))

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

	cfg := seedConfigPath(t)
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

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
			t.Errorf("seed config declares %s: live — a duration key must ship commented out (attnAge reads the file, so arming it changes every instance), and an armed key is invisible to this pin", k.key)
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
		got, drift := seedDefaultDrift(t, k.resolver, k.key, value, def)
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
		t.Logf("%s: documented %q resolves to %s = %s", k.key, value, got, k.constant)
	}

	if compared < 3 {
		t.Errorf("compared %d documented duration defaults, want at least 3 — the seed documents attn_question_age, attn_guard_stuck, attn_parked_age and verify_box_max_age, so a census that found fewer stopped reading the file", compared)
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
		if _, drift := seedDefaultDrift(t, k.resolver, key, value, def); strings.Contains(drift, "does not parse") || strings.Contains(drift, "no value") {
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
			_, drift := seedDefaultDrift(t, "attnAge", "attn_parked_age", tc.text, def)
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
}
