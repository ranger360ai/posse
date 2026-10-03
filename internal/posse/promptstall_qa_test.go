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
