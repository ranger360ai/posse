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
	"os/exec"
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
//
// It is one key per repo now rather than a walk (ranger-base-elf2v), so the
// arms below are the same three questions asked of the KEY instead of the
// lookup — plus the arm the old walk got wrong: claude 2.1.288 does not walk
// past a dir that is in no repo either (MEASURED, ClaudeTrustKey's table).
func TestTrustIsPerRepoAndNotPerAncestor(t *testing.T) {
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
	// The arm that changed. A dir in NO repo keys on itself, so a trusted
	// ancestor does not reach it — measured on 2.1.288, where the old walk
	// read this as trusted and the launch then wrote nothing for a `posse
	// new` scratch dir under a parent the operator once accepted by hand.
	plain := filepath.Join(parent, "scratch")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if ClaudeTrusted(trustedParent, plain) {
		t.Error("outside a repo the key is the dir itself; a trusted ancestor must not cover it")
	}
	if !ClaudeTrusted(map[string]any{
		"projects": map[string]any{ClaudeTrustKey(plain): map[string]any{"hasTrustDialogAccepted": true}},
	}, plain) {
		t.Error("outside a repo the dir's own entry must count")
	}
	// Inside the repo, the repo root's own entry is what counts — and it is
	// the key for the subdir too, which is how a subdir is covered at all.
	trustedRepo := map[string]any{
		"projects": map[string]any{ClaudeTrustKey(repo): map[string]any{"hasTrustDialogAccepted": true}},
	}
	if !ClaudeTrusted(trustedRepo, inner) {
		t.Error("the enclosing repo root's entry must cover a dir inside it")
	}
	if got := ClaudeTrustKey(inner); got != ClaudeTrustKey(repo) {
		t.Errorf("a subdir must key on the repo root: got %q, want %q", got, ClaudeTrustKey(repo))
	}
	// There is no "intermediate key" left to get wrong: inside a repo every
	// dir keys on the root, so a key planted on repo/pkg IS the repo's key.
	// That is the line above, said the other way round, and it is why the
	// walk had nothing left to do inside a repo.
	if got, want := ClaudeTrustKey(filepath.Join(repo, "pkg")), ClaudeTrustKey(repo); got != want {
		t.Errorf("a key on an intermediate dir must be the repo's: got %q, want %q", got, want)
	}
}

// ranger-base-elf2v, the bead's own pin: a worktree of an UNTRUSTED main
// repo seeds the main repo's key, and a worktree of a trusted one writes
// nothing. The first half is the dead seat — three HCN seats opened on the
// modal with projects[<worktree>] sitting in the config, true and
// timestamped, because claude looks the worktree up under its main repo.
func TestSeedTrustSeedsTheWorktreesMainRepo(t *testing.T) {
	t.Parallel()
	main, wt := qaLinkedWorktree(t)

	cfg := filepath.Join(t.TempDir(), ".claude.json")
	if _, err := SeedClaudeTrust(cfg, claudeRuntime(t), wt); err != nil {
		t.Fatal(err)
	}
	projects, _ := readConfig(t, cfg)["projects"].(map[string]any)
	mainKey := ClaudeTrustKey(main)
	if e, _ := projects[mainKey].(map[string]any); e["hasTrustDialogAccepted"] != true {
		t.Errorf("the main repo was not seeded: want key %q, got %v", mainKey, keysOf(projects))
	}
	// And only that one. A key on the worktree grants nothing (measured), so
	// writing one is a line of the operator's config that means nothing and
	// one more per session worktree the fleet ever makes.
	if len(projects) != 1 {
		t.Errorf("want exactly the main repo's key, got %v", keysOf(projects))
	}

	// The other half of the pin: the main repo already trusted, so there is
	// nothing to write — which is also why this never showed up until a repo
	// the fleet had not accepted by hand turned up (every control seat in
	// ~/src/posse had no entry of its own and opened on a live composer).
	cfg2 := filepath.Join(t.TempDir(), ".claude.json")
	writeConfig(t, cfg2, map[string]any{
		ClaudeOutsideReadSeenKey: true,
		"projects": map[string]any{
			mainKey: map[string]any{"hasTrustDialogAccepted": true},
		},
	})
	before, err := os.ReadFile(cfg2)
	if err != nil {
		t.Fatal(err)
	}
	wrote, err := SeedClaudeTrust(cfg2, claudeRuntime(t), wt)
	if err != nil {
		t.Fatal(err)
	}
	if wrote != "" {
		t.Errorf("seed reported a write (%s) for a worktree of an already-trusted repo", wrote)
	}
	if after, _ := os.ReadFile(cfg2); string(after) != string(before) {
		t.Error("a worktree of a trusted repo must write nothing")
	}
	if !ClaudeTrusted(readConfig(t, cfg2), wt) {
		t.Error("a trusted main repo must read as trust for its worktree")
	}
}

// The shapes claude does NOT hop on, each measured by asking the CLI which
// key it wants (ClaudeTrustKey's table). These matter in both directions:
// posse bailing where claude hops writes a key claude never reads, and posse
// hopping where claude bails writes the main repo's key while claude asks
// for the worktree's. Both are the same dead seat.
func TestTrustKeyFollowsTheSamePointerChainClaudeDoes(t *testing.T) {
	t.Parallel()
	t.Run("a bare main repo keys on the bare repo dir", func(t *testing.T) {
		t.Parallel()
		root := qaEvalPath(t, t.TempDir())
		bare := filepath.Join(root, "bare.git")
		wt := filepath.Join(root, "wt")
		qaWorktreePointers(t, wt, filepath.Join(bare, "worktrees", "wt"), "../..")
		if got, want := ClaudeTrustKey(wt), bare; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("a submodule keys on itself", func(t *testing.T) {
		t.Parallel()
		root := qaEvalPath(t, t.TempDir())
		mod := filepath.Join(root, "super", "mod")
		// `modules`, not `worktrees`: the layout check is what refuses this.
		qaWorktreePointers(t, mod, filepath.Join(root, "super", ".git", "modules", "mod"), "../..")
		if got, want := ClaudeTrustKey(mod), mod; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("the key is the pointer's spelling, not a resolved one", func(t *testing.T) {
		t.Parallel()
		// The bead's case note: `bd` lists ~/src/hcn, the filesystem spells
		// HCN, and claude's exact-string lookup only matches git's spelling.
		// Claude derives the main repo from the pointer files and never
		// realpaths the answer (MEASURED: its canonical-root function
		// NFC-normalizes and nothing else), so posse must not either — a key
		// posse "corrected" is a key claude never looks up.
		root := qaEvalPath(t, t.TempDir())
		real := filepath.Join(root, "Main")
		link := filepath.Join(root, "main-link")
		if err := os.MkdirAll(filepath.Join(real, ".git", "worktrees", "wt"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
		wt := filepath.Join(root, "wt")
		qaWorktreePointers(t, wt, filepath.Join(link, ".git", "worktrees", "wt"), "../..")
		if got, want := ClaudeTrustKey(wt), link; got != want {
			t.Errorf("got %q, want the pointer's own spelling %q", got, want)
		}
	})
	t.Run("a pointer that does not round-trip keys on the worktree", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		main, wt := qaLinkedWorktree(t)
		stray := filepath.Join(root, "stray")
		if err := os.MkdirAll(stray, 0o755); err != nil {
			t.Fatal(err)
		}
		// Somebody copied the `.git` file. The forward pointer resolves and
		// the layout checks out; the main repo's back-pointer still names
		// the real worktree, so neither posse nor claude follows it.
		b, err := os.ReadFile(filepath.Join(wt, ".git"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stray, ".git"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := ClaudeTrustKey(stray); got == ClaudeTrustKey(main) {
			t.Errorf("a copied .git file must not borrow the main repo's key, got %q", got)
		}
		if got, want := ClaudeTrustKey(stray), qaEvalPath(t, stray); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("a dangling .git symlink is not a root", func(t *testing.T) {
		t.Parallel()
		// MEASURED: claude resolves the entry and takes file-or-directory, so
		// it walks PAST this dir to the enclosing repo. posse stopping here
		// would key a path claude never looks up.
		root := qaEvalPath(t, t.TempDir())
		repo, inner := filepath.Join(root, "repo"), filepath.Join(root, "repo", "dangle")
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(inner, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "nope"), filepath.Join(inner, ".git")); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
		if got, want := ClaudeTrustKey(inner), repo; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("a relative gitdir pointer resolves against the worktree", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		main := filepath.Join(root, "main")
		wt := filepath.Join(root, "wt")
		// What `git worktree add --relative-paths` writes (git 2.48+).
		qaWorktreePointers(t, wt, filepath.Join(main, ".git", "worktrees", "wt"), "../..")
		if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: ../main/.git/worktrees/wt\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, want := ClaudeTrustKey(wt), ClaudeTrustKey(main); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func qaEvalPath(t *testing.T, dir string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(real)
}

// qaWorktreePointers plants the three files git keeps in step for a linked
// worktree — the `.git` file, the git dir's `commondir` and its `gitdir`
// back-pointer — by hand rather than by `git worktree add`, so a test can
// vary one of them. The worktree dir and the git dir are created.
func qaWorktreePointers(t *testing.T, wt, gitdir, commondir string) {
	t.Helper()
	for _, d := range []string{wt, gitdir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, content string) {
		if err := os.WriteFile(p, []byte(content+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(wt, ".git"), "gitdir: "+gitdir)
	write(filepath.Join(gitdir, "commondir"), commondir)
	write(filepath.Join(gitdir, "gitdir"), filepath.Join(qaEvalPath(t, wt), ".git"))
}

// qaLinkedWorktree makes a real `git worktree add` worktree and returns the
// main repo and the worktree. Real git and not planted files, because the
// pointer layout is git's to define and this is the shape the fleet runs in.
//
// It SKIPS only when there is no git at all, and FAILS on anything else git
// says. The tempting shape is a skip per command, and it is the wrong one
// here: this fixture backs the pin for ranger-base-elf2v, a bug whose whole
// cost was a check that said nothing, and a crew seat's PATH carries a `git`
// shim that refuses an unqualified `git commit`. A fixture that skipped on
// that would take the pin with it and still print ok.
func qaLinkedWorktree(t *testing.T) (main, wt string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git on PATH: %v", err)
	}
	// gitTempDir and not t.TempDir: a `git init` leaves read-only object
	// files behind and TempDir's own cleanup reports the removal error
	// (TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot is the pin, and
	// it caught this fixture).
	root := gitTempDir(t)
	main = filepath.Join(root, "main")
	wt = filepath.Join(root, "wt")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=qa", "GIT_AUTHOR_EMAIL=qa@example.invalid",
			"GIT_COMMITTER_NAME=qa", "GIT_COMMITTER_EMAIL=qa@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main", main)
	run("-C", main, "commit", "-q", "--allow-empty", "-m", "init")
	run("-C", main, "worktree", "add", "-q", wt, "-b", "wtbr")
	// The pointer chain the hop reads has to actually be there: a `git
	// worktree add` that "worked" on an unborn HEAD would leave this fixture
	// half-built and the pin asserting about a plain directory.
	if b, err := os.ReadFile(filepath.Join(wt, ".git")); err != nil || !strings.HasPrefix(string(b), "gitdir:") {
		t.Fatalf("%s/.git is not a worktree pointer (%v): %q", wt, err, b)
	}
	return main, wt
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
