package posse_test

// QA pin for ranger-base-qlrx, precondition 4 of docs/runbooks/release.md.
//
// The tarball ships README.md and INSTALL.md, the formula installs them into
// `doc/`, and its caveats send the reader there — so THE DOCUMENTATION A BREW
// USER READS IS FROZEN AT THE TAG. A version claim that is stale when the tag
// is cut cannot be corrected in place; the fix rides the next release.
//
// v0.3.0 shipped with nine such claims, all of them saying what `posse
// version` prints, all of them written by hand, and nothing red when the
// const moved. This is the mechanical half of that precondition: it does not
// judge whether the install sequence is right (a human reads that), only that
// every version the two pages promise is the version this source IS.

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
)

// A backticked token that is entirely a version: `0.4.0`, `v0.4.0`,
// `0.4.0+<sha>`, `0.4.0+<sha>[-dirty]`. Deliberately anchored at both ends,
// which is what keeps the OTHER tools' versions on these pages out of it —
// the pinned beads release, Homebrew 6.0.19 and claude 2.1.250 are named in
// running prose, never as a bare backticked version standing alone.
var docVersionToken = regexp.MustCompile("`(v?)([0-9]+\\.[0-9]+\\.[0-9]+)((?:\\+<sha>[^`]*)?)`")

func TestReaderDocsPromiseThisSourcesVersion(t *testing.T) {
	// A count, not just an absence: a regex that stopped matching would
	// otherwise pass this test by measuring nothing, and the failure it is
	// here to catch is exactly a version claim going unnoticed.
	// Nine at v0.3.0, plus INSTALL.md §2's `brew info` expectation
	// (ranger-base-63q3): the version brew RESOLVES is a promise of exactly
	// this kind, and on a brew older than 6.0.14 it is the one that 404s.
	const wantSites = 10

	sites := 0
	for _, f := range []string{"INSTALL.md", "README.md"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			for _, m := range docVersionToken.FindAllStringSubmatch(line, -1) {
				sites++
				if m[2] != rhq.Version {
					t.Errorf("%s:%d promises `%s` but this source is %s — a brew user reads the copy frozen at the tag, and it cannot be corrected in place.\n  %s",
						f, i+1, m[1]+m[2]+m[3], rhq.Version, strings.TrimSpace(line))
				}
			}
		}
	}
	if sites != wantSites {
		t.Errorf("scanned %d version claims across INSTALL.md and README.md, expected %d.\n"+
			"If a claim was deliberately added or removed, move this number. If it dropped to 0, the pattern stopped matching and this test is measuring nothing.", sites, wantSites)
	}
	t.Logf("scanned %d version claims across INSTALL.md and README.md against %s", sites, rhq.Version)
}
