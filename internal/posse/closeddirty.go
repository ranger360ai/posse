package posse

// ADR 0041 — a close that leaves uncommitted paths in its session tree is
// written ON THE BEAD and routed back to the closer (ranger-base-tc2pp,
// discovered from ranger-base-k77sk).
//
// THE INCIDENT. ranger-base-yeg1 closed naming four deliverables and not one
// of them was committed: the branch's reflog is a single "Created from main".
// The launcher SAW it — the pass printed the uncommitted count, then "closed
// with no commit … nothing to merge", and the reap KEPT the tree for the same
// reason. Three lines, one log. A day later a pin file carried three claims
// copied out of that close comment, all false at HEAD. Every reading went to
// a retrospective instrument (dispatch-watch.log) or a pull surface (`posse
// worktrees`); nothing wrote it where the false claim lives, which is under
// the close comment on the bead the next reader copies from.
//
// WHAT IS WRITTEN, AND WHERE THE COUNT LIVES. The dirty set is a fact ABOUT
// THE BEAD, so it is counted on the bead — settleopen.go's argument, and ADR
// 0011's: a $StateDir ledger would be the fifth store, and the first one that
// could disagree with bd about a bead. So §1 is a comment (`closed dirty [`)
// and §2 is a handoff bead assigned to the closer, in fileMergeBlocked's
// exact shape.
//
// THE TRIGGER IS DIRTY ALONE (§5). Eight of the twelve commit-less closes
// measured on 2026-09-01 were correct — design, question and verify closes
// legitimately produce no commit — so an empty branch is not a finding.
// `Commits` rides in the sentence as context and never decides anything: a
// clean tree gets no comment however many commits it has, and a dirty one
// gets one however few.
//
// THE THREE SITES, AND WHY THE DEDUPE IS THE COMMENT'S OWN PREFIX. mergeBack
// (the judged close), landClosedTrees (the sweep, every pass) and the kill's
// landing all read a closed bead's tree, and the last two repeat forever. The
// key is the marker at the head of the text rather than an id returned by a
// write, because bd's create is not atomic (ranger-base-muoo) and a dedupe
// keyed on something a timeout can lose files again every pass — 33 duplicate
// P1 beads is what that cost last time. It is the same trick settleopen.go
// plays with `settled open [` and fileMergeBlocked with its title.
//
// A KNOWN GAP, stated rather than papered over: landClosedTrees skips a tree
// with nothing ahead of its base BEFORE it reads the tree, so a close nobody
// watched (the ranger-base-nurl class) that committed NOTHING and left dirt
// is caught by the judged close and by the kill, not by the sweep. Widening
// that skip costs a `git status` and a `bd show` per live session per pass,
// on the pass's hot loop, for a case two other sites already cover.
//
// WHAT THIS NEVER DOES (§4): it does not reopen the bead — status is the
// closer's write in the store of record, and a reopened bead re-enters `bd
// ready` and dispatches into a NEW, clean tree — and it does not commit the
// paths (ADR 0022: a commit by anyone but the writer attributes content
// nobody chose to ship; yeg1's closer had decided part of that diff was not
// to land).

import (
	"fmt"
	"strings"
)

const (
	// closedDirtyPrefix opens the comment and IS its dedupe key, so nothing
	// may precede it in the text. The count sits inside the brackets the way
	// settleOpenPrefix carries its status: one glance says how much did not
	// land, and a reader who copies the close comment above it sees the
	// correction without opening anything.
	//
	// The scan is of the TEXT, not of the comment's author, for
	// settleOpenPrefix's reason: nothing in this repo has ever read
	// BdComment.Author back from real bd, and a dedupe keyed on an unverified
	// field fails silently in the direction of never noting anything at all.
	// A persona that forges the marker on its own bead suppresses one comment
	// about its own tree, which is a thing it can already do by committing.
	closedDirtyPrefix = "closed dirty ["

	// closedDirtyTitlePrefix opens the §2 handoff's title, so the bead id
	// follows immediately and a title scan needs no parser.
	closedDirtyTitlePrefix = "closed dirty: "

	// closedDirtyTitleSep closes the fixed head of the title — everything
	// after it MOVES. See closedDirtyTitleKey.
	closedDirtyTitleSep = " — "
)

// closedDirtyTitleKey is the §2 handoff's dedupe key: the fixed head of the
// title, `closed dirty: <id> — `, and nothing after it.
//
// NOT the exact title, which is what openMergeBlocked keys on and what this
// keyed on until ranger-base-a3zvb. The difference is what the two titles
// carry after the id. The merge-back title carries the BRANCH, which is cut
// per bead and cannot move; this one carries a COUNT, and the handoff's own
// description is what moves it — "COMMIT them under <id> in that worktree"
// invites a closer to commit some of the paths, and a closer who commits one
// of two and stops has a tree whose next reading spells a different title.
// The exact-title key then files a SECOND P1 bead at the same closer for the
// same tree, which is the duplicate-handoff failure this file's header prices
// at 33 beads (measured 2026-09-02 on the pass's own lines, verifying
// ranger-base-tc2pp; pinned as TestClosedDirtyDedupeSurvivesAPartialCommit).
// §1's comment never had the bug: its key is `closed dirty [` with the count
// INSIDE the brackets, past the key.
//
// The id is terminated by the separator, so `a-1` cannot swallow `a-10` and
// one seat's handoff cannot swallow another's: the bead id IS the tree and
// the closer. Everything the operator's listing reads as a sentence — the
// count, the branch — stays in the title and out of the key.
func closedDirtyTitleKey(id string) string {
	return closedDirtyTitlePrefix + id + closedDirtyTitleSep
}

// closedDirtyComment is the sentence ADR 0041 §1 specifies. `Commits` is
// context inside it and never a trigger — see the header.
//
// The path list is bounded by dirtyList for the reason the reap refusal
// bounds its own: the exact count is already in the brackets, and a
// 300-path comment that scrolls its own point away says less than a short
// one.
func closedDirtyComment(t *SessionTree, o MergeOutcome) string {
	return fmt.Sprintf("%s%d path(s)]: %s in %s; %d commit(s) on %s — nothing carries these; "+
		"the tree is kept until they are committed or discarded (`posse worktrees`)",
		closedDirtyPrefix, len(o.Dirty), dirtyList(o.Dirty), AbbrevHome(t.Path), o.Commits, t.Branch)
}

// closedDirtyNoted reports whether one of the three sites has already written
// the marker on this bead.
func closedDirtyNoted(cs []BdComment) bool {
	for _, c := range cs {
		if strings.HasPrefix(c.Text, closedDirtyPrefix) {
			return true
		}
	}
	return false
}

// closedDirtyTitle is the handoff's title and its dedupe key in one. Branch
// and count are in it so the operator's listing reads as a sentence, and the
// bead id is at a fixed offset so the dedupe needs no parser.
func closedDirtyTitle(id string, n int, branch string) string {
	return fmt.Sprintf("%s%d uncommitted path(s) in %s", closedDirtyTitleKey(id), n, branch)
}

// noteClosedDirty is the whole of ADR 0041 §1 and §2 for one closed bead
// whose tree is dirty, called from every site that has just read that tree.
// A no-op on a clean tree, which is the ordinary close.
//
// Best effort throughout and never quiet, on mergeBack's rule: a comment
// that cannot be written must not turn a bead the persona really closed into
// a failed dispatch, but a close whose work did not land is a fact the pass
// owes out loud. say/warn are the caller's own printf pair — the dispatcher's
// are serialized on its output mutex and must not be bypassed.
func noteClosedDirty(bd Bd, dir, id, persona string, t *SessionTree, o MergeOutcome, say, warn func(string, ...any)) {
	if len(o.Dirty) == 0 {
		return
	}
	commentClosedDirty(bd, dir, id, t, o, warn)
	fileClosedDirty(bd, dir, id, persona, t, o, say, warn)
}

// commentClosedDirty is §1: at most one marker on the bead, between all three
// sites and every later pass.
func commentClosedDirty(bd Bd, dir, id string, t *SessionTree, o MergeOutcome, warn func(string, ...any)) {
	cs, err := bd.Comments(dir, id)
	if err != nil {
		// The read is the dedupe, not the record. A graph that will not
		// answer must not cost a closed bead the correction that belongs
		// under its close comment: say so and write, because a duplicate is
		// visible and a missing one is not (fileMergeBlocked's rule).
		warn("posse: %s could not be checked for an existing `%s` comment (%v) — commenting anyway\n",
			id, closedDirtyPrefix, err)
	} else if closedDirtyNoted(cs) {
		return
	}
	if err := bd.Comment(dir, id, closedDirtyComment(t, o), VerifyActor); err != nil {
		warn("posse: %s not commented with its uncommitted paths (%v) — %s still holds them\n", id, err, AbbrevHome(t.Path))
	}
}

// fileClosedDirty is §2: the close goes back to the closer as a bead, in the
// merge-back handoff's shape and deduped over its OPEN titles, which bd
// writes in the same breath as the issue and a timed-out create therefore
// cannot lose — but on the title's fixed HEAD (closedDirtyTitleKey), because
// this title's tail carries a count the closer can move.
//
// A scratch file files too. The launcher cannot tell an 814-line rewrite from
// a stray `calls.log`, and §4 forbids it from guessing; the census says ~1 in
// 4 of these is scratch, and that price — one `git clean` for the closer — is
// the accepted cost of never quietly discarding the other three.
func fileClosedDirty(bd Bd, dir, id, persona string, t *SessionTree, o MergeOutcome, say, warn func(string, ...any)) {
	title := closedDirtyTitle(id, len(o.Dirty), t.Branch)
	key := closedDirtyTitleKey(id)
	if open, err := openPrefixedBead(bd, dir, MergeBlockedLabel, key); err != nil {
		warn("posse: %s could not be checked for an existing closed-dirty bead (%v) — filing one\n", id, err)
	} else if open != "" {
		say("  ↳ %s already filed for %s — not re-filed\n", open, persona)
		return
	}
	filed, err := bd.Create(dir, BdNew{
		Title:    title,
		Assignee: persona,
		Labels:   []string{MergeBlockedLabel},
		Deps:     []string{"discovered-from:" + id},
		Priority: "1",
		Actor:    VerifyActor,
		Description: fmt.Sprintf(
			"%s closed %s, and %d uncommitted path(s) are still in its session tree. Only commits\n"+
				"move — the launcher fast-forwards %s onto %s — so this part of the close did not land,\n"+
				"and `closed dirty [` on %s says so under the close comment (ADR 0041 §1).\n\n"+
				"paths:    %s\nworktree: %s\nbranch:   %s\nrepo:     %s\n%s%s\n\n"+
				"Two ways to end it, and only you can say which:\n"+
				"  - COMMIT them under %s in that worktree (`git commit -m '...' -- <paths>`), and a\n"+
				"    launcher pass or `posse kill` lands them onto %s; or\n"+
				"  - DISCARD them (`git checkout -- <paths>`, `git clean`) if they were never to ship.\n\n"+
				"`posse kill` retires that tree only after one of those: it refuses while the tree\n"+
				"still holds work. The launcher does neither for you — it does not reopen %s and it\n"+
				"does not commit paths you did not choose to ship (ADR 0041 §4, ADR 0022).",
			persona, id, len(o.Dirty),
			t.Branch, orDetached(t.Base), id,
			dirtyList(o.Dirty), AbbrevHome(t.Path), t.Branch, AbbrevHome(t.Repo),
			discoveredFromMarkerPrefix, id,
			id, orDetached(t.Base), id),
	})
	if err != nil {
		// bd may have committed the issue and failed on the `--deps` edge
		// alone (verifyMarkerPrefix has the measurement); the exit code
		// cannot tell those apart, so the GRAPH decides what is reported. A
		// bead that IS there is filed — edgeless, and named — not missing,
		// or the operator goes looking for a handoff that exists.
		found, ferr := openPrefixedBead(bd, dir, MergeBlockedLabel, key)
		switch {
		case ferr != nil:
			warn("posse: could not file the closed-dirty bead for %s (%v) — %s still holds the paths, and the graph would not say whether one landed anyway (%v)\n",
				id, err, AbbrevHome(t.Path), ferr)
			return
		case found == "":
			warn("posse: could not file the closed-dirty bead for %s (%v) — %s still holds the paths\n", id, err, AbbrevHome(t.Path))
			return
		}
		say("  ↳ filed %s for %s WITHOUT its discovered-from:%s edge (%v) — its provenance is the description and the comment on %s\n",
			found, persona, id, err, id)
		return
	}
	say("  ↳ filed %s for %s — %d uncommitted path(s) in %s\n", filed, persona, len(o.Dirty), t.Branch)
}

// openTitledBead is the id of the OPEN bead in this lane with exactly this
// title, or "" for none — the merge-back handoff's dedupe. Closed does not
// count, for both readers: a persona that resolved one and a tree that is
// dirty again are two handoffs.
//
// EXACTLY, never a prefix: that title carries the BRANCH the handoff is
// about, a whole title of fixed parts, and a prefix match would let one
// seat's open bead swallow every other seat's silently — measured as
// mutation E6 in mergebackdedupe_qa_test.go. The closed-dirty title is not
// that shape (closedDirtyTitleKey) and reads through openPrefixedBead.
func openTitledBead(bd Bd, dir, label, title string) (string, error) {
	return openMatchedBead(bd, dir, label, func(t string) bool { return t == title })
}

// openPrefixedBead is the same read for a handoff whose title has a moving
// tail: the id of the OPEN bead in this lane whose title STARTS with prefix.
//
// Only ever called with a key that terminates the bead id
// (closedDirtyTitleKey), which is what keeps this from being the E6 mutation
// openTitledBead's comment refuses. A prefix that stopped short of the
// separator would let `a-1` answer for `a-10`; a prefix that stopped at the
// label would let one seat's handoff swallow every other seat's.
func openPrefixedBead(bd Bd, dir, label, prefix string) (string, error) {
	return openMatchedBead(bd, dir, label, func(t string) bool { return strings.HasPrefix(t, prefix) })
}

func openMatchedBead(bd Bd, dir, label string, match func(string) bool) (string, error) {
	open, err := bd.OpenLabeledAny(dir, label)
	if err != nil {
		return "", err
	}
	for _, b := range open {
		if match(b.Title) {
			return b.ID, nil
		}
	}
	return "", nil
}

// noteClosedDirtyOnKill is the kill's arm of §1–§2, and the last reading
// anything gets of this tree: the sweep skips a branch with nothing ahead of
// its base before it looks at the tree, so a close nobody watched that
// committed nothing and left dirt reaches the bead HERE or nowhere.
//
// It asks bd for the status rather than assuming one. A kill lands open beads
// too — the reap guard refuses that pair only as far as `--force` — and a
// persona's work in progress is not a close that did not land. Ignorance is
// reported and never guessed past, in the direction that files nothing:
// unlike the guard, nothing here is about to destroy anything, and the tree
// (which a dirty status keeps) is still there for the next reader.
func (b *HerdrBackend) noteClosedDirtyOnKill(m *HerdrMeta, t *SessionTree, o MergeOutcome) {
	if m == nil || m.Bead == "" || t == nil || len(o.Dirty) == 0 {
		return
	}
	bd := b.bd()
	is, err := bd.Show(t.Repo, m.Bead)
	if err != nil {
		b.warn("posse: %s left %d uncommitted path(s) in %s and bd could not say whether it is closed (%v) — not noted on the bead\n",
			m.Bead, len(o.Dirty), AbbrevHome(t.Path), err)
		return
	}
	if is.Status != "closed" {
		return
	}
	// bd records no close actor, so the closer is the assignee that held the
	// bead, then whoever filed it (verifyCloser), then the persona this
	// session was launched as — which is the one answer the store cannot
	// give and the meta can.
	persona := verifyCloser(is)
	if persona == "" {
		persona = m.Agent
	}
	noteClosedDirty(bd, t.Repo, m.Bead, persona, t, o, b.warn, b.warn)
}
