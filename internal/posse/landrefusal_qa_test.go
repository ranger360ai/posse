//go:build posse_arm2

package posse

// The landing refusal's evidence — ranger-base-4yefx, ask 2.
//
// THE INCIDENT. `posse kill <persona>-<repo>-ranger-base-tvokd` refused twice,
// at the default 10m and again at `--timeout 20m`, with one sentence:
// "still working after 10m0s — let it finish and kill it again (or --timeout
// 20m0s, or --no-land …)". Every word true, none of it evidence. The seat's
// bead had closed 41.6h earlier, its tree was clean, and what claude reported
// `working` about was background work its turn had already handed off — so
// the two waits that sentence recommends could not settle, and the 30 minutes
// they cost bought nothing.
//
// Both facts that answer "working at what?" were at that line and thrown
// away: the reading `MuxWaitTimeout.Last` carries (ranger-base-hcig1 put it
// there for this caller in those words) and the pane's own `PaneHold`.
//
// THESE ARMS ARE SPLIT BY WHAT THEY CAN HOLD. The ROUTE is pinned end to end
// over the herdr fake, which is the one occupant a kill fixture exists for
// (memoryRepo/appendOrders): does the refusal carry the stage, the hold and
// the residue clause at all. The WORDING for a Rex-shaped refusal — the one
// that carries a last reading, with the runtime's own msg and updated_at in
// it — is pinned at the unit, because that is where the reading can be the
// one actually measured off the live incident rather than a fake's echo.
//
//	go test -tags posse_arm2 -run TestQALandingRefusal ./internal/posse

import (
	"path/filepath"
	"strings"
	"testing"
)

// ─── the route: a kill whose settle wait runs out ────────────────────────────

func TestQALandingRefusalCarriesWhatItWasLookingAt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	// Memory no commit holds, which is what gates the landing turn at all.
	appendOrders(t, repo, "dev", "- a lesson the landing turn is for.\n")
	// The settle wait runs out: the shape the incident refused on, where no
	// landing prompt is ever typed.
	write(t, filepath.Join(fake, "wait-error"), "timeout|no settled state in w1:p1 within 10m0s")
	// And the screen the incident had: live background work, no turn in
	// flight, nothing in the box.
	armScreen(t, fake, "  ⏵⏵ auto mode on · 3 shells · ↓ to manage\n", emptyBox)

	var out strings.Builder
	_, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true, Out: &out})
	if err == nil {
		t.Fatal("a landing turn that never settled must still stop the kill — that refusal is the loss this path exists to prevent")
	}
	if _, ok := b.readMeta("s1"); !ok {
		t.Error("a refused kill must leave the session's record where it was")
	}
	for _, want := range []string{
		// the sentence that was always there, unchanged
		"still working",
		"--no-land",
		// which leg refused: no prompt was typed on this one
		"waiting for it to go idle before typing the landing prompt",
		// what the pane is holding, in panework.go's own words
		"3 shells still running",
		// the claim this bead is for
		"background work claude carries across a turn boundary is not an edit in flight",
		// and the clause only a kill can add
		"a kill would take nothing uncommitted",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q:\n%v", want, err)
		}
	}
	// herdr's own wait carries no reading back (MuxWaitTimeout's last
	// paragraph), and the refusal must say that rather than let the operator
	// read `still working` as something posse looked at — which is the
	// assumption the two wasted waits were made on.
	if !strings.Contains(err.Error(), "the wait running out and not a reading") {
		t.Errorf("on an occupant that returns no reading, the refusal must say so:\n%v", err)
	}
}

// The other leg: the prompt landed and its turn has not ended. "the text was
// typed" and "nothing was ever typed" are different facts about a seat, and
// the refusal that names neither reads as though the turn never started.
func TestQALandingRefusalNamesTheLegThatRefused(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	appendOrders(t, repo, "dev", "- a lesson.\n")
	write(t, filepath.Join(fake, "prompt-error"), "timeout|timed out waiting for agent status")

	var out strings.Builder
	_, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true, Out: &out})
	if err == nil {
		t.Fatal("a landing turn still running must stop the kill")
	}
	if !strings.Contains(err.Error(), "waiting for the landing turn it was handed to end") {
		t.Errorf("the refusal must say the prompt WAS typed and its turn is what has not ended:\n%v", err)
	}
	if strings.Contains(err.Error(), "before typing the landing prompt") {
		t.Errorf("the refusal named the wrong leg — this one typed the prompt:\n%v", err)
	}
}

// A refusal that could read nothing prints exactly the sentence it printed
// before this file existed. Nothing here may invent a clause, and a reading
// that failed is not evidence about a seat (panework.go's standing rule).
func TestQALandingRefusalWithNothingReadableIsTheOldSentence(t *testing.T) {
	t.Parallel()
	r := landRefusal{}
	if got := r.Lines(); len(got) != 0 {
		t.Errorf("the empty refusal rendered %d clause(s): %q", len(got), got)
	}
	if got := landRefusalTail(nil); got != "" {
		t.Errorf("landRefusalTail(nil) = %q, want the empty string so the Die message is unchanged", got)
	}
}

// ─── the wording for a refusal that DOES carry a reading ─────────────────────

// The Rex shape, with the reading built out of what the live incident would
// have carried: `rexEvidence`'s own rendering of a root program_status
// record, whose `msg` and `updated_at` are the two fields that say what the
// `working` is about and how long it has said it. MEASURED 2026-11-23 on this
// box: post-turn the msg reads "<N> shells still running" and updated_at
// freezes at the turn's end.
func TestQALandingRefusalQuotesTheReadingTheOccupantHeld(t *testing.T) {
	t.Parallel()
	read := MuxReading{
		State: MuxWorking,
		Seen:  true,
		Evidence: rexEvidence(RexStatusRecord{
			State: "working", App: "claude-code",
			Msg:       "3 shells still running",
			UpdatedAt: "2026-11-21T09:23:04.697707754-05:00",
		}),
	}
	r := landRefusal{Last: read, Reading: true, Stage: landStageSettle,
		Hold: PaneHold{Work: "3 shells"}}
	got := strings.Join(r.Lines(), "\n")
	for _, want := range []string{
		"last reading `working`",
		"program_status state=working",
		"msg=3 shells still running",
		"updated_at=2026-11-21T09:23:04",
		"3 shells still running",
		"no --timeout settles it",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the refusal does not carry %q:\n%s", want, got)
		}
	}
	// The occupant answered, so the no-reading clause must NOT fire: that
	// line is about herdr's bare envelope, and printing both would tell the
	// operator two different things about the same silence.
	if strings.Contains(got, "named no state when its window closed") {
		t.Errorf("a refusal carrying a reading claimed the occupant named no state:\n%s", got)
	}
}

// An occupant that WAS asked and named no state is a third answer, kept apart
// from both a timeout with a reading and a refusal with none (MuxReading's
// own doc: the empty state is an answer and not an error).
func TestQALandingRefusalKeepsTheThreeSilencesApart(t *testing.T) {
	t.Parallel()
	asked := landRefusal{Reading: true, Stage: landStageSettle}
	if got := strings.Join(asked.Lines(), "\n"); !strings.Contains(got, "no readable state at all") {
		t.Errorf("an occupant that answered with no state must say so:\n%s", got)
	}
	none := landRefusal{Stage: landStageSettle}
	if got := strings.Join(none.Lines(), "\n"); !strings.Contains(got, "the wait running out and not a reading") {
		t.Errorf("a refusal carrying no reading at all must say THAT:\n%s", got)
	}
}

// The hold's other half: a box holding an unsent prompt is named, and the
// background-work claim is not made over a seat whose footer showed none —
// that sentence is a claim about what `working` is ABOUT, and posse has no
// other evidence for it.
func TestQALandingRefusalDoesNotClaimBackgroundWorkItDidNotRead(t *testing.T) {
	t.Parallel()
	r := landRefusal{Stage: landStagePrompt, Hold: PaneHold{Typed: "commit and close"}}
	got := strings.Join(r.Lines(), "\n")
	if !strings.Contains(got, "UNSENT") {
		t.Errorf("an unsent prompt in the box is part of what the pane is holding:\n%s", got)
	}
	if strings.Contains(got, "background work claude carries") {
		t.Errorf("the refusal claimed background work over a footer that named none:\n%s", got)
	}
}
