package posse

// QA pin for ranger-base-lm22v (ADR 0024 D2/D4 residue, ruled on
// ranger-base-imiif): every OpsPatterns hit in this repo's TRACKED MARKDOWN is
// dispositioned by a ruled shape, or this test is red.
//
// WHY A SHAPE TABLE AND NOT ZERO. ranger-base-99ps's DONE WHEN was "zero
// OpsPatterns hits in the public tree", and that bar is unmeetable by
// construction: the credential class exists to name the mechanism ADR 0019
// documents, so the ADR that documents it can never be clean. Every class
// carries hits that are the SOFTWARE's own public vocabulary (a vendor's list
// price, a vendor's window mechanic, the values examples/config.yaml
// documents, a runtime's credential-store path) alongside the facts the class
// exists to catch, and only a SHAPE tells the two apart. The bar that can hold
// is therefore not "zero hits" but "zero hits OUTSIDE the ruled shapes" —
// and a shape table is only a bar if adding to it is a reviewed edit, the same
// class as PublicDocsGenres. DO NOT WIDEN OR NARROW A SHAPE WITHOUT A RULING;
// every ruling below was made by the security reviewer on ranger-base-lm22v,
// and each answers ADR 0024 D1's question: could ANY deployer of this
// software have written this line?
//
// SCOPE, and it is the whole point of the pin's honesty. This measures TRACKED
// MARKDOWN, from the REPOSITORY ROOT — the same scope ADR 0024 D2 check 2
// states, and no more. It is not a tree-wide property: in source the fixture
// shape IS the target shape (visibility.go's own assembled plan-brand names
// are the canonical case), so 434 line-hits in 93 tracked non-markdown files
// (MEASURED at 53170e9, ranger-base-4v7f9) are detector vocabulary, not
// residue. Source is held by check 3's derived identity literals and by a
// value-equality sweep at verify time, never by this table. A green run here
// says nothing about code.
//
// Scoped to the package directory `git ls-files` returns zero markdown files
// and this pin would measure nothing while passing, so the census asserts it
// found files — and the planted controls below assert the shapes can still
// say no. A pin with no red arm pins nothing.
//
// MUTATION-CHECKED (2026-09-02, 20 mutants, all killed): deleting any one of
// C1, C2, P1, P2, G1, G2, G3, K1, K2, K3 or either K-RED reds the standing
// tree or a planted control; widening G3's path limit to the whole tree greens
// the ADR control; dropping any of the three value right edges greens a longer
// figure sharing a ruled prefix; relaxing the containment test in
// dispositionOf to a line-level match greens a line carrying a ruled token
// beside an unruled one; and dropping or inverting the G2 key filter mints
// shapes from prose lines the config parse picked up. Everything after the
// first clause is held by the controls alone — nothing in the standing tree
// would have noticed any of it.

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// opsAllow is one ruled shape: a line that carries it may keep the hit it
// covers. path, when set, limits the ruling to a corner of the tree.
type opsAllow struct {
	class string
	name  string
	key   string // the examples/config.yaml key a derived shape came from
	why   string
	path  *regexp.Regexp
	re    *regexp.Regexp
}

// opsRed is a shape that reds a line of its class regardless of which allows
// cover it. unless, when set, is the one spelling the red does not fire on —
// two regexes because RE2 has no lookahead and "with a -s item that is not the
// runtime's own" is not a single pattern.
type opsRed struct {
	class  string
	name   string
	why    string
	re     *regexp.Regexp
	unless *regexp.Regexp
}

// opsHit is one OpsPattern match on one line, with the byte range it matched:
// disposition is per HIT, not per line. A line-level allow would let one ruled
// token wave through an unruled one beside it (`the list price is $3/MTok and
// the pass ran $715/wk` is one line and two very different facts), so an allow
// counts only when its own match CONTAINS the hit's range.
type opsHit struct {
	path  string
	num   int
	line  string
	class string
	text  string
	lo    int
	hi    int
}

// exampleConfigValues parses the commented key lines of examples/config.yaml —
// the shipped example IS the public vocabulary the G2 and C2 rulings rest on.
// Derived at test time and never hardcoded here: a pin asserting against the
// constant it guards pins nothing (a change to the example must move the pin
// with it, not be blessed by it).
func exampleConfigValues(t *testing.T, root string) map[string][]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "examples", "config.yaml"))
	if err != nil {
		t.Fatalf("examples/config.yaml is the anchor for the G2/C2 shapes: %v", err)
	}
	keyLine := regexp.MustCompile(`^#[ \t]*([a-z][a-z0-9_]*):[ \t]+([^ \t#]+)`)
	out := map[string][]string{}
	for _, l := range strings.Split(string(body), "\n") {
		if m := keyLine.FindStringSubmatch(l); m != nil {
			out[m[1]] = append(out[m[1]], m[2])
		}
	}
	if len(out) < 20 {
		t.Fatalf("parsed %d documented keys out of examples/config.yaml — the parse, not the file, is broken", len(out))
	}
	return out
}

// opsShapeTable is the ruling, in code. Every shape carries the ruling that
// admitted it; a shape with no ruling is a hole.
func opsShapeTable(t *testing.T, cfg map[string][]string) ([]opsAllow, []opsRed) {
	t.Helper()

	// A value's right edge: the documented value and not a longer one that
	// starts with it ($30 is not $300, plan_guard_5h: 7 is not 70). The
	// decimal case ($250 against $250.75) is caught by containment instead —
	// the OpsPatterns hit there is longer than this match, so nothing covers it.
	const rightEdge = `([^0-9]|$)`
	const keyValue = `[[:space:]]*:[[:space:]]*`

	allow := []opsAllow{
		{
			class: "cost", name: "C1", why: "a vendor's public list price per million tokens — every deployer's fact, and NOTES.md names this residual itself",
			re: regexp.MustCompile(`\$[0-9]+(\.[0-9]+)?/MTok`),
		},
	}
	// C2: the blessed ceilings (ranger-base-axft option B1, ratified
	// 2026-08-27) — a dollar figure equal to what examples/config.yaml states
	// for budget_pass:/budget_day:, bare or with /pass or /day. While the
	// example carries placeholders instead of numbers the ceilings have no
	// public anchor and the ADR 0018 lines quoting them are HONESTLY RED;
	// that is the pin working, not a bug in it.
	for _, key := range []string{"budget_pass", "budget_day"} {
		for _, v := range cfg[key] {
			if _, err := strconv.Atoi(v); err != nil {
				continue
			}
			allow = append(allow, opsAllow{
				class: "cost", name: "C2 (" + key + ")", key: key,
				why: "the ceiling examples/config.yaml documents for " + key + ": — public vocabulary since the axft bless",
				re:  regexp.MustCompile(`\$` + regexp.QuoteMeta(v) + `(/pass|/day)?` + rightEdge),
			})
		}
	}

	// PLAN. The brands are ASSEMBLED here for the reason visibility.go
	// assembles them: written whole, a detector's fixture and a deployment
	// stating its own plan are the same bytes, and the seed preflight's
	// plan-brand grep over the ship set cannot tell them apart.
	allow = append(allow,
		opsAllow{
			class: "plan", name: "P1", why: "a vendor's window mechanic (the week has no intra-week reset), not whose account this is",
			re: regexp.MustCompile(`Super` + `Grok` + `[[:space:]]week`),
		},
		opsAllow{
			class: "plan", name: "P2", why: "the name of a software feature — the ADR 0034 title",
			re: regexp.MustCompile(`Codex` + `[[:space:]]plan[[:space:]]hint`),
		},
	)

	// GUARD. G1: the sentinel vocabulary the docs must be able to show —
	// plan_guard_blind_max: 0 is the documented disable, autostart_max_beads: 0
	// the documented unbounded. The key half is deliberately loose: the class
	// only fires on the guard ERE's own keys, so the hit has already proved
	// which key it is and repeating that list here would only rot.
	allow = append(allow, opsAllow{
		class: "guard", name: "G1", why: "a documented sentinel value (true/false/0), not a threshold in force",
		re: regexp.MustCompile(`[a-z][a-z0-9_]*` + keyValue + `(true|false|0)([^0-9A-Za-z_]|$)`),
	})
	// G2: the value examples/config.yaml documents for THAT key. A deployment
	// quoting the shipped defaults tells nothing (the ranger-base-xsw5 class:
	// derivable from public defaults). Only keys the guard class can actually
	// fire on get a shape, and WHICH THOSE ARE is asked of the shipped pattern
	// rather than listed here: the parse above also picks up prose lines that
	// happen to read `word: value`, and a shape built from one of those is a
	// regex nothing ruled.
	for _, key := range sortedConfigKeys(cfg) {
		if !opsClassFires("guard", key+": 0") {
			continue
		}
		for _, v := range cfg[key] {
			allow = append(allow, opsAllow{
				class: "guard", name: "G2 (" + key + ")", key: key,
				why: "the value examples/config.yaml documents for " + key + ":",
				re:  regexp.MustCompile(regexp.QuoteMeta(key) + keyValue + regexp.QuoteMeta(v) + `([^0-9A-Za-z_]|$)`),
			})
		}
	}
	// G3: a unit cap is the smallest cap that can trip — a reproduction
	// fixture in two write-ups (ranger-base-2y96, ranger-base-lasj), not a
	// threshold in force. Ruled for docs/notes.d only, which is where
	// reproductions live.
	allow = append(allow, opsAllow{
		class: "guard", name: "G3", why: "a unit overflow cap in a write-up: the smallest cap that can trip, a reproduction fixture",
		path: regexp.MustCompile(`^docs/notes\.d/`),
		re:   regexp.MustCompile(`plan_guard_overflow_cap` + keyValue + `1` + rightEdge),
	})

	// CREDENTIAL — the class is a MAP, never a value. K1/K2/K3 rule the map
	// shapes that are the vendors' and the runtime's, identical on every box;
	// the reds below fire on a value regardless. `refresh_token` is in the
	// shipped ERE and deliberately has NO shape here: it is the one credential
	// spelling with no vendor-documented location behind it, so a new
	// occurrence is red until somebody rules it.
	allow = append(allow,
		opsAllow{
			class: "credential", name: "K1", why: "a runtime's own credential-store path, vendor-documented and identical on every box (ADR 0019 is about them)",
			re: regexp.MustCompile(`(~/\.claude/)?\.credentials\.json[A-Za-z0-9._*-]*` +
				`|(~/\.codex/)?auth\.json` +
				`|(~/\.grok/)?auth\.json`),
		},
		opsAllow{
			class: "credential", name: "K2", why: "an env var NAME — the name is the vendor's, the value is the fact (a value makes it K-RED)",
			re: regexp.MustCompile(`(OAUTH_TOKEN|API_KEY)`),
		},
		opsAllow{
			class: "credential", name: "K3", why: "a keychain read with no item, or with the runtime's own default service name",
			re: regexp.MustCompile(`find-generic-password`),
		},
	)

	red := []opsRed{
		{
			class: "credential", name: "K-RED (value)",
			why: "a credential-class line also carrying a value-shaped token — the class exists to keep the map without the keys",
			re:  regexp.MustCompile(`sk-ant-|xai-|ghp_|eyJ[A-Za-z0-9_-]{10,}|[A-Za-z0-9+/_-]{40,}|(OAUTH_TOKEN|API_KEY)=[A-Za-z0-9]`),
		},
		{
			class: "credential", name: "K-RED (item)",
			why:    "a keychain item name chosen by an instance — the map this class exists for; the runtime's own default is K3",
			re:     regexp.MustCompile(`find-generic-password.*-s[[:space:]]+[^[:space:]]`),
			unless: regexp.MustCompile(`-s[[:space:]]+` + regexp.QuoteMeta(KeychainService)),
		},
	}
	return allow, red
}

// opsClassFires asks the SHIPPED pattern of a class whether it matches s —
// one list, so a shape table built on "which keys are guard keys" cannot
// disagree with the detector it is dispositioning.
func opsClassFires(class, s string) bool {
	for _, p := range OpsPatterns {
		if p.Class == class && p.Match(s) {
			return true
		}
	}
	return false
}

func sortedConfigKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// disposition returns "" when the hit is ruled, else why it is not.
func dispositionOf(h opsHit, allow []opsAllow, red []opsRed) string {
	for _, r := range red {
		if r.class != h.class || !r.re.MatchString(h.line) {
			continue
		}
		if r.unless != nil && r.unless.MatchString(h.line) {
			continue
		}
		return r.name + ": " + r.why
	}
	for _, a := range allow {
		if a.class != h.class {
			continue
		}
		if a.path != nil && !a.path.MatchString(h.path) {
			continue
		}
		for _, m := range a.re.FindAllStringIndex(h.line, -1) {
			if m[0] <= h.lo && m[1] >= h.hi {
				return ""
			}
		}
	}
	return "no ruled shape of the " + h.class + " class covers it"
}

// scanOpsHits walks every tracked markdown file under root and returns one
// opsHit per (line, pattern, match).
func scanOpsHits(t *testing.T, root string) ([]opsHit, int) {
	t.Helper()
	args := append([]string{"-C", root, "ls-files", "-z", "--"}, MarkdownPathspecs...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		t.Fatalf("git ls-files over %v: %v", MarkdownPathspecs, err)
	}
	var hits []opsHit
	files := 0
	for _, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if rel == "" {
			continue
		}
		f, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		files++
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		n := 0
		for sc.Scan() {
			n++
			line := sc.Text()
			for _, p := range OpsPatterns {
				for _, m := range p.re.FindAllStringIndex(line, -1) {
					hits = append(hits, opsHit{
						path: rel, num: n, line: line, class: p.Class,
						text: line[m[0]:m[1]], lo: m[0], hi: m[1],
					})
				}
			}
		}
		if err := sc.Err(); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		f.Close()
	}
	return hits, files
}

func repoRootForOpsScan(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not inside a git checkout")
	}
	return strings.TrimSpace(string(out))
}

func TestQAEveryOpsHitInTrackedMarkdownIsRuled(t *testing.T) {
	t.Parallel()
	root := repoRootForOpsScan(t)
	allow, red := opsShapeTable(t, exampleConfigValues(t, root))

	hits, files := scanOpsHits(t, root)
	// Scoped to internal/posse, ls-files returns zero markdown files and the
	// loop below would pass having measured nothing.
	if files == 0 {
		t.Fatalf("git ls-files %v under %s returned no markdown files — this pin measured nothing", MarkdownPathspecs, root)
	}
	if len(hits) == 0 {
		t.Fatalf("censused %d markdown files and found no OpsPatterns hit at all — the shipped detector, not the tree, is what changed", files)
	}

	census := map[string]int{}
	bad := 0
	for _, h := range hits {
		census[h.class]++
		if why := dispositionOf(h, allow, red); why != "" {
			bad++
			t.Errorf("%s:%d — undispositioned %s hit %q\n  %s\n  line: %s\n%s",
				h.path, h.num, h.class, h.text, why, h.line, OpsProseWayThrough)
		}
	}
	t.Logf("%d tracked markdown files, %d hits (cost %d, plan %d, guard %d, credential %d), %d undispositioned",
		files, len(hits), census["cost"], census["plan"], census["guard"], census["credential"], bad)
}

// The controls. "Every hit is ruled" is satisfied by a table that rules
// everything, so plant one line per class that MUST be red and one that must
// be green, and require the same disposition function to say so.
func TestQAOpsShapeTableCanStillSayNo(t *testing.T) {
	t.Parallel()
	root := repoRootForOpsScan(t)
	cfg := exampleConfigValues(t, root)
	allow, red := opsShapeTable(t, cfg)

	// Assembled, not written whole — same reason as the P1/P2 shapes above.
	claudeMax := "Claude" + " Max"

	for _, c := range []struct {
		name  string
		line  string
		class string
		path  string // "" = a public doc outside docs/notes.d
		want  bool   // true = must be dispositioned (green)
	}{
		// One red and one green per class — the four rulings, exercised.
		{"cost: a live burn figure", "the pass ran $715/wk against the ceiling", "cost", "", false},
		{"cost: a vendor list price", "the list price is $3/MTok for input", "cost", "", true},
		{"plan: whose account this is", "the account is on " + claudeMax + " and the fleet is inside it", "plan", "", false},
		{"plan: a vendor window mechanic", "the " + "Super" + "Grok" + " week has no intra-week reset", "plan", "", true},
		{"guard: a threshold in force", "plan_guard_5h: 42", "guard", "", false},
		{"guard: a documented sentinel", "`plan_guard_blind_max: 0` is the escape hatch", "guard", "", true},
		{"credential: an instance's keychain item", "security find-generic-password -s acme-prod -w", "credential", "", false},
		{"credential: a runtime's own store", "off darwin it is `~/.claude/.credentials.json`", "credential", "", true},

		// The edges each ruling is only worth anything at.
		//
		// Disposition is per HIT, not per line: a line carrying one ruled
		// token and one unruled token of the same class is red. Mutating the
		// containment test in dispositionOf back to a line-level match greens
		// this row and nothing else in the tree notices.
		{"cost: a ruled price beside an unruled burn", "the list price is $3/MTok, and the pass ran $715/wk", "cost", "", false},
		// A ceiling's right edge: $250 is ruled, $250.75 is a different figure.
		{"cost: a longer figure sharing the ceiling's prefix", "a degraded day bounded at $250.75/day", "cost", "", false},
		{"cost: a longer figure sharing the ceiling's prefix, no dot", "a day bounded at $3000/day", "cost", "", false},
		// And a value's right edge in the guard class, both ways: 7 is not 70
		// and 700 is not 70 either. Both edges are load-bearing — containment
		// alone catches the short spelling and not the long one.
		{"guard: a prefix of the documented value", "`plan_guard_5h: 7` is not the documented 70", "guard", "", false},
		{"guard: a longer value sharing the documented prefix", "`plan_guard_5h: 700` is not the documented 70", "guard", "", false},
		{"guard: a longer value sharing a sentinel's prefix", "`plan_guard_blind_max: 07` is not the documented disable", "guard", "", false},
		// G3 is ruled for docs/notes.d only — a unit cap is a reproduction
		// fixture where reproductions live, and a threshold anywhere else.
		{"guard: a unit overflow cap in a write-up", "with `plan_guard_overflow_cap: 1` and a full pool", "guard", "docs/notes.d/control.md", true},
		{"guard: the same unit cap in an ADR", "with `plan_guard_overflow_cap: 1` and a full pool", "guard", "docs/adr/0000-control.md", false},
		{"guard: a cap that is not the unit cap", "with `plan_guard_overflow_cap: 15` and a full pool", "guard", "docs/notes.d/control.md", false},
		// K-RED fires through K1/K2/K3: the map is ruled, a value never is.
		{"credential: a store path beside a token", "`~/.claude/.credentials.json` held sk-ant-notarealtoken", "credential", "", false},
		{"credential: an env var name with a value", "export ANTHROPIC_API_KEY=abcd1234 in the env set", "credential", "", false},
		{"credential: the runtime's own keychain item", "security find-generic-password -s " + KeychainService + " -w", "credential", "", true},
	} {
		path := c.path
		if path == "" {
			path = "docs/adr/0000-control.md"
		}
		var got []opsHit
		for _, p := range OpsPatterns {
			for _, m := range p.re.FindAllStringIndex(c.line, -1) {
				got = append(got, opsHit{
					path: path, num: 1, line: c.line,
					class: p.Class, text: c.line[m[0]:m[1]], lo: m[0], hi: m[1],
				})
			}
		}
		if len(got) == 0 {
			t.Errorf("control %q: %q tripped no OpsPattern at all — the control measures nothing", c.name, c.line)
			continue
		}
		ruled := true
		for _, h := range got {
			if h.class != c.class {
				continue
			}
			if why := dispositionOf(h, allow, red); why != "" {
				ruled = false
			}
		}
		if ruled != c.want {
			verb := "was dispositioned"
			if !ruled {
				verb = "was NOT dispositioned"
			}
			t.Errorf("control %q (%s class): %q %s, want the opposite", c.name, c.class, c.line, verb)
		}
	}

	// The G2 shapes are derived from a parse of examples/config.yaml, and that
	// parse also picks up prose lines that read `word: value`. A shape minted
	// from one of those is a regex nothing ruled, so require every derived
	// shape's key to be a key the guard class can actually fire on — the
	// narrowing is measured here because nothing in the standing tree would
	// notice its absence.
	for _, a := range allow {
		if !strings.HasPrefix(a.name, "G2 (") {
			continue
		}
		if !opsClassFires("guard", a.key+": 0") {
			t.Errorf("shape %s was minted for %q, which the guard class never fires on — the examples/config.yaml parse leaked a prose line into the table", a.name, a.key)
		}
	}

	// And the C2/G2 anchors must be REAL: derived from the example, not from a
	// placeholder. If examples/config.yaml goes back to placeholders the
	// ceilings lose their public anchor and ADR 0018's lines are honestly red
	// — say which, rather than letting a quiet table shrink.
	for _, key := range []string{"budget_pass", "budget_day"} {
		numeric := false
		for _, v := range cfg[key] {
			if _, err := strconv.Atoi(v); err == nil {
				numeric = true
			}
		}
		if !numeric {
			t.Errorf("examples/config.yaml documents no numeric %s: — the C2 shape has no anchor and the ADR 0018 ceilings are unruled (ranger-base-axft option B1)", key)
		}
	}
}
