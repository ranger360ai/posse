//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-vso72 — the reading that names how far the WALL
// RENDERER is behind, and the answers it must never collapse into "OK".
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
	if !strings.Contains(got, "is the tip of main") || !strings.Contains(got, "walls it renders, the launch-time checks it decides at every launch and the commit-time gates it judges at every commit are current") {
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

// ─── the launch-check half (ranger-base-atvd4) ───────────────────────────────
//
// A launch-time CHECK is install-gated for exactly the reason a wall is — the
// installed binary decides it — but it renders no file, so the wall class
// (keyed on a `do not edit` header a render writes about itself) cannot see
// it. MEASURED 2026-10-31 over ranger-base-qr7me's own exposure window,
// installed fe3b8036 against main at d4d105d2, 29 commits: 2 rendering a
// wall, 8 deciding a launch-time check — two of those eight being the grok
// project-config detector that was measurably guarding no seat. The operator
// read the 2 and nothing named the category the detector was in.

// allCatsRig is wallRig plus a launch-check source and a commit-gate source
// already in history, so a later commit to any of the three is a CHANGE
// rather than an addition. It seeds every category the reading has, so a row
// added later is one line here and no new rig (ranger-base-m1obc).
func allCatsRig(t *testing.T) (repo, stamp string) {
	t.Helper()
	repo, _ = posseRig(t, posseModule)
	commitIn(t, repo, WallRenderSources[0], "package posse // v1\n", "a wall render, before")
	commitIn(t, repo, LaunchCheckSources[0], "package posse // v1\n", "a launch check, before")
	commitIn(t, repo, CommitGateSources[0], "package posse // v1\n", "a commit-time gate judge, before")
	return repo, mustGit(t, repo, "rev-parse", "--short=7", "HEAD")
}

// The reading this bead is about: a lag whose commits decide a launch-time
// check and render NO wall. The wall count is honestly 0 and the line must
// still name what is stale — the old reading stopped at "none of them
// touching a path posse renders a wall from" and said nothing else.
func TestWallRendererNamesTheLaggingLaunchCheckCommits(t *testing.T) {
	t.Parallel()
	repo, stamp := allCatsRig(t)
	landCommits(t, repo, 3)
	commitIn(t, repo, LaunchCheckSources[0], "package posse // v2\n", "ranger-base-l12j9: grok declares its project-config channel")

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if !w.Lag.Known() || w.Lag.Behind != 4 {
		t.Fatalf("behind = %d known = %v (%s)", w.Lag.Behind, w.Lag.Known(), w.Lag.Why)
	}
	if len(w.Commits) != 0 || w.Why != "" {
		t.Fatalf("wall half = %v / %q; want an empty, read answer", w.Commits, w.Why)
	}
	if w.CheckWhy != "" {
		t.Fatalf("the launch-check half abstained: %s", w.CheckWhy)
	}
	if len(w.CheckCommits) != 1 || !strings.Contains(w.CheckCommits[0], "ranger-base-l12j9") {
		t.Fatalf("launch-check commits = %v, want the one l12j9 commit", w.CheckCommits)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	for _, want := range []string{
		"4 commit(s) behind main",
		// BOTH counts, always — a category said only when it is nonzero is a
		// category whose zero cannot be told from a reading that never ran.
		"0 of them rendering a wall",
		"1 of them deciding a launch-time check",
		// The consequence, in the words that distinguish it from a wall's:
		// the check is REPORTED, so the seat reads a clean verdict.
		"the landed fix guards nobody",
		"ranger-base-qr7me",
		"make install",
		"ranger-base-l12j9",
		"log --oneline " + stamp + "..main -- " + LaunchCheckSources[0],
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line missing %q:\n%s", want, got)
		}
	}
	// The wall half has nothing to list, so it prints no block and no
	// command: an operator who runs one for an empty answer stops trusting
	// the surface (the tip arm's own rule, applied per category).
	if strings.Contains(got, WallRenderSources[0]) {
		t.Fatalf("the empty wall half printed its pathspecs:\n%s", got)
	}
}

// One lag, THREE categories, counted separately and never collapsed: the
// whole shape ranger-base-vso72's bead asked for ("29 behind: 0 wall, 2
// launch-check"), with each category's own commits under its own
// consequence, and since ranger-base-m1obc with the commit-time gate beside
// them. The counts do NOT sum — one commit can be in two categories — and
// the headline is the whole lag either way.
//
// Both surfaces, because that is what ranger-base-m1obc's done-when 3 asks:
// `posse gates` and the promote epilogue print the same Lines(), so a
// category readable at one is readable at the other, and the `where` is the
// only thing that differs.
func TestWallRendererCountsTheThreeCategoriesSeparately(t *testing.T) {
	t.Parallel()
	repo, stamp := allCatsRig(t)
	commitIn(t, repo, WallRenderSources[0], "package posse // v2\n", "ranger-base-prjck: a fourth credential read-deny")
	commitIn(t, repo, LaunchCheckSources[0], "package posse // v2\n", "ranger-base-2zkcy: grok declares .grok/lsp.json")
	commitIn(t, repo, CommitGateSources[0], "package posse // v2\n", "ranger-base-ed07e: check 4 drops the branch's own bead")

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if len(w.Commits) != 1 || len(w.CheckCommits) != 1 || len(w.GateCommits) != 1 {
		t.Fatalf("wall = %v, check = %v, gate = %v; want one each", w.Commits, w.CheckCommits, w.GateCommits)
	}
	for _, where := range []string{"promote", "gates"} {
		got := strings.Join(w.Lines(where), "\n")
		for _, want := range []string{
			"3 commit(s) behind main",
			"1 of them rendering a wall, 1 of them deciding a launch-time check, 1 of them judging a commit-time gate",
			"renders the OLD seatbelt",
			"ranger-base-prjck",
			"OLD parity matrix and the OLD runtime declarations",
			"ranger-base-2zkcy",
			// The commit-time gate's own consequence, in the words that
			// distinguish it from the other two: the hook BODY is current
			// and the judge behind it is not, and the committer can type
			// no remedy.
			"judged by the OLD judge",
			"the hook BODY that pipes to it is a render",
			"no remedy to type",
			"ranger-base-v81vo",
			"ranger-base-ed07e",
			"make install",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("line at %q missing %q:\n%s", where, want, got)
			}
		}
		// Each commit sits under its own category's consequence, in list
		// order (wall, launch-check, commit-gate), so a reader cannot
		// attribute one to another — the mis-attribution ranger-base-m1obc
		// was filed for, where the one commit-gate commit that appeared at
		// all was printed under the WALL cost sentence.
		// `offsetOf` and not `at`: cmd/testparallel reads an identifier
		// named `at` as a reach into this package's written `at` var and
		// reports the test UNCLEARED (measured on the first draft of this
		// arm). A local closure is not that var; a distinct name is cheaper
		// than a parallelOK line that would waive a reason nobody cleared.
		offsetOf := func(sub string) int { return strings.Index(got, sub) }
		order := []int{
			offsetOf("renders the OLD seatbelt"), offsetOf("ranger-base-prjck"),
			offsetOf("OLD parity matrix"), offsetOf("ranger-base-2zkcy"),
			offsetOf("judged by the OLD judge"), offsetOf("ranger-base-ed07e"),
		}
		for i := 1; i < len(order); i++ {
			if order[i-1] >= order[i] {
				t.Fatalf("the three categories' blocks are interleaved at %q (offsets %v):\n%s", where, order, got)
			}
		}
	}
}

// No count is an all-clear, and the line says so naming EVERY list. Each is
// a floor — the wall list by the render-header class it derives from, the
// launch-check list by its two clauses, the commit-gate list by its
// render-and-dispatch intersection — so "no commit in any of them" may never
// render as "nothing to install". The "all N lists" clause counts the rows
// rather than spelling a number, so a fourth row cannot leave this sentence
// claiming three.
func TestWallRendererWithNoCategoryTouchedIsStillNotAnAllClear(t *testing.T) {
	t.Parallel()
	repo, stamp := allCatsRig(t)
	landCommits(t, repo, 6)

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if len(w.Commits) != 0 || len(w.CheckCommits) != 0 || len(w.GateCommits) != 0 ||
		w.Why != "" || w.CheckWhy != "" || w.GateWhy != "" {
		t.Fatalf("wall = %v/%q check = %v/%q gate = %v/%q; want three empty, read answers",
			w.Commits, w.Why, w.CheckCommits, w.CheckWhy, w.GateCommits, w.GateWhy)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	for _, want := range []string{
		"6 commit(s) behind main",
		"0 of them rendering a wall",
		"0 of them deciding a launch-time check",
		"0 of them judging a commit-time gate",
		"all 3 lists",
		"are a FLOOR",
		"WallRenderSources",
		"LaunchCheckSources",
		"CommitGateSources",
		"make install",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line missing %q:\n%s", want, got)
		}
	}
	if len(w.Lines("gates")) != 1 {
		t.Fatalf("a lag touching no list is one sentence, not a block:\n%s", got)
	}
}

// LaunchCheckSources must be paths in THIS repo, spelled slash-first and
// relative — they are handed to `git log -- <pathspec>` in whatever checkout
// the stamp resolved to, and an absolute or ./-prefixed spelling silently
// matches nothing there. The sibling arm for WallRenderSources, against the
// second list, because the failure is silent in exactly the same way.
func TestLaunchCheckSourcesAreRelativeRepoPaths(t *testing.T) {
	t.Parallel()
	if len(LaunchCheckSources) == 0 {
		t.Fatal("no launch check sources")
	}
	for _, p := range LaunchCheckSources {
		if filepath.IsAbs(p) || strings.HasPrefix(p, "./") || strings.Contains(p, "\\") {
			t.Errorf("%q is not a repo-relative slash path", p)
		}
		if !strings.HasSuffix(p, ".go") {
			t.Errorf("%q is not a Go source file", p)
		}
	}
}

// A subset that cannot be READ is UNKNOWN per category, never a zero. This
// is the arm the bead's own rule asks for ("UNKNOWN, never OK") applied one
// level down: a `git log` that fails over one pathspec list must not render
// as "0 of them", because 0 is the answer an operator acts on by doing
// nothing. Reached by handing the reading a Repo that is not a git checkout
// while the lag itself is counted — the shape a moved or pruned checkout
// takes between the count and the narrowing.
func TestWallRendererSaysUNKNOWNPerCategoryWhenASubsetCannotBeRead(t *testing.T) {
	t.Parallel()
	notARepo := t.TempDir()
	w := ReadWallRenderer(LauncherLag{
		Version: "0.5.0+abc1234", Rev: "abc1234", Base: "main", Repo: notARepo, Behind: 7,
	})
	if !w.Lag.Known() || w.Lag.Behind != 7 {
		t.Fatalf("the lag itself must still be counted: behind = %d, why = %q", w.Lag.Behind, w.Lag.Why)
	}
	if w.Why == "" || w.CheckWhy == "" || w.GateWhy == "" {
		t.Fatalf("a subset that could not be read reported no reason: why = %q, checkWhy = %q, gateWhy = %q", w.Why, w.CheckWhy, w.GateWhy)
	}
	if len(w.Commits) != 0 || len(w.CheckCommits) != 0 || len(w.GateCommits) != 0 {
		t.Fatalf("an unreadable subset invented commits: %v / %v / %v", w.Commits, w.CheckCommits, w.GateCommits)
	}
	// Each abstention names its OWN category, so a reader can tell which
	// question went unanswered rather than mistrusting all of them.
	if !strings.Contains(w.Why, "wall-rendering") || !strings.Contains(w.CheckWhy, "launch-check") || !strings.Contains(w.GateWhy, "commit-gate") {
		t.Fatalf("the three abstentions do not name their categories: %q / %q / %q", w.Why, w.CheckWhy, w.GateWhy)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	for _, want := range []string{
		"7 commit(s) behind main",
		"UNKNOWN how many of them render a wall",
		"UNKNOWN how many of them decide a launch-time check",
		"UNKNOWN how many of them judge a commit-time gate",
		"make install",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line missing %q:\n%s", want, got)
		}
	}
	// The one reading that would be a lie: an unread subset rendered as a
	// count of zero.
	for _, never := range []string{
		"0 of them rendering a wall",
		"0 of them deciding a launch-time check",
		"0 of them judging a commit-time gate",
		"are current",
	} {
		if strings.Contains(got, never) {
			t.Fatalf("an unreadable subset rendered as %q:\n%s", never, got)
		}
	}
}

// ─── the commit-time gate half (ranger-base-m1obc) ───────────────────────────
//
// A COMMIT-time gate is install-gated for exactly the reason a wall and a
// launch-time check are — the installed binary judges it — and for one more
// that is its own: the hook BODY is a render, already counted under
// WallRenderSources through gates.go, while the judge it pipes through is a
// subprocess of whatever `posse` is on PATH, resolved fresh at commit time on
// purpose (vulnPrivateChecks). So a current body can carry a stale verdict,
// and the file holding that verdict is in neither sibling list by both their
// derivations' own correct answers.
//
// MEASURED 2026-11-14 on this box over its own exposure window, installed
// 0.5.3+7d0f8c31 against origin/main at 1d03f9d1, 59 commits: 8 rendering a
// wall, 7 deciding a launch-time check, 4 judging a commit-time gate. ONE of
// the four appeared in the printed reading at all, under the WALL cost
// sentence because it also touched gates.go; the other three were inside the
// headline 59 and named by no category. ranger-base-ed07e's own-bead
// exemption is one of the four, so until it was installed an open
// vuln-private bead's own fix could carry no descriptive commit subject and
// its notes fragment could not land at all.

// The reading this bead is about: a lag whose commits judge a commit-time
// gate and render NO wall and decide NO launch-time check. Both other counts
// are honestly 0 and the line must still name what is stale — the old
// reading stopped at "none of them touching a path posse renders a wall from
// or decides a launch-time check in" and said nothing else over four commits
// that were refusing every vuln-private fix on the box.
func TestWallRendererNamesTheLaggingCommitGateCommits(t *testing.T) {
	t.Parallel()
	repo, stamp := allCatsRig(t)
	landCommits(t, repo, 3)
	commitIn(t, repo, CommitGateSources[0], "package posse // v2\n", "ranger-base-ed07e: check 4 drops the branch's own bead")

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if !w.Lag.Known() || w.Lag.Behind != 4 {
		t.Fatalf("behind = %d known = %v (%s)", w.Lag.Behind, w.Lag.Known(), w.Lag.Why)
	}
	if len(w.Commits) != 0 || w.Why != "" || len(w.CheckCommits) != 0 || w.CheckWhy != "" {
		t.Fatalf("wall = %v/%q check = %v/%q; want two empty, read answers", w.Commits, w.Why, w.CheckCommits, w.CheckWhy)
	}
	if w.GateWhy != "" {
		t.Fatalf("the commit-gate half abstained: %s", w.GateWhy)
	}
	if len(w.GateCommits) != 1 || !strings.Contains(w.GateCommits[0], "ranger-base-ed07e") {
		t.Fatalf("commit-gate commits = %v, want the one ed07e commit", w.GateCommits)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	for _, want := range []string{
		"4 commit(s) behind main",
		// EVERY count, always — a category said only when it is nonzero is
		// a category whose zero cannot be told from a reading that never ran.
		"0 of them rendering a wall",
		"0 of them deciding a launch-time check",
		"1 of them judging a commit-time gate",
		// The consequence, in the words that distinguish it from the other
		// two: the render is current, the judge is not, and the committer
		// has nothing to type.
		"judged by the OLD judge",
		"the hook BODY that pipes to it is a render",
		"no remedy to type",
		"ranger-base-v81vo",
		"make install",
		"ranger-base-ed07e",
		"log --oneline " + stamp + "..main -- " + CommitGateSources[0],
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line missing %q:\n%s", want, got)
		}
	}
	// The empty halves have nothing to list, so they print no block and no
	// command: an operator who runs one for an empty answer stops trusting
	// the surface (the tip arm's own rule, applied per category).
	for _, never := range []string{WallRenderSources[0], LaunchCheckSources[0]} {
		if strings.Contains(got, never) {
			t.Fatalf("an empty category printed its pathspecs (%s):\n%s", never, got)
		}
	}
	// And the one mis-attribution the bead was filed for: a commit-gate
	// commit printed under the WALL cost sentence.
	if strings.Contains(got, "renders the OLD seatbelt") {
		t.Fatalf("a lag with no wall commit printed the wall's cost sentence — the mis-attribution ranger-base-m1obc measured:\n%s", got)
	}
}

// THE COUNTS DO NOT SUM, and the headline is the whole lag either way
// (ranger-base-m1obc done-when 3, and LaunchCheckSources' "NOT DISJOINT"
// clause made a test). A file can be in two pathspec lists — bwrap.go
// renders a wall AND decides a launch-time check — so one lagging commit is
// honestly counted in two categories and the per-category counts then exceed
// the lag. That is not a defect to normalise: each category's question is
// "what is stale about this commit FOR THIS CATEGORY", each has its own
// consequence and its own remedy sentence, and a reader who adds them up has
// read the wrong number. The headline, which is `make install`'s whole
// subject, stays the lag.
//
// Derived, not hardcoded: the lists are not pinned disjoint and are not
// pinned to overlap either, so the arm finds the overlap and says so when
// there is none rather than asserting a membership some later bead may
// honestly change.
func TestWallRendererPerCategoryCountsDoNotSumToTheHeadline(t *testing.T) {
	t.Parallel()
	inTwo := ""
	for _, p := range WallRenderSources {
		for _, q := range LaunchCheckSources {
			if p == q {
				inTwo = p
			}
		}
	}
	if inTwo == "" {
		t.Skip("no path is in both WallRenderSources and LaunchCheckSources — the two lists are not pinned to overlap, so there is nothing to double-count here")
	}
	repo, stamp := allCatsRig(t)
	commitIn(t, repo, inTwo, "package posse // v2\n", "ranger-base-djl3c: the tier's second mechanism")

	w := ReadWallRenderer(FindLauncher("0.5.0+"+stamp, []string{repo}))
	if w.Lag.Behind != 1 {
		t.Fatalf("behind = %d, want the one commit", w.Lag.Behind)
	}
	if len(w.Commits) != 1 || len(w.CheckCommits) != 1 {
		t.Fatalf("%s is in both lists, so one commit to it is counted twice: wall = %v, check = %v", inTwo, w.Commits, w.CheckCommits)
	}
	got := strings.Join(w.Lines("gates"), "\n")
	// The lag leads and it is ONE, beside two counts of one each.
	for _, want := range []string{
		"1 commit(s) behind main",
		"1 of them rendering a wall, 1 of them deciding a launch-time check, 0 of them judging a commit-time gate",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line missing %q:\n%s", want, got)
		}
	}
	// And the commit is named under BOTH consequences, because both are
	// true of it. A reading that printed it once would be choosing a
	// category for the operator — the mis-attribution one category over.
	sha := strings.Fields(w.Commits[0])[0]
	if n := strings.Count(got, sha); n != 2 {
		t.Fatalf("the doubly-counted commit %s appears %d time(s), want once under each of its two categories:\n%s", sha, n, got)
	}
}

// CommitGateSources must be paths in THIS repo, spelled slash-first and
// relative — they are handed to `git log -- <pathspec>` in whatever checkout
// the stamp resolved to, and an absolute or ./-prefixed spelling silently
// matches nothing there. The third sibling of the same arm, because the
// failure is silent in exactly the same way against a third list.
func TestCommitGateSourcesAreRelativeRepoPaths(t *testing.T) {
	t.Parallel()
	if len(CommitGateSources) == 0 {
		t.Fatal("no commit gate sources")
	}
	for _, p := range CommitGateSources {
		if filepath.IsAbs(p) || strings.HasPrefix(p, "./") || strings.Contains(p, "\\") {
			t.Errorf("%q is not a repo-relative slash path", p)
		}
		if !strings.HasSuffix(p, ".go") {
			t.Errorf("%q is not a Go source file", p)
		}
	}
}
