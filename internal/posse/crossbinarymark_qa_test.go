//go:build !posse_arm2 && !posse_arm3

package posse

// QA, ranger-base-6jj60 (verifying ranger-base-e6yct) — THE TWO MARKS THAT
// CROSS TWO BINARIES ARE PINNED AS LITERALS, and that is the whole point of
// this file: every other reader of them in this tree reads the const.
//
// ADR 0092 D4 amendment 3 and rule 4 both rest on a MARK on stdout rather
// than on an exit code: an installing `posse upgrade` asks the binary it has
// just swapped in a question, and reads the answer by the line's opening
// token. The asker is the OLD binary. So the spelling of that token is a
// WIRE FORMAT between two builds, and the one edit nothing in this tree can
// see is a rename of it — because the printer and every reader (and every
// pin) spell it `UpgradeClassMark` / `RenderProbeMark`, so both sides of
// each comparison move together and stay equal.
//
// MEASURED 2026-11-21, at 467da92a, by `go test -overlay` over a mutant
// whose only change is `const UpgradeClassMark = "upgrade-klass:"`:
//
//	internal/posse -run UpgradeClass                       ok   0.085s
//	internal/posse -run 'Upgrade|Probe|Help|Catalog|Mark'   ok  11.075s
//	cmd/posse -run TestQAUpgradeClassesFlagIsInTheBinary    ok   6.833s
//
// green — against nine other mutants of the same work, every one of which
// red at the pin whose name is the behaviour (the mark dropped from the
// line, the sort removed, a header added, the fail-closed word changed, the
// answer moved below D2's fence, the flag renamed out of cmd/posse's ladder,
// the `--classes <sha>` refusal cut, a line on stderr). The rename is the
// one change that would be read by another BUILD, and it was the one change
// nothing noticed.
//
// So this pin asserts the published spelling itself, in the two places a
// renamer has to pass: the constant, and the token the renderer actually
// writes. `render-probe:` is here beside it because the hole is the same one
// (it has carried the identical shape since amendment 3) and because the
// marks are one class — a reader of either is another binary.
//
// WHEN THIS REDS, the rename is not a refactor. A mark may only change
// spelling in a release that no longer has to answer the previous one, and
// what has to move with it is ADR 0092 (D4 rule 4 and amendment 3 both
// quote the token), cmd/posse's help catalog, and the reader in
// upgradeverify.go — none of which this file can see, which is why it names
// them here.

import (
	"strings"
	"testing"
)

func TestQACrossBinaryMarksKeepTheirPublishedSpelling(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name, got, want, who string
	}{
		{
			name: "UpgradeClassMark",
			got:  UpgradeClassMark,
			want: "upgrade-class:",
			who:  "`posse upgrade --classes`, read by an installing posse upgrade of the PREVIOUS build (ADR 0092 D4 rule 4)",
		},
		{
			name: "RenderProbeMark",
			got:  RenderProbeMark,
			want: "render-probe:",
			who:  "`posse gates render-probe`, read by upgradeRenderAsked in the PREVIOUS build (ADR 0092 D4 amendment 3)",
		},
	} {
		if m.got != m.want {
			t.Errorf("%s is %q, want %q — that token is the interface %s, and the binary doing the reading was built before this change: a rename makes its answer read as `not asked` and rolls the install back",
				m.name, m.got, m.want, m.who)
		}
	}

	// And the token the RENDERER writes, not only the constant it reads. A
	// mark kept in the const and dropped or respelled in the format string
	// is the same wire break one line over, and the pins that walk this
	// answer parse it with the const too (upgradeClassParse).
	lines := UpgradeClassLines()
	if len(lines) == 0 {
		t.Fatal("UpgradeClassLines is empty, so the arm below would assert nothing")
	}
	for _, ln := range lines {
		if !strings.HasPrefix(ln, "upgrade-class: ") {
			t.Errorf("a line of `posse upgrade --classes` opens %q and not with the published `upgrade-class: ` — the previous build's reader skips every line it cannot open, which it reports as `did not answer`", ln)
		}
	}
}
