package posse

// ranger-base-e6y. The seed source used to be chosen on the NAME of a
// directory beside the binary, so `go install`'s ~/go/examples, or any
// project with bin/ and examples/, could hand init a tree that was not a
// seed. What made it P-worthy was the silence at both ends: the wrong source
// was announced as one word on the success line, and copyDir swallowed the
// read error for every root, so the instance came up crewless at exit 0 and a
// second init repaired nothing.
//
// initseed_qa_test.go pins the headline case. This file pins the shape of the
// rule around it: which directories are seeds, what a non-seed source does
// when it reaches initFrom anyway, and that a home already half-seeded by an
// older binary comes back whole — manifest included.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// writeSeedDir writes a directory that answers everything init asks of a source.
func writeSeedDir(t *testing.T, dir string) string {
	t.Helper()
	for _, r := range []string{"agents", "recipes", "envs"} {
		if err := os.MkdirAll(filepath.Join(dir, r), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("# seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSeedOverrideIsTakenOnlyWhenTheDirectoryIsASeedTree(t *testing.T) {
	t.Parallel()
	// Each case is a real shape somebody has on disk, not a permutation for
	// its own sake: an empty examples/ (the loud half of the bug — exit 1
	// with a half-made home), a project's own examples/ holding anything at
	// all, a seed missing one root, and the dev checkout.
	cases := []struct {
		name     string
		build    func(t *testing.T, dir string)
		override bool
	}{
		{"absent", func(*testing.T, string) {}, false},
		{"empty", func(t *testing.T, dir string) { mkdirT(t, dir) }, false},
		{"foreign files only", func(t *testing.T, dir string) {
			mkdirT(t, dir)
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("default_dir: ~\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"a seed missing envs/", func(t *testing.T, dir string) {
			writeSeedDir(t, dir)
			if err := os.RemoveAll(filepath.Join(dir, "envs")); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"config.yaml is a directory", func(t *testing.T, dir string) {
			writeSeedDir(t, dir)
			if err := os.RemoveAll(filepath.Join(dir, "config.yaml")); err != nil {
				t.Fatal(err)
			}
			mkdirT(t, filepath.Join(dir, "config.yaml"))
		}, false},
		{"the dev checkout", func(t *testing.T, dir string) { writeSeedDir(t, dir) }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			bin := filepath.Join(tmp, "bin")
			mkdirT(t, bin)
			tc.build(t, filepath.Join(tmp, "examples"))

			_, from := seedSource(bin)
			if got := from != "embedded"; got != tc.override {
				t.Fatalf("seedSource chose %q — override taken=%v, want %v", from, got, tc.override)
			}
		})
	}
}

func mkdirT(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

// The other end of the same silence: whatever the choosing rule is, a source
// that cannot supply a root must not come back as success. The error has to
// name the source and the root, because the operator's next question is
// which directory init was reading — the one thing the old exit-0 line never
// said.
func TestInitRefusesASourceMissingASeedRootAndNamesIt(t *testing.T) {
	t.Parallel()
	foreign := t.TempDir()
	if err := os.WriteFile(filepath.Join(foreign, "config.yaml"), []byte("default_dir: ~\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := initTestApp(t)
	err := a.initFrom(io.Discard, os.DirFS(foreign), foreign)
	if err == nil {
		t.Fatalf("init from %s succeeded with %d persona(s) — a source with no recipes/ is not a seed", foreign, len(a.ListAgents()))
	}
	for _, want := range []string{foreign, "recipes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

// A home an older binary half-seeded is still out there, and re-running init
// is the advertised upgrade path (INSTALL.md §7). It must come back whole —
// and its manifest with it. A seeded manifest is a hash of what init laid
// down (ADR 0015 §3), so filling the gap without re-stamping would leave
// every dispatched launch refusing over the files the repair just restored:
// a repair that trades a crewless home for a home that cannot launch.
func TestInitRepairsAHalfSeededHomeAndRestampsItsSeededManifest(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	// Exactly what the bug left behind: config.yaml from a foreign examples/,
	// the five roots, and a seeded manifest over the one file.
	for _, d := range []string{a.Home, a.RecipesDir, a.EnvsDir, a.SecretsDir, a.StateDir, a.AgentsDir, a.SkillsDir(), a.ExampleAgentsDir()} {
		mkdirT(t, d)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: ~\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("the fixture does not start verifiable: %+v", v)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("re-init on a half-seeded home: %v\n%s", err, out.String())
	}

	shelf := &App{Home: a.Home, AgentsDir: a.ExampleAgentsDir()}
	if names := shelf.ListAgents(); len(names) < 9 {
		t.Errorf("the shelf holds %d example PID(s) (%v) — the second init did not repair the home", len(names), names)
	}
	for _, dir := range []string{a.RecipesDir, a.EnvsDir} {
		if ents, _ := os.ReadDir(dir); len(ents) == 0 {
			t.Errorf("%s is still empty after the repair", dir)
		}
	}
	// The operator's own file is still theirs: repair fills gaps, it does not
	// overwrite.
	if b, _ := os.ReadFile(a.ConfigPath); string(b) != "default_dir: ~\n" {
		t.Errorf("the repair overwrote config.yaml: %q", b)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("the repaired home no longer matches its seeded manifest — every dispatched launch refuses here: %+v", v)
	}
	if s := out.String(); !strings.Contains(s, "promoted.json") {
		t.Errorf("init re-stamped the manifest and did not say so:\n%s", s)
	}
}
