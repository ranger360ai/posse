//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-x0xct / ADR 0073 B2 — the bd shim's write-lock
// arm (renderBdWriteTail) and the subcommand it calls (RunGatesBdWrite,
// bdwriteshim.go). This is the WIRING pin: a read verb never execs posse
// at all, a write verb does and inherits its exit code, a write the
// line-cliff judge cannot project (dep add) still goes through posse
// rather than being skipped, and the degrade-to-unlocked arm fires when
// posse cannot be found at render time. The judge itself (ProjectBdWrite)
// is B1's and already pinned in bdline_qa_test.go; the lock itself is
// bdwritelock_test.go's.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBdWritePosse builds a real posse binary: ADR 0073 D2's arm calls
// out to "the absolute posse path recorded at render time", and a fake
// stub cannot stand in for RunGatesBdWrite's own lock-and-judge-and-exec,
// only record that a call happened (which TestRenderedShimSkipsPosseForReads
// needs instead).
func buildBdWritePosse(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/posse: %v\n%s", err, out)
	}
	return bin
}

// bdWriteShimFixture renders the bd shim with renderBdWriteTail wired to
// posseBin, in front of a fake "bd" that only records whether it ran and
// with what argv — real bd semantics are not this arm's concern (B1 and
// bdlinelive_test.go already cover ProjectBdWrite against the real
// binary).
func bdWriteShimFixture(t *testing.T, posseBin string) (shim, bdRanMarker, bdArgvFile, log string) {
	t.Helper()
	dir := t.TempDir()
	realBin := filepath.Join(dir, "real", "bd")
	bdRanMarker = filepath.Join(dir, "bd-ran")
	bdArgvFile = filepath.Join(dir, "bd-argv")
	write(t, realBin, "#!/bin/sh\necho ran >> "+shQuote(bdRanMarker)+"\necho \"$@\" >> "+shQuote(bdArgvFile)+"\necho real bd \"$@\"\n")
	if err := os.Chmod(realBin, 0o755); err != nil {
		t.Fatal(err)
	}
	log = filepath.Join(dir, "refusals.log")
	rules := ParseShimRules([]string{"Bash(bd daemon:*)"})["bd"]
	body := renderShim("security", "bd", realBin, log, "/bin/date", rules, nil, posseBin)
	shim = filepath.Join(dir, "bd")
	if err := WriteExecutable(shim, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return shim, bdRanMarker, bdArgvFile, log
}

// runWriteShim runs the rendered shim the way a persona's session does:
// BEADS_DIR and RHQ_PERSONA inherited from the environment, exactly as a
// real bd call would see them, plus XDG_CACHE_HOME pinned to a scratch
// dir so the lock this arm takes never touches the real
// ~/.cache/posse/bd-write (lock files hold nothing per ADR 0073, but a
// test has no business writing there either).
func runWriteShim(t *testing.T, shim, beadsDir, cacheDir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(shim, args...)
	cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir, "RHQ_PERSONA=security", "XDG_CACHE_HOME="+cacheDir)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running the shim: %v", err)
	}
	return out.String(), errb.String(), code
}

func TestRenderedShimRefusesAndPassesTheLineCliff(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	posseBin := buildBdWritePosse(t)
	cacheDir := t.TempDir()

	// The ADR's own "a no-db rig at 60 KB" shape (Context, Verification
	// item 2): one row at 60 KB, a 6 KB file that crosses the cliff and a
	// 1 KB file that does not.
	const before = 60 * 1024
	base := bdRow("t-2", "")
	pad := before - len(base)
	if pad < 0 {
		t.Fatalf("fixture premise: bdRow's own envelope is already %d bytes, wider than the 60 KB target", len(base))
	}
	census := writeCensus(t, bdRow("t-1", "small"), bdRow("t-2", strings.Repeat("a", pad)), bdRow("t-3", "small too"))
	beadsDir := filepath.Dir(census)
	before1, err := os.ReadFile(census)
	if err != nil {
		t.Fatal(err)
	}

	file6k := filepath.Join(t.TempDir(), "6k.txt")
	if err := os.WriteFile(file6k, []byte(strings.Repeat("a", 6000)), 0o644); err != nil {
		t.Fatal(err)
	}
	file1k := filepath.Join(t.TempDir(), "1k.txt")
	if err := os.WriteFile(file1k, []byte(strings.Repeat("a", 1000)), 0o644); err != nil {
		t.Fatal(err)
	}
	if proj, ok, perr := ProjectBdWrite(census, []string{"comments", "add", "t-2", "-f", file6k}); perr != nil || !ok || proj.After <= BdLineLimit-BdLineHeadroom {
		t.Fatalf("fixture premise: the 6 KB file must cross the line cliff: proj=%+v ok=%v err=%v", proj, ok, perr)
	}
	if proj, ok, perr := ProjectBdWrite(census, []string{"comments", "add", "t-2", "-f", file1k}); perr != nil || !ok || proj.After > BdLineLimit-BdLineHeadroom {
		t.Fatalf("fixture premise: the 1 KB file must stay under the line cliff: proj=%+v ok=%v err=%v", proj, ok, perr)
	}

	shim, bdRan, _, log := bdWriteShimFixture(t, posseBin)

	// The crossing write: refused, bd never runs, the census line is
	// untouched, and refusals.log carries the class line with no argv.
	_, stderr, code := runWriteShim(t, shim, beadsDir, cacheDir, "comments", "add", "t-2", "-f", file6k)
	if code != 1 {
		t.Fatalf("comments add -f 6k.txt: exit %d, want 1\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "refused by posse gate:") || !strings.Contains(stderr, "t-2") || !strings.Contains(stderr, "ADR 0072") {
		t.Errorf("refusal text missing the ADR 0072 D2 shape:\n%s", stderr)
	}
	if _, serr := os.Stat(bdRan); serr == nil {
		t.Errorf("bd ran on a refused write")
	}
	after1, err := os.ReadFile(census)
	if err != nil {
		t.Fatal(err)
	}
	if string(after1) != string(before1) {
		t.Errorf("the census line changed on a refused write")
	}
	logBody, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("refusals.log: %v", err)
	}
	if !strings.Contains(string(logBody), "bd line cliff [bd shim] t-2") || !strings.Contains(string(logBody), "session security") {
		t.Errorf("refusals.log missing the D2 line:\n%s", logBody)
	}
	if strings.Contains(string(logBody), "6k.txt") || strings.Contains(string(logBody), strings.Repeat("a", 50)) {
		t.Errorf("refusals.log echoed argv or file content:\n%s", logBody)
	}

	// The same call under the line: passes, bd runs.
	_, stderr, code = runWriteShim(t, shim, beadsDir, cacheDir, "comments", "add", "t-2", "-f", file1k)
	if code != 0 {
		t.Fatalf("comments add -f 1k.txt: exit %d, want 0\nstderr:\n%s", code, stderr)
	}
	if _, serr := os.Stat(bdRan); serr != nil {
		t.Errorf("bd did not run on a passing write: %v", serr)
	}
	if err := os.Remove(bdRan); err != nil {
		t.Fatal(err)
	}

	// A residual the judge declines to touch — stdin — passes too.
	cmd := exec.Command(shim, "comments", "add", "t-2", "-f", "-")
	cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir, "RHQ_PERSONA=security", "XDG_CACHE_HOME="+cacheDir)
	cmd.Stdin = strings.NewReader(strings.Repeat("a", 6000))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("comments add -f - (stdin): %v\n%s", err, out)
	}
	if _, serr := os.Stat(bdRan); serr != nil {
		t.Errorf("bd did not run for a stdin-sourced comment: %v", serr)
	}
}

// TestRenderedShimSkipsPosseForReads is a read verb never execing posse
// at all — asserted through a stub that records every call it is given,
// so "never called" is a fact about the recording file and not an
// inference from a passing exit code.
func TestRenderedShimSkipsPosseForReads(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	posseCalls := filepath.Join(dir, "posse-calls")
	posseStub := filepath.Join(dir, "posse-stub")
	write(t, posseStub, "#!/bin/sh\necho \"$@\" >> "+shQuote(posseCalls)+"\nexit 0\n")
	if err := os.Chmod(posseStub, 0o755); err != nil {
		t.Fatal(err)
	}

	shim, bdRan, _, _ := bdWriteShimFixture(t, posseStub)
	census := writeCensus(t, bdRow("t-1", "small"))
	beadsDir := filepath.Dir(census)

	_, stderr, code := runWriteShim(t, shim, beadsDir, t.TempDir(), "list", "--limit", "1")
	if code != 0 {
		t.Fatalf("list --limit 1: exit %d, want 0\nstderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(bdRan); err != nil {
		t.Fatalf("a read verb did not run bd: %v", err)
	}
	if _, err := os.Stat(posseCalls); err == nil {
		t.Errorf("a read verb called posse at all: %s", posseCalls)
	}
}

// A write ProjectBdWrite has never heard of — `dep add` — still has to
// route through posse (so D1's lock covers it), even though the line-
// cliff judge inside "gates bd-write" will find nothing to project and
// let it straight through to bd.
func TestRenderedShimRoutesAnUnprojectedWriteThroughPosse(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	posseBin := buildBdWritePosse(t)
	shim, bdRan, bdArgv, _ := bdWriteShimFixture(t, posseBin)
	census := writeCensus(t, bdRow("t-1", "small"), bdRow("t-2", "small too"))
	beadsDir := filepath.Dir(census)

	_, stderr, code := runWriteShim(t, shim, beadsDir, t.TempDir(), "dep", "add", "t-1", "t-2")
	if code != 0 {
		t.Fatalf("dep add t-1 t-2: exit %d, want 0\nstderr:\n%s", code, stderr)
	}
	if _, err := os.Stat(bdRan); err != nil {
		t.Fatalf("an unprojected write did not run bd: %v", err)
	}
	argv, err := os.ReadFile(bdArgv)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(argv)) != "dep add t-1 t-2" {
		t.Errorf("bd's own argv was not preserved across the posse hop: %q", argv)
	}
}

// posse missing at render time: the write still runs, unlocked and
// unjudged, with one stderr line saying so — ADR 0073 D2's own fail-open
// posture, never a wall that blocks every bd write because posse moved.
func TestRenderedShimDegradesToUnlockedWhenPosseIsMissing(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	shim, bdRan, _, _ := bdWriteShimFixture(t, "")
	census := writeCensus(t, bdRow("t-1", "small"))
	beadsDir := filepath.Dir(census)

	_, stderr, code := runWriteShim(t, shim, beadsDir, t.TempDir(), "close", "t-1")
	if code != 0 {
		t.Fatalf("close t-1 with no posse: exit %d, want 0\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "ADR 0073") {
		t.Errorf("missing the degrade notice:\n%s", stderr)
	}
	if _, err := os.Stat(bdRan); err != nil {
		t.Fatalf("a degraded write did not run bd: %v", err)
	}
}

// posseSupportsBdWrite is RenderGates' own guard against a REAL, working
// posse that simply predates this subcommand (ranger-base-x0xct's own
// bootstrap window — this box's installed posse is exactly this case).
// TestShippedPIDsLetBeadsOwnHooksRun (internal/treepins) is the end-to-end
// pin that this guard keeps green; these two are the unit of it.
func TestPosseSupportsBdWriteOnARealBinary(t *testing.T) {
	t.Parallel()
	if !posseSupportsBdWrite(buildBdWritePosse(t)) {
		t.Error("a freshly built posse must answer yes for its own subcommand")
	}
}

func TestPosseSupportsBdWriteOnAStaleBinary(t *testing.T) {
	t.Parallel()
	// The exact shape a posse built before this subcommand existed takes:
	// Die("no such agent: %s ...") on an unrecognized top-level verb,
	// exit 1 — never reaching bd.
	stale := filepath.Join(t.TempDir(), "posse")
	write(t, stale, "#!/bin/sh\necho \"posse: no such agent: $1\" >&2\nexit 1\n")
	if err := os.Chmod(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if posseSupportsBdWrite(stale) {
		t.Error("a binary that cannot even run the subcommand must answer no")
	}
	if posseSupportsBdWrite(filepath.Join(t.TempDir(), "does-not-exist")) {
		t.Error("a binary that is not there at all must answer no")
	}
}
