package posse

// What these pin (rangerhq-w4uf): a claude launched in a directory it has
// never run in opens on a full-screen trust modal, so the launch has to
// have answered it — in the operator's config, before the line is typed,
// because claude 2.1.241 has no flag and no settings key for it.
//
// Two of these tests are about the *file* rather than the feature, and
// they are the ones worth keeping: ~/.claude.json holds the operator's
// whole claude state, and this is the only thing a launch writes outside
// RHQ_HOME and the session dir.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readConfig(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	state := map[string]any{}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	return state
}

func project(t *testing.T, state map[string]any, dir string) map[string]any {
	t.Helper()
	projects, _ := state["projects"].(map[string]any)
	if projects == nil {
		t.Fatalf("config has no projects map: %v", state)
	}
	e, _ := projects[ClaudeTrustKey(dir)].(map[string]any)
	return e
}

func writeConfig(t *testing.T, p string, state map[string]any) {
	t.Helper()
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func claudeRuntime(t *testing.T) *Runtime {
	t.Helper()
	rt, err := (&App{}).LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// The path resolution is claude's own, and it has three branches — getting
// it wrong writes a real file that grants nothing, which is the failure a
// pane would show as the dialog posse thought it had answered.
func TestClaudeConfigFileFollowsClaudesOwnResolution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	if got, want := ClaudeConfigFile(), filepath.Join(home, ".claude.json"); got != want {
		t.Errorf("plain HOME: got %s, want %s", got, want)
	}

	// CLAUDE_CONFIG_DIR moves the basename, not just the lookup dir.
	cfgDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfgDir)
	if got, want := ClaudeConfigFile(), filepath.Join(cfgDir, ".claude.json"); got != want {
		t.Errorf("CLAUDE_CONFIG_DIR: got %s, want %s", got, want)
	}

	// .config.json in the config dir wins over both when it exists.
	newer := filepath.Join(cfgDir, ".config.json")
	if err := os.WriteFile(newer, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ClaudeConfigFile(); got != newer {
		t.Errorf(".config.json must win where it exists: got %s, want %s", got, newer)
	}
}

// The bug itself: a directory claude has never run in. MEASURED on 2.1.241
// (herdr scratch panes) that this key is what decides the modal, so this is
// what a launch has to have written by the time the line is typed.
func TestSeedTrustAnswersTheDialogForAFreshDir(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json") // a HOME that never ran claude
	dir := t.TempDir()

	wrote, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != cfg {
		t.Fatalf("seed reported %q, want %q", wrote, cfg)
	}
	e := project(t, readConfig(t, cfg), dir)
	if e["hasTrustDialogAccepted"] != true {
		t.Errorf("the key that decides the modal is not set: %v", e)
	}
	if e["hasCompletedProjectOnboarding"] != true {
		t.Errorf("the welcome panel is not silenced: %v", e)
	}
	// Account state lives in this file too; a fresh one is not world-readable.
	st, err := os.Stat(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("new config mode %v, want 0600", st.Mode().Perm())
	}
	// And the launch's own question is answered by the file it just wrote.
	if !ClaudeTrusted(readConfig(t, cfg), dir) {
		t.Error("posse wrote a key it does not itself read as trust")
	}
}

// The key is the path claude looks up, not the path posse was handed: the
// cwd claude reads is the kernel's, so /tmp is /private/tmp on macOS and a
// key under the symlinked spelling grants nothing.
func TestSeedTrustKeysTheResolvedPath(t *testing.T) {
	t.Parallel()
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	if _, err := SeedClaudeTrust(cfg, claudeRuntime(t), link); err != nil {
		t.Fatal(err)
	}
	projects, _ := readConfig(t, cfg)["projects"].(map[string]any)
	if _, ok := projects[ClaudeTrustKey(real)]; !ok {
		t.Errorf("want the resolved path as the key, got %v", keysOf(projects))
	}
}

func keysOf(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// This file is the operator's, not posse's: everything already in it has to
// survive, including projects posse never launched in.
func TestSeedTrustMergesTheOperatorsConfig(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	other := t.TempDir()
	writeConfig(t, cfg, map[string]any{
		"theme":                  "dark",
		"hasCompletedOnboarding": true,
		"projects": map[string]any{
			ClaudeTrustKey(other): map[string]any{"hasTrustDialogAccepted": false, "exampleCount": float64(3)},
		},
	})

	dir := t.TempDir()
	if _, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir); err != nil {
		t.Fatal(err)
	}
	state := readConfig(t, cfg)
	if state["theme"] != "dark" || state["hasCompletedOnboarding"] != true {
		t.Errorf("top-level operator state was dropped: %v", state)
	}
	kept := project(t, state, other)
	if kept["hasTrustDialogAccepted"] != false || kept["exampleCount"] != float64(3) {
		t.Errorf("another project's entry was rewritten: %v", kept)
	}
	if project(t, state, dir)["hasTrustDialogAccepted"] != true {
		t.Error("the session dir was not seeded")
	}
}

// A launch in the fleet's own long-lived checkout must not rewrite a
// 100KB config it has nothing to add to — the write is the risk here, not
// the read.
func TestSeedTrustWritesNothingWhenTheDirIsAlreadyTrusted(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	dir := t.TempDir()
	// Both of the launch's questions already answered — trust for this dir,
	// and the outside-read notice for the config dir (ranger-base-d3fwo).
	// Either one missing is a write, and the next test is the half of that
	// pair this one used to hide.
	writeConfig(t, cfg, map[string]any{
		ClaudeOutsideReadSeenKey: true,
		"projects": map[string]any{
			ClaudeTrustKey(dir): map[string]any{"hasTrustDialogAccepted": true},
		},
	})
	before, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	stBefore, _ := os.Stat(cfg)
	time.Sleep(10 * time.Millisecond)

	wrote, err := SeedClaudeTrust(cfg, claudeRuntime(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != "" {
		t.Errorf("seed reported a write (%s) for an already-trusted dir", wrote)
	}
	after, _ := os.ReadFile(cfg)
	stAfter, _ := os.Stat(cfg)
	if string(after) != string(before) || !stAfter.ModTime().Equal(stBefore.ModTime()) {
		t.Error("the config was rewritten with nothing to add")
	}
}

// Claude's own rule, and codex's: a trusted parent does NOT cover a repo
// underneath it. Read it wrong in the permissive direction and the launch
// skips a seed it needed, which is the dialog coming back.
func TestTrustWalkStopsAtTheRepoRoot(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	inner := filepath.Join(repo, "pkg", "sub")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	trustedParent := map[string]any{
		"projects": map[string]any{ClaudeTrustKey(parent): map[string]any{"hasTrustDialogAccepted": true}},
	}
	if ClaudeTrusted(trustedParent, inner) {
		t.Error("a trusted parent must not cover a repo underneath it")
	}
	// Same config, a dir outside any repo: the walk runs past it.
	plain := filepath.Join(parent, "scratch")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if !ClaudeTrusted(trustedParent, plain) {
		t.Error("outside a repo the walk must reach the trusted ancestor")
	}
	// Inside the repo, the repo root's own entry is what counts.
	trustedRepo := map[string]any{
		"projects": map[string]any{ClaudeTrustKey(repo): map[string]any{"hasTrustDialogAccepted": true}},
	}
	if !ClaudeTrusted(trustedRepo, inner) {
		t.Error("the enclosing repo root's entry must cover a dir inside it")
	}
}

// A config posse cannot parse is a config posse does not replace — and it
// is also a config with no trusted directory in it, so the session it was
// about to launch would have opened on the dialog. Refuse, name both ways
// out, leave the bytes alone.
func TestSeedTrustRefusesAnUnparseableConfigWithoutTouchingIt(t *testing.T) {
	t.Parallel()
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	junk := "{ this is not json"
	if err := os.WriteFile(cfg, []byte(junk), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := SeedClaudeTrust(cfg, claudeRuntime(t), t.TempDir())
	if err == nil {
		t.Fatal("want a refusal, got a launch")
	}
	if !strings.Contains(err.Error(), "trust dialog") {
		t.Errorf("the refusal must name the way out by hand: %v", err)
	}
	if b, _ := os.ReadFile(cfg); string(b) != junk {
		t.Error("posse rewrote a config it could not read")
	}
}

// Two opt-outs, both load-bearing. The runtime one keeps posse from
// inventing a config for a CLI whose dialogs it has not measured; the
// empty-path one is what keeps every test backend out of the operator's
// real ~/.claude.json.
func TestSeedTrustOnlySeedsClaudeAndOnlyWhereItWasPointed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"codex", "grok"} {
		rt, err := (&App{}).LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		cfg := filepath.Join(t.TempDir(), ".claude.json")
		if wrote, err := SeedClaudeTrust(cfg, rt, dir); err != nil || wrote != "" {
			t.Errorf("%s: seeded %q (%v) — posse types codex's trust on the line and grok needs none", name, wrote, err)
		}
		if fileExists(cfg) {
			t.Errorf("%s: a claude config was created for another runtime", name)
		}
	}
	if wrote, err := SeedClaudeTrust("", claudeRuntime(t), dir); err != nil || wrote != "" {
		t.Errorf("an unpointed backend must write nothing: %q %v", wrote, err)
	}
}

// The regression this bead is: both paths that type a persona command into
// a pane have to arrive at a promptable screen, so both have to seed —
// `posse new`/dispatch/the cockpit through CreateSession, and the crash
// restart through RelaunchAgent. Same shape as
// TestEveryLaunchPathTypesTheMode, and for the same reason.
func TestEveryLaunchPathSeedsDirectoryTrust(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	cfg := filepath.Join(t.TempDir(), ".claude.json")
	b.ClaudeConfig = cfg
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\ndeny: [Bash(git push:*)]\n---\nYou are developer.\n"), 0o644)

	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "d1", Agent: "developer", Dir: dir})
	if !ClaudeTrusted(readConfig(t, cfg), dir) {
		t.Fatalf("posse new launched into an unanswered trust dialog: %v", readConfig(t, cfg))
	}

	// A relaunch re-types the whole line, so it re-asserts the same fact —
	// against a config that has meanwhile lost the entry (a `claude project
	// purge`, a restored backup).
	writeConfig(t, cfg, map[string]any{"projects": map[string]any{}})
	os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("d1")
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	if ok, err := b.RelaunchAgent("d1", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	if !ClaudeTrusted(readConfig(t, cfg), dir) {
		t.Error("a re-typed line lands in the dialog: relaunch did not re-seed")
	}
}

// A test backend is not pointed at a config, and that is the whole
// protection: `go test` must never be able to add a temp dir to the
// operator's real claude config. Pinned because the seam is invisible —
// the field is empty in a struct literal and nothing else says so.
func TestATestBackendSeedsNothing(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	if b.ClaudeConfig != "" {
		t.Errorf("a test backend must not name a claude config, got %q", b.ClaudeConfig)
	}
	if got := NewHerdrBackend(&App{}).ClaudeConfig; got != ClaudeConfigFile() {
		t.Errorf("a real backend must name the operator's config: got %q, want %q", got, ClaudeConfigFile())
	}
}

// One statement of what posse writes into a claude config, shared with the
// cage's HOME seed — the dialog is the same dialog on both sides of the
// boundary, and a key that drifted on one would be a modal nobody sees.
func TestOneStatementOfWhatPosseSeeds(t *testing.T) {
	t.Parallel()
	state := map[string]any{"projects": map[string]any{
		"/other": map[string]any{"hasTrustDialogAccepted": false},
	}}
	claudeSeedProject(state, "/work")
	projects := state["projects"].(map[string]any)
	if projects["/other"].(map[string]any)["hasTrustDialogAccepted"] != false {
		t.Error("seeding one project rewrote another")
	}
	got := projects["/work"].(map[string]any)
	for _, k := range []string{"hasTrustDialogAccepted", "hasCompletedProjectOnboarding"} {
		if got[k] != true {
			t.Errorf("%s not set: %v", k, got)
		}
	}
}

// `posse runtime check` told an onboarder that posse "names that key and
// never writes it" for every runtime that declared no interstitial. Claude
// now has one posse does write, and the check has to say so — an
// undeclared exception is the kind of thing this table exists to prevent.
func TestClaudeDeclaresTheTrustDialogItSeeds(t *testing.T) {
	t.Parallel()
	rt := claudeRuntime(t)
	if len(rt.Interstitials) == 0 {
		t.Fatal("claude declares no interstitial, so `runtime check` still claims posse writes no such key")
	}
	in := rt.Interstitials[0]
	if !in.Seeded {
		t.Error("the trust dialog is seeded by the launch; the row must say so")
	}
	if !strings.Contains(in.Key, "hasTrustDialogAccepted") {
		t.Errorf("the row names the wrong key: %q", in.Key)
	}
	if in.Probe == nil {
		t.Error("the row needs a probe: an onboarder's question is whether THIS dir is trusted")
	}
	if in.Danger != "" {
		t.Error("Danger means LAUNCH REFUSE until the operator silences it — this one the launch answers")
	}
}
