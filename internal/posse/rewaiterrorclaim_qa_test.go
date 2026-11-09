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
		persona: "ranger", session: rewaitClaimSession, target: "w1:p1", runtime: "claude",
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
}

// THE COUNTERFACTUAL, and the reason the fix is a discriminator rather than a
// retreat. The SAME untyped error on the FIRST leg — nothing yet says the
// prompt landed — still hands the bead back, which is rangerhq-81d's
// contract: a prompt that never reached the agent must not strand the bead as
// in_progress with nobody working it.
//
// Fed by hand, because the fixture's own first leg is a `agent prompt --wait`
// and arming THAT to crash would be a pin about a different call. What is
// under test is gather's verdict on a given leg result, and the only
// difference between this arm and the one above is which leg produced it.
func TestQAFirstLegErrorWithNoCodeStillUnclaims(t *testing.T) {
	t.Parallel()
	d, fake, p := crashedWaitSeat(t)
	p.result <- promptResult{err: crashErr(), at: time.Now()}
	inFlight, err := d.gather(p)
	out := dispatcherOut(d)
	if err == nil {
		t.Fatalf("81d: a first leg that failed with nothing saying the prompt landed must hand the bead back:\n%s", out)
	}
	if inFlight {
		t.Errorf("a handed-back bead is not in flight:\n%s", out)
	}
	if !strings.Contains(err.Error(), "unclaimed") {
		t.Errorf("the error must say the bead was unclaimed, got %v:\n%s", err, out)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "--status open") {
		t.Errorf("81d: the bead must go back to open — nobody is working it:\n%s", calls)
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
