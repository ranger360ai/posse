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
// WHAT IT DOES NOT DO, AND WHAT IT DOES NOW (ADR 0058). This paragraph used
// to read "it does not remove a tree (that is `posse kill`'s, which refuses
// while anything would be lost)". That restraint was written about `--land`,
// which reads git alone and so cannot tell a dead session's tree from one a
// persona is working in this second — and it was never true of this sweep,
// which reads the bead fresh from the store of record, walks live herdr
// sessions and holds the launcher lock. Read as a design intent for two
// weeks, it made the trees permanent: MEASURED 2026-09-05, 70 standing in
// ~/src/posse, 38 of them dead, clean, closed and fully landed, 36 by plain
// fast-forward, with the `n == 0` continue below the only thing that had
// ever looked at them.
//
// So a tree the four facts of ADR 0058 D1 all hold for is retired here, with
// the lock held and facts 2 and 3 read again inside it (retire.go). What is
// still not this sweep's, and prints exactly the sentence it printed before:
// an OPEN bead's tree (the persona is working in it), a tree no bead record
// accounts for (ADR 0006 — `--land --force` is the operator's word), a dirty
// tree (ADR 0041), and any commit whose landing is a decision rather than a
// measurement.
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
	"time"
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
	grace := d.App.graceAfter("retire_tree_after", DefaultRetireTreeAfter, d.errWriter())
	for _, t := range trees {
		// Whether there is anything to land at all, asked on BOTH tips a
		// session's work can sit on (nothingToLand, below). `landed` is where
		// 36 of the 70 trees on this box sit: this used to `continue` here in
		// silence, which is why nothing in posse had ever looked at them (ADR
		// 0058's census). It still lands nothing — what it now goes on to ask
		// is whether anything is left for the tree to be FOR.
		//
		// The BRANCH count alone would not do, and ranger-base-vavx2 measured
		// why: `<base>..<branch>` is ZERO over a tree whose HEAD is DETACHED,
		// so a container-tier session's whole work reads as landed and never
		// reaches MergeSessionWork at all. Both tips here, and the retire's
		// fact 2 asks them again from removalTips (retire.go) — two readings,
		// and the stricter one governs the destroy.
		landed := nothingToLand(t)
		id := t.Bead
		if id == "" {
			id = d.beadFromMeta(t)
		}
		if id == "" {
			if landed {
				// ADR 0006's rule, restated as ADR 0058 D4: no record
				// accounts for this tree, so nothing unattended may act on
				// it — and there is no unlanded work to report either. The
				// listing says the sentence; a line per pass here would only
				// repeat it at somebody who has not been asked to act.
				continue
			}
			d.printf("◑ %s holds work that is not on %s and no record says which bead — `posse worktrees` shows it and `--land --force` decides it\n",
				t.Branch, orDetached(t.Base))
			continue
		}
		// Read the bead fresh from the store of record, like the reap does
		// (autoreap.go): this sweep's whole premise is a close nobody
		// watched, so nothing cached can have seen it. Since ADR 0058 the
		// read is made for every tree and not only for one holding work —
		// the retire's fact 1 is this answer, and the landed trees are the
		// population it exists for. So is beadFromMeta's backfill above,
		// which now reaches a LANDED tree whose branch was never stamped:
		// that stamp is the only thing that can ever name such a tree's
		// bead once the kill has taken its meta, and a tree nothing can
		// name is one nothing may retire.
		is, err := d.Bd.Show(t.Repo, id)
		if err != nil {
			if landed {
				// The old sentence would be false here (nothing is
				// unlanded), and the retire needs an answer it did not get,
				// so the tree stands. Said on every pass it is true: an
				// unreadable store is not a transient the operator should
				// have to infer from a tree that never goes away.
				d.printf("◑ %-14s %s kept: bd could not say whether it is closed (%v)\n", id, t.Branch, err)
				continue
			}
			d.printf("◑ %-14s %s holds unlanded work and bd could not say whether it is closed (%v)\n", id, t.Branch, err)
			continue
		}
		if is.Status != "closed" {
			continue // open: the persona is still working in it
		}
		said := false
		if !landed {
			if d.DryRun {
				d.printf("would land %s from %s onto %s (bead %s closed)\n", t.Branch, AbbrevHome(t.Path), orDetached(t.Base), id)
				continue
			}
			// The lock is taken once, on the first tree that needs it, and
			// held for the rest of the sweep: moving a repo's branch is
			// check-then-act against a store two launchers share (ADR 0011
			// §1). Taken lazily so the common pass — nothing to land and
			// nothing to retire — never waits on another launcher at all.
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
			// This pass has now said something about this tree, and a
			// second line saying it is still standing would be that
			// sentence again in other words. The retire below is asked
			// anyway — a tree whose work was equivalent or just landed is
			// exactly the one that becomes retirable — but its KEEP is
			// left to the line above.
			said = true
		}
		d.retireTree(t, id, is.Status, grace, &lock, said)
	}
}

// retireTree is ADR 0058 D2: the landing sweep's own act on a tree there is
// nothing left to keep. It is the last thing said about a tree, after the
// landing, because a tree that just landed is exactly the one that becomes
// retirable — the equivalence case prints its `≡` line one final time and
// then goes.
//
// The predicate is asked TWICE and that is the point (ADR 0011 §2's reclaim
// rule, rangerhq-3a5t): once outside the launcher lock, cheaply, so that the
// common tree — inside its grace, or with a session still alive — costs no
// lock at all; and then facts 2 and 3 again with the lock HELD, because
// evidence read before the lock is a fact about the instant it was read and
// not about the instant `worktree remove` lands. Between the two a `posse
// new` can create this session's name again, and a commit can arrive in the
// tree.
//
// `said` is whether the caller already printed a line about this tree. One
// tree, one line: a keep is worth saying on every pass it holds (kftx), and
// saying it twice is how a board of 70 trees stops being read at all.
func (d *Dispatcher) retireTree(t *SessionTree, id, status string, grace time.Duration, lock **LaunchLock, said bool) {
	v := retirable(t, status, d.HB, grace)
	if !v.retire {
		if !v.quiet && !said {
			d.printf("◑ %-14s %s kept: %s\n", id, t.Branch, v.why)
		}
		return
	}
	if d.DryRun {
		d.printf("would retire %s and %s (bead %s: %s)\n", AbbrevHome(t.Path), t.Branch, id, v.why)
		return
	}
	if *lock == nil {
		l, err := lockLaunches(d.App, d.outWriter())
		if err != nil {
			d.eprintf("posse: %s not retired — the launcher lock is unavailable (%v)\n", t.Branch, err)
			return
		}
		*lock = l
	}
	if why := retireHeldOrAlive(t, d.HB); why != "" {
		// It changed under us in the window the lock exists to close. Said
		// whatever the caller printed: this is not the standing condition
		// `said` covers, it is the race being caught.
		d.printf("◑ %-14s %s kept: %s (it changed while the lock was being taken)\n", id, t.Branch, why)
		return
	}
	if err := RemoveSessionTree(t, false); err != nil {
		d.printf("⚠ %-14s %s not retired: %v\n", id, t.Branch, err)
		return
	}
	d.printf("⌫ %-14s %s retired: %s\n", id, t.Branch, v.why)
}

// nothingToLand is the sweep's landing test: true only when this tree holds
// nothing the base does not, on EITHER of the two tips a session's work can
// sit on, which is the one state with nothing to land and nothing to say
// about landing. It used to be the SKIP test and the whole of what this
// sweep asked; since ADR 0058 a true answer only means the landing half has
// nothing to do, and the tree goes on to the retire below.
//
// IT ASKS BOTH TIPS (ranger-base-vavx2), for removalTips' reason
// (ranger-base-v2rj7): `<base>..<branch>` is ZERO over a worktree whose HEAD
// is DETACHED, because a commit made there writes no ref and the branch is
// still where it was cut — and that is what a container-tier session is
// launched on ON PURPOSE, since a ref-less commit is what buys the `:ro`
// common dir (PrepareSessionHead, ranger-base-t4f1). So the whole of such a
// session's work read here as "already on the base", and the tree was skipped
// before the bead was read, before MergeSessionWork was called, and before
// any line was printed — over exactly the population this sweep exists for, a
// close nobody watched. MEASURED 2026-09-05: a stamped detached tree with one
// commit on it and a CLOSED bead drew the empty string out of this pass and
// left main not reaching the commit.
//
// The head is not asked INSTEAD, which would be a trade and not a fix: a
// branch holding a commit its own worktree walked away from is landable work
// the head does not reach, and MergeSessionWork lands the branch for it.
// Skipping is what has to be unanimous, so any tip with something ahead of
// the base sends this tree on to be read, merged and reported.
//
// Every unanswerable question fails to the same side the count always did: a
// detached REPO (no base) or a tip git would not count is not "nothing to
// land", so it goes on to ask the bead and lets MergeSessionWork say in words
// why it cannot happen (ranger-base-gs9j).
func nothingToLand(t *SessionTree) bool {
	if t.Base == "" {
		return false
	}
	branchSHA := ""
	if branchExists(t.Repo, t.Branch) {
		branchSHA, _ = git(t.Repo, "rev-parse", "refs/heads/"+t.Branch)
		if n, ok := unlandedCount(t); !ok || n > 0 {
			return false
		}
	}
	// `head != branchSHA` and not "is the HEAD detached", removalTips'
	// threshold: what matters is whether the branch tip above already
	// accounted for this commit, and where the two are the same commit it
	// did — so this arm is the detached case and only it, a no-op over a tree
	// whose HEAD is on its own branch.
	if head, ok := workHead(t); ok && head != branchSHA {
		if n, ok := unlandedAhead(t, head); !ok || n > 0 {
			return false
		}
	}
	return true
}

// unlandedCount is how many commits the BRANCH has that its base does not,
// and whether the question could be answered at all. false is a detached
// repo or a branch git would not count — neither of which is "nothing to
// land", so both go on to ask the bead and let MergeSessionWork say in words
// why it cannot happen.
//
// The branch tip and nothing else, because that is one of the two questions
// its callers ask and they compose the other themselves: the SWEEP asks
// nothingToLand above, and unaccountedFor (worktree.go) asks this first —
// its sentences are the branch's, `git log <base>..<branch>` to read and
// `--land --force` to run — and then asks unlandedAhead of the tree's HEAD
// when the branch has nothing, rewriting the sentence for the tip it found
// (ranger-base-qihvt). Neither caller may take this count for the whole
// answer: it is ZERO over a detached tree holding a session's entire work.
func unlandedCount(t *SessionTree) (int, bool) { return unlandedAhead(t, t.Branch) }

// unlandedAhead is that count asked of one named tip.
func unlandedAhead(t *SessionTree, tip string) (int, bool) {
	if t.Base == "" {
		return 0, false
	}
	out, err := git(t.Repo, "rev-list", "--count", t.Base+".."+tip)
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
	m, ok := d.HB.readMeta(SessionOfBranch(t.Branch))
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
