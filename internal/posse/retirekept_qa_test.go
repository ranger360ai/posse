//go:build posse_arm3

package posse

// ranger-base-daa60: ADR 0058's amendment of 2026-09-06 (ranger-base-qz3cr)
// — a tree whose landing was a DECISION, or whose branch a closed verdict
// answered, retires with its tip kept at refs/posse/retired/<branch>.
//
// WHAT THIS FILE MEASURES AND WHAT IT DELIBERATELY DOES NOT. The retire
// itself — that the trailer-paired tree now goes, that both surfaces write
// the same ref, that a MEASURED retire writes none — is pinned in the two
// tables that already exist (retiresweep_qa_test.go, retirecmd_qa_test.go),
// because those tables are where the two surfaces are held to one answer and
// a third table beside them is how they drift. `keptAt` is the field, and it
// is asserted in BOTH directions on every retiring arm there, which is the
// amendment's Verification (a) and (g) and the "measured arm writes a ref"
// mutation.
//
// What is here is everything those tables have no fixture shape for: the
// merge-back record that decides WHETHER the tip may be kept (b, c, d), and
// the three ways writing the ref can fail or must not happen (e, f, h).
//
// THE ARMS ARE EACH OTHER'S CONTROLS, and the pairs are chosen so that no
// one-sided defect passes: (b) keeps on an open block and its twin retires
// with a PIN planted and no bead — a licence read off the pin instead of the
// bead reds the twin, and a licence that ignores the record entirely reds
// (b). (c) retires on a standing verdict and keeps when the branch moved
// past it. (d) keeps where nothing was ever decided, which is what stops
// "unpaired" from being a licence on its own.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// qzPaired is the class the amendment was measured over: one commit on the
// branch, and on main a hand landing carrying git's `-x` trailer for it. The
// pairing is a DECISION — nothing measures what the resolution kept — so
// fact 2 refuses the tip and the ref is what makes the removal safe.
//
// The base moves first and with its own object, or the pick rebuilds the
// identical commit and main reaches it by sha, which measures nothing
// (ranger-base-g2xf's fixture rule).
func qzPaired(t *testing.T, repo string, tr *SessionTree) string {
	t.Helper()
	commitOldIn(t, tr.Path, "adr.md", "status: accepted\n", "a-1: the fix")
	sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
	commitOldIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	commitOldIn(t, repo, "adr.md", "status: accepted (reworded)\n",
		"a-1: the fix, by hand\n\n(cherry picked from commit "+sha+")")
	return sha
}

// qzUnpaired is the other half of the class: a commit NOTHING on main
// accounts for, on a branch whose merge-back is blocked. Both halves are
// load-bearing — main touches the same lines from the same ancestor, so the
// rebase conflicts and MergeSessionWork answers no, which is the only way a
// tree stays unpaired past a landing sweep.
func qzUnpaired(t *testing.T, repo string, tr *SessionTree) string {
	t.Helper()
	commitOldIn(t, tr.Path, "f.go", "package p\n\nfunc f() int { return 1 }\n", "a-1: the branch's answer")
	commitOldIn(t, repo, "f.go", "package p\n\nfunc f() int { return 2 }\n", "main's own answer, conflicting")
	return mustGit(t, tr.Path, "rev-parse", "HEAD")
}

// qzBlock plants the merge-back record for this branch in the listing the
// fake serves `--label-any` from: the same title mergeBlockedTitle builds,
// so what the retire finds is the bead the sweep would have filed and not a
// bead that merely mentions the branch.
//
// `closed` zero writes an OPEN row; otherwise the row is closed and carries
// that instant as its verdict.
func qzBlock(t *testing.T, repo string, tr *SessionTree, id string, closed time.Time) {
	t.Helper()
	row := map[string]any{
		"id":     id,
		"title":  mergeBlockedTitle(tr.Branch, "main"),
		"status": "open",
		"labels": []string{MergeBlockedLabel},
	}
	if !closed.IsZero() {
		row["status"] = "closed"
		row["closed_at"] = closed.Format(time.RFC3339)
		row["updated_at"] = closed.Format(time.RFC3339)
	}
	b, err := json.Marshal([]map[string]any{row})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo, "fake-list-labeled.json"), string(b))
}

// qzRetired is where the kept tip actually is, "" for no such ref.
func qzRetired(t *testing.T, tr *SessionTree) string {
	t.Helper()
	return refSHA(tr.Repo, retiredTipRef(tr.Branch))
}

// ADR 0058 amendment, Verification (b) AND the mutation it names: "the
// licence read from the pin instead of the bead".
//
// The two arms are one fixture apart. An OPEN block bead is a handoff in
// flight and its branch is the evidence, so the tree stays and the sentence
// names the bead a reader has to go and answer. A PIN with no open bead is
// the state a failed prune leaves behind (prunePinnedBlocks is best effort),
// and it must decide nothing: the pin is DERIVED from the bead, so reading
// it would be two readings of one fact (ADR 0011) whose failure mode is a
// tree kept forever with no sentence naming anything.
func TestAnOpenBlockKeepsThePairedTreeAndAStalePinDecidesNothing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		open   bool // an OPEN block bead names this branch
		pin    bool // refs/posse/merge-blocked/<branch> exists
		retire bool
		says   []string
	}{{
		name:   "an open merge-back block keeps the tree and names the bead",
		open:   true,
		retire: false,
		says:   []string{"holds 1 commit(s) main does not", "b-9 is still open on it", "handoff in flight"},
	}, {
		name:   "a pin with no open bead decides nothing and the tree retires",
		pin:    true,
		retire: true,
		says:   []string{"are kept at refs/posse/retired/"},
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d, repo, tr := izFixture(t, "closed", "")
			branchTip := qzPaired(t, repo, tr)
			if c.open {
				qzBlock(t, repo, tr, "b-9", time.Time{})
			}
			if c.pin {
				if _, ref := pinBlockedWork(tr); ref == "" {
					t.Fatal("the pin fixture did not arm — nothing to measure")
				}
			}
			n27xvQuiet(t, tr)

			out := izRetire(t, d, repo)
			for _, want := range c.says {
				if !strings.Contains(out, want) {
					t.Errorf("--retire does not say %q:\n%s", want, out)
				}
			}
			if !c.retire {
				if !strings.Contains(out, "kept: ") {
					t.Errorf("--retire kept the tree without saying so:\n%s", out)
				}
				izStanding(t, tr, branchTip, out)
				if got := qzRetired(t, tr); got != "" {
					t.Errorf("a kept tree had its tip written to %s (%s) — the ref is written only by a retire that then removes:\n%s",
						retiredTipRef(tr.Branch), got, out)
				}
				return
			}
			if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
				t.Errorf("the tree is still standing: %v\n%s", err, out)
			}
			if got := qzRetired(t, tr); got != branchTip {
				t.Errorf("%s is at %q, want %q:\n%s", retiredTipRef(tr.Branch), got, branchTip, out)
			}
		})
	}
}

// Verification (c) and (d): the UNPAIRED tip, where nothing at all accounts
// for the commits and the only thing that can say no landing is still owed
// is a verdict somebody reached and closed.
//
// Three arms, and the second and third are the first one's controls. A
// standing verdict retires; a verdict the branch has moved past does not,
// because a branch that gained a commit after its verdict is a new question
// (priorMergeBlocked's own rule, asked here for the same reason); and no
// verdict at all does not, because nobody has decided this branch's landing
// and "unpaired" is not itself a licence.
//
// Asked through `posse worktrees --retire`, which asks D1 and files nothing.
// The sweep's landing half would file a merge-back bead over two of these
// fixtures on its way past — correctly — and the bead it filed would then be
// the OPEN block the arm above already pins.
func TestAnUnpairedTreeRetiresOnlyOnAVerdictItsBranchHasNotMovedPast(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		verdict time.Duration // how long ago the block was closed; 0 = no block at all
		retire  bool
		says    []string
	}{{
		name:    "a closed verdict the branch has not moved past retires it, keeping the tip",
		verdict: time.Hour,
		retire:  true,
		says: []string{"its 1 commit(s) main accounts for only by the closed verdict b-7 are kept at refs/posse/retired/",
			"compare `git log main..refs/posse/retired/"},
	}, {
		name:    "a branch that moved after its verdict is kept and the sentence says so",
		verdict: 30 * time.Hour,
		says:    []string{"holds 1 commit(s) main does not", "has moved since b-7 answered it", "a new question"},
	}, {
		name: "no verdict at all keeps it, and says nobody has decided its landing",
		says: []string{"holds 1 commit(s) main does not", "no verdict has been reached on landing them"},
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d, repo, tr := izFixture(t, "closed", "")
			branchTip := qzUnpaired(t, repo, tr)
			if c.verdict != 0 {
				qzBlock(t, repo, tr, "b-7", time.Now().Add(-c.verdict))
			}
			n27xvQuiet(t, tr)

			// The fixture has to BE unpaired or every arm here measures the
			// arm above instead.
			if eq := equivalentOnBase(tr.Repo, "main", tr.Branch); len(eq) > 0 {
				t.Fatalf("the fixture is paired after all (%v) — this is not the unpaired class", equivNotes(eq))
			}

			out := izRetire(t, d, repo)
			for _, want := range c.says {
				if !strings.Contains(out, want) {
					t.Errorf("--retire does not say %q:\n%s", want, out)
				}
			}
			if !c.retire {
				izStanding(t, tr, branchTip, out)
				if got := qzRetired(t, tr); got != "" {
					t.Errorf("a kept tree had its tip written to %s (%s):\n%s", retiredTipRef(tr.Branch), got, out)
				}
				return
			}
			if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
				t.Errorf("the tree is still standing: %v\n%s", err, out)
			}
			if branchExists(tr.Repo, tr.Branch) {
				t.Errorf("%s survived the retire:\n%s", tr.Branch, out)
			}
			if got := qzRetired(t, tr); got != branchTip {
				t.Errorf("%s is at %q, want %q:\n%s", retiredTipRef(tr.Branch), got, branchTip, out)
			}
			// The whole point of the ref: the commits are still reachable
			// and the command the line prints is the one that reads them.
			if log := mustGit(t, repo, "log", "--format=%H", "main..refs/posse/retired/"+tr.Branch); log != branchTip {
				t.Errorf("`git log main..%s` prints %q, not the tip that was kept (%q)",
					retiredTipRef(tr.Branch), log, branchTip)
			}
		})
	}
}

// Verification (e): the ref already exists at ANOTHER sha — a reopened bead
// relaunched into the same seat name and retired here twice. Overwriting
// would lose the first tip, so the tree is kept and both shas are named; the
// remedy is the operator's `update-ref -d` and the sentence hands it to them.
func TestARetiredRefAtAnotherShaKeepsTheTreeAndNamesBothShas(t *testing.T) {
	t.Parallel()
	d, repo, tr := izFixture(t, "closed", "")
	branchTip := qzPaired(t, repo, tr)
	// Somebody else's tip already under this branch's name. main's own is a
	// commit the branch does not reach, which is what makes it "another".
	other := mustGit(t, repo, "rev-parse", "HEAD")
	if other == branchTip {
		t.Fatal("the fixture's two shas are one — nothing here is a collision")
	}
	mustGit(t, repo, "update-ref", retiredTipRef(tr.Branch), other)
	n27xvQuiet(t, tr)

	out := izRetire(t, d, repo)
	for _, want := range []string{"already holds " + abbrevSHA(other), abbrevSHA(branchTip), "update-ref -d " + retiredTipRef(tr.Branch)} {
		if !strings.Contains(out, want) {
			t.Errorf("--retire does not say %q:\n%s", want, out)
		}
	}
	izStanding(t, tr, branchTip, out)
	if got := qzRetired(t, tr); got != other {
		t.Errorf("%s moved off the tip that was already there: %q, want %q", retiredTipRef(tr.Branch), got, other)
	}
}

// Verification (f), AND the mutation the ADR names first: "the ref written
// after the removal instead of before".
//
// A PATH shim that refuses `update-ref` and delegates everything else. With
// the write BEFORE the removal — the order the record specifies — the
// refusal costs nothing: the tree, the branch and the commit all stand and
// the line says why. With the write after, the tree is already gone by the
// time the shim says no, and the commits this class is DEFINED by — the ones
// no measurement accounts for — are reachable from nothing at all. That is
// what the standing assertions below catch, and they are the reason this arm
// exists rather than a second sentence check.
//
// TWO SHIMS, and the second is why the read-back exists. A write that
// FAILS LOUDLY is caught by its exit status; a write that exits 0 and leaves
// no ref is caught by nothing except asking git what the ref resolves to,
// which is the only instrument for the claim being made ("a ref now reaches
// this sha"). Without the second arm the rev-parse in keepRetiredTip is
// decoration.
//
// Not parallel: it sets PATH for the process.
func TestARefusedUpdateRefKeepsTheTreeAndRemovesNothing(t *testing.T) {
	for _, c := range []struct {
		name string
		body string // the shim's update-ref arm
		says string
	}{{
		name: "update-ref refusing keeps the tree and removes nothing",
		body: "echo 'fatal: cannot lock ref' >&2\n    exit 1",
		says: "could not be written",
	}, {
		name: "update-ref exiting 0 and writing nothing keeps the tree too",
		body: "exit 0",
		says: "did not read back at",
	}} {
		t.Run(c.name, func(t *testing.T) {
			d, repo, tr := izFixture(t, "closed", "")
			branchTip := qzPaired(t, repo, tr)
			n27xvQuiet(t, tr)

			shim := t.TempDir()
			// Resolved BEFORE the shim goes on PATH, so the shim can
			// delegate to the real binary by absolute path.
			real, err := exec.LookPath("git")
			if err != nil {
				t.Skip("no git on PATH to delegate to")
			}
			write(t, filepath.Join(shim, "git"), fmt.Sprintf(
				"#!/bin/sh\nfor a in \"$@\"; do\n  if [ \"$a\" = update-ref ]; then\n    %s\n  fi\ndone\nexec %q \"$@\"\n", c.body, real))
			if err := os.Chmod(filepath.Join(shim, "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
			// The shim has to BE the trap, or this pin measures a plain
			// green run: after it, no update-ref may leave a ref behind.
			git(repo, "update-ref", "refs/posse/probe", branchTip)
			if refSHA(repo, "refs/posse/probe") != "" {
				t.Fatal("the shim did not intercept update-ref — nothing here is measured")
			}

			out := izRetire(t, d, repo)
			if !strings.Contains(out, c.says) || !strings.Contains(out, "nothing is removed") {
				t.Errorf("--retire did not say %q and that nothing was removed:\n%s", c.says, out)
			}
			izStanding(t, tr, branchTip, out)
			if list := mustGit(t, repo, "worktree", "list", "--porcelain"); !strings.Contains(list, tr.Path) {
				t.Errorf("the worktree registration was taken although the ref never landed:\n%s", list)
			}
			if got := qzRetired(t, tr); got != "" {
				t.Errorf("%s exists at %s after a write that did not land", retiredTipRef(tr.Branch), got)
			}
		})
	}
}

// The two keeps that are not about the merge-back record at all, and both
// would be silent losses without a pin: fact 2's OTHER refusals must still
// reach the operator in their own words, and no ref may be written on the
// way past them.
//
//   - a DIRTY tree of this class (ADR 0041). Every fixture in the arms above
//     is clean, so nothing there tells a dirty tree apart from a clean one:
//     keptTip's dirty guard removed, this tree writes a ref, RemoveSessionTree
//     then refuses for the dirt, and the operator gets "⚠ not retired" over
//     a handoff bead's own evidence plus a ref nothing prunes.
//   - a store that will not answer. "bd is down" is not evidence that a
//     landing is no longer owed, and this is the one question in the kept
//     retire whose answer comes from outside git.
func TestFactTwosOtherRefusalsKeepTheTreeAndWriteNoRef(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		setUp func(t *testing.T, repo string, tr *SessionTree)
		says  []string
	}{{
		name: "a dirty tree of this class keeps ADR 0041's own sentence",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			write(t, filepath.Join(tr.Path, "scratch.txt"), "not committed\n")
		},
		says: []string{"has uncommitted work", "scratch.txt"},
	}, {
		name: "a store that will not answer keeps the tree",
		setUp: func(t *testing.T, repo string, tr *SessionTree) {
			write(t, filepath.Join(repo, "fake-list-fail"), "")
		},
		says: []string{"holds 1 commit(s) main does not", "could not say whether a merge-back block is still owed"},
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d, repo, tr := izFixture(t, "closed", "")
			branchTip := qzPaired(t, repo, tr)
			c.setUp(t, repo, tr)
			n27xvQuiet(t, tr)

			out := izRetire(t, d, repo)
			if !strings.Contains(out, "kept: ") {
				t.Errorf("--retire did not keep the tree:\n%s", out)
			}
			for _, want := range c.says {
				if !strings.Contains(out, want) {
					t.Errorf("--retire does not say %q:\n%s", want, out)
				}
			}
			izStanding(t, tr, branchTip, out)
			if got := qzRetired(t, tr); got != "" {
				t.Errorf("%s was written at %s over a tree nothing retired:\n%s", retiredTipRef(tr.Branch), got, out)
			}
		})
	}
}

// ranger-base-v2rj7's shape, held by the amendment exactly where ADR 0058
// left it: a commit the tree's own HEAD holds and its BRANCH does not reach.
//
// The ref is written at the BRANCH tip, so it would not keep that commit —
// and a kept retire that removed the tree anyway would drop the only
// reference to a whole detached session's work, which is the loss v2rj7 was
// filed about. Every OTHER fact here says go: the bead is closed, the
// session is gone, the tree is quiet, and a closed verdict stands over the
// branch. Only the tip's own shape keeps it, which is what makes this the
// arm that measures keptTip's refusal of a non-branch tip rather than any of
// the record checks around it.
func TestADetachedTreesWorkIsNotTheRefsToKeep(t *testing.T) {
	t.Parallel()
	d, repo, tr := izFixture(t, "closed", "")
	mustGit(t, tr.Path, "checkout", "-q", "--detach")
	commitOldIn(t, tr.Path, "adr.md", "status: accepted\n", "a-1: off its own branch")
	head := mustGit(t, tr.Path, "rev-parse", "HEAD")
	qzBlock(t, repo, tr, "b-3", time.Now().Add(-time.Hour))
	n27xvQuiet(t, tr)

	out := izRetire(t, d, repo)
	if !strings.Contains(out, "kept: ") {
		t.Errorf("--retire did not keep a tree whose HEAD holds work its branch does not:\n%s", out)
	}
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("the detached tree was removed: %v\n%s", err, out)
	}
	if _, err := git(repo, "rev-parse", "--verify", "--quiet", head+"^{commit}"); err != nil {
		t.Errorf("the detached HEAD's commit %s is gone from the object store: %v", abbrevSHA(head), err)
	}
	if got := qzRetired(t, tr); got != "" {
		t.Errorf("%s was written at %s over a tip it does not reach — the ref keeps the branch, not the tree's HEAD",
			retiredTipRef(tr.Branch), got)
	}
}

// Verification (h): --dry-run reads and says, and writes NO ref. The flag's
// whole promise is that a diagnostic pass changes no state, and a ref under
// a namespace nothing prunes is state that outlives every tree on the board.
//
// ASKED OF retireTree DIRECTLY, and that is a statement about the sweep and
// not a convenience. MEASURED while this landed: landClosedTrees under
// --dry-run prints `would land …` and `continue`s for every tree with
// anything ahead of the base, so it never reaches the retire at all — and
// every tree of THIS class has something ahead by construction, that being
// what fact 2 refused it over. So the sweep's dry run says nothing about a
// kept retire today, the sentence below is reachable only from here, and
// widening that `continue` is a change to what --dry-run predicts over every
// class (a tree that would genuinely fast-forward reads as unpaired until it
// has, so the retire asked before the land answers about a tree that will
// not exist). Reported on ranger-base-daa60 rather than decided here.
//
// The two halves are both load-bearing: the sentence, because a dry run that
// does not name the ref is a dry run the operator cannot check; and the
// silence on disk, because the ref is the one piece of state this record
// adds that nothing ever prunes.
func TestDryRunOverAKeptRetireSaysWhatItWouldKeepAndWritesNoRef(t *testing.T) {
	t.Parallel()
	d, repo, tr := izFixture(t, "closed", "")
	branchTip := qzPaired(t, repo, tr)
	n27xvQuiet(t, tr)
	d.DryRun = true

	var lock *LaunchLock
	grace := d.App.graceAfter("retire_tree_after", DefaultRetireTreeAfter, d.errWriter())
	d.retireTree(tr, "a-1", "closed", newBlockedRecord(d.Bd), grace, &lock, false)
	out := dispatcherOut(d)
	if lock != nil {
		t.Errorf("--dry-run took the launcher lock")
	}
	for _, want := range []string{"would retire", "keeping 1 commit(s) at " + retiredTipRef(tr.Branch)} {
		if !strings.Contains(out, want) {
			t.Errorf("--dry-run does not say %q:\n%s", want, out)
		}
	}
	izStanding(t, tr, branchTip, out)
	if got := qzRetired(t, tr); got != "" {
		t.Errorf("--dry-run wrote %s at %s — a dry run changes no state", retiredTipRef(tr.Branch), got)
	}

	// And the sweep really does return before this, so the paragraph above
	// is a measurement and not a story: if it ever stops being true, this
	// pin is the one that should be rewritten to go through the sweep.
	d2, repo2, tr2 := izFixture(t, "closed", "")
	qzPaired(t, repo2, tr2)
	n27xvQuiet(t, tr2)
	d2.DryRun = true
	d2.landClosedTrees(repo2)
	if sweep := dispatcherOut(d2); strings.Contains(sweep, "would retire") {
		t.Errorf("the sweep's dry run now reaches the retire — pin it there instead of here:\n%s", sweep)
	}
	if got := qzRetired(t, tr2); got != "" {
		t.Errorf("the sweep's dry run wrote %s at %s", retiredTipRef(tr2.Branch), got)
	}
}

// Verification (i): the LISTING's clause and `--retire` say the same thing
// about the same tree, over two fixtures because the second surface destroys
// the first's. The listing is a report and carries the count and the ref;
// the act carries the whole sentence, the evidence and the compare command.
// What must not happen is a listing that promises a plain retire over a tree
// whose commits are about to be kept — the operator would have no reason to
// go and look at refs/posse/retired at all.
func TestTheListingAndTheRetireAgreeAboutAKeptRetire(t *testing.T) {
	t.Parallel()
	d, repo, tr := izFixture(t, "closed", "")
	qzPaired(t, repo, tr)
	n27xvQuiet(t, tr)

	list := izList(t, d, repo)
	want := "retirable — the next pass takes it, keeping 1 commit(s) at " + retiredTipRef(tr.Branch)
	if !strings.Contains(list, want) {
		t.Errorf("the listing does not say %q:\n%s", want, list)
	}
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("`posse worktrees` removed a tree: %v", err)
	}

	d2, repo2, tr2 := izFixture(t, "closed", "")
	qzPaired(t, repo2, tr2)
	n27xvQuiet(t, tr2)
	act := izRetire(t, d2, repo2)
	if !strings.Contains(act, "are kept at "+retiredTipRef(tr2.Branch)) {
		t.Errorf("--retire does not name the ref the listing promised:\n%s", act)
	}
}
