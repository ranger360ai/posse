package rhq

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

import (
	"fmt"
	"time"
)

const (
	// promptReadyPoll paces the gate. It is also the threshold under which
	// a wait is not reported: one poll's worth of delay is the ordinary
	// cost of asking, not news.
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
	wait := b.promptReadyWait(session)
	start := time.Now()
	deadline := start.Add(wait)
	// answered: herdr explained at least once in this window. It is the
	// difference between "the screen is unrecognized" (a real answer, worth
	// the full wait and worth refusing on) and "the diagnostic is not
	// working" (worth neither).
	answered := false
	var lastGuess AgentDetection
	var lastErr string
	for {
		det, err := b.H.AgentExplain(target)
		switch {
		case err != nil:
			lastErr = err.Error()
		case det.Seen():
			if waited := time.Since(start); waited >= promptReadyPoll {
				return det, fmt.Sprintf("waited %s for %s to draw a screen herdr recognizes (%s)",
					waited.Round(100*time.Millisecond), session, seenBy(det)), nil
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
	reason := lastGuess.FallbackReason
	if reason == "" {
		reason = "no rule matched"
	}
	return lastGuess, "", Die("nothing was sent: herdr has not recognized a screen in %s within %s — it reports %q with %s, "+
		"which is what a CLI that has not taken the keyboard yet looks like, and text typed there lands in whatever has it "+
		"(ranger-base-3p0). Prompt again once it has settled, look first (posse peek %s), or send it anyway with --now.%s",
		session, wait, lastGuess.State, reason, session, lastGuess.WhatHerdrSaw())
}

// seenBy names the positive evidence the gate opened on, in herdr's own
// words — the rule that matched, or the chrome it saw without one.
func seenBy(det AgentDetection) string {
	if det.Rule.ID != "" {
		return fmt.Sprintf("%s via rule %q", det.State, det.Rule.ID)
	}
	return fmt.Sprintf("%s, visible chrome", det.State)
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
