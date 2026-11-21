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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// landedArmFixture is a dispatcher whose one configured repo ships a landed
// check, with nothing on record: the restart case, before the loop arms.
//
// `beads:` names the repo itself, because the sweep under test derives its
// repos the way every landing path does (BeadsDirs → mainCheckoutsOf).
func landedArmFixture(t *testing.T) (d *Dispatcher, repo, log string) {
	t.Helper()
	d, repos, logs := landedArmFixtureIn(t, 1, func(repos []string) []string { return repos })
	return d, repos[0], logs[0]
}

// landedArmFixtureIn is that fixture with both of its derivations spelled by
// the caller: `n` repos, each shipping its own landed check into its own run
// log, and `beads:` holding whatever dirs cfg makes of them.
//
// THE CONFIGURED DIRS ARE A PARAMETER because the two narrowing arms at the
// end of this file are about the two steps between `beads:` and the repos
// this sweep judges, and a config that names exactly one repo by its own
// root cannot reach either (ranger-base-x78s5, findings 3 and 4, escaped
// from ranger-base-d9xby): a `--dir` filter needs a second configured repo
// to exclude, and `mainCheckoutsOf` needs a configured dir that is not
// already a main checkout. Every other arm here passes the repos through,
// which is what landedArmFixture does.
//
// The equality check below is ranger-base-isq3's rule and matters more here
// than at most fixtures: an absent or misspelled key is not an error and not
// a red — BeadsDirs answers [""], and this sweep would then judge whatever
// repo the test binary's cwd sits in, which for `go test ./internal/posse`
// is this worktree and resolves to the live checkout.
func landedArmFixtureIn(t *testing.T, n int, cfg func(repos []string) []string) (d *Dispatcher, repos, logs []string) {
	t.Helper()
	b, _ := newTestBackend(t)
	d = newTestDispatcher(t, b)
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		repo := wtRepo(t)
		log := filepath.Join(dir, fmt.Sprintf("runs-%d", i))
		// The hold file is never written, so the check answers and returns:
		// what these arms count is runs, and holding one open measures the
		// join instead (TestTheLandingReturnsBeforeTheCheckFinishes does
		// that). The version is per repo so a run in the wrong log names
		// the copy that produced it.
		shipLandedCheck(t, repo, landedCheckBody(fmt.Sprintf("v%d", i+1), log, filepath.Join(dir, "hold"), 0), "ship the landed check")
		repos = append(repos, repo)
		logs = append(logs, log)
	}
	want := cfg(repos)
	if err := os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+strings.Join(want, "\n  - ")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := b.App.BeadsDirs(); !slices.Equal(got, want) {
		t.Fatalf("fixture is not hermetic: BeadsDirs() = %q, want %q — the sweep below would read the live checkout", got, want)
	}
	// Before Watch, so the loop's own landedChecks() returns this checker:
	// it is made on first use and this access is the first (landedChecks).
	d.landedChecks().cage = plainCage
	for _, repo := range repos {
		if tree, v := landedRecorded(landedJudgeRule, repo); tree != "" || v != landUnanswered {
			t.Fatalf("the fixture starts with %s = %q %s in %s — every arm here is about a tip NOTHING has judged", landedCheckKey, tree, v, repo)
		}
	}
	return d, repos, logs
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
	return landedArmWatchIn(t, d, wantPasses, "")
}

// landedArmWatchIn is that, with the loop's own `--dir` filter — the one
// argument of Watch this sweep reads (landedarm.go's dirFilter), and ""
// everywhere but finding 3's arm.
func landedArmWatchIn(t *testing.T, d *Dispatcher, wantPasses int, dirFilter string) string {
	t.Helper()
	tap := newPassTap(wantPasses)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = d.Watch(ctx, dirFilter, "", 0, 10*time.Millisecond, 20*time.Millisecond)
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

// ─── the two narrowings between `beads:` and the repos this sweep judges ────
//
// Both were UNPINNED until ranger-base-x78s5 (findings 3 and 4, escaped from
// ranger-base-d9xby's close): deleting `if dirFilter != ""` and replacing
// `mainCheckoutsOf(dirs)` with a bare `range dirs` were each GREEN over all
// five arms above, because every one of those arms drives
// `d.Watch(ctx, "", …)` and configures one repo by its own root. landedarm.go
// calls the pair load-bearing in the same sentence — "a repo this sweep names
// is exactly a repo a landing could name, and the dirFilter narrows both the
// same way" — so what each arm here holds is one half of that sentence.

// A `--dir X` loop arms over X AND NOTHING ELSE. Without the narrowing it
// schedules a judge — minutes of caged `make` each — for every other repo the
// box has configured, which is the opposite of what the operator typed.
//
// TWO REPOS AND ONE FILTER, so the verdict is a reading and not an absence:
// the filtered repo is judged and the other is not, in the same loop and the
// same pass. The mutant judges both, and the second judgement is a run in a
// log this arm reads as empty.
func TestAWatchLoopArmingWithADirFilterJudgesOnlyThatRepo(t *testing.T) {
	t.Parallel()
	d, repos, logs := landedArmFixtureIn(t, 2, func(repos []string) []string { return repos })
	named, other := repos[1], repos[0]
	tip := mustGit(t, named, "rev-parse", "HEAD")

	s := landedArmWatchIn(t, d, 2, named)

	runs := landedRuns(t, logs[1])
	if len(runs) != 1 || runs[0].Tip != tip {
		t.Fatalf("the filtered repo's inherited tip was not judged at arm: %+v (want one run over %s):\n%s", runs, abbrevSHA(tip), s)
	}
	if got := landedRuns(t, logs[0]); len(got) != 0 {
		t.Fatalf("a repo OUTSIDE `--dir %s` was judged at arm (%+v) — `beads:` names it, but a loop narrowed to one repo lands into that one repo, so the tips it inherits are that repo's (landedarm.go: \"the dirFilter narrows both the same way\"):\n%s", named, got, s)
	}
	if tree, v := landedRecorded(landedJudgeRule, named); tree != revTreeID(named, tip) || v != landGreen {
		t.Errorf("%s = %q %s in the filtered repo, want its inherited tip's tree recorded green", landedCheckKey, tree, v)
	}
	if tree, v := landedRecorded(landedJudgeRule, other); tree != "" || v != landUnanswered {
		t.Errorf("%s = %q %s in the repo outside the filter, want nothing recorded", landedCheckKey, tree, v)
	}
}

// THE REPOS ARE `mainCheckoutsOf(BeadsDirs())`, not the configured dirs: a
// configured dir is RESOLVED to the main checkout whose tip a landing there
// would move. The mutant — a bare `range dirs` — judges the configured dir
// itself, at ITS OWN branch, which for a LINKED WORKTREE is a tree no landing
// lands into: `repoBranch` answers there, so `Schedule` does not drop the
// pair, and the judge then spends minutes of caged `make` writing a verdict
// over a tip nothing will ever read it for.
//
// THE CONFIGURED DIR IS A LINKED WORKTREE ON AN ORDINARY BRANCH, moved off
// main by one commit, and the verdict is WHICH TIP was judged. Two earlier
// shapes read identically under the mutant and are worth not re-deriving
// (both MEASURED 2026-11-21, ranger-base-x78s5 finding 4):
//
//   - The bead's own recipe, a SUBDIRECTORY of the repo configured alone, on
//     the reading that the mutant's judge would find no check to extract
//     there. It finds one: a judge handed `<repo>/docs` reads
//     `<branch>:scripts/posse-landed-check`, which git resolves against the
//     repository ROOT and not the cwd, then judges the same tip and records
//     through `git config` in the same repo. Mutant green.
//   - The same repo configured TWICE, by its root and by that subdirectory,
//     asserting one run for the dedupe half of `mainCheckoutsOf`. D4's cache
//     absorbs it: the second judge reads the record the first one just wrote
//     — the same `git config`, both spellings — and returns. Mutant green.
//
// So the reading has to be a tree, and the dir has to be one whose own branch
// is a DIFFERENT tree. `posse/…` is the prefix that would make this worktree
// a session tree (SessionTreesIn), and an ordinary branch name is what keeps
// the premise every arm in this file relies on — no session worktree, so no
// landing can happen in any of these passes, and every run in the log is the
// arm's.
func TestAWatchLoopArmingOverAConfiguredWorktreeJudgesItsMainCheckout(t *testing.T) {
	t.Parallel()
	var wt string
	d, repos, logs := landedArmFixtureIn(t, 1, func(repos []string) []string {
		wt = filepath.Join(t.TempDir(), "sidecar")
		mustGit(t, repos[0], "worktree", "add", "-q", "-b", "sidecar", wt)
		commitIn(t, wt, "sidecar.txt", "a branch no landing lands into\n", "move the sidecar branch off main")
		return []string{wt}
	})
	repo, log := repos[0], logs[0]
	if got, ok := MainCheckout(wt); !ok || resolveExisting(got) != resolveExisting(repo) {
		t.Fatalf("MainCheckout(%s) = %q %v, want %q — this arm is about that resolution and has nothing to read without it", wt, got, ok, repo)
	}
	if b := repoBranch(wt); b == "" {
		t.Fatal("repoBranch answers nothing in the configured worktree, so `Schedule` would DROP the mutant's pair for the reason the detached arm above pins — and this arm would then pass over the mutant it is written for")
	}
	tip := mustGit(t, repo, "rev-parse", "HEAD")
	if side := mustGit(t, wt, "rev-parse", "HEAD"); side == tip {
		t.Fatal("the configured worktree's branch is at main's tip, so judging the worktree and judging the repo are the same reading and this arm cannot tell them apart")
	}

	s := landedArmWatch(t, d, 2)

	runs := landedRuns(t, log)
	if len(runs) != 1 || runs[0].Tip != tip {
		t.Fatalf("the main checkout of a configured worktree (%s) was not judged at arm: %+v (want one run over %s, main's tip) — this sweep's repos are BeadsDirs through mainCheckoutsOf, the enumeration every landing path makes, and a tip judged at any other branch is a tree no landing lands into:\n%s", wt, runs, abbrevSHA(tip), s)
	}
	if strings.Contains(s, wt) {
		t.Errorf("the loop's own log names the configured worktree %s as a repo it judged:\n%s\nA dir this sweep names must be one a landing could name, and a landing names the main checkout.", wt, s)
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); tree != revTreeID(repo, tip) || v != landGreen {
		t.Errorf("%s = %q %s after the loop armed, want main's inherited tip's tree recorded green", landedCheckKey, tree, v)
	}
}
