//go:build posse_arm3

package posse

// THE CEILING'S BANNER MAY NOT TELL A REFUSED WRITER THAT CHECK 3 HAS NO
// MESSAGE ARM (ranger-base-d49fx, finding 2 of ranger-base-n8qwj).
//
// Both walls scan the commit MESSAGE — the ceiling's since ADR 0050 D2 as
// amended 2026-09-03 (ranger-base-pqlxr), check 3's since ADR 0024 D2 /
// ADR 0048 D2 as amended (ranger-base-1nbtn), through one messageArm. For
// one commit after that second landing the ceiling's rendered head still
// read "the ceiling's own and check 3 does not have it", which is
// REFUSAL-ADJACENT TEXT IN THE ARTIFACT ITSELF: a writer refused by check
// 3's message arm who opens the hook to learn the scope was told, in the
// hook, that check 3 has no such arm. Nothing pinned it either way.
//
// WHAT IS PINNED IS THE RENDER, not the source, because the render is what
// an operator reads: the doc comment at dataCeilingCheck and the header of
// dataceiling_qa_test.go carried the same sentence and were fixed with it,
// but they ship to nobody and a pin over source prose ages into a grep.
//
// FLATTENED FIRST, per ranger-base-z771z: the head is shComment-wrapped, so
// a per-line scan is blind to the phrase the moment a word moves across a
// newline. Measured there: a flatten that only collapsed whitespace left
// the wrapped mutant green.
//
// MUTATION-CHECKED (run through `go test -overlay`, worktree untouched):
// restoring the pre-fix sentence in gates.go's ceiling head reds the
// absence arm alone; deleting either message-arm banner reds the control
// arm alone. Runs recorded on ranger-base-d49fx.

import (
	"strings"
	"testing"
)

// TestQACeilingBannerDoesNotDenyCheckThreesMessageArm renders the hook with
// a ceiling class configured — the only configuration in which the ceiling
// block renders at all — and reads its prose two ways: the false claim is
// absent, and both message arms are present. Either arm alone would pass
// over an empty render.
func TestQACeilingBannerDoesNotDenyCheckThreesMessageArm(t *testing.T) {
	t.Parallel()
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{
		Extra:   []OpsPattern{{Class: "e", ERE: "zzz"}},
		Ceiling: []OpsPattern{{Class: "c", ERE: "yyy"}},
	}, IdentityLiteral{Class: "username", Value: "qa-fixture-operator"})

	var prose strings.Builder
	for _, line := range strings.Split(render, "\n") {
		prose.WriteString(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		prose.WriteString(" ")
	}
	flat := strings.Join(strings.Fields(strings.ToLower(prose.String())), " ")

	// ABSENCE: no wall may say the other one does not scan the message.
	for _, claim := range []string{
		"check 3 does not have",
		"an arm check 3 does not",
		"a third arm check 3 does not",
	} {
		if i := strings.Index(flat, claim); i >= 0 {
			t.Errorf("the rendered hook denies check 3's message arm (%q); both walls scan the commit MESSAGE and differ in the gate and the remedy, not the subject:\n\t...%s...",
				claim, flat[max(0, i-100):min(len(flat), i+100)])
		}
	}

	// CONTROL: both message arms really are in this render, so the absence
	// above is a swept claim and not an empty hook.
	for _, banner := range []string{
		"the data ceiling, third arm: the commit message",
		"check 3, third arm: the commit message",
	} {
		if !strings.Contains(flat, banner) {
			t.Errorf("control: the rendered hook is missing %q, so the absence assertion above measured nothing", banner)
		}
	}
}
