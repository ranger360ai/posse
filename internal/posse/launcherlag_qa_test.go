package posse

// ranger-base-z3hx6 / ranger-base-pju9t — the eight-hour install lag,
// pinned at the surface that is meant to catch it.
//
// THE INCIDENT. ~/.local/bin/posse was built 2026-09-03 23:54 from c592683
// and stayed that way until 07:16 — 34 commits behind main by the end. Two
// of those 34 each independently stop a merge-back block being re-filed
// against one branch (67effd0 emgdb, c3ab918 j8qmj). Both landed inside the
// window; the running binary had neither, so the block was filed a FOURTH
// time at 07:08 and a dispatched seat spent a session re-deriving a
// do-not-land verdict that two commits on main already held. Nothing said
// the launcher was behind: the stale binary does not fail, and `git log`
// shows a perfect main.
//
// WHY THESE PINS DRIVE A REAL LOOP RATHER THAN CALLING THE READING. The
// unit arms in launcherlag_test.go prove the number; what cost eight hours
// was that no SURFACE printed it. The claim under test here is about the
// watch loop's output, so the loop is what is run.
//
// WHY THEY HAND THE READING IN. It keys off VersionString(), and a test
// binary carries no vcs stamp at all — every test in this package sees
// "0.4.0+dev" and would pin the abstention only. d.Lag is the seam
// (dispatch.go), the same shape Hints already is.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// lagWatchFixture is a dispatcher whose queue is a scratch dir — without
// the `beads:` write the loop reads the operator's LIVE fleet queue once a
// pass (ranger-base-uk0v/isq3) — and whose launcher reading is whatever the
// caller hands back.
func lagWatchFixture(t *testing.T, lag func() LauncherLag) *Dispatcher {
	t.Helper()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	scratch := t.TempDir()
	if err := os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+scratch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := b.App.BeadsDirs(); len(got) != 1 || got[0] != scratch {
		t.Fatalf("fixture is not hermetic: BeadsDirs() = %q, want [%q]", got, scratch)
	}
	d.Lag = lag
	return d
}

// runLagWatch runs the loop until it has completed wantPasses passes and
// returns everything it wrote.
func runLagWatch(t *testing.T, d *Dispatcher, wantPasses int) string {
	t.Helper()
	tap := newPassTap(wantPasses)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond); done <- p }()
	select {
	case <-done:
	case <-time.After(90 * time.Second):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}
	s := tap.String()
	if n := strings.Count(s, passHeader); n < wantPasses {
		t.Fatalf("only %d pass(es) ran, wanted %d:\n%s", n, wantPasses, s)
	}
	return s
}

// The pin the bead exists for: a loop running behind its own repo SAYS SO,
// in the log an operator reads back — and says it once, not once per pass.
//
// Both halves matter and they pull against each other. A line that never
// prints is the eight hours; a line that prints every pass is ~90 lines
// over those same eight hours, which is how a visible line becomes an
// invisible one (the rule the preamble already keeps for the hook wall and
// the stale-plan typo). Three behind, and the next say is due at six, so
// four passes must produce exactly one line.
//
// MUTATION: drop the drumbeat and print every pass → the count is 4, red.
// Move the reading back into the preamble → it prints once, but see
// TestQAWatchLagIsCountedPerPassNotAtLoopStart for why that is not this
// claim. Delete the pass block → the count is 0, red.
func TestQAWatchSaysTheLauncherIsBehindOnceNotEveryPass(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 3)
	d := lagWatchFixture(t, func() LauncherLag {
		return FindLauncher("0.4.0+"+stamp, []string{repo})
	})

	s := runLagWatch(t, d, 4)
	n := strings.Count(s, "launcher behind ·")
	if n != 1 {
		t.Fatalf("said the launcher is behind %d time(s) across 4 passes, want exactly 1:\n%s", n, s)
	}
	if !strings.Contains(s, "3 commit(s) behind main") {
		t.Fatalf("the line reached the log without its number:\n%s", s)
	}
	// And it is re-runnable by hand, or the operator cannot see WHICH fixes
	// the fleet is missing — the whole reason the incident took a session
	// to diagnose rather than a command.
	if !strings.Contains(s, "log --oneline "+stamp+"..main") {
		t.Fatalf("the line does not name the command that lists the gap:\n%s", s)
	}
}

// THE ARM THAT REJECTED THE OBVIOUS DESIGN. The binary's identity already
// prints once in the preamble (ReportPosseBinary), and the lag looks like it
// belongs beside it. It does not, and this is the measurement that says so:
// a launcher that was current when the loop started goes stale UNDER the
// loop, because main keeps moving and the running binary cannot. Measured on
// the box both times it happened — c592683 was committed 23:52:01 and built
// at 23:54 with a gap of 0, and 9920e75 was installed at 07:16:28 with a gap
// of 0 — so a start-of-loop reading speaks at the one moment the number is
// always zero.
//
// Here the repo gains its commits AFTER the loop is running: a preamble-only
// reading prints nothing at all, and the pass finds them.
//
// MUTATION: take the reading once and print it in the preamble instead of
// re-counting per pass → nothing is ever said, red.
func TestQAWatchLagIsCountedPerPassNotAtLoopStart(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)
	// Current at loop start, exactly as an install leaves it.
	lag := FindLauncher("0.4.0+"+stamp, []string{repo})
	if !lag.Known() || lag.Behind != 0 {
		t.Fatalf("fixture is not current at start: behind=%d (%s)", lag.Behind, lag.Why)
	}
	d := lagWatchFixture(t, func() LauncherLag { return lag })

	// main moves under the running loop, the way it did for eight hours.
	landCommits(t, repo, 2)

	s := runLagWatch(t, d, 3)
	if !strings.Contains(s, "launcher behind ·") {
		t.Fatalf("a loop that went stale under itself said nothing — this is the eight hours:\n%s", s)
	}
	if !strings.Contains(s, "2 commit(s) behind main") {
		t.Fatalf("the pass did not re-count after main moved:\n%s", s)
	}
}

// A pass reports CONDITIONS. A current launcher is not one, and a loop that
// said so every pass for ten hours would be the noise this reading has to
// stay out of to keep working.
//
// MUTATION: make BehindLine print at zero → red.
func TestQAWatchIsSilentWhenTheLauncherIsCurrent(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)
	d := lagWatchFixture(t, func() LauncherLag {
		return FindLauncher("0.4.0+"+stamp, []string{repo})
	})

	s := runLagWatch(t, d, 3)
	if strings.Contains(s, "launcher behind") {
		t.Fatalf("a current launcher printed into the pass:\n%s", s)
	}
	if strings.Contains(s, "not counted") {
		t.Fatalf("a reading that succeeded reported an abstention:\n%s", s)
	}
}

// A reading that cannot be taken is said, once, in the preamble — because
// silence here reads exactly like a fleet that is up to date, which is the
// error this whole file exists to stop. Once and not per pass on the same
// rule as everything else in that preamble.
//
// "+dev" is the real shape of it: a plain `go build` from a linked worktree
// carries no vcs stamp, so a launcher built that way names no commit at all.
//
// MUTATION: drop the preamble abstention → nothing is said, red. Say it
// every pass → the count is 3, red.
func TestQAWatchSaysAnUnreadableLauncherOnceInThePreamble(t *testing.T) {
	t.Parallel()
	// A perfectly good checkout is present on purpose: the abstention is
	// about the BINARY naming no commit, not about a missing repo, and a
	// fixture with no repo could not tell those two apart.
	repo, _ := posseRig(t, posseModule)
	d := lagWatchFixture(t, func() LauncherLag {
		return FindLauncher("0.4.0+dev", []string{repo})
	})

	s := runLagWatch(t, d, 3)
	if n := strings.Count(s, "not counted"); n != 1 {
		t.Fatalf("said the reading could not be taken %d time(s) across 3 passes, want exactly 1:\n%s", n, s)
	}
	if !strings.Contains(s, "names no commit") {
		t.Fatalf("the abstention does not say why:\n%s", s)
	}
}

// The seam defaults to the real thing. Without this arm every pin above
// would still pass with d.Lag wired to nothing at all, and the shipped loop
// would take no reading — the pins would be measuring the fixture.
//
// It asserts the abstention on purpose: a test binary carries no vcs stamp,
// so VersionString() here is "0.4.0+dev" whatever the tree holds, and that
// exact string coming back out of the loop is proof the loop asked THIS
// instance rather than a fixture. (In the fleet the same call is the real
// stamp; possebinary.go and cagestale.go have the measurement.)
//
// MUTATION: drop the `if readLag == nil` fallback → nothing is said, red.
func TestQAWatchDefaultsToThisInstancesOwnReading(t *testing.T) {
	t.Parallel()
	d := lagWatchFixture(t, nil)
	if d.Lag != nil {
		t.Fatal("fixture set a reading; this pin is about the loop taking its own")
	}

	s := runLagWatch(t, d, 2)
	if !strings.Contains(s, "not counted") || !strings.Contains(s, VersionString()) {
		t.Fatalf("the loop did not take this instance's own reading (expected %q in an abstention):\n%s", VersionString(), s)
	}
}
