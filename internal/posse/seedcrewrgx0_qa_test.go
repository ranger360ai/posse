//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-rgx0: `seeded` was weaker than it read, and the retirement
// believed it.
//
// SeedPromoteManifest hashes whatever is on disk AT THE MOMENT IT RUNS, and
// init used to call it on any home that had no manifest — an upgrade
// included. Every home created before 95c4b70 therefore got its manifest from
// a later init, not from the init that seeded it, so on a home where the
// operator had adopted a generic in place the `seeded` manifest attests to
// THEIR file. The old rule read that as untouched-since-seeded and retired
// it. Measured live in two inits: init #1 correctly kept the adopted qa.md
// and printed why, and then wrote the record that made init #2 take it.
//
// Since ranger-base-h7cd, init writes no manifest over a home that already
// has a constitution — but that is not what makes this safe, and this pin
// must not start passing because of it: every home an older posse
// initialised still HAS that post-hoc manifest, and the retirement still
// reads `seeded` (init.go). So the fixture plants it the way that init did,
// and the pin asks the question it always asked.
//
// The fix judges from posse's side of the line instead — the table of digests
// posse has shipped (exampledigests.go) — so the home's own say-so is never
// evidence about who wrote a file. Both halves are pinned here: the operator's
// file survives any number of inits, and an older RELEASE still retires with
// no manifest in the home at all.
//
// Self-contained (own helpers, own fixtures): a pin an edit next door can
// carry away pinned nothing.

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// olderHomeRgx0 is a home as a posse older than ADR 0015 left it: the nine
// examples live in agents/, no shelf, and no promoted.json — the shape every
// install that predates ranger-base-qajs has. src is the bytes that older
// binary seeded with.
func olderHomeRgx0(t *testing.T, seed map[string][]byte) *App {
	t.Helper()
	// Hermetic against the operator fence (ADR 0031 §2): see initTestApp.
	a := NewAppAt(filepath.Join(t.TempDir(), "home"))
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("seeding the fixture home: %v", err)
	}
	for name, b := range seed {
		if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(a.ExampleAgentsDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	return a
}

// embeddedNine is what this posse ships, read the way an older init laid it
// down: live, in agents/.
func embeddedNine(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, n := range exampleAgentNames(posse.Seed) {
		b, err := fs.ReadFile(posse.Seed, "agents/"+n+".md")
		if err != nil {
			t.Fatal(err)
		}
		out[n] = b
	}
	if len(out) < 9 {
		t.Fatalf("the seed ships %d example PID(s), want the nine", len(out))
	}
	return out
}

// TestAPostHocSeededManifestNeverRetiresAnAdoptedPersona is the measured
// two-init reproduction. It fails if the retirement ever takes the home's own
// manifest as evidence of who wrote a file again.
func TestAPostHocSeededManifestNeverRetiresAnAdoptedPersona(t *testing.T) {
	t.Parallel()
	a := olderHomeRgx0(t, embeddedNine(t))
	const mine = "qa"
	live := filepath.Join(a.AgentsDir, mine+".md")
	b, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	// The operator adopts it in place, years of bd history under that name.
	adopted := string(b) + "\n<!-- mine -->\n"
	if err := os.WriteFile(live, []byte(adopted), 0o644); err != nil {
		t.Fatal(err)
	}

	// init #1: keeps it, correctly — and, on the posse that was measured here,
	// writes the manifest that hashes it. A posse since ranger-base-h7cd
	// writes none over a populated home, so the fixture lays down exactly what
	// that older init left behind: the state every upgraded home is in today.
	var out1 strings.Builder
	if err := a.initFrom(&out1, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init #1: %v", err)
	}
	if got, err := os.ReadFile(live); err != nil || string(got) != adopted {
		t.Fatalf("init #1 already took %s.md — it differs from the example this posse ships:\n%s", mine, out1.String())
	}
	man, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if man == nil {
		if err := a.SeedPromoteManifest(); err != nil {
			t.Fatalf("fixture: planting the older init's manifest: %v", err)
		}
		if man, err = ReadPromoteManifest(a.PromoteManifestPath()); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	if man == nil || !man.Seeded {
		t.Fatalf("fixture: no seeded manifest to reproduce the hazard with (%+v)", man)
	}
	if man.Files["agents/"+mine+".md"] != sha256Bytes([]byte(adopted)) {
		t.Fatalf(`fixture: the seeded manifest does not attest to the operator's own file,
so the hazard this pin exists for is not built. Re-read ranger-base-rgx0.`)
	}

	// init #2: the record init #1 wrote must not be able to authorise this.
	var out2 strings.Builder
	if err := a.initFrom(&out2, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init #2: %v", err)
	}
	got, err := os.ReadFile(live)
	if err != nil || string(got) != adopted {
		t.Fatalf(`ranger-base-rgx0 REGRESSED — agents/%s.md stopped routing on the second
init of a home that predates the manifest. A `+"`seeded`"+` manifest hashes whatever
was on disk when it was first written, so it can attest to a persona the
operator adopted in place; provenance for a seed file comes from
isShippedExample (exampledigests.go), never from the home's own record.
init #2 said:
%s`, mine, out2.String())
	}
	if !strings.Contains(out2.String(), "kept agents/"+mine+".md") {
		t.Errorf("init #2 kept %s.md without saying so — a persona that stays silently is one the operator cannot audit:\n%s", mine, out2.String())
	}
}

// TestAnOlderReleaseRetiresOnTheShippedTableAlone is the other half, against
// bytes posse really shipped rather than a fixture: the first published
// examples (5668b76), on a home with no manifest at all. If retirement ever
// needs the manifest again, this goes red — and if the table forgets that
// release, so does it.
func TestAnOlderReleaseRetiresOnTheShippedTableAlone(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	const rev = "5668b76" // posse: initial publication (2026-08-23)
	seed := map[string][]byte{}
	for _, n := range exampleAgentNames(posse.Seed) {
		b, err := exec.Command("git", "show", rev+":examples/agents/"+n+".md").Output()
		if err != nil {
			t.Skipf("no history for %s at %s: %v", n, rev, err)
		}
		seed[n] = b
	}
	a := olderHomeRgx0(t, seed)
	// The fixture only means something if those bytes are not this build's.
	cur, err := os.ReadFile(filepath.Join(a.AgentsDir, "qa.md"))
	if err != nil {
		t.Fatal(err)
	}
	if embedded, err := fs.ReadFile(posse.Seed, "agents/qa.md"); err == nil && string(cur) == string(embedded) {
		t.Skipf("examples/agents has not changed since %s — no version skew to test", rev)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	if live := a.ListAgents(); len(live) != 0 {
		t.Fatalf(`ranger-base-8ehw REGRESSED — %d generic(s) from %s still routing after an
upgrade, on a home with no manifest to consult. Identity comes from the table
of digests posse has shipped (exampledigests.go), which needs nothing in the
home to answer.
init said:
%s`, len(live), rev, out.String())
	}
	if strings.Contains(out.String(), "it is yours now") {
		t.Errorf("init blamed the operator for bytes posse itself shipped at %s:\n%s", rev, out.String())
	}
	// A move, not a delete: those bytes exist nowhere else in the home.
	for n, was := range seed {
		shelf, err := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), n+".md"))
		if err != nil {
			t.Fatalf("%s left the home entirely: %v", n, err)
		}
		if string(shelf) != string(was) {
			t.Errorf("shelf copy of %s is not what was retired — the retired bytes are gone from the home", n)
		}
	}
}
