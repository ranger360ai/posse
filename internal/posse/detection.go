package posse

// The launch stage's first observable, as ONE reading (ADR 0013 §1, amended
// 2026-09-28 by ranger-base-i3q6g; built under ranger-base-d8riq, and the
// relaunch arms added under ranger-base-enmu2).
//
// The question is "does herdr have a detection manifest for this runtime's
// argv0", and until this file it was asked twice — once by `posse runtime
// check`'s launch row and once by the preflight's detection gap — and by no
// launch at all. The grid said *refuse the launch* and nothing refused one.
// That is the third time a row of ADR 0013 §1's grid held a printed sentence
// with no launch behind it (9r33's danger refuse, vbp3's declared screen,
// now detection), which is why the reading is a value here rather than a
// switch at each call site: a "missing → refuse" cell in that grid is a
// claim about a launch, and three surfaces reading one function is the only
// way the grid and the launcher cannot drift apart again.
//
// THREE ANSWERS, AND ONLY ONE OF THEM REFUSES.
//
//   - ManifestKnown — herdr names it. Launch.
//   - ManifestUnknownAgent — herdr's OWN answer that it has no manifest for
//     this label. This is the refuse: every state of such a session is
//     `agent_not_found`, so `working` and every settled state are guesses and
//     dispatch cannot address the pane at all.
//   - ManifestUnreadable — herdr could not be asked, or its output moved.
//     UNKNOWN, never a "no": nothing refuses, and the launch proceeds exactly
//     as it did before this file existed. That is 9r33's rule, and it is what
//     keeps this from walling a box over a diagnostic verb that went missing.
//
// ASKED THE WAY HERDR RESOLVES IT. `agent explain --agent <label>` over an
// empty screen, so a manifest reached through another agent's
// `aliases = [...]` resolves too — on herdr 0.8.0 the only route a CLI herdr
// was not built with has to detection at all (Herdr.AgentManifest). The
// compiled kind list is the fallback for a herdr whose `agent explain` could
// not be read, and the reading remembers which route answered because the
// two do not know the same things: the kind list cannot see an alias, so an
// absence there is "does not recognize" and not "has no manifest".
//
// NOT CACHED. MEASURED 2026-09-28, herdr 0.9.1 on this box: 0.00s for an
// unknown label, 0.03–0.04s for a known one. A cached reading would be a
// second store of a fact herdr owns, stale across the very upgrade the bob
// tripwire waits for (ADR 0013 §1, "Rejected, priced").

import "fmt"

// ManifestState is herdr's answer about one agent label, in the three shapes
// a caller can act on.
type ManifestState int

const (
	// ManifestUnreadable is UNKNOWN — the zero value, because every way of
	// failing to read has to land here rather than on a "no".
	ManifestUnreadable ManifestState = iota
	ManifestKnown
	ManifestUnknownAgent
)

// ManifestReading is one reading of the launch observable for one runtime's
// argv0, carrying everything the three surfaces print about it.
type ManifestReading struct {
	Argv0   string // the label that was asked about
	Version string // herdr's manifest version, where it named one
	State   ManifestState
	// ViaKinds: the answer came from the compiled `[possible values:]` list
	// rather than from `agent explain`. It cannot see an alias, so its
	// absence is the weaker claim and is worded as one.
	ViaKinds bool
}

// Undetectable is the one state a launch refuses on, named as a predicate so
// no caller has to remember which of the other two is the safe one.
func (r ManifestReading) Undetectable() bool { return r.State == ManifestUnknownAgent }

// ReadDetection asks herdr the launch stage's first question about one
// runtime's argv0. It is the function `posse runtime check`'s launch row,
// the preflight's detection gap and every launch refusal all read.
func ReadDetection(h Herdr, exe string) ManifestReading {
	r := ManifestReading{Argv0: exe}
	// An empty argv0 is a different fact with its own blocking gap
	// ("command: renders no executable at all"), and herdr was never asked
	// about it — AgentManifest refuses an empty label before it runs
	// anything. Reporting it as a detection answer would put herdr's name on
	// a sentence herdr never said.
	if exe == "" {
		return r
	}
	if ver, known, ok := h.AgentManifest(exe); ok {
		r.State, r.Version = ManifestUnknownAgent, ""
		if known {
			r.State, r.Version = ManifestKnown, ver
		}
		return r
	}
	kinds := h.KnownAgentKinds()
	if kinds == nil {
		return r
	}
	r.ViaKinds, r.State = true, ManifestUnknownAgent
	if containsString(kinds, exe) {
		r.State = ManifestKnown
	}
	return r
}

// DetectionGapLine is the reading as the preflight's gap sentence — the
// operator-facing half of `runtime check`'s `✗ detection`. Empty for every
// reading that is not a refusal, so a caller cannot print a gap over an
// UNKNOWN by forgetting to branch.
//
// It ends in the same DetectionDoor the refusal does, which is the whole
// point of them sharing a file: the grid used to tell a built-in's operator
// to author a manifest for an argv0 whose detection is upstream's to ship,
// and once a launch REFUSES on this reading, the gap and the refusal saying
// different things about the way out is a drift with a cost.
func DetectionGapLine(rt *Runtime, r ManifestReading) string {
	if !r.Undetectable() {
		return ""
	}
	// "does not recognize" where the compiled kind list answered and "has no
	// manifest" where `agent explain` did: the kind list cannot see an alias,
	// so an absence there is the weaker of the two claims and is worded as
	// one.
	saw := fmt.Sprintf("herdr has no detection manifest for argv0 %q", r.Argv0)
	if r.ViaKinds {
		saw = fmt.Sprintf("herdr does not recognize argv0 %q", r.Argv0)
	}
	return saw + " — a dispatched session is agent_not_found, so it cannot be addressed at all. " + DetectionDoor(rt)
}

// DetectionRow is the reading as the launch row's own clause in the grid.
func DetectionRow(r ManifestReading) string {
	switch {
	case r.State == ManifestUnreadable:
		return "herdr recognition UNKNOWN (herdr not on PATH, or its output moved)"
	case r.Undetectable():
		return fmt.Sprintf("herdr does NOT recognize argv0 %q — no detection here, so work/settle are guesses", r.Argv0)
	case r.Version == "":
		return fmt.Sprintf("herdr recognizes argv0 %q", r.Argv0)
	default:
		return fmt.Sprintf("herdr recognizes argv0 %q (detection manifest %s)", r.Argv0, r.Version)
	}
}

// DetectionDoor is where an operator goes to END this refusal, and it is the
// one clause that branches on the runtime rather than on the reading.
//
// A DECLARED runtime's door is the manifest runbook: the profile's author
// writes the toml, or aliases their CLI onto a manifest that already exists,
// and detection arrives on their own box.
//
// A BUILT-IN's is not. Its argv0 is posse's own and its detection is
// upstream's to ship — posse ships the filing and the operator sends it
// (ADR 0060 D2, etc/herdr/agent-detection/upstream-<name>.md). Branched on
// rt.Builtin and never on a runtime NAME: a name-keyed clause here would be
// the ADR 0017 §3 shadow predicate ADR 0060 rejected a whole adapter over,
// and would say the wrong thing about the second built-in that meets this.
func DetectionDoor(rt *Runtime) string {
	if rt.Builtin {
		return "detection for a built-in is upstream's to ship, not yours to author: posse ships the filing at etc/herdr/agent-detection/upstream-" +
			rt.Name + ".md and the operator sends it (ADR 0060 D2)"
	}
	return "author a detection manifest, or alias " + rt.Exe() + " onto one that exists: docs/runbooks/" + detectionDoc
}

// DetectionRefusal is the launch refusal itself — the ADR 0013 §1 property
// that says the line names the CAUSE and the DOOR and never the session.
//
// It never says "check the session": there is none to check, and that is the
// whole point of refusing here. The line the gap replaces is
// `no agent detected in <session> after 45s — check the session`, which
// reads as a slow start whose remedy is a larger `startup_wait:` — a remedy
// that changes nothing, on a session that had already spent a worktree, a
// workspace, a pane and the runtime's own first turn.
//
// A plain error, like DangerRefusal, so fireLoop's three-way switch lands it
// on the default arm and benches the SLOT: every bead routed to this persona
// on this runtime meets the same missing manifest, and claiming them one at
// a time to refuse them one at a time is the sterilised queue ADR 0013 §2
// named once (property 4).
func DetectionRefusal(rt *Runtime, r ManifestReading) error {
	return Die("%s launch refused: herdr has no detection manifest for argv0 %q\n"+
		"  ADR 0013 §1: a dispatched session there is agent_not_found and cannot be addressed at all — every state herdr reports for it is a guess, so the prompt, the settle and the turn's own outcome are all unreadable\n"+
		"  %s\n"+
		"  the whole grid, with what this box reads today: posse runtime check %s",
		rt.Name, r.Argv0, DetectionDoor(rt), rt.Name)
}

// DetectionRetypeRefusal is the RE-TYPE arm's refusal — ADR 0013 §1 property
// 3's third path, the one that does not create anything (ranger-base-enmu2).
//
// It needs its own sentence because its cause is the one place this rule is
// not about a session that would be created. `RelaunchAgent` fires on the
// *agent gone, session kept* reading: the workspace is alive, herdr reports
// no agent in it, and on every runtime herdr CAN name that means the CLI
// exited and left a bare shell, so re-typing the launch line there is a full
// persona restart (rangerhq-vk2).
//
// On a runtime herdr cannot name, "no agent in this session" is the STEADY
// STATE of a perfectly live CLI — every reading of such a session is
// `agent_not_found` — so the same signal is not evidence of anything, and the
// line goes into the running TUI's composer as a chat turn. That is the
// costliest end of this whole gap: the first arm spends a pane, this one
// spends the operator's live session and a turn of the model's attention on a
// prompt that reads as a user message.
//
// A plain error like DetectionRefusal, for the same reason: launchSession
// hands RelaunchAgent's error straight back, so fireLoop's three-way switch
// lands it on the default arm and benches the SLOT rather than blaming the
// pane (sessionFailure) or the bead.
func DetectionRetypeRefusal(rt *Runtime, r ManifestReading, session string) error {
	return Die("%s: refusing to retype the %s launch line — herdr has no detection manifest for argv0 %q\n"+
		"  ADR 0013 §1: this path fires because herdr reports no agent in the session, and on a runtime it cannot name that is the steady state of a LIVE CLI, not evidence that one died — so the line would land in the running composer as a chat turn\n"+
		"  %s\n"+
		"  the whole grid, with what this box reads today: posse runtime check %s",
		session, rt.Name, r.Argv0, DetectionDoor(rt), rt.Name)
}

// DetectionDegraded is the interactive half of ADR 0015 §3's asymmetry
// (property 5). An operator's own `posse new` PROCEEDS, loudly.
//
// It is the escape hatch and there is no other: the fixtures an upstream
// detection filing needs are captured from an interactive session (ADR 0060
// D2), so a posse that refused `posse new` on an undetectable runtime would
// have walled off the only route that can end its own refusal. That is why
// `--allow-undetected` was rejected in 0060 D2 and stays rejected — the
// asymmetry already IS the flag, and it is one nobody can leave switched on.
func DetectionDegraded(rt *Runtime, r ManifestReading) string {
	return fmt.Sprintf("DEGRADED — herdr has no detection manifest for argv0 %q, so posse cannot read this session's state and will not dispatch to it; "+
		"an interactive launch proceeds because your own keyboard is what this session is for — and capturing its screens is how the manifest gets written (ADR 0013 §1, ADR 0015 §3). %s",
		r.Argv0, DetectionDoor(rt))
}
