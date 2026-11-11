package main

// The door's own pins (ADR 0085 D2, ranger-base-97w45). Every one of them
// RUNS the supervisor over a child and reads what the child's execs cost,
// because that is the only thing the door claims: a test whose verdict
// changes when the first exec of a planted file is slow will change its
// verdict under this, and no reading of either source decides it.
//
// WHAT EACH PIN PROTECTS (ADR 0006 §7). The process is `make coldexec-check`
// — a seat typing it over tests it just wrote, and ranger-base-xpxo6's full
// run over all three arms. What breaks without these is not a compile
// error: it is a door that runs, prints a count and covers less than it
// says. The cases below are each one row of ADR 0085's MEASURED list, which
// is what the design was accepted on, and each one of them has a way to
// fail silently — a plant resolved against the SUPERVISOR's cwd instead of
// the caller's reads as absent (the relative-path pin), a `(dev, ino)` key
// without the ctime stops a rewritten file from being cold again (the
// rewrite pin), an exec counted per-exec instead of per-inode charges D
// every time (the second-exec half of the first pin), and a door that
// delays nothing at all still exits 0 (the two pins that assert exit 3).
//
// The timing is a floor and a ceiling on either side of one partition, not
// a measurement: D is 600ms and the partition is at 400ms, so a cold exec
// has to come in over 400ms and a warm one under it. Both bounds are far
// from what either case actually costs here — a warm exec of these fixtures
// is 1-11ms, MEASURED — and the arithmetic that could regress (which exec
// paid, and how many) is read off the supervisor's own report instead,
// which is exact and has no load sensitivity at all. This box reaches a
// load average of 50-100 (NOTES.md), and a pin that read the ratio of two
// wall times would be a flake there.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// pinDelay is the D these pins inject. Not the door's own 2.5s: that
	// number is darwin's worst case, and paying it in a unit test would
	// cost the suite ten seconds to assert something a tenth of it decides.
	pinDelay = 600 * time.Millisecond
	// pinPartition is the line between "paid D" and "did not". See the
	// header: it is nowhere near either side's real cost.
	pinPartition = 400 * time.Millisecond
)

var (
	buildOnce sync.Once
	doorBin   string
	buildErr  error
	buildOut  string
)

// builtDoor builds cmd/coldexec once per test binary and answers the path. `go
// test` runs a test binary with its own package directory as the working
// directory, so "." is this command (the shape cmd/testparallel's own
// self-test uses).
func builtDoor(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "coldexec-pins")
		if err != nil {
			buildErr = err
			return
		}
		doorBin = filepath.Join(dir, "coldexec")
		out, err := exec.Command("go", "build", "-o", doorBin, ".").CombinedOutput()
		buildErr, buildOut = err, string(out)
	})
	if buildErr != nil {
		t.Fatalf("building the door under test: %v\n%s", buildErr, buildOut)
	}
	return doorBin
}

// doorRun is one supervised run: what it printed, what it exited with, how
// many cold execs it says it delayed, and how long each timed step took.
type doorRun struct {
	code    int
	out     string
	delayed int
	elapsed map[string]time.Duration
}

var (
	reDelayed = regexp.MustCompile(`(\d+) cold first execs delayed`)
	reElapsed = regexp.MustCompile(`ELAPSED (\S+) (\d+)ms`)
	// ARM 14's two: the supervisor's own pid, as the supervised shell's
	// ppid, and the ppid an orphaned descendant lands on.
	reSupervisor = regexp.MustCompile(`SUPERVISOR (\d+)`)
	reOrphanPPID = regexp.MustCompile(`ORPHAN PPID (\d+)`)
)

// runDoor runs the supervisor over `/bin/sh -c <script>` in a fresh
// directory, with `tm <label> <argv…>` available to the script for timing a
// step. The script's own plants are made INSIDE the run, which is the whole
// point: the door's epoch is its own start, so a fixture written before it
// starts is the checkout as far as it is concerned.
func runDoor(t *testing.T, script string, args ...string) doorRun {
	t.Helper()
	const preamble = `tm() { l=$1; shift; s=$(date +%s%N); "$@"; e=$(date +%s%N); echo "ELAPSED $l $(( (e-s)/1000000 ))ms"; }
`
	argv := append([]string{"-delay", pinDelay.String(), "-verbose"}, args...)
	argv = append(argv, "--", "/bin/sh", "-c", preamble+script)
	cmd := exec.Command(builtDoor(t), argv...)
	cmd.Dir = t.TempDir()
	// A scrubbed delay, so a seat running these under `make coldexec-check`
	// — or anything else that exports it — still measures pinDelay.
	cmd.Env = append(os.Environ(), "POSSE_COLD_EXEC_DELAY=")
	out, err := cmd.CombinedOutput()

	r := doorRun{out: string(out), elapsed: map[string]time.Duration{}}
	var ee *exec.ExitError
	switch {
	case err == nil:
	case asExit(err, &ee):
		r.code = ee.ExitCode()
	default:
		t.Fatalf("running the door: %v\n%s", err, out)
	}
	if m := reDelayed.FindStringSubmatch(r.out); m == nil {
		t.Fatalf("the run printed no report line, so it accounted for nothing:\n%s", r.out)
	} else {
		r.delayed, _ = strconv.Atoi(m[1])
	}
	for _, m := range reElapsed.FindAllStringSubmatch(r.out, -1) {
		ms, _ := strconv.Atoi(m[2])
		r.elapsed[m[1]] = time.Duration(ms) * time.Millisecond
	}
	return r
}

func asExit(err error, out **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*out = ee
	}
	return ok
}

// cold asserts that a timed step paid D.
func (r doorRun) cold(t *testing.T, label string) {
	t.Helper()
	d, ok := r.elapsed[label]
	if !ok {
		t.Fatalf("the run timed no step called %q, so it measured nothing:\n%s", label, r.out)
	}
	if d < pinPartition {
		t.Errorf("%s took %v, want at least %v — the door did not hold its first exec:\n%s", label, d, pinPartition, r.out)
	}
}

// warm asserts that a timed step did not.
func (r doorRun) warm(t *testing.T, label string) {
	t.Helper()
	d, ok := r.elapsed[label]
	if !ok {
		t.Fatalf("the run timed no step called %q, so it measured nothing:\n%s", label, r.out)
	}
	if d >= pinPartition {
		t.Errorf("%s took %v, want under %v — it was held and should not have been:\n%s", label, d, pinPartition, r.out)
	}
}

func (r doorRun) wantDelayed(t *testing.T, n int) {
	t.Helper()
	if r.delayed != n {
		t.Errorf("the door reports %d cold execs delayed, want %d:\n%s", r.delayed, n, r.out)
	}
}

func (r doorRun) wantCode(t *testing.T, n int) {
	t.Helper()
	if r.code != n {
		t.Errorf("the door exited %d, want %d:\n%s", r.code, n, r.out)
	}
}

// wantNoHoles asserts the run adjudicated every notification it took. A
// hole is an execve the door let through without knowing what it was, and
// a door accumulating them is covering less than its count says.
func (r doorRun) wantNoHoles(t *testing.T) {
	t.Helper()
	if strings.Contains(r.out, "un-adjudicated") {
		t.Errorf("the run left execs un-adjudicated:\n%s", r.out)
	}
}

// ─── the pins ────────────────────────────────────────────────────────────────

// ARM 1, and it is the whole class in one row: a test plants an executable
// and execs it under a deadline. The first exec pays D and the second does
// not, which is darwin's own shape (156-714ms mean cold, 6.3-10.9ms warm,
// MEASURED on 25.6.0). Charging every exec instead of the first would make
// every warmed helper in internal/posse — hangingBin, hangingGit, the
// `--warm` pre-flights — pay D at the moment they are relied on NOT to,
// and the door would red the tests it exists to clear.
func TestFirstExecOfAPlantedScriptPaysDAndTheSecondDoesNot(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `p=$PWD/plant.sh
printf '#!/bin/sh\necho ran\n' > "$p"; chmod +x "$p"
tm first "$p"
tm second "$p"
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 1)
	r.cold(t, "first")
	r.warm(t, "second")
	r.wantNoHoles(t)
	if n := strings.Count(r.out, "ran"); n != 2 {
		t.Errorf("the plant ran %d times, want 2 — the door is meant to delay an exec, not replace it:\n%s", n, r.out)
	}
}

// ARM 2 is the gap the old door could not close, and it is why the
// mechanism moved (ADR 0085 D5, amended under ranger-base-dacil): a plant
// with no `#!` line could not have a shim written into it, and 840 of 1711
// plant events — 49% — were exactly that. A copied binary is an inode like
// any other to this door. If this pin goes, the door is back to covering
// about half of what it claims, and nothing in the output would say so.
func TestACopiedBinaryPaysD(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `cp /bin/true copy
tm copied ./copy
tm again ./copy
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 1)
	r.cold(t, "copied")
	r.warm(t, "again")
	r.wantNoHoles(t)
}

// ARM 3: `sh <path>` execs /bin/sh and never the plant, so it is never
// delayed. This is ADR 0085 D1's claim that the adjudications fall out of
// EXECUTION rather than out of reading — ranger-base-aru4c's census could
// not decide this row by looking at the source, and watchpid_test.go's
// `liveProcess` is the live fixture that depends on the answer. A door that
// delayed on the WRITE instead of the exec would hold this one and red a
// test whose code is fine.
func TestShRunningThePlantIsNotDelayed(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `printf '#!/bin/sh\necho ran\n' > plant.sh; chmod +x plant.sh
tm viash /bin/sh plant.sh
`)
	// Nothing was delayed, so the door says so and exits 3 — which is the
	// other half of this pin: ADR 0085 D5's "zero delayed is a failed door
	// and not a clean suite" has to be visible in the exit code, or a `make
	// coldexec-check` whose injection broke reads as a green suite.
	r.wantCode(t, 3)
	r.wantDelayed(t, 0)
	r.warm(t, "viash")
	r.wantNoHoles(t)
}

// ARM 4: a git hook. 23 of the 26 tests the content-rewrite door redded
// were certifying a planted hook against posse's own render, so hooks are
// the fixture class that forced this mechanism — and they are also the one
// a test never execs itself: git does, from inside a `git` the test
// started. `git hook run` is used rather than a commit because the pin is
// about the exec and not about git's index. The filter is inherited by
// every descendant, which is what this pin reads.
func TestAHookGitExecsPaysD(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `git init -q repo 2>/dev/null
cd repo
printf '#!/bin/sh\necho hook ran\n' > .git/hooks/pre-commit; chmod +x .git/hooks/pre-commit
tm hook git hook run pre-commit
`)
	if strings.Contains(r.out, "is not a git command") {
		t.Skipf("this git has no `hook run` subcommand (2.36+), so the pin cannot reach a hook exec:\n%s", r.out)
	}
	r.wantCode(t, 0)
	r.wantDelayed(t, 1)
	r.cold(t, "hook")
	if !strings.Contains(r.out, "hook ran") {
		t.Errorf("git never ran the hook, so the delay above was somebody else's:\n%s", r.out)
	}
	if !regexp.MustCompile(`delayed\s+pid \d+ .*pre-commit`).MatchString(r.out) {
		t.Errorf("the delayed exec was not the hook — the door delayed something else in this run:\n%s", r.out)
	}
}

// ARM 5: a RELATIVE path, which is resolved against the caller's cwd and
// not the supervisor's. The supervisor runs from wherever `make` was typed
// and the test runs in its own t.TempDir, so a door that resolved `./x`
// against its own cwd would find nothing, call every relative exec absent,
// and delay none of them — while still printing a report and exiting 0 on
// any run that delayed one plant by an absolute path. Nothing but this pin
// separates those two.
func TestARelativePlantPaysD(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `mkdir -p sub
printf '#!/bin/sh\necho ran\n' > sub/x; chmod +x sub/x
cd sub
tm relative ./x
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 1)
	r.cold(t, "relative")
	r.wantNoHoles(t)
}

// ARM 6: a plant rewritten in place is cold AGAIN. The old door gave every
// write its own nonce, so re-planting a fixture made it cold; here it falls
// out of the ctime being part of the inode's identity. internal/posse has
// fixtures that plant, exec, re-plant and exec again inside one test — a
// hook re-rendered after an upgrade arm, most of them — and on darwin the
// second write is assessed again. A `(dev, ino)` key without the ctime
// would pass this run's second exec through warm, silently.
func TestAPlantRewrittenInPlaceIsColdAgain(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `printf '#!/bin/sh\necho one\n' > p; chmod +x p
tm first ./p
printf '#!/bin/sh\necho two\n' > p
tm rewritten ./p
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 2)
	r.cold(t, "first")
	r.cold(t, "rewritten")
	r.wantNoHoles(t)
	if !strings.Contains(r.out, "one") || !strings.Contains(r.out, "two") {
		t.Errorf("the two versions of the plant did not both run, so the second delay was not the rewrite's:\n%s", r.out)
	}
}

// ARM 7: an inode older than the door is not delayed — the toolchain, the
// checkout, /bin. This is the bound on the whole mechanism (ADR 0085 D5's
// first named edge) and it is what makes the door affordable: internal/posse
// takes tens of thousands of execs per arm and all but a hundred or so of
// them are the box's own binaries. A door that delayed on first-exec alone,
// without the epoch, would add D to every distinct binary the suite runs.
func TestAnOldBinaryIsNotDelayed(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `tm old /bin/true
tm oldagain /bin/true
`)
	r.wantCode(t, 3)
	r.wantDelayed(t, 0)
	r.warm(t, "old")
	r.warm(t, "oldagain")
	r.wantNoHoles(t)
}

// ARM 8: a door under a door REFUSES, and the command does not run. The
// kernel allows one user-notification listener per filter chain, so the
// inner seccomp is EBUSY — MEASURED, and ADR 0085 D5 names it as an edge
// that fails loudly rather than silently. The failure that matters is the
// quiet one: an inner supervisor that shrugged, ran the command anyway and
// printed a count would be a nested `make coldexec-check` reporting a clean
// suite over a run it never covered.
func TestANestedDoorRefusesRatherThanRunning(t *testing.T) {
	t.Parallel()
	bin := builtDoor(t)
	dir := t.TempDir()
	mark := filepath.Join(dir, "the-inner-command-ran")
	inner := fmt.Sprintf("%s -delay %s -- /usr/bin/touch %s", bin, pinDelay, mark)
	r := runDoor(t, fmt.Sprintf(`%s
echo "INNER EXIT $?"
`, inner))
	if !strings.Contains(r.out, "EBUSY") {
		t.Errorf("the nested door did not name EBUSY, so a seat meeting this has nothing to read:\n%s", r.out)
	}
	if !strings.Contains(r.out, "INNER EXIT 2") {
		t.Errorf("the nested door did not exit 2 (the refusal code):\n%s", r.out)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Errorf("the nested door ran its command anyway — a run nothing was covering:\n%s", r.out)
	}
}

// ARM 9: the supervised command's own exit code is what `make` sees. All
// three coldexec-check lines are a `go test`, and a suite that fails under
// the door has to fail the make line with ITS code and not with one of the
// door's two — a door that returned 1 for everything would make a failed
// suite and a broken door the same event.
func TestTheCommandsOwnExitCodeIsForwarded(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `printf '#!/bin/sh\nexit 7\n' > p; chmod +x p
tm failing ./p
exit 7
`)
	r.wantCode(t, 7)
	r.wantDelayed(t, 1)
	r.cold(t, "failing")
}

// ARM 10: the door refuses with no argv rather than exiting 0 over nothing.
// `make coldexec-check` builds its command line from three Makefile
// variables, and an empty one would otherwise be a line that passes.
func TestNoCommandRefuses(t *testing.T) {
	t.Parallel()
	out, err := exec.Command(builtDoor(t), "-delay", pinDelay.String()).CombinedOutput()
	var ee *exec.ExitError
	if !asExit(err, &ee) || ee.ExitCode() != 2 {
		t.Fatalf("coldexec with no argv exited %v, want 2:\n%s", err, out)
	}
	if !strings.Contains(string(out), "usage: coldexec") {
		t.Errorf("the refusal did not print the usage:\n%s", out)
	}
}

// ARM 11: POSSE_COLD_EXEC_DELAY reaches the SUPERVISOR and nothing under
// it, and both halves are asserted here because each one is load-bearing on
// its own.
//
// The variable is the Makefile's switch, and until ranger-base-7zkxw it is
// ALSO what makes internal/posse's TestMain install the content-rewrite
// door this supervisor replaces. A run where the child saw it too would
// have both doors over one suite — the shim rewriting the bytes of every
// plant and the supervisor delaying the shim's own exec — which is exactly
// the 26 red rows ADR 0085's amendment exists to clear, reappearing with no
// line in the output to say why. The other half matters as much: a
// supervisor that stopped reading it would silently fall back to 2.5s and
// make `COLDEXEC_DELAY` in the Makefile a variable that changes nothing.
func TestTheDelayVariableReachesTheSupervisorAndNothingUnderIt(t *testing.T) {
	t.Parallel()

	// No -delay flag, so D can only come from the environment.
	cmd := exec.Command(builtDoor(t), "-verbose", "--", "/bin/sh", "-c",
		`printf 'CHILD SAW [%s]\n' "$POSSE_COLD_EXEC_DELAY"
printf '#!/bin/sh\nexit 0\n' > p; chmod +x p; ./p`)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "POSSE_COLD_EXEC_DELAY="+pinDelay.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the run failed (%v):\n%s", err, out)
	}
	if !strings.Contains(string(out), "CHILD SAW []") {
		t.Errorf("the supervised command saw POSSE_COLD_EXEC_DELAY — it reaches the door alone, "+
			"or internal/posse's TestMain installs the content-rewrite door underneath this one:\n%s", out)
	}
	if !strings.Contains(string(out), "delayed "+pinDelay.String()) {
		t.Errorf("the door did not take its D from the environment, so COLDEXEC_DELAY in the Makefile "+
			"changes nothing:\n%s", out)
	}
	if !regexp.MustCompile(`1 cold first execs delayed`).MatchString(string(out)) {
		t.Errorf("the run delayed something other than the one plant:\n%s", out)
	}
}

// ARM 12: the epoch test reads the CTIME, and a plant whose MTIME predates
// the run is still a plant. This is the witness for the choice door_linux.go
// documents at the stat — "the ctime, not the mtime: a copied binary keeps
// its source's mtime and a `chmod +x` moves only the ctime, and both of
// those are plants" — which until now nothing measured: mutating
// `st.Ctim` to `st.Mtim` in that one comparison left all eleven pins above
// green, ok 1.535s (MEASURED 2026-11-10, ranger-base-tfqlr finding 1), so a
// later "simplify ctime to mtime" landed green over a door that had stopped
// covering half of ADR 0085 D5's artifact class.
//
// Both halves of the comment are here because each one is a real spelling of
// that class and the mtime reading misses both. ARM 2 cannot see it: its
// plant is a plain `cp`, which sets a FRESH mtime, so it is cold under
// either reading. The mtime-preserving copy is what `cp -p`, `install -p`
// and a tar/zip extraction do, and the second half is a file that was
// already in the tree and gains its exec bit — a chmod moves the ctime and
// nothing else, so under the mutant door both of these read "old" and are
// let through at full speed. Under the shipped door both pay D, which is
// what this asserts.
func TestAPlantWhoseMtimePredatesTheRunStillPaysD(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `cp -p /bin/true preserved
tm preserved ./preserved
printf '#!/bin/sh\necho ran\n' > gained
touch -t 202001010000 gained
chmod +x gained
tm gained ./gained
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 2)
	// The copy keeps /bin/true's mtime; only its ctime is this run's.
	r.cold(t, "preserved")
	// `touch -t` puts the mtime in 2020 and the chmod moves the ctime alone.
	r.cold(t, "gained")
	r.wantNoHoles(t)
	if !strings.Contains(r.out, "ran") {
		t.Errorf("the backdated plant did not run, so the second delay was not its exec:\n%s", r.out)
	}
}

// ARM 13: the warm/cold DEDUP KEY reads the ctime too, and that is a second
// decision rather than the same one twice. ARM 12 above witnesses the EPOCH
// test at door_linux.go's stat; the line below it builds
// `inode{dev, ino, csec, cnsec}` from `st.Ctim` for a different purpose —
// telling a first exec of this inode-at-this-version from a repeat — and
// until now nothing measured THAT: mutating those two fields alone to
// `st.Mtim` left all twelve pins above green, ok 1.395s (MEASURED
// 2026-11-11, ranger-base-8do8p finding 2, re-measured here).
//
// It is not an equivalent mutant, because the key is only reached once the
// epoch test has passed: the two readings can part company solely on a
// SECOND exec of one inode whose ctime moved and whose mtime did not, which
// is a metadata-only change — the `chmod +x` half of the comment ARM 12
// quotes, happening to a file that has already been exec'd. Under the mtime
// key that re-exec reads WARM and is let through at full speed (MEASURED
// 2026-11-11, two doors built from the same tree: shipped "2 cold first
// execs delayed ... 0 warm", mutant "1 cold first execs delayed ... 1
// warm", the second step 1502ms against 3ms). That is a MISSED injection,
// so a test that would flake on ci.yml's macos-latest leg passes here,
// which is the one thing ADR 0085 D5 exists to prevent.
//
// No pin above can see it: every other plant either predates the door or
// moves mtime and ctime together — `cp`, `cp -p` onto a fresh inode, a
// rewrite in place (ARM 6 moves BOTH) — and ARM 12 holds the mtime back but
// execs each plant once.
func TestAReExecAfterAMetadataOnlyChangeIsColdAgain(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `printf '#!/bin/sh\necho ran\n' > p; chmod +x p
tm first ./p
chmod 0750 p
tm chmodded ./p
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 2)
	r.cold(t, "first")
	// The chmod moves the ctime and leaves the mtime where the printf put
	// it, so a key built from the mtime calls this exec warm.
	r.cold(t, "chmodded")
	r.wantNoHoles(t)
	if strings.Count(r.out, "ran") != 2 {
		t.Errorf("the plant did not run twice, so the second delay was not its re-exec:\n%s", r.out)
	}
}

// ARM 14: a notification the KERNEL takes back is not a coverage hole, and
// the door must not report it as one.
//
// This is ranger-base-j0riv. The verdict was called `gone` and it was in
// `holes`, so `wantNoHoles` in ARM 3 above redded intermittently — only
// inside `go test ./...`, where cmd/coldexec shares the box with its
// sibling packages, and more often the busier the box (MEASURED on the
// bead: FAIL at a one-minute loadavg of 12.95, ok three times at 7.32 and
// below). arm 1 is `make test`'s first test line and nobody's optional
// door, so the cost of that misclassification was a red package and a
// non-zero exit for a door whose injection was never in question.
//
// The mechanism is in the note above `holes` in door_linux.go. What this
// pin needs from it is one line: seccomp's wait is interruptible, so a
// deliverable signal makes the kernel REMOVE the notification and return
// ERESTARTSYS — and a caller whose handler carries SA_RESTART then RESTARTS
// the execve, which raises a fresh notification that the door adjudicates
// normally. The kernel never completes an execve whose notification it took
// back, so nothing escapes the door; one execve just spends two
// notifications.
//
// THE FIXTURE, and every part of it is load-bearing:
//
//   - python3 is the caller because `signal.siginterrupt(sig, False)` is
//     the shortest way to install an SA_RESTART handler, which is what the
//     Go runtime installs for its own preemption SIGURG — and the
//     supervisor's own Go child is the process that produces these in the
//     wild. A `sh` caller cannot stand in: it resets handled dispositions
//     before `exec`, so the signal is fatal instead (MEASURED: exit 138),
//     which is a different ending of the same mechanism.
//   - the nudging is a FORKED child of the caller rather than a sleep in
//     the script, and it is guarded by the caller's own
//     /proc/<pid>/exe. The window to hit is the door's D, and this box
//     reaches a loadavg of 50-100 (NOTES.md), so a timed sleep would be
//     the flake this file's header warns about: the loop keeps nudging
//     until the exec lands, which is deterministic in both directions —
//     a signal that arrives too early is simply handled and retried, and
//     the instant /proc/<pid>/exe changes the plant is running under that
//     pid and must not be signalled. 40/40 identical on a quiet box,
//     MEASURED 2026-11-11.
//   - the plant traps USR1 to nothing, which covers the microseconds
//     between the exec landing and the nudger's next readlink.
//
// WHAT IT DOES NOT ASSERT, on purpose: the wall time of the delayed exec.
// The withdrawal releases the caller at the nudge rather than at D, so this
// run's injection is TRUNCATED — the plant's exec goes through in
// milliseconds although the door charged it D and its handler sleeps the
// full D. That is a second defect and it is filed, not fixed here
// (ranger-base-4kars); pinning either wall time would make this pin red
// when that one lands. What is asserted is the
// classification and the coverage: no holes, the withdrawal named in the
// report, and the withdrawn execve adjudicated a second time and run.
func TestAWithdrawnNotificationIsNotAHole(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("no python3, so the pin cannot install an SA_RESTART handler in a caller: %v", err)
	}
	r := runDoor(t, `cat > caller.py <<'PY'
import os, signal, sys, time

# A handler WITH SA_RESTART, which is the Go runtime's own shape. Installed
# before the fork, so the nudger below can never catch this process with
# SIGUSR1 still fatal.
signal.signal(signal.SIGUSR1, lambda *a: None)
signal.siginterrupt(signal.SIGUSR1, False)

me = os.getpid()
mine = os.readlink("/proc/%d/exe" % me)
if os.fork() == 0:
    # Nudge the parent until its execve has landed. The guard is the
    # parent's OWN exe link: while it still reads as this python the execve
    # has not completed, so a signal can only land on the stopped syscall.
    for _ in range(500):
        try:
            if os.readlink("/proc/%d/exe" % me) != mine:
                break
            os.kill(me, signal.SIGUSR1)
        except OSError:
            break
        time.sleep(0.01)
    os._exit(0)

os.execv(sys.argv[1], [sys.argv[1]])
PY
printf '#!/bin/sh\ntrap "" USR1\necho the plant ran\n' > plant; chmod +x plant
python3 -I caller.py ./plant
echo "CALLER EXIT $?"
`)
	// Something was delayed, so the door does not exit 3 over this run.
	r.wantCode(t, 0)

	// THE ASSERTION THIS PIN EXISTS FOR.
	r.wantNoHoles(t)

	// And the withdrawal is not silent either: a seat whose run spent two
	// notifications on one execve has a line that says so.
	if !strings.Contains(r.out, "withdrawn   pid") {
		t.Fatalf("the nudging produced no withdrawn notification, so this run did not reach the "+
			"mechanism and its wantNoHoles above proved nothing:\n%s", r.out)
	}
	if !strings.Contains(r.out, "notifications were withdrawn before the door could answer") {
		t.Errorf("the report did not account for the withdrawal, so a seat reading it has nothing "+
			"to read:\n%s", r.out)
	}

	// Coverage: the withdrawn execve was adjudicated a SECOND time — the
	// restart's own notification — rather than let through blind. Either
	// verdict counts: it is `warm` today and would be `delayed` if the
	// truncation above is ever fixed.
	adjudications := regexp.MustCompile(`(?m)^coldexec: (delayed|warm) +pid \d+ .*plant$`).FindAllString(r.out, -1)
	if len(adjudications) < 2 {
		t.Errorf("the plant's exec was adjudicated %d times, want at least 2 — the withdrawn "+
			"notification's restart went through without one:\n%s", len(adjudications), r.out)
	}

	// And it ran, under that pid, so the second adjudication was a real
	// exec and not a notification for something that never happened.
	if !strings.Contains(r.out, "the plant ran") {
		t.Errorf("the plant never ran, so the execve the door withdrew a notification for did not "+
			"complete:\n%s", r.out)
	}
	if !strings.Contains(r.out, "CALLER EXIT 0") {
		t.Errorf("the caller did not exit 0 — the signal ended it instead of restarting its "+
			"execve, which is a different ending of the mechanism:\n%s", r.out)
	}
}

// ARM 15: an ORPHANED descendant's plant pays D too, and the supervisor
// still leaves no zombie behind. Both halves, because the fix has two
// moving parts and each one on its own is a regression.
//
// THE HOLE. The door's ptrace reach is ancestry and nothing else: readPath
// opens /proc/<pid>/mem, which needs PTRACE_MODE_ATTACH, which on a box
// with yama/ptrace_scope=1 (this one) is granted to an ancestor and to
// nobody else. A descendant whose own parent exited is reparented past the
// supervisor — to pid 1, or on a systemd user session to `systemd --user`,
// MEASURED here as ppid 1140 — and falls out of that reach. Its execs then
// read `unreadable`, so a plant it execs is NEVER DELAYED and the test that
// planted it is not covered at all, silently as far as that test can tell.
// MEASURED 2026-11-11 (ranger-base-orgvt, Linux 7.2.15): 134 `permission
// denied` in the internal/treepins arm's 29744 notifications, clustered in
// the suite-lock self-tests, which orphan holder processes on purpose —
// against none at all in cmd/posse's 8427, whose tests do not orphan. (The
// bead reads 145 un-adjudicated; the other 11 were `gone`, retired as a
// verdict by ARM 14's bead and not this edge.) On the repro below the
// pre-fix door delays the kept child's plant and passes the orphan's
// through at 2-3ms, with no line anywhere saying a plant went by.
//
// THE KEPT HALF IS NOT DECORATION. It is what tells this pin's red apart
// from a door that has stopped delaying anything at all: a run where
// neither child pays D is ARM 1's failure, and a run where only the orphan
// does not is this one.
//
// THE ZOMBIE HALF. PR_SET_CHILD_SUBREAPER is what closes the hole, and on
// its own it opens a visible one: being the subreaper makes the supervisor
// the parent of every orphan, so an orphan that exits unwaited-on stays a
// zombie — it answers `kill -0`, where under init it would have been reaped
// and gone. That is not cosmetic here. `kill -0` is this tree's own
// liveness predicate: codextrust_test.go reads ESRCH as "ended and reaped"
// and anything else as a leaked zombie. MEASURED 2026-11-11 with the prctl
// and no reaper, on this script: the orphan's plant is delayed 604ms AND
// its pid still answers `kill -0` ten seconds after it exited. So
// reapOrphans is pinned here beside the delay it pays for, and dropping
// either one reds this and nothing else.
//
// THE HANDSHAKE, because the orphan cannot time this for itself. It has no
// way to tell an initial ppid read taken BEFORE its parent exited from one
// taken after — reparenting is fast and usually wins — so a loop waiting
// for that ppid to change either breaks at once or never breaks, and the
// first draft of this pin spent 10s of every run in the second case. The
// main shell is the process that knows: the kernel reparents in
// `exit_notify`, before the exiting parent can be reaped, so by the time
// `wait` has returned the orphan is already somebody else's child. The
// `reparented` file is that `wait` returning, and the orphan reads its
// ppid once, after it.
//
// What the pin does NOT assert is the orphan's ppid being 1, because under
// the door it is the supervisor's — the mechanism, and doc.go's one new
// stated edge. It asserts the opposite, as the witness that the arm is set
// up: an orphan whose ppid is the supervisor really was reparented away
// from the shell that forked it and really did land inside the reach.
func TestAnOrphanedDescendantsPlantPaysDAndLeavesNoZombie(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `read _p _c _s SUP _rest < /proc/$$/stat
echo "SUPERVISOR $SUP"
printf '#!/bin/sh\necho kept ran\n' > kept.sh; chmod +x kept.sh
printf '#!/bin/sh\necho orphan ran\n' > orph.sh; chmod +x orph.sh
tm kept ./kept.sh
( /bin/sh -c 'echo $$ > orphan.pid
i=0; while [ ! -f reparented ] && [ $i -lt 400 ]; do sleep 0.05; i=$((i+1)); done
read _p _c _s pp _r < /proc/$$/stat
echo "ORPHAN PPID $pp"
s=$(date +%s%N); ./orph.sh; e=$(date +%s%N); echo "ELAPSED orphan $(( (e-s)/1000000 ))ms"
: > orphan.done' & ) &
wait
: > reparented
i=0; while [ ! -f orphan.done ] && [ $i -lt 300 ]; do sleep 0.1; i=$((i+1)); done
p=$(cat orphan.pid)
i=0; while kill -0 "$p" 2>/dev/null && [ $i -lt 100 ]; do sleep 0.1; i=$((i+1)); done
kill -0 "$p" 2>/dev/null && echo "ORPHAN $p STILL ANSWERS kill -0" || echo "ORPHAN $p IS GONE"
`)
	r.wantCode(t, 0)

	// The arm is set up: the orphan outlived the shell that forked it and
	// was reparented onto the supervisor, which is the whole mechanism. A
	// run where this does not hold measures nothing below it.
	sup := reSupervisor.FindStringSubmatch(r.out)
	opp := reOrphanPPID.FindStringSubmatch(r.out)
	if sup == nil || opp == nil {
		t.Fatalf("the run did not print both the supervisor's pid and the orphan's ppid, so the arm is not set up:\n%s", r.out)
	}
	if sup[1] != opp[1] {
		t.Fatalf("the orphan's ppid is %s and the supervisor is %s — it was reparented somewhere else "+
			"(pid 1, or `systemd --user`), so PR_SET_CHILD_SUBREAPER is not in effect and the arm "+
			"measures the pre-fix door:\n%s", opp[1], sup[1], r.out)
	}

	// Both plants pay D, and the orphan's is the one this pin exists for.
	r.wantDelayed(t, 2)
	r.cold(t, "kept")
	r.cold(t, "orphan")
	r.wantNoHoles(t)
	if !strings.Contains(r.out, "kept ran") || !strings.Contains(r.out, "orphan ran") {
		t.Errorf("one of the two plants did not run, so one of the delays above was somebody else's:\n%s", r.out)
	}

	// And the supervisor waited on the orphan it adopted.
	if !strings.Contains(r.out, "IS GONE") {
		t.Errorf("the orphan's pid still answered `kill -0` after it exited — the supervisor adopted it "+
			"and never reaped it, so every orphan under the door is a zombie and `kill -0` stops "+
			"meaning what this tree reads it to mean:\n%s", r.out)
	}
	if !regexp.MustCompile(`\d+ orphaned descendants were reparented onto this process and reaped`).MatchString(r.out) {
		t.Errorf("the report does not account for the orphan it reaped, so a real arm has no witness that "+
			"the subreaper arm fired at all:\n%s", r.out)
	}
}
