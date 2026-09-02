package posse

// The L4 cage: the engine template, the mounts and env names that cross
// the boundary, the credential precondition, the seeded runtime state, and
// the parity claims the tier is allowed to make while it is still arriving
// in pieces (ADR 0002 §3, rangerhq-9fv).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cageApp is an App whose engine is hermetic: `env` is on PATH everywhere,
// takes the docker-shaped flags as plain arguments, and `true` answers the
// image probe without a container runtime in sight. Tests that want the
// tier *unavailable* point default_engine: at an engine whose binary is
// not there.
func cageApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	a := &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		EnvsDir: filepath.Join(home, "envs"), StateDir: filepath.Join(home, "state"),
		AgentsDir: filepath.Join(home, "agents"),
	}
	os.MkdirAll(a.CagesDir(), 0o755)
	os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"), []byte(
		"command: env {net} {mounts} {env} -w {workdir} {image} {cmd}\n"+
			"probe: true {image}\n"+
			// The egress route in the same hermetic shape: `env` takes the
			// docker-shaped words as plain arguments and nothing runs a
			// container. Spelling them is what makes this engine one that
			// CAN realize `egress:` (rangerhq-9d0).
			"net: --network {net}\n"+
			"net_create: env network create --internal {net}\n"+
			"net_join: env network connect bridge {proxy}\n"+
			"net_remove: env network rm {net}\n"+
			"proxy_up: env run -d --name {proxy} --hostname {host} --network {net} {mounts} {image} {cmd}\n"+
			"proxy_down: env rm -f {proxy}\n"), 0o644)
	os.WriteFile(a.ConfigPath, []byte("default_engine: fake\n"), 0o644)
	return a
}

func cageAgent(t *testing.T, a *App, front string) *AgentFile {
	t.Helper()
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "p.md"), []byte("---\nname: p\n"+front+"---\nYou are p.\n"), 0o644)
	ag, err := a.LoadAgent("p")
	if err != nil {
		t.Fatal(err)
	}
	return ag
}

func TestEngineIsATemplateNotDockerRun(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	// The built-in is still docker, and it is still reachable by name.
	if d, err := a.LoadEngine("docker"); err != nil || !d.Builtin || !strings.HasPrefix(d.Command, "docker run ") || d.Binary() != "docker" {
		t.Fatalf("built-in docker: %+v %v", d, err)
	}
	// config default_engine: chooses; cages/<name>.yaml defines.
	if a.ResolveEngine() != "fake" {
		t.Errorf("default_engine: not honoured: %q", a.ResolveEngine())
	}
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		t.Fatal(err)
	}
	// Unset keys keep docker's spellings — an engine that answers the same
	// CLI (OrbStack) is a swap, not a rewrite.
	if e.Mount != "-v {src}:{dst}" || e.MountRO != "-v {src}:{dst}:ro" || e.Env != "-e {var}" || e.Home != "/root" {
		t.Errorf("docker spellings must be the defaults: %+v", e)
	}
	if _, err := a.LoadEngine("nope"); err == nil {
		t.Error("an unknown engine must not resolve to docker silently")
	}
	os.WriteFile(filepath.Join(a.CagesDir(), "empty.yaml"), []byte("home: /x\n"), 0o644)
	if _, err := a.LoadEngine("empty"); err == nil {
		t.Error("an engine yaml with no command: must be an error")
	}
	// The engine's binary decides whether the tier exists at all.
	if !a.ContainerAvailable() {
		t.Error("engine `env` is on PATH — the tier must be available")
	}
	os.WriteFile(filepath.Join(a.CagesDir(), "ghost.yaml"), []byte("command: no-such-engine-binary run {cmd}\n"), 0o644)
	os.WriteFile(a.ConfigPath, []byte("default_engine: ghost\n"), 0o644)
	if a.ContainerAvailable() {
		t.Error("an engine whose binary is missing must not read as available")
	}
}

func TestCageRenderMountsAndEnvNames(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	ag := cageAgent(t, a, "skills: [dataviz]\n")
	e, _ := a.LoadEngine("fake")
	dir := t.TempDir()
	ms := a.CageMounts(ag, e, dir, "s1")
	// The repo, the memory dir, the PID (read-only), the skills tree
	// (read-only), the cage HOME, this session's refusals spool the inner
	// gates append to (ADR 0025 §4, rangerhq-6so) — and nothing else of the
	// host.
	if len(ms) != 6 {
		t.Fatalf("want exactly the six mounts, got %d: %+v", len(ms), ms)
	}
	for _, m := range ms[:3] {
		if m.Src != m.Dst {
			t.Errorf("mounts are same-path in and out: %+v", m)
		}
	}
	if ms[0].Src != dir || ms[0].RO || ms[1].Src != ag.MemoryDir || ms[2].Src != ag.Path || !ms[2].RO {
		t.Errorf("repo rw, memory rw, PID ro: %+v", ms[:3])
	}
	if ms[3].Src != ag.SkillsStateDir || !ms[3].RO {
		t.Errorf("bound skills ride in read-only: %+v", ms[3])
	}
	if ms[4].Src != a.CageHome("p") || ms[4].Dst != "/root" {
		t.Errorf("the cage HOME mounts at the image's HOME: %+v", ms[4])
	}
	// The spool, never the canonical log (ADR 0025 §4): rendered inside,
	// folded into the host's audit trail by a host process, never by the
	// cage.
	if ms[5].Src != a.CageSpoolPath("p", "s1") || ms[5].Dst != CageGatesDir("p")+"/refusals.log" || ms[5].RO {
		t.Errorf("the refusals spool mounts out of the cage, writable, never the canonical log: %+v", ms[5])
	}
	// A PID that binds no skills gets no skills mount.
	if got := a.CageMounts(cageAgent(t, a, ""), e, dir, "s1"); len(got) != 5 {
		t.Errorf("no skills bound → no skills mount: %+v", got)
	}

	// Env crosses as NAMES ONLY: the typed line lands in the pane's
	// scrollback, so a credential's value must never be on it.
	names := CageEnvNames([]EnvVar{{"CLAUDE_CODE_OAUTH_TOKEN", "sk-secret"}, {"BAD NAME", "x"}})
	argv := e.RenderArgv(CageRender{Name: "posse-s", Image: "img:tag", Workdir: dir,
		Inner: []string{"sh", "-c", "exec claude --model 'm'"}, Mounts: ms, Env: names})
	if argv[0] != "env" {
		t.Errorf("the engine's own binary leads the argv: %q", argv)
	}
	for _, want := range [][]string{
		{"-e", "RHQ_HOME"}, {"-e", "RHQ_LAUNCH_HOME"}, {"-e", "BD_ACTOR"}, {"-e", "RHQ_TOOLS_DENY"}, {"-e", "CLAUDE_CODE_OAUTH_TOKEN"},
		{"-w", dir}, {"img:tag"},
		{"-v", ag.Path + ":" + ag.Path + ":ro"}, {"-v", dir + ":" + dir},
	} {
		if !argvHas(argv, want...) {
			t.Errorf("rendered argv missing %q:\n%q", want, argv)
		}
	}
	for _, a := range argv {
		if strings.Contains(a, "{") || strings.Contains(a, "sk-secret") || a == "BAD NAME" || a == "" {
			t.Errorf("unrendered placeholder, a secret value, an unquotable name or an empty argument: %q in %q", a, argv)
		}
	}
	// The inner command is last and keeps its shell — inside the container,
	// where the same-path mounts make `$(cat …)` read the same file.
	if got := argv[len(argv)-3:]; got[0] != "sh" || got[1] != "-c" || got[2] != "exec claude --model 'm'" {
		t.Errorf("the runtime command must end the argv, shelled and exec'd: %q", got)
	}
	// Nothing on this path is shell-quoted, so a path with a space in it is
	// still exactly one argument — which is the shape a launcher execs.
	spaced := []CageMount{{Src: "/a b/repo", Dst: "/a b/repo"}}
	if got := e.RenderArgv(CageRender{Name: "n", Image: "i", Workdir: "/a b/repo", Mounts: spaced}); !argvHas(got, "-v", "/a b/repo:/a b/repo") || !argvHas(got, "-w", "/a b/repo") {
		t.Errorf("a path with a space must stay one argument: %q", got)
	}
	// {net} is the egress route's join (rangerhq-9d0): a render with no
	// network renders no argument at all, rather than an empty one.
	if argvHas(argv, "{net}") || argvHas(argv, "") || argvHas(argv, "--network") {
		t.Errorf("no network in the render → {net} renders to nothing: %q", argv)
	}
	// And the display spelling of that same argv is the operator's, not
	// something anything execs — it quotes only what a shell would need.
	line := CageLine(argv)
	if !strings.Contains(line, " -e BD_ACTOR ") || !strings.Contains(line, "'exec claude --model '\\''m'\\''") {
		t.Errorf("CageLine must read back as a shell line:\n%s", line)
	}
}

// argvHas reports whether want appears as consecutive arguments.
func argvHas(argv []string, want ...string) bool {
	for i := 0; i+len(want) <= len(argv); i++ {
		hit := true
		for j, w := range want {
			if argv[i+j] != w {
				hit = false
				break
			}
		}
		if hit {
			return true
		}
	}
	return false
}

// ADR 0002 §4: auth is not a gate but it is a precondition — the operator
// mints the credential (rangerhq-kiz) and the launch refuses without it
// rather than spending a session that cannot reach its API.
func TestCageCredentialPrecondition(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, _ := a.LoadRuntime("claude")
	if got := CageCredential(claude); got != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Errorf("claude's container credential: %q", got)
	}
	if err := CheckCageCredential(claude, []string{"BD_ACTOR", "CLAUDE_CODE_OAUTH_TOKEN"}); err != nil {
		t.Errorf("credential present must pass: %v", err)
	}
	err := CheckCageCredential(claude, []string{"BD_ACTOR"})
	if err == nil || !strings.Contains(err.Error(), "claude setup-token") {
		t.Errorf("missing credential must refuse and say how to mint it: %v", err)
	}
	// The metered key was rejected on the money line, so it does not
	// stand in for the token.
	if err := CheckCageCredential(claude, []string{"ANTHROPIC_API_KEY"}); err == nil {
		t.Error("ANTHROPIC_API_KEY is metered spending and was rejected as the container credential")
	}
	// codex/grok: undecided, so the launch says so instead of starting an
	// unauthenticated session.
	for _, n := range []string{"codex", "grok"} {
		rt, _ := a.LoadRuntime(n)
		if err := CheckCageCredential(rt, []string{"CLAUDE_CODE_OAUTH_TOKEN"}); err == nil || !strings.Contains(err.Error(), "rangerhq-kiz") {
			t.Errorf("%s's container credential is undecided: %v", n, err)
		}
	}
	// A template-only runtime names its own.
	os.MkdirAll(a.RuntimesDir(), 0o755)
	os.WriteFile(filepath.Join(a.RuntimesDir(), "own.yaml"), []byte("command: own {file}\ncage_cred: OWN_TOKEN\n"), 0o644)
	own, err := a.LoadRuntime("own")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckCageCredential(own, []string{"OWN_TOKEN"}); err != nil {
		t.Errorf("cage_cred: is the template-only runtime's answer: %v", err)
	}
}

// A fresh container has no ~/.claude.json, so claude opens the
// theme/onboarding wizard and treats the workdir as untrusted — the same
// dialog-nobody-is-watching failure ADR 0002 seeds codex's trust against.
func TestSeedCageHome(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	ag := cageAgent(t, a, "")
	claude, _ := a.LoadRuntime("claude")
	dir := "/work/repo"
	home, err := a.SeedCageHome(ag, claude, dir)
	if err != nil {
		t.Fatal(err)
	}
	read := func() map[string]any {
		b, err := os.ReadFile(filepath.Join(home, ".claude.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := read()
	if m["hasCompletedOnboarding"] != true || m["theme"] == nil || m["autoUpdates"] != false {
		t.Errorf("onboarding/theme/autoUpdates must be seeded: %v", m)
	}
	proj := m["projects"].(map[string]any)[dir].(map[string]any)
	if proj["hasTrustDialogAccepted"] != true || proj["hasCompletedProjectOnboarding"] != true {
		t.Errorf("the workdir must start trusted: %v", proj)
	}
	// claude writes its own state there between launches; the seed merges
	// rather than resetting the persona's container HOME every time.
	m["userID"] = "kept"
	b, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(home, ".claude.json"), b, 0o644)
	if _, err := a.SeedCageHome(ag, claude, "/work/other"); err != nil {
		t.Fatal(err)
	}
	m = read()
	if m["userID"] != "kept" || m["projects"].(map[string]any)[dir] == nil || m["projects"].(map[string]any)["/work/other"] == nil {
		t.Errorf("re-seeding must merge, not overwrite: %v", m)
	}
	// codex is seeded on the command line and grok needs nothing.
	codex, _ := a.LoadRuntime("codex")
	h2 := t.TempDir()
	a2 := &App{Home: h2, ConfigPath: filepath.Join(h2, "config.yaml"), StateDir: filepath.Join(h2, "state")}
	if _, err := a2.SeedCageHome(ag, codex, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a2.CageHome("p"), ".claude.json")); err == nil {
		t.Error("only claude gets a seeded .claude.json")
	}
}

// The tier lands in pieces, and the ADR's rule is that the strongest cage
// is never the one that silently loses `git push`. rangerhq-6so renders L1
// inside and mounts the repo :ro; L3 rides the repo mount but is claimed
// only by CheckParityIn after executing that repo's hook. The gate no tier
// realizes (a stdio MCP server, which never leaves the cage) is still
// refused however strong the tier's name is.
func TestContainerParityClaimsOnlyWhatItHolds(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit, Write, Bash(git push:*), WebFetch, mcp__probe__x]\negress: [api.example.com]\n")
	p := a.CheckParity(ag, claude, CageContainer, TierStrong)
	// The egress route is built, so it is claimed — and the fetch gate is
	// claimed only to the edge of the allowlist, which the degraded list
	// says out loud when a PID sets both (rangerhq-rm5).
	if !strings.Contains(p.Realized["egress: api.example.com"].Detail, "--internal network") {
		t.Errorf("the egress route is realized at this tier now: %+v", p.Realized)
	}
	if p.Realized["WebFetch"].Detail == "" {
		t.Errorf("WebFetch is realized as far as egress: goes: %+v", p.Realized)
	}
	if !strings.Contains(strings.Join(p.Degraded, "\n"), "not a fetch through an allowed one") {
		t.Errorf("egress: + WebFetch must say what it does not buy: %+v", p.Degraded)
	}
	// The tiers are cumulative in gates realized (ADR 0002 §3): the shell
	// verb keeps its shim, rendered inside, and Edit/Write is the mount
	// boundary that replaces L2. This directory-independent check does not
	// claim an unprobed pre-push hook.
	if !strings.Contains(p.Realized["Bash(git push:*)"].Detail, "rendered inside the cage") ||
		strings.Contains(p.Realized["Bash(git push:*)"].Detail, "pre-push hook") {
		t.Errorf("the shell verb is realized by L1 here; unprobed L3 must be absent: %+v", p.Realized)
	}
	if !strings.Contains(p.Realized["Edit"].Detail, ":ro") || !strings.Contains(p.Realized["Write"].Detail, ":ro") {
		t.Errorf("Edit/Write are the mount boundary at this tier: %+v", p.Realized)
	}
	// A stdio MCP server is a child process inside the cage: the allowlist
	// never sees it, so the tier must not claim it.
	found := false
	for _, u := range p.Unrealized {
		if strings.HasPrefix(u, "mcp__probe__x") && strings.Contains(u, "need never leave the container") {
			found = true
		}
	}
	if !found {
		t.Errorf("a tool-name deny is realized by no tier:\n  %s", strings.Join(p.Unrealized, "\n  "))
	}
	// An engine that spells no route cannot realize the gate however
	// strong the tier is — the claim belongs to the mechanism, not to the
	// tier's name.
	os.WriteFile(a.ConfigPath, []byte("default_engine: routeless\n"), 0o644)
	os.WriteFile(filepath.Join(a.CagesDir(), "routeless.yaml"), []byte("command: echo run {cmd}\n"), 0o644)
	pr := a.CheckParity(ag, claude, CageContainer, TierStrong)
	if !strings.Contains(strings.Join(pr.Unrealized, "\n"), "spells no --internal network") {
		t.Errorf("an engine with no net_create:/proxy_up: cannot realize egress:: %+v", pr.Unrealized)
	}
	// Below the tier, the shims are the host's and really do hold — the
	// container claims must not have cost the tier that works.
	ps := a.CheckParity(ag, claude, CageShims, TierStrong)
	if ps.Realized["Bash(git push:*)"].Detail == "" {
		t.Errorf("L1 still realizes the shell verb at shims: %+v", ps)
	}
	// And an unavailable container tier degrades to that host launch, so
	// it reads the same way: the shim claim is true again.
	os.WriteFile(a.ConfigPath, []byte("default_engine: ghost\n"), 0o644)
	os.WriteFile(filepath.Join(a.CagesDir(), "ghost.yaml"), []byte("command: no-such-engine-binary run {cmd}\n"), 0o644)
	pu := a.CheckParity(ag, claude, CageContainer, TierStrong)
	if pu.Realized["Bash(git push:*)"].Detail == "" {
		t.Errorf("a cage that cannot be provided falls back to the host wall: %+v", pu)
	}
	if !strings.Contains(strings.Join(pu.Degraded, "\n"), "cage container is not available") {
		t.Errorf("and says the tier is unavailable: %+v", pu.Degraded)
	}
}

// The launch end to end: the typed line is the engine's, the runtime's
// command is inside it, and the host's gate prefix — whose shims exec host
// paths and whose gate shell is the host's zsh — is not on it.
func TestCagedLaunchTypesTheEngineLine(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.CagesDir(), 0o755)
	os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"), []byte(
		"command: env {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\n"), 0o644)
	os.WriteFile(a.ConfigPath, []byte("default_engine: fake\n"), 0o644)
	os.MkdirAll(a.EnvsDir, 0o700)
	os.WriteFile(filepath.Join(a.EnvsDir, "container.env"), []byte("CLAUDE_CODE_OAUTH_TOKEN=sk-not-real\n"), 0o600)
	os.MkdirAll(a.AgentsDir, 0o755)
	pid := "---\nname: caged\ndescription: test\ncage: container\nenvs: [container]\n---\nYou are caged.\n"
	os.WriteFile(filepath.Join(a.AgentsDir, "caged.md"), []byte(pid), 0o644)

	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "cg", Agent: "caged", Dir: dir})
	log := calls(t, fake)
	typed := ""
	for _, ln := range strings.Split(log, "\n") {
		if strings.HasPrefix(ln, "pane run ") {
			typed = ln
		}
	}
	// The pane is handed the argv0 launcher and its plan, not the engine
	// invocation: herdr identifies the agent by the pane's argv0
	// (rangerhq-1k1), and an engine line runs past 1.5KB, which is lost
	// when it is typed into a freshly created workspace whose shell is
	// still starting (rangerhq-ybec).
	launcher := a.CageLauncher("caged", "claude")
	plan := a.CageArgvFile("caged", "cg")
	if typed != "pane run w1:p1 '"+launcher+"' --posse-cage '"+plan+"'" {
		t.Fatalf("the typed line must be the launcher and its plan:\n%s", log)
	}
	// A binary or a symlink to one — a #!/bin/sh wrapper would hand herdr
	// argv0=sh — and in its own directory, never the gates bin, whose
	// entries are refusing shims on the session's PATH.
	target, err := os.Readlink(launcher)
	if err != nil {
		t.Fatalf("the launcher must be a symlink to this binary: %v", err)
	}
	if self, _ := os.Executable(); target != self {
		t.Errorf("the launcher points at %s, not at this posse (%s)", target, self)
	}
	if strings.Contains(launcher, a.GatesDir("caged")) {
		t.Errorf("the launcher must not live among the gate shims: %s", launcher)
	}
	b3, err := os.ReadFile(plan)
	if err != nil {
		t.Fatalf("launch plan: %v", err)
	}
	var got CageLaunchPlan
	if err := json.Unmarshal(b3, &got); err != nil {
		t.Fatalf("launch plan is not json: %v\n%s", err, b3)
	}
	// The engine is what gets exec'd; the runtime's name is what herdr
	// sees, and it is the only thing argv[0] may be.
	if !strings.HasSuffix(got.Path, "/env") || got.Argv[0] != "claude" {
		t.Errorf("plan must exec the engine with argv[0]=claude: %+v", got)
	}
	typed = strings.Join(got.Argv, " ")
	if !strings.Contains(typed, "claude ") || !strings.Contains(typed, "--append-system-prompt") {
		t.Errorf("the runtime's own command must ride inside it:\n%s", typed)
	}
	if strings.Contains(typed, `:"$PATH" `) || strings.Contains(typed, "GATES ") {
		t.Errorf("the host gate prefix must not be on the caged line:\n%s", typed)
	}
	// The value reaches the session the way every env set does — through
	// the workspace's environment — and the engine forwards it by NAME, so
	// nothing secret is on the line typed into the pane, nor in the plan.
	if !strings.Contains(typed, "-e CLAUDE_CODE_OAUTH_TOKEN") || strings.Contains(string(b3), "sk-not-real") {
		t.Errorf("the credential crosses as a name, never as a value:\n%s", b3)
	}
	// One plan per session, because a persona holds one session per bead
	// and their mounts and workdirs differ.
	dir2 := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "cg-two", Agent: "caged", Dir: dir2})
	b4, err := os.ReadFile(a.CageArgvFile("caged", "cg-two"))
	if err != nil || !strings.Contains(string(b4), dir2) || strings.Contains(string(b4), dir) {
		t.Errorf("a second session must get its own plan: %v\n%s", err, b4)
	}
	if b5, err := os.ReadFile(plan); err != nil || !strings.Contains(string(b5), dir) {
		t.Errorf("and must not have overwritten the first: %v\n%s", err, b5)
	}

	// The seed is on disk, per persona, naming the session dir.
	seed, err := os.ReadFile(filepath.Join(a.CageHome("caged"), ".claude.json"))
	if err != nil || !strings.Contains(string(seed), dir) {
		t.Errorf("the caged HOME must be seeded for the session dir: %v %s", err, seed)
	}

	// Without the credential in the session's env, the launch refuses and
	// creates nothing — ADR 0002 §4's "nothing is spent".
	os.WriteFile(filepath.Join(a.AgentsDir, "broke.md"),
		[]byte("---\nname: broke\ndescription: test\ncage: container\n---\nYou are broke.\n"), 0o644)
	err = b.CreateSession(NewSessionOpts{Name: "cg2", Agent: "broke", Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "CLAUDE_CODE_OAUTH_TOKEN") {
		t.Fatalf("a caged launch with no credential must refuse: %v", err)
	}
	if strings.Contains(calls(t, fake), "workspace create cg2") {
		t.Error("and must not have created the workspace first")
	}
}

// Two facts, reported as one until ranger-base-1mu9r: `probe:` against an
// engine that does not answer fails with a socket-connect error, and it
// fails identically for a tag that certainly exists and one that certainly
// cannot (measured), so every reader of the pair blamed a missing image for
// a missing engine and advised a build that runs through the same engine.
// This pins the three answers apart, and pins the ORDER that keeps liveness
// out of the verdict.
func TestCageNotReadyNamesTheEngineAndNotTheImage(t *testing.T) {
	t.Parallel()
	// `env` leads every command: on PATH, and it runs nothing.
	load := func(t *testing.T, yaml string) (*App, *Engine) {
		t.Helper()
		a := cageApp(t)
		if err := os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"),
			[]byte("command: env {image} {cmd}\n"+yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		e, err := a.LoadEngine("fake")
		if err != nil {
			t.Fatal(err)
		}
		return a, e
	}

	// The engine that cannot be asked answers YES — `probe:`'s precedent.
	// "Could not ask" is not "the answer is no", and an engine with no
	// liveness spelling must behave exactly as it did before this existed.
	if (&App{}).CageEngineLive(&Engine{Name: "mute", Command: "env {cmd}"}) != true {
		t.Error("an engine with no live: must read as live, not as dead")
	}
	if (&App{}).CageEngineLive(nil) != true {
		t.Error("no engine at all is not a dead engine")
	}
	// …and the built-in, plus every yaml engine that does not overrule it,
	// CAN be asked: docker's spellings are the defaults here as they are for
	// probe: and build:, because the engine a yaml swaps in is normally one
	// answering the same CLI (OrbStack).
	if d, err := (&App{}).LoadEngine("docker"); err != nil || d.Live != "docker version" {
		t.Errorf("the built-in must carry a liveness spelling: %+v %v", d, err)
	}
	if _, e := load(t, ""); e.Live != "docker version" {
		t.Errorf("an unset live: inherits docker's spelling: %q", e.Live)
	}

	// A live engine and a built image: ready, and nothing to say.
	if a, e := load(t, "probe: true {image}\nlive: true\n"); a.CageNotReady(e, "img:1") != "" {
		t.Errorf("engine answering + image present must be ready: %q", a.CageNotReady(e, "img:1"))
	}
	// The order, which is the design: an image the engine described is proof
	// the engine answered, so a `live:` that says otherwise must not be able
	// to refuse a launch that works. Liveness picks WORDS, never the verdict.
	if a, e := load(t, "probe: true {image}\nlive: false\n"); a.CageNotReady(e, "img:1") != "" {
		t.Errorf("a wrong live: must not gate a host whose image probe answers: %q", a.CageNotReady(e, "img:1"))
	}

	// The bug: probe fails and the engine is not there to have answered it.
	a, e := load(t, "probe: false {image}\nlive: false\n")
	dead := a.CageNotReady(e, "img:1")
	if !strings.Contains(dead, "fake") || !strings.Contains(dead, "nothing answers it") {
		t.Errorf("a dead engine must be named as the reason: %q", dead)
	}
	if strings.Contains(dead, "is not built") || strings.Contains(dead, "run `posse cage build`") {
		t.Errorf("a dead engine must not be reported as a missing image, and must not advise a build that needs it: %q", dead)
	}
	if a.CageEngineNotReady(e) != dead {
		t.Errorf("the image-free caller gets the same sentence: %q vs %q", a.CageEngineNotReady(e), dead)
	}

	// The other half still works: the engine answers and the image is absent.
	a, e = load(t, "probe: false {image}\nlive: true\n")
	if got, want := a.CageNotReady(e, "img:1"), "image img:1 is not built — run `posse cage build`"; got != want {
		t.Errorf("a live engine with no image keeps its build instruction: %q", got)
	}
	if a.CageEngineNotReady(e) != "" {
		t.Errorf("the engine is fine; only the image is missing: %q", a.CageEngineNotReady(e))
	}

	// And the first question of the three is still the binary: nothing is
	// probed on a host where the engine is not installed at all.
	a, e = load(t, "probe: false {image}\nlive: false\n")
	e.Command = "no-such-engine-binary run {cmd}"
	for _, got := range []string{a.CageNotReady(e, "img:1"), a.CageEngineNotReady(e)} {
		if !strings.Contains(got, "not on PATH") || strings.Contains(got, "img:1") {
			t.Errorf("a missing binary is neither a dead engine nor a missing image: %q", got)
		}
	}
}
