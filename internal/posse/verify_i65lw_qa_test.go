//go:build posse_arm2

package posse

// The one conjunct of the `working`-over-a-closed-bead arm that nothing
// witnessed — ranger-base-i65lw, verifying ranger-base-4yefx.
//
// WHY IT IS HERE. mayReapWorkingSeat (reapworking.go) is a conjunction of ten
// clauses in front of a sweep that KILLS SESSIONS, and its own doc lists "the
// records: residueHolds is empty — a kill takes nothing" as a member. MEASURED
// 2026-11-23 at dceb3642, each clause neutralised separately (`&& false`) and
// graded against every reap, landing and kill test in this arm:
//
//	clause                                   line  verdict
//	class == reapFleetClosed                  132  survived — the sweep's own
//	                                               pre-filter refuses the
//	                                               class first, and THAT is
//	                                               pinned (widening
//	                                               workingSeatWorthAStoreRead
//	                                               reds the pre-gate arm)
//	status == "working"                       143  survived, same pre-filter
//	the run record is readable                147  survived — fail-safe, and
//	                                               its drop is a nil meta
//	                                               rather than a wrong reap
//	the screen named background work          151  killed (two arms)
//	no turn in flight                         168  killed
//	the composer is empty                     177  killed
//	the dial is armed                         192  killed
//	the record is dated                       198  survived — subsumed: an
//	                                               undated record reads idle
//	                                               0, which the grace below
//	                                               then refuses anyway, so
//	                                               the drop changes only
//	                                               which line prints
//	the grace has elapsed                     203  killed
//	residueHolds is empty                     208  SURVIVED, and alone among
//	                                               the five it is neither
//	                                               duplicated nor subsumed
//
// THE CLAUSE IS NOT DUPLICATED, which is what makes this arm worth a test
// rather than a note. reapWhy's general grace-and-residue block is guarded by
// `if class != reapFleetClosed` (autoreap.go), so this population never
// reaches it: line 208 is the only residue question asked of a `working` seat
// over a closed bead. Dropping it does not lose the work — the reap lands the
// plane — but it makes this arm a reap that DECIDES, which is the one thing
// residueHolds' own doc says the widened arms may not do ("a reap that lands
// is a reap that decides, and these two arms are the ones the operator did not
// previously trust to decide anything").
//
// THE PAIR IS THE POINT, on this file's neighbour's rule: the two sessions
// below differ in exactly one byte-level fact — one checkout has a file
// nothing committed — so a green keep is caused by the residue and not by a
// fixture that cannot be reaped at all. The branch half of the same question
// (commits the base does not hold) is residueHolds' own business and is
// witnessed for the other two arms in reapresidue_test.go; this arm needs one
// route through the clause, not two.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQAWorkingReapKeepsASeatWhoseCheckoutHoldsUncommittedWork(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")

	// Both are the incident's own shape — dispatch's per-bead session, the
	// bead closed, the occupant reading `working`, the footer naming live
	// background work with no turn in flight, well past the grace — in a
	// real git checkout so residueHolds has something to answer about.
	heldRepo, cleanRepo := wtRepo(t), wtRepo(t)
	held := SessionForBead("ranger", heldRepo, "a-1")
	clean := SessionForBead("ranger", cleanRepo, "a-2")
	residueSession(t, b, NewSessionOpts{Name: held, Dir: heldRepo, Bead: "a-1"}, "closed", DefaultWorkingReapAfter+time.Minute)
	residueSession(t, b, NewSessionOpts{Name: clean, Dir: cleanRepo, Bead: "a-2"}, "closed", DefaultWorkingReapAfter+time.Minute)
	// residueSession's `bd show` answer is a FILE in the session's own dir,
	// so in a real checkout the fixture makes its own tree dirty — both
	// sessions read as holding work and the control proves nothing. Commit
	// it. This is why the residue arms next door, which use this repo
	// helper, all pass an empty bead status: this is the first arm that
	// needs a CLOSED bead and a git tree at once.
	for _, repo := range []string{heldRepo, cleanRepo} {
		mustGit(t, repo, "add", "fake-show.json")
		mustGit(t, repo, "commit", "-q", "-m", "the fixture's bd answer", "--", "fake-show.json")
	}
	// And now the one difference between the two sessions.
	write(t, filepath.Join(heldRepo, "unsaved.txt"), "lines nothing else holds\n")

	// Both panes mid-`working`: the state this whole arm exists to stop
	// reading as "somebody is in there". Named for both workspaces, because
	// a session herdr reports nothing for is `""` — which past RelaunchGrace
	// is settled, and would reap the control for the wrong reason.
	if err := os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"working","pane_id":"w1:p1","workspace_id":"w1"},`+
			`{"agent":"claude","agent_status":"working","pane_id":"w2:p1","workspace_id":"w2"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	armScreen(t, fake, endedTurnFooter, emptyBox)

	d.autoReapPass(afterRouting)
	out := dispatcherOut(d)

	if _, ok := b.readMeta(held); !ok {
		t.Errorf("a `working` seat over a closed bead whose checkout holds uncommitted work was REAPED — the one residue question this population is ever asked is reapworking.go's own, and a reap that lands is a reap that decides:\n%s", out)
	}
	if !strings.Contains(out, "NOT reaped") || !strings.Contains(out, "unsaved.txt") {
		t.Errorf("the keep must name the session AND what it is holding, every pass it is true — a standing decline said in silence is the other half of ranger-base-4yefx:\n%s", out)
	}
	// The control, in the same pass: without the uncommitted file the very
	// same shape IS taken. Without this the test above is satisfied by an
	// arm that refuses everything, which is how a residue clause can read
	// green over a fixture nothing could ever reap.
	if _, ok := b.readMeta(clean); ok {
		t.Errorf("the control was not reaped, so this test's keep says nothing about the residue clause:\n%s", out)
	}
}

// ─── ask 2's other unwitnessed branch ────────────────────────────────────────

// The refusal's kill clause has two branches and only the CLEAN one was ever
// rendered by a test: MEASURED 2026-11-23 at dceb3642, neutralising
// landRefusalKillLines' `a kill WOULD take something` arm
// (landrefusal.go:213) reds nothing in this arm's whole reap/landing/kill
// set, while the clean arm beside it is read by
// TestQALandingRefusalCarriesWhatItWasLookingAt.
//
// WHICH DIRECTION THAT IS. The clause exists so `--no-land` can be typed on
// evidence rather than on hope — "a kill would take nothing uncommitted …
// so --no-land skips only the agent's own wrap-up". Over a session checkout
// that DOES hold work, the unwitnessed branch is the only thing standing
// between that sentence and an operator who types the flag it recommends.
// So this is the same shape as the arm above, one layer out: a clause whose
// absence is silent and whose consequence is somebody else's work.
func TestQALandingRefusalSaysWhenAKillWouldTakeSomething(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	dir := devSession(t, b, "s1")
	appendOrders(t, repo, "dev", "- a lesson the landing turn is for.\n")
	// The session's own checkout, holding work no commit does. devSession's
	// dir is a plain directory, and dirtyPaths asks GIT — so a tree it can
	// answer about is part of the fixture rather than an aside.
	mustGit(t, dir, "init", "-q", "-b", "main", ".")
	write(t, filepath.Join(dir, "unsaved.txt"), "the 353 lines, again\n")
	// The incident's refusal: the settle wait runs out, so no landing prompt
	// is ever typed.
	write(t, filepath.Join(fake, "wait-error"), "timeout|no settled state in w1:p1 within 10m0s")
	armScreen(t, fake, "  ⏵⏵ auto mode on · 3 shells · ↓ to manage\n", emptyBox)

	var out strings.Builder
	_, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true, Out: &out})
	if err == nil {
		t.Fatal("a landing turn that never settled must still stop the kill")
	}
	if !strings.Contains(err.Error(), "a kill WOULD take something") || !strings.Contains(err.Error(), "unsaved.txt") {
		t.Errorf("the refusal must name what a kill would take, and name it by path — this is the clause that decides whether `--no-land` is safe to type:\n%v", err)
	}
	// And it must not say the opposite in the same breath: the two branches
	// are exclusive, and the clean one recommends the flag.
	if strings.Contains(err.Error(), "a kill would take nothing uncommitted") {
		t.Errorf("the refusal told the operator a kill takes nothing over a checkout holding uncommitted work:\n%v", err)
	}
}
