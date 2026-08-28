package rhq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// foreignHolder plants a live herdr workspace posse has NO session meta for,
// labelled `name` and holding an idle claude — the post-wipe shape from
// rangerhq-ggm8: the meta was pruned (hand cleanup of state/herdr/, an older
// binary, a scratch-server op) while the workspace it named lived on.
// Resolve still finds it by label, and every field dispatch reads off a
// session — Crew, Agent, Runtime, Bead — is zero.
func foreignHolder(t *testing.T, fake, name string) {
	t.Helper()
	ws := append(fakeLoadWSFrom(t, fake), fakeWS{WorkspaceID: "wForeign", Label: name, AgentStatus: "idle"})
	saveWSTo(t, fake, ws)
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"idle","pane_id":"wForeign:p1","workspace_id":"wForeign"}]`), 0o644)
}

// rangerhq-ynx8: dispatch fails CLOSED on a session name a foreign workspace
// wears. The crew shield (ADR 0008) asks `s.Crew`, and a row with no meta
// cannot be crew — so a wipe that deletes the operator's crew mark used to
// hand their own conversation back to the fleet as a promptable holder. The
// bead is claimed and a work prompt tiered and caged for the routed persona
// is typed into whatever agent that pane holds: no gates, no cage, wrong
// persona. Every launch route says no instead, in every leg, and neither the
// claim nor the prompt happens.
func TestDispatchRefusesAForeignHolder(t *testing.T) {
	for _, leg := range []struct {
		name string
		set  func(*Dispatcher)
	}{
		{"--dry-run", func(d *Dispatcher) { d.DryRun = true }},
		{"normal", func(d *Dispatcher) {}},
		// --resume overrides a holder's idleness, never somebody else's
		// ownership: it is the route security reached the in_progress bead by.
		{"--resume", func(d *Dispatcher) { d.Resume = true }},
	} {
		t.Run(leg.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			writePersona(t, b.App, "ranger", "[go]")
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"closed","assignee":"ranger"}]`)
			session := SessionForBead("ranger", repo, "a-1")
			foreignHolder(t, fake, session)

			d := newTestDispatcher(t, b)
			leg.set(d)
			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			want := "held by a foreign workspace " + session + " (no session meta)"
			if n != 0 || !strings.Contains(out, want) {
				t.Errorf("want the foreign line and nothing dispatched, got n=%d:\n%s", n, out)
			}
			// The operator has to be told how to get the name back — the
			// refusal is permanent until they do, unlike a busy session.
			if !strings.Contains(out, "posse kill "+session) {
				t.Errorf("refusal does not say how to free the name:\n%s", out)
			}
			if log := calls(t, fake); strings.Contains(log, "agent prompt") {
				t.Errorf("a work prompt reached the foreign pane:\n%s", log)
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create") {
				t.Errorf("a twin was created beside the foreign row:\n%s", log)
			}
			if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
				t.Errorf("bead claimed onto a session that is not the fleet's:\n%s", log)
			}
		})
	}
}

// The cockpit's `d` is a launcher too (ADR 0011 §1) and makes the same
// refusal — it was the shortest route to the splice: an idle foreign row
// passed the crew, working/blocked and PromptGrace checks in one pass.
func TestLaunchBeadRefusesAForeignHolder(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	session := SessionForBead("ranger", repo, "a-1")
	foreignHolder(t, fake, session)

	_, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "held by a foreign workspace "+session) {
		t.Fatalf("want a foreign-hold refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "posse kill "+session) {
		t.Errorf("refusal does not say how to free the name: %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a work prompt reached the foreign pane:\n%s", log)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("bead claimed onto a foreign workspace:\n%s", log)
	}
}

// The backstop, asserted at the function every dispatch flavor funnels
// through: a route that reaches launchSession with a foreign name still
// refuses, and does NOT fall through to CreateSession. Reading "foreign" as
// "no session yet" would collide on the label rather than fix anything —
// and on a `prompt: argv` runtime it would put the work prompt on a fresh
// launch line beside the workspace already wearing the name.
func TestLaunchSessionRefusesAForeignRowRatherThanCreating(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	repo := t.TempDir()
	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
	foreignHolder(t, fake, "squatter")

	prompt := func() string { return "work prompt" }
	if _, err := d.launchSession(is, "ranger", "squatter", "", "fast", prompt); err == nil ||
		!strings.Contains(err.Error(), "held by a foreign workspace squatter") {
		t.Fatalf("want a foreign-hold refusal, got %v", err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a session was created under a label herdr already holds:\n%s", log)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a work prompt reached the foreign pane:\n%s", log)
	}
}

// The other half of failing closed: this is a refusal in DISPATCH, not in
// Resolve. `posse prompt`, `posse peek` and `posse kill` address a foreign
// row by label on purpose — the operator naming a workspace they can see —
// and taking that away would leave a squatting label with no way to clear it.
func TestResolveStillAnswersForeignRowsForTheOperator(t *testing.T) {
	b, fake := newTestBackend(t)
	foreignHolder(t, fake, "handmade")

	s, err := b.Resolve("handmade")
	if err != nil {
		t.Fatalf("the operator's own commands must still resolve it: %v", err)
	}
	if !s.Foreign || s.WorkspaceID != "wForeign" {
		t.Errorf("want the foreign row, got %+v", s)
	}
	if _, err := b.AgentTarget("handmade"); err != nil {
		t.Errorf("posse prompt/peek must still find its pane: %v", err)
	}
}
