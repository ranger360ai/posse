//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQAGitGlobalSnapshotIsUnderNoCagedWriteGrant (ranger-base-161t2): the
// GIT_CONFIG_GLOBAL row's snapshot, its lock directory and the operator's
// own global files are outside the caged write set. A snapshot under a
// granted dir (`.cache/posse` was the first draft) is one a caged seat can
// rewrite for every sibling launched over the same bytes, core.hooksPath
// included.
func TestQAGitGlobalSnapshotIsUnderNoCagedWriteGrant(t *testing.T) {
	// No t.Parallel: the fixture sets HOME.
	f := weFixtureIn(t, wallUngrantedDir(t))
	for _, e := range os.Environ() {
		if k, _, ok := strings.Cut(e, "="); ok && strings.HasPrefix(k, "GIT_CONFIG") {
			unsetenvForTest(t, k)
		}
	}
	unsetenvForTest(t, "XDG_CONFIG_HOME")
	if os.Getenv("HOME") != f.home {
		t.Fatalf("control: the fixture's HOME is %q, not %q, so the snapshot below is not this home's", os.Getenv("HOME"), f.home)
	}
	op := filepath.Join(f.home, ".gitconfig")
	if err := os.WriteFile(op, []byte("[user]\n\temail = op@example.invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := gitGlobalScope()
	if err != nil || file != op {
		t.Fatalf("control: the global scope resolved to %q (%v), want %s", file, err, op)
	}
	snap := gitGlobalPinOf(t, file)
	if !insideDir(SeatWallRoot(f.home), snap) {
		t.Errorf("the snapshot %s is not under the wall root %s", snap, SeatWallRoot(f.home))
	}

	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, f.rt.StateDirs...)
	if !writeGranted(w, absResolve(filepath.Join(f.home, ".cache", "posse"))) {
		t.Fatalf("control: `.cache/posse` is not granted, so the rows below measure an empty write set: %v", w)
	}
	for _, p := range []string{snap, snap + ".lock", filepath.Dir(snap), op, filepath.Join(f.home, ".config", "git", "config")} {
		if writeGranted(w, absResolve(p)) {
			t.Errorf("the caged write set grants %s — a seat could change the global scope its siblings or the operator read (ranger-base-161t2)\n set: %v", p, w)
		}
	}
}
