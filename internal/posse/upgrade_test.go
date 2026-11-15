//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-zhw0h — `posse upgrade`, ADR 0092 D1-D3 and the
// rollback-line half of D4.
//
// Each claim here is a direction this verb could go wrong in silence, which
// is the only kind of wrong a thing that installs the fleet's binary gets to
// be — an install that fails loudly is a bad afternoon, and an install that
// succeeds over a guardrail it skipped is what the guardrails are for:
//
//	the fence       lets through exactly one persona and the operator, and
//	                an absent key lets through no persona at all.
//	the target      refuses every commit guardrail 1 refuses — unpushed,
//	                backwards, already installed, not in the repo at all —
//	                and never fetches to make one acceptable.
//	the order       a red suite, a failed build and an unwritable target
//	                each stop the run BEFORE anything is written, and the
//	                green path runs test → build → epilogue in that order.
//	the swap        never leaves the target absent or short, which is the
//	                whole reason it is a rename(2) and not the install(1)
//	                the Makefile uses.
//	the record      D6's line is LAST on stdout and the rollback command is
//	                printed BEFORE the swap it might have to undo.
//	the wiring      the verb reaches those guardrails in that order, and a
//	                refusal reads nothing after it (ranger-base-anczm — the
//	                three above were deletable green until the seam at the
//	                foot of this file).

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ─── D2, the fence ───────────────────────────────────────────────────────────

func TestUpgradeFenceLetsThroughTheDelegateAndTheOperatorAndNobodyElse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		persona  string
		delegate string
		allow    bool
		says     string
	}{
		// The operator has no RHQ_PERSONA and may always run it, with or
		// without a key — deleting the key is ADR 0092's own exit hatch, and
		// an exit hatch that also locks the operator out is a trap.
		{"operator, no key", "", "", true, ""},
		{"operator, key set", "", "farrier", true, ""},
		// An ABSENT key refuses EVERY persona and not merely an unnamed one.
		{"persona, no key", "farrier", "", false, "no install_delegate: is configured"},
		{"wrong persona", "wheelwright", "farrier", false, "posse upgrade is farrier's"},
		{"the delegate", "farrier", "farrier", true, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := upgradeFence(c.persona, c.delegate)
			if c.allow {
				if err != nil {
					t.Fatalf("upgradeFence(%q, %q) refused: %v", c.persona, c.delegate, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("upgradeFence(%q, %q) allowed the run — the fleet's binary is installable by a persona the operator did not name", c.persona, c.delegate)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("refusal does not say %q:\n%v", c.says, err)
			}
			// Every refusal names the persona it refused, because the
			// delegate reading it has to be able to tell "I am not the
			// delegate" from "this box has no delegate".
			if !strings.Contains(err.Error(), c.persona) {
				t.Errorf("refusal does not name the persona %q:\n%v", c.persona, err)
			}
		})
	}
}

// The key is read from config and trimmed, so a trailing space in a YAML
// value cannot silently lock the delegate out of a verb they own.
func TestUpgradeDelegateIsTheTrimmedConfigValue(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	write(t, a.ConfigPath, "install_delegate:  farrier  \n")
	if got := a.UpgradeDelegate(); got != "farrier" {
		t.Errorf("UpgradeDelegate() = %q, want %q", got, "farrier")
	}
}

func TestUpgradeDelegateIsEmptyWhenTheKeyIsAbsent(t *testing.T) {
	t.Parallel()
	a := wtApp(t)
	write(t, a.ConfigPath, "operator: someone\n")
	if got := a.UpgradeDelegate(); got != "" {
		t.Errorf("UpgradeDelegate() = %q over a config with no key, want \"\" — which is what refuses every persona", got)
	}
}

// ─── D3.1, the target ────────────────────────────────────────────────────────

// upgradeFixture is a posse-source-shaped repo: four commits on main, with
// refs/remotes/origin/main pinned at the THIRD. So the fourth is a landing
// the coordinator has not pushed — the shape `make install` installs by
// construction and this verb must refuse (ADR 0092, Context).
type upgradeFixture struct {
	repo           string
	a, b, c, d     string
	target, staged string
}

func newUpgradeFixture(t *testing.T) upgradeFixture {
	t.Helper()
	f := upgradeFixture{repo: wtRepo(t)}
	f.a = mustGit(t, f.repo, "rev-parse", "HEAD")
	for i, sha := range []*string{&f.b, &f.c, &f.d} {
		commitIn(t, f.repo, "f.txt", fmt.Sprintf("v%d\n", i), fmt.Sprintf("commit %d", i))
		*sha = mustGit(t, f.repo, "rev-parse", "HEAD")
	}
	// update-ref rather than a real remote: what this verb reads is the
	// remote-tracking ref, and the point is that NOTHING here fetches, so a
	// fixture with a fetchable remote would test the opposite of the claim.
	mustGit(t, f.repo, "update-ref", upgradeDefaultRef, f.c)

	dir := t.TempDir()
	f.target = filepath.Join(dir, "posse")
	f.staged = f.target + ".new"
	if err := WriteExecutable(f.target, []byte("OLD BINARY\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestUpgradeResolveTakesTheTipOfOriginMainByDefault(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	p, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	if p.To != f.c {
		t.Errorf("default target = %s, want %s (the tip of %s) — NOT %s, which is on main and unpushed", p.To, f.c, upgradeDefaultRef, f.d)
	}
	if p.From != f.a {
		t.Errorf("From = %s, want the installed stamp %s", p.From, f.a)
	}
	if p.Count != 2 {
		t.Errorf("Count = %d, want 2 — the line the delegate posts on the install bead says how many commits this install carries", p.Count)
	}
}

func TestUpgradeResolveRefusesACommitTheCoordinatorHasNotPushed(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	_, err := upgradeResolve(f.repo, f.target, f.a, f.d)
	if err == nil {
		t.Fatal("upgradeResolve accepted a commit that is on main and NOT on origin/main — guardrail 1 is that the push is the record that posse-land-check and the vuln-private scan ran")
	}
	for _, want := range []string{"not an ancestor", upgradeDefaultRef, "PUSHED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not say %q:\n%v", want, err)
		}
	}
}

func TestUpgradeResolveRefusesGoingBackwards(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	// Installed at c, asked for a. `a` IS on origin/main, so the only thing
	// that refuses this is the descendant test.
	_, err := upgradeResolve(f.repo, f.target, f.c, f.a)
	if err == nil {
		t.Fatal("upgradeResolve accepted a downgrade — a rollback is one coreutils command (ADR 0092 D4), not this verb")
	}
	if !strings.Contains(err.Error(), "not a descendant") {
		t.Errorf("refusal does not say it is not a descendant:\n%v", err)
	}
	// And it hands over the command that IS the way to go backwards,
	// because a refusal whose remedy the reader has to go and look up is a
	// refusal they will work around.
	if !strings.Contains(err.Error(), upgradeRollbackCmd(f.target)) {
		t.Errorf("refusal does not carry the rollback command %q:\n%v", upgradeRollbackCmd(f.target), err)
	}
}

func TestUpgradeResolveRefusesTheCommitAlreadyInstalled(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	_, err := upgradeResolve(f.repo, f.target, f.c, "")
	if err == nil {
		t.Fatal("upgradeResolve accepted an install of the commit already installed — twenty minutes of suite for no change")
	}
	if !strings.Contains(err.Error(), "already the installed binary") {
		t.Errorf("refusal does not say the commit is already installed:\n%v", err)
	}
}

func TestUpgradeResolveRefusesAShaTheRepoDoesNotHold(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	_, err := upgradeResolve(f.repo, f.target, f.a, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatal("upgradeResolve accepted a sha this repo does not hold")
	}
	// The remedy is NOT "and so I fetched it": never fetching is D3.1.
	if !strings.Contains(err.Error(), "never fetches") {
		t.Errorf("refusal does not say the verb never fetches:\n%v", err)
	}
}

func TestUpgradeResolveRefusesARepoWithNoOriginMain(t *testing.T) {
	t.Parallel()
	f := newUpgradeFixture(t)
	mustGit(t, f.repo, "update-ref", "-d", upgradeDefaultRef)
	_, err := upgradeResolve(f.repo, f.target, f.a, f.b)
	if err == nil {
		t.Fatal("upgradeResolve ran against a repo with no origin/main — then there is no record of any push, and every sha would pass guardrail 1 vacuously")
	}
	if !strings.Contains(err.Error(), "never fetches") {
		t.Errorf("refusal does not say the verb never fetches:\n%v", err)
	}
}

// ─── D3.2, a red suite writes nothing ────────────────────────────────────────

// stubUpgradeSteps replaces the three shelling-out steps for one test and
// restores them after. They are package vars precisely so a pin can refuse
// a twenty-minute suite and still assert the order around it.
func stubUpgradeSteps(t *testing.T, test func(io.Writer, string) error, build func(io.Writer, string, string, string) error, epilogue func(io.Writer, string, upgradePlan) string) {
	t.Helper()
	savedTest, savedBuild, savedEpilogue := upgradeTest, upgradeBuild, upgradeEpilogue
	t.Cleanup(func() { upgradeTest, upgradeBuild, upgradeEpilogue = savedTest, savedBuild, savedEpilogue })
	upgradeTest, upgradeBuild, upgradeEpilogue = test, build, epilogue
}

func TestUpgradeRefusesOnARedSuiteBeforeAnythingIsWritten(t *testing.T) {
	f := newUpgradeFixture(t)
	built := false
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error {
			fmt.Fprintln(w, "FAIL github.com/ranger360ai/posse/internal/posse")
			return errors.New("exit status 1")
		},
		func(w io.Writer, dir, repo, out string) error {
			built = true
			return nil
		},
		func(w io.Writer, wt string, plan upgradePlan) string {
			t.Error("the epilogue ran after a red suite")
			return upgradeGateUnchanged
		})

	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	var out bytes.Buffer
	if err := upgradeRunPlan(&out, plan); err == nil {
		t.Fatal("a red `make test` did not refuse the upgrade")
	} else if !strings.Contains(err.Error(), "did not pass") {
		t.Errorf("the refusal does not say the suite did not pass:\n%v", err)
	}

	if built {
		t.Error("the build ran after a red suite — step 3 must not start until step 2 has answered green (ADR 0092 D3)")
	}
	// The three paths that would exist if anything had been written. The
	// claim is "nothing was written", and the only way to check a claim
	// like that is to name every file this run could have made.
	for _, p := range []string{f.staged, upgradePrevPath(f.target)} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s exists after a red suite — nothing is written before the suite answers", filepath.Base(p))
		}
	}
	if body, err := os.ReadFile(f.target); err != nil || string(body) != "OLD BINARY\n" {
		t.Errorf("the target changed after a red suite: %q (%v) — the binary in place must be the one that was there before the command ran", body, err)
	}
	// The suite's own output is the delegate's evidence and must reach the
	// report rather than being swallowed into the error.
	if !strings.Contains(out.String(), "FAIL github.com/ranger360ai/posse") {
		t.Errorf("the suite's output is not in the report:\n%s", out.String())
	}
}

func TestUpgradeRefusesOnAFailedBuildWithoutSwapping(t *testing.T) {
	f := newUpgradeFixture(t)
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error { return nil },
		func(w io.Writer, dir, repo, out string) error {
			// A failed build may well have left a partial file behind.
			// That must not become the fleet's binary, and must not be
			// left lying beside it either.
			if err := WriteExecutable(out, []byte("HALF A BINARY"), 0o755); err != nil {
				t.Fatal(err)
			}
			return errors.New("exit status 2")
		},
		func(w io.Writer, wt string, plan upgradePlan) string {
			t.Error("the epilogue ran after a failed build")
			return upgradeGateUnchanged
		})

	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	if err := upgradeRunPlan(io.Discard, plan); err == nil {
		t.Fatal("a failed build did not refuse the upgrade")
	}
	if _, err := os.Stat(f.staged); err == nil {
		t.Error("the half-built staging file is still beside the target — the next run would have to tell it from a real one")
	}
	if body, _ := os.ReadFile(f.target); string(body) != "OLD BINARY\n" {
		t.Errorf("the target changed after a failed build: %q", body)
	}
}

// An unwritable target is a refusal the run can make for free, and must
// make before it spends a suite finding out.
func TestUpgradeRefusesAnUnwritableTargetBeforeRunningTheSuite(t *testing.T) {
	f := newUpgradeFixture(t)
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error {
			t.Error("the suite ran although the new binary had nowhere to land — that is twenty minutes and two box suite slots spent on a refusal that was free")
			return nil
		},
		func(w io.Writer, dir, repo, out string) error { return nil },
		func(w io.Writer, wt string, plan upgradePlan) string { return upgradeGateUnchanged })

	// A directory where the staging file has to go is the simplest
	// unwritable target there is, and it needs no chmod — which under a
	// test that may run as root would not make anything unwritable at all.
	if err := os.Mkdir(f.staged, 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	err = upgradeRunPlan(io.Discard, plan)
	if err == nil {
		t.Fatal("upgradeRunPlan ran with no place to put the new binary")
	}
	if !strings.Contains(err.Error(), "nowhere to land") {
		t.Errorf("the refusal does not say the new binary has nowhere to land:\n%v", err)
	}
	if body, _ := os.ReadFile(f.target); string(body) != "OLD BINARY\n" {
		t.Errorf("the target changed: %q", body)
	}
}

// The green path, end to end below the stamp read: the steps run in D3's
// order, the swap happens, and D6's line is LAST.
func TestUpgradeGreenPathSwapsAndEndsWithTheRecordLine(t *testing.T) {
	f := newUpgradeFixture(t)
	// D4 reads the target before and after the swap, so the binaries this
	// pin writes have to be able to answer. The fixture's "OLD BINARY\n"
	// reads as a binary that runs and names no commit, which is all the
	// BEFORE side is asked for; the one the build writes is stamped with
	// the commit being installed, which is what D4's first bullet checks.
	stubUpgradeProbe(t)
	built := fakePosse{Stamp: f.c}
	var order []string
	var testDir string
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error {
			order = append(order, "test")
			// The checkout it is handed is a real detached checkout of the
			// commit being installed — and in a repository of its OWN, whose
			// object store is not the source repo's (ranger-base-p8ymi): a
			// worktree of the source reads the shared store, which every
			// walled seat may write.
			if got := strings.TrimSpace(mustGit(t, dir, "rev-parse", "HEAD")); got != f.c {
				t.Errorf("`make test` ran at %s, want the target commit %s", got, f.c)
			}
			if got, want := upgradeObjectsDir(t, dir), upgradeObjectsDir(t, f.repo); got == want {
				t.Errorf("`make test` ran in %s, whose object store is the source repo's %s — D3.2 asks for a private, re-hashed copy (ranger-base-p8ymi)", dir, want)
			}
			if got := mustGit(t, dir, "rev-parse", upgradeDefaultRef); got != f.c {
				t.Errorf("the private copy's %s is %q, want the tip the plan read, %s", upgradeDefaultRef, got, f.c)
			}
			testDir = dir
			return nil
		},
		func(w io.Writer, dir, repo, out string) error {
			order = append(order, "build")
			if dir != testDir {
				t.Errorf("the build ran in %s, want the copy the suite passed in, %s", dir, testDir)
			}
			if repo != f.repo {
				t.Errorf("the build was handed %q as the source checkout to stamp, want %q — a stamp naming the throwaway worktree is dead by the time anything reads it (ranger-base-eiwhk)", repo, f.repo)
			}
			return os.WriteFile(out, built.bytes(), 0o644)
		},
		func(w io.Writer, wt string, plan upgradePlan) string {
			order = append(order, "epilogue")
			if wt != testDir {
				t.Errorf("the epilogue ran from %s, want the copy the suite passed in, %s", wt, testDir)
			}
			if plan.Target != f.target {
				t.Errorf("the epilogue was handed %q as POSSE, want the just-installed %q", plan.Target, f.target)
			}
			return upgradeGateUnchanged
		})

	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	var out bytes.Buffer
	if err := upgradeRunPlan(&out, plan); err != nil {
		t.Fatalf("upgradeRunPlan: %v", err)
	}

	if got := strings.Join(order, ","); got != "test,build,epilogue" {
		t.Errorf("the steps ran %s, want test,build,epilogue (ADR 0092 D3)", got)
	}
	if body, _ := os.ReadFile(f.target); !bytes.Equal(body, built.bytes()) {
		t.Errorf("the target holds %q after a green run, want the new binary", body)
	}
	if body, _ := os.ReadFile(upgradePrevPath(f.target)); string(body) != "OLD BINARY\n" {
		t.Errorf("%s holds %q, want the binary that was live — a rollback with nothing to roll back to is not one", upgradePrevPath(f.target), body)
	}
	if st, err := os.Stat(f.target); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Errorf("the installed binary is not executable: %v %v", st, err)
	}
	if _, err := os.Stat(f.staged); err == nil {
		t.Error("the staging file is still there after the swap — rename(2) moves it, so this means something else wrote it")
	}

	// D6: the last line, and the rollback command is in it.
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	last := lines[len(lines)-1]
	want := fmt.Sprintf("upgraded %s..%s (2 commits) · gate %s · rollback: %s", shortSha(f.a), shortSha(f.c), upgradeGateUnchanged, upgradeRollbackCmd(f.target))
	if last != want {
		t.Errorf("the last stdout line is\n  %q\nwant\n  %q", last, want)
	}
	// And the rollback command was printed BEFORE the swap, which is the
	// half of D4 this bead carries: the reader who needs it most is the one
	// whose next command is the one that broke.
	roll := strings.Index(out.String(), "rollback, if the new binary misbehaves")
	inst := strings.Index(out.String(), "installed: ")
	if roll < 0 || inst < 0 || roll > inst {
		t.Errorf("the rollback line is not printed before the swap (rollback at %d, installed at %d)", roll, inst)
	}
}

// The stub-based pins above can only see the ARGUMENT upgradeBuild is
// handed. What reaches clean-build.sh is an environment variable, and that
// is the whole of the fix: without it the verb stamps the throwaway worktree
// it compiles in, every installed binary carries a path that is gone by the
// time anything reads it, and the freshness reading goes back to the
// permanent UNKNOWN of ranger-base-eiwhk — green suite, green install, dead
// reading. So this one runs the real upgradeBuild against a script that
// reports what it was given.
func TestUpgradeBuildHandsCleanBuildTheRealSourceCheckout(t *testing.T) {
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(filepath.Join(wt, "scripts", "clean-build.sh"),
		[]byte("#!/bin/sh\necho \"source=${POSSE_BUILD_SOURCE-UNSET} out=$1 cwd=$PWD\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := upgradeBuild(&out, wt, "/src/posse", "/bin/posse.new"); err != nil {
		t.Fatalf("upgradeBuild: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "source=/src/posse") {
		t.Errorf("clean-build.sh was not told which checkout to stamp: %q — it would stamp the throwaway worktree, and the stamp is dead the moment this verb removes it (ranger-base-eiwhk, ADR 0092 D3.3)", got)
	}
	if !strings.Contains(got, "out=/bin/posse.new") {
		t.Errorf("clean-build.sh was not told where to write: %q", got)
	}
	// And it runs IN the worktree, because that is what makes it build the
	// commit the tests passed on rather than whatever the cwd holds.
	if !strings.Contains(got, "cwd="+wt) {
		t.Errorf("clean-build.sh ran somewhere other than the worktree: %q, want cwd=%s", got, wt)
	}
}

// ─── D3.5, the swap leaves no gap ────────────────────────────────────────────

// TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort is the measurement ADR
// 0092 D3.5 rests on, turned into a pin.
//
// install(1) — what the Makefile uses — unlinks the target and then writes
// it, so a launch that execs the binary inside that window gets a missing or
// a short file. MEASURED 2026-11-13 on this box (GNU coreutils 9.12, 64 MiB
// over 64 MiB, three trials): the target was ENOENT for 1,791-2,473 of
// ~84,000 observations and short for 12,606-13,384 of them, over a 58 ms
// window. rename(2) over the same probe: zero and zero, every trial.
//
// So this reader is the same probe at pin scale: it opens the target, stats
// it and reads its first byte as fast as it can for the whole swap, and the
// claim is that every observation is one of exactly two complete files —
// ENOENT is absent, a size short of the binary is short, the first byte is
// which version. A swap that went back to install(1), or that truncated the
// target in place, reds here.
//
// THE OBSERVATION'S COST IS THE WHOLE DESIGN (ranger-base-6sbng). It used
// to be a full 8 MiB os.ReadFile, and that cost more than the window it was
// looking for — ~1.7 ms an observation against an install(1) window at
// 8 MiB that is mostly narrower than one observation — so the pin reported
// a clean green over a swap that was genuinely leaving the target absent.
// MEASURED 2026-11-14 on this Linux box at a loadavg of 7.5, 50 trials a
// reader, same 20 ms samples, same install(1)-shaped write, all else equal:
//
//	reader                   observations / 40 ms   caught the gap
//	8 MiB os.ReadFile        19-26                  14 of 50
//	open + Fstat + 1 byte    7,400-8,500            50 of 50
//
// Nothing about the window changed; the probe stopped being the slowest
// thing in the room. Against the real rename(2) both readers are 0 of 50,
// so the density buys power and not noise, and against a truncate-in-place
// write — the other half of this pin's name, where the target is never
// absent and only ever short — the dense reader is 10 of 10 with 590-880
// short observations a trial.
//
// THE WINDOW IS STILL SAMPLED: two 20 ms sleeps AND a wait for the witness
// (ranger-base-51ttd). The sleeps are what the absence check above is
// measured over, so they are unconditional; the waits are what stop a
// descheduled reader from spending the arm. MEASURED 2026-11-14 at a box
// loadavg of 5.9 rising past 30, a reader that managed 3 reads in those
// 40 ms did all 3 on one side of the rename, and the witness t.Fatal'd on
// "I could not measure" — which reds internal/posse, and a red
// internal/posse exits `make test` 2 for every seat on the box. Waiting for
// the witness costs wall time where sampling for it cost a red.
//
// AND THE PIN CARRIES ITS OWN POSITIVE CONTROL, which is what makes the
// green mean something. "Saw old, then saw new" does not witness coverage
// of the swap: a complete read before the unlink and a complete read after
// the write is what a descheduled reader and a diligent one both look like,
// and MEASURED 2026-11-14 that witness read MET in 12 of 12 trials of which
// 3 saw the gap. So the pin runs the same reader a second time against a
// deliberately install(1)-shaped write (installShapedSwap below) and
// requires it to SEE that gap. That is the probe's power measured on this
// box under this run's load, rather than inferred from a gap statistic, and
// a probe coarsened by a later edit is caught the moment it is coarsened.
//
// THE CONTROL IS NOT ENOUGH ON ITS OWN, so the coverage witness beside it
// is a budget on the MEAN observation. MEASURED 2026-11-14 with the probe
// mutated back to the 8 MiB os.ReadFile, the control still saw its own gap
// in 2 runs of 3: INSIDE a gap a coarse probe's reads get cheap — an ENOENT
// open, a short file — and its count catches up, so "the control saw it" is
// a coin flip about the subject's own 40 ms rather than a guarantee. What a
// coarse probe cannot fake is the cost of an observation OUTSIDE the gap.
// So the pin budgets the mean observation at a tenth of the window it must
// resolve (swapProbeMaxMeanGap), where the separation is ~300x and not
// marginal: 5.5-6.2 us as this pin prints it, over every run taken at a
// loadavg of 2-17, against 1.7-2.1 ms for the reader this replaced.
//
// A maximum-HOLE bound was the other candidate for that job and was
// MEASURED and dropped (ranger-base-6sbng): over 74 rename(2) trials at a
// loadavg of 7-17 the widest hole in a trial ran 23 us-2.67 ms, at the high
// end a GC pause, against an install(1) window at 8 MiB of 2.6-4.2 ms. The
// largest honest bound is therefore ~2.5 ms against a window of 2.6 ms — a
// few percent of margin, and a constant to re-measure on every box. The max
// hole is reported in every message below and asserted nowhere.
//
// Unmet, any of the three witnesses SKIPS rather than reds; the foot of the
// pin says why that is honest here. docs/notes.d/ranger-base-6sbng.md has
// the trial tables.
func TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort(t *testing.T) {
	t.Parallel()
	old := bytes.Repeat([]byte("O"), swapProbeSize)
	fresh := bytes.Repeat([]byte("N"), swapProbeSize)

	dir := t.TempDir()
	target := filepath.Join(dir, "posse")
	staged := target + ".new"
	if err := WriteExecutable(target, old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, fresh, 0o644); err != nil {
		t.Fatal(err)
	}
	got := probeSwapWindow(t, target, old, fresh, func() error { return upgradeSwap(target, staged) })
	if got.swapErr != nil {
		t.Fatalf("upgradeSwap: %v", got.swapErr)
	}

	// THE SUBJECT. It is read first, and unconditionally, because a nonzero
	// reading here is a real finding whatever the witnesses below say: the
	// reader saw a state the target is never allowed to be in, and no
	// amount of "the box was busy" makes that observation go away.
	if got.absent != 0 || got.partial != 0 {
		t.Errorf("over %d observations (%s apart on average, widest hole %s) the target was absent %d times and neither complete binary %d times — short, or a blend of the two — so the swap is not atomic, and a launch inside that window execs a missing or half-written binary (ADR 0092 D3.5)",
			got.reads, got.meanGap, got.maxGap, got.absent, got.partial)
	}
	if body, _ := os.ReadFile(target); !bytes.Equal(body, fresh) {
		t.Error("the target does not hold the new binary after the swap")
	}
	if body, _ := os.ReadFile(upgradePrevPath(target)); !bytes.Equal(body, old) {
		t.Error("the .prev copy does not hold the binary that was live")
	}

	// THE POSITIVE CONTROL: the same reader, this box, this run, against a
	// swap that really does leave the gap. It measures the one thing the
	// absence check above cannot establish about itself — that a gap of the
	// width this pin exists to catch would have been seen.
	//
	// It gets up to swapControlAttempts of them, and stops at the first one
	// that sees the gap. A reader starved across a whole control window is
	// a fact about the box and not about the probe — MEASURED 2026-11-14 at
	// GOMAXPROCS=1 it happened in 2 runs of 10, each with a widest hole of
	// 3.7 ms, the whole window, against 0 in 60 runs on sixteen Ps — and
	// one attempt would turn that into a non-reading rather than retrying
	// something that costs 55 ms.
	control, controlAttempts := swapControl(t, old, fresh, "install(1)-shaped",
		installShapedSwap, func(p swapProbe) bool { return p.absent != 0 || p.partial != 0 })

	// THE SECOND CONTROL, for the one arm the first cannot witness
	// (ranger-base-0eysw). installShapedSwap unlinks the target and writes a
	// new one, so every observation it leaves behind is ABSENT or SHORT —
	// the two-ends comparison is never the arm that fires, and a control
	// graded on the union of the three lets those two vouch for it.
	//
	// MEASURED 2026-11-14 on this box, three runs each, with
	// `case buf[0] != buf[1]` made unreachable and nothing else changed:
	// this pin passed 3 of 3 against the real rename(2) AND 3 of 3 against
	// a swap that rewrites the target in place, which is the shape that arm
	// is the only reader of. So the claim that reading both ends keeps that
	// shape seen stood on a mutant nothing runs, where the absent and short
	// arms each had a control in the pin itself. This is that control: the
	// same reader, this box, this run, against a write that leaves the
	// target full length with its two ends disagreeing.
	blend, blendAttempts := swapControl(t, old, fresh, "in-place rewrite",
		blendShapedSwap, func(p swapProbe) bool { return p.blend != 0 })

	// THE WITNESSES, and this pin needs them more than most: a reader
	// descheduled for the whole swap window reports zero absent and zero
	// partial for the same reason a correct swap does, which is a vacuous
	// pass that no absence check can tell from the real thing.
	//
	// Unmet, they SKIP and do not red, and the three checks above are why
	// that is honest rather than a hole: everything about this swap that
	// can be judged from its end state has already been judged, and a skip
	// is reachable only when all of it was green. What is left over is
	// exactly "this run observed nothing", which is neither a swap that
	// left a gap (the red this pin exists to print) nor a swap that did not
	// (the green it is entitled to) — and an unreachable witness must not
	// be able to spend a whole arm, because a red internal/posse exits
	// `make test` 2 for every seat on the box. A skip after a t.Error still
	// reports FAIL, so a real gap outranks both of them.
	switch {
	case !got.sawOldBeforeSwap || !got.sawNewAfterSwap:
		t.Skipf("the reader saw the old binary %d times and the new one %d times over %d observations, after sampling %s and then waiting %s a side — it never completed a read on one whole side of the rename, so the absence checked above measures nothing and this run is not a reading (ranger-base-51ttd)",
			got.sawOld, got.sawNew, got.reads, swapProbeSample, swapWitnessBudget)
	case got.meanGap > swapProbeMaxMeanGap || control.meanGap > swapProbeMaxMeanGap:
		t.Skipf("the probe averaged one observation every %s over the swap and every %s over the control, against a budget of %s — a tenth of the %s window it exists to resolve. A probe that coarse cannot tell a swap that left no gap from a gap it could not see (ranger-base-6sbng); widest holes %s and %s over %d and %d observations",
			got.meanGap, control.meanGap, swapProbeMaxMeanGap, swapProbeWindow, got.maxGap, control.maxGap, got.reads, control.reads)
	case control.absent == 0 && control.partial == 0:
		t.Skipf("the control left the target absent and then short for milliseconds and this probe saw neither, in %d attempts, the last of %d observations %s apart with a widest hole of %s — so the green above is not evidence that the real swap left no gap, only that this run could not have seen one (ranger-base-6sbng)",
			controlAttempts, control.reads, control.meanGap, control.maxGap)
	case blend.blend == 0:
		t.Skipf("the in-place-rewrite control left the target full length with its two ends disagreeing for milliseconds and the two-ends arm never fired, in %d attempts, the last of %d observations %s apart (partial %d, widest hole %s) — so the probe is not shown able to see the one shape that arm is the only reader of, and a swap that rewrote the binary in place would read as green (ranger-base-0eysw)",
			blendAttempts, blend.reads, blend.meanGap, blend.partial, blend.maxGap)
	}
}

// swapControl runs ONE control shape until its probe sees the gap that shape
// leaves, up to swapControlAttempts, and reports the last attempt and how
// many it took. saw is the shape's OWN arm and not the union of the probe's
// three: a control is evidence about the arm that would have to fire, so one
// shape's detections must not be able to vouch for another shape's arm
// (ranger-base-0eysw).
//
// The retry is the pin's own, for its own reason: a reader starved across a
// whole control window is a fact about the box and not about the probe.
func swapControl(t *testing.T, old, fresh []byte, shape string,
	swap func(target string, fresh []byte) error, saw func(swapProbe) bool,
) (swapProbe, int) {
	t.Helper()
	var p swapProbe
	attempts := 0
	for attempts < swapControlAttempts {
		attempts++
		ctl := filepath.Join(t.TempDir(), "posse")
		if err := WriteExecutable(ctl, old, 0o755); err != nil {
			t.Fatal(err)
		}
		p = probeSwapWindow(t, ctl, old, fresh, func() error { return swap(ctl, fresh) })
		if p.swapErr != nil {
			t.Fatalf("the %s control's write: %v", shape, p.swapErr)
		}
		if saw(p) {
			break
		}
	}
	return p, attempts
}

// blendShapedSwap is the OTHER non-atomic shape, and the one installShapedSwap
// cannot stand in for: it opens the live target and overwrites it in place in
// install(1)'s chunks, never unlinking it and never truncating it. The target
// is therefore full length throughout and only its CONTENTS are inconsistent
// — head already the new binary while the tail is still the old one — which
// is the state a probe reading one end cannot see, the full-content compare
// this pin's probe replaced could, and the two-ends comparison exists for.
//
// Nothing in this tree swaps a binary this way; like installShapedSwap it is
// here to be caught (ranger-base-0eysw).
func blendShapedSwap(target string, fresh []byte) error {
	f, err := os.OpenFile(target, os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	for off := 0; off < len(fresh); off += swapProbeChunk {
		if _, err := f.Write(fresh[off:min(off+swapProbeChunk, len(fresh))]); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

// swapProbe is what one run of the reader below saw.
//
// blend is the subset of partial that the TWO-ENDS comparison caught — full
// length, ends disagreeing — and it is counted apart from partial only so a
// control can require that one arm rather than the union of three
// (ranger-base-0eysw). Nothing asserts on partial any differently for it.
type swapProbe struct {
	reads, absent, partial, blend, sawOld, sawNew int64
	meanGap, maxGap                               time.Duration
	sawOldBeforeSwap, sawNewAfterSwap             bool
	swapErr                                       error
}

// probeSwapWindow runs the dense reader across swap and reports what it
// saw. The pin's subject and its positive control share this reader on
// purpose: a control that measures a different probe measures nothing about
// this one.
//
// One observation is open + Fstat + the file's two end bytes — ~6 us
// MEASURED 2026-11-14 on this box, against ~1.7 ms for the 8 MiB
// os.ReadFile this replaced — and it resolves every state the pin switches
// on: a failed open is absent, a size other than the binary's is short, two
// ends that disagree is a target being rewritten in place, and otherwise
// the byte says which version. Not reading the other 8 MiB is the whole of
// ranger-base-6sbng; reading BOTH ends rather than one is what keeps the
// in-place-rewrite shape visible, which the full-content compare saw and a
// first-byte probe does not (the mutation run is in the notes fragment).
func probeSwapWindow(t *testing.T, target string, old, fresh []byte, swap func() error) swapProbe {
	t.Helper()
	// The observation below reads two bytes and infers the other 8 MiB from
	// them, which it may do only because each version is one byte repeated.
	// Say so here rather than in a comment: a fixture that stopped being
	// uniform would make every classification below a guess.
	if len(old) != len(fresh) || old[0] == fresh[0] ||
		old[0] != old[len(old)-1] || fresh[0] != fresh[len(fresh)-1] {
		t.Fatalf("the fixture must give the two versions equal lengths, one repeated byte each, and different bytes: %d and %d bytes, ends %q/%q and %q/%q",
			len(old), len(fresh), old[0], old[len(old)-1], fresh[0], fresh[len(fresh)-1])
	}

	var stop atomic.Bool
	var reads, absent, partial, blend, sawOld, sawNew atomic.Int64
	// Written by the reader goroutine and read after <-done, which is what
	// orders them. They are reported and never asserted on — see the pin's
	// comment on why a max-gap bound was dropped.
	var meanGap, maxGap time.Duration
	done := make(chan struct{})
	go func() {
		defer close(done)
		var buf [2]byte
		first := time.Now()
		last := first
		for !stop.Load() {
			now := time.Now()
			if gap := now.Sub(last); gap > maxGap {
				maxGap = gap
			}
			last = now
			reads.Add(1)
			f, err := os.Open(target)
			if err != nil {
				absent.Add(1)
				continue
			}
			st, serr := f.Stat()
			nh, errh := f.Read(buf[:1])
			nt, errt := 0, error(nil)
			if serr == nil && st.Size() > 0 {
				nt, errt = f.ReadAt(buf[1:2], st.Size()-1)
			}
			f.Close()
			switch {
			case serr != nil || errh != nil || errt != nil || nh != 1 || nt != 1:
				partial.Add(1)
			case st.Size() != int64(len(old)):
				partial.Add(1)
			case buf[0] != buf[1]:
				// Full length, but its two ends disagree: a writer is
				// rewriting the target in place and this is a blend of the
				// two binaries. The full-content compare this probe
				// replaced saw that shape, so reading both ends is what
				// keeps it seen (ranger-base-6sbng) — and the second
				// control below is what shows this arm still sees it
				// (ranger-base-0eysw).
				partial.Add(1)
				blend.Add(1)
			case buf[0] == old[0]:
				sawOld.Add(1)
			case buf[0] == fresh[0]:
				sawNew.Add(1)
			default:
				partial.Add(1)
			}
		}
		if n := reads.Load(); n > 1 {
			meanGap = last.Sub(first) / time.Duration(n-1)
		}
	}()
	// Let the reader get going, so the swap lands inside its loop rather
	// than before it — and then, if the sample was not enough to get it one
	// complete read, WAIT for that read instead of proceeding without it.
	// The sleep is what the absence check is measured over and the wait is
	// what keeps the witness reachable; neither substitutes for the other.
	var p swapProbe
	time.Sleep(swapProbeSample)
	p.sawOldBeforeSwap = waitForAtLeast(&sawOld, 1, swapWitnessBudget)
	p.swapErr = swap()
	time.Sleep(swapProbeSample)
	p.sawNewAfterSwap = waitForAtLeast(&sawNew, 1, swapWitnessBudget)
	stop.Store(true)
	<-done

	p.reads, p.absent, p.partial, p.blend = reads.Load(), absent.Load(), partial.Load(), blend.Load()
	p.sawOld, p.sawNew = sawOld.Load(), sawNew.Load()
	p.meanGap, p.maxGap = meanGap, maxGap
	return p
}

// installShapedSwap is what the Makefile's install(1) does and what
// upgradeSwap must not: unlink the target, then create it and write it in
// 64 KiB chunks, leaving the target ENOENT and then short for as long as
// the write takes — 2.6-4.2 ms at 8 MiB, MEASURED 2026-11-14 on this box at
// a loadavg of 7.5 over 50 trials. It is the pin's positive control, it is
// the only thing in this tree that swaps a binary this way on purpose, and
// nothing but that control runs it.
func installShapedSwap(target string, fresh []byte) error {
	if err := os.Remove(target); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	for off := 0; off < len(fresh); off += swapProbeChunk {
		if _, err := f.Write(fresh[off:min(off+swapProbeChunk, len(fresh))]); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

const (
	// swapProbeSize is how big each of the two binaries is. Big enough that
	// a non-atomic write has a window a reader can land in — the 58 ms
	// measured above was over 64 MiB — and small enough to keep the pin,
	// now that it runs its probe twice, in the low hundreds of
	// milliseconds.
	swapProbeSize = 8 << 20
	// swapProbeChunk is install(1)'s own write size, which is what makes
	// the control's window the width install(1)'s is.
	swapProbeChunk = 64 << 10
	// swapProbeSample is how long the probe runs either side of the swap.
	// It is the window the absence check is measured over, so it is
	// unconditional; the witness's wait is the part that can stretch.
	swapProbeSample = 20 * time.Millisecond
	// swapProbeWindow is the NARROWEST install(1)-shaped window measured at
	// swapProbeSize on this box — 2.6 ms of the 2.6-4.2 ms seen over 50
	// trials, MEASURED 2026-11-14 at a loadavg of 7.5. It is the width this
	// pin exists to resolve. Nothing is asserted against it directly; it is
	// here to derive the budget below.
	swapProbeWindow = 2600 * time.Microsecond
	// swapProbeMaxMeanGap is that window over the number of observations a
	// probe must get into it to be a probe at all. Ten is the figure and
	// the derivation is the point: the one thing a coarse probe cannot fake
	// is the cost of an observation OUTSIDE the gap, because inside the gap
	// its reads get cheap — an ENOENT open, a short file — and its count
	// catches up. MEASURED 2026-11-14 at a loadavg of 2-17, the mean
	// observation was 5.5-6.2 us as the pin prints it, so this budget sits
	// ~42x above every reading taken; the 8 MiB os.ReadFile it replaced was
	// 1.7-2.1 ms, 7x over it. That is the separation a max-hole bound did
	// not have.
	//
	// The SUBJECT's mean is the load-bearing half of the witness that reads
	// it: a control spends a slice of its window inside the gap it made, so
	// it reads cheaper than it is — MEASURED 158 us with the probe
	// coarsened to the 8 MiB read, where the same run's subject read
	// 1.95 ms. Both are checked because an OR of the two can only be
	// stricter, never looser.
	swapProbeMaxMeanGap = swapProbeWindow / 10
	// swapControlAttempts is how many times the pin will run its positive
	// control before calling the run a non-reading. Three, because the
	// thing being retried is a scheduling accident with a measured rate of
	// 2 in 10 at GOMAXPROCS=1 and 0 in 60 on this box's sixteen, and each
	// attempt costs ~55 ms only on the runs that need it.
	swapControlAttempts = 3
)

// swapWitnessBudget is how long the pin above waits for its reader
// goroutine to be scheduled on one side of the rename. It bounds a wait for
// a thing that normally happens in microseconds, so it is sized to be
// unreachable rather than tight: the focused pin runs in 0.10s, and the
// loaded run that made this budget necessary had its 3 reads inside 40ms —
// one read a side is all the witness asks for, so nothing but a reader
// starved for ten seconds can reach the end of it.
const swapWitnessBudget = 10 * time.Second

// waitForAtLeast polls until c has reached want, and REPORTS whether it did
// rather than failing. The caller's answer to "the reader never got
// scheduled" is not its answer to "the swap left a gap" — see the witness
// at the foot of the pin above — and a helper that calls t.Fatal cannot
// tell the two apart.
func waitForAtLeast(c *atomic.Int64, want int64, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for {
		if c.Load() >= want {
			return true
		}
		if !time.Now().Before(deadline) {
			return c.Load() >= want
		}
		time.Sleep(time.Millisecond)
	}
}

// D6's four-word gate field, read off verify-gate-freshness's own exit codes
// (0 clean, 1 findings, 2 nothing measured) by the gate arm, and the default
// is the one that claims nothing.
//
// The exit codes are read HERE as well as in upgradegate_test.go's arm pins
// because this is the reading that cannot be stubbed: a real *exec.ExitError
// from a real child, so a change to how a child's status is recovered reds
// with the code it stopped seeing. `fresh` and `stale` are retired
// (ranger-base-estjw) — the words are now what the ARM did, and an arm that
// installs has two outcomes where a reading had one.
func TestUpgradeExitCodeIsWhatTheVerifierActuallyExitedWith(t *testing.T) {
	t.Parallel()
	if got := upgradeExitCode(nil); got != 0 {
		t.Errorf("a child that exited 0 reads as %d", got)
	}
	for _, code := range []int{1, 2, 127} {
		if got := upgradeExitCode(exitErr(t, code)); got != code {
			t.Errorf("exit %d is recovered as %d — this is the code the gate arm's verdict is read off", code, got)
		}
	}
	// An error that is not an exit at all — the script could not be run —
	// must not read as clean, or a gate arm that never probed anything would
	// report its green word.
	if got := upgradeExitCode(errors.New("no such file")); got == 0 {
		t.Error("a child that never ran reads as exit 0")
	}
	// And the four words are the four D6 settled on: `fresh` and `stale`
	// were the verify-only words of the build that had no arm to report on.
	words := []string{upgradeGateUnchanged, upgradeGateInstalled, upgradeGateRefused, upgradeGateUnknown}
	seen := map[string]bool{}
	for _, w := range words {
		if w == "fresh" || w == "stale" {
			t.Errorf("%q is a retired word of the pre-D5 build (ADR 0092 D6 as amended, ranger-base-estjw)", w)
		}
		if seen[w] {
			t.Errorf("%q is two of the four words — one word per outcome is what makes the field readable", w)
		}
		seen[w] = true
	}
}

// exitErr is a real *exec.ExitError carrying code, taken from a child that
// actually exited with it: the struct cannot be built by hand, and a
// hand-rolled stand-in would pin the shape of this test's own fake.
func exitErr(t *testing.T, code int) error {
	t.Helper()
	err := upgradeStep(io.Discard, t.TempDir(), "sh", "-c", fmt.Sprintf("exit %d", code))
	if err == nil {
		t.Fatalf("sh -c 'exit %d' exited 0", code)
	}
	return err
}

// The upgrade lock lives beside the suite slots and moves with
// XDG_CACHE_HOME, so a box that relocated its cache does not get a lock in a
// directory nothing else uses.
func TestUpgradeLockPathSitsInThePosseCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-zhw0h")
	if got, want := UpgradeLockPath(), "/tmp/xdg-zhw0h/posse/upgrade.lock"; got != want {
		t.Errorf("UpgradeLockPath() = %q, want %q", got, want)
	}
	// And it is NOT the suite queue's override: a seat that moved its suite
	// slots has not asked for the box's upgrade lock to follow them.
	t.Setenv("POSSE_SUITE_LOCK_DIR", "/tmp/elsewhere")
	if got, want := UpgradeLockPath(), "/tmp/xdg-zhw0h/posse/upgrade.lock"; got != want {
		t.Errorf("POSSE_SUITE_LOCK_DIR moved the upgrade lock to %q, want it left at %q", got, want)
	}
	if got, want := SuiteLockDir(), "/tmp/elsewhere"; got != want {
		t.Errorf("SuiteLockDir() = %q, want %q — the refactor that shared the cache dir must not have cost the suite its own override", got, want)
	}
}

// ─── the wiring, ranger-base-anczm ───────────────────────────────────────────
//
// Everything above this line pins a guardrail as a FUNCTION: upgradeFence
// over its two strings, upgradeResolve over a repo, upgradeRunPlan over a
// plan. None of them pinned the verb that calls them, because nothing in
// the tree reached it — and three guardrails were therefore deletable with
// the suite and `make tree-check` both green (MEASURED 2026-11-13, one
// one-line mutation each: the fence call unwired, `if lag.Dirty` made
// `if false && lag.Dirty`, the flock removed; `go test -run TestUpgrade
// ./internal/posse` green for all three, 18 PASS 0 FAIL).
//
// THE SHAPE OF EVERY PIN HERE. A deleted guardrail changes no output — it
// changes which READINGS happen after it. So the seams record the order
// they were asked in, and each pin asserts what the verb did not read as
// well as what it did. An unwired fence reads the binary; a dropped
// dirty-stamp refusal reaches the lock; a removed flock reaches the resolve.

// upgradeSeams is a recording upgradeReads, plus the answers it should give.
// The zero value answers with a binary that cannot name itself, which is
// the verb's second refusal and a safe default for a pin about the first.
type upgradeSeams struct {
	bin     PosseBinary
	lag     LauncherLag
	lockErr error

	read []string // the readings the verb took, in the order it took them
}

func (s *upgradeSeams) reads() upgradeReads {
	return upgradeReads{
		Bin: func() PosseBinary {
			s.read = append(s.read, "bin")
			return s.bin
		},
		Lag: func() LauncherLag {
			s.read = append(s.read, "lag")
			return s.lag
		},
		Lock: func(w io.Writer) (*upgradeLock, error) {
			s.read = append(s.read, "lock")
			if s.lockErr != nil {
				return nil, s.lockErr
			}
			// A zero upgradeLock holds no fd, so it holds no lock and
			// Release is still safe to defer — this pin is about the lock
			// being TAKEN in the right place, and taking the box's real one
			// would serialize the suite against every other seat.
			return &upgradeLock{}, nil
		},
	}
}

func (s *upgradeSeams) order() string { return strings.Join(s.read, ",") }

// D2 is the FIRST statement of the verb, and a refusal by it reads nothing
// at all — not the running binary, not the lag, not the box lock. That is
// the difference between a fence and a fence-shaped function nobody calls.
func TestUpgradeFenceRefusesBeforeTheVerbReadsAnything(t *testing.T) {
	a := wtApp(t)
	write(t, a.ConfigPath, "operator: someone\n") // no install_delegate:
	t.Setenv(EnvPersona, "wheelwright")

	s := &upgradeSeams{}
	err := a.cmdUpgrade(io.Discard, UpgradeOpts{}, s.reads())
	if err == nil {
		t.Fatal("the verb ran under a persona with no install_delegate: configured — the fleet's binary is installable by anybody (ADR 0092 D2)")
	}
	if !strings.Contains(err.Error(), "no "+UpgradeDelegateKey+": is configured") {
		t.Errorf("the refusal is not D2's:\n%v", err)
	}
	if s.order() != "" {
		t.Errorf("the verb read %s after refusing, so the fence is not its first statement — a reading taken past a refusal is a guardrail that only looks like one", s.order())
	}
}

// The other direction, and it is half the claim: the delegate is let
// THROUGH the fence and the verb then reads the running binary. Without
// this arm a fence hard-wired to refuse would pass the pin above.
func TestUpgradeFenceLetsTheDelegateReachTheBinaryRead(t *testing.T) {
	a := wtApp(t)
	write(t, a.ConfigPath, "install_delegate: farrier\n")
	t.Setenv(EnvPersona, "farrier")

	s := &upgradeSeams{} // the zero binary: no Exe, which is refusal two
	err := a.cmdUpgrade(io.Discard, UpgradeOpts{}, s.reads())
	if err == nil {
		t.Fatal("the verb accepted a process that cannot name its own binary — there is no target to replace")
	}
	if !strings.Contains(err.Error(), "cannot name its own binary") {
		t.Errorf("the delegate was refused by something other than the binary read:\n%v", err)
	}
	if s.order() != "bin" {
		t.Errorf("the delegate's run read %s, want bin — the fence must let the one named persona through, and the binary is what it reads next", s.order())
	}
}

// D1: a binary built from a tree with uncommitted edits does not get to
// install the fleet's. The refusal is BEFORE the lock, so a `go run` does
// not queue behind somebody's real twenty-minute upgrade to be told no.
func TestUpgradeRefusesADirtyStampBeforeTakingTheBoxLock(t *testing.T) {
	a := wtApp(t)
	t.Setenv(EnvPersona, "") // the operator, so D2 is not what refuses

	s := &upgradeSeams{
		bin: PosseBinary{Exe: "/nonexistent/bin/posse", Version: "0.4.0+abc1234-dirty"},
		lag: LauncherLag{Version: "0.4.0+abc1234-dirty", Rev: "abc1234", Dirty: true, Repo: "/src/posse", Base: "main"},
	}
	err := a.cmdUpgrade(io.Discard, UpgradeOpts{}, s.reads())
	if err == nil {
		t.Fatal("a binary built from a dirty tree installed the fleet's — its code is in no commit, so nothing can say it came through this gate (ADR 0092 D1)")
	}
	if !strings.Contains(err.Error(), "uncommitted edits") {
		t.Errorf("the refusal is not D1's:\n%v", err)
	}
	// The version is the reader's only handle on WHICH binary was dirty.
	if !strings.Contains(err.Error(), s.bin.Version) {
		t.Errorf("the refusal does not name the running version %q:\n%v", s.bin.Version, err)
	}
	if s.order() != "bin,lag" {
		t.Errorf("the verb read %s, want bin,lag — a refusal that already took the box lock made every other delegate wait for it", s.order())
	}
}

// The same read's other refusal: no checkout to resolve `origin/main` in.
// Also before the lock, and for the same reason.
func TestUpgradeRefusesAnUnresolvableSourceCheckoutBeforeTheLock(t *testing.T) {
	for _, c := range []struct {
		name string
		lag  LauncherLag
	}{
		// Why set at all is an abstention: FindLauncher could not take the
		// reading, so there is no checkout it can name.
		{"no reading", LauncherLag{Rev: "abc1234", Why: "no posse checkout is configured"}},
		// And a reading that answered with no repo, which is the same hole
		// by another route.
		{"no repo", LauncherLag{Rev: "abc1234", Base: "main"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := wtApp(t)
			t.Setenv(EnvPersona, "")
			s := &upgradeSeams{bin: PosseBinary{Exe: "/nonexistent/bin/posse", Version: "0.4.0+abc1234"}, lag: c.lag}
			err := a.cmdUpgrade(io.Discard, UpgradeOpts{}, s.reads())
			if err == nil {
				t.Fatal("the verb ran with no source checkout to read origin/main in (ADR 0092 D3.1)")
			}
			if !strings.Contains(err.Error(), upgradeDefaultRef) {
				t.Errorf("the refusal does not name the ref it cannot read:\n%v", err)
			}
			if s.order() != "bin,lag" {
				t.Errorf("the verb read %s, want bin,lag", s.order())
			}
		})
	}
}

// D3: the box-wide lock is taken BEFORE the resolve, so two delegates who
// typed the verb at once do not both resolve, both test and both swap. A
// lock the verb never takes reaches the resolve instead — and that is what
// the stubbed steps below are watching for.
func TestUpgradeTakesTheBoxLockBeforeItResolvesAnything(t *testing.T) {
	f := newUpgradeFixture(t)
	a := wtApp(t)
	t.Setenv(EnvPersona, "")
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error {
			t.Error("`make test` ran although the box upgrade lock was never granted — this run resolved and spent a suite behind another upgrade's back (ADR 0092 D3)")
			return errors.New("exit status 1")
		},
		func(w io.Writer, dir, repo, out string) error { t.Error("the build ran with no lock held"); return nil },
		func(w io.Writer, wt string, plan upgradePlan) string {
			t.Error("the epilogue ran with no lock held")
			return upgradeGateUnchanged
		})

	held := errors.New("upgrade lock: held by another upgrade")
	s := &upgradeSeams{
		bin:     PosseBinary{Exe: f.target, Version: "0.4.0+" + shortSha(f.a)},
		lag:     LauncherLag{Version: "0.4.0+" + shortSha(f.a), Rev: f.a, Repo: f.repo, Base: "main"},
		lockErr: held,
	}
	err := a.cmdUpgrade(io.Discard, UpgradeOpts{}, s.reads())
	if !errors.Is(err, held) {
		t.Errorf("cmdUpgrade returned %v, want the lock's own error — a lock refusal the verb swallows is a run that proceeded unserialized", err)
	}
	if s.order() != "bin,lag,lock" {
		t.Errorf("the verb read %s, want bin,lag,lock", s.order())
	}
}

// The target is the binary THIS PROCESS IS RUNNING OUT OF, and the source
// checkout is the one the lag reading named. Both are read in the verb and
// neither appears anywhere else, so this is the only place either can be
// pinned.
//
// It runs the plan as far as the suite and no further: a red `make test`
// refuses before anything is written (the pin above this section proves
// that separately), which makes the header line a free and complete reading
// of what the verb resolved.
func TestUpgradeTargetsTheRunningBinaryAndTheLagsCheckout(t *testing.T) {
	f := newUpgradeFixture(t)
	a := wtApp(t)
	t.Setenv(EnvPersona, "")
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error { return errors.New("exit status 1") },
		func(w io.Writer, dir, repo, out string) error { t.Error("the build ran after a red suite"); return nil },
		func(w io.Writer, wt string, plan upgradePlan) string {
			t.Error("the epilogue ran after a red suite")
			return upgradeGateUnchanged
		})

	s := &upgradeSeams{
		bin: PosseBinary{Exe: f.target, Version: "0.4.0+" + shortSha(f.a)},
		lag: LauncherLag{Version: "0.4.0+" + shortSha(f.a), Rev: f.a, Repo: f.repo, Base: "main"},
	}
	var out bytes.Buffer
	if err := a.cmdUpgrade(&out, UpgradeOpts{}, s.reads()); err == nil {
		t.Fatal("a red `make test` did not refuse the upgrade")
	} else if !strings.Contains(err.Error(), "did not pass") {
		t.Fatalf("the refusal is not the suite's:\n%v", err)
	}
	if s.order() != "bin,lag,lock" {
		t.Fatalf("the verb read %s, want bin,lag,lock", s.order())
	}

	head := strings.SplitN(out.String(), "\n", 2)[0]
	want := fmt.Sprintf("posse upgrade · %s → %s (2 commit(s)) · source %s · target %s",
		shortSha(f.a), shortSha(f.c), AbbrevHome(f.repo), AbbrevHome(f.target))
	if head != want {
		t.Errorf("the run's first line is\n  %q\nwant\n  %q\n— the target is RunningPosse().Exe and the source is the lag reading's checkout; a second spelling of either is a path to go wrong on a box that moved it (upgrade.go's WHICH BINARY note)", head, want)
	}

	// And `<sha>` reaches the resolve, which is the other thing the verb
	// plumbs and nothing else can see.
	out.Reset()
	s2 := &upgradeSeams{bin: s.bin, lag: s.lag}
	if err := a.cmdUpgrade(&out, UpgradeOpts{Sha: " " + f.b + " "}, s2.reads()); err == nil {
		t.Fatal("a red `make test` did not refuse the upgrade")
	}
	head = strings.SplitN(out.String(), "\n", 2)[0]
	if !strings.Contains(head, fmt.Sprintf("%s → %s (1 commit(s))", shortSha(f.a), shortSha(f.b))) {
		t.Errorf("`posse upgrade <sha>` resolved something other than the sha it was given:\n  %q", head)
	}
}
