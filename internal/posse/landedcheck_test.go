//go:build posse_arm2

package posse

// ADR 0088 D1-D4 at the site it runs: after a landing the launcher reads
// main's new tree, caged, outside the lock, one run at a time, coalesced to
// the newest tip and cached by tree id (ranger-base-5fond; landedcheck.go's
// header has the incident and the five differences from the land check).
//
// THE FIXTURE IS A REAL CHILD, for landcheck_test.go's reason and one more.
// The thing being pinned is that posse extracts a judge OUT OF GIT, creates
// a detached worktree, and runs that judge over that worktree — so a stubbed
// verdict would pin the plumbing and nothing about WHICH BYTES get judged or
// WHICH TREE they are judged over. Every check here writes one line per run
// into a log outside the repo: its own version, and the tip it found in its
// cwd. That log is the whole witness — "which copy ran" and "which tip it
// judged" are the two questions D1 and D4 are about, and nothing else in the
// system can answer either.
//
// THE CAGE IS THE ONE THING STUBBED, and it is stubbed through the seam the
// production code carries for it (landedCageFn). Two reasons, both of them
// about what a real wall would measure here: the suite would then be pinning
// bwrap's mount semantics rather than this runner, and the ONE arm most
// worth having — D3's "a refused cage gives no answer and never runs
// uncaged" — is unreachable on a box where the cage works. The stub is not a
// bypass: `plainCage` still returns the judge as the argv, so the uncaged
// arm the production code has no spelling for has none here either, and
// TestQATheLandedCheckExecsNothingButTheCagesOwnArgv holds that structurally
// over the real file.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// landedCheckBody is a check that announces itself and the tip it is
// judging, then optionally holds the run open while a file exists — which is
// how a test can be inside a run rather than beside it.
//
// `git rev-parse HEAD` and not a baked-in sha: the contract is that cwd is
// the tree to judge, so the check asking its own cwd is the only reading
// that can tell "judged the newest tip" from "judged the tip posse captured
// when the landing happened".
func landedCheckBody(version, log, hold string, exit int) string {
	return "#!/bin/sh\n" +
		"printf '%s %s\\n' " + shq(version) + " \"$(git rev-parse HEAD)\" >> " + shq(log) + "\n" +
		"n=0\n" +
		"while [ -f " + shq(hold) + " ] && [ \"$n\" -lt 600 ]; do\n" +
		"  n=$((n + 1))\n" +
		"  sleep 0.05\n" +
		"done\n" +
		fmt.Sprintf("exit %d\n", exit)
}

// shipLandedCheck writes a check into a tree and commits it path-limited,
// the way every other fixture in this package commits.
func shipLandedCheck(t *testing.T, dir, body, msg string) {
	t.Helper()
	p := filepath.Join(dir, landedCheckScript)
	write(t, p, body)
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "--", landedCheckScript)
	mustGit(t, dir, "commit", "-q", "-m", msg, "--", landedCheckScript)
}

// landedRun is one line of the log: which copy ran, and over which tip.
type landedRun struct{ Version, Tip string }

// landedRuns reads the log back. The log is the only witness the fixture
// has, and an absent one is zero runs rather than an error: "the check never
// ran" is the assertion three arms here are about.
func landedRuns(t *testing.T, log string) []landedRun {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		return nil
	}
	var runs []landedRun
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if f := strings.Fields(ln); len(f) == 2 {
			runs = append(runs, landedRun{f[0], f[1]})
		}
	}
	return runs
}

// plainCage is the stub: the child as its own argv, with the environment the
// renderer was handed. It renders no wall, which is exactly what makes it
// unfit for production and fit for a suite — see this file's header.
//
// IT IGNORES THE KIND (ADR 0097 D1's flag), and that is right for every arm
// that uses it: what the kind decides is three bwrap flags, and a stub that
// renders no bwrap has nowhere to put them. The venue's own argv is pinned
// against the REAL renderer instead (wallvenueargv_qa_test.go), which is the
// only place it can be read.
func plainCage(_ *App, _ landedCageKind, child, _ string, _ []string, env []string) (landedCage, error) {
	return landedCage{Argv: []string{child}, Env: env}, nil
}

// refusedCage is D3's bottom row: a mechanism that will not render.
func refusedCage(_ *App, _ landedCageKind, _, _ string, _ []string, _ []string) (landedCage, error) {
	return landedCage{}, Die("no cage renders on this host (fixture)")
}

// landedCheckFixture is a repo on main shipping a check, a checker wired to
// the plain cage, and the paths the check writes and watches.
func landedCheckFixture(t *testing.T, exit int) (c *landedChecker, out *bytes.Buffer, repo, log, hold string) {
	t.Helper()
	a := wtApp(t)
	repo = wtRepo(t)
	d := t.TempDir()
	log, hold = filepath.Join(d, "runs"), filepath.Join(d, "hold")
	shipLandedCheck(t, repo, landedCheckBody("v1", log, hold, exit), "ship the landed check")
	out = &bytes.Buffer{}
	c = newLandedChecker(a, out)
	c.cage = plainCage
	return c, out, repo, log, hold
}

// landedWaitFor polls a condition the CHILD satisfies, so a test can act
// from inside a run. Bounded, and a timeout is a failure rather than a
// skip: every use of it here is waiting on a check this test started.
func landedWaitFor(t *testing.T, why string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", why)
}

// THE HEADLINE (D1): a landed tree is judged, the verdict is printed, and
// the tree id is recorded.
func TestALandedTreeIsJudgedAndItsVerdictRecorded(t *testing.T) {
	t.Parallel()
	c, out, repo, log, _ := landedCheckFixture(t, 0)
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	c.Schedule(repo, "main")
	c.Wait()

	runs := landedRuns(t, log)
	if len(runs) != 1 || runs[0].Tip != tip {
		t.Fatalf("the landed tree was not judged: %+v (want one run over %s)", runs, abbrevSHA(tip))
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != revTreeID(repo, tip) || v != landGreen {
		t.Errorf("%s = %q %s, want the tip's tree recorded green — D4's cache is what makes a coalesced run free", landedCheckKey, tree, v)
	}
	if s := out.String(); !strings.Contains(s, abbrevSHA(tip)) || !strings.Contains(s, "green") {
		t.Errorf("the pass said %q, which does not name the tip and its verdict — this slice PRINTS the verdict and that line is the whole record until ranger-base-ddgfr files", s)
	}
}

// A red is a red, with the check's own last lines as the cause — the
// sentence ranger-base-ddgfr turns into a bead.
func TestALandedTreeThatFailsTheCheckIsRecordedRedWithItsCause(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	log := filepath.Join(t.TempDir(), "runs")
	// Spelled out rather than built by landedCheckBody, because this one is
	// about the TAIL: what the bead reads under D5 is the last non-empty
	// lines of combined output, so the fixture prints the cause last and
	// nothing after it.
	shipLandedCheck(t, repo,
		"#!/bin/sh\n"+
			"printf '%s %s\\n' v1 \"$(git rev-parse HEAD)\" >> "+shq(log)+"\n"+
			"echo 'corpus-check: the register is stale' >&2\n"+
			"exit 1\n", "ship a red check")
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)
	c.cage = plainCage
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	c.Schedule(repo, "main")
	c.Wait()

	if tree, v := landedRecorded(landedJudgeRule, repo); tree != revTreeID(repo, tip) || v != landRed {
		t.Errorf("%s = %q %s, want the tip's tree recorded red", landedCheckKey, tree, v)
	}
	s := out.String()
	for _, want := range []string{"RED", abbrevSHA(tip), "the register is stale"} {
		if !strings.Contains(s, want) {
			t.Errorf("the red line does not name %q:\n%s", want, s)
		}
	}
	if n := len(landedRuns(t, log)); n != 1 {
		t.Errorf("the check ran %d times for one landing", n)
	}
}

// landedWallSaid is D6.2's wall as a verdict line says it: seconds with one
// decimal, which is the unit the 240s ceiling is written in. A regexp and
// not a literal because the number is a MEASUREMENT — what has to hold is
// that one is printed, not what this box took.
var landedWallSaid = regexp.MustCompile(`\b\d+\.\ds\b`)

// D6.2 (ADR 0088's amendment, ranger-base-7idk4): EVERY verdict line prints
// its wall — the green, the red, and the ⚠ that says there is no verdict at
// all. The ceiling is 240s and it is READ off these lines instead of
// enforced by a kill, so a line without a wall leaves the price unreadable,
// which is exactly the defect the amendment was written for: while one number
// was both the kill and the price, the only signal a crossing could produce
// was the absence of one.
//
// The unanswered arm is in here beside the two verdicts on purpose. That is
// the line the 2026-11-12 crossing actually printed, and the question a
// reader had then — "was this a hang, or was it slow?" — is answerable only
// if the silence carries a wall too.
func TestEveryVerdictLinePrintsItsWall(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		arm        func(*testing.T) (*landedChecker, *bytes.Buffer, string)
	}{
		{"green", "green at", func(t *testing.T) (*landedChecker, *bytes.Buffer, string) {
			c, out, repo, _, _ := landedCheckFixture(t, 0)
			return c, out, repo
		}},
		{"red", "is RED at", func(t *testing.T) (*landedChecker, *bytes.Buffer, string) {
			c, out, repo, _, _ := landedCheckFixture(t, 1)
			return c, out, repo
		}},
		{"no answer", "NOT judged", func(t *testing.T) (*landedChecker, *bytes.Buffer, string) {
			c, out, repo, _, _ := landedCheckFixture(t, 0)
			c.cage = refusedCage
			return c, out, repo
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, out, repo := tc.arm(t)
			c.Schedule(repo, "main")
			c.Wait()
			s := out.String()
			if !strings.Contains(s, tc.want) {
				t.Fatalf("the %s arm printed no verdict line (want one naming %q):\n%s", tc.name, tc.want, s)
			}
			// The wall is matched against the verdict LINE and not the
			// whole buffer, which is what this pin's name claims and what
			// D6.2 says (ranger-base-mdku2). Against the buffer, a wall
			// moved onto a line of its own — or any other number shaped
			// like one, anywhere in the output — keeps the pin green while
			// the verdict line it names carries nothing.
			line := tc.want
			for _, l := range strings.Split(s, "\n") {
				if strings.Contains(l, tc.want) {
					line = l
					break
				}
			}
			if !landedWallSaid.MatchString(line) {
				t.Errorf("the %s verdict line carries no wall, so D6.2's %s ceiling cannot be read off it — and reading these lines is the only reader that ceiling has, now that nothing kills on it:\n%s\nwhole output:\n%s",
					tc.name, landedCheckCeiling, line, s)
			}
		})
	}
}

// D6.1 and D6.2 as a RELATION, which is the part of the amendment neither
// number can say on its own: the kill has to sit ABOVE the price. An
// answered run is the only reader the ceiling has, so a limit at or under it
// means every crossing is spelled as silence again — the 2026-11-12 defect
// exactly, with new numbers in it.
func TestTheHangBoundSitsAboveThePrice(t *testing.T) {
	t.Parallel()
	if landedCheckLimit <= landedCheckCeiling {
		t.Errorf("landedCheckLimit is %s and landedCheckCeiling is %s — a kill at or under the price can only ever report a crossing as silence (ADR 0088 D6.1/D6.2)",
			landedCheckLimit, landedCheckCeiling)
	}
}

// DONE-WHEN (1): runs coalesce to the NEWEST tip. Two landings arrive while
// one check is in flight; when it ends, exactly one more run happens and it
// judges the newest tip — not the one that was current when the landing was
// scheduled, and not both.
func TestRunsCoalesceToTheNewestTip(t *testing.T) {
	t.Parallel()
	c, _, repo, log, hold := landedCheckFixture(t, 0)
	write(t, hold, "held\n")
	first := mustGit(t, repo, "rev-parse", "HEAD")

	c.Schedule(repo, "main")
	landedWaitFor(t, "the first check to start", func() bool { return len(landedRuns(t, log)) == 1 })

	// Two landings while the first run is in flight. The second is the one
	// the next run must judge; the first must never be judged at all.
	commitIn(t, repo, "b.txt", "a landing mid-check\n", "main: b")
	skipped := mustGit(t, repo, "rev-parse", "HEAD")
	c.Schedule(repo, "main")
	commitIn(t, repo, "c.txt", "and another\n", "main: c")
	newest := mustGit(t, repo, "rev-parse", "HEAD")
	c.Schedule(repo, "main")

	if err := os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	c.Wait()

	runs := landedRuns(t, log)
	if len(runs) != 2 {
		t.Fatalf("%d runs for three landings, want 2 — D4 is at most one in flight, and one successor for everything that arrived while it ran: %+v", len(runs), runs)
	}
	if runs[0].Tip != first {
		t.Errorf("the first run judged %s, want %s", abbrevSHA(runs[0].Tip), abbrevSHA(first))
	}
	if runs[1].Tip != newest {
		t.Errorf("the second run judged %s, want the NEWEST tip %s — D4 says the next run judges the newest tip only", abbrevSHA(runs[1].Tip), abbrevSHA(newest))
	}
	for _, r := range runs {
		if r.Tip == skipped {
			t.Errorf("%s was judged, and it is the tip two landings went past — coalescing that judges every intermediate tip is not coalescing", abbrevSHA(skipped))
		}
	}
}

// D4's cache: a tree already on record is not judged again. This is what
// makes a coalesced run free when the landings left the tree where it was,
// and it is invisible without the counter — a cached green and a measured
// green print the same line.
func TestATreeAlreadyOnRecordIsNotJudgedAgain(t *testing.T) {
	t.Parallel()
	c, _, repo, log, _ := landedCheckFixture(t, 0)

	c.Schedule(repo, "main")
	c.Wait()
	c.Schedule(repo, "main")
	c.Wait()

	if n := len(landedRuns(t, log)); n != 1 {
		t.Errorf("the check ran %d times over one unchanged tree — D4 keys the record on the TREE because the check reads bytes, and the bytes did not move", n)
	}
}

// DONE-WHEN (2): the judge is the LAST-GREEN tree's copy, not the landed
// tree's own. A landing that rewrites the check does not change what judges
// it (D1).
func TestTheJudgeIsTheLastGreenTreesCopyAndNotTheLandedTrees(t *testing.T) {
	t.Parallel()
	c, _, repo, log, hold := landedCheckFixture(t, 0)

	// One green landing, so a tree is on record.
	c.Schedule(repo, "main")
	c.Wait()
	green := mustGit(t, repo, "rev-parse", "HEAD")

	// Now main lands a check that says it is v2. If the landed tree's own
	// copy judged, the log's second line would say so.
	shipLandedCheck(t, repo, landedCheckBody("v2", log, hold, 0), "main: weaken the check")
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	c.Schedule(repo, "main")
	c.Wait()

	runs := landedRuns(t, log)
	if len(runs) != 2 {
		t.Fatalf("%d runs, want 2: %+v", len(runs), runs)
	}
	if runs[1].Version != "v1" {
		t.Errorf("the second run was judged by %q, want v1 — the copy from %s, the tree last recorded green. A landing that weakens the script must not change what judges it (ADR 0088 D1)",
			runs[1].Version, abbrevSHA(green))
	}
	if runs[1].Tip != tip {
		t.Errorf("the second run judged %s, want the landed tip %s — the last-green tree supplies the JUDGE, never the tree judged", abbrevSHA(runs[1].Tip), abbrevSHA(tip))
	}
}

// …and with nothing on record it is the landed tree's own copy, said once.
func TestWithNoGreenOnRecordTheLandedTreeJudgesItselfAndSaysSo(t *testing.T) {
	t.Parallel()
	c, out, repo, log, _ := landedCheckFixture(t, 0)

	c.Schedule(repo, "main")
	c.Wait()
	if runs := landedRuns(t, log); len(runs) != 1 || runs[0].Version != "v1" {
		t.Fatalf("the first ever run did not use the landed tree's own copy: %+v", runs)
	}
	if s := out.String(); !strings.Contains(s, "OWN") {
		t.Errorf("the first run said %q, which does not say that the tree judged itself — D1 says the absent case is said once", s)
	}
	// Said ONCE: a second landing with a green now on record takes the
	// recorded copy and says nothing about it.
	commitIn(t, repo, "b.txt", "a second landing\n", "main: b")
	before := out.Len()
	c.Schedule(repo, "main")
	c.Wait()
	if s := out.String()[before:]; strings.Contains(s, "OWN") {
		t.Errorf("the second run said it again:\n%s", s)
	}
}

// DONE-WHEN (3): a refused cage gives NO ANSWER, records nothing, and never
// runs the judge uncaged. The last clause is the one that matters — a check
// that fell back would execute a landed tree's `make` as the launcher, which
// is the one thing D3 spells out by hand.
func TestARefusedCageGivesNoAnswerAndNeverRunsUncaged(t *testing.T) {
	t.Parallel()
	c, out, repo, log, _ := landedCheckFixture(t, 0)
	c.cage = refusedCage
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	c.Schedule(repo, "main")
	c.Wait()

	if runs := landedRuns(t, log); len(runs) != 0 {
		t.Fatalf("the judge RAN with no cage rendered (%+v) — ADR 0088 D3: it never falls back to running uncaged", runs)
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != "" || v != landUnanswered {
		t.Errorf("%s = %q %s after a run that could not be made, want nothing recorded — a cache entry written over silence is read by the next run as a judgement", landedCheckKey, tree, v)
	}
	s := out.String()
	if !strings.Contains(s, "⚠") || !strings.Contains(s, "NOT be judged") && !strings.Contains(s, "NOT judged") {
		t.Errorf("a cage that would not render printed %q, want a ⚠ saying the tree was not judged", s)
	}
	if strings.Contains(s, "RED") {
		t.Errorf("silence was reported as a verdict:\n%s", s)
	}
	if !strings.Contains(s, abbrevSHA(tip)) {
		t.Errorf("the ⚠ does not name the tip it could not judge:\n%s", s)
	}
}

// DONE-WHEN (4): the landing returns before the check finishes. Driven
// through the dispatcher's own landing site, which is the call every landing
// arm makes (`scheduleLandedCheck`), so what is measured is the thing the
// lock-holder does and not a helper beside it.
func TestTheLandingReturnsBeforeTheCheckFinishes(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	d := t.TempDir()
	log, hold := filepath.Join(d, "runs"), filepath.Join(d, "hold")
	shipLandedCheck(t, repo, landedCheckBody("v1", log, hold, 0), "ship the landed check")
	write(t, hold, "held\n")

	out := &bytes.Buffer{}
	dd := &Dispatcher{App: a, Out: out}
	dd.landedChecks().cage = plainCage
	tr := &SessionTree{Repo: repo, Path: repo, Branch: "posse/s-1", Base: "main"}

	start := time.Now()
	dd.scheduleLandedCheck(tr, MergeOutcome{Merged: true, Commits: 1})
	took := time.Since(start)

	// It is in flight: the child has written its line and is holding.
	landedWaitFor(t, "the check to start", func() bool { return len(landedRuns(t, log)) == 1 })
	if took > 2*time.Second {
		t.Errorf("the landing spent %s inside the landed check — ADR 0088 D2: the landing has already happened and the check never blocks it", took)
	}
	if _, v := landedRecorded(landedJudgeRule, repo); v != landUnanswered {
		t.Errorf("a verdict was on record while the check was still running (%s), so the landing had waited for it after all", v)
	}

	if err := os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	dd.landedChecks().Wait()
	if n := len(landedRuns(t, log)); n != 1 {
		t.Errorf("%d runs after the pass joined, want 1", n)
	}
	if _, v := landedRecorded(landedJudgeRule, repo); v != landGreen {
		t.Errorf("the verdict is %s after the join — the pass waits for its own child, which is what keeps a caged `make` from outliving the process that started it", v)
	}
}

// A repo that ships no check gets nothing, which is every repo until it opts
// in (D1, residual (3)) — and in particular no worktree, no ⚠ and no record.
func TestARepoThatShipsNoLandedCheckIsLeftAlone(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)
	c.cage = plainCage

	c.Schedule(repo, "main")
	c.Wait()

	if s := out.String(); s != "" {
		t.Errorf("a repo with no %s said:\n%s", landedCheckScript, s)
	}
	if _, v := landedRecorded(landedJudgeRule, repo); v != landUnanswered {
		t.Errorf("a repo with no check has a verdict on record (%s)", v)
	}
	if out, err := git(repo, "worktree", "list"); err == nil && strings.Count(out, "\n") != 0 {
		t.Errorf("a repo with no check was given a worktree:\n%s", out)
	}
}

// The detached worktree is the launcher's, and it is REMOVED — both the
// directory and the repo's administrative record of it. A runner that leaves
// one behind per landing fills the state dir and leaves `git worktree list`
// naming paths that are gone.
func TestTheDetachedWorktreeIsRemovedAfterTheRun(t *testing.T) {
	t.Parallel()
	c, _, repo, _, _ := landedCheckFixture(t, 1)

	c.Schedule(repo, "main")
	c.Wait()

	if out := mustGit(t, repo, "worktree", "list"); strings.Count(strings.TrimSpace(out), "\n") != 0 {
		t.Errorf("the landed check's worktree is still registered:\n%s", out)
	}
	ents, err := os.ReadDir(landedCheckWorkDir(c.a))
	if err != nil {
		return // never made, which is also "nothing left behind"
	}
	if len(ents) != 0 {
		t.Errorf("%d run director(ies) left under %s", len(ents), AbbrevHome(landedCheckWorkDir(c.a)))
	}
}

// Concurrent landings never put two checks in flight (D4's first sentence).
// The check writes its line and holds; if a second run were started beside
// it, two lines would appear before anything is released.
func TestAtMostOneCheckIsInFlight(t *testing.T) {
	t.Parallel()
	c, _, repo, log, hold := landedCheckFixture(t, 0)
	write(t, hold, "held\n")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.Schedule(repo, "main") }()
	}
	wg.Wait()
	landedWaitFor(t, "a check to start", func() bool { return len(landedRuns(t, log)) >= 1 })
	if n := len(landedRuns(t, log)); n != 1 {
		t.Errorf("%d checks are in flight at once, want 1 — eight landings are one question about the tip they leave behind", n)
	}
	if err := os.Remove(hold); err != nil {
		t.Fatal(err)
	}
	c.Wait()
	if n := len(landedRuns(t, log)); n != 1 {
		// One run for eight landings, and the successor the loop does start
		// costs nothing: by then the record names this tree and D4's cache
		// answers without a child. Eight landings over an unmoved tree are
		// one question, asked once.
		t.Errorf("%d runs for eight concurrent landings over one unmoved tree, want 1", n)
	}
}

// THE STRUCTURAL HALF of done-when (3), over the real file: the only child
// this runner starts is the cage's own argv. An uncaged fallback added
// tomorrow would be a second exec whose program is a path this package
// computed, and nothing in a green test run would say so — D3's rule is
// about a branch that does not exist rather than one that is taken.
func TestQATheLandedCheckExecsNothingButTheCagesOwnArgv(t *testing.T) {
	t.Parallel()
	qaExecsOnlyTheCagesArgv(t, "landedcheck.go")
}

// qaExecsOnlyTheCagesArgv is that reading over one source file, because ADR
// 0097 D2's wall check has the same rule and the same sentence to print
// (wallcheck_test.go): one child, and its program is the cage's rendered
// argv. One implementation, so the two cannot drift into reading different
// shapes — which is what would make the newer of the two the weaker.
func qaExecsOnlyTheCagesArgv(t *testing.T, src string) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	execs := 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "exec" || !strings.HasPrefix(sel.Sel.Name, "Command") {
			return true
		}
		execs++
		// The program is the cage's rendered argv and nothing else. `exec`
		// takes it as the first argument after the context, so the argument
		// list is scanned rather than indexed: a Command/CommandContext
		// split would otherwise make this pin read the wrong operand and
		// pass over the very edit it exists to catch.
		for _, a := range call.Args {
			ix, ok := a.(*ast.IndexExpr)
			if !ok {
				continue
			}
			if s, ok := ix.X.(*ast.SelectorExpr); ok && s.Sel.Name == "Argv" {
				return true
			}
		}
		t.Errorf("%s:%d execs a program that is not the cage's rendered argv — ADR 0088 D3 has no uncaged arm, and a fallback added here would run a landed tree's `make` as the launcher",
			src, call.Pos())
		return true
	})
	if execs != 1 {
		t.Errorf("%s has %d exec call(s); this file starts exactly one child and it is the caged one", src, execs)
	}
}
