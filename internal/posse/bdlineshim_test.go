//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-a0vdz / ADR 0072 B2 — the bd shim's write-time
// arm (renderBdLineArm) and the subcommand it calls (RunGatesBdLine,
// bdlineshim.go). The judge itself (ProjectBdWrite, CensusLongLines) is
// B1's and already pinned in bdline_qa_test.go; this file pins the WIRING:
// the shim skips a read verb entirely (never execs posse for one), calls a
// REAL posse for the four write verbs, propagates its exit code without
// ever running bd on a refusal, and never echoes argv anywhere a refusal
// can be read. Its own census fixture matches the ADR's "a no-db rig at
// 60 KB" shape (ADR 0072 Context / Verification item 2).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBdLinePosse builds a real posse binary — the one pin here that must
// exercise RunGatesBdLine as a separate process, since ADR 0072 D2's arm
// calls out to "the absolute posse path recorded at render time" and a
// fake stub cannot stand in for that call's own judgment (only record that
// a call happened, which TestRenderedShimSkipsPosseForReads needs instead).
func buildBdLinePosse(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/posse: %v\n%s", err, out)
	}
	return bin
}

// bdLineShimFixture renders the bd shim with renderBdLineArm wired to
// posseBin, in front of a fake "bd" that only records whether it ran —
// real bd semantics are not this arm's concern (B1 and bdlinelive_test.go
// already cover ProjectBdWrite against the real binary).
func bdLineShimFixture(t *testing.T, posseBin string) (shim, bdRanMarker, log string) {
	t.Helper()
	dir := t.TempDir()
	realBin := filepath.Join(dir, "real", "bd")
	bdRanMarker = filepath.Join(dir, "bd-ran")
	write(t, realBin, "#!/bin/sh\necho ran >> "+shQuote(bdRanMarker)+"\necho real bd \"$@\"\n")
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
	return shim, bdRanMarker, log
}

// runShim runs the rendered shim the way a persona's session does: BEADS_DIR
// and RHQ_PERSONA inherited from the environment, exactly as a real bd call
// would see them. stdin, when non-empty, is what -f - would read.
func runShim(t *testing.T, shim, beadsDir, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(shim, args...)
	cmd.Env = append(os.Environ(), "BEADS_DIR="+beadsDir, "RHQ_PERSONA=security")
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
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
	posseBin := buildBdLinePosse(t)

	// The ADR's own "a no-db rig at 60 KB" shape (Context, Verification
	// item 2): one row at 60 KB, a 6 KB file that crosses the cliff and a
	// 1 KB file that does not — margin wide enough that the exact envelope
	// bytes ProjectBdWrite adds (already pinned in bdline_qa_test.go)
	// cannot flip either verdict.
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

	shim, bdRan, log := bdLineShimFixture(t, posseBin)

	// The crossing write: refused, bd never runs, the census line is
	// untouched, and refusals.log carries the class line with no argv.
	_, stderr, code := runShim(t, shim, beadsDir, "", "comments", "add", "t-2", "-f", file6k)
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
	_, stderr, code = runShim(t, shim, beadsDir, "", "comments", "add", "t-2", "-f", file1k)
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
	_, stderr, code = runShim(t, shim, beadsDir, strings.Repeat("a", 6000), "comments", "add", "t-2", "-f", "-")
	if code != 0 {
		t.Fatalf("comments add -f - (stdin): exit %d, want 0\nstderr:\n%s", code, stderr)
	}
	if _, serr := os.Stat(bdRan); serr != nil {
		t.Errorf("bd did not run for a stdin-sourced comment: %v", serr)
	}
}

// TestRenderedShimSkipsPosseForReads is ADR 0072 Verification item 7's
// shape: a read verb never execs posse at all — asserted through a stub
// that records every call it is given, so "never called" is a fact about
// the recording file and not an inference from a passing exit code.
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

	shim, bdRan, _ := bdLineShimFixture(t, posseStub)
	census := writeCensus(t, bdRow("t-1", "small"))
	beadsDir := filepath.Dir(census)

	_, stderr, code := runShim(t, shim, beadsDir, "", "list", "--limit", "1")
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
