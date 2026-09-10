package treepins

// Every posse verb resolves claude's CONFIG dir before it runs — even
// `posse --help`: herdrback.go's NewHerdrBackend calls ClaudeConfigFile,
// which calls ClaudeConfigDirIn (trust.go). That resolution answers
// $CLAUDE_CONFIG_DIR FIRST and only falls to $HOME/.claude when it is
// unset, so hookfreshness_qa_test.go:130's
// append(os.Environ(), "HOME="+r.home, "RHQ_HOME="+r.rhqHome) fences HOME
// but not CLAUDE_CONFIG_DIR — every posse-dispatched seat exports it, so on
// the box this suite actually runs on, the child resolves the operator's
// own live ~/.claude straight past the sandbox HOME.
//
// MEASURED (ranger-base-773pj, from -scts5's sibling finding): a probe
// inside ClaudeConfigDirIn during
// `go test . -run TestQAHookFreshnessFreshBoxPasses -count=1` resolved the
// config dir twice, both to the operator's live ~/.claude.
//
// Nothing READ the operator's data through that resolution — the verb it
// runs (`gates install-hooks`) computes a path and stats two files. What was
// missing was the fence, and the reader is one verb away: `posse runtime
// check claude` reads ~/.claude/.claude.json through this same resolution
// and its OUTPUT changes with operator state. TestClaudeConfigDirIsFenced
// below is that verb, run both ways — the same pin cmd/posse's sibling file
// carries for that package.
//
// The fence is set here, in TestMain, rather than as a row in
// hookfreshness_qa_test.go:130 alone, for the same two reasons as
// cmd/posse/configdirfence_test.go: it also covers any site added tomorrow
// that builds env from os.Environ() the same way, and a test that wants the
// leak can still override it per-test with t.Setenv.
//
// This package cannot borrow internal/posse/herdr_test.go's spelling
// (replace HOME for the whole binary, then clear the fallback) because it
// does not replace HOME for the whole binary — quickstart_test.go and
// others in this package deliberately keep the operator's real HOME — so
// the fence has to name a directory, the same as cmd/posse's TestMain.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestMain(m *testing.M) {
	// All tree-relative reads, stats, walks and inherited exec working
	// directories resolve through the same root, before any parallel test.
	// Fixtures that set cmd.Dir explicitly retain their own working directory.
	root, err := repoRoot()
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(root); err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "posse-configdir-fence-")
	if err != nil {
		panic("config-dir fence: " + err.Error())
	}
	// Set, not appended to some env slice: os.Environ() is what every
	// os.Environ()-based child-env site in this package's test files builds
	// from, so one row here is carried by every child this binary launches.
	if err := os.Setenv("CLAUDE_CONFIG_DIR", dir); err != nil {
		panic("config-dir fence: " + err.Error())
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// The pin, two arms one variable apart, on the one verb in this package's
// reach that READS the config dir rather than only resolving it. See
// cmd/posse/configdirfence_test.go's TestClaudeConfigDirIsFenced — this is
// the same pin, one package over.
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
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)

	out, code := runRhq(t, bin, env, "runtime", "check", "claude")
	if code != 0 {
		t.Fatalf("posse runtime check claude: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, fence) {
		t.Errorf("the trust probe never names the fenced config dir %s — the resolution landed somewhere else:\n%s", fence, out)
	}
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

func buildRhq(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runRhq(t *testing.T, bin string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse %s: %v", strings.Join(args, " "), err)
	}
	return string(out), code
}

// flatten undoes the grid's wrapping for phrase matching: wrapGrid never
// breaks a word, it only inserts a newline and an indent BETWEEN words, so
// collapsing every run of whitespace to one space restores the string the
// producer built. Paths stay matchable through it (they are one word), and a
// phrase stops being invisible because of where its row happened to break.
func flatten(s string) string { return strings.Join(strings.Fields(s), " ") }
