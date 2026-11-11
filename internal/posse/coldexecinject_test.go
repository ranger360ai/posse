//go:build !posse_arm2 && !posse_arm3

package posse

// The injector's own pins (ADR 0085). coldexec_test.go carries the
// mechanism and the argument for it; these are the five claims `make
// coldexec-check` rests on, and they are claims about a REAL kernel exec of
// a REAL planted file, because a gate whose subject is "the verdict changes
// when the first exec is slow" cannot be pinned by reading anything.
//
// HOW THEY AVOID BEING TIMING TESTS, which is the thing worth knowing before
// editing one. The marker file is created if and only if the shim takes its
// first-exec branch, so "the shim ran" and "the shim ran once" are
// filesystem facts and not readings of a clock. Exactly one assertion is a
// duration, and it is one-directional — the first exec took AT LEAST most of
// the injected delay — which load can only make truer. The companion
// assertion that a re-exec was not delayed needs a clock, so the delay these
// pins inject is a second and a half and the bound is a full second of
// slack: a warm `/bin/sh` on this box costs about a millisecond (MEASURED
// 2026-11-10, n=40, mean 0.96ms).
//
// They drive their own coldExecInjector rather than the package variable
// TestMain installs. The variable is read by WriteExecutable on every
// parallel test in the binary, so a pin that assigned it would be injecting
// into the rest of the suite; an injector of its own, with its own marker
// dir, measures the same code with none of that. The one claim that cannot
// be made that way is the WIRING — that the hook is consulted, and only for
// a _test.go caller — and that one re-execs this binary with the variable
// set, which is this package's established shape for a question about its
// own TestMain.

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// coldExecPinDelay is what these pins inject, and coldExecPinFloor is the
// bound they read against it. Not coldExecDefaultDelay's 2.5s: that number
// is sized for darwin's worst cold exec and these pins only need the sleep
// to dominate a warm exec by a margin no load can close.
const (
	coldExecPinDelay = 1500 * time.Millisecond
	coldExecPinFloor = time.Second
)

// coldExecTestInjector builds a per-test injector. The marker dir is its own
// t.TempDir, so it is outside every script dir a pin plants into — which is
// ADR 0085 D2's reason for putting markers outside the script dir at all: a
// test that lists the directory it planted into must see no extra file.
func coldExecTestInjector(t *testing.T) *coldExecInjector {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return &coldExecInjector{shim: exe, markerDir: t.TempDir()}
}

// coldExecPlant writes body through the injector and then through
// WriteExecutable — the one door — and returns the nonce the shim will key
// on, read back off the file by the same reader the shim uses. Under `make
// coldexec-check` the hook is live and sees these bytes a second time; it
// leaves them alone, which is what rewrite's idempotence is for.
func coldExecPlant(t *testing.T, inj *coldExecInjector, path, body string) string {
	t.Helper()
	if err := WriteExecutable(path, inj.rewrite(path, []byte(body)), 0o755); err != nil {
		t.Fatalf("plant %s: %v", path, err)
	}
	script, nonce, interp, ok := coldExecShimArgs([]string{inj.shim, path})
	if !ok {
		got, _ := os.ReadFile(path)
		t.Fatalf("the plant at %s does not read back as rewritten:\n%s", path, got)
	}
	if script != path {
		t.Fatalf("the reader named %s, not the plant at %s", script, path)
	}
	if interp != "/bin/sh" {
		t.Fatalf("the rewrite stored the interpreter line %q, not the plant's own", interp)
	}
	return nonce
}

// coldExecRunPlant execs argv with the injected delay set to d, and returns
// how long it took and what it printed. The delay is REPLACED rather than
// appended to, so a pin measures its own number under `make coldexec-check`
// (where the process already carries one) exactly as it does under an
// ordinary run.
func coldExecRunPlant(t *testing.T, d time.Duration, argv ...string) (time.Duration, string) {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = coldExecEnvWith(d)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("%v: %v\n%s", argv, err, out)
	}
	return elapsed, string(out)
}

// coldExecEnvWith is os.Environ with the delay variable set to d and any
// inherited copy of it removed, because a duplicate key is the platform's
// choice to make and not ours.
func coldExecEnvWith(d time.Duration) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, coldExecDelayEnv+"=") {
			env = append(env, kv)
		}
	}
	return append(env, coldExecDelayEnv+"="+d.String())
}

// ARM 1 — the whole claim, on a real kernel exec: the first exec of a
// planted script pays the injected delay, and the second does not.
//
// This is the test that would have caught ranger-base-aru4c's two sites by
// running them. Both were in bdwritelock_test.go, neither was named by the
// grep that went looking, and one of them bounded the first exec of a
// planted `bd` at 2s — BELOW the measured darwin worst case of 2.17s —
// reporting "a read verb waited on a write lock held by somebody else", the
// defect under test, over code that was fine.
func TestColdExecInjectionDelaysTheFirstExecAndOnlyTheFirst(t *testing.T) {
	t.Parallel()
	inj := coldExecTestInjector(t)
	path := filepath.Join(t.TempDir(), "plant.sh")
	nonce := coldExecPlant(t, inj, path, "#!/bin/sh\necho ran\n")

	// The control for every marker assertion below: it is absent until an
	// exec creates it, so its presence afterwards is evidence and not the
	// state the pin started in.
	if _, err := os.Stat(nonce); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the marker %s existed before any exec (%v) — nothing below measures the first-exec branch", nonce, err)
	}

	first, out := coldExecRunPlant(t, coldExecPinDelay, path)
	if out != "ran\n" {
		t.Errorf("the plant's body printed %q, not its own line — the shim did not reach the real interpreter", out)
	}
	if _, err := os.Stat(nonce); err != nil {
		t.Fatalf("the first exec created no marker, so the shim never took the first-exec branch: %v", err)
	}
	if first < coldExecPinFloor {
		t.Errorf("the first exec of a freshly planted script returned in %v under a %v injected delay — the cost is not being injected, so a test whose deadline cannot survive a cold exec would stay green",
			first, coldExecPinDelay)
	}

	second, out := coldExecRunPlant(t, coldExecPinDelay, path)
	if out != "ran\n" {
		t.Errorf("the re-exec printed %q, not the body's own line", out)
	}
	if second >= coldExecPinFloor {
		t.Errorf("a re-exec of the same path took %v, as long as the first — the delay is charged on every exec rather than once, which prices every warm re-exec in the suite at D",
			second)
	}
	t.Logf("first exec %v, re-exec %v, injected delay %v", first, second, coldExecPinDelay)
}

// ARM 2 — a rewrite is cold again. A fresh nonce per write is the whole of
// it (ADR 0085 D2), and it matters because a fixture that re-plants the same
// path between two timed calls is a fixture whose second plant is as cold as
// its first — on darwin it pays Gatekeeper again — and a gate that warmed
// the path once and then stopped looking would call it warm.
func TestColdExecRewriteMakesAPlantColdAgain(t *testing.T) {
	t.Parallel()
	inj := coldExecTestInjector(t)
	path := filepath.Join(t.TempDir(), "plant.sh")
	const body = "#!/bin/sh\necho ran\n"

	first := coldExecPlant(t, inj, path, body)
	if d, _ := coldExecRunPlant(t, coldExecPinDelay, path); d < coldExecPinFloor {
		t.Fatalf("the first exec returned in %v — arm 1's premise does not hold here, so nothing below is about the rewrite", d)
	}

	second := coldExecPlant(t, inj, path, body)
	if second == first {
		t.Fatalf("re-planting the same path reused the nonce %s, so the file the kernel now sees is one the shim has already charged for", second)
	}
	if _, err := os.Stat(second); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the new nonce %s already exists (%v)", second, err)
	}
	d, out := coldExecRunPlant(t, coldExecPinDelay, path)
	if out != "ran\n" {
		t.Errorf("the re-planted body printed %q", out)
	}
	if d < coldExecPinFloor {
		t.Errorf("the first exec of a RE-planted path returned in %v under a %v delay — a rewritten file is not cold again, and a fixture that re-plants between two timed calls is unmeasured",
			d, coldExecPinDelay)
	}
	if _, err := os.Stat(second); err != nil {
		t.Errorf("the exec after the rewrite created no new marker: %v", err)
	}
}

// ARM 3 — `sh <path>` is not an exec of <path>, so it is not delayed. This
// is ranger-base-aru4c's adjudication rule 2 turned into behaviour: the
// kernel execs /bin/sh, long warm, and the planted file is opened as data,
// so no first-exec assessment happens at all. watchpid_test.go's
// liveProcess is the site that rests on it, and ADR 0085's consequences
// name it as a test that must stay green under injection.
//
// The deterministic half is the marker: `sh <path>` must not create one,
// because the shim never ran. The shebang and the injected line are both
// `#` comments to the shell, which is why the body still runs.
func TestColdExecShimIsNotReachedByShRunningThePlant(t *testing.T) {
	t.Parallel()
	inj := coldExecTestInjector(t)
	path := filepath.Join(t.TempDir(), "plant.sh")
	nonce := coldExecPlant(t, inj, path, "#!/bin/sh\necho ran\n")

	elapsed, out := coldExecRunPlant(t, coldExecPinDelay, "/bin/sh", path)
	if out != "ran\n" {
		t.Errorf("`sh <plant>` printed %q, not the body's own line — the injected lines are not comments to the shell", out)
	}
	if _, err := os.Stat(nonce); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("`sh <plant>` created the first-exec marker %s (%v) — the shim ran, which it cannot have: the kernel exec'd /bin/sh", nonce, err)
	}
	if elapsed >= coldExecPinFloor {
		t.Errorf("`sh <plant>` took %v under a %v delay — it is being charged for a cold exec it never made", elapsed, coldExecPinDelay)
	}
}

// ARM 4 — the shim keeps the pid and passes the args through. ADR 0085 D2
// buys both with syscall.Exec rather than a child, and they are what make
// the injector invisible to everything except a clock: a test that reads a
// pid file, kills what it started or echoes `$$` names the same process it
// would have named with no injector in the way.
func TestColdExecShimKeepsThePidAndTheArgs(t *testing.T) {
	t.Parallel()
	inj := coldExecTestInjector(t)
	path := filepath.Join(t.TempDir(), "plant.sh")
	coldExecPlant(t, inj, path, "#!/bin/sh\necho \"$$ $*\"\n")

	cmd := exec.Command(path, "alpha", "beta")
	cmd.Env = coldExecEnvWith(time.Millisecond)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("exec %s: %v", path, err)
	}
	fields := strings.Fields(string(out))
	if len(fields) != 3 {
		t.Fatalf("the plant printed %q, want its pid and two arguments", out)
	}
	if want := strconv.Itoa(cmd.Process.Pid); fields[0] != want {
		t.Errorf("the body ran as pid %s and the parent started pid %s — the shim forked instead of exec'ing, so every pid a test reads through a plant is one process too shallow",
			fields[0], want)
	}
	if fields[1] != "alpha" || fields[2] != "beta" {
		t.Errorf("the plant was handed %v, not its own arguments", fields[1:])
	}
}

// ARM 5 — a plant with no `#!` is passed through unchanged and COUNTED
// (ADR 0085 D5). A copied binary cannot be rewritten, so the gate says
// nothing about its cold first exec; what it can do is make the gap a number
// the run prints. The count is also what tells a later measurement how big
// that gap is — D5 ASSUMED it small, and slice 2 measures it.
//
// The same arm holds the two cases that must NOT be counted: a script, and
// bytes that have already been rewritten.
func TestColdExecPassesThroughAndCountsANonScriptPlant(t *testing.T) {
	t.Parallel()
	inj := coldExecTestInjector(t)
	dir := t.TempDir()

	binary := []byte("\x7fELF\x02\x01\x01\x00\x00not a script\n")
	if got := inj.rewrite(filepath.Join(dir, "copied"), binary); string(got) != string(binary) {
		t.Errorf("a plant with no #! was rewritten, which would corrupt every copied binary a test plants:\n%q", got)
	}
	if n := inj.passedThrough.Load(); n != 1 {
		t.Errorf("the pass-through counter reads %d after one non-script plant, want 1 — the gap D5 leaves open would be a silence", n)
	}
	if n := inj.rewritten.Load(); n != 0 {
		t.Errorf("the rewrite counter reads %d after no rewritable plant, want 0", n)
	}

	script := []byte("#!/bin/sh\necho ran\n")
	once := inj.rewrite(filepath.Join(dir, "plant.sh"), script)
	if string(once) == string(script) {
		t.Fatal("a script plant was passed through unchanged")
	}
	if n := inj.passedThrough.Load(); n != 1 {
		t.Errorf("a rewritable plant was counted as a pass-through (counter now %d)", n)
	}
	if n := inj.rewritten.Load(); n != 1 {
		t.Errorf("the rewrite counter reads %d after one rewritten plant, want 1", n)
	}

	// Idempotence, which is why a pin may plant bytes it rewrote itself
	// through the one door and still get one rewrite on disk.
	if twice := inj.rewrite(filepath.Join(dir, "plant.sh"), once); string(twice) != string(once) {
		t.Errorf("already-rewritten bytes were rewritten again, burying one shim inside another:\n%q", twice)
	}
	if n := inj.passedThrough.Load(); n != 1 {
		t.Errorf("already-rewritten bytes were counted as a pass-through (counter now %d)", n)
	}
	if n := inj.rewritten.Load(); n != 1 {
		t.Errorf("already-rewritten bytes were counted as a second rewrite (counter now %d)", n)
	}
}

// ARM 6 — the WIRING, in a child process, because the subject is the package
// variable a parallel suite is reading. Three claims at once, and all three
// are ADR 0085 D3:
//
//   - the hook is consulted by WriteExecutable when the variable is set;
//   - it applies only when the IMMEDIATE caller is a _test.go file, so a
//     shim production code writes keeps its exact bytes — which is what
//     keeps a content read-back pin over a rendered gate, hook or probe
//     reading the product's bytes rather than the injector's;
//   - the run prints the pass-through count (D5).
//
// The child is this binary with POSSE_COLD_EXEC_DELAY set, run against one
// named test. A millisecond of delay: the question here is which bytes get
// written, and no exec of a plant happens at all.
func TestColdExecHookAppliesOnlyToATestCaller(t *testing.T) {
	t.Parallel()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe, "-test.run=^TestColdExecHookWiringChild$", "-test.v")
	// The child must run as a test binary and not as the fake substrate
	// TestMain turns it into whenever RHQ_FAKE_HERDR is set, and it must
	// carry our own delay rather than an inherited one.
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") && !strings.HasPrefix(kv, coldExecDelayEnv+"=") {
			env = append(env, kv)
		}
	}
	child.Env = append(env, coldExecDelayEnv+"=1ms", "RHQ_QA_COLDEXEC_CHILD=1")
	out, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("the child's own assertions failed (%v):\n%s", err, out)
	}
	if !strings.Contains(string(out), "--- PASS: TestColdExecHookWiringChild") {
		t.Fatalf("the child exited 0 without running the named test, so it asserted nothing:\n%s", out)
	}
	// D5's report line, with the one non-script plant the child makes from a
	// test caller. Read off the child's own output rather than reasoned
	// about, because an uninstalled injector prints nothing at all and that
	// is the failure worth telling apart from a count of zero.
	if !strings.Contains(string(out), "posse coldexec: 1 plants rewritten, 1 executable plants had no #!") {
		t.Errorf("the child did not report one rewrite and one pass-through — the run's own account of what it injected:\n%s", out)
	}
}

// The child half of ARM 6. It runs only when that pin asks for it: a test
// that re-execs this binary has to be able to name one test, and a child
// that also ran the parent would recurse.
func TestColdExecHookWiringChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_QA_COLDEXEC_CHILD") == "" {
		t.Skip("the child half of TestColdExecHookAppliesOnlyToATestCaller")
	}
	if coldExecRewrite == nil {
		t.Fatalf("%s is set and TestMain installed no hook", coldExecDelayEnv)
	}
	dir := t.TempDir()
	const body = "#!/bin/sh\necho ran\n"

	// A TEST caller: the bytes are rewritten.
	mine := filepath.Join(dir, "mine.sh")
	if err := WriteExecutable(mine, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(mine)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == body {
		t.Errorf("a plant written from a _test.go file kept its own shebang — the hook is installed and WriteExecutable is not consulting it")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(got), "\n")
	if len(lines) < 3 || lines[0] != "#!"+exe {
		t.Errorf("the rewritten shebang is %q, want #!%s", lines[0], exe)
	} else if !strings.HasPrefix(lines[1], coldExecMarkerPrefix) || !strings.HasSuffix(lines[1], " /bin/sh") {
		t.Errorf("the injected line is %q, want the marker, a nonce and the original interpreter line", lines[1])
	} else if strings.Join(lines[2:], "\n") != "echo ran\n" {
		t.Errorf("the body after the injected line is %q, not the plant's own", strings.Join(lines[2:], "\n"))
	}

	// A PRODUCTION caller, two frames of which the immediate one is
	// skills.go: copySkillFile routes an exec-bit copy through
	// WriteExecutable, so this is a real call site of the door and not a
	// stand-in for one. Its bytes must arrive byte for byte.
	src := filepath.Join(dir, "src.sh")
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	theirs := filepath.Join(dir, "theirs.sh")
	if err := copySkillFile(src, theirs, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(theirs); err != nil {
		t.Fatal(err)
	} else if string(got) != body {
		t.Errorf("copySkillFile's bytes were rewritten:\n%q\nwant\n%q\n"+
			"a shim production code writes must keep its exact bytes, or every content read-back pin over a rendered gate, hook or probe reds for the injector's reason and not its own", got, body)
	}

	// And one non-script plant from a test caller, so the parent has a
	// pass-through count to read that is not zero either way.
	if err := WriteExecutable(filepath.Join(dir, "copied"), []byte("\x7fELF not a script\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
