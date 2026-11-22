package treepins

// QA pins for ranger-base-nw9zg — the test-binary churn guard.
//
// Claim: `go test <pkg>` pays macOS a Gatekeeper assessment for a binary the
// box has already scanned, every single invocation, and scripts/gotest.sh is
// the way not to.
//
// MEASURED 2026-09-02 on the box that runs every session's suite, by execution
// and not by reading:
//
//   - `go test` COPIES the linked test binary out of the build cache into a
//     fresh $TMPDIR/go-buildNNN work dir per invocation. Two cached runs of the
//     identical command over ./internal/posse: inode 243177760 and inode
//     243178437, link count 1 each, one content.
//   - the first exec of a fresh copy of that 23 MB binary, with -test.run set
//     to a pattern matching nothing so the figure is startup and nothing else,
//     costs 0.806s / 1.066s / 1.059s. The second exec of the same file costs
//     0.030s / 0.035s / 0.039s.
//   - and the PATH is not what is keyed. 200 execs of 200 hard links to one
//     inode: 1 assessment. 200 execs of 200 byte-identical copies: 217. 200
//     execs of one path: 2. Idle controls of the same wall length on either
//     side of each arm put the box's own background at 0.8-5.3/sec.
//
// That last arm is why this guard is a binary cache and not a stable GOTMPDIR.
// One directory for every link still writes a new file per link, and a new
// file is a new assessment whatever it is called.
//
// TWO ARMS, because the guard can be lost two ways:
//
//  1. the script survives and nothing runs its self-test. `make verify-gotest`
//     runs it; a plain `go test ./...` does not, so a gutted script would ship
//     green. Arm 2 runs it from inside the suite.
//  2. `make verify-gotest` is deleted or stops calling the script, at which
//     point the crew's only re-measuring artifact is gone. Arm 1 pins the
//     recipe.
//
// And a third arm, because arms 1 and 2 together are still green over a
// self-test whose failure path prints `ok`. That mutation SURVIVED the first
// mutation pass — every label was present, the exit status was zero, and both
// arms passed over a guard that could no longer fail. Arm 3 breaks a COPY of
// the script on purpose and requires the self-test to notice, which is the
// only way to know the other two arms are reading a live instrument.
//
// What this does NOT claim, and no close should: that it moves this box's
// Gatekeeper load. Three packages here have tests, against 1644 DISTINCT
// executables assessed in a ten-minute window, and 590 of 613 assessments in a
// five-minute sample carry no code-signing identifier at all where a Go-linked
// binary reports `a.out`. The Go toolchain is a few percent of the rate. What
// this fixes is ours: ~1 second and one XProtect yara scan per invocation.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The wrapper, by path. Named rather than pattern-matched: this is one
// specific script carrying one specific measurement.
const testBinWrapper = "scripts/gotest.sh"

// Arm 1: `make verify-gotest` still runs the script's own self-test. Without
// this the second arm is the only thing keeping the guard alive, and a reader
// looking for the re-measuring command finds nothing.
func TestQAMakeVerifyGotestRunsTheWrapperSelfTest(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	recipe := makeRecipe(string(makefile), "verify-gotest")
	if len(recipe) == 0 {
		t.Fatal("the Makefile has no `verify-gotest` target — the guard has no re-measuring command")
	}

	var found bool
	for _, l := range recipe {
		if isComment(l) {
			continue
		}
		if strings.Contains(l, testBinWrapper) && strings.Contains(l, "--self-test") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("`make verify-gotest` no longer runs %s --self-test:\n%s",
			testBinWrapper, strings.Join(recipe, "\n"))
	}
}

// Arm 2: the self-test still proves what it says, arm by arm. Required by arm
// LABEL rather than by the guard's own strings — a self-test that quietly
// stopped exercising the reuse would otherwise go green by deletion.
//
// The pairs matter more than the count. "reuse: unchanged package keeps one
// inode" is equally green over a script that ignores the source entirely and
// reruns a stale binary forever; "control: a changed test file rebuilds" and
// "control: the new test actually ran" are what stop that, and they are why a
// content-blind cache key was killed by this file rather than by a reviewer.
func TestQATheWrapperStillProvesItReusesTheBinary(t *testing.T) {
	if _, err := os.Stat(testBinWrapper); err != nil {
		t.Fatalf("%s is gone: %v", testBinWrapper, err)
	}

	out, err := exec.Command("bash", testBinWrapper, "--self-test").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test failed: %v\n%s", testBinWrapper, err, out)
	}
	got := string(out)

	for _, want := range []string{
		"reuse: unchanged package keeps one inode",
		"reuse: no second binary was written",
		"control: the plant runs when exec'd",
		"poison: a planted file under the cached name is not exec'd",
		"poison: the plant is replaced by the fresh build",
		"control: a changed test file rebuilds",
		"control: the new test actually ran",
		"exit status: a red test reds the wrapper",
		"exit status: a green test greens the wrapper",
		"build failure: a package that will not compile reds",
		"prune: keeps POSSE_TESTBIN_KEEP per package",
		// ranger-base-nywsk. The poison arm plants IN PLACE over a
		// binary the self-test has just exec'd twice, and the kernel
		// releases an exec'd file's write deny after the process that
		// held it is gone — so that write is ETXTBSY under load, which
		// took the whole self-test down with `set -e` and reddened
		// `make tree-check` for every seat on the box. These two are
		// the deterministic half of the same refusal: the write really
		// is refused while a process executes the file, and the
		// wrapper's retrying plant waits it out. Required here because
		// the bug itself showed once at a load average of 14 and a
		// re-run in isolation was green, so nothing else would notice
		// the fix being removed.
		//
		// THESE TWO LABELS HAVE TWO GREEN READINGS, AND THE `ok` LINE
		// SAYS WHICH (ranger-base-5oiz4). The deny is not enforced on
		// every kernel: MEASURED 2026-11-22 on macos-26-arm64 (run
		// 44968403951, job 137042528883) the naive write over the
		// running holder LANDED and the plant's first attempt landed
		// too, so both arms FAILed on every macos arm-1 leg over a
		// premise that is linux's. The control is a probe now and the
		// plant arm reads its verdict, so a box with no deny prints
		// `ok` with the reading appended rather than reddening — which
		// is why the Contains below is on the label and not on the
		// whole line. What the row still holds there is that the plant
		// LANDS, read back against a third content; the retry's own
		// evidence is the enforcing kernel's, and ci.yml's ubuntu
		// arm-1 leg supplies one on every run.
		"control: a write over a live executable is refused",
		"plant: the in-place plant waits out a live executable",
	} {
		if !strings.Contains(got, "ok    "+want) {
			t.Errorf("%s --self-test no longer proves %q — the arm is gone, failing, or renamed\n%s",
				testBinWrapper, want, got)
		}
	}
}

// Arm 3: the self-test can actually fail. A guard is worth nothing until it has
// been shown to refuse — a `--self-test` that prints its arm labels and exits 0
// no matter what satisfies arms 1 and 2 exactly as well as a working one does,
// and that is not a hypothetical: it is the one mutant that survived the first
// pass over this file.
//
// So: take the real script, disable the one branch that makes reuse reuse, and
// require the self-test to go red AND to name the reuse arm while doing it.
func TestQATheWrapperSelfTestCanFail(t *testing.T) {
	src, err := os.ReadFile(testBinWrapper)
	if err != nil {
		t.Fatal(err)
	}

	// The branch that hands back an already-assessed file instead of the
	// one just built. If this string is gone the mutation lands nowhere and
	// the arm measures nothing, so say so rather than passing.
	const reuseBranch = "\tif [ -f \"$target\" ]; then"
	if !strings.Contains(string(src), reuseBranch) {
		t.Fatalf("%s no longer contains the reuse branch this arm mutates (%q) — "+
			"the arm cannot fail and is therefore not evidence", testBinWrapper, reuseBranch)
	}
	broken := strings.Replace(string(src), reuseBranch, "\tif false; then", 1)

	dir := t.TempDir()
	path := filepath.Join(dir, "gotest-broken.sh")
	if err := WriteExecutable(path, []byte(broken), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("bash", path, "--self-test").CombinedOutput()
	if err == nil {
		t.Fatalf("%s --self-test passed with reuse disabled: the self-test cannot fail, "+
			"so the other arms in this file prove nothing\n%s", testBinWrapper, out)
	}
	if !strings.Contains(string(out), "FAIL  reuse: unchanged package keeps one inode") {
		t.Errorf("the self-test failed, but not on the arm that was broken — "+
			"it is refusing for some other reason and is not measuring reuse\n%s", out)
	}
}

// Arm 4 (ranger-base-g66m3): the wrapper's DEFAULT cache is outside every
// home dir a walled seat is granted write on. The cache holds files the
// wrapper execs, so a writer of it picks what the next run executes — the
// operator's uncaged run included. The self-test's poison arm proves a
// planted file is not exec'd; this arm keeps the default where no seat can
// plant in the first place, which also closes the window between the
// wrapper's byte comparison and its exec.
//
// Both sides are READ, not restated: the default from the script's own
// `CACHE=` line, the grants from homeCacheGrants' string literals in
// internal/posse/seatbelt.go, every GOOS arm of it. Blindness control: the
// default this arm replaced, `.cache/posse/testbin`, must be caught by the
// same predicate over the same grant list.
func TestQATheWrapperCacheDefaultIsOutsideEverySeatWriteGrant(t *testing.T) {
	src, err := os.ReadFile(testBinWrapper)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "CACHE=${POSSE_TESTBIN_CACHE:-$HOME/"
	var def string
	for _, l := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(l, prefix) && strings.HasSuffix(l, "}") {
			def = strings.TrimSuffix(strings.TrimPrefix(l, prefix), "}")
			break
		}
	}
	if def == "" {
		t.Fatalf("%s has no `%s...}` line — this arm cannot read the default cache", testBinWrapper, prefix)
	}

	sb, err := os.ReadFile("internal/posse/seatbelt.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(sb)
	i := strings.Index(body, "\nfunc homeCacheGrants(")
	if i < 0 {
		t.Fatal("internal/posse/seatbelt.go has no homeCacheGrants — this arm cannot read the grant list")
	}
	body = body[i:]
	if j := strings.Index(body, "\n}\n"); j >= 0 {
		body = body[:j]
	}
	// Split on quotes: every odd piece is a string literal.
	var lits []string
	for k, part := range strings.Split(body, "\"") {
		if k%2 == 1 {
			lits = append(lits, part)
		}
	}
	if len(lits) == 0 {
		t.Fatal("read no string literals out of homeCacheGrants — the reader is blind")
	}

	overlaps := func(p string) (string, bool) {
		for _, g := range lits {
			if p == g || strings.HasPrefix(p, g+"/") || strings.HasPrefix(g, p+"/") {
				return g, true
			}
		}
		return "", false
	}
	if _, ok := overlaps(".cache/posse/testbin"); !ok {
		t.Fatalf("the old default .cache/posse/testbin is not caught against %q — the predicate cannot fail", lits)
	}
	if g, ok := overlaps(def); ok {
		t.Errorf("%s's default cache $HOME/%s overlaps the seat write grant %q (homeCacheGrants): "+
			"a walled seat can plant the binary the next run execs", testBinWrapper, def, g)
	}
}
