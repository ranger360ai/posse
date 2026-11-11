//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-f67jx: the launcher's OWN wall prep creates files and
// directories in the operator's home, and some of those names are entries on
// a runtime's project-config list. A session that starts in the home is then
// classified against what posse made there.
//
// The coupling neither side could see from where it sits. The seed's
// correctness argument is about mount coverage and says "an EMPTY one is
// exactly as inert as none (seedBody)" (SeatbeltCarveOut.Seeded). The
// classifier's is about proving a PRESENT file safe, and it has a pin of its
// own saying "an empty file is not a missing file"
// (projectconfigtrust_qa_test.go, ranger-base-txd7). Both are right about
// their own half. Together they mean posse degrades a launch over a file
// posse created, and the remedy the refusal names — "remove the file" — is
// undone by the next caged launch, which seeds it again.
//
// It was first seen as a red test: TestPersonaLaunchRuntime's grok recipe
// named no `dir:`, so it launched in the test binary's shared $HOME, where a
// parallel test's real bwrap prep had seeded `.grok/config.toml`. That is
// fixed at the test (ranger-base-zvgql gave the recipe its own dir); these
// pins are for the mechanism under it, which is not fixed and is not this
// bead's to decide. The decision is ranger-base-mkrsv.
//
// Two assertions, both MEASURED 2026-11-11 on linux:
//
//   - the CENSUS: which project-config entries the wall prep creates, which
//     runtime's prep creates each, and what the classifier then finds for it.
//     A new wall member or a new project-config entry that collides reds here
//     and names itself. The contrast is the point — `{}` seeds (claude's
//     settings.json) classify CLEAN, zero-byte seeds and mkdir'd directories
//     do not.
//   - the VERDICT: what a launch in that home is told, per runtime, with the
//     control that the same launch in the same home is clean before the prep
//     runs.
//
// bwrap-only, because the seed is (SeatbeltCarveOut.Seeded: a mount cannot
// cover a name that is not there, and an SBPL subpath deny needs no name).
// Nothing is created on darwin, so there is no collision to pin there.
//
//	go test ./internal/posse -run 'WallSeedProjectConfig' -v

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// wspRuntimes are the runtimes this box launches whose project-config list
// can meet a wall prep at all. bob is left out on its list and not on its
// wall: bobProjectConfig is `.bob/*` only, and no carve here makes a `.bob/`
// name, so bob has no row in either direction.
var wspRuntimes = []string{"claude", "codex", "grok"}

// wspEntry is one project-config entry the wall prep creates: who lists it,
// who makes it, how, and what the classifier says about the thing that was
// made. Finding "" is a clean read — the seed really was inert for that one.
type wspEntry struct {
	Runtime string // whose project_config: names the entry
	Entry   string // the entry, as that runtime declared it
	By      string // whose wall prep creates it
	Kind    PrepKind
	Finding string
}

// wspFinding strips the boilerplate either side of the classifier's finding,
// which is the part these pins differ in. projectconfigtrust_qa_test.go has
// the same helper for arm 2; they are in packages no build sees at once.
func wspFinding(t *testing.T, why string) string {
	t.Helper()
	if why == "" {
		return ""
	}
	i, j := strings.Index(why, "turn — "), strings.Index(why, "; project-owned")
	if i < 0 || j < 0 {
		t.Fatalf("refusal text no longer has a finding clause: %q", why)
	}
	return why[i+len("turn — ") : j]
}

// wspFixture is a box in miniature with EMPTY CLI homes: nothing in it is a
// project config until the wall's own prep puts one there.
type wspFixture struct {
	home, gates string
	a           *App
	ag          *AgentFile
	rts         map[string]*Runtime
}

func wspFixtureIn(t *testing.T, root string) wspFixture {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("the seed is the bwrap mechanism's; seatbelt creates nothing (SeatbeltCarveOut.Seeded)")
	}
	at := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	f := wspFixture{home: at("home"), rts: map[string]*Runtime{}}
	for _, d := range []string{"home/.claude", "home/.codex", "home/.grok", "home/.cache", "posse/state", "posse/personas/developer"} {
		if err := os.MkdirAll(at(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", f.home)
	t.Setenv(CodexHomeVar, "")
	t.Setenv(GrokHomeVar, "")
	t.Setenv(ClaudeConfigDirVar, at("home", ".claude"))
	f.a = &App{Home: at("posse"), StateDir: at("posse", "state")}
	f.ag = &AgentFile{Name: "developer", MemoryDir: at("posse", "personas", "developer")}
	f.gates = f.a.GatesDir(f.ag.Name)
	if err := os.MkdirAll(f.gates, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range wspRuntimes {
		rt, err := f.a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("load runtime %s: %v", name, err)
		}
		f.rts[name] = rt
	}
	return f
}

// prep renders each runtime's wall for a session that starts in `dir` and
// returns the launcher's steps, keyed by the runtime that asked for them.
// The render reads the host and writes nothing; applying is the caller's.
func (f wspFixture) prep(t *testing.T, dir string) map[string][]PrepStep {
	t.Helper()
	out := map[string][]PrepStep{}
	for _, name := range wspRuntimes {
		r, err := f.a.bwrapRender(f.ag, f.rts[name], dir, f.gates)
		if err != nil {
			t.Fatalf("bwrap render for %s: %v", name, err)
		}
		out[name] = r.Prep
	}
	return out
}

// The census. Every project-config entry the wall prep creates in the home,
// with the classifier's finding for the thing that was made.
func TestQAWallSeedProjectConfigCensusOfWhatThePrepCreates(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wspFixtureIn(t, t.TempDir())
	steps := f.prep(t, f.home)

	// Control: an empty home is a clean launch for every runtime, so
	// everything below is the prep's doing and not the fixture's.
	for _, name := range wspRuntimes {
		if why := ProjectConfigTrust(f.rts[name], f.ag, f.home); why != "" {
			t.Fatalf("%s: a home with no CLI config must classify clean before any prep runs: %s", name, why)
		}
	}

	// Attribution by OBSERVATION and not by reading the step list: a step
	// deeper in the tree brings its parents into being too (PrepMkdirAll is
	// `mkdir -p`, and PrepSeed makes the seed's directory first), so an entry
	// can be created by a step that does not name it. Each runtime's prep is
	// applied in turn and the entry belongs to the first one after which it
	// is there.
	exists := func(p string) bool { _, err := os.Lstat(p); return err == nil }
	var got []wspEntry
	seen := map[string]bool{}
	for _, by := range wspRuntimes {
		if err := runBwrapPrep(steps[by]); err != nil {
			t.Fatalf("prep for %s: %v", by, err)
		}
		for _, name := range wspRuntimes {
			rt := f.rts[name]
			for _, rel := range rt.ProjectConfig {
				p := filepath.Join(f.home, filepath.FromSlash(rel))
				if seen[name+" "+rel] || !exists(p) {
					continue
				}
				seen[name+" "+rel] = true
				// The step that names the entry says how it was made; an
				// entry made on the way to a deeper one is a directory
				// however that step was spelled.
				kind := PrepMkdirAll
				for _, s := range steps[by] {
					if absResolve(s.Path) == absResolve(p) {
						kind = s.Kind
					}
				}
				got = append(got, wspEntry{name, rel, by, kind, wspFinding(t, projectConfigTrustFile(rt, rel, p))})
			}
		}
	}
	sort.Slice(got, func(i, j int) bool {
		if got[i].Runtime != got[j].Runtime {
			return got[i].Runtime < got[j].Runtime
		}
		return got[i].Entry < got[j].Entry
	})

	const (
		invalid = "project config classification failed: invalid JSON: unexpected end of JSON input"
		notFile = "project config classification failed: unreadable: a directory, not a regular file"
		present = "project config is present"
		inertOK = "" // the seed really was as inert as no file at all
	)
	// MEASURED 2026-11-11, linux 7.2.15, go1.27.3. The three findings are
	// three different mechanisms and only the first row class is inert:
	//
	//   - a `{}` seed over a KEYED list declares no key, so it reads clean —
	//     this is the case ranger-base-6akzw's claim was written for, and it
	//     is the only one where it holds.
	//   - a `{}` seed over a WHOLE-FILE entry degrades on presence alone, so
	//     the body never gets a say (.grok/lsp.json, ProjectConfigWholeFile).
	//   - a ZERO-BYTE seed is invalid JSON, and a directory the prep mkdir'd
	//     is not a regular file. Both fail closed on a thing posse made.
	want := []wspEntry{
		{"claude", ".claude/settings.json", "claude", PrepSeed, inertOK},
		{"claude", ".claude/settings.local.json", "claude", PrepSeed, inertOK},
		{"codex", ".codex/config.toml", "codex", PrepSeed, present},
		{"grok", ".claude/plugins", "claude", PrepMkdirAll, notFile},
		{"grok", ".claude/settings.json", "claude", PrepSeed, inertOK},
		{"grok", ".claude/settings.local.json", "claude", PrepSeed, inertOK},
		{"grok", ".grok/config.toml", "grok", PrepSeed, invalid},
		{"grok", ".grok/hooks", "grok", PrepMkdirAll, notFile},
		{"grok", ".grok/lsp.json", "grok", PrepSeed, present},
		{"grok", ".grok/plugins", "grok", PrepMkdirAll, notFile},
	}
	if len(got) != len(want) {
		t.Errorf("the collision census changed size: %d rows, want %d\n got: %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for i := range got {
		if i >= len(want) {
			t.Errorf("new collision, unrecorded: %+v — a wall member or a project_config: entry joined the other list", got[i])
			continue
		}
		if got[i] != want[i] {
			t.Errorf("collision row %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
	for i := len(got); i < len(want); i++ {
		t.Errorf("recorded collision gone: %+v — if that is the fix, this row and the decision bead go with it", want[i])
	}
}

// The verdict a real launch in that home gets, and the remedy the refusal
// names. ProjectConfigTrust returns the FIRST entry with something to say in
// declared order, so this is one line per runtime and not a set.
func TestQAWallSeedProjectConfigRefusesALaunchInTheHomeItSeeded(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wspFixtureIn(t, t.TempDir())
	steps := f.prep(t, f.home)
	for _, name := range wspRuntimes {
		if err := runBwrapPrep(steps[name]); err != nil {
			t.Fatalf("prep for %s: %v", name, err)
		}
	}
	// MEASURED 2026-11-11. claude is clean because both of its entries are
	// `{}` seeds under a keyed list; the other two are not.
	for _, tc := range []struct{ name, want string }{
		{"claude", ""},
		{"codex", "project config is present"},
		{"grok", "project config classification failed: unreadable: a directory, not a regular file"},
	} {
		got := wspFinding(t, ProjectConfigTrust(f.rts[tc.name], f.ag, f.home))
		if got != tc.want {
			t.Errorf("%s launching in the home its own wall prepped: got %q, want %q", tc.name, got, tc.want)
		}
	}

	// The remedy the refusal text names does not survive the next launch.
	// Removing the file is what an operator is told to do, and the very next
	// caged launch seeds it again — so the degrade is not something the
	// operator can clear while posse keeps launching caged seats there.
	cfg := filepath.Join(f.home, ".grok", "config.toml")
	if b, err := os.ReadFile(cfg); err != nil || len(b) != 0 {
		t.Fatalf("the seed must be a zero-byte file: %v %q", err, b)
	}
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(f.prep(t, f.home)["grok"]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Errorf("the next launch must seed it again (that is the defect, not a surprise): %v", err)
	}
	if why := projectConfigTrustFile(f.rts["grok"], ".grok/config.toml", cfg); wspFinding(t, why) == "" {
		t.Errorf("remove-and-relaunch cleared the finding; if that is the fix, this assertion and ranger-base-mkrsv go with it")
	}
}
