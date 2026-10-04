//go:build posse_arm2

package posse

// ranger-base-eh1kr: the queue a pass works is `bd ready` PLUS the
// interrupted runs that query excludes by definition (interrupted.go).
//
// Every one of these drives the bead through `bd list --status in_progress`
// ALONE — `fake-ready.json` is empty in all of them — because that is the
// only fixture shape real bd can produce. The fake's `ready` case serves its
// file whole, in_progress rows included, so a test that puts the bead in
// fake-ready.json is testing the fire loop against a queue bd never hands
// it: that is exactly how two stranded claims (ranger-base-4mrmc,
// ranger-base-mz8ud) stayed invisible to dispatch while 49 test files
// exercised the in_progress branches of the loop.
//
// MUTATION-CHECKED, one red each and all six run: deleting the
// `interruptedRuns` call in `Run` reds the relaunch pin and both dry-run
// arms; deleting the one in `refire` reds ONLY the refill pin's rolling arm,
// which is what says the two call sites are read separately; widening
// interruptedRun to offer every holder reds the worked-bead pin; dropping
// the `bd blocked` subtraction reds the blocked-claim pin; dropping the
// lane-of-one rule reds the foreign-claim pin; dropping heldLane's
// exact-spelling clause reds the folded-assignee pin.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedClaimed is the fake bd's own STATE — the store's answer to
// `update --claim`, which is not fake-show.json and not fake-list.json.
// Without it the fake grants the claim as if the bead were unheld, and the
// resume arm this pins (Bd.Claim's "held by this actor already", rangerhq-kux)
// never fires. The path is the one fakeBdStatePath builds, from the cwd bd
// runs in, which for a dispatch pass is the repo.
func seedClaimed(t *testing.T, fake, repo, id, persona string) {
	t.Helper()
	st := map[string]fakeBdIssue{id: {ID: id, Status: "in_progress", Assignee: &persona}}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	// Both spellings of the repo path: bd's cwd is reported RESOLVED
	// (/private/var/… on darwin, where t.TempDir hands back /var/…), and
	// which one fakeBdStatePath renders is the child's own reading.
	paths := map[string]bool{repo: true}
	if real, err := filepath.EvalSymlinks(repo); err == nil {
		paths[real] = true
	}
	for dir := range paths {
		name := "bd-state-" + strings.ReplaceAll(strings.TrimPrefix(dir, "/"), "/", "_") + ".json"
		if err := os.WriteFile(filepath.Join(fake, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// heldRepo is a repo whose ready queue is EMPTY and whose claimed list holds
// one bead, assigned to persona and in_progress — `posse kill --no-land`'s
// leftovers, and the shape `bd ready` cannot report.
func heldRepo(t *testing.T, a *App, persona, id string) string {
	t.Helper()
	row := []map[string]any{{
		"id": id, "title": "t", "status": "in_progress", "assignee": persona,
		"labels": []string{"go"},
	}}
	repo := qaRepo(t, a, `[]`, "")
	writeJSON(t, repo, "fake-list.json", row)
	writeJSON(t, repo, "fake-show.json", row)
	return repo
}

// The bug itself. The holder session is gone — killed, reaped or crashed —
// the bead is still claimed, and nothing in bd's ready query will ever
// mention it. The pass relaunches it in a session of its own and the claim
// RESUMES rather than being taken afresh.
func TestQADispatchRelaunchesAnInterruptedRunBdReadyCannotSee(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := heldRepo(t, b.App, "ranger", "a-1")
	seedClaimed(t, fake, repo, "a-1", "ranger")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 1 {
		t.Fatalf("a stranded claim was not relaunched (n=%d):\n%s", n, out)
	}
	if want := SessionForBead("ranger", repo, "a-1"); !strings.Contains(calls(t, fake), "workspace create --label "+want) {
		t.Errorf("no session was created for the resumed bead (want %s):\n%s", want, calls(t, fake))
	}
	if !strings.Contains(delivered(t, b.App, fake), "Work beads issue a-1") {
		t.Errorf("the relaunched session was never told which bead it holds:\n%s", delivered(t, b.App, fake))
	}
	// The claim is RESUMED, not re-taken: the assignee the work was done
	// under survives the relaunch (Bd.Claim's own arm, rangerhq-kux).
	if !strings.Contains(out, "already claimed by ranger — resuming") {
		t.Errorf("the relaunch took the claim afresh instead of resuming it:\n%s", out)
	}
}

// --dry-run names it too. `posse dispatch --dry-run -n 0 --resume` printing
// no line at all for either stranded bead is how the operator found this:
// a diagnostic that cannot see the population is worse than no diagnostic,
// because it reads as "there is nothing there".
func TestQADryRunNamesAnInterruptedRun(t *testing.T) {
	t.Parallel()
	for _, leg := range []struct {
		name string
		set  func(*Dispatcher)
	}{
		{"--dry-run", func(d *Dispatcher) { d.DryRun = true }},
		{"--dry-run --resume", func(d *Dispatcher) { d.DryRun, d.Resume = true, true }},
	} {
		t.Run(leg.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			writePersona(t, b.App, "ranger", "[go]")
			heldRepo(t, b.App, "ranger", "a-1")
			idleClaude(t, fake)
			agentPerLaunch(t, fake)

			d := newTestDispatcher(t, b)
			leg.set(d)
			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if !strings.Contains(out, "a-1") {
				t.Errorf("a dry pass said nothing about the stranded claim:\n%s", out)
			}
			if strings.Contains(out, "no ready work") {
				t.Errorf("a queue holding an interrupted run reported itself empty:\n%s", out)
			}
		})
	}
}

// The other half of the population, and the one that must stay out: a bead
// whose own session is live with an agent mid-turn is being WORKED. The
// cockpit's IN PROGRESS section is where that is read (ADR 0004 §2); a line
// per pass per in-flight bead would bury the lines that mean something, and
// a second session beside a working one is the twin every guard in the fire
// loop exists to prevent.
func TestQADispatchLeavesAWorkedBeadAlone(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := heldRepo(t, b.App, "ranger", "a-1")
	holder := SessionForBead("ranger", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: holder, Dir: repo, Agent: "ranger", Bead: "a-1"})
	sessionAgent(t, fake, holder, "working")

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 || strings.Contains(out, "a-1") {
		t.Errorf("a bead with a working holder was offered a seat (n=%d):\n%s", n, out)
	}
	// The twin cannot be seen in the create log — a relaunch of the holder
	// and a twin beside it carry the same Dial F label — so what is asserted
	// is that nothing was TYPED at the working agent and nothing re-claimed
	// the bead under it.
	if strings.Contains(delivered(t, b.App, fake), "Work beads issue a-1") {
		t.Errorf("a prompt was typed at an agent mid-turn:\n%s", delivered(t, b.App, fake))
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("a bead being worked was re-claimed:\n%s", bdCalls(t, fake))
	}
}

// A claim whose dependencies are not met is not dispatchable work, and this
// scan subtracts `bd blocked` exactly as Bd.Ready does (ranger-base-lpz0o:
// `bd blocked` lists in_progress rows, and the live queue had one as this
// was written). ranger-base-mz8ud sat here until its blocker closed — and
// from that moment it is the first test's shape, not this one's.
func TestQADispatchLeavesABlockedClaimAlone(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	repo := heldRepo(t, b.App, "ranger", "a-1")
	writeJSON(t, repo, "fake-blocked.json", []map[string]any{{"id": "a-1", "title": "t"}})
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); n != 0 || strings.Contains(out, "a-1") {
		t.Errorf("a dep-blocked claim was dispatched (n=%d):\n%s", n, out)
	}
}

// The claim is not re-laned. An in_progress bead held by somebody who is not
// a persona here — the operator's own hand-work, a name whose PID was
// removed — is NOT offered to whoever owns its labels: the claim would lose
// (ClaimLostError) after a seat and a session had been spent on it, and ADR
// 0020 §2's rule is that an assignment is never silently rerouted.
func TestQADispatchDoesNotRelaneAForeignClaim(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	heldRepo(t, b.App, "quartermaster", "a-1") // loads no PID in this fixture
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); n != 0 || strings.Contains(out, "a-1") {
		t.Errorf("a claim held by a non-persona was routed by label (n=%d):\n%s", n, out)
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("a bead somebody else holds was claimed:\n%s", bdCalls(t, fake))
	}
}

// The refill reads the claimed half too (ADR 0028 §1). A rolling Run under
// --watch may never return to the fire pass — 7h09m in one measured pass
// (ranger-base-t8tq) — so a stranded claim whose seat was busy at the head
// of that pass has to be re-offered at the settle that frees it, or it waits
// for a bounce.
//
// The shape, in one fixture: hopper is mid-turn on another bead's session
// when the pass starts, so the claim it holds is skipped lane-busy; ranger's
// ready bead launches and settles; the refill finds hopper free and relaunches
// the claim.
//
// MUTATION: drop the interruptedRuns call in refire → red (the one-shot arm
// stays green, which is what says the two call sites are read separately).
func TestQARefillRelaunchesAnInterruptedRun(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		refill   bool
		relaunch bool
	}{
		{"one-shot: the claim waits for a later pass", false, false},
		{"rolling: the claim goes out at the settle that frees the seat", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			writePersona(t, b.App, "ranger", "[go]")
			writePersona(t, b.App, "hopper", "[rust]")
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"hot","priority":1,"labels":["go"]}]`,
				`[{"id":"a-1","status":"closed"},{"id":"b-1","title":"t","status":"in_progress","assignee":"hopper"}]`)
			writeJSON(t, repo, "fake-list.json", []map[string]any{
				{"id": "b-1", "title": "t", "status": "in_progress", "assignee": "hopper"},
			})
			// hopper is working — on ANOTHER bead's session, so this claim's
			// own holder join still answers "nobody" and the bead is an
			// interrupted run, not one being worked.
			mustCreate(t, b, NewSessionOpts{Name: SessionForBead("hopper", repo, "b-9"), Dir: repo, Agent: "hopper", Bead: "b-9"})
			workingClaude(t, fake)
			agentPerLaunch(t, fake)

			d := newTestDispatcher(t, b)
			d.Refill = tc.refill
			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out, log := dispatcherOut(d), calls(t, fake)
			if !strings.Contains(out, "busy this pass") {
				t.Fatalf("the fixture must make hopper busy at the head of the pass:\n%s", out)
			}
			settle := strings.Index(out, "✓ a-1")
			if settle < 0 {
				t.Fatalf("the hot seat must launch and settle in both arms:\n%s", out)
			}
			launch := strings.Index(log, "workspace create --label "+SessionForBead("hopper", repo, "b-1"))
			if !tc.relaunch {
				if launch >= 0 {
					t.Errorf("a one-shot Run refills nothing — the claim waits for a later pass (ADR 0028 §4):\n%s", log)
				}
				return
			}
			if launch < 0 {
				t.Errorf("the seat is free now and a stranded claim is waiting — the refill must offer it the queue:\n%s\n%s", out, log)
			}
		})
	}
}

// The claim's spelling is not re-attributed either. CanonAgent folds case,
// so `Ranger` resolves to the persona `ranger` — but every in_progress
// branch of the fire loop compares `is.Assignee == persona` as a string, so
// a folded name arrives with the holder join, the crew shield and ADR 0030's
// tiebreak all abstaining and gets claimed afresh under the other spelling.
// A resume keeps the claim it was made under.
//
// MUTATION: drop heldLane's `lane.seats[0].name != is.Assignee` clause → red.
func TestQADispatchDoesNotReattributeAFoldedAssignee(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	heldRepo(t, b.App, "Ranger", "a-1")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	d := newTestDispatcher(t, b)
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); n != 0 || strings.Contains(out, "a-1") {
		t.Errorf("a claim held under another spelling was dispatched (n=%d):\n%s", n, out)
	}
	if strings.Contains(bdCalls(t, fake), "--claim") {
		t.Errorf("the claim was re-taken under the folded name:\n%s", bdCalls(t, fake))
	}
}
