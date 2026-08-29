package rhq

// CmdInit seeds RHQ_HOME from the example instance, never overwriting.
//
// The seed tree has two possible sources (ADR 0012 D5). examples/ beside the
// binary wins when it is there: that is a dev build run out of the checkout,
// where an edit to examples/ must take effect without a rebuild. Otherwise
// the copy embedded at build time is used — which is the whole point of the
// embed, because a release binary on a fresh laptop has no repo to read.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ranger360ai/posse"
)

// seedSource resolves the seed tree for a binary living in exeDir, and names
// where it came from — worth printing, because "embedded" and "a directory
// you can edit" behave differently on the next run.
func seedSource(exeDir string) (fs.FS, string) {
	if exeDir != "" {
		dir := filepath.Join(exeDir, "..", "examples")
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return os.DirFS(dir), dir
		}
	}
	return posse.Seed, "embedded"
}

func (a *App) CmdInit(w io.Writer) error {
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		exeDir = filepath.Dir(exe)
	}
	src, from := seedSource(exeDir)
	return a.initFrom(w, src, from)
}

// initFrom copies src into RHQ_HOME. Paths inside src are slash-separated
// (io/fs), paths under Home are the platform's — hence path vs filepath.
func (a *App) initFrom(w io.Writer, src fs.FS, from string) error {
	// Both facts are about the home init FOUND, and init writes into the
	// promoted set itself — so they are only knowable here, before the first
	// copy. What they decide is the manifest stamp at the bottom
	// (ranger-base-h7cd).
	before, err := HashPromotedSet(a.Home)
	if err != nil {
		return err
	}
	man, manErr := ReadPromoteManifest(a.PromoteManifestPath())
	fresh := manErr == nil && man == nil && len(before) == 0

	for _, d := range []string{a.Home, a.RecipesDir, a.EnvsDir, a.SecretsDir, a.StateDir, a.AgentsDir, a.SkillsDir(), a.ExampleAgentsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	copyIfMissing := func(fromPath, to string, mode os.FileMode) error {
		if _, err := os.Stat(to); err == nil {
			return nil
		}
		b, err := fs.ReadFile(src, fromPath)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, mode)
	}
	if err := copyIfMissing("config.yaml", a.ConfigPath, 0o644); err != nil {
		return err
	}
	copyDir := func(fromDir, toDir string, mode os.FileMode) error {
		ents, _ := fs.ReadDir(src, fromDir)
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			if err := copyIfMissing(path.Join(fromDir, e.Name()), filepath.Join(toDir, e.Name()), mode); err != nil {
				return err
			}
		}
		return nil
	}
	// A skill is a directory (SKILL.md plus references/), so skills is the
	// one seed root that must be walked rather than listed. The shipped seed
	// carries examples/skills/distributed-systems (ADR 0012 D2, the generic
	// canon); a seed dir that has no skills/ at all seeds nothing, quietly.
	copyTree := func(fromDir, toDir string, mode os.FileMode) error {
		if st, err := fs.Stat(src, fromDir); err != nil || !st.IsDir() {
			return nil
		}
		return fs.WalkDir(src, fromDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(strings.TrimPrefix(p, fromDir), "/")
			if rel == "" {
				return nil
			}
			dst := filepath.Join(toDir, filepath.FromSlash(rel))
			if d.IsDir() {
				return os.MkdirAll(dst, 0o755)
			}
			return copyIfMissing(p, dst, mode)
		})
	}
	if err := copyDir("recipes", a.RecipesDir, 0o644); err != nil {
		return err
	}
	if err := copyDir("envs", a.EnvsDir, 0o600); err != nil {
		return err
	}
	// The example PIDs go to the shelf, NOT to agents/ (ranger-base-qajs).
	// Seeding them as live personas made every one of them a lane: label
	// routing walks the roster in name order, so `architect`, `business-
	// manager`, `developer` and `devops` each sorted ahead of the persona
	// the operator had actually written for that lane and silently took
	// every unassigned bead in it — measured on one crew as 14 lifetime
	// closes for the seeded `developer` and 8 open beads parked on generics
	// nobody staffed. An example is a thing you copy from; it is not a hire.
	if err := copyDir("agents", a.ExampleAgentsDir(), 0o644); err != nil {
		return err
	}
	if err := copyTree("skills", a.SkillsDir(), 0o644); err != nil {
		return err
	}
	// Env sets hold secrets: keep them out of reach of other local users.
	os.Chmod(a.EnvsDir, 0o700)
	ents, _ := os.ReadDir(a.EnvsDir)
	for _, e := range ents {
		if !e.IsDir() {
			os.Chmod(filepath.Join(a.EnvsDir, e.Name()), 0o600)
		}
	}
	// secrets/ is seeded EMPTY, and that is the whole seeding (ADR 0019 D1):
	// the directory is the class split made real, so a harness credential has
	// somewhere to live that no PID key can name. There is no seed file and
	// there is deliberately no plan-guard.env — the plan guard is not a
	// consumer (P1 measured 403 for every mintable form; the meter token
	// stays the runtime's own). No copyDir here on purpose: a seed tree that
	// grew a secrets/ directory would be shipping a credential file in the
	// binary.
	os.Chmod(a.SecretsDir, 0o700)
	// Existing installs: the generics are already in agents/, and shipping
	// a new binary that merely stops seeding them leaves every one of them
	// routing. Retire them here, on the terms below.
	if err := a.retireExamplePIDs(w, src); err != nil {
		return err
	}
	// ADR 0015 §3: a home this init actually SEEDED gets a manifest too, so
	// the launch verify has a true anchor from the first launch on a clean
	// box instead of firing on an install nobody promoted. Marked `seeded` —
	// a real manifest with no commit behind it (promote.go).
	//
	// A home that already had a constitution gets none, and that half is
	// ranger-base-h7cd. Such a home was launching fine unmanifested — no
	// manifest is what VerifyPromoted reads as "nothing was promoted here",
	// and it is what keeps every install predating ADR 0015 running. Stamping
	// one ARMS the verify over files nobody ratified, and nothing fails at
	// init time: the operator's next ordinary edit to config.yaml or a PID is
	// what turns every DISPATCHED launch into a hard refusal, hours later,
	// with nothing connecting it to the init that caused it. Arming §3 is
	// what `posse promote` IS — a ratification — and init does not perform
	// one on the operator's behalf. Measured on the live home before the fix:
	// one `posse init` stamped 11 personas, config.yaml, 10 recipes and the
	// skills tree, and said nothing. Re-running init on an existing instance
	// is an advertised upgrade path (INSTALL.md §7), which is how the
	// unattended fleet reached it.
	if fresh {
		if err := a.SeedPromoteManifest(); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "initialized %s (seed: %s)\n", a.Home, from)
	// Either way it is said out loud: an armed launch verify and an unarmed
	// one are the difference between a dispatched launch refusing and not,
	// and an operator who cannot tell which one they have finds out from the
	// fleet.
	switch {
	case fresh:
		fmt.Fprintf(w, "stamped %s (seeded): every launch now hashes agents/, config.yaml, recipes/ and skills/ against it — a dispatched launch refuses on a mismatch, an interactive one warns (ADR 0015 §3)\n",
			AbbrevHome(a.PromoteManifestPath()))
		fmt.Fprintf(w, "  `posse promote` is what re-stamps it after you change any of them\n")
	case manErr == nil && man == nil:
		fmt.Fprintf(w, "left this home unstamped: it already had a constitution, and a manifest init wrote over it would arm the launch verify on prose nobody ratified (ADR 0015 §3)\n")
		fmt.Fprintf(w, "  the verify stays off until you run `posse promote`; until then no launch is refused for it\n")
	}
	// A fresh instance has no crew, and that is the shipped state, not a
	// half-seed: say where the reference PIDs are and how to get a real
	// one, or the next command an operator runs is a dispatch pass that
	// routes nothing and does not say why.
	if len(a.ListAgents()) == 0 {
		if n := len(exampleAgentNames(src)); n > 0 {
			fmt.Fprintf(w, "no personas installed — %s holds %d example PID(s) to copy from; `posse agent new <name>` scaffolds one\n",
				AbbrevHome(a.ExampleAgentsDir()), n)
		}
	}
	return nil
}

// exampleAgentNames is the set of persona names the seed ships as examples
// — the only files retireExamplePIDs will ever consider moving.
func exampleAgentNames(src fs.FS) []string {
	ents, _ := fs.ReadDir(src, "agents")
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return out
}

// retireExamplePIDs moves agents/<name>.md onto the shelf for every persona
// that IS the shipped example, byte for byte.
//
// The upgrade path is the whole problem here: a home seeded by an older
// binary has the nine generics in agents/, and a build that only stops
// seeding them fixes nothing on any instance that already exists. But
// agents/ is the operator's directory, so the rules are narrow, and each
// one buys back a way this could have made things worse than the bug:
//
//   - IT IS POSSE'S FILE, AND POSSE CAN PROVE IT. An edited example is not
//     an example any more — it is the persona the operator adopted in place,
//     with bd history and an assignee under that name. It stays, and init
//     names it, because a retirement that took it would leave real work
//     parked on a persona that no longer loads. "It is ours" has two true
//     answers and the running binary's bytes are only one of them
//     (ranger-base-8ehw): the example PIDs are shipped prose and they change
//     — 95c4b70 added `- Bash(posse promote:*)` to the deny list of all nine
//     — so a home seeded by any earlier posse holds files that are
//     byte-for-byte what THAT posse shipped and byte-different from this
//     one's. Judged against the embed alone, every such home retires nothing
//     (the whole leak, still open) and init blames the operator for edits
//     they never made. The second answer is the table of digests posse has
//     shipped for each example (exampledigests.go): posse's record of its
//     own releases, so a file the operator wrote can never be in it. The
//     home's own seeded manifest looked like that record and is not — it
//     attests to whatever was on disk when it was first written, upgrades
//     included (ranger-base-rgx0).
//   - NEVER A NAME THE CONFIG DEPENDS ON. `coordinator:`, `default_persona:`
//     and `verify_assignee:` each turn a persona name into behaviour; a
//     home that retired one of them would come up with an unresolvable
//     coordinator or a fallback lane that is not there.
//   - NEVER UNDER A REAL PROMOTION. On a home `posse promote` manages, the
//     agents/ tree is a copy of a commit and the manifest is a claim about
//     it (ADR 0015 §3) — moving a file out from under that turns the next
//     launch's verify into a MISSING, which refuses dispatch. The fix
//     there belongs in the constitution repo, and init says so instead of
//     doing it. A `seeded` manifest has no commit behind it, so init
//     re-stamps it and the home stays verifiable.
//
// It is a move, not a delete: the file lands on the shelf beside the other
// examples, so an operator who wanted that generic can copy it straight
// back. That has to be a real rename now rather than a remove-because-the-
// shelf-already-matches: once an older version's bytes can be retired, the
// live file is the only copy of them in the home.
func (a *App) retireExamplePIDs(w io.Writer, src fs.FS) error {
	names := exampleAgentNames(src)
	if len(names) == 0 {
		return nil
	}
	// Read the manifest first: on a promoted home nothing moves at all.
	man, manErr := ReadPromoteManifest(a.PromoteManifestPath())
	pinned := map[string]bool{}
	for _, key := range []string{"coordinator", "default_persona", "verify_assignee"} {
		if v := strings.TrimSpace(a.CfgGet(key, "")); v != "" {
			pinned[strings.ToLower(v)] = true
		}
	}
	var retired, kept []string
	for _, name := range names {
		rel := path.Join("agents", name+".md")
		live := filepath.Join(a.AgentsDir, name+".md")
		have, err := os.ReadFile(live)
		if err != nil {
			continue // not installed here — nothing to retire
		}
		want, wantErr := fs.ReadFile(src, rel)
		shipped := wantErr == nil && string(have) == string(want)
		// The other answer: the table of every digest posse has shipped for
		// this example (exampledigests.go), which recognises an older
		// release's bytes that the embed cannot. It is posse's own record and
		// not the home's, so nothing the operator wrote can enter it — a
		// `seeded` manifest cannot say that much, because it hashes whatever
		// was on disk the first time a manifest-writing posse ran init here,
		// an adopted generic included (ranger-base-rgx0).
		if !shipped && !isShippedExample(rel, have) {
			kept = append(kept, name+".md (differs from every example PID posse has shipped — it is yours now)")
			continue
		}
		if pinned[strings.ToLower(name)] {
			kept = append(kept, name+".md (named in config.yaml)")
			continue
		}
		if manErr != nil {
			kept = append(kept, name+".md (promoted.json is unreadable — fix it first)")
			continue
		}
		if man != nil && !man.Seeded {
			kept = append(kept, name+".md (this home is promoted — retire it in the constitution repo, then `posse promote`)")
			continue
		}
		// The shelf slot must still be the example copyIfMissing wrote
		// above; if the operator edited the shelf, theirs wins and nothing
		// moves onto it.
		shelf := filepath.Join(a.ExampleAgentsDir(), name+".md")
		if b, err := os.ReadFile(shelf); err != nil || string(b) != string(want) {
			kept = append(kept, name+".md (the shelf copy differs — not overwriting it)")
			continue
		}
		// Rename, not remove. When the live file is an older release's
		// example its bytes exist nowhere else in the home, and the shelf
		// slot it replaces is the one thing here init can always lay down
		// again. Whatever leaves agents/ is still readable, under the name
		// init just printed.
		if err := os.Rename(live, shelf); err != nil {
			return err
		}
		retired = append(retired, name)
	}
	if len(retired) > 0 {
		fmt.Fprintf(w, "retired %d example PID(s) to %s — they were shipped as examples and were taking beads in label routing: %s\n",
			len(retired), AbbrevHome(a.ExampleAgentsDir()), strings.Join(retired, ", "))
		fmt.Fprintf(w, "  work parked on them is not reassigned: check `bd list --assignee <name>` in each repo you dispatch from\n")
		// A seeded manifest is a hash of what init laid down; init just
		// changed that, so it re-stamps rather than leaving the next launch
		// to report the files it removed on purpose as MISSING.
		if man != nil && man.Seeded {
			files, err := HashPromotedSet(a.Home)
			if err != nil {
				return err
			}
			man.Files = files
			if err := man.write(a.PromoteManifestPath()); err != nil {
				return err
			}
		}
	}
	for _, k := range kept {
		fmt.Fprintf(w, "kept agents/%s\n", k)
	}
	return nil
}
