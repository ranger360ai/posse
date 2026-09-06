//go:build posse_arm2

package posse

// QA pin for ranger-base-5356: the public disclosure entry stays inside the
// SOFTWARE's posture.
//
// CHANGELOG.md is the one file in this repo written to be read by strangers
// who run this software, and the entry that lands there is a security
// disclosure — the shape most likely to reach for a deployment's own facts to
// make itself concrete. NOTES.md's "Privacy model" draws the line: posture of
// the software itself is public, posture of ONE deployment is not, and neither
// are cost, plan, or credential-location facts.
//
// OpsPatterns is that line's existing detector — the same list the beads
// visibility hook greps with. Pointing it at the changelog costs nothing and
// means the disclosure is held to the rule the beads are held to, rather than
// to whoever last edited it.
//
// OUT OF SCOPE ON PURPOSE (ranger-base-cqoj). The SCRUB BOUNDARY on
// ranger-base-5356 named two more classes: a seatbelt profile string (an
// `(allow file-write* (subpath "..."))` rule naming a real path) and a
// crew-wide Bash allowlist grant (`Bash(<cmd>:*)` stated as granted to every
// PID here). Neither has an OpsPatterns class and this pin cannot see them —
// a clean run below is not clearance for those two. That gap is deliberate,
// not unmeasured: the general case was ruled un-lintable on
// ranger-base-xsw5, because both shapes are ALSO the software's own public
// vocabulary (seatbelt.go emits the same rule shape for everyone;
// pidcheck.go:114's own doc comment uses Bash(posse:*) as its example), so
// the only tell is deixis — "this machine", "here", "the crew's" — and a
// source-wide grep on that hit ~20 legitimate uses to one violation. A
// pattern here would be the same trade. What covers this class instead: the
// human disclosure-scrub review that caught ranger-base-xsw5 (the security
// review process on ranger-base-5356), not an automated pin — read a
// changelog entry for those two shapes by eye before it ships.

import (
	"os"
	"strings"
	"testing"
)

func TestChangelogCarriesNoInstanceOpsContent(t *testing.T) {
	body, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)

	for _, p := range OpsPatterns {
		if p.Match(text) {
			t.Errorf("CHANGELOG.md carries %s content — %s.\nmatched: %q\n%s",
				p.Class, p.Why, p.MatchedText(text, 5), VisibilityRule)
		}
	}

	// The control. An assertion that nothing matched is satisfied by a
	// detector that matches nothing, so plant one line of each class the
	// entry could plausibly reach for and require the same loop to see it.
	for _, planted := range []struct{ line, class string }{
		{"the pass ran $715/wk against the ceiling", "cost"},
		{"keys via security find-generic-password -s some-item", "credential"},
	} {
		fired := ""
		for _, p := range OpsPatterns {
			if p.Match(text + "\n" + planted.line + "\n") {
				fired = p.Class
				break
			}
		}
		if fired != planted.class {
			t.Errorf("control: planting %q into the changelog fired %q, want %q — the scan above measured nothing",
				planted.line, fired, planted.class)
		}
	}
}

// The entry has to remain useful to the person deciding whether to upgrade,
// and that is a different failure from leaking: an entry scrubbed until it
// says nothing is also a disclosure that did not happen. The content pin lives
// with the release machinery (releasenotes_qa_test.go at the repo root); this
// one only asserts the file is here and is not empty, so a scrub that deleted
// it cannot pass as "no ops content found".
func TestChangelogIsNotEmpty(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(body))) < 200 {
		t.Fatalf("CHANGELOG.md is %d bytes — TestChangelogCarriesNoInstanceOpsContent would pass on an empty file", len(body))
	}
}
