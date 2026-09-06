//go:build !posse_arm2 && !posse_arm3

package posse

// The guard on the shared reading (rangerhq-tdy8): a pass costs the usage
// endpoint a request only when nobody has taken one recently, and a shared
// reading never buys the blind window grace it did not earn.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Consecutive passes inside the TTL are one request. This is the fleet's
// share of the ~30 requests/hour that made the endpoint 429 for three
// hours: a --watch loop on a 30s interval used to ask on every pass.
func TestPlanGuardSharesOneReadingAcrossPasses(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true

	r.run(t)
	if got := r.ps.hits.Load(); got != 1 {
		t.Fatalf("setup: first pass takes the reading, got %d requests", got)
	}
	for _, at := range []time.Duration{30 * time.Second, 2 * time.Minute, 4 * time.Minute} {
		r.at(at)
		r.run(t)
		if got := r.ps.hits.Load(); got != 1 {
			t.Fatalf("a pass %s later must reuse the shared reading, got %d requests", at, got)
		}
	}
	// …and past it, one pass pays for everybody again.
	r.at(6 * time.Minute)
	r.run(t)
	if got := r.ps.hits.Load(); got != 2 {
		t.Errorf("past the TTL the next pass takes a fresh reading, got %d requests", got)
	}
}

// The blind clock counts from when the reading was TAKEN. A cached hit that
// reset it to now would be the cache handing out grace: the guard would go
// on dispatching on a reading that is hours old as long as somebody kept
// serving it. Here the last reading is at T, the pass at T+3m runs on it,
// and the pass at T+11m is past the 10m budget — measured from T.
func TestBlindClockCountsFromTheReadingNotTheHit(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.d.Unattended = true

	if n := r.run(t); n != 1 {
		t.Fatalf("setup: a good reading dispatches, got %d", n)
	}
	r.blind()

	// Inside the TTL: served from the snapshot, no request, pass runs.
	r.at(3 * time.Minute)
	r.run(t)
	if got := r.ps.hits.Load(); got != 1 {
		t.Fatalf("setup: want the cache hit, got %d requests", got)
	}
	if strings.Contains(r.out(), "— skipped") {
		t.Fatalf("a 3m-old reading is inside the budget:\n%s", r.out())
	}

	// Past the TTL the fetch is attempted and fails — and the age it
	// reports is measured from the reading, not from the hit. Use a fresh
	// ready bead: the first one is now held by its session, and the guard is
	// a per-bead launch decision rather than a whole-pass stop.
	if err := os.WriteFile(filepath.Join(r.repo, "fake-ready.json"),
		[]byte(`[{"id":"a-2","title":"u","labels":["go"]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	r.at(11 * time.Minute)
	if n := r.run(t); n != 0 {
		t.Errorf("11m past the last READING is past the 10m budget: %d dispatched\n%s", n, r.out())
	}
	if want := "blind 11m"; !strings.Contains(r.out(), want) {
		t.Errorf("want the age counted from the reading (%s), got:\n%s", want, r.out())
	}
}

// A pass never decides on a reading older than half its blind budget, so a
// tight budget shortens what the cache may serve it — the guard's own
// setting stays the thing that decides.
func TestPlanGuardTightBlindBudgetShortensTheSharedReading(t *testing.T) {
	r := newBlindRig(t, guardOn+"\nplan_guard_blind_max: 2m")
	r.d.Unattended = true

	r.run(t)
	if got := r.ps.hits.Load(); got != 1 {
		t.Fatalf("setup: got %d requests", got)
	}
	// Inside half the budget: the snapshot still answers.
	r.at(50 * time.Second)
	r.run(t)
	if got := r.ps.hits.Load(); got != 1 {
		t.Errorf("50s is inside half a 2m budget, got %d requests", got)
	}
	// Past it: the guard asks for itself, well before the default TTL.
	r.at(70 * time.Second)
	r.run(t)
	if got := r.ps.hits.Load(); got != 2 {
		t.Errorf("past half the budget the guard takes its own reading, got %d requests", got)
	}
}

// The read log is written from the dispatch path too, so the evidence file
// names every poller — that is what makes it an answer to "is the 429
// ours?" rather than half of one.
func TestPlanGuardLogsItsReads(t *testing.T) {
	r := newBlindRig(t, guardOn)
	r.run(t)
	c := r.d.App.PlanCache("dispatch")
	b, err := os.ReadFile(c.Log)
	if err != nil {
		t.Fatalf("dispatch must leave a read-cadence line: %v", err)
	}
	if !strings.Contains(string(b), " dispatch ok") {
		t.Errorf("want a dispatch line, got %q", b)
	}
}
