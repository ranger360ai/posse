//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-3clx6, ranger-base-uc7ww: the launch-site WIRING
// behind ADR 0090 D1 and D2. Each of the four lines below has a pinned
// PRODUCER and a pinned RENDER, with one call or one assignment between
// them that no existing test reads — so reverting that one line leaves
// every arm green while the guarantee it carries is gone (docs/notes.d/
// ranger-base-3clx6.md has the class and the measurement).
//
// The shape every arm here follows is the fifth line's, the one that
// already had it right (TestBwrapRenderWiresTheRuntimeDirsIntoTheWall,
// bwraphide_qa_test.go): render POSSE'S OWN call site and ask what it
// produced, rather than calling the producer or the render directly the
// way every existing pin for these four lines does.
//
//	go test ./internal/posse -run 'LaunchSiteWiring' -v

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 1. herdrback.go planLaunch's call to MCPConfigWallRefusal. Every existing
// pin drives mcpConfigWallRefusalOn directly; nothing drives a LAUNCH
// through it. REPRO (ranger-base-3clx6, MEASURED): replace the three-line
// `if err := a.MCPConfigWallRefusal(...); err != nil { return nil, err }`
// block with `_ = a.MCPConfigWallRefusal` and all three arms stay green.
func TestLaunchSiteWiringPlanLaunchRefusesAnMCPConfigInsideTheWriteSet(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// Not claude: the refusal's own logic is pinned by mcpstrict_qa_test.go
	// at every spelling already. What is missing is a LAUNCH reaching the
	// call at all, and any runtime whose line names --mcp-config reaches it.
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
		[]byte("command: mycli --mcp-config .mcp.json {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := wallUngrantedDir(t)
	// .mcp.json resolves against cwd (mcpConfigWallRefusalOn), and cwd is
	// inside this seat's own write set — exactly the file ADR 0090 D1 says
	// a launch line must never be allowed to name.
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err == nil || !strings.Contains(err.Error(), "ADR 0090 D1") {
		t.Fatalf("planLaunch with a launch line naming --mcp-config .mcp.json, inside this seat's own write set, = %v, want a refusal naming ADR 0090 D1 — the call to MCPConfigWallRefusal at this launch site is read by no other test (ranger-base-3clx6, escaped from ranger-base-m0bfv)", err)
	}
	// The control: the same line, outside the write set, must launch clean —
	// otherwise the refusal above is not ABOUT the write set.
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
		[]byte("command: mycli --strict-mcp-config {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("control: a launch line naming no --mcp-config at all must not be refused: %v", err)
	}
}

// 2. herdrback.go RelaunchAgent's call to the same function, the same gap.
// The PID and the runtime file are both re-read from disk on this path
// (ADR 0076 D6/D7/D8's comment, herdrback.go), so the fixture below creates
// a session with a clean runtime line, then edits the runtime's OWN yaml
// before relaunching — the same technique credentialdirrelaunch_qa_test.go
// uses for the sibling credential-dir refusal on this path.
//
// REPRO (ranger-base-3clx6, MEASURED): replace
// `if err := b.App.MCPConfigWallRefusal(...); err != nil { return false, err }`
// with `_ = b.App.MCPConfigWallRefusal` and all three arms stay green.
func TestLaunchSiteWiringRelaunchAgentRefusesAnMCPConfigInsideTheWriteSet(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
		[]byte("command: mycli --strict-mcp-config {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: mycli\ncage: seatbelt\ndeny: [Bash(git push:*)]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := b.CreateSession(NewSessionOpts{Name: "rl", Dir: t.TempDir(), Agent: "ranger", AllowDegraded: true}); err != nil {
		t.Fatalf("the launch with a clean line must succeed: %v", err)
	}
	m, found := b.readMeta("rl")
	if !found {
		t.Fatal("no meta for the session that just launched")
	}
	if m.Cage != CageSeatbelt {
		t.Fatalf("the session came up at cage %q, not %q — NOTHING MEASURED: this refusal only exists where posse renders the wall", m.Cage, CageSeatbelt)
	}

	died := func() {
		t.Helper()
		m, found := b.readMeta("rl")
		if !found {
			t.Fatal("no meta for the session being killed")
		}
		m.Launched = time.Now().Add(-time.Hour)
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
		os.Remove(filepath.Join(fake, "agents.json")) // the CLI died
	}

	died()
	if ok, err := b.RelaunchAgent("rl", time.Second); err != nil || !ok {
		t.Fatalf("witness: a session whose runtime names no --mcp-config must re-type: ok=%v err=%v", ok, err)
	}

	// The runtime file is re-read fresh at relaunch — edited here to the
	// state a launch would have refused, reached only after the launch
	// already rendered a profile from the clean line above.
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "mycli.yaml"),
		[]byte("command: mycli --mcp-config .mcp.json {file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	died()
	ok, rerr := b.RelaunchAgent("rl", time.Second)
	if rerr == nil || !strings.Contains(rerr.Error(), "ADR 0090 D1") {
		t.Fatalf("a relaunch whose edited runtime names --mcp-config inside this seat's write set = ok=%v err=%v, want a refusal naming ADR 0090 D1 — the call to MCPConfigWallRefusal at THIS launch site is read by no other test (ranger-base-3clx6, escaped from ranger-base-m0bfv)", ok, rerr)
	}
	if ok {
		t.Error("RelaunchAgent reported it re-typed the session it had just refused")
	}
}

// 3. runtimecheck.go launchRow's use of strictMCPClause. The clause itself
// is pinned (TestQAStrictMCPDoorLeavesAWrapperAloneAndTheCheckSaysSo); no
// test asserts that the GRID `posse runtime check` prints actually carries
// what the clause returns. REPRO (ranger-base-3clx6, MEASURED): delete the
// `if mcp := strictMCPClause(rt.Command); mcp != "" { un += "; " + mcp }`
// block and all three arms stay green.
func TestLaunchSiteWiringLaunchRowCarriesTheStrictMCPClause(t *testing.T) {
	t.Parallel()
	a := checkApp(t)

	// A claude line wrapped behind a first word EnsureStrictMCPLine cannot
	// repair (lineRunsClaude true, filepath.Base(f[0]) != "claude") — the
	// "NO --strict-mcp-config" arm of strictMCPClause.
	missing := writeRuntime(t, a, "wrapped", "command: env FOO=1 claude {file}\n")
	var b1 strings.Builder
	a.RuntimeCheck(missing, Herdr{Bin: "no-such-herdr-binary"}, &b1)
	flat1 := strings.Join(strings.Fields(b1.String()), " ")
	if !strings.Contains(flat1, "NO "+ClaudeStrictMCPFlag) {
		t.Errorf("the launch row does not carry the missing-%s clause (ranger-base-3clx6, escaped from ranger-base-m0bfv):\n%s", ClaudeStrictMCPFlag, flat1)
	}

	// The present arm, so a reader of this pin cannot mistake "the grid
	// always says NO" for the fix.
	present := writeRuntime(t, a, "straight", "command: claude --strict-mcp-config {file}\n")
	var b2 strings.Builder
	a.RuntimeCheck(present, Herdr{Bin: "no-such-herdr-binary"}, &b2)
	flat2 := strings.Join(strings.Fields(b2.String()), " ")
	if !strings.Contains(flat2, ClaudeStrictMCPFlag+" on the line") {
		t.Errorf("the launch row does not carry the present-%s clause:\n%s", ClaudeStrictMCPFlag, flat2)
	}
}

// 4a. wall.go WallReport's own call to SeatbeltStateDirs(rt), separate from
// the launch's (renderWallOn, already pinned by seatbeltclaudeconfigdir_qa_
// test.go through sbcdProfile). Driven at the sandbox-exec mechanism from
// any host (wallMechanismForTest): WallReport is a pure render plus a print,
// so this runs on Linux exactly as it would on darwin.
//
// REPRO (ranger-base-3clx6, MEASURED): revert `SeatbeltStateDirs(rt)` to
// `rt.StateDirs` on this call and all three arms stay green.
func TestLaunchSiteWiringWallReportUsesTheSubstitutedClaudeStateDirs(t *testing.T) {
	// No t.Parallel: the fixture sets HOME, CLAUDE_CONFIG_DIR and
	// wallMechanismHost, all package-level for the duration of the test.
	f := weFixtureIn(t, wallUngrantedDir(t))
	wallMechanismForTest(t, "darwin")
	sbcdPlantHomeFile(t, f.home)
	elsewhere := filepath.Join(filepath.Dir(f.home), "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ClaudeConfigDirVar, elsewhere)

	var buf strings.Builder
	if err := f.a.WallReport(f.ag, f.rt, f.dir, &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	homeFile := filepath.Join(f.home, claudeConfigFileName)
	if strings.Contains(got, homeFile) {
		t.Errorf("WallReport still names %s with %s pointing elsewhere — that file is no seat's to write (ADR 0090 D2), and the call this report makes to SeatbeltStateDirs(rt) is its own, not the launch's (ranger-base-3clx6, escaped from ranger-base-ntnst):\n%s", homeFile, ClaudeConfigDirVar, got)
	}
	if !strings.Contains(got, elsewhere) {
		t.Errorf("WallReport does not name %s, the directory %s now names — reverting WallReport's own SeatbeltStateDirs(rt) call to rt.StateDirs leaves this report describing a write set the rendered profile does not grant (ADR 0090 D2, ranger-base-3clx6):\n%s", elsewhere, ClaudeConfigDirVar, got)
	}
}

// 4b. reachability.go's own call to SeatbeltStateDirs(rt), inside
// applyRecordReach's seatbelt-mechanism arm — reachable only where
// SeatbeltAvailable() is measured true, i.e. a real darwin host with
// sandbox-exec on PATH (the function hardcodes runtime.GOOS, so nothing
// drives this one arm from Linux the way WallReport's can be).
//
// recordTargets' own target (the store of record) is granted unconditionally
// by SeatbeltWritable whenever it sits outside the session's own tree,
// independent of state dirs — so a store placed there alone would never
// make this line's wiring observable. What DOES: claude's executable config
// (claudeExecConfig, wallexecconfig.go) denies a fixed set of subdirectories
// — "skills" among them — under whichever directory the SAME stateDirs
// argument resolves to be claude's config dir. Put the store under
// <CLAUDE_CONFIG_DIR>/skills and the two readings diverge: the correct call
// resolves claude's config dir to CLAUDE_CONFIG_DIR and denies the store
// (last-match-wins); the reverted call (rt.StateDirs, which never contains
// CLAUDE_CONFIG_DIR) denies nothing there, and the gate reads reachable over
// a profile that does not grant it.
func TestLaunchSiteWiringReachabilityUsesTheSubstitutedClaudeStateDirs(t *testing.T) {
	if !SeatbeltAvailable() {
		t.Skip("NOTHING MEASURED: this host has no sandbox-exec to probe applyRecordReach's seatbelt arm against")
	}
	root := sbRoot(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := sbMkdir(t, filepath.Join(root, "elsewhere"))
	t.Setenv(ClaudeConfigDirVar, elsewhere)
	work := sbMkdir(t, filepath.Join(root, "work"))
	store := sbMkdir(t, filepath.Join(elsewhere, "skills", "store"))
	sbWrite(t, filepath.Join(work, beadsDirName, beadsRedirect), filepath.Join(store, beadsDirName)+"\n")
	sbMkdir(t, filepath.Join(store, beadsDirName))

	a := NewAppAt(sbMkdir(t, filepath.Join(root, "posse-home")))
	ag := &AgentFile{Name: "security", Deny: []string{"Edit", "Write"}, Cage: CageSeatbelt,
		MemoryDir: sbMkdir(t, filepath.Join(a.PersonasDir(), "security"))}
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	_ = home

	p := a.CheckParityIn(ag, rt, CageSeatbelt, TierStrong, work)
	row := reachRow(p)
	if row == "" || !strings.Contains(row, "is not writable under the profile") {
		t.Fatalf("CheckParityIn's record-reach gate = %q, want a finding: the store of record sits under %s's own skills/ — denied read-only as claude's executable config (ADR 0090 D2, wallexecconfig.go) — and a profile rendered from reachability.go's own un-pinned SeatbeltStateDirs(rt) call never resolves that deny to %s at all (ranger-base-3clx6, escaped from ranger-base-ntnst)",
			row, ClaudeConfigDirVar, elsewhere)
	}
}
