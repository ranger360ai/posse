package posse

// CmdInit seeds RHQ_HOME from the example instance, never overwriting.
//
// The seed tree has two possible sources (ADR 0012 D5). examples/ beside the
// binary wins when it IS one: that is a dev build run out of the checkout,
// where an edit to examples/ must take effect without a rebuild. Otherwise
// the copy embedded at build time is used — which is the whole point of the
// embed, because a release binary on a fresh laptop has no repo to read.
//
// "when it is one" is load-bearing and used not to be (ranger-base-e6y): the
// arm was a bare stat on the name, so any directory called examples/ beside
// the binary was seeded from. bin/ beside examples/ is an ordinary repo
// shape and `go install` puts the binary in ~/go/bin, which makes the
// consulted directory ~/go/examples — a path no install doc names. A
// stranger's directory then won over the embed and laid down a home with no
// crew at exit 0, which a second init could not repair because nothing here
// ever overwrites.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ranger360ai/posse"
)

// seedRoots are the directories a seed tree has because init copies them.
// skills/ is deliberately not among them: a seed with no skills/ seeds none,
// quietly and on purpose (copyTree below).
var seedRoots = []string{"agents", "recipes", "envs"}

// looksLikeSeed asks whether a directory IS a seed tree rather than merely
// being named like one. The test is exactly what init requires of a source —
// config.yaml, and the three roots it copies — so nothing that passes it can
// half-seed, and nothing that fails it is a seed anybody meant.
func looksLikeSeed(src fs.FS) bool {
	if st, err := fs.Stat(src, "config.yaml"); err != nil || st.IsDir() {
		return false
	}
	for _, r := range seedRoots {
		if st, err := fs.Stat(src, r); err != nil || !st.IsDir() {
			return false
		}
	}
	return true
}

// seedOverrideDir is the directory the override arm considers, or "" when
// there is nothing there. Separate from the choice below so CmdInit can say
// which directory it looked at and passed over.
func seedOverrideDir(exeDir string) string {
	if exeDir == "" {
		return ""
	}
	dir := filepath.Join(exeDir, "..", "examples")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return dir
}

// seedSource resolves the seed tree for a binary living in exeDir, and names
// where it came from — worth printing, because "embedded" and "a directory
// you can edit" behave differently on the next run.
func seedSource(exeDir string) (fs.FS, string) {
	if dir := seedOverrideDir(exeDir); dir != "" {
		if src := os.DirFS(dir); looksLikeSeed(src) {
			return src, dir
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
	// A directory considered and passed over is said out loud. Otherwise the
	// only witness to the choice is one word at the end of the success line,
	// and an operator whose ~/go/examples was skipped has no way to learn
	// that it was ever consulted.
	if dir := seedOverrideDir(exeDir); dir != "" && from == "embedded" {
		fmt.Fprintf(w, "ignored %s: not a seed tree (a seed has config.yaml and %s/) — seeding from the copy embedded in this binary\n",
			dir, strings.Join(seedRoots, "/, "))
	}
	return a.initFrom(w, src, from)
}

// initFrom copies src into RHQ_HOME. Paths inside src are slash-separated
// (io/fs), paths under Home are the platform's — hence path vs filepath.
func (a *App) initFrom(w io.Writer, src fs.FS, from string) error {
	// ADR 0031 §2: init joins the operator fence, keyed on the target home
	// rather than blanket-refusing under EnvPersona (the promote/refresh
	// shape) — a persona's throwaway `RHQ_HOME=<scratch> posse init` is how
	// QA seeds fixtures and how the leak this ADR fixes was itself measured,
	// and only the target home decides whether a write is harmful. §3: no
	// PID deny line — the L1 shim sees only argv, never what RHQ_HOME
	// resolves to, so it cannot express "this target, not that one".
	if os.Getenv(EnvPersona) != "" {
		origin := os.Getenv(EnvLaunchHome)
		if origin == "" {
			// Fail closed: a session that cannot prove where it came from
			// does not get to write anywhere it might have come from. Only
			// reachable in the window between promoting a post-0031 binary
			// and a session's next relaunch (a pre-0031 launcher never
			// stamped EnvLaunchHome).
			return Die("posse init refuses to run (ADR 0031 §2): %s is set but %s is not, so this session cannot prove it wasn't launched from the home it's about to write — relaunch the session (a post-0031 launcher stamps %s), or ask the operator to run init themselves\n  to seed a scratch home from here instead: RHQ_HOME=<scratch> posse init",
				EnvPersona, EnvLaunchHome, EnvLaunchHome)
		}
		// underDir resolves the longest existing prefix of each side through
		// symlinks (a throwaway target usually does not exist yet; the
		// pre-cutover home is a symlink onto the instance repo, ADR 0015
		// §2) before comparing cleaned paths, so this catches both the
		// exact-match case and a target nested inside the origin.
		if underDir(origin, a.Home) {
			return Die("posse init refuses to write the home it was launched from (ADR 0031 §2): %s resolves inside %s\n  to seed a scratch home instead: RHQ_HOME=<scratch> posse init",
				a.Home, origin)
		}
	}
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

	// A PROMOTED home is not init's to write (ranger-base-39jnl). ADR 0031
	// §2's fence keys on the persona marker and so covers only sessions; the
	// operator typing `posse init` by hand walks straight past it, and on
	// 2026-09-02 that is what happened — the work-laptop install steps run
	// on the fleet box, which re-seeded examples/ and secrets/ under a
	// constitution `posse promote` owns.
	//
	// The line is `seeded`, not "has a manifest", and it is drawn there on
	// purpose. A SEEDED manifest is a fresh install's anchor with no commit
	// behind it, and re-running init on one is the generics upgrade
	// INSTALL.md §7 advertises: it fills gaps and re-stamps for exactly what
	// it wrote (`repaired` below). A PROMOTED manifest is a claim about a
	// commit that only `posse promote` may restate — so init has no re-stamp
	// available to it there and cannot leave the home consistent even when
	// it succeeds, which is the whole of ranger-base-pith. Refusing is the
	// answer pith could not reach from inside init: the copy that breaks the
	// launch verify simply does not happen.
	//
	// An UNREADABLE manifest keeps its old behaviour and is not covered
	// here: posse cannot say whether that home was promoted or seeded, and
	// the arm on the way out already names it.
	if manErr == nil && man != nil && !man.Seeded {
		return Die("posse init refuses to write %s: it carries a promoted constitution (%s, promoted %s%s)\n"+
			"  `posse promote` owns every path init would copy here, and only promote may re-stamp the manifest — an init that adds a recipe or a skill leaves the launch verify refusing every dispatched launch (ADR 0015 §3, ranger-base-pith)\n"+
			"  to update this home: posse promote <constitution dir>\n"+
			"  to see what a seed would lay down: RHQ_HOME=<scratch> posse init",
			AbbrevHome(a.Home), PromoteManifestFile, man.PromotedAt, promotedFrom(man))
	}

	for _, d := range []string{a.Home, a.RecipesDir, a.EnvsDir, a.SecretsDir, a.StateDir, a.AgentsDir, a.SkillsDir(), a.ExampleAgentsDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	// What this run actually laid down. A seeded manifest is a hash of what
	// init wrote (ADR 0015 §3), so a later init that fills a gap has to
	// re-stamp it or leave the next dispatched launch refusing over the very
	// files it just repaired.
	wrote := 0
	// touchedPromoted is the subset of what init wrote that actually falls
	// inside PromotedPaths — envs/ and the example shelf are copied through
	// the same helper but are neither promoted nor part of what the seeded
	// manifest attests to. Narrowing the "repaired" re-stamp below to just
	// these paths keeps it from re-anchoring drift already sitting in a
	// promoted file this run never touched (ranger-base-9afo).
	var touchedPromoted []string
	promotedRel := func(to string) (string, bool) {
		rel, err := filepath.Rel(a.Home, to)
		if err != nil {
			return "", false
		}
		rel = filepath.ToSlash(rel)
		for _, base := range PromotedPaths {
			if rel == base || strings.HasPrefix(rel, base+"/") {
				return rel, true
			}
		}
		return "", false
	}
	copyIfMissing := func(fromPath, to string, mode os.FileMode) error {
		if _, err := os.Stat(to); err == nil {
			return nil
		}
		b, err := fs.ReadFile(src, fromPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, b, mode); err != nil {
			return err
		}
		wrote++
		if rel, ok := promotedRel(to); ok {
			touchedPromoted = append(touchedPromoted, rel)
		}
		return nil
	}
	if err := copyIfMissing("config.yaml", a.ConfigPath, 0o644); err != nil {
		return err
	}
	// Not swallowed, unlike copyTree's skills/: agents/, recipes/ and envs/
	// are never legitimately absent from a seed, so a read that fails here
	// says the source is not one — and the swallow turned that into an
	// instance missing a root at exit 0 (ranger-base-e6y).
	copyDir := func(fromDir, toDir string, mode os.FileMode) error {
		ents, err := fs.ReadDir(src, fromDir)
		if err != nil {
			return fmt.Errorf("seed %s is missing %s/, which every seed tree has: %w", from, fromDir, err)
		}
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
	// A home that was already seeded and has just had a gap filled: the
	// manifest describes a set this run changed, and re-stamping it is the
	// same rule retireExamplePIDs follows a few lines up. A promoted home
	// (not seeded) is never re-stamped here — there the manifest is a claim
	// about a commit, and only `posse promote` may restate it.
	repaired := !fresh && wrote > 0 && man != nil && man.Seeded
	if repaired {
		for _, rel := range touchedPromoted {
			sum, err := sha256File(filepath.Join(a.Home, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			man.Files[rel] = sum
		}
		// This binary computed those hashes, walking ITS PromotedPaths — so
		// it is this binary the manifest now attests for (ranger-base-39jnl,
		// promote.go stampWriter).
		man.stampWriter()
		if err := man.write(a.PromoteManifestPath()); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "initialized %s (seed: %s)\n", a.Home, from)
	// Either way it is said out loud: an armed launch verify and an unarmed
	// one are the difference between a dispatched launch refusing and not,
	// and an operator who cannot tell which one they have finds out from the
	// fleet.
	switch {
	case repaired:
		fmt.Fprintf(w, "filled %d missing seed file(s) and re-stamped %s (seeded): the manifest follows what init lays down, so the launch verify matches the repaired home (ADR 0015 §3)\n",
			wrote, AbbrevHome(a.PromoteManifestPath()))
	case fresh:
		// The set is READ from PromotedPaths, not spelled here: this is the
		// sentence that tells a new operator what the launch verify covers,
		// and it named four of five for the whole life of `runtimes` in the
		// set (ranger-base-b22vq).
		fmt.Fprintf(w, "stamped %s (seeded): every launch now hashes %s against it — a dispatched launch refuses on a mismatch, an interactive one warns (ADR 0015 §3)\n",
			AbbrevHome(a.PromoteManifestPath()), PromotedProse("and"))
		fmt.Fprintf(w, "  `posse promote` is what re-stamps it after you change any of them\n")
	case !fresh && wrote == 0 && man != nil && man.Seeded:
		// ranger-base-g4cm: a seeded home whose re-run fills no gap matched
		// none of the arms above, so init said nothing here too — and
		// INSTALL.md §14's row read that silence as a promoted home, wrongly.
		// Naming the case removes the ambiguity at the source instead of
		// asking the row to infer it from an absence.
		fmt.Fprintf(w, "nothing missing: %s was already fully seeded, so this run copied no files and left the manifest (seeded) as it was — a file an earlier seed already wrote under the right name keeps that seed's content, not this one's (copyIfMissing never overwrites)\n",
			AbbrevHome(a.Home))
	case manErr == nil && man == nil:
		fmt.Fprintf(w, "left this home unstamped: it already had a constitution, and a manifest init wrote over it would arm the launch verify on prose nobody ratified (ADR 0015 §3)\n")
		fmt.Fprintf(w, "  the verify stays off until you run `posse promote`; until then no launch is refused for it\n")
	}
	// The arms above cover every way THIS run can leave the launch verify
	// clean; what they do not cover is a home the verify already reads as
	// broken for a reason none of them names — a promoted (not seeded) home
	// this run just added recipes/ or skills/ files to (ranger-base-pith:
	// copyIfMissing only ever ADDS, and only `posse promote` may re-stamp a
	// promoted manifest, so init cannot fix this one itself), or a
	// promoted.json this run found already unreadable (ranger-base-pith,
	// comment 2026-08-29).
	// Both used to reach exit 0 with nothing printed, so a dispatched launch
	// started refusing hours later with nothing connecting it back to this
	// init. One check closes both: whatever VerifyPromoted reads on the way
	// out is what the next dispatched launch will read too.
	if v := a.VerifyPromoted(); !v.OK() {
		fmt.Fprintf(w, "%s — every dispatched launch will refuse until you run `posse promote` (ADR 0015 §3)\n", v.Line())
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
// — what init lays on the shelf and counts. What retireExamplePIDs walks is
// wider by the names posse has retired: retirableExampleNames below.
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

// retirableExampleNames is every persona name posse has ever shipped an
// example for: the names in this seed, plus the ones only the digest table
// still knows (exampledigests.go). A rename is why the second half exists —
// rangerhq-o7y4 renamed agents/ranger.md to agents/ops.md, and a home seeded
// by any release before it holds agents/ranger.md. Walking the embed alone
// would leave that generic in agents/ taking beads in label routing forever,
// which is the ranger-base-8ehw leak arriving by a different door: retiring
// nothing and saying nothing. The proof a file is posse's is unchanged and
// still bytes (isShippedExample) — this only decides which names to ask about.
func retirableExampleNames(src fs.FS) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range exampleAgentNames(src) {
		seen[n] = true
		out = append(out, n)
	}
	for rel := range shippedExampleDigests {
		dir, file := path.Split(rel)
		if dir != "agents/" || !strings.HasSuffix(file, ".md") {
			continue
		}
		if n := strings.TrimSuffix(file, ".md"); !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// retireExamplePIDs moves agents/<name>.md onto the shelf for every persona
// that IS the shipped example, byte for byte — over every name posse has
// shipped one under, this seed's and the renamed-away
// (retirableExampleNames).
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
	names := retirableExampleNames(src)
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
		// The shelf slot must hold bytes posse shipped under this name: if
		// the operator edited the shelf, theirs wins and nothing moves onto
		// it. That is the same question as the live file's above and it has
		// the same two answers, which is the whole of ranger-base-788w —
		// judged against the running binary's embed alone, a slot holding an
		// EARLIER release's example reads as operator-edited. Every home
		// whose shelf an earlier posse wrote holds exactly that slot,
		// because copyIfMissing returns the moment the destination exists
		// and so never rewrites an occupied one; the version-skew confusion
		// ranger-base-8ehw took out of the live-file test was still sitting
		// one line down, blocking the retirement on the very homes 8ehw was
		// filed to rescue. The digest table (exampledigests.go) answers it
		// for the shelf the same way and needs nothing in the home to do it.
		// A name this seed no longer ships has no slot to compare against —
		// copyIfMissing writes nothing for it — so there the shelf may
		// simply be free.
		shelf := filepath.Join(a.ExampleAgentsDir(), name+".md")
		b, shelfErr := os.ReadFile(shelf)
		switch {
		case shelfErr != nil:
			// A name this seed ships had that slot written by copyIfMissing
			// a few lines up, so failing to read it back is a fault in the
			// home rather than an empty slot to move onto.
			if wantErr == nil {
				kept = append(kept, name+".md (the shelf copy is unreadable — not overwriting it)")
				continue
			}
		case !isShippedExample(rel, b):
			kept = append(kept, name+".md (the shelf copy differs — not overwriting it)")
			continue
		}
		// Rename, not remove. When the live file is an older release's
		// example its bytes exist nowhere else in the home. What the slot
		// held is posse's own prose — the gate above just proved it — so
		// overwriting it costs some release's copy of a file posse still
		// has, while removing the live file would cost the home the only
		// copy it has of anything. The slot then keeps what was retired,
		// for good: copyIfMissing returns the moment the destination
		// exists, so no later init rewrites an occupied slot, and
		// `RHQ_HOME=<scratch> posse init` is how to read what this seed
		// would have put there. Deliberate, and the opposite of what this
		// comment used to claim — "the one thing here init can always lay
		// down again" was the shelf slot, which is the one thing it never
		// lays down twice (ranger-base-xxar). Whatever leaves agents/ is
		// still readable, under the name init just printed.
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
		// to report the files it removed on purpose as MISSING. Narrow to
		// exactly the paths retirement touched: dropping the retired
		// agents/<name>.md entries, not re-hashing the whole promoted set,
		// which would silently re-anchor any unrelated drift already
		// sitting in config.yaml, recipes/ or skills/ — exactly what the
		// ADR 0015 §3 launch verify exists to refuse (ranger-base-9afo).
		if man != nil && man.Seeded {
			for _, name := range retired {
				delete(man.Files, path.Join("agents", name+".md"))
			}
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

// promotedFrom is the ", from <sha> in <repo>" clause a promoted manifest
// can carry, or "" — a manifest written before those fields, or by a
// promote that could not name the commit, still refuses; it just says less.
func promotedFrom(m *PromoteManifest) string {
	switch {
	case m.SHA != "" && m.Repo != "":
		return fmt.Sprintf(", from %s in %s", short(m.SHA), AbbrevHome(m.Repo))
	case m.SHA != "":
		return ", from " + short(m.SHA)
	case m.Source != "":
		return ", from " + AbbrevHome(m.Source)
	}
	return ""
}
