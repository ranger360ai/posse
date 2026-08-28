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

// The same refusal through the pass (ranger-base-5s2o): the operator's
// checkout is the one store on this path the launcher lock does not govern,
// so a branch switch during a session must leave the work on its branch and
// file the bead that says where it is — never land it on whatever the
// operator is standing on.
func TestMergeBackRefusesWhenTheOperatorLeftTheBase(t *testing.T) {
	d, repo, tree := wtqaPassWithWork(t, func(repo, _ string) {
		mustGit(t, repo, "checkout", "-q", "-b", "operator-side")
	})
	out := dispatcherOut(d)

	if !strings.Contains(out, "did NOT reach main") || !strings.Contains(out, "operator-side") {
		t.Errorf("the pass must name the branch in the way:\n%s", out)
	}
	if !strings.Contains(out, "↳ filed q-") {
		t.Errorf("an unlanded merge must file a bead:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err == nil {
		t.Error("the persona's commit landed on the operator's branch")
	}
	if _, err := os.Stat(filepath.Join(tree, "fix.txt")); err != nil {
		t.Errorf("the work left the tree that still holds it: %v", err)
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
	if p := a.probeL3Hooks(tr.Path, false); !p.Repo || !p.CommitGuard {
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

	// L2: the seatbelt profile's writable list. The per-worktree dir is
	// granted whole; the COMMON one is shared with the operator's checkout
	// and every other session, so only the three paths a commit writes
	// there are named (ranger-base-m2wf, pinned in
	// seatbeltworktreegit_qa_test.go). A `strings.Contains` on the common
	// dir would pass on `<common>/objects` and measure nothing.
	ag := &AgentFile{Name: "p", MemoryDir: t.TempDir()}
	set := NewAppAt(t.TempDir()).SeatbeltWritable(ag, tr.Path, t.TempDir())
	w := strings.Join(set, "\n")
	for _, p := range []string{
		dirs[0],
		filepath.Join(dirs[1], "objects"),
		filepath.Join(dirs[1], "logs"),
		filepath.Join(dirs[1], "refs", "heads", tr.Branch),
	} {
		if !sbCovers(set, p) {
			t.Errorf("seatbelt does not grant %s:\n%s", p, w)
		}
	}
	// And the shared half of it is not named. Asked of the entries UNDER
	// the common dir rather than of the whole set, because this fixture
	// sits inside TMPDIR and the profile's blanket temp grant covers it —
	// `sbCovers` would answer yes here for a path no git grant names. The
	// coverage question is pinned on a fixture outside TMPDIR instead
	// (seatbeltworktreegit_qa_test.go).
	base := filepath.Join(dirs[1], "refs", "heads", tr.Base)
	for _, g := range set {
		if underDir(dirs[1], g) && underDir(g, base) {
			t.Errorf("%s grants the branch the launcher merges into (%s):\n%s", g, base, w)
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

// The merge target is the branch recorded when the session tree was made,
// not whichever branch the operator happens to have checked out when the
// persona closes its bead. A branch switch must refuse rather than land the
// persona's commit on the operator's unrelated branch and report the
// original base as merged.
func TestQAMergeBackDoesNotLandOnTheOperatorsCurrentBranch(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-branch-switch", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "session work")
	baseBefore := mustGit(t, repo, "rev-parse", tr.Base)

	mustGit(t, repo, "checkout", "-q", "-b", "operator-side")
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatalf("merge-back reported %s merged while the operator had operator-side checked out: %+v", tr.Base, o)
	}
	if got := mustGit(t, repo, "rev-parse", tr.Base); got != baseBefore {
		t.Fatalf("recorded base %s moved across a branch-switch refusal: %s -> %s", tr.Base, baseBefore, got)
	}
	if got := mustGit(t, repo, "rev-parse", "operator-side"); got != baseBefore {
		t.Fatalf("persona work landed on operator-side: %s -> %s", baseBefore, got)
	}
}

// Relaunch starts from the recorded worktree path, not the main checkout.
// A detached operator checkout removes the merge target temporarily, but it
// must not erase the repo/branch provenance from the recreated run record:
// that would make later close/kill paths treat a private tree as shared and
// silently skip its landing.
func TestQARelaunchKeepsWorktreeProvenanceWhileOperatorHeadIsDetached(t *testing.T) {
	wtqaHome(t)
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	if err := b.CreateSession(NewSessionOpts{Name: "s-detached", Dir: repo, Cmd: "true", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta("s-detached")
	if !ok || m.Repo == "" || m.Branch == "" {
		t.Fatalf("initial session has no worktree provenance: %+v", m)
	}
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")

	p, err := b.planLaunch(RecreateOpts(m))
	if err != nil {
		t.Fatal(err)
	}
	// Repo is compared with symlinks resolved, and only Repo: git answers
	// `--git-common-dir` relatively in the main checkout and absolutely —
	// already resolved — from inside a linked worktree, so the create and
	// the relaunch legitimately spell one repo two ways under a symlinked
	// path like macOS's /var (MainCheckout says so). What must not change
	// is WHICH repo, and that it is named at all.
	if p.Dir != m.Dir || p.Branch != m.Branch || resolveExisting(p.Repo) != resolveExisting(m.Repo) || p.Repo == "" {
		t.Fatalf("relaunch demoted a private tree while operator HEAD was detached: plan dir/repo/branch = %q/%q/%q, want %q/%q/%q", p.Dir, p.Repo, p.Branch, m.Dir, m.Repo, m.Branch)
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

// The pin above stops at the launch plan. The harm ranger-base-q5p1 named
// was two steps further down: the recreated RUN RECORD carried the blanked
// repo/branch, SessionTreeOf answered nil for it, and the kill then read a
// live private tree as a shared checkout and skipped its landing without a
// word. This drives the whole chain — relaunch under a detached checkout,
// kill under it, catch-up after it — through the real paths.
func TestQADetachedRelaunchStillLandsTheSessionsWork(t *testing.T) {
	wtqaHome(t)
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	if err := b.CreateSession(NewSessionOpts{Name: "s-detached", Dir: repo, Cmd: "true", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	before, ok := b.readMeta("s-detached")
	if !ok || before.Branch == "" {
		t.Fatalf("the session got no tree: %+v", before)
	}
	commitIn(t, before.Dir, "fix.txt", "the persona's work\n", "s-detached: the fix")
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s-detached", NoLand: true, Force: true}); err != nil {
		t.Fatalf("relaunch under a detached checkout: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "SHARED checkout") {
		t.Errorf("the relaunch reported a live private tree as shared:\n%s", out.String())
	}

	// The record the kill will read, not just the plan the launch used.
	after, ok := b.readMeta("s-detached")
	if !ok {
		t.Fatal("the recreated session has no meta")
	}
	if after.Dir != before.Dir || after.Branch != before.Branch || after.Repo == "" {
		t.Fatalf("the recreated run record lost its worktree provenance: dir/repo/branch = %q/%q/%q, want %q/<non-empty>/%q",
			after.Dir, after.Repo, after.Branch, before.Dir, before.Branch)
	}
	if SessionTreeOf(after) == nil {
		t.Fatal("SessionTreeOf(recreated meta) = nil — every later close and kill would skip the landing")
	}

	// Killed while the operator is STILL detached: the landing is deferred
	// out loud, and nothing is destroyed. Silence here was the bug.
	landing, err := b.KillSessionAndLand("s-detached")
	if err != nil {
		t.Fatal(err)
	}
	if landing.Tree == nil || landing.Line() == "" {
		t.Fatalf("the kill saw no tree to land: %+v", landing)
	}
	if landing.Kept == "" {
		t.Fatalf("a tree whose base is unreachable was retired: %+v", landing)
	}
	if _, err := os.Stat(filepath.Join(before.Dir, "fix.txt")); err != nil {
		t.Fatalf("the deferred kill destroyed the work: %v", err)
	}
	if !branchExists(repo, before.Branch) {
		t.Fatal("the branch holding the unmerged work was deleted")
	}

	// And the operator coming back off the bisect finishes it.
	mustGit(t, repo, "checkout", "-q", "main")
	var land strings.Builder
	if err := LandSessionTrees(&land, b.App, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err != nil {
		t.Fatalf("the work never reached main after the detach ended:\n%s", land.String())
	}
}

// The other side of that tree surviving a detach: what the persona is TOLD
// about it. A session branch with no recorded base — the legacy shape baseOf
// documents and falls back for — answers Base == "" under a detached
// checkout, and the work prompt interpolates it raw. The launch warning
// already says "the branch it was cut from" there (orDetached); the prompt
// says nothing at all, twice. ranger-base-nfgh.
func TestQADetachedLegacyBranchPromptNamesTheBase(t *testing.T) {
	t.Skip("ranger-base-nfgh: the work prompt renders an empty base — unskip with the fix")
	wtqaHome(t)
	b, _ := newTestBackend(t)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A branch cut before posseBase was recorded: nothing can recover its
	// true base, and baseOf answers "" once HEAD has no branch to fall back
	// on either.
	mustGit(t, repo, "config", "--unset", baseKey(tr.Branch))
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t"}, Dir: repo}
	exe, _ := os.Executable()
	p := workPrompt(is, b.App.promptContext(Bd{Bin: exe}, is, "claude", "standard", session, nil))
	i := strings.Index(p, "your own worktree")
	if i < 0 {
		t.Fatal("the session was not told about the tree it is working in")
	}
	for _, empty := range []string{"fast-forwards " + tr.Branch + " onto\n   in ", "never merge to  yourself"} {
		if strings.Contains(p, empty) {
			t.Errorf("the work prompt names no branch where a base belongs (%q):\n%s", empty, p[i:])
		}
	}
}

// ─── placement, adversarially: a root that only LOOKS like it is under $HOME ──

// The under-$HOME rule exists because a session scratchpad is reaped and a
// reaped worktree destroys the only copy of a persona's work. A textual
// prefix test would pass a symlink that sits under $HOME and lands in the
// reaper's path — `~/scratch -> /private/tmp/x` reads as "under $HOME" and is
// not. WorktreeRoot must refuse it.
//
// Pinned on rangerhq-qnjo, where the surrounding claim ("bd holds no net
// under us") turned out to be right for the wrong reason: bd DOES refuse a
// BEADS_DIR that resolves under /tmp — because it canonicalises through
// EvalSymlinks BEFORE validating. This is the same resolution step, and it is
// the only thing standing between a persona's work and a reaper.
func TestQAWorktreeRootRefusesASymlinkOutOfHome(t *testing.T) {
	a := wtApp(t)
	home := os.Getenv("HOME")

	// The control: a real directory under $HOME is accepted, so a refusal
	// below means "outside", not "unwritable" or "missing".
	inside := filepath.Join(home, "trees")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, a.ConfigPath, "worktrees: "+inside+"\n")
	if got, err := a.WorktreeRoot(); err != nil || got != inside {
		t.Fatalf("a real dir under $HOME must be accepted: got %q, %v", got, err)
	}

	// The attack: same shape, same prefix, but the bytes lead out of $HOME.
	reaped := t.TempDir() // a sibling temp dir — what a scratchpad looks like
	link := filepath.Join(home, "scratch")
	if err := os.Symlink(reaped, link); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, home+string(filepath.Separator)) {
		t.Fatalf("the fixture is not shaped like the attack: %s is not textually under %s", link, home)
	}
	write(t, a.ConfigPath, "worktrees: "+link+"\n")
	got, err := a.WorktreeRoot()
	if err == nil {
		t.Fatalf("a symlink under $HOME pointing at %s was accepted as %q — a reaper walks there", reaped, got)
	}
	if !strings.Contains(err.Error(), "outside $HOME") {
		t.Errorf("the refusal must name the rule it enforces, got: %v", err)
	}

	// And through the link's own children, not just the link itself.
	write(t, a.ConfigPath, "worktrees: "+filepath.Join(link, "posse")+"\n")
	if _, err := a.WorktreeRoot(); err == nil {
		t.Errorf("a path THROUGH a symlink out of $HOME was accepted")
	}
}
