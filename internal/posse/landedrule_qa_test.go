//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-wplhv — the two landedRules, read as a PAIR.
//
// ADR 0097 D5 says the wall check reuses ADR 0088 D5's ratchet and D6.3's
// watchdog, "parameterised rather than copied", with one requirement about
// the parameters themselves: "Dedupe markers must differ from the judge's."
// This file is that requirement, plus the two premises a struct of literals
// has that a function does not.
//
// WHY THE MARKERS ARE THE SHARP EDGE. The dedupe is "the newest OPEN bead in
// this lane whose description carries this LINE" (openMarkedBead,
// landedDescriptionCarries). Both ratchets file `-l code` and both watchdogs
// file `-l architecture`, so a marker either rule could match would make a
// red wall comment on a red main's bead: one stretch's bead carrying
// another's verdicts, and neither stretch filing its own. That is the
// ranger-base-4v4r6 shape with a new subject, and it is invisible in a green
// test run because both rules would be doing exactly what they were told.
//
// WHY THE FORMAT VERBS NEED A PIN. Three of the rule's fields are format
// strings (RedAdvice, SilentAdvice, CeilingAdvice), rendered with
// `fmt.Fprintf(&b, r.Field, …)`. `go vet` checks a format string it can see
// at the call site and NOT one it reads out of a variable, so a field with
// one verb too many reaches a bead as `%!s(MISSING)` — in the one paragraph
// of that bead that tells the reader what to do about the thing it is
// reporting. Rendering both rules' four descriptions and refusing any `%!` is
// the whole check, and it is the reason those fields carry their verbs in a
// comment.

import (
	"strings"
	"testing"
	"time"
)

func TestQATheTwoLandedRulesShareNoRecordOrMarker(t *testing.T) {
	t.Parallel()
	rules := landedRules()
	if len(rules) != 2 {
		t.Fatalf("landedRules has %d entries — this pin reads them as a pair, and a third rule needs its own record, its own two markers and a row here", len(rules))
	}
	const repo, base = "/src/posse", "main"
	for i, a := range rules {
		for j, b := range rules {
			if i >= j {
				continue
			}
			for _, c := range []struct {
				what string
				x, y string
				why  string
			}{
				{"the verdict record", a.Key, b.Key, "one `git config` line cannot hold two checks' verdicts: the ratchet reads the record as the transition's left-hand column, so a shared key makes every landing a transition of whichever check ran last"},
				{"the quiet counter", a.QuietKey, b.QuietKey, "the watchdog's dedupe IS the count (landedhealth.go), so a shared counter files one check's silence against the other's stretch"},
				{"the ratchet's marker prefix", a.FileMarkerPrefix, b.FileMarkerPrefix, "ADR 0097 D5: dedupe markers must differ from the judge's"},
				{"the watchdog's marker prefix", a.HealthMarkerPrefix, b.HealthMarkerPrefix, "ADR 0097 D5, in the other lane"},
				{"the ratchet's rendered marker", a.fileMarker(repo, base), b.fileMarker(repo, base), "the marker is matched as a WHOLE LINE, so this is the string the dedupe actually compares"},
				{"the watchdog's rendered marker", a.healthMarker(repo), b.healthMarker(repo), "likewise, per repo rather than per base"},
				{"the extracted copy's name", a.Extract, b.Extract, "one run extracts both copies into one run dir, so a shared name has the venue exec the judge"},
			} {
				if c.x == c.y {
					t.Errorf("%s and %s share %s (%q) — %s", a.Name, b.Name, c.what, c.x, c.why)
				}
			}
			// And the two rendered markers may not nest either, which a
			// prefix comparison would miss: landedDescriptionCarries trims a
			// line and compares it whole, so equality is the only collision
			// — but a prefix is how one becomes the other after an edit, and
			// saying so here is cheaper than finding out from a bead.
			for _, pair := range [][2]string{
				{a.fileMarker(repo, base), b.fileMarker(repo, base)},
				{a.healthMarker(repo), b.healthMarker(repo)},
			} {
				if strings.HasPrefix(pair[0], pair[1]) || strings.HasPrefix(pair[1], pair[0]) {
					t.Errorf("%q and %q are prefixes of one another — one edit away from a shared dedupe", pair[0], pair[1])
				}
			}
		}
	}
}

// Every field of every rule is FILLED, which is the premise a struct of
// literals has and a function does not: an empty field renders an empty
// clause into a bead and nothing says so. `RedSubject` is the one field whose
// empty value is a decision (it means "the branch that landed", which is the
// judge's subject), so it is read through `subject` instead.
func TestQAEveryLandedRuleFieldIsFilled(t *testing.T) {
	t.Parallel()
	for _, r := range landedRules() {
		for what, got := range map[string]string{
			"Name":               r.Name,
			"Agent":              r.Agent,
			"Script":             r.Script,
			"Extract":            r.Extract,
			"Key":                r.Key,
			"QuietKey":           r.QuietKey,
			"FileMarkerPrefix":   r.FileMarkerPrefix,
			"HealthMarkerPrefix": r.HealthMarkerPrefix,
			"RedWhen":            r.RedWhen,
			"RedAdvice":          r.RedAdvice,
			"CeilingTitle":       r.CeilingTitle,
			"SilentAdvice":       r.SilentAdvice,
			"CeilingAdvice":      r.CeilingAdvice,
			"SilentCost":         r.SilentCost,
			"RunADR":             r.RunADR,
			"FileADR":            r.FileADR,
			"HealthADR":          r.HealthADR,
			"PriceADR":           r.PriceADR,
			"BoundsADR":          r.BoundsADR,
			"CeilingADR":         r.CeilingADR,
			"subject(main)":      r.subject("main"),
		} {
			if strings.TrimSpace(got) == "" {
				t.Errorf("%s leaves %s empty — an empty field is an empty clause in a bead a persona reads", r.Name, what)
			}
		}
	}
}

// And the three format-string fields render. Four descriptions per rule, each
// built the way the production path builds it, and no `%!` anywhere: a verb
// error in one of those fields reaches the bead as `%!s(MISSING)` in the one
// paragraph that says what to do.
func TestQABothLandedRulesRenderTheirBeadsWithNoFormatVerbLeftOver(t *testing.T) {
	t.Parallel()
	const repo, base, tip, tree, green = "/src/posse", "main", "deadbeefdeadbeef", "feedfacefeedface", "0123456789abcdef"
	c := &landedChecker{}
	window := []landedLanding{{SHA: tip, Tree: tree, Subject: "ranger-base-aaaa1: a landing", Bead: "ranger-base-aaaa1", Persona: "reviewer"}}
	for _, r := range landedRules() {
		for what, desc := range map[string]string{
			"the ratchet's green→red bead": c.redDescription(r, repo, base, tip, tree, green, "the tail", "", "", window),
			"the ratchet's red→red bead":   c.redDescription(r, repo, base, tip, tree, "", "the tail", "the bead for this red stretch is not open any more", "a note", nil),
			"the watchdog's silent bead":   c.silentDescription(r, repo, tree, 2, "run one said this", "run two said this", false),
			"the watchdog's ceiling bead":  c.ceilingDescription(r, repo, tree, 321*time.Second),
		} {
			if i := strings.Index(desc, "%!"); i >= 0 {
				t.Errorf("%s: %s carries a format error at byte %d — the verbs each field takes are named in its comment (landedrule.go):\n%s", r.Name, what, i, desc)
			}
			// The marker is the FIRST line of every one of these, because
			// that is what the next run reads back.
			if !strings.HasPrefix(desc, r.FileMarkerPrefix) && !strings.HasPrefix(desc, r.HealthMarkerPrefix) {
				t.Errorf("%s: %s does not open with a marker line:\n%s", r.Name, what, desc)
			}
			// And each one names its own script, which is the one thing a
			// reader needs before they can reproduce anything.
			if !strings.Contains(desc, r.Script) {
				t.Errorf("%s: %s never names %s:\n%s", r.Name, what, r.Script, desc)
			}
		}
	}
}
