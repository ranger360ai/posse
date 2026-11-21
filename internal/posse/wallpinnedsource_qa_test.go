//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ADR 0100 D2(b): a path is PINNED in a render when every component
// of it that lies strictly inside a write grant is itself a mount
// destination of that render — and EVERY bind source that differs from its
// destination must be pinned, in all three renders this package makes.
//
// WHAT THE RULE IS FOR. bwrap resolves a bind SOURCE at mount time and
// follows links; the render's component walk (bwrapSrcWalk) judged the same
// source an instant earlier. Anything that can write a COMPONENT of the
// source between the two redirects the bind, and a leftover of the seat
// needs no timing at all — it toggles the component in a loop. D1 answers
// the class by moving every source under the per-seat wall dir; this
// predicate is what says so of each RENDER rather than of the design, and it
// is the thing that reds if a source is put back.
//
// THE FIXTURE HOMES ARE UNDER wallUngrantedDir, for the reason
// wallseatwall_qa_test.go's header gives at length: `/tmp` is granted
// unconditionally, so a home under it puts every source inside a grant, and
// the rule stands down there on purpose (bwrapRefuseUnpinnedSource).
//
//	go test ./internal/posse -run 'WallPinned' -v

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ─── The predicate itself ───────────────────────────────────────────────────

// The shapes ADR 0100 D2(b) names, each as a row: what is loose, what is
// pinned, and why. The rows about the seat store are the real paths' shape,
// written short.
func TestQAWallPinnedPredicateAnswersTheADRsRows(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		p      string
		grants []string
		dsts   []string
		want   string // the loose component, "" for pinned
	}{
		{
			name: "inside no grant at all — every source under the wall dir",
			p:    "/h/.cache/posse-wall/k/seat-objects", grants: []string{"/h/wt", "/h/.cache/posse-seat/k"},
			dsts: []string{"/h/wt"},
		},
		{
			name: "the old seat store: inside the per-worktree git dir, which the seat is granted",
			p:    "/r/.git/worktrees/s/seat-objects", grants: []string{"/r/.git/worktrees/s"},
			dsts: []string{"/r/.git/worktrees/s", "/r/.git/objects"},
			want: "/r/.git/worktrees/s/seat-objects",
		},
		{
			name: "the store's SIDE bind: its source is inside a whole-<common> grant and IS the store's own mount",
			p:    "/r/.git/objects", grants: []string{"/r/.git"},
			dsts: []string{"/r/.git", "/r/.git/objects"},
		},
		{
			name: "the same source with the store's mount gone: nothing pins it",
			p:    "/r/.git/objects", grants: []string{"/r/.git"},
			dsts: []string{"/r/.git"},
			want: "/r/.git/objects",
		},
		{
			name: "a grant AT the path is not strictly above it — its parent is outside",
			p:    "/h/run/tmp", grants: []string{"/h/run/tmp"},
			dsts: []string{"/h/run/tmp"},
		},
		{
			name: "the judge's /tmp source: inside the run-dir grant, and a mount of its own pins it",
			p:    "/h/run/tmp", grants: []string{"/h/run"},
			dsts: []string{"/h/run", "/h/run/tmp"},
		},
		{
			name: "the same with no in-place bind",
			p:    "/h/run/tmp", grants: []string{"/h/run"},
			dsts: []string{"/h/run"},
			want: "/h/run/tmp",
		},
		{
			name: "the SHALLOWEST loose component is the one named, because pinning it pins the rest",
			p:    "/g/a/b/c", grants: []string{"/g"},
			dsts: []string{"/g"},
			want: "/g/a",
		},
		{
			name: "/dev/null over a masked file: nothing grants /dev",
			p:    "/dev/null", grants: []string{"/h/wt"},
			dsts: []string{"/dev"},
		},
		// The in-place rows (ranger-base-dw2du). A grant is bound `--bind p
		// p`, so its own name is always a mount of the render and the only
		// question left is about its ANCESTORS — which is why a nest has to
		// be two deep before anything is loose.
		{
			name: "a grant ONE component inside another: its own mount is what pins it",
			p:    "/g/a", grants: []string{"/g", "/g/a"},
			dsts: []string{"/g", "/g/a"},
		},
		{
			name: "a grant TWO components inside another, nothing mounted between",
			p:    "/g/a/b", grants: []string{"/g", "/g/a/b"},
			dsts: []string{"/g", "/g/a/b"},
			want: "/g/a",
		},
		{
			name: "the same nest with the component between bound in place — the prescribed fix",
			p:    "/g/a/b", grants: []string{"/g", "/g/a/b"},
			dsts: []string{"/g", "/g/a", "/g/a/b"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			loose, ok := bwrapPinned(c.p, c.grants, c.dsts)
			if c.want == "" {
				if !ok {
					t.Fatalf("bwrapPinned(%s) says %s is loose, and ADR 0100 D2(b) says it is pinned\n grants %v\n mounts %v", c.p, loose, c.grants, c.dsts)
				}
				return
			}
			if ok {
				t.Fatalf("bwrapPinned(%s) says PINNED; %s lies strictly inside a write grant and is no mount of the render, so anything in that namespace renames it and the next bind follows the link\n grants %v\n mounts %v", c.p, c.want, c.grants, c.dsts)
			}
			if loose != c.want {
				t.Errorf("bwrapPinned(%s) names %s loose, want %s", c.p, loose, c.want)
			}
		})
	}
}

// The grant side is compared BOTH as written and resolved, so a home behind
// a symlink cannot narrow a rule whose job is to refuse. The component side
// stays composed: resolving it would follow a link a seat planted and judge
// the link's TARGET, which is D2(a)'s insideDir/underDir distinction one
// level down.
func TestQAWallPinnedComparesTheGrantResolvedAndTheComponentComposed(t *testing.T) {
	t.Parallel()
	root := absResolve(t.TempDir())
	real, link := filepath.Join(root, "real"), filepath.Join(root, "link")
	if err := os.MkdirAll(filepath.Join(real, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// The grant is spelled through the link; the source is composed from the
	// real name. Resolving the grant is what sees that it contains it.
	if _, ok := bwrapPinned(filepath.Join(real, "inner", "src"), []string{link}, nil); ok {
		t.Errorf("a grant spelled %s does not reach a source composed as %s — a seat granted the link can write every component of it all the same", link, real)
	}
	// And the other way round: a grant on the real name, a source composed
	// through the link. Resolving the grant cannot help here, and resolving
	// the SOURCE must not be how it is answered — so this is the stated
	// residual, not a claim. What it must not do is report PINNED while
	// saying the component is somewhere it is not.
	if loose, ok := bwrapPinned(filepath.Join(link, "inner", "src"), []string{real}, nil); !ok && !strings.HasPrefix(loose, link) {
		t.Errorf("a loose component is reported as %s, which is not a component of the path as spelled", loose)
	}
}

// ─── The rule over the renders ──────────────────────────────────────────────

// wpsSeatTree is a seat fixture whose home is outside every blanket temp
// grant and whose session dir is a real linked worktree, which is the only
// shape where the seat store renders at all.
type wpsSeat struct {
	wrcFixture
	l seatStoreLayout
}

func wpsSeatTree(t *testing.T) wpsSeat {
	t.Helper()
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount + " (bwrapWallRoot)")
	}
	repo := filepath.Join(filepath.Dir(f.dir), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "root")
	mustGit(t, repo, "worktree", "add", "-q", "--detach", f.dir)
	l, ok := seatStoreFor(f.dir)
	if !ok {
		t.Fatal("CONTROL: the fixture's dir is not a linked worktree, so no store renders and nothing below is measured")
	}
	return wpsSeat{wrcFixture: f, l: l}
}

// Pin: the seat render's own sources are all pinned, and the two the ADR
// argues about by name are measured one at a time — the store's source
// (under the wall dir, inside no grant) and the store's SIDE bind, whose
// source `<common>/objects` IS inside a grant whenever something grants the
// common dir whole and is pinned by being the store's own mount destination.
//
// MUTATION RUN: moving the store back under the per-worktree git dir
// (seatStoreFrom's Store) reds the refusal arm below, which is the same
// render with the legacy source handed to it.
func TestQAWallPinnedSeatRenderPinsEverySource(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	ms := bwrapMounts(r.Argv)
	dsts := bwrapMountDsts(ms)
	grants := shallowestFirst(r.Set)
	var differing int
	for _, m := range ms {
		if m.Src == "" || m.Src == m.Dst {
			continue
		}
		differing++
		if loose, ok := bwrapPinned(m.Src, grants, dsts); !ok {
			t.Errorf("the seat render binds %s over %s and %s is loose — a leftover of this seat renames it and the next launch binds what it points at (ADR 0100 D2(b))", m.Src, m.Dst, loose)
		}
	}
	if differing < 2 {
		t.Fatalf("CONTROL: this render has %d sources that differ from their destination, so the loop above measured nothing:\n  %s", differing, bwJoin(r.Argv))
	}

	// The store's source, by name: under the wall dir and inside no grant.
	if loose, ok := bwrapPinned(f.l.Store, grants, dsts); !ok {
		t.Errorf("the seat store's source %s is loose at %s", f.l.Store, loose)
	}
	if writeGranted(grants, f.l.Store) {
		t.Errorf("the write set grants the store's source %s, so it is pinned by nothing but this render's own mount:\n %v", f.l.Store, grants)
	}

	// The SIDE bind's source, explicitly, under a grant that covers
	// `<common>` WHOLE — the store-of-record shape the ADR names. It passes
	// because `<common>/objects` is the store's own mount destination, and
	// the control is the same question with that mount taken away.
	common := filepath.Dir(f.l.Objects)
	wide := append(append([]string{}, grants...), common)
	if loose, ok := bwrapPinned(f.l.Objects, wide, dsts); !ok {
		t.Errorf("the side bind's source %s is loose at %s under a whole-<common> grant, and it should be pinned by the store's own mount over that name (ADR 0100 D2(b))", f.l.Objects, loose)
	}
	var without []string
	for _, d := range dsts {
		if absResolve(d) != f.l.Objects {
			without = append(without, d)
		}
	}
	if _, ok := bwrapPinned(f.l.Objects, wide, without); ok {
		t.Error("CONTROL: the side bind's source reads as pinned with the store's mount removed, so the row above measures the grant and not the mount")
	}
}

// Pin: the render REFUSES a store source put back under the per-worktree git
// dir — the ADR 0099 placement — and the refusal names the loose component
// and the wall root to move it to. The control is the same render with the
// shipped source, which must pass.
func TestQAWallPinnedRenderRefusesAStorePutBackUnderTheGitDir(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatalf("control: the shipped render is already refused (%v), so the arm below measures nothing", err)
	}
	if !writeGranted(r.Set, f.l.Own) {
		t.Skipf("NOTHING MEASURED: this seat is not granted its per-worktree git dir %s, so the legacy source lies inside no grant here", f.l.Own)
	}
	legacy := []PrivateBind{
		{Src: f.l.Objects, Dst: f.l.Base, SrcRoot: filepath.Dir(f.l.Objects), ReadOnly: true},
		{Src: f.l.Legacy, Dst: f.l.Objects, SrcRoot: f.l.Own},
	}
	if err := os.MkdirAll(f.l.Legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	carve := r.Carve
	carve.Private = legacy
	argv, _, err := BwrapArgs(f.dir, r.Set, carve, r.Keep...)
	switch {
	case err == nil:
		t.Fatalf("the render bound %s over %s: that source sits inside the seat's own write grant %s, so anything the seat leaves behind points it at the SHARED object store and the next launch mounts that read-write (ADR 0099 D1's whole subject, ADR 0100 D2(b))\n argv: %q", f.l.Legacy, f.l.Objects, f.l.Own, argv)
	case !strings.Contains(err.Error(), f.l.Legacy):
		t.Errorf("the refusal does not name the loose component %s: %v", f.l.Legacy, err)
	case !strings.Contains(err.Error(), bwrapWallRoot()):
		t.Errorf("the refusal does not name the wall root to move it to, which is the only instruction it carries: %v", err)
	}
}

// Pin: the rule is asked of ALL THREE of this package's renders, and the two
// that are not a seat are the ones whose private /tmp lies inside their own
// run-dir grant — so each must carry the in-place bind that pins it. ADR
// 0100 said those two passed D2(b) as they stood; MEASURED 2026-11-15 while
// building it, they did not, and this is the fix the ADR itself prescribes
// for a miss (an in-place bind, never an exemption).
//
// MUTATION RUN: dropping BwrapArgs' step 2a-bis reds the judge arm, and
// dropping wallVenueArgv's in-place bind reds the venue arm.
func TestQAWallPinnedJudgeAndVenueSourcesArePinnedToo(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := newLandedCageFixture(t)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount)
	}
	if WallMechanism() != WallBwrap {
		t.Skip("NOTHING MEASURED: D2(b) is the bwrap render's; this host's mechanism is " + WallMechanism())
	}
	child := filepath.Join(f.run, "judge")
	writable := []string{f.run}
	for _, kind := range []landedCageKind{landedCageJudge, landedCageVenue} {
		cage, err := boxLandedCage(&App{}, kind, child, f.tree, writable, landedRunEnv(f.run))
		if err != nil {
			t.Fatalf("the %s cage is refused (%v) — if that refusal is this rule's, the render needs the in-place bind and not an exemption", kind, err)
		}
		ms := bwrapMounts(cage.Argv)
		dsts := bwrapMountDsts(ms)
		var differing int
		for _, m := range ms {
			if m.Src == "" || m.Src == m.Dst {
				continue
			}
			differing++
			if loose, ok := bwrapPinned(m.Src, writable, dsts); !ok {
				t.Errorf("the %s binds %s over %s and %s lies inside its own run-dir grant with no mount on it (ADR 0100 D2(b))\n argv: %s", kind, m.Src, m.Dst, loose, bwJoin(cage.Argv))
			}
		}
		if differing == 0 {
			t.Errorf("CONTROL: the %s render has no source that differs from its destination, so this arm measured nothing:\n %s", kind, bwJoin(cage.Argv))
		}
	}
}

// ─── D2(b) over the IN-PLACE binds too (ranger-base-dw2du) ──────────────────

// Pin AND CENSUS: every IN-PLACE bind source is pinned as well, in every
// render this package makes. The first text of D2(b) asked this only of the
// sources that DIFFER from their destination, which left shape 1's `--bind p
// p` for each write grant — and every in-place `--ro-bind` — judged by
// neither arm, since the destination arm skips the same equality. They are
// not safe for being in place: bwrap resolves the same string again at
// mount(2), so a component of a grant path a walled seat can rename is
// togglable between a directory and a link to the wall root, and the next
// launch mounts that root read-write at the grant's own name. The attacker
// is the seat's own leftover, so it needs no sibling and no timing.
//
// IT IS THE ANSWER TO THE ONE THING THE RE-READ LEFT ASSUMED: whether any
// render today has a grant nested two or more components inside another with
// a non-mount component between them. MEASURED 2026-11-21 by running this:
// no — 150 in-place binds over the five argvs (claude 55, codex 31, grok 50,
// judge 7, venue 7), every one pinned, read at da79a784. The COUNT moves with
// every landing that adds or removes a mount, and did while this bead was in
// flight (it was 160 three landings earlier, over the same five renders); what
// does not move is that none of them is loose, which is why this is a pin and
// not a paragraph about today's renders.
//
// MUTATION RUN 2026-11-21: putting the `m.Src == m.Dst` skip back into
// bwrapRefuseUnpinnedSource reds the refusal arm below and nothing else —
// this census passes either way, which is exactly what it is for.
func TestQAWallPinnedSourceEveryRenderPinsItsInPlaceBindsToo(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	if err := os.MkdirAll(ClaudeConfigDirIn(f.home), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex", "grok"} {
		r, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err != nil {
			t.Fatalf("%s: the render is refused (%v) — if that is this rule's refusal, the render needs an in-place bind of the loose component and not an exemption", name, err)
		}
		n := wpsCountPinnedInPlace(t, name, r.Argv, shallowestFirst(r.Set))
		// Logged and not only counted, so `-v` re-derives the number the
		// head comment carries rather than leaving it to be retyped from a
		// run nobody can repeat (ranger-base-dot6w).
		t.Logf("CENSUS: the %s render makes %d in-place binds", name, n)
		if n < 2 {
			t.Errorf("CONTROL: the %s render makes %d in-place binds, so this arm measured nothing:\n %s", name, n, bwJoin(r.Argv))
		}
	}
	g := newLandedCageFixture(t)
	if WallMechanism() != WallBwrap {
		t.Skip("the judge and venue arms want this host's bwrap render; its mechanism is " + WallMechanism())
	}
	writable := []string{g.run}
	for _, kind := range []landedCageKind{landedCageJudge, landedCageVenue} {
		cage, err := boxLandedCage(&App{}, kind, filepath.Join(g.run, "judge"), g.tree, writable, landedRunEnv(g.run))
		if err != nil {
			t.Fatalf("%s: the render is refused (%v) — same reading as the seat arms above", kind, err)
		}
		n := wpsCountPinnedInPlace(t, kind.String(), cage.Argv, writable)
		t.Logf("CENSUS: the %s render makes %d in-place binds", kind, n)
		if n == 0 {
			t.Errorf("CONTROL: the %s render makes no in-place bind at all:\n %s", kind, bwJoin(cage.Argv))
		}
	}
}

// wpsCountPinnedInPlace is the census over one argv: it reds every in-place
// bind whose path has a loose component and answers how many it judged, so a
// caller can say its arm measured something.
func wpsCountPinnedInPlace(t *testing.T, who string, argv, grants []string) int {
	t.Helper()
	ms := bwrapMounts(argv)
	dsts := bwrapMountDsts(ms)
	var n int
	for _, m := range ms {
		if m.Src == "" || m.Src != m.Dst {
			continue
		}
		n++
		if loose, ok := bwrapPinned(m.Src, grants, dsts); !ok {
			t.Errorf("the %s render binds %s in place and %s lies strictly inside a write grant with no mount on it — a leftover of this seat renames it and bwrap resolves this grant's own path to what it points at, read-write, at the next launch (ADR 0100 D2(b))\n argv: %s", who, m.Src, loose, bwJoin(argv))
		}
	}
	return n
}

// Pin: a render carrying a TWO-DEEP nest BINDS THE COMPONENT BETWEEN IN
// PLACE, so the nest renders and is pinned — and the refusal is still what a
// render without that bind gets, named by the loose component.
//
// THIS TEST READ THE OTHER WAY ROUND UNTIL ranger-base-dot6w. The widening
// above (ranger-base-dw2du) judged the in-place binds and nothing emitted
// the pin, so a two-deep nest was refused outright and this pin asserted the
// refusal. The shape then arrived: a store of record inside the same repo
// grants `<common>` whole, which puts `<common>/worktrees/<seat>` two
// components inside that grant with `worktrees` loose between them, and the
// live seat-store pin's second arm builds exactly it
// (TestLiveBwrapSeatStoreRefusesRewritingASharedObject, red in ADR 0097 D1's
// venue). A render refused is a seat that does not launch, and the fix ADR
// 0100 prescribes for a miss is not an exemption but the bind: shape 1 now
// emits an in-place bind of every directory between a grant and the grant
// above it (bwrapGrantMounts). The refusal it replaces is unreachable from
// this render and is NOT dead — the judge and the venue add mounts after
// BwrapArgs and are judged again there (boxLandedCage), so the second arm
// below is the shipped predicate over a render with the pin taken out.
//
// MUTATION RUN 2026-11-21: dropping the `m.Pin` arm of BwrapArgs' step 2 reds
// the first arm; restoring the `m.Src == m.Dst` skip in
// bwrapRefuseUnpinnedSource reds the second.
func TestQAWallPinnedSourceBindsATwoDeepNestsLooseComponentInPlace(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	root := wallUngrantedDir(t)
	home, proj := filepath.Join(root, "home"), filepath.Join(root, "proj")
	outer := filepath.Join(proj, "g")
	mid, inner := filepath.Join(outer, "a"), filepath.Join(outer, "a", "b")
	for _, d := range []string{home, proj, inner} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount + " (bwrapWallRoot)")
	}

	// CONTROL, and it is two controls in one: the outer grant alone, and the
	// nest one component deep whose inner grant is its own mount. Both must
	// render, or the refusal below is a refusal of everything.
	for _, c := range []struct {
		name   string
		grants []string
	}{
		{"the outer grant alone", []string{outer}},
		{"a grant one component inside it", []string{outer, mid}},
		{"the two-deep nest with the component between granted too", []string{outer, mid, inner}},
	} {
		if _, _, err := BwrapArgs(proj, c.grants, SeatbeltCarveOut{}); err != nil {
			t.Errorf("CONTROL: %s is refused (%v) — every component of every grant there is either a mount of this render or outside every grant, so nothing is loose", c.name, err)
		}
	}

	// The shipped render: the component between is bound IN PLACE, after the
	// grant that contains it and before the grant it pins, so each mount
	// lands on top of the one that contains it.
	argv, _, err := BwrapArgs(proj, []string{outer, inner}, SeatbeltCarveOut{})
	if err != nil {
		t.Fatalf("a render granting %s inside %s is refused (%v) — the fix ADR 0100 prescribes for a loose component is an in-place bind of it, emitted by the render that already grants it, and never an exemption", inner, outer, err)
	}
	line := bwJoin(argv)
	if !strings.Contains(line, "--bind "+mid+" "+mid+" ") {
		t.Fatalf("the render grants %s two components inside %s and binds nothing over %s, so a leftover of this seat renames %s and bwrap resolves the grant's own path to whatever the plant points at — the wall root, read-write, at the next launch (ADR 0100 D2(b))\n %s", inner, outer, mid, mid, line)
	}
	if i, j, k := strings.Index(line, "--bind "+outer+" "), strings.Index(line, "--bind "+mid+" "), strings.Index(line, "--bind "+inner+" "); !(i < j && j < k) {
		t.Errorf("shape 1's mounts are not shallowest-first (%d, %d, %d), so a mount lands under one that contains it (ADR 0080 D17)\n %s", i, j, k, line)
	}

	// The same render with that one bind taken out: refused, by the name of
	// the loose component and with the fix in the message.
	var without []string
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--bind" && i+2 < len(argv) && argv[i+1] == mid && argv[i+2] == mid {
			i += 2
			continue
		}
		without = append(without, argv[i])
	}
	if len(without) == len(argv) {
		t.Fatalf("CONTROL: there was no in-place bind of %s to drop, so the arm below measures nothing", mid)
	}
	err = bwrapRefuseUnpinnedSource(without, []string{outer, inner})
	switch {
	case err == nil:
		t.Errorf("a render granting %s inside %s with nothing bound over %s is not refused — that is the whole of what the in-place bind above buys (ADR 0100 D2(b))", inner, outer, mid)
	case !strings.Contains(err.Error(), mid):
		t.Errorf("the refusal does not name the loose component %s, which is the only thing the operator can act on: %v", mid, err)
	case !strings.Contains(err.Error(), "in place"):
		t.Errorf("the refusal does not name the fix (bind the loose component in place): %v", err)
	}
}

// Pin: A LINK AT A GRANT'S OR A PIN'S OWN NAME REFUSES THE RENDER, in both
// of step 2's arms, and a DANGLING one asks the launcher for nothing
// (bwrapGrantSrcStat, ADR 0100 D2(b) as amended by ranger-base-gi22r).
//
// WHY THE IN-PLACE BIND ABOVE WAS NOT THE WHOLE REMEDY. Step 2 decided both
// arms with bwrapStat — os.Stat, which FOLLOWS — and then emitted
// `--bind p p`. bwrap resolves p at mount(2) and follows a link at a bind's
// source and at its destination alike, so a link at that name mounts the
// link's TARGET read-write at p's own name: a directory of the planter's
// choosing, inside a write wall that granted it nowhere. Nothing downstream
// said so. The predicate at step 6 asks only whether each ANCESTOR of a
// bind source is a mount destination, and p is always its own destination,
// so a link AT p passes; D2(a) resolves a grant, but only against the wall
// root.
//
// WHO PLANTS IT: any namespace that can write p's PARENT and holds no mount
// on p. A render granting only the directory above p leaves p itself loose
// and writable — the first CONTROL of the test above renders exactly that —
// so the same seat relaunched with a nested grant gets the plant's target,
// and a sibling namespace can plant it too, because a mount refuses a
// rename only in the namespace that holds it (P4/P4b).
//
// BOTH ARMS, because the grant arm is not the pin arm's corollary: a pin is
// a path no operator declared and a nested grant is one that an operator
// did, and this fleet had long stat'd a declared grant. Both are bound in
// place by the same loop and both resolve at mount(2), so both walk.
//
// AND BOTH DIRECTIONS, because a refusal that fires on everything is not a
// refusal: each arm renders over a REAL directory first and must bind it in
// place, or what follows is a refusal of the shape and not of the link.
//
// THE DANGLING ROW IS NOT ABOUT THE MOUNT AT ALL. os.Stat answers "not
// there" for a dangling link, which is the `!exists` arm, so the unfixed
// render asked the launcher for a PrepMkdirAll at that name — and the prep
// runs in the UNCAGED launcher, where MkdirAll follows the link and makes
// its target. The walk lstats, so a dangling link is a link and never an
// absence.
//
// MUTATION RUN 2026-11-21: swapping bwrapGrantSrcStat's walk back to a
// bwrapStat reds all three rows of both arms, and nothing else in
// `-run 'Wall|Bwrap'` moves.
func TestQAWallPinnedSourceRefuseALinkAtAGrantOrItsPinsOwnName(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	root := wallUngrantedDir(t)
	home, proj := filepath.Join(root, "home"), filepath.Join(root, "proj")
	outside := filepath.Join(root, "outside")
	outer := filepath.Join(proj, "g")
	mid, inner := filepath.Join(outer, "a"), filepath.Join(outer, "a", "b")
	for _, d := range []string{home, proj, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount + " (bwrapWallRoot)")
	}

	// `mid` is the name under attack in both arms — the PIN arm's loose
	// component between two grants, and the GRANT arm's own declared path —
	// so one plant drives both and the only thing that changes is the grant
	// set that makes step 2 bind it.
	plant := func(t *testing.T, link string) {
		t.Helper()
		if err := os.RemoveAll(mid); err != nil {
			t.Fatal(err)
		}
		if link == "" {
			if err := os.MkdirAll(inner, 0o755); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.Symlink(link, mid); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range []struct {
		name   string
		grants []string
	}{
		{"the PIN arm: the component between two nested grants", []string{outer, inner}},
		{"the GRANT arm: a grant one component inside another", []string{outer, mid}},
	} {
		// CONTROL: a real directory at that name renders, and step 2 binds
		// it in place. Without this the arms below measure nothing.
		plant(t, "")
		argv, _, err := BwrapArgs(proj, c.grants, SeatbeltCarveOut{})
		if err != nil {
			t.Fatalf("CONTROL: %s, over a real directory at %s, is refused: %v", c.name, mid, err)
		}
		if wcpMountAt(argv, "--bind", mid, mid) < 0 {
			t.Fatalf("CONTROL: %s binds nothing in place over %s, so the plant below is at a name this render never resolves\n %s", c.name, mid, bwJoin(argv))
		}

		// THE PLANT: the same name, a link to a directory inside NO grant of
		// this render, so every stat that FOLLOWS answers "a directory" and
		// the render goes through. Where it points is not the question — a
		// seat repoints it between the render and mount(2), the
		// check-then-use this ADR rejects A2 for — but a target outside
		// every grant is what the hole BUYS, and naming one is how this test
		// says what was at stake.
		plant(t, outside)
		argv, prep, err := BwrapArgs(proj, c.grants, SeatbeltCarveOut{})
		switch {
		case err == nil:
			t.Errorf("%s: a link at %s was bound in place, so bwrap resolves both ends of that bind to %s and mounts it read-write at a name this wall granted nowhere (ADR 0100 D2(b))\n %s", c.name, mid, outside, bwJoin(argv))
		case !strings.Contains(err.Error(), mid):
			t.Errorf("%s: the refusal does not name %s, which is the plant the operator has to remove: %v", c.name, mid, err)
		case !strings.Contains(err.Error(), "symlink"):
			t.Errorf("%s: the refusal does not say what is at that name in words, so a reader cannot tell it from a missing directory: %v", c.name, err)
		case !strings.Contains(err.Error(), "relaunch"):
			t.Errorf("%s: the refusal does not say to remove it and relaunch, which is the whole of what the operator can do about it: %v", c.name, err)
		default:
			if argv != nil || prep != nil {
				t.Errorf("%s: a refused render returned argv %v and prep %v", c.name, argv, bwPrep(prep))
			}
		}

		// THE DANGLING ROW: refused for the same reason, and asking the
		// launcher for nothing at that name.
		plant(t, filepath.Join(outside, "not-there"))
		argv, prep, err = BwrapArgs(proj, c.grants, SeatbeltCarveOut{})
		if err == nil {
			t.Errorf("%s: a DANGLING link at %s renders, so os.Stat's \"not there\" is still what decides this arm\n %s", c.name, mid, bwJoin(argv))
		}
		for _, s := range bwPrep(prep) {
			if strings.Contains(s, mid) {
				t.Errorf("%s: the render asks the launcher for %q over a dangling link at %s — that prep runs UNCAGED, and MkdirAll follows the link and makes its target (PrepStep's head comment)", c.name, s, mid)
			}
		}
	}
}

// Pin: WHICH GRANT THE WALK IS ROOTED AT — the shallowest one that STRICTLY
// contains the path, and nothing when no grant but its own does
// (bwrapOutermostContainingGrant, ranger-base-gi22r). As rows, because no
// render arm can see this choice: by the time two containing grants differ,
// the mount of the deeper one has already been judged by the walk from the
// shallower, so a wrong root here would show up as a refusal that is missing
// and never as one that is wrong.
//
// SHALLOWEST AND NOT DEEPEST, which is the row this exists for. A walk from
// the deepest containing grant would start BELOW the directories between
// that grant and the one above it — and a directory between two grants is
// exactly what this class is about (bwrapGrantMounts). The deeper grant's own
// mount does not help: it is emitted by this same loop, after this question
// is asked of it, and a link at its name is the thing being looked for.
//
// NOTHING WHERE THE PATH IS INSIDE NO OTHER GRANT, which is the condition
// bwrapGrantMounts carries and not an exemption: such a path's own PARENT
// lies outside every grant, so no namespace this wall describes can plant at
// its name, and what is left is the race ADR 0100 D1 prices. That arm keeps
// bwrapStat, which is what every grant in this fleet has always had.
func TestQAWallPinnedOutermostContainingGrantIsTheWalksRoot(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		p      string
		grants []string
		want   string // "" for "inside no grant but its own"
	}{
		{
			name: "inside no grant at all",
			p:    "/r/proj/g/a", grants: []string{"/r/other"},
		},
		{
			name: "the path IS the grant — strictly inside wants a parent that is granted",
			p:    "/r/proj/g", grants: []string{"/r/proj/g"},
		},
		{
			name: "one grant above it",
			p:    "/r/proj/g/a", grants: []string{"/r/proj/g"},
			want: "/r/proj/g",
		},
		{
			name: "TWO grants above it: the shallower one, so the walk judges the components between them",
			p:    "/r/proj/g/a/b/c", grants: []string{"/r/proj/g", "/r/proj/g/a"},
			want: "/r/proj/g",
		},
		{
			name: "the grants in the other order: the answer is the set's and not the slice's",
			p:    "/r/proj/g/a/b/c", grants: []string{"/r/proj/g/a", "/r/proj/g"},
			want: "/r/proj/g",
		},
		{
			name: "a SIBLING grant reaches nothing",
			p:    "/r/proj/g/a", grants: []string{"/r/proj/h", "/r/proj/g"},
			want: "/r/proj/g",
		},
		{
			name: "the root is a grant: every path is strictly inside it",
			p:    "/r", grants: []string{"/"},
			want: "/",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := bwrapOutermostContainingGrant(c.p, c.grants)
			if c.want == "" {
				if ok {
					t.Errorf("%s lies inside no grant of %v but its own, and the walk was rooted at %s — a root the render does not trust is a walk that judges components it has no business judging", c.p, c.grants, got)
				}
				return
			}
			if !ok || got != c.want {
				t.Errorf("the walk over %s is rooted at %q (ok %v), want %q", c.p, got, ok, c.want)
			}
		})
	}
}

// Pin: and the grant's RESOLVED spelling is a candidate root too, which the
// composed rows above cannot show because absResolve asks this filesystem.
// bwrapStrictlyInsideGrant compares both spellings and its head comment says
// why — either spelling reaching the component is a seat that can write it,
// and a predicate whose job is to refuse must not be narrowed by a host whose
// home sits behind a symlink. The ROOT has to come from the same union, or a
// grant declared through the link reaches the path and the walk is rooted
// nowhere.
func TestQAWallPinnedOutermostContainingGrantReadsAGrantsResolvedSpelling(t *testing.T) {
	t.Parallel()
	root := absResolve(t.TempDir())
	real, link := filepath.Join(root, "real"), filepath.Join(root, "link")
	p := filepath.Join(real, "a", "b")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// CONTROL: the link's own spelling does not reach p as written.
	if got, ok := bwrapOutermostContainingGrant(p, []string{link}); !ok || got != real {
		t.Errorf("the grant %s resolves to %s, which contains %s — the walk was rooted at %q (ok %v), so a grant declared through a symlinked home roots the walk nowhere and every component of it is trusted", link, real, p, got, ok)
	}
}

// Pin: what shape 1's mount list IS — every write grant, plus every
// directory strictly between a grant and the grant that contains it,
// shallowest first — as rows, one per shape the predicate argues about.
// The render arms above and below drive the same function through BwrapArgs;
// this is the function's own answer, where a row costs nothing to add.
func TestQAWallPinnedGrantMountsNamesTheDirectoriesBetweenNestedGrants(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		grants []string
		want   []string // the list in order, a pin marked with a leading +
	}{
		{
			name:   "two grants neither of which contains the other",
			grants: []string{"/a", "/b"},
			want:   []string{"/a", "/b"},
		},
		{
			name:   "a grant ONE component inside another: its own mount pins it",
			grants: []string{"/g", "/g/a"},
			want:   []string{"/g", "/g/a"},
		},
		{
			name:   "TWO components inside: the one between is pinned",
			grants: []string{"/g", "/g/a/b"},
			want:   []string{"/g", "+/g/a", "/g/a/b"},
		},
		{
			name:   "THREE components inside: both between, shallowest first",
			grants: []string{"/g", "/g/a/b/c"},
			want:   []string{"/g", "+/g/a", "+/g/a/b", "/g/a/b/c"},
		},
		{
			name:   "two nested grants sharing one loose component: one pin",
			grants: []string{"/g", "/g/a/b", "/g/a/c"},
			want:   []string{"/g", "+/g/a", "/g/a/b", "/g/a/c"},
		},
		{
			name:   "the component between is granted as well: it is a grant and not a pin",
			grants: []string{"/g", "/g/a", "/g/a/b"},
			want:   []string{"/g", "/g/a", "/g/a/b"},
		},
		{
			name:   "no grant above it at all: a deep grant on its own pins nothing",
			grants: []string{"/g/a/b"},
			want:   []string{"/g/a/b"},
		},
		{
			// The shape this list was written for (ranger-base-dot6w): a
			// store of record inside the same repo grants <common> whole.
			name:   "a store of record in the same repo: <common> whole, and the seat's own git dir two deep",
			grants: []string{"/r/.git", "/r/.git/objects", "/r/.git/logs", "/r/.git/worktrees/s-1"},
			want:   []string{"/r/.git", "/r/.git/logs", "/r/.git/objects", "+/r/.git/worktrees", "/r/.git/worktrees/s-1"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, m := range bwrapGrantMounts(c.grants) {
				if m.Pin {
					if !writeGranted(c.grants, m.For) || !insideDir(m.Path, m.For) {
						t.Errorf("the pin %s names %s as the grant it pins, which is not a grant strictly below it", m.Path, m.For)
					}
					got = append(got, "+"+m.Path)
					continue
				}
				got = append(got, m.Path)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("shape 1's mounts over %v are\n %v\nwant\n %v", c.grants, got, c.want)
			}
		})
	}
}

// Pin: the SEAT render in the shape this bead is from — a store of record
// inside the same repo, which grants `<common>` whole — renders, and the
// seat's own git dir is pinned by an in-place bind of `<common>/worktrees`
// between them. Same shape as the live seat-store pin's second arm, read off
// the argv instead of through bwrap, so it reds on any box.
//
// THE CONTROL IS THE SAME RENDER WITHOUT THAT GRANT: no `<common>` grant, no
// nest, no pin — which is what "it widens nothing" means here.
func TestQAWallPinnedSourceWholeCommonGrantPinsTheSeatsOwnGitDir(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("claude"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if !writeGranted(r.Set, f.l.Own) {
		t.Skipf("NOTHING MEASURED: this seat is not granted its per-worktree git dir %s, so no grant is nested in the <common> grant below", f.l.Own)
	}
	common := filepath.Dir(f.l.Objects)
	between := filepath.Dir(f.l.Own)
	if !insideDir(common, between) || !insideDir(between, f.l.Own) {
		t.Fatalf("CONTROL: %s does not lie between the <common> dir %s and the seat's own git dir %s, so this fixture is not the shape under test", between, common, f.l.Own)
	}
	if containsString(bwrapMountDsts(bwrapMounts(r.Argv)), between) {
		t.Errorf("the shipped render, which does not grant %s whole, still binds %s: a pin inside no grant hands out a write nobody asked for\n %s", common, between, bwJoin(r.Argv))
	}

	wide := append(append([]string{}, r.Set...), common)
	argv, _, err := BwrapArgs(f.dir, wide, r.Carve, r.Keep...)
	if err != nil {
		t.Fatalf("a render granting <common> whole is refused (%v) — that is a seat in a linked worktree that does not launch at all, and the fix ADR 0100 prescribes is the in-place bind and not an exemption (ranger-base-dot6w)", err)
	}
	ms := bwrapMounts(argv)
	dsts := bwrapMountDsts(ms)
	if !containsString(dsts, between) {
		t.Errorf("nothing is bound over %s, which lies strictly inside the <common> grant and holds the seat's own git dir: a leftover of this seat renames it and the next launch binds what the name points at, read-write (ADR 0100 D2(b))\n %s", between, bwJoin(argv))
	}
	grants := shallowestFirst(wide)
	for _, m := range ms {
		if m.Src == "" {
			continue
		}
		if loose, ok := bwrapPinned(m.Src, grants, dsts); !ok {
			t.Errorf("the render binds %s over %s and %s is loose\n %s", m.Src, m.Dst, loose, bwJoin(argv))
		}
	}
}
