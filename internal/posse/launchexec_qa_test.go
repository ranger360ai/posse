//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-1l1en — a seat's pane must not outlive its runtime
// as an interactive shell, so every launch line posse types REPLACES the
// shell that runs it.
//
// launchexec.go carries the measurement for why that is `exec` and not a
// trailing `; exit`, and where in the line the word has to go. These are the
// pins over what posse actually types: both Start sites (the create path and
// the re-type path), both PaneLine forms (typed, and spilled to
// state/launch/<session>.sh), and every built-in runtime — because the
// insertion point is decided by the LINE's own shape, and each runtime
// renders a different one.
//
// READ RAW, not through launchLog. gatePrefixRe folds the exec into the
// `GATES ` marker beside PATH= and the two `/usr/bin/env -u` pins, for the
// reason it folds those: fifteen pins whose subject is the claude line would
// otherwise have had to spell the launcher's plumbing. That makes this file
// the reader that must not collapse anything (herdr_test.go's own note on
// the expression says so).
//
// WHAT IS NOT HERE. That a shell really is gone after an exec, and really is
// NOT after `cmd; exit` when cmd dies of SIGINT, is a fact about a tty and a
// real bash: launchexeclive_linux_test.go (arm 2, env-gated) is that pin.
// That no fifth caller of the slot's Start verb types a launch without
// coming through PaneLine is a fact about the tree:
// internal/treepins/launchexec_qa_test.go is that one.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// lxAgent writes the one persona these pins launch: a PID with a deny rule
// (so the gates dir and its prefix are real) and nothing else, so the line
// under test is the launcher's own rendering and not a fixture's.
func lxAgent(t *testing.T, a *App, name string) {
	t.Helper()
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndeny: [Bash(git push:*)]\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// lxCommandLines is every launch as it LANDED, raw: the typed line itself,
// or — for a line over PaneLineMax — the command line out of the script the
// pane sources. Nothing is collapsed and nothing is normalized, because the
// subject is the bytes.
func lxCommandLines(t *testing.T, fake string) []string {
	t.Helper()
	var out []string
	for _, ln := range paneRunLines(t, fake) {
		body := resolvedTypedLine(t, ln)
		// A typed line is itself; a spilled body is its one command line
		// among the comments PaneLine writes above it.
		cmd := ""
		for _, l := range strings.Split(body, "\n") {
			if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
				continue
			}
			if cmd != "" {
				t.Fatalf("a spilled launch script carries more than one command line, so this reader cannot say which is the launch:\n%s", body)
			}
			cmd = l
		}
		if cmd == "" {
			t.Fatalf("a launch line resolved to nothing:\n%q", ln)
		}
		out = append(out, cmd)
	}
	return out
}

// lxAssertExecd is the whole claim about one landed line, in the three
// pieces that can each be wrong on their own.
func lxAssertExecd(t *testing.T, what, line string) {
	t.Helper()
	if !LaunchLineIsExecd(line) {
		t.Errorf("%s does not replace the shell that runs it, so that shell comes back to a prompt when the runtime dies and reads whatever is typed at the seat next (ranger-base-1l1en):\n%s", what, line)
		return
	}
	// The exec is ON the command word and not in front of the line: `exec
	// FOO=bar cmd` is "exec: FOO=bar: not found" (MEASURED, launchexec.go),
	// so a line that had an assignment prefix and now starts with `exec`
	// would not launch at all — and the launch that did would carry none of
	// the gate PATH, the runtime home or the credential dir.
	if strings.HasPrefix(line, "exec ") && strings.Contains(line, `:"$PATH" `) {
		t.Errorf("%s puts `exec` in front of its own assignment prefix — the shell would look for a program called PATH=…:\n%s", what, line)
	}
	// And the fallback is a fallback: nothing posse renders today is a
	// command LIST, and the `/bin/sh -c` arm costs the pane's foreground
	// process being a shell instead of the runtime, which is what herdr's
	// argv0 detection and every kill path read.
	if strings.HasPrefix(line, "exec /bin/sh -c ") {
		t.Errorf("%s went through the compound fallback: some renderer now emits a command list, and the seat's foreground process is a shell rather than its runtime. Either make the line one simple command or price what that costs detection (ranger-base-1l1en):\n%s", what, line)
	}
}

// Arm 1: the create path, every built-in runtime. The gate prefix, the
// runtime-home pin and the transcript ceiling all render into the head of
// this line, and each of them moves where the command word is — so the
// insertion point is pinned per runtime rather than once.
func TestQAEveryBuiltinRuntimeIsLaunchedWithAnExecdLine(t *testing.T) {
	t.Parallel()
	for i := range builtinRuntimes {
		rt := builtinRuntimes[i].Name
		t.Run(rt, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			lxAgent(t, b.App, "dev")
			if err := b.CreateSession(NewSessionOpts{Name: "s1", Agent: "dev", Runtime: rt, Dir: t.TempDir()}); err != nil {
				t.Fatalf("create on %s: %v", rt, err)
			}
			lines := lxCommandLines(t, fake)
			if len(lines) != 1 {
				t.Fatalf("want one launch, got %d: %q", len(lines), lines)
			}
			lxAssertExecd(t, "the "+rt+" create line", lines[0])
		})
	}
}

// Arm 2: the re-type path (RelaunchAgent). It renders its own line, so it
// can lose the exec on its own — and it is the path that types into a pane
// that already exists, which is the state this bead exists to end.
func TestQATheRetypedLineIsExecdToo(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	lxAgent(t, b.App, "dev")
	mustCreate(t, b, NewSessionOpts{Name: "r1", Agent: "dev", Dir: t.TempDir()})
	// The fake herdr stops reporting an agent, and the launch is pushed past
	// the relaunch grace — the same fixture TestEveryLaunchPathTypesTheMode
	// uses to reach this path.
	os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("r1")
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	if ok, err := b.RelaunchAgent("r1", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	lines := lxCommandLines(t, fake)
	if len(lines) < 2 {
		t.Fatalf("fixture: the relaunch typed no second line (%d), so this pin would be reading the create again: %q", len(lines), lines)
	}
	lxAssertExecd(t, "the re-typed line", lines[len(lines)-1])
}

// Arm 3: the spilled form. A line over PaneLineMax is not typed at all — the
// pane sources a script — and `exec` inside a sourced file replaces the
// sourcing shell, so the script's command line is where the exec has to be.
// In production this is the NORMAL case (paneline.go: every persona's line
// measured at 1867-2058 bytes against the 1023 limit), which is exactly why
// a pin that only ever read a typed line would be reading the exception.
func TestQAASpilledLaunchScriptExecsToo(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	bigPersona(t, b, "big")
	mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "big", Dir: t.TempDir()})
	typed := paneRunLines(t, fake)
	if len(typed) != 1 {
		t.Fatalf("want one launch, got %d: %q", len(typed), typed)
	}
	if !strings.HasPrefix(typed[0], ". ") {
		t.Fatalf("fixture: this line was TYPED (%d bytes), so it is not the spilled form this pin is about:\n%s", len(typed[0]), typed[0])
	}
	body, err := os.ReadFile(b.App.LaunchScript("s1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ranger-base-1l1en") {
		t.Errorf("a rendered file an operator will find while debugging must say why its command is exec'd:\n%s", body)
	}
	lxAssertExecd(t, "the spilled launch script's command line", lxCommandLines(t, fake)[0])
}

// Arm 4: the unit, over the three shapes launchexec.go claims. The rows are
// the real line heads this tree renders — the gate prefix's assignments, the
// transcript ceiling's `/usr/bin/env -u`, and the bwrap launch's mid-command
// `3<` redirection, which is the one that makes `exec` land on a word that is
// not the program.
func TestQAExecGoesOnTheCommandWordAndNowhereElse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, in, want string
		wrapped        bool
	}{
		{"a bare command", "claude", "exec claude", false},
		{"behind an assignment prefix",
			`PATH='/g/bin':"$PATH" SHELL='/g/sh' claude --model 'm'`,
			`PATH='/g/bin':"$PATH" SHELL='/g/sh' exec claude --model 'm'`, false},
		{"in front of an env -u the prefix ends with",
			`PATH='/g/bin':"$PATH" /usr/bin/env -u CLAUDE_CODE_SKIP_PROMPT_HISTORY claude`,
			`PATH='/g/bin':"$PATH" exec /usr/bin/env -u CLAUDE_CODE_SKIP_PROMPT_HISTORY claude`, false},
		{"past a command substitution in double quotes",
			`A='1' claude --append-system-prompt "$(cat '/a/p.md')"`,
			`A='1' exec claude --append-system-prompt "$(cat '/a/p.md')"`, false},
		{"before a redirection that precedes the program",
			`A='1' 3<'/s/bwrap.args' '/u/bwrap' --args 3 -- claude`,
			`A='1' exec 3<'/s/bwrap.args' '/u/bwrap' --args 3 -- claude`, false},
		{"a list falls back to sh -c", "cd /x && claude",
			`exec /bin/sh -c 'cd /x && claude'`, true},
		{"a pipeline falls back too", "claude | tee /l",
			`exec /bin/sh -c 'claude | tee /l'`, true},
		{"a trailing exit is NOT what this renders", "claude; exit",
			`exec /bin/sh -c 'claude; exit'`, true},
		{"an unterminated quote is not taken apart", `claude --x '/a`,
			`exec /bin/sh -c 'claude --x '\''/a'`, true},
		{"nothing to exec comes back unchanged", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, wrapped := ExecLaunchLine(c.in)
			if got != c.want {
				t.Errorf("ExecLaunchLine(%q)\n got %q\nwant %q", c.in, got, c.want)
			}
			if wrapped != c.wrapped {
				t.Errorf("ExecLaunchLine(%q) wrapped=%v, want %v", c.in, wrapped, c.wrapped)
			}
		})
	}
}

// Arm 5: the reader the pins above ask has to be able to say NO — otherwise
// every arm here is a spelling exercise. The rows are the two forms this
// bead rejected and the one it replaced.
func TestQALaunchLineIsExecdRefusesAShellThatSurvives(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, line string
		want       bool
	}{
		{"the exec'd form", `PATH='/g':"$PATH" exec claude`, true},
		{"the compound fallback", `exec /bin/sh -c 'cd /x && claude'`, true},
		{"the line as it was before this bead", `PATH='/g':"$PATH" claude --model 'm'`, false},
		{"a trailing exit, which SIGINT skips", `PATH='/g':"$PATH" claude; exit`, false},
		{"exec in front of the assignments, which launches nothing", `exec PATH='/g':"$PATH" claude`, false},
		{"an exec that is only an argument", `claude --x exec`, false},
		{"a second command after the exec", `exec claude; echo done`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := LaunchLineIsExecd(c.line); got != c.want {
				t.Errorf("LaunchLineIsExecd(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}
