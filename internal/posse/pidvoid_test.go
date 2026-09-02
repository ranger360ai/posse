package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ranger-base-64qx. grok's `--system-prompt-override` replaces the system
// prompt verbatim and skips `--rules` — the channel every grok launch line
// delivers the PID on. Measured on 1.0.5 (see Runtime.PIDVoid): with both
// flags on the line the assembled system prompt is the override text alone,
// no `<human_rules>`, no PID marker, while the native rulebooks arrive
// untouched. So the reachable failure is a PID's own hand-written
// `command:` opening a session that has every repo rulebook and no persona.
//
// These pin the two halves separately: PIDVoided (what counts as naming the
// flag) and the launch (that anything actually asks).

func TestPIDVoidedMatchesTheFlagsOwnToken(t *testing.T) {
	t.Parallel()
	grok, err := (&App{}).LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	claude, err := (&App{}).LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		rt   *Runtime
		line string
		want string
	}{
		// Both spellings void the PID; grok's own --help names the second
		// as a compat alias of the first, and the probe measured them
		// identical (19 B of override text, no <human_rules>, either way).
		{grok, `grok --rules="$(cat '/p/g.md')" --system-prompt-override 'be terse'`, "--system-prompt-override"},
		{grok, `grok --rules="$(cat '/p/g.md')" --system-prompt 'be terse'`, "--system-prompt"},
		{grok, `grok --system-prompt-override='be terse' --rules="$(cat '/p/g.md')"`, "--system-prompt-override"},
		{grok, `grok --system-prompt='be terse'`, "--system-prompt"},
		// The built-in template — the line the whole fleet runs today.
		{grok, `grok --rules="$(cat '/p/g.md')"`, ""},
		// Token, not substring: a longer flag that starts with one of
		// these is a different flag, and --system-prompt being a prefix of
		// --system-prompt-override is why both are listed rather than one.
		{grok, `grok --system-prompt-overrides 'x'`, ""},
		{grok, `grok --no-system-prompt`, ""},
		// The declaration is the runtime's own, not a fleet-wide grep: a
		// runtime that has measured no such flag refuses nothing.
		{claude, `claude --system-prompt-override 'be terse' --append-system-prompt "$(cat '/p/g.md')"`, ""},
	} {
		if got := c.rt.PIDVoided(c.line); got != c.want {
			t.Errorf("%s: PIDVoided(%q) = %q, want %q", c.rt.Name, c.line, got, c.want)
		}
	}
}

// writeVoidPID writes a PID with an explicit command: — the one path that
// can put a voiding flag on a launch line, since no built-in template does.
func writeVoidPID(t *testing.T, a *App, name, runtime, command string) {
	t.Helper()
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	front := "---\nname: " + name + "\ndescription: t\nruntime: " + runtime + "\n"
	if command != "" {
		front += "command: " + command + "\n"
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"),
		[]byte(front+"---\nYou are "+name+", the developer of the crew.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The launch half: a refusal, not a DEGRADED launch. `degraded` is for a
// gate no wall layer could realize; a persona that is not in the session at
// all is not a weaker persona.
func TestLaunchRefusesAGrokLineThatVoidsThePID(t *testing.T) {
	b, _ := newTestBackend(t)
	a := b.App

	rules := ` --rules="$(cat {file})"`
	writeVoidPID(t, a, "voided", "grok", `grok --system-prompt-override 'be terse'`+rules)
	writeVoidPID(t, a, "aliased", "grok", `grok --system-prompt 'be terse'`+rules)
	// Controls. `plain` is the built-in template (no command: at all) —
	// the line every grok persona runs today. `nearmiss` names a token
	// that merely starts with the flag. `claudeside` puts the very same
	// flag on a runtime that declares none.
	writeVoidPID(t, a, "plain", "grok", "")
	writeVoidPID(t, a, "nearmiss", "grok", `grok --system-prompt-overrides 'x'`+rules)
	writeVoidPID(t, a, "claudeside", "claude", `claude --system-prompt-override 'be terse' --append-system-prompt "$(cat {file})"`)

	for _, c := range []struct{ agent, flag string }{
		{"voided", "--system-prompt-override"},
		{"aliased", "--system-prompt"},
	} {
		_, err := b.planLaunch(NewSessionOpts{Name: "s-" + c.agent, Dir: t.TempDir(), Agent: c.agent})
		if err == nil {
			t.Fatalf("%s: a launch line naming %s must be refused — it opens a session with no PID", c.agent, c.flag)
		}
		if !strings.Contains(err.Error(), c.flag) {
			t.Errorf("%s: the refusal must name the flag it saw, got: %v", c.agent, err)
		}
	}
	for _, name := range []string{"plain", "nearmiss", "claudeside"} {
		if _, err := b.planLaunch(NewSessionOpts{Name: "s-" + name, Dir: t.TempDir(), Agent: name}); err != nil {
			t.Errorf("%s: must plan clean, got: %v", name, err)
		}
	}
}

// The other rendering path. RelaunchAgent re-types a persona command into a
// pane that is already open, so it renders the PID again and planLaunch's
// refusal never runs. Reachable even though the create was refused: the PID
// is re-read from disk, so a `command:` edited after the session opened
// arrives here first — and a crashed CLI would come back as a session with
// the persona's name and none of its identity.
func TestRelaunchRefusesALineThatVoidsThePID(t *testing.T) {
	b, fake := newTestBackend(t)
	b.Warn = &strings.Builder{}
	agentPerLaunch(t, fake)
	writeVoidPID(t, b.App, "quiet", "grok", "")
	if err := b.CreateSession(NewSessionOpts{Name: "rq", Agent: "quiet", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}

	// The positive witness first: with the PID as created, this rig really
	// does re-type — so the refusal below is the flag's doing and not a
	// relaunch that was never going to happen.
	died := func() {
		t.Helper()
		m, _ := b.readMeta("rq")
		m.Launched = time.Now().Add(-time.Hour)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		os.Remove(filepath.Join(fake, "agents.json")) // the CLI died
	}
	died()
	if ok, err := b.RelaunchAgent("rq", time.Second); err != nil || !ok {
		t.Fatalf("witness: a clean PID must re-type: ok=%v err=%v", ok, err)
	}

	writeVoidPID(t, b.App, "quiet", "grok", `grok --system-prompt-override 'be terse' --rules="$(cat {file})"`)
	died()
	ok, err := b.RelaunchAgent("rq", time.Second)
	if err == nil {
		t.Fatalf("a re-typed line that discards the PID must be refused: ok=%v", ok)
	}
	if !strings.Contains(err.Error(), "--system-prompt-override") {
		t.Errorf("the refusal must name the flag it saw, got: %v", err)
	}
}
