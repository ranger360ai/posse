//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-ajkqx: claude's projects/ is read-only inside a caged
// claude seat but for the seat's own key dir, and memory/ is read-only at
// every key, the own one included (wallclaudeprojects.go).
//
// Same split as the 5632o pins: render pins from any host, and a live pin
// that hands the render to the real bwrap and tries the plants, then runs the
// real claude under it to show its own transcript still lands.
//
//	go test ./internal/posse -run 'WallClaudeProjects' -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cpFixture is weFixture with its cwd turned into a session worktree this
// App would have made (<WorktreeRoot>/<repo>/<session>, a linked worktree),
// plus a peer key's transcript and memory in the shared projects/, the way
// the operator's own sessions leave them.
type cpFixture struct {
	weFixture
	main             string // the repo's main checkout: a cwd that is NOT the seat's own
	peerMem, peerLog string
}

func cpFixtureIn(t *testing.T, root string) cpFixture {
	t.Helper()
	f := cpFixture{weFixture: weFixtureIn(t, root)}
	f.main = filepath.Join(root, "home", "src", "repo")
	wtRoot := filepath.Join(f.home, ".posse", "worktrees")
	f.a.WorktreeRootDefault = wtRoot
	tree := filepath.Join(wtRoot, "repo", "dinesh-repo-x1")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", f.main},
		{"-C", f.main, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "root"},
		{"-C", f.main, "worktree", "add", "-q", "-b", "posse/dinesh-repo-x1", tree},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	f.dir = tree
	peer := filepath.Join(f.cfg, "projects", claudeProjectDir(absResolve(f.home)))
	f.peerMem = filepath.Join(peer, "memory", "MEMORY.md")
	f.peerLog = filepath.Join(peer, "11111111-2222-3333-4444-555555555555.jsonl")
	for p, body := range map[string]string{
		f.peerMem: "# the operator's auto-memory\n",
		f.peerLog: `{"type":"user","cwd":"` + f.home + `"}` + "\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// argvIndex is the index of `flag src dst` in argv, or -1.
func argvIndex(argv []string, flag, src, dst string) int {
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == src && argv[i+2] == dst {
			return i
		}
	}
	return -1
}

// The render: projects/ read-only after the config-dir grant, the own key
// read-write after that, its memory/ read-only after that. A seat whose cwd is
// not its own session worktree gets the first and nothing that reopens any of
// it, and a seat on another runtime gets none of them.
func TestQAWallClaudeProjectsRendersOwnKeyInsideAReadOnlyProjects(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := cpFixtureIn(t, t.TempDir())
	cfg := absResolve(f.cfg)
	projects := filepath.Join(cfg, "projects")
	own := filepath.Join(projects, claudeProjectDir(absResolve(f.dir)))
	mem := filepath.Join(own, "memory")

	if got := f.a.claudeOwnSessionTree(f.dir); got != absResolve(f.dir) {
		t.Fatalf("the fixture's session worktree is not recognised as the seat's own (%q), so nothing below is measured", got)
	}
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	grant := bindIndex(r.Argv, cfg, cfg)
	if grant < 0 {
		t.Fatalf("the fixture's config dir is not granted, so nothing here is measured: %q", r.Argv)
	}
	ro := argvIndex(r.Argv, "--ro-bind", projects, projects)
	rw := argvIndex(r.Argv, "--bind", own, own)
	mro := argvIndex(r.Argv, "--ro-bind", mem, mem)
	switch {
	case ro < 0 || rw < 0 || mro < 0:
		t.Errorf("missing a projects bind (ranger-base-ajkqx): projects ro at %d, own rw at %d, own memory ro at %d\n argv: %q", ro, rw, mro, r.Argv)
	case !(grant < ro && ro < rw && rw < mro):
		t.Errorf("projects binds out of order: grant %d, projects ro %d, own rw %d, own memory ro %d — each must narrow the one before", grant, ro, rw, mro)
	}

	for name, cwd := range map[string]string{
		"the main checkout": f.main,
		"a plain dir":       filepath.Join(f.home, "plain"),
		"under the tree":    filepath.Join(f.dir, "sub"),
	} {
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := f.a.claudeOwnSessionTree(cwd); got != "" {
			t.Errorf("%s (%s) is read as the seat's own session worktree (%q)", name, cwd, got)
		}
		nr, err := f.a.bwrapRender(f.ag, f.rt, cwd, f.gates)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if argvIndex(nr.Argv, "--ro-bind", projects, projects) < 0 {
			t.Errorf("%s: projects/ is not read-only (ranger-base-ajkqx D3)\n argv: %q", name, nr.Argv)
		}
		for i := 0; i+2 < len(nr.Argv); i++ {
			if nr.Argv[i] == "--bind" && underDir(projects, nr.Argv[i+2]) {
				t.Errorf("%s: a write bind into projects/ at %d (%s), for a seat that has no own key", name, i, nr.Argv[i+2])
			}
		}
	}

	other := &Runtime{Name: "codex", StateDirs: []string{"~/.codex"}}
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cr, err := f.a.bwrapRender(f.ag, other, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	for _, pb := range cr.Carve.Private {
		if underDir(cfg, pb.Dst) {
			t.Errorf("a codex seat gets a bind into claude's config dir, which nothing granted it: %v", pb)
		}
	}
}

// A link at the own key dir or at memory/ refuses the render: every claude
// can write projects/, so the link may be another seat's, and bwrap would
// mount where it points. Absent dirs become prep, made once.
func TestQAWallClaudeProjectsRefusesALinkAtABindPoint(t *testing.T) {
	// No t.Parallel: sets CLAUDE_CONFIG_DIR.
	d := t.TempDir()
	cfg := filepath.Join(d, "cfg")
	elsewhere := filepath.Join(d, "elsewhere")
	for _, p := range []string{cfg, elsewhere} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(ClaudeConfigDirVar, cfg)
	binds := claudeProjectsBinds(d, []string{cfg}, filepath.Join(d, "tree"))
	if len(binds) != 3 {
		t.Fatalf("binds = %v, want projects, own, memory", binds)
	}
	_, prep, err := BwrapArgs(d, []string{cfg}, SeatbeltCarveOut{Private: binds})
	if err != nil {
		t.Fatal(err)
	}
	var made []string
	for _, s := range prep {
		if s.Kind == PrepMkdirAll {
			made = append(made, s.Path)
		}
	}
	for _, pb := range binds {
		n := 0
		for _, m := range made {
			if m == pb.Dst {
				n++
			}
		}
		if n != 1 {
			t.Errorf("prep makes %s %d times, want once: %v", pb.Dst, n, made)
		}
	}
	for _, link := range []string{binds[1].Dst, binds[2].Dst} {
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, link); err != nil {
			t.Fatal(err)
		}
		if _, _, err := BwrapArgs(d, []string{cfg}, SeatbeltCarveOut{Private: binds}); err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("a link at %s rendered (err %v), so bwrap would mount where another seat pointed it", link, err)
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(link, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// The live join: the rendered wall and the real bwrap. Another key's memory
// and transcript refuse every plant and stay byte-identical on the host; the
// seat's own memory refuses too; the seat's own transcript is written, by a
// shell and by the real claude.
func TestLiveWallClaudeProjectsRefusesPeerPlants(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	wrcNeedBwrap(t)
	f := cpFixtureIn(t, wallUngrantedDir(t))
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	try := func(script string) (bool, string) {
		cmd := exec.Command("bwrap", append(append([]string{}, r.Argv...), "/bin/sh", "-c", script)...)
		out, err := cmd.CombinedOutput()
		return err == nil, strings.TrimSpace(string(out))
	}
	q := shellQuote
	projects := filepath.Join(f.cfg, "projects")
	own := filepath.Join(projects, claudeProjectDir(absResolve(f.dir)))
	ownMem := filepath.Join(own, "memory", "MEMORY.md")
	newKey := filepath.Join(projects, "-planted-key")
	peerDir := filepath.Dir(f.peerLog)
	before := map[string][]byte{}
	for _, p := range []string{f.peerMem, f.peerLog} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[p] = b
	}

	// Every plant is a write the seat must be refused. The result that counts
	// is the host, read after each one; the shell's own exit is reported too,
	// because a plant that "succeeded" into nothing still says the wall leaks.
	for _, step := range []struct{ name, script string }{
		{"peer memory append", "echo PLANT >> " + q(f.peerMem)},
		{"peer memory replace", "echo PLANT > " + q(f.peerMem+".x") + " && mv " + q(f.peerMem+".x") + " " + q(f.peerMem)},
		{"peer memory dir moved", "mv " + q(filepath.Dir(f.peerMem)) + " " + q(filepath.Dir(f.peerMem)+".old")},
		{"peer transcript append", "echo PLANT >> " + q(f.peerLog)},
		{"peer transcript replace", "echo PLANT > " + q(f.peerLog+".x") + " && mv " + q(f.peerLog+".x") + " " + q(f.peerLog)},
		{"peer transcript new", "echo PLANT > " + q(filepath.Join(peerDir, "99999999-0000-0000-0000-000000000000.jsonl"))},
		{"new key memory", "mkdir -p " + q(filepath.Join(newKey, "memory")) + " && echo PLANT > " + q(filepath.Join(newKey, "memory", "MEMORY.md"))},
		{"own memory write", "echo PLANT > " + q(ownMem)},
		{"own memory dir moved", "mv " + q(filepath.Dir(ownMem)) + " " + q(filepath.Dir(ownMem)+".old")},
		{"own key dir moved", "mv " + q(own) + " " + q(own+".old")},
		{"projects moved", "mv " + q(projects) + " " + q(projects+".old")},
		{"via a link in the tree", "ln -s " + q(peerDir) + " " + q(filepath.Join(f.dir, "p")) + " && echo PLANT >> " + q(filepath.Join(f.dir, "p", filepath.Base(f.peerLog)))},
	} {
		ok, out := try(step.script)
		if ok {
			t.Errorf("%s: the write SUCCEEDED under the rendered wall (ranger-base-ajkqx)\n script: %s\n out: %s", step.name, step.script, out)
		}
		for p, b := range before {
			if got, err := os.ReadFile(p); err != nil || string(got) != string(b) {
				t.Errorf("%s: %s changed on the host (%q, %v)", step.name, p, got, err)
				_ = os.MkdirAll(filepath.Dir(p), 0o755)
				_ = os.WriteFile(p, b, 0o644)
			}
		}
		for _, p := range []string{newKey, ownMem, filepath.Join(peerDir, "99999999-0000-0000-0000-000000000000.jsonl"),
			filepath.Dir(f.peerMem) + ".old", filepath.Dir(ownMem) + ".old", own + ".old", projects + ".old"} {
			if _, err := os.Lstat(p); err == nil {
				t.Errorf("%s: %s exists on the host", step.name, p)
			}
		}
	}

	// The seat's own transcript: written in place, so posse's readers and a
	// --resume find it at claude's own path.
	mine := filepath.Join(own, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee.jsonl")
	for _, step := range []struct{ name, script string }{
		{"own transcript write", "echo '{}' > " + q(mine)},
		{"own transcript append", "echo '{}' >> " + q(mine)},
		{"own session dir", "mkdir -p " + q(filepath.Join(own, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "subagents"))},
	} {
		if ok, out := try(step.script); !ok {
			t.Errorf("%s: REFUSED under the rendered wall — claude writes this\n script: %s\n out: %s", step.name, step.script, out)
		}
	}
	if b, err := os.ReadFile(mine); err != nil || string(b) != "{}\n{}\n" {
		t.Errorf("the seat's own transcript on the host: %q, %v", b, err)
	}

	// The real claude, under the same argv. A dead endpoint: nothing leaves
	// the box and no turn is billed. Its transcript is written before the
	// first request (transcriptceiling_live_test.go has that measurement),
	// so it must appear in the own key dir and nowhere else.
	bin := wrcRealBinary("claude")
	if bin == "" {
		t.Log("NOT MEASURED: no claude binary, so the real CLI's own transcript arm did not run")
		return
	}
	beforeOwn, _ := filepath.Glob(filepath.Join(own, "*.jsonl"))
	cmd := exec.Command("bwrap", append(append([]string{}, r.Argv...), bin, "-p", "say hi")...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(), "HOME="+f.home, ClaudeConfigDirVar+"="+f.cfg,
		"ANTHROPIC_BASE_URL=http://127.0.0.1:1", "ANTHROPIC_API_KEY=sk-dead-ajkqx",
		"DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "CLAUDE_CODE_MAX_RETRIES=0")
	cmd.Stdin = nil
	done := make(chan struct{})
	var out []byte
	go func() { out, _ = cmd.CombinedOutput(); close(done) }()
	select {
	case <-done:
	case <-time.After(90 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("claude -p under the wall did not end in 90s:\n%s", out)
	}
	t.Logf("claude -p under the rendered wall (%s): %s", bin, strings.TrimSpace(string(out)))
	afterOwn, _ := filepath.Glob(filepath.Join(own, "*.jsonl"))
	if len(afterOwn) <= len(beforeOwn) {
		t.Errorf("the real claude wrote no transcript in its own key dir %s under the rendered wall\n out: %s", own, out)
	}
	for p, b := range before {
		if got, err := os.ReadFile(p); err != nil || string(got) != string(b) {
			t.Errorf("the real claude's run changed %s on the host (%q, %v)", p, got, err)
		}
	}
}
