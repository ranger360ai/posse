package treepins

// QA pins for ranger-base-ebrwn — the suite-lock self-test rig must decide its
// own environment, for every POSSE_* name the library reads.
//
// THE DEFECT. `scripts/suite-lock.sh`'s self-test forks real holder processes,
// which inherit the seat's environment. The rig set POSSE_SUITE_LOCK_DIR,
// POSSE_SUITE_LOCK_POLL and POSSE_SUITE_SLOTS and unset POSSE_SUITE_LOCK_HELD
// (with a comment saying why). It did not touch POSSE_SUITE_LOCK — the
// header's own documented opt-out, "run unserialized, and say so on one line",
// which a seat may export for a session, from a wrapper or from a shell
// profile. Every holder then took the opt-out, every acquire returned at once,
// and the arms read no lock where they hold one.
//
// MEASURED 2026-09-11, darwin/arm64, at f500084f:
//
//	$ POSSE_SUITE_LOCK=0 bash scripts/suite-lock.sh --self-test
//	FAIL  slots: two concurrent full suites both run: got slots 'none' and 'none'
//	FAIL  queue: a third full suite waits: it started anyway, on slot none
//	... 10 FAIL lines, then: suite-lock --self-test: FAILED
//
// `make test` gates on verify-suite-lock, so that reds the WHOLE suite before
// a single package builds — and it reds it with lock-slot messages, over a
// variable the reader set on purpose and was told they could.
//
// The direction matters and is why this needs a pin rather than a fix alone:
// an ambient value does not make an arm fail loudly. It makes the arm measure
// nothing — a lock that was never taken and a fork that was never scheduled
// print the same `slot:none` — and then report that reading as a verdict. The
// same class in the other direction (POSSE_SUITE_LOCK_HELD) had already been
// found once, and the rig's comment about it is what makes this one's absence
// legible.
//
// TWO ARMS, because the property has a general half and the general half is
// not executable:
//
//  1. by EXECUTION: the self-test passes with every one of those names set
//     hostilely at once. That is the bead's REPRO, widened — a rig that
//     decides its own environment does not care what it was handed.
//  2. by SOURCE: the set of POSSE_* names the LIBRARY reads is exactly the set
//     the rig neutralizes, and the set arm 1 poisons. A sixth variable added
//     to the library is the next instance of this defect, and no run on a
//     clean box can find it — nothing about a name nobody exported is visible
//     from the inside.

import (
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every POSSE_* name the library reads, and a value that is WRONG for the
// self-test — wrong in the direction that makes arms pass over measurements of
// nothing, not in the direction that makes them crash. Keyed by name so arm 2
// can require this map to be the census; add a name to the library and this
// map must gain a row, or both arms red.
var suiteLockHostileEnv = map[string]string{
	// The bead. Every acquire returns at once and unserialized.
	"POSSE_SUITE_LOCK": "0",
	// The sibling class, already fixed: every acquire declines to take a
	// second slot and reports the one it thinks it is inside.
	"POSSE_SUITE_LOCK_HELD": "9",
	// A queue four slots wider than the arms assert against: arms 1-3 would
	// then be reading a box that really can run seven suites at once.
	"POSSE_SUITE_SLOTS": "7",
	// Longer than any arm's backstop, so a queued holder never gets a second
	// attempt and "it waited" and "it never retried" become one reading.
	"POSSE_SUITE_LOCK_POLL": "97",
	// Not the scratch dir the rig makes, and not openable — the sandbox arm's
	// own condition, applied to every arm at once.
	"POSSE_SUITE_LOCK_DIR": "/nonexistent/posse-suite-lock-ranger-base-ebrwn",
}

// A read, not a mention: `${NAME` or `$NAME`. The library says these names in
// its own diagnostics too ("POSSE_SUITE_SLOTS to change, POSSE_SUITE_LOCK=0 to
// opt out"), and a message naming a variable is not a process inheriting one.
var slEnvRead = regexp.MustCompile(`\$\{?(POSSE_[A-Z0-9_]+)`)

// `export NAME=` / `unset NAME [NAME...]` at the head of a line, which is
// where the rig's preamble lives. Deliberately NOT matching mid-line: the arms
// below set these names per-command on purpose (arm 9 exports the opt-out,
// arms 12 and 15 a bad slot count), and a per-command value is the subject of
// an arm rather than the rig's decision about its own environment.
var (
	slRigExport = regexp.MustCompile(`^\s*export\s+(POSSE_[A-Z0-9_]+)=`)
	slRigUnset  = regexp.MustCompile(`^\s*unset\s+(.*)$`)
)

// slLibraryLines returns the script's lines with the self-test body removed —
// the library half, which is what a forked holder actually runs.
func slLibraryLines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(suiteLockScript)
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Split(string(b), "\n")
	body, off := slSelfTestBody(t)
	lib := append([]string{}, all[:off]...)
	return append(lib, all[off+len(body):]...)
}

func slNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Arm 1: the rig's verdict does not depend on the environment it was handed.
func TestQASuiteLockSelfTestIgnoresAnAmbientPosseEnvironment(t *testing.T) {
	if _, err := os.Stat(suiteLockScript); err != nil {
		t.Fatalf("%s is gone: %v", suiteLockScript, err)
	}
	cmd := exec.Command("bash", suiteLockScript, "--self-test")
	cmd.Env = os.Environ()
	for n, v := range suiteLockHostileEnv {
		cmd.Env = append(cmd.Env, n+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test fails when the seat has exported the names the library reads (%v).\n\n"+
			"Every holder the rig forks inherits this process's environment, so the rig\n"+
			"must set or unset each of them itself — POSSE_SUITE_LOCK is the documented\n"+
			"opt-out, and a seat that exports it reds all of `make test` at\n"+
			"verify-suite-lock, before a package builds (ranger-base-ebrwn).\n\nenv: %v\n\n%s",
			suiteLockScript, err, suiteLockHostileEnv, out)
	}
	// The positive witness. A `--self-test` that stopped running its arms
	// would exit 0 here just as happily; arm 3 of suitelock_qa_test.go holds
	// that it can fail at all, and this only refuses a run that decided
	// nothing.
	if !strings.Contains(string(out), "ok    opt-out: POSSE_SUITE_LOCK=0 runs unserialized and says so") {
		t.Fatalf("%s --self-test exited 0 under the hostile environment without running the opt-out arm — this arm is measuring nothing\n%s",
			suiteLockScript, out)
	}
}

// Arm 2: the census. Every POSSE_* name the library reads is one the rig
// decides, and one arm 1 poisons.
func TestQASuiteLockRigNeutralizesEveryPosseNameTheLibraryReads(t *testing.T) {
	read := map[string]bool{}
	for _, raw := range slLibraryLines(t) {
		if strings.HasPrefix(strings.TrimLeft(raw, " \t"), "#") {
			continue
		}
		for _, m := range slEnvRead.FindAllStringSubmatch(raw, -1) {
			read[m[1]] = true
		}
	}
	// A scan that went blind reports an empty census and passes. There were
	// five such names when this pin was written.
	if len(read) < 5 {
		t.Fatalf("the census found only %d POSSE_* names read by %s's library half (%v) — it was 5, so the scan has gone blind and a clean result here means nothing",
			len(read), suiteLockScript, slNames(read))
	}

	body, _ := slSelfTestBody(t)
	decided := map[string]bool{}
	for _, raw := range body {
		if strings.HasPrefix(strings.TrimLeft(raw, " \t"), "#") {
			continue
		}
		if m := slRigExport.FindStringSubmatch(raw); m != nil {
			decided[m[1]] = true
		}
		if m := slRigUnset.FindStringSubmatch(raw); m != nil {
			for _, f := range strings.Fields(m[1]) {
				if strings.HasPrefix(f, "POSSE_") {
					decided[f] = true
				}
			}
		}
	}

	var undecided, unpoisoned []string
	for _, n := range slNames(read) {
		if !decided[n] {
			undecided = append(undecided, n)
		}
		if _, ok := suiteLockHostileEnv[n]; !ok {
			unpoisoned = append(unpoisoned, n)
		}
	}
	if len(undecided) > 0 {
		t.Errorf("%s's self-test rig inherits %v from the seat.\n\n"+
			"The rig forks REAL holder processes, so every name its library half reads\n"+
			"must be set or unset in the rig's own preamble. An inherited value does not\n"+
			"make an arm fail loudly — it makes the arm measure nothing and then report\n"+
			"that reading as a verdict (ranger-base-ebrwn MEASURED POSSE_SUITE_LOCK=0\n"+
			"printing 10 FAIL lines, which reds all of `make test`).",
			suiteLockScript, undecided)
	}
	if len(unpoisoned) > 0 {
		t.Errorf("suiteLockHostileEnv has no wrong value for %v, so arm 1 runs with those names clean and proves nothing about them.\n\n"+
			"Add a row per name, with a value that is wrong in the direction that makes an\n"+
			"arm pass over a measurement of nothing.", unpoisoned)
	}

	// And the other direction: a name the rig decides that the library no
	// longer reads is a stale line, and a stale line is how the next reader
	// concludes the preamble is complete.
	var stale []string
	for _, n := range slNames(decided) {
		if !read[n] {
			stale = append(stale, n)
		}
	}
	if len(stale) > 0 {
		t.Errorf("%s's self-test rig sets or unsets %v, which its library half no longer reads — drop the line or the preamble stops being readable as the whole list",
			suiteLockScript, stale)
	}
}
