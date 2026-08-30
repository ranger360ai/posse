package rhq

// Settle-open escalation — ranger-base-9hm, discovered from ranger-base-f0g.
//
// `--resume` re-prompts an in_progress bead whose persona settled idle
// without closing it, and that is right the FIRST time: the measured cases
// (f0g) were three agents that had finished and simply never said so, and
// one nudge cleared all three. It is wrong the second time. A persona that
// settles open on the SAME bead again is not an agent that missed a prompt;
// it is a standing disagreement between the agent (believes it is done) and
// the bead (says in_progress), and repeating the same prompt every pass is
// an infinite polite retry — a token loop with no reader, which is exactly
// what rangerhq-zom's non-resuming default was protecting against and what
// f0g's resume-by-default made reachable again.
//
// So: on the second settle-open, dispatch stops nudging and files ONE
// `-l question` bead for the operator, then blocks the stuck bead on it.
// `bd ready` is what dispatch selects from, so the block is the stop —
// MEASURED: a dep'd bead drops out of the queue while the
// in_progress ones stay in it. Infinite retry becomes exactly one route to
// a human and one bead that is not dispatched again.
//
// WHERE THE COUNT LIVES, and why it is not a new file in $StateDir. The
// bead proposed a resume ledger beside `overflow.log`; ADR 0011's whole
// diagnosis argues against one. Its incident class is "dispatch infers
// cross-store facts from single-store snapshots", and the fact being
// counted here — did this bead settle open before, with the bead saying
// the same thing it says now — is a fact ABOUT THE BEAD. Counting it in bd
// is one store, not two that can disagree; counting it in $StateDir would
// be the fifth store the ADR rejects, and the first one that decides
// something. seatidle.go's ledger is the shape that IS allowed there, and
// it is allowed precisely because nothing reads it back into a decision.
//
// The count is therefore a comment the harness writes on the bead
// (`settled open [<status>]: …`), which also earns its keep twice over: it
// is the evidence the next reader of that bead needs, and the re-prompted
// persona reads its own bead's comments in the work prompt.
//
// A comment that fails to write undercounts, which costs one more
// re-prompt. A comment that writes and then a create that fails costs one
// more pass. Both are the safe direction: this mechanism must never file an
// escalation the shop did not earn.
//
// IDEMPOTENCE is the part flagged on review as the one that would bite, and
// it is NOT keyed on the comment. bd 0.49.1's create is not atomic
// (ranger-base-muoo, verifyafter.go's long note): the issue commits and the
// client then times out, so a second write keyed on the returned id never
// happens and the next pass files again — 33 duplicate P1 beads is what
// that cost last time. The dedupe is therefore the escalation's own TITLE,
// written by bd in the same breath as the issue: at most one OPEN question
// bead whose title names this stuck bead. An orphaned create still dedupes.
// An escalation the operator has answered and closed re-arms, which is
// right — a bead that sticks again after a human looked at it is news
// again.

import (
	"fmt"
	"strings"
)

const (
	// settleOpenPrefix opens the comment dispatch leaves on the FIRST
	// settle-open. The bead's status rides in the brackets because the
	// disagreement is between two named states: a bead whose status has
	// MOVED since (the operator reopened it, a persona blocked it) is a
	// different disagreement and starts its own count, which is the
	// "cleared when its status changes" half of the bead's shape.
	//
	// The count is of the TEXT, not of the comment's author. bd does report
	// one, but nothing in this repo has ever read `BdComment.Author` back
	// from real bd, and a dedupe keyed on an unverified field fails silently
	// in the direction of never escalating at all. A persona that forges the
	// marker on its own bead only escalates itself one pass early — which is
	// a rung it can already pull by hand.
	settleOpenPrefix = "settled open ["
	settleOpenAfter  = "]: "

	// settleStuckPrefix opens the escalation bead's title and is the
	// dedupe of record (see the muoo note above). The stuck bead's id
	// follows it immediately, so a title scan needs no parser.
	settleStuckPrefix = "settled open twice: "

	// SettleQuestionLabel is the label an escalation carries. `-l question`
	// is the operator's lane — and it is also what keeps the escalation
	// itself out of the queue: fireLoop refuses to dispatch a question.
	SettleQuestionLabel = "question"
)

// settleOpenComment is the marker text for one settle-open. Both halves of
// the disagreement are named, in the words their own store uses.
func settleOpenComment(status, session, settled string) string {
	return fmt.Sprintf("%s%s%s%s settled %q without closing the bead — re-prompted. "+
		"Settling open again escalates to the operator instead of re-prompting forever (ranger-base-9hm).",
		settleOpenPrefix, status, settleOpenAfter, session, settled)
}

// settleEscalatedComment is the breadcrumb left on the stuck bead naming the
// escalation filed for it. It replaces provenance the `discovered-from` edge
// cannot carry here (escalateSettleOpen), so it names both beads' side of the
// story — and it deliberately does NOT open with settleOpenPrefix, because
// the count reads every comment on this bead back.
func settleEscalatedComment(qid, status string) string {
	return fmt.Sprintf("escalated to %s — settled open twice with the bead saying %q, so dispatch stopped "+
		"re-prompting it and blocked it on %s (ranger-base-9hm). Closing %s puts it back in bd ready.",
		qid, status, qid, qid)
}

// settleOpenStatus recovers the bead status a settle-open comment recorded,
// or "" when the text is not one the harness wrote.
func settleOpenStatus(text string) string {
	if !strings.HasPrefix(text, settleOpenPrefix) {
		return ""
	}
	rest := text[len(settleOpenPrefix):]
	i := strings.Index(rest, settleOpenAfter)
	if i <= 0 {
		return ""
	}
	return rest[:i]
}

// settleStuckTitle is the escalation bead's title — the dedupe key, so the
// stuck bead's id sits at a fixed offset and nothing else may precede it.
func settleStuckTitle(id, persona, status string) string {
	return fmt.Sprintf("%s%s — %s believes it is done, the bead says %q", settleStuckPrefix, id, persona, status)
}

// settleStuckSource recovers the stuck bead's id from an escalation title.
func settleStuckSource(title string) string {
	if !strings.HasPrefix(title, settleStuckPrefix) {
		return ""
	}
	rest := title[len(settleStuckPrefix):]
	if i := strings.IndexByte(rest, ' '); i > 0 {
		rest = rest[:i]
	}
	if beadIDRe.MatchString(rest) {
		return rest
	}
	return ""
}

// noteSettleOpen is the whole rung, called from gather where the pass has
// just judged a settle against a bead that is not closed.
//
// It runs only under --resume, because --resume is what makes the retry
// infinite: without it the next pass prints "stopped on purpose?" and moves
// on, which is already a bounded answer, and escalating there would file
// question beads for a loop that is not running. --dry-run writes nothing,
// for the reason every writer in this package refuses to (seatidle.go): a
// pass that acted on nothing must not leave state a later pass counts.
func (d *Dispatcher) noteSettleOpen(p *pendingBead, settled, status string) {
	if d.DryRun || !d.Resume {
		return
	}
	cs, err := d.Bd.Comments(p.is.Dir, p.is.ID)
	if err != nil {
		// Not knowing whether this is the second time is not evidence that
		// it is. Say so and leave the bead to be re-prompted: one more
		// nudge is cheap, an escalation nobody earned is not.
		d.eprintf("posse: %s settle-open not counted — bd could not read its comments (%v)\n", p.is.ID, err)
		return
	}
	before := 0
	for _, c := range cs {
		if settleOpenStatus(c.Text) == status {
			before++
		}
	}
	if before == 0 {
		if err := d.Bd.Comment(p.is.Dir, p.is.ID, settleOpenComment(status, p.session, settled), VerifyActor); err != nil {
			d.eprintf("posse: %s settle-open not recorded (%v) — the next one counts as the first\n", p.is.ID, err)
		}
		return
	}
	d.escalateSettleOpen(p, settled, status)
}

// escalateSettleOpen files the operator's question and takes the stuck bead
// out of `bd ready`. Best effort throughout and never quiet: an escalation
// that cannot be filed must not turn a settle into a failed dispatch, but a
// loop that is still going to spin is a fact the pass owes out loud.
func (d *Dispatcher) escalateSettleOpen(p *pendingBead, settled, status string) {
	qid, err := d.openSettleEscalation(p.is)
	if err != nil {
		d.eprintf("posse: %s not escalated — bd could not list the open questions (%v)\n", p.is.ID, err)
		return
	}
	if qid == "" {
		// NO `Deps: discovered-from:<stuck>` here, and that absence is the
		// whole of ranger-base-23oo. bd 0.49.1's cycle check spans EVERY
		// dependency type, not only `blocks`: the create writes qid
		// --discovered-from--> stuck, and the `dep add stuck qid` two lines
		// below then closes a cycle and is refused, exit 1. Measured both
		// orderings — the edges are mutually exclusive whichever lands
		// first, so this is not a reordering. The block is the deliverable
		// (without it the bead stays in `bd ready` and --resume re-prompts
		// it forever, which is the loop this rung exists to stop), so the
		// provenance moves to where nothing can refuse it: the
		// discoveredFromMarkerPrefix line in the body, and a comment on the
		// stuck bead naming the escalation — fileMergeBlocked's idiom, for
		// the neighbouring reason.
		qid, err = d.Bd.Create(p.is.Dir, BdNew{
			Title:       settleStuckTitle(p.is.ID, p.persona, status),
			Assignee:    d.App.CfgGet("operator", ""),
			Labels:      []string{SettleQuestionLabel},
			Priority:    "1",
			Actor:       VerifyActor,
			Description: d.settleStuckBody(p, settled, status),
		})
		if err != nil {
			d.eprintf("posse: %s not escalated (%v) — --resume will re-prompt it again next pass\n", p.is.ID, err)
			return
		}
		d.printf("  ↳ %-14s escalated to %s — the operator decides; not re-prompted\n", p.is.ID, qid)
		// The pointer back, and the other half of the provenance the edge
		// no longer carries. Best effort: the escalation exists either way,
		// so a failed comment costs a breadcrumb, not the stop — and it must
		// never cost a second question bead. It is not the settle-open
		// marker and must never read as one (settleOpenStatus refuses it),
		// or an escalated bead would count its own escalation as a settle.
		if err := d.Bd.Comment(p.is.Dir, p.is.ID, settleEscalatedComment(qid, status), VerifyActor); err != nil {
			d.eprintf("posse: %s not commented with %s (%v) — the escalation exists, the pointer back does not\n", p.is.ID, qid, err)
		}
	} else {
		// The escalation exists and the bead was dispatched anyway, so the
		// blocking edge is what is missing — the one half of this that bd
		// can lose on its own. Retry it rather than file a second question.
		d.printf("  ↳ %-14s already escalated to %s — not re-filed\n", p.is.ID, qid)
	}
	d.blockOnEscalation(p.is, qid)
}

// blockOnEscalation is the stop: `bd dep add <stuck> <qid>` takes the bead
// out of `bd ready`, which is the set fireLoop selects from.
//
// The outcome is read back from the graph rather than off the exit code,
// for the reason Bd.Claim gives: an edge that already exists is a refusal
// with nothing wrong, and an edge that landed is worth more than a zero
// status. Only the read decides what the pass reports.
func (d *Dispatcher) blockOnEscalation(is RepoIssue, qid string) {
	addErr := d.Bd.DepAdd(is.Dir, is.ID, qid, VerifyActor)
	deps, err := d.Bd.DepList(is.Dir, is.ID)
	if err == nil {
		for _, dep := range deps {
			if dep.ID == qid {
				d.printf("  ↳ %-14s blocked on %s — out of bd ready until the operator answers\n", is.ID, qid)
				return
			}
		}
	}
	why := "the graph does not carry the edge"
	if addErr != nil {
		why = addErr.Error()
	} else if err != nil {
		why = err.Error()
	}
	d.eprintf("posse: %s is NOT blocked on %s (%s) — it stays in bd ready and --resume re-prompts it\n", is.ID, qid, why)
}

// openSettleEscalation is the dedupe: the id of the OPEN question bead
// already escalating this stuck bead, or "" for none.
func (d *Dispatcher) openSettleEscalation(is RepoIssue) (string, error) {
	qs, err := d.Bd.OpenLabeledAny(is.Dir, SettleQuestionLabel)
	if err != nil {
		return "", err
	}
	for _, q := range qs {
		if settleStuckSource(q.Title) == is.ID {
			return q.ID, nil
		}
	}
	return "", nil
}

// settleStuckBody is what the operator reads. The uncommitted count is the
// first fact in it deliberately: ranger-base-1cc sat on 353 uncommitted
// lines in a session worktree, and that — not the open bead — is what made
// this urgent rather than untidy. A tree is only reported when the session
// has one of its own; a session sharing the checkout has no dirt of its own
// to name.
func (d *Dispatcher) settleStuckBody(p *pendingBead, settled, status string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s was prompted, settled %q, and the bead still says %q. That has now happened twice,\n"+
		"so dispatch stopped re-prompting it: the agent believes it is done and the bead disagrees,\n"+
		"and a third identical prompt is a token loop with no reader (ranger-base-9hm).\n\n",
		p.is.ID, settled, status)
	fmt.Fprintf(&b, "bead:     %s (%s)\nassignee: %s\nsession:  %s\nrepo:     %s\n%s%s\n\n",
		p.is.ID, p.is.Title, p.persona, p.session, AbbrevHome(p.is.Dir),
		discoveredFromMarkerPrefix, p.is.ID)
	b.WriteString(d.settleTreeLines(p.session))
	fmt.Fprintf(&b, "\nWhat to decide: whether the work is done (close %s), whether it is not (say what is\n"+
		"left on it), or whether the session cannot finish it (kill and relaunch — a session keeps the\n"+
		"command line it was born with, so a promotion does not reach it). Closing THIS bead unblocks\n"+
		"%s and puts it back in bd ready.\n", p.is.ID, p.is.ID)
	return b.String()
}

// settleTreeLines names the session's own worktree and anything uncommitted
// left in it. Best effort: a session with no meta, no tree, or a git that
// will not answer produces a line saying so, never a silent absence — "no
// uncommitted work" and "nobody looked" are different facts to a reader
// deciding whether it is safe to kill the session.
func (d *Dispatcher) settleTreeLines(session string) string {
	m, ok := d.HB.readMeta(session)
	if !ok {
		return "tree:     unknown — no session meta to read (uncommitted work, if any, is NOT accounted for)\n"
	}
	t := SessionTreeOf(m)
	if t == nil {
		return "tree:     none of its own — the session shares the checkout, so its commits are already there\n"
	}
	dirty := dirtyPaths(t.Path)
	var b strings.Builder
	fmt.Fprintf(&b, "tree:     %s on %s (merges to %s at close)\n", AbbrevHome(t.Path), t.Branch, t.Base)
	if len(dirty) == 0 {
		fmt.Fprintf(&b, "uncommitted: none — nothing is lost by killing the session\n")
		return b.String()
	}
	fmt.Fprintf(&b, "uncommitted: %d path(s) in that tree, WHICH A SESSION REAP WOULD DESTROY: %s\n",
		len(dirty), strings.Join(dirty, " "))
	return b.String()
}
