//go:build posse_arm2

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// QA pins for ranger-base-l12j9 — grok's repo→box project-config channel,
// which was UNDECLARED until this bead and so skipped by ProjectConfigTrust
// entirely: hooks that RAN and MCP servers that SPAWNED in every trusted grok
// launch, printed as a CLEAN launch, while a claude seat over the very same
// `.claude/settings.json` degraded. That asymmetry is the whole finding
// (ranger-base-1g8gf, MEASURED 2026-10-31 on grok 1.0.52 (0920c8169e27)); the
// measurement table and the per-entry reasoning live on grokProjectConfig in
// runtime.go, and docs/notes.d/ranger-base-l12j9.md carries both.
//
// Why pins at all, and why one per ENTRY. The failure mode here has no
// diagnostic: a path dropped from the list is a channel ProjectConfigTrust
// stops looking at, and every launch over it goes back to reading clean. The
// list is also mixed by type on purpose — two JSON settings files, three
// JSON-by-convention files, one TOML, two directories — so the arms below
// assert the FINDING and not merely that something degraded: that is what
// tells a key match from the fail-closed classification, and what would
// notice the whole list quietly becoming whole-file or the keys quietly
// becoming nil.
//
// Arm 1 is the declaration itself, including the one value that looks like
// tightening and is not (`permissions`). Arms 2-4 are the DONE WHEN of the
// bead: a fixture per entry degrades, a permissions-only settings file does
// not, and the PID opt-in clears all of it.

// grokProjectConfigFixture is one entry of the declared list and the body
// that makes it live, with the finding projectConfigTrustFile should reach.
type grokProjectConfigFixture struct {
	entry string // the declared path, relative to the session dir
	body  string // "" means: plant it as a DIRECTORY, with one file inside
	want  string // the finding clause, as projectConfigVerdict strips it
}

// The eight declared entries, in declaration order, each with the shape the
// measurement found on it. The three verdicts are the three paths through
// projectConfigTrustFile, which is why they are asserted by name:
//
//   - a key match, for everything grok reads as JSON;
//   - "invalid JSON", for .grok/config.toml — TOML parsed by a JSON
//     classifier. The RIGHT verdict (that file contributes mcp_servers,
//     permissions and plugins) reached by a misleading reason, which is the
//     declared design note and not a defect to fix here;
//   - "a directory, not a regular file", for the two directory entries, which
//     is the same fail-closed arm a FIFO or a dangling link takes.
var grokProjectConfigFixtures = []grokProjectConfigFixture{
	{entry: ClaudeProjectConfig,
		body: `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"touch marker"}]}]}}`,
		want: "matched top-level project config keys: hooks"},
	{entry: ClaudeProjectConfigLocal,
		body: `{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"touch marker"}]}]}}`,
		want: "matched top-level project config keys: hooks"},
	{entry: ".mcp.json",
		body: `{"mcpServers":{"probe":{"command":"/bin/sh","args":["-c","touch marker"]}}}`,
		want: "matched top-level project config keys: mcpServers"},
	{entry: ".cursor/mcp.json",
		body: `{"mcpServers":{"probe":{"command":"/bin/sh","args":["-c","touch marker"]}}}`,
		want: "matched top-level project config keys: mcpServers"},
	// Dormant on 1.0.52 — loaded, then refused in grok's own words ("this
	// Cursor hook won't run until Grok reads Cursor hook replies") — and
	// declared anyway, because the refusal is a version's and the file is
	// already being read. This is the row a future grok is expected to
	// change, and the pin is what makes the change visible rather than
	// welcome.
	{entry: ".cursor/hooks.json",
		body: `{"version":1,"hooks":{"beforeShellExecution":[{"command":"touch marker"}]}}`,
		want: "matched top-level project config keys: hooks"},
	{entry: ".grok/config.toml",
		body: "[mcp_servers.probe]\ncommand = \"/bin/sh\"\nargs = [\"-c\", \"touch marker\"]\n",
		want: "project config classification failed: invalid JSON",
	},
	{entry: ".grok/hooks", want: "project config classification failed: unreadable: a directory, not a regular file"},
	{entry: ".grok/plugins", want: "project config classification failed: unreadable: a directory, not a regular file"},
}

// plantGrokProjectConfig writes one fixture into a fresh session dir.
func plantGrokProjectConfig(t *testing.T, dir string, f grokProjectConfigFixture) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(f.entry))
	if f.body == "" {
		// A directory entry: a tree, with something in it, because an EMPTY
		// one is the same verdict for a less interesting reason and a reader
		// should not have to wonder which this measured.
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("fixture %s: %v", f.entry, err)
		}
		if err := os.WriteFile(filepath.Join(p, "probe.json"), []byte(`{"hooks":{}}`), 0o644); err != nil {
			t.Fatalf("fixture %s: %v", f.entry, err)
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("fixture %s: %v", f.entry, err)
	}
	if err := os.WriteFile(p, []byte(f.body), 0o644); err != nil {
		t.Fatalf("fixture %s: %v", f.entry, err)
	}
}

// Arm 1: the declaration. Spelled out rather than compared to a copy of
// itself — the list IS the guard's reach, and an entry silently dropped from
// it is a channel nobody looks at any more.
func TestQAGrokDeclaresItsProjectConfigChannel(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	grok, err := b.App.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".claude/settings.json", ".claude/settings.local.json",
		".mcp.json", ".cursor/mcp.json", ".cursor/hooks.json",
		".grok/config.toml", ".grok/hooks", ".grok/plugins",
	}
	if strings.Join(grok.ProjectConfig, " ") != strings.Join(want, " ") {
		t.Fatalf("grok's project-config channel is not the measured list (ranger-base-l12j9): got %v, want %v", grok.ProjectConfig, want)
	}
	// The two settings paths come from the shared constants, so claude's
	// list and grok's cannot drift apart over the same two files.
	if grok.ProjectConfig[0] != ClaudeProjectConfig || grok.ProjectConfig[1] != ClaudeProjectConfigLocal {
		t.Errorf("the settings entries must be the shared constants: %v", grok.ProjectConfig[:2])
	}
	if strings.Join(grok.ProjectConfigKeys, " ") != "hooks mcpServers" {
		t.Fatalf("grok's keys must be claude's two: %v", grok.ProjectConfigKeys)
	}
	// `permissions` is the one addition here that would look like
	// tightening. Grok DOES load a repo's permissions under trust (MEASURED:
	// 2 + 1 entries, against 0 untrusted) and it still does not belong: a
	// permission grant is not an exec channel, claude's declaration excludes
	// it for that reason, and posse's own .claude/settings.json is
	// permissions-only — so adding it would degrade every crew grok seat in
	// this repo.
	for _, k := range grok.ProjectConfigKeys {
		if k == "permissions" {
			t.Error("`permissions` must not be a project-config key: grok loading a repo's permissions is the same posture claude already takes, and posse's own permissions-only .claude/settings.json has to stay a clean grok launch")
		}
	}
	// The keys are what keep this list from being a whole-file predicate —
	// the shape that would degrade every launch in this very repo.
	if len(grok.ProjectConfigKeys) == 0 {
		t.Error("an empty key list turns this declaration into the whole-file predicate")
	}
}

// Arm 2, the bead's first DONE WHEN: a fixture per entry degrades a grok
// launch, names the file, names the finding, and offers the two ways out.
func TestQAEveryGrokProjectConfigEntryDegradesTheLaunch(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")
	grok, _ := b.App.LoadRuntime("grok")
	codex, _ := b.App.LoadRuntime("codex")

	if len(grokProjectConfigFixtures) != len(grok.ProjectConfig) {
		t.Fatalf("this table has %d entries and grok declares %d — a declared channel with no fixture is a channel nobody measured", len(grokProjectConfigFixtures), len(grok.ProjectConfig))
	}
	for i, f := range grokProjectConfigFixtures {
		if f.entry != grok.ProjectConfig[i] {
			t.Fatalf("fixture %d is %s and the declaration's entry %d is %s — the table must walk the declared list in order, because ProjectConfigTrust names the FIRST offender", i, f.entry, i, grok.ProjectConfig[i])
		}
	}

	for _, f := range grokProjectConfigFixtures {
		// A session name is letters, digits, - and _, so the entry's own
		// spelling cannot be one.
		slug := strings.Trim(strings.NewReplacer("/", "-", ".", "-").Replace(f.entry), "-")
		t.Run(slug, func(t *testing.T) {
			repo := t.TempDir()
			// Clean before, so each arm measures the entry it planted and
			// not something the fixture dir came with.
			if why := ProjectConfigTrust(grok, dev, repo); why != "" {
				t.Fatalf("an empty session dir must be clean on grok: %q", why)
			}
			plantGrokProjectConfig(t, repo, f)

			got := projectConfigVerdict(t, ProjectConfigTrust(grok, dev, repo))
			if !strings.HasPrefix(got, f.want) {
				t.Fatalf("%s: want finding %q, got %q", f.entry, f.want, got)
			}
			// Degraded, never Unrealized: this says what the launch gives
			// away, not which gate went unenforced.
			p := b.App.CheckParityIn(dev, grok, CageShims, TierStrong, repo)
			if len(p.Degraded) != 1 || !strings.Contains(p.Degraded[0], f.entry) {
				t.Fatalf("%s must be the one degradation, named: %+v", f.entry, p.Degraded)
			}
			// The launch refuses by default and names both ways out.
			err := b.CreateSession(NewSessionOpts{Name: "g-" + slug, Agent: "dev", Runtime: "grok", Dir: repo})
			if err == nil || !strings.Contains(err.Error(), f.entry) ||
				!strings.Contains(err.Error(), f.want) ||
				!strings.Contains(err.Error(), "trust_project_config: true") ||
				!strings.Contains(err.Error(), "--allow-degraded") {
				t.Fatalf("a grok launch over %s must refuse, naming the file, the finding and the two ways out: %v", f.entry, err)
			}
			// Nobody else's surface: codex declares one file and it is not
			// on this list, so the same dir is clean there. That is the
			// per-runtime half of the asymmetry this bead closed — the
			// channel belongs to the engine that reads it.
			if f.entry != CodexProjectConfig {
				if why := ProjectConfigTrust(codex, dev, repo); why != "" {
					t.Errorf("%s is not codex's surface: %q", f.entry, why)
				}
			}
			// And the dir-independent matrix stays dir-independent.
			if p := b.App.CheckParity(dev, grok, CageShims, TierStrong); len(p.Degraded) != 0 {
				t.Errorf("CheckParity must not stat anything: %+v", p)
			}
		})
	}
}

// Arm 3, the bead's second DONE WHEN: the file this repo actually ships.
// posse's own .claude/settings.json is permissions-only, and every crew grok
// seat starts in a checkout that has it — so a key list that caught it would
// degrade all of them. The literal body here is that file's shape; the arm
// that reads the REAL file is in internal/treepins, where the test binary's
// cwd is the repo root (grokprojectconfigclean_qa_test.go).
func TestQAGrokLaunchIsCleanOverPermissionsOnlySettings(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")
	grok, _ := b.App.LoadRuntime("grok")

	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
	body := `{"permissions":{"allow":["Edit","Bash(make:*)"],"deny":["Bash(bd daemon:*)","Edit(.claude/**)"]}}`
	if err := os.WriteFile(filepath.Join(repo, ClaudeProjectConfig), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if why := ProjectConfigTrust(grok, dev, repo); why != "" {
		t.Fatalf("permissions-only settings must keep a grok launch clean — every crew seat in this repo starts in a checkout that has one: %q", why)
	}
	mustCreate(t, b, NewSessionOpts{Name: "gk-permissions", Agent: "dev", Runtime: "grok", Dir: repo})
	if m, _ := b.readMeta("gk-permissions"); m == nil || m.Degraded != "" {
		t.Errorf("that launch is not degraded: %+v", m)
	}
	// The discriminator: the SAME file with an exec key is the finding, so
	// the arm above is measuring the key list and not a dead check.
	if err := os.WriteFile(filepath.Join(repo, ClaudeProjectConfig),
		[]byte(`{"permissions":{"allow":["Edit"]},"hooks":{"SessionStart":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := projectConfigVerdict(t, ProjectConfigTrust(grok, dev, repo)); got != "matched top-level project config keys: hooks" {
		t.Errorf("permissions PLUS hooks is the finding: %q", got)
	}
}

// Arm 4, the bead's third DONE WHEN: the PID opt-in clears every entry, and
// it is the same durable opt-in the other three runtimes have. One PID, the
// whole list — because the operator's next move is the same for any of them.
func TestQAGrokProjectConfigTrustIsOptInPerPID(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "trusting.md"),
		[]byte("---\nname: trusting\ntrust_project_config: true\ndeny: [Bash(git push:*)]\n---\nYou are trusting.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")
	trusting, _ := b.App.LoadAgent("trusting")
	grok, _ := b.App.LoadRuntime("grok")

	// Every entry at once, so the opt-in is shown to clear the whole list
	// and not just whichever one comes first.
	repo := t.TempDir()
	for _, f := range grokProjectConfigFixtures {
		plantGrokProjectConfig(t, repo, f)
	}
	if why := ProjectConfigTrust(grok, dev, repo); why == "" {
		t.Fatal("a dir holding every declared entry must degrade without the opt-in")
	}
	if why := ProjectConfigTrust(grok, trusting, repo); why != "" {
		t.Fatalf("trust_project_config: true on the PID must clear it: %q", why)
	}
	mustCreate(t, b, NewSessionOpts{Name: "gk-trusting", Agent: "trusting", Runtime: "grok", Dir: repo})
	if m, _ := b.readMeta("gk-trusting"); m == nil || m.Degraded != "" {
		t.Errorf("an opted-in launch is not degraded: %+v", m)
	}
	// --allow-degraded is the per-launch waiver, and it stays MARKED.
	mustCreate(t, b, NewSessionOpts{Name: "gk-waived", Agent: "dev", Runtime: "grok", Dir: repo, AllowDegraded: true})
	if m, _ := b.readMeta("gk-waived"); m == nil || !strings.Contains(m.Degraded, ClaudeProjectConfig) {
		t.Errorf("a waived launch must stay marked: %+v", m)
	}
	// tier fast: the operator's consent is not on offer there (ADR 0003 §3).
	if p := b.App.CheckParityIn(dev, grok, CageShims, TierFast, repo); !p.NoDegrade {
		t.Errorf("fast must not be waivable: %+v", p)
	}
}

// Arm 5: the asymmetry itself, which is the thing ranger-base-1g8gf found and
// the thing no single-runtime pin above can state. ONE repo, ONE settings
// file carrying hooks — claude degraded on it from the start, and grok has to
// degrade on it too, or the wall depends on which CLI a dispatch happened to
// pick.
func TestQAProjectConfigHooksDegradeEveryRuntimeThatReadsThem(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")

	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
	os.WriteFile(filepath.Join(repo, ClaudeProjectConfig),
		[]byte(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"touch marker"}]}]}}`), 0o644)

	for _, name := range []string{"claude", "grok"} {
		rt, err := b.App.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		got := projectConfigVerdict(t, ProjectConfigTrust(rt, dev, repo))
		if got != "matched top-level project config keys: hooks" {
			t.Errorf("%s reads this file's hooks before any turn, so the launch must say so: %q", name, got)
		}
	}
	// codex is the control: it reads its own file and not this one, so a
	// pass over the list above cannot be passing because every runtime
	// degrades on everything.
	codex, _ := b.App.LoadRuntime("codex")
	if why := ProjectConfigTrust(codex, dev, repo); why != "" {
		t.Errorf("codex does not read .claude/settings.json; the control arm must be clean: %q", why)
	}
}
