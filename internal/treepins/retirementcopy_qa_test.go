package treepins

// QA pin for ranger-base-y9b9p.
//
// Claim: INSTALL.md §7's "Upgrading an instance that has the generics?"
// paragraph is the only prose an upgrading operator reads about what
// `posse init` takes back out of `agents/`, and it described the identity
// test as byte-identity against THIS binary's example — the rule
// ranger-base-8ehw removed. Since 8ehw/rgx0 the live file's identity, and
// since ranger-base-788w the shelf slot's, both come from the table of every
// digest posse has shipped for that example (internal/posse/exampledigests.go,
// isShippedExample), so a generic seeded by ANY earlier release retires too.
//
// Read as it was written, the paragraph told exactly the population 8ehw was
// filed for — an operator upgrading a home an older posse seeded — that their
// generics would NOT be moved, on the homes where the retirement matters most.
//
// The doc claim is only true while the code judges that way, so this pins both
// ends: the paragraph must name the any-release rule and must not reinstate
// the embed-only one, and retireExamplePIDs must still ask isShippedExample
// about both copies it decides on.

import (
	"regexp"
	"strings"
	"testing"
)

// retirementAnchor opens the paragraph. It is the pin's subject: if it is
// gone, this test is measuring nothing and says so rather than passing.
const retirementAnchor = "**Upgrading an instance that has the generics?**"

// staleRetirementRule is the rule 8ehw removed, in the shapes that state it —
// a definite, singular example belonging to the running binary.
var staleRetirementRule = regexp.MustCompile(`(?i)\bthe shipped example\b|\bthis (binary|release)'s example\b`)

func TestUpgradeParagraphNamesEveryExamplePosseShipped(t *testing.T) {
	doc := readRepoFile(t, "INSTALL.md")
	i := strings.Index(doc, retirementAnchor)
	if i < 0 {
		t.Fatalf("INSTALL.md no longer contains %q — this pin has lost its subject. Repoint it at wherever the retirement is documented now, or drop it; it cannot pass by measuring nothing.", retirementAnchor)
	}
	para := doc[i:]
	if end := strings.Index(para, "\n\n"); end >= 0 {
		para = para[:end]
	}
	// Whitespace-collapsed, so a rewrap is not a failure and a reworded
	// claim is.
	flat := flattenLanding(para)

	if m := staleRetirementRule.FindString(flat); m != "" {
		t.Errorf(`INSTALL.md §7 says %q — that is the pre-ranger-base-8ehw rule, and it is not what init does.
The identity test is isShippedExample: every digest posse has shipped for that example, this release's and every earlier one's.
An operator upgrading a home an older posse seeded reads this paragraph and concludes their generics will not be moved.
  %s`, m, flat)
	}
	for _, want := range []string{
		"an example posse shipped",
		"any earlier one",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf(`INSTALL.md §7's retirement paragraph no longer says %q.
The audience for this paragraph IS the home seeded by an older posse (ranger-base-8ehw); the version-skew half of the rule is the half it is here to state.
  %s`, want, flat)
		}
	}
}

func TestRetirementJudgesBothCopiesAgainstTheShippedTable(t *testing.T) {
	src := readRepoFile(t, "internal/posse/init.go")
	i := strings.Index(src, "func (a *App) retireExamplePIDs(")
	if i < 0 {
		t.Fatal("internal/posse/init.go has no retireExamplePIDs — this pin has lost its subject; repoint it at the retirement's new home or drop it.")
	}
	body := src[i:]
	// The first column-0 closing brace ends the function; every brace inside
	// it is indented.
	if end := strings.Index(body, "\n}\n"); end >= 0 {
		body = body[:end]
	}
	if n := strings.Count(body, "isShippedExample("); n != 2 {
		t.Errorf(`retireExamplePIDs asks isShippedExample %d time(s), want 2: the live file (ranger-base-8ehw) and the shelf slot it moves onto (ranger-base-788w).
INSTALL.md §7 promises an operator that a generic byte-identical to any example posse has shipped is retired. A gate judged against the running binary's embed alone makes that promise false on every home an older posse seeded, and the promise is not correctable in place — a brew user reads the copy frozen at the tag.`, n)
	}
}
