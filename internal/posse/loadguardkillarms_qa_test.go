package posse

// QA pins for the two load-guard arm-2 behaviours that ranger-base-gvp2p's
// close names in as many words and that nothing in the suite reached
// (verifying that close under ranger-base-xbox4). Both were found by
// mutation: the line was deleted, the whole arm-2 selection stayed green.
//
//  1. "non-positive pids refused" (deliverable 4). signalPID's own guard can
//     be deleted and TestTheArmRefusesAnythingThatIsNotOnePositivePid still
//     passes — because os.Process.Signal refuses pid 0 by itself, and the
//     kernel refuses a stray process GROUP with EPERM/ESRCH. The pin was
//     green on somebody else's refusal, so the sentence it was written to
//     hold was never measured. A refusal that comes back from the kernel
//     means the group kill was actually ATTEMPTED, which is the whole thing
//     the guard exists to prevent.
//
//  2. killVerify's fail-closed re-verify. Its five branches decide what a
//     destructive arm signals, and the test named for it never calls it —
//     TestKillVerifyDropsWhatIsNoLongerAKillTarget parses a table and then
//     re-implements the predicate by hand (`r.PPID == 1 && oursNow &&
//     !declared`). A hand-written twin of the code under test agrees with
//     it by construction and drifts from it silently: deleting the
//     declared-in-the-meantime branch outright leaves that test green.
//
// Neither is a defect in shipped behaviour — the code is right both times.
// They are the arm's two audit promises with no reader.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The refusal must be THIS arm's, named in its own words. Asserting only
// that some error came back is satisfied by the kernel declining a process
// group the guard should never have named — a pass that means the opposite
// of what it says.
func TestSignalPIDRefusesANonPositivePidInItsOwnWords(t *testing.T) {
	t.Parallel()
	for _, pid := range []int{0, -1, -2, -49235} {
		err := signalPID(pid, syscall.SIGTERM)
		if err == nil {
			t.Errorf("signalPID(%d) must refuse: 0 and negatives name process GROUPS", pid)
			continue
		}
		if !strings.Contains(err.Error(), "refusing to signal pid") {
			t.Errorf("signalPID(%d) was refused by %q — that is the kernel or the runtime, "+
				"not this arm's guard, so the group signal was attempted before it was declined "+
				"(ranger-base-gvp2p deliverable 4)", pid, err)
		}
	}
}

// fakePS puts a `ps` on PATH that prints one fixed table, so killVerify's own
// branches can be driven over rows a real process table will not hold on
// demand — a reparented orphan, a pid recycled onto a stranger's argv, and a
// marker that appeared in the window between the census and the signal.
func fakePS(t *testing.T, table string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncat <<'ROWS'\n" + table + "ROWS\n"
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// killVerify is the fail-closed half, and every row it drops must say why:
// "a destructive arm that says nothing about a target it skipped is a
// destructive arm nobody can audit" is the function's own comment. This
// calls it rather than restating its predicate.
func TestKillVerifyItselfNamesTheOutcomeOfEveryRowItDrops(t *testing.T) {
	ours := gateArgv(teauPayload)
	declared := gateArgv("POSSE_KEEP=ranger-base-abcd " + teauPayload)
	fakePS(t, ""+
		"  100     1 "+ours+"\n"+ // still a leak: the only killable row
		"  101  4021 "+ours+"\n"+ // reparented back onto a live session
		"  102     1 "+declared+"\n"+ // declared in the window
		"  103     1 go build ./...\n") // pid recycled onto a stranger
	// 104 is deliberately absent from the table: gone on its own.

	targets := []Proc{}
	for _, pid := range []int{100, 101, 102, 103, 104} {
		targets = append(targets, Proc{PID: pid, PPID: 1, CPU: 99, Age: time.Hour, Args: ours})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := map[int]string{}
	live := killVerify(ctx, targets, out)

	// The control. If the fake never ran, killVerify read the real table for
	// pids it does not own and every row below is green for the wrong
	// reason — so say that, rather than reporting a clean verdict.
	if strings.Contains(out[100], "could not be re-read") {
		t.Fatalf("the planted `ps` was never reached (%q) — every assertion below would measure nothing", out[100])
	}

	if len(live) != 1 || !live[100] {
		t.Errorf("only the row that is still a leak may be signalled, got live=%v", live)
	}
	if out[100] != "" {
		t.Errorf("a killable row needs no excuse written for it, got %q", out[100])
	}
	for _, tc := range []struct {
		pid  int
		want string
	}{
		{101, "no longer an orphan"},
		{102, "declared " + LoadOrphanKeepMarker},
		{102, "in the meantime"},
		{103, "argv is no longer one of ours"},
		{104, "already gone"},
	} {
		if !strings.Contains(out[tc.pid], tc.want) {
			t.Errorf("pid %d: outcome %q does not say %q — the reason a destructive arm "+
				"skipped a target is the only audit it has (ranger-base-gvp2p)", tc.pid, out[tc.pid], tc.want)
		}
	}
	// Every pid offered gets an entry, the arm's stated promise.
	for _, p := range targets {
		if p.PID == 100 {
			continue
		}
		if out[p.PID] == "" {
			t.Errorf("pid %d was dropped with no outcome recorded at all", p.PID)
		}
	}
}
