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
func stubUpgradeSteps(t *testing.T, test func(io.Writer, string) error, build func(io.Writer, string, string, string) error, epilogue func(io.Writer, string, string) string) {
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
		func(w io.Writer, wt, target string) string {
			t.Error("the epilogue ran after a red suite")
			return "fresh"
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
		func(w io.Writer, wt, target string) string {
			t.Error("the epilogue ran after a failed build")
			return "fresh"
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
		func(w io.Writer, wt, target string) string { return "fresh" })

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
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error {
			order = append(order, "test")
			// The worktree it is handed is a real detached checkout of the
			// commit being installed — not the source repo, and not main.
			if got := strings.TrimSpace(mustGit(t, dir, "rev-parse", "HEAD")); got != f.c {
				t.Errorf("`make test` ran at %s, want the target commit %s", got, f.c)
			}
			if sameDir, _ := MainCheckout(dir); sameDir == dir {
				t.Errorf("`make test` ran in %s, which is a main checkout — D3.2 asks for a throwaway worktree", dir)
			}
			return nil
		},
		func(w io.Writer, dir, repo, out string) error {
			order = append(order, "build")
			if repo != f.repo {
				t.Errorf("the build was handed %q as the source checkout to stamp, want %q — a stamp naming the throwaway worktree is dead by the time anything reads it (ranger-base-eiwhk)", repo, f.repo)
			}
			return os.WriteFile(out, built.bytes(), 0o644)
		},
		func(w io.Writer, wt, target string) string {
			order = append(order, "epilogue")
			if target != f.target {
				t.Errorf("the epilogue was handed %q as POSSE, want the just-installed %q", target, f.target)
			}
			return "fresh"
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
	want := fmt.Sprintf("upgraded %s..%s (2 commits) · gate fresh · rollback: %s", shortSha(f.a), shortSha(f.c), upgradeRollbackCmd(f.target))
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
// So this reader is the same probe at pin scale: it stats and reads the
// target as fast as it can for the whole swap, and the claim is that every
// observation is one of exactly two complete files. A swap that went back
// to install(1), or that truncated the target in place, reds here.
func TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "posse")
	// Big enough that a non-atomic write has a window a reader can land in
	// — the 58 ms measured above was over 64 MiB. 8 MiB keeps the pin under
	// a second and still leaves a window orders of magnitude wider than one
	// read.
	old := bytes.Repeat([]byte("O"), 8<<20)
	fresh := bytes.Repeat([]byte("N"), 8<<20)
	if err := WriteExecutable(target, old, 0o755); err != nil {
		t.Fatal(err)
	}
	staged := target + ".new"
	if err := os.WriteFile(staged, fresh, 0o644); err != nil {
		t.Fatal(err)
	}

	var stop atomic.Bool
	var reads, absent, partial, sawOld, sawNew atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() {
			reads.Add(1)
			body, err := os.ReadFile(target)
			switch {
			case err != nil:
				absent.Add(1)
			case bytes.Equal(body, old):
				sawOld.Add(1)
			case bytes.Equal(body, fresh):
				sawNew.Add(1)
			default:
				partial.Add(1)
			}
		}
	}()
	// Let the reader get going, so the swap lands inside its loop rather
	// than before it.
	time.Sleep(20 * time.Millisecond)
	if err := upgradeSwap(target, staged); err != nil {
		stop.Store(true)
		<-done
		t.Fatalf("upgradeSwap: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	stop.Store(true)
	<-done

	// THE POSITIVE WITNESS, and this pin needs one more than most: a reader
	// descheduled for the whole swap window reports zero absent and zero
	// partial for the same reason a correct swap does, which is a vacuous
	// pass that no absence check can tell from the real thing. Seeing BOTH
	// complete versions is proof the loop was actually running on either
	// side of the rename.
	if sawOld.Load() == 0 || sawNew.Load() == 0 {
		t.Fatalf("the reader saw the old binary %d times and the new one %d times over %d reads — it was not running across the swap, so the absence below measures nothing",
			sawOld.Load(), sawNew.Load(), reads.Load())
	}
	if absent.Load() != 0 || partial.Load() != 0 {
		t.Errorf("over %d reads the target was absent %d times and neither-version %d times — the swap is not atomic, and a launch inside that window execs a missing or short binary (ADR 0092 D3.5)",
			reads.Load(), absent.Load(), partial.Load())
	}
	if body, _ := os.ReadFile(target); !bytes.Equal(body, fresh) {
		t.Error("the target does not hold the new binary after the swap")
	}
	if body, _ := os.ReadFile(upgradePrevPath(target)); !bytes.Equal(body, old) {
		t.Error("the .prev copy does not hold the binary that was live")
	}
}

// The gate word is read off verify-gate-freshness's own exit codes
// (0 clean, 1 findings, 2 nothing measured), and the default is the one
// that claims nothing.
func TestUpgradeGateWordReadsTheVerifierExitCode(t *testing.T) {
	t.Parallel()
	if got := upgradeGateWord(nil); got != "fresh" {
		t.Errorf("exit 0 reads as %q, want fresh", got)
	}
	for code, want := range map[int]string{1: "stale", 2: "unknown", 127: "unknown"} {
		if got := upgradeGateWord(exitErr(t, code)); got != want {
			t.Errorf("exit %d reads as %q, want %q", code, got, want)
		}
	}
	// An error that is not an exit at all — the script could not be run —
	// must not read as clean.
	if got := upgradeGateWord(errors.New("no such file")); got != "unknown" {
		t.Errorf("a child that never ran reads as %q, want unknown", got)
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
		func(w io.Writer, wt, target string) string {
			t.Error("the epilogue ran with no lock held")
			return "fresh"
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
		func(w io.Writer, wt, target string) string {
			t.Error("the epilogue ran after a red suite")
			return "fresh"
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
