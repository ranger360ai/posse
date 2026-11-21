package posse

// A RUN ITS OWN LAUNCHER'S STOP ENDED IS INTERRUPTED, NOT SILENCE (ADR 0088
// D6.5, from ranger-base-gihmm). landedcheck.go judges the landed tree,
// landedfile.go files D5's attributed bead and landedhealth.go is D6.3's
// watchdog on the judge itself. This file is the one question those three
// could not ask: WHO stopped the judge.
//
// THE INCIDENT. On 2026-11-20/21 D6.3 filed "landed check silent at
// ~/src/posse" (ranger-base-gihmm) over two runs that said `it was signalled
// (signal: terminated)` after 82.9s and 50.0s — far under the 10m bound, so
// neither was a hang, and the same script over the same tree was MEASURED
// green in 146s. Both runs ended in the same second as `systemd: Stopping
// posse dispatch --watch (ranger)`: operator restarts after a delegated
// `posse upgrade` (ranger-base-rrgnc). The unit renders systemd's default
// `KillMode=control-group`, so a stop SIGTERMs every process in the cgroup
// and the judge's bwrap is in that cgroup; a terminal's ctrl-c signals the
// whole foreground process group and does the same. So every supervisor this
// loop runs under delivers its stop to the judge as well, and the launcher
// never cancelled anything — it was waiting on a judge that was already
// dead.
//
// What that cost: a false P1 in architecture's lane, filed by the watchdog
// whose job is to be believed, and an unjudged tip nothing came back for
// (`once()` runs only when a landing schedules it). rrgnc's remedy makes a
// loop restart part of every install, and installs follow landings, so the
// shape recurs by design rather than by bad luck.
//
// SO D6.5 IS A THIRD NON-VERDICT, and it is neither of the other two: it
// records nothing in D4 (no answer was given) and nothing in D6.3's quiet
// counter (the judge was not the one that failed). One ◷ line, and the next
// launcher reads the tip (D6.6).
//
// TWO CASES, AND THEY ARE TWO BECAUSE THE DELIVERIES RACE:
//
//  1. THE LAUNCHER STOPPED IT. The judge's deadline hangs off the stop
//     context (judge(), measure()), so a stop cancels the in-flight child
//     and the run loop drops its pending set at the next cycle. A stop is
//     then bounded by generatorKillGrace instead of holding `Wait()` for up
//     to landedCheckLimit — which under systemd ended in TimeoutStopSec's
//     SIGKILL anyway.
//  2. THE STOP'S OWN SIGNAL REACHED IT FIRST. The cgroup or process group
//     delivers SIGTERM to the judge and to the launcher together, so the
//     child's `Wait` can return before `signal.NotifyContext` has cancelled
//     anything. A run that comes back SIGNALLED therefore waits up to
//     landedInterruptGrace for the stop context before it is classified.
//     ASSUMED, not measured: the two deliveries land within milliseconds of
//     each other, so 5s is a grace and not a guess at the gap. The only cost
//     is up to 5s of latency on a real silence's ⚠ line.
//
// AND A SIGNAL FROM ANYONE ELSE IS STILL SILENCE — an OOM kill, the load
// guard's kill arm (loadguardkill.go), an operator's `kill <judge pid>`.
// Those are exactly what D6.3 exists to see, so the predicate below asks
// about the STOP and never about which signal arrived: "treat every SIGTERM
// as interrupted" is the alternative D6.5 rejects by name.
//
// ONE EDGE IS KNOWN AND ACCEPTED, said here rather than left for the next
// reader to find: a judge killed by its own 10m deadline, in a launcher that
// then stops within the grace, reads INTERRUPTED and is not counted. It
// costs one count out of D6.3's two, and the run was going to be killed
// either way — against the other direction, which is the incident above: a
// restart filing a P1 every time.

import (
	"context"
	"os/exec"
	"sort"
	"time"
)

// landedInterruptGrace is D6.5's case (2): how long a SIGNALLED run waits for
// the stop context before it is classified. The judge and the launcher are
// signalled by one stop, in one cgroup or one process group, and the order
// they are delivered in is not ours to choose — so the classification waits
// for the launcher's own cancellation to arrive rather than reading the
// child's death as a verdict about the judge.
//
// FIVE SECONDS IS A GRACE AND NOT A MEASUREMENT OF THE GAP (D6.5, ASSUMED):
// the two deliveries are milliseconds apart, and what this number buys is
// that a slow `signal.NotifyContext` cannot turn a stop into a filed P1. What
// it costs is up to 5s of latency on a REAL silence's ⚠ line, which is a line
// nothing waits on.
//
// AND IT IS THE OTHER HALF OF WHAT A STOP COSTS, said here rather than left
// to be discovered: D6.5 bounds a stop by generatorKillGrace, and a signalled
// run spends up to THIS grace classifying itself before that wait is over —
// so the honest bound on `Wait()` after a stop is the two of them. Which is
// why this one may not exceed the reaping grace, and why a pin holds it there
// (TestTheInterruptGraceSeamDefaultsToTheShippedGrace).
const landedInterruptGrace = 5 * time.Second

// setStop hands the checker the watch loop's stop context. Called by
// Dispatcher.landedChecks() on every access rather than at construction: the
// checker is made on first use, which can be a pass earlier than the one
// Watch assigns `stopCtx` in, and a checker that captured nil then would
// never honour a stop for the life of the loop.
//
// nil IS LEFT ALONE, never written over a live context: `posse dispatch`
// without --watch and `posse worktrees --land` have no loop to stop, and
// "nil" is what says so (stopping()'s own rule).
func (c *landedChecker) setStop(ctx context.Context) {
	if c == nil || ctx == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stop = ctx
}

// stopRoot is the context a run's deadline hangs off: the stop, so that a
// stop cancels the child, and context.Background() when there is no loop to
// stop. NEVER nil — a nil parent panics in context.WithTimeout, and "no loop"
// must read as "nothing to interrupt me" (the fail-closed direction).
func (c *landedChecker) stopRoot() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stop == nil {
		return context.Background()
	}
	return c.stop
}

// stopFiredLocked reports that this launcher has been asked to stop. The
// caller holds c.mu — the run loop reads it at every cycle, where it already
// holds the lock the pending set is under.
func (c *landedChecker) stopFiredLocked() bool {
	return c.stop != nil && c.stop.Err() != nil
}

// dropOnStopLocked empties the pending set when the launcher is stopping and
// returns the repos it dropped, for the caller to report once it has let the
// lock go. The caller holds c.mu.
//
// A LANDING DROPPED HERE IS NOT A SILENCE EITHER: nothing judged it, nothing
// could have, and D6.6's arm-time sweep reads the tip when the next loop
// arms. Returning the names rather than printing them is what keeps this
// mutex out of the output writer's (loop()).
func (c *landedChecker) dropOnStopLocked() []string {
	if !c.stopFiredLocked() || len(c.pending) == 0 {
		return nil
	}
	dropped := make([]string, 0, len(c.pending))
	for r := range c.pending {
		dropped = append(dropped, r)
		delete(c.pending, r)
	}
	sort.Strings(dropped) // one order, so two repos read the same way twice
	return dropped
}

// graceOf is landedInterruptGrace as this checker reads it, with the test
// seam `interruptGrace` in front of it — ceilingOf's shape and for its
// reason: the arm where the grace EXPIRES is D3's silence, and pinning that
// over the shipped number costs 5s of suite wall for every run of it.
func (c *landedChecker) graceOf() time.Duration {
	if c.interruptGrace <= 0 {
		return landedInterruptGrace
	}
	return c.interruptGrace
}

// landedSignalled reports that a child came back with no exit status of its
// own: killed by a signal, by a deadline, or by a cancelled context. It is
// the same reading landChildVerdict makes to tell a red from a silence
// (`ExitCode() < 0`), asked again here because D6.5's second case is about
// a child that DIED rather than one that answered.
//
// A nil ProcessState is false: the child never ran at all — a cage that would
// not render, a binary that would not exec — and that is silence whoever is
// stopping.
func landedSignalled(cmd *exec.Cmd) bool {
	return cmd != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() < 0
}

// classifyStop is D6.5 itself: turn a silence this launcher's own stop caused
// into landInterrupted, and leave everything else exactly as it was.
//
// IT NARROWS NOTHING ELSE. A green is a green and a red is a red — a check
// that ANSWERED before the stop reached it has judged the tree, and throwing
// that away would lose the one verdict the run bought. Only D3's silence is
// reconsidered, and only in the two cases the header names.
//
// THE WAIT IS THE SECOND CASE AND IT IS BOUNDED BY graceOf(). It is not
// reached when there is no loop to stop (nil), when the stop has already
// fired (case 1 answers immediately), or when the run was never signalled —
// a refused cage and an unmade worktree are silence without a child, and
// waiting on them would add the grace to every one of D6.3's real findings.
func (c *landedChecker) classifyStop(v landVerdict, signalled bool) landVerdict {
	if v != landUnanswered {
		return v
	}
	stop := c.stopRoot()
	if stop.Done() == nil {
		return v // no loop to stop: context.Background() has no Done channel
	}
	if stop.Err() != nil {
		return landInterrupted // case 1: the launcher cancelled this run
	}
	if !signalled {
		return v
	}
	// Case 2: the stop's own signal reached the judge first. Wait for this
	// launcher's cancellation to arrive before calling the child's death a
	// silence.
	t := time.NewTimer(c.graceOf())
	defer t.Stop()
	select {
	case <-stop.Done():
		return landInterrupted
	case <-t.C:
		return v // nobody stopped us: D3's silence, counted as it always was
	}
}
