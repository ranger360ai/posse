package posse

// ADR 0073 D1/B1 — bdWriteLock's acquisition and bdVerbWrites' classifier.
//
// These tests take real flocks (never a fake): flock is per open file
// description, so two Open+Flock pairs inside this one test process contend
// exactly as two processes do, and a faked lock would only test the fake —
// launchlock_test.go's reasoning, applied to this lock. None of them calls
// t.Parallel, which is what that file's serialization note is about and
// what keeps t.Setenv legal here (go vet refuses t.Setenv in a parallel
// test outright).

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ─── bdVerbWrites: the classifier ────────────────────────────────────────────

func TestBdVerbWritesClassifies(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		argv  []string
		write bool
	}{
		{"show", []string{"--no-daemon", "show", "t-1", "--json"}, false},
		{"list", []string{"--no-daemon", "list", "--json"}, false},
		{"ready", []string{"--no-daemon", "ready", "--json"}, false},
		{"blocked", []string{"--no-daemon", "blocked", "--json"}, false},
		{"info", []string{"--no-daemon", "info", "--json"}, false},
		{"where", []string{"--no-daemon", "where"}, false},
		{"stats", []string{"--no-daemon", "stats"}, false},
		{"search", []string{"--no-daemon", "search", "foo"}, false},
		{"count", []string{"--no-daemon", "count"}, false},
		{"version", []string{"--no-daemon", "version"}, false},
		{"help", []string{"--no-daemon", "help"}, false},
		{"prime", []string{"--no-daemon", "prime"}, false},
		{"onboard", []string{"--no-daemon", "onboard"}, false},
		{"bare comments", []string{"--no-daemon", "comments"}, false},
		{"comments by id", []string{"--no-daemon", "comments", "t-1", "--json"}, false},
		{"dep list", []string{"--no-daemon", "dep", "list", "t-1", "--json"}, false},
		{"dep tree", []string{"--no-daemon", "dep", "tree", "t-1", "--json"}, false},
		{"global opt skipped on a read", []string{"--actor", "tester", "show", "t-1"}, false},

		{"comments add", []string{"--no-daemon", "comments", "add", "t-1", "text"}, true},
		{"dep add", []string{"--no-daemon", "dep", "add", "t-1", "t-2"}, true},
		{"update --claim", []string{"--no-daemon", "update", "t-1", "--claim", "--json"}, true},
		{"create", []string{"--no-daemon", "create", "title", "--json"}, true},
		{"close", []string{"--no-daemon", "close", "t-1", "--json"}, true},
		{"sync --import-only, the self-heal", []string{"--no-daemon", "sync", "--import-only"}, true},
		{"sync --flush-only", []string{"--no-daemon", "sync", "--flush-only"}, true},
		{"an unlisted verb fails safe to a write", []string{"--no-daemon", "delete", "t-1"}, true},
		{"no verb at all fails safe to a write", []string{"--no-daemon"}, true},
		{"empty argv fails safe to a write", nil, true},
		{"global opt skipped on a write", []string{"--actor", "tester", "close", "t-1"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bdVerbWrites(c.argv); got != c.write {
				t.Errorf("bdVerbWrites(%q) = %v, want %v", c.argv, got, c.write)
			}
		})
	}
}

// ─── bdWriteStore: ADR 0072 D2's resolution ──────────────────────────────────

func TestBdWriteStoreEmptyDirUsesBeadsDirEnv(t *testing.T) {
	t.Setenv("BEADS_DIR", "/tmp/ranger-base-g8lmt-store")
	if got, want := bdWriteStore(""), "/tmp/ranger-base-g8lmt-store"; got != want {
		t.Errorf("bdWriteStore(\"\") = %q, want the session's own BEADS_DIR %q", got, want)
	}
}

func TestBdWriteStoreEmptyDirFallsBackToBeadsHomeOfCwd(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	if got, want := bdWriteStore(""), beadsHome("."); got != want {
		t.Errorf("bdWriteStore(\"\") = %q, want beadsHome(\".\") = %q", got, want)
	}
}

func TestBdWriteStoreNamedDirUsesBeadsHomeOfThatDir(t *testing.T) {
	t.Setenv("BEADS_DIR", "/tmp/should-be-shed-for-a-named-dir")
	dir := t.TempDir()
	if got, want := bdWriteStore(dir), beadsHome(dir); got != want {
		t.Errorf("bdWriteStore(%q) = %q, want beadsHome(dir) = %q", dir, got, want)
	}
}

// ─── bdWriteLockPath: the realpath hop ───────────────────────────────────────

func TestBdWriteLockPathCollapsesASymlinkedAlias(t *testing.T) {
	t.Parallel()
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	dir := t.TempDir()
	p1, p2 := bdWriteLockPath(dir, real), bdWriteLockPath(dir, link)
	if p1 != p2 {
		t.Errorf("a direct spelling (%s) and a symlinked alias (%s) of the same store hashed to different lock files: %q vs %q",
			real, link, p1, p2)
	}
}

func TestBdWriteLockPathDiffersAcrossStores(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, b := bdWriteLockPath(dir, t.TempDir()), bdWriteLockPath(dir, t.TempDir())
	if a == b {
		t.Errorf("two distinct stores hashed to the same lock file: %q", a)
	}
}

// ─── bdWriteLock: acquisition, contention, timeout, degrade ─────────────────

// The bead's acceptance criterion: two in-process holders serialize. The
// second is provably blocked before the first releases — arranged, not
// raced for, the same shape TestLaunchLockSecondLauncherWaits takes.
func TestBdWriteLockSecondHolderWaits(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	store := t.TempDir()

	first, err := bdWriteLock(store, []string{"close", "t-1"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil {
		t.Fatal("the control lock degraded to unlocked — check the XDG_CACHE_HOME wiring")
	}

	type got struct {
		lock *BdWriteLock
		err  error
	}
	done := make(chan got, 1)
	go func() {
		l, err := bdWriteLock(store, []string{"update", "t-1", "--claim"}, 5*time.Second)
		done <- got{l, err}
	}()

	select {
	case g := <-done:
		first.Release()
		g.lock.Release()
		t.Fatalf("the second holder did not wait for the first: %+v", g)
	case <-time.After(300 * time.Millisecond):
	}

	first.Release()
	select {
	case g := <-done:
		if g.err != nil {
			t.Fatal(g.err)
		}
		if g.lock == nil {
			t.Fatal("the second holder did not take the lock once it was free")
		}
		g.lock.Release()
	case <-time.After(5 * time.Second):
		t.Fatal("the second holder never took the released lock")
	}
}

// A released lock is free at once: the file outlives its holder (the same
// reason LaunchLock's does — a fresh inode would let two holders lock two
// different files at one path), so that must not read as still held.
func TestBdWriteLockFreeAfterRelease(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	store := t.TempDir()

	first, err := bdWriteLock(store, []string{"close", "t-1"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	first.Release()
	first.Release() // idempotent

	start := time.Now()
	second, err := bdWriteLock(store, []string{"close", "t-1"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if second == nil {
		t.Fatal("the second attempt degraded to unlocked")
	}
	defer second.Release()
	if elapsed := time.Since(start); elapsed > bdWriteLockPoll*5 {
		t.Errorf("a free lock made the next attempt wait %s", elapsed)
	}
}

// Two different stores never contend: the hash keys them apart, so a held
// lock on one must not block a write against the other.
func TestBdWriteLockDifferentStoresDoNotContend(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	a, err := bdWriteLock(t.TempDir(), []string{"close", "a-1"}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("control lock A degraded to unlocked")
	}
	defer a.Release()

	b, err := bdWriteLock(t.TempDir(), []string{"close", "b-1"}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if b == nil {
		t.Fatal("lock B blocked on lock A's store despite being a different store")
	}
	b.Release()
}

// The bead's other acceptance criterion: a lock-timeout test with a short
// injected wait. bdWriteLock takes the bound as a parameter for exactly
// this — the suite must not spend BdWriteLockWait's real 30s to prove the
// timeout arm.
func TestBdWriteLockTimesOut(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	store := t.TempDir()

	held, err := bdWriteLock(store, []string{"close", "t-1"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if held == nil {
		t.Fatal("the control lock degraded to unlocked")
	}
	defer held.Release()

	wait := 150 * time.Millisecond
	start := time.Now()
	lock, err := bdWriteLock(store, []string{"update", "t-1"}, wait)
	elapsed := time.Since(start)
	if lock != nil {
		lock.Release()
		t.Fatal("a timed-out attempt still returned a held lock")
	}
	var te *BdWriteLockTimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("want *BdWriteLockTimeoutError, got %v (%T)", err, err)
	}
	if te.Store != store {
		t.Errorf("the timeout error names store %q, want %q", te.Store, store)
	}
	if te.Wait != wait {
		t.Errorf("the timeout error names wait %s, want %s", te.Wait, wait)
	}
	if elapsed < wait {
		t.Errorf("gave up after %s, want at least the %s bound", elapsed, wait)
	}
	want := "refused by posse gate: " + store + "'s bd write lock was held for " + wait.String() +
		" — a bd write is still running against it; nothing was written (ADR 0073)"
	if err.Error() != want {
		t.Errorf("timeout error text:\ngot  %q\nwant %q", err.Error(), want)
	}
	if !IsBdWriteLockTimeout(err) {
		t.Errorf("IsBdWriteLockTimeout(%v) = false", err)
	}
	if IsBdWriteLockTimeout(nil) {
		t.Error("IsBdWriteLockTimeout(nil) = true")
	}
}

// The lock holds pid + the argv head, for the operator's eyes — an flock
// records no holder of its own (ADR 0059 D3), which is the whole reason
// this courtesy exists.
func TestBdWriteLockStampsHolder(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	store := t.TempDir()

	lock, err := bdWriteLock(store, []string{"--no-daemon", "close", "t-1", "--json"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil {
		t.Fatal("the control lock degraded to unlocked")
	}
	defer lock.Release()

	path := bdWriteLockPath(bdWriteLockDir(), store)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stamp := string(b)
	if want := "pid: " + strconv.Itoa(os.Getpid()); !strings.Contains(stamp, want) {
		t.Errorf("lock file does not stamp our pid:\n%s", stamp)
	}
	// Only the first three argv tokens — the fourth, --json, must not appear.
	if want := "argv: --no-daemon close t-1"; !strings.Contains(stamp, want) {
		t.Errorf("lock file does not stamp the argv head %q:\n%s", want, stamp)
	}
	if strings.Contains(stamp, "--json") {
		t.Errorf("lock file stamped past argv[0..2]:\n%s", stamp)
	}
}

// "Lock dir unopenable: one stderr line, proceed unlocked" — no HOME to
// resolve the cache dir under.
func TestBdWriteLockDegradesWithNoHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	lock, err := bdWriteLock(t.TempDir(), []string{"close", "t-1"}, time.Second)
	if err != nil {
		t.Fatalf("a missing HOME must degrade, not fail the write: %v", err)
	}
	if lock != nil {
		lock.Release()
		t.Fatal("no HOME to resolve the lock dir under, yet a lock was taken")
	}
}

// Same degrade, by a lock directory that cannot be made at all (a cage
// without the ~/.cache grant reads this way — ranger-base-r3czg).
func TestBdWriteLockDegradesWhenDirCannotBeMade(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "posse")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", base)

	lock, err := bdWriteLock(t.TempDir(), []string{"close", "t-1"}, time.Second)
	if err != nil {
		t.Fatalf("an unmakeable lock dir must degrade, not fail the write: %v", err)
	}
	if lock != nil {
		lock.Release()
		t.Fatal("a lock was taken despite an unmakeable lock dir")
	}
}

// ─── Bd.runOnce: the lock actually wraps the child ───────────────────────────

// waitForFile polls until path exists or the deadline passes, so a test does
// not have to guess how long a child needs to start.
func waitForFile(t *testing.T, path string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("%s never appeared within %s", path, within)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The bead's build item (3): Bd.runOnce holds D1's lock across cmd.Run for a
// write verb — proven at the OS level, not the Go call stack: while a fake
// bd child is still running (sleeping, after touching a marker), an
// INDEPENDENT bdWriteLock attempt on the same store must find the lock
// genuinely held, and must find it free again only once the child has
// exited.
func TestRunOnceHoldsWriteLockAcrossChild(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	storeDir := t.TempDir()

	scratch := t.TempDir()
	marker := filepath.Join(scratch, "started")
	bin := filepath.Join(scratch, "bd")
	body := "#!/bin/sh\ntouch " + marker + "\nsleep 0.4\necho '{}'\n"
	if err := WriteExecutable(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	b := Bd{Bin: bin}

	done := make(chan error, 1)
	go func() {
		_, err := b.runOnce(storeDir, "close", "t-1", "--json")
		done <- err
	}()

	waitForFile(t, marker, 3*time.Second)

	store := bdWriteStore(storeDir)
	if _, err := bdWriteLock(store, []string{"update"}, 150*time.Millisecond); !IsBdWriteLockTimeout(err) {
		t.Fatalf("a concurrent write's lock was not held while runOnce's child was running: err=%v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runOnce's child never finished")
	}

	lock, err := bdWriteLock(store, []string{"update"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil {
		t.Fatal("the lock did not free once runOnce released it")
	}
	lock.Release()
}

// A read verb never takes the lock at all: it must run to completion
// unimpeded while a write against the SAME store holds it.
func TestRunOnceDoesNotLockOnARead(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	storeDir := t.TempDir()

	bin := filepath.Join(t.TempDir(), "bd")
	if err := WriteExecutable(bin, []byte("#!/bin/sh\necho '[]'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := Bd{Bin: bin}

	held, err := bdWriteLock(bdWriteStore(storeDir), []string{"close", "t-1"}, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if held == nil {
		t.Fatal("the control lock degraded to unlocked")
	}
	defer held.Release()

	done := make(chan error, 1)
	go func() {
		_, err := b.runOnce(storeDir, "list", "--json")
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a read verb waited on a write lock held by somebody else")
	}
}
