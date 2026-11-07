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
	"strconv"
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
// a runtime that writes none is unreadable from its first keystroke — and one
// of the two runtimes refused today is `prompt: argv` (grok, until Amendment
// 3's config line lands), where the work prompt rides the launch line. A
// launch-then-wait refusal for that one arrives after a claim, a worktree and
// a live billed turn, which is what ADR 0013 §1 rule 3 ("before anything is
// spent") forbids. So the fact is read off the runtime table before the
// claim, and the wait after the launch is only its confirmation (ADR 0076 D2,
// awaitFirstReport).
//
// WHAT THE YES MEANS WIDENED WITH D9 and the predicate did not move: a record
// whose owner is the runtime, written natively (claude) or by posse's route
// on the runtime's own events (codex, ADR 0076 Amendment 2 D6/D7). The second
// kind is a line posse has to type and a version it was measured on, which is
// RexVersionDrift below — the admission's second reading, asked at the same
// two call sites and before the same claim.
func RexAdmits(rt *Runtime) bool { return rt != nil && rt.ReportsOnRex }

// RexVersionDrift is ADR 0076 D9's version pin, read as the second half of
// admission: the CLI line this box answers when it is not the one the Rex
// route was MEASURED on. ok=false is a refusal; the string is what was read,
// and "" there means the CLI would not answer `--version` at all.
//
// WHY A ROUTE NEEDS A VERSION AND A NATIVE REPORT DOES NOT. claude's record
// is claude's own output: it can regress, and a regression shows up as the
// first-report wait missing (D2), with nothing typed and nothing spent.
// codex's record exists because posse typed two `-c` values it derived from an
// `[experimental]` RPC and from session-flag hook semantics, neither promised
// by anyone. If those move, the most likely shape is not an error — it is a
// line codex accepts, a launch that completes, a seat sitting on the startup
// review dialog, and `prompt: argv`'s HOLD keeping the claim over a turn
// nobody can see. So the version is checked where D1 is checked: before the
// claim, off a declaration, with no launch spent finding out (D9).
//
// UNREADABLE IS REFUSED TOO, which is D9's sentence read the way ADR 0013 §1
// reads unknown — never as no. The pinned claim is "this route was measured
// on THIS version", and a CLI that will not say which version it is leaves
// that claim unsupported; the cost of refusing is one launch that would have
// opened on a codex posse cannot identify, and the cost of admitting is the
// stall above. A codex that cannot answer `--version` is also not going to
// hand the seat a record inside its 45 s `startup_wait`.
//
// THE READER IS skillsDriftVersion, parity.go's seam, on purpose: one
// `--version` reader, one per-process memo (ProbeCLIVersion), and one place a
// hermetic test can stand in for the box's own CLIs. The memo is why this
// costs the dispatch loop one exec per process and not one per pass — and
// why a codex upgraded under a running `posse dispatch --watch` is not
// noticed until the next process, which is the same property `posse runtime
// drift` has and the reason that command exists.
//
// THE COMPARE IS skillsVersionRelease's (ADR 0007 §6c): a trailing bracketed
// field is box state, not release identity. codex prints no such field, so
// for it this is the raw line either way — the rule is shared rather than
// re-decided.
func RexVersionDrift(rt *Runtime) (string, bool) {
	if rt == nil || rt.RexVerified == "" {
		return "", true
	}
	now := skillsDriftVersion(rt.Exe())
	if now != "" && skillsVersionRelease(now) == skillsVersionRelease(rt.RexVerified) {
		return now, true
	}
	return now, false
}

// rexVersionLines are the sentences the version refusal and its interactive
// warning share, so the dispatched and at-the-keyboard arms cannot drift into
// describing one gap differently — rexRefusalLines' rule, one reading over.
func rexVersionLines(rt *Runtime, got string) (string, string) {
	cause := rt.Name + " reports " + strconv.Quote(got) + " and this instance's Rex route was measured on " + strconv.Quote(rt.RexVerified)
	if got == "" {
		cause = rt.Name + " would not answer `--version`, and this instance's Rex route was measured on " + strconv.Quote(rt.RexVerified)
	}
	return cause, "ADR 0076 D9: " + rt.Name + "'s record on Rex is not its own output — posse types its reporter onto the launch line as session-flag hooks and trusts them with the hash " +
		rt.Name + " itself reported, which rests on an `[experimental]` RPC and on session-flag hook semantics nobody promises. A version this route was not measured on most likely fails as a line " +
		rt.Name + " ACCEPTS: the launch completes, the seat sits on the startup review dialog, no record arrives, and `" + PromptArgv +
		"` delivery means the work prompt is already with the CLI (ADR 0076 D2's HOLD keeps the claim over a turn nobody can read)"
}

// rexVersionDoor is the re-measurement, named in both arms: two probe runs
// and no model turn, which is what D9 priced the pin at.
func rexVersionDoor(rt *Runtime) string {
	return "re-measure the route and move `rex_verified:` on the " + rt.Name +
		" built-in: docs/adr/0076-trust-ask.probe.sh (arm `ask`, the whole route; arm `state` is the cheap one — codex's own hooks/list, no Rex block and no prompt) and docs/adr/0076-reporter.probe.sh. Both are about two minutes and spend no model turn (ADR 0076 D9). Until then " +
		rexHerdrDoor(rt)
}

// RexVersionRefusal is the dispatched arm of the version pin, and a plain
// error for RexRuntimeRefusal's reason: fireLoop's busy-key split lands it on
// the persona/runtime arm and benches the slot, because every bead routed to
// this persona on this runtime meets the same unmeasured CLI, and claiming
// them one at a time to refuse them one at a time is the sterilised queue ADR
// 0013 §2 named once.
func RexVersionRefusal(rt *Runtime, got string) error {
	cause, why := rexVersionLines(rt, got)
	return Die("%s launch refused: %s, and this instance runs `%s: %s`\n"+
		"  %s\n"+
		"  ADR 0076 D9: refused HERE — before the claim, before a worktree and before a Rex session exists — because the evidence a try-and-see would wait for arrives only after the prompt has been delivered, and on a `%s` runtime that is a billed turn nobody observes\n"+
		"  %s\n",
		rt.Name, cause, MultiplexerKey, MuxRex, why, rt.PromptMode(), rexVersionDoor(rt))
}

// RexVersionDegraded is the INTERACTIVE arm, with RexRuntimeDegraded's
// asymmetry and its reason: the operator is at the keyboard, and launching a
// codex this route has not been measured on is how the next measurement gets
// taken.
func RexVersionDegraded(rt *Runtime, got string) string {
	cause, why := rexVersionLines(rt, got)
	return "DEGRADED — " + cause + ", and this instance runs `" + MultiplexerKey + ": " + MuxRex +
		"`; an interactive launch proceeds because you are at the keyboard and can read the screen posse cannot. " + why +
		". If it opens on a review dialog, that is this pin firing late. " + rexVersionDoor(rt)
}

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
//   - argv (codex, since Amendment 2's D9 admitted it): the prompt rode the
//     launch line and is already with the CLI, so a miss is a HOLD and not a
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
// refuses nothing.
//
// REACHABLE SINCE ranger-base-8gh58, which is the admission D2 wrote it for:
// codex is `prompt: argv` and reports through posse's own hooks (D6/D7), so
// the shapes that land here are a declaration codex would not run, a trust
// line that missed, and a CLI that died at its splash. The three refusals
// ahead of it — the version pin (D9), the declaration render and the trust
// ask (D7) — exist so that this arm is the LAST thing between posse and a
// turn it cannot see, and not the first.
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
