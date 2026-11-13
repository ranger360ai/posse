package posse

// ONE ATTRIBUTED BEAD PER GREEN-TO-RED STRETCH (ADR 0088 D5, from
// ranger-base-ddgfr). landedcheck.go judges the landed tree and prints the
// verdict; this file is what the verdict BECOMES. The split is the ADR's:
// D1-D4 is a reader, D5 is a filer, and the filer's whole subject is the
// TRANSITION rather than the verdict.
//
// WHY A RATCHET AND NOT A BEAD PER RED. main stays red until somebody fixes
// it, and every landing in between is judged again — so a rule that filed on
// every red would file one P1 per landing for one exit code. That is
// ranger-base-4v4r6, measured: three concurrent P1s, three worktrees and
// three suite runs for one whitespace character, because `make test` exits 2
// for every seat on the box at once and every seat is a filer. D5's table is
// therefore keyed on the transition:
//
//	last recorded   this tree   action
//	green           green       record it (landedcheck.go; nothing here)
//	green           RED         file one -t bug P1 -l code, "<base> red at <tree>"
//	red             red         one comment naming the new tree; no new bead
//	red             green       comment "green at <tree>"; closing is the fixer's
//	no answer       any         nothing recorded, nothing filed
//
// WHO IT IS ASSIGNED TO, and why the assignment is not an accusation. The
// window is every landing between the tree last recorded green and this one,
// and the NEWEST of them is the assignee: that landing is the one whose tree
// first held both halves, which is ADR 0087 D5's merge-back rule read one
// level out. Every seat in the window ran the doors on its own base and was
// green there — the incident this ADR answers is exactly that
// (ranger-base-s64a5 shipped the pin, ranger-base-in8dr shipped the arms
// that fire it, neither seat ever had both) — so the bead says so in its own
// body rather than leaving the assignment to read as carelessness.
//
// THE OPEN BEAD IS FOUND IN THE STORE, BY A MARKER, and not by a second
// config record beside D4's. Three reasons, in the order of what they cost
// when they are wrong: (1) D4 spells its record `<tree-id> <green|red>`, and
// a third field or a second key is a record to keep in step with the first
// in a process that can die between two `git config` writes; (2) a marker
// survives a create whose READ-BACK failed (ADR 0073) — the bead is in the
// store, the next red finds it, and no twin is filed; (3) it is the house
// pattern, because verify-after dedupes the same way and for the same reason
// (verifiedSources, verifyafter.go).
//
// IT FILES NO `discovered-from:` EDGE, deliberately. ADR 0006 §1's edge is a
// handoff from a BEAD, and what discovered this is a landing. The window
// names every bead in it in the description, which is the provenance that
// survives — and a `--deps` on a create is the write bd loses: against a
// tangled closure the daemon commits the issue and outruns the client, so
// the create exits 1 with the bead already filed and the edge missing
// (ranger-base-muoo). A red main must not turn into two beads over an edge
// nothing reads.
//
// EVERY WRITE IS READ BACK, and that is Bd.Create's and Bd.Comment's own
// doing rather than a loop here (ADR 0073 B3/D3): both verbs confirm through
// a second read and return BdWriteNotLandedError / BdWriteUnverifiedError
// when the store disagrees with the exit code. What this file adds is that
// such an error is SAID and never swallowed — a landed check whose bead was
// lost on the wire must not read, in the pass's own log, like one that filed.

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// landedFileLabel, landedFileType and landedFilePriority are D5's three
	// words for the bead it files: `-t bug` P1 `-l code`. Hard-coded rather
	// than config, because D5 names them and because "code" is already this
	// harness's shipped lane label (DefaultVerifyLabels[0]) — a key nobody
	// sets would be a second spelling of the same default.
	landedFileLabel    = "code"
	landedFileType     = "bug"
	landedFilePriority = 1

	// landedFileMarkerPrefix opens the ONE line of a filed bead's
	// description that says this rule filed it. It is the dedupe of record
	// (see the header), so it is matched as a whole line and every field
	// that could carry a newline into the description is flattened first —
	// a commit subject is attacker-shaped text in a repo that takes
	// branches, and a crafted one must not be able to forge a marker and
	// make the next red comment on a bead of its choosing.
	landedFileMarkerPrefix = "landed-check: "

	// landedWindowMax bounds the walk back to the last green tree. A window
	// this long means main has been red across fifty landings, which is not
	// a window anybody reads — past it the bead says the count is a FLOOR
	// and names what it has.
	landedWindowMax = 50

	// landedAttributeMax bounds the bd reads one filing spends. The window
	// is normally one to five landings; the cap is what keeps a pathological
	// window off the pass that JOINS this goroutine, because every `bd show`
	// can spend BdTimeout before it answers (beads.go). Past it the older
	// landings are named by commit alone and the bead says so.
	landedAttributeMax = 12
)

// landedFileMarker is the marker line for one repo's base. It carries both,
// because a beads store can serve more than one repo here — two of this
// instance's three stores are reached through a `.beads/redirect` — so "the
// open bead" has to be the open bead for THIS branch of THIS repo.
func landedFileMarker(repo, base string) string {
	return fmt.Sprintf("%s%s %s (ADR 0088 D5)", landedFileMarkerPrefix, AbbrevHome(repo), base)
}

// landedLanding is one commit in the window: what git says, and what the
// store says about it. Bead and Persona are empty until attribute fills
// them, and an empty Bead means "no id this store knows" and never "the
// subject had no prefix" — landedSubjectBead and attribute between them
// collapse those two cases on purpose, because the bead is read by a human
// who wants to know who to talk to and both answers are "nobody this store
// can name".
type landedLanding struct {
	SHA     string
	Tree    string
	Subject string
	Bead    string
	Persona string
}

// file is D5's table, called once per judged tree with the verdict that was
// just recorded and the one that was on record before it.
//
// THE ORDER IS RECORD FIRST, FILE SECOND, and once() has it that way for a
// reason worth stating here: if the record is written and the filing fails,
// the next landing reads red->red, finds no open bead and files — one bead,
// one landing late. If the filing happened first and the record write failed,
// the next landing would read green->red over a stretch that already has a
// bead and file a second one. The cheap failure is the one that happens.
//
// `had` and `prev` are the record as it read BEFORE this run — once() has
// them already, for D4's cache, and they are the only source of the last
// green tree: by the time this runs the record names this tree instead.
//
// AND `recorded` SAYS WHETHER THE RECORD WAS WRITTEN, because the green→red
// row is the one row whose dedupe IS that record (ranger-base-w9w37). The
// marker in the store dedupes the other rows — it is what makes a second red
// comment instead of file — but a green→red transition files whatever is
// already open, by construction: a bead stays open after its stretch ends
// (closing it is the fixer's), so "an open bead exists" cannot tell a NEW
// stretch from a repeat reading of this one. Only the record can, and a `git
// config` write that lost `.git/config.lock` leaves it naming the stale GREEN
// tree — so every later red reads green→red again and files again, one P1 per
// landing, which is the ranger-base-4v4r6 incident this rule exists to end.
// MEASURED 2026-11-11 (ranger-base-w9w37): three red landings judged over a
// held lock, three open P1s.
//
// So a verdict that could not be recorded files NOTHING and says so. The next
// landing re-judges the same tree — which is what D4's cache does everywhere
// else — and the bead arrives when the record does, one landing late, instead
// of once per landing. The comment rows are not gated: their dedupe is the
// marker, which the record has no part in.
func (c *landedChecker) file(repo, base, tip, tree, had string, prev, v landVerdict, said string, recorded bool) {
	switch {
	case v == landUnanswered:
		// D5's bottom row, and it is never reached from once() — which
		// returns on silence before it gets here. Spelled anyway, because
		// the one thing this file may not do is turn "no answer" into a P1.
		return
	case v == landGreen && prev != landRed:
		return // green -> green, and the first ever green: the record is the whole of it
	case v == landGreen:
		c.commentStretch(repo, base, fmt.Sprintf("green at %s (%s, ADR 0088 D5) — closing this bead is the fixer's job, so the launcher does not.", abbrevSHA(tree), abbrevSHA(tip)),
			"main went green and there is no open bead to say so on")
	case prev == landRed:
		// red -> red: one comment naming the new tree, no new bead. When the
		// stretch's bead is GONE — a fixer closed it while main was still
		// red, or the create that was meant to file it lost its write and
		// its bead too — there is nothing to comment on and nobody is
		// attributed, so this files. That is the degenerate half of this
		// row and not a second rule: it leaves one OPEN bead per red
		// stretch, which is what D5's last sentence is for.
		//
		// GONE IS ONE OF THREE THINGS THE STORE CAN SAY, and only this one
		// may be answered by filing (ranger-base-w9w37). A read that FAILED
		// says nothing about whether this stretch has a bead; a bead found
		// OPEN whose comment failed says it has one. A bool collapsed all
		// three into "did not comment", so a locked database or a failed
		// `bd comments add` filed a twin P1 whose body read "the bead for
		// this red stretch is not open any more" — false in the third case,
		// which the line above had just disproved, and unknown in the second.
		switch c.commentStretch(repo, base, fmt.Sprintf("still red at %s (%s), %s — ADR 0088 D5", abbrevSHA(tree), abbrevSHA(tip), said), "") {
		case landedStretchOpen:
			return
		case landedStretchUnknown:
			// The ⚠ commentStretch printed is the record of this verdict, and
			// it must not also become a P1: an unknown is re-asked by the
			// next landing for free, and a twin is a seat, a worktree and a
			// suite run.
			c.say("· %s: nothing was filed for the red at %s — the store could not say whether this red stretch already has a bead, and the next landing asks again (ADR 0088 D5)", AbbrevHome(repo), abbrevSHA(tree))
			return
		}
		// landedStretchNone: the store answered, and this stretch has no open
		// bead. The degenerate half, and the only one that files.
		c.fileRed(repo, base, tip, tree, had, prev, said, "the bead for this red stretch is not open any more")
	default:
		// green -> red, and the first ever red: D5's filing row, gated on the
		// record for the reason this function's header gives. The record is
		// this row's whole dedupe, so a filing over a record that was never
		// written is one P1 per landing.
		if !recorded {
			c.say("· %s: nothing was filed for the red at %s — the verdict could not be recorded (⚠ above), so this green→red reading is the stale record's and not a transition; the next landing re-judges this tree (ADR 0088 D5)", AbbrevHome(repo), abbrevSHA(tree))
			return
		}
		c.fileRed(repo, base, tip, tree, had, prev, said, "")
	}
}

// landedStretchState is what the store could say about this stretch's open
// bead. Three answers and not a bool, because exactly one of them may be
// answered by filing a bead (ranger-base-w9w37) — see file's red→red arm.
type landedStretchState int

const (
	// landedStretchUnknown: the store could not be read, so whether this
	// stretch has an open bead is not known. Never a reason to file.
	landedStretchUnknown landedStretchState = iota
	// landedStretchNone: the store answered and no OPEN bead carries this
	// stretch's marker. The one answer that files.
	landedStretchNone
	// landedStretchOpen: the bead is there and open. Whether the comment
	// LANDED is a separate question — said in the log either way, and not
	// this one, which is only whether the stretch already has a bead.
	landedStretchOpen
)

// commentStretch puts one comment on the open bead for this repo and base,
// and reports what the STORE said about that bead rather than whether the
// comment landed — the two are different facts and the caller's question is
// the first one (ranger-base-w9w37). `absent` is what to say when there is
// no bead; empty says nothing, for the caller that has something better to do
// about it than print.
func (c *landedChecker) commentStretch(repo, base, text, absent string) landedStretchState {
	is, err := c.openStretchBead(repo, base)
	if err != nil {
		c.warn("%s: the open landed-check bead could not be read, so %q was not recorded on it (%v)", AbbrevHome(repo), text, err)
		return landedStretchUnknown
	}
	if is.ID == "" {
		if absent != "" {
			c.say("· %s: %s", AbbrevHome(repo), absent)
		}
		return landedStretchNone
	}
	if err := c.bd.Comment(repo, is.ID, text, VerifyActor); err != nil {
		c.warn("%s: %s was not commented on (%v) — the verdict is in this log and nowhere else", AbbrevHome(repo), is.ID, err)
		return landedStretchOpen
	}
	c.say("· %s: %s — %s", AbbrevHome(repo), is.ID, text)
	return landedStretchOpen
}

// openStretchBead is the open bead this rule filed for this repo and base,
// or a zero issue when there is none. The NEWEST one when there are several:
// a green->red transition files whatever is already open (D5's table says
// file, and a stretch that ended is over however long its bead stays open),
// so the newest marker-carrying bead is this stretch's by construction.
func (c *landedChecker) openStretchBead(repo, base string) (BdIssue, error) {
	return c.openMarkedBead(landedFileLabel, repo, landedFileMarker(repo, base))
}

// openMarkedBead is the mechanics of that dedupe, over one lane and one
// marker: the NEWEST open bead in `label` whose description carries `marker`
// as a whole line, or a zero issue when there is none.
//
// ONE IMPLEMENTATION, because D6.3's health bead dedupes the same way over a
// different lane and a different marker (landedhealth.go), and the only
// thing worth keeping in step between the two is this: which bead counts
// when there are several, and what "carries the marker" means.
func (c *landedChecker) openMarkedBead(label, repo, marker string) (BdIssue, error) {
	open, err := c.bd.OpenLabeledAny(repo, label)
	if err != nil {
		return BdIssue{}, err
	}
	best := BdIssue{}
	for _, is := range open {
		if !landedDescriptionCarries(is.Description, marker) {
			continue
		}
		if best.ID == "" || is.Created.After(best.Created) {
			best = is
		}
	}
	return best, nil
}

// landedDescriptionCarries is the marker match: a WHOLE line, so nothing a
// commit subject or a check's output tail can carry inside a longer line
// counts as one.
func landedDescriptionCarries(desc, marker string) bool {
	for _, ln := range strings.Split(desc, "\n") {
		if strings.TrimSpace(ln) == marker {
			return true
		}
	}
	return false
}

// fileRed files D5's one bead. `why` is the extra sentence for the
// degenerate red->red arm, empty on the ordinary green->red one.
func (c *landedChecker) fileRed(repo, base, tip, tree, had string, prev landVerdict, said, why string) {
	// THE FAR EDGE OF THE WINDOW, and the one thing only the caller knows.
	// The record has already been overwritten with this red by the time this
	// runs (see file's header on the order), so the green tree is gone from
	// it — `had` is the record as it read before the run, and it is the last
	// green tree exactly when the verdict it carried was green. On the
	// degenerate red->red arm there IS no green tree on record: D4 keeps one
	// entry and not a history (docs/notes.d/ranger-base-5fond.md), so the
	// window is unbounded and the bead says so rather than guessing an edge.
	green := ""
	if prev == landGreen {
		green = had
	}
	window, note := landedWindow(repo, tip, green)
	note = joinNotes(note, c.attribute(repo, window))
	title := fmt.Sprintf("%s red at %s", base, abbrevSHA(tree))
	desc := c.redDescription(repo, base, tip, tree, green, said, why, note, window)
	// The cheap half of the visibility guard, as verify-after does it
	// (rangerhq-hrz): this description is built out of a check's output and
	// a repo's commit subjects, so it carries whatever those carried. Warn,
	// never refuse — a red main that goes unfiled because its tail named a
	// path is worse than one filed and flagged.
	c.a.WarnOpsContent(c.out, repo, "the landed-check bead for "+abbrevSHA(tree), title+"\n"+desc)
	assignee := ""
	if len(window) > 0 {
		assignee = window[0].Persona
	}
	id, err := c.bd.Create(repo, BdNew{
		Title:       title,
		Description: desc,
		Labels:      []string{landedFileLabel},
		Assignee:    assignee,
		Priority:    strconv.Itoa(landedFilePriority),
		Type:        landedFileType,
		Actor:       VerifyActor,
	})
	switch {
	case err != nil && id != "":
		// ADR 0073's shape: bd committed the bead and the confirming read
		// disagreed or could not run. Named, because the next landing's
		// red->red arm will find it by its marker and comment rather than
		// file a twin — and if it did NOT land, that arm files.
		c.warn("%s: %s may have been filed for the red at %s and could not be confirmed (%v) — check it before filing by hand", AbbrevHome(repo), id, abbrevSHA(tree), err)
	case err != nil:
		c.warn("%s: the red at %s could NOT be filed (%v) — this log is the only record of it", AbbrevHome(repo), abbrevSHA(tree), err)
	default:
		c.say("· %s: filed %s P%d %q%s", AbbrevHome(repo), id, landedFilePriority, title, landedAssignedTo(assignee))
	}
}

func landedAssignedTo(assignee string) string {
	if assignee == "" {
		return ", unassigned"
	}
	return " for " + assignee
}

// redDescription is what the fixer reads first: the verdict, the cause, and
// the window — in that order, because the first question is "what is broken"
// and the second is "which two landings met".
func (c *landedChecker) redDescription(repo, base, tip, tree, green, said, why, note string, window []landedLanding) string {
	var b strings.Builder
	// The marker, first line, on its own (see the header): this is the
	// dedupe of record and the next red reads it back.
	fmt.Fprintf(&b, "%s\n\n", landedFileMarker(repo, base))
	fmt.Fprintf(&b, "%s is RED at %s (tree %s) in %s, judged by `%s` after a landing, outside the lock (ADR 0088 D1/D2).\n\n",
		base, abbrevSHA(tip), abbrevSHA(tree), AbbrevHome(repo), landedCheckScript)
	fmt.Fprintf(&b, "The check's last lines:\n\n    %s\n\n", orElse(oneLine(said), "it said nothing"))
	if why != "" {
		fmt.Fprintf(&b, "Filed although this is not a green→red transition: %s (ADR 0088 D5's red→red row).\n\n", why)
	}
	b.WriteString(landedWindowSection(green, note, window))
	fmt.Fprintf(&b, "\nWhat to do: reproduce with `%s` on a fresh %s — it is the prerequisites of `make test`, so a seat hits the same red before any suite arm runs. ", landedCheckScript, base)
	b.WriteString("Fix it on a branch and land it as usual. ")
	fmt.Fprintf(&b, "The launcher comments `green at <tree>` on this bead when %s is green again and does NOT close it: closing is the fixer's job (ADR 0088 D5).\n", base)
	return b.String()
}

// landedWindowSection is the attribution, and the paragraph that says what
// the assignment means. It is prose on purpose: the assignee did not break
// anything by itself, and a bead that only names a persona gets read as a
// bead that says they did.
func landedWindowSection(green, note string, window []landedLanding) string {
	var b strings.Builder
	if green != "" {
		fmt.Fprintf(&b, "LANDINGS in this window — every commit between %s, the tree last recorded green, and this one, newest first:\n\n", abbrevSHA(green))
	} else {
		b.WriteString("LANDINGS in this window, newest first:\n\n")
	}
	if len(window) == 0 {
		b.WriteString("- (none: git named no commit for this tip)\n")
	}
	for _, l := range window {
		fmt.Fprintf(&b, "- %s %s — %s\n", abbrevSHA(l.SHA), landedWho(l), orElse(l.Subject, "(no subject)"))
	}
	if note != "" {
		fmt.Fprintf(&b, "\n(%s)\n", note)
	}
	if len(window) == 0 {
		return b.String()
	}
	newest := window[0]
	fmt.Fprintf(&b, "\nAssigned to %s, the persona of the NEWEST landing above (%s). ",
		orElse(newest.Persona, "nobody — that landing names no persona this store knows"), abbrevSHA(newest.SHA))
	b.WriteString("That is ADR 0088 D5's rule and it is not a finding about care: ")
	b.WriteString("every seat in the window ran the doors on ITS OWN base, which held only its own half, and was green there. ")
	b.WriteString("The newest landing is simply the one whose tree first held them all. ")
	b.WriteString("Read the window as the list of halves that met, and if the fix belongs to another lane, hand it off (ADR 0006 §1).\n")
	return b.String()
}

// landedWho is one landing's attribution for a line a human reads.
func landedWho(l landedLanding) string {
	switch {
	case l.Bead != "" && l.Persona != "":
		return fmt.Sprintf("%s (%s)", l.Bead, l.Persona)
	case l.Bead != "":
		return fmt.Sprintf("%s (nobody holds it)", l.Bead)
	default:
		return "(no bead id this store knows)"
	}
}

// landedWindow is every commit between the tree last recorded green
// (exclusive) and tip (inclusive), newest first, with the note that says
// when the list is a floor rather than the window.
//
// IT IS BOUNDED BY A TREE AND NOT A COMMIT because that is what D4 records,
// and the walk is how the one is turned into the other: a tree this rule
// recorded green was a tip of this branch, so it is in the log. When it is
// not — the record came from a history that was rewritten, or from before
// this branch — the walk runs out and says so, which is the honest answer
// and not an empty window.
func landedWindow(repo, tip, green string) ([]landedLanding, string) {
	// Nothing green on record: there is no far edge, so the window is the one
	// landing that was just judged and the bead says that is all it can
	// bound. NOT fifty commits of a history nobody has a green reading for,
	// and not a walk bounded by the tip's parent either — a parent is a
	// guess about where the stretch began, and on a root commit there is no
	// parent to guess from.
	limit, unbounded := landedWindowMax+1, ""
	if green == "" {
		limit, unbounded = 1, "no tree was on record green before this red, so the window could not be bounded — the landing above is the one that was judged, and whatever came before it is not named"
	}
	out, err := git(repo, "log", "--format=%H %T %s", "-n", strconv.Itoa(limit), tip)
	if err != nil {
		return nil, fmt.Sprintf("the window could not be read out of git (%s)", gitSaid(err))
	}
	var window []landedLanding
	for _, ln := range strings.Split(out, "\n") {
		f := strings.SplitN(strings.TrimSpace(ln), " ", 3)
		if len(f) < 2 {
			continue
		}
		if green != "" && f[1] == green {
			return window, ""
		}
		if len(window) == landedWindowMax {
			return window, fmt.Sprintf("more than %d landings since %s was recorded green, so this list is a FLOOR and not the window", landedWindowMax, abbrevSHA(green))
		}
		l := landedLanding{SHA: f[0], Tree: f[1]}
		if len(f) == 3 {
			l.Subject = oneLine(f[2])
		}
		window = append(window, l)
	}
	if unbounded != "" {
		return window, unbounded
	}
	return window, fmt.Sprintf("%s, the tree last recorded green, is not among this tip's last %d commits — this list is every commit the walk read and not the window between the two", abbrevSHA(green), landedWindowMax)
}

// attribute fills in each landing's bead and persona, and returns the note
// that says what it could not.
//
// THE STORE IS THE AUTHORITY ON WHETHER A PREFIX IS A BEAD ID. The subject
// convention here is `<bead-id>: <what changed>` — 599 of 608 commits on
// main (AGENTS.md) — but "fix: a typo" has the same shape, and a filing
// that invented bead ids out of English would attribute a red to a word.
// So the prefix is a CANDIDATE and `bd show` is the answer: the id appears
// on the bead only when the store resolves it to itself (Bd.Show refuses a
// prefix collision for us), and the persona is the assignee it names, else
// whoever filed it — verifyCloser's reading, because this is the same
// question about a different event.
func (c *landedChecker) attribute(repo string, window []landedLanding) string {
	if len(window) == 0 {
		return ""
	}
	if !c.bd.Available() {
		return "bd could not be run here, so no landing above names a bead or a persona"
	}
	type ref struct {
		known   bool
		persona string
	}
	seen, spent := map[string]ref{}, 0
	for i := range window {
		id := landedSubjectBead(window[i].Subject)
		if id == "" {
			continue
		}
		r, cached := seen[id]
		if !cached {
			if spent == landedAttributeMax {
				return fmt.Sprintf("only the newest %d landings were looked up in bd; the older ones above are named by commit alone", landedAttributeMax)
			}
			spent++
			is, err := c.bd.Show(repo, id)
			if IsBdHang(err) {
				return fmt.Sprintf("bd stopped answering while this window was being attributed (%v), so the landings above may be named by commit alone", err)
			}
			r = ref{known: err == nil && is.ID == id, persona: verifyCloser(is)}
			seen[id] = r
		}
		if r.known {
			window[i].Bead, window[i].Persona = id, r.persona
		}
	}
	return ""
}

// landedSubjectBead is the candidate bead id at the head of a commit
// subject, "" when the subject is not that shape. A candidate and never an
// answer: see attribute.
func landedSubjectBead(subject string) string {
	i := strings.Index(subject, ": ")
	if i <= 0 {
		return ""
	}
	id := subject[:i]
	if !beadIDRe.MatchString(id) {
		return ""
	}
	return id
}

// joinNotes puts two of these parenthetical honesty notes in one, so the
// description carries at most one of them.
func joinNotes(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}
