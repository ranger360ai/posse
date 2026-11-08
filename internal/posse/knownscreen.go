package posse

// The known-screen heading reader — ADR 0066 D3 as amended 2026-10-04
// (ranger-base-qk9tr) and ruled 2026-10-05 (ranger-base-gy3io, option B).
//
// WHAT IT IS FOR. When herdr's readiness gate refuses because no rule
// matched, `WhatHerdrSaw` prints herdr's working — every rule it tried and a
// preview of the region each one read (unrecognized.go, ranger-base-3j8).
// That block says which rules failed; it does not say what the screen WAS.
// Three distinct screens on that bead produced one identical failure line —
// a consent banner, a version splash, and a pane whose OSC chrome had not
// been emitted yet — and telling them apart cost a hand-launch and a
// `posse peek` each, two of them needing opposite fixes. This names the
// screen in the failure line and on the D3 record, from the screen's own
// heading, so the next reader starts from a name instead of from zero.
//
// IT IS A REPORT AND NOTHING READS IT (ADR 0066 D2, verbatim: "no guard,
// launch eligibility, hand-back, kill, hire or keystroke reads it"). The
// only two consumers are the failure line's last row and the `looks_like`
// field on the D3 record, which `scripts/readings-census.py` counts. A
// reading that said "this is the update menu" and then pressed something
// would be the whole of what ADR 0057, 0060 D2, 0061 D3 and 0063 D1 refuse.
//
// WHY A SEPARATE TABLE AND NOT THE INTERSTITIAL REGISTRY (the one design
// choice ranger-base-6uokf flagged, decided here and recorded on the bead):
//
//   - the registry IS read by a guard. `DangerUnsilenced` walks
//     GrokInterstitials/CodexInterstitials/… and `DangerRefusal` turns its
//     answer into a launch refusal, on three surfaces that have to agree
//     (ADR 0013 §2). Markers on those rows would put this reader's table
//     inside the one table a refusal is derived from, and D2's "no guard
//     reads it" would hold by care rather than by construction. Here it
//     holds by grep.
//   - the registry's subject is narrower than "known screen". Every row
//     carries `Where`, `Key`, `Silence` and a `Probe` — an
//     operator-silenceable first-run screen — and four of the screens posse
//     owns a capture of (codex's hooks review, trust directory and model
//     picker, and claude's fullscreen upsell) have no silence key, nothing
//     for an operator to have done first, and nothing for a probe to read.
//     Four such rows would also grow four permanently-unknown rows in
//     `posse runtime check`'s grid, which is the sentence bobSignInSilence
//     exists to avoid writing. claude's is the one ranger-base-yk5qb ruled
//     on directly: that class is per ACCOUNT rather than per repo, its key
//     is written by the CLI when the screen is answered, and a release adds
//     notices under keys nobody has seen.
//   - the two sets disagree in both directions and on their keys. The
//     registry carries claude's TRUST modal and outside-read notice and
//     bob's three, which posse owns no capture of and can write no marker
//     for, while this table carries a claude screen the registry has no row
//     for (ranger-base-72oz4) — so the disagreement runs both ways inside
//     one runtime now, not only between runtimes. It is per RUNTIME and
//     keyed by a prose `Screen` sentence. This table is keyed by the herdr
//     rule ids that name the screen today — the axis
//     `scripts/d3-reader-eval.py`'s OPTIONS already uses, the axis a census
//     groups by, and the axis the pin can compare.
//
// So the registry is untouched and the markers live only here. Not both:
// the one second copy is the spike script's OPTIONS, and
// d3knownscreen_qa_test.go compares the two rather than trusting them.
//
// WHY HEADINGS AND NOT RULES. A rule is herdr's and replayable at the
// codepoint; that is the reader that keeps the decision, and this one does
// not compete with it — it only ever speaks where that one said nothing. A
// heading is the one part of these screens that is addressed to a human, so
// it is the part a layout change, a caret codepoint, an extra logo row or a
// reworded footer leaves alone: MEASURED 2026-10-04 (ranger-base-qk9tr,
// docs/notes.d/ranger-base-qk9tr.md) over the fifteen labelled fixtures and
// three perturbation classes taken from the incident record, 11 of 11
// residue cases named with 0 false positives on the idle screens. The one
// class it cannot survive is a reworded HEADING, which has zero incidents
// in the record and is where a model's claimed value would have to live.
//
// SET-VALUED ON PURPOSE. Three of the shipped fixtures show two true screens
// at once — grok's startup splash with the consent banner drawn over it —
// and a `choice` returns one. A reader that had to pick would be wrong on a
// fifth of the corpus by construction, and the failure line would name the
// screen whose fix is not the one needed.
//
// NO MODEL, NO NETWORK, NO CONFIG KEY. It is a lower-case substring scan
// over bytes posse already read, ~70 µs, in process (ADR 0066 D3(c): a model
// on this seam is not recommended, and D4 would make it two operator
// rulings).

import (
	"sort"
	"strings"
)

// KnownScreen is one screen posse owns a capture of, named the way a D3 line
// and a census row name it.
//
// Rules are the herdr rule ids that name the screen TODAY, and they are
// carried for two reasons that are not "so something can match on them":
// they are what makes a row's name checkable against the manifests, and they
// are what lets an evaluation score this reader only on the residue — the
// cases where a known screen is present and herdr's rule did not name it,
// which is the only place this reader has anything to say. An empty Rules is
// a real answer: grok's consent banner is drawn OVER another screen and no
// rule of herdr's is about it.
//
// Markers is the screen's own heading phrase(s), lower-cased and
// whitespace-collapsed. The outer slice is alternatives — any one of them
// present names the screen — and each inner slice is a CONJUNCTION, every
// phrase of which must be present. The conjunction is not decoration:
// grok's startup splash is keyed on "new worktree" AND "resume session",
// because either phrase alone appears in text that is not that screen.
type KnownScreen struct {
	Name    string
	Rules   []string
	Markers [][]string
}

// KnownScreens is the table, in the order the spike's OPTIONS lists it.
//
// EVERY MARKER IS THE SCREEN'S OWN WORDS, read off the fixture under
// etc/herdr/agent-detection/testdata/ and nothing else. A marker nobody can
// point at a capture for is a guess about a screen, and this reader's whole
// claim is that it reads what is there.
//
// THAT BAR IS WHY claude HAS ONE ROW AND NOT TWO (ranger-base-72oz4). The
// fullscreen upsell's sibling — "Teach auto mode about your environment?"
// over "Auto mode works better when it knows your environment. Takes about a
// minute.", the same Yes / Not now / Don't show again shape, and posse runs
// claude seats in auto mode — could not be drawn on this box. MEASURED
// 2026-11-08: the whole LOCAL half of its gate was satisfied in a scratch
// config dir (auto mode on, `numStartups` 40 against a floor of 5,
// `autoModeEnvSetup.denials` 40 against a floor of 5, not dismissed, no
// `dismissedAt`) and the screen did not draw. Read off claude 2.1.319's own
// bundle, what is left is one clause — `Xu()?.envOnboarding === true` inside
// `Zge()`, beside the skill-availability test `mZe()`. The bundle's own
// enumeration of its environment variables carries nothing that reaches it:
// the only `CLAUDE_CODE_*ONBOARD*` name in the binary is
// `CLAUDE_CODE_POWERUP_ONBOARDING`, which is a different screen. (The other
// gate, `hHe`'s `lC().envOnboardingEligible`, is not a second condition — it
// is a startup LATCH of this same function: `if($Mr()===void
// 0)uUt(…mode==="auto"&&dO())`.) ASSUMED and not measured: that
// `envOnboarding` is an account-side flag. `Xu` is a name the bundle reuses
// across chunks for three unrelated functions and the one in that chunk's
// scope was not resolved — what is measured is that no local lever reaches
// it, which is the part this decision rests on. Unlike the upsell, whose
// CLAUDE_CODE_FORCE_FULLSCREEN_UPSELL is read before its own counter clause.
//
// So the sibling's heading is known from the bundle and from no capture, and
// a row read off a bundle is exactly the guess the paragraph above refuses.
// docs/notes.d/ranger-base-72oz4.md has the gate chain for whoever's account
// draws it.
var KnownScreens = []KnownScreen{{
	Name:    "codex.update_menu",
	Rules:   []string{"update_menu", "startup_update"},
	Markers: [][]string{{"update available"}},
}, {
	Name:  "codex.signin_menu",
	Rules: []string{"signin_menu"},
	// Two alternatives rather than a conjunction: the narrow layout draws
	// the device-code line below the fold, so a screen can carry one
	// heading without the other (codex/blocked-signin-narrow).
	Markers: [][]string{{"sign in with chatgpt"}, {"sign in with device code"}},
}, {
	Name:    "codex.signin_api_key",
	Rules:   []string{"signin_api_key"},
	Markers: [][]string{{"paste or type your api key"}, {"use your own openai api key"}},
}, {
	Name:    "codex.hooks_review",
	Rules:   []string{"hooks_review"},
	Markers: [][]string{{"hooks need review"}},
}, {
	Name:    "codex.trust_directory",
	Rules:   []string{"trust_directory"},
	Markers: [][]string{{"do you trust the contents of this directory"}},
}, {
	Name:    "codex.model_picker",
	Rules:   []string{"live_strong_blocker"},
	Markers: [][]string{{"select model and effort"}},
}, {
	Name:  "grok.startup_splash",
	Rules: []string{"startup_splash"},
	// The conjunction. "New worktree" alone is a menu row posse's own
	// output carries, and "Resume session" alone is in the changelog line.
	Markers: [][]string{{"new worktree", "resume session"}},
}, {
	Name: "grok.consent_banner",
	// No rule names it, and that is the finding rather than a gap: the
	// banner is drawn OVER the splash or over the composer, so herdr reads
	// whichever screen is under it and the banner is the thing nobody was
	// told about (ranger-base-3j8's first diagnosis).
	Rules:   nil,
	Markers: [][]string{{"help improve grok"}},
}, {
	// claude, and the one row in this table whose Rules field is the reason
	// NOT to expect anything of it (ranger-base-72oz4, from
	// ranger-base-yk5qb). MEASURED 2026-11-08 — herdr 0.9.1, claude 2.1.319
	// darwin-arm64, the shipped capture replayed through `herdr agent
	// explain --file --agent claude`: state `blocked`, rule
	// `live_blocked_form`, region `after_last_horizontal_rule`, priority
	// 980, out of herdr's own BUNDLED claude manifest 2026.09.11.1. So this
	// screen is a RECOGNIZED blocker, its residue is empty by construction,
	// and neither consumer of this reader is reached on it: awaitDelivered's
	// D3 block fires only when `det.Seen()` is false, and awaitSettled
	// returns at `case det.State == "blocked"` above its own. The bare line
	// an operator actually got — `never settled idle (status "blocked")`,
	// awaitAgent's Die — holds that detection and prints the status alone;
	// naming it there is ranger-base-x2grk and not this table's job.
	//
	// The row is here because the screen is real and the marker is read off
	// a capture this tree now ships, which is the only bar this file sets.
	// A claude release that reshapes the dialog past
	// `after_last_horizontal_rule` is what would make it speak, and until
	// then an honest empty residue is a better record than no row at all —
	// the alternative was a fixture labelled "no known screen here" over a
	// pane that plainly has one.
	//
	// ONE PHRASE, no conjunction, and the `?` dropped: the same shape as
	// codex.trust_directory, whose heading also ends in one. "try the new
	// fullscreen renderer" appears in no text that is not this screen except
	// prose about it, which is the standing caveat on every row here.
	Name:    "claude.fullscreen_upsell",
	Rules:   []string{"live_blocked_form"},
	Markers: [][]string{{"try the new fullscreen renderer"}},
}}

// KnownScreensIn is the reader: the SET of known screens whose heading is in
// the text it is handed, by name, sorted, nil when none is.
//
// IT TAKES THE RECORD'S REGION TEXTS — every one of them, the whole-pane
// capture included — and that input clause is the measured half of the
// amendment. herdr's `agent explain` emits a 243-character PREVIEW of each
// region, and over the previews alone this reader names 5 of the 11 residue
// cases instead of 11: for 9 of the 15 labelled screens the heading is
// outside the preview, so a reader fed the previews reads logo art
// (ranger-base-qk9tr). The capture that fixes it is `PaneCaptureRegion`,
// which ranger-base-76gc4 put on the D3 record for exactly this.
//
// SORTED, so the failure line and the census row are stable: a set printed
// in map order would make two identical readings look like two findings.
//
// NIL AND NOT AN EMPTY SLICE when nothing matched, because that is the
// answer `omitempty` has to be able to leave out of a record and
// `LooksLikeLine` has to be able to print nothing for. An empty set is the
// right answer on an idle composer and must not read as a reading that
// failed.
func KnownScreensIn(texts []string) []string {
	blob := normKnownScreen(strings.Join(texts, "\n"))
	if blob == "" {
		return nil
	}
	var hits []string
	for _, s := range KnownScreens {
		for _, marker := range s.Markers {
			if knownScreenMarkerIn(blob, marker) {
				hits = append(hits, s.Name)
				break
			}
		}
	}
	sort.Strings(hits)
	return hits
}

// knownScreenMarkerIn is the conjunction: every phrase of one marker.
//
// An EMPTY marker matches nothing, rather than matching everything the way
// an `all` over no elements would. A row whose markers went missing in an
// edit must name no screen at all; the pin is what says so out loud, and
// this is what keeps it from naming every screen in the table instead.
func knownScreenMarkerIn(blob string, marker []string) bool {
	if len(marker) == 0 {
		return false
	}
	for _, phrase := range marker {
		if !strings.Contains(blob, normKnownScreen(phrase)) {
			return false
		}
	}
	return true
}

// normKnownScreen lower-cases and collapses whitespace, which is the whole
// of the normalization and is the same two operations
// `scripts/d3-reader-eval.py`'s `norm` performs in the same order — the pin
// compares the two tables and this is what makes comparing them mean
// something.
//
// It is run over the MARKERS as well as the text, so a marker typed with a
// capital or a double space is still the marker it reads as. The alternative
// — trusting the table to be pre-normalized — is a row that silently matches
// nothing, which is the one failure shape a substring reader cannot report.
//
// Whitespace and nothing else. A TUI screen's heading is drawn with box
// borders, carets and padding around it but not inside it, and a reader that
// also folded punctuation would start matching prose about these screens —
// this file, the interstitial registry's own `Screen:` sentences, a runbook
// quoted into a pane.
func normKnownScreen(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// LooksLikeLine renders the report for a failure line: `looks like: <name>`,
// comma-separated where the set has more than one, and "" for an empty set.
//
// "" AND NOT "looks like: nothing". An empty set is the ordinary answer on
// every screen that is not in the table — which is most screens — and a
// row saying so on every refusal would be a diagnostic that trained its
// reader to skip the block (the reason WhatHerdrSaw groups by region and not
// by rule).
//
// The FULL dotted name, `codex.update_menu` and not `update_menu`: the
// failure line is read by somebody who does not yet know which runtime's
// screen they are looking at, and the two agents' tables will eventually
// hold a name in common.
func LooksLikeLine(screens []string) string {
	if len(screens) == 0 {
		return ""
	}
	return "looks like: " + strings.Join(screens, ", ")
}
