package rhq

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
func (d *Dispatcher) autoReapPass() {
	if d.NoReap || !d.App.AutoReap() {
		return
	}
	sessions, err := d.HB.Sessions()
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.Crew || s.Bead == "" || s.Dir == "" || s.Agent == "" {
			continue
		}
		// The pre-Dial-F slot: NoteBead points it at whichever bead last
		// resumed into it, but it is the persona's reusable session, never a
		// per-bead one, and never Dial F's to reap (rangerhq-v330's join
		// depends on it surviving between beads).
		if s.Name == SessionFor(s.Agent, s.Dir) {
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
		is, err := d.Bd.Show(s.Dir, s.Bead)
		if err != nil || is.Status != "closed" {
			continue
		}
		// Two shapes of finished reach here and the operator hand-reaps them
		// for different reasons, so the line says which one it found.
		why := "closed"
		if s.Status == "" {
			why = "closed, no agent left in it"
		}
		if d.DryRun {
			fmt.Fprintf(d.Out, "would reap %s (bead %s %s)\n", s.Name, s.Bead, why)
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
		fmt.Fprintf(d.Out, "reaped %s (bead %s %s)\n", s.Name, s.Bead, why)
		if line := landing.Line(); line != "" {
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
