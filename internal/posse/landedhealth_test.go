//go:build posse_arm2

package posse

// ADR 0088 D6.3 at the site it files: the judge's OWN health is a bead, and
// the reason it has to be is that the failure it answers was SILENCE
// (ranger-base-lhiy1, from ranger-base-7idk4; landedhealth.go's header has
// the incident and the table).
//
// THE FIXTURE DRIVES landedChecker.once, over the fake judge
// landedcheck_test.go already has, for landedfile_test.go's reason: what
// D6.3 is about is a run's OUTCOME over several runs — two silences in a
// row, an answered run after them, an answered run that cost too much — and
// a test that called `health` with a verdict and a Duration made up beside
// it would pin the table and nothing about the thing the table is read
// from. Every arm here schedules real runs and lets landedcheck.go measure
// them.
//
// ONE SEAM IS NEW AND IT IS THE PRICE (landedChecker.ceiling). A 240s wall
// cannot be reached inside a test, so the arm D6.3 exists to file — an
// answered run OVER the ceiling — has no spelling without it. It is the
// same kind of seam as `cage`: a field with a zero default, so production
// reads the constant, and TestTheCeilingSeamDefaultsToTheShippedPrice holds
// that. It can only change WHEN this rule files; the kill
// (landedCheckLimit) has no seam at all.
//
// WHAT THE CAGE DOES HERE. D3 spells every unanswerable run as silence, and
// a refused cage is the cheapest honest one — the same stub
// landedcheck_test.go's own no-answer arm uses. `lhSilentCage` only gives
// each refusal a DIFFERENT sentence, because "carrying both `said` lines" is
// a claim two identical sentences cannot measure.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// lhSilentCage is a cage that refuses with a different sentence each time,
// so the bead's two `said` lines are distinguishable. Called only from the
// goroutine landedChecker.loop runs on, which is where the production
// renderer is called from too.
func lhSilentCage(said ...string) landedCageFn {
	n := 0
	return func(*App, string, string, []string, []string) (landedCage, error) {
		s := said[len(said)-1]
		if n < len(said) {
			s = said[n]
		}
		n++
		return landedCage{}, Die("%s", s)
	}
}

// The two sentences the silent arms refuse with, and the third for the run
// nobody reads.
const (
	lhSaidOne   = "no cage renders on this host (fixture one)"
	lhSaidTwo   = "the run dir could not be written (fixture two)"
	lhSaidThree = "something else again (fixture three)"
)

// lhBeads is every open bead in D6.3's lane, which is NOT D5's — that
// separation is itself a claim (TestASilentJudgeAndARedMainAreTwoBeadsInTwoLanes).
func lhBeads(t *testing.T, c *landedChecker, repo string) []BdIssue {
	t.Helper()
	open, err := c.bd.OpenLabeledAny(repo, landedHealthLabel)
	if err != nil {
		t.Fatalf("the %s lane could not be read back: %v", landedHealthLabel, err)
	}
	return open
}

// lhFiled is the one health bead, and a failure when there is not exactly
// one — "one open bead per stretch" is the whole dedupe claim, so every arm
// asserts the count rather than picking a row out of a list.
func lhFiled(t *testing.T, c *landedChecker, repo string) BdIssue {
	t.Helper()
	open := lhBeads(t, c, repo)
	if len(open) != 1 {
		t.Fatalf("%d open %s bead(s), want exactly 1 (ADR 0088 D6.3): %s", len(open), landedHealthLabel, ldfTitles(open))
	}
	return open[0]
}

// lhCreateFields is the tail of the `bd create` argv for a health bead:
// `-t bug`, P1, `-l architecture`, and NO `-a`. The assignee is absent on
// purpose — D5 attributes a red main to the landing whose tree first held
// both halves, and a judge that will not answer has no such landing.
func lhCreateFields() string {
	return fmt.Sprintf(" -l %s -p %d -t %s", landedHealthLabel, landedFilePriority, landedFileType)
}

// lhQuiet is the counter as the record holds it.
func lhQuiet(t *testing.T, repo string) int {
	t.Helper()
	_, n := landedQuietRecorded(repo)
	return n
}

// THE HEADLINE, and done-when (i): two consecutive unanswered runs file ONE
// -t bug P1 -l architecture bead carrying BOTH `said` lines — and ONE does
// not. The second half is the arm that makes the first mean something: a
// threshold of one would file on every stranded lock and every cage hiccup,
// which is what D3's ⚠ is already for, and it would still leave exactly one
// bead at the end of this test.
func TestTwoConsecutiveSilencesFileOneArchitectureBeadCarryingBothSaidLines(t *testing.T) {
	t.Parallel()
	c, out, repo, log := ldfFixture(t)
	c.cage = lhSilentCage(lhSaidOne, lhSaidTwo)
	tree := revTreeID(repo, mustGit(t, repo, "rev-parse", "HEAD"))

	// ONE silence: counted, and nothing filed.
	ldfJudge(t, c, repo)
	if n := lhQuiet(t, repo); n != 1 {
		t.Fatalf("%s counted %d after one unanswered run, want 1 — the count is this rule's whole dedupe", landedQuietKey, n)
	}
	if open := lhBeads(t, c, repo); len(open) != 0 {
		t.Fatalf("one silence filed %s — D6.3 is TWO runs, because a single silence is a stranded lock or a cage hiccup and D3's ⚠ is what that is for", ldfTitles(open))
	}

	// TWO: the pattern, and the bead.
	ldfJudge(t, c, repo)
	if n := lhQuiet(t, repo); n != 2 {
		t.Fatalf("%s counted %d after two unanswered runs, want 2", landedQuietKey, n)
	}
	is := lhFiled(t, c, repo)
	if want := "landed check silent at " + AbbrevHome(repo); is.Title != want {
		t.Errorf("the bead is titled %q, want %q", is.Title, want)
	}
	if !hasLabel(is.Labels, landedHealthLabel) {
		t.Errorf("the bead carries %v, want the %s lane — a judge that will not answer is a finding about the design's own numbers, not about a red tree (ADR 0088 D6.3)", is.Labels, landedHealthLabel)
	}
	if want := lhCreateFields(); !strings.Contains(ldfArgv(t), want) {
		t.Errorf("no create carried %q — see ldfArgv on why this reads the argv:\n%s", want, ldfArgv(t))
	}
	if strings.Contains(ldfArgv(t), " -a ") {
		t.Errorf("the health bead was assigned to somebody — there is no landing to attribute a silent judge to (ADR 0006 §1):\n%s", ldfArgv(t))
	}
	for _, want := range []string{
		lhSaidOne, lhSaidTwo, // BOTH sentences: one run's is not a pattern
		abbrevSHA(tree),
		landedHealthMarker(repo),    // the dedupe of record
		landedCheckLimit.String(),   // the hang bound, so a reader can tell a kill from a refusal
		landedCheckCeiling.String(), // and the price, which is the other number
		"no landing is judged",
		"closing is architecture's",
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	if s := out.String(); !strings.Contains(s, is.ID) {
		t.Errorf("the pass did not say which bead it filed:\n%s", s)
	}
	// And the judge never ran at all, which is what a refused cage means
	// (D3 has no uncaged arm) — so nothing above was read over a run that
	// quietly happened.
	if n := len(landedRuns(t, log)); n != 0 {
		t.Errorf("%d judge run(s) over a refused cage", n)
	}
}

// Done-when (ii): a third silence adds NOTHING — no second bead and no
// comment. D6.3's second row is "the bead already says so", and the cost of
// getting it wrong is the ranger-base-4v4r6 shape with a new subject: one P1
// per landing for one broken detector.
func TestAThirdSilenceAddsNoSecondBeadAndNoComment(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	c.cage = lhSilentCage(lhSaidOne, lhSaidTwo, lhSaidThree)

	ldfJudge(t, c, repo)
	ldfJudge(t, c, repo)
	first := lhFiled(t, c, repo)
	before := out.Len()

	ldfJudge(t, c, repo)

	if is := lhFiled(t, c, repo); is.ID != first.ID {
		t.Fatalf("the third silence filed %s beside %s — one open bead per stretch (ADR 0088 D6.3)", is.ID, first.ID)
	}
	if n := lhQuiet(t, repo); n != 3 {
		t.Errorf("%s counted %d after three unanswered runs, want 3 — the count keeps going even though the bead does not", landedQuietKey, n)
	}
	if cs := ldfComments(t, c, repo, first.ID); len(cs) != 0 {
		t.Errorf("%d comment(s) on %s after a further silence, want none: %s", len(cs), first.ID, ldfCommentTexts(cs))
	}
	// It is not silent about it either: the log says the bead already
	// covers this run, which is how a reader of the pass knows the rule
	// looked rather than that it never fired.
	if s := out.String()[before:]; !strings.Contains(s, first.ID) {
		t.Errorf("the third silence said nothing about %s:\n%s", first.ID, s)
	}
	if s := out.String()[before:]; strings.Contains(s, "filed") {
		t.Errorf("the third silence said it filed something:\n%s", s)
	}
}

// Done-when (iii): an ANSWERED run over the ceiling files, with its wall in
// the bead. This is the row that makes D6.2 a reading rather than a hope —
// the whole 2026-11-12 defect was one number doing two jobs, so that a
// crossing could only ever be reported by the silence it caused.
func TestAnAnsweredRunOverTheCeilingFilesWithItsWall(t *testing.T) {
	t.Parallel()
	c, out, repo, log := ldfFixture(t)
	c.ceiling = time.Nanosecond // any real run is over this; see the header
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	ldfJudge(t, c, repo)

	// It ANSWERED: the verdict is on record and the judge really ran. A
	// filing over a run that was killed would be the silent row wearing
	// this row's title.
	if tree, v := landedCheckRecorded(repo); tree != revTreeID(repo, tip) || v != landGreen {
		t.Fatalf("the run did not answer green (%s %s), so this arm is not the one being measured", tree, v)
	}
	if n := len(landedRuns(t, log)); n != 1 {
		t.Fatalf("%d judge run(s), want 1", n)
	}
	is := lhFiled(t, c, repo)
	if want := "landed check over its D6 ceiling at " + AbbrevHome(repo); is.Title != want {
		t.Errorf("the bead is titled %q, want %q", is.Title, want)
	}
	if !landedWallSaid.MatchString(is.Description) {
		t.Errorf("the bead carries no wall, which is the whole finding — D6.2's ceiling is READ off answered runs:\n%s", is.Description)
	}
	for _, want := range []string{
		landedHealthMarker(repo),
		abbrevSHA(revTreeID(repo, tip)),
		landedCheckLimit.String(), // nothing was killed, and the bead says which number did not fire
		"closing is architecture's",
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	if want := lhCreateFields(); !strings.Contains(ldfArgv(t), want) {
		t.Errorf("no create carried %q:\n%s", want, ldfArgv(t))
	}
	if s := out.String(); !strings.Contains(s, is.ID) {
		t.Errorf("the pass did not say which bead it filed:\n%s", s)
	}
}

// …and a second crossing COMMENTS on it rather than filing a twin, which is
// the same dedupe the silent rows have and the reason the marker exists.
func TestASecondCrossingCommentsOnTheOpenCeilingBead(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	c.ceiling = time.Nanosecond

	ldfJudge(t, c, repo)
	first := lhFiled(t, c, repo)

	commitIn(t, repo, "fine.txt", "an ordinary landing onto a slow judge\n", "ranger-base-aaaa1: ordinary work")
	second := mustGit(t, repo, "rev-parse", "HEAD")
	ldfJudge(t, c, repo)

	if is := lhFiled(t, c, repo); is.ID != first.ID {
		t.Fatalf("a second crossing filed %s beside %s", is.ID, first.ID)
	}
	cs := ldfComments(t, c, repo, first.ID)
	if len(cs) != 1 {
		t.Fatalf("%d comment(s) on %s after a second crossing, want 1: %s", len(cs), first.ID, ldfCommentTexts(cs))
	}
	for _, want := range []string{"answered at " + abbrevSHA(revTreeID(repo, second)), "still over"} {
		if !strings.Contains(cs[0].Text, want) {
			t.Errorf("the comment does not say %q: %q", want, cs[0].Text)
		}
	}
}

// Done-when (iv): an answered run after a silent stretch COMMENTS on the
// open bead and RESETS the counter — and does not close the bead, because
// closing it is architecture's, exactly as D5 leaves a red main's bead to
// the fixer.
//
// The reset is the half with teeth. Without it the counter stays past the
// threshold for ever, so the next single silence — a stranded lock, a cage
// hiccup, the thing D6.3 deliberately does not file on — reads as the second
// of a pattern.
func TestAnAnsweredRunAfterSilenceCommentsResetsTheCounterAndLeavesTheBeadOpen(t *testing.T) {
	t.Parallel()
	c, out, repo, log := ldfFixture(t)
	c.cage = lhSilentCage(lhSaidOne, lhSaidTwo)

	ldfJudge(t, c, repo)
	ldfJudge(t, c, repo)
	silent := lhFiled(t, c, repo)

	// The judge answers again. Nothing is recorded over a silence, so the
	// same tree is judged — which is what D4's cache does everywhere else.
	c.cage = plainCage
	tip := mustGit(t, repo, "rev-parse", "HEAD")
	ldfJudge(t, c, repo)

	if n := len(landedRuns(t, log)); n != 1 {
		t.Fatalf("%d judge run(s) after the cage was fixed, want 1 — the answered arm was never reached", n)
	}
	if _, v := landedCheckRecorded(repo); v != landGreen {
		t.Fatalf("the run did not answer green (%s)", v)
	}
	if n := lhQuiet(t, repo); n != 0 {
		t.Errorf("%s still counts %d after an answered run, want 0 — any answered verdict resets it (ADR 0088 D6.3), and a counter stuck past the threshold makes the next single silence read as a pattern", landedQuietKey, n)
	}
	is := lhFiled(t, c, repo) // still exactly one, and still OPEN
	if is.ID != silent.ID || is.Status == "closed" {
		t.Fatalf("the bead read back as %s/%s, want %s still open — closing it is architecture's (ADR 0088 D6.3)", is.ID, is.Status, silent.ID)
	}
	cs := ldfComments(t, c, repo, silent.ID)
	if len(cs) != 1 {
		t.Fatalf("%d comment(s) on %s, want the one saying the judge answered: %s", len(cs), silent.ID, ldfCommentTexts(cs))
	}
	if want := "answered at " + abbrevSHA(revTreeID(repo, tip)); !strings.Contains(cs[0].Text, want) {
		t.Errorf("the comment is %q, want it to say %q", cs[0].Text, want)
	}
	if !landedWallSaid.MatchString(cs[0].Text) {
		t.Errorf("the comment carries no wall, so the one line that says the judge is working again does not say what it cost: %q", cs[0].Text)
	}
	if s := out.String(); !strings.Contains(s, silent.ID) {
		t.Errorf("the pass did not say which bead it commented on:\n%s", s)
	}
}

// THE COUNTER IS THIS RULE'S DEDUPE, so a counter that could not be written
// files NOTHING (ranger-base-w9w37's rule, which D6.3 names). `git config`
// on a checkout several personas share loses `.git/config.lock` to any
// concurrent one — a shape AGENTS.md and ADR 0059 D3 both name on this box.
//
// Ungated it is the ranger-base-4v4r6 incident with a new subject: the
// record keeps reading one below the threshold, so every later silence
// computes the same n in memory and files again, one P1 per landing, for one
// broken detector.
//
// The first silence has to LAND for this arm to measure the gate rather than
// a counter that never advanced: with the record stuck at 1, each run
// afterwards reaches the threshold and is refused by the gate alone.
func TestSilencesOverAnUnwritableCounterFileNothingUntilItLands(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	c.cage = lhSilentCage(lhSaidOne, lhSaidTwo, lhSaidThree, lhSaidOne)

	ldfJudge(t, c, repo) // silence 1, counted
	if n := lhQuiet(t, repo); n != 1 {
		t.Fatalf("%s counted %d, want 1 — the arm below needs a landed count to reach the threshold over", landedQuietKey, n)
	}

	// From here the counter cannot be written at all.
	lock := filepath.Join(repo, ".git", "config.lock")
	write(t, lock, "")

	for i := 0; i < 2; i++ {
		ldfJudge(t, c, repo)
		if n := lhQuiet(t, repo); n != 1 {
			t.Fatalf("%s moved to %d although its write failed, so this arm measured nothing", landedQuietKey, n)
		}
		if open := lhBeads(t, c, repo); len(open) != 0 {
			t.Fatalf("silence %d filed %s over a counter that was never written — every later silence reads the same count and files again (ranger-base-4v4r6)", i+2, ldfTitles(open))
		}
	}
	for _, want := range []string{"could not be written", "nothing is filed"} {
		if s := out.String(); !strings.Contains(s, want) {
			t.Errorf("the pass does not say %q — a judge that stopped answering and went unfiled must say so in the log that is then its only record:\n%s", want, s)
		}
	}

	// The lock frees. The next silence counts, reaches the threshold, and
	// files the one bead D6.3 owes this stretch — one landing late and not
	// one per landing.
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	ldfJudge(t, c, repo)

	if n := lhQuiet(t, repo); n != 2 {
		t.Fatalf("%s counted %d after the lock freed, want 2", landedQuietKey, n)
	}
	is := lhFiled(t, c, repo)
	if want := "landed check silent at " + AbbrevHome(repo); is.Title != want {
		t.Errorf("the bead is titled %q, want %q", is.Title, want)
	}
}

// A SILENT JUDGE AND A RED MAIN ARE TWO BEADS IN TWO LANES, and neither
// rule's dedupe can see the other's bead. D6.3 asks for a marker distinct
// from D5's, and the cost of sharing one is in both directions: a silent
// judge commenting "answered at <tree>" onto a red main's bead, or a red
// main filed as a second architecture bead nobody owns.
func TestASilentJudgeAndARedMainAreTwoBeadsInTwoLanes(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldfJudge(t, c, repo) // green→red: D5's bead, in the code lane
	red := ldfFiled(t, c, repo)

	// Now the judge stops answering, twice. A landing between the two runs,
	// because D4's cache answers for a tree already on record and this
	// stretch starts from a recorded RED — so without a new tree the second
	// run would never reach the judge at all.
	c.cage = lhSilentCage(lhSaidOne, lhSaidTwo)
	commitIn(t, repo, "more.txt", "a landing onto a red main\n", "ranger-base-cccc3: more work")
	ldfJudge(t, c, repo)
	commitIn(t, repo, "more2.txt", "another landing onto a red main\n", "ranger-base-dddd4: more work still")
	ldfJudge(t, c, repo)

	health := lhFiled(t, c, repo)
	if health.ID == red.ID {
		t.Fatalf("one bead is carrying both findings (%s)", health.ID)
	}
	if got := ldfFiled(t, c, repo); got.ID != red.ID {
		t.Fatalf("the %s lane holds %s, want D5's %s alone — a health bead filed into the code lane would be read as a red main", landedFileLabel, got.ID, red.ID)
	}
	if cs := ldfComments(t, c, repo, red.ID); len(cs) != 0 {
		t.Errorf("the silent judge commented on D5's red-main bead: %s", ldfCommentTexts(cs))
	}
	if landedDescriptionCarries(health.Description, landedFileMarker(repo, "main")) {
		t.Errorf("the health bead carries D5's marker, so the next red main would comment on it instead of filing:\n%s", health.Description)
	}
	if landedDescriptionCarries(red.Description, landedHealthMarker(repo)) {
		t.Errorf("D5's bead carries the health marker, so the next silence would comment on a red main's bead instead of filing:\n%s", red.Description)
	}
}

// The seam cannot silently lower what production charges. `ceiling` exists
// so a test can reach an over-the-price run at all (this file's header); a
// checker built the way the launcher builds one reads D6.2's constant, and a
// zero-value checker does too.
func TestTheCeilingSeamDefaultsToTheShippedPrice(t *testing.T) {
	t.Parallel()
	if got := newLandedChecker(wtApp(t), &bytes.Buffer{}).ceilingOf(); got != landedCheckCeiling {
		t.Errorf("a checker built by newLandedChecker prices at %s, want D6.2's %s", got, landedCheckCeiling)
	}
	if got := (&landedChecker{}).ceilingOf(); got != landedCheckCeiling {
		t.Errorf("a zero-value checker prices at %s, want D6.2's %s", got, landedCheckCeiling)
	}
}

// The counter's own parse table. ANY other shape is no record, which is
// landedCheckRecorded's rule and for its reason: this is one line of git
// config a human can edit, and a half-parsed count must read as ZERO — the
// direction that files nothing — rather than as a stretch nobody measured.
func TestTheQuietCounterReadsZeroOverAnyShapeThatIsNotItsOwn(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)
	for _, tc := range []struct {
		set  string
		tree string
		n    int
		why  string
	}{
		{"abc123 0", "abc123", 0, "a landed reset"},
		{"abc123 2", "abc123", 2, "the record at the threshold"},
		{"abc123 17", "abc123", 17, "a long silent stretch"},
		{"abc123", "", 0, "a tree with no count"},
		{"abc123 2 extra", "", 0, "a third field"},
		{"abc123 two", "", 0, "a count that is not a number"},
		{"abc123 -1", "", 0, "a negative count"},
		{"abc123 1.5", "", 0, "a fractional count"},
		{"", "", 0, "an empty value"},
	} {
		if _, err := git(repo, "config", landedQuietKey, tc.set); err != nil {
			t.Fatalf("the fixture could not set %q: %v", tc.set, err)
		}
		tree, n := landedQuietRecorded(repo)
		if tree != tc.tree || n != tc.n {
			t.Errorf("landedQuietRecorded over %q = %q %d, want %q %d — %s", tc.set, tree, n, tc.tree, tc.n, tc.why)
		}
	}
	// And an absent key is zero, which is what makes a reset spelled as
	// `<tree> 0` and a record that was never written the same reading.
	if _, err := git(repo, "config", "--unset", landedQuietKey); err != nil {
		t.Fatal(err)
	}
	if tree, n := landedQuietRecorded(repo); tree != "" || n != 0 {
		t.Errorf("landedQuietRecorded over an absent key = %q %d, want %q 0", tree, n, "")
	}
}

// A STORE THAT CANNOT ANSWER IS NOT A STORE THAT SAID NO (ranger-base-w9w37;
// this arm added verifying ranger-base-lhiy1 under ranger-base-vzn36).
// healthBeadState's three answers are landedfile.go's three and not a bool,
// and only landedStretchNone may be answered by FILING: a `bd list` that
// fails — a locked database, a repo with no bd init — must file nothing, say
// which answer it got, and leave the next landing to ask again, because a
// stretch whose bead cannot be read is a stretch that may already have one.
//
// WHY THIS ARM EXISTS. The mutant it kills is `healthBeadState` returning
// landedStretchNone on a read error, and MEASURED 2026-11-13 all nine of
// D6.3's other pins are GREEN with that mutant in place — the fail-open arm
// was the one branch of this rule that no fixture walked. What it costs is
// the price healthBeadState's own header names: one P1 architecture bead per
// landing for as long as the store is unreadable, which is a seat, a
// worktree and a suite run each.
//
// The counter still advances over the unknown answer, and that is the
// design, not a hole: the COUNT is this row's dedupe and the store read is
// the second gate, so the crossing is remembered and re-asked rather than
// spent. The third run here is what proves that — same stretch, store
// healthy again, exactly one bead.
func TestASilentJudgeOverAnUnreadableStoreFilesNothingAndTheNextLandingAsksAgain(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	c.cage = lhSilentCage(lhSaidOne, lhSaidTwo, lhSaidThree)
	fail := filepath.Join(repo, "fake-list-fail")

	// One silence: counted, and this run asks the store nothing at all.
	ldfJudge(t, c, repo)

	// The second is the crossing — and the store cannot be asked.
	write(t, fail, "")
	ldfJudge(t, c, repo)

	if n := lhQuiet(t, repo); n != 2 {
		t.Fatalf("%s counted %d after two unanswered runs, want 2 — the count is written before the store is asked, so an unreadable store may not cost the crossing", landedQuietKey, n)
	}
	argv := ldfArgv(t)
	// POSITIVE CONTROL first: the run really did reach the store read, so
	// the absence asserted next is an absence of filing and not of running.
	if !strings.Contains(argv, "--label-any "+landedHealthLabel) {
		t.Fatalf("no `bd list` for the %s lane reached the fake bd, so this arm measured nothing — it has to get as far as the store read for a refusal there to mean anything:\n%s", landedHealthLabel, argv)
	}
	if strings.Contains(argv, " create ") {
		t.Errorf("a bead was filed over a store that could not say whether this stretch already has one — only landedStretchNone may be answered by filing (ranger-base-w9w37):\n%s", argv)
	}
	if s := out.String(); !strings.Contains(s, "the store could not say whether this silent stretch already has a bead") {
		t.Errorf("the pass did not say the store could not answer, so a reader cannot tell this from a run that filed nothing because there was nothing to file:\n%s", s)
	}

	// THE PROMISE: the next landing asks again. Same silent stretch, store
	// healthy, and now exactly one bead carrying the last two sentences.
	if err := os.Remove(fail); err != nil {
		t.Fatal(err)
	}
	ldfJudge(t, c, repo)
	is := lhFiled(t, c, repo)
	for _, want := range []string{lhSaidTwo, lhSaidThree, landedHealthMarker(repo)} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead filed by the run after the unreadable one does not name %q:\n%s", want, is.Description)
		}
	}
}

// A COUNT THAT CAME THROUGH A FAILED RESET IS NOT A PATTERN, AND THE BEAD
// SAYS WHICH (ranger-base-4jroi, from verifying ranger-base-lhiy1 under
// ranger-base-vzn36). The path: one silence counts, the answered run after it
// cannot write the reset — `.git/config.lock` to any concurrent persona, a
// shape AGENTS.md and ADR 0059 D3 both name as routine on this box — and the
// NEXT silence crosses the threshold over a count that spans an answered
// run.
//
// Both halves of the bead that crossing used to file were false. It claimed
// two CONSECUTIVE silences, when an answered GREEN run sat between them, and
// it blamed the absent first sentence on "another launcher process", when ONE
// launcher judged all three runs and cleared that sentence itself two lines
// above the failed reset. The reader of this bead is architecture; it sent
// them looking for a restart that never happened.
//
// WHY NO LANDING IS NEEDED BETWEEN THE ANSWERED RUN AND THE SILENCE. D4's
// cache answers for a tree already on record, so a stretch that resumes over
// an unchanged tip normally needs a new commit to reach the judge at all
// (TestASilentJudgeAndARedMainAreTwoBeadsInTwoLanes). Here the lock blocks
// EVERY `git config` write, the verdict's own included, so the answered run
// records nothing and the tip is still unjudged — which is also what the
// shape looks like in the wild, where one lost lock is one lost lock.
func TestASilenceCountedThroughAFailedResetClaimsNoPatternAndNoRestart(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)

	// Silence 1: counted, nothing filed, and its sentence is in memory.
	c.cage = lhSilentCage(lhSaidOne)
	ldfJudge(t, c, repo)
	if n := lhQuiet(t, repo); n != 1 {
		t.Fatalf("%s counted %d after one unanswered run, want 1 — the arm below needs a landed count for the failed reset to carry", landedQuietKey, n)
	}

	// The judge answers, and the reset it owes cannot be written.
	lock := filepath.Join(repo, ".git", "config.lock")
	write(t, lock, "")
	c.cage = plainCage
	ldfJudge(t, c, repo)
	if n := lhQuiet(t, repo); n != 1 {
		t.Fatalf("%s reads %d after an answered run whose reset failed, want the 1 it could not clear — this arm measures the count that CARRIES", landedQuietKey, n)
	}
	if s := out.String(); !strings.Contains(s, "could not be reset") {
		t.Fatalf("the answered run did not warn that %s could not be reset, so the path this arm is about was never walked:\n%s", landedQuietKey, s)
	}

	// The lock frees, and the next silence crosses the threshold.
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	c.cage = lhSilentCage(lhSaidThree)
	ldfJudge(t, c, repo)

	if n := lhQuiet(t, repo); n != 2 {
		t.Fatalf("%s counted %d after the second silence, want 2 — counting from 1 again is ranger-base-w9w37's rule and is NOT what this arm disputes", landedQuietKey, n)
	}
	is := lhFiled(t, c, repo)
	for _, want := range []string{
		"THEY ARE NOT CONSECUTIVE", // the claim the count cannot carry
		"could not be written",     // and why: the reset, not a restart
		"the ANSWERED run between the two cleared it",
		lhSaidThree,              // the one sentence this process does have
		landedHealthMarker(repo), // still the one bead per stretch
		"closing is architecture's",
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	for _, bad := range []struct{ s, why string }{
		{"another launcher process", "ONE launcher judged all three runs; the absent sentence is this process's own, cleared by the answered run"},
		{"consecutive judged runs", "an answered GREEN run sat between the two silences, so the pair is not consecutive"},
		{lhSaidOne, "the sentence from BEFORE the answered run is not this stretch's — landedhealth.go clears it on the answered run, and that clear is right"},
	} {
		if strings.Contains(is.Description, bad.s) {
			t.Errorf("the bead claims %q — %s:\n%s", bad.s, bad.why, is.Description)
		}
	}
}

// …and the restart wording is still the restart's, which is what makes the
// arm above a DISTINCTION rather than a second constant. A crossing is two
// runs of one long-lived launcher by construction (ADR 0028 §1), and when it
// is not, the bead says the earlier sentence is held by the process that has
// it rather than inventing one — ADR 0088 D6.3's own residual, and the cause
// ranger-base-lhiy1 recorded as ASSUMED.
//
// A SECOND CHECKER OVER THE SAME REPO IS THE RESTART. The counter is a git
// config record and survives it; the sentence is in memory (D6.3 spells the
// record `<tree> <n>`) and does not. The App is shared because what this arm
// is about is the checker's own map, and the cage closure is shared so the
// second refusal still has a sentence of its own.
func TestASilenceAfterALauncherRestartSaysTheEarlierSentenceIsHeldElsewhere(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	c.cage = lhSilentCage(lhSaidOne, lhSaidTwo)

	ldfJudge(t, c, repo)
	if n := lhQuiet(t, repo); n != 1 {
		t.Fatalf("%s counted %d after one unanswered run, want 1", landedQuietKey, n)
	}

	next := newLandedChecker(c.a, out)
	next.cage, next.bd = c.cage, c.bd
	ldfJudge(t, next, repo)

	if n := lhQuiet(t, repo); n != 2 {
		t.Fatalf("%s counted %d after the restarted launcher's silence, want 2 — the record is what survives a restart", landedQuietKey, n)
	}
	is := lhFiled(t, next, repo)
	for _, want := range []string{
		"another launcher process", // this branch's own cause
		"consecutive judged runs",  // and the count IS consecutive here
		lhSaidTwo,
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	for _, bad := range []struct{ s, why string }{
		{"THEY ARE NOT CONSECUTIVE", "both runs were silent and nothing answered between them"},
		{"could not be reset", "no reset was owed or attempted on this path"},
		{lhSaidOne, "the first run's sentence is in the process that died, and this bead may not invent it"},
	} {
		if strings.Contains(is.Description, bad.s) {
			t.Errorf("the bead claims %q — %s:\n%s", bad.s, bad.why, is.Description)
		}
	}
}
