package posse

// ranger-base-qajs. `posse init` used to seed the nine example PIDs straight
// into $RHQ_HOME/agents/, which is not a shelf — it is the roster dispatch
// routes against. Every generic was therefore a live lane, and label routing
// walks the roster in persona-name order, so `architect`, `business-manager`,
// `developer` and `devops` each sorted ahead of the persona the operator had
// actually written for that lane and took its unassigned beads. Measured on
// the crew this was found on: 14 lifetime closes for the seeded `developer`,
// 3 for `architect`, zero for six of the nine, and 8 open beads parked on
// generics nobody staffed (ranger-base-1t7r).
//
// Two claims are pinned here, and they are different claims: WHERE a fresh
// install puts the examples, and what the UPGRADE does to a home that already
// has them in agents/. The second is the one that can be worse than the bug —
// a retirement that takes a persona real work is assigned to, or that leaves
// a promoted home failing its own launch verify.
//
// Self-contained (own helpers, own fixtures): personas share this checkout,
// and a pin is worth nothing if an edit next door carries it away.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

func crewQAHome(t *testing.T) *App {
	t.Helper()
	// Hermetic against the operator fence (ADR 0031 §2): these fixtures drive
	// init as the operator, and a test process inside a real persona session
	// otherwise inherits RHQ_PERSONA from the ambient env.
	return NewAppAt(filepath.Join(t.TempDir(), "home"))
}

// seedNames is what the embed ships as examples — read from the seed rather
// than typed out, so adding a tenth example does not need this file edited.
func seedNames(t *testing.T) []string {
	t.Helper()
	names := exampleAgentNames(posse.Seed)
	if len(names) < 9 {
		t.Fatalf("the embed ships %d example PID(s) — expected the nine references", len(names))
	}
	return names
}

// A fresh install: no crew, and the examples readable where nothing loads
// them. `posse agent new` and a copy are how a persona comes to exist.
func TestFreshInitInstallsNoPersonas(t *testing.T) {
	t.Parallel()
	a := crewQAHome(t)
	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if got := a.ListAgents(); len(got) != 0 {
		t.Errorf("ListAgents after a fresh init: %v — an example in agents/ is a lane, and dispatch routes to it", got)
	}
	for _, n := range seedNames(t) {
		p := filepath.Join(a.ExampleAgentsDir(), n+".md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not on the shelf: %v — the examples must stay readable", p, err)
		}
	}
	// The empty roster is stated, not left for the operator to discover on a
	// dispatch pass that routes nothing.
	if !strings.Contains(out.String(), "no personas installed") {
		t.Errorf("init said %q — a crewless home must say where the examples are", out.String())
	}
}

// The upgrade. A home seeded by an older binary has the generics in agents/;
// re-running init retires the untouched ones and says so.
func TestInitRetiresUnmodifiedExamplePIDsFromAnExistingHome(t *testing.T) {
	t.Parallel()
	a := crewQAHome(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Put the shelf back into agents/ — that IS the pre-fix home.
	names := seedNames(t)
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), n+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.ListAgents()) != len(names) {
		t.Fatalf("fixture: %d persona(s) in agents/, want %d", len(a.ListAgents()), len(names))
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	if got := a.ListAgents(); len(got) != 0 {
		t.Errorf("agents/ after the upgrade: %v — the shipped generics must not keep routing", got)
	}
	if !strings.Contains(out.String(), "retired") {
		t.Errorf("init said %q — a retirement the operator cannot see is a file that vanished", out.String())
	}
	// A move, not a delete: every retired PID is still on the shelf to copy
	// back, and the print points at work that may be parked on the name.
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(a.ExampleAgentsDir(), n+".md")); err != nil {
			t.Errorf("%s left the home entirely: %v", n, err)
		}
	}
	if !strings.Contains(out.String(), "bd list --assignee") {
		t.Errorf("init said %q — retiring a name does not reassign what is parked on it, and must say so", out.String())
	}
}

// The rule that keeps the upgrade from being worse than the bug: an example
// the operator edited in place is not an example, it is their persona.
func TestInitKeepsACustomisedExamplePID(t *testing.T) {
	t.Parallel()
	a := crewQAHome(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v", err)
	}
	names := seedNames(t)
	mine, plain := names[0], names[1]
	for _, n := range []string{mine, plain} {
		b, _ := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), n+".md"))
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	live := filepath.Join(a.AgentsDir, mine+".md")
	b, _ := os.ReadFile(live)
	edited := string(b) + "\n<!-- the operator made this one theirs -->\n"
	if err := os.WriteFile(live, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	got, err := os.ReadFile(live)
	if err != nil || string(got) != edited {
		t.Fatalf("%s: %v — an edited PID carries bd history under its name; retiring it parks real work on a persona that no longer loads", live, err)
	}
	if !strings.Contains(out.String(), mine) {
		t.Errorf("init said %q — the one it kept must be named, or the operator cannot tell it apart from the ones it took", out.String())
	}
	if _, err := os.Stat(filepath.Join(a.AgentsDir, plain+".md")); err == nil {
		t.Errorf("%s survived — an untouched example beside a customised one still retires", plain)
	}
}

// A name config turns into behaviour is never retired: a home whose
// `coordinator:` or `default_persona:` resolves to nothing is a home that
// refuses or misroutes on the next pass.
func TestInitKeepsExamplePIDsTheConfigDependsOn(t *testing.T) {
	t.Parallel()
	names := exampleAgentNames(posse.Seed)
	for _, key := range []string{"coordinator", "default_persona", "verify_assignee"} {
		t.Run(key, func(t *testing.T) {
			a := crewQAHome(t)
			if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
				t.Fatalf("init: %v", err)
			}
			pinned := names[0]
			b, _ := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), pinned+".md"))
			if err := os.WriteFile(filepath.Join(a.AgentsDir, pinned+".md"), b, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(a.ConfigPath, []byte(key+": "+pinned+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
				t.Fatalf("re-init: %v", err)
			}
			if got := a.ListAgents(); len(got) != 1 || got[0] != pinned {
				t.Errorf("agents/ = %v — %s: %s names behaviour, and retiring it leaves the key pointing at nothing", got, key, pinned)
			}
		})
	}
}

// A promoted home is a copy of a commit with a manifest making a claim about
// it (ADR 0015 §3). Moving a file out from under that turns the next launch's
// verify into a MISSING, which refuses dispatch — so init does not, and says
// where the retirement belongs instead.
func TestInitDoesNotRetireUnderARealPromotion(t *testing.T) {
	t.Parallel()
	a := crewQAHome(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v", err)
	}
	names := seedNames(t)
	pinned := names[0]
	b, _ := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), pinned+".md"))
	if err := os.WriteFile(filepath.Join(a.AgentsDir, pinned+".md"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := HashPromotedSet(a.Home)
	if err != nil {
		t.Fatal(err)
	}
	m := &PromoteManifest{
		Version:    promoteManifestVersion,
		PromotedAt: "2026-08-27T00:00:00Z",
		Source:     "/somewhere/rhq",
		Repo:       "/somewhere",
		SHA:        "0000000000000000000000000000000000000000",
		Files:      files,
	}
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	if got := a.ListAgents(); len(got) != 1 || got[0] != pinned {
		t.Errorf("agents/ = %v — a promoted home's roster is the constitution's, not init's", got)
	}
	if !strings.Contains(out.String(), "constitution") {
		t.Errorf("init said %q — refusing without naming where the fix belongs is a silent no-op", out.String())
	}
	if v := a.VerifyPromoted(); len(v.Missing) > 0 || len(v.Changed) > 0 || v.Err != nil {
		t.Errorf("VerifyPromoted after init: missing=%v changed=%v err=%v — the launch verify must still pass", v.Missing, v.Changed, v.Err)
	}
}

// A SEEDED manifest has no commit behind it, so init owns it: after a
// retirement it re-stamps, and the next launch verifies clean. Without this
// the upgrade traded a routing leak for a home that refuses to dispatch at
// all — which is the "worse than the bug" the bead named.
func TestRetirementReStampsASeededManifest(t *testing.T) {
	t.Parallel()
	a := crewQAHome(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, n := range seedNames(t) {
		b, _ := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), n+".md"))
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Re-stamp the seeded manifest over the pre-fix layout, the way an older
	// binary's init would have left it.
	files, err := HashPromotedSet(a.Home)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil || !m.Seeded {
		t.Fatalf("fixture: manifest %+v, %v — init must leave a seeded manifest", m, err)
	}
	m.Files = files
	if err := m.write(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}

	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	v := a.VerifyPromoted()
	if v.Err != nil || len(v.Missing) > 0 || len(v.Changed) > 0 || len(v.Added) > 0 {
		t.Errorf("VerifyPromoted after the retirement: missing=%v changed=%v added=%v err=%v — dispatch refuses on any of these", v.Missing, v.Changed, v.Added, v.Err)
	}
	if m, _ := ReadPromoteManifest(a.PromoteManifestPath()); m == nil || !m.Seeded {
		t.Errorf("manifest after the retirement: %+v — it is still a seeded home, with no commit behind it", m)
	}
}

// The retirement only ever touches what the seed ships. A persona of the
// operator's that happens to sit in agents/ is never a candidate, whatever
// it is named.
func TestRetirementNeverTouchesAPersonaTheSeedDoesNotShip(t *testing.T) {
	t.Parallel()
	a := crewQAHome(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init: %v", err)
	}
	mine := filepath.Join(a.AgentsDir, "developer.md")
	body := "---\nname: developer\nlabels: [code]\n---\nYou are Developer.\n"
	if err := os.WriteFile(mine, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	if b, err := os.ReadFile(mine); err != nil || string(b) != body {
		t.Fatalf("%s: %v — init must never move a persona it did not ship", mine, err)
	}
}
