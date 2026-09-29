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

// ─── the one detection READING for a reported pane (ADR 0061 D3) ─────────────
//
// ranger-base-nm5i6. Everything above is DELIVERY — how a typed prompt
// reaches a pane herdr labels but does not detect. This is DETECTION: what
// every gate in this codebase reads before it decides that pane is settled,
// promptable, or holding something.
//
// ONE DOOR, AND WHY IT IS AgentExplain. Six readings in this repo judge a
// pane from an AgentDetection — the dispatch settle ladder, the delivered
// wait, the promptable gate, the pulse, the pane-holding read, the probe's
// detection observable — and every one of them got there through
// `Herdr.AgentExplain`. herdr refuses that verb outright for a reported pane
// (agent_explain_unavailable), so before this block all six read an ERROR and
// each fell into its own concession: dispatch spent a whole startup wait and
// then prompted on a state it had not seen, the probe recorded "herdr could
// not explain the pane", and the promptable gate needed a second ladder of
// its own (8eqaa's awaitReportedPromptable, folded into the first by this
// bead). Six concessions over one pane that has a perfectly good reading is
// the drift ADR 0061 D3 closes: the reported arm goes INSIDE the door, so no
// consumer branches and no two readings can disagree.
//
// THE READING COSTS ONE REFUSED CALL. `agent explain` is asked first, every
// time, because there is no way to know a pane is reported without asking —
// and that is the right order anyway: the route is re-derived per read, so a
// pane that becomes detected (the day herdr ships the kind, or the instant an
// aliased manifest loads) is read as detected from the next poll on, with
// nothing to clear. MEASURED 0.00s for the refusal on herdr 0.9.1.
//
// IDENTITY IS NOT LIVENESS, and this is where that title is paid for. A
// reported label is a pidfile: herdr ties it to the PANE, and only the
// reporter or the pane's close ever clears it (MEASURED 2026-09-28, ADR 0061
// Claims fact 1 — a pane reported `working` ran `sleep 6`, returned to its
// shell prompt, and `agent get` still read `working` ten minutes later; the
// herdr-bob plugin releases on `pane.exited` and on nothing else). So a Bob
// that exits to its shell keeps reading `idle`, and a work prompt typed at
// that reading is typed at a shell, which EXECUTES every line of it. The
// detected route gets this guard for free — herdr drops a detected label when
// argv0 leaves the pane (Claims fact 3) — and the liveness half below is the
// one keystroke-shaped hazard the reported route adds. It is herdr's own
// process reading, so no argv is matched, no shell list is kept, and no
// runtime is named (ADR 0017 §3).

// reportedExplainRefusal is herdr's OWN code for "this pane has no detected
// agent label", and the one thing that routes a reading onto the arm below.
// An error code used as a type discriminator, which is what herdr leaves
// posse to infer "this label was reported" from: `agent get` carries no
// source field, even though `pane report-agent --source` accepts one
// (ranger-base-mx5x9 #4, and ADR 0061 D4(b) is the ask).
//
// Narrow on purpose. Every OTHER way `agent explain` can fail — a timeout, a
// herdr off PATH, `agent_not_found` — stays the error it always was, because
// only this code is positive evidence that herdr looked and declined.
const reportedExplainRefusal = "agent_explain_unavailable"

// reportedStateIsWatched reports whether a reported lifecycle state is
// positive evidence that some authority has actually watched this CLI.
//
// `idle`, `working`, `done` and `blocked` are: an authority does not report a
// lifecycle state about a CLI it has not seen start. `unknown` — and no state
// at all — is the reported way of saying nothing is known, and it waits
// exactly as herdr's own idle guess does (ADR 0061 D3.1). herdr agrees about
// that one from the other side: `agent wait --until idle --until done --until
// blocked` on a reported `unknown` times out rather than settling (MEASURED
// 2026-09-28).
//
// A closed set, and it has to be: the point of Seen() is positive evidence,
// so a state this list has never heard of is not evidence, the same way a
// fallback herdr names differently tomorrow is not readiness.
func reportedStateIsWatched(state string) bool {
	switch state {
	case "idle", "working", "done", "blocked":
		return true
	}
	return false
}

// reportedDetection is the reported arm of the one reading. It answers only
// for the positively-measured shape — herdr REFUSED to explain this pane, and
// the label it carries is one herdr has no manifest for — and every other way
// of reading nothing hands the caller back the error `agent explain` gave,
// exactly as before this arm existed.
//
// explainErr is taken as an argument rather than re-derived so that the
// discriminator lives in one file with the measurement that justifies it.
func (h Herdr) reportedDetection(target string, explainErr error) (AgentDetection, bool) {
	if !IsHerdrCode(explainErr, reportedExplainRefusal) {
		return AgentDetection{}, false
	}
	ag, ok := h.ReportedAgent(target)
	if !ok {
		return AgentDetection{}, false
	}
	return AgentDetection{
		State:           ag.AgentStatus,
		Reported:        ag.Agent,
		ShellForeground: h.paneShellForeground(ag.PaneID),
	}, true
}

// PaneProcesses is the part of `herdr pane process-info` the liveness half
// reads: the pane's own shell, and the pids herdr says are in its foreground.
//
// Two fields out of a rich answer, deliberately. `foreground_process_group_id`
// sits right beside them and argv0/name are in the same rows
// (ranger-base-mx5x9 #1, #5) — and reading the argv would be a second
// detection keyed on a name, which is the shape ADR 0061 rejected ("liveness
// by matching argv against the label": it fails on interpreter-launched CLIs
// like `node …/bob`, and the label's own source IS argv0, so argv is not
// independent evidence about it). A pid compared to a pid needs no name.
type PaneProcesses struct {
	ShellPID   int
	Foreground []int
}

// AtShellPrompt reports whether every foreground process in the pane IS the
// pane's own shell — the pane is sitting at a bare prompt, whatever any label
// says about it.
//
// MEASURED 2026-09-28 (herdr 0.9.1, ADR 0061 Claims fact 2): at a bare prompt
// `foreground_processes` is exactly `[{pid 34661, argv0 zsh}]` beside
// `shell_pid 34661`; during `sleep 5` in the same pane it is `[{pid 82630,
// argv0 sleep}]`. So the comparison is the whole reading.
//
// AN EMPTY ANSWER IS NOT A YES, and neither is a missing shell_pid. "Every"
// over an empty set is vacuously true, which would turn a herdr whose output
// moved into a box where every reported label reads stale and no reported
// pane is ever promptable. Both are UNKNOWN here, and UNKNOWN refuses
// nothing — the rule every reading in this repo keeps.
func (p PaneProcesses) AtShellPrompt() bool {
	if p.ShellPID == 0 || len(p.Foreground) == 0 {
		return false
	}
	for _, pid := range p.Foreground {
		if pid != p.ShellPID {
			return false
		}
	}
	return true
}

// PaneProcessInfo is `herdr pane process-info --pane <pane>` — liveness from
// state the kernel ties to a process's existence, which is the field's own
// answer to "is this thing still alive", read from herdr.
//
// MEASURED 0.00s (ADR 0061 Claims; 0.01–0.02s under a `time` that resolves
// finer, ranger-base-mx5x9 #5), which is what makes it affordable once per
// read rather than once per session.
func (h Herdr) PaneProcessInfo(pane string) (PaneProcesses, error) {
	res, err := h.Run("pane", "process-info", "--pane", pane)
	if err != nil {
		return PaneProcesses{}, err
	}
	var payload struct {
		Info struct {
			ShellPID   int `json:"shell_pid"`
			Foreground []struct {
				PID int `json:"pid"`
			} `json:"foreground_processes"`
		} `json:"process_info"`
	}
	if err := json.Unmarshal(res, &payload); err != nil {
		return PaneProcesses{}, Die("herdr pane process-info --pane %s: bad response", pane)
	}
	p := PaneProcesses{ShellPID: payload.Info.ShellPID}
	for _, f := range payload.Info.Foreground {
		p.Foreground = append(p.Foreground, f.PID)
	}
	return p, nil
}

// paneShellForeground is the liveness half as one bool, with the concession
// this whole file is built on: a process reading that cannot be TAKEN is not
// evidence that the pane is at a shell. It answers false, the label stands on
// its own as it did before ADR 0061, and nothing is refused over a diagnostic
// verb that went missing.
//
// That direction is the uncomfortable one and it is still right. False here
// means a stranded label can be believed on a herdr that will not answer —
// but true would mean a herdr that will not answer makes every reported pane
// unpromptable, which is a box that has stopped dispatching rather than one
// running a risk somebody measured.
func (h Herdr) paneShellForeground(pane string) bool {
	p, err := h.PaneProcessInfo(pane)
	if err != nil {
		return false
	}
	return p.AtShellPrompt()
}
