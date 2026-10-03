//go:build posse_arm2

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── a stalled prompt is an unobserved turn (ranger-base-uauvn) ──────────────
//
// The incident these pin: the pass prompted a seat under loadavg 94, herdr
// saw no turn start inside its own 5s window and returned
// agent_prompt_stalled, and dispatch handed the bead back — while the seat
// went on to measure, edit five files and commit on its branch. The bead was
// open and ready with finished work on it (promptstall.go carries the
// measurement and herdr's own wording for why the code is not about
// delivery).
//
// Each arm here is one answer the second reading can get, and the pair that
// matters is arms 1-4 against arm 5: the hand-back is NOT removed, it is
// narrowed to the readings that are evidence about delivery.

// stalledSeat is the fixture every arm starts from: one ready `go` bead, one
// idle claude, and a prompt that comes back with the herdr code named.
func stalledSeat(t *testing.T, code string) (*Dispatcher, string, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-error"), code)
	return d, fake, repo
}

// stallWait arms the answer herdr gives the SECOND reading — the `agent wait
// --until working --until blocked` posse makes on its own clock. "code|msg"
// is the refusal arm.
func stallWait(t *testing.T, fake, answer string) {
	t.Helper()
	write(t, filepath.Join(fake, "stall-wait-status"), answer)
}

// Arm 1, the incident itself. herdr's window missed the start of the turn;
// posse's own wait sees it. The claim must stay, the leg must be waited
// again, and the bead must reach its ordinary judgement — the pass that used
// to lose this bead now dispatches it.
func TestStalledPromptWhoseTurnStartsLateKeepsItsClaim(t *testing.T) {
	t.Parallel()
	d, fake, _ := stalledSeat(t, "agent_prompt_stalled")
	stallWait(t, fake, "working")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("a stalled prompt whose turn started must not fail the pass: %v", err)
	}
	out := dispatcherOut(d)
	if n != 1 || !strings.Contains(out, "closed by ranger") {
		t.Errorf("the bead was worked and closed, so the pass judges it closed: got n=%d\n%s", n, out)
	}
	if strings.Contains(out, "unclaimed") {
		t.Errorf("a turn that started is not a prompt that never landed:\n%s", out)
	}
	if bd := bdCalls(t, fake); strings.Contains(bd, "--status open") {
		t.Errorf("the claim must be kept — this is the double-hire ranger-base-uauvn is about:\n%s", bd)
	}
	if !strings.Contains(out, "a turn is under way") || !strings.Contains(out, "claim kept, waiting again") {
		t.Errorf("the operator is owed the reason the stall was not a hand-back:\n%s", out)
	}
	// The re-read is herdr's own stall question with posse's patience, and
	// the settle leg after it is the ordinary one — both or the bead is
	// judged on a state nobody waited for.
	log := calls(t, fake)
	if !strings.Contains(log, "--until working --until blocked") {
		t.Errorf("the second reading asks herdr for the turn it did not see:\n%s", log)
	}
	if got := strings.Count(log, "--until idle --until done --until blocked"); got < 1 {
		t.Errorf("the settle leg must be waited again after the stall:\n%s", log)
	}
}

// Arm 2, the narrowing's other side. Nothing says the turn started and
// nothing says anything was committed, so the bead IS handed back — the
// rangerhq-81d cleanup is still there, and a stalled prompt over a seat that
// really did nothing still frees the bead.
func TestStalledPromptWithNoTurnAndNoCommitStillHandsBack(t *testing.T) {
	t.Parallel()
	d, fake, _ := stalledSeat(t, "agent_prompt_stalled")
	// No stall-wait lever: the fake answers the second reading `idle`, which
	// is herdr saying "no turn" rather than refusing to say.

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a stalled prompt must not abort the pass: %v", err)
	}
	out, bd := dispatcherOut(d), bdCalls(t, fake)
	if !strings.Contains(out, "unclaimed") {
		t.Errorf("nothing said the prompt landed, so the bead goes back (rangerhq-81d):\n%s", out)
	}
	if !strings.Contains(bd, "--actor ranger update a-1 --status open --assignee  --json") {
		t.Errorf("a-1 must be handed back:\n%s", bd)
	}
	// Ask 3: the pass's line is not what the bead's next reader has.
	if !strings.Contains(bd, "comments add a-1") {
		t.Errorf("an unclaim must say so on the bead:\n%s", bd)
	}
	if !strings.Contains(bd, "agent_prompt_stalled") || !strings.Contains(bd, "posse worktrees") {
		t.Errorf("the comment must name the herdr code and where the work would be:\n%s", bd)
	}
}

// Arm 3. herdr will not say what the agent is doing. That is IGNORANCE and
// never a verdict (rangerhq-khc): the claim stays and the bead is not judged
// this pass.
func TestStalledPromptOverAnUnreadableAgentKeepsItsClaim(t *testing.T) {
	t.Parallel()
	d, fake, _ := stalledSeat(t, "agent_prompt_stalled")
	stallWait(t, fake, "server_unreachable|herdr went away")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("an unreadable agent must not fail the pass: %v", err)
	}
	out, bd := dispatcherOut(d), bdCalls(t, fake)
	if !strings.Contains(out, "still with their agent — claims kept") {
		t.Errorf("the leg stays in flight and the pass reports it:\n%s", out)
	}
	if strings.Contains(bd, "--status open") {
		t.Errorf("a herdr that cannot be asked is not evidence the prompt never landed:\n%s", bd)
	}
	if !strings.Contains(out, "claim kept, not judged this pass") {
		t.Errorf("the pass must say where to look:\n%s", out)
	}
}

// Arm 4. No turn was observed — but the session has COMMITTED. A branch is
// cut per bead (SessionForBead), so that commit is this bead's work and a
// hand-back would be the title of the bead: a finished branch on an open
// bead.
func TestStalledPromptWhoseBranchHoldsACommitKeepsItsClaim(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"open"}]`)
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-error"), "agent_prompt_stalled")

	// The seat's own tree with the seat's own commit on it — EnsureSessionTree
	// is idempotent, so the pass lands in the tree that is already there.
	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a stalled prompt over a branch with work must not fail the pass: %v", err)
	}
	out, bd := dispatcherOut(d), bdCalls(t, fake)
	if strings.Contains(bd, "--status open") {
		t.Errorf("a bead with a commit on its own branch must not be handed back:\n%s", bd)
	}
	if !strings.Contains(out, "commit(s) on its own branch") || !strings.Contains(out, "the prompt landed") {
		t.Errorf("the pass must name the evidence it kept the claim on:\n%s", out)
	}
	// The control: with the commit gone, the same fixture DOES hand back —
	// so this arm is reading the commit and not merely the worktree.
	if n, read := d.committedWork(&pendingBead{session: session}); !read || n == 0 {
		t.Fatalf("setup: the commit reading must see the seat's commit, got (%d, %v)", n, read)
	}
}

// Arm 5, the keying. agent_not_ready and agent_blocked are herdr saying it
// sent NOTHING ("submission is rejected ... before any input is sent"), so
// they are evidence about delivery and hand back at once — no grace spent,
// no second reading taken.
func TestRefusedPromptHandsBackWithNoSecondReading(t *testing.T) {
	t.Parallel()
	d, fake, _ := stalledSeat(t, "agent_not_ready")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a refused prompt must not abort the pass: %v", err)
	}
	out, bd, log := dispatcherOut(d), bdCalls(t, fake), calls(t, fake)
	if !strings.Contains(out, "unclaimed") || !strings.Contains(bd, "--status open") {
		t.Errorf("a submission herdr refused sends nothing — the bead goes back at once:\n%s\n%s", out, bd)
	}
	if strings.Contains(log, "--until working") {
		t.Errorf("the second reading is keyed on agent_prompt_stalled, not on every prompt failure:\n%s", log)
	}
	if !strings.Contains(bd, "comments add a-1") || !strings.Contains(bd, "agent_not_ready") {
		t.Errorf("every unclaim says so on the bead, naming the code:\n%s", bd)
	}
}

// Arm 6, the branch the incident actually takes — and the one no arm above
// reaches (ranger-base-szxlx, verifying this file's close).
//
// afterStall has two ways to say "no turn", and they are different events:
// herdr NAMES a state that is not working or blocked (the `default:` arm,
// "herdr answered %q rather than a turn starting"), or herdr watches the
// whole grace out and returns its own `timeout` code — which is what a seat
// whose screen has not moved yet looks like, i.e. the loadavg-94 incident
// this file exists for. Both verdicts are stallNoTurn, so on the hand-back
// path the two are observationally identical, which is exactly how one of
// them ends up unpinned.
//
// MEASURED 2026-10-03: with no lever the fake answers the second reading
// `idle`, so arms 2 and 4 — the only two that reach a hand-back — both
// travel `default:`. Inverting the TIMEOUT arm's verdict to stallUnreadable
// (never hand back) left all eight arms of this file green; inverting
// `default:`'s reds arms 2 and 4. Two guards, one visible outcome: deleting
// the one no fixture drives is free.
//
// What this arm pins is the VERDICT on that branch, which is what the
// mutant moved. It does not pin the two branches' sentences apart — on a
// hand-back judgeStall prints neither, and the distinguishing text is only
// reachable on the keep-the-claim side.
func TestStalledPromptWhoseGraceExpiresWithNoTurnHandsBack(t *testing.T) {
	t.Parallel()
	d, fake, _ := stalledSeat(t, "agent_prompt_stalled")
	// herdr accepted the submission, then watched posse's whole grace out
	// without seeing a transition — its own `timeout`, not a named state.
	stallWait(t, fake, "timeout|timed out waiting for agent status")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a stalled prompt whose grace expired must not abort the pass: %v", err)
	}
	out, bd, log := dispatcherOut(d), bdCalls(t, fake), calls(t, fake)

	// THE FIXTURE PREMISE, asserted rather than assumed: the second reading
	// was actually taken, and it is the leg the lever answers. Without this
	// the arm below is also green over a build that never asks herdr again
	// and hands back on the code alone — the ranger-base-uauvn defect.
	if !strings.Contains(log, "--until working --until blocked") {
		t.Fatalf("setup: the second reading was never asked, so this arm is not about afterStall's timeout verdict:\n%s", log)
	}
	// And the verdict: herdr answered, and what it answered was "no turn in
	// the whole grace either". That is evidence, not ignorance, so the
	// rangerhq-81d cleanup still happens.
	if !strings.Contains(out, "unclaimed") {
		t.Errorf("a grace that expired with no turn is herdr ANSWERING, not refusing to answer — the bead goes back:\n%s", out)
	}
	if !strings.Contains(bd, "--actor ranger update a-1 --status open --assignee  --json") {
		t.Errorf("a-1 must be handed back:\n%s", bd)
	}
	// The pair that makes the verdict legible rather than silent, same as
	// arm 2: the bead says why it is open and names the code.
	if !strings.Contains(bd, "comments add a-1") || !strings.Contains(bd, "agent_prompt_stalled") {
		t.Errorf("the unclaim must say so on the bead, naming the code:\n%s", bd)
	}
	// The control on the arm's own reading of "answered": an unreadable
	// herdr is the OTHER outcome on the same leg, and arm 3 pins it. If this
	// arm's lever were being read as a refusal, it would print arm 3's line.
	if strings.Contains(out, "claim kept, not judged this pass") {
		t.Errorf("herdr's own timeout is an answer, not a herdr that could not be asked (that is arm 3):\n%s", out)
	}
}

// A comment the store would not take must not turn a cleaned-up claim into a
// stranded one: the unclaim stands, the pass says what could not be written.
func TestUnclaimSurvivesABeadThatWillNotTakeTheComment(t *testing.T) {
	t.Parallel()
	d, fake, repo := stalledSeat(t, "agent_not_ready")
	write(t, filepath.Join(repo, "fake-comment-fail"), "database is locked")
	errw := dispatcherErr(t, d)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a comment that could not be written must not abort the pass: %v", err)
	}
	if bd := bdCalls(t, fake); !strings.Contains(bd, "--status open") {
		t.Errorf("the unclaim itself still has to happen:\n%s", bd)
	}
	if got := errw.String(); !strings.Contains(got, "could not be told why") {
		t.Errorf("a bead that could not be written is said out loud:\n%s", got)
	}
}

// The grace is the runtime's own startup_wait — the one measured number in
// this shop for "how long this CLI takes to put something on screen" — and
// not a number coined here. A runtime that declares one gets it; everything
// else gets the pass default.
func TestStallGraceIsTheRuntimesStartupWait(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	if got := d.stallGrace("nosuchruntime"); got != d.StartupWait {
		t.Errorf("an undeclared runtime takes the pass default %s, got %s", d.StartupWait, got)
	}
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRuntime(t, b.App, "slowcli", "command: slowcli\nstartup_wait: 90s\n")
	if got := d.stallGrace("slowcli"); got != 90*time.Second {
		t.Errorf("a declared startup_wait is the grace, got %s", got)
	}
}

// committedWork's three answers, pinned apart. The middle one is the design:
// "could not be read" and "read, and there is nothing" decide opposite
// things, and the suite above can only reach two of the three through a pass.
func TestCommittedWorkIsATriState(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)

	if n, read := d.committedWork(&pendingBead{session: "nosuchsession"}); read || n != 0 {
		t.Errorf("no run record is ignorance, not an answer: got (%d, %v)", n, read)
	}

	// A session that shares the checkout: no branch of its own, so there is
	// no such evidence to be had — and that is an ANSWER, the shape every
	// session had before per-session worktrees landed.
	mustCreate(t, b, NewSessionOpts{Name: "shared"})
	if n, read := d.committedWork(&pendingBead{session: "shared"}); !read || n != 0 {
		t.Errorf("a session with no branch reads zero, readably: got (%d, %v)", n, read)
	}

	// A record naming a tree and a branch that are not there. Two ways git
	// says nothing and both keep the claim: no base to count against, and a
	// head that cannot be read.
	m, ok := b.readMeta("shared")
	if !ok {
		t.Fatal("no meta for shared")
	}
	gone := filepath.Join(t.TempDir(), "gone")
	m.Repo, m.Dir, m.Branch = gone, gone, "posse/shared"
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	if n, read := d.committedWork(&pendingBead{session: "shared"}); read || n != 0 {
		t.Errorf("a repo that is not there has no base to count against: got (%d, %v)", n, read)
	}
	// A real repo (so the base falls back to its own branch) with the tree
	// and the branch missing: this is the workHead arm.
	m.Repo, m.Dir, m.Branch = wtRepo(t), gone, "posse/nope"
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	if t2 := SessionTreeOf(m); t2 == nil || t2.Base == "" {
		t.Fatalf("setup: this arm has to reach workHead, not the base check: %+v", t2)
	}
	if n, read := d.committedWork(&pendingBead{session: "shared"}); read || n != 0 {
		t.Errorf("a head git will not name is ignorance: got (%d, %v)", n, read)
	}
}
