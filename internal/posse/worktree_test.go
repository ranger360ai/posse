package posse

// Per-session git worktrees (rangerhq-09o2): the unit half. These drive
// worktree.go directly against real git repositories — git is the substrate
// under test, and a fake one would only pin what we believe about it.
//
// The dispatch-level and cross-process claims live in worktree_qa_test.go.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// wtApp is an App whose default worktree root is this test's own, so the
// under-$HOME placement rule is satisfied for real and a test never writes
// into the operator's ~/.posse. It used to buy that with t.Setenv("HOME"),
// which held 71 tests serial after ADR 0047 made the guarantee two cheaper
// ways: TestMain gives the binary a temp $HOME, and hermetic puts the root
// at $HOME/worktrees/<t.Name()> (ranger-base-pj87l).
func wtApp(t *testing.T) *App {
	t.Helper()
	h := t.TempDir()
	return hermetic(t, &App{
		Home: h, ConfigPath: filepath.Join(h, "config.yaml"),
		RecipesDir: filepath.Join(h, "recipes"), EnvsDir: filepath.Join(h, "envs"),
		StateDir: filepath.Join(h, "state"), AgentsDir: filepath.Join(h, "agents"),
	})
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
	t.Parallel()
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
	t.Parallel()
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
func TestTheTestBinaryGetsATempHome(t *testing.T) {
	t.Parallel()
	// The guarantee moved from newTestBackend to TestMain with ADR 0047 D1
	// — one temp home for the binary rather than one per test — but it is
	// the same guarantee and this is still the test that measures it, by
	// running the call that made the stray tree.
	real := operatorHome
	b, _ := newTestBackend(t)
	home := os.Getenv("HOME")
	if real != "" && home == real {
		t.Fatalf("the test binary left $HOME at the operator's own %s — every write under it is real", real)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	a := wtApp(t)
	write(t, a.ConfigPath, "worktree_link:\n  - ../../etc\n")
	_, err := a.EnsureSessionTree(wtRepo(t), "s-1", nil)
	if err == nil || !strings.Contains(err.Error(), "inside the repo") {
		t.Fatalf("a worktree_link escaping the repo must refuse, got %v", err)
	}
}

// ─── merging back (option A, rangerhq-jbyr) ──────────────────────────────────

func TestMergeSessionWorkFastForwards(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// ranger-base-59fs: the base moving DURING the rebase, which is the one
// thing the rebase exists to absorb and the one thing nothing re-asked
// after it. `notOnBase` was re-asked (the operator switching branches), the
// base's POSITION was not, so a fast-forward that lost the race printed the
// same "would not fast-forward" sentence as a branch that genuinely cannot
// land, and the launcher filed a merge-back-blocked bead for a human — 15
// seconds before the next pass landed the same untouched branch.
//
// The seam is git's own `post-rewrite` hook: it fires exactly once per
// rebase, from the COMMON hooks dir, for a rebase run in a linked worktree
// (measured, git 2.50.1) — so a commit made from it lands on the base in
// precisely the window between the rebase finishing and the ff running.
//
// Three arms, because the claim has three halves: it recovers, it does not
// replay when nothing moved, and it stops.
func TestMergeSessionWorkReplaysWhenTheBaseMovesUnderTheRebase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// moves is how many rebases the operator commits under. 0 is the
		// control: without it, "the loop recovered" is indistinguishable
		// from "there was never a race".
		moves      int
		wantMerged bool
		wantRebase int    // post-rewrite firings — the loop's real trip count
		wantReason string // "" when it must land
	}{
		{
			name:  "nothing moves under the rebase, so it replays once and lands",
			moves: 0, wantMerged: true, wantRebase: 1,
		},
		{
			name:  "the base moves under the first rebase, so the second lands it",
			moves: 1, wantMerged: true, wantRebase: 2,
		},
		{
			name:       "the base moves under every replay, so it stops and says which",
			moves:      mergeRebaseAttempts,
			wantMerged: false, wantRebase: mergeRebaseAttempts,
			wantReason: "never held still",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := wtApp(t)
			repo := wtRepo(t)
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
			// The base has to have moved ALREADY, or MergeSessionWork
			// fast-forwards on the first try and never reaches a rebase at
			// all — the hook would then never fire and every arm here would
			// measure nothing.
			commitIn(t, repo, "other.txt", "meanwhile\n", "main moved")
			count := raceOnRebase(t, repo, c.moves)

			o, err := MergeSessionWork(tr)
			if err != nil {
				t.Fatal(err)
			}
			if got := countIn(t, count); got != c.wantRebase {
				t.Errorf("the rebase ran %d time(s), want %d — the fixture did not race the way this arm needs", got, c.wantRebase)
			}
			if o.Merged != c.wantMerged {
				t.Fatalf("Merged = %v, want %v (reason %q)", o.Merged, c.wantMerged, o.Reason)
			}
			if !o.Rebased {
				t.Error("Rebased = false over a base that had moved")
			}
			if c.wantMerged {
				if o.Reason != "" {
					t.Errorf("a landed merge carries a reason: %q", o.Reason)
				}
				// Both halves of the race are on the base: the session's
				// work, and everything the operator committed while it was
				// being replayed. A retry that dropped either would still
				// satisfy "Merged".
				for _, f := range append([]string{"fix.txt", "other.txt"}, racedFiles(c.moves)...) {
					if _, err := os.Stat(filepath.Join(repo, f)); err != nil {
						t.Errorf("%s is not in the main checkout after the merge: %v", f, err)
					}
				}
				return
			}
			if !strings.Contains(o.Reason, c.wantReason) {
				t.Errorf("reason = %q, want it to say %q", o.Reason, c.wantReason)
			}
			if strings.Contains(o.Reason, "still would not fast-forward after the rebase") {
				t.Error("a lost race still reports as a branch that cannot land — that is the sentence this bead was filed over")
			}
			// Giving up costs nothing: the work is still named by the
			// branch, so the next pass (or `posse worktrees --land`) has
			// everything it needs.
			head := mustGit(t, tr.Path, "rev-parse", "HEAD")
			if !reaches(repo, tr.Branch, head) {
				t.Errorf("%s does not reach the tree's work %s — giving up stranded it", tr.Branch, abbrevSHA(head))
			}
			if reaches(repo, tr.Base, head) {
				t.Error("the base reaches the work, so this arm never measured a refusal at all")
			}
		})
	}
}

// raceOnRebase installs the `post-rewrite` hook that makes the operator
// commit on the base while a rebase is finishing, `moves` times, and returns
// the path of the counter every firing bumps. The counter is the positive
// witness: an assertion that a merge did or did not happen is payable by a
// fixture that never raced, and this is the only thing that can tell them
// apart.
//
// The hook commits path-limited because the crew's L1 shim is on PATH in a
// persona's session and refuses the sweeping forms — a fixture spelled `git
// commit -qm <msg>` is red for those personas and green for everyone else.
func raceOnRebase(t *testing.T, repo string, moves int) string {
	t.Helper()
	hooks := mustGit(t, repo, "rev-parse", "--git-path", "hooks")
	if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(repo, hooks)
	}
	count := filepath.Join(t.TempDir(), "rebases")
	hook := filepath.Join(hooks, "post-rewrite")
	write(t, hook, "#!/bin/sh\n"+
		"n=$(cat "+shq(count)+" 2>/dev/null || echo 0)\n"+
		"n=$((n+1))\n"+
		"echo \"$n\" > "+shq(count)+"\n"+
		"if [ \"$n\" -le "+strconv.Itoa(moves)+" ]; then\n"+
		"  unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_REFLOG_ACTION GIT_AUTHOR_DATE GIT_COMMITTER_DATE\n"+
		"  cd "+shq(repo)+" || exit 0\n"+
		"  echo \"$n\" > \"raced-$n.txt\"\n"+
		"  git add -- \"raced-$n.txt\"\n"+
		"  git commit -q -m \"the operator commits on the base mid-rebase ($n)\" -- \"raced-$n.txt\"\n"+
		"fi\n"+
		"exit 0\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	return count
}

func racedFiles(moves int) []string {
	var f []string
	for i := 1; i <= moves; i++ {
		f = append(f, "raced-"+strconv.Itoa(i)+".txt")
	}
	return f
}

func countIn(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("unreadable rebase count %q: %v", b, err)
	}
	return n
}

// The failure that must never cost work: a conflict leaves the branch, the
// tree and the repo exactly as they were, and says why.
func TestMergeSessionWorkRefusesAConflictAndKeepsEverything(t *testing.T) {
	t.Parallel()
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

// ranger-base-5hqa: EVERY non-zero exit of the replay printed the conflict
// sentence, and the P1 under it told a persona to go fix conflicts that were
// not there — measured on a pass where the box was at 100% disk and the
// branch it named replayed onto all 16 main tips in the range cleanly. The
// three arms are the whole claim: a rebase that stops ON a merge still says
// conflicts, one that never got that far says what git said instead, and the
// same fixture WITHOUT the refusal lands — so the middle arm's failure is the
// refusal and not a fixture that could never merge.
func TestMergeSessionWorkTellsAConflictFromARebaseThatNeverMerged(t *testing.T) {
	t.Parallel()
	const hookSaid = "no space left on device (simulated)"
	// A pre-rebase hook is the cheapest honest stand-in for the class the
	// bead is about — a full disk, a lock, a bad object: git exits non-zero
	// before any merge, and leaves no rebase state behind.
	setup := func(t *testing.T, conflict, refuse bool) (*SessionTree, string) {
		t.Helper()
		a := wtApp(t)
		repo := wtRepo(t)
		tr, err := a.EnsureSessionTree(repo, "s-1", nil)
		if err != nil {
			t.Fatal(err)
		}
		commitIn(t, tr.Path, "clash.txt", "the session's line\n", "s-1: mine")
		if conflict {
			commitIn(t, repo, "clash.txt", "the operator's line\n", "main: theirs")
		} else {
			// Main moved, so the ff refuses and the replay runs — but the
			// replay itself has nothing to conflict over.
			commitIn(t, repo, "elsewhere.txt", "the operator's line\n", "main: moved on")
		}
		if refuse {
			hooks := t.TempDir()
			hook := filepath.Join(hooks, "pre-rebase")
			write(t, hook, "#!/bin/sh\necho '"+hookSaid+"' >&2\nexit 1\n")
			if err := os.Chmod(hook, 0o755); err != nil {
				t.Fatal(err)
			}
			mustGit(t, repo, "config", "core.hooksPath", hooks)
		}
		return tr, repo
	}

	t.Run("a rebase that stopped on a merge still says conflicts", func(t *testing.T) {
		tr, _ := setup(t, true, false)
		o, err := MergeSessionWork(tr)
		if err != nil {
			t.Fatal(err)
		}
		if o.Merged {
			t.Fatal("a conflicting branch was reported merged")
		}
		if !strings.Contains(o.Reason, "conflicts — the rebase was aborted, so this attempt changed nothing") {
			t.Errorf("reason = %q, want the conflict sentence", o.Reason)
		}
		// And git's own words are in it either way, which is the witness
		// nobody had when the disk was the cause.
		if !strings.Contains(o.Reason, "could not apply") {
			t.Errorf("reason = %q, want git's own message in it", o.Reason)
		}
	})

	t.Run("a rebase that never merged names what git said", func(t *testing.T) {
		tr, repo := setup(t, false, true)
		before := mustGit(t, repo, "rev-parse", tr.Branch)

		o, err := MergeSessionWork(tr)
		if err != nil {
			t.Fatal(err)
		}
		if o.Merged {
			t.Fatal("a branch whose replay failed was reported merged")
		}
		if strings.Contains(o.Reason, "onto it conflicts") {
			t.Errorf("a rebase that never merged was reported as a conflict: %q", o.Reason)
		}
		if !strings.Contains(o.Reason, hookSaid) {
			t.Errorf("reason = %q, want git's own message (%q) in it", o.Reason, hookSaid)
		}
		// The refusal costs nothing, the way the conflict one does not.
		if after := mustGit(t, repo, "rev-parse", tr.Branch); after != before {
			t.Errorf("the branch moved under a failed replay: %s → %s", before, after)
		}
		if st := mustGit(t, tr.Path, "status", "--porcelain"); st != "" {
			t.Errorf("the session tree was left mid-rebase:\n%s", st)
		}
	})

	t.Run("the same fixture lands without the refusal", func(t *testing.T) {
		tr, repo := setup(t, false, false)
		o, err := MergeSessionWork(tr)
		if err != nil {
			t.Fatal(err)
		}
		if !o.Merged || !o.Rebased {
			t.Fatalf("outcome = %+v, want the replay to land — the arm above is measuring its hook, not an unmergeable fixture", o)
		}
		if body, _ := os.ReadFile(filepath.Join(repo, "clash.txt")); string(body) != "the session's line\n" {
			t.Errorf("the work did not reach the base: %q", body)
		}
	})
}

// The predicate the arms above rest on, asked directly: the state directory
// is what tells the two apart, and it is gone after the abort — so reading it
// even one line later answers "not a conflict" for every conflict there is.
func TestRebaseStoppedIsTrueOnlyWhileTheMergeIsWaiting(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "clash.txt", "the session's line\n", "s-1: mine")
	commitIn(t, repo, "clash.txt", "the operator's line\n", "main: theirs")

	if rebaseStopped(tr.Path) {
		t.Fatal("rebaseStopped is true before any rebase ran")
	}
	if _, err := git(tr.Path, "rebase", "main"); err == nil {
		t.Fatal("the fixture rebased cleanly — it is not the conflict shape")
	}
	if !rebaseStopped(tr.Path) {
		t.Error("a rebase stopped on a conflict was not seen as stopped")
	}
	if _, err := git(tr.Path, "rebase", "--abort"); err != nil {
		t.Fatal(err)
	}
	if rebaseStopped(tr.Path) {
		t.Error("the rebase state outlived the abort")
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// ranger-base-as19: RemoveSessionTree asked its own question by sha, and by
// sha a cherry-picked commit is ahead of the base forever. So the tree of a
// branch MergeSessionWork had just reported Merged — naming the pairing —
// was refused retirement every pass, and the operator's only escape was the
// same override that stands down a real strand's refusal.
//
// Four arms, and the split between them IS the fix: patch-id equivalence
// measures content, so the branch is the last copy of nothing and retires;
// git's `-x` trailer records a human's decision about a resolution that may
// have dropped a hunk, so the branch is kept and told apart from a strand in
// words. The last two arms are the controls — without them "it retires" is
// satisfied by a guard that was simply deleted.
func TestRemoveSessionTreeRetiresOnlyWhatIsMeasuredOnTheBase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		land func(t *testing.T, repo, sha string)
		// "" means the tree and its branch must be gone; anything else is
		// the refusal the operator must be able to read.
		kept string
	}{{
		name: "a clean -x pick is patch-id equivalent and retires",
		land: func(t *testing.T, repo, sha string) { mustGit(t, repo, "cherry-pick", "-x", sha) },
	}, {
		// The trailer is not what licenses this one: there is none.
		name: "a pick with no trailer retires on patch-id alone",
		land: func(t *testing.T, repo, sha string) { mustGit(t, repo, "cherry-pick", sha) },
	}, {
		// The second layer of the content guard, alone (ranger-base-x8jp).
		// The base's TREE no longer holds the branch's bytes — it was edited
		// after the pick — but its history does, so this is not the last
		// copy of anything. Without this arm the guard could refuse on the
		// tree comparison alone and every other pin stays green, which is
		// the every-pass refusal ranger-base-as19 removed, back again.
		name: "a clean pick the base then built on top of still retires",
		land: func(t *testing.T, repo, sha string) {
			mustGit(t, repo, "cherry-pick", "-x", sha)
			commitIn(t, repo, "adr.md", "status: accepted (2026-08-29, amended on main)\n",
				"main: built on top of the pick")
		},
	}, {
		name: "a hand-resolved pick is kept, and says why",
		land: func(t *testing.T, repo, sha string) {
			commitIn(t, repo, "adr.md", "status: accepted (2026-08-29, amended)\n",
				"s-1: the fix\n\n(cherry picked from commit "+sha+")")
		},
		kept: "-x trailer",
	}, {
		name: "real unlanded work is kept in the unchanged words",
		land: func(t *testing.T, repo, sha string) {
			commitIn(t, repo, "adr.md", "status: rejected\n", "main: the operator's own line")
		},
		kept: "commit(s) not on main — not removed",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := wtApp(t)
			repo := wtRepo(t)
			commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			// The base has to move first or the pick rebuilds the identical
			// commit object and the base reaches it by sha, which is the
			// case that was never broken (ranger-base-g2xf).
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			c.land(t, repo, sha)
			// Ahead by sha in every arm: that is the guard's own question,
			// and without this the retiring arms could be passing because
			// the guard was never reached.
			if n := mustGit(t, repo, "rev-list", "--count", "main.."+tr.Branch); n != "1" {
				t.Fatalf("rev-list --count main..%s = %s; the fixture must be ahead by sha", tr.Branch, n)
			}

			err = RemoveSessionTree(tr, false)
			if c.kept != "" {
				if err == nil {
					t.Fatal("the tree was retired on evidence that cannot prove the work is not lost")
				}
				if !strings.Contains(err.Error(), c.kept) {
					t.Errorf("the refusal must say which evidence it has, got: %v", err)
				}
				if _, serr := os.Stat(tr.Path); serr != nil {
					t.Errorf("the refused tree was removed anyway: %v", serr)
				}
				if !branchExists(repo, tr.Branch) {
					t.Error("the refused branch was deleted anyway")
				}
				return
			}
			if err != nil {
				t.Fatalf("a branch whose every patch is already on the base was not retired: %v", err)
			}
			if _, serr := os.Stat(tr.Path); serr == nil {
				t.Error("the worktree directory survived its removal")
			}
			if branchExists(repo, tr.Branch) {
				t.Error("the session branch survived its removal")
			}
			if list := mustGit(t, repo, "worktree", "list"); strings.Contains(list, tr.Path) {
				t.Errorf("git still lists the removed worktree:\n%s", list)
			}
		})
	}
}

// The kill's whole path, not the guard alone: `posse kill` reported Merged
// and then kept the tree anyway, which is the bug as the operator met it.
func TestKillRetiresACherryPickedTree(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
	sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
	commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	mustGit(t, repo, "cherry-pick", "-x", sha)

	o, err := MergeSessionWork(tr)
	if err != nil || !o.Merged || len(o.Equivalent) == 0 {
		t.Fatalf("outcome = %+v, %v; want the pick reported as already landed", o, err)
	}
	l := &KillLanding{Tree: tr, Merge: o}
	if err := RemoveSessionTree(tr, false); err != nil {
		l.Kept = err.Error()
	}
	if l.Kept != "" {
		t.Fatalf("the kill kept a tree it had just reported landed: %s", l.Kept)
	}
	if line := l.Line(); !strings.Contains(line, "removed") || strings.Contains(line, "KEPT") {
		t.Errorf("the kill's line = %q, want the retirement", line)
	}
}

// ─── the listing ─────────────────────────────────────────────────────────────

func TestListSessionTreesNamesWhatHasNotLanded(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	// The stamp a dispatched launch writes (worktree.go recordBead): --land
	// reads it and reports rather than lands a tree no record accounts for
	// (ranger-base-atxe), which is its own pin below.
	if err := recordBead(tr.Repo, tr.Branch, "a-1"); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := LandSessionTrees(&out, a, []string{repo}, false); err != nil {
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

// `--land` reads the bead record before it merges anything (ranger-base-atxe).
// The shape it exists for was measured in the field: a session tree held
// one commit main did not have BY SHA whose content is
// byte-identical to a commit already on main — re-landed
// by hand under another bead id, so no patch-id and no `-x` trailer connects
// them and equivalentOnBase is blind to it. The only thing that told the two
// apart was that nothing recorded which bead the tree was working, and the
// listing said "1 commit(s) not on main" either way.
//
// Three arms, because the refusal is only worth anything if it is narrow: a
// tree with a bead lands untouched, a tree without one is reported and NOT
// merged, and --force lands the second one anyway.
func TestLandWillNotTakeWorkNoBeadRecordAccountsFor(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)

	recorded, err := a.EnsureSessionTree(repo, "s-recorded", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, recorded.Path, "recorded.txt", "accounted for\n", "s-recorded: the fix")
	if err := recordBead(recorded.Repo, recorded.Branch, "a-1"); err != nil {
		t.Fatal(err)
	}
	orphan, err := a.EnsureSessionTree(repo, "s-orphan", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, orphan.Path, "orphan.txt", "nobody can say\n", "s-orphan: the fix")

	var out strings.Builder
	if err := LandSessionTrees(&out, a, []string{repo}, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	// The control: the gate is narrow, and a recorded tree still lands in
	// the same words it always did. Without this arm a `return "refused"`
	// for every tree passes everything below.
	if _, err := os.Stat(filepath.Join(repo, "recorded.txt")); err != nil {
		t.Errorf("--land refused a tree whose branch names its bead:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "orphan.txt")); err == nil {
		t.Errorf("--land merged work no record accounts for:\n%s", got)
	}
	for _, want := range []string{orphan.Branch, "no record says which bead", "--force"} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, got)
		}
	}
	// Reported, not destroyed: the branch and the tree still hold it.
	if !branchExists(repo, orphan.Branch) {
		t.Error("the refused tree's branch was deleted")
	}
	if _, err := os.Stat(filepath.Join(orphan.Path, "orphan.txt")); err != nil {
		t.Errorf("the refused tree lost its work: %v", err)
	}

	out.Reset()
	if err := LandSessionTrees(&out, a, []string{repo}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "orphan.txt")); err != nil {
		t.Errorf("--force did not land the unaccounted tree:\n%s", out.String())
	}
}

// The refusal's own sentence, one function up from the listing hk02 fixed
// (ranger-base-3nn9c). The gate is the record — no bead accounts for any of
// these trees and none of them is landed here — but the sentence printed
// beside the refusal used to assert "NOT landed" from a sha count alone, over
// a branch the listing in the same pass called nothing unlanded, and pointed
// at --force for work that does not exist.
//
// Same three arms and the same fixture premise as
// TestListSessionTreesTellsACherryPickedBranchFromAStrand: the base moves
// first, so a pick onto it is a new sha rather than the identical commit
// object. Every arm asserts the gate STILL HOLDS ("no record says which
// bead", nothing merged) — the ≡ line a dropped gate would print says
// "nothing here is unlanded" too, and only the record half tells the two
// apart.
func TestLandRefusalTellsAnAlreadyLandedTreeFromAStrand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		land   func(t *testing.T, repo, sha string)
		want   []string
		unwant []string
	}{{
		name: "a clean cherry-pick is refused for the record, not as unlanded work",
		land: func(t *testing.T, repo, sha string) {
			mustGit(t, repo, "cherry-pick", "-x", sha)
		},
		want:   []string{"no record says which bead", "equivalent patch on main", "nothing here is unlanded"},
		unwant: []string{"NOT landed", "--force", "recorded as landed in"},
	}, {
		name: "a hand-resolved pick is refused as recorded but not measured",
		land: func(t *testing.T, repo, sha string) {
			commitIn(t, repo, "adr.md", "status: accepted (2026-08-29, amended)\n",
				"s-1: the fix\n\n(cherry picked from commit "+sha+")")
		},
		want:   []string{"no record says which bead", "recorded as landed in", "not a measurement of what the resolution kept"},
		unwant: []string{"NOT landed", "--force", "nothing here is unlanded"},
	}, {
		name: "real unlanded work still reads as a strand, unchanged",
		land: func(t *testing.T, repo, sha string) {
			commitIn(t, repo, "adr.md", "status: rejected\n", "main: the operator's own line")
		},
		want:   []string{"no record says which bead", "1 commit(s) not on main", "NOT landed", "--force"},
		unwant: []string{"equivalent patch on main", "recorded as landed in"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := wtApp(t)
			repo := wtRepo(t)
			commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			c.land(t, repo, sha)
			// No recordBead: every arm here is a tree the gate refuses.
			was := mustGit(t, repo, "rev-parse", "main")

			var out strings.Builder
			if err := LandSessionTrees(&out, a, []string{repo}, false); err != nil {
				t.Fatal(err)
			}
			got := out.String()
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("the refusal does not say %q:\n%s", want, got)
				}
			}
			for _, unwant := range c.unwant {
				if strings.Contains(got, unwant) {
					t.Errorf("the refusal should not say %q:\n%s", unwant, got)
				}
			}
			if now := mustGit(t, repo, "rev-parse", "main"); now != was {
				t.Errorf("the gate did not hold: main moved %s → %s\n%s", was, now, got)
			}
			if !branchExists(repo, tr.Branch) {
				t.Error("the refused tree's branch was deleted")
			}
		})
	}
}

// The half an operator reads BEFORE they type --land. "1 commit(s) not on
// main" was the whole basis for that decision and it is true of a strand and
// of an already-landed duplicate alike; which bead the work belongs to is the
// difference, and both answers have to be printable (ranger-base-atxe).
func TestListSessionTreesNamesWhichBeadTheUnlandedWorkIsFor(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")

	var out strings.Builder
	if err := ListSessionTrees(&out, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 commit(s) not on main, no record says which bead") {
		t.Errorf("the listing hides that nothing accounts for this tree's work:\n%s", out.String())
	}

	if err := recordBead(tr.Repo, tr.Branch, "a-1"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := ListSessionTrees(&out, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 commit(s) not on main, for a-1") {
		t.Errorf("the listing does not name the bead the work belongs to:\n%s", out.String())
	}
	if strings.Contains(out.String(), "no record says which bead") {
		t.Errorf("a stamped branch still reads as unrecorded:\n%s", out.String())
	}
}

// ranger-base-g2xf: ahead by SHA is not ahead by work. A commit that was
// cherry-picked onto the base keeps its own sha on the branch, so the branch
// counts as ahead; and when the landing was resolved BY HAND the patches are
// no longer identical, so the replay cannot drop it by patch-id either and
// conflicts on the same hunk every pass. What the operator got was a strand
// report word-for-word identical to a real one — over work already on main.
//
// Three arms, because the difference between them is the whole feature: a
// hand-resolved pick (the reported bug, recognisable only by git's own `-x`
// trailer), a clean pick (patch-id equivalent, what `git cherry` sees), and
// the control — real unlanded work, which must still report a strand in the
// unchanged words. Without the control an "is not a strand" assertion is
// satisfied by a rig that could never have produced one.
func TestMergeSessionWorkTellsACherryPickedBranchFromAStrand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// land replays the session's commit onto the repo's branch the way
		// this arm's history did it, and returns "" for the arm that never
		// landed at all.
		land       func(t *testing.T, repo, sha string) string
		equivalent bool
		// measured says the pairing is a patch-id measurement of content
		// and so may print the confident sentence. The trailer arm is not
		// one, and saying so is ranger-base-dmzk7.
		measured bool
	}{{
		name: "a hand-resolved cherry-pick is not a strand",
		land: func(t *testing.T, repo, sha string) string {
			// The resolution keeps main's own newer wording, so the patch
			// differs from the session's and `git cherry` says `+`. The
			// trailer is the only thing left that knows.
			commitIn(t, repo, "adr.md", "status: accepted (2026-08-29, amended)\n",
				"s-1: the fix\n\n(cherry picked from commit "+sha+")")
			return mustGit(t, repo, "rev-parse", "HEAD")
		},
		equivalent: true,
	}, {
		name: "a clean cherry-pick is not a strand either",
		land: func(t *testing.T, repo, sha string) string {
			mustGit(t, repo, "cherry-pick", "-x", sha)
			return mustGit(t, repo, "rev-parse", "HEAD")
		},
		equivalent: true,
		measured:   true,
	}, {
		// The other half of equivalentOnBase, alone: no trailer at all, so
		// only patch-id can see it. Without this arm the `git cherry` half
		// could be deleted whole and every pin above stays green — `-x`
		// writes the trailer the other arms are recognised by.
		name: "a pick with no -x trailer is seen by patch-id alone",
		land: func(t *testing.T, repo, sha string) string {
			mustGit(t, repo, "cherry-pick", sha)
			return mustGit(t, repo, "rev-parse", "HEAD")
		},
		equivalent: true,
		measured:   true,
	}, {
		name: "real unlanded work still is",
		land: func(t *testing.T, repo, sha string) string {
			// The same conflicting shape and NO record that the session's
			// commit is what landed — because it is not.
			commitIn(t, repo, "adr.md", "status: rejected\n", "main: the operator's own line")
			return ""
		},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := wtApp(t)
			repo := wtRepo(t)
			commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			// The base moves first, in every arm: that is the premise of
			// the whole bug, and without it a cherry-pick onto an unmoved
			// base rebuilds the IDENTICAL commit object (same tree, parent,
			// message and identity — measured), which the base then reaches
			// by sha and no arm here measures anything.
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			pick := c.land(t, repo, sha)
			before := mustGit(t, repo, "rev-parse", tr.Branch)

			o, err := MergeSessionWork(tr)
			if err != nil {
				t.Fatal(err)
			}
			if !c.equivalent {
				if o.Merged || len(o.Equivalent) > 0 {
					t.Fatalf("real unlanded work was not reported as a strand: %+v", o)
				}
				// Unchanged words, so a genuine strand still reads the way
				// every runbook says it does.
				if !strings.Contains(o.Reason, "conflicts — the rebase was aborted, so this attempt changed nothing") {
					t.Errorf("the strand's reason changed: %q", o.Reason)
				}
				return
			}
			if !o.Merged || o.Reason != "" {
				t.Fatalf("a branch whose work is already on the base reported a strand: %+v", o)
			}
			if len(o.Equivalent) != 1 {
				t.Fatalf("Equivalent = %v, want the one commit paired with what holds it", o.Equivalent)
			}
			// The pairing has to be checkable by hand: it names both shas.
			if !strings.Contains(o.Equivalent[0], abbrevSHA(sha)) {
				t.Errorf("Equivalent %q does not name the session's commit %s", o.Equivalent[0], abbrevSHA(sha))
			}
			note := o.EquivalentNote()
			// "nothing here is unlanded" is a measurement claim, so only
			// the arms that MEASURED get to make it: the hand-resolved
			// pick's whole evidence is a trailer somebody wrote, which
			// cannot say what the resolution kept (ranger-base-dmzk7).
			last := "nothing here is unlanded"
			if !c.measured {
				last = "not a measurement of what the resolution kept"
			}
			for _, want := range []string{"1 commit(s)", tr.Branch, tr.Base, last} {
				if !strings.Contains(note, want) {
					t.Errorf("EquivalentNote() = %q, missing %q", note, want)
				}
			}
			if !c.measured && strings.Contains(note, "nothing here is unlanded") {
				t.Errorf("evidence that is not a measurement claimed one: %q", note)
			}
			// Nothing was touched to reach that answer: this is a read.
			if after := mustGit(t, repo, "rev-parse", tr.Branch); after != before {
				t.Errorf("the branch moved: %s → %s", before, after)
			}
			if head := mustGit(t, repo, "rev-parse", "HEAD"); head != pick {
				t.Errorf("the repo's checkout moved: %s → %s", pick, head)
			}
			if st := mustGit(t, tr.Path, "status", "--porcelain"); st != "" {
				t.Errorf("the session tree was left dirty:\n%s", st)
			}
		})
	}
}

// The fixture the arm above rests on has to be the reported shape and not an
// easier one: WITHOUT the trailer the replay really does conflict, so the
// hand-resolved arm is passing on the new evidence rather than on a rebase
// that would have succeeded anyway.
func TestHandResolvedPickReallyDoesConflictOnReplay(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
	sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
	commitIn(t, repo, "adr.md", "status: accepted (2026-08-29, amended)\n",
		"s-1: the fix\n\n(cherry picked from commit "+sha+")")

	if out, err := git(repo, "cherry", "main", tr.Branch); err != nil || !strings.HasPrefix(out, "+ ") {
		t.Fatalf("git cherry = %q, %v; the fixture must NOT be patch-id equivalent", out, err)
	}
	if _, err := git(tr.Path, "rebase", "main"); err == nil {
		t.Error("the fixture rebased cleanly — it is not the hand-resolved shape the bug is about")
	}
	_, _ = git(tr.Path, "rebase", "--abort")
}

// "Landed" is all-or-nothing, and nothing below the whole branch will do.
// One commit picked onto the base and one that never left the tree is a
// STRAND — the tree is still the only copy of the second — and a predicate
// that returns what it accounted for instead of refusing would call the
// branch landed on the strength of the first. (Mutation: turning
// equivalentOnBase's `return nil` into `continue` is invisible to every
// single-commit arm; this is the one that sees it.)
func TestMergeSessionWorkStillStrandsAPartlyLandedBranch(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
	picked := mustGit(t, tr.Path, "rev-parse", "HEAD")
	commitIn(t, tr.Path, "later.txt", "the part nobody landed\n", "s-1: the follow-up")

	commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	commitIn(t, repo, "adr.md", "status: accepted (2026-08-29, amended)\n",
		"s-1: the fix\n\n(cherry picked from commit "+picked+")")

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged || len(o.Equivalent) > 0 {
		t.Fatalf("a branch still holding one unlanded commit was called landed: %+v", o)
	}
	// The reason names the REPLAY as what refused. It used to assert "still
	// holds the work", which ranger-base-m3195 took out of this sentence:
	// the reason is embedded verbatim in a merge-back bead a seat opens some
	// unbounded time later, by which point the branch may have been retired
	// out from under it, so the wording reports what the attempt did rather
	// than promising what will still be true. What this test is about is
	// unchanged and is asserted above — !Merged with nothing accounted for.
	if !strings.Contains(o.Reason, "onto it conflicts") {
		t.Errorf("outcome = %+v, want the strand reason naming the failed replay", o)
	}
	// And the part that IS unlanded is still in the tree afterwards.
	if _, err := os.Stat(filepath.Join(tr.Path, "later.txt")); err != nil {
		t.Errorf("the unlanded commit's file is gone from the session tree: %v", err)
	}
}

// ranger-base-hk02: treeState counted shas the way MergeSessionWork used to
// (ranger-base-g2xf) — "N commit(s) not on main" reads the same for a strand
// and for a branch RemoveSessionTree is about to delete on the next pass. It
// now asks equivalentOnBase the same question those two already ask, and
// prints the two answers RemoveSessionTree already tells apart (measuredOnBase
// vs. the -x trailer alone) rather than one sentence for both.
//
// Three arms: a clean pick (measured, safe to retire), a hand-resolved pick
// (recorded only by the -x trailer, not yet safe), and the control — real
// unlanded work must still read as the unchanged strand sentence. Without the
// control, a listing that always said "nothing unlanded" would pass the first
// two arms.
func TestListSessionTreesTellsACherryPickedBranchFromAStrand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		land   func(t *testing.T, repo, sha string) string
		want   []string
		unwant []string
	}{{
		name: "a clean cherry-pick reads as nothing unlanded",
		land: func(t *testing.T, repo, sha string) string {
			mustGit(t, repo, "cherry-pick", "-x", sha)
			return mustGit(t, repo, "rev-parse", "HEAD")
		},
		want:   []string{"nothing unlanded", "equivalent patch on main"},
		unwant: []string{"not on main, no record", "compare before retiring"},
	}, {
		name: "a hand-resolved pick reads as recorded but not yet measured",
		land: func(t *testing.T, repo, sha string) string {
			commitIn(t, repo, "adr.md", "status: accepted (2026-08-29, amended)\n",
				"s-1: the fix\n\n(cherry picked from commit "+sha+")")
			return mustGit(t, repo, "rev-parse", "HEAD")
		},
		want:   []string{"commit(s) not on main by sha, recorded as landed in", "compare before retiring"},
		unwant: []string{"nothing unlanded", "no record says which bead"},
	}, {
		name: "real unlanded work still reads as a strand, unchanged",
		land: func(t *testing.T, repo, sha string) string {
			commitIn(t, repo, "adr.md", "status: rejected\n", "main: the operator's own line")
			return ""
		},
		want:   []string{"1 commit(s) not on main, no record says which bead"},
		unwant: []string{"nothing unlanded", "compare before retiring"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := wtApp(t)
			repo := wtRepo(t)
			commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-1: the fix")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			// The base moves first, same premise as ranger-base-g2xf's fixture:
			// a pick onto an unmoved base rebuilds the identical commit object
			// and the base reaches it by sha, measuring nothing here.
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			c.land(t, repo, sha)

			var out strings.Builder
			if err := ListSessionTrees(&out, []string{repo}); err != nil {
				t.Fatal(err)
			}
			got := out.String()
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("listing does not say %q:\n%s", want, got)
				}
			}
			for _, unwant := range c.unwant {
				if strings.Contains(got, unwant) {
					t.Errorf("listing should not say %q:\n%s", unwant, got)
				}
			}
		})
	}
}

// TestListSessionTreesWillNotCallAHalfLandedOrSquashedBranchLanded is the
// other side of TestListSessionTreesTellsACherryPickedBranchFromAStrand,
// added verifying ranger-base-hk02's close (verify bead ranger-base-pqc4v).
//
// hk02's fix taught the listing to say "nothing unlanded" — the one phrase
// that tells the operator a tree is safe to retire. The three arms that pin
// it all start from a branch every commit of which is a measured patch-id
// match. These two are the ways a branch is PARTLY on the base: half its
// commits picked, or all its content squashed into one commit whose
// per-commit patch-id matches nothing. Both must keep reading as a strand —
// widening the equivalence claim to cover either is how the listing starts
// inviting the operator to delete work that only LOOKS landed.
func TestListSessionTreesWillNotCallAHalfLandedOrSquashedBranchLanded(t *testing.T) {
	t.Parallel()
	listing := func(t *testing.T, repo string) string {
		t.Helper()
		var out strings.Builder
		if err := ListSessionTrees(&out, []string{repo}); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}

	t.Run("half the commits picked", func(t *testing.T) {
		a := wtApp(t)
		repo := wtRepo(t)
		commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
		tr, err := a.EnsureSessionTree(repo, "s-half", nil)
		if err != nil {
			t.Fatal(err)
		}
		commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-half: first")
		first := mustGit(t, tr.Path, "rev-parse", "HEAD")
		commitIn(t, tr.Path, "other.md", "second\n", "s-half: second, never landed")
		commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
		mustGit(t, repo, "cherry-pick", "-x", first)

		got := listing(t, repo)
		if strings.Contains(got, "nothing unlanded") {
			t.Errorf("a branch with one commit still off %s must not read as settled:\n%s", "main", got)
		}
		if !strings.Contains(got, "2 commit(s) not on main") {
			t.Errorf("the listing must still count what is off main:\n%s", got)
		}
	})

	t.Run("squashed onto the base", func(t *testing.T) {
		a := wtApp(t)
		repo := wtRepo(t)
		commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
		tr, err := a.EnsureSessionTree(repo, "s-squash", nil)
		if err != nil {
			t.Fatal(err)
		}
		commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s-squash: first")
		commitIn(t, tr.Path, "other.md", "second\n", "s-squash: second")
		commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
		// One commit on the base carrying BOTH changes: the content is
		// there, no per-commit patch-id is.
		write(t, filepath.Join(repo, "adr.md"), "status: accepted\n")
		write(t, filepath.Join(repo, "other.md"), "second\n")
		mustGit(t, repo, "add", "adr.md", "other.md")
		mustGit(t, repo, "commit", "-q", "-m", "squash of s-squash", "--", "adr.md", "other.md")

		got := listing(t, repo)
		if strings.Contains(got, "nothing unlanded") {
			t.Errorf("a squash matches no per-commit patch-id and must not read as settled:\n%s", got)
		}
	})
}

// ranger-base-d8o6: ONE TREE, ONE ANSWER. The two surfaces that say what a
// session tree holds — the listing an operator reads (`posse worktrees`,
// treeState) and the sweep that acts (MergeSessionWork, whose sentence both
// landClosedTrees and LandSessionTrees print) — used to ask different
// questions of the same tree and print contradicting answers about it, with
// the operator-facing one wrong. ranger-base-hk02 closed the first half: the
// listing counted shas where the sweep measured patch equivalence, so an
// already-re-landed duplicate read as work at risk forever.
//
// The half this pins is the other direction and the dangerous one: the
// listing asked `<base>..<branch>`, which is ZERO over a worktree whose HEAD
// is detached — the shape a caged launch creates ON PURPOSE
// (PrepareSessionHead, ranger-base-t4f1) — so it printed "nothing unlanded",
// its one phrase for a tree that is safe to retire, over the whole of that
// session's committed work, while MergeSessionWork on the same tree said the
// work is on neither the base nor the branch (ranger-base-dybv).
//
// So the assertion is the AGREEMENT itself and not two hand-written
// sentences: `heldWork` is derived from each surface in the vocabulary that
// surface actually publishes, and the arms compare them. Two sentences
// asserted separately is what let the disagreement live — each side's own
// pins were green while they contradicted each other in the field.
//
// The listing is read BEFORE the sweep in every arm, because the sweep ACTS:
// it lands the strand and splices the caged tree, and a listing read after
// would be describing a tree the assertion just changed.
//
// FOUR ARMS, and each is a control on the others. A listing that always said
// "nothing unlanded" passes the duplicate and fails the strand; one that
// never did passes the strand and fails the duplicate; and the two detached
// arms are what the sha count cannot see at all — with the launch stamp (the
// caged session, where the sweep splices the work back and lands it) and
// without it (where the sweep refuses and names the `branch -f` that rescues
// it, which the listing now names too).
func TestListingAndSweepAgreeOnWhatOneTreeHolds(t *testing.T) {
	t.Parallel()
	// THE VERDICT, read the way an operator reads it: off the sentence each
	// command prints. Both surfaces already publish the same three-word
	// vocabulary and they are asked for it here in the same words —
	// EquivalentNote's "nothing here is unlanded" is treeState's "nothing
	// unlanded", and its "before retiring the tree" is treeState's "compare
	// before retiring" — which is what makes an agreement assertion possible
	// at all rather than two hand-written expectations that can drift apart
	// while both stay green.
	//
	// Three and not two, because the middle one is the whole of
	// ranger-base-dmzk7: an equivalence whose evidence is a decision or an
	// identity match is neither settled nor a strand, and collapsing it onto
	// either neighbour is how a surface starts overstating what it knows.
	verdict := func(printed string) string {
		switch {
		case strings.Contains(printed, "before retiring"):
			return "recorded, not measured"
		case strings.Contains(printed, "nothing unlanded"),
			strings.Contains(printed, "nothing here is unlanded"),
			strings.Contains(printed, "had nothing to land"):
			return "settled"
		}
		return "unlanded work"
	}
	// The sweep is the REAL command and not a reproduction of its printer
	// switch: `posse worktrees --land` is what an operator runs beside the
	// listing, and a copy of its switch here would keep agreeing with a
	// listing after the real one had stopped. It carries the same three
	// answers through one more surface than MergeSessionWork alone —
	// unaccountedFor's refusal, which a tree with no bead record gets
	// instead of a merge, and which says them in the same words.
	land := func(t *testing.T, a *App, repo string) string {
		t.Helper()
		var out strings.Builder
		if err := LandSessionTrees(&out, a, []string{repo}, false); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}

	cases := []struct {
		name string
		// setUp leaves the fixture in the state an operator would list.
		setUp func(t *testing.T, repo string, tr *SessionTree)
		// want is the verdict BOTH surfaces owe about it.
		want string
		// says is what the listing must spell out beyond the verdict, and
		// unsays what it must not: the off-branch clause is a claim about
		// WHERE the work sits, and printing it over a tree whose HEAD is on
		// its own branch sends the operator to run `branch -f` for nothing.
		says   []string
		unsays []string
	}{{
		name: "an already-landed duplicate is settled on both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s: the fix")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			// The base moves first: a pick onto an unmoved base rebuilds the
			// identical commit object and the base reaches it by SHA, which
			// measures nothing here (ranger-base-g2xf's fixture).
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			mustGit(t, repo, "cherry-pick", "-x", sha)
		},
		want:   "settled",
		says:   []string{"equivalent patch on main"},
		unsays: []string{"detached HEAD"},
	}, {
		name: "a real strand is unlanded on both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s: the fix")
		},
		want:   "unlanded work",
		says:   []string{"1 commit(s) not on main"},
		unsays: []string{"detached HEAD"},
	}, {
		// The middle verdict, and the arm that pins landed()'s threshold at
		// `len(eq) > 0` rather than measuredOnBase: the branch never moved,
		// so the equivalence question is asked from landed() and nowhere
		// else, and the evidence is a human's `-x` trailer. Refusing here
		// would print "a retire would lose it" over work the operator's own
		// record says they landed; calling it settled would assert a
		// measurement nobody took. Both surfaces owe the third answer.
		name: "a hand-resolved pick is recorded but not measured on both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s: the fix")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			// The trailer without the patch: what a pick whose conflict a
			// human resolved leaves behind.
			commitIn(t, repo, "adr.md", "status: accepted (amended)\n",
				"s: the fix\n\n(cherry picked from commit "+sha+")")
		},
		want: "recorded, not measured",
		says: []string{"recorded as landed in", "compare before retiring"},
	}, {
		// The one arm where the two facts pull opposite ways, and the reason
		// the off-branch clause hangs off the UNLANDED arms only: the work is
		// on no branch, and it is also on the base by measurement. Nothing
		// here can be lost by a ref, so saying `branch -f` first would send
		// the operator to rescue what the base already holds.
		//
		// It is also the arm that found landed() still answering by ancestry
		// alone: the branch never moved, so MergeSessionWork returns through
		// landed() without reaching either of the two equivalence questions
		// it asks later, and landed() reported the work "unreferenced" while
		// the listing beside it called the same tree settled.
		name: "a detached tree whose work is already on the base is settled anyway",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s: the fix, off its branch")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			mustGit(t, repo, "cherry-pick", "-x", sha)
		},
		want:   "settled",
		says:   []string{"equivalent patch on main"},
		unsays: []string{"detached HEAD"},
	}, {
		name: "a caged session's detached work is unlanded on both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			// What PrepareSessionHead does at the container tier, and the
			// record it leaves so the merge knows to splice.
			mustGit(t, repo, "config", detachedKey(tr.Branch), "1")
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s: the caged work")
		},
		want: "unlanded work",
		says: []string{"detached HEAD", "branch -f " + SessionBranch("s-1")},
	}, {
		name: "a detached tree nothing will splice is unlanded on both",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			// No stamp: MergeSessionWork does not splice, and landed()
			// refuses with the sentence this listing now shares.
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s: off its own branch")
		},
		want: "unlanded work",
		says: []string{"detached HEAD", "branch -f " + SessionBranch("s-1")},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := wtApp(t)
			repo := wtRepo(t)
			commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			c.setUp(t, repo, tr)

			// The listing is read BEFORE the sweep in every arm, because the
			// sweep ACTS: it lands the strand and splices the caged tree, and
			// a listing read after would describe a tree the assertion just
			// changed.
			var out strings.Builder
			if err := ListSessionTrees(&out, []string{repo}); err != nil {
				t.Fatal(err)
			}
			listing := out.String()
			sweep := land(t, a, repo)

			if got := verdict(listing); got != c.want {
				t.Errorf("the listing's verdict is %q, want %q:\n%s", got, c.want, listing)
			}
			if got := verdict(sweep); got != c.want {
				t.Errorf("the sweep's verdict is %q, want %q:\n%s", got, c.want, sweep)
			}
			// The agreement itself, stated once. Redundant with the two
			// above while both hold, and the one that survives a future
			// change that moves BOTH surfaces off this fixture's verdict
			// together — which would still be one binary with one answer.
			if verdict(listing) != verdict(sweep) {
				t.Errorf("one binary, two answers about one tree — listing %q says %q, sweep %q says %q",
					strings.TrimSpace(listing), verdict(listing), sweep, verdict(sweep))
			}
			for _, want := range c.says {
				if !strings.Contains(listing, want) {
					t.Errorf("the listing does not say %q:\n%s", want, listing)
				}
			}
			for _, unwant := range c.unsays {
				if strings.Contains(listing, unwant) {
					t.Errorf("the listing should not say %q:\n%s", unwant, listing)
				}
			}
		})
	}
}

// The listing PRESCRIBES a command, and until now nothing ran it: the arms
// above assert the sentence CONTAINS `branch -f <branch>`, which a
// prescription naming the wrong directory satisfies just as well. That one
// is not hypothetical — `git -C <repo> branch -f <branch> HEAD` typed at the
// main checkout instead of the tree moves the branch to the BASE's tip and
// throws away the very commits the line exists to rescue, and it reads
// correctly in the listing.
//
// So this arm takes the command out of the printed sentence the way an
// operator does — copy, paste, run — and then asks the two questions the
// string match cannot: did the branch end up at the WORK, and does the
// listing stop asking for the rescue once it has been done.
// (ranger-base-cakl7, verifying ranger-base-d8o6)
func TestTheOffBranchPrescriptionIsRunnableAndRescuesTheWork(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "adr.md", "status: proposed\n", "seed the adr")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	// A caged session's shape: detached on purpose, its commits on no ref.
	mustGit(t, repo, "config", detachedKey(tr.Branch), "1")
	mustGit(t, tr.Path, "checkout", "-q", "--detach")
	commitIn(t, tr.Path, "adr.md", "status: accepted\n", "s: the caged work")
	work := mustGit(t, tr.Path, "rev-parse", "HEAD")

	var out strings.Builder
	if err := ListSessionTrees(&out, []string{repo}); err != nil {
		t.Fatal(err)
	}
	listing := out.String()
	m := regexp.MustCompile("`(git -C [^`]+)`").FindStringSubmatch(listing)
	if m == nil {
		t.Fatalf("the listing prescribes no command to run:\n%s", listing)
	}
	// Split the way a shell would. The fixture's paths carry no spaces, and
	// a session tree's name never does either (SessionForBead).
	argv := strings.Fields(m[1])
	if b, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("the prescription the listing printed does not run: %v\n%s\nline: %s", err, b, m[1])
	}

	if got := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); got != work {
		t.Errorf("after the prescription %s is at %s, want the work at %s — the line ran and rescued nothing",
			tr.Branch, got, work)
	}
	var after strings.Builder
	if err := ListSessionTrees(&after, []string{repo}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(after.String(), "detached HEAD") {
		t.Errorf("the listing still asks for a rescue that has been done:\n%s", after.String())
	}
	if !strings.Contains(after.String(), "1 commit(s) not on main") {
		t.Errorf("the work is on the branch and unlanded, and the listing owes that count:\n%s", after.String())
	}
}

// ─── the detached head a caged session works on (ranger-base-t4f1) ──────────

// The whole round trip, and the one arm that says the mechanism is worth
// having: a caged launch detaches, the persona commits with NO ref write at
// all (which is what lets the container tier mount the git common dir `:ro`),
// and the close splices the work back onto the branch and lands it.
//
// The ref-write claim is measured here rather than asserted, from the branch
// ref itself: after the commit the branch is still exactly where it was cut,
// so nothing under `<common>/refs` was touched to make that commit — the fact
// sessionCommonDirWrites rests on.
func TestCagedWorktreeSessionCommitsDetachedAndTheCloseSplicesItBack(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	cutAt := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch)
	// `logs/` TAKEN AWAY first, or the assertion below measures the fixture:
	// wtRepo commits before the worktree is cut, so git has already written a
	// reflog there and the launcher's mkdir would be a no-op that a REMOVED
	// mkdir passes just as well (measured — the mutant survived until this
	// line was added).
	own := LinkedGitDirs(tr.Path)
	if len(own) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", own)
	}
	if err := os.RemoveAll(filepath.Join(own[1], "logs")); err != nil {
		t.Fatal(err)
	}

	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, err := git(tr.Path, "symbolic-ref", "--quiet", "HEAD"); err == nil && b != "" {
		t.Fatalf("a caged session must launch on a detached HEAD, got %s", b)
	}
	if !launchedDetached(tr.Repo, tr.Branch) {
		t.Fatal("the detach was not recorded, so the close would read it as the accidental case")
	}
	// The launcher's mkdir: a read-write overlay of an absent source is
	// dropped by cageOverlay, and the reflog write of the first commit would
	// then be refused on the `:ro` common dir.
	if st, err := os.Stat(filepath.Join(own[1], "logs")); err != nil || !st.IsDir() {
		t.Fatalf("the launcher must make <common>/logs before a caged launch: %v", err)
	}

	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != cutAt {
		t.Fatalf("a detached commit moved %s (%s → %s) — the narrowed mount grants no ref write, so this shape would not commit inside the cage", tr.Branch, cutAt[:12], now[:12])
	}

	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || o.Commits != 1 {
		t.Fatalf("the splice did not put the work where the merge could take it: %+v", o)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "fix.txt")); err != nil || string(body) != "the work\n" {
		t.Errorf("a caged session's committed work is not on the repo's branch: %v", err)
	}
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != head {
		t.Errorf("the splice did not move %s onto the tree's HEAD: %s vs %s", tr.Branch, now[:12], head[:12])
	}
}

// The launcher's OTHER create, beside `logs/` (ADR 0038 decision 4b,
// ranger-base-p9h9d): `config.worktree`, which the container tier's `:ro`
// file bind of the identity chain needs a SOURCE for and which no live
// worktree carries — posse never sets `extensions.worktreeConfig` and
// `git worktree add` writes no such file. Absent, cageOverlayFile's Stat
// drops the bind (the arm is TestAbsentWorktreeConfigIsNeitherBoundNorCreated
// in cageoverlay_test.go) and the path stays creatable by the session under
// the read-write `worktrees/<own>` overlay — in the repo where the operator
// DID turn the extension on, that is the wall gone. The alternative was to
// key the wall on a config key instead of making the file; a wall
// conditional on a setting is a wall that reads a different repo from the
// one beside it.
//
// The create has to be INERT as well as present, or the launcher has
// changed what git reads in the operator's tree in order to build itself a
// mountpoint. Measured both ways rather than asserted, because "empty" is
// only inert if git agrees: with the extension off git never reads the
// file, with it on an empty one carries no keys, and the hooks slot is
// still the common dir's in both.
func TestACagedLaunchMakesTheWorktreeConfigItsBindNeeds(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	own := LinkedGitDirs(tr.Path)
	if len(own) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", own)
	}
	cw := filepath.Join(own[0], "config.worktree")
	// The premise, so a git that started writing the file itself turns this
	// pin into a no-op loudly rather than quietly.
	if _, err := os.Stat(cw); err == nil {
		t.Fatalf("`git worktree add` already wrote %s, so the create this pins measures nothing", cw)
	}

	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(cw)
	if err != nil {
		t.Fatalf("a caged launch must make %s — without it the :ro bind is dropped for want of a source: %v", cw, err)
	}
	if st.Size() != 0 {
		t.Errorf("the file the launcher makes carries no keys, got %d bytes", st.Size())
	}

	// Inert in both directions, in the tree itself.
	for _, ext := range []string{"off", "on"} {
		if ext == "on" {
			mustGit(t, repo, "config", "extensions.worktreeConfig", "true")
		}
		mustGit(t, tr.Path, "status", "--short")
		mustGit(t, tr.Path, "rev-parse", "HEAD")
		if h := mustGit(t, tr.Path, "rev-parse", "--git-path", "hooks"); !underDir(own[1], h) {
			t.Errorf("extensions.worktreeConfig %s: the hooks slot moved out of the common dir to %s", ext, h)
		}
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")

	// And a second launch does not TRUNCATE what is there. An operator who
	// turned the extension on and set a key in this file would lose it, and
	// posse deleting the operator's own per-worktree config is a worse bug
	// than the one the create closes.
	body := "[core]\n\tsparseCheckout = false\n"
	if err := os.WriteFile(cw, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(cw); err != nil || string(got) != body {
		t.Errorf("the relaunch rewrote the operator's %s: %q (%v)", cw, got, err)
	}
}

// The splice runs for a session posse detached ON PURPOSE and for no other,
// which is what keeps the ranger-base-dybv guard catching the accidental
// case. Same fixture, same detached HEAD, one bit different — and the two
// arms have to disagree, or the guard has been silenced for every session.
func TestOnlyARecordedDetachSplices(t *testing.T) {
	t.Parallel()
	for _, recorded := range []bool{true, false} {
		t.Run(fmt.Sprintf("recorded=%v", recorded), func(t *testing.T) {
			a := wtApp(t)
			repo := wtRepo(t)
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			if recorded {
				if err := recordDetached(tr.Repo, tr.Branch, true); err != nil {
					t.Fatal(err)
				}
			}
			commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")

			o, err := MergeSessionWork(tr)
			if err != nil {
				t.Fatal(err)
			}
			if recorded {
				if !o.Merged {
					t.Fatalf("a designed detach must be spliced and landed: %+v", o)
				}
				return
			}
			if o.Merged {
				t.Fatalf("an ACCIDENTAL detach was landed — the dybv guard is gone: %+v", o)
			}
			if !strings.Contains(o.Reason, "the tree's HEAD is off its own branch") {
				t.Errorf("the accidental case must still be reported in its own words: %q", o.Reason)
			}
		})
	}
}

// The tier is a property of the LAUNCH and the tree outlives it. A PID that
// drops `cage: container`, or a `--cage seatbelt` relaunch, must not inherit
// a detached HEAD forever — that would take the dybv guard's sensitivity with
// it for the life of the branch.
func TestAnUncagedLaunchPutsADetachedTreeBackOnItsBranch(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")

	if err := PrepareSessionHead(tr, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := git(tr.Path, "symbolic-ref", "--short", "--quiet", "HEAD"); b != tr.Branch {
		t.Fatalf("an uncaged launch must put the tree back on %s, HEAD is on %q", tr.Branch, b)
	}
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != head {
		t.Errorf("the re-attach lost the detached commits: %s vs %s", now[:12], head[:12])
	}
	if launchedDetached(tr.Repo, tr.Branch) {
		t.Error("the record still says detached, so a later close would splice for a session posse never detached")
	}
	// And the guard is live again on this branch: an accidental detach from
	// here is reported, not landed.
	mustGit(t, tr.Path, "checkout", "-q", "--detach")
	commitIn(t, tr.Path, "again.txt", "more\n", "s-1: more")
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Errorf("the dybv guard did not come back with the branch: %+v", o)
	}
}

// A second caged launch into the same tree moves the branch up to the work
// FIRST. Between two caged sessions a kill can retire the tree, and commits
// of the first that no ref names are exactly what such a retire destroys.
func TestARelaunchIntoADetachedTreeSplicesBeforeItRuns(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")

	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != head {
		t.Errorf("the relaunch left the first session's commit on no branch: %s vs %s", now[:12], head[:12])
	}
	if b, err := git(tr.Path, "symbolic-ref", "--quiet", "HEAD"); err == nil && b != "" {
		t.Errorf("the relaunch re-attached a caged session's tree to %s", b)
	}
}

// `branch -f` is a ref write with no ancestry check, so a branch tip the
// tree's HEAD does not reach is work this would DELETE. Nothing in posse's
// own paths can produce that pair — the branch of a detached tree moves only
// through the splice — so the refusal is about what posse does not control:
// an operator's own `branch -f`, or a stale record on a reused branch.
func TestTheSpliceRefusesToRewindABranch(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "s-1: the fix")
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")
	// The branch taken somewhere the tree's HEAD does not reach. git allows
	// it precisely BECAUSE the tree is detached — a branch a worktree has
	// checked out is refused, and a detached tree has none, which is the
	// hole this refusal stands in.
	commitIn(t, repo, "elsewhere.txt", "somebody else\n", "main: elsewhere")
	mustGit(t, repo, "branch", "-f", tr.Branch, "main")
	moved := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch)
	if moved == head {
		t.Fatalf("the fixture did not move the branch off the tree's HEAD (%s)", moved[:12])
	}

	if err := spliceDetachedWork(tr); err == nil {
		t.Fatal("the splice overwrote a branch the tree's HEAD does not reach")
	}
	if now := mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch); now != moved {
		t.Errorf("the refused splice moved the branch anyway: %s vs %s", now[:12], moved[:12])
	}
}

// A DETACHED tree is one of ours. `worktree list --porcelain` prints
// `detached` where it would have printed `branch refs/heads/…`, so keying on
// that line alone dropped every container-tier session out of the landing
// sweep, `posse worktrees` and the merge — over exactly the sessions whose
// work is in the tree and not on the branch.
func TestTheSweepSeesADetachedSessionTree(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareSessionHead(tr, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")

	trees, err := SessionTreesIn([]string{repo})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, got := range trees {
		if resolveExisting(got.Path) == resolveExisting(tr.Path) {
			found = true
			if got.Branch != tr.Branch {
				t.Errorf("the sweep named the wrong branch for a detached tree: %q, want %q", got.Branch, tr.Branch)
			}
		}
	}
	if !found {
		t.Fatalf("a detached session tree is invisible to the sweep that exists to find stranded work: %+v", trees)
	}
	// The main checkout is in that listing too and must never be claimed as
	// a session tree, whatever its HEAD is doing. The branch it would be
	// claimed UNDER has to exist for this arm to measure anything — without
	// it branchExists refuses on its own and the guard is untested.
	mustGit(t, repo, "branch", SessionBranch(filepath.Base(repo)), "main")
	mustGit(t, repo, "checkout", "-q", "--detach")
	trees, err = SessionTreesIn([]string{repo})
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range trees {
		if resolveExisting(got.Path) == resolveExisting(repo) {
			t.Errorf("the main checkout was claimed as a session tree: %+v", got)
		}
	}
}
