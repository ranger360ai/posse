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
// One disagreement is deliberate and is pinned here rather than merely
// allowed: an unstamped meta asked by an unstamped pass is one server
// talking about itself, and the write proceeds. Taking the prune's arm there
// would refuse every name on the operator's own path — `posse` from a plain
// terminal has HERDR_SOCKET_PATH unset — so the prune pays a kept file for
// the pre-field metas and the write does not. Pinning it means a SECOND
// disagreement is red, and this one cannot silently widen (rangerhq-y4z owns
// making "" and the default path one server, which retires the exception).
func TestPruneAndCreateAgreeOnEveryBoard(t *testing.T) {
	const (
		sockA = "/tmp/jeu2/A.sock"
		sockB = "/tmp/jeu2/B.sock"
	)
	cases := []struct {
		name      string
		metaSock  string
		passSock  string
		empty     bool
		mayDiffer bool // the one arm the write deliberately does not take
	}{
		{name: "unstamped meta, unstamped pass", mayDiffer: true},
		{name: "unstamped meta, unstamped pass, empty board", empty: true},
		{name: "unstamped meta, named pass", passSock: sockA},
		{name: "unstamped meta, named pass, empty board", passSock: sockA, empty: true},
		{name: "two named servers", metaSock: sockA, passSock: sockB},
		{name: "the meta's own server", metaSock: sockA, passSock: sockA},
		{name: "the meta's own server, empty board", metaSock: sockA, passSock: sockA, empty: true},
		{name: "named meta, unstamped pass", metaSock: sockA},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HERDR_SOCKET_PATH", c.passSock)
			b, fake := newTestBackend(t)
			// A bystander keeps the listing non-empty for the boards that
			// are not about emptiness, and is the workspace the empty ones
			// take away.
			mustCreate(t, b, NewSessionOpts{Name: "bystander", Cmd: "claude"})
			mustCreate(t, b, NewSessionOpts{Name: "victim", Cmd: "claude"})
			vm, ok := b.readMeta("victim")
			if !ok {
				t.Fatal("no meta for victim")
			}
			vm.Socket = c.metaSock // the server the record says wrote it
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

			switch {
			case !guarded:
				return // no socket guard fired; the per-id query decides, as always
			case c.mayDiffer:
				if err != nil {
					t.Fatalf("the write took an arm it must not: an unstamped meta asked by an unstamped "+
						"pass is one server talking about itself, and refusing it costs every name on the "+
						"operator's own path (rangerhq-jeu2's close, rangerhq-y4z): %v", err)
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

// ─── the cost of the arm the write DID take (rangerhq-7dn4) ──────────────────

// cannotAnswerFor tests `listed == 0` before it compares sockets, so the
// write refuses on an empty board even when the meta names THIS very socket
// — the one board where the socket evidence says this server is the one
// that would know. On the delete side that ordering costs a kept file. Here
// it costs the name, and it costs relaunch: a herdr restart empties the
// listing for the whole fleet at once, so every session's recovery is
// refused until some workspace exists on the board.
//
// It is not clearly wrong — an empty listing can be a herdr that just came
// up and has not re-attached its workspaces, and writing in that window
// orphans exactly what jeu2 is about. It is unpriced: the same cost was
// measured and rejected for the unstamped arm in jeu2's close, and no filed
// repro needs this arm on the write side (the arm-(3) repro's board is a
// socket MISMATCH and refuses without it). Direction is the implementer's;
// if the
// refusal stays, invert these rather than delete them.
func TestNameStaysReusableOnThisServersOwnEmptyBoard(t *testing.T) {
	t.Skip("rangerhq-7dn4: the empty-listing arm refuses the name and blocks relaunch on this server's own empty board")

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
