//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ranger-base-k06gw: the rex tsnet node's state dir is resolved for both
// homes and for an absolute XDG_STATE_HOME, and nothing for a relative one.
func TestQARexNodeStateDirsResolvesEveryHomeAndXDGState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a, b, x := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "xdg")
	got := rexNodeStateDirs([]string{a, "", b, a}, x)
	want := []string{
		absResolve(filepath.Join(a, ".local", "state", "rex", "tsnet")),
		absResolve(filepath.Join(b, ".local", "state", "rex", "tsnet")),
		absResolve(filepath.Join(x, "rex", "tsnet")),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rexNodeStateDirs = %q, want %q", got, want)
	}
	if got := rexNodeStateDirs(nil, "relative/state"); len(got) != 0 {
		t.Errorf("a relative XDG_STATE_HOME resolved to %q: the XDG spec calls it invalid, and a mask does not guess", got)
	}
}

// BwrapArgs step 5b: an empty tmpfs with nothing bound back, after every
// grant; a FILE at the name refuses; an absent one renders nothing; and a
// DenyRead literal under it renders no /dev/null mount of its own.
func TestQABwrapArgsDenyReadDirIsAnEmptyTmpfs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	grant := filepath.Join(root, "grant")
	dir := filepath.Join(root, "state", "rex", "tsnet")
	key := filepath.Join(dir, "node", "tailscaled.state")
	writeFile(t, key, "{}", 0o600)
	if err := os.MkdirAll(grant, 0o755); err != nil {
		t.Fatal(err)
	}
	argv, _, err := BwrapArgs("", []string{grant}, SeatbeltCarveOut{DenyReadDir: []string{dir}, DenyRead: []string{key}})
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(argv)
	tmpfs := bwIndex(argv, "--tmpfs", dir)
	if tmpfs < 0 {
		t.Fatalf("no tmpfs over %s, so the node credential under it is readable (ranger-base-k06gw):\n  %s", dir, line)
	}
	if g := bwIndex(argv, "--bind", grant, grant); g < 0 || g > tmpfs {
		t.Errorf("the tmpfs over %s is not after the write grant (grant at %d, tmpfs at %d):\n  %s", dir, g, tmpfs, line)
	}
	for i := tmpfs + 2; i < len(argv); i++ {
		if underDir(dir, argv[i]) {
			t.Errorf("%s is mounted under the %s tmpfs: nothing is bound back there:\n  %s", argv[i], dir, line)
		}
	}

	f := filepath.Join(root, "file", "tsnet")
	writeFile(t, f, "x", 0o600)
	if _, _, err := BwrapArgs("", nil, SeatbeltCarveOut{DenyReadDir: []string{f}}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("a FILE at %s rendered a wall (err=%v)", f, err)
	}
	absent := filepath.Join(root, "nohome", "tsnet")
	argv, _, err = BwrapArgs("", nil, SeatbeltCarveOut{DenyReadDir: []string{absent}})
	if err != nil {
		t.Errorf("an absent dir refused: %v", err)
	}
	if bwIndex(argv, "--tmpfs", absent) >= 0 {
		t.Errorf("an absent dir got a tmpfs, which bwrap would have to create: %v", argv)
	}
}

// The renders: every seat's bwrap wall hides the tsnet dir, the pusher's
// included, and so does its seatbelt profile, as a subpath read deny after
// every read allow. Both reports name it.
func TestQAEverySeatsWallHidesTheRexNodeState(t *testing.T) {
	// No t.Parallel: the fixture sets HOME, and this sets XDG_STATE_HOME.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	home := filepath.Join(f.home, ".local", "state", "rex", "tsnet")
	xdg := filepath.Join(filepath.Dir(f.home), "xdg-state")
	t.Setenv("XDG_STATE_HOME", xdg)
	xdgDir := filepath.Join(xdg, "rex", "tsnet")
	for _, d := range []string{home, xdgDir} {
		writeFile(t, filepath.Join(d, "node", "tailscaled.state"), "{}", 0o600)
	}
	dirs := []string{absResolve(home), absResolve(xdgDir)}

	for _, c := range []struct {
		name        string
		deny, allow []string
	}{
		{"push-denied", []string{"Bash(git push:*)"}, nil},
		{"the pusher", nil, []string{"Bash(git push:*)"}},
	} {
		f.ag.Deny, f.ag.Allow = c.deny, c.allow
		r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range dirs {
			if bwIndex(r.Argv, "--tmpfs", d) < 0 {
				t.Errorf("%s: the bwrap wall mounts no tmpfs over %s, so the rex node credential is readable (ranger-base-k06gw):\n  %s", c.name, d, bwJoin(r.Argv))
			}
		}

		p, err := f.a.RenderSeatbelt(f.ag, f.dir)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		prof := string(b)
		lastAllow := strings.LastIndex(prof, "(allow file-read*")
		for _, d := range dirs {
			i := strings.Index(prof, "  (subpath "+sbQuote(d)+")\n")
			if i < 0 {
				t.Errorf("%s: the seatbelt profile does not read-deny %s:\n%s", c.name, d, prof)
				continue
			}
			open := strings.LastIndex(prof[:i], "(deny file-read*\n")
			if open < 0 || strings.Contains(prof[open:i], ")\n(") || strings.Contains(prof[open:i], "file-write*") {
				t.Errorf("%s: %s is not inside a file-read* deny block:\n%s", c.name, d, prof)
			}
			if i < lastAllow {
				t.Errorf("%s: a file-read* allow follows the %s deny and could outvote it:\n%s", c.name, d, prof)
			}
		}
		var rep strings.Builder
		if err := f.a.SeatbeltReport(f.ag, f.dir, &rep); err != nil {
			t.Fatal(err)
		}
		for _, d := range dirs {
			if want := "r " + AbbrevHome(d) + "/ (file-read* subpath deny — the rex tsnet node"; !strings.Contains(rep.String(), want) {
				t.Errorf("%s: the seatbelt report does not name %s:\n%s", c.name, d, rep.String())
			}
		}
	}
}
