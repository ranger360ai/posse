//go:build posse_arm2

package posse

// QA pins for ranger-base-ng8wz / ADR 0084 — a prompt call that says
// nothing about DELIVERY is indeterminate: it is READ, never spent as a
// hand-back.
//
// WHERE THIS SITS. ranger-base-uauvn took `agent_prompt_stalled` out of the
// hand-back (promptstall.go's header) and ranger-base-2mqp3 took the
// RE-WAIT legs out of it (rewaiterrorclaim_qa_test.go). What was left is the
// FIRST leg, `agent prompt --wait`, which still handed the bead back on any
// error that was not herdr's `timeout` code — a deny-list of one reading a
// contract that is an allow-list.
//
// WHAT AN UNTYPED FIRST-LEG ERROR ACTUALLY IS, from the ADR's Context
// (MEASURED 2026-11-09 against herdr 0.9.1's own docs): herdr submits "text
// followed by encoded Enter as one ordered submission" server-side, and the
// caller's `--timeout` "includes submission time". A client child that is
// signalled, hangs past its deadline or loses its server can die BEFORE,
// DURING or AFTER that submission, and the error carries no envelope — so
// posse cannot tell which. The leg is up to PromptWaitMS (15 min) long, so
// such an error can arrive after a whole turn has run and settled.
//
// The field has a name for this. Jepsen records a call whose outcome cannot
// be known as `:info`, "allow[ing] that it might or might not have taken
// place", and converts it to `:fail` only when the caller KNOWS the value
// did not change; gRPC says the same of DEADLINE_EXCEEDED. Handing the bead
// back on one is reading `:info` as `:fail`. Prompting an agent is not
// idempotent and carries no key, so the indeterminate case cannot be
// retried — it can only be resolved by reading the receiver, which is
// exactly what judgeStall already does for a stall.
//
// THE ARMS, one per decision:
//
//	1  D2  untyped, seat idle, session tree clean   → handed back (81d)
//	2  D2  untyped, a turn under way on the re-read → rewait, claim kept
//	3  D2  untyped, herdr unreadable on the re-read → claim kept
//	4  D1  server_not_running (a code, not on the list) → READ, not spent
//	5  D1  agent_blocked                            → hand back, no re-read
//	6  D1  isNotDelivered itself, as a table
//	7  D5  LaunchBead (cockpit `d`) on an untyped error → claim kept
//
// Arm 1 is the one that keeps rangerhq-81d: the widening must not turn a
// live, idle seat that never got the text into a permanent claim. Arms 2
// and 3 are what the hand-back is traded for, and arm 4 is why the guard is
// an ALLOW-LIST — under "not a timeout" that code handed the bead back with
// no reading taken at all, which is what ranger-base-2mqp3's superseding
// note called "the rangerhq-khc failure mode arriving through the one door
// the khc fix does not cover".
//
// THE LEVER is `prompt-crash`, added for these arms: an `agent prompt` that
// exits nonzero with nothing on stdout and nothing on stderr, so
// Herdr.Run's last arm turns it into an untyped `Die` naming the call with
// HerdrCodeOf of it "". It is wait-crash's half for the first leg, and it is
// the one shape `prompt-error` cannot make — that lever always writes an
// envelope, and an envelope is the very thing the incident class lacks.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// crashedPromptSeat is stalledSeat's sibling: one ready `go` bead, one idle
// claude, and a FIRST leg that dies with no envelope at all. `tree` asks for
// a repo with per-session worktrees, so the work reading has a tree and a
// branch of its own to read (the arms that reach it say which they need).
func crashedPromptSeat(t *testing.T, tree bool) (*Dispatcher, string, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	ready := `[{"id":"a-1","title":"t","labels":["go"]}]`
	repo := qaRepo(t, b.App, ready, `[{"id":"a-1","status":"closed"}]`)
	if tree {
		repo = wtqaRepo(t, b.App, ready, "")
	}
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-crash"), "3")
	return d, fake, repo
}

// Arm 1, and it is rangerhq-81d's own case: the first leg died with no
// envelope, herdr answers the re-read with no turn, and the session's tree
// and branch are both empty. Both readings ANSWERED and both found nothing,
// so the bead goes back after one startup_wait rather than being stranded
// in_progress with nobody working it.
//
// The widening this arm controls is the whole risk of ADR 0084: every other
// arm in this file keeps a claim, so without this one a build that simply
// stopped handing back would be green across the file and would strand
// every bead whose prompt really did not land.
func TestIndeterminatePromptOverACleanIdleSeatHandsBack(t *testing.T) {
	t.Parallel()
	d, fake, repo := crashedPromptSeat(t, true)

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := d.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	// CONTROL, the dependency the tree half of the reading rests on: a
	// PREPARED tree is porcelain-clean, so this arm's "no work" is the
	// seat's emptiness and not a reading that found posse's own scaffolding
	// (seedScaffoldExcludes, ranger-base-e01op; both prepare paths are
	// measured in TestPreparedSessionTreeIsPorcelainCleanOnEveryPreparePath).
	if out := mustGit(t, tr.Path, "status", "--porcelain", "--untracked-files=all"); strings.TrimSpace(out) != "" {
		t.Fatalf("CONTROL: a freshly prepared tree must be porcelain-clean, or this arm is not about an idle seat:\n%s", out)
	}

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a first leg that crashed must not abort the pass: %v", err)
	}
	out, bd, log := dispatcherOut(d), bdCalls(t, fake), calls(t, fake)

	// THE FIXTURE PREMISE, asserted rather than assumed: the readings were
	// actually TAKEN. Without this the arm is also green over the old build,
	// which handed back on the error alone — the hand-back is the same, and
	// only the route to it is the subject.
	if !strings.Contains(log, "--until working --until blocked") {
		t.Fatalf("D2: an indeterminate call gets the readings a stall gets — the re-read was never asked:\n%s", log)
	}
	// D4's sentence: the print says what is NOT known, and names the error,
	// because that is the only thing anybody knows about the call.
	if !strings.Contains(out, "ended without saying whether the text went in") {
		t.Errorf("D4: the pass must say the call said nothing about delivery:\n%s", out)
	}
	if !strings.Contains(out, "exit status") {
		t.Errorf("D4: the trigger names the error — a reader tells a signalled child from a lost server by it:\n%s", out)
	}
	if strings.Contains(out, "saw no turn start inside its own 5s window") {
		t.Errorf("this was not a stall — herdr never said it accepted the submission:\n%s", out)
	}
	// And the verdict, which is 81d's: the bead goes back to open.
	if !strings.Contains(out, "unclaimed") {
		t.Errorf("81d: both readings answered nothing, so the bead must go back:\n%s", out)
	}
	if !strings.Contains(bd, "--actor ranger update a-1 --status open --assignee  --json") {
		t.Errorf("a-1 must be handed back:\n%s", bd)
	}
	// The bead's own record of why it is open (ranger-base-uauvn ask 3).
	if !strings.Contains(bd, "comments add a-1") || !strings.Contains(bd, "posse worktrees") {
		t.Errorf("the unclaim must say so on the bead, and name where the work would be:\n%s", bd)
	}
	// D4's record half: the D5 reading keeps its decision id and its rule —
	// this IS the same decision — and carries the trigger as a prefix on the
	// Verdict, so a census can split the two without a new field.
	rec, ok := lastStallReading(t, d.App, tr.Path)
	if !ok {
		t.Fatalf("the verdict must be recorded (ADR 0066 D1)")
	}
	if !strings.HasPrefix(rec.Verdict, "indeterminate: hand back: ") {
		t.Errorf("D4: the record's Verdict must name this trigger, got %q", rec.Verdict)
	}
	if rec.Rule != RuleStallVerdict || rec.Decision != DecisionStallVerdict {
		t.Errorf("D4: decision and rule are UNCHANGED — the same decision read the same way, got %q/%q", rec.Decision, rec.Rule)
	}
	if !strings.Contains(rec.Verdict, "no commit on its branch and nothing uncommitted in its tree") {
		t.Errorf("the record must name both halves of the reading it handed back on, got %q", rec.Verdict)
	}
}

// Arm 2, the case the hand-back was trading away. The call said nothing
// about delivery — and a turn is under way, so the text plainly DID land.
// The claim stays, the leg is waited again, and the bead reaches its
// ordinary judgement.
//
// This is ranger-base-2mqp3's incident arriving through the FIRST leg
// instead of the re-wait: there the seat worked 23 minutes while its bead
// was reassigned under it, and two branches ended up holding byte-identical
// work.
func TestIndeterminatePromptWithATurnUnderWayKeepsItsClaim(t *testing.T) {
	t.Parallel()
	d, fake, _ := crashedPromptSeat(t, false)
	stallWait(t, fake, "working")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("an indeterminate call over a working seat must not fail the pass: %v", err)
	}
	out := dispatcherOut(d)
	if n != 1 || !strings.Contains(out, "closed by ranger") {
		t.Errorf("the seat worked and closed the bead, so the pass judges it closed: got n=%d\n%s", n, out)
	}
	if strings.Contains(out, "unclaimed") {
		t.Errorf("a turn under way is a prompt that LANDED — nothing may be handed back:\n%s", out)
	}
	if bd := bdCalls(t, fake); strings.Contains(bd, "--status open") {
		t.Errorf("this is the 2mqp3 duplicate dispatch arriving through the first leg:\n%s", bd)
	}
	if !strings.Contains(out, "a turn is under way") || !strings.Contains(out, "claim kept, waiting again") {
		t.Errorf("the operator is owed the reason the error was not a hand-back:\n%s", out)
	}
	// Both legs: the re-read herdr's stall question asks, and the settle leg
	// after it. Without the second the bead would be judged on a state
	// nobody waited for.
	log := calls(t, fake)
	if !strings.Contains(log, "--until working --until blocked") {
		t.Errorf("the first reading asks herdr for a turn:\n%s", log)
	}
	if !strings.Contains(log, "--until idle --until done --until blocked") {
		t.Errorf("the settle leg must be waited again after the verdict:\n%s", log)
	}
}

// Arm 3. herdr will not say what the agent is doing, so posse does not know
// whether the text landed AND cannot find out. That is IGNORANCE, and
// ignorance is never a hand-back (rangerhq-khc): the claim stays and the
// bead is not judged this pass.
//
// The ADR states the cost of this direction plainly rather than hiding it:
// when the readings themselves cannot be taken, a bead whose prompt really
// did not land stays claimed until the operator runs `--resume`. ADR 0011
// already chose that — a visible stranded claim is cheaper than an
// invisible second seat, which was MEASURED in 2mqp3 at a full duplicate of
// one bead and wrong provenance on one of two branches.
func TestIndeterminatePromptOverAnUnreadableAgentKeepsItsClaim(t *testing.T) {
	t.Parallel()
	d, fake, _ := crashedPromptSeat(t, false)
	stallWait(t, fake, "server_unreachable|herdr went away")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("an unreadable agent must not fail the pass: %v", err)
	}
	out, bd := dispatcherOut(d), bdCalls(t, fake)
	if strings.Contains(bd, "--status open") {
		t.Errorf("a herdr that cannot be asked is not evidence the prompt never landed:\n%s", bd)
	}
	if !strings.Contains(out, "claim kept, not judged this pass") {
		t.Errorf("the pass must say the claim was kept and where to look:\n%s", out)
	}
}

// Arm 4, and the reason D1 is an ALLOW-LIST rather than "not a timeout".
// `server_not_running` is a herdr CODE, and under the old guard a code that
// was not `timeout` handed the bead back with no reading taken at all. It is
// not a statement about delivery: a server replaced under an in-flight
// prompt (`herdr update --handoff`, MEASURED on herdr 0.8.0 under
// ranger-base-7t4 / ranger-base-kcr) answers every later call with it, and
// the prompt it answers for may well have landed.
//
// So it is indeterminate like every other code off the list, and it is READ.
// This arm's re-read is unreadable, so the verdict is the keep — what is
// pinned is that the reading HAPPENED, which is the whole difference from
// the old build.
func TestAHerdrCodeOffTheAllowListIsReadNotSpent(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-error"), "server_not_running|the herdr server is not running")
	stallWait(t, fake, "server_unreachable|herdr went away")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a code off the allow-list must not abort the pass: %v", err)
	}
	out, bd, log := dispatcherOut(d), bdCalls(t, fake), calls(t, fake)
	if !strings.Contains(log, "--until working --until blocked") {
		t.Fatalf("D1: a code that is not isNotDelivered is INDETERMINATE and gets the readings:\n%s", log)
	}
	if strings.Contains(bd, "--status open") {
		t.Errorf("`server_not_running` says nothing about delivery — the bead must not go back on it:\n%s", bd)
	}
	if !strings.Contains(out, "claim kept, not judged this pass") {
		t.Errorf("the pass must say the claim was kept:\n%s", out)
	}
}

// Arm 5, the keying, and the other half of the allow-list.
// TestRefusedPromptHandsBackWithNoSecondReading pins `agent_not_ready`; this
// is `agent_blocked`, which herdr documents as a submission "rejected ...
// BEFORE ANY INPUT IS SENT" (`herdr agent prompt --help`, 0.9.1). That is
// positive evidence about delivery, so it hands back at once: no grace
// spent, no reading taken.
func TestABlockedSubmissionHandsBackWithNoReading(t *testing.T) {
	t.Parallel()
	d, fake, repo := stalledSeat(t, "agent_blocked")
	clearFakeShow(t, repo)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a refused submission must not abort the pass: %v", err)
	}
	out, bd, log := dispatcherOut(d), bdCalls(t, fake), calls(t, fake)
	if !strings.Contains(out, "unclaimed") || !strings.Contains(bd, "--status open") {
		t.Errorf("herdr refused the submission before any input was sent — the bead goes back at once:\n%s\n%s", out, bd)
	}
	if strings.Contains(log, "--until working") {
		t.Errorf("nothing is re-read: herdr already said it sent nothing:\n%s", log)
	}
	if strings.Contains(out, "ended without saying whether the text went in") {
		t.Errorf("this call DID say whether the text went in — it said no:\n%s", out)
	}
	if !strings.Contains(bd, "comments add a-1") || !strings.Contains(bd, "agent_blocked") {
		t.Errorf("every unclaim says so on the bead, naming the code:\n%s", bd)
	}
}

// Arm 6, the allow-list itself, as a table. The pins above drive it through
// a pass; this one states the list, because the ADR's D1 is a list and the
// guard "must never be written as 'not timeout' again" — a spelling no
// behavioural arm can refuse, since the two agree on every code that has a
// fixture.
//
// `nil` is on it as `false` for the same reason: isNotDelivered is only
// asked about a non-nil error, and a guard that answered true on nil would
// hand back a bead whose prompt succeeded.
func TestIsNotDeliveredIsAnAllowListOfTwoCodes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err  error
		want bool
		why  string
	}{
		{HerdrAPIError{Code: "agent_not_ready"}, true, "herdr will not address the pane — nothing was sent"},
		{HerdrAPIError{Code: "agent_blocked"}, true, "rejected before any input is sent"},
		{HerdrAPIError{Code: "agent_prompt_stalled"}, false, "an ACCEPTED submission, unobserved — the text WAS sent"},
		{HerdrAPIError{Code: "timeout"}, false, "herdr took the text and stopped watching"},
		{HerdrAPIError{Code: "server_not_running"}, false, "a replaced server says this about calls whose prompt landed"},
		{HerdrAPIError{Code: ""}, false, "an envelope with no code is not herdr saying anything"},
		{Die("herdr agent prompt w1:p1 --wait: signal: interrupt"), false, "a signalled child: no envelope, so nobody knows"},
		{&HerdrHangError{}, false, "a hang past the deadline: the submission may have completed"},
		{nil, false, "a prompt that succeeded"},
	} {
		if got := isNotDelivered(c.err); got != c.want {
			t.Errorf("isNotDelivered(%v) = %v, want %v — %s", c.err, got, c.want, c.why)
		}
	}
}

// Arm 7, ADR 0084 D5: the cockpit's `d`. It prompts without `--wait` and has
// no gather loop, so it has nothing to re-read with and no later pass to
// judge the bead — the only two verdicts available are hand back now or keep
// the claim. On an indeterminate error it KEEPS the claim and names `posse
// peek`: the operator pressed the key and is watching, and a hand-back on a
// seat that did get the text would be a second dispatch they did not ask
// for.
//
// TestLaunchBeadPromptErrorUnclaims is the counterfactual and is unchanged:
// `agent_not_ready` on the same path still hands the bead back, because that
// is herdr saying it sent nothing.
func TestLaunchBeadKeepsTheClaimOnAnIndeterminatePrompt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-crash"), "3")

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	_, err := d.LaunchBead(is)
	if err == nil {
		t.Fatalf("the operator must be told the call said nothing about delivery")
	}
	got, bd := err.Error(), bdCalls(t, fake)
	if !strings.Contains(got, "ended without saying whether the text went in") {
		t.Errorf("D5: the error must say what is not known, got %v", err)
	}
	if !strings.Contains(got, "claim kept") || !strings.Contains(got, "posse peek") {
		t.Errorf("D5: the error must say the claim was kept and where to read the seat, got %v", err)
	}
	// `— unclaimed` is unclaimAfterPromptFailure's own suffix, and matching
	// the suffix rather than the word is deliberate: this error embeds the
	// argv, which embeds the whole work prompt, so a bare word match is a
	// pin on the prompt's wording.
	if strings.Contains(got, "— unclaimed") {
		t.Errorf("D5: it must not unclaim, got %v", err)
	}
	// THE FIXTURE PREMISE: the claim really was made, so "kept" is about a
	// claim that exists. Without it this arm would be green over a build
	// that failed before claiming.
	if !strings.Contains(bd, "a-1 --claim") {
		t.Errorf("setup: the bead must have been claimed before the prompt:\n%s", bd)
	}
	if strings.Contains(bd, "--status open") {
		t.Errorf("D5: nothing may send the bead back to open — the operator is watching:\n%s", bd)
	}
	// And the session is remembered as PROMPTED, which is the same reading
	// one step on: the text may have gone in, so a second `d` inside
	// PromptGrace is refused rather than typing a second instruction into a
	// seat that is mid-turn (the ADR's rejected "retry the prompt" — the
	// prompt is not idempotent and has no key).
	session := SessionForBead("ranger", repo, "a-1")
	if _, recent := d.promptedRecently(session); !recent {
		t.Errorf("an indeterminate prompt is still a prompt: %s must be inside PromptGrace", session)
	}
}

// A guard against this file's own fixture rotting into a no-op: the lever
// must really produce an error with NO herdr code, because every pin above
// is about what posse does when there is no envelope to read.
func TestPromptCrashLeverHasNoHerdrCode(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-crash"), "3")
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: b.App.WorkspaceLabel("ranger-qa-x-1"), AgentStatus: "idle"}})

	_, err := b.Mux().Prompt(paneSeat("w1:p1"), "the work prompt", true, 10)
	if err == nil {
		t.Fatalf("the lever must fail the prompt")
	}
	if code := HerdrCodeOf(err); code != "" {
		t.Errorf("the lever must produce an error with no envelope, got code %q (%v)", code, err)
	}
	if isNotDelivered(err) {
		t.Errorf("an error with no envelope is not herdr saying nothing was sent: %v", err)
	}
	if !strings.Contains(err.Error(), "agent prompt") {
		t.Errorf("the error must name the call, like Herdr.Run's last arm does: %v", err)
	}
	_ = os.Remove(filepath.Join(fake, "prompt-crash"))
}
