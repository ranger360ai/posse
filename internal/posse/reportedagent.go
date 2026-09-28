package posse

// Typed prompt delivery for a REPORTED agent — ranger-base-8eqaa.
//
// THE GAP. herdr's `agent prompt` verb addresses only the agent kinds herdr
// itself detects. A pane labelled through `herdr pane report-agent` — the
// documented "integrate your own agent" route, and the route every herdr
// PLUGIN takes — is labelled, listed and stateful, and `agent prompt`
// refuses it anyway:
//
//	agent get w2KJ:p1     → {"agent":"fakebob","agent_status":"idle",…}
//	agent explain w2KJ:p1 → agent_explain_unavailable: does not have a
//	                        detected agent label
//	agent prompt w2KJ:p1  → agent_not_ready: is not an active named agent
//
// MEASURED 2026-09-28 on herdr 0.9.1, this box, against a scratch pane
// reported under a label of no significance — the shape is the report, not
// the runtime. It is the same answer the MartinLoeper/herdr-bob plugin's
// probe got for a live Bob pane in idle AND in done (ranger-base-p8afi), so
// `posse prompt` on such a session refused with nothing sent.
//
// WHAT THIS FILE DOES. One route decision, taken from herdr's own facts and
// nothing else: a pane that carries an agent LABEL herdr has no detection
// manifest for was labelled by somebody else, so the prompt is delivered by
// `pane send-text` + `pane send-keys enter` instead. Every other pane —
// every claude, codex and grok pane there has ever been — takes `agent
// prompt`, unchanged, because the manifest answers for its label.
//
// NOT KEYED ON A RUNTIME NAME. ADR 0060 rejected a report-agent adapter
// partly as "two mechanisms, both keyed on one runtime's name" — the ADR
// 0017 §3 shadow-predicate class. There is no `bob` in this file and no
// runtime lookup: the discriminator is a property of the PANE, asked of
// herdr, so a template runtime whose detection arrives from any plugin
// tomorrow is delivered to without a line changing here. The day herdr
// compiles the kind in, AgentManifest answers `known` and this whole path
// stops being taken — no flag to clear, no list to prune.
//
// UNKNOWN IS NEVER A "NO". Both readings degrade to the old route. A herdr
// that cannot be asked, a pane with no label, a manifest probe that will not
// answer: all of them mean `agent prompt`, exactly as before. Only the
// positively-measured shape — a label herdr HAS been asked about and does
// NOT know — switches routes. That is the same rule KnownAgentKinds and
// RuntimeGaps already apply, and it is what keeps this from typing a work
// prompt at a shell the way ranger-base-3p0 did.

import (
	"encoding/json"
	"strings"
	"time"
)

// herdrPromptStallMS is herdr's OWN number, quoted rather than chosen:
// `herdr agent prompt --help` on 0.9.1 states "When an accepted submission
// starts from another non-working state, --wait requires an observed working
// or blocked state within 5000ms; otherwise it returns agent_prompt_stalled."
//
// The send-text route mirrors that contract instead of inventing one,
// because dispatch already branches on `agent_prompt_stalled` as "the prompt
// never reached the agent" (dispatch.go, rangerhq-1z0). A route that settled
// on silence would hand that branch a different meaning for the same code.
const herdrPromptStallMS = 5000

// AgentInfo is `herdr agent get <target>` — the pane's agent label and the
// state herdr carries for it, whoever put them there.
//
// It is the one reading that answers for a reported pane at all: `agent
// explain` refuses it (agent_explain_unavailable), so every gate in this
// codebase built on AgentDetection is blind to it, and `agent list` would
// make a per-pane question cost a whole listing.
func (h Herdr) AgentInfo(target string) (HerdrAgent, error) {
	res, err := h.Run("agent", "get", target)
	if err != nil {
		return HerdrAgent{}, err
	}
	var payload struct {
		Agent HerdrAgent `json:"agent"`
	}
	if err := json.Unmarshal(res, &payload); err != nil {
		return HerdrAgent{}, Die("herdr agent get %s: bad response", target)
	}
	return payload.Agent, nil
}

// ReportedAgent reports whether target's pane carries an agent label that
// herdr did not detect and cannot address — a label some other authority
// stated through `pane report-agent`.
//
// Two readings, in this order, and a miss on either one answers false:
//
//  1. `agent get` — is there a label at all, and which pane holds it. A pane
//     herdr will not describe, or one with no label, is not this shape.
//  2. AgentManifest — does herdr have a detection manifest for that label.
//     Asked the way RuntimeGaps asks it, through `agent explain --file` on
//     an empty screen, so an `aliases = […]` route resolves here too: an
//     operator who aliased their CLI onto another manifest HAS herdr
//     detection, and `agent prompt` addresses them.
//
// The false answer is deliberately unconditional — no error surfaces. Every
// way of failing to read means "deliver the way posse always has", which is
// the behaviour that predates this file; an error return would invite a
// caller to refuse a prompt because a diagnostic verb was missing, and
// promptready.go's whole concession exists to say that is worse than the
// race it would guard.
func (h Herdr) ReportedAgent(target string) (HerdrAgent, bool) {
	ag, err := h.AgentInfo(target)
	if err != nil || ag.Agent == "" || ag.PaneID == "" {
		return HerdrAgent{}, false
	}
	switch _, known, ok := h.AgentManifest(ag.Agent); {
	case !ok, known:
		return HerdrAgent{}, false
	}
	return ag, true
}

// sendTextPrompt delivers text to a reported agent's pane and mirrors the
// contract `agent prompt` documents for the route it will not take:
//
//   - already blocked → agent_blocked, and NOTHING is sent. herdr rejects a
//     submission to a blocked agent before any input, because 0.8.0 typed
//     into the dialog and the text was swallowed (rangerhq-ejf). The same
//     keystroke is the same swallow here.
//   - --wait from a non-working state → the turn must be OBSERVED working
//     or blocked within herdrPromptStallMS, else agent_prompt_stalled.
//     Without it, `agent wait --until idle` would return instantly on the
//     state that was already there and report a turn that never started.
//   - already working → no such wait, exactly as herdr says: "if the agent
//     is already working, that active turn's completion may match".
//
// MEASURED 2026-09-28 (herdr 0.9.1, the scratch pane above): send-text +
// send-keys enter lands, `agent wait --until working` returns on a reported
// transition, and both waits answer in the `result.agent.agent_status` shape
// agentStatusFromResult already reads — so every caller's judgement of the
// settle is the judgement it was already making.
func (h Herdr) sendTextPrompt(ag HerdrAgent, text string, wait bool, timeoutMS int) (json.RawMessage, error) {
	if ag.AgentStatus == "blocked" {
		return nil, HerdrAPIError{Code: "agent_blocked",
			Message: "agent " + ag.PaneID + " is blocked awaiting input; nothing was typed"}
	}
	// ADR 0060 D5, and the one hazard that gets a guard rather than a line
	// in a profile now that this route has a caller. A leading '/' opens the
	// CLI's command picker, and the picker EATS the Enter that follows: the
	// text sits in the composer, this call reports success, and nobody finds
	// out — ranger-base-3p0's failure with the cause one layer over. It is
	// refused on this route only, because this route is the one with no
	// herdr envelope behind it to say the submit happened and no `agent
	// explain` to read the box back with (ConfirmSubmitted is blind to a
	// reported pane by construction).
	if strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), "/") {
		return nil, Die("nothing was sent: %s is a %s pane herdr labels but does not detect, so posse types the prompt in "+
			"(pane send-text + Enter) — and a prompt starting with '/' opens the CLI's command picker, which eats the Enter "+
			"and leaves the text sitting in the composer while this call reports success (ADR 0060 D5). "+
			"Reword it without the leading '/', or drive the picker by hand (posse peek)",
			ag.PaneID, ag.Agent)
	}
	if _, err := h.Run("pane", "send-text", ag.PaneID, text); err != nil {
		return nil, err
	}
	if _, err := h.Run("pane", "send-keys", ag.PaneID, "enter"); err != nil {
		return nil, err
	}
	if !wait {
		// Honest about the two things this route cannot claim: it names the
		// delivery, and it carries NO agent_status, because nothing has been
		// observed since the keystroke. agentStatusFromResult reads "" out
		// of it, which is what every caller here does with a no-wait prompt
		// result already.
		type reportedPane struct {
			Agent  string `json:"agent"`
			PaneID string `json:"pane_id"`
		}
		return json.Marshal(struct {
			Type  string       `json:"type"`
			Via   string       `json:"via"`
			Agent reportedPane `json:"agent"`
		}{"agent_prompted", "pane send-text", reportedPane{ag.Agent, ag.PaneID}})
	}
	start := time.Now()
	left := func() int {
		if timeoutMS <= 0 {
			return 0
		}
		if ms := timeoutMS - int(time.Since(start)/time.Millisecond); ms > 0 {
			return ms
		}
		return 1 // expired: let herdr return its own typed timeout
	}
	if ag.AgentStatus != "working" {
		// "A caller timeout that expires first returns timeout" — so a
		// caller who asked for less patience than herdr's stall window gets
		// their own answer back, not a stall. The two mean opposite things
		// to dispatch and the shorter of the two is what actually expired.
		stall, caller := herdrPromptStallMS, false
		if timeoutMS > 0 && timeoutMS < stall {
			stall, caller = timeoutMS, true
		}
		res, err := h.AgentWait(ag.PaneID, []string{"working", "blocked"}, stall)
		switch {
		case IsHerdrCode(err, "timeout") && !caller:
			return nil, HerdrAPIError{Code: "agent_prompt_stalled",
				Message: "typed the prompt into " + ag.PaneID + " but no turn started within " +
					(time.Duration(stall) * time.Millisecond).String()}
		case err != nil:
			return nil, err
		}
		// A turn that went straight to blocked has settled on the one state
		// the second wait would answer instantly anyway; return the reading
		// that saw it rather than taking it again.
		if agentStatusFromResult(res) == "blocked" {
			return res, nil
		}
	}
	return h.AgentWait(ag.PaneID, []string{"idle", "done", "blocked"}, left())
}
