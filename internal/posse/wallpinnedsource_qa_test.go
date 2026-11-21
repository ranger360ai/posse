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
		if n := wpsCountPinnedInPlace(t, name, r.Argv, shallowestFirst(r.Set)); n < 2 {
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
		if n := wpsCountPinnedInPlace(t, kind.String(), cage.Argv, writable); n == 0 {
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

// Pin: a render carrying a TWO-DEEP unpinned grant is REFUSED, by the name
// of the loose component, and the one-deep nest beside it still renders.
// This is the shape a PID would arrive in — a `writable:` extra two or more
// components inside a grant the seat already has — and it is the arm the
// widening above exists for. The refusal's instruction is the one ADR 0100
// prescribes everywhere else: bind the loose component in place, or narrow
// the grant above it; never exempt it.
//
// MUTATION RUN 2026-11-21: restoring the `m.Src == m.Dst` skip in
// bwrapRefuseUnpinnedSource reds this test alone.
func TestQAWallPinnedSourceRefusesATwoDeepUnpinnedGrant(t *testing.T) {
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

	argv, prep, err := BwrapArgs(proj, []string{outer, inner}, SeatbeltCarveOut{})
	switch {
	case err == nil:
		t.Fatalf("a render granting %s inside %s rendered: %s lies strictly inside the outer grant and is a mount of nothing, so a leftover of this seat renames it and bwrap resolves %s to whatever the plant points at — the wall root, read-write, at the next launch (ADR 0100 D2(b))\n argv: %q", inner, outer, mid, inner, argv)
	case !strings.Contains(err.Error(), mid):
		t.Errorf("the refusal does not name the loose component %s, which is the only thing the operator can act on: %v", mid, err)
	case !strings.Contains(err.Error(), "in place"):
		t.Errorf("the refusal does not name the fix (bind the loose component in place): %v", err)
	}
	if len(prep) != 0 {
		t.Errorf("the refused render came back with prep %v for the uncaged launcher to run", prep)
	}
}
