//go:build posse_arm2

package posse

// LIVE pin for ranger-base-5fond — ADR 0088 D3 run against the box's REAL
// cage, over this repo's own tree and this repo's own
// `scripts/posse-landed-check`.
//
//	RHQ_LIVE_LANDEDCHECK=1 go test -tags posse_arm2 ./internal/posse \
//	  -run TestLiveLandedCheck -v -timeout 15m
//
// PAST landedCheckLimit ON PURPOSE. Since D6.1 that bound is ten minutes,
// which is also go test's default, so a judge that really hung would be
// killed by the TEST's clock first and the run would read as a panicking
// suite rather than as the deadline this pin now names (D6.4).
//
// WHY IT IS SEPARATE FROM landedcheck_test.go. That file stubs the cage,
// deliberately: a suite that rendered a real wall would be pinning bwrap's
// mount semantics rather than this runner, and the arm most worth having
// there — a cage that REFUSES — is unreachable on a box where the cage
// works. What the stub cannot answer is the only question left: are the
// write grants D3 names enough for this repo's check to run at all? The
// check forks `make`, which forks `go test` and `go vet`, and those want a
// build cache, a temp directory and a git index. A grant list that is one
// entry short does not render wrong — it renders, and the check reds for a
// reason that has nothing to do with the tree.
//
// SO THIS IS A MEASUREMENT AND NOT A GATE. It is off by default, it is not a
// prerequisite of anything, and `make test` does not run it. Every reading
// it has produced is in docs/notes.d/ranger-base-5fond.md, which is the one
// place they are said.
//
// IT WRITES TO THE SHARED STORE, which is why the env var is not a
// formality: `git worktree add` registers under the repo's COMMON
// `.git/worktrees/`, so a run of this from a seat is a write every other
// worktree of this repo sees. It is removed on the way out, by the same
// deferred `worktree remove` production uses.

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveLandedCheck(t *testing.T) {
	if os.Getenv("RHQ_LIVE_LANDEDCHECK") == "" {
		t.Skip("set RHQ_LIVE_LANDEDCHECK=1 to judge this tree behind the box's real cage")
	}
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// The package directory is not the repo root, and the runner is a
	// repo-level actor: ask git rather than climbing by hand.
	top, err := git(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	branch := repoBranch(top)
	if branch == "" {
		t.Skipf("%s has a detached HEAD, so there is no branch for a landing to have moved", AbbrevHome(top))
	}
	// A SUITE IS NOT A LAUNCHER, and this is the one place that matters.
	// The child inherits this process's environment (D3 changes TMPDIR and
	// nothing else, which is ADR 0087's land check's rule too), and this
	// process is a posse test binary — whose TestMain turns ANY posse test
	// binary carrying RHQ_FAKE_HERDR into a fake bd. This repo's check runs
	// `make`, whose doors run `go test ./internal/posse`, so the marker
	// reaches the inner test binary and it answers `fake bd: unhandled
	// -test.run=…` and exits 1. MEASURED 2026-11-11: `crew-check` red in
	// 0.005s under the cage and green in 4.0s outside it, over one tree.
	// That is this suite's marker and nothing about the runner — a launcher
	// carries none of these — so it is shed for the run.
	//
	// Process-wide, so this test is run alone (it is env-gated, which is
	// what makes that true rather than hoped).
	for _, k := range []string{"RHQ_FAKE_HERDR", "RHQ_FAKE_GATED_BD_STORE", "RHQ_FAKE_POSSE"} {
		t.Setenv(k, "")
	}
	a := wtApp(t) // its own StateDir: the run directory is this test's, not the box's
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)

	// The record is the REAL repo's, so it is read and put back: a live run
	// that left `posse.landedCheck` pointing at this tree would hand the
	// next landing a cached verdict this test measured rather than the
	// launcher did.
	had, hadV := landedCheckRecorded(top)
	t.Cleanup(func() {
		if had == "" {
			_, _ = git(top, "config", "--unset", landedCheckKey)
			return
		}
		_, _ = git(top, "config", landedCheckKey, had+" "+hadV.String())
	})

	start := time.Now()
	c.Schedule(top, branch)
	c.Wait()
	took := time.Since(start)

	s := out.String()
	t.Logf("MEASURED %s: %s", took.Round(time.Second), strings.TrimSpace(s))
	if strings.Contains(s, "NOT judged") {
		// D6.4: A FAILURE MUST NAME ITS OWN CAUSE. The verdict already
		// distinguishes the two silences, so this must too. A deadline is a
		// PRICE — the judge ran and was killed holding its answer — and
		// reading it as a wrong write set is what sent the 2026-11-12
		// crossing to the wrong lane (ranger-base-7idk4). The phrase is
		// landChildVerdict's own, which is where the sentence is built.
		if strings.Contains(s, "no answer in ") {
			t.Fatalf("the judge hit landedCheckLimit (%s) over %s after %s; this is a price, see ADR 0088 D6 — the tree is not red, the judge was killed holding the answer:\n%s",
				landedCheckLimit, AbbrevHome(top), took.Round(time.Second), s)
		}
		t.Fatalf("the box's real cage could not judge %s in %s — this is D3's write set being wrong, not the tree being red:\n%s", AbbrevHome(top), took.Round(time.Second), s)
	}
	if s == "" {
		t.Fatalf("nothing was said about %s: either it ships no %s or its tree is already on record", AbbrevHome(top), landedCheckScript)
	}
	// Red is a legitimate answer about a tree and is NOT a failure of this
	// pin — what this measures is that the cage can carry the check at all.
	//
	// D6.2's ceiling is 240s and it is READ, never enforced: a run past it
	// still answers, and the reader that turns a crossing into an
	// architecture bead is ranger-base-lhiy1's, over the launcher's own runs.
	// So this is logged and not asserted, for two reasons. A suite running
	// beside two other seats' suites is not the box D6 priced — and the
	// assertion that used to sit here compared the wall to the KILL, which
	// is now ten minutes and bounds a hang rather than a price, so it would
	// have asserted nothing a reader of this log needs (ADR 0088 D6.1/D6.2).
	over := "under"
	if took > landedCheckCeiling {
		over = "OVER"
	}
	// Both numbers through landedCheckWall, so the log compares like with
	// like: the verdict line above says `114.1s` and Duration.String would
	// have said `4m0s` for the ceiling beside it.
	t.Logf("MEASURED %s, %s D6.2's %s ceiling — a series of quiet-box readings over it sends the price back to architecture (ADR 0088 D6.2)",
		landedCheckWall(took), over, landedCheckWall(landedCheckCeiling))
}
