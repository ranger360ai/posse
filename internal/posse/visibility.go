package posse

// The beads visibility guard (rangerhq-hrz) — the seatbelt under the
// routing rule in NOTES.md's "Privacy model".
//
// The fleet aggregates beads from more than one repo (config `beads:`), and
// beads inherit their repo's visibility. Nothing structural stopped
// instance-ops content — cost figures, plan names, credential locations,
// live guard settings — from being filed into a db whose repo is public.
// Routing discipline is the real control; this is the seatbelt for the day
// a beads repo is public again.
//
// AND IT IS A LINT, NOT A BOUNDARY — same class as the L1 allowlist. The
// boundary is the routing rule plus repo visibility; the lint exists so a
// mis-routed bead is a refusal at commit time instead of a public artifact.
//
// One pattern list, two readers: the prepare-commit-msg hook (gates.go)
// greps ADDED lines of `.beads/*.jsonl` with these EREs, and the harness
// warns with the same ones before it files a bead itself. The EREs are
// therefore written in the intersection of POSIX ERE and Go's regexp: no
// `\t`/`\s`/`\d`/`\b` (POSIX brackets instead), and no single quote (they
// are rendered into single-quoted shell words).

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"strings"
)

// Visibility marks. UNMARKED IS PUBLIC — fail closed, so a newly added repo
// gets the guard until someone states it is private, rather than silence
// until someone remembers to mark it public.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// VisibilityOverrideEnv / VisibilityOverrideValue are the operator's escape
// hatch, and the value is spelled out on purpose: an override that a stray
// `=1` could produce is not a decision. Nothing in the harness ever puts
// this in a session's environment (TestVisibilityOverrideIsNeverDispatched)
// — the way through for a persona is to re-file the bead in the private db.
const (
	VisibilityOverrideEnv   = "RHQ_VISIBILITY_OVERRIDE"
	VisibilityOverrideValue = "i-mean-it"
)

// VisibilityRule is the rule a refusal names, quoted from NOTES.md's
// "Privacy model" — the refusal has to say which rule it is enforcing or
// it is just a regex saying no.
const VisibilityRule = `NOTES.md "Privacy model": a bead belongs in a public repo's db only when any
deployer of this software could have written it. Everything that describes ONE
deployment goes in that instance's private db — instance ops, cost and plan
data, credential locations, the deployment's security posture — even when the
code it touches is this repo's.`

// VisibilityWayThrough is the remedy, and it is the same one every time:
// the bead is not wrong, its db is.
const VisibilityWayThrough = `the way through: re-file the bead in the instance's PRIVATE db and cite its id
from the public one. A private bead can be re-filed public later; the reverse
is a history purge.`

// PublicDocsGenres is the docs/ subdirectories a staged NEW file may land in,
// in a public-stamped repo (ADR 0024 D2 check 1) — a shipped constant, not
// config: admitting a genre to the public tree is a reviewed code change,
// which is exactly the review the wall exists to force. A file staged
// directly under docs/, with no subdirectory, has no genre and is refused
// the same as an unlisted one.
var PublicDocsGenres = []string{"adr", "runbooks", "notes.d"}

// DocsGenreRule is what a check-1 refusal names.
const DocsGenreRule = `ADR 0024 D2 check 1: a public repo's docs/ tree only carries genres any
deployer of this software could have written — an allowlisted docs/
subdirectory, shipped as a constant beside OpsPatterns rather than config,
because admitting a genre to the public tree is a reviewed code change.`

// DocsGenreWayThrough names both ways through a check-1 refusal: the
// instance tree, or a deliberate constant edit — both named so a session
// does not have to guess which one applies.
const DocsGenreWayThrough = `the way through, either one: write it in the instance tree instead, whichever
worktree this session holds — or, if this genre belongs in every deployer's
public tree, add it to PublicDocsGenres in internal/posse/visibility.go (a
deliberate, reviewed code change) and re-run posse gates install-hooks.`

// OpsProseRule is what a check-2 (markdown prose) refusal names — the same
// classification test as VisibilityRule, restated for prose rather than a
// bead: ADR 0024 D1 extends NOTES.md's "Privacy model" to every artifact,
// not only beads.
const OpsProseRule = `NOTES.md "Privacy model", extended to every artifact by ADR 0024 D1: an
artifact belongs in a public repo only when any deployer of this software
could have written it. Instance-ops content — cost figures, plan names, live
guard values, credential locations — belongs in the instance's private tree
even in prose, even when the file it is written into is this repo's.`

// MarkdownPathspecs is WHICH FILES check 2 owns, as git pathspecs, and it
// is a list in one place for the reason OpsPatterns is (ranger-base-4b1z4):
// the wall's markdown spellings are a decision, and a decision spelled
// inline in the rendered hook is one nothing can measure or widen.
//
// Two facts it encodes, both measured (git 2.50.1, ranger-base-4b1z4):
//
//   - GIT PATHSPEC MATCHING IS CASE-SENSITIVE, so the earlier `*.md` did not
//     match `docs/adr/x.MD` — `git diff --cached -U0 HEAD -- '*.md'` came
//     back EMPTY for it and check 2 never saw the file at all. One character
//     defeated the arm, and on macOS's default case-insensitive filesystem
//     the two spellings are the same file to everything except git.
//     `:(icase)` is git's own pathspec magic for exactly this.
//   - `.markdown` IS markdown. It walked through for the same reason and is
//     the same class of artifact; the wall owns both spellings and no
//     others. A spelling nothing writes (`.mkd`, `.mdown`) is not added
//     speculatively — widening this list is the same deliberate, reviewed
//     edit PublicDocsGenres is.
//
// Check 3 is unaffected either way: it scans every staged text file
// whatever it is named.
var MarkdownPathspecs = []string{":(icase)*.md", ":(icase)*.markdown"}

// OpsProseWayThrough is the remedy a check-2 refusal names: restate the
// invariant rather than the measurement (ADR 0024 D3, restate-and-cite).
const OpsProseWayThrough = `the way through (ADR 0024 D3, restate-and-cite): write the full account in
the instance tree first, then have the public document restate the mechanism
and cite the instance artifact by bead id or a private-path label — never
excerpt the measurement itself.`

// OpsPattern is one class of instance-ops content: a name for the refusal,
// the ERE both readers use, and what it is looking for.
type OpsPattern struct {
	Class string
	ERE   string
	Why   string
	re    *regexp.Regexp
}

// Match reports whether s carries this class, and MatchedText returns what
// matched — a refusal that shows the operator the string it tripped on is
// actionable; one that only names a class is a puzzle.
func (p OpsPattern) Match(s string) bool { return p.re.MatchString(s) }

func (p OpsPattern) MatchedText(s string, n int) []string {
	return p.re.FindAllString(s, n)
}

// OpsPatterns is THE list — one place, both readers.
//
// The start set came from rangerhq-hrz; every narrowing below was measured
// against this repo's own 402-bead public db rather than argued, because a
// lint that fires on a quarter of commits is a lint that gets uninstalled:
//
//   - `\$[0-9]` matched 37 beads, 22 of them shell positional parameters
//     (`"$1"`, `$0`, `$2`) quoted in beads about the hooks themselves. The
//     money shapes below keep every real figure ($715/wk, $160.26)
//     and drop every `$1`. Residual, stated: a single-digit amount ($0, $4)
//     and a vendor's public list price ($3/MTok) are on opposite sides of
//     the rule and the same side of the regex — the price gets flagged, the
//     $0 does not.
//   - The config-key class needs a VALUE, not the key: `budget_pass:` and
//     `plan_guard_<window>:` are this harness's own public
//     vocabulary and appear in prose constantly (43 beads on the bare
//     names, 17 with a number after the colon — and those 17 are almost all
//     genuine: live thresholds, an armed autostart interval).
//   - Bare `keychain` is dropped: the plan-usage adapter reading the macOS
//     keychain is the SOFTWARE's mechanism and is documented publicly in
//     examples/config.yaml (19 beads). `security find-generic-password -s
//     <item>` is the map to where the keys live, and stays.
//   - `~/.config/rhq` with a value on the same line is dropped: 57 beads
//     name the harness's own documented config home. What that pattern was
//     reaching for — a live config value — is what the config-key class
//     already catches, with a tenth of the noise.
var OpsPatterns = []OpsPattern{
	{
		Class: "cost",
		Why:   "a dollar figure — spend, per-bead cost, a window's burn",
		ERE: `\$[0-9]+\.[0-9]` +
			`|\$[0-9][0-9,]+` +
			`|\$[0-9]+(\.[0-9]+)?/(wk|week|day|mo|month|hr|hour|pass|bead|session|MTok)` +
			`|\$[0-9]+(\.[0-9]+)?[kKmM]([^A-Za-z]|$)`,
	},
	{
		Class: "plan",
		Why:   "a subscription plan's name or size — whose account this is",
		// THE BRAND NAMES ARE ASSEMBLED, NOT WRITTEN WHOLE, and the joins are
		// load-bearing — do not "simplify" them back into one literal. This
		// file ships in the public repo, and the seed preflight's plan-brand
		// check (docs/runbooks/0012-seed-publication.sh check 2) cannot tell a
		// detector's target from a deployment stating its own plan: written
		// whole, they are the same bytes. Splitting at the space keeps that
		// check's grep over the ship set at zero while the compiled ERE is
		// unchanged — including in this comment, which is why it names the
		// check instead of quoting its phrases; go read them there.
		// TestPlanBrandsAreNotShippedVerbatim pins both halves.
		// The alternative — asking the security reviewer to rule pattern
		// constants PASS-class, the way ADR 0003's $0 sentence is — was
		// declined: the script's author says do not wave a hit through,
		// and a detector is not special enough to be the one exception
		// (rangerhq-vm43).
		ERE: `Claude` + ` Max` +
			`|Max [0-9]+x` +
			`|Super` + `Grok` +
			`|Codex` + ` plan`,
	},
	{
		Class: "guard",
		Why:   "a live guard/budget/autostart value — what is set HERE",
		ERE: `(plan_guard_[a-z0-9_]*|budget_pass|budget_day|plan_usage_ttl` +
			`|model_preflight|model_probe_ttl|dispatch_epoch|autostart_[a-z_]*)` +
			`[[:space:]]*:[[:space:]]*([0-9]|true|false)`,
	},
	{
		Class: "credential",
		Why:   "where the keys live — a map, even with no secret values",
		ERE:   `find-generic-password|credentials\.json|auth\.json|refresh_token|OAUTH_TOKEN|API_KEY`,
	},
}

func init() {
	// The shipped list is held to the same dialect an instance's own
	// patterns are (validateOpsERE) — one validator, so a rule that is
	// enforced against config cannot quietly stop applying here. Panic at
	// init, not at commit time.
	for i := range OpsPatterns {
		if err := validateOpsERE(OpsPatterns[i].ERE); err != nil {
			panic("rhq: OpsPatterns " + OpsPatterns[i].Class + ": " + err.Error())
		}
		OpsPatterns[i].re = regexp.MustCompile(OpsPatterns[i].ERE)
	}
}

// ScanOps returns the classes of instance-ops content s carries, over the
// shipped list plus whatever set adds (the zero OpsPatternSet is the
// shipped list alone — a caller that knows nothing about an instance still
// gets the guard, never an empty one). Empty is the common case and the
// cheap one.
func ScanOps(s string, set OpsPatternSet) []OpsPattern {
	var hits []OpsPattern
	for _, p := range set.All() {
		if p.Match(s) {
			hits = append(hits, p)
		}
	}
	return hits
}

// ─── which repos are public ──────────────────────────────────────────────────

// BeadsVisibility answers what config says about the beads db in the repo at
// dir, and how it knows. The mark is EXPLICIT and LOCAL — a one-level map in
// the operator's config, never a gh or network lookup at guard time:
//
//	beads_visibility:
//	  ~/src/myproject: public
//	  ~/src/myproject-instance: private
//
// Only an explicit `private` buys the exemption. An unlisted repo, a typo'd
// path, an unreadable config and a value that is neither word all come back
// public — every way of being unsure fails the same way, closed.
func (a *App) BeadsVisibility(dir string) (visibility, source string) {
	want := resolvedPath(dir)
	for _, kv := range YamlMapPairs(a.ConfigPath, "beads_visibility") {
		if resolvedPath(ExpandTilde(kv[0])) != want {
			continue
		}
		switch v := strings.ToLower(strings.TrimSpace(kv[1])); v {
		case VisibilityPrivate:
			return VisibilityPrivate, "config beads_visibility:"
		case VisibilityPublic:
			return VisibilityPublic, "config beads_visibility:"
		default:
			return VisibilityPublic, fmt.Sprintf("config beads_visibility: says %q, which is neither public nor private — treated as public", v)
		}
	}
	return VisibilityPublic, "unmarked in config beads_visibility: — unmarked is public (fail closed)"
}

// resolvedPath is how two spellings of one repo are compared: expanded,
// cleaned, and with symlinks resolved when they resolve (/tmp → /private/tmp
// on macOS is the case that bites, and it bites tests first).
func resolvedPath(p string) string {
	p = filepath.Clean(ExpandTilde(p))
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	return p
}

// WarnOpsContent is the cheap in-session half of the guard: the harness
// says so BEFORE it files a bead of its own into a public repo's db. It
// warns and returns — the refusal lives in the commit hook, which catches
// every entry path (bd create, comments, sync, hand edits) rather than the
// one this process happens to own. Reports whether it warned.
func (a *App) WarnOpsContent(w io.Writer, dir, what, text string) bool {
	vis, _ := a.BeadsVisibility(dir)
	if vis != VisibilityPublic {
		return false
	}
	hits := ScanOps(text, a.OpsPatternSet())
	if len(hits) == 0 {
		return false
	}
	var classes []string
	for _, h := range hits {
		classes = append(classes, h.Class)
	}
	fmt.Fprintf(w, "visibility: %s carries ops-class content (%s) and %s is public — %s\n",
		what, strings.Join(classes, ", "), AbbrevHome(dir), "the commit hook will refuse it; re-file it in the private db (NOTES.md, Privacy model)")
	return true
}

// ─── an instance's own vocabulary ────────────────────────────────────────────

// OpsPatternsConfigKey is the config map through which ONE instance teaches
// this lint its own confidential vocabulary — a client's name, a project
// codename — without patching the harness. The shipped list is what any
// deployer of this software needs; a name that is confidential HERE is not
// the public repo's to carry, and config is how it stays out of it.
//
//	beads_visibility_patterns:
//	  client-acme: Acme[[:space:]]*(Corp|Holdings)
//	  codename: (BLUEBIRD|REDSHIFT)
//
// The key is the class the refusal names; the value is an ERE in the same
// two-reader dialect as the shipped list. Flat-YAML's limits apply to the
// value: one line, a trailing " #" starts a comment, and a wrapping pair of
// double quotes is stripped.
//
// AND IT IS STILL A LINT, NOT A BOUNDARY — same class as the allowlist. An
// instance pattern is friction that turns one mis-routed bead into a
// refusal at commit time. What keeps a data owner's content out of a public
// repo is the routing rule plus repo visibility; a confidential name nobody
// thought to add is exactly the case a pattern list cannot see.
const OpsPatternsConfigKey = "beads_visibility_patterns"

// opsPatternConfigWhy is the reason line an instance pattern's refusal
// carries. It says where the pattern came from and nothing about what it
// is for — the harness does not know, and guessing in a refusal is worse
// than saying so.
const opsPatternConfigWhy = "an instance-defined class (config " + OpsPatternsConfigKey + ":)"

// OpsInstanceRule is what an instance-pattern refusal names (ADR 0048 D2),
// and it is a SEPARATE rule from OpsProseRule because the scope is: the
// shipped list is markdown-only because its own source and tests are
// byte-identical to a hit, and a wall carrying an allowlist of its own files
// is a wall with a hole list. That argument is about the shipped list. A
// config pattern is never in source — it lives in the operator's config and
// the rendered hook, both untracked — so it shares check 3's property, no
// legitimate public use anywhere, and gets check 3's scope.
const OpsInstanceRule = `ADR 0048 D2: an instance-defined visibility class (config ` + OpsPatternsConfigKey + `:)
is ONE deployment's own confidential vocabulary, and it has no legitimate
public use anywhere in this repo. Unlike the shipped ops-pattern list, which
is markdown-only because its own source is byte-identical to a hit, an
instance pattern is scanned over the ADDED lines of every staged text file,
code included, and over the ADDED staged paths.`

// OpsInstanceWayThrough is the remedy an instance-pattern refusal names. It
// covers both arms for the reason IdentityWayThrough does — the arms differ
// in what they matched, not in what the writer should do about it — and it
// names the operator's config as the one place the pattern itself can
// change, because a persona cannot edit it and should not try.
const OpsInstanceWayThrough = `the way through: the class named above is this instance's vocabulary, not
this repo's — generalize whatever carries it, or write the content in the
instance tree and have the public artifact restate the mechanism and cite it
(ADR 0024 D3, restate-and-cite). For a PATH: name the file without it. If the
pattern itself is wrong, only the operator can change it (config ` + OpsPatternsConfigKey + `:)
and re-run posse gates install-hooks.`

// opsClassRE is what a class name may be. It is rendered into a shell word
// and into a hook comment, and it is what a human reads in the refusal, so
// it stays boring on purpose.
var opsClassRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// opsEREBanned are the escapes Go and GNU grep share and POSIX ERE does
// not — the list TestOpsPatternsArePortable has always held the shipped
// patterns to, now the one place both readers of it look.
var opsEREBanned = []string{`\t`, `\s`, `\d`, `\b`, `\w`}

// validateOpsERE holds one ERE to the intersection dialect: no single
// quote (it is rendered into a single-quoted sh word), none of the GNU/Go
// escapes POSIX brackets replace, and it has to compile.
//
// The errors NEVER quote the expression. An instance's pattern IS the
// confidential vocabulary — that is the whole point of the key — and this
// message is printed to a terminal and stamped into a hook file. Go's own
// regexp error carries the expression, so it is reduced to its Code.
func validateOpsERE(ere string) error {
	if strings.TrimSpace(ere) == "" {
		return fmt.Errorf("empty pattern")
	}
	if strings.Contains(ere, "'") {
		return fmt.Errorf("a single quote cannot be escaped inside a single-quoted sh word")
	}
	for _, bad := range opsEREBanned {
		if strings.Contains(ere, bad) {
			return fmt.Errorf("%s is a GNU/Go escape POSIX ERE does not share — use a [[:class:]]", bad)
		}
	}
	if _, err := regexp.Compile(ere); err != nil {
		var serr *syntax.Error
		if errors.As(err, &serr) {
			return fmt.Errorf("not a usable regexp: %s", serr.Code)
		}
		return fmt.Errorf("not a usable regexp")
	}
	return nil
}

// NewOpsPattern builds one instance-defined pattern, or says why it cannot
// — by class name only, never by echoing the value.
func NewOpsPattern(class, ere string) (OpsPattern, error) {
	class = strings.TrimSpace(class)
	if !opsClassRE.MatchString(class) {
		return OpsPattern{}, fmt.Errorf("a class name is [A-Za-z0-9][A-Za-z0-9_.-]* — it is rendered into the hook and read in the refusal")
	}
	if err := validateOpsERE(ere); err != nil {
		return OpsPattern{}, err
	}
	return OpsPattern{Class: class, ERE: ere, Why: opsPatternConfigWhy, re: regexp.MustCompile(ere)}, nil
}

// OpsPatternSet is one instance's additions to the shipped list, plus the
// config entries that were refused. The rejects are CARRIED, not dropped:
// a pattern the operator believes in and that is not there is worse than
// no pattern, so every reader of the set can say so — the hook file
// records them in a comment, `posse gates install-hooks` prints them.
//
// The zero value is the shipped list alone, which is what makes it safe to
// pass from a caller that has no config to read.
type OpsPatternSet struct {
	Extra    []OpsPattern
	Rejected []string // "class: reason", in config order
}

// All is the effective list, shipped first: the order the hook renders its
// checks in and the order a refusal names classes in.
func (s OpsPatternSet) All() []OpsPattern {
	out := make([]OpsPattern, 0, len(OpsPatterns)+len(s.Extra))
	out = append(out, OpsPatterns...)
	return append(out, s.Extra...)
}

// OpsPatternSet reads config beads_visibility_patterns:. A class that is
// already shipped is refused rather than shadowing or duplicating it — the
// refusal names the class, so it has to be one thing.
func (a *App) OpsPatternSet() OpsPatternSet {
	var set OpsPatternSet
	seen := map[string]bool{}
	for _, p := range OpsPatterns {
		seen[p.Class] = true
	}
	for _, kv := range YamlMapPairs(a.ConfigPath, OpsPatternsConfigKey) {
		p, err := NewOpsPattern(kv[0], kv[1])
		if err == nil && seen[p.Class] {
			err = fmt.Errorf("that class is already taken — the refusal names the class, so it has to be one")
		}
		if err != nil {
			set.Rejected = append(set.Rejected, opsClassLabel(kv[0])+": "+err.Error())
			continue
		}
		seen[p.Class] = true
		set.Extra = append(set.Extra, p)
	}
	return set
}

// opsClassLabel is how a REFUSED entry's class is displayed: a bad class
// name is still the thing the operator has to find in their config, so it
// is shown — reduced to printable ASCII and short, because it lands in a
// generated shell comment and it is not trusted to be a name. It is never
// empty: YamlMapPairs splits at a colon it found past index 0, so the key
// it hands back always has a non-space first character.
func opsClassLabel(class string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(class) {
		if r < 0x20 || r > 0x7e {
			r = '?'
		}
		b.WriteRune(r)
		if b.Len() >= 40 {
			break
		}
	}
	return b.String()
}

// ─── ADR 0024 D2 check 3: identity literals ──────────────────────────────

// IdentityLiteral is one identity value the hook installer derived from
// this box — a value with no legitimate public use anywhere (ADR 0024 D2
// check 3). It lives only in the rendered hook file: never a shipped
// constant, never committed anywhere, derived fresh at every render.
type IdentityLiteral struct {
	Class string
	Value string
}

// IdentityRule is what a check-3 refusal names.
const IdentityRule = `ADR 0024 D2 check 3: the operator's identity has no legitimate public use in
this repo — the box's own username, git email, and the instance repo's path
are derived at hook-render time (never shipped, never committed) and refused
wherever they appear in the ADDED lines of any staged text file, code
included, and in the ADDED staged paths (a filename is exactly where an
operator-shaped artifact puts the operator; move detection off, so a move's
destination counts as new).`

// IdentityWayThrough is the remedy a check-3 refusal names — both ways
// through, since ranger-base-dmsbu gave check 3 a second arm: content is
// generalized or restated, and a PATH is renamed or written in the instance
// tree. Two constants, not three: the arms differ in what they matched, not
// in what the writer should do about it.
const IdentityWayThrough = `the way through: nothing that names this operator, this box, or this
instance's path belongs in a public repo's history — generalize whatever
carries it (a comment, a fixture, a hardcoded path), or move the content to
the instance tree (ADR 0024 D3, restate-and-cite). For a PATH: name the file
without it, or write it in the instance tree — a rename is a fresh added
entry and is scanned again, so the new name has to be clean too.`

// identityLiteralMaxLen guards against a pathologically long identity value
// rendering into the hook a pattern so long the hook file stops being
// something a human can read. No source DeriveIdentityLiterals reads from
// should ever approach it; a literal past it is skipped, not refused —
// surprising box state should cost one uncovered source, not a broken
// install.
const identityLiteralMaxLen = 4096

// DeriveIdentityLiterals derives ADR 0024 D2 check 3's identity literals
// from the box, for the repo at dir (hookRepo's answer — the shared repo a
// worktree's hook actually belongs to). Each source is independent and
// contributes nothing silently when it has nothing to say — there is no
// identity to protect on a box with no git email configured, and the
// renderer says so only for what it DID derive:
//
//   - username: os/user's Current().Username — not $USER, which a caller
//     can set to anything.
//   - email: `git config user.email`, read with no --local/--global flag,
//     which is exactly repo-then-global (git's own config file priority).
//   - the instance repo path, when dir's .beads/redirect names one: the
//     dirname of the redirect's target (the same resolution beadsHome and
//     seedBeadsRedirect use — relative resolves against the repo root, not
//     against .beads/ itself), rendered BOTH ~-relative (AbbrevHome) and
//     absolute. Skipped whole when there is no .beads, or no redirect in
//     it.
//
// Every literal is checked for a single quote before it is returned — it is
// rendered into a single-quoted sh word, and a literal that broke that
// quoting is the renderer's own init-panic class of bug, refused rather
// than rendered wrong.
func DeriveIdentityLiterals(dir string) ([]IdentityLiteral, error) {
	var out []IdentityLiteral
	seen := map[string]bool{} // AbbrevHome(p) == p whenever p is not under $HOME (or HOME is unset): one literal, not two.
	add := func(class, value string) error {
		if value == "" || len(value) > identityLiteralMaxLen || seen[value] {
			return nil
		}
		if strings.Contains(value, "'") {
			return fmt.Errorf("identity literal (%s) contains a single quote — cannot render into a single-quoted sh word", class)
		}
		seen[value] = true
		out = append(out, IdentityLiteral{Class: class, Value: value})
		return nil
	}

	if u, err := user.Current(); err == nil {
		if err := add("username", u.Username); err != nil {
			return nil, err
		}
	}

	if email, err := git(dir, "config", "user.email"); err == nil {
		if err := add("email", email); err != nil {
			return nil, err
		}
	}

	if target := identityRedirectTarget(dir); target != "" {
		repo := filepath.Dir(target)
		if err := add("instance-path", AbbrevHome(repo)); err != nil {
			return nil, err
		}
		if err := add("instance-path-abs", repo); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// identityRedirectTarget reads dir's .beads/redirect and returns its
// target, resolved exactly as beadsHome (beadloss.go) and seedBeadsRedirect
// (worktree.go) resolve it — relative against the repo root, not against
// .beads/ itself. "" when dir has no .beads, or .beads has no redirect:
// check 3 has nothing to derive from a repo with no instance to point at.
func identityRedirectTarget(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, beadsDirName, beadsRedirect))
	if err != nil {
		return ""
	}
	target := strings.TrimSpace(firstLine(string(b)))
	if target == "" {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	return filepath.Clean(target)
}

// identityLiteralERE renders a fixed-string identity literal as an ERE
// matching only that literal — every ERE metacharacter escaped, nothing
// else touched — so the shipped patterns' own two-reader dialect (POSIX
// ERE ∩ Go regexp, see the OpsPatterns comment above) covers it too, and
// check 3 reuses the SAME posse_check shell function checks 0 and 2 already
// render rather than a second matcher with its own escaping bugs to find.
//
// This is also what keeps a bead-id prefix from tripping check 3
// (ADR 0024 D2): the rendered ERE is the WHOLE literal, slashes included,
// so it only matches where the full path appears — a bare hyphenated bead
// id with none of the path around it is a proper substring miss, not a
// match (TestIdentityLiteralDoesNotTripOnABareBeadID).
func identityLiteralERE(s string) string {
	const meta = `.^$*+?()[]{}|\`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(meta, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
