//go:build !posse_arm2 && !posse_arm3

package posse

// The live half of ADR 0090 D4 (ranger-base-m0bfv): posse's own bwrap render,
// the real claude, and a server definition planted where a caged seat CAN
// write — the seat's `.claude.json` (user and project scope) and a repo
// `.mcp.json` with its approval. Each planted server and the one legitimate
// server touch a marker when started, so "which servers started" is a file
// listing and not a parse of the CLI's output.
//
// Three arms, and the two controls are what make the pass mean anything:
//
//   - the plant itself must SUCCEED under the wall — the file has to stay
//     writable for claude (ADR 0080 D19), and a refused plant would make
//     the strict arm pass for the wrong reason;
//   - the rendered line, plus one legitimate --mcp-config outside the wall:
//     the legitimate server starts and no planted one does;
//   - the same line WITHOUT --strict-mcp-config: every planted server
//     starts. That arm reds this pin if a claude release moves where MCP is
//     read from, which is the only way the strict arm could go quietly green.
//
// NO TURN IS BILLED, and no packet leaves the box: the key is a dummy, and
// the walled process runs with --unshare-net added in front of posse's argv
// (a narrowing of the wall, never a widening), because the settings pin on
// the rendered line names the real API endpoint. MEASURED in the ADR: servers
// start before the first API call, so nothing here needs one.
//
//	go test ./internal/posse -run 'StrictMCP' -v

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// liveClaudeBin is the claude binary itself, not a mise command wrapper: the
// wrapper resolves its tool from the operator's HOME, and the fixture moves
// HOME. "" when this host has no claude.
func liveClaudeBin() string {
	if out, err := exec.Command("mise", "which", "claude").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	if p, err := exec.LookPath("claude"); err == nil && !strings.Contains(p, "command-wrappers") {
		return p
	}
	return ""
}

func TestLiveStrictMCPStartsOnlyTheLineServersUnderTheRenderedWall(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16); the darwin arm is the verify lane's re-verify")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	if why := liveBwrapApplyRefusal(); why != "" {
		t.Skip("NOTHING MEASURED: " + why)
	}
	bin := liveClaudeBin()
	if bin == "" {
		t.Skip("NOTHING MEASURED: no claude binary on this host")
	}
	root := wallUngrantedDir(t)
	f := weFixtureIn(t, root)
	pid := filepath.Join(root, "posse", "agents", "developer.md")
	if err := os.MkdirAll(filepath.Dir(pid), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pid, []byte("---\nname: developer\n---\nYou are developer.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.ag.Path = pid
	f.ag.Body = "You are developer.\n"

	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	walled := func(ctx context.Context, script string, env ...string) *exec.Cmd {
		argv := append([]string{"--unshare-net"}, r.Argv...)
		c := exec.CommandContext(ctx, "bwrap", append(argv, "/bin/sh", "-c", script)...)
		c.Env = append(os.Environ(), env...)
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
		return c
	}
	q := shellQuote
	markers := filepath.Join(f.dir, "markers")
	if err := os.MkdirAll(markers, 0o755); err != nil {
		t.Fatal(err)
	}
	server := func(name string) map[string]any {
		return map[string]any{"type": "stdio", "command": "/bin/sh", "args": []string{"-c", "touch " + q(filepath.Join(markers, name)) + "; exec sleep 60"}}
	}
	proj := absResolve(f.dir)
	planted := map[string]any{
		"mcpServers": map[string]any{"planted-user": server("planted-user")},
		"projects": map[string]any{proj: map[string]any{
			"mcpServers":                 map[string]any{"planted-project": server("planted-project")},
			"enabledMcpjsonServers":      []string{"planted-repo"},
			"enableAllProjectMcpServers": true,
			"hasTrustDialogAccepted":     true,
		}},
	}
	userJSON, _ := json.Marshal(planted)
	repoJSON, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"planted-repo": server("planted-repo")}})
	legitJSON, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"legit": server("legit")}})
	legit := filepath.Join(root, "legit-mcp.json")
	if err := os.WriteFile(legit, legitJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// Control 1: the plant succeeds under the wall — both files.
	plant := "printf '%s' " + q(string(userJSON)) + " > " + q(filepath.Join(f.cfg, ".claude.json")) +
		" && printf '%s' " + q(string(repoJSON)) + " > " + q(filepath.Join(f.dir, ".mcp.json"))
	if out, err := walled(context.Background(), plant).CombinedOutput(); err != nil {
		t.Fatalf("CONTROL: the plant was refused under the wall, so the strict arm below would pass for the wrong reason — the seat's .claude.json must stay writable (ADR 0080 D19): %v\n%s", err, out)
	}
	// And the legitimate file is outside the write set, by the wall and by
	// the launch's own check.
	if out, err := walled(context.Background(), "echo x >> "+q(legit)).CombinedOutput(); err == nil {
		t.Fatalf("the legitimate --mcp-config file is writable under the wall, so it is no control: %s", out)
	}
	if err := f.a.mcpConfigWallRefusalOn(WallBwrap, f.ag, f.rt, f.dir, "claude --mcp-config "+legit); err != nil {
		t.Fatalf("the launch refuses a --mcp-config the wall does not let the seat write: %v", err)
	}
	if err := f.a.mcpConfigWallRefusalOn(WallBwrap, f.ag, f.rt, f.dir, "claude --mcp-config "+filepath.Join(f.dir, ".mcp.json")); err == nil {
		t.Fatal("the launch accepts a --mcp-config the seat can rewrite (ADR 0090 D1)")
	}
	// The home file is out of the bwrap write set (D19), so a seat cannot
	// reach the operator's own unset-variable claude through it.
	if out, err := walled(context.Background(), "echo '{}' > "+q(filepath.Join(f.home, ".claude.json"))).CombinedOutput(); err == nil {
		t.Errorf("$HOME/.claude.json is writable under the wall — a bare claude on the box reads that file (ADR 0090 D2): %s", out)
	}

	var crt *Runtime
	for i := range builtinRuntimes {
		if builtinRuntimes[i].Name == DefaultRuntime {
			x := builtinRuntimes[i]
			crt = &x
		}
	}
	line := f.ag.RenderCommandFor(crt, DefaultRuntime, TierStrong)
	if !strings.HasPrefix(line, "claude ") {
		t.Fatalf("the rendered line does not start claude: %s", line)
	}
	line = q(bin) + strings.TrimPrefix(line, "claude") + " --mcp-config " + q(legit) + " -p hello"

	// run starts the line and returns the markers present once every name in
	// want has appeared (or a minute has passed), plus a settle: the servers
	// a launch starts start together, so a planted one that has not appeared
	// several seconds after the legitimate one is a server that was not read.
	run := func(t *testing.T, line string, want ...string) []string {
		t.Helper()
		for _, e := range []string{"legit", "planted-user", "planted-project", "planted-repo"} {
			os.Remove(filepath.Join(markers, e))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		c := walled(ctx, "cd "+q(f.dir)+" && exec "+line, "ANTHROPIC_API_KEY=sk-ant-dummy-not-a-key", "ANTHROPIC_BASE_URL=http://127.0.0.1:9", "DISABLE_AUTOUPDATER=1")
		var out strings.Builder
		c.Stdout, c.Stderr = &out, &out
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() { c.Wait(); close(done) }()
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			all := true
			for _, w := range want {
				if _, err := os.Stat(filepath.Join(markers, w)); err != nil {
					all = false
				}
			}
			if all {
				break
			}
			select {
			case <-done:
				deadline = time.Now()
			case <-time.After(200 * time.Millisecond):
			}
		}
		// Servers start together; give the rest a moment past the first.
		time.Sleep(8 * time.Second)
		cancel()
		<-done
		var started []string
		ents, _ := os.ReadDir(markers)
		for _, e := range ents {
			started = append(started, e.Name())
		}
		if len(started) == 0 {
			t.Logf("claude output:\n%s", out.String())
		}
		return started
	}

	t.Run("strict", func(t *testing.T) {
		if !strings.Contains(line, " "+ClaudeStrictMCPFlag) {
			t.Fatalf("the rendered line does not carry %s: %s", ClaudeStrictMCPFlag, line)
		}
		got := run(t, line, "legit")
		if strings.Join(got, ",") != "legit" {
			t.Errorf("servers started under the rendered line: %v, want [legit] alone (ADR 0090 D1)", got)
		}
	})
	t.Run("control without the flag", func(t *testing.T) {
		bare := strings.Replace(line, " "+ClaudeStrictMCPFlag, "", 1)
		got := run(t, bare, "planted-user", "planted-project", "planted-repo")
		have := map[string]bool{}
		for _, g := range got {
			have[g] = true
		}
		if !have["planted-user"] || !have["planted-project"] || !have["planted-repo"] {
			t.Errorf("CONTROL: without the flag the started servers were %v, want all three planted ones — claude no longer reads MCP where this pin plants it, so the strict arm proves nothing; re-measure ADR 0090's table on this release", got)
		}
	})
}
