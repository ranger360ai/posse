package posse

// TestMain's temp HOME: one dir per test TREE, and mise's dirs left where the
// operator's are (ranger-base-32bdv).
//
// Both halves are about the same accident. TestMain replaces $HOME so that no
// test reaches the operator's live ~/.posse, and anything that resolves its
// own storage under $HOME follows it there. GOMODCACHE and GOCACHE were the
// first readers found doing it (ranger-base-6vvgs, which pinned them); mise
// is the second, and it costs more, because on this box `claude`, `codex`,
// `grok`, `gh` and `node` are mise command-wrappers rather than binaries.
//
// MEASURED 2026-11-11 on linux/amd64. /tmp here is a 14G tmpfs mounted
// usrquota and every seat shares uid posse, so it is one box-wide budget:
// 427 leaked temp homes, 11G of 14G spent, and main's corpus-check failing
// with "disk quota exceeded" for every seat. 417 of the 427 were the 4.0K
// .codex-only dir a re-exec CHILD leaves — and 5.25G of the bytes were in
// four of them, every byte under .cache/mise and
// .local/share/mise/installs/{claude,codex,grok}, because `mise ls
// --installed` under a temp HOME reports nothing installed and re-installs
// the toolchain before answering the wrapper.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The three dirs mise resolves under $HOME, and so the three TestMain pins.
// MISE_CONFIG_DIR is deliberately NOT here: configuration is an input to this
// binary and the temp HOME is what fences the operator's out.
var misePinnedDirs = []string{"MISE_DATA_DIR", "MISE_CACHE_DIR", "MISE_STATE_DIR"}

func TestMiseDirsArePinnedOutsideTheTempHome(t *testing.T) {
	t.Parallel()

	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("no $HOME — TestMain always sets one")
	}
	if operatorHome == "" {
		t.Skip("no operator home to pin against")
	}
	for _, key := range misePinnedDirs {
		v := os.Getenv(key)
		if v == "" {
			t.Errorf("%s is unset — a test that execs a mise command-wrapper (claude, codex, grok, gh, node) makes mise re-install the toolchain under the temp HOME, 2.3G in the worst leaked dir measured", key)
			continue
		}
		if v == home || strings.HasPrefix(v, home+string(os.PathSeparator)) {
			t.Errorf("%s = %q is under the temp HOME %q — this is the defect, not the fix", key, v, home)
		}
	}
	// And the fence the pin must not have broken: the operator's config is
	// still out of reach, so mise installs nothing on its own account.
	if v := os.Getenv("MISE_CONFIG_DIR"); v != "" && !strings.HasPrefix(v, home) {
		t.Errorf("MISE_CONFIG_DIR = %q points outside the temp HOME — mise's user config is an INPUT to this binary and the temp HOME is what keeps it out", v)
	}
}

// TestTempHomeChild is the child arm of the pin below. It reports the $HOME it
// was started with, which is the whole measurement: a child that makes its
// own prints a different dir, and leaves it behind when the parent kills it.
func TestTempHomeChild(t *testing.T) {
	if os.Getenv("RHQ_QA_TESTHOME_CHILD") == "" {
		t.Skip("child of TestTheTempHomeIsOnePerTestTree")
	}
	os.Stdout.WriteString("CHILDHOME " + os.Getenv("HOME") + "\n")
	// …and the codex credential the CREATOR planted, which an adopting child
	// no longer writes for itself. Reported as bytes rather than as a verdict
	// so the parent is the one that decides: an empty read is the truncation
	// race the guard exists to prevent, and it must not look like an absence.
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".codex", "auth.json"))
	if err != nil {
		os.Stdout.WriteString("CHILDCRED error " + err.Error() + "\n")
		return
	}
	os.Stdout.WriteString("CHILDCRED bytes " + strconv.Itoa(len(b)) + "\n")
}

// A re-exec child adopts the parent's temp HOME instead of cutting a second
// one. This is the pin for the 417: the removal in TestMain is reached only
// when m.Run RETURNS, and the fifteen sites that start a child kill it long
// before that, so every home a child made was a home nobody removed.
func TestTheTempHomeIsOnePerTestTree(t *testing.T) {
	t.Parallel()

	home := os.Getenv("HOME")
	if got := os.Getenv(envTestHome); got != home {
		t.Fatalf("%s = %q, want the temp HOME %q — a child reads this to adopt, and adopts nothing when it is wrong", envTestHome, got, home)
	}
	if got := os.Getenv(envTestOperatorHome); got != operatorHome {
		t.Errorf("%s = %q, want the operator's %q — a child-run test that reads operatorHome otherwise measures its parent's tempdir", envTestOperatorHome, got, operatorHome)
	}

	t.Logf("this binary's temp HOME: %s", home)
	started := time.Now()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe, "-test.run=^TestTempHomeChild$", "-test.v")
	// RHQ_FAKE_HERDR taken out, or TestMain answers as the fake substrate and
	// the child never runs the test (reexecenv_qa_test.go is the census).
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") {
			env = append(env, kv)
		}
	}
	child.Env = append(env, "RHQ_QA_TESTHOME_CHILD=1")
	out, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}

	var reported string
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "CHILDHOME "); ok {
			reported = rest
		}
	}
	if reported == "" {
		t.Fatalf("the child never reported its HOME:\n%s", out)
	}
	if reported != home {
		t.Errorf("the child ran under HOME=%q, want the parent's %q — it cut its own, and a parent that kills it leaves that dir on a tmpfs every seat shares", reported, home)
	}

	// The fixture the creator plants and an adopter does not: a child must
	// still FIND it, or guarding that write on ownHome traded a truncation
	// race for codexSigninProbe reading "cannot authenticate" in every child.
	want := "CHILDCRED bytes " + strconv.Itoa(len(`{}`))
	if !strings.Contains(string(out), want) {
		var got string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "CHILDCRED ") {
				got = strings.TrimSpace(line)
			}
		}
		t.Errorf("the child read the codex credential as %q, want %q — the creator plants it and an adopting child does not, so this is the whole of what makes that safe", got, want)
	}
	// And it created nothing of its own beside ours.
	siblings, _ := filepath.Glob(filepath.Join(filepath.Dir(home), "posse-testhome-*"))
	for _, s := range siblings {
		if s == home {
			continue
		}
		if fi, serr := os.Stat(s); serr == nil && fi.ModTime().After(started) {
			t.Errorf("%s appeared during this test — something still cuts a second temp HOME", s)
		}
	}
}

// ─── ADR 0089: the creator's lock, and the reaper that reads it ──────────────
//
// WHAT THE PIN BELOW PROTECTS, stated as ADR 0006 §7 asks: an operator typing
// `scripts/reap-test-tmp.sh --apply` while suites are in flight, and that
// sweep removing the temp HOME of a run that is still using it. MEASURED
// 2026-11-11 before this bead (ADR 0089, from ranger-base-glogz): a live
// probe of TestMain's exact shape, with its dir's mtime set back two hours,
// was reported `live temp HOMEs seen: 0` and `collectable: 1 dirs`. Nothing
// about that run looked wrong, which is the whole problem — the dir's own
// creator was invisible to every gate the reaper had, because os.Setenv does
// not rewrite /proc/<pid>/environ and the process held no fd on the dir.
//
// Two pins, in the two places the mechanism can break independently:
//
//   - TestTheCreatorHoldsAFlockOnItsTempHome asks only whether THIS process
//     is holding the lock. It is the D1 half, it needs no script and no
//     child, and it is the one that reds if the fd is ever dropped (a local
//     instead of the package var, a stray Close, a refactor that moves the
//     flock after a return path).
//   - TestTheReaperKeepsALiveTempHomeAndTakesADeadOne is the D1+D2 round
//     trip: a real creator child, a real mtime past the grace, and the real
//     script's real verdict on both sides of a kill -9.

// qrtReapScript returns scripts/reap-test-tmp.sh, or "" when this tree does
// not hold it.
//
// THE ROOT COMES THROUGH THIS HELPER RATHER THAN OUT OF THE PIN'S OWN BODY,
// and that is a classification and not a style choice. The tree-wide class in
// internal/treepins/treewidedoor_qa_test.go is derived from "the test's own
// body calls qibRepoRoot", and every member of it needs a Makefile door
// because its subject is the TREE: any bead that adds a file anywhere can red
// it. This pin's subject is ONE script and ONE TestMain, and no file added
// anywhere else can touch it, so it is not in that class. The shape that says
// so is qspSeedScript's, which that file's own comment names as the
// precedent: "qspSeedScript looks up one path at the root and its callers red
// only when that script changes, which is not this class".
func qrtReapScript(t *testing.T) string {
	t.Helper()
	p := filepath.Join(qibRepoRoot(t), "scripts", "reap-test-tmp.sh")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// qrtFlockProbe asks flock(1) the one question the reaper asks: can an
// exclusive, non-blocking lock be taken on this directory right now. It
// returns 0 when the lock was free, 9 when somebody is holding it, and
// anything else when flock could not even open the dir — the same three
// answers, spelled the same way, as the script's own flock_filter.
func qrtFlockProbe(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("flock", "-n", "-E", "9", dir, "true")
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		t.Fatalf("flock -n %s: %v", dir, err)
	}
	return 0
}

// qrtSkipUnlessReapable skips when this platform or this cage cannot run the
// reaper at all. The script reads /proc and refuses without it, and ADR 0089
// D3 leaves it linux-only; flock(1) is util-linux and the script refuses
// without that too (exit 2, never a silent fall-back to the two gates ADR
// 0089 measured collecting a live run's home).
func qrtSkipUnlessReapable(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("scripts/reap-test-tmp.sh reads /proc and is linux-only (ADR 0089 D3); this is %s", runtime.GOOS)
	}
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skipf("no flock(1) on PATH in this cage: %v — the script itself refuses with exit 2 here, which is the designed answer", err)
	}
}

// D1, asked of the process doing the asking. The creator of a temp HOME holds
// an exclusive flock on it for its whole life, so the kernel's release of
// that lock is this run's death certificate and nothing has to be inferred
// from a pid, a stamp or an mtime (ADR 0011 §1's argument, one directory
// over).
//
// SKIPPED IN AN ADOPTING CHILD, which is the ownHome guard read from the
// outside: a re-exec child shares its parent's home and must not lock it, or
// the parent's own lock would be the thing it collided with.
//
// THE SKIP IS KEYED ON testHomeOwned AND NOT ON testHomeLock, and that is the
// difference between a pin and a decoration. testHomeLock is nil for an
// adopting child AND for a creator whose flock failed, so a skip keyed on it
// reads "correctly took no lock" over a process that was supposed to have one
// — MEASURED under ranger-base-z056i by deleting TestMain's lockTestHome
// call, which left this test SKIPPING green while the round trip below reddened.
func TestTheCreatorHoldsAFlockOnItsTempHome(t *testing.T) {
	t.Parallel()
	qrtSkipUnlessReapable(t)

	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("no $HOME — TestMain always sets one")
	}
	if !testHomeOwned {
		t.Skip("this process adopted its parent's temp HOME and correctly takes no lock on it")
	}

	// From OUTSIDE the process, because that is the only vantage the reaper
	// ever has. A syscall.Flock on our own second fd would succeed: flock
	// locks are per open file description and the same process re-locking is
	// not a conflict, so an in-process probe would pass over a lock nobody
	// else can see.
	if got := qrtFlockProbe(t, home); got != 9 {
		why := ""
		if testHomeLock == nil {
			why = " TestMain prints a 'temp HOME … NOT locked' line when the flock itself failed — look for it above; if there is none, the lockTestHome call is gone."
		}
		t.Errorf("flock -n on the temp HOME %s answered %d, want 9 (busy) — ADR 0089 D1's lock is not held, so scripts/reap-test-tmp.sh has nothing to read and this run's dir is protected by its mtime alone. A 60m-timeout arm on a loaded box outlives that grace.%s", home, got, why)
	}
}

// TestTempHomeLockChild is the creator arm of the round trip below. It reports
// the temp HOME it made through a file the parent names, then sits still until
// it is SIGKILLed — which is the shape the whole mechanism exists for, because
// neither a signal handler nor a deferred RemoveAll can run after one.
//
// It reports the home through a file rather than through stdout because the
// parent has to read it WHILE this process is still alive, and reading a pipe
// that the child is still writing is the one thing a *bytes.Buffer shared with
// a running exec.Cmd must not do.
func TestTempHomeLockChild(t *testing.T) {
	ready := os.Getenv("RHQ_QA_TESTHOME_LOCK_READY")
	if ready == "" {
		t.Skip("child of TestTheReaperKeepsALiveTempHomeAndTakesADeadOne")
	}
	// "NOLOCK" rather than silence, so a parent whose child reached this
	// point without a lock gets told that instead of a timeout — the two
	// have completely different causes and only one of them is this bead's.
	report := os.Getenv("HOME")
	if testHomeLock == nil {
		report = "NOLOCK " + report
	}
	if err := os.WriteFile(ready, []byte(report), 0o644); err != nil {
		t.Fatalf("reporting the temp HOME: %v", err)
	}
	// Long enough that a loaded box cannot outrun the parent's two reaper
	// runs, short enough to be a bounded leak if the parent dies without
	// killing us (it kills us in a defer).
	time.Sleep(2 * time.Minute)
}

// D1 and D2 together, end to end, against the real script: the reaper keeps a
// live run's temp HOME and takes the same dir once its creator is gone.
//
// WHY THE MTIME IS MOVED BACK. The grace is the gate that used to carry all
// the weight, and a dir minutes old is kept by it whatever the lock says — so
// a pin that did not age the dir would pass identically with D1 deleted. Three
// hours back puts it past every grace this class has ever had, which leaves
// the lock as the only thing that can still keep it.
func TestTheReaperKeepsALiveTempHomeAndTakesADeadOne(t *testing.T) {
	t.Parallel()
	qrtSkipUnlessReapable(t)
	script := qrtReapScript(t)
	if script == "" {
		t.Skip("this tree holds no scripts/reap-test-tmp.sh")
	}

	// The child's TMPDIR and the reaper's --root are the same dir, so the
	// candidate set is exactly one dir and every count below is exact.
	// Resolved, because MkdirTemp's answer is and the script compares paths
	// as text.
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	ready := filepath.Join(t.TempDir(), "childhome")
	childLog := filepath.Join(t.TempDir(), "child.log")

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	logf, err := os.Create(childLog)
	if err != nil {
		t.Fatal(err)
	}
	defer logf.Close()

	child := exec.Command(exe, "-test.run=^TestTempHomeLockChild$", "-test.v")
	// Three variables out of the child's environment, each for its own
	// reason, and the loop takes them OUT rather than listing what goes in
	// because the child needs the rest of this binary's world (PATH,
	// GOMODCACHE, the MISE_*_DIRs) to start at all:
	//   - RHQ_FAKE_HERDR, or TestMain answers as the fake substrate and the
	//     child never runs a test (reexecenv_qa_test.go is the census);
	//   - RHQ_TEST_HOME, or the child ADOPTS our home, creates nothing and
	//     takes no lock — it is the whole point that this child is a CREATOR;
	//   - TMPDIR, replaced below, so os.MkdirTemp lands under the one root
	//     the reaper is pointed at.
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") ||
			strings.HasPrefix(kv, envTestHome+"=") ||
			strings.HasPrefix(kv, "TMPDIR=") {
			continue
		}
		env = append(env, kv)
	}
	child.Env = append(env, "TMPDIR="+root, "RHQ_QA_TESTHOME_LOCK_READY="+ready)
	child.Stdout, child.Stderr = logf, logf
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	defer func() {
		if !reaped {
			_ = child.Process.Kill()
			_, _ = child.Process.Wait()
		}
	}()

	var childHome string
	deadline := time.Now().Add(90 * time.Second)
	for childHome == "" {
		if b, rerr := os.ReadFile(ready); rerr == nil && len(b) > 0 {
			childHome = strings.TrimSpace(string(b))
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the creator child never reported a temp HOME in 90s\n%s", qrtRead(childLog))
		}
		time.Sleep(100 * time.Millisecond)
	}
	if rest, bad := strings.CutPrefix(childHome, "NOLOCK "); bad {
		t.Fatalf("the creator child made %s and did not lock it — ADR 0089 D1 is not in effect, and TestMain's 'NOT locked' line says why:\n%s", rest, qrtRead(childLog))
	}
	if filepath.Dir(childHome) != root {
		t.Fatalf("the child's temp HOME %s is not under the root %s this test points the reaper at — the TMPDIR handover did not take, and every count below would be measuring the wrong dir", childHome, root)
	}
	t.Logf("creator child pid %d, temp HOME %s", child.Process.Pid, childHome)

	// Past every grace this class has ever carried, so the lock is the only
	// gate left that can keep the dir.
	past := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(childHome, past, past); err != nil {
		t.Fatal(err)
	}

	if got := qrtFlockProbe(t, childHome); got != 9 {
		t.Errorf("flock -n on the live child's temp HOME %s answered %d, want 9 (busy)", childHome, got)
	}

	// ─── while the creator lives ───
	out := qrtReap(t, script, root)
	lockLine := "  HELD (lock) " + childHome
	if !strings.Contains(out, lockLine) {
		t.Errorf("a dry run over a LIVE creator's temp HOME did not report it held by the lock.\nwant a line: %s\ngot:\n%s", lockLine, out)
	}
	if !strings.Contains(out, "): 0 collectable") {
		t.Errorf("a dry run over a LIVE creator's temp HOME called it collectable — this is the bug ADR 0089 was written for, and an operator's --apply would have removed a running suite's $HOME.\n%s", out)
	}
	// THE SCAN SEES IT TOO NOW, and the reason is worth saying because it is
	// not the one the ADR expected. MEASURED 2026-11-11 under this bead: an
	// flock holder's lock fd appears in its own /proc/<pid>/fd, so D1's open
	// directory fd is picked up by the reaper's existing fd arm as well —
	// which is the "keep an fd open so the fd scan sees the creator"
	// alternative ADR 0089 rejected, arriving for free beside the lock it
	// chose. The environ arm is still blind (os.Setenv does not rewrite
	// /proc/<pid>/environ) and the fd arm is still conditional on a readable
	// /proc, which is exactly why the lock and not the fd is the decision.
	//
	// Logged and not asserted, in BOTH directions: a cage that masks /proc
	// takes the second signal away and must not red a pin whose subject is
	// the first one.
	if strings.Contains(out, "\n  HELD "+childHome) {
		t.Logf("the /proc fd arm saw the dir as well, through D1's own lock fd — a second, /proc-dependent signal beside the lock")
	} else {
		t.Logf("the /proc scan did not see the dir at all, as ADR 0089 measured: the lock is doing all of the work here")
	}

	// ─── and once it is gone ───
	if err := child.Process.Kill(); err != nil {
		t.Fatalf("kill -9 the creator child: %v", err)
	}
	_, _ = child.Process.Wait()
	reaped = true

	// The kernel releases the lock at death, but "at death" is not "by the
	// time Wait returned" on every box, so poll for it rather than sleep a
	// number somebody guessed.
	deadline = time.Now().Add(15 * time.Second)
	for {
		if qrtFlockProbe(t, childHome) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the lock on %s was still held 15s after kill -9 — the release IS the death certificate, so a lock that outlives its holder is the one failure this mechanism cannot have", childHome)
		}
		time.Sleep(100 * time.Millisecond)
	}

	out = qrtReap(t, script, root)
	if strings.Contains(out, "HELD (lock) "+childHome) {
		t.Errorf("the dead creator's temp HOME is still reported held by a lock — a lock that survives its holder is a stale lock, and the mechanism would be a pidfile with extra steps.\n%s", out)
	}
	if !strings.Contains(out, "): 1 collectable") {
		t.Errorf("a dry run after kill -9 did not offer the abandoned temp HOME %s for collection — the reaper now keeps dirs it should take, which is a 14G tmpfs every seat shares filling up quietly.\n%s", childHome, out)
	}
}

// qrtReap runs the reaper as a dry run over one root and one class.
//
// A REFUSAL IS NOT A VERDICT, and the two the script has are not failures of
// this pin: exit 2 is a missing /proc or flock(1), exit 3 is a /proc whose
// environs this uid cannot open, and in either case the script says so and
// deletes nothing — which is the designed behaviour and the opposite of the
// defect under test. Those SKIP, naming the refusal. Every other non-zero
// exit is a broken script and fatal.
func qrtReap(t *testing.T, script, root string) string {
	t.Helper()
	cmd := exec.Command("bash", script, "--root", root, "--class", "posse-testhome", "--min-age", "60")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && (ee.ExitCode() == 2 || ee.ExitCode() == 3) {
		t.Skipf("the reaper refused in this cage (exit %d) and collected nothing, which is its designed answer rather than a verdict this pin can read:\n%s", ee.ExitCode(), out)
	}
	t.Fatalf("%s --root %s: %v\n%s", script, root, err, out)
	return ""
}

func qrtRead(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "(" + err.Error() + ")"
	}
	return string(b)
}
