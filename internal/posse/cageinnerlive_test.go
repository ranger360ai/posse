package posse

// Live pin for rangerhq-6so — ADR 0002's verification 8 and 10, run against
// the real engine and the real cage image. cageinner_test.go proves what is
// *rendered*; this proves the only thing that matters about it:
//
//	inside the cage, the wall is really there — the shims are the image's,
//	the hook fires on the repo mount, a read-only repo is read-only, and the
//	one carve-out in it (`.beads`) is a bead the persona can still comment on.
//
//	RHQ_LIVE_DOCKER=1 go test ./internal/rhq -run TestLiveInnerGates -v
//
// Needs docker (or an engine answering its CLI) and the cage image built —
// `posse cage build ~/src/posse`. It spends no API turn: the "runtime" is a
// shell script, because every claim here is about the environment the
// runtime is handed and none of them is about the runtime.
//
// Measured 2026-08-22, macOS 26.4.1, Docker 29.0.1, image posse-cage:latest;
// the `.beads` carve-out clause 2026-08-27 on the same host (rangerhq-abvm),
// against an image rebuilt from that tree — the inner render is the IMAGE's
// posse, so a source change to it is not in the cage until `posse cage build`.

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// liveCageRepo is a scratch git repo with a bare remote inside it (so
// `git push` reaches the pre-push hook rather than dying on "no remote"),
// posse's L3 hooks installed, and a probe script the cage runs as its
// "runtime". Everything is inside the one directory, which is the one mount
// the boundary is about.
func liveCageRepo(t *testing.T, probe string) string {
	t.Helper()
	dir := shortTempDir(t)
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir, c.Env = dir, append(os.Environ(), "PATH="+PathOutsideGates(""))
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
	}
	git("init", "-b", "main")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	// The bare remote lives INSIDE .git on purpose: a worktree's repo mount
	// is the worktree, not the main checkout, and the only other thing the
	// cage mounts for it is the git common dir. Putting the remote there is
	// what lets one fixture answer both runs — `git push` has to reach a
	// remote that exists before it will run a pre-push hook at all.
	git("init", "--bare", filepath.Join(dir, ".git", "bare.git"))
	git("remote", "add", "origin", filepath.Join(dir, ".git", "bare.git"))
	os.WriteFile(filepath.Join(dir, "f"), []byte("x\n"), 0o644)
	git("add", "f")
	git("commit", "-m", "one")
	if _, err := InstallPrePushHook(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.sh"), []byte(probe), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// shortTempDir is t.TempDir() with a name a unix socket fits in: macOS caps
// sun_path at 104 bytes and the go test temp dir alone spends most of it.
// The sockets here are the point of two of the assertions, so the path has
// to be short before anything else can be true.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "posse6so")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	// Resolved, because /tmp is a symlink to /private/tmp on macOS and git
	// writes the RESOLVED path into a worktree's .git pointer and into a
	// local remote's url. Mounts are same-path in and out, so a fixture that
	// mixed the two spellings would be testing the symlink, not the cage.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		return real
	}
	return dir
}

func TestLiveInnerGatesHoldInsideTheCage(t *testing.T) {
	if os.Getenv("RHQ_LIVE_DOCKER") == "" {
		t.Skip("set RHQ_LIVE_DOCKER=1 (needs docker and `posse cage build`)")
	}
	home := t.TempDir()
	a := &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		EnvsDir: filepath.Join(home, "envs"), StateDir: filepath.Join(home, "state"),
		AgentsDir: filepath.Join(home, "agents"),
	}
	// The built-in docker template runs `-i -t` because the runtimes are
	// TUIs and the pane is one. `go test` is not a terminal, so this run
	// takes the same engine with that one flag dropped — derived from the
	// built-in rather than retyped, so a change to it cannot leave this test
	// verifying an engine line nobody launches.
	d := builtinEngines[0]
	os.MkdirAll(a.CagesDir(), 0o755)
	if err := os.WriteFile(filepath.Join(a.CagesDir(), "notty.yaml"), []byte(strings.Join([]string{
		"command: " + strings.Replace(d.Command, " -i -t", "", 1),
		"mount: " + d.Mount, "mount_ro: " + d.MountRO, "env: " + d.Env, "env_set: " + d.EnvSet,
		"home: " + d.Home, "probe: " + d.Probe, "inner: " + d.Inner,
		"net: " + d.Net, "net_create: " + d.NetCreate, "net_join: " + d.NetJoin,
		"net_remove: " + d.NetRemove, "proxy_up: " + d.ProxyUp, "proxy_down: " + d.ProxyDown, "",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_engine: notty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		t.Fatal(err)
	}
	if !a.ContainerAvailable() {
		t.Skipf("engine %s is not on this host", e.Name)
	}
	image := a.CageImage()
	if !a.CageImageBuilt(e, image) {
		t.Skipf("%s is not built — run `posse cage build`", image)
	}
	// …and built from THIS source, which is the third state of a pin whose
	// subject is a build artifact (ranger-base-nwj7). The inner render is
	// the image's own posse, so an image behind this tree renders a wall
	// this tree does not describe: every clause below would then be
	// measuring a different build's render, and the first one to notice
	// would say so as a FAIL. Skipped with both idents instead — see
	// cagestale.go for why "cannot tell" is not this arm.
	requireCurrentCageImage(t, a, e, image)
	// The claim parity rests on, asked of the image itself.
	if !a.CageInnerGatesReady(e, image) {
		t.Fatalf("%s answers no to `posse gates wrap %s` — it carries no Linux posse, so this tier renders no gates", image, GatesWrapProbe)
	}

	// Two socket questions, because the answer differs by HOW the socket
	// crosses and that difference decides what this tier can promise:
	// one reached through a bind-mounted *directory* (`.beads/bd.sock` on
	// the repo mount) and one bind-mounted as a *file* (`sockets: [herdr]`).
	probe := `
conn() { node -e 'require("net").connect(process.argv[1]).on("connect",function(){console.log("connected");process.exit(0)}).on("error",function(e){console.log(e.code);process.exit(0)})' "$1"; }
echo "shell=$SHELL"
echo "gates=$RHQ_GATES_DIR"
echo "which-git=$(command -v git)"
echo "shim: $(git push origin main 2>&1 | head -1)"
echo "hook: $(/usr/bin/git push origin main 2>&1 | grep -i refused | head -1)"
touch ./written 2>/dev/null && echo "touch=wrote" || echo "touch=refused"
echo "which-bd=$(command -v bd)"
touch ./.beads/written 2>/dev/null && echo "beadswrite=wrote" || echo "beadswrite=refused"
if [ -n "$RHQ_BD_PROBE_ID" ]; then
  echo "bdcomment=$(bd comments add "$RHQ_BD_PROBE_ID" 'from inside the cage' 2>&1 | tail -1)"
fi
echo "dirsock=$(conn ./probe.sock)"
if [ -S "$RHQ_HERDR_SOCK_PROBE" ]; then echo "herdr=$(conn "$RHQ_HERDR_SOCK_PROBE")"; else echo "herdr=absent"; fi
`
	dir := liveCageRepo(t, probe)
	bead := liveCageBeadStore(t, dir)
	ln, err := net.Listen("unix", filepath.Join(dir, "probe.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	// The herdr socket the PID may ask for: a real one, so the mount has
	// something to mount (a bind mount of a missing file makes a directory).
	sockDir := shortTempDir(t)
	hsock := filepath.Join(sockDir, "herdr.sock")
	hl, err := net.Listen("unix", hsock)
	if err != nil {
		t.Fatal(err)
	}
	defer hl.Close()
	t.Setenv("HERDR_SOCKET_PATH", hsock)

	rt, _ := a.LoadRuntime("claude")
	deny := []string{"Bash(git push:*)", "Edit", "Write"}

	run := func(t *testing.T, dir, front, session string) map[string]string {
		t.Helper()
		ag := cageAgent(t, a, front)
		if err := ag.EnsureMemoryDir(); err != nil {
			t.Fatal(err)
		}
		line, err := a.WrapInCage(ag, rt, session, dir, "sh ./probe.sh",
			append(CageEnvNames(nil), "CLAUDE_CODE_OAUTH_TOKEN", "RHQ_HERDR_SOCK_PROBE", "RHQ_BD_PROBE_ID"), "")
		if err != nil {
			t.Fatal(err)
		}
		_ = line
		b, err := os.ReadFile(a.CageArgvFile(ag.Name, session))
		if err != nil {
			t.Fatal(err)
		}
		var plan CageLaunchPlan
		if err := json.Unmarshal(b, &plan); err != nil {
			t.Fatal(err)
		}
		if plan.Egress != nil {
			t.Cleanup(func() { runCageSteps(plan.Egress.Down, os.Stderr) })
			runCageSteps(plan.Egress.Down, os.Stderr) // a leftover from a previous run
			if err := runCageSteps(plan.Egress.Up, os.Stderr); err != nil {
				t.Fatal(err)
			}
		}
		c := exec.Command(plan.Path)
		c.Args = plan.Argv
		c.Env = append(os.Environ(),
			"RHQ_TOOLS_DENY="+strings.Join(deny, "\n"),
			"RHQ_PERSONA="+ag.Name,
			"RHQ_HERDR_SOCK_PROBE="+hsock,
			// The carve-out's two halves need one name each: the bead to
			// comment on, and the actor the comment is attributed to — which
			// is the same BD_ACTOR the host forwards into a real caged
			// session (CageEnvNames), not something this test invented.
			"RHQ_BD_PROBE_ID="+bead,
			"BD_ACTOR=caged-"+ag.Name,
		)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("the cage did not run: %v\n%s", err, out)
		}
		got := map[string]string{}
		for _, ln := range strings.Split(string(out), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(ln), "="); ok {
				got[k] = v
			} else if k, v, ok := strings.Cut(strings.TrimSpace(ln), ": "); ok {
				got[k] = v
			}
		}
		t.Logf("cage said:\n%s", out)
		return got
	}

	// ─── verification 8 ──────────────────────────────────────────────────
	got := run(t, dir, "cage: container\ndeny: ["+strings.Join(deny, ", ")+"]\n", "live")
	inner := CageGatesDir("p")
	if got["which-git"] != filepath.Join(inner, "bin", "git") {
		t.Errorf("`command -v git` inside must answer the INNER shim (%s/bin/git), got %q", inner, got["which-git"])
	}
	if got["gates"] != inner {
		t.Errorf("RHQ_GATES_DIR must name the inner render, got %q", got["gates"])
	}
	if !strings.HasPrefix(got["shell"], inner+"/shell/") {
		t.Errorf("SHELL must point at the gate shell rendered inside, got %q", got["shell"])
	}
	if !strings.Contains(got["shim"], "refused by posse gate") {
		t.Errorf("the L1 shim must refuse `git push` inside the cage, got %q", got["shim"])
	}
	if !strings.Contains(got["hook"], "pre-push hook") {
		t.Errorf("/usr/bin/git push must meet the L3 hook on the repo mount, got %q", got["hook"])
	}
	if got["touch"] != "refused" {
		t.Errorf("a PID denying Edit/Write gets a READ-ONLY repo: touch said %q", got["touch"])
	}
	// The measurement that cost ADR 0002's verification-8 line its bd clause
	// (rangerhq-6so, macOS 26.4.1 / Docker 29.0.1): a unix socket reached
	// through a bind-mounted DIRECTORY is not connectable on VirtioFS —
	// ENOTSUP, and read-only has nothing to do with it (it fails the same
	// way on a read-write mount). So `.beads/bd.sock` is never the route
	// into the cage, and with the repo :ro bd cannot open its SQLite either.
	// Pinned rather than wished away: the day an engine makes this work is a
	// day this tier gets a capability back, and it should be noticed.
	if got["dirsock"] == "connected" {
		t.Errorf("a socket through a directory mount now connects (%q) — re-open the bd question this tier was scoped around and fix the NOTES", got["dirsock"])
	}
	// The `.beads` carve-out, which is the rest of verification 8 (ADR 0002's
	// amendment via rangerhq-3nxk): the repo is read-only and the bead store
	// inside it is NOT, and the bd on the inner PATH is one that can use it.
	// The three claims are separate on purpose — a writable mount with a bd
	// that cannot open it is not a capability, and neither is the reverse.
	if bead != "" {
		if got["beadswrite"] != "wrote" {
			t.Errorf("the .beads carve-out must be writable over the :ro repo, got %q", got["beadswrite"])
		}
		if want := filepath.Join(inner, "bin", "bd"); got["which-bd"] != want {
			t.Errorf("`command -v bd` must answer the inner no-db wrapper (%s), got %q — an image older than that render answers with its own bd, so rebuild it (`posse cage build`) before reading this as a regression", want, got["which-bd"])
		}
		if !strings.Contains(got["bdcomment"], "Comment added") {
			t.Errorf("a caged persona denying Edit/Write must still be able to comment on its bead, got %q", got["bdcomment"])
		}
		// And the write is on the ordinary host path, with nothing imported
		// by hand: the caged bd appended to issues.jsonl and the repo's
		// running daemon imports it before answering. "Running" is load-
		// bearing — see liveCageBeadStore, where stopping it first turns
		// this same read into a staleness refusal.
		c := exec.Command("bd", "show", bead)
		c.Dir, c.Env = dir, append(os.Environ(), "PATH="+PathOutsideGates(""))
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("host bd show %s: %v\n%s", bead, err, b)
		}
		if !strings.Contains(string(b), "from inside the cage") {
			t.Errorf("the caged comment must be visible to an ordinary host `bd show`, with no manual import:\n%s", b)
		}
	}
	// Both refusals land in the host's audit trail — never by a mount any
	// more (ADR 0025 §4), but by a host-side FOLD of this session's spool.
	if err := a.FoldRefusalsSpool("p", "live"); err != nil {
		t.Fatalf("fold: %v", err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatalf("refusals.log must survive the container: %v", err)
	}
	log := string(b)
	if !strings.Contains(log, "git push") || !strings.Contains(log, "[pre-push hook]") {
		t.Errorf("both layers must append to gates/p/refusals.log on the host, via the fold:\n%s", log)
	}
	if !strings.Contains(log, "session=live") {
		t.Errorf("a folded line must name the session it came from:\n%s", log)
	}

	// A WORKTREE, where `.git` is a file pointing at the main repo's common
	// dir somewhere else on the host. Nothing about L1 changes; L3 is the
	// question, because the hook it must run lives outside the repo mount.
	wt := filepath.Join(shortTempDir(t), "wt")
	c := exec.Command("git", "worktree", "add", wt)
	c.Dir, c.Env = dir, append(os.Environ(), "PATH="+PathOutsideGates(""))
	if b, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, b)
	}
	os.WriteFile(filepath.Join(wt, "probe.sh"), []byte(probe), 0o755)
	if _, err := InstallPrePushHook(wt); err != nil {
		t.Fatal(err)
	}
	w := run(t, wt, "cage: container\ndeny: ["+strings.Join(deny, ", ")+"]\n", "live-wt")
	if !strings.Contains(w["shim"], "refused by posse gate") {
		t.Errorf("the L1 shim must refuse in a worktree too, got %q", w["shim"])
	}
	if !strings.Contains(w["hook"], "pre-push hook") {
		t.Errorf("L3 must reach the common dir mounted alongside a worktree, got %q — .git there is a file pointing outside the repo mount", w["hook"])
	}

	// ─── verification 10 ─────────────────────────────────────────────────
	if got["herdr"] != "absent" {
		t.Errorf("the herdr socket is a fleet-wide capability: absent unless the PID names it, got %q", got["herdr"])
	}
	held := run(t, dir, "cage: container\nsockets: [herdr]\ndeny: ["+strings.Join(deny, ", ")+"]\n", "live-sock")
	// A socket bind-mounted as a FILE is the one shape that does cross — the
	// spike measured it for herdr (rangerhq-89a) and it is what makes this
	// opt-in a real capability rather than a mounted decoration.
	if held["herdr"] != "connected" {
		t.Errorf("sockets: [herdr] must mount a socket that can be SPOKEN to, got %q", held["herdr"])
	}
}

// liveCageBeadStore puts a real bd store in the fixture and returns the id of
// one issue in it — the thing the `.beads` carve-out exists for. "" when the
// host has no bd, which costs that clause and nothing else: the image carries
// its own, and the host's is needed only to build the fixture and to read the
// result back on the ordinary path.
func liveCageBeadStore(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("bd"); err != nil {
		t.Log("no bd on the host: skipping verification 8's .beads carve-out clause")
		return ""
	}
	bd := func(args ...string) string {
		t.Helper()
		c := exec.Command("bd", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "PATH="+PathOutsideGates(""), "BD_ACTOR=fixture")
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("bd %v: %v\n%s", args, err, b)
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		return strings.TrimSpace(lines[len(lines)-1])
	}
	// --skip-hooks: posse's L3 hooks are already installed in this fixture
	// and bd's own are not what is being verified here.
	bd("init", "-q", "-p", "cage", "--skip-hooks", "--skip-merge-driver")
	id := bd("q", "the bead a caged persona has to be able to comment on")
	bd("sync") // db → issues.jsonl, which is what the no-db bd inside reads

	// The daemon those commands started is LEFT RUNNING, and that is part of
	// the claim rather than laziness. "Visible with no manual import" is a
	// property of the daemon: it imports a newer JSONL before answering. Stop
	// it first and the same read fails instead — measured 2026-08-27, the
	// cold store's `bd show` waits 5s for a daemon it cannot start, drops to
	// direct mode and refuses with *Database out of sync with JSONL. Run 'bd
	// sync --import-only' to fix.* A caged persona writes into a repo whose
	// daemon is up, which is what this fixture reproduces.
	//
	// Cleanup stops it, because the fixture dir is about to be removed and a
	// daemon holding a deleted repo is the stale kind `make verify-bd-pin`
	// goes looking for. The workspace is an ARGUMENT, not the cwd: `bd daemon
	// stop` on 0.49.1 exits 1 with "accepts 1 arg(s), received 0" whatever
	// directory it is run from (measured 2026-08-27), and a cleanup whose
	// error is ignored is a cleanup that silently leaves the daemon behind.
	t.Cleanup(func() {
		c := exec.Command("bd", "daemon", "stop", dir)
		c.Dir, c.Env = dir, append(os.Environ(), "PATH="+PathOutsideGates(""))
		if b, err := c.CombinedOutput(); err != nil {
			t.Logf("bd daemon stop %s: %v\n%s", dir, err, b)
		}
		// …and the backstop, because that command exits 0 and leaves the
		// process running anyway (measured 2026-08-27, bd 0.49.1). The pid
		// file is the one this fixture's own daemon wrote seconds ago in a
		// directory nothing else has ever seen, so signalling it is not a
		// guess about somebody else's process.
		b, err := os.ReadFile(filepath.Join(dir, ".beads", "daemon.pid"))
		if err != nil {
			return
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	})
	return id
}
