package posse

// PARKED QA pin for ranger-base-t07yx finding 1, found verifying
// ranger-base-7hx87's close under ranger-base-kl9ui.
//
// THE DEFECT. ranger-base-7hx87 was an arm of `scripts/test-times.sh
// --self-test` reporting a line absent when what had failed was the matcher,
// and 0b5c1c4 fixed it by taking the fork out of the two `printf | grep -q`
// conditions. One line below them, the cross-check arm still reads
//
//	got_mnt=$(printf '%s' "$out" | sed -n 's/.*DISK: [0-9]* MB free on \(.*\) — .*/\1/p' | head -1)
//
// (scripts/test-times.sh:831). A `sed` that is signalled, or that cannot be
// forked under load — the condition the 2026-09-02 sighting happened in —
// yields an empty $got_mnt, and the arm then reports the DISK line as naming
// nothing. MEASURED 2026-09-05 at 51b1195, two consecutive lines of ONE run
// with a `sed` on PATH whose whole body is `kill -TERM $$`:
//
//	ok    disk: the preflight line names free MB, the filesystem and what fills it
//	FAIL  disk: the line names ''; df says $TMPDIR is on '/System/Volumes/Data'
//
// The fixed arm and the arm below it disagree about the same $out, and the
// second one is wrong. That contradiction is what this pin holds, because it
// needs no tolerance and no fixture: one run, two arms, and only the
// apparatus between them.
//
// It matters here rather than in some cold corner because `make test` runs
// verify-test-times FIRST, so this reds a suite before a single package runs,
// with a message about a DISK line — the least likely thing a reader connects
// to their diff. That is the whole reason ranger-base-7hx87 was a P2.
//
// PARKED, not failing: the fix is the devops lane's (ranger-base-t07yx), and
// a red suite is not how a verifier hands work over. Un-skip it with the fix.
// Verified before parking, both arms on this tree: un-skipped it FAILS on the
// contradiction alone, and it PASSES against a tree whose extraction is done
// with bash's own `${...}` instead of the pipeline.
//
// The control arm below is not decoration. A probe that sabotages `sed` could
// pass by breaking the run so thoroughly that neither disk arm reports at all,
// so the same self-test has to be shown green and both arms ok with the real
// `sed` on PATH before the sabotaged run is allowed to mean anything.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// The two arm texts this pin rests on, spelled as the script prints them.
	sfPreflightOK = "ok    disk: the preflight line names free MB"
	sfCrossOK     = "ok    disk: the line names the filesystem df attributes"
	sfCrossFAIL   = "FAIL  disk: the line names ''"
)

// sfSelfTest runs the self-test with dir prepended to PATH (empty for none)
// and answers its combined output. The exit status is deliberately not the
// test: a sabotaged run is expected to be non-zero, and what this pin reads
// is which arms reported what.
func sfSelfTest(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("bash", "scripts/test-times.sh", "--self-test")
	if dir != "" {
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	out, err := cmd.CombinedOutput()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		t.Fatalf("bash scripts/test-times.sh --self-test: %v\n%s", err, out)
	}
	return string(out)
}

// sfDyingBin writes a `name` on PATH whose whole body is a TERM to itself.
// A fork failure under load (rc 127) has the same shape and is silent but for
// a stderr line, so this stands in for the load condition without needing one.
func sfDyingBin(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/bash\nkill -TERM $$\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestQATheSelfTestsDiskArmsDoNotContradictEachOtherWhenAForkFails(t *testing.T) {
	t.Skip("ranger-base-t07yx: scripts/test-times.sh:831 extracts the mount point through a pipeline, so a signalled or un-exec'd sed makes the arm report the DISK line as naming nothing")

	// CONTROL. With the real `sed` the self-test is green and both disk arms
	// report ok, so the sabotaged run below is measuring the apparatus and not
	// a script that was broken to begin with.
	clean := sfSelfTest(t, "")
	for _, want := range []string{sfPreflightOK, sfCrossOK} {
		if !strings.Contains(clean, want) {
			t.Fatalf("control: the unsabotaged self-test does not report %q, so this pin measures nothing:\n%s", want, clean)
		}
	}

	// And the run under a `sed` that dies of TERM. The DISK line is produced
	// by df and awk, so it is still there and still well formed — the arm
	// above says so in this very output.
	got := sfSelfTest(t, sfDyingBin(t, "sed"))
	if !strings.Contains(got, sfPreflightOK) {
		t.Fatalf("the preflight arm did not report ok under a sabotaged sed, so the contradiction this pin is about cannot arise here:\n%s", got)
	}
	if strings.Contains(got, sfCrossFAIL) {
		t.Errorf("one self-test run says the DISK line names free MB, the filesystem and what fills it, and then says it names '' — the second arm reported a failed fork as a missing filesystem (scripts/test-times.sh:831, ranger-base-t07yx). make test runs this gate before any package, so it reds a suite with a message about a disk:\n%s", got)
	}
}
