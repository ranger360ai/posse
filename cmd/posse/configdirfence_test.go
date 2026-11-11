package main

// Every posse verb resolves claude's CONFIG dir before it runs — even
// `posse --help`: herdrback.go's NewHerdrBackend calls ClaudeConfigFile,
// which calls ClaudeConfigDirIn (trust.go). That resolution answers
// $CLAUDE_CONFIG_DIR FIRST and only falls to $HOME/.claude when it is
// unset, so a child launched with append(os.Environ(), "HOME="+tmp) has
// NOT been sandboxed: every posse-dispatched seat exports the variable, so
// on the box this suite actually runs on the operator's own ~/.claude
// walked straight past the sandbox HOME.
//
// MEASURED before this file existed (ranger-base-jxuiy, from -t7hgi): a
// full `go test ./cmd/posse -count=1` with a probe inside
// ClaudeConfigDirIn resolved the config dir 155 times, 126 of them to the
// operator's own live ~/.claude, from 48 tests across 15 files. The 29 that
// landed in a sandbox were costplan_test.go's planEnvAt, the one helper
// that already carried the row.
//
// Nothing READ the operator's data on those 126 — the verbs those tests
// run compute a path and stat two files. What was missing was the fence,
// and the reader is one verb away: `posse runtime check claude` reads
// ~/.claude/.claude.json through this same resolution and its OUTPUT
// changes with operator state. TestClaudeConfigDirIsFenced below is that
// verb, run both ways.
//
// The fence is set here, in TestMain, rather than as a row in each of the
// 18 env-building sites in this package, for two reasons: it also covers
// the sites that fence nothing at all (main_test.go's readyEnv, exactmodel,
// runtimeprobe, runtimeyamlv2, backupsurface all inherit HOME deliberately,
// and $HOME/.claude is the same operator tree by the other branch), and a
// site added tomorrow inherits it without anyone remembering. A test that
// wants the leak overrides it per-test with t.Setenv — costplan_test.go's
// control arm does exactly that.
//
// internal/posse/herdr_test.go's TestMain is the same fence one package
// over, in its other spelling: it replaces HOME for the whole binary and
// then CLEARS the three CLI home overrides, because an empty
// CLAUDE_CONFIG_DIR falls back to a HOME that is already a tempdir. This
// package cannot borrow that spelling — main_test.go's readyEnv keeps the
// operator's HOME deliberately, so that the child abbreviates paths against
// the same $HOME this process does — so the fence has to name a directory.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "posse-configdir-fence-")
	if err != nil {
		panic("config-dir fence: " + err.Error())
	}
	// Set, not appended to some env slice: os.Environ() is what the 18
	// child-env sites in this package's 11 test files build from, so one row
	// here is carried by every child this binary launches.
	if err := os.Setenv("CLAUDE_CONFIG_DIR", dir); err != nil {
		panic("config-dir fence: " + err.Error())
	}
	// The credential locator reads CLAUDE_SECURESTORAGE_CONFIG_DIR FIRST,
	// ahead of the row above (credentialDirNamed, credential.go), so a seat
	// whose launcher exports it walks every child straight past the fence to
	// the operator's real credential store (ranger-base-6sddu). Unset, not
	// "": a present-but-empty value is a third state that shadows
	// CLAUDE_CONFIG_DIR and resolves to $HOME/.claude, which in this package
	// is the operator's. credentialfence_test.go holds this function against
	// credential.go's own list of the variables, so a third name reds here.
	if err := os.Unsetenv("CLAUDE_SECURESTORAGE_CONFIG_DIR"); err != nil {
		panic("config-dir fence: " + err.Error())
	}
	// The cache fence, for the same reason and in the same one place. Since
	// ADR 0080 D11 the plan-usage snapshot and its read log live under
	// `~/.cache/posse/plan-usage` keyed by the config dir above, NOT under
	// $RHQ_HOME/state — so a child that inherits the operator's HOME
	// (main_test.go's readyEnv does, deliberately) appends its read log to
	// a file in the operator's own cache directory. The key is the fence
	// dir's, so it could never be the operator's LIVE snapshot; what it
	// would be is litter in a directory this suite has no business writing
	// to, and the suite slots and bd-write locks are that directory's other
	// tenants. XDG_CACHE_HOME is the resolution's own first branch
	// (planUsageCacheDirIn, bdWriteLockDir, SuiteLockDir), so one row moves
	// all three for every child and for this process alike.
	cache, err := os.MkdirTemp("", "posse-cache-fence-")
	if err != nil {
		panic("cache fence: " + err.Error())
	}
	if err := os.Setenv("XDG_CACHE_HOME", cache); err != nil {
		panic("cache fence: " + err.Error())
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.RemoveAll(cache)
	os.Exit(code)
}

// planFenced gives an App its own plan-usage store, and it is the price of
// the cache fence above rather than a second fence.
//
// Since ADR 0080 D11 the store is keyed by the LOGIN: `<cache>/posse/
// plan-usage/<sha256 of the resolved config dir>.{json,log}`, and this
// binary pins both halves process-wide — one cache root and one config dir
// for every test in it. So every in-process App resolves ONE file, and a
// snapshot one test writes is read by the next: the arm that found this was
// `quiet flag, no snapshot`, which saw a 3-minute-old reading a sibling
// subtest had just seeded.
//
// It is not solved by moving the fences. They are what keeps a child built
// from os.Environ() out of the operator's own `~/.claude` and `~/.cache`,
// and `XDG_CACHE_HOME` has to win over `$HOME` here because it wins over it
// in the shipped resolver (planUsageCacheDirIn, bdWriteLockDir,
// SuiteLockDir). The seam the move built for exactly this is the field:
// empty is the shipped resolution, which is what every real process gets
// and what the out-of-process pins here still measure through their
// children.
func planFenced(t *testing.T, a *posse.App) *posse.App {
	t.Helper()
	a.PlanUsageDir = filepath.Join(t.TempDir(), "plan-usage")
	return a
}

// The pin, two arms one variable apart, on the one verb in this package's
// reach that READS the config dir rather than only resolving it.
//
//	RIGHT  the fence TestMain set        -> "state unknown", naming the fence
//	WRONG  a config dir with a trust row -> "already trusted", naming that dir
//
// The wrong arm is not decoration. Without it, a `runtime check` that
// stopped reading the file at all — a renamed key, a probe dropped from the
// interstitial table — would leave the right arm green while measuring
// nothing. It plants its own fixture rather than pointing at the operator's
// real tree, because a control arm that performs the leak is not a control.
func TestClaudeConfigDirIsFenced(t *testing.T) {
	fence := os.Getenv("CLAUDE_CONFIG_DIR")
	if fence == "" {
		t.Fatal("CLAUDE_CONFIG_DIR is unset in this test binary — TestMain's fence is gone, and every child launched from os.Environ() here resolves the operator's ~/.claude")
	}
	if home, err := os.UserHomeDir(); err == nil && fence == filepath.Join(home, ".claude") {
		t.Fatalf("the fence names the operator's own config dir %s — it fences nothing", fence)
	}

	bin := buildRhq(t)
	home := t.TempDir()
	// The bare shape 15 files in this package use, verbatim: RHQ_HOME and a
	// herdr that must not run, and otherwise whatever the process carries.
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)

	out, code := runRhq(t, bin, env, "runtime", "check", "claude")
	if code != 0 {
		t.Fatalf("posse runtime check claude: exit %d\n%s", code, out)
	}
	// The path is asserted whole and alone: the renderer wraps the ROW, so
	// "unreadable <path>" is split across lines while the path itself is not.
	if !strings.Contains(out, fence) {
		t.Errorf("the trust probe never names the fenced config dir %s — the resolution landed somewhere else:\n%s", fence, out)
	}
	// The PHRASES are asserted against the unwrapped output, because they are
	// several words long and the same wrap that leaves a path intact splits
	// them. wrapGrid (runtimecheck.go) wraps at a fixed 78 columns — not the
	// terminal's width, so this is deterministic — but the row it wraps carries
	// the session dir, so where the break lands moves with the length of the
	// checkout path. That is what born-red meant here: the phrase survived
	// whole in the dispatched seat's worktree and broke after "trusted" in the
	// bare checkout (ranger-base-ikkfn). Collapsing whitespace TIGHTENS this
	// arm — a leaked row whose phrase happened to wrap used to read as absence.
	for _, live := range []string{"is already trusted in", "is already set in"} {
		if strings.Contains(flatten(out), live) {
			t.Errorf("the probe read live operator state (%q) through the fence:\n%s", live, out)
		}
	}

	// WRONG arm: same binary, same env, a config dir this test wrote.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	planted := t.TempDir()
	state, err := json.Marshal(map[string]any{
		"projects": map[string]any{
			posse.ClaudeTrustKey(cwd): map[string]any{"hasTrustDialogAccepted": true},
		},
		posse.ClaudeOutsideReadSeenKey: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planted, ".claude.json"), state, 0o600); err != nil {
		t.Fatal(err)
	}
	out, code = runRhq(t, bin, append(env, "CLAUDE_CONFIG_DIR="+planted), "runtime", "check", "claude")
	if code != 0 {
		t.Fatalf("posse runtime check claude (planted): exit %d\n%s", code, out)
	}
	for _, want := range []string{"is already trusted in", "is already set in", planted} {
		if !strings.Contains(flatten(out), want) {
			t.Errorf("with a planted config dir the probe never printed %q — this verb no longer reads the config dir, so the arm above proves nothing:\n%s", want, out)
		}
	}
}

// flatten undoes the grid's wrapping for phrase matching: wrapGrid never
// breaks a word, it only inserts a newline and an indent BETWEEN words, so
// collapsing every run of whitespace to one space restores the string the
// producer built. Paths stay matchable through it (they are one word), and a
// phrase stops being invisible because of where its row happened to break.
func flatten(s string) string { return strings.Join(strings.Fields(s), " ") }
