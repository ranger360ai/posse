package rhq

// The table in exampledigests.go is posse's record of what posse shipped.
// A record nothing checks is a claim, so these are the checks: the current
// embed must be IN it, and it must hold every version this repo's history
// knows about. Both fail loudly and say which line to add, because the way
// this table goes wrong is silence — a missed version retires nothing on the
// homes that hold it (ranger-base-8ehw's leak, reopened).

import (
	"io/fs"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// TestEveryEmbeddedExamplePIDIsInTheShippedTable is the pin the next change
// to examples/agents trips. ranger-base-8zhr will change them.
func TestEveryEmbeddedExamplePIDIsInTheShippedTable(t *testing.T) {
	names := exampleAgentNames(posse.Seed)
	if len(names) == 0 {
		t.Fatal("the seed ships no example PIDs — this table has nothing to describe")
	}
	for _, n := range names {
		rel := path.Join("agents", n+".md")
		b, err := fs.ReadFile(posse.Seed, rel)
		if err != nil {
			t.Fatal(err)
		}
		if !isShippedExample(rel, b) {
			t.Errorf(`%s is not in shippedExampleDigests (exampledigests.go). APPEND this
line to its entry, never replace one — an entry that leaves the table is a
home posse can no longer recognise its own file in:

	%q: {
		...
		%q, // this commit
	},

Until it is there, no home holding this version retires it: init reads the
file as the operator's and keeps it (ranger-base-rgx0, ranger-base-8ehw).`,
				rel, rel, sha256Bytes(b))
		}
	}
}

// TestShippedExampleTableCoversEveryVersionInGitHistory is the other half:
// the table must not have forgotten a release. Every distinct content
// examples/agents/<name>.md has had in this repo is a version some home was
// seeded from. Skipped where there is no checkout to ask (a release tarball,
// a vendored build) — the table is still the thing that ships.
func TestShippedExampleTableCoversEveryVersionInGitHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not a git checkout")
	}
	dir := strings.TrimSpace(string(root))
	for _, n := range exampleAgentNames(posse.Seed) {
		rel := path.Join("agents", n+".md")
		gitPath := path.Join("examples", rel)
		out, err := exec.Command("git", "-C", dir, "rev-list", "HEAD", "--", gitPath).Output()
		if err != nil {
			t.Skipf("git rev-list %s: %v", gitPath, err)
		}
		revs := strings.Fields(string(out))
		if len(revs) == 0 {
			t.Skipf("no history for %s — shallow clone?", gitPath)
		}
		for _, rev := range revs {
			b, err := exec.Command("git", "-C", dir, "show", rev+":"+gitPath).Output()
			if err != nil {
				continue // the file was not at that path in that commit
			}
			if !isShippedExample(rel, b) {
				t.Errorf(`shippedExampleDigests is missing the version of %s that %s shipped:

		%q, // %s

Posse shipped those bytes, so a home seeded from that release holds them and
init must recognise them as its own. Append, never replace (exampledigests.go).`,
					rel, rev[:12], sha256Bytes(b), rev[:12])
			}
		}
	}
}

// isShippedExample answers about bytes, not about names: an example the
// operator changed by one byte is theirs, which is the rule the whole
// retirement turns on (ranger-base-qajs).
func TestIsShippedExampleRejectsAnEditedExample(t *testing.T) {
	rel := "agents/qa.md"
	b, err := fs.ReadFile(posse.Seed, rel)
	if err != nil {
		t.Fatal(err)
	}
	if isShippedExample(rel, append(append([]byte{}, b...), '\n')) {
		t.Error("an edited example matched the shipped table — the digest test is not comparing bytes")
	}
	if isShippedExample("agents/not-an-example.md", b) {
		t.Error("a name posse ships no example for matched the shipped table")
	}
}
