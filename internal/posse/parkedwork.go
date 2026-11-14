package posse

// A CLOSE WHOSE COMMIT IS PARKED OFF THE BASE GETS THE BACK-POINTER ON THE
// BEAD, from verify bead ranger-base-vuj7l's findings. This bead:
// ranger-base-yc3i3
//
// THE INCIDENT, and it is the second reading of the same class in two days.
// The push gate holds a commit the shop may not publish yet: the merge-back
// has already landed it on the base, so the hold is a RESET of the base plus
// a ref kept under refs/posse/retired so the bytes survive. Both halves are
// correct. What was missing is a line on the bead the commit was made for.
//
// MEASURED 2026-11-14 in this repo, at base e3b41b30: 9 commits under
// refs/posse/retired that no live branch carries and whose bead id heads no
// commit's subject on the base, over 6 distinct beads — and every one of
// those 6 beads is CLOSED, with a closing comment saying the work landed and
// the suite was green. Every one of the 9 shas is named by an OPEN bead
// somewhere (the hold the gate filed), so the forward direction is complete
// and the reverse direction did not exist: a reader of any of those 6 beads
// — the next verify seat, anyone asking whether a fix is in — is told it
// shipped. Two had been found by hand the day before, two by hand on the day
// (this bead), and the remaining two had been found by nobody.
//
// WHY A COMMENT AND NOT A STATUS CHANGE. A closed bead's status is the
// closer's write and the operator's to undo (ADR 0006 §2), and a reopened
// bead re-enters `bd ready` and dispatches into a new, clean tree — which is
// exactly wrong for work that already exists and is only unpublished. So
// this writes the one thing a harness may write on somebody else's close:
// a comment, under the close comment, where the false claim lives. It is
// closeddirty.go's argument (ADR 0041 §1) over a different fact, and it ends
// in the same place for the same reason.
//
// WHY A SWEEP AND NOT A SITE AT THE HOLD. Nothing in posse resets the base;
// the hold is a human's write, minutes or hours after the landing it undoes,
// and posse is not running at that instant. The only site that can see the
// result is one that LOOKS, repeatedly, which is the dispatch pass — the
// same argument landsweep.go makes for the close nobody watched.
//
// THE JOIN IS THE BEAD ID IN THE SUBJECT, which is this shop's provenance
// (AGENTS.md "Landing the plane"; ADR 0022's `git log --grep <id>` promise):
// 599 of 608 commits on the base named a bead when that line was written.
// So "did this parked commit's work land" is asked as "does any commit on
// the base head its subject with the same id", and not as a patch
// comparison. MEASURED over the same 21 refs on 2026-11-14, the two
// disagree in both directions and the subject is right both times:
//
//   - `git cherry` called a commit NOT upstream whose work had been
//     re-landed by a cherry-pick (a different patch id over a different
//     parent) — a false alarm on a bead that is genuinely in the tree.
//   - a whole-message `--grep` called a commit landed because the BASE's
//     own source names that bead id in a comment, in prose about a cost the
//     bead was filed to remove — a false silence on exactly the bead this
//     mechanism exists for.
//
// Both readings are cheap to take and neither is a record; the subject is a
// record, kept by hand by every persona and pinned by the tree's own doors.
//
// AND THE CENSUS IS "ON NO LIVE BRANCH", not "not on the base". A closed
// bead whose branch still exists unlanded is the landing sweep's business
// and is reported by it every pass; a commit no branch carries at all is
// what nothing was reading. So the question git is asked is `--not
// --branches --remotes`, and a parked ref whose commits a live branch also
// carries contributes nothing.
//
// WHAT IT COSTS A PASS. Three git children in a repo with parked refs and
// one in a repo without, then one `bd comments` per closed bead the census
// names, every pass, forever. MEASURED 2026-11-14 on this box: the ref list
// and the parked-commit walk are 7ms together, the base's subject list is
// 31ms over 2,696 commits and 382KB, and a bd read is 50ms. So today's six
// findings cost about 0.3s a pass, and the bound is the number of parked
// unlanded closed beads — which is the number this mechanism exists to
// drive to zero, not a number that grows with history. A $StateDir ledger
// would be cheaper and is refused for closeddirty.go's reason: it would be
// a second store that could disagree with bd about a bead.

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// parkedWorkPrefix opens the comment and is the head of its dedupe key, so
// nothing may precede it in the text. closeddirty.go's note on scanning the
// TEXT rather than the author applies unchanged: nothing in this repo has
// ever read BdComment.Author back from real bd.
const parkedWorkPrefix = "work parked ["

// parkedWorkKey is the comment's dedupe key: the marker and the sha set,
// sorted, inside the brackets.
//
// THE SHA SET IS IN THE KEY, which is the opposite choice from
// closedDirtyTitleKey, and the difference is what moves the tail. That one
// carries a COUNT its own handoff invites a persona to change, so a key
// holding it files a second bead for the same tree. This one carries the
// identity of the commits the line is about: a new parked copy of the same
// bead's work is a new fact, worth a new line, and the only way a reader
// learns about it. Sorted, because git's walk order is not a promise and an
// unsorted key would file the same line twice over the same set.
func parkedWorkKey(shas []string) string {
	s := make([]string, len(shas))
	copy(s, shas)
	sort.Strings(s)
	return parkedWorkPrefix + strings.Join(s, " ") + "]"
}

// parkedWorkNoted reports whether this bead already carries the line for
// exactly this sha set.
func parkedWorkNoted(cs []BdComment, key string) bool {
	for _, c := range cs {
		if strings.HasPrefix(c.Text, key) {
			return true
		}
	}
	return false
}

// parkedCommit is one commit under refs/posse/retired that no live branch
// carries and whose bead id heads no commit's subject on the base.
type parkedCommit struct {
	SHA string // the full sha, as git printed it
	Ref string // the ref it is parked at — the whole of why the bytes survive
	ID  string // the bead id that heads its subject
}

// parkedIDShape is what a leading subject token must look like to be worth
// asking bd about: a lowercase id with at least one dash, which is every
// bead prefix any store in this shop issues. It is a filter and never an
// answer — bd resolves the id, and a token that is no bead is dropped there.
// Without it, every `Merge branch ...` and `fix: ...` on a parked ref would
// cost a bd child per pass.
var parkedIDShape = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)+$`)

// parkedLeadingID is the bead id a commit subject names, or "" for a subject
// that names none. Both house spellings are one rule — `<id>: <text>` and a
// bare `<id>`, which is what a vuln-private commit message is (AGENTS.md's
// 2026-11-11 ruling).
func parkedLeadingID(subject string) string {
	tok := strings.TrimSpace(subject)
	if i := strings.IndexAny(tok, ": \t"); i >= 0 {
		tok = tok[:i]
	}
	if !parkedIDShape.MatchString(tok) {
		return ""
	}
	return tok
}

// parkedCommits censuses repo for work that is parked off base: every commit
// reachable from a ref under refs/posse/retired, carried by no live branch or
// remote, whose leading bead id heads no commit's subject on base.
//
// Read-only and bounded by the refs, so a repo with none costs one git child
// and answers nothing. base == "" is no question at all (a detached checkout
// has no base to have landed on) and answers nil.
func parkedCommits(repo, base string) ([]parkedCommit, error) {
	if base == "" {
		return nil, nil
	}
	refs, err := git(repo, "for-each-ref", "--format=%(refname)", strings.TrimSuffix(retiredTipPrefix, "/"))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(refs) == "" {
		return nil, nil
	}
	args := []string{"log", "--source", "--format=%H %S %s"}
	args = append(args, strings.Fields(refs)...)
	args = append(args, "--not", "--branches", "--remotes")
	parked, err := git(repo, args...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parked) == "" {
		return nil, nil
	}
	// The base's subjects are read only once something is parked, so the one
	// child that scales with history is not taken on the ordinary pass.
	landed, err := baseSubjectIDs(repo, base)
	if err != nil {
		return nil, err
	}
	var out []parkedCommit
	for _, line := range strings.Split(parked, "\n") {
		f := strings.SplitN(strings.TrimSpace(line), " ", 3)
		if len(f) < 3 {
			continue
		}
		id := parkedLeadingID(f[2])
		if id == "" || landed[id] {
			continue
		}
		out = append(out, parkedCommit{SHA: f[0], Ref: f[1], ID: id})
	}
	return out, nil
}

// baseSubjectIDs is the set of bead ids that head a commit subject on base —
// the shop's record of what has landed.
func baseSubjectIDs(repo, base string) (map[string]bool, error) {
	out, err := git(repo, "log", "--format=%s", base)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, s := range strings.Split(out, "\n") {
		if id := parkedLeadingID(s); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}

// parkedHolds answers, for each sha, which OPEN beads name it — the hold the
// push gate filed. A sha is matched by its abbreviation anywhere in a title
// or a description, because that is how the holds on record name it and the
// convention is kept by hand.
//
// Closed beads do not count, for OpenLabeledAny's reason: the question is
// what is still WAITING to be landed, and a closed hold is answered. The
// read is `--all` and filtered here rather than a label query, because a
// hold carries whatever label its lane needs — the two on record carry
// different ones — and the sha is the only thing they share.
func parkedHolds(bd Bd, dir string, shas []string) map[string][]string {
	all, err := bd.ListAll(dir)
	if err != nil {
		return nil
	}
	holds := map[string][]string{}
	for _, is := range all {
		if is.Status == "closed" {
			continue
		}
		text := is.Title + "\n" + is.Description
		for _, sha := range shas {
			if strings.Contains(text, shortSHA(sha)) {
				holds[sha] = append(holds[sha], is.ID)
			}
		}
	}
	return holds
}

// parkedWorkComment is the line ADR 0041 §1's shape, over this fact: what is
// parked, where it is parked, and which open bead names it.
//
// The sha, the ref and the open bead are what makes the line worth reading —
// the sha so a reader can look at the work, the ref so they can reach it
// without the reflog, and the open bead so they are reading a live bead a
// line later instead of guessing whether anyone owns this.
//
// IT SAYS "NAMED BY", NOT "HELD BY", because that is what was measured: a
// text match for the sha over the open beads, and the hold is only the most
// likely of them. A verify bead or a findings bundle that quotes the sha
// matches too, which is useful — it is the reading trail — and calling it
// the hold would be the harness asserting something it did not check.
func parkedWorkComment(base string, cs []parkedCommit, holds map[string][]string) string {
	var b strings.Builder
	shas := make([]string, 0, len(cs))
	for _, c := range cs {
		shas = append(shas, shortSHA(c.SHA))
	}
	fmt.Fprintf(&b, "%s: this bead is CLOSED and its commit(s) are on no branch.\n"+
		"%s does not carry them, and no commit on %s heads its subject with\n"+
		"%s, so the graph reads as shipped for work that is not in the tree.\n\n",
		parkedWorkKey(shas), base, base, cs[0].ID)
	for _, c := range cs {
		named := "named by NO OPEN BEAD — nothing in the queue is holding this sha"
		if h := holds[c.SHA]; len(h) > 0 {
			named = "named by open bead(s): " + strings.Join(h, ", ")
		}
		fmt.Fprintf(&b, "  %s  parked at %s\n    %s\n    `git log %s..%s`\n",
			shortSHA(c.SHA), c.Ref, named, base, c.Ref)
	}
	fmt.Fprintf(&b, "\nNothing here reopens this bead — status is the closer's write and the operator's\n"+
		"to undo (ADR 0006 §2) — and nothing here lands anything. The decision to publish\n"+
		"or to re-land is on one of the open beads above, which is where the push gate\n"+
		"files its hold; if none of them is that bead, it was never filed.")
	return b.String()
}

// NoteParkedWork is the dispatch pass's back-pointer: one comment on every
// CLOSED bead whose work is parked off the base, once per sha set, between
// every pass.
//
// An OPEN bead is left alone. Its unlanded branch is the landing sweep's
// business and is said out loud by it every pass, and the bead itself is in
// the queue where a reader will find it — the defect here is a bead that
// reads as SHIPPED, and only a closed one does.
//
// Best effort throughout and never quiet in the direction that matters,
// which is noteClosedDirty's contract: a comment that cannot be written must
// not fail the pass, and a close whose work is on no branch is a fact the
// pass owes out loud. Returns the number of beads noted.
func (a *App) NoteParkedWork(bd Bd, dirs []string, out, errw io.Writer) int {
	n := 0
	for _, dir := range dirs {
		base := repoBranch(dir)
		if base == "" {
			// A detached checkout has no base a landing could have reached,
			// so there is no question to ask here. Said by nothing extra:
			// every other reading in the pass that needs a base reports it.
			continue
		}
		cs, err := parkedCommits(dir, base)
		if err != nil {
			fmt.Fprintf(errw, "parked-work check: %s: %v\n", DirLabel(dir), err)
			continue
		}
		for _, pb := range parkedByBead(cs) {
			if noteParkedBead(bd, dir, base, pb.id, pb.commits, out, errw) {
				n++
			}
		}
	}
	return n
}

// parkedBead is one bead's parked commits, in the order git walked them.
type parkedBead struct {
	id      string
	commits []parkedCommit
}

// parkedByBead groups a census by bead, keeping git's own order so the
// comment reads newest-first the way `git log` does.
func parkedByBead(cs []parkedCommit) []parkedBead {
	var order []string
	byID := map[string][]parkedCommit{}
	for _, c := range cs {
		if _, seen := byID[c.ID]; !seen {
			order = append(order, c.ID)
		}
		byID[c.ID] = append(byID[c.ID], c)
	}
	out := make([]parkedBead, 0, len(order))
	for _, id := range order {
		out = append(out, parkedBead{id: id, commits: byID[id]})
	}
	return out
}

// noteParkedBead writes one bead's line, or nothing, and reports whether it
// wrote one.
//
// The STATUS is read first and the comments second, which is the cheap order
// for the steady state: an open bead is dropped on one read, and only a
// closed one pays the dedupe read. A bead bd cannot resolve is dropped
// silently — the leading token of a subject that is no bead at all answers
// with the same error as a store that will not talk, so this cannot tell
// them apart, and every other bd reader in the pass reports the second.
func noteParkedBead(bd Bd, dir, base, id string, cs []parkedCommit, out, errw io.Writer) bool {
	is, err := bd.Show(dir, id)
	if err != nil || is.Status != "closed" {
		return false
	}
	shas := make([]string, 0, len(cs))
	for _, c := range cs {
		shas = append(shas, shortSHA(c.SHA))
	}
	key := parkedWorkKey(shas)
	if existing, err := bd.Comments(dir, id); err != nil {
		// The read is the dedupe, not the record (commentClosedDirty's
		// rule): a graph that will not answer must not cost a closed bead
		// the correction that belongs under its close comment. A duplicate
		// is visible and a missing line is not.
		fmt.Fprintf(errw, "posse: %s could not be checked for an existing `%s` comment (%v) — commenting anyway\n",
			id, parkedWorkPrefix, err)
	} else if parkedWorkNoted(existing, key) {
		return false
	}
	holds := parkedHolds(bd, dir, parkedSHAs(cs))
	if err := bd.Comment(dir, id, parkedWorkComment(base, cs, holds), VerifyActor); err != nil {
		fmt.Fprintf(errw, "posse: %s is closed and its %d commit(s) are parked off %s, and the bead could not be told (%v) — %s\n",
			id, len(cs), base, err, cs[0].Ref)
		return false
	}
	fmt.Fprintf(out, "  ↳ %s is closed and its %d commit(s) are on no branch — noted on the bead (%s)\n",
		id, len(cs), key)
	return true
}

// parkedSHAs is the full shas of a census slice, which is what parkedHolds
// keys its answer on.
func parkedSHAs(cs []parkedCommit) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.SHA)
	}
	return out
}
