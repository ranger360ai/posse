package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ranger-base-txd7 (verify of ranger-base-gyqi): the keyed project-config
// classifier survived every hostile body it was fed, but three of its arms
// were held by nothing — mutation-checked at HEAD, each of these mutations
// left all three packages green before this file existed:
//
//   - delete the `obj == nil` guard (a settings file containing `null`
//     unmarshals to a nil map with NO error, so it read clean),
//   - `os.Lstat` → `os.Stat` (a dangling symlink read clean instead of
//     failing closed, against the comment that says Lstat is deliberate),
//   - force codex through the keyed path (`len(ProjectConfigKeys) == 0` →
//     `false`): a TOML config.toml is invalid JSON, so it still degraded and
//     the existing codex test — which asserts the path and the way out, never
//     the finding — could not tell whole-file presence from key matching.
//
// These are the arms, one assertion each.

// projectConfigVerdict is the classifier's finding for one settings body,
// with the boilerplate either side of it stripped: what the operator is
// told is WHY, and why is what these arms differ in.
func projectConfigVerdict(t *testing.T, why string) string {
	t.Helper()
	if why == "" {
		return ""
	}
	i := strings.Index(why, "turn — ")
	j := strings.Index(why, "; project-owned")
	if i < 0 || j < 0 {
		t.Fatalf("refusal text no longer has a finding clause: %q", why)
	}
	return why[i+len("turn — ") : j]
}

func TestQAProjectConfigTrustClassifiesHostileBodies(t *testing.T) {
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")
	claude, _ := b.App.LoadRuntime("claude")

	const (
		clean     = ""
		nonObject = "project config classification failed: not a top-level JSON object"
		hooksHit  = "matched top-level project config keys: hooks"
	)
	for _, tc := range []struct {
		name, body, want string
	}{
		// The nil-map hole: `null` is valid JSON that unmarshals into a
		// map[string]json.RawMessage without an error, leaving obj nil. The
		// only thing between it and "clean" is the explicit nil check.
		{"json-null", `null`, nonObject},
		{"json-null-padded", "  null\n", nonObject},
		{"json-true", `true`, nonObject},
		{"json-number", `3`, nonObject},
		{"json-string", `"hooks"`, nonObject},
		{"json-array", `[]`, nonObject},
		// An empty file is not a missing file.
		{"empty", ``, "project config classification failed: invalid JSON"},
		{"whitespace-only", "   \n", "project config classification failed: invalid JSON"},
		{"utf8-bom", "\xef\xbb\xbf{\"permissions\":{}}", "project config classification failed: invalid JSON"},
		{"trailing-garbage", `{"permissions":{}} zzz`, "project config classification failed: invalid JSON"},
		{"two-objects", `{"permissions":{}}{"hooks":{}}`, "project config classification failed: invalid JSON"},
		// Key presence is decided after the decoder has unescaped the key,
		// so \u0068ooks is hooks and hiding behind an escape buys nothing.
		{"escaped-key", "{\"\\u0068ooks\":{}}", hooksHit},
		{"duplicate-key", `{"permissions":{},"hooks":1,"hooks":2}`, hooksHit},
		{"both-keys", `{"mcpServers":{},"hooks":{}}`, "matched top-level project config keys: hooks, mcpServers"},
		// …and the other direction: the predicate is TOP-LEVEL key presence.
		// A different case, or the same word nested, is a different key.
		{"wrong-case", `{"Hooks":{}}`, clean},
		{"nested-only", `{"permissions":{"hooks":{}}}`, clean},
		{"permissions-only", `{"permissions":{"allow":["Read"]}}`, clean},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
			if err := os.WriteFile(filepath.Join(repo, ClaudeProjectConfig), []byte(tc.body), 0o644); err != nil {
				t.Fatalf("fixture: %v", err)
			}
			got := projectConfigVerdict(t, ProjectConfigTrust(claude, dev, repo))
			if tc.want == clean {
				if got != "" {
					t.Fatalf("%q must be clean, got %q", tc.body, got)
				}
				return
			}
			if !strings.HasPrefix(got, tc.want) {
				t.Fatalf("%q: want finding %q, got %q", tc.body, tc.want, got)
			}
			// Everything that is not clean refuses the launch by default,
			// naming the same finding the classifier gave.
			err := b.CreateSession(NewSessionOpts{Name: "hb-" + tc.name, Agent: "dev", Runtime: "claude", Dir: repo})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%q must refuse the launch naming %q: %v", tc.body, tc.want, err)
			}
		})
	}
}

// The classifier Lstats before it reads, so an existing-but-unreadable path
// is never confused with a missing one — and a symlink is an existing path
// whatever its target. os.Stat here would call a dangling link "missing".
func TestQAProjectConfigTrustSymlinkedSettingsAreNotMissing(t *testing.T) {
	claude := &Runtime{Name: "claude",
		ProjectConfig:     []string{ClaudeProjectConfig, ClaudeProjectConfigLocal},
		ProjectConfigKeys: []string{"hooks", "mcpServers"}}

	dangling := t.TempDir()
	os.MkdirAll(filepath.Join(dangling, ".claude"), 0o755)
	if err := os.Symlink(filepath.Join(dangling, "gone.json"), filepath.Join(dangling, ClaudeProjectConfig)); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dangling, ClaudeProjectConfig)); err != nil {
		t.Fatalf("fixture is not a symlink: %v", err)
	}
	if got := projectConfigVerdict(t, ProjectConfigTrust(claude, nil, dangling)); !strings.HasPrefix(got, "project config classification failed: unreadable") {
		t.Errorf("a dangling settings symlink must fail closed, not read as missing: %q", got)
	}

	// The control: with no symlink at all the same directory is clean, so
	// the arm above is measuring the link and not the empty .claude dir.
	empty := t.TempDir()
	os.MkdirAll(filepath.Join(empty, ".claude"), 0o755)
	if why := ProjectConfigTrust(claude, nil, empty); why != "" {
		t.Errorf("a directory with no settings file is clean: %q", why)
	}

	// A live link is read through: the executable channel is the target's
	// content, wherever the target lives.
	linked := t.TempDir()
	os.MkdirAll(filepath.Join(linked, ".claude"), 0o755)
	os.WriteFile(filepath.Join(linked, "elsewhere.json"), []byte(`{"hooks":{}}`), 0o644)
	if err := os.Symlink("../elsewhere.json", filepath.Join(linked, ClaudeProjectConfig)); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if got := projectConfigVerdict(t, ProjectConfigTrust(claude, nil, linked)); got != "matched top-level project config keys: hooks" {
		t.Errorf("a settings symlink pointing at hooks must degrade: %q", got)
	}
}

// ADR 0002 amendment 2026-08-26 §2: codex keeps the whole-file predicate —
// "any such file is a hit", because untyped TOML settings are live under
// trust. The existing codex test plants TOML, which is also invalid JSON, so
// it cannot tell the two predicates apart. This body can: it is a readable
// top-level JSON object with neither claude key, which the keyed path calls
// clean and the whole-file path calls present.
func TestQACodexProjectConfigStaysWholeFileNotKeyed(t *testing.T) {
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	dev, _ := b.App.LoadAgent("dev")
	codex, _ := b.App.LoadRuntime("codex")
	if len(codex.ProjectConfigKeys) != 0 {
		t.Fatalf("codex must declare no keys: %v", codex.ProjectConfigKeys)
	}

	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".codex"), 0o755)
	os.WriteFile(filepath.Join(repo, CodexProjectConfig), []byte(`{"permissions":{}}`), 0o644)
	if got := projectConfigVerdict(t, ProjectConfigTrust(codex, dev, repo)); got != "project config is present" {
		t.Errorf("presence alone is the codex predicate, whatever the bytes say: %q", got)
	}
	if err := b.CreateSession(NewSessionOpts{Name: "cx-present", Agent: "dev", Runtime: "codex", Dir: repo}); err == nil ||
		!strings.Contains(err.Error(), "project config is present") {
		t.Errorf("codex must refuse on presence: %v", err)
	}
	// An empty file is the same hit — there is no content test to pass.
	os.WriteFile(filepath.Join(repo, CodexProjectConfig), nil, 0o644)
	if got := projectConfigVerdict(t, ProjectConfigTrust(codex, dev, repo)); got != "project config is present" {
		t.Errorf("an empty codex config is still present: %q", got)
	}
}

// ADR 0002 §4 via the amendment's §5: the check is re-asked on relaunch, and
// planLaunch runs before anything is destroyed — so a repo that grew `hooks`
// while the session was running refuses the refresh with the session still
// alive, rather than killing it and discovering the refusal afterwards.
func TestQAProjectConfigTrustIsRecheckedOnRelaunch(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
	os.WriteFile(filepath.Join(repo, ClaudeProjectConfig), []byte(`{"permissions":{"allow":["Read"]}}`), 0o644)
	mustCreate(t, b, NewSessionOpts{Name: "rl1", Agent: "dev", Runtime: "claude", Dir: repo})
	if m, _ := b.readMeta("rl1"); m == nil || m.Degraded != "" {
		t.Fatalf("permission-only settings launch clean: %+v", m)
	}

	// The repo grows an executable channel under the running session.
	os.WriteFile(filepath.Join(repo, ClaudeProjectConfig),
		[]byte(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"id"}]}]}}`), 0o644)
	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "rl1"})
	if err == nil || !strings.Contains(err.Error(), "matched top-level project config keys: hooks") {
		t.Fatalf("relaunch must re-check the dir and refuse: %v\n%s", err, out.String())
	}
	if !strings.Contains(err.Error(), "was NOT closed") {
		t.Errorf("the refusal must come before the kill: %v", err)
	}
	if m, _ := b.readMeta("rl1"); m == nil {
		t.Errorf("the refused relaunch must leave the session alive")
	}
	if s := out.String(); strings.Contains(s, "killed rl1") {
		t.Errorf("nothing may be destroyed before the re-check: %s", s)
	}
}
