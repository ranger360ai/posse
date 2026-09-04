package posse

// The landing sweep (ranger-base-nurl): every pass lands the session tree of
// every bead the store of record now calls CLOSED, whoever closed it and
// whenever.
//
// What it is for. mergeBack (dispatch.go) runs where a pass JUDGES a close,
// and that is not every close. A bead whose wait ran out keeps its claim and
// is not judged this pass; a persona that closes it ten minutes later closes
// it in front of nobody, and its branch is then unlanded with nothing left
// watching. MEASURED 2026-08-27, four closed beads at once whose commits were
// on their session branch and not on main — one of them a 2172-line
// governance surface a later bead then built on and could not find. Nothing
// was lost (the branch holds it, `posse worktrees --land` finishes it), but
// the invariant the ADR 0006 §3 verify pass reads — closed means it is on
// main — was false four times over, and the only way to see that was to run
// the census by hand.
//
// Why it reads GIT and not the session list. autoReapPass already sweeps
// closed beads, and lands what it reaps; this exists for what that sweep
// cannot see. It walks live herdr sessions, so a tree whose session was
// killed, whose herdr restarted, or whose meta a kill removed on its way out
// (rangerhq-09o2) is invisible to it — and those are precisely the trees
// that strand. The one record that survives every path is the one git keeps:
// the worktree registration, the branch, and the two values recorded on that
// branch (worktree.go baseKey/beadKey).
//
// The transition, and the second record. A tree cut before the branch was
// stamped still has a session meta naming its bead where it is alive, and
// beadFromMeta joins on it and writes the answer onto the branch — so the
// backfill happens once, on the first pass that looks, and survives the kill
// that takes the meta away.
//
// Why it never guesses the bead. The branch name carries the bead id
// (SessionForBead), and parsing it back out would be a guess: persona names
// and repo basenames both contain '-'. So the sweep asks the record instead,
// and a branch with no record is REPORTED and not landed — the whole point
// is that "closed means it is on main" stops being asserted from something
// nobody wrote down.
//
// What it does not do. It does not remove a tree (that is `posse kill`'s,
// which refuses while anything would be lost) and it does not touch a tree
// whose bead is open (the persona is working in it).
//
// WHAT IT FILES, AND WHY THE SPAM OBJECTION WAS NEVER THE RIGHT ONE
// (ranger-base-5nf8m). This paragraph used to refuse to file a merge-back
// bead here at all: "mergeBack files there because a judged close happens
// once; this runs every pass, and a bead per pass over a permanently
// conflicted branch is spam, not a handoff. The ⚠ line repeats instead."
// Fourteen lines below it, the same file already answered that objection for
// the bead it DOES file — and the answer applies unchanged: the spam is
// priced by the KEY, not by the site. Both handoffs dedupe on their OPEN title (the
// merge-back's carries branch+base, which a branch cut per bead cannot move;
// closeddirty's carries the bead id at a fixed offset) and the closed-dirty
// comment dedupes on its `closed dirty [` marker, so N passes over one
// blocked or dirty tree leave one bead and one comment, not N. A bead per
// pass is what the paragraph ruled out and none of these is one.
//
// So both are filed from here, for the reason this file exists: the sweep is
// the only site that sees a close nobody watched, which makes it the site
// most likely to be a strand's ONLY reader, and a ⚠ line on a pass nobody
// was watching is not a record. ranger-base-aupee is the measurement —
// closed at 861b0e6, 134 files that never reached main, no merge-back bead
// in the store, and a human finding it hours later by hand.
//
//   - a blocked merge → noteMergeBlocked (dispatch.go), the same handoff the
//     judged close files. ranger-base-dybv fixed the inference in landed()
//     and closed over the reporting on the stated assumption that this site
//     already filed one. It did not; now it does.
//   - uncommitted paths → noteClosedDirty (closeddirty.go, ADR 0041 §1–§2),
//     routed back to the closer because a close nobody watched is exactly
//     the one whose dirt nobody is going to notice.

import (
	"strconv"
	"strings"
)

// landClosedTrees lands every session tree whose bead is closed. Called at
// pass start, after the reap and before routing: a pass that dies in gather
// (every --watch instance on record has died somewhere in that window,
// ranger-base-v674) still lands what the previous pass left, and a bead
// closed after this pass stops watching lands on the next one.
//
// Best effort throughout, and never quiet in the direction that matters: a
// pass that dispatched real work does not fail because a merge could not
// happen, but a closed bead whose code is not on the base is said out loud
// every time it is true.
func (d *Dispatcher) landClosedTrees(dirFilter string) {
	dirs := d.App.BeadsDirs()
	if dirFilter != "" {
		dirs = []string{dirFilter}
	}
	// The pins first, and OUTSIDE the tree walk, because a pin outlives the
	// tree that earned it: by the time a block's pin is droppable its
	// worktree is gone, so SessionTreesIn no longer reaches that branch at
	// all and nothing below this line would ever see it again
	// (ranger-base-m3195). A dry run reads and reports; it does not delete.
	if !d.DryRun {
		for _, repo := range mainCheckoutsOf(dirs) {
			prunePinnedBlocks(d.Bd, repo, d.eprintf)
		}
	}
	trees, err := SessionTreesIn(dirs)
	if err != nil {
		d.eprintf("posse: session worktrees could not be listed (%v) — unlanded work is not being checked this pass\n", err)
		return
	}
	var lock *LaunchLock
	defer func() {
		if lock != nil {
			lock.Release()
		}
	}()
	for _, t := range trees {
		if n, ok := unlandedCount(t); ok && n == 0 {
			continue // its work is already on the base: nothing to say
		}
		id := t.Bead
		if id == "" {
			id = d.beadFromMeta(t)
		}
		if id == "" {
			d.printf("◑ %s holds work that is not on %s and no record says which bead — `posse worktrees` shows it and `--land --force` decides it\n",
				t.Branch, orDetached(t.Base))
			continue
		}
		// Read the bead fresh from the store of record, like the reap does
		// (autoreap.go): this sweep's whole premise is a close nobody
		// watched, so nothing cached can have seen it.
		is, err := d.Bd.Show(t.Repo, id)
		if err != nil {
			d.printf("◑ %-14s %s holds unlanded work and bd could not say whether it is closed (%v)\n", id, t.Branch, err)
			continue
		}
		if is.Status != "closed" {
			continue // open: the persona is still working in it
		}
		if d.DryRun {
			d.printf("would land %s from %s onto %s (bead %s closed)\n", t.Branch, AbbrevHome(t.Path), orDetached(t.Base), id)
			continue
		}
		// The lock is taken once, on the first tree that needs it, and held
		// for the rest of the sweep: moving a repo's branch is check-then-act
		// against a store two launchers share (ADR 0011 §1). Taken lazily so
		// the common pass — nothing to land — never waits on another
		// launcher at all.
		if lock == nil {
			if lock, err = lockLaunches(d.App, d.outWriter()); err != nil {
				d.eprintf("posse: unlanded work not swept — the launcher lock is unavailable (%v)\n", err)
				return
			}
		}
		o, err := MergeSessionWork(t)
		switch {
		case err != nil:
			d.printf("⚠ %-14s %s not landed onto %s: %v — the branch still holds the work\n", id, t.Branch, orDetached(t.Base), err)
		case len(o.Equivalent) > 0:
			// Not a strand and not a landing this pass did: the base was
			// already holding the work under other shas (ranger-base-g2xf).
			d.printf("≡ %-14s %s\n", id, o.EquivalentNote())
		case o.Merged && o.Commits > 0:
			how := "fast-forwarded"
			if o.Rebased {
				how = "rebased and fast-forwarded"
			}
			d.printf("⤴ %-14s %d commit(s) %s from %s onto %s in %s (closed after its pass)\n",
				id, o.Commits, how, t.Branch, t.Base, AbbrevHome(t.Repo))
		case !o.Merged:
			d.printf("⚠ %-14s %d commit(s) on %s did NOT reach %s: %s\n", id, o.Commits, t.Branch, orDetached(t.Base), o.Reason)
			// And on the bead, or the ⚠ line is the whole record again and
			// this sweep repeats it every pass with nobody reading it — the
			// half ranger-base-dybv's close assumed mergeBack covered and
			// mergeBack cannot (see the header). Deduped on the handoff's
			// own title, so a permanently blocked branch costs one bead.
			// The closer is the bead's assignee, then whoever filed it — bd
			// records no close actor (verifyCloser).
			noteMergeBlocked(d.Bd, t.Repo, id, verifyCloser(is), t, o, d.printf, d.eprintf)
		}
		if len(o.Dirty) > 0 {
			d.printf("◑ %-14s %d uncommitted path(s) left in %s — closed, and this part did not land\n",
				id, len(o.Dirty), AbbrevHome(t.Path))
			// The bead is where that has to be written, or the pass line is
			// the only record again and this sweep repeats it forever with
			// nobody reading it (ADR 0041 §1–§2, closeddirty.go). The marker
			// dedupes across all three sites, so the sweep's every-pass
			// cadence costs at most the one comment mergeBack may already
			// have left. The closer is the bead's assignee, then whoever
			// filed it — bd records no close actor (verifyCloser).
			noteClosedDirty(d.Bd, t.Repo, id, verifyCloser(is), t, o, d.printf, d.eprintf)
		}
	}
}

// unlandedCount is how many commits the branch has that its base does not,
// and whether the question could be answered at all. false is a detached
// repo or a branch git would not count — neither of which is "nothing to
// land", so both go on to ask the bead and let MergeSessionWork say in words
// why it cannot happen.
func unlandedCount(t *SessionTree) (int, bool) {
	if t.Base == "" {
		return 0, false
	}
	out, err := git(t.Repo, "rev-list", "--count", t.Base+".."+t.Branch)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, false
	}
	return n, true
}

// beadFromMeta is the other record, and the transition: a tree cut before
// the branch was stamped (every tree standing when this landed) still has a
// live session meta naming its bead, and that is the record mergeBack itself
// reads. Joined on the branch and not on the name alone — a meta whose
// session was recreated elsewhere is a different tree's record — and the
// answer is written onto the branch, so the backfill happens once and
// survives the kill that takes the meta away.
//
// "" whenever the two records cannot be joined, which the caller reports and
// never guesses past.
func (d *Dispatcher) beadFromMeta(t *SessionTree) string {
	m, ok := d.HB.readMeta(strings.TrimPrefix(t.Branch, "posse/"))
	if !ok || m.Bead == "" || m.Branch != t.Branch {
		return ""
	}
	// The backfill is a WRITE, so --dry-run reads the meta and stamps
	// nothing: the flag's whole promise is that a diagnostic pass changes no
	// state, git config included.
	if !d.DryRun {
		if err := recordBead(t.Repo, t.Branch, m.Bead); err != nil {
			d.eprintf("posse: %s not stamped with bead %s (%v)\n", t.Branch, m.Bead, err)
		}
	}
	return m.Bead
}
