package treepins

// QA pin for ranger-base-l12j9, over THIS repo's own files.
//
// ranger-base-l12j9 declared grok's repo→box project-config channel (eight
// entries, keys `hooks` and `mcpServers` — grokProjectConfig in
// internal/posse/runtime.go carries the 2026-10-31 measurement on grok 1.0.52
// that each entry came from). Declaring a channel makes every launch over it
// DEGRADE, which is the point — and it means a key list drawn one notch wider
// than the measurement would refuse every crew grok seat in this checkout
// instead of refusing an attack.
//
// This repo ships `.claude/settings.json`, permissions-only. That is the file
// the declaration has to keep clean, and this pin reads the REAL one rather
// than a fixture copy of its shape (internal/posse's arm 3 holds the
// fixture). Two ways the pin earns its place:
//
//   - the declaration grows `permissions`, or loses its keys and falls back
//     to the whole-file predicate. Either way every grok launch in this
//     checkout refuses, and the refusal arrives at the next dispatch rather
//     than here.
//   - this repo's own settings file grows `hooks` or `mcpServers`. It is
//     constitution class (ADR 0015 §3) so no persona can commit one, but the
//     operator can — and then every grok seat here degrades, for a real
//     reason, which is exactly the thing an operator should be told at the
//     commit and not at the launch.
//
// It reads eight paths BY NAME and enumerates nothing, so it is not in the
// tree-wide pin class and carries no Makefile door of its own (the
// enumeration rule's negative case — see treewidedoor_qa_test.go).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestQAThisRepoKeepsAGrokLaunchCleanOfProjectConfig(t *testing.T) {
	t.Parallel()
	// The test binary's cwd is the repo root (TestMain), so the session dir
	// this check is asked about IS this checkout.
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	app := &posse.App{StateDir: filepath.Join(t.TempDir(), "state")}
	grok, err := app.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	if len(grok.ProjectConfig) == 0 {
		t.Fatal("grok declares no project-config channel — ranger-base-l12j9 declared one, and an undeclared channel is the silent state ProjectConfigTrust skips entirely (runtimecheck.go projectConfigRow)")
	}

	// The arm has to be able to fail, so the file it is about must be there
	// and must be the shape the claim is about: readable JSON, with
	// permissions and without either exec key.
	body, err := os.ReadFile(filepath.FromSlash(posse.ClaudeProjectConfig))
	if err != nil {
		t.Fatalf("this repo's %s is unreadable, so this pin would measure nothing: %v", posse.ClaudeProjectConfig, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("this repo's %s is not a top-level JSON object — which would FAIL CLOSED and degrade every grok and claude launch here: %v", posse.ClaudeProjectConfig, err)
	}
	if _, ok := obj["permissions"]; !ok {
		t.Errorf("this repo's %s declares no permissions — the arm below would then be clean for the wrong reason", posse.ClaudeProjectConfig)
	}
	for _, k := range grok.ProjectConfigKeys {
		if _, ok := obj[k]; ok {
			t.Errorf("this repo's %s now declares %q, a key grok RUNS or SPAWNS before any turn under folder trust — every grok seat in this checkout degrades until it is removed or a PID opts in with trust_project_config: true (ADR 0002; it is constitution class, so this is the operator's own hand)", posse.ClaudeProjectConfig, k)
		}
	}

	// The whole verdict, over the real directory: nothing posse ships in its
	// own checkout may degrade a grok launch.
	if why := posse.ProjectConfigTrust(grok, nil, root); why != "" {
		t.Errorf("this checkout degrades a grok launch on its own files: %s", why)
	}
	// Said once more for the runtimes that share entries with grok's list,
	// so the one that first grows a file here is named whichever it is.
	for _, name := range []string{"claude", "codex", "bob"} {
		rt, err := app.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		if why := posse.ProjectConfigTrust(rt, nil, root); why != "" {
			t.Errorf("this checkout degrades a %s launch on its own files: %s", name, why)
		}
	}

	// And the other direction: of grok's eight entries this repo carries
	// exactly the one above, so the clean verdict is a fact about the files
	// and not about a check that stopped looking.
	var present []string
	for _, rel := range grok.ProjectConfig {
		if _, err := os.Lstat(filepath.FromSlash(rel)); err == nil {
			present = append(present, rel)
		}
	}
	if strings.Join(present, " ") != posse.ClaudeProjectConfig {
		t.Errorf("this repo carries %v of grok's declared project-config entries; only %s is expected, and anything else here is a repo→box channel this checkout did not have when the declaration was measured", present, posse.ClaudeProjectConfig)
	}
}
