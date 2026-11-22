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
	// SeatWallRoot COMPOSES FROM THE RESOLVED HOME on purpose (ADR 0100
	// D2(a), stated at the function), so every expectation about its answer
	// resolves the home too. The literal above is a real directory's parent
	// on darwin: `/home` is an autofs link to `/System/Volumes/Data/home`, so
	// the raw join named a path production would never answer and this arm
	// was red on every macos leg while being green here, where `/home` is an
	// ordinary directory and absResolve is the identity (ranger-base-zbmxc,
	// MEASURED on macos arm 1 at b2509fba).
	rhome := absResolve(home)
	a, b := SeatWallDir(home, "/r1/wt/devops-x"), SeatWallDir(home, "/r2/wt/devops-x")
	if a == b {
		t.Errorf("SeatWallDir names one wall dir for two session dirs: %s", a)
	}
	root := SeatWallRoot(home)
	if root != filepath.Join(rhome, ".cache", "posse-wall") {
		t.Errorf("SeatWallRoot = %q, want <resolved home>/.cache/posse-wall", root)
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
	// The cache root as SeatCacheDir composes it — from the RAW home, which
	// is the one place the two functions differ — and the containment asked
	// through underDir, which resolves both sides: the subject here is the
	// two DIRECTORIES and not either spelling of them, so a home behind a
	// link must not be able to answer "disjoint" by spelling alone.
	cacheRoot := filepath.Join(home, seatCacheRootRel)
	if underDir(cacheRoot, root) || underDir(root, cacheRoot) {
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
	if len(binds) != len(claudeSessionStateDirNames)+2 {
		t.Fatalf("claudePrivateSessionState rendered %d binds, want one per member plus the two file binds, history.jsonl and .last-cleanup — the loop below would measure a subset", len(binds))
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

// Pin (b) at the SEAT render: a PID `writable:` extra is the shape an
// operator would actually type, and the render refuses it with the same
// sentence — named outright, and spelled as a LINK to the wall root, which
// is the spelling only the resolved grant gives away. The control is the
// identical render with no extra at all.
func TestQAWallRootGrantIsRefusedByTheSeatRender(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	cache, wallRoot, link := wrgHome(t, f.home)
	if _, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates); err != nil {
		t.Fatalf("control: this seat's render is already refused without any extra (%v), so the arms below measure nothing", err)
	}
	for _, extra := range []string{cache, link} {
		f.ag.Writable = []string{extra}
		_, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
		if err == nil || !strings.Contains(err.Error(), wallRoot) {
			t.Errorf("a `writable:` extra of %s rendered (err %v) — the seat would hold a component of every seat's bind sources (ADR 0100 D2(a))", extra, err)
		}
	}
}

// Pin: D2(a) BOTH DIRECTIONS (ranger-base-dw2du). "At or above the wall
// root" was the whole rule while the attack in view was a grant that holds a
// COMPONENT of a source. A grant INSIDE the root is neither at nor above it,
// and it is the same hole from below: a `writable:` extra naming another
// seat's wall dir, or one directory inside it, is a write grant on that
// seat's private bind SOURCE — and for the private members a seat sources
// before every command it runs, a sibling that can write there runs code in
// it. No rename, no race, no leftover needed.
//
// Spelled three ways, because an operator and an attacker spell it
// differently: another seat's wall dir, a directory two levels inside one,
// and a path of the seat's own that is a LINK into the root — the last
// caught only because the grant side is compared RESOLVED, which is the one
// re-resolution BwrapArgs makes.
//
// MUTATION RUN 2026-11-21: dropping the `insideDir(root, gr)` arm of
// bwrapRefuseWallRootGrant reds all three rows and nothing else; the arms
// above it (at or above the root) stay green, which is what says the two
// directions are two rules and not one.
func TestQAWallRootGrantIsRefusedInsideTheWallRootToo(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	root := wallUngrantedDir(t)
	home, proj := filepath.Join(root, "home"), filepath.Join(root, "proj")
	for _, d := range []string{home, proj} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	_, wallRoot, _ := wrgHome(t, home)

	// A sibling seat's wall dir, keyed for a session dir that is not this
	// render's — which is the whole point: the root holds every seat's.
	sibling := SeatWallDir(home, filepath.Join(root, "another-seat"))
	inside := filepath.Join(sibling, "claude", "shell-snapshots")
	linkIn := filepath.Join(proj, "scratch-link")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, linkIn); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, grant string }{
		{"another seat's wall dir", sibling},
		{"a private source directory inside one", inside},
		{"a path of this seat's own that LINKS into the root", linkIn},
	} {
		argv, prep, err := BwrapArgs(proj, []string{c.grant}, SeatbeltCarveOut{})
		switch {
		case err == nil:
			t.Errorf("a grant of %s (%s) rendered: it lies inside the wall root %s, where the bind SOURCES of every walled seat on this box live, so this seat writes another seat's private source directly (ADR 0100 D2(a))\n argv: %q", c.name, c.grant, wallRoot, argv)
		case !strings.Contains(err.Error(), wallRoot):
			t.Errorf("a grant of %s was refused with %q, which does not name the wall root — the refusal is the operator's only instruction for what to narrow", c.name, err)
		}
		if len(prep) != 0 {
			t.Errorf("a grant of %s came back with prep %v for the uncaged launcher to run", c.name, prep)
		}
	}
}

// Pin, the NEGATIVE case of the arm above, and it is what says the widening
// reds nothing legitimate: a seat's own private DESTINATIONS are grants —
// `<cfg>` and the rest — but its own private SOURCES are not granted by any
// render, which is the whole of D1. So no shipped render has a grant under
// the wall root to lose, and the sources that ARE under it are reached
// through mounts and not through the write set.
//
// Both halves are measured over one render, because either alone is vacuous:
// a render with no grant inside the root and no source under the wall dir
// would pass the first assertion while measuring nothing.
func TestQAWallRootGrantTheSeatsOwnSourcesAreUnderItAndItsGrantsAreNot(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	wallRoot := bwrapWallRoot()
	for _, name := range []string{"claude", "codex", "grok"} {
		r, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err != nil {
			t.Fatalf("%s: the render is refused (%v) — if that is D2(a)'s own refusal, this instance has a grant inside the wall root and the arm below cannot measure it", name, err)
		}
		for _, g := range r.Set {
			if insideDir(wallRoot, absResolve(g)) {
				t.Errorf("the %s render grants %s, which lies inside the wall root %s: that is a grant on some seat's private bind source, and D2(a) refuses it — narrow the producer, do not exempt the render (ADR 0100 D2(a))", name, g, wallRoot)
			}
		}
		var under int
		for _, m := range bwrapMounts(r.Argv) {
			if m.Src != "" && m.Src != m.Dst && insideDir(wallRoot, absResolve(m.Src)) {
				under++
			}
		}
		if under == 0 {
			t.Errorf("CONTROL: the %s render has no bind source under the wall root at all, so the rows above measured a render with nothing to protect:\n %s", name, bwJoin(r.Argv))
		}
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

// ─── two arms that were load-bearing and unpinned (ranger-base-lng6p) ────────

// THE WALL ROOT IS COMPOSED FROM THE **RESOLVED** HOME, and until this pin
// nothing measured the absResolve in SeatWallRoot. Every D2(a) arm above
// sets HOME to a real directory under wallUngrantedDir, where absResolve is
// the identity, so all four hold with the resolution dropped — MEASURED
// 2026-11-15 by mutation (`filepath.Join(absResolve(home), seatWallRootRel)`
// -> `filepath.Join(home, seatWallRootRel)`): every pin in this file, and
// every test naming SeatWallDir or BwrapArgs, stayed green.
//
// A HOME REACHED THROUGH A SYMLINK is the case that separates them, and it
// separates them in the direction that matters. bwrapRefuseWallRootGrant
// resolves the GRANT side on purpose, because only the resolved spelling
// says where a bind would land; with the root composed from the unresolved
// home the two sides are then spelled in different trees, insideDir answers
// false for every grant, and D2(a) refuses NOTHING on such a box — silently,
// and box-wide, since the root holds every seat's wall dir. The grant and
// the root have to be resolved together or not at all.
//
// Not reachable from this instance's own home today (`/home/posse` resolves
// to itself, MEASURED 2026-11-15); reachable wherever a home is reached
// through a link, which is every darwin box whose home sits under a
// symlinked volume and any operator who moved one.
func TestQAWallRootGrantRefusesAHomeSpelledThroughASymlink(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	root := wallUngrantedDir(t)
	realHome, proj := filepath.Join(root, "real-home"), filepath.Join(root, "proj")
	link := filepath.Join(root, "home-link")
	for _, d := range []string{filepath.Join(realHome, ".cache"), proj} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(realHome, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", link)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: this render's wall-root refusal stands down for a home under " + bwrapTmpMount)
	}
	// The fixture is only worth anything while the two spellings differ.
	wallRoot := SeatWallRoot(link)
	if wallRoot == filepath.Join(link, ".cache", "posse-wall") {
		t.Fatalf("NOTHING MEASURED: SeatWallRoot(%s) = %s, the unresolved spelling, so this fixture cannot separate the two sides", link, wallRoot)
	}
	// The control: this render is clean with no extra at all, so the arms
	// below are about the grant and not about the fixture.
	if _, _, err := BwrapArgs(proj, nil, SeatbeltCarveOut{}); err != nil {
		t.Fatalf("control: the render is already refused with no grant at all (%v)", err)
	}
	for _, c := range []struct{ name, grant string }{
		{"the home as HOME spells it, through the link", link},
		{"~/.cache through the link", filepath.Join(link, ".cache")},
		{"the home resolved", realHome},
	} {
		argv, prep, err := BwrapArgs(proj, []string{c.grant}, SeatbeltCarveOut{})
		switch {
		case err == nil:
			t.Errorf("a grant of %s (%s) rendered: it contains the wall root %s, where every walled seat's bind sources lie (ADR 0100 D2(a))\n argv: %q", c.name, c.grant, wallRoot, argv)
		case !strings.Contains(err.Error(), wallRoot):
			t.Errorf("a grant of %s was refused with %q, which does not name the wall root — the refusal is the operator's only instruction for what to narrow", c.name, err)
		}
		if len(prep) != 0 {
			t.Errorf("a grant of %s came back with prep %v for the uncaged launcher to run", c.name, prep)
		}
	}
}

// THE WALL DIR GOES WITH THE RETIRED TREE (ADR 0100 D1's retire order), and
// until this pin nothing measured that it goes at all — nor that the seat
// CACHE dir does, which has been unpinned since ranger-base-olr0g put it
// there. MEASURED 2026-11-15 by mutation (RemoveSessionTree's
// `os.RemoveAll(seatWall)` replaced by `_ = seatWall`): nothing in this
// package reds.
//
// WHY IT WAS UNREACHABLE BEFORE: RemoveSessionTree composes both dirs from
// os.UserHomeDir(), so a fixture that leaves HOME alone names a path under
// the OPERATOR's real `~/.cache` keyed by a hash of a temp dir — a path that
// never existed, whose RemoveAll is a no-op either way. Setting HOME is the
// whole of what makes the observable visible, and it is why none of the
// eleven existing RemoveSessionTree fixtures sees it.
//
// WHAT THIS PIN DOES NOT COVER, said rather than implied: the ORDER D1 asks
// for — import the seat's objects, then `worktree remove`, then this. A
// mutant that removes the wall dir EARLY as well as late survives this pin
// and every other in the package. The order is inert until the seat's own
// object store moves into the wall dir (ranger-base-ltmyh), which is when a
// pin on it can observe something; writing it now is what the close that
// landed this asked for.
func TestQAWallSeatWallDirGoesWithTheRetiredTree(t *testing.T) {
	// No t.Parallel: this sets HOME, which RemoveSessionTree composes both
	// per-seat directories from.
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	wall, cache := SeatWallDir(home, tr.Path), SeatCacheDir(home, tr.Path)
	if wall == "" || cache == "" {
		t.Fatalf("the fixture names no per-seat dirs (wall %q, cache %q), so this measures nothing", wall, cache)
	}
	// What a live seat leaves behind: its private /tmp, with the one file
	// the bind exists to keep on the host, and its build cache.
	signal := filepath.Join(wall, "tmp", "posse-test-signal.log")
	if err := os.MkdirAll(filepath.Dir(signal), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signal, []byte("the seat wrote this\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cache, "go-build"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The control: both are on disk before the retire, so an absence after
	// it is the removal and not the fixture.
	for _, d := range []string{wall, cache} {
		if _, err := os.Stat(d); err != nil {
			t.Fatalf("control: %s is not there before the retire (%v)", d, err)
		}
	}
	if err := RemoveSessionTree(tr, false); err != nil {
		t.Fatalf("the fixture tree must retire cleanly, or nothing below ran: %v", err)
	}
	for _, c := range []struct{ name, dir string }{
		{"the seat's wall dir", wall},
		{"the seat's cache dir", cache},
	} {
		if _, err := os.Stat(c.dir); !os.IsNotExist(err) {
			t.Errorf("%s %s survived the retire (stat err %v): the tree it belongs to is gone, nothing names it any more, and its private /tmp still holds whatever the seat wrote there (ADR 0100 D1, ranger-base-olr0g)", c.name, c.dir, err)
		}
	}
}

// THE STAND-DOWN IS THE ONE THING NO D2(a) PIN COULD FAIL ON, and that is
// what this pin is for. Every arm above reads the rule through a guard that
// SKIPS when `bwrapWallRoot()` answers "" — correctly, because on a box whose
// home is under the shared /tmp the refusal is inert by design (ADR 0100
// Consequences, "RESIDUAL, a home under the shared /tmp"). The cost is that
// the guard cannot tell "inert by design" from "inert because the rule is
// gone": MEASURED 2026-11-15 by mutation (`bwrapWallRoot`'s
// `return SeatWallRoot(home)` -> `return ""`), D2(a) is disabled box-wide and
// NOT ONE test in this package reds — all four arms, and both arms added
// under ranger-base-lng6p, skip quietly and `go test` exits 0 over a seat
// wall that refuses nothing at all.
//
// So this arm asserts the PREMISE the others skip on, and it is the only one
// here that must not skip on it: for a home nothing grants, the stand-down is
// not in force, and `bwrapWallRoot` names the same root `SeatWallRoot` does.
// wallUngrantedDir has already skipped if THIS host's /var/tmp is itself
// inside a blanket temp grant, which is the one honest reason for an
// ungranted home to stand the rule down — so past that helper, an empty
// answer is the rule being gone and nothing else.
//
// WHAT IT DOES NOT COVER, said rather than implied: it pins that the rule is
// LIVE for such a home, not which grants it then refuses — that is what the
// four arms above are, and they are reached through the same guard. This pin
// is what makes their skip a measurement instead of a silence.
func TestQAWallRootStandDownIsNotInForceForAnUngrantedHome(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	home := filepath.Join(wallUngrantedDir(t), "home")
	if err := os.MkdirAll(filepath.Join(home, ".cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	got := bwrapWallRoot()
	if got == "" {
		t.Fatalf("bwrapWallRoot() is empty for the home %s, which lies under no blanket temp grant: ADR 0100 D2(a) then refuses NO grant on this box, every other arm in this file skips rather than reds, and every walled seat's bind sources sit under a root any write grant may contain", home)
	}
	if want := SeatWallRoot(home); got != want {
		t.Errorf("bwrapWallRoot() = %s, want %s — the root a grant is judged against and the root the sources are composed under must be one directory, or D2(a) compares two trees and refuses nothing (ADR 0100 D2(a))", got, want)
	}
}
