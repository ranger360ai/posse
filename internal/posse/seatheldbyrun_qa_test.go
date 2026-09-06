//go:build !posse_arm2 && !posse_arm3

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
	"time"
)

func TestQASeatThisRunFiredIntoStaysHeldAcrossFirePasses(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "hopper", "[rust]")
	repo := qaRepo(t, b.App,
		`[{"id":"b-1","title":"first","priority":0,"labels":["rust"]},{"id":"b-2","title":"second","priority":1,"labels":["rust"]}]`,
		`[{"id":"b-1","status":"open"},{"id":"b-2","status":"open"}]`)
	agentPerLaunch(t, fake)
	d.PromptGrace = 0
	// The first fireLoop below must really launch b-1 — the witness at line
	// ~54 depends on it, and so does the seat-hold invariant this test
	// exists for. That launch is awaitAgent's two full gates (awaitTarget
	// then awaitSettled), each armed for newTestDispatcher's default 2s,
	// and each poll is another `agent list`/`agent explain` fork of this
	// whole test binary via the fake herdr (RHQ_FAKE_HERDR). agentPerLaunch
	// answers both on their FIRST poll, so ordinarily this costs no wall
	// time at all — the 2s is headroom, spent only when a full
	// `go test ./...` run has this box's CPU busy enough that a subprocess
	// fork does not schedule inside it (SIGHTED once, ranger-base-qhf8: red
	// 1/3 full internal/rhq runs, green alone and green on a second full
	// run in the same tree). Widened well past that so the launch survives
	// the load this suite runs under, not to change what either gate does.
	d.StartupWait = 15 * time.Second

	// The first pass below really launches, and a launch leaves an `agent
	// prompt` leg in flight that only gather joins — and there is no gather
	// here. joinPrompts is the join, registered after the last t.TempDir
	// above so LIFO runs it before any removal (ranger-base-nqtvs).
	var inFlight []*pendingBead
	t.Cleanup(func() { joinPrompts(t, inFlight) })

	slot := SessionFor("hopper", repo)
	busy := map[string]string{}
	sessFail := map[string]int{}
	first := []RepoIssue{{BdIssue: BdIssue{ID: "b-1", Title: "first", Labels: []string{"rust"}}, Dir: repo}}
	second := []RepoIssue{{BdIssue: BdIssue{ID: "b-2", Title: "second", Labels: []string{"rust"}}, Dir: repo}}

	if _, p, _, err := d.fireLoop(first, "", 0, busy, sessFail); err != nil {
		t.Fatal(err)
	} else {
		inFlight = append(inFlight, p...)
	}
	// The witness: this fire pass really did put a bead on the seat. An
	// assertion that the next pass launches nothing is otherwise satisfied
	// by a fixture that never launched anything at all.
	if log := calls(t, fake); !strings.Contains(log, "workspace create --label "+SessionForBead("hopper", repo, "b-1")) {
		t.Fatalf("the first fire pass must launch b-1, or the second proves nothing:\n%s\n%s", dispatcherOut(d), log)
	}
	// The map now records WHICH bead holds the seat, not merely that one
	// does (ranger-base-wj7e9): the busy line an operator reads names it.
	if busy[slot] != "b-1" {
		t.Fatalf("a seat this Run fired into is this Run's occupancy and must outlive the fire pass that took it, naming its bead (ADR 0028 §3); got %q:\n%s", busy[slot], dispatcherOut(d))
	}

	// Now the live read goes quiet — herdr lists no agent for hopper, which
	// is what a launch that has not registered yet looks like from here, and
	// what a working session looks like the instant its pane is re-listed.
	// The seat is NOT free: b-1 has not settled. Only the Run map knows.
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(`[]`), 0o644)

	if _, p, _, err := d.fireLoop(second, "", 0, busy, sessFail); err != nil {
		t.Fatal(err)
	} else {
		inFlight = append(inFlight, p...)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("hopper", repo, "b-2")) {
		t.Errorf("a second bead went onto a seat this Run already fired into and whose bead has not settled:\n%s\n%s", dispatcherOut(d), log)
	}
	if busy[slot] != "b-1" {
		t.Errorf("the hold must survive the second fire pass too, still naming b-1 — it is released at the settle, not at the end of a pass; got %q", busy[slot])
	}
}
