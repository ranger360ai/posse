// The tests in this file assert on flock ACQUISITION — who holds the lock,
// and whether a released one reads as free — and they are SERIAL on purpose,
// which is why none of them carries t.Parallel. Two of them side by side read
// a released lock as still held, on lock files that are per test: 3-6 failures
// in 60 at -parallel 8 over this file alone (ranger-base-9l77f, filed off
// ranger-base-aupee, product cause not yet found). cmd/testparallel names them
// so that re-running it cannot put t.Parallel back. The hundreds of tests that
// merely TAKE the launcher lock on their way through a pass are unaffected.

package posse

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
	"io"
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
	t.Parallel()
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

// Our own pid gets its own phrasing. A wait on a lock this process holds is
// legitimate — the cockpit lists on its select loop while LaunchBead
// launches on its own goroutine — and it is also what a caller that should
// have been handed the lock and was not looks like, forever. The line cannot
// tell them apart, so it says the one thing it knows: the holder is us, and
// not this goroutine (ranger-base-deaz).
func TestLaunchLockHolderNamesOurOwnProcess(t *testing.T) {
	b, _ := newTestBackend(t)
	path := LaunchLockPath(b.App)
	l := mustHoldLock(t, b.App) // stamps our pid
	defer l.Release()

	h := lockHolder(path)
	if !strings.Contains(h, "this process") || !strings.Contains(h, "another goroutine") {
		t.Errorf("holder %q — a lock held by this process on another goroutine must say so", h)
	}
	// Still names the pid: the waiting line's oldest job (rangerhq-9nso).
	if want := strconv.Itoa(os.Getpid()); !strings.Contains(h, want) {
		t.Errorf("holder %q does not name our pid %s", h, want)
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

// The cockpit's shape (rangerhq-ecl2): its Dispatcher writes to io.Discard,
// because a line printed straight at a TUI is garbage on the frame — so the
// one line ADR 0011 §1 promises a blocked launcher has nowhere to land
// unless Progress takes it. With Progress set the line arrives there,
// naming the holder, and Out stays clean.
func TestLaunchBeadReportsTheLockWaitToProgress(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	var out, prog syncBuf
	d.Out = &out
	d.Progress = func(line string) { prog.Write([]byte(line + "\n")) }
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

	waitForOut(t, &prog, "waiting")
	if want := "pid " + strconv.Itoa(os.Getpid()); !strings.Contains(prog.String(), want) {
		t.Errorf("the progress line does not name the holder (%s):\n%s", want, prog.String())
	}
	if strings.Contains(out.String(), "waiting") {
		t.Errorf("the wait line also went to Out, which a cockpit renders as garbage:\n%s", out.String())
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

// ─── the non-blocking take, and what it may say ─────────────────────────────

// The kill's non-blocking lock: with a launcher holding it, the session is
// still killed, nothing is merged, and nothing is lost.
//
// It lived in worktree_test.go with t.Parallel until ranger-base-zppcv, which
// is the one acquisition test the ranger-base-9l77f rule missed — it holds a
// lock, releases it, and asserts the release read as free, which is exactly
// the shape this file is serial for. The line that failed the pass that filed
// the bead is the free-lock take below, once, unreproducible at -count=200
// alone. That is a candidate cause for that red and not a proof of one; the
// product cause stays open on ranger-base-9l77f.
func TestTryLockLaunchesDoesNotWait(t *testing.T) {
	a := wtApp(t)
	held, err := lockLaunches(a, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	lock, why := tryLockLaunches(a)
	if lock != nil {
		lock.Release()
		t.Fatal("the non-blocking take succeeded while the lock was held")
	}
	if why != "a launcher is running" {
		t.Errorf("a held lock is contention and has to say so, got %q", why)
	}
	held.Release()
	got, why := tryLockLaunches(a)
	if got == nil {
		t.Fatalf("the non-blocking take failed on a free lock: %s", why)
	}
	if why != "" {
		t.Errorf("a lock that was TAKEN must carry no reason, got %q", why)
	}
	got.Release()
}

// The failures that are not contention, each saying which it is.
//
// ranger-base-zppcv: tryLockLaunches answered an unmakeable state dir, an
// unopenable lock file and a genuinely held lock with one bare false, so
// every reader downstream — `posse kill`'s deferral line, the prune's
// sparing, the pin above — asserted "a launcher is running" over three
// unrelated events. A red on the free-lock arm then cost a whole
// verification pass, because nothing in the pin or under it could say which
// of the three had happened.
//
// Both broken arms are built out of the filesystem rather than an -overlay
// mutant, so what is pinned is the real MkdirAll and the real OpenFile
// failing: a state dir that is a regular FILE is ENOTDIR, and a lock path
// that is a DIRECTORY is EISDIR for O_RDWR. The fourth arm — an flock that
// fails for reasons of its own (EBADF, ENOLCK) — has no portable fixture
// and is left to the reading of launchlock.go.
//
// Serial with the rest of the file: its contention arm asserts acquisition.
func TestTryLockLaunchesNamesWhichFailure(t *testing.T) {
	// The class is the reason with its error text cut off. The texts carry
	// a temp path, so comparing them whole would report four distinct
	// reasons even if all four arms returned one sentence — the very thing
	// this test exists to catch.
	class := func(why string) string {
		if i := strings.Index(why, ":"); i >= 0 {
			return why[:i]
		}
		return why
	}
	seen := map[string]string{}
	note := func(arm, why string) {
		if prev, dup := seen[class(why)]; dup {
			t.Errorf("%s reports what %s reports (%q) — the arms are indistinguishable again", arm, prev, why)
		}
		seen[class(why)] = arm
	}

	// Taken: no reason at all.
	free := wtApp(t)
	lock, why := tryLockLaunches(free)
	if lock == nil {
		t.Fatalf("the control arm could not take a free lock: %s", why)
	}
	lock.Release()
	// Said out loud, not left to the dedupe below: `note` only catches a
	// taken lock that carries some OTHER arm's class, so a reason of its
	// own walked past this test entirely. MEASURED under -overlay
	// (ranger-base-2ljyf): a take returning "lock taken" left this test
	// green and was caught only by TestTryLockLaunchesDoesNotWait, while a
	// take returning "a launcher is running" failed both. An empty why is
	// the whole contract of the taken arm — both readers branch on the lock
	// being nil, and neither has any business printing a reason for a lock
	// it holds.
	if why != "" {
		t.Errorf("a lock that was TAKEN must carry no reason, got %q", why)
	}
	note("a free lock", why)

	// Held: the one arm that means wait for someone else.
	busy := wtApp(t)
	held, err := lockLaunches(busy, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	lock, why = tryLockLaunches(busy)
	if lock != nil {
		lock.Release()
		t.Fatal("the non-blocking take succeeded while the lock was held")
	}
	held.Release()
	if why != "a launcher is running" {
		t.Errorf("a held lock: want %q, got %q", "a launcher is running", why)
	}
	note("a held lock", why)

	broken := []struct {
		arm     string
		arrange func(t *testing.T, a *App)
		want    string
	}{
		{"a state dir that is a regular file", func(t *testing.T, a *App) {
			if err := os.WriteFile(a.StateDir, []byte("not a directory\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "directory could not be made"},
		{"a lock path that is a directory", func(t *testing.T, a *App) {
			if err := os.MkdirAll(LaunchLockPath(a), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "could not be opened"},
	}
	for _, tc := range broken {
		a := wtApp(t)
		tc.arrange(t, a)
		lock, why := tryLockLaunches(a)
		if lock != nil {
			lock.Release()
			t.Errorf("%s: the lock was taken anyway", tc.arm)
			continue
		}
		if !strings.Contains(why, tc.want) {
			t.Errorf("%s: want a reason naming %q, got %q", tc.arm, tc.want, why)
		}
		if strings.Contains(why, "a launcher is running") {
			t.Errorf("%s: reads as contention, so every pass from here on spares the work for a launcher that does not exist: %q", tc.arm, why)
		}
		note(tc.arm, why)
	}
}
