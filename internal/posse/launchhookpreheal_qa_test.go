package posse

// QA pins for ranger-base-2mogn — part 1 of ranger-base-qxwd's "actual hole,
// in two parts" (part 2, the enumerator, is ranger-base-ixv4's SweepHookWall,
// pinned in hookwallsweep_qa_test.go).
//
// THE DEFECT. planLaunch (herdrback.go) called a.InstallCommitGuardHook(dir)
// immediately before a.CheckParityIn(...) on the next line. A launch into a
// repo whose installed hook had drifted from config re-stamped it and THEN
// probed the fresh render it had just written — so the parity check always
// saw this binary's own render, never the drift that was actually on disk a
// moment earlier. The operator never learned the wall was wrong, or for how
// long; SweepHookWall does not help here either, because it only ever sees
// the same post-heal state everyone else does.
//
// THE FIX. planLaunch now probes the commit-guard slot BEFORE the install
// call overwrites it, and — if that pre-heal probe disagreed with what
// config now says — warns naming the disagreement, before proceeding with
// the (still self-healing) launch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lhpFixture gives a backend a home with one declared repo, an agent it can
// launch, and returns the backend and the repo's path.
func lhpFixture(t *testing.T, visibility string) (*HerdrBackend, string) {
	t.Helper()
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"), []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := hwsRepo(t, t.TempDir(), "declared")
	if err := os.WriteFile(a.ConfigPath, []byte("beads_visibility:\n  "+repo+": "+visibility+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return b, repo
}

// A repo whose installed prepare-commit-msg stamp disagrees with what config
// NOW says (planted PUBLIC, config says private — the direction that fails
// toward disclosure, ranger-base-qxwd) is reported as a finding naming the
// disagreement BEFORE the launch's own install call silently repairs it.
func TestQALaunchReportsPreHealHookDriftBeforeSilentlyRepairingIt(t *testing.T) {
	b, repo := lhpFixture(t, VisibilityPrivate)
	a := b.App

	// Plant a whole, current, marker-bearing PUBLIC render — stale in
	// exactly the one line that decides the exemption, same fixture shape
	// TestHookWallSweepCatchesAStampThatDisagreesWithConfig uses.
	hook := hwsHook(t, repo, "prepare-commit-msg")
	if err := os.WriteFile(hook, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.BeadsVisibility(repo); v != VisibilityPrivate {
		t.Fatalf("fixture config did not take: visibility is %q", v)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("launch was refused rather than self-healed: %v", err)
	}
	got := warn.String()
	if !strings.Contains(got, repo) {
		t.Fatalf("no pre-heal finding names the repo:\n%s", got)
	}
	if !strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("no pre-heal finding names the drift:\n%s", got)
	}
	if !strings.Contains(got, "ours but stale") {
		t.Fatalf("finding does not carry the probe's own reason:\n%s", got)
	}

	// The launch still self-heals: the installed hook now carries the
	// CURRENT config's render (identity ∧ behavior, ADR 0023), so the
	// session that opens is protected — this bead asks for a loud finding,
	// not a refusal.
	if post := a.probeL3Hooks(repo, false); !post.CommitGuard {
		t.Fatalf("the launch did not re-stamp the hook to the current render: %s", post.CommitGuardDegraded)
	}
}

// The control: a repo whose wall already carries this binary's current
// render is not reported as drifted. Without this arm, a launch that always
// prints the pre-heal line would pass the positive pin above for the wrong
// reason.
func TestQALaunchIsQuietWhenTheWallAlreadyCarriesTheCurrentRender(t *testing.T) {
	b, repo := lhpFixture(t, VisibilityPrivate)
	a := b.App
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("launch into a fresh wall was refused: %v", err)
	}
	if got := warn.String(); strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("a fresh wall was reported as drifted:\n%s", got)
	}
}
