package posse

// Filed verifying ranger-base-t8tq's close (ranger-base-ogzh). That close
// split one map in two — a seat this Run FIRED into (seatMap.hold, released
// at the bead's settle) and what a fire pass merely READ about a seat
// (seatMap.note, expiring with the pass). Its four mutations, and the
// sibling pins here, all drive the `note` half: TestQASeatBusyInOneFirePassIsReReadInTheNext
// asserts a reading does not outlive its pass, and asserts that a seat this
// Run did not fire into is absent from the Run map.
//
// The `hold` half was pinned by nothing. MEASURED 2026-08-28: changing the
// successful-launch site from `seats.hold(slot)` to `seats.note(slot)` left
// the whole internal/rhq package green (-count=1, 457s). With that mutation a
// seat holds its occupancy only until the current fire pass ends, and under a
// rolling Run the next settle is seconds away — so the only thing left
// refusing a second bead into an occupied seat is the live read, in exactly
// the window where the live read is least likely to have caught up with a
// launch that was just typed.
//
// So this is the other arm: a seat this Run fired into stays the Run's until
// its bead settles, EVEN WHEN the live read says the seat is free.
// MUTATION: `seats.hold(slot)` → `seats.note(slot)` at the launch site in
// fireLoop → red.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQASeatThisRunFiredIntoStaysHeldAcrossFirePasses(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "hopper", "[rust]")
	repo := qaRepo(t, b.App,
		`[{"id":"b-1","title":"first","priority":0,"labels":["rust"]},{"id":"b-2","title":"second","priority":1,"labels":["rust"]}]`,
		`[{"id":"b-1","status":"open"},{"id":"b-2","status":"open"}]`)
	agentPerLaunch(t, fake)
	d.PromptGrace = 0

	slot := SessionFor("hopper", repo)
	busy := map[string]bool{}
	sessFail := map[string]int{}
	first := []RepoIssue{{BdIssue: BdIssue{ID: "b-1", Title: "first", Labels: []string{"rust"}}, Dir: repo}}
	second := []RepoIssue{{BdIssue: BdIssue{ID: "b-2", Title: "second", Labels: []string{"rust"}}, Dir: repo}}

	if _, _, _, err := d.fireLoop(first, "", 0, busy, sessFail); err != nil {
		t.Fatal(err)
	}
	// The witness: this fire pass really did put a bead on the seat. An
	// assertion that the next pass launches nothing is otherwise satisfied
	// by a fixture that never launched anything at all.
	if log := calls(t, fake); !strings.Contains(log, "workspace create --label "+SessionForBead("hopper", repo, "b-1")) {
		t.Fatalf("the first fire pass must launch b-1, or the second proves nothing:\n%s\n%s", dispatcherOut(d), log)
	}
	if !busy[slot] {
		t.Fatalf("a seat this Run fired into is this Run's occupancy and must outlive the fire pass that took it (ADR 0028 §3):\n%s", dispatcherOut(d))
	}

	// Now the live read goes quiet — herdr lists no agent for hopper, which
	// is what a launch that has not registered yet looks like from here, and
	// what a working session looks like the instant its pane is re-listed.
	// The seat is NOT free: b-1 has not settled. Only the Run map knows.
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(`[]`), 0o644)

	if _, _, _, err := d.fireLoop(second, "", 0, busy, sessFail); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("hopper", repo, "b-2")) {
		t.Errorf("a second bead went onto a seat this Run already fired into and whose bead has not settled:\n%s\n%s", dispatcherOut(d), log)
	}
	if !busy[slot] {
		t.Errorf("the hold must survive the second fire pass too — it is released at the settle, not at the end of a pass")
	}
}
