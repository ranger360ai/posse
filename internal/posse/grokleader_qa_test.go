//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-76mcd: a grok seat must not use a grok leader.
//
// Three halves. The line pins say every grok line posse renders or is handed
// carries GrokNoLeaderFlag exactly once (grok refuses it twice). The render
// pins say grok's daemon/run/ is a private tmpfs on bwrap and a directory
// deny on seatbelt. The live pin runs the grok TUI, under a pty and under
// posse's own bwrap render, with `[cli] use_leader = true` in the seat's
// config.toml: posse's line starts no leader and leaves no leader.sock, and
// the same line without the door — the control — does both, which is what
// reds this pin if a grok release renames the flag or moves the socket.
//
//	go test ./internal/posse -run 'GrokLeader' -v

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func glOnce(t *testing.T, where, line string) {
	t.Helper()
	if n := strings.Count(" "+line+" ", " "+GrokNoLeaderFlag+" "); n != 1 {
		t.Errorf("%s: %s appears %d times, want once (grok refuses it twice, and a grok seat without it uses the operator's leader — ranger-base-76mcd):\n%s", where, GrokNoLeaderFlag, n, line)
	}
}

func TestQAGrokLeaderEveryGrokLineCarriesNoLeaderOnce(t *testing.T) {
	t.Parallel()
	for _, rt := range builtinRuntimes {
		if rt.Exe() == grokExe {
			glOnce(t, "built-in "+rt.Name+" template", rt.Command)
		} else if strings.Contains(rt.Command, GrokNoLeaderFlag) {
			t.Errorf("%s: %s is grok's flag, typed at %s", rt.Name, GrokNoLeaderFlag, rt.Exe())
		}
	}

	plain := loadTestAgent(t, "---\nname: p\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")
	grok := builtinRuntime(t, "grok")
	for _, tier := range Tiers {
		glOnce(t, "grok/"+tier, plain.RenderCommandFor(grok, "grok", tier))
	}
	for _, name := range []string{"claude", "codex"} {
		if got := plain.RenderCommandFor(builtinRuntime(t, name), name, DefaultTier); strings.Contains(got, GrokNoLeaderFlag) {
			t.Errorf("%s line carries grok's flag: %q", name, got)
		}
	}

	// A PID's own command: on grok — the template posse did not write.
	for _, c := range []struct{ name, command string }{
		{"bare", `grok --rules="$(cat {file})"`},
		{"own mode", `grok --permission-mode default {allow}`},
		{"already", `grok --no-leader --rules="$(cat {file})"`},
	} {
		ag := loadTestAgent(t, "---\nname: p\nruntime: grok\ncommand: "+c.command+"\n---\nYou are p.\n")
		glOnce(t, "command: "+c.name, ag.RenderCommandFor(grok, "grok", DefaultTier))
	}

	// The no-persona `--cmd` line.
	glOnce(t, "--cmd grok", EnsureUnattendedLine("grok"))
	glOnce(t, "--cmd absolute grok", EnsureUnattendedLine("/opt/bin/grok -m x"))
	glOnce(t, "--cmd grok, already", EnsureUnattendedLine("grok --no-leader"))
	for _, in := range []string{"claude", "codex", "env FOO=1 grok", "run {file}", ""} {
		if got := EnsureNoLeaderLine(in); got != in {
			t.Errorf("EnsureNoLeaderLine(%q) = %q, want unchanged: posse cannot see that program's dialect", in, got)
		}
	}
}

// grok's daemon/run/: a private empty tmpfs after the home grant on bwrap,
// covered by no bind from the host; a directory deny on seatbelt. Never for a
// codex seat, and GROK_HOME moves it.
func TestQAGrokLeaderDaemonRunIsPrivateOrReadOnly(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, t.TempDir())
	h := absResolve(filepath.Join(f.home, ".grok"))
	run := filepath.Join(h, "daemon", "run")

	rt := wrcRuntime("grok")
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, rt.StateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, rt.StateDirs...)
	if !containsString(c.Deny, run) || c.FileShaped(run) {
		t.Errorf("seatbelt: %s must be a directory deny (ranger-base-76mcd): deny=%v", run, c.Deny)
	}
	if prof := SeatbeltProfile(f.ag.Name, w, nil, c); !strings.Contains(prof, sbQuote(run)) {
		t.Errorf("seatbelt: the profile names no deny over %s", run)
	}

	r, err := f.a.bwrapRender(f.ag, rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	grant := bindIndex(r.Argv, h, h)
	if grant < 0 {
		t.Fatalf("the fixture's grok home is not granted, so nothing here is measured: %q", r.Argv)
	}
	if containsString(r.Carve.Deny, run) {
		t.Errorf("bwrap: %s is still a deny, and its read-only bind from the host would cover the private tmpfs", run)
	}
	at := -1
	for i := 0; i+1 < len(r.Argv); i++ {
		if r.Argv[i] == "--tmpfs" && r.Argv[i+1] == run {
			at = i
		}
		if (r.Argv[i] == "--ro-bind" || r.Argv[i] == "--bind") && i+2 < len(r.Argv) && r.Argv[i+2] == run {
			t.Errorf("bwrap: %s is bound from the host at %d — the seat would see the host's sockets", run, i)
		}
	}
	switch {
	case at < 0:
		t.Errorf("bwrap: no private tmpfs over %s (ranger-base-76mcd)\n argv: %q", run, r.Argv)
	case at < grant:
		t.Errorf("bwrap: the tmpfs over %s comes before the home grant at %d, so the grant covers it again", run, grant)
	}

	codex := wrcRuntime("codex")
	cw := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, codex.StateDirs...)
	if cc := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, cw, codex.StateDirs...); containsString(cc.Deny, run) {
		t.Errorf("codex: the carve names %s, which nothing granted this seat", run)
	}

	moved := filepath.Join(f.home, "elsewhere", "grok")
	t.Setenv(GrokHomeVar, moved)
	if got := grokEphemeralDirs(f.home, []string{absResolve(moved)}); len(got) != 1 || got[0] != filepath.Join(absResolve(moved), "daemon", "run") {
		t.Errorf("GROK_HOME=%s: %v", moved, got)
	}
	if got := grokEphemeralDirs(f.home, []string{h}); len(got) != 0 {
		t.Errorf("GROK_HOME moved away from the grant, yet %v", got)
	}
}

// glLeaders returns the pids of grok processes whose environment carries
// HOME=home — this fixture's home and nobody else's, so the kill that follows
// can only reach what this test started (AGENTS.md, "Ending anything").
func glLeaders(home string) (pids []int) {
	ents, _ := os.ReadDir("/proc")
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", e.Name(), "environ"))
		if err != nil || !bytes.Contains(append([]byte{0}, env...), []byte("\x00HOME="+home+"\x00")) {
			continue
		}
		cmd, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if bytes.Contains(cmd, []byte("agent\x00leader")) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// MEASURED 2026-11-13, grok 1.0.77, bubblewrap 0.13.1, D1 host: the control
// arm leaves leader.sock and a `grok agent leader --no-exit-on-disconnect`
// that outlives the TUI; posse's line leaves neither.
func TestLiveGrokLeaderUseLeaderTUIUnderTheWallStartsNoLeader(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	wrcNeedBwrap(t)
	bin := wrcRealBinary("grok")
	if bin == "" || filepath.Base(bin) != grokExe {
		t.Skip("NOTHING MEASURED: grok does not resolve to a binary named grok on this host")
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("NOTHING MEASURED: no `script` to give the TUI a pty: " + err.Error())
	}
	// /var/tmp, and short: a leader socket path over 108 bytes makes the
	// leader fail on its own (ranger-base-qyq59), which would pass
	// the control arm's absence for the wrong reason.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	h := filepath.Join(f.home, ".grok")
	sock := filepath.Join(h, "leader.sock")
	if len(sock) > 100 {
		t.Skipf("NOTHING MEASURED: %s is too long for a unix socket", sock)
	}
	if err := os.WriteFile(filepath.Join(h, "config.toml"), []byte("[cli]\nuse_leader = true\n\n[models]\ndefault = \"fake\"\n\n[model.fake]\nmodel = \"fake\"\nbase_url = \"http://127.0.0.1:9/v1\"\nenv_key = \"FAKE_KEY\"\napi_backend = \"chat_completions\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, pid := range glLeaders(f.home) {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	})
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("grok"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	door := EnsureUnattendedLine(bin + " -m fake")
	control := strings.Replace(door, " "+GrokNoLeaderFlag, "", 1)
	if control == door {
		t.Fatalf("the door did not add %s to %q", GrokNoLeaderFlag, door)
	}
	arm := func(name, line string) (sockSeen bool, leaders []int) {
		var q []string
		q = append(q, "bwrap")
		for _, a := range r.Argv {
			q = append(q, shellQuote(a))
		}
		cmd := exec.Command(script, "-qfec", strings.Join(q, " ")+" "+line, "/dev/null")
		cmd.Dir = f.dir
		cmd.Env = append(os.Environ(), "FAKE_KEY=sk-dummy", GrokDisableAutoUpdaterVar+"=1")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if st, err := os.Stat(sock); err == nil && st.Mode()&os.ModeSocket != 0 {
				sockSeen = true
				break
			}
		}
		if !sockSeen {
			time.Sleep(time.Second) // the door arm: give a leader every chance
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		time.Sleep(500 * time.Millisecond)
		leaders = glLeaders(f.home)
		t.Logf("%s arm: leader.sock seen=%v, leaders alive after the TUI=%v", name, sockSeen, leaders)
		return sockSeen, leaders
	}

	seen, leaders := arm("posse", door)
	if seen || len(leaders) != 0 {
		t.Errorf("posse's grok line, use_leader on, under the wall: leader.sock seen=%v, leaders=%v — the seat would hand its tools to a leader (ranger-base-76mcd)", seen, leaders)
	}
	seen, leaders = arm("control", control)
	if !seen || len(leaders) == 0 {
		t.Errorf("control arm (the line without %s): leader.sock seen=%v, leaders=%v, want both — a grok release moved leader mode, so this pin no longer proves the flag works", GrokNoLeaderFlag, seen, leaders)
	}
}
