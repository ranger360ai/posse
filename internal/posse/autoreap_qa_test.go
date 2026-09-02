package posse

// Adversarial QA for the end-of-pass auto-reap (rangerhq-us8, verified
// under ranger-base-xa1p). autoreap_test.go pins the spec's own matrix over
// PLAIN sessions; every session dispatch actually creates since ADR 0013 is
// a WORKTREE session, and the reap of one of those runs a merge, a branch
// delete and a tree removal that the plain shape never reaches. These are
// the shapes that were untested:
//
//   - the production shape end to end: worktree session, bead closed, tree
//     retired and branch deleted by the reap;
//   - a worktree the reap CANNOT retire (uncommitted work) — the kill still
//     happens, so the one thing that must not happen is silence;
//   - the shared-checkout dirty warning, which is the whole of coordinator's
//     08-25 sharpening #2 and had no test at all;
//   - the two ways the sweep can be asked a question it cannot answer —
//     bd unreadable, agent gone — where it must fail closed, not open.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// reapCandidateIn is reapCandidate over a caller-chosen dir, so a test can
// hand it a real git checkout rather than a bare temp dir.
func reapCandidateIn(t *testing.T, b *HerdrBackend, dir, name, bead, show string) {
	t.Helper()
	// The kill's own reap guard (reapguard.go) reads the bead through the
	// BACKEND's runner, not the dispatcher's; without this it shells out to
	// the ambient bd and refuses every dirty tree it cannot ask about.
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	if show != "" {
		os.WriteFile(filepath.Join(dir, "fake-show.json"), []byte(show), 0o644)
	}
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: dir, Agent: "ranger", Bead: bead}); err != nil {
		t.Fatal(err)
	}
}

// dispatcherErr captures the sweep's stderr, which errw() otherwise sends
// to the process's own.
func dispatcherErr(t *testing.T, d *Dispatcher) *strings.Builder {
	t.Helper()
	var e strings.Builder
	d.Err = &e
	return &e
}

// fakeBdInTree lets the fake bd answer from inside a session worktree: the
// sweep reads the bead from the SESSION's dir (ADR 0011 — fresh, at reap
// time), and for a worktree session that dir is the tree, not the repo. In
// production the tree's own .beads/redirect carries that (worktree_qa_test
// pins it); the fake reads a file, so it needs one — kept out of git so it
// is not the uncommitted work under test.
func fakeBdInTree(t *testing.T, repo, tree, show string) {
	t.Helper()
	write(t, filepath.Join(tree, "fake-show.json"), show)
	write(t, filepath.Join(repo, ".git", "info", "exclude"), "fake-show.json\nfake-ready.json\n")
}

// ─── the production shape ────────────────────────────────────────────────────

// Every session dispatch creates carries Worktree: true. Reaping one is not
// the plain kill the spec matrix tests: it merges the branch, removes the
// tree and deletes the branch. Pass 1 dispatches and lands the closed
// bead's commit; pass 2 is the one that reaps.
func TestAutoReapRetiresAWorktreeSessionsTreeAndBranch(t *testing.T) {
	wtqaHome(t)
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	fakeBdInTree(t, repo, tr.Path, `[{"id":"a-1","status":"closed"}]`)
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta(session); !ok {
		t.Fatalf("pass 1 must not reap the session it just prompted:\n%s", dispatcherOut(d))
	}

	// Pass 2: nothing ready, the bead is closed, the agent is idle, and the
	// prompt is past PromptGrace — which since ADR 0028 §3 is what "a later
	// pass" has to mean, the guard being the run record's stamp and not a
	// set of names this pass fired at.
	write(t, filepath.Join(repo, "fake-ready.json"), `[]`)
	agePrompt(t, b, session, d.PromptGrace+time.Minute)
	d2 := newTestDispatcher(t, b)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)

	if _, ok := b.readMeta(session); ok {
		t.Errorf("a worktree session whose bead is closed and whose agent is idle must be reaped:\n%s", out)
	}
	if !strings.Contains(out, "reaped "+session+" (bead a-1 closed)") {
		t.Errorf("the reap must say what it did:\n%s", out)
	}
	if _, err := os.Stat(tr.Path); !os.IsNotExist(err) {
		t.Errorf("the reap left %s behind — worktree accumulation is the same leak as session accumulation: %v", tr.Path, err)
	}
	if branchExists(repo, tr.Branch) {
		t.Errorf("the reap left branch %s behind", tr.Branch)
	}
	if !strings.Contains(out, "worktree and "+tr.Branch+" removed") {
		t.Errorf("the reap must report the landing of the tree it retired:\n%s", out)
	}
}

// The reap closes the workspace and drops the meta BEFORE the landing runs,
// so a tree the landing then refuses to remove has nothing left pointing at
// it. Uncommitted work in a session worktree is exactly that case, and the
// one line naming it is the only notice the operator will ever get.
func TestAutoReapNamesAWorktreeItCouldNotRetire(t *testing.T) {
	wtqaHome(t)
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	fakeBdInTree(t, repo, tr.Path, `[{"id":"a-1","status":"closed"}]`)
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	// The persona closed the bead and then kept typing: work in the tree
	// that no commit holds.
	write(t, filepath.Join(tr.Path, "scratch.txt"), "not committed\n")

	write(t, filepath.Join(repo, "fake-ready.json"), `[]`)
	agePrompt(t, b, session, d.PromptGrace+time.Minute)
	d2 := newTestDispatcher(t, b)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)

	if _, err := os.Stat(filepath.Join(tr.Path, "scratch.txt")); err != nil {
		t.Fatalf("the reap destroyed uncommitted work in a session worktree: %v\n%s", err, out)
	}
	if !strings.Contains(out, "KEPT") || !strings.Contains(out, "scratch.txt") {
		t.Errorf("a tree the reap could not retire must be named, with what is in it — the session that pointed at it is gone:\n%s", out)
	}
}

// ─── the shared-checkout warning (coordinator's sharpening #2) ────────────────────

// A session with no worktree has no branch for the landing to refuse over,
// so the landing's own KEPT line cannot fire and the kill is silent. The
// sweep is supposed to name the dirty tree itself. Untested by the spec
// matrix, whose candidates all sit in bare temp dirs where `git status`
// simply errors.
func TestAutoReapWarnsBeforeSweepingADirtySharedCheckout(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	dir := wtRepo(t)
	write(t, filepath.Join(dir, "left-behind.txt"), "353 lines the operator will never find\n")
	reapCandidateIn(t, b, dir, "ranger-repo-a-1", "a-1", `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("a dirty shared checkout under a CLOSED bead is the operator's own scratch — the kill still proceeds")
	}
	// The warning names the DIRECTORY, not the files in it — unlike the
	// worktree half, whose landing line spells out what it kept, and unlike
	// the reap guard's own dirtyList(). Pinned as it stands: the shared
	// checkout is the half with no other safety net, so if this line ever
	// goes quiet a test says so.
	if !strings.Contains(errs.String(), "reap: ranger-repo-a-1 (bead a-1, closed) leaves ") ||
		!strings.Contains(errs.String(), "dirty — no session branch to land it on") {
		t.Errorf("the sweep must name the dirty shared checkout it is about to orphan, got stderr:\n%s", errs.String())
	}
}

// --dry-run acts on nothing, and warning about a tree nothing is going to
// touch is a false alarm.
func TestAutoReapDryRunDoesNotWarnAboutADirtyCheckout(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	errs := dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	dir := wtRepo(t)
	write(t, filepath.Join(dir, "left-behind.txt"), "x\n")
	reapCandidateIn(t, b, dir, "ranger-repo-a-1", "a-1", `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if errs.String() != "" {
		t.Errorf("--dry-run must not warn about a tree it is not going to touch:\n%s", errs.String())
	}
}

// ─── the questions the sweep cannot answer ───────────────────────────────────

// bd is the store of record, and "bd could not say" is not "the bead is
// closed". The condition is live in this repo — an out-of-sync beads
// database answers every `bd show` with an error until it is re-imported.
func TestAutoReapKeepsASessionWhoseBeadBdCannotRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	// No fake-show.json at all: `bd show` fails rather than answering.
	reapCandidateIn(t, b, dir, "ranger-repo-a-1", "a-1", "")
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("a bead whose status bd cannot answer for is not a closed bead — the sweep must fail closed")
	}
}

// ─── the dead shells (ranger-base-kftx) ──────────────────────────────────────

// ageLaunch moves a session's `launched:` stamp back, which is the only way
// a test can say "and this CLI has been gone a while": settledForReap reads
// that stamp through RelaunchGrace to tell a CLI that has EXITED from one
// that has not finished starting. RelaunchGrace is deliberately not
// shortened for tests (ranger-base-ze9p — it is measured against a session's
// real age), so the session has to be aged instead.
func ageLaunch(t *testing.T, b *HerdrBackend, session string, by time.Duration) {
	t.Helper()
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("no meta for %s to age", session)
	}
	m.Launched = m.Launched.Add(-by)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
}

// A session whose CLI exited leaves a bare shell: herdr detects no agent
// there and reports no status at all. That is neither idle/done nor
// working/blocked, and the shipped predicate said nothing about it — three
// of the five candidates measured on the live fleet 2026-08-27 were this,
// and they are the operator's own first complaint on the us8 thread ("we
// have a bunch of dead shells"). Past the launch grace it is a dead shell,
// and the sweep now claims it.
func TestAutoReapSweepsASessionWithNoAgentLeftInIt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(`[]`), 0o644)
	ageLaunch(t, b, "ranger-repo-a-1", d.RelaunchGrace+time.Minute)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Errorf("a bare shell over a closed bead is a dead shell — the sweep must claim it:\n%s", dispatcherOut(d))
	}
	// The two shapes are hand-reaped for different reasons, so the line says
	// which one it found rather than reporting a settled agent that is not
	// there.
	if !strings.Contains(dispatcherOut(d), "reaped ranger-repo-a-1 (bead a-1 closed, no agent left in it)") {
		t.Errorf("the reap must say it found no agent, not report a settled one:\n%s", dispatcherOut(d))
	}
}

// The ambiguity the grace is for: "" is also what a CLI that is still
// starting looks like, because detection is blind for the first seconds of a
// launch (rangerhq-vk2, and RelaunchAgent's own refusal). A session launched
// moments ago is not a dead shell however its bead reads.
func TestAutoReapKeepsASessionWhoseAgentMayStillBeStarting(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(`[]`), 0o644)
	// Launched just now — inside RelaunchGrace, where no-agent-detected and
	// agent-not-detected-YET are the same reading.
	ageLaunch(t, b, "ranger-repo-a-1", d.RelaunchGrace/2)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Errorf("inside the launch grace a missing agent is a starting one — the sweep must wait:\n%s", dispatcherOut(d))
	}
}

// A session herdr calls working or blocked has somebody in it, and the
// widening above must not have quietly made every non-idle status reapable.
// `blocked` is the one that would hurt: it is a persona WAITING on
// something, and it sits in the fire loop's own settled triad.
func TestAutoReapKeepsABlockedSessionOverAClosedBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"blocked","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	ageLaunch(t, b, "ranger-repo-a-1", d.RelaunchGrace+time.Minute)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Errorf("blocked is a persona waiting, not a session nobody is in:\n%s", dispatcherOut(d))
	}
}

// A candidate the sweep has to skip must not end the sweep: it walks every
// session, and the reasons to skip one are per-session. `a-1` sorts first
// and is the one bd cannot answer for.
func TestAutoReapKeepsSweepingPastACandidateItMustSkip(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidateIn(t, b, t.TempDir(), "ranger-repo-a-1", "a-1", "") // no bd answer
	reapCandidate(t, b, "ranger-repo-a-2", "a-2", "closed")
	// Two sessions, two workspaces — herdr must see a settled agent in both
	// or the second one is skipped for a reason this test is not about.
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("the unanswerable candidate must be kept")
	}
	if _, ok := b.readMeta("ranger-repo-a-2"); ok {
		t.Errorf("a skipped candidate must not end the sweep — the next one is a different session:\n%s", dispatcherOut(d))
	}
}

// ─── the boundary of the population ──────────────────────────────────────────

// The sweep's population is "sessions carrying a `bead:` pointer", not
// "sessions whose NAME ends in a bead id" — and the two differ. A session
// hand-launched with a Dial-F-shaped name (`posse new <persona>-<repo>-<id>
// --agent <persona>`) gets no pointer and no worktree, so the sweep cannot
// ask which bead it holds and leaves it standing, forever and silently.
//
// MEASURED on the live fleet, 2026-08-27: five sessions had a settled agent
// and a closed bead; `posse dispatch --dry-run` named two. The other three
// (developer-2-posse-ranger-base-i0s8, reviewer-posse-rangerhq-rukj,
// developer-3-posse-ranger-base-82u, all launched 04:04) carried no `bead:`.
// Failing closed is right — the name is a lossy encoding of the id
// (sessionSanitizeRe folds `.` into `-`), so a name is not an id. This pins
// the boundary so it is a decision on the record rather than a surprise.
//
// ranger-base-kftx read those three as hand-launched, and that attribution
// is CORRECTED: `posse new` marks every session it creates CREW (cmd/posse's
// `case "new"`, and so do `up`/`local` and every recipe), so a hand-launched
// session is skipped one arm EARLIER, on ADR 0008 — see
// TestAutoReapSkipsAHandLaunchedSessionOnTheCrewMarkNotThePointer. What
// carries no pointer and no crew mark is a meta written before the pointer
// landed (4793e00, 2026-08-26; those three carried no `repo:`/`branch:`
// either, which the worktree launch has stamped since 32ccff0 the day
// before), or a `posse new` session the operator later released with
// `posse crew --off`. So the shape stands and this test with it — what
// changed is that `posse ls` now says so out loud (NoBeadTag) instead of
// leaving the operator to conclude the reaper broke.
//
// AMENDED by ranger-base-f6lk: "outside the sweep permanently" was the whole
// complaint by 2026-08-29 — one such session sat idle twelve hours and was
// hand-reaped, which is the mechanism the sweep exists to replace. The
// pointer is still never inferred from the name, and this session is still
// not reaped over a BEAD; what takes it now is its own arm, on age and a
// provably empty tree (reapresidue_test.go). Inside `reap_unpointed_after:`
// nothing has changed, and that is what this pins — the fixture is a
// freshly-created session and stays one deliberately.
func TestAutoReapLeavesADialFNamedSessionThatCarriesNoBeadPointer(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-show.json"), []byte(`[{"id":"a-1","status":"closed"}]`), 0o644)
	// Exactly the name dispatch would have used — and nothing else of it.
	name := SessionForBead("ranger", dir, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: dir, Agent: "ranger"}); err != nil {
		t.Fatal(err)
	}
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Error("a session with no bead pointer must not be reaped on the strength of its name alone — its own arm's grace has not passed and no bead has been read")
	}
}

// ─── the boundary made visible (ranger-base-kftx option b) ───────────────────

// Failing closed on a missing pointer is right and stays right, but doing it
// SILENTLY is what cost the operator the hand-reaps: three sessions sat over
// closed beads and `posse dispatch --dry-run` said nothing about them, which
// reads exactly like a broken reaper. `posse ls` now names them.
//
// The negative arms are the point — a tag that fires on everything says
// nothing. The persona's reusable slot legitimately carries no pointer until
// a bead resumes into it, and a crew session is not MISSING one.
func TestListMarksAPerBeadSessionTheSweepCannotReap(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	unpointed := filepath.Join(t.TempDir(), "alpha")
	pointed := filepath.Join(t.TempDir(), "bravo")
	slotDir := filepath.Join(t.TempDir(), "charlie")
	crewDir := filepath.Join(t.TempDir(), "delta")
	for _, d := range []string{unpointed, pointed, slotDir, crewDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustCreate(t, b, NewSessionOpts{Name: "ranger-alpha-zzz1", Dir: unpointed, Agent: "ranger"})
	mustCreate(t, b, NewSessionOpts{Name: "ranger-bravo-zzz2", Dir: pointed, Agent: "ranger", Bead: "zzz2"})
	mustCreate(t, b, NewSessionOpts{Name: SessionFor("ranger", slotDir), Dir: slotDir, Agent: "ranger"})
	mustCreate(t, b, NewSessionOpts{Name: "ranger-delta-zzz4", Dir: crewDir, Agent: "ranger", Crew: true})

	var out strings.Builder
	if err := b.CmdList(&out); err != nil {
		t.Fatal(err)
	}
	tagged := map[string]bool{}
	for _, ln := range strings.Split(out.String(), "\n") {
		for _, name := range []string{"ranger-alpha-zzz1", "ranger-bravo-zzz2", SessionFor("ranger", slotDir), "ranger-delta-zzz4"} {
			if strings.Contains(ln, name) {
				tagged[name] = strings.Contains(ln, NoBeadTag)
			}
		}
	}
	if len(tagged) != 4 {
		t.Fatalf("expected all four sessions listed, saw %v in:\n%s", tagged, out.String())
	}
	if !tagged["ranger-alpha-zzz1"] {
		t.Errorf("a per-bead-shaped session with no pointer can never be asked about a bead and must say so:\n%s", out.String())
	}
	if tagged["ranger-bravo-zzz2"] {
		t.Errorf("a session the sweep CAN ask about must not wear the tag:\n%s", out.String())
	}
	if tagged[SessionFor("ranger", slotDir)] {
		t.Errorf("the persona's reusable slot carries no pointer by design — tagging it would make the tag noise:\n%s", out.String())
	}
	if tagged["ranger-delta-zzz4"] {
		t.Errorf("ADR 0008 keeps a crew session out of the sweep on its own account, and %s already says so:\n%s", CrewTag, out.String())
	}
}

// The correction the measurement needs, pinned rather than argued: what
// actually keeps a HAND-launched session out of the sweep at HEAD is the
// crew mark, not the missing pointer. `posse new` (cmd/posse/main.go, `case
// "new"`), `posse up`/`local` and every recipe pass Crew: true, and the
// sweep skips on Crew before it ever asks about a bead — so `posse new
// --bead <id>` would buy nothing: the pointer would sit on a session ADR
// 0008 already excludes. Releasing it (`posse crew --off`) is the supported
// way to hand one to dispatch, and dispatch's own NoteBead stamps the
// pointer the moment it resumes a bead into it.
func TestAutoReapSkipsAHandLaunchedSessionOnTheCrewMarkNotThePointer(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-show.json"), []byte(`[{"id":"a-1","status":"closed"}]`), 0o644)
	// Exactly what `posse new <name> --agent ranger` builds, plus
	// the pointer option (a) would add: still not reaped, and the pointer is
	// why we know the crew mark is the arm that fires.
	//
	// The NAME is the operator's, and since ranger-base-f6lk that is
	// load-bearing rather than incidental. f6lk's crew arm reaps a
	// crew-marked session whose name is the one dispatch would have rendered
	// for this persona, dir and bead — the operator stepping into a FLEET
	// session — so this fixture is the shape `posse new` actually produces
	// (it has no --bead flag and cannot cut a worktree, so the pointer can
	// only have arrived later, from `posse prompt`). What stays uncovered is
	// small and stated on the record: an operator who types dispatch's exact
	// name into `posse new` and is then hand-dispatched that same bead is
	// indistinguishable from dispatch's own session.
	name := "ranger-staffing"
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: dir, Agent: "ranger", Bead: "a-1", Crew: true}); err != nil {
		t.Fatal(err)
	}
	idleClaude(t, fake)
	// Far past every grace in the file: the pin is the shape, not the clock.
	ageLaunch(t, b, name, 30*24*time.Hour)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a hand-launched session is the operator's, however its name reads and whatever pointer it carries:\n%s", dispatcherOut(d))
	}
	// And it is not the missing-pointer tag's business either — the crew tag
	// is the one that explains this session.
	var out strings.Builder
	if err := b.CmdList(&out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), NoBeadTag) {
		t.Errorf("a crew session must be explained by %s, not by the sweep-boundary tag:\n%s", CrewTag, out.String())
	}
}
