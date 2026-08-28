package rhq

// rangerhq-v6pe / rangerhq-c6u6: Route's coordinator refusal must hold for
// every spelling LoadAgent accepts, not the four strings the escape named.
// LoadAgent joins a path (AgentsDir/<name>.md); filepath.Join on Unix treats
// a leading slash on the second element as a separator, so `/coordinator`
// and `//coordinator` reach coordinator.md too. isCoordinator keys on
// identity (case-fold + Base(Clean)), so those hire nothing.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func coordinatorLoadSpellings() []string {
	return []string{
		"Coordinator",
		"COORDINATOR",
		"CoOrDiNaToR",
		"coordinator",
		"./coordinator",
		"./Coordinator",
		"./COORDINATOR",
		".//coordinator",
		"coordinator/../coordinator",
		"Coordinator/../Coordinator",
		"coordinator/../Coordinator",
		"./coordinator/../coordinator",
		"foo/../coordinator",
		"coordinator//../coordinator",
		"../agents/coordinator",
		"coordinator.md/../coordinator",
		"/coordinator",
		"//coordinator",
	}
}

// Every spelling that actually loads the coordinator PID is refused on the
// assignee path, with the ADR's why — not silently rerouted to a lane.
func TestRouteRefusesEveryLoadAgentAcceptedCoordinatorSpelling(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	writePersona(t, b.App, "developer", "[code, ops, hygiene]")
	cfg(t, b.App, "coordinator: coordinator\n")

	var loaded int
	for _, spelling := range coordinatorLoadSpellings() {
		if _, err := b.App.LoadAgent(spelling); err != nil {
			continue
		}
		loaded++
		p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "g9md", Assignee: spelling, Labels: []string{"hygiene"}}})
		if p != "" {
			t.Errorf("LoadAgent(%q) hired %q (%s)", spelling, p, why)
		}
		if !strings.Contains(why, "not a lane") {
			t.Errorf("LoadAgent(%q): why %q should name the coordinator refusal", spelling, why)
		}
	}
	if loaded < 4 {
		t.Fatalf("expected the original four variants to load on this filesystem, got %d", loaded)
	}
}

func TestDispatchNeverHiresCoordinatorByPathSpelling(t *testing.T) {
	for _, spelling := range []string{"./coordinator", "coordinator/../coordinator", "/coordinator"} {
		t.Run(spelling, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writeCoordinatorPID(t, b.App, "coordinator")
			idleClaude(t, fake)
			if _, err := b.App.LoadAgent(spelling); err != nil {
				t.Skipf("%q does not resolve to a PID on this filesystem", spelling)
			}
			body, _ := json.Marshal([]map[string]any{{
				"id": "g9md", "title": "queue hygiene", "priority": 1,
				"labels": []string{"hygiene"}, "assignee": spelling,
			}})
			repo := qaRepo(t, b.App, string(body), "")
			cfg(t, b.App, "coordinator: coordinator\nbeads:\n  - "+repo+"\n")
			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("dispatched %d beads", n)
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create") {
				t.Errorf("a session carrying the coordinator's PID was created:\n%s", log)
			}
			if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
				t.Errorf("the coordinator's bead was claimed:\n%s", log)
			}
		})
	}
}

func TestLaunchBeadRefusesCoordinatorPathSpelling(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	idleClaude(t, fake)
	cfg(t, b.App, "coordinator: coordinator\n")
	repo := t.TempDir()

	for _, spelling := range []string{"./coordinator", "coordinator/../coordinator", "/coordinator"} {
		t.Run(spelling, func(t *testing.T) {
			if _, err := b.App.LoadAgent(spelling); err != nil {
				t.Skipf("%q does not resolve", spelling)
			}
			session, err := d.LaunchBead(RepoIssue{BdIssue: BdIssue{ID: "g9md", Title: "queue hygiene", Labels: []string{"hygiene"}, Assignee: spelling}, Dir: repo})
			if err == nil {
				t.Fatalf("cockpit launched %s", session)
			}
			if !strings.Contains(err.Error(), "unroutable") {
				t.Errorf("got %v", err)
			}
		})
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("the cockpit created a session:\n%s", log)
	}
}

// Live config names business-manager, not coordinator. Same identity rule.
func TestRouteRefusesConfiguredCoordinatorNameVariants(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "business-manager")
	writePersona(t, b.App, "developer", "[hygiene]")
	cfg(t, b.App, "coordinator: business-manager\n")

	for _, spelling := range []string{"Business-Manager", "BUSINESS-MANAGER", "./business-manager", "business-manager/../business-manager"} {
		t.Run(spelling, func(t *testing.T) {
			if _, err := b.App.LoadAgent(spelling); err != nil {
				t.Skipf("%q does not resolve", spelling)
			}
			p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "g9md", Assignee: spelling, Labels: []string{"hygiene"}}})
			if p != "" {
				t.Errorf("hired %q (%s)", p, why)
			}
			if !strings.Contains(why, "not a lane") {
				t.Errorf("why %q", why)
			}
		})
	}
}

func TestRouteRefusesCoordinatorDefaultPersonaPathSpelling(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	cfg(t, b.App, "coordinator: coordinator\ndefault_persona: ./coordinator\n")

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-3", Labels: []string{"mystery"}}})
	if p != "" {
		t.Errorf("default_persona: ./coordinator routed as %q (%s)", p, why)
	}
	if !strings.Contains(why, "config error") {
		t.Errorf("why %q should name the config error", why)
	}
}

func TestCoordinatorKeyAndCanonAgentAgreeOnASCII(t *testing.T) {
	// CanonAgent uses EqualFold; isCoordinator uses ToLower. For ValidName
	// ASCII they must agree, or the assignee branch can return a canonical
	// coordinator the raw-string check missed. Non-ASCII never reaches
	// CanonAgent: ValidName is [A-Za-z0-9_-].
	names := []string{"coordinator", "Coordinator", "COORDINATOR", "business-manager", "Business-Manager"}
	for _, a := range names {
		for _, b := range names {
			if strings.EqualFold(a, b) != (strings.ToLower(a) == strings.ToLower(b)) {
				t.Errorf("EqualFold(%q,%q) disagrees with ToLower", a, b)
			}
		}
	}
	for _, s := range []string{"ß", "İ", "K", "ς", "ſ"} {
		if ValidName(s) {
			t.Errorf("ValidName(%q) is true; the EqualFold/ToLower split becomes reachable", s)
		}
	}
	if filepath.Base(filepath.Clean("/coordinator")) != "coordinator" {
		t.Fatal("filepath.Base(Clean(/coordinator)) is no longer coordinator; /coordinator would slip the key")
	}
}

func TestCoordinatorPrefixNameIsStillALane(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	writePersona(t, b.App, "coordinator-ops", "[ops]")
	cfg(t, b.App, "coordinator: coordinator\n")

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "x", Assignee: "coordinator-ops"}})
	if p != "coordinator-ops" {
		t.Errorf("prefix name refused or rerouted: %q (%s)", p, why)
	}
}
