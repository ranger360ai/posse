//go:build posse_arm2

package posse

// ADR 0033 §1–2 — the coordinator is not a lane: dispatch never hires the
// persona that governs it. The g9md sighting (ranger-base-kb7) was a bead
// assigned to the coordinator minting a per-bead session that carried its
// whole PID — session direction and `git push` — and pushing unattended.
// The refusal is keyed on config `coordinator:` alone and lives in Route,
// which both
// launchers share, so these tests assert on the two things the operator can
// count: zero sessions created, zero claims taken.

import (
	"os"
	"strings"
	"testing"
)

// writeCoordinatorPID gives the coordinator a real, loadable PID with a
// lane's label vocabulary — `labels: [orchestration, ops]` on it, the
// second of the three holes. The refusal must hold anyway.
func writeCoordinatorPID(t *testing.T, a *App, name string) {
	t.Helper()
	writePersona(t, a, name, "[orchestration, ops]")
}

// cfg writes config.yaml. Extra top-level keys go before `beads:` — the
// flat-YAML block list ends at the next column-0 key, and betting a test on
// that is how you debug the wrong thing.
func cfg(t *testing.T, a *App, body string) {
	t.Helper()
	if err := os.WriteFile(a.ConfigPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// §1: absent key = no coordinator = pre-0033 behavior, wholesale. The
// engine carries no crew name (rangerhq-gk4k); nothing is refused until the
// instance names someone.
func TestCoordinatorAbsentKeepsOldRouting(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	cfg(t, b.App, "default_persona: coordinator\n")

	if got := b.App.Coordinator(); got != "" {
		t.Errorf("no coordinator: key should mean no coordinator, got %q", got)
	}
	for _, tc := range []struct {
		name, assignee string
		labels         []string
	}{
		{"assignee", "coordinator", nil},
		{"label", "", []string{"ops"}},
		{"default_persona", "", []string{"mystery"}},
	} {
		p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "x", Assignee: tc.assignee, Labels: tc.labels}})
		if p != "coordinator" {
			t.Errorf("%s: want coordinator routed with no coordinator: key, got %q (%s)", tc.name, p, why)
		}
	}
}

// §2: every path Route has, refused — and the why says what to do instead.
func TestRouteRefusesCoordinatorOnEveryPath(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	writePersona(t, b.App, "developer", "[code, ops]")

	cfg(t, b.App, "coordinator: coordinator\n")

	for _, tc := range []struct {
		name    string
		is      BdIssue
		wantWhy []string
	}{
		{
			"assignee",
			BdIssue{ID: "a-1", Assignee: "coordinator", Labels: []string{"code"}},
			[]string{"not a lane", "coordinator triages by hand"},
		},
		{
			// Her PID's `labels: [orchestration, ops]` overlap the bead's;
			// the loop skips her PID, so the bead reads as label-unmatched.
			"label",
			BdIssue{ID: "a-2", Labels: []string{"orchestration"}},
			[]string{"no assignee/label match"},
		},
	} {
		p, why := d.Route(RepoIssue{BdIssue: tc.is})
		if p != "" {
			t.Errorf("%s: coordinator routed as %q", tc.name, p)
		}
		for _, want := range tc.wantWhy {
			if !strings.Contains(why, want) {
				t.Errorf("%s: why %q missing %q", tc.name, why, want)
			}
		}
	}

	// The third hole: naming her as the fallback lane is a config error, and
	// the line says so rather than leaving the operator to guess why every
	// fallthrough bead went quiet.
	cfg(t, b.App, "coordinator: coordinator\ndefault_persona: coordinator\n")
	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-3", Labels: []string{"mystery"}}})
	if p != "" {
		t.Errorf("default_persona: coordinator routed as %q", p)
	}
	for _, want := range []string{"default_persona", "coordinator", "config error"} {
		if !strings.Contains(why, want) {
			t.Errorf("default_persona: why %q missing %q", why, want)
		}
	}

	// An explicit assignment is never silently rerouted to a lane that
	// shares its labels: unroutable-and-loud, not "dispatch guessed".
	if got, _ := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-4", Assignee: "coordinator", Labels: []string{"code"}}}); got == "developer" {
		t.Error("a coordinator-assigned bead fell through to label routing")
	}
	// A lane is untouched by any of this.
	if got, _ := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-5", Labels: []string{"code"}}}); got != "developer" {
		t.Errorf("lane routing broke: got %q", got)
	}
}

// §5's reasoning, from the other side: authorization is keyed on config
// alone, never on what loads or what a PID grants. A named coordinator with
// no readable PID is still refused — otherwise the assignee branch falls
// through and label routing hands her bead to a lane.
func TestRouteRefusesCoordinatorWithNoLoadablePID(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "developer", "[code]")
	cfg(t, b.App, "coordinator: coordinator\n")

	p, why := d.Route(RepoIssue{BdIssue: BdIssue{ID: "a-1", Assignee: "coordinator", Labels: []string{"code"}}})
	if p != "" {
		t.Errorf("unloadable coordinator routed as %q (%s)", p, why)
	}
	if !strings.Contains(why, "not a lane") {
		t.Errorf("why %q should name the coordinator refusal", why)
	}
}

// The g9md repro over the fake queue: a queue-hygiene bead assigned to
// the coordinator. `hygiene` is a Dial B label (→ fast), which is how the
// clone ran at a tier nobody chose — so the pass is run at every tier the
// operator can force, and each must launch nothing.
func TestDispatchNeverHiresCoordinatorAtAnyTier(t *testing.T) {
	t.Parallel()
	for _, tier := range []string{"", TierFast, TierStandard, TierStrong} {
		name := tier
		if name == "" {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.Tier = tier
			writeCoordinatorPID(t, b.App, "coordinator")
			writePersona(t, b.App, "developer", "[code]")
			idleClaude(t, fake)

			repo := qaRepo(t, b.App, `[{"id":"g9md","title":"queue hygiene","priority":1,"labels":["hygiene"],"assignee":"coordinator"}]`, "")
			cfg(t, b.App, "coordinator: coordinator\nbeads:\n  - "+repo+"\n")

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("dispatched %d beads to the coordinator", n)
			}
			out := dispatcherOut(d)
			if !strings.Contains(out, "g9md") || !strings.Contains(out, "not a lane") {
				t.Errorf("no refusal line for the coordinator-assigned bead:\n%s", out)
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create") {
				t.Errorf("a session was created for the coordinator:\n%s", log)
			}
			if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
				t.Errorf("the coordinator's bead was claimed:\n%s", log)
			}
		})
	}
}

// --persona is a filter over Route's answer, not a way past it (§2: no flag
// overrides — not --persona, not --tier, not --resume).
func TestDispatchPersonaFlagCannotHireCoordinator(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	idleClaude(t, fake)

	repo := qaRepo(t, b.App, `[{"id":"g9md","title":"queue hygiene","priority":1,"labels":["hygiene"],"assignee":"coordinator"}]`, "")
	cfg(t, b.App, "coordinator: coordinator\nbeads:\n  - "+repo+"\n")

	n, err := d.Run("", "coordinator", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("--persona coordinator dispatched %d beads", n)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("--persona coordinator created a session:\n%s", log)
	}
}

// An in_progress bead the coordinator already holds: unroutable, and
// --resume does not re-prompt it. The Consequences say it is left alone —
// today's interim behavior made permanent, until G9 names it on the board.
func TestDispatchResumeLeavesCoordinatorHeldBeadAlone(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.Resume = true
	writeCoordinatorPID(t, b.App, "coordinator")
	idleClaude(t, fake)

	repo := qaRepo(t, b.App, `[{"id":"g9md","title":"queue hygiene","priority":1,"labels":["hygiene"],"assignee":"coordinator","status":"in_progress"}]`, "")
	cfg(t, b.App, "coordinator: coordinator\nbeads:\n  - "+repo+"\n")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("--resume dispatched %d coordinator-held beads", n)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") || strings.Contains(log, "agent prompt") {
		t.Errorf("--resume touched the coordinator's held bead:\n%s", log)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "update") {
		t.Errorf("--resume wrote to the coordinator's held bead:\n%s", log)
	}
}

// The cockpit's `d` is a launcher too: same Route, so the same refusal, as
// an error the operator reads instead of a pass line.
func TestLaunchBeadRefusesCoordinator(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writeCoordinatorPID(t, b.App, "coordinator")
	writePersona(t, b.App, "developer", "[code]")
	idleClaude(t, fake)

	repo := t.TempDir()
	cfg(t, b.App, "coordinator: coordinator\n")

	for _, tc := range []struct {
		name string
		is   BdIssue
	}{
		{"assigned", BdIssue{ID: "g9md", Title: "queue hygiene", Labels: []string{"hygiene"}, Assignee: "coordinator"}},
		{"label", BdIssue{ID: "g9me", Title: "ops thing", Labels: []string{"ops"}}},
	} {
		session, err := d.LaunchBead(RepoIssue{BdIssue: tc.is, Dir: repo})
		if err == nil {
			t.Fatalf("%s: cockpit dispatch launched %s for the coordinator", tc.name, session)
		}
		if !strings.Contains(err.Error(), "unroutable") {
			t.Errorf("%s: want unroutable refusal, got %v", tc.name, err)
		}
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("the cockpit created a session for the coordinator:\n%s", log)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("the cockpit claimed a bead for the coordinator:\n%s", log)
	}

	// The lane next door still launches — the refusal is one name wide.
	if _, err := d.LaunchBead(RepoIssue{BdIssue: BdIssue{ID: "g9mf", Title: "fix", Labels: []string{"code"}}, Dir: repo}); err != nil {
		t.Fatalf("lane dispatch broke: %v", err)
	}
	if !strings.Contains(calls(t, fake), "workspace create") {
		t.Error("the lane's session was never created")
	}
}
