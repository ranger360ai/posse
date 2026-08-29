package rhq

// Repro for the cockpit's `d` (resume) key on an IN PROGRESS row whose holder
// is on the pre-Dial-F *slot* session (ADR 0004 §2-3, bead rangerhq-ehu),
// found verifying it under rangerhq-hkz. Filed as rangerhq-lwx and fixed
// there (LaunchBead walks the join's two names); the skip is gone and this
// is now a live regression test, with the rest of the fix pinned below.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlotHeldBeadIsNotDoublePrompted(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()

	// The holder is the persona's SLOT session — the pre-Dial-F half of the
	// cockpit's holder join — and it is actively working.
	slot := SessionFor("ranger", repo)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"working","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)
	mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger"})
	ws := fakeLoadWSFrom(t, fake)
	for i := range ws {
		ws[i].AgentStatus = "working"
	}
	saveWSTo(t, fake, ws)

	// The bead the slot session is working: in_progress, assigned to ranger.
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "held on the slot session",
		Labels: []string{"go"}, Status: "in_progress", Assignee: "ranger"}, Dir: repo}

	// This is what the cockpit's IN PROGRESS row displays as the holder:
	// holderSession tries SessionForBead first, then falls back to the slot.
	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	var holder *HerdrSession
	for i := range sessions {
		if sessions[i].Name == slot {
			holder = &sessions[i]
		}
	}
	if holder == nil {
		t.Fatalf("slot session %s not listed: %v", slot, sessions)
	}
	if holder.Status != "working" {
		t.Fatalf("holder status %q, want working", holder.Status)
	}

	// `d` on that row is LaunchBead with Resume. ADR 0004 §3: it re-prompts
	// THE HOLDER. The holder is working, so it must refuse — the same
	// invariant TestLaunchBead pins for a Dial F holder.
	d.Resume = true
	session, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "working") {
		t.Errorf("working holder on the slot session was not refused: session=%q err=%v", session, err)
		if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
			t.Errorf("a SECOND session was created for a bead the slot session is working:\n%s", log)
		}
		if log := calls(t, fake); strings.Contains(log, "agent prompt w2:p1") {
			t.Errorf("the second session was prompted with the same bead:\n%s", log)
		}
	}
}

// ─── the fix (rangerhq-lwx) ──────────────────────────────────────────────────

// slotHolder sets up a persona whose pre-Dial-F slot session is alive in the
// repo with the given herdr status, holding bead a-1. Returns (slot, bead).
func slotHolder(t *testing.T, b *HerdrBackend, fake, repo, status string, crew bool) (string, RepoIssue) {
	t.Helper()
	slot := SessionFor("ranger", repo)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"`+status+`","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)
	mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger", Crew: crew})
	ws := fakeLoadWSFrom(t, fake)
	for i := range ws {
		ws[i].AgentStatus = status
	}
	saveWSTo(t, fake, ws)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"held on the slot session","status":"in_progress","assignee":"ranger"}]`), 0o644)
	return slot, RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "held on the slot session",
		Labels: []string{"go"}, Status: "in_progress", Assignee: "ranger"}, Dir: repo}
}

// The other half of the same invariant: when the slot holder has settled,
// `d` re-prompts THAT session (ADR 0004 §3, "re-prompt the holder") — it
// does not open a per-bead session alongside it.
func TestSlotHeldBeadResumesInTheHolderSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	slot, is := slotHolder(t, b, fake, repo, "idle", false)

	d.Resume = true
	session, err := d.LaunchBead(is)
	if err != nil {
		t.Fatalf("resume into the idle holder failed: %v", err)
	}
	if session != slot {
		t.Errorf("resumed session = %q, want the holder %q", session, slot)
	}
	log := calls(t, fake)
	if strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
		t.Errorf("a second session was created beside the holder:\n%s", log)
	}
	if !strings.Contains(log, "agent prompt w1:p1") {
		t.Errorf("the holder session was not prompted:\n%s", log)
	}
}

// ADR 0008: the slot half of the join reaches the operator's own persona
// slot sessions (business-manager-posse, qa-posse, …). Now that `d`
// acts on the holder, a crew holder must be refused rather than prompted —
// the same line a pass prints, and --resume does not override it.
func TestCrewSlotHolderIsNotResumed(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	slot, is := slotHolder(t, b, fake, repo, "idle", true)

	d.Resume = true
	session, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "crew session "+slot) {
		t.Errorf("crew holder was not refused: session=%q err=%v", session, err)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("the operator's session was prompted:\n%s", log)
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("bead claimed behind the operator's back:\n%s", bdCalls(t, fake))
	}
}

// The smaller half of rangerhq-lwx: Route falls through to label match when
// the assignee is not a loadable persona, which would launch a stranger onto
// a bead someone else holds. `d` acts on the holder the row named or not at all.
func TestHeldByNonPersonaIsNotDispatched(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"},
		Status: "in_progress", Assignee: "a-human"}, Dir: repo}
	d.Resume = true
	session, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "held by a-human") {
		t.Errorf("bead held by a non-persona was dispatched: session=%q err=%v", session, err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a session was created for someone else's bead:\n%s", log)
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("someone else's bead was claimed:\n%s", bdCalls(t, fake))
	}
}

// The boundary the slot fallback must not cross: an unheld bead is Dial F
// work and gets its own per-bead session even when the persona's slot
// session is alive and idle.
func TestReadyBeadStillGetsItsOwnSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)
	mustCreate(t, b, NewSessionOpts{Name: SessionFor("ranger", repo), Dir: repo, Agent: "ranger"})

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	session, err := d.LaunchBead(is)
	if err != nil {
		t.Fatal(err)
	}
	if want := SessionForBead("ranger", repo, "a-1"); session != want {
		t.Errorf("session = %q, want the bead's own %q", session, want)
	}
	if log := calls(t, fake); !strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
		t.Errorf("ready bead did not get its own session:\n%s", log)
	}
}

// ─── rangerhq-9vy (verify of rangerhq-lwx) ───────────────────────────────────
// Hardening of the join/launch agreement the fix claimed, plus two escapes
// that survived: Run --resume still twins an idle slot holder (rangerhq-v330)
// and crewHeld on the unused slot masks a Dial F holder (rangerhq-2um2).

func setWSStatus(t *testing.T, fake, label, status string) {
	t.Helper()
	ws := fakeLoadWSFrom(t, fake)
	for i := range ws {
		if ws[i].Label == label {
			ws[i].AgentStatus = status
		}
	}
	saveWSTo(t, fake, ws)
}

func bothHolders(t *testing.T, b *HerdrBackend, fake, repo, dialStatus, slotStatus string, slotCrew bool) (slot, dial string, is RepoIssue) {
	t.Helper()
	slot = SessionFor("ranger", repo)
	dial = SessionForBead("ranger", repo, "a-1")
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"`+slotStatus+`","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"`+dialStatus+`","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)
	mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger", Crew: slotCrew})
	mustCreate(t, b, NewSessionOpts{Name: dial, Dir: repo, Agent: "ranger"})
	setWSStatus(t, fake, slot, slotStatus)
	setWSStatus(t, fake, dial, dialStatus)
	os.WriteFile(filepath.Join(repo, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"held","status":"in_progress","assignee":"ranger"}]`), 0o644)
	is = RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "held", Labels: []string{"go"},
		Status: "in_progress", Assignee: "ranger"}, Dir: repo}
	return slot, dial, is
}

// The working/blocked guard is one condition. rangerhq-lwx pinned working;
// a blocked slot holder is the other half of the same refusal.
func TestSlotHeldBeadBlockedIsNotDoublePrompted(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	slot, is := slotHolder(t, b, fake, repo, "blocked", false)

	d.Resume = true
	session, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Errorf("blocked holder on the slot session was not refused: session=%q err=%v", session, err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
		t.Errorf("a SECOND session was created for a bead the slot session is blocked on:\n%s", log)
	}
	if session != "" && session != slot {
		t.Errorf("refusal named %q, want the slot %q", session, slot)
	}
}

// Cockpit display prefers Dial F when both names are live (TestQAHolderJoinPrecision).
// `d` must act on that same session, not the slot sitting beside it.
func TestDialFHolderWinsWhenBothExist(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	_, dial, is := bothHolders(t, b, fake, repo, "idle", "working", false)

	d.Resume = true
	session, err := d.LaunchBead(is)
	if err != nil {
		t.Fatalf("idle Dial F holder refused because the unused slot was working: session=%q err=%v", session, err)
	}
	if session != dial {
		t.Errorf("resumed %q, want the displayed Dial F holder %q", session, dial)
	}
	log := calls(t, fake)
	if strings.Contains(log, "agent prompt w1:p1") {
		t.Errorf("the slot (working, not displayed) was prompted:\n%s", log)
	}
	if !strings.Contains(log, "agent prompt w2:p1") {
		t.Errorf("the Dial F holder was not prompted:\n%s", log)
	}
}

// Same join order: a working Dial F is refused and no third session is born.
func TestDialFWorkingHolderIsRefusedWhenSlotAlsoExists(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	_, _, is := bothHolders(t, b, fake, repo, "working", "idle", false)

	d.Resume = true
	session, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "working") {
		t.Errorf("working Dial F holder was not refused: session=%q err=%v", session, err)
	}
	if log := calls(t, fake); strings.Count(log, "workspace create") > 2 {
		t.Errorf("a third session was created beside Dial F and the slot:\n%s", log)
	}
}

// Display shows Dial F (not crew). A crew mark on the unused slot fallback
// must not retarget `d`. crewHeld on the whole name list, before picking the
// live holder, is the shape that did (rangerhq-2um2); the guards now ask
// about the join's holder and the names ahead of it (namesThrough).
func TestCrewSlotDoesNotMaskDialFHolder(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()
	slot, dial, is := bothHolders(t, b, fake, repo, "idle", "idle", true)

	d.Resume = true
	session, err := d.LaunchBead(is)
	if err != nil {
		t.Errorf("Dial F holder masked by a crew slot: session=%q err=%v", session, err)
		if strings.Contains(err.Error(), "crew session "+slot) {
			t.Errorf("refused the displayed holder because the unused slot %s is crew", slot)
		}
		return
	}
	if session != dial {
		t.Errorf("resumed %q, want Dial F %q", session, dial)
	}
	if log := calls(t, fake); !strings.Contains(log, "agent prompt w2:p1") {
		t.Errorf("the Dial F holder was not prompted:\n%s", log)
	}
}

// The pass's copy of the same false refusal. rangerhq-2um2 names both paths:
// fireLoop asked crewHeld about its whole `crewNames` list too, so a crew
// mark on the unused slot skipped a bead whose live Dial F session was the
// holder — the shield freezing the fleet on a session that is not the
// operator's.
func TestDispatchResumeCrewSlotDoesNotMaskDialFHolder(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.Resume = true
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	slot := SessionFor("ranger", repo)
	dial := SessionForBead("ranger", repo, "a-1")
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"idle","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644)
	mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger", Crew: true})
	mustCreate(t, b, NewSessionOpts{Name: dial, Dir: repo, Agent: "ranger"})
	setWSStatus(t, fake, slot, "idle")
	setWSStatus(t, fake, dial, "idle")
	agentPerLaunch(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out, log := dispatcherOut(d), calls(t, fake)
	if strings.Contains(out, "held by crew session") {
		t.Errorf("the pass refused the Dial F holder because the unused slot %s is crew:\n%s", slot, out)
	}
	if n != 1 {
		t.Errorf("want 1 resumed into the Dial F holder, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(log, "agent prompt w2:p1") {
		t.Errorf("the Dial F holder %s was not re-prompted:\n%s\n%s", dial, out, log)
	}
	if strings.Contains(log, "agent prompt w1:p1") {
		t.Errorf("the operator's crew slot was prompted:\n%s", log)
	}
}

// The other arm of the same truncation, and the reason it keeps the names
// AHEAD of the holder: the pass's join (heldSession) only adopts a name that
// has an agent, so the operator's crew session with its agent gone is not
// the holder — and a guard narrowed to the holder alone would stop seeing
// it. ADR 0008 fails closed: a crew mark is the operator's whether or not
// herdr can see an agent in the pane (ranger-base-adb7's shape, one agent
// further out).
func TestDispatchSkipsAgentlessCrewHolderUnderResume(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.Resume = true
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	// The operator's own session, holding the bead by the run record, with
	// no agent herdr can see: `posse new ranger-staffing` and the agent quit.
	crew := "ranger-staffing"
	mustCreate(t, b, NewSessionOpts{Name: crew, Dir: repo, Agent: "ranger", Crew: true, Bead: "a-1"})
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(`[]`), 0o644)
	agentPerLaunch(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out, log := dispatcherOut(d), calls(t, fake)
	want := "held by crew session " + crew + " (operator's) — skipped"
	if n != 0 || !strings.Contains(out, want) {
		t.Errorf("want %q and nothing dispatched, got n=%d:\n%s", want, n, out)
	}
	if strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
		t.Errorf("a fleet twin was born beside the operator's agentless session:\n%s", log)
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("bead claimed behind the operator's back:\n%s", bdCalls(t, fake))
	}
}

// Run --resume is the semantics ADR 0004 §3 says cockpit `d` has. LaunchBead
// was fixed under rangerhq-lwx; fireLoop still always launched
// SessionForBead, so an idle slot holder got a twin (rangerhq-v330). Live
// regression test now — the pass fires into the holder the join names.
func TestDispatchResumeSlotHeldIdleDoesNotCreateTwin(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.Resume = true
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`)
	slot := SessionFor("ranger", repo)
	mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger"})
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	log := calls(t, fake)
	dial := SessionForBead("ranger", repo, "a-1")
	if strings.Contains(log, "workspace create --label "+dial) {
		t.Errorf("--resume created a Dial F twin beside the idle slot holder (n=%d):\n%s\n%s", n, out, log)
	}
	if n != 1 {
		t.Errorf("want 1 resumed into the slot, got n=%d:\n%s", n, out)
	}
	// The other half of the fix: not-twinning is not enough — the pass has
	// to have re-prompted THE HOLDER. w1 is the slot's pane; a skip would
	// satisfy the twin check alone.
	if !strings.Contains(log, "agent prompt w1:p1") {
		t.Errorf("the idle slot holder was not re-prompted (n=%d):\n%s\n%s", n, out, log)
	}
	if strings.Contains(out, "creating session "+dial) {
		t.Errorf("the pass announced a Dial F session for a slot-held bead:\n%s", out)
	}
	if !strings.Contains(out, "→ "+slot) {
		t.Errorf("the pass did not report prompting the slot %s:\n%s", slot, out)
	}
}

// The residual half of the same disagreement, found fixing rangerhq-v330 and
// filed as ranger-base-6bu: the slot session is alive but its AGENT is gone.
// LaunchBead picks the holder by Resolve success alone, so the bare slot is
// the holder and the agent is relaunched in place. fireLoop's walk also
// required a status, because it fed the rangerhq-zom stopped-on-purpose skip
// — which must not fire on an agentless session — so --resume found no
// holder and the Dial F name stood, and the pass built a second session
// beside the live slot.
//
// Fixed by splitting the two conditions, which is what the bead was filed
// to decide: heldSession returns the holder AND its status, the skip turns
// on the STATUS (a session nobody is in did not stop on purpose), the
// retarget on the HOLDER. Both legs run because the split changes the
// NON-resume path too — rangerhq-zom's "agent gone → the launch
// creates/relaunches" never said which name, and until now the pass created
// a Dial F session while cockpit `d` relaunched the slot. One answer on both
// paths is what ADR 0004 §2's "the same two names" already claims; the leg
// below is what holds the pass to it.
func TestDispatchResumeSlotAgentGoneDoesNotCreateTwin(t *testing.T) {
	for _, leg := range []struct {
		name   string
		resume bool
	}{
		{"--resume", true},
		{"normal pass", false},
	} {
		t.Run(leg.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.Resume = leg.resume
			writePersona(t, b.App, "ranger", "[go]")
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
				`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`)
			slot := SessionFor("ranger", repo)
			mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger"})
			// No agent anywhere: the session is a bare shell.
			os.WriteFile(filepath.Join(fake, "agents.json"), []byte(`[]`), 0o644)
			// Old enough for RelaunchAgent to act. Inside RelaunchGrace a
			// crashed CLI cannot be told from one still starting, so the
			// relaunch refuses for that reason alone — and the positive half
			// below would be measuring the grace, not the join.
			ageLaunch(t, b, slot, d.RelaunchGrace+time.Minute)
			agentPerLaunch(t, fake)

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out, log := dispatcherOut(d), calls(t, fake)
			dial := SessionForBead("ranger", repo, "a-1")
			if strings.Contains(log, "workspace create --label "+dial) {
				t.Errorf("a Dial F twin was created beside the agentless slot holder %s (n=%d):\n%s\n%s", slot, n, out, log)
			}
			if strings.Contains(out, "creating session "+dial) {
				t.Errorf("the pass announced a Dial F session for a slot-held bead:\n%s", out)
			}
			// Not twinning is not enough — every assertion above is also
			// satisfied by a skip. The pass has to have relaunched the
			// persona IN the holder and prompted it there.
			if n != 1 {
				t.Errorf("want 1 dispatched into the agentless slot holder, got n=%d:\n%s", n, out)
			}
			if !strings.Contains(out, "relaunching ranger in "+slot) {
				t.Errorf("the agent was not relaunched in the holder %s:\n%s", slot, out)
			}
			if strings.Count(log, "workspace create") != 1 {
				t.Errorf("the relaunch must reuse the holder's workspace, not create one:\n%s", log)
			}
			if !strings.Contains(log, "agent prompt w1:p1") {
				t.Errorf("the holder was not prompted:\n%s\n%s", out, log)
			}
		})
	}
}

// The dry pass's copy of the same answer. `--dry-run` is the operator's
// preview and the fire loop is written so a dry pass "says the same thing a
// real one would do"; the retarget sits ABOVE that branch, so the preview
// names the holder. Verifying rangerhq-v330 this was the one measured
// behaviour with nothing pinning it: before the fix the dry line read
// `in session ranger-<repo>-a-1` — a session the real pass would not use —
// and it did so silently, because a dry pass creates nothing for the
// no-twin assertion above to catch (ranger-base-vw6).
func TestDispatchResumeDryRunNamesTheHolderNotATwin(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.Resume = true
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	slot := SessionFor("ranger", repo)
	mustCreate(t, b, NewSessionOpts{Name: slot, Dir: repo, Agent: "ranger"})
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 {
		t.Errorf("want 1 would-be dispatch into the slot, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "in session "+slot+" ") {
		t.Errorf("the dry pass did not name the holder %s:\n%s", slot, out)
	}
	if dial := SessionForBead("ranger", repo, "a-1"); strings.Contains(out, "in session "+dial) {
		t.Errorf("the dry pass previewed a Dial F twin beside the idle holder:\n%s", out)
	}
}

// The record's arm of the holder join, with no name to fall back on. Both
// name patterns are guesses that a session which exists would be CALLED
// this; `bead:` is what the launcher wrote. A holder living under neither
// name — `posse new <anything>` plus a work prompt, which NoteBeadFromPrompt
// stamps — is found only by the record, and deleting that arm leaves the
// whole package green (measured, ranger-base-adb7): the pass then treats the
// bead as unheld and builds a twin beside a live holder, which is the
// rangerhq-v330 class one naming scheme further out.
func TestRunRecordHolderIsJoinedUnderAnyName(t *testing.T) {
	for _, leg := range []struct {
		name   string
		resume bool
		want   string
	}{
		{"normal pass", false, "held by ranger, ranger-staffing idle"},
		{"--resume", true, "→ ranger-staffing"},
	} {
		t.Run(leg.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			writePersona(t, b.App, "ranger", "[go]")
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"],"assignee":"ranger","status":"in_progress"}]`,
				// Still open at the gather: a bead that reads closed would have
				// the end-of-pass reaper kill the holder before it is measured.
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
			// Fleet, not crew: this is the join, not ADR 0008's shield.
			holder := "ranger-staffing"
			mustCreate(t, b, NewSessionOpts{Name: holder, Dir: repo, Agent: "ranger", Bead: "a-1"})
			idleClaude(t, fake)
			agentPerLaunch(t, fake)

			d := newTestDispatcher(t, b)
			d.Resume = leg.resume
			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if !strings.Contains(out, leg.want) {
				t.Errorf("want %q, got:\n%s", leg.want, out)
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
				t.Errorf("a twin was created beside the record's holder:\n%s", log)
			}
		})
	}
}
