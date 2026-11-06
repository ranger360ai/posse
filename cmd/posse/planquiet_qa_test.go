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
	"fmt"
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

// quietSeedAge is how old the reading is at the instant the two
// out-of-process `posse status` pins below write it: three hours, which is
// past PlanUsageStaleAfterDefault (2h) on the BLIND side and far enough
// past it that a jump cannot take the premise back under the threshold —
// an age only ever gets older.
const quietSeedAge = 3 * time.Hour

// seedAgedQuietSnapshot seeds the snapshot quietSeedAge old and hands back
// the reference the caller brackets and skips with (ranger-base-dsuk7).
//
// Both pins used to assert "3h00m" exactly, off `time.Now().UTC().Add(-3h)`,
// and that is two statements about a clock the test does not own. The age
// is rendered in ANOTHER process out of a wall-clock timestamp this one
// wrote to disk, so it reds on a wall-clock jump — this box slept 2h51m
// inside one cmd/posse test on 2026-11-04 and a sibling pin printed
// `2h54m` while its monotonic bracket said `3m` (ranger-base-gt0q9) — and,
// independently, on any launch slower than a minute, which in a package
// measured at 590.639s on a loaded box is not far-fetched.
//
// The fix is the bracket gt0q9 landed, not a clock the binary is told
// about: `posse status` reads time.Now() and nothing else, so "stopping
// the clock" for these two would mean a shipped binary that believes an
// env var about what time it is, for the sake of two test strings.
//
// It does not weaken the assertion. What `3h00m` pinned is that the line
// carries the age of the reading it is not refreshing — BlindFor handed
// `now - At` — and agedAgesOf names every age that is, from the write to
// the moment the test checks. Anything outside it is still red. The one
// thing the exact string caught and the bracket does not is a binary that
// rendered one minute too much on a launch that really did take a minute,
// which is the false positive and not a defect. The skip beside it is what
// keeps the bracket tight: with a jump of over a minute skipped, the
// bracket can only ever span the launch's own wall time, so it is two
// members wide at worst.
func seedAgedQuietSnapshot(t *testing.T, stateDir string) time.Time {
	t.Helper()
	at := time.Now().Add(-quietSeedAge)
	seedQuietSnapshot(t, stateDir, at.UTC())
	return at
}

// planLine is the one line of a `posse status` report that carries
// `marker`, or the whole report when no line does.
//
// It is a diagnostic and nothing else: wantOneOf quotes what it was given
// with %q, and `posse status` in a fixture home prints forty lines of bd
// errors and sandbox warnings around the one sentence these pins are
// about — a bracket miss handed the whole thing is a single escaped
// monster line nobody reads. The fallback is the whole report on purpose:
// when the line is missing, missing-from-what is the question.
func planLine(out, marker string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, marker) {
			return l
		}
	}
	return out
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
//
// The snapshot is seeded immediately before the run it brackets rather than
// at a fixed 3m30s: one `runPosse` launch is one process with its own
// clock, and nothing bounds how long it takes (ranger-base-nmab1/boafa), so
// agedAges names every age BlindFor could honestly have printed between the
// seed and the process exiting.
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
	at := time.Now().Add(-agedSeed)
	seedQuietSnapshot(t, filepath.Join(home, "state"), at)

	stdout, stderr, code := runPosse(t, bin, planEnvAt(home, srv.URL+"/usage"), "cost", "--plan")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	var want []string
	for _, age := range agedAges(at) {
		want = append(want, fmt.Sprintf("plan windows: 5h 46%% · 7d 29%%, read %s ago\n", age))
	}
	wantOneOf(t, "stdout", stdout, want, func(got, w string) bool { return got == w })
	if n := hits.Load(); n != 0 {
		t.Errorf("%d requests", n)
	}
}

// A mute nobody can see is the failure this shop check exists to end: with
// the flag set, `posse status` says so and names the last reading it is not
// refreshing. The controls are the two states where the line would be
// furniture or a lie — an unarmed guard (most shops, always silent about a
// meter they do not have) and an armed one that is asking normally.
func TestQAStatusSaysTheMeterIsQuiet(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   string
		wants bool
	}{
		{"quiet flag", "plan_guard_5h: 70\nplan_usage_quiet: true\n", true},
		{"guard off", "", false},
		{"armed and asking", "plan_guard_5h: 70\n", false},
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
			if err := os.MkdirAll(filepath.Join(home, "state"), 0o755); err != nil {
				t.Fatal(err)
			}
			at := seedAgedQuietSnapshot(t, filepath.Join(home, "state"))

			out, _, _ := runPosse(t, bin, planEnvAt(home, srv.URL+"/usage"), "status")
			skipIfTheBoxSleptMidTest(t, at)
			if got := strings.Contains(out, "plan meter QUIET (plan_usage_quiet)"); got != tc.wants {
				t.Errorf("%s: said = %v, want %v:\n%s", tc.name, got, tc.wants, out)
			}
			if tc.wants {
				if !strings.Contains(out, "5h 46% · 7d 29%") {
					t.Errorf("the line must carry the reading it is not refreshing:\n%s", out)
				}
				var ages []string
				for _, age := range agedAgesOf(at, quietSeedAge) {
					ages = append(ages, age+" ago")
				}
				wantOneOf(t, "the line must carry that reading's age",
					planLine(out, "plan meter QUIET"), ages, strings.Contains)
				if strings.Contains(out, "plan meter BLIND") {
					t.Errorf("quiet is not blind — no headroom rule is ruling on anything:\n%s", out)
				}
			}
			if n := hits.Load(); tc.wants && n != 0 {
				t.Errorf("a shop check that reported the quiet gap by breaking it: %d requests", n)
			}
		})
	}
}
