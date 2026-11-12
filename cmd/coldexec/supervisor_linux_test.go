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
// A PLANT'S OUTPUT MARKER IS SHOUTED, and that is not decoration
// (ranger-base-4kars). These pins read the plant's own stdout out of the
// SAME buffer that carries the door's -verbose lines, and those lines name
// the plant's absolute path. So a marker that can occur inside a path is an
// assertion that reads the path as output: `strings.Count(r.out, "ran")`
// counted 4 instead of 2 the first time these ran under
// TMPDIR=/var/tmp/posse-tmp-ranger-base-4kars, because "ranger" contains
// "ran" — and EVERY bead id on this box begins "ranger-", so any temp path
// named after the session trips it. ARM 12 had it the other way and was
// worse: its `Contains(r.out, "ran")` would have passed on the path alone,
// with the plant never running. Hence THE PLANT RAN, VERSION ONE and
// VERSION TWO: upper case with spaces, which no path here spells. The
// fixture-shaped reason this matters now is ranger-base-w5z7u, whose fix is
// to give the arms a TMPDIR of their own.
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

	"golang.org/x/sys/unix"
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
	doorDir   string
	doorBin   string
	buildErr  error
	buildOut  string
)

// doorDirPrefix names the one directory these pins build the door into, and
// it begins "posse-" for a reason TestMain below cannot cover
// (ranger-base-7hyn8). TestMain removes the dir on a normal exit; it does not
// run at all when this binary is SIGKILLed or dies on a -timeout panic, and
// this box kills suites (AGENTS.md, "Ending anything"). What collects THAT
// run's dir is scripts/reap-test-tmp.sh, whose posse-scratch class is the
// glob `posse-*` at a 120m grace — 2x the longest -timeout anything here
// carries — so a root inside posse's own temp namespace is collectable by
// construction and a root outside it is invisible to the reaper forever. The
// old name was `coldexec-pins`, matched by no class: MEASURED 2026-11-11 on
// this box, 278 of them on /tmp holding 914M, against a 14G tmpfs that had
// been at 100% of its 1,048,576 inodes that morning.
const doorDirPrefix = "posse-coldexec-pins-"

// TestMain removes the directory builtDoor builds into. That removal cannot
// be a t.Cleanup — the build is shared by every test in this binary and
// therefore outlives each of them, which is why there was no remover at all
// until ranger-base-7hyn8 — and it cannot be a defer, because os.Exit runs
// none.
//
// A removal that fails REDS the package rather than returning quietly. The
// house shape elsewhere (cmd/posse/configdirfence_test.go's TestMain) drops
// the error, and dropping it here is how this leak would come back silently:
// nothing else in the tree ever looks at this dir. The dir holds one file we
// built ourselves, under TMPDIR, with no other writer, so a failure here is
// a fact about the filesystem worth a name.
func TestMain(m *testing.M) {
	code := m.Run()
	if doorDir != "" {
		if err := os.RemoveAll(doorDir); err != nil {
			fmt.Fprintf(os.Stderr, "coldexec pins: removing the door build dir %s: %v\n", doorDir, err)
			if code == 0 {
				code = 1
			}
		}
	}
	os.Exit(code)
}

// builtDoor builds cmd/coldexec once per test binary and answers the path. `go
// test` runs a test binary with its own package directory as the working
// directory, so "." is this command (the shape cmd/testparallel's own
// self-test uses).
func builtDoor(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", doorDirPrefix)
		if err != nil {
			buildErr = err
			return
		}
		doorDir = dir
		doorBin = filepath.Join(dir, "coldexec")
		out, err := exec.Command("go", "build", "-o", doorBin, ".").CombinedOutput()
		buildErr, buildOut = err, string(out)
	})
	if buildErr != nil {
		t.Fatalf("building the door under test: %v\n%s", buildErr, buildOut)
	}
	return doorBin
}

// TestTheDoorIsBuiltInsidePossesOwnTempNamespace pins the half of
// ranger-base-7hyn8's fix that TestMain cannot: the name. TestMain's removal
// covers every exit this binary chooses, and the reaper's `posse-*` glob
// covers the ones it does not — a kill, a -timeout panic, a box that goes
// down mid-run. Rename the dir out of posse's namespace and both halves look
// fine: the pins stay green, the dirs stop being collectable, and nobody
// finds out until /tmp is out of inodes. Asserted over the dir that was
// actually created, not over the constant, so handing os.MkdirTemp some
// other literal reds here too.
func TestTheDoorIsBuiltInsidePossesOwnTempNamespace(t *testing.T) {
	dir := filepath.Dir(builtDoor(t))
	if !strings.HasPrefix(filepath.Base(dir), "posse-") {
		t.Errorf("the door is built into %s, which scripts/reap-test-tmp.sh cannot see: its posse-scratch class is the glob `posse-*` and no class of its class table matches anything else. A run that dies before TestMain's removal therefore leaks this dir permanently — MEASURED 2026-11-11, 278 dirs and 914M of them on this box's /tmp under the old `coldexec-pins` name.", dir)
	}
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
printf '#!/bin/sh\necho THE PLANT RAN\n' > "$p"; chmod +x "$p"
tm first "$p"
tm second "$p"
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 1)
	r.cold(t, "first")
	r.warm(t, "second")
	r.wantNoHoles(t)
	if n := strings.Count(r.out, "THE PLANT RAN"); n != 2 {
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
	r := runDoor(t, `printf '#!/bin/sh\necho VERSION ONE\n' > p; chmod +x p
tm first ./p
printf '#!/bin/sh\necho VERSION TWO\n' > p
tm rewritten ./p
`)
	r.wantCode(t, 0)
	r.wantDelayed(t, 2)
	r.cold(t, "first")
	r.cold(t, "rewritten")
	r.wantNoHoles(t)
	if !strings.Contains(r.out, "VERSION ONE") || !strings.Contains(r.out, "VERSION TWO") {
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
printf '#!/bin/sh\necho THE PLANT RAN\n' > gained
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
	if !strings.Contains(r.out, "THE PLANT RAN") {
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
	r := runDoor(t, `printf '#!/bin/sh\necho THE PLANT RAN\n' > p; chmod +x p
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
	if strings.Count(r.out, "THE PLANT RAN") != 2 {
		t.Errorf("the plant did not run twice, so the second delay was not its re-exec:\n%s", r.out)
	}
}

// withdrawalFixture is the ONE live run the four pins below read, each for
// its own claim: ARM 14 the classification and the coverage
// (ranger-base-j0riv), ARM 16 that the restart PAYS the remainder
// (ranger-base-91fu2), ARM 17 the charge nothing restarts
// (ranger-base-4kars), ARM 20 what that charge leaves behind
// (ranger-base-dgbys). It is a helper rather than four copies because the
// fixture is the expensive part and the claims are read off the same
// output; it is four pins rather than one because they red for different
// reasons and a seat that breaks one should not be told about the others.
//
// `after` is shell appended to the run once the nudged caller has ended,
// and it is empty for every pin but ARM 20 — which is the only one with a
// question about what the run looks like AFTERWARDS.
//
// saRestart PICKS THE ENDING, and it is one line of the caller: with it the
// handler carries SA_RESTART and the withdrawn execve is RESTARTED (ARM 14,
// ARM 16); without it python3's own default disposition applies, the execve
// returns EINTR and never happens, and the charge is cut short with nothing
// to carry it (ARM 17). MEASURED 2026-11-11, both endings on this box.
//
// EVERY PART OF IT IS LOAD-BEARING:
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
//   - `held` is a SECOND plant, exec'd before the caller and never
//     signalled, and it is what makes `wantCode(t, 0)` an assertion rather
//     than a coincidence. Since ranger-base-4kars a truncated charge is not
//     in the delayed count, so an EINTR run whose only plant is the nudged
//     one delays NOTHING and exits 3 — correctly, and it would make the
//     pins below read an exit code that says nothing about their own
//     subjects. ARM 17 reads the count off it.
//   - the nudged caller is timed by hand and not by the `tm` preamble,
//     because `tm`'s own exit status is its `echo`'s: ARM 14 reads
//     `CALLER EXIT 0` to tell a restarted execve from a caller the signal
//     ended, and that fact and the wall time are both wanted here.
func withdrawalFixture(t *testing.T, saRestart bool, after string) doorRun {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("no python3, so the pin cannot install a signal handler in a caller: %v", err)
	}
	mode := "eintr"
	if saRestart {
		mode = "restart"
	}
	return runDoor(t, strings.Replace(`cat > caller.py <<'PY'
import os, signal, sys, time

# Installed before the fork, so the nudger below can never catch this
# process with SIGUSR1 still fatal. siginterrupt(False) is the shortest way
# to ask for SA_RESTART, which is the Go runtime's own shape; without it
# python3's default disposition is to interrupt, and the execve takes EINTR.
signal.signal(signal.SIGUSR1, lambda *a: None)
if sys.argv[2] == "restart":
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
printf '#!/bin/sh\necho the held plant ran\n' > held; chmod +x held
tm held ./held
s=$(date +%s%N)
python3 -I caller.py ./plant MODE
c=$?
e=$(date +%s%N)
echo "CALLER EXIT $c"
echo "ELAPSED nudged $(( (e-s)/1000000 ))ms"
`, "MODE", mode, 1)+after)
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
// WHAT IT DOES NOT ASSERT, on purpose: anything about the delay. Whether
// the charge this withdrawal interrupted is paid by the restart is ARM 16's
// subject and not this one's. This pin is about the classification and the
// coverage: no holes, the withdrawal named in the report, and the withdrawn
// execve adjudicated a second time and run.
func TestAWithdrawnNotificationIsNotAHole(t *testing.T) {
	t.Parallel()
	r := withdrawalFixture(t, true, "")

	// Something was delayed — `held`, which nothing signals — so the door
	// does not exit 3 over this run.
	r.wantCode(t, 0)

	// THE ASSERTION THIS PIN EXISTS FOR.
	r.wantNoHoles(t)

	// And the withdrawal is not silent either: a seat whose run spent two
	// notifications on one execve has a line that says so. Either verbose
	// spelling counts — a withdrawal that cut a delay short reads
	// `truncated` and one that cost nothing but a notification reads
	// `withdrawn` — because which one this run produces is the mechanism's
	// business and ARM 16's subject, not this pin's.
	if !regexp.MustCompile(`(?m)^coldexec: (withdrawn|truncated) +pid `).MatchString(r.out) {
		t.Fatalf("the nudging produced no withdrawn notification, so this run did not reach the "+
			"mechanism and its wantNoHoles above proved nothing:\n%s", r.out)
	}
	if !regexp.MustCompile(`notifications were withdrawn before the door could answer|injections were TRUNCATED`).MatchString(r.out) {
		t.Errorf("the report did not account for the withdrawal, so a seat reading it has nothing "+
			"to read:\n%s", r.out)
	}

	// Coverage: the withdrawn execve was adjudicated a SECOND time — the
	// restart's own notification — rather than let through blind. Any
	// verdict counts; what this reads is that there were two of them. A
	// withdrawal that was holding a charge names the plant it handed on
	// (ranger-base-91fu2), so `withdrawn` is one of the spellings.
	adjudications := regexp.MustCompile(`(?m)^coldexec: (delayed|warm|truncated|withdrawn) +pid \d+ .*plant$`).FindAllString(r.out, -1)
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

// ARM 16: the RESTART of a withdrawn charge pays the REMAINDER, so a
// caller the kernel signalled mid-hold is still held D from where its
// execve entered the kernel.
//
// This is ranger-base-91fu2, and it is the inversion of the pin
// ranger-base-4kars left here. That bead stopped the door CLAIMING a delay
// it had not paid: `adjudicate` marked the inode before it slept, so the
// restart read it as warm and was answered at once, and the plant's exec
// completed in milliseconds against the D the report charged it — MEASURED
// 2026-11-11, D = 600ms: ~10-20ms, 40/40. It said so instead of hiding it,
// and left the fix here.
//
// The fix is a second map, from (task, inode) to the absolute deadline of a
// charge in flight. A restarted execve necessarily arrives on the SAME task
// — the kernel restarts the syscall in place — and a task can only be
// inside one execve at a time, so a notification matching a live entry IS
// that restart and nothing else. It sleeps the remainder to the original
// deadline, which makes the charge ONE charge spent over two or more
// notifications. ARM 18 pins the kernel fact that makes the key unique.
//
// WHAT IT READS, and why one of them IS a wall time now. Before this bead
// there was nothing to time: the nudged plant's exec was fast and that was
// the finding. Now the whole claim is that it is slow, and a FLOOR on it is
// the direct statement — this file's header forbids a RATIO of two wall
// times on a box that reaches a loadavg of 50-100, and load can only push a
// floor further the right way.
//
// THE ACCOUNTING beside it has two honest spellings, and it is the wall
// time above that tells them apart from the mutant — ranger-base-k9xsd,
// found while pinning ranger-base-dgbys. A restart that lands after the
// deadline has already passed finds no inflight entry left to claim and
// reads warm, so `plant`'s charge is spent as `warm` instead of `delayed`
// and the run reports 1 delayed (`held`) rather than 2 — one line of
// over-reporting on an exec that still paid D, per the note above
// recordWithdrawal in door_linux.go. What leads there is not always a
// single `truncated` line: it can also be a CASCADE of further restarts
// each superseded in turn (MEASURED: 58 withdrawn, 0 truncated, 1 final
// warm), so the pin reads the terminal verdict for `plant` and not the
// presence of `truncated`. Rare on a quiet box (10/150, jian-yang's
// measurement) and not load-only: the window is the withdrawal landing
// within microseconds of the deadline.
//
// THE MUTANT it reds on is the one the fix removes: answering the restart
// warm unconditionally. Under that door `plant` ALSO ends in `warm` — the
// same bookkeeping as the rare race above — but every run, and with a
// `nudged` step of a few tens of ms rather than the ~D the race still
// pays. The wall time is what the accounting alone cannot tell apart.
func TestARestartedExecvePaysTheRestOfItsCharge(t *testing.T) {
	t.Parallel()
	r := withdrawalFixture(t, true, "")
	r.wantCode(t, 0)
	r.wantNoHoles(t)

	// The mechanism was reached: a charge was being held when the kernel
	// took the notification back, and the line names the plant whose charge
	// it handed on. Without this the assertions below are satisfied by a run
	// that was never signalled at all.
	if !regexp.MustCompile(`(?m)^coldexec: withdrawn +pid \d+ .*plant$`).MatchString(r.out) {
		t.Fatalf("no withdrawn verdict names the plant, so this run never interrupted a charge "+
			"and the assertions below prove nothing:\n%s", r.out)
	}

	// THE ASSERTION THIS PIN EXISTS FOR: the nudged caller's execve was held
	// to the end of the charge it started, however many notifications it
	// took to get there — and this wall time is what the real mutant still
	// reds on: a door that stopped carrying the charge answers the restart
	// warm in milliseconds, every run, not in one row of 150.
	r.cold(t, "nudged")

	// The accounting has two honest spellings. door_linux.go's note above
	// recordWithdrawal names one of them: a restart that arrives after the
	// deadline has already passed finds no inflight entry left to claim, so
	// it is answered warm — one line of over-reporting on an exec that
	// still paid D, never an exec let through early. The terminal verdict
	// for `plant` is what tells the two spellings apart, and not the
	// presence of a `truncated` line: that race can also play out as a
	// CASCADE of further restarts chasing the same deadline, each withdrawn
	// in turn (so the notification N1 was holding never itself reads
	// truncated — it reads withdrawn, handed to a restart that is itself
	// superseded) until the last one lands past the deleted entry and
	// reads warm — MEASURED here (ranger-base-k9xsd): 69 execve
	// notifications, 58 withdrawn, 1 warm, no truncated line at all. Either
	// way the charge is spent as 1 delayed (`held`) plus a `plant` that
	// ends in `warm` instead of 2 delayed. Rare on a quiet box (10/150,
	// jian-yang's measurement) but not load-only: the window is the
	// withdrawal landing within microseconds of the deadline, not a slow
	// box.
	plantWasWarm := regexp.MustCompile(`(?m)^coldexec: warm +pid \d+ .*plant$`).MatchString(r.out)
	if plantWasWarm {
		r.wantDelayed(t, 1)
	} else {
		r.wantDelayed(t, 2)
	}
	r.cold(t, "held")

	// And it ran, under that caller, so what was held was a real execve and
	// not a notification for something that never happened.
	if !strings.Contains(r.out, "the plant ran") {
		t.Errorf("the plant never ran, so the charge was paid by nobody's exec:\n%s", r.out)
	}
	if !strings.Contains(r.out, "the held plant ran") {
		t.Errorf("the held plant never ran, so one of the two delays was somebody else's:\n%s", r.out)
	}
	if !strings.Contains(r.out, "CALLER EXIT 0") {
		t.Errorf("the caller did not exit 0 — the signal ended it instead of restarting its "+
			"execve, which is ARM 17's ending and not this one's:\n%s", r.out)
	}
}

// ARM 17: a charge NOTHING restarts is what `truncated` means after
// ranger-base-91fu2, and that execve never happens.
//
// Same fixture, one line of the caller different: no `siginterrupt(False)`,
// so the handler is python3's own default disposition and carries no
// SA_RESTART. The kernel withdraws the notification, the execve returns
// EINTR to the caller and is NOT restarted, and the charge the door was
// holding ends with nothing to carry it. MEASURED 2026-11-11 on this box:
// InterruptedError, exit 1, and the plant does not run.
//
// So this pin is what is LEFT of ranger-base-4kars' subject, and it is a
// narrower claim than that bead's was: a truncation is no longer an exec
// that went through at full speed — ARM 16 is that one and it is held now —
// it is a charge spent on an execve that did not occur. What it leaves
// behind is a plant marked warm, which is why the verdict is still counted
// and named rather than folded into `withdrawn`.
//
// No wall time, on purpose. "The caller was released at the signal and not
// at D" would be a CEILING, and a ceiling on a box that reaches a loadavg
// of 50-100 is this file's header's definition of a flake. The accounting
// and the caller's own exit code say it exactly.
func TestAChargeNothingRestartsReadsTruncated(t *testing.T) {
	t.Parallel()
	r := withdrawalFixture(t, false, "")
	r.wantCode(t, 0)
	r.wantNoHoles(t)

	// THE ASSERTION THIS PIN EXISTS FOR.
	if !regexp.MustCompile(`(?m)^coldexec: truncated +pid \d+ .*plant$`).MatchString(r.out) {
		t.Fatalf("no truncated verdict names the plant, so this run did not cut a charge short "+
			"and the rest of this pin proves nothing:\n%s", r.out)
	}
	if !strings.Contains(r.out, "charges were TRUNCATED") {
		t.Errorf("the report did not name the truncation, so a seat whose door spent a charge on "+
			"an exec that never happened has nothing to read:\n%s", r.out)
	}

	// The charge is not in the delayed count: `held` is the only plant this
	// run held to the end, and it is the reason the door does not exit 3
	// over a run that delayed nothing.
	r.wantDelayed(t, 1)
	r.cold(t, "held")

	// And the other half of what `truncated` now means: the execve did not
	// happen, so nothing escaped the door at full speed.
	if strings.Contains(r.out, "the plant ran") {
		t.Errorf("the plant ran, so this caller RESTARTED its execve — that is ARM 16's ending, "+
			"and this run is not the one this pin is about:\n%s", r.out)
	}
	if !strings.Contains(r.out, "CALLER EXIT 1") {
		t.Errorf("the caller did not exit 1 — EINTR out of execv is what ends it, and without "+
			"that this run reached a different ending of the mechanism:\n%s", r.out)
	}
}

// ARM 18: a notification names the TASK that raised it, not its thread
// group — which is the one kernel fact ARM 16's mechanism rests on.
//
// The in-flight charge is keyed by (pid, inode), and a restarted execve is
// told from an unrelated exec of the same plant by that pid alone. The
// kernel fills `seccomp_notif.pid` from `task_pid_vnr(current)`, so it is
// the tid; if it were `task_tgid_vnr` two threads of one process exec'ing
// one plant would collide on the key and a concurrent exec would be
// mistaken for a restart and made to sleep somebody else's remainder.
// MEASURED 2026-11-11 (this box, linux 7.2.15 x86_64, go1.27.3): 3 of 3,
// tid != tgid in all three.
//
// Nothing in the arms execs from a non-leader thread today, so this is a
// pin on an assumption and not on a behaviour anyone has seen break — which
// is exactly the kind ADR 0085 D5 asks to be written down rather than
// carried in a comment.
func TestANonLeaderThreadsExecIsAdjudicatedUnderItsOwnTid(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("no python3, so the pin cannot exec a plant from a second thread: %v", err)
	}
	r := runDoor(t, `cat > thread.py <<'PY'
import os, sys, threading

def go():
    # The execve happens on THIS task. It keeps only the calling thread, so
    # the join below never returns.
    print("TGID %d TID %d" % (os.getpid(), threading.get_native_id()), flush=True)
    os.execv(sys.argv[1], [sys.argv[1]])

t = threading.Thread(target=go)
t.start()
t.join()
PY
printf '#!/bin/sh\necho THE THREADED PLANT RAN\n' > plant; chmod +x plant
python3 -I thread.py ./plant
`)
	r.wantCode(t, 0)
	r.wantNoHoles(t)
	if !strings.Contains(r.out, "THE THREADED PLANT RAN") {
		t.Fatalf("the plant never ran, so no execve of it was adjudicated:\n%s", r.out)
	}

	m := regexp.MustCompile(`TGID (\d+) TID (\d+)`).FindStringSubmatch(r.out)
	if m == nil {
		t.Fatalf("the caller printed no tgid/tid pair, so there is nothing to compare:\n%s", r.out)
	}
	tgid, tid := m[1], m[2]
	if tgid == tid {
		t.Fatalf("the exec'ing thread WAS the group leader (tgid %s), so this run cannot tell the "+
			"two readings apart:\n%s", tgid, r.out)
	}

	// THE ASSERTION THIS PIN EXISTS FOR.
	v := regexp.MustCompile(`(?m)^coldexec: delayed +pid (\d+) .*plant$`).FindStringSubmatch(r.out)
	if v == nil {
		t.Fatalf("the plant's first exec was not delayed, so the door did not adjudicate it:\n%s", r.out)
	}
	if v[1] != tid {
		t.Errorf("the notification named pid %s; the exec'ing TASK was tid %s (tgid %s). The "+
			"in-flight charge key is (pid, inode) and it is unique only while that pid is the "+
			"task id — see the note above recordWithdrawal:\n%s", v[1], tid, tgid, r.out)
	}
	r.wantDelayed(t, 1)
}

// ARM 21 (ranger-base-nr3qf, escaped from ranger-base-91fu2): the `pid`
// component of the in-flight charge key is asserted by NO other arm here,
// although ARM 18 above exists only to pin the kernel fact that makes it
// sound — a key of `(pid, inode)` is unique only because the pid the kernel
// hands a task's notification is that task's own, so two DIFFERENT tasks
// exec'ing the same fresh inode at the same time can never collide on it.
//
// REPRO (MEASURED 2026-11-11, this box): zero the pid in the key at
// door_linux.go:597 (`charge{pid: 0, key: key}`) and every other arm in this
// file, 14/15/16/17/18 included, stays green — none of them puts two
// DIFFERENT tasks on the same fresh inode at once. This one does: two
// separate `sh`-forked tasks exec the one plant concurrently. Shipped, the
// loser of the race is simply warm — the door's cost model charges an inode
// once, not per task (ADR 0085 D5, see ARM 20's header) — so this run reads
// 1 delayed, 1 warm. Zero the pid and the two charges collide on one map
// entry: the loser's own exec is misread as the RESTART of the winner's
// charge and is made to sleep the winner's remainder too, so the run reads
// 2 delayed, 0 warm.
func TestTwoTasksExecingOnePlantConcurrentlyChargeOnlyOneOfThem(t *testing.T) {
	t.Parallel()
	r := runDoor(t, `printf '#!/bin/sh\necho THE PLANT RAN\n' > plant; chmod +x plant
tm first ./plant &
tm second ./plant &
wait
`)
	r.wantCode(t, 0)
	r.wantNoHoles(t)

	// THE ASSERTION THIS PIN EXISTS FOR: of the two concurrent execs of one
	// fresh inode, exactly one pays D. Two tasks colliding on a pid-less key
	// read 2 here, and no other arm's count would move.
	r.wantDelayed(t, 1)

	cold := 0
	for _, label := range []string{"first", "second"} {
		d, ok := r.elapsed[label]
		if !ok {
			t.Fatalf("the run timed no step called %q, so it measured nothing:\n%s", label, r.out)
		}
		if d >= pinPartition {
			cold++
		}
	}
	if cold != 1 {
		t.Errorf("%d of the two concurrent execs paid D, want exactly 1 — a pid-less key would make the "+
			"second collide with the first's charge and pay it too:\n%s", cold, r.out)
	}

	if n := strings.Count(r.out, "THE PLANT RAN"); n != 2 {
		t.Errorf("the plant ran %d times, want 2 — both concurrent execs must have gone through:\n%s", n, r.out)
	}
	// Neither exec was withdrawn or restarted: this is plain concurrency, not
	// a signal cutting a charge short (that is ARMs 16/17's subject).
	if strings.Contains(r.out, "truncated") || strings.Contains(r.out, "withdrawn") {
		t.Errorf("a charge read as truncated or withdrawn — this run signalled nothing, so there was "+
			"nothing to cut short or hand on:\n%s", r.out)
	}
}

// ARM 19: a HOLE whose notification the kernel took back is recorded as no
// hole at all — the second half of ranger-base-j0riv's rule, which that
// bead landed and pinned nothing of (ranger-base-dgbys).
//
// THE RULE IS TWO PARTS and ARM 14 above reaches only the first. (a) a
// withdrawal is not itself a hole — that is ARM 14, and the mutant that
// puts `vWithdrawn` back into `holes` reds it. (b) a hole is an execve the
// door let through without knowing what it was, so WHETHER IT WAS LET
// THROUGH AT ALL is decided at the answer and nowhere else: adjudicate
// records no hole, every hole rides down to answer(), and answer() records
// it only once ID_VALID has said the notification was still live.
//
// MEASURED at 436f467c, by mutation: the three lines that revert (b) for
// the holes — `handle` recording its own pending hole before the answer,
// which is what the shape looked like before j0riv — leave `go test
// ./cmd/coldexec` GREEN, at -count=1 and at -count=5. The delay half of the
// same rule is pinned, by ARM 17's `wantDelayed`: reverting (b) for the
// delays too reds ARMs 14/16/17 at once. So what was unpinned is exactly
// the holes, and this is the pin for them.
//
// WHY THIS ONE IS NOT A SUPERVISED RUN, which every other pin in this file
// is. The live shape needs the caller to be GONE while the door is inside
// readPath or stat for that same notification, and the caller only gets
// there by being withdrawn — so the window is the microseconds between the
// door taking the notification off the listener and finishing its /proc
// reads, against a caller that takes milliseconds to die. MEASURED
// 2026-11-11 on this box (linux 7.2.15 x86_64, go1.27.3) over 900 nudged
// EINTR callers — 100 at each of nine nudge intervals from a tight loop to
// 10ms, 207 of them withdrawn — ZERO produced a hole. The rate the finding
// that filed this read (11 of 250 runs) was taken on a box under `go test
// ./...`, which is where j0riv's original red lived and where this file's
// header says a wall-clock fixture is a flake. A fixture pin would
// therefore cost seconds a run to assert nothing here and red somewhere
// else, so this reads the rule off `handle` instead: a hole verdict
// adjudicate really produced, against an answer that does not land.
//
// THE LISTENER IS A CLOSED FD, and that is faithful rather than
// convenient: answer() branches on ID_VALID returning an error and on
// nothing else, deliberately — a door is not in a position to tell the
// kernel's ENOENT for a notification it no longer has from any other
// reason it cannot ask. The pin asserts the branch that error picks.
func TestAHoleIsNotRecordedWhenTheKernelTookTheNotificationBack(t *testing.T) {
	t.Parallel()

	// A pid that is GONE, which is what every real hole in this tree has
	// been: readPath opens /proc/<pid>/mem and the caller is not there.
	// Run() waits, so the child is reaped and its /proc entry removed
	// before this returns.
	gone := exec.Command("/bin/true")
	if err := gone.Run(); err != nil {
		t.Fatalf("running /bin/true to get a pid that is over with: %v", err)
	}
	dead := uint32(gone.Process.Pid)

	newDoor := func() *door {
		return &door{
			listener: -1, // every ioctl on it fails: see the note above
			delay:    pinDelay,
			started:  time.Now(),
			seen:     map[inode]bool{},
			inflight: map[charge]*held{},
		}
	}

	for _, c := range []struct {
		name string
		n    *seccompNotif
		want verdict
	}{
		// The hole the tree actually produces (ranger-base-orgvt measured
		// 134 of them in one arm before the subreaper flag): the path
		// argument cannot be read out of the caller.
		{"unreadable", &seccompNotif{ID: 1, Pid: dead, Data: seccompData{Nr: int32(unix.SYS_EXECVE)}}, vUnreadable},
		// The hole that says the filter and adjudicate's switch have
		// drifted apart. Nothing can raise it today, which is why it is
		// here: it reaches the same rule with no /proc in the way at all.
		{"foreign", &seccompNotif{ID: 2, Pid: dead, Data: seccompData{Nr: int32(unix.SYS_READ)}}, vForeign},
	} {
		t.Run(c.name, func(t *testing.T) {
			// THE POSITIVE CONTROL. adjudicate really does come back with
			// this hole, and really does leave it unrecorded — so what the
			// assertion below reads is a hole DROPPED at the answer and not
			// a hole that was never made.
			d := newDoor()
			pending, taken, target := d.adjudicate(c.n)
			if pending != c.want {
				t.Fatalf("adjudicate answered %s, want %s — this case no longer reaches a hole and "+
					"the rest of it proves nothing (pid %d reused?)", verdictNames[pending], verdictNames[c.want], dead)
			}
			if taken != vWithdrawn {
				t.Errorf("a hole's withdrawal verdict is %s, want %s: a hole was never a charge, so "+
					"there is nothing for the kernel to have cut short", verdictNames[taken], verdictNames[vWithdrawn])
			}
			if target == "" {
				t.Errorf("the hole named no target, so recordWithdrawal cannot say what the door " +
					"could not read")
			}
			if n := d.verdicts[c.want].Load(); n != 0 {
				t.Errorf("adjudicate recorded %d %s itself — it must record no hole at all, because "+
					"whether the execve was let through is not its to decide", n, verdictNames[c.want])
			}

			// THE ASSERTION THIS PIN EXISTS FOR.
			d = newDoor()
			d.handle(c.n)
			if n := d.verdicts[c.want].Load(); n != 0 {
				t.Errorf("the run was charged %d %s for a notification the kernel took back. The "+
					"door let no live execve through here — there was nothing left to let through — "+
					"and a hole counted for one is `wantNoHoles` redding on load and not on code, "+
					"which is ranger-base-j0riv's own symptom", n, verdictNames[c.want])
			}
			if n := d.verdicts[vWithdrawn].Load(); n != 1 {
				t.Errorf("the withdrawal was recorded %d times, want 1: a notification that spends no "+
					"verdict at all is one the report cannot account for", n)
			}
		})
	}
}

// ARM 20: what a TRUNCATED charge leaves behind — the plant is marked warm,
// so the first exec of it that really happens is not held. ADR 0085 D5
// states that edge and doc.go repeats it; this is its witness
// (ranger-base-dgbys).
//
// ARM 17 above has the first half: a caller whose handler lacks SA_RESTART
// takes EINTR, its execve never happens, and the charge the door was
// holding ends with nothing to carry it. What ARM 17 stops short of is the
// consequence, and the consequence is a coverage claim: `d.seen[key]` was
// marked before the sleep and is not unmarked, so the NEXT caller's execve
// of that same plant — the first exec of that inode that occurs at all —
// reads `warm` and goes through at full speed, which is the one thing ADR
// 0085 D5's headline says cannot happen.
//
// MEASURED 2026-11-11, this box (linux 7.2.15 x86_64, go1.27.3), D = 600ms,
// against the door at 436f467c: the abandoned execve is charged and the
// plant's real first exec takes 2ms. Deterministic.
//
// IT PINS THE EDGE AS STATED AND NOT A FIX, and the reason is measured
// rather than preferred. ranger-base-91fu2 made the RESTART case pay —
// which is every truncation this tree has ever produced, since the only
// callers here that can be withdrawn at all are the Go runtime's own
// (SA_RESTART on SIGURG), and that bead measured the truncation count go 11
// -> 0 over the arm that had them. So the remaining edge needs a caller
// shape no arm has. Closing it means unmarking the inode when a charge is
// cut short, and `d.seen` is also what stops two concurrent first execs by
// DIFFERENT tasks from both paying D — a cost-model change, which ADR 0085
// D5 decided the other way in as many words ("the `seen` mark stays where
// it was"). So: if this pin ever reds green-side-up — the second exec
// reading `delayed` — somebody has closed the edge, and D5's cut-short
// bullet and doc.go's "WHAT IT DOES NOT COVER" are what to update with it.
//
// No wall time ASSERTED, deliberately, and it loses nothing: the door holds an
// execve if and only if it answers `delayed`, so the verdict on the second
// exec and the run's own delayed count say "not held" exactly, with none of
// the load sensitivity this file's header warns a ceiling carries.
func TestATruncatedChargeLeavesItsPlantWarm(t *testing.T) {
	t.Parallel()
	// The sleep is the door's OWN hold and not a guess at one: the handler
	// holding the truncated charge is asleep until its deadline, and until
	// it wakes there is nothing a fix could have done about the mark yet.
	// It is not an assertion — a sleep that fell short would cost this pin
	// its grip on such a fix and could never red a door that has none — so
	// the load this file's header warns about cannot reach it.
	after := fmt.Sprintf("sleep %.1f\ntm firstreal ./plant\n", (pinDelay + 400*time.Millisecond).Seconds())
	truncatedPlant := regexp.MustCompile(`(?m)^coldexec: truncated +pid \d+ .*plant$`)

	// THE POSITIVE CONTROL IS RETRIED, and that is ranger-base-9n4y9's own
	// prescription rather than this pin being lenient. The fixture's nudge
	// can reach the caller as a signal that interrupts the execve BEFORE
	// the door is holding its notification at all: the kernel then removes
	// a notification the door never received, the caller takes EINTR and
	// exits 1 exactly as it should, and no charge was ever cut short — so
	// the run reached none of the mechanism and has nothing to say about
	// this pin's subject either way. MEASURED 2026-11-11 on a quiet box, 150
	// executions of each: this pin and ARM 17 both lost 1, with the same
	// message and the same cause, which is the rate that bead was filed for.
	// A trial that cannot be run is not a failing trial; a door that has
	// stopped truncating fails all of them, so the Fatalf below still fires
	// for the thing this pin is here to catch.
	const tries = 4
	var r doorRun
	for i := 1; ; i++ {
		r = withdrawalFixture(t, false, after)
		if truncatedPlant.MatchString(r.out) {
			break
		}
		if i == tries {
			t.Fatalf("no truncated verdict named the plant in %d runs, so the door cut no charge "+
				"short — at ranger-base-9n4y9's measured rate that is not the nudge missing "+
				"(0.0067^%d), it is the mechanism gone:\n%s", tries, tries, r.out)
		}
		t.Logf("run %d reached no truncation — the nudge interrupted the execve before the door "+
			"held it (ranger-base-9n4y9); retrying", i)
	}
	r.wantCode(t, 0)
	r.wantNoHoles(t)
	if !strings.Contains(r.out, "CALLER EXIT 1") {
		t.Fatalf("the caller did not exit 1, so its execve was restarted rather than abandoned — "+
			"that is ARM 16's ending and the plant below would not be cold either way:\n%s", r.out)
	}
	if strings.Count(r.out, "the plant ran") != 1 {
		t.Fatalf("the plant ran %d times, want exactly 1 — the abandoned execve must NOT have run "+
			"it and the exec after it must have:\n%s", strings.Count(r.out, "the plant ran"), r.out)
	}

	// THE ASSERTION THIS PIN EXISTS FOR. That one exec is the first exec of
	// this inode that ever happened, and the door read it as a repeat.
	if !regexp.MustCompile(`(?m)^coldexec: warm +pid \d+ .*plant$`).MatchString(r.out) {
		t.Errorf("the plant's first exec that actually HAPPENED was not read as warm. If it was "+
			"held, the edge in ADR 0085 D5's cut-short bullet is closed and that bullet, doc.go's "+
			"\"WHAT IT DOES NOT COVER\" and the note above recordWithdrawal all still say it is "+
			"open:\n%s", r.out)
	}

	// And the arithmetic says the same thing without reading a clock:
	// `held` is the only exec this run paid D for.
	r.wantDelayed(t, 1)
	r.cold(t, "held")
}
