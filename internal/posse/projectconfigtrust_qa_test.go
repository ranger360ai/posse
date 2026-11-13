//go:build posse_arm2

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// ADR 0091 D1/D3's NEGATIVE CONTROLS (ranger-base-f24yz). D1 exempts an entry
// that resolves to a declared executable member of a CLI home, by EQUALITY.
// These are the neighbours of such a member, and each one still classifies
// exactly as it did — each names the process it protects, which is the one
// this check was always about: the classifier reading REPO-WRITTEN executable
// config.
//
// HELD BY NOTHING OTHERWISE, and that is MEASURED and not argued — three
// mutations at HEAD, 2026-11-12, linux 7.2.15, go1.27.3:
//
//	mutation                                  arm 1 census/verdict   this test
//	D1's skip deleted (the ADR's exit hatch)   RED, both pins         RED
//	samePath -> samePath || underDir(home)     green                  RED (a, d)
//	keyed on samePath(dir, wallHome())         green                  RED (b, d)
//
// So the f67jx pins in arm 1 do hold D1 itself — they are what reds when the
// exit hatch is taken — and they hold NOTHING about its width: every one of
// their ten rows IS a member, so a census of members cannot tell a correct
// exemption from an over-wide one. That is this test's whole job.
func TestQAProjectConfigTrustExemptsOnlyTheHomeMemberItself(t *testing.T) {
	// No t.Parallel: the exemption resolves against wallHome() and the CLI
	// home variables, which this sets.
	root := t.TempDir()
	home := filepath.Join(root, "home")
	for _, d := range []string{
		filepath.Join(home, ".claude"), filepath.Join(home, ".codex"), filepath.Join(home, ".grok"),
		filepath.Join(root, "posse", "state"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv(CodexHomeVar, "")
	t.Setenv(GrokHomeVar, "")
	t.Setenv(ClaudeConfigDirVar, filepath.Join(home, ".claude"))

	a := &App{Home: filepath.Join(root, "posse"), StateDir: filepath.Join(root, "posse", "state")}
	ag := &AgentFile{Name: "dev"}
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	grok, err := a.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}

	// THE POSITIVE CONTROL every arm below is measured against: a ZERO-BYTE
	// settings.json at the member itself. That body is `invalid JSON` to the
	// classifier everywhere else in this file (the txd7 pin's "empty" row),
	// and a launch in the home is clean on it — so the arms below are reading
	// D1's equality and not some property of the bytes.
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rt := range []*Runtime{claude, grok} {
		if why := ProjectConfigTrust(rt, ag, home); why != "" {
			t.Fatalf("%s: the member itself must be exempt (ADR 0091 D1): %s", rt.Name, why)
		}
	}

	// (a) A `.claude` INSIDE a CLI home. `dir=~/.claude` resolves claude's
	// own declared entry to `~/.claude/.claude/settings.json`: inside a home,
	// named by no carve, and writable by any seat whose state grant reaches
	// the config dir — so it is a live repo→box channel and classifies as
	// before. This is the arm an `underDir` exemption would have lost, and
	// the example the ADR spells out in D3.
	t.Run("a-claude-inside-a-claude-is-no-member", func(t *testing.T) {
		inner := filepath.Join(home, ".claude", ".claude")
		if err := os.MkdirAll(inner, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inner, "settings.json"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, rt := range []*Runtime{claude, grok} {
			got := projectConfigVerdict(t, ProjectConfigTrust(rt, ag, filepath.Join(home, ".claude")))
			if !strings.HasPrefix(got, "project config classification failed: invalid JSON") {
				t.Errorf("%s launching in ~/.claude: want invalid JSON over the inner settings.json, got %q", rt.Name, got)
			}
		}
	})

	// (b) `$HOME/.mcp.json`. It is in NO CLI home and covered by NO carve, so
	// a seat with a home grant can write it and a `$HOME` launch still
	// degrades on it. This is the arm the rejected `dir == $HOME` exemption
	// would have lost (ADR 0091, alternative 2, which also names
	// `$HOME/.cursor/*`).
	t.Run("b-home-mcp-json-is-in-no-home-and-no-carve", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(home, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := projectConfigVerdict(t, ProjectConfigTrust(grok, ag, home)); got != "matched top-level project config keys: mcpServers" {
			t.Errorf("$HOME/.mcp.json must still degrade a grok launch in $HOME: %q", got)
		}
		// The contrast that says this is grok's DECLARED LIST talking and not
		// the directory: claude names no `.mcp.json` entry, so the same file
		// leaves a claude launch in the same dir clean.
		if why := ProjectConfigTrust(claude, ag, home); why != "" {
			t.Errorf("claude declares no .mcp.json entry: %s", why)
		}
	})

	// (c) A REPO's own `.grok/config.toml`, outside every home. The writer
	// here is the repo's committer, which is the threat the whole check is
	// for, and the exemption must not reach it by sharing the entry's name
	// with a member.
	t.Run("c-a-repos-own-grok-config-is-no-member", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".grok"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".grok", "config.toml"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := projectConfigVerdict(t, ProjectConfigTrust(grok, ag, repo)); !strings.HasPrefix(got, "project config classification failed: invalid JSON") {
			t.Errorf("a repo's own .grok/config.toml must still degrade: %q", got)
		}
	})

	// (d) The one case the ADR ACCEPTS, pinned so it is a decision and not a
	// surprise: a repo may symlink its entry AT a home member, samePath
	// follows the link, and the entry is exempt. The repo supplies a pointer
	// and the operator supplies the content; a repo cannot write the target.
	// Its control is the same link pointing at a home path that is NOT a
	// member, which degrades — so what the link buys is the member and not
	// the home.
	t.Run("d-a-link-at-a-member-is-exempt-and-only-at-a-member", func(t *testing.T) {
		target := filepath.Join(home, ".grok", "config.toml")
		if err := os.WriteFile(target, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		notMember := filepath.Join(home, ".grok", "bundles.d", "config.toml")
		if err := os.MkdirAll(filepath.Dir(notMember), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(notMember, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ name, at, want string }{
			{"at-the-member", target, ""},
			{"at-a-home-path-no-carve-names", notMember, "project config classification failed: invalid JSON"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				repo := t.TempDir()
				if err := os.MkdirAll(filepath.Join(repo, ".grok"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(tc.at, filepath.Join(repo, ".grok", "config.toml")); err != nil {
					t.Fatal(err)
				}
				got := projectConfigVerdict(t, ProjectConfigTrust(grok, ag, repo))
				if tc.want == "" {
					if got != "" {
						t.Errorf("a link at %s must be exempt: %q", AbbrevHome(tc.at), got)
					}
					return
				}
				if !strings.HasPrefix(got, tc.want) {
					t.Errorf("a link at %s: want %q, got %q", AbbrevHome(tc.at), tc.want, got)
				}
			})
		}
	})
}

// ranger-base-afqqz (finding ranger-base-lu513): D1's exemption must take no
// members from a CLI home whose variable the operator typed RELATIVE.
// runtimeHomeOverride absolutizes such a value against posse's own cwd, and
// an operator who types `posse <persona>` in a repo has cwd == session dir —
// so `CLAUDE_CONFIG_DIR=.claude` made the REPO's .claude/settings.json a
// "member" and the committer's hook was never reported. The arms set the
// same cwd and the same dirs and differ only in how the variable is spelled:
// absolute is the operator choosing that dir (exempt, D1 as written);
// relative is the cwd choosing it (classified).
func TestQAProjectConfigTrustTakesNoMembersFromARelativeHomeVariable(t *testing.T) {
	// No t.Parallel: t.Chdir, and the exemption reads HOME and the home
	// variables.
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	for _, d := range []string{
		home, filepath.Join(repo, ".claude"), filepath.Join(repo, ".grok"),
		filepath.Join(repo, ".codex"), filepath.Join(root, "posse", "state"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"),
		[]byte(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"touch /tmp/pwned"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".grok", "config.toml"),
		[]byte("[mcp_servers.x]\ncommand = \"touch /tmp/pwned\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".codex", "config.toml"),
		[]byte("[mcp_servers.x]\ncommand = \"touch /tmp/pwned\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Chdir(repo)

	a := &App{Home: filepath.Join(root, "posse"), StateDir: filepath.Join(root, "posse", "state")}
	ag := &AgentFile{Name: "dev"}
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	grok, err := a.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	// codex is the third home the filter covers and was the one this pin
	// did not exercise, so a member row tagged with the wrong home variable
	// there reddened nothing (ranger-base-d78w3, verifying the close).
	codex, err := a.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name                     string
		claudeDir, gHome, cxHome string
		exempt                   bool
	}{
		// The control: the same dirs named absolute ARE the operator's
		// homes, and D1 exempts their members — which is what makes the
		// relative arm a test of the spelling and not of the bytes.
		{"absolute-is-the-operators-choice", filepath.Join(repo, ".claude"), filepath.Join(repo, ".grok"), filepath.Join(repo, ".codex"), true},
		{"relative-is-the-cwds-choice", ".claude", ".grok", ".codex", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(ClaudeConfigDirVar, tc.claudeDir)
			t.Setenv(GrokHomeVar, tc.gHome)
			t.Setenv(CodexHomeVar, tc.cxHome)
			members := trustExemptCLIHomeExecMembers(wallHome())
			for _, c := range []struct {
				rt   *Runtime
				rel  string
				want string
			}{
				{claude, ".claude/settings.json", "matched top-level project config keys: hooks"},
				{grok, ".claude/settings.json", "matched top-level project config keys: hooks"},
				// want "" = any finding: grok's classifier fails this file closed
				// today, and what is pinned here is that it is READ, not how.
				{grok, ".grok/config.toml", ""},
				{codex, ".codex/config.toml", ""},
			} {
				got := projectConfigVerdict(t, projectConfigTrustEntry(c.rt, repo, c.rel, members))
				switch {
				case tc.exempt && got != "":
					t.Errorf("%s %s: an absolute home's member must be exempt (D1): %q", c.rt.Name, c.rel, got)
				case !tc.exempt && (got == "" || c.want != "" && got != c.want):
					t.Errorf("%s %s under a relative home variable: want a finding (%q), got %q — the repo's own file was exempted as a home member", c.rt.Name, c.rel, c.want, got)
				}
			}
			// And the launch-level answer, the one the operator sees.
			for _, rt := range []*Runtime{claude, grok, codex} {
				why := ProjectConfigTrust(rt, ag, repo)
				if (why == "") != tc.exempt {
					t.Errorf("%s: ProjectConfigTrust in the repo = %q, exempt want %v", rt.Name, why, tc.exempt)
				}
			}
		})
	}
}

// ranger-base-rgyto (finding from ranger-base-h0lla's verify): the WHOLE-FILE
// branch asks the same presence question the keyed branch asks.
//
// Both branches are now one hoisted os.Lstat. Before it, the whole-file
// branch decided presence with os.Stat — which FOLLOWS the link — and
// returned clean when that stat failed, so a DANGLING SYMLINK at a
// whole-file entry got the same "" an empty session dir gets, while the
// keyed branch degraded on exactly that shape (the arm above). The field is
// documented as "presence is the predicate" and a dangling link is present:
// Lstat answers for it without reading anything. MEASURED 2026-11-13 at the
// commit before this one (linux 7.2.15 x86_64, go1.27.3): dangling →
// "" CLEAN for .claude/scheduled_tasks.json and .grok/lsp.json, against
// "unreadable" for .claude/settings.json.
//
// Mutation-checked at this commit (2026-11-13, linux 7.2.15, go1.27.3):
// `os.Lstat` → `os.Stat` in projectConfigTrustFile's hoisted presence check
// reds BOTH whole-file rows here AND the keyed arm above. That is the hoist
// saying so — there is one presence check now, so one mutation reaches both
// branches, and before the hoist the same mutation reddened only the keyed
// arm while these two rows did not exist to red.
//
// The entries are read off the runtimes rather than spelled, and BOTH
// whole-file entries in the tree are exercised — grok's `.grok/lsp.json` is
// what says this is about the class (ranger-base-2zkcy, which introduced it)
// and not about claude's cron file (ranger-base-g3pfl, which doubled it).
func TestQAProjectConfigTrustWholeFileDanglingSymlinkIsPresent(t *testing.T) {
	t.Parallel()
	a := &App{Home: t.TempDir(), StateDir: t.TempDir()}

	type entry struct {
		rt  *Runtime
		rel string
	}
	var entries []entry
	for _, name := range []string{"claude", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if len(rt.ProjectConfigWholeFile) == 0 {
			t.Fatalf("%s declares no whole-file entry — the exemption this pin is about is gone", name)
		}
		for _, rel := range rt.ProjectConfigWholeFile {
			entries = append(entries, entry{rt, rel})
		}
	}
	if len(entries) < 2 {
		t.Fatalf("both whole-file entries in the tree must be here, got %d", len(entries))
	}

	// plant builds a session dir holding ONE whole-file entry in the shape
	// the row names, and returns the launch-level finding for it.
	plant := func(t *testing.T, e entry, shape func(p string)) string {
		t.Helper()
		dir := t.TempDir()
		p := filepath.Join(dir, filepath.FromSlash(e.rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if shape != nil {
			shape(p)
		}
		return projectConfigVerdict(t, ProjectConfigTrust(e.rt, nil, dir))
	}

	const present = "project config is present"
	for _, e := range entries {
		t.Run(e.rt.Name+"-"+strings.ReplaceAll(e.rel, "/", "-"), func(t *testing.T) {
			// THE ROW THIS PIN EXISTS FOR.
			got := plant(t, e, func(p string) {
				if err := os.Symlink(filepath.Join(filepath.Dir(p), "no-such-target.json"), p); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(p); err != nil {
					t.Fatalf("fixture is not a symlink: %v", err)
				}
				if _, err := os.Stat(p); err == nil {
					t.Fatal("fixture symlink is not dangling")
				}
			})
			if got != present {
				t.Errorf("a dangling %s must be present, not clean: %q", e.rel, got)
			}

			// The control: with nothing planted the same dir is clean, so
			// the row above is measuring the link and not the dir.
			if got := plant(t, e, nil); got != "" {
				t.Errorf("a dir with no %s is clean: %q", e.rel, got)
			}

			// And the shapes that already degraded still do, by the same
			// finding — the fix moved one row of the table, not the branch.
			for _, tc := range []struct {
				name  string
				shape func(p string)
			}{
				{"symlink-to-a-body", func(p string) {
					target := filepath.Join(filepath.Dir(p), "elsewhere.json")
					if err := os.WriteFile(target, []byte(`{}`), 0o644); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, p); err != nil {
						t.Fatal(err)
					}
				}},
				{"directory", func(p string) {
					if err := os.MkdirAll(p, 0o755); err != nil {
						t.Fatal(err)
					}
				}},
				{"regular-file", func(p string) {
					if err := os.WriteFile(p, []byte(`{"anything":{}}`), 0o644); err != nil {
						t.Fatal(err)
					}
				}},
			} {
				if got := plant(t, e, tc.shape); got != present {
					t.Errorf("%s as a %s must be present: %q", e.rel, tc.name, got)
				}
			}
		})
	}
}

// ranger-base-9n98o (finding from ranger-base-obato's verify of
// ranger-base-rgyto): the hoisted presence check FAILS CLOSED, and until this
// arm existed that half of it was held by nothing.
//
// ranger-base-rgyto hoisted the Lstat above the branch and is well pinned for
// the row it MOVED — a dangling symlink at a whole-file entry (the arm
// above), where every mutant including a full revert of the hoist reds. What
// the same change carried with it, and named in its own commit message as a
// widening — "a non-IsNotExist Lstat error now degrades 'classification
// failed: unreadable' there too, where it used to be called clean" — had no
// arm at all. MEASURED 2026-11-13 (linux 7.2.15 x86_64, go1.27.3): replacing
// that degrade with `return ""` — failing OPEN on a project-config entry the
// launch cannot classify, in a check whose whole job is to fail closed — was
// GREEN over `-run 'TestQAProjectConfigTrust|ProjectConfig'`. The gap
// predates the hoist on the keyed branch; the hoist doubled the surface it
// leaves unguarded, exactly as ranger-base-g3pfl doubled the surface of the
// defect ranger-base-rgyto fixed.
//
// AN UNSEARCHABLE PARENT IS THE CHEAP SHAPE: no root, no fixture beyond a
// chmod, and it is a real one — a repo can ship a mode-0 directory, and a
// launch that cannot read what is inside one may not call it clean.
//
// EVERY DECLARED ENTRY OF EVERY RUNTIME, read off the runtimes rather than
// spelled, because what this is about is the check and not one of the four
// paths that reach it: a runtime that declares a new project-config entry
// tomorrow is in this arm the day it lands. Asserted per ENTRY, which is the
// answer the f67jx census reads, because two entries can share a parent and
// ProjectConfigTrust returns only the first offender in declared order — and
// then once at the launch level, which is the answer the operator sees.
func TestQAProjectConfigTrustUnreadablePresenceFailsClosed(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the mode bits this arm plants; the Lstat would succeed")
	}
	a := &App{Home: t.TempDir(), StateDir: t.TempDir()}
	members := trustExemptCLIHomeExecMembers(wallHome())

	const want = "project config classification failed: unreadable: "
	seen := 0
	for _, name := range a.ListRuntimes() {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		for _, rel := range rt.ProjectConfig {
			seen++
			t.Run(name+"-"+strings.ReplaceAll(rel, "/", "-"), func(t *testing.T) {
				dir := t.TempDir()
				p := filepath.Join(dir, filepath.FromSlash(rel))
				parent := filepath.Dir(p)
				if err := os.MkdirAll(parent, 0o755); err != nil {
					t.Fatal(err)
				}
				// The entry IS there. What the launch cannot do is see it.
				if err := os.WriteFile(p, []byte(`{"anything":{}}`), 0o644); err != nil {
					t.Fatal(err)
				}
				// The control first, while the dir is still readable: this
				// body is classified, so the finding below is the chmod's
				// and not the file's.
				if got := projectConfigVerdict(t, projectConfigTrustEntry(rt, dir, rel, members)); strings.HasPrefix(got, want) {
					t.Fatalf("%s %s is already unreadable before the chmod, so this arm measures nothing: %q", name, rel, got)
				}
				if err := os.Chmod(parent, 0o000); err != nil {
					t.Fatal(err)
				}
				// t.TempDir's own cleanup cannot descend into a mode-0 dir.
				t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

				got := projectConfigVerdict(t, projectConfigTrustEntry(rt, dir, rel, members))
				if !strings.HasPrefix(got, want) || !strings.Contains(got, "permission denied") {
					t.Errorf("%s %s under an unsearchable parent = %q, want %q… naming the permission error — an entry the launch cannot classify may not be called clean (ADR 0002 amendment 2026-08-26 §4)", name, rel, got, want)
				}
				// And the launch-level answer. Another entry sharing this
				// parent may be the one it names, which is why the row
				// above is the per-entry one; what is asserted here is that
				// the walk does not hand the operator a clean launch.
				if why := ProjectConfigTrust(rt, nil, dir); why == "" {
					t.Errorf("%s: the launch is clean with %s unreadable", name, rel)
				}
			})
		}
	}
	if seen < 2 {
		t.Fatalf("%d declared project-config entries reached this arm — the runtimes in this tree declare more than that, so the enumeration is reading nothing", seen)
	}
}
