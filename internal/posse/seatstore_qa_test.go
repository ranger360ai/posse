//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-21i98 that need no bwrap: the L4 mount list (pin 2
// on the bead) and the launcher's preparation of the store. The live L3 rows
// are in bwrapseatstorelive_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pin (2): the container tier's read-write mount at `<common>/objects` is the
// seat's OWN store, the shared store is read-only at the side path, and there
// is exactly one mount at each destination. A whole-tree-deny PID gets
// neither (ADR 0086 D8).
//
// MUTATION RUN: skipping seatStoreCageMounts' store mount reds the Src row
// (the overlay left at `<common>/objects` is the shared store, read-write).
func TestQASeatStoreCageMountsTheSeatStoreOverObjects(t *testing.T) {
	t.Parallel()
	wt := gitWorktreeFixture(t)
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dirs := LinkedGitDirs(wt)
	if len(dirs) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", dirs)
	}
	l, ok := seatStoreFor(wt)
	if !ok {
		t.Fatal("no store layout for a linked worktree")
	}
	for _, d := range []string{l.Store, l.Base, filepath.Join(dirs[1], "logs")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	ms := a.CageMounts(cageAgent(t, a, "cage: container\n"), e, wt, "s1")
	m, ok := mountAt(ms, l.Objects)
	switch {
	case !ok:
		t.Fatalf("nothing is mounted at <common>/objects:\n%s", showMounts(ms))
	case m.RO:
		t.Errorf("<common>/objects is read-only, so the seat cannot commit: %+v", m)
	case absResolve(m.Src) != l.Store:
		t.Errorf("<common>/objects is bound from %s, not the seat's own store %s — a committing seat can rewrite history's objects", m.Src, l.Store)
	}
	wantMode(t, ms, l.Base, true, "the shared store at the side path")
	if b, ok := mountAt(ms, l.Base); ok && absResolve(b.Src) != l.Objects {
		t.Errorf("the side path is bound from %s, not the shared store %s", b.Src, l.Objects)
	}
	for _, p := range []string{l.Objects, l.Base} {
		n := 0
		for _, m := range ms {
			if absResolve(m.Dst) == absResolve(p) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%d mounts land at %s; two binds at one destination is an engine error:\n%s", n, p, showMounts(ms))
		}
	}

	deny := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\n"), e, wt, "s1")
	wantNoMount(t, deny, l.Objects, "a whole-tree-deny PID gets no git-dir write, the store included (ADR 0086 D8)")
	wantNoMount(t, deny, l.Base, "a whole-tree-deny PID gets no side path")
}

// The store's mount is the deepest over `<common>/objects` even when another
// mount grants `<common>` WHOLE read-write (a store of record inside the same
// repo), and an existing mount AT that destination is replaced, not doubled.
func TestQASeatStoreCageMountsAreDeepestOverAWholeCommonGrant(t *testing.T) {
	t.Parallel()
	wt := gitWorktreeFixture(t)
	dirs := LinkedGitDirs(wt)
	if len(dirs) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", dirs)
	}
	own, common := absResolve(dirs[0]), absResolve(dirs[1])
	home, _ := os.UserHomeDir()
	wall := SeatWallDir(home, wt)
	l := seatStoreFrom(own, common, wall)
	in := []CageMount{
		{Src: common, Dst: common, Why: "a store of record that grants the common dir whole"},
		{Src: own, Dst: own, Why: "the per-worktree dir"},
		{Src: l.Objects, Dst: l.Objects, Why: "the old overlay of the shared store"},
	}
	ms := seatStoreCageMounts(in, own, common, wall)
	m, ok := mountAt(ms, l.Objects)
	if !ok || m.RO || absResolve(m.Src) != l.Store {
		t.Fatalf("<common>/objects under a whole-common grant is %+v (ok=%v), want the seat store read-write", m, ok)
	}
	deepest, _ := cageCovering(ms, filepath.Join(l.Objects, "ab"))
	if deepest < 0 || absResolve(ms[deepest].Src) != l.Store {
		t.Fatalf("the deepest mount over an object is not the seat store:\n%s", showMounts(ms))
	}
	if len(ms) != len(in)+1 {
		t.Errorf("want the exact objects mount replaced and only the side path added, got %d mounts from %d:\n%s", len(ms), len(in), showMounts(ms))
	}
}

// The walled prepare makes the store and the side path and writes the store's
// alternates to name the side path — and rewrites it on every launch, so a
// value the seat left there does not survive.
func TestQASeatStorePrepareWritesTheAlternates(t *testing.T) {
	t.Parallel()
	a, repo := wtApp(t), wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareSeatStore(tr); err != nil {
		t.Fatal(err)
	}
	l, ok := seatStoreFor(tr.Path)
	if !ok {
		t.Fatal("no store layout for a linked worktree")
	}
	alt := filepath.Join(l.Store, "info", "alternates")
	read := func() string {
		b, err := os.ReadFile(alt)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}
	if got := read(); got != l.Base {
		t.Fatalf("alternates reads %q, want the side path %s", got, l.Base)
	}
	if fi, err := os.Stat(l.Base); err != nil || !fi.IsDir() {
		t.Fatalf("the side path is not a directory: %v", err)
	}
	write(t, alt, "/somewhere/the/seat/chose\n")
	if err := prepareSeatStore(tr); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != l.Base {
		t.Fatalf("a relaunch left the seat's alternates %q in place", got)
	}
}

// A link the seat planted in its store is never written through: not at
// `info`, not at `info/alternates`, and not at the store itself.
func TestQASeatStorePrepareFollowsNoLinkOutOfTheStore(t *testing.T) {
	t.Parallel()
	for _, at := range []string{"store", "info", "alternates"} {
		t.Run(at, func(t *testing.T) {
			t.Parallel()
			a, repo := wtApp(t), wtRepo(t)
			tr, err := a.EnsureSessionTree(repo, "s-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			l, _ := seatStoreFor(tr.Path)
			// The wall dir by hand, because the plant goes in before the
			// launcher's own mkdir of it (ADR 0100 D1 moved the store
			// there). Nothing in production can do this — no render grants
			// the wall root — and that is the point: the os.Root discipline
			// stays as the refusal a plant by anything ELSE still meets.
			if err := os.MkdirAll(l.Wall, 0o755); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(t.TempDir(), "victim")
			if err := os.MkdirAll(filepath.Join(victim, "info"), 0o755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(victim, "info", "alternates")
			write(t, target, "untouched\n")
			switch at {
			case "store":
				if err := os.Symlink(victim, l.Store); err != nil {
					t.Fatal(err)
				}
			case "info":
				if err := os.MkdirAll(l.Store, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(victim, "info"), filepath.Join(l.Store, "info")); err != nil {
					t.Fatal(err)
				}
			case "alternates":
				if err := os.MkdirAll(filepath.Join(l.Store, "info"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(l.Store, "info", "alternates")); err != nil {
					t.Fatal(err)
				}
			}
			_ = prepareSeatStore(tr)
			if b, _ := os.ReadFile(target); string(b) != "untouched\n" {
				t.Fatalf("a link at the seat's %s wrote the launcher's alternates outside the store: %q", at, b)
			}
		})
	}
}

// Every walled tier gets a store since ranger-base-5blmt: bwrap and the
// container bind it, darwin's sandbox-exec grants it beside a deny. Only the
// sandbox-exec tier's store is sideless.
func TestQASeatStoreMechanism(t *testing.T) {
	t.Parallel()
	if !seatStoreMechanism(true) {
		t.Error("the container tier must always get a seat store")
	}
	if got := seatStoreMechanism(false); got != (WallMechanism() != "") {
		t.Errorf("seatStoreMechanism(false) = %v on a host whose mechanism is %q", got, WallMechanism())
	}
	if seatStoreSideless(true) {
		t.Error("the container tier mounts the shared store at a side path; it is never sideless")
	}
	if got := seatStoreSideless(false); got != (WallMechanism() == WallSandboxExec) {
		t.Errorf("seatStoreSideless(false) = %v on a host whose mechanism is %q", got, WallMechanism())
	}
}

// THE NO-WALL ARMS of all three producers (ranger-base-r9slg, escaped from
// ranger-base-ltmyh). A launch that can name no per-seat wall dir — no HOME —
// is the one state in which the seat store cannot be placed where ADR 0100 D1
// requires, and each producer answers it in the only way its layer allows:
// L3 and the prepare REFUSE, L4 cannot (it renders a mount list and returns
// no error) so it fails CLOSED, mounting the shared store read-only over its
// own name.
//
// Nothing drove any of them. MEASURED 2026-11-15 at 4b3013cd, each mutant
// applied alone with the package run unfiltered (4054 tests, 0 FAIL):
// `put(l.Objects, l.Objects, true,` -> `false` — the shared object store
// read-write inside a caged seat, ADR 0099 D1's whole subject — and
// seatStoreBinds' refusal replaced by `return nil, nil`, which renders
// nothing and leaves the seat the shared store read-write from the plain
// grant. The reason both lived: `seatStoreNoWall` appears in no test file in
// this tree, and both shipped mount pins compose `wall` from a real
// os.UserHomeDir()/SeatWallDir, so `l.Store` was never "" in a test.
//
// Every arm carries its populated row as the control, because an absent
// store and a refusing producer look identical when the fixture never had
// one.
func TestQASeatStoreWithNoWallDirFailsClosedAtEveryProducer(t *testing.T) {
	t.Run("L4 mounts the shared store read-only over its own name", func(t *testing.T) {
		t.Parallel()
		wt := gitWorktreeFixture(t)
		dirs := LinkedGitDirs(wt)
		if len(dirs) != 2 {
			t.Fatalf("a linked worktree has two git dirs, got %v", dirs)
		}
		own, common := absResolve(dirs[0]), absResolve(dirs[1])
		in := []CageMount{{Src: common, Dst: common, Why: "a grant of the common dir whole"}}
		l := seatStoreFrom(own, common, "")
		if l.Store != "" {
			t.Fatalf("FIXTURE: a layout with no wall dir still names a store %q, so this arm is not the no-wall case", l.Store)
		}

		ms := seatStoreCageMounts(in, own, common, "")
		m, ok := mountAt(ms, l.Objects)
		if !ok {
			t.Fatalf("nothing is mounted at <common>/objects:\n%s", showMounts(ms))
		}
		if !m.RO {
			t.Errorf("with no wall dir to hold a store, <common>/objects is mounted READ-WRITE: that is the SHARED object store, writable inside a caged seat, which is what ADR 0099 D1 exists to stop (%+v)", m)
		}
		if absResolve(m.Src) != l.Objects {
			t.Errorf("the fail-closed mount is bound from %s, want the shared store over its own name %s", m.Src, l.Objects)
		}
		wantNoMount(t, ms, l.Base, "with no store there is no side path for its alternates to name")
		if len(ms) != len(in)+1 {
			t.Errorf("want exactly one mount added, got %d from %d:\n%s", len(ms), len(in), showMounts(ms))
		}

		// CONTROL: the same call WITH a wall dir gives the seat its own store
		// read-write, so the read-only row above is about the missing wall dir
		// and not about this fixture.
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		wall := SeatWallDir(home, wt)
		if wall == "" {
			t.Fatal("CONTROL: this binary's HOME names no wall dir, so there is nothing to compare the arm above against")
		}
		cs := seatStoreCageMounts(in, own, common, wall)
		c, ok := mountAt(cs, l.Objects)
		if !ok || c.RO || absResolve(c.Src) != seatStoreFrom(own, common, wall).Store {
			t.Fatalf("CONTROL: with a wall dir <common>/objects is %+v (ok=%v), want the seat's own store read-write — the arm above measured nothing", c, ok)
		}
	})

	t.Run("L3 refuses rather than rendering nothing", func(t *testing.T) {
		t.Parallel()
		wt := gitWorktreeFixture(t)
		b, err := seatStoreBinds(nil, "", wt)
		if err == nil {
			t.Errorf("seatStoreBinds with no home rendered %d bind(s) and no error: a render that says nothing leaves the seat the SHARED object store read-write from the plain grant (ADR 0100 D1)", len(b))
		} else if !strings.Contains(err.Error(), "ADR 0100 D1") {
			t.Errorf("the no-home refusal does not name the decision it enforces: %v", err)
		}
		if len(b) != 0 {
			t.Errorf("the refusal came with %d bind(s): %+v", len(b), b)
		}

		// CONTROL: with a home the same call renders the two binds, so the
		// refusal above is the no-home arm and not a broken fixture.
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		got, err := seatStoreBinds(nil, home, wt)
		if err != nil || len(got) != 2 {
			t.Fatalf("CONTROL: seatStoreBinds(home) = %d bind(s), %v; want 2 and no error — the arm above measured nothing", len(got), err)
		}
	})

	t.Run("the prepare refuses", func(t *testing.T) {
		// No t.Parallel: the no-home state is HOME, which is per process.
		a, repo := wtApp(t), wtRepo(t)
		tr, err := a.EnsureSessionTree(repo, "s-1", nil)
		if err != nil {
			t.Fatal(err)
		}
		// CONTROL first, while the home is still real: the prepare does make
		// a store here, so the refusal below is about the home.
		if err := prepareSeatStore(tr); err != nil {
			t.Fatalf("CONTROL: the prepare refused with a real home (%v), so the arm below measures nothing", err)
		}
		t.Setenv("HOME", "")
		if l, ok := seatStoreFor(tr.Path); !ok || l.Store != "" {
			t.Fatalf("FIXTURE: HOME=\"\" still names the store %q (ok=%v), so this is not the no-wall case", l.Store, ok)
		}
		err = prepareSeatStore(tr)
		if err == nil {
			t.Error("the prepare with no home returned nil: the launch then goes on to render a wall over a store that was never made (ADR 0100 D1)")
		} else if !strings.Contains(err.Error(), "ADR 0100 D1") {
			t.Errorf("the no-home refusal does not name the decision it enforces: %v", err)
		}
	})
}

// The L3 wiring: posse's OWN render (bwrapRender) carries the store's bind
// over `<common>/objects` and the shared store read-only at the side path,
// AFTER every write grant, so the store is the deepest mount there. The live
// rows drive seatStoreBinds straight into BwrapArgs; this is what says the
// shipped render calls it. A whole-tree-deny PID's render carries neither.
//
// MUTATION RUN: removing the seatStoreBinds append from bwrapRender reds this.
func TestBwrapRenderBindsTheSeatStoreOverObjects(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	repo := filepath.Join(filepath.Dir(f.dir), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "root")
	mustGit(t, repo, "worktree", "add", "-q", "--detach", f.dir)
	l, ok := seatStoreFor(f.dir)
	if !ok {
		t.Fatal("CONTROL: the fixture's dir is not a linked worktree")
	}

	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	line := bwJoin(r.Argv)
	store := bwIndex(r.Argv, "--bind", l.Store, l.Objects)
	if store < 0 {
		t.Fatalf("the wall posse renders does not bind the seat's own store over <common>/objects, so a committing seat writes the shared store:\n  %s", line)
	}
	if bwIndex(r.Argv, "--ro-bind", l.Objects, l.Base) < 0 {
		t.Errorf("the wall posse renders does not bind the shared store read-only at the side path:\n  %s", line)
	}
	for i := store + 1; i+2 < len(r.Argv); i++ {
		if r.Argv[i] == "--bind" && absResolve(r.Argv[i+2]) == l.Objects {
			t.Errorf("a later read-write bind lands on <common>/objects after the store's, so the store is not the deepest mount:\n  %s", line)
		}
	}

	f.ag.Deny = []string{"Edit", "Write"}
	r, err = f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	if bwIndex(r.Argv, "--bind", l.Store, l.Objects) >= 0 || bwIndex(r.Argv, "--ro-bind", l.Objects, l.Base) >= 0 {
		t.Errorf("a whole-tree-deny PID's render carries the seat store (ADR 0086 D3):\n  %s", bwJoin(r.Argv))
	}
}
