package rhq

// What herdr was looking at when it recognized nothing (ranger-base-3j8).
//
// A dispatch that cannot become promptable prints herdr's verdict —
// `only "idle" (default_known_agent_idle_fallback)`. That sentence is
// honest and it is useless: it says a rule did not match without saying
// which rules were tried or what was on the screen when they were. Three
// separate diagnoses on this bead — a consent banner, a version splash, and
// a pane whose OSC chrome had simply not been emitted yet — all produced
// that identical line, and each one cost a hand-launch and a `posse peek`
// to tell apart. Two of them needed opposite fixes.
//
// herdr already has the answer. `agent explain --json` carries an
// `evaluated_rules` array: every rule it tried, the screen region that rule
// reads, and how many bytes were in that region with a preview of them.
// Reading it turns the failure line into the thing the coordinator had to
// assemble by hand:
//
//	  osc_title                    0 bytes  ""
//	  bottom_non_empty_lines(2)  124 bytes  "╰─ Grok 4.6 (high) ─╯ …"
//
// Empty regions say the CLI has not spoken yet; a region full of splash
// text says it is up and parked on a screen posse does not know. This is
// diagnosis only — nothing here decides anything, and no key is pressed on
// the strength of it. ADR 0013 §2 keeps interstitials on the argv sidestep
// and the operator's own config; the point of this block is that when the
// sidestep does not save a launch, the next person does not start from
// zero.

import (
	"fmt"
	"strings"
	"unicode"
)

// whatHerdrSawPreview is how much of a region's contents the failure line
// carries. Long enough to recognize a splash by eye, short enough that a
// pass log stays a pass log.
const whatHerdrSawPreview = 72

// whatHerdrSawRules is how many rule ids are named per region before the
// rest are counted. The ids are there to point at the manifest, not to
// reproduce it.
const whatHerdrSawRules = 3

// WhatHerdrSaw renders herdr's working as an indented block to append to a
// promptability failure: one row per screen REGION, in the order herdr
// evaluated them, with the bytes and preview it read there and the rules
// that read it.
//
// Regions rather than rules because rules outnumber regions three to one
// and share their evidence — twelve rows of which eleven repeat is how a
// diagnostic gets skimmed past.
//
// It returns "" when there is nothing to add: an older herdr that does not
// emit `evaluated_rules`, or a detection that has none. The caller's
// message must stand on its own without this.
func (d AgentDetection) WhatHerdrSaw() string {
	if len(d.EvaluatedRules) == 0 {
		return ""
	}
	type region struct {
		name    string
		bytes   int
		preview string
		rules   []string
	}
	var order []*region
	at := map[string]*region{}
	matched := 0
	for _, r := range d.EvaluatedRules {
		if r.Matched {
			matched++
		}
		g, ok := at[r.Region]
		if !ok {
			g = &region{name: r.Region, bytes: r.Evidence.RegionBytes, preview: r.Evidence.RegionPreview}
			at[r.Region] = g
			order = append(order, g)
		}
		g.rules = append(g.rules, r.ID)
	}
	verdict := fmt.Sprintf("%d matched", matched)
	if matched == 0 {
		verdict = "matched none"
	}
	width := 0
	for _, g := range order {
		if len(g.name) > width {
			width = len(g.name)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n    herdr evaluated %d rules there and %s. What it was reading:", len(d.EvaluatedRules), verdict)
	for _, g := range order {
		fmt.Fprintf(&b, "\n      %-*s  %5d bytes  %s  — %s",
			width, g.name, g.bytes, quoteOneLine(g.preview, whatHerdrSawPreview), namedFew(g.rules, whatHerdrSawRules))
	}
	return b.String()
}

// quoteOneLine flattens a region preview onto one line and quotes it, so an
// empty region reads as `""` rather than as a gap in the output. herdr's
// previews carry real newlines and its own trailing ellipsis; both survive
// the flattening as themselves.
//
// Rules are collapsed first. A TUI screen preview begins with box-drawing
// borders — measured on the wide grok splash, 70 of the first 72 characters
// were one repeated `─`, which is a preview of nothing. Collapsing a run of
// four or more identical non-alphanumeric runes to two spends the budget on
// the text between the borders instead, which is the part that names the
// screen.
func quoteOneLine(s string, max int) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	s = collapseRules(s)
	if r := []rune(s); len(r) > max {
		s = strings.TrimSpace(string(r[:max])) + "…"
	}
	return fmt.Sprintf("%q", s)
}

// collapseRules shortens a run of four or more of the same non-alphanumeric
// rune to two of it. Border art only: a run of letters or digits is content
// and is left exactly as herdr read it.
func collapseRules(s string) string {
	var b strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); {
		j := i
		for j < len(r) && r[j] == r[i] {
			j++
		}
		n := j - i
		if n >= 4 && !isAlnum(r[i]) {
			n = 2
		}
		b.WriteString(strings.Repeat(string(r[i]), n))
		i = j
	}
	return b.String()
}

func isAlnum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// namedFew lists the first n of a set and counts the rest. Naming every
// rule id in a region is the manifest, not a diagnosis.
func namedFew(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(names[:n], ", "), len(names)-n)
}
