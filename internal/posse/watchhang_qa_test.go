package posse

// ranger-base-wj7e9: the pins for the clocks a `dispatch --watch` loop was
// missing — a deadline on its children, and a report on its own silence.
//
// FOUND 2026-09-03 12:04Z. Watch pid 8861 alive, its log's last line
// 04:53:18Z, one child: `herdr agent wait w1NN:p1 --until idle --until done
// --until blocked --timeout 900000`, etime 07:21:37. Read at the time as a
// 7h11m hang; it was not one. The laptop was ASLEEP for that window with the
// lid closed, and the bead was retracted on that ground.
//
// These pins survive the retraction because not one of them asserts that a
// hang happened. They assert that posse now bounds a child which does not
// return and says so — missing on 09-03 either way — and each drives a REAL
// process that really stops answering, so what is pinned is this package's
// behaviour and never the incident's cause.
//
// Four arms, one per fix:
//
//	the child deadline    a hung herdr child and a hung bd child are both
//	                      ended by posse, with a line naming the argv and
//	                      the wait, and the child is really gone.
//	the silence watchdog  a loop that has written nothing past its budget
//	                      says so, repeatedly, and a quiet-but-healthy one
//	                      does not.
//	the busy line         a lane-busy verdict names the SESSION and the
//	                      READING behind each busy seat, and a refill's
//	                      summary carries them once.
//	the backup sampler    the level is read far more often than it fires,
//	                      which is the defect the missed 05:21Z archive
//	                      actually was.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ─── the child deadline ──────────────────────────────────────────────────────

// hangingBin writes a binary that behaves exactly like the child this bead is
// about: it records its own pid, ignores every argument, and then stops
// answering. Nothing about it is a mock — it is a real process on a real
// pipe, which is the whole of what exec.Command could not get out of.
//
// It `exec`s the sleep rather than running it, so the whole child is ONE
// process: the pid it wrote is the pid that is killed, and no grandchild is
// left holding the pipes that os/exec's Wait is copying. (That case is real
// and is what herdrWaitDelay bounds — a shell that forks its sleep keeps
// Wait blocked for the full WaitDelay after the kill lands. It is bounded
// either way; this shape is what lets the pin assert the deadline itself
// rather than the deadline plus that backstop.)
//
// The sleep is BOUNDED so a kill that does not land costs this test a few
// seconds instead of leaking a process into the suite (AGENTS.md: an
// undeclared long-lived child is a leak, and the load guard ends it).
//
// It is WARMED before it is handed back, and that is not tidiness. macOS
// assesses a freshly written executable on its first exec, and MEASURED here
// 2026-09-03 that assessment costs more than 300ms and less than 2s: an
// unwarmed script under a 250ms deadline is killed before `sh` reaches its
// first line, so the test would be measuring Gatekeeper and not the
// deadline. The warm run pays that cost once, outside every timed window,
// and clears the pid file behind it.
func hangingBin(t *testing.T, sleep time.Duration) (bin, pidFile string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "hang")
	pidFile = filepath.Join(dir, "pid")
	script := fmt.Sprintf("#!/bin/sh\necho $$ > %q\n[ \"$1\" = --warm ] && exit 0\nexec sleep %.3f\n",
		pidFile, sleep.Seconds())
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(bin, "--warm").Run(); err != nil {
		t.Fatalf("the hanging child cannot be run at all (%v) — this test would measure nothing", err)
	}
	if err := os.Remove(pidFile); err != nil {
		t.Fatal(err)
	}
	return bin, pidFile
}

// awaitPid reads the pid the hanging child wrote, waiting for it to appear.
func awaitPid(t *testing.T, pidFile string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(pidFile); err == nil {
			var pid int
			if _, err := fmt.Sscan(strings.TrimSpace(string(b)), &pid); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the child never wrote its pid to %s — it did not run", pidFile)
	return 0
}

// assertReaped is the half of the pin that a returned error cannot give. A
// caller that gave up on a child and left it running would satisfy every
// assertion about the error and none about the box: the 7h11m child was
// still burning a slot when the operator found it. The child's sleep is
// still running at this point unless something ended it.
func assertReaped(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pid %d is still alive after the deadline blew — nothing killed it", pid)
}

// The herdr arm, in the shape of the incident: `agent wait` with its own
// --timeout, a child that never answers, and a caller that must not wait for
// it.
func TestQAHungHerdrChildIsKilledAndNamed(t *testing.T) {
	t.Parallel()
	const sleep = 6 * time.Second
	bin, pidFile := hangingBin(t, sleep)
	var log strings.Builder
	// The two test-only fields: the production numbers are a ceiling on a
	// hang, not a latency budget, and a test that waited them out would BE
	// the hang. WaitGrace is what `--timeout` is topped up by, so this call
	// is bounded at 100ms + 150ms.
	h := Herdr{Bin: bin, WaitGrace: 250 * time.Millisecond, Hangw: &log}

	started := time.Now()
	_, err := h.AgentWait("w1:p1", []string{"idle", "done", "blocked"}, 100)
	waited := time.Since(started)
	pid := awaitPid(t, pidFile)

	if !IsHerdrHang(err) {
		t.Fatalf("a child that never answered returned %v, want a HerdrHangError", err)
	}
	if waited > sleep/2 {
		t.Fatalf("the call waited %s of the child's %s sleep — the deadline did not bound it", waited, sleep)
	}
	// The named finding: the call, its argv, and how long it hung. Every one
	// of these is a thing the 09-03 log did not have.
	for _, want := range []string{"agent wait", "w1:p1", "--until idle", "--timeout 100", "hung"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the hang error does not name %q:\n%s", want, err.Error())
		}
	}
	if !strings.Contains(log.String(), "agent wait w1:p1") {
		t.Errorf("nothing was written where the hang happened; Hangw got:\n%s", log.String())
	}
	assertReaped(t, pid)
}

// A call that declares NO timeout of its own is bounded too — by
// HerdrControlTimeout, which is what every `workspace`/`pane`/`agent list`
// read in a pass runs under. Without this arm the fix would cover the one
// call that happened to hang and nothing else.
func TestQAHungHerdrControlCallIsKilledAndNamed(t *testing.T) {
	t.Parallel()
	const sleep = 6 * time.Second
	bin, pidFile := hangingBin(t, sleep)
	var log strings.Builder
	h := Herdr{Bin: bin, ControlTimeout: 250 * time.Millisecond, Hangw: &log}

	started := time.Now()
	_, err := h.Workspaces()
	waited := time.Since(started)
	pid := awaitPid(t, pidFile)

	if !IsHerdrHang(err) {
		t.Fatalf("a hung `workspace list` returned %v, want a HerdrHangError", err)
	}
	if waited > sleep/2 {
		t.Fatalf("the call waited %s of the child's %s sleep — the control ceiling did not bound it", waited, sleep)
	}
	if !strings.Contains(err.Error(), "workspace list") {
		t.Errorf("the hang error does not name the call:\n%s", err.Error())
	}
	assertReaped(t, pid)
}

// The bd arm. No bd hang has been observed here; this is the same missing
// clock on the other child a pass depends on, and the bead asked for it by
// name ("and any other child").
func TestQAHungBdChildIsSignalledAndNamed(t *testing.T) {
	t.Parallel()
	const sleep = 6 * time.Second
	bin, pidFile := hangingBin(t, sleep)
	var log strings.Builder
	b := Bd{Bin: bin, Timeout: 250 * time.Millisecond, Hangw: &log}

	started := time.Now()
	_, err := b.runOnce("", "ready", "--json")
	waited := time.Since(started)
	pid := awaitPid(t, pidFile)

	if !IsBdHang(err) {
		t.Fatalf("a hung bd returned %v, want a BdHangError", err)
	}
	if waited > sleep/2 {
		t.Fatalf("the call waited %s of the child's %s sleep — BdTimeout did not bound it", waited, sleep)
	}
	// The global flags ride in the argv the error names: an operator who
	// retypes the verb without them gets a different bd (beads.go).
	for _, want := range []string{"ready", "--json", "--no-daemon", "hung"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the bd hang error does not name %q:\n%s", want, err.Error())
		}
	}
	if !strings.Contains(log.String(), "bd child hung") {
		t.Errorf("nothing was written where the bd hang happened; Hangw got:\n%s", log.String())
	}
	assertReaped(t, pid)
}

// The deadline is sized BY THE CALL, and this is the whole of that rule: a
// wait call gets what it declared plus the grace, a control call gets the
// ceiling. Sized wrong in either direction the fix is worse than nothing —
// a control ceiling over `agent wait --timeout 900000` would cut a healthy
// fifteen-minute wait off at two minutes and unclaim live work.
func TestQAHerdrCallDeadlineIsSizedByTheCall(t *testing.T) {
	t.Parallel()
	h := Herdr{Bin: "herdr"}
	if got, want := h.callDeadline([]string{"workspace", "list"}), HerdrControlTimeout; got != want {
		t.Errorf("a control call got %s, want %s", got, want)
	}
	// The incident's own argv.
	incident := []string{"agent", "wait", "w1:p1", "--until", "idle", "--until", "done", "--until", "blocked", "--timeout", "900000"}
	if got, want := h.callDeadline(incident), 15*time.Minute+HerdrWaitGrace; got != want {
		t.Errorf("the 2026-09-03 call got %s, want its own 15m plus %s of grace", got, HerdrWaitGrace)
	}
	if got, want := h.callDeadline([]string{"agent", "wait", "w1:p1", "--timeout=30000"}), 30*time.Second+HerdrWaitGrace; got != want {
		t.Errorf("the joined --timeout= spelling got %s, want %s", got, want)
	}
	// An unreadable number is treated as no declaration at all: the control
	// ceiling ends the call and says so, where trusting it would be trusting
	// nothing.
	if got, want := h.callDeadline([]string{"agent", "wait", "w1:p1", "--timeout", "forever"}), HerdrControlTimeout; got != want {
		t.Errorf("an unparseable --timeout got %s, want the control ceiling %s", got, want)
	}
}

// ─── the silence watchdog ────────────────────────────────────────────────────

// The budget is built from the longest quiet a HEALTHY loop can have, and
// this pins both terms against the production numbers. A budget under one
// wait leg would name every ordinary fifteen-minute leg a stall.
func TestQAWatchdogBudgetClearsAHealthyWaitLeg(t *testing.T) {
	t.Parallel()
	const promptWaitMS = 15 * 60 * 1000 // dispatch.go's production default
	leg := time.Duration(promptWaitMS)*time.Millisecond + HerdrWaitGrace

	// The live loop's shape: `--watch 3m --max-interval 3m`.
	got := watchdogBudget(3*time.Minute, promptWaitMS)
	if got <= leg {
		t.Fatalf("budget %s does not clear one wait leg (%s) — every healthy leg would trip it", got, leg)
	}
	if got != WatchdogFactor*leg {
		t.Errorf("budget %s, want %d x the longest legitimate quiet (%s)", got, WatchdogFactor, leg)
	}
	// A loop backed off further than a leg is bounded by the interval
	// instead: the larger of the two terms wins, never the base.
	if got := watchdogBudget(2*time.Hour, promptWaitMS); got != WatchdogFactor*2*time.Hour {
		t.Errorf("a 2h max-interval got %s, want the interval to be the term that wins", got)
	}
}

// The reading itself: quiet inside the budget says nothing, quiet past it
// says so — and says so AGAIN, because the loop is still stalled and a line
// printed once hours ago in a scrollback nobody is watching is the silence
// this bead was about.
func TestQAWatchdogNamesASilentLoopAndKeepsSaying(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	at := time.Date(2026, 9, 3, 4, 53, 18, 0, time.UTC)
	d.Now = func() time.Time { return at }

	d.printf("── pass 3 · 00:53:18\n")
	budget := 32 * time.Minute

	// Inside the budget: a rolling pass mid-leg is not a stall.
	at = at.Add(20 * time.Minute)
	d.watchdogTick(budget)
	if strings.Contains(dispatcherOut(d), "watchdog") {
		t.Fatalf("a 20m quiet inside a 32m budget was reported:\n%s", dispatcherOut(d))
	}

	// Past it: the incident's own silence, named.
	at = at.Add(20 * time.Minute)
	d.watchdogTick(budget)
	first := dispatcherOut(d)
	if !strings.Contains(first, "watchdog") || !strings.Contains(first, "40m") {
		t.Fatalf("a 40m silence past a 32m budget was not named:\n%s", first)
	}
	if !strings.Contains(first, "04:53:18") {
		t.Errorf("the watchdog line does not say when the last line was:\n%s", first)
	}

	// And again, with the number grown. This is what fails if the watchdog
	// ever writes through printf: its own line would reset LastWrite and the
	// stall would be reported exactly once.
	at = at.Add(1 * time.Hour)
	d.watchdogTick(budget)
	if !strings.Contains(dispatcherOut(d), "1h40m") {
		t.Fatalf("the second reading did not reprint with the silence grown:\n%s", dispatcherOut(d))
	}

	// A line from the loop clears it: the watchdog measures silence, not
	// elapsed time.
	d.printf("── pass 4 · 06:33:18\n")
	before := len(dispatcherOut(d))
	d.watchdogTick(budget)
	if len(dispatcherOut(d)) != before {
		t.Errorf("the watchdog fired after the loop wrote again:\n%s", dispatcherOut(d)[before:])
	}
}

// ─── the busy line (ask 3) ───────────────────────────────────────────────────

// The line that started the second half of this bead: a code lane reported
// busy by three names and nothing else, while all three of those sessions
// had been reaped — no session named, and no way to tell a bead this Run is
// still holding from a live herdr reading.
//
// The fix does NOT widen this line. ADR 0020 §2 specifies its shape by
// example ("code lane busy: <a>, <b>") and so does the seat clause beside it
// ("<b>; <a> busy"); widening either is an amendment for the architecture
// lane to make, not an implementation detail. So this pin guards the ADR's shape against the
// reading now carried alongside it, and the refill summary below is where
// that reading is actually rendered.
func TestQALaneBusyKeepsTheADRShapeWhileCarryingTheReading(t *testing.T) {
	t.Parallel()
	lane := routeLane{label: "code", seats: []routeMatch{{name: "developer"}, {name: "developer-2"}, {name: "developer-3"}}}
	passed := []seatPass{
		{name: "developer", where: "a-1", doing: "busy"},
		{name: "developer-2", where: "developer-2-003-a-2", doing: "working"},
		{name: "developer-3", where: "", doing: "busy"},
	}
	got := laneBusyLine(lane, passed, "/repo/posse")

	// ADR 0020 §2, verbatim shape: the lane, then bare seat names in routing
	// order. The two pins in seatselect_m9m9_qa_test.go quote it too; this
	// one fails in the same breath if `where` ever leaks onto the line.
	if want := "code lane busy: developer, developer-2, developer-3 — waits for a later pass"; !strings.Contains(got, want) {
		t.Errorf("the lane-busy line no longer has ADR 0020 §2's shape:\n got %s\nwant %s", got, want)
	}
	// "busy:" is not a leak marker — "code lane busy:" is the line's own
	// prefix. The clause renders its reading inside parentheses, so "(" plus
	// the two values is what actually separates the shapes.
	for _, leak := range []string{"a-1", "developer-2-003-a-2", "("} {
		if strings.Contains(got, leak) {
			t.Errorf("the reading leaked onto the ADR-specified line (%q):\n%s", leak, got)
		}
	}

	// And the reading itself is carried, or the refill summary below would
	// have nothing to render: a run-hold names its bead, a live read names
	// the per-bead SESSION, and a seat benched earlier in this pass names
	// neither. Rendering those three identically is what made the 04:53Z
	// line unactionable.
	if c := passed[0].clause(); c != "developer (busy: a-1)" {
		t.Errorf("a seat held by this Run's own bead does not carry the bead: %s", c)
	}
	if c := passed[1].clause(); c != "developer-2 (working: developer-2-003-a-2)" {
		t.Errorf("a seat read live off herdr does not carry the session: %s", c)
	}
	if c := passed[2].clause(); c != "developer-3 (busy)" {
		t.Errorf("a seat benched by an earlier reading in the same pass is not distinguished: %s", c)
	}
}

// Inside a refill the per-bead lines are counted, not printed, so `123 lane
// busy` was the whole of what the log carried. The seats are few and the
// beads are many: name them once, on the summary.
func TestQARefillSummaryNamesTheBusySeats(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)

	d.beginRefill("developer-3-003", "a-9", 133)
	for i := 0; i < 124; i++ {
		// What seatFor does on every lane-busy verdict: the same seats,
		// once per skipped bead.
		d.refilling.noteBusy([]seatPass{
			{name: "developer", where: "a-1", doing: "busy"},
			{name: "developer-2", where: "developer-2-003-a-2", doing: "working"},
		})
		d.skipf(skipLaneBusy, "– unused\n")
	}
	d.endRefill(0)

	got := dispatcherOut(d)
	if !strings.Contains(got, "124 lane busy") {
		t.Fatalf("the refill summary lost its count:\n%s", got)
	}
	if !strings.Contains(got, "busy: developer (busy: a-1), developer-2 (working: developer-2-003-a-2)") {
		t.Errorf("the refill summary does not name the busy seats once:\n%s", got)
	}
	// Once, not 124 times: the whole reason this is on the summary.
	if n := strings.Count(got, "developer-2-003-a-2"); n != 1 {
		t.Errorf("the busy seats are named %d times, want once per refill:\n%s", n, got)
	}
}

// ─── the backup sampler ──────────────────────────────────────────────────────

// The arithmetic of the missed 05:21Z archive, from the box's own records.
// It was reported on this bead as a symptom of a 7h11m hang. There was no
// hang — that window was a sleep — and it is not a symptom of the sleep
// either: no backup tick fell inside 04:53Z-12:05Z at all. The level was
// simply never read, and would not have been on a box that never slept.
func TestQABackupLevelIsSampledFasterThanItFires(t *testing.T) {
	t.Parallel()
	// state/dispatch-watch.pid, state/backup/ and config.yaml, 2026-09-03.
	loopStart := time.Date(2026, 9, 3, 1, 53, 52, 0, time.UTC)
	newest := time.Date(2026, 9, 2, 5, 21, 14, 0, time.UTC)
	const interval = 24 * time.Hour

	// The start evaluation was right to decline: 20h32m is under 24h.
	if age := loopStart.Sub(newest); age >= interval {
		t.Fatalf("the archive was %s old at the loop's start — the start evaluation would have written one", age)
	}
	// Under a ticker at the interval the next look is a whole day later, by
	// which time the archive is nearly two intervals old. That is the bug.
	if stale := loopStart.Add(interval).Sub(newest); stale < 44*time.Hour {
		t.Fatalf("a look one interval later finds a %s-old archive; the premise of this pin is wrong", stale)
	}
	// Sampled instead: the level is read within BackupSampleMax of coming
	// up, so the worst case is interval + 15m rather than 2 x interval.
	every := backupSampleEvery(interval)
	if every != BackupSampleMax {
		t.Errorf("a 24h interval samples every %s, want the %s cap", every, BackupSampleMax)
	}
	due := newest.Add(interval) // 2026-09-03T05:21:14Z
	if late := every; late > 15*time.Minute {
		t.Errorf("the archive due at %s would be up to %s late, want no more than 15m", due.Format(time.RFC3339), late)
	}
	// A short interval stays proportional, and a sub-second one behaves
	// exactly as it did before this bead — the fast rigs in
	// backuploop_test.go depend on that.
	if got, want := backupSampleEvery(8*time.Minute), time.Minute; got != want {
		t.Errorf("an 8m interval samples every %s, want %s", got, want)
	}
	if got, want := backupSampleEvery(50*time.Millisecond), 50*time.Millisecond; got != want {
		t.Errorf("a 50ms interval samples every %s, want the interval itself", got)
	}
}

// The sampler, running. Two arms, because a sampler that wrote on every tick
// regardless of the level would pass the first one alone.
func TestQABackupLoopSamplesTheLevelBetweenIntervals(t *testing.T) {
	d, a, at := backupLoopRig(t, "60s")
	cfg, err := LoadBackupConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	// The cold start writes one and sets the level.
	runBackupLoop(t, d, a, cfg, 1, 60*time.Second)
	if got := archives(t, a); len(got) != 1 {
		t.Fatalf("the first loop wrote %v, want one archive", got)
	}

	// TREATMENT: the level comes up 70 seconds in. The sampler reads it at
	// 7.5s; a ticker at cfg.Interval would not look until 60s, so a grace
	// well under a minute is what separates the two.
	*at = at.Add(70 * time.Second)
	took := runBackupLoop(t, d, a, cfg, 2, 40*time.Second)
	if got := archives(t, a); len(got) != 2 {
		t.Fatalf("the level was up and %s of sampling wrote %v, want two archives", took, got)
	}
	if took >= cfg.Interval {
		t.Fatalf("the second archive took %s, which a ticker at the %s interval would also have managed — this measures nothing", took, cfg.Interval)
	}

	// ABSENCE: the clock does not move, so the level is down. A sampler that
	// fired on the tick rather than on the level would write a third here,
	// and it is given longer than the treatment arm needed.
	quiet := runBackupLoop(t, d, a, cfg, 3, 3*took+10*time.Second)
	if got := archives(t, a); len(got) != 2 {
		t.Fatalf("%s of sampling under a down level wrote %v, want the same two — the sampler is firing on its tick", quiet, got)
	}
}

// ─── the watchdog under the loop ─────────────────────────────────────────────

// The wiring, not the reading: Watch starts the watchdog and JOINS it, on the
// same rule the pulse, the backup clock and the guard clock keep. An
// unjoined tick writes into a Dispatcher whose caller believes the loop is
// over (ranger-base-el3g).
func TestQAWatchStartsAndJoinsTheWatchdog(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// An empty queue of its own: without a `beads:` the App falls back to
	// the process cwd, which in this suite is the repo under test.
	write(t, b.App.ConfigPath, "beads:\n  - "+t.TempDir()+"\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the loop ends before its first pass
	if _, err := d.Watch(ctx, "", "", 1, 20*time.Millisecond, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	// Watch returning means every clock it started has been joined. If the
	// watchdog were left running, this write would race it — which is what
	// the race detector is for, and what the join makes impossible.
	d.printf("after the loop\n")
	if !strings.Contains(dispatcherOut(d), "after the loop") {
		t.Fatal("the dispatcher is unusable after Watch returned")
	}
}
