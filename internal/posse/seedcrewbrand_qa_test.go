package posse

// ranger-base-ungb (QA, verifying the close of rangerhq-dh5g).
//
// dh5g took this instance's crew brand out of the identity line of all nine
// shipped example PIDs — "of the Ranger crew." became "of the crew." — so a
// fresh deployer's seed no longer names the development instance's crew.
// The close is sound. Nothing pinned it.
//
// MEASURED, not assumed. With the brand put back in all nine files and the
// nine resulting digests APPENDED exactly as exampledigests.go's own
// contract instructs, the whole suite is green:
//   - the digest table (TestEveryEmbeddedExamplePIDIsInTheShippedTable)
//     notices BYTES, and the contract's own remedy for changed bytes is to
//     append the new digest, which is what a brand edit would do;
//   - identityLineRe (pidcheck.go:25) lints the identity line's SHAPE, and
//     "You are the Developer of the Ranger crew." satisfies it;
//   - seedcrewo7y4_qa_test.go pins the other half of the same rule, a
//     personal NAME, and a crew brand is not one.
// So the brand could come back on any edit and every test would still pass.
//
// Self-contained (own reader, own enumeration): a pin an edit next door can
// carry away pinned nothing.

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// noBrandTail is the ending dh5g chose and pidcheck.go:19-23 documents as
// the no-name shape ("You are the QA engineer of the crew."). An instance
// puts its own crew in that slot when it copies the file; the file posse
// SHIPS may not arrive with one already in it.
const noBrandTail = " of the crew."

// thisInstancesCrew is the brand dh5g removed, matched case-insensitively.
// It is the development instance's, and ADR 0012 D2's rule is that a seed
// carries no instance particular of ours.
const thisInstancesCrew = "ranger crew"

// seedExamplePIDs is every example PID the binary would seed a fresh home
// with, read straight out of the embedded FS — the bytes that actually ship,
// not the worktree's copy of them.
func seedExamplePIDs(t *testing.T) map[string]string {
	t.Helper()
	ents, err := fs.ReadDir(posse.Seed, "agents")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := fs.ReadFile(posse.Seed, "agents/"+e.Name())
		if err != nil {
			t.Fatal(err)
		}
		out["agents/"+e.Name()] = string(b)
	}
	if len(out) < 9 {
		t.Fatalf("the seed ships %d example PID(s) — this pin has nothing to read", len(out))
	}
	return out
}

// seedIdentityLine is the PID's opening claim, found the way a reader finds
// it: the first line that opens "You are ". Deliberately not pidcheck.go's
// identityLine — a pin that reads the file through the thing it is pinning
// goes quiet when that thing changes its mind.
func seedIdentityLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "You are ") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// TestNoShippedExamplePIDNamesACrew is dh5g itself: the identity line ends
// in the no-brand form, for every example, in the bytes that ship.
func TestNoShippedExamplePIDNamesACrew(t *testing.T) {
	for path, body := range seedExamplePIDs(t) {
		line := seedIdentityLine(body)
		if line == "" {
			t.Errorf("%s: no identity line — a PID opens with one (pidcheck.go)", path)
			continue
		}
		if !strings.HasSuffix(line, noBrandTail) {
			t.Errorf(`%s opens %q.
A shipped example may not arrive naming a crew: it is copied and RUN as-is
(INSTALL.md), so whatever crew it names becomes the deployer's crew, and the
one it would name is ours. rangerhq-dh5g took %q out of all nine identity
lines; end the line %q and let the instance edit it.
If a later bead deliberately changes the no-name form, change it here too —
this pin is the only thing that notices, and the digest table will not.`,
				path, line, "of the Ranger crew.", noBrandTail)
		}
	}
}

// TestNoShippedExamplePIDCarriesThisInstancesCrewBrand is the same rule over
// the whole file rather than the one line. The closer swept for this by hand
// at close time and the sweep was clean; a hand sweep runs once.
func TestNoShippedExamplePIDCarriesThisInstancesCrewBrand(t *testing.T) {
	for path, body := range seedExamplePIDs(t) {
		if strings.Contains(strings.ToLower(body), thisInstancesCrew) {
			t.Errorf(`%s carries %q.
That is this development instance's crew brand, and the seed is a fresh
deployer's starting point (ADR 0012 D2: no instance particular of ours ships
in it). rangerhq-dh5g removed it; it is back.`, path, thisInstancesCrew)
		}
	}
}
