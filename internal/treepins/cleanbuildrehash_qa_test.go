package treepins

// QA pin for ranger-base-la61c's build half, added while verifying its close
// under ranger-base-4giu8: the source scripts/clean-build.sh builds from has
// to arrive through a path that re-hashes it.
//
// baa19884 replaced this script's `git worktree add` of the shared repo with a
// `git init` plus a fetch into that fresh repo. git trusts an object's bytes
// by the name they are filed under and never re-hashes them on a local read,
// so a worktree add, a checkout or a cat-file serves whatever bytes sit in the
// object store under each SHA — and the binary `make install`, `make release`
// and `posse upgrade` produce is what the whole fleet then runs. A fetch goes
// through the pack protocol instead, where index-pack (or unpack-objects for a
// small pack) hashes every object it receives: bytes that do not hash to the
// name they were asked for under arrive under a different name, the commit's
// tree is then incomplete, and the fetch fails before anything is checked out.
//
// WHY THIS ARM AND NOT THE ONE NEXT DOOR. Reverting the fetch today already
// reds TestQABdPinDeclarationAndWiring in bdpin_qa_test.go — MEASURED
// 2026-11-15 by mutation, swapping `fetch --quiet --no-tags "$repo" HEAD` back
// to a `worktree add --detach`: it fails. But that assertion's subject is the
// bd-pin pre-flight's ORDERING, and it fails saying "the pin pre-flight must
// run BEFORE clean-build.sh fetches the source". A reader who hits that red
// learns nothing about re-hashing, and can satisfy it by moving one line back
// above the checkout they just reintroduced. This arm says what the property
// is, so the red names what it costs to lose.
//
// Named file, no enumeration: this reads scripts/clean-build.sh by name, so it
// is not in the tree-wide door class (treewidedoor_qa_test.go) and needs no
// Makefile door of its own. `make treepins-check` runs it.

import (
	"os"
	"strings"
	"testing"
)

func TestQACleanBuildFetchesItsSourceSoEveryObjectIsReHashed(t *testing.T) {
	body, err := os.ReadFile("scripts/clean-build.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)

	if !strings.Contains(s, `git -C "$src" fetch`) {
		t.Error("clean-build.sh no longer FETCHES its source. Every object has to " +
			"arrive through the pack protocol, which re-hashes it; a local `worktree " +
			"add`, `checkout` or `cat-file` of the shared repo serves whatever bytes " +
			"its object store holds under each SHA, so the binary `make install` " +
			"promotes to the whole fleet would be built from bytes nothing verified " +
			"(ranger-base-la61c)")
	}
	// The whole call, not the prefix `git init`: MEASURED while building this
	// pin, a mutant that spelled the verb `git initx` kept the substring and
	// survived the arm that was written to kill it.
	if !strings.Contains(s, `git init --quiet "$src"`) {
		t.Error("clean-build.sh fetches into something that is not a fresh `git " +
			"init` repo. A receiver that already holds these objects advertises them " +
			"as `have`, the sender sends nothing, and nothing is re-hashed " +
			"(ranger-base-la61c)")
	}
	// A refusal that is not fatal is the same build with a warning on it.
	if !strings.Contains(s, "did not verify") {
		t.Error("clean-build.sh no longer REFUSES on a failed fetch. A source that " +
			"did not verify must never reach `go build`, and the run must write no " +
			"binary (ranger-base-la61c)")
	}
	// The fetched tip must be the HEAD that was read, in full. Without this the
	// fetch is only proven to have delivered SOMETHING.
	if !strings.Contains(s, "rev-parse FETCH_HEAD") {
		t.Error("clean-build.sh does not compare FETCH_HEAD against the HEAD it " +
			"read. The fetch has to be proven to have delivered that commit and not " +
			"another (ranger-base-la61c)")
	}
}
