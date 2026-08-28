package rhq

// ranger-base-w4h5: relaunch's destructive steps share the launcher lock.
//
// rangerhq-3a5t put the prune's unlink and the whole of CreateSession under
// the launcher lock, so proof-of-death and the act on it are one critical
// section. Relaunch has the same shape twice and neither was covered:
// clearDeadMeta proves death with mustNotOrphan and then os.Removes the
// path, and keepRecipe proves the same thing and then writeMetas over it.
// A create for that name landing between check and act is deleted by the
// first and blanked by the second — rangerhq-i2g9's and rangerhq-cpeh's
// damage, reached through the write/delete interleave.
//
// Same rig as prunerace_test.go, for the same reason: real flock in a temp
// RHQ_HOME, and the concurrent write comes from a real second process (the
// fake herdr is the test binary re-execed), firing inside the window
// without the code under test knowing a harness exists.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// deadMeta writes a meta this server can answer for whose workspace is gone
// — the board where relaunch reaches clearDeadMeta rather than the kill.
func deadMeta(t *testing.T, b *HerdrBackend, name, dir string) {
	t.Helper()
	if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "name: " + name + "\nworkspace: w404\npane: w404:p1\nemoji: x\ndir: " + dir +
		"\nsocket: " + raceSock + "\n"
	if err := os.WriteFile(b.metaPath(name), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// armCreateInterleave makes the fake herdr write a fresh meta for name — a
// create's own, naming a live workspace, stamped launched: now — at the
// instant it answers the per-id query about onID. That is the window
// between mustNotOrphan proving death and the act destroying the record.
//
// The workspace it names is hidden from `workspace list` and answers
// `workspace get`, which is how a create that has just landed looks to a
// pass holding an older listing — and it keeps the preflight
// (nameWornElsewhere, which reads the listing) from refusing over a
// workspace that does not exist yet when the relaunch starts.
func armCreateInterleave(t *testing.T, b *HerdrBackend, fake, name, onID, ws, dir string) {
	t.Helper()
	fresh := "name: " + name + "\nworkspace: " + ws + "\npane: " + ws + ":p1\nemoji: v\ndir: " + dir +
		"\nsocket: " + raceSock + "\nlaunched: " + time.Now().UTC().Format(time.RFC3339) + "\n"
	if err := os.WriteFile(filepath.Join(fake, "interleave-write"),
		[]byte(onID+"\n"+b.metaPath(name)+"\n"+fresh), 0o644); err != nil {
		t.Fatal(err)
	}
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: ws, Label: name}))
	hideFromTheListing(t, fake, ws)
}

// The delete. A create landing in clearDeadMeta's check-to-unlink window
// keeps its meta: the record that survives must be the new one, and the
// relaunch must refuse rather than build a second workspace under a label
// the create's session already holds.
func TestRelaunchDoesNotUnlinkAMetaACreateRewroteUnderIt(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, fake := newTestBackend(t)
	dir := t.TempDir()
	// A live session keeps the board non-empty, so the guards reach their
	// per-id query rather than stopping at the emptyBoard arm.
	mustCreate(t, b, NewSessionOpts{Name: "live"})
	deadMeta(t, b, "victim", dir)
	armCreateInterleave(t, b, fake, "victim", "w404", "w9", dir)
	os.Remove(filepath.Join(fake, "calls.log"))

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "victim", NoLand: true})

	m, ok := b.readMeta("victim")
	if !ok {
		t.Fatalf("relaunch deleted a meta a create had just written: the session is live and nothing on disk names its workspace (ranger-base-w4h5)\n%s", out.String())
	}
	if m.Workspace != "w9" {
		t.Errorf("victim meta is not the one the create wrote: workspace %q, want w9", m.Workspace)
	}
	if err == nil {
		t.Fatalf("relaunch continued over a live session's record:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "NOT closed") {
		t.Errorf("refusal did not say the session was left alone: %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a second workspace was built under a label the create's session holds:\n%s", log)
	}
}

// The same window with nothing racing into it: relaunch still clears a meta
// it proved dead and still recreates the session. Without this the fix above
// is indistinguishable from a relaunch that refuses everything.
func TestRelaunchStillClearsAProvenDeadMeta(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, fake := newTestBackend(t)
	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "live"})
	deadMeta(t, b, "victim", dir)
	os.Remove(filepath.Join(fake, "calls.log"))

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "victim", NoLand: true}); err != nil {
		t.Fatalf("relaunch of a proven-dead meta refused: %v\n%s", err, out.String())
	}
	m := metaOf(t, b, "victim")
	if m.Workspace == "" || m.Workspace == "w404" {
		t.Errorf("the recreate did not rewrite the record: workspace %q", m.Workspace)
	}
	if log := calls(t, fake); !strings.Contains(log, "workspace create") {
		t.Errorf("no workspace was created, so this pass measured nothing:\n%s", log)
	}
}

// The write, the fourth meta-destroying step (rangerhq-9jk1). A create
// landing between keepRecipe's proof and its writeMeta must keep its
// workspace id: blanking it is the cpeh damage, and the id is the one field
// the operator cannot get back from anywhere else.
func TestKeepRecipeDoesNotBlankAMetaACreateRewroteUnderIt(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, fake := newTestBackend(t)
	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "live"})
	deadMeta(t, b, "s1", dir)
	m := metaOf(t, b, "s1")
	armCreateInterleave(t, b, fake, "s1", "w404", "w9", dir)

	kept := b.keepRecipe(m)

	cur := metaOf(t, b, "s1")
	if cur.Workspace != "w9" {
		t.Errorf("keepRecipe blanked the record of a live session: workspace %q, want w9 (ranger-base-w4h5)", cur.Workspace)
	}
	if kept != "w9" {
		t.Errorf("keepRecipe reported kept %q, want w9 — the caller's error must name the workspace it did not blank", kept)
	}
}

// And the control: after an ordinary kill there is nothing to protect, so
// the recipe is still written back and `posse relaunch` is still its own
// retry.
func TestKeepRecipeStillWritesTheRecipeBack(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, _ := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "live"})
	mustCreate(t, b, NewSessionOpts{Name: "s1"})
	m := metaOf(t, b, "s1")
	if err := b.KillSession("s1"); err != nil {
		t.Fatal(err)
	}

	if kept := b.keepRecipe(m); kept != "" {
		t.Fatalf("keepRecipe after a real kill must write, kept %q", kept)
	}
	cur := metaOf(t, b, "s1")
	if cur.Workspace != "" || cur.Pane != "" {
		t.Errorf("the recipe was not blanked: workspace %q pane %q", cur.Workspace, cur.Pane)
	}
	if cur.Dir != m.Dir || cur.Name != "s1" {
		t.Errorf("the recipe did not survive: %+v", cur)
	}
}

// The lock is really held while relaunch is destroying and rebuilding, not
// merely taken somewhere. Only another process can answer that — flock is
// per open file description — so the fake herdr posse forks for the
// recreate probes it (fakeProbeLaunchLock).
func TestRelaunchHoldsTheLaunchLockWhileItReplacesTheSession(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "s1"})
	// The probe must be able to open the lock file whether or not anything
	// has taken it, so that "held" can only mean the flock was contended.
	if err := os.MkdirAll(filepath.Dir(LaunchLockPath(b.App)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "probe-launch-lock"), []byte(LaunchLockPath(b.App)), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fake, "launch-lock-probe"))

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1", NoLand: true}); err != nil {
		t.Fatalf("relaunch: %v\n%s", err, out.String())
	}

	got, err := os.ReadFile(filepath.Join(fake, "launch-lock-probe"))
	if err != nil {
		t.Fatal("the fake herdr never probed the lock: ", err)
	}
	if string(got) != "held" {
		t.Errorf("the launcher lock was %s while relaunch was replacing the session — its kill and its recreate are unserialized, and a prune or a create for this name can land between them (ranger-base-w4h5)", got)
	}
}

// And a relaunch INSIDE a launcher's lock must not wait on the lock its own
// process holds: flock would block there forever, which is a pass that
// hangs on its first refresh.
func TestRelaunchInsideAHeldLaunchLockDoesNotDeadlock(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", raceSock)
	b, _ := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "s1"})
	was := metaOf(t, b, "s1").Workspace

	lock, err := lockLaunches(b.App, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()

	done := make(chan error, 1)
	var out syncBuf
	go func() { done <- b.RelaunchSession(&out, RelaunchOpts{Name: "s1", NoLand: true}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("relaunch: %v\n%s", err, out.String())
		}
	case <-time.After(90 * time.Second):
		t.Fatal("RelaunchSession blocked on the launcher lock its own process holds")
	}
	if now := metaOf(t, b, "s1").Workspace; now == was || now == "" {
		t.Errorf("the session was not replaced under the held lock: workspace %q, was %q", now, was)
	}
}
