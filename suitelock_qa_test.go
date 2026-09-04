package posse

// QA pins for ranger-base-uvzjk — the box-wide suite queue.
//
// Claim: at most POSSE_SUITE_SLOTS (2) full `go test ./...` runs happen on
// this machine at once, and the rest wait with a line naming the worktree
// they are waiting on.
//
// MEASURED 2026-09-04 02:35Z, from the post-WindowServer-crash sitting. This
// box carried FIVE concurrent `go test ./...` runs from five crew worktrees —
// three bare `go test -timeout 25m ./...`, two under scripts/test-times.sh —
// on eight cores, at 14% free memory with 1.47M pageouts. `go run
// ./cmd/checkorphans` was clean, so none of it was a leak. Each suite ran at
// 2-3x its solo time (one pane reported the root package at 551s), and the
// 1-minute loadavg was 899 against the fleet load guard's ceiling of 60 — so
// the shop stopped hiring at exactly the moment five seats were about to
// free. No load run was made to reproduce it: the standing operator ruling
// (2026-08-31) is no load testing on this box, and the ambient numbers above
// are the measurement.
//
// FIVE ARMS, because the queue can be lost four ways and there are two
// wrappers that can lose it the fourth way:
//
//  1. the Makefile stops running the self-test, at which point the crew's
//     only re-measuring artifact is gone. Arm 1 pins the recipe.
//  2. the script survives and nothing runs its self-test. `make
//     verify-suite-lock` runs it; a plain `go test ./...` does not, so a
//     gutted lock would ship green. Arm 2 runs it from inside the suite.
//  3. the self-test stops being able to fail. Arm 3 breaks a COPY of the
//     script — flock always succeeds, which is what "no lock at all" looks
//     like from the inside — and requires the self-test to notice.
//  4. the LIBRARY keeps working and a wrapper stops calling it. That is the
//     failure a source-reading pin would sleep through, so arms 4 and 5 hold
//     the only slot from this process and require a real
//     scripts/test-times.sh and a real scripts/gotest.sh to queue behind it
//     and then run — by execution, not by grep.
//
// What this does NOT claim, and no close should: that two slots is a measured
// optimum. Two is the number ranger-base-uvzjk asked for. What is measured is
// that five was too many.

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The queue, by path. Named rather than pattern-matched: one specific script
// carrying one specific measurement.
const suiteLockScript = "scripts/suite-lock.sh"

// The arm labels the self-test prints, by name. Required here rather than
// counted, because a self-test that quietly stopped exercising an arm would
// otherwise go green by deletion.
var suiteLockArms = []string{
	"slots: two concurrent full suites both run",
	"queue: a third full suite waits",
	"queue: the waiting line names the holding worktree",
	"unlocked: a -run filtered suite takes no slot",
	"unlocked: a single-package run takes no slot",
	"queue: a freed slot is taken by the waiter",
	"crash: the slot of a kill -9 run is reclaimed",
	"nested: a run inside a held slot takes no second slot",
	"opt-out: POSSE_SUITE_LOCK=0 runs unserialized and says so",
	"release: a slot handed back is free before the process exits",
	"set -e: a queued acquire does not kill the wrapper",
	// ranger-base-jhyiv: the two arms that hold the header's "it never makes
	// the suite unrunnable" promise against the variable that decides the
	// width. A non-numeric value queued forever against a line naming
	// nobody; 0 and -1 handed out MORE slots than the default, because seq
	// counts down.
	"slots: a bad POSSE_SUITE_SLOTS runs on the default and says so",
	"slots: a negative POSSE_SUITE_SLOTS does not widen the queue",
}

// Arm 1: `make test` still runs the queue's self-test, and `make
// verify-suite-lock` is still what runs it. Without this the queue has no
// re-measuring command a reader can find.
func TestQAMakeTestRunsTheSuiteLockSelfTest(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	src := string(makefile)

	recipe := makeRecipe(src, "verify-suite-lock")
	if len(recipe) == 0 {
		t.Fatal("the Makefile has no `verify-suite-lock` target — the queue has no re-measuring command")
	}
	var runs bool
	for _, l := range recipe {
		if isComment(l) {
			continue
		}
		if strings.Contains(l, suiteLockScript) && strings.Contains(l, "--self-test") {
			runs = true
			break
		}
	}
	if !runs {
		t.Errorf("`make verify-suite-lock` no longer runs %s --self-test:\n%s",
			suiteLockScript, strings.Join(recipe, "\n"))
	}

	// And `make test` still depends on it, or arm 2 is the only thing
	// keeping the queue alive on this box.
	var deps string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "test:") {
			deps = line
			break
		}
	}
	if deps == "" {
		t.Fatal("the Makefile has no `test` target")
	}
	if !strings.Contains(deps, "verify-suite-lock") {
		t.Errorf("`make test` no longer depends on verify-suite-lock: %q", deps)
	}
}

// Arm 2: the self-test still proves what it says, arm by arm.
func TestQATheSuiteQueueStillProvesItQueues(t *testing.T) {
	if _, err := os.Stat(suiteLockScript); err != nil {
		t.Fatalf("%s is gone: %v", suiteLockScript, err)
	}
	out, err := exec.Command("bash", suiteLockScript, "--self-test").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --self-test failed: %v\n%s", suiteLockScript, err, out)
	}
	got := string(out)
	for _, want := range suiteLockArms {
		if !strings.Contains(got, "ok    "+want) {
			t.Errorf("%s --self-test no longer proves %q — the arm is gone, failing, or renamed\n%s",
				suiteLockScript, want, got)
		}
	}
}

// Arm 3: the self-test can actually fail. A `--self-test` that prints its arm
// labels and exits 0 whatever the script does satisfies arms 1 and 2 exactly
// as well as a working one — that mutant survived the first pass over
// scripts/gotest.sh's self-test, and this file is not going to repeat it.
//
// The mutation is the one that matters: make the flock always succeed, which
// is what having no lock at all looks like from the inside. It is written
// against the FUNCTION rather than against python3 or flock(1) so it lands
// the same way on a box that has only one of them.
func TestQATheSuiteQueueSelfTestCanFail(t *testing.T) {
	src, err := os.ReadFile(suiteLockScript)
	if err != nil {
		t.Fatal(err)
	}
	const flockFn = "_suite_lock_flock() {\n\tcase ${_SUITE_LOCK_TOOL:-} in"
	if !strings.Contains(string(src), flockFn) {
		t.Fatalf("%s no longer contains the function this arm mutates (%q) — "+
			"the arm cannot fail and is therefore not evidence", suiteLockScript, flockFn)
	}
	broken := strings.Replace(string(src), flockFn,
		"_suite_lock_flock() {\n\treturn 0\n\tcase ${_SUITE_LOCK_TOOL:-} in", 1)

	path := filepath.Join(t.TempDir(), "suite-lock-broken.sh")
	if err := os.WriteFile(path, []byte(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", path, "--self-test").CombinedOutput()
	if err == nil {
		t.Fatalf("%s --self-test passed with the lock disabled: the self-test cannot fail, "+
			"so the other arms in this file prove nothing\n%s", suiteLockScript, out)
	}
	if !strings.Contains(string(out), "FAIL  queue: a third full suite waits") {
		t.Errorf("the self-test failed, but not on the arm that was broken — "+
			"it is refusing for some other reason and is not measuring the queue\n%s", out)
	}
}

// Arms 4 and 5: the WRAPPERS are wired to the queue, proved by holding the
// only slot from this process and watching a real wrapper wait for it.
//
// This is what a grep for `suite_lock_acquire` cannot replace. The library
// can be perfect and fully self-tested while the wrapper that is supposed to
// call it does not, and that failure looks exactly like the incident: five
// suites, no queue, nothing in any log saying so.

// holdTheOnlySlot takes slot 1 of a scratch lock dir the way another suite
// would — an flock, from a process that is not the wrapper — and stamps it.
// The stamp is a courtesy for the waiting line and nothing reads it to decide
// anything (the kernel holds the lock), but an unstamped slot gets the honest
// generic phrasing instead of a worktree, and these arms are about what the
// waiting reader is told. The returned func frees it.
func holdTheOnlySlot(t *testing.T, dir string) func() {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "suite-slot.1.lock"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		t.Fatalf("could not hold the scratch slot: %v", err)
	}
	if _, err := f.WriteString("pid: 4242\nsince: 2026-09-04T00:00:00Z\nworktree: /a/held/worktree\n"); err != nil {
		f.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return func() {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
	}
}

// queueEnv is the scratch queue: one slot, a fast poll, and no inherited
// holder. POSSE_SUITE_LOCK_HELD is set empty on purpose — it is inherited
// when this test runs inside a queued suite, and a wrapper that reads it
// skips the queue, which is correct behaviour and would leave these arms
// measuring nothing.
func queueEnv(dir string) []string {
	return append(os.Environ(),
		"POSSE_SUITE_LOCK_DIR="+dir,
		"POSSE_SUITE_SLOTS=1",
		"POSSE_SUITE_LOCK_POLL=0.2",
		"POSSE_SUITE_LOCK_HELD=",
		"POSSE_TEST_SIGNAL_LOG="+filepath.Join(dir, "signal.log"),
	)
}

// awaitWaitingLine reads stderr until the wrapper says it is queued, and
// fails the test if it never does.
func awaitWaitingLine(t *testing.T, what string, stderr io.Reader) {
	t.Helper()
	said := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if strings.Contains(sc.Text(), "waiting for suite lock held by") {
				said <- sc.Text()
				return
			}
		}
		close(said)
	}()
	select {
	case line, open := <-said:
		if !open {
			t.Fatalf("%s never said it was waiting for a slot — the wrapper is not calling the queue", what)
		}
		if !strings.Contains(line, "/a/held/worktree") {
			t.Errorf("%s: the waiting line does not name the holding worktree: %q", what, line)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("%s printed no waiting line in 20s — the wrapper is not calling the queue", what)
	}
}

// Arm 4: scripts/test-times.sh, the wrapper `make test` runs.
func TestQATestTimesQueuesBehindAHeldSlot(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	free := holdTheOnlySlot(t, dir)

	// A stub for `go`, so the arm measures the queue and never runs a
	// suite. It records that it ran, which is the thing that must NOT
	// happen while the slot is held.
	ran := filepath.Join(dir, "it-ran")
	stub := filepath.Join(dir, "faketest")
	if err := os.WriteFile(stub, []byte("#!/usr/bin/env bash\ntouch "+ran+"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "scripts/test-times.sh", stub, "test", "-timeout", "25m", "./...")
	cmd.Env = queueEnv(dir)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	awaitWaitingLine(t, "scripts/test-times.sh", stderr)

	// ...and it must not have RUN. The line without the wait would be
	// theatre.
	if _, err := os.Stat(ran); err == nil {
		t.Fatal("scripts/test-times.sh ran the command while the only slot was held")
	}

	// Free the slot: the wrapper must then run, and exit clean. This is
	// the control — without it, "it did not run" is equally green over a
	// wrapper that never runs anything.
	free()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("scripts/test-times.sh failed after the slot was freed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("scripts/test-times.sh never started after the slot was freed — the queue does not drain")
	}
	if _, err := os.Stat(ran); err != nil {
		t.Errorf("the slot was freed and the command still never ran: %v", err)
	}
}

// Arm 5: scripts/gotest.sh, which `make test-reuse` runs over `./...` and
// which is the OTHER wrapper that can put a full suite on this box. Driven
// over a one-package tree (./cmd/buildstamp/... — it has no test files, so
// this costs a link and nothing else) because the argv is what the queue
// reads, not the size of the run behind it.
func TestQAGotestQueuesBehindAHeldSlot(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	free := holdTheOnlySlot(t, dir)

	cmd := exec.Command("bash", "scripts/gotest.sh", "./cmd/buildstamp/...")
	cmd.Env = append(queueEnv(dir), "POSSE_TESTBIN_CACHE="+filepath.Join(dir, "testbin"))
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	awaitWaitingLine(t, "scripts/gotest.sh", stderr)
	if out.Len() != 0 {
		t.Fatalf("scripts/gotest.sh ran packages while the only slot was held:\n%s", out.String())
	}

	free()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("scripts/gotest.sh failed after the slot was freed: %v\n%s", err, out.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatal("scripts/gotest.sh never started after the slot was freed — the queue does not drain")
	}
	if !strings.Contains(out.String(), "cmd/buildstamp") {
		t.Errorf("the slot was freed and the wrapper still ran nothing:\n%s", out.String())
	}
}
