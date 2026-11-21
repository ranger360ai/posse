//go:build posse_arm2

package posse

// ADR 0088 D6.6 at the site it fires: a watch loop that ARMS schedules one
// landed check per repo it lands into, at the base it lands to
// (ranger-base-d9xby, design ranger-base-gihmm; landedarm.go has the
// derivation and the incident).
//
// WHAT THE DEFECT WAS. `once()` ran only when a landing scheduled it, so the
// tip a new loop INHERITED was read by nobody: after the 00:08:53 restart on
// 2026-11-21, main's tip was unjudged and would have stayed that way until
// something landed. These arms are driven through `d.Watch` and not through
// `scheduleInheritedTips` for exactly that reason — the bead is about a
// control nothing called, so a pin that called the sweep directly would have
// been green over the defect.
//
// THE PREMISE EVERY ARM HERE RELIES ON: the fixture repo holds no session
// worktree, so no landing can happen in any of these passes and the landing
// path can schedule nothing. Every run that appears in the log is therefore
// the arm's, and every arm that expects none is reading a loop that armed and
// found nothing to ask.
//
// THE CONTROL IS IN THE CACHE-HIT ARM and it is the reason that arm is one
// test rather than two: "no judge ran" is also what a sweep wired to nothing
// says, so that arm moves the tip and asserts the judge DOES run, in the
// same fixture and the same loop shape.
//
// The cage is the one thing stubbed, through the production seam
// (landedCageFn) — landedcheck_test.go's header says why, and the judge is a
// real child process here as it is there.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// landedArmFixture is a dispatcher whose one configured repo ships a landed
// check, with nothing on record: the restart case, before the loop arms.
//
// `beads:` names the repo itself, because the sweep under test derives its
// repos the way every landing path does (BeadsDirs → mainCheckoutsOf). The
// equality check below is ranger-base-isq3's rule and matters more here than
// at most fixtures: an absent or misspelled key is not an error and not a
// red — BeadsDirs answers [""], and this sweep would then judge whatever
// repo the test binary's cwd sits in, which for `go test ./internal/posse`
// is this worktree and resolves to the live checkout.
func landedArmFixture(t *testing.T) (d *Dispatcher, repo, log string) {
	t.Helper()
	b, _ := newTestBackend(t)
	d = newTestDispatcher(t, b)
	repo = wtRepo(t)
	dir := t.TempDir()
	log = filepath.Join(dir, "runs")
	// The hold file is never written, so the check answers and returns: what
	// these arms count is runs, and holding one open measures the join
	// instead (TestTheLandingReturnsBeforeTheCheckFinishes does that).
	shipLandedCheck(t, repo, landedCheckBody("v1", log, filepath.Join(dir, "hold"), 0), "ship the landed check")
	if err := os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := b.App.BeadsDirs(); len(got) != 1 || got[0] != repo {
		t.Fatalf("fixture is not hermetic: BeadsDirs() = %q, want [%q] — the sweep below would read the live checkout", got, repo)
	}
	// Before Watch, so the loop's own landedChecks() returns this checker:
	// it is made on first use and this access is the first (landedChecks).
	d.landedChecks().cage = plainCage
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != "" || v != landUnanswered {
		t.Fatalf("the fixture starts with %s = %q %s — every arm here is about a tip NOTHING has judged", landedCheckKey, tree, v)
	}
	return d, repo, log
}

// landedArmWatch arms the loop, waits for `wantPasses` pass headers, stops it
// and JOINS it.
//
// TWO PASSES IS THE ORDERING ARGUMENT, not a cadence: the arm-time sweep runs
// before pass 1, and `Run` defers `landedChecks().Wait()` at its top, so by
// the time pass 2's header is printed any judge the arm scheduled has
// finished and recorded. Reading the log after one pass header would race the
// child, and stopping the loop over a live judge would classify it D6.5
// INTERRUPTED — which records nothing, and is a different arm's subject
// (landedinterrupt_test.go).
func landedArmWatch(t *testing.T, d *Dispatcher, wantPasses int) string {
	t.Helper()
	tap := newPassTap(wantPasses)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond)
	}()
	// A hang guard and not a budget: the real cost is a git worktree, a
	// trivial shell child and two 10ms passes.
	select {
	case <-tap.reached:
	case <-time.After(90 * time.Second):
		t.Fatalf("the loop never reached %d pass(es):\n%s", wantPasses, tap.String())
	}
	cancel()
	select {
	case <-done:
	case <-time.After(90 * time.Second):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}
	return tap.String()
}

// THE HEADLINE (D6.6): a loop that arms over a tip nothing has judged judges
// it, with no landing anywhere — which is the restart ranger-base-gihmm
// measured, where the old behaviour came back for that tip only at the next
// landing.
func TestAWatchLoopThatArmsJudgesTheTipItInherited(t *testing.T) {
	t.Parallel()
	d, repo, log := landedArmFixture(t)
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	s := landedArmWatch(t, d, 2)

	runs := landedRuns(t, log)
	if len(runs) != 1 || runs[0].Tip != tip {
		t.Fatalf("the inherited tip was not judged at arm: %+v (want one run over %s, and no landing could have scheduled one — this repo holds no session worktree)", runs, abbrevSHA(tip))
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != revTreeID(repo, tip) || v != landGreen {
		t.Errorf("%s = %q %s after the loop armed, want the inherited tip's tree recorded green", landedCheckKey, tree, v)
	}
	if !strings.Contains(s, abbrevSHA(tip)) || !strings.Contains(s, "green") {
		t.Errorf("the loop's own log does not name the tip it judged and its verdict:\n%s", s)
	}
}

// D6.6's cost, and the control that makes the first half of it mean
// something: a tip ALREADY on record costs one `git config` read and no
// judge, and the same fixture over a tip that has MOVED judges it. Without
// the second half, "no judge ran" is equally what a sweep wired to nothing
// would say.
func TestAWatchLoopThatArmsOverARecordedTipRunsNoJudgeAndJudgesOneThatMoved(t *testing.T) {
	t.Parallel()
	d, repo, log := landedArmFixture(t)
	tip := mustGit(t, repo, "rev-parse", "HEAD")
	if err := recordLandedCheck(landedJudgeRule, repo, revTreeID(repo, tip), landGreen); err != nil {
		t.Fatal(err)
	}

	s := landedArmWatch(t, d, 2)

	if runs := landedRuns(t, log); len(runs) != 0 {
		t.Fatalf("a recorded tip was judged again at arm (%+v) — D4's cache is the whole reason an arm-time sweep is affordable:\n%s", runs, s)
	}

	// The tip MOVES, with no loop running — the landing a restart inherits.
	commitIn(t, repo, "moved.txt", "landed while no loop was running\n", "a landing nothing judged")
	moved := mustGit(t, repo, "rev-parse", "HEAD")
	if moved == tip {
		t.Fatal("the fixture did not move the tip, so the arm below is the cache hit again")
	}

	s = landedArmWatch(t, d, 2)

	runs := landedRuns(t, log)
	if len(runs) != 1 || runs[0].Tip != moved {
		t.Fatalf("the moved tip was not judged at arm: %+v (want one run over %s) — the zero above was the cache and not an unwired sweep:\n%s", runs, abbrevSHA(moved), s)
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != revTreeID(repo, moved) || v != landGreen {
		t.Errorf("%s = %q %s, want the moved tip's tree recorded green", landedCheckKey, tree, v)
	}
}

// "A one-shot `posse dispatch` does not do this" (D6.6, last sentence). It
// has no loop to arm, and its own landings schedule their own checks — so a
// typed pass over the same inherited tip judges nothing, which is what keeps
// this amendment to the loop it is about.
func TestAOneShotPassDoesNotJudgeTheTipItInherited(t *testing.T) {
	t.Parallel()
	d, repo, log := landedArmFixture(t)
	var out strings.Builder
	d.Out = &out

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("pass: %v\n%s", err, out.String())
	}

	if runs := landedRuns(t, log); len(runs) != 0 {
		t.Fatalf("a one-shot pass judged the tip it inherited (%+v):\n%s", runs, out.String())
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != "" || v != landUnanswered {
		t.Errorf("%s = %q %s after a one-shot pass, want nothing recorded", landedCheckKey, tree, v)
	}
}

// A DRY RUN SCHEDULES NOTHING. The judge writes D4's record, may file D5's
// bead and spends minutes of box: all three are acting, which is the rule the
// pass's own filing sweeps keep. The loop is real here — the arm is about
// what `--dry-run --watch` does, not about the flag being readable.
func TestADryRunWatchLoopSchedulesNoLandedCheck(t *testing.T) {
	t.Parallel()
	d, repo, log := landedArmFixture(t)
	d.DryRun = true

	s := landedArmWatch(t, d, 2)

	if runs := landedRuns(t, log); len(runs) != 0 {
		t.Fatalf("a --dry-run loop judged the tip it inherited (%+v):\n%s", runs, s)
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != "" || v != landUnanswered {
		t.Errorf("%s = %q %s after a dry run, want nothing recorded — a dry run does not write a record", landedCheckKey, tree, v)
	}
}

// A repo whose HEAD is DETACHED has no base, so there is no tip it could
// have inherited: `repoBranch` answers "" and `Schedule` drops the pair. The
// one derivation in this sweep that can fail, pinned so that it fails as a
// skip rather than as a judgement of whatever branch git would have guessed
// — and so that a loop still arms over the other repos beside it.
func TestAWatchLoopArmingOverADetachedCheckoutSchedulesNothing(t *testing.T) {
	t.Parallel()
	d, repo, log := landedArmFixture(t)
	mustGit(t, repo, "checkout", "--detach", "-q")
	if b := repoBranch(repo); b != "" {
		t.Fatalf("the fixture is not detached: repoBranch = %q", b)
	}

	s := landedArmWatch(t, d, 2)

	if runs := landedRuns(t, log); len(runs) != 0 {
		t.Fatalf("a detached checkout was judged at arm (%+v) — it has no base to land to:\n%s", runs, s)
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != "" || v != landUnanswered {
		t.Errorf("%s = %q %s over a detached checkout, want nothing recorded", landedCheckKey, tree, v)
	}
}
