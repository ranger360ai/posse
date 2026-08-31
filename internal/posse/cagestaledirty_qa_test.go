package posse

// ranger-base-b6fh, found verifying ranger-base-nwj7 (ranger-base-7sq9).
//
// cagestale.go's own header says its whole point is to prevent a false
// CURRENT verdict. SourceBuildStamp is shortSha(HEAD) + "-dirty" — a bool,
// not a content hash — so two DIFFERENT dirty tree states at the SAME HEAD
// stamp identically, and CageAgeVsSource/CageAgeVsPosse read them as one
// build. Reachable in practice as: `posse cage build` from a dirty tree,
// then further edits to the same uncommitted files before the next `posse
// cage` / live pin run, no commit in between.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQACageStaleIsBlindToWhichDirtyEditIsThere(t *testing.T) {
	src := tempGitTree(t)
	f := filepath.Join(src, "f")

	// "the image is built" against this dirty state.
	if err := os.WriteFile(f, []byte("x\ndirty edit A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stampAtBuild := SourceBuildStamp(src)
	versionAtBuild := SourceBuildVersion(src)
	if stampAtBuild == "" {
		t.Fatal("expected a dirty stamp, got empty")
	}

	// The operator keeps editing, uncommitted, same HEAD — a DIFFERENT
	// render than what the image above carries.
	if err := os.WriteFile(f, []byte("x\ndirty edit A\nAND MORE, DIFFERENT RENDER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	versionNow := SourceBuildVersion(src)

	got := cageAge("img", "this source", versionAtBuild, versionNow)
	if got.State == CageImageCurrent {
		t.Errorf("false CURRENT: image built at dirty state A (%s) reads current against a since-edited dirty source (%s) — %s",
			versionAtBuild, versionNow, got)
	}
}
