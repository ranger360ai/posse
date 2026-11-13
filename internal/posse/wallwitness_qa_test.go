//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-ctxeq — ADR 0086 D5: the Edit/Write rows are proved
// by a WITNESS at the launch, not by the tier being available on this host.
//
// Claim: for a PID with a whole-tree write deny at `cage: seatbelt`, the
// launch types the RENDERED prefix and tries the write the rule forbids —
// once in cwd and once in the session's git objects dir (D3). Both creates
// must fail, or the row leaves Realized with the path named and the launch
// refuses. And a probe that could not measure anything says so instead of
// passing.
//
// THE PAIR THAT MAKES THIS MEASURABLE, and it is the whole shape of this
// file: ONE fixture body under TWO roots, with nothing else varied.
//
//   - under `/var/tmp` (wwRoot): no grant in the writable set reaches it, so
//     the production render really walls the tree and the witness must read
//     WITNESSED. That is the live arm, and it runs unasked on whichever
//     mechanism this host has.
//   - under `/tmp` (wwTempRoot): `SeatbeltWritable` grants `/tmp` and TMPDIR
//     unconditionally (seatbelt.go) — on both mechanisms — so the same
//     render over the same PID leaves the tree WRITABLE, and the witness must
//     take the row away and name the path.
//
// Same production code, same PID, opposite verdicts, and the difference is
// the wall and not the test. That is what ADR 0086's own caveat about its
// "move main" row asked for: a fixture under `/tmp` measures nothing about a
// wall, so here it is the arm whose subject IS a wall that does not hold.
//
// WHY A NEGATIVE PROBE NEEDS ITS OWN PINS. Every other probe in this package
// passes when a command succeeds, so a probe that cannot run is loud. This
// one passes when a command FAILS: a stub binary that swallows its argv, a
// prefix that exits nonzero before the namespace, a target that is not there
// — each of them makes the create not happen, and a reading that said "the
// wall holds" over any of them would be a green row over a seat with no wall
// at all. So three of the arms below drive exactly those, and each must read
// NOT WITNESSED and never "".
//
// EVERY ARM RUNS IN A REAL LINKED WORKTREE AND IN A MAIN CHECKOUT (D6). R0
// escaped because no pin ever drove a whole-tree-deny PID in a linked
// worktree, where `<cwd>/.git` is a FILE; the objects dir this witness probes
// is in a different repository in those two shapes, and the fixture is built
// so the arms cannot tell which one they are in except by asking.

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// ─── the fixture ─────────────────────────────────────────────────────────────

// wwFixture is one launch: a repo, this session's linked worktree on its own
// branch, the App whose home renders the wall, and a PID with the one rule
// ADR 0086 is about.
type wwFixture struct {
	a          *App
	ag, open   *AgentFile
	rt         *Runtime
	root       string
	repo, tree string
}

// wwRoot is the fixture root for an arm whose wall must HOLD: a directory no
// grant reaches (wallUngrantedDir, wall_helpers_test.go), under a HOME that
// is a fixture so the home cache grants name one too.
func wwRoot(t *testing.T) string {
	t.Helper()
	wtqaHome(t) // HOME elsewhere: the home cache grants name a fixture home
	return wallUngrantedDir(t)
}

// wwTempRoot is the other half of the pair: a root INSIDE the blanket temp
// grant, where the same render leaves the tree writable. gitTempDir, so it is
// the tolerant root the init rule asks for and the grant is TMPDIR's own.
func wwTempRoot(t *testing.T) string {
	t.Helper()
	wtqaHome(t)
	return absResolve(gitTempDir(t))
}

// wwFixtureIn is the fixture body under a root the caller chose. One body,
// two roots, for wgFixtureIn's reason: a second copy of the worktree shape
// would be a fixture free to disagree with this one about what a session
// looks like, and the two arms' whole value is that they differ in the ROOT
// and in nothing else.
func wwFixtureIn(t *testing.T, root string) wwFixture {
	t.Helper()
	repo := sbMkdir(t, filepath.Join(root, "repo"))
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	sbWrite(t, filepath.Join(repo, "README.md"), "seed\n")
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	mustGit(t, repo, "pack-refs", "--all") // as a real checkout is

	// The launcher's shape: a linked worktree on `posse/<session>`, slash and
	// all, where `<cwd>/.git` is the FILE that refused R0's render.
	tree := filepath.Join(root, "trees", "s1")
	mustGit(t, repo, "worktree", "add", "-q", "-b", "posse/s1", tree)
	if dirs := LinkedGitDirs(tree); len(dirs) != 2 {
		t.Fatalf("no linked worktree was made, so half the arms measure the wrong shape: LinkedGitDirs(%s) = %v", tree, dirs)
	}

	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	f := wwFixture{a: a, root: root, repo: repo, tree: tree,
		// Not claude: ADR 0080 D19's config-dir demand is its own pin and not
		// a precondition of this one (bwlSeat's reason, one file over).
		rt: &Runtime{Name: "mycli", StateDirs: []string{sbMkdir(t, filepath.Join(root, "mystate"))}},
	}
	mem := sbMkdir(t, filepath.Join(a.Home, "personas", "witness"))
	sbMkdir(t, a.GatesDir("witness"))
	// `Edit` alone and not the triple: deniesFileWrite is an any-of union
	// (cageinner.go), so a PID spelling all three could not tell a wall that
	// reads the union from one that happens to match on `Write`.
	f.ag = &AgentFile{Name: "witness", Deny: []string{"Edit"}, MemoryDir: mem}
	f.open = &AgentFile{Name: "witness", MemoryDir: mem}
	return f
}

// wwClaim is the parity matrix as `wholeTreeWriteWall`'s seatbelt arm leaves
// it, which is the input D5 judges: the PID's rule, realized by the claim
// that the tier is available here.
//
// Built by hand and asserted against the real arm by
// TestQAWallWitnessJudgesTheRowParityActuallyWrites below, so these arms can
// drive one door without carrying a whole CheckParity fixture — and cannot
// drift from the row parity really writes, which is the only way a pin over
// a hand-built matrix goes quietly wrong.
func wwClaim(cage string, gates ...string) *Parity {
	p := &Parity{Runtime: "mycli", Cage: cage, Tier: TierStrong, Realized: map[string]RealizedGate{}}
	for _, g := range gates {
		p.Realized[g] = RealizedGate{Class: Enforced, Detail: wholeTreeSeatbeltClaim}
	}
	return p
}

// wwSkipUnlessTheWallIsReal gates every arm whose measurement is a wall this
// process puts up. Three questions and not one, the way wall.go keeps them
// apart: does this host have a mechanism, does it provide the tier, and can
// THIS PROCESS apply it. NOTHING MEASURED on each, because a skip that read
// like a pass is this file's own subject.
func wwSkipUnlessTheWallIsReal(t *testing.T) {
	t.Helper()
	if m := WallMechanism(); m == "" {
		t.Skip("NOTHING MEASURED: this GOOS has no wall mechanism, so no launch here renders one (ADR 0080 D16)")
	}
	if !AvailableCages[CageSeatbelt] {
		t.Skip("NOTHING MEASURED: this host does not provide the seatbelt tier, so there is no wall to witness")
	}
	if why := wallApplyRefusal(); why != "" {
		t.Skip("NOTHING MEASURED: this process cannot put this host's wall up (" + why + "), which is the abstention arm's subject and not this one's")
	}
	// The sandbox-exec half of the same question, by the door every other
	// sandbox-probing file in this package goes through — it reads a written
	// package func var, so it is named here and not re-derived
	// (TestQAEverySandboxProbeFileIsGated).
	if WallMechanism() == WallSandboxExec {
		sbSkipUnlessSandboxable(t)
	}
}

// wwNoWitnessFileLeft is the assertion every arm makes, pass or fail: the
// witness creates and the witness cleans up. A probe that leaves its file in
// a session tree has written into the tree of a PID whose whole rule is that
// it cannot — and it would show up as an untracked file in the seat's own
// `git status`.
func wwNoWitnessFileLeft(t *testing.T, root string) {
	t.Helper()
	var left []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasPrefix(d.Name(), wallWitnessFile) {
			left = append(left, p)
		}
		return nil
	})
	if len(left) > 0 {
		t.Errorf("the witness left its own file behind:\n  %s\nADR 0086 D5: a create that succeeds is removed, and one that fails never existed", strings.Join(left, "\n  "))
	}
}

// ─── the live arm: a real wall, witnessed ────────────────────────────────────

// D5's happy path on the real mechanism of this host, through the production
// render. It runs unasked wherever the tier is real — bwrap on the D1 host,
// sandbox-exec on a darwin seat — which is what makes "the real wall must
// pass on Linux and on darwin" one arm and not two claims nobody checks.
//
// THE CONTROL IS THE SAME CALL WITH NO WALL IN FRONT OF IT. A refusal proves
// nothing unless the identical create lands when the prefix is empty, and
// that is also what proves the target was really there: the whole file would
// pass over a probe that always reported "refused" without it.
func TestLiveWallWitnessHoldsUnderTheRenderedWall(t *testing.T) {
	// No t.Parallel: wwRoot sets TMPDIR and HOME for this process.
	wwSkipUnlessTheWallIsReal(t)
	f := wwFixtureIn(t, wwRoot(t))

	for _, tc := range []struct{ what, cwd string }{
		{"a linked worktree, the shape every dispatched session has", f.tree},
		{"the operator's main checkout", f.repo},
	} {
		t.Run(tc.what, func(t *testing.T) {
			w, err := f.a.RenderWall(f.ag, f.rt, tc.cwd)
			if err != nil {
				t.Fatalf("the wall for a whole-tree-deny PID in %s cannot be rendered: %v — that is R0, and ADR 0086 D3 is supposed to have cleared it", tc.cwd, err)
			}
			targets := wallWitnessTargets(tc.cwd)
			if len(targets) != 2 {
				t.Fatalf("the witness has %d targets in %s (%v) — D5 is cwd AND the session's git objects dir, and an arm over one of them says nothing about the capability D3 removed", len(targets), tc.cwd, targets)
			}
			for _, target := range targets {
				// The control first: with no wall, this create lands. So the
				// target is real, writable to this process, and the refusal
				// below is the wall's.
				why, unmeasured := wallWitnessRow(WallRender{Mechanism: "none"}, target)
				if why == "" || unmeasured != "" {
					t.Fatalf("the control did not land a create in %s with NO wall in front of it (why=%q unmeasured=%q) — every refusal in this arm would then be measuring something that is not a wall", target, why, unmeasured)
				}
				// And the same create behind the rendered prefix.
				why, unmeasured = wallWitnessRow(w, target)
				switch {
				case unmeasured != "":
					t.Errorf("the witness measured NOTHING over %s under the real %s wall:\n%s", target, w.Mechanism, unmeasured)
				case why != "":
					t.Errorf("the real %s wall ALLOWED the create this PID's rule forbids:\n%s", w.Mechanism, why)
				}
			}
			// And the door: the row parity claimed is upgraded in place, with
			// D5's own words on it.
			p := wwClaim(CageSeatbelt, "Edit")
			f.a.applyWallWitness(p, f.ag, f.rt, tc.cwd)
			want := wholeTreeSeatbeltClaim + " (" + w.Mechanism + "), witnessed at launch"
			if got := p.Realized["Edit"].Detail; got != want {
				t.Errorf("the witnessed row reads %q, want %q (ADR 0086 D5)", got, want)
			}
			if len(p.Degraded) != 0 || len(p.Unrealized) != 0 {
				t.Errorf("a wall that holds degrades nothing: %v %v", p.Degraded, p.Unrealized)
			}
			if p.Realized["Edit"].Class != Enforced {
				t.Errorf("the witness must not reclass the row: %q", p.Realized["Edit"].Class)
			}
		})
	}
	wwNoWitnessFileLeft(t, f.root)
}

// ─── the other half of the pair: a wall that does not hold ───────────────────

// The same production render over the same PID, one root over. `/tmp` is in
// every writable set this package builds, so the tree is writable under the
// wall, the create LANDS, and D5 says what must happen: the file is removed,
// the rule is unrealized with the path named, and the launch refuses.
//
// This is the fixture whose wall does not hold, and it is a REAL wall rather
// than a stub one — which is the only way to pin that the finding names a
// path an operator can go and look at.
func TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate(t *testing.T) {
	// No t.Parallel: wwTempRoot sets HOME for this process.
	wwSkipUnlessTheWallIsReal(t)
	f := wwFixtureIn(t, wwTempRoot(t))

	for _, tc := range []struct{ what, cwd string }{
		{"a linked worktree", f.tree},
		{"a main checkout", f.repo},
	} {
		t.Run(tc.what, func(t *testing.T) {
			p := wwClaim(CageSeatbelt, "Edit", "Write")
			f.a.applyWallWitness(p, f.ag, f.rt, tc.cwd)
			for _, gate := range []string{"Edit", "Write"} {
				if _, ok := p.Realized[gate]; ok {
					t.Errorf("%s is still a REALIZED row over a wall that allows the create: %q — Realized must not keep a row for a rule nothing holds (ADR 0086 D5)", gate, p.Realized[gate].Detail)
				}
				if !wwSays(p.Unrealized, gate) {
					t.Errorf("%s is not in Unrealized: %v", gate, p.Unrealized)
				}
				if !wwSays(p.Degraded, gate) {
					t.Errorf("%s is not in Degraded, so the launch does not refuse: %v", gate, p.Degraded)
				}
			}
			// The finding's load-bearing words: the path, the mechanism, the
			// artifact, and that the row's old claim was about the HOST.
			why := strings.Join(p.Unrealized, "\n")
			for _, want := range []string{
				AbbrevHome(tc.cwd), WallMechanism(), wallWitnessFile,
				"is a claim about the host and not about this wall", "ADR 0086 D5",
			} {
				if !strings.Contains(why, want) {
					t.Errorf("the finding must carry %q so an operator can go and look at it:\n%s", want, why)
				}
			}
		})
	}
	wwNoWitnessFileLeft(t, f.root)
}

// ─── the false pass: a probe that measured nothing ───────────────────────────

// THE ARM THIS FILE EXISTS FOR. The witness passes when a command FAILS, so
// everything that stops the create from being attempted looks exactly like a
// wall holding. Three such prefixes, and each must read NOT WITNESSED:
//
//   - a binary that swallows its argument and exits 0 — the shape a fixture
//     with a stub `bwrap` has, and the one that returns success;
//   - a binary that exits nonzero before running anything — a namespace the
//     kernel refused, an args file that cannot be read;
//   - a prefix naming nothing that resolves at all.
//
// None of them is exotic: the first is in this very package
// (bwrapprepauthority_qa_test.go stubs /usr/bin/bwrap), and the second is
// what every bwrap failure on an L4 host looks like.
func TestQAWallWitnessAbstainsWhereNothingRanBehindThePrefix(t *testing.T) {
	// No t.Parallel: wwTempRoot sets HOME for this process.
	f := wwFixtureIn(t, wwTempRoot(t))
	for _, tc := range []struct{ what, prefix string }{
		{"a stub that swallows the command and exits 0", "/usr/bin/env true "},
		{"a wall that exits nonzero before running anything", "/usr/bin/env false "},
		{"a prefix that resolves to nothing", "/nonexistent/posse-no-such-wall "},
		// And the two that defeated the control while it was a SUBSTRING
		// (ranger-base-sjjf4 finding 1): the marker is a literal inside the
		// argv the prefix is handed, so a prefix that PRINTS its argument
		// prints the marker without running anything — and the row read "the
		// wall refused the create" over a create nobody attempted, which is
		// the one false pass this file exists to prevent. Both spellings,
		// because they print it differently: `echo` puts the whole inner
		// command on one line, `printf %s\n` one argument per line.
		{"a prefix that echoes its argv instead of running it", "/usr/bin/echo "},
		{"a prefix that prints its argv one argument per line", "/usr/bin/env printf '%s\\n' "},
	} {
		t.Run(tc.what, func(t *testing.T) {
			for _, target := range []string{f.tree, f.repo} {
				w := WallRender{Mechanism: WallBwrap, Artifact: filepath.Join(f.root, "bwrap.args"), Prefix: tc.prefix}
				why, unmeasured := wallWitnessRow(w, target)
				if unmeasured == "" {
					t.Fatalf("the witness read %q as a measurement over %s (why=%q) — a prefix that runs nothing refuses every write there is, and reading that as `the wall holds` is a green row over a seat with no wall (ADR 0086 D5)", tc.prefix, target, why)
				}
				if !strings.Contains(unmeasured, "never printed its marker") {
					t.Errorf("the abstention must say the command never ran: %s", unmeasured)
				}
			}
		})
	}
	// And at the door: a prefix that measured nothing leaves the row realized
	// and degrades nobody — a launch must not be refused by a check that did
	// not run (ADR 0013 §4, ranger-base-heur) — while never reading as a pass.
	wwSkipUnlessTheWallIsReal(t)
	wwStubApplyRefusal(t, "the kernel refused the apply (fixture)")
	p := wwClaim(CageSeatbelt, "Edit")
	f.a.applyWallWitness(p, f.ag, f.rt, f.tree)
	row := p.Realized["Edit"].Detail
	switch {
	case len(p.Degraded) != 0 || len(p.Unrealized) != 0:
		t.Errorf("an abstention degraded the launch: %v %v", p.Degraded, p.Unrealized)
	case !strings.Contains(row, "NOT WITNESSED"):
		t.Errorf("the row must say the wall was not witnessed, and why: %q", row)
	case strings.Contains(row, "witnessed at launch"):
		t.Errorf("an abstention must not read like the pass: %q", row)
	}
	// Asked BEFORE the render, which is the other half of the claim: a
	// process that cannot put the namespace up should not be writing the args
	// file and making the host-side prep to find out (applyRecordReach's own
	// reason, one file over).
	//
	// Stated over the PROBE's own render dir and no longer over the gates dir
	// (ranger-base-6s5mo): the witness renders under reportWallDir now, so a
	// gates dir with no artifact in it is true of every run and would say
	// nothing about this one. The live dir gets its own arm, where the
	// subject is the probe that RAN —
	// TestLiveWallWitnessProbeDoesNotWriteTheLiveWall.
	dir, err := f.a.reportWallDir("witness")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, wallArtifactName(WallMechanism()))); err == nil {
		t.Errorf("the abstention rendered the wall anyway — rendering WRITES, and the apply refusal is asked first for exactly that reason")
	}
	wwNoWitnessFileLeft(t, f.root)
}

// wwStubApplyRefusal makes this process one that cannot put the wall up, at
// whichever mechanism this host has. The two halves are written package func
// vars (seatbelt.go, bwrap.go) and this is the spelling their own pins use
// (reachability_qa_test.go, wallmechanism_qa_test.go).
func wwStubApplyRefusal(t *testing.T, why string) {
	t.Helper()
	sb, bw := sandboxApplyRefusal, bwrapApplyRefusal
	sandboxApplyRefusal = func() string { return why }
	bwrapApplyRefusal = func() string { return why }
	t.Cleanup(func() { sandboxApplyRefusal, bwrapApplyRefusal = sb, bw })
}

// ─── which wall could not be put up, and which build has no arm for it ──────

// wallWitnessUnmeasured's TWO sentences, one per mechanism, and the one this
// host does not have driven from the other one (ranger-base-sjjf4 finding 2,
// out of ranger-base-7urt1).
//
// The function is two whole sentences rather than one with the binary
// substituted in because what the abstention IS differs — applying a profile
// on one host, unsharing a mount namespace on the other — and so does the
// clause that says the launch is unaffected, which is the part an operator
// acts on. Its ONE call site keys on WallMechanism(), so one of the two was
// dead on every host and NOTHING read either: the abstention arm above
// asserts `NOT WITNESSED` and never which wall.
//
// THROUGH applyWallWitness AND NOT THROUGH THE FUNCTION, because the thing
// that escaped is the row an operator reads: wallWitnessAbstain wraps the
// sentence in the availability claim and the mechanism, and the door is what
// decides which sentence a mechanism gets. wallMechanismForTest is how every
// other mechanism-keyed LAUNCH arm in this package reaches the other host's
// arm (bwrapprepauthority_qa_test.go, walllaunchmechanism_qa_test.go), and
// wall.go's own comment claims exactly that drivability for this function.
//
// The apply is stubbed REFUSED, so neither arm renders anything and neither
// needs this host's wall to be real — the one precondition left is the tier,
// because wallRendered is what puts a launch behind an L2 wall at all.
func TestQAWallWitnessAbstentionNamesTheWallThisProcessCannotPutUp(t *testing.T) {
	// No t.Parallel: wwTempRoot and wallMechanismForTest write process state.
	if !AvailableCages[CageSeatbelt] {
		t.Skip("NOTHING MEASURED: this host does not provide the seatbelt tier, so no launch here is behind an L2 wall at all (wallRendered)")
	}
	f := wwFixtureIn(t, wwTempRoot(t))
	for _, tc := range []struct{ goos, mech, says, unaffected, notTheOther string }{
		{"linux", WallBwrap, "put up a bwrap mount namespace", "its bwrap is typed into a pane", "apply a seatbelt profile"},
		{"darwin", WallSandboxExec, "apply a seatbelt profile", "its sandbox-exec is typed into a herdr pane", "put up a bwrap mount namespace"},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			wallMechanismForTest(t, tc.goos)
			if got := WallMechanism(); got != tc.mech {
				t.Fatalf("a launch pinned to %s reads mechanism %q, want %s — the fixture is not driving the arm it names", tc.goos, got, tc.mech)
			}
			wwStubApplyRefusal(t, "the kernel refused the apply (fixture)")
			p := wwClaim(CageSeatbelt, "Edit")
			f.a.applyWallWitness(p, f.ag, f.rt, f.tree)
			row := p.Realized["Edit"].Detail
			if len(p.Degraded) != 0 || len(p.Unrealized) != 0 {
				t.Errorf("an abstention degraded the launch: %v %v", p.Degraded, p.Unrealized)
			}
			for _, want := range []string{"NOT WITNESSED", "(" + tc.mech + ")", tc.says, tc.unaffected, "the kernel refused the apply (fixture)"} {
				if !strings.Contains(row, want) {
					t.Errorf("the %s abstention does not say %q — an operator reads this row to decide whether to re-run `posse gates` outside the cage, and a sentence about the other host's mechanism tells them to do the wrong thing:\n%s", tc.mech, want, row)
				}
			}
			if strings.Contains(row, tc.notTheOther) {
				t.Errorf("the %s abstention carries the OTHER mechanism's sentence (%q), so the two arms have collapsed into one:\n%s", tc.mech, tc.notTheOther, row)
			}
			if strings.Contains(row, "witnessed at launch") {
				t.Errorf("an abstention must not read like the pass: %q", row)
			}
		})
	}
	wwNoWitnessFileLeft(t, f.root)
}

// wallApplyRefusalOn's THIRD arm: a mechanism this build renders no wall for.
// Its only caller hands it WallMechanism(), which answers one of two
// mechanisms or "", so nothing ever drove the default — and the sentence is
// one a ROW prints, so a reader will meet it on a host posse was not built
// for (ranger-base-sjjf4 finding 2).
//
// Asked of the function and not through a launch, which is the whole of the
// honest reading: there is no host here to put behind a mechanism that does
// not exist, and wallMechanismForTest pins a GOOS rather than a mechanism.
func TestQAWallApplyRefusalAnswersAMechanismThisBuildHasNoArmFor(t *testing.T) {
	t.Parallel()
	for _, mech := range []string{"", "no-such-mechanism"} {
		why := wallApplyRefusalOn(mech)
		for _, want := range []string{"this build renders no wall for mechanism", strconv.Quote(mech), runtime.GOOS, "ADR 0080 D16/D18"} {
			if !strings.Contains(why, want) {
				t.Errorf("wallApplyRefusalOn(%q) does not say %q — a mechanism with no arm is a wall nothing puts up, and the row that says so has to name which mechanism and which host:\n%s", mech, want, why)
			}
		}
	}
}

// The baseline, which is the other way a create fails for a reason that is
// not the wall: a target this process cannot write at all. A create into a
// directory that is not there fails with ENOENT, and ENOENT is not a wall.
func TestQAWallWitnessAbstainsWhereTheBaselineCreateCannotLand(t *testing.T) {
	t.Parallel()
	gone := filepath.Join(t.TempDir(), "no-such-dir")
	why, unmeasured := wallWitnessRow(WallRender{Mechanism: "none"}, gone)
	if unmeasured == "" {
		t.Fatalf("a target posse cannot write outside any wall was read as a measurement (why=%q) — the witness is a create that must FAIL, so a target like this makes it fail for a reason that is not this PID's rule", why)
	}
	if !strings.Contains(unmeasured, "outside any wall") {
		t.Errorf("the abstention must say the baseline is what failed: %s", unmeasured)
	}
}

// R0's own shape: the row is printed because the TIER is available, and the
// wall cannot be built at all. That is the disagreement ADR 0086 found by
// running the real render, and a rule realized by a wall nothing can put up
// is realized by nothing.
//
// The render is broken mechanism-neutrally — the gates dir cannot be made,
// because its parent is a FILE — so both arms of RenderWall fail at their
// first line and the arm runs on either host. R0's own cause (a grant on a
// path that is a file) refuses the bwrap render and renders fine on darwin,
// so it would have been a Linux-only pin for a row both mechanisms write.
func TestQAWallWitnessTakesTheRowAwayWhenTheWallCannotBeRenderedAtAll(t *testing.T) {
	// No t.Parallel: wwTempRoot sets HOME for this process.
	wwSkipUnlessTheWallIsReal(t)
	f := wwFixtureIn(t, wwTempRoot(t))
	blocked := filepath.Join(f.root, "blocked")
	sbWrite(t, blocked, "a file where the home should be\n")
	broken := NewAppAt(filepath.Join(blocked, "home"))

	p := wwClaim(CageSeatbelt, "Edit")
	broken.applyWallWitness(p, f.ag, f.rt, f.tree)
	if _, ok := p.Realized["Edit"]; ok {
		t.Errorf("the row survived a wall that cannot be rendered: %q", p.Realized["Edit"].Detail)
	}
	why := strings.Join(p.Unrealized, "\n")
	for _, want := range []string{"cannot be rendered", "realized by nothing", "ADR 0086 D5, R0"} {
		if !strings.Contains(why, want) {
			t.Errorf("the finding must carry %q:\n%s", want, why)
		}
	}
	if !wwSays(p.Degraded, "Edit") {
		t.Errorf("a wall that cannot be rendered must degrade the launch: %v", p.Degraded)
	}
	// The control: the SAME call over the same fixture with a home that
	// works renders, so what this arm varied is the render and nothing else.
	q := wwClaim(CageSeatbelt, "Edit")
	f.a.applyWallWitness(q, f.ag, f.rt, f.tree)
	if len(q.Unrealized) != 0 && strings.Contains(strings.Join(q.Unrealized, "\n"), "cannot be rendered") {
		t.Fatalf("the control cannot render either, so the arm above measures nothing: %v", q.Unrealized)
	}
	wwNoWitnessFileLeft(t, f.root)
}

// ─── the targets ─────────────────────────────────────────────────────────────

// D5's two targets, in both tree shapes. The objects dir is the half that
// says this is D3's rule and not a statement about cwd: with `objects`
// writable, a seat that cannot edit a file still mints a commit by plumbing
// (W14/W15), and in a linked worktree that directory is not under cwd at all.
func TestQAWallWitnessTargetsAreTheTreeAndItsObjectStore(t *testing.T) {
	// No t.Parallel: wwTempRoot sets HOME for this process.
	f := wwFixtureIn(t, wwTempRoot(t))
	common := absResolve(LinkedGitDirs(f.tree)[1])

	for _, tc := range []struct {
		what, cwd string
		want      []string
	}{
		{"a linked worktree: the objects dir is the main checkout's", f.tree,
			[]string{absResolve(f.tree), filepath.Join(common, "objects")}},
		{"a main checkout: it is inside cwd", f.repo,
			[]string{absResolve(f.repo), filepath.Join(absResolve(f.repo), ".git", "objects")}},
	} {
		t.Run(tc.what, func(t *testing.T) {
			got := wallWitnessTargets(tc.cwd)
			for i := range got {
				got[i] = absResolve(got[i])
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("wallWitnessTargets(%s) =\n  %s\nwant\n  %s", tc.cwd, strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "))
			}
		})
	}

	// Not a repo at all: the tree, and nothing invented beside it.
	plain := t.TempDir()
	if got := wallWitnessTargets(plain); len(got) != 1 || absResolve(got[0]) != absResolve(plain) {
		t.Errorf("wallWitnessTargets over a directory that is no repo = %v, want just the directory", got)
	}
	if got := sessionGitObjectsDir(plain); got != "" {
		t.Errorf("sessionGitObjectsDir over a directory that is no repo = %q", got)
	}
	// And a target that is NOT THERE is dropped rather than probed: a create
	// into a missing directory fails with ENOENT, which this probe would read
	// as the wall refusing it.
	if err := os.RemoveAll(filepath.Join(common, "objects")); err != nil {
		t.Fatal(err)
	}
	if got := wallWitnessTargets(f.tree); len(got) != 1 {
		t.Errorf("an objects dir that is not there is still a target: %v — a create into a missing directory fails for a reason that is not a wall", got)
	}
}

// ─── the door, and the rows it must leave alone ──────────────────────────────

// The witness keys on the ROW parity wrote, so this is the pin that keeps the
// hand-built matrix above honest: the real arm's Detail for a real PID at the
// seatbelt tier IS `wholeTreeSeatbeltClaim`, in both of ADR 0014 §1's
// spellings. If parity ever says it differently, every arm in this file would
// quietly stop finding a row to judge — and that is a witness that measures
// nothing while printing nothing.
func TestQAWallWitnessJudgesTheRowParityActuallyWrites(t *testing.T) {
	// No t.Parallel: it reads AvailableCages, which other tests write.
	if !AvailableCages[CageSeatbelt] {
		t.Skip("NOTHING MEASURED: this host does not provide the seatbelt tier, so parity writes no such row")
	}
	a := cageApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"Edit", "Write", "NotebookEdit", "Edit(**)"} {
		ag := cageAgent(t, a, "cage: seatbelt\ndeny: ["+rule+"]\n")
		p := a.CheckParity(ag, rt, CageSeatbelt, TierStrong)
		if got := p.Realized[rule].Detail; got != wholeTreeSeatbeltClaim {
			t.Errorf("parity's whole-tree row for %s reads %q, and the witness looks for %q — one const, or D5 judges a row nobody writes", rule, got, wholeTreeSeatbeltClaim)
		}
		if rows := wallWitnessRows(&p); len(rows) != 1 || rows[0] != rule {
			t.Errorf("wallWitnessRows over that matrix = %v, want [%s]", rows, rule)
		}
	}
}

// What the witness must NOT touch. Each of these rows is realized by
// something that is not this launch's wall, or by nothing at all, and a
// witness that spoke about any of them would be claiming a measurement it
// did not take.
func TestQAWallWitnessSaysNothingAboutARowItDidNotClaim(t *testing.T) {
	// No t.Parallel: wwTempRoot sets HOME for this process.
	f := wwFixtureIn(t, wwTempRoot(t))
	// This fixture's wall ALLOWS the create (it is under /tmp), so if the
	// witness runs at all in any arm below it will be loud about it — which
	// is what makes these arms assertions and not hopes.
	for _, tc := range []struct {
		what  string
		cage  string
		rt    *Runtime
		rows  map[string]RealizedGate
		stays bool
	}{
		{"another tier's row: the mount boundary realized it, not this wall", CageContainer, f.rt,
			map[string]RealizedGate{"Edit": {Class: Enforced, Detail: "L4 mount boundary (repo mounted :ro)"}}, true},
		{"a runtime that cages itself: posse renders no wall to type", CageSeatbelt, &Runtime{Name: "codex", SelfSandbox: true},
			map[string]RealizedGate{"Edit": {Class: Enforced, Detail: wholeTreeSeatbeltClaim}}, true},
		{"a path-scoped subtree deny: a different row and a different grant", CageSeatbelt, f.rt,
			map[string]RealizedGate{"Edit(docs/**)": {Class: Enforced, Detail: "L2 trailing deny (subpath docs)"}}, true},
		{"a rule parity already left unrealized", CageSeatbelt, f.rt, map[string]RealizedGate{}, true},
	} {
		t.Run(tc.what, func(t *testing.T) {
			p := &Parity{Runtime: "mycli", Cage: tc.cage, Tier: TierStrong, Realized: map[string]RealizedGate{}}
			for k, v := range tc.rows {
				p.Realized[k] = v
			}
			f.a.applyWallWitness(p, f.ag, tc.rt, f.tree)
			for k, v := range tc.rows {
				if got := p.Realized[k]; got != v {
					t.Errorf("the witness rewrote a row it did not claim: %s was %+v, now %+v", k, v, got)
				}
			}
			if len(p.Degraded) != 0 || len(p.Unrealized) != 0 {
				t.Errorf("the witness degraded a launch whose rows it does not judge: %v %v", p.Degraded, p.Unrealized)
			}
		})
	}
	// No cwd at all: `posse gates <persona>` with no directory asks the
	// dir-independent matrix, and nothing here may run.
	p := wwClaim(CageSeatbelt, "Edit")
	f.a.applyWallWitness(p, f.ag, f.rt, "")
	if got := p.Realized["Edit"].Detail; got != wholeTreeSeatbeltClaim {
		t.Errorf("the witness ran without a launch dir: %q", got)
	}
	wwNoWitnessFileLeft(t, f.root)
}

// ─── the wiring ──────────────────────────────────────────────────────────────

// D5 is a LAUNCH observable, so the door has to be the one a launch goes
// through. CreateSession and `posse gates` both reach CheckParityIn; a
// witness wired only into a function nobody calls is a wall whose record
// still says `L2 seatbelt` because the tier is available — the exact row ADR
// 0086 D5 replaced.
func TestQAWallWitnessReachesTheLaunchThroughCheckParityIn(t *testing.T) {
	// No t.Parallel: wwRoot sets TMPDIR and HOME for this process.
	wwSkipUnlessTheWallIsReal(t)
	f := wwFixtureIn(t, wwRoot(t))
	p := f.a.CheckParityIn(f.ag, f.rt, CageSeatbelt, TierStrong, f.tree)
	if got := p.Realized["Edit"].Detail; !strings.Contains(got, "witnessed at launch") {
		t.Errorf("CheckParityIn's Edit row reads %q — the witness is not on the launch path, so a launch's record names a wall nobody typed (ADR 0086 D5)\n%s", got, p.String())
	}
	// And the dir-independent matrix is untouched: a persona-shaped statement
	// has no cwd to witness in, and must not pretend otherwise.
	if got := f.a.CheckParity(f.ag, f.rt, CageSeatbelt, TierStrong).Realized["Edit"].Detail; got != wholeTreeSeatbeltClaim {
		t.Errorf("CheckParity (no dir) reads %q, want the bare claim: the witness is directory-aware and this matrix has no directory", got)
	}
	wwNoWitnessFileLeft(t, f.root)
}

// wwSays reports whether any line of a parity list is about this gate.
func wwSays(lines []string, gate string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, gate+" — ") {
			return true
		}
	}
	return false
}
