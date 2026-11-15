//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-v84cn: the deny directories a host process keeps
// removing while they are empty get an anchor file before their bind, so a
// host rmdir is refused and cannot take the wall off a live seat.
// SeatbeltCarveOut.Anchored and claudeSweptConfigDirNames have the
// measurement.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The render: an anchored deny DIRECTORY gets a seed of the anchor file
// after its own mkdir (absent) or alone (present). An anchored name that is
// a FILE gets nothing, and so does an unanchored dir.
//
// MUTATION-CHECKED: dropping the anchor step from BwrapArgs reds the first
// two rows; dropping the `!exists || dir` guard reds the file row.
func TestBwrapArgsAnchorsAnAnchoredDenyDirectoryAndNothingElse(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"wt", "cfg/present", "cfg/plain"}, []string{"cfg/afile"})
	grant := bwAt("cfg")
	present, absent, afile, plain := bwAt("cfg", "present"), bwAt("cfg", "absent"), bwAt("cfg", "afile"), bwAt("cfg", "plain")

	argv, prep, err := BwrapArgs(bwAt("wt"), []string{bwAt("wt"), grant}, SeatbeltCarveOut{
		Deny:   []string{present, absent, afile, plain},
		Anchor: []string{present, absent, afile},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(bwPrep(prep), "\n")
	for _, want := range []string{
		"seed " + filepath.Join(present, wallAnchorName),
		"mkdir -p " + absent + "\nseed " + filepath.Join(absent, wallAnchorName),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prep has no %q — an anchored deny dir left empty is one host rmdir away from losing its mount:\n%s", want, got)
		}
	}
	for _, no := range []string{filepath.Join(afile, wallAnchorName), filepath.Join(plain, wallAnchorName)} {
		if strings.Contains(got, no) {
			t.Errorf("prep anchors %s, which is a file or not anchored:\n%s", no, got)
		}
	}
	line := bwJoin(argv)
	for _, d := range []string{present, absent, afile, plain} {
		if !strings.Contains(line, "--ro-bind "+d+" "+d) {
			t.Errorf("the anchor changed the wall: %s is no longer bound read-only:\n%s", d, line)
		}
	}
}

// End to end over claude's config dir: the carve anchors exactly
// claudeSweptConfigDirNames, the launcher's prep plants the anchor in each,
// a host rmdir of each is then refused, and a second prep (the next launch)
// leaves the anchor as it is. A dir that loads code gets no anchor.
func TestQAWallAnchorKeepsClaudesSweptConfigDirsFromAHostRmdir(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	stateDirs, err := BwrapStateDirs(f.rt)
	if err != nil {
		t.Fatal(err)
	}
	cfg := absResolve(f.cfg)
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, stateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, stateDirs...)

	// Hand-typed, so a name dropped from the table reds here.
	swept := []string{"access-audit", "logs", "mcp-discovery-cache", "mcp-skill-archives", "shares", "statsig", "todos"}
	var want []string
	for _, n := range swept {
		want = append(want, filepath.Join(cfg, n))
	}
	if strings.Join(c.Anchor, "\n") != strings.Join(want, "\n") {
		t.Fatalf("carve anchors %v, want %v", c.Anchor, want)
	}

	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // the second run is the next launch
		if err := runBwrapPrep(r.Prep); err != nil {
			t.Fatalf("prep run %d: %v", i+1, err)
		}
	}
	for _, d := range want {
		if fi, err := os.Lstat(filepath.Join(d, wallAnchorName)); err != nil || !fi.Mode().IsRegular() || fi.Size() != 0 {
			t.Errorf("%s holds no empty anchor file after the prep: %v", d, err)
			continue
		}
		if err := syscall.Rmdir(d); !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
			t.Errorf("host rmdir of %s = %v, want ENOTEMPTY — an rmdir that succeeds takes the seat's mount off", d, err)
		}
	}
	for _, n := range []string{"daemon", "jobs", "skills", "hooks", "state"} {
		if _, err := os.Lstat(filepath.Join(cfg, n, wallAnchorName)); err == nil {
			t.Errorf("%s/ got an anchor; only claudeSweptConfigDirNames are anchored, and a file in a dir claude acts on is not measured inert", n)
		}
	}
}
