//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-s7tkc (fix (a) for ranger-base-al0d0): the bwrap wall
// hides the host's crash stores — systemd-coredump's directory and the
// persistent and volatile journals — from every seat, by riding the same Hide
// list and the same 1b loop as the user runtime dir.
//
// THE NAMES ARE SPELLED HERE, not read off bwrapCrashStoreDirs: a loop over
// the list under test has nothing to say when an entry is dropped
// (ranger-base-3clx6's lesson, recorded in bwraphide_qa_test.go).

import (
	"os"
	"strings"
	"testing"
)

var crashStoreSpelled = []string{
	"/var/lib/systemd/coredump",
	"/var/log/journal",
	"/run/log/journal",
}

// The list carries all three, so a host that has any of them gets it hidden.
func TestBwrapCrashStoreListNamesEveryStore(t *testing.T) {
	t.Parallel()
	for _, d := range crashStoreSpelled {
		if !containsString(bwrapCrashStoreDirs, d) {
			t.Errorf("bwrapCrashStoreDirs dropped %s, so every bwrap seat reads a same-uid peer's crash out of it (ranger-base-s7tkc):\n  %v", d, bwrapCrashStoreDirs)
		}
	}
}

// Absent is skipped, a file is skipped, a symlink resolves, a directory is
// kept — and none of them refuses the launch, because nothing a seat runs
// needs these and a host without systemd-coredump must still render.
func TestBwrapPresentDirsKeepsOnlyDirectoriesAndNeverRefuses(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"store"}, []string{"file"})
	if err := os.Symlink(bwAt("store"), bwAt("link")); err != nil {
		t.Fatal(err)
	}
	got := bwrapPresentDirs([]string{bwAt("store"), bwAt("file"), bwAt("missing"), bwAt("link")})
	want := []string{bwAt("store"), bwAt("store")}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("bwrapPresentDirs = %v, want %v (the directory, and the symlink resolved to it; the file and the absent name skipped)", got, want)
	}
}

// What the seat's kernel sees: posse's own render carries `--tmpfs` over each
// crash store that is a directory on THIS host. Read against the spelled
// names, so dropping one from the list, or the append that wires the list
// into liveBwrapRuntimeDirs, reds here on any host that has it.
func TestBwrapRenderHidesTheHostCrashStores(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	present := bwrapPresentDirs(crashStoreSpelled)
	if len(present) == 0 {
		t.Skip("NOTHING MEASURED: this host has none of the crash stores for the render to hide")
	}
	f := wrcFixtureIn(t, wallUngrantedDir(t))
	r, err := f.a.bwrapRender(f.ag, wrcRuntime("codex"), f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range present {
		if !containsString(r.Carve.Hide, d) {
			t.Errorf("bwrapRender's Hide list does not carry the crash store %s (ranger-base-s7tkc):\n  %v", d, r.Carve.Hide)
		}
		if bwIndex(r.Argv, "--tmpfs", d) < 0 {
			t.Errorf("the wall posse renders carries no `--tmpfs %s`, so a seat reads a same-uid peer's core or environment out of it (ranger-base-s7tkc):\n  %s", d, bwJoin(r.Argv))
		}
	}
}
