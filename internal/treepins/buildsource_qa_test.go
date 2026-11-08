package treepins

// QA pins for ranger-base-eiwhk — the two build lines that stamp WHERE this
// binary was built, and the one that must not.
//
// THE SILENT DIRECTION, and it is the same one ranger-base-ctath's pin exists
// for. posse's freshness reading looks for its own checkout in `beads:` and
// the process cwd, and on an instance whose crew works on something other
// than posse neither of those ever names a posse checkout — so `posse gates`
// and `posse status` read "UNKNOWN whether this binary carries the wall
// renders and the launch-time checks that landed" permanently, with the
// checkout `make install` was run from sitting on the same disk. The fix is
// one extra `-X` on two build lines: internal/posse.BuildSource, which
// internal/posse/launcherlag.go reads as the LAST candidate.
//
// Drop that flag and the defect comes straight back, invisibly: every Go test
// in the tree still passes (BuildSource's default is "" and every test binary
// is already unstamped), the binary builds, installs and runs, and the only
// place it shows is `posse gates` on a box whose queue is not posse. That is
// the direction no unit arm can hold, which is why it is held against the
// build lines' own text.
//
// AND IT HAS TO BE THE RIGHT SYMBOL. `go build -ldflags "-X <symbol>=<v>"` on
// a symbol that does not exist is SILENT — exit 0, the variable keeps its
// default (cagestale_test.go measured it for the Build stamp) — so the lines
// are compared against internal/posse.BuildSourceSymbol rather than against a
// second copy of the string. The other half of that loop is executable and
// lives in cmd/posse/gatesbuildsource_qa_test.go, which builds through that
// constant and reads the state back off `posse gates`: together they say the
// constant names a real symbol AND the build lines stamp that constant.
//
// Named files, no enumeration: this reads four build-line files by name, so
// it is not in the tree-wide door class (treewidedoor_qa_test.go) and needs
// no Makefile door — releasestamp_qa_test.go's note, for the same reason.

import (
	"os"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// readTreeFile is a tracked file as the tree holds it. Not a fixture: what is
// pinned is what `make install` actually runs.
func readTreeFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(body)
}

// `make build` stamps it, through the symbol the Go side names, from git's own
// answer, and only when that answer is a single word.
func TestQATheDevBuildStampsTheBuildSource(t *testing.T) {
	body := readTreeFile(t, "Makefile")
	want := "-X " + posse.BuildSourceSymbol + "=$(BUILD_SOURCE)"
	if !strings.Contains(body, want) {
		t.Fatalf("the Makefile does not carry %q — a dev build then names no source dir and the reading is back to `beads:` plus the cwd (ranger-base-eiwhk)", want)
	}
	// From git, not from $(PWD) or $(CURDIR): the reading resolves a
	// candidate through MainCheckout, and `rev-parse --show-toplevel` is
	// the answer that is a repository root rather than wherever make was
	// invoked from.
	if !strings.Contains(body, "BUILD_SOURCE := $(shell git rev-parse --show-toplevel") {
		t.Error("BUILD_SOURCE is no longer git's own answer for the checkout root")
	}
	// THE WHITESPACE GUARD. go splits the -ldflags VALUE on spaces, so a
	// checkout path containing one would become a second linker flag — the
	// shape ranger-base-ctath hardened the release script against. `words`
	// is the guard, and it also covers the empty (not-a-checkout) case.
	if !strings.Contains(body, "ifeq ($(words $(BUILD_SOURCE)),1)") {
		t.Error("the $(words …) guard is gone: a checkout path with a space in it now reaches the -ldflags string as a second flag")
	}
}

// `make release` — and so `make install`, which is how the fleet's binary is
// built and the install the bead was filed against — stamps the SOURCE
// checkout and not the throwaway worktree it compiles in.
func TestQATheCleanBuildStampsTheSourceCheckoutAndNotItsTempWorktree(t *testing.T) {
	body := readTreeFile(t, "scripts/clean-build.sh")
	want := "-X " + posse.BuildSourceSymbol + "=$repo"
	if !strings.Contains(body, want) {
		t.Fatalf("scripts/clean-build.sh does not carry %q — `make install` is the build the bead was filed against, so without this the installed binary reads UNKNOWN forever on any box whose queue is not posse (ranger-base-eiwhk)", want)
	}
	// $src is `git worktree add`'s detached checkout of HEAD and is REMOVED
	// by this script's own trap on the way out. Stamping it would bake a
	// path that is guaranteed not to exist by the time anything reads it —
	// an answer that looks like a fix and degrades to the same UNKNOWN.
	if strings.Contains(body, posse.BuildSourceSymbol+"=$src") {
		t.Error("the stamp names $src, the temp worktree this script deletes on exit — it must name $repo, the checkout the operator still has")
	}
	// Beside the Build stamp, in the same -ldflags value: the reading
	// refuses a candidate that does not hold the commit the binary names,
	// so a source dir with no sha beside it answers nothing.
	if !strings.Contains(body, "-X github.com/ranger360ai/posse/internal/posse.Build=$sha") {
		t.Error("the sha stamp is gone from the ldflags this script composes, so the source dir has no commit to be checked against")
	}
	// The same whitespace refusal as the Makefile's, in sh.
	if !strings.Contains(body, "*[[:space:]]*)") {
		t.Error("the whitespace refusal is gone: a checkout path with a space would become a second linker flag")
	}
}

// NOTHING THAT SHIPS MAY STAMP IT, which is the other half of the fix being
// honest rather than merely green.
//
// A release tarball is cut from a throwaway worktree on a machine the reader
// does not have (internal/posse/releasestamp.go), so the path would be dead
// on arrival — and a published binary has no business carrying a stranger's
// home directory. A release's answer to "are my walls current" is its TAG,
// which it already has. The cage's inner binary is the same argument from the
// other side: it runs in a container where a host path means nothing.
//
// A FLOOR, and read as one: these are the build lines the tree has today,
// named rather than enumerated, for releasestamp_qa_test.go's reason — a
// stamp added to another build line is a visible, deliberate edit to a line
// somebody is already reading, while the flag quietly going missing is the
// direction that ships.
func TestQANoShippedOrCagedBuildLineStampsTheBuildSource(t *testing.T) {
	for _, path := range []string{
		"scripts/release-artifacts.sh", // the four cross-compiled tarballs and the tap
		"internal/posse/cagestale.go",  // the cage's inner binary, built for a container
		".github/workflows/ci.yml",     // every branch build CI makes
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("cannot read %s: %v", path, err)
			continue
		}
		if strings.Contains(string(body), posse.BuildSourceSymbol) {
			t.Errorf("%s names %s — a binary that leaves this box must not carry the directory it was built in, and the path would be dead wherever it is read (ranger-base-eiwhk)", path, posse.BuildSourceSymbol)
		}
	}
}
