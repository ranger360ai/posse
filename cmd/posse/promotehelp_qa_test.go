package main

// ranger-base-b22vq, the half of the sweep that lives in the usage catalog.
//
// `posse` usage told an operator that promote copies "agents/, config.yaml,
// recipes/, skills/" — four of the five paths it copies, from the day ADR
// 0039 D2 (ranger-base-ight8) added `runtimes` to the promoted set. A reader
// concluded that a home `runtimes/` was untouched by promote. It is not:
// promote copies out what the constitution commit carries and REMOVES what
// it does not, so a hand-placed overlay leaves — which the CHANGELOG's
// Upgrading note said and the verb's own help did not.
//
// The block now renders posse.PromotedProse, so the next member widens it in
// the same edit, and it names the removal. This is the reader that stops the
// next one going stale.
//
// The set is SPELLED OUT here rather than taken from posse.PromotedPaths for
// the reason constitutionClassSpec is (ranger-base-ak3e): a case list
// generated from the thing under test shrinks with it silently. Dropping a
// member would delete its own case and pass.

import (
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// promoteHelpSpec is the promoted set as the catalog must name it — the twin
// of internal/posse's promotedProseSpec, on the other side of the package
// boundary.
var promoteHelpSpec = []string{"agents/", "config.yaml", "recipes/", "runtimes/", "skills/"}

// promoteBlock is the catalog's `posse promote` entry: its header line and
// every continuation under it, up to the next command header.
func promoteBlock(t *testing.T, out string) string {
	t.Helper()
	var b strings.Builder
	in := false
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "  posse ") {
			if in {
				break
			}
			in = strings.HasPrefix(ln, "  posse promote ")
		}
		if in {
			b.WriteString(ln)
			b.WriteString("\n")
		}
	}
	if b.Len() == 0 {
		t.Fatal("the usage catalog no longer has a `posse promote` entry — this pin is reading nothing")
	}
	return b.String()
}

func TestPromoteHelpNamesTheWholePromotedSet(t *testing.T) {
	block := promoteBlock(t, helpText(t))

	// The ENUMERATION span, not the whole block. Measured: with the whole
	// block as the subject this pin stayed GREEN under the mutation it
	// exists to catch (`runtimes` dropped from posse.PromotedPaths), because
	// the removal sentence four lines below names `runtimes/` as its worked
	// example and Contains found that instead.
	const from, to = "the promoted set —", "— from the constitution repo"
	i := strings.Index(block, from)
	j := strings.Index(block, to)
	if i < 0 || j <= i {
		t.Fatalf("the promote entry no longer enumerates the promoted set between %q and %q:\n%s", from, to, block)
	}
	list := block[i+len(from) : j]
	for _, m := range promoteHelpSpec {
		if !strings.Contains(list, m) {
			t.Errorf("`posse promote`'s help tells an operator the verb copies a set without %q:\n%s", m, list)
		}
	}
	// The join to the code's own answer, so this file's hand-written spec
	// cannot drift from the set the binary actually promotes. The spec is
	// what makes the loop above measure something; this is what makes the
	// spec honest.
	if got, want := posse.PromotedProse(""), strings.Join(promoteHelpSpec, ", "); got != want {
		t.Errorf("the promoted set renders as %q and this file specifies %q — widen promoteHelpSpec in the same edit", got, want)
	}
}

// The other half of the same defect: the help enumerated what promote never
// touches (envs/, state/, personas/) and said nothing about what it takes
// AWAY, so a home `runtimes/` read as untouched either way.
func TestPromoteHelpSaysPromoteRemovesWhatTheCommitDoesNotCarry(t *testing.T) {
	block := promoteBlock(t, helpText(t))
	for _, want := range []string{"REMOVES", "printing each removal", "runtimes/"} {
		if !strings.Contains(block, want) {
			t.Errorf("`posse promote`'s help does not carry %q — an uncommitted overlay at the home leaves, loudly, and the verb's own help is where that belongs:\n%s", want, block)
		}
	}
	// The worked example has to stay a member of the set it illustrates, and
	// that is asked of the CODE, not of this file's spec: a loop over
	// promoteHelpSpec here would be a literal checking a literal and could
	// never fail. If `runtimes` ever leaves the promoted set this sentence
	// is the next thing to go stale, and this is what says so.
	found := false
	for _, m := range posse.PromotedPaths {
		if m == "runtimes" {
			found = true
		}
	}
	if !found {
		t.Error("the help's worked example names runtimes/ and the promoted set no longer holds it — rewrite the example with the removal rule intact")
	}
}
