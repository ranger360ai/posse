//go:build !posse_arm2 && !posse_arm3

package posse

// Render pins for ADR 0090 D1 (ranger-base-m0bfv): every claude line posse can
// produce carries --strict-mcp-config, and a line that names its own
// --mcp-config is refused at both wall mechanisms when the seat could rewrite
// that file. The live join is mcpstrict_live_test.go.
//
//	go test ./internal/posse -run 'StrictMCP|MCPConfig' -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAStrictMCPOnEveryClaudeLinePosseCanProduce(t *testing.T) {
	t.Parallel()
	once := func(t *testing.T, where, line string) {
		t.Helper()
		n := 0
		for _, w := range strings.Fields(line) {
			if w == ClaudeStrictMCPFlag {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: %s appears %d times, want once:\n%s", where, ClaudeStrictMCPFlag, n, line)
		}
	}
	// The templates themselves.
	once(t, "DefaultAgentCommand", DefaultAgentCommand)
	for _, rt := range builtinRuntimes {
		if rt.Exe() == claudeExe {
			once(t, "built-in "+rt.Name+" template", rt.Command)
		} else if strings.Contains(rt.Command, ClaudeStrictMCPFlag) {
			t.Errorf("%s: %s is claude's flag, typed at %s", rt.Name, ClaudeStrictMCPFlag, rt.Exe())
		}
	}

	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "myclaude.yaml"), []byte("command: claude --append-system-prompt \"$(cat {file})\"\n"), 0o644)
	plain := loadTestAgent(t, "---\nname: p\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")

	// Every built-in runtime at every tier: claude carries it, the others
	// do not (D3 decides them per CLI, by measurement).
	for _, name := range []string{"claude", "codex", "grok", "myclaude"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		for _, tier := range Tiers {
			got := plain.RenderCommandFor(rt, name, tier)
			if strings.HasPrefix(got, "claude") {
				once(t, name+"/"+tier, got)
			} else if strings.Contains(got, ClaudeStrictMCPFlag) {
				t.Errorf("%s/%s: claude's flag on a %s line:\n%s", name, tier, name, got)
			}
		}
	}

	// A PID's own command: — the template posse did not write.
	for _, c := range []struct{ name, command string }{
		{"bare", `claude --append-system-prompt "$(cat {file})"`},
		{"own mode", `claude --permission-mode plan {allow}`},
		{"own settings", `claude --settings '{}' {file}`},
		{"already strict", `claude --strict-mcp-config {file}`},
		{"own servers", `claude --mcp-config /etc/posse/mcp.json {file}`},
	} {
		ag := loadTestAgent(t, "---\nname: p\ncommand: "+c.command+"\n---\nYou are p.\n")
		once(t, "command: "+c.name, ag.RenderCommand())
	}

	// The no-persona `--cmd` line.
	once(t, "--cmd claude", EnsureUnattendedLine("claude"))
	once(t, "--cmd claude, own mode", EnsureUnattendedLine("claude --permission-mode plan"))
	once(t, "--cmd absolute claude", EnsureUnattendedLine("/opt/bin/claude"))
	if got := EnsureUnattendedLine("codex"); strings.Contains(got, ClaudeStrictMCPFlag) {
		t.Errorf("--cmd codex got claude's flag: %q", got)
	}
}

// The door refuses to guess past a first word it cannot see, the same as the
// other doors — and the runtime check names that case instead of passing it.
func TestQAStrictMCPDoorLeavesAWrapperAloneAndTheCheckSaysSo(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"env FOO=1 claude", "run {file}", ""} {
		if got := EnsureStrictMCPLine(in); got != in {
			t.Errorf("EnsureStrictMCPLine(%q) = %q, want unchanged", in, got)
		}
	}
	if c := strictMCPClause(DefaultAgentCommand); !strings.HasPrefix(c, ClaudeStrictMCPFlag+" on the line") {
		t.Errorf("claude template clause: %q", c)
	}
	if c := strictMCPClause("claude {file}"); !strings.HasPrefix(c, ClaudeStrictMCPFlag+" on the line") {
		t.Errorf("a claude template without the flag is repaired by the door, so the clause reads on the line: %q", c)
	}
	if c := strictMCPClause("env FOO=1 claude {file}"); !strings.HasPrefix(c, "NO "+ClaudeStrictMCPFlag) {
		t.Errorf("wrapped claude must be reported missing, like a missing unattended flag: %q", c)
	}
	if c := strictMCPClause("codex -a never"); c != "" {
		t.Errorf("a codex template got a claude clause: %q", c)
	}
}

func TestQAMCPConfigFilesReadsEverySpelling(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line       string
		files, unv []string
	}{
		{"claude --strict-mcp-config", nil, nil},
		{"claude --mcp-config /a.json", []string{"/a.json"}, nil},
		{"claude --mcp-config=/a.json --model x", []string{"/a.json"}, nil},
		{"claude --mcp-config /a.json 'b.json' --add-dir /m", []string{"/a.json", "b.json"}, nil},
		{`claude --mcp-config '{"mcpServers":{}}' --model x`, nil, nil},
		{"claude --mcp-config $HOME/x.json", nil, []string{"$HOME/x.json"}},
		{"claude --mcp-config \"$(cat p)\"", nil, []string{"$(cat"}},
		{"claude --add-dir /m /n", nil, nil},
	} {
		f, u := MCPConfigFiles(c.line)
		if strings.Join(f, "|") != strings.Join(c.files, "|") || strings.Join(u, "|") != strings.Join(c.unv, "|") {
			t.Errorf("MCPConfigFiles(%q) = %q, %q; want %q, %q", c.line, f, u, c.files, c.unv)
		}
	}
}

// The refusal, at BOTH mechanisms, against each mechanism's own write set.
func TestQAMCPConfigInsideTheWriteSetIsRefusedAtBothMechanisms(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	// Outside every blanket temp grant, or "outside" would be writable.
	root := wallUngrantedDir(t)
	f := weFixtureIn(t, root)
	outside := filepath.Join(root, "etc-mcp.json")
	os.WriteFile(outside, []byte("{}"), 0o644)
	inTree := filepath.Join(f.dir, ".mcp.json")
	os.WriteFile(inTree, []byte("{}"), 0o644)
	link := filepath.Join(f.dir, "linked.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	inward := filepath.Join(root, "inward.json")
	if err := os.Symlink(inTree, inward); err != nil {
		t.Fatal(err)
	}
	for _, mech := range []string{WallSandboxExec, WallBwrap} {
		refuse := func(why, line string) {
			if err := f.a.mcpConfigWallRefusalOn(mech, f.ag, f.rt, f.dir, line); err == nil {
				t.Errorf("%s: %s: accepted %q", mech, why, line)
			}
		}
		accept := func(why, line string) {
			if err := f.a.mcpConfigWallRefusalOn(mech, f.ag, f.rt, f.dir, line); err != nil {
				t.Errorf("%s: %s: refused %q: %v", mech, why, line, err)
			}
		}
		accept("no --mcp-config", "claude --strict-mcp-config")
		accept("a file outside every grant", "claude --mcp-config "+outside)
		accept("inline JSON names no file", `claude --mcp-config '{"mcpServers":{}}'`)
		refuse("the session tree", "claude --mcp-config "+inTree)
		refuse("relative, against the session tree", "claude --mcp-config .mcp.json")
		refuse("the config dir's .claude.json", "claude --mcp-config="+filepath.Join(f.cfg, ".claude.json"))
		refuse("a link in the write set the seat could repoint", "claude --mcp-config "+link)
		refuse("a link outside pointing in", "claude --mcp-config "+inward)
		refuse("a value posse cannot resolve without a shell", "claude --mcp-config $HOME/x.json")
		refuse("the second of two values", "claude --mcp-config "+outside+" "+inTree)
	}
}
