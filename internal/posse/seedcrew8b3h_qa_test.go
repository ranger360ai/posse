//go:build posse_arm2

package posse

// ranger-base-8b3h, verifying ranger-base-qajs. The fresh-install half of
// that close holds: `posse init` puts the nine examples on the shelf and
// leaves agents/ empty. The UPGRADE half — the half the bead called the one
// that "must not be worse than the bug" — had two defects, pinned here.
//
// Both are FIXED. DEFECT 1 (ranger-base-8ehw) lives in retireExamplePIDs
// (init.go); its pin is the inversion — it asserts the fixed behaviour and
// fails if judging ever goes back to the running binary's bytes alone.
// DEFECT 2 (ranger-base-9afo) spanned retireExamplePIDs and initFrom's
// "repaired" re-stamp; its pin now asserts the fixed behaviour directly
// (an upgrade retirement leaves unrelated drift anchored, so the launch
// verify still refuses it) rather than pinning the defect.
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
	// Hermetic against the operator fence (ADR 0031 §2): see initTestApp.
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

// shipFixtureAsARelease enters the fixture's example PIDs in posse's record
// of what posse has shipped (exampledigests.go). That table is generated from
// this repo's history, so a fixture standing in for a release that never
// existed has to be entered the way a real one is — by appending its digest.
// The real entries are never disturbed and the table is restored after.
func shipFixtureAsARelease(t *testing.T, src fs.FS) {
	t.Helper()
	for _, n := range exampleAgentNames(src) {
		rel := "agents/" + n + ".md"
		b, err := fs.ReadFile(src, rel)
		if err != nil {
			t.Fatal(err)
		}
		was := shippedExampleDigests[rel]
		t.Cleanup(func() { shippedExampleDigests[rel] = was })
		shippedExampleDigests[rel] = append(append([]string{}, was...), sha256Bytes(b))
	}
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

// DEFECT 1 (ranger-base-8b3h), FIXED by ranger-base-8ehw. retireExamplePIDs
// used to decide "is this still the shipped example?" against the example the
// RUNNING binary embeds, which can only ever recognise the current version's
// bytes. The example PIDs are shipped prose and they change — 95c4b70 added
// `- Bash(posse promote:*)` to the deny list of all nine — so every home
// seeded by an earlier posse retired NOTHING (the leak ranger-base-qajs was
// filed to remove, still open) and init told the operator it kept them
// because they "edited" them, which they did not.
//
// The rule now asks posse's own record instead: the table of every digest
// posse has shipped for each example (exampledigests.go) recognises an older
// release's bytes for any version, and — unlike the home's `seeded` manifest,
// which ranger-base-rgx0 took out of this decision — nothing an operator
// wrote can be in it. This pin is the inversion of the one it replaces: it
// fails if judging ever goes back to the running binary's bytes alone.
func TestUpgradeFromAnOlderSeedRetiresTheNine(t *testing.T) {
	src := olderSeed(t)
	shipFixtureAsARelease(t, src)
	a := preFixHome(t, src)
	names := exampleAgentNames(posse.Seed)

	// Every one of the nine is a version posse shipped and is byte-different
	// from what this posse ships, which is the whole point of the fixture.
	for _, n := range names {
		rel := "agents/" + n + ".md"
		live := filepath.Join(a.AgentsDir, n+".md")
		have, err := os.ReadFile(live)
		if err != nil {
			t.Fatal(err)
		}
		if !isShippedExample(rel, have) {
			t.Fatalf("fixture: %s is not entered as a release — the fixture is wrong, not the code", rel)
		}
		shipped, err := fs.ReadFile(posse.Seed, rel)
		if err != nil {
			t.Fatal(err)
		}
		if string(have) == string(shipped) {
			t.Fatalf("fixture: %s is byte-identical to the current embed — the fixture cannot express version skew", rel)
		}
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	if live := a.ListAgents(); len(live) != 0 {
		t.Fatalf(`ranger-base-8ehw REGRESSED — %d generic(s) still routing after an upgrade from
an older seed: %v. Retirement is judged against the digests posse has shipped
(exampledigests.go), not byte-identity against the running binary; a home
seeded by any earlier posse retires nothing under the latter.
init said:
%s`, len(live), live, out.String())
	}
	if !strings.Contains(out.String(), "retired") {
		t.Errorf("init said %q — a retirement the operator cannot see is a file that vanished", out.String())
	}
	// And nobody is blamed for an edit they did not make. "it is yours now"
	// is the wording init keeps a file under today; assert the phrase, not a
	// string that no longer exists, or this stops pinning anything.
	if strings.Contains(out.String(), "it is yours now") {
		t.Errorf(`init said %q — version skew is not an operator edit, and saying so sends
them looking for a change they never made`, out.String())
	}
	// A move, not a delete: what left agents/ is readable on the shelf. On a
	// skewed home the live bytes are the OLDER release's and exist nowhere
	// else in the home, so the shelf must hold them and not the embed's.
	for _, n := range names {
		shelf, err := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), n+".md"))
		if err != nil {
			t.Fatalf("%s left the home entirely: %v", n, err)
		}
		was, err := fs.ReadFile(src, "agents/"+n+".md")
		if err != nil {
			t.Fatal(err)
		}
		if string(shelf) != string(was) {
			t.Errorf("shelf copy of %s is not what was retired — the retired bytes are gone from the home", n)
		}
	}
}

// ranger-base-8ehw, the sharp edge the version-skew rule brings with it and
// the guarantee that blunts it.
//
// Once an older release's bytes can be retired, what leaves agents/ need not
// be what the shelf already holds: on a skewed home the live file is the only
// copy of those bytes anywhere in the home. So the retirement is a rename,
// never a remove — whatever leaves agents/ is still readable on the shelf
// under the name init printed. Under the old rule the live bytes were always
// byte-identical to the embed and os.Remove lost nothing.
//
// The fixture is the case that made that non-negotiable: a generic the
// operator adopted in place on a home that predates ADR 0015, which a
// post-hoc `seeded` manifest then attested to as if init had laid it down.
// That file is KEPT now (ranger-base-rgx0 — identity comes from the digests
// posse has shipped, and the home's own manifest is no longer evidence about
// who wrote a file). This pin does not care which way that decision goes: it
// asserts only that the bytes are still in the home afterwards, so it keeps
// holding if the decision ever moves again.
func TestRetirementNeverDestroysTheBytesItTakes(t *testing.T) {
	t.Parallel()
	a := preFixHome(t, posse.Seed)
	names := exampleAgentNames(posse.Seed)
	mine := names[0]
	live := filepath.Join(a.AgentsDir, mine+".md")
	b, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	adopted := string(b) + "\n<!-- the operator adopted this one before this home had a manifest -->\n"
	if err := os.WriteFile(live, []byte(adopted), 0o644); err != nil {
		t.Fatal(err)
	}
	// ...and only THEN does a manifest-writing posse first run init here.
	if err := os.Remove(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	// Whether it was kept or retired, the bytes are still in the home.
	found := ""
	for _, p := range []string{live, filepath.Join(a.ExampleAgentsDir(), mine+".md")} {
		if got, err := os.ReadFile(p); err == nil && string(got) == adopted {
			found = p
		}
	}
	if found == "" {
		t.Fatalf(`ranger-base-8ehw REGRESSED — %s.md is gone from the home. A seeded manifest
can attest to an operator's own file (it hashes whatever was on disk the first
time any manifest-writing posse ran init here), so the retirement must rename
onto the shelf, never os.Remove.
init said:
%s`, mine, out.String())
	}
}

// DEFECT 2 (ranger-base-8b3h), FIXED by ranger-base-9afo. When a retirement
// happened on a `seeded` home, init used to re-stamp the manifest with
// HashPromotedSet(Home) — the WHOLE promoted set, not just the paths it
// actually changed — which silently re-anchored any drift already sitting in
// config.yaml, recipes/ or skills/ and let the ADR 0015 §3 launch verify come
// back clean over it. The same defect lived a second place in this file: the
// "repaired" re-stamp in initFrom (fired when a gap-filling init writes a
// missing seed file) did the identical whole-tree rehash, and reproduced the
// same blessing on its own even after retireExamplePIDs was narrowed — a
// single init call exercises both, and the pin here stayed green against the
// first fix alone until the second site was narrowed too.
//
// Both sites now touch only the paths this run actually wrote or moved:
// retireExamplePIDs drops the retired agents/<name>.md entries, and the
// initFrom "repaired" branch re-hashes only the promoted files copyIfMissing
// wrote (tracked as touchedPromoted, which excludes envs/ and the example
// shelf — copied through the same helper but neither promoted nor part of
// what the seeded manifest attests to).
func TestRetirementRestampBlessesUnrelatedConstitutionDrift(t *testing.T) {
	t.Parallel()
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
	if after.Files[rel] != anchored {
		t.Fatalf(`ranger-base-9afo REGRESSED — %s is no longer anchored at the pre-drift hash after
an upgrade init that retired the nine. The re-stamp must touch only the paths
this run actually changed (the retired agents/ entries, and whatever
copyIfMissing wrote), never re-hash the whole promoted set — that re-anchors
unrelated drift and lets the ADR 0015 §3 launch verify come back clean over
it. got %q, want the pre-drift hash %q`, rel, after.Files[rel], anchored)
	}
	if v := a.VerifyPromoted(); v.OK() {
		t.Fatalf("ranger-base-9afo REGRESSED — the drift in %s is no longer refused by the launch verify: %s", rel, v.Line())
	}
}
