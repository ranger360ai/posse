package posse

// QA pins for ranger-base-htafy: an idle agent with live background work,
// or with a prompt still sitting in its composer, is WAITING — not settled.
//
// The unit readings live in panework_test.go. These pin the three places
// that acted on the wrong answer, each against a control arm that still
// behaves as it did before the fix — without one, "no settle-open was
// recorded" is also what a pass that never reached the rung prints
// (ranger-base-71ki).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// armScreen makes every `agent explain` in a test carry the two screen
// regions herdr previews while evaluating its rules — claude's footer and
// its prompt box (panework.go). Both strings are the shapes measured on the
// live shop on 2026-09-02.
func armScreen(t *testing.T, fake, footer, box string) {
	t.Helper()
	rules, err := json.Marshal([]EvaluatedRule{
		screenRule("live_blocked_form", footerRegion, "blocked", footer),
		screenRule("live_prompt_box", composerRegion, "idle", box),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "explain-rules"), rules, 0o644); err != nil {
		t.Fatal(err)
	}
}

func screenRule(id, region, state, preview string) EvaluatedRule {
	r := EvaluatedRule{ID: id, Region: region, State: state}
	r.Evidence.RegionBytes = len(preview)
	r.Evidence.RegionPreview = preview
	return r
}

// idleFooter is what claude draws with nothing running behind it, and
// busyFooter is the bead's own state, captured verbatim off a live pane at
// 14:38:10 that herdr's listing called idle and its detection matched
// live_prompt_box in — settled by every reading posse had before this bead,
// with a background shell still going. `esc to interrupt` is gone because
// the turn ended; the task summary is not, because it is drawn from the
// task list alone.
const (
	idleFooter = "  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents\n"
	busyFooter = "  ⏵⏵ auto mode on · 1 shell · ← for agents · ↓ to manage\n"
	emptyBox   = "❯\n"
	unsentBox  = "❯ commit and close the bead once the suite is green\n"
)

// settleWaitingPass runs one resuming pass over a bead that settles without
// closing — TestDispatchPassCountsASettleOpen's shape — with the screen the
// caller arms. It hands back the pass output and the repo, so a caller can
// ask both what was printed and what was written to bd.
func settleWaitingPass(t *testing.T, footer, box string) (string, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := settleRepo(t)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	armScreen(t, fake, footer, box)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	return dispatcherOut(d), repo
}

// The bead. A pass whose agent settled while its own suite run is still
// going must not judge the bead, must not count a settle-open, and must say
// what it is waiting on.
func TestQASettleBehindLiveBackgroundWorkIsNotASettleOpen(t *testing.T) {
	out, repo := settleWaitingPass(t, busyFooter, emptyBox)
	if !strings.Contains(out, "1 shell still running") {
		t.Fatalf("the pass did not name the work it is waiting on:\n%s", out)
	}
	if !strings.Contains(out, "waiting, not judged this pass") {
		t.Errorf("the pass judged a settle behind live background work:\n%s", out)
	}
	if strings.Contains(out, "but issue is") {
		t.Errorf("the pass reported a settle-open for an agent that is waiting:\n%s", out)
	}
	if cs := readComments(t, repo); len(cs) != 0 {
		t.Fatalf("the pass counted %d settle-open(s) against a waiting agent: %v", len(cs), cs)
	}
}

// The other half, which the third occurrence proved is not specific to a
// monitor: the re-prompt was typed and never submitted, so the agent has
// nothing to settle FROM.
func TestQASettleWithAnUnsentPromptInTheBoxIsNotASettleOpen(t *testing.T) {
	out, repo := settleWaitingPass(t, idleFooter, unsentBox)
	if !strings.Contains(out, "UNSENT") || !strings.Contains(out, "commit and close the bead") {
		t.Fatalf("the pass did not show the operator the prompt that never landed:\n%s", out)
	}
	if cs := readComments(t, repo); len(cs) != 0 {
		t.Fatalf("the pass counted %d settle-open(s) against a prompt that never landed: %v", len(cs), cs)
	}
}

// THE CONTROL, and the reason the two above are measurements rather than
// coincidences: the same pass over the same bead with an idle footer and an
// empty box still counts the settle-open it always counted (ranger-base-9hm).
func TestQASettleWithAnEmptyScreenStillCounts(t *testing.T) {
	out, repo := settleWaitingPass(t, idleFooter, emptyBox)
	if !strings.Contains(out, `settled "idle" but issue is "in_progress"`) {
		t.Fatalf("the control arm did not reach a settle-without-close:\n%s", out)
	}
	if cs := readComments(t, repo); len(cs) != 1 {
		t.Fatalf("the control arm recorded %d settle-opens, want 1: %v", len(cs), cs)
	}
}

// A herdr that reports no screen regions at all — an older one, or a
// manifest that no longer reads them — leaves the rung exactly as it was.
// Ignorance must never be read as a wait, or a genuinely stuck bead would
// be held claimed forever.
func TestQAAScreenPosseCannotReadStillCounts(t *testing.T) {
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := settleRepo(t)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	// No explain-rules armed: the key is absent, which is what an older
	// herdr emits.
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if cs := readComments(t, repo); len(cs) != 1 {
		t.Fatalf("a herdr that cannot show a screen changed the verdict: %d settle-opens: %v", len(cs), cs)
	}
}

// ─── the --resume skip ───────────────────────────────────────────────────────

// --resume overrides a persona that STOPPED. The second pass over the same
// bead finds a live holder herdr calls idle, and must not type a second
// prompt into a session that is waiting on its own work.
func TestQAResumeDoesNotRePromptAHolderThatIsWaiting(t *testing.T) {
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := settleRepo(t)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)
	armScreen(t, fake, busyFooter, emptyBox)

	for i := 0; i < 2; i++ {
		if _, err := d.Run("", "", 0); err != nil {
			t.Fatalf("pass %d: %v", i+1, err)
		}
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "waiting, not re-prompted") {
		t.Fatalf("a resuming pass re-prompted a holder that is waiting on its own work:\n%s", out)
	}
}

// ─── the pulse's G2 row ──────────────────────────────────────────────────────

// The condition the coordinator was told about twice on 2026-09-02 for two
// sessions that were working as designed.
func TestQAGovG2DropsAHolderWaitingOnItsOwnWork(t *testing.T) {
	b, fake := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	sessionAgent(t, fake, "developer-x", "idle")
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})
	armScreen(t, fake, busyFooter, emptyBox)

	if g := find(shopSet(t, govIn(t, b)), "G2"); g != nil {
		t.Errorf("an agent waiting on its own suite run was reported settled-but-holding: %+v", *g)
	}

	// The control: the same session with nothing running behind it is the
	// row this surface has always raised (TestGovG2SettledHolder).
	armScreen(t, fake, idleFooter, emptyBox)
	if g := find(shopSet(t, govIn(t, b)), "G2"); g == nil || g.Key != "settled:bd-1" {
		t.Errorf("the settled-but-holding row stopped firing for a holder that really did stop: %+v", g)
	}
}

// Unsent text does NOT drop the row: nobody has actually spoken to that
// session, which is the one thing here a human has to fix. It keeps the row
// and says so.
func TestQAGovG2KeepsAHolderWithAnUnsentPrompt(t *testing.T) {
	b, fake := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	sessionAgent(t, fake, "developer-x", "idle")
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})
	armScreen(t, fake, idleFooter, unsentBox)

	g := find(shopSet(t, govIn(t, b)), "G2")
	if g == nil {
		t.Fatal("a session whose prompt never landed dropped off the surface entirely")
	}
	if g.Key != "settled-unsent:bd-1" {
		t.Errorf("G2 key = %q, want settled-unsent:bd-1", g.Key)
	}
	if !strings.Contains(g.Detail, "UNSENT") {
		t.Errorf("the row does not tell the operator what to fix: %q", g.Detail)
	}
}

// ─── the read-back ───────────────────────────────────────────────────────────

// herdr answers agent_prompted for a prompt it typed and did not submit, so
// the return value is not evidence the turn started. ConfirmSubmitted is
// what `posse prompt` asks afterwards.
func TestQAConfirmSubmittedReportsAPromptThatNeverLeftTheBox(t *testing.T) {
	b, fake := newTestBackend(t)
	armScreen(t, fake, idleFooter, unsentBox)
	if left := b.ConfirmSubmitted("w1:p1"); !strings.Contains(left, "commit and close the bead") {
		t.Errorf("a prompt still sitting in the box read as submitted: %q", left)
	}
	armScreen(t, fake, idleFooter, emptyBox)
	if left := b.ConfirmSubmitted("w1:p1"); left != "" {
		t.Errorf("an empty box read as unsubmitted: %q", left)
	}
}

// ─── the pulse's own prompt ──────────────────────────────────────────────────

// A shop check typed after somebody's unsent prompt makes one garbled
// message out of two — the third strand of the incident, from the detection
// the pulse's readiness gate has already paid for. The tick comes round
// again; the stale text is the operator's to clear.
func TestQAPulseDoesNotTypeAfterAnUnsentPrompt(t *testing.T) {
	b, fake := newTestBackend(t)
	personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pulseFastRuntime(t, b, "coordinator-work", "400ms")
	unpushedRepo(t, b)
	armScreen(t, fake, idleFooter, unsentBox)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute, RenagMax: 4 * time.Hour}
	d.pulseOnce(cfg)

	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a shop check was typed on top of an unsent prompt:\n%s", log)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "UNSENT") {
		t.Errorf("the skip does not say what is in the box:\n%s", out)
	}

	// The control: with an empty box the same tick prompts, as it always has.
	b2, fake2 := newTestBackend(t)
	personaSession(t, b2, fake2, "coordinator-work", "coordinator", "idle", false)
	pulseFastRuntime(t, b2, "coordinator-work", "400ms")
	unpushedRepo(t, b2)
	armScreen(t, fake2, idleFooter, emptyBox)
	clock2 := time.Now()
	d2 := deliveryDispatcher(t, b2, &clock2)
	d2.pulseOnce(cfg)
	if log := calls(t, fake2); !strings.Contains(log, "agent prompt") {
		t.Errorf("the control tick delivered no shop check at all:\n%s", log)
	}
}
