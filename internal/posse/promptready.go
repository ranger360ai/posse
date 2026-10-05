package posse

// Readiness for the TYPED prompt path — ranger-base-3p0.
//
// THE INCIDENT. `posse new <session>`, footer read auto-mode-on, and forty
// seconds later `posse prompt <session> "Work beads issue ..."`. herdr
// returned agent_prompted success. The pane held:
//
//	Unknown command: /Work. Did you mean /fork?
//	Args from unknown skill: beads issue ...        (plus stray "mc")
//
// A leading '/' the operator never typed turned the dispatch marker into a
// slash command and the rest of the work prompt into its arguments. A
// second, identical `posse prompt` after the session settled landed clean.
// Nothing about the text was wrong; the CLI was not holding the keyboard
// yet, and the keys landed in whatever was.
//
// WHY DISPATCH DOES NOT HAVE THIS BUG. awaitSettled (dispatch.go) already
// carries the whole diagnosis and the fix: detection is not readiness.
// herdr answers `idle` for a pane it has identified as a known agent even
// when NO rule matched anything — `agent explain` calls that
// default_known_agent_idle_fallback and reports matched_rule null,
// visible_idle false. That guess arrives before the CLI does, and a prompt
// typed into that window is typed at a shell, buffered through the exec,
// and delivered somewhere nobody chose. Dispatch waits for a state herdr
// has SEEN. `posse prompt` went straight from AgentTarget to AgentPrompt
// and waited for nothing — the same race, one entry point down.
//
// WHAT THIS GATE IS, AND WHAT IT DELIBERATELY IS NOT. It waits for herdr to
// have SEEN a screen — positive evidence, a matched rule or visible chrome
// (AgentDetection.Seen) — and NOT for a particular state. Dispatch wants
// `idle`, because it is starting work in a session it just created; the
// operator prompting by hand may well be nudging an agent that is mid-turn,
// and holding that prompt until the turn ends would be a new behaviour
// nobody asked for. A working pane herdr recognizes is a pane whose CLI has
// the keyboard, which is the only question this bug asks. So an established
// session pays exactly one `agent explain` call and nothing else; only a
// session herdr cannot recognize — the fresh one, mid-boot — waits at all.
//
// WHEN IT REFUSES. At the deadline with nothing but guesses, the prompt is
// NOT sent and the error says so. That is the direction the incident
// argues for: the mangled prompt reported success, so the operator had no
// signal at all, and the recovery was to prompt again by hand — which is
// exactly what the error line asks for. `--now` is the escape hatch for a
// runtime whose rules cannot see its working screen.
//
// WHEN DETECTION CANNOT BE READ AT ALL. Same concession dispatch makes: an
// `agent explain` that errors is not evidence of unreadiness any more than
// of readiness, and a prompt refused because a diagnostic call failed is
// worse than the race it guards. It prompts anyway, out loud, naming the
// error — but it does NOT spend the whole startup wait finding that out.
// An explain that has never once answered is a diagnostic that is not
// working (an older herdr, no such verb), not a measurement in progress,
// and making every hand prompt cost 45 silent seconds against such a herdr
// would be a worse regression than the bug. A guess is a real answer and
// holds the full wait; only the never-answered case is cut short.
//
// AND THE PANE WITH NO SCREEN READING AT ALL (ranger-base-8eqaa, folded into
// this gate by ranger-base-nm5i6). Everything above is about `agent explain`,
// which herdr refuses outright for a pane whose agent label it did not detect
// but somebody else REPORTED (agent_explain_unavailable). Against such a pane
// every poll of the gate errored and it fell into the concession in the
// paragraph above — the weakest reading there is, over a pane that has a
// perfectly good one.
//
// 8eqaa answered that with a SECOND gate beside this one, reading the reported
// lifecycle state off `agent get`. ADR 0061 D3 puts that reading inside
// `AgentExplain` instead, where all six consumers of a detection share it, and
// this file keeps ONE ladder again: the loop below is unchanged, every
// `det.Seen()` it reads is now true of a reported pane whose authority has
// watched a live CLI, and the only thing that branches is the REFUSAL's
// wording — because there is no screen to describe and no rule to name, so
// "herdr has not recognized a screen" would be a sentence about the wrong
// thing (D3.3).
//
// Two readings that could disagree was the whole risk: the second gate opened
// on ANY named state, where the one door also asks whether the pane is still
// live (D3.2), so a stranded label was promptable through one ladder and not
// the other. The fold is why that cannot be true of one build.

import (
	"fmt"
	"time"
)

const (
	// promptReadyPoll paces the gate: the sleep between one `agent explain`
	// and the next while the screen is still a guess.
	promptReadyPoll = 250 * time.Millisecond

	// promptExplainGrace bounds the never-answered concession above. Long
	// enough that a transient herdr hiccup is retried a few times, short
	// enough that a herdr with no `agent explain` costs a hand prompt an
	// eyeblink instead of a startup wait.
	promptExplainGrace = time.Second
)

// AwaitPromptable holds until herdr reports a detection it has SEEN in
// target's pane, and returns an error — with nothing typed — when it never
// does. The string is a note for the operator (a wait that was long enough
// to mention, or the concession above); "" means there is nothing to say.
//
// The detection is the evidence the gate opened on, handed back for callers
// whose rule is about the STATE and not only about the screen: the pulse
// will not interrupt a persona mid-turn, and reading that from the session
// listing is reading the very guess this gate exists to distrust
// (ranger-base-k99a). It is Seen() only on the opening path — the
// never-answered concession returns the zero detection, because a
// diagnostic that is not working is evidence of no state at all — and on
// the refusal it is the last guess, which is what the error already
// describes.
func (b *HerdrBackend) AwaitPromptable(session, target string) (AgentDetection, string, error) {
	// No reported pre-check here since ranger-base-nm5i6: AgentExplain's own
	// refusal is what routes a reported pane onto its reading, so the loop
	// below reads one and judges it with the same `Seen()` it judges a screen
	// with (ADR 0061 D3.1). A reported pane pays one refused `agent explain`
	// per poll for that — MEASURED 0.00s on herdr 0.9.1 — and buys a route
	// that is re-derived every read rather than decided once.
	wait := b.promptReadyWait(session)
	start := time.Now()
	deadline := start.Add(wait)
	// answered: herdr explained at least once in this window. It is the
	// difference between "the screen is unrecognized" (a real answer, worth
	// the full wait and worth refusing on) and "the diagnostic is not
	// working" (worth neither).
	answered := false
	attempts := 0
	var lastGuess AgentDetection
	var lastErr string
	for {
		attempts++
		det, err := b.H.AgentExplain(target)
		switch {
		case err != nil:
			lastErr = err.Error()
		case det.Seen():
			// News is a second `agent explain`, not a slow first one: a box
			// under load can make one subprocess call take longer than a
			// poll all by itself, and that is not the gate looping — it is
			// the ordinary cost of asking (ranger-base-vstc).
			if attempts > 1 {
				return det, fmt.Sprintf("waited %s for %s %s (%s)",
					time.Since(start).Round(100*time.Millisecond), session, waitedFor(det), seenBy(det)), nil
			}
			return det, "", nil
		default:
			lastErr, answered, lastGuess = "", true, det
		}
		if !answered && time.Since(start) >= promptExplainGrace {
			break
		}
		if !time.Now().Add(promptReadyPoll).Before(deadline) {
			break
		}
		time.Sleep(promptReadyPoll)
	}
	if !answered {
		return AgentDetection{}, fmt.Sprintf("herdr cannot explain %s (%s) — prompting without the readiness gate", session, lastErr), nil
	}
	// The REPORTED wording (ADR 0061 D3.3). Same refusal, same remedies, and
	// a different sentence about the cause, because on this route there is no
	// screen herdr failed to recognize: what there is, is another authority's
	// word, and either it says nothing useful or the pane it describes has
	// gone back to its shell. ReportedNotSeen is the one copy of that clause.
	if why := lastGuess.ReportedNotSeen(); why != "" {
		b.logUnrecognized(session, target, lastGuess, "reported-not-seen: "+why)
		return lastGuess, "", Die("nothing was sent: %s is labelled by something other than herdr's own detection, and %s "+
			"(ranger-base-3p0). Prompt again once it has settled, look first (posse peek %s), or send it anyway with --now.%s",
			session, why, session, lastGuess.WhatHerdrSaw(b.reportedWhy(session)))
	}
	reason := lastGuess.FallbackReason
	if reason == "" {
		reason = "no rule matched"
	}
	b.logUnrecognized(session, target, lastGuess, "unrecognized screen: "+lastGuess.State+" ("+reason+")")
	return lastGuess, "", Die("nothing was sent: herdr has not recognized a screen in %s within %s — it reports %q with %s, "+
		"which is what a CLI that has not taken the keyboard yet looks like, and text typed there lands in whatever has it "+
		"(ranger-base-3p0). Prompt again once it has settled, look first (posse peek %s), or send it anyway with --now.%s",
		session, wait, lastGuess.State, reason, session, lastGuess.WhatHerdrSaw(""))
}

// reportedWhy is the DECLARATION behind a reported pane's label — the
// runtime's `detection_why:`, which is the sentence that names the authority
// (ADR 0061 D1 requires it exactly so this line has something to name).
//
// Read on the FAILURE path only, and only from the profile, so it costs no
// herdr call and the happy exit pays nothing for it. Empty for every way of
// not having one: no meta, no runtime, a profile that will not load, or a
// runtime that declares `herdr` — a pane can be reported on a runtime that
// never declared it (the reading is a property of the pane, not of the
// profile), and quoting a `detection_why:` that is not there would be posse
// putting words in an operator's mouth.
func (b *HerdrBackend) reportedWhy(session string) string {
	m, ok := b.readMeta(session)
	if !ok || m.Runtime == "" || b.App == nil {
		return ""
	}
	rt, err := b.App.LoadRuntime(m.Runtime)
	if err != nil || rt.DetectionMode() != DetectionReported {
		return ""
	}
	return rt.DetectionWhy
}

// orUnknown names the state a reported pane has when it has none, in the
// authority's own vocabulary rather than as an empty string in a message.
func orUnknown(state string) string {
	if state == "" {
		return "unknown"
	}
	return state
}

// waitedFor names what the gate was waiting FOR, which is not the same
// question on the two routes and must not be reported as though it were: a
// reported pane never draws a screen herdr recognizes and never will, so
// saying it finally did would be the one sentence in the note that is false
// (ADR 0061 D3.3, the success side of it).
func waitedFor(det AgentDetection) string {
	if det.Reported != "" {
		return "to be labelled by the authority that labels its panes"
	}
	return "to draw a screen herdr recognizes"
}

// seenBy names the positive evidence the gate opened on, in herdr's own
// words — the rule that matched, the chrome it saw without one, or, for a
// pane herdr did not detect at all, the label whoever did report it used.
func seenBy(det AgentDetection) string {
	switch {
	case det.Rule.ID != "":
		return fmt.Sprintf("%s via rule %q", det.State, det.Rule.ID)
	case det.VisibleIdle:
		return fmt.Sprintf("%s, visible chrome", det.State)
	}
	return fmt.Sprintf("%s, reported for agent %q (herdr detects no such kind)", det.State, det.Reported)
}

// promptReadyWait is how long this session's runtime is given to reach a
// screen herdr knows — the same patience its launch got (Runtime.Wait), so
// a runtime measured to be slow off the mark is not gated tighter by hand
// than dispatch gates it. A session with no meta, no runtime, or a runtime
// that will not load falls back to the claude-shaped default: unknown means
// the ordinary patience, never none.
func (b *HerdrBackend) promptReadyWait(session string) time.Duration {
	m, ok := b.readMeta(session)
	if !ok || m.Runtime == "" || b.App == nil {
		return DefaultStartupWait
	}
	rt, err := b.App.LoadRuntime(m.Runtime)
	if err != nil {
		return DefaultStartupWait
	}
	return rt.Wait()
}
