package posse

// ranger-base-n27xv: ADR 0058 D1/D2 — the landing sweep retires a session
// tree whose bead is closed, whose work is measured on the base, whose
// session herdr proves gone, and which nobody has written to inside the
// grace.
//
// The arms below are the ADR's Verification 1-4 and they are EACH OTHER'S
// CONTROLS, which is the whole design of this file. A predicate that
// retired everything would pass verification 1 alone; one that retired
// nothing would pass every other arm. So one fixture is built, ONE fact is
// broken in it, and the pin asks for the retire on the untouched fixture and
// for a keep NAMING THAT FACT on each of the others.
//
// Each arm is asked twice, and the two questions are not the same one:
//
//   - `retirable` — the predicate. It is what `posse worktrees --retire`
//     (D3) will ask too, so its verdict is the thing that must name the fact.
//   - `landClosedTrees` — the sweep, over the same fixture, which is where
//     the destroy actually happens. A predicate that says "keep" while the
//     sweep removes the tree anyway is the only failure that matters, and
//     asking both about one fixture is what catches it.
//
// WHY THE FIXTURE'S COMMITS AND FILES ARE BACKDATED BY HAND. Fact 4 is
// denominated in tree WRITES, so a fixture built by this test is a tree
// written to a millisecond ago and every arm would be kept by the grace
// before it reached the fact it is about. gitOld dates the commits and
// n27xvQuiet ages the git dir's FILES — and deliberately not the directory
// itself, which stays freshly stamped: git creates and renames index.lock in
// there on every `git status`, so a lastTreeWrite that read the directory
// would answer "written just now" over every tree on the board and retire
// none of them. That is a measurement, not a guess (2026-09-05, macOS/APFS,
// git 2.51: five consecutive statuses in a quiescent worktree moved the
// directory every time and the index file twice), and this fixture is where
// it is pinned: a lastTreeWrite that walks directories as well as files reds
// this file (measured, one mutant per claim, 2026-09-05).

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// n27xvAge is how old the fixture's commits and git-dir files are made. Well
// past the 1h default grace, so an arm that reaches fact 4 fails it for its
// own reason and not for the age of the box's clock.
const n27xvAge = 26 * time.Hour

// gitOld runs git with both dates set n27xvAge back. The COMMITTER date is
// the one that matters (commitTime reads %cI) and the author date is set
// with it so the fixture does not read as a replay of itself.
func gitOld(t *testing.T, dir string, args ...string) string {
	t.Helper()
	when := time.Now().Add(-n27xvAge).Format(time.RFC3339)
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+when, "GIT_AUTHOR_DATE="+when)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitOldIn is commitIn with an old date, in the path-limited shape the
// wall requires of a persona.
func commitOldIn(t *testing.T, dir, path, body, msg string) {
	t.Helper()
	write(t, filepath.Join(dir, path), body)
	mustGit(t, dir, "config", "user.email", "p@example.com")
	mustGit(t, dir, "config", "user.name", "p")
	mustGit(t, dir, "add", path)
	gitOld(t, dir, "commit", "-q", "-m", msg, "--", path)
}

// n27xvQuiet makes the tree read as one nobody has touched for n27xvAge, in
// the three steps a fixture needs and a real tree gets for free.
//
//  1. the CHECKOUT's files are aged first, and one hour further back than
//     the git dir will be;
//  2. one `git status` settles the index against them. Without it every
//     later read of this tree REWRITES the index and the tree is never
//     quiet: git calls an entry whose recorded mtime is not older than the
//     index file's own "racily clean", re-checks it and writes the index
//     back — so a fixture that aged the index alone would have every arm
//     kept by fact 4, naming the wrong fact. MEASURED here (the first cut of
//     this file did exactly that);
//  3. the git dir's FILES are aged, and its DIRECTORY is deliberately left
//     with a fresh mtime: git creates and renames index.lock in there on
//     every status, so a lastTreeWrite that read the directory would answer
//     "written just now" over every tree on the board and retire none of
//     them.
func n27xvQuiet(t *testing.T, tr *SessionTree) {
	t.Helper()
	age := func(root string, when time.Time, skipGit bool) int {
		n := 0
		if err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if skipGit && e.Name() == ".git" {
				if e.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if e.IsDir() {
				return nil
			}
			if err := os.Chtimes(p, when, when); err != nil {
				return err
			}
			n++
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	age(tr.Path, time.Now().Add(-n27xvAge-time.Hour), true)
	mustGit(t, tr.Path, "status", "--porcelain")
	gd := mustGit(t, tr.Path, "rev-parse", "--absolute-git-dir")
	if n := age(gd, time.Now().Add(-n27xvAge), false); n == 0 {
		t.Fatalf("no files under %s to age — the fixture is not measuring fact 4", gd)
	}
}

// n27xvFixture is the population ADR 0058 exists for: a session tree whose
// bead the store calls closed, whose work is on the base, and whose session
// nothing on this box holds — with a herdr that is UP and listing somebody
// else's workspace, because an empty board is its own keep (rangerhq-snd)
// and a fixture that leaned on it would prove nothing about the rest.
func n27xvFixture(t *testing.T, status, grace string) (*Dispatcher, string, *SessionTree) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	commitOldIn(t, repo, "README.md", "seed\n", "seed")
	write(t, filepath.Join(repo, "fake-ready.json"), `[]`)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"`+status+`"}]`)
	cfg := "beads:\n  - " + repo + "\n"
	if grace != "" {
		cfg += "retire_tree_after: " + grace + "\n"
	}
	write(t, b.App.ConfigPath, cfg)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w9", Label: "somebody-elses-work"}})

	tr, err := b.App.EnsureSessionTree(repo, SessionForBead("ranger", repo, "a-1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := recordBead(tr.Repo, tr.Branch, "a-1"); err != nil {
		t.Fatal(err)
	}
	return d, repo, tr
}

// n27xvMeta writes the session meta a kill would have removed, naming this
// server so its listing is evidence about it (rangerhq-8fq).
func n27xvMeta(t *testing.T, d *Dispatcher, tr *SessionTree, workspace, socket string) {
	t.Helper()
	name := SessionOfBranch(tr.Branch)
	if socket == "" {
		socket = SocketID()
	}
	write(t, d.HB.metaPath(name), "name: "+name+"\nworkspace: "+workspace+
		"\npane: "+workspace+":p1\nemoji: x\nsocket: "+socket+
		"\ndir: "+tr.Path+"\nrepo: "+tr.Repo+"\nbranch: "+tr.Branch+"\nbead: a-1\n")
}

func TestSweepRetiresADeadLandedTreeAndNamesEveryFactThatKeepsOne(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status string // what the store says about a-1
		grace  string // `retire_tree_after:` ("" = the 1h default)
		setUp  func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree)
		// post runs after the fixture is aged, and is how an arm plants a
		// FRESH tree write — the one thing the ageing would otherwise undo.
		post   func(t *testing.T, tr *SessionTree)
		retire bool
		// says is what the VERDICT must spell out — the fact that decided
		// it, in the words the operator reads.
		says []string
		// line is whether the SWEEP prints that verdict. A keep inside the
		// grace is transient and silent; a keep the sweep already spoke
		// about in a landing line is not said twice (ADR 0058 D2).
		line bool
		// landSays is what the sweep prints INSTEAD of a keep line, on an
		// arm where the LANDING half speaks about this tree first and `said`
		// therefore swallows the keep. Set it and `line` must be false: one
		// tree, one line, and this is the arm that proves which line wins.
		landSays []string
	}{{
		// Verification 1. The 36 trees of the census, in one tree.
		name:   "a closed bead's landed tree with no session is retired",
		retire: true,
		says:   []string{"its bead is closed", "nothing here is unlanded", "session gone"},
		line:   true,
	}, {
		// The same tree with a meta still on disk, naming a workspace this
		// herdr does not hold: the prune's own proof of death (rangerhq-6bg7,
		// ADR 0011 §2), and the control for the two keeps below it.
		name: "a meta whose workspace herdr says is gone is proof of death",
		setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
			n27xvMeta(t, d, tr, "w404", "")
		},
		retire: true,
		says:   []string{"session gone"},
		line:   true,
	}, {
		// Fact 1. An open bead's tree is a seat a relaunch reuses.
		name:   "an open bead's tree is kept and the bead is named",
		status: "in_progress",
		says:   []string{"its bead is in_progress"},
	}, {
		// Fact 2, the shape ADR 0041 keeps on purpose.
		name: "one uncommitted path keeps the tree",
		setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
			write(t, filepath.Join(tr.Path, "scratch.txt"), "not committed\n")
		},
		says: []string{"has uncommitted work", "scratch.txt"},
		line: true,
	}, {
		// Fact 2, the 13 trees the census leaves to a human: the base holds
		// a commit carrying git's `-x` trailer for this one, which is
		// somebody's DECISION that it landed and not a measurement of what
		// the landing kept (ranger-base-as19).
		name: "a commit whose landing is only a -x trailer keeps the tree",
		setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
			commitOldIn(t, tr.Path, "adr.md", "status: accepted\n", "a-1: the fix")
			sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
			commitOldIn(t, repo, "adr.md", "status: accepted (reworded)\n",
				"a-1: the fix, by hand\n\n(cherry picked from commit "+sha+")")
		},
		says: []string{"holds 1 commit(s)"},
	}, {
		// Verification 3, and the reason this bead waited on
		// ranger-base-v2rj7: `main..<branch>` is ZERO here while the tree's
		// own HEAD holds the whole of the session's work. The VERDICT is
		// still the one that matters — the tree stands and the commit stays
		// referenced — but the sweep no longer reaches the retire with
		// nothing said: since ranger-base-vavx2 the landing half asks both
		// tips too (nothingToLand), so MergeSessionWork is called, reports
		// the strand and prescribes the `branch -f` that rescues it, and
		// `said` swallows the keep line behind it (ranger-base-edsiu, where
		// the two changes met).
		name: "a detached tree's work keeps it, and the commit stays referenced",
		setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
			mustGit(t, tr.Path, "checkout", "-q", "--detach")
			commitOldIn(t, tr.Path, "adr.md", "status: accepted\n", "a-1: off its own branch")
		},
		says:     []string{"holds 1 commit(s)"},
		landSays: []string{"did NOT reach", "branch -f "},
	}, {
		// Fact 3: herdr holds the workspace right now.
		name: "a session herdr still lists keeps the tree",
		setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
			n27xvMeta(t, d, tr, "w1", "")
			saveWSTo(t, fakeDirOf(t), []fakeWS{{
				WorkspaceID: "w1",
				Label:       d.App.WorkspaceLabel(SessionOfBranch(tr.Branch)),
			}})
		},
		says: []string{"workspace w1 is alive"},
		line: true,
	}, {
		// Fact 3: the meta was written against another server, so this
		// server's listing says nothing at all about it (rangerhq-8fq).
		name: "a meta this server cannot answer for keeps the tree",
		setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
			n27xvMeta(t, d, tr, "w404", "/tmp/somebody-elses/herdr.sock")
		},
		says: []string{"was written against", "this pass is talking to"},
		line: true,
	}, {
		// Fact 3's other belt: an empty listing looks exactly like
		// "everything died" and is also a server that just came up
		// (rangerhq-snd).
		name: "an empty herdr board keeps every tree",
		setUp: func(t *testing.T, d *Dispatcher, repo string, tr *SessionTree) {
			saveWSTo(t, fakeDirOf(t), nil)
		},
		says: []string{"lists no workspaces at all"},
		line: true,
	}, {
		// Fact 4. One touched file inside the git dir, which is what an
		// operator's `git status` in the tree leaves behind. It is a `post`
		// and not a `setUp` because the ageing below would take it back.
		name: "an index touched inside the grace keeps the tree, silently",
		post: func(t *testing.T, tr *SessionTree) {
			gd := mustGit(t, tr.Path, "rev-parse", "--absolute-git-dir")
			now := time.Now()
			if err := os.Chtimes(filepath.Join(gd, "index"), now, now); err != nil {
				t.Fatal(err)
			}
		},
		says: []string{"inside the", "grace"},
	}, {
		// The dial, spelled as the two reap graces are.
		name:  "retire_tree_after: off keeps every tree, silently",
		grace: "off",
		says:  []string{"`retire_tree_after:` is off"},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			status := c.status
			if status == "" {
				status = "closed"
			}
			d, repo, tr := n27xvFixture(t, status, c.grace)
			if c.setUp != nil {
				c.setUp(t, d, repo, tr)
			}
			n27xvQuiet(t, tr)
			if c.post != nil {
				c.post(t, tr)
			}
			head, _ := workHead(tr)

			grace := d.App.graceAfter("retire_tree_after", DefaultRetireTreeAfter, d.errWriter())
			v := retirable(tr, status, d.HB, grace)
			if v.retire != c.retire {
				t.Errorf("retirable = %+v, want retire=%v", v, c.retire)
			}
			for _, want := range c.says {
				if !strings.Contains(v.why, want) {
					t.Errorf("the verdict does not say %q: %q", want, v.why)
				}
			}

			// And the sweep, over the same tree: the predicate is advice
			// until this line acts on it. Nothing is re-aged between the
			// two — a settled tree stays quiet under both reads, and an arm
			// that stopped being true here would be saying that reading a
			// tree writes to it.
			d.landClosedTrees(repo)
			out := dispatcherOut(d)

			if !c.retire {
				if _, err := os.Stat(tr.Path); err != nil {
					t.Fatalf("the sweep removed a tree the predicate kept: %v\n%s", err, out)
				}
				if !branchExists(tr.Repo, tr.Branch) {
					t.Fatalf("the sweep deleted %s, which the predicate kept:\n%s", tr.Branch, out)
				}
				// What every keep is for: the work is still reachable.
				if head != "" {
					if _, err := git(tr.Repo, "rev-parse", "--verify", "--quiet", head+"^{commit}"); err != nil {
						t.Errorf("the kept tree's work %s is gone from the object store: %v", abbrevSHA(head), err)
					}
				}
				said := strings.Contains(out, "kept: ")
				if said != c.line {
					t.Errorf("the sweep said %v about this keep, want %v (ADR 0058 D2: the grace is silent, every other keep is said every pass):\n%s", said, c.line, out)
				}
				if c.line {
					for _, want := range c.says {
						if !strings.Contains(out, want) {
							t.Errorf("the sweep's keep line does not say %q:\n%s", want, out)
						}
					}
				}
				// The other way a keep is spoken for: the landing line got
				// there first. Asserted in the same breath as the silence
				// above, or "one tree, one line" is satisfied by a pass
				// that said nothing at all.
				for _, want := range c.landSays {
					if !strings.Contains(out, want) {
						t.Errorf("the sweep's landing line does not say %q — the keep it swallows is then the only record:\n%s", want, out)
					}
				}
				return
			}
			if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
				t.Errorf("the tree is still standing after a retire: %v\n%s", err, out)
			}
			if branchExists(tr.Repo, tr.Branch) {
				t.Errorf("%s survived the retire:\n%s", tr.Branch, out)
			}
			if !strings.Contains(out, "⌫") || !strings.Contains(out, tr.Branch) || !strings.Contains(out, "retired:") {
				t.Errorf("the pass did not say what it retired:\n%s", out)
			}
			// The registration too, or `posse worktrees` lists a directory
			// that is not there.
			if list := mustGit(t, repo, "worktree", "list", "--porcelain"); strings.Contains(list, tr.Path) {
				t.Errorf("the worktree registration outlived the tree:\n%s", list)
			}
		})
	}
}

// Verification 4: --dry-run is the operator's diagnostic, and a retire is
// the most destructive thing this sweep does. It says what it would take and
// takes nothing.
func TestSweepUnderDryRunSaysWhatItWouldRetireAndRetiresNothing(t *testing.T) {
	t.Parallel()
	d, repo, tr := n27xvFixture(t, "closed", "")
	n27xvQuiet(t, tr)
	d.DryRun = true

	d.landClosedTrees(repo)
	out := dispatcherOut(d)
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("--dry-run removed a session tree: %v\n%s", err, out)
	}
	if !branchExists(tr.Repo, tr.Branch) {
		t.Fatalf("--dry-run deleted %s:\n%s", tr.Branch, out)
	}
	if !strings.Contains(out, "would retire") || !strings.Contains(out, tr.Branch) {
		t.Errorf("--dry-run did not say what a real pass would retire:\n%s", out)
	}
}

// The whole point of the record, end to end: it is a PASS that retires the
// tree, not a human running a command. Through Run, so the sweep is reached
// where it actually runs.
func TestAPassRetiresTheTreeOfABeadNobodyIsWorkingOn(t *testing.T) {
	t.Parallel()
	d, _, tr := n27xvFixture(t, "closed", "")
	n27xvQuiet(t, tr)
	idleClaude(t, fakeDirOf(t))

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
		t.Errorf("a pass left the tree standing: %v\n%s", err, out)
	}
	if branchExists(tr.Repo, tr.Branch) {
		t.Errorf("a pass left %s behind:\n%s", tr.Branch, out)
	}
}

// The equivalence case the bead was filed from (ranger-base-wo980): a tree
// whose commits the base holds under OTHER shas printed `≡ … nothing here is
// unlanded` on every pass forever — 41 lines for one tree in the current
// rotated log. It is a one-pass event now: the sweep says ≡ once and takes
// the tree in the same pass.
func TestTheEquivalenceLineBecomesAOnePassEvent(t *testing.T) {
	t.Parallel()
	d, repo, tr := n27xvFixture(t, "closed", "")
	commitOldIn(t, tr.Path, "adr.md", "status: accepted\n", "a-1: the fix")
	sha := mustGit(t, tr.Path, "rev-parse", "HEAD")
	// The base moves first, or the pick rebuilds the identical commit object
	// and the base reaches it by SHA, which measures nothing
	// (ranger-base-g2xf's fixture).
	commitOldIn(t, repo, "moved.txt", "meanwhile\n", "main moved on")
	gitOld(t, repo, "cherry-pick", "-x", sha)
	n27xvQuiet(t, tr)

	d.landClosedTrees(repo)
	out := dispatcherOut(d)
	if !strings.Contains(out, "≡") {
		t.Errorf("the equivalence was not reported on the pass that retired it:\n%s", out)
	}
	if !strings.Contains(out, "retired:") {
		t.Errorf("the tree whose work is measured on the base was not retired:\n%s", out)
	}
	if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
		t.Errorf("the tree is still standing: %v\n%s", err, out)
	}
	// One line about it, not two: the ≡ said it, and a `kept:` line beside
	// it would be the same sentence in other words.
	if strings.Contains(out, "kept: ") {
		t.Errorf("the sweep said the same thing twice about one tree:\n%s", out)
	}
}

// n27xvStale ages the git dir's files and nothing else, which leaves the
// tree in the state fact 4 has to survive: the index is "racily clean"
// (its entries record mtimes no older than the index file itself), so the
// next `git status` in this tree — which is what fact 2's own read is —
// re-checks every entry and WRITES THE INDEX BACK.
func n27xvStale(t *testing.T, tr *SessionTree) {
	t.Helper()
	gd := mustGit(t, tr.Path, "rev-parse", "--absolute-git-dir")
	when := time.Now().Add(-n27xvAge)
	if err := filepath.WalkDir(gd, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		return os.Chtimes(p, when, when)
	}); err != nil {
		t.Fatal(err)
	}
}

// The order of the four facts is load-bearing and this is the pin on it: a
// tree whose index is stale is retired by the pass that finds it, not put
// back inside its own grace by the pass's own reading of it.
//
// Two decisions in retire.go are what make that true, and breaking either
// one reds this test. Fact 4 is read BEFORE fact 2, so the grace is measured
// on what somebody else wrote and not on what this pass just did. And the
// re-read under the launcher lock is facts 2 and 3 ONLY (retireHeldOrAlive)
// — a re-read that asked the whole predicate again would read back the index
// refresh that fact 2 had just written, refuse, and do it again on every
// pass forever: a retire that can never happen, arrived at in silence.
func TestARetireIsNotDefeatedByThePassesOwnReadOfTheTree(t *testing.T) {
	t.Parallel()
	d, repo, tr := n27xvFixture(t, "closed", "")
	n27xvStale(t, tr)

	// The fixture has to BE the trap or this pin measures nothing: fact 2's
	// read must be what moves the tree's last write to now.
	if q, ok := treeQuietFor(tr); !ok || q < time.Hour {
		t.Fatalf("the aged tree does not read as quiet (%s, ok=%v)", q, ok)
	}
	if held := treeHolds(tr); held != "" {
		t.Fatalf("the fixture holds work: %s", held)
	}
	if q, _ := treeQuietFor(tr); q > time.Minute {
		t.Skipf("this git did not rewrite the index over a racily-clean tree (quiet %s) — the trap this pin exists for is not reachable here", q)
	}

	n27xvStale(t, tr)
	d.landClosedTrees(repo)
	out := dispatcherOut(d)
	if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
		t.Errorf("the pass kept the tree it had just written to itself: %v\n%s", err, out)
	}
	if !strings.Contains(out, "retired:") {
		t.Errorf("the tree was not retired:\n%s", out)
	}
}

// The launcher lock, and what is re-read inside it (ADR 0058 D2, ADR 0011
// §2's reclaim rule). Evidence gathered before the lock is a fact about the
// instant it was read: between that read and `worktree remove` a `posse new`
// can create this very session's name again, and the tree the retire is
// about becomes a live seat.
//
// The window is planted from OUTSIDE the process under test, with
// rangerhq-rrg2's own lever: a workspace hidden from `workspace list`
// becomes visible the moment the launcher lock is HELD. So the pre-lock
// reading of fact 3 is "no row wears this name — the session is gone", and
// the reading taken under the lock is "it is listed right now". A retire
// that never took the lock, or took it and did not ask fact 3 again inside
// it, removes a live session's tree here; this pass must keep it and say so.
func TestTheRetireRereadsHerdrUnderTheLauncherLock(t *testing.T) {
	t.Parallel()
	d, repo, tr := n27xvFixture(t, "closed", "")
	n27xvQuiet(t, tr)
	fake := fakeDirOf(t)
	name := SessionOfBranch(tr.Branch)
	// No meta: the population this record exists for is a tree whose kill
	// took the meta away (rangerhq-09o2), so the listing is the only
	// evidence there is about the name.
	saveWSTo(t, fake, []fakeWS{
		{WorkspaceID: "w1", Label: d.App.WorkspaceLabel(name)},
		{WorkspaceID: "w9", Label: "somebody-elses-work"},
	})
	write(t, filepath.Join(fake, "hidden-from-list"), "w1\n")
	write(t, filepath.Join(fake, "unhide-when-locked"), LaunchLockPath(d.App)+"\n")

	// The plant has to be invisible BEFORE the lock or the pin measures
	// nothing: this is the reading the retire would act on.
	if gone, why := d.HB.sessionGone(name); !gone {
		t.Fatalf("the hidden workspace is already visible (%s) — the lever did not arm", why)
	}

	d.landClosedTrees(repo)
	out := dispatcherOut(d)
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("the tree of a session that appeared while the lock was being taken was removed: %v\n%s", err, out)
	}
	if !branchExists(tr.Repo, tr.Branch) {
		t.Fatalf("%s was deleted under a live session:\n%s", tr.Branch, out)
	}
	if !strings.Contains(out, "kept:") || !strings.Contains(out, "w1") {
		t.Errorf("the pass did not say which fact stopped the retire:\n%s", out)
	}
	// And the lever really did fire, so the arm is measuring the window and
	// not simply a listing that never changed.
	if _, err := os.Stat(filepath.Join(fake, "unhide-when-locked")); !os.IsNotExist(err) {
		t.Errorf("the unhide lever never fired: the launcher lock was not held while herdr was re-read (%v)", err)
	}
}

// A tree no bead record accounts for is ADR 0006's, and ADR 0058 D4 leaves
// it exactly where it was: nothing unattended acts on it, and the sweep says
// nothing new about it either.
func TestSweepNeverRetiresATreeNoRecordAccountsFor(t *testing.T) {
	t.Parallel()
	d, repo, tr := n27xvFixture(t, "closed", "")
	mustGit(t, repo, "config", "--unset", beadKey(tr.Branch))
	n27xvQuiet(t, tr)

	d.landClosedTrees(repo)
	out := dispatcherOut(d)
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("a tree with no bead record was retired: %v\n%s", err, out)
	}
	if strings.Contains(out, tr.Branch) {
		t.Errorf("a landed tree with no record is the listing's to report, not the sweep's:\n%s", out)
	}
}
