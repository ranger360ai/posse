//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-y13h7 — "only installing closes it" and "nothing needs a human"
// in the same view.
//
// THE INCIDENT, and it is a silence inside a surface that was speaking. The
// launcher-lag reading has printed on every `posse status` and at every
// doubling of a watch pass since ranger-base-z3hx6, ending in the words "only
// installing closes it". Installing is a live change reserved to the operator
// (guardrail 3), so that sentence is a request for a human — and the
// governance summary two lines under it said `nothing needs a human`, because
// GovReport's all-clear answers an EMPTY GovSet and no condition read the lag
// at all. MEASURED 2026-10-05: four days of that reading, ~/.local/bin/posse
// at 101 commits behind main, every pass naming the figure and nobody acting
// on it.
//
// WHAT THESE PINS HOLD. The unit arms in launcherlag_test.go prove the NUMBER
// and launcherlag_qa_test.go proves the watch log says it; this file is about
// the number reaching the one surface whose question is "does anything need a
// human" — and about the all-clear being unable to print beneath it, which is
// the bead's own done-when and the only arm that reproduces the symptom.
//
// WHY THEY HAND THE READING IN. The same reason dispatch.Lag is a seam: the
// reading keys off VersionString(), a test binary carries no vcs stamp at all,
// so every test in this package sees "+dev" and would pin the abstention only.
// GovInputs.Lag is that seam here. The candidate-list arm below is the one
// claim a seam cannot hold, and it is pinned one level down for that reason.

import (
	"bytes"
	"strings"
	"testing"
)

// lagGov is a governance fixture whose launcher reading is a planted repo
// behind by n commits. Everything else is govIn's hermetic shop: a fake bd
// over a scratch dir, a scratch herdr, a frozen clock.
func lagGov(t *testing.T, n int) GovInputs {
	t.Helper()
	b, _ := newTestBackend(t)
	in := govIn(t, b)
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, n)
	lag := FindLauncher("0.4.0+"+stamp, []string{repo})
	if !lag.Known() || lag.Behind != n {
		t.Fatalf("fixture is not %d behind: behind=%d (%s)", n, lag.Behind, lag.Why)
	}
	in.Lag = func() LauncherLag { return lag }
	return in
}

// THE BEAD'S OWN DONE-WHEN: the all-clear cannot print beneath a lagging
// launcher. Both arms matter and the control is what makes the other one mean
// anything — the fixture CAN produce "nothing needs a human", and does, right
// up to the depth where the lag becomes a condition.
//
// MUTATION: delete the G11 block in ShopCheck → the deep arm prints "nothing
// needs a human" under a 16-commit lag, which is the bug, red. Make the row
// URGENT instead of LANE → still green here and correct to be: the claim is
// that the set is not empty, not which class it lands in.
func TestTheAllClearCannotPrintBeneathALaggingLauncher(t *testing.T) {
	t.Parallel()
	var clear, deep bytes.Buffer
	GovReport(&clear, shopSet(t, lagGov(t, DefaultLauncherBehindMax-1)), nil)
	GovReport(&deep, shopSet(t, lagGov(t, DefaultLauncherBehindMax)), nil)

	if !strings.Contains(clear.String(), "nothing needs a human") {
		t.Fatalf("the control arm raised something else — this fixture cannot state the claim:\n%s", clear.String())
	}
	got := deep.String()
	if strings.Contains(got, "nothing needs a human") {
		t.Errorf("a launcher %d commits behind printed the all-clear — this IS the bead:\n%s",
			DefaultLauncherBehindMax, got)
	}
	// And the row carries the remedy, re-runnable: the next thing anyone asks
	// is "which fixes", and the answer must not need the rev re-derived.
	if !strings.Contains(got, "only installing closes it") || !strings.Contains(got, "log --oneline") {
		t.Errorf("the row does not name the gap or the remedy:\n%s", got)
	}
}

// The threshold, both sides of it. The boundary is the measured number — a
// pin that only asserted "deep raises a row" would be green with the
// comparison inverted at its own edge.
func TestG11FiresAtTheThresholdAndNotBelowIt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		behind int
		want   bool
	}{
		{1, false},
		{DefaultLauncherBehindMax - 1, false},
		{DefaultLauncherBehindMax, true}, // exactly the bound, already a condition
		{DefaultLauncherBehindMax + 1, true},
	} {
		set := shopSet(t, lagGov(t, c.behind))
		row := find(set, "G11")
		if (row != nil) != c.want {
			t.Errorf("%d behind: G11 present = %v, want %v (keys %v)", c.behind, row != nil, c.want, set.Keys())
			continue
		}
		if row == nil {
			continue
		}
		// LANE on ADR 0029's own definition: URGENT means the shop is
		// stopped, and a stale launcher stops nothing — it keeps running,
		// with the defects its own repo fixed.
		if row.Class != GovLane {
			t.Errorf("%d behind: class = %s, want %s", c.behind, row.Class, GovLane)
		}
		if row.Row() != "G11" {
			t.Errorf("%d behind: row renders %q — ADR 0029's table carries this one by name", c.behind, row.Row())
		}
	}
}

// A lag of zero is not a condition, and this is the arm that keeps the
// threshold from being the only thing standing between a fresh install and a
// row. A binary IS built from the tip; both times it was measured the count
// at the instant of the build was 0 (launcherlag.go's header).
func TestG11IsSilentForACurrentLauncher(t *testing.T) {
	t.Parallel()
	in := lagGov(t, 0)
	if row := find(shopSet(t, in), "G11"); row != nil {
		t.Errorf("a current launcher raised %q", row.Key)
	}
	// And at a threshold of zero it STILL says nothing: `0` means "any lag
	// at all", not "a launcher that is the tip".
	appendConfig(t, in.App, "launcher_behind_max: 0\n")
	if row := find(shopSet(t, in), "G11"); row != nil {
		t.Errorf("launcher_behind_max: 0 raised a row over a current launcher: %q", row.Key)
	}
}

// The KEY carries the doubling step, and the key is what the pulse
// fingerprints: a lag that keeps deepening re-prompts at 16, 32, 64 and
// nowhere in between. Same steps the watch log's drumbeat prints on, so the
// pass line and the row escalate together.
//
// MUTATION: put Behind in the key → every commit that lands re-prompts the
// coordinator, red here. Drop the bucket and key on the threshold alone → a
// lag that goes from 16 to 180 never says so again, red on the last rows.
func TestG11KeyCarriesTheDoublingStep(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		behind int
		want   string
	}{
		{16, "launcher-behind:16"},
		{31, "launcher-behind:16"},
		{32, "launcher-behind:32"},
		{63, "launcher-behind:32"},
		{64, "launcher-behind:64"},
		{101, "launcher-behind:64"}, // the depth the bead was filed at
	} {
		row := find(shopSet(t, lagGov(t, c.behind)), "G11")
		if row == nil {
			t.Fatalf("%d behind raised no G11", c.behind)
		}
		if row.Key != c.want {
			t.Errorf("%d behind: key = %q, want %q", c.behind, row.Key, c.want)
		}
	}
}

// The step is the CONFIGURED threshold doubled, not a power of two: an
// instance that moved the dial gets its own cadence rather than one keyed to
// a default it rejected.
func TestLagBucketDoublesTheConfiguredThreshold(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ behind, max, want int }{
		{20, 20, 20},
		{39, 20, 20},
		{40, 20, 40},
		{100, 20, 80},
		// A zero threshold still buckets, from a floor of 1.
		{5, 0, 4},
		{1, 0, 1},
		// Below the threshold the bucket IS the threshold. No row reads it
		// there — GovRows has already returned nil — and it is pinned so the
		// floor cannot drift into a bucket of zero.
		{3, 16, 16},
	} {
		if got := lagBucket(c.behind, c.max); got != c.want {
			t.Errorf("lagBucket(%d, %d) = %d, want %d", c.behind, c.max, got, c.want)
		}
	}
}

// `launcher_behind_max:` moves the line, and a typo does not move it silently:
// the default stands and the operator is told, because a threshold nobody can
// see is worse than a wrong one.
func TestLauncherBehindMaxIsConfigurable(t *testing.T) {
	t.Parallel()
	in := lagGov(t, 40)
	appendConfig(t, in.App, "launcher_behind_max: 64\n")
	if row := find(shopSet(t, in), "G11"); row != nil {
		t.Errorf("a configured depth of 64 still raised a row at 40 behind: %q", row.Key)
	}

	var errw bytes.Buffer
	typo := lagGov(t, 40)
	appendConfig(t, typo.App, "launcher_behind_max: soon\n")
	typo.Errw = &errw
	row := find(shopSet(t, typo), "G11")
	if row == nil {
		t.Fatal("a typo'd threshold stood the row down — a value nobody can parse must leave the default in place")
	}
	if !strings.Contains(errw.String(), "launcher_behind_max") {
		t.Errorf("the typo was swallowed: %q", errw.String())
	}
	if !strings.Contains(row.Detail, "launcher_behind_max: 16") {
		t.Errorf("the row does not say what it was judged against:\n%s", row.Detail)
	}
}

// AN ABSTENTION RAISES NOTHING AND IS NOT A PARTIAL SET. "+dev" is the real
// shape of it — a `go run`, or a plain `go build` from a linked worktree,
// names no commit at all — so filing it in `failed` would make every
// operator's `go run ./cmd/posse status` exit non-zero over a build working
// exactly as designed. The rule it would otherwise fall under is satisfied
// out loud elsewhere: `posse status` prints Line() in every case, and the
// watch preamble says it once.
//
// MUTATION: report the abstention as an unreadable store → failed is
// non-empty, red.
func TestG11AbstainsOnAStampThatNamesNoCommit(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	in := govIn(t, b)
	repo, _ := posseRig(t, posseModule)
	landCommits(t, repo, 100)
	in.Lag = func() LauncherLag { return FindLauncher("0.4.0+dev", []string{repo}) }

	set, failed := ShopCheck(in)
	if row := find(set, "G11"); row != nil {
		t.Errorf("an unreadable stamp raised %q", row.Key)
	}
	for _, err := range failed {
		if strings.Contains(err.Error(), "G11") || strings.Contains(err.Error(), "launcher") {
			t.Errorf("the abstention was reported as an unreadable store: %v", err)
		}
	}
	// The control: the same repo and a real stamp DOES raise one, or the arm
	// above would be green over a fixture that could never raise anything.
	ok := lagGov(t, 100)
	if find(shopSet(t, ok), "G11") == nil {
		t.Fatal("the control arm raised no row — the abstention arm proves nothing")
	}
}

// The seam defaults to the real thing. Without this arm every pin above would
// pass with ShopCheck wired to nothing at all, and the shipped surface would
// take no reading.
//
// It asserts the ABSTENTION on purpose, and for the reason
// TestQAWatchDefaultsToThisInstancesOwnReading does: a test binary carries no
// vcs stamp, so this instance's own reading is "+dev" whatever the tree holds
// — and a set with no G11 in it over a config whose `beads:` is a scratch dir
// is proof the check asked THIS instance rather than a fixture.
//
// MUTATION: drop the `if in.Lag != nil` fallback → in.lag() returns a zero
// LauncherLag, Known() is TRUE over a zero Why, and Behind 0 keeps it silent
// — so this arm is green either way and the one below is the one that bites.
func TestShopCheckDefaultsToThisInstancesOwnLauncherReading(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	in := govIn(t, b)
	if in.Lag != nil {
		t.Fatal("fixture set a reading; this pin is about the check taking its own")
	}
	if row := find(shopSet(t, in), "G11"); row != nil {
		t.Errorf("a scratch instance raised %q — the check is not reading this binary", row.Key)
	}
	if got := in.lag(); got.Known() || !strings.Contains(got.Why, VersionString()) {
		t.Errorf("in.lag() = %+v, want this instance's own abstention naming %q", got, VersionString())
	}
}

// THE CANDIDATE LIST, which is the one claim no seam can hold and the one
// that keeps this surface's pins off the operator's live lag.
//
// G11's reading counts over the CONFIGURED checkouts alone. A test binary
// built from a worktree of this repo carries that worktree's stamp, and cwd
// in a test is the package directory, whose MainCheckout is the operator's
// real checkout — so a governance reading that fell through to cwd would
// count every `posse status` pin in the tree against the operator's `main`
// and go red per HOUR rather than per commit (the class ranger-base-rp2y cost
// a day to). It is also what the row claims: a condition is about the fleet
// this instance dispatches, and the fleet is `beads:`.
//
// MUTATION: give LauncherOverFleet the cwd candidate → red here, and
// TestStatusClearShopExitsZero in cmd/posse goes red the next time the
// operator is 16 commits behind.
func TestTheGovernanceReadingDoesNotFallThroughToTheProcessCwd(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)

	fleet := b.App.launcherCandidates(false)
	if len(fleet) != 1 || fleet[0] != dir {
		t.Fatalf("the governance candidates are %q, want exactly the configured [%q]", fleet, dir)
	}
	line := b.App.launcherCandidates(true)
	if len(line) != 2 || line[0] != dir {
		t.Fatalf("the status-line candidates are %q, want the configured dir plus the cwd", line)
	}
	if line[1] == dir {
		t.Fatal("the cwd candidate is the configured dir — this fixture cannot tell the two lists apart")
	}
}
