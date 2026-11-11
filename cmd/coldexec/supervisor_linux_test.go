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
