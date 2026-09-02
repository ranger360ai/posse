package posse

// The residue arms of the end-of-pass sweep (ranger-base-f6lk): the two
// populations autoreap.go skipped PERMANENTLY, and which the operator was
// therefore reaping by hand — a crew mark on a session dispatch made, and a
// per-bead-named session with no `bead:` pointer at all.
//
// Every test here is a pair. The widened arm alone would be "the sweep takes
// more", which is not the bead: the bead asked for a PREDICATE, and a
// predicate is only pinned by what it refuses. So each reap below is followed
// by the nearest shape that must survive it — the same session inside its
// grace, the same session with an operator-chosen name, the same session over
// a tree that still holds something.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// residueSession is a settled session in a dir of its own with a `bd show`
// answer the test controls, aged so that neither the launch nor the prompt
// stamp is fresh. by is how far back BOTH stamps go: residueIdle believes the
// later of the two, so a test that ages one and not the other measures the
// other.
func residueSession(t *testing.T, b *HerdrBackend, o NewSessionOpts, beadStatus string, by time.Duration) {
	t.Helper()
	if o.Dir == "" {
		o.Dir = t.TempDir()
	}
	if beadStatus != "" {
		write(t, filepath.Join(o.Dir, "fake-show.json"), `[{"id":"`+o.Bead+`","status":"`+beadStatus+`"}]`)
	}
	if o.Agent == "" {
		o.Agent = "ranger"
	}
	if err := b.CreateSession(o); err != nil {
		t.Fatal(err)
	}
	ageResidue(t, b, o.Name, by)
}

// ageResidue moves both stamps residueIdle reads back by the same amount.
// MarkPrompted refuses to move `prompted:` backwards — it is a high-water
// mark — so this writes the record, exactly as agePrompt does.
func ageResidue(t *testing.T, b *HerdrBackend, session string, by time.Duration) {
	t.Helper()
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("no run record for %s to age", session)
	}
	m.Launched = m.Launched.Add(-by)
	if !m.Prompted.IsZero() {
		m.Prompted = m.Prompted.Add(-by)
	}
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
}

// ─── the crew arm: a mark on a session dispatch made ─────────────────────────

// The measured shape (dispatch-watch.log, 2026-08-29):
// `<persona>-<repo>-ranger-base-3j3t` — a session dispatch created for one
// bead, then crew-marked by cockpit `p` or `posse prompt`, and skipped by the
// ADR 0008 shield on every pass thereafter. Its bead is closed, its agent is
// settled, its tree holds nothing, and it is not a conversation the operator
// ever made: it is Dial F's own per-bead session wearing a 👤.
func TestAutoReapTakesACrewMarkedSessionDispatchMadePastItsGrace(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1", Crew: true},
		"closed", DefaultCrewReapAfter+time.Minute)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a crew mark on a session dispatch made is the operator stepping into a FLEET session — past its grace, over a closed bead and an empty tree, it is the sweep's:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "reaped "+name+" (bead a-1 closed, crew-marked on a session dispatch made, idle ") {
		t.Errorf("the line must say which arm took it — the operator reads this to know the shield did not simply stop working:\n%s", out)
	}
}

// The grace is the whole of the concession ADR 0008 §1 refused a timer for:
// a conversation has no timeout, so the only honest thing a clock can buy is
// a long one. Inside it the shield holds exactly as it always did.
func TestAutoReapKeepsACrewMarkedSessionInsideItsGrace(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1", Crew: true},
		"closed", DefaultCrewReapAfter-time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("inside the grace a crew session is untouched — a conversation with a gap in it is not residue:\n%s", dispatcherOut(d))
	}
}

// The bead's "not the operator's own", and the promise that is stronger than
// the longer grace it asked for: a session the operator MADE is never taken,
// at any age. `posse new` cannot set `Bead:` (no flag does) and cannot cut a
// worktree, so a hand-made session that carries a pointer got it from `posse
// prompt` afterwards — ADR 0008's adb7 amendment, the operator hand-dispatching
// into their own conversation. The name is what tells the two apart, because
// the name dispatch WOULD have given this session can be re-rendered from the
// session's own record and compared.
func TestAutoReapNeverTakesACrewSessionTheOperatorMade(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	// An operator-chosen name — what `posse new ranger-staffing` builds —
	// plus the pointer a later `posse prompt` stamps, plus a closed bead and
	// a month of idleness. Still not the sweep's.
	residueSession(t, b, NewSessionOpts{Name: "ranger-staffing", Dir: dir, Bead: "a-1", Crew: true},
		"closed", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-staffing"); !ok {
		t.Errorf("a session the operator made is theirs however old it gets — the crew arm takes only the name dispatch itself would have used:\n%s", dispatcherOut(d))
	}
}

// The bead's "or the coordinator", by name and not by shape: ADR 0027's carve-out
// delivers the shop check into pulse_persona:'s live session, and reaping it
// turns every later tick into "undeliverable (no live session for X)". The
// exclusion is asked of the PERSONA, so it holds even for a session shaped
// exactly like the one the crew arm takes.
func TestAutoReapNeverTakesThePulsePersonasSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "coordinator", "[go]")
	write(t, b.App.ConfigPath, "pulse_persona: coordinator\n")
	dir := t.TempDir()
	name := SessionForBead("coordinator", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Agent: "coordinator", Bead: "a-1", Crew: true},
		"closed", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("the pulse has nowhere to deliver once its target is reaped (ADR 0027):\n%s", dispatcherOut(d))
	}
}

// ─── the stampless arm: a per-bead name with no pointer ──────────────────────

// ranger-base-kftx measured three of these on the live fleet and made them
// VISIBLE (🏷️no-bead) rather than reapable, because nothing can ever supply
// the pointer: NoteBead stamps only a session dispatch resumes into, and a
// closed bead is never dispatched again. Visible was not enough — one sat
// idle twelve hours and the operator reaped it by hand, which is the
// mechanism this file exists to replace.
func TestAutoReapTakesAStamplessSessionPastItsGrace(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir}, "", DefaultUnpointedReapAfter+time.Minute)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a per-bead-named session with no pointer is outside the pointer sweep FOREVER — past its grace, over an empty tree, it is this arm's:\n%s", dispatcherOut(d))
	}
	if !strings.Contains(dispatcherOut(d), "reaped "+name+" (no bead pointer, idle ") {
		t.Errorf("the line must name the arm — a session reaped for having no pointer is not a session reaped over a closed bead:\n%s", dispatcherOut(d))
	}
}

func TestAutoReapKeepsAStamplessSessionInsideItsGrace(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir}, "", DefaultUnpointedReapAfter/2)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("inside the grace nothing changes — a session minutes old with no pointer yet is not residue:\n%s", dispatcherOut(d))
	}
}

// The persona's reusable slot legitimately carries no pointer between beads
// and must never be taken by the arm that reaps sessions for not having one
// (rangerhq-v330's join depends on it surviving).
func TestAutoReapNeverTakesTheSlotForHavingNoPointer(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	slot := SessionFor("ranger", dir)
	residueSession(t, b, NewSessionOpts{Name: slot, Dir: dir}, "", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(slot); !ok {
		t.Errorf("the persona's repo slot is the session the next resume rejoins, and carries no pointer by design:\n%s", dispatcherOut(d))
	}
}

// ─── never a tree that holds work (the codex 353-line lesson) ────────────────

// The narrow arm proceeds over a dirty tree and warns (ADR 0041's business).
// These two do not: they rest on less evidence, so a kill must take nothing.
// And the refusal SPEAKS — a session left standing with no line said about it
// is exactly what read as a broken reaper and cost the hand-reaps (kftx).
func TestAutoReapWillNotTakeResidueOverAnUncommittedTree(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := SessionForBead("ranger", repo, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: repo}, "", 30*24*time.Hour)
	write(t, filepath.Join(repo, "unsaved.txt"), "353 lines nobody committed\n")
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a tree holding uncommitted work is never reaped by the widened arms — a kill would be the only copy's last moment:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "NOT reaped") || !strings.Contains(out, "unsaved.txt") {
		t.Errorf("the refusal must name the session AND what it is holding, every pass it is true:\n%s", out)
	}
}

// The as19/x8jp half of the same question, asked of the branch instead of the
// working tree: a session worktree whose commits the base does not have is
// the last copy of them, however finished the session looks.
func TestAutoReapWillNotTakeResidueHoldingUnlandedCommits(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "work nothing else holds\n", "the fix")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: repo, Worktree: true}, "", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a branch holding commits main does not have is not residue, whatever the session looks like:\n%s", dispatcherOut(d))
	}
	if out := dispatcherOut(d); !strings.Contains(out, "NOT reaped") || !strings.Contains(out, tr.Branch) {
		t.Errorf("the refusal must name the branch and the count, so `posse worktrees` is the obvious next command:\n%s", out)
	}
}

// And the other side of that pair: once the work IS on the base, the same
// session is taken. Without this the test above is satisfied by an arm that
// refuses everything.
func TestAutoReapTakesResidueOnceItsBranchHasLanded(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtRepo(t)
	name := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "work nothing else holds\n", "the fix")
	mustGit(t, repo, "merge", "--ff-only", tr.Branch)
	residueSession(t, b, NewSessionOpts{Name: name, Dir: repo, Worktree: true}, "", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a branch whose every commit the base holds is the last copy of nothing:\n%s", dispatcherOut(d))
	}
}

// ─── the arms are config, and `off` is the old behaviour ─────────────────────

// ADR 0008 §2's permanent skip is one config line away, for an operator who
// reads the widening as the wrong trade. Both arms, separately, because they
// rest on separate evidence and an operator may well want one and not the
// other.
func TestReapGracesOffRestoreThePermanentSkip(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	write(t, b.App.ConfigPath, "reap_crew_after: off\nreap_unpointed_after: never\n")
	crewDir, stampDir := t.TempDir(), t.TempDir()
	crew := SessionForBead("ranger", crewDir, "a-1")
	stampless := SessionForBead("ranger", stampDir, "a-2")
	residueSession(t, b, NewSessionOpts{Name: crew, Dir: crewDir, Bead: "a-1", Crew: true}, "closed", 30*24*time.Hour)
	residueSession(t, b, NewSessionOpts{Name: stampless, Dir: stampDir}, "", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(crew); !ok {
		t.Error("reap_crew_after: off must restore ADR 0008 §2's permanent skip")
	}
	if _, ok := b.readMeta(stampless); !ok {
		t.Error("reap_unpointed_after: never must restore kftx's permanent skip")
	}
}

// A grace that will not parse is NAMED and the default stands. Reading a typo
// as zero would turn one bad config line into a sweep with no grace at all,
// which is why `off` is spelled and not numeric.
func TestUnreadableReapGraceIsNamedAndTheDefaultStands(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	e := dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	write(t, b.App.ConfigPath, "reap_crew_after: soonish\n")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1", Crew: true},
		"closed", DefaultCrewReapAfter-time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a config typo must not shorten the grace to nothing:\n%s", dispatcherOut(d))
	}
	if !strings.Contains(e.String(), "reap_crew_after") {
		t.Errorf("an unreadable grace must be named on stderr, not silently defaulted:\n%s", e.String())
	}
}

// ─── the guards that already existed still fire first ────────────────────────

// settledForReap is asked of all three arms, and it is the only thing
// standing between the widened sweep and a live conversation: typing in a
// pane is invisible to posse's own stamps (ADR 0008 §1 accepted that) but
// herdr reports it as `working`.
func TestAutoReapKeepsWorkingResidueHoweverOld(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	crewDir, stampDir := t.TempDir(), t.TempDir()
	crew := SessionForBead("ranger", crewDir, "a-1")
	stampless := SessionForBead("ranger", stampDir, "a-2")
	residueSession(t, b, NewSessionOpts{Name: crew, Dir: crewDir, Bead: "a-1", Crew: true}, "closed", 30*24*time.Hour)
	residueSession(t, b, NewSessionOpts{Name: stampless, Dir: stampDir}, "", 30*24*time.Hour)
	// Both workspaces, not just the first: workingClaude names w1 alone, and
	// a second session herdr reports NOTHING for is `""` — which past
	// RelaunchGrace is settled, not working, and would make this test's
	// second arm pass for the wrong reason.
	write(t, filepath.Join(fake, "agents.json"),
		`[{"agent":"claude","agent_status":"working","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"working","pane_id":"w2:p1","workspace_id":"w2"}]`)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(crew); !ok {
		t.Error("a session herdr calls working is somebody being in there — the age says nothing against it")
	}
	if _, ok := b.readMeta(stampless); !ok {
		t.Error("the same, for the stampless arm")
	}
}

// A meta with neither stamp has no age, and no age is not "old enough" — the
// same fail-closed the unreadable bead gets. Reachable for a record written
// before `launched:` existed.
func TestUndatedResidueIsNotOldEnough(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir}, "", 0)
	m, _ := b.readMeta(name)
	m.Launched, m.Prompted = time.Time{}, time.Time{}
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Errorf("a record that cannot be dated must fail closed, not read as infinitely old:\n%s", dispatcherOut(d))
	}
}

// --dry-run says what it would take on the widened arms too, and takes
// nothing. The flag's promise is that a diagnostic pass changes no state.
func TestReapResidueDryRunOnlyLists(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir}, "", 30*24*time.Hour)
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Error("--dry-run must not kill residue either")
	}
	if !strings.Contains(dispatcherOut(d), "would reap "+name+" (no bead pointer, idle ") {
		t.Errorf("--dry-run must say what it would take and why:\n%s", dispatcherOut(d))
	}
}

// ─── the unpointed arm waits for routing ─────────────────────────────────────

// A stampless session is not unambiguously residue: dispatch reaches a
// session by NAME, pointer or no pointer, so one sitting at a live bead's
// Dial F name is a SEAT this pass is about to relaunch into and reuse
// (rangerhq-vk2), not a dead shell. The pass-start sweep cannot tell the two
// apart — nobody has asked which beads want which sessions yet — so the arm
// holds until a sweep that runs past routing, where anything the pass used
// has either been prompted (promptedRecently) or been stamped with a pointer
// by NoteBead and left this population.
func TestUnpointedArmHoldsUntilThePassHasRouted(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir}, "", DefaultUnpointedReapAfter+time.Minute)
	idleClaude(t, fake)

	d.autoReapPass(beforeRouting)

	if _, ok := b.readMeta(name); !ok {
		t.Fatalf("the pass-start sweep must not take a session this pass may be about to reuse:\n%s", dispatcherOut(d))
	}

	// The same session, the same sweep, past routing: now it is residue.
	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("past routing, a session still carrying no pointer is one no bead in this pass's queue claimed:\n%s", dispatcherOut(d))
	}
}

// The crew arm keeps BOTH sites, and must: its bead is closed, and a closed
// bead is never dispatched again, so no pass is coming for that session at
// any point. Making the two arms share one rule would cost the crew arm the
// starvation fix ranger-base-v674 added the pass-start sweep for.
func TestCrewArmSweepsBeforeRoutingToo(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	name := SessionForBead("ranger", dir, "a-1")
	residueSession(t, b, NewSessionOpts{Name: name, Dir: dir, Bead: "a-1", Crew: true},
		"closed", DefaultCrewReapAfter+time.Minute)
	idleClaude(t, fake)

	d.autoReapPass(beforeRouting)

	if _, ok := b.readMeta(name); ok {
		t.Errorf("a closed bead's crew-marked session is nobody's at pass start either:\n%s", dispatcherOut(d))
	}
}
