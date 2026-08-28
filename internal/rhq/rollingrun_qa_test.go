package rhq

// ranger-base-t8tq: what ADR 0028 §1's long-lived Run did to everything that
// used to be denominated in a PASS.
//
// MEASURED on a live loop, 2026-08-28 (dispatch-watch.log): one pass ran
// 7h09m, and the next "pass" line in the log is a bounce, not a second pass.
// Inside those seven hours, two things that had always happened "every pass"
// happened exactly once, at the top:
//
//   - the auto-reap sweep. 22 per-bead sessions over closed beads piled up
//     behind it; they were hand-reaped, and a bounced loop swept the rest in
//     its first seconds. Nothing was wrong with the sweep — it ran once per
//     Run, and the Run did not end.
//   - the seat offer. Every refill was narrowed to the persona whose bead had
//     just settled, and every seat busy at the head of the pass was written
//     into the Run's busy map and never read again. ~90% of that day's closes
//     are the one seat that kept settling; three seats with ready work in
//     their lanes sat out the day.
//
// These are the pins for both, and for the third symptom the same shape
// produced — `-n` denominated per epoch while the flag still said 6.
//
// Each test says which mutation reds it; all four were run.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The sweep, at the event that makes a session sweepable.
//
// Two beads for one seat under a refilling Run: a-1 fires, settles closed,
// and its per-bead session is then a graveyard entry — closed bead, idle
// agent, nobody's. The Run goes on to fire a-2 into the freed seat. The
// pin is the ORDER: a-1's session must be reaped BEFORE a-2 launches, which
// is only true if the settle itself sweeps. With the sweep only at Run start
// and Run end, "reaped" still appears — in the epilogue, after every launch
// this Run makes — so an assertion that the line exists at all would be
// green on the bug. MUTATION: delete the `d.autoReapPass()` at the settle in
// Run's gather loop → the reap line moves after a-2's launch → red.
//
// PromptGrace is zeroed because the grace is the sweep's other guard (ADR
// 0028 §3) and a session this Run prompted seconds ago is inside it: the
// production pile-up was hours old, and a test that waited out 30s of grace
// would be pinning the clock instead of the call site.
func TestQAReapSweepsAtEverySettleUnderARollingRun(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","status":"closed"},{"id":"a-2","status":"closed"}]`)
	// The queue the refill re-reads: a-1 closed and gone from `bd ready`
	// (what real bd does with it), a-2 waiting. Without the swap the fake
	// hands a-1 back forever and the refill spends the seat re-launching a
	// closed bead instead of taking the next one.
	write(t, filepath.Join(repo, "fake-ready-next.json"), `[{"id":"a-2","title":"u","labels":["go"]}]`)
	agentPerLaunch(t, fake)
	d.Refill = true
	d.PromptGrace = 0

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)

	// The witness that the fixture ran at all: both beads went out, in the
	// order the refill makes them go out. An absence-only assertion would be
	// satisfied by a Run that never launched anything.
	launch1 := strings.Index(out, "· a-1            creating session")
	launch2 := strings.Index(out, "· a-2            creating session")
	if launch1 < 0 || launch2 < 0 {
		t.Fatalf("both beads must launch inside this one Run:\n%s", out)
	}
	reap := strings.Index(out, "reaped "+SessionForBead("ranger", repo, "a-1"))
	if reap < 0 {
		t.Fatalf("a-1's session is closed, idle and outside the prompt grace — the sweep must reap it:\n%s", out)
	}
	if reap > launch2 {
		t.Errorf("a-1's session must be reaped at its own settle, not in the Run's epilogue — a Run that refills does not reach its epilogue (reap@%d, a-2 launch@%d):\n%s", reap, launch2, out)
	}
}

// The readings a fire pass takes about a seat expire with the fire pass.
//
// This is fireLoop called twice over one busy map — exactly what Run and its
// refires do (Run creates the map, fireLoop and every refire share it). The
// seat "hopper" is working when the first call walks the lane and free when
// the second one does, and the second call must FIRE it: occupancy is a live
// read, and the map is for the seats this Run put beads on.
//
// MUTATION: put the `personaActive` verdict back in the Run map
// (`seats.note` → `busy[slot] = true` in seatFor) → the second call still
// believes the first call's reading → no launch → red.
func TestQASeatBusyInOneFirePassIsReReadInTheNext(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "hopper", "[rust]")
	repo := qaRepo(t, b.App, `[{"id":"b-1","title":"t","labels":["rust"]}]`, `[{"id":"b-1","status":"closed"}]`)
	// hopper is mid-turn in this repo: its own pre-Dial-F session, with an
	// agent herdr calls working — what personaActive reads as a busy seat.
	if err := b.CreateSession(NewSessionOpts{Name: SessionFor("hopper", repo), Dir: repo, Agent: "hopper"}); err != nil {
		t.Fatal(err)
	}
	workingClaude(t, fake)

	beads := []RepoIssue{{BdIssue: BdIssue{ID: "b-1", Title: "t", Labels: []string{"rust"}}, Dir: repo}}
	busy := map[string]bool{}
	sessFail := map[string]int{}
	bead := SessionForBead("hopper", repo, "b-1")

	if _, _, _, err := d.fireLoop(beads, "", 0, busy, sessFail); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(calls(t, fake), "workspace create --label "+bead) {
		t.Fatalf("a working seat must not be fired into:\n%s", dispatcherOut(d))
	}
	if !strings.Contains(dispatcherOut(d), "lane busy: hopper") {
		t.Fatalf("the fixture must actually make hopper busy, or the second pass proves nothing:\n%s", dispatcherOut(d))
	}
	if busy[SessionFor("hopper", repo)] {
		t.Error("a seat this Run did not fire into may not be recorded as this Run's occupancy — that reading is the fire pass's")
	}

	// The turn ended: herdr sees no agent in hopper's session any more.
	os.WriteFile(filepath.Join(fake, "agents.json"), []byte(`[]`), 0o644)
	agentPerLaunch(t, fake)

	if _, _, _, err := d.fireLoop(beads, "", 0, busy, sessFail); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, fake); !strings.Contains(log, "workspace create --label "+bead) {
		t.Errorf("hopper is free now — the next fire pass must re-read the seat and launch, not replay the last pass's reading:\n%s\n%s", dispatcherOut(d), log)
	}
}

// End to end, the shape the operator watched all day: a seat busy when the
// Run started, ready work in its lane, and one other seat cycling. The
// starved seat must get its bead inside the SAME Run.
//
// The fixture makes hopper busy at the head of the pass (its own session,
// agent working) and lets ranger's launch be what frees it — the fake
// replaces the agent listing on every pane run, which is a settle from
// hopper's point of view and needs no clock. So b-1 is skipped "lane busy"
// on the way in, and the only thing that can still launch it is the refill.
//
// TWO mutations red it, because the defect had two halves and either one
// alone re-starves the seat:
//   - `d.refire(personaFilter, …)` → `d.refire(g.persona, …)`: the refill is
//     narrowed to the settled persona again and b-1 is "outside ranger's
//     lane" forever.
//   - `seats.note` → `busy[slot] = true` in seatFor: hopper stays busy on the
//     morning's reading and the unnarrowed refill skips it just the same.
//
// The no-Refill arm is the control: a one-shot Run has no refill at all, so
// b-1 legitimately waits for a later pass, and the arm is what keeps the
// Refill arm's launch from reading as something every Run always did.
func TestQARefillOffersTheQueueToASeatThisRunNeverFired(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refill  bool
		starved bool // b-1 goes out inside this Run
	}{
		{"one-shot: the busy seat's bead waits for a later pass", false, false},
		{"rolling: the busy seat is re-offered the queue at the next settle", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writePersona(t, b.App, "ranger", "[go]")
			writePersona(t, b.App, "hopper", "[rust]")
			// b-1 is P0 so the seat walk reaches it BEFORE ranger's launch
			// wipes the agent listing: the skip has to be taken while hopper
			// really is working, or the pass proves nothing.
			repo := qaRepo(t, b.App,
				`[{"id":"b-1","title":"starved","priority":0,"labels":["rust"]},{"id":"a-1","title":"hot","priority":1,"labels":["go"]}]`,
				`[{"id":"a-1","status":"closed"},{"id":"b-1","status":"closed"}]`)
			if err := b.CreateSession(NewSessionOpts{Name: SessionFor("hopper", repo), Dir: repo, Agent: "hopper"}); err != nil {
				t.Fatal(err)
			}
			workingClaude(t, fake)
			agentPerLaunch(t, fake)
			d.Refill = tc.refill

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out, log := dispatcherOut(d), calls(t, fake)

			// Witnesses, in both arms: hopper really was busy on the way in,
			// and the hot seat really did launch and settle.
			if !strings.Contains(out, "lane busy: hopper") {
				t.Fatalf("the fixture must make hopper busy at the head of the pass:\n%s", out)
			}
			settle := strings.Index(out, "✓ a-1")
			if settle < 0 || !strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
				t.Fatalf("the hot seat must launch and settle in both arms:\n%s\n%s", out, log)
			}

			launch := strings.Index(log, "workspace create --label "+SessionForBead("hopper", repo, "b-1"))
			if !tc.starved {
				if launch >= 0 {
					t.Errorf("a one-shot Run refills nothing — b-1 waits for a later pass (ADR 0028 §4):\n%s", log)
				}
				return
			}
			if launch < 0 {
				t.Errorf("the seat this Run never fired into is free now and its bead is ready — the refill must offer it the queue:\n%s\n%s", out, log)
			}
			// And only after the settle that freed the tick: occupancy is
			// still live (ADR 0028 §5 observable 4), so this is a refill and
			// not the initial fire loop having ignored a working seat.
			if l := strings.Index(out, "· b-1            creating session"); l >= 0 && l < settle {
				t.Errorf("b-1 must launch after a-1 settled, not while hopper was still working (launch@%d settle@%d):\n%s", l, settle, out)
			}
		})
	}
}

// Fix ask (b): the loop says what `-n` is denominated in, out loud, once,
// where the log begins. The stale flag that starved the shop was `-n 6`
// carried over from the per-pass days, and nothing printed the unit — the
// only line that mentioned an epoch was the exhaustion line, which the
// seats that never launched never reached.
//
// MUTATION: drop the LaunchCapLine call from Watch → red.
func TestQAWatchNamesTheLaunchCapsDenomination(t *testing.T) {
	if got := LaunchCapLine(6, time.Hour); !strings.Contains(got, "-n 6") ||
		!strings.Contains(got, "per 1h0m0s EPOCH") || !strings.Contains(got, "not per pass") {
		t.Errorf("the cap line must name the number AND the unit it is spent in, got:\n%s", got)
	}
	if got := LaunchCapLine(0, 30*time.Minute); !strings.Contains(got, "no cap") || !strings.Contains(got, "30m0s") {
		t.Errorf("-n 0 is no cap, and the epoch is still worth naming, got:\n%s", got)
	}

	// And it is really on the loop's own first lines, not merely available:
	// a stopped context runs zero passes, so anything in d.Out here was
	// printed by the loop's preamble.
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	qaRepo(t, b.App, `[]`, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := d.Watch(ctx, "", "", 6, 10*time.Millisecond, 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dispatcherOut(d), LaunchCapLine(6, time.Hour)) {
		t.Errorf("--watch must name the launch cap's denomination at the top of its log:\n%s", dispatcherOut(d))
	}
}
