package posse

// Which occupant of the multiplexer slot this instance runs (ADR 0076 §6),
// and which runtimes that occupant can be asked about (§3, amended D1).
//
// ONE KEY, READ IN ONE PLACE. `multiplexer: rex` is per INSTANCE and not per
// PID: ADR 0076's "Alternatives rejected" priced the per-PID key and left it
// out because no persona needs a mixed instance today, and because the key
// can be narrowed later without breaking a record already on disk. An absent
// key is herdr, so every home on this box reads unchanged and herdr's code
// paths do not move at all — which is §6's "this is not a fork" made
// mechanical.
//
// A MALFORMED VALUE READS AS HERDR AND REFUSES THE LAUNCH, which is
// App.Instance's rule and is here for App.Instance's reason: a listing has to
// keep working, and the launch is the one place where guessing costs
// something. The two halves are Multiplexer (validating, for the launch) and
// MultiplexerName (quiet, for every read path).

import (
	"strings"
	"time"
)

// MultiplexerKey is the instance-config key that names the occupant.
const MultiplexerKey = "multiplexer"

// ValidMultiplexer is the closed set: the two occupants that exist. A third
// name is a refusal and never a fallback, because the fallback is herdr and
// an instance that believed it was on Rex would launch a seat herdr's way
// and record it as Rex's.
func ValidMultiplexer(v string) bool { return v == MuxHerdr || v == MuxRex }

// Multiplexer is the validated occupant name for this instance: MuxHerdr
// (the default and the empty value) or MuxRex.
func (a *App) Multiplexer() (string, error) {
	v := strings.TrimSpace(a.CfgGet(MultiplexerKey, ""))
	if v == "" {
		return MuxHerdr, nil
	}
	if !ValidMultiplexer(v) {
		return "", Die("bad %s '%s' in %s (want %s or %s — ADR 0076 §6) — the key names which multiplexer holds this instance's seats, and an unrecognised name cannot be read as either one",
			MultiplexerKey, v, a.ConfigPath, MuxHerdr, MuxRex)
	}
	return v, nil
}

// MultiplexerName is Multiplexer for the read paths — the `posse list`
// header, the `posse gates` row, and NewHerdrBackend's choice of occupant.
// A malformed value reads as herdr here, exactly as a malformed `instance:`
// reads as unset: the launch is where it is refused, and that refusal is
// what keeps a home from ever CREATING a seat under a name this ignores.
func (a *App) MultiplexerName() string {
	name, err := a.Multiplexer()
	if err != nil {
		return MuxHerdr
	}
	return name
}

// MultiplexerLine is the one sentence `posse list` and `posse gates` print
// about which multiplexer this instance runs. Rendered in one place so the
// two surfaces cannot drift into saying different things about one key.
func (a *App) MultiplexerLine() string {
	name := a.MultiplexerName()
	if name == MuxRex {
		return "multiplexer: rex — seats are Rex sessions and agent state is the runtime's own program_status report (ADR 0076 §3); only a runtime that reports there may launch"
	}
	return "multiplexer: herdr — seats are herdr workspaces and agent state is herdr's own detection (the default; set `" + MultiplexerKey + ": " + MuxRex + "` to run Rex, ADR 0076 §6)"
}

// ─── which runtimes the occupant can be asked about (ADR 0076 §3, D1) ────────

// RexAdmits is ADR 0076's amendment D1 read as a predicate: may a seat of
// this runtime be opened on a Rex instance at all?
//
// IT IS A DECLARATION AND NOT A TRY-AND-SEE, which is the whole of D1. On
// Rex posse reads the runtime's OWN OSC 7501 record and invents no state, so
// a runtime that writes none is unreadable from its first keystroke — and
// the two runtimes that are refused today are both `prompt: argv`, where the
// work prompt rides the launch line. A launch-then-wait refusal for one of
// those arrives after a claim, a worktree and a live billed turn, which is
// what ADR 0013 §1 rule 3 ("before anything is spent") forbids. So the fact
// is read off the runtime table before the claim, and the wait after the
// launch is only its confirmation (ADR 0076 D2, awaitFirstReport).
func RexAdmits(rt *Runtime) bool { return rt != nil && rt.ReportsOnRex }

// rexRefusalLines are the two sentences shared by the refusal and the
// interactive warning, so the dispatched and the at-the-keyboard arms cannot
// drift into describing the same gap differently.
func rexRefusalLines(rt *Runtime) (string, string) {
	return rt.Name + " does not report its own state on Rex",
		"ADR 0076 §3: on Rex posse reads the runtime's OWN program_status record (OSC 7501, the runtime writes it) and there are no screen rules and no posse-written hooks to fall back on, so every state of a " +
			rt.Name + " seat here — promptable, working, settled, the turn's own outcome — would be unknown (ADR 0013 §1: unknown is never read as no)"
}

// rexHerdrDoor is the door every Rex refusal and warning ends with: herdr is
// how this runtime runs, and the key is per instance so nothing else has to
// move. ADR 0076 §3 names it in those words ("a message that names herdr as
// the way to run that runtime").
func rexHerdrDoor(rt *Runtime) string {
	return "the way to run " + rt.Name + " is herdr, where its state is herdr's own detection: drop `" +
		MultiplexerKey + ": " + MuxRex + "` from this instance's config, or route this persona to an instance that runs herdr — the key is per INSTANCE (ADR 0076 §6), so herdr keeps running " +
		rt.Name + " unchanged"
}

// RexRuntimeRefusal is the launch refusal itself (ADR 0076 D1), shaped like
// DangerRefusal and DetectionRefusal and for the same reason: the dispatch
// loop and planLaunch must not drift into saying different things about one
// gap. It names the CAUSE and the DOOR and never the session — there is no
// session to check, and that is the point of refusing here.
//
// A plain error, so fireLoop's busy-key split lands it on the
// persona/runtime arm and benches the slot: every bead routed to this
// persona on this runtime meets the same missing report, and claiming them
// one at a time to refuse them one at a time is the sterilised queue ADR
// 0013 §2 named once.
func RexRuntimeRefusal(rt *Runtime) error {
	cause, why := rexRefusalLines(rt)
	return Die("%s launch refused: %s, and this instance runs `%s: %s`\n"+
		"  %s\n"+
		"  ADR 0076 D1: refused HERE — before the claim, before a worktree and before a Rex session exists — because the evidence a try-and-see would wait for arrives only after the prompt has been delivered, and on a `%s` runtime that is a billed turn nobody observes\n"+
		"  %s\n",
		rt.Name, cause, MultiplexerKey, MuxRex, why, rt.PromptMode(), rexHerdrDoor(rt))
}

// RexRuntimeDegraded is the INTERACTIVE arm (ADR 0013 §1 rule 5, ADR 0076
// D1's last line): `posse new` warns and proceeds. The operator is at the
// keyboard, which is the whole asymmetry — they can read the screen posse
// cannot, and a posse that refused here would wall off the only way to look
// at a runtime on Rex and find out what it does report.
func RexRuntimeDegraded(rt *Runtime) string {
	cause, why := rexRefusalLines(rt)
	return "DEGRADED — " + cause + ", and this instance runs `" + MultiplexerKey + ": " + MuxRex +
		"`; an interactive launch proceeds because you are at the keyboard and can read the screen posse cannot. " + why +
		". `posse list` and every dispatch reading of this session will say unknown. " + rexHerdrDoor(rt)
}

// ─── the launch-stage confirmation (ADR 0076 §3, amendment D2) ───────────────
//
// D1 above decided ADMISSION from a declaration, before anything is spent.
// What follows is its CONFIRMATION: the launch typed the runtime's line, and
// a bounded wait asks whether a record actually arrived. The two are not
// redundant — the declaration is a fact about the BINARY that was measured
// once, and this is a fact about THIS launch, which a version bump, a
// config the runtime read, or a CLI that died at its splash can each falsify.
//
// WHAT A MISS COSTS IS KEYED ON `prompt:`, which is D2's whole content:
//
//   - typed (claude): the work prompt has NOT been delivered, so a miss is a
//     refusal with nothing typed. The seat is unreadable, and typing into an
//     unreadable seat is the blind prompt ADR 0013 §1 refuses.
//   - argv (none admitted on Rex today, D3/D4): the prompt rode the launch
//     line and is already with the CLI, so a miss is a HOLD and not a
//     refusal (ADR 0066 D1, herdr's awaitDelivered consequence) — the claim
//     is kept and the bead is not judged this pass. Refusing would judge a
//     turn that may well be running.
//
// Both arms print the same sentence about what was seen; only the verdict
// differs.

// rexNoReportSaw is what the bound actually ran out on — the difference
// between a block that never reported at all and one whose only record was
// written by its own shell or by a process that has since exited. An
// operator reading either line needs to know which, because they have
// different causes and different next steps.
func rexNoReportSaw(read MuxReading) string {
	if read.Evidence == "" {
		return "no program_status record"
	}
	return read.Evidence
}

// RexNoReportRefusal is the typed arm's refusal: the record never came, so
// nothing was typed and nothing will be.
//
// It names the session rather than hiding it, which is the one place this
// line departs from DetectionRefusal ("it never says check the session:
// there is none to check"). Here there IS one — the seat was created and the
// runtime's own line was typed into it, because that typing is what the
// report is evidence of — and it is left running on purpose: it is the only
// copy of why no record arrived, and a launch that deleted its own evidence
// would make this refusal unexplainable.
func RexNoReportRefusal(rt *Runtime, session string, within time.Duration, read MuxReading) error {
	return Die("%s launch refused in %s: no program_status record from the runtime within %s (saw: %s)\n"+
		"  ADR 0076 §3: on Rex the RUNTIME reports its own state and posse invents none, so a seat that has not reported cannot be read at all — the prompt, the settle and the turn's own outcome would each be a guess (ADR 0013 §1)\n"+
		"  NOTHING WAS TYPED into it: this runtime is `%s`, so the work prompt was still posse's to deliver and the bead is untouched (ADR 0076 D2)\n"+
		"  the session is left running, because it is the only evidence of why no record came: posse peek %s, then posse kill %s\n"+
		"  %s\n",
		rt.Name, session, within, rexNoReportSaw(read), rt.PromptMode(), session, session, rexHerdrDoor(rt))
}

// RexNoReportHold is the argv arm's line (ADR 0076 D2, ADR 0066 D1): the
// prompt is already with the CLI, so this says what is not known and
// refuses nothing. Unreachable today — D1 admits no `prompt: argv` runtime
// on Rex — and written anyway, because the arm that decides what a miss
// costs is the arm a future admission lands on.
func RexNoReportHold(rt *Runtime, session string, within time.Duration, read MuxReading) string {
	return "HOLD — no program_status record from " + rt.Name + " in " + session + " within " +
		within.String() + " (saw: " + rexNoReportSaw(read) + "). This runtime is `" + PromptArgv +
		"`, so the work prompt rode the launch line and is already with the CLI: the claim is KEPT and this pass judges nothing (ADR 0076 D2, ADR 0066 D1). A turn may be running unobserved — posse peek " + session
}

// RexNoReportDegraded is the INTERACTIVE arm: `posse new` warns and
// proceeds, for ADR 0013 §1 rule 5's reason and for one more that is
// specific to this reading — the operator typed a session to LOOK at, and
// looking at it is how a runtime whose report nobody has measured gets
// measured.
func RexNoReportDegraded(rt *Runtime, session string, within time.Duration, read MuxReading) string {
	return "DEGRADED — no program_status record from " + rt.Name + " in " + session + " within " +
		within.String() + " (saw: " + rexNoReportSaw(read) +
		"). On Rex that is the whole of posse's reading, so this session's state will show as unknown everywhere and dispatch could not prompt it; an interactive launch proceeds because you are at the keyboard (ADR 0013 §1 rule 5). posse peek " + session
}
