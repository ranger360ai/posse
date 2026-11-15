//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-pxprm, verifying ranger-base-i92tj — ADR 0080 D31 AT THE
// JUDGE'S ARGV, which is the half of that sentence no run reached.
//
// THE CLAIM is the one TestQALandedCheckVerdictSaysWhenThereIsNoSocketWall's
// header states and only half-pins: "on a host whose Landlock ABI is below 9
// the judge's cage carries no socket-wall step, and every verdict SAYS SO."
// The second clause is pinned there, over a stubbed ABI, read off the
// runner's own output. The first clause — the RENDER — was not, and the arm
// that would have said so is already written sixty lines up, in
// TestQALandedCheckJudgeIsNoStrongerThanASeat's `else`:
//
//	} else {
//	        if i := bwIndex(argv, WallUnixVerb); i >= 0 { ... }
//
// That arm is selected by `if wallUnixAvailableFor(WallBwrap)` — THIS HOST's
// real answer — and this fleet is ABI 10, so the arm is unreachable here and
// its own header says so ("on this fleet the first is taken"). An arm no run
// reaches is a mutant nobody runs.
//
// MEASURED 2026-11-14 by mutation (D1 host, Linux 7.2.15-1-omarchy, ABI 10):
// dropping `wallUnixAvailableFor(m)` from the render's gate in
// boxLandedCage — so the step is typed on every host, the sub-ABI-9 one
// included — left `-run
// 'Anchor|Bwrap|Wall|Cage|Seatbelt|Landed|Venue'` over this package GREEN,
// 88.9s, every other mutant of that change having died one at a time. The
// disease is the one the close diagnosed for the sentence, one mechanism
// over: a branch only another host takes, decided and never exercised.
//
// WHY THAT MATTERS AND IS NOT COSMETIC. RunWallUnix refuses rather than execs
// when the domain will not apply (ADR 0080 D4/D30), and the render is what
// keeps it off a host where it cannot. A step typed below the ABI it needs is
// therefore a judge that never runs: every landing answers `landUnanswered`,
// forever, while the verdict line says DEGRADED — the report and the render
// disagreeing about the same layer.
//
// WHY THE ABI IS A PARAMETER HERE. `wallUnixABI` is a var for exactly this
// (wallunixrender_qa_test.go drives the Linux render from darwin the same
// way), so the host answer is stubbed and BOTH directions are read in one
// run: the step present where the layer is available, absent where it is not.
// A pin with only the absent rows would stay green over a render that never
// types the step at all, which is the opposite defect and the one the close's
// own first mutant killed.
//
// SERIAL, no t.Parallel: wallUnixABI is a package-level var and the fixture
// sets HOME.

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestQALandedCheckJudgeTypesNoSocketWallStepBelowTheABIItNeeds(t *testing.T) {
	if m := WallMechanism(); m != WallBwrap {
		t.Skipf("NOTHING MEASURED: the socket-wall step is scoped to %s and this host renders %s (the darwin half is ranger-base-q9bqu)", WallBwrap, m)
	}
	restore := wallUnixABI
	t.Cleanup(func() { wallUnixABI = restore })

	for _, c := range []struct {
		name string
		abi  int
		step bool // the judge's argv must carry the step
	}{
		{"ABI 10, the D1 host", 10, true},
		{"ABI exactly 9, the version that added RESOLVE_UNIX", WallUnixABIMin, true},
		{"ABI 8, where handled_access_fs would be refused outright", WallUnixABIMin - 1, false},
		{"no landlock_create_ruleset on this kernel at all", -38, false},
	} {
		t.Run(fmt.Sprintf("%s (ABI %d)", c.name, c.abi), func(t *testing.T) {
			wallUnixABI = func() int { return c.abi }
			f := newLandedCageFixture(t)
			child := filepath.Join(f.run, "judge")
			cage, err := boxLandedCage(&App{}, landedCageJudge, child, f.tree, []string{f.run}, landedRunEnv(f.run))
			if err != nil {
				t.Skip("NOTHING MEASURED: no landed-check cage renders on this host: " + err.Error())
			}
			argv := cage.Argv
			line := strings.Join(argv, " ")
			// The child is read first, so a row below that finds no step is
			// reading a cage that was rendered and not an empty one.
			if len(argv) == 0 || argv[len(argv)-1] != child {
				t.Fatalf("the judge's cage does not end with the child %s, so this row is reading something else:\n  %s", child, line)
			}
			i := bwIndex(argv, WallUnixVerb)
			switch {
			case c.step && i < 0:
				t.Errorf("the socket wall is available at Landlock ABI %d and the judge's cage carries no %q anywhere — a bwrap seat's launch line carries that step and ADR 0088 D3.1 says the judge is never stronger than a seat (ranger-base-i92tj):\n  %s",
					c.abi, WallUnixVerb, line)
			case !c.step && i >= 0:
				t.Errorf("the judge's cage carries the socket-wall step at argv[%d] on a host whose Landlock ABI is %d, below the %d RESOLVE_UNIX needs. The step REFUSES rather than execs when the domain will not apply (ADR 0080 D4/D30), so this renders a judge that never runs and a landing that answers nothing, at every landing, while the verdict line says DEGRADED (ADR 0080 D31, ranger-base-i92tj):\n  %s",
					i, c.abi, WallUnixABIMin, line)
			}
			// AND THE SENTENCE IS THE RENDER'S COMPLEMENT, in the same run
			// and over the same stub: the two halves of D31 are one claim,
			// and a host that types no step must say so. Pinned on its own
			// over the runner's output by
			// TestQALandedCheckVerdictSaysWhenThereIsNoSocketWall; read here
			// so neither half can move without the other.
			if said := landedJudgeSocketWallNote(); (said == "") != c.step {
				t.Errorf("at Landlock ABI %d the judge's cage %s the step and its verdict note is %q — the render and the report disagree about the same layer (ADR 0080 D31)",
					c.abi, map[bool]string{true: "carries", false: "does not carry"}[c.step], said)
			}
		})
	}
}
