package rhq

// The bd half of the L1 shim's option-aware verb match (ranger-base-3bqn).
//
// A `Bash(bd <verb>:*)` permission rule matches a token prefix of the typed
// line, so `bd --no-daemon daemon --help` walks past it (MEASURED on az93).
// The shim is the layer that does not have that hole — but only where
// globalValueOpts knows which of the command's global options eat the next
// word. bd has four. Without them `bd --db /tmp/x daemon stop` resolves to
// the verb `/tmp/x`.
//
// The two tables below pin the entry from both sides, so deleting it fails
// this test twice: `--db /x daemon stop` stops being refused, and `--actor
// daemon show x` — whose OPTION VALUE is the word `daemon` — starts being.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBdShimResolvesTheVerbBehindGlobalOptions(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(realBin, "bd"), []byte("#!/bin/sh\necho \"real bd $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("devops", []string{"Bash(bd daemon:*)"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "bd")
	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(shim, args...)
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return out.String(), errb.String(), code
	}

	// Every spelling that reaches `bd daemon`, including the one the fleet
	// is taught to type and the four options that take a separate value.
	refused := [][]string{
		{"daemon", "stop"},
		{"--no-daemon", "daemon", "--help"},
		{"--json", "--no-daemon", "daemon", "stop"},
		{"--db", "/tmp/x", "daemon", "stop"},
		{"--db=/tmp/x", "daemon", "stop"},
		{"--actor", "someone", "daemon", "stop"},
		{"--lock-timeout", "5s", "daemon", "stop"},
		{"--dolt-auto-commit", "on", "daemon", "stop"},
		{"--allow-stale", "--no-auto-flush", "--sandbox", "daemon"},
		{"-q", "-v", "daemon", "killall"},
	}
	for _, args := range refused {
		out, errs, code := run(args...)
		want := "refused by posse gate: bd " + strings.Join(args, " ") + " (deny: Bash(bd daemon:*))"
		if code != 1 || !strings.Contains(errs, want) || out != "" {
			t.Errorf("bd %s must be refused: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}

	// …and the shim stays out of the way otherwise. The middle rows are the
	// other half of the entry: `daemon` as an option's VALUE is not the verb.
	passed := [][]string{
		{"show", "ranger-base-3bqn"},
		{"--no-daemon", "ready", "--json"},
		{"--actor", "daemon", "show", "x"},
		{"--db", "daemon", "list"},
		{"--lock-timeout", "daemon", "status"},
		{"create", "daemon", "-t", "task"},
		{"daemons", "list"}, // a different word: the shim matches tokens
	}
	for _, args := range passed {
		out, errs, code := run(args...)
		if code != 0 || strings.TrimSpace(out) != "real bd "+strings.Join(args, " ") {
			t.Errorf("bd %s must pass through: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}

	// A dangling value option is bd's own usage error, not the shim's.
	if _, _, code := run("--db"); code != 0 {
		t.Errorf("dangling --db must reach the real bd: code=%d", code)
	}
}

// And what parity is allowed to say about it: option-aware, so the rule is
// matched faithfully — which is what the entry buys. `daemons` is a second
// rule, not something the first one covers, and the hidden-verb class needs
// the allow-list gate (scripts/bd-argv-gate.py), not this.
func TestBdRuleIsOptionAwareForParity(t *testing.T) {
	rules := ParseShimRules([]string{"Bash(bd daemon:*)"})["bd"]
	if len(rules) != 1 {
		t.Fatalf("want one bd rule, got %+v", rules)
	}
	kind, faithful := matcherFor("bd", rules[0])
	if kind != "subcommand, option-aware" || !faithful {
		t.Errorf("bd rules must be option-aware for parity, got %q faithful=%v", kind, faithful)
	}
}
