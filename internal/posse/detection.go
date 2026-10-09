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
//
// AND THE FOURTH ANSWER, which is a DECLARATION and not herdr's (ADR 0061
// D1–D2, ranger-base-rx7l7). `unknown_agent` means "no manifest", and until
// ADR 0061 posse read that as "nothing can name this pane". It is false of a
// pane an outside authority labels through `herdr pane report-agent` —
// herdr's documented "integrate your own agent" route, and the route every
// herdr plugin takes. A runtime whose operator has measured that such an
// authority exists on THIS box declares `detection: reported`, and then the
// same `unknown_agent` is not the refusal: what the launch reads instead is
// whether this herdr can carry a report AT ALL (`pane report-agent --help`,
// absent on 0.8.2 and present on 0.9.0+, both on seats of this shop the same
// day — ranger-base-v1yrt).
//
// So the reading is now a pair — herdr's answer about the argv0, and what
// the runtime declared about who labels its panes — and the five states it
// resolves to are named as PREDICATES here rather than reconstructed by each
// caller from the pair. The declaration is read, never inferred: a
// `reported` runtime herdr DOES have a manifest for is an inert declaration
// and says so, and nothing in this file or anywhere below it keys on a
// runtime NAME (ADR 0017 §3).

import (
	"fmt"
	"time"
)

// detectionFilingDoc is where the upstream filing route is written down —
// how posse tracks a filing, what goes in one, and what the fixtures are
// for. Named as the door for a built-in posse carries NO filing for, because
// there the operator is not sending something that exists; they are deciding
// whether one has to be written at all (ADR 0060 D2).
//
// The repo-relative PATH, and it reaches a sentence only through publicDoc
// (publicdoc.go): this is a page to read and the reader of this door is on a
// box with no checkout, so the path alone was the ranger-base-mhv7j defect
// one layer out (ranger-base-x8bv0).
const detectionFilingDoc = "etc/herdr/agent-detection/README.md"

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

// ReportSurface is whether this herdr carries `pane report-agent` at all —
// the verb an outside authority labels a pane through, and the one thing a
// `detection: reported` launch reads before it spends anything.
//
// The zero value is UNKNOWN and it covers both ways of not knowing: the
// question did not arise (no `reported` declaration, so nothing was asked)
// and herdr could not be asked. Both mean the same thing to every caller —
// refuse nothing — which is why they share a value rather than being told
// apart for the sake of it. ReportSurfaceAbsent is reachable only by herdr
// having ANSWERED, the same discipline ManifestUnreadable keeps.
type ReportSurface int

const (
	ReportSurfaceUnknown ReportSurface = iota
	ReportSurfacePresent
	ReportSurfaceAbsent
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
	// Declared is the runtime's own `detection:` — DetectionHerdr or
	// DetectionReported, resolved (never ""), and Why is its
	// `detection_why:`. They are the half of this reading herdr did not
	// supply, and they are carried HERE rather than looked up per surface so
	// that every line printed about one reading quotes the same declaration.
	Declared string
	Why      string
	// Surface is `pane report-agent --help` — asked only where the
	// declaration makes it matter (Reported), so a detected runtime pays
	// nothing for this arm existing.
	Surface ReportSurface
}

// The five states, as predicates. Each caller reads the one it acts on and
// none of them rebuilds the pair, which is the same rule that put the three
// manifest answers behind Undetectable() in the first place.

// NoManifest is herdr's raw answer — it has no detection manifest for this
// argv0 — before any declaration is consulted. It is the fact, and the
// predicates below are what the launch stage makes of it.
//
// It is also the RELAUNCH arm's whole reading (ADR 0061 D3.4): "herdr reports
// no agent in this session" is not evidence a CLI died on a runtime herdr
// cannot name, and a reported runtime does not change that — there the
// silence is the reporter's, which is if anything a weaker signal.
func (r ManifestReading) NoManifest() bool { return r.State == ManifestUnknownAgent }

// Reported is the ADR 0061 arm: herdr has no manifest AND this runtime
// declares that somebody else labels its panes. The launch proceeds, and
// every state it will read afterwards is the reporter's word.
func (r ManifestReading) Reported() bool {
	return r.NoManifest() && r.Declared == DetectionReported
}

// Undetectable is the i3q6g refusal, unchanged in cause and narrowed in
// scope by exactly one clause: herdr has no manifest and NOTHING declares
// who else would. That is the state where a dispatched session is
// `agent_not_found` with no second authority to ask.
func (r ManifestReading) Undetectable() bool { return r.NoManifest() && !r.Reported() }

// NoReportSurface is the refusal ADR 0061 D2 property 1 ADDS: the runtime
// declares `reported`, and this herdr has no `pane report-agent` verb for
// anyone to report through. The declaration cannot be honoured by this
// binary, so the label can never arrive and the wait can only run out.
//
// Reachable only from ReportSurfaceAbsent, so a herdr that could not be
// asked refuses nothing here either.
func (r ManifestReading) NoReportSurface() bool {
	return r.Reported() && r.Surface == ReportSurfaceAbsent
}

// InertDeclaration is `detection: reported` on a runtime herdr DOES have a
// manifest for — the day herdr ships the kind. herdr's own detection wins,
// nothing changes, and the operator is told to drop the key rather than left
// with a declaration that reads as load-bearing (ADR 0061 D1).
func (r ManifestReading) InertDeclaration() bool {
	return r.State == ManifestKnown && r.Declared == DetectionReported
}

// Refuses is the ONE predicate a bead-carrying launch reads: the two states
// a launch must not proceed through, asked as one question so no launch path
// can grow an arm for one and miss the other.
func (r ManifestReading) Refuses() bool { return r.Undetectable() || r.NoReportSurface() }

// ReadDetection asks herdr the launch stage's first question about one
// runtime's argv0, and pairs the answer with what the runtime declared about
// who labels its panes. It is the function `posse runtime check`'s launch
// row, the preflight's detection gap and every launch refusal all read.
//
// It takes the RUNTIME and not just an argv0 since ADR 0061: the second half
// of the reading is `detection:`, and a signature that took only the exe
// would have made every call site look the declaration up for itself — five
// places deciding separately what `reported` means, which is the drift this
// file was created to end.
func ReadDetection(h Herdr, rt *Runtime) ManifestReading {
	exe := rt.Exe()
	r := ManifestReading{Argv0: exe, Declared: rt.DetectionMode(), Why: rt.DetectionWhy}
	if r.Declared != DetectionReported {
		r.Why = ""
	}
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
		return r.withReportSurface(h)
	}
	kinds := h.KnownAgentKinds()
	if kinds == nil {
		return r
	}
	r.ViaKinds, r.State = true, ManifestUnknownAgent
	if containsString(kinds, exe) {
		r.State = ManifestKnown
	}
	return r.withReportSurface(h)
}

// withReportSurface asks the second question, and asks it in exactly the one
// case where the answer can change a launch: the runtime declared `reported`
// and herdr has no manifest for the argv0.
//
// Not asked otherwise, and that is a property and not an optimisation. On a
// detected runtime the surface decides nothing — herdr's own manifest
// answers — so a call there would put a refusal's worth of weight on a verb
// no launch depends on. On an INERT declaration it decides nothing either:
// herdr's detection wins whether or not this binary can carry a report, and
// asking would let a 0.8.2 box turn a "drop this key" note into a refusal.
func (r ManifestReading) withReportSurface(h Herdr) ManifestReading {
	if !r.Reported() {
		return r
	}
	switch present, ok := h.HasReportAgent(); {
	case !ok:
		r.Surface = ReportSurfaceUnknown
	case present:
		r.Surface = ReportSurfacePresent
	default:
		r.Surface = ReportSurfaceAbsent
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
	if !r.Refuses() {
		return ""
	}
	if r.NoReportSurface() {
		return detectionSaw(r) + " and this runtime declares " + reportedDecl(r) +
			" — but this herdr has no `pane report-agent` verb at all, so no authority can label the pane and the launch can only wait out its startup_wait. " + DetectionReportDoor()
	}
	return detectionSaw(r) + " — a dispatched session is agent_not_found, so it cannot be addressed at all. " + DetectionDoor(rt)
}

// detectionSaw is herdr's own answer as a clause, and the one place the
// weaker wording lives: "does not recognize" where the compiled kind list
// answered and "has no manifest" where `agent explain` did, because the kind
// list cannot see an alias and an absence there is the weaker of the two
// claims.
func detectionSaw(r ManifestReading) string {
	if r.ViaKinds {
		return fmt.Sprintf("herdr does not recognize argv0 %q", r.Argv0)
	}
	return fmt.Sprintf("herdr has no detection manifest for argv0 %q", r.Argv0)
}

// reportedDecl renders the declaration with its why, which is the only form
// it is ever printed in: `detection: reported` on its own names a mechanism,
// and what a reader needs is the authority. The why is REQUIRED by the
// loader, so the bare fallback is unreachable through a loaded runtime and
// kept for a reading assembled by hand.
func reportedDecl(r ManifestReading) string {
	if r.Why == "" {
		return "detection: " + DetectionReported
	}
	return "detection: " + DetectionReported + " (" + r.Why + ")"
}

// DetectionReportedGapLine is `runtime check`'s detection line on a runtime
// that declares `reported` and has the surface for it — a NON-BLOCKING
// degrade and never a refusal (ADR 0061 D2 property 3).
//
// It is a degrade and not a clean row because what this runtime gives up is
// real and unreadable from anywhere else on the grid: every state posse acts
// on here is the REPORTER's word. herdr reads no screen for such a pane —
// `agent explain` answers agent_explain_unavailable — so there is no matched
// rule behind `working`, no chrome behind `idle`, and `blocked` is whatever
// the reporter happens to be able to see, which on herdr-bob today is
// nothing at all (ranger-base-p8afi). That last one is moot for a dispatched
// seat under --auto-approve and real for a hand-launched session, so it is
// said rather than ranked.
//
// Non-blocking is also what lets `posse runtime probe` run here, which is
// the point: the probe is the surface that measures a reported runtime's
// detection observable for real, and a blocking gap would have refused the
// one command that could answer it.
func DetectionReportedGapLine(r ManifestReading) string {
	line := detectionSaw(r) + ", and this runtime declares " + reportedDecl(r) +
		" — so the label comes from outside and every state posse reads here is the reporter's word: herdr matches no rule and reads no screen for such a pane (agent explain answers agent_explain_unavailable), and `blocked` is whatever the reporter can see, which may be nothing. Not a refusal — `posse runtime probe` measures what detection on this runtime actually reads (ADR 0061 D2)"
	if r.Surface == ReportSurfaceUnknown {
		line += ". Whether this herdr carries `pane report-agent` is UNKNOWN here, not no — it could not be asked"
	}
	return line
}

// DetectionInertGapLine is the other non-blocking line: a `reported`
// declaration on a runtime herdr detects ITSELF. herdr's own manifest wins,
// nothing about the launch changes, and the key is dead weight.
//
// Worth a line rather than silence because the day it appears is the day
// herdr ships the kind, and a declaration that has stopped mattering reads
// exactly like one that is load-bearing. The remedy is to drop the key, and
// this says so.
func DetectionInertGapLine(r ManifestReading) string {
	ver := ""
	if r.Version != "" {
		ver = " (detection manifest " + r.Version + ")"
	}
	return fmt.Sprintf("this runtime declares %s, but herdr detects argv0 %q itself%s — the declaration is INERT: herdr's own detection wins, so drop detection:/detection_why: and this runtime is read the way every detected one is (ADR 0061 D1)",
		reportedDecl(r), r.Argv0, ver)
}

// DetectionRow is the reading as the launch row's own clause in the grid.
func DetectionRow(r ManifestReading) string {
	switch {
	case r.State == ManifestUnreadable:
		return "herdr recognition UNKNOWN (herdr not on PATH, or its output moved)"
	case r.NoReportSurface():
		return fmt.Sprintf("herdr does NOT recognize argv0 %q, %s declared — and this herdr has NO `pane report-agent` verb, so nothing can label the pane", r.Argv0, reportedDecl(r))
	case r.Reported():
		row := fmt.Sprintf("herdr does NOT recognize argv0 %q — detection here is BY REPORT: %s, so every state is the reporter's word and herdr reads no screen", r.Argv0, reportedDecl(r))
		if r.Surface == ReportSurfaceUnknown {
			row += " (whether this herdr carries `pane report-agent` is UNKNOWN here)"
		}
		return row
	case r.Undetectable():
		return fmt.Sprintf("herdr does NOT recognize argv0 %q — no detection here, so work/settle are guesses", r.Argv0)
	case r.InertDeclaration():
		ver := ""
		if r.Version != "" {
			ver = " (detection manifest " + r.Version + ")"
		}
		return fmt.Sprintf("herdr recognizes argv0 %q%s — %s is INERT here, herdr detects it itself", r.Argv0, ver, reportedDecl(r))
	case r.Version == "":
		return fmt.Sprintf("herdr recognizes argv0 %q", r.Argv0)
	default:
		return fmt.Sprintf("herdr recognizes argv0 %q (detection manifest %s)", r.Argv0, r.Version)
	}
}

// DetectionReportDoor is where an operator goes to end the NO-SURFACE
// refusal, and it is deliberately not the manifest runbook: nothing is wrong
// with the profile and nothing needs authoring. What is missing is a herdr
// that has the verb.
//
// The version is named as the DOOR and never as the reading (HasReportAgent):
// posse asks the verb whether it exists, and tells the operator which release
// to be on. Both numbers are MEASURED on seats of this shop the same day
// (ranger-base-v1yrt), which is the whole reason this is a per-box fact.
func DetectionReportDoor() string {
	return "`pane report-agent` arrives in herdr 0.9.0 — check `herdr --version` and upgrade, or drop detection: reported and this runtime is read as undetected again (ADR 0061 D2)"
}

// DetectionReportedNote is the ONE line a launch prints when it proceeds onto
// a reported runtime (ADR 0061 D2 property 1, "Present → proceed, and print
// one line saying detection here is by report and whose").
//
// It is printed on the way IN, not on a failure, because that is the only
// moment the sentence is cheap: a session whose every later reading is the
// reporter's word should have said so once, before anybody reads a `working`
// off it and takes it for a matched rule.
func DetectionReportedNote(rt *Runtime, r ManifestReading) string {
	note := fmt.Sprintf("%s: detection here is BY REPORT — herdr has no manifest for argv0 %q, and detection_why: says who labels these panes: %s",
		rt.Name, r.Argv0, r.Why)
	if r.Surface == ReportSurfaceUnknown {
		return note + ". This herdr could not be asked whether it carries `pane report-agent`, so nothing was refused — UNKNOWN is never a no (ADR 0061 D2)"
	}
	return note + ". Every state posse reads on this session is the reporter's word, not a screen herdr matched (ADR 0061 D2)"
}

// DetectionDoor is where an operator goes to END this refusal, and it is the
// one clause that branches on the runtime rather than on the reading.
//
// A DECLARED runtime's door is the manifest runbook: the profile's author
// writes the toml, or aliases their CLI onto a manifest that already exists,
// and detection arrives on their own box.
//
// A BUILT-IN's is not. Its argv0 is posse's own and its detection is
// upstream's to ship, so the door is the upstream route and never a toml the
// operator authors (ADR 0060 D2). Branched on rt.Builtin and never on a
// runtime NAME: a name-keyed clause here would be the ADR 0017 §3 shadow
// predicate ADR 0060 rejected a whole adapter over, and would say the wrong
// thing about the second built-in that meets this.
//
// WHETHER POSSE CARRIES THE FILING IS THE SECOND BRANCH, and it is read off
// rt.DetectionFiling — a declaration — rather than rendered from the name.
// The clause used to spell `upstream-<rt.Name>.md` for every built-in, and
// the tree ships upstream-bob.md and nothing else, so on claude, codex or
// grok all FOUR surfaces that print this door handed the operator a path that
// is not in the tree — the launch refusal, the re-type refusal, `runtime
// check`'s detection gap and the interactive DEGRADED warn
// (ranger-base-ecchw). That is not a cosmetic wrong:
// this door is only ever read on a box whose herdr LACKS the manifest — an
// older herdr, a trimmed manifest set — which is the one moment the sentence
// is the whole remedy.
//
// THE VERB COMES BEFORE THE PATH, and that is the second escape, found on
// the work box the day a brew-installed posse first refused a bob seat
// (ranger-base-mhv7j, github issue #4). "posse ships the filing at <path>"
// was true of the REPO and false of every release: the tarball and the
// bottle carry the binary and two docs, so the operator was sent to a file
// that did not exist on his machine — on the one box this door is read on,
// with no checkout to fall back to, and `posse prompt` refusing the
// unlabelled pane meanwhile. The filing is embedded now (embed.go,
// internal/posse/filing.go) and the sentence leads with the command that
// prints it, because that command is true everywhere the binary is. The
// repo-relative path stays, in parentheses: it is what a persona standing in
// a checkout wants, and it is the declaration the pin holds.
//
// So a built-in with a filing names it, and a built-in without one says the
// true thing instead: nothing is carried, and an old herdr is the likely
// cause before anybody writes a manifest. The version check comes first
// deliberately — for the three built-ins upstream already detects, upgrading
// IS the door, and authoring a filing for them would be work nobody wants.
func DetectionDoor(rt *Runtime) string {
	if rt.Builtin {
		if rt.DetectionFiling == "" {
			return "detection for a built-in is upstream's to ship, not yours to author — this argv0 is posse's own, and posse carries no filing for " +
				rt.Name + ": check `herdr --version` first, because a current herdr may already carry this manifest and an old or trimmed one is the usual cause; if it does not, the filing has to be written and sent (" + publicDoc(detectionFilingDoc) + ", ADR 0060 D2)"
		}
		return "detection for a built-in is upstream's to ship, not yours to author: posse CARRIES the filing — `posse runtime filing " +
			rt.Name + "` prints it and `--out <dir>` writes it with its draft manifest and pane snapshots (" +
			rt.DetectionFiling + " in a checkout) — and the operator sends it (ADR 0060 D2)"
	}
	return "author a detection manifest, or alias " + rt.Exe() + " onto one that exists: " + publicDoc(detectionDocPath)
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
	// The SECOND cause, added by ADR 0061 D2 property 1, and refused with the
	// same shape as the first for the same reason: before anything is spent.
	// A runtime declaring `reported` on a herdr with no `pane report-agent`
	// verb is a launch whose observable cannot happen — nothing can label the
	// pane, so the wait can only run out, and it runs out having spent a
	// worktree, a workspace, a pane and the runtime's own first turn.
	//
	// The line names all three things the operator needs and nothing else:
	// the argv0 (what was asked about), the DECLARATION with its why (what
	// this box claims labels those panes, so a wrong declaration is visible
	// as a wrong declaration), and the door — which is a herdr upgrade and
	// NOT the manifest runbook, because nothing here is the profile's fault.
	if r.NoReportSurface() {
		return Die("%s launch refused: this runtime declares %s, and this herdr has no `pane report-agent` surface at all\n"+
			"  ADR 0061 D2: %s, so the label can only come from an outside authority — and the verb an authority reports THROUGH does not exist in this binary, so no label can ever arrive and the launch would spend a worktree, a workspace and a pane to wait out its startup_wait\n"+
			"  %s\n"+
			"  the whole grid, with what this box reads today: posse runtime check %s",
			rt.Name, reportedDecl(r), detectionSaw(r), DetectionReportDoor(), rt.Name)
	}
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
	// A `reported` runtime is on the REFUSING side of this arm, which is the
	// one place ADR 0061 leaves the i3q6g rule exactly as it found it (D3.4).
	// "herdr reports no agent in this session" is not evidence a CLI died
	// here either: it is the REPORTER's silence — the plugin's watcher not
	// started, or a pane it has not adopted yet — and a live CLI reads
	// identically. So the line would go into a running composer as a chat
	// turn, which is this gap's worst end whatever the declaration says.
	//
	// The trigger to revisit it is named in the ADR and is not this reading:
	// the first stranded reported pane whose FOREGROUND is the shell, which
	// D3.2's process reading can tell apart. That is a relaunch onto a shell
	// and correct — and it is a different fact from this one.
	if r.Reported() {
		return Die("%s: refusing to retype the %s launch line — this runtime declares %s, and no agent is reported in the session\n"+
			"  ADR 0061 D3.4: this path fires on \"herdr reports no agent here\", and on a reported runtime that is the AUTHORITY's silence, not the CLI's death — a live CLI whose reporter has not labelled it reads exactly the same — so the line would land in the running composer as a chat turn\n"+
			"  first remedy: herdr plugin list (is the authority installed, enabled and watching?) — not a larger startup_wait:\n"+
			"  the whole grid, with what this box reads today: posse runtime check %s",
			session, rt.Name, reportedDecl(r), rt.Name)
	}
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
	// The no-surface cause takes the same asymmetry and says a different
	// thing about what the operator is opening: the profile claims an
	// authority labels these panes and this herdr cannot carry a report at
	// all, so the session will read as undetected for its whole life. The
	// door is the upgrade, not the runbook.
	if r.NoReportSurface() {
		return fmt.Sprintf("DEGRADED — this runtime declares %s, and this herdr has no `pane report-agent` surface, so no authority can label these panes and posse will not dispatch here; "+
			"an interactive launch proceeds because your own keyboard is what this session is for (ADR 0061 D2, ADR 0015 §3). %s",
			reportedDecl(r), DetectionReportDoor())
	}
	return fmt.Sprintf("DEGRADED — herdr has no detection manifest for argv0 %q, so posse cannot read this session's state and will not dispatch to it; "+
		"an interactive launch proceeds because your own keyboard is what this session is for — and capturing its screens is how the manifest gets written (ADR 0013 §1, ADR 0015 §3). %s",
		r.Argv0, DetectionDoor(rt))
}

// NoAgentLine is the launch's own failure line when the startup wait runs out
// with herdr listing no agent for the workspace — dispatch's awaitTarget, the
// gate every delivery ladder passes through.
//
// ON A DETECTED RUNTIME it is the sentence it always was. Nothing above it
// refused, so herdr HAS a manifest for this argv0 and the pane genuinely
// produced no agent: the CLI did not start, or it started into something
// herdr matched no rule on. "check the session" is the right advice there —
// there is a session, and looking at it is what answers.
//
// ON A REPORTED RUNTIME that sentence is wrong in the way ADR 0061 D2
// property 2 names. The observable here is the LABEL APPEARING, and an
// absent label says nothing about the pane: the CLI may be up and working
// with nobody having reported it. So the line says what was declared, quotes
// the authority, and puts the first remedy first — `herdr plugin list`, is
// the authority installed, enabled and actually watching. It never says
// "check the session" and never offers a larger `startup_wait:` as the first
// move, because on this runtime a bigger wait is patience for a report that
// nothing is going to send (the plugin's watcher needs a hand start today,
// ranger-base-p8afi #3).
//
// It takes the READING and not the runtime, and that is what makes it the one
// copy of both sentences: a caller that could not load the profile passes the
// zero reading and gets the detected wording, rather than keeping a second
// copy of that wording for the case where it has nothing to quote.
func NoAgentLine(r ManifestReading, session string, wait time.Duration) string {
	if !r.Reported() {
		return fmt.Sprintf("no agent detected in %s after %s — check the session (posse peek %s)", session, wait, session)
	}
	return fmt.Sprintf("no agent reported in %s within %s — this runtime declares %s, so the observable was an outside authority LABELLING the pane, and none did. "+
		"That is not evidence about the CLI: it may be up and working with nobody having reported it. First remedy: `herdr plugin list` — is that authority installed, enabled, and watching? (ADR 0061 D2)",
		session, wait, reportedDecl(r))
}
