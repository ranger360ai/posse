package treepins

// The witness for the verify of ranger-base-8yv7z and ranger-base-na4rm
// (ranger-base-tw3t7): a reading that mixes the package's OWN directory with
// a climb OUT of it, in one list, was seen by neither half of the tree-wide
// derivation for internal/posse — and it was live three times.
//
// CLOSED BY ranger-base-g31e0, one fix to each half, and this file is the
// regression: the four rows below are the spellings it was filed for and
// every one of them is now a YES from at least one rule. They were written
// as `hole` rows asserting the gap, with the instruction to FLIP the want
// rather than delete the row; that is what happened, and the CONTROL rows
// are what say the two fixes took nothing with them. Revert either fix and
// the rows that were holes red with the measurement in their message.
//
// WHAT EACH HALF READS, and where the two missed each other.
//
//   - THE FENCE (twdClimbSet, ranger-base-8yv7z) read a climb held in a
//     local, a local const or a package const, and followed it any number of
//     parameter hops to the walker. It did NOT read a climb that reaches
//     the walker as a RANGE VARIABLE over a list of literals — the set bound
//     at an AssignStmt or a ValueSpec, and `roots := []string{...}` binds the
//     SLICE, not its elements, so `for _, r := range roots { Walk(r) }` never
//     entered it. That spelling is the one twdRelEnv has read since
//     ranger-base-g6sb1, one package over, so the fence was narrower than the
//     derivation it serves on a shape its sibling already resolved. It reads
//     a list now, element by element and by exactly the test it gives a
//     scalar, so the jurisdiction line is unmoved (twdClimbSet.climbingList).
//
//   - THE ENUMERATION SEED (twdRelTreeList) does read a range over a list of
//     literals — but one element it could not read made the WHOLE list
//     unknown, and its head comment gave the reason: "a range over that list
//     walks the element the rule could not read too, so claiming the list
//     would be claiming a root this file does not know." That is right in
//     internal/treepins, where a `../` climb LEAVES the tree. It was wrong for
//     internal/posse, where a climb is how a test REACHES the tree: the
//     unreadable element makes the pin more tree-wide, not less, and refusing
//     the list discarded the readable `"."` beside it — the one-package census
//     ranger-base-m94g2 decided IS in the class. A CLIMBING element is
//     skipped now and every other kind still voids the list, so the row
//     below that plants a variable beside a readable path keeps its answer.
//
// So a pin that read its own package directory AND cmd/ in one range was
// outside the class precisely BECAUSE it did both, and got no Makefile
// door from either bead.
//
// THE THREE SITES THAT WERE LIVE, as of 975d0d82 (named here rather than
// enumerated, so this pin reads no tree and needs no door of its own — the
// same shape as execwaitdelayholes_qa_test.go). All three are in
// $(QA_PKGCENSUS_PINS) now; the first two are DERIVED members through the
// readable `"."` the seed stopped discarding, and the third took the root
// from qibRepoRoot because every element of its list climbs and the seed can
// read none of them — which is what keeps these two findings two:
//
//	internal/posse/visibility_test.go:161       TestVisibilityOverrideIsNeverDispatched
//	  roots := []string{"..", "../../cmd"}; filepath.Walk(root, …)
//	internal/posse/gatedkeychain_test.go:443    TestQAKeychainStoreIsWiredToTheAbsoluteBinary
//	  for _, root := range []string{".", "../../cmd"} { filepath.Walk(root, …) }
//	internal/posse/exactmodel_qa_test.go:539    TestExactModelHasNoDeclarationSurface
//	  for _, dir := range []string{".", "../../cmd/posse"} { os.ReadDir(dir) }
//
// Each walks every non-test .go file under a top-level directory, so a bead
// that adds one joins its population and can red it, and none of the three
// was named by any Makefile door variable or dispositioned in
// treewidedoor_qa_test.go. All three are arm 1 or shared, so the untagged
// door reaches them with no `-tags`.
//
// HOW TO READ THE ROWS. Every row goes through the register's own harnesses
// (twdFixtureClimbs, twdFixtureEnumerates), so a row cannot pass against a
// second, friendlier copy of either rule. A `filed` row is one of the four
// spellings this witness was filed for; it asserted the gap while the gap
// was open and asserts the FIX now, and the floor at the bottom holds every
// one of them to being a YES from at least one rule — which is the whole of
// what makes this file a regression rather than a record. The CONTROL rows
// fail closed: each is a shape that had to keep its answer whichever way the
// gap was closed, and each still does.

import "testing"

// cihCase is one planted source and what each half of the derivation says
// about it. wantClimb is the fence (twdClimbSet, via twdAnalyze.climbEnum);
// wantEnum is the enumeration seed (twdRelTreeList, via relEnum reach).
type cihCase struct {
	name      string
	src       string
	wantClimb bool
	wantEnum  bool
	// filed: one of the four spellings this witness was filed for. It held
	// the GAP while the gap was open (both rules false) and holds the FIX
	// now; the floor at the bottom requires four of them and requires each
	// to be a YES from at least one rule, so neither fix can be reverted
	// without a row reddening here.
	filed bool
	why   string
}

var cihCases = []cihCase{
	// ── THE FOUR THIS WITNESS WAS FILED FOR ─────────────────────────────
	{
		name: "a climb reaching the walker as a range variable over a named list",
		src: `func TestX(t *testing.T) { roots := []string{"..", "../../cmd"}
	for _, r := range roots {
		filepath.Walk(r, nil)
	} }`,
		wantClimb: true, wantEnum: false, filed: true,
		why: "visibility_test.go:161's shape — the fence now binds a range variable, and every element climbs so the seed reads nothing either: the FENCE is this one's only route, which is why that site took the root from qibRepoRoot",
	},
	{
		name: "a climb reaching the walker as a range variable over an inline list",
		src: `func TestX(t *testing.T) { for _, r := range []string{"..", "../../cmd"} {
		filepath.Walk(r, nil)
	} }`,
		wantClimb: true, wantEnum: false, filed: true,
		why: "the same shape with the list written at the range — nothing about the hole depended on the list being named, and nothing about the fix does either",
	},
	{
		name: "the package's own directory and a climb in ONE list, at Walk",
		src: `func TestX(t *testing.T) { for _, r := range []string{".", "../../cmd"} {
		filepath.Walk(r, nil)
	} }`,
		wantClimb: true, wantEnum: true, filed: true,
		why: "gatedkeychain_test.go:443's shape — BOTH halves read it now: the seed keeps the readable `\".\"` beside the climb, which is what doors that site where it stands, and the fence sees the climb (the ARM stands down on a body the seed derives, which is a separate decision from the rule's answer here)",
	},
	{
		name: "the package's own directory and a climb in ONE list, at ReadDir",
		src: `func TestX(t *testing.T) { for _, d := range []string{".", "../../cmd/posse"} {
		os.ReadDir(d)
	} }`,
		wantClimb: true, wantEnum: true, filed: true,
		why: "exactmodel_qa_test.go:539's shape, the same gap and the same fix at a different walker",
	},

	// ── CONTROLS: the two halves, each shown able to say YES ─────────────
	// Without these the four rows above are satisfied by a pair of rules
	// that answer false to everything, which is exactly what an assertion
	// of absence is worth without a planted positive.
	{
		name:      "CONTROL the same list with the climbing element removed is a member by enumeration",
		src:       `func TestX(t *testing.T) { for _, d := range []string{".", "testdata"} { os.ReadDir(d) } }`,
		wantClimb: false, wantEnum: true,
		why: "ONE element decides it: drop the climb and the identical range is derived. This is the control that makes the two rows above a finding about the climb and not about ranges",
	},
	{
		name:      "CONTROL a climb held in a local IS fenced",
		src:       `func TestX(t *testing.T) { dir := "../../examples"; os.ReadDir(dir) }`,
		wantClimb: true, wantEnum: false,
		why: "ranger-base-8yv7z's own spelling — the fence reads an identifier when the binding is an assignment",
	},
	{
		name:      "CONTROL a climb spelled at the walker IS fenced",
		src:       `func TestX(t *testing.T) { os.ReadDir(filepath.Join("..", "..", "examples")) }`,
		wantClimb: true, wantEnum: false,
		why: "ranger-base-nt0pi's spelling, the base case both later beads widened from",
	},
	{
		name:      "CONTROL a bare relative literal IS a member by enumeration",
		src:       `func TestX(t *testing.T) { os.ReadDir(".") }`,
		wantClimb: false, wantEnum: true,
		why: "ranger-base-m94g2's one-package census, ranger-base-na4rm's third seed — the half the mixed list above drops out of",
	},

	// ── CONTROLS: shapes that must keep their answer either way ──────────
	{
		name:      "CONTROL a range over a list of bare roots stays the hidden arm's",
		src:       `func TestX(t *testing.T) { for _, r := range []string{"..", "../.."} { os.ReadDir(r) } }`,
		wantClimb: false, wantEnum: false,
		why: "the jurisdiction line ranger-base-8yv7z drew by argument: a climb naming NO path below the root is twdRootExprs' business (detectionRig and its nine callers), so widening the fence to range variables must not pick this up",
	},
	{
		name:      "CONTROL a range over scratch directories is not a climb and not an enumeration",
		src:       `func TestX(t *testing.T) { for _, r := range []string{t.TempDir()} { os.ReadDir(r) } }`,
		wantClimb: false, wantEnum: false,
		why: "a fixture walk must stay out of the class under any fix — admitting it is a door demanded for a t.TempDir",
	},
	{
		name:      "CONTROL a range variable that roots no walker is neither",
		src:       `func TestX(t *testing.T) { for _, p := range []string{"../../AGENTS.md"} { os.ReadFile(p) } }`,
		wantClimb: false, wantEnum: false,
		why: "reading named files is not enumerating a population — the key of the class is the enumeration, and a fix that keys on `a climb in a range` instead would take this too",
	},
}

// TestQATheClimbFenceAndTheOnePackageSeedBothMissAMixedRootList is the
// witness. It is GREEN at this commit: the four `hole` rows assert what the
// register reads today, and the message says what the right answer is.
func TestQATheClimbFenceAndTheOnePackageSeedBothMissAMixedRootList(t *testing.T) {
	t.Parallel()
	filed, positives, filedPositives := 0, 0, 0
	for _, tc := range cihCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := twdFixtureClimbs(t, tc.src); got != tc.wantClimb {
				t.Errorf("the CLIMB fence says %v for this source and this row says %v.\n"+
					"  What the row holds it to: %s.\n"+
					"  If this is one of the four rows this witness was filed for (%v), ranger-base-g31e0's fence fix has been reverted or narrowed: twdClimbSet binds at an AssignStmt and a ValueSpec, and a climb reaching the walker as a RANGE VARIABLE over a list of literals needs twdClimbSet.climbingList to be read at all. Restore it rather than flipping this row — three live pins were undoored for exactly this.\n%s",
					got, tc.wantClimb, tc.why, tc.filed, tc.src)
			}
			if got := twdFixtureEnumerates(t, tc.src); got != tc.wantEnum {
				t.Errorf("the ENUMERATION seed says %v for this source and this row says %v.\n"+
					"  What the row holds it to: %s.\n"+
					"  If this is one of the four (%v), twdRelTreeList has gone back to voiding a whole list on one unreadable element: a CLIMBING element is skipped and every other kind still voids it, which is what keeps the readable `\".\"` of a mixed list a derived member (ranger-base-m94g2, ranger-base-g31e0).\n%s",
					got, tc.wantEnum, tc.why, tc.filed, tc.src)
			}
		})
	}
	// The floor, and it is the half of this file that survives the fix.
	// While the gap was open it held the table to four rows asserting "and
	// neither rule sees this" plus four a rule says YES to, because an
	// absence asserted with no planted positive measures the harness. Now
	// that the gap is closed it holds the four to being YES — a reverted fix
	// cannot pass by flipping a want back, because the floor fails too, with
	// this message rather than a row's.
	for _, tc := range cihCases {
		if tc.filed {
			filed++
			if tc.wantClimb || tc.wantEnum {
				filedPositives++
			}
		}
		if tc.wantClimb || tc.wantEnum {
			positives++
		}
	}
	if filed < 4 {
		t.Errorf("%d rows marked `filed`, want at least 4 — the three live sites and the named/inline pair are what this witness exists to hold, and dropping one is how a closed gap stops being watched", filed)
	}
	if filed != filedPositives {
		t.Errorf("%d of %d `filed` rows are a YES from at least one rule, want all of them — ranger-base-g31e0 closed this gap and these four rows ARE the regression: a row of it reading false again means the fence stopped reading a range over a list, or the seed went back to voiding a mixed list, and the right repair is the rule and not the want", filedPositives, filed)
	}
	if positives < 4 {
		t.Errorf("only %d row(s) in this table are a YES from either rule — an absence asserted with no planted positive measures the harness, not the tree", positives)
	}
}
