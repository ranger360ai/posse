package rhq

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
// Why it never guesses the bead. The branch name carries the bead id
// (SessionForBead), and parsing it back out would be a guess: persona names
// and repo basenames both contain '-'. So the sweep asks the record instead,
// and a branch with no record is REPORTED and not landed — the whole point
// is that "closed means it is on main" stops being asserted from something
// nobody wrote down.
//
// What it does not do. It does not remove a tree (that is `posse kill`'s,
// which refuses while anything would be lost), it does not touch a tree
// whose bead is open (the persona is working in it), and it does not FILE a
// bead when a merge is blocked. mergeBack files there because a judged close
// happens once; this runs every pass, and a bead per pass over a permanently
// conflicted branch is spam, not a handoff. The ⚠ line repeats instead — on
// every pass, which is what "the shop can see it" means here.

import "strconv"

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
		if t.Bead == "" {
			d.printf("◑ %s holds work that is not on %s and no record says which bead — `posse worktrees --land` decides it\n",
				t.Branch, orDetached(t.Base))
			continue
		}
		// Read the bead fresh from the store of record, like the reap does
		// (autoreap.go): this sweep's whole premise is a close nobody
		// watched, so nothing cached can have seen it.
		is, err := d.Bd.Show(t.Repo, t.Bead)
		if err != nil {
			d.printf("◑ %-14s %s holds unlanded work and bd could not say whether it is closed (%v)\n", t.Bead, t.Branch, err)
			continue
		}
		if is.Status != "closed" {
			continue // open: the persona is still working in it
		}
		if d.DryRun {
			d.printf("would land %s from %s onto %s (bead %s closed)\n", t.Branch, AbbrevHome(t.Path), orDetached(t.Base), t.Bead)
			continue
		}
		// The lock is taken once, on the first tree that needs it, and held
		// for the rest of the sweep: moving a repo's branch is check-then-act
		// against a store two launchers share (ADR 0011 §1). Taken lazily so
		// the common pass — nothing to land — never waits on another
		// launcher at all.
		if lock == nil {
			if lock, err = lockLaunches(d.App, d.Out); err != nil {
				d.eprintf("posse: unlanded work not swept — the launcher lock is unavailable (%v)\n", err)
				return
			}
		}
		o, err := MergeSessionWork(t)
		switch {
		case err != nil:
			d.printf("⚠ %-14s %s not landed onto %s: %v — the branch still holds the work\n", t.Bead, t.Branch, orDetached(t.Base), err)
		case o.Merged && o.Commits > 0:
			how := "fast-forwarded"
			if o.Rebased {
				how = "rebased and fast-forwarded"
			}
			d.printf("⤴ %-14s %d commit(s) %s from %s onto %s in %s (closed after its pass)\n",
				t.Bead, o.Commits, how, t.Branch, t.Base, AbbrevHome(t.Repo))
		case !o.Merged:
			d.printf("⚠ %-14s %d commit(s) on %s did NOT reach %s: %s\n", t.Bead, o.Commits, t.Branch, orDetached(t.Base), o.Reason)
		}
		if len(o.Dirty) > 0 {
			d.printf("◑ %-14s %d uncommitted path(s) left in %s — closed, and this part did not land\n",
				t.Bead, len(o.Dirty), AbbrevHome(t.Path))
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
