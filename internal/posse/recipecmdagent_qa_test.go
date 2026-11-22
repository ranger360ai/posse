//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-g27p4 at the recipe boundary: a recipe declaring `command:`
// beside `agent:` must refuse, because posse launched the persona's rendered
// line and threw the declared one away without a word — `posse new --cmd
// --agent`'s silent discard (ranger-base-k2olp) one layer up, and the arm
// that bead filed rather than took.
//
// Asserted over LaunchRecipe and not over the built binary, which is where
// this diverges from cmd/posse/cmdagent_qa_test.go's twin: that refusal's
// only path out is `die`, so the claim "exit 1, nothing created" had to be
// made about a process. This one RETURNS its error, and LaunchRecipe is the
// whole of `posse recipe` — the one entry point whose NewSessionOpts are
// built inside this package (loadguard_test.go's own note) — so the boundary
// and the test are in the same place and the fixture can read the host back.
//
// Every row here is a LOADED value and not a key's presence, which is the
// one thing this check could get wrong while looking right: `command: null`
// is what examples/recipes/codex-projA.yaml prescribes and what
// ~/homes/ranger/recipes/codex-src.yaml already carries, YamlGet reads it as
// unset, and a check written over the key would refuse both.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRecipeFile plants one flat-YAML recipe and returns its path.
func writeRecipeFile(t *testing.T, b *HerdrBackend, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(b.App.RecipesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(b.App.RecipesDir, name+".yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRecipeRefusesCommandBesideAgent(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	writePersona(t, b.App, "dev", "[go]")
	dir := t.TempDir()

	// The live shape, MEASURED 2026-11-22 on this box:
	// ~/homes/ranger/recipes/grok-fam.yaml declares `command: grok` beside
	// `agent: grok`. The persona here is INSTALLED, so what is refused is
	// the pair and not a recipe naming a persona that is not there.
	p := writeRecipeFile(t, b, "qa-pair",
		"name: qa-pair\ndir: "+dir+"\ncommand: claude --dangerously-skip-permissions\nagent: dev\nemoji: 🧪\n")

	var out strings.Builder
	err := b.LaunchRecipe(&out, "qa-pair")
	if err == nil {
		t.Fatalf("a recipe declaring command: beside agent: launched — the declared line is discarded in silence, "+
			"which is what this bead was filed about\nprinted: %s", out.String())
	}
	// The rule and the fix, not just a refusal: the reader is editing a YAML
	// file, so the message has to name the two KEYS, the file to open, the
	// line being thrown away, and the spelling that launches.
	for _, want := range []string{
		"command:", "agent:", "dev", p,
		"claude --dangerously-skip-permissions", // the line that would have been dropped
		"command: null",                         // the fix, in the spelling the seed recipe documents
		"rendered",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q, so it does not state the rule or the fix:\n%v", want, err)
		}
	}
	// Refused BEFORE anything on the host moved: no session, and the
	// launcher's writer never got the `launched recipe` line. A check asked
	// after CreateSession would print it and pass every assertion above.
	if b.HasSession("qa-pair") {
		t.Error("the refused recipe created its session anyway — the check is downstream of the launch")
	}
	if s := out.String(); s != "" {
		t.Errorf("a refused recipe printed to the launcher's writer: %q", s)
	}

	// Order, over a persona that is NOT installed: the pair is refused
	// before posse resolves anything, so the reader is told about their two
	// keys and not about a missing persona. This is the row that fails if
	// the check moves past the `agent:` resolution in planLaunch.
	writeRecipeFile(t, b, "qa-pair-noagent",
		"name: qa-pair-noagent\ndir: "+dir+"\ncommand: grok\nagent: nobody-here\nemoji: 🧪\n")
	err = b.LaunchRecipe(io.Discard, "qa-pair-noagent")
	if err == nil {
		t.Fatal("the pair launched when its persona did not even exist")
	}
	if strings.Contains(err.Error(), "no such agent") {
		t.Errorf("posse went on to resolve the persona, so the pair is refused somewhere downstream of the recipe:\n%v", err)
	}
}

// The documented form stays legal, and it is what makes the test above an
// assertion about the PAIR rather than about any recipe naming a persona.
// `command: null` reads as unset (yamlflat.go), so a refusal written over
// the key's presence reds here — and that mutant is the whole reason this
// row exists, because the form it would break is the one the seed recipe
// prescribes and the one codex-src.yaml on this box already uses.
func TestRecipeWithNullCommandBesideAgentStillLaunches(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	writePersona(t, b.App, "dev", "[go]")
	dir := t.TempDir()

	for _, tc := range []struct{ name, command string }{
		{"qa-null", "command: null\n"}, // the seed recipe's own spelling
		{"qa-tilde", "command: ~\n"},   // the other unset spelling YamlGet reads
		{"qa-absent", ""},              // no key at all
	} {
		writeRecipeFile(t, b, tc.name,
			"name: "+tc.name+"\ndir: "+dir+"\n"+tc.command+"agent: dev\nemoji: 🧪\n")
		if err := b.LaunchRecipe(io.Discard, tc.name); err != nil {
			t.Errorf("%s: a recipe whose command: is unset must launch the persona as it always did: %v", tc.name, err)
			continue
		}
		if !b.HasSession(tc.name) {
			t.Errorf("%s: no session behind the recipe", tc.name)
		}
	}

	// And the other half of the surface: a `command:` with no `agent:` is a
	// plain pane launch and is untouched.
	writeRecipeFile(t, b, "qa-plain", "name: qa-plain\ndir: "+dir+"\ncommand: claude\nemoji: 🧪\n")
	if err := b.LaunchRecipe(io.Discard, "qa-plain"); err != nil {
		t.Errorf("a recipe with command: and no agent: must launch: %v", err)
	}
}

// The refusal is asked at LAUNCH and not at LOAD, and that is a decision
// with a cost either way (CheckRecipeCmdAgent's comment): `posse recipes`
// prints every recipe it can load and DROPS the ones that error
// (`if r, err := a.LoadRecipe(n); err == nil`, cmd/posse/main.go), so a
// refusal moved into LoadRecipe would make a recipe carrying the pair vanish
// from the listing — the same silent discard, traded for the one this bead
// fixes. This row holds the trade: the file still loads, with both values
// read back, so the operator can still SEE the recipe they have to fix.
func TestRecipeCarryingThePairStillLoadsAndLists(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	writeRecipeFile(t, b, "qa-listed",
		"name: qa-listed\npurpose: grok\ndir: /tmp\ncommand: grok\nagent: grok\nemoji: 🧪\n")

	r, err := b.App.LoadRecipe("qa-listed")
	if err != nil {
		t.Fatalf("loading a recipe that carries the pair must still work, or `posse recipes` stops listing it: %v", err)
	}
	if r.Command != "grok" || r.Agent != "grok" {
		t.Errorf("LoadRecipe must read both declared values back (the refusal reads them, it does not hide them): %+v", r)
	}
	var found bool
	for _, n := range b.App.ListRecipes() {
		if n == "qa-listed" {
			found = true
		}
	}
	if !found {
		t.Errorf("`posse recipes` no longer lists a recipe carrying the pair: %v", b.App.ListRecipes())
	}
}
