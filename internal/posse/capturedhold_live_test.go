//go:build !posse_arm2 && !posse_arm3

package posse

// Live pin for ranger-base-r5131, run against the real rex server and a real
// claude seat rather than the fake — because the fake is a file this repo
// writes, and the thing being claimed is that CLAUDE puts a sentence on a
// screen that REX will hand back.
//
//	RHQ_LIVE_REX=1 RHQ_LIVE_CLAUDE_DIR=$HOME/.claude \
//	  go test ./internal/posse/ -run TestLiveCapturedHoldReadsThisSeatsOwnFooter -v
//
// It reads the seat it is RUNNING IN: `REX_SESSION` and `REX_BLOCK` are in
// every Rex seat's environment (MEASURED 2026-11-10 in this one), so the pin
// needs no fixture, no launch and no cleanup — the live claude whose footer
// it reads is the one running the test.
//
// WHAT IT PROVES THAT THE HERMETIC ARMS CANNOT. capturedhold_qa_test.go pins
// the route and the anchor over a capture this repo wrote. Three things
// outside this repo have to hold for that to mean anything, and all three
// are somebody else's to change: claude has to put its background-task
// summary in the footer, rex's `capture --format text` has to return a
// screen with the rules in it, and the footer has to survive the end of the
// turn. The first and third were MEASURED 2026-11-10 by sampling this seat
// across a turn boundary (panework.go's capturedHold note); this pin is how
// the first two get re-checked on a later claude or a later rex, by hand,
// in one command.
//
// It asserts the SHAPE and not the work, because a seat with nothing running
// has an honest empty footer and that is not a failure. Set
// RHQ_LIVE_REX_WORK=1 when something IS running in the seat — a background
// shell, a monitor — and the pin then also requires the hold to see it,
// which is the arm that would have caught this bead.
//
// IT GREW A FOURTH CLAIM WITH ranger-base-qzujz, and it is the one most
// exposed to somebody else's release: that claude writes a session row for
// the process rex names as this block's owner, that the row's procStart
// still matches what the kernel says about that pid, and that the id in it is
// the one claude's own submit log is keyed on (ADR 0076, Amendment 6). That
// chain is what tells a held prompt from an echoed one, so when it goes quiet
// every composer on this instance reads as a hold. It needs the operator's
// real claude dir NAMED — RHQ_LIVE_CLAUDE_DIR — because the suite's own
// TestMain points HOME at a temp dir before anything reads it, and
// liveCapturedComposerChain's own note says why that is right rather than
// inconvenient.
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveCapturedHoldReadsThisSeatsOwnFooter(t *testing.T) {
	// Read-only throughout: three rex calls that capture a screen and read a
	// status, no seat created, nothing typed, no env mutated.
	t.Parallel()

	if os.Getenv("RHQ_LIVE_REX") == "" {
		t.Skip("RHQ_LIVE_REX unset: this pin shells out to the operator's rex and reads the seat it runs in")
	}
	block, bin := os.Getenv("REX_BLOCK"), os.Getenv(RexBinEnv)
	if block == "" {
		t.Skip("REX_BLOCK unset: not running inside a Rex seat, so there is no live footer to read")
	}
	// BOTH HALVES of the seat, which is what the verbs under test are
	// addressed with since ranger-base-k0hd6 — a Rex block lives under
	// `session.*`, so an occupant handed a pane-only Seat resolves the
	// session half out of the server's own listing first (RexMux.hold,
	// ranger-base-nfc2b). The environment carries both, so this pin pays
	// neither that listing nor the chance of resolving to the wrong seat.
	// REX_SESSION empty is not a skip: the occupant still answers for half
	// a Seat it is handed, and that licence is not withdrawn.
	seat := Seat{Session: os.Getenv("REX_SESSION"), Pane: block}
	if bin == "" {
		bin = RexDefaultBin()
	}

	b := &HerdrBackend{Slot: RexMux{R: Rex{Bin: bin}}}

	// The §4 door must still refuse, or the fallback under test is not the
	// code path this seat takes.
	if _, err := b.explain(seat); err == nil {
		t.Fatal("`agent explain` answered on a Rex slot — this pin is about the route taken when it cannot")
	} else if verb, missing := muxVerbMissing(err); !missing {
		t.Fatalf("explain failed with something other than §4's refusal (%v) — the capture route is not reached and PaneHolding answers the empty hold", err)
	} else if verb != "agent explain" {
		t.Fatalf("§4 refusal named %q, want \"agent explain\"", verb)
	}

	screen, err := b.Mux().Read(seat, captureFooterLines, false)
	if err != nil {
		t.Fatalf("rex could not capture this seat's own block %s: %v", block, err)
	}
	footer := captureFooter(screen)
	t.Logf("captured %d bytes of screen tail; footer = %q", len(screen), footer)
	if footer == "" {
		t.Fatalf("no footer after the last horizontal rule in a live claude screen — either claude stopped drawing the rules or rex stopped returning them, and either way the WORK half of htafy's hold is off on this instance again. Screen tail:\n%s", screen)
	}

	hold := b.PaneHolding(seat)
	// Whatever the footer says, these two must agree: the hold is read off
	// the footer and nothing else on this route.
	if (hold.Work != "") != (backgroundWorkIn(footer) != "") {
		t.Errorf("PaneHolding().Work = %q but the footer's own vocabulary reading is %q — the hold is no longer the footer's", hold.Work, backgroundWorkIn(footer))
	}
	// ranger-base-qzujz: the composer half reads here too now, and the three
	// things it rests on are all outside this repo — claude still drawing its
	// box between two rules, claude still writing `sessions/<pid>.json` with
	// a procStart the kernel agrees with, and rex still handing the owner pid
	// back. The hermetic arms cannot check any of them.
	if got := captureComposer(screen); got != hold.Typed && got != hold.Sent && got != hold.Ghost {
		t.Errorf("captureComposer read %q off this screen and the hold carries Typed=%q Sent=%q Ghost=%q — the box is no longer between the last two rules, or the reading stopped being the one the hold spends", got, hold.Typed, hold.Sent, hold.Ghost)
	}
	liveCapturedComposerChain(t, b, seat)

	if os.Getenv("RHQ_LIVE_REX_WORK") == "" {
		t.Logf("RHQ_LIVE_REX_WORK unset: not requiring live work. Work = %q", hold.Work)
		return
	}
	// The arm that reproduces the bead. Something is running in this seat,
	// the turn it was started from may well have ended, and the hold has to
	// see it — or dispatch counts a settle-open and re-prompts a seat that is
	// doing exactly what it was told.
	if hold.Work == "" {
		t.Fatalf("RHQ_LIVE_REX_WORK is set and the hold sees no live work. footer = %q", footer)
	}
	if !hold.Waiting() {
		t.Fatalf("Waiting() = false with Work = %q", hold.Work)
	}
	if !strings.Contains(hold.Why(), "still running") {
		t.Errorf("Why() = %q, want it to say what is still running", hold.Why())
	}
	t.Logf("live hold: Work = %q, Why = %q", hold.Work, hold.Why())
}

// liveCapturedComposerChain is ranger-base-qzujz's live arm: the join that
// makes a composer reading honest on this occupant, against the real stores.
//
// IT READS THE OPERATOR'S OWN CLAUDE CONFIG DIR, which no hermetic arm may
// do and which is the whole point of a pin behind RHQ_LIVE_REX: the claim is
// that CLAUDE writes a session row for the process REX names as the owner of
// this block, and that the row's procStart still matches what the kernel says
// about that pid. Both are somebody else's to change — a claude release, a
// platform — and when either moves, the echo reading goes quiet and every
// composer on this instance reads as a hold again.
//
// Read-only: one `program_status`, one `process`, two file reads.
func liveCapturedComposerChain(t *testing.T, b *HerdrBackend, seat Seat) {
	t.Helper()

	// The live store, named here and nowhere else in this file: every other
	// reading in it is rex's. It cannot be RESOLVED here, which is the one
	// thing about this arm that is not obvious: the suite's TestMain points
	// HOME and the three CLI home overrides at a temp dir before anything
	// reads them, so `claudeHistoryPath()` inside a test answers
	// /tmp/posse-testhome-…/.claude whatever the operator's real dir is. That
	// is the discipline keeping every hermetic arm off the operator's own
	// prompt history (herdrback.go, and HerdrBackend.ClaudeHistory being
	// empty in every test backend) — so the one pin that is ABOUT that store
	// has the operator NAME it, and skips otherwise.
	dir := os.Getenv("RHQ_LIVE_CLAUDE_DIR")
	if dir == "" {
		t.Skip("RHQ_LIVE_CLAUDE_DIR unset: this arm reads the operator's own claude store, and the suite's temp HOME means it has to be named to be reached (RHQ_LIVE_CLAUDE_DIR=$HOME/.claude)")
	}
	history := filepath.Join(dir, claudeHistoryFile)
	b.ClaudeHistory = history

	sess, err := b.paneAgentSession(seat)
	if err != nil {
		t.Fatalf("no runtime session id for this seat's own block: %v\n"+
			"  That is ADR 0076 Amendment 6's claim failing on the live stores. Without it the echo check cannot run, "+
			"every box showing the line this pane last submitted reads as a prompt sitting UNSENT, and the --resume skip "+
			"and the pulse's G2 -unsent row both act on an invented hold.", err)
	}
	if len(sess) < 32 || !strings.Contains(sess, "-") {
		t.Errorf("runtime session id = %q, which is not the uuid shape claude's submit log is keyed on", sess)
	}
	t.Logf("live join: block %s -> claude session %s", seat.Pane, sess)

	// The last link: that id is the one the submit log is keyed on. This
	// seat has submitted at least the prompt that started the turn running
	// the test, so a store that answers and holds no row for this session is
	// the join landing on the wrong conversation.
	sent, ok := lastSubmitted(history, sess)
	if !ok {
		t.Fatalf("claude's submit log (%s) holds no row for session %s, though this seat has submitted at least one prompt — the pid→session join is naming another conversation", history, sess)
	}
	t.Logf("live chain complete: last submit in this session was %q", ellipsis(sent, 60))
}

// TestLiveRexPromptSeesThisSeatsOwnComposer is ranger-base-hlnyx's live pin,
// and it guards the hazard ADR 0076 Amendment 7 names out loud: the composer
// gate refuses a prompt into a claude block whose screen has no box on it, so
// if a later claude stops drawing the two rules around its box, every prompt
// on the instance is refused. The hermetic arms cannot see that — their
// screens are quoted bytes from 2026-11-10 — and this one can, because the
// seat running the suite is a live claude block with its composer drawn.
//
// READ-ONLY, and nothing is typed: this is `captureComposerSight` over one
// capture of this seat's own screen, not a `Prompt`. A pin that actually
// prompted would be typing into the seat running it.
func TestLiveRexPromptSeesThisSeatsOwnComposer(t *testing.T) {
	t.Parallel()

	if os.Getenv("RHQ_LIVE_REX") == "" {
		t.Skip("RHQ_LIVE_REX unset: this pin shells out to the operator's rex and reads the seat it runs in")
	}
	block, bin := os.Getenv("REX_BLOCK"), os.Getenv(RexBinEnv)
	if block == "" {
		t.Skip("REX_BLOCK unset: not running inside a Rex seat, so there is no live screen to read")
	}
	if bin == "" {
		bin = RexDefaultBin()
	}
	seat := Seat{Session: os.Getenv("REX_SESSION"), Pane: block}
	m := RexMux{R: Rex{Bin: bin}}

	screen, err := m.Read(seat, captureFooterLines, false)
	if err != nil {
		t.Fatalf("rex could not capture this seat's own block %s: %v", block, err)
	}
	sight := captureComposerSight(screen)
	t.Logf("live sight = %d (open=%d), last line = %q", sight, composerOpen, captureLastLine(screen))
	if sight != composerOpen {
		t.Fatalf("this seat's own live screen does not read as a composer posse may type into (sight %d).\n"+
			"  That is ADR 0076 Amendment 7's named hazard arriving: refuseWithoutComposer now refuses EVERY prompt "+
			"on this instance — dispatch, the pulse, relaunch, the probe and `posse prompt` alike. Either claude "+
			"stopped drawing its box between two U+2500 rules or rex stopped returning them. Screen tail:\n%s", sight, screen)
	}
	// The other half of the same guard: this seat IS claude, so the owner
	// scope (D4) must not be conceding the whole reading on a live seat.
	owner, err := m.RuntimeOwner(seat)
	if err != nil {
		t.Fatalf("no runtime owner for this seat's own block: %v — the gate concedes on this, so it is off on every seat", err)
	}
	if owner.Name != claudeRuntimeProcName {
		t.Errorf("this seat's block is owned by %q and not %q, so the composer gate concedes here", owner.Name, claudeRuntimeProcName)
	}
	// ranger-base-6pew3's live arm, and it is the other direction of the
	// same hazard. refuseAtDeadOwner refuses on two of §3's guards, so a
	// rex that stopped naming this block's owner — or named it with a pid
	// or a name the process table disagrees with — would read rexOwnerDead
	// on a seat a claude is sitting in and refuse every prompt on the
	// instance. The hermetic arms seed both halves of that comparison and
	// so cannot see it; this seat is a live claude whose record rex wrote.
	if _, verdict, err := m.runtimeOwner(seat); verdict != rexOwnerAnswers {
		t.Fatalf("this seat's own live block reads owner verdict %d, want %d (%v).\n"+
			"  That is the liveness gate turned against the fleet: refuseAtDeadOwner now refuses EVERY prompt on "+
			"this instance — dispatch, the pulse, relaunch, the probe and `posse prompt` alike. Either rex stopped "+
			"naming the owner of a block a runtime holds, or its name and pid no longer agree with the block's own "+
			"`process` answer", verdict, rexOwnerAnswers, err)
	}
}
