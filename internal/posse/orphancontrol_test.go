//go:build posse_arm3

package posse

// The PLANTED POSITIVE CONTROL for the load guard's orphan report
// (ranger-base-apwr). A detector that has never fired has not been shown
// able to fire, and the unit pins next door hand the predicate rows that a
// test wrote. This one plants the real thing — a gate-shell child that is
// really reparented to init and really burning a core — and reads it back
// through the real `ps`, the real census and the real render.
//
// It runs ONLY inside the throwaway CPU-limited container that
// scripts/verify-orphan-report.sh starts, because it does on purpose the two
// things the operator has ruled off this box: it loads a CPU, and it leaves
// orphans behind while it measures them (ranger-base-teau, and the coordinator's
// ORDERS). The skip is not a dead pin hiding a broken assertion — the script
// is the way it is run, and it fails loudly there.
//
// FOUR arms, and the three that must stay untouched are what make the first
// one mean anything. Each is run twice — once with the kill arm off, where
// arm 1 names the leak and ends nothing, and once with `load_guard_kill:
// true`, where the leak must be gone from the NEXT census and the other
// three must still be running (ranger-base-gvp2p, deliverable 3):
//
//	gated + orphaned + burning              → named, then killed
//	ungated + orphaned + burning            → silent, survives (a leak, but not OURS)
//	gated + burning + parent alive          → silent, survives (ordinary fleet work)
//	gated + orphaned + burning + POSSE_KEEP → spared, survives (declared)

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// spinner is a subshell that never execs — so it keeps its parent's whole -c
// string, preamble and all — with its output off the pipe its parent's
// caller is waiting on.
const spinner = `(while :; do :; done) >/dev/null 2>&1 & echo $!`

// keepReason is what the declared arm gives as its reason — a bead id, the
// convention the marker is documented with. The control reads it back out of
// the report, so a render that dropped the reason is red here.
const keepReason = "ranger-base-gvp2p"

// declaredSpinner is the same leak with the declare-or-die marker on the
// front of the line. The ASSIGNMENT-STATEMENT form, not the prefix form: an
// assignment prefix is only valid ahead of a simple command, and `(…)` is
// not one — so `POSSE_KEEP=x (while …) &` is a syntax error and this is the
// spelling that actually works in front of a backgrounded subshell.
const declaredSpinner = `POSSE_KEEP=` + keepReason + `; ` + spinner

func TestOrphanReportControlNamesAPlantedLeak(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_ORPHAN_CONTROL") != "1" {
		t.Skip("planted-leak control: it burns a core and strands orphans on purpose, so it runs only under scripts/verify-orphan-report.sh, in a throwaway CPU-limited container (operator ruling, ranger-base-teau)")
	}
	dir := t.TempDir()
	gatesDir := filepath.Join(dir, "gates")
	wrapper, err := writeGateShell("developer", gatesDir, filepath.Join(gatesDir, "bin"), "/bin/sh", "sh")
	if err != nil {
		t.Fatalf("writeGateShell: %v", err)
	}
	kill := func(pid int) { syscall.Kill(pid, syscall.SIGKILL) }

	// An ORPHAN: the parent runs to the end of its command list and exits,
	// which is exactly how teau's sixteen were made. In a container the
	// reaper is pid 1, so the child lands on ppid 1 the same way it lands
	// on launchd here.
	plantOrphan := func(shell, line string) int {
		t.Helper()
		out, err := exec.Command(shell, "-c", line).Output()
		if err != nil {
			t.Fatalf("%s -c: %v", shell, err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			t.Fatalf("%s printed no pid: %q", shell, out)
		}
		t.Cleanup(func() { kill(pid) })
		return pid
	}
	leak := plantOrphan(wrapper, spinner)    // ours, orphaned, burning — the control
	stray := plantOrphan("/bin/sh", spinner) // orphaned and burning, but not ours
	// The DECLARED exception: identical to the leak in every field the
	// guard can read — same wrapper, same spinner, same ppid, same CPU,
	// same age — and separated from it by the marker alone. That is the
	// whole of the ruling, and this is where it is measured.
	declared := plantOrphan(wrapper, declaredSpinner)

	// A LIVE session's busy child: same gate shell, same spinner, parent
	// still there. Ordinary fleet work, and it must never be called a leak.
	live := exec.Command(wrapper, "-c", spinner+"; sleep 600")
	pipe, err := live.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := live.Start(); err != nil {
		t.Fatalf("live parent: %v", err)
	}
	t.Cleanup(func() { live.Process.Kill(); live.Wait() })
	line, err := bufio.NewReader(pipe).ReadString('\n')
	if err != nil {
		t.Fatalf("live parent printed no pid: %v", err)
	}
	busy, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("live parent printed no pid: %q", line)
	}
	t.Cleanup(func() { kill(busy) })

	// Wait out the REAL age floor rather than lowering it: the constant the
	// report ships with is part of what is under test here.
	deadline := time.Now().Add(LoadOrphanMinAge + 2*time.Minute)
	var procs []Proc
	for {
		if time.Now().After(deadline) {
			t.Fatalf("the planted leak never reached %s and %g%% CPU — the control could not be set up, which is not a result: %+v",
				BlindFor(LoadOrphanMinAge), LoadCulpritOrphanCPU, procs)
		}
		time.Sleep(2 * time.Second)
		if procs, err = SysTopCPU(); err != nil {
			t.Fatalf("SysTopCPU: %v", err)
		}
		ready := 0
		for _, p := range procs {
			if (p.PID == leak || p.PID == declared) && p.orphanSuspect() {
				ready++
			}
		}
		if ready == 2 {
			break
		}
	}

	// Every arm must actually be what it says it is, or the two silences
	// below are silence about nothing.
	seen := map[int]Proc{}
	for _, p := range procs {
		seen[p.PID] = p
	}
	for _, w := range []struct {
		name string
		pid  int
		ppid int
	}{{"the planted leak", leak, 1}, {"the ungated orphan", stray, 1},
		{"the DECLARED orphan", declared, 1}, {"the live session's child", busy, live.Process.Pid}} {
		p, ok := seen[w.pid]
		if !ok {
			t.Fatalf("%s (pid %d) is not in the process table at all", w.name, w.pid)
		}
		if p.PPID != w.ppid {
			t.Fatalf("%s (pid %d) has ppid %d, want %d — the arm is not set up", w.name, w.pid, p.PPID, w.ppid)
		}
		if p.CPU < LoadCulpritOrphanCPU {
			t.Fatalf("%s (pid %d) is at %.1f%% CPU, under the %g%% floor — give the container more room", w.name, w.pid, p.CPU, LoadCulpritOrphanCPU)
		}
	}

	a := NewAppAt(t.TempDir()) // TopCPU nil and ReapOrphans nil: the real
	got := a.LoadCulpritLine() // census, the real ps, the real signals
	i := strings.Index(got, "orphaned gate-shell")
	if i < 0 {
		t.Fatalf("the report did not fire on a planted, orphaned, burning gate-shell child (pid %d):\n%s", leak, got)
	}
	report := got[i:]
	if !strings.Contains(report, "pid "+strconv.Itoa(leak)+" ") {
		t.Errorf("the report must name the planted leak, pid %d:\n%s", leak, report)
	}
	if !strings.Contains(report, "(while :; do :; done)") {
		t.Errorf("the report must show the command after the preamble:\n%s", report)
	}
	if strings.Contains(report, "pid "+strconv.Itoa(stray)+" ") {
		t.Errorf("an orphan that did not come out of a gate shell is not ours to name, pid %d:\n%s", stray, report)
	}
	if strings.Contains(report, "pid "+strconv.Itoa(busy)+" ") {
		t.Errorf("a live session's busy child was called a leak, pid %d:\n%s", busy, report)
	}
	if strings.Contains(report, "pid "+strconv.Itoa(declared)+" ") {
		t.Errorf("a DECLARED orphan is not a leak, pid %d:\n%s", declared, report)
	}
	if !strings.Contains(got, "1 declared orphan spared") ||
		!strings.Contains(got, "["+keepReason+"]") {
		t.Errorf("the declared orphan must be named as spared, pid %d:\n%s", declared, got)
	}
	// Arm 1 kills nothing, and the control is where that is measured rather
	// than asserted: all four are still running after the report.
	for _, pid := range []int{leak, stray, busy, declared} {
		if err := syscall.Kill(pid, 0); err != nil {
			t.Errorf("the report ended pid %d — arm 1 must not be able to kill anything: %v", pid, err)
		}
	}

	// ─── arm 2: the same census, armed (ranger-base-gvp2p) ───────────────
	//
	// Same App, same real `ps`, same real render — the ONLY thing that
	// changes is the config key. That is what makes this a measurement of
	// the kill arm rather than of a second test setup.
	if err := os.WriteFile(a.ConfigPath, []byte("load_guard_kill: true\n"), 0o644); err != nil {
		t.Fatalf("arming the kill arm: %v", err)
	}
	if armed, warn := a.LoadGuardKill(); !armed || warn != "" {
		t.Fatalf("the arm did not arm (%v, %q) — nothing below measures anything", armed, warn)
	}
	killed := a.LoadCulpritLine()
	if !strings.Contains(killed, "load_guard_kill: true, 1 of 1 ended:") {
		t.Errorf("the armed line must say what it ended, and end exactly the one leak:\n%s", killed)
	}
	if !strings.Contains(killed, "pid "+strconv.Itoa(leak)+" ") || !strings.Contains(killed, "TERM: gone") {
		t.Errorf("the attempt on pid %d must be logged with its signal and outcome:\n%s", leak, killed)
	}

	// GONE FROM THE NEXT CENSUS, which is the deliverable — the render
	// saying "gone" and the process table agreeing are two different
	// claims, and only the second one is the reaper working.
	next, err := SysTopCPU()
	if err != nil {
		t.Fatalf("SysTopCPU after the kill: %v", err)
	}
	for _, p := range next {
		if p.PID == leak {
			t.Errorf("the planted leak is still in the census after the kill arm ran: %+v", p)
		}
	}
	if err := syscall.Kill(leak, 0); err == nil {
		t.Errorf("the planted leak (pid %d) is still alive after the kill arm ran", leak)
	}

	// And the three that must survive it, still burning. Survival is the
	// half of this control that costs something: a reaper that killed
	// everything would pass every assertion above.
	for _, w := range []struct {
		name string
		pid  int
	}{{"the ungated orphan", stray}, {"the live session's child", busy}, {"the DECLARED orphan", declared}} {
		if err := syscall.Kill(w.pid, 0); err != nil {
			t.Errorf("%s (pid %d) was killed by the arm and must not have been: %v", w.name, w.pid, err)
		}
	}
	if !strings.Contains(killed, "1 declared orphan spared") {
		t.Errorf("the declared orphan must still be named as spared once armed:\n%s", killed)
	}
}
