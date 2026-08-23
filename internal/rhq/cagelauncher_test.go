package rhq

// The argv0 launcher (rangerhq-1k1). The claim under test is the one the
// spike measured: with `docker run …` on the pane line herdr answers
// `agent_not_found`, and with a launcher named for the runtime in front —
// exec'ing the engine with argv[0] reset — the caged session is an agent
// again. Everything here is that sentence taken apart: the launcher is a
// binary (a symlink to this one), it is not among the gate shims, the plan
// it reads names the engine, and the argv that reaches the engine really
// does start with `claude`.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cageLaunchArgs pulls the launcher path and the plan path back out of the
// line WrapInCage returns — the same two words the pane is handed.
func cageLaunchArgs(t *testing.T, line string) (launcher, plan string) {
	t.Helper()
	f := strings.SplitN(line, "' "+CageLaunchFlag+" '", 2)
	if len(f) != 2 || !strings.HasPrefix(f[0], "'") || !strings.HasSuffix(f[1], "'") {
		t.Fatalf("the pane line must be `<launcher> %s <plan>`: %s", CageLaunchFlag, line)
	}
	return strings.TrimPrefix(f[0], "'"), strings.TrimSuffix(f[1], "'")
}

// The end-to-end shape, with a real exec: the pane's line runs the
// launcher, the launcher becomes the engine, and the engine's argv[0] is
// the runtime's canonical name — which is the whole of what herdr reads to
// decide there is a claude in this pane.
func TestCageLauncherExecsTheEngineAsTheRuntime(t *testing.T) {
	a := cageApp(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The engine is this test binary in argv mode: it writes down the argv
	// it was exec'd with, argv[0] included.
	os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"),
		[]byte("command: "+self+" {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\n"), 0o644)
	ag := cageAgent(t, a, "")
	rt, _ := a.LoadRuntime("claude")
	dir := t.TempDir()
	inner := ag.RenderCommandFor(rt, "claude", TierStrong)
	line, err := a.WrapInCage(ag, rt, "s1", dir, inner, []string{"CLAUDE_CODE_OAUTH_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	launcher, plan := cageLaunchArgs(t, line)
	if launcher != a.CageLauncher("p", "claude") || plan != a.CageArgvFile("p", "s1") {
		t.Fatalf("launcher/plan paths: %s %s", launcher, plan)
	}

	out := filepath.Join(t.TempDir(), "argv")
	cmd := exec.Command(launcher, CageLaunchFlag, plan)
	cmd.Env = append(os.Environ(), "RHQ_CAGE_ARGV_OUT="+out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running the launcher: %v\n%s", err, b)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the launcher never reached the engine: %v", err)
	}
	argv := strings.Split(string(b), "\n")
	// This is the assertion the tier exists for.
	if argv[0] != "claude" {
		t.Errorf("the engine must be exec'd with argv[0]=claude, not %q — herdr identifies the pane's agent by that name and answers agent_not_found for anything else", argv[0])
	}
	if !argvHas(argv, "-w", dir) || !argvHas(argv, "-e", "CLAUDE_CODE_OAUTH_TOKEN") || !argvHas(argv, "-v", ag.Path+":"+ag.Path+":ro") {
		t.Errorf("the engine's own arguments must arrive intact:\n%q", argv)
	}
	if got := argv[len(argv)-3:]; got[0] != "sh" || got[1] != "-c" || !strings.HasPrefix(got[2], "exec claude ") {
		t.Errorf("the runtime's command must arrive as one argument, shelled inside the cage:\n%q", got)
	}
	if !strings.Contains(argv[len(argv)-1], "$(cat ") {
		t.Errorf("the PID expansion belongs to the container's shell (the mounts are same-path): %q", argv[len(argv)-1])
	}
}

// The two constraints the spike named, and the plan's own shape.
func TestCageLauncherIsABinaryOutsideTheGates(t *testing.T) {
	a := cageApp(t)
	ag := cageAgent(t, a, "")
	rt, _ := a.LoadRuntime("claude")
	dir := t.TempDir()
	// A leftover from the shell-script launch of rangerhq-9fv: it is not
	// what runs any more, and a rendering nothing runs misleads whoever
	// opens it next.
	os.MkdirAll(a.CageDir("p"), 0o755)
	stale := filepath.Join(a.CageDir("p"), "launch.sh")
	os.WriteFile(stale, []byte("#!/bin/sh\nexec docker run …\n"), 0o755)

	line, err := a.WrapInCage(ag, rt, "s1", dir, "claude --model 'm'", []string{"CLAUDE_CODE_OAUTH_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	launcher, plan := cageLaunchArgs(t, line)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the superseded launch.sh must be removed: %v", err)
	}
	// A binary or a symlink to one: a #!/bin/sh wrapper hands herdr
	// argv0=sh, so the launcher is this binary under another name.
	fi, err := os.Lstat(launcher)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the launcher must be a symlink: %v %v", fi, err)
	}
	target, _ := os.Readlink(launcher)
	self, _ := os.Executable()
	if target != self {
		t.Errorf("the launcher must point at this posse: %s ≠ %s", target, self)
	}
	// Its own directory. gates/<persona>/bin is a PATH of refusing shims;
	// a launcher named `claude` next to a gate named `git` is a collision.
	if filepath.Dir(launcher) == a.GatesDir("p")+"/bin" || strings.Contains(launcher, "/gates/") {
		t.Errorf("the launcher must not live in the gates bin: %s", launcher)
	}
	if filepath.Base(launcher) != "claude" {
		t.Errorf("the launcher is named for the runtime, because that name is what herdr matches: %s", launcher)
	}
	// Rendered fresh at every launch, like the gates: a stale symlink is
	// replaced, not trusted.
	os.Remove(launcher)
	os.Symlink("/nonexistent", launcher)
	if _, err := a.RenderCageLauncher("p", "claude"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.Readlink(launcher); got != self {
		t.Errorf("a stale launcher must be re-rendered: %s", got)
	}

	var got CageLaunchPlan
	b, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("the plan must be readable json: %v\n%s", err, b)
	}
	if !filepath.IsAbs(got.Path) || filepath.Base(got.Path) != "env" {
		t.Errorf("the plan execs the engine binary, resolved at render time: %+v", got)
	}
	if got.Argv[0] != "claude" || !argvHas(got.Argv, "-v", ag.Path+":"+ag.Path+":ro") {
		t.Errorf("argv[0] is the runtime's name and the engine's own arguments follow it: %q", got.Argv)
	}
	// The engine's own network flag joins the cage to the session's
	// --internal network, and the plan carries the route's own steps
	// (rangerhq-9d0).
	if !argvHas(got.Argv, "--network", EgressPrefix+"s1") || got.Egress == nil || got.Egress.Proxy != EgressPrefix+"s1" {
		t.Errorf("the caged session joins its egress network and the plan carries the route: %q %+v", got.Argv, got.Egress)
	}
	if !strings.Contains(got.Note, "rangerhq-1k1") || !strings.Contains(got.Line, "-e CLAUDE_CODE_OAUTH_TOKEN") {
		t.Errorf("the plan must say what it is and read back as a line: %+v", got)
	}

	// A runtime whose executable name could not be a launcher is refused
	// rather than launched into a session herdr cannot see.
	if _, err := a.RenderCageLauncher("p", "../claude"); err == nil {
		t.Error("a launcher name that is not a plain file name must refuse")
	}
	// And the launcher itself refuses a plan it cannot use, instead of
	// exec'ing something else.
	if err := RunCageLaunch([]string{"claude", CageLaunchFlag}); err == nil {
		t.Error("the launcher takes exactly one plan")
	}
	if err := RunCageLaunch([]string{"claude", CageLaunchFlag, filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Error("a missing plan must be an error")
	}
	empty := filepath.Join(t.TempDir(), "empty.argv")
	os.WriteFile(empty, []byte(`{"argv":[]}`), 0o644)
	if err := RunCageLaunch([]string{"claude", CageLaunchFlag, empty}); err == nil {
		t.Error("a plan with no engine must be an error")
	}
	if IsCageLaunch([]string{"posse", "list"}) || !IsCageLaunch([]string{"claude", CageLaunchFlag, "x"}) {
		t.Error("IsCageLaunch reads the flag, not the name")
	}
}

// The launcher execs, so when the container exits the pane's process is
// gone — there is no shell left holding the pane. A relaunch has to put
// the same short line back (rangerhq-1k1's note on RelaunchAgent).
func TestCagedRelaunchRetypesTheLauncher(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.CagesDir(), 0o755)
	os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"), []byte(
		"command: env {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\n"), 0o644)
	os.WriteFile(a.ConfigPath, []byte("default_engine: fake\n"), 0o644)
	os.MkdirAll(a.EnvsDir, 0o700)
	os.WriteFile(filepath.Join(a.EnvsDir, "container.env"), []byte("CLAUDE_CODE_OAUTH_TOKEN=sk-not-real\n"), 0o600)
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "caged.md"),
		[]byte("---\nname: caged\ndescription: test\ncage: container\nenvs: [container]\n---\nYou are caged.\n"), 0o644)

	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "cg", Agent: "caged", Dir: dir})
	m, ok := b.readMeta("cg")
	if !ok {
		t.Fatal("no session meta for cg")
	}
	m.Launched = time.Now().Add(-time.Hour)
	b.writeMeta(m)
	os.Remove(filepath.Join(fake, "agents.json"))
	if ok, err := b.RelaunchAgent("cg", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	want := "'" + a.CageLauncher("caged", "claude") + "' " + CageLaunchFlag + " '" + a.CageArgvFile("caged", "cg") + "'"
	if got := calls(t, fake); strings.Count(got, want) != 2 {
		t.Errorf("a relaunch must re-type the launcher line, not the engine's:\n%s", got)
	}
	// And the plan is re-rendered for the same session, not orphaned.
	if b2, err := os.ReadFile(a.CageArgvFile("caged", "cg")); err != nil || !strings.Contains(string(b2), dir) {
		t.Errorf("the plan must be re-rendered on relaunch: %v\n%s", err, b2)
	}
}
