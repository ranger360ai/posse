//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ADR 0100 D1 and D2(a): every bwrap bind SOURCE that differs from
// its destination lies under the per-seat WALL dir, and no render grants
// write at or above the wall ROOT.
//
// WHAT THE CLASS IS. bwrap resolves a mount source at mount time and follows
// links; the render's component walk (ranger-base-wsgus) judges the same
// source a moment earlier. While the sources lived under the seat's own
// cache dir — a directory the seat is granted by name — anything the seat
// left behind could toggle a component between the two instants, and for the
// private /tmp there was no race to win at all: that source went through
// os.Stat, which follows, so a planted link was simply mounted read-write
// over /tmp (MEASURED, ranger-base-p47q6). D1 moves the sources where no
// walled seat can write a component of them; D2(a) is the refusal that keeps
// that true as grants change.
//
// THE FIXTURE HOMES ARE UNDER wallUngrantedDir, every one of them, and that
// is not decoration: `/tmp` is granted unconditionally (SeatbeltWritable), a
// home under it makes the wall root's parent a granted directory, and the
// refusal stands down there on purpose (bwrapWallRoot). A pin written over a
// t.TempDir() home would measure the stand-down and read green for it.
//
//	go test ./internal/posse -run 'WallSeatWall|WallRootGrant' -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ─── D1: where the sources live ──────────────────────────────────────────────

// The composition, as a pure predicate: one wall dir per session dir, under
// `<home>/.cache/posse-wall`, keyed exactly as the seat cache dir is keyed —
// and a sibling of that cache, never a child of it, because the cache is
// granted and the wall must not be.
func TestQAWallSeatWallDirIsAPerSessionSiblingOfTheSeatCache(t *testing.T) {
	t.Parallel()
	const home = "/home/someone"
	a, b := SeatWallDir(home, "/r1/wt/devops-x"), SeatWallDir(home, "/r2/wt/devops-x")
	if a == b {
		t.Errorf("SeatWallDir names one wall dir for two session dirs: %s", a)
	}
	root := SeatWallRoot(home)
	if root != filepath.Join(home, ".cache", "posse-wall") {
		t.Errorf("SeatWallRoot = %q, want <home>/.cache/posse-wall", root)
	}
	if !insideDir(root, a) || filepath.Dir(a) != root {
		t.Errorf("SeatWallDir = %q, want a direct child of the wall root %q", a, root)
	}
	// The key is the cache dir's key: one function, so a reader can pair the
	// two directories of one seat by eye.
	if filepath.Base(a) != filepath.Base(SeatCacheDir(home, "/r1/wt/devops-x")) {
		t.Errorf("SeatWallDir %q and SeatCacheDir %q are keyed differently", a, SeatCacheDir(home, "/r1/wt/devops-x"))
	}
	// And the wall root is not inside the cache root, nor the other way
	// round. A wall dir under the seat cache is the defect this ADR closes.
	cacheRoot := filepath.Join(home, seatCacheRootRel)
	if insideDir(cacheRoot, root) || insideDir(root, cacheRoot) {
		t.Errorf("the wall root %q and the seat cache root %q contain one another — the seat is granted its cache, so its bind sources would sit under a grant again (ADR 0100 D1)", root, cacheRoot)
	}
	if SeatWallDir("", "/x") != "" || SeatWallDir("/h", "") != "" || SeatWallRoot("") != "" {
		t.Error("SeatWallDir/SeatWallRoot with no home or no session dir must name nothing")
	}
}

// Pin (c): EVERY private source this package composes — the seat's /tmp,
// claude's private member dirs, its private history.jsonl, and the two paths
// an UNCAGED posse reads back — is under the one SeatWallDir, and each
// private bind declares that dir as the root its walk may trust.
//
// Protects the thing a list of paths cannot: a reader or a producer that
// composes its own copy of the path and is left behind under the granted
// cache dir, which is exactly how the two uncaged readers would have been
// missed (ADR 0100's Consequences).
func TestQAWallSeatWallHoldsEveryPrivateSourceAndEveryReader(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	wall := SeatWallDir(f.home, f.dir)
	if wall == "" {
		t.Fatal("no wall dir for the fixture's home and session dir, so nothing below is measured")
	}
	grants := runtimeStateGrants(f.rt.StateDirs)
	binds := claudePrivateSessionState(f.home, f.dir, grants)
	if len(binds) != len(claudeSessionStateDirNames)+1 {
		t.Fatalf("claudePrivateSessionState rendered %d binds, want one per member plus history.jsonl — the loop below would measure a subset", len(binds))
	}
	for _, pb := range binds {
		if !insideDir(wall, pb.Src) {
			t.Errorf("the private source %s is not under the seat's wall dir %s, so a walled seat can write a component of it and swap the bind (ADR 0100 D1)", pb.Src, wall)
		}
		if pb.SrcRoot != wall {
			t.Errorf("the private bind over %s declares SrcRoot %q, want the wall dir %q — the walk may trust no deeper root", pb.Dst, pb.SrcRoot, wall)
		}
	}
	readers := map[string]string{
		"SeatTmpDir":                  SeatTmpDir(f.home, f.dir),
		"claudePrivateStateDir":       claudePrivateStateDir(f.home, f.dir),
		"claudePrivateHistoryPath":    claudePrivateHistoryPath(f.home, f.dir),
		"claudePrivateSessionRowPath": claudePrivateSessionRowPath(f.home, f.dir, 4242),
	}
	for name, p := range readers {
		if p == "" {
			t.Errorf("%s names nothing for a fixture that has both a home and a session dir", name)
			continue
		}
		if !insideDir(wall, p) {
			t.Errorf("%s = %s, which is not under the wall dir %s — an uncaged posse would read it through components a seat can replace (ADR 0100 D1)", name, p, wall)
		}
	}

	// And nothing grants any of it. The write set is where this is decided:
	// the seat reaches the CONTENTS through the mounts at Dst and needs no
	// grant on the sources at all.
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, f.rt.StateDirs...)
	if !writeGranted(w, absResolve(SeatCacheDir(f.home, f.dir))) {
		t.Fatalf("control: the seat's own CACHE dir is not granted either, so the rows below measure an empty write set: %v", w)
	}
	for _, p := range append([]string{SeatWallRoot(f.home), wall}, readers["SeatTmpDir"], readers["claudePrivateStateDir"]) {
		if writeGranted(w, absResolve(p)) {
			t.Errorf("the write set grants %s — every bind source under it is one the seat can swap for a link (ADR 0100 D1/D2(a))\n set: %v", p, w)
		}
	}
}

// ─── D2(a): no grant at or above the wall root ───────────────────────────────

// wrgHome is a home outside every blanket temp grant, with the wall root and
// a `~/.cache` on disk, and the grant paths a render would be refused for.
// On a host where the refusal stands down it skips, saying so.
func wrgHome(t *testing.T, home string) (cache, wallRoot, link string) {
	t.Helper()
	cache, wallRoot = filepath.Join(home, ".cache"), SeatWallRoot(home)
	link = filepath.Join(home, "cache-link")
	if err := os.MkdirAll(wallRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wallRoot, link); err != nil {
		t.Fatal(err)
	}
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: this render's wall-root refusal stands down for a home under " + bwrapTmpMount + " (bwrapWallRoot), so no grant here can be refused")
	}
	return cache, wallRoot, link
}

// Pin (b), the one door: BwrapArgs is where all three of this package's
// renders meet, so a grant at or above the wall root is refused there — and
// a grant BESIDE it still renders, which is what keeps this from being a
// refusal of everything.
func TestQAWallRootGrantIsRefusedByBwrapArgs(t *testing.T) {
	// No t.Parallel: this sets HOME, which is what the wall root is
	// composed from.
	root := wallUngrantedDir(t)
	home := filepath.Join(root, "home")
	proj := filepath.Join(root, "proj")
	for _, d := range []string{home, proj} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	cache, wallRoot, link := wrgHome(t, home)

	for _, c := range []struct{ name, grant string }{
		{"the wall root itself", wallRoot},
		{"the cache dir above it", cache},
		{"the home above that", home},
		// The grant side is compared RESOLVED, because that is what the
		// bind exposes: a `writable:` extra a seat planted as a link at a
		// path of its own names the wall root in no other spelling.
		{"a link to the wall root", link},
	} {
		argv, prep, err := BwrapArgs(proj, []string{c.grant}, SeatbeltCarveOut{})
		switch {
		case err == nil:
			t.Errorf("a grant of %s (%s) rendered: every walled seat's bind sources lie under %s, and this grant hands one seat a component of all of them (ADR 0100 D2(a))\n argv: %q", c.name, c.grant, wallRoot, argv)
		case !strings.Contains(err.Error(), wallRoot):
			t.Errorf("a grant of %s was refused with %q, which does not name the wall root — the refusal is the operator's only instruction for what to narrow", c.name, err)
		}
		if len(prep) != 0 {
			t.Errorf("a grant of %s came back with prep %v for the uncaged launcher to run", c.name, prep)
		}
	}

	// The siblings, which are the whole point of the wall root being a
	// sibling: the seat's own cache dir and the box's lock dir still render.
	for _, g := range []string{
		SeatCacheDir(home, proj),
		filepath.Join(home, ".cache", "posse"),
	} {
		if err := os.MkdirAll(g, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := BwrapArgs(proj, []string{g}, SeatbeltCarveOut{}); err != nil {
			t.Errorf("a grant of %s is refused (%v) — it is beside the wall root, not above it, and a seat that cannot build is not a wall", g, err)
		}
	}
}

// THE WALL ROOT IS COMPARED AS COMPOSED AND THE GRANT AS RESOLVED, and the
// two sides are not interchangeable (ADR 0100 D2(a), and the design review that asked for it). underDir
// resolves BOTH, so a link planted at the wall root's own name would make it
// judge the link's TARGET: a grant that contains the name — the one a seat
// would delete and replace — then reads as containing nothing. This is the
// disagreement, pinned on its own, because every other arm here renders a
// real directory at that name and the two predicates agree over one.
//
// Mutating insideDir into underDir in bwrapRefuseWallRootGrant must red
// this and nothing else.
func TestQAWallRootGrantComparesTheComposedSpellingNotTheResolved(t *testing.T) {
	// No t.Parallel: this sets HOME.
	root := wallUngrantedDir(t)
	home, proj, elsewhere := filepath.Join(root, "home"), filepath.Join(root, "proj"), filepath.Join(root, "elsewhere")
	for _, d := range []string{home, proj, elsewhere, filepath.Join(home, ".cache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: this render's wall-root refusal stands down for a home under " + bwrapTmpMount)
	}
	wallRoot := SeatWallRoot(home)
	if err := os.Symlink(elsewhere, wallRoot); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(home, ".cache")
	_, _, err := BwrapArgs(proj, []string{cache}, SeatbeltCarveOut{})
	if err == nil || !strings.Contains(err.Error(), wallRoot) {
		t.Fatalf("a grant of %s rendered (err %v) while %s is a link to %s: the grant holds the NAME every source is composed under, so the seat can unlink it and plant its own — and the sources are composed from that name, not from where it happens to point today (ADR 0100 D2(a))", cache, err, wallRoot, elsewhere)
	}
}

// Pin (b) at the SEAT render: a PID `writable:` extra naming `~/.cache` is
// the shape an operator would actually type, and the render refuses it with
// the same sentence. The control is the identical render without the extra.
func TestQAWallRootGrantIsRefusedByTheSeatRender(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	cache, wallRoot, _ := wrgHome(t, f.home)
	if _, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates); err != nil {
		t.Fatalf("control: this seat's render is already refused without any extra (%v), so the arm below measures nothing", err)
	}
	f.ag.Writable = []string{cache}
	_, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err == nil || !strings.Contains(err.Error(), wallRoot) {
		t.Fatalf("a `writable:` extra of %s rendered (err %v) — the seat would hold a component of every seat's bind sources (ADR 0100 D2(a))", cache, err)
	}
}

// Pin (b) at the other two renders: the landed-check judge and the wall
// venue. Both compose their own writable set and both come through
// BwrapArgs, which is what this measures — a render that grew a second argv
// builder would pass the two arms above and fail here.
func TestQAWallRootGrantIsRefusedByTheLandedJudgeAndTheVenue(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := newLandedCageFixture(t)
	_, wallRoot, _ := wrgHome(t, f.ssh.home)
	judge := filepath.Join(f.run, "judge")
	for _, kind := range []landedCageKind{landedCageJudge, landedCageVenue} {
		if _, err := boxLandedCage(&App{}, kind, judge, f.tree, []string{f.run}, nil); err != nil {
			t.Skipf("NOTHING MEASURED: this host renders no %s cage at all: %v", kind, err)
		}
		cage, err := boxLandedCage(&App{}, kind, judge, f.tree, []string{f.run, filepath.Join(f.ssh.home, ".cache")}, nil)
		if WallMechanism() != WallBwrap {
			t.Skip("NOTHING MEASURED: the wall-root refusal is the bwrap render's; this host's mechanism is " + WallMechanism())
		}
		if err == nil || !strings.Contains(err.Error(), wallRoot) {
			t.Errorf("the %s cage rendered with a grant above the wall root (err %v): a child that can write %s can swap any seat's bind source (ADR 0100 D2(a))\n argv: %q", kind, err, wallRoot, cage.Argv)
		}
	}
}

// ─── D1 pin (a): the private /tmp source takes the component walk ────────────

// A link at the private /tmp source refuses the render and emits NO prep
// step — the prep runs in the UNCAGED launcher, so a render that followed
// the link would also aim the launcher's own 0755 mkdir at the link's
// target. Both components are measured: the source's own name, and its
// parent with a real `tmp` directory inside the target, which is the shape
// that passes a check of the last component alone.
//
// Mutating bwrapSrcWalk back to a bwrapStat in the 1c arm must red this.
func TestBwrapArgsRefusesASeatTmpThatIsALink(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj", "wall", "elsewhere/tmp"}, nil)
	for _, c := range []struct{ name, link, at string }{
		{"a link at the private tmp source itself", bwAt("elsewhere", "tmp"), bwAt("wall", "tmp")},
		{"a link at its parent, with a real tmp inside the target", bwAt("elsewhere"), bwAt("wall2")},
	} {
		if err := os.Symlink(c.link, c.at); err != nil {
			t.Fatal(err)
		}
		tmp := bwAt("wall", "tmp")
		if filepath.Base(c.at) != "tmp" {
			tmp = filepath.Join(c.at, "tmp")
		}
		argv, prep, err := BwrapArgs(bwAt("proj"), nil, SeatbeltCarveOut{Tmp: tmp})
		if err == nil {
			t.Errorf("%s rendered, so bwrap mounts %s read-write over %s inside the seat's namespace (ranger-base-p47q6)\n argv: %q", c.name, c.link, bwrapTmpMount, argv)
		}
		if len(prep) != 0 {
			t.Errorf("%s came back with prep %v, which the UNCAGED launcher runs", c.name, prep)
		}
		if err := os.Remove(c.at); err != nil {
			t.Fatal(err)
		}
	}
	// The control: a real directory at the same name renders the bind, so
	// the refusals above are about the link and not about the fixture.
	if err := os.MkdirAll(bwAt("wall", "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := BwrapArgs(bwAt("proj"), nil, SeatbeltCarveOut{Tmp: bwAt("wall", "tmp")}); err != nil {
		t.Errorf("control: a real directory at the private tmp source is refused too (%v) — then the arms above measure the fixture", err)
	}
}

// The live half: what the refused render would have handed bwrap. The
// control is the measurement the fix rests on — bwrap FOLLOWS a source link,
// so the seat's writes to /tmp land in the link's target on the host — and
// the arm beside it is this package's render declining to produce that argv.
func TestLiveBwrapFollowsATmpSourceLinkAndTheRenderRefusesIt(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	bwrapNestSkip(t) // ADR 0096 D3: this arm execs a bwrap
	root := absResolve(t.TempDir())
	victim := filepath.Join(root, "victim")
	link := filepath.Join(root, "wall", "tmp")
	for _, d := range []string{victim, filepath.Dir(link), filepath.Join(root, "proj")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}

	// The control: the argv the old render emitted for this source, handed
	// to the real bwrap.
	argv := []string{"--die-with-parent", "--ro-bind", "/", "/", "--dev-bind", "/dev", "/dev", "--unshare-user", "--bind", link, bwrapTmpMount}
	out, err := exec.Command("bwrap", append(argv, "sh", "-c", "echo seat-wrote > /tmp/planted")...).CombinedOutput()
	if err != nil {
		t.Fatalf("bwrap %v: %v\n%s", argv, err, out)
	}
	b, rerr := os.ReadFile(filepath.Join(victim, "planted"))
	if rerr != nil || !strings.Contains(string(b), "seat-wrote") {
		t.Fatalf("NOTHING MEASURED: this bwrap did not follow the source link (%v, %q), so the refusal below guards nothing on this host", rerr, b)
	}

	// The fix: this package's render will not produce that argv.
	if got, prep, err := BwrapArgs(filepath.Join(root, "proj"), nil, SeatbeltCarveOut{Tmp: link}); err == nil {
		t.Errorf("the render emitted %q for a source link that bwrap was just measured following into %s (ADR 0100 D1)\n prep: %v", got, victim, prep)
	}
}
