//go:build posse_arm3

package posse

// ranger-base-3ula, the verify of rangerhq-qfzr: the seatbelt's state grant
// is derived from the App's home rather than spelled. The half of that
// claim which is the BUG — "a second RHQ_HOME's caged sessions get no write
// access to their own state dir while being granted write into the default
// instance's" — was pinned only over the []string SeatbeltWritable returns
// (seatbeltconstitution_qa_test.go). A grant is graded by what the kernel
// does with it, and the two are not the same statement: a set that names
// the right path and a profile the kernel matches it in are different
// claims, and only the second is the wall.
//
// The control is the pre-fix shape rendered deliberately — the same session,
// the same carve-out, the one grant swapped for the other home's state.
// Both state dirs are real and writable outside any sandbox, so a refusal
// that showed up under BOTH profiles would be a missing fixture rather than
// a wall; the control is what tells those apart, and its two flipped
// verdicts are the witness that the probes above measured the grant.

import (
	"path/filepath"
	"testing"
)

func TestQAKernelGrantsOwnInstanceStateAndRefusesTheOthers(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	root := sbRoot(t) // HOME elsewhere, TMPDIR a sibling: nothing here is granted by accident
	one := NewAppAt(filepath.Join(root, "home1"))
	two := NewAppAt(filepath.Join(root, "home2"))
	for _, a := range []*App{one, two} {
		homeWithConstitution(t, a, "")
	}
	cwd := sbMkdir(t, filepath.Join(root, "work"))
	gates := two.GatesDir("developer")
	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(two.PersonasDir(), "developer")}

	// The profile a session under RHQ_HOME=<home2> actually launches under:
	// RenderSeatbelt is the same method herdrback puts on the launch line.
	prof, err := two.RenderSeatbelt(ag, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if !sbRun(t, prof, "touch "+filepath.Join(two.StateDir, "probe-own")) {
		t.Errorf("a session under %s cannot write its own state dir %s", two.Home, two.StateDir)
	}
	if sbRun(t, prof, "touch "+filepath.Join(one.StateDir, "probe-cross")) {
		t.Errorf("a session under %s wrote the other instance's state dir %s (rangerhq-qfzr)", two.Home, one.StateDir)
	}

	// The control: the pre-fix grant, which named a home this session is not
	// running against.
	swapped := false
	var pre []string
	for _, g := range two.SeatbeltWritable(ag, cwd, gates) {
		if g == absResolve(two.StateDir) {
			g, swapped = absResolve(one.StateDir), true
		}
		pre = append(pre, g)
	}
	if !swapped {
		t.Fatalf("control built nothing: %s is in no grant to swap:\n  %v", two.StateDir, pre)
	}
	ctrl := sbRenderProfile(t, "prefix.sb", SeatbeltProfile(ag.Name, pre, nil, two.SeatbeltCarveOut(ag, cwd, gates, pre)))
	if sbRun(t, ctrl, "touch "+filepath.Join(two.StateDir, "probe-own-control")) {
		t.Errorf("control: the state grant is not what makes %s writable — it is writable with the grant pointed elsewhere", two.StateDir)
	}
	if !sbRun(t, ctrl, "touch "+filepath.Join(one.StateDir, "probe-cross-control")) {
		t.Errorf("control: %s is refused even when granted — the fixture, not the wall, is what the probes above measured", one.StateDir)
	}
}
