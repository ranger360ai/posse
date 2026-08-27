package rhq

// posse agent check — lint a PID against the ADR 0001 contract. Deliberately
// tiny: it reports what a reviewer would otherwise eyeball, exits non-zero
// on findings so an instance repo can run it in CI, and knows nothing the
// ADR does not say.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// identityLineRe is the PID identity line's *shape*, not any crew's brand
// (ADR 0012 D2: persona names become roles): an optional personal name, the
// role the persona plays, and the crew it plays it in — "You are Ada, the
// QA engineer of the Vantage crew." or "You are the QA engineer of the
// crew." The name is optional because a reference PID names a role and
// nothing else; what is not optional is that the line says what this
// persona *is* and stops, because it is the first thing the model reads.
var identityLineRe = regexp.MustCompile(`^You are (?:[^,]+, )?the .+ of .+\.$`)

// identityLine is the body's first non-empty line, trimmed — what the
// identity rule is about.
func identityLine(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// MetricCatalog is derived, not declared (ADR 0001 amendment 2026-08-18):
// the union of every loaded PID's `metrics:` plus config `metric_ids:`,
// mapped to the personas that declare each id (empty = config only). A
// persona naming how it is judged is the source of truth, so nothing here
// rejects an id — the linter checks the vocabulary stays *one* spelling and
// `posse scorecard --catalog` says which ids it can actually compute.
func (a *App) MetricCatalog() map[string][]string {
	cat := map[string][]string{}
	for _, n := range a.ListAgents() {
		ag, err := a.LoadAgent(n)
		if err != nil {
			continue
		}
		for _, id := range ag.Metrics {
			cat[id] = append(cat[id], n)
		}
	}
	for _, id := range YamlList(a.ConfigPath, "metric_ids") {
		if _, ok := cat[id]; !ok {
			cat[id] = nil
		}
	}
	return cat
}

// MetricCatalogIDs is the catalog's ids, sorted — a stable order for the
// linter's findings and the catalog listing.
func MetricCatalogIDs(cat map[string][]string) []string {
	ids := make([]string, 0, len(cat))
	for id := range cat {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// MetricDeclaredBy renders who declares an id, for a finding or a listing.
func MetricDeclaredBy(cat map[string][]string, id string) string {
	if by := cat[id]; len(by) > 0 {
		return strings.Join(by, ", ")
	}
	return "config metric_ids:"
}

// metricKey normalizes a metric id for the near-duplicate check: lowercase,
// split on anything that is not a letter or digit, then stem each word
// loosely (-ing, -s, -e, repeatedly) so `findings-survive-triage` and
// `findings-surviving-triage` collapse onto one key. A heuristic on
// purpose: it only ever produces a "pick one spelling" finding, never a
// rejection, so a false positive costs a sentence and a miss costs nothing
// the reviewer wasn't already doing by eye.
func metricKey(id string) string {
	words := strings.FieldsFunc(strings.ToLower(id), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for i, w := range words {
	stem:
		for {
			switch {
			case len(w) > 5 && strings.HasSuffix(w, "ing"):
				w = w[:len(w)-3]
			case len(w) > 3 && (strings.HasSuffix(w, "s") || strings.HasSuffix(w, "e")):
				w = w[:len(w)-1]
			default:
				break stem
			}
		}
		words[i] = w
	}
	return strings.Join(words, "-")
}

// balancedParens reports whether a permission rule's parentheses close.
// An unbalanced one is the signature of an inline list split on a comma
// *inside* a rule (`allow: [Bash(git commit -m a,b)]` parses as
// `Bash(git commit -m a` + `b)`), which is the only thing wrong with
// inline form — the crew's `[Bash(posse:*), Bash(git push:*)]` is fine.
func balancedParens(rule string) bool {
	depth := 0
	for _, r := range rule {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// CheckAgent returns one line per contract violation (empty = clean) and
// one per advisory warning (optional sections absent — never a failure).
func (a *App) CheckAgent(name string) (findings, warnings []string, err error) {
	ag, err := a.LoadAgent(name)
	if err != nil {
		return nil, nil, err
	}
	raw, _ := os.ReadFile(ag.Path)
	front, body := agentFrontmatter(string(raw))
	var out []string
	add := func(f string, args ...any) { out = append(out, fmt.Sprintf(f, args...)) }
	warn := func(f string, args ...any) { warnings = append(warnings, fmt.Sprintf(f, args...)) }

	// Frontmatter.
	if front == nil {
		add("no frontmatter block")
	}
	for _, key := range []string{"allow", "deny"} {
		rules := ag.Allow
		if key == "deny" {
			rules = ag.Deny
		}
		inline := strings.HasPrefix(yamlGetLines(front, key), "[")
		for _, rule := range rules {
			if balancedParens(rule) {
				continue
			}
			if inline {
				add("%s: inline list split %q on a comma inside a permission rule — use block form", key, rule)
			} else {
				add("%s: %q has unbalanced parentheses", key, rule)
			}
		}
	}
	for _, ph := range []string{"{allow}", "{deny}"} {
		if j := strings.Index(ag.Command, ph); j >= 0 {
			rest := strings.NewReplacer("{allow}", "", "{deny}", "").Replace(ag.Command[j:])
			if strings.TrimSpace(rest) != "" {
				add("command: %s must be last (claude's tool flags are variadic and would swallow what follows)", ph)
				break
			}
		}
	}
	if ag.Runtime != "" {
		if _, err := a.LoadRuntime(ag.Runtime); err != nil {
			add("runtime: %v", err)
		}
	}
	if ag.Tier != "" && !ValidTier(ag.Tier) {
		add("tier: %q is not strong | standard | fast", ag.Tier)
	}
	if ag.Cage != "" && !ValidCage(ag.Cage) {
		add("cage: %q is not shims | seatbelt | container", ag.Cage)
	}
	// `sockets:` (ADR 0002 §5): a name nothing mounts reads in the PID as a
	// capability the session was given, so it is a finding here and a
	// refusal at launch — the {model} rule, one key over.
	if err := CheckSockets(ag); err != nil {
		add("%v", err)
	} else if len(ag.Sockets) > 0 && ResolveCage("", ag) != CageContainer {
		add("sockets: %s is a container-tier key and this PID launches at %s — nothing is mounted (add cage: container or drop it)", strings.Join(ag.Sockets, ", "), ResolveCage("", ag))
	}
	// route_order: a spelling that is not an integer takes the default and
	// the PID still loads (a lane must not go silent over an ordering
	// hint) — so the mistake has to surface here, or `route_order: high`
	// reads as an ordering nobody has.
	if raw := yamlGetLines(front, "route_order"); raw != "" {
		if _, ok := parseRouteOrder(raw); !ok {
			add("route_order: %q is not an integer — this PID routes at the default %d", raw, RouteOrderDefault)
		}
	}
	if ag.TierFloor != "" && !ValidTier(ag.TierFloor) {
		add("tier_floor: %q is not strong | standard | fast", ag.TierFloor)
	} else if ValidTier(ag.Tier) && BelowFloor(ag, ag.Tier) {
		add("tier: %s is below this PID's own tier_floor: %s — every launch that takes the PID default would refuse (ADR 0003 §3)", ag.Tier, ag.TierFloor)
	}
	// Skills (ADR 0007 §5): a name that resolves to nothing binds nothing,
	// and a PID whose own command: forgot {skills} would launch without the
	// skills it declares — the {model} rule again: never leave a token
	// unrendered, never silently skip one either.
	if _, unknown := a.ResolveSkills(ag.Skills); len(unknown) > 0 {
		for _, s := range unknown {
			add("skills: unknown skill %q — no %s", s, AbbrevHome(filepath.Join(a.SkillPath(s), "SKILL.md")))
		}
	}
	if len(ag.Skills) > 0 && ag.Command != "" && !strings.Contains(ag.Command, "{skills}") {
		add("command: has no {skills} while skills: names %s — this PID's own runtime would launch without them (add {skills} or drop command: for the built-in template)", strings.Join(ag.Skills, ", "))
	}
	if ag.Command != "" && !strings.Contains(ag.Command, "{model}") {
		add("command: has no {model} — the tier will not select a model on this PID's own runtime (add {model} or drop command: for the built-in template)")
	}
	// Metrics: the catalog is the union of what the PIDs declare, so an id
	// is never "unknown" (ADR 0001 amendment). What still costs the crew is
	// two spellings of one metric, which splits the scorecard silently.
	cat := a.MetricCatalog()
	catIDs := MetricCatalogIDs(cat)
	for _, id := range ag.Metrics {
		for _, other := range catIDs {
			if other != id && metricKey(other) == metricKey(id) {
				add("metrics: %q is near %q in %s — one spelling", id, other, MetricDeclaredBy(cat, other))
			}
		}
	}

	// Body.
	if line := identityLine(body); !identityLineRe.MatchString(line) {
		add("body must open with the identity line (\"You are [<Name>, ]the <role> of the <crew>.\"), got %q", line)
	}
	pos := -1
	for _, h := range PIDHeadings {
		i := strings.Index("\n"+body, "\n"+h+"\n")
		if i < 0 {
			if OptionalPIDHeadings[h] {
				warn("optional section %s absent — this persona adds nothing to its work prompts (ADR 0005 §3)", h)
			} else {
				add("missing section %s", h)
			}
			continue
		}
		if i < pos {
			add("section %s out of contract order", h)
		}
		pos = i
	}
	if strings.Contains(body, "\n## Intents\n") && !strings.Contains(body, "| intent | mode | done when |") {
		add("## Intents lacks the table header `| intent | mode | done when |`")
	}
	if strings.Contains(body, "\n## Guardrails\n") && !strings.Contains(body, HardRiskLines) {
		add("## Guardrails does not carry the four hard risk lines verbatim")
	}
	return out, warnings, nil
}
