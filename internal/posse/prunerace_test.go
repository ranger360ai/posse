//go:build !posse_arm2 && !posse_arm3

package posse

// rangerhq-3a5t: the prune's unlink and the session-meta write share the
// launcher lock.
//
// prunable() proving death is evidence about the instant it was read.
// os.Remove then acts on a PATH, and between the two a create for the same
// name can legitimately pass mustNotOrphan (the old workspace really is
// dead) and write a fresh meta there. The unlink deletes the new record —
// rangerhq-9nso's damage reached through the write/delete interleave.
//
// Real flock in a temp RHQ_HOME, like launchlock_test.go, and a real second
// process for the concurrent write: the fake herdr is the test binary
// re-execed, so `interleave-write` fires from outside this process, inside
// the window, without the code under test knowing a harness exists.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const raceSock = "/tmp/this/herdr.sock"

// staleMeta writes a meta this server can answer for whose workspace is
// gone: the prune's own case, and the one the interleave attacks.
func staleMeta(t *testing.T, b *HerdrBackend, name string) {
	t.Helper()
	os.MkdirAll(b.metaDir(), 0o755)
	meta := "name: " + name + "\nworkspace: w404\npane: w404:p1\nemoji: x\nsocket: " + raceSock + "\n"
	if err := os.WriteFile(b.metaPath(name), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sessionsWithin runs a listing on another goroutine so a lock this test
// holds shows up as a failure with a name on it rather than as a suite that
// hangs until the CI timeout.
func sessionsWithin(t *testing.T, b *HerdrBackend, d time.Duration) []HerdrSession {
	t.Helper()
	type res struct {
		s   []HerdrSession
		err error
	}
	done := make(chan res, 1)
	go func() {
		s, err := b.Sessions()
		done <- res{s, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.s
	case <-time.After(d):
		t.Fatalf("Sessions() did not return within %s — the prune is waiting on a lock instead of sparing the file", d)
		return nil
	}
}

// A create landing in the check-to-unlink window keeps its meta. The fake
// herdr writes a fresh meta for the SAME name at the instant it answers
// workspace_not_found for the old workspace — which is exactly where a
// concurrent `posse new` or launcher lands — and the record that survives
// must be the new one.
func TestPruneDoesNotUnlinkAMetaACreateRewroteUnderIt(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, fake := newTestBackend(t)
	warn := &syncBuf{}
	b.Warn = warn

	// A live session keeps the board non-empty, so the prune reaches its
	// per-id query rather than stopping at the emptyBoard arm.
	mustCreate(t, b, NewSessionOpts{Name: "live"})
	staleMeta(t, b, "victim")

	// The concurrent create, as it really lands: a meta naming a live
	// workspace, stamped launched: now.
	fresh := "name: victim\nworkspace: w9\npane: w9:p1\nemoji: v\nsocket: " + raceSock +
		"\nlaunched: " + time.Now().UTC().Format(time.RFC3339) + "\n"
	if err := os.WriteFile(filepath.Join(fake, "interleave-write"),
		[]byte("w404\n"+b.metaPath("victim")+"\n"+fresh), 0o644); err != nil {
		t.Fatal(err)
	}
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "w9", Label: "victim"}))

	sessionsWithin(t, b, 90*time.Second)

	m, ok := b.readMeta("victim")
	if !ok {
		t.Fatal("the prune deleted a meta a create had just written: the session is live and nothing on disk names its workspace (rangerhq-3a5t)")
	}
	if m.Workspace != "w9" {
		t.Errorf("victim meta is not the one the create wrote: workspace %q, want w9", m.Workspace)
	}
	if !strings.Contains(warn.String(), "victim") {
		t.Errorf("a kept meta must be reported: %q", warn.String())
	}
}

// The same window, with nothing racing into it: the prune this bead
// hardened still prunes. Without this the fix above is indistinguishable
// from disabling the prune.
func TestPruneStillPrunesAProvenDeadMeta(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, _ := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "live"})
	staleMeta(t, b, "dead")

	sessionsWithin(t, b, 90*time.Second)

	if _, ok := b.readMeta("dead"); ok {
		t.Error("a meta whose workspace this server proved dead was not pruned")
	}
}

// A pass holding the launcher lock still lists sessions. flock is per open
// file description, so the prune's own LOCK_NB cannot succeed inside a
// launcher — and the answer to that must be sparing the file, never
// blocking a listing the cockpit reads on its select loop.
func TestListingInsideAHeldLaunchLockSparesInsteadOfDeadlocking(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, _ := newTestBackend(t)
	warn := &syncBuf{}
	b.Warn = warn
	mustCreate(t, b, NewSessionOpts{Name: "live"})
	staleMeta(t, b, "dead")

	lock, err := lockLaunches(b.App, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	sessions := sessionsWithin(t, b, 90*time.Second)

	if len(sessions) != 1 || sessions[0].Name != "live" {
		t.Errorf("want only the live session listed, got %+v", sessions)
	}
	if _, ok := b.readMeta("dead"); !ok {
		t.Error("a meta was unlinked while a launcher held the lock")
	}
	if !strings.Contains(warn.String(), "launch lock") {
		t.Errorf("the spared meta must say why it was kept: %q", warn.String())
	}
}

// The write half: a create really does hold the launcher lock while it is
// making the workspace and writing the meta. Only another process can
// answer that — flock is per open file description — so the fake herdr,
// which posse forks for the create, probes it (fakeProbeLaunchLock).
func TestCreateSessionHoldsTheLaunchLock(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, fake := newTestBackend(t)
	// The probe must be able to open the lock file whether or not anything
	// has taken it, so that "held" can only mean the flock was contended.
	if err := os.MkdirAll(filepath.Dir(LaunchLockPath(b.App)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "probe-launch-lock"), []byte(LaunchLockPath(b.App)), 0o644); err != nil {
		t.Fatal(err)
	}

	mustCreate(t, b, NewSessionOpts{Name: "s1"})

	got, err := os.ReadFile(filepath.Join(fake, "launch-lock-probe"))
	if err != nil {
		t.Fatal("the fake herdr never probed the lock: ", err)
	}
	if string(got) != "held" {
		t.Errorf("the launcher lock was %s during a create — `posse new` writes its meta unserialized, and a prune that proved this name dead can unlink it (rangerhq-3a5t)", got)
	}
}

// And a create INSIDE a launcher's lock — LaunchBead's own shape — must not
// wait on the lock it is already inside. flock is per open file description,
// so re-taking it would block forever: a dispatch pass that hangs on its
// first launch.
//
// What says it is inside is the lock itself, handed down (createSession's
// held). This used to be a process-wide depth counter, which answered a
// question about the PROCESS where the claim is about the CALLER — see the
// sibling below, which is the case that told the two apart
// (ranger-base-deaz).
//
// The goroutine and the select are a watchdog, so a regression is a named
// failure rather than a suite that hangs to the CI timeout; production's
// nested create runs on the goroutine that took the lock. The token is what
// is under test, not the stack it arrives on.
func TestCreateSessionInsideAHeldLaunchLockDoesNotDeadlock(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, _ := newTestBackend(t)

	lock, err := lockLaunches(b.App, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	done := make(chan error, 1)
	go func() { done <- b.createSession(NewSessionOpts{Name: "s1", Dir: t.TempDir()}, lock) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(90 * time.Second):
		t.Fatal("a create handed the launcher lock it runs inside waited for that lock — a dispatch pass would hang on its first launch")
	}
	if _, ok := b.readMeta("s1"); !ok {
		t.Error("no meta written for a session created under a held launcher lock")
	}
}

// The other half of the same question, and the one the depth counter got
// wrong: a create on a goroutine that holds NOTHING, in a process where
// another goroutine holds the launcher lock, must WAIT.
//
// This is not hypothetical shape. cmd/posse/cockpit.go runs LaunchBead —
// which holds the lock for its whole body — on its own goroutine, while the
// cockpit's select loop lists on another. Under the process-wide depth the
// listener's process "held the lock", so any create it grew would have read
// a critical section it was not in as its own and run nameFree/writeMeta
// inside it: rangerhq-3a5t's window, reopened (ranger-base-deaz).
//
// Discriminating in both directions: the create must not finish while the
// lock is held, and it must finish once it is released. Pre-fix the first
// arm failed in 0.03s.
func TestCreateOnAnotherGoroutineWaitsForTheLauncherLock(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, _ := newTestBackend(t)

	lock, err := lockLaunches(b.App, io.Discard) // goroutine M takes it
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release() // idempotent; the release below is the real one

	done := make(chan error, 1)
	// Goroutine B holds nothing. Nothing it can read tells it M's lock is
	// not its own except that it was handed none.
	go func() { done <- b.CreateSession(NewSessionOpts{Name: "b1", Dir: t.TempDir()}) }()

	select {
	case err := <-done:
		_, wrote := b.readMeta("b1")
		t.Fatalf("a create on a goroutine holding NO lock finished while another goroutine of this process held the launcher lock (err %v, meta written %v) — its nameFree and writeMeta ran inside a critical section it was not in (rangerhq-3a5t, ranger-base-deaz)", err, wrote)
	case <-time.After(2 * time.Second):
		// Waiting, which is the whole claim.
	}

	lock.Release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(90 * time.Second):
		t.Fatal("the create never finished after the launcher lock was released — it is waiting on something else")
	}
	if _, ok := b.readMeta("b1"); !ok {
		t.Error("no meta written by the create that waited for the lock")
	}
}
