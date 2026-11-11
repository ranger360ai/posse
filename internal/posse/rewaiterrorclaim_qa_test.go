//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-2mqp3 — a WAIT that failed is not a PROMPT that
// failed, so a re-wait leg's error never hands the bead back.
//
// THE INCIDENT, 2026-11-09, from the record on ranger-base-bu8rz. At 13:57 a
// pass commented on that bead: "posse dispatch unclaimed this bead: the
// prompt to <persona>-posse-ranger-base-bu8rz failed — herdr agent wait
// w36W:p1 --until idle --until done --until blocked --timeout 900000: signal:
// interrupt." (The session name is redacted: ADR 0012 App.A 5 — the shipped
// tree names roles, not this instance's crew.) The bead went open and
// unassigned and was re-dispatched to a second persona. At 14:20 the FIRST
// seat commented on the same bead with a
// full implementation and a commit: it had been working the whole 23 minutes
// and had watched itself be reassigned. The second seat, finding that tree
// holding uncommitted work with no commit on its branch, reasonably read it
// as an interrupted session about to be retired, adopted it and committed it.
// Two branches ended up holding byte-identical work for one bead — e7bcf63e
// and c2ed8024, each a single commit on the same base — of which only one
// can land, and the provenance in the other is wrong.
//
// WHAT WENT WRONG IS A TAXONOMY ERROR, the same one promptstall.go's header
// names for a different code. `signal: interrupt` on `herdr agent wait` says
// the WAIT was interrupted. It says nothing about the agent it was waiting
// on, and it cannot say anything about delivery, because `agent wait`
// submits nothing: the prompt had gone in fifteen minutes earlier, and the
// pass had already WATCHED it land — the first leg timed out over an agent
// its own status probe read as `working`, which is the only way a re-wait
// starts at all.
//
// WHAT LET IT THROUGH is the shape of gather's hand-back guard: it was
// written as "not a `timeout`" — a deny-list of one — against
// unclaimAfterPromptFailure's contract, which is an allow-list: "the claim
// was made but the prompt never reached the agent". So every error without a
// herdr envelope behind it (a signalled child, a replaced server, a hang)
// fell through it and was spent as a failed prompt. A bd claim is a lease
// WITHOUT expiry by ADR 0011's own decision, precisely because expiry
// without a fencing token the resource checks is double-dispatch; unclaiming
// a live seat on a wait error is that expiry, reintroduced through an exec
// failure.
//
// THREE PINS, ONE PER ARM OF THAT GUARD, and the second one is the point:
// a fix that simply stopped unclaiming would also undo rangerhq-81d, whose
// contract is that a prompt which never landed must not strand the bead as
// in_progress with nobody working it. The discriminator is WHICH LEG failed,
// not which error it was, so the same error is fed to both arms.
//
// AND THE SECOND PIN WAS REPLACED, NOT DELETED (ADR 0084, ranger-base-ng8wz).
// It was `TestQAFirstLegErrorWithNoCodeStillUnclaims`, and it pinned the
// first leg handing the bead back on that error IMMEDIATELY. ADR 0084
// refines the sentence above rather than overturning it: the discriminator
// is still which leg failed — a re-wait leg's error is not evidence about
// delivery at all, and this file's first arm is unchanged — but a FIRST
// leg's untyped error is not evidence either. It is INDETERMINATE: herdr
// submits the text server-side and the caller's child can die before,
// during or after that submission with no envelope to say which. So the
// first leg is now READ before it is spent — judgeStall's two readings,
// exactly as a stall — and only "no turn AND no work" reaches the unclaim.
// rangerhq-81d is kept by that arm, not by this one, and the replacement
// here (`…IsJudgedNotSpent`) pins what this fixture can see: the readings
// are taken, and an unreadable herdr keeps the claim. The full set of D2's
// arms is indeterminateprompt_qa_test.go.
//
// THE FIXTURE drives gather() directly, for waitlegpace_qa_test.go's reason:
// the subject is the leg loop, and a launch would put its own occupant-shaped
// settle gate in front of every assertion. HERDR only — `wait-crash` is a
// herdr-fake lever, and the defect is in dispatch's reading of an error and
// not in either occupant's way of producing one.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rewaitClaimSession is the one seat these fixtures hire.
const rewaitClaimSession = "ranger-qa-a-1"

// rewaitClaimCeiling bounds every pass here. The claim-kept arms are now
// correct and therefore ENDLESS in this fixture — every leg fails at once and
// the seat reads working forever — so they end where a working seat always
// ends, on gather's WaitCeiling. No pin here depends on a settle or a judged
// bead.
const rewaitClaimCeiling = 2 * time.Second

// crashedWaitSeat is a herdr seat that reads WORKING throughout and whose
// `agent wait` dies with no envelope at all: nonzero exit, nothing on
// stdout, nothing on stderr. Herdr.Run's last arm turns that into
// `Die("herdr agent wait …: exit status 3")` — untyped, naming the call, and
// with HerdrCodeOf of it "". That is the incident's class, and the one shape
// the `wait-error` lever cannot make, since it always writes an envelope.
//
// It returns the dispatcher and the pendingBead, built for the TYPED path
// (`delivered: false`, runtime claude) because that is where the defect
// lived: on `prompt: argv` the launch line already settled delivery (ADR
// 0013 §2) and no wait error has ever reached the hand-back there.
func crashedWaitSeat(t *testing.T) (*Dispatcher, string, *pendingBead) {
	t.Helper()
	b, fake := newTestBackend(t)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: b.App.WorkspaceLabel(rewaitClaimSession), AgentStatus: "working"}})
	workingClaude(t, fake)
	if err := os.WriteFile(filepath.Join(fake, "wait-crash"), []byte("3"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.writeMeta(&HerdrMeta{Name: rewaitClaimSession, Workspace: "w1", Pane: "w1:p1",
		Agent: "ranger", Socket: SocketID(), Gen: ServerGen(), Launched: time.Now()}); err != nil {
		t.Fatal(err)
	}
	d := newTestDispatcher(t, b)
	d.PromptWaitMS = 1
	d.WaitCeiling = rewaitClaimCeiling
	d.LegFloor = 200 * time.Millisecond
	// The fixture's own control: the arms that keep the claim only reach the
	// re-wait because the seat reads working, and a seat reading anything
	// else would end the loop somewhere this file is not about.
	if st, err := d.agentStatus(rewaitClaimSession); err != nil || st != "working" {
		t.Fatalf("the fixture seat reads (%q, %v), want working — nothing below would be about a live seat", st, err)
	}
	now := time.Now()
	p := &pendingBead{is: RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t"}, Dir: t.TempDir()},
		persona: "ranger", session: rewaitClaimSession, target: Seat{Session: "w1", Pane: "w1:p1"}, runtime: "claude",
		result: make(chan promptResult, 1), prompted: now, legAt: now, launched: now}
	return d, fake, p
}

// crashErr is the same leg error the fixture's own child produces, for the
// arms that feed it to gather by hand rather than through a real leg. Spelled
// as Herdr.Run spells it — a Die naming the call, with no envelope and
// therefore no code.
func crashErr() error {
	return Die("herdr %s: %s",
		"agent wait w1:p1 --until idle --until done --until blocked --timeout 900000",
		"signal: interrupt")
}

// THE PIN THE BEAD WAS FILED FOR. A re-wait leg that dies with no envelope
// keeps the claim, and the loop goes on asking the agent's own state — which
// is the only thing that ever said whether work was happening (rangerhq-1z0).
//
// Driven through a REAL leg, not a hand-fed result, so that it also pins
// rewait() setting the flag: a fix that guarded on the field but never wrote
// it would pass the hand-fed arms below and fail here.
func TestQARewaitLegErrorWithNoCodeKeepsTheClaim(t *testing.T) {
	t.Parallel()
	d, fake, p := crashedWaitSeat(t)
	d.rewait(p)
	inFlight, err := d.gather(p)
	out := dispatcherOut(d)
	if err != nil {
		t.Fatalf("a re-wait leg that died says nothing about the prompt — gather must not fail the bead: %v\n%s", err, out)
	}
	if !inFlight {
		t.Fatalf("the seat never stopped working, so this bead is still in flight:\n%s", out)
	}
	if strings.Contains(out, "unclaimed") {
		t.Errorf("THE INCIDENT: a wait error spent as a failed prompt, over a seat that is mid-turn:\n%s", out)
	}
	if !strings.Contains(out, "claim kept") {
		t.Errorf("the pass must say the claim was kept, so the next reader knows the bead is still somebody's:\n%s", out)
	}
	// The leg error is still SAID — keeping the claim is not swallowing the
	// failure, and this line is what an operator greps (ranger-base-aj79z).
	if !strings.Contains(out, "wait leg ended") {
		t.Errorf("the leg's own error must still be named:\n%s", out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "--status open") {
		t.Errorf("nothing may send the bead back to open while its seat is working (rangerhq-khc):\n%s", calls)
	}
	// THE DISCRIMINATOR'S OTHER HALF (ADR 0084). A re-wait leg's error is
	// not evidence about delivery, so there is nothing to investigate: no
	// reading is taken at all. The first-leg arm below asserts the same
	// call is made there, and the pair is what "which LEG failed" means now
	// that both verdicts are "claim kept".
	if log := calls(t, fake); strings.Contains(log, "--until working --until blocked") {
		t.Errorf("a re-wait leg's error says nothing about delivery — nothing is re-read on it:\n%s", log)
	}
}

// THE COUNTERFACTUAL, and the reason the fix is a discriminator rather than a
// retreat. The SAME untyped error on the FIRST leg — where nothing yet says
// the prompt landed — is not waved through: it is READ, with the two
// readings judgeStall takes for a stall (ADR 0084 D2), and it can still
// reach the unclaim when both of them answer that nothing happened.
//
// THIS FIXTURE'S OWN ANSWER IS THE UNREADABLE ONE, and it is one of D2's
// arms: `wait-crash` is armed, so the re-read dies exactly as the leg did,
// and a herdr that cannot be asked is IGNORANCE rather than evidence —
// claim kept, bead not judged this pass (rangerhq-khc). The arms where the
// readings succeed are in indeterminateprompt_qa_test.go, which has a
// prepared session tree to read: idle and clean hands the bead back (81d),
// a turn under way rewaits.
//
// WHAT SEPARATES THIS ARM FROM THE ONE ABOVE is therefore no longer the
// verdict — both keep the claim — but the READING. A first leg's error is
// about delivery and is investigated; a re-wait leg's error cannot be, and
// nothing is asked. The two arms assert each other's absence.
//
// Fed by hand, because the fixture's own first leg is an `agent prompt
// --wait` and arming THAT to crash is a different fixture (it is
// `prompt-crash`, and it is what the file above drives). What is under test
// is gather's route for a given leg result, and the only difference between
// this arm and the one above is which leg produced it.
func TestQAFirstLegErrorWithNoCodeIsJudgedNotSpent(t *testing.T) {
	t.Parallel()
	d, fake, p := crashedWaitSeat(t)
	p.result <- promptResult{err: crashErr(), at: time.Now()}
	inFlight, err := d.gather(p)
	out := dispatcherOut(d)
	if err != nil {
		t.Fatalf("an unreadable re-read is ignorance, not a verdict — gather must not fail the bead: %v\n%s", err, out)
	}
	if !inFlight {
		t.Fatalf("the claim is kept and the bead is not judged this pass, so it is in flight:\n%s", out)
	}
	// THE ROUTE, which is the whole of the change: the error was READ.
	if log := calls(t, fake); !strings.Contains(log, "--until working --until blocked") {
		t.Errorf("ADR 0084 D2: a first leg's error is indeterminate and gets the readings a stall gets:\n%s", log)
	}
	if !strings.Contains(out, "ended without saying whether the text went in") {
		t.Errorf("D4: the pass must say the call said nothing about delivery:\n%s", out)
	}
	// And the verdict on THIS fixture's reading.
	if strings.Contains(out, "unclaimed") {
		t.Errorf("nothing may be spent on a reading that could not be taken (rangerhq-khc):\n%s", out)
	}
	if !strings.Contains(out, "claim kept, not judged this pass") {
		t.Errorf("the pass must say the claim was kept, so the next reader knows the bead is still somebody's:\n%s", out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "--status open") {
		t.Errorf("a bead whose seat cannot be read is not a bead nobody is working:\n%s", calls)
	}
}

// THE DRAIN, which is the same taxonomy error one rung up and the reason the
// stop check now sits ABOVE the hand-back (ranger-base-e9d9's verdict,
// stated in stopClaim: "a drain is not evidence about the agent, and the
// agent is still working").
//
// A SIGINT reaches the whole process group, so the signal that stops the
// watch loop also kills the `herdr agent wait` child it is holding — and
// that child's death arrives in gather as exactly the error above. With the
// stop check below the hand-back, a Ctrl-C over a pass with prompts in
// flight unclaimed every one of their beads on the way out, under seats that
// were all still working. First leg or re-wait makes no difference here,
// which is why this arm uses the first: a drain never judges a bead.
func TestQADrainDoesNotUnclaimOnAKilledWaitChild(t *testing.T) {
	t.Parallel()
	d, fake, p := crashedWaitSeat(t)
	ctx, cancel := context.WithCancel(context.Background())
	d.stopCtx = ctx
	cancel()
	p.result <- promptResult{err: crashErr(), at: time.Now()}
	inFlight, err := d.gather(p)
	out := dispatcherOut(d)
	if err != nil {
		t.Fatalf("a drain judges nothing: %v\n%s", err, out)
	}
	if !inFlight {
		t.Fatalf("the bead is left claimed and in flight on the way out:\n%s", out)
	}
	if strings.Contains(out, "unclaimed") {
		t.Errorf("a Ctrl-C that killed the wait child must not hand the bead back:\n%s", out)
	}
	if !strings.Contains(out, "watch loop stopping") {
		t.Errorf("the pass must name the drain as the reason it stopped judging:\n%s", out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "--status open") {
		t.Errorf("nothing is unclaimed on the way out:\n%s", calls)
	}
}
