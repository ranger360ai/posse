package posse

// The reap guard, ADR 0013 §4: a session whose bead is still `in_progress`
// and whose working directory holds uncommitted work is NOT killed.
//
// MEASURED, and this is the near-miss it is built from (ranger-base-0fb):
// three dispatched codex sessions in a row did their work and skipped the
// bookkeeping — no comment, no close — and one of them was sitting on 353
// uncommitted lines in the SHARED checkout when the operator went to reap
// it. Nothing between the reap and the loss: the shared checkout has no
// session branch to land, so `KillSessionAndLand` has no tree to refuse over
// (SessionTreeOf returns nil) and the kill is a plain workspace close. L3's
// pathspec rule already stops an unqualified commit; it does not stop a
// kill. The per-session worktree that would have caught it is a property of
// the launch, not of the reap, and every crew session and every pre-worktree
// session still shares the checkout.
//
// So the guard is asked of the two stores that already know, and of nothing
// else:
//
//   - the BEAD — the store of record (ADR 0011). in_progress means work
//     somebody claimed and nobody recorded finishing. `bead:` in the session
//     meta is a POINTER to which one, written at dispatch; the status is
//     read from bd every time, so the meta can never disagree with the
//     store about whether the work is done.
//   - GIT — `git status --porcelain` in the session's own cwd. Uncommitted
//     is the only shape of loss a kill can cause: committed work survives on
//     a branch, and a clean tree has nothing to lose however open its bead.
//
// Both arms have to fire. An open bead over a clean tree is a bookkeeping
// skip, which is gather's line to print and `--resume`'s to retry, not a
// reason to keep a workspace alive. A dirty tree under a closed bead is the
// operator's own scratch and none of the harness's business.
//
// It fails CLOSED on ignorance, and only inside that pair: a session with a
// bead pointer and a dirty tree whose status bd cannot answer for is refused
// too. The same rule RemoveSessionTree already applies — the safe answer to
// an unanswerable question about destroying work is no — and the cost of
// being wrong is asymmetric: a spurious refusal costs one `--force`, and a
// spurious kill costs work that has no other copy.

import (
	"fmt"
	"strings"
)

// bd is the beads runner the reap guard reads the store of record with. A
// backend built without one (every test backend that never asks) resolves
// the ambient binary, exactly as dispatch does.
func (b *HerdrBackend) bd() Bd {
	if b.Bd.Bin != "" {
		return b.Bd
	}
	return NewBd()
}

// ReapRefusal is why a kill of this session would destroy work, or "" when
// it would not. It is the REASON only — one line, no verdict and no cure:
// what a kill and a refresh do about it differ, and they say so themselves.
// One line because it is printed by `posse kill` and shown on the cockpit's
// status line, which has room for exactly one.
//
// Callers pass the meta they already read rather than a name, because the
// kill path reads the meta before anything can prune it — and a guard that
// re-reads is a guard that can be asked about a different session than the
// one being killed.
func (b *HerdrBackend) ReapRefusal(m *HerdrMeta) string {
	if m == nil || m.Bead == "" || m.Dir == "" {
		// Not a dispatched bead session: an interactive `posse new`, a crew
		// session, a recipe, or a session from before the pointer existed.
		// There is no bead to ask about, so there is nothing to refuse over
		// and the kill is what it always was.
		return ""
	}
	dirty := dirtyPaths(m.Dir)
	if len(dirty) == 0 {
		return "" // nothing a kill could take that a commit does not hold
	}
	is, err := b.bd().Show(m.Dir, m.Bead)
	switch {
	case err != nil:
		return fmt.Sprintf("%s holds %s, %s has uncommitted work (%s), and bd could not say whether that bead is finished (%v)",
			m.Name, m.Bead, AbbrevHome(m.Dir), dirtyList(dirty), err)
	case is.Status != "in_progress":
		return ""
	}
	return fmt.Sprintf("%s still holds %s (in_progress) and %s has uncommitted work (%s)",
		m.Name, m.Bead, AbbrevHome(m.Dir), dirtyList(dirty))
}

// dirtyList names what would be lost, bounded: the operator needs to
// recognize the tree, not to read a diff. A 300-file refusal that scrolls
// its own reason off the screen says less than a short one.
func dirtyList(paths []string) string {
	const max = 5
	if len(paths) <= max {
		return strings.Join(paths, " ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(paths[:max], " "), len(paths)-max)
}
