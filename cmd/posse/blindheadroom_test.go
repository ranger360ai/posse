package main

// The header half of ranger-base-c3vqe. On 2026-08-31 this segment read
// "— ledger brake" for nineteen hours over a snapshot frozen at 89% while
// the account climbed to 96%: the caps WERE armed and the ledger WAS
// counting, so by the two-state rule the header was telling the truth about
// Dial E and the opposite of what the pass should have been doing.
//
// It is the same rule ranger-base-3nvt wrote for §3 — a header must not name
// a brake that is not holding — reaching the fourth outcome.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// seedSnapshot puts a last reading in the instance's shared snapshot and
// then reads it back through the shipped cache. The read-back is the point:
// this is the one fixture here that is written by hand rather than by the
// code under test, so it is checked against the reader that will consume it
// — a shape change fails here instead of going quietly green on an entry
// nothing can parse.
func seedSnapshot(t *testing.T, a *posse.App, at time.Time, name string, pct float64) {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"at":      at,
		"windows": []map[string]any{{"name": name, "pct": pct}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "plan-usage.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	u, got, ok := a.PlanCache("test").LastReading()
	if !ok || len(u) != 1 || u[0].Name != name || u[0].Pct != pct || !got.Equal(at) {
		t.Fatalf("seed did not read back as one %s window at %g%% taken at %v: %v %v %v", name, pct, at, u, got, ok)
	}
}

// planLedgerCockpit is planClassCockpit with Dial E armed — the incident's
// own config: a guard on a window, caps set, and an endpoint that 401s the
// way a stale credential does.
func planLedgerCockpit(t *testing.T) *cockpit {
	t.Helper()
	c := planClassCockpit(t, http.StatusUnauthorized)
	cfg := c.app.ConfigPath
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, append(b, []byte("plan_guard_7d: 90\nbudget_pass: 30\nbudget_day: 250\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	return c
}

// End to end: real endpoint, real reader, real cache, real scan. A struct
// literal could not pin this — the segment cases below all stay green with
// scanPlan's assignment deleted, which is exactly how a header field that
// nothing sets goes unnoticed.
func TestCockpitHeaderSaysParkedWhenTheLastReadingLeftNoHeadroom(t *testing.T) {
	c := planLedgerCockpit(t)
	seedSnapshot(t, c.app, c.clock().Add(-19*time.Hour), "7d", 89)

	got := scanOnce(t, c)
	if !strings.Contains(got, "no headroom at last reading, parked") {
		t.Errorf("the 19h day must not read as a brake that is holding, got %q", got)
	}
	if strings.Contains(got, "ledger brake") {
		t.Errorf("caps armed is not the same as caps licensing the pass, got %q", got)
	}
	// The clock and the class are untouched by the new clause.
	if !strings.Contains(got, "guard blind") || !strings.Contains(got, "credential stale (401)") {
		t.Errorf("the segment must still carry the blind clock and the class, got %q", got)
	}
}

// WITHOUT-ARM, on the same path: a last reading WITH room is still the
// declared degrade. Delete the new clause and the assertion above fails
// while this one keeps passing — which is what makes the pair a measurement
// rather than a pair of green lines.
func TestCockpitHeaderStillSaysLedgerBrakeWithHeadroom(t *testing.T) {
	c := planLedgerCockpit(t)
	seedSnapshot(t, c.app, c.clock().Add(-19*time.Hour), "7d", 61)

	got := scanOnce(t, c)
	if !strings.Contains(got, "— ledger brake") {
		t.Errorf("a reading with room is ADR 0018 §1 unchanged, got %q", got)
	}
	if strings.Contains(got, "no headroom") {
		t.Errorf("61%% is headroom, got %q", got)
	}
}

// And with no snapshot at all — the 2026-08-26 shape, a credential that
// never read once — the header says what it has always said. The clause
// reports evidence, so with no evidence it reports nothing.
func TestCockpitHeaderWithNoReadingEverIsUnchanged(t *testing.T) {
	c := planLedgerCockpit(t)

	got := scanOnce(t, c)
	if !strings.Contains(got, "— ledger brake") {
		t.Errorf("no reading is §1's arm, got %q", got)
	}
	if strings.Contains(got, "parked") {
		t.Errorf("nothing here is parked, got %q", got)
	}
}

// The segment's own arithmetic, without the scan: the fourth clause and the
// three that were already there, in one place, so their PRECEDENCE is pinned
// too. An unreadable ledger and a reading with no headroom both park; when
// both are true the header names the meter, because that is the one the
// operator has to fix before anything runs again.
func TestCockpitPlanSegmentParkClauses(t *testing.T) {
	at := time.Date(2026, 8, 31, 18, 10, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		unread int
		r      planRead
		want   string
	}{
		{"armed and readable", 0, planRead{guarded: true, ledger: true},
			"plan — · guard blind 19h00m — ledger brake"},
		{"no headroom", 0, planRead{guarded: true, ledger: true, noHeadroom: true},
			"plan — · guard blind 19h00m — no headroom at last reading, parked"},
		{"unreadable ledger", 3, planRead{guarded: true, ledger: true},
			"plan — · guard blind 19h00m — ledger unreadable, parked"},
		{"both parks, the meter is named", 3, planRead{guarded: true, ledger: true, noHeadroom: true},
			"plan — · guard blind 19h00m — no headroom at last reading, parked"},
		// Dial E unset is the unarmed park: no clause then, and none now —
		// this rule is about caps that would otherwise license the degrade.
		{"unarmed", 0, planRead{guarded: true, noHeadroom: true},
			"plan — · guard blind 19h00m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := at
			c := &cockpit{now: func() time.Time { return now }, costUnread: tc.unread}
			// A reading first, so the blind clock counts from something real.
			c.planSegment(planRead{line: "5h 30% · 7d 89%", guarded: true, ledger: true})
			now = now.Add(19 * time.Hour)
			if got := c.planSegment(tc.r); got != tc.want {
				t.Errorf("planSegment = %q, want %q", got, tc.want)
			}
		})
	}
}
