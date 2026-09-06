package posse

// Delivery half of the pulse (ADR 0027 §3-4, rangerhq-44w1): prompt on a
// new fingerprint, suppress inside renag, repeat at ONE fixed renag
// interval (the doubling ladder left with ranger-base-thm0j), idle-only
// skip, no crew mark written. Builds on rangerhq-4ish's ShopCheck and fixtures
// (pulse_test.go); pulseOnce is called directly with a controlled clock
// rather than through the ticker, so renag timing is deterministic.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// personaSession creates a live posse session for persona with the given
// herdr agent status and crew mark — the sibling to pulse_test.go's
// blockedSession, for delivery's idle/working/crew fixtures. Returns the
// workspace id, so a later status change can go through setAgentStatus
// instead of recreating the (already-taken) session name.
func personaSession(t *testing.T, b *HerdrBackend, fake, name, persona, status string, crew bool) string {
	t.Helper()
	writePersona(t, b.App, persona, "code")
	mustCreate(t, b, NewSessionOpts{Name: name, Agent: persona, Crew: crew})
	ws := fakeLoadWSFrom(t, fake)
	var id string
	for _, w := range ws {
		if w.Label == name {
			id = w.WorkspaceID
		}
	}
	if id == "" {
		t.Fatalf("no workspace created for session %q: %+v", name, ws)
	}
	setAgentStatus(t, fake, id, status)
	return id
}

// setAgentStatus rewrites the fake herdr's agent listing for one workspace
// to a new status, without touching the session/workspace itself — the way
// a live agent's status actually changes underneath a session that already
// exists.
func setAgentStatus(t *testing.T, fake, id, status string) {
	t.Helper()
	agents := fmt.Sprintf(`[{"agent":"claude","agent_status":%q,"pane_id":%q,"workspace_id":%q}]`, status, id+":p1", id)
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(agents), 0o644)
}

// agentState is one row of the fake herdr's agent listing, for the fixtures
// that need more than one live agent at a time.
type agentState struct{ id, status string }

// setAgentStatuses is setAgentStatus for SEVERAL workspaces: the listing is
// one file, so a fixture with two live sessions must write both rows at
// once or the one it leaves out reads as a session with no agent.
func setAgentStatuses(t *testing.T, fake string, states ...agentState) {
	t.Helper()
	rows := make([]string, 0, len(states))
	for _, s := range states {
		rows = append(rows, fmt.Sprintf(`{"agent":"claude","agent_status":%q,"pane_id":%q,"workspace_id":%q}`,
			s.status, s.id+":p1", s.id))
	}
	agents := "[" + strings.Join(rows, ",") + "]"
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(agents), 0o644)
}

// unpushedRepo is TestShopCheckUnpushedCommits's fixture, factored out and
// wired into config's beads: (pulseOnce reads condition (b) through
// d.App.BeadsDirs(), unlike ShopCheck's own tests which pass the dir
// straight in) — a condition that has nothing to do with session status, so
// delivery tests can hold the target session's status fixed and still get a
// non-empty condition set.
func unpushedRepo(t *testing.T, b *HerdrBackend) string {
	t.Helper()
	repo := wtRepo(t)
	bare := t.TempDir()
	mustGit(t, bare, "init", "-q", "--bare")
	mustGit(t, repo, "remote", "add", "origin", bare)
	mustGit(t, repo, "push", "-q", "-u", "origin", "main")
	commitIn(t, repo, "extra.txt", "x", "extra")
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)
	return repo
}

func deliveryDispatcher(t *testing.T, b *HerdrBackend, clock *time.Time) *Dispatcher {
	t.Helper()
	d := newTestDispatcher(t, b)
	d.Now = func() time.Time { return *clock }
	return d
}

func TestPulsePromptsOnNewFingerprint(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	out := dispatcherOut(d)
	if !strings.Contains(out, "→ prompted coordinator-work") {
		t.Errorf("no prompt logged:\n%s", out)
	}
	log := calls(t, fake)
	// The pane, not the session label — herdr's real AgentPrompt addresses
	// panes (ranger-base-5qe6); "agent prompt coordinator-work" is the bug
	// this fixes, and would 404 against real herdr with agent_not_found.
	if !strings.Contains(log, "agent prompt "+pane+" Pulse check:") {
		t.Errorf("calls.log missing the pulse prompt:\n%s", log)
	}
	if n := strings.Count(log, "agent prompt "+pane); n != 1 {
		t.Errorf("want exactly one prompt, got %d:\n%s", n, log)
	}

	state := ReadPulseState(PulsePath(b.App))
	if state.PromptedFingerprint == "" || state.PromptedAt.IsZero() {
		t.Errorf("bookkeeping not recorded after a successful prompt: %+v", state)
	}
}

func TestPulseSuppressedOnUnchangedInsideRenag(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg) // prompts once, sets the renag clock
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Fatalf("setup: want exactly one prompt before the suppression window, got %d", n)
	}

	clock = clock.Add(10 * time.Minute) // inside the 30m renag window
	d.pulseOnce(cfg)

	log := calls(t, fake)
	if n := strings.Count(log, "agent prompt "+pane); n != 1 {
		t.Errorf("renag window must suppress the re-prompt, got %d prompts:\n%s", n, log)
	}
}

// The repeat is ONE interval, over and over, and this is the test that used
// to be TestPulseRenagDoublesUpToMax. It is written as the arm that
// separates the two designs rather than as a restatement of the new one:
// three ticks 30m apart must yield three prompts. Under the ladder the
// third would have been suppressed: the second prompt doubled the stored
// interval to 60m, and the third tick is only 30m after it. Restoring the
// doubling reds this test and, measured, nothing else in the package.
//
// 117.3h of the operator's dispatch-watch.log say nothing real ever got
// this far: 386 delivery episodes, 11 of them repeated once, none twice
// (ranger-base-thm0j). The ladder's second rung was written to disk and
// never read.
func TestPulseRenagRepeatsAtOneFixedInterval(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg) // 1st: due immediately — a new fingerprint

	clock = clock.Add(30 * time.Minute)
	d.pulseOnce(cfg) // 2nd: one renag interval since the last delivery
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
		t.Fatalf("want 2 prompts after the first renag, got %d", n)
	}

	clock = clock.Add(30 * time.Minute)
	d.pulseOnce(cfg) // 3rd: the SAME interval again, not a doubled one
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 3 {
		t.Fatalf("the repeat interval must not grow: want 3 prompts at +30m each, got %d", n)
	}

	// And the clock runs from the last DELIVERY, not from the episode's
	// first: 29m after the third prompt is still inside the window.
	clock = clock.Add(29 * time.Minute)
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 3 {
		t.Errorf("29m after the third delivery is inside the window, got %d prompts", n)
	}
}

func TestPulseIdleOnlySkipsWorkingSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "working", false)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	out := dispatcherOut(d)
	if !strings.Contains(out, "pulse: skipped (working)") {
		t.Errorf("want 'pulse: skipped (working)', got:\n%s", out)
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("a working session must never be prompted:\n%s", calls(t, fake))
	}

	// Untouched bookkeeping: the very next tick (no renag wait) retries.
	state := ReadPulseState(PulsePath(b.App))
	if state.PromptedFingerprint != "" || !state.PromptedAt.IsZero() {
		t.Errorf("a skipped prompt must not advance delivery bookkeeping: %+v", state)
	}

	// The session goes idle; the same tick's fingerprint is retried right
	// away, not held behind a renag wait it never actually served.
	setAgentStatus(t, fake, id, "idle")
	d.pulseOnce(cfg)
	if !strings.Contains(calls(t, fake), "agent prompt "+pane) {
		t.Errorf("must retry next tick once idle:\n%s", calls(t, fake))
	}
}

func TestPulseUndeliverableWithNoLiveSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	out := dispatcherOut(d)
	if !strings.Contains(out, "undeliverable (no live session for coordinator)") {
		t.Errorf("want an undeliverable line, got:\n%s", out)
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("no live session must never be prompted:\n%s", calls(t, fake))
	}
	if strings.Contains(calls(t, fake), "workspace create") {
		t.Errorf("an undeliverable pulse must never create a session:\n%s", calls(t, fake))
	}
}

// ADR 0008 §2's carve-out amended by ADR 0027: the pulse may reach a
// crew-marked session — the operator's own conversation — and must set no
// crew mark doing it, unlike every other prompt path (personaActive,
// crewHeld) which treats a crew session as if it did not exist.
func TestPulseTargetsCrewSessionAndWritesNoCrewMark(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", true)
	pane := id + ":p1"
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	if !strings.Contains(calls(t, fake), "agent prompt "+pane) {
		t.Errorf("a crew-marked session must still receive the pulse:\n%s", calls(t, fake))
	}
	s, err := b.Resolve("coordinator-work")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Crew {
		t.Errorf("the pulse must not clear an existing crew mark")
	}
	if strings.Contains(calls(t, fake), "session set-meta") || strings.Contains(calls(t, fake), "crew") {
		t.Errorf("the pulse must set no crew mark of its own (AgentPrompt only):\n%s", calls(t, fake))
	}
}

func TestPulseClearedSetResetsRenagClock(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	id := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	pane := id + ":p1"
	repo := unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg) // condition present, prompts once
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Fatalf("setup: want exactly one prompt, got %d", n)
	}

	mustGit(t, repo, "push", "-q") // clears the unpushed condition
	clock = clock.Add(time.Minute) // well inside the 30m renag window
	d.pulseOnce(cfg)
	state := ReadPulseState(PulsePath(b.App))
	if state.PromptedFingerprint != "" || !state.PromptedAt.IsZero() {
		t.Errorf("a cleared set must reset the renag clock: %+v", state)
	}

	commitIn(t, repo, "extra2.txt", "y", "extra2") // same condition shape recurs
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 2 {
		t.Errorf("a fresh episode after a clear must prompt again immediately, got %d prompts", n)
	}
}

// An armed pulse whose target is "" — no pulse_persona:, no coordinator:
// (ranger-base-q3gp) — delivers to nobody and says so. The fixture is the
// arm that makes it a pin rather than a tautology: a live IDLE session with
// no agent at all, which `s.Agent == persona` would have matched on the
// empty string, delivering a shop check into whatever session happened to
// be agentless.
func TestPulseWithNoPersonaDeliversToNobody(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "agentless"})
	ws := fakeLoadWSFrom(t, fake)
	for _, w := range ws {
		if w.Label == "agentless" {
			setAgentStatus(t, fake, w.WorkspaceID, "idle")
		}
	}
	unpushedRepo(t, b)

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "", Renag: 30 * time.Minute}

	d.pulseOnce(cfg)

	out := dispatcherOut(d)
	if !strings.Contains(out, "undeliverable (no pulse_persona: and no coordinator:)") {
		t.Errorf("want the no-target line, got:\n%s", out)
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("a pulse with no target must prompt nobody:\n%s", calls(t, fake))
	}
	// The conditions were still sensed and logged — a pulse with no target
	// is blind delivery, not blind sensing.
	if !strings.Contains(out, "unpushed") {
		t.Errorf("the tick must still sense and log its conditions:\n%s", out)
	}
}

// The same rule one level down, pinned where it lives: deliverPulse's guard
// is the one a caller trips, and this is the belt behind it — an empty
// persona matches no session, including the agentless one whose Agent field
// is also "" (mutate either guard away on its own and one of the two tests
// goes red).
func TestPulseTargetEmptyPersonaMatchesNothing(t *testing.T) {
	t.Parallel()
	sessions := []HerdrSession{
		{Name: "agentless", Agent: "", Status: "idle"},
		{Name: "qa-work", Agent: "qa", PaneID: "w1:p1", Status: "idle"},
	}
	if name, _, _, found := pulseTarget(sessions, ""); found {
		t.Errorf("empty persona matched %q; it must match nothing", name)
	}
	if name, pane, _, found := pulseTarget(sessions, "qa"); !found || name != "qa-work" || pane != "w1:p1" {
		t.Errorf("named persona must still match: %q %q %v", name, pane, found)
	}
}

// A condition set that GROWS inside the renag window is a NEW fingerprint,
// so it is due at once. This is deliverPulse's `changed` arm, and it was the
// one arm of that rule no test reached: every other new-fingerprint test
// starts from an EMPTY delivery record, where PromptedAt is zero and the
// IsZero arm answers first, so deleting `changed ||` left all 91 tests in
// this package that can reach pulseOnce green (ranger-base-r9bdn, found by
// mutation-sweeping ranger-base-bv2nq).
//
// Here a delivery has just happened, so the other two arms both say NOT due
// — the record has a timestamp, and it is one minute old against a 30m
// interval. Only `changed` can prompt. Without it the second condition
// waits up to one full renag interval, and the fingerprint on disk stays
// stale meanwhile, which is the suppression ADR 0027 §3 rules out by name.
func TestPulseGrownSetPromptsInsideRenagWindow(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	cid := personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
	did := personaSession(t, b, fake, "developer-work", "developer", "idle", false)
	pane := cid + ":p1"
	unpushedRepo(t, b)
	// personaSession writes the listing per session, so the second call
	// dropped the first's row: put both back, both idle.
	setAgentStatuses(t, fake, agentState{cid, "idle"}, agentState{did, "idle"})

	clock := time.Now()
	d := deliveryDispatcher(t, b, &clock)
	cfg := PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute}

	d.pulseOnce(cfg) // set A: the unpushed repo alone
	if n := strings.Count(calls(t, fake), "agent prompt "+pane); n != 1 {
		t.Fatalf("setup: want exactly one prompt for the first set, got %d", n)
	}
	first := ReadPulseState(PulsePath(b.App))
	if strings.Contains(first.PromptedFingerprint, "blocked:") {
		t.Fatalf("setup: the first set must not already carry a blocked condition: %q", first.PromptedFingerprint)
	}

	// The developer blocks one minute in — deep inside the 30m window.
	setAgentStatuses(t, fake, agentState{cid, "idle"}, agentState{did, "blocked"})
	clock = clock.Add(time.Minute)
	d.pulseOnce(cfg)

	log := calls(t, fake)
	if n := strings.Count(log, "agent prompt "+pane); n != 2 {
		t.Fatalf("a set that grew inside the renag window must prompt at once, got %d prompts:\n%s", n, log)
	}
	if !strings.Contains(log, "blocked:developer-work") {
		t.Errorf("the second prompt must name the condition that arrived:\n%s", log)
	}

	state := ReadPulseState(PulsePath(b.App))
	if !strings.Contains(state.PromptedFingerprint, "blocked:developer-work") {
		t.Errorf("the delivery record must carry the NEW fingerprint, got %q", state.PromptedFingerprint)
	}
	// The record is written to the second, so this is "moved to the second
	// tick", not an instant equality: what it pins is that the renag clock
	// restarted on this delivery rather than still running from the first.
	if !state.PromptedAt.After(first.PromptedAt) {
		t.Errorf("PromptedAt = %v, want it advanced past the first delivery (%v)", state.PromptedAt, first.PromptedAt)
	}
}
