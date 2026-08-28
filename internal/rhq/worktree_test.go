package rhq

// Per-session git worktrees (rangerhq-09o2): the unit half. These drive
// worktree.go directly against real git repositories — git is the substrate
// under test, and a fake one would only pin what we believe about it.
//
// The dispatch-level and cross-process claims live in worktree_qa_test.go.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wtApp is an App whose $HOME is a temp dir, so the under-$HOME placement
// rule is satisfied by the default worktree root and a test never writes
// into the operator's real ~/.posse.
func wtApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	h := t.TempDir()
	return &App{
		Home: h, ConfigPath: filepath.Join(h, "config.yaml"),
		RecipesDir: filepath.Join(h, "recipes"), EnvsDir: filepath.Join(h, "envs"),
		StateDir: filepath.Join(h, "state"), AgentsDir: filepath.Join(h, "agents"),
		ModelLister: &ModelLister{},
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(dir, args...)
	if err != nil {
		t.Fatalf("git %s in %s: %v", strings.Join(args, " "), dir, err)
	}
	return out
}

// wtRepo is a real one-commit git repo on `main`.
func wtRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, "README.md"), "seed\n")
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	return repo
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitIn makes one commit the way a persona does: the path-limited form
// the wall requires, so the test exercises the same shape it does. The `add`
// is not optional and is worth knowing — `git commit -- <path>` on a path
// git has never seen fails with "pathspec did not match any file(s) known to
// git", so the blessed form needs an `add` in front of it for a NEW file.
func commitIn(t *testing.T, dir, path, body, msg string) {
	t.Helper()
	write(t, filepath.Join(dir, path), body)
	mustGit(t, dir, "config", "user.email", "p@example.com")
	mustGit(t, dir, "config", "user.name", "p")
	mustGit(t, dir, "add", path)
	mustGit(t, dir, "commit", "-q", "-m", msg, "--", path)
}

// ─── placement ───────────────────────────────────────────────────────────────

// The one rule this feature owns, and the reason it owns it: bd's own
// boundary check is PARTIAL (ranger-base-9ypc — it refuses a tmp BEADS_DIR
// only on the ~50 commands that reach GetRepoContext, and `bd worktree
// create /tmp/<name>` succeeds while writing a redirect that silently does
// not resolve), so nothing under us stops a session worktree landing in a
// directory a reaper walks.
func TestWorktreeRootMustBeUnderHome(t *testing.T) {
	a := wtApp(t)
	if _, err := a.WorktreeRoot(); err != nil {
		t.Fatalf("the default root is under $HOME and must be accepted: %v", err)
	}
	outside := t.TempDir() // a sibling temp dir: not under the test's $HOME
	write(t, a.ConfigPath, "worktrees: "+outside+"\n")
	_, err := a.WorktreeRoot()
	if err == nil {
		t.Fatalf("a worktrees root outside $HOME was accepted: %s", outside)
	}
	if !strings.Contains(err.Error(), "outside $HOME") {
		t.Errorf("the refusal must name the rule it enforces, got: %v", err)
	}
}

func TestWorktreeRootHonoursConfig(t *testing.T) {
	a := wtApp(t)
	want := filepath.Join(os.Getenv("HOME"), "trees")
	write(t, a.ConfigPath, "worktrees: ~/trees\n")
	got, err := a.WorktreeRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("worktrees root = %s, want %s", got, want)
	}
}

// The backend harness must hand out a temp $HOME too, not just wtApp:
// DefaultWorktreeRoot reads $HOME at call time, so a backend test that
// reaches EnsureSessionTree without one cuts a real git worktree in the
// operator's live ~/.posse (ranger-base-gvrh — a stray tree was found
// there). The root check runs before anything writes, so this test cannot
// itself litter the operator's home when it fails.
func TestNewTestBackendGetsATempHome(t *testing.T) {
	real := os.Getenv("HOME")
	b, _ := newTestBackend(t)
	home := os.Getenv("HOME")
	if home == real {
		t.Fatalf("newTestBackend left $HOME at the operator's own %s — every write under it is real", real)
	}
	// The write itself: this is the call that made the stray tree. It runs
	// only after the $HOME check above, so a regression fails before it can
	// cut anything.
	tree, err := b.App.EnsureSessionTree(wtRepo(t), "ranger-posse-a-1", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if tree == nil {
		t.Fatal("no session tree was cut, so this test measured nothing")
	}
	if real != "" && pathUnder(tree.Path, filepath.Join(real, ".posse")) {
		t.Fatalf("session tree %s was cut in the operator's live ~/.posse", tree.Path)
	}
	if !pathUnder(tree.Path, home) {
		t.Fatalf("session tree %s was cut outside the test's $HOME %s", tree.Path, home)
	}
}

// ─── making the tree ─────────────────────────────────────────────────────────

func TestEnsureSessionTreeIsPrivateAndIdempotent(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)

	tr, err := a.EnsureSessionTree(repo, "developer-repo-x-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if tr == nil {
		t.Fatal("a git repo on a branch must get a session worktree")
	}
	if tr.Branch != "posse/developer-repo-x-1" || tr.Base != "main" {
		t.Errorf("branch/base = %q/%q", tr.Branch, tr.Base)
	}
	if resolveExisting(tr.Path) == resolveExisting(repo) {
		t.Fatal("the session tree is the shared checkout — the whole point is that it is not")
	}
	// Its own HEAD, and its own index: a linked worktree's git dir is not
	// the common one, which is what "own index" means in git's terms.
	if head := mustGit(t, tr.Path, "symbolic-ref", "--short", "HEAD"); head != tr.Branch {
		t.Errorf("worktree HEAD = %q, want %q", head, tr.Branch)
	}
	gd := mustGit(t, tr.Path, "rev-parse", "--absolute-git-dir")
	cd := mustGit(t, tr.Path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if resolveExisting(gd) == resolveExisting(cd) {
		t.Errorf("the worktree shares the common git dir (%s) — its index is not private", gd)
	}

	again, err := a.EnsureSessionTree(repo, "developer-repo-x-1", nil)
	if err != nil {
		t.Fatalf("a second launch into the same session must find its tree: %v", err)
	}
	if again.Path != tr.Path {
		t.Errorf("second ensure moved the tree: %s → %s", tr.Path, again.Path)
	}
}

// Two sessions in one repo are the incident's own shape (rangerhq-nyqj): two
// trees, two indexes, and neither commit touching the other's staged work.
func TestTwoSessionsGetSeparateTreesAndIndexes(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)

	one, err := a.EnsureSessionTree(repo, "developer-repo-a-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.EnsureSessionTree(repo, "qa-repo-b-2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if one.Path == two.Path {
		t.Fatal("two sessions got one tree")
	}

	// developer stages six paths and has not committed yet — the rangerhq-2f5r
	// posture exactly.
	write(t, filepath.Join(one.Path, "mine.txt"), "developer's fix\n")
	mustGit(t, one.Path, "add", "mine.txt")

	// qa commits in its tree, the unqualified way that used to sweep.
	write(t, filepath.Join(two.Path, "theirs.txt"), "qa's fix\n")
	mustGit(t, two.Path, "config", "user.email", "l@example.com")
	mustGit(t, two.Path, "config", "user.name", "l")
	mustGit(t, two.Path, "add", "theirs.txt")
	mustGit(t, two.Path, "commit", "-q", "-m", "qa's bead")

	// The sweep: qa's commit must not carry developer's path…
	files := mustGit(t, two.Path, "show", "--name-only", "--format=", "HEAD")
	if strings.Contains(files, "mine.txt") {
		t.Errorf("qa's commit swept developer's staged file:\n%s", files)
	}
	// …and developer's staging must still be there afterwards.
	staged := mustGit(t, one.Path, "diff", "--cached", "--name-only")
	if staged != "mine.txt" {
		t.Errorf("developer's index after qa's commit = %q, want \"mine.txt\"", staged)
	}
	// The working-tree half of the same incident: qa's tree never had
	// developer's file to commit in the first place.
	if _, err := os.Stat(filepath.Join(two.Path, "mine.txt")); err == nil {
		t.Error("developer's in-flight file is visible in qa's tree")
	}
}

func TestEnsureSessionTreeSkipsWhatItCannotIsolate(t *testing.T) {
	a := wtApp(t)

	if tr, err := a.EnsureSessionTree(t.TempDir(), "s-1", nil); err != nil || tr != nil {
		t.Errorf("a dir that is not a git repo must fall back to itself, got (%v, %v)", tr, err)
	}

	repo := wtRepo(t)
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")
	var warn strings.Builder
	tr, err := a.EnsureSessionTree(repo, "s-2", &warn)
	if err != nil || tr != nil {
		t.Errorf("a detached HEAD has no branch to cut from or merge into, got (%v, %v)", tr, err)
	}
	if !strings.Contains(warn.String(), "detached HEAD") || !strings.Contains(warn.String(), "SHARED checkout") {
		t.Errorf("the fallback must say what the session lost:\n%s", warn.String())
	}
}

// The other side of that fallback: a session whose tree and branch already
// exist keeps them while the operator's checkout is detached. The branch
// carries its own base, so nothing here has to be guessed from HEAD — and
// answering "no worktree" would tell every later close and kill that a live
// private tree is the shared checkout, with nothing to land (ranger-base-q5p1).
func TestEnsureSessionTreeKeepsAnExistingTreeWhileHeadIsDetached(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	first, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil || first == nil {
		t.Fatalf("EnsureSessionTree = (%v, %v)", first, err)
	}
	commitIn(t, first.Path, "fix.txt", "the persona's work\n", "s-1: the fix")
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")

	var warn strings.Builder
	again, err := a.EnsureSessionTree(repo, "s-1", &warn)
	if err != nil {
		t.Fatal(err)
	}
	if again == nil {
		t.Fatal("a detached checkout demoted an existing session tree to the shared checkout")
	}
	if *again != *first {
		t.Errorf("the tree changed under a detached HEAD: %+v, want %+v", again, first)
	}
	// The landing is deferred, not lost, and the operator is told which of
	// the two it is.
	if !strings.Contains(warn.String(), "detached HEAD") || !strings.Contains(warn.String(), first.Base) {
		t.Errorf("the deferral must name the branch the work still lands on:\n%s", warn.String())
	}
	if strings.Contains(warn.String(), "SHARED checkout") {
		t.Errorf("a private tree was reported as shared:\n%s", warn.String())
	}
	if o, err := MergeSessionWork(again); err != nil || o.Merged || !strings.Contains(o.Reason, "detached HEAD") {
		t.Errorf("merge-back = (%+v, %v), want a deferral naming the detached HEAD", o, err)
	}
}

// ─── the beads redirect: the graph must not fork ─────────────────────────────

func TestSessionTreeSeedsAnAbsoluteBeadsRedirect(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	write(t, filepath.Join(repo, ".beads", "issues.jsonl"), "")

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := readRedirect(t, tr.Path)
	if got != filepath.Join(repo, ".beads") {
		t.Errorf("redirect = %q, want the repo's own .beads (%q)", got, filepath.Join(repo, ".beads"))
	}
	if !filepath.IsAbs(got) {
		t.Errorf("redirect must be absolute — bd's relative form resolves against the worktree ROOT, and one %q off falls back silently", "..")
	}
}

// A repo whose own .beads is already a redirect (this one is: posse's .beads
// serves ranger-base's database) must not get a CHAIN — the worktree points
// at the real database directly.
func TestSessionTreeResolvesAChainedRedirect(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	real := t.TempDir()
	write(t, filepath.Join(repo, ".beads", "redirect"), real+"\n")

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRedirect(t, tr.Path); got != real {
		t.Errorf("redirect = %q, want the resolved database %q (not a chain through the repo)", got, real)
	}
}

func TestSessionTreeRedirectFollowsARelativeOne(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "elsewhere", "keep"), "")
	write(t, filepath.Join(repo, ".beads", "redirect"), "elsewhere\n")

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readRedirect(t, tr.Path); got != filepath.Join(repo, "elsewhere") {
		t.Errorf("redirect = %q, want it resolved against the repo root", got)
	}
}

func TestNoBeadsInRepoSeedsNoRedirect(t *testing.T) {
	a := wtApp(t)
	tr, err := a.EnsureSessionTree(wtRepo(t), "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tr.Path, ".beads")); err == nil {
		t.Error("a repo with no beads got a .beads directory it never had")
	}
}

func readRedirect(t *testing.T, tree string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tree, ".beads", "redirect"))
	if err != nil {
		t.Fatalf("no beads redirect in the session tree: %v", err)
	}
	return strings.TrimSpace(string(b))
}

// ─── the gitignored things a checkout does not carry ─────────────────────────

func TestWorktreeLinkSeedsDeclaredPaths(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "plugin", "bin", "posse"), "#!/bin/sh\n")
	write(t, a.ConfigPath, "worktree_link:\n  - plugin/bin\n  - never/made\n")

	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tr.Path, "plugin", "bin")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("plugin/bin was not linked into the session tree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(link, "posse")); err != nil {
		t.Errorf("the link does not reach the main checkout's file: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(tr.Path, "never", "made")); err == nil {
		t.Error("a declared path the main checkout does not have was invented in the worktree")
	}
}

func TestWorktreeLinkRefusesToEscapeTheRepo(t *testing.T) {
	a := wtApp(t)
	write(t, a.ConfigPath, "worktree_link:\n  - ../../etc\n")
	_, err := a.EnsureSessionTree(wtRepo(t), "s-1", nil)
	if err == nil || !strings.Contains(err.Error(), "inside the repo") {
		t.Fatalf("a worktree_link escaping the repo must refuse, got %v", err)
	}
}

// ─── merging back (option A, rangerhq-jbyr) ──────────────────────────────────

func TestMergeSessionWorkFastForwards(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || o.Commits != 1 || o.Rebased {
		t.Fatalf("outcome = %+v, want one commit fast-forwarded", o)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the work\n" {
		t.Errorf("the work is not in the main checkout: %v", err)
	}
	// Idempotent: a second merge has nothing to do and says so rather than
	// failing, because a kill may land what a pass already landed.
	again, err := MergeSessionWork(tr)
	if err != nil || !again.Merged || again.Commits != 0 {
		t.Errorf("second merge = %+v, %v; want merged with nothing to do", again, err)
	}
}

func TestMergeSessionWorkRebasesWhenTheBaseMoved(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	commitIn(t, repo, "other.txt", "meanwhile\n", "main moved")

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || !o.Rebased {
		t.Fatalf("outcome = %+v, want a rebase then a fast-forward", o)
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err != nil {
		t.Errorf("the session's work did not land: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "other.txt")); err != nil {
		t.Errorf("the merge lost what main already had: %v", err)
	}
}

// The failure that must never cost work: a conflict leaves the branch, the
// tree and the repo exactly as they were, and says why.
func TestMergeSessionWorkRefusesAConflictAndKeepsEverything(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "clash.txt", "the session's line\n", "s-1: mine")
	commitIn(t, repo, "clash.txt", "the operator's line\n", "main: theirs")
	before := mustGit(t, repo, "rev-parse", tr.Branch)

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatal("a conflicting branch was reported merged")
	}
	if o.Commits != 1 || !strings.Contains(o.Reason, "conflict") {
		t.Errorf("outcome = %+v, want one unmerged commit and a conflict reason", o)
	}
	if after := mustGit(t, repo, "rev-parse", tr.Branch); after != before {
		t.Errorf("the branch moved under a failed merge: %s → %s", before, after)
	}
	// The rebase was aborted, not left half-applied — the session's tree is
	// usable and still holds its own version.
	if st := mustGit(t, tr.Path, "status", "--porcelain"); st != "" {
		t.Errorf("the session tree was left mid-rebase:\n%s", st)
	}
	if body, _ := os.ReadFile(filepath.Join(repo, "clash.txt")); string(body) != "the operator's line\n" {
		t.Errorf("the main checkout was modified by a failed merge: %q", body)
	}
}

// ranger-base-dybv, the shape that cost rangerhq-vojc a day: the persona's
// commits are in the TREE and the branch does not reach them, so every
// question asked of the branch answers "nothing to land" and the close
// reports success over work that is on no base, and one `posse kill` away
// from gone.
//
// One arm per sentence the refusal can say, plus the control — an assertion
// that a merge did NOT happen is satisfied by a fixture that could never
// have merged at all, and a shared prescription makes two reasons look like
// one until something asks them apart.
func TestMergeSessionWorkRefusesWorkTheBranchDoesNotReach(t *testing.T) {
	cases := []struct {
		name    string
		detach  bool
		killRef bool   // delete the branch too, so nothing names the work
		want    string // the phrase only this arm says
	}{
		{name: "head on the branch (control)"},
		{name: "head off the branch", detach: true, want: "on neither main nor "},
		{name: "no branch reaches it", detach: true, killRef: true, want: "no branch here reaches it"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := wtApp(t)
			repo := wtRepo(t)
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			if c.detach {
				mustGit(t, tr.Path, "checkout", "-q", "--detach")
			}
			commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
			if c.killRef {
				mustGit(t, repo, "branch", "-D", tr.Branch)
			}
			head := mustGit(t, tr.Path, "rev-parse", "HEAD")

			o, err := MergeSessionWork(tr)
			if err != nil {
				t.Fatal(err)
			}
			if c.want == "" {
				if !o.Merged || o.Commits != 1 {
					t.Fatalf("the control did not land: %+v", o)
				}
				return
			}
			if o.Merged {
				t.Fatalf("work on no branch was reported merged: %+v", o)
			}
			if o.Commits != 1 {
				t.Errorf("outcome = %+v, want the tree's one unlanded commit counted, not the branch's zero", o)
			}
			// The sentence has to carry the sha and the way back: this is
			// the only place the commit is named, and nothing else knows it.
			if !strings.Contains(o.Reason, head[:12]) {
				t.Errorf("the reason does not name the stranded commit %s: %q", head[:12], o.Reason)
			}
			if !strings.Contains(o.Reason, "branch -f "+tr.Branch+" HEAD") {
				t.Errorf("the reason does not say how to get the work back: %q", o.Reason)
			}
			if !strings.Contains(o.Reason, c.want) {
				t.Errorf("the reason does not say which obstacle this is (want %q): %q", c.want, o.Reason)
			}
			// And nothing was moved to say it.
			if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err == nil {
				t.Error("the work reached the main checkout after all")
			}
			if _, err := os.Stat(filepath.Join(tr.Path, "fix.txt")); err != nil {
				t.Errorf("the tree that holds the only copy was disturbed: %v", err)
			}
		})
	}
}

func TestMergeSessionWorkReportsUncommittedWork(t *testing.T) {
	a := wtApp(t)
	tr, err := a.EnsureSessionTree(wtRepo(t), "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "committed\n", "s-1: the fix")
	write(t, filepath.Join(tr.Path, "forgotten.txt"), "never committed\n")

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged {
		t.Fatalf("uncommitted work must not block the commits that exist: %+v", o)
	}
	if len(o.Dirty) != 1 || !strings.Contains(o.Dirty[0], "forgotten.txt") {
		t.Errorf("dirty = %v, want the uncommitted path named", o.Dirty)
	}
}

func TestMergeSessionWorkSaysSoOnADetachedRepo(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")

	o, err := MergeSessionWork(&SessionTree{Repo: tr.Repo, Path: tr.Path, Branch: tr.Branch, Base: repoBranch(repo)})
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged || !strings.Contains(o.Reason, "detached HEAD") {
		t.Errorf("outcome = %+v, want a refusal naming the detached HEAD", o)
	}
}

// ranger-base-5s2o: the base is the branch the session was CUT from, and it
// is read back from the branch itself. It used to be read out of the repo's
// HEAD at merge time, which made an operator's `git checkout -b` redirect
// the persona's commits onto the operator's own branch.
func TestSessionTreeRemembersTheBaseItWasCutFrom(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	mustGit(t, repo, "checkout", "-q", "-b", "operator-side")

	// Every path that answers "where does this land" answers main, not the
	// branch the operator happens to be on now.
	plan, err := a.PlanSessionTree(repo, "s-1")
	if err != nil || plan == nil || plan.Base != "main" {
		t.Fatalf("planned base = %+v, %v; want main", plan, err)
	}
	if got := SessionTreeOf(&HerdrMeta{Repo: repo, Dir: tr.Path, Branch: tr.Branch}); got.Base != "main" {
		t.Errorf("the run record's base = %q, want main", got.Base)
	}
	trees, err := SessionTreesIn([]string{repo})
	if err != nil || len(trees) != 1 || trees[0].Base != "main" {
		t.Fatalf("listed trees = %+v, %v; want one on main", trees, err)
	}

	// And the merge refuses rather than landing on operator-side.
	before := mustGit(t, repo, "rev-parse", "main")
	o, err := MergeSessionWork(trees[0])
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged || !strings.Contains(o.Reason, "operator-side") {
		t.Fatalf("outcome = %+v, want a refusal naming the branch in the way", o)
	}
	for _, b := range []string{"main", "operator-side"} {
		if got := mustGit(t, repo, "rev-parse", b); got != before {
			t.Errorf("%s moved across a refusal: %s → %s", b, before, got)
		}
	}

	// Nothing is lost by refusing: back on the base, the same call lands it.
	mustGit(t, repo, "checkout", "-q", "main")
	if o, err = MergeSessionWork(trees[0]); err != nil || !o.Merged || o.Commits != 1 {
		t.Fatalf("outcome back on the base = %+v, %v; want the commit landed", o, err)
	}
}

// ─── retiring the tree ───────────────────────────────────────────────────────

func TestRemoveSessionTreeRefusesWhileWorkWouldBeLost(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")

	if err := RemoveSessionTree(tr, false); err == nil {
		t.Fatal("a tree holding an unmerged commit was removed")
	} else if !strings.Contains(err.Error(), "not on main") {
		t.Errorf("the refusal must name what would be lost, got: %v", err)
	}

	if _, err := MergeSessionWork(tr); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(tr.Path, "scratch.txt"), "uncommitted\n")
	if err := RemoveSessionTree(tr, false); err == nil {
		t.Fatal("a tree holding uncommitted work was removed")
	} else if !strings.Contains(err.Error(), "uncommitted") {
		t.Errorf("the refusal must name what would be lost, got: %v", err)
	}

	os.Remove(filepath.Join(tr.Path, "scratch.txt"))
	if err := RemoveSessionTree(tr, false); err != nil {
		t.Fatalf("a merged, clean tree must retire: %v", err)
	}
	if _, err := os.Stat(tr.Path); err == nil {
		t.Error("the worktree directory survived its removal")
	}
	if branchExists(repo, tr.Branch) {
		t.Error("the session branch survived its removal")
	}
	if list := mustGit(t, repo, "worktree", "list"); strings.Contains(list, tr.Path) {
		t.Errorf("git still lists the removed worktree:\n%s", list)
	}
}

// ─── the listing ─────────────────────────────────────────────────────────────

func TestListSessionTreesNamesWhatHasNotLanded(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	write(t, filepath.Join(tr.Path, "scratch.txt"), "uncommitted\n")

	var out strings.Builder
	if err := ListSessionTrees(&out, []string{repo}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{tr.Branch, "1 commit(s) not on main", "1 uncommitted path(s)"} {
		if !strings.Contains(got, want) {
			t.Errorf("listing does not say %q:\n%s", want, got)
		}
	}

	if _, err := MergeSessionWork(tr); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(tr.Path, "scratch.txt"))
	out.Reset()
	if err := ListSessionTrees(&out, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing unlanded") {
		t.Errorf("a landed, clean tree must read as safe to remove:\n%s", out.String())
	}
}

// The kill defers rather than waits when a launcher holds the lock, so
// `--land` is the path that finishes the job. It merges and never removes:
// it reads git, so it cannot tell a dead session's tree from a live one's.
func TestLandSessionTreesFinishesWhatAKillDeferred(t *testing.T) {
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")

	var out strings.Builder
	if err := LandSessionTrees(&out, a, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 commit(s) onto main") {
		t.Errorf("--land did not land the branch:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "fix.txt")); err != nil {
		t.Errorf("the work is not on main: %v", err)
	}
	if _, err := os.Stat(tr.Path); err != nil {
		t.Errorf("--land removed a tree it cannot prove is dead: %v", err)
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("--land deleted the branch of a tree it left standing")
	}
}

// The kill's non-blocking lock: with a launcher holding it, the session is
// still killed, nothing is merged, and nothing is lost.
func TestTryLockLaunchesDoesNotWait(t *testing.T) {
	a := wtApp(t)
	held, err := lockLaunches(a, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tryLockLaunches(a); ok {
		t.Fatal("the non-blocking take succeeded while the lock was held")
	}
	held.Release()
	got, ok := tryLockLaunches(a)
	if !ok {
		t.Fatal("the non-blocking take failed on a free lock")
	}
	got.Release()
}
