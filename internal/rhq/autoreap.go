package rhq

// The end-of-pass auto-reap (rangerhq-us8): a per-bead session (ADR 0013 §4
// Dial F, <persona>-<repobase>-<bead>) whose bead the store of record now
// calls CLOSED, and whose agent herdr calls idle or done, is killed and
// landed exactly as `posse kill` would — one line said about it either way.
// Dial F gives every dispatched bead its own session and never reaps it
// itself (dispatch.go's own doc: "left idle for the operator or --watch to
// reap"), so without this an instance accumulates one dead pane per closed
// bead forever.
//
// MEASURED (monica, rangerhq-us8 comments): ~50 sessions reaped by hand over
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
//   - PROMPTED THIS PASS: a settle read moments after a fresh prompt is the
//     same race PromptGrace exists for in the fire loop — a bead that closed
//     mid-pass is safer left for the next pass's read than reaped on this
//     one's.
//
// And it reads the bead fresh, at reap time, never from the pass's own
// gathered results: `--resume` can close a bead between one pass's gather
// and this epilogue, and a cached status here would disagree with the store
// it is supposed to defer to (ADR 0011).
import (
	"fmt"
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
// either call site is equally safe. justPrompted names sessions this same
// pass fired a prompt at (pendingBead.session) — never a reap candidate,
// whatever herdr says about them right now; it is nil at the start-of-pass
// call, since nothing has been prompted yet. A read failure is this sweep's
// own to swallow: a pass that dispatched real work does not fail because a
// reap sweep could not list sessions.
func (d *Dispatcher) autoReapPass(justPrompted map[string]bool) {
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
		if justPrompted[s.Name] {
			continue
		}
		if s.Status != "idle" && s.Status != "done" {
			continue
		}
		is, err := d.Bd.Show(s.Dir, s.Bead)
		if err != nil || is.Status != "closed" {
			continue
		}
		if d.DryRun {
			fmt.Fprintf(d.Out, "would reap %s (bead %s closed)\n", s.Name, s.Bead)
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
		fmt.Fprintf(d.Out, "reaped %s (bead %s closed)\n", s.Name, s.Bead)
		if line := landing.Line(); line != "" {
			fmt.Fprintf(d.Out, "  %s\n", line)
		}
	}
}
