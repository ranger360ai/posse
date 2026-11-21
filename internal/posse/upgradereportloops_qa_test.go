//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-r1zdr — the WIRING of the two loop lines `posse
// upgrade` ends its report with, asserted through upgradeRunPlan where the D4
// pins already are.
//
// WHY THIS FILE EXISTS. ADR 0092 D8's claim is one sentence: "Both lines
// print, and neither claims the other's reach." Everything that renders those
// lines was witnessed — eight mutants over boxloops.go each red their own pin,
// and watchimage_qa_test.go holds the per-instance side — and the wiring that
// reaches the renderers at all was held by nothing. The verify lane measured
// it over ranger-base-is1qp's close (ranger-base-msir3, filed as this bead);
// re-measured here on 2026-11-21, linux 7.2.15-1-omarchy, over the 98 tests
// `^Test(QA)?(Live)?Box(Loop|Census|Proc)|^TestQATheVerbsBoxLine|^TestLiveBoxCensus|^TestUpgrade`
// selects, with upgrade.go restored byte-identical after each arm:
//
//	B1   delete `plan.Box  = func() BoxReading { … }`        → rc=0, 3.174s
//	B1L  delete `plan.Loop = func() (WatchImage, bool) { … }` → rc=0, 3.248s
//	B2   make the box line's print unreachable               → rc=0, 3.367s
//	B2L  make the per-instance line's print unreachable      → rc=0, 3.360s
//
// Four ways for that sentence to become false silently, and the two the bead
// names are B1 and B2. A report whose per-instance line is the only loop
// reading in it reads as an all-clear for the box — which is the exact hole
// ranger-base-is1qp exists to close, so losing the line loses the close.
//
// Every one of the four reds a pin in this file now, each naming its own
// line, and so do two more that a thinner pin over the same gap would have
// let through — the ones that keep a line in the report and change what is
// IN it:
//
//	B3  read the box against `a.Home` rather than `plan.Target`  → the
//	    subject rule, D8's own: the file the install renames is box-wide.
//	B4  print `upgradeWatchLine` in the box line's slot          → two lines,
//	    one reading, and the wider half of the coverage silently gone.
//
// WHAT THESE PINS ARE ABOUT, said because the renderers' pins are one file
// over and the two are easy to confuse: nothing here asserts what either
// sentence SAYS beyond the clause that is each line's own reach. The subject
// is that a run prints both, in that order, each through its own reader, and
// that a plan carrying no reader prints neither.

import (
	"strings"
	"testing"
)

// r1zdrLoopLines finds the report's two loop lines by the prefix each renderer
// owns, and says where each stood — because the ORDER is part of the claim
// (ADR 0092 D8: the box line comes after the per-instance one, not instead of
// it, and both come before D6's record line).
//
// Prefix-matched on a whole line and not `strings.Contains`: both lines are
// printed unindented and everything else in the report is either indented or
// a `──` banner, so a prefix is the one key that cannot match a condition
// detail quoting one of these sentences back. `dispatch (box): ` is not
// prefixed by `dispatch: `, so the two keys do not overlap.
//
// A COUNT AND NOT A FIND: a report that printed one renderer twice would
// satisfy "both lines are here" and have lost a reading, so each line is
// required to appear exactly once.
func r1zdrLoopLines(t *testing.T, report string) (watch, box string, watchAt, boxAt, recordAt int) {
	t.Helper()
	watchAt, boxAt, recordAt = -1, -1, -1
	var watches, boxes int
	for i, line := range strings.Split(report, "\n") {
		switch {
		case strings.HasPrefix(line, "dispatch (box): "):
			box, boxAt = line, i
			boxes++
		case strings.HasPrefix(line, "dispatch: "):
			watch, watchAt = line, i
			watches++
		case strings.HasPrefix(line, "upgraded "):
			recordAt = i
		}
	}
	if watches > 1 || boxes > 1 {
		t.Errorf("the report carries %d per-instance loop lines and %d box lines; one of each or none:\n%s", watches, boxes, report)
	}
	return watch, box, watchAt, boxAt, recordAt
}

// TestQATheUpgradeReportPrintsBothLoopLinesAndNeitherClaimsTheOthersReach is
// ADR 0092 D8's sentence, through upgradeRunPlan, over a plan carrying a
// reading on each side.
//
// Both readings are STALE ones, because stale is the state the lines exist
// for and the one whose content is unmistakable: the per-instance line can
// only name pid 424242 if the Loop reader was read, and the box line can only
// name the weller instance if the Box reader was — and the per-instance
// reader cannot see a sibling instance at all, which is the whole reason the
// second line is in the report.
//
// MUTATION-CHECKED 2026-11-21 (the arms in this file's header): B2 and B2L
// each red this test on the missing line and B4 on the duplicate one; B1, B1L
// and B3 red the wiring pin below.
func TestQATheUpgradeReportPrintsBothLoopLinesAndNeitherClaimsTheOthersReach(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{})
	img := WatchImage{
		Pid: 424242, Path: f.target,
		Image: "54:1", OnDisk: "54:2",
		Version: "0.5.3+deadbee", Home: "/homes/ranger",
	}
	if !img.Stale() {
		t.Fatalf("the fixture's reading is not stale, so this test cannot state its claim: %+v", img)
	}
	boxPath, probe := theMeasuredBox(9999)
	f.readers = func(p *upgradePlan) {
		p.Loop = func() (WatchImage, bool) { return img, true }
		p.Box = func() BoxReading { return readBoxLoops(boxPath, probe) }
	}
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("a run that had two loop readings to report rolled itself back: %v\n%s", err, out)
	}
	watch, box, watchAt, boxAt, recordAt := r1zdrLoopLines(t, out)
	if watchAt < 0 {
		t.Fatalf("the report carries no per-instance `dispatch:` line (ranger-base-rrgnc) — a reader is told nothing about the loop the rename left on the old inode:\n%s", out)
	}
	if boxAt < 0 {
		t.Fatalf("the report carries no `dispatch (box):` line (ranger-base-is1qp, ADR 0092 D8) — so the per-instance line above is the only loop reading in it, which is the all-clear this close exists to prevent:\n%s", out)
	}

	// Each line is the reading its own reader took, and only that reader
	// could have produced what is asserted here.
	for _, want := range []string{"dispatcher pid 424242", "posse 0.5.3+deadbee"} {
		if !strings.Contains(watch, want) {
			t.Errorf("the per-instance line does not carry %q, so it is not this plan's Loop reading:\n%s", want, watch)
		}
	}
	for _, want := range []string{"weller", "STALE", "ranger-base-is1qp"} {
		if !strings.Contains(box, want) {
			t.Errorf("the box line does not carry %q, so it is not this plan's Box reading — and the weller loop is a sibling instance the per-instance reader cannot see at all:\n%s", want, box)
		}
	}

	// NEITHER CLAIMS THE OTHER'S REACH. Each scope sentence belongs to one
	// renderer, so a report in which one line carries the other's clause is
	// one where the same reading was printed twice under two headings.
	if !strings.Contains(watch, "scope: one instance's loop") {
		t.Errorf("the per-instance line does not say its reach is one instance (ranger-base-nqaa1):\n%s", watch)
	}
	if strings.Contains(watch, "every process on this box") {
		t.Errorf("the per-instance line claims the box-wide reading's reach:\n%s", watch)
	}
	if !strings.Contains(box, "scope: every process on this box") {
		t.Errorf("the box line does not say what it read:\n%s", box)
	}
	if strings.Contains(box, "one instance's loop") {
		t.Errorf("the box line claims the per-instance reading's reach:\n%s", box)
	}

	// THE ORDER (ADR 0092 D8). The per-instance line first — it is the loop
	// the delegate's own next seat comes through and the one its reader acts
	// on first — the box line after it, and D6's record line last, because
	// that is the line the delegate posts on the install bead.
	if watchAt > boxAt {
		t.Errorf("the box line (line %d) printed BEFORE the per-instance one (line %d) — the delegate's own loop is the reading they act on first:\n%s", boxAt, watchAt, out)
	}
	if recordAt < 0 || recordAt < boxAt {
		t.Errorf("D6's record line is at %d and the box line at %d — the record is the LAST line of this report:\n%s", recordAt, boxAt, out)
	}
}

// TestQATheUpgradeReportPrintsNeitherLoopLineWithoutAReader is the control
// for the test above, and the report-level half of both renderers' "nil
// prints nothing" rule.
//
// It is what makes the assertions up there about the READERS: the same
// fixture, the same fake binaries, the same report — and with no reader
// installed, the two prefixes this file keys on appear nowhere, so neither of
// them is matching something else `upgradeRunPlan` prints.
func TestQATheUpgradeReportPrintsNeitherLoopLineWithoutAReader(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{})
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("the control run rolled itself back: %v\n%s", err, out)
	}
	_, _, watchAt, boxAt, recordAt := r1zdrLoopLines(t, out)
	if watchAt >= 0 || boxAt >= 0 {
		t.Errorf("a plan carrying no loop reader printed a loop line (per-instance at %d, box at %d) — an empty-looking reading is worse than no line:\n%s", watchAt, boxAt, out)
	}
	if recordAt < 0 {
		t.Fatalf("the control run printed no record line, so it is not a complete report and proves nothing about what is missing from it:\n%s", out)
	}
}

// TestQACmdUpgradeInstallsBothLoopReadersOnItsPlan is the other end of the
// wiring: the function cmdUpgrade reaches the plan through, which is where
// B1 and B1L now live.
//
// THE SUBJECTS ARE ASSERTED ON THE READERS AND NOT ON THE PRINTED LINES, on
// purpose. The box line names its path only when the census answered, and off
// linux it abstains (boxloops_other.go) while from a seat in a child user
// namespace every same-uid peer is refused (ranger-base-rph5z) — both are
// correct readings and neither names a path. `BoxReading.Path` is the subject
// the reading was ABOUT in every branch including those two, so it is the
// assertion that holds on every box this suite runs on.
func TestQACmdUpgradeInstallsBothLoopReadersOnItsPlan(t *testing.T) {
	f := newD4Fixture(t, fakePosse{Stamp: "aaaaaaa"}, fakePosse{})
	a := wtApp(t)

	p := upgradePlan{Target: f.target}
	upgradeLoopReaders(&p, a)
	if p.Loop == nil {
		t.Fatal("no Loop reader on the plan — upgradeWatchLine prints nothing for a nil one, so the report says nothing about the loop the rename left behind (ranger-base-rrgnc)")
	}
	if p.Box == nil {
		t.Fatal("no Box reader on the plan — upgradeBoxLine prints nothing for a nil one, so the report's only loop reading is the per-instance one and reads as an all-clear for the box (ranger-base-is1qp, ADR 0092 D8)")
	}
	if got := p.Box().Path; got != f.target {
		t.Errorf("the box reading is about %q, want the plan's target %q — the subject is the path THIS RUN replaces, which is the file every instance's loop execs and not this home's anything (ADR 0092 D8)", got, f.target)
	}
	if img, _ := p.Loop(); img.Home != a.Home {
		t.Errorf("the loop reading names instance %q, want the App this verb runs in (%q) — a reading of someone else's instance is not the loop this delegate's next seat comes through", img.Home, a.Home)
	}

	// And a report driven through that same wiring prints both lines. The
	// readings here are whatever this box answers — an abstention is one of
	// them, and an abstention still prints (which is the half of D8 that
	// makes the line worth having).
	f.readers = func(p *upgradePlan) { upgradeLoopReaders(p, a) }
	out, err := f.run(t)
	if err != nil {
		t.Fatalf("a run with the production loop readers installed rolled itself back: %v\n%s", err, out)
	}
	watch, box, watchAt, boxAt, _ := r1zdrLoopLines(t, out)
	if watchAt < 0 {
		t.Fatalf("the per-instance `dispatch:` line is absent from a report whose plan carries the Loop reader cmdUpgrade installs:\n%s", out)
	}
	if boxAt < 0 {
		t.Fatalf("the `dispatch (box):` line is absent from a report whose plan carries the Box reader cmdUpgrade installs:\n%s", out)
	}
	// The per-instance line names the instance it read in every branch,
	// abstention included, so this holds wherever the suite runs.
	if !strings.Contains(watch, AbbrevHome(a.Home)) {
		t.Errorf("the per-instance line does not name the instance this plan's reader was given (%s), so it is not a reading of this App:\n%s", AbbrevHome(a.Home), watch)
	}
	if watchAt > boxAt {
		t.Errorf("the box line (line %d) printed before the per-instance one (line %d):\n%s", boxAt, watchAt, out)
	}
	if strings.Contains(box, "one instance's loop") {
		t.Errorf("the box line carries the per-instance reading's scope clause, so the same reading was printed twice:\n%s", box)
	}
}
