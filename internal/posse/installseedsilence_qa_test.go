//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-g4cm, found verifying ranger-base-1vpf's close (ranger-base-c9l8).
//
// INSTALL.md §14's seeding row ends by inferring a PROMOTED home from init
// saying nothing about promoted.json. This measures the counterexample: a
// SEEDED home, verifiable, that init is equally silent about — because the
// re-stamp arm is guarded by `wrote > 0` (init.go) and a re-run that fills no
// gap writes nothing.
//
// Reachable from the row's own stated cause. It says a real seed tree sits one
// level above the binary; the ordinary instance is another posse checkout,
// whose examples/ carries the same FILENAMES as the embed —
// TestEmbeddedSeedMatchesExamplesDir pins the embed as examples/ byte for
// byte — and same filenames is exactly `wrote == 0`.
//
// Green on both sides of the doc fix, on purpose: it asserts what init DOES,
// not what the row says about it, so correcting the row does not red it. It
// reds if initFrom ever starts speaking in this case — which would make the
// row's inference sound and is worth knowing either way — and it reds if the
// silent home stops verifying, which is the outage the row is about.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// fullForeignSeedDir is the row's cause in its commonest shape: a real seed
// tree that misses NOTHING the embed carries, with content of its own.
func fullForeignSeedDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "examples")
	err := fs.WalkDir(posse.Seed, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return os.WriteFile(dst, []byte("stale: true\n"), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestQAInitSilenceDoesNotMeanAPromotedHome(t *testing.T) {
	t.Parallel()
	a := initTestApp(t)
	var out strings.Builder
	if err := a.initFrom(&out, os.DirFS(fullForeignSeedDir(t)), "stale"); err != nil {
		t.Fatalf("the wrong seed: %v\n%s", err, out.String())
	}
	// The fixture's witness, and the thing that makes the silence below mean
	// anything: this home really is SEEDED, not promoted and not unstamped.
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil || !m.Seeded {
		t.Fatalf("the fixture must be a seeded home, or this measures nothing: %+v %v", m, err)
	}

	// THE ROW'S FIX, on a home the wrong seed left no gap in.
	out.Reset()
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("the re-run: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "promoted.json") {
		t.Errorf("init now speaks about the manifest on a re-run that filled no gap — INSTALL.md §14's row infers a PROMOTED home from that silence, so rewrite the row's last sentence and this test with it (ranger-base-g4cm):\n%s", out.String())
	}
	// And the home the row calls promoted-and-refusing is neither.
	if v := a.VerifyPromoted(); !v.OK() {
		t.Errorf("a seeded home the re-run filled no gap in must still verify: %s\n%s", v.Line(), out.String())
	}
	if m, err := ReadPromoteManifest(a.PromoteManifestPath()); err != nil || m == nil || !m.Seeded {
		t.Errorf("the manifest stopped being seeded across a re-run that wrote nothing: %+v %v", m, err)
	}

	// The control arm: the SAME procedure over a seed that does miss files —
	// ranger-base-e6y's repaired arm — where init speaks and the row's
	// happy half holds. Without it "init said nothing" reads identically
	// against an init that never says anything at all.
	c := initTestApp(t)
	var cout strings.Builder
	if err := c.initFrom(&cout, os.DirFS(foreignSeedDir(t)), "stale"); err != nil {
		t.Fatalf("the gappy seed: %v\n%s", err, cout.String())
	}
	cout.Reset()
	if err := c.initFrom(&cout, posse.Seed, "embedded"); err != nil {
		t.Fatalf("the re-run: %v\n%s", err, cout.String())
	}
	if !strings.Contains(cout.String(), "re-stamped") {
		t.Errorf("a re-run that DID fill a gap must still say so, or the silence above is not the wrote==0 case:\n%s", cout.String())
	}
}
