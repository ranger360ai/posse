//go:build posse_arm2

package posse

// ADR 0088 D6.5 at the site it classifies: a run its own launcher's stop
// ended is INTERRUPTED, and a run signalled by anyone else is still D3's
// silence (ranger-base-soqa1, from ranger-base-gihmm; landedinterrupt.go's
// header has the incident and the two cases).
//
// THE FIXTURE IS A REAL CHILD THAT SIGNALS ITSELF, which is the whole reason
// these arms mean anything. What the incident produced was `it was signalled
// (signal: terminated)` out of a judge the cgroup's SIGTERM reached — so a
// stubbed verdict would pin the table and nothing about the reading it is
// made from. `kill -TERM $$` inside the check is that exact shape: a child
// that comes back with no exit status of its own, with the launcher's own
// stop either fired, about to fire, or never coming. One `kill` of one pid
// and never a pattern, which is AGENTS.md's rule and here also the only
// spelling that can be about THIS test's child.
//
// THE TWO ARMS THAT MATTER ARE THE SAME RUN WITH ONE DIFFERENCE, and it is
// not the signal: it is whether this launcher was stopping. That is D6.5's
// claim ("what matters is who stopped the judge, not which signal it got"),
// and the pair below — ...IsInterrupted and ...StaysSilence — is where the
// two readings are held apart over one byte-identical child.
//
// THE GRACE IS SEAMED (landedChecker.interruptGrace, ldiGrace) for
// `ceiling`'s reason one rule over: the arm where the grace EXPIRES is the
// one D6.3 must keep counting, and over the shipped 5s every run of it would
// spend 5s of suite wall. The seam's default is pinned too.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ldiGrace is the grace these arms run with: long enough that a test which
// has just seen the child's own log line can cancel inside it, short enough
// that the arm where it expires costs no suite wall worth naming.
const ldiGrace = 2 * time.Second

// ldiSignalBody is a check that announces itself and then dies by a signal,
// which is the incident's shape: `ExitCode() < 0`, no verdict, and a
// sentence that says `signal: terminated`.
//
// The trailing `exit 0` is unreachable and is the point — if the signal were
// somehow not delivered, the arm would read GREEN and fail loudly rather than
// read a silence that happened for another reason.
func ldiSignalBody(log string) string {
	return "#!/bin/sh\n" +
		"printf 'v1 %s\\n' \"$(git rev-parse HEAD)\" >> " + shq(log) + "\n" +
		"kill -TERM $$\n" +
		"sleep 5\n" +
		"exit 0\n"
}

// ldiFixture is landedcheck_test.go's fixture with the body in the caller's
// hands and D6.5's two seams set: the grace, and a stop context that is
// nil until an arm hands one over.
func ldiFixture(t *testing.T, body func(log string) string) (c *landedChecker, out *bytes.Buffer, repo, log string) {
	t.Helper()
	a := wtApp(t)
	repo = wtRepo(t)
	log = filepath.Join(t.TempDir(), "runs")
	shipLandedCheck(t, repo, body(log), "ranger-base-soqa1: ship the landed check")
	write(t, filepath.Join(repo, "fake-show.json"), ldfStore)
	out = &bytes.Buffer{}
	c = newLandedChecker(a, out)
	c.cage, c.bd = plainCage, testBd(t)
	c.interruptGrace = ldiGrace
	return c, out, repo, log
}

// ldiSlowOut is the checker's writer, with every write made to take its
// time, and it is what turns an ORDERING claim into a deterministic arm.
//
// `Wait()` is the join ADR 0088 D2 makes a pass take, so a line this runner
// owes has to be written before Wait returns — and the first draft of the
// drop below closed the done channel first. That was green in isolation and
// red in the arm-2 suite under load (ranger-base-soqa1), which is the worst
// shape a pin can have: it holds the claim only when the box is busy. A
// writer that sleeps makes the window wide every time.
//
// It is mutex-guarded because the point is that a reader and the loop
// goroutine touch it at the same instant: a bare bytes.Buffer there is a data
// race the race arm would report instead of a failure anyone can read.
type ldiSlowOut struct {
	d  time.Duration
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *ldiSlowOut) Write(p []byte) (int, error) {
	time.Sleep(w.d)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *ldiSlowOut) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// ldiStop is a stop context an arm can fire by hand, which is what
// `signal.NotifyContext` hands the loop in production.
func ldiStop(t *testing.T, c *landedChecker) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.setStop(ctx)
	return cancel
}

// ldiLine is the one output line naming `want`, so a claim about a verdict
// line is read against THAT line and not against the whole buffer — which is
// TestEveryVerdictLinePrintsItsWall's rule (ranger-base-mdku2).
func ldiLine(s, want string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, want) {
			return l
		}
	}
	return ""
}

// The ◷ line's two halves, named so the arms that assert its presence and the
// arms that assert its ABSENCE cannot drift apart.
const (
	ldiStopping  = "was not judged: this launcher is stopping"
	ldiNotJudged = "NOT judged"
)

// DONE-WHEN (i): a judge signalled while this launcher's stop is cancelled
// within the grace reads INTERRUPTED — one ◷ line, and D6.3's counter
// untouched.
//
// WHY THE COUNTER IS THE ASSERTION AND NOT JUST THE LINE: the counter is what
// filed the false P1 (ranger-base-gihmm). Two restarts in a row counted two
// silences and filed a bead in architecture's lane saying no landing was
// being judged, over a judge that was green in 146s on the same tree.
func TestAJudgeEndedByItsOwnLaunchersStopIsInterruptedAndNotCounted(t *testing.T) {
	t.Parallel()
	c, out, repo, log := ldiFixture(t, ldiSignalBody)
	cancel := ldiStop(t, c)
	tip := mustGit(t, repo, "rev-parse", "HEAD")

	c.Schedule(repo, "main")
	// Inside the run: the child has announced itself and is about to die by
	// its own SIGTERM. The stop fires HERE, which is the race D6.5's second
	// case is about — the cgroup reaches the judge and the launcher in one
	// stop, and either delivery may be first.
	landedWaitFor(t, "the judge to start", func() bool { return len(landedRuns(t, log)) == 1 })
	cancel()
	c.Wait()

	s := out.String()
	line := ldiLine(s, ldiStopping)
	if line == "" {
		t.Fatalf("no ◷ line for a run this launcher's own stop ended (ADR 0088 D6.5):\n%s", s)
	}
	for _, want := range []string{"◷", abbrevSHA(tip), "the next launcher reads the tip", "ADR 0088 D6.5"} {
		if !strings.Contains(line, want) {
			t.Errorf("the interrupted line does not name %q — D6.5 prescribes it:\n%s", want, line)
		}
	}
	// D6.2 reaches this line too: every line this runner prints about a run
	// carries the run's wall, including the ones that are not verdicts.
	if !landedWallSaid.MatchString(line) {
		t.Errorf("the interrupted line carries no wall (ADR 0088 D6.2):\n%s", line)
	}
	if strings.Contains(s, ldiNotJudged) {
		t.Errorf("the run was ALSO reported as a silence — INTERRUPTED is a third non-verdict and not a second sentence about the same run (ADR 0088 D6.5):\n%s", s)
	}
	// The three records, none of which an interrupted run may touch.
	if n := lhQuiet(t, repo); n != 0 {
		t.Errorf("%s counted %d over an interrupted run, want 0 — the judge was not the one that failed (ADR 0088 D6.5)", landedQuietKey, n)
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); v != landUnanswered {
		t.Errorf("the run recorded %s %s — no answer was given, so D4's record may hold nothing (ADR 0088 D6.5)", tree, v)
	}
	if open := lhBeads(t, c, repo); len(open) != 0 {
		t.Errorf("an interrupted run filed %s — a launcher restart is not a finding about anything (ADR 0088 D6.5)", ldfTitles(open))
	}
}

// DONE-WHEN (ii): the same child, signalled with NO stop, is D3's silence and
// is counted. This is the arm that makes the one above mean something — a
// rule that read every signal as a stop would hide the OOM kill, the load
// guard's kill arm and an operator's `kill <judge pid>`, which are exactly
// what D6.3 exists to see.
func TestAJudgeSignalledWithNoStopIsSilenceAndCounted(t *testing.T) {
	t.Parallel()
	c, out, repo, log := ldiFixture(t, ldiSignalBody)
	// No stop at all: a one-shot `posse dispatch` has no loop to stop, and
	// nil means that and never "stopped".
	c.Schedule(repo, "main")
	c.Wait()

	s := out.String()
	if !strings.Contains(s, ldiNotJudged) {
		t.Fatalf("a signalled judge under no stop printed no silence line (ADR 0088 D3):\n%s", s)
	}
	if strings.Contains(s, ldiStopping) {
		t.Errorf("a signalled judge under NO stop was called interrupted — nothing stopped this launcher (ADR 0088 D6.5):\n%s", s)
	}
	if n := lhQuiet(t, repo); n != 1 {
		t.Errorf("%s counted %d, want 1 — a signal from anyone other than this launcher's stop still counts (ADR 0088 D6.5)", landedQuietKey, n)
	}
	if n := len(landedRuns(t, log)); n != 1 {
		t.Errorf("%d judge run(s), want 1 — the arm above is only about a child that really ran and really died", n)
	}
}

// And the third reading, which is the one neither of the two above can make:
// a live loop that is NOT stopping, whose judge is signalled by somebody
// else. The grace EXPIRES and the run is silence — counted, so D6.3 still
// files on the second one.
func TestASignalUnderALiveLoopThatIsNotStoppingStaysSilence(t *testing.T) {
	t.Parallel()
	c, out, repo, _ := ldiFixture(t, ldiSignalBody)
	c.interruptGrace = 150 * time.Millisecond // the expiry, priced for a suite
	ldiStop(t, c)                             // a stop context that never fires

	start := time.Now()
	c.Schedule(repo, "main")
	c.Wait()
	took := time.Since(start)

	s := out.String()
	if strings.Contains(s, ldiStopping) {
		t.Errorf("a signalled judge under a loop that is not stopping was called interrupted (ADR 0088 D6.5):\n%s", s)
	}
	if !strings.Contains(s, ldiNotJudged) {
		t.Fatalf("no silence line (ADR 0088 D3):\n%s", s)
	}
	if n := lhQuiet(t, repo); n != 1 {
		t.Errorf("%s counted %d, want 1", landedQuietKey, n)
	}
	if took < c.interruptGrace {
		t.Errorf("the run was classified in %s, under the %s grace — a signalled run waits for the stop before it is called a silence, which is what keeps the race from deciding (ADR 0088 D6.5)", took, c.interruptGrace)
	}
}

// DONE-WHEN (iii): a stop during a long judge CANCELS it, so `Wait()` returns
// within the reaping grace instead of holding the stop for up to
// landedCheckLimit — which under systemd ended in TimeoutStopSec's SIGKILL
// anyway (D6.5's first case).
func TestAStopDuringALongJudgeCancelsItAndWaitReturns(t *testing.T) {
	t.Parallel()
	c, out, repo, log := ldiFixture(t, func(log string) string {
		return landedCheckBody("v1", log, filepath.Join(filepath.Dir(log), "hold"), 0)
	})
	hold := filepath.Join(filepath.Dir(log), "hold")
	write(t, hold, "held\n")
	cancel := ldiStop(t, c)

	c.Schedule(repo, "main")
	landedWaitFor(t, "the judge to start", func() bool { return len(landedRuns(t, log)) == 1 })
	cancel()
	start := time.Now()
	c.Wait()
	took := time.Since(start)

	// The fixture's own hold is 30s and the shipped bound is 10m, so a
	// `Wait` that waited the judge out would be far over this. The bound is
	// the reaping grace plus room for a loaded box, which is what D6.5
	// promises: "a stop no longer waits up to 10m for a judge".
	if bound := generatorKillGrace + 10*time.Second; took > bound {
		t.Errorf("Wait() took %s after the stop, over the %s bound — a stop must cancel the judge rather than wait it out (ADR 0088 D6.5)", took, bound)
	}
	s := out.String()
	line := ldiLine(s, ldiStopping)
	if line == "" {
		t.Fatalf("the cancelled run printed no ◷ line:\n%s", s)
	}
	// The sentence says what ended the run, and the hang bound is not it: a
	// line quoting "no answer in 10m" over a run cancelled seconds in is a
	// number the reader of a restart's log has to un-learn
	// (landChildVerdict's cancelled arm).
	if strings.Contains(line, landedCheckLimit.String()) {
		t.Errorf("the ◷ line quotes the %s bound over a run this launcher cancelled after %s:\n%s", landedCheckLimit, took, line)
	}
	if !strings.Contains(line, "cancelled") {
		t.Errorf("the ◷ line does not say the run was cancelled, so a reader cannot tell it from a judge that died on its own:\n%s", line)
	}
	// The positive control on "cancelled rather than finished": the hold file
	// is still there, so the body's own loop had not reached its exit when
	// the stop ended it — and nothing was recorded, which a judge that ran
	// to its `exit 0` would have been.
	if _, err := os.Stat(hold); err != nil {
		t.Errorf("the hold file is gone (%v), so the fixture's own witness is unreadable and this arm measured nothing", err)
	}
	if tree, v := landedRecorded(landedJudgeRule, repo); v != landUnanswered {
		t.Errorf("the cancelled run recorded %s %s — it answered nothing, so D4's record may hold nothing (ADR 0088 D6.5)", tree, v)
	}
	if n := lhQuiet(t, repo); n != 0 {
		t.Errorf("%s counted %d over a cancelled run, want 0 (ADR 0088 D6.5)", landedQuietKey, n)
	}
}

// And the pending set: a landing scheduled behind the stop is DROPPED and
// said out loud, rather than started into a judge the same stop would cancel
// a moment later. D6.6's arm-time sweep is what reads that tip instead.
func TestAStopDropsThePendingSetAndSaysSo(t *testing.T) {
	t.Parallel()
	c, _, first, log := ldiFixture(t, func(log string) string {
		return landedCheckBody("v1", log, filepath.Join(filepath.Dir(log), "hold"), 0)
	})
	// The slow writer, so "said before Wait returns" is an arm and not a
	// coin toss (ldiSlowOut's header).
	out := &ldiSlowOut{d: 50 * time.Millisecond}
	c.out = out
	write(t, filepath.Join(filepath.Dir(log), "hold"), "held\n")
	cancel := ldiStop(t, c)

	// A second repo, landed into while the first repo's judge is held open:
	// one entry in the pending set, which is the thing being dropped.
	second := wtRepo(t)
	secondLog := filepath.Join(t.TempDir(), "runs-2")
	shipLandedCheck(t, second, landedCheckBody("v2", secondLog, filepath.Join(t.TempDir(), "nohold"), 0), "ranger-base-soqa1: ship the landed check")

	c.Schedule(first, "main")
	landedWaitFor(t, "the first judge to start", func() bool { return len(landedRuns(t, log)) == 1 })
	c.Schedule(second, "main")
	cancel()
	c.Wait()

	if n := len(landedRuns(t, secondLog)); n != 0 {
		t.Errorf("the pending repo was judged %d time(s) by a launcher that is stopping — the set is dropped (ADR 0088 D6.5)", n)
	}
	s := out.String()
	if !strings.Contains(s, "was never started") {
		t.Errorf("the dropped landing was not reported at all, which is the shape this amendment answers:\n%s", s)
	}
	if !strings.Contains(s, AbbrevHome(second)) {
		t.Errorf("the dropped line does not name %s:\n%s", AbbrevHome(second), s)
	}
	if tree, v := landedRecorded(landedJudgeRule, second); v != landUnanswered {
		t.Errorf("the dropped repo recorded %s %s — nothing judged it", tree, v)
	}
}

// AND ADR 0097's CHECK GOES THROUGH THE SAME PATH (D6.5: "Both D3 rules go
// through the same checker, so this covers both"). An interrupted judge
// leaves the wall check NOT RUN, with the stop named as the reason — because
// "not run (judge interrupted)" and "not run" are different facts to
// whoever reads a launcher's log after a restart — and it counts nothing on
// either record.
//
// NOT PARALLEL, and the mechanism is planted: ADR 0097 D1's venue is a
// bwrap, so the line below is only printed on a host whose wall is one.
// wallMechanismForTest writes a package var (wall.go), which is also why
// this arm cannot be parallel.
func TestAnInterruptedJudgeLeavesTheWallCheckNotRunAndCountsNothing(t *testing.T) {
	c, out, repo, wallLog := wckFixture(t)
	wallMechanismForTest(t, "linux")
	if m := WallMechanism(); m != WallBwrap {
		t.Fatalf("WallMechanism() reads %q, so this arm cannot reach ADR 0097 D2's judged-verdict gate", m)
	}
	c.interruptGrace = ldiGrace
	judgeLog := filepath.Join(t.TempDir(), "judge-runs")
	shipLandedCheck(t, repo, ldiSignalBody(judgeLog), "ranger-base-soqa1: a judge that dies by a signal")
	cancel := ldiStop(t, c)

	c.Schedule(repo, "main")
	landedWaitFor(t, "the judge to start", func() bool { return len(landedRuns(t, judgeLog)) == 1 })
	cancel()
	c.Wait()

	s := out.String()
	if want := "wall check not run (launcher stopping)"; !strings.Contains(s, want) {
		t.Errorf("the pass does not say %q — a reader after a restart cannot tell this from a verdict about the tree (ADR 0088 D6.5):\n%s", want, s)
	}
	if runs := landedRuns(t, wallLog); len(runs) != 0 {
		t.Errorf("the wall check RAN behind an interrupted judge: %v — it runs after a GREEN judgement (ADR 0097 D2)", runs)
	}
	if tree, v := wckRecorded(t, repo); tree != "" || v != landUnanswered {
		t.Errorf("%s reads %q %s — a step that did not run records nothing", wallCheckKey, tree, v)
	}
	if n := wckQuiet(t, repo); n != 0 {
		t.Errorf("%s counted %d over a launcher that is stopping — the wall check was not the one that failed (ADR 0088 D6.5)", wallCheckQuietKey, n)
	}
	if n := lhQuiet(t, repo); n != 0 {
		t.Errorf("%s counted %d over an interrupted judge, want 0", landedQuietKey, n)
	}
	wckNone(t, c, repo, landedFileLabel, wallCheckRule.fileMarker(repo, "main"))
	wckNone(t, c, repo, landedHealthLabel, wallCheckRule.healthMarker(repo))
}

// classifyStop NARROWS AND NEVER WIDENS, which is the half of the rule no run
// can show: a verdict the judge gave before the stop reached it is still that
// verdict, and the one run that bought it is not thrown away.
func TestAStopNeverRewritesAnAnswerTheJudgeGave(t *testing.T) {
	t.Parallel()
	c := &landedChecker{pending: map[string]string{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.setStop(ctx)
	for _, v := range []landVerdict{landGreen, landRed} {
		for _, signalled := range []bool{false, true} {
			if got := c.classifyStop(v, signalled); got != v {
				t.Errorf("a %s run under a fired stop (signalled=%v) was reclassified as %s — only D3's silence is reconsidered (ADR 0088 D6.5)", v, signalled, got)
			}
		}
	}
	// And the nil case: no loop to stop is never "stopped", which is the
	// fail-closed direction and what keeps a one-shot `posse dispatch`
	// byte-identical to what it was.
	none := &landedChecker{pending: map[string]string{}}
	if got := none.classifyStop(landUnanswered, true); got != landUnanswered {
		t.Errorf("a signalled run with NO stop context read %s — nil means no loop to stop, never stopped (ADR 0088 D6.5)", got)
	}
}

// The seam defaults to the shipped grace, which is TheCeilingSeamDefaults...'s
// claim for the other seam: production reads the constant, and a test that
// forgets to set this one still measures D6.5's real number.
func TestTheInterruptGraceSeamDefaultsToTheShippedGrace(t *testing.T) {
	t.Parallel()
	c := &landedChecker{}
	if got := c.graceOf(); got != landedInterruptGrace {
		t.Errorf("graceOf() is %s over a zero seam, want the shipped %s", got, landedInterruptGrace)
	}
	c.interruptGrace = 7 * time.Millisecond
	if got := c.graceOf(); got != 7*time.Millisecond {
		t.Errorf("graceOf() is %s over a set seam, want 7ms", got)
	}
	// The grace is a race-closer and not a bound on anything, so it has to
	// sit far under the reaping grace a stop is promised to return within
	// (D6.5) — otherwise the classification itself is what holds the stop.
	if landedInterruptGrace > generatorKillGrace {
		t.Errorf("landedInterruptGrace is %s and generatorKillGrace is %s — a grace over the reaping grace makes the classification the slowest part of a stop (ADR 0088 D6.5)", landedInterruptGrace, generatorKillGrace)
	}
}

// ldiGateOut is ldiSlowOut with a door on it: it closes `entered` as the
// FIRST write BEGINS and only then holds that write open for `d`. The gate is
// what makes the arm below deterministic rather than a wide-window bet — a
// test that has received from `entered` knows the loop goroutine is inside
// c.say and therefore provably outside c.mu, which is the one instant the
// claim is about. `d` is the margin after that, not the mechanism.
//
// Mutex-guarded for ldiSlowOut's reason: a reader and the loop goroutine
// touch the buffer at the same instant, and a bare bytes.Buffer there is a
// data race the -race arm would report in place of a failure anyone can read.
type ldiGateOut struct {
	d       time.Duration
	entered chan struct{}
	once    sync.Once
	mu      sync.Mutex
	b       bytes.Buffer
}

func (w *ldiGateOut) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	time.Sleep(w.d)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *ldiGateOut) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// AND THE RE-READ OF THE PENDING SET IS THE OTHER HALF OF THAT FINISHING
// BLOCK (loop(), landedcheck.go). TestAStopDropsThePendingSetAndSaysSo above
// pins the ORDERING claim — a line this runner owes is written before Wait
// returns — and this arm pins the one the same block makes about the set
// itself: the dropped lines are said with c.mu RELEASED, so a landing can
// arrive while they print, and it finds `running` true and only adds itself.
// `Schedule`'s fast path is entitled to do that — "a run is in flight, it
// re-drains this map when it finishes" is its own comment — so a loop that
// exited here would orphan the landing: an entry in `pending` with no
// goroutine coming for it, no ◷ line, and nothing in the record or the quiet
// count to say a tree landed and was never looked at.
//
// FILED FROM ranger-base-rh8y0 (ranger-base-vcfc1 F4): the re-read is named
// load-bearing by its own comment and `if len(c.pending) > 0` → `if false`
// survived every Land*/Wall*/Judge* test in both arms — 264 of them — before
// this arm existed.
//
// THE STOP IS FIRED BEFORE ANYTHING IS SCHEDULED, which is what makes the
// window reachable at all: `dropped` is non-empty only when this launcher is
// stopping, so the stopping path is the one place the loop provably prints
// with the lock down. What the re-read buys behind a fired stop is the second
// landing's ◷ LINE and an empty set, not a judgement — D6.6's arm-time sweep
// still owns that tip, and the two assertions below are written to say so.
func TestAStopDropsALandingThatArrivesWhileItIsSayingSo(t *testing.T) {
	t.Parallel()
	c, _, first, firstLog := ldiFixture(t, func(log string) string {
		return landedCheckBody("v1", log, filepath.Join(t.TempDir(), "nohold"), 0)
	})
	out := &ldiGateOut{d: 250 * time.Millisecond, entered: make(chan struct{})}
	c.out = out
	cancel := ldiStop(t, c)
	cancel()

	// A second repo, landed into while the loop is mid-sentence about the
	// first — the landing that has nobody coming for it if the loop exits.
	second := wtRepo(t)
	secondLog := filepath.Join(t.TempDir(), "runs-2")
	shipLandedCheck(t, second, landedCheckBody("v2", secondLog, filepath.Join(t.TempDir(), "nohold"), 0), "ranger-base-rh8y0: ship the landed check")

	c.Schedule(first, "main")
	<-out.entered // the loop is inside the first ◷ line, so c.mu is down
	c.Schedule(second, "main")
	c.Wait()

	// THE INVARIANT ITSELF: a loop that has exited leaves nothing behind it.
	c.mu.Lock()
	orphans, running := len(c.pending), c.running
	c.mu.Unlock()
	if orphans != 0 || running {
		t.Errorf("the loop returned with %d landing(s) still pending (running=%v) — a landing that arrived while the drop lines printed was orphaned, and nothing is coming for it (ADR 0088 D6.5)", orphans, running)
	}
	// AND ITS OBSERVABLE HALF: the orphan's symptom is a tree that landed in
	// silence, so the ◷ line naming it is what a reader of the log has.
	s := out.String()
	if l := ldiLine(s, AbbrevHome(second)); !strings.Contains(l, ldiStopping) {
		t.Errorf("the landing that arrived mid-sentence got no %q line (its line: %q) — it landed and no reader can tell:\n%s", ldiStopping, l, s)
	}
	if l := ldiLine(s, AbbrevHome(first)); !strings.Contains(l, ldiStopping) {
		t.Errorf("the first dropped landing lost its own line (%q), so this arm is not measuring two rounds of the loop:\n%s", l, s)
	}
	// What the re-read does NOT do: judge it. Behind a fired stop both tips
	// are D6.6's to read, and an arm that let one through here would be
	// pinning the opposite of D6.5.
	if n := len(landedRuns(t, secondLog)); n != 0 {
		t.Errorf("the landing picked up by the re-read was JUDGED %d time(s) — the set is dropped behind a stop, not worked through (ADR 0088 D6.5)", n)
	}
	if n := len(landedRuns(t, firstLog)); n != 0 {
		t.Errorf("the first landing was judged %d time(s) by a launcher that was already stopping", n)
	}
	if tree, v := landedRecorded(landedJudgeRule, second); v != landUnanswered {
		t.Errorf("the re-read's landing recorded %s %s — nothing judged it", tree, v)
	}
}
