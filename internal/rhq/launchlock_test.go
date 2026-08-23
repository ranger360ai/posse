package rhq

// The launcher lock (ADR 0011 §1, rangerhq-tzdf). Real flock in a temp
// RHQ_HOME — no fake: flock is per open file description, so two Open+Flock
// pairs inside one test process contend exactly as two processes do, and a
// faked lock would only test the fake.
//
// Every test here starts by taking the lock in the test goroutine and
// releasing it on cue, so contention is arranged rather than raced for: the
// launcher under test is provably blocked before anything is released, and
// nothing depends on which goroutine the scheduler runs first.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuf is an io.Writer a test goroutine may read while the code under
// test is still writing to it.
type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitForOut blocks until the writer has said want. The budget is generous
// on purpose: every herdr call here forks the test binary, and under -race
// one launch is tens of seconds.
func waitForOut(t *testing.T, buf *syncBuf, want string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), want) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("output never contained %q:\n%s", want, buf.String())
}

// mustHoldLock takes the launcher lock for the test itself — the other
// launcher every test here needs.
func mustHoldLock(t *testing.T, a *App) *LaunchLock {
	t.Helper()
	l, err := lockLaunches(a, &syncBuf{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Release)
	return l
}

// rawCalls is the fake herdr's log without calls()'s gate-prefix assertion:
// these tests read it while a pass is mid-flight, when a partial line is
// normal and is not evidence of a missing wall.
func rawCalls(t *testing.T, fake string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(fake, "calls.log"))
	return string(b)
}

// ─── the lock itself ─────────────────────────────────────────────────────────

// The core of §1: the second launcher waits, and says whose pid it is
// waiting on before it does — a dispatch stopped for a reason must not read
// as a dispatch that hung.
func TestLaunchLockSecondLauncherWaits(t *testing.T) {
	b, _ := newTestBackend(t)
	held := mustHoldLock(t, b.App)

	type got struct {
		out string
		err error
	}
	done := make(chan got, 1)
	go func() {
		var buf syncBuf
		l, err := lockLaunches(b.App, &buf)
		l.Release()
		done <- got{buf.String(), err}
	}()

	select {
	case g := <-done:
		t.Fatalf("second launcher did not wait for the holder: %+v", g)
	case <-time.After(150 * time.Millisecond):
	}

	held.Release()
	select {
	case g := <-done:
		if g.err != nil {
			t.Fatal(g.err)
		}
		if !strings.Contains(g.out, "waiting") {
			t.Errorf("no waiting line:\n%s", g.out)
		}
		if want := "pid " + strconv.Itoa(os.Getpid()); !strings.Contains(g.out, want) {
			t.Errorf("waiting line does not name the holder (%s):\n%s", want, g.out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("second launcher never took the released lock")
	}
}

// A released lock is free at once and silently: the file is left on disk on
// purpose (unlinking it would let the next launcher lock a fresh inode), so
// the path outliving the holder must not look like a held lock.
func TestLaunchLockFreeAfterRelease(t *testing.T) {
	b, _ := newTestBackend(t)
	first, err := lockLaunches(b.App, &syncBuf{})
	if err != nil {
		t.Fatal(err)
	}
	first.Release()
	first.Release() // idempotent: callers both defer it and drop it early

	if _, err := os.Stat(LaunchLockPath(b.App)); err != nil {
		t.Fatalf("the lock file must survive its holder: %v", err)
	}
	var buf syncBuf
	second, err := lockLaunches(b.App, &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if strings.Contains(buf.String(), "waiting") {
		t.Errorf("a free lock made the next launcher wait:\n%s", buf.String())
	}
}

// The holder line is a courtesy, and courtesies must not fail the pass: a
// lock file with no readable pid still serializes, it just cannot name a
// number, and a number that names nobody is worse than none.
func TestLaunchLockHolderUnknown(t *testing.T) {
	b, _ := newTestBackend(t)
	path := LaunchLockPath(b.App)
	os.MkdirAll(filepath.Dir(path), 0o755)

	for _, tc := range []struct{ name, body string }{
		{"empty", ""},
		{"not yaml", "garbage\n"},
		{"dead pid", "pid: 2147483646\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.WriteFile(path, []byte(tc.body), 0o644)
			if h := lockHolder(path); h != "another launcher" {
				t.Errorf("holder %q, want the generic phrasing", h)
			}
		})
	}
}

// ─── Run: the fire loop is locked, the gather is not ─────────────────────────

// A pass whose launcher lock is held launches nothing until it is free —
// and says so. This is the 9nso window closed from the launcher side.
func TestDispatchFireLoopWaitsForTheLock(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var out syncBuf
	d.Out = &out
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")

	held := mustHoldLock(t, b.App)

	done := make(chan int, 1)
	go func() {
		n, err := d.Run("", "", 0)
		if err != nil {
			t.Error(err)
		}
		done <- n
	}()

	waitForOut(t, &out, "waiting")
	// Blocked before the first launch, not between them: the guards and the
	// launch they authorize are one critical section.
	if log := rawCalls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a blocked pass created a session:\n%s", log)
	}
	select {
	case n := <-done:
		t.Fatalf("the pass ran while the lock was held (%d dispatched)", n)
	case <-time.After(100 * time.Millisecond):
	}

	held.Release()
	select {
	case n := <-done:
		if n != 1 {
			t.Errorf("dispatched %d, want 1 once the lock was free:\n%s", n, out.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the pass never resumed after the lock was released")
	}
	if log := rawCalls(t, fake); !strings.Contains(log, "workspace create") {
		t.Errorf("no session created after the lock was released:\n%s", log)
	}
}

// The gather only reads and judges, so it must not hold the queue: the ADR
// bounds hold time by the launches, not by how long the beads run. Proven by
// taking the lock while a pass is provably still gathering.
func TestDispatchGatherRunsUnlocked(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var out syncBuf
	d.Out = &out
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	// The prompt settles slowly, so the gather is still in flight when the
	// test asks for the lock — and the lock is a syscall, so the window it
	// needs is microseconds after the gathering line appears.
	os.WriteFile(filepath.Join(fake, "prompt-delay-ms"), []byte("5000"), 0o644)

	done := make(chan struct{})
	go func() {
		if _, err := d.Run("", "", 0); err != nil {
			t.Error(err)
		}
		close(done)
	}()

	// Printed after fireLoop returns, which is after its lock is released.
	waitForOut(t, &out, "in flight, gathering")

	took := make(chan *LaunchLock, 1)
	go func() {
		l, err := lockLaunches(b.App, &syncBuf{})
		if err != nil {
			t.Error(err)
		}
		took <- l
	}()
	select {
	case l := <-took:
		l.Release()
	case <-done:
		t.Fatal("the pass finished before the lock could be tested — it was never observed unlocked mid-gather")
	case <-time.After(2 * time.Second):
		t.Fatal("the gather held the launcher lock")
	}
	<-done
	if log := rawCalls(t, fake); !strings.Contains(log, "agent prompt") {
		t.Errorf("nothing was ever in flight:\n%s", log)
	}
}

// A dry pass acts on nothing. Making it queue behind a live pass would turn
// a read-only command into a blocking one, so it is deliberately unlocked —
// ADR 0011 §1 names the fire loop, and a dry run never fires.
func TestDispatchDryRunDoesNotTakeTheLock(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var out syncBuf
	d.Out = &out
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")

	mustHoldLock(t, b.App)

	done := make(chan int, 1)
	go func() {
		n, err := d.Run("", "", 0)
		if err != nil {
			t.Error(err)
		}
		done <- n
	}()
	select {
	case n := <-done:
		if n != 1 {
			t.Errorf("routable %d, want 1:\n%s", n, out.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("--dry-run blocked on the launcher lock")
	}
	if strings.Contains(out.String(), "waiting") {
		t.Errorf("--dry-run waited on the lock:\n%s", out.String())
	}
}

// ─── LaunchBead: the cockpit's `d` is a launcher too ─────────────────────────

// Held for the whole body (ADR 0011 §1): the cockpit's guards read state a
// running pass mutates, so the check and the launch it authorizes are one
// critical section.
func TestLaunchBeadWaitsForTheLock(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var out syncBuf
	d.Out = &out
	writePersona(t, b.App, "ranger", "[go]")
	agentPerLaunch(t, fake)
	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}

	held := mustHoldLock(t, b.App)

	done := make(chan error, 1)
	go func() {
		_, err := d.LaunchBead(is)
		done <- err
	}()

	waitForOut(t, &out, "waiting")
	if log := rawCalls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a blocked cockpit launch created a session:\n%s", log)
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
	if want := "workspace create --label " + SessionForBead("ranger", repo, "a-1"); !strings.Contains(rawCalls(t, fake), want) {
		t.Errorf("session not created after the lock was released:\n%s", rawCalls(t, fake))
	}
}

// ─── the bead's acceptance criterion ─────────────────────────────────────────

// Two simultaneous passes, two personas, two repos: one holds, the other
// waits, and no launch of one lands inside the other's fire loop. Both are
// started against a lock the test holds, so both are provably contending
// before either can run — the interleaving this rules out is arranged, not
// hoped against.
func TestTwoPassesDoNotInterleaveLaunches(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	writePersona(t, b.App, "alpha", "[go]")
	writePersona(t, b.App, "beta", "[py]")

	repo1, repo2 := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(repo1, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]},{"id":"b-1","title":"t","labels":["py"]}]`), 0o644)
	os.WriteFile(filepath.Join(repo2, "fake-ready.json"),
		[]byte(`[{"id":"a-2","title":"t","labels":["go"]},{"id":"b-2","title":"t","labels":["py"]}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo1+"\n  - "+repo2+"\n"), 0o644)

	held := mustHoldLock(t, b.App)

	outs := map[string]*syncBuf{"alpha": {}, "beta": {}}
	done := make(chan string, 2)
	for _, persona := range []string{"alpha", "beta"} {
		d := newTestDispatcher(t, b)
		d.Out = outs[persona]
		go func() {
			if _, err := d.Run("", persona, 0); err != nil {
				t.Error(err)
			}
			done <- persona
		}()
	}
	// Both are queued behind the test's lock before either may run.
	waitForOut(t, outs["alpha"], "waiting")
	waitForOut(t, outs["beta"], "waiting")

	held.Release()
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(60 * time.Second):
			t.Fatalf("a pass never finished:\nalpha:\n%s\nbeta:\n%s", outs["alpha"], outs["beta"])
		}
	}

	// Exactly one launcher ran at a time, so the four sessions were created
	// in two blocks of two. Any interleaving is a second transition.
	order := createdPersonas(t, fake)
	if len(order) != 4 {
		t.Fatalf("want 4 sessions created, got %v\nalpha:\n%s\nbeta:\n%s", order, outs["alpha"], outs["beta"])
	}
	transitions := 0
	for i := 1; i < len(order); i++ {
		if order[i] != order[i-1] {
			transitions++
		}
	}
	if transitions != 1 {
		t.Errorf("fire loops interleaved (%d transitions): %v", transitions, order)
	}
}

// createdPersonas is the persona of each session the fake herdr was asked to
// create, in creation order.
func createdPersonas(t *testing.T, fake string) []string {
	t.Helper()
	var order []string
	for _, ln := range strings.Split(rawCalls(t, fake), "\n") {
		if !strings.HasPrefix(ln, "workspace create ") {
			continue
		}
		fields := strings.Fields(ln)
		for i, f := range fields {
			if f != "--label" || i+1 >= len(fields) {
				continue
			}
			persona, _, _ := strings.Cut(fields[i+1], "-")
			order = append(order, persona)
		}
	}
	return order
}
