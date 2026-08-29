package rhq

// QA suite for the rangerhq-jeu2 fix (verifying its close, rangerhq-hew0):
// the socket guards the prune keeps in front of its per-id query, now asked
// by the write too, through one predicate — cannotAnswerFor.
//
// jeu2 was the two halves of one rule disagreeing on an identical board: the
// prune kept the meta and printed its refusal, and the create overwrote the
// same file one line later. The fix's own tests walk the three boards that
// bead filed. These walk the whole grid instead, because "one predicate" is
// a claim about every board, not about three of them.
//
// Tests marked t.Skip pin a filed bug: they encode the expected behavior and
// fail today. Remove the skip when the bead closes.

import (
	"io"
	"strings"
	"testing"
)

// ─── the two halves must agree ───────────────────────────────────────────────

// The invariant jeu2 bought, stated over the whole board rather than the
// three the bead happened to file: wherever the PRUNE keeps a meta because
// this server is not the one that would know, the WRITE must refuse it. A
// board where the delete keeps the file and the create destroys it is jeu2
// by definition, whatever the socket values that produced it.
//
// ONE disagreement remains, and it is one rule, not a list: the write
// proceeds wherever the meta names THIS very socket. The prune's extra arm —
// an empty listing — fires only on boards where the socket comparison has
// nothing to object to, and on those boards this server is the one that would
// know. The prune pays a kept file for it because a kept file is taken back
// by the next listing; the write cannot pay, because there the same refusal
// costs the NAME, and `posse relaunch` with it (rangerhq-7dn4).
//
// It used to be two. The unstamped meta was the other, and rangerhq-y4z
// retired it by making SocketID() resolve: a pass on herdr's default server
// now names that server's own path, so it stamps every meta it writes, and
// socket: "" no longer says "the default server" — it says "written before
// the field existed", which names no server at all. Both halves refuse that
// now, through cannotAnswerFor.
//
// So mayDiffer is derived, not listed: it is exactly "the meta names a server
// and it is this one". Stating it that way is what makes a THIRD disagreement
// red — a board where the sockets differ and the write still proceeds is jeu2
// itself, and no widening of the exception can hide in a table row.
func TestPruneAndCreateAgreeOnEveryBoard(t *testing.T) {
	const (
		sockA = "/tmp/jeu2/A.sock"
		sockB = "/tmp/jeu2/B.sock"
		// The socket THIS pass resolves to, whatever that is — the only way
		// to spell "the default server" in the table now that it has a path
		// rather than an empty string. Substituted inside the subtest.
		self = "@this-pass"
	)
	cases := []struct {
		name     string
		metaSock string
		passSock string
		empty    bool
	}{
		// A meta recording no socket is a pre-field legacy meta: written by
		// a binary from before the field, naming no server. Nothing on disk
		// says this server ever held it, on any board (rangerhq-y4z).
		{name: "pre-field meta, default-socket pass"},
		{name: "pre-field meta, default-socket pass, empty board", empty: true},
		{name: "pre-field meta, named pass", passSock: sockA},
		{name: "pre-field meta, named pass, empty board", passSock: sockA, empty: true},
		{name: "two named servers", metaSock: sockA, passSock: sockB},
		{name: "the meta's own server", metaSock: sockA, passSock: sockA},
		{name: "the meta's own server, empty board", metaSock: sockA, passSock: sockA, empty: true},
		{name: "named meta, default-socket pass", metaSock: sockA},
		// rangerhq-y4z's own board, which the grid could not even spell
		// before: herdr injects the concrete default-socket path into every
		// pane, so a session created inside one records it; the pass reading
		// it back from a plain terminal has $HERDR_SOCKET_PATH unset. One
		// server, and both halves must see one.
		{name: "the default server, from a pane and from a terminal", metaSock: self},
		{name: "the default server, from a pane and from a terminal, empty board", metaSock: self, empty: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HERDR_SOCKET_PATH", c.passSock)
			b, fake := newTestBackend(t)
			metaSock := c.metaSock
			if metaSock == self {
				metaSock = SocketID() // what a pane on this server would have stamped
				if metaSock == "" {
					t.Fatal("setup: this pass cannot name its own socket")
				}
			}
			// A bystander keeps the listing non-empty for the boards that
			// are not about emptiness, and is the workspace the empty ones
			// take away.
			mustCreate(t, b, NewSessionOpts{Name: "bystander", Cmd: "claude"})
			mustCreate(t, b, NewSessionOpts{Name: "victim", Cmd: "claude"})
			vm, ok := b.readMeta("victim")
			if !ok {
				t.Fatal("no meta for victim")
			}
			vm.Socket = metaSock // the server the record says wrote it
			if err := b.writeMeta(vm); err != nil {
				t.Fatal(err)
			}
			bm, _ := b.readMeta("bystander")
			// victim's workspace is off this server's listing either way:
			// the whole question is what THIS server's silence is evidence of.
			if c.empty {
				saveWSTo(t, fake, nil)
			} else {
				saveWSTo(t, fake, []fakeWS{{WorkspaceID: bm.Workspace, Label: "bystander"}})
			}

			// The delete half, on this board.
			var warn strings.Builder
			b.Warn = &warn
			if _, err := b.Sessions(); err != nil {
				t.Fatal(err)
			}
			if _, kept := b.readMeta("victim"); !kept {
				t.Fatalf("setup: the prune deleted the meta on this board — that is rangerhq-8fq or rangerhq-9nso, not this test:\n%s", warn.String())
			}
			guarded := strings.Contains(warn.String(), "does not hold their workspaces")

			// The write half, on the same board, one line later.
			b.Warn = io.Discard
			err := b.CreateSession(NewSessionOpts{Name: "victim", Cmd: "claude", Dir: t.TempDir()})

			// The one rule, derived from the board rather than listed
			// beside it: the write's exception is a meta that names a
			// server, and that server is this one.
			mayDiffer := metaSock != "" && metaSock == SocketID()

			switch {
			case !guarded:
				return // no socket guard fired; the per-id query decides, as always
			case mayDiffer:
				if err != nil {
					t.Fatalf("the write took an arm it must not: a meta naming this very socket is one "+
						"server talking about itself, and refusing it costs the name — `posse relaunch` "+
						"fleet-wide after a herdr restart, since one empties the listing for every meta at "+
						"once (rangerhq-7dn4): %v", err)
				}
			case err == nil:
				now, _ := b.readMeta("victim")
				t.Fatalf("the halves disagree on this board: the prune kept the file behind a socket guard "+
					"and the create overwrote it (meta now names %s, was %s) — that is rangerhq-jeu2\n%s",
					now.Workspace, vm.Workspace, warn.String())
			}
		})
	}
}

// ─── the arm the write no longer takes (rangerhq-7dn4) ───────────────────────

// The empty-listing arm is the prune's, and only the prune's. It used to be
// asked by the write too, ahead of the socket comparison, so the create
// refused on an empty board even when the meta named THIS very socket — the
// one board where the socket evidence says this server IS the one that would
// know. On the delete side that costs a kept file the next listing takes
// back. On the write side it cost the name, and it cost relaunch: a herdr
// restart empties the listing for the whole fleet at once, so every
// session's recovery was refused until some workspace existed on the board.
//
// The reading it was protecting — a herdr that just came up and has not
// re-attached its workspaces — does not survive its own evidence: herdr
// restores workspaces across a restart (measured, rangerhq-snd), so a server
// answering on this socket with an empty board is an empty board. Its other
// reading, "one that never held this session", is the socket comparison's,
// and the comparison decides it better. What is left is the per-id query,
// which is the strong evidence anyway.
//
// These two are the operator's ordinary path, and they are why: the board
// the last session on a server leaves behind, and the board a restart leaves
// for every meta at once.
func TestNameStaysReusableOnThisServersOwnEmptyBoard(t *testing.T) {
	t.Run("the name after the last workspace dies", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "alpha", Cmd: "claude"})
		m, _ := b.readMeta("alpha")
		saveWSTo(t, fake, nil) // its workspace died; this server holds nothing

		if err := b.CreateSession(NewSessionOpts{Name: "alpha", Cmd: "claude", Dir: t.TempDir()}); err != nil {
			t.Fatalf("a dead session's name must stay reusable on this server's own empty board "+
				"(the meta names this socket, and %s is not alive anywhere): %v", m.Workspace, err)
		}
	})

	// The same board through relaunch, which is the operator's recovery
	// command: clearDeadMeta asks mustNotOrphan one line before the unlink.
	t.Run("relaunch after a herdr restart", func(t *testing.T) {
		t.Setenv("HERDR_SOCKET_PATH", "")
		b, fake := newTestBackend(t)
		mustCreate(t, b, NewSessionOpts{Name: "alpha", Cmd: "claude"})
		saveWSTo(t, fake, nil) // herdr restarted: every workspace gone, every meta kept

		if err := b.RelaunchSession(io.Discard, RelaunchOpts{Name: "alpha"}); err != nil {
			t.Fatalf("relaunch must recover a session whose workspace is gone from its own server: %v", err)
		}
	})
}
