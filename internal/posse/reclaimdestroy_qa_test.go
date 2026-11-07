//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-43mxx — ADR 0080 D10: a proven-dead seat on a
// non-herdr occupant is DESTROYED, not only forgotten.
//
// WHAT WAS LEAKING. On Rex a seat ends by its block closing, and the session
// then stays on the server holding no terminal block. The occupant's List()
// drops it, ADR 0076 §5 reads that absence as death, and the prune unlinks
// the record — leaving a session nothing lists and nothing names, still
// counted toward the server's `--max-sessions`, a cap SHARED by every posse
// home on that server (ADR 0080 D2). MEASURED 2026-11-06 against a scratch
// server at `--max-sessions 3`: three such sessions filled the cap, the
// fourth create was refused `409 conflict`, and one destroy freed the slot
// (docs/notes.d/ranger-base-43mxx.md).
//
// EVERY ARM HERE IS ABOUT AN ORDER OR A SILENCE, because nothing about this
// bug is visible in a green run or on a listing:
//
//  1. the destroy comes BEFORE the unlink, and under the launcher lock. The
//     record is the only thing that names the id, so a pass that unlinks
//     first has nothing left to destroy with — and the leak then has no
//     record of itself anywhere.
//  2. a destroy that FAILS keeps the record. Unlinking after a destroy that
//     did not land is the leak made permanent, and the next pass can only
//     retry if the record is still there to retry from.
//  3. the one refusal that is not a failure: the server saying it does not
//     know the id. That is positive evidence the destroy already happened,
//     it is read off rex's own measured sentence, and an occupant that
//     cannot read its own refusal gets the keep ("not available" is never
//     "no").
//  4. herdr is untouched. A dead herdr workspace is gone, so there is no
//     husk to destroy, and `workspace close` on an id herdr has recycled
//     would close a stranger's workspace.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pruneUntil runs listing passes until cond() holds, and fails naming the
// warning stream when it never does.
//
// WHY A LOOP AND NOT ONE PASS. reclaim's answer to a contended launcher lock
// is to spare the file and let the next quiet pass prune (ADR 0011 §1), and
// this suite's own concurrency produces that contention: a lock, release and
// re-lock cycle on a file no other test can even name reads EWOULDBLOCK
// intermittently while the package's other tests are forking their fakes,
// because flock is per open file description and a child between fork and
// exec still holds a reference to the one the parent just closed.
//
// MEASURED 2026-11-06, and it is NOT this bead's code: a throwaway test that
// did nothing but tryLockLaunches / Release five times over its own
// newTestBackend home failed the same way under `-count=5` beside four other
// tests (the close comment on ranger-base-43mxx has the run). In production
// that reads as one spared file and the documented "the next quiet pass
// prunes it"; in a test it would make every arm here flaky over a fact about
// the box. So the pass is asked again — and a prune that genuinely refuses
// still never satisfies cond, which is what keeps the arms falsifiable.
func pruneUntil(t *testing.T, b *HerdrBackend, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 20; i++ {
		if _, err := b.Sessions(); err != nil {
			t.Fatal(err)
		}
		if cond() {
			return
		}
	}
	t.Fatalf("after 20 listing passes, %s\n%s", what, warnBuf(t, b).String())
}

// metaGone is "this record is no longer on disk", as a cond for pruneUntil.
func metaGone(b *HerdrBackend, name string) func() bool {
	return func() bool {
		_, ok := b.readMeta(name)
		return !ok
	}
}

// ─── arm 1: destroy, then unlink, under the lock ─────────────────────────────

func TestQAAProvenDeadNonHerdrSeatIsDestroyedBeforeItsRecordIsUnlinked(t *testing.T) {
	t.Parallel()
	b, stub := muxMetaRexBoard(t, "husk", "01a1902c-husk", false)
	// The witness is taken INSIDE the destroy, because the ordering claim is
	// about an instant and not about an end state: both orders end with the
	// record gone and the seat closed, and only this tells them apart.
	var sawRecord, sawLock bool
	stub.onClose = func(Seat) error {
		_, err := os.Stat(b.metaPath("husk"))
		sawRecord = err == nil
		// And the act is the lock's. flock is per open file description, so
		// a second open+LOCK_NB from this very process conflicts with the
		// one reclaim is holding — which is what makes "could not take it"
		// the positive reading here (retirecmd_qa_test.go uses the same
		// idiom from the other side).
		if lock, _ := tryLockLaunches(b.App); lock != nil {
			lock.Release()
		} else {
			sawLock = true
		}
		return nil
	}
	pruneUntil(t, b, "the prune destroyed no seat at all — a proven-dead seat on a non-herdr occupant was forgotten and not destroyed, which leaves a session holding one of the server's --max-sessions slots for every home on it (ADR 0080 D10)",
		func() bool { return len(stub.closes) > 0 })
	if len(stub.closes) != 1 {
		t.Fatalf("the prune destroyed %d seat(s), want 1", len(stub.closes))
	}
	// The seat the RECORD names, both ids, and not a session id lifted out
	// of it: the record is what aims the destroy (HerdrMeta.Seat).
	if want := (Seat{Session: "01a1902c-husk", Pane: "block-1"}); stub.closes[0] != want {
		t.Errorf("the destroy was aimed at %+v, want %+v", stub.closes[0], want)
	}
	if !sawRecord {
		t.Error("the record was already unlinked when the destroy ran — the record is the only thing that names that session id, so this order leaks the seat AND loses the evidence that it leaked (ADR 0080 D10)")
	}
	if !sawLock {
		t.Error("the destroy ran with the launcher lock free — reclaim's whole shape is that the act is taken under ADR 0011 §1's lock with the proof re-taken inside it, and a destroy outside it acts on evidence read before it")
	}
	if _, err := os.Stat(b.metaPath("husk")); !os.IsNotExist(err) {
		t.Errorf("the record survived a destroy that succeeded (%v) — the prune proved death, destroyed the seat and then kept the file, so the session is gone and its name is held forever", err)
	}
}

// ─── arm 2: a failed destroy keeps the record, and the next pass retries ─────

func TestQAAFailedDestroyKeepsTheRecordAndRetries(t *testing.T) {
	t.Parallel()
	b, stub := muxMetaRexBoard(t, "husk", "01a1902c-husk", false)
	stub.onClose = func(Seat) error { return errors.New("server refused the destroy") }
	pruneUntil(t, b, "the prune never reached the destroy at all", func() bool { return len(stub.closes) > 0 })
	if _, ok := b.readMeta("husk"); !ok {
		t.Fatal("a destroy that FAILED still unlinked the record — the record is the only thing that names that session id, so the seat is now leaked with nothing left to address it by, and no later pass can retry (ADR 0080 D10)")
	}
	// A kept file is spared SILENTLY unless the pass says why, and this
	// sparing is the one an operator has to be able to act on: it names a
	// server that is refusing.
	warn := warnBuf(t, b).String()
	for _, want := range []string{"01a1902c-husk", "server refused the destroy", "ADR 0080 D10"} {
		if !strings.Contains(warn, want) {
			t.Errorf("the sparing does not name %q:\n%s", want, warn)
		}
	}
	// And the retry is real, not a promise: the same board, asked again.
	pruneUntil(t, b, "the destroy was attempted once and never again — a kept record whose destroy is never retried is the leak with one extra file",
		func() bool { return len(stub.closes) > 1 })
	if _, ok := b.readMeta("husk"); !ok {
		t.Fatal("the record went on a pass whose destroy also failed")
	}
	// The failing arm's own control: once the destroy lands, the record goes.
	// Without it every assertion above is satisfied by a prune that stopped
	// pruning at all.
	stub.onClose = nil
	pruneUntil(t, b, "the record survived every pass once the destroy succeeded — the keep is unconditional, so nothing is ever reclaimed", metaGone(b, "husk"))
}

// ─── arm 3: rex's own `not found` is the destroy already having happened ─────

// The end-to-end half, over the real occupant and the real sentence. The stub
// above cannot reach it: reading a refusal is the occupant's own
// (MuxCloseGone), and the fake rex answers in the CLI's measured words.
//
// The middle row is the one the whole bead is about — a husk the server DOES
// hold, destroyed — and it is the row that reads the server's own state
// afterwards rather than posse's.
func TestQAARexHuskIsDestroyedAndOnlyNotFoundCountsAsAlreadyDestroyed(t *testing.T) {
	t.Parallel()
	const husk = "01a1902c-husk"
	for _, c := range []struct {
		name string
		// seat is the husk session as the server holds it: a real husk has
		// NO terminal block, which is what makes List() drop it and §5 read
		// it as dead.
		seat   bool
		fail   string
		pruned bool
		// left is whether the server still holds the husk session when the
		// pass is over.
		left bool
	}{
		// The husk itself: present on the server, holding no terminal block,
		// and the destroy lands. Without D10 this row ends with the record
		// gone and the session still there, which is the leak.
		{name: "a husk the server holds is destroyed", seat: true, pruned: true, left: false},
		// An id the server does not hold at all. No lever is set, so the
		// refusal is the FAKE's own — the measured sentence, exit 3 — which
		// is also what makes this row a pin on the fake's fidelity.
		{name: "an id the server does not hold", pruned: true},
		// Any OTHER refusal is a destroy that did not land. Same exit code,
		// same stream, same `Error:` shape — only the sentence differs, which
		// is the whole of what rexSessionGone may read. The session is still
		// there, and so is the record that names it.
		{name: "another refusal, same shape", seat: true, fail: "kill\tthe daemon is shutting down", pruned: false, left: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			mux, dir := newTestRexMux(t)
			b.Slot = mux
			// One session of somebody else's, WITH a terminal block, so the
			// occupant's listing is not empty — the prune's own belt would
			// otherwise answer every row here (emptyBoard).
			sessions := []fakeRexSession{{SessionID: "01a1902c-other", Label: "somebody-else",
				Blocks: []fakeRexBlock{{BlockID: "block:other", Flavor: RexTerminalFlavor}}}}
			if c.seat {
				sessions = append(sessions, fakeRexSession{SessionID: husk, Label: "husk"})
			}
			rexSeedSessions(t, sessions...)
			if c.fail != "" {
				if err := os.WriteFile(filepath.Join(dir, "rex-fail"), []byte(c.fail), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := b.writeMeta(&HerdrMeta{
				Name: "husk", Workspace: husk, Pane: "block:husk", Emoji: "🤠", Agent: "scout",
				Mux: MuxRex, Socket: mux.Server().Addr,
				Launched: time.Now().Add(-2 * PruneGrace),
			}); err != nil {
				t.Fatal(err)
			}
			// Until the pass has actually asked for the destroy: a listing
			// that spared the record on the launcher lock has not reached
			// the branch under test (pruneUntil's doc).
			asked := func() bool {
				for _, c := range rexCalls(t) {
					if strings.Contains(c, "kill") && strings.Contains(c, husk) {
						return true
					}
				}
				return false
			}
			pruneUntil(t, b, "the prune never asked rex to destroy the seat", asked)
			if c.pruned {
				// The unlink is in the same pass as the destroy, so one more
				// pass is only the lock-sparing retry above.
				pruneUntil(t, b, "the record was kept after a destroy that counts as done", metaGone(b, "husk"))
			}
			if _, kept := b.readMeta("husk"); kept == c.pruned {
				if c.pruned {
					t.Errorf("the record was kept: rex said it does not hold that session, which is positive evidence the destroy has already happened — keeping the record on it makes the meta immortal, since every later pass proves the same death, asks for the same destroy and is refused the same way\n%s", warnBuf(t, b).String())
				} else {
					t.Error(`a destroy rex REFUSED unlinked the record anyway — only session target "<id>" not found says the seat is gone, and every other refusal is a seat still there with nothing left to name it (rexSessionGone)`)
				}
			}
			if !c.seat {
				return
			}
			// The server's own state, which is the half posse's listing can
			// never show: did the session actually go?
			var left []fakeRexSession
			raw, err := os.ReadFile(filepath.Join(dir, "rex-sessions.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &left); err != nil {
				t.Fatal(err)
			}
			var holds bool
			for _, s := range left {
				if s.SessionID == husk {
					holds = true
				}
			}
			if holds != c.left {
				if c.left {
					t.Errorf("the server no longer holds %s after a destroy it REFUSED — the fake is not answering the lever, so this row measures nothing", husk)
				} else {
					t.Errorf("the server still holds %s after the prune — the record is gone and the session is not, which is the whole leak: it goes on counting toward --max-sessions for every home on that server (ADR 0080 D10)", husk)
				}
			}
		})
	}
}

// And the sentence itself, read directly: the branch above is one substring
// pair over text posse does not control, so the readings it must and must not
// make are pinned where they can be seen.
func TestQAARexSessionGoneReadsOnlyRexsOwnNotFound(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		gone bool
	}{
		// MEASURED 2026-11-06, rex 0.0.1793377904, `rex kill <id>` on a
		// scratch server: this line on stderr, nothing on stdout, exit 3.
		// It is wrapped the way Rex.callErr wraps it.
		{"the measured refusal", Die(`rex kill 01a1902c-husk: Error: session target "01a1902c-husk" not found`), true},
		{"nothing failed at all", nil, false},
		// The reading that must never be made: a hang is posse never having
		// reached the server, and read as "gone" it would skip the destroy
		// on the one answer that proves nothing. This arm pins the OUTCOME
		// and not the line that produces it — a hang's text is its argv and
		// its wait, so the substring test refuses it on its own and
		// rexSessionGone's IsHerdrHang guard is belt (said so there). What
		// the arm does catch is a rewrite that keys on the EXIT CODE, where
		// a killed child and a refusal are the same nonzero.
		{"a blown deadline", &HerdrHangError{Argv: []string{"rex", "kill", "01a1902c-husk"}, Limit: time.Minute, Waited: time.Minute}, false},
		{"another refusal, same shape", Die(`rex kill 01a1902c-husk: Error: the daemon is shutting down`), false},
		// The neighbouring sentence, over the OTHER target kind rex spells
		// (rex.go's head comment, item 2): a block that is gone says nothing
		// about its session.
		{"a block that is gone", Die(`rex block call -b block:husk program_status: Error: block target "block:husk" not found`), false},
	} {
		if got := rexSessionGone(c.err); got != c.gone {
			t.Errorf("%s: rexSessionGone = %v, want %v (err: %v)", c.name, got, c.gone, c.err)
		}
	}
}

// ─── arm 4: herdr is untouched ───────────────────────────────────────────────

// A dead herdr workspace is gone — there is no husk — and herdr RECYCLES
// workspace ids across a server restart or handoff (rangerhq-6bg7), so a
// close on a recorded id the prune has just proved absent is the one call
// that could reach a stranger's workspace. The gate is the record's own mux
// name, and this is the arm that keeps it.
//
// It runs over the real herdr occupant and reads its CALLS, because herdr's
// half of the prune is herdr's own per-id query behind ADR 0076 §4's door
// (idEvidence -> workspaceGet): a stub occupant cannot reach the arm at all,
// and a prune that refused for that reason would pass a "no close" assertion
// while measuring nothing.
func TestQAAThePruneNeverDestroysAHerdrWorkspace(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	// A board holding one workspace of somebody else's, so the listing is
	// not empty — the prune's own belt would otherwise answer this arm
	// (emptyBoard) — and not holding workspace 7, which is what the per-id
	// query answers not_found for.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "9", Label: "somebody-else"}})
	if err := b.writeMeta(&HerdrMeta{
		Name: "dead", Workspace: "7", Pane: "%1", Emoji: "🤠", Agent: "scout",
		Socket: SocketID(), Launched: time.Now().Add(-2 * PruneGrace),
	}); err != nil {
		t.Fatal(err)
	}
	// The control first: this record really was pruned, so the claim below
	// is about a prune that ran and not about one that refused for some
	// other reason.
	pruneUntil(t, b, "the herdr record was not pruned at all, so this arm measures nothing: it is the wrong board, not herdr being spared", metaGone(b, "dead"))
	if log := calls(t, fake); strings.Contains(log, "workspace close") {
		t.Errorf("the prune closed a workspace on a HERDR record — a dead herdr workspace is already gone, and herdr re-issues workspace ids across a restart or handoff (rangerhq-6bg7), so the id in a proven-dead record is exactly the one a stranger's workspace may be holding now (ADR 0080 D10: \"herdr needs nothing here\"):\n%s", log)
	}
}
