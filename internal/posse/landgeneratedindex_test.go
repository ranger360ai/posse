//go:build posse_arm2

package posse

// The landing reproduces a generated file instead of replaying it
// (ranger-base-7h8k4). generatedindex.go's header has the measurement; this
// is the pin the bead asked for, in its own words: a branch with an unindexed
// fragment lands with the index current, and two such branches land back to
// back without a conflict.
//
// It runs the REAL generator — scripts/notes-index.py, copied into the
// fixture repo and committed there — because the thing being pinned is that
// what lands satisfies the tree pin on main
// (internal/treepins' TestNotesFragmentIndexIsCurrent), and that pin runs
// this program. A stub generator would pin the plumbing and nothing about the
// file main ends up holding.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// notesGenerator is the real script's bytes. Read as ONE file at the repo
// root, which is what ~48 test files here already do; it walks nothing and
// computes no root to walk (treewidedoor_qa_test.go's class).
func notesGenerator(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", notesIndexGenerator))
	if err != nil {
		t.Fatalf("the generator this landing runs is not in the tree: %v", err)
	}
	return body
}

// runNotesGenerator runs the generator the way a seat does: from the tree it
// belongs to, with no --directory, so the fixture also measures that the
// script's own default is the directory beside it.
func runNotesGenerator(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := exec.Command("python3", append([]string{filepath.Join(dir, notesIndexGenerator)}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	// Logged only for a WRITE: a failing `--check` is this program answering
	// the question, not a program that would not run, and the fixtures below
	// ask it over a deliberately stale index.
	if err != nil && len(args) == 0 && len(out) > 0 {
		t.Logf("generator: %s", out)
	}
	return err
}

// indexIsCurrent asks the generator's own --check, which is the question the
// tree pin on main asks.
func indexIsCurrent(t *testing.T, dir string) bool {
	t.Helper()
	return runNotesGenerator(t, dir, "--check") == nil
}

// notesRepo is a repo that ships the generator and holds one indexed
// fragment on main — the state every landing below starts from.
func notesRepo(t *testing.T) (*App, string) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	write(t, filepath.Join(repo, notesIndexGenerator), string(notesGenerator(t)))
	mustGit(t, repo, "add", "--", notesIndexGenerator)
	mustGit(t, repo, "commit", "-q", "-m", "ship the generator", "--", notesIndexGenerator)
	commitIn(t, repo, filepath.Join(notesFragmentDir, "ranger-base-aaaa.md"), "# the fragment main already has\n\n2026-10-01\n", "main: a fragment")
	if err := runNotesGenerator(t, repo); err != nil {
		t.Fatalf("fixture: the generator would not run: %v", err)
	}
	mustGit(t, repo, "add", "--", notesIndexPath)
	mustGit(t, repo, "commit", "-q", "-m", "index the fragment", "--", notesIndexPath)
	if !indexIsCurrent(t, repo) {
		t.Fatal("fixture: main's index is stale before anything has landed")
	}
	return a, repo
}

// fragmentBranch is a seat that writes a fragment. indexed says whether it
// remembered to run the generator — the forgetful half is what put three
// stale indexes on main in one day, and the diligent half is what made five
// branches conflict with each other.
func fragmentBranch(t *testing.T, a *App, repo, session, bead, title string, indexed bool) *SessionTree {
	t.Helper()
	tr, err := a.EnsureSessionTree(repo, session, nil)
	if err != nil || tr == nil {
		t.Fatalf("session tree for %s: %v", session, err)
	}
	tr.Bead = bead
	commitIn(t, tr.Path, filepath.Join(notesFragmentDir, bead+".md"),
		"# "+title+"\n\n2026-10-02\n", bead+": a notes fragment")
	if indexed {
		if err := runNotesGenerator(t, tr.Path); err != nil {
			t.Fatalf("fixture: the generator would not run in %s: %v", tr.Path, err)
		}
		mustGit(t, tr.Path, "add", "--", notesIndexPath)
		mustGit(t, tr.Path, "commit", "-q", "-m", bead+": index it", "--", notesIndexPath)
		if !indexIsCurrent(t, tr.Path) {
			t.Fatalf("fixture: %s ran the generator and the index is still stale", session)
		}
	} else if indexIsCurrent(t, tr.Path) {
		t.Fatalf("fixture: %s wrote a fragment and the index is current anyway — the forgetful half of this pin is unreachable", session)
	}
	return tr
}

// landedIndexNames is the measurement main's readers make: the index on main
// names every fragment that reached it.
func landedIndexNames(t *testing.T, repo string, beads ...string) {
	t.Helper()
	if !indexIsCurrent(t, repo) {
		t.Errorf("%s is stale on main after the landing — this is the tree pin reading red, which is the defect", notesIndexPath)
	}
	body, err := os.ReadFile(filepath.Join(repo, notesIndexPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range beads {
		if !strings.Contains(string(body), b) {
			t.Errorf("the index on main does not name %s:\n%s", b, body)
		}
	}
}

// THE PIN, HALF ONE: a branch with an unindexed fragment lands with the index
// current. This is ranger-base-rb05v, -knux2 and -4ch00 — a plain
// fast-forward, no rebase, nothing in anyone's way.
func TestLandingRegeneratesAnUnindexedFragmentsIndex(t *testing.T) {
	t.Parallel()
	a, repo := notesRepo(t)
	tr := fragmentBranch(t, a, repo, "s-1", "ranger-base-bbbb", "the seat's own fragment", false)

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged {
		t.Fatalf("the branch did not land: %s", o.Reason)
	}
	if o.Rebased {
		t.Fatal("fixture: main never moved, so this arm must be the plain fast-forward")
	}
	if got := o.Regenerated; len(got) != 1 || got[0] != notesIndexPath {
		t.Errorf("the outcome does not report reproducing the index: %v", got)
	}
	if o.Commits != 2 {
		t.Errorf("the landing reports %d commit(s), want 2 — the seat's fragment and the index the launcher wrote", o.Commits)
	}
	landedIndexNames(t, repo, "ranger-base-aaaa", "ranger-base-bbbb")

	// The commit is traceable to what occasioned it: a reader of `git log` on
	// main finds a commit nobody in the session typed, and the bead id is how
	// this shop answers "why is this here" (AGENTS.md).
	msg := mustGit(t, repo, "log", "-1", "--pretty=%B")
	for _, want := range []string{"landing: regenerate " + notesIndexPath, "ranger-base-bbbb", notesIndexCommand} {
		if !strings.Contains(msg, want) {
			t.Errorf("the landing commit's message does not say %q:\n%s", want, msg)
		}
	}
	if only := mustGit(t, repo, "show", "--name-only", "--pretty=format:", "HEAD"); strings.TrimSpace(only) != notesIndexPath {
		t.Errorf("the landing commit touches %q, want %s alone", strings.TrimSpace(only), notesIndexPath)
	}
	// And it is on the BRANCH, not a commit the launcher put straight on the
	// operator's checkout: what main took is the branch's tip.
	if mustGit(t, repo, "rev-parse", tr.Branch) != mustGit(t, repo, "rev-parse", "main") {
		t.Error("main and the branch are not the same commit — the index did not land as part of the fast-forward")
	}
}

// THE PIN, HALF TWO: two such branches land back to back without a conflict.
// Both halves of the day's measurement in one test — the first branch lands
// by fast-forward, and the second has to be REPLAYED onto it.
func TestTwoFragmentBranchesLandBackToBack(t *testing.T) {
	t.Parallel()
	for _, indexed := range []bool{false, true} {
		name := "neither ran the generator"
		if indexed {
			name = "both ran the generator"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, repo := notesRepo(t)
			// Two seats, both already committed, before either lands: the
			// shape that matters is the SECOND landing, whose branch was cut
			// from a main that no longer exists.
			one := fragmentBranch(t, a, repo, "s-1", "ranger-base-cccc", "the first seat's fragment", indexed)
			two := fragmentBranch(t, a, repo, "s-2", "ranger-base-dddd", "the second seat's fragment", indexed)

			first, err := MergeSessionWork(a, one)
			if err != nil {
				t.Fatal(err)
			}
			if !first.Merged {
				t.Fatalf("the first branch did not land: %s", first.Reason)
			}

			// THE POSITIVE WITNESS, and without it this test is payable by a
			// fixture that never conflicted. git's own answer, by running the
			// replay the landing is about to run and aborting it: the diligent
			// half MUST stop on a conflict — that is the five branches landed
			// by hand — and the forgetful half must not, or it is measuring
			// the replay arm over a branch that had no conflict in it.
			//
			// MEASURED 2026-10-03, git 2.50.1: mergesCleanly is NOT the
			// instrument for this. It answers true for the diligent pair,
			// because `merge-tree` merges the two tips in one go and the two
			// index insertions merge; the commit-by-commit REPLAY of the same
			// branch conflicts. That is the divergence mergesCleanly's own
			// header states ("a filter and not an authority"), here as a
			// reading rather than a caveat.
			// Undone either way, so the landing under test starts from the
			// branch the seat actually committed: an aborted replay is git's
			// own undo, and a replay that SUCCEEDED has moved the branch, so
			// the probe puts it back where it was.
			tip := mustGit(t, two.Path, "rev-parse", "HEAD")
			_, _ = git(two.Path, "rebase", "main")
			stopped := rebaseStopped(two.Path)
			if stopped {
				mustGit(t, two.Path, "rebase", "--abort")
			} else {
				mustGit(t, two.Path, "reset", "--hard", tip)
			}
			if now := mustGit(t, two.Path, "rev-parse", "HEAD"); now != tip {
				t.Fatalf("the witness probe moved the branch under the test (%s → %s)", tip, now)
			}
			if stopped != indexed {
				t.Fatalf("fixture: replaying %s onto main stopped=%v with indexed=%v — this arm is not the shape it says it is", two.Branch, stopped, indexed)
			}

			second, err := MergeSessionWork(a, two)
			if err != nil {
				t.Fatal(err)
			}
			if !second.Merged {
				t.Fatalf("the second branch did not land: %s — this is the conflict five branches were landed by hand for", second.Reason)
			}
			if !second.Rebased {
				t.Fatal("fixture: the second landing did not replay, so it never met the first one's index")
			}
			landedIndexNames(t, repo, "ranger-base-aaaa", "ranger-base-cccc", "ranger-base-dddd")
			if got := second.Regenerated; len(got) != 1 || got[0] != notesIndexPath {
				t.Errorf("the second landing does not report reproducing the index: %v", got)
			}
			// Nothing of the replay is left behind: a rebase that had to be
			// resolved must not leave the tree mid-rebase (AGENTS.md's
			// stranded-sequencer reading is about a worktree posse hands back).
			if rebaseStopped(two.Path) {
				t.Error("the session tree is still mid-rebase after a landing that reported success")
			}
			if left := strings.TrimSpace(mustGit(t, two.Path, "status", "--porcelain")); left != "" {
				t.Errorf("the landing left the session tree dirty:\n%s", left)
			}
		})
	}
}

// A seat that ran the generator itself is left alone: no landing commit, and
// nothing written into its tree. The write matters on its own — a session
// tree's mtime is ADR 0058 fact 4, and a landing that touched every tree it
// looked at would keep the grace clock from reaching zero.
func TestLandingWritesNothingWhenThereIsNothingToReproduce(t *testing.T) {
	t.Parallel()
	a, repo := notesRepo(t)
	for _, c := range []struct {
		name      string
		tree      func(*testing.T) *SessionTree
		wantAhead int
	}{
		{"the seat ran the generator", func(t *testing.T) *SessionTree {
			return fragmentBranch(t, a, repo, "s-indexed", "ranger-base-eeee", "an indexed fragment", true)
		}, 2},
		{"the branch touched no fragment at all", func(t *testing.T) *SessionTree {
			tr, err := a.EnsureSessionTree(repo, "s-elsewhere", nil)
			if err != nil || tr == nil {
				t.Fatalf("session tree: %v", err)
			}
			commitIn(t, tr.Path, "fix.txt", "the work\n", "ranger-base-ffff: the fix")
			return tr
		}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			tr := c.tree(t)
			before, err := os.Stat(filepath.Join(tr.Path, notesIndexPath))
			if err != nil {
				t.Fatal(err)
			}
			o, err := MergeSessionWork(a, tr)
			if err != nil {
				t.Fatal(err)
			}
			if !o.Merged {
				t.Fatalf("the branch did not land: %s", o.Reason)
			}
			if len(o.Regenerated) != 0 {
				t.Errorf("the landing reproduced %v over a branch with nothing to reproduce", o.Regenerated)
			}
			if o.Commits != c.wantAhead {
				t.Errorf("the landing reports %d commit(s), want %d — a commit was minted that nothing asked for", o.Commits, c.wantAhead)
			}
			after, err := os.Stat(filepath.Join(tr.Path, notesIndexPath))
			if err != nil {
				t.Fatal(err)
			}
			if !after.ModTime().Equal(before.ModTime()) {
				t.Errorf("%s was written in the session tree with nothing to reproduce — that mtime is ADR 0058 fact 4", notesIndexPath)
			}
		})
	}
}

// ADR 0041 §1–§2: the generated file is reproducible, and it is still the
// persona's uncommitted work. A landing that overwrote it would be destroying
// what a close-dirty handoff is in the middle of reporting.
func TestLandingWillNotOverwriteAnUncommittedIndex(t *testing.T) {
	t.Parallel()
	a, repo := notesRepo(t)
	tr := fragmentBranch(t, a, repo, "s-1", "ranger-base-gggg", "a fragment mid-edit", false)
	const mine = "# Notes fragments\n\nhalf-written by the persona\n"
	write(t, filepath.Join(tr.Path, notesIndexPath), mine)

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged {
		t.Fatalf("the branch did not land: %s", o.Reason)
	}
	if len(o.Regenerated) != 0 {
		t.Errorf("the landing reproduced %v over the persona's own uncommitted copy", o.Regenerated)
	}
	body, err := os.ReadFile(filepath.Join(tr.Path, notesIndexPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != mine {
		t.Errorf("the landing overwrote uncommitted work in the session tree:\n%s", body)
	}
	// And it is reported as the dirt it is, which is how the persona hears
	// that this part did not land (closeddirty.go).
	if len(o.Dirty) == 0 {
		t.Error("the uncommitted index is not in the dirt report, so nothing tells the persona it stayed behind")
	}
}

// THE PROGRAM IN HAND MUST BE THE TREE'S OWN, in bytes, or the landing runs
// nothing: the index that belongs on main is the one the tree's generator
// writes, and nothing unattended here may run a program off the branch it is
// landing or an operator's half-finished edit. Both ways the two can part,
// over a fixture whose generator would be LOUD if it ran.
func TestLandingWillNotRunAGeneratorThatIsNotTheTreesOwn(t *testing.T) {
	t.Parallel()
	const loud = "import sys\nsys.exit('a generator that was not the tree\\'s own ran')\n"
	for _, c := range []struct {
		name string
		part func(*testing.T, string, *SessionTree)
	}{
		{"the branch carries its own generator", func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, notesIndexGenerator, loud, "ranger-base-hhhh: teach the generator something")
		}},
		{"the operator is mid-edit on theirs", func(t *testing.T, repo string, tr *SessionTree) {
			write(t, filepath.Join(repo, notesIndexGenerator), loud)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, repo := notesRepo(t)
			tr := fragmentBranch(t, a, repo, "s-1", "ranger-base-hhhh", "a fragment", true)
			c.part(t, repo, tr)
			want, err := os.ReadFile(filepath.Join(tr.Path, notesIndexPath))
			if err != nil {
				t.Fatal(err)
			}

			o, err := MergeSessionWork(a, tr)
			if err != nil {
				t.Fatal(err)
			}
			if !o.Merged {
				t.Fatalf("the branch did not land: %s", o.Reason)
			}
			if len(o.Regenerated) != 0 {
				t.Errorf("the landing reproduced %v with a program that is not the tree's own", o.Regenerated)
			}
			got, err := os.ReadFile(filepath.Join(repo, notesIndexPath))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("main's index is not the one the branch committed:\n%s", got)
			}
		})
	}
}

// brokenGeneratorTree is the fixture for the one refusal this adds, driven as
// a case in mergeBlockedCases (mergeblocked_test.go) as well as by the test
// below: an index nothing can reproduce must not reach main as whatever the
// seat committed.
//
// Broken ON MAIN, and so in the session tree cut from it — the landing runs
// the base's copy and only runs it while the two are the same bytes, so a
// fixture that broke one side would be measuring the gate above instead. The
// staleness check inside fragmentBranch is answered by the broken program here
// rather than by a missing line; what this fixture is for is the fragment.
func brokenGeneratorTree(t *testing.T) (*App, *SessionTree) {
	t.Helper()
	a, repo := notesRepo(t)
	commitIn(t, repo, notesIndexGenerator, "import sys\nsys.exit('the generator is broken')\n",
		"main: break the generator")
	tr := fragmentBranch(t, a, repo, "s-1", "ranger-base-iiii", "a fragment", false)
	return a, tr
}

func TestLandingRefusesWhenTheGeneratorFails(t *testing.T) {
	t.Parallel()
	a, tr := brokenGeneratorTree(t)
	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatal("the landing proceeded over an index nothing could reproduce")
	}
	for _, want := range []string{"could not reproduce it", "the generator is broken", notesIndexCommand, "posse worktrees --land"} {
		if !strings.Contains(o.Reason, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, o.Reason)
		}
	}
	// Refused, not half-done: nothing is left in the tree for the next pass to
	// read as the persona's uncommitted work (ADR 0041 §1-§2).
	if left := strings.TrimSpace(mustGit(t, tr.Path, "status", "--porcelain")); left != "" {
		t.Errorf("a refused landing left the session tree dirty:\n%s", left)
	}
}

// THE HOOK DECISION, pinned because both halves of it are load-bearing and
// neither is visible in a passing landing (commitGeneratedIndex's header).
// `--no-verify` is bd's pre-commit flush and nothing else: the flush stages
// `.beads/issues.jsonl`, a path-limited commit does not take it, and what is
// left is a staged entry over a tree that already matches HEAD — which the
// next pass reads as the persona's uncommitted work. posse's own wall lives
// in prepare-commit-msg precisely so that `--no-verify` cannot dodge it.
func TestTheLandingCommitSkipsTheFlushHookAndNotTheWall(t *testing.T) {
	t.Parallel()
	a, repo := notesRepo(t)
	tr := fragmentBranch(t, a, repo, "s-1", "ranger-base-jjjj", "a fragment", false)

	hooks := mustGit(t, repo, "rev-parse", "--git-path", "hooks")
	if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(repo, hooks)
	}
	marks := t.TempDir()
	for _, h := range []string{"pre-commit", "prepare-commit-msg"} {
		p := filepath.Join(hooks, h)
		write(t, p, "#!/bin/sh\necho fired > "+shq(filepath.Join(marks, h))+"\nexit 0\n")
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || len(o.Regenerated) == 0 {
		t.Fatalf("fixture: nothing was committed, so no hook was in the path (%v, %s)", o.Regenerated, o.Reason)
	}
	if _, err := os.Stat(filepath.Join(marks, "pre-commit")); err == nil {
		t.Error("the landing commit ran pre-commit — bd's flush stages the beads db, and a path-limited commit leaves it staged over a clean tree (AGENTS.md, rangerhq-be7k)")
	}
	if _, err := os.Stat(filepath.Join(marks, "prepare-commit-msg")); err != nil {
		t.Error("the landing commit did not run prepare-commit-msg — that is where posse's commit wall lives, and `--no-verify` must not have dodged it")
	}
}

// halfWritingGenerator is the generator failure the shipped pin above cannot
// have: it takes the real program's `--directory` contract, answers `--check`
// the way a stale index does, then WRITES the file and dies. The fixture
// brokenGeneratorTree uses writes nothing at all, so its "nothing is left in
// the tree" assertion passed over the restore and over its absence alike
// (ranger-base-jqe3b). Production reaches this shape through the kill at
// generatorTimeout — `Path.write_text` truncates on open, so a kill between
// the truncate and the close leaves a short file — and through any write
// error after that truncate, ENOSPC being the ordinary one.
const halfWritingGenerator = "import sys\n" +
	"import argparse\n" +
	"from pathlib import Path\n" +
	"p = argparse.ArgumentParser()\n" +
	"p.add_argument('--check', action='store_true')\n" +
	"p.add_argument('--directory', type=Path)\n" +
	"a = p.parse_args()\n" +
	"if a.check:\n" +
	"    sys.exit(1)\n" +
	"(a.directory / 'README.md').write_text('half an index\\n')\n" +
	"sys.exit('the generator is broken')\n"

// THE REFUSAL LEAVES NO DIRT, AND THE COST OF DIRT IS A STALE INDEX ON MAIN
// (ranger-base-jqe3b, escaped from ranger-base-7h8k4). Two passes over one
// session tree: the first refuses with the generator half-writing, the second
// runs a whole one over the same unindexed fragment. The second pass is the
// point — refreshGeneratedIndex abstains at `differsFromHEAD` on an
// uncommitted index, which is the right rule for a persona's edit and the
// wrong one for the launcher's own leftovers, so a refusal that left dirt
// turned the gate off and fast-forwarded a stale index onto main. That is the
// defect ranger-base-7h8k4 was filed to stop, produced by its own fix.
//
// Observed RED before the restore moved, in both halves: pass one left
// `M docs/notes.d/README.md`, and pass two came back merged=true
// regenerated=[] with main's index naming only the fragment main already had.
func TestLandingRefusalDirtLandsAStaleIndex(t *testing.T) {
	t.Parallel()
	a, repo := notesRepo(t)
	real := string(notesGenerator(t))
	commitIn(t, repo, notesIndexGenerator, halfWritingGenerator,
		"main: a generator that writes and then dies")
	tr := fragmentBranch(t, a, repo, "s-1", "ranger-base-pppp", "a fragment", false)

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatal("pass one landed over an index nothing reproduced")
	}
	if left := strings.TrimSpace(mustGit(t, tr.Path, "status", "--porcelain", "--untracked-files=all")); left != "" {
		t.Errorf("a refused landing left the session tree dirty: %q", left)
	}

	// The generator is whole again in BOTH copies, so the licence to run it
	// holds (generatorIsTheTreesOwn). Nothing else about the branch changed.
	write(t, filepath.Join(repo, notesIndexGenerator), real)
	write(t, filepath.Join(tr.Path, notesIndexGenerator), real)

	o2, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o2.Merged {
		t.Fatalf("pass two refused a landing nothing is wrong with: %s", o2.Reason)
	}
	landedIndexNames(t, repo, "ranger-base-aaaa", "ranger-base-pppp")
}

// THE HALF `git restore` CANNOT EXPRESS: the generated file is not in HEAD at
// all, so putting it back the way the last commit has it means removing it
// (restoreGeneratedIndex's own measurement). Reached by a branch that brings
// the fragment directory whole — here, main with the index deleted — where the
// generator CREATES the file instead of rewriting one. An untracked leftover
// costs exactly what a modified one does: `differsFromHEAD` counts untracked
// (its own header says so), so the next pass abstains and lands stale.
func TestLandingRefusalRemovesAGeneratedFileHEADNeverHad(t *testing.T) {
	t.Parallel()
	a, repo := notesRepo(t)
	mustGit(t, repo, "rm", "-q", "--", notesIndexPath)
	mustGit(t, repo, "commit", "-q", "-m", "main: no committed index at all", "--", notesIndexPath)
	commitIn(t, repo, notesIndexGenerator, halfWritingGenerator,
		"main: a generator that writes and then dies")
	tr := fragmentBranch(t, a, repo, "s-1", "ranger-base-qqqq", "a fragment", false)
	if _, err := os.Stat(filepath.Join(tr.Path, notesIndexPath)); !os.IsNotExist(err) {
		t.Fatalf("fixture: the session tree has a committed index, so the generator would rewrite one (%v)", err)
	}

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatal("the landing proceeded over an index nothing could reproduce")
	}
	if left := strings.TrimSpace(mustGit(t, tr.Path, "status", "--porcelain", "--untracked-files=all")); left != "" {
		t.Errorf("a refused landing left the generator's new file in the tree: %q", left)
	}
}
