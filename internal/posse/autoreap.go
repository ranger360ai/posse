package posse

// The end-of-pass auto-reap (rangerhq-us8): a per-bead session (ADR 0013 §4
// Dial F, <persona>-<repobase>-<bead>) whose bead the store of record now
// calls CLOSED, and in which nobody is working any more, is killed and
// landed exactly as `posse kill` would — one line said about it either way.
// "Nobody is working any more" is settledForReap below: an agent herdr calls
// idle or done, or — since ranger-base-kftx — no agent left in there at all,
// past the grace inside which that is indistinguishable from one still
// starting.
// Dial F gives every dispatched bead its own session and never reaps it
// itself (dispatch.go's own doc: "left idle for the operator or --watch to
// reap"), so without this an instance accumulates one dead pane per closed
// bead forever.
//
// MEASURED (rangerhq-us8 comments): ~50 sessions reaped by hand over
// two days, all of them this exact predicate — bead closed, agent idle. Every
// one that had a bead pointer reported no commits of its own: the leak is
// session accumulation, not stranded work.
//
// Three guards, one per near-miss already on record:
//
//   - CREW (ADR 0008): the operator's own conversation does not exist as far
//     as any harness sweep is concerned.
//   - the SLOT, not just the bead: `NoteBead` stamps `bead:` onto the
//     pre-Dial-F persona session too, when a bead resumes into it (ADR 0004
//     §2) — so a bead pointer alone does not mean "this name is disposable".
//     Only a session whose own name is not the bare persona/repo slot is
//     Dial F's to reap; the slot is what the next resume rejoins.
//   - PROMPTED RECENTLY: a settle read moments after a fresh prompt is the
//     same race PromptGrace exists for in the fire loop — a bead that closed
//     moments ago is safer left for the next sweep's read than reaped on
//     this one's.
//
// That last guard used to key on `justPrompted`, the set of sessions THIS
// PASS had fired at, and ADR 0028 §3 re-keys it onto `promptedRecently` —
// the same question asked of the session's own run record (ADR 0011 §3),
// which is persisted and cross-process. Two reasons, and the second is the
// one the ADR is about:
//
//   - The in-memory set could only see prompts this process sent. A session
//     the cockpit's `d` or a second launcher prompted a second ago was, to
//     this sweep, a session nobody had touched — the same blindness the run
//     record was introduced to end (rangerhq-tzdf's remaining half).
//   - It was denominated in PASSES. Under ADR 0028 §1 a pass is the whole
//     life of a long-lived Run, so a set carried "for the pass" would grow
//     without bound and guard sessions prompted hours ago; the grace this
//     was always reaching for is PromptGrace, and now it says so.
//
// It is a WIDER guard in the direction that matters and a narrower one where
// narrowing is correct: it covers prompts this process never sent, and it
// stops covering a session prompted 75 minutes ago whose bead the store now
// calls closed and whose agent herdr calls idle — which is precisely a
// session to reap.
//
// And it reads the bead fresh, at reap time, never from the pass's own
// gathered results: `--resume` can close a bead between one pass's gather
// and this epilogue, and a cached status here would disagree with the store
// it is supposed to defer to (ADR 0011).
import (
	"fmt"
	"io"
	"strings"
	"time"
)

// AutoReap is config `auto_reap:` (default true — the reaper runs). `false`
// is today's behaviour, before this bead: nothing kills a finished session
// but the operator or `--watch`'s own judgment.
func (a *App) AutoReap() bool {
	return a.CfgGet("auto_reap", "true") != "false"
}

// autoReapPass sweeps closed-and-idle sessions. Run calls it twice: once at
// pass start, before routing, and once as its own epilogue (ranger-base-v674
// — a real pass with real beads gathers for 15m-4h, and every --watch
// instance on record has died somewhere inside that window, so the epilogue
// alone left the sweep starved). It reads every bead fresh (see below), so
// either call site is equally safe — and since ADR 0028 §3 they are also
// the same call: the prompt guard is a question about the SESSION now, not
// an argument about what this pass did, so neither site has to tell the
// other what it fired. A read failure is this sweep's own to swallow: a pass
// that dispatched real work does not fail because a reap sweep could not
// list sessions.
//
// `when` is the one thing the two sites do not share, and it gates exactly
// one arm: a stampless session may be a seat this pass is about to reuse, and
// only a sweep past routing can know it is not (ranger-base-f6lk, below).
func (d *Dispatcher) autoReapPass(when reapWhen) {
	// The refusals-spool fold (ADR 0025 §4, refusalfold.go) rides this sweep
	// for the same reason the reap itself does: it is a host loop that
	// already runs at pass start, mid-pass and pass epilogue, and every one
	// of those is a fold point the ADR names. Unconditional, ahead of the
	// NoReap/AutoReap gate below: whether closed sessions get KILLED is a
	// reap policy, and the audit trail's own integrity is not that policy's
	// to hold hostage.
	d.foldRefusalSpools()
	if d.NoReap || !d.App.AutoReap() {
		return
	}
	sessions, err := d.HB.Sessions()
	if err != nil {
		return
	}
	// One policy reading per sweep, taken at the head of it: three
	// populations and N sessions are answered by the same two graces, and a
	// config typo is named once rather than once per session (verifyPolicy's
	// own rule).
	pol := d.reapPolicy(when)
	for _, s := range sessions {
		class := reapClassOf(s, pol)
		if class == reapNothing {
			continue
		}
		// ADR 0028 §3, and PromptGrace's own window: any launcher's prompt
		// counts, and the run record is where a prompt this process never
		// sent is legible.
		if _, recent := d.promptedRecently(s.Name); recent {
			continue
		}
		if !d.settledForReap(s) {
			continue
		}
		why, ok := d.reapWhy(s, class, pol)
		if !ok {
			continue
		}
		if d.DryRun {
			fmt.Fprintf(d.Out, "would reap %s (%s)\n", s.Name, why)
			continue
		}
		// A shared checkout (no session worktree) has no branch for the
		// landing below to refuse over, and closing the workspace does not
		// touch its files — but the operator inherits a dirty tree with
		// nothing left pointing at who left it that way. Named once, on
		// stderr, and the kill proceeds: a closed bead over a dirty tree is
		// the operator's own scratch (reapguard.go), just not a silent one.
		// A worktree session gets the same warning for free below, from the
		// landing's own "KEPT" line.
		//
		// Only reapFleetClosed reaches it: the two widened arms require
		// that a kill take nothing at all, and a dirty tree is the first
		// thing residueHolds refuses over.
		if m, ok := d.HB.readMeta(s.Name); ok && SessionTreeOf(m) == nil {
			if len(dirtyPaths(s.Dir)) > 0 {
				fmt.Fprintf(d.errw(), "reap: %s (bead %s, closed) leaves %s dirty — no session branch to land it on\n",
					s.Name, s.Bead, AbbrevHome(s.Dir))
			}
		}
		landing, err := d.HB.KillSessionAndLand(s.Name)
		if err != nil {
			fmt.Fprintf(d.errw(), "reap: %s not killed: %v\n", s.Name, err)
			continue
		}
		fmt.Fprintf(d.Out, "reaped %s (%s)\n", s.Name, why)
		// Since ranger-base-qxvh that includes what became of the persona's
		// standing orders: the sweep commits them, because this is the path
		// that reaps at scale — ~30 in a day on the instance that motivated
		// it — and memory is the one artifact with no other copy. It spends
		// no landing TURN doing it: the bead is closed and the agent has
		// settled, so the persona has already had its wrap-up, and N
		// bounded turns in a row would stall the pass this sweep is an
		// epilogue to.
		for _, line := range landing.Lines() {
			fmt.Fprintf(d.Out, "  %s\n", line)
		}
	}
}

// settledForReap says nobody is working in this session any more — the half
// of the predicate herdr answers, the bead being the other half.
//
// It covers TWO shapes, and ranger-base-kftx is the second one arriving.
// MEASURED on the live fleet 2026-08-27: of five sessions with a finished
// agent over a closed bead, the sweep named two. Three carried herdr status
// "" — the operator's own first complaint on the us8 thread, "we have a
// bunch of dead shells".
//
//   - idle / done: a settled agent, the shape the spec shipped with. These
//     are the states the fire loop itself already treats as finished
//     (AgentWait's idle/done/blocked triad, less `blocked`, which is a
//     persona waiting on something and not a persona that has stopped).
//   - "": herdr detects NO agent in the workspace at all (Sessions()'s
//     status(), which reports a status only where `hasAgent` is true). The
//     persona's CLI exited — crash, /exit, the operator closing it — and
//     left a bare shell holding a closed bead. That is neither idle/done nor
//     working/blocked, so the shipped predicate said nothing about it and it
//     sat forever.
//
// "" is ALSO what a CLI that is still starting looks like: detection is
// blind for the first seconds of a launch. That ambiguity is not new and
// posse already has an answer for it, one line of evidence and one number —
// RelaunchAgent refuses to re-type into a session younger than
// RelaunchGrace, and dispatch.go's own `else if s.Status == ""` arm relaunches
// past it. So this asks the same question of the same field rather than
// coining a second liveness rule for the same ambiguity.
//
// RelaunchGrace, not StartupWait, and the bead named StartupWait: ranger-base-ze9p
// split those two knobs precisely here. StartupWait is the pass's DETECTION
// patience and tests shorten it to stay fast; RelaunchGrace is "how long a
// starting CLI may stay invisible to detection", measured against a session's
// real age, and nothing that shortens a test may shorten it. This is the
// second question, and in production both are the same 45s the bead asked for.
//
// What it is willing to be wrong about: detection blinking on a live agent.
// Then this kills a CLI mid-turn. It is the same evidence, on the same grace,
// that RelaunchAgent already acts on by TYPING a persona command into the
// pane — which lands inside a live CLI's input box if it is wrong — and it is
// spent here on a session whose bead the store of record calls closed, where
// no work is expected and where the landing still refuses to remove a dirty
// tree (it prints KEPT instead) and the shared-checkout warning still fires.
// Strictly less consequence than the risk already shipped on this signal.
func (d *Dispatcher) settledForReap(s HerdrSession) bool {
	switch s.Status {
	case "idle", "done":
		return true
	case "":
		// No meta is no age, and no age is not "old enough" — the same
		// fail-closed the unreadable bead gets. Only a meta rewritten or
		// pruned between Sessions()'s own read and this one reaches it
		// (Sessions() drops a session it cannot read), which is the same
		// interleaving RelaunchAgent guards against by comparing workspace
		// ids; here the safe answer is simply to wait for the next sweep.
		m, ok := d.HB.readMeta(s.Name)
		return ok && time.Since(m.Launched) >= d.RelaunchGrace
	}
	return false // working, blocked: somebody is in there
}

// foldRefusalSpools folds every live session's refusals spool into its
// persona's canonical log (ADR 0025 §4, refusalfold.go). Every session with
// a name and a persona, not just the ones settledForReap would act on: a
// long-lived session that is never reaped still needs its spool folded on a
// bounded cadence, or the tamper window this design accepts (lines
// appended and then truncated BETWEEN two folds) is unbounded instead of
// "the sweep cadence". A fold failure is this sweep's own to swallow, the
// same as a read failure above — a pass with real beads to dispatch does
// not fail because one persona's spool could not be folded.
func (d *Dispatcher) foldRefusalSpools() {
	sessions, err := d.HB.Sessions()
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.Agent == "" || s.Name == "" {
			continue
		}
		if err := d.App.FoldRefusalsSpool(s.Agent, s.Name); err != nil {
			fmt.Fprintf(d.errw(), "fold refusals spool for %s: %v\n", s.Name, err)
		}
	}
}

// ─── the residue the narrow sweep skipped (ranger-base-f6lk) ─────────────────

// Two populations sat PERMANENTLY outside the sweep above, and by 2026-08-29
// they were what the operator was reaping by hand — the mechanism this whole
// file exists to replace. Both are admitted here by the same three questions
// the narrow sweep already asks (settled, not just prompted, and the store of
// record says the work is finished); what changes is who may be asked, and
// what the tree has to be holding for the answer to count.
//
//   - CREW-MARKED (ADR 0008). The mark says "the operator is talking to
//     this", and §2 puts a marked session outside every sweep. But two
//     different things wear it, and only one of them is a conversation the
//     operator OWNS: `posse new` MAKES a session to talk to, while cockpit
//     `p` and `posse prompt` mark a session DISPATCH made for one bead
//     (§1's third row) that the operator merely stepped into. The second was
//     never theirs to keep — it is Dial F's per-bead session, its bead
//     closes, and what is left is a dead shell wearing a 👤. MEASURED in
//     dispatch-watch.log: two of them — `<persona>-<repo>-ranger-base-3j3t`
//     and `<persona>-<repo>-ranger-base-teau` — each skipped by the crew
//     shield on hundreds of consecutive passes.
//   - STAMPLESS (ranger-base-kftx), and ONLY PAST ROUTING. A per-bead-NAMED session carrying no
//     `bead:` pointer — a meta written by a binary from before the pointer
//     landed, or a `posse new` session later released with `posse crew
//     --off`. kftx tagged them 🏷️no-bead so the boundary would be visible
//     instead of silent, and left the reap to the operator; twelve hours of
//     one sitting idle is what that turned into.
//
// WHICH SHAPE A CREW SESSION IS, IS RENDERED AND NOT GUESSED. Dispatch names
// its per-bead session `SessionForBead(persona, dir, bead)`, and the session
// carries all three of those values in its own meta — so the name it WOULD
// have been given can be re-rendered and compared. That is not the inversion
// kftx refuses (`sessionSanitizeRe` folds `.` into `-`, so a name is a lossy
// encoding of a bead id and can never be read back into one): this goes the
// other way, from the record to the name. And the pointer is half the
// evidence on its own — `Bead:` at creation is set at exactly two call sites,
// both in dispatch.go, and `posse new` has no flag that can set it. So a
// crew session whose name is anything else — an operator-chosen name, the
// persona's reusable slot, a conversation `posse prompt` later stamped a
// bead onto (ADR 0008's adb7 amendment) — is the operator's own and is not
// touched at all, which is a stronger promise than the longer grace the bead
// asked for. What remains uncovered is stated rather than hidden: an
// operator who types dispatch's exact name into `posse new` and is then
// hand-dispatched that same bead is indistinguishable from dispatch's own
// session, and pays the graces and the empty-tree test below for it.
//
// WHY THE UNPOINTED ARM WAITS FOR ROUTING. A stampless session is not
// unambiguously residue, and this is the one place the two widened arms
// differ in kind. Dispatch reaches a session by NAME — `SessionForBead` for
// the bead it is about to work — whether or not that session carries a
// pointer, so a stampless session at a live bead's name is a SEAT this pass
// is about to relaunch into and reuse (rangerhq-vk2), not a dead shell.
// MEASURED: TestDispatchRelaunchesDeadAgent is exactly that shape — a
// per-bead-named session, no pointer, no agent, an hour old — and a
// pass-start sweep took it out from under the relaunch that was coming for
// it.
//
// The pass itself answers the question, so nothing new has to ask it. Any
// session a pass uses is either prompted (promptedRecently covers it, ADR
// 0028 §3) or resumed into, and a resume stamps the pointer (`NoteBead`) —
// which takes it out of this population altogether. So a session still
// unpointed at a sweep that runs PAST routing is one no bead in that pass's
// queue claimed, which is the predicate "no bead can claim this name" answered
// by the only thing that can answer it. Before routing the same session is a
// question nobody has asked yet, and the sweep says nothing about it.
//
// What that costs, stated: a pass that dies in gather (ranger-base-v674 — the
// reason the pass-START sweep exists at all) sweeps no stampless residue. It
// is the passes with real beads that die in that window, and a QUIET pass —
// the steady state, and the one this residue accumulates across — reaches its
// epilogue in seconds. The crew arm keeps both sites and needs to: its bead is
// CLOSED, and a closed bead is never dispatched again, so nothing is coming
// for that session at any point in any pass.
//
// AND NEITHER ARM MAY TAKE THE PULSE'S TARGET. ADR 0027's carve-out delivers
// the shop check into `pulse_persona:`'s live session; a sweep that reaps it
// turns every later tick into "undeliverable (no live session for X)". That
// persona is excluded by NAME in both widened arms — the bead's "not the
// operator's own or the coordinator", second half. (First half: the previous
// paragraph.)
//
// WHAT A KILL MUST TAKE FROM THEM: NOTHING. The narrow sweep proceeds over a
// dirty tree — a closed bead over uncommitted work is ADR 0041's business,
// and the landing keeps the tree and prints KEPT. These two arms do not.
// They rest on less evidence (a mark the operator's own hands put there, or
// no bead at all), so they demand that a kill take nothing that nothing else
// holds: `residueHolds` asks RemoveSessionTree's own refusal as a QUESTION,
// of the same two records (ranger-base-as19, ranger-base-x8jp). A tree that
// still holds work is reported and left standing; `landClosedTrees` lands it
// on this same pass and the next sweep takes the session. One extra pass, and
// never the codex 353-line near-miss (reapguard.go).

// reapClass is which population a session belongs to, and so which evidence
// the sweep needs before it may take it.
type reapClass int

const (
	// reapNothing: not the sweep's, on one of the standing exclusions.
	reapNothing reapClass = iota
	// reapFleetClosed is the population this file shipped with: dispatch's
	// own per-bead session, no crew mark, carrying the `bead:` pointer that
	// lets the store of record be asked whether the work is finished.
	reapFleetClosed
	// reapCrewClosed is a reapFleetClosed session the operator stepped into.
	// Same bead and the same reading of it, plus a longer grace and a tree
	// that must hold nothing.
	reapCrewClosed
	// reapUnpointed is kftx's 🏷️no-bead shape: a per-bead-named session with
	// no pointer, so no store can ever be asked about it. Judged on age and
	// an empty tree alone, which is why it is the strictest of the three.
	reapUnpointed
)

// DefaultCrewReapAfter and DefaultUnpointedReapAfter are the two graces, and
// they are POLICY DIALS rather than measurements of anything — no number here
// was read off the fleet, and both are config (`reap_crew_after:`,
// `reap_unpointed_after:`; `off` on either turns that arm back into the
// permanent skip it was).
//
// They are ordered, and the order is the argument. The crew grace has to
// outlast a conversation's own gaps — the operator steps away from a session
// they are talking to and comes back, and nothing posse records says how long
// that takes (typing in a pane leaves no stamp; ADR 0008 §1 accepted exactly
// that blindness). Four hours covers a meal or a meeting and is far short of
// the overnight accumulation this bead is about. The unpointed grace protects
// much less: nobody is in there, no bead exists to be unfinished, and the tree
// is provably holding nothing — so all it buys past `RelaunchGrace` is not
// racing a session somebody is about to use, and an hour is generous for that.
const (
	DefaultCrewReapAfter      = 4 * time.Hour
	DefaultUnpointedReapAfter = time.Hour
)

// reapWhen says whether this sweep is running BEFORE the pass has routed
// anything or AFTER it has, and it decides exactly one thing: whether the
// unpointed arm may fire (see reapClassOf).
type reapWhen bool

const (
	beforeRouting reapWhen = false
	afterRouting  reapWhen = true
)

// reapPolicy is one sweep's resolved policy.
type reapPolicy struct {
	crew      time.Duration // 0 = the crew arm is off
	unpointed time.Duration // 0 = the unpointed arm is off (see reapClassOf)
	pulse     string        // the persona ADR 0027's pulse delivers to ("" = none)
	when      reapWhen
}

func (d *Dispatcher) reapPolicy(when reapWhen) reapPolicy {
	return reapPolicy{
		crew:      d.App.reapAfter("reap_crew_after", DefaultCrewReapAfter, d.errw()),
		unpointed: d.App.reapAfter("reap_unpointed_after", DefaultUnpointedReapAfter, d.errw()),
		pulse:     pulsePersona(d.App),
		when:      when,
	}
}

// reapAfter is one grace from config. `off`/`never`/`0` disables that arm —
// spelled rather than numeric, because a grace of literally zero on a
// destructive sweep is nobody's intent and reading it as one would turn a
// typo into a kill. An unreadable value is named and the default stands, on
// verifyBatchAge's rule: this bound is the only thing between a live
// conversation and a closed workspace.
func (a *App) reapAfter(key string, def time.Duration, errw io.Writer) time.Duration {
	raw := strings.TrimSpace(YamlGet(a.ConfigPath, key))
	switch raw {
	case "":
		return def
	case "off", "never", "0":
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		fmt.Fprintf(errw, "reap: config %s: %q is not a positive duration (4h, 90m) or `off` — using %s\n", key, raw, def)
		return def
	}
	return d
}

// reapClassOf sorts a session into its population. It decides nothing about
// whether the session is FINISHED — that is reapWhy's, on the store of record
// — only whose question it is.
func reapClassOf(s HerdrSession, pol reapPolicy) reapClass {
	if s.Foreign || s.Name == "" || s.Dir == "" || s.Agent == "" {
		return reapNothing
	}
	// The pre-Dial-F slot: NoteBead points it at whichever bead last resumed
	// into it, but it is the persona's reusable session, never a per-bead
	// one, and never Dial F's to reap (rangerhq-v330's join depends on it
	// surviving between beads).
	if s.Name == SessionFor(s.Agent, s.Dir) {
		return reapNothing
	}
	if !s.Crew && s.Bead != "" {
		return reapFleetClosed
	}
	if pol.pulse != "" && s.Agent == pol.pulse {
		return reapNothing // ADR 0027's target, above
	}
	if s.Crew {
		if pol.crew <= 0 || s.Bead == "" || s.Name != SessionForBead(s.Agent, s.Dir, s.Bead) {
			return reapNothing
		}
		return reapCrewClosed
	}
	if pol.when == afterRouting && pol.unpointed > 0 && UnpointedBeadSession(s) {
		return reapUnpointed
	}
	return reapNothing
}

// reapWhy is the rest of the predicate and the words the line says, or false
// when this session is not the sweep's to take after all.
//
// The bead is read FRESH here, at reap time, never from the pass's own
// gathered results: `--resume` can close a bead between one pass's gather and
// this epilogue, and a cached status would disagree with the store it is
// supposed to defer to (ADR 0011).
func (d *Dispatcher) reapWhy(s HerdrSession, class reapClass, pol reapPolicy) (string, bool) {
	var why string
	switch class {
	case reapFleetClosed, reapCrewClosed:
		is, err := d.Bd.Show(s.Dir, s.Bead)
		if err != nil || is.Status != "closed" {
			return "", false
		}
		why = "bead " + s.Bead + " closed"
	case reapUnpointed:
		// Nothing to ask: the pointer is what makes a store askable, and
		// this population is defined by not having one.
		why = "no bead pointer"
	default:
		return "", false
	}
	if class != reapFleetClosed {
		grace := pol.crew
		if class == reapUnpointed {
			grace = pol.unpointed
		}
		m, ok := d.HB.readMeta(s.Name)
		if !ok {
			return "", false // no record is no age, and no age is not old enough
		}
		idle, dated := residueIdle(m)
		if !dated || idle < grace {
			return "", false
		}
		if held := residueHolds(m); held != "" {
			// Said out loud on every pass it is true, not once: the silence
			// is what read as a broken reaper and cost the hand-reaps
			// (kftx), and this is the same repeat-until-fixed line the
			// landing sweep prints for the same reason.
			d.printf("◑ %s idle %s over %s and NOT reaped: %s\n",
				s.Name, idle.Round(time.Minute), why, held)
			return "", false
		}
		if class == reapCrewClosed {
			why += ", crew-marked on a session dispatch made"
		}
		why += fmt.Sprintf(", idle %s", idle.Round(time.Minute))
	}
	// Two shapes of finished reach here and the operator hand-reaps them for
	// different reasons, so the line says which one it found.
	if s.Status == "" {
		why += ", no agent left in it"
	}
	return why, true
}

// residueIdle is how long posse has not touched this session: the later of
// the launch and the last work prompt, which are the only two moments any
// record here is stamped with.
//
// It is NOT "how long since the operator typed" — nothing records that, and
// ADR 0008 §1 accepted that blindness when it refused a timer. What covers
// the gap is the guard already ahead of this one: typing in a pane is what
// herdr reports as `working`, and settledForReap refuses a working session
// outright. So this is the honest floor of the two readings, not a claim to
// be the whole one.
//
// A meta with neither stamp has no age, and no age is not "old enough" — the
// same fail-closed the unreadable bead and the unreadable meta both get.
func residueIdle(m *HerdrMeta) (time.Duration, bool) {
	if m == nil {
		return 0, false
	}
	last := m.Launched
	if m.Prompted.After(last) {
		last = m.Prompted
	}
	if last.IsZero() {
		return 0, false
	}
	return time.Since(last), true
}

// residueHolds is what a kill of this session would take that nothing else
// holds — "" when a kill takes nothing at all. It is RemoveSessionTree's own
// refusal asked as a QUESTION rather than performed as an act, over the same
// two records, and it fails closed on every question it cannot answer.
//
//   - GIT in the session's own cwd. Uncommitted is the only shape of loss a
//     kill can cause in a shared checkout, and that is where the near-miss
//     was (reapguard.go, ranger-base-0fb).
//   - the SESSION BRANCH, for a worktree session. Ahead by sha is not ahead
//     by work (ranger-base-g2xf), and only the half of that equivalence which
//     MEASURES content may license a destroy: every commit patch-id
//     equivalent (ranger-base-as19 — git's `-x` trailer is somebody's
//     decision, not a measurement), AND the base holding the branch's bytes
//     for every path it touched (ranger-base-x8jp — patch-id normalises
//     whitespace, so "holds this patch" was never "holds this content").
//
// This is stricter than the kill that follows it needs to be, on purpose. A
// reapCrewClosed session's bead is closed, so KillSessionAndLand would land
// the branch itself — but a reap that lands is a reap that decides, and these
// two arms are the ones the operator did not previously trust to decide
// anything. Deferring costs one pass: landClosedTrees lands the same branch
// at the head of the very next one, and the sweep after that finds nothing
// held and takes the session.
func residueHolds(m *HerdrMeta) string {
	if dirty := dirtyPaths(m.Dir); len(dirty) > 0 {
		return fmt.Sprintf("%s has uncommitted work (%s)", AbbrevHome(m.Dir), dirtyList(dirty))
	}
	t := SessionTreeOf(m)
	if t == nil {
		// A shared checkout has no branch of its own, so the clean tree
		// above is the whole question.
		return ""
	}
	if !branchExists(t.Repo, t.Branch) {
		// Retired already — `posse worktrees --land` or a kill that landed
		// took the branch, and a branch that does not exist is the last copy
		// of nothing.
		return ""
	}
	n, ok := unlandedCount(t)
	if !ok {
		return fmt.Sprintf("what %s holds beyond %s cannot be counted", t.Branch, orDetached(t.Base))
	}
	if n == 0 {
		return ""
	}
	if eq := equivalentOnBase(t.Repo, t.Base, t.Branch); measuredOnBase(eq) {
		if lost, err := contentNotOnBase(t.Repo, t.Base, t.Branch); err == nil && len(lost) == 0 {
			return ""
		}
	}
	return fmt.Sprintf("%s holds %d commit(s) %s does not", t.Branch, n, orDetached(t.Base))
}
