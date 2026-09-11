package posse

// The landing gate, ADR 0013 §4's reading carried one step further: a kill
// merges a session's branch onto the repo's branch only when the BEAD says
// the work is finished.
//
// MEASURED, and this is the loss it is built from (ranger-base-pqque,
// 2026-09-11). A seat held an in_progress bead with one committed change on
// its branch and stopped ON PURPOSE, because landing that change deploys a
// live site: it filed a question bead, blocked its own bead on it, and left
// the decision to the operator. The operator then ran `posse kill <session>`
// to free the seat — and the kill performed the landing the seat had
// refused to perform, onto the repo's branch, in one line of output. The
// gated act was not the commit and not the kill; it was the landing, and
// the kill was the one path to it that asked nothing.
//
// So this asks the same store the reap guard asks, about the same session,
// one step later:
//
//   - the reap guard (reapguard.go) refuses to KILL a session whose bead is
//     in_progress over an UNCOMMITTED tree, because a workspace close over
//     uncommitted work destroys it.
//   - this refuses to LAND a session whose bead is not closed, because a
//     commit is not the record releasing the work — the close is. Work that
//     is committed is not at risk from the kill, so the kill proceeds; what
//     it does NOT do is move it onto the branch the operator ships from.
//
// A kept tree loses nothing and needs no rescue. The branch stays, the
// worktree stays, and the branch carries its own `bead:` stamp (worktree.go
// beadKey), which outlives the session meta the kill removes — so when the
// question is answered and the bead is closed, landClosedTrees lands it on
// the next pass with nobody typing anything. `posse worktrees --land` is
// the same landing by hand, for an operator who has decided and does not
// want to wait for a pass.
//
// WHAT IT DOES NOT GATE. A tree with nothing ahead of its base is not a
// landing at all — there is no act for this to gate, and refusing there
// would leave every finished session's empty tree standing forever, the
// reap's unpointed and crew arms included (autoreap.go, which reaches this
// path only over trees residueHolds has already found empty). So the first
// question is whether there is anything to land, and only then whose
// permission it needs.
//
// `--force` does NOT stand it down, and that is ADR 0013 §4's own sentence
// about the flag: it says the operator has looked at that session's
// unfinished work, and it stands the reap guard down "and nothing else —
// not the foreign-row refusal ... and not the tree's contents, which are
// left where they are". Landing is a decision about the repo's branch, not
// about the session, and nothing about reading a kill refusal is evidence
// for it.
//
// It fails CLOSED on ignorance, like every other reader of this pair: a bead
// bd cannot answer for is not a bead that said yes, and the cost is
// asymmetric — a spurious keep costs a tree that stands until the next
// sweep reads it, and a spurious landing puts work on the operator's branch
// that nobody released.

import (
	"fmt"
	"strings"
)

// landRefusal is why this kill must not land the session's branch, or ""
// when it may. The whole sentence, cure included, because it is printed as
// the KEEP reason (KillLanding.Line) with nothing else to hold it up.
func (b *HerdrBackend) landRefusal(m *HerdrMeta, t *SessionTree) string {
	if m == nil || t == nil || m.Bead == "" {
		// No bead pointer is no record to ask: an interactive `posse new`
		// tree, a recipe, or a session from before the pointer existed.
		// The sweep says its own sentence about a tree no record accounts
		// for (landsweep.go, ADR 0058 D4); this gate has nothing to read
		// and does not invent a verdict.
		return ""
	}
	if nothingToLand(t) {
		return "" // no landing to gate — and the removal below is cleanup, not a decision
	}
	is, err := b.bd().Show(t.Repo, m.Bead)
	switch {
	case err != nil:
		return fmt.Sprintf("bd could not say whether %s is finished (%v) — not landed; `posse worktrees --land` finishes it once it can", m.Bead, err)
	case is.Status == "closed":
		return ""
	}
	held := m.Bead + " is " + is.Status
	if blockers := b.openBlockers(t.Repo, m.Bead); blockers != "" {
		held += " and blocked on " + blockers
	}
	return held + " — not landed; closing the bead lands it on the next pass, or `posse worktrees --land` lands it now"
}

// openBlockers names the unanswered beads holding this one out of the
// queue, or "" for none. It is read only on the refusal path, where the
// operator is being told why a tree stayed and the blocking bead is the
// thing they have to act on — a kill that lands says nothing and asks
// nothing.
//
// The blocking edge is `blocks` or the unspelled default, the same reading
// promptContext makes of the same listing (dispatch.go). A store that
// cannot be asked names nothing: the status above is already the refusal,
// and a second unanswerable question does not improve it.
func (b *HerdrBackend) openBlockers(repo, id string) string {
	deps, err := b.bd().DepList(repo, id)
	if err != nil {
		return ""
	}
	var open []string
	for _, d := range deps {
		if d.DependencyType != "blocks" && d.DependencyType != "" {
			continue
		}
		if d.Status != "closed" {
			open = append(open, d.ID)
		}
	}
	// Bounded for dirtyList's reason: the operator needs to know which
	// decision is owed, not to read the graph.
	const max = 3
	if len(open) > max {
		return fmt.Sprintf("%s and %d more", strings.Join(open[:max], ", "), len(open)-max)
	}
	return strings.Join(open, ", ")
}
