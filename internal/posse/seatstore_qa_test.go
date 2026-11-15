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
	l := seatStoreFrom(dirs[0], dirs[1])
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
	l := seatStoreFrom(own, common)
	in := []CageMount{
		{Src: common, Dst: common, Why: "a store of record that grants the common dir whole"},
		{Src: own, Dst: own, Why: "the per-worktree dir"},
		{Src: l.Objects, Dst: l.Objects, Why: "the old overlay of the shared store"},
	}
	ms := seatStoreCageMounts(in, own, common)
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

// darwin's tier cannot bind, so its walled prepare makes no store (ADR 0099
// D7) — and the container tier always does.
func TestQASeatStoreMechanism(t *testing.T) {
	t.Parallel()
	if !seatStoreMechanism(true) {
		t.Error("the container tier must always get a seat store")
	}
	if got := seatStoreMechanism(false); got != (WallMechanism() == WallBwrap) {
		t.Errorf("seatStoreMechanism(false) = %v on a host whose mechanism is %q", got, WallMechanism())
	}
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
