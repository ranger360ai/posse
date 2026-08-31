package posse

// QA pins for the launcher lock (ADR 0011 §1, rangerhq-tzdf, verified
// under rangerhq-s7qz).
//
// The in-package launchlock_test.go proves the lock in one process, on two
// open file descriptions — the right claim for flock, and still a claim. These
// cross the process boundary, exercise the failure path, and hold the two
// halves of the acceptance criterion the in-process tests do not reach.
//
// Self-contained on purpose (own writer, own helpers): they must survive
// whatever the next persona does to launchlock_test.go.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// qaBuf is an io.Writer a test goroutine may read while the code under test
// is still writing to it.
type qaBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (q *qaBuf) Write(p []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.b.Write(p)
}

func (q *qaBuf) String() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.b.String()
}

func qaWaitFor(t *testing.T, b *qaBuf, want string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(b.String(), want) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("output never contained %q:\n%s", want, b.String())
}

func qaHerdrCalls(t *testing.T, fake string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(fake, "calls.log"))
	return string(b)
}

// qaOneBeadRepo is one persona, one repo, one bead — the shape that makes
// two passes contend for the same work rather than for different work.
func qaOneBeadRepo(t *testing.T, a *App) string {
	t.Helper()
	writePersona(t, a, "alpha", "[go]")
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	return repo
}

// ─── the lock across a real process boundary ─────────────────────────────────

// The child half of TestLaunchLockHoldsAcrossProcesses: take the launcher
// lock of the RHQ_HOME handed to it, say so, and hold until killed.
func TestLaunchLockChildHolder(t *testing.T) {
	home := os.Getenv("RHQ_QA_HOLD_HOME")
	if home == "" {
		t.Skip("child of TestLaunchLockHoldsAcrossProcesses")
	}
	l, err := lockLaunches(NewApp(), os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	os.Stdout.WriteString("HELD " + strconv.Itoa(os.Getpid()) + "\n")
	time.Sleep(2 * time.Minute)
}

// Two real processes, not two file descriptions: a foreign holder blocks
// this one, the waiting line names the foreign pid, and killing the holder
// frees the lock with nothing to reap — the whole argument for flock over a
// second pidfile (rangerhq-ct9/ppy9), tested rather than reasoned.
func TestLaunchLockHoldsAcrossProcesses(t *testing.T) {
	b, _ := newTestBackend(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	child := exec.Command(exe, "-test.run=TestLaunchLockChildHolder", "-test.v")
	// The child must run as a test binary, not as the fake substrate TestMain
	// turns it into whenever RHQ_FAKE_HERDR is set.
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") && !strings.HasPrefix(kv, "RHQ_HOME=") {
			env = append(env, kv)
		}
	}
	child.Env = append(env, "RHQ_QA_HOLD_HOME="+b.App.Home, "RHQ_HOME="+b.App.Home)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { child.Process.Kill(); child.Wait() }()

	buf, seen := make([]byte, 4096), ""
	deadline := time.Now().Add(60 * time.Second)
	for !strings.Contains(seen, "HELD ") && time.Now().Before(deadline) {
		n, err := stdout.Read(buf)
		seen += string(buf[:n])
		if err != nil {
			break
		}
	}
	if !strings.Contains(seen, "HELD ") {
		t.Fatalf("the holding process never took the lock:\n%s", seen)
	}
	childPid := strings.Fields(seen[strings.Index(seen, "HELD "):])[1]

	var out qaBuf
	got := make(chan error, 1)
	go func() {
		l, err := lockLaunches(b.App, &out)
		if l != nil {
			l.Release()
		}
		got <- err
	}()

	select {
	case err := <-got:
		t.Fatalf("took the lock while another process held it (err=%v)", err)
	case <-time.After(2 * time.Second):
	}
	if !strings.Contains(out.String(), "waiting") {
		t.Errorf("no waiting line while a foreign process held the lock: %q", out.String())
	}
	if want := "pid " + childPid; !strings.Contains(out.String(), want) {
		t.Errorf("waiting line does not name the holding process (%s): %q", want, out.String())
	}

	// Release *is* process death. No grace, no staleness, nothing to reap.
	child.Process.Kill()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the lock was not released when the holding process died")
	}
}

// ─── the failure path ────────────────────────────────────────────────────────

// A launcher that cannot take the lock must not launch unserialized: the
// lock's failure is the pass's failure, never a warning line it walks past.
func TestLaunchLockFailureFailsThePass(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	qaOneBeadRepo(t, b.App)

	// A directory at the lock path: O_RDWR on it cannot succeed.
	if err := os.MkdirAll(LaunchLockPath(b.App), 0o755); err != nil {
		t.Fatal(err)
	}

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err == nil {
		t.Errorf("pass succeeded with no lock (dispatched %d): %s", n, dispatcherOut(d))
	}
	if strings.Contains(qaHerdrCalls(t, fake), "workspace create") {
		t.Errorf("a session was created without the lock:\n%s", qaHerdrCalls(t, fake))
	}
}

// ─── --watch is a launcher that sleeps ───────────────────────────────────────

// The loop must drop the lock between passes. Holding it across the backoff
// would make a hand-run `posse dispatch` wait out the whole interval — the
// bound ADR 0011 §1 states is -n × (create + StartupWait), not the loop's.
func TestWatchReleasesLockBetweenPasses(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := t.TempDir()
	// Unroutable: the pass runs to completion without launching anything.
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["nobody"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	d := newTestDispatcher(t, b)
	var out qaBuf
	d.Out = &out
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 30*time.Second, 30*time.Second); done <- p }()

	qaWaitFor(t, &out, "next pass in") // the loop is in its backoff now
	free := make(chan error, 1)
	go func() {
		l, err := lockLaunches(b.App, &qaBuf{})
		if l != nil {
			l.Release()
		}
		free <- err
	}()
	select {
	case err := <-free:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Error("--watch holds the launcher lock across its backoff")
	}
	cancel()
	<-done
}

// ─── the half of the acceptance criterion the lock does not close ────────────

// tzdf's acceptance criterion is "no double-claims, no interleaved fire
// loops". The lock closed the second half — and the duplicate `workspace
// create` that was rangerhq-9nso's damage — but not the first: `Run` reads
// `bd ready` BEFORE fireLoop locks, so the waiting pass fires from a list
// the holder already consumed. Inside the lock its guards then abstain in
// order — `busy` is per-pass, `personaActive` sees the fresh agent as idle
// rather than working, and the in_progress guard reads the stale row, which
// still says open. `lastPrompt` is per-process, so PromptGrace never runs.
//
// Measured on 30b67b3^ (no lock): creates=2 — two workspaces, one session
// name. On 30b67b3: creates=1, prompts=2, claims=2.
//
// ADR 0011 §3 owns the fix (persisted `prompted:` in the meta, read by
// PromptGrace across processes); repro on that bead.
func TestTwoPassesDoNotDoubleClaimOneBead(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	qaOneBeadRepo(t, b.App)

	held := mustHoldLock(t, b.App)
	outs := []*qaBuf{{}, {}}
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		d := newTestDispatcher(t, b)
		d.Out = outs[i]
		go func() { d.Run("", "", 0); done <- struct{}{} }()
	}
	qaWaitFor(t, outs[0], "waiting")
	qaWaitFor(t, outs[1], "waiting")
	held.Release()
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(90 * time.Second):
			t.Fatalf("a pass never finished:\n0:\n%s\n1:\n%s", outs[0], outs[1])
		}
	}

	log := qaHerdrCalls(t, fake)
	if n := strings.Count(log, "workspace create "); n != 1 {
		t.Errorf("one bead, %d sessions created (want 1)", n)
	}
	if n := strings.Count(log, "agent prompt "); n != 1 {
		t.Errorf("one bead prompted %d times by two serialized passes (want 1)\n0:\n%s\n1:\n%s",
			n, outs[0], outs[1])
	}
	if n := strings.Count(bdCalls(t, fake), "--claim"); n != 1 {
		t.Errorf("one bead claimed %d times (want 1)", n)
	}
}

// ─── what the lock does not cover: Run acts before it locks ──────────────────

// `Run` calls VerifyAfter — which files beads — before fireLoop takes the
// lock, and VerifyAfter's dedupe is two check-then-act pairs (the watermark,
// then `Dependents` before `Create`). Two passes that start together both
// read the pre-pass watermark and both file. Sequentially the second files
// nothing, which is the control this asserts first.
//
// Fixed under rangerhq-th7l: VerifyAfter takes the launcher lock itself, so
// the loser reads the watermark the winner already advanced. Called directly
// rather than through Run on purpose — `posse ready` files by the same rule and
// reaches the same code with no fire loop behind it.
func TestVerifyAfterDoesNotDoubleFileUnderConcurrentPasses(t *testing.T) {
	seed := closedList("c-0", `["code"]`, time.Now().Add(-time.Hour).Format(time.RFC3339))
	fresh := closedList("c-1", `["code"]`, time.Now().Add(-time.Minute).Format(time.RFC3339))

	b, _ := newTestBackend(t)
	repo := vaRepo(t, b.App, seed)
	bd := testBd(t)
	vaRun(t, b.App, bd) // first sight of the repo seeds the watermark
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(fresh), 0o644)

	var wg sync.WaitGroup
	filed := make([]int, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			var out, errb strings.Builder
			filed[i] = b.App.VerifyAfter(bd, b.App.BeadsDirs(), &out, &errb)
		}(i)
	}
	close(start)
	wg.Wait()

	if total := filed[0] + filed[1]; total != 1 {
		t.Errorf("two concurrent passes filed %d verify beads for one close (want 1): %v", total, filed)
	}
}

// ─── "for its whole body" is the claim, and it was unpinned ──────────────────

// ADR 0011 §1 gives LaunchBead the lock "for its whole body": the cockpit's
// guards — crew-held, the holder join, working/blocked — and the launch they
// authorize are one critical section. Every pin that existed for it asserts
// only that a blocked `d` creates no session, and a lock taken *after* the
// guards satisfies that too: moving lockLaunches down to launchSession leaves
// `go test ./...` entirely green (measured, rangerhq-s7qz).
//
// What separates the two placements is what the guards touch. crewHeld and
// the SessionForBead/SessionFor Resolve pair are herdr reads, so the claim
// with teeth is that a launcher still waiting for the lock has read nothing
// yet — its guards have not run, so there is no reading for another launcher
// to invalidate before it acts.
func TestLaunchBeadLocksBeforeItsGuardsRead(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var out qaBuf
	d.Out = &out
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}

	held := mustHoldLock(t, b.App)
	defer held.Release()
	before := qaHerdrCalls(t, fake)

	done := make(chan error, 1)
	go func() { _, err := d.LaunchBead(is); done <- err }()

	qaWaitFor(t, &out, "waiting")
	// The waiting line is printed before the blocking flock, so give a
	// guard that runs after it its chance to show up too.
	time.Sleep(200 * time.Millisecond)
	if got := strings.TrimSpace(strings.TrimPrefix(qaHerdrCalls(t, fake), before)); got != "" {
		t.Errorf("a launcher still waiting for the lock had already read herdr:\n%s", got)
	}

	held.Release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the cockpit launch never resumed after the lock was released")
	}
}
