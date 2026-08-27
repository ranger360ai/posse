package rhq

// L1/L3 inside the cage, the mount boundary and `sockets:` (ADR 0002 §3 as
// amended by rangerhq-rm5; bead rangerhq-6so). These are the hermetic
// halves — what is rendered, what crosses the boundary, and what parity is
// therefore allowed to claim. The property that needs a real container and
// a real repo (verification 8 and 10) is pinned live in cageinnerlive_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// envOf reads a name out of a rendered environment.
func envOf(t *testing.T, env []string, key string) string {
	t.Helper()
	return envGet(env, key)
}

// The inner command's whole job: render the persona's gates HERE — against
// this side's PATH and this side's shell — and become the runtime behind
// the same typed prefix the host types.
func TestGatesWrapRendersInsideAndTypesTheHostsPrefix(t *testing.T) {
	a := &App{StateDir: t.TempDir()}
	deny := []string{"Bash(git push:*)", "Edit"}
	env := []string{"PATH=/usr/bin:/bin", "RHQ_PERSONA=p", "RHQ_GATES_DIR=/host/gates/p"}
	w, err := a.PrepareGatesWrap("p", deny, false, []string{"sh", "-c", "exec claude"}, env)
	if err != nil {
		t.Fatal(err)
	}
	// The shim exists and was resolved on THIS side: the whole reason the
	// host's gates dir must not be mounted in is that its `git` execs
	// /opt/homebrew/bin/git, which no Linux image has.
	shim := filepath.Join(w.Bin, "git")
	b, err := os.ReadFile(shim)
	if err != nil {
		t.Fatalf("the deny list must render a shim inside: %v", err)
	}
	real := resolveOutside("git", w.Bin)
	if real != "" && !strings.Contains(string(b), real) {
		t.Errorf("the shim must exec the git resolved on THIS side (%s):\n%s", real, b)
	}
	// The typed prefix, as environment rather than as a shell line — the
	// wrap execs, so there is no shell to type it into.
	if got := envOf(t, w.Env, "PATH"); !strings.HasPrefix(got, w.Bin+":") {
		t.Errorf("the shim dir must win the PATH race: %q", got)
	}
	// Replaced, not appended: execve takes the last, but a duplicate PATH in
	// a process's environment is what misleads the next debugging session.
	n := 0
	for _, kv := range w.Env {
		if strings.HasPrefix(kv, "PATH=") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("PATH must appear exactly once, got %d: %q", n, w.Env)
	}
	if envOf(t, w.Env, "SHELL") != w.Shell || envOf(t, w.Env, "GROK_SHELL") != w.Shell || w.Shell == "" {
		t.Errorf("SHELL/GROK_SHELL must point at the gate shell (ADR 0009): %q", w.Env)
	}
	// RHQ_GATES_DIR is what the pre-push hook on the repo mount appends its
	// refusal to. Inside the cage it has to name the inner render, not the
	// host path it arrived carrying.
	if got := envOf(t, w.Env, "RHQ_GATES_DIR"); got != w.Gates || got == "/host/gates/p" {
		t.Errorf("RHQ_GATES_DIR must name the inner render: %q (rendered %q)", got, w.Gates)
	}
	if w.Path == "" || w.Argv[0] != "sh" {
		t.Errorf("the wrap execs what follows `--`, argv intact: %q %q", w.Path, w.Argv)
	}
	// A runtime that opts out of the gate shell (ADR 0009 §2) gets the same
	// prefix minus the two vars — the host's rule, applied on this side.
	ns, err := a.PrepareGatesWrap("p", deny, true, []string{"sh", "-c", "x"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if ns.Shell != "" || envOf(t, ns.Env, "SHELL") != "" || envOf(t, ns.Env, "GROK_SHELL") != "" {
		t.Errorf("--no-gate-shell must drop SHELL/GROK_SHELL: %q", ns.Env)
	}
	if _, err := a.PrepareGatesWrap("p", deny, false, nil, env); err == nil {
		t.Error("nothing after `--` must be an error, not an exec of the empty string")
	}
}

// The deny list crosses the boundary as RHQ_TOOLS_DENY, because RHQ_HOME
// does not cross at all — the same var the pre-push hook on the repo mount
// reads, so both layers inside the cage answer to one source.
func TestDenyCrossesTheBoundaryAsTheEnvVar(t *testing.T) {
	t.Setenv("RHQ_TOOLS_DENY", "Bash(git push:*)\n\n  Edit  \n")
	got := DenyFromEnv()
	if len(got) != 2 || got[0] != "Bash(git push:*)" || got[1] != "Edit" {
		t.Errorf("newline-joined, trimmed, blanks dropped: %q", got)
	}
	t.Setenv("RHQ_TOOLS_DENY", "")
	if got := DenyFromEnv(); len(got) != 0 {
		t.Errorf("a PID with no denies renders no shims: %q", got)
	}
}

// A gate shell whose REAL cannot be exec'd is a dead wall, and a dead wall
// is a shell verb that is not refused. Inside the image $SHELL is unset and
// /bin/zsh is not there, so the renderer resolves a shell that exists.
func TestGateShellResolvesAShellThatIsReallyThere(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("SHELL", filepath.Join(t.TempDir(), "zsh")) // named like a shell, not there
	real, base := realShell(bin)
	if _, err := os.Stat(real); err != nil {
		t.Fatalf("realShell must resolve something that exists, got %s: %v", real, err)
	}
	if base != filepath.Base(real) && base != "sh" {
		t.Errorf("the basename must be the shell's own (a runtime picks its dialect from it): %s %s", real, base)
	}
	// A $SHELL that IS there is still the answer — the host's behaviour must
	// not have changed under it.
	if _, err := os.Stat("/bin/sh"); err == nil {
		t.Setenv("SHELL", "/bin/zsh")
		if _, err := os.Stat("/bin/zsh"); err == nil {
			if r, b := realShell(bin); r != "/bin/zsh" || b != "zsh" {
				t.Errorf("an existing $SHELL wins: %s %s", r, b)
			}
		}
	}
}

// The mount boundary — ADR 0002 §4's successor to L2, which cannot wrap a
// container. The repo alone goes read-only: a persona that may not edit the
// work still keeps its own notes and the runtime still keeps its state.
func TestMountBoundaryIsTheRepoAndOnlyTheRepo(t *testing.T) {
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := t.TempDir()
	ag := cageAgent(t, a, "cage: container\ndeny: [Write]\n")
	ms := a.CageMounts(ag, e, dir)
	if !ms[0].RO || !strings.Contains(ms[0].Why, "READ-ONLY") {
		t.Errorf("a PID denying Write gets the repo :ro: %+v", ms[0])
	}
	for _, m := range ms[1:] {
		switch m.Src {
		case ag.MemoryDir, a.CageHome("p"), a.RefusalsLogPath("p"):
			if m.RO {
				t.Errorf("memory, HOME and the refusals log stay writable: %+v", m)
			}
		}
	}
	// The engine's read-only spelling is what actually renders it.
	argv := e.RenderArgv(CageRender{Image: "img", Workdir: dir, Mounts: ms, Inner: []string{"x"}})
	if !argvHas(argv, "-v", dir+":"+dir+":ro") {
		t.Errorf("the boundary is the mount flag, not a claim: %q", argv)
	}
	// No such deny, no such boundary — and parity says so rather than the
	// mount quietly appearing.
	if plain := a.CageMounts(cageAgent(t, a, "cage: container\n"), e, dir); plain[0].RO {
		t.Errorf("a PID with no Edit/Write deny keeps a writable repo: %+v", plain[0])
	}
}

// L3 rides in on the repo mount — except in a worktree, where `.git` is a
// FILE pointing at the main repo's common dir somewhere else on the host.
// A hook the container cannot see is a `git push` this tier lost.
func TestWorktreeGitCommonDirCrossesTheBoundary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	main := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
	}
	run(main, "init", "-b", "main")
	run(main, "config", "user.email", "t@example.com")
	run(main, "config", "user.name", "t")
	os.WriteFile(filepath.Join(main, "f"), []byte("x\n"), 0o644)
	run(main, "add", "f")
	run(main, "commit", "-m", "one")
	wt := filepath.Join(t.TempDir(), "wt")
	run(main, "worktree", "add", wt)

	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	ag := cageAgent(t, a, "cage: container\ndeny: [Bash(git push:*)]\n")
	common, err := hooksDir(wt)
	if err != nil {
		t.Fatal(err)
	}
	common = filepath.Dir(common)
	found := false
	for _, m := range a.CageMounts(ag, e, wt) {
		if m.Src == common && m.Dst == common {
			found = true
		}
	}
	if !found {
		t.Errorf("the worktree's git common dir (%s) must be mounted — .git/hooks/pre-push lives there:\n%+v", common, a.CageMounts(ag, e, wt))
	}
	// An ordinary repo keeps its .git under the repo mount: nothing extra.
	for _, m := range a.CageMounts(ag, e, main) {
		if m.Src == filepath.Join(main, ".git") {
			t.Errorf("an ordinary repo needs no second mount: %+v", m)
		}
	}
	// Neither does a directory that is not a repo at all.
	if got := gitCommonDirOutside(t.TempDir()); got != "" {
		t.Errorf("not a repo → nothing to mount, got %q", got)
	}
}

// The herdr socket is a fleet-wide capability: a persona holding it can
// prompt or close every other pane. Off unless the PID names it — and when
// it does, the launch says so where the operator reads sessions.
func TestSocketsAreOffUnlessThePIDNamesThem(t *testing.T) {
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := t.TempDir()
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(t.TempDir(), "herdr.sock"))
	sock := herdrSocketPath()

	plain := cageAgent(t, a, "cage: container\n")
	for _, m := range a.CageMounts(plain, e, dir) {
		if m.Src == sock {
			t.Errorf("the herdr socket must not be mounted by default: %+v", m)
		}
	}
	if CageSocketTag(plain) != "" || len(CageSocketVars(plain)) != 0 {
		t.Errorf("nothing declared, nothing recorded: %q", CageSocketTag(plain))
	}

	held := cageAgent(t, a, "cage: container\nsockets: [herdr]\n")
	found := false
	for _, m := range a.CageMounts(held, e, dir) {
		if m.Src == sock && m.Dst == sock && !m.RO {
			found = true
		}
	}
	if !found {
		t.Errorf("sockets: [herdr] mounts it, same path, writable (a socket is spoken to): %+v", a.CageMounts(held, e, dir))
	}
	// HOME inside is the image's, so posse's own default resolution would look
	// in /root and find nothing: the launch names it.
	if v := CageSocketVars(held); len(v) != 1 || v[0].Key != "HERDR_SOCKET_PATH" || v[0].Value != sock {
		t.Errorf("the cage must point the tools inside at the socket it mounted: %+v", v)
	}
	if CageSocketTag(held) != "herdr" || CageTag(CageContainer, "herdr") != "🔒container+herdr" {
		t.Errorf("the operator reads it on the session line: %q %q", CageSocketTag(held), CageTag(CageContainer, "herdr"))
	}
	if CageTag(CageShims, "") != "" {
		t.Error("the default tier is every session's, so saying so is noise")
	}
	// A name nothing mounts reads in the PID as a capability that was given.
	if err := CheckSockets(cageAgent(t, a, "sockets: [docker]\n")); err == nil {
		t.Error("an unknown socket name must refuse, not be a silent no-op")
	}
}

// Cumulative-in-realization is a promise about what the tier *renders*, and
// the render happens in the image — so the image is what gets asked. An
// image that cannot run `posse gates wrap` is a container tier with no shims,
// and the ADR's rule is that the strongest cage is never the one that
// silently loses `git push`.
func TestInnerGatesAreClaimedOnlyWhenTheImageCanRender(t *testing.T) {
	a := cageApp(t)
	claude, _ := a.LoadRuntime("claude")
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit, Bash(git push:*)]\n")

	// An engine that spells no `inner:` cannot be asked, and answers yes —
	// `probe:`'s precedent: a launch that fails at the engine is a better
	// error than a refusal this package invented.
	if !a.CageInnerGates() {
		t.Error("an engine with no inner: must be assumed ready")
	}

	// One that can be asked and says no: refused, with the reason.
	os.WriteFile(filepath.Join(a.CagesDir(), "empty-image.yaml"), []byte(
		"command: env {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\ninner: false {image} {cmd}\n"), 0o644)
	os.WriteFile(a.ConfigPath, []byte("default_engine: empty-image\n"), 0o644)
	if a.CageInnerGates() {
		t.Fatal("an image that answers no to the probe is not ready")
	}
	p := a.CheckParity(ag, claude, CageContainer, TierStrong)
	u := strings.Join(p.Unrealized, "\n")
	if !strings.Contains(u, "Bash(git push:*)") || !strings.Contains(u, "posse cage build") {
		t.Errorf("a shell verb must be unrealized here, naming the fix:\n%s", u)
	}
	if !strings.Contains(u, "Edit") {
		t.Errorf("so must the mount boundary's gate:\n%s", u)
	}

	// One that can be asked and says yes: the gates are claimed, and the
	// claim names where the render happened.
	os.WriteFile(filepath.Join(a.CagesDir(), "real-image.yaml"), []byte(
		"command: env {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\ninner: true {image} {cmd}\n"), 0o644)
	os.WriteFile(a.ConfigPath, []byte("default_engine: real-image\n"), 0o644)
	ok := a.CheckParity(ag, claude, CageContainer, TierStrong)
	if !strings.Contains(ok.Realized["Bash(git push:*)"], "rendered inside the cage") {
		t.Errorf("the shim is claimed as what it is: %+v", ok.Realized)
	}
	if len(ok.Unrealized) != 0 {
		t.Errorf("nothing else of this PID is unrealized here: %+v", ok.Unrealized)
	}
}

// The line the pane runs, end to end: the runtime keeps its shell inside
// the container, and the image's own posse goes in front of it.
func TestWrapInCagePutsTheGatesInFrontOfTheRuntime(t *testing.T) {
	a := cageApp(t)
	os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"), []byte(
		"command: env {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\ninner: true {image} {cmd}\n"), 0o644)
	ag := cageAgent(t, a, "cage: container\ndeny: [Bash(git push:*)]\n")
	rt, _ := a.LoadRuntime("claude")
	dir := t.TempDir()
	inner := ag.RenderCommandFor(rt, "claude", TierStrong)
	if _, err := a.WrapInCage(ag, rt, "s1", dir, inner, []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.CageArgvFile("p", "s1"))
	if err != nil {
		t.Fatal(err)
	}
	line := string(b)
	if !strings.Contains(line, `"posse"`) || !strings.Contains(line, `"wrap"`) || !strings.Contains(line, `"p"`) {
		t.Errorf("the inner command must lead with `posse gates wrap <persona> --`:\n%s", line)
	}
	if !strings.Contains(line, `"exec `+strings.Split(inner, " ")[0]) {
		t.Errorf("the runtime keeps its own shell behind the wrap:\n%s", line)
	}
	// The audit trail the inner gates append to has to EXIST before the
	// engine mounts it: a bind mount of a missing file makes a directory,
	// and the shims would then append refusals into a path that eats them.
	if st, err := os.Stat(a.RefusalsLogPath("p")); err != nil || st.IsDir() {
		t.Errorf("refusals.log must be a file the launch created: %v", err)
	}
	// An image that cannot render leaves the runtime's line alone rather
	// than putting a word on it that the container would die on — parity has
	// already had its say about what that costs.
	os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"), []byte(
		"command: env {mounts} {env} -w {workdir} {image} {cmd}\nprobe: true {image}\ninner: false {image} {cmd}\n"), 0o644)
	if _, err := a.WrapInCage(ag, rt, "s2", dir, inner, []string{"CLAUDE_CODE_OAUTH_TOKEN"}, ""); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(a.CageArgvFile("p", "s2"))
	if strings.Contains(string(b), `"wrap"`) {
		t.Errorf("no posse in the image → no posse on the line:\n%s", b)
	}
}

// The `.beads` carve-out (ADR 0002's amendment, rangerhq-3nxk): the boundary
// is the work, not the store. A repo that goes `:ro` gets its bead store
// mounted back read-write over it, because the personas this tier exists for
// — the reviewer, the auditor — are exactly the ones who have to be able to
// report what they found.
func TestBeadsCarveOutIsMountedBackOverTheReadOnlyRepo(t *testing.T) {
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := t.TempDir()
	beads := filepath.Join(dir, ".beads")
	ag := cageAgent(t, a, "cage: container\ndeny: [Write]\n")

	// No store, no mount. A bind whose source does not exist is a mountpoint
	// the engine has to CREATE on a read-only mount, which is the failure
	// rangerhq-6so measured on `.beads/bd.sock`.
	for _, m := range a.CageMounts(ag, e, dir) {
		if m.Src == beads {
			t.Errorf("no .beads directory → nothing to carve out: %+v", m)
		}
	}
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	ms := a.CageMounts(ag, e, dir)
	at := -1
	for i, m := range ms {
		if m.Src == beads {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("a :ro repo must carry its .beads read-write: %+v", ms)
	}
	if ms[at].Dst != beads || ms[at].RO {
		t.Errorf("same path in and out, and WRITABLE — that is the whole carve-out: %+v", ms[at])
	}
	// Later mount wins is the mechanism, so the order is the mechanism too.
	if !ms[0].RO || at == 0 {
		t.Errorf("the carve-out must come after the :ro repo it carves out of: %+v", ms)
	}
	// The engine's own spelling: read-write is the ABSENCE of `:ro`, so this
	// asserts the rendered word and not the struct field twice.
	argv := e.RenderArgv(CageRender{Image: "img", Workdir: dir, Mounts: ms, Inner: []string{"x"}})
	if !argvHas(argv, "-v", beads+":"+beads) || !argvHas(argv, "-v", dir+":"+dir+":ro") {
		t.Errorf("the carve-out is a mount flag, not a claim: %q", argv)
	}
	// A repo nobody made read-only has nothing to carve out of: the store is
	// already writable on the repo mount and a second bind would only be a
	// second thing to explain.
	for _, m := range a.CageMounts(cageAgent(t, a, "cage: container\n"), e, dir) {
		if m.Src == beads {
			t.Errorf("a read-write repo needs no carve-out: %+v", m)
		}
	}
}

// The other half of the carve-out: the mount makes `.beads` writable, this
// makes bd able to use it. A no-db wrapper on the same PATH the inner gates
// own — because the socket does not cross (ENOTSUP, rangerhq-6so) and SQLite
// through a mount spends ~5s a command failing to start a daemon.
func TestInnerBdWrapperRunsNoDbOnTheGatesPath(t *testing.T) {
	real := resolveOutside("bd", "")
	if real == "" {
		t.Skip("needs bd on PATH (in the cage it is the image's)")
	}
	a := &App{StateDir: t.TempDir()}
	env := []string{"PATH=" + PathOutsideGates(""), "RHQ_PERSONA=p"}
	w, err := a.PrepareGatesWrap("p", []string{"Edit", "Write"}, false, []string{"sh", "-c", "x"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if w.Bd != filepath.Join(w.Bin, "bd") {
		t.Fatalf("the wrapper goes in the shim dir, the one PATH entry the inner render owns: %q", w.Bd)
	}
	b, err := os.ReadFile(w.Bd)
	if err != nil {
		t.Fatal(err)
	}
	script := string(b)
	// Resolved and written in, like a shim's REAL: first on PATH under its
	// own name, so a wrapper that searched PATH again would find itself.
	if !strings.Contains(script, "exec "+shQuote(real)+" ") {
		t.Errorf("the wrapper must exec the bd resolved on THIS side (%s):\n%s", real, script)
	}
	// Global flags BEFORE the subcommand: `bd --no-db close x`, not
	// `bd close x --no-db` (measured — the trailing spelling is not a flag
	// the subcommand knows).
	for _, f := range CageBdFlags {
		if !strings.Contains(script, " "+f+" ") {
			t.Errorf("%s must be prepended as a global flag:\n%s", f, script)
		}
	}
	if i, j := strings.Index(script, CageBdFlags[0]), strings.Index(script, `"$@"`); i < 0 || j < 0 || i > j {
		t.Errorf("the persona's own argv comes last:\n%s", script)
	}
	if st, err := os.Stat(w.Bd); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Errorf("the wrapper has to be executable: %v %v", st, err)
	}

	// A gate outranks a convenience: a PID that denies a bd verb keeps its
	// refusal, and the wrapper does not overwrite it.
	g := &App{StateDir: t.TempDir()}
	gw, err := g.PrepareGatesWrap("p", []string{"Bash(bd close:*)"}, false, []string{"sh", "-c", "x"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if gw.Bd != "" {
		t.Errorf("a rendered bd shim owns the name: %q", gw.Bd)
	}
	shim, err := os.ReadFile(filepath.Join(gw.Bin, "bd"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shim), "refused by posse gate") {
		t.Errorf("the deny must still be the thing on that PATH entry:\n%s", shim)
	}
}
