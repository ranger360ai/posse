package posse

// The load guard's kill arm, arm 2 of ranger-base-apwr (ranger-base-gvp2p).
// Every pin here drives the selection and the render through App.ReapOrphans
// — a fake — for the reason the arm-1 pins drive the census through TopCPU:
// the one thing this feature must never do in a unit test is signal a
// process on the machine the suite is running on. The REAL reaper is
// measured by the planted control (orphancontrol_test.go), which runs only
// in the throwaway container scripts/verify-orphan-report.sh starts.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// armed writes the config key that turns the kill arm on, and returns the
// App with a fake reaper that records what it was offered.
func killArmApp(t *testing.T, on bool) (*App, *[]int, map[int]string) {
	t.Helper()
	a := NewAppAt(t.TempDir())
	if on {
		if err := os.WriteFile(a.ConfigPath, []byte("load_guard_kill: true\n"), 0o644); err != nil {
			t.Fatalf("config: %v", err)
		}
	}
	offered := &[]int{}
	outcomes := map[int]string{}
	a.ReapOrphans = func(targets []Proc) map[int]string {
		for _, p := range targets {
			*offered = append(*offered, p.PID)
		}
		return outcomes
	}
	return a, offered, outcomes
}

func TestLoadGuardKillConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cfg   string
		armed bool
		warn  string
	}{
		// Absent is OFF, and absent is what this bead ships: the live flip
		// is the operator's, behind arm-1 field data.
		{"", false, ""},
		{"load_guard_kill: false\n", false, ""},
		{"load_guard_kill: true\n", true, ""},
		// A typo must not read as a declaration nobody made, in either
		// direction, and it must be visible.
		{"load_guard_kill: yes\n", false, "not true or false"},
		{"load_guard_kill: 1\n", false, "not true or false"},
		{"load_guard_kill: TRUE\n", false, "not true or false"},
	} {
		a := NewAppAt(t.TempDir())
		if tc.cfg != "" {
			os.WriteFile(a.ConfigPath, []byte(tc.cfg), 0o644)
		}
		got, warn := a.LoadGuardKill()
		if got != tc.armed {
			t.Errorf("LoadGuardKill(%q) = %v, want %v", tc.cfg, got, tc.armed)
		}
		if tc.warn == "" && warn != "" {
			t.Errorf("LoadGuardKill(%q) said %q and should have been quiet", tc.cfg, warn)
		}
		if tc.warn != "" && !strings.Contains(warn, tc.warn) {
			t.Errorf("LoadGuardKill(%q) must name the typo, said %q", tc.cfg, warn)
		}
	}
}

// The shipped default. Arm 1's wording is load-bearing here: it is what the
// field data the operator is reading was gathered under.
func TestKillArmIsOffUntilTheOperatorFlipsIt(t *testing.T) {
	t.Parallel()
	a, offered, _ := killArmApp(t, false)
	a.TopCPU = func() ([]Proc, error) { return leakRows(16), nil }
	got := a.LoadCulpritLine()
	if !strings.Contains(got, "REPORT ONLY, nothing was killed") {
		t.Errorf("the shipped default must still be arm 1:\n%s", got)
	}
	if len(*offered) != 0 {
		t.Errorf("a disarmed guard offered %v to the reaper", *offered)
	}
}

// A typo in the key is visible in the line the guard was already printing —
// LoadCulpritLine takes no writer, and a warning nobody sees is a kill arm
// somebody believes is on.
func TestAMisspeltKillKeyIsNamedInTheLine(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	os.WriteFile(a.ConfigPath, []byte("load_guard_kill: yes\n"), 0o644)
	a.TopCPU = func() ([]Proc, error) { return leakRows(1), nil }
	got := a.LoadCulpritLine()
	if !strings.Contains(got, `load_guard_kill: "yes" is not true or false`) ||
		!strings.Contains(got, "the kill arm stays OFF") {
		t.Errorf("the typo must be named where the report is read:\n%s", got)
	}
	if !strings.Contains(got, "REPORT ONLY") {
		t.Errorf("and the arm must stay off:\n%s", got)
	}
}

func TestArmedGuardOffersEveryUndeclaredLeakAndNamesEveryAttempt(t *testing.T) {
	t.Parallel()
	a, offered, outcomes := killArmApp(t, true)
	rows := leakRows(16)
	for _, p := range rows {
		outcomes[p.PID] = "TERM: gone"
	}
	outcomes[rows[2].PID] = "STILL RUNNING after TERM and KILL"
	a.TopCPU = func() ([]Proc, error) { return rows, nil }
	got := a.LoadCulpritLine()

	if len(*offered) != 16 {
		t.Errorf("every undeclared leak must be offered, got %d: %v", len(*offered), *offered)
	}
	if !strings.Contains(got, "load_guard_kill: true, 15 of 16 ended:") {
		t.Errorf("the header must count what actually ended:\n%s", got)
	}
	if strings.Contains(got, "REPORT ONLY") || strings.Contains(got, "more like these") {
		t.Errorf("an armed guard neither reports-only nor elides an attempt:\n%s", got)
	}
	// Deliverable 4: pid, argv head, signal and outcome, for EVERY attempt.
	// A kill is irreversible and this line is the only record it happened,
	// so three-and-count is not a log.
	for _, p := range rows {
		if !strings.Contains(got, "pid "+strconv.Itoa(p.PID)+" ") {
			t.Errorf("attempt on pid %d was not named:\n%s", p.PID, got)
		}
	}
	if n := strings.Count(got, "— TERM: gone"); n != 15 {
		t.Errorf("want 15 outcome words, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, "STILL RUNNING after TERM and KILL") {
		t.Errorf("a pid that would not die must say so rather than be counted dead:\n%s", got)
	}
	if !strings.Contains(got, "MARK=teau2;") || strings.Contains(got, gateShellPreambleHead) {
		t.Errorf("the attempt must name the persona's command, not our preamble:\n%s", got)
	}
}

// "Already gone before it was signalled" is not something this arm ended,
// and the header must not claim it. A count that drifts from the body is a
// wrong claim about an irreversible act.
func TestAlreadyGoneIsNotCountedAsEnded(t *testing.T) {
	t.Parallel()
	a, _, outcomes := killArmApp(t, true)
	rows := leakRows(2)
	outcomes[rows[0].PID] = killedByTERM
	outcomes[rows[1].PID] = "already gone before it was signalled"
	a.TopCPU = func() ([]Proc, error) { return rows, nil }
	got := a.LoadCulpritLine()
	if !strings.Contains(got, "load_guard_kill: true, 1 of 2 ended:") {
		t.Errorf("only what this arm ended may be counted:\n%s", got)
	}
	if !strings.Contains(got, "already gone before it was signalled") {
		t.Errorf("and the other one must still be named:\n%s", got)
	}
}

// A pid the reaper never reported on is not a pid we may call dead.
func TestAnUnreportedPidIsNotCalledEnded(t *testing.T) {
	t.Parallel()
	a, _, _ := killArmApp(t, true)
	a.TopCPU = func() ([]Proc, error) { return leakRows(2), nil }
	got := a.LoadCulpritLine()
	if !strings.Contains(got, "load_guard_kill: true, 0 of 2 ended:") ||
		strings.Count(got, "not re-verified: skipped") != 2 {
		t.Errorf("silence from the reaper is a skip, not a kill:\n%s", got)
	}
}

// ─── declare or die ─────────────────────────────────────────────────────────

func TestOrphanDeclaredReadsTheMarkerAndItsReason(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		args   string
		reason string
		ok     bool
	}{
		{"the documented form, head of the line",
			gateArgv("POSSE_KEEP=ranger-base-abcd ./bench.sh &"), "ranger-base-abcd", true},
		{"the assignment-statement form",
			gateArgv("POSSE_KEEP=ranger-base-abcd; cd /repo && ./bench.sh &"), "ranger-base-abcd", true},
		// Deeper in the line still spares. The marker is the SPARE and a
		// loose read there costs a leak that arm 1 still reports, while a
		// tight read costs a process somebody meant to keep — which is
		// irreversible. The two anchors point opposite ways on purpose.
		{"further down the line", gateArgv("cd /repo && POSSE_KEEP=bench ./bench.sh &"), "bench", true},
		{"quoted", gateArgv("POSSE_KEEP='ranger-base-abcd' ./bench.sh &"), "ranger-base-abcd", true},
		{"no reason given still declares", gateArgv("POSSE_KEEP= ./bench.sh &"), "", true},
		{"nothing of the sort", gateArgv(teauPayload), "", false},
		// The token carries its own `=`, so the longer name is a different
		// word and not a declaration.
		{"a near miss is not the marker", gateArgv("POSSE_KEEPER=x ./bench.sh &"), "", false},
		{"an empty argv", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason, ok := orphanDeclared(tc.args)
			if ok != tc.ok || reason != tc.reason {
				t.Errorf("orphanDeclared(...) = %q, %v; want %q, %v", reason, ok, tc.reason, tc.ok)
			}
		})
	}
}

// The whole point of the ruling: a declared process is never offered to the
// reaper, is never counted as a leak, and is named loudly instead.
func TestADeclaredOrphanIsSparedAndSaidSo(t *testing.T) {
	t.Parallel()
	for _, on := range []bool{false, true} {
		a, offered, _ := killArmApp(t, on)
		rows := leakRows(3)
		rows[1].Args = gateArgv("POSSE_KEEP=ranger-base-abcd (while :; do :; done) &")
		a.TopCPU = func() ([]Proc, error) { return rows, nil }
		got := a.LoadCulpritLine()

		for _, pid := range *offered {
			if pid == rows[1].PID {
				t.Errorf("armed=%v: a declared orphan was offered to the reaper:\n%s", on, got)
			}
		}
		if !strings.Contains(got, "1 declared orphan spared (POSSE_KEEP=), not killed:") {
			t.Errorf("armed=%v: the spare must be loud:\n%s", on, got)
		}
		if !strings.Contains(got, "pid "+strconv.Itoa(rows[1].PID)+" 2h30m [ranger-base-abcd]:") {
			t.Errorf("armed=%v: the spare must name pid and reason:\n%s", on, got)
		}
		// Two leaks, not three: while the arm is off this is what keeps the
		// field data the flip is waiting on honest.
		if !strings.Contains(got, "2 orphaned gate-shell children") {
			t.Errorf("armed=%v: a declared process is not one of the leaks:\n%s", on, got)
		}
	}
}

// Declared and nothing else: no leak header at all, and still a witness.
func TestOnlyDeclaredOrphansIsNotALeakReport(t *testing.T) {
	t.Parallel()
	a, offered, _ := killArmApp(t, true)
	rows := leakRows(1)
	rows[0].Args = gateArgv("POSSE_KEEP=ranger-base-abcd (while :; do :; done) &")
	a.TopCPU = func() ([]Proc, error) { return rows, nil }
	got := a.LoadCulpritLine()
	if strings.Contains(got, "orphaned gate-shell chil") {
		t.Errorf("a declared process is not a leaked gate-shell child:\n%s", got)
	}
	if !strings.Contains(got, "1 declared orphan spared") {
		t.Errorf("but it is still named:\n%s", got)
	}
	if len(*offered) != 0 {
		t.Errorf("nothing may be offered: %v", *offered)
	}
}

// A declaration on something that is NOT ours changes nothing: the preamble
// still decides what this arm may touch, and it is still read at the head.
func TestTheMarkerCannotEnrolAStrangerProcess(t *testing.T) {
	t.Parallel()
	a, offered, _ := killArmApp(t, true)
	a.TopCPU = func() ([]Proc, error) {
		return []Proc{{PID: 900, PPID: 1, CPU: 99, Age: time.Hour, Comm: "go",
			Args: "go build ./... POSSE_KEEP=x"}}, nil
	}
	got := a.LoadCulpritLine()
	if strings.Contains(got, "declared orphan") || strings.Contains(got, "orphaned gate-shell") {
		t.Errorf("the operator's own process is not ours to spare OR to kill:\n%s", got)
	}
	if len(*offered) != 0 {
		t.Errorf("nothing may be offered: %v", *offered)
	}
}

// ─── the real reaper (ranger-base-gvp2p deliverables 2 and 4) ───────────────

// The re-verify is the FAIL CLOSED half, and it is what a stale table costs.
func TestKillVerifyDropsWhatIsNoLongerAKillTarget(t *testing.T) {
	t.Parallel()
	ours := gateArgv(teauPayload)
	table := strings.Join([]string{
		"  100     1 " + ours, // still a leak
		"  101  4021 " + ours, // reparented back: not an orphan
		"  102     1 " + gateArgv("POSSE_KEEP=ranger-base-abcd sleep 1"), // declared in between
		"  103     1 go build ./...",                                     // recycled onto something else
		"  104     1",                                                    // unreadable row
	}, "\n")
	rows := parseVerifyTable(table)
	if len(rows) != 4 {
		t.Fatalf("want 4 readable rows, got %d: %v", len(rows), rows)
	}
	if rows[100].PPID != 1 || rows[100].Args != ours {
		t.Errorf("the argv must come back whole and unshifted: %+v", rows[100])
	}
	// The predicate killVerify re-applies, row by row, so a change to
	// either half is caught here as well as in the reaper.
	for pid, want := range map[int]bool{100: true, 101: false, 102: false, 103: false} {
		r := rows[pid]
		_, oursNow := gateShellForkPayload(r.Args)
		_, declared := orphanDeclared(r.Args)
		if got := r.PPID == 1 && oursNow && !declared; got != want {
			t.Errorf("pid %d killable = %v, want %v (%+v)", pid, got, want, r)
		}
	}
}

// The measured signal (deliverable 2), on a planted control small enough for
// the operator's no-load-testing rule: one bounded busy loop that exits on
// its own if every signal fails, killed here regardless.
//
// Two arms, and the second is what makes the first mean anything: a payload
// with a TERM handler is what a persona's own cleanup looks like from
// outside, and if the escalation to KILL were decorative that arm would
// survive both signals.
func TestTheReaperEndsATermIgnoringChildToo(t *testing.T) {
	t.Parallel()
	// Bounded: ~seconds of one core, and it ends by itself if nothing else
	// ends it. Never a sleep — a sleeping process measures no signal a busy
	// one does not, and this is the one arm where the box is really touched.
	const bound = `n=0; while [ $n -lt 40000000 ]; do n=$((n+1)); done`
	for _, tc := range []struct {
		name  string
		setup string
		want  string
	}{
		// setup is what the payload does BEFORE it announces itself, so an
		// arm's disposition is always installed before this test is allowed
		// to signal it. Everything after the announcement is the same loop.
		{"default disposition", "", killedByTERM},
		{"a payload that ignores TERM", `trap "" TERM; `, killedByKILL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The payload says when it is ready; the test never guesses.
			// `echo $!` prints the pid the instant sh FORKS the subshell,
			// before the child has run a single builtin, so the TERM-ignoring
			// arm still carried the DEFAULT disposition for a window that the
			// very next act here — signalPID(TERM) — could land in. Under
			// fleet load that window is wide enough to hit, and the arm whose
			// whole job is to survive TERM died to it (ranger-base-85scr).
			// procAlive cannot close the window: the pid exists from the fork
			// onward and answers "there is a process", never "the trap is
			// installed". A poll on the trap from outside is not portable and
			// a fixed sleep only buys the same race at a different width, so
			// the readiness edge has to come from the payload: it creates the
			// marker as its first act AFTER its setup, and this waits on that.
			ready := filepath.Join(t.TempDir(), "ready")
			body := tc.setup + ": > '" + ready + "'; " + bound
			out, err := exec.Command("/bin/sh", "-c", "("+body+") >/dev/null 2>&1 & echo $!").Output()
			if err != nil {
				t.Fatalf("plant: %v", err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
			if err != nil {
				t.Fatalf("plant printed no pid: %q", out)
			}
			defer syscall.Kill(pid, syscall.SIGKILL)
			if !procAlive(pid) {
				t.Fatalf("the planted control was gone before it was signalled — nothing was measured")
			}
			// Bounded, and generous: this is a wait for one shell builtin and
			// one redirect on a box that may be running several suites, not a
			// measurement of anything. Blowing it means the plant never ran,
			// which is a broken control, not a broken ladder.
			for deadline := time.Now().Add(30 * time.Second); ; {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				if !procAlive(pid) {
					t.Fatalf("the planted control died before it was ready — nothing was measured")
				}
				if time.Now().After(deadline) {
					t.Fatalf("the planted control never announced it was ready — nothing was measured")
				}
				time.Sleep(time.Millisecond)
			}
			// sysReapOrphans' own re-verify would refuse this pid (its argv
			// is not a gate shell's), which is the point of that half; the
			// signal ladder is what is under test, so drive it directly.
			got := map[int]string{}
			if err := signalPID(pid, syscall.SIGTERM); err != nil {
				t.Fatalf("TERM: %v", err)
			}
			left := waitGone([]int{pid}, loadKillGrace, got, killedByTERM)
			for _, p := range left {
				if err := signalPID(p, syscall.SIGKILL); err != nil {
					t.Fatalf("KILL: %v", err)
				}
			}
			for _, p := range waitGone(left, loadKillConfirm, got, killedByKILL) {
				got[p] = "STILL RUNNING after TERM and KILL"
			}
			if got[pid] != tc.want {
				t.Errorf("pid %d: got %q, want %q", pid, got[pid], tc.want)
			}
			if procAlive(pid) {
				t.Errorf("pid %d outlived the ladder", pid)
			}
		})
	}
}

// The FAIL CLOSED half, end to end, against a REAL process — the one arm of
// this that a caged session can measure without a process table.
//
// The census is faked (TopCPU), so the guard is told a live, real pid is an
// orphaned leak of ours. The REAPER is the real one. It must not die, for
// whichever of the two reasons this box supplies: where `ps` runs, the
// re-verify sees a ppid that is not 1 and refuses a stale row (the
// pid-recycling defence); where `ps` is exec-denied, the table cannot be
// re-read at all and a reading it cannot take kills nothing. Both are the
// same rule, and either one failing kills something the census was wrong
// about.
func TestAStaleCensusRowKillsNothing(t *testing.T) {
	t.Parallel()
	const bound = `n=0; while [ $n -lt 40000000 ]; do n=$((n+1)); done`
	live := exec.Command("/bin/sh", "-c", bound)
	if err := live.Start(); err != nil {
		t.Fatalf("plant: %v", err)
	}
	pid := live.Process.Pid
	defer func() { live.Process.Kill(); live.Wait() }()

	a := NewAppAt(t.TempDir()) // ReapOrphans nil: the REAL reaper
	os.WriteFile(a.ConfigPath, []byte("load_guard_kill: true\n"), 0o644)
	a.TopCPU = func() ([]Proc, error) {
		// Everything the census can be wrong about, asserted as true.
		return []Proc{{PID: pid, PPID: 1, CPU: 99, Age: time.Hour, Comm: "sh",
			Args: gateArgv(teauPayload)}}, nil
	}
	got := a.LoadCulpritLine()
	if !strings.Contains(got, "load_guard_kill: true, 0 of 1 ended:") {
		t.Errorf("a row the reaper could not re-verify must end nothing:\n%s", got)
	}
	if !strings.Contains(got, "not signalled:") {
		t.Errorf("and it must say WHY it signalled nothing:\n%s", got)
	}
	if !procAlive(pid) {
		t.Fatalf("the real process pid %d was ended off a stale census row:\n%s", pid, got)
	}
	t.Logf("reaper said: %s", got)
}

// Fail open (deliverable 4): the arm is bounded by ONE shared grace, not one
// per pid, so forty leaks cost what one costs — and a pid nothing can end is
// a word in a line rather than a pass that is late.
func TestTheKillArmIsBoundedWhateverItIsHandedAndNeverErrors(t *testing.T) {
	t.Parallel()
	// A pid that cannot exist: signal 0 says gone, so the ladder must not
	// wait on it at all.
	var many []Proc
	for i := 0; i < 40; i++ {
		many = append(many, Proc{PID: -(i + 2), PPID: 1, CPU: 99, Age: time.Hour})
	}
	start := time.Now()
	out := sysReapOrphans(many)
	if el := time.Since(start); el > loadKillVerifyTimeout+loadKillGrace+loadKillConfirm+2*time.Second {
		t.Errorf("the kill arm took %s — it must be bounded whatever it is handed", el)
	}
	if len(out) != len(many) {
		t.Errorf("every pid offered must get an outcome: %d of %d", len(out), len(many))
	}
	for _, p := range many {
		if out[p.PID] == "" {
			t.Errorf("pid %d got no outcome at all", p.PID)
		}
	}
	// And nothing at all is an immediate, empty answer.
	if got := sysReapOrphans(nil); len(got) != 0 {
		t.Errorf("an empty batch must be an empty answer, got %v", got)
	}
}

// kill(2) reads 0 as "my own process group" and a negative pid as a whole
// process group. A mis-parsed `ps` row must therefore never reach the
// kernel, or one leak becomes a group kill of the fleet — and every path
// into this arm goes through these two.
func TestTheArmRefusesAnythingThatIsNotOnePositivePid(t *testing.T) {
	t.Parallel()
	for _, pid := range []int{0, -1, -2, -49235} {
		if err := signalPID(pid, syscall.SIGTERM); err == nil {
			t.Errorf("signalPID(%d) must refuse: 0 and negatives name process GROUPS", pid)
		}
		if procAlive(pid) {
			t.Errorf("procAlive(%d) must not ask the kernel about a process group", pid)
		}
	}
	// And the outcome the render prints says which pid it refused.
	out := sysReapOrphans([]Proc{{PID: -49235, PPID: 1, CPU: 99, Age: time.Hour}})
	if !strings.Contains(out[-49235], "not signalled") {
		t.Errorf("a refused pid must still get an audited outcome, got %q", out[-49235])
	}
}

// The marker's spelling reaches the persona at the moment it is being told
// it leaked something — the only moment it is worth anything.
func TestSelfCheckTellsAPersonaHowToDeclare(t *testing.T) {
	t.Parallel()
	leak := Proc{PID: 900, PPID: 1, Age: time.Hour, Args: teauPayload}
	kept := Proc{PID: 901, PPID: 1, Age: time.Hour, Args: "POSSE_KEEP=ranger-base-abcd ./bench.sh &"}

	note := SelfOrphanKeepNote([]Proc{leak})
	if !strings.Contains(note, LoadOrphanKeepMarker+"<reason>") {
		t.Errorf("the note must carry the literal token: %q", note)
	}
	// Telling a persona to declare what it has already declared is how a
	// tool teaches people to stop reading it.
	if got := SelfOrphanKeepNote([]Proc{kept}); got != "" {
		t.Errorf("nothing left to declare, so nothing to say: %q", got)
	}
	if got := SelfOrphanKeepNote([]Proc{leak, kept}); got == "" {
		t.Errorf("one undeclared leak is still worth the note")
	}
	// And the census still lists the declared one, marked — it IS running.
	out := FormatSelfOrphans([]Proc{leak, kept})
	if !strings.Contains(out, "pid 901, 1h00m old [declared POSSE_KEEP=ranger-base-abcd]:") {
		t.Errorf("a declared process must be listed and marked:\n%s", out)
	}
	if strings.Contains(out, "pid 900, 1h00m old [declared") {
		t.Errorf("an undeclared leak must not be marked:\n%s", out)
	}
	src, err := os.ReadFile("../../cmd/checkorphans/main.go")
	if err != nil {
		t.Fatalf("read checkorphans: %v", err)
	}
	if !strings.Contains(string(src), "SelfOrphanKeepNote(") {
		t.Errorf("checkorphans must print it — it is the tool AGENTS.md sends personas to")
	}
}

// The marker is documented where a persona is told to look, and the docs
// carry the LITERAL token rather than a description of it.
func TestTheMarkerIsDocumentedForEveryPersona(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"../../AGENTS.md", "../../NOTES.md"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(b), LoadOrphanKeepMarker) {
			t.Errorf("%s must carry the literal %q — a marker nobody knows the spelling of is a marker nobody writes", path, LoadOrphanKeepMarker)
		}
	}
}
