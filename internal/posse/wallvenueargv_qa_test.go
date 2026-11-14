//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-wplhv — ADR 0097 D1: the venue is the judge's cage PLUS EXACTLY
// THREE THINGS, rendered by the same function through a flag.
//
// THE CLAIM, in the ADR's own words: "A QA pin at the real call site asserts
// that the venue's argv, minus the judge's, is those additions and nothing
// else." Both halves of that are load-bearing and this pin reads both:
//
//   - ADDED: the three things, and nothing beside them. A fourth flag in the
//     venue is a capability nobody priced — D1's third bullet is a careful
//     argument that ONE axis (root in a user namespace of its own) is weaker
//     than a seat, and an addition the ADR does not name voids it.
//   - REMOVED: nothing. "It is never weaker than the judge" (D1, first
//     bullet) is the sentence that lets landed code into this cage at all. A
//     venue that dropped `--unshare-net`, a hide or a credential mask would
//     be a wall nobody reviewed, and the removal is the direction no reader
//     would think to check.
//
// WHY A MULTISET AND NOT A PREFIX. The additions do not all land at the end:
// the two mounts go in before the network flag and the `--setenv` pair comes
// out of the env loop at the tail, so a positional expectation would be a
// second copy of the render's own order. A multiset difference answers
// exactly the ADR's question — which tokens appear in one and not the other,
// and how many times — in both directions at once, and it cannot be satisfied
// by a reordering that a later bead introduces for its own reasons. The ORDER
// that matters (the three additions' own adjacency, since bwrap applies mounts
// in argv order) is asserted separately, as runs.
//
// WHY THE SAME CHILD PATH IS HANDED TO BOTH. In production the judge runs
// `<run>/judge` and the wall check runs `<run>/wall-check` (landedRule.Extract),
// so their argvs differ in the last element too. That element is the CHILD and
// not the wall: it sits after `--`, nothing in the cage depends on it, and
// including it here would be this pin asserting a fact about which script runs
// while claiming to assert a fact about the wall. The call-site arm below is
// what holds the child paths apart, and it is the arm that reads what the
// production call sites actually pass.
//
// THE MUTANTS THIS WAS DRIVEN AGAINST are listed in
// docs/notes.d/ranger-base-wplhv.md, one change at a time, with what each one
// printed.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// wallVenueExpectedAdditions is ADR 0097 D1's three, as argv tokens: a pid
// namespace with a procfs over it, the private /var/tmp bind, and the
// environment variable as the `--setenv` pair the render emits for it.
//
// Spelled from the SAME constants the render uses, deliberately: what this
// pin is for is the SET of additions, and a literal copy of each flag here
// would red on a rename that changed nothing about the wall. The spellings
// themselves are held to ADR 0097 by TestQAWallVenueContractLiteralsAreADR0097s
// (the environment variable) and by the ADR's own D1 text (the two flags),
// which is why this file quotes neither.
func wallVenueExpectedAdditions(run string) []string {
	return []string{
		wallVenuePidNS, wallVenueProc, wallVenueProcMount,
		"--bind", filepath.Join(run, wallVenueVarTmpName), wallVenueVarTmpMount,
		"--setenv", wallVenueEnvVar, "1",
	}
}

// wallVenueArgvDelta is the multiset difference both ways: tokens `to` has
// that `from` does not, and tokens `from` has that `to` does not, each as many
// times over as the counts differ. Sorted, so a table can name them.
func wallVenueArgvDelta(from, to []string) (added, removed []string) {
	count := map[string]int{}
	for _, a := range to {
		count[a]++
	}
	for _, a := range from {
		count[a]--
	}
	for a, n := range count {
		for ; n > 0; n-- {
			added = append(added, a)
		}
		for ; n < 0; n++ {
			removed = append(removed, a)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func TestQAWallVenueArgvIsTheJudgesPlusADR0097sThree(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := newLandedCageFixture(t)
	// ONE child path for both renders — see the header. The run dir and the
	// write set are the production ones (landedCheckWritable's, less the
	// worktree git dir the fixture has no repo for), and so is the env.
	child := filepath.Join(f.run, "judge")
	writable := []string{f.run}
	env := landedRunEnv(f.run)

	judge, err := boxLandedCage(&App{}, landedCageJudge, child, f.tree, writable, env)
	if err != nil {
		t.Skip("NOTHING MEASURED: no landed-check cage renders on this host: " + err.Error())
	}
	venue, verr := boxLandedCage(&App{}, landedCageVenue, child, f.tree, writable, env)

	switch m := WallMechanism(); m {
	case WallSandboxExec:
		// D1's other half: on sandbox-exec the venue REFUSES, because the
		// step does not exist on darwin (ADR 0096: no darwin change). A
		// render that handed back the judge's profile under the venue's name
		// would be a wall check measuring inside a wall that forbids nesting
		// — every member abstaining, exit 3, and D5's watchdog filing
		// silence at a host nobody asked.
		if verr == nil {
			t.Fatalf("the venue RENDERED on %s: %v — ADR 0097 D1 says the step does not exist here, and a cage that renders is a wall check that runs and abstains", m, venue.Argv)
		}
		for _, want := range []string{"ADR 0097", "ADR 0096"} {
			if !strings.Contains(verr.Error(), want) {
				t.Errorf("the refusal does not cite %s, so a reader cannot tell a host decision from a broken render: %v", want, verr)
			}
		}
		return
	case WallBwrap:
	default:
		t.Fatalf("boxLandedCage rendered a cage on mechanism %q, which this pin does not know how to read", m)
	}
	if verr != nil {
		t.Fatalf("the venue would not render where the judge did: %v", verr)
	}

	added, removed := wallVenueArgvDelta(judge.Argv, venue.Argv)
	want := wallVenueExpectedAdditions(f.run)
	sort.Strings(want)
	if strings.Join(added, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("the venue's argv minus the judge's is\n  %v\nand ADR 0097 D1 says it is exactly\n  %v\nThe venue is the judge's cage plus three things: `%s %s %s`, a private %s bound from the run dir, and %s=1. An addition the ADR does not name is a capability nobody priced (D1's third bullet is an argument about ONE axis), and a missing one is a venue the pins cannot measure in.\n  judge: %s\n  venue: %s",
			added, want, wallVenuePidNS, wallVenueProc, wallVenueProcMount, wallVenueVarTmpMount, wallVenueEnvVar, bwJoin(judge.Argv), bwJoin(venue.Argv))
	}
	if len(removed) != 0 {
		t.Errorf("the venue's argv DROPS %v from the judge's — ADR 0097 D1's first bullet is \"it is never weaker than the judge\", and that sentence is what lets landed code into this cage at all:\n  judge: %s\n  venue: %s",
			removed, bwJoin(judge.Argv), bwJoin(venue.Argv))
	}

	// The three as RUNS, because bwrap applies mounts in argv order and
	// `--proc` and `--bind` each take their destination as an argument: a
	// multiset that is right with the tokens scattered would be a render
	// nobody could read and a wall nobody could predict.
	for _, seq := range [][]string{
		{wallVenuePidNS},
		{wallVenueProc, wallVenueProcMount},
		{"--bind", filepath.Join(f.run, wallVenueVarTmpName), wallVenueVarTmpMount},
		{"--setenv", wallVenueEnvVar, "1"},
	} {
		if bwIndex(venue.Argv, seq...) < 0 {
			t.Errorf("the venue's argv does not carry `%s` as adjacent tokens: %s", strings.Join(seq, " "), bwJoin(venue.Argv))
		}
	}

	// THE PREP IS THE OTHER HALF OF THE /var/tmp ADDITION: bwrap binds only a
	// source that exists, so a bind with no mkdir in front of it is a cage
	// that refuses at exec time, which D5 reads as silence.
	src := filepath.Join(f.run, wallVenueVarTmpName)
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		t.Errorf("the venue render left no directory at %s (%v) — the bind's source is made in prep, OUTSIDE the namespace, exactly as the judge's private /tmp is", src, err)
	}

	// AND THE ENVIRONMENT HALF, at the layer D1 names: the variable is added
	// AFTER landedJudgeEnvKeys, which is what `over` means in landedJudgeEnv.
	// Read off the child's final environment and not off cage.Env, because
	// that final list is what the process gets.
	jenv, venv := landedJudgeEnv(judge.Env), landedJudgeEnv(venue.Env)
	if !containsString(venv, wallVenueEnvVar+"=1") {
		t.Errorf("the venue child's environment does not carry %s=1, so every member of the run abstains and the body answers exit 3 — which D5's watchdog reports as silence and not as a wall regression: %v", wallVenueEnvVar, venv)
	}
	for _, kv := range jenv {
		if strings.HasPrefix(kv, wallVenueEnvVar+"=") {
			t.Errorf("the JUDGE's environment carries %q — the variable is the one signal that tells a process it is the venue, and a judge that sets it would make D4's gate FAIL every pin that abstains inside the judge's own cage", kv)
		}
	}
}

// And the call sites: the two cages one landed check renders differ in the
// KIND and in the CHILD, and in nothing else.
//
// WHY THIS ARM IS HERE AND NOT FOLDED INTO THE ONE ABOVE. The argv arm is
// handed its inputs by the test; this one reads what the PRODUCTION call
// sites pass — the write grant, the environment, the tree — through the cage
// seam, on a real repo, through `judge` and `measure` themselves. Without it
// the pin above would hold the renderer to D1 while a call site quietly
// handed the venue a wider write set or a second environment variable, and
// every word of D1's argument is about the cage the WALL CHECK actually runs
// in.
func TestQATheVenueCallSiteDiffersFromTheJudgesInTheKindAndTheChildOnly(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	repo := wtRepo(t)
	for _, p := range []string{landedCheckScript, wallCheckScript} {
		write(t, filepath.Join(repo, p), "#!/bin/sh\nexit 0\n")
		mustGit(t, repo, "add", "--", p)
	}
	mustGit(t, repo, "commit", "-q", "-m", "ship both checks", "--", landedCheckScript, wallCheckScript)

	type call struct {
		kind     landedCageKind
		child    string
		tree     string
		writable []string
		env      []string
	}
	var calls []call
	c := newLandedChecker(a, &strings.Builder{})
	c.cage = func(_ *App, kind landedCageKind, child, tree string, writable, env []string) (landedCage, error) {
		calls = append(calls, call{kind, child, tree, writable, env})
		// The child as its own argv, like plainCage: the judge must answer
		// GREEN for the wall step to be reached at all (ADR 0097 D2).
		return landedCage{Argv: []string{child}, Env: env}, nil
	}
	c.once(repo, "main")

	if len(calls) != 2 {
		t.Fatalf("one landed check over a tree shipping both scripts rendered %d cage(s), want 2 — the judge's and the venue's (ADR 0097 D2): %+v", len(calls), calls)
	}
	j, v := calls[0], calls[1]
	if j.kind != landedCageJudge || v.kind != landedCageVenue {
		t.Fatalf("the two renders are kinds %v and %v, want judge then venue — the wall check runs AFTER a green judgement and in the venue, never the other way round (ADR 0097 D2)", j.kind, v.kind)
	}
	// The two copies are extracted SIDE BY SIDE in the one run dir, under the
	// two names landedRule.Extract gives them: one run extracts both, and a
	// shared name would have the venue exec the judge.
	if filepath.Dir(j.child) != filepath.Dir(v.child) {
		t.Errorf("the judge ran out of %s and the wall check out of %s — ADR 0097 D2 is one run dir per landing, and two means a second worktree and a second removal", filepath.Dir(j.child), filepath.Dir(v.child))
	}
	if got, want := filepath.Base(j.child), landedJudgeRule.Extract; got != want {
		t.Errorf("the judge's extracted copy is named %q, want %q", got, want)
	}
	if got, want := filepath.Base(v.child), wallCheckRule.Extract; got != want {
		t.Errorf("the wall check's extracted copy is named %q, want %q — two names, because one run extracts both", got, want)
	}
	if j.tree != v.tree {
		t.Errorf("the venue judges %s and the judge judged %s — ADR 0097 D2 says the same tree, and a second worktree is a second `git worktree add` per landing", v.tree, j.tree)
	}
	if strings.Join(j.writable, "\x00") != strings.Join(v.writable, "\x00") {
		t.Errorf("the venue's write set is %v and the judge's is %v — D1's venue is the judge's cage plus three things, and a write grant is not one of them", v.writable, j.writable)
	}
	if strings.Join(j.env, "\x00") != strings.Join(v.env, "\x00") {
		t.Errorf("the venue's env is %v and the judge's is %v — POSSE_WALL_VENUE is added by the RENDER (D1), so a call site that passed it would be a second place the venue is decided, and landedRunEnv is the one spelling of the rest", v.env, j.env)
	}
}

// D1's OTHER HALF, READ FROM THIS HOST: on sandbox-exec the venue REFUSES.
//
// The arm above has this case, and only a darwin host takes it — so on the box
// this is developed on, and on every Linux leg of ci.yml, the one refusal the
// venue adds is never executed. MEASURED 2026-11-14 (ranger-base-rs6qo,
// verifying ranger-base-wplhv): deleting that `if kind == landedCageVenue`
// block outright left every arm of both tables and both argv pins green.
// `wallMechanismForTest` is exactly what that costs to fix — it is why
// `wallMechanismHost` is a var (ranger-base-temhf) — so the refusal is read
// here rather than left to a host nobody on this fleet runs the suite on.
//
// WHAT IT WOULD COST TO BE WRONG (ADR 0097 D1/D2): a render that handed back
// the judge's profile under the venue's name is a wall check measuring inside
// a wall that forbids nesting — every member abstaining, exit 3, and D5's
// watchdog filing "wall check silent at <repo>" at a host that was never asked
// to answer. The caller checks the mechanism first
// (TestAHostWhoseWallIsNotABwrapRunsNoVenueAndCountsNoSilence); this refusal
// is what makes that structural rather than a single guard.
//
// SERIAL, no t.Parallel: the fixture sets HOME and wallMechanismForTest writes
// a package-level var.
func TestQATheVenueRefusesOnTheOtherHostsWallMechanism(t *testing.T) {
	f := newLandedCageFixture(t)
	wallMechanismForTest(t, "darwin")
	if m := WallMechanism(); m != WallSandboxExec {
		t.Fatalf("WallMechanism() reads %q behind wallMechanismForTest(darwin), so this pin cannot reach the arm it exists for", m)
	}
	cage, err := boxLandedCage(&App{}, landedCageVenue, filepath.Join(f.run, "judge"), f.tree, []string{f.run}, landedRunEnv(f.run))
	if err == nil {
		t.Fatalf("the venue RENDERED on %s: %v — ADR 0097 D1 says the step does not exist here, and a cage that renders is a wall check that runs and abstains on every member", WallSandboxExec, cage.Argv)
	}
	// The refusal has to say WHOSE decision it is, because the reader of this
	// line is someone whose wall check stopped running: the two records it
	// cites are what separate "this host does not do this" from a render that
	// broke, and the mechanism is what separates it from a missing seatbelt.
	for _, want := range []string{"ADR 0097", "ADR 0096", CageSeatbelt, WallSandboxExec} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q, so a reader cannot tell a host decision from a broken render: %v", want, err)
		}
	}
	// And the judge still renders behind the same mechanism, or this arm would
	// pass over a render that refuses everything.
	if _, jerr := boxLandedCage(&App{}, landedCageJudge, filepath.Join(f.run, "judge"), f.tree, []string{f.run}, landedRunEnv(f.run)); jerr != nil && !strings.Contains(jerr.Error(), "sandbox-exec") && !strings.Contains(jerr.Error(), CageSeatbelt) {
		t.Errorf("the JUDGE refused on %s for a reason that is not the mechanism (%v), so the venue's refusal above may be that same reason and not D1's", WallSandboxExec, jerr)
	}
}
