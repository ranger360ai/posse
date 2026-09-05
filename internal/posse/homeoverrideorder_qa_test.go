package posse

// ranger-base-v9ptt, verifying ranger-base-pghf4: the ORDER of the two arms
// the hand-merge composed, which was the one claim that merge made and the
// one claim nothing held.
//
// ranger-base-58b5 gave grokHomeIn/codexHomeIn an empty-home arm (Join drops
// an empty element, so a home read from an unset $HOME becomes a path under
// the cwd). ranger-base-z65xu gave them a $GROK_HOME/$CODEX_HOME arm. The
// merge's whole content is that the OVERRIDE is asked FIRST: a CLI home the
// operator named by hand is a home whatever $HOME says, and an empty $HOME
// is not a reason to stop reading it.
//
// Measured unpinned 2026-09-05 (ranger-base-v9ptt): hoisting the `home == ""`
// check above the override in either resolver survived the whole tree, and a
// census of every test that sets GROK_HOME/CODEX_HOME against every test that
// empties $HOME found no overlap — nohomeprobe_qa_test.go sets HOME and both
// overrides to "" together, so both orders answer "" there and the arm is
// invisible. That mutant is a live regression: `env -i RHQ_HOME=... \
// GROK_HOME=... posse doctor` reads a home the operator named and would
// report UNKNOWN for it.
//
// The fixture is three-way on purpose, so each wrong order fails differently
// rather than all of them collapsing into one "not silenced":
//
//	the override home says SILENCED  -> the right answer
//	the cwd says NOT silenced        -> an override arm that was skipped
//	no home at all says UNKNOWN      -> an empty-home arm that ran first

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestACLIHomeOverrideOutranksAnEmptyHOME(t *testing.T) {
	// The cwd is the decoy, and it answers every key the probes ask for —
	// in the OPPOSITE direction, so a reader that lands here is not merely
	// wrong, it is distinguishable from a reader that read nothing.
	cwd := t.TempDir()
	for _, f := range []struct{ path, body string }{
		// Where a resolver that skipped the override and joined onto an
		// empty home lands: Join("", ".grok") is ".grok".
		{".grok/config.toml", "auto_update = true\n"},
		{".codex/config.toml", "check_for_update_on_startup = true\n"},
		// And where one that returned the empty home itself lands:
		// Join("", "config.toml") is "config.toml".
		{"config.toml", "auto_update = true\ncheck_for_update_on_startup = true\n"},
	} {
		p := filepath.Join(cwd, f.path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(cwd)

	// The homes the operator named by hand. Outside the cwd, so nothing
	// here can be reached by accident from a relative path.
	grokDir := t.TempDir()
	codexDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(grokDir, "config.toml"),
		[]byte("privacy_banner_acked = \"2026-08-24T21:35:58Z\"\nauto_update = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"),
		[]byte("check_for_update_on_startup = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Empty and not merely absent, which is what os.Getenv can tell apart
	// from neither: the resolvers read the string.
	t.Setenv("HOME", "")
	t.Setenv("GROK_HOME", grokDir)
	t.Setenv("CODEX_HOME", codexDir)

	for _, tc := range []struct {
		name, got, want string
	}{
		{"grokHome", grokHome(), grokDir},
		{"codexHome", codexHome(), codexDir},
	} {
		if tc.got != tc.want {
			t.Errorf("%s() with $HOME empty and the CLI's own override set = %q, want %q — an override names a home whatever $HOME says, and asking the empty-home arm first throws it away",
				tc.name, tc.got, tc.want)
		}
	}

	// The resolvers are the mechanism; these are the reading an operator
	// actually gets, and the reason the order matters at all.
	for _, tc := range []struct {
		name string
		sil  Silence
		home string
	}{
		{"grokPrivacyProbe", grokPrivacyProbe(), grokDir},
		{"grokAutoUpdateProbe", grokAutoUpdateProbe(), grokDir},
		{"codexUpdateProbe", codexUpdateProbe(), codexDir},
	} {
		if tc.sil.Unknown {
			t.Errorf("%s with $HOME empty but the override set: UNKNOWN (why=%q) — the operator named a home and it was readable; unknown is the answer for having no home at all, not for having one $HOME did not supply",
				tc.name, tc.sil.Why)
			continue
		}
		if !tc.sil.Silenced {
			t.Errorf("%s with the override set: Silenced=false why=%q — the override home says silenced and the cwd decoy says the opposite, so this read the cwd",
				tc.name, tc.sil.Why)
		}
		// AbbrevHome cannot shorten these: $HOME is empty, so the why
		// carries the override path whole.
		if !strings.Contains(tc.sil.Why, tc.home) {
			t.Errorf("%s: why=%q names neither the home it was told to read (%s) — the path printed has to be the path read",
				tc.name, tc.sil.Why, tc.home)
		}
	}

	// The third reader of the same class (ranger-base-58b5's other site).
	// ClaudeConfigFile spells the rule differently — one early return for
	// BOTH names empty, rather than an empty-home arm per resolver — so it
	// is structurally right where the other two were right by ordering. It
	// is pinned here anyway because it is the same operator-visible claim
	// and nothing else asks it: nohomeprobe_qa_test.go empties
	// CLAUDE_CONFIG_DIR alongside $HOME, so it only ever measures the
	// both-empty arm.
	//
	// A NON-EMPTY override deliberately, which is what keeps this clear of
	// ranger-base-e9xba (open): that bead is the present-but-EMPTY
	// CLAUDE_CONFIG_DIR question in ClaudeConfigDirIn, and it resolves
	// either way without meeting this arm.
	claudeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(claudeDir, ".config.json"), []byte(`{"projects":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	if got, want := ClaudeConfigFile(), filepath.Join(claudeDir, ".config.json"); got != want {
		t.Errorf("ClaudeConfigFile() with $HOME empty and CLAUDE_CONFIG_DIR set = %q, want %q — an override names a home whatever $HOME says", got, want)
	}
	if sil := claudeTrustProbe(); sil.Unknown {
		t.Errorf("claudeTrustProbe with $HOME empty but CLAUDE_CONFIG_DIR set: UNKNOWN (why=%q) — the operator named a config dir and it was readable", sil.Why)
	}
}
