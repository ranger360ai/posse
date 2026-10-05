package posse

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
// emit `evaluated_rules`, or a detection that has none, and no D3 report
// either. The caller's message must stand on its own without this. The two
// halves are independent — a capture can name a screen for a detection that
// evaluated no rules at all, and that is the richest thing this block ever
// says about the shape rangerhq-7ia was.
//
// ON A REPORTED READING there is no working to print — herdr evaluated no
// rules, because it read no screen — so it prints the REPORTER's word
// instead, which is the whole of the evidence on that route (ADR 0061 D3.3,
// ranger-base-nm5i6). `why` is the runtime's `detection_why:`, the sentence
// that names the authority; a caller that does not have the profile in hand
// passes "" and the block stands without it.
//
// The why is an ARGUMENT rather than a field on the detection because the
// reading is taken from herdr, which has never heard of a posse runtime: a
// detection that carried a declaration would be a reading claiming to know
// something only the profile can say. Every caller therefore decides whether
// it has one, which is what ADR 0061 D3.3's "where the runtime is in hand"
// means.
//
// `looksLike` IS THE SAME SHAPE OF ARGUMENT, one reading later (ADR 0066 D3,
// ranger-base-6uokf). It is the set of known screens the heading reader
// named in the bytes the D3 RECORD carries — herdr's previews plus the
// whole-pane capture (knownscreen.go, ReadingEvidence.LooksLike) — and it
// cannot be computed from a detection, because the capture is not on one.
// Over the previews alone the same reader names 5 of 11 residue cases
// instead of 11, which is why the caller that built the record passes its
// answer in rather than this function taking a second, worse reading of its
// own. An EMPTY set means no row at all, and every caller that has no record
// to hand passes nil.
//
// IT IS APPENDED, NEVER SUBSTITUTED. herdr's working stays the whole of the
// block above it: this reader has no rule, no codepoint and no replay, so a
// line that displaced the verdict would be trading the diagnostic that can
// be chased for the one that cannot (ADR 0066 D2 — it reports BESIDE the
// verdict). It is also the last row on purpose; a reader who stops at the
// regions has lost nothing.
func (d AgentDetection) WhatHerdrSaw(why string, looksLike []string) string {
	return d.whatHerdrSawBody(why) + looksLikeRow(looksLike)
}

// looksLikeRow is the appended row, indented as a sibling of "What it was
// reading:" rather than of the per-region rows under it — it is a reading of
// the whole screen and not of one region.
func looksLikeRow(looksLike []string) string {
	line := LooksLikeLine(looksLike)
	if line == "" {
		return ""
	}
	return "\n    " + line
}

// whatHerdrSawBody is WhatHerdrSaw without the D3 report: the block as it
// stood before ranger-base-6uokf, which is still the whole of what a reader
// can chase to a rule.
func (d AgentDetection) whatHerdrSawBody(why string) string {
	if d.Reported != "" {
		return d.whatTheReporterSaid(why)
	}
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

// whatTheReporterSaid is WhatHerdrSaw's reported arm: herdr's working replaced
// by the only evidence this route has, in the same indented shape, so a
// failure line reads the same whichever route produced it.
//
// It names the label and the state on their own line because those two are
// what a reader has to act on: the label says WHO to go ask, and the state
// says what that authority last believed. The stale clause is named
// separately, not folded into the state, because a stale `idle` and a live
// `unknown` are opposite problems with opposite remedies and the old line
// described them identically — the same failure WhatHerdrSaw was written for
// one reading over (ranger-base-3j8).
func (d AgentDetection) whatTheReporterSaid(why string) string {
	var b strings.Builder
	b.WriteString("\n    No screen evidence on this route: herdr has no detection manifest for this pane, so `agent explain` refuses it outright and every state here is an outside authority's word (ADR 0061 D3).")
	fmt.Fprintf(&b, "\n      label %q, reported state %s", d.Reported, orUnknown(d.State))
	if d.ShellForeground {
		b.WriteString("\n      and every foreground process in the pane is the pane's own SHELL — so that label is STALE whatever it says, and nothing is holding the keyboard (ADR 0061 D3.2)")
	}
	if why != "" {
		fmt.Fprintf(&b, "\n      detection_why: %s", why)
	}
	b.WriteString("\n      first remedy: `herdr plugin list` — is that authority installed, enabled and watching? Not a larger startup_wait:")
	return b.String()
}
