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
// ranger-base-nfazj: BOTH OSes' XDG state defaults under each home — the
// darwin one is in the list on linux too, which is what keeps this a pure
// function and lets the pin run on either OS.
func TestQARexNodeStateDirsResolvesEveryHomeAndXDGState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a, b, x := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "xdg")
	got := rexNodeStateDirs([]string{a, "", b, a}, x)
	want := []string{
		absResolve(filepath.Join(a, ".local", "state", "rex", "tsnet")),
		absResolve(filepath.Join(a, "Library", "Application Support", "rex", "tsnet")),
		absResolve(filepath.Join(b, ".local", "state", "rex", "tsnet")),
		absResolve(filepath.Join(b, "Library", "Application Support", "rex", "tsnet")),
		absResolve(filepath.Join(x, "rex", "tsnet")),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rexNodeStateDirs = %q, want %q", got, want)
	}
	if got := rexNodeStateDirs(nil, "relative/state"); len(got) != 0 {
		t.Errorf("a relative XDG_STATE_HOME resolved to %q: the XDG spec calls it invalid, and a mask does not guess", got)
	}
}

// ranger-base-3cjzs: the mask reads XDG_STATE_HOME the way rex's own library
// reads it, which is ExpandHome FIRST and filepath.IsAbs second. Reading
// IsAbs on the raw value — what this did — masked NEITHER the path rex writes
// (not absolute raw, so the arm contributed nothing) nor anything else rex
// uses (the GOOS defaults it fell back to are not where an expanded value
// sends it), so the tailnet node identity was readable by every seat at both
// mechanisms on a box whose operator spells the variable that way.
//
// EVERY ROW HERE IS A PATH REX WOULD WRITE TO under some home, and the "tidy"
// rule is the hole: the library tests `path[0] == '~'` and
// `HasPrefix(path, "$HOME")`, so `~st` and `$HOMEx` expand as surely as
// `~/st` does, and a '~/'-only rule would leave those two uncovered.
func TestQARexNodeStateDirsExpandsAHomePrefixedXDGStateHome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	homes := []string{a, b}
	// The GOOS defaults, which stay listed whatever XDG_STATE_HOME says: an
	// entry rex does not use costs one inert tmpfs, and wider is the safe
	// direction for a mask.
	defaults := []string{
		absResolve(filepath.Join(a, ".local", "state", "rex", "tsnet")),
		absResolve(filepath.Join(a, "Library", "Application Support", "rex", "tsnet")),
		absResolve(filepath.Join(b, ".local", "state", "rex", "tsnet")),
		absResolve(filepath.Join(b, "Library", "Application Support", "rex", "tsnet")),
	}
	// underEveryHome is the rows one expandable value produces: the library's
	// own "/" fallback home first (it is what UserHomeDir answers when $HOME
	// is empty, and the mask cannot know which home rex ran under), then each
	// candidate home in turn.
	underEveryHome := func(rest ...string) []string {
		var out []string
		for _, h := range []string{"/", a, b} {
			out = append(out, absResolve(filepath.Join(append([]string{h}, rest...)...)))
		}
		return out
	}
	abs := filepath.Join(root, "xdg")
	for _, c := range []struct {
		xdg  string
		rows []string
	}{
		// The bead's own case, and `$HOME/st` is the same path by the
		// library's other branch.
		{"~/st", underEveryHome("st", "rex", "tsnet")},
		{"$HOME/st", underEveryHome("st", "rex", "tsnet")},
		// ANY leading '~': `~st` is <home>/st and NOT user st's home.
		{"~st", underEveryHome("st", "rex", "tsnet")},
		// ANY '$HOME' prefix: `$HOMEx` is <home>/x.
		{"$HOMEx", underEveryHome("x", "rex", "tsnet")},
		// `~` and `$HOME` alone Join to the home itself.
		{"~", underEveryHome("rex", "tsnet")},
		{"$HOME", underEveryHome("rex", "tsnet")},
		// An ALREADY-absolute value expands to itself under every home, so it
		// dedupes back to the single row it has always had, in the position it
		// has always had.
		{abs, []string{absResolve(filepath.Join(abs, "rex", "tsnet"))}},
		// Still relative after expansion: invalid by the XDG spec, ignored by
		// the library, and not guessed at here either.
		{"st", nil},
		{"./st", nil},
		{"", nil},
	} {
		want := append(append([]string{}, defaults...), c.rows...)
		got := rexNodeStateDirs(homes, c.xdg)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("rexNodeStateDirs(%q, %q) =\n  %q\nwant\n  %q", homes, c.xdg, got, want)
		}
	}
	// AND THE NATIVE ARM, which creates and anchors: one resolution shared
	// with the mask, so the directory posse makes is the one rex will write
	// to and not the GOOS default it would then not use. Expanded against its
	// OWN home — the create has to be exactly rex's resolution, where the
	// mask is deliberately wider.
	for _, c := range []struct{ goos, home, xdg, want string }{
		{"linux", a, "~/st", filepath.Join(a, "st", "rex", "tsnet")},
		{"darwin", a, "~/st", filepath.Join(a, "st", "rex", "tsnet")},
		{"linux", a, "$HOME/st", filepath.Join(a, "st", "rex", "tsnet")},
		{"linux", a, "~st", filepath.Join(a, "st", "rex", "tsnet")},
		{"linux", a, "$HOMEx", filepath.Join(a, "x", "rex", "tsnet")},
		{"linux", a, "~", filepath.Join(a, "rex", "tsnet")},
		{"darwin", a, "$HOME", filepath.Join(a, "rex", "tsnet")},
		// Not expandable, so the GOOS default stands, as before.
		{"linux", a, "st", filepath.Join(a, ".local", "state", "rex", "tsnet")},
		{"darwin", a, "./st", filepath.Join(a, "Library", "Application Support", "rex", "tsnet")},
		// NO $HOME: the library would expand against "/", and a create
		// deliberately stops short of that — making no directory is the
		// fail-closed direction for a mkdir, and the mask carries the "/" row
		// anyway.
		{"linux", "", "~/st", ""},
		{"darwin", "", "$HOME/st", ""},
	} {
		if got := rexNodeStateDirNative(c.goos, c.home, c.xdg); got != absResolveIfSet(c.want) {
			t.Errorf("rexNodeStateDirNative(%q, %q, %q) = %q, want %q", c.goos, c.home, c.xdg, got, c.want)
		}
	}
	// And every expanded native path is still a member of the mask, on the
	// GOOS it answers for: a create at a path nothing masks would be a
	// posse-made directory with no wall over it at all.
	for _, xdg := range []string{"~/st", "$HOME/st", "~st", "$HOMEx", "~", "$HOME", "st", ""} {
		for _, goos := range []string{"linux", "darwin"} {
			native := rexNodeStateDirNative(goos, a, xdg)
			if !wallMember(rexNodeStateDirs([]string{a}, xdg), native) {
				t.Errorf("XDG_STATE_HOME=%q: %s's native path %s is in no rexNodeStateDirs entry, so this launcher would create a directory its own mask does not cover", xdg, goos, native)
			}
		}
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
	mac := filepath.Join(f.home, "Library", "Application Support", "rex", "tsnet")
	xdg := filepath.Join(filepath.Dir(f.home), "xdg-state")
	t.Setenv("XDG_STATE_HOME", xdg)
	xdgDir := filepath.Join(xdg, "rex", "tsnet")
	for _, d := range []string{home, mac, xdgDir} {
		writeFile(t, filepath.Join(d, "node", "tailscaled.state"), "{}", 0o600)
	}
	// ranger-base-nfazj: the darwin default is masked by both renders too, on
	// whatever GOOS the pin runs — a dir that exists is masked, OS or not.
	dirs := []string{absResolve(home), absResolve(mac), absResolve(xdgDir)}

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

// AND THE SAME THING AT THE REAL RENDER, with XDG_STATE_HOME spelled the way
// the bead found it (ranger-base-3cjzs): `~/<dir>`, which rex expands against
// its own $HOME and writes the node identity under. The pure-function pins
// above are the mechanism; this is the level the exposure lived at, because
// the raw-IsAbs test made this whole arm contribute nothing to either wall —
// the dir below got no tmpfs and no read deny, with the credential in it.
func TestQAEverySeatsWallHidesAHomePrefixedRexNodeState(t *testing.T) {
	// No t.Parallel: the fixture sets HOME, and this sets XDG_STATE_HOME.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	t.Setenv("XDG_STATE_HOME", "~/xdg-state-tilde")
	dir := absResolve(filepath.Join(f.home, "xdg-state-tilde", "rex", "tsnet"))
	writeFile(t, filepath.Join(dir, "node", "tailscaled.state"), "{}", 0o600)

	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if bwIndex(r.Argv, "--tmpfs", dir) < 0 {
		t.Errorf("the bwrap wall mounts no tmpfs over %s, so the rex node credential is readable (ranger-base-3cjzs):\n  %s", dir, bwJoin(r.Argv))
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
	i := strings.Index(prof, "  (subpath "+sbQuote(dir)+")\n")
	if i < 0 {
		t.Fatalf("the seatbelt profile does not read-deny %s:\n%s", dir, prof)
	}
	if open := strings.LastIndex(prof[:i], "(deny file-read*\n"); open < 0 || strings.Contains(prof[open:i], ")\n(") {
		t.Errorf("%s is not inside a file-read* deny block:\n%s", dir, prof)
	}
	if i < strings.LastIndex(prof, "(allow file-read*") {
		t.Errorf("a file-read* allow follows the %s deny and could outvote it:\n%s", dir, prof)
	}

	// And the dir posse CREATES when it is absent is that same path, not the
	// GOOS default rex would then not use.
	var carve SeatbeltCarveOut
	withRexNodeReadDeny(&carve)
	if !wallMember(carve.DenyReadDirMake, dir) {
		t.Errorf("DenyReadDirMake = %q, which does not name %s: the launcher would create and anchor a directory rex does not write to", carve.DenyReadDirMake, dir)
	}
}
