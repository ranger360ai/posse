//go:build posse_arm2

package posse

// ADR 0088 D5 at the site it files: the landed check's verdict becomes ONE
// attributed bead per green→red stretch, and the four other rows of D5's
// table are a comment or nothing at all (ranger-base-ddgfr; landedfile.go's
// header has the table and why the ratchet is keyed on the transition).
//
// THE FIXTURE IS THE RUNNER, END TO END, and never a call to `file` with a
// verdict made up beside it. What D5 is about is which TRANSITION a landing
// is, and a transition is two runs over two trees with a record written
// between them — so a test that handed `file` a `prev` would pin the table
// and nothing about the thing the table is read from. Every arm here lands
// real commits on a real main and lets landedcheck.go judge them.
//
// SO THE CHECK'S VERDICT HAS TO DEPEND ON THE TREE IT JUDGES, which is what
// `boom.txt` is for (ldfCheckBody): the copy that judges comes from the tree
// last recorded GREEN (D1), so a landing that changed the script could not
// move the verdict even if a fixture wanted it to. One script, red over any
// tree carrying that file, is the only shape that can walk green→red→red→
// green with the judge held fixed the way D1 holds it.
//
// THE STORE IS THE FAKE BD every other filing path in this package is pinned
// against (herdr_test.go), reached through the one seam the production code
// carries for it (landedChecker.bd). It is the store that says whether a
// commit subject's prefix is a bead id and who holds it, so the window's
// attribution is measured against a store's answers rather than a regexp's
// opinion — which is the whole of attribute's doctrine.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two halves of the incident ADR 0088 was written for, as this fixture's
// store holds them: each landed green on its own base, each carries a
// persona, and the NEWER of the two is the one D5 assigns.
const (
	// The two personas are ROLES and not this instance's crew, which ADR
	// 0012 App.A 5 holds the shipped tree to (`make crew-check`).
	ldfOlderBead   = "ranger-base-aaaa1"
	ldfOlderWho    = "reviewer"
	ldfNewerBead   = "ranger-base-bbbb2"
	ldfNewerWho    = "developer"
	ldfRedTail     = "corpus-check: the waiver register is stale"
	ldfBoomMessage = ldfNewerBead + ": add the arms that fire the pin"
	ldfHalfMessage = ldfOlderBead + ": ship the stale-register pin"
)

// ldfStore is the fake bd's `show` fixture: the two beads above, closed, each
// held by a persona. A `create` lands in it too (fakeBdAppendCreated), so a
// bead this rule files is readable by the same store afterwards.
const ldfStore = `[` +
	`{"id":"` + ldfOlderBead + `","title":"the stale-register pin","status":"closed","assignee":"` + ldfOlderWho + `"},` +
	`{"id":"` + ldfNewerBead + `","title":"the coldexec arms","status":"closed","assignee":"` + ldfNewerWho + `"}]`

// ldfCheckBody is a check whose verdict is a property of the TREE: red over
// any tree holding `boom.txt`, green otherwise, and one log line per run
// saying which tip it judged (landedcheck_test.go's witness, kept because
// "which tree was judged" is still the question underneath every arm).
func ldfCheckBody(log string) string {
	return "#!/bin/sh\n" +
		"printf '%s %s\\n' v1 \"$(git rev-parse HEAD)\" >> " + shq(log) + "\n" +
		"if [ -f boom.txt ]; then\n" +
		"  echo " + shq(ldfRedTail) + " >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
}

// ldfFixture is a repo on main shipping that check, a checker wired to the
// plain cage and the fake bd, and the run log.
func ldfFixture(t *testing.T) (*landedChecker, *bytes.Buffer, string, string) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	log := filepath.Join(t.TempDir(), "runs")
	shipLandedCheck(t, repo, ldfCheckBody(log), "ranger-base-ship1: ship the landed check")
	write(t, filepath.Join(repo, "fake-show.json"), ldfStore)
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)
	c.cage, c.bd = plainCage, testBd(t)
	return c, out, repo, log
}

// ldfJudge runs one landed check over main and waits for it.
func ldfJudge(t *testing.T, c *landedChecker, repo string) {
	t.Helper()
	c.Schedule(repo, "main")
	c.Wait()
}

// ldfGreen takes the repo to a recorded GREEN, which is the left-hand column
// of D5's second row and the far edge of every window below.
func ldfGreen(t *testing.T, c *landedChecker, repo string) string {
	t.Helper()
	ldfJudge(t, c, repo)
	tree, v := landedCheckRecorded(repo)
	if v != landGreen {
		t.Fatalf("the fixture did not reach a recorded green: %s %s", tree, v)
	}
	return tree
}

// ldfBeads is every open `-l code` bead in the store, which is the lane D5
// files into.
func ldfBeads(t *testing.T, c *landedChecker, repo string) []BdIssue {
	t.Helper()
	open, err := c.bd.OpenLabeledAny(repo, landedFileLabel)
	if err != nil {
		t.Fatalf("the store could not be read back: %v", err)
	}
	return open
}

// ldfFiled is the one bead this rule filed, and a failure when there is not
// exactly one — "one open bead per red stretch" is the whole claim, so every
// arm asserts the count rather than picking a row out of a list.
func ldfFiled(t *testing.T, c *landedChecker, repo string) BdIssue {
	t.Helper()
	open := ldfBeads(t, c, repo)
	if len(open) != 1 {
		t.Fatalf("%d open %s bead(s) after this stretch, want exactly 1 (ADR 0088 D5): %s", len(open), landedFileLabel, ldfTitles(open))
	}
	return open[0]
}

func ldfTitles(open []BdIssue) string {
	ts := make([]string, len(open))
	for i, is := range open {
		ts[i] = is.ID + " " + is.Title
	}
	return strings.Join(ts, " | ")
}

// ldfBreak lands two commits, the second of which reds the check. TWO,
// because D5's window is the interesting thing: the pin and the arms that
// fire it landed one after the other, each green on its own base, and the
// bead has to name both and assign the newer.
func ldfBreak(t *testing.T, repo string) (older, newer string) {
	t.Helper()
	commitIn(t, repo, "half.txt", "the half that is green on its own base\n", ldfHalfMessage)
	older = mustGit(t, repo, "rev-parse", "HEAD")
	commitIn(t, repo, "boom.txt", "the other half\n", ldfBoomMessage)
	return older, mustGit(t, repo, "rev-parse", "HEAD")
}

// ldfArgv is every bd argv this test's fake recorded, as one string.
//
// WHY THE ARGV AND NOT THE ROW for three of D5's fields. The fake's created
// row models what the suite needed before now — id, title, description,
// status, labels and a timestamp (fakeBdAppendCreated) — and `-t`, `-p` and
// `-a` are not in it, so a row read back is silent about the type, the
// priority and the assignee however they were asked for. In a real store the
// argv is what decides them, so the argv is what this pins; the label, which
// the row does carry, is pinned through the row beside it.
func ldfArgv(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fakeDirOf(t), "bd-calls.log"))
	if err != nil {
		t.Fatalf("the fake bd recorded no call at all: %v", err)
	}
	return string(b)
}

// ldfCreateFields is the tail of a `bd create` argv as Bd.Create builds it:
// the assignee (omitted when there is none), the label, the priority and the
// type, in that order. Asserted as one string because the ORDER is Create's
// own and a drift in it is worth seeing.
func ldfCreateFields(assignee string) string {
	if assignee != "" {
		assignee = " -a " + assignee
	}
	return fmt.Sprintf("%s -l %s -p %d -t %s", assignee, landedFileLabel, landedFilePriority, landedFileType)
}

func ldfComments(t *testing.T, c *landedChecker, repo, id string) []BdComment {
	t.Helper()
	cs, err := c.bd.Comments(repo, id)
	if err != nil {
		t.Fatalf("the comments on %s could not be read back: %v", id, err)
	}
	return cs
}

func ldfCommentTexts(cs []BdComment) string {
	ts := make([]string, len(cs))
	for i, c := range cs {
		ts[i] = c.Text
	}
	return strings.Join(ts, "\n---\n")
}

// THE HEADLINE (D5 row 2, and the done-when): a green→red transition files
// ONE -t bug P1 -l code bead, titled for the tree, carrying the check's own
// last lines, naming EVERY landing in the window with its bead and persona,
// and assigned to the persona of the NEWEST of them.
func TestAGreenToRedTransitionFilesOneAttributedP1NamingEveryLandingInTheWindow(t *testing.T) {
	t.Parallel()
	c, out, repo, log := ldfFixture(t)
	green := ldfGreen(t, c, repo)
	older, newer := ldfBreak(t, repo)

	ldfJudge(t, c, repo)

	if n := len(landedRuns(t, log)); n != 2 {
		t.Fatalf("%d runs, want 2 (one green, one red): the fixture never reached the transition", n)
	}
	is := ldfFiled(t, c, repo)
	tree := revTreeID(repo, newer)
	if want := "main red at " + abbrevSHA(tree); is.Title != want {
		t.Errorf("the bead is titled %q, want %q — D5 titles it for the TREE, which is the key the verdict is recorded against", is.Title, want)
	}
	if !hasLabel(is.Labels, landedFileLabel) {
		t.Errorf("the bead carries %v, want the %s lane (ADR 0088 D5)", is.Labels, landedFileLabel)
	}
	// `-t bug`, P1, and the assignee: the persona of the NEWEST landing in
	// the window, because that landing is the one whose tree first held both
	// halves (D5, as with ADR 0087 D5's merge-back).
	if want := ldfCreateFields(ldfNewerWho); !strings.Contains(ldfArgv(t), want) {
		t.Errorf("no create carried %q — see ldfArgv on why this reads the argv:\n%s", want, ldfArgv(t))
	}
	for _, want := range []string{
		ldfRedTail,       // the cause, which is what a fixer reads first
		abbrevSHA(newer), // the tip that was judged
		abbrevSHA(green), // the far edge of the window: the tree last recorded green
		abbrevSHA(older), // the OLDER landing, which is green on its own base
		ldfOlderBead, ldfOlderWho,
		ldfNewerBead, ldfNewerWho,
		landedFileMarker(repo, "main"), // the dedupe of record
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	// The assignment is not a finding about care, and the bead says so: every
	// seat in the window ran the doors on its own base and was green there.
	if !strings.Contains(is.Description, "ITS OWN base") {
		t.Errorf("the bead assigns a persona without saying what the assignment means — D5's window is the halves that met, not the seat that was careless:\n%s", is.Description)
	}
	if s := out.String(); !strings.Contains(s, is.ID) {
		t.Errorf("the pass did not say which bead it filed:\n%s", s)
	}
}

// D5 row 3: red→red adds ONE comment naming the new tree to the open bead,
// and files no second bead. This is the row that replaces several seats each
// filing a P1 for one exit code (ranger-base-4v4r6).
func TestARedThatStaysRedCommentsTheNewTreeAndFilesNoSecondBead(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldfJudge(t, c, repo)
	first := ldfFiled(t, c, repo)

	// A landing that does not fix it: still red, on a new tree.
	commitIn(t, repo, "more.txt", "a landing onto a red main\n", "ranger-base-cccc3: something else entirely")
	second := mustGit(t, repo, "rev-parse", "HEAD")
	ldfJudge(t, c, repo)

	if is := ldfFiled(t, c, repo); is.ID != first.ID {
		t.Fatalf("a second red filed %s beside %s — D5 row 3 is one comment and no new bead", is.ID, first.ID)
	}
	cs := ldfComments(t, c, repo, first.ID)
	if len(cs) != 1 {
		t.Fatalf("%d comment(s) on %s after one further red, want 1: %s", len(cs), first.ID, ldfCommentTexts(cs))
	}
	for _, want := range []string{abbrevSHA(revTreeID(repo, second)), ldfRedTail} {
		if !strings.Contains(cs[0].Text, want) {
			t.Errorf("the red→red comment does not name %q: %q", want, cs[0].Text)
		}
	}
}

// D5 row 4: red→green comments "green at <tree>" on the open bead and does
// NOT close it. Closing is the fixer's job, and a launcher that closed it
// would answer a bead nobody has read.
func TestARedThatGoesGreenCommentsGreenAndLeavesTheBeadOpen(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldfJudge(t, c, repo)
	red := ldfFiled(t, c, repo)

	// The fix: the tree no longer carries what the check reds over.
	mustGit(t, repo, "rm", "-q", "--", "boom.txt")
	mustGit(t, repo, "commit", "-q", "-m", "ranger-base-dddd4: delete the matching row", "--", "boom.txt")
	fixed := mustGit(t, repo, "rev-parse", "HEAD")
	ldfJudge(t, c, repo)

	if _, v := landedCheckRecorded(repo); v != landGreen {
		t.Fatalf("the fixture never went green again: %s", v)
	}
	is := ldfFiled(t, c, repo) // still exactly one, and still OPEN
	if is.ID != red.ID || is.Status == "closed" {
		t.Fatalf("the bead read back as %s/%s, want %s still open — D5: closing it is the fixer's job", is.ID, is.Status, red.ID)
	}
	cs := ldfComments(t, c, repo, red.ID)
	if len(cs) != 1 {
		t.Fatalf("%d comment(s) on %s, want the one green comment: %s", len(cs), red.ID, ldfCommentTexts(cs))
	}
	if !strings.Contains(cs[0].Text, "green at "+abbrevSHA(revTreeID(repo, fixed))) {
		t.Errorf("the red→green comment is %q, want `green at <the new tree>`", cs[0].Text)
	}
}

// D5 row 1: green→green records the tree and files nothing. The row is
// trivial and the arm is not — a filer that keyed on the VERDICT rather than
// the transition would file here on every landing of a healthy main, which
// is the one failure mode nobody would notice until the queue was full of
// them.
func TestAGreenThatStaysGreenFilesNothingAndSaysNothingAboutBeads(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)

	before := out.Len()
	commitIn(t, repo, "fine.txt", "an ordinary landing\n", "ranger-base-eeee5: ordinary work")
	ldfJudge(t, c, repo)

	if _, v := landedCheckRecorded(repo); v != landGreen {
		t.Fatalf("the second green was not recorded: %s", v)
	}
	if open := ldfBeads(t, c, repo); len(open) != 0 {
		t.Errorf("a green main filed %s", ldfTitles(open))
	}
	if s := out.String()[before:]; strings.Contains(s, "filed") {
		t.Errorf("a green landing said something about filing:\n%s", s)
	}
}

// D5 row 5: no answer records nothing and files nothing. A cage that will not
// render is the case that matters — the check never ran, so there is no
// verdict to attribute, and a P1 filed over silence would name a persona for
// a red nobody measured.
func TestATreeThatCouldNotBeJudgedFilesNothing(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	c.cage = refusedCage

	ldfJudge(t, c, repo)

	if open := ldfBeads(t, c, repo); len(open) != 0 {
		t.Errorf("silence was filed as %s — ADR 0088 D5's bottom row is ⚠ and nothing recorded", ldfTitles(open))
	}
	if s := out.String(); !strings.Contains(s, "⚠") {
		t.Errorf("a tree that could not be judged said:\n%s", s)
	}
}

// The degenerate half of row 3: the stretch's bead is gone — a fixer closed
// it while main was still red, or a create lost its write — so there is
// nothing to comment on and nobody is attributed. One bead is filed, which
// leaves one OPEN bead per red stretch, and it says why it was filed outside
// the green→red row.
func TestARedWhoseStretchBeadIsNoLongerOpenFilesOneAgain(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldfJudge(t, c, repo)
	first := ldfFiled(t, c, repo)
	if err := c.bd.Close(repo, first.ID, ldfNewerWho); err != nil {
		t.Fatalf("the fixture could not close %s: %v", first.ID, err)
	}

	commitIn(t, repo, "more.txt", "a landing onto a red main\n", "ranger-base-ffff6: more work")
	ldfJudge(t, c, repo)

	is := ldfFiled(t, c, repo)
	if is.ID == first.ID {
		t.Fatalf("%s read back open after the fixture closed it — this arm is about the bead being GONE", is.ID)
	}
	if !strings.Contains(is.Description, "not open any more") {
		t.Errorf("the replacement bead does not say why it was filed over a red→red transition:\n%s", is.Description)
	}
}

// AND THE OTHER TWO THINGS THE STORE CAN SAY ARE NOT THAT (ranger-base-w9w37).
// The arm above is "the bead is gone", which is the one answer D5's red→red
// row may answer by filing. A read that FAILED and a comment that failed are
// different facts, and ADR 0073 measured both as the norm under concurrency
// rather than the exception — so the twin they used to file was a P1, a seat,
// a worktree and a suite run, for a red stretch that already had a bead.
//
// A read that failed: nothing is known about this stretch's bead, and an
// unknown is re-asked by the next landing for free.
func TestARedWhoseStretchBeadCouldNotBeReadFilesNoTwin(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldfJudge(t, c, repo)
	first := ldfFiled(t, c, repo)

	// The lane listing fails the way a locked database fails it.
	marker := filepath.Join(repo, "fake-list-fail")
	write(t, marker, "")
	commitIn(t, repo, "more.txt", "a landing onto a red main\n", "ranger-base-cccc3: something else entirely")
	ldfJudge(t, c, repo)
	if err := os.Remove(marker); err != nil { // …so the assertions below can read the store
		t.Fatal(err)
	}

	if is := ldfFiled(t, c, repo); is.ID != first.ID {
		t.Fatalf("a store that could not be read filed %s beside %s — a failed read is not evidence that this stretch has no bead (ADR 0088 D5)", is.ID, first.ID)
	}
	if cs := ldfComments(t, c, repo, first.ID); len(cs) != 0 {
		t.Errorf("%d comment(s) on %s although the listing that finds it failed: %s", len(cs), first.ID, ldfCommentTexts(cs))
	}
	for _, want := range []string{"could not be read", "nothing was filed"} {
		if s := out.String(); !strings.Contains(s, want) {
			t.Errorf("the pass does not say %q — a landing that filed nothing over an unreadable store must say so, because this log is then the only record of the red:\n%s", want, s)
		}
	}
}

// A comment that failed: the bead was read as OPEN one line earlier, so the
// twin's own body — "the bead for this red stretch is not open any more" —
// was a sentence the filer had already disproved.
func TestARedWhoseCommentFailedFilesNoTwinOverABeadItReadAsOpen(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldfJudge(t, c, repo)
	first := ldfFiled(t, c, repo)

	write(t, filepath.Join(repo, "fake-comment-fail"), "database is locked")
	commitIn(t, repo, "more.txt", "a landing onto a red main\n", "ranger-base-cccc3: something else entirely")
	ldfJudge(t, c, repo)

	if is := ldfFiled(t, c, repo); is.ID != first.ID {
		t.Fatalf("a failed `bd comments add` filed %s beside %s, which the same pass had just read as OPEN (ADR 0088 D5)", is.ID, first.ID)
	}
	if cs := ldfComments(t, c, repo, first.ID); len(cs) != 0 {
		t.Errorf("the fixture's failing add landed a comment anyway, so this arm measured nothing: %s", ldfCommentTexts(cs))
	}
	s := out.String()
	if !strings.Contains(s, "was not commented on") {
		t.Errorf("a comment that could not be written is not said:\n%s", s)
	}
	if strings.Contains(s, "not open any more") {
		t.Errorf("the pass says this stretch's bead is gone over a bead it read as open:\n%s", s)
	}
}

// THE RECORD IS THE GREEN→RED ROW'S WHOLE DEDUPE (ranger-base-w9w37), so a
// verdict that could not be recorded files nothing. `recordLandedCheck` is
// best effort by contract (ADR 0088 D4) and a `git config` write on a
// checkout several personas share loses `.git/config.lock` to any concurrent
// one — a shape AGENTS.md and ADR 0059 D3 both already name on this box.
// Left ungated it is the ranger-base-4v4r6 incident exactly: the record keeps
// naming the stale GREEN tree, so every later red reads green→red again.
// MEASURED 2026-11-11, three red landings over a held lock: three open P1s.
//
// The marker cannot cover this row — a bead stays open after its stretch ends
// (closing it is the fixer's), so an open bead cannot tell a NEW stretch from
// a repeat reading of this one. What the gate costs instead is one landing:
// the next judge of the same tree re-asks, and files when the record lands.
func TestAVerdictThatCouldNotBeRecordedFilesNothingUntilTheRecordLands(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	green := ldfGreen(t, c, repo)
	ldfBreak(t, repo)

	// The one write that fails is recordLandedCheck's: `git config` cannot
	// take a lock that is already there, and MEASURED on this box nothing
	// else a judged landing runs needs it — `worktree add`, `add`, `commit`
	// and `worktree remove` all go through with this lock held.
	lock := filepath.Join(repo, ".git", "config.lock")
	write(t, lock, "")

	// Three red landings, each on a new tree, with the record unwritable
	// throughout.
	for i, next := range []string{"one.txt", "two.txt", ""} {
		ldfJudge(t, c, repo)
		if tree, v := landedCheckRecorded(repo); tree != green || v != landGreen {
			t.Fatalf("the record moved to %s %s although its write failed, so this arm measured nothing", tree, v)
		}
		if open := ldfBeads(t, c, repo); len(open) != 0 {
			t.Fatalf("landing %d filed %s over a verdict that was never recorded — every later red reads green→red again, so this is one P1 per landing (ranger-base-4v4r6)", i+1, ldfTitles(open))
		}
		if next != "" {
			ldfLandUnderLock(t, repo, next, "a further landing onto a red main\n", "ranger-base-iiii9: more work")
		}
	}
	for _, want := range []string{"could not be recorded", "nothing was filed"} {
		if s := out.String(); !strings.Contains(s, want) {
			t.Errorf("the pass does not say %q — a red main that goes unfiled must say so in the log that is then its only record:\n%s", want, s)
		}
	}

	// The lock frees. The same red tree is judged again, the record lands,
	// and so does the one bead D5 owes this stretch — one landing late and
	// not one per landing.
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	tip := mustGit(t, repo, "rev-parse", "HEAD")
	ldfJudge(t, c, repo)

	tree := revTreeID(repo, tip)
	if got, v := landedCheckRecorded(repo); got != tree || v != landRed {
		t.Fatalf("the record reads %s %s after the lock freed, want %s red", got, v, tree)
	}
	is := ldfFiled(t, c, repo)
	if want := "main red at " + abbrevSHA(tree); is.Title != want {
		t.Errorf("the bead is titled %q, want %q", is.Title, want)
	}
	if !strings.Contains(is.Description, abbrevSHA(green)) {
		t.Errorf("the bead does not name %s, the tree last recorded green — the window is still bounded by the record it finally wrote:\n%s", abbrevSHA(green), is.Description)
	}
	// …and the ratchet is unharmed: the next red comments on that bead.
	ldfLandUnderLock(t, repo, "three.txt", "one more landing onto a red main\n", "ranger-base-jjj10: more work still")
	ldfJudge(t, c, repo)
	if got := ldfFiled(t, c, repo); got.ID != is.ID {
		t.Fatalf("the landing after the record landed filed %s beside %s", got.ID, is.ID)
	}
	if cs := ldfComments(t, c, repo, is.ID); len(cs) != 1 {
		t.Errorf("%d comment(s) on %s, want the one naming the newest tree: %s", len(cs), is.ID, ldfCommentTexts(cs))
	}
}

// ldfLandUnderLock lands one commit without touching `git config`, which
// commitIn does on every call: a fixture holding `.git/config.lock` cannot
// afford that, and wtRepo has set the identity already.
func ldfLandUnderLock(t *testing.T, repo, path, body, msg string) {
	t.Helper()
	write(t, filepath.Join(repo, path), body)
	mustGit(t, repo, "add", "--", path)
	mustGit(t, repo, "commit", "-q", "-m", msg, "--", path)
}

// ATTRIBUTION IS THE STORE'S ANSWER AND NOT A PREFIX. "fix: a typo" has the
// shape of this shop's provenance line, and a filer that read English as a
// bead id would attribute a red main to the word `fix` and assign the bead
// to nobody while saying it had named somebody.
func TestASubjectWhosePrefixIsNoBeadIdIsNamedByCommitAlone(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	commitIn(t, repo, "boom.txt", "broken\n", "fix: a typo in the register")
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	ldfJudge(t, c, repo)

	is := ldfFiled(t, c, repo)
	if strings.Contains(is.Description, "fix (") || strings.Contains(is.Description, "- "+abbrevSHA(tip)+" fix") {
		t.Errorf("`fix` was attributed as a bead id:\n%s", is.Description)
	}
	if !strings.Contains(is.Description, "no bead id this store knows") {
		t.Errorf("a landing the store cannot attribute is not said to be one:\n%s", is.Description)
	}
	if want := ldfCreateFields(""); !strings.Contains(ldfArgv(t), want) {
		t.Errorf("the create did not carry %q — over a window that names no persona, unassigned is the honest answer and the bead is still P1:\n%s", want, ldfArgv(t))
	}
}

// The window is bounded by the tree last recorded GREEN, and when there is
// none on record there is no far edge to bound it with: the bead names the
// landing that was judged and says the window could not be bounded, rather
// than printing fifty commits nobody has a reading for.
func TestAFirstEverRedNamesTheJudgedLandingAndSaysTheWindowIsUnbounded(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	commitIn(t, repo, "boom.txt", "broken from the start\n", ldfBoomMessage)
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	ldfJudge(t, c, repo) // the first run ever, and it is red

	is := ldfFiled(t, c, repo)
	if !strings.Contains(is.Description, "could not be bounded") {
		t.Errorf("a red with nothing green on record claims a window it cannot have:\n%s", is.Description)
	}
	if !strings.Contains(is.Description, abbrevSHA(tip)) {
		t.Errorf("the bead does not name the landing it judged:\n%s", is.Description)
	}
	if want := ldfCreateFields(ldfNewerWho); !strings.Contains(ldfArgv(t), want) {
		t.Errorf("the create did not carry %q — the one landing it could name is the one it is assigned to:\n%s", want, ldfArgv(t))
	}
}

// A marker is matched as a WHOLE LINE, so nothing a commit subject or a
// check's output tail carries can pass for one. Unit-level on purpose: the
// forging path runs through text this rule writes into a description it later
// reads back, and the arm has to be able to say no.
func TestTheStretchMarkerIsMatchedAsAWholeLine(t *testing.T) {
	t.Parallel()
	marker := landedFileMarker("/src/posse", "main")
	for _, tc := range []struct {
		desc string
		want bool
		why  string
	}{
		{marker, true, "the marker on its own line"},
		{"first\n" + marker + "\nlast", true, "the marker among other lines"},
		{"  " + marker + "  ", true, "the marker with surrounding space, which bd may round-trip"},
		{"a subject mentioning " + marker + " inline", false, "a line that merely CONTAINS the marker"},
		{landedFileMarker("/src/posse", "release"), false, "another branch's marker"},
		{landedFileMarker("/src/other", "main"), false, "another repo's marker"},
		{"", false, "nothing"},
	} {
		if got := landedDescriptionCarries(tc.desc, marker); got != tc.want {
			t.Errorf("landedDescriptionCarries(%q) = %v, want %v — %s", tc.desc, got, tc.want, tc.why)
		}
	}
}

// And the repo that never opted in files nothing, because nothing judged it
// (D1, residual (3)). The cheap arm that keeps the filer downstream of the
// runner: a filer wired anywhere else would have to re-ask the opt-in
// question for itself.
func TestARepoThatShipsNoLandedCheckFilesNothing(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "fake-show.json"), ldfStore)
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)
	c.cage, c.bd = plainCage, testBd(t)

	ldfJudge(t, c, repo)

	if open := ldfBeads(t, c, repo); len(open) != 0 {
		t.Errorf("a repo with no %s was filed against: %s", landedCheckScript, ldfTitles(open))
	}
	if s := out.String(); s != "" {
		t.Errorf("a repo with no %s said:\n%s", landedCheckScript, s)
	}
}

// A create that bd loses on the wire is SAID, and the next red finds the
// bead by its marker instead of filing a twin — which is the whole reason
// the dedupe is a marker in the store and not a second config record
// (landedfile.go's header, reason (2)).
func TestAFiledBeadWhoseReadBackFailedIsSaidAndNotFiledTwice(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	// ADR 0073's shape for `create`: the bead commits and the confirming
	// read cannot run.
	marker := filepath.Join(repo, "fake-show-fail")
	write(t, marker, "")

	ldfJudge(t, c, repo)

	if s := out.String(); !strings.Contains(s, "could not be confirmed") {
		t.Fatalf("a create whose read-back failed did not say so:\n%s", s)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	first := ldfFiled(t, c, repo) // it did land, and the store has it

	commitIn(t, repo, "more.txt", "another landing onto a red main\n", "ranger-base-gggg7: more")
	ldfJudge(t, c, repo)

	if is := ldfFiled(t, c, repo); is.ID != first.ID {
		t.Fatalf("the next red filed %s beside the orphan %s — the marker is what makes an unconfirmed create safe", is.ID, first.ID)
	}
	if cs := ldfComments(t, c, repo, first.ID); len(cs) != 1 {
		t.Errorf("%d comment(s) on the orphan, want the one naming the new tree: %s", len(cs), ldfCommentTexts(cs))
	}
}

// THE STRUCTURAL HALF of "closing it is the fixer's job": this filer has no
// close arm and no other write than the two it needs. The behaviour arm
// above can only show that a close did not happen on the one path it drove;
// a `bd close` added tomorrow — on the red→green row, where it would read
// like tidying up — would answer a bead nobody has read, and nothing in a
// green suite run would say so.
//
// It is an allow-list rather than a search for `Close`, because the next way
// to answer a bead without reading it is `update --status`, and the pin
// should red on that too.
func TestQATheFilerNeverClosesOrOtherwiseAnswersABead(t *testing.T) {
	t.Parallel()
	const src = "landedfile.go"
	f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"Create":         true, // D5's one filing
		"Comment":        true, // D5's three comment rows
		"Show":           true, // the window's attribution
		"OpenLabeledAny": true, // finding the open bead for this stretch
		"Available":      true, // and asking whether there is a bd at all
	}
	verbs := 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.SelectorExpr)
		if !ok || recv.Sel.Name != "bd" {
			return true
		}
		verbs++
		if !allowed[sel.Sel.Name] {
			t.Errorf("%s:%d calls bd.%s — ADR 0088 D5 gives this rule one write (a bead) and one amendment (a comment); closing or answering a bead is the fixer's",
				src, call.Pos(), sel.Sel.Name)
		}
		return true
	})
	if verbs == 0 {
		t.Errorf("%s makes no bd call at all, so this pin is reading the wrong thing", src)
	}
}

// landedSubjectBead's own table: a candidate is a token followed by ": ",
// and nothing else is one. The store decides whether a candidate is real
// (TestASubjectWhosePrefixIsNoBeadIdIsNamedByCommitAlone); this is only
// about what gets ASKED.
func TestTheCandidateBeadIdIsTheSubjectsOwnPrefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ subject, want string }{
		{"ranger-base-ddgfr: the landed check files", "ranger-base-ddgfr"},
		{"fix: a typo", "fix"},
		{"no colon here at all", ""},
		{": leading colon", ""},
		{"two words: and a subject", ""},
		{"ranger-base-ddgfr:no space after the colon", ""},
		{"", ""},
	} {
		if got := landedSubjectBead(tc.subject); got != tc.want {
			t.Errorf("landedSubjectBead(%q) = %q, want %q", tc.subject, got, tc.want)
		}
	}
}

// One bd write per landing at most, and NONE on a green main. The filer sits
// on the pass that joins the checker, so a rule that asked the store a
// question per green landing would spend the launcher's own wall on a repo
// where nothing is wrong.
func TestAGreenLandingSpendsNoBdCall(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	ldfGreen(t, c, repo)

	calls := filepath.Join(fakeDirOf(t), "bd-calls.log")
	if b, err := os.ReadFile(calls); err == nil && strings.TrimSpace(string(b)) != "" {
		t.Errorf("a green landing made bd calls:\n%s", b)
	}
	commitIn(t, repo, "fine.txt", "ordinary\n", "ranger-base-hhhh8: ordinary")
	ldfJudge(t, c, repo)
	if b, err := os.ReadFile(calls); err == nil && strings.TrimSpace(string(b)) != "" {
		t.Errorf("a second green landing made bd calls:\n%s", b)
	}
	if n := len(ldfBeads(t, c, repo)); n != 0 {
		t.Fatalf("%d bead(s) over a green main", n)
	}
	// …and the fixture is not green-blind: a red does reach the store.
	ldfBreak(t, repo)
	ldfJudge(t, c, repo)
	if _, err := os.Stat(calls); err != nil {
		t.Fatalf("a red landing made no bd call either, so the arm above measured nothing: %v", err)
	}
}
