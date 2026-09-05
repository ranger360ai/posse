package posse

// ranger-base-58b5: the $HOME callers that build a path with
// filepath.Join(os.Getenv("HOME"), ...). Join DROPS an empty element, so
// with no HOME the result is RELATIVE and resolves against whatever cwd the
// process happens to have — a different defect from ranger-base-a3t1's, and
// a worse one: a3t1's two helpers invented a home at the filesystem ROOT,
// these re-root it at the cwd, where a repo's own dotfiles are.
//
// The fixture IS the defect. The scratch tree carries exactly the files
// these readers name, filled with answers no operator gave, and every
// assertion here is that posse reports UNKNOWN rather than that tree's
// answers as the operator's.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoHomeReadsNoCWDRelativeConfig(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []struct{ path, body string }{
		// Both grok keys say SILENCED, so a probe that reads this tree
		// answers "silenced" — the false clean, not a false alarm.
		{".grok/config.toml", "privacy_banner_acked = \"2026-08-24T21:35:58Z\"\nauto_update = false\n"},
		{".codex/config.toml", "check_for_update_on_startup = false\n"},
		{".codex/version.json", `{"latest_version":"0.1.0","dismissed_version":"0.1.0"}`},
		// Parseable and trusting nothing, so the claude probes read it as a
		// live answer about the operator rather than as unreadable.
		{".claude.json", `{"projects":{}}`},
		{".claude/.config.json", `{"projects":{}}`},
		// The tree's ROOT, not a dotdir: Join("", "config.toml") is
		// "config.toml", so a reader whose home came back empty and joined
		// onto it anyway lands here. A repo root holding a config.toml is
		// not exotic, and this one answers every key a probe asks for.
		{"config.toml", "privacy_banner_acked = \"2026-08-24T21:35:58Z\"\nauto_update = false\ncheck_for_update_on_startup = false\n"},
		{"version.json", `{"latest_version":"0.1.0","dismissed_version":"0.1.0"}`},
	} {
		p := filepath.Join(dir, f.path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	// Empty, not merely absent: os.Getenv cannot tell the two apart, and an
	// exported-but-empty HOME is the reachable half anyway.
	t.Setenv("HOME", "")
	t.Setenv("GROK_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	for _, tc := range []struct {
		name string
		got  string
	}{
		{"grokHome", grokHome()},
		{"codexHome", codexHome()},
		{"ClaudeConfigFile", ClaudeConfigFile()},
	} {
		if tc.got != "" {
			t.Errorf("%s() with no home = %q, want \"\" — a path that is not under a home is a path under the cwd", tc.name, tc.got)
		}
	}

	for _, tc := range []struct {
		name string
		sil  Silence
	}{
		{"grokPrivacyProbe", grokPrivacyProbe()},
		{"grokAutoUpdateProbe", grokAutoUpdateProbe()},
		{"codexUpdateProbe", codexUpdateProbe()},
		{"claudeTrustProbe", claudeTrustProbe()},
		{"claudeOutsideReadProbe", claudeOutsideReadProbe()},
	} {
		if !tc.sil.Unknown || tc.sil.Silenced {
			t.Errorf("%s with no home: Unknown=%v Silenced=%v why=%q — want unknown and not silenced, because the only file it could have read is the cwd's",
				tc.name, tc.sil.Unknown, tc.sil.Silenced, tc.sil.Why)
		}
		// And the reading has to name what it could not read. An unknown
		// reached by joining onto an empty home and failing prints its
		// blank path back at the onboarder ("unreadable  — cannot tell
		// whether this directory is trusted"), which is the right verdict
		// arrived at by the wrong route and worded as a truncated line.
		if !strings.Contains(tc.sil.Why, "$HOME") {
			t.Errorf("%s with no home: why=%q — an unknown must say the home is missing, not print a blank path", tc.name, tc.sil.Why)
		}
	}

	// The seed is the WRITE half, and it is the one that would have put the
	// operator's trusted directory into a repo. SeedClaudeTrust already
	// documents cfg == "" as a no-op and not a fallback; this is that
	// contract being what catches an empty home. The census is the
	// assertion, not the return value: pre-fix the seed takes a lock
	// (<cfg>.posse-lock), rewrites the config through a temp file and a
	// rename, and every one of those lands in the cwd.
	before := treeCensus(t, dir)
	if got, err := SeedClaudeTrust(ClaudeConfigFile(), &Runtime{Name: "claude"}, dir); err != nil || got != "" {
		t.Errorf("SeedClaudeTrust with no home = (%q, %v), want (\"\", nil) — the empty case is a no-op", got, err)
	}
	for path, sum := range treeCensus(t, dir) {
		if was, ok := before[path]; !ok {
			t.Errorf("no home, but posse created %s under the cwd", path)
		} else if was != sum {
			t.Errorf("no home, but posse rewrote %s under the cwd", path)
		}
	}
}

// treeCensus is every file under root, by relative path and content. Content
// and not mtime: the seed writes through a temp file and a rename, so the
// question is what the bytes say, and a rename onto an identical file would
// otherwise read as a change.
func treeCensus(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
