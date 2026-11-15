package treepins

// QA pin for ranger-base-9lsg2: the source scripts/release-artifacts.sh builds
// the release from has to arrive through a path that re-hashes it — the same
// property cleanbuildrehash_qa_test.go pins for scripts/clean-build.sh.
//
// This script used to read its build source with `git worktree add --detach`
// out of the repo it runs in. git trusts an object's bytes by the name they are
// filed under and never re-hashes them on a local read, so a worktree add
// serves whatever bytes the object store holds under each SHA — and this script
// mints the release tarballs, the bottles, checksums.txt and the Homebrew
// formula from them, so the checksums would certify bytes nothing verified. A
// fetch into a fresh repo goes through the pack protocol, whose receiving side
// hashes every object it is sent: bytes that do not hash to their name leave
// the commit's tree incomplete and the fetch fails before anything is built.
//
// Named file, no enumeration: this reads scripts/release-artifacts.sh by name,
// so it is not in the tree-wide door class (treewidedoor_qa_test.go) and needs
// no Makefile door of its own. `make treepins-check` runs it.

import (
	"os"
	"strings"
	"testing"
)

func TestQAReleaseArtifactsFetchesItsSourceSoEveryObjectIsReHashed(t *testing.T) {
	body, err := os.ReadFile("scripts/release-artifacts.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)

	// Comment lines say why the worktree add went; only CODE may not carry it.
	for i, line := range strings.Split(s, "\n") {
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "#") {
			continue
		}
		if strings.Contains(code, "worktree add") {
			t.Errorf("scripts/release-artifacts.sh:%d checks its build source out with "+
				"`git worktree add`, a local read that never re-hashes an object, so the "+
				"release and the checksums that certify it would be built from whatever "+
				"bytes the object store holds (ranger-base-9lsg2): %s", i+1, code)
		}
	}
	if !strings.Contains(s, `git -C "$src" fetch`) {
		t.Error("release-artifacts.sh no longer FETCHES its source. Every object has " +
			"to arrive through the pack protocol, which re-hashes it (ranger-base-9lsg2)")
	}
	// The whole call, not the prefix `git init`: cleanbuildrehash_qa_test.go
	// measured a `git initx` mutant surviving a prefix match.
	if !strings.Contains(s, `git init --quiet "$src"`) {
		t.Error("release-artifacts.sh fetches into something that is not a fresh " +
			"`git init` repo. A receiver that already holds these objects advertises " +
			"them as `have`, the sender sends nothing, and nothing is re-hashed " +
			"(ranger-base-9lsg2)")
	}
	if !strings.Contains(s, "did not verify") {
		t.Error("release-artifacts.sh no longer REFUSES on a failed fetch. A source " +
			"that did not verify must never reach `go build` (ranger-base-9lsg2)")
	}
	// The fetch must also come BEFORE the first build, or the refusal guards
	// nothing.
	fetch := strings.Index(s, `git -C "$src" fetch`)
	build := strings.Index(s, `build \`)
	if fetch >= 0 && build >= 0 && fetch > build {
		t.Error("release-artifacts.sh fetches its source AFTER the first `go build`; " +
			"the verified fetch has to come first (ranger-base-9lsg2)")
	}
	if !strings.Contains(s, "rev-parse FETCH_HEAD") {
		t.Error("release-artifacts.sh does not compare FETCH_HEAD against the commit " +
			"it resolved. The fetch has to be proven to have delivered that commit " +
			"and not another (ranger-base-9lsg2)")
	}
}
