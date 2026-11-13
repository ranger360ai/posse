//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-f67jx, flipped to ADR 0091's resolution
// (ranger-base-f24yz): the launcher's OWN wall prep creates files and
// directories in the operator's home, and some of those names are entries on
// a runtime's project-config list. A session that starts in the home used to
// be classified against what posse made there.
//
// The coupling neither side could see from where it sits. The seed's
// correctness argument is about mount coverage and says "an EMPTY one is
// exactly as inert as none (seedBody)" (SeatbeltCarveOut.Seeded). The
// classifier's is about proving a PRESENT file safe, and it has a pin of its
// own saying "an empty file is not a missing file"
// (projectconfigtrust_qa_test.go, ranger-base-txd7). Both are right about
// their own half. Together they meant posse degraded a launch over a file
// posse created, and the remedy the refusal names — "remove the file" — was
// undone by the next caged launch, which seeds it again.
//
// It was first seen as a red test: TestPersonaLaunchRuntime's grok recipe
// named no `dir:`, so it launched in the test binary's shared $HOME, where a
// parallel test's real bwrap prep had seeded `.grok/config.toml`. That is
// fixed at the test (ranger-base-zvgql gave the recipe its own dir).
//
// WHICH SIDE GAVE: the classifier's premise, not the seed (ADR 0091 D1, from
// ranger-base-mkrsv). A declared executable member of a CLI home is not
// project scope, because no writer a repo's committer controls can write one
// — so ProjectConfigTrust SKIPS such an entry and every row below now reads
// clean. These pins were built to fail by name when that landed, and this is
// that landing. What they pin now is the property D1 rests on, which is
// strictly more than the old table said: every project-config entry the wall
// prep creates is a declared member of the exemption set. A future prep step
// that creates one OUTSIDE that set reds here with Member false and a
// degraded Finding beside it — which the old rows could not have caught,
// since they asserted the degrade.
//
// Two assertions, both MEASURED 2026-11-11 on linux and re-measured
// 2026-11-12 under D1:
//
//   - the CENSUS: which project-config entries the wall prep creates, which
//     runtime's prep creates each, whether D1 exempts it, what the launch is
//     told for it, and what the classifier ALONE still says about the thing
//     that was made. The last column is the fail-closed floor D4 leaves
//     untouched, and it is the old table unchanged — three shapes, only the
//     first of them inert: a `{}` seed under a KEYED list declares no key and
//     reads clean, a `{}` seed under a WHOLE-FILE entry degrades on presence
//     alone, and a zero-byte seed or an mkdir'd directory fails closed. It is
//     also the exit hatch's evidence: delete D1's skip and these are the
//     verdicts that come back.
//   - the VERDICT: what a launch in that home is told, per runtime, with the
//     control that the same launch in the same home is clean before the prep
//     runs — so the clean reads below are D1's and not the fixture's.
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
// who makes it, how, whether ADR 0091 D1 exempts it, what the launch is told
// for it, and what the classifier alone says about the thing that was made.
type wspEntry struct {
	Runtime string // whose project_config: names the entry
	Entry   string // the entry, as that runtime declared it
	By      string // whose wall prep creates it
	Kind    PrepKind
	Member  bool   // D1: <home>/<entry> IS a declared CLI-home exec member
	Finding string // the walk's answer for this entry; "" is nothing to say
	Floor   string // projectConfigTrustFile alone — the floor D4 leaves alone
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
	// D1's input, read the way ProjectConfigTrust reads it: off wallHome(),
	// which the fixture has pointed at f.home, and UNFILTERED by any grant.
	members := trustExemptCLIHomeExecMembers(wallHome())
	if len(members) == 0 {
		t.Fatalf("no CLI home members for HOME=%s — the exemption's input is empty and every row below would be vacuous", wallHome())
	}
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
				got = append(got, wspEntry{name, rel, by, kind,
					isCLIHomeExecMember(members, p),
					wspFinding(t, projectConfigTrustEntry(rt, f.home, rel, members)),
					wspFinding(t, projectConfigTrustFile(rt, rel, p))})
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
	// MEASURED 2026-11-11, linux 7.2.15, go1.27.3; the Member and Finding
	// columns re-measured 2026-11-12 under ADR 0091 D1, same box, go1.27.3.
	//
	// EVERY ROW IS A MEMBER, and that is the census's claim now: the wall
	// prep cannot create a project-config entry it does not also carve,
	// because the names it seeds and mkdirs come from the same
	// cliHomeExecMembers list D1's exemption reads (ADR 0091 D2). So every
	// Finding is "" — the launch is told nothing about any of them.
	//
	// The FLOOR column is the pre-D1 table, unchanged, and it is three
	// different mechanisms of which only the first row class is inert:
	//
	//   - a `{}` seed over a KEYED list declares no key, so it reads clean —
	//     this is the case ranger-base-6akzw's claim was written for, and it
	//     is the only one where it holds.
	//   - a `{}` seed over a WHOLE-FILE entry degrades on presence alone, so
	//     the body never gets a say (.grok/lsp.json, ProjectConfigWholeFile).
	//   - a ZERO-BYTE seed is invalid JSON, and a directory the prep mkdir'd
	//     is not a regular file. Both still fail closed on the thing posse
	//     made (D4) — they are simply no longer read for a launch in the home.
	want := []wspEntry{
		{"claude", ".claude/settings.json", "claude", PrepSeed, true, inertOK, inertOK},
		{"claude", ".claude/settings.local.json", "claude", PrepSeed, true, inertOK, inertOK},
		{"codex", ".codex/config.toml", "codex", PrepSeed, true, inertOK, present},
		{"grok", ".claude/plugins", "claude", PrepMkdirAll, true, inertOK, notFile},
		{"grok", ".claude/settings.json", "claude", PrepSeed, true, inertOK, inertOK},
		{"grok", ".claude/settings.local.json", "claude", PrepSeed, true, inertOK, inertOK},
		{"grok", ".grok/config.toml", "grok", PrepSeed, true, inertOK, invalid},
		{"grok", ".grok/hooks", "grok", PrepMkdirAll, true, inertOK, notFile},
		{"grok", ".grok/lsp.json", "grok", PrepSeed, true, inertOK, present},
		{"grok", ".grok/plugins", "grok", PrepMkdirAll, true, inertOK, notFile},
	}
	if len(got) != len(want) {
		t.Errorf("the collision census changed size: %d rows, want %d\n got: %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for i := range got {
		if i >= len(want) {
			t.Errorf("new collision, unrecorded: %+v — a wall member or a project_config: entry joined the other list; if Member is false, D1 does not exempt it and the launch degrades", got[i])
			continue
		}
		if got[i] != want[i] {
			t.Errorf("collision row %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
	for i := len(got); i < len(want); i++ {
		t.Errorf("recorded collision gone: %+v — a prep step or a project_config: entry was dropped; record the new census", want[i])
	}
}

// The verdict a real launch in that home gets. ProjectConfigTrust returns the
// FIRST entry with something to say in declared order, so this is one line
// per runtime and not a set — and under ADR 0091 D1 every line is clean,
// which is the whole point of the decision.
//
// RENAMED from …RefusesALaunchInTheHomeItSeeded, which is the pin the ADR
// meant by "both f67jx pins red by name": a name that still said "refuses"
// over a body asserting clean would be the trap those pins exist to avoid.
// The `-run 'WallSeedProjectConfig'` filter reaches it either way.
func TestQAWallSeedProjectConfigLaunchInTheHomeItSeededIsClean(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wspFixtureIn(t, t.TempDir())
	steps := f.prep(t, f.home)
	for _, name := range wspRuntimes {
		if err := runBwrapPrep(steps[name]); err != nil {
			t.Fatalf("prep for %s: %v", name, err)
		}
	}
	// MEASURED 2026-11-12 under ADR 0091 D1. Before it, claude alone was
	// clean — both of its entries are `{}` seeds under a keyed list — while
	// codex read "project config is present" on its zero-byte config.toml
	// and grok read "unreadable: a directory, not a regular file" on the
	// `.claude/plugins` a claude seat's prep had mkdir'd. All three are
	// members of their own CLI home, so all three are skipped now.
	for _, name := range wspRuntimes {
		if got := wspFinding(t, ProjectConfigTrust(f.rts[name], f.ag, f.home)); got != "" {
			t.Errorf("%s launching in the home its own wall prepped must be clean (ADR 0091 D1): %q", name, got)
		}
	}

	// THE SEED IS UNCHANGED, AND THAT IS THE RECORD OF WHICH SIDE GAVE. The
	// file the old refusal told the operator to remove is still re-created by
	// the very next caged launch — nothing in BwrapArgs' step list moved —
	// and the classifier still calls that file unreadable when it is asked
	// about it directly, which is D4's floor. What changed is that a launch
	// in the home no longer asks: D1 skips the entry before the floor sees
	// it. Delete the skip and this arm's last two assertions swap places.
	cfg := filepath.Join(f.home, ".grok", "config.toml")
	if b, err := os.ReadFile(cfg); err != nil || len(b) != 0 {
		t.Fatalf("the seed must be a zero-byte file: %v %q", err, b)
	}
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	if why := ProjectConfigTrust(f.rts["grok"], f.ag, f.home); why != "" {
		t.Errorf("a grok launch with the file REMOVED must be clean too — the exemption is about the path, not about what is at it: %s", why)
	}
	if err := runBwrapPrep(f.prep(t, f.home)["grok"]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Errorf("the next launch must seed it again (unchanged by ADR 0091, which took the classifier's side): %v", err)
	}
	if why := ProjectConfigTrust(f.rts["grok"], f.ag, f.home); why != "" {
		t.Errorf("…and the reseeded file must leave the launch clean: %s", why)
	}
	if why := projectConfigTrustFile(f.rts["grok"], ".grok/config.toml", cfg); wspFinding(t, why) == "" {
		t.Errorf("D4's floor is untouched: asked about the file itself, the classifier must still refuse a zero-byte seed")
	}
}
