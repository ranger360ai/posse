//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-e6yct — ADR 0092 D4 rule 4, `posse upgrade
// --classes`: the partition this binary compiled in, printed for the OLD
// binary that is deciding whether to keep it.
//
// The three claims, and each is a direction the answer could go wrong while
// the install that reads it stays green:
//
//	the answer   one marked line per row of upgradeConditionSource, with
//	             this binary's own class for it, sorted, and nothing else.
//	             A line the reader cannot parse is a row it will class
//	             `binary` and trip on (rule 5).
//	the words    the three the ADR publishes — shop, binary, install — and
//	             a class with no word of its own answers `binary`, which is
//	             the answer that trips rather than the one that excuses.
//	the position FIRST in the verb, above D2's fence and above every
//	             reading. The asker is another binary's `posse upgrade`
//	             mid-install, holding the box lock and knowing nothing about
//	             this home's `install_delegate:`; a refusal here reads as
//	             "did not answer", which rolls back the install that put
//	             this binary in place.
//
// The cross-binary half — that the flag is in the BUILT binary at all, and
// what a binary without it answers — is cmd/posse's
// TestQAUpgradeClassesFlagIsInTheBinary, because it needs a subprocess.

import (
	"bytes"
	"io"
	"sort"
	"strings"
	"testing"
)

// upgradeClassParse splits the verb's answer into (row, word) pairs, failing
// on any line that is not exactly `upgrade-class: <row> <word>` — three
// fields and the mark first. The reader rule 5 describes does the same walk
// over another binary's stdout, so a shape this refuses is a shape that
// reaches that reader as silence.
func upgradeClassParse(t *testing.T, out string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Fields(ln)
		if len(f) != 3 || f[0] != UpgradeClassMark {
			t.Fatalf("a line of the answer is not `%s <row> <class>`: %q", UpgradeClassMark, ln)
		}
		if was, dup := got[f[1]]; dup {
			t.Fatalf("row %s is answered twice (%s then %s) — a reader keyed by row would take whichever came last", f[1], was, f[2])
		}
		got[f[1]] = f[2]
	}
	return got
}

func TestUpgradeClassesIsTheWholeTableAndNothingElse(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := upgradeWriteClasses(&out); err != nil {
		t.Fatalf("upgradeWriteClasses: %v", err)
	}

	// The floor first: a table that emptied would make every claim below
	// vacuous, and `--classes` printing nothing is exactly what an old
	// binary prints (MEASURED — see cmd/posse's pin).
	if n := len(upgradeConditionSource); n < 12 {
		t.Fatalf("upgradeConditionSource has %d entries — too few for this pin to be measuring the partition at all", n)
	}

	got := upgradeClassParse(t, out.String())
	if len(got) != len(upgradeConditionSource) {
		t.Errorf("the answer carries %d row(s), want the table's %d:\n%s", len(got), len(upgradeConditionSource), out.String())
	}
	for row, class := range upgradeConditionSource {
		word, ok := got[row]
		if !ok {
			t.Errorf("the answer never names %s, so the binary reading it has no class for a row this binary PRINTS — rule 5 would class it `binary` and trip on it", row)
			continue
		}
		if want := upgradeClassWord(class); word != want {
			t.Errorf("the answer classes %s %s, want %s", row, word, want)
		}
	}

	// Sorted, which is what makes two runs byte-identical and therefore two
	// binaries' answers comparable by eye.
	rows := make([]string, 0, len(got))
	for _, ln := range UpgradeClassLines() {
		rows = append(rows, strings.Fields(ln)[1])
	}
	if !sort.StringsAreSorted(rows) {
		t.Errorf("the rows are not sorted: %v", rows)
	}

	// `and does nothing else`: no header, no count, no trailing summary.
	// Asserted as a line count against the table, because the parse above
	// would have failed on prose and a pin that only parses cannot see a
	// line that happens to parse.
	if n := len(strings.Split(strings.TrimRight(out.String(), "\n"), "\n")); n != len(upgradeConditionSource) {
		t.Errorf("the answer is %d line(s) over a table of %d — something else is being printed:\n%s", n, len(upgradeConditionSource), out.String())
	}
	if strings.Contains(out.String(), "\n\n") {
		t.Errorf("the answer carries a blank line:\n%q", out.String())
	}
}

// The vocabulary is the interface. The constants are iota values and could
// not cross two binaries even in principle, so what rule 5 reads is these
// three words — ADR 0092 D4 rule 4 spells them `<shop|binary|install>`.
func TestUpgradeClassWordIsTotal(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		class upgradeCondClass
		want  string
	}{
		{upgradeShopState, "shop"},
		{upgradeBinaryRendered, "binary"},
		{upgradeInstallSubject, "install"},
	} {
		if got := upgradeClassWord(c.class); got != c.want {
			t.Errorf("upgradeClassWord(%d) = %q, want %q — the word is what another binary reads, so renaming one is an interface change (ADR 0092 D4 rule 4)", c.class, got, c.want)
		}
	}

	// Every class the table actually uses has a word. The three above are
	// the whole enumeration today; this arm is what says so over the table
	// rather than over the constants, so a fourth class used by one row is
	// caught by its own row.
	words := map[string]bool{"shop": true, "binary": true, "install": true}
	for row, class := range upgradeConditionSource {
		if !words[upgradeClassWord(class)] {
			t.Errorf("%s is classed %d, which prints %q — not one of the three words ADR 0092 D4 rule 4 publishes", row, class, upgradeClassWord(class))
		}
	}

	// And the fail-closed default is a DECISION: a class nobody taught this
	// printer about answers `binary`, the answer that trips. The other way
	// round — a silent `shop` or an empty field — would let a new binary
	// excuse a row on nothing but the printer's ignorance.
	if got := upgradeClassWord(upgradeInstallSubject + 1); got != "binary" {
		t.Errorf("an unknown class prints %q, want \"binary\" — rule 5 excuses `shop` and `install`, so any other default excuses a row nobody classified", got)
	}
}

// Rule 4's position in the verb: above D2's fence and above every reading.
//
// The arm is the one D2 refuses hardest — a persona under a config with no
// `install_delegate:` at all, which upgradeFence refuses for every persona
// rather than only for an unnamed one. The upgradeSeams order() is how this
// file's neighbour pins "reads nothing": a reading taken on this path is a
// lock claim or a git read inside a verb the ADR says takes neither.
func TestUpgradeClassesAnswersBeforeTheFenceAndReadsNothing(t *testing.T) {
	a := wtApp(t)
	write(t, a.ConfigPath, "operator: someone\n") // no install_delegate:
	t.Setenv(EnvPersona, "wheelwright")

	s := &upgradeSeams{}
	var out bytes.Buffer
	if err := a.cmdUpgrade(&out, UpgradeOpts{Classes: true}, s.reads()); err != nil {
		t.Fatalf("`posse upgrade --classes` was refused under a persona no install_delegate: names: %v\nAn installing binary reads a refusal as `did not answer`, and rule 5 trips on that — so this fence would roll back the install that delivered it.", err)
	}
	if s.order() != "" {
		t.Errorf("the flag took the %s reading(s) — rule 4 takes no lock, reads no git and names no worktree", s.order())
	}
	if got, want := out.String(), strings.Join(UpgradeClassLines(), "\n")+"\n"; got != want {
		t.Errorf("the verb printed something other than the partition:\n got %q\nwant %q", got, want)
	}

	// The control: the same call WITHOUT the flag is refused by the fence,
	// over the same App and the same persona. Without this arm a verb that
	// had lost its fence entirely would pass the claim above.
	if err := a.cmdUpgrade(io.Discard, UpgradeOpts{}, s.reads()); err == nil {
		t.Fatal("control: the ordinary verb ran under a persona with no install_delegate: — the fence is gone, so this file proves nothing about where --classes sits relative to it")
	}
}
