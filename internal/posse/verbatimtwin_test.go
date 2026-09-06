package posse

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ADR 0058's amendment of 2026-09-06, pinned: "the base holds the branch's
// bytes" is measured two ways and EITHER licenses, and the second one — a
// whitespace-EXACT patch-id twin (`git patch-id --verbatim`) — is what
// finally takes the row this whole record was filed about.
//
// Why a second instrument at all. The blob walk (contentNotOnBase,
// ranger-base-x8jp) asks whether the base ever held the branch's BLOB for
// each touched path. A tree only reaches the equivalence row because its
// landing was not a fast-forward, and a landing that is not a fast-forward
// writes a NEW blob for every file the base moved in the meantime — so for
// an append-heavy file the branch's blob is on the base NOWHERE, the tree is
// kept on every pass forever, and the corner ADR 0058 was written to close
// turned out to be empty rather than small (ranger-base-lwd29). MEASURED
// 2026-09-06 in ~/src/posse: olwk's commit and its landing 7ff3e4da share a
// --verbatim id while the blob walk loses CHANGELOG.md and INSTALL.md.
//
// THE WRONG ARMS ARE THE POINT, and they are named here so the next reader
// does not have to find them: "re-indented by hand" and "landed with a
// trailing space" are the two ranger-base-x8jp opened this guard for, and a
// twin measured with the --verbatim flag DROPPED calls both of them
// equivalent — `git patch-id` normalises whitespace, which is the entire
// defect. Mutate `--verbatim` away in patchIDsVerbatim and those two arms
// must go red (they retire a branch holding the last copy of its bytes);
// mutate the twin arm away entirely and arms 1 and 6 must go red (they are
// kept on every pass again). Measured both ways on the way in.
func TestRemoveSessionTreeRetiresOnAWhitespaceExactTwin(t *testing.T) {
	t.Parallel()

	// The append's three context lines are l8..l10, so an edit to line 1 is
	// OUTSIDE the hunk: the pick applies clean and writes a blob the base
	// has never held. That distance is the fixture.
	const ten = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	const tenAdded = ten + "ADDED by the session\n"
	const tenL1 = "L1 rewritten on main\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	const tenL1Added = tenL1 + "ADDED by the session\n"
	const tenL1L2Added = "L1 rewritten on main\nL2 also on main\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nADDED by the session\n"

	cases := []struct {
		name  string
		build func(t *testing.T, repo string, tr *SessionTree)
		// measured: every commit ahead is patch-id equivalent, so the guard
		// reaches the bytes question at all. blobLost: the cheap walk found
		// paths the base does not hold, so the TWIN is what answered.
		measured bool
		blobLost bool
		// "" means the tree and its branch must be gone; anything else is a
		// substring of the refusal the operator has to be able to read.
		kept string
	}{{
		// ARM 1 — the licensing arm, and the one this bead exists for. The
		// blob is on the base nowhere and the twin is exact, so nothing here
		// is the last copy of anything. Before the twin arm this retired
		// NOTHING: it is the `≡ … nothing here is unlanded` line, on every
		// pass, forever.
		name: "a clean pick the base moved the file outside the hunk for retires on the twin alone",
		build: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "f.txt", tenAdded, "s-1: append a line")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			commitIn(t, repo, "f.txt", tenL1, "main: rewrite line 1, outside the hunk")
			mustGit(t, repo, "cherry-pick", "-x", sha)
			// `git` trims, so the fixture's own newline is not part of the
			// comparison; what is being checked is that the pick applied
			// rather than conflicted.
			if got := mustGit(t, repo, "show", "main:f.txt"); got != strings.TrimRight(tenL1Added, "\n") {
				t.Fatalf("the pick did not land as the fixture assumes:\n%q", got)
			}
		},
		measured: true, blobLost: true,
	}, {
		// ARM 6 — the same shape with the base then building ON TOP of the
		// landing. The twin has to survive later edits or it answers only
		// for a base that stopped moving, which no base does.
		name: "a twin is still found under later edits the base made on top",
		build: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "f.txt", tenAdded, "s-1: append a line")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			commitIn(t, repo, "f.txt", tenL1, "main: rewrite line 1, outside the hunk")
			mustGit(t, repo, "cherry-pick", "-x", sha)
			commitIn(t, repo, "f.txt", tenL1L2Added, "main: built on top of the pick")
		},
		measured: true, blobLost: true,
	}, {
		// ARM 2 — THE WRONG ARM (ranger-base-x8jp's own shape). Plain
		// patch-id calls a re-indentation equivalent; --verbatim does not,
		// and the tab about to be deleted is the last copy of itself.
		name: "a hand landing that re-indented has no exact twin and is kept",
		build: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "f.go", "func f() {\n\treturn 1\n}\n", "s-1: the fix, indented with a TAB")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			commitIn(t, repo, "f.go", "func f() {\n    return 1\n}\n", "main: the same fix by hand, re-indented")
		},
		measured: true, blobLost: true,
		kept: "has no whitespace-exact twin",
	}, {
		// ARM 3 — THE OTHER WRONG ARM. Trailing whitespace is invisible to
		// plain patch-id for exactly the same reason and is a different
		// mutation of the same line, so neither arm can be the other's
		// duplicate.
		name: "a hand landing with trailing whitespace has no exact twin and is kept",
		build: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "f.go", "func f() {\n\treturn 1\n}\n", "s-1: the fix")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			commitIn(t, repo, "f.go", "func f() {\n\treturn 1 \n}\n", "main: the same fix by hand, with a trailing space")
		},
		measured: true, blobLost: true,
		kept: "has no whitespace-exact twin",
	}, {
		// ARM 4 — a control on the OTHER side of the widening: the same
		// line, but under a neighbour the landing changed. Plain patch-id
		// hashes context, so this never reached the bytes question and must
		// not start reaching it — it is kept by the count, in the count's
		// words.
		name: "the same line under a changed neighbour is unmeasured and kept as a strand",
		build: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "f.go", "func f() {\n\treturn 1\n}\n", "s-1: the fix")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			commitIn(t, repo, "f.go", "func f() { // reworded on main\n\treturn 1\n}\n", "main: the same line, a changed neighbour")
		},
		kept: "commit(s) not on main",
	}, {
		// ARM 5 — a landing that DROPPED one of the commit's two hunks. The
		// half that is gone is the whole reason this guard exists, and it
		// must never reach a twin lookup that would compare only what
		// landed.
		name: "a landing that dropped a hunk is unmeasured and kept as a strand",
		build: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "f.txt", "X1 by the session\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nY10 by the session\n", "s-1: two hunks")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			commitIn(t, repo, "f.txt", "X1 by the session\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n", "main: only the first hunk landed")
		},
		kept: "commit(s) not on main",
	}, {
		// ARM 7 — a SQUASH. Two commits ahead, one commit on the base
		// carrying both patches: neither branch commit has a twin of its own
		// and neither is patch-id equivalent, so the pairing never forms.
		// The twin is per-commit by construction and this pins that it is.
		name: "a squash of two commits pairs neither of them and is kept",
		build: func(t *testing.T, repo string, tr *SessionTree) {
			commitIn(t, tr.Path, "a.txt", "one\n", "s-1: the first")
			commitIn(t, tr.Path, "b.txt", "two\n", "s-1: the second")
			commitIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
			write(t, filepath.Join(repo, "a.txt"), "one\n")
			write(t, filepath.Join(repo, "b.txt"), "two\n")
			mustGit(t, repo, "add", "a.txt", "b.txt")
			mustGit(t, repo, "commit", "-q", "-m", "main: both, squashed into one", "--", "a.txt", "b.txt")
		},
		kept: "commit(s) not on main",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := wtApp(t)
			repo := wtRepo(t)
			commitIn(t, repo, "f.txt", ten, "seed the ten lines")
			commitIn(t, repo, "f.go", "func f() {\n\treturn 0\n}\n", "seed the go file")
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			c.build(t, repo, tr)

			// Ahead by SHA in every arm, or the guard under test is never
			// reached and every verdict below is about nothing
			// (ranger-base-as19's own fixture rule).
			if n := mustGit(t, repo, "rev-list", "--count", "main.."+tr.Branch); n == "0" {
				t.Fatalf("the fixture must leave %s ahead of main by sha", tr.Branch)
			}
			branchTip := mustGit(t, repo, "rev-parse", tr.Branch)

			// The fixture is only worth anything in the state it claims. A
			// git that stopped normalising whitespace, or a pick that landed
			// the identical blob after all, would leave these arms passing
			// for a reason that is not the one they are named for.
			eq := equivalentOnBase(repo, "main", tr.Branch)
			if measuredOnBase(eq) != c.measured {
				t.Fatalf("measuredOnBase = %v, want %v — this fixture is not the shape it says it is: %+v", measuredOnBase(eq), c.measured, eq)
			}
			if c.measured {
				lost, lerr := contentNotOnBase(repo, "main", tr.Branch)
				if lerr != nil {
					t.Fatal(lerr)
				}
				if (len(lost) > 0) != c.blobLost {
					t.Fatalf("contentNotOnBase = %v, want a non-empty answer=%v — the blob walk is not in the state this arm measures the twin against", lost, c.blobLost)
				}
			}

			// treeHolds first: RemoveSessionTree below ACTS, and the two are
			// one answer by contract (baseHoldsBytes is one function for
			// exactly that reason).
			held := treeHolds(tr)
			err = RemoveSessionTree(tr, false)

			if (err != nil) != (c.kept != "") {
				t.Fatalf("RemoveSessionTree = %v, want a refusal=%v", err, c.kept != "")
			}
			if (held != "") != (err != nil) {
				t.Errorf("one guard, two answers about one tree — treeHolds says %q, RemoveSessionTree says %v", held, err)
			}
			if c.kept != "" {
				if !strings.Contains(err.Error(), c.kept) {
					t.Errorf("the refusal must say %q, got: %v", c.kept, err)
				}
				if c.measured && !strings.Contains(err.Error(), abbrevSHA(branchTip)) {
					t.Errorf("the refusal must name the commit the base has no exact twin for (%s), got: %v", abbrevSHA(branchTip), err)
				}
				if _, serr := os.Stat(tr.Path); serr != nil {
					t.Errorf("the refused tree was removed anyway: %v", serr)
				}
				if !branchExists(repo, tr.Branch) {
					t.Error("the refused branch was deleted anyway")
				}
				return
			}
			if _, serr := os.Stat(tr.Path); serr == nil {
				t.Error("the worktree directory survived its removal")
			}
			if branchExists(repo, tr.Branch) {
				t.Error("the session branch survived its removal")
			}
		})
	}
}

// ARM 8, and it needs its own fixture because equivalentOnBase never lets a
// MERGE reach the twin lookup: `git cherry` skips merges, the trailer and
// replay arms then fail to account for one, and the tree is kept by the
// strand sentence before baseHoldsBytes is asked at all. That is the right
// answer today — and it is somebody else's guard, so the fail-closed branch
// inside verbatimUnpaired would be unmeasured if this pin went through
// RemoveSessionTree like the others.
//
// `git log -p` prints no patch for a merge, so the range form prints no id
// for it. An id lookup that read that absence as "nothing to compare, carry
// on" would license deleting a merge's own resolution — the one thing on a
// branch that exists nowhere else by construction.
func TestVerbatimTwinFailsClosedOnAMergeItCannotSee(t *testing.T) {
	t.Parallel()
	const ten = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	const tenL1 = "L1 rewritten on main\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"

	repo := wtRepo(t)
	commitIn(t, repo, "f.txt", ten, "seed the ten lines")
	commitIn(t, repo, "g.txt", "g0\n", "seed the side file")

	mustGit(t, repo, "checkout", "-q", "-b", "br")
	commitIn(t, repo, "f.txt", ten+"ADDED by the branch\n", "br: append a line")
	appended := mustGit(t, repo, "rev-parse", "HEAD")

	mustGit(t, repo, "checkout", "-q", "-b", "side", "main")
	commitIn(t, repo, "g.txt", "g1\n", "side: the other file")
	sideSHA := mustGit(t, repo, "rev-parse", "HEAD")

	mustGit(t, repo, "checkout", "-q", "br")
	// --no-ff so there is a merge COMMIT: the whole subject here.
	mustGit(t, repo, "merge", "--no-ff", "-m", "br: merge side", "side")

	mustGit(t, repo, "checkout", "-q", "main")
	commitIn(t, repo, "f.txt", tenL1, "main: rewrite line 1, outside the hunk")
	mustGit(t, repo, "cherry-pick", "-x", appended)
	mustGit(t, repo, "cherry-pick", "-x", sideSHA)

	// The fixture is the blind spot or it measures nothing: the base holds
	// both real commits' patches, the blob walk still refuses, and a merge
	// is standing in the range with no patch of its own.
	lost, err := contentNotOnBase(repo, "main", "br")
	if err != nil {
		t.Fatal(err)
	}
	if len(lost) == 0 {
		t.Fatalf("the blob walk must refuse here, or the twin arm is never reached")
	}
	merges := mustGit(t, repo, "rev-list", "--merges", "main..br")
	if len(strings.Fields(merges)) != 1 {
		t.Fatalf("rev-list --merges main..br = %q, want exactly the one merge this arm is about", merges)
	}
	mergeSHA := strings.Fields(merges)[0]
	ids, err := patchIDsVerbatim(repo, "main..br")
	if err != nil {
		t.Fatal(err)
	}
	if ids[mergeSHA] != "" {
		t.Fatalf("git printed a patch id for the merge %s; this fixture measures nothing", abbrevSHA(mergeSHA))
	}

	unpaired, err := verbatimUnpaired(repo, "main", "br")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(unpaired, abbrevSHA(mergeSHA)) {
		t.Errorf("verbatimUnpaired = %q, want it to name the merge %s it has no patch for", unpaired, abbrevSHA(mergeSHA))
	}
	if !strings.Contains(unpaired, "merge") {
		t.Errorf("verbatimUnpaired = %q, want it to say WHY there is no twin — a sha alone sends the operator to `git show` for it", unpaired)
	}
	held, gone, why, err := baseHoldsBytes(repo, "main", "br")
	if err != nil {
		t.Fatal(err)
	}
	if held {
		t.Errorf("the base was said to hold bytes it was never asked about: a merge's own resolution is on the branch and nowhere else")
	}
	if len(gone) == 0 || why == "" {
		t.Errorf("a keep has to say what it keeps for, got lost=%v unpaired=%q", gone, why)
	}

	// THE CONTROL: the same repo without the merge in the range. Every
	// commit ahead now has an exact twin, so the base does hold the bytes —
	// without this the arm above is satisfied by a helper that never says
	// yes to anything.
	mustGit(t, repo, "branch", "-f", "nomerge", appended)
	if lost, err := contentNotOnBase(repo, "main", "nomerge"); err != nil || len(lost) == 0 {
		t.Fatalf("the control must fail the blob walk too, got %v, %v", lost, err)
	}
	held, gone, why, err = baseHoldsBytes(repo, "main", "nomerge")
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Errorf("baseHoldsBytes refused a range whose every commit has a whitespace-exact twin: lost=%v unpaired=%q", gone, why)
	}
}

// THE GIT FLOOR, failing closed. `git patch-id --verbatim` is git 2.39+
// (and it cannot be combined with `--stable`), so on an older box the second
// instrument does not exist. What must NOT happen there is the retire going
// ahead on a blob walk that already said no — an unanswerable question about
// destroying work is answered no, the same rule RemoveSessionTree applies to
// a detached HEAD with no base.
//
// The fixture is arm 1, the one shape that retires TODAY and retires on the
// twin alone, run against a git that rejects the flag. Not parallel and not
// overlaid: the flag is passed to a binary found on PATH, so a shim on PATH
// is the only seam there is, and it execs the real git for every other
// argv — which is what keeps the rest of this test about the guard rather
// than about the shim.
func TestVerbatimTwinKeepsTheTreeOnAGitTooOldForTheFlag(t *testing.T) {
	const ten = "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	const tenL1 = "L1 rewritten on main\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"

	a := wtApp(t)
	repo := wtRepo(t)
	commitIn(t, repo, "f.txt", ten, "seed the ten lines")
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "f.txt", ten+"ADDED by the session\n", "s-1: append a line")
	sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
	commitIn(t, repo, "f.txt", tenL1, "main: rewrite line 1, outside the hunk")
	mustGit(t, repo, "cherry-pick", "-x", sha)

	// The control comes FIRST and on the real git: this exact tree retires,
	// so a keep below is the flag's absence and not the fixture's shape.
	// treeHolds does not act, so asking it costs the fixture nothing.
	if held := treeHolds(tr); held != "" {
		t.Fatalf("the fixture must be retirable on a git that has the flag, got %q", held)
	}

	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git on PATH")
	}
	shim := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do\n  if [ \"$a\" = \"--verbatim\" ]; then\n    echo \"error: unknown option \\`verbatim'\" >&2\n    exit 129\n  fi\ndone\nexec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The shim is only a measurement while it still rejects the flag and
	// still forwards everything else.
	if out, err := git(repo, "patch-id", "--verbatim"); err == nil {
		t.Fatalf("the shim did not reject --verbatim (%q); this arm measures nothing", out)
	}
	if mustGit(t, repo, "rev-parse", "--abbrev-ref", "HEAD") != "main" {
		t.Fatal("the shim does not forward an ordinary git command; this arm measures the shim")
	}

	held := treeHolds(tr)
	err = RemoveSessionTree(tr, false)
	if err == nil {
		t.Fatal("a tree was retired on a git that cannot answer the question that licensed it")
	}
	if !strings.Contains(err.Error(), "patch-id") {
		t.Errorf("the refusal must name what could not be asked, got: %v", err)
	}
	if held == "" {
		t.Errorf("treeHolds took a tree RemoveSessionTree refused: %v", err)
	}
	if _, serr := os.Stat(tr.Path); serr != nil {
		t.Errorf("the refused tree was removed anyway: %v", serr)
	}
	if !branchExists(repo, tr.Branch) {
		t.Error("the refused branch was deleted anyway")
	}
}

// The twin lookup is COUNTED and not a set, pinned because a set is the
// obvious way to write it and passes every other arm in this file.
//
// One id can belong to two commits ahead — an add, a revert, and the same
// add again is the ordinary way to get there — and a base that holds ONE of
// them holds one of them. A set pairs both against that single twin and
// licenses deleting the second, which is a branch being the last copy of its
// own work while a guard says it is not.
func TestVerbatimTwinIsConsumedByTheCommitItPairs(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)
	commitIn(t, repo, "f.txt", "a\n", "seed")

	mustGit(t, repo, "checkout", "-q", "-b", "br")
	commitIn(t, repo, "f.txt", "a\nX\n", "br: add X")
	commitIn(t, repo, "f.txt", "a\n", "br: take X back")
	commitIn(t, repo, "f.txt", "a\nX\n", "br: add X again")

	mustGit(t, repo, "checkout", "-q", "main")
	commitIn(t, repo, "f.txt", "a\nX\n", "main: add X")
	commitIn(t, repo, "f.txt", "a\n", "main: take X back")

	// The fixture is the collision or it measures nothing: two of the three
	// commits ahead must carry ONE id, and the base must hold exactly one
	// copy of it.
	ahead, err := patchIDsVerbatim(repo, "main..br")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, id := range ahead {
		counts[id]++
	}
	dup := ""
	for id, n := range counts {
		if n == 2 {
			dup = id
		}
	}
	if dup == "" {
		t.Fatalf("no id is carried by two commits ahead (%v); this fixture measures nothing", counts)
	}
	onBase, err := patchIDsVerbatim(repo, "br..main")
	if err != nil {
		t.Fatal(err)
	}
	held := 0
	for _, id := range onBase {
		if id == dup {
			held++
		}
	}
	if held != 1 {
		t.Fatalf("the base holds %d copies of the shared id, want exactly 1", held)
	}

	unpaired, err := verbatimUnpaired(repo, "main", "br")
	if err != nil {
		t.Fatal(err)
	}
	if unpaired == "" {
		t.Error("two commits ahead were paired against one twin on the base — the second is the last copy of itself")
	}

	// THE CONTROL: the base lands the third one too, so there are two twins
	// for two commits and the range is held. Without it "it refused" is
	// satisfied by a lookup that pairs nothing.
	commitIn(t, repo, "f.txt", "a\nX\n", "main: add X again")
	unpaired, err = verbatimUnpaired(repo, "main", "br")
	if err != nil {
		t.Fatal(err)
	}
	if unpaired != "" {
		t.Errorf("verbatimUnpaired = %q over a range whose every commit has a twin of its own", unpaired)
	}
}
