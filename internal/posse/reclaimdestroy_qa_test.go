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
	"fmt"
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
//
// WHY THE BUDGET IS WALL CLOCK AND NOT A PASS COUNT (ranger-base-6mpke).
// This loop was `for i := 0; i < 20; i++` with nothing between the passes,
// and that bound read like twenty chances at a 5.31% event
// (forkflockrace_live_test.go's measured rate) when it was in fact ONE. What
// is being ridden out is a WINDOW in time — fork to execve in a sibling
// goroutine's child — and twenty back-to-back passes do not sample twenty
// instants, they sample one.
//
// MEASURED 2026-11-07, this box (darwin 25.6.0, go1.26.5), over this very
// fixture: one listing pass costs 167-196µs, so the whole twenty-pass budget
// spanned 1.7-3.9ms. Held deliberately from a second open file description,
// a launcher lock held for 10ms — far shorter than any window worth worrying
// about — exhausted the entire budget, and so did 50ms and 200ms, every one
// of them deterministically. TestQAAThePruneRetryBudgetRidesOutAHeldLaunchLock
// pins a 300ms hold, for the reason it gives there.
// That is the flake ranger-base-6mpke was filed off: a full `make test` at a
// one-minute load of 49-75 spent passes 2 through 20 inside one such window,
// and arm 2 reported "the destroy was attempted once and never again" over a
// prune that was working — a red arm 1, a ~680s run discarded, and the next
// reader sent after a merge defect that was not there.
//
// So the loop spends TIME, not attempts: pruneUntilBudget of it.
//
// AND A SPARED PASS IS NOT A PASS — which is the fix, and the gap below is
// not. A pass reclaim ended on the launcher lock never reached the branch
// under test at all, so it does not count against pruneUntilReaches, and the
// only thing a window of contention can exhaust is the wall clock. The reach
// count is then what a broken prune cannot outrun: with the lock free every
// pass counts, and a genuine break fails in milliseconds rather than at the
// deadline. Both halves are needed and each is pinned on its own
// (MUTATION-CHECKED 2026-11-07, below).
//
// The GAP is not part of that argument and is not asserted as if it were. A
// budget that spun this loop flat out for the whole of a window would be
// adding load to the box whose load produced the window — so the passes are
// spaced, and what the pin holds is that riding out a known hold costs tens
// of passes and not tens of thousands.
const (
	// pruneUntilBudget is the wall clock the loop will spend riding out a
	// contended launcher lock. It only has to outlast a fork-to-exec window
	// on a loaded box, and seconds are orders of magnitude more than that.
	// It is spent in full only on a box where that lock is contended for the
	// whole of it — which is a broken box, and is reported as one rather
	// than as a claim about the prune.
	pruneUntilBudget = 15 * time.Second
	// pruneUntilGap spaces the passes. Not for the sampling — the reach
	// accounting already means a contended lock cannot exhaust anything but
	// the clock — but because the alternative is burning a core flat out
	// against contention this box is already short of.
	pruneUntilGap = 5 * time.Millisecond
	// pruneUntilReaches is how many passes that actually REACHED reclaim's
	// decision may fail to satisfy cond before this is a break and not a
	// flake. It is the old bound, applied to the only passes it was ever a
	// statement about.
	pruneUntilReaches = 20
)

// lockSparedSentence is the half of reclaim's lock sparing that pruneUntil
// keys on to tell "this pass never got there" from "this pass got there and
// answered no". It is pinned, in the same test as the budget, because a
// rename on reclaim's side would break no build: it would silently turn every
// spared pass back into a counted one, and the budget back into the four
// milliseconds ranger-base-6mpke was filed off.
const lockSparedSentence = "the launch lock was not taken"

func pruneUntil(t *testing.T, b *HerdrBackend, what string, cond func() bool) {
	t.Helper()
	gaveUp, _, err := prunePasses(b, warnBuf(t, b), pruneUntilBudget, cond)
	if err != nil {
		t.Fatal(err)
	}
	if gaveUp != "" {
		t.Fatalf("%s%s\n%s", gaveUp, what, warnBuf(t, b).String())
	}
}

// prunePasses is pruneUntil without a *testing.T, so that the budget's own
// two guards can be MEASURED rather than asserted about from outside a helper
// that fatals. It returns "" when cond held, and otherwise the clause naming
// why it gave up, which the caller's `what` completes.
//
// The budget is a parameter for one reason: the deadline guard's own pin
// would otherwise cost pruneUntilBudget of real suite wall to reach, and a
// guard too expensive to pin is a guard nobody has seen fire.
func prunePasses(b *HerdrBackend, warn *syncBuf, budget time.Duration, cond func() bool) (gaveUp string, passes int, err error) {
	start := time.Now()
	deadline := start.Add(budget)
	var reached int
	for {
		at := len(warn.String())
		if _, err := b.Sessions(); err != nil {
			return "", passes, err
		}
		passes++
		if cond() {
			return "", passes, nil
		}
		// Only a pass that got past the lock is evidence about the prune.
		if !strings.Contains(warn.String()[at:], lockSparedSentence) {
			reached++
		}
		if reached >= pruneUntilReaches {
			return fmt.Sprintf("after %d listing passes in %v, %d of which reached reclaim's own decision, ",
				passes, time.Since(start).Round(time.Millisecond), reached), passes, nil
		}
		if time.Now().After(deadline) {
			// It reports what it counted rather than asserting nobody got
			// there. A deadline reached with reached > 0 is rare — it needs
			// fewer than pruneUntilReaches passes to have got past the lock
			// in pruneUntilBudget — but it is reachable, and a give-up that
			// said "NOT ONE" over nineteen that did would send its reader
			// exactly where this bead's own flake sent one.
			if reached > 0 {
				return fmt.Sprintf("after %d listing passes over %v, %d of which reached reclaim's own decision before the clock ran out, ",
					passes, time.Since(start).Round(time.Millisecond), reached), passes, nil
			}
			return fmt.Sprintf("after %d listing passes over %v, NOT ONE of which reached reclaim's own decision — "+
				"every one was spared on the launcher lock, so this is a box on which that lock is never free, and not a claim about the prune (ranger-base-6mpke), ",
				passes, time.Since(start).Round(time.Millisecond)), passes, nil
		}
		time.Sleep(pruneUntilGap)
	}
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
		// refusal is the FAKE's own — the measured sentence, exit 3. This
		// row does NOT pin that: a fake that answered 0 for an unknown id
		// (which is what it did before this bead) ends this row the same
		// way, through the destroy-succeeded path instead. The fake's own
		// answer is pinned below, where it can be seen.
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

// The fake's own answer for an id the server does not hold, which is the one
// thing the rows above cannot witness: each of them ends with the record gone
// whether the kill was refused or succeeded. MEASURED 2026-11-06 on rex
// 0.0.1793377904 — `session target "<id>" not found` on stderr, exit 3 — and
// the fake answered exit 0 for every id until this bead, which left the
// production branch that reads that refusal unreachable from the suite.
func TestQAATheFakeRexRefusesAKillOfAnIdItDoesNotHold(t *testing.T) {
	t.Parallel()
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: "01a1902c-other", Label: "somebody-else",
		Blocks: []fakeRexBlock{{BlockID: "block:other", Flavor: RexTerminalFlavor}}})
	err := m.Close(Seat{Session: "01a1902c-husk", Pane: "block:husk"})
	if err == nil {
		t.Fatal("the fake destroyed an id it does not hold and called it success — the real CLI refuses with exit 3, so the branch that reads that refusal (rexSessionGone, ADR 0080 D10) cannot be reached from this suite at all and every pin on it passes through the success path instead")
	}
	if !strings.Contains(err.Error(), `session target "01a1902c-husk" not found`) {
		t.Errorf("the fake's refusal is %q, not the sentence the real CLI prints", err)
	}
	// And the occupant reads its own refusal, which is the wiring: the
	// interface is optional and reached by a type assertion, so a rename on
	// either side would simply stop matching.
	if !m.CloseSaysGone(err) {
		t.Error("the occupant does not read its own `not found` as gone")
	}
	var gone MuxCloseGone = m
	if !gone.CloseSaysGone(err) {
		t.Error("RexMux does not satisfy MuxCloseGone through the interface the prune asserts on")
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

// ─── the destroy's PLACE: the half the four arms above do not reach ──────────
//
// ranger-base-8rg06, verifying ranger-base-43mxx. destroyRecordedSeat
// carries the ordering claim in its own words — "it goes HERE and nowhere else
// in this function: ... one that destroys before the re-proof destroys on
// evidence read outside the lock" — and the four arms above pin only one end of
// it. MEASURED 2026-11-07 on this tree: moving the call from BELOW the
// `prunable` re-proof to ABOVE it, and again above `cannotAnswerFor` and the
// recipe check, left the whole arm-1 internal/posse package green. Arm 1 above
// witnesses that the record still exists and the lock is held when the destroy
// runs; neither says the re-proof had already answered.
//
// WHAT BEHAVES DIFFERENTLY IF THIS PIN IS DELETED (ADR 0006 §7): the prune
// destroys a LIVE Rex seat. reclaim is reached on evidence read outside the
// launcher lock (rangerhq-3a5t, and reclaim's own doc), so between that
// reading and the unlink a create can have put the seat back — and a destroy
// taken before the re-proof ends the session the re-proof was about to spare,
// while prunable's refusal then KEEPS the record, so posse goes on listing a
// seat it has just killed.

// reclaimRaceMux is an occupant whose listing re-grows the seat exactly while
// the launcher lock is HELD: that is the listing reclaim takes INSIDE the lock
// and none of the listings the pass takes outside it. flock is per open file
// description, so a second open+LOCK_NB from this very process conflicts with
// the one reclaim is holding — arm 1 above reads the same fact from the other
// side.
//
// It is NOT a List() call count, deliberately. A fixture that re-grew the seat
// on the Nth listing would stop aiming at reclaim the moment a listing was
// added or removed anywhere in the pass, and the arm would then pass by never
// reaching the branch under test at all.
type reclaimRaceMux struct {
	*muxMetaStub
	app  *App
	seat Seat
	// race is false in the control row, which is the same rig with the lever
	// off: the seat is absent from every listing and the destroy must happen.
	race bool
}

func (m *reclaimRaceMux) List() ([]SeatInfo, error) {
	seats, err := m.muxMetaStub.List()
	if err != nil || !m.race {
		return seats, err
	}
	if lock, _ := tryLockLaunches(m.app); lock != nil {
		lock.Release()
		return seats, nil // free: a listing taken outside reclaim
	}
	return append(append([]SeatInfo{}, seats...), SeatInfo{Seat: m.seat, Label: "husk"}), nil
}

func TestQAATheDestroyIsNotTakenOnEvidenceTheReProofWouldSpare(t *testing.T) {
	t.Parallel()
	const husk = "01a1902c-husk"
	for _, c := range []struct {
		name string
		race bool
	}{
		{name: "the re-proof under the lock finds the seat back", race: true},
		{name: "control: absent outside the lock and inside it", race: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, stub := muxMetaRexBoard(t, "husk", husk, false)
			b.Slot = &reclaimRaceMux{muxMetaStub: stub, app: b.App,
				seat: Seat{Session: husk, Pane: "block-1"}, race: c.race}
			if !c.race {
				// The control first, because it is what makes the row below
				// an absence of something this rig can produce: with the
				// lever off, the same board destroys the seat and unlinks
				// the record.
				pruneUntil(t, b, "the control row destroyed nothing, so the rig cannot produce a destroy at all and the raced row measures nothing",
					func() bool { return len(stub.closes) > 0 })
				pruneUntil(t, b, "the control row kept the record after a destroy that succeeded", metaGone(b, "husk"))
				return
			}
			// The pass is asked again until the WITNESS appears, for
			// pruneUntil's reason pointed at this rig's own lever: the lock
			// flake its doc measures makes tryLockLaunches answer "held"
			// when nobody holds it, and read by List() above that re-grows
			// the seat for a listing taken OUTSIDE reclaim — so the outer
			// pass spares the record itself and never reaches the branch
			// under test. That costs the witness and never the assertion: a
			// pass that did not reach reclaim also took no destroy. Asking
			// again is what makes the arm falsifiable rather than flaky, and
			// a prune that genuinely never spares on its own re-proof still
			// never produces the witness.
			//
			// It is pruneUntil's own budget and not a hand-rolled count, for
			// pruneUntil's own reason (ranger-base-6mpke): twenty back-to-back
			// passes here sampled one instant of the same window, so this loop
			// was flaky in exactly the way the arms it guards were. The
			// positive witness is seatEvidence's own sentence carried out as
			// the sparing reason — without it every assertion below is
			// satisfied by a pass that never reached reclaim.
			witness := "seat " + husk + " is alive"
			pruneUntil(t, b, "the prune never spared this record on its own re-proof, so this arm never reached reclaim at all",
				func() bool { return strings.Contains(warnBuf(t, b).String(), witness) })
			if n := len(stub.closes); n != 0 {
				t.Errorf("the prune destroyed the seat %d time(s) although its own re-proof under the launcher lock found it ALIVE — the destroy is being taken on evidence read outside the lock, which is the reading reclaim re-takes precisely because a create can be in flight (rangerhq-3a5t, ADR 0080 D10). A live seat is ended, and prunable's refusal then keeps the record, so posse goes on listing a session it has just killed", n)
			}
			if _, ok := b.readMeta("husk"); !ok {
				t.Error("the record was unlinked although the re-proof spared it")
			}
		})
	}
}

// holdLaunchLock takes the launcher lock for a test that means to HOLD it,
// retrying the probe.
//
// Because the probe this test makes is subject to the race this test is
// pinning (ranger-base-6mpke, and the irony is the point): a sibling
// goroutine's fork duplicates some other test's launcher-lock fd into its
// child, and tryLockLaunches here reads EWOULDBLOCK on a lock nobody holds.
// MEASURED 2026-11-07: one such read in nine runs of this file, as a 0.01s
// red saying "could not take the launcher lock to hold it" — the arms below
// ride out that window for the PRUNE's probe and would have flaked on their
// own. The retry is bounded, so a lock a real launcher holds for the whole
// budget still fails, and says which it was.
func holdLaunchLock(t *testing.T, b *HerdrBackend) *LaunchLock {
	t.Helper()
	deadline := time.Now().Add(pruneUntilBudget)
	for {
		lock, why := tryLockLaunches(b.App)
		if lock != nil {
			return lock
		}
		if time.Now().After(deadline) {
			t.Fatalf("this test could not take the launcher lock to hold it, over %v of trying: %s", pruneUntilBudget, why)
		}
		time.Sleep(pruneUntilGap)
	}
}

// ─── the retry budget itself: what it rides out, and what it must not ───────
//
// ranger-base-6mpke. pruneUntil's budget IS the mitigation for the accepted
// fork/flock false-busy (ranger-base-a40d9, operator ruling ACCEPT on
// ranger-base-5mulm), and for as long as it was a bare count of twenty
// back-to-back passes it was a mitigation that covered 1.7-3.9ms of wall
// clock. It read like an arbitrary 20 and was in fact the whole of it.
//
// WHAT BEHAVES DIFFERENTLY IF THIS PIN IS DELETED (ADR 0006 §7): nothing in
// production — the subject here is the suite. What goes is the only statement
// anywhere that the budget's unit is TIME, and the next edit that spells it as
// a pass count again, or drops the gap between passes, or starts counting
// lock-spared passes against the reach guard, lands green and reddens arm 1
// at a load average instead.
func TestQAAThePruneRetryBudgetRidesOutAHeldLaunchLock(t *testing.T) {
	t.Parallel()

	// ── it rides out a window no count of passes could span ──
	//
	// MEASURED 2026-11-07: a listing pass over this fixture costs 167-196µs,
	// so twenty back-to-back passes span 1.7-3.9ms, and a launcher lock held
	// for as little as 10ms exhausted all twenty — deterministically, at
	// every hold from 10ms to 200ms. The hold here is taken from a SECOND
	// open file description in this very process, which is how reclaim's
	// probe reads EWOULDBLOCK (arm 1 above reads the same fact from the other
	// side): a deterministic stand-in for the fork/exec window, with no forks
	// and no load on a box that is not to be load-tested (AGENTS.md).
	//
	// THE HOLD IS 300ms AND THAT NUMBER IS LOAD-BEARING. It is longer than
	// pruneUntilReaches passes can span even WITH the gap (20 × 5ms = 100ms),
	// so the only thing that can ride it out is the reach accounting: a
	// spared pass not counting. Shorter, and the arm would pass with that
	// accounting deleted, which is the one mutant it exists to catch.
	//
	// AND THE PASS COUNT IS THE GAP'S OWN PIN. MEASURED 2026-11-07: ~60
	// passes ride out 300ms at a 5ms gap, and with the sleep removed the same
	// ride-out spent 1212-1418, flat out on a core. The ceiling is ten times
	// the spaced figure, so the subject is "it is not spinning" and never a
	// timing assertion: the spaced run is sleep-dominated and cannot approach
	// 600 whatever the box is doing, while the unspaced one is pass-cost
	// dominated and so its own margin narrows as the box gets busier.
	t.Run("a lock held past any pass count is ridden out", func(t *testing.T) {
		t.Parallel()
		b, stub := muxMetaRexBoard(t, "husk", "01a1902c-husk", false)
		stub.onClose = func(Seat) error { return errors.New("server refused the destroy") }
		pruneUntil(t, b, "the prune never reached the destroy at all", func() bool { return len(stub.closes) > 0 })
		lock := holdLaunchLock(t, b)
		const hold = 300 * time.Millisecond
		released := make(chan struct{})
		go func() {
			time.Sleep(hold)
			lock.Release()
			close(released)
		}()
		gaveUp, passes, err := prunePasses(b, warnBuf(t, b), pruneUntilBudget,
			func() bool { return len(stub.closes) > 1 })
		<-released
		if err != nil {
			t.Fatal(err)
		}
		if gaveUp != "" {
			t.Fatalf("the destroy was never retried across a launcher lock held for %v: %s\n"+
				"The retry budget is spending attempts and not time, so it samples one instant of the window it exists to ride out (ranger-base-6mpke)\n%s",
				hold, gaveUp, warnBuf(t, b).String())
		}
		if max := int(hold/pruneUntilGap) * 10; passes > max {
			t.Errorf("riding out %v cost %d listing passes (ceiling %d) — the passes are not spaced, so this loop spins a core flat out against contention for as long as the contention lasts, on the box whose load produced it",
				hold, passes, max)
		}
	})

	// ── a broken prune still fails, and fails fast ──
	//
	// The falsifiability half. With the lock free every pass reaches
	// reclaim's own decision, so the reach guard — the old bound, applied to
	// the only passes it was ever a statement about — is what answers, in
	// milliseconds and not at the deadline. A budget that rode out a genuine
	// break for fifteen seconds a call would be ten of these pins' worth of
	// suite wall on every red run.
	t.Run("a cond that can never hold gives up on the reach guard", func(t *testing.T) {
		t.Parallel()
		b, stub := muxMetaRexBoard(t, "husk", "01a1902c-husk", false)
		stub.onClose = func(Seat) error { return errors.New("server refused the destroy") }
		start := time.Now()
		gaveUp, _, err := prunePasses(b, warnBuf(t, b), pruneUntilBudget, metaGone(b, "husk"))
		if err != nil {
			t.Fatal(err)
		}
		if gaveUp == "" {
			t.Fatal("the record went although every destroy failed — this arm measures nothing")
		}
		if !strings.Contains(gaveUp, "reached reclaim's own decision") || strings.Contains(gaveUp, "NOT ONE") {
			t.Errorf("a break was reported as a contended box: %q", gaveUp)
		}
		// The one wall-clock assertion here, and deliberately a 35x margin:
		// MEASURED 2026-11-07 at 0.13-0.15s against a ceiling of 5s. What it
		// holds is that the REACH guard answered and not the deadline, which
		// the message above cannot say on its own now that the deadline
		// reports its own count. Reaching the ceiling would take a sustained
		// ~98% false-busy rate for five seconds against the 5.31% measured
		// (forkflockrace_live_test.go) — a margin, not a bet on the box.
		if took := time.Since(start); took > pruneUntilBudget/3 {
			t.Errorf("a prune that reached its decision %d times spent %v giving up, with a budget of %v — the reach guard is not holding, so every red arm here now costs the whole budget",
				pruneUntilReaches, took, pruneUntilBudget)
		}
		// And the other give-up over the same board: a budget too short for
		// pruneUntilReaches passes to fit in (20 × 5ms = 100ms against 30),
		// so the DEADLINE is what answers, over passes that DID reach. It
		// must report them and not claim nobody got there — the whole cost of
		// this bead's own flake was a message that aimed its reader at the
		// wrong thing.
		//
		// AND IT IS ASKED AGAIN, for this file's own reason. MEASURED
		// 2026-11-07: written as one probe it reddened a full `make test`,
		// having spent its whole short budget inside one window — one pass,
		// spared, so the give-up was the OTHER branch and correctly so. A
		// short budget is by construction a budget that one window can
		// swallow, which is the entire finding of ranger-base-6mpke applied
		// to the pin that pins it. So the probe is repeated until the box
		// gives it a pass that got through, bounded the way everything else
		// here is bounded.
		clockDeadline := time.Now().Add(pruneUntilBudget)
		for {
			gaveUp, _, err = prunePasses(b, warnBuf(t, b), 30*time.Millisecond, metaGone(b, "husk"))
			if err != nil {
				t.Fatal(err)
			}
			if gaveUp == "" {
				t.Fatal("the record went although every destroy failed — this arm measures nothing")
			}
			if !strings.Contains(gaveUp, "NOT ONE") {
				break
			}
			if time.Now().After(clockDeadline) {
				t.Fatalf("over %v, not one pass of a short-budget prune ever got past the launcher lock, so this arm never reached the give-up it is about — that is a box on which that lock is never free: %s",
					pruneUntilBudget, gaveUp)
			}
		}
		if !strings.Contains(gaveUp, "before the clock ran out") {
			t.Errorf("the deadline answered over passes that reached reclaim's own decision and its give-up does not say it was the clock: %q", gaveUp)
		}
	})

	// ── a lock that is never free is reported as a box, not as a prune ──
	//
	// And this arm is also the pin on lockSparedSentence, which is the one
	// coupling here that no build can check: reclaim writes that sentence and
	// prunePasses reads it, and a rename on reclaim's side would make every
	// spared pass count against the reach guard instead. That failure is
	// silent in the only direction that matters — the budget would quietly
	// go back to being a count — so what is asserted is the DEADLINE message
	// over the reach message, which can only be reached through the match.
	t.Run("a lock held throughout is named as the box", func(t *testing.T) {
		t.Parallel()
		b, stub := muxMetaRexBoard(t, "husk", "01a1902c-husk", false)
		stub.onClose = func(Seat) error { return errors.New("server refused the destroy") }
		defer holdLaunchLock(t, b).Release()
		// A budget of its own, for prunePasses' own reason: the real one is
		// fifteen seconds of suite wall to witness one message.
		gaveUp, _, err := prunePasses(b, warnBuf(t, b), 50*time.Millisecond, func() bool { return len(stub.closes) > 0 })
		if err != nil {
			t.Fatal(err)
		}
		if gaveUp == "" {
			t.Fatal("the prune destroyed a seat with the launcher lock held by this test — reclaim's whole shape is that the act is taken under ADR 0011 §1's lock")
		}
		if !strings.Contains(gaveUp, "NOT ONE") {
			t.Errorf("every pass was spared on the launcher lock and the give-up blames the prune: %q\n"+
				"That is lockSparedSentence no longer matching what reclaim writes — which also means every spared pass now counts against the reach guard, and the budget is a count of twenty again (ranger-base-6mpke)", gaveUp)
		}
		if warn := warnBuf(t, b).String(); !strings.Contains(warn, lockSparedSentence) {
			t.Errorf("the sparing does not name the lock at all:\n%s", warn)
		}
	})
}
