package posse

// ADR 0033 §2 authorizes on identity, but Route compares the raw assignee
// string out of issues.jsonl to config `coordinator:`. LoadAgent resolves a
// *path* (AgentsDir/<name>.md), so it accepts spellings the equality does
// not — and the string that walks past the refusal is the one the launcher
// then cats a PID from. §3 names issues.jsonl as hostile input; this is the
// seam that survives a name-equality implementation.
//
// Filed as rangerhq-c6u6 (escape from rangerhq-t4qm), fixed there: Route
// compares canonical identity (isCoordinator) and returns the agents-dir
// spelling (CanonAgent). Each of these failed against Route before that.

import (
	"strings"
	"testing"
)

// The spellings a hostile issues.jsonl can write. ./coordinator and
// coordinator/../coordinator load on any filesystem; the case variants load
// wherever the agents dir is case-insensitive (APFS default).
var coordinatorSpellings = []string{
	"Coordinator",
	"COORDINATOR",
	"./coordinator",
	"coordinator/../coordinator",
}

// Route refuses on identity, not on spelling.
func TestRouteRefusesCoordinatorNameVariants(t *testing.T) {
	t.Parallel()
	for _, spelling := range coordinatorSpellings {
		t.Run(spelling, func(t *testing.T) {
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writeCoordinatorPID(t, b.App, "coordinator")
			cfg(t, b.App, "coordinator: coordinator\n")

			// Only meaningful where this spelling actually reaches the PID:
			// that is what makes it an escalation rather than a typo.
			if _, err := b.App.LoadAgent(spelling); err != nil {
				t.Skipf("%q does not resolve to a PID on this filesystem", spelling)
			}
			p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "g9md", Assignee: spelling, Labels: []string{"hygiene"}}})
			if p != "" {
				t.Errorf("assignee %q hired the coordinator as %q (%s)", spelling, p, why)
			}
			if !strings.Contains(why, "not a lane") {
				t.Errorf("assignee %q: why %q should name the coordinator refusal", spelling, why)
			}
		})
	}
}

// The whole point, stated the way the operator can check it: the g9md repro
// with one letter capitalized still creates no session and takes no claim.
func TestDispatchNeverHiresCoordinatorByNameVariant(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	idleClaude(t, fake)

	repo := qaRepo(t, b.App, `[{"id":"g9md","title":"queue hygiene","priority":1,"labels":["hygiene"],"assignee":"Coordinator"}]`, "")
	cfg(t, b.App, "coordinator: coordinator\nbeads:\n  - "+repo+"\n")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("dispatched %d beads to the coordinator under a case variant", n)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a session carrying the coordinator's PID was created:\n%s", log)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("the coordinator's bead was claimed:\n%s", log)
	}
}

// default_persona shares the compare, so it shares the hole — and fails
// quietly: no config-error line, just the coordinator as the fallback lane.
func TestRouteRefusesCoordinatorAsDefaultPersonaVariant(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	cfg(t, b.App, "coordinator: coordinator\ndefault_persona: Coordinator\n")

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-3", Labels: []string{"mystery"}}})
	if p != "" {
		t.Errorf("default_persona: Coordinator routed as %q (%s)", p, why)
	}
	if !strings.Contains(why, "config error") {
		t.Errorf("why %q should name the config error", why)
	}
}

// The quiet one: the drift is in the operator's file, not the queue.
// Capitalizing a name is how people write names — and it disables all three
// refusals at once while the instance looks correctly configured.
func TestCoordinatorKeySpellingDriftStillRefuses(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator") // file: coordinator.md
	cfg(t, b.App, "coordinator: Coordinator\n")  // key: as one writes a name

	// The label loop's skip is keyed on the same string.
	if p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-2", Labels: []string{"ops"}}}); p != "" {
		t.Errorf("label routing hired the coordinator as %q (%s)", p, why)
	}
	// And with the key drifted, even the exact-case assignee routes.
	if p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-1", Assignee: "coordinator"}}); p != "" {
		t.Errorf("assignee routing hired the coordinator as %q (%s)", p, why)
	}
}
