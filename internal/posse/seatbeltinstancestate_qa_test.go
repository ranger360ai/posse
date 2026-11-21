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
// AND SINCE ranger-base-6akzw THERE IS NO STATE GRANT AT EITHER HOME. That
// bead took `<home>/state` out of the caged write set outright — it holds
// what the LAUNCHER and the dispatcher write and later run or trust outside
// every wall, and no member of it ever had a measured seat writer
// (seatbelt.go states the census). So this file's two probes are now the
// same answer on both sides: a session under home2 writes neither
// instance's state dir. What it DOES write there is its own gates dir, and
// that row is here because without it both refusals would be true for the
// wrong reason — a home the profile does not reach at all.
//
// THE CONTROLS ARE THE TWO PRE-FIX GRANTS, rendered deliberately and one
// per claim: 6akzw's own grant put back for the first, and qfzr's
// wrong-home grant for the second. Both state dirs are real and writable
// outside any sandbox, so a refusal that showed up under the control too
// would be a missing fixture rather than a wall; the controls are what
// tells those apart, and their flipped verdicts are the witness that the
// probes above measured a grant and not a typo.
//
// IT WAS RED ON EVERY MACOS LEG FROM 6akzw's LANDING, and on no other
// reader (ranger-base-jyqk4): sbSkipUnlessSandboxable means a darwin host
// is this file's only venue, so the stale `want ALLOWED` on the own-state
// probe — and a control that could no longer be BUILT, `is in no grant to
// swap` — were invisible to every seat and to both ubuntu arms of ci.yml.

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
	gates := sbMkdir(t, two.GatesDir("developer"))
	ag := &AgentFile{Name: "developer", MemoryDir: filepath.Join(two.PersonasDir(), "developer")}

	// The profile a session under RHQ_HOME=<home2> actually launches under:
	// RenderSeatbelt is the same method herdrback puts on the launch line.
	prof, err := two.RenderSeatbelt(ag, cwd)
	if err != nil {
		t.Fatal(err)
	}
	// The cost check first, because the two refusals below are worthless
	// without it: what a seat writes under its own state dir is its two
	// audit logs, re-allowed over the gates deny by the carve-out's Keep
	// (seatbelt.go), and the profile reaches those. NOT an arbitrary file
	// in the gates dir — that dir is granted and then denied, so a seat
	// cannot write even its own L1 shim (seatbeltcarveout_qa_test.go's
	// `rewrite this session's own L1 shim` row), and a cost check there
	// would fail over the deny rather than over the home.
	if !sbRun(t, prof, "echo refusal >> "+filepath.Join(gates, "refusals.log")) {
		t.Fatalf("a session under %s cannot append to its own audit trail under %s — the profile reaches nothing of this home, so neither refusal below is about a grant", two.Home, gates)
	}
	if sbRun(t, prof, "touch "+filepath.Join(two.StateDir, "probe-own")) {
		t.Errorf("a session under %s wrote its own state dir %s — it holds what the launcher writes and later runs outside every wall (ranger-base-6akzw)", two.Home, two.StateDir)
	}
	if sbRun(t, prof, "touch "+filepath.Join(one.StateDir, "probe-cross")) {
		t.Errorf("a session under %s wrote the other instance's state dir %s (rangerhq-qfzr)", two.Home, one.StateDir)
	}

	// Control 1, ranger-base-6akzw's grant put back: this session's OWN
	// state dir appended to the set it shipped before that bead.
	own := sbInstanceStateProfile(t, two, ag, cwd, gates, "pre6akzw.sb", absResolve(two.StateDir))
	if !sbRun(t, own, "touch "+filepath.Join(two.StateDir, "probe-own-control")) {
		t.Errorf("control: %s is refused even when granted — the fixture, not the wall, is what the own-state probe above measured", two.StateDir)
	}

	// Control 2, rangerhq-qfzr's shape: the grant pointed at a home this
	// session is not running against. The cross probe above is only about
	// the DERIVATION while this one flips.
	cross := sbInstanceStateProfile(t, two, ag, cwd, gates, "preqfzr.sb", absResolve(one.StateDir))
	if !sbRun(t, cross, "touch "+filepath.Join(one.StateDir, "probe-cross-control")) {
		t.Errorf("control: %s is refused even when granted — the fixture, not the wall, is what the cross-instance probe above measured", one.StateDir)
	}
	if sbRun(t, cross, "touch "+filepath.Join(two.StateDir, "probe-own-under-cross")) {
		t.Errorf("control: granting %s made %s writable too, so neither control isolates the grant it names", one.StateDir, two.StateDir)
	}
}

// sbInstanceStateProfile renders this session's shipped profile with ONE
// extra grant — a state dir no shipped producer names any more — and
// returns its path. The carve-out is recomputed over the widened set, not
// over the shipped one: the deny is what a trailing-deny wall is, and a
// control that kept the narrow carve-out would be measuring two changes.
func sbInstanceStateProfile(t *testing.T, a *App, ag *AgentFile, cwd, gates, name, extra string) string {
	t.Helper()
	w := a.SeatbeltWritable(ag, cwd, gates)
	if writeGranted(w, extra) {
		t.Fatalf("%s is already in the shipped grant, so this control changes nothing:\n  %v", extra, w)
	}
	w = append(append([]string{}, w...), extra)
	return sbRenderProfile(t, name, SeatbeltProfile(ag.Name, w, nil, a.SeatbeltCarveOut(ag, cwd, gates, w)))
}
