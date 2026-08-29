package posse

// QA pins for ranger-base-krra — the suite's OTHER environmental red.
//
// Claim: when this box runs out of disk, `make test` says so in words, at the
// two moments a reader is present.
//
// MEASURED 2026-08-29 on the machine that runs every session's suite: `make
// test` came back exit 2 with ~80 reds in internal/rhq, every one of them
//
//	--- FAIL: TestWatchPidRoundTrip (0.00s)
//	    testing.go:1426: TempDir: mkdir /var/folders/.../TestWatchPid...:
//	        no space left on device
//
// with 231Mi free, a 41G go build cache and 670 leaked Test* dirs going back
// two days. `t.TempDir()` calls `t.Fatal` on ENOSPC, so ONE full filesystem is
// reported ONCE PER TEST that wanted a temp dir. Through the house filter
// (`grep -E '^(---|ok|FAIL)'`) that is a list of unrelated test names —
// worktree, watch, dispatch, merge — and reads exactly like a broken change.
// The word `disk` appears nowhere a reader looks first. ranger-base-2ggb and
// ranger-base-7xla had already put a guard on the sibling red, the -timeout
// ceiling; this one had none.
//
// The guard lives in scripts/test-times.sh, which is the same place and the
// same shape as the clock's: a DISK line before the packages run, and a block
// after a run whose log carries ENOSPC. Neither deletes anything and neither
// goes red on free space — what to clear on a shared box is the operator's
// call (`go clean -cache` slows every concurrent session; deleting from
// $TMPDIR can take a live test's TempDir out from under it), and a floor is a
// claim about a box, which is the class of red this whole script exists to
// explain rather than to throw.
//
// TWO ARMS, because the guard can be lost two ways:
//
//  1. `make test` stops running the suite THROUGH the wrapper. Nothing else
//     pinned that. suitetimeout_qa_test.go pins the `-timeout` on the recipe
//     line and stays green if the wrapper is dropped from in front of it, at
//     which point every explainer in this file is unreachable and the suite
//     goes back to reporting the box as eighty broken tests.
//  2. the wrapper keeps its place and loses the guard. The script's own
//     `--self-test` covers the behaviour arm by arm, but only `make
//     verify-test-times` runs it — a plain `go test ./...` never does. This
//     arm runs it, so removing the guard reds the suite rather than reding
//     nothing.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The wrapper `make test` must route through, by path. Named rather than
// pattern-matched: this is one specific script carrying two specific
// explainers, and a different one would need its own pins.
const suiteWrapper = "scripts/test-times.sh"

// Arm 1: the recipe runs the suite through the wrapper. Without this the
// explainers below are dead code on every run anybody actually does.
func TestQAMakeTestRunsTheSuiteThroughTheDiskAndClockWrapper(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	recipe := makeRecipe(string(makefile), "test")
	if len(recipe) == 0 {
		t.Fatal("the Makefile has no `test` target — the suite command is the thing under test")
	}

	var line string
	for _, l := range recipe {
		if isComment(l) {
			continue
		}
		if goTestArgs(l) != nil {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("`make test`'s recipe invokes no `go test`:\n%s", strings.Join(recipe, "\n"))
	}

	fields := strings.Fields(line)
	if len(fields) == 0 || strings.Trim(fields[0], `"'`) != suiteWrapper {
		t.Errorf("`make test` runs `go test` directly instead of through %s:\n\t%s\n"+
			"the wrapper is what prints the DISK line before the packages and what explains an\n"+
			"ENOSPC log afterwards; without it a full disk is reported as ~80 unrelated test\n"+
			"failures naming worktree, watch and dispatch (ranger-base-krra), and a timeout is\n"+
			"reported as a goroutine dump (ranger-base-7xla)", suiteWrapper, strings.TrimSpace(line))
	}
}

// Arm 2: the wrapper still does it. Runs the script's own --self-test, which
// drives the real script end to end over fixtures — including the ENOSPC log
// measured on 2026-08-29 — and requires the disk arms by name, so deleting
// the guard and its arms together still reds this.
func TestQATheWrapperStillExplainsAFullDiskAndABrokenStdBuild(t *testing.T) {
	if _, err := os.Stat(suiteWrapper); err != nil {
		t.Fatalf("%s is gone: %v", suiteWrapper, err)
	}

	out, err := exec.Command("bash", suiteWrapper, "--self-test").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test failed: %v\n%s", suiteWrapper, err, out)
	}
	got := string(out)

	// The arm LABELS, not the guard's own strings: a self-test that stopped
	// exercising these would go green by deletion, and this is the deletion.
	for _, want := range []string{
		"disk: the preflight line names free MB",
		"disk floor: silent above the floor",
		"disk floor: warns below the floor",
		"enospc: block printed and ENOSPC failures counted",
		"enospc: exit status 1 preserved",
		"clean log: no ENOSPC block",
		"std break: block printed and the compiler line quoted",
	} {
		if !strings.Contains(got, "ok    "+want) {
			t.Errorf("%s --self-test no longer proves %q — the arm is gone, failing, or renamed\n%s", suiteWrapper, want, got)
		}
	}
}
