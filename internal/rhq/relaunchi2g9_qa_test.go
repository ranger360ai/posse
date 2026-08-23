package rhq

// rangerhq-2w1u — verifying rangerhq-i2g9, relaunch's unlink of a meta on a
// listing snapshot.
//
// The fix holds: clearDeadMeta asks mustNotOrphan one line before the
// evidence it reads would be gone, and the boards it does not walk are here.
// Two of them refuse correctly and are pinned below. The third is an ESCAPE
// (rangerhq-9jk1): RelaunchSession has FOUR meta-destroying steps, not three
// — keepRecipe is a write over a meta, and a write over a meta is as
// unrecoverable as the delete (rangerhq-cpeh) — so the guard can refuse in
// order to protect a record that the same call stack blanks one line later.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// i2g9Board is the incident's board: a second session keeps the listing
// non-empty (so the rangerhq-8fq arms stay quiet and this is about the
// snapshot, not the socket), and s1's workspace is returned.
func i2g9Board(t *testing.T) (*HerdrBackend, string, string) {
	t.Helper()
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/i2g9b/herdr.sock")
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	mustCreate(t, b, NewSessionOpts{Name: "mine"})
	mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
	m := metaOf(t, b, "s1")
	return b, fake, m.Workspace
}

// rangerhq-9jk1. HasSession answers out of Resolve, which falls back to
// FOREIGN workspaces by label — so a workspace merely wearing the session's
// name answers true while the session's own workspace is missing from this
// listing snapshot (the rangerhq-9nso condition i2g9 is about). Relaunch
// then takes the kill arm on that stranger, the create refuses because the
// meta names a workspace herdr says is alive, and keepRecipe writes the
// meta back blank on the way out: the only record of the live session,
// destroyed by the same error that says it is refusing to destroy it.
//
// The board needs a herdr workspace labelled <name> this RHQ_HOME has no
// meta for — an earlier incident's orphan (it wears the label), a second
// RHQ_HOME on one herdr (rangerhq-snd), or one an operator labelled.
func TestRelaunchMustNotBlankTheRecordOfALiveSession(t *testing.T) {
	setup := func(t *testing.T) (*HerdrBackend, string, string) {
		t.Helper()
		b, fake, ws := i2g9Board(t)
		saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "wForeign", Label: "s1"}))
		hideFromTheListing(t, fake, ws)
		os.Remove(filepath.Join(fake, "calls.log"))
		return b, fake, ws
	}

	// The headline: whatever relaunch decides, the workspace id of a live
	// session may not leave the disk. It is the one field the operator needs
	// to get the session back, and state/ is outside git.
	t.Run("the record survives the refusal", func(t *testing.T) {
		b, _, ws := setup(t)

		var out strings.Builder
		if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err == nil {
			t.Fatalf("expected a refusal — s1's workspace is alive:\n%s", out.String())
		}
		m, ok := b.readMeta("s1")
		if !ok || m.Workspace != ws {
			t.Fatalf("the only record of live workspace %s was destroyed: meta now %+v", ws, m)
		}
		if alive, err := b.H.WorkspaceAlive(ws); err != nil || !alive {
			t.Errorf("the live session was closed without ever being landed (alive=%v err=%v)", alive, err)
		}
	})

	// The refusal tells the operator to retry, and the retry must not be the
	// thing that finishes the job: a meta blanked in pass 1 names no
	// workspace, so pass 2 clears it without asking and creates a SECOND
	// workspace under the same label — i2g9's own harm, err == nil.
	t.Run("the advised retry does not orphan the live session", func(t *testing.T) {
		b, _, ws := setup(t)

		var out strings.Builder
		_ = b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
		out.Reset()
		err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})

		m, _ := b.readMeta("s1")
		alive, aerr := b.H.WorkspaceAlive(ws)
		if aerr != nil {
			t.Fatal(aerr)
		}
		if alive && m.Workspace != ws {
			t.Fatalf("the retry orphaned a live session: the meta names %s, %s is still running its agent (err=%v)\n%s",
				m.Workspace, ws, err, out.String())
		}
	})

	// The destructive half of relaunch may only act on the workspace the
	// meta names. Resolving by label is right for `posse kill` on a foreign
	// row; here it closes a live workspace nobody matched to this session,
	// and without a landing turn.
	t.Run("no workspace is closed that was never matched to the meta", func(t *testing.T) {
		b, fake, _ := setup(t)

		var out strings.Builder
		_ = b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
		if log := calls(t, fake); strings.Contains(log, "workspace close wForeign") {
			t.Errorf("relaunch closed a workspace it never matched to s1's meta:\n%s", log)
		}
	})
}

// The arms clearDeadMeta inherits from mustNotOrphan beyond the incident's
// own. Both are correct refusals — this pass cannot answer for the meta, so
// it may not clear it — and both are pinned because relaunch is a THIRD
// caller of that predicate and nothing else asserts what it does with a
// refusal. The empty-board arm is the same shape and is rangerhq-7dn4.
func TestRelaunchRefusesOnEveryBoardItCannotAnswerFor(t *testing.T) {
	// rangerhq-yt1p: herdr's allocator recomputes from the live set at every
	// server start, so a restart hands the meta's id to somebody else. That
	// answer is about a stranger's workspace and proves nothing about this
	// session — so it is not death, and the meta stays.
	t.Run("a stranger holds the recorded id after a restart", func(t *testing.T) {
		sock := idProbeSocket(t)
		b, fake := newTestBackend(t)
		agentPerLaunch(t, fake)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
		ws := metaOf(t, b, "s1").Workspace

		newGeneration(t, sock)
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: ws, Label: "somebody-else"}})

		var out strings.Builder
		err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
		if err == nil {
			t.Fatalf("cleared a meta on an answer about a stranger's workspace:\n%s", out.String())
		}
		if m, ok := b.readMeta("s1"); !ok || m.Workspace != ws {
			t.Fatalf("meta lost to a recycled id: %+v", m)
		}
		// The operator has to be able to act on it: the refusal names the
		// session, says nothing was closed, and says how to discard it.
		for _, want := range []string{"s1", "NOT closed", "by hand"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal missing %q: %v", want, err)
			}
		}
	})

	// rangerhq-8fq: the meta was written against another herdr, so this
	// server's listing says nothing about it and its silence is not death.
	t.Run("the meta belongs to another herdr", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "/tmp/i2g9b/A.sock")
		b, fake := newTestBackend(t)
		agentPerLaunch(t, fake)
		mustCreate(t, b, NewSessionOpts{Name: "mine"})
		mustCreate(t, b, NewSessionOpts{Name: "s1", Cmd: "claude"})
		ws := metaOf(t, b, "s1").Workspace

		t.Setenv("HERDR_SOCKET_PATH", "/tmp/i2g9b/B.sock")
		saveWSTo(t, fake, []fakeWS{{WorkspaceID: "wOther", Label: "theirs"}}) // B holds one of its own

		var out strings.Builder
		err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
		if err == nil {
			t.Fatalf("relaunched a session whose meta names another herdr:\n%s", out.String())
		}
		if m, ok := b.readMeta("s1"); !ok || m.Workspace != ws {
			t.Fatalf("another herdr's record destroyed: %+v", m)
		}
		if log := calls(t, fake); strings.Contains(log, "workspace close") {
			t.Errorf("closed something on the wrong server:\n%s", log)
		}
	})
}
