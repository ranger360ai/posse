//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-5632o: claude's shell-snapshots/ and session-env/ are
// run by every claude seat ahead of its Bash commands, and the shared config
// dir grant let one seat write another's. Each seat now gets a private copy
// of both, bound over the shared one (wallclaudesessionstate.go).
//
// Same split as the 6akzw pins: render pins from any host, and a live pin
// that hands the render to the real bwrap and tries the plant.
//
//	go test ./internal/posse -run 'WallClaudeSessionState' -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// cssFixture is weFixture plus another seat's live session state in the
// shared config dir, as a box with two claude seats has it.
func cssFixture(t *testing.T) (f weFixture, peerSnap, peerHook string) {
	t.Helper()
	f = weFixtureIn(t, wallUngrantedDir(t))
	peerSnap = filepath.Join(f.cfg, "shell-snapshots", "snapshot-bash-1-peer.sh")
	peerHook = filepath.Join(f.cfg, "session-env", "peer-session", "sessionstart-hook-0.sh")
	for p, body := range map[string]string{
		peerSnap: "# the peer seat's shell snapshot\n",
		peerHook: "# the peer seat's hook env\n",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f, peerSnap, peerHook
}

// bindIndex is the argv index of `--bind src dst`, or -1.
func bindIndex(argv []string, src, dst string) int {
	for i := 0; i+2 < len(argv); i++ {
		if argv[i] == "--bind" && argv[i+1] == src && argv[i+2] == dst {
			return i
		}
	}
	return -1
}

// The render: a claude seat binds its own dir over each member, AFTER the
// config-dir grant it narrows, and a seat on another runtime binds nothing.
func TestQAWallClaudeSessionStateRendersPrivateBindsOverTheGrant(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f, _, _ := cssFixture(t)
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	cfg := absResolve(f.cfg)
	grant := bindIndex(r.Argv, cfg, cfg)
	if grant < 0 {
		t.Fatalf("the fixture's config dir is not granted, so nothing here is measured: %q", r.Argv)
	}
	own := SeatCacheDir(f.home, f.dir)
	for _, n := range claudeSessionStateDirNames {
		src, dst := filepath.Join(own, claudePrivateSessionStateRel, n), filepath.Join(cfg, n)
		i := bindIndex(r.Argv, src, dst)
		switch {
		case i < 0:
			t.Errorf("no private bind over %s — every claude seat can write what the next one runs ahead of Bash (ranger-base-5632o)\n argv: %q", dst, r.Argv)
		case i < grant:
			t.Errorf("the private bind over %s comes before the config-dir grant at %d, so the grant covers it again", dst, grant)
		}
	}
	other := &Runtime{Name: "codex", StateDirs: []string{"~/.codex"}}
	if err := os.MkdirAll(filepath.Join(f.home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	ro, err := f.a.bwrapRender(f.ag, other, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	// A codex seat has private mounts of its own since ranger-base-6ob20
	// (codexEphemeralDirs), in its own home; none may land in claude's.
	for _, pb := range ro.Carve.Private {
		if underDir(cfg, pb.Dst) {
			t.Errorf("a codex seat gets a private bind into claude's config dir, which nothing granted it: %v", pb)
		}
	}
}

// A link at a member's name refuses the render. Every seat can write the
// parent, so the link may be another seat's, and bwrap would mount where it
// points.
func TestQAWallClaudeSessionStateRefusesALinkAtTheMember(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	grant := filepath.Join(d, "cfg")
	elsewhere := filepath.Join(d, "elsewhere")
	for _, p := range []string{grant, elsewhere} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(grant, "shell-snapshots")
	if err := os.Symlink(elsewhere, dst); err != nil {
		t.Fatal(err)
	}
	carve := SeatbeltCarveOut{Private: []PrivateBind{{Src: filepath.Join(d, "seat"), Dst: dst}}}
	if _, _, err := BwrapArgs(d, []string{grant}, carve); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("a link at the member rendered (err %v), so bwrap would mount where another seat pointed it", err)
	}
	// And an absent member and source become prep, in that order.
	carve.Private[0].Dst = filepath.Join(grant, "session-env")
	_, prep, err := BwrapArgs(d, []string{grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	var made []string
	for _, s := range prep {
		if s.Kind == PrepMkdirAll {
			made = append(made, s.Path)
		}
	}
	if !containsString(made, carve.Private[0].Dst) || !containsString(made, carve.Private[0].Src) {
		t.Errorf("prep %v does not make both the absent member and its source", made)
	}
}

// The live join: the rendered wall, the real bwrap, another seat's snapshot
// and hook env planted at, and the seat's own snapshot written and sourced.
func TestLiveWallClaudeSessionStateRefusesPeerPlants(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	if why := liveBwrapApplyRefusal(); why != "" {
		t.Skip("NOTHING MEASURED: " + why)
	}
	f, peerSnap, peerHook := cssFixture(t)
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
	snaps, envs := filepath.Join(f.cfg, "shell-snapshots"), filepath.Join(f.cfg, "session-env")
	before := map[string][]byte{}
	for _, p := range []string{peerSnap, peerHook} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[p] = b
	}

	// Another seat's state is not even visible. Asked FIRST, because a plant
	// below creates the peer's name in this seat's own private dir.
	for name, script := range map[string]string{
		"peer snapshot listed": "ls " + q(snaps) + " | grep -q peer",
		"peer hook env listed": "test -e " + q(filepath.Dir(peerHook)),
	} {
		if ok, out := try(script); ok {
			t.Errorf("%s: another seat's session state is visible to this one\n script: %s\n out: %s", name, script, out)
		}
	}

	// Each plant, tried whether or not the shell reports an error: an append
	// to a name the seat cannot see CREATES a file in its private dir, so the
	// result that counts is the peer's file on the host, read below.
	for name, script := range map[string]string{
		"peer snapshot append":   "echo PLANT >> " + q(peerSnap),
		"peer snapshot replace":  "echo PLANT > " + q(snaps+"/x.tmp") + " && mv " + q(snaps+"/x.tmp") + " " + q(peerSnap),
		"peer hook env append":   "mkdir -p " + q(filepath.Dir(peerHook)) + " && echo PLANT >> " + q(peerHook),
		"snapshots dir moved":    "mv " + q(snaps) + " " + q(snaps+".old") + " && mkdir " + q(snaps) + " && echo PLANT > " + q(peerSnap),
		"session-env dir moved":  "mv " + q(envs) + " " + q(envs+".old"),
		"config dir moved":       "mv " + q(f.cfg) + " " + q(f.cfg+".old"),
		"snapshots dir via link": "ln -s " + q(snaps) + " " + q(filepath.Join(f.dir, "s")) + " && echo PLANT >> " + q(filepath.Join(f.dir, "s", filepath.Base(peerSnap))),
	} {
		_, out := try(script)
		for p, b := range before {
			if got, err := os.ReadFile(p); err != nil || string(got) != string(b) {
				t.Errorf("%s: the peer's %s changed on the host (%q, %v) — it runs ahead of the peer's next Bash command (ranger-base-5632o)\n script: %s\n out: %s", name, filepath.Base(p), got, err, script, out)
				_ = os.WriteFile(p, b, 0o644)
			}
		}
		if _, err := os.Stat(snaps + ".old"); err == nil {
			t.Errorf("%s: the shared shell-snapshots dir was renamed out from under the mount", name)
		}
	}

	// The seat's own: claude's writes, then a LATER shell sourcing the
	// snapshot, which is what the Bash tool does on every command. Both land
	// in the seat's own dir on the host and never in the shared one.
	//
	// A SLICE, not a map: "own snapshot sourced" reads the file "own snapshot
	// write" creates, and map iteration order is random — as a map this block
	// sourced an absent file in 6/40, 2/20 and 3/27 runs, and reddened arm 1
	// four times (ranger-base-l1d9i). The plant loops above are maps because
	// each of their steps is independent.
	own := filepath.Join(SeatCacheDir(f.home, f.dir), claudePrivateSessionStateRel)
	mine := filepath.Join(snaps, "snapshot-bash-2-mine.sh")
	for _, step := range []struct{ name, script string }{
		{"own snapshot write", "echo 'MARK=sourced' > " + q(mine)},
		{"own snapshot sourced", ". " + q(mine) + " && test \"$MARK\" = sourced"},
		{"own hook env write", "mkdir -p " + q(filepath.Join(envs, "mine")) + " && : > " + q(filepath.Join(envs, "mine", "sessionstart-hook-0.sh"))},
		{"projects still write", ": > " + q(filepath.Join(f.cfg, "projects", "x"))},
	} {
		name, script := step.name, step.script
		if ok, out := try(script); !ok {
			t.Errorf("%s: REFUSED under the rendered wall — claude needs this\n script: %s\n out: %s", name, script, out)
		}
	}
	if _, err := os.Stat(filepath.Join(own, "shell-snapshots", filepath.Base(mine))); err != nil {
		t.Errorf("the seat's own snapshot is not in its private dir on the host: %v", err)
	}
	if _, err := os.Stat(mine); err == nil {
		t.Errorf("the seat's own snapshot landed in the SHARED dir %s", snaps)
	}
}
