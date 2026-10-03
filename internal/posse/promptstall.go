package posse

// A stalled prompt is an UNOBSERVED turn, not an undelivered one —
// ranger-base-uauvn.
//
// THE INCIDENT. MEASURED 2026-10-02 ~23:2x local (posse 0.5.0+1b8ffe2d): a
// pass prompted a seat and logged "herdr: agent prompt produced no observed
// working or blocked state within 5000 ms; current status is idle
// (agent_prompt_stalled) — unclaimed". The seat had in fact taken the
// prompt: it measured, edited five files, committed on its own branch and
// settled waiting on its suite. The bead sat OPEN and ready with finished
// work on a branch, and the operator re-claimed it by hand. The 1-minute
// loadavg at that moment was 94 — the same pass skipped a refill on the load
// guard — so the seat's first screen change took longer than five seconds.
//
// WHY THE OLD VERDICT WAS WRONG, in herdr's own words. `herdr agent prompt
// --help` on 0.9.1: "If the agent is already blocked, submission is rejected
// with agent_blocked BEFORE ANY INPUT IS SENT. When an ACCEPTED SUBMISSION
// starts from another non-working state, --wait requires an observed working
// or blocked state within 5000ms; otherwise it returns agent_prompt_stalled."
// So the three codes dispatch handed back a claim on (rangerhq-81d) are not
// one fact:
//
//	agent_not_ready       herdr will not address the pane — nothing was sent.
//	agent_blocked         herdr refused the submission — nothing was sent.
//	agent_prompt_stalled  the submission was ACCEPTED, so the text WAS sent;
//	                      what is missing is an OBSERVATION of the turn
//	                      starting inside herdr's own five seconds.
//
// The first two are positive evidence about DELIVERY. The third is a
// statement about herdr's window and says nothing about delivery at all — it
// is the same shape as a `--wait` timeout, which has never been allowed to
// unclaim (rangerhq-1z0, rangerhq-khc; runtimecheck.go states the rule as "a
// timeout is a check-in, NEVER an unclaim"). A bd claim is a
// lease-WITHOUT-expiry by ADR 0011's own decision, precisely because expiry
// without a fencing token the resource checks is double-dispatch; unclaiming
// on an unobserved turn is that expiry, reintroduced through a herdr code.
//
// WHY THE WINDOW IS NOT WIDENED INSTEAD. herdr owns those five seconds and
// 0.9.1 has no flag for them: MEASURED 2026-10-03 on this box, `herdr agent
// prompt --help` offers --wait, --until and --timeout and nothing else, and
// --timeout is the CALLER's patience, which herdr documents as returning
// `timeout` when it expires first — a different code with a different
// meaning. posse could only reach the window by dropping --wait and watching
// the pane itself, and that re-opens the race --wait exists to close: an
// `agent wait --until idle` started after the submission returns on the
// state that was already there and reports a turn that never started
// (reportedagent.go carries the same contract for the one route herdr will
// not take). So the window stays herdr's, and the VERDICT becomes posse's.

import (
	"fmt"
	"strconv"
	"time"
)

// stallVerdict is what the second reading decided about a stalled prompt.
// Three outcomes and only one of them hands the bead back, which is the
// whole point: the two that do not are what the incident needed.
type stallVerdict int

const (
	// stallRewait: a turn is under way. The claim stays and the settle is
	// waited for again, exactly as a working agent after a timed-out leg.
	stallRewait stallVerdict = iota
	// stallKeep: the claim stays and the bead is not judged this pass —
	// the verdict every reading that is IGNORANCE rather than evidence
	// gets here, plus the blocked one (an operator's to clear).
	stallKeep
	// stallHandBack: nothing says the prompt landed and nothing says work
	// happened. Only this one reaches the unclaim.
	stallHandBack
)

// judgeStall is the second reading a stalled prompt gets before anything is
// judged, and it is the only caller of the two readings below. It prints the
// line that explains the verdict, because the operator reading the pass is
// the person who had to re-claim the bead by hand last time.
//
// TWO READINGS, BOTH POSITIVE-EVIDENCE, AND IN THIS ORDER:
//
//  1. Ask herdr for the turn it did not see, on posse's clock
//     (stallGrace). A turn that starts late is the measured incident, and
//     an `agent wait` is the right instrument because it OBSERVES rather
//     than polls — a poll started after the keystroke reads the state that
//     was already there, which is the very mistake the 5s window exists to
//     refuse.
//  2. Ask git whether this session has COMMITTED anything. It covers the
//     case the first reading cannot: a turn that started and finished
//     inside the grace, so there was no transition left to observe by the
//     time anyone looked. A session's branch is cut per bead
//     (SessionForBead), so a commit ahead of its base is this bead's work
//     and nobody else's.
//
// Ignorance is never a hand-back. A herdr that will not answer and a git
// that will not answer both keep the claim, on the rule gather's own
// unreadable-status arm already applies: not knowing what the agent is doing
// is not proof that it is doing nothing, and rangerhq-khc is what unclaiming
// on it costs. The direction is deliberate and it is the cheap one —
// recovery from a kept claim is `--resume` and a line in the cockpit, while
// recovery from a wrong hand-back is a second seat on a bead that is already
// finished.
func (d *Dispatcher) judgeStall(p *pendingBead) stallVerdict {
	grace := d.stallGrace(p.runtime)
	d.printf("◷ %-14s herdr typed the prompt into %s and saw no turn start inside its own 5s window — re-reading for up to %s before anything is judged (ranger-base-uauvn)\n",
		p.is.ID, p.session, grace)
	st, why := d.afterStall(p, grace)
	switch st {
	case "working":
		d.printf("◷ %-14s a turn is under way in %s after all (the stall was a slow start, not a lost prompt) — claim kept, waiting again\n",
			p.is.ID, p.session)
		return stallRewait
	case "blocked":
		d.printf("⛔ %-14s blocked in %s — intervene (posse attach %s); claim kept\n", p.is.ID, p.session, p.session)
		return stallKeep
	case stallUnreadable:
		d.printf("◷ %-14s %s — claim kept, not judged this pass (posse peek %s)\n", p.is.ID, why, p.session)
		return stallKeep
	}
	// st is stallNoTurn from here down.
	// herdr watched the grace out and no turn started in it. The second
	// reading decides, and only a git that ANSWERED "nothing" hands back.
	switch n, read := d.committedWork(p); {
	case !read:
		d.printf("◷ %-14s %s, and whether %s has committed anything cannot be read — claim kept, not judged this pass (posse peek %s)\n",
			p.is.ID, why, p.session, p.session)
		return stallKeep
	case n > 0:
		d.printf("◷ %-14s %s, but %s has %d commit(s) on its own branch — the prompt landed; claim kept, not judged this pass (posse peek %s)\n",
			p.is.ID, why, p.session, n, p.session)
		return stallKeep
	}
	return stallHandBack
}

// stallGrace is how long posse watches for the turn herdr did not see.
//
// It is the runtime's own `startup_wait:` — the one number in this shop that
// already means "how long this CLI may take to put something on screen",
// MEASURED per runtime (45s on claude) and raisable by the operator for a
// runtime or a box that is slower. No new number is coined and no new config
// key is added: the bead's own ask was that the window scale "with load (or
// with startup_wait)", and load is not a thing posse can read into a
// deadline honestly — what it can do is spend the patience the runtime was
// already measured to need. ASSUMED 2026-10-03, not measured: that a seat
// whose screen takes S seconds to appear takes no more than S seconds to
// show its first turn. The verdict it protects fails safe in either
// direction — a grace too short falls through to the commit reading, and a
// grace too long only delays a hand-back nobody is waiting on.
func (d *Dispatcher) stallGrace(runtime string) time.Duration { return d.runtimeWait(runtime) }

// afterStall's two non-state answers, kept apart because they decide
// opposite things. "A tri-state probe's third state is the design": a guard
// that fires on ignorance is a wall, so only the answered one goes on to the
// commit reading and from there to a possible hand-back.
const (
	// stallNoTurn: herdr answered, and no turn had started.
	stallNoTurn = "none"
	// stallUnreadable: herdr could not be asked at all.
	stallUnreadable = ""
)

// afterStall asks herdr for the transition it did not see, with posse's own
// patience: "working", "blocked", stallNoTurn or stallUnreadable, with the
// sentence that explains the last two beside it.
//
// `--until working --until blocked` is herdr's own stall question, re-asked
// with a longer window: the same pair reportedagent.go waits on when posse
// has to mirror this contract by hand for a pane herdr labels but does not
// detect. Only `blocked` is not news — herdr reaches a blocked state by
// drawing a dialog, and the operator is the one who clears it.
func (d *Dispatcher) afterStall(p *pendingBead, grace time.Duration) (state, why string) {
	ms := int(grace / time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	res, err := d.HB.H.AgentWait(p.target, []string{"working", "blocked"}, ms)
	switch {
	case IsHerdrCode(err, "timeout"):
		return stallNoTurn, fmt.Sprintf("no turn started in %s either", grace)
	case err != nil:
		return stallUnreadable, fmt.Sprintf("herdr could not say what %s is doing (%v)", p.session, err)
	}
	switch st := agentStatusFromResult(res); st {
	case "working", "blocked":
		return st, ""
	case stallUnreadable:
		return stallUnreadable, fmt.Sprintf("herdr answered the wait for %s with no state at all", p.session)
	default:
		// herdr returned without the states that were asked for. That is
		// not a transition and it is not ignorance either: it is herdr
		// naming a state, so it goes to the commit reading like a timeout.
		return stallNoTurn, fmt.Sprintf("herdr answered %q rather than a turn starting", st)
	}
}

// committedWork is how many commits this session is ahead of the branch it
// was cut from, and whether that could be read at all.
//
// (0, false) is the ignorance arm and keeps the claim; (0, true) is the only
// answer in this file that can reach an unclaim. A session that shares the
// checkout answers (0, true) honestly — it has no branch of its own, so
// there is no such evidence to be had, which is the shape every session had
// before per-session worktrees landed.
//
// Asked through workHead, so the count is of the work and not of the ref: a
// container-tier session commits on a DETACHED HEAD by design and leaves its
// branch where it was cut (PrepareSessionHead), and counting the branch
// there would read a seat's real commits as nothing — the false-negative
// MergeSessionWork's splice exists for, and here it would hand back a bead
// whose work is sitting in a tree.
func (d *Dispatcher) committedWork(p *pendingBead) (int, bool) {
	m, ok := d.HB.readMeta(p.session)
	if !ok {
		return 0, false
	}
	t := SessionTreeOf(m)
	if t == nil {
		return 0, true // shares the checkout: no branch of its own to read
	}
	if t.Base == "" {
		return 0, false // a repo on a detached HEAD: nothing to count against
	}
	head, ok := workHead(t)
	if !ok {
		return 0, false
	}
	n, err := git(t.Repo, "rev-list", "--count", t.Base+".."+head)
	if err != nil {
		return 0, false
	}
	c, err := strconv.Atoi(n)
	if err != nil {
		return 0, false
	}
	return c, true
}

// unclaimedOnTheBead is the third of the bead's asks, and the one that is
// not about the decision at all: when a pass DOES hand a bead back, the bead
// itself says so.
//
// The pass printed a line, and the pass's output is not what the next reader
// has. A bead that went open with a session still standing beside it — maybe
// holding work, maybe not — reads to a human as a bead nobody ever started,
// and the only record that travels with it is a comment on it. So the
// comment names the session, so `posse worktrees` can be pointed at
// something, and the herdr code, so the next reader can tell a refused
// submission from an unobserved one.
//
// Best effort, and deliberately after the unclaim: a store that will not
// take a comment must not turn a cleaned-up claim into a stranded one. It
// says so on stderr, where every other "the bead could not be written"
// sentence in this pass goes.
func (d *Dispatcher) unclaimedOnTheBead(is RepoIssue, persona, session string, promptErr error) {
	if session == "" {
		return
	}
	body := fmt.Sprintf("posse dispatch unclaimed this bead: the prompt to %s failed — %v.\n\n"+
		"It is open and unassigned again, so it can be re-dispatched. If %s turns out to hold work anyway, "+
		"`posse worktrees` names its tree and this comment is why the bead was open (ranger-base-uauvn).",
		session, promptErr, session)
	if err := d.Bd.Comment(is.Dir, is.ID, body, persona); err != nil {
		d.eprintf("posse: %s was unclaimed but the bead could not be told why (%v)\n", is.ID, err)
	}
}
