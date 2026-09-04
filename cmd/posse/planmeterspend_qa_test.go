package main

// QA pins for ranger-base-ddivo on the surface the incident was reproduced
// on: the cockpit header, and the shop check beside it.
//
// MEASURED 2026-09-03. With `plan_guard_5h:`/`plan_guard_7d:` commented out
// since 09-01 and `budget_pass:`/`budget_day:` set, the cockpit was opened
// twice (16:10Z, 19:52Z) with the cooldown fields cleared out of
// state/plan-usage.json by hand — and no request left the machine either
// time. The last reading on the box was stamped 2026-09-01T23:23 local and
// the weekly window was found exhausted by hand.
//
// Requests are counted at a real loopback listener, and every row has a
// counterpart whose count is the other number: an "it asked" pin over a rig
// that asks unconditionally would be green with ranger-base-4rfw1's quiet
// rule deleted, which is the regression this pair of beads is balanced
// between.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The cockpit tick, in the states that differ only in whether the shop is
// spending. An unarmed guard mutes the guard; it does not mute the meter
// while a dollar cap is in force.
//
// No cache-hit row here, deliberately: this rig's clock is fixed at the
// incident hour while the cache ages a snapshot against the real one, so
// every seeded reading is a miss whatever age it is given. The fix moves
// who may ask and not how often, and the TTL half is pinned where it can be
// — three dispatch passes, one request (internal/posse).
func TestQACockpitAsksWhileTheShopSpends(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   string
		age   time.Duration
		wants string
		hits  int64
	}{
		// The repro. Guard unarmed, a cap set, a reading two days old: the
		// tick asks, and the header shows what came back rather than the
		// number nobody was refreshing.
		{"cap set, stale snapshot", "budget_day: 3000\n", 46 * time.Hour, "5h 46% · 7d 29%", 1},
		{"cap set, no snapshot", "budget_pass: 150\n", 0, "5h 46% · 7d 29%", 1},
		// The control, and the state 4rfw1 pinned: nothing armed and
		// nothing spending, so the header shows the old number with its age
		// and asks nobody.
		{"idle, stale snapshot", "", 46 * time.Hour, "guard off · last reading 46h00m", 0},
		// The operator's own mute still outranks the spending. It is the
		// one full mute left, and it has to hold here or a 429 window
		// cannot be drained without switching off Dial E as well.
		{"quiet flag over a cap", "budget_pass: 150\nplan_usage_quiet: true\n", 46 * time.Hour,
			"meter quiet · last reading 46h00m", 0},
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

// `posse status`, out of process: the age of a reading nobody is ruling on
// is the whole of what the operator had no way to see for two days. The
// line is the loud one (ranger-base-lpoui) with its middle clause forked —
// there is no headroom rule running here, and saying there is would be the
// lie that kept this state silent.
//
// Files only either way. A shop check that reported the gap by adding to a
// 429 storm is the joke ranger-base-4rfw1 closed, and the request count is
// how that stays true.
func TestQAStatusSaysTheGuardIsUnarmedOverAnOldReading(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   string
		wants bool
	}{
		// The bead: unarmed guard, a cap in force, a reading three hours
		// old and nothing having asked since.
		{"unarmed and spending", "budget_pass: 150\n", true},
		// The control: nothing spending, so nothing is being burnt against
		// the window and a permanent line on every unarmed shop would be
		// the furniture the header's own blind line became.
		{"unarmed and idle", "", false},
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
			seedQuietSnapshot(t, filepath.Join(home, "state"), time.Now().UTC().Add(-3*time.Hour))

			out, _, _ := runPosse(t, bin, planEnvAt(home, srv.URL+"/usage"), "status")
			if got := strings.Contains(out, "plan meter BLIND 3h00m"); got != tc.wants {
				t.Errorf("%s: said = %v, want %v:\n%s", tc.name, got, tc.wants, out)
			}
			if tc.wants {
				for _, want := range []string{
					"the plan guard is UNARMED (budget_pass:/budget_day: is set)",
					"5h 46% · 7d 29%",
					"no request has left this machine since",
				} {
					if !strings.Contains(out, want) {
						t.Errorf("the line must carry %q:\n%s", want, out)
					}
				}
				if strings.Contains(out, "ruling on it under the headroom rule") {
					t.Errorf("no threshold is set — no rule is ruling on anything:\n%s", out)
				}
			}
			if n := hits.Load(); n != 0 {
				t.Errorf("`posse status` is files only: %d requests", n)
			}
		})
	}
}
