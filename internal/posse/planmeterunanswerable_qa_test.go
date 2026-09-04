package posse

// QA pin for the third arm of PlanMeterSpender (verify ranger-base-s5j1t over
// the close of ranger-base-ddivo).
//
// PlanMeterSpender's watch probe is a question with THREE answers, not two:
// a loop is running, no loop is running, and the lock could not be read at
// all. planquiet.go states the rule in as many words — "A probe that cannot
// be ANSWERED reads as spending ... wrong that way costs one request per TTL
// on an idle shop; wrong the other way is this bead" — and nothing measured
// it: the string `the watch-loop lock could not be read` occurs exactly once
// in the tree, in planquiet.go, and no test drove the branch that returns it.
// Measured with a mutant: collapsing that arm to `return ""` left every
// meter, quiet, stale and dispatch pin in this package green.
//
// Collapsing "could not tell" into "no" is this codebase's own recurring
// regression (loop_alive, WatchStatus, the tree-sweep counts, the plan-guard
// reads), and here "no" is the state the bead was filed on: an unarmed guard
// with nothing recorded as spending mutes the meter, so the shop hires
// against a window nobody is reading.
//
// The fixture is the one shape that BLOCKS the answer rather than changing
// it: a lock file that exists — so WatchLoopRunning is past its
// os.IsNotExist arm and cannot answer "free" — and cannot be opened. Both
// control arms are here, because an assertion that the meter is awake is
// green over a rig that never mutes it: the same box with no lock file at
// all is quiet and asks nothing, and the same box with a readable, free lock
// is quiet and asks nothing.

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQAUnreadableWatchLockCountsAsSpending(t *testing.T) {
	t.Parallel()
	// No thresholds and no dollar caps: the cap arm answers "" and the
	// watch probe is the only thing that can decide.
	a, ps := quietRig(t, "")
	lock := WatchLockPath(a)
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}

	// CONTROL 1 — no lock file. The quiet rule the bead did not touch.
	if why := a.PlanMeterSpender(); why != "" {
		t.Fatalf("no lock file: nothing is spending, got %q", why)
	}
	if a.PlanCache("cockpit").Quiet == nil {
		t.Fatal("no lock file: an idle unarmed shop must stay quiet")
	}

	// CONTROL 2 — the lock exists, is readable and is free. Still quiet, so
	// what moves the verdict below is the unreadability and not the file.
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if why := a.PlanMeterSpender(); why != "" {
		t.Fatalf("a free lock is not a running loop, got %q", why)
	}
	if a.PlanCache("cockpit").Quiet == nil {
		t.Fatal("a free lock must leave the shop quiet")
	}
	if got := ps.hits.Load(); got != 0 {
		t.Fatalf("the controls asked %d times, want 0", got)
	}

	// SUBJECT — the answer is blocked, not changed.
	if err := os.Chmod(lock, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(lock, 0o644) })
	// The fixture premise, asserted: a uid that can still open a 0000 file
	// (root) would make every line below measure the readable case.
	if _, err := WatchLoopRunning(a); err == nil {
		t.Skip("this uid can open a 0000 file, so the unanswerable arm is unreachable here")
	}

	why := a.PlanMeterSpender()
	if why == "" {
		t.Error("an unanswerable watch probe read as `nothing is spending` — the meter is muted on a question nobody could answer")
	}
	if q := a.PlanMeterQuiet(io.Discard); q != nil {
		t.Errorf("the meter is quiet over an unanswerable probe: %v", q)
	}
	// And the consequence the bead is about: a caller gets a request out.
	now := time.Date(2026, 9, 3, 21, 23, 0, 0, time.UTC)
	cacheOver(a, ps, "cockpit", now).Read(5 * time.Minute)
	if got := ps.hits.Load(); got != 1 {
		t.Errorf("%d requests with the probe unanswerable, want 1", got)
	}
}
