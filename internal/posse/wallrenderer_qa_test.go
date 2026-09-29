//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-vso72 — the reading that names how far the WALL
// RENDERER is behind, and the two answers it must never collapse into "OK".
//
// The gap these hold shut, from ranger-base-mmhhc: `posse promote` recorded
// the renderer (m.Posse) beside the sha it promoted and never compared the
// two, so a binary 40 commits behind the sha it had that moment promoted —
// one of them a seatbelt.go change — printed nothing, and 12 caged launches
// then rendered the superseded wall while their own headers said "rendered
// from the PID at launch".

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// wallRig is posseRig with a wall-rendering file already in history, so a
// later commit to it is a CHANGE rather than an addition — the shape the
// pathspec has to catch either way, and the shape ranger-base-prjck was.
func wallRig(t *testing.T) (repo, stamp string) {
	t.Helper()
	repo, _ = posseRig(t, posseModule)
	commitIn(t, repo, WallRenderSources[0], "package posse // v1\n", "a wall render, before")
	return repo, mustGit(t, repo, "rev-parse", "--short=7", "HEAD")
}

// The reading the bead is about: a lagging commit that touches a wall render
// is NAMED, and the line says what it costs and what closes it.
func TestWallRendererNamesTheLaggingWallCommits(t *testing.T) {
	t.Parallel()
	repo, stamp := wallRig(t)
	landCommits(t, repo, 3)
	commitIn(t, repo, WallRenderSources[0], "package posse // v2\n", "ranger-base-prjck: a fourth credential read-deny")

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if !w.Lag.Known() {
		t.Fatalf("not counted: %s", w.Lag.Why)
	}
	if w.Lag.Behind != 4 {
		t.Fatalf("behind = %d, want 4", w.Lag.Behind)
	}
	if w.Why != "" {
		t.Fatalf("the wall half abstained: %s", w.Why)
	}
	if len(w.Commits) != 1 || !strings.Contains(w.Commits[0], "ranger-base-prjck") {
		t.Fatalf("wall commits = %v, want the one prjck commit", w.Commits)
	}
	got := strings.Join(w.Lines("promote"), "\n")
	// The whole lag, the wall subset, the consequence and the remedy. Each
	// of the four is a thing the silent surface did not say.
	for _, want := range []string{
		"4 commit(s) behind main",
		"1 of them rendering a wall",
		"renders the OLD seatbelt",
		"make install",
		"ranger-base-prjck",
		"log --oneline " + stamp + "..main -- " + WallRenderSources[0],
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line missing %q:\n%s", want, got)
		}
	}
}

// A lag whose commits touch no wall render is still a lag. This is the arm
// that keeps the wall count from becoming an all-clear: the pathspec list is
// a floor (WallRenderSources' own comment), so "no wall commit among the
// paths posse watches" may never render as "nothing to install".
func TestWallRendererWithNoWallCommitIsStillNotAnAllClear(t *testing.T) {
	t.Parallel()
	repo, stamp := wallRig(t)
	landCommits(t, repo, 6)

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if len(w.Commits) != 0 || w.Why != "" {
		t.Fatalf("commits = %v, why = %q; want an empty, read answer", w.Commits, w.Why)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	for _, want := range []string{"6 commit(s) behind main", "none of them touching a path posse renders a wall from", "make install"} {
		if !strings.Contains(got, want) {
			t.Fatalf("line missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "rendering a wall —") {
		t.Fatalf("a lag with no wall commit claimed one: %s", got)
	}
}

// Current is said plainly, and says what is current: the WALLS, not the
// binary. `posse gates` prints this above a parity matrix whose every row is
// a claim about a render this binary just made.
func TestWallRendererAtTheTipSaysTheWallsAreCurrent(t *testing.T) {
	t.Parallel()
	repo, stamp := wallRig(t)

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if w.Lag.Behind != 0 || !w.Lag.Known() {
		t.Fatalf("behind = %d known = %v (%s)", w.Lag.Behind, w.Lag.Known(), w.Lag.Why)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	if !strings.Contains(got, "is the tip of main") || !strings.Contains(got, "walls it renders at every launch are current") {
		t.Fatalf("line = %s", got)
	}
	// No git log was needed, so none may be printed: a command that lists
	// nothing is a command an operator runs and then mistrusts the surface.
	if strings.Contains(got, "log --oneline") {
		t.Fatalf("a current renderer printed a range to inspect: %s", got)
	}
}

// The bead's own rule: a binary that carries no vcs.revision says UNKNOWN,
// never OK. A dev build is the ordinary way to reach this (cagestale.go: a
// `go build` from a linked worktree carries neither a pseudo-version nor
// vcs.*), and every persona on this box works in a linked worktree.
func TestWallRendererWithNoCommitSaysUnknownAndNotOK(t *testing.T) {
	t.Parallel()
	repo, _ := wallRig(t)

	w := ReadWallRenderer(FindLauncher("0.5.0+dev", []string{repo}))
	got := strings.Join(w.Lines("promote"), "\n")
	if !strings.Contains(got, "UNKNOWN") {
		t.Fatalf("a binary naming no commit did not say UNKNOWN: %s", got)
	}
	if strings.Contains(got, "is the tip of") || strings.Contains(got, "are current") {
		t.Fatalf("an unreadable answer rendered as current: %s", got)
	}
	// Silence is the failure mode this replaces, so there is always a line.
	if len(w.Lines("promote")) == 0 {
		t.Fatal("no line at all")
	}
}

// A dirty stamp names the last COMMIT of a tree that also had uncommitted
// edits, so the wall it renders is in no commit and the count is a floor.
// The version suffix does not say that; the clause does.
func TestWallRendererSaysADirtyRenderIsInNoCommit(t *testing.T) {
	t.Parallel()
	repo, stamp := wallRig(t)
	landCommits(t, repo, 2)

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp+"-dirty", []string{repo}))
	if !w.Lag.Dirty {
		t.Fatal("the -dirty suffix was not read")
	}
	if got := strings.Join(w.Lines("promote"), "\n"); !strings.Contains(got, "in no commit at all") {
		t.Fatalf("no floor clause: %s", got)
	}
}

// ReportWallRenderer prints in every case and reports a lag without
// refusing one — `posse promote` has already promoted by the time this runs,
// and installing over a binary that is dispatching a live fleet is the
// operator's (guardrail 3).
func TestReportWallRendererAlwaysPrintsAndNeverRefuses(t *testing.T) {
	t.Parallel()
	repo, _ := wallRig(t)
	landCommits(t, repo, 1)
	commitIn(t, repo, WallRenderSources[0], "package posse // v3\n", "a gate fix")

	a := &App{}
	var buf bytes.Buffer
	// The stamp is this process's, not the rig's, so the candidate list is
	// exercised and the answer is whatever this binary honestly is; what is
	// pinned is that a line came out and nothing failed.
	behind := a.ReportWallRenderer(&buf, "promote", repo)
	if buf.Len() == 0 {
		t.Fatal("printed nothing")
	}
	if !strings.HasPrefix(buf.String(), "wall renderer (promote) · ") {
		t.Fatalf("one writer, one prefix: %q", buf.String())
	}
	_ = behind
}

// WallRenderSources must be paths in THIS repo, spelled slash-first and
// relative — they are handed to `git log -- <pathspec>` in whatever checkout
// the stamp resolved to, and an absolute or ./-prefixed spelling silently
// matches nothing there.
func TestWallRenderSourcesAreRelativeRepoPaths(t *testing.T) {
	t.Parallel()
	if len(WallRenderSources) == 0 {
		t.Fatal("no wall render sources")
	}
	for _, p := range WallRenderSources {
		if filepath.IsAbs(p) || strings.HasPrefix(p, "./") || strings.Contains(p, "\\") {
			t.Errorf("%q is not a repo-relative slash path", p)
		}
		if !strings.HasSuffix(p, ".go") {
			t.Errorf("%q is not a Go source file", p)
		}
	}
}

// ─── the wiring: a control nothing calls is the state this bead is about ─────

// `posse promote`'s ratification read. The bead's whole finding 2 is that
// this command RECORDED the renderer (m.Posse) and never read it — so a pin
// on the reading that does not check it FIRES here would leave the gap
// exactly where it was found.
//
// Both halves: the dry run (the ratification read, the moment an operator can
// still act) and the real promote. The fixture's constitution is not a posse
// checkout, so the verdict here is whatever this test binary honestly is —
// what is pinned is that a line comes out, in both, on the one prefix.
func TestPromoteReadsTheRendererItRecords(t *testing.T) {
	t.Parallel()
	a, src, _ := promoteFixture(t)

	const prefix = "wall renderer (promote) · "
	for _, dry := range []bool{true, false} {
		var b bytes.Buffer
		if err := a.CmdPromote(&b, PromoteOpts{Source: src, DryRun: dry}); err != nil {
			t.Fatalf("promote (dry=%v): %v\n%s", dry, err, b.String())
		}
		if !strings.Contains(b.String(), prefix) {
			t.Fatalf("promote (dry=%v) does not read the renderer it records — the control fires nowhere:\n%s", dry, b.String())
		}
		// Before the act, not after it: on a dry run there is no act, and on
		// a real one the operator reads the diff and this together.
		if at, act := strings.Index(b.String(), prefix), strings.Index(b.String(), "files from "); dry && act >= 0 && at > act {
			t.Fatalf("the reading came after the dry run's verdict:\n%s", b.String())
		}
	}
}
