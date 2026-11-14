package posse

// THE JUDGE'S OWN HEALTH IS FILED, BECAUSE SILENCE WAS THE FAILURE HERE (ADR
// 0088 D6.3, from ranger-base-7idk4). landedcheck.go judges the landed tree
// and landedfile.go turns a verdict into D5's attributed bead; this file is
// the watchdog on both of them, and its subject is the DETECTOR rather than
// the tree.
//
// WHY A WATCHDOG AT ALL. On 2026-11-12 the judge took 125.3s twice and was
// killed at a 120s deadline. A deadline is spelled as SILENCE (D3: a check
// that cannot answer never answers red), so nothing was recorded, D5 never
// filed, and the only line that said so was a ⚠ in dispatch's output, which
// nobody reads. The crossing's one signal was the ABSENCE of a signal —
// which is what makes it a class rather than a bad number: D6 derives the
// judge's body from the Makefile on purpose, so the body grows with every
// door somebody adds, and a fixed kill over a growing body gets crossed
// again. D6.1 moved the kill up to a hang bound and D6.2 made the price a
// reading; this file is what READS it.
//
// D6.3's table, which is the whole of this file:
//
//	event                                    action
//	2nd consecutive unanswered run           file one -t bug P1 -l architecture
//	further unanswered, bead open            nothing (the bead already says so)
//	answered run over the ceiling, none open file the same class, with the wall
//	answered run, bead open                  one comment, "answered at <tree> in <wall>"
//
// TWO RUNS AND NOT ONE. A single silence is a stranded lock or a cage
// hiccup, which is exactly what D3's ⚠ is for; two in a row is a pattern,
// and the 2026-11-12 crossing was two of two, so it would have filed on the
// second landing.
//
// THE COUNTER IS ITS OWN RECORD (landedQuietKey) and D4's is untouched. D5's
// left-hand column is the tree last recorded GREEN and it is the far edge of
// every window a red bead names, so writing "unanswered" into that key would
// cost the attribution the ADR exists to buy.
//
// IT ASKS THE STORE ONLY WHEN IT HAS SOMETHING TO SAY, which is the one
// place this file reads D6.3's table more narrowly than its prose. Row 4 is
// reached by an answered run whose counter was not zero (a silent stretch
// just ended) or whose wall is over the ceiling (a crossing stretch is
// running) — never by an ordinary answered landing inside the price. An
// unconditional read would be one `bd list` per landing on a healthy main,
// on the pass that JOINS this goroutine, which is the cost
// TestAGreenLandingSpendsNoBdCall pins against and which buys nothing: a
// bead whose stretch has ended already carries its "answered" comment.
//
// THE DEDUPE IS landedfile.go's, through its own mechanics (openMarkedBead),
// with a marker of its own — one open bead per stretch, found by a whole
// line in the description, in the architecture lane rather than the code
// one. Reason (2) of landedfile.go's header is the one that matters here
// too: a marker survives a create whose read-back failed (ADR 0073), so an
// unconfirmed create is not filed twice.
//
// AND A COUNTER THAT COULD NOT BE WRITTEN FILES NOTHING (ranger-base-w9w37's
// rule, which D6.3 names). The silent row's dedupe is the COUNT: over a
// record that cannot be written, every later silence computes the same n in
// memory and would file again, one P1 per landing, which is the
// ranger-base-4v4r6 incident. The COMMENT row is not gated, for
// ranger-base-9xt70's reason one file over — a comment is the only place a
// verdict is written down once the warn has scrolled.

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// landedQuietKey is D6.3's one new record: `<tree> <n>`, where n counts
	// CONSECUTIVE judged runs that returned no answer and the tree is the
	// one the last of them could not judge.
	//
	// ITS OWN KEY, beside landedCheckKey and never inside it. D4's record
	// holds the last tree JUDGED and D5 reads its green value as the far
	// edge of a red window, so a third field or an "unanswered" spelling
	// there would overwrite the attribution — which is the alternative D6.3
	// rejects by name.
	//
	// SAME SHAPE AS D4's, and parsed the same way, so a human who has read
	// one git config line of this family can read the other.
	landedQuietKey = "posse.landedCheckQuiet"

	// landedQuietThreshold is D6.3's "2nd consecutive unanswered run". One
	// silence is a stranded lock or a cage hiccup and D3's ⚠ is what that
	// is for; two in a row is a pattern. The crossing this rule was written
	// for was two of two.
	landedQuietThreshold = 2

	// landedHealthLabel is the lane: this is a finding about the DESIGN's
	// own numbers — a body that outgrew its bound, or a price that wants
	// reviewing — and D6.2 says the ceiling is where the design goes back to
	// architecture. D5's bead is `-l code` because a red main is a red main;
	// a judge that cannot answer is not.
	landedHealthLabel = "architecture"

	// landedHealthMarkerPrefix opens the one line that says this rule filed
	// the bead, and it is DISTINCT from landedFileMarkerPrefix on purpose:
	// the two rules dedupe over different stretches in different lanes, and
	// a marker either could match would make a silent judge comment on a red
	// main's bead.
	landedHealthMarkerPrefix = "landed-check health: "
)

// The marker line itself is landedRule.healthMarker (landedrule.go), per REPO
// and not per base — which is the one place this differs from D5's: the
// counter is a repo-level git config record and the bead D6.3 titles says "at
// <repo>", because a check that cannot answer cannot answer about any branch.
// It is on the rule because ADR 0097 D5 gives the wall check this same
// watchdog, keyed separately, and the markers may not collide.
//
// ceilingOf is on the rule's file too, and is the same number for both: see
// landedrule.go.

// health is D6.3, called once per judged run with the verdict and the wall
// once() measured. It is called on the UNANSWERED arm too — before once()
// returns on silence — because silence is the event this whole rule exists
// for.
//
// `r` IS WHICH CHECK'S WATCHDOG THIS IS (ADR 0097 D5): its counter, its
// marker, its lane is the same lane, and the noun this file calls it by. The
// table is one table, and the wall check's silence is counted over its own
// record and filed on its own bead.
func (c *landedChecker) health(r landedRule, repo, tree string, v landVerdict, took time.Duration, said string) {
	if repo == "" || tree == "" {
		return
	}
	_, n := landedQuietRecorded(r, repo)
	if v == landUnanswered {
		c.healthSilent(r, repo, tree, n, said)
		return
	}
	c.healthAnswered(r, repo, tree, n, took)
}

// healthSilent is the top two rows: count this silence, and on the second
// consecutive one file the bead that says the judge is not judging.
func (c *landedChecker) healthSilent(r landedRule, repo, tree string, n int, said string) {
	if n == 0 {
		// The SAME clear healthAnswered does on the same reading, and for
		// the same reason: a count that reads zero carried nothing over
		// anything, whichever verdict is being counted. Without it the mark
		// is cleared on one of the two paths out of `health`, so a record
		// somebody zeroed by hand — or one landedQuietRecorded could not
		// parse, which it reads as zero deliberately — leaves the caveat
		// standing, and the next crossing of two GENUINELY consecutive
		// silences files a bead saying THEY ARE NOT CONSECUTIVE. That is
		// ranger-base-4jroi's own defect with the sign flipped: a D6.3
		// description claiming something that did not happen, sending
		// architecture to look for an answered run that never ran
		// (ranger-base-9n98o). Free, like the other one — a map delete is
		// no git write and no bd call.
		c.markQuietUnreset(r, repo, false)
	}
	// Read the remembered sentence BEFORE overwriting it: the bead carries
	// both runs' `said` lines, and this run is the second of them.
	prev := c.quietSaid[r.memKey(repo)]
	c.rememberQuietSaid(r, repo, said)
	n++
	if err := recordLandedQuiet(r, repo, tree, n); err != nil {
		// The count IS this row's dedupe (see the header), so a count that
		// did not land files nothing. The next landing re-judges and counts
		// again, which is one bead one landing late instead of one per
		// landing.
		c.warn("%s: the %s did not answer and %s could not be written (%v) — nothing is filed for it, and the next landing counts again (%s)", AbbrevHome(repo), r.Agent, r.QuietKey, err, r.HealthADR)
		return
	}
	if n < landedQuietThreshold {
		// D6.3's "two runs, not one": one silence is a stranded lock or a
		// cage hiccup, and the ⚠ once() has already printed is what D3 has
		// for it.
		return
	}
	is, st := c.healthBeadState(r, repo)
	switch st {
	case landedStretchUnknown:
		// An unknown is re-asked by the next landing for free, and a twin
		// P1 is a seat, a worktree and a suite run — landedfile.go's rule
		// on the same three answers.
		c.say("· %s: nothing was filed for the silent %s — the store could not say whether this silent stretch already has a bead, and the next landing asks again (%s)", AbbrevHome(repo), r.Agent, r.HealthADR)
		return
	case landedStretchOpen:
		// Row 2: nothing. The bead already says the judge is silent, and
		// this run is more of the same sentence.
		c.say("· %s: the %s did not answer for the %s consecutive time and %s already says so (%s)", AbbrevHome(repo), r.Agent, ordinal(n), is.ID, r.HealthADR)
		return
	}
	c.fileHealth(r, repo, tree, r.silentTitle(repo),
		c.silentDescription(r, repo, tree, n, prev, said, c.quietUnreset[r.memKey(repo)]))
}

// healthAnswered is the bottom two rows: the counter resets, an open bead is
// told the judge answered, and a wall over the ceiling with no bead open is
// filed.
func (c *landedChecker) healthAnswered(r landedRule, repo, tree string, n int, took time.Duration) {
	over := took > c.ceilingOf()
	if n == 0 {
		// Nothing on the counter, so nothing carried over an answered run
		// either: a record somebody zeroed by hand must not leave this mark
		// behind to caveat the next honest crossing. Free — a map delete is
		// no git write and no bd call, which is what the early return below
		// and TestAGreenLandingSpendsNoBdCall are about.
		//
		// healthSilent makes the same reading on its own way in, because a
		// hand-zeroed record reaches the next crossing by EITHER path and
		// the clear has to be on both (ranger-base-9n98o).
		c.markQuietUnreset(r, repo, false)
	}
	if n == 0 && !over {
		// The ordinary landing on a healthy main: answered, inside the
		// price, nothing to reset and nothing to say. It spends no git
		// write and no bd call at all, which is the header's reason for
		// asking the store only when there is something to say.
		return
	}
	c.rememberQuietSaid(r, repo, "")
	reset := true
	if n > 0 {
		// "Any answered verdict resets it" (D6.3). Written only when there
		// is something to reset, so the common case above stays free.
		if err := recordLandedQuiet(r, repo, tree, 0); err != nil {
			reset = false
			c.warn("%s: the %s answered and %s could not be reset (%v) — the next unanswered run counts from %d (%s)", AbbrevHome(repo), r.Agent, r.QuietKey, err, n, r.HealthADR)
		}
		// AND THE NEXT SILENCE MAY NOT CALL THAT COUNT CONSECUTIVE
		// (ranger-base-4jroi). Counting on from n is ranger-base-w9w37's rule
		// working — a count whose write did not land keeps counting rather
		// than files — but the count now spans THIS run, which answered. The
		// mark is what lets silentDescription say so, instead of claiming a
		// pattern and blaming a launcher restart that did not happen.
		c.markQuietUnreset(r, repo, !reset)
	}
	is, st := c.healthBeadState(r, repo)
	switch st {
	case landedStretchUnknown:
		c.say("· %s: the store could not say whether the %s has an open health bead, so this answered run is in this log and nowhere else, and the next landing asks again (%s)", AbbrevHome(repo), r.Agent, r.HealthADR)
		return
	case landedStretchOpen:
		// Row 4. Never a close: closing is architecture's, exactly as D5
		// leaves a red main's bead to the fixer.
		text := fmt.Sprintf("answered at %s in %s (%s) — closing this bead is architecture's, so the launcher does not.", abbrevSHA(tree), landedCheckWall(took), r.HealthADR)
		if over {
			text = fmt.Sprintf("answered at %s in %s, still over the %s ceiling (%s) — closing this bead is architecture's, so the launcher does not.", abbrevSHA(tree), landedCheckWall(took), landedCheckCeiling, r.CeilingADR)
		}
		if err := c.bd.Comment(repo, is.ID, text, VerifyActor); err != nil {
			c.warn("%s: %s was not commented on (%v) — %q is in this log and nowhere else", AbbrevHome(repo), is.ID, err, text)
			return
		}
		c.say("· %s: %s — %s", AbbrevHome(repo), is.ID, text)
		return
	}
	if !over {
		// Answered, inside the price, and no bead open: the counter has
		// been reset and there is nothing for architecture to read.
		return
	}
	if !reset {
		c.say("· %s: nothing was filed for the %s run — %s could not be written (⚠ above), and a rule whose record did not land files once per landing (%s)", AbbrevHome(repo), landedCheckWall(took), r.QuietKey, r.HealthADR)
		return
	}
	c.fileHealth(r, repo, tree, r.ceilingTitle(repo), c.ceilingDescription(r, repo, tree, took))
}

// healthBeadState is the three answers the store can give about this repo's
// health bead, which are landedfile.go's three and not a bool
// (ranger-base-w9w37): only landedStretchNone may be answered by filing.
func (c *landedChecker) healthBeadState(r landedRule, repo string) (BdIssue, landedStretchState) {
	is, err := c.openMarkedBead(landedHealthLabel, repo, r.healthMarker(repo))
	if err != nil {
		c.warn("%s: the open %s health bead could not be read (%v)", AbbrevHome(repo), r.Name, err)
		return BdIssue{}, landedStretchUnknown
	}
	if is.ID == "" {
		return BdIssue{}, landedStretchNone
	}
	return is, landedStretchOpen
}

// fileHealth files one bead of D6.3's class. UNASSIGNED, deliberately: D5's
// assignee is the landing whose tree first held both halves, and there is no
// such landing here — the finding is about the judge's own numbers, and ADR
// 0006 §1 wants no `-a` unless the work needs that person.
func (c *landedChecker) fileHealth(r landedRule, repo, tree, title, desc string) {
	// The cheap half of the visibility guard, as D5's filing does it: this
	// description carries a check's own output tail and a cage renderer's
	// error, so it carries whatever those carried. Warn, never refuse.
	c.a.WarnOpsContent(c.out, repo, "the "+r.Name+" health bead for "+AbbrevHome(repo), title+"\n"+desc)
	// The type and the priority are D5's own two constants, because D6.3
	// names the same two words (`-t bug` P1) and a second spelling of them
	// would be two literals to keep in step for nothing. The LABEL is the
	// one word that differs, and it has its own constant above.
	id, err := c.bd.Create(repo, BdNew{
		Title:       title,
		Description: desc,
		Labels:      []string{landedHealthLabel},
		Priority:    strconv.Itoa(landedFilePriority),
		Type:        landedFileType,
		Actor:       VerifyActor,
	})
	switch {
	case err != nil && id != "":
		// ADR 0073's shape: bd committed the bead and the confirming read
		// disagreed or could not run. The marker is what makes that safe —
		// the next silent run finds it and says nothing instead of filing.
		c.warn("%s: %s may have been filed (%q) and could not be confirmed (%v) — check it before filing by hand", AbbrevHome(repo), id, title, err)
	case err != nil:
		c.warn("%s: %q could NOT be filed (%v) — this log is the only record of it", AbbrevHome(repo), title, err)
	default:
		c.say("· %s: filed %s P%d %q, unassigned (%s)", AbbrevHome(repo), id, landedFilePriority, title, r.HealthADR)
	}
}

// silentDescription is what architecture reads first: that no landing is
// being judged at all, and the two sentences the two runs gave for it.
//
// `unreset` IS A CLAIM THIS BEAD MAY NOT MAKE WITHOUT (ranger-base-4jroi).
// The count reads as consecutive silences only while every answered run in
// between reset it; over a reset that could not be written it spans a run
// that ANSWERED, and both halves of the bead this function used to write
// were then false — a pattern that did not happen, and a launcher restart
// that did not happen either. The reader of this bead is architecture, and a
// bead that sends them looking for a restart costs a seat.
func (c *landedChecker) silentDescription(r landedRule, repo, tree string, n int, prev, said string, unreset bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", r.healthMarker(repo))
	if unreset {
		fmt.Fprintf(&b, "The %s at %s returned NO ANSWER on %d of the judged runs counted under %s, the last of them over tree %s. THEY ARE NOT CONSECUTIVE, which is what this count otherwise means: a run between them ANSWERED, and the reset that answered run owed the counter could not be written (the ⚠ is in the launcher's log). A count whose write did not land keeps counting rather than files (ranger-base-w9w37's rule), so this one carried on over that answer instead of starting again from 1. Read %d as how many of the last few judged runs were silent, and the answered one in the middle as saying the %s is unreliable rather than dead.\n\n",
			r.Name, AbbrevHome(repo), n, r.QuietKey, abbrevSHA(tree), n, r.Agent)
	} else {
		fmt.Fprintf(&b, "The %s at %s returned NO ANSWER on %d consecutive judged runs, the last of them over tree %s.\n\n",
			r.Name, AbbrevHome(repo), n, abbrevSHA(tree))
	}
	// What the silence COSTS, which is the one sentence here that is about
	// the check and not about the counter: the judge's silence is a landing
	// nobody judged, and the wall check's is a landing whose pins nobody
	// measured. Both end "and <its ratchet> files nothing", because that is
	// why this bead exists at all.
	fmt.Fprintf(&b, "While the %s is not answering, %s — a deadline and a refused cage are both spelled as SILENCE (D3), so this bead is the only thing that says so.\n\n", r.Agent, r.SilentCost)
	b.WriteString("What the runs said:\n\n")
	switch {
	case prev != "":
		fmt.Fprintf(&b, "    1. %s\n    2. %s\n\n", oneLine(prev), orElse(oneLine(said), "it said nothing"))
	case unreset:
		// Not another process's sentence: this process cleared it itself,
		// two lines above the failed reset, and that clear is right — a
		// sentence from before an answered run is not this stretch's.
		fmt.Fprintf(&b, "    1. (not recorded: the ANSWERED run between the two cleared it, which is right — a sentence from before an answered run is not this stretch's)\n    2. %s\n\n", orElse(oneLine(said), "it said nothing"))
	default:
		fmt.Fprintf(&b, "    1. (not recorded: the earlier run was judged by another launcher process, which is where this sentence is held)\n    2. %s\n\n", orElse(oneLine(said), "it said nothing"))
	}
	fmt.Fprintf(&b, "The two numbers to read it against (%s): the %s is killed at %s, which bounds a HANG and not a price, and the price at which this design goes back to architecture is %s, read off answered runs. A run killed at the bound says `no answer in %s`; anything else above is a cage, a worktree or a git read that could not be made.\n\n",
		r.BoundsADR, r.Agent, landedCheckLimit, landedCheckCeiling, landedCheckLimit)
	fmt.Fprintf(&b, r.SilentAdvice, r.Script, AbbrevHome(repo), landedCheckLimit)
	fmt.Fprintf(&b, "The launcher comments `answered at <tree> in <wall>` on this bead when the %s answers again, and does NOT close it: closing is architecture's (%s).\n", r.Agent, r.HealthADR)
	return b.String()
}

// ceilingDescription is D6.2's reading, filed. The wall is the finding, so
// the wall is the first number in it.
func (c *landedChecker) ceilingDescription(r landedRule, repo, tree string, took time.Duration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", r.healthMarker(repo))
	fmt.Fprintf(&b, "The %s at %s ANSWERED at tree %s, and it took %s — over %s of %s (%s). The verdict itself is in the pass's own log and in whatever %s filed for it; this bead is about the price.\n\n",
		r.Name, AbbrevHome(repo), abbrevSHA(tree), landedCheckWall(took), r.CeilingTitle, landedCheckCeiling, r.PriceADR, r.FileADR)
	fmt.Fprintf(&b, "WHY THAT NUMBER IS A PRICE AND NOT A TIMEOUT. The ceiling is chosen for ATTRIBUTION: D5's bead names every landing between the tree last recorded green and the red one, and a %s that answers inside the p25 landing gap keeps that window to one landing most of the time. A %s this slow widens every window it reports. Nothing was killed — the hang bound is %s and it is a separate number (D6.1) — so the %s is still answering and the detector is still working.\n\n",
		r.Agent, r.Agent, landedCheckLimit, r.Agent)
	fmt.Fprintf(&b, r.CeilingAdvice, r.Script)
	fmt.Fprintf(&b, "The launcher comments `answered at <tree> in <wall>` on this bead on every later run it has something to say about, and does NOT close it: closing is architecture's (%s).\n", r.HealthADR)
	return b.String()
}

// rememberQuietSaid holds one repo's last unanswered sentence, so the bead
// filed on the second silence can carry BOTH. In memory and not in the
// record, because D6.3 spells that record `<tree> <n>` and a sentence is
// neither: a crossing is two runs of one launcher process by construction
// (Dispatcher.Run is long-lived, ADR 0028 §1), and when it is not — the
// launcher restarted between the two — the bead says the earlier sentence is
// not recorded rather than inventing one.
//
// Touched only from the goroutine landedChecker.loop runs on, which is
// saidOwnCopy's rule and for its reason; the mutex Schedule and loop share
// is what carries the writes across one cycle's end to the next cycle's
// goroutine.
//
// KEYED BY RULE AND REPO (landedRule.memKey): one launcher judges one repo
// under both rules, and a map keyed on the repo alone would file the wall
// check's sentence on the judge's bead (ADR 0097 D5).
func (c *landedChecker) rememberQuietSaid(r landedRule, repo, said string) {
	if said == "" {
		delete(c.quietSaid, r.memKey(repo))
		return
	}
	if c.quietSaid == nil {
		c.quietSaid = map[string]string{}
	}
	c.quietSaid[r.memKey(repo)] = said
}

// markQuietUnreset holds, or clears, "this repo's count came through an
// answered run whose reset did not land" (ranger-base-4jroi). Set by
// healthAnswered and read by the silent row's description, which is the only
// thing that may not claim a consecutive pattern over it.
//
// A delete rather than a false, so an absent entry and a cleared one are one
// reading — landedQuietRecorded's rule one file over, and the same
// goroutine-ownership note as rememberQuietSaid's.
func (c *landedChecker) markQuietUnreset(r landedRule, repo string, unreset bool) {
	if !unreset {
		delete(c.quietUnreset, r.memKey(repo))
		return
	}
	if c.quietUnreset == nil {
		c.quietUnreset = map[string]bool{}
	}
	c.quietUnreset[r.memKey(repo)] = true
}

// landedQuietRecorded reads the counter. ANY other shape is no record, which
// is landedRecorded's rule and for its reason: this is one line of git
// config a human can edit, and a half-parsed count must read as zero rather
// than as a stretch nobody measured.
func landedQuietRecorded(r landedRule, repo string) (string, int) {
	out, err := git(repo, "config", "--get", r.QuietKey)
	if err != nil {
		return "", 0
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return "", 0
	}
	n, cerr := strconv.Atoi(f[1])
	if cerr != nil || n < 0 {
		return "", 0
	}
	return f[0], n
}

// recordLandedQuiet writes it. A reset is `<tree> 0` and never an `--unset`:
// `git config --unset` exits non-zero for a key that is not there, so an
// unset reset would make "already clear" indistinguishable from "the write
// failed" — and the failure of a write is the one thing this rule's filing
// is gated on.
func recordLandedQuiet(r landedRule, repo, tree string, n int) error {
	if tree == "" {
		return nil
	}
	if n < 0 {
		n = 0
	}
	_, err := git(repo, "config", r.QuietKey, tree+" "+strconv.Itoa(n))
	return err
}

// ordinal says 2nd, 3rd, 4th for a line a human reads. Small on purpose: the
// only reader is the row that says a bead already covers this silence, and
// "the 3 consecutive time" is the kind of sentence that makes a reader doubt
// the rest of the line.
func ordinal(n int) string {
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		return strconv.Itoa(n) + "st"
	case n%10 == 2:
		return strconv.Itoa(n) + "nd"
	case n%10 == 3:
		return strconv.Itoa(n) + "rd"
	}
	return strconv.Itoa(n) + "th"
}
