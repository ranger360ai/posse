package main

// QA pins for ranger-base-4rfw1 on the three surfaces that ask: the cockpit
// header, `posse status` and `posse cost`.
//
// MEASURED 2026-09-02. The plan guard's thresholds were commented out and
// the watch restarted, to drain a 429 window. The box was silent for 94
// minutes — and then `posse cockpit`, opened in a herdr pane, asked the
// endpoint itself at 20:13:57Z and again at 21:15:39Z from a second
// instance. Both drew `429 Retry-After: 3600` and re-armed the hour the gap
// was draining ($StateDir/plan-usage.log, caller `cockpit`).
//
// Every pin here counts requests against a real loopback listener, and
// every one has a control arm whose count is NOT zero: an "asked nothing"
// assertion over a rig that cannot ask is green with the fix deleted.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// quietServer is the endpoint, counting. It answers a valid reading, so
// nothing below is passing because the response was unusable.
func quietServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"five_hour":{"utilization":46,"resets_at":"2026-09-03T02:00:00Z"},` +
			`"seven_day":{"utilization":29,"resets_at":"2026-09-08T02:00:00Z"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// quietCockpit is a cockpit in a home of its own, pointed at that endpoint
// through the shipped reader and the shipped cache. `at` is the snapshot's
// timestamp; the zero time seeds no snapshot at all.
func quietCockpit(t *testing.T, cfg string, snapshotAge time.Duration) (*cockpit, *atomic.Int64) {
	t.Helper()
	srv, hits := quietServer(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RHQ_PLAN_USAGE_URL", srv.URL)
	a := posse.NewAppAt(filepath.Join(home, "config"))
	if err := os.MkdirAll(a.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 2, 20, 13, 57, 0, time.UTC)
	if snapshotAge > 0 {
		seedQuietSnapshot(t, a.StateDir, now.Add(-snapshotAge))
	}
	return &cockpit{app: a, now: func() time.Time { return now }, plans: make(chan planRead, 1)}, hits
}

func seedQuietSnapshot(t *testing.T, stateDir string, at time.Time) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"at": at, "windows": []map[string]any{
		{"name": "5h", "pct": 46}, {"name": "7d", "pct": 29},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "plan-usage.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The bead, on the surface that broke it: the cockpit's plan tick asks
// nothing while the meter is quiet, and says which state it is showing.
func TestQACockpitAsksNothingWhileTheMeterIsQuiet(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   string
		age   time.Duration
		wants string
		hits  int64
	}{
		// The state the incident was in, with the reading the 94-minute gap
		// had left behind. The age is the half that keeps it honest: with
		// nobody refreshing it, this number only ever gets older.
		{"guard off, snapshot", "", 2 * time.Minute,
			"5h 46% · 7d 29% · guard off · last reading 2m", 0},
		// Hours old and still shown, still with its age — the alternative
		// is a header that goes blank the moment the guard is switched off,
		// which is how an operator loses the last number anybody had.
		{"guard off, old snapshot", "", 6 * time.Hour,
			"guard off · last reading 6h00m", 0},
		// The flag, with the guard armed: the thresholds stay set (the
		// brake is not what the operator is switching off) and no request
		// goes out.
		{"quiet flag", "plan_guard_5h: 70\nplan_usage_quiet: true\n", 3 * time.Minute,
			"meter quiet · last reading 3m", 0},
		// Armed and quiet with nothing to show: said, because the
		// thresholds are set and the operator believes there is a brake.
		{"quiet flag, no snapshot", "plan_guard_5h: 70\nplan_usage_quiet: true\n", 0,
			"plan — · meter quiet, no reading", 0},
		// THE CONTROL. Same rig, same tick, guard armed and readable: one
		// request. Without this row every assertion above is green over a
		// cockpit that could not have asked anyway.
		{"armed and readable", "plan_guard_5h: 70\n", 0, "5h 46% · 7d 29%", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, hits := quietCockpit(t, tc.cfg, tc.age)
			got := scanOnce(t, c)
			if !strings.Contains(got, tc.wants) {
				t.Errorf("segment = %q, want it to contain %q", got, tc.wants)
			}
			if n := hits.Load(); n != tc.hits {
				t.Errorf("%d requests, want %d", n, tc.hits)
			}
		})
	}
}

// The other control, and the one a "say the state out loud" fix gets wrong
// in the other direction: a shop that never armed the guard and has no
// reading gets no segment at all. A permanent "guard off" on every such
// header is furniture, and furniture is exactly what the header's own blind
// line became on 2026-08-22.
func TestQACockpitSaysNothingWhereThereIsNothingToSay(t *testing.T) {
	c, hits := quietCockpit(t, "", 0)
	if got := scanOnce(t, c); got != "" {
		t.Errorf("an unarmed guard with no reading draws nothing, got %q", got)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("%d requests from a shop with no meter guard", n)
	}
}

// `posse status` and `posse cost`, out of process, through the built
// binary: the same rule, counted at the listener.
func TestQAStatusAndCostAskNothingWhileTheMeterIsQuiet(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  string
		hits int64
	}{
		{"guard off", "", 0},
		{"quiet flag", "plan_guard_5h: 70\nplan_usage_quiet: true\n", 0},
		// The control: armed and readable, both commands read the meter —
		// one request each, since each is its own process and the snapshot
		// an override fetches may not be shared (credpin.go rule 5).
		{"armed and readable", "plan_guard_5h: 70\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := buildRhq(t)
			srv, hits := quietServer(t)
			home := t.TempDir()
			repo := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, "config.yaml"),
				[]byte("beads:\n  - "+repo+"\n"+tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"status"}, {"cost", "--plan"}} {
				runPosse(t, bin, planEnvAt(home, srv.URL+"/usage"), args...)
			}
			if n := hits.Load(); n != tc.hits {
				t.Errorf("%s: %d requests from status+cost, want %d", tc.name, n, tc.hits)
			}
		})
	}
}

// `posse cost --plan` while the meter is quiet: the snapshot, with its age,
// and no request. The reading IS this command's output and a persona greps
// it, so going silent would be the wrong fix — as would printing a
// six-hour-old number with nothing on it.
func TestQACostPlanServesTheSnapshotWhileQuiet(t *testing.T) {
	bin := buildRhq(t)
	srv, hits := quietServer(t)
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	seedQuietSnapshot(t, filepath.Join(home, "state"), time.Now().UTC().Add(-3*time.Minute-30*time.Second))

	stdout, stderr, code := runPosse(t, bin, planEnvAt(home, srv.URL+"/usage"), "cost", "--plan")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "plan windows: 5h 46% · 7d 29%, read 3m ago\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("%d requests", n)
	}
}
