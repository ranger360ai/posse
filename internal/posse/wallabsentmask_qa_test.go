//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-bncbk: a directory shape 5 masks is MADE when the host
// has nothing at the name, so the mask covers that name before anything
// writes a credential at it.
//
// THE HOLE. Steps 5a (~/.ssh) and 5b (the rex tsnet node's state dir) used to
// `continue` on an absent directory: no tmpfs, no watch, nothing. The stated
// reason — "a seat cannot create it either, nothing grants its parent" — is
// true and beside the point, because the HOST creates it. Fresh box, no
// ~/.ssh, seats running, the operator runs ssh-keygen to set up a push key:
// ssh-keygen mkdirs ~/.ssh and writes the private key, every live bwrap seat
// reads it through `--ro-bind / /`, and because no mount ever existed at that
// name nothing detached for `posse wall-watch` to see. The exposure lasted
// until each seat ended. 5b is the same story for a tailnet identity when rex
// first starts during a seat's life. It is ADR 0080's snapshot-at-launch
// residual at the two arms 5c's mkdir (ranger-base-a8qfg) never reached, and
// the seatbelt render never had it: a subpath read deny is about a PATH and
// covers a directory created later.
//
// The EMPTY-at-launch half (ranger-base-lin9y) is the other direction and the
// anchor is its answer: an empty mount point is one host rmdir from
// detaching, which was already loud here — a wall-watch kill and a P1 ROTATE
// bead — and is now prevented.
//
// WHAT IT COSTS, accepted by the security lane on lin9y: an empty
// .posse-wall-anchor in ~/.ssh and in rex's tsnet dir, and posse creating
// ~/.ssh 0700 on a box that has none. ssh ignores unknown names in ~/.ssh and
// StrictModes checks modes rather than extra names, and a launcher-made 0700
// ~/.ssh is byte-for-byte what ssh-keygen would have made.

import (
	"bytes"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The render, end to end over a fixture home that has neither directory: both
// are made 0700, anchored, covered by a tmpfs and watched as read masks — and
// the OTHER OS's XDG default, listed by rexNodeStateDirs and masked whenever
// it resolves, is created by nobody.
//
// Under wallUngrantedDir because a t.TempDir() is inside the blanket /tmp
// write grant, where `reopens` skips every one of these masks (wwPrivateTmp's
// comment has the same note).
//
// MUTATION-CHECKED 2026-11-21: dropping 5a's PrepMkdirPrivate step reds the
// ssh rows; dropping 5b's reds the rex rows; making 5b create every absent
// entry rather than carve.MakesDenyReadDir's reds the other-OS row.
func TestQAWallAbsentReadMaskDirIsMadeAnchoredAndWatched(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	f.ag.Deny = []string{"Bash(git push:*)"} // a seat that is not the pusher, so 5a renders
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	sshDir := absResolve(filepath.Join(f.home, ".ssh"))
	native := rexNodeStateDirNative(runtime.GOOS, f.home, "")
	otherOS := absResolve(filepath.Join(f.home, "Library", "Application Support", "rex", "tsnet"))
	if runtime.GOOS == "darwin" {
		otherOS = absResolve(filepath.Join(f.home, ".local", "state", "rex", "tsnet"))
	}
	for _, d := range []string{sshDir, native, otherOS} {
		if _, err := os.Lstat(d); !os.IsNotExist(err) {
			t.Fatalf("%s is already on the host (%v), so this pin would measure the present-directory arm instead", d, err)
		}
	}
	if !wallMember(r.Carve.DenyReadDir, otherOS) {
		t.Fatalf("the carve-out does not mask %s at all, so the row below says nothing about an absent entry posse may NOT create: %v", otherOS, r.Carve.DenyReadDir)
	}

	prep := strings.Join(bwPrep(r.Prep), "\n")
	line := bwJoin(r.Argv)
	for _, c := range []struct{ dir, why string }{
		{sshDir, "a key ssh-keygen writes here during this seat's life would be readable by every live seat, with no mount to detach and nothing for wall-watch to say (ranger-base-bncbk)"},
		{native, "a tailnet identity rex writes here on its first start would be readable by every live seat, and a copy of it outlives the seat and the box (ranger-base-k06gw, ranger-base-bncbk)"},
	} {
		if want := "mkdir -p -m 700 " + c.dir; !strings.Contains(prep, want) {
			t.Errorf("the prep has no %q — %s\n%s", want, c.why, prep)
		}
		if want := "seed " + filepath.Join(c.dir, wallAnchorName); !strings.Contains(prep, want) {
			t.Errorf("the prep has no %q — this launcher makes that directory and leaves it empty, so one host rmdir detaches the mask (ranger-base-ua625)\n%s", want, prep)
		}
		if bwIndex(r.Argv, "--tmpfs", c.dir) < 0 {
			t.Errorf("no tmpfs over %s — %s\n%s", c.dir, c.why, line)
		}
		if !wallWatchListed(r.Watch, c.dir) {
			t.Errorf("%s is mounted and not watched, so a host rmdir of it would take the mask off a live seat and `posse wall-watch` would never say so", c.dir)
		}
	}
	// AND THE ONE THE LAUNCHER MAY NOT MAKE. rexNodeStateDirs lists both
	// OSes' XDG defaults so that the MASK is wider than rex's own
	// resolution — the safe direction for a mask — but a create has to be
	// exactly rex's resolution, or posse invents a directory nothing on this
	// OS will ever write and then masks and anchors it forever.
	for _, no := range []string{"mkdir -p -m 700 " + otherOS, "mkdir -p " + otherOS, "seed " + filepath.Join(otherOS, wallAnchorName)} {
		if strings.Contains(prep, no) {
			t.Errorf("the prep has %q: %s is the OTHER OS's XDG state default, which nothing on %s writes — posse must mask it when it resolves and never create it (SeatbeltCarveOut.MakesDenyReadDir)\n%s", no, otherOS, runtime.GOOS, prep)
		}
	}
	if bwIndex(r.Argv, "--tmpfs", otherOS) >= 0 {
		t.Errorf("a tmpfs over %s, which is not on the host and which this render may not create — bwrap would have to make it: %s", otherOS, line)
	}
	if wallWatchListed(r.Watch, otherOS) {
		t.Errorf("%s is watched and nothing is mounted over it, so wall-watch would read the HOST's answer at that name as the seat's wall", otherOS)
	}

	// The prep runs, and then the SAME render asks for nothing: the anchor
	// that is already there emits no second step. That is PrepStep's own rule
	// and prepRequire rests on it, and bwrapAnchorStep broke it until this
	// bead — invisibly, because the one pin that holds it
	// (TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere) runs over a fixture
	// whose every credential store sits inside the blanket /tmp grant and so
	// renders no anchor at all.
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatalf("the prep this render asked for refused: %v", err)
	}
	for _, d := range []string{sshDir, native} {
		fi, err := os.Lstat(d)
		if err != nil || !fi.IsDir() {
			t.Fatalf("the prep did not make %s a directory: %v", d, err)
		}
		if got := fi.Mode().Perm(); got != 0o700 {
			t.Errorf("%s is mode %o, want 700 — that is the mode ssh-keygen and rex's tsnet node give these directories themselves, and a posse-made one at another mode is a directory the operator can tell from the one it preempted", d, got)
		}
	}
	if _, err := os.Lstat(otherOS); !os.IsNotExist(err) {
		t.Errorf("the prep made %s (%v): nothing on %s writes there", otherOS, err, runtime.GOOS)
	}
	again, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range again.Prep {
		if filepath.Base(s.Path) == wallAnchorName || s.Path == sshDir || s.Path == native {
			t.Errorf("after the prep ran, the same render still asks for %s (%s). prepRequire reads an empty step list as `this host has the whole wall`, so a step over a path that is already there makes a reader's probe abstain forever (wallPrep, wall.go)", s.Path, s.Kind)
		}
	}
}

// LIVE, and the only arm that measures the hole itself rather than the render
// that closes it: with no ~/.ssh at launch, the seat reads no byte of a key
// the HOST writes there while the seat is running.
//
// THE SEQUENCE IS THE INCIDENT'S, and that is the whole reason this arm is
// live rather than a render pin. The seat signals when its namespace is up;
// only then does the host write the key; only then is the seat told to read.
// A key written before the mount would be measuring the launch snapshot, which
// is what shape 5 always covered and not what this bead is about.
//
// MEASURED 2026-11-21 (linux 7.2.15-1-omarchy, bwrap 0.13.1), both directions:
// with 5a reverted to its skip the seat reads the key in full, and the control
// arm at the foot reads it with no mask at all — so a green here is this wall
// and not a probe that could never read.
//
// IT MEASURES NOTHING FROM A CAGED SEAT (ADR 0096 D3, ADR 0097): a bwrap
// inside a bwrap is refused, so bwrapNestSkip takes the arm out wherever the
// suite runs behind the wall, and the wall venue is where the reading above
// was taken.
func TestLiveBwrapAbsentSSHDirMasksAKeyTheHostWritesAfterLaunch(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	bwrapNestSkip(t)
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	f.ag.Deny = []string{"Bash(git push:*)"}
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	sshDir := absResolve(filepath.Join(f.home, ".ssh"))
	if _, serr := os.Lstat(sshDir); !os.IsNotExist(serr) {
		t.Fatalf("%s is already there (%v): this arm's whole premise is a box with no ~/.ssh", sshDir, serr)
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(sshDir, "id_ed25519")
	const secret = "PRIVATE-KEY-WRITTEN-AFTER-LAUNCH"
	// Both signals live in the project grant: /tmp is the seat's own private
	// tmpfs here, so a file the host drops there is not the file the seat sees.
	up, goAhead := filepath.Join(f.dir, "seat-up"), filepath.Join(f.dir, "seat-go")
	script := `: >"$1"; while [ ! -f "$2" ]; do sleep 0.02; done; cat -- "$3" 2>&1 || true`
	// THE OUTPUT IS READ AFTER cmd.Wait RETURNS and never before, which is
	// both the correctness and the race: the first draft returned out.String()
	// with the wait in a defer, so it read the buffer while the child was
	// still writing it — and read "" from the CONTROL arm, where "" is the
	// answer that would have made the masked arm above vacuous.
	read := func(argv []string) string {
		t.Helper()
		os.Remove(up)
		os.Remove(goAhead)
		cmd := exec.Command("bwrap", append(append([]string{}, argv...), "sh", "-c", script, "sh", up, goAhead, key)...)
		out := &bytes.Buffer{}
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Start(); err != nil {
			t.Fatalf("bwrap: %v", err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		// The seat's namespace is up when it says so, and the host acts only
		// after that: a key written before the mount would be measuring the
		// snapshot this bead is not about.
		for i := 0; ; i++ {
			if _, err := os.Lstat(up); err == nil {
				break
			}
			select {
			case werr := <-done:
				t.Fatalf("the seat exited (%v) before signalling at %s; it never reached the read: %s", werr, up, out)
			default:
			}
			if i > 1000 {
				_ = cmd.Process.Kill()
				<-done
				t.Fatalf("the seat never signalled it was up (%s): %s", up, out)
			}
			time.Sleep(10 * time.Millisecond)
		}
		writeFile(t, key, secret+"\n", 0o600)
		writeFile(t, goAhead, "", 0o600)
		if werr := <-done; werr != nil {
			t.Fatalf("the seat exited %v: %s", werr, out)
		}
		return out.String()
	}
	got := read(r.Argv)
	if strings.Contains(got, secret) {
		t.Errorf("inside a seat launched with NO ~/.ssh, a key the host wrote there afterwards read %q — the mask must cover the name before the key exists at it (ranger-base-bncbk)", got)
	}
	// THE CONTROL: the same probe with no mask over ~/.ssh reads the key in
	// full, so the row above is about the wall and not about a read that
	// could never have worked.
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	bare := []string{"--die-with-parent", "--ro-bind", "/", "/", "--dev-bind", "/dev", "/dev", "--unshare-user", "--bind", f.dir, f.dir}
	if ctl := read(bare); !strings.Contains(ctl, secret) {
		t.Fatalf("the unmasked control read %q and not the key — then the arm above measured a probe that cannot read, not a wall", ctl)
	}
}

// 5a's own arm, over a PRESENT but empty ~/.ssh: no mkdir — the host made it
// — and the anchor all the same, which is ranger-base-lin9y's residual turned
// from detect-and-kill into prevent. A host rmdir is then refused.
func TestQAWallEmptySSHDirIsAnchoredAgainstAHostRmdir(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"wt", "home/.ssh"}, nil)
	sshDir := bwAt("home", ".ssh")
	_, prep, err := BwrapArgs(bwAt("wt"), []string{bwAt("wt")}, SeatbeltCarveOut{SSHDir: []string{sshDir}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(bwPrep(prep), "\n"), "seed "+filepath.Join(sshDir, wallAnchorName); got != want {
		t.Fatalf("the prep for a present empty ~/.ssh is\n  %s\nwant\n  %s\n(no mkdir: the host made the directory. The anchor: an empty mount point is one host rmdir from detaching, MEASURED for the other arms under ranger-base-ua625)", got, want)
	}
	if err := runBwrapPrep(prep); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sshDir); err == nil {
		t.Errorf("a host rmdir of %s succeeded after the prep — that detaches the read mask off every live seat, which then reads a key the host writes at that name afterwards", sshDir)
	}
}

// The native path, as a pure function, on both OSes from either: $XDG_STATE_HOME
// when it is absolute, else the GOOS default, and "" when there is nothing to
// resolve against. It is the one half of the rex list that is per-GOOS, and
// everything posse CREATES is keyed on it.
func TestRexNodeStateDirNativeIsOneGOOSsAnswer(t *testing.T) {
	t.Parallel()
	home := absResolve(t.TempDir())
	xdg := absResolve(filepath.Join(home, "xdg"))
	for _, c := range []struct{ goos, home, xdg, want string }{
		{"linux", home, "", filepath.Join(home, ".local", "state", "rex", "tsnet")},
		{"darwin", home, "", filepath.Join(home, "Library", "Application Support", "rex", "tsnet")},
		// An absolute XDG_STATE_HOME outranks the default on either OS, which
		// is what the xdg library rex links does (EnvPath).
		{"linux", home, xdg, filepath.Join(xdg, "rex", "tsnet")},
		{"darwin", home, xdg, filepath.Join(xdg, "rex", "tsnet")},
		// A RELATIVE one is invalid by the XDG spec, so the default stands —
		// rexNodeStateDirs' own rule, and a create must not guess what it meant.
		{"linux", home, "rel/state", filepath.Join(home, ".local", "state", "rex", "tsnet")},
		// NO HOME AND NO ABSOLUTE XDG: nothing is created. The fail-closed
		// direction for a mkdir, and the mask is unaffected either way.
		{"linux", "", "", ""},
		{"darwin", "", "rel", ""},
	} {
		if got := rexNodeStateDirNative(c.goos, c.home, c.xdg); got != absResolveIfSet(c.want) {
			t.Errorf("rexNodeStateDirNative(%q, %q, %q) = %q, want %q", c.goos, c.home, c.xdg, got, c.want)
		}
	}
	// And it is always one of the paths the MASK covers, on the GOOS it
	// answers for: a create at a path nothing masks would be a posse-made
	// directory with no wall over it at all.
	for _, goos := range []string{"linux", "darwin"} {
		native := rexNodeStateDirNative(goos, home, "")
		if !wallMember(rexNodeStateDirs([]string{home}, ""), native) {
			t.Errorf("%s's native path %s is in no rexNodeStateDirs entry, so this launcher would create a directory its own mask does not cover", goos, native)
		}
	}
}

// absResolveIfSet is absResolve for a want that may be "", which absResolve
// would turn into the cwd.
func absResolveIfSet(p string) string {
	if p == "" {
		return ""
	}
	return absResolve(p)
}

// ─── the precondition: a test's prep writes inside the fixtures ──────────────

// THE FENCE IS UP IN THIS BINARY, and a step outside it refuses. The hard arm
// of prephostfence.go: steps 5a and 5b mkdir and anchor now, and two of the
// render's host inputs ignore $HOME — the passwd home liveSSHHomes reads and
// XDG_STATE_HOME — so without the fence every test here that renders a whole
// wall would plant a file in the OPERATOR's ~/.ssh and in rex's live tsnet
// state dir. ranger-base-lin9y measured that happening; the files were
// removed by hand.
//
// A pin keyed on "the fence refused something" alone would pass with the
// fence down, so both halves are here: that roots are declared at all, and
// that a path outside them is refused.
func TestQATheBwrapPrepHostFenceIsUpAndRefusesAStepOutsideIt(t *testing.T) {
	t.Parallel()
	roots := PrepHostFenceRoots()
	if len(roots) == 0 {
		t.Fatal("the bwrap prep host fence declares no root, so runBwrapPrep in this test binary will write wherever a render's unfenced host inputs point. FenceCLIHomes is what puts it up (prephostfence.go, ranger-base-bncbk)")
	}
	inside := filepath.Join(t.TempDir(), "made-here")
	if err := runBwrapPrep([]PrepStep{{PrepMkdirPrivate, inside, "a fixture path"}}); err != nil {
		t.Fatalf("the fence refused a step under t.TempDir(): %v — then every wall pin in this package is measuring the fence instead of a wall", err)
	}
	if fi, err := os.Lstat(inside); err != nil || !fi.IsDir() {
		t.Fatalf("the control step made nothing at %s (%v), so the refusal below is not about the fence", inside, err)
	}
	// The one path this bead is about, and the reason it cannot be a
	// t.TempDir(): the fence has to refuse a path chosen by the HOST rather
	// than by the test. `$HOME/.ssh` of the real passwd home is what the
	// render used to produce here.
	outside := filepath.Join("/etc", "posse-fence-probe", ".ssh")
	err := runBwrapPrep([]PrepStep{{PrepMkdirPrivate, outside, "the operator's own area"}})
	if err == nil {
		t.Fatalf("runBwrapPrep made %s with the fence up: a prep step is a write on the HOST in the seat's name, so an unfenced one is a test writing into somebody's live area", outside)
	}
	for _, want := range []string{outside, "test binary"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
	if _, serr := os.Lstat(outside); !os.IsNotExist(serr) {
		t.Errorf("the refused step made %s anyway (%v)", outside, serr)
	}
	// AND THE SECOND ARM, the KEEP OUT at the passwd home, which refuses
	// whatever the roots say. The roots alone are blind on a box whose real
	// home is itself under a temp root, where "under a fixture root" would be
	// true of exactly the directory the fence exists to protect. Driven at
	// the real path and not a fixture, because that path is what the render
	// produced before this bead.
	u, uerr := user.Current()
	if uerr != nil || u.HomeDir == "" {
		t.Skip("NOTHING MEASURED for the keep-out arm: passwd has no home for this uid")
	}
	keyPath := filepath.Join(u.HomeDir, ".ssh", wallAnchorName)
	kerr := runBwrapPrep([]PrepStep{{PrepSeed, keyPath, "the anchor ranger-base-lin9y measured landing in the operator's own ~/.ssh"}})
	if kerr == nil {
		t.Fatalf("runBwrapPrep seeded %s: that is the write ranger-base-lin9y measured, and no root may permit it", keyPath)
	}
	if !strings.Contains(kerr.Error(), "passwd home") {
		t.Errorf("the refusal does not name the arm that made it: %v", kerr)
	}
}

// AND THE FIX THE FENCE IS A PIN OVER: a render under a fixture $HOME names
// that home's ~/.ssh and rex dirs and no other home's. Before this bead the
// same render named the operator's real ~/.ssh first, because liveSSHHomes
// reads the passwd database and `t.Setenv("HOME", …)` does not move it.
func TestQAARenderUnderAFixtureHomeNamesNoOtherHomesSSHDir(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	f.ag.Deny = []string{"Bash(git push:*)"}
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	home := absResolve(f.home)
	for _, d := range append(append([]string{}, r.Carve.SSHDir...), append(append([]string{}, r.Carve.DenyReadDir...), r.Carve.DenyReadDirMake...)...) {
		if !underDir(home, d) {
			t.Errorf("the render names %s, which is outside this fixture's home %s. Steps 5a and 5b mkdir and anchor now, so a path from another home is this test about to write there — see prephostfence.go for the two host inputs that do not move with $HOME", d, home)
		}
	}
	if len(r.Carve.SSHDir) == 0 || len(r.Carve.DenyReadDir) == 0 {
		t.Errorf("the render names no ssh dir (%v) or no read-masked state dir (%v), so the rows above are vacuous", r.Carve.SSHDir, r.Carve.DenyReadDir)
	}
}
