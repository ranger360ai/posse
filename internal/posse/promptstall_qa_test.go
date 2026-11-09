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

// clearFakeShow drops fake-show.json so `show` falls back to the live state
// overlay (fakeBdState) rather than a stale declared answer. stalledSeat's
// own "closed" is load-bearing for the arms that keep the claim and are
// judged against `show` at settle time (arm 1's "closed by ranger"); an arm
// that hands the bead back BEFORE any such judgement has no use for it, and
// Bd.Unclaim now reads its own write back through Show (ADR 0073 B3) —
// which a fixture's explicit, never-updated `show` answer is the suite's
// own idiom for a write that did NOT land (TestBdUnclaimNotLandedWhenShowDisagrees's
// comment, bdwriteback_test.go). Call this right after stalledSeat in every
// arm that expects the unclaim itself to succeed (ranger-base-vj105).
func clearFakeShow(t *testing.T, repo string) {
	t.Helper()
	if err := os.Remove(filepath.Join(repo, "fake-show.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
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
	d, fake, repo := stalledSeat(t, "agent_prompt_stalled")
	clearFakeShow(t, repo)
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
	// And it names the COMMIT and not the tree (ADR 0084 D3): the two kinds
	// of evidence send the reader somewhere different, so a verdict that
	// said "uncommitted change(s)" over a committed branch would point at a
	// tree there is nothing in.
	if strings.Contains(out, "uncommitted change(s)") {
		t.Errorf("the evidence here is a commit, not a dirty tree:\n%s", out)
	}
	// The control: with the commit gone, the same fixture DOES hand back —
	// so this arm is reading the commit and not merely the worktree.
	if w, read := d.sessionWork(&pendingBead{session: session}); !read || w.Commits == 0 {
		t.Fatalf("setup: the work reading must see the seat's commit, got (%+v, %v)", w, read)
	}
}

// Arm 4b, and the whole of ADR 0084 D3: no turn, NO COMMIT, and the seat's
// tree holding uncommitted work. That is ranger-base-2mqp3's own tree — six
// modified files, one untracked, nothing committed — and under a commit
// count alone it was indistinguishable from a seat that never got the text.
//
// The branch count was enough while the only caller was a stall (herdr
// watched 5s from submission, so a whole turn inside that window is not
// plausible); it is not enough for an indeterminate prompt call, whose error
// can arrive a full `--wait` leg after submission. Either kind of evidence
// means a turn ran, and only "both halves read, both empty" hands back.
//
// MUTATIONS RUN 2026-11-09, over this arm, 4, 4c and
// TestSessionWorkIsATriState (which of them each one reds is named):
//
//	sessionWork ignores the tree half (D3 reverted)      → this arm
//	dirtyPathsRead reads a failed status as a clean tree → tri-state
//	the `if w.found()` short-circuit removed             → tri-state
//	found() needs BOTH kinds of evidence                 → this arm, 4, tri-state
//	the tree half always reports one change (over-keep)  → 4c, this arm, 4, tri-state
//	String() names the commit as dirt and the dirt as a commit → this arm, 4
func TestStalledPromptWhoseTreeHoldsUncommittedWorkKeepsItsClaim(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"open"}]`)
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-error"), "agent_prompt_stalled")

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	// CONTROL, and it is the dependency the tree half rests on: the PREPARED
	// tree is porcelain-clean before anything is written into it. Without
	// this the arm below is green over a reading that found the launcher's
	// own scaffolding — which would keep every claim forever and lose
	// rangerhq-81d (seedScaffoldExcludes, ranger-base-e01op; both prepare
	// paths are measured in
	// TestPreparedSessionTreeIsPorcelainCleanOnEveryPreparePath).
	//
	// Asked of git and not of sessionWork, which reads the session's run
	// record — and there is no record until the pass below creates the
	// session. The sessionWork reading is asserted after the pass.
	if out := mustGit(t, tr.Path, "status", "--porcelain", "--untracked-files=all"); strings.TrimSpace(out) != "" {
		t.Fatalf("CONTROL: a freshly prepared tree must be porcelain-clean, or this arm reads posse's own files as the seat's work:\n%s", out)
	}
	// The 2mqp3 shape: a tracked file edited and a new file written, no commit.
	write(t, filepath.Join(tr.Path, "README.md"), "the seat's edit\n")
	write(t, filepath.Join(tr.Path, "wip.txt"), "the seat's new file\n")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a stalled prompt over a dirty tree must not fail the pass: %v", err)
	}
	out, bd := dispatcherOut(d), bdCalls(t, fake)
	if strings.Contains(bd, "--status open") {
		t.Errorf("a bead whose seat has uncommitted work must not be handed back — that is the 2mqp3 duplicate:\n%s", bd)
	}
	if !strings.Contains(out, "2 uncommitted change(s) in its tree") || !strings.Contains(out, "the prompt landed") {
		t.Errorf("the pass must name the evidence it kept the claim on, and name it as the TREE:\n%s", out)
	}
	if strings.Contains(out, "commit(s) on its own branch") {
		t.Errorf("there is no commit here — a verdict naming one points at a branch with nothing on it:\n%s", out)
	}
	// The reading itself, now that the session has a run record: the dirt is
	// what it found and the branch half really is empty, so the keep above
	// rests on the new half and on nothing else.
	if w, read := d.sessionWork(&pendingBead{session: session}); !read || w.Dirty != 2 || w.Commits != 0 {
		t.Errorf("the work reading must be the tree's two changes and no commit, got (%+v, %v)", w, read)
	}
	// The control on the VERDICT is the next arm: the same fixture with a
	// clean tree hands the bead back, so this one measures the dirt and not
	// the worktree's existence.
}

// Arm 4c, the control on 4b and the reason D3's widening is not a wall:
// rangerhq-81d's own case is a live, idle seat that never got the text, and
// its tree is CLEAN. Both halves of the work reading answer, both find
// nothing, and the bead still goes back after one startup_wait.
//
// Arm 2 reaches the hand-back through a session that shares the checkout —
// where there is no tree to read at all — so without this arm the widening
// could have turned every session WITH a tree into a permanent claim and
// left that arm green.
func TestStalledPromptOverACleanSessionTreeStillHandsBack(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)
	write(t, filepath.Join(fake, "prompt-error"), "agent_prompt_stalled")

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	// CONTROL: the tree is really there, so this arm is the with-a-tree
	// shape and not arm 2's shared checkout wearing a different fixture.
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("CONTROL: the session's own tree must exist for this arm to be about it: %v", err)
	}

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("a stalled prompt over a clean tree must not abort the pass: %v", err)
	}
	out, bd := dispatcherOut(d), bdCalls(t, fake)
	if !strings.Contains(out, "unclaimed") {
		t.Errorf("a clean tree with no commit is both halves ANSWERING nothing — the bead goes back (rangerhq-81d):\n%s", out)
	}
	if !strings.Contains(bd, "--actor ranger update a-1 --status open --assignee  --json") {
		t.Errorf("a-1 must be handed back:\n%s", bd)
	}
	// And the record says which readings got it there, in the words the
	// keep arms use for the same two halves.
	// And it names its TRIGGER, which is the half of ADR 0084 D4 this arm
	// holds: the same decision, read the same way, reached by herdr's own
	// stall rather than by a call that said nothing at all. The census
	// splits the two on this prefix, so a build that tagged every verdict
	// `indeterminate:` would read as having no stalls.
	if rec, ok := lastStallReading(t, b.App, tr.Path); !ok ||
		!strings.HasPrefix(rec.Verdict, "stalled: hand back: ") ||
		!strings.Contains(rec.Verdict, "no commit on its branch and nothing uncommitted in its tree") {
		t.Errorf("the D5 record must name its trigger and both halves of the reading it handed back on: %+v", rec)
	}
}

// lastStallReading reads the newest DecisionStallVerdict record out of the
// session tree's readings log (ADR 0066 D1).
func lastStallReading(t *testing.T, a *App, tree string) (Reading, bool) {
	t.Helper()
	path, err := a.ReadingsLogPath(tree)
	if err != nil {
		t.Fatalf("ReadingsLogPath: %v", err)
	}
	rs, err := ReadReadings(path)
	if err != nil {
		return Reading{}, false
	}
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i].Decision == DecisionStallVerdict {
			return rs[i], true
		}
	}
	return Reading{}, false
}

// Arm 5, the keying. agent_not_ready and agent_blocked are herdr saying it
// sent NOTHING ("submission is rejected ... before any input is sent"), so
// they are evidence about delivery and hand back at once — no grace spent,
// no second reading taken.
func TestRefusedPromptHandsBackWithNoSecondReading(t *testing.T) {
	t.Parallel()
	d, fake, repo := stalledSeat(t, "agent_not_ready")
	clearFakeShow(t, repo)

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
	d, fake, repo := stalledSeat(t, "agent_prompt_stalled")
	clearFakeShow(t, repo)
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
	clearFakeShow(t, repo)
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

// sessionWork's three answers, pinned apart. The middle one is the design:
// "could not be read" and "read, and there is nothing" decide opposite
// things, and the suite above can only reach two of the three through a pass.
//
// SINCE ADR 0084 D3 THERE ARE TWO HALVES to be ignorant about, and the rule
// over them is one line: a half that will not answer is only ignorance when
// the other half found NOTHING. So the arms below walk each half's failure
// on a tree with no work in it — where it must keep the claim — and the last
// one walks a half's failure beside evidence from the other, where the
// verdict is already settled and a failed read changes nothing.
func TestSessionWorkIsATriState(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)

	if w, read := d.sessionWork(&pendingBead{session: "nosuchsession"}); read || w.found() {
		t.Errorf("no run record is ignorance, not an answer: got (%+v, %v)", w, read)
	}

	// A session that shares the checkout: no tree or branch of its own, so
	// there is no such evidence to be had — and that is an ANSWER, the shape
	// every session had before per-session worktrees landed. Reading the
	// shared checkout's own status here would read every other persona's
	// dirt as this bead's work.
	mustCreate(t, b, NewSessionOpts{Name: "shared"})
	if w, read := d.sessionWork(&pendingBead{session: "shared"}); !read || w.found() {
		t.Errorf("a session with no tree reads nothing, readably: got (%+v, %v)", w, read)
	}

	// A record naming a tree and a branch that are not there. Two ways git
	// says nothing and both keep the claim: no base to count against, and a
	// head that cannot be read. The tree's status is unreadable in both —
	// the directory is not there — so these also pin the new half.
	m, ok := b.readMeta("shared")
	if !ok {
		t.Fatal("no meta for shared")
	}
	gone := filepath.Join(t.TempDir(), "gone")
	m.Repo, m.Dir, m.Branch = gone, gone, "posse/shared"
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	if w, read := d.sessionWork(&pendingBead{session: "shared"}); read || w.found() {
		t.Errorf("a repo that is not there has no base to count against: got (%+v, %v)", w, read)
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
	if w, read := d.sessionWork(&pendingBead{session: "shared"}); read || w.found() {
		t.Errorf("a head git will not name is ignorance: got (%+v, %v)", w, read)
	}

	// THE POINT OF THE SPLIT, and the half no arm above isolates: a branch
	// git counts fine over a TREE it will not read. The commit half answers
	// "zero" and the tree half answers nothing at all, so this is ignorance
	// and the claim stays — a `dirtyPaths` that reads a failed status as
	// "clean" would hand this bead back (which is why sessionWork takes
	// dirtyPathsRead).
	repo := wtRepo(t)
	tr, err := b.App.EnsureSessionTree(repo, "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	m.Repo, m.Dir, m.Branch = repo, tr.Path, tr.Branch
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	// CONTROL: as prepared, both halves answer and both find nothing. That
	// is the arm that CAN hand back, and it is what the next two move off.
	if w, read := d.sessionWork(&pendingBead{session: "shared"}); !read || w.found() {
		t.Fatalf("CONTROL: a prepared tree reads as no work, answered — got (%+v, %v)", w, read)
	}
	// Now take the tree's status away without touching the branch: the
	// worktree's own git dir is where `status` reads the index from, and a
	// linked worktree whose administrative dir is gone answers nothing while
	// `refs/heads/<branch>` in the main repo is untouched.
	own := LinkedGitDirs(tr.Path)
	if len(own) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", own)
	}
	if err := os.Rename(own[0], own[0]+".away"); err != nil {
		t.Fatal(err)
	}
	if _, ok := commitsAhead(SessionTreeOf(m)); !ok {
		t.Fatalf("setup: the COMMIT half has to still answer, or this arm is the previous one again")
	}
	if _, ok := dirtyPathsRead(tr.Path); ok {
		t.Fatalf("setup: the TREE half has to be unreadable for this arm to be about it")
	}
	w, read := d.sessionWork(&pendingBead{session: "shared"})
	if read || w.found() {
		t.Errorf("a tree whose status git will not read is ignorance, not an empty tree: got (%+v, %v)", w, read)
	}
	if !strings.Contains(w.Unread, "tree's status") {
		t.Errorf("the verdict has to be able to say WHICH half went unread, got %q", w.Unread)
	}

	// And the asymmetry: with a commit on the branch, the same unreadable
	// tree is not ignorance at all. Evidence beats a failed read — a reading
	// that found a commit does not need the tree's status to know a turn ran.
	if err := os.Rename(own[0]+".away", own[0]); err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "the fix")
	if err := os.Rename(own[0], own[0]+".away"); err != nil {
		t.Fatal(err)
	}
	if w, read := d.sessionWork(&pendingBead{session: "shared"}); !read || w.Commits == 0 {
		t.Errorf("a commit is an answer whatever the tree says: got (%+v, %v)", w, read)
	}
}

// The verdict's one spelling, which the print, the keep record and the
// hand-back record all share (ADR 0084 D3: the evidence is NAMED, because a
// commit sends the reader to `posse worktrees` and a dirty tree sends them
// to the seat).
func TestWorkEvidenceNamesWhichEvidenceItFound(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		w    workEvidence
		want string
	}{
		{workEvidence{}, "no commit on its branch and nothing uncommitted in its tree"},
		{workEvidence{Commits: 2}, "2 commit(s) on its own branch"},
		{workEvidence{Dirty: 7}, "7 uncommitted change(s) in its tree"},
		{workEvidence{Commits: 1, Dirty: 7}, "1 commit(s) on its own branch and 7 uncommitted change(s) in its tree"},
	} {
		if got := c.w.String(); got != c.want {
			t.Errorf("%+v renders %q, want %q", c.w, got, c.want)
		}
		if got := c.w.found(); got != (c.w.Commits > 0 || c.w.Dirty > 0) {
			t.Errorf("%+v found()=%v", c.w, got)
		}
	}
}
