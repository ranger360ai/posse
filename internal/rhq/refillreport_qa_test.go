package rhq

// ranger-base-59jd: what a refill SAYS, and which arm of ADR 0028 §5
// observable 1 a window belongs to.
//
// MEASURED 2026-08-28 ~09:15 on the first live refill: the seat-scoped fire
// path printed a per-bead wall plus `– 131 ready bead(s) outside gwart's
// lane — skipped by --persona`, at every settle, attributed to nothing. The
// operator read it as a rogue persona-filtered loop and went to an alarm
// footing. And every idle-to-next line still said "no refill has shipped,
// this is the control arm" — a hardcoded string, unmoved by the refill going
// live, which would have made observable 1's before/after unreadable.
//
// Each test carries its own control arm, because both of these are about
// output and an assertion on output that only ever sees one arm is a
// sticker. Each says which mutation reds it; both were run.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A refill names the seat it is refilling for and reports its skips in one
// line; a fire pass that is NOT a refill still enumerates per bead.
//
// The queue is the same in both arms and so are the skips: one go bead the
// busy lane cannot take, one rust bead outside the filtered persona's lane.
// Only the CALL PATH differs.
//
// MUTATION A: drop the `d.refilling` branch in skipNf (always print) → the
// refill arm's per-bead lines come back → red on the absence assertions.
// MUTATION B: make skipNf always count → the control arm loses its per-bead
// lines → red. MUTATION C: delete beginRefill's printf → red on the header.
func TestQARefillNamesItsSeatAndSummarisesSkipsInOneLine(t *testing.T) {
	// The refill arm: one rolling Run. a-1 fires from the head of the pass,
	// settles closed, and the settle refills the seat it freed.
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	writePersona(t, b.App, "hopper", "[rust]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","status":"closed"},{"id":"a-2","status":"closed"},{"id":"a-3","status":"open"},{"id":"b-9","status":"open"}]`)
	write(t, filepath.Join(repo, "fake-ready-next.json"),
		`[{"id":"a-2","title":"u","labels":["go"]},{"id":"a-3","title":"v","labels":["go"]},{"id":"b-9","title":"w","labels":["rust"]}]`)
	agentPerLaunch(t, fake)
	d.Refill = true
	d.PromptGrace = 0

	// The fixture's queue swaps ONCE (fake-ready-next.json), so a-2 is still
	// on the list after it closes and the settle behind it makes a third
	// refill that re-offers a closed bead and fails its claim. That is the
	// fake, not the report: the assertions below name the FIRST refill's own
	// summary line, and the absences hold across every refill this Run makes.
	if _, err := d.Run("", "ranger", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	seat := SessionFor("ranger", repo)

	// The fixture's own witness: the refill really ran and really launched,
	// or the absence assertions below would be green on a Run that did
	// nothing at all.
	if !strings.Contains(out, "· a-2            creating session") {
		t.Fatalf("the refill must launch a-2 into the freed seat, or this test measures nothing:\n%s", out)
	}
	if !strings.Contains(out, "↻ refill for settled seat "+seat+" (a-1 settled)") {
		t.Errorf("the refill must name the seat it is refilling and the settle that freed it:\n%s", out)
	}
	if !strings.Contains(out, "↻ refill for settled seat "+seat+": 1 launched, 2 skipped (1 lane busy, 1 outside ranger's lane)") {
		t.Errorf("the refill's skips belong on one line, counted by reason:\n%s", out)
	}
	// The wall itself: neither the per-bead line nor the --persona tail may
	// be printed again at every settle.
	if strings.Contains(out, "– a-3") {
		t.Errorf("a refill must not enumerate its skips per bead — that is the wall the operator read as a rogue loop:\n%s", out)
	}
	if strings.Contains(out, "skipped by --persona") {
		t.Errorf("the --persona tail is the line that masqueraded as a persona-filtered loop; inside a refill it belongs in the summary:\n%s", out)
	}

	// The control arm: the same queue, the same skips, through the head of a
	// one-shot Run. Nothing is summarised and nothing announces a refill.
	cb, cfake := newTestBackend(t)
	cd := newTestDispatcher(t, cb)
	writePersona(t, cb.App, "ranger", "[go]")
	writePersona(t, cb.App, "hopper", "[rust]")
	crepo := qaRepo(t, cb.App,
		`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-3","title":"v","labels":["go"]},{"id":"b-9","title":"w","labels":["rust"]}]`,
		`[{"id":"a-1","status":"closed"},{"id":"a-3","status":"open"},{"id":"b-9","status":"open"}]`)
	_ = crepo
	agentPerLaunch(t, cfake)

	if _, err := cd.Run("", "ranger", 0); err != nil {
		t.Fatal(err)
	}
	cout := dispatcherOut(cd)
	if strings.Contains(cout, "↻ refill") {
		t.Errorf("a one-shot Run refills nothing and must not report as one:\n%s", cout)
	}
	if !strings.Contains(cout, "– a-3") {
		t.Errorf("outside a refill the enumeration is still per bead — the summary is the refill's, not dispatch's:\n%s", cout)
	}
	if !strings.Contains(cout, "outside ranger's lane — skipped by --persona") {
		t.Errorf("outside a refill the --persona tail still prints as it always did:\n%s", cout)
	}
}

// The arm stamp is read off the call path, not off a string.
//
// Same instrument, same seat, two Runs' worth of windows: one closed by a
// refill, one closed by the head of a pass. The stamp and the report's arm
// must differ, and neither arm may be assumed — the bug was exactly that
// one of them was hardcoded and nobody could tell.
//
// MUTATION: `r.Rolling = d.refilling != nil` → `r.Rolling = false` (or back
// to a constant arm string in seatIdleArm) → the rolling arm reds.
func TestQASeatIdleArmFollowsTheRefillPath(t *testing.T) {
	t.Run("closed by a refill", func(t *testing.T) {
		b, fake := newTestBackend(t)
		d := newTestDispatcher(t, b)
		writePersona(t, b.App, "ranger", "[go]")
		repo := qaRepo(t, b.App,
			`[{"id":"a-1","title":"t","labels":["go"]}]`,
			`[{"id":"a-1","status":"closed"},{"id":"a-2","status":"closed"}]`)
		write(t, filepath.Join(repo, "fake-ready-next.json"), `[{"id":"a-2","title":"u","labels":["go"]}]`)
		agentPerLaunch(t, fake)
		d.Refill = true
		d.PromptGrace = 0

		if _, err := d.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		out := dispatcherOut(d)
		if !strings.Contains(out, "a-1 settled") || !strings.Contains(out, "a-2 launched") {
			t.Fatalf("the refill must close a window for this seat, or there is no stamp to judge:\n%s", out)
		}
		if !strings.Contains(out, "[ADR 0028 §5 obs.1 rolling]") {
			t.Errorf("a window the refill closed is a treatment window and its line must say so:\n%s", out)
		}
		if !strings.Contains(out, "1 of 1 window(s) closed by a refill — treatment arm") {
			t.Errorf("the report's arm must be counted from the windows, not stamped from a constant:\n%s", out)
		}
		if strings.Contains(out, "control arm") {
			t.Errorf("the refill has shipped; nothing here may still claim the control arm:\n%s", out)
		}
	})

	t.Run("closed by the head of a pass", func(t *testing.T) {
		b, fake := newTestBackend(t)
		d := newTestDispatcher(t, b)
		writePersona(t, b.App, "ranger", "[go]")
		repo := qaRepo(t, b.App,
			`[{"id":"a-1","title":"t","labels":["go"]}]`,
			`[{"id":"a-1","status":"closed"},{"id":"a-2","status":"closed"}]`)
		agentPerLaunch(t, fake)

		if _, err := d.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(`[{"id":"a-2","title":"u","labels":["go"]}]`), 0o644)
		d.Out = &strings.Builder{}
		d.seatRefills = nil
		if _, err := d.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
		out := dispatcherOut(d)
		if !strings.Contains(out, "a-1 settled") || !strings.Contains(out, "a-2 launched") {
			t.Fatalf("pass 2 must close the window pass 1 opened, or there is no stamp to judge:\n%s", out)
		}
		if !strings.Contains(out, "[ADR 0028 §5 obs.1 baseline]") {
			t.Errorf("a window no refill closed is a baseline window and its line must say so:\n%s", out)
		}
		if !strings.Contains(out, "no window here was closed by a refill — control arm") {
			t.Errorf("a Run that refilled nothing is the control arm and must say which:\n%s", out)
		}
	})
}
