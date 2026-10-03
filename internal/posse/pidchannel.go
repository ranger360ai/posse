package posse

// The PID channel (ADR 0062 D3): WHICH placeholder of the template a
// persona line is rendered from delivers the PID — `{file}`, `{mode}`, or
// none.
//
// Before this file, "PID delivered" was a SENTENCE. `runtime check`'s launch
// row printed *PID delivered by the template* unconditionally, and the only
// PID-delivery check on any launch path was `PIDVoided`, which asks whether a
// FLAG on the rendered line makes the CLI discard the PID and never whether
// the template carries one at all (MEASURED by reading the call sites
// 2026-10-03, docs/notes.d/ranger-base-f1ytb.md §3). The two are siblings
// and the gap between them is a whole launch: ranger-base-5jjtn measured
// bob's `-p` dead and removed `{file}` from its template, after which a
// dispatched bob seat spent a worktree, a pane and a `startup_wait` on a
// session carrying every native rulebook and no persona — PIDVoided's exact
// harm, reached by a template that names no flag to refuse on — and the row
// still read *delivered*.
//
// ONE READING, THREE SURFACES (ADR 0013 §1's 9r33 shape: they cannot
// disagree, because there is one function to disagree with):
//
//   - `runtime check`'s launch row (runtimecheck.go launchRow) says which
//     placeholder delivers it, and the preflight carries a BLOCKING `pid`
//     gap when none does (runtimepreflight.go).
//   - `agent check` (pidcheck.go) — the `{model}` finding's sibling, one
//     placeholder over: a PID's own `command:` that names no PID channel.
//   - the launch, beside `PIDVoided` on both paths that render a persona line
//     (planLaunch, RelaunchAgent's retype arm) and on dispatch's own early
//     refusal above the claim (launchSession). DISPATCHED REFUSES,
//     INTERACTIVE WARNS (ADR 0013 §1 property 5, ADR 0015 §3).

import (
	"fmt"
	"strings"
)

// The two channels a persona-launch template can deliver the PID through.
//
// `{file}` is the PID's path, shell-quoted into whatever system-prompt or
// rules flag the CLI reads at launch (`--append-system-prompt "$(cat …)"`,
// `-c developer_instructions=…`, `--rules=…`).
//
// `{mode}` is ADR 0062 D1's, landed by ranger-base-qllt8: a runtime whose
// CLI has NO launch-time system flag at all, where the PID arrives as a
// posse-rendered custom mode in the session tree (personamode.go) and the
// line selects it. bob is the one built-in that carries it.
//
// It renders ONLY on a runtime declaring a persona-mode materializer, so a
// hand-written profile that spells it elsewhere delivers nothing — the
// placeholder vanishes with its space and the launch reads as no channel at
// all. PIDChannels below is where that is decided and the door is where it
// is said; `posse runtime check` is where an operator reads it.
const (
	PIDChannelFile = "{file}"
	PIDChannelMode = "{mode}"
)

// PIDChannelPlaceholders is the channel list in the order every reading
// names them — the row's phrase, the gap, the finding and the refusal all
// spell them from here, so "neither {file} nor {mode}" cannot drift from
// what the reading actually looked for.
var PIDChannelPlaceholders = []string{PIDChannelFile, PIDChannelMode}

// PIDChannels reports which PID channels a persona-launch TEMPLATE carries,
// in PIDChannelPlaceholders order; empty means this template delivers no PID
// at all.
//
// The argument is the TEMPLATE and never the rendered line, because by the
// time the line exists the placeholder is gone — `{file}` has become a
// quoted path indistinguishable from `{memory}`'s, and a template that
// carried nothing renders a perfectly good launch line with the persona
// silently missing. It is the inverse of ADR 0053 D1's `{model}` refusal,
// which CAN interrogate the rendered line, because a model id is a string
// the launch can look FOR — the PID's absence has no shape at all.
// AgentFile.LaunchTemplate names the template a launch renders from, and
// RenderCommandForModel renders that same expression, so the two cannot
// answer about different templates.
//
// A method on Runtime because WHICH channels a runtime can deliver through
// is a property of the runtime, and since ADR 0062 D1 landed the receiver is
// READ: `{mode}` renders to `--mode posse-<persona>` on a runtime that
// declares a persona-mode materializer and to NOTHING on one that does not
// (the `{skills}`/`{allow}` seam shape, ADR 0012 D4). So a template spelling
// `{mode}` on a runtime with no such seam carries a placeholder that
// vanishes with its space, and crediting it with a channel would be this
// file's own defect pointing the other way — a sentence with no reading
// behind it. That template reads as NO channel, and the door below says why
// rather than claiming the placeholder is absent.
//
// Keeping the reading on the Runtime is what stopped D1 from growing a
// second one somewhere else: this is the only place the seam decides what a
// template means.
func (rt *Runtime) PIDChannels(tmpl string) []string {
	var out []string
	for _, ph := range PIDChannelPlaceholders {
		if !strings.Contains(tmpl, ph) {
			continue
		}
		if ph == PIDChannelMode && rt.PersonaMode == nil {
			continue
		}
		out = append(out, ph)
	}
	return out
}

// pidChannelNames is "{file} nor {mode}" — the channel list as the refusals
// and the gap spell it, from the one list the reading looked in.
func pidChannelNames() string {
	return strings.Join(PIDChannelPlaceholders, " nor ")
}

// LaunchTemplate is the template a persona line is rendered FROM: the PID's
// own `command:` when the launch is on the PID's own runtime, else the
// runtime's own.
//
// One expression, because two surfaces need it — RenderCommandForModel
// renders it and the PID-channel reading asks about it — and a reading taken
// from a template no launch would render is a reading about nothing. It is
// also why the launch's refusal can name a FILE: the door is "add {file} to
// this template", which is useless without knowing which of the two it was.
func (ag *AgentFile) LaunchTemplate(rt *Runtime, ownRuntime string) string {
	if ag.ownsLaunchTemplate(rt, ownRuntime) {
		return ag.Command
	}
	return rt.Command
}

// ownsLaunchTemplate is that choice as a predicate, so the template and the
// place it is DECLARED cannot answer about different files — the drift this
// whole file exists to stop, one scope down.
func (ag *AgentFile) ownsLaunchTemplate(rt *Runtime, ownRuntime string) bool {
	return rt.Name == ownRuntime && ag.Command != ""
}

// LaunchTemplateWhere names where that template is DECLARED, for a door an
// operator can walk through: a PID's own file, the runtime yaml that set
// `command:`, or the built-in.
func (ag *AgentFile) LaunchTemplateWhere(rt *Runtime, ownRuntime string) string {
	if ag.ownsLaunchTemplate(rt, ownRuntime) {
		return AbbrevHome(ag.Path) + "'s own command:"
	}
	return runtimeTemplateWhere(rt)
}

// runtimeTemplateWhere is LaunchTemplateWhere's runtime half, and the whole
// of it for `runtime check`, which is asked about a PROFILE and has no PID
// in front of it.
func runtimeTemplateWhere(rt *Runtime) string {
	if rt.Path != "" && YamlGet(rt.Path, "command") != "" {
		return AbbrevHome(rt.Path) + "'s command:"
	}
	if rt.Builtin {
		return "the built-in " + rt.Name + " template (runtime.go)"
	}
	return "runtimes/" + rt.Name + ".yaml's command:"
}

// PIDChannelDoor is the remedy. It names the template the reading was taken
// from rather than "the template" — the ranger-base-ecchw rule, where four
// surfaces printed a door that was not in the tree.
//
// AN EITHER/OR AND NEVER AN IMPERATIVE, which is the whole of this
// function's design. "add {file}" is the right door for a CLI that reads a
// system-prompt or rules flag at launch and is ACTIVE HARM for one that does
// not: the runtime that reaches this sentence first on this box is the one
// whose own template says DO NOT put `-p` back, because the flag parses, is
// accepted, is silently ignored, and makes a reader checking "does this
// deliver a PID?" answer yes (ranger-base-5jjtn, runtime.go's BobCommand
// block). A door that sent that reader to re-add the measured-dead flag
// would be this bead's own defect class, one layer down.
//
// SO THE RUNTIME DECIDES WHICH DOOR IS OFFERED, and since ADR 0062 D1 it can:
// a runtime declaring a persona-mode materializer is one whose CLI has been
// MEASURED to have no launch-time system flag, so `{mode}` is its only door
// and `{file}` is the wrong one. Everything else gets `{file}`, and is told
// that `{mode}` is not a door it has — because a placeholder no seam renders
// is a literal on the launch line, which is the loudest way to be wrong and
// still the wrong advice to give.
func PIDChannelDoor(rt *Runtime, where string) string {
	if rt.PersonaMode != nil {
		return "the door is " + PIDChannelMode + " in " + where +
			" — this CLI has no launch-time system-prompt or rules flag (MEASURED; see its own template comment in runtime.go), so the PID arrives as the custom mode posse renders into the session tree and the line selects it with that placeholder (ADR 0062 D1). " +
			PIDChannelFile + " is NOT the door here: there is no flag to put it in, and a `-p` that parses and is ignored is what ranger-base-5jjtn cost"
	}
	return "the door is " + PIDChannelFile + " in " + where +
		" — the PID's path, shell-quoted into a launch-time system-prompt or rules flag this CLI really reads. " +
		PIDChannelMode + " is NOT a door on " + rt.Name +
		": it renders only on a runtime declaring a persona-mode materializer (ADR 0062 D1), so a template that spells it here delivers nothing and the placeholder reaches the shell as a literal"
}

// PIDChannelRow is the launch row's PID clause (surface 1).
func PIDChannelRow(ch []string) string {
	if len(ch) == 0 {
		return "NO PID channel — the template carries neither " + pidChannelNames() +
			", so a bead-carrying launch REFUSES and an interactive one warns (ADR 0062 D3)"
	}
	return "PID delivered by " + strings.Join(ch, " and ") + " in the template"
}

// PIDChannelGapName is what to say when you say "gap" — spelled once,
// because the probe exempts this gap BY NAME (runtimeprobe.go) and a gap
// whose name drifted from its exemption is a probe that refuses on the one
// reading it exists to measure.
const PIDChannelGapName = "pid"

// PIDChannelGapLine is the preflight's blocking `pid` gap (surface 1's other
// half). Blocking exactly when the launcher refuses on it, the
// ranger-base-9r33 rule: this runtime cannot take dispatched work until its
// template has a channel, which is what `blocking` means here — and it is
// what makes `runtime check`'s exit code an onboarding gate rather than a
// screen to read and nod at.
func PIDChannelGapLine(rt *Runtime) string {
	where := runtimeTemplateWhere(rt)
	return fmt.Sprintf("%s has neither %s, so this template delivers no PID: a dispatched session here would open carrying every native rulebook and no persona at all, and the launch refuses by name (ADR 0062 D3). %s",
		where, pidChannelNames(), PIDChannelDoor(rt, where))
}

// PIDChannelFinding is `agent check`'s finding (surface 2) — the `{model}`
// finding's sibling one placeholder over.
//
// Its first door is the sibling findings' ("drop command: for the built-in
// template"), and it is the one door here that is NOT unconditional: what
// dropping `command:` hands the launch is the runtime's own template, which
// may carry no channel either — which is precisely the state the runtime
// this reading first fires on is in. So it says where to go and ask, rather
// than promising an answer this reader cannot see from where they stand.
func PIDChannelFinding(rt *Runtime, where string) string {
	return "command: has no PID channel — neither " + pidChannelNames() +
		", so this PID's own runtime would launch carrying every native rulebook and no persona at all (ADR 0062 D3). " +
		"Dropping command: renders " + rt.Name + "'s own template instead, and `posse runtime check " + rt.Name +
		"` says whether THAT delivers a PID; to keep this one, " + PIDChannelDoor(rt, where)
}

// PIDChannelRefusal is the launch refusal (surface 3), shared by dispatch's
// early arm above the claim and by planLaunch's backstop inside the create,
// so the two cannot say different things about one template.
//
// A PLAIN error, like DangerRefusal and DetectionRefusal: fireLoop's busy-key
// split reads it on the default arm and benches the SLOT, not the bead. Every
// bead routed to this persona on this runtime renders the same template and
// meets the same absence, so claiming them one at a time to refuse them one
// at a time is the sterilised queue ADR 0013 §2 named once.
//
// It never says "check the session": there is none, and that is the whole
// point of refusing here.
func PIDChannelRefusal(agent string, rt *Runtime, where string) error {
	return Die("%s: the rendered %s launch line delivers no PID — %s carries neither %s, so no PID text reaches the CLI\n"+
		"  ADR 0062 D3: the session would open carrying every native rulebook and no persona at all — PIDVoided's harm (ranger-base-64qx), reached by a template that names no flag to refuse on\n"+
		"  %s\n"+
		"  the whole grid, with what this box reads today: posse runtime check %s",
		agent, rt.Name, where, pidChannelNames(), PIDChannelDoor(rt, where), rt.Name)
}

// PIDChannelRetypeRefusal is the re-type arm's refusal — the third path that
// renders a persona line, and the one that types into a pane that already
// exists (ranger-base-enmu2's shape).
//
// Unconditional, because that path has one caller and it is the unattended
// one (dispatch.launchSession): there is no interactive arm to hold ADR 0015
// §3's asymmetry open here. Reachable even though the create was refused —
// the PID and the runtime file are both re-read from disk, so a `{file}`
// edited out of either since the session opened arrives here, which is the
// same reason PIDVoided is asked twice.
func PIDChannelRetypeRefusal(agent string, rt *Runtime, session, where string) error {
	return Die("%s: refusing to retype the %s launch line in %s — %s carries neither %s, so the revived session would carry every native rulebook and no persona at all (ADR 0062 D3)\n"+
		"  %s",
		agent, rt.Name, session, where, pidChannelNames(), PIDChannelDoor(rt, where))
}

// PIDChannelDegraded is the interactive warn — ADR 0013 §1 property 5 and
// ADR 0015 §3's asymmetry, for the two reasons the ADR states: the operator
// at the keyboard can paste the PID into the session they just opened, and
// `posse runtime probe` renders a persona line and is the surface the
// instance side MEASURES the channel with. A probe that refused on this
// could never measure the channel that lifts the refusal.
func PIDChannelDegraded(rt *Runtime, where string) string {
	return fmt.Sprintf("DEGRADED — the rendered %s launch line delivers no PID: %s carries neither %s, so this session opens carrying every native rulebook and no persona at all; "+
		"an interactive launch proceeds because your own keyboard can paste the PID, and because the probe renders a persona line and is how the channel that lifts this gets measured (ADR 0062 D3, ADR 0015 §3). %s",
		rt.Name, where, pidChannelNames(), PIDChannelDoor(rt, where))
}
