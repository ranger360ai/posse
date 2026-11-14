//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-6b46n — the landed-check cage reads none of the host's
// secrets.
//
// boxLandedCage rendered its wall with an empty carve-out, and under
// `--ro-bind / /` an empty DenyRead is the host's whole read surface: the
// judge — landed code, which ADR 0088 D3 cages because landing reviews
// nothing — read every ssh key the box pushes with and every runtime's
// credential file. These pin the render (every host with a mechanism) and
// the read itself (Linux, bwrap), each with a control that reads, so a green
// is never a probe that could not.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type landedCageFixture struct {
	root, tree, run string
	secrets         []string // every file the judge must not read
	ssh             sshFixture
}

// newLandedCageFixture plants an ssh config naming keys, claude's credential
// in its home spelling and codex's, all outside every grant, and points the
// account's HOME at them. Not parallel-safe: it sets HOME.
func newLandedCageFixture(t *testing.T) landedCageFixture {
	t.Helper()
	root := wallUngrantedDir(t)
	f := landedCageFixture{root: root, tree: filepath.Join(root, "tree"), run: filepath.Join(root, "run")}
	for _, d := range []string{f.tree, f.run} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.ssh = newSSHFixture(t, root)
	home := f.ssh.home
	t.Setenv("HOME", home)
	t.Setenv("SSHFIX", home)
	t.Setenv(ClaudeConfigDirVar, filepath.Join(home, ".claude"))
	t.Setenv(CodexHomeVar, "")
	t.Setenv(GrokHomeVar, "")
	claude := filepath.Join(home, ".claude", ".credentials.json")
	codex := filepath.Join(home, ".codex", "auth.json")
	for _, p := range []string{claude, codex} {
		writeFile(t, p, "PRIVATE "+filepath.Base(p)+"\n", 0o600)
	}
	f.secrets = append(append([]string{}, f.ssh.keys...), claude, codex)
	return f
}

func TestQALandedCheckCageRendersTheSecretMasks(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := newLandedCageFixture(t)
	judge := filepath.Join(f.run, "judge")
	// The rex tsnet node's state (ranger-base-k06gw): a directory, hidden whole.
	t.Setenv("XDG_STATE_HOME", "")
	tsnet := absResolve(filepath.Join(f.ssh.home, ".local", "state", "rex", "tsnet"))
	writeFile(t, filepath.Join(tsnet, "node", "tailscaled.state"), "PRIVATE\n", 0o600)
	cage, err := boxLandedCage(&App{}, judge, f.tree, []string{f.run}, nil)
	if err != nil {
		t.Skip("NOTHING MEASURED: this host has no landed-check cage to render: " + err.Error())
	}
	switch WallMechanism() {
	case WallBwrap:
		line := bwJoin(cage.Argv)
		if bwIndex(cage.Argv, "--tmpfs", tsnet) < 0 {
			t.Errorf("the landed-check cage mounts no tmpfs over %s, so the judge reads the rex node credential (ranger-base-k06gw):\n  %s", tsnet, line)
		}
		// A key under ~/.ssh is covered by the tmpfs over the directory
		// (ranger-base-12ggv); everything else by its own /dev/null bind.
		sshDir := absResolve(filepath.Join(f.ssh.home, ".ssh"))
		hidden := bwIndex(cage.Argv, "--tmpfs", sshDir) >= 0
		if !hidden {
			t.Errorf("the landed-check cage mounts no tmpfs over %s, so a key no config names is readable to the judge (ranger-base-12ggv):\n  %s", sshDir, line)
		}
		for _, s := range f.secrets {
			if hidden && underDir(sshDir, absResolve(s)) {
				continue
			}
			if bwIndex(cage.Argv, "--ro-bind", "/dev/null", absResolve(s)) < 0 {
				t.Errorf("the landed-check cage does not mask %s, so the judge — landed code — reads it (ranger-base-6b46n):\n  %s", s, line)
			}
		}
	case WallSandboxExec:
		b, err := os.ReadFile(filepath.Join(f.run, "landed-check.sb"))
		if err != nil {
			t.Fatal(err)
		}
		prof := string(b)
		i := strings.Index(prof, "(deny file-read*")
		if i < 0 {
			t.Fatalf("the landed-check profile has no file-read deny block at all (ranger-base-6b46n):\n%s", prof)
		}
		for _, s := range f.secrets {
			if !strings.Contains(prof[i:], `"`+absResolve(s)+`"`) {
				t.Errorf("the landed-check profile does not deny reading %s (ranger-base-6b46n)", s)
			}
		}
		if !strings.Contains(prof[i:], "(subpath "+sbQuote(tsnet)+")") {
			t.Errorf("the landed-check profile does not deny reading under %s (ranger-base-k06gw)", tsnet)
		}
	default:
		t.Fatalf("boxLandedCage rendered a cage on mechanism %q, which this pin does not know how to read", WallMechanism())
	}
}

// Live: the judge, run in the cage boxLandedCage builds, reads no byte of
// any planted secret; the same judge in a cage with an empty carve — the
// shape before this fix — reads them all.
func TestLiveLandedCheckCageReadsNoSecret(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if WallMechanism() != WallBwrap {
		t.Skip("NOTHING MEASURED: this host's wall mechanism is " + WallMechanism())
	}
	f := newLandedCageFixture(t)
	judge := filepath.Join(f.run, "judge")
	var body strings.Builder
	body.WriteString("#!/bin/sh\n")
	for _, s := range f.secrets {
		body.WriteString("printf '%s=' " + shQuote(s) + "; head -c 64 -- " + shQuote(s) + " 2>/dev/null | tr -d '\\n'; echo\n")
	}
	if err := WriteExecutable(judge, []byte(body.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	read := func(argv []string) map[string]string {
		t.Helper()
		out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", bwJoin(argv), err, out)
		}
		res := map[string]string{}
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if k, v, ok := strings.Cut(l, "="); ok {
				res[k] = v
			}
		}
		return res
	}

	cage, err := boxLandedCage(&App{}, judge, f.tree, []string{f.run}, nil)
	if err != nil {
		t.Skip("NOTHING MEASURED: no landed-check cage on this host: " + err.Error())
	}
	got := read(cage.Argv)
	for _, s := range f.secrets {
		if got[s] != "" {
			t.Errorf("inside the landed-check cage, %s read %q — want no bytes (ranger-base-6b46n)", s, got[s])
		}
	}

	// The control: the identical call with the carve this fix replaced.
	ctl, prep, err := BwrapArgs(f.tree, []string{f.run}, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(prep); err != nil {
		t.Fatal(err)
	}
	bin, err := bwrapBin()
	if err != nil {
		t.Fatal(err)
	}
	got = read(append(append([]string{bin}, ctl...), "--", judge))
	for _, s := range f.secrets {
		if !strings.HasPrefix(got[s], "PRIVATE") {
			t.Errorf("control: %s inside an unmasked cage read %q — the probe cannot read, so the arm above proves nothing", s, got[s])
		}
	}
}

// TestQAEveryCageRenderCarriesTheSecretMasks is the census finding 2 of
// ranger-base-6b46n asks for. The ssh mask was ONE helper for both
// mechanisms (ranger-base-ighvi, ranger-base-7m3up) and the credential
// denies one function, but nothing held the SET of callers: the landed-check
// cage was a third render path that called neither, and every pin stayed
// green because every pin rendered through one of the other two.
//
// So the subject is the render and not a name list: any function in the
// MODULE's non-test sources that calls a cage renderer (BwrapArgs, its
// watched twin bwrapArgsWatched, SeatbeltProfile) must, at a line before its first render, call
// withSSHKeyReadDeny AND reach the credential denies — through
// SeatbeltCarveOut, which carries them, or credentialReadDenyLiterals
// directly. A render whose carve arrives from a caller would fail this: say
// why in the pin before exempting it, the way a wall states what it does not
// cover. Outside internal/posse the helpers are unexported, so a render
// there cannot carry them and is red for that reason.
//
// The floor is the wall against the sweep reading nothing: three paths
// render today — bwrapRender, RenderSeatbelt and boxLandedCage.
func TestQAEveryCageRenderCarriesTheSecretMasks(t *testing.T) {
	t.Parallel()
	renders := map[string]bool{"BwrapArgs": true, "bwrapArgsWatched": true, "SeatbeltProfile": true}
	fset := token.NewFileSet()
	paths := 0
	for _, src := range cdmModuleSources(t) {
		f, err := parser.ParseFile(fset, src.abs, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			first := map[string]int{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch g := call.Fun.(type) {
				case *ast.SelectorExpr:
					name = g.Sel.Name
				case *ast.Ident:
					name = g.Name
				}
				if l := fset.Position(call.Pos()).Line; name != "" && (first[name] == 0 || l < first[name]) {
					first[name] = l
				}
				return true
			})
			render := 0
			for name := range renders {
				if l := first[name]; l != 0 && (render == 0 || l < render) {
					render = l
				}
			}
			if render == 0 {
				continue
			}
			paths++
			who := src.rel + "." + fn.Name.Name
			if !strings.HasPrefix(src.rel, "internal/posse/") {
				t.Errorf("%s renders a cage at line %d OUTSIDE internal/posse, where withSSHKeyReadDeny and credentialReadDenyLiterals are unexported — the cage it builds reads every ssh key and credential on the host (ranger-base-6b46n)", who, render)
				continue
			}
			before := func(name string) bool { return first[name] != 0 && first[name] < render }
			if !before("withSSHKeyReadDeny") {
				t.Errorf("%s renders a cage at line %d without calling withSSHKeyReadDeny first — the child reads the ssh keys this box pushes with (ranger-base-6b46n, ranger-base-ighvi)", who, render)
			}
			if !before("withRexNodeReadDeny") {
				t.Errorf("%s renders a cage at line %d without calling withRexNodeReadDeny first — the child reads the rex tsnet node's tailnet credential (ranger-base-k06gw)", who, render)
			}
			if !before("SeatbeltCarveOut") && !before("credentialReadDenyLiterals") {
				t.Errorf("%s renders a cage at line %d without reaching the credential denies first (SeatbeltCarveOut or credentialReadDenyLiterals) — the child reads every runtime's credential file (ranger-base-6b46n, ranger-base-hw18)", who, render)
			}
		}
	}
	if paths < 3 {
		t.Fatalf("%d function(s) in this module render a cage (%v); there are three — bwrapRender, RenderSeatbelt, boxLandedCage — so the sweep is reading less than the tree (ranger-base-6b46n)", paths, renders)
	}
}
