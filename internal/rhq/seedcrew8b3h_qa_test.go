package rhq

// ranger-base-8b3h, verifying ranger-base-qajs. The fresh-install half of
// that close holds: `posse init` puts the nine examples on the shelf and
// leaves agents/ empty. The UPGRADE half — the half the bead called the one
// that "must not be worse than the bug" — has two defects, pinned here.
//
// Both live in retireExamplePIDs (init.go). Both are pinned the way
// rangerhq-th7l asks for: the CURRENT behaviour is asserted, and the
// assertion fails loudly the moment it changes, so the suite stays green and
// neither defect can be fixed — or made worse — without this file saying so.
//
// Self-contained (own helpers, own fixtures): a pin an edit next door can
// carry away pinned nothing.

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ranger360ai/posse"
)

func homeQA8b3h(t *testing.T) *App {
	t.Helper()
	return NewAppAt(filepath.Join(t.TempDir(), "home"))
}

// olderSeed is posse.Seed with every example PID carrying one extra line —
// which is exactly what a real release did: 95c4b70 added
// `- Bash(posse promote:*)` to the deny list of all nine, so every home
// seeded before that commit holds bytes the running binary no longer ships.
// The example PIDs are shipped prose; they change, and every change makes
// another population of homes look like this fixture.
func olderSeed(t *testing.T) fs.FS {
	t.Helper()
	m := fstest.MapFS{}
	err := fs.WalkDir(posse.Seed, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(posse.Seed, p)
		if rerr != nil {
			return rerr
		}
		if strings.HasPrefix(p, "agents/") && strings.HasSuffix(p, ".md") {
			b = append(append([]byte{}, b...), []byte("\n# one line an older posse shipped\n")...)
		}
		m[p] = &fstest.MapFile{Data: b, Mode: 0o644}
		return nil
	})
	if err != nil {
		t.Fatalf("building the older seed: %v", err)
	}
	return m
}

// preFixHome is a home as an OLDER posse left it: the examples seeded live,
// into agents/, which is what every install that predates ranger-base-qajs
// looks like. src is that older binary's seed tree.
func preFixHome(t *testing.T, src fs.FS) *App {
	t.Helper()
	a := homeQA8b3h(t)
	if err := a.initFrom(io.Discard, src, "older"); err != nil {
		t.Fatalf("seeding the pre-fix home: %v", err)
	}
	// Pre-fix init copied `agents` into AgentsDir; today's copies it to the
	// shelf. Move them where the older binary put them, then drop the shelf
	// so the home is shaped exactly as that binary left it.
	names := exampleAgentNames(src)
	if len(names) < 9 {
		t.Fatalf("fixture seed ships %d example PID(s), want the nine", len(names))
	}
	for _, n := range names {
		b, err := fs.ReadFile(src, "agents/"+n+".md")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(a.ExampleAgentsDir()); err != nil {
		t.Fatal(err)
	}
	// Re-stamp the seeded manifest over that shape: this is the manifest the
	// older `posse init` would have written, and it is the record of what was
	// laid down — the fact the second test below turns on.
	if err := os.Remove(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	return a
}

// DEFECT 1 (ranger-base-8b3h). retireExamplePIDs decides "is this still the
// shipped example?" by comparing the live file with the example the RUNNING
// binary embeds. That comparison can only ever recognise the current
// version's bytes, so a home seeded by any earlier posse retires NOTHING —
// the nine generics keep routing, which is the whole bug ranger-base-qajs
// was filed to remove — and init tells the operator it kept them because
// they "edited" them, which they did not.
//
// The evidence that this is version skew and not a real edit is in the home
// already: the seeded manifest holds the hash init recorded when it laid the
// file down, and the live file still matches it. retireExamplePIDs reads
// that manifest three lines earlier, for a different question.
func TestUpgradeFromAnOlderSeedRetiresNothingAndBlamesTheOperator(t *testing.T) {
	src := olderSeed(t)
	a := preFixHome(t, src)
	names := exampleAgentNames(posse.Seed)

	// The home is untouched since it was seeded — the manifest says so.
	man, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || man == nil {
		t.Fatalf("fixture manifest: %v", err)
	}
	for _, n := range names {
		rel := "agents/" + n + ".md"
		want, ok := man.Files[rel]
		if !ok {
			t.Fatalf("fixture: manifest does not name %s", rel)
		}
		got, err := sha256File(filepath.Join(a.AgentsDir, n+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("fixture: %s already differs from what was seeded — the fixture is wrong, not the code", rel)
		}
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	live := a.ListAgents()

	if len(live) == 0 {
		t.Fatalf(`ranger-base-8b3h DEFECT 1 LOOKS FIXED — the upgrade now retires a home
seeded by an older posse (agents/ is empty). If retirement was moved onto the
seeded manifest's own hashes (untouched-since-seeded) rather than the running
binary's bytes, that is the fix: delete this pin and say so on the bead.
init said:
%s`, out.String())
	}
	if len(live) != len(names) {
		t.Fatalf(`ranger-base-8b3h DEFECT 1 MOVED — %d of %d generic(s) still routing after the
upgrade, expected all %d. Re-read the bead before touching this pin.
init said:
%s`, len(live), len(names), len(names), out.String())
	}
	// ...and the reason given is false. The operator edited nothing.
	if !strings.Contains(out.String(), "edited since it was seeded — it is yours now") {
		t.Fatalf(`ranger-base-8b3h DEFECT 1 CHANGED — the nine are still kept, but init no longer
calls them operator edits. If the message now names version skew, the pin needs
rewording, not deleting: the leak is still open.
init said:
%s`, out.String())
	}
	// And the shelf now holds the CURRENT bytes, so nothing on disk tells the
	// operator that the nine live files were ever ours.
	for _, n := range names {
		shelf, err := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), n+".md"))
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		want, err := fs.ReadFile(posse.Seed, "agents/"+n+".md")
		if err != nil {
			t.Fatal(err)
		}
		if string(shelf) != string(want) {
			t.Errorf("shelf copy of %s is not the current shipped example", n)
		}
	}
}

// DEFECT 2 (ranger-base-8b3h). When a retirement happens on a `seeded` home,
// init re-stamps the manifest with HashPromotedSet(Home) — the WHOLE promoted
// set, not the agents/ entries it just changed. Anything already drifted in
// config.yaml, recipes/ or skills/ is silently re-anchored as the new truth,
// so the ADR 0015 §3 launch verify that would have refused dispatch on it
// comes back clean instead. Pre-fix `posse init` never wrote over an existing
// manifest at all (SeedPromoteManifest returns early), so this is new, and it
// is wider than the close's stated intent ("re-stamps rather than leaving the
// next launch to report the files it removed on purpose as MISSING").
//
// The narrow form is available: drop the retired paths from man.Files.
func TestRetirementRestampBlessesUnrelatedConstitutionDrift(t *testing.T) {
	a := preFixHome(t, posse.Seed) // same-version home, so the retirement fires
	recipe := filepath.Join(a.RecipesDir, "scratch.yaml")
	if _, err := os.Stat(recipe); err != nil {
		ents, _ := os.ReadDir(a.RecipesDir)
		if len(ents) == 0 {
			t.Skip("seed ships no recipes to drift")
		}
		recipe = filepath.Join(a.RecipesDir, ents[0].Name())
	}
	before, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || before == nil {
		t.Fatalf("fixture manifest: %v", err)
	}
	rel, err := filepath.Rel(a.Home, recipe)
	if err != nil {
		t.Fatal(err)
	}
	rel = filepath.ToSlash(rel)
	anchored, ok := before.Files[rel]
	if !ok {
		t.Fatalf("fixture: manifest does not name %s", rel)
	}

	// Drift the constitution the way something the verify exists to catch
	// would: content the operator did not put there.
	b, err := os.ReadFile(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recipe, append(b, []byte("\n# not the operator\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted, err := sha256File(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if drifted == anchored {
		t.Fatal("fixture: the drift did not change the hash")
	}
	if v := a.VerifyPromoted(); v.OK() {
		t.Fatal("fixture: the launch verify does not see the drift — nothing to launder")
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	if !strings.Contains(out.String(), "retired") {
		t.Fatalf("fixture: no retirement fired, so no re-stamp: %s", out.String())
	}

	after, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || after == nil {
		t.Fatalf("manifest after: %v", err)
	}
	switch after.Files[rel] {
	case anchored:
		t.Fatalf(`ranger-base-8b3h DEFECT 2 LOOKS FIXED — the retirement re-stamp no longer
re-hashes %s. If it now only drops the retired agents/ entries, that is the fix:
delete this pin and say so on the bead.`, rel)
	case drifted:
		// The pinned defect: drift outside agents/ is now blessed, and the
		// verify that would have refused dispatch comes back clean.
		if v := a.VerifyPromoted(); !v.OK() {
			t.Fatalf("re-stamp recorded the drifted hash but the verify still fails: %s", v.Line())
		}
	default:
		t.Fatalf(`ranger-base-8b3h DEFECT 2 MOVED — %s is now anchored at neither the seeded
hash nor the drifted one. Re-read the bead before touching this pin.`, rel)
	}
}
