package treepins

// The witness for the verify of ranger-base-8yv7z and ranger-base-na4rm
// (ranger-base-tw3t7): a reading that mixes the package's OWN directory with
// a climb OUT of it, in one list, is seen by neither half of the tree-wide
// derivation for internal/posse — and it is live three times.
//
// WHAT EACH HALF READS, and where the two miss each other.
//
//   - THE FENCE (twdClimbSet, ranger-base-8yv7z) reads a climb held in a
//     local, a local const or a package const, and follows it any number of
//     parameter hops to the walker. It does NOT read a climb that reaches
//     the walker as a RANGE VARIABLE over a list of literals — the set binds
//     at an AssignStmt or a ValueSpec, and `roots := []string{...}` binds the
//     SLICE, not its elements, so `for _, r := range roots { Walk(r) }` never
//     enters it. That spelling is the one twdRelEnv has read since
//     ranger-base-g6sb1, one package over, so the fence is narrower than the
//     derivation it serves on a shape its sibling already resolves.
//
//   - THE ENUMERATION SEED (twdRelTreeList) does read a range over a list of
//     literals — but one element it cannot read makes the WHOLE list unknown,
//     and its head comment gives the reason: "a range over that list walks
//     the element the rule could not read too, so claiming the list would be
//     claiming a root this file does not know." That is right in
//     internal/treepins, where a `../` climb LEAVES the tree. It is wrong for
//     internal/posse, where a climb is how a test REACHES the tree: the
//     unreadable element makes the pin more tree-wide, not less, and refusing
//     the list discards the readable `"."` beside it — the one-package census
//     ranger-base-m94g2 decided IS in the class.
//
// So a pin that reads its own package directory AND cmd/ in one range is
// outside the class precisely BECAUSE it does both, and gets no Makefile
// door from either bead.
//
// THE THREE LIVE SITES, as of 975d0d82 (named here rather than enumerated,
// so this pin reads no tree and needs no door of its own — the same shape as
// execwaitdelayholes_qa_test.go):
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
// is named by any Makefile door variable or dispositioned in
// treewidedoor_qa_test.go. All three are arm 1 or shared, so a door reaches
// them with no `-tags`.
//
// HOW TO READ THE ROWS. Every row goes through the register's own harnesses
// (twdFixtureClimbs, twdFixtureEnumerates), so a row cannot pass against a
// second, friendlier copy of either rule. A `hole` row asserts what the
// rules say TODAY, with the inversion in the failure message — it ships
// GREEN and is not a waiver: when the gap is closed, flip that row's want
// rather than deleting it, and the CONTROL rows are what keep the flip
// honest. The controls fail closed: each is a shape that must keep its
// current answer whichever way the hole is fixed.

import "testing"

// cihCase is one planted source and what each half of the derivation says
// about it. wantClimb is the fence (twdClimbSet, via twdAnalyze.climbEnum);
// wantEnum is the enumeration seed (twdRelTreeList, via relEnum reach).
type cihCase struct {
	name      string
	src       string
	wantClimb bool
	wantEnum  bool
	hole      bool
	why       string
}

var cihCases = []cihCase{
	// ── THE HOLES ────────────────────────────────────────────────────────
	{
		name: "HOLE a climb reaching the walker as a range variable over a named list",
		src: `func TestX(t *testing.T) { roots := []string{"..", "../../cmd"}
	for _, r := range roots {
		filepath.Walk(r, nil)
	} }`,
		wantClimb: false, wantEnum: false, hole: true,
		why: "visibility_test.go:161's shape — the fence does not bind a range variable, and every element climbs so the seed reads nothing either",
	},
	{
		name: "HOLE a climb reaching the walker as a range variable over an inline list",
		src: `func TestX(t *testing.T) { for _, r := range []string{"..", "../../cmd"} {
		filepath.Walk(r, nil)
	} }`,
		wantClimb: false, wantEnum: false, hole: true,
		why: "the same shape with the list written at the range — nothing about the hole depends on the list being named",
	},
	{
		name: "HOLE the package's own directory and a climb in ONE list, at Walk",
		src: `func TestX(t *testing.T) { for _, r := range []string{".", "../../cmd"} {
		filepath.Walk(r, nil)
	} }`,
		wantClimb: false, wantEnum: false, hole: true,
		why: "gatedkeychain_test.go:443's shape — the readable `\".\"` is discarded because the list also holds a climb, and the climb is invisible to the fence",
	},
	{
		name: "HOLE the package's own directory and a climb in ONE list, at ReadDir",
		src: `func TestX(t *testing.T) { for _, d := range []string{".", "../../cmd/posse"} {
		os.ReadDir(d)
	} }`,
		wantClimb: false, wantEnum: false, hole: true,
		why: "exactmodel_qa_test.go:539's shape, the same gap at a different walker",
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
	holes, positives := 0, 0
	for _, tc := range cihCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := twdFixtureClimbs(t, tc.src); got != tc.wantClimb {
				t.Errorf("the CLIMB fence says %v for this source and this row says %v.\n"+
					"  What the row holds it to: %s.\n"+
					"  If this row is a HOLE (%v), a fix that closed it has arrived: flip wantClimb here rather than deleting the row, and keep the CONTROL rows — they are what says the fix did not also take a fixture walk or a bare-root climb.\n%s",
					got, tc.wantClimb, tc.why, tc.hole, tc.src)
			}
			if got := twdFixtureEnumerates(t, tc.src); got != tc.wantEnum {
				t.Errorf("the ENUMERATION seed says %v for this source and this row says %v.\n"+
					"  What the row holds it to: %s.\n"+
					"  If this row is a HOLE (%v), the list rule has stopped discarding a readable element beside an unreadable one: flip wantEnum here rather than deleting the row.\n%s",
					got, tc.wantEnum, tc.why, tc.hole, tc.src)
			}
		})
	}
	// The floor. Four rows asserting "neither rule sees this" are satisfied
	// by two rules that see nothing at all, so the table must also hold rows
	// each rule says YES to — and it must keep holding them if the holes are
	// flipped.
	for _, tc := range cihCases {
		if tc.hole {
			holes++
		}
		if tc.wantClimb || tc.wantEnum {
			positives++
		}
	}
	if holes < 4 {
		t.Errorf("%d HOLE rows, want at least 4 — three live sites and the named/inline pair are what this witness exists to hold", holes)
	}
	if positives < 4 {
		t.Errorf("only %d row(s) in this table are a YES from either rule — an absence asserted with no planted positive measures the harness, not the tree", positives)
	}
}
