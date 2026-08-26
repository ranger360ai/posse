package rhq

// QA pins for per-session git worktrees (rangerhq-09o2), one per clause of
// the bead's DONE WHEN:
//
//   - two personas dispatched into the SAME repo get different trees, and
//     neither commit touches the other's index (the rangerhq-nyqj repro
//     sweeps nothing);
//   - the merge-back rangerhq-jbyr chose — option A, the launcher merges —
//     runs when a bead closes, so a closed bead's code is on the repo's
//     branch;
//   - a merge that cannot happen loses nothing and files a bead;
//   - a kill lands the work and refuses to remove a tree that still holds
//     any;
//   - the shared-index wall stands down in a tree where no index is shared,
//     and still stands in the checkout where one is (laurie's finding on
//     this bead).
//
// The unit half — placement, the beads redirect, merge mechanics — is
// worktree_test.go. These go through dispatch and through real git hooks.

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// wtqaRepo is a real git repo that is also a fake-bd beads repo: the two
// substrates a dispatched session sits between.
func wtqaRepo(t *testing.T, a *App, ready, show string) string {
	t.Helper()
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "fake-ready.json"), ready)
	if show != "" {
		write(t, filepath.Join(repo, "fake-show.json"), show)
	}
	write(t, a.ConfigPath, "beads:\n  - "+repo+"\n")
	return repo
}

// wtqaHome points $HOME at a temp dir so session worktrees land under it and
// never in the operator's real ~/.posse. It must run before newTestBackend's
// dirs are used for anything HOME-derived.
func wtqaHome(t *testing.T) { t.Helper(); t.Setenv("HOME", t.TempDir()) }

// ─── the first clause: two personas, one repo, two trees ─────────────────────

func TestDispatchGivesEachPersonaItsOwnTree(t *testing.T) {
	wtqaHome(t)
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	writePersona(t, b.App, "scout", "[py]")
	repo := wtqaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"b-1","title":"u","labels":["py"]}]`,
		`[{"id":"a-1","status":"closed"},{"id":"b-1","status":"closed"}]`)
	agentPerLaunch(t, fake)
	write(t, filepath.Join(fake, "agents.json"),
		`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","workspace_id":"w2"}]`)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}

	one := SessionForBead("ranger", repo, "a-1")
	two := SessionForBead("scout", repo, "b-1")
	mOne, ok := b.readMeta(one)
	if !ok {
		t.Fatalf("no meta for %s:\n%s", one, dispatcherOut(d))
	}
	mTwo, ok := b.readMeta(two)
	if !ok {
		t.Fatalf("no meta for %s:\n%s", two, dispatcherOut(d))
	}

	// Different trees, and neither of them the shared checkout.
	if mOne.Dir == mTwo.Dir {
		t.Fatalf("both personas were dispatched into one tree: %s", mOne.Dir)
	}
	for _, m := range []*HerdrMeta{mOne, mTwo} {
		if resolveExisting(m.Dir) == resolveExisting(repo) {
			t.Fatalf("%s runs in the shared checkout", m.Name)
		}
		if m.Repo == "" || m.Branch == "" {
			t.Fatalf("%s's run record does not name its repo/branch (%q/%q)", m.Name, m.Repo, m.Branch)
		}
		if got := mustGit(t, m.Dir, "symbolic-ref", "--short", "HEAD"); got != m.Branch {
			t.Errorf("%s HEAD = %q, want %q", m.Name, got, m.Branch)
		}
	}
	// herdr was told to open each workspace in its own tree, not in the repo.
	log := calls(t, fake)
	for _, m := range []*HerdrMeta{mOne, mTwo} {
		if !strings.Contains(log, "--cwd "+m.Dir) {
			t.Errorf("workspace for %s was not created in its own tree:\n%s", m.Name, log)
		}
	}

	// The rangerhq-nyqj repro, through the trees dispatch actually made:
	// ranger stages and does not commit; scout commits the unqualified way.
	write(t, filepath.Join(mOne.Dir, "rangers.txt"), "ranger's in-flight fix\n")
	mustGit(t, mOne.Dir, "add", "rangers.txt")
	write(t, filepath.Join(mTwo.Dir, "scouts.txt"), "scout's fix\n")
	mustGit(t, mTwo.Dir, "config", "user.email", "s@example.com")
	mustGit(t, mTwo.Dir, "config", "user.name", "s")
	mustGit(t, mTwo.Dir, "add", "scouts.txt")
	mustGit(t, mTwo.Dir, "commit", "-q", "-m", "scout's bead")

	if files := mustGit(t, mTwo.Dir, "show", "--name-only", "--format=", "HEAD"); strings.Contains(files, "rangers.txt") {
		t.Errorf("scout's commit swept ranger's staged work:\n%s", files)
	}
	if staged := mustGit(t, mOne.Dir, "diff", "--cached", "--name-only"); staged != "rangers.txt" {
		t.Errorf("ranger's index after scout's commit = %q, want it intact", staged)
	}

	// And the pass said where each session went, so the operator can find it.
	out := dispatcherOut(d)
	for _, m := range []*HerdrMeta{mOne, mTwo} {
		if !strings.Contains(out, "own tree "+AbbrevHome(m.Dir)) {
			t.Errorf("the pass did not name %s's tree:\n%s", m.Name, out)
		}
	}
}

// The beads redirect is what keeps two trees on ONE work graph. Measured
// live (see worktree.go): without it bd builds a second database in the
// worktree and the graph forks, silently.
func TestDispatchedTreesShareOneBeadsDatabase(t *testing.T) {
	wtqaHome(t)
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	write(t, filepath.Join(repo, ".beads", "issues.jsonl"), "")
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta(SessionForBead("ranger", repo, "a-1"))
	if !ok {
		t.Fatalf("no meta:\n%s", dispatcherOut(d))
	}
	got := readRedirect(t, m.Dir)
	if got != filepath.Join(repo, ".beads") {
		t.Errorf("the session tree's beads redirect = %q, want the repo's own database %q", got, filepath.Join(repo, ".beads"))
	}
}

// ─── the merge-back: option A (rangerhq-jbyr) ────────────────────────────────

// wtqaPassWithWork runs a pass over one closed bead, having pre-made the
// session's tree and put a commit on its branch — which is what the pass
// finds when a persona really did the work, since EnsureSessionTree is
// idempotent and the pass lands in the tree that is already there.
func wtqaPassWithWork(t *testing.T, extra func(repo, tree string)) (*Dispatcher, string, string) {
	t.Helper()
	wtqaHome(t)
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")
	if extra != nil {
		extra(repo, tr.Path)
	}
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	return d, repo, tr.Path
}

func TestClosedBeadLandsOnTheRepoBranch(t *testing.T) {
	d, repo, tree := wtqaPassWithWork(t, nil)
	out := dispatcherOut(d)

	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the persona's work\n" {
		t.Fatalf("a closed bead's code is not on the repo's branch: %v\n%s", err, out)
	}
	if !strings.Contains(out, "1 commit(s) fast-forwarded") || !strings.Contains(out, "onto main") {
		t.Errorf("the pass did not report the landing:\n%s", out)
	}
	// The tree stays: the session is still alive in it and the operator can
	// still read the turn. Retiring it is the kill's.
	if _, err := os.Stat(tree); err != nil {
		t.Errorf("the merge removed the session's tree out from under a live session: %v", err)
	}
}

func TestClosedBeadWithNoCommitSaysSo(t *testing.T) {
	wtqaHome(t)
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "closed with no commit on posse/") {
		t.Errorf("a bead closed with nothing committed must be said out loud:\n%s", out)
	}
}

func TestUncommittedWorkIsNamedAndNotLost(t *testing.T) {
	d, _, tree := wtqaPassWithWork(t, func(_, tree string) {
		write(t, filepath.Join(tree, "forgotten.txt"), "never committed\n")
	})
	out := dispatcherOut(d)
	if !strings.Contains(out, "uncommitted path(s) left in") || !strings.Contains(out, "forgotten.txt") {
		t.Errorf("uncommitted work must be named on the pass:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tree, "forgotten.txt")); err != nil {
		t.Errorf("uncommitted work was destroyed: %v", err)
	}
}

// A merge that cannot happen files a bead, because a closed bead whose code
// is on a branch nobody is told about is the failure mode this whole path
// exists to avoid (ADR 0006 §1: a handoff is a bead).
func TestMergeBlockedKeepsTheWorkAndFilesABead(t *testing.T) {
	d, repo, _ := wtqaPassWithWork(t, func(repo, _ string) {
		commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	})
	out := dispatcherOut(d)

	if !strings.Contains(out, "did NOT reach main") || !strings.Contains(out, "conflict") {
		t.Errorf("a blocked merge must say so:\n%s", out)
	}
	if !strings.Contains(out, "↳ filed q-") {
		t.Errorf("a blocked merge must file a bead:\n%s", out)
	}
	if body, _ := os.ReadFile(filepath.Join(repo, "fix.txt")); string(body) != "the operator's line\n" {
		t.Errorf("a failed merge modified the main checkout: %q", body)
	}
	// The bead it filed is the persona's, carries the label that routes it
	// back to a code lane, and hangs off the bead whose close exposed it.
	bd, _ := os.ReadFile(filepath.Join(os.Getenv("RHQ_FAKE_DIR"), "bd-calls.log"))
	// The argv is logged space-joined and the description is multi-line, so
	// the call spans lines in the log; read from the create onwards.
	i := strings.Index(string(bd), "create merge-back blocked")
	if i < 0 {
		t.Fatalf("no create call for the merge-back bead:\n%s", bd)
	}
	call := string(bd)[i:]
	for _, want := range []string{"-a ranger", "-l code", "--deps discovered-from:a-1", "worktree: "} {
		if !strings.Contains(call, want) {
			t.Errorf("the filed bead is missing %q:\n%s", want, call)
		}
	}
}

// ─── the kill: land it, or keep it and say so ────────────────────────────────

func TestKillLandsTheWorkAndRetiresTheTree(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")

	if err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: repo, Cmd: "true", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta("s1")
	if !ok || m.Branch == "" {
		t.Fatalf("the session got no tree: %+v", m)
	}
	commitIn(t, m.Dir, "fix.txt", "the work\n", "s1: the fix")

	landing, err := b.KillSessionAndLand("s1")
	if err != nil {
		t.Fatal(err)
	}
	if landing.Kept != "" {
		t.Fatalf("a mergeable, clean tree was kept: %s", landing.Kept)
	}
	if !strings.Contains(landing.Line(), "1 commit(s) fast-forwarded onto main") {
		t.Errorf("kill line = %q", landing.Line())
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err != nil {
		t.Errorf("the session's work did not land: %v", err)
	}
	if _, err := os.Stat(m.Dir); err == nil {
		t.Error("the worktree survived a clean landing")
	}
	if branchExists(repo, m.Branch) {
		t.Error("the session branch survived a clean landing")
	}
}

func TestKillKeepsATreeItCannotLand(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")

	if err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: repo, Cmd: "true", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, _ := b.readMeta("s1")
	commitIn(t, m.Dir, "clash.txt", "the session's line\n", "s1: mine")
	commitIn(t, repo, "clash.txt", "the operator's line\n", "main: theirs")

	landing, err := b.KillSessionAndLand("s1")
	if err != nil {
		t.Fatal(err)
	}
	if landing.Kept == "" {
		t.Fatal("a tree holding unmerged work was retired")
	}
	if !strings.Contains(landing.Line(), "KEPT") {
		t.Errorf("the kill line must say the tree was kept: %q", landing.Line())
	}
	if _, err := os.Stat(filepath.Join(m.Dir, "clash.txt")); err != nil {
		t.Errorf("the kept tree lost its work: %v", err)
	}
	if !branchExists(repo, m.Branch) {
		t.Error("the branch holding the unmerged work was deleted")
	}
	// And it is findable afterwards, with the session's meta gone.
	var out strings.Builder
	if err := ListSessionTrees(&out, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), m.Branch) || !strings.Contains(out.String(), "not on main") {
		t.Errorf("an orphaned tree must still be findable:\n%s", out.String())
	}
}

// A session that shares the checkout — every crew session, and every session
// from before this landed — must kill exactly as it always did.
func TestKillOfASharedCheckoutSessionIsUnchanged(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	if err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: repo, Cmd: "true"}); err != nil {
		t.Fatal(err)
	}
	landing, err := b.KillSessionAndLand("s1")
	if err != nil {
		t.Fatal(err)
	}
	if landing.Tree != nil || landing.Line() != "" {
		t.Errorf("a shared-checkout session reported a landing: %+v", landing)
	}
	if list := mustGit(t, repo, "worktree", "list"); strings.Count(list, "\n") != 0 {
		t.Errorf("a shared-checkout session made a worktree:\n%s", list)
	}
}

// ─── the wall: right in the checkout, quiet in the tree ──────────────────────

// laurie measured (on this bead) that the prepare-commit-msg wall installs
// into the COMMON git dir, so a linked worktree inherits it and is refused
// there too — under a message that says the index is "shared by every
// persona", which is exactly what a session worktree's index is not. The arm
// now asks whether the index really is shared.
func TestCommitGuardStandsDownInALinkedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	a := wtApp(t)
	repo := wtRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	commit := func(dir string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"PATH="+PathOutsideGates(""), "RHQ_PERSONA=developer", "RHQ_GATES_DIR=",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// In the session's own tree: nothing is shared, so the unqualified form
	// is safe and must go through.
	write(t, filepath.Join(tr.Path, "mine.txt"), "the session's work\n")
	mustGit(t, tr.Path, "add", "mine.txt")
	if out, err := commit(tr.Path, "commit", "-m", "my own tree"); err != nil {
		t.Fatalf("a commit in a worktree with a private index must pass:\n%v %s", err, out)
	}

	// In the shared checkout the same form is still refused — the wall did
	// not move, it learned which tree it is standing in.
	write(t, filepath.Join(repo, "theirs.txt"), "someone else's\n")
	mustGit(t, repo, "add", "theirs.txt")
	out, err := commit(repo, "commit", "-m", "sweep")
	if err == nil || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("the shared checkout must still be walled:\n%v %s", err, out)
	}
}

// The L3 parity probe asks its question in the checkout where the wall
// applies, so a launch into a session worktree is not degraded by a wall
// that is right to be quiet there.
func TestL3ProbeSeesTheWallFromInsideAWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	a := wtApp(t)
	repo := wtRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p := probeL3Hooks(tr.Path, false); !p.Repo || !p.CommitGuard {
		t.Errorf("probe from inside the session tree = %+v, want the commit guard counted", p)
	}
}

// ─── what the persona is told ────────────────────────────────────────────────

func TestWorkPromptNamesTheSessionTree(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t"}, Dir: repo}
	exe, _ := os.Executable()

	ctx := b.App.promptContext(Bd{Bin: exe}, is, "claude", "standard", "ranger-x-a-1", nil)
	if ctx.Tree == nil {
		t.Fatal("the prompt context did not resolve the session's tree")
	}
	p := workPrompt(is, ctx)
	for _, want := range []string{
		"your own worktree", ctx.Tree.Branch,
		"commit everything you want kept", "never merge to main yourself",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the work prompt does not say %q:\n%s", want, p)
		}
	}

	// A dir with no worktree says nothing about one, rather than promising a
	// tree the launch will not make.
	plain := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t"}, Dir: t.TempDir()}
	if strings.Contains(workPrompt(plain, b.App.promptContext(Bd{Bin: exe}, plain, "claude", "standard", "s", nil)), "your own worktree") {
		t.Error("a session with no tree was told it had one")
	}
}

// ─── the write boundaries have to know where the index went ──────────────────

// A session worktree's `.git` is a file and its index lives outside the
// tree, so a boundary drawn around the tree alone leaves a persona unable to
// commit in it. Both boundaries posse draws must name those dirs.
func TestWriteBoundariesReachTheWorktreesGitDirs(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	dirs := LinkedGitDirs(tr.Path)
	if len(dirs) != 2 {
		t.Fatalf("LinkedGitDirs(worktree) = %v, want the per-worktree and common git dirs", dirs)
	}
	for _, d := range dirs {
		if underDir(tr.Path, d) {
			t.Errorf("%s is inside the tree — then it would need no grant of its own", d)
		}
	}
	if got := LinkedGitDirs(repo); got != nil {
		t.Errorf("LinkedGitDirs(main checkout) = %v, want none — .git is already inside it", got)
	}

	// L2: the seatbelt profile's writable list.
	ag := &AgentFile{Name: "p", MemoryDir: t.TempDir()}
	w := strings.Join(SeatbeltWritable(ag, tr.Path, t.TempDir()), "\n")
	for _, d := range dirs {
		if !strings.Contains(w, absResolve(d)) {
			t.Errorf("seatbelt does not grant %s:\n%s", d, w)
		}
	}

	// The self-sandboxing runtimes' writable roots (codex --add-dir).
	r := realizeCodex(nil, nil, ag.MemoryDir, append([]string{""}, dirs...)...)
	for _, d := range dirs {
		if !strings.Contains(r.Deny, d) {
			t.Errorf("codex is not given %s as a writable root: %s", d, r.Deny)
		}
	}
}

// A kill while a launcher is firing must not freeze on the lock and must not
// merge unserialized: it kills, keeps the tree, and names the way to finish.
func TestKillDefersTheLandingWhileALauncherRuns(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	if err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: repo, Cmd: "true", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, _ := b.readMeta("s1")
	commitIn(t, m.Dir, "fix.txt", "the work\n", "s1: the fix")

	held, err := lockLaunches(b.App, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan *KillLanding, 1)
	go func() {
		l, err := b.KillSessionAndLand("s1")
		if err != nil {
			t.Error(err)
		}
		done <- l
	}()
	var landing *KillLanding
	select {
	case landing = <-done:
	case <-time.After(20 * time.Second):
		held.Release()
		t.Fatal("the kill waited on the launcher lock — the cockpit would have frozen")
	}
	held.Release()

	if !strings.Contains(landing.Kept, "launcher is running") || !strings.Contains(landing.Kept, "--land") {
		t.Errorf("the deferral must say why and how to finish it: %q", landing.Kept)
	}
	if _, err := os.Stat(filepath.Join(m.Dir, "fix.txt")); err != nil {
		t.Errorf("the deferred kill destroyed the tree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err == nil {
		t.Error("the deferred kill merged without the lock")
	}
	// And the catch-up finishes it.
	var out strings.Builder
	if err := LandSessionTrees(&out, b.App, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err != nil {
		t.Errorf("`posse worktrees --land` did not finish the deferred landing:\n%s", out.String())
	}
}
