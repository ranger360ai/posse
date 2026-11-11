//go:build linux

package main

// WHAT EACH PIN PROTECTS (ADR 0006 §7), for the room check — the refusal
// `make coldexec-check`'s arms carry so that a box they do not fit in is a
// statement in milliseconds rather than a hundred unrelated red test names at
// the end of an hour (ranger-base-w5z7u, ADR 0085 D5).
//
//  1. THE REFUSAL IS THE DOOR'S OWN REFUSAL. Exit 2, nothing run. The arms
//     read exit 2 as "the door is not up" everywhere else, and a short room
//     that exited 1 would be read as the suite's own red.
//  2. AND IT SAYS THE NUMBER. A refusal that does not name what it read, and
//     what it wanted, sends the seat to `df` — where a quota looks fine
//     (ranger-base-akk9f).
//  3. THE CHECK IS OFF WHEN NOTHING ASKED FOR IT. The supervisor has callers
//     that are not the arms; a floor nobody set must not ground them.
//  4. IT READS THE DIRECTORY THE SUITE WILL WRITE INTO, which is $TMPDIR and
//     not /tmp. The arms nest their own directory under it, so a check that
//     read /tmp regardless would be measuring a filesystem the run never
//     touches.
//  5. THE BINDING NUMBER IS THE SMALLER OF THE TWO. Both a quota and a device
//     can be the short one, and the reason this check exists at all is that
//     on this box the quota is short while the device is not.
//  6. AN UNMEASURED QUOTA IS NOT A SPENT ONE. quotaUnmeasured is -1 and not
//     0: a filesystem with no quota on it — most of them, darwin included —
//     must read as "the device is the only limit" and never as "your share is
//     gone", which would ground the door everywhere it is not needed.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// runRoom runs the door over a command that leaves a witness file behind, so
// a test can tell "refused" from "ran and failed". It does not go through
// runDoor: that helper requires the report line every run that got as far as
// the filter prints, and a refusal prints no report at all — which is the
// thing being pinned.
func runRoom(t *testing.T, env []string, args ...string) (code int, out string, ran bool) {
	t.Helper()
	dir := t.TempDir()
	witness := filepath.Join(dir, "ran")
	argv := append([]string{}, args...)
	argv = append(argv, "--", "/bin/sh", "-c", "touch "+witness)

	cmd := exec.Command(builtDoor(t), argv...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "POSSE_COLD_EXEC_DELAY="), env...)
	b, err := cmd.CombinedOutput()

	var ee *exec.ExitError
	switch {
	case err == nil:
	case asExit(err, &ee):
		code = ee.ExitCode()
	default:
		t.Fatalf("running the door: %v\n%s", err, b)
	}
	_, statErr := os.Stat(witness)
	return code, string(b), statErr == nil
}

// TestAShortRoomIsARefusalAndNothingRuns is pins 1 and 2.
func TestAShortRoomIsARefusalAndNothingRuns(t *testing.T) {
	t.Parallel()

	// A floor no filesystem on any box meets, so this pin is about the
	// refusal and never about the box it runs on.
	code, out, ran := runRoom(t, nil, "-min-temp-mb", "999999999")

	if code != exitRefused {
		t.Errorf("a short room exited %d, want %d — the arms read %d as `the door is not up`, and anything else as the suite's own verdict:\n%s", code, exitRefused, exitRefused, out)
	}
	if ran {
		t.Errorf("the command RAN under a refused door. A refusal that starts the suite anyway is worse than no check: the tests die on their writes and the log reads like a broken change (ranger-base-w5z7u):\n%s", out)
	}
	for _, want := range []string{
		"TEMP ROOM:",  // what it read
		"999999999",   // what it wanted
		"NOTHING RAN", // and that the suite did not start
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal does not say %q:\n%s\nA refusal that does not name the number it read and the number it wanted sends the reader to `df`, which cannot see a quota at all (ranger-base-akk9f).", want, out)
		}
	}
}

// TestNoFloorAsksForNoRoomCheck is pin 3.
func TestNoFloorAsksForNoRoomCheck(t *testing.T) {
	t.Parallel()

	code, out, ran := runRoom(t, nil)

	if !ran {
		t.Errorf("the command did not run with no -min-temp-mb given:\n%s", out)
	}
	if strings.Contains(out, "TEMP ROOM") {
		t.Errorf("the door read the room with no floor asked for:\n%s\nThe supervisor has callers that are not `make %s`'s arms, and a reading none of them asked for is noise in their output.", out, "coldexec-check")
	}
	// Exit 3 is the door's own "delayed nothing", which this command does:
	// it plants no executable. That is the run reaching the end, which is
	// all this pin is about.
	if code != exitNoneDelayed {
		t.Errorf("exit %d, want %d (the run delayed nothing, which is what a `touch` does):\n%s", code, exitNoneDelayed, out)
	}
}

// TestTheRoomIsReadWhereTheSuiteWillWrite is pin 4.
func TestTheRoomIsReadWhereTheSuiteWillWrite(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	// A floor of 1 MB: the reading is the subject, not the verdict, and any
	// directory a test can write into has that much.
	_, out, ran := runRoom(t, []string{"TMPDIR=" + tmp}, "-min-temp-mb", "1")

	if !ran {
		t.Fatalf("the command did not run with a 1 MB floor:\n%s", out)
	}
	if !strings.Contains(out, "TEMP ROOM: "+tmp+" ") {
		t.Errorf("the room line does not name $TMPDIR (%s):\n%s\nEvery t.TempDir() and every `go build` work dir of a run lands there — cmd/go hands the test binary GOTMPDIR, which defaults to TMPDIR (ranger-base-krra) — so a check that read /tmp regardless would be measuring a filesystem the run never touches.", tmp, out)
	}
}

// TestTheBindingNumberIsTheSmallerOfTheTwo is pin 5, and
// TestAnUnmeasuredQuotaIsNotASpentOne below is pin 6. Both over the reading
// itself: the two numbers a box happens to have are not something a pin may
// arrange, and the decision between them is.
func TestTheBindingNumberIsTheSmallerOfTheTwo(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		free  int64
		quota int64
		want  int64
	}{
		{"the quota is the short one — this box", 5026, 2242, 2242},
		{"the device is the short one", 300, 2242, 300},
		{"no quota on the filesystem", 910609, quotaUnmeasured, 910609},
		{"a spent quota is zero and not unmeasured", 5026, 0, 0},
		{"equal", 1000, 1000, 1000},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := roomReading{Dir: "/x", FreeMB: c.free, QuotaMB: c.quota}
			if got := r.BindingMB(); got != c.want {
				t.Errorf("roomReading{free:%d quota:%d}.BindingMB() = %d, want %d", c.free, c.quota, got, c.want)
			}
		})
	}
}

// TestAnUnmeasuredQuotaIsNotASpentOne is pin 6.
func TestAnUnmeasuredQuotaIsNotASpentOne(t *testing.T) {
	t.Parallel()

	if quotaUnmeasured >= 0 {
		t.Fatalf("quotaUnmeasured is %d. It has to be negative: BindingMB compares it against the device, so an unmeasured quota of 0 would read as a SPENT one and refuse on every filesystem that has no quota — which is most of them, darwin included.", quotaUnmeasured)
	}

	// And the reading itself, on a real directory: a room that cannot be
	// read at all reports no free space, which refuses — the door's bias
	// everywhere else — so this asserts the opposite case, a directory that
	// exists and is writable.
	r := readRoom(t.TempDir())
	if r.FreeMB <= 0 {
		t.Errorf("readRoom(%s) reports %d MB free on the device, and a test that is running has somewhere to write:\n  %s", r.Dir, r.FreeMB, r.Line())
	}
	if r.QuotaBy == "" {
		t.Errorf("the reading says nothing about HOW the quota was read or why it was not:\n  %s\nThat clause is what tells a seat whether the number in front of it is the one that binds.", r.Line())
	}
	if !strings.Contains(r.Line(), strconv.FormatInt(r.BindingMB(), 10)) {
		t.Errorf("the room line does not carry the binding number %d:\n  %s", r.BindingMB(), r.Line())
	}
}
