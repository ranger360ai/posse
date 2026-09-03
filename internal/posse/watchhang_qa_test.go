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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	d, a, _ := backupLoopRig(t, "16s")
	// The rig's clock is a bare variable; this pin moves it WHILE the loop
	// runs, so the reads and the write need a lock of their own.
	var mu sync.Mutex
	at := time.Date(2026, 9, 1, 3, 15, 0, 0, time.UTC)
	d.Now = func() time.Time { mu.Lock(); defer mu.Unlock(); return at }
	advance := func(by time.Duration) { mu.Lock(); at = at.Add(by); mu.Unlock() }
	cfg, err := LoadBackupConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	// The cold start writes one and sets the level.
	runBackupLoop(t, d, a, cfg, 1, 16*time.Second)
	if got := archives(t, a); len(got) != 1 {
		t.Fatalf("the first loop wrote %v, want one archive", got)
	}

	// TREATMENT: the loop starts with the level DOWN, and it comes up a
	// second later, while the loop is running. That ordering is the whole
	// pin: backupLoop evaluates the level once at start (the restart rule),
	// so a level already up when the loop starts is written by that
	// evaluation and proves nothing about the ticker — a ticker at the full
	// interval passed the earlier form of this arm (ranger-base-iyg9w,
	// mutation-checked). Sampling at interval/8 = 2s reads the level within
	// a few seconds of it coming up; a ticker at the 16s interval would not
	// look inside the 10s grace at all.
	raise := time.AfterFunc(time.Second, func() { advance(20 * time.Second) })
	defer raise.Stop()
	took := runBackupLoop(t, d, a, cfg, 2, 10*time.Second)
	if got := archives(t, a); len(got) != 2 {
		t.Fatalf("the level came up 1s in and %s of sampling wrote %v, want two archives — the level is read once per interval, not sampled", took, got)
	}
	if took < time.Second {
		t.Fatalf("the second archive took %s, before the level was raised — the start evaluation wrote it and this arm measured nothing", took)
	}
	if took >= cfg.Interval {
		t.Fatalf("the second archive took %s, which a ticker at the %s interval would also have managed — this measures nothing", took, cfg.Interval)
	}

	// ABSENCE: the clock does not move, so the level is down. A sampler that
	// fired on the tick rather than on the level would write a third here,
	// and it is given longer than the treatment arm needed.
	quiet := runBackupLoop(t, d, a, cfg, 3, 3*took+6*time.Second)
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

// ─── the kill's second half, and the signal's first (ranger-base-iyg9w) ─────

// hangingBinWith is hangingBin with a body of the pin's choosing after the
// pid line and the warm exit. The shapes below need a grandchild, or a
// trap, and the exec'd single-process shape above deliberately has neither.
func hangingBinWith(t *testing.T, body string) (bin, pidFile string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "hang")
	pidFile = filepath.Join(dir, "pid")
	script := fmt.Sprintf("#!/bin/sh\necho $$ > %q\n[ \"$1\" = --warm ] && exit 0\n%s\n", pidFile, body)
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

// The grandchild on the pipe. herdr.go names this shape as the reason for
// herdrWaitDelay: os/exec's Wait does not return until the stdout/stderr
// copies are done, and a grandchild that inherited those pipes keeps them
// open after the child is killed. Dropping WaitDelay survived every pin
// above (ranger-base-iyg9w, mutation-checked), because hangingBin execs its
// sleep on purpose. This child forks one instead.
func TestQAHungHerdrChildWithAGrandchildOnThePipeStillReturns(t *testing.T) {
	t.Parallel()
	const sleep = 12 * time.Second
	body := fmt.Sprintf("sleep %.0f &\nexec sleep %.0f", sleep.Seconds(), sleep.Seconds())
	bin, pidFile := hangingBinWith(t, body)
	var log strings.Builder
	h := Herdr{Bin: bin, ControlTimeout: 250 * time.Millisecond, Hangw: &log}

	started := time.Now()
	_, err := h.Workspaces()
	waited := time.Since(started)
	pid := awaitPid(t, pidFile)

	if !IsHerdrHang(err) {
		t.Fatalf("a hung child with a grandchild on its pipes returned %v, want a HerdrHangError", err)
	}
	// The deadline plus WaitDelay, with room for a loaded box; well short of
	// the grandchild's own sleep, which is when Wait returns without it.
	if bound := herdrWaitDelay + 4*time.Second; waited > bound {
		t.Fatalf("the call waited %s — past the %s deadline and the %s WaitDelay, so the grandchild's pipe held Wait until the grandchild finished",
			waited, h.ControlTimeout, herdrWaitDelay)
	}
	assertReaped(t, pid)
}

// The bd child is TERMed first, and that is a claim beads.go makes for a
// reason — bd writes SQLite, and a TERM is what lets it roll the WAL back —
// which the pin above cannot see: a KILL reaps the child just as well.
// Swapping the TERM for CommandContext's default KILL survived it
// (ranger-base-iyg9w, mutation-checked). This child traps TERM and says so.
func TestQAHungBdChildIsTERMedBeforeItIsKilled(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "term")
	// The sleep is backgrounded with its pipes closed so nothing but the
	// trap decides when the call returns; `wait` is what a TERM interrupts.
	body := fmt.Sprintf("trap 'echo got > %q; exit 0' TERM\nsleep 12 >/dev/null 2>&1 &\nwait", marker)
	bin, pidFile := hangingBinWith(t, body)
	var log strings.Builder
	b := Bd{Bin: bin, Timeout: 250 * time.Millisecond, Hangw: &log}

	started := time.Now()
	_, err := b.runOnce("", "ready", "--json")
	waited := time.Since(started)
	pid := awaitPid(t, pidFile)

	if !IsBdHang(err) {
		t.Fatalf("a hung bd returned %v, want a BdHangError", err)
	}
	if waited > bdKillGrace {
		t.Fatalf("the call waited %s — past bdKillGrace, so the TERM was not delivered and the KILL is what ended it", waited)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatalf("the child's TERM trap never ran (%v) — bd was killed outright, and a SQLite writer killed outright is a WAL left open", statErr)
	}
	assertReaped(t, pid)
}

// ─── the busy seats, THROUGH seatFor (ranger-base-iyg9w) ────────────────────

// The two busy-line pins above build their seatPass values by hand, so the
// wiring between the seat walk and the refill summary was pinned by
// nothing: dropping seatFor's noteBusy call, and dropping the bead from
// seatMap.why, each survived every pin in this file (mutation-checked).
// This one goes in at the front door — a lane whose seats this Run holds,
// inside a refill — and reads the summary.
func TestQASeatForCarriesTheBusySeatsToTheRefillSummary(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	const dir = "/repo/posse"
	lane := routeLane{label: "code", seats: []routeMatch{{name: "developer"}, {name: "developer-2"}}}
	// Both seats held by this Run: seatMap.why answers before any herdr
	// read, so the summary must name the bead each holds.
	seats := newSeatMap(map[string]string{
		SessionFor("developer", dir):   "a-1",
		SessionFor("developer-2", dir): "a-2",
	})

	d.beginRefill("developer-3-003", "a-9", 3)
	idx, _, line := d.seatFor(lane, RepoIssue{Dir: dir}, "", seats)
	if idx != -1 {
		t.Fatalf("a lane whose every seat this Run holds seated the bead at index %d: %q", idx, line)
	}
	if want := "code lane busy: developer, developer-2"; !strings.Contains(line, want) {
		t.Fatalf("the lane-busy line lost ADR 0020 §2's shape:\n got %s\nwant %s", line, want)
	}
	d.skipf(skipLaneBusy, "%s\n", line)
	d.endRefill(0)

	got := dispatcherOut(d)
	if want := "busy: developer (busy: a-1), developer-2 (busy: a-2)"; !strings.Contains(got, want) {
		t.Fatalf("the refill summary does not carry the seats seatFor stepped over, with the bead each holds:\nwant %s\n got:\n%s", want, got)
	}
}

// ─── the watchdog, STARTED by Watch (ranger-base-iyg9w) ─────────────────────

// TestQAWatchStartsAndJoinsTheWatchdog above pins the join; a Watch that
// never started the watchdog at all passed it (mutation-checked). This one
// needs the line: a pass that stalls in a bd child that never answers,
// under a clock that runs fast enough for the budget to expire inside that
// stall, and the watchdog's own line in the output. The clock lies only
// about rate — every reader in the loop sees the same monotonic time, so
// the loop is not confused, merely hurried.
func TestQAWatchStartsTheWatchdogAndItNamesAStalledPass(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	write(t, b.App.ConfigPath, "beads:\n  - "+t.TempDir()+"\n")
	// The stall: bd stops answering, for 2s of real time under its own
	// deadline. That is the child the deadline above bounds, and the loop
	// writes nothing while it waits on it.
	bin, _ := hangingBin(t, 6*time.Second)
	d.Bd = Bd{Bin: bin, Timeout: 2 * time.Second, Hangw: io.Discard}
	// 10000x: a 32m production budget is ~200ms of real time, inside the
	// 2s stall, and the watchdog ticks at the 20ms base.
	epoch := time.Now()
	t0 := time.Date(2026, 9, 3, 4, 53, 18, 0, time.UTC)
	d.Now = func() time.Time { return t0.Add(time.Since(epoch) * 10000) }

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := d.Watch(ctx, "", "", 1, 20*time.Millisecond, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	got := dispatcherOut(d)
	if !strings.Contains(got, "watchdog: this loop has written nothing for") {
		t.Fatalf("a pass stalled in a hung bd child past the budget was never named — Watch did not start the watchdog:\n%s", got)
	}
}

// ─── the clocks are not the loop (ranger-base-0fz98 finding 3) ──────────────

// The watchdog's only input is LastWrite, and the guard clock's tick wrote
// through printf on its own goroutine every base interval for as long as
// the box was over the load line — so under exactly the condition that
// makes a stall likely, a stalled pass was never named: the reading was
// refreshed every 3m and the 32m budget was unreachable. The backup clock
// wrote the same way. Both are readings of the shop, not signs of life
// from the loop, and this pin is the sentence watchdog.go's head already
// carried made measurable: a line from a clock inside the budget does NOT
// clear the reading, and the next watchdog tick reports with the silence
// grown.
//
// The shop pulse is the third clock (pulse.go, ranger-base-frqmn). Its
// tick was quiet by accident — bare fmt.Fprintf, no stamp — which was the
// right reading arrived at by omission, and one refactor to d.printf away
// from re-opening this. Now it is quiet by the same pair the other two use,
// and this arm is what keeps it so.
//
// Rig: the named stall from TestQAWatchdogNamesASilentLoopAndKeepsSaying,
// then one tick of the clock under test, then the watchdog again. The
// clock's line must be in the log (a quiet writer that also dropped the
// line would pass a LastWrite check and lose the reading) and LastWrite
// must still say the pass's own last line.
func TestQAClockLinesDoNotFeedTheWatchdog(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		build func(t *testing.T) (*Dispatcher, *time.Time)
		speak func(d *Dispatcher) // one tick of the clock under test
		says  string              // what the clock's own line carries
	}{
		{"guard clock over the line", func(t *testing.T) (*Dispatcher, *time.Time) {
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.App.Load1 = func() (float64, error) { return 85.5, nil }
			d.App.TopCPU = func() ([]Proc, error) { return nil, nil }
			at := time.Date(2026, 9, 3, 4, 53, 18, 0, time.UTC)
			d.Now = func() time.Time { return at }
			return d, &at
		}, func(d *Dispatcher) { d.guardTick() }, "guard clock"},
		{"backup clock", func(t *testing.T) (*Dispatcher, *time.Time) {
			d, _, at := backupLoopRig(t, "50ms")
			return d, at
		}, func(d *Dispatcher) {
			cfg, err := LoadBackupConfig(d.App)
			if err != nil {
				t.Fatal(err)
			}
			d.backupTick(cfg)
		}, "backup · scheduled"},
		{"shop pulse", func(t *testing.T) (*Dispatcher, *time.Time) {
			// The own-line rig (TestQAPulseTickLogsTheShopPulseOnItsOwnLine):
			// an unpushed repo makes the condition set non-empty, so the
			// tick prints — an empty set writes nothing and would measure
			// nothing here — and an idle coordinator makes it deliver, so
			// the prompted line goes through the same writer.
			b, fake := newTestBackend(t)
			personaSession(t, b, fake, "coordinator-work", "coordinator", "idle", false)
			unpushedRepo(t, b)
			at := time.Date(2026, 9, 3, 4, 53, 18, 0, time.UTC)
			d := deliveryDispatcher(t, b, &at)
			return d, &at
		}, func(d *Dispatcher) {
			d.pulseOnce(PulseConfig{Armed: true, Persona: "coordinator", Renag: 30 * time.Minute, RenagMax: 4 * time.Hour})
		}, "pulse: shop "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, at := tc.build(t)
			budget := 32 * time.Minute

			d.printf("── pass 3 · %s\n", at.Format("15:04:05"))
			passWrote := d.LastWrite()

			*at = at.Add(40 * time.Minute)
			d.watchdogTick(budget)
			if !strings.Contains(dispatcherOut(d), "watchdog") {
				t.Fatalf("a 40m silence past a 32m budget was not named:\n%s", dispatcherOut(d))
			}

			// The clock speaks, inside the budget of the NEXT reading.
			*at = at.Add(3 * time.Minute)
			before := len(dispatcherOut(d))
			tc.speak(d)
			if !strings.Contains(dispatcherOut(d)[before:], tc.says) {
				t.Fatalf("the clock wrote no %q line, so this measures nothing:\n%s", tc.says, dispatcherOut(d)[before:])
			}
			if got := d.LastWrite(); !got.Equal(passWrote) {
				t.Errorf("the clock's line moved LastWrite from %s to %s: a clock is not the loop writing (ranger-base-0fz98)",
					passWrote.Format("15:04:05"), got.Format("15:04:05"))
			}

			// And the watchdog still counts from the pass's last line.
			*at = at.Add(1 * time.Hour)
			before = len(dispatcherOut(d))
			d.watchdogTick(budget)
			if got := dispatcherOut(d)[before:]; !strings.Contains(got, "1h43m") {
				t.Errorf("after the clock's line the watchdog did not reprint with the silence grown to 1h43m:\n%q", got)
			}
		})
	}
}

// The other half of ranger-base-frqmn, which the arm above cannot see: a
// pulse line written with fmt.Fprintf(d.Out, ...) takes no outMu, and a
// gather writes the same stream from one goroutine per pending bead (ADR
// 0028 §1), so a pulse line could land half-way through a launch line. A
// bare write is also quiet, so the LastWrite arm above stays green over it.
// This is the source pin for the writer: every write pulse.go makes to Out
// or errw() goes through quietf/equietf — never the fmt functions and never
// the stamping three, which would feed the watchdog. It counts what it
// found, so a sweep over the wrong file fails instead of passing.
func TestQAPulseWritesGoThroughTheQuietPair(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("pulse.go")
	if err != nil {
		t.Fatal(err)
	}
	quiet := strings.Count(string(src), "d.quietf(") + strings.Count(string(src), "d.equietf(")
	if quiet < 5 {
		t.Fatalf("found %d quiet writes in pulse.go; the sweep is not reading the file it thinks it is", quiet)
	}
	for i, ln := range strings.Split(string(src), "\n") {
		for _, bad := range []string{
			"Fprintf(d.Out", "Fprintln(d.Out", "Fprint(d.Out",
			"Fprintf(d.errw()", "Fprintln(d.errw()", "Fprint(d.errw()",
			"d.printf(", "d.eprintf(", "d.println(",
		} {
			if strings.Contains(ln, bad) {
				t.Errorf("pulse.go:%d writes the watch stream with %s — outside outMu, or stamping LastWrite from a clock (ranger-base-frqmn):\n%s", i+1, bad, ln)
			}
		}
	}
	t.Logf("checked %d quiet writes in pulse.go", quiet)
}
