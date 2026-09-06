//go:build !posse_arm2 && !posse_arm3

package posse

// QA suite for the rangerhq-cpeh guard (verifying its close, rangerhq-ykzq):
// mustNotOrphan, the rangerhq-9nso rule applied to the destructive *write*.
//
// The fix's own tests walk the incident's board: a meta naming a workspace
// this server holds, hidden from this pass's listing. These are the boards
// it does not walk — the ones where the guard asks its question of a server
// that cannot answer it, and reads that server's truthful "I never held
// that" as death.
//
// Tests marked t.Skip pin a filed bug: they encode the expected behavior and
// fail today. Remove the skip when the bead closes.

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// aliveOn reports whether the fake herdr server whose state lives in dir
// still holds this workspace — asked of that server's own store, so it is
// evidence about the *other* server than the one the backend is pointed at.
func aliveOn(t *testing.T, dir, id string) bool {
	t.Helper()
	old := os.Getenv("RHQ_FAKE_DIR")
	os.Setenv("RHQ_FAKE_DIR", dir)
	defer os.Setenv("RHQ_FAKE_DIR", old)
	for _, w := range fakeLoadWS() {
		if w.WorkspaceID == id {
			return true
		}
	}
	return false
}

// ─── the guard asks the wrong server ─────────────────────────────────────────

// The prune asks WorkspaceAlive only after the rangerhq-8fq socket guards
// have established that this server is the one that would know: an empty
// listing, a meta written against another socket, and a meta with no socket
// recorded all keep the file *without ever asking*, because this server's
// "workspace_not_found" is a true answer about an id it never held and no
// evidence at all about the session. prunable()'s own comment says it:
// "called only after the socket guards above have established that this
// server is the one that would know."
//
// mustNotOrphan has none of them. It asks the current server about a
// workspace the meta may say plainly belongs to a different one, and takes
// not_found as free real estate. So the board rangerhq-snd already cost the
// fleet once — the real RHQ_HOME pointed at a scratch herdr — reaches the
// same harm through the write: the meta is not deleted, it is overwritten,
// which rangerhq-cpeh is the bead saying is exactly as unrecoverable.
//
// The control is the point: on this identical board the delete refuses and
// the write proceeds, so the two halves of one rule disagree.
func TestCreateMustNotOverwriteAMetaFromAnotherHerdrServer(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qacpeh/A.sock")
	b, fakeA := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "shared", Cmd: "claude"})
	m, ok := b.readMeta("shared")
	if !ok {
		t.Fatal("no meta for shared")
	}
	wsA := m.Workspace
	if m.Socket != "/tmp/qacpeh/A.sock" {
		t.Fatalf("setup: meta records socket %q, wanted server A's", m.Socket)
	}

	// Server B: a different herdr, holding a workspace of its own (so this
	// is not the empty-listing arm), sharing the one meta dir — which is why
	// the socket guards exist at all.
	fakeB := t.TempDir()
	// Server B numbers its own workspaces; seed the counter so its ids
	// cannot collide with A's (a real herdr's id space is its own, and an
	// id collision would be a different bug than the one under test).
	if err := os.WriteFile(filepath.Join(fakeB, "ws-counter"), []byte("100"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qacpeh/B.sock")
	t.Setenv("RHQ_FAKE_DIR", fakeB)
	mustCreate(t, b, NewSessionOpts{Name: "local", Cmd: "claude"})

	// The delete side, on this exact board: refuses, keeps the file (8fq).
	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("shared"); !ok {
		t.Fatal("setup: the prune deleted a meta from another socket — that is rangerhq-8fq, not this bead")
	}
	if b.HasSession("shared") {
		t.Fatal("setup: server B was not supposed to hold shared's workspace")
	}

	err := b.CreateSession(NewSessionOpts{Name: "shared", Cmd: "claude", Dir: t.TempDir()})

	if !aliveOn(t, fakeA, wsA) {
		return // the session really did die on A; nothing was orphaned
	}
	now, ok := b.readMeta("shared")
	if err == nil && ok && now.Workspace != wsA {
		t.Fatalf("overwrote the only record of a session alive on another herdr: the meta now names %s "+
			"and nothing names %s, which server A still holds with its agent running "+
			"(the prune refuses this same board — rangerhq-8fq guards in front of prunable; mustNotOrphan has none)",
			now.Workspace, wsA)
	}
}

// The same gap with nothing on disk to argue about: a meta with no socket
// recorded (written before the field existed, or by a session created
// outside a herdr pane — backfillSocket's own note says those stay
// unstamped, and so unprunable forever). The prune keeps such a meta
// unconditionally. mustNotOrphan asks anyway, and this server's not_found
// is the only answer it can give about a workspace it never held.
func TestCreateMustNotOverwriteAMetaWithNoSocketRecorded(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qacpeh/A.sock")
	b, fakeA := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "legacy", Cmd: "claude"})
	m, _ := b.readMeta("legacy")
	wsA := m.Workspace
	m.Socket = "" // a meta from before socket: existed
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	fakeB := t.TempDir()
	// Server B numbers its own workspaces; seed the counter so its ids
	// cannot collide with A's (a real herdr's id space is its own, and an
	// id collision would be a different bug than the one under test).
	if err := os.WriteFile(filepath.Join(fakeB, "ws-counter"), []byte("100"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qacpeh/B.sock")
	t.Setenv("RHQ_FAKE_DIR", fakeB)
	mustCreate(t, b, NewSessionOpts{Name: "local", Cmd: "claude"})

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("legacy"); !ok {
		t.Fatal("setup: the prune deleted an unstamped meta — that is rangerhq-8fq, not this bead")
	}

	err := b.CreateSession(NewSessionOpts{Name: "legacy", Cmd: "claude", Dir: t.TempDir()})

	if !aliveOn(t, fakeA, wsA) {
		return
	}
	now, ok := b.readMeta("legacy")
	if err == nil && ok && now.Workspace != wsA {
		t.Fatalf("overwrote an unstamped meta whose workspace is alive elsewhere: meta now names %s, %s orphaned",
			now.Workspace, wsA)
	}
}

// ─── the guard, bypassed by deleting its evidence first ──────────────────────

// mustNotOrphan can only refuse what it can read. RelaunchSession reads the
// meta, then — when HasSession says no, which is the rangerhq-9nso condition
// itself: alive on the server, absent from this listing — os.Remove()s the
// meta as "ours to clear" and calls CreateSession. The guard then finds no
// record, correctly concludes there is nothing to orphan, and creates a
// second workspace under the same label. The live one is orphaned with its
// agent running: the cpeh harm exactly, reached by a path where the fix
// cannot fire because its evidence was deleted one line earlier.
//
// HasSession answers out of Sessions(), the same listing snapshot the whole
// bead chain is about, and it is the only thing standing in front of that
// unlink.
func TestRelaunchDoesNotOrphanALiveWorkspaceBehindAStaleListing(t *testing.T) {
	b, fake, ws := qa9nsoSetup(t)
	// This pass's listing was taken before newborn's workspace existed;
	// `workspace get` still answers for it, because it is alive.
	if err := os.WriteFile(filepath.Join(fake, "hidden-from-list"), []byte(ws), 0o644); err != nil {
		t.Fatal(err)
	}
	if b.HasSession("newborn") {
		t.Fatal("setup: the session was supposed to be invisible to this pass")
	}

	err := b.RelaunchSession(io.Discard, RelaunchOpts{Name: "newborn"})

	alive, aerr := b.H.WorkspaceAlive(ws)
	if aerr != nil {
		t.Fatal(aerr)
	}
	if !alive {
		return // the workspace really did die; nothing was orphaned
	}
	m, ok := b.readMeta("newborn")
	if !ok {
		t.Fatalf("relaunch removed the meta of a session whose workspace %s is still alive "+
			"(err=%v) — nothing on disk names it now", ws, err)
	}
	if m.Workspace != ws {
		t.Fatalf("relaunch created a second workspace over a live session: the meta names %s, "+
			"%s is still alive with its agent running (err=%v)", m.Workspace, ws, err)
	}
}

// The third arm, and the one with a receipt: an empty listing. rangerhq-snd
// is a pass with the real RHQ_HOME talking to a scratch herdr that holds
// nothing — the board the prune refuses first ("a herdr that just came up,
// or one that never held these sessions at all"). The create asks that
// server about every name it is given and believes every not_found.
func TestCreateMustNotOverwriteAMetaAgainstAnEmptyListing(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qacpeh/A.sock")
	b, fakeA := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "fleet", Cmd: "claude"})
	m, _ := b.readMeta("fleet")
	wsA := m.Workspace

	// A scratch herdr: up, reachable, holding nothing.
	scratch := t.TempDir()
	if err := os.WriteFile(filepath.Join(scratch, "ws-counter"), []byte("100"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/qacpeh/scratch.sock")
	t.Setenv("RHQ_FAKE_DIR", scratch)

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("fleet"); !ok {
		t.Fatal("setup: the prune deleted the fleet's meta against an empty listing — that is rangerhq-snd, not this bead")
	}

	err := b.CreateSession(NewSessionOpts{Name: "fleet", Cmd: "claude", Dir: t.TempDir()})

	if !aliveOn(t, fakeA, wsA) {
		return
	}
	now, ok := b.readMeta("fleet")
	if err == nil && ok && now.Workspace != wsA {
		t.Fatalf("a scratch herdr's empty listing let the create overwrite the fleet's own record: "+
			"meta now names %s, %s still alive on the real server", now.Workspace, wsA)
	}
}
