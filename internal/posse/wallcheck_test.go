//go:build posse_arm2

package posse

// ADR 0097 D2/D5 at the site it runs: after a judgement that answered GREEN,
// the launcher runs the wall check on the same tree, and its verdict goes
// through ADR 0088 D5's ratchet and D6.3's watchdog over records of its own
// (ranger-base-wplhv; wallcheck.go's header has the three pieces, and
// landedrule.go has why the two rules are one implementation).
//
// THE FIXTURE IS THE RUNNER, END TO END, which is landedfile_test.go's rule
// and for its reason: what D5 is about is which TRANSITION a landing is, and
// a transition is two runs over two trees with a record written between them.
// Every arm here lands real commits on a real main and lets the production
// `once` → `wall` → `measure` path judge them. Nothing calls `file` or
// `health` with a verdict made up beside it.
//
// SO THE WALL CHECK'S VERDICT IS A PROPERTY OF THE TREE IT JUDGES
// (wckCheckBody): `wallboom.txt` reds it and `wallquiet.txt` makes it answer
// D3's exit 3. That is the only shape that can walk green→red→red→green with
// the copy that RUNS held fixed the way D1 holds it — the copy comes from the
// last tree recorded green, so a landing that changed the script could not
// move the verdict even if a fixture wanted it to.
//
// AND THE JUDGE IS ALWAYS GREEN HERE unless an arm is about the judge, because
// D2 keys the whole step on a green judgement. The two arms that make the
// judge red or silent are about exactly that line.
//
// THE CAGE IS THE ONE THING STUBBED, through the seam the production code
// carries (landedCageFn), which is landedcheck_test.go's doctrine. `plainCage`
// ignores ADR 0097 D1's kind for the reason its own comment gives: what the
// kind decides is three bwrap flags, and the venue's argv is pinned against
// the REAL renderer in wallvenueargv_qa_test.go, which is the only place it
// can be read. What is pinned HERE is the table.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two files a tree can carry, and what each one makes the wall check
// answer. Named, because three arms plant them and one asserts the tail.
const (
	wckRedFile   = "wallboom.txt"
	wckQuietFile = "wallquiet.txt"
	wckRedTail   = "wall pins RED: 1 of 32 member(s) FAILed in this venue (fixture)"
	wckQuietTail = "wall pins NOT MEASURED: 18 of 32 member(s) abstained (fixture)"
)

// wckCheckBody is a wall check whose verdict is a property of the TREE, with
// one log line per run saying which copy ran and which tip it judged — the
// same witness landedcheck_test.go keeps, because "which copy ran" and "which
// tree was judged" are still the two questions underneath every arm.
//
// The three exits are ADR 0097 D3's, and the tail goes LAST on each: the
// runner keeps the last few non-empty lines of combined output and flattens
// them into a bead somebody reads later.
func wckCheckBody(version, log string) string {
	return "#!/bin/sh\n" +
		"printf '%s %s\\n' " + shq(version) + " \"$(git rev-parse HEAD)\" >> " + shq(log) + "\n" +
		"if [ -f " + wckRedFile + " ]; then\n" +
		"  echo " + shq(wckRedTail) + " >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"if [ -f " + wckQuietFile + " ]; then\n" +
		"  echo " + shq(wckQuietTail) + " >&2\n" +
		"  exit " + fmt.Sprint(wallCheckNoAnswerExit) + "\n" +
		"fi\n" +
		"echo 'wall check: green, 32 pins measured' >&2\n" +
		"exit 0\n"
}

// shipWallCheck writes a wall check into a tree and commits it path-limited,
// the way shipLandedCheck does one file over.
func shipWallCheck(t *testing.T, dir, body, msg string) {
	t.Helper()
	p := filepath.Join(dir, wallCheckScript)
	write(t, p, body)
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "--", wallCheckScript)
	mustGit(t, dir, "commit", "-q", "-m", msg, "--", wallCheckScript)
}

// wckJudgeBody is the landed check these arms ship, and its verdict is a
// property of the tree for landedfile_test.go's reason: the copy that judges
// comes from the tree last recorded GREEN (ADR 0088 D1), so a landing that
// changed the script could not move the verdict even if a fixture wanted it
// to. `boom.txt` is how an arm reds MAIN; the wall check's own two files are
// above.
func wckJudgeBody() string {
	return "#!/bin/sh\n" +
		"if [ -f boom.txt ]; then\n" +
		"  echo " + shq(ldfRedTail) + " >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 0\n"
}

// wckFixture is a repo on main shipping BOTH checks — the tree-reading judge
// and the tree-reading wall check above — with a checker wired to the plain
// cage and the fake bd, and the wall check's own run log.
func wckFixture(t *testing.T) (*landedChecker, *bytes.Buffer, string, string) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	log := filepath.Join(t.TempDir(), "wall-runs")
	shipLandedCheck(t, repo, wckJudgeBody(), "ranger-base-ship1: ship the landed check")
	shipWallCheck(t, repo, wckCheckBody("v1", log), "ranger-base-ship2: ship the wall check")
	write(t, filepath.Join(repo, "fake-show.json"), ldfStore)
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)
	c.cage, c.bd = plainCage, testBd(t)
	return c, out, repo, log
}

// wckRecorded is ADR 0097 D5's first record, and wckQuiet its second.
func wckRecorded(t *testing.T, repo string) (string, landVerdict) {
	t.Helper()
	return landedRecorded(wallCheckRule, repo)
}

func wckQuiet(t *testing.T, repo string) int {
	t.Helper()
	_, n := landedQuietRecorded(wallCheckRule, repo)
	return n
}

// wckGreen takes the repo to a recorded GREEN on BOTH records, which is the
// left-hand column of D5's second row and the far edge of every window below.
func wckGreen(t *testing.T, c *landedChecker, repo string) string {
	t.Helper()
	ldfJudge(t, c, repo)
	tree, v := wckRecorded(t, repo)
	if v != landGreen {
		t.Fatalf("the fixture did not reach a recorded green wall check: %q %s\n%s", tree, v, c.out)
	}
	return tree
}

// wckMarked is the open bead in `label` carrying this rule's marker, and a
// failure when there is not exactly one: "one open bead per stretch" is the
// whole dedupe claim, and the point of the markers being distinct is that a
// lane can hold the judge's bead and the wall check's at once.
func wckMarked(t *testing.T, c *landedChecker, repo, label, marker string) BdIssue {
	t.Helper()
	open, err := c.bd.OpenLabeledAny(repo, label)
	if err != nil {
		t.Fatalf("the %s lane could not be read back: %v", label, err)
	}
	var found []BdIssue
	for _, is := range open {
		if landedDescriptionCarries(is.Description, marker) {
			found = append(found, is)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d open %s bead(s) carry %q, want exactly 1 (ADR 0097 D5): %s", len(found), label, marker, ldfTitles(open))
	}
	return found[0]
}

func wckRed(t *testing.T, c *landedChecker, repo string) BdIssue {
	t.Helper()
	return wckMarked(t, c, repo, landedFileLabel, wallCheckRule.fileMarker(repo, "main"))
}

func wckSilent(t *testing.T, c *landedChecker, repo string) BdIssue {
	t.Helper()
	return wckMarked(t, c, repo, landedHealthLabel, wallCheckRule.healthMarker(repo))
}

// wckNone is the other half of every dedupe arm: no open bead in `label`
// carries this rule's marker at all.
func wckNone(t *testing.T, c *landedChecker, repo, label, marker string) {
	t.Helper()
	open, err := c.bd.OpenLabeledAny(repo, label)
	if err != nil {
		t.Fatalf("the %s lane could not be read back: %v", label, err)
	}
	for _, is := range open {
		if landedDescriptionCarries(is.Description, marker) {
			t.Fatalf("%s carries %q and nothing should have filed it: %s", is.ID, marker, is.Title)
		}
	}
}

// ─── ADR 0088 D5's TABLE, as ADR 0097 D5 reuses it ──────────────────────────

// Row 1 and row 2, in one walk, because the second is only a transition if
// the first happened: a green wall check is RECORDED and files nothing, and
// the first red after it files ONE attributed P1 naming every landing in the
// window.
//
// THE TITLE IS THE PINS AND NOT THE BRANCH (ADR 0097 D5, landedRule.RedSubject):
// a bead titled "main red at <tree>" would send its reader to `make test`,
// which is GREEN over these pins on every seat by design (ADR 0096 D3) — so
// the subject in the title is the one thing a reader of this bead has to get
// right before they start.
func TestAGreenWallCheckIsRecordedAndTheFirstRedFilesOneAttributedP1(t *testing.T) {
	t.Parallel()
	c, out, repo, log := wckFixture(t)
	green := wckGreen(t, c, repo)
	if runs := landedRuns(t, log); len(runs) != 1 {
		t.Fatalf("%d wall check run(s) over the first landing, want 1: %v", len(runs), runs)
	}
	wckNone(t, c, repo, landedFileLabel, wallCheckRule.fileMarker(repo, "main"))
	if !strings.Contains(out.String(), "wall pins green at") {
		t.Errorf("the pass does not say the wall pins were green:\n%s", out)
	}
	// D2's verdict line carries the venue argv and the wall — and, on green,
	// the body's own last line, because `wall check: green, <n> pins
	// measured` is the only place `n` is said and a green exit over zero
	// measured pins is the hole D3's third exit closes.
	for _, want := range []string{"venue: ", "wall check: green, 32 pins measured"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the green verdict line does not carry %q (ADR 0097 D2/D3):\n%s", want, out)
		}
	}

	// The two halves of the window, each green on its own base, newer last —
	// ldfBreak's shape, with the wall check's own red file.
	commitIn(t, repo, "half.txt", "the half that is green on its own base\n", ldfHalfMessage)
	older := mustGit(t, repo, "rev-parse", "HEAD")
	commitIn(t, repo, wckRedFile, "the other half\n", ldfBoomMessage)
	newer := mustGit(t, repo, "rev-parse", "HEAD")
	ldfJudge(t, c, repo)

	tree, v := wckRecorded(t, repo)
	if want := revTreeID(repo, newer); tree != want || v != landRed {
		t.Fatalf("%s reads %q %s, want %q red", wallCheckKey, tree, v, want)
	}
	is := wckRed(t, c, repo)
	if want := "wall pins red at " + abbrevSHA(tree); is.Title != want {
		t.Errorf("the bead is titled %q, want %q (ADR 0097 D5)", is.Title, want)
	}
	for _, want := range []string{
		wallCheckRule.fileMarker(repo, "main"), // the dedupe of record
		wckRedTail,                             // the cause, from the check's own tail
		wallCheckScript,                        // what judged it
		abbrevSHA(green),                       // the far edge of the window
		abbrevSHA(older), ldfOlderBead,         // and both landings in it
		abbrevSHA(newer), ldfNewerBead,
		"ADR 0096 D3", // why a seat's green `make test` is not a reproduction
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	// `-t bug` P1 `-l code`, assigned to the NEWEST landing's persona: D5's
	// three words and its assignment rule, unchanged by ADR 0097.
	if argv := ldfArgv(t); !strings.Contains(argv, ldfCreateFields(ldfNewerWho)) {
		t.Errorf("the create argv does not carry %q:\n%s", ldfCreateFields(ldfNewerWho), argv)
	}
	// AND THE COPY THAT RAN IS THE LAST GREEN TREE'S, not the red tree's
	// (ADR 0088 D1, kept by D3): one version string in the log for both runs.
	for _, r := range landedRuns(t, log) {
		if r.Version != "v1" {
			t.Errorf("a run used copy %q — the copy comes from the tree last recorded green: %v", r.Version, landedRuns(t, log))
		}
	}
}

// Row 3 and row 4: a red that stays red comments the new tree and files no
// second bead, and the red that goes green comments `green at <tree>` and
// leaves the bead OPEN — closing is the fixer's.
func TestARedWallThatStaysRedCommentsAndTheGreenOneLeavesTheBeadOpen(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := wckFixture(t)
	wckGreen(t, c, repo)
	commitIn(t, repo, wckRedFile, "red\n", ldfNewerBead+": the half that reds the pins")
	ldfJudge(t, c, repo)
	is := wckRed(t, c, repo)

	// Red -> red: a new tree, still red. One comment, and the SAME bead.
	commitIn(t, repo, "more.txt", "another landing onto a red wall\n", ldfOlderBead+": more work")
	ldfJudge(t, c, repo)
	again := wckRed(t, c, repo)
	if again.ID != is.ID {
		t.Fatalf("the second red filed %s beside %s — one open bead per red stretch (ADR 0097 D5)", again.ID, is.ID)
	}
	tree, v := wckRecorded(t, repo)
	if v != landRed {
		t.Fatalf("%s reads %s after the second red", wallCheckKey, v)
	}
	cs := ldfComments(t, c, repo, is.ID)
	if len(cs) != 1 || !strings.Contains(cs[0].Text, "still red at "+abbrevSHA(tree)) {
		t.Fatalf("the red→red row did not comment the new tree: %s", ldfCommentTexts(cs))
	}

	// Red -> green: the file goes, the pins measure, the bead is told and
	// stays open.
	mustGit(t, repo, "rm", "-q", "--", wckRedFile)
	mustGit(t, repo, "commit", "-q", "-m", ldfNewerBead+": fix the pins", "--", wckRedFile)
	ldfJudge(t, c, repo)
	tree, v = wckRecorded(t, repo)
	if v != landGreen {
		t.Fatalf("%s reads %q %s after the fix, want green", wallCheckKey, tree, v)
	}
	still := wckRed(t, c, repo) // still OPEN: closing is the fixer's
	if still.ID != is.ID {
		t.Fatalf("the green run filed %s rather than commenting %s", still.ID, is.ID)
	}
	cs = ldfComments(t, c, repo, is.ID)
	if len(cs) != 2 || !strings.Contains(cs[1].Text, "green at "+abbrevSHA(tree)) {
		t.Fatalf("the red→green row did not comment the green tree: %s", ldfCommentTexts(cs))
	}
	if !strings.Contains(cs[1].Text, "closing this bead is the fixer's job") {
		t.Errorf("the green comment does not say who closes the bead: %q", cs[1].Text)
	}
}

// Row 5, and ADR 0097 D3's whole reason for a third exit: a run that
// MEASURED NOTHING records nothing and files no red. An abstention is never a
// pass (D5) — and it is never a red either, because the tree did not do it.
//
// THE MUTANT THIS KILLS is `wallChildVerdict` returning what
// `landChildVerdict` returned: exit 3 would then be a red, and the ADR's own
// `/var/tmp` row — a fixture helper that took 18 pins dark — would file a P1
// against whoever landed last for a wall nobody asked a question.
func TestAWallCheckThatMeasuredNothingRecordsNothingAndFilesNoRed(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := wckFixture(t)
	green := wckGreen(t, c, repo)

	commitIn(t, repo, wckQuietFile, "a fixture helper took 18 pins dark\n", ldfNewerBead+": a helper change unrelated to the wall")
	ldfJudge(t, c, repo)

	if tree, v := wckRecorded(t, repo); tree != green || v != landGreen {
		t.Errorf("%s reads %q %s — silence records NOTHING, so the record must still name the last green tree %q", wallCheckKey, tree, v, green)
	}
	wckNone(t, c, repo, landedFileLabel, wallCheckRule.fileMarker(repo, "main"))
	if n := wckQuiet(t, repo); n != 1 {
		t.Errorf("%s counted %d after one unanswered run, want 1 — an abstention is counted, which is how it reaches a reader (ADR 0097 D5)", wallCheckQuietKey, n)
	}
	for _, want := range []string{"NOT MEASURED", wckQuietTail, fmt.Sprintf("exit %d", wallCheckNoAnswerExit)} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the pass does not say %q:\n%s", want, out)
		}
	}
	// The judge itself is untouched by any of this: one broken thing, one
	// subject.
	if _, v := landedRecorded(landedJudgeRule, repo); v != landGreen {
		t.Errorf("the judge's record reads %s over a tree whose wall pins abstained", v)
	}
}

// ─── ADR 0088 D6.3's TABLE, as ADR 0097 D5 reuses it ────────────────────────

// Rows 1 and 2: the SECOND consecutive unanswered run files one -t bug P1
// -l architecture bead titled "wall check silent at <repo>", and a third adds
// nothing. One silence files nothing — that is what D6.3's threshold is, and
// the ⚠ above is what D3 has for it.
func TestTwoSilentWallChecksFileOneArchitectureBeadAndAThirdAddsNothing(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := wckFixture(t)
	wckGreen(t, c, repo)

	commitIn(t, repo, wckQuietFile, "dark\n", ldfOlderBead+": the first silence")
	ldfJudge(t, c, repo)
	if n := wckQuiet(t, repo); n != 1 {
		t.Fatalf("%s counted %d after one silence, want 1", wallCheckQuietKey, n)
	}
	wckNone(t, c, repo, landedHealthLabel, wallCheckRule.healthMarker(repo))

	commitIn(t, repo, "second.txt", "another landing, still dark\n", ldfNewerBead+": the second silence")
	ldfJudge(t, c, repo)
	if n := wckQuiet(t, repo); n != 2 {
		t.Fatalf("%s counted %d after the second silence, want 2", wallCheckQuietKey, n)
	}
	is := wckSilent(t, c, repo)
	if want := "wall check silent at " + AbbrevHome(repo); is.Title != want {
		t.Errorf("the bead is titled %q, want %q (ADR 0097 D5)", is.Title, want)
	}
	for _, want := range []string{
		wallCheckRule.healthMarker(repo),
		wckQuietTail,                // the sentence the runs gave
		wallCheckScript,             // and what to run by hand
		landedCheckLimit.String(),   // the hang bound
		landedCheckCeiling.String(), // and the price
		"no landing's wall pins are measured anywhere",
		"ADR 0096 D3", // why no seat's suite would have said so
		"closing is architecture's",
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	if argv := ldfArgv(t); !strings.Contains(argv, lhCreateFields()) {
		t.Errorf("the create argv does not carry %q (unassigned, -l architecture):\n%s", lhCreateFields(), argv)
	}

	// Row 2: a third silence adds no second bead and no comment.
	before := ldfComments(t, c, repo, is.ID)
	commitIn(t, repo, "third.txt", "and again\n", ldfNewerBead+": the third silence")
	ldfJudge(t, c, repo)
	if again := wckSilent(t, c, repo); again.ID != is.ID {
		t.Fatalf("the third silence filed %s beside %s", again.ID, is.ID)
	}
	if after := ldfComments(t, c, repo, is.ID); len(after) != len(before) {
		t.Errorf("the third silence commented on the bead that already says so: %s", ldfCommentTexts(after))
	}
	if !strings.Contains(out.String(), "did not answer for the 3rd consecutive time") {
		t.Errorf("the pass does not say the third silence was covered:\n%s", out)
	}
}

// Rows 3 and 4: an answered run OVER the ceiling files the same class with
// its wall, and a later answered run comments on the open bead rather than
// filing a second one.
//
// The ceiling comes through the checker's seam, for the reason its own field
// comment gives: a 240s wall is unreachable inside a test, so without it the
// row that must exist — an answered run that IS over the price — would have
// no spelling here either.
func TestAnAnsweredWallCheckOverTheCeilingFilesWithItsWallThenComments(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := wckFixture(t)
	c.ceiling = 1 // every real run is over a one-nanosecond price
	wckGreen(t, c, repo)

	is := wckSilent(t, c, repo)
	if want := "wall check over its ceiling at " + AbbrevHome(repo); is.Title != want {
		t.Errorf("the bead is titled %q, want %q (ADR 0097 D5)", is.Title, want)
	}
	for _, want := range []string{
		wallCheckRule.healthMarker(repo),
		"ANSWERED at tree",
		"over its ceiling of " + landedCheckCeiling.String(),
		"closing is architecture's",
	} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("the bead does not name %q:\n%s", want, is.Description)
		}
	}
	// Row 4: the next answered run comments, and does not file.
	commitIn(t, repo, "again.txt", "an ordinary landing onto a slow wall check\n", ldfNewerBead+": ordinary work")
	ldfJudge(t, c, repo)
	if again := wckSilent(t, c, repo); again.ID != is.ID {
		t.Fatalf("the second crossing filed %s beside %s", again.ID, is.ID)
	}
	cs := ldfComments(t, c, repo, is.ID)
	if len(cs) != 1 || !strings.Contains(cs[0].Text, "still over the") {
		t.Fatalf("the second crossing did not comment its wall: %s", ldfCommentTexts(cs))
	}
	if !strings.Contains(out.String(), "wall pins green at") {
		t.Errorf("the pass does not say the pins were green while the price was over:\n%s", out)
	}
}

// ─── ADR 0097 D2's OWN THREE LINES ──────────────────────────────────────────

// The judge's verdict gates the step, and a red or silent judge records
// NOTHING and says why — which is what keeps one broken tree to one filed
// bead instead of two in two lanes.
func TestAWallCheckDoesNotRunWhenTheJudgeIsRedOrSilent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		why     string
		verdict string
		set     func(t *testing.T, c *landedChecker, repo string)
	}{
		{
			"the judge reds", landRed.String(),
			func(t *testing.T, c *landedChecker, repo string) {
				commitIn(t, repo, "boom.txt", "a landing that reds main's own doors\n", ldfNewerBead+": a landing that reds main")
			},
		},
		{
			"the judge's cage refuses", landUnanswered.String(),
			func(t *testing.T, c *landedChecker, repo string) { c.cage = refusedCage },
		},
	} {
		t.Run(tc.why, func(t *testing.T) {
			t.Parallel()
			c, out, repo, log := wckFixture(t)
			tc.set(t, c, repo)
			ldfJudge(t, c, repo)

			if runs := landedRuns(t, log); len(runs) != 0 {
				t.Fatalf("the wall check RAN behind a judge that answered %s: %v — ADR 0097 D2 runs it after a GREEN judgement", tc.verdict, runs)
			}
			if tree, v := wckRecorded(t, repo); tree != "" || v != landUnanswered {
				t.Errorf("%s reads %q %s — a step that did not run records nothing", wallCheckKey, tree, v)
			}
			if n := wckQuiet(t, repo); n != 0 {
				t.Errorf("%s counted %d — \"not run (judge %s)\" is neither a green nor a silence of its own, so one broken tree is filed once (ADR 0097 D2)", wallCheckQuietKey, n, tc.verdict)
			}
			wckNone(t, c, repo, landedFileLabel, wallCheckRule.fileMarker(repo, "main"))
			wckNone(t, c, repo, landedHealthLabel, wallCheckRule.healthMarker(repo))
			if want := fmt.Sprintf("wall check not run (judge %s)", tc.verdict); !strings.Contains(out.String(), want) {
				t.Errorf("the pass does not say %q:\n%s", want, out)
			}
		})
	}
}

// A tree that ships no `scripts/posse-wall-check` gets NOTHING — not a
// record, not a bead, and not a line. That is every repo until it ships one,
// and it is why every other landed-check pin in this package is unaffected by
// this step existing.
func TestATreeThatShipsNoWallCheckIsLeftAloneEntirely(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	shipLandedCheck(t, repo, "#!/bin/sh\nexit 0\n", "ranger-base-ship1: ship the landed check only")
	write(t, filepath.Join(repo, "fake-show.json"), ldfStore)
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)
	c.cage, c.bd = plainCage, testBd(t)
	ldfJudge(t, c, repo)

	if _, v := landedRecorded(landedJudgeRule, repo); v != landGreen {
		t.Fatalf("the judge did not reach green, so this arm measured nothing:\n%s", out)
	}
	if tree, v := wckRecorded(t, repo); tree != "" || v != landUnanswered {
		t.Errorf("%s reads %q %s at a repo that never opted in", wallCheckKey, tree, v)
	}
	if s := out.String(); strings.Contains(s, "wall") {
		t.Errorf("the pass says something about the wall at a repo that ships no %s:\n%s", wallCheckScript, s)
	}
}

// A HOST WHOSE WALL IS NOT A BWRAP RUNS NO VENUE, AND HAS NOT GONE SILENT.
//
// This is the second of the three questions `wall` asks in order (wallcheck.go's
// header), and it was the one with no row: ADR 0097 D2 says the step DOES NOT
// EXIST where the wall mechanism is not bwrap (ADR 0096: no darwin change), so
// it says so, records nothing, and counts nothing. "Counts nothing" is the
// load-bearing half — a host that was never asked to answer has not gone
// silent, and a counted silence here reaches D5's watchdog as "wall check
// silent at <repo>", a bead filed against nobody, on every landing, forever.
//
// MEASURED 2026-11-14 (ranger-base-rs6qo, verifying ranger-base-wplhv):
// deleting that guard outright left every other arm of both tables green, and
// so did deleting the venue's own sandbox-exec refusal in boxLandedCage — the
// second is held by wallvenueargv_qa_test.go's `case WallSandboxExec` arm,
// which only a darwin host takes. `wallMechanismForTest` is what makes the
// first readable from here, which is what it exists for (ranger-base-temhf).
//
// SERIAL, no t.Parallel: wallMechanismForTest writes a package-level var.
func TestAHostWhoseWallIsNotABwrapRunsNoVenueAndCountsNoSilence(t *testing.T) {
	c, out, repo, log := wckFixture(t)
	wallMechanismForTest(t, "darwin")
	if m := WallMechanism(); m != WallSandboxExec {
		t.Fatalf("WallMechanism() reads %q behind wallMechanismForTest(darwin), so this arm cannot put the step on a host that is not a bwrap and measures nothing", m)
	}
	ldfJudge(t, c, repo)

	// The positive control, and this arm is worthless without it: the JUDGE
	// still ran and answered green, so the step was reached and declined
	// rather than never asked.
	if _, v := landedRecorded(landedJudgeRule, repo); v != landGreen {
		t.Fatalf("the judge did not reach green, so this arm measured nothing:\n%s", out)
	}
	if runs := landedRuns(t, log); len(runs) != 0 {
		t.Fatalf("the wall check RAN on a host whose wall is %s: %v — ADR 0097 D1's venue is a bwrap, so the step does not exist here", WallSandboxExec, runs)
	}
	if tree, v := wckRecorded(t, repo); tree != "" || v != landUnanswered {
		t.Errorf("%s reads %q %s on a host where the step does not exist — a step that cannot run records nothing", wallCheckKey, tree, v)
	}
	if n := wckQuiet(t, repo); n != 0 {
		t.Errorf("%s counted %d on a host whose wall is %s — a host that was never asked to answer has not gone silent, and a counted silence here files D5's watchdog bead against nobody on every landing (ADR 0097 D2)", wallCheckQuietKey, n, WallSandboxExec)
	}
	wckNone(t, c, repo, landedFileLabel, wallCheckRule.fileMarker(repo, "main"))
	wckNone(t, c, repo, landedHealthLabel, wallCheckRule.healthMarker(repo))
	for _, want := range []string{"wall check not run", WallSandboxExec, "the step does not exist here"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the pass does not say %q, so a reader cannot tell a host decision from a step that silently did nothing:\n%s", want, out)
		}
	}
}

// AND THE TWO RULES ARE TWO STRETCHES IN ONE LANE (ADR 0097 D5's one hard
// requirement about the markers). Both ratchets file `-l code`, so a marker
// either rule could match would make a red wall comment on a red main's bead
// — one stretch's bead carrying another's verdicts, and neither stretch
// filing its own.
//
// The walk: the wall pins red first (the judge is green, so the step runs),
// then main itself reds. Two beads, each carrying its own marker and neither
// carrying the other's.
func TestARedWallAndARedMainAreTwoBeadsInOneLaneWithTwoMarkers(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := wckFixture(t)
	wckGreen(t, c, repo)

	commitIn(t, repo, wckRedFile, "the pins red first\n", ldfOlderBead+": reds the pins")
	ldfJudge(t, c, repo)
	wall := wckRed(t, c, repo)

	// Now main's own doors red, which stops the wall check running at all.
	commitIn(t, repo, "boom.txt", "and now main's own doors\n", ldfNewerBead+": reds main")
	ldfJudge(t, c, repo)
	main := wckMarked(t, c, repo, landedFileLabel, landedJudgeRule.fileMarker(repo, "main"))

	if main.ID == wall.ID {
		t.Fatalf("one bead carries both markers (%s) — the two stretches would then share a dedupe", main.ID)
	}
	if landedDescriptionCarries(wall.Description, landedJudgeRule.fileMarker(repo, "main")) {
		t.Errorf("the wall bead carries the judge's marker:\n%s", wall.Description)
	}
	if landedDescriptionCarries(main.Description, wallCheckRule.fileMarker(repo, "main")) {
		t.Errorf("the judge's bead carries the wall check's marker:\n%s", main.Description)
	}
	if cs := ldfComments(t, c, repo, wall.ID); len(cs) != 0 {
		t.Errorf("the red main commented on the wall check's bead: %s", ldfCommentTexts(cs))
	}
	// And the records are two: the wall's still red, the judge's now red too.
	if _, v := wckRecorded(t, repo); v != landRed {
		t.Errorf("%s reads %s", wallCheckKey, v)
	}
	if _, v := landedRecorded(landedJudgeRule, repo); v != landRed {
		t.Errorf("%s reads %s", landedCheckKey, v)
	}
}

// With no green tree on record, the landed tree's own copy runs and the
// launcher says so ONCE per rule — the judge's sentence and the wall check's
// are two sentences, because one launcher runs both and a single flag would
// have the first spend the second's.
func TestWithNoGreenOnRecordEachRuleSaysItRanItsOwnCopyOnce(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := wckFixture(t)
	wckGreen(t, c, repo)
	for _, want := range []string{
		"is judged by its OWN " + landedCheckScript,
		"is judged by its OWN " + wallCheckScript,
	} {
		if n := strings.Count(out.String(), want); n != 1 {
			t.Errorf("the pass says %q %d time(s), want exactly 1:\n%s", want, n, out)
		}
	}
	// A second landing has both records green, so neither sentence repeats.
	commitIn(t, repo, "next.txt", "more\n", ldfNewerBead+": more work")
	before := out.String()
	ldfJudge(t, c, repo)
	if added := strings.TrimPrefix(out.String(), before); strings.Contains(added, "its OWN") {
		t.Errorf("a landing with both records green said it ran its own copy:\n%s", added)
	}
}

// ADR 0097 D3's third exit as a RULE, row by row: only a RED of exactly 3 is
// reconsidered, and every other answer keeps the sentence landChildVerdict
// built for it.
//
// THE ARM ABOVE DRIVES THIS END TO END and this one reads it without a
// process, which is the half that can say what the rule does NOT do: a
// `-run` of the fake script can only produce the exits the fake script has.
func TestQAWallThirdExitReadsOnlyARedThree(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		why   string
		v     landVerdict
		said  string
		exit  int
		want  landVerdict
		keeps bool // the sentence is landChildVerdict's, unchanged
	}{
		{"a red that exited 3 is no answer", landRed, wckQuietTail, wallCheckNoAnswerExit, landUnanswered, false},
		{"a red that exited 1 is a red", landRed, wckRedTail, 1, landRed, true},
		{"a red that exited 2 is a red", landRed, "unknown argument", 2, landRed, true},
		{"a green is a green", landGreen, "wall check: green, 32 pins measured", 0, landGreen, true},
		// The arms landChildVerdict already answered unanswered for: a
		// deadline, a signal, a child that would not start. Each has a
		// sentence saying WHICH, and this rule may not replace it with a
		// sentence about an exit status the child never had.
		{"a deadline keeps its own sentence", landUnanswered, "no answer in 10m0s, signalled (…)", -1, landUnanswered, true},
		{"a signal keeps its own sentence", landUnanswered, "it was signalled (signal: killed)", -1, landUnanswered, true},
		{"a child that would not run keeps its own sentence", landUnanswered, "it would not run: no such file", -1, landUnanswered, true},
	} {
		got, said := wallThirdExit(c.v, c.said, c.exit)
		if got != c.want {
			t.Errorf("%s: wallThirdExit(%v, exit=%d) = %v, want %v", c.why, c.v, c.exit, got, c.want)
			continue
		}
		if c.keeps && said != c.said {
			t.Errorf("%s: the sentence became %q, want it unchanged (%q)", c.why, said, c.said)
		}
		if !c.keeps {
			for _, want := range []string{fmt.Sprintf("exit %d", wallCheckNoAnswerExit), "no answer", c.said} {
				if !strings.Contains(said, want) {
					t.Errorf("%s: the sentence does not carry %q: %q", c.why, want, said)
				}
			}
		}
	}
	// An empty tail still says something, because this sentence is what a
	// watchdog bead carries and "it said nothing" is a finding.
	if _, said := wallThirdExit(landRed, "", wallCheckNoAnswerExit); !strings.Contains(said, "it said nothing") {
		t.Errorf("a silent exit 3 reads %q", said)
	}
}

// AND THE QA SIBLING of landedcheck_test.go's: this file's runner starts
// exactly one child and it is the venue's rendered argv. ADR 0097 D2 has no
// uncaged arm — "a venue that will not render" is an unanswered run (D5), not
// a reason to run the body here.
func TestQATheWallCheckExecsNothingButTheCagesOwnArgv(t *testing.T) {
	t.Parallel()
	qaExecsOnlyTheCagesArgv(t, "wallcheck.go")
}
