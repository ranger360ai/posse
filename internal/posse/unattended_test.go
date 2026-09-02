package posse

// The mode a session starts in is a LAUNCH fact (rangerhq-qs5r). Nothing on
// the claude line named one, so every claude persona session got whatever
// the CLI defaulted to — and that default moved: developer-…-fuom and
// qa-…-8hlg both landed in `manual` and sat on approval dialogs nobody
// was watching. These pin the flag onto every path that starts a persona,
// so a session can never again be one CLI release away from blocking.
//
// Live evidence behind the constant, claude 2.1.239 on a herdr pane running
// the full fleet-shaped line: `--permission-mode manual` → footer "⏸ manual
// mode on" (the footer's exact wording), `--permission-mode auto` → "⏵⏵ auto
// mode on". The flag takes in both directions.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every built-in runtime's own template names its unattended flag — the
// first line of defence, and the only one for a PID that writes no command:.
func TestEveryBuiltinTemplateIsUnattended(t *testing.T) {
	t.Parallel()
	for _, rt := range builtinRuntimes {
		if rt.Unattended == "" {
			t.Errorf("%s: no unattended flag — a persona session on it can start asking for approvals", rt.Name)
			continue
		}
		if !strings.Contains(rt.Command, rt.Unattended) {
			t.Errorf("%s: template does not carry %q:\n%s", rt.Name, rt.Unattended, rt.Command)
		}
	}
	if !strings.Contains(DefaultAgentCommand, ClaudeFleetFlags) {
		t.Errorf("the default command is a launch path too:\n%s", DefaultAgentCommand)
	}
}

// A PID with no command: — the whole fleet — renders the mode on every
// runtime it can be launched on, at every tier.
func TestRenderedPersonaCommandIsUnattended(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	ag := loadTestAgent(t, "---\nname: p\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")
	for _, name := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		for _, tier := range Tiers {
			got := ag.RenderCommandFor(rt, "claude", tier)
			if !strings.Contains(got, rt.Unattended) {
				t.Errorf("%s/%s missing %q:\n%s", name, tier, rt.Unattended, got)
			}
			if strings.Count(got, rt.Unattended) != 1 {
				t.Errorf("%s/%s says %q twice:\n%s", name, tier, rt.Unattended, got)
			}
		}
	}
}

// The one template posse did not write. A hand-written command: that starts
// the runtime's own CLI still gets the mode; one that says a mode keeps the
// mode it said; one that starts something else is left alone, because posse
// knows no dialect there and a flag typed at the wrong program is a launch
// that does not start at all.
func TestOwnCommandStillLaunchesUnattended(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, command, want string }{
		{"appended", `claude --append-system-prompt "$(cat {file})"`,
			`claude --append-system-prompt "$(cat FILE)" --permission-mode auto`},
		{"explicit mode kept", `claude --permission-mode plan {allow}`,
			`claude --permission-mode plan`},
		{"foreign program untouched", `run {file} {allow} {deny}`,
			`run FILE`},
	} {
		t.Run(c.name, func(t *testing.T) {
			ag := loadTestAgent(t, "---\nname: p\ncommand: "+c.command+"\n---\nYou are p.\n")
			got := strings.ReplaceAll(ag.RenderCommand(), shellQuote(ag.Path), "FILE")
			if got != c.want {
				t.Errorf("\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

// A template-only runtime is a CLI posse knows nothing about: no flag, so
// nothing appended — the gap is honest, a guessed flag would be a session
// that never starts.
func TestTemplateOnlyRuntimeGetsNoGuessedFlag(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "plain.yaml"), []byte("command: plain {file}\n"), 0o644)
	rt, err := a.LoadRuntime("plain")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ag := loadTestAgent(t, "---\nname: p\n---\nYou are p.\n")
	if got := ag.RenderCommandFor(rt, "plain", TierStrong); strings.Contains(got, "--permission-mode") {
		t.Errorf("guessed a flag for a runtime posse does not know: %q", got)
	}
}

// The launch paths themselves: what is typed into the pane by `posse new`
// (CreateSession, which dispatch and the cockpit go through too) and by the
// relaunch that re-types the line. Both were the paths that landed manual.
func TestEveryLaunchPathTypesTheMode(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\ndeny: [Bash(git push:*)]\n---\nYou are developer.\n"), 0o644)

	mustCreate(t, b, NewSessionOpts{Name: "d1", Agent: "developer", Dir: t.TempDir()})
	if log := calls(t, fake); !strings.Contains(log, ClaudeFleetFlags) {
		t.Errorf("posse new typed no permission mode:\n%s", log)
	}

	os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("d1")
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	if ok, err := b.RelaunchAgent("d1", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	if got := calls(t, fake); strings.Count(got, ClaudeFleetFlags) != 2 {
		t.Errorf("a re-typed line must carry the mode too:\n%s", got)
	}
}

// The risky append position, which nothing pinned: claude's tool lists are
// VARIADIC, and EnsureUnattended puts the flag at the END of the line — so
// on a PID command: that ends in {allow}/{deny} the mode lands after an
// open list. If the parser swallowed it there it would become two deny
// PATTERNS and the session would silently take the CLI's default mode,
// which today is `auto` — i.e. it would look right and be wrong, and no
// footer check could tell (rangerhq-beby).
//
// Measured, claude 2.1.240: an INVALID mode trailing after --disallowedTools
// and after --allowedTools still errors with "option '--permission-mode'
// argument 'zzzz' is invalid" — the flag reaches the option parser, so the
// variadic list does end at the next option token. TestCLIStillKnowsTheMode
// keeps that measured; this pins the shape posse renders into it.
func TestModeIsAppendedAfterAVariadicToolList(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, command, wantList string }{
		{"after deny", `claude --append-system-prompt "$(cat {file})" {deny}`, "--disallowedTools"},
		{"after allow", `claude --append-system-prompt "$(cat {file})" {allow}`, "--allowedTools"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ag := loadTestAgent(t, "---\nname: p\nallow: [Bash(posse:*)]\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")
			ag.Command = c.command
			got := strings.ReplaceAll(ag.RenderCommand(), shellQuote(ag.Path), "FILE")
			if !strings.Contains(got, c.wantList) {
				t.Fatalf("test renders no variadic list, so it pins nothing:\n%s", got)
			}
			if strings.Count(got, ClaudeFleetFlags) != 1 {
				t.Errorf("want the mode exactly once:\n%s", got)
			}
			// The whole point: it sits after the open list, not before it.
			if !strings.HasSuffix(got, ClaudeFleetFlags) {
				t.Errorf("mode is not at the end of the line:\n%s", got)
			}
			if strings.Index(got, c.wantList) > strings.Index(got, ClaudeFleetFlags) {
				t.Errorf("the list must come first — this is the shape being pinned:\n%s", got)
			}
		})
	}
}

// The other half of the directive (rangerhq-oaya). qs5r pinned every path
// that starts a PERSONA; a launch with no persona took `cmd := o.Cmd`
// verbatim and never went near a mode, so `posse new x --cmd "claude"` and
// a recipe with no `agent:` ran on whatever the CLI defaulted to — measured
// live at claude 2.1.240 as the single word `claude` in argv, with a footer
// reading "auto mode on" for the wrong reason. What the line may and may
// not be guessed from is the whole content of the fix.
func TestNoPersonaLineTypesTheMode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, cmd, want string }{
		{"bare claude", `claude`, `claude --permission-mode auto`},
		{"bare grok", `grok`, `grok --permission-mode auto`},
		{"bare codex", `codex`, `codex -a never`},
		{"claude with flags", `claude --model x`, `claude --model x --permission-mode auto`},
		// The operator's own choice is never overridden — this is what
		// makes the mode a default rather than an imposition.
		{"explicit mode kept", `claude --permission-mode plan`, `claude --permission-mode plan`},
		{"explicit codex mode kept", `codex -a on-request`, `codex -a on-request`},
		// Found by path: an installed CLI named absolutely is the same CLI.
		{"absolute path", `/opt/bin/claude -x`, `/opt/bin/claude -x --permission-mode auto`},
		// And the refusals to guess. A flag typed at the wrong program is a
		// launch that does not start at all, which is worse than the mode.
		{"a shell is left alone", `zsh -l`, `zsh -l`},
		{"a wrapper is left alone", `env FOO=1 claude`, `env FOO=1 claude`},
		{"an unknown CLI is left alone", `plain --go`, `plain --go`},
		{"no command at all", ``, ``},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := EnsureUnattendedLine(c.cmd); got != c.want {
				t.Errorf("\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

// Idempotent, because relaunch re-plans from the line it recorded: a
// session whose meta already carries the appended flag must not collect a
// second one on every refresh.
func TestNoPersonaModeIsIdempotent(t *testing.T) {
	t.Parallel()
	got := EnsureUnattendedLine(EnsureUnattendedLine("claude"))
	if strings.Count(got, ClaudeFleetFlags) != 1 {
		t.Errorf("re-planning the recorded line said the mode twice: %q", got)
	}
}

// The launch paths themselves, typed into the pane: `posse new --cmd` and a
// recipe with no `agent:` — the two spellings of the hole, both measured
// live on argv before this landed.
func TestNoPersonaLaunchPathsTypeTheMode(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App

	mustCreate(t, b, NewSessionOpts{Name: "bare", Cmd: "claude", Crew: true})
	if log := calls(t, fake); !strings.Contains(log, "claude "+ClaudeFleetFlags) {
		t.Errorf("posse new --cmd typed no permission mode:\n%s", log)
	}

	os.MkdirAll(a.RecipesDir, 0o755)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(a.RecipesDir, "qa-bare.yaml"),
		[]byte("name: qa-bare\ndir: "+dir+"\ncommand: claude\nemoji: 🧪\n"), 0o644)
	if err := b.LaunchRecipe(io.Discard, "qa-bare"); err != nil {
		t.Fatalf("recipe: %v", err)
	}
	if got := strings.Count(calls(t, fake), "claude "+ClaudeFleetFlags); got != 2 {
		t.Errorf("a recipe with no agent: typed no permission mode (%d of 2):\n%s", got, calls(t, fake))
	}
}
