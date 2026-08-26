package rhq

// `posse init` must work from a bare release binary — no repo beside it (ADR
// 0012 D5: the release binary carries examples/, or a work laptop cannot
// seed an instance at all). The failure this file pins is silent in the
// worst way: the embed compiles, init runs, and the instance comes up
// missing whichever seed root nobody thought to copy.

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

func initTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("RHQ_HOME", filepath.Join(t.TempDir(), "home"))
	return NewApp()
}

// The embed is a directory pattern, so it cannot drift file-by-file — but it
// can be pointed at the wrong root, or lose a subtree to an exclusion rule.
// Byte-for-byte against the checkout is the cheap total assertion.
func TestEmbeddedSeedMatchesExamplesDir(t *testing.T) {
	root := filepath.Join("..", "..", "examples")
	n := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		want, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		got, err := fs.ReadFile(posse.Seed, filepath.ToSlash(rel))
		if err != nil {
			t.Errorf("examples/%s is on disk but not in the binary: %v", rel, err)
			return nil
		}
		if string(got) != string(want) {
			t.Errorf("examples/%s: embedded copy differs from the checkout", rel)
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 10 {
		t.Fatalf("walked only %d files under examples/ — the seed is bigger than that", n)
	}
}

// The dev override, both ways round: a binary in a checkout reads the live
// examples/ (edit-and-run, no rebuild); a binary anywhere else reads itself.
func TestSeedSourcePrefersExamplesBesideBinary(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(filepath.Join(tmp, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "examples", "config.yaml"), []byte("# on-disk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	src, from := seedSource(bin)
	if from == "embedded" {
		t.Fatalf("seedSource(%s) = embedded — examples/ beside the binary must win", bin)
	}
	if b, err := fs.ReadFile(src, "config.yaml"); err != nil || string(b) != "# on-disk\n" {
		t.Errorf("read from the on-disk seed: %q, %v", b, err)
	}

	// An empty PATH dir: nothing at ../examples, so the binary is on its own.
	lonely := filepath.Join(t.TempDir(), "bin")
	os.MkdirAll(lonely, 0o755)
	src, from = seedSource(lonely)
	if from != "embedded" {
		t.Fatalf("seedSource(%s) = %q — a binary with no repo must fall back to the embed", lonely, from)
	}
	want, _ := os.ReadFile(filepath.Join("..", "..", "examples", "config.yaml"))
	if b, err := fs.ReadFile(src, "config.yaml"); err != nil || string(b) != string(want) {
		t.Errorf("embedded config.yaml: %v (%d bytes, want %d)", err, len(b), len(want))
	}
}

// The bead's acceptance criterion, hermetically: seed a scratch RHQ_HOME
// from the embed alone and assert the instance is whole.
func TestInitFromEmbeddedSeed(t *testing.T) {
	a := initTestApp(t)
	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "initialized "+a.Home) {
		t.Errorf("init said %q", out.String())
	}

	for _, d := range []string{a.RecipesDir, a.EnvsDir, a.StateDir, a.AgentsDir, a.SkillsDir()} {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			t.Errorf("%s not created: %v", d, err)
		}
	}
	// Every seeded root arrives populated, and config.yaml verbatim.
	seed, _ := os.ReadFile(filepath.Join("..", "..", "examples", "config.yaml"))
	got, err := os.ReadFile(a.ConfigPath)
	if err != nil || string(got) != string(seed) {
		t.Errorf("config.yaml: %v (%d bytes, want %d)", err, len(got), len(seed))
	}
	for dir, want := range map[string]int{a.AgentsDir: 9, a.RecipesDir: 3, a.EnvsDir: 2} {
		ents, _ := os.ReadDir(dir)
		if len(ents) < want {
			t.Errorf("%s: %d files, want at least %d", dir, len(ents), want)
		}
	}
	// examples/skills ships the generic distributed-systems canon (ADR 0012
	// D2), and a skill is a tree: SKILL.md plus references/. Assert it
	// arrives whole from the embed, since the tails-stay-instance-side rule
	// is only worth anything if the canon actually reaches a fresh instance.
	if got := a.ListSkills(); len(got) != 1 || got[0] != "distributed-systems" {
		t.Fatalf("ListSkills after init: %v — want the seeded canon", got)
	}
	skill := a.SkillPath("distributed-systems")
	if b, err := os.ReadFile(filepath.Join(skill, "SKILL.md")); err != nil ||
		!strings.Contains(string(b), "description:") {
		t.Errorf("SKILL.md: %v — a skill without a description binds to nothing on some runtimes", err)
	}
	refs, err := os.ReadDir(filepath.Join(skill, "references"))
	if err != nil {
		t.Fatalf("references/: %v", err)
	}
	gotRefs := map[string]bool{}
	for _, e := range refs {
		gotRefs[e.Name()] = true
	}
	// Names, not just a count: len==7 would still pass if the seven files were
	// renamed out from under SKILL.md's index (ranger-base-tkc).
	wantRefs := []string{
		"db-as-queue.md",
		"delivery-and-idempotency.md",
		"fencing-and-leases.md",
		"liveness-and-identity.md",
		"safe-reclamation.md",
		"single-writer-and-stores.md",
		"toctou.md",
	}
	if len(refs) != len(wantRefs) {
		t.Errorf("references/: %d files %v — want the seven named canon concepts", len(refs), gotRefs)
	}
	for _, n := range wantRefs {
		if !gotRefs[n] {
			t.Errorf("references/: missing %s", n)
		}
	}

	// Env sets hold secrets: 0700 dir, 0600 files, from the embed too.
	if st, _ := os.Stat(a.EnvsDir); st.Mode().Perm() != 0o700 {
		t.Errorf("envs/ mode %v, want 0700", st.Mode().Perm())
	}
	ents, _ := os.ReadDir(a.EnvsDir)
	for _, e := range ents {
		st, _ := os.Stat(filepath.Join(a.EnvsDir, e.Name()))
		if st.Mode().Perm() != 0o600 {
			t.Errorf("envs/%s mode %v, want 0600", e.Name(), st.Mode().Perm())
		}
	}

	// Never overwrites: re-running fills gaps and leaves edits alone.
	os.WriteFile(a.ConfigPath, []byte("# mine\n"), 0o644)
	agents, _ := os.ReadDir(a.AgentsDir)
	gone := filepath.Join(a.AgentsDir, agents[0].Name())
	os.Remove(gone)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(a.ConfigPath); string(b) != "# mine\n" {
		t.Error("init overwrote an edited config.yaml")
	}
	if _, err := os.Stat(gone); err != nil {
		t.Errorf("init did not restore the missing %s: %v", gone, err)
	}
}

// Skills are trees, not files. examples/skills is a later bead, so the
// recursion is pinned against a synthetic seed rather than waiting for it.
func TestInitCopiesSkillTrees(t *testing.T) {
	seed := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(seed, "config.yaml"), []byte("# seed\n"), 0o644))
	must(os.MkdirAll(filepath.Join(seed, "skills", "distributed-systems", "references"), 0o755))
	must(os.WriteFile(filepath.Join(seed, "skills", "distributed-systems", "SKILL.md"), []byte("---\nname: distributed-systems\ndescription: d\n---\n"), 0o644))
	must(os.WriteFile(filepath.Join(seed, "skills", "distributed-systems", "references", "leases.md"), []byte("canon\n"), 0o644))

	a := initTestApp(t)
	if err := a.initFrom(io.Discard, os.DirFS(seed), seed); err != nil {
		t.Fatal(err)
	}
	if got := a.ListSkills(); len(got) != 1 || got[0] != "distributed-systems" {
		t.Fatalf("ListSkills after init: %v", got)
	}
	ref := filepath.Join(a.SkillPath("distributed-systems"), "references", "leases.md")
	if b, err := os.ReadFile(ref); err != nil || string(b) != "canon\n" {
		t.Errorf("%s: %q, %v — nested skill files must come across", ref, b, err)
	}
}
