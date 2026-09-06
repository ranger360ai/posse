//go:build posse_arm3

package posse

// ranger-base-iz8fx: ADR 0058 D3 — the operator gets the sweep's own retire
// predicate on demand (`posse worktrees --retire`), and the listing stops
// telling them a human can retire the tree.
//
// WHAT THIS FILE IS FOR, AND WHY IT IS NOT retiresweep_qa_test.go AGAIN.
// D1's four facts are pinned there, over the sweep. What is new here is that
// a SECOND surface asks them, and the failure this file exists to catch is
// the one a second surface always has: a predicate reimplemented at the new
// call site, agreeing with the first one on the day it is written and drifting
// afterwards. So every arm below is asked of BOTH commands over
// byte-identical fixtures, and the assertion is that they answer with the
// same act and the same fact — not that each answers something reasonable.
//
// They are asked over two fixtures and not one because the retirable arm
// DESTROYS its fixture: whichever command ran first would leave the second
// with no tree to have an opinion about.
//
// THE CONTROLS, and they run in both directions. A predicate that retired
// everything passes the retirable arm alone; one that retired nothing passes
// every keep. The table holds both, and the last arm is the one the bead
// asked for by name: a tree that could NEVER have been retirable — its bead
// open, its tree dirty and its session alive — so that no single-fact defect
// can reach a retire on it and a pass that took it would be broken three
// ways at once.
//
// WHY THE FIXTURE WRITES TWO bd FILES. The fake serves `bd show` from
// fake-show.json and `bd list --all` from fake-list.json, and ADR 0058 D3
// has the two surfaces read the store through DIFFERENT verbs on purpose
// (RetireAsk: a report may read it once, an act reads it per tree). Two
// files that could disagree is a fixture defect and not a feature, so
// izFixture writes both from one status and the pins never set them apart.
//
// WITH ONE DELIBERATE EXCEPTION, added by ranger-base-9ycqa finding 5.
// Every arm agreeing meant nothing measured WHICH verb either surface read:
// swapping `--retire`'s per-tree r.fresh for the cached r.reported — the
// natural optimisation, 1.5s against 41s over ADR 0058's 70-tree census, and
// exactly what the read-cost note in RetireAsk prices — left the suite
// green. izSplitFixture is the one fixture that sets the two files apart on
// purpose, and it is used by exactly one test.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// izFixture is n27xvFixture plus the listing's own read of the store: one
// status, written into both files the fake serves, so no arm here can pass
// because `bd show` and `bd list` were told different things.
func izFixture(t *testing.T, status, grace string) (*Dispatcher, string, *SessionTree) {
	t.Helper()
	d, repo, tr := n27xvFixture(t, status, grace)
	write(t, filepath.Join(repo, "fake-list.json"), `[{"id":"a-1","title":"t","status":"`+status+`"}]`)
	return d, repo, tr
}

// izAsk is what the two operator commands carry where the sweep carries a
// Dispatcher: the same store, the same herdr, the same dial.
func izAsk(t *testing.T, d *Dispatcher) *RetireAsk {
	t.Helper()
	return NewRetireAsk(d.App, d.Bd, d.HB, io.Discard)
}

// izRetire runs `posse worktrees --retire` over one repo.
func izRetire(t *testing.T, d *Dispatcher, repo string) string {
	t.Helper()
	var out strings.Builder
	if err := RetireSessionTrees(&out, d.App, izAsk(t, d), []string{repo}); err != nil {
		t.Fatalf("posse worktrees --retire: %v", err)
	}
	return out.String()
}

// izList runs `posse worktrees` over one repo.
func izList(t *testing.T, d *Dispatcher, repo string) string {
	t.Helper()
	var out strings.Builder
	if err := ListSessionTrees(&out, []string{repo}, izAsk(t, d)); err != nil {
		t.Fatalf("posse worktrees: %v", err)
	}
	return out.String()
}

// izArm is one fixture, one broken fact, and what BOTH commands owe about it.
type izArm struct {
	name   string
	status string // what the store says about a-1 ("" = closed)
	grace  string // `retire_tree_after:` ("" = the 1h default)
	setUp  func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree)
	// post runs after the fixture is aged: the one way an arm plants a
	// FRESH tree write without the ageing taking it back.
	post   func(t *testing.T, tr *SessionTree)
	retire bool
	// says is the fact that decided it, in the operator's words. Both
	// commands owe it wherever both speak.
	says []string
	// sweepQuiet is an arm where the SWEEP says nothing about this tree, on
	// one of the three grounds it has: the keep is transient (the grace, the
	// dial off), a landing line got there first and swallowed it (ADR 0058
	// D2's one tree, one line), or the sweep never reached the predicate at
	// all — an open bead's tree is a seat and landClosedTrees skips it, and
	// a tree no record names is ADR 0006's and it skips that too.
	// `--retire` still speaks: it is a command a human ran and is reading
	// the output of, and a tree it says nothing about is one they have to
	// ask about twice (D3's one line per tree).
	sweepQuiet bool
	// noRecord unsets the branch's bead stamp: ADR 0006's population, which
	// D4 leaves exactly where it was.
	noRecord bool
	// keptAt is ADR 0058's 2026-09-06 amendment: this retire writes the
	// branch tip to refs/posse/retired/<branch> before it removes anything,
	// and BOTH surfaces must leave the ref at that sha. A measured retire
	// must leave none.
	keptAt bool
}

var izArms = []izArm{{
	// ADR 0058 Verification 1, and the only arm where anything is destroyed.
	name:   "a closed bead's landed tree with no session is retired by both",
	retire: true,
	says:   []string{"its bead is closed", "nothing here is unlanded", "session gone"},
}, {
	// Fact 1, and the sweep's OWN skip rather than the retire's keep: an
	// open bead's tree is a seat, and landClosedTrees leaves a seat before
	// the predicate is ever asked (`if is.Status != "closed" { continue }`).
	// Nothing is retired either way, which is what the two surfaces owe each
	// other; what differs is that a human who asked gets told why.
	name:       "an open bead's tree is kept by both and --retire names the bead",
	status:     "in_progress",
	says:       []string{"its bead is in_progress"},
	sweepQuiet: true,
}, {
	// Fact 2, ADR 0041's class.
	name: "one uncommitted path keeps it in both",
	setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
		write(t, filepath.Join(tr.Path, "scratch.txt"), "not committed\n")
	},
	says: []string{"has uncommitted work", "scratch.txt"},
}, {
	// The class ADR 0058's 2026-09-06 amendment took off D4's list
	// (ranger-base-qz3cr): a `-x` trailer is somebody's DECISION that this
	// landed, not a measurement of what the landing kept — so it is still no
	// licence to delete, and the tree goes anyway because the tip is kept at
	// refs/posse/retired/<branch> first. It is the second arm here that
	// destroys its fixture, and it is the one that matters most for this
	// file's question: the sweep and `--retire` must write the same ref and
	// say the same sentence about it, or one of them is losing a commit.
	name: "a commit whose landing is only a -x trailer retires from both, with its tip kept",
	setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
		commitOldIn(t, tr.Path, "adr.md", "status: accepted\n", "a-1: the fix")
		sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
		commitOldIn(t, repo, "adr.md", "status: accepted (reworded)\n",
			"a-1: the fix, by hand\n\n(cherry picked from commit "+sha+")")
	},
	retire: true,
	keptAt: true,
	says: []string{"its 1 commit(s) main accounts for only by git's own -x trailer are kept at refs/posse/retired/",
		"compare `git log main..refs/posse/retired/"},
}, {
	// Fact 3.
	name: "a session herdr still lists keeps it in both",
	setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
		n27xvMeta(t, d, tr, "w1", "")
		saveWSTo(t, fakeDirOf(t), []fakeWS{{
			WorkspaceID: "w1",
			Label:       d.App.WorkspaceLabel(SessionOfBranch(tr.Branch)),
		}})
	},
	says: []string{"workspace w1 is alive"},
}, {
	// Fact 4, and the first of the two keeps the sweep is silent about.
	name: "an index touched inside the grace keeps it — silently in the sweep, out loud on demand",
	post: func(t *testing.T, tr *SessionTree) {
		gd := mustGit(t, tr.Path, "rev-parse", "--absolute-git-dir")
		now := time.Now()
		if err := os.Chtimes(filepath.Join(gd, "index"), now, now); err != nil {
			t.Fatal(err)
		}
	},
	says:       []string{"inside the", "grace"},
	sweepQuiet: true,
}, {
	// The dial. Same shape: the sweep says it once by not acting, and the
	// human who just asked is told why nothing happened.
	name:       "retire_tree_after: off keeps it — silently in the sweep, out loud on demand",
	grace:      "off",
	says:       []string{"`retire_tree_after:` is off"},
	sweepQuiet: true,
}, {
	// ADR 0006, unchanged by D4: no record accounts for this tree, so
	// nothing unattended may act on it — and there is no `--force` here to
	// say otherwise.
	name:       "a tree no bead record accounts for is kept by both and stays a human's",
	noRecord:   true,
	says:       []string{"no record says which bead", "ADR 0006", "stays a human's"},
	sweepQuiet: true,
}, {
	// Fact 2's OTHER half, and the arm that made this bead change a sentence
	// it had not planned to. `git patch-id` NORMALISES WHITESPACE
	// (ranger-base-x8jp), so a branch whose fix main re-indented is
	// "equivalent" by patch-id while the bytes about to be deleted are the
	// last copy of themselves — treeState calls that `nothing unlanded
	// (… as an equivalent patch on main)` and fact 2 keeps the tree.
	//
	// Before D3 those two answers were never printed together. The listing
	// puts them one line apart, so the keep has to say WHICH measurement it
	// has and which it lacks, or the operator reads two verdicts about one
	// tree and no way to tell which is the careful one. MEASURED 2026-09-06
	// on this box: three standing trees read exactly that way, olwk among
	// them — the very tree ADR 0058's census counted as retirable.
	name: "a patch-id equivalence whose bytes main does not hold keeps it, and says which half it has",
	setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
		commitOldIn(t, tr.Path, "f.go", "func f() {\n    return 1\n}\n", "a-1: the fix")
		// The base moves FIRST and with its own object, or main reaches the
		// branch's commit by sha and nothing here is measured at all
		// (ranger-base-g2xf's fixture rule).
		commitOldIn(t, repo, "f.go", "func f() {\n\treturn 1\n}\n", "the same fix by hand, re-indented")
	},
	says: []string{"equivalent patches", "patch-id normalises whitespace", "does not hold their bytes for f.go"},
	// The sweep's landing half reaches this tree first and reports the
	// equivalence there, so the keep is swallowed exactly as the -x arm's is.
	sweepQuiet: true,
}, {
	// THE CONTROL the bead asked for by name: a tree that could never have
	// been retirable. Three facts broken at once, so no single-fact defect
	// anywhere in D1 can reach a retire on it, and a command that took it
	// would be broken in three independent places.
	name:       "a tree that could never have been retirable is kept by both",
	status:     "in_progress",
	sweepQuiet: true,
	setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
		write(t, filepath.Join(tr.Path, "scratch.txt"), "not committed\n")
		n27xvMeta(t, d, tr, "w1", "")
		saveWSTo(t, fakeDirOf(t), []fakeWS{{
			WorkspaceID: "w1",
			Label:       d.App.WorkspaceLabel(SessionOfBranch(tr.Branch)),
		}})
	},
	says: []string{"its bead is in_progress"},
}}

// izBuild builds one arm's fixture, in the state an operator would meet it.
func izBuild(t *testing.T, c izArm) (*Dispatcher, string, *SessionTree) {
	t.Helper()
	status := c.status
	if status == "" {
		status = "closed"
	}
	d, repo, tr := izFixture(t, status, c.grace)
	if c.setUp != nil {
		c.setUp(t, d, repo, tr)
	}
	if c.noRecord {
		mustGit(t, repo, "config", "--unset", beadKey(tr.Branch))
		tr.Bead = ""
	}
	n27xvQuiet(t, tr)
	if c.post != nil {
		c.post(t, tr)
	}
	return d, repo, tr
}

// izKeptTip is ADR 0058's amendment asked of whichever surface just acted:
// the branch tip is at refs/posse/retired/<branch> when this arm's retire
// keeps it, and no such ref exists when the retire was a MEASURED one. Both
// directions from one field — a ref over the measured class is the trash
// directory the record rejected, and no ref over this one is a lost commit,
// and the line printed reads identically either way.
func izKeptTip(t *testing.T, tr *SessionTree, branchTip string, want bool, out string) {
	t.Helper()
	got := refSHA(tr.Repo, retiredTipRef(tr.Branch))
	switch {
	case want && got != branchTip:
		t.Errorf("%s is at %q, want the branch tip %q it was retired from:\n%s",
			retiredTipRef(tr.Branch), got, branchTip, out)
	case !want && got != "":
		t.Errorf("a measured retire wrote %s at %s:\n%s", retiredTipRef(tr.Branch), got, out)
	}
}

// izStanding is what every keep is for: the tree, the branch and the work
// are all still there.
func izStanding(t *testing.T, tr *SessionTree, head, out string) {
	t.Helper()
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("a tree the predicate keeps was removed: %v\n%s", err, out)
	}
	if !branchExists(tr.Repo, tr.Branch) {
		t.Fatalf("%s was deleted under a keep:\n%s", tr.Branch, out)
	}
	if head != "" {
		if _, err := git(tr.Repo, "rev-parse", "--verify", "--quiet", head+"^{commit}"); err != nil {
			t.Errorf("the kept tree's work %s is gone from the object store: %v", abbrevSHA(head), err)
		}
	}
}

// ADR 0058 Verification 5: `posse worktrees --retire` prints the same
// verdicts the sweep does, over the same facts, and destroys exactly what
// the sweep would destroy.
func TestRetireOnDemandActsAndSpeaksExactlyAsTheSweepDoes(t *testing.T) {
	t.Parallel()
	for _, c := range izArms {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// The COMMAND, over its own fixture.
			d, repo, tr := izBuild(t, c)
			head, _ := workHead(tr)
			branchTip := refSHA(tr.Repo, "refs/heads/"+tr.Branch)
			out := izRetire(t, d, repo)

			// One line per tree, always — the half of D3 that is not the
			// sweep's behaviour.
			if n := strings.Count(strings.TrimSpace(out), "\n") + 1; n != 1 {
				t.Errorf("--retire printed %d lines about one tree, want 1:\n%s", n, out)
			}
			if !strings.Contains(out, tr.Branch) {
				t.Errorf("--retire never named the tree it was asked about:\n%s", out)
			}
			for _, want := range c.says {
				if !strings.Contains(out, want) {
					t.Errorf("--retire does not say %q: %s", want, out)
				}
			}
			if c.retire {
				if !strings.Contains(out, "⌫") || !strings.Contains(out, "retired:") {
					t.Errorf("--retire did not say what it retired:\n%s", out)
				}
				if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
					t.Errorf("--retire left the tree standing: %v\n%s", err, out)
				}
				if branchExists(tr.Repo, tr.Branch) {
					t.Errorf("%s survived --retire:\n%s", tr.Branch, out)
				}
				if list := mustGit(t, repo, "worktree", "list", "--porcelain"); strings.Contains(list, tr.Path) {
					t.Errorf("the worktree registration outlived the tree:\n%s", list)
				}
				izKeptTip(t, tr, branchTip, c.keptAt, out)
			} else {
				if !strings.Contains(out, "kept: ") {
					t.Errorf("--retire kept the tree without saying so:\n%s", out)
				}
				izStanding(t, tr, head, out)
			}

			// THE SWEEP, over a second fixture built the same way. This is
			// the whole point of the file: two surfaces, one predicate, and
			// an assertion that catches the second one drifting.
			d2, repo2, tr2 := izBuild(t, c)
			head2, _ := workHead(tr2)
			branchTip2 := refSHA(tr2.Repo, "refs/heads/"+tr2.Branch)
			d2.landClosedTrees(repo2)
			sweep := dispatcherOut(d2)

			if c.retire {
				izKeptTip(t, tr2, branchTip2, c.keptAt, sweep)
				if !strings.Contains(sweep, "retired:") {
					t.Errorf("the sweep did not retire a tree --retire took:\n%s", sweep)
				}
				if _, err := os.Stat(tr2.Path); !os.IsNotExist(err) {
					t.Errorf("the sweep left standing a tree --retire took: %v\n%s", err, sweep)
				}
				for _, want := range c.says {
					if !strings.Contains(sweep, want) {
						t.Errorf("the sweep's retire line does not say %q:\n%s", want, sweep)
					}
				}
				return
			}
			izStanding(t, tr2, head2, sweep)
			if c.sweepQuiet {
				// Silent about the KEEP specifically: the landing half may
				// still have spoken, and on the -x arm it must have.
				if strings.Contains(sweep, "kept: ") {
					t.Errorf("the sweep spoke a keep this arm says it swallows:\n%s", sweep)
				}
				return
			}
			if !strings.Contains(sweep, "kept: ") {
				t.Errorf("the sweep kept the tree without saying so:\n%s", sweep)
			}
			for _, want := range c.says {
				if !strings.Contains(sweep, want) {
					t.Errorf("the sweep's keep line does not say %q — the two surfaces have drifted:\n%s", want, sweep)
				}
			}
		})
	}
}

// ADR 0058 Verification 6: the listing says which pass takes the tree, and
// never again that a human can.
//
// The three arms are the ADR's three shapes and they are each other's
// controls: a listing hardwired to "retirable" fails the control, one
// hardwired to a keep fails the first arm, and one that treats a tree with
// no bead record as just another keep fails the third — ADR 0006's sentence
// is not a fact that failed, it is the reason the four facts were never
// asked.
func TestTheListingSaysWhichPassTakesTheTreeAndNeverPromisesAHuman(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		arm    izArm
		says   []string
		unsays []string
	}{{
		name: "a retirable tree is told which pass takes it",
		arm:  izArm{},
		says: []string{"retirable — the next pass takes it"},
	}, {
		// The control the bead asked for: a tree that could never have been
		// retirable. Without it "retirable" is a word a broken listing
		// prints over everything.
		name:   "a tree that could never have been retirable says which fact keeps it",
		arm:    izArms[len(izArms)-1],
		says:   []string{"kept: ", "its bead is in_progress"},
		unsays: []string{"retirable — the next pass takes it"},
	}, {
		name:   "a tree no record accounts for keeps ADR 0006's sentence",
		arm:    izArm{noRecord: true},
		says:   []string{"no record says which bead", "ADR 0006", "stays a human's"},
		unsays: []string{"retirable — the next pass takes it"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d, repo, tr := izBuild(t, c.arm)
			out := izList(t, d, repo)

			if !strings.Contains(out, tr.Branch) {
				t.Fatalf("the listing does not hold the tree at all:\n%s", out)
			}
			for _, want := range c.says {
				if !strings.Contains(out, want) {
					t.Errorf("the listing does not say %q:\n%s", want, out)
				}
			}
			for _, no := range c.unsays {
				if strings.Contains(out, no) {
					t.Errorf("the listing says %q about a tree nothing will retire:\n%s", no, out)
				}
			}
			// The sentence this record exists to delete, asserted on every
			// arm rather than once: it was true of no tree and it must be
			// printed about none.
			if strings.Contains(out, "a human can retire the tree") {
				t.Errorf("the listing still promises a human:\n%s", out)
			}
			// And the listing removes nothing. It is a report.
			if _, err := os.Stat(tr.Path); err != nil {
				t.Errorf("`posse worktrees` removed a tree: %v\n%s", err, out)
			}
		})
	}
}

// The other surface that carried the sentence, and the one arm that can
// reach it: `--land`'s refusal over a tree whose commits the base already
// holds as measured equivalents and whose bead no record names
// (unaccountedFor). It said "nothing here is unlanded and a human can retire
// the tree" — of a tree that is, by construction, the one population fact 1
// keeps forever. The ADR 0006 sentence stands; the promise goes.
func TestTheLandRefusalNoLongerPromisesAHumanOverAnEquivalentTree(t *testing.T) {
	t.Parallel()
	d, repo, tr := izFixture(t, "closed", "")
	commitOldIn(t, tr.Path, "adr.md", "status: accepted\n", "a-1: the fix")
	sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
	// The base moves first, or the pick rebuilds the identical object and
	// the base reaches it by SHA, which measures nothing (ranger-base-g2xf).
	commitOldIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	gitOld(t, repo, "cherry-pick", "-x", sha)
	mustGit(t, repo, "config", "--unset", beadKey(tr.Branch))
	n27xvQuiet(t, tr)

	var out strings.Builder
	if err := LandSessionTrees(&out, d.App, []string{repo}, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	// The fixture has to REACH the arm, or this pin measures nothing.
	if !strings.Contains(got, "nothing here is unlanded") {
		t.Fatalf("the fixture did not reach the measured-equivalence refusal:\n%s", got)
	}
	if strings.Contains(got, "a human can retire the tree") {
		t.Errorf("the land refusal still promises a human:\n%s", got)
	}
	for _, want := range []string{"no record says which bead", "ADR 0006"} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal dropped %q — ADR 0006's sentence is the part D3 keeps:\n%s", want, got)
		}
	}
}

// ranger-base-9ycqa finding 4: ADR 0058 D3's own acceptance is that
// `--retire` walks the board "under one BLOCKING launcher lock", and nothing
// measured the lock. Deleting lockLaunches and its `defer lock.Release()`
// from RetireSessionTrees left the whole internal/posse suite green
// (-overlay, mutant ok 217.9s against a 240.6s control). Without it,
// `--retire` destroys trees while a launcher is creating a session in one:
// the ADR 0011 §1 reclaim race the lock exists for.
//
// ONE ARM COVERS BOTH COMMANDS because both were unmeasured. `--land` takes
// the same lock for the same reason and its `defer` was equally free to go.
//
// The wait is measured by the lock's OWN waiting line and not by a clock. A
// command that never took the lock runs straight to completion, so `done`
// wins the race below and the arm reds at once rather than after a timeout —
// which is also why the two channels are selected on together instead of
// polling for the line alone.
func TestTheBoardWideCommandsWaitForTheLauncherLock(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// run is built on the test goroutine and called on another, so
		// nothing inside it touches t.
		run func(d *Dispatcher, repo string, ask *RetireAsk, w io.Writer) error
		// did is what the command must have done once the lock is free —
		// the other half of the pin, because a command that returned an
		// error while blocked would otherwise read as "it waited".
		did string
	}{{
		name: "posse worktrees --retire",
		run: func(d *Dispatcher, repo string, ask *RetireAsk, w io.Writer) error {
			return RetireSessionTrees(w, d.App, ask, []string{repo})
		},
		did: "retired:",
	}, {
		name: "posse worktrees --land",
		run: func(d *Dispatcher, repo string, ask *RetireAsk, w io.Writer) error {
			return LandSessionTrees(w, d.App, []string{repo}, false)
		},
		did: "",
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d, repo, tr := izBuild(t, izArm{})
			ask := izAsk(t, d)

			held, err := lockLaunches(d.App, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			released := false
			release := func() {
				if !released {
					released = true
					held.Release()
				}
			}
			defer release()

			buf := &syncBuf{}
			done := make(chan error, 1)
			go func() { done <- c.run(d, repo, ask, buf) }()

			// Either it says it is waiting for the lock, or it finished
			// without one. There is no third outcome, and a deadline here
			// is only the safety net.
			deadline := time.After(90 * time.Second)
			for waiting := false; !waiting; {
				select {
				case err := <-done:
					t.Fatalf("%s walked the board WITHOUT the launcher lock (it returned %v while the lock was held) — ADR 0011 §1's reclaim race is open:\n%s",
						c.name, err, buf.String())
				case <-deadline:
					t.Fatalf("%s neither waited for the lock nor finished:\n%s", c.name, buf.String())
				case <-time.After(2 * time.Millisecond):
					waiting = strings.Contains(buf.String(), "launcher lock held by")
				}
			}
			// It is blocked, and the destructive half has not run: the
			// waiting line is printed BEFORE flock, so the tree standing
			// here is what says the wait is real and not just announced.
			if _, err := os.Stat(tr.Path); err != nil {
				t.Errorf("%s destroyed a tree while it was waiting for the lock: %v", c.name, err)
			}

			release()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("%s: %v\n%s", c.name, err, buf.String())
				}
			case <-time.After(90 * time.Second):
				t.Fatalf("%s never finished after the lock was released:\n%s", c.name, buf.String())
			}
			if !strings.Contains(buf.String(), tr.Branch) {
				t.Errorf("%s said nothing about the tree it was waiting to act on:\n%s", c.name, buf.String())
			}
			if c.did != "" && !strings.Contains(buf.String(), c.did) {
				t.Errorf("%s waited for the lock and then did not %q:\n%s", c.name, c.did, buf.String())
			}
		})
	}
}

// izSplitFixture is izFixture with the two bd reads told DIFFERENT things:
// `bd show` says showStatus and `bd list --all` says listStatus. It is the
// deliberate exception to this file's fixture rule (see the header) and
// nothing but the test below may use it — an arm that disagreed by accident
// would be measuring the fake, not the rule.
func izSplitFixture(t *testing.T, showStatus, listStatus string) (*Dispatcher, string, *SessionTree) {
	t.Helper()
	d, repo, tr := izFixture(t, showStatus, "")
	write(t, filepath.Join(repo, "fake-list.json"), `[{"id":"a-1","title":"t","status":"`+listStatus+`"}]`)
	n27xvQuiet(t, tr)
	return d, repo, tr
}

// ADR 0011's reclaim rule, as ADR 0058 D3 splits it between the two
// surfaces: the LISTING is a report and may read the store once per repo
// (`reported`), the ACT reads fact 1 fresh per tree at the instant it acts
// on that tree (`fresh`), because nothing destroys work on a status somebody
// read earlier.
//
// This is the only place the rule is measurable. Everywhere else in this
// file the two reads are told the same thing on purpose, so `--retire`
// reading the cached listing instead of `bd show` — the optimisation RetireAsk's
// own cost note argues for and declines — left the suite green
// (ranger-base-9ycqa finding 5).
//
// The two arms run in both directions, which is what makes them a pin and
// not a preference: arm 1 is a bead CLOSED since the listing was taken, and
// the act must take the tree the report calls kept; arm 2 is a bead REOPENED
// since, and the act must keep the tree the report calls retirable. A
// surface reading the other's verb reds one arm; a surface reading a
// constant reds both.
func TestTheActReadsBdShowWhileTheListingReadsBdList(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name       string
		show, list string
		retires    bool
		says       string // --retire, which reads `bd show`
		listSays   string // `posse worktrees`, which reads `bd list --all`
	}{{
		name: "a bead closed since the listing was taken is retired anyway",
		show: "closed", list: "in_progress",
		retires:  true,
		says:     "retired:",
		listSays: "kept: its bead is in_progress",
	}, {
		// The direction that costs work if it is wrong: the listing is
		// stale in the OTHER direction and an act that trusted it would
		// destroy a live seat's tree.
		name: "a bead reopened since the listing was taken is kept anyway",
		show: "in_progress", list: "closed",
		retires:  false,
		says:     "kept: its bead is in_progress",
		listSays: "retirable — the next pass takes it",
	}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// The ACT, over its own fixture, because arm 1 destroys it.
			d, repo, tr := izSplitFixture(t, c.show, c.list)
			out := izRetire(t, d, repo)
			if !strings.Contains(out, c.says) {
				t.Errorf("--retire read `bd list` (%s) where it owes `bd show` (%s); it does not say %q:\n%s",
					c.list, c.show, c.says, out)
			}
			if _, err := os.Stat(tr.Path); c.retires != os.IsNotExist(err) {
				t.Errorf("--retire acted on the LISTING's status %q instead of `bd show`'s %q (tree gone = %v, want %v)\n%s",
					c.list, c.show, os.IsNotExist(err), c.retires, out)
			}

			// The LISTING, over a second one built the same way.
			d2, repo2, tr2 := izSplitFixture(t, c.show, c.list)
			list := izList(t, d2, repo2)
			if !strings.Contains(list, c.listSays) {
				t.Errorf("the listing read `bd show` (%s) where a report may read `bd list` once (%s); it does not say %q:\n%s",
					c.show, c.list, c.listSays, list)
			}
			if _, err := os.Stat(tr2.Path); err != nil {
				t.Errorf("`posse worktrees` removed a tree: %v\n%s", err, list)
			}
		})
	}
}
